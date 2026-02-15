package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/resolver"
)

// DecisionAnalyticsHandler handles decision analytics and outcome resolution endpoints.
type DecisionAnalyticsHandler struct {
	db       *db.DB
	resolver *resolver.Resolver
}

// NewDecisionAnalyticsHandler creates a new handler.
func NewDecisionAnalyticsHandler(database *db.DB) *DecisionAnalyticsHandler {
	gammaClient := gamma.NewClient()
	return &DecisionAnalyticsHandler{
		db:       database,
		resolver: resolver.NewResolver(database, gammaClient),
	}
}

// RegisterRoutes registers the decision analytics routes.
func (h *DecisionAnalyticsHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc) {
	decisions := rg.Group("/decisions")
	{
		decisions.GET("/analytics", readMiddleware, h.handleGetAnalytics)
		decisions.GET("/outcomes", readMiddleware, h.handleGetOutcomes)
	}

	admin := rg.Group("/admin")
	admin.Use(authMiddleware)
	{
		admin.POST("/resolve-outcomes", h.handleResolveOutcomes)
	}
}

func (h *DecisionAnalyticsHandler) handleGetAnalytics(c *gin.Context) {
	sinceStr := c.DefaultQuery("since", "30d")
	since := parseDuration(sinceStr)

	analytics, err := h.db.GetDecisionAnalytics(c.Request.Context(), since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    analytics,
	})
}

func (h *DecisionAnalyticsHandler) handleGetOutcomes(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	outcomes, err := h.db.GetDecisionsWithOutcomes(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    outcomes,
	})
}

func (h *DecisionAnalyticsHandler) handleResolveOutcomes(c *gin.Context) {
	polyResolved, binanceResolved, err := h.resolver.ResolveAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":               err.Error(),
			"polymarket_resolved": polyResolved,
			"binance_resolved":    binanceResolved,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"polymarket_resolved": polyResolved,
		"binance_resolved":    binanceResolved,
	})
}

// parseDuration parses a human-friendly duration like "30d", "7d", "1h".
func parseDuration(s string) time.Time {
	if len(s) < 2 {
		return time.Now().AddDate(0, -1, 0)
	}
	num, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return time.Now().AddDate(0, -1, 0)
	}
	unit := s[len(s)-1]
	switch unit {
	case 'h':
		return time.Now().Add(-time.Duration(num) * time.Hour)
	case 'd':
		return time.Now().AddDate(0, 0, -num)
	case 'm':
		return time.Now().AddDate(0, -num, 0)
	case 'y':
		return time.Now().AddDate(-num, 0, 0)
	default:
		return time.Now().AddDate(0, -1, 0)
	}
}
