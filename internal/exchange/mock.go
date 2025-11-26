package exchange

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// MockExchange simulates a trading exchange for paper trading
type MockExchange struct {
	orders map[string]*Order
	fills  map[string][]Fill
	mu     sync.RWMutex

	// Mock market data for order fills
	marketPrices map[string]float64

	// Market simulation parameters
	baseSlippage float64 // Base slippage percentage
	marketImpact float64 // Market impact per unit of quantity
	maxSlippage  float64 // Maximum slippage percentage
	makerFee     float64 // Maker fee percentage
	takerFee     float64 // Taker fee percentage

	// Database for persistence
	db *db.DB

	// Session tracking
	currentSessionID *uuid.UUID
}

// NewMockExchange creates a new mock exchange with default fee configuration
func NewMockExchange(database *db.DB) *MockExchange {
	// Use default Binance-like fees
	defaultFees := config.FeeConfig{
		Maker:        0.001,  // 0.1%
		Taker:        0.001,  // 0.1%
		BaseSlippage: 0.0005, // 0.05%
		MarketImpact: 0.0001, // 0.01%
		MaxSlippage:  0.003,  // 0.3%
	}
	return NewMockExchangeWithFees(database, defaultFees)
}

// NewMockExchangeWithFees creates a new mock exchange with custom fee configuration
func NewMockExchangeWithFees(database *db.DB, fees config.FeeConfig) *MockExchange {
	log.Info().
		Float64("maker_fee", fees.Maker).
		Float64("taker_fee", fees.Taker).
		Float64("base_slippage", fees.BaseSlippage).
		Msg("Mock exchange initialized (paper trading mode)")

	return &MockExchange{
		orders:       make(map[string]*Order),
		fills:        make(map[string][]Fill),
		marketPrices: make(map[string]float64),

		// Configurable market simulation parameters
		baseSlippage: fees.BaseSlippage,
		marketImpact: fees.MarketImpact,
		maxSlippage:  fees.MaxSlippage,
		makerFee:     fees.Maker,
		takerFee:     fees.Taker,

		db: database,
	}
}

