package db

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// txContextKey is the context key for storing an active transaction
type txContextKey struct{}

// savepointCounter generates unique savepoint names
var savepointCounter atomic.Int64

// DBTX is an interface satisfied by both *pgxpool.Pool and pgx.Tx,
// allowing functions to work with either a pool or a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxFromContext extracts the active transaction from context, if any.
func TxFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx
}

// Querier returns the transaction from context if available, otherwise the pool.
// Use this in DB methods to automatically participate in transactions.
func (db *DB) Querier(ctx context.Context) DBTX {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return db.pool
}

// WithTransaction executes fn within a database transaction.
// If a transaction already exists in the context (nested call), it uses a savepoint.
// On success, the transaction is committed (or savepoint released).
// On error or panic, the transaction is rolled back (or savepoint rolled back).
func (db *DB) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	// Check for existing transaction (nested transaction → savepoint)
	if existingTx := TxFromContext(ctx); existingTx != nil {
		return db.withSavepoint(ctx, existingTx, fn)
	}

	// Begin new transaction
	if db.pool == nil {
		return fmt.Errorf("begin transaction: connection pool is nil")
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Put transaction in context
	txCtx := context.WithValue(ctx, txContextKey{}, tx)

	// Handle panics
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			log.Error().Interface("panic", p).Msg("Transaction rolled back due to panic")
			err = fmt.Errorf("transaction panic: %v", p)
		}
	}()

	// Execute the function
	if err = fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			log.Error().Err(rbErr).Msg("Failed to rollback transaction")
		}
		return err
	}

	// Commit
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// withSavepoint executes fn within a savepoint (nested transaction).
func (db *DB) withSavepoint(ctx context.Context, tx pgx.Tx, fn func(ctx context.Context) error) (err error) {
	id := savepointCounter.Add(1)
	savepointName := fmt.Sprintf("sp_%d", id)

	if _, err = tx.Exec(ctx, "SAVEPOINT "+savepointName); err != nil {
		return fmt.Errorf("create savepoint: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepointName)
			log.Error().Interface("panic", p).Str("savepoint", savepointName).Msg("Savepoint rolled back due to panic")
			err = fmt.Errorf("savepoint panic: %v", p)
		}
	}()

	if err = fn(ctx); err != nil {
		if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepointName); rbErr != nil {
			log.Error().Err(rbErr).Str("savepoint", savepointName).Msg("Failed to rollback savepoint")
		}
		return err
	}

	if _, err = tx.Exec(ctx, "RELEASE SAVEPOINT "+savepointName); err != nil {
		return fmt.Errorf("release savepoint: %w", err)
	}

	return nil
}

// BeginTx starts a new transaction and returns it with an updated context.
// Prefer WithTransaction for most use cases. Use this only when you need manual control.
func (db *DB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, context.Context, error) {
	tx, err := db.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, ctx, fmt.Errorf("begin transaction: %w", err)
	}
	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	return tx, txCtx, nil
}

// PoolForTesting returns the pool - for tests only
func (db *DB) PoolForTesting() *pgxpool.Pool {
	return db.pool
}
