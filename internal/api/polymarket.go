package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/paper"
)

const sideYES = "YES"

// PolymarketHandler handles HTTP requests for Polymarket paper trading.
type PolymarketHandler struct {
	db          *db.DB
	gammaClient *gamma.Client
}

// NewPolymarketHandler creates a new Polymarket handler.
func NewPolymarketHandler(database *db.DB) *PolymarketHandler {
	return &PolymarketHandler{
		db:          database,
		gammaClient: gamma.NewClient(),
	}
}

// RegisterRoutesWithRateLimiter registers Polymarket routes with rate limiting.
func (h *PolymarketHandler) RegisterRoutesWithRateLimiter(router *gin.RouterGroup, readMiddleware, writeMiddleware gin.HandlerFunc) {
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

	pm := router.Group("/polymarket")
	{
		pm.GET("/markets", applyRead(h.ListMarkets)...)
		pm.GET("/markets/:id", applyRead(h.GetMarket)...)
		pm.GET("/positions", applyRead(h.ListPositions)...)
		pm.GET("/positions/:id", applyRead(h.GetPosition)...)
		pm.POST("/trade", applyWrite(h.ExecuteTrade)...)
		pm.GET("/portfolio", applyRead(h.GetPortfolio)...)
		pm.GET("/trades", applyRead(h.ListTrades)...)
		pm.GET("/predictions", applyRead(h.ListPredictions)...)
		pm.GET("/performance", applyRead(h.GetPerformance)...)
	}
}

// ListMarkets returns active Polymarket markets from the DB.
func (h *PolymarketHandler) ListMarkets(c *gin.Context) {
	ctx := c.Request.Context()
	activeOnly := c.DefaultQuery("active", queryParamTrue) == queryParamTrue

	markets, err := h.db.ListPolymarketMarkets(ctx, activeOnly)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list polymarket markets")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve markets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"markets": markets,
		"count":   len(markets),
	})
}

// GetMarket returns a single market by ID.
func (h *PolymarketHandler) GetMarket(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	market, err := h.db.GetPolymarketMarket(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found", "id": id})
		return
	}

	c.JSON(http.StatusOK, gin.H{"market": market})
}

// ListPositions returns open paper positions, optionally enriched with live prices.
func (h *PolymarketHandler) ListPositions(c *gin.Context) {
	ctx := c.Request.Context()
	enrichLive := c.DefaultQuery("live", "false") == queryParamTrue

	positions, err := h.db.GetOpenPolymarketPositions(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list polymarket positions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve positions"})
		return
	}

	type enrichedPosition struct {
		*db.PolymarketPosition
		CurrentPrice   *float64 `json:"current_price,omitempty"`
		UnrealizedPnl  *float64 `json:"unrealized_pnl,omitempty"`
		MarketQuestion string   `json:"market_question,omitempty"`
	}

	result := make([]enrichedPosition, 0, len(positions))
	for _, p := range positions {
		ep := enrichedPosition{PolymarketPosition: p}

		// Try to get market question from DB
		if market, err := h.db.GetPolymarketMarket(ctx, p.MarketID); err == nil {
			ep.MarketQuestion = market.Question

			// Use DB prices as baseline
			var currentPrice float64
			if p.Side == sideYES && market.YesPrice != nil {
				currentPrice = *market.YesPrice
			} else if market.NoPrice != nil {
				currentPrice = *market.NoPrice
			}
			ep.CurrentPrice = &currentPrice
			unrealized := p.Shares*currentPrice - p.CostBasis
			ep.UnrealizedPnl = &unrealized
		}

		// Optionally enrich with live Gamma API prices
		if enrichLive && h.gammaClient != nil {
			if liveMarket, err := h.gammaClient.FetchMarketByID(p.MarketID); err == nil {
				var livePrice float64
				if p.Side == sideYES {
					livePrice = liveMarket.OutcomeYesPrice
				} else {
					livePrice = liveMarket.OutcomeNoPrice
				}
				if livePrice > 0 {
					ep.CurrentPrice = &livePrice
					unrealized := p.Shares*livePrice - p.CostBasis
					ep.UnrealizedPnl = &unrealized
				}
			}
		}

		result = append(result, ep)
	}

	c.JSON(http.StatusOK, gin.H{
		"positions": result,
		"count":     len(result),
	})
}

// GetPosition returns a single position by ID.
func (h *PolymarketHandler) GetPosition(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid position ID format"})
		return
	}

	position, err := h.db.GetPolymarketPosition(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found", "id": idStr})
		return
	}

	c.JSON(http.StatusOK, gin.H{"position": position})
}

