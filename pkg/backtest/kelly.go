package backtest

import (
	"context"
	"fmt"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TradingStats holds statistical data for Kelly Criterion calculation
type TradingStats struct {
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	AvgWin        float64 `json:"avg_win"`        // Average profit per winning trade
	AvgLoss       float64 `json:"avg_loss"`       // Average loss per losing trade (positive value)
	WinRate       float64 `json:"win_rate"`       // Percentage of winning trades (0.0 to 1.0)
	AvgReturn     float64 `json:"avg_return"`     // Average return per trade
	TotalProfit   float64 `json:"total_profit"`   // Total profit from all winning trades
	TotalLoss     float64 `json:"total_loss"`     // Total loss from all losing trades (positive value)
	LargestWin    float64 `json:"largest_win"`    // Largest single win
	LargestLoss   float64 `json:"largest_loss"`   // Largest single loss (positive value)
	WinLossRatio  float64 `json:"win_loss_ratio"` // AvgWin / AvgLoss
}

// KellyCalculator calculates optimal position sizes using Kelly Criterion
type KellyCalculator struct {
	db *db.DB
}

// NewKellyCalculator creates a new Kelly Criterion calculator
func NewKellyCalculator(database *db.DB) *KellyCalculator {
	return &KellyCalculator{
		db: database,
	}
}

// CalculateStats computes trading statistics from historical data
// This can be used for database-backed statistics
func (kc *KellyCalculator) CalculateStats(ctx context.Context, sessionID uuid.UUID) (*TradingStats, error) {
	if kc.db == nil {
		return nil, fmt.Errorf("database connection required for historical stats")
	}

	query := `
		SELECT
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE realized_pl > 0) as winning_trades,
			COUNT(*) FILTER (WHERE realized_pl <= 0) as losing_trades,
			COALESCE(AVG(realized_pl) FILTER (WHERE realized_pl > 0), 0) as avg_win,
			COALESCE(ABS(AVG(realized_pl)) FILTER (WHERE realized_pl <= 0), 0) as avg_loss,
			COALESCE(SUM(realized_pl) FILTER (WHERE realized_pl > 0), 0) as total_profit,
			COALESCE(ABS(SUM(realized_pl)) FILTER (WHERE realized_pl <= 0), 0) as total_loss,
			COALESCE(MAX(realized_pl), 0) as largest_win,
			COALESCE(ABS(MIN(realized_pl)), 0) as largest_loss
		FROM positions
		WHERE session_id = $1
		  AND status = 'CLOSED'
		  AND realized_pl IS NOT NULL
	`

	var stats TradingStats
	row := kc.db.Pool().QueryRow(ctx, query, sessionID)

	err := row.Scan(
		&stats.TotalTrades,
		&stats.WinningTrades,
		&stats.LosingTrades,
		&stats.AvgWin,
		&stats.AvgLoss,
		&stats.TotalProfit,
		&stats.TotalLoss,
		&stats.LargestWin,
		&stats.LargestLoss,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query trading stats: %w", err)
	}

	// Calculate derived statistics
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinningTrades) / float64(stats.TotalTrades)
		stats.AvgReturn = (stats.TotalProfit - stats.TotalLoss) / float64(stats.TotalTrades)
	}

	if stats.AvgLoss > 0 {
		stats.WinLossRatio = stats.AvgWin / stats.AvgLoss
	}

	return &stats, nil
}

// CalculateStatsFromTrades computes trading statistics from in-memory trades
// This is useful for backtesting where we don't have database access
func CalculateStatsFromTrades(closedPositions []*ClosedPosition) *TradingStats {
	stats := &TradingStats{}

	if len(closedPositions) == 0 {
		return stats
	}

	stats.TotalTrades = len(closedPositions)

	for _, position := range closedPositions {
		pl := position.RealizedPL

		if pl > 0 {
			stats.WinningTrades++
			stats.TotalProfit += pl
			if pl > stats.LargestWin {
				stats.LargestWin = pl
			}
		} else {
			stats.LosingTrades++
			absLoss := -pl // Convert to positive
			stats.TotalLoss += absLoss
			if absLoss > stats.LargestLoss {
				stats.LargestLoss = absLoss
			}
		}
	}

	// Calculate averages
	if stats.WinningTrades > 0 {
		stats.AvgWin = stats.TotalProfit / float64(stats.WinningTrades)
	}

	if stats.LosingTrades > 0 {
		stats.AvgLoss = stats.TotalLoss / float64(stats.LosingTrades)
	}

	// Calculate win rate
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinningTrades) / float64(stats.TotalTrades)
		stats.AvgReturn = (stats.TotalProfit - stats.TotalLoss) / float64(stats.TotalTrades)
	}

	// Calculate win/loss ratio
	if stats.AvgLoss > 0 {
		stats.WinLossRatio = stats.AvgWin / stats.AvgLoss
	}

	return stats
}

