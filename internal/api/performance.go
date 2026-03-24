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
// NOTE: This endpoint only aggregates across currently active (not stopped) sessions.
// Stopped session P&L is excluded from all reported metrics.
func (h *PerformanceHandler) GetSummary(c *gin.Context) {
	ctx := c.Request.Context()

	sessions, err := h.db.ListActiveSessions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
		return
	}

	var totalPnL, maxInitialCapital float64
	var totalTrades, winningTrades, losingTrades int
	for _, s := range sessions {
		totalPnL += s.TotalPnL
		// Use max InitialCapital across sessions rather than summing, because all
		// concurrent sessions draw from the same underlying portfolio. Summing
		// would double-count capital and artificially deflate return_percent.
		if s.InitialCapital > maxInitialCapital {
			maxInitialCapital = s.InitialCapital
		}
		totalTrades += s.TotalTrades
		winningTrades += s.WinningTrades
		losingTrades += s.LosingTrades
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
	}
	returnPct := 0.0
	if maxInitialCapital > 0 {
		returnPct = totalPnL / maxInitialCapital * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_pnl":      math.Round(totalPnL*100) / 100,
		"total_trades":   totalTrades,
		"winning_trades": winningTrades,
		"losing_trades":  losingTrades,
		"win_rate":       math.Round(winRate*100) / 100,
		"return_percent": math.Round(returnPct*100) / 100,
		"session_count":  len(sessions),
		"scope":          "active_sessions_only", // only running sessions included; stopped sessions are excluded
	})
}
