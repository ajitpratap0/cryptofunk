// Package api provides HTTP handlers for the CryptoFunk trading system.
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// =============================================================================
// Dashboard Types
// =============================================================================

// DashboardData represents the main dashboard response with all key metrics
type DashboardData struct {
	TradingStatus   TradingStatusInfo   `json:"trading_status"`
	PositionSummary PositionSummaryInfo `json:"position_summary"`
	PnLSummary      PnLSummaryInfo      `json:"pnl_summary"`
	SystemStatus    SystemStatusInfo    `json:"system_status"`
	Timestamp       time.Time           `json:"timestamp"`
}

// TradingStatusInfo contains information about current trading state
type TradingStatusInfo struct {
	IsActive       bool       `json:"is_active"`
	IsPaused       bool       `json:"is_paused"`
	Mode           string     `json:"mode"` // "PAPER" or "LIVE"
	ActiveSessions int        `json:"active_sessions"`
	CurrentSession *uuid.UUID `json:"current_session,omitempty"`
}

// PositionSummaryInfo contains summary of all positions
type PositionSummaryInfo struct {
	OpenPositions   int     `json:"open_positions"`
	TotalExposure   float64 `json:"total_exposure"`
	LongPositions   int     `json:"long_positions"`
	ShortPositions  int     `json:"short_positions"`
	TotalUnrealized float64 `json:"total_unrealized_pnl"`
}

// PnLSummaryInfo contains profit and loss summary
type PnLSummaryInfo struct {
	TotalPnL       float64  `json:"total_pnl"`
	RealizedPnL    float64  `json:"realized_pnl"`
	UnrealizedPnL  float64  `json:"unrealized_pnl"`
	TotalTrades    int      `json:"total_trades"`
	WinningTrades  int      `json:"winning_trades"`
	LosingTrades   int      `json:"losing_trades"`
	WinRate        float64  `json:"win_rate"`
	MaxDrawdown    float64  `json:"max_drawdown"`
	TotalFees      float64  `json:"total_fees"`
	InitialCapital float64  `json:"initial_capital"`
	CurrentCapital float64  `json:"current_capital"`
	ReturnPercent  *float64 `json:"return_percent,omitempty"`
}

// SystemStatusInfo contains system health information
type SystemStatusInfo struct {
	Status        string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Uptime        string            `json:"uptime"`
	DatabaseOK    bool              `json:"database_ok"`
	ActiveAgents  int               `json:"active_agents"`
	AgentSummary  map[string]int    `json:"agent_summary,omitempty"`
	Version       string            `json:"version"`
	LastHeartbeat *time.Time        `json:"last_heartbeat,omitempty"`
	Components    map[string]string `json:"components,omitempty"`
}

// PositionDetails contains detailed information about a position
type PositionDetails struct {
	ID            uuid.UUID  `json:"id"`
	SessionID     *uuid.UUID `json:"session_id,omitempty"`
	Symbol        string     `json:"symbol"`
	Exchange      string     `json:"exchange"`
	Side          string     `json:"side"` // "LONG" or "SHORT"
	EntryPrice    float64    `json:"entry_price"`
	CurrentPrice  *float64   `json:"current_price,omitempty"`
	Quantity      float64    `json:"quantity"`
	EntryTime     time.Time  `json:"entry_time"`
	StopLoss      *float64   `json:"stop_loss,omitempty"`
	TakeProfit    *float64   `json:"take_profit,omitempty"`
	UnrealizedPnL *float64   `json:"unrealized_pnl,omitempty"`
	RealizedPnL   *float64   `json:"realized_pnl,omitempty"`
	Fees          float64    `json:"fees"`
	EntryReason   *string    `json:"entry_reason,omitempty"`
	Duration      string     `json:"duration"`
}

// StartTradingRequest is the request body for starting trading
type StartTradingRequest struct {
	Symbol         string  `json:"symbol" binding:"required"`
	InitialCapital float64 `json:"initial_capital" binding:"required,gt=0"`
	Mode           string  `json:"mode" binding:"omitempty,oneof=paper live PAPER LIVE"`
}