// PlaceOrder places a new order in the mock exchange
func (m *MockExchange) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate request
	if err := m.validateOrder(req); err != nil {
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

	// Create order
	now := time.Now()
	order := &Order{
		ID:        uuid.New().String(),
		Symbol:    req.Symbol,
		Side:      req.Side,
		Type:      req.Type,
		Quantity:  req.Quantity,
		Price:     req.Price,
		FilledQty: 0,
		Status:    OrderStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Store order
	m.orders[order.ID] = order

	// Persist to database if available
	if m.db != nil {
		dbOrder := m.convertToDBOrder(order)
		if err := m.db.InsertOrder(ctx, dbOrder); err != nil {
			log.Error().
				Err(err).
				Str("order_id", order.ID).
				Msg("Failed to persist order to database")
			// Continue even if database insert fails (paper trading mode)
		}
	}

	log.Info().
		Str("order_id", order.ID).
		Str("symbol", order.Symbol).
		Str("side", string(order.Side)).
		Str("type", string(order.Type)).
		Float64("quantity", order.Quantity).
		Msg("Order placed")

	// Simulate immediate fill for market orders
	if req.Type == OrderTypeMarket {
		m.simulateMarketFill(ctx, order)
	} else {
		order.Status = OrderStatusOpen
		order.UpdatedAt = time.Now()

		// Update status in database
		if m.db != nil {
			m.updateOrderStatusInDB(ctx, order)
		}
	}

	return &PlaceOrderResponse{
		OrderID: order.ID,
		Status:  order.Status,
		Message: "Order placed successfully",
	}, nil
}

// CancelOrder cancels an open order
func (m *MockExchange) CancelOrder(ctx context.Context, orderID string) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status != OrderStatusOpen && order.Status != OrderStatusPending {
		return nil, fmt.Errorf("cannot cancel order in status: %s", order.Status)
	}

	order.Status = OrderStatusCancelled
	cancelledAt := time.Now()
	order.UpdatedAt = cancelledAt

	// Update in database
	if m.db != nil {
		orderUUID, _ := uuid.Parse(orderID)
		status := db.ConvertOrderStatus(string(order.Status))
		err := m.db.UpdateOrderStatus(
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
		Msg("Order cancelled")

	return order, nil
}

// GetOrder retrieves order details
func (m *MockExchange) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	order, exists := m.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

// GetOrderFills retrieves all fills for an order
func (m *MockExchange) GetOrderFills(ctx context.Context, orderID string) ([]Fill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fills, exists := m.fills[orderID]
	if !exists {
		return []Fill{}, nil
	}

	return fills, nil
}

// SetMarketPrice sets the current market price for a symbol (for testing)
func (m *MockExchange) SetMarketPrice(symbol string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.marketPrices[symbol] = price
}

// validateOrder validates order parameters
func (m *MockExchange) validateOrder(req PlaceOrderRequest) error {
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

// simulateMarketFill simulates realistic fill for market orders with slippage and market impact
func (m *MockExchange) simulateMarketFill(ctx context.Context, order *Order) {
	now := time.Now()

	// Use stored market price or simulate one
	midPrice, exists := m.marketPrices[order.Symbol]
	if !exists {
		// Simulate a price (in production, would come from market data)
		midPrice = 50000.0 // Default BTC price for simulation
	}

	// Calculate realistic slippage based on order size and market conditions
	slippage := m.calculateSlippage(order.Quantity, midPrice)

	// Apply slippage based on order side
	var fillPrice float64
	if order.Side == OrderSideBuy {
		// Buying means paying the ask price (higher than mid)
		fillPrice = midPrice * (1 + slippage)
	} else {
		// Selling means receiving the bid price (lower than mid)
		fillPrice = midPrice * (1 - slippage)
	}

	// Simulate partial fills for large orders (more realistic)
	fills := m.simulatePartialFills(order, fillPrice, now)

	// Calculate average fill price
	var totalValue float64
	var totalQty float64
	for _, fill := range fills {
		totalValue += fill.Price * fill.Quantity
		totalQty += fill.Quantity
	}
	avgPrice := totalValue / totalQty

	// Update order
	order.FilledQty = order.Quantity
	order.AvgFillPrice = avgPrice
	order.Status = OrderStatusFilled
	order.UpdatedAt = now
	order.FilledAt = &now

	// Store fills
	m.fills[order.ID] = fills

	// Persist fills to database
	if m.db != nil {
		for _, fill := range fills {
			m.persistTradeInDB(ctx, order.ID, fill)
		}
		// Update order status in database
		m.updateOrderStatusInDB(ctx, order)
	}

	log.Info().
		Str("order_id", order.ID).
		Float64("quantity", order.Quantity).
		Float64("avg_price", avgPrice).
		Float64("slippage_pct", slippage*100).
		Int("num_fills", len(fills)).
		Msg("Order filled")
}

// calculateSlippage calculates realistic slippage based on order size
func (m *MockExchange) calculateSlippage(quantity, price float64) float64 {
	// Normalize quantity (assuming BTC as baseline)
	// For simplicity, use quantity * price as a proxy for order size in USD
	orderSize := quantity * price

	// Base slippage + market impact based on order size
	// Larger orders have more market impact
	normalizedSize := orderSize / 1000000.0 // Normalize to millions of USD
	marketImpact := m.marketImpact * normalizedSize

	// Total slippage = base + market impact, capped at max
	totalSlippage := m.baseSlippage + marketImpact
	if totalSlippage > m.maxSlippage {
		totalSlippage = m.maxSlippage
	}

	return totalSlippage
}

// simulatePartialFills simulates multiple partial fills for large orders
func (m *MockExchange) simulatePartialFills(order *Order, basePrice float64, startTime time.Time) []Fill {
	// For small orders, fill in one go
	if order.Quantity < 1.0 {
		return []Fill{
			{
				OrderID:   order.ID,
				Quantity:  order.Quantity,
				Price:     basePrice,
				Timestamp: startTime,
			},
		}
	}

	// For larger orders, simulate partial fills with slight price variation
	fills := []Fill{}
	remainingQty := order.Quantity
	fillTime := startTime
	fillCount := 0
	maxFills := 5 // Maximum number of partial fills

	for remainingQty > 0 && fillCount < maxFills {
		// Each fill is a random portion of the remaining quantity
		fillQty := remainingQty
		if fillCount < maxFills-1 {
			// Fill 20-40% of remaining quantity
			portion := 0.2 + (0.2 * float64(fillCount) / float64(maxFills))
			fillQty = remainingQty * portion
			if fillQty < 0.01 {
				fillQty = remainingQty // Fill the rest if too small
			}
		}

		// Slight price variation for each partial fill (simulate order book depth)
		priceVariation := 0.0001 * float64(fillCount) // 0.01% per fill
		var fillPrice float64
		if order.Side == OrderSideBuy {
			fillPrice = basePrice * (1 + priceVariation)
		} else {
			fillPrice = basePrice * (1 - priceVariation)
		}

		fills = append(fills, Fill{
			OrderID:   order.ID,
			Quantity:  fillQty,
			Price:     fillPrice,
			Timestamp: fillTime,
		})

		remainingQty -= fillQty
		fillCount++
		// Simulate small time delay between fills (microseconds)
		fillTime = fillTime.Add(time.Microsecond * time.Duration(100+fillCount*50))
	}

	return fills
}

// convertToDBOrder converts application Order to database Order
func (m *MockExchange) convertToDBOrder(order *Order) *db.Order {
	orderID, _ := uuid.Parse(order.ID)

	var price *float64
	if order.Price > 0 {
		price = &order.Price
	}

	var exchangeOrderID *string
	if order.ID != "" {
		exchangeOrderID = &order.ID
	}

	return &db.Order{
		ID:                    orderID,
		SessionID:             m.currentSessionID, // Set from current session
		PositionID:            nil,                // Will be set by position tracking layer
		ExchangeOrderID:       exchangeOrderID,
		Symbol:                order.Symbol,
		Exchange:              "PAPER", // Paper trading exchange
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

// updateOrderStatusInDB updates order status in database
func (m *MockExchange) updateOrderStatusInDB(ctx context.Context, order *Order) {
	if m.db == nil {
		return
	}

	orderID, _ := uuid.Parse(order.ID)
	status := db.ConvertOrderStatus(string(order.Status))
	quoteQty := order.FilledQty * order.AvgFillPrice

	err := m.db.UpdateOrderStatus(
		ctx,
		orderID,
		status,
		order.FilledQty,
		quoteQty,
		order.FilledAt,
		nil, // canceledAt
		nil, // errorMessage
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("order_id", order.ID).
			Msg("Failed to update order status in database")
	}
}

// persistTradeInDB persists a trade (fill) to database
func (m *MockExchange) persistTradeInDB(ctx context.Context, orderID string, fill Fill) {
	if m.db == nil {
		return
	}

	orderUUID, _ := uuid.Parse(orderID)
	order := m.orders[orderID]

	// Calculate commission based on order type
	// Market orders are always taker, limit orders can be maker or taker
	isMaker := order.Type == OrderTypeLimit
	commission := fill.Price * fill.Quantity * m.takerFee
	if isMaker {
		commission = fill.Price * fill.Quantity * m.makerFee
	}

	dbTrade := &db.Trade{
		ID:              uuid.New(),
		OrderID:         orderUUID,
		ExchangeTradeID: nil,
		Symbol:          order.Symbol,
		Exchange:        "PAPER",
		Side:            db.ConvertOrderSide(string(order.Side)),
		Price:           fill.Price,
		Quantity:        fill.Quantity,
		QuoteQuantity:   fill.Price * fill.Quantity,
		Commission:      commission,
		CommissionAsset: nil,
		ExecutedAt:      fill.Timestamp,
		IsMaker:         isMaker,
		Metadata:        nil,
		CreatedAt:       fill.Timestamp,
	}

	if err := m.db.InsertTrade(ctx, dbTrade); err != nil {
		log.Error().
			Err(err).
			Str("order_id", orderID).
			Msg("Failed to persist trade to database")
	}
}

// SetSession sets the current trading session
func (m *MockExchange) SetSession(sessionID *uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentSessionID = sessionID

	if sessionID != nil {
		log.Info().
			Str("session_id", sessionID.String()).
			Msg("Trading session set for exchange")
	} else {
		log.Info().Msg("Trading session cleared for exchange")
	}
}

// GetSession returns the current trading session ID
func (m *MockExchange) GetSession() *uuid.UUID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.currentSessionID
}
