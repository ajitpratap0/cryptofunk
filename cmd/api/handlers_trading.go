package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/exchange"
)

// ptrStr and ptrF64 are local pointer helpers used only within this file.
// They are not duplicated elsewhere in the codebase (verified via grep).
func ptrStr(s string) *string   { return &s }
func ptrF64(f float64) *float64 { return &f }

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

	// Mark tracking order as FILLED so AggregateSessionStats counts it.
	// Fill price is 0 in this record — actual price is in executor's trade records.
	now := time.Now()
	if err := s.db.UpdateOrderStatus(ctx, order.ID, db.OrderStatusFilled, order.Quantity, 0, &now, nil, nil); err != nil {
		log.Error().Err(err).Str("order_id", order.ID.String()).Msg("Failed to update order to FILLED")
	}
	// KNOWN LIMITATION: The tracking record stores price=0 and ExecutedQuoteQuantity=0.
	// The real fill price lives in the executor's own trade records (inserted by order-executor MCP).
	// AggregateSessionStats counts this order as a filled trade, but any P&L derived
	// from this record's price fields will be incorrect. Do not use for price arithmetic.
	order.Status = db.OrderStatusFilled
	order.ExecutedQuantity = order.Quantity
	filledAt := now
	order.FilledAt = &filledAt

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

