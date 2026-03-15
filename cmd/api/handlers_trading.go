package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// Session handlers
func (s *APIServer) handleListSessions(c *gin.Context) {
	ctx := c.Request.Context()

	sessions, err := s.db.ListActiveSessions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (s *APIServer) handleGetSession(c *gin.Context) {
	idStr := c.Param("id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session ID",
		})
		return
	}

	ctx := c.Request.Context()
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "session not found",
			"id":    idStr,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session": session,
	})
}

// Position handlers
func (s *APIServer) handleListPositions(c *gin.Context) {
	ctx := c.Request.Context()

	// Optional: filter by session_id query param
	sessionIDStr := c.Query("session_id")

	var positions []*db.Position
	var err error

	if sessionIDStr != "" {
		// Parse session ID
		sessionID, parseErr := parseUUID(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid session_id format",
			})
			return
		}
		positions, err = s.db.GetPositionsBySession(ctx, sessionID)
	} else {
		// Get all open positions (no session filter)
		positions, err = s.db.GetAllOpenPositions(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve positions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"positions": positions,
		"count":     len(positions),
	})
}

func (s *APIServer) handleGetPosition(c *gin.Context) {
	symbol := c.Param("symbol")
	ctx := c.Request.Context()

	// Optional: filter by session_id
	sessionIDStr := c.Query("session_id")

	var position *db.Position
	var err error

	if sessionIDStr != "" {
		sessionID, parseErr := parseUUID(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid session_id format",
			})
			return
		}
		position, err = s.db.GetPositionBySymbolAndSession(ctx, symbol, sessionID)
	} else {
		// Get latest position for symbol
		position, err = s.db.GetLatestPositionBySymbol(ctx, symbol)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  "position not found",
			"symbol": symbol,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"position": position,
	})
}

// Order handlers
func (s *APIServer) handleListOrders(c *gin.Context) {
	ctx := c.Request.Context()

	// Optional filters
	sessionIDStr := c.Query("session_id")
	symbol := c.Query("symbol")
	status := c.Query("status")

	var orders []*db.Order
	var err error

	if sessionIDStr != "" {
		sessionID, parseErr := parseUUID(sessionIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid session_id format",
			})
			return
		}
		orders, err = s.db.GetOrdersBySession(ctx, sessionID)
	} else if symbol != "" {
		orders, err = s.db.GetOrdersBySymbol(ctx, symbol)
	} else if status != "" {
		orders, err = s.db.GetOrdersByStatus(ctx, db.ConvertOrderStatus(status))
	} else {
		// Get recent orders (limit 100)
		orders, err = s.db.GetRecentOrders(ctx, 100)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve orders",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"count":  len(orders),
	})
}

func (s *APIServer) handleGetOrder(c *gin.Context) {
	orderIDStr := c.Param("id")
	ctx := c.Request.Context()

	orderID, err := parseUUID(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid order_id format",
		})
		return
	}

	order, err := s.db.GetOrderByID(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "order not found",
			"order_id": orderIDStr,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order": order,
	})
}

