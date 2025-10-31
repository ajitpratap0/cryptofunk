package api

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Root handler
func (s *Server) handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "CryptoFunk API",
		"version": "1.0.0",
		"status":  "running",
		"time":    time.Now().UTC(),
	})
}

// T152: Status endpoints

// handleGetStatus returns comprehensive system status
func (s *Server) handleGetStatus(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Check database connection
	dbStatus := "healthy"
	if s.db != nil {
		if err := s.db.Ping(c.Request.Context()); err != nil {
			dbStatus = "unhealthy"
			log.Warn().Err(err).Msg("Database health check failed")
		}
	} else {
		dbStatus = "not_configured"
	}

	// Determine overall system status
	systemStatus := "healthy"
	if dbStatus != "healthy" {
		systemStatus = "degraded"
	}

	status := gin.H{
		"status":    systemStatus,
		"timestamp": time.Now().UTC(),
		"uptime":    time.Since(startTime).Seconds(),
		"version":   "1.0.0",
		"components": gin.H{
			"database": gin.H{
				"status": dbStatus,
			},
			"exchange_service": gin.H{
				"status": func() string {
					if s.service != nil {
						return "configured"
					}
					return "not_configured"
				}(),
			},
		},
		"system": gin.H{
			"goroutines": runtime.NumGoroutine(),
			"memory": gin.H{
				"alloc_mb":       toMB(memStats.Alloc),
				"total_alloc_mb": toMB(memStats.TotalAlloc),
				"sys_mb":         toMB(memStats.Sys),
				"num_gc":         memStats.NumGC,
			},
			"go_version": runtime.Version(),
		},
	}

	c.JSON(http.StatusOK, status)
}

// handleGetHealth returns a simple health check (for load balancers)
func (s *Server) handleGetHealth(c *gin.Context) {
	// Quick health check - just verify database connectivity
	if s.db != nil {
		if err := s.db.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "database unavailable",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().UTC(),
	})
}

// T153: Agent endpoints

func (s *Server) handleListAgents(c *gin.Context) {
	// Return mock data for now - agent system not fully integrated yet
	agents := []gin.H{
		{
			"name":        "technical-agent",
			"type":        "analysis",
			"status":      "active",
			"last_signal": time.Now().Add(-5 * time.Minute).UTC(),
			"confidence":  0.75,
		},
		{
			"name":        "trend-agent",
			"type":        "strategy",
			"status":      "active",
			"last_signal": time.Now().Add(-2 * time.Minute).UTC(),
			"confidence":  0.82,
		},
		{
			"name":        "risk-agent",
			"type":        "risk",
			"status":      "active",
			"last_signal": time.Now().Add(-1 * time.Minute).UTC(),
			"confidence":  0.90,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"total":  len(agents),
	})
}

func (s *Server) handleGetAgent(c *gin.Context) {
	name := c.Param("name")

	// Mock agent data
	agent := gin.H{
		"name":        name,
		"type":        "analysis",
		"status":      "active",
		"uptime":      3600.0,
		"last_signal": time.Now().Add(-2 * time.Minute).UTC(),
		"confidence":  0.75,
		"metrics": gin.H{
			"signals_generated": 145,
			"avg_confidence":    0.78,
			"success_rate":      0.65,
		},
	}

	c.JSON(http.StatusOK, agent)
}

func (s *Server) handleGetAgentStatus(c *gin.Context) {
	name := c.Param("name")

	// Mock status data
	status := gin.H{
		"name":           name,
		"status":         "active",
		"health":         "healthy",
		"last_heartbeat": time.Now().Add(-30 * time.Second).UTC(),
		"metrics": gin.H{
			"response_time_ms": 45.2,
			"error_rate":       0.02,
			"cpu_usage":        15.3,
			"memory_mb":        128.5,
		},
	}

	c.JSON(http.StatusOK, status)
}

// T154: Position endpoints

func (s *Server) handleListPositions(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "database not available",
		})
		return
	}

	// Parse query parameters
	openOnly := c.DefaultQuery("open_only", "true") == "true"
	symbol := c.Query("symbol")

	var symbolPtr *string
	if symbol != "" {
		symbolPtr = &symbol
	}

	// TODO: Get session ID from context/auth
	// For now, list all positions
	positions, err := s.db.ListPositions(c.Request.Context(), nil, symbolPtr, openOnly, 100, 0)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list positions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve positions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"positions": positions,
		"total":     len(positions),
	})
}

