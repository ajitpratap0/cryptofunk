package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DecisionRepositoryInterface defines methods for decision data access
type DecisionRepositoryInterface interface {
	ListDecisions(ctx context.Context, filter DecisionFilter) ([]Decision, error)
	GetDecision(ctx context.Context, id uuid.UUID) (*Decision, error)
	GetDecisionStats(ctx context.Context, filter DecisionFilter) (*DecisionStats, error)
	FindSimilarDecisions(ctx context.Context, id uuid.UUID, limit int) ([]Decision, error)
	SearchDecisions(ctx context.Context, req SearchRequest) ([]SearchResult, error)
}

// DecisionHandler handles HTTP requests for decision explainability
type DecisionHandler struct {
	repo DecisionRepositoryInterface
}

// NewDecisionHandler creates a new decision handler
func NewDecisionHandler(repo *DecisionRepository) *DecisionHandler {
	return &DecisionHandler{repo: repo}
}

// RegisterRoutes registers all decision-related routes without rate limiting.
// For production use, prefer RegisterRoutesWithRateLimiter.
func (h *DecisionHandler) RegisterRoutes(router *gin.RouterGroup) {
	h.RegisterRoutesWithRateLimiter(router, nil, nil)
}

// RegisterRoutesWithRateLimiter registers all decision-related routes with rate limiting.
// readMiddleware is applied to GET endpoints, writeMiddleware to POST endpoints.
func (h *DecisionHandler) RegisterRoutesWithRateLimiter(router *gin.RouterGroup, readMiddleware, writeMiddleware gin.HandlerFunc) {
	// Helper to conditionally apply middleware
	applyRead := func(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
		if readMiddleware != nil {
			return append([]gin.HandlerFunc{readMiddleware}, handlers...)
		}
		return handlers
	}
	applyWrite := func(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
		if writeMiddleware != nil {
			return append([]gin.HandlerFunc{writeMiddleware}, handlers...)
		}
		return handlers
	}

	decisions := router.Group("/decisions")
	{
		// Read-only endpoints
		decisions.GET("", applyRead(h.ListDecisions)...)
		decisions.GET("/stats", applyRead(h.GetStats)...) // Must be before :id to avoid conflict
		decisions.GET("/:id", applyRead(h.GetDecision)...)
		decisions.GET("/:id/similar", applyRead(h.GetSimilarDecisions)...)

		// Search endpoint (POST but read-only, apply write middleware for rate limiting)
		decisions.POST("/search", applyWrite(h.SearchDecisions)...)
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
		Limit:        DefaultListLimit,
		Offset:       0,
	}

	// Parse limit (validate non-negative)
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit < 0 {
				limit = DefaultListLimit
			} else if limit > MaxListLimit {
				limit = MaxListLimit
			}
			filter.Limit = limit
		}
	}

	// Parse offset (validate non-negative)
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			if offset < 0 {
				offset = 0
			}
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
		log.Err(err).Msg("Failed to fetch decisions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch decisions",
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
			"error": "Invalid decision ID format",
		})
		return
	}

	// Fetch decision
	decision, err := h.repo.GetDecision(c.Request.Context(), id)
	if err != nil {
		log.Err(err).Str("decision_id", idStr).Msg("Failed to fetch decision")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch decision",
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
			"error": "Invalid decision ID format",
		})
		return
	}

	// Parse limit
	limit := DefaultSimilarLimit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			if parsedLimit > MaxSimilarLimit {
				parsedLimit = MaxSimilarLimit
			}
			limit = parsedLimit
		}
	}

	// Find similar decisions
	similar, err := h.repo.FindSimilarDecisions(c.Request.Context(), id, limit)
	if err != nil {
		log.Err(err).Str("decision_id", idStr).Msg("Failed to find similar decisions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to find similar decisions",
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
		log.Err(err).Msg("Failed to fetch statistics")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// SearchDecisions handles POST /api/v1/decisions/search
// @Summary Search decisions using text or semantic search
// @Description Performs semantic search if embedding is provided (1536-dim vector),
// @Description otherwise falls back to PostgreSQL full-text search on prompt and response fields.
// @Tags Decisions
// @Accept json
// @Produce json
// @Param request body SearchRequest true "Search parameters"
// @Success 200 {object} map[string]interface{} "Search results with relevance scores"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /decisions/search [post]
func (h *DecisionHandler) SearchDecisions(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate that at least query or embedding is provided
	if req.Query == "" && len(req.Embedding) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Either 'query' (text search) or 'embedding' (vector search) must be provided",
		})
		return
	}

	// Validate embedding dimension if provided
	if len(req.Embedding) > 0 && len(req.Embedding) != EmbeddingDimension {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid embedding dimension, must be 1536 (OpenAI text-embedding-ada-002 format)",
		})
		return
	}

	// Validate query length to prevent performance issues
	if len(req.Query) > MaxSearchQueryLength {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Query too long, maximum 500 characters allowed",
		})
		return
	}

	// Perform search
	results, err := h.repo.SearchDecisions(c.Request.Context(), req)
	if err != nil {
		log.Err(err).Str("query", req.Query).Msg("Search failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Search failed",
		})
		return
	}

	// Determine search type for response
	searchType := "text"
	if len(req.Embedding) == EmbeddingDimension {
		searchType = "semantic"
	}

	c.JSON(http.StatusOK, gin.H{
		"results":     results,
		"count":       len(results),
		"search_type": searchType,
		"query":       req.Query,
	})
}
