package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
)

// Input validation constants
const (
	MaxCommentLength = 2000
	MaxTags          = 20
	MaxTagLength     = 100
)

// FeedbackRepositoryInterface defines methods for feedback data access
type FeedbackRepositoryInterface interface {
	CreateFeedback(ctx context.Context, req CreateFeedbackRequest) (*Feedback, error)
	GetFeedback(ctx context.Context, id uuid.UUID) (*Feedback, error)
	GetFeedbackByDecision(ctx context.Context, decisionID uuid.UUID) ([]Feedback, error)
	ListFeedback(ctx context.Context, filter FeedbackFilter) ([]Feedback, error)
	UpdateFeedback(ctx context.Context, id uuid.UUID, req UpdateFeedbackRequest) (*Feedback, error)
	DeleteFeedback(ctx context.Context, id uuid.UUID) error
	GetFeedbackStats(ctx context.Context, filter FeedbackFilter) (*FeedbackStats, error)
	GetDecisionsNeedingReview(ctx context.Context, limit int) ([]DecisionNeedingReview, error)
	RefreshStatsView(ctx context.Context) error
}

// FeedbackHandler handles HTTP requests for decision feedback
type FeedbackHandler struct {
	repo FeedbackRepositoryInterface
}

// feedbackRequestBody is the HTTP request body for creating feedback
// (decision_id comes from the path parameter, not the body)
type feedbackRequestBody struct {
	UserID  *uuid.UUID     `json:"user_id,omitempty"`
	Rating  FeedbackRating `json:"rating" binding:"required"`
	Comment *string        `json:"comment,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
}

// NewFeedbackHandler creates a new feedback handler
func NewFeedbackHandler(repo *FeedbackRepository) *FeedbackHandler {
	return &FeedbackHandler{repo: repo}
}

// RegisterRoutes registers all feedback-related routes
func (h *FeedbackHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Feedback routes nested under decisions
	router.POST("/decisions/:id/feedback", h.CreateFeedback)
	router.GET("/decisions/:id/feedback", h.GetDecisionFeedback)

	// Standalone feedback routes
	feedback := router.Group("/feedback")
	{
		feedback.GET("", h.ListFeedback)
		feedback.GET("/stats", h.GetFeedbackStats)
		feedback.GET("/review", h.GetDecisionsNeedingReview)
		feedback.GET("/tags", h.GetCommonTags)
		feedback.POST("/refresh-stats", h.RefreshStats)
		feedback.GET("/:id", h.GetFeedback)
		feedback.PUT("/:id", h.UpdateFeedback)
		feedback.DELETE("/:id", h.DeleteFeedback)
	}
}

// CreateFeedback handles POST /api/v1/decisions/:id/feedback
// @Summary Submit feedback for a decision
// @Tags Feedback
// @Accept json
// @Produce json
// @Param id path string true "Decision ID (UUID)"
// @Param feedback body CreateFeedbackRequest true "Feedback data"
// @Success 201 {object} Feedback "Created feedback"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Decision not found"
// @Failure 409 {object} map[string]string "Duplicate feedback"
// @Router /decisions/{id}/feedback [post]
func (h *FeedbackHandler) CreateFeedback(c *gin.Context) {
	// Parse decision ID from path
	decisionIDStr := c.Param("id")
	decisionID, err := uuid.Parse(decisionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid decision ID format",
		})
		return
	}

	// Parse request body
	var body feedbackRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate rating
	if body.Rating != FeedbackPositive && body.Rating != FeedbackNegative {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Rating must be 'positive' or 'negative'",
		})
		return
	}

	// Validate comment length
	if body.Comment != nil && len(*body.Comment) > MaxCommentLength {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Comment too long, maximum 2000 characters allowed",
		})
		return
	}

	// Validate tags
	if len(body.Tags) > MaxTags {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Too many tags, maximum 20 allowed",
		})
		return
	}
	for _, tag := range body.Tags {
		if len(tag) > MaxTagLength {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Tag too long, maximum 100 characters per tag",
			})
			return
		}
		if len(tag) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Empty tags are not allowed",
			})
			return
		}
	}

	// Build the full request with decision ID from path
	req := CreateFeedbackRequest{
		DecisionID: decisionID,
		UserID:     body.UserID,
		Rating:     body.Rating,
		Comment:    body.Comment,
		Tags:       body.Tags,
	}

	// Create feedback
	feedback, err := h.repo.CreateFeedback(c.Request.Context(), req)
	if err != nil {
		// Check for specific errors
		if err.Error() == "decision not found: "+decisionID.String() {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Decision not found",
			})
			return
		}

		// Check for duplicate constraint violation
		if isDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Feedback already submitted for this decision by this user",
			})
			return
		}

		log.Err(err).Str("decision_id", decisionIDStr).Msg("Failed to create feedback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create feedback",
		})
		return
	}

	c.JSON(http.StatusCreated, feedback)
}

// GetDecisionFeedback handles GET /api/v1/decisions/:id/feedback
// @Summary Get all feedback for a decision
// @Tags Feedback
// @Param id path string true "Decision ID (UUID)"
// @Success 200 {object} map[string]interface{} "Decision feedback"
// @Failure 400 {object} map[string]string "Invalid request"
// @Router /decisions/{id}/feedback [get]
func (h *FeedbackHandler) GetDecisionFeedback(c *gin.Context) {
	// Parse decision ID
	decisionIDStr := c.Param("id")
	decisionID, err := uuid.Parse(decisionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid decision ID format",
		})
		return
	}

	// Get feedback for this decision
	feedbacks, err := h.repo.GetFeedbackByDecision(c.Request.Context(), decisionID)
	if err != nil {
		log.Err(err).Str("decision_id", decisionIDStr).Msg("Failed to get decision feedback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get feedback",
		})
		return
	}

	// Calculate summary
	positiveCount := 0
	negativeCount := 0
	for _, f := range feedbacks {
		if f.Rating == FeedbackPositive {
			positiveCount++
		} else {
			negativeCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"decision_id": decisionID,
		"feedback":    feedbacks,
		"count":       len(feedbacks),
		"summary": gin.H{
			"positive": positiveCount,
			"negative": negativeCount,
		},
	})
}

// ListFeedback handles GET /api/v1/feedback
// @Summary List feedback with filtering
// @Tags Feedback
// @Param rating query string false "Filter by rating (positive, negative)"
// @Param agent_name query string false "Filter by agent name"
// @Param symbol query string false "Filter by symbol"
// @Param decision_type query string false "Filter by decision type"
// @Param from_date query string false "Filter from date (RFC3339)"
// @Param to_date query string false "Filter to date (RFC3339)"
// @Param limit query int false "Limit results (default 50, max 200)"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} map[string]interface{} "Feedback list"
// @Router /feedback [get]
func (h *FeedbackHandler) ListFeedback(c *gin.Context) {
	filter := FeedbackFilter{
		AgentName:    c.Query("agent_name"),
		Symbol:       c.Query("symbol"),
		DecisionType: c.Query("decision_type"),
		Limit:        DefaultFeedbackLimit,
		Offset:       0,
	}

	// Parse rating filter
	if ratingStr := c.Query("rating"); ratingStr != "" {
		rating := FeedbackRating(ratingStr)
		if rating != FeedbackPositive && rating != FeedbackNegative {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Rating must be 'positive' or 'negative'",
			})
			return
		}
		filter.Rating = &rating
	}

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit > MaxFeedbackLimit {
				limit = MaxFeedbackLimit
			}
			if limit > 0 {
				filter.Limit = limit
			}
		}
	}

	// Parse offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// Parse dates
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		if fromDate, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			filter.FromDate = &fromDate
		}
	}
	if toDateStr := c.Query("to_date"); toDateStr != "" {
		if toDate, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			filter.ToDate = &toDate
		}
	}

	// Fetch feedback
	feedbacks, err := h.repo.ListFeedback(c.Request.Context(), filter)
	if err != nil {
		log.Err(err).Msg("Failed to list feedback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list feedback",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"feedback": feedbacks,
		"count":    len(feedbacks),
		"filter":   filter,
	})
}

// GetFeedback handles GET /api/v1/feedback/:id
// @Summary Get feedback by ID
// @Tags Feedback
// @Param id path string true "Feedback ID (UUID)"
// @Success 200 {object} Feedback "Feedback details"
// @Failure 404 {object} map[string]string "Feedback not found"
// @Router /feedback/{id} [get]
func (h *FeedbackHandler) GetFeedback(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid feedback ID format",
		})
		return
	}

	feedback, err := h.repo.GetFeedback(c.Request.Context(), id)
	if err != nil {
		log.Err(err).Str("feedback_id", idStr).Msg("Failed to get feedback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get feedback",
		})
		return
	}

	if feedback == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Feedback not found",
		})
		return
	}

	c.JSON(http.StatusOK, feedback)
}

// UpdateFeedback handles PUT /api/v1/feedback/:id
// @Summary Update existing feedback
// @Tags Feedback
// @Accept json
// @Produce json
// @Param id path string true "Feedback ID (UUID)"
// @Param feedback body UpdateFeedbackRequest true "Updated feedback data"
// @Success 200 {object} Feedback "Updated feedback"
// @Failure 404 {object} map[string]string "Feedback not found"
// @Router /feedback/{id} [put]
func (h *FeedbackHandler) UpdateFeedback(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid feedback ID format",
		})
		return
	}

	var req UpdateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate rating if provided
	if req.Rating != nil {
		if *req.Rating != FeedbackPositive && *req.Rating != FeedbackNegative {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Rating must be 'positive' or 'negative'",
			})
			return
		}
	}

	feedback, err := h.repo.UpdateFeedback(c.Request.Context(), id, req)
	if err != nil {
		log.Err(err).Str("feedback_id", idStr).Msg("Failed to update feedback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update feedback",
		})
		return
	}

	if feedback == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Feedback not found",
		})
		return
	}

	c.JSON(http.StatusOK, feedback)
}

// DeleteFeedback handles DELETE /api/v1/feedback/:id
// @Summary Delete feedback
// @Tags Feedback
// @Param id path string true "Feedback ID (UUID)"
// @Success 204 "Feedback deleted"
// @Failure 404 {object} map[string]string "Feedback not found"
// @Router /feedback/{id} [delete]
func (h *FeedbackHandler) DeleteFeedback(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid feedback ID format",
		})
		return
	}

	// Check if feedback exists first
	existing, err := h.repo.GetFeedback(c.Request.Context(), id)
	if err != nil {
		log.Err(err).Str("feedback_id", idStr).Msg("Failed to check feedback existence")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete feedback",
		})
		return
	}

	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Feedback not found",
		})
		return
	}

	if err := h.repo.DeleteFeedback(c.Request.Context(), id); err != nil {
		log.Err(err).Str("feedback_id", idStr).Msg("Failed to delete feedback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete feedback",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetFeedbackStats handles GET /api/v1/feedback/stats
// @Summary Get aggregated feedback statistics
// @Tags Feedback
// @Param agent_name query string false "Filter by agent name"
// @Param symbol query string false "Filter by symbol"
// @Param from_date query string false "Filter from date (RFC3339)"
// @Param to_date query string false "Filter to date (RFC3339)"
// @Success 200 {object} FeedbackStats "Feedback statistics"
// @Router /feedback/stats [get]
func (h *FeedbackHandler) GetFeedbackStats(c *gin.Context) {
	filter := FeedbackFilter{
		AgentName: c.Query("agent_name"),
		Symbol:    c.Query("symbol"),
	}

	// Parse dates
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		if fromDate, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			filter.FromDate = &fromDate
		}
	}
	if toDateStr := c.Query("to_date"); toDateStr != "" {
		if toDate, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			filter.ToDate = &toDate
		}
	}

	stats, err := h.repo.GetFeedbackStats(c.Request.Context(), filter)
	if err != nil {
		log.Err(err).Msg("Failed to get feedback stats")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get feedback statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetDecisionsNeedingReview handles GET /api/v1/feedback/review
// @Summary Get decisions with multiple negative ratings
// @Tags Feedback
// @Param limit query int false "Limit results (default 20, max 100)"
// @Success 200 {object} map[string]interface{} "Decisions needing review"
// @Router /feedback/review [get]
func (h *FeedbackHandler) GetDecisionsNeedingReview(c *gin.Context) {
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	decisions, err := h.repo.GetDecisionsNeedingReview(c.Request.Context(), limit)
	if err != nil {
		log.Err(err).Msg("Failed to get decisions needing review")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get decisions needing review",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"decisions": decisions,
		"count":     len(decisions),
	})
}

// GetCommonTags handles GET /api/v1/feedback/tags
// @Summary Get list of common feedback tags
// @Tags Feedback
// @Success 200 {object} map[string]interface{} "Common tags"
// @Router /feedback/tags [get]
func (h *FeedbackHandler) GetCommonTags(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tags": CommonFeedbackTags,
	})
}

// RefreshStats handles POST /api/v1/feedback/refresh-stats
// @Summary Refresh the materialized view for feedback statistics
// @Tags Feedback
// @Success 200 {object} map[string]string "Stats refreshed"
// @Router /feedback/refresh-stats [post]
func (h *FeedbackHandler) RefreshStats(c *gin.Context) {
	if err := h.repo.RefreshStatsView(c.Request.Context()); err != nil {
		log.Err(err).Msg("Failed to refresh stats view")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to refresh statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Feedback statistics refreshed successfully",
		"refreshed_at": time.Now(),
	})
}

// isDuplicateKeyError checks if an error is a PostgreSQL unique constraint violation
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}
