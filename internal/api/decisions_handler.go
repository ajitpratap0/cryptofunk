package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DecisionHandler handles HTTP requests for decision explainability
type DecisionHandler struct {
	repo *DecisionRepository
}

// NewDecisionHandler creates a new decision handler
func NewDecisionHandler(repo *DecisionRepository) *DecisionHandler {
	return &DecisionHandler{repo: repo}
}

// RegisterRoutes registers all decision-related routes
func (h *DecisionHandler) RegisterRoutes(router *gin.RouterGroup) {
	decisions := router.Group("/decisions")
	{
		decisions.GET("", h.ListDecisions)
		decisions.GET("/:id", h.GetDecision)
		decisions.GET("/:id/similar", h.GetSimilarDecisions)
		decisions.GET("/stats", h.GetStats)
	}
}

// ListDecisions handles GET /api/v1/decisions
// @Summary List LLM decisions with filtering
// @Tags Decisions
// @Param symbol query string false "Filter by symbol (e.g., BTC/USDT)"
// @Param decision_type query string false "Filter by decision type (signal, risk_approval, etc.)"
// @Param outcome query string false "Filter by outcome (SUCCESS, FAILURE, PENDING)"
// @Param model query string false "Filter by model (claude-sonnet-4, gpt-4-turbo, etc.)"
// @Param from_date query string false "Filter from date (RFC3339 format)"
// @Param to_date query string false "Filter to date (RFC3339 format)"
// @Param limit query int false "Limit results (default 50, max 500)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Success 200 {object} map[string]interface{} "List of decisions"
// @Router /decisions [get]
func (h *DecisionHandler) ListDecisions(c *gin.Context) {
	// Parse query parameters
	filter := DecisionFilter{
		Symbol:       c.Query("symbol"),
		DecisionType: c.Query("decision_type"),
		Outcome:      c.Query("outcome"),
		Model:        c.Query("model"),
		Limit:        50, // default
		Offset:       0,  // default
	}

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit > 500 {
				limit = 500 // cap at 500
			}
			filter.Limit = limit
		}
	}

	// Parse offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
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

	// Fetch decisions
	decisions, err := h.repo.ListDecisions(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch decisions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"decisions": decisions,
		"count":     len(decisions),
		"filter":    filter,
	})
}

// GetDecision handles GET /api/v1/decisions/:id
// @Summary Get decision details by ID
// @Tags Decisions
// @Param id path string true "Decision ID (UUID)"
// @Success 200 {object} Decision "Decision details"
// @Failure 404 {object} map[string]string "Decision not found"
// @Router /decisions/{id} [get]
func (h *DecisionHandler) GetDecision(c *gin.Context) {
	// Parse ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid decision ID",
			"details": err.Error(),
		})
		return
	}

	// Fetch decision
	decision, err := h.repo.GetDecision(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch decision",
			"details": err.Error(),
		})
		return
	}

	if decision == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Decision not found",
		})
		return
	}

	c.JSON(http.StatusOK, decision)
}

// GetSimilarDecisions handles GET /api/v1/decisions/:id/similar
// @Summary Find decisions with similar market context (vector similarity)
// @Tags Decisions
// @Param id path string true "Decision ID (UUID)"
// @Param limit query int false "Number of similar decisions to return (default 10, max 50)"
// @Success 200 {object} map[string]interface{} "Similar decisions"
// @Failure 404 {object} map[string]string "Decision not found"
// @Router /decisions/{id}/similar [get]
func (h *DecisionHandler) GetSimilarDecisions(c *gin.Context) {
	// Parse ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid decision ID",
			"details": err.Error(),
		})
		return
	}

	// Parse limit
	limit := 10 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			if parsedLimit > 50 {
				parsedLimit = 50 // cap at 50
			}
			limit = parsedLimit
		}
	}

	// Find similar decisions
	similar, err := h.repo.FindSimilarDecisions(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to find similar decisions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"decision_id": id,
		"similar":     similar,
		"count":       len(similar),
	})
}

// GetStats handles GET /api/v1/decisions/stats
// @Summary Get aggregated decision statistics
// @Tags Decisions
// @Param symbol query string false "Filter by symbol"
// @Param decision_type query string false "Filter by decision type"
// @Param from_date query string false "Filter from date (RFC3339 format)"
// @Param to_date query string false "Filter to date (RFC3339 format)"
// @Success 200 {object} DecisionStats "Aggregated statistics"
// @Router /decisions/stats [get]
func (h *DecisionHandler) GetStats(c *gin.Context) {
	// Parse query parameters
	filter := DecisionFilter{
		Symbol:       c.Query("symbol"),
		DecisionType: c.Query("decision_type"),
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

	// Fetch stats
	stats, err := h.repo.GetDecisionStats(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch statistics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}
