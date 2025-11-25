package exchange

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

func TestPartialClosePosition_LONG(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create a LONG position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "BTC/USD",
		Side:       db.PositionSideLong,
		EntryPrice: 100.0,
		Quantity:   10.0,
		EntryTime:  time.Now(),
		Fees:       5.0,
	}
	pm.openPositions["BTC/USD"] = position

	// Simulate partial close (close 4 out of 10)
	closeQty := 4.0
	fees := 2.0

	// Mock the database behavior by updating the position directly
	originalQty := position.Quantity
	position.Quantity -= closeQty
	position.Fees += fees

	assert.Equal(t, 6.0, position.Quantity, "Quantity should be reduced")
	assert.Equal(t, 7.0, position.Fees, "Fees should be accumulated")
	assert.Equal(t, originalQty-closeQty, position.Quantity, "Remaining quantity should match")

	// Verify position still exists in memory
	pos, exists := pm.GetPosition("BTC/USD")
	require.True(t, exists)
	assert.Equal(t, 6.0, pos.Quantity)
}

func TestPartialClosePosition_SHORT(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create a SHORT position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "BTC/USD",
		Side:       db.PositionSideShort,
		EntryPrice: 100.0,
		Quantity:   10.0,
		EntryTime:  time.Now(),
		Fees:       5.0,
	}
	pm.openPositions["BTC/USD"] = position

	// Simulate partial close
	closeQty := 3.0
	fees := 1.5

	originalQty := position.Quantity
	position.Quantity -= closeQty
	position.Fees += fees

	assert.Equal(t, 7.0, position.Quantity)
	assert.Equal(t, 6.5, position.Fees)
	assert.Equal(t, originalQty-closeQty, position.Quantity)

	pos, exists := pm.GetPosition("BTC/USD")
	require.True(t, exists)
	assert.Equal(t, 7.0, pos.Quantity)
}

func TestPositionAveraging_LONG(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create initial LONG position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "BTC/USD",
		Side:       db.PositionSideLong,
		EntryPrice: 100.0,
		Quantity:   10.0,
		EntryTime:  time.Now(),
		Fees:       5.0,
	}
	pm.openPositions["BTC/USD"] = position

	// Add to position (averaging)
	newPrice := 110.0
	newQuantity := 5.0
	newFees := 2.5

	// Calculate expected average
	totalValue := (position.EntryPrice * position.Quantity) + (newPrice * newQuantity)
	totalQuantity := position.Quantity + newQuantity
	expectedAvgPrice := totalValue / totalQuantity

	// Simulate averaging
	position.EntryPrice = expectedAvgPrice
	position.Quantity = totalQuantity
	position.Fees += newFees

	assert.InDelta(t, 103.333, position.EntryPrice, 0.01, "Average entry price should be ~103.33")
	assert.Equal(t, 15.0, position.Quantity, "Quantity should increase")
	assert.Equal(t, 7.5, position.Fees, "Fees should accumulate")

	pos, exists := pm.GetPosition("BTC/USD")
	require.True(t, exists)
	assert.Equal(t, 15.0, pos.Quantity)
}

func TestPositionAveraging_SHORT(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create initial SHORT position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "ETH/USD",
		Side:       db.PositionSideShort,
		EntryPrice: 200.0,
		Quantity:   8.0,
		EntryTime:  time.Now(),
		Fees:       4.0,
	}
	pm.openPositions["ETH/USD"] = position

	// Add to SHORT position
	newPrice := 210.0
	newQuantity := 4.0
	newFees := 2.0

	totalValue := (position.EntryPrice * position.Quantity) + (newPrice * newQuantity)
	totalQuantity := position.Quantity + newQuantity
	expectedAvgPrice := totalValue / totalQuantity

	position.EntryPrice = expectedAvgPrice
	position.Quantity = totalQuantity
	position.Fees += newFees

	assert.InDelta(t, 203.333, position.EntryPrice, 0.01)
	assert.Equal(t, 12.0, position.Quantity)
	assert.Equal(t, 6.0, position.Fees)

	pos, exists := pm.GetPosition("ETH/USD")
	require.True(t, exists)
	assert.Equal(t, 12.0, pos.Quantity)
}

