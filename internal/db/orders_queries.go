package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListOrders returns all orders for a session, optionally filtered by status
func (db *DB) ListOrders(ctx context.Context, sessionID *uuid.UUID, status *OrderStatus, limit int, offset int) ([]*Order, error) {
	query := `
		SELECT
			id, session_id, symbol, side, type, status, quantity, price,
			stop_price, executed_quantity, executed_quote_quantity, time_in_force,
			placed_at, filled_at, canceled_at, updated_at, error_message
		FROM orders
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	if sessionID != nil {
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, sessionID)
		argCount++
	}

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *status)
		argCount++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
		argCount++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		err := rows.Scan(
			&order.ID,
			&order.SessionID,
			&order.Symbol,
			&order.Side,
			&order.Type,
			&order.Status,
			&order.Quantity,
			&order.Price,
			&order.StopPrice,
			&order.ExecutedQuantity,
			&order.ExecutedQuoteQuantity,
			&order.TimeInForce,
			&order.PlacedAt,
			&order.FilledAt,
			&order.CanceledAt,
			&order.UpdatedAt,
			&order.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, nil
}

// GetOrdersBySession returns all orders for a specific session
func (db *DB) GetOrdersBySession(ctx context.Context, sessionID uuid.UUID) ([]*Order, error) {
	return db.ListOrders(ctx, &sessionID, nil, 0, 0)
}

// CountOrders returns the total count of orders matching the criteria
func (db *DB) CountOrders(ctx context.Context, sessionID *uuid.UUID, status *OrderStatus) (int, error) {
	query := "SELECT COUNT(*) FROM orders WHERE 1=1"

	args := []interface{}{}
	argCount := 1

	if sessionID != nil {
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, sessionID)
		argCount++
	}

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *status)
	}

	var count int
	err := db.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	return count, nil
}
