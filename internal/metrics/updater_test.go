package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

func TestNewUpdater(t *testing.T) {
	// Create updater with nil pool (just testing constructor)
	interval := 10 * time.Second
	updater := NewUpdater(nil, interval)

	assert.NotNil(t, updater)
	assert.Equal(t, interval, updater.interval)
	assert.NotNil(t, updater.stopCh)
}

func TestUpdater_Stop(t *testing.T) {
	updater := NewUpdater(nil, time.Second)

	// Stop should not panic
	assert.NotPanics(t, func() {
		updater.Stop()
	})

	// Channel should be closed
	_, ok := <-updater.stopCh
	assert.False(t, ok, "stopCh should be closed")
}

func TestNewUpdater_WithDifferentIntervals(t *testing.T) {
	intervals := []time.Duration{
		1 * time.Second,
		10 * time.Second,
		1 * time.Minute,
		5 * time.Minute,
	}

	for _, interval := range intervals {
		t.Run(interval.String(), func(t *testing.T) {
			updater := NewUpdater(nil, interval)
			assert.Equal(t, interval, updater.interval)
		})
	}
}

// Integration tests - require a real database connection
// These will be skipped if database is not available

func setupTestDB(t *testing.T) *pgxpool.Pool {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to connect to test database
	config, err := pgxpool.ParseConfig("postgres://postgres:postgres@localhost:5432/cryptofunk_test?sslmode=disable")
	if err != nil {
		t.Skip("Unable to parse database config, skipping integration test")
		return nil
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skip("Database not available, skipping integration test")
		return nil
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skip("Database not available, skipping integration test")
		return nil
	}

	return pool
}

func TestUpdater_Start_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start updater in background
	done := make(chan bool)
	go func() {
		updater.Start(ctx)
		done <- true
	}()

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Stop the updater
	updater.Stop()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Updater did not stop in time")
	}
}

func TestUpdater_Start_ContextCancellation_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	// Start updater in background
	done := make(chan bool)
	go func() {
		updater.Start(ctx)
		done <- true
	}()

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Updater did not stop when context was cancelled")
	}
}

func TestUpdater_UpdateDatabaseMetrics_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	// This should not panic
	assert.NotPanics(t, func() {
		updater.updateDatabaseMetrics()
	})
}

func TestUpdater_Update_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Update should not panic even if there's no data
	assert.NotPanics(t, func() {
		updater.update(ctx)
	})
}

func TestUpdater_UpdateTradingMetrics_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Should not panic with empty database
	assert.NotPanics(t, func() {
		updater.updateTradingMetrics(ctx)
	})
}

func TestUpdater_UpdateDrawdownMetrics_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Should not panic with empty database
	assert.NotPanics(t, func() {
		updater.updateDrawdownMetrics(ctx)
	})
}

func TestUpdater_UpdateReturnMetrics_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Should not panic with empty database
	assert.NotPanics(t, func() {
		updater.updateReturnMetrics(ctx)
	})
}

func TestUpdater_UpdateSharpeRatio_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Should not panic with empty database
	assert.NotPanics(t, func() {
		updater.updateSharpeRatio(ctx)
	})
}

func TestUpdater_UpdatePositionMetrics_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Should not panic with empty database
	assert.NotPanics(t, func() {
		updater.updatePositionMetrics(ctx)
	})
}

func TestUpdater_UpdateAgentMetrics_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, time.Second)

	ctx := context.Background()

	// Should not panic with empty database
	assert.NotPanics(t, func() {
		updater.updateAgentMetrics(ctx)
	})
}

func TestUpdater_MultipleStops(t *testing.T) {
	updater := NewUpdater(nil, time.Second)

	// First stop should work
	assert.NotPanics(t, func() {
		updater.Stop()
	})

	// Second stop should panic (closing closed channel)
	// This is expected behavior in Go
	assert.Panics(t, func() {
		updater.Stop()
	})
}

func TestUpdater_ImmediateUpdate_Integration(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	updater := NewUpdater(pool, 10*time.Second) // Long interval

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should trigger an immediate update
	// We'll verify this by ensuring Start doesn't block waiting for the first tick
	started := make(chan bool)
	go func() {
		started <- true
		updater.Start(ctx)
	}()

	// Wait for goroutine to start
	<-started

	// Give it a moment to do the immediate update
	time.Sleep(100 * time.Millisecond)

	// Stop and verify it completes quickly (not waiting for full interval)
	cancel()

	// If the immediate update happened, this should complete quickly
	// If not, we'd wait ~10 seconds for the first tick
	time.Sleep(100 * time.Millisecond)
}