// StopTradingRequest is the request body for stopping trading
type StopTradingRequest struct {
	SessionID    string  `json:"session_id" binding:"required"`
	FinalCapital float64 `json:"final_capital" binding:"required,gte=0"`
}

// TradingResponse is the response for trading control operations
type TradingResponse struct {
	Success   bool       `json:"success"`
	Message   string     `json:"message"`
	SessionID *uuid.UUID `json:"session_id,omitempty"`
	Action    string     `json:"action"`
	Timestamp time.Time  `json:"timestamp"`
}

// =============================================================================
// Dashboard Repository Interface
// =============================================================================

// DashboardRepositoryInterface defines methods for dashboard data access
type DashboardRepositoryInterface interface {
	// Session management
	ListActiveSessions(ctx context.Context) ([]*db.TradingSession, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (*db.TradingSession, error)
	CreateSession(ctx context.Context, session *db.TradingSession) error
	StopSession(ctx context.Context, sessionID uuid.UUID, finalCapital float64) error

	// Position management
	GetAllOpenPositions(ctx context.Context) ([]*db.Position, error)
	GetPositionsBySession(ctx context.Context, sessionID uuid.UUID) ([]*db.Position, error)

	// Health checks
	Ping(ctx context.Context) error

	// Trading state
	IsTradingPaused(ctx context.Context) (bool, error)

	// Agent status
	GetAllAgentStatuses(ctx context.Context) ([]*db.AgentStatus, error)
}

// OrchestratorInterface defines methods for orchestrator control
type OrchestratorInterface interface {
	Pause() error
	Resume() error
	IsPaused() bool
	GetActiveAgentCount() int
}

// =============================================================================
// Dashboard Handler
// =============================================================================

// DashboardHandler handles HTTP requests for dashboard functionality
type DashboardHandler struct {
	repo         DashboardRepositoryInterface
	orchestrator OrchestratorInterface
	startTime    time.Time
	version      string
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(repo DashboardRepositoryInterface) *DashboardHandler {
	return &DashboardHandler{
		repo:      repo,
		startTime: time.Now(),
		version:   "1.0.0",
	}
}

// NewDashboardHandlerWithOrchestrator creates a dashboard handler with orchestrator support
func NewDashboardHandlerWithOrchestrator(repo DashboardRepositoryInterface, orch OrchestratorInterface, version string) *DashboardHandler {
	return &DashboardHandler{
		repo:         repo,
		orchestrator: orch,
		startTime:    time.Now(),
		version:      version,
	}
}

// SetOrchestrator sets the orchestrator for trading control
func (h *DashboardHandler) SetOrchestrator(orch OrchestratorInterface) {
	h.orchestrator = orch
}

// RegisterRoutes registers all dashboard-related routes
func (h *DashboardHandler) RegisterRoutes(router *gin.RouterGroup) {
	h.RegisterRoutesWithRateLimiter(router, nil, nil)
}

// RegisterRoutesWithRateLimiter registers dashboard routes with rate limiting
func (h *DashboardHandler) RegisterRoutesWithRateLimiter(router *gin.RouterGroup, readMiddleware, writeMiddleware gin.HandlerFunc) {
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

	dashboard := router.Group("/dashboard")
	{
		// Read-only endpoints
		dashboard.GET("", applyRead(h.GetDashboard)...)
		dashboard.GET("/positions", applyRead(h.GetPositions)...)
		dashboard.GET("/pnl", applyRead(h.GetPnL)...)
		dashboard.GET("/status", applyRead(h.GetStatus)...)

		// Trading control endpoints (write operations)
		trading := dashboard.Group("/trading")
		{
			trading.POST("/start", applyWrite(h.StartTrading)...)
			trading.POST("/stop", applyWrite(h.StopTrading)...)
			trading.POST("/pause", applyWrite(h.PauseTrading)...)
			trading.POST("/resume", applyWrite(h.ResumeTrading)...)
		}
	}
}

// =============================================================================
// Handler Methods
// =============================================================================

// GetDashboard returns the main dashboard data
// @Summary Get dashboard data
// @Tags Dashboard
// @Produce json
// @Success 200 {object} DashboardData
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard [get]
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	ctx := c.Request.Context()

	// Get trading status
	tradingStatus, err := h.getTradingStatus(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get trading status for dashboard")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get trading status",
		})
		return
	}

	// Get position summary
	positionSummary, err := h.getPositionSummary(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get position summary for dashboard")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get position summary",
		})
		return
	}

	// Get P&L summary
	pnlSummary, err := h.getPnLSummary(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get P&L summary for dashboard")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get P&L summary",
		})
		return
	}

	// Get system status
	systemStatus := h.getSystemStatus(ctx)

	dashboard := DashboardData{
		TradingStatus:   tradingStatus,
		PositionSummary: positionSummary,
		PnLSummary:      pnlSummary,
		SystemStatus:    systemStatus,
		Timestamp:       time.Now(),
	}

	c.JSON(http.StatusOK, dashboard)
}