// CalculatePositionSize calculates optimal position size using Kelly Criterion
//
// Kelly Criterion Formula:
// f* = (p * b - q) / b
//
// Where:
// - f* = fraction of capital to bet (Kelly percentage)
// - p = probability of winning (win rate)
// - q = probability of losing (1 - p)
// - b = ratio of average win to average loss (win/loss ratio)
//
// The formula can also be written as:
// f* = (W - L) / W
// Where W = average win, L = average loss
//
// Kelly fraction is applied to reduce risk (typically 0.25 to 0.5 for quarter or half Kelly)
func (kc *KellyCalculator) CalculatePositionSize(
	stats *TradingStats,
	capital float64,
	kellyFraction float64,
) float64 {
	// Default to 10% if not enough data
	if stats.TotalTrades < 30 {
		log.Debug().
			Int("total_trades", stats.TotalTrades).
			Msg("Not enough historical trades for Kelly Criterion - using conservative 10%")
		return capital * 0.10
	}

	// Validate win rate
	if stats.WinRate <= 0 || stats.WinRate >= 1 {
		log.Warn().
			Float64("win_rate", stats.WinRate).
			Msg("Invalid win rate - using conservative 10%")
		return capital * 0.10
	}

	// Validate average win and loss
	if stats.AvgWin <= 0 || stats.AvgLoss <= 0 {
		log.Warn().
			Float64("avg_win", stats.AvgWin).
			Float64("avg_loss", stats.AvgLoss).
			Msg("Invalid average win/loss - using conservative 10%")
		return capital * 0.10
	}

	// Calculate Kelly percentage using the formula: f* = (p * b - q) / b
	// Where b = win/loss ratio, p = win rate, q = loss rate
	p := stats.WinRate
	q := 1 - p
	b := stats.WinLossRatio

	kellyPercent := (p*b - q) / b

	log.Debug().
		Float64("win_rate", p).
		Float64("loss_rate", q).
		Float64("win_loss_ratio", b).
		Float64("raw_kelly_percent", kellyPercent).
		Msg("Kelly Criterion calculation")

	// Handle negative Kelly (negative expected value - should not trade)
	if kellyPercent <= 0 {
		log.Warn().
			Float64("kelly_percent", kellyPercent).
			Msg("Negative Kelly percentage - no positive edge, using minimal 1%")
		return capital * 0.01
	}

	// Apply Kelly fraction to be more conservative
	// Full Kelly can be very aggressive and lead to large drawdowns
	adjustedKelly := kellyPercent * kellyFraction

	// Cap at 25% of capital to prevent over-leveraging
	if adjustedKelly > 0.25 {
		log.Warn().
			Float64("adjusted_kelly", adjustedKelly).
			Msg("Kelly percentage exceeds 25% cap - capping at 25%")
		adjustedKelly = 0.25
	}

	// Floor at 1% of capital to ensure some position
	if adjustedKelly < 0.01 {
		adjustedKelly = 0.01
	}

	positionSize := capital * adjustedKelly

	log.Info().
		Int("total_trades", stats.TotalTrades).
		Float64("win_rate", stats.WinRate*100).
		Float64("avg_win", stats.AvgWin).
		Float64("avg_loss", stats.AvgLoss).
		Float64("win_loss_ratio", stats.WinLossRatio).
		Float64("kelly_percent", kellyPercent*100).
		Float64("kelly_fraction", kellyFraction).
		Float64("adjusted_percent", adjustedKelly*100).
		Float64("capital", capital).
		Float64("position_size", positionSize).
		Msg("Kelly Criterion position sizing")

	return positionSize
}

// GetRecommendation provides interpretation of Kelly percentage
func GetRecommendation(kellyPercent float64) string {
	percent := kellyPercent * 100

	if percent <= 0 {
		return "No position recommended - negative edge (expected value < 0)"
	} else if percent <= 2 {
		return "Very small position - minimal edge"
	} else if percent <= 5 {
		return "Conservative position - moderate edge"
	} else if percent <= 10 {
		return "Standard position - good edge"
	} else if percent <= 20 {
		return "Large position - strong edge (monitor risk carefully)"
	} else if percent <= 30 {
		return "Very large position - exceptional edge (high risk/reward)"
	} else {
		return "Warning: Extremely large position suggested - verify calculations and strongly consider reducing Kelly fraction"
	}
}
