package exchange

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	binance "github.com/adshao/go-binance/v2"
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
	orders map[string]*Order
	fills  map[string][]Fill

	// Session tracking
	currentSessionID *uuid.UUID

	// Configuration
	testnet      bool
	retryConfig  RetryConfig
	alertManager *AlertManager
}

// BinanceConfig contains configuration for Binance exchange
type BinanceConfig struct {
	APIKey      string
	SecretKey   string
	Testnet     bool
	RetryConfig RetryConfig
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

	return &BinanceExchange{
		client:       client,
		db:           database,
		orders:       make(map[string]*Order),
		fills:        make(map[string][]Fill),
		testnet:      config.Testnet,
		retryConfig:  config.RetryConfig,
		alertManager: NewAlertManager(),
	}, nil
}

// PlaceOrder places a new order on Binance
func (b *BinanceExchange) PlaceOrder(req PlaceOrderRequest) (*PlaceOrderResponse, error) {
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

	ctx := context.Background()
	side := binance.SideTypeBuy
	if req.Side == OrderSideSell {
		side = binance.SideTypeSell
	}

	// Wrap API call in retry logic
	err = WithRetry(ctx, b.retryConfig, func() error {
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
	})

	if err != nil {
		// Send alert for order placement failure
		ctx := context.Background()
		alert := AlertOrderPlacementFailed(err, req.Symbol, req.Side, req.Quantity, req.Type)
		b.alertManager.SendAlert(ctx, alert)

		return &PlaceOrderResponse{
			Status:  OrderStatusRejected,
			Message: err.Error(),
		}, fmt.Errorf("failed to place order: %w", err)
	}

	// Convert Binance order to internal Order struct
	order := b.convertBinanceOrder(binanceOrder, req)

	// Store order
	b.orders[order.ID] = order

	// Persist to database
	if b.db != nil {
		dbOrder := b.convertToDBOrder(order)
		if err := b.db.InsertOrder(context.Background(), dbOrder); err != nil {
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
func (b *BinanceExchange) CancelOrder(orderID string) (*Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	order, exists := b.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status != OrderStatusOpen && order.Status != OrderStatusPending {
		return nil, fmt.Errorf("cannot cancel order in status: %s", order.Status)
	}

	// Parse Binance order ID from our internal ID
	// In production, would store mapping between internal ID and exchange ID
	binanceOrderID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID format: %w", err)
	}

	// Cancel order on Binance with retry logic
	ctx := context.Background()
	err = WithRetry(ctx, b.retryConfig, func() error {
		_, err := b.client.NewCancelOrderService().
			Symbol(order.Symbol).
			OrderID(binanceOrderID).
			Do(ctx)
		return err
	})

	if err != nil {
		// Send alert for order cancellation failure
		ctx := context.Background()
		alert := AlertOrderCancellationFailed(err, orderID)
		b.alertManager.SendAlert(ctx, alert)
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
			context.Background(),
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
func (b *BinanceExchange) GetOrder(orderID string) (*Order, error) {
	b.mu.RLock()
	order, exists := b.orders[orderID]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	// Query Binance for latest order status with retry logic
	binanceOrderID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return order, nil // Return cached order if ID parsing fails
	}

	ctx := context.Background()
	var binanceOrder *binance.Order
	err = WithRetry(ctx, b.retryConfig, func() error {
		var retryErr error
		binanceOrder, retryErr = b.client.NewGetOrderService().
			Symbol(order.Symbol).
			OrderID(binanceOrderID).
			Do(ctx)
		return retryErr
	})

	if err != nil {
		// Send alert for order query failure
		ctx := context.Background()
		alert := AlertOrderQueryFailed(err, orderID)
		b.alertManager.SendAlert(ctx, alert)
		return order, nil
	}

	// Update order with latest data
	b.mu.Lock()
	b.updateOrderFromBinance(order, binanceOrder)
	b.mu.Unlock()

	return order, nil
}

// GetOrderFills retrieves all fills for an order
func (b *BinanceExchange) GetOrderFills(orderID string) ([]Fill, error) {
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

	// Use Binance order ID as our internal ID
	orderID := strconv.FormatInt(binanceOrder.OrderID, 10)

	return &Order{
		ID:           orderID,
		Symbol:       binanceOrder.Symbol,
		Side:         req.Side,
		Type:         req.Type,
		Quantity:     req.Quantity,
		Price:        req.Price,
		FilledQty:    executedQty,
		AvgFillPrice: avgFillPrice,
		Status:       status,
		CreatedAt:    now,
		UpdatedAt:    now,
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
		ExchangeOrderID:       &order.ID,
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
