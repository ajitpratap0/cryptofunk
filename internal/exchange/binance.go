package exchange

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	binance "github.com/adshao/go-binance/v2"
	"github.com/ajitpratap0/cryptofunk/internal/alerts"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// BinanceExchange implements Exchange interface for real Binance trading
type BinanceExchange struct {
	client *binance.Client
	db     *db.DB
	mu     sync.RWMutex

	// Order tracking
	orders                  map[string]*Order // Internal UUID -> Order
	fills                   map[string][]Fill // Internal UUID -> Fills
	exchangeOrderToInternal map[string]string // Exchange OrderID -> Internal UUID

	// Session tracking
	currentSessionID *uuid.UUID

	// Configuration
	testnet bool

	// WebSocket
	wsClient    *binance.Client
	listenKey   string
	wsStopChan  chan struct{}
	wsErrChan   chan error
	positionMgr *PositionManager
	wsConnected bool
}

// BinanceConfig contains configuration for Binance exchange
type BinanceConfig struct {
	APIKey    string
	SecretKey string
	Testnet   bool
}

// NewBinanceExchange creates a new Binance exchange client
func NewBinanceExchange(config BinanceConfig, database *db.DB) (*BinanceExchange, error) {
	// Create Binance client
	client := binance.NewClient(config.APIKey, config.SecretKey)

	// Set testnet if configured
	if config.Testnet {
		binance.UseTestnet = true
		log.Info().Msg("Binance exchange initialized (TESTNET mode)")
	} else {
		log.Warn().Msg("Binance exchange initialized (LIVE TRADING mode)")
	}

	exchange := &BinanceExchange{
		client:                  client,
		db:                      database,
		orders:                  make(map[string]*Order),
		fills:                   make(map[string][]Fill),
		exchangeOrderToInternal: make(map[string]string),
		testnet:                 config.Testnet,
		wsStopChan:              make(chan struct{}),
		wsErrChan:               make(chan error, 10),
		positionMgr:             NewPositionManager(database),
	}

	return exchange, nil
}

// PlaceOrder places a new order on Binance
func (b *BinanceExchange) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Validate request
	if err := b.validateOrder(req); err != nil {
		log.Warn().
			Err(err).
			Str("symbol", req.Symbol).
			Str("side", string(req.Side)).
			Msg("Order validation failed")

		return &PlaceOrderResponse{
			Status:  OrderStatusRejected,
			Message: err.Error(),
		}, nil
	}

	// Create Binance order with retry logic
	var binanceOrder *binance.CreateOrderResponse
	var err error
	side := binance.SideTypeBuy
	if req.Side == OrderSideSell {
		side = binance.SideTypeSell
	}

	// Wrap order placement in retry logic
	operationName := fmt.Sprintf("place_%s_order_%s", req.Type, req.Symbol)
	err = retryWithBackoff(func() error {
		if req.Type == OrderTypeMarket {
			// Market order
			binanceOrder, err = b.client.NewCreateOrderService().
				Symbol(req.Symbol).
				Side(side).
				Type(binance.OrderTypeMarket).
				Quantity(fmt.Sprintf("%.8f", req.Quantity)).
				Do(ctx)
		} else {
			// Limit order
			binanceOrder, err = b.client.NewCreateOrderService().
				Symbol(req.Symbol).
				Side(side).
				Type(binance.OrderTypeLimit).
				TimeInForce(binance.TimeInForceTypeGTC).
				Quantity(fmt.Sprintf("%.8f", req.Quantity)).
				Price(fmt.Sprintf("%.8f", req.Price)).
				Do(ctx)
		}
		return err
	}, operationName)

	if err != nil {
		log.Error().
			Err(err).
			Str("symbol", req.Symbol).
			Str("side", string(req.Side)).
			Msg("Failed to place order on Binance after retries")

		// Send critical alert for order failure
		alerts.AlertOrderFailed(ctx, req.Symbol, string(req.Side), req.Quantity, err)

		return &PlaceOrderResponse{
			Status:  OrderStatusRejected,
			Message: err.Error(),
		}, fmt.Errorf("failed to place order: %w", err)
	}

	// Convert Binance order to internal Order struct
	order := b.convertBinanceOrder(binanceOrder, req)

	// Store order and reverse mapping
	b.orders[order.ID] = order
	b.exchangeOrderToInternal[order.ExchangeOrderID] = order.ID

	// Persist to database
	if b.db != nil {
		dbOrder := b.convertToDBOrder(order)
		if err := b.db.InsertOrder(ctx, dbOrder); err != nil {
			log.Error().
				Err(err).
				Str("order_id", order.ID).
				Msg("Failed to persist order to database")
			// Continue even if database insert fails
		}
	}

	log.Info().
		Str("order_id", order.ID).
		Str("exchange_order_id", strconv.FormatInt(binanceOrder.OrderID, 10)).
		Str("symbol", order.Symbol).
		Str("side", string(order.Side)).
		Str("status", string(order.Status)).
		Msg("Order placed on Binance")

	return &PlaceOrderResponse{
		OrderID: order.ID,
		Status:  order.Status,
		Message: "Order placed successfully",
	}, nil
}

