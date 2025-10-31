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
}

// NewPositionManager creates a new position manager
func NewPositionManager(database *db.DB) *PositionManager {
	return &PositionManager{
		db:            database,
		openPositions: make(map[string]*db.Position),
	}
}

// SetSession sets the current trading session
func (pm *PositionManager) SetSession(sessionID *uuid.UUID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.currentSessionID = sessionID

	// Load open positions for this session
	if sessionID != nil {
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
		// Assume 0.1% maker/taker fee for paper trading
		totalFees += fill.Price * fill.Quantity * 0.001
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
					// TODO: Handle partial closes (requires position quantity tracking)
					log.Warn().Msg("Partial position close not fully implemented")
				}
			} else {
				// Adding to LONG position (not implemented - would require averaging)
				log.Warn().Msg("Adding to existing LONG position not implemented")
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
					log.Warn().Msg("Partial position close not fully implemented")
				}
			} else {
				// Adding to SHORT position
				log.Warn().Msg("Adding to existing SHORT position not implemented")
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

	// Create in database
	err := pm.db.CreatePosition(ctx, position)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create position in database")
		return err
	}

	// Store in memory
	pm.openPositions[symbol] = position

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
	// Close in database (calculates realized P&L)
	err := pm.db.ClosePosition(ctx, position.ID, exitPrice, reason, fees)
	if err != nil {
		log.Error().Err(err).Msg("Failed to close position in database")
		return err
	}

	// Remove from memory
	delete(pm.openPositions, position.Symbol)

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

// UpdateUnrealizedPnL updates unrealized P&L for all open positions
func (pm *PositionManager) UpdateUnrealizedPnL(ctx context.Context, prices map[string]float64) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for symbol, position := range pm.openPositions {
		currentPrice, ok := prices[symbol]
		if !ok {
			continue
		}

		err := pm.db.UpdateUnrealizedPnL(ctx, position.ID, currentPrice)
		if err != nil {
			log.Error().
				Err(err).
				Str("symbol", symbol).
				Msg("Failed to update unrealized P&L")
			continue
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
