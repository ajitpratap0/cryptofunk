package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// OrderSide represents buy or sell (database enum)
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType represents order type (database enum)
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
)

// OrderStatus represents order status (database enum)
type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
)

// Order represents a database order record
type Order struct {
	ID                    uuid.UUID
	SessionID             *uuid.UUID
	PositionID            *uuid.UUID
	ExchangeOrderID       *string
	Symbol                string
	Exchange              string
	Side                  OrderSide
	Type                  OrderType
	Status                OrderStatus
	Price                 *float64
	StopPrice             *float64
	Quantity              float64
	ExecutedQuantity      float64
	ExecutedQuoteQuantity float64
	TimeInForce           *string
	PlacedAt              time.Time
	FilledAt              *time.Time
	CanceledAt            *time.Time
	ErrorMessage          *string
	Metadata              map[string]interface{}
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Trade represents a database trade record (fill)
type Trade struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	ExchangeTradeID *string
	Symbol          string
	Exchange        string
	Side            OrderSide
	Price           float64
	Quantity        float64
	QuoteQuantity   float64
	Commission      float64
	CommissionAsset *string
	ExecutedAt      time.Time
	IsMaker         bool
	Metadata        map[string]interface{}
	CreatedAt       time.Time
}

// InsertOrder inserts a new order into the database
func (db *DB) InsertOrder(ctx context.Context, order *Order) error {
	query := `
		INSERT INTO orders (
			id, session_id, position_id, exchange_order_id, symbol, exchange,
			side, type, status, price, stop_price, quantity, executed_quantity,
			executed_quote_quantity, time_in_force, placed_at, filled_at,
			canceled_at, error_message, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22
		)
	`

	_, err := db.pool.Exec(ctx, query,
		order.ID,
		order.SessionID,
		order.PositionID,
		order.ExchangeOrderID,
		order.Symbol,
		order.Exchange,
		order.Side,
		order.Type,
		order.Status,
		order.Price,
		order.StopPrice,
		order.Quantity,
		order.ExecutedQuantity,
		order.ExecutedQuoteQuantity,
		order.TimeInForce,
		order.PlacedAt,
		order.FilledAt,
		order.CanceledAt,
		order.ErrorMessage,
		order.Metadata,
		order.CreatedAt,
		order.UpdatedAt,
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("order_id", order.ID.String()).
			Str("symbol", order.Symbol).
			Msg("Failed to insert order")
		return fmt.Errorf("failed to insert order: %w", err)
	}

	log.Debug().
		Str("order_id", order.ID.String()).
		Str("symbol", order.Symbol).
		Str("status", string(order.Status)).
		Msg("Order inserted into database")

	return nil
}

// UpdateOrderStatus updates an order's status and related fields
func (db *DB) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status OrderStatus, executedQty, executedQuoteQty float64, filledAt, canceledAt *time.Time, errorMsg *string) error {
	query := `
		UPDATE orders
		SET status = $1,
		    executed_quantity = $2,
		    executed_quote_quantity = $3,
		    filled_at = $4,
		    canceled_at = $5,
		    error_message = $6,
		    updated_at = NOW()
		WHERE id = $7
	`

	result, err := db.pool.Exec(ctx, query,
		status,
		executedQty,
		executedQuoteQty,
		filledAt,
		canceledAt,
		errorMsg,
		orderID,
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("order_id", orderID.String()).
			Msg("Failed to update order status")
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found: %s", orderID.String())
	}

	log.Debug().
		Str("order_id", orderID.String()).
		Str("status", string(status)).
		Msg("Order status updated")

	return nil
}

