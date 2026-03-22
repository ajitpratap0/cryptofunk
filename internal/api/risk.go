package api

import (
	"context"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// GetMetrics returns VaR, CVaR, open position count, total exposure, max_drawdown, and sharpe_ratio.
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

	// Fetch active sessions once; reused by the VaR block and drawdown block below.
	// NOTE: scaling by InitialCapital (not CurrentCapital) because TradingSession
	// does not yet track current portfolio value. Dollar VaR will be inaccurate
	// after significant P&L. Track in TASKS.md follow-up.
	activeSessions, sessErr := h.db.ListActiveSessions(ctx)

	if dataPoints >= 10 {
		// CalculateVaR requires []interface{} not []float64
		returnsIface := make([]interface{}, dataPoints)
		for i, v := range returns {
			returnsIface[i] = v
		}

		// Sum InitialCapital across all active sessions to get total portfolio value.
		// Used to convert fractional VaR (e.g. 0.023) into dollar VaR (e.g. $2,300).
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

	// Compute max_drawdown and sharpe_ratio from equity snapshots.
	// Instead of calling ListEquitySnapshots once per session (N+1 queries), we:
	//   1. Collect all active session IDs into a string slice — O(n) in memory.
	//   2. Issue ONE query to find the session with the most snapshots.
	//   3. Issue ONE query to fetch that session's snapshots.
	// Total: 2 DB round-trips regardless of how many sessions are active.
	var drawdownSnapshots []*db.EquitySnapshot
	if sessErr == nil && len(activeSessions) > 0 {
		sessionIDs := make([]string, 0, len(activeSessions))
		for _, s := range activeSessions {
			sessionIDs = append(sessionIDs, s.ID.String())
		}

		bestID, idErr := h.db.GetSessionIDWithMostSnapshots(ctx, sessionIDs)
		if idErr != nil {
			log.Debug().Err(idErr).Msg("GetSessionIDWithMostSnapshots failed; skipping drawdown/Sharpe")
		} else if bestID != "" {
			// Parse the UUID string back so we can call ListEquitySnapshots.
			bestUUID, parseErr := uuid.Parse(bestID)
			if parseErr != nil {
				log.Debug().Err(parseErr).Str("session_id", bestID).Msg("failed to parse best session UUID")
			} else {
				drawdownSnapshots, _ = h.db.ListEquitySnapshots(ctx, bestUUID, 500)
			}
		}
	}
	response["max_drawdown"] = computeMaxDrawdown(drawdownSnapshots)
	response["sharpe_ratio"] = computeSharpeRatio(drawdownSnapshots) // *float64 or nil

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

// computeMaxDrawdown returns the maximum peak-to-trough decline as a positive fraction
// (e.g. 0.15 means 15% drawdown). Returns 0 when snapshots has fewer than two entries.
func computeMaxDrawdown(snapshots []*db.EquitySnapshot) float64 {
	if len(snapshots) < 2 {
		return 0
	}
	peak := snapshots[0].Equity
	maxDD := 0.0
	for _, s := range snapshots[1:] {
		if s.Equity > peak {
			peak = s.Equity
		}
		if peak > 0 {
			if dd := (peak - s.Equity) / peak; dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

// computeSharpeRatio returns the annualised Sharpe ratio (risk-free rate = 0) computed
// from inter-snapshot returns. Returns nil when there are fewer than two snapshots.
func computeSharpeRatio(snapshots []*db.EquitySnapshot) *float64 {
	if len(snapshots) < 2 {
		return nil
	}
	rets := make([]float64, 0, len(snapshots)-1)
	for i := 1; i < len(snapshots); i++ {
		if prev := snapshots[i-1].Equity; prev > 0 {
			rets = append(rets, (snapshots[i].Equity-prev)/prev)
		}
	}
	n := len(rets)
	if n < 2 {
		return nil
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(n)
	variance := 0.0
	for _, r := range rets {
		d := r - mean
		variance += d * d
	}
	variance /= float64(n - 1)
	stddev := math.Sqrt(variance)
	if stddev == 0 {
		return nil
	}
	sharpe := mean / stddev * math.Sqrt(252)
	return &sharpe
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
