package exchange

import (
	"context"

	"github.com/google/uuid"
)

// Exchange defines the interface for all exchange implementations
// Both MockExchange (paper trading) and BinanceExchange (live trading) implement this interface
type Exchange interface {
	// PlaceOrder places a new order
	PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*PlaceOrderResponse, error)

	// CancelOrder cancels an existing order
	CancelOrder(ctx context.Context, orderID string) (*Order, error)

	// GetOrder retrieves order details
	GetOrder(ctx context.Context, orderID string) (*Order, error)

	// GetOrderFills retrieves all fills for an order
	GetOrderFills(ctx context.Context, orderID string) ([]Fill, error)

	// SetMarketPrice sets the current market price for a symbol (mock exchange only)
	SetMarketPrice(symbol string, price float64)

	// SetSession sets the current trading session
	SetSession(sessionID *uuid.UUID)

	// GetSession returns the current trading session ID
	GetSession() *uuid.UUID
}
