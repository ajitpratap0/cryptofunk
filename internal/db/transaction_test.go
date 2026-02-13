package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTxFromContext_NoTx(t *testing.T) {
	tx := TxFromContext(context.Background())
	if tx != nil {
		t.Fatal("expected nil tx from empty context")
	}
}

func TestWithTransaction_NilPool(t *testing.T) {
	db := &DB{} // nil pool - will fail on Begin

	err := db.WithTransaction(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error with nil pool")
	}
}

func TestQuerier_ReturnsPoolWhenNoTx(t *testing.T) {
	db := &DB{} // nil pool
	q := db.Querier(context.Background())
	// With nil pool, Querier returns the nil *pgxpool.Pool (non-nil interface)
	// This is expected - callers should use WithTransaction instead
	if q == nil {
		t.Fatal("expected non-nil interface (nil *pgxpool.Pool) querier")
	}
}

func TestQuerier_ReturnsTxFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), txContextKey{}, mockTx{})
	db := &DB{}
	q := db.Querier(ctx)
	if q == nil {
		t.Fatal("expected non-nil querier when tx in context")
	}
	if _, ok := q.(mockTx); !ok {
		t.Fatal("expected mockTx type")
	}
}

func TestWithSavepoint_ErrorRollback(t *testing.T) {
	db := &DB{}
	expectedErr := errors.New("inner error")

	mt := &mockTxWithExec{}
	err := db.withSavepoint(context.Background(), mt, func(ctx context.Context) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestWithSavepoint_PanicRecovery(t *testing.T) {
	db := &DB{}
	mt := &mockTxWithExec{}

	err := db.withSavepoint(context.Background(), mt, func(ctx context.Context) error {
		panic("test panic")
	})

	if err == nil {
		t.Fatal("expected error from panic")
	}
	if err.Error() != "savepoint panic: test panic" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithSavepoint_Success(t *testing.T) {
	db := &DB{}
	mt := &mockTxWithExec{}

	err := db.withSavepoint(context.Background(), mt, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have SAVEPOINT and RELEASE SAVEPOINT calls
	if len(mt.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(mt.execCalls))
	}
}

// mockTx implements pgx.Tx minimally for context tests
type mockTx struct{ pgx.Tx }

// mockTxWithExec implements pgx.Tx with Exec support for savepoint tests
type mockTxWithExec struct {
	pgx.Tx
	execCalls []string
}

func (m *mockTxWithExec) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, sql)
	return pgconn.NewCommandTag(""), nil
}
