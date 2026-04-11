// Package api provides HTTP handlers for the CryptoFunk trading system.
package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// =============================================================================
// Dashboard Types
// =============================================================================

// Component health status strings used across the dashboard system status.
const (
	systemStatusHealthy  = "healthy"
	systemStatusDegraded = "degraded"
	componentHealthy     = "healthy"
	componentUnhealthy   = "unhealthy"
	componentUnavailable = "unavailable"
)

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
	GetClosedFeesBySessionIDs(ctx context.Context, sessionIDs []uuid.UUID) (float64, error)
	GetPositionsBySession(ctx context.Context, sessionID uuid.UUID) ([]*db.Position, error)

	// Health checks
	Ping(ctx context.Context) error

	// Trading state
	IsTradingPaused(ctx context.Context) (bool, error)

	// Agent status
	GetAllAgentStatuses(ctx context.Context) ([]*db.AgentStatus, error)
}

// OrchestratorInterface defines methods for orchestrator control. Implementations
// that cross a process boundary should also implement contextualOrchestrator so
// handlers can propagate request cancellation to outbound calls.
type OrchestratorInterface interface {
	Pause() error
	Resume() error
	IsPaused() bool
	GetActiveAgentCount() int
}

// contextualOrchestrator is an optional extension implemented by orchestrator
// clients that make cross-process calls. When the handler's orchestrator
// satisfies this interface, handlers call the Ctx variants with
// c.Request.Context() so HTTP client disconnects abort the outbound RPC and
// don't pin goroutines for the full 5s timeout. In-process implementations
// (e.g. the real *orchestrator.Orchestrator in tests) only need to satisfy
// OrchestratorInterface.
type contextualOrchestrator interface {
	PauseCtx(ctx context.Context) error
	ResumeCtx(ctx context.Context) error
	IsPausedCtx(ctx context.Context) bool
	GetActiveAgentCountCtx(ctx context.Context) int
}

// pauseOrchestrator calls PauseCtx if the orchestrator supports it, falling
// back to the non-context method otherwise.
func pauseOrchestrator(ctx context.Context, orch OrchestratorInterface) error {
	if ctxOrch, ok := orch.(contextualOrchestrator); ok {
		return ctxOrch.PauseCtx(ctx)
	}
	return orch.Pause()
}

// resumeOrchestrator calls ResumeCtx if the orchestrator supports it, falling
// back to the non-context method otherwise.
func resumeOrchestrator(ctx context.Context, orch OrchestratorInterface) error {
	if ctxOrch, ok := orch.(contextualOrchestrator); ok {
		return ctxOrch.ResumeCtx(ctx)
	}
	return orch.Resume()
}

// isPausedOrchestrator queries pause state using the request context when the
// orchestrator supports it.
func isPausedOrchestrator(ctx context.Context, orch OrchestratorInterface) bool {
	if ctxOrch, ok := orch.(contextualOrchestrator); ok {
		return ctxOrch.IsPausedCtx(ctx)
	}
	return orch.IsPaused()
}

