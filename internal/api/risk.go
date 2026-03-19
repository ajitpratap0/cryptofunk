package api

import (
	"context"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/risk"
)

// RiskHandler provides REST endpoints for risk metrics and circuit breaker status.
type RiskHandler struct {
	db          *db.DB
	riskService *risk.Service
}

// NewRiskHandler creates a new RiskHandler backed by the given database.
func NewRiskHandler(database *db.DB) *RiskHandler {
	return &RiskHandler{
		db:          database,
		riskService: risk.NewService(),
	}
}

// RegisterRoutes mounts the /risk sub-group under the provided router group.
func (h *RiskHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc) {
	r := rg.Group("/risk")
	r.Use(readMiddleware)
	r.GET("/metrics", h.GetMetrics)
	r.GET("/circuit-breakers", h.GetCircuitBreakers)
	r.GET("/exposure", h.GetExposure)
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

	if len(returns) >= 10 {
		// CalculateVaR requires []interface{} not []float64
		returnsIface := make([]interface{}, len(returns))
		for i, v := range returns {
			returnsIface[i] = v
		}

		res95, err := h.riskService.CalculateVaR(map[string]interface{}{
			"returns":          returnsIface,
			"confidence_level": 0.95,
		})
		if err == nil {
			if varResult, ok := res95.(*risk.VaRResult); ok {
				response["var_95"] = varResult.VaR
			}
		}

		res99, err := h.riskService.CalculateVaR(map[string]interface{}{
			"returns":          returnsIface,
			"confidence_level": 0.99,
		})
		if err == nil {
			if varResult, ok := res99.(*risk.VaRResult); ok {
				response["var_99"] = varResult.VaR
				response["expected_shortfall"] = varResult.CVaR
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

	// Max Daily Loss: triggered when cumulative losses exceed $5000
	lossAmount := math.Abs(math.Min(totalPnL, 0))
	breakers := []gin.H{
		buildBreaker("Max Daily Loss", lossAmount, 5000),
		buildBreaker("Max Drawdown %", maxDrawdown*100, 10),
		buildBreaker("Total Trade Count", float64(totalTrades), 100),
	}

	c.JSON(http.StatusOK, gin.H{"circuit_breakers": breakers, "count": len(breakers)})
}

func buildBreaker(name string, current, threshold float64) gin.H {
	status := "OK"
	if current >= threshold {
		status = "TRIGGERED"
	} else if threshold > 0 && current/threshold >= 0.8 {
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

	c.JSON(http.StatusOK, gin.H{"exposure": result, "count": len(result)})
}

// collectClosedReturns gathers RealizedPnL from closed positions across active sessions.
func (h *RiskHandler) collectClosedReturns(ctx context.Context) ([]float64, error) {
	sessions, err := h.db.ListActiveSessions(ctx)
	if err != nil {
		return nil, err
	}

	var returns []float64
	for _, s := range sessions {
		positions, err := h.db.GetPositionsBySession(ctx, s.ID)
		if err != nil {
			continue
		}
		for _, p := range positions {
			if p.ExitTime != nil && p.RealizedPnL != nil {
				returns = append(returns, *p.RealizedPnL)
			}
		}
	}
	return returns, nil
}
