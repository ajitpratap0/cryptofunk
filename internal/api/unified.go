package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/models"
)

// UnifiedHandler serves the cross-platform unified portfolio view.
type UnifiedHandler struct {
	db *db.DB
}

// NewUnifiedHandler creates a new UnifiedHandler.
func NewUnifiedHandler(database *db.DB) *UnifiedHandler {
	return &UnifiedHandler{db: database}
}

// RegisterRoutes registers the unified dashboard routes.
func (h *UnifiedHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc) {
	dashboard := rg.Group("/dashboard")
	dashboard.GET("/unified", readMiddleware, h.GetUnifiedPortfolio)
}

// GetUnifiedPortfolio returns a combined portfolio view across Binance and Polymarket.
func (h *UnifiedHandler) GetUnifiedPortfolio(c *gin.Context) {
	ctx := c.Request.Context()
	portfolio := models.NewEmptyPortfolio()

	// 1. Fetch Binance positions
	binancePositions, err := h.db.GetAllOpenPositions(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Binance positions")
		// Non-fatal: continue with Polymarket
	} else {
		for _, p := range binancePositions {
			portfolio.AddPosition(models.BinancePositionToUnified(p))
		}
	}

	// 2. Fetch Polymarket positions
	polyPositions, err := h.db.GetOpenPolymarketPositions(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Polymarket positions")
	} else {
		for _, p := range polyPositions {
			// Try to enrich with market data
			market, _ := h.db.GetPolymarketMarket(ctx, p.MarketID)
			portfolio.AddPosition(models.PolymarketPositionToUnified(p, market))
		}
	}

	// 3. Get trade counts for platform summaries
	polyTrades, err := h.db.GetPolymarketTrades(ctx, 10000)
	if err == nil {
		portfolio.SetPlatformTradeCount(models.PlatformPolymarket, len(polyTrades))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    portfolio,
	})
}