// getActiveAgentCountOrchestrator reads the active-agent count using the
// request context when the orchestrator supports it.
func getActiveAgentCountOrchestrator(ctx context.Context, orch OrchestratorInterface) int {
	if ctxOrch, ok := orch.(contextualOrchestrator); ok {
		return ctxOrch.GetActiveAgentCountCtx(ctx)
	}
	return orch.GetActiveAgentCount()
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

// dashboardBundle is the pre-fetched state a single dashboard request
// needs. Populated in parallel by fetchDashboardBundle so the four
// summary views are built from shared data instead of re-querying.
type dashboardBundle struct {
	sessions    []*db.TradingSession
	positions   []*db.Position
	pingErr     error
	isPaused    bool
	isPausedErr error // non-nil only when falling back to repo.IsTradingPaused
}

// fetchDashboardBundle runs the independent queries GetDashboard needs in
// parallel: active sessions, all open positions, a DB ping, and the
// orchestrator's pause state. Returns the first hard error encountered by
// any of the essential queries (sessions/positions); pingErr and
// isPausedErr are surfaced via the bundle for non-fatal downstream use.
//
// PERF-001 (#134): replaces the previous pattern where getTradingStatus,
// getPositionSummary, and getPnLSummary each re-queried ListActiveSessions
// and GetAllOpenPositions serially, totalling 6-8 serial round-trips per
// dashboard request. This reduces the critical path to one parallel batch
// of 3-4 queries followed by a single serial GetClosedFeesBySessionIDs.
func (h *DashboardHandler) fetchDashboardBundle(ctx context.Context) (*dashboardBundle, error) {
	var (
		bundle      dashboardBundle
		sessionsErr error
		posErr      error
		wg          sync.WaitGroup
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		s, err := h.repo.ListActiveSessions(ctx)
		if err != nil {
			sessionsErr = err
			return
		}
		bundle.sessions = s
	}()

	go func() {
		defer wg.Done()
		p, err := h.repo.GetAllOpenPositions(ctx)
		if err != nil {
			posErr = err
			return
		}
		bundle.positions = p
	}()

	go func() {
		defer wg.Done()
		bundle.pingErr = h.repo.Ping(ctx)
	}()

	// Pause state — orchestrator RPC if available, DB fallback otherwise.
	// Run in a fourth goroutine only when orchestrator is configured, since
	// the DB fallback would duplicate IsTradingPaused which we can resolve
	// lazily in the main goroutine if needed.
	if h.orchestrator != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bundle.isPaused = isPausedOrchestrator(ctx, h.orchestrator)
		}()
	}

	wg.Wait()

	// Essential query errors abort the whole dashboard.
	if sessionsErr != nil {
		return nil, fmt.Errorf("list active sessions: %w", sessionsErr)
	}
	if posErr != nil {
		return nil, fmt.Errorf("get open positions: %w", posErr)
	}

	// DB fallback for pause state only if orchestrator wasn't wired.
	if h.orchestrator == nil {
		paused, err := h.repo.IsTradingPaused(ctx)
		if err != nil {
			bundle.isPausedErr = err
		} else {
			bundle.isPaused = paused
		}
	}

	return &bundle, nil
}

// GetDashboard returns the main dashboard data
// @Summary Get dashboard data
// @Tags Dashboard
// @Produce json
// @Success 200 {object} DashboardData
// @Failure 500 {object} map[string]string
// @Router /api/v1/dashboard [get]
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	ctx := c.Request.Context()

	bundle, err := h.fetchDashboardBundle(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch dashboard bundle")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load dashboard",
		})
		return
	}

	tradingStatus := h.buildTradingStatus(bundle)
	positionSummary := h.calculatePositionSummary(bundle.positions)

	pnlSummary, err := h.buildPnLSummary(ctx, bundle)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build P&L summary for dashboard")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get P&L summary",
		})
		return
	}

	systemStatus := h.buildSystemStatus(ctx, bundle)

	dashboard := DashboardData{
		TradingStatus:   tradingStatus,
		PositionSummary: positionSummary,
		PnLSummary:      pnlSummary,
		SystemStatus:    systemStatus,
		Timestamp:       time.Now(),
	}

	c.JSON(http.StatusOK, dashboard)
}

// buildTradingStatus is a pure-in-memory variant of getTradingStatus that
// builds the TradingStatusInfo view from a pre-fetched dashboardBundle.
// No DB or RPC calls — those happened in fetchDashboardBundle.
func (h *DashboardHandler) buildTradingStatus(bundle *dashboardBundle) TradingStatusInfo {
	status := TradingStatusInfo{
		Mode: "PAPER", // Default
	}
	status.ActiveSessions = len(bundle.sessions)
	status.IsActive = len(bundle.sessions) > 0
	if len(bundle.sessions) > 0 {
		latestSession := bundle.sessions[0]
		status.CurrentSession = &latestSession.ID
		status.Mode = string(latestSession.Mode)
	}
	status.IsPaused = bundle.isPaused
	return status
}

