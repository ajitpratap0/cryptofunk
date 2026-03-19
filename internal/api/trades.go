package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// TradesHandler serves fill records from the trades table.
type TradesHandler struct {
	db *db.DB
}

func NewTradesHandler(database *db.DB) *TradesHandler {
	return &TradesHandler{db: database}
}

func (h *TradesHandler) RegisterRoutes(rg *gin.RouterGroup, readMiddleware gin.HandlerFunc) {
	trades := rg.Group("/trades")
	trades.Use(readMiddleware)
	trades.GET("", h.ListTrades)
}

// ListTrades returns recent trade fills, newest first.
// GET /api/v1/trades?limit=50&offset=0
func (h *TradesHandler) ListTrades(c *gin.Context) {
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	ctx := c.Request.Context()
	trades, err := h.db.ListAllTrades(ctx, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch trades"})
		return
	}
	if trades == nil {
		trades = []*db.Trade{}
	}

	total, err := h.db.CountAllTrades(ctx)
	if err != nil {
		// Non-fatal: return the page without total rather than failing the request.
		total = -1
	}

	c.JSON(http.StatusOK, gin.H{
		"trades": trades,
		"count":  len(trades),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
