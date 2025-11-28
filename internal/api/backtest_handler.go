package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/backtest"
)

// BacktestHandler handles HTTP requests for backtesting
type BacktestHandler struct {
	jobManager *backtest.JobManager
}

// NewBacktestHandler creates a new backtest handler
func NewBacktestHandler(db *pgxpool.Pool) *BacktestHandler {
	return &BacktestHandler{
		jobManager: backtest.NewJobManager(db),
	}
}

// RunBacktestRequest defines the request body for starting a backtest
type RunBacktestRequest struct {
	Name           string                 `json:"name" binding:"required"`
	StartDate      string                 `json:"start_date" binding:"required"`
	EndDate        string                 `json:"end_date" binding:"required"`
	Symbols        []string               `json:"symbols" binding:"required,min=1"`
	InitialCapital float64                `json:"initial_capital" binding:"required,gt=0"`
	Strategy       map[string]interface{} `json:"strategy" binding:"required"`
	ParameterGrid  map[string]interface{} `json:"parameter_grid,omitempty"`
}

// RunBacktest starts a new backtest job (async)
// @Summary Start a backtest job
// @Tags Backtest
// @Accept json
// @Produce json
// @Param request body RunBacktestRequest true "Backtest configuration"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/backtest/run [post]
func (h *BacktestHandler) RunBacktest(c *gin.Context) {
	var req RunBacktestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid start_date format",
			"details": "Expected format: YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid end_date format",
			"details": "Expected format: YYYY-MM-DD",
		})
		return
	}

	// Extract user info from context (if authentication is implemented)
	createdBy := c.GetString("user_id")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	// Create job
	job := &backtest.BacktestJob{
		Name:           req.Name,
		StartDate:      startDate,
		EndDate:        endDate,
		Symbols:        req.Symbols,
		InitialCapital: req.InitialCapital,
		StrategyConfig: req.Strategy,
		ParameterGrid:  req.ParameterGrid,
		CreatedBy:      createdBy,
	}

	ctx := c.Request.Context()
	if err := h.jobManager.CreateJob(ctx, job); err != nil {
		log.Error().Err(err).Msg("Failed to create backtest job")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create backtest job",
			"details": err.Error(),
		})
		return
	}

	// TODO: Trigger async backtest execution
	// For now, we just create the job and return
	// In a production system, this would enqueue the job to a worker queue (e.g., NATS, Redis Queue)

	c.JSON(http.StatusAccepted, gin.H{
		"id":      job.ID.String(),
		"status":  job.Status,
		"message": "Backtest job created successfully. Use GET /api/v1/backtest/:id to check status.",
	})
}

// GetBacktest retrieves a backtest job by ID
// @Summary Get backtest status and results
// @Tags Backtest
// @Produce json
// @Param id path string true "Backtest Job ID"
// @Success 200 {object} backtest.BacktestJob
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/backtest/{id} [get]
func (h *BacktestHandler) GetBacktest(c *gin.Context) {
	idStr := c.Param("id")

	// Parse UUID
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid job ID format",
			"details": "Expected UUID format",
		})
		return
	}

	ctx := c.Request.Context()
	job, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		log.Warn().Err(err).Str("job_id", idStr).Msg("Backtest job not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Backtest job not found",
			"job_id":  idStr,
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ListBacktests retrieves a paginated list of backtest jobs
// @Summary List user's backtests
// @Tags Backtest
// @Produce json
// @Param limit query int false "Number of results per page" default(20)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/backtest [get]
func (h *BacktestHandler) ListBacktests(c *gin.Context) {
	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid limit parameter",
			"details": "Limit must be between 1 and 100",
		})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid offset parameter",
			"details": "Offset must be >= 0",
		})
		return
	}

	// Extract user info from context (if authentication is implemented)
	createdBy := c.GetString("user_id")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	ctx := c.Request.Context()
	jobs, total, err := h.jobManager.ListJobs(ctx, createdBy, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list backtest jobs")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list backtest jobs",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"backtests": jobs,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
		"has_more":  offset+len(jobs) < total,
	})
}

