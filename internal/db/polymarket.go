package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PolymarketMarket represents a tracked Polymarket market.
type PolymarketMarket struct {
	ID        string     `db:"id"`
	Question  string     `db:"question"`
	Category  *string    `db:"category"`
	YesPrice  float64    `db:"yes_price"`
	NoPrice   float64    `db:"no_price"`
	Volume    float64    `db:"volume"`
	EndDate   *time.Time `db:"end_date"`
	Active    bool       `db:"active"`
	UpdatedAt time.Time  `db:"updated_at"`
}

// PolymarketPosition represents a paper trading position.
type PolymarketPosition struct {
	ID          uuid.UUID   `db:"id"`
	SessionID   uuid.UUID   `db:"session_id"`
	MarketID    string      `db:"market_id"`
	Side        string      `db:"side"`
	Shares      float64     `db:"shares"`
	AvgPrice    float64     `db:"avg_price"`
	CostBasis   float64     `db:"cost_basis"`
	Status      string      `db:"status"`
	OpenedAt    time.Time   `db:"opened_at"`
	ClosedAt    *time.Time  `db:"closed_at"`
	RealizedPnl *float64    `db:"realized_pnl"`
	Metadata    interface{} `db:"metadata"`
	CreatedAt   time.Time   `db:"created_at"`
	UpdatedAt   time.Time   `db:"updated_at"`
}

// PolymarketTrade represents a paper trade.
type PolymarketTrade struct {
	ID         uuid.UUID `db:"id"`
	PositionID uuid.UUID `db:"position_id"`
	MarketID   string    `db:"market_id"`
	Action     string    `db:"action"`
	Side       string    `db:"side"`
	Amount     float64   `db:"amount"`
	Price      float64   `db:"price"`
	Shares     float64   `db:"shares"`
	Timestamp  time.Time `db:"timestamp"`
}

// PolymarketPrediction represents a prediction record.
type PolymarketPrediction struct {
	ID            uuid.UUID  `db:"id"`
	MarketID      string     `db:"market_id"`
	PredictedProb float64    `db:"predicted_prob"`
	MarketPrice   float64    `db:"market_price"`
	Edge          float64    `db:"edge"`
	Confidence    float64    `db:"confidence"`
	Category      *string    `db:"category"`
	Reasoning     *string    `db:"reasoning"`
	Outcome       *float64   `db:"outcome"`
	Resolved      bool       `db:"resolved"`
	Pnl           *float64   `db:"pnl"`
	CreatedAt     time.Time  `db:"created_at"`
	ResolvedAt    *time.Time `db:"resolved_at"`
}

// PolymarketPortfolioSummary aggregates portfolio info.
type PolymarketPortfolioSummary struct {
	TotalCostBasis float64
	PositionCount  int
	OpenPositions  []*PolymarketPosition
}

// ---------------------------------------------------------------------------
// Markets
// ---------------------------------------------------------------------------

