package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListAllTrades returns recent trade fills across all orders, newest first.
// For fills by specific order, use GetTradesByOrderID in orders.go instead.
func (db *DB) ListAllTrades(ctx context.Context, limit, offset int) ([]*Trade, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, order_id, exchange_trade_id, symbol, exchange, side,
		       price, quantity, quote_quantity, commission, commission_asset,
		       executed_at, is_maker, metadata, created_at
		FROM trades
		ORDER BY executed_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		t := &Trade{}
		if err := rows.Scan(
			&t.ID, &t.OrderID, &t.ExchangeTradeID, &t.Symbol, &t.Exchange, &t.Side,
			&t.Price, &t.Quantity, &t.QuoteQuantity, &t.Commission, &t.CommissionAsset,
			&t.ExecutedAt, &t.IsMaker, &t.Metadata, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan trade row: %w", err)
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

// CountAllTrades returns an approximate count of trade fill records using pg_class statistics.
// This is O(1) instead of O(n) — avoids a full sequential COUNT(*) scan on every request.
// The estimate is sourced from pg_class.reltuples which is updated by ANALYZE/autovacuum.
// If the table has never been analyzed (reltuples = -1), the function returns 0.
// to_regclass('public.trades') is used instead of current_schema() to avoid a wrong
// result when TimescaleDB adds _timescaledb_internal (or another schema) to search_path,
// which would make current_schema() return something other than 'public'.
func (db *DB) CountAllTrades(ctx context.Context) (int, error) {
	var estimate int64
	err := db.pool.QueryRow(ctx,
		`SELECT COALESCE(reltuples::bigint, -1) AS estimate
		 FROM pg_class
		 WHERE oid = to_regclass('public.trades')`,
	).Scan(&estimate)
	if err != nil {
		// pgx returns pgx.ErrNoRows when to_regclass returns NULL (table doesn't exist).
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to count trades: %w", err)
	}
	// reltuples is -1 for a freshly created table that has never been analyzed.
	if estimate < 0 {
		return 0, nil
	}
	return int(estimate), nil
}