func (s *Server) handleGetPosition(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "database not available",
		})
		return
	}

	symbol := c.Param("symbol")

	// TODO: Get session ID from context/auth
	// For now, return error asking for session ID in query
	sessionIDStr := c.Query("session_id")
	if sessionIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id query parameter required",
		})
		return
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	position, err := s.db.GetPositionBySymbol(c.Request.Context(), sessionID, symbol)
	if err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Position not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("no open position found for %s", symbol),
		})
		return
	}

	c.JSON(http.StatusOK, position)
}

// T155: Order endpoints

func (s *Server) handleListOrders(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "database not available",
		})
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	// TODO: Get session ID from context/auth
	// For now, list all orders
	orders, err := s.db.ListOrders(c.Request.Context(), nil, nil, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list orders")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve orders",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"total":  len(orders),
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleGetOrder(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "database not available",
		})
		return
	}

	orderIDStr := c.Param("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid order ID format",
		})
		return
	}

	order, err := s.db.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		log.Warn().Err(err).Str("order_id", orderIDStr).Msg("Order not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "order not found",
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (s *Server) handlePlaceOrder(c *gin.Context) {
	if s.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "exchange service not configured",
		})
		return
	}

	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Side     string  `json:"side" binding:"required"`
		Type     string  `json:"type" binding:"required"`
		Quantity float64 `json:"quantity" binding:"required,gt=0"`
		Price    float64 `json:"price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// Build parameters for exchange service
	params := map[string]interface{}{
		"symbol":   req.Symbol,
		"side":     req.Side,
		"quantity": req.Quantity,
	}

	if req.Price > 0 {
		params["price"] = req.Price
	}

	// Call appropriate service method based on order type
	var result interface{}
	var err error

	if req.Type == "market" {
		result, err = s.service.PlaceMarketOrder(params)
	} else if req.Type == "limit" {
		result, err = s.service.PlaceLimitOrder(params)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "order type must be 'market' or 'limit'",
		})
		return
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to place order")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to place order: %v", err),
		})
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (s *Server) handleCancelOrder(c *gin.Context) {
	if s.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "exchange service not configured",
		})
		return
	}

	orderID := c.Param("id")

	params := map[string]interface{}{
		"order_id": orderID,
	}

	result, err := s.service.CancelOrder(params)
	if err != nil {
		log.Error().Err(err).Str("order_id", orderID).Msg("Failed to cancel order")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to cancel order: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// T156: Control endpoints

func (s *Server) handleStartTrading(c *gin.Context) {
	var req struct {
		Symbol         string                 `json:"symbol" binding:"required"`
		InitialCapital float64                `json:"initial_capital" binding:"required,gt=0"`
		Config         map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if s.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "exchange service not configured",
		})
		return
	}

	// Start trading session
	params := map[string]interface{}{
		"symbol":          req.Symbol,
		"initial_capital": req.InitialCapital,
	}

	if req.Config != nil {
		params["config"] = req.Config
	}

	result, err := s.service.StartSession(params)
	if err != nil {
		log.Error().Err(err).Msg("Failed to start trading session")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to start trading: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) handleStopTrading(c *gin.Context) {
	if s.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "exchange service not configured",
		})
		return
	}

	params := map[string]interface{}{}
	result, err := s.service.StopSession(params)
	if err != nil {
		log.Error().Err(err).Msg("Failed to stop trading session")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to stop trading: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) handlePauseTrading(c *gin.Context) {
	// Pause is not implemented in the exchange service yet
	// Return a stub response for now
	c.JSON(http.StatusOK, gin.H{
		"status":  "paused",
		"message": "trading paused (note: pause functionality not fully implemented yet)",
		"time":    time.Now().UTC(),
	})
}

// T157: Config endpoints

func (s *Server) handleGetConfig(c *gin.Context) {
	// Return current configuration
	// In a real implementation, this would load from Viper or a config manager
	config := gin.H{
		"api": gin.H{
			"host": "0.0.0.0",
			"port": 8080,
		},
		"exchange": gin.H{
			"mode": func() string {
				if s.service != nil {
					return "configured"
				}
				return "not_configured"
			}(),
		},
		"database": gin.H{
			"status": func() string {
				if s.db != nil {
					return "connected"
				}
				return "not_connected"
			}(),
		},
		"features": gin.H{
			"paper_trading": true,
			"live_trading":  true,
			"websocket":     false, // Not implemented yet
		},
	}

	c.JSON(http.StatusOK, config)
}

func (s *Server) handleUpdateConfig(c *gin.Context) {
	var updates map[string]interface{}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// In a real implementation, this would update Viper config
	// For now, just acknowledge the request
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "configuration updates acknowledged (note: config persistence not fully implemented yet)",
		"updates": updates,
		"time":    time.Now().UTC(),
	})
}

// Utility functions

var startTime = time.Now()

func toMB(bytes uint64) uint64 {
	return bytes / 1024 / 1024
}