// buildPnLSummary builds the P&L view from the pre-fetched bundle. Issues
// exactly one additional DB query (GetClosedFeesBySessionIDs) instead of
// re-querying sessions and open positions.
func (h *DashboardHandler) buildPnLSummary(ctx context.Context, bundle *dashboardBundle) (PnLSummaryInfo, error) {
	pnl := PnLSummaryInfo{}

	sessionIDs := make([]uuid.UUID, 0, len(bundle.sessions))
	for _, session := range bundle.sessions {
		sessionIDs = append(sessionIDs, session.ID)
		pnl.TotalPnL += session.TotalPnL
		pnl.TotalTrades += session.TotalTrades
		pnl.WinningTrades += session.WinningTrades
		pnl.LosingTrades += session.LosingTrades
		pnl.InitialCapital += session.InitialCapital

		if session.MaxDrawdown > pnl.MaxDrawdown {
			pnl.MaxDrawdown = session.MaxDrawdown
		}
	}

	for _, p := range bundle.positions {
		if p.UnrealizedPnL != nil {
			pnl.UnrealizedPnL += *p.UnrealizedPnL
		}
		// RealizedPnL from open positions is intentionally omitted — see the
		// note in getPnLSummary for rationale.
		pnl.TotalFees += p.Fees
	}

	closedFees, err := h.repo.GetClosedFeesBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return pnl, err
	}
	pnl.TotalFees += closedFees

	if pnl.TotalTrades > 0 {
		pnl.WinRate = float64(pnl.WinningTrades) / float64(pnl.TotalTrades) * 100
	}
	pnl.RealizedPnL = pnl.TotalPnL

	pnl.CurrentCapital = pnl.InitialCapital + pnl.TotalPnL + pnl.UnrealizedPnL
	if pnl.InitialCapital > 0 {
		returnPercent := ((pnl.CurrentCapital - pnl.InitialCapital) / pnl.InitialCapital) * 100
		pnl.ReturnPercent = &returnPercent
	}

	return pnl, nil
}