// handlePaperTrade executes a paper (simulated) trade order.
// Market orders are immediately filled; limit orders remain open (NEW status).
// POST /api/v1/trade
func (s *APIServer) handlePaperTrade(c *gin.Context) {
	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Side     string  `json:"side" binding:"required,oneof=buy sell BUY SELL"`
		Type     string  `json:"type" binding:"required,oneof=market limit MARKET LIMIT"`
		Quantity float64 `json:"quantity" binding:"required,gt=0"`
		Price    float64 `json:"price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	isLimit := strings.EqualFold(req.Type, "limit")
	if isLimit && req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price is required for limit orders"})
		return
	}

	ctx := c.Request.Context()

	// 1. Resolve or create paper session
	sessions, err := s.db.ListActiveSessions(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list active sessions for paper trade")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve trading session"})
		return
	}
	var sessionID *uuid.UUID
	for i := range sessions {
		if sessions[i].Mode == db.TradingModePaper {
			id := sessions[i].ID
			sessionID = &id
			break
		}
	}
	if sessionID == nil {
		newSession := &db.TradingSession{
			ID:             uuid.New(),
			Mode:           db.TradingModePaper,
			Symbol:         req.Symbol,
			Exchange:       "paper",
			InitialCapital: 100_000.0,
			StartedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.db.CreateSession(ctx, newSession); err != nil {
			// Only retry on unique constraint violation (PG error code 23505).
			// A concurrent request may have inserted the same paper session between the
			// ListActiveSessions call above and this insert (TOCTOU race). In that case
			// we look up and reuse the existing session.
			// All other errors (e.g. connection timeout) are returned immediately.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				log.Warn().Err(err).Msg("Unique constraint on paper session; retrying lookup for concurrent session")
				sessions2, err2 := s.db.ListActiveSessions(ctx)
				if err2 == nil {
					for i := range sessions2 {
						if sessions2[i].Mode == db.TradingModePaper {
							id := sessions2[i].ID
							sessionID = &id
							break
						}
					}
				}
				if sessionID == nil {
					log.Error().Err(err).Msg("Failed to find paper session after unique constraint violation")
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create trading session"})
					return
				}
			} else {
				log.Error().Err(err).Msg("Failed to create paper session")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("failed to create paper session: %v", err),
				})
				return
			}
		} else {
			sessionID = &newSession.ID
		}
	}

	// 2. Determine execution price
	refPrice := req.Price
	if !isLimit && refPrice <= 0 {
		mockEx := exchange.NewMockExchange(s.db)
		refPrice = mockEx.GetMarketPrice(req.Symbol)
		if refPrice <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no market price configured for symbol; provide a price field",
			})
			return
		}
	}
	execPrice := refPrice
	if !isLimit {
		if strings.EqualFold(req.Side, "BUY") {
			execPrice = refPrice * 1.001
		} else {
			execPrice = refPrice * 0.999
		}
	}

	// 3. Insert order
	now := time.Now()
	var pricePtr *float64
	if req.Price > 0 {
		pricePtr = ptrF64(req.Price)
	}
	orderSide := db.ConvertOrderSide(req.Side)
	orderType := db.ConvertOrderType(req.Type)

	order := &db.Order{
		ID:        uuid.New(),
		SessionID: sessionID,
		Symbol:    req.Symbol,
		Exchange:  "paper",
		Side:      orderSide,
		Type:      orderType,
		Quantity:  req.Quantity,
		Price:     pricePtr,
		Status:    db.OrderStatusNew,
		PlacedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.InsertOrder(ctx, order); err != nil {
		log.Error().Err(err).Msg("Failed to insert paper trade order")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create paper trade order"})
		return
	}

	// 4. Immediate fill for market orders
	if order.Type == db.OrderTypeMarket {
		execQuoteQty := execPrice * req.Quantity
		// Use the configured paper trading commission rate (taker fee from exchanges config).
		// Falls back to 0.001 (0.1%) if not configured, matching Binance standard tier.
		commissionRate := s.config.Trading.CommissionRate
		if commissionRate <= 0 {
			commissionRate = 0.001
		}
		commission := execQuoteQty * commissionRate

		if err := s.db.UpdateOrderStatus(ctx, order.ID, db.OrderStatusFilled,
			req.Quantity, execQuoteQty, &now, nil, nil); err != nil {
			log.Warn().Err(err).Str("order_id", order.ID.String()).Msg("Failed to mark paper order filled")
		} else {
			order.Status = db.OrderStatusFilled
			order.ExecutedQuantity = req.Quantity
			order.ExecutedQuoteQuantity = execQuoteQty
			order.FilledAt = &now
			order.UpdatedAt = now
		}

		// Write fill record
		commissionAsset := "USDT"
		trade := &db.Trade{
			ID:              uuid.New(),
			OrderID:         order.ID,
			Symbol:          req.Symbol,
			Exchange:        "paper",
			Side:            orderSide,
			Price:           execPrice,
			Quantity:        req.Quantity,
			QuoteQuantity:   execQuoteQty,
			Commission:      commission,
			CommissionAsset: &commissionAsset,
			ExecutedAt:      now,
			IsMaker:         false,
			CreatedAt:       now,
		}
		if err := s.db.InsertTrade(ctx, trade); err != nil {
			log.Warn().Err(err).Msg("Failed to insert paper trade fill row")
		}

		// Create or average into existing position
		existingPos, posErr := s.db.GetPositionBySymbolAndSession(ctx, req.Symbol, *sessionID)
		if posErr != nil && !errors.Is(posErr, pgx.ErrNoRows) {
			log.Warn().Err(posErr).Msg("Error looking up existing position")
		}

		posSide := db.PositionSideLong
		if orderSide == db.OrderSideSell {
			posSide = db.PositionSideShort
		}

		if existingPos != nil && existingPos.Side != posSide {
			// Opposite-side trade on an existing open position. Proper close/reduce
			// logic (netting, realized PnL calculation) is not yet implemented.
			// For now we average into the opposite direction, which is incorrect for
			// a long→short flip. This is a known limitation.
			// TODO: implement position close/reduce logic.
			log.Warn().
				Str("symbol", req.Symbol).
				Str("existing_side", string(existingPos.Side)).
				Str("order_side", string(posSide)).
				Msg("Opposite-side trade on existing position; position close logic not yet implemented")
		}

		if existingPos == nil {
			pos := &db.Position{
				ID:            uuid.New(),
				SessionID:     sessionID,
				Symbol:        req.Symbol,
				Exchange:      "paper",
				Side:          posSide,
				EntryPrice:    execPrice,
				Quantity:      req.Quantity,
				EntryTime:     now,
				Fees:          commission,
				EntryReason:   ptrStr("paper_trade_api"),
				UnrealizedPnL: ptrF64(0),
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := s.db.CreatePosition(ctx, pos); err != nil {
				log.Warn().Err(err).Msg("Failed to create position for paper trade")
			}
		} else {
			totalQty := existingPos.Quantity + req.Quantity
			weightedAvg := (existingPos.Quantity*existingPos.EntryPrice + req.Quantity*execPrice) / totalQty
			if err := s.db.UpdatePositionAveraging(ctx, existingPos.ID, weightedAvg, totalQty, commission); err != nil {
				log.Warn().Err(err).Msg("Failed to update position for paper trade")
			}
		}

		if err := s.db.AggregateSessionStats(ctx, *sessionID); err != nil {
			log.Warn().Err(err).Msg("Failed to aggregate session stats after paper trade")
		}
	}

	if err := s.BroadcastOrderUpdate(order); err != nil {
		log.Warn().Err(err).Msg("Failed to broadcast paper trade order update")
	}

	c.JSON(http.StatusCreated, gin.H{
		"order":        order,
		"message":      "Paper trade order executed successfully",
		"trading_mode": "paper",
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

	// Default to paper trading if not specified; normalize to uppercase for DB enum
	if req.Mode == "" {
		req.Mode = "PAPER"
	} else {
		req.Mode = strings.ToUpper(req.Mode)
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

	// Cleanup any stale NEW orders older than 5 minutes, scoped to this session only.
	if cleaned, err := s.db.CleanupStaleOrders(ctx, sessionID, 5*time.Minute); err != nil {
		log.Warn().Err(err).Msg("Failed to cleanup stale orders")
	} else if cleaned > 0 {
		log.Info().Int64("cleaned", cleaned).Msg("Cleaned up stale NEW orders")
	}

	// Clear active session AFTER the DB stop completes, so any in-flight
	// handlePlaceOrder that already snapshotted the session ID can still link.
	s.setActiveSessionID(nil)

	// Get updated session — handle error gracefully to avoid nil dereference panic
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to retrieve session after stop")
		c.JSON(http.StatusOK, gin.H{
			"message":       "Trading stopped successfully",
			"session_id":    sessionID.String(),
			"final_capital": req.FinalCapital,
		})
		return
	}

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
