package api

import (
	"context"
	"math"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/risk"
)

// RiskHandler provides REST endpoints for risk metrics and circuit breaker status.
type RiskHandler struct {
	db          *db.DB
	riskService *risk.Service
	cfg         *config.RiskConfig
}

// NewRiskHandler creates a new RiskHandler backed by the given database and risk config.
// If cfg is nil a zero-value RiskConfig is used.
func NewRiskHandler(database *db.DB, cfg *config.RiskConfig) *RiskHandler {
	if cfg == nil {
		cfg = &config.RiskConfig{}
	}
	return &RiskHandler{
		db:          database,
		riskService: risk.NewService(),
		cfg:         cfg,
	}
}

// RegisterRoutes mounts the /risk sub-group under the provided router group.
// If authMiddleware is non-nil it is applied to all routes alongside readMiddleware.
func (h *RiskHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	r := rg.Group("/risk")
	if authMiddleware != nil {
		r.GET("/metrics", readMiddleware, authMiddleware, h.GetMetrics)
		r.GET("/circuit-breakers", readMiddleware, authMiddleware, h.GetCircuitBreakers)
		r.GET("/exposure", readMiddleware, authMiddleware, h.GetExposure)
	} else {
		r.Use(readMiddleware)
		r.GET("/metrics", h.GetMetrics)
		r.GET("/circuit-breakers", h.GetCircuitBreakers)
		r.GET("/exposure", h.GetExposure)
	}
}

// GetMetrics returns VaR, CVaR, open position count, and total exposure.
// GET /api/v1/risk/metrics
func (h *RiskHandler) GetMetrics(c *gin.Context) {
	ctx := c.Request.Context()

	openPositions, err := h.db.GetAllOpenPositions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query open positions"})
		return
	}

	openCount := len(openPositions)
	var totalExposure float64
	for _, p := range openPositions {
		totalExposure += p.Quantity * p.EntryPrice
	}

	// Compute portfolio value from active sessions' InitialCapital for VaR scaling.
	// Falls back to totalExposure when no sessions exist. VaR is suppressed (nil) when
	// portfolioValue == 0 because multiplying a fractional VaR by zero gives a
	// misleadingly valid 0.0 result instead of an absent/nil.
	sessions, _ := h.db.ListActiveSessions(ctx) // best-effort; VaR still nil on error
	var portfolioValue float64
	for _, s := range sessions {
		portfolioValue += s.InitialCapital
	}
	if portfolioValue == 0 {
		portfolioValue = totalExposure
	}

	returns, err := h.collectClosedReturns(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to collect returns"})
		return
	}

	response := gin.H{
		"open_positions":     openCount,
		"total_exposure":     math.Round(totalExposure*100) / 100,
		"data_points":        len(returns),
		"var_95":             nil,
		"var_99":             nil,
		"expected_shortfall": nil,
	}

	if len(returns) >= 10 && portfolioValue > 0 {
		// CalculateVaR requires []interface{} not []float64
		returnsIface := make([]interface{}, len(returns))
		for i, v := range returns {
			returnsIface[i] = v
		}

		res95, err := h.riskService.CalculateVaR(map[string]interface{}{
			"returns":          returnsIface,
			"confidence_level": 0.95,
		})
		if err != nil {
			log.Debug().Err(err).Msg("VaR calculation failed (95%)")
		} else {
			if varResult, ok := res95.(*risk.VaRResult); ok {
				response["var_95"] = math.Round(varResult.VaR*portfolioValue*100) / 100
			}
		}

		res99, err := h.riskService.CalculateVaR(map[string]interface{}{
			"returns":          returnsIface,
			"confidence_level": 0.99,
		})
		if err != nil {
			log.Debug().Err(err).Msg("VaR calculation failed (99%)")
		} else {
			if varResult, ok := res99.(*risk.VaRResult); ok {
				response["var_99"] = math.Round(varResult.VaR*portfolioValue*100) / 100
				response["expected_shortfall"] = math.Round(varResult.CVaR*portfolioValue*100) / 100
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetCircuitBreakers returns the status of system-level circuit breakers.
// GET /api/v1/risk/circuit-breakers
func (h *RiskHandler) GetCircuitBreakers(c *gin.Context) {
	ctx := c.Request.Context()

	sessions, err := h.db.ListActiveSessions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
		return
	}

	var totalPnL, maxDrawdown float64
	var totalTrades int
	for _, s := range sessions {
		totalPnL += s.TotalPnL
		if s.MaxDrawdown > maxDrawdown {
			maxDrawdown = s.MaxDrawdown
		}
		totalTrades += s.TotalTrades
	}

	// Max Daily Loss: triggered when cumulative losses exceed the configured threshold
	lossAmount := math.Abs(math.Min(totalPnL, 0))
	breakers := []gin.H{
		buildBreaker("Max Daily Loss", lossAmount, h.cfg.MaxDailyLossDollars),
		buildBreaker("Max Drawdown %", maxDrawdown*100, h.cfg.MaxDrawdownPct),
		buildBreaker("Total Trade Count", float64(totalTrades), float64(h.cfg.MaxTradeCount)),
	}

	c.JSON(http.StatusOK, gin.H{"circuit_breakers": breakers, "count": len(breakers)})
}

func buildBreaker(name string, current, threshold float64) gin.H {
	status := "OK"
	if threshold <= 0 {
		// A zero or negative threshold means the breaker is disabled/unconfigured.
		// Guard against false positives: never fire for unset config values.
		status = "DISABLED"
	} else if current >= threshold {
		status = "TRIGGERED"
	} else if current/threshold >= 0.8 {
		status = "WARNING"
	}
	return gin.H{
		"name":      name,
		"current":   math.Round(current*100) / 100,
		"threshold": threshold,
		"status":    status,
	}
}

// GetExposure returns open position exposure grouped by symbol.
// GET /api/v1/risk/exposure
func (h *RiskHandler) GetExposure(c *gin.Context) {
	ctx := c.Request.Context()

	openPositions, err := h.db.GetAllOpenPositions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query open positions"})
		return
	}

	exposureBySymbol := make(map[string]float64)
	for _, p := range openPositions {
		exposureBySymbol[p.Symbol] += p.Quantity * p.EntryPrice
	}

	type symbolExposure struct {
		Symbol   string  `json:"symbol"`
		Exposure float64 `json:"exposure"`
	}
	result := make([]symbolExposure, 0, len(exposureBySymbol))
	for sym, exp := range exposureBySymbol {
		result = append(result, symbolExposure{
			Symbol:   sym,
			Exposure: math.Round(exp*100) / 100,
		})
	}
	// Sort by symbol for deterministic response ordering (Go map iteration is randomised)
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })

	c.JSON(http.StatusOK, gin.H{"exposure": result, "count": len(result)})
}

// collectClosedReturns gathers RealizedPnL from all closed positions in a single query.
func (h *RiskHandler) collectClosedReturns(ctx context.Context) ([]float64, error) {
	positions, err := h.db.GetAllClosedPositions(ctx)
	if err != nil {
		return nil, err
	}

	var returns []float64
	for _, p := range positions {
		if p.RealizedPnL != nil {
			notional := p.EntryPrice * p.Quantity
			if notional == 0 {
				continue
			}
			returns = append(returns, *p.RealizedPnL/notional)
		}
	}
	return returns, nil
}