func (db *DB) InsertPolymarketMarket(ctx context.Context, m *PolymarketMarket) error {
	query := `
		INSERT INTO polymarket_markets (id, question, category, yes_price, no_price, volume, end_date, active, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	m.UpdatedAt = time.Now()
	_, err := db.pool.Exec(ctx, query, m.ID, m.Question, m.Category, m.YesPrice, m.NoPrice, m.Volume, m.EndDate, m.Active, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert polymarket market: %w", err)
	}
	return nil
}

func (db *DB) UpsertPolymarketMarket(ctx context.Context, m *PolymarketMarket) error {
	query := `
		INSERT INTO polymarket_markets (id, question, category, yes_price, no_price, volume, end_date, active, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			question = EXCLUDED.question,
			category = EXCLUDED.category,
			yes_price = EXCLUDED.yes_price,
			no_price = EXCLUDED.no_price,
			volume = EXCLUDED.volume,
			end_date = EXCLUDED.end_date,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
	`
	m.UpdatedAt = time.Now()
	_, err := db.pool.Exec(ctx, query, m.ID, m.Question, m.Category, m.YesPrice, m.NoPrice, m.Volume, m.EndDate, m.Active, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert polymarket market: %w", err)
	}
	return nil
}

func (db *DB) GetPolymarketMarket(ctx context.Context, id string) (*PolymarketMarket, error) {
	query := `SELECT id, question, category, yes_price, no_price, volume, end_date, active, updated_at FROM polymarket_markets WHERE id = $1`
	var m PolymarketMarket
	err := db.pool.QueryRow(ctx, query, id).Scan(&m.ID, &m.Question, &m.Category, &m.YesPrice, &m.NoPrice, &m.Volume, &m.EndDate, &m.Active, &m.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("polymarket market not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get polymarket market: %w", err)
	}
	return &m, nil
}

func (db *DB) ListPolymarketMarkets(ctx context.Context, activeOnly bool) ([]*PolymarketMarket, error) {
	query := `SELECT id, question, category, yes_price, no_price, volume, end_date, active, updated_at FROM polymarket_markets`
	if activeOnly {
		query += ` WHERE active = TRUE`
	}
	query += ` ORDER BY volume DESC`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list polymarket markets: %w", err)
	}
	defer rows.Close()

	var markets []*PolymarketMarket
	for rows.Next() {
		var m PolymarketMarket
		if err := rows.Scan(&m.ID, &m.Question, &m.Category, &m.YesPrice, &m.NoPrice, &m.Volume, &m.EndDate, &m.Active, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan polymarket market: %w", err)
		}
		markets = append(markets, &m)
	}
	return markets, rows.Err()
}

// ---------------------------------------------------------------------------
// Positions
// ---------------------------------------------------------------------------

func (db *DB) InsertPolymarketPosition(ctx context.Context, p *PolymarketPosition) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.OpenedAt.IsZero() {
		p.OpenedAt = now
	}

	query := `
		INSERT INTO polymarket_positions (id, session_id, market_id, side, shares, avg_price, cost_basis, status, opened_at, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := db.pool.Exec(ctx, query, p.ID, p.SessionID, p.MarketID, p.Side, p.Shares, p.AvgPrice, p.CostBasis, p.Status, p.OpenedAt, p.Metadata, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert polymarket position: %w", err)
	}
	return nil
}

func (db *DB) UpdatePolymarketPosition(ctx context.Context, p *PolymarketPosition) error {
	p.UpdatedAt = time.Now()
	query := `
		UPDATE polymarket_positions
		SET shares = $2, avg_price = $3, cost_basis = $4, status = $5, closed_at = $6, realized_pnl = $7, metadata = $8, updated_at = $9
		WHERE id = $1
	`
	result, err := db.pool.Exec(ctx, query, p.ID, p.Shares, p.AvgPrice, p.CostBasis, p.Status, p.ClosedAt, p.RealizedPnl, p.Metadata, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update polymarket position: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("polymarket position not found: %s", p.ID)
	}
	return nil
}

func (db *DB) GetPolymarketPosition(ctx context.Context, id uuid.UUID) (*PolymarketPosition, error) {
	query := `SELECT id, session_id, market_id, side, shares, avg_price, cost_basis, status, opened_at, closed_at, realized_pnl, metadata, created_at, updated_at FROM polymarket_positions WHERE id = $1`
	var p PolymarketPosition
	err := db.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.SessionID, &p.MarketID, &p.Side, &p.Shares, &p.AvgPrice, &p.CostBasis, &p.Status, &p.OpenedAt, &p.ClosedAt, &p.RealizedPnl, &p.Metadata, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("polymarket position not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get polymarket position: %w", err)
	}
	return &p, nil
}

func (db *DB) GetOpenPolymarketPositions(ctx context.Context) ([]*PolymarketPosition, error) {
	query := `SELECT id, session_id, market_id, side, shares, avg_price, cost_basis, status, opened_at, closed_at, realized_pnl, metadata, created_at, updated_at FROM polymarket_positions WHERE status = 'OPEN' ORDER BY opened_at DESC`
	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query open polymarket positions: %w", err)
	}
	defer rows.Close()

	var positions []*PolymarketPosition
	for rows.Next() {
		var p PolymarketPosition
		if err := rows.Scan(&p.ID, &p.SessionID, &p.MarketID, &p.Side, &p.Shares, &p.AvgPrice, &p.CostBasis, &p.Status, &p.OpenedAt, &p.ClosedAt, &p.RealizedPnl, &p.Metadata, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan polymarket position: %w", err)
		}
		positions = append(positions, &p)
	}
	return positions, rows.Err()
}

