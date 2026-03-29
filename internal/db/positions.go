package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PositionSide represents the side of a position
type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
	PositionSideFlat  PositionSide = "FLAT"
)

// varLookbackDays is the rolling window used by GetAllClosedPositions and
// GetClosedPositionReturns when collecting historical trade returns for VaR
// calculations. 90 days provides roughly one calendar quarter of returns —
// recent enough to reflect current volatility regimes while giving enough data
// points for a statistically meaningful tail estimate.
//
// Trade-off: in a prolonged calm period this window may underestimate tail risk
// compared to a longer lookback. If the strategy is highly seasonal or the
// exchange has had low activity, consider extending this window or making it
// configurable via the risk config struct.
const varLookbackDays = 90

// Position represents a trading position
type Position struct {
	ID            uuid.UUID    `db:"id"`
	SessionID     *uuid.UUID   `db:"session_id"`
	Symbol        string       `db:"symbol"`
	Exchange      string       `db:"exchange"`
	Side          PositionSide `db:"side"`
	EntryPrice    float64      `db:"entry_price"`
	ExitPrice     *float64     `db:"exit_price"`
	Quantity      float64      `db:"quantity"`
	EntryTime     time.Time    `db:"entry_time"`
	ExitTime      *time.Time   `db:"exit_time"`
	StopLoss      *float64     `db:"stop_loss"`
	TakeProfit    *float64     `db:"take_profit"`
	RealizedPnL   *float64     `db:"realized_pnl"`
	UnrealizedPnL *float64     `db:"unrealized_pnl"`
	Fees          float64      `db:"fees"`
	EntryReason   *string      `db:"entry_reason"`
	ExitReason    *string      `db:"exit_reason"`
	Metadata      interface{}  `db:"metadata"`
	CreatedAt     time.Time    `db:"created_at"`
	UpdatedAt     time.Time    `db:"updated_at"`
}

// CreatePosition inserts a new position into the database
func (db *DB) CreatePosition(ctx context.Context, position *Position) error {
	query := `
		INSERT INTO positions (
			id, session_id, symbol, exchange, side, entry_price, quantity,
			entry_time, stop_loss, take_profit, entry_reason, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	if position.ID == uuid.Nil {
		position.ID = uuid.New()
	}
	if position.CreatedAt.IsZero() {
		position.CreatedAt = time.Now()
	}
	if position.UpdatedAt.IsZero() {
		position.UpdatedAt = time.Now()
	}

	_, err := db.pool.Exec(ctx, query,
		position.ID,
		position.SessionID,
		position.Symbol,
		position.Exchange,
		position.Side,
		position.EntryPrice,
		position.Quantity,
		position.EntryTime,
		position.StopLoss,
		position.TakeProfit,
		position.EntryReason,
		position.Metadata,
		position.CreatedAt,
		position.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create position: %w", err)
	}

	return nil
}

// UpdatePosition updates an existing position
func (db *DB) UpdatePosition(ctx context.Context, position *Position) error {
	query := `
		UPDATE positions
		SET
			exit_price = $2,
			exit_time = $3,
			realized_pnl = $4,
			unrealized_pnl = $5,
			fees = $6,
			exit_reason = $7,
			metadata = $8,
			updated_at = $9
		WHERE id = $1
	`

	position.UpdatedAt = time.Now()

	result, err := db.pool.Exec(ctx, query,
		position.ID,
		position.ExitPrice,
		position.ExitTime,
		position.RealizedPnL,
		position.UnrealizedPnL,
		position.Fees,
		position.ExitReason,
		position.Metadata,
		position.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update position: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("position not found: %s", position.ID)
	}

	return nil
}

// GetPosition retrieves a position by ID
func (db *DB) GetPosition(ctx context.Context, id uuid.UUID) (*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE id = $1
	`

	var position Position
	err := db.pool.QueryRow(ctx, query, id).Scan(
		&position.ID,
		&position.SessionID,
		&position.Symbol,
		&position.Exchange,
		&position.Side,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryTime,
		&position.ExitTime,
		&position.StopLoss,
		&position.TakeProfit,
		&position.RealizedPnL,
		&position.UnrealizedPnL,
		&position.Fees,
		&position.EntryReason,
		&position.ExitReason,
		&position.Metadata,
		&position.CreatedAt,
		&position.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("position not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get position: %w", err)
	}

	return &position, nil
}

