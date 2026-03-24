package api

import (
	"context"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// PerformanceRepository is the minimal database interface required by PerformanceHandler.
type PerformanceRepository interface {
	GetPairPerformance(ctx context.Context) ([]db.PairPerformance, error)
	ListActiveSessions(ctx context.Context) ([]*db.TradingSession, error)
}

// Compile-time assertion: *db.DB must satisfy PerformanceRepository.
var _ PerformanceRepository = (*db.DB)(nil)

// PerformanceHandler provides REST endpoints for per-symbol performance metrics.
type PerformanceHandler struct {
	db PerformanceRepository
}

// NewPerformanceHandler creates a new PerformanceHandler backed by the given database.
func NewPerformanceHandler(database *db.DB) *PerformanceHandler {
	return &PerformanceHandler{db: database}
}

// RegisterRoutes mounts the /performance sub-group under the provided router group.
// If authMiddleware is non-nil it is applied to all routes alongside readMiddleware.
func (h *PerformanceHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	g := rg.Group("/performance")
	if authMiddleware != nil {
		g.GET("/pairs", readMiddleware, authMiddleware, h.GetPairPerformance)
		g.GET("/summary", readMiddleware, authMiddleware, h.GetSummary)
	} else {
		g.Use(readMiddleware)
		g.GET("/pairs", h.GetPairPerformance)
		g.GET("/summary", h.GetSummary)
	}
}

// GetPairPerformance returns realized PnL aggregated by trading pair across all active sessions.
// GET /api/v1/performance/pairs
func (h *PerformanceHandler) GetPairPerformance(c *gin.Context) {
	ctx := c.Request.Context()

	rows, err := h.db.GetPairPerformance(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to aggregate pair performance"})
		return
	}

	type pairPerf struct {
		Symbol      string  `json:"symbol"`
		RealizedPnL float64 `json:"realized_pnl"`
		TradeCount  int     `json:"trade_count"`
	}
	pairs := make([]pairPerf, 0, len(rows))
	for _, r := range rows {
		pairs = append(pairs, pairPerf{
			Symbol:      r.Symbol,
			RealizedPnL: math.Round(r.RealizedPnL*100) / 100,
			TradeCount:  r.TradeCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"pairs": pairs, "count": len(pairs)})
}

// GetSummary returns aggregate portfolio performance metrics across all active sessions.
// GET /api/v1/performance/summary
//
// NOTE: This endpoint aggregates across ALL currently active sessions regardless of
// trading mode (PAPER and LIVE sessions are combined). Stopped session P&L is excluded.
// Use the session-level endpoints to filter by mode when paper/live breakdown is needed.
func (h *PerformanceHandler) GetSummary(c *gin.Context) {
	ctx := c.Request.Context()

	sessions, err := h.db.ListActiveSessions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
		return
	}

	var totalPnL, totalInitialCapital float64
	var totalTrades, winningTrades, losingTrades int
	for _, s := range sessions {
		totalPnL += s.TotalPnL
		// Sum InitialCapital across all active sessions: each session represents a
		// distinct capital allocation, so the correct portfolio-level denominator
		// for return_percent is the total capital deployed, not the maximum of any
		// single session.
		totalInitialCapital += s.InitialCapital
		totalTrades += s.TotalTrades
		winningTrades += s.WinningTrades
		// Note: winning_trades + losing_trades may be < total_trades when sessions
		// have open positions that are not yet resolved (neither a win nor a loss).
		// This is expected behavior for the "active sessions only" summary.
		losingTrades += s.LosingTrades
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
	}
	returnPct := 0.0
	if totalInitialCapital > 0 {
		returnPct = totalPnL / totalInitialCapital * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_pnl":      math.Round(totalPnL*100) / 100,
		"total_trades":   totalTrades,
		"winning_trades": winningTrades,
		"losing_trades":  losingTrades,
		"win_rate":       math.Round(winRate*100) / 100,
		"return_percent": math.Round(returnPct*100) / 100,
		"capital_basis":  math.Round(totalInitialCapital*100) / 100, // sum of InitialCapital across all active sessions
		"session_count":  len(sessions),
		"scope":          "active_sessions_only", // only running sessions included; stopped sessions are excluded
	})
}
