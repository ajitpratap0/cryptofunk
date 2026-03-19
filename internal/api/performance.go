package api

import (
	"math"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// PerformanceHandler provides REST endpoints for per-symbol performance metrics.
type PerformanceHandler struct {
	db *db.DB
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
	} else {
		g.Use(readMiddleware)
		g.GET("/pairs", h.GetPairPerformance)
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
