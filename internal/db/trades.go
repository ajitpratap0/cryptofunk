package db

import (
	"context"
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
		return nil, err
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
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}