func (s *APIServer) handlePlaceOrder(c *gin.Context) {
	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Side     string  `json:"side" binding:"required,oneof=buy sell BUY SELL"`
		Type     string  `json:"type" binding:"required,oneof=market limit MARKET LIMIT"`
		Quantity float64 `json:"quantity" binding:"required,gt=0"`
		Price    float64 `json:"price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate price for limit orders
	if (req.Type == "limit" || req.Type == "LIMIT") && req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "price is required for limit orders",
		})
		return
	}

	ctx := c.Request.Context()

	// Snapshot the active session under lock to avoid a data race with
	// handleStartTrading/handleStopTrading which also hold sessionMu.
	s.sessionMu.Lock()
	sessionID := s.activeSessionID
	s.sessionMu.Unlock()

	// Create a tracking record with a known UUID so we can return it to the caller.
	price := &req.Price
	if req.Price == 0 {
		price = nil
	}
	order := &db.Order{
		ID:        uuid.New(),
		SessionID: sessionID,
		Symbol:    req.Symbol,
		Exchange:  "API",
		Side:      db.ConvertOrderSide(req.Side),
		Type:      db.ConvertOrderType(req.Type),
		Quantity:  req.Quantity,
		Price:     price,
		Status:    db.OrderStatusNew,
		PlacedAt:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.InsertOrder(ctx, order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	// Submit to the order-executor MCP server for execution
	execPrice := req.Price
	if err := s.executeOrder(ctx, req.Symbol, strings.ToUpper(req.Side), strings.ToUpper(req.Type), req.Quantity, execPrice); err != nil {
		log.Error().Err(err).Str("order_id", order.ID.String()).Msg("Order execution failed")

		// Mark as REJECTED
		errMsg := err.Error()
		now := time.Now()
		if updateErr := s.db.UpdateOrderStatus(ctx, order.ID, db.OrderStatusRejected, 0, 0, nil, &now, &errMsg); updateErr != nil {
			log.Error().Err(updateErr).Str("order_id", order.ID.String()).Msg("Failed to update order status to REJECTED")
		}
		order.Status = db.OrderStatusRejected
		order.ErrorMessage = &errMsg

		// Broadcast rejection to WebSocket clients
		if broadcastErr := s.BroadcastOrderUpdate(order); broadcastErr != nil {
			log.Warn().Err(broadcastErr).Msg("Failed to broadcast order rejection")
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"order":   order,
			"error":   "order execution failed",
			"details": errMsg,
		})
		return
	}

	// The order-executor MCP server creates its own FILLED order/trade records
	// with the actual fill price. We don't mark this tracking record as FILLED
	// because we don't have the fill price — storing averagePrice=0 would corrupt
	// P&L calculations. The tracking record stays as NEW (submitted to executor).
	// TODO: have executeOrder return fill details so we can update this record.

	log.Info().
		Str("order_id", order.ID.String()).
		Str("symbol", req.Symbol).
		Str("side", req.Side).
		Float64("quantity", req.Quantity).
		Msg("Order submitted to executor")

	// Update session stats if we have an active session (use snapshot from above)
	if sessionID != nil {
		if err := s.db.AggregateSessionStats(ctx, *sessionID); err != nil {
			log.Warn().Err(err).Str("session_id", sessionID.String()).Msg("Failed to aggregate session stats")
		}
	}

	// Broadcast success to WebSocket clients
	if broadcastErr := s.BroadcastOrderUpdate(order); broadcastErr != nil {
		log.Warn().Err(broadcastErr).Msg("Failed to broadcast order update")
	}

	c.JSON(http.StatusCreated, gin.H{
		"order":   order,
		"message": "Order executed successfully",
	})
}

func (s *APIServer) handleCancelOrder(c *gin.Context) {
	orderIDStr := c.Param("id")
	ctx := c.Request.Context()

	orderID, err := parseUUID(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid order_id format",
		})
		return
	}

	// Get the order first
	order, err := s.db.GetOrderByID(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "order not found",
			"order_id": orderIDStr,
		})
		return
	}

	// Check if order can be cancelled
	if order.Status != db.OrderStatusNew && order.Status != db.OrderStatusPartiallyFilled {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "order cannot be cancelled",
			"status": order.Status,
		})
		return
	}

	// Update order status to cancelled
	cancelledAt := time.Now()
	err = s.db.UpdateOrderStatus(ctx, orderID, db.OrderStatusCanceled, order.ExecutedQuantity, order.ExecutedQuoteQuantity, order.FilledAt, &cancelledAt, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to cancel order",
		})
		return
	}

	// Get updated order
	order, _ = s.db.GetOrderByID(ctx, orderID)

	// Broadcast order update to WebSocket clients
	if err := s.BroadcastOrderUpdate(order); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast order cancellation")
	}

	c.JSON(http.StatusOK, gin.H{
		"order":   order,
		"message": "Order cancelled successfully",
	})
}

// Trading control handlers
func (s *APIServer) handleStartTrading(c *gin.Context) {
	var req struct {
		Symbol         string  `json:"symbol" binding:"required"`
		InitialCapital float64 `json:"initial_capital" binding:"required,gt=0"`
		Mode           string  `json:"mode" binding:"oneof=paper live PAPER LIVE"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Default to paper trading if not specified
	if req.Mode == "" {
		req.Mode = "paper"
	}

	// Determine exchange from config (defaults to "binance" if not configured)
	exchange := s.config.Trading.Exchange
	if exchange == "" {
		exchange = "binance"
		log.Warn().Msg("No exchange configured in config.Trading.Exchange, defaulting to binance")
	}

	// Create a new trading session
	session := &db.TradingSession{
		Mode:           db.TradingMode(req.Mode),
		Symbol:         req.Symbol,
		Exchange:       exchange,
		StartedAt:      time.Now(),
		InitialCapital: req.InitialCapital,
	}

	ctx := c.Request.Context()
	if err := s.db.CreateSession(ctx, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create trading session",
		})
		return
	}

	// Track active session so subsequent orders are linked
	s.setActiveSessionID(&session.ID)

	// Broadcast system status update
	metadata := map[string]interface{}{
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
		"mode":       session.Mode,
		"event":      "trading_started",
	}
	if err := s.BroadcastSystemStatus("trading_started", "Trading session started", metadata); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast trading start")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Trading started successfully",
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
		"mode":       session.Mode,
		"started_at": session.StartedAt,
	})
}