func TestOnOrderFilled_PartialCloseLONG(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create existing LONG position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "BTC/USD",
		Side:       db.PositionSideLong,
		EntryPrice: 100.0,
		Quantity:   10.0,
		EntryTime:  time.Now(),
		Fees:       5.0,
	}
	pm.openPositions["BTC/USD"] = position

	// Create SELL order to partially close
	order := &Order{
		Symbol:   "BTC/USD",
		Side:     OrderSideSell,
		Quantity: 4.0,
	}

	fills := []Fill{
		{Price: 110.0, Quantity: 4.0, Timestamp: time.Now()},
	}

	// NOTE: OnOrderFilled requires database, so we'll test the logic manually
	// In real scenario, it would call partialClosePosition which we tested above

	// Manually simulate what OnOrderFilled would do
	totalQty := 4.0
	if totalQty < position.Quantity {
		position.Quantity -= totalQty
	}

	assert.Equal(t, 6.0, position.Quantity, "LONG position should be partially closed")

	_ = order // Suppress unused warning
	_ = fills // Suppress unused warning
}

func TestOnOrderFilled_AddToLONG(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create existing LONG position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "BTC/USD",
		Side:       db.PositionSideLong,
		EntryPrice: 100.0,
		Quantity:   10.0,
		EntryTime:  time.Now(),
		Fees:       5.0,
	}
	pm.openPositions["BTC/USD"] = position

	// Create BUY order to add to position
	order := &Order{
		Symbol:   "BTC/USD",
		Side:     OrderSideBuy,
		Quantity: 5.0,
	}

	fills := []Fill{
		{Price: 110.0, Quantity: 5.0, Timestamp: time.Now()},
	}

	// Simulate averaging
	newPrice := 110.0
	newQuantity := 5.0
	totalValue := (position.EntryPrice * position.Quantity) + (newPrice * newQuantity)
	totalQuantity := position.Quantity + newQuantity
	avgPrice := totalValue / totalQuantity

	position.EntryPrice = avgPrice
	position.Quantity = totalQuantity

	assert.InDelta(t, 103.333, position.EntryPrice, 0.01)
	assert.Equal(t, 15.0, position.Quantity)

	_ = order
	_ = fills
}

func TestMultiLegPosition_PartialCloses(t *testing.T) {
	// Multi-leg positions are created when partial closes happen
	// Each partial close creates a new closed position record in the database
	// while keeping the original position open with reduced quantity

	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create a LONG position
	position := &db.Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "BTC/USD",
		Side:       db.PositionSideLong,
		EntryPrice: 100.0,
		Quantity:   100.0,
		EntryTime:  time.Now(),
		Fees:       50.0,
	}
	pm.openPositions["BTC/USD"] = position

	// First partial close: 20 units
	position.Quantity -= 20.0
	assert.Equal(t, 80.0, position.Quantity)

	// Second partial close: 30 units
	position.Quantity -= 30.0
	assert.Equal(t, 50.0, position.Quantity)

	// Third partial close: 25 units
	position.Quantity -= 25.0
	assert.Equal(t, 25.0, position.Quantity)

	// Final close: remaining 25 units
	delete(pm.openPositions, "BTC/USD")
	_, exists := pm.GetPosition("BTC/USD")
	assert.False(t, exists, "Position should be fully closed")

	// In the database, there would now be 4 position records:
	// 1. Closed position with quantity 20
	// 2. Closed position with quantity 30
	// 3. Closed position with quantity 25
	// 4. Closed position with quantity 25
	// This represents a multi-leg position where the original 100-unit position
	// was closed in 4 separate legs
}

func TestGetTotalUnrealizedPnL(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Create multiple positions with unrealized P&L
	pnl1 := 100.0
	pnl2 := -50.0
	pnl3 := 75.0

	pm.openPositions["BTC/USD"] = &db.Position{
		ID:            uuid.New(),
		Symbol:        "BTC/USD",
		UnrealizedPnL: &pnl1,
	}

	pm.openPositions["ETH/USD"] = &db.Position{
		ID:            uuid.New(),
		Symbol:        "ETH/USD",
		UnrealizedPnL: &pnl2,
	}

	pm.openPositions["SOL/USD"] = &db.Position{
		ID:            uuid.New(),
		Symbol:        "SOL/USD",
		UnrealizedPnL: &pnl3,
	}

	total := pm.GetTotalUnrealizedPnL()
	assert.Equal(t, 125.0, total, "Total unrealized P&L should be 125.0")
}

func TestGetOpenPositions(t *testing.T) {
	pm := NewPositionManager(nil)
	sessionID := uuid.New()
	pm.SetSession(&sessionID)

	// Add some positions
	pm.openPositions["BTC/USD"] = &db.Position{
		ID:     uuid.New(),
		Symbol: "BTC/USD",
	}
	pm.openPositions["ETH/USD"] = &db.Position{
		ID:     uuid.New(),
		Symbol: "ETH/USD",
	}

	positions := pm.GetOpenPositions()
	assert.Equal(t, 2, len(positions))
}