// InsertTrade inserts a new trade (fill) into the database
func (db *DB) InsertTrade(ctx context.Context, trade *Trade) error {
	query := `
		INSERT INTO trades (
			id, order_id, exchange_trade_id, symbol, exchange, side,
			price, quantity, quote_quantity, commission, commission_asset,
			executed_at, is_maker, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err := db.pool.Exec(ctx, query,
		trade.ID,
		trade.OrderID,
		trade.ExchangeTradeID,
		trade.Symbol,
		trade.Exchange,
		trade.Side,
		trade.Price,
		trade.Quantity,
		trade.QuoteQuantity,
		trade.Commission,
		trade.CommissionAsset,
		trade.ExecutedAt,
		trade.IsMaker,
		trade.Metadata,
		trade.CreatedAt,
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("trade_id", trade.ID.String()).
			Str("order_id", trade.OrderID.String()).
			Msg("Failed to insert trade")
		return fmt.Errorf("failed to insert trade: %w", err)
	}

	log.Debug().
		Str("trade_id", trade.ID.String()).
		Str("order_id", trade.OrderID.String()).
		Float64("price", trade.Price).
		Float64("quantity", trade.Quantity).
		Msg("Trade inserted into database")

	return nil
}

// GetOrder retrieves an order by ID
func (db *DB) GetOrder(ctx context.Context, orderID uuid.UUID) (*Order, error) {
	query := `
		SELECT id, session_id, position_id, exchange_order_id, symbol, exchange,
		       side, type, status, price, stop_price, quantity, executed_quantity,
		       executed_quote_quantity, time_in_force, placed_at, filled_at,
		       canceled_at, error_message, metadata, created_at, updated_at
		FROM orders
		WHERE id = $1
	`

	var order Order
	err := db.pool.QueryRow(ctx, query, orderID).Scan(
		&order.ID,
		&order.SessionID,
		&order.PositionID,
		&order.ExchangeOrderID,
		&order.Symbol,
		&order.Exchange,
		&order.Side,
		&order.Type,
		&order.Status,
		&order.Price,
		&order.StopPrice,
		&order.Quantity,
		&order.ExecutedQuantity,
		&order.ExecutedQuoteQuantity,
		&order.TimeInForce,
		&order.PlacedAt,
		&order.FilledAt,
		&order.CanceledAt,
		&order.ErrorMessage,
		&order.Metadata,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// GetTradesByOrderID retrieves all trades for an order
func (db *DB) GetTradesByOrderID(ctx context.Context, orderID uuid.UUID) ([]*Trade, error) {
	query := `
		SELECT id, order_id, exchange_trade_id, symbol, exchange, side,
		       price, quantity, quote_quantity, commission, commission_asset,
		       executed_at, is_maker, metadata, created_at
		FROM trades
		WHERE order_id = $1
		ORDER BY executed_at ASC
	`

	rows, err := db.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		var trade Trade
		err := rows.Scan(
			&trade.ID,
			&trade.OrderID,
			&trade.ExchangeTradeID,
			&trade.Symbol,
			&trade.Exchange,
			&trade.Side,
			&trade.Price,
			&trade.Quantity,
			&trade.QuoteQuantity,
			&trade.Commission,
			&trade.CommissionAsset,
			&trade.ExecutedAt,
			&trade.IsMaker,
			&trade.Metadata,
			&trade.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade: %w", err)
		}
		trades = append(trades, &trade)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trades: %w", err)
	}

	return trades, nil
}

// ConvertOrderSide converts application order side to database enum
func ConvertOrderSide(side string) OrderSide {
	switch strings.ToUpper(side) {
	case "BUY":
		return OrderSideBuy
	case "SELL":
		return OrderSideSell
	default:
		return OrderSideBuy // Default to buy if unknown
	}
}

// ConvertOrderType converts application order type to database enum
func ConvertOrderType(orderType string) OrderType {
	switch strings.ToUpper(orderType) {
	case "MARKET":
		return OrderTypeMarket
	case "LIMIT":
		return OrderTypeLimit
	default:
		return OrderTypeMarket // Default to market if unknown
	}
}

// ConvertOrderStatus converts application order status to database enum
func ConvertOrderStatus(status string) OrderStatus {
	switch strings.ToUpper(status) {
	case "PENDING", "NEW":
		return OrderStatusNew
	case "OPEN", "PARTIALLY_FILLED":
		return OrderStatusPartiallyFilled
	case "FILLED":
		return OrderStatusFilled
	case "CANCELLED", "CANCELED":
		return OrderStatusCanceled
	case "REJECTED":
		return OrderStatusRejected
	default:
		return OrderStatusNew // Default to new if unknown
	}
}

// GetOrderByID is an alias for GetOrder
func (db *DB) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*Order, error) {
	return db.GetOrder(ctx, orderID)
}

// GetOrdersBySession retrieves all orders for a specific session
func (db *DB) GetOrdersBySession(ctx context.Context, sessionID uuid.UUID) ([]*Order, error) {
	query := `
		SELECT id, session_id, position_id, exchange_order_id, symbol, exchange,
		       side, type, status, price, stop_price, quantity, executed_quantity,
		       executed_quote_quantity, time_in_force, placed_at, filled_at,
		       canceled_at, error_message, metadata, created_at, updated_at
		FROM orders
		WHERE session_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders by session: %w", err)
	}
	defer rows.Close()

	return scanOrders(rows)
}

// GetOrdersBySymbol retrieves all orders for a specific symbol
func (db *DB) GetOrdersBySymbol(ctx context.Context, symbol string) ([]*Order, error) {
	query := `
		SELECT id, session_id, position_id, exchange_order_id, symbol, exchange,
		       side, type, status, price, stop_price, quantity, executed_quantity,
		       executed_quote_quantity, time_in_force, placed_at, filled_at,
		       canceled_at, error_message, metadata, created_at, updated_at
		FROM orders
		WHERE symbol = $1
		ORDER BY created_at DESC
	`

	rows, err := db.pool.Query(ctx, query, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders by symbol: %w", err)
	}
	defer rows.Close()

	return scanOrders(rows)
}

// GetOrdersByStatus retrieves all orders with a specific status
func (db *DB) GetOrdersByStatus(ctx context.Context, status OrderStatus) ([]*Order, error) {
	query := `
		SELECT id, session_id, position_id, exchange_order_id, symbol, exchange,
		       side, type, status, price, stop_price, quantity, executed_quantity,
		       executed_quote_quantity, time_in_force, placed_at, filled_at,
		       canceled_at, error_message, metadata, created_at, updated_at
		FROM orders
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := db.pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders by status: %w", err)
	}
	defer rows.Close()

	return scanOrders(rows)
}

// GetRecentOrders retrieves recent orders (limited)
func (db *DB) GetRecentOrders(ctx context.Context, limit int) ([]*Order, error) {
	query := `
		SELECT id, session_id, position_id, exchange_order_id, symbol, exchange,
		       side, type, status, price, stop_price, quantity, executed_quantity,
		       executed_quote_quantity, time_in_force, placed_at, filled_at,
		       canceled_at, error_message, metadata, created_at, updated_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent orders: %w", err)
	}
	defer rows.Close()

	return scanOrders(rows)
}

// scanOrders is a helper to scan multiple order rows
func scanOrders(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*Order, error) {
	var orders []*Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID,
			&order.SessionID,
			&order.PositionID,
			&order.ExchangeOrderID,
			&order.Symbol,
			&order.Exchange,
			&order.Side,
			&order.Type,
			&order.Status,
			&order.Price,
			&order.StopPrice,
			&order.Quantity,
			&order.ExecutedQuantity,
			&order.ExecutedQuoteQuantity,
			&order.TimeInForce,
			&order.PlacedAt,
			&order.FilledAt,
			&order.CanceledAt,
			&order.ErrorMessage,
			&order.Metadata,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, &order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, nil
}