func (db *DB) ClosePolymarketPosition(ctx context.Context, id uuid.UUID, realizedPnl float64) error {
	now := time.Now()
	query := `UPDATE polymarket_positions SET status = 'CLOSED', closed_at = $2, realized_pnl = $3, updated_at = $4 WHERE id = $1 AND status = 'OPEN'`
	result, err := db.pool.Exec(ctx, query, id, now, realizedPnl, now)
	if err != nil {
		return fmt.Errorf("failed to close polymarket position: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("polymarket position not found or already closed: %s", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Trades
// ---------------------------------------------------------------------------

func (db *DB) InsertPolymarketTrade(ctx context.Context, t *PolymarketTrade) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now()
	}
	query := `INSERT INTO polymarket_trades (id, position_id, market_id, action, side, amount, price, shares, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := db.pool.Exec(ctx, query, t.ID, t.PositionID, t.MarketID, t.Action, t.Side, t.Amount, t.Price, t.Shares, t.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to insert polymarket trade: %w", err)
	}
	return nil
}

func (db *DB) GetPolymarketTrades(ctx context.Context, limit int) ([]*PolymarketTrade, error) {
	query := `SELECT id, position_id, market_id, action, side, amount, price, shares, timestamp FROM polymarket_trades ORDER BY timestamp DESC LIMIT $1`
	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query polymarket trades: %w", err)
	}
	defer rows.Close()

	var trades []*PolymarketTrade
	for rows.Next() {
		var t PolymarketTrade
		if err := rows.Scan(&t.ID, &t.PositionID, &t.MarketID, &t.Action, &t.Side, &t.Amount, &t.Price, &t.Shares, &t.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan polymarket trade: %w", err)
		}
		trades = append(trades, &t)
	}
	return trades, rows.Err()
}

// ---------------------------------------------------------------------------
// Predictions
// ---------------------------------------------------------------------------

func (db *DB) InsertPolymarketPrediction(ctx context.Context, p *PolymarketPrediction) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	query := `INSERT INTO polymarket_predictions (id, market_id, predicted_prob, market_price, edge, confidence, category, reasoning, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := db.pool.Exec(ctx, query, p.ID, p.MarketID, p.PredictedProb, p.MarketPrice, p.Edge, p.Confidence, p.Category, p.Reasoning, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert polymarket prediction: %w", err)
	}
	return nil
}

func (db *DB) ResolvePrediction(ctx context.Context, id uuid.UUID, outcome float64, pnl float64) error {
	now := time.Now()
	query := `UPDATE polymarket_predictions SET outcome = $2, resolved = TRUE, pnl = $3, resolved_at = $4 WHERE id = $1`
	result, err := db.pool.Exec(ctx, query, id, outcome, pnl, now)
	if err != nil {
		return fmt.Errorf("failed to resolve prediction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("prediction not found: %s", id)
	}
	return nil
}

func (db *DB) GetPolymarketPredictions(ctx context.Context, resolvedOnly bool) ([]*PolymarketPrediction, error) {
	query := `SELECT id, market_id, predicted_prob, market_price, edge, confidence, category, reasoning, outcome, resolved, pnl, created_at, resolved_at FROM polymarket_predictions`
	if resolvedOnly {
		query += ` WHERE resolved = TRUE`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query polymarket predictions: %w", err)
	}
	defer rows.Close()

	var predictions []*PolymarketPrediction
	for rows.Next() {
		var p PolymarketPrediction
		if err := rows.Scan(&p.ID, &p.MarketID, &p.PredictedProb, &p.MarketPrice, &p.Edge, &p.Confidence, &p.Category, &p.Reasoning, &p.Outcome, &p.Resolved, &p.Pnl, &p.CreatedAt, &p.ResolvedAt); err != nil {
			return nil, fmt.Errorf("failed to scan polymarket prediction: %w", err)
		}
		predictions = append(predictions, &p)
	}
	return predictions, rows.Err()
}

// ---------------------------------------------------------------------------
// Portfolio Summary
// ---------------------------------------------------------------------------

func (db *DB) GetPolymarketPortfolioSummary(ctx context.Context) (*PolymarketPortfolioSummary, error) {
	positions, err := db.GetOpenPolymarketPositions(ctx)
	if err != nil {
		return nil, err
	}

	var totalCost float64
	for _, p := range positions {
		totalCost += p.CostBasis
	}

	return &PolymarketPortfolioSummary{
		TotalCostBasis: totalCost,
		PositionCount:  len(positions),
		OpenPositions:  positions,
	}, nil
}