// GetPositions returns all current open positions
// @Summary Get current positions
// @Tags Dashboard
// @Produce json
// @Param session_id query string false "Filter by session ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard/positions [get]
func (h *DashboardHandler) GetPositions(c *gin.Context) {
	ctx := c.Request.Context()

	var positions []*db.Position
	var err error

	// Optional session filter
	sessionIDStr := c.Query("session_id")
	if sessionIDStr != "" {
		sessionID, parseErr := uuid.Parse(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid session_id format",
			})
			return
		}
		positions, err = h.repo.GetPositionsBySession(ctx, sessionID)
	} else {
		positions, err = h.repo.GetAllOpenPositions(ctx)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to get positions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve positions",
		})
		return
	}

	// Convert to detailed positions
	positionDetails := make([]PositionDetails, 0, len(positions))
	for _, p := range positions {
		positionDetails = append(positionDetails, h.toPositionDetails(p))
	}

	// Calculate summary
	summary := h.calculatePositionSummary(positions)

	c.JSON(http.StatusOK, gin.H{
		"positions": positionDetails,
		"count":     len(positionDetails),
		"summary":   summary,
	})
}

// GetPnL returns the P&L summary
// @Summary Get P&L summary
// @Tags Dashboard
// @Produce json
// @Param session_id query string false "Filter by session ID"
// @Success 200 {object} PnLSummaryInfo
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard/pnl [get]
func (h *DashboardHandler) GetPnL(c *gin.Context) {
	ctx := c.Request.Context()

	// Check for session filter
	sessionIDStr := c.Query("session_id")
	if sessionIDStr != "" {
		sessionID, parseErr := uuid.Parse(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid session_id format",
			})
			return
		}

		// Get session-specific P&L
		session, err := h.repo.GetSession(ctx, sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "Session not found",
				"session_id": sessionIDStr,
			})
			return
		}

		// Get positions for this session
		positions, err := h.repo.GetPositionsBySession(ctx, sessionID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Failed to get session positions")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve session positions",
			})
			return
		}

		pnl := h.calculateSessionPnL(session, positions)
		c.JSON(http.StatusOK, pnl)
		return
	}

	// Get aggregate P&L
	pnlSummary, err := h.getPnLSummary(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get P&L summary")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve P&L summary",
		})
		return
	}

	c.JSON(http.StatusOK, pnlSummary)
}

// GetStatus returns the system status
// @Summary Get system status
// @Tags Dashboard
// @Produce json
// @Success 200 {object} SystemStatusInfo
// @Router /api/v1/dashboard/status [get]
func (h *DashboardHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	status := h.getSystemStatus(ctx)
	c.JSON(http.StatusOK, status)
}

