package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	session := &TradingSession{
		Mode:           TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
		Config: map[string]interface{}{
			"strategy": "trend_following",
			"risk":     0.02,
		},
		Metadata: map[string]interface{}{
			"test": true,
		},
	}

	err := db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Verify ID was generated
	assert.NotEqual(t, uuid.Nil, session.ID)
	assert.NotZero(t, session.CreatedAt)
	assert.NotZero(t, session.UpdatedAt)
}

func TestCreateSession_WithProvidedID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	customID := uuid.New()

	session := &TradingSession{
		ID:             customID,
		Mode:           TradingModeLive,
		Symbol:         "ETH/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 5000.0,
	}

	err := db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Verify custom ID was preserved
	assert.Equal(t, customID, session.ID)
}

func TestGetSession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a session
	originalSession := &TradingSession{
		Mode:           TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
		Config: map[string]interface{}{
			"strategy": "mean_reversion",
		},
	}

	err := db.CreateSession(ctx, originalSession)
	require.NoError(t, err)

	// Retrieve it
	retrievedSession, err := db.GetSession(ctx, originalSession.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedSession)

	// Verify fields
	assert.Equal(t, originalSession.ID, retrievedSession.ID)
	assert.Equal(t, originalSession.Mode, retrievedSession.Mode)
	assert.Equal(t, originalSession.Symbol, retrievedSession.Symbol)
	assert.Equal(t, originalSession.Exchange, retrievedSession.Exchange)
	assert.Equal(t, originalSession.InitialCapital, retrievedSession.InitialCapital)
	assert.NotNil(t, retrievedSession.Config)
}

func TestGetSession_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentID := uuid.New()

	session, err := db.GetSession(ctx, nonExistentID)
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestUpdateSessionStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a session
	session := &TradingSession{
		Mode:           TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}

	err := db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Update stats
	sharpeRatio := 1.8
	err = db.UpdateSessionStats(ctx, session.ID, SessionStats{
		TotalTrades:   25,
		WinningTrades: 15,
		LosingTrades:  10,
		TotalPnL:      1500.0,
		MaxDrawdown:   -200.0,
		SharpeRatio:   &sharpeRatio,
	})
	require.NoError(t, err)

	// Retrieve and verify
	updated, err := db.GetSession(ctx, session.ID)
	require.NoError(t, err)

	assert.Equal(t, 25, updated.TotalTrades)
	assert.Equal(t, 15, updated.WinningTrades)
	assert.Equal(t, 10, updated.LosingTrades)
	assert.Equal(t, 1500.0, updated.TotalPnL)
	assert.Equal(t, -200.0, updated.MaxDrawdown)
	assert.NotNil(t, updated.SharpeRatio)
	assert.Equal(t, 1.8, *updated.SharpeRatio)
}

func TestStopSession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a session
	session := &TradingSession{
		Mode:           TradingModePaper,
		Symbol:         "BTC/USDT",
		Exchange:       "binance",
		StartedAt:      time.Now(),
		InitialCapital: 10000.0,
	}

	err := db.CreateSession(ctx, session)
	require.NoError(t, err)

	// Stop it
	err = db.StopSession(ctx, session.ID, 10500.0)
	require.NoError(t, err)

	// Verify
	stopped, err := db.GetSession(ctx, session.ID)
	require.NoError(t, err)

	assert.NotNil(t, stopped.StoppedAt)
	assert.NotNil(t, stopped.FinalCapital)
	assert.Equal(t, 10500.0, *stopped.FinalCapital)
}

// TestListActiveSessions is skipped because ListActiveSessions() method doesn't exist yet
// TODO: Implement ListActiveSessions() method and enable this test
// func TestListActiveSessions(t *testing.T) {
// 	db, cleanup := setupTestDB(t)
// 	defer cleanup()
//
// 	ctx := context.Background()
//
// 	// Create multiple sessions
// 	session1 := &TradingSession{
// 		Mode:           TradingModePaper,
// 		Symbol:         "BTC/USDT",
// 		Exchange:       "binance",
// 		StartedAt:      time.Now(),
// 		InitialCapital: 10000.0,
// 	}
// 	err := db.CreateSession(ctx, session1)
// 	require.NoError(t, err)
//
// 	session2 := &TradingSession{
// 		Mode:           TradingModeLive,
// 		Symbol:         "ETH/USDT",
// 		Exchange:       "binance",
// 		StartedAt:      time.Now(),
// 		InitialCapital: 5000.0,
// 	}
// 	err = db.CreateSession(ctx, session2)
// 	require.NoError(t, err)
//
// 	// Stop one session
// 	err = db.StopSession(ctx, session2.ID, 5200.0)
// 	require.NoError(t, err)
//
// 	// List active sessions
// 	activeSessions, err := db.ListActiveSessions(ctx)
// 	require.NoError(t, err)
//
// 	// Should have at least session1 (session2 is stopped)
// 	foundSession1 := false
// 	foundSession2 := false
//
// 	for _, s := range activeSessions {
// 		if s.ID == session1.ID {
// 			foundSession1 = true
// 			assert.Nil(t, s.StoppedAt)
// 		}
// 		if s.ID == session2.ID {
// 			foundSession2 = true
// 		}
// 	}
//
// 	assert.True(t, foundSession1, "Active session should be in list")
// 	assert.False(t, foundSession2, "Stopped session should not be in active list")
// }

// TestGetSessionsBySymbol is skipped because GetSessionsBySymbol() method doesn't exist yet
// TODO: Implement GetSessionsBySymbol() method and enable this test
// func TestGetSessionsBySymbol(t *testing.T) {
// 	db, cleanup := setupTestDB(t)
// 	defer cleanup()
//
// 	ctx := context.Background()
//
// 	// Create sessions with different symbols
// 	btcSession := &TradingSession{
// 		Mode:           TradingModePaper,
// 		Symbol:         "BTC/USDT",
// 		Exchange:       "binance",
// 		StartedAt:      time.Now(),
// 		InitialCapital: 10000.0,
// 	}
// 	err := db.CreateSession(ctx, btcSession)
// 	require.NoError(t, err)
//
// 	ethSession := &TradingSession{
// 		Mode:           TradingModePaper,
// 		Symbol:         "ETH/USDT",
// 		Exchange:       "binance",
// 		StartedAt:      time.Now(),
// 		InitialCapital: 5000.0,
// 	}
// 	err = db.CreateSession(ctx, ethSession)
// 	require.NoError(t, err)
//
// 	// Get BTC sessions
// 	btcSessions, err := db.GetSessionsBySymbol(ctx, "BTC/USDT")
// 	require.NoError(t, err)
//
// 	// Should contain our BTC session
// 	found := false
// 	for _, s := range btcSessions {
// 		if s.ID == btcSession.ID {
// 			found = true
// 			assert.Equal(t, "BTC/USDT", s.Symbol)
// 		}
// 		// Should not contain ETH session
// 		assert.NotEqual(t, ethSession.ID, s.ID)
// 	}
// 	assert.True(t, found, "BTC session should be in results")
// }

func TestSessionModes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name string
		mode TradingMode
	}{
		{"Paper trading", TradingModePaper},
		{"Live trading", TradingModeLive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &TradingSession{
				Mode:           tt.mode,
				Symbol:         "BTC/USDT",
				Exchange:       "binance",
				StartedAt:      time.Now(),
				InitialCapital: 10000.0,
			}

			err := db.CreateSession(ctx, session)
			require.NoError(t, err)

			retrieved, err := db.GetSession(ctx, session.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.mode, retrieved.Mode)
		})
	}
}