// ExecuteTrade executes a paper buy or sell.
func (h *PolymarketHandler) ExecuteTrade(c *gin.Context) {
	var req struct {
		Action   string  `json:"action" binding:"required,oneof=buy sell BUY SELL"`
		MarketID string  `json:"market_id" binding:"required"`
		Question string  `json:"question"`
		Side     string  `json:"side" binding:"required,oneof=YES NO"`
		Amount   float64 `json:"amount" binding:"required,gt=0"`
		Price    float64 `json:"price" binding:"required,gt=0,lt=1"`
		// For sell: number of shares to sell
		Shares float64 `json:"shares"`
		// Optional: reuse an existing session
		SessionID string `json:"session_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Create or reuse a paper engine
	var engine *paper.DBPaperEngine
	var err error

	if req.SessionID != "" {
		sessionID, parseErr := uuid.Parse(req.SessionID)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session_id format"})
			return
		}
		engine, err = paper.NewDBPaperEngineWithSession(h.db, sessionID)
	} else {
		engine, err = paper.NewDBPaperEngine(h.db)
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to create paper engine")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize paper trading engine"})
		return
	}

	action := req.Action
	if action == "buy" {
		action = "BUY"
	}
	if action == "sell" {
		action = "SELL"
	}

	var trade *paper.Trade
	if action == "BUY" {
		trade, err = engine.Buy(req.MarketID, req.Question, paper.Side(req.Side), req.Amount, req.Price)
	} else {
		shares := req.Shares
		if shares <= 0 {
			shares = req.Amount / req.Price
		}
		trade, err = engine.Sell(req.MarketID, paper.Side(req.Side), shares, req.Price)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trade":      trade,
		"balance":    engine.GetBalance(),
		"session_id": engine.SessionID().String(),
		"message":    "Trade executed successfully",
	})
}

// GetPortfolio returns the portfolio summary.
// Accepts an optional ?session_id=<uuid> query parameter to scope the summary
// to a single paper trading session. Without it, the global aggregate is returned.
func (h *PolymarketHandler) GetPortfolio(c *gin.Context) {
	ctx := c.Request.Context()

	sessionIDStr := c.Query("session_id")

	var summary *db.PolymarketPortfolioSummary
	var err error

	if sessionIDStr != "" {
		sessionID, parseErr := uuid.Parse(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session_id format"})
			return
		}
		summary, err = h.db.GetPolymarketPortfolioSummaryBySession(ctx, sessionID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Failed to get session portfolio summary")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve portfolio"})
			return
		}
		// GetPolymarketPortfolioSummaryBySession does not populate OpenPositions; fetch them separately.
		summary.OpenPositions, err = h.db.GetOpenPolymarketPositionsBySession(ctx, sessionID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sessionIDStr).Msg("Failed to get session positions")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve portfolio positions"})
			return
		}
	} else {
		summary, err = h.db.GetPolymarketPortfolioSummary(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get portfolio summary")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve portfolio"})
			return
		}
	}

	// Calculate total value with current prices
	totalValue := 0.0
	for _, p := range summary.OpenPositions {
		if market, err := h.db.GetPolymarketMarket(ctx, p.MarketID); err == nil {
			var price float64
			if p.Side == sideYES && market.YesPrice != nil {
				price = *market.YesPrice
			} else if market.NoPrice != nil {
				price = *market.NoPrice
			}
			totalValue += p.Shares * price
		}
	}

	// Balance = initial capital - cost basis
	balance := paper.DefaultBalance - summary.TotalCostBasis
	unrealizedPnl := totalValue - summary.TotalCostBasis

	c.JSON(http.StatusOK, gin.H{
		"balance":        balance,
		"total_value":    totalValue,
		"cost_basis":     summary.TotalCostBasis,
		"unrealized_pnl": unrealizedPnl,
		"position_count": summary.PositionCount,
	})
}

// ListTrades returns trade history.
func (h *PolymarketHandler) ListTrades(c *gin.Context) {
	ctx := c.Request.Context()
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	trades, err := h.db.GetPolymarketTrades(ctx, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list polymarket trades")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve trades"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trades": trades,
		"count":  len(trades),
	})
}

// ListPredictions returns prediction history.
func (h *PolymarketHandler) ListPredictions(c *gin.Context) {
	ctx := c.Request.Context()
	resolvedOnly := c.DefaultQuery("resolved", "false") == queryParamTrue

	predictions, err := h.db.GetPolymarketPredictions(ctx, resolvedOnly)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list polymarket predictions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve predictions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"predictions": predictions,
		"count":       len(predictions),
	})
}

// GetPerformance returns aggregated performance stats.
func (h *PolymarketHandler) GetPerformance(c *gin.Context) {
	ctx := c.Request.Context()

	predictions, err := h.db.GetPolymarketPredictions(ctx, false)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get predictions for performance")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve performance data"})
		return
	}

	// Calculate stats
	var (
		totalPredictions int
		resolvedCount    int
		correctCount     int
		totalPnl         float64
		brierSum         float64
		categoryStats    = make(map[string]struct {
			Total    int     `json:"total"`
			Correct  int     `json:"correct"`
			WinRate  float64 `json:"win_rate"`
			TotalPnl float64 `json:"total_pnl"`
		})
	)

	for _, p := range predictions {
		totalPredictions++
		cat := "unknown"
		if p.Category != nil {
			cat = *p.Category
		}

		stats := categoryStats[cat]
		stats.Total++

		if p.Resolved {
			resolvedCount++
			if p.Outcome != nil {
				// Brier score: (predicted - actual)^2
				diff := p.PredictedProb - *p.Outcome
				brierSum += diff * diff

				// Count correct: predicted > 0.5 and outcome == 1, or predicted < 0.5 and outcome == 0
				predicted := p.PredictedProb > 0.5
				actual := *p.Outcome > 0.5
				if predicted == actual {
					correctCount++
					stats.Correct++
				}
			}
			if p.Pnl != nil {
				totalPnl += *p.Pnl
				stats.TotalPnl += *p.Pnl
			}
		}

		if stats.Total > 0 {
			stats.WinRate = float64(stats.Correct) / float64(stats.Total) * 100
		}
		categoryStats[cat] = stats
	}

	winRate := 0.0
	brierScore := 0.0
	if resolvedCount > 0 {
		winRate = float64(correctCount) / float64(resolvedCount) * 100
		brierScore = brierSum / float64(resolvedCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"total_predictions": totalPredictions,
		"resolved":          resolvedCount,
		"correct":           correctCount,
		"win_rate":          winRate,
		"brier_score":       brierScore,
		"total_pnl":         totalPnl,
		"by_category":       categoryStats,
		"timestamp":         time.Now(),
	})
}
