package exchange

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket"
)

// PolymarketExchange implements the Exchange interface for Polymarket CLOB trading.
// In paper mode, orders are simulated via the DB paper engine.
// In live mode, orders are sent to the Polymarket CLOB.
type PolymarketExchange struct {
	client    *polymarket.Client
	db        *db.DB
	paperMode bool

	mu               sync.RWMutex
	orders           map[string]*Order // internal UUID -> Order
	fills            map[string][]Fill
	marketPrices     map[string]float64
	currentSessionID *uuid.UUID

	// safetyCheck is an optional function called before order submission.
	// It receives (ctx, PlaceOrderRequest, orderValue) and returns an error to reject.
	safetyCheck func(ctx context.Context, req PlaceOrderRequest, orderValue float64) error
}

// NewPolymarketExchange creates a new Polymarket exchange adapter.
// In paper mode, the polymarket client is still created (for market data) but orders
// are simulated locally.
func NewPolymarketExchange(privateKey string, paperMode bool, database *db.DB, opts ...polymarket.ClientOption) (*PolymarketExchange, error) {
	client, err := polymarket.NewClient(privateKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("create polymarket client: %w", err)
	}

	mode := "LIVE"
	if paperMode {
		mode = "PAPER"
	}
	log.Info().Str("mode", mode).Msg("Polymarket exchange initialized")

	return &PolymarketExchange{
		client:       client,
		db:           database,
		paperMode:    paperMode,
		orders:       make(map[string]*Order),
		fills:        make(map[string][]Fill),
		marketPrices: make(map[string]float64),
	}, nil
}

// SetDB sets or replaces the database connection used for order persistence.
func (p *PolymarketExchange) SetDB(database *db.DB) {
	p.db = database
}

// SetSafetyCheck sets a safety check function called before order submission.
// This avoids import cycles between exchange and safety packages.
func (p *PolymarketExchange) SetSafetyCheck(fn func(ctx context.Context, req PlaceOrderRequest, orderValue float64) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.safetyCheck = fn
}

// Close cleans up the underlying Polymarket client resources.
func (p *PolymarketExchange) Close() {
	if p.client != nil {
		p.client.Close()
	}
}

// Client returns the underlying Polymarket CLOB client for direct access
// to Polymarket-specific functionality (order book, midpoint, etc.).
func (p *PolymarketExchange) Client() *polymarket.Client {
	return p.client
}

// PlaceOrder places an order. For Polymarket:
// - Symbol is the token ID
// - Side maps to polymarket BUY/SELL
// - Only limit orders are supported (Polymarket CLOB is limit-order only)
// - Price is 0.0-1.0 (probability)
// - Quantity is the number of shares (in USDC terms for sizing)
func (p *PolymarketExchange) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate
	if req.Symbol == "" {
		return nil, fmt.Errorf("symbol (token ID) is required")
	}
	if req.Price <= 0 || req.Price >= 1 {
		return nil, fmt.Errorf("price must be between 0 and 1 (exclusive) for prediction markets")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	// Safety check
	if p.safetyCheck != nil {
		orderValue := req.Price * req.Quantity
		if err := p.safetyCheck(ctx, req, orderValue); err != nil {
			return &PlaceOrderResponse{
				Status:  OrderStatusRejected,
				Message: err.Error(),
			}, nil
		}
	}

	// Map side
	var pmSide polymarket.Side
	if req.Side == OrderSideBuy {
		pmSide = polymarket.Buy
	} else {
		pmSide = polymarket.Sell
	}

	now := time.Now()
	internalID := uuid.New().String()

	if p.paperMode {
		// Paper mode: simulate fill at requested price
		order := &Order{
			ID:              internalID,
			ExchangeOrderID: "paper-" + internalID[:8],
			Symbol:          req.Symbol,
			Side:            req.Side,
			Type:            OrderTypeLimit,
			Quantity:        req.Quantity,
			Price:           req.Price,
			FilledQty:       req.Quantity,
			AvgFillPrice:    req.Price,
			Status:          OrderStatusFilled,
			CreatedAt:       now,
			UpdatedAt:       now,
			FilledAt:        &now,
		}
		p.orders[internalID] = order
		p.fills[internalID] = []Fill{{
			OrderID:   internalID,
			Quantity:  req.Quantity,
			Price:     req.Price,
			Timestamp: now,
		}}

		// Persist to DB
		if p.db != nil {
			dbOrder := p.convertToDBOrder(order)
			if err := p.db.InsertOrder(ctx, dbOrder); err != nil {
				log.Error().Err(err).Str("order_id", internalID).Msg("Failed to persist paper order to database")
			}
		}

		log.Info().
			Str("order_id", internalID).
			Str("token_id", req.Symbol).
			Str("side", string(req.Side)).
			Float64("price", req.Price).
			Float64("quantity", req.Quantity).
			Msg("Paper order filled on Polymarket")

		return &PlaceOrderResponse{
			OrderID: internalID,
			Status:  OrderStatusFilled,
			Message: "Paper order filled",
		}, nil
	}

	// Live mode: submit to Polymarket CLOB
	resp, err := p.client.CreateOrder(ctx, req.Symbol, pmSide, req.Price, req.Quantity)
	if err != nil {
		log.Error().Err(err).Str("token_id", req.Symbol).Msg("Failed to place order on Polymarket CLOB")
		return &PlaceOrderResponse{
			Status:  OrderStatusRejected,
			Message: err.Error(),
		}, fmt.Errorf("polymarket order failed: %w", err)
	}

	if !resp.Success {
		return &PlaceOrderResponse{
			Status:  OrderStatusRejected,
			Message: resp.ErrorMsg,
		}, fmt.Errorf("polymarket order rejected: %s", resp.ErrorMsg)
	}

	order := &Order{
		ID:              internalID,
		ExchangeOrderID: resp.OrderID,
		Symbol:          req.Symbol,
		Side:            req.Side,
		Type:            OrderTypeLimit,
		Quantity:        req.Quantity,
		Price:           req.Price,
		Status:          OrderStatusOpen,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	p.orders[internalID] = order

	// Persist to DB
	if p.db != nil {
		dbOrder := p.convertToDBOrder(order)
		if err := p.db.InsertOrder(ctx, dbOrder); err != nil {
			log.Error().Err(err).Str("order_id", internalID).Msg("Failed to persist order to database")
		}
	}

	log.Info().
		Str("order_id", internalID).
		Str("exchange_order_id", resp.OrderID).
		Str("token_id", req.Symbol).
		Str("side", string(req.Side)).
		Float64("price", req.Price).
		Float64("quantity", req.Quantity).
		Msg("Order placed on Polymarket CLOB")

	return &PlaceOrderResponse{
		OrderID: internalID,
		Status:  OrderStatusOpen,
		Message: "Order placed on Polymarket",
	}, nil
}

// CancelOrder cancels an order on Polymarket.
func (p *PolymarketExchange) CancelOrder(ctx context.Context, orderID string) (*Order, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	order, exists := p.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status != OrderStatusOpen && order.Status != OrderStatusPending {
		return nil, fmt.Errorf("cannot cancel order in status: %s", order.Status)
	}

	if !p.paperMode {
		if err := p.client.CancelOrder(ctx, order.ExchangeOrderID); err != nil {
			return nil, fmt.Errorf("failed to cancel on Polymarket: %w", err)
		}
	}

	order.Status = OrderStatusCancelled
	order.UpdatedAt = time.Now()

	log.Info().Str("order_id", orderID).Msg("Order cancelled on Polymarket")
	return order, nil
}

// GetOrder retrieves order details.
func (p *PolymarketExchange) GetOrder(_ context.Context, orderID string) (*Order, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	order, exists := p.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}
	return order, nil
}