func (s *APIServer) handleStopTrading(c *gin.Context) {
	var req struct {
		SessionID    string  `json:"session_id" binding:"required"`
		FinalCapital float64 `json:"final_capital" binding:"required,gte=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := parseUUID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	ctx := c.Request.Context()
	if err := s.db.StopSession(ctx, sessionID, req.FinalCapital); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to stop trading session",
		})
		return
	}

	// Clear active session AFTER the DB stop completes, so any in-flight
	// handlePlaceOrder that already snapshotted the session ID can still link.
	s.setActiveSessionID(nil)

	// Get updated session
	session, _ := s.db.GetSession(ctx, sessionID)

	// Broadcast system status update
	metadata := map[string]interface{}{
		"session_id":    session.ID.String(),
		"final_capital": req.FinalCapital,
		"total_pnl":     session.TotalPnL,
		"total_trades":  session.TotalTrades,
		"event":         "trading_stopped",
	}
	if err := s.BroadcastSystemStatus("trading_stopped", "Trading session stopped", metadata); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast trading stop")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Trading stopped successfully",
		"session_id":    session.ID.String(),
		"final_capital": req.FinalCapital,
		"total_pnl":     session.TotalPnL,
		"total_trades":  session.TotalTrades,
		"stopped_at":    session.StoppedAt,
	})
}

func (s *APIServer) handlePauseTrading(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := parseUUID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	// Get session to verify it exists
	ctx := c.Request.Context()
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": req.SessionID,
		})
		return
	}

	// Call orchestrator to pause trading with retry
	orchestratorURL := s.getOrchestratorURL()
	resp, err := s.callOrchestratorWithRetry(orchestratorURL + "/pause")
	if err != nil {
		log.Error().Err(err).Msg("Failed to call orchestrator pause endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to pause trading",
			"details": err.Error(),
		})
		return
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Error().Err(cerr).Msg("Failed to close response body")
		}
	}()

	// Check orchestrator response
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{
			"error": "orchestrator failed to pause trading",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Trading paused successfully",
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
	})
}

func (s *APIServer) handleResumeTrading(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	sessionID, err := parseUUID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid session_id format",
		})
		return
	}

	// Get session to verify it exists
	ctx := c.Request.Context()
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": req.SessionID,
		})
		return
	}

	// Call orchestrator to resume trading with retry
	orchestratorURL := s.getOrchestratorURL()
	resp, err := s.callOrchestratorWithRetry(orchestratorURL + "/resume")
	if err != nil {
		log.Error().Err(err).Msg("Failed to call orchestrator resume endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to resume trading",
			"details": err.Error(),
		})
		return
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Error().Err(cerr).Msg("Failed to close response body")
		}
	}()

	// Check orchestrator response
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{
			"error": "orchestrator failed to resume trading",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Trading resumed successfully",
		"session_id": session.ID.String(),
		"symbol":     session.Symbol,
	})
}