// StartTrading starts a new trading session
// @Summary Start trading
// @Tags Dashboard
// @Accept json
// @Produce json
// @Param request body StartTradingRequest true "Start trading request"
// @Success 200 {object} TradingResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard/trading/start [post]
func (h *DashboardHandler) StartTrading(c *gin.Context) {
	var req StartTradingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Default to paper trading
	if req.Mode == "" {
		req.Mode = "PAPER"
	}

	// Create trading session
	session := &db.TradingSession{
		Mode:           db.TradingMode(req.Mode),
		Symbol:         req.Symbol,
		Exchange:       "binance", // Default exchange
		StartedAt:      time.Now(),
		InitialCapital: req.InitialCapital,
	}

	ctx := c.Request.Context()
	if err := h.repo.CreateSession(ctx, session); err != nil {
		log.Error().Err(err).Msg("Failed to create trading session")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start trading session",
		})
		return
	}

	log.Info().
		Str("session_id", session.ID.String()).
		Str("symbol", session.Symbol).
		Str("mode", string(session.Mode)).
		Msg("Trading session started via dashboard")

	c.JSON(http.StatusOK, TradingResponse{
		Success:   true,
		Message:   "Trading session started successfully",
		SessionID: &session.ID,
		Action:    "start",
		Timestamp: time.Now(),
	})
}

// StopTrading stops an active trading session
// @Summary Stop trading
// @Tags Dashboard
// @Accept json
// @Produce json
// @Param request body StopTradingRequest true "Stop trading request"
// @Success 200 {object} TradingResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard/trading/stop [post]
func (h *DashboardHandler) StopTrading(c *gin.Context) {
	var req StopTradingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid session_id format",
		})
		return
	}

	ctx := c.Request.Context()

	// Verify session exists
	_, err = h.repo.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Session not found",
			"session_id": req.SessionID,
		})
		return
	}

	// Stop the session
	if err := h.repo.StopSession(ctx, sessionID, req.FinalCapital); err != nil {
		log.Error().Err(err).Str("session_id", req.SessionID).Msg("Failed to stop trading session")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to stop trading session",
		})
		return
	}

	log.Info().
		Str("session_id", req.SessionID).
		Float64("final_capital", req.FinalCapital).
		Msg("Trading session stopped via dashboard")

	c.JSON(http.StatusOK, TradingResponse{
		Success:   true,
		Message:   "Trading session stopped successfully",
		SessionID: &sessionID,
		Action:    "stop",
		Timestamp: time.Now(),
	})
}

// PauseTrading pauses the trading system
// @Summary Pause trading
// @Tags Dashboard
// @Produce json
// @Success 200 {object} TradingResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard/trading/pause [post]
func (h *DashboardHandler) PauseTrading(c *gin.Context) {
	if h.orchestrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Orchestrator not available",
		})
		return
	}

	if err := h.orchestrator.Pause(); err != nil {
		log.Error().Err(err).Msg("Failed to pause trading")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to pause trading",
			"details": err.Error(),
		})
		return
	}

	log.Info().Msg("Trading paused via dashboard")

	c.JSON(http.StatusOK, TradingResponse{
		Success:   true,
		Message:   "Trading paused successfully",
		Action:    "pause",
		Timestamp: time.Now(),
	})
}

// ResumeTrading resumes the trading system
// @Summary Resume trading
// @Tags Dashboard
// @Produce json
// @Success 200 {object} TradingResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard/trading/resume [post]
func (h *DashboardHandler) ResumeTrading(c *gin.Context) {
	if h.orchestrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Orchestrator not available",
		})
		return
	}

	if err := h.orchestrator.Resume(); err != nil {
		log.Error().Err(err).Msg("Failed to resume trading")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to resume trading",
			"details": err.Error(),
		})
		return
	}

	log.Info().Msg("Trading resumed via dashboard")

	c.JSON(http.StatusOK, TradingResponse{
		Success:   true,
		Message:   "Trading resumed successfully",
		Action:    "resume",
		Timestamp: time.Now(),
	})
}

