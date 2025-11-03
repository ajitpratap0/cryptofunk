package exchange

import "time"

// OrderSide represents buy or sell
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// OrderType represents market or limit order
type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

// OrderStatus represents the current state of an order
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusOpen      OrderStatus = "open"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRejected  OrderStatus = "rejected"
)

// Order represents a trading order
type Order struct {
	ID              string      `json:"id"`
	ExchangeOrderID string      `json:"exchange_order_id,omitempty"` // Exchange-specific order ID (e.g., Binance int64 as string)
	Symbol          string      `json:"symbol"`
	Side            OrderSide   `json:"side"`
	Type            OrderType   `json:"type"`
	Quantity        float64     `json:"quantity"`
	Price           float64     `json:"price,omitempty"` // For limit orders
	FilledQty       float64     `json:"filled_qty"`
	AvgFillPrice    float64     `json:"avg_fill_price,omitempty"`
	Status          OrderStatus `json:"status"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	FilledAt        *time.Time  `json:"filled_at,omitempty"`
	RejectReason    string      `json:"reject_reason,omitempty"`
}

// Fill represents a partial or complete order fill
type Fill struct {
	OrderID   string    `json:"order_id"`
	Quantity  float64   `json:"quantity"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// PlaceOrderRequest represents a request to place an order
type PlaceOrderRequest struct {
	Symbol   string    `json:"symbol"`
	Side     OrderSide `json:"side"`
	Type     OrderType `json:"type"`
	Quantity float64   `json:"quantity"`
	Price    float64   `json:"price,omitempty"` // For limit orders
}

// PlaceOrderResponse represents the response after placing an order
type PlaceOrderResponse struct {
	OrderID string      `json:"order_id"`
	Status  OrderStatus `json:"status"`
	Message string      `json:"message,omitempty"`
}