// CancelOrder cancels an open order on Binance
func (b *BinanceExchange) CancelOrder(ctx context.Context, orderID string) (*Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	order, exists := b.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status != OrderStatusOpen && order.Status != OrderStatusPending {
		return nil, fmt.Errorf("cannot cancel order in status: %s", order.Status)
	}

	// Use the stored exchange order ID
	binanceOrderID, err := strconv.ParseInt(order.ExchangeOrderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid exchange order ID format: %w", err)
	}

	// Cancel order on Binance with retry logic
	operationName := fmt.Sprintf("cancel_order_%s", order.Symbol)
	err = retryWithBackoff(func() error {
		_, err = b.client.NewCancelOrderService().
			Symbol(order.Symbol).
			OrderID(binanceOrderID).
			Do(ctx)
		return err
	}, operationName)

	if err != nil {
		log.Error().
			Err(err).
			Str("order_id", orderID).
			Msg("Failed to cancel order on Binance after retries")

		// Send warning alert for cancel failure
		alerts.AlertOrderCancelFailed(ctx, orderID, order.Symbol, err)

		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	// Update order status
	order.Status = OrderStatusCancelled
	cancelledAt := time.Now()
	order.UpdatedAt = cancelledAt

	// Update in database
	if b.db != nil {
		orderUUID, _ := uuid.Parse(orderID)
		status := db.ConvertOrderStatus(string(order.Status))
		err := b.db.UpdateOrderStatus(
			ctx,
			orderUUID,
			status,
			order.FilledQty,
			order.FilledQty*order.AvgFillPrice,
			order.FilledAt,
			&cancelledAt,
			nil,
		)
		if err != nil {
			log.Error().
				Err(err).
				Str("order_id", orderID).
				Msg("Failed to update cancelled order in database")
		}
	}

	log.Info().
		Str("order_id", orderID).
		Msg("Order cancelled on Binance")

	return order, nil
}

// GetOrder retrieves order details from Binance
func (b *BinanceExchange) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	b.mu.RLock()
	order, exists := b.orders[orderID]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	// Query Binance for latest order status
	binanceOrderID, err := strconv.ParseInt(order.ExchangeOrderID, 10, 64)
	if err != nil {
		return order, nil // Return cached order if exchange ID parsing fails
	}

	// Query Binance with retry logic
	var binanceOrder *binance.Order
	operationName := fmt.Sprintf("get_order_%s", order.Symbol)
	err = retryWithBackoff(func() error {
		binanceOrder, err = b.client.NewGetOrderService().
			Symbol(order.Symbol).
			OrderID(binanceOrderID).
			Do(ctx)
		return err
	}, operationName)

	if err != nil {
		log.Warn().
			Err(err).
			Str("order_id", orderID).
			Msg("Failed to query order status from Binance after retries, returning cached")
		return order, nil
	}

	// Update order with latest data
	b.mu.Lock()
	b.updateOrderFromBinance(order, binanceOrder)
	b.mu.Unlock()

	return order, nil
}

// GetOrderFills retrieves all fills for an order
func (b *BinanceExchange) GetOrderFills(ctx context.Context, orderID string) ([]Fill, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	fills, exists := b.fills[orderID]
	if !exists {
		return []Fill{}, nil
	}

	return fills, nil
}

// SetMarketPrice is a no-op for real exchange (market prices come from exchange)
func (b *BinanceExchange) SetMarketPrice(symbol string, price float64) {
	// No-op for real exchange
	log.Debug().
		Str("symbol", symbol).
		Float64("price", price).
		Msg("SetMarketPrice called on BinanceExchange (no-op)")
}

