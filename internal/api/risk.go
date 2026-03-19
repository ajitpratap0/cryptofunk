package api

import (
	"context"
	"math"
	"net/http"

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
// If cfg is nil a zero-value RiskConfig is used, which causes setDefaults values to be
// applied at load time and therefore cfg should always be non-nil in production.
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
	// NOTE: exposure is calculated at cost-basis (entry_price), not mark-to-market.
	// Current market price is not stored on the position; a live price lookup
	// would be needed for accurate mark-to-market exposure.
	var totalExposure float64
	for _, p := range openPositions {
		totalExposure += p.Quantity * p.EntryPrice
	}

	returns, err := h.collectClosedReturns(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to collect returns"})
		return
	}

	dataPoints := len(returns)
	response := gin.H{
		"open_positions":     openCount,
		"total_exposure":     math.Round(totalExposure*100) / 100,
		"data_points":        dataPoints,
		"var_95":             nil,
		"var_99":             nil,
		"expected_shortfall": nil,
	}
	if dataPoints < 10 {
		log.Debug().Int("data_points", dataPoints).Msg("insufficient data for meaningful VaR estimate; need at least 10 closed positions in the last 90 days")
	}

	if dataPoints >= 10 {
		// CalculateVaR requires []interface{} not []float64
		returnsIface := make([]interface{}, dataPoints)
		for i, v := range returns {
			returnsIface[i] = v
		}

		// Sum InitialCapital across all active sessions to get total portfolio value.
		// Used to convert fractional VaR (e.g. 0.023) into dollar VaR (e.g. $2,300).
		// NOTE: scaling by InitialCapital (not CurrentCapital) because TradingSession
		// does not yet track current portfolio value. Dollar VaR will be inaccurate
		// after significant P&L. Track in TASKS.md follow-up.
		activeSessions, sessErr := h.db.ListActiveSessions(ctx)
		portfolioValue := 0.0
		if sessErr == nil {
			for _, s := range activeSessions {
				portfolioValue += s.InitialCapital
			}
		} else {
			log.Warn().Err(sessErr).Msg("ListActiveSessions failed; skipping VaR calculation")
		}

		// Skip VaR entirely when there is no portfolio to compute it for.
		// var_95, var_99, and expected_shortfall remain nil in the response.
		if portfolioValue > 0 {
			res95, err := h.riskService.CalculateVaR(map[string]interface{}{
				"returns":          returnsIface,
				"confidence_level": 0.95,
			})
			if err != nil {
				log.Debug().Err(err).Msg("VaR calculation failed (95%)")
			} else {
				if varResult, ok := res95.(*risk.VaRResult); ok {
					response["var_95"] = varResult.VaR * portfolioValue
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
					response["var_99"] = varResult.VaR * portfolioValue
					response["expected_shortfall"] = varResult.CVaR * portfolioValue
				}
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

	// Session Total Loss: triggered when cumulative session losses exceed the configured threshold (dollars).
	// NOTE: totalPnL is the sum of TotalPnL across all active sessions (lifetime session PnL),
	// not a rolling daily figure. The threshold field MaxDailyLossDollars still applies as the
	// absolute dollar loss limit; only the label has been corrected to avoid confusion.
	lossAmount := math.Abs(math.Min(totalPnL, 0))
	breakers := []gin.H{
		buildBreaker("Session Total Loss", lossAmount, h.cfg.MaxDailyLossDollars),
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

	// NOTE: exposure is calculated at cost-basis (entry_price), not mark-to-market.
	// Current market price is not stored on the position; a live price lookup
	// would be needed for accurate mark-to-market exposure.
	rows, err := h.db.GetExposureBySymbol(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query exposure by symbol"})
		return
	}

	type symbolExposureJSON struct {
		Symbol   string  `json:"symbol"`
		Exposure float64 `json:"exposure"`
	}
	result := make([]symbolExposureJSON, 0, len(rows))
	for _, r := range rows {
		result = append(result, symbolExposureJSON{
			Symbol:   r.Symbol,
			Exposure: math.Round(r.Exposure*100) / 100,
		})
	}

	c.JSON(http.StatusOK, gin.H{"exposure": result, "count": len(result)})
}

// collectClosedReturns gathers fractional returns from all closed positions.
// Each return is RealizedPnL / (EntryPrice * Quantity) so values are dimensionless
// fractions (e.g. 0.023 = 2.3%) suitable for VaR calculations.
func (h *RiskHandler) collectClosedReturns(ctx context.Context) ([]float64, error) {
	positions, err := h.db.GetAllClosedPositions(ctx)
	if err != nil {
		return nil, err
	}

	var returns []float64
	for _, p := range positions {
		// RealizedPnL is non-nil here: GetAllClosedPositions filters realized_pnl IS NOT NULL.
		// This guard is retained as defense-in-depth against future query changes.
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
