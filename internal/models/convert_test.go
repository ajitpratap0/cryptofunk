package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// TestBinancePositionToUnified_OpenPosition tests conversion of an open Binance position
// with nil optional fields.
func TestBinancePositionToUnified_OpenPosition(t *testing.T) {
	posID := uuid.New()
	entryTime := time.Now().Add(-1 * time.Hour)

	p := &db.Position{
		ID:         posID,
		Symbol:     "BTC/USDT",
		Side:       db.PositionSideLong,
		EntryPrice: 50000.0,
		Quantity:   0.5,
		EntryTime:  entryTime,
		// nil optionals: SessionID, UnrealizedPnL, RealizedPnL, ExitTime, ExitPrice, EntryReason
	}

	u := BinancePositionToUnified(p)

	assert.Equal(t, posID, u.ID)
	assert.Equal(t, PlatformBinance, u.Platform)
	assert.Equal(t, "BTC/USDT", u.Symbol)
	assert.Equal(t, "long", u.Side, "Side should be lowercased")
	assert.InDelta(t, 50000.0, u.EntryPrice, 1e-9)
	assert.InDelta(t, 0.5, u.Quantity, 1e-9)
	assert.InDelta(t, 50000.0*0.5, u.CostBasis, 1e-9, "CostBasis = EntryPrice * Quantity")
	assert.Equal(t, PositionStatusOpen, u.Status)
	assert.Nil(t, u.ClosedAt)
	assert.InDelta(t, 0.0, u.UnrealizedPnL, 1e-9, "nil UnrealizedPnL should default to 0")
	assert.InDelta(t, 0.0, u.RealizedPnL, 1e-9, "nil RealizedPnL should default to 0")
	assert.Equal(t, uuid.Nil, u.SessionID, "nil SessionID should default to uuid.Nil")
}

// TestBinancePositionToUnified_ClosedPosition tests conversion of a closed position with all
// optional fields populated.
func TestBinancePositionToUnified_ClosedPosition(t *testing.T) {
	posID := uuid.New()
	sessionID := uuid.New()
	entryTime := time.Now().Add(-2 * time.Hour)
	exitTime := time.Now().Add(-1 * time.Hour)
	exitPrice := 52000.0
	unrealizedPnL := 0.0
	realizedPnL := 1000.0
	entryReason := "technical-agent"

	p := &db.Position{
		ID:            posID,
		SessionID:     &sessionID,
		Symbol:        "ETH/USDT",
		Side:          db.PositionSideShort,
		EntryPrice:    3000.0,
		Quantity:      2.0,
		EntryTime:     entryTime,
		ExitTime:      &exitTime,
		ExitPrice:     &exitPrice,
		UnrealizedPnL: &unrealizedPnL,
		RealizedPnL:   &realizedPnL,
		EntryReason:   &entryReason,
	}

	u := BinancePositionToUnified(p)

	assert.Equal(t, posID, u.ID)
	assert.Equal(t, PlatformBinance, u.Platform)
	assert.Equal(t, "short", u.Side, "Side should be lowercased")
	assert.Equal(t, PositionStatusClosed, u.Status)
	assert.NotNil(t, u.ClosedAt)
	assert.Equal(t, exitTime, *u.ClosedAt)
	assert.InDelta(t, 52000.0, u.CurrentPrice, 1e-9)
	assert.InDelta(t, unrealizedPnL, u.UnrealizedPnL, 1e-9)
	assert.InDelta(t, realizedPnL, u.RealizedPnL, 1e-9)
	assert.Equal(t, sessionID, u.SessionID)
	assert.Equal(t, "technical-agent", u.Agent)
	assert.InDelta(t, 3000.0*2.0, u.CostBasis, 1e-9)
}

// TestBinanceTradeToUnified tests conversion of a Binance trade+order pair.
func TestBinanceTradeToUnified(t *testing.T) {
	tradeID := uuid.New()
	orderID := uuid.New()
	positionID := uuid.New()
	executedAt := time.Now()

	trade := &db.Trade{
		ID:            tradeID,
		OrderID:       orderID,
		Symbol:        "BTC/USDT",
		Side:          db.OrderSideBuy,
		Price:         48000.0,
		Quantity:      0.1,
		QuoteQuantity: 4800.0,
		ExecutedAt:    executedAt,
	}

	order := &db.Order{
		ID:         orderID,
		PositionID: &positionID,
	}

	u := BinanceTradeToUnified(trade, order)

	assert.Equal(t, tradeID, u.ID)
	assert.Equal(t, PlatformBinance, u.Platform)
	assert.Equal(t, "BTC/USDT", u.Symbol)
	assert.InDelta(t, 48000.0, u.Price, 1e-9)
	assert.InDelta(t, 0.1, u.Quantity, 1e-9)
	assert.InDelta(t, 4800.0, u.Amount, 1e-9)
	assert.Equal(t, positionID, u.PositionID)
	assert.Equal(t, "BUY", u.Action)
	assert.Equal(t, executedAt, u.Timestamp)
}