// SetSession sets the current trading session
func (b *BinanceExchange) SetSession(sessionID *uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.currentSessionID = sessionID

	if sessionID != nil {
		log.Info().
			Str("session_id", sessionID.String()).
			Msg("Trading session set for Binance exchange")
	} else {
		log.Info().Msg("Trading session cleared for Binance exchange")
	}
}

// GetSession returns the current trading session ID
func (b *BinanceExchange) GetSession() *uuid.UUID {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.currentSessionID
}

// Helper methods

// retryConfig holds retry configuration
const (
	maxRetries     = 3
	baseRetryDelay = 100 * time.Millisecond
)

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "network is unreachable") {
		return true
	}

	// Binance API rate limit
	if strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") {
		return true
	}

	// Server errors (5xx)
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "service unavailable") {
		return true
	}

	return false
}

// retryWithBackoff executes a function with exponential backoff
func retryWithBackoff(operation func() error, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Try the operation
		err := operation()
		if err == nil {
			if attempt > 0 {
				log.Info().
					Str("operation", operationName).
					Int("attempts", attempt+1).
					Msg("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !isRetryableError(err) {
			log.Debug().
				Err(err).
				Str("operation", operationName).
				Msg("Error is not retryable")
			return err
		}

		// Don't sleep after the last attempt
		if attempt < maxRetries {
			// Calculate exponential backoff: baseDelay * 2^attempt
			delay := baseRetryDelay * time.Duration(1<<uint(attempt))

			log.Warn().
				Err(err).
				Str("operation", operationName).
				Int("attempt", attempt+1).
				Int("max_attempts", maxRetries+1).
				Dur("retry_after", delay).
				Msg("Retrying operation after error")

			time.Sleep(delay)
		}
	}

	log.Error().
		Err(lastErr).
		Str("operation", operationName).
		Int("attempts", maxRetries+1).
		Msg("Operation failed after all retries")

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries+1, lastErr)
}

func (b *BinanceExchange) validateOrder(req PlaceOrderRequest) error {
	if req.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if req.Side != OrderSideBuy && req.Side != OrderSideSell {
		return fmt.Errorf("invalid order side: %s", req.Side)
	}

	if req.Type != OrderTypeMarket && req.Type != OrderTypeLimit {
		return fmt.Errorf("invalid order type: %s", req.Type)
	}

	if req.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}

	if req.Type == OrderTypeLimit && req.Price <= 0 {
		return fmt.Errorf("limit orders must have a positive price")
	}

	return nil
}

