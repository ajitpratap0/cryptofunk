package exchange

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// PositionManager handles position tracking and P&L calculation
type PositionManager struct {
	db               *db.DB
	mu               sync.RWMutex
	openPositions    map[string]*db.Position // symbol -> position
	currentSessionID *uuid.UUID
	feeRate          float64                 // Average fee rate for calculations
}

// NewPositionManager creates a new position manager with default fee rate
func NewPositionManager(database *db.DB) *PositionManager {
	// Default to 0.1% fee (average of maker/taker)
	return NewPositionManagerWithFees(database, 0.001)
}

// NewPositionManagerWithFees creates a new position manager with custom fee configuration
func NewPositionManagerWithFees(database *db.DB, feeRate float64) *PositionManager {
	return &PositionManager{
		db:            database,
		openPositions: make(map[string]*db.Position),
		feeRate:       feeRate,
	}
}

// SetSession sets the current trading session
func (pm *PositionManager) SetSession(sessionID *uuid.UUID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.currentSessionID = sessionID

	// Load open positions for this session (only if database is available)
	if sessionID != nil && pm.db != nil {
		pm.loadOpenPositions(*sessionID)
	} else {
		pm.openPositions = make(map[string]*db.Position)
	}
}

// loadOpenPositions loads open positions from database
func (pm *PositionManager) loadOpenPositions(sessionID uuid.UUID) {
	ctx := context.Background()
	positions, err := pm.db.GetOpenPositions(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load open positions")
		return
	}

	pm.openPositions = make(map[string]*db.Position)
	for _, pos := range positions {
		pm.openPositions[pos.Symbol] = pos
	}

	log.Info().
		Int("count", len(positions)).
		Msg("Loaded open positions from database")
}