// DeleteBacktest deletes a backtest job
// @Summary Delete a backtest job
// @Tags Backtest
// @Param id path string true "Backtest Job ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/backtest/{id} [delete]
func (h *BacktestHandler) DeleteBacktest(c *gin.Context) {
	idStr := c.Param("id")

	// Parse UUID
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid job ID format",
			"details": "Expected UUID format",
		})
		return
	}

	// Check if job exists and belongs to user
	ctx := c.Request.Context()
	job, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Backtest job not found",
			"job_id":  idStr,
			"details": err.Error(),
		})
		return
	}

	// Verify ownership (if authentication is implemented)
	createdBy := c.GetString("user_id")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	if job.CreatedBy != createdBy {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You don't have permission to delete this backtest job",
		})
		return
	}

	// Delete the job
	if err := h.jobManager.DeleteJob(ctx, jobID); err != nil {
		log.Error().Err(err).Str("job_id", idStr).Msg("Failed to delete backtest job")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete backtest job",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backtest job deleted successfully",
		"job_id":  idStr,
	})
}

// CancelBacktest cancels a running backtest job
// @Summary Cancel a running backtest job
// @Tags Backtest
// @Param id path string true "Backtest Job ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/backtest/{id}/cancel [post]
func (h *BacktestHandler) CancelBacktest(c *gin.Context) {
	idStr := c.Param("id")

	// Parse UUID
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid job ID format",
			"details": "Expected UUID format",
		})
		return
	}

	ctx := c.Request.Context()
	job, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Backtest job not found",
			"job_id":  idStr,
			"details": err.Error(),
		})
		return
	}

	// Check if job can be cancelled
	if job.Status != backtest.JobStatusPending && job.Status != backtest.JobStatusRunning {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Cannot cancel backtest job",
			"details": "Job is not in pending or running state",
			"status":  job.Status,
		})
		return
	}

	// Update status to cancelled
	if err := h.jobManager.UpdateJobStatus(ctx, jobID, backtest.JobStatusCancelled, "Cancelled by user"); err != nil {
		log.Error().Err(err).Str("job_id", idStr).Msg("Failed to cancel backtest job")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to cancel backtest job",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backtest job cancelled successfully",
		"job_id":  idStr,
		"status":  backtest.JobStatusCancelled,
	})
}

// RegisterRoutes registers all backtest-related routes
func (h *BacktestHandler) RegisterRoutes(router *gin.RouterGroup) {
	backtest := router.Group("/backtest")
	{
		backtest.POST("/run", h.RunBacktest)
		backtest.GET("", h.ListBacktests)
		backtest.GET("/:id", h.GetBacktest)
		backtest.DELETE("/:id", h.DeleteBacktest)
		backtest.POST("/:id/cancel", h.CancelBacktest)
	}
}

// RegisterRoutesWithRateLimiter registers backtest routes with rate limiting
func (h *BacktestHandler) RegisterRoutesWithRateLimiter(router *gin.RouterGroup, readMiddleware, writeMiddleware gin.HandlerFunc) {
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

	backtest := router.Group("/backtest")
	{
		// Read operations
		backtest.GET("", applyRead(h.ListBacktests)...)
		backtest.GET("/:id", applyRead(h.GetBacktest)...)

		// Write operations (creating/cancelling backtests can be expensive)
		backtest.POST("/run", applyWrite(h.RunBacktest)...)
		backtest.DELETE("/:id", applyWrite(h.DeleteBacktest)...)
		backtest.POST("/:id/cancel", applyWrite(h.CancelBacktest)...)
	}
}

// ExecuteBacktestJob executes a backtest job (this would typically run in a worker)
// This is a placeholder implementation for demonstration
func ExecuteBacktestJob(ctx context.Context, job *backtest.BacktestJob, jobManager *backtest.JobManager) error {
	// Update status to running
	if err := jobManager.UpdateJobStatus(ctx, job.ID, backtest.JobStatusRunning, ""); err != nil {
		return err
	}

	// TODO: Implement actual backtest execution
	// This would involve:
	// 1. Loading historical data for the symbols
	// 2. Creating a backtest engine
	// 3. Running the strategy
	// 4. Collecting results
	// 5. Saving results to database

	// For now, just return success (job will remain in "running" state)
	log.Warn().Str("job_id", job.ID.String()).Msg("Backtest execution not yet implemented - job will remain in running state")

	return nil
}