// =============================================================================
// Helper Methods
// =============================================================================

// getTradingStatus returns the current trading status
func (h *DashboardHandler) getTradingStatus(ctx context.Context) (TradingStatusInfo, error) {
	status := TradingStatusInfo{
		Mode: "PAPER", // Default
	}

	// Get active sessions
	sessions, err := h.repo.ListActiveSessions(ctx)
	if err != nil {
		return status, err
	}

	status.ActiveSessions = len(sessions)
	status.IsActive = len(sessions) > 0

	// Get latest session info
	if len(sessions) > 0 {
		latestSession := sessions[0]
		status.CurrentSession = &latestSession.ID
		status.Mode = string(latestSession.Mode)
	}

	// Check pause state
	if h.orchestrator != nil {
		status.IsPaused = h.orchestrator.IsPaused()
	} else {
		// Fallback to database
		isPaused, err := h.repo.IsTradingPaused(ctx)
		if err == nil {
			status.IsPaused = isPaused
		}
	}

	return status, nil
}

// getPositionSummary returns a summary of all positions
func (h *DashboardHandler) getPositionSummary(ctx context.Context) (PositionSummaryInfo, error) {
	positions, err := h.repo.GetAllOpenPositions(ctx)
	if err != nil {
		return PositionSummaryInfo{}, err
	}

	return h.calculatePositionSummary(positions), nil
}

// calculatePositionSummary calculates summary statistics for positions
func (h *DashboardHandler) calculatePositionSummary(positions []*db.Position) PositionSummaryInfo {
	summary := PositionSummaryInfo{}

	for _, p := range positions {
		summary.OpenPositions++
		summary.TotalExposure += p.EntryPrice * p.Quantity

		switch p.Side {
		case db.PositionSideLong:
			summary.LongPositions++
		case db.PositionSideShort:
			summary.ShortPositions++
		}

		if p.UnrealizedPnL != nil {
			summary.TotalUnrealized += *p.UnrealizedPnL
		}
	}

	return summary
}

// getPnLSummary returns aggregate P&L information
func (h *DashboardHandler) getPnLSummary(ctx context.Context) (PnLSummaryInfo, error) {
	pnl := PnLSummaryInfo{}

	// Get all active sessions to aggregate P&L
	sessions, err := h.repo.ListActiveSessions(ctx)
	if err != nil {
		return pnl, err
	}

	for _, session := range sessions {
		pnl.TotalPnL += session.TotalPnL
		pnl.TotalTrades += session.TotalTrades
		pnl.WinningTrades += session.WinningTrades
		pnl.LosingTrades += session.LosingTrades
		pnl.InitialCapital += session.InitialCapital

		if session.MaxDrawdown > pnl.MaxDrawdown {
			pnl.MaxDrawdown = session.MaxDrawdown
		}
	}

	// Get all open positions for unrealized P&L
	positions, err := h.repo.GetAllOpenPositions(ctx)
	if err != nil {
		return pnl, err
	}

	for _, p := range positions {
		if p.UnrealizedPnL != nil {
			pnl.UnrealizedPnL += *p.UnrealizedPnL
		}
		if p.RealizedPnL != nil {
			pnl.RealizedPnL += *p.RealizedPnL
		}
		pnl.TotalFees += p.Fees
	}

	// Calculate win rate
	if pnl.TotalTrades > 0 {
		pnl.WinRate = float64(pnl.WinningTrades) / float64(pnl.TotalTrades) * 100
	}

	// Calculate current capital and return
	pnl.CurrentCapital = pnl.InitialCapital + pnl.RealizedPnL + pnl.UnrealizedPnL
	if pnl.InitialCapital > 0 {
		returnPercent := ((pnl.CurrentCapital - pnl.InitialCapital) / pnl.InitialCapital) * 100
		pnl.ReturnPercent = &returnPercent
	}

	return pnl, nil
}