// GetOpenPositions retrieves all open positions for a session
func (db *DB) GetOpenPositions(ctx context.Context, sessionID uuid.UUID) ([]*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE session_id = $1 AND exit_time IS NULL
		ORDER BY entry_time DESC
	`

	rows, err := db.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open positions: %w", err)
	}
	defer rows.Close()

	var positions []*Position
	for rows.Next() {
		var position Position
		err := rows.Scan(
			&position.ID,
			&position.SessionID,
			&position.Symbol,
			&position.Exchange,
			&position.Side,
			&position.EntryPrice,
			&position.ExitPrice,
			&position.Quantity,
			&position.EntryTime,
			&position.ExitTime,
			&position.StopLoss,
			&position.TakeProfit,
			&position.RealizedPnL,
			&position.UnrealizedPnL,
			&position.Fees,
			&position.EntryReason,
			&position.ExitReason,
			&position.Metadata,
			&position.CreatedAt,
			&position.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, &position)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating positions: %w", err)
	}

	return positions, nil
}

// ClosePosition closes a position with exit price and reason
func (db *DB) ClosePosition(ctx context.Context, id uuid.UUID, exitPrice float64, exitReason string, fees float64) error {
	// Get the position first to calculate realized P&L
	position, err := db.GetPosition(ctx, id)
	if err != nil {
		return err
	}

	if position.ExitTime != nil {
		return fmt.Errorf("position already closed: %s", id)
	}

	// Calculate realized P&L
	var realizedPnL float64
	if position.Side == PositionSideLong {
		// LONG: profit when exit price > entry price
		realizedPnL = (exitPrice - position.EntryPrice) * position.Quantity
	} else {
		// SHORT: profit when exit price < entry price
		realizedPnL = (position.EntryPrice - exitPrice) * position.Quantity
	}

	// Subtract fees
	realizedPnL -= fees

	// Update position
	now := time.Now()
	query := `
		UPDATE positions
		SET
			exit_price = $2,
			exit_time = $3,
			realized_pnl = $4,
			unrealized_pnl = 0,
			fees = fees + $5,
			exit_reason = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := db.pool.Exec(ctx, query,
		id,
		exitPrice,
		now,
		realizedPnL,
		fees,
		exitReason,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("position not found: %s", id)
	}

	return nil
}

// UpdateUnrealizedPnL updates the unrealized P&L for an open position
func (db *DB) UpdateUnrealizedPnL(ctx context.Context, id uuid.UUID, currentPrice float64) error {
	// Get the position
	position, err := db.GetPosition(ctx, id)
	if err != nil {
		return err
	}

	if position.ExitTime != nil {
		return fmt.Errorf("cannot update unrealized P&L for closed position: %s", id)
	}

	// Calculate unrealized P&L
	var unrealizedPnL float64
	if position.Side == PositionSideLong {
		unrealizedPnL = (currentPrice - position.EntryPrice) * position.Quantity
	} else {
		unrealizedPnL = (position.EntryPrice - currentPrice) * position.Quantity
	}

	// Update position
	query := `
		UPDATE positions
		SET
			unrealized_pnl = $2,
			updated_at = $3
		WHERE id = $1
	`

	_, err = db.pool.Exec(ctx, query, id, unrealizedPnL, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update unrealized P&L: %w", err)
	}

	return nil
}

// GetAllOpenPositions retrieves all open positions (no session filter)
func (db *DB) GetAllOpenPositions(ctx context.Context) ([]*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE exit_time IS NULL
		ORDER BY entry_time DESC
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all open positions: %w", err)
	}
	defer rows.Close()

	return scanPositions(rows)
}

