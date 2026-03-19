package api

import (
	"context"
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
func (h *PerformanceHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc) {
	g := rg.Group("/performance")
	g.Use(readMiddleware)
	g.GET("/pairs", h.GetPairPerformance)
}

// GetPairPerformance returns realized PnL aggregated by trading pair across all active sessions.
// GET /api/v1/performance/pairs
func (h *PerformanceHandler) GetPairPerformance(c *gin.Context) {
	ctx := c.Request.Context()

	pairs, err := h.aggregatePairPerformance(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to aggregate pair performance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pairs": pairs, "count": len(pairs)})
}

type pairPerf struct {
	Symbol      string  `json:"symbol"`
	RealizedPnL float64 `json:"realized_pnl"`
	TradeCount  int     `json:"trade_count"`
}

func (h *PerformanceHandler) aggregatePairPerformance(ctx context.Context) ([]pairPerf, error) {
	// Fetch all closed positions in one query to avoid N+1 per session.
	positions, err := h.db.GetAllClosedPositions(ctx)
	if err != nil {
		return nil, err
	}

	type agg struct {
		pnl   float64
		count int
	}
	bySymbol := make(map[string]*agg)

	for _, p := range positions {
		if p.RealizedPnL != nil {
			if _, ok := bySymbol[p.Symbol]; !ok {
				bySymbol[p.Symbol] = &agg{}
			}
			bySymbol[p.Symbol].pnl += *p.RealizedPnL
			bySymbol[p.Symbol].count++
		}
	}

	result := make([]pairPerf, 0, len(bySymbol))
	for sym, a := range bySymbol {
		result = append(result, pairPerf{
			Symbol:      sym,
			RealizedPnL: math.Round(a.pnl*100) / 100,
			TradeCount:  a.count,
		})
	}
	return result, nil
}