// calculateSessionPnL calculates P&L for a specific session
func (h *DashboardHandler) calculateSessionPnL(session *db.TradingSession, positions []*db.Position) PnLSummaryInfo {
	pnl := PnLSummaryInfo{
		TotalPnL:       session.TotalPnL,
		TotalTrades:    session.TotalTrades,
		WinningTrades:  session.WinningTrades,
		LosingTrades:   session.LosingTrades,
		MaxDrawdown:    session.MaxDrawdown,
		InitialCapital: session.InitialCapital,
	}

	// Add position P&L
	for _, p := range positions {
		if p.UnrealizedPnL != nil {
			pnl.UnrealizedPnL += *p.UnrealizedPnL
		}
		if p.RealizedPnL != nil {
			pnl.RealizedPnL += *p.RealizedPnL
		}
		pnl.TotalFees += p.Fees
	}

	// Calculate win rate
	if pnl.TotalTrades > 0 {
		pnl.WinRate = float64(pnl.WinningTrades) / float64(pnl.TotalTrades) * 100
	}

	// Calculate current capital
	if session.FinalCapital != nil {
		pnl.CurrentCapital = *session.FinalCapital
	} else {
		pnl.CurrentCapital = pnl.InitialCapital + pnl.RealizedPnL + pnl.UnrealizedPnL
	}

	if pnl.InitialCapital > 0 {
		returnPercent := ((pnl.CurrentCapital - pnl.InitialCapital) / pnl.InitialCapital) * 100
		pnl.ReturnPercent = &returnPercent
	}

	return pnl
}

// getSystemStatus returns the current system status
func (h *DashboardHandler) getSystemStatus(ctx context.Context) SystemStatusInfo {
	status := SystemStatusInfo{
		Status:     "healthy",
		Uptime:     time.Since(h.startTime).String(),
		Version:    h.version,
		Components: make(map[string]string),
	}

	// Check database
	if err := h.repo.Ping(ctx); err != nil {
		status.DatabaseOK = false
		status.Status = "degraded"
		status.Components["database"] = "unhealthy"
	} else {
		status.DatabaseOK = true
		status.Components["database"] = "healthy"
	}

	// Get agent count
	if h.orchestrator != nil {
		status.ActiveAgents = h.orchestrator.GetActiveAgentCount()
		status.Components["orchestrator"] = "healthy"
	} else {
		// Try to get agent status from database
		agents, err := h.repo.GetAllAgentStatuses(ctx)
		if err == nil {
			status.ActiveAgents = len(agents)
			status.AgentSummary = make(map[string]int)
			for _, agent := range agents {
				status.AgentSummary[agent.Status]++
			}
		}
		status.Components["orchestrator"] = "unavailable"
	}

	// Set overall status based on components
	if !status.DatabaseOK {
		status.Status = "unhealthy"
	} else if status.ActiveAgents == 0 {
		status.Status = "degraded"
	}

	return status
}

// toPositionDetails converts a db.Position to PositionDetails
func (h *DashboardHandler) toPositionDetails(p *db.Position) PositionDetails {
	duration := time.Since(p.EntryTime)

	return PositionDetails{
		ID:            p.ID,
		SessionID:     p.SessionID,
		Symbol:        p.Symbol,
		Exchange:      p.Exchange,
		Side:          string(p.Side),
		EntryPrice:    p.EntryPrice,
		Quantity:      p.Quantity,
		EntryTime:     p.EntryTime,
		StopLoss:      p.StopLoss,
		TakeProfit:    p.TakeProfit,
		UnrealizedPnL: p.UnrealizedPnL,
		RealizedPnL:   p.RealizedPnL,
		Fees:          p.Fees,
		EntryReason:   p.EntryReason,
		Duration:      formatDuration(duration),
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	if d < 24*time.Hour {
		return d.Round(time.Hour).String()
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}