// GetClosedFeesBySessionIDs returns the sum of fees from closed positions belonging to
// the specified sessions. If sessionIDs is empty, returns 0 immediately without querying.
// Uses a single SQL aggregate instead of loading all rows.
func (db *DB) GetClosedFeesBySessionIDs(ctx context.Context, sessionIDs []uuid.UUID) (float64, error) {
	if len(sessionIDs) == 0 {
		return 0, nil
	}
	var total float64
	err := db.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(fees), 0)
		FROM positions
		WHERE exit_time IS NOT NULL AND session_id = ANY($1::uuid[])
	`, sessionIDs).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum closed position fees: %w", err)
	}
	return total, nil
}

// GetAllClosedPositions returns positions closed within the last varLookbackDays days
// across all sessions, ordered by exit_time DESC. See varLookbackDays for a discussion
// of the scope trade-off.
func (db *DB) GetAllClosedPositions(ctx context.Context) ([]*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE exit_time IS NOT NULL
		  AND exit_time > NOW() - ($1 * INTERVAL '1 day')
		  AND realized_pnl IS NOT NULL
		ORDER BY exit_time DESC
	`

	rows, err := db.pool.Query(ctx, query, varLookbackDays)
	if err != nil {
		return nil, fmt.Errorf("failed to query closed positions: %w", err)
	}
	defer rows.Close()

	return scanPositions(rows)
}

// ClosedPositionReturn holds the minimal fields needed to compute fractional returns
// for VaR calculations: realized_pnl / (entry_price * quantity).
type ClosedPositionReturn struct {
	RealizedPnL float64
	EntryPrice  float64
	Quantity    float64
}