// TestBinanceTradeToUnified_NilOrder verifies that a nil order doesn't panic and PositionID
// is left as zero value.
func TestBinanceTradeToUnified_NilOrder(t *testing.T) {
	trade := &db.Trade{
		ID:     uuid.New(),
		Symbol: "SOL/USDT",
		Side:   db.OrderSideSell,
		Price:  100.0,
	}

	u := BinanceTradeToUnified(trade, nil)

	assert.Equal(t, PlatformBinance, u.Platform)
	assert.Equal(t, "SOL/USDT", u.Symbol)
	assert.Equal(t, uuid.Nil, u.PositionID, "PositionID should be zero when order is nil")
	assert.Equal(t, "SELL", u.Action)
}

// TestNewEmptyPortfolio verifies the portfolio is non-nil with an empty positions slice.
func TestNewEmptyPortfolio(t *testing.T) {
	p := NewEmptyPortfolio()

	require.NotNil(t, p)
	require.NotNil(t, p.Positions)
	assert.Empty(t, p.Positions)
	require.NotNil(t, p.ByPlatform)
	assert.Contains(t, p.ByPlatform, PlatformBinance)
	assert.Contains(t, p.ByPlatform, PlatformPolymarket)
	assert.Equal(t, 0, p.OpenPositions)
	assert.InDelta(t, 0.0, p.TotalValue, 1e-9)
}

// TestUnifiedPortfolio_AddPosition verifies that adding a position updates the portfolio.
func TestUnifiedPortfolio_AddPosition(t *testing.T) {
	p := NewEmptyPortfolio()

	pos := UnifiedPosition{
		ID:            uuid.New(),
		Platform:      PlatformBinance,
		Symbol:        "BTC/USDT",
		EntryPrice:    50000.0,
		CurrentPrice:  51000.0,
		Quantity:      1.0,
		UnrealizedPnL: 1000.0,
		RealizedPnL:   500.0,
		Status:        PositionStatusOpen,
	}

	p.AddPosition(pos)

	require.Len(t, p.Positions, 1)
	assert.Equal(t, pos, p.Positions[0])
	assert.Equal(t, 1, p.OpenPositions)
	assert.InDelta(t, 1000.0, p.UnrealizedPnL, 1e-9)
	assert.InDelta(t, 500.0, p.RealizedPnL, 1e-9)
	assert.InDelta(t, 1500.0, p.TotalPnL, 1e-9)
	assert.InDelta(t, 51000.0, p.TotalValue, 1e-9, "TotalValue = CurrentPrice * Quantity")

	binanceSummary := p.ByPlatform[PlatformBinance]
	assert.Equal(t, 1, binanceSummary.PositionCount)
	assert.InDelta(t, 51000.0, binanceSummary.TotalValue, 1e-9)
}

// TestUnifiedPortfolio_AddPosition_Closed verifies that adding a closed position does not
// increment OpenPositions or UnrealizedPnL.
func TestUnifiedPortfolio_AddPosition_Closed(t *testing.T) {
	p := NewEmptyPortfolio()

	closedAt := time.Now()
	pos := UnifiedPosition{
		ID:           uuid.New(),
		Platform:     PlatformBinance,
		Symbol:       "ETH/USDT",
		CurrentPrice: 3000.0,
		Quantity:     1.0,
		RealizedPnL:  200.0,
		Status:       PositionStatusClosed,
		ClosedAt:     &closedAt,
	}

	p.AddPosition(pos)

	assert.Equal(t, 0, p.OpenPositions)
	assert.InDelta(t, 0.0, p.UnrealizedPnL, 1e-9)
	assert.InDelta(t, 200.0, p.RealizedPnL, 1e-9)
}

// TestUnifiedPortfolio_SetPlatformTradeCount verifies that trade counts are stored and
// TotalTrades is updated correctly.
func TestUnifiedPortfolio_SetPlatformTradeCount(t *testing.T) {
	p := NewEmptyPortfolio()

	p.SetPlatformTradeCount(PlatformBinance, 10)
	p.SetPlatformTradeCount(PlatformPolymarket, 5)

	assert.Equal(t, 10, p.ByPlatform[PlatformBinance].TradeCount)
	assert.Equal(t, 5, p.ByPlatform[PlatformPolymarket].TradeCount)
	assert.Equal(t, 15, p.TotalTrades)

	// Updating one platform recalculates total correctly.
	p.SetPlatformTradeCount(PlatformBinance, 20)
	assert.Equal(t, 20, p.ByPlatform[PlatformBinance].TradeCount)
	assert.Equal(t, 25, p.TotalTrades)
}

// TestUnifiedPosition_IsOpen tests the IsOpen method for both open and closed positions.
func TestUnifiedPosition_IsOpen(t *testing.T) {
	open := &UnifiedPosition{
		Status: PositionStatusOpen,
	}
	assert.True(t, open.IsOpen(), "position with status OPEN should return true from IsOpen")

	closedAt := time.Now()
	closed := &UnifiedPosition{
		Status:   PositionStatusClosed,
		ClosedAt: &closedAt,
	}
	assert.False(t, closed.IsOpen(), "position with status CLOSED should return false from IsOpen")
}