// buildSystemStatus builds the SystemStatusInfo view from the pre-fetched
// bundle (which already ran the DB ping in parallel). Only issues an
// additional GetAllAgentStatuses query when the orchestrator is unreachable.
func (h *DashboardHandler) buildSystemStatus(ctx context.Context, bundle *dashboardBundle) SystemStatusInfo {
	status := SystemStatusInfo{
		Status:     systemStatusHealthy,
		Uptime:     time.Since(h.startTime).String(),
		Version:    h.version,
		Components: make(map[string]string),
	}

	if bundle.pingErr != nil {
		status.DatabaseOK = false
		status.Status = systemStatusDegraded
		status.Components["database"] = componentUnhealthy
	} else {
		status.DatabaseOK = true
		status.Components["database"] = componentHealthy
	}

	if h.orchestrator != nil {
		count := getActiveAgentCountOrchestrator(ctx, h.orchestrator)
		if count >= 0 {
			status.ActiveAgents = count
			status.Components["orchestrator"] = componentHealthy
		} else {
			status.Components["orchestrator"] = componentUnavailable
			agents, err := h.repo.GetAllAgentStatuses(ctx)
			if err == nil {
				status.ActiveAgents = len(agents)
				status.AgentSummary = make(map[string]int)
				for _, agent := range agents {
					status.AgentSummary[agent.Status]++
				}
			}
		}
	} else {
		agents, err := h.repo.GetAllAgentStatuses(ctx)
		if err == nil {
			status.ActiveAgents = len(agents)
			status.AgentSummary = make(map[string]int)
			for _, agent := range agents {
				status.AgentSummary[agent.Status]++
			}
		}
	}

	return status
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

	if err := pauseOrchestrator(c.Request.Context(), h.orchestrator); err != nil {
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

	if err := resumeOrchestrator(c.Request.Context(), h.orchestrator); err != nil {
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

	sessionIDs := make([]uuid.UUID, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
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
		// NOTE: RealizedPnL from open positions is intentionally omitted here.
		// After a position closes (SELL), it no longer appears in GetAllOpenPositions,
		// so summing RealizedPnL from open positions would always be 0 once trades
		// complete. TotalPnL from session records (already summed above) is the
		// authoritative source for realized P&L.
		pnl.TotalFees += p.Fees
	}

	// Also sum fees from closed positions in the active sessions so TotalFees reflects the full
	// round-trip cost without leaking fees from historical inactive sessions whose P&L is not counted.
	// Uses a SQL aggregate to avoid loading all rows and the 90-day truncation issue.
	//
	// Race note: a position may transition open→closed between the GetAllOpenPositions call above
	// and this GetClosedFeesBySessionIDs call. If that happens, the position's fees could appear in
	// both the open-position loop (pnl.TotalFees += p.Fees) and the closed-fees query, causing a
	// one-position double-count. The window is tiny (milliseconds) and the impact is limited to a
	// single position's fees, so this is accepted as a known approximation in the aggregate view.
	closedFees, err := h.repo.GetClosedFeesBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return pnl, err
	}
	pnl.TotalFees += closedFees

	// Calculate win rate
	if pnl.TotalTrades > 0 {
		pnl.WinRate = float64(pnl.WinningTrades) / float64(pnl.TotalTrades) * 100
	}

	// RealizedPnL mirrors TotalPnL from session stats (which IS realized PnL).
	// Do NOT sum both fields — they are the same value.
	// TODO: deprecate RealizedPnL when all clients migrate to TotalPnL.
	pnl.RealizedPnL = pnl.TotalPnL

	// Derive CurrentCapital from TotalPnL (session-level realized P&L) plus any
	// open-position unrealized P&L. Using RealizedPnL summed from open positions
	// would return 0 after all positions close because closed positions are no
	// longer returned by GetAllOpenPositions.
	pnl.CurrentCapital = pnl.InitialCapital + pnl.TotalPnL + pnl.UnrealizedPnL
	if pnl.InitialCapital > 0 {
		// ReturnPercent is a mark-to-market total return: it includes both realised P&L
		// (TotalPnL from closed session stats) and any unrealised P&L from open positions.
		// This differs from realised-only return = TotalPnL / InitialCapital.
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
		Status:     systemStatusHealthy,
		Uptime:     time.Since(h.startTime).String(),
		Version:    h.version,
		Components: make(map[string]string),
	}

	// Check database
	if err := h.repo.Ping(ctx); err != nil {
		status.DatabaseOK = false
		status.Status = systemStatusDegraded
		status.Components["database"] = componentUnhealthy
	} else {
		status.DatabaseOK = true
		status.Components["database"] = componentHealthy
	}

	// Get agent count from orchestrator (or fall back to database)
	if h.orchestrator != nil {
		count := getActiveAgentCountOrchestrator(ctx, h.orchestrator)
		if count >= 0 {
			status.ActiveAgents = count
			status.Components["orchestrator"] = componentHealthy
		} else {
			// Orchestrator unreachable (-1 sentinel), fall back to DB
			status.Components["orchestrator"] = componentUnavailable
			agents, err := h.repo.GetAllAgentStatuses(ctx)
			if err == nil {
				status.ActiveAgents = len(agents)
				status.AgentSummary = make(map[string]int)
				for _, agent := range agents {
					status.AgentSummary[agent.Status]++
				}
			}
		}
	} else {
		// No orchestrator client configured, use database
		agents, err := h.repo.GetAllAgentStatuses(ctx)
		if err == nil {
			status.ActiveAgents = len(agents)
			status.AgentSummary = make(map[string]int)
			for _, agent := range agents {
				status.AgentSummary[agent.Status]++
			}
		}
		status.Components["orchestrator"] = componentUnavailable
	}

	// Set overall status based on components
	if !status.DatabaseOK {
		status.Status = componentUnhealthy
	} else if status.ActiveAgents == 0 {
		status.Status = systemStatusDegraded
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