// GetClosedPositionReturns fetches only the columns required for VaR return calculations
// (realized_pnl, entry_price, quantity) from positions closed within the last
// varLookbackDays days. This projects fewer columns than GetAllClosedPositions,
// reducing per-row data transfer for the risk/metrics endpoint (issue #143).
// See varLookbackDays for a discussion of the scope trade-off.
func (db *DB) GetClosedPositionReturns(ctx context.Context) ([]ClosedPositionReturn, error) {
	query := `
		SELECT realized_pnl, entry_price, quantity
		FROM positions
		WHERE exit_time IS NOT NULL
		  AND exit_time > NOW() - ($1 * INTERVAL '1 day')
		  AND realized_pnl IS NOT NULL
		  AND entry_price > 0
		  AND quantity > 0
		ORDER BY exit_time DESC
	`

	rows, err := db.pool.Query(ctx, query, varLookbackDays)
	if err != nil {
		return nil, fmt.Errorf("failed to query closed position returns: %w", err)
	}
	defer rows.Close()

	results := make([]ClosedPositionReturn, 0, 256)
	for rows.Next() {
		var r ClosedPositionReturn
		if err := rows.Scan(&r.RealizedPnL, &r.EntryPrice, &r.Quantity); err != nil {
			return nil, fmt.Errorf("failed to scan closed position return: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating closed position returns: %w", err)
	}
	return results, nil
}

// GetPositionsBySession retrieves all positions (including closed) for a session
func (db *DB) GetPositionsBySession(ctx context.Context, sessionID uuid.UUID) ([]*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE session_id = $1
		ORDER BY entry_time DESC
	`

	rows, err := db.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query positions by session: %w", err)
	}
	defer rows.Close()

	return scanPositions(rows)
}

// GetPositionBySymbolAndSession retrieves a position by symbol and session
func (db *DB) GetPositionBySymbolAndSession(ctx context.Context, symbol string, sessionID uuid.UUID) (*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE symbol = $1 AND session_id = $2 AND exit_time IS NULL
		ORDER BY entry_time DESC
		LIMIT 1
	`

	var position Position
	err := db.pool.QueryRow(ctx, query, symbol, sessionID).Scan(
		&position.ID,
		&position.SessionID,
		&position.Symbol,
		&position.Exchange,
		&position.Side,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryTime,
		&position.ExitTime,
		&position.StopLoss,
		&position.TakeProfit,
		&position.RealizedPnL,
		&position.UnrealizedPnL,
		&position.Fees,
		&position.EntryReason,
		&position.ExitReason,
		&position.Metadata,
		&position.CreatedAt,
		&position.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get position by symbol and session: %w", err)
	}

	return &position, nil
}

// GetOpenPositionBySymbolTx retrieves the most recent open position for a symbol within a session
// using an existing transaction, providing a consistent read within the transaction boundary.
// Returns (nil, nil) when no open position is found.
func (db *DB) GetOpenPositionBySymbolTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, symbol string) (*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE symbol = $1 AND session_id = $2 AND exit_time IS NULL
		ORDER BY entry_time DESC
		LIMIT 1
		FOR UPDATE
	`

	var position Position
	err := tx.QueryRow(ctx, query, symbol, sessionID).Scan(
		&position.ID,
		&position.SessionID,
		&position.Symbol,
		&position.Exchange,
		&position.Side,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryTime,
		&position.ExitTime,
		&position.StopLoss,
		&position.TakeProfit,
		&position.RealizedPnL,
		&position.UnrealizedPnL,
		&position.Fees,
		&position.EntryReason,
		&position.ExitReason,
		&position.Metadata,
		&position.CreatedAt,
		&position.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get open position by symbol in transaction: %w", err)
	}

	return &position, nil
}

// GetLatestPositionBySymbol retrieves the latest position for a symbol (any session)
func (db *DB) GetLatestPositionBySymbol(ctx context.Context, symbol string) (*Position, error) {
	query := `
		SELECT
			id, session_id, symbol, exchange, side, entry_price, exit_price,
			quantity, entry_time, exit_time, stop_loss, take_profit,
			realized_pnl, unrealized_pnl, fees, entry_reason, exit_reason,
			metadata, created_at, updated_at
		FROM positions
		WHERE symbol = $1 AND exit_time IS NULL
		ORDER BY entry_time DESC
		LIMIT 1
	`

	var position Position
	err := db.pool.QueryRow(ctx, query, symbol).Scan(
		&position.ID,
		&position.SessionID,
		&position.Symbol,
		&position.Exchange,
		&position.Side,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryTime,
		&position.ExitTime,
		&position.StopLoss,
		&position.TakeProfit,
		&position.RealizedPnL,
		&position.UnrealizedPnL,
		&position.Fees,
		&position.EntryReason,
		&position.ExitReason,
		&position.Metadata,
		&position.CreatedAt,
		&position.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest position by symbol: %w", err)
	}

	return &position, nil
}

// scanPositions is a helper to scan multiple position rows
func scanPositions(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*Position, error) {
	var positions []*Position
	for rows.Next() {
		var position Position
		err := rows.Scan(
			&position.ID,
			&position.SessionID,
			&position.Symbol,
			&position.Exchange,
			&position.Side,
			&position.EntryPrice,
			&position.ExitPrice,
			&position.Quantity,
			&position.EntryTime,
			&position.ExitTime,
			&position.StopLoss,
			&position.TakeProfit,
			&position.RealizedPnL,
			&position.UnrealizedPnL,
			&position.Fees,
			&position.EntryReason,
			&position.ExitReason,
			&position.Metadata,
			&position.CreatedAt,
			&position.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, &position)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating positions: %w", err)
	}

	return positions, nil
}

// UpdatePositionQuantity updates the quantity of an open position (for partial closes)
func (db *DB) UpdatePositionQuantity(ctx context.Context, id uuid.UUID, newQuantity float64, additionalFees float64) error {
	query := `
		UPDATE positions
		SET
			quantity = $2,
			fees = fees + $3,
			updated_at = $4
		WHERE id = $1 AND exit_time IS NULL
	`

	result, err := db.pool.Exec(ctx, query,
		id,
		newQuantity,
		additionalFees,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update position quantity: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("position not found or already closed: %s", id)
	}

	return nil
}

// CreatePositionTx inserts a new position into the database within an existing transaction.
func (db *DB) CreatePositionTx(ctx context.Context, tx pgx.Tx, position *Position) error {
	query := `
		INSERT INTO positions (
			id, session_id, symbol, exchange, side, entry_price, quantity,
			entry_time, stop_loss, take_profit, entry_reason, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	if position.ID == uuid.Nil {
		position.ID = uuid.New()
	}
	if position.CreatedAt.IsZero() {
		position.CreatedAt = time.Now()
	}
	if position.UpdatedAt.IsZero() {
		position.UpdatedAt = time.Now()
	}

	_, err := tx.Exec(ctx, query,
		position.ID,
		position.SessionID,
		position.Symbol,
		position.Exchange,
		position.Side,
		position.EntryPrice,
		position.Quantity,
		position.EntryTime,
		position.StopLoss,
		position.TakeProfit,
		position.EntryReason,
		position.Metadata,
		position.CreatedAt,
		position.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create position: %w", err)
	}

	return nil
}

// UpdatePositionAveragingTx updates entry price and quantity when adding to a position,
// within an existing transaction.
func (db *DB) UpdatePositionAveragingTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, newEntryPrice, newQuantity float64, additionalFees float64) error {
	query := `
		UPDATE positions
		SET
			entry_price = $2,
			quantity = $3,
			fees = fees + $4,
			updated_at = $5
		WHERE id = $1 AND exit_time IS NULL
	`

	result, err := tx.Exec(ctx, query,
		id,
		newEntryPrice,
		newQuantity,
		additionalFees,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update position averaging: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("position not found or already closed: %s", id)
	}

	return nil
}