func (b *BinanceExchange) convertBinanceOrder(binanceOrder *binance.CreateOrderResponse, req PlaceOrderRequest) *Order {
	now := time.Now()

	// Parse executed quantity
	executedQty, _ := strconv.ParseFloat(binanceOrder.ExecutedQuantity, 64)
	cummulativeQuoteQty, _ := strconv.ParseFloat(binanceOrder.CummulativeQuoteQuantity, 64)

	// Calculate average fill price
	var avgFillPrice float64
	if executedQty > 0 {
		avgFillPrice = cummulativeQuoteQty / executedQty
	}

	// Map Binance status to internal status
	var status OrderStatus
	switch binanceOrder.Status {
	case binance.OrderStatusTypeNew:
		status = OrderStatusOpen
	case binance.OrderStatusTypePartiallyFilled:
		status = OrderStatusOpen
	case binance.OrderStatusTypeFilled:
		status = OrderStatusFilled
	case binance.OrderStatusTypeCanceled:
		status = OrderStatusCancelled
	case binance.OrderStatusTypeRejected:
		status = OrderStatusRejected
	default:
		status = OrderStatusPending
	}

	// Generate UUID for internal ID, store Binance OrderID separately
	internalID := uuid.New().String()
	exchangeOrderID := strconv.FormatInt(binanceOrder.OrderID, 10)

	return &Order{
		ID:              internalID,
		ExchangeOrderID: exchangeOrderID,
		Symbol:          binanceOrder.Symbol,
		Side:            req.Side,
		Type:            req.Type,
		Quantity:        req.Quantity,
		Price:           req.Price,
		FilledQty:       executedQty,
		AvgFillPrice:    avgFillPrice,
		Status:          status,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (b *BinanceExchange) updateOrderFromBinance(order *Order, binanceOrder *binance.Order) {
	// Parse values
	executedQty, _ := strconv.ParseFloat(binanceOrder.ExecutedQuantity, 64)
	cummulativeQuoteQty, _ := strconv.ParseFloat(binanceOrder.CummulativeQuoteQuantity, 64)

	// Calculate average fill price
	var avgFillPrice float64
	if executedQty > 0 {
		avgFillPrice = cummulativeQuoteQty / executedQty
	}

	// Update order fields
	order.FilledQty = executedQty
	order.AvgFillPrice = avgFillPrice
	order.UpdatedAt = time.Now()

	// Map status
	switch binanceOrder.Status {
	case binance.OrderStatusTypeNew:
		order.Status = OrderStatusOpen
	case binance.OrderStatusTypePartiallyFilled:
		order.Status = OrderStatusOpen
	case binance.OrderStatusTypeFilled:
		order.Status = OrderStatusFilled
		now := time.Now()
		order.FilledAt = &now
	case binance.OrderStatusTypeCanceled:
		order.Status = OrderStatusCancelled
	case binance.OrderStatusTypeRejected:
		order.Status = OrderStatusRejected
	}
}

func (b *BinanceExchange) convertToDBOrder(order *Order) *db.Order {
	orderID, _ := uuid.Parse(order.ID)

	var price *float64
	if order.Price > 0 {
		price = &order.Price
	}

	exchangeName := "BINANCE"
	if b.testnet {
		exchangeName = "BINANCE_TESTNET"
	}

	return &db.Order{
		ID:                    orderID,
		SessionID:             b.currentSessionID,
		PositionID:            nil,
		ExchangeOrderID:       &order.ExchangeOrderID,
		Symbol:                order.Symbol,
		Exchange:              exchangeName,
		Side:                  db.ConvertOrderSide(string(order.Side)),
		Type:                  db.ConvertOrderType(string(order.Type)),
		Status:                db.ConvertOrderStatus(string(order.Status)),
		Price:                 price,
		StopPrice:             nil,
		Quantity:              order.Quantity,
		ExecutedQuantity:      order.FilledQty,
		ExecutedQuoteQuantity: order.FilledQty * order.AvgFillPrice,
		TimeInForce:           nil,
		PlacedAt:              order.CreatedAt,
		FilledAt:              order.FilledAt,
		CanceledAt:            nil,
		ErrorMessage:          nil,
		Metadata:              nil,
		CreatedAt:             order.CreatedAt,
		UpdatedAt:             order.UpdatedAt,
	}
}

// WebSocket Methods (T147)

// StartUserDataStream starts the WebSocket connection to receive real-time updates
func (b *BinanceExchange) StartUserDataStream(ctx context.Context) error {
	b.mu.Lock()
	if b.wsConnected {
		b.mu.Unlock()
		log.Info().Msg("User data stream already connected")
		return nil
	}
	// Set wsConnected immediately to prevent race
	b.wsConnected = true
	// Recreate stop channel for this new stream
	b.wsStopChan = make(chan struct{})
	b.mu.Unlock()

	// Create listen key for user data stream
	listenKey, err := b.client.NewStartUserStreamService().Do(ctx)
	if err != nil {
		// Revert wsConnected on error
		b.mu.Lock()
		b.wsConnected = false
		b.mu.Unlock()
		return fmt.Errorf("failed to create listen key: %w", err)
	}

	b.mu.Lock()
	b.listenKey = listenKey
	b.mu.Unlock()

	log.Info().
		Str("listen_key", listenKey[:10]+"...").
		Msg("User data stream listen key created")

	// Start WebSocket handler
	go b.runUserDataStream(ctx, listenKey)

	// Start listen key keep-alive goroutine
	go b.keepAliveListenKey(ctx)

	return nil
}

// StopUserDataStream stops the WebSocket connection
func (b *BinanceExchange) StopUserDataStream(ctx context.Context) error {
	b.mu.Lock()
	if !b.wsConnected {
		b.mu.Unlock()
		return nil
	}

	listenKey := b.listenKey
	b.wsConnected = false
	b.mu.Unlock()

	// Signal stop
	close(b.wsStopChan)

	// Close listen key
	if listenKey != "" {
		err := b.client.NewCloseUserStreamService().ListenKey(listenKey).Do(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to close listen key")
		}
	}

	log.Info().Msg("User data stream stopped")
	return nil
}

// runUserDataStream handles the WebSocket connection
func (b *BinanceExchange) runUserDataStream(ctx context.Context, listenKey string) {
	defer func() {
		b.mu.Lock()
		b.wsConnected = false
		b.mu.Unlock()
	}()

	// Create WebSocket handler
	wsHandler := func(event *binance.WsUserDataEvent) {
		b.handleUserDataEvent(event)
	}

	errHandler := func(err error) {
		log.Error().Err(err).Msg("WebSocket error")

		// Send alert for WebSocket errors
		alerts.AlertConnectionError(context.Background(), "Binance WebSocket", err)

		select {
		case b.wsErrChan <- err:
		default:
			// Channel full, drop error
		}
	}

	// Start WebSocket
	doneC, stopC, err := binance.WsUserDataServe(listenKey, wsHandler, errHandler)
	if err != nil {
		log.Error().Err(err).Msg("Failed to start user data WebSocket")

		// Send alert for WebSocket startup failure
		alerts.AlertConnectionError(ctx, "Binance WebSocket", err)

		return
	}

	log.Info().Msg("User data WebSocket connected")

	// Wait for stop signal or context cancellation
	select {
	case <-b.wsStopChan:
		log.Info().Msg("Stop signal received, closing WebSocket")
		stopC <- struct{}{}
	case <-ctx.Done():
		log.Info().Msg("Context cancelled, closing WebSocket")
		stopC <- struct{}{}
	case <-doneC:
		log.Info().Msg("WebSocket connection closed")
	}
}

// handleUserDataEvent processes user data events from WebSocket
func (b *BinanceExchange) handleUserDataEvent(event *binance.WsUserDataEvent) {
	switch event.Event {
	case binance.UserDataEventTypeOutboundAccountPosition:
		// Account balance update
		log.Debug().
			Int("balance_count", len(event.AccountUpdate.WsAccountUpdates)).
			Msg("Account position update received")

	case binance.UserDataEventTypeExecutionReport:
		// Order update
		b.handleOrderUpdate(event)

	case binance.UserDataEventTypeBalanceUpdate:
		// Balance update
		log.Debug().
			Str("asset", event.BalanceUpdate.Asset).
			Str("change", event.BalanceUpdate.Change).
			Msg("Balance update received")

	default:
		log.Debug().
			Str("event_type", string(event.Event)).
			Msg("Unknown user data event received")
	}
}

// handleOrderUpdate processes order execution reports
func (b *BinanceExchange) handleOrderUpdate(event *binance.WsUserDataEvent) {
	orderUpdate := event.OrderUpdate

	exchangeOrderID := strconv.FormatInt(orderUpdate.Id, 10)

	log.Info().
		Str("exchange_order_id", exchangeOrderID).
		Str("symbol", orderUpdate.Symbol).
		Str("side", orderUpdate.Side).
		Str("status", orderUpdate.Status).
		Str("filled_volume", orderUpdate.FilledVolume).
		Msg("Order update received via WebSocket")

	b.mu.Lock()
	defer b.mu.Unlock()

	// Look up internal order ID from exchange order ID
	internalID, mapped := b.exchangeOrderToInternal[exchangeOrderID]
	if !mapped {
		// Order not found in mapping - might be from a different session
		log.Warn().
			Str("exchange_order_id", exchangeOrderID).
			Msg("Received order update for unknown exchange order ID")
		return
	}

	// Update or create order
	order, exists := b.orders[internalID]
	if !exists {
		// Create new order from WebSocket update
		executedQty, _ := strconv.ParseFloat(orderUpdate.FilledVolume, 64)
		filledQuoteVolume, _ := strconv.ParseFloat(orderUpdate.FilledQuoteVolume, 64)
		qty, _ := strconv.ParseFloat(orderUpdate.Volume, 64)
		price, _ := strconv.ParseFloat(orderUpdate.Price, 64)

		var avgFillPrice float64
		if executedQty > 0 {
			avgFillPrice = filledQuoteVolume / executedQty
		}

		var orderSide OrderSide
		if orderUpdate.Side == string(binance.SideTypeBuy) {
			orderSide = OrderSideBuy
		} else {
			orderSide = OrderSideSell
		}

		var orderType OrderType
		if orderUpdate.Type == string(binance.OrderTypeMarket) {
			orderType = OrderTypeMarket
		} else {
			orderType = OrderTypeLimit
		}

		order = &Order{
			ID:              internalID,
			ExchangeOrderID: exchangeOrderID,
			Symbol:          orderUpdate.Symbol,
			Side:            orderSide,
			Type:            orderType,
			Quantity:        qty,
			Price:           price,
			FilledQty:       executedQty,
			AvgFillPrice:    avgFillPrice,
			CreatedAt:       time.Unix(0, orderUpdate.CreateTime*int64(time.Millisecond)),
			UpdatedAt:       time.Unix(0, orderUpdate.TransactionTime*int64(time.Millisecond)),
		}

		b.orders[internalID] = order
	}

	// Update order status
	executedQty, _ := strconv.ParseFloat(orderUpdate.FilledVolume, 64)
	filledQuoteVolume, _ := strconv.ParseFloat(orderUpdate.FilledQuoteVolume, 64)

	order.FilledQty = executedQty
	if executedQty > 0 {
		order.AvgFillPrice = filledQuoteVolume / executedQty
	}
	order.UpdatedAt = time.Unix(0, orderUpdate.TransactionTime*int64(time.Millisecond))

	switch orderUpdate.Status {
	case string(binance.OrderStatusTypeFilled):
		order.Status = OrderStatusFilled
		now := time.Now()
		order.FilledAt = &now

		// Create fills and update positions
		b.handleOrderFilled(order, &orderUpdate)

	case string(binance.OrderStatusTypePartiallyFilled):
		order.Status = OrderStatusOpen

	case string(binance.OrderStatusTypeCanceled):
		order.Status = OrderStatusCancelled

	case string(binance.OrderStatusTypeRejected):
		order.Status = OrderStatusRejected

	case string(binance.OrderStatusTypeNew):
		order.Status = OrderStatusOpen
	}

	// Update database
	if b.db != nil {
		dbOrder := b.convertToDBOrder(order)
		ctx := context.Background()
		err := b.db.UpdateOrderStatus(
			ctx,
			dbOrder.ID,
			dbOrder.Status,
			dbOrder.ExecutedQuantity,
			dbOrder.ExecutedQuoteQuantity,
			dbOrder.FilledAt,
			dbOrder.CanceledAt,
			dbOrder.ErrorMessage,
		)
		if err != nil {
			log.Error().
				Err(err).
				Str("order_id", order.ID).
				Msg("Failed to update order status in database")
		}
	}
}

// handleOrderFilled processes filled orders and updates positions
func (b *BinanceExchange) handleOrderFilled(order *Order, orderUpdate *binance.WsOrderUpdate) {
	// Create fill records
	lastQty, _ := strconv.ParseFloat(orderUpdate.LatestVolume, 64)
	lastPrice, _ := strconv.ParseFloat(orderUpdate.LatestPrice, 64)

	if lastQty > 0 && lastPrice > 0 {
		fill := Fill{
			OrderID:   order.ID,
			Quantity:  lastQty,
			Price:     lastPrice,
			Timestamp: time.Unix(0, orderUpdate.TransactionTime*int64(time.Millisecond)),
		}

		b.fills[order.ID] = append(b.fills[order.ID], fill)

		// Update positions through PositionManager
		if b.positionMgr != nil && b.currentSessionID != nil {
			ctx := context.Background()
			err := b.positionMgr.OnOrderFilled(ctx, order, []Fill{fill})
			if err != nil {
				log.Error().
					Err(err).
					Str("order_id", order.ID).
					Msg("Failed to update position after order fill")
			}
		}
	}
}

// keepAliveListenKey keeps the listen key alive by pinging every 30 minutes
func (b *BinanceExchange) keepAliveListenKey(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.mu.RLock()
			listenKey := b.listenKey
			connected := b.wsConnected
			b.mu.RUnlock()

			if !connected {
				log.Debug().Msg("WebSocket disconnected, stopping keep-alive")
				return
			}

			err := b.client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to keep alive listen key")
			} else {
				log.Debug().Msg("Listen key kept alive")
			}

		case <-b.wsStopChan:
			log.Debug().Msg("Stop signal received, stopping keep-alive")
			return

		case <-ctx.Done():
			log.Debug().Msg("Context cancelled, stopping keep-alive")
			return
		}
	}
}