// OnOrderFilled handles order fill events and updates positions
func (pm *PositionManager) OnOrderFilled(ctx context.Context, order *Order, fills []Fill) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.currentSessionID == nil {
		return fmt.Errorf("no active session")
	}

	// Calculate average fill price and total fees
	var totalValue float64
	var totalQty float64
	var totalFees float64

	for _, fill := range fills {
		totalValue += fill.Price * fill.Quantity
		totalQty += fill.Quantity
		// Use configured fee rate
		totalFees += fill.Price * fill.Quantity * pm.feeRate
	}

	avgFillPrice := totalValue / totalQty

	// Check if we have an existing position for this symbol
	existingPos, hasPosition := pm.openPositions[order.Symbol]

	if order.Side == OrderSideBuy {
		// BUY order
		if hasPosition {
			if existingPos.Side == db.PositionSideShort {
				// Closing or reducing SHORT position
				if totalQty >= existingPos.Quantity {
					// Fully closing SHORT position
					closeQty := existingPos.Quantity
					err := pm.closePosition(ctx, existingPos, avgFillPrice, "Closed by BUY order", totalFees)
					if err != nil {
						return err
					}

					// If quantity > existing position, open new LONG position
					if totalQty > closeQty {
						remainingQty := totalQty - closeQty
						err := pm.openPosition(ctx, order.Symbol, db.PositionSideLong, avgFillPrice, remainingQty, "Opened after closing SHORT", totalFees)
						if err != nil {
							return err
						}
					}
				} else {
					// Partially closing SHORT position
					err := pm.partialClosePosition(ctx, existingPos, totalQty, avgFillPrice, "Partially closed by BUY order", totalFees)
					if err != nil {
						return err
					}
				}
			} else {
				// Adding to LONG position (position averaging)
				err := pm.averagePosition(ctx, existingPos, avgFillPrice, totalQty, totalFees)
				if err != nil {
					return err
				}
			}
		} else {
			// Opening new LONG position
			err := pm.openPosition(ctx, order.Symbol, db.PositionSideLong, avgFillPrice, totalQty, "Opened by BUY order", totalFees)
			if err != nil {
				return err
			}
		}
	} else {
		// SELL order
		if hasPosition {
			if existingPos.Side == db.PositionSideLong {
				// Closing or reducing LONG position
				if totalQty >= existingPos.Quantity {
					// Fully closing LONG position
					closeQty := existingPos.Quantity
					err := pm.closePosition(ctx, existingPos, avgFillPrice, "Closed by SELL order", totalFees)
					if err != nil {
						return err
					}

					// If quantity > existing position, open new SHORT position
					if totalQty > closeQty {
						remainingQty := totalQty - closeQty
						err := pm.openPosition(ctx, order.Symbol, db.PositionSideShort, avgFillPrice, remainingQty, "Opened after closing LONG", totalFees)
						if err != nil {
							return err
						}
					}
				} else {
					// Partially closing LONG position
					err := pm.partialClosePosition(ctx, existingPos, totalQty, avgFillPrice, "Partially closed by SELL order", totalFees)
					if err != nil {
						return err
					}
				}
			} else {
				// Adding to SHORT position (position averaging)
				err := pm.averagePosition(ctx, existingPos, avgFillPrice, totalQty, totalFees)
				if err != nil {
					return err
				}
			}
		} else {
			// Opening new SHORT position
			err := pm.openPosition(ctx, order.Symbol, db.PositionSideShort, avgFillPrice, totalQty, "Opened by SELL order", totalFees)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// openPosition creates a new position
func (pm *PositionManager) openPosition(ctx context.Context, symbol string, side db.PositionSide, entryPrice, quantity float64, reason string, fees float64) error {
	position := &db.Position{
		ID:          uuid.New(),
		SessionID:   pm.currentSessionID,
		Symbol:      symbol,
		Exchange:    "PAPER", // TODO: Use actual exchange name
		Side:        side,
		EntryPrice:  entryPrice,
		Quantity:    quantity,
		EntryTime:   time.Now(),
		Fees:        fees,
		EntryReason: &reason,
	}

	// Store in memory
	pm.openPositions[symbol] = position

	// Create in database (if available)
	if pm.db != nil {
		err := pm.db.CreatePosition(ctx, position)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create position in database")
			return err
		}
	}

	log.Info().
		Str("position_id", position.ID.String()).
		Str("symbol", symbol).
		Str("side", string(side)).
		Float64("entry_price", entryPrice).
		Float64("quantity", quantity).
		Msg("Position opened")

	return nil
}

// closePosition closes an existing position
func (pm *PositionManager) closePosition(ctx context.Context, position *db.Position, exitPrice float64, reason string, fees float64) error {
	// Remove from memory
	delete(pm.openPositions, position.Symbol)

	// Close in database (if available)
	if pm.db != nil {
		err := pm.db.ClosePosition(ctx, position.ID, exitPrice, reason, fees)
		if err != nil {
			log.Error().Err(err).Msg("Failed to close position in database")
			return err
		}
	}

	// Calculate realized P&L for logging
	var realizedPnL float64
	if position.Side == db.PositionSideLong {
		realizedPnL = (exitPrice - position.EntryPrice) * position.Quantity
	} else {
		realizedPnL = (position.EntryPrice - exitPrice) * position.Quantity
	}
	realizedPnL -= fees

	log.Info().
		Str("position_id", position.ID.String()).
		Str("symbol", position.Symbol).
		Str("side", string(position.Side)).
		Float64("entry_price", position.EntryPrice).
		Float64("exit_price", exitPrice).
		Float64("realized_pnl", realizedPnL).
		Msg("Position closed")

	return nil
}

// partialClosePosition partially closes a position
func (pm *PositionManager) partialClosePosition(ctx context.Context, position *db.Position, closeQuantity, exitPrice float64, reason string, fees float64) error {
	// Use database method for partial close
	if pm.db != nil {
		closedPos, err := pm.db.PartialClosePosition(ctx, position.ID, closeQuantity, exitPrice, reason, fees)
		if err != nil {
			log.Error().Err(err).Msg("Failed to partial close position in database")
			return err
		}

		// Update in-memory position
		position.Quantity -= closeQuantity
		position.Fees += fees

		log.Info().
			Str("position_id", position.ID.String()).
			Str("closed_position_id", closedPos.ID.String()).
			Str("symbol", position.Symbol).
			Str("side", string(position.Side)).
			Float64("close_quantity", closeQuantity).
			Float64("remaining_quantity", position.Quantity).
			Float64("exit_price", exitPrice).
			Float64("realized_pnl", *closedPos.RealizedPnL).
			Msg("Position partially closed")
	}

	return nil
}

// averagePosition adds to an existing position with price averaging
func (pm *PositionManager) averagePosition(ctx context.Context, position *db.Position, newPrice, newQuantity, fees float64) error {
	// Calculate new average entry price
	totalValue := (position.EntryPrice * position.Quantity) + (newPrice * newQuantity)
	totalQuantity := position.Quantity + newQuantity
	newAvgPrice := totalValue / totalQuantity

	// Update in-memory position
	oldPrice := position.EntryPrice
	oldQuantity := position.Quantity
	position.EntryPrice = newAvgPrice
	position.Quantity = totalQuantity
	position.Fees += fees

	// Update in database (if available)
	if pm.db != nil {
		err := pm.db.UpdatePositionAveraging(ctx, position.ID, newAvgPrice, totalQuantity, fees)
		if err != nil {
			// Rollback in-memory changes
			position.EntryPrice = oldPrice
			position.Quantity = oldQuantity
			position.Fees -= fees

			log.Error().Err(err).Msg("Failed to update position averaging in database")
			return err
		}
	}

	log.Info().
		Str("position_id", position.ID.String()).
		Str("symbol", position.Symbol).
		Str("side", string(position.Side)).
		Float64("old_entry_price", oldPrice).
		Float64("new_entry_price", newAvgPrice).
		Float64("old_quantity", oldQuantity).
		Float64("new_quantity", totalQuantity).
		Float64("added_quantity", newQuantity).
		Msg("Position averaged")

	return nil
}

// UpdateUnrealizedPnL updates unrealized P&L for all open positions
func (pm *PositionManager) UpdateUnrealizedPnL(ctx context.Context, prices map[string]float64) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for symbol, position := range pm.openPositions {
		currentPrice, ok := prices[symbol]
		if !ok {
			continue
		}

		// Update in database (if available)
		if pm.db != nil {
			err := pm.db.UpdateUnrealizedPnL(ctx, position.ID, currentPrice)
			if err != nil {
				log.Error().
					Err(err).
					Str("symbol", symbol).
					Msg("Failed to update unrealized P&L")
				continue
			}
		}

		// Update in-memory position
		var unrealizedPnL float64
		if position.Side == db.PositionSideLong {
			unrealizedPnL = (currentPrice - position.EntryPrice) * position.Quantity
		} else {
			unrealizedPnL = (position.EntryPrice - currentPrice) * position.Quantity
		}
		position.UnrealizedPnL = &unrealizedPnL
	}

	return nil
}

// GetOpenPositions returns all open positions
func (pm *PositionManager) GetOpenPositions() []*db.Position {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	positions := make([]*db.Position, 0, len(pm.openPositions))
	for _, pos := range pm.openPositions {
		positions = append(positions, pos)
	}

	return positions
}

// GetPosition returns a specific position by symbol
func (pm *PositionManager) GetPosition(symbol string) (*db.Position, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pos, ok := pm.openPositions[symbol]
	return pos, ok
}

// GetTotalUnrealizedPnL calculates total unrealized P&L across all positions
func (pm *PositionManager) GetTotalUnrealizedPnL() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var total float64
	for _, pos := range pm.openPositions {
		if pos.UnrealizedPnL != nil {
			total += *pos.UnrealizedPnL
		}
	}

	return total
}