// UpdatePositionAveraging updates entry price and quantity when adding to a position
func (db *DB) UpdatePositionAveraging(ctx context.Context, id uuid.UUID, newEntryPrice, newQuantity float64, additionalFees float64) error {
	query := `
		UPDATE positions
		SET
			entry_price = $2,
			quantity = $3,
			fees = fees + $4,
			updated_at = $5
		WHERE id = $1 AND exit_time IS NULL
	`

	result, err := db.pool.Exec(ctx, query,
		id,
		newEntryPrice,
		newQuantity,
		additionalFees,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update position averaging: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("position not found or already closed: %s", id)
	}

	return nil
}

// PartialClosePosition partially closes a position and returns a new position for the closed part
func (db *DB) PartialClosePosition(ctx context.Context, id uuid.UUID, closeQuantity, exitPrice float64, exitReason string, fees float64) (*Position, error) {
	// Get the original position
	position, err := db.GetPosition(ctx, id)
	if err != nil {
		return nil, err
	}

	if position.ExitTime != nil {
		return nil, fmt.Errorf("position already closed: %s", id)
	}

	if closeQuantity >= position.Quantity {
		return nil, fmt.Errorf("close quantity (%f) >= position quantity (%f), use ClosePosition instead", closeQuantity, position.Quantity)
	}

	// Calculate realized P&L for closed portion
	var realizedPnL float64
	if position.Side == PositionSideLong {
		realizedPnL = (exitPrice - position.EntryPrice) * closeQuantity
	} else {
		realizedPnL = (position.EntryPrice - exitPrice) * closeQuantity
	}
	realizedPnL -= fees

	// Create a new closed position record for the partial close
	now := time.Now()
	closedPosition := &Position{
		ID:          uuid.New(),
		SessionID:   position.SessionID,
		Symbol:      position.Symbol,
		Exchange:    position.Exchange,
		Side:        position.Side,
		EntryPrice:  position.EntryPrice,
		ExitPrice:   &exitPrice,
		Quantity:    closeQuantity,
		EntryTime:   position.EntryTime,
		ExitTime:    &now,
		StopLoss:    position.StopLoss,
		TakeProfit:  position.TakeProfit,
		RealizedPnL: &realizedPnL,
		Fees:        fees,
		EntryReason: position.EntryReason,
		ExitReason:  &exitReason,
		Metadata:    position.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Insert the closed portion as a new record.
	// CreatePosition only inserts open-position columns (exit_time is omitted), so we
	// use a direct INSERT here to include exit_time, exit_price, realized_pnl, etc.
	// Without exit_time the row would land as NULL, violating the UNIQUE partial index
	// idx_positions_open_session_symbol_uniq (session_id, symbol) WHERE exit_time IS NULL.
	insertClosedQuery := `
		INSERT INTO positions (
			id, session_id, symbol, exchange, side, entry_price, exit_price, quantity,
			entry_time, exit_time, stop_loss, take_profit, realized_pnl, fees,
			entry_reason, exit_reason, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
	`
	_, err = db.pool.Exec(ctx, insertClosedQuery,
		closedPosition.ID,
		closedPosition.SessionID,
		closedPosition.Symbol,
		closedPosition.Exchange,
		closedPosition.Side,
		closedPosition.EntryPrice,
		closedPosition.ExitPrice,
		closedPosition.Quantity,
		closedPosition.EntryTime,
		closedPosition.ExitTime,
		closedPosition.StopLoss,
		closedPosition.TakeProfit,
		closedPosition.RealizedPnL,
		closedPosition.Fees,
		closedPosition.EntryReason,
		closedPosition.ExitReason,
		closedPosition.Metadata,
		closedPosition.CreatedAt,
		closedPosition.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create closed position record: %w", err)
	}

	// Update the original position to reduce quantity
	remainingQuantity := position.Quantity - closeQuantity
	err = db.UpdatePositionQuantity(ctx, id, remainingQuantity, 0) // fees already accounted for in closed portion
	if err != nil {
		return nil, fmt.Errorf("failed to update remaining position quantity: %w", err)
	}

	return closedPosition, nil
}

// PairPerformance holds aggregated realized PnL and trade count for a single trading pair.
type PairPerformance struct {
	Symbol      string  `db:"symbol"`
	RealizedPnL float64 `db:"realized_pnl"`
	TradeCount  int     `db:"trade_count"`
}

// GetPairPerformance returns realized PnL and trade count grouped by symbol using SQL GROUP BY,
// covering all closed positions where realized_pnl is not NULL.
func (db *DB) GetPairPerformance(ctx context.Context) ([]PairPerformance, error) {
	query := `
		SELECT symbol, COALESCE(SUM(realized_pnl), 0) AS realized_pnl, COUNT(*) AS trade_count
		FROM positions
		WHERE exit_time IS NOT NULL AND realized_pnl IS NOT NULL
		GROUP BY symbol
		ORDER BY realized_pnl DESC
		-- Cap at 200 rows — sufficient for dashboard display; a full paginated API is a follow-up.
		LIMIT 200
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pair performance: %w", err)
	}
	defer rows.Close()

	var results []PairPerformance
	for rows.Next() {
		var p PairPerformance
		if err := rows.Scan(&p.Symbol, &p.RealizedPnL, &p.TradeCount); err != nil {
			return nil, fmt.Errorf("failed to scan pair performance row: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pair performance rows: %w", err)
	}
	return results, nil
}

// SymbolExposure holds the cost-basis exposure for a single symbol across all open positions.
type SymbolExposure struct {
	Symbol   string  `db:"symbol"`
	Exposure float64 `db:"exposure"`
}

// GetExposureBySymbol returns the total open-position exposure (quantity * entry_price) grouped by
// symbol using SQL GROUP BY. Exposure is calculated at cost-basis, not mark-to-market.
func (db *DB) GetExposureBySymbol(ctx context.Context) ([]SymbolExposure, error) {
	query := `
		SELECT symbol, SUM(quantity * entry_price) AS exposure
		FROM positions
		WHERE exit_time IS NULL
		GROUP BY symbol
		ORDER BY exposure DESC
		-- Cap at 200 rows — sufficient for dashboard display; a full paginated API is a follow-up.
		LIMIT 200
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query exposure by symbol: %w", err)
	}
	defer rows.Close()

	var results []SymbolExposure
	for rows.Next() {
		var s SymbolExposure
		if err := rows.Scan(&s.Symbol, &s.Exposure); err != nil {
			return nil, fmt.Errorf("failed to scan symbol exposure row: %w", err)
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating symbol exposure rows: %w", err)
	}
	return results, nil
}

// ConvertPositionSide converts a string to PositionSide
func ConvertPositionSide(side string) PositionSide {
	switch side {
	case "LONG", "long", "buy", "BUY":
		return PositionSideLong
	case "SHORT", "short", "sell", "SELL":
		return PositionSideShort
	default:
		return PositionSideFlat
	}
}

// ClosePositionTx fully closes a position inside an existing transaction.
// Uses the entry_price and quantity already on the position row to compute realized P&L.
// Open/closed state is determined by exit_time IS NULL — there is no status column.
//
// fees is the total fees attributable to this position's closed portion (entry fees
// already recorded on the row plus any exit commission). The column is SET (not added)
// because the caller (handlePaperTrade) computes proportionalFees = existingPos.Fees *
// (closeQty / existingPos.Quantity), which is already a slice of the entry fees stored
// in the column. Using fees=fees+$5 would double-count those entry fees.
func (db *DB) ClosePositionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, exitPrice float64, exitReason string, fees float64) error {
	// NOTE: The SELECT below re-reads fields that the caller already has from a
	// FOR UPDATE lock (via GetOpenPositionBySymbolTx). The re-read is redundant but
	// safe — it provides a defence-in-depth guard and keeps this function
	// self-contained. Eliminating it (by accepting entryPrice/side as parameters)
	// is a valid follow-up refactor but is out of scope here.
	var entryPrice, qty float64
	var side PositionSide
	if err := tx.QueryRow(ctx,
		`SELECT entry_price, quantity, side FROM positions WHERE id=$1 AND exit_time IS NULL`,
		id).Scan(&entryPrice, &qty, &side); err != nil {
		return fmt.Errorf("ClosePositionTx: read position: %w", err)
	}

	var realizedPnL float64
	if side == PositionSideLong {
		realizedPnL = (exitPrice-entryPrice)*qty - fees
	} else {
		realizedPnL = (entryPrice-exitPrice)*qty - fees
	}

	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE positions
		SET exit_price=$2, exit_time=$3, realized_pnl=$4, unrealized_pnl=0,
		    fees=$5, exit_reason=$6, updated_at=$7
		WHERE id=$1
	`, id, exitPrice, now, realizedPnL, fees, exitReason, now)
	return err
}

// PartialClosePositionTx partially closes a position inside an existing transaction.
// Creates a new closed position row for the closed portion and reduces the open
// position's quantity. existingPos must be the tx-locked position (from GetOpenPositionBySymbolTx).
// Returns the updated open position (in-memory; not re-fetched from DB).
func (db *DB) PartialClosePositionTx(ctx context.Context, tx pgx.Tx, existingPos *Position, closeQty, exitPrice float64, exitReason string, closeFees float64) (*Position, error) {
	remainQty := existingPos.Quantity - closeQty
	if remainQty < 1e-10 {
		return nil, fmt.Errorf("PartialClosePositionTx: remainQty must be > 0; use ClosePositionTx for full close")
	}
	if existingPos.Quantity == 0 {
		return nil, fmt.Errorf("PartialClosePositionTx: existing position has zero quantity")
	}
	now := time.Now()
	// Fees proportional to the remaining open size stay on the position
	remainFees := existingPos.Fees * (remainQty / existingPos.Quantity)

	var realizedPnL float64
	if existingPos.Side == PositionSideLong {
		realizedPnL = (exitPrice-existingPos.EntryPrice)*closeQty - closeFees
	} else {
		realizedPnL = (existingPos.EntryPrice-exitPrice)*closeQty - closeFees
	}

	// Insert a new row for the closed portion
	_, err := tx.Exec(ctx, `
		INSERT INTO positions (
			id, session_id, symbol, exchange, side,
			entry_price, exit_price, quantity, entry_time, exit_time,
			realized_pnl, fees, entry_reason, exit_reason,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		uuid.New(), existingPos.SessionID, existingPos.Symbol, existingPos.Exchange, existingPos.Side,
		existingPos.EntryPrice, exitPrice, closeQty, existingPos.EntryTime, now,
		realizedPnL, closeFees, existingPos.EntryReason, &exitReason,
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("PartialClosePositionTx: insert closed portion: %w", err)
	}

	// Scale unrealized_pnl proportionally to remaining quantity so the open position
	// no longer carries P&L attributed to the closed portion.
	remainUnrealizedPnL := 0.0
	if existingPos.UnrealizedPnL != nil && existingPos.Quantity > 0 {
		remainUnrealizedPnL = *existingPos.UnrealizedPnL * (remainQty / existingPos.Quantity)
	}

	// Update remaining open position's quantity, fees, and unrealized_pnl (SET, not ADD)
	_, err = tx.Exec(ctx, `
		UPDATE positions
		SET quantity=$2, fees=$3, unrealized_pnl=$5, updated_at=$4
		WHERE id=$1 AND exit_time IS NULL
	`, existingPos.ID, remainQty, remainFees, now, remainUnrealizedPnL)
	if err != nil {
		return nil, fmt.Errorf("PartialClosePositionTx: update remaining: %w", err)
	}

	// Return updated in-memory copy
	remain := *existingPos
	remain.Quantity = remainQty
	remain.Fees = remainFees
	remain.UnrealizedPnL = &remainUnrealizedPnL
	remain.UpdatedAt = now
	return &remain, nil
}