// GetOrderFills retrieves fills for an order.
func (p *PolymarketExchange) GetOrderFills(_ context.Context, orderID string) ([]Fill, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fills, exists := p.fills[orderID]
	if !exists {
		return []Fill{}, nil
	}
	return fills, nil
}

// SetMarketPrice sets a market price for a token ID (used in paper mode).
func (p *PolymarketExchange) SetMarketPrice(symbol string, price float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.marketPrices[symbol] = price
}

// GetMarketPrice returns the cached market price or fetches midpoint from CLOB.
func (p *PolymarketExchange) GetMarketPrice(symbol string) float64 {
	p.mu.RLock()
	price, exists := p.marketPrices[symbol]
	p.mu.RUnlock()

	if exists {
		return price
	}

	// Try fetching midpoint from CLOB
	mid, err := p.client.GetMidpoint(context.Background(), symbol)
	if err != nil {
		return 0
	}
	f, _ := strconv.ParseFloat(mid, 64)
	return f
}

// SetSession sets the current trading session.
func (p *PolymarketExchange) SetSession(sessionID *uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentSessionID = sessionID
}

// GetSession returns the current trading session ID.
func (p *PolymarketExchange) GetSession() *uuid.UUID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentSessionID
}

// --- Polymarket-specific methods ---

// GetOrderBook returns the order book for a token.
func (p *PolymarketExchange) GetOrderBook(ctx context.Context, tokenID string) (*polymarket.OrderBook, error) {
	return p.client.GetOrderBook(ctx, tokenID)
}

// GetOpenOrders returns open orders from Polymarket CLOB.
func (p *PolymarketExchange) GetOpenOrders(ctx context.Context) ([]polymarket.Order, error) {
	if p.paperMode {
		// Return paper orders that are still open
		p.mu.RLock()
		defer p.mu.RUnlock()
		var orders []polymarket.Order
		for _, o := range p.orders {
			if o.Status == OrderStatusOpen {
				orders = append(orders, polymarket.Order{
					ID:      o.ExchangeOrderID,
					AssetID: o.Symbol,
					Side:    string(o.Side),
					Price:   fmt.Sprintf("%.4f", o.Price),
					Status:  "LIVE",
				})
			}
		}
		return orders, nil
	}
	return p.client.GetOpenOrders(ctx)
}

// GetMidpoint returns the midpoint price for a token.
func (p *PolymarketExchange) GetMidpoint(ctx context.Context, tokenID string) (string, error) {
	return p.client.GetMidpoint(ctx, tokenID)
}

// convertToDBOrder converts an internal order to a DB order.
func (p *PolymarketExchange) convertToDBOrder(order *Order) *db.Order {
	orderID, _ := uuid.Parse(order.ID)

	var price *float64
	if order.Price > 0 {
		price = &order.Price
	}

	exchangeName := "POLYMARKET"
	if p.paperMode {
		exchangeName = "POLYMARKET_PAPER"
	}

	return &db.Order{
		ID:                    orderID,
		SessionID:             p.currentSessionID,
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
