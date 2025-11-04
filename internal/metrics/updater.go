package metrics

import (
	"context"
	"math"
	"time

"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Updater periodically updates metrics from the database
type Updater struct {
	db       *pgxpool.Pool
	interval time.Duration
	stopCh   chan struct{}
}

// NewUpdater creates a new metrics updater
func NewUpdater(db *pgxpool.Pool, interval time.Duration) *Updater {
	return &Updater{
		db:       db,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the metrics update loop
func (u *Updater) Start(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	// Update immediately on start
	u.update(ctx)

	for {
		select {
		case <-ticker.C:
			u.update(ctx)
		case <-u.stopCh:
			log.Info().Msg("Metrics updater stopped")
			return
		case <-ctx.Done():
			log.Info().Msg("Metrics updater context cancelled")
			return
		}
	}
}

// Stop stops the metrics updater
func (u *Updater) Stop() {
	close(u.stopCh)
}

// update fetches and updates all metrics
func (u *Updater) update(ctx context.Context) {
	log.Debug().Msg("Updating metrics from database")

	// Update trading performance metrics
	u.updateTradingMetrics(ctx)

	// Update position metrics
	u.updatePositionMetrics(ctx)

	// Update agent metrics
	u.updateAgentMetrics(ctx)

	// Update database pool metrics
	u.updateDatabaseMetrics()

	log.Debug().Msg("Metrics updated successfully")
}

// updateTradingMetrics updates trading performance metrics
func (u *Updater) updateTradingMetrics(ctx context.Context) {
	// Calculate total P&L, win rate, total trades
	var totalPnL float64
	var totalTrades, winningTrades int64

	query := `
		SELECT
			COALESCE(SUM(pnl), 0) as total_pnl,
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE pnl > 0) as winning_trades
		FROM trades
		WHERE status = 'FILLED' AND exit_time IS NOT NULL
	`

	err := u.db.QueryRow(ctx, query).Scan(&totalPnL, &totalTrades, &winningTrades)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch trading metrics")
		return
	}

	// Update metrics
	TotalPnL.Set(totalPnL)

	if totalTrades > 0 {
		winRate := float64(winningTrades) / float64(totalTrades)
		WinRate.Set(winRate)
	} else {
		WinRate.Set(0)
	}

	// Calculate risk/reward ratio
	var avgWin, avgLoss float64
	query = `
		SELECT
			COALESCE(AVG(pnl) FILTER (WHERE pnl > 0), 0) as avg_win,
			COALESCE(ABS(AVG(pnl)) FILTER (WHERE pnl < 0), 0) as avg_loss
		FROM trades
		WHERE status = 'FILLED' AND exit_time IS NOT NULL
	`

	err = u.db.QueryRow(ctx, query).Scan(&avgWin, &avgLoss)
	if err == nil && avgLoss > 0 {
		RiskRewardRatio.Set(avgWin / avgLoss)
	}

	// Calculate drawdown
	u.updateDrawdownMetrics(ctx)

	// Calculate returns
	u.updateReturnMetrics(ctx)

	// Calculate Sharpe ratio
	u.updateSharpeRatio(ctx)
}

// updateDrawdownMetrics calculates current drawdown
func (u *Updater) updateDrawdownMetrics(ctx context.Context) {
	query := `
		WITH cumulative_pnl AS (
			SELECT
				exit_time,
				SUM(pnl) OVER (ORDER BY exit_time) as cumulative_pnl
			FROM trades
			WHERE status = 'FILLED' AND exit_time IS NOT NULL
			ORDER BY exit_time
		),
		peak_pnl AS (
			SELECT
				exit_time,
				cumulative_pnl,
				MAX(cumulative_pnl) OVER (ORDER BY exit_time) as peak
			FROM cumulative_pnl
		)
		SELECT
			COALESCE(
				CASE
					WHEN MAX(peak) > 0 THEN (MAX(peak) - MIN(cumulative_pnl)) / MAX(peak)
					ELSE 0
				END,
				0
			) as max_drawdown
		FROM peak_pnl
	`

	var drawdown float64
	err := u.db.QueryRow(ctx, query).Scan(&drawdown)
	if err == nil {
		CurrentDrawdown.Set(drawdown)
	}
}

// updateReturnMetrics calculates daily, weekly, and monthly returns
func (u *Updater) updateReturnMetrics(ctx context.Context) {
	// Daily return
	query := `
		SELECT COALESCE(SUM(pnl), 0)
		FROM trades
		WHERE status = 'FILLED'
		AND exit_time >= NOW() - INTERVAL '1 day'
	`

	var dailyPnL float64
	err := u.db.QueryRow(ctx, query).Scan(&dailyPnL)
	if err == nil {
		// Assuming initial capital of 10000 (should be configurable)
		initialCapital := 10000.0
		DailyReturn.Set(dailyPnL / initialCapital)
	}

	// Weekly return
	query = `
		SELECT COALESCE(SUM(pnl), 0)
		FROM trades
		WHERE status = 'FILLED'
		AND exit_time >= NOW() - INTERVAL '7 days'
	`

	var weeklyPnL float64
	err = u.db.QueryRow(ctx, query).Scan(&weeklyPnL)
	if err == nil {
		initialCapital := 10000.0
		WeeklyReturn.Set(weeklyPnL / initialCapital)
	}

	// Monthly return
	query = `
		SELECT COALESCE(SUM(pnl), 0)
		FROM trades
		WHERE status = 'FILLED'
		AND exit_time >= NOW() - INTERVAL '30 days'
	`

	var monthlyPnL float64
	err = u.db.QueryRow(ctx, query).Scan(&monthlyPnL)
	if err == nil {
		initialCapital := 10000.0
		MonthlyReturn.Set(monthlyPnL / initialCapital)
	}
}

// updateSharpeRatio calculates the Sharpe ratio
func (u *Updater) updateSharpeRatio(ctx context.Context) {
	// Calculate daily returns for last 30 days
	query := `
		SELECT
			DATE(exit_time) as trade_date,
			SUM(pnl) as daily_pnl
		FROM trades
		WHERE status = 'FILLED'
		AND exit_time >= NOW() - INTERVAL '30 days'
		GROUP BY DATE(exit_time)
		ORDER BY trade_date
	`

	rows, err := u.db.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate Sharpe ratio")
		return
	}
	defer rows.Close()

	var returns []float64
	initialCapital := 10000.0

	for rows.Next() {
		var date time.Time
		var pnl float64
		if err := rows.Scan(&date, &pnl); err != nil {
			continue
		}
		returns = append(returns, pnl/initialCapital)
	}

	if len(returns) > 1 {
		// Calculate mean return
		var sum float64
		for _, r := range returns {
			sum += r
		}
		mean := sum / float64(len(returns))

		// Calculate standard deviation
		var variance float64
		for _, r := range returns {
			diff := r - mean
			variance += diff * diff
		}
		variance /= float64(len(returns))
		stdDev := math.Sqrt(variance)

		// Sharpe ratio (assuming risk-free rate of 0)
		if stdDev > 0 {
			sharpe := mean / stdDev * math.Sqrt(252) // Annualized
			SharpeRatio.Set(sharpe)
		}
	}
}

// updatePositionMetrics updates position-related metrics
func (u *Updater) updatePositionMetrics(ctx context.Context) {
	// Count open positions
	var openCount int64
	query := `SELECT COUNT(*) FROM positions WHERE status = 'OPEN'`
	err := u.db.QueryRow(ctx, query).Scan(&openCount)
	if err == nil {
		OpenPositions.Set(float64(openCount))
	}

	// Update position values by symbol
	query = `
		SELECT
			symbol,
			SUM(quantity * entry_price) as position_value
		FROM positions
		WHERE status = 'OPEN'
		GROUP BY symbol
	`

	rows, err := u.db.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch position values")
		return
	}
	defer rows.Close()

	// Reset all symbols to 0 first
	// (This is a simplification - in production, track which symbols have positions)
	for rows.Next() {
		var symbol string
		var value float64
		if err := rows.Scan(&symbol, &value); err != nil {
			continue
		}
		UpdatePositionValue(symbol, value)
	}
}

// updateAgentMetrics updates agent-related metrics
func (u *Updater) updateAgentMetrics(ctx context.Context) {
	// Count active agents
	var activeCount int64
	query := `
		SELECT COUNT(*)
		FROM agent_status
		WHERE status = 'ONLINE'
		AND last_heartbeat >= NOW() - INTERVAL '1 minute'
	`
	err := u.db.QueryRow(ctx, query).Scan(&activeCount)
	if err == nil {
		ActiveAgents.Set(float64(activeCount))
	}

	// Update agent status metrics
	query = `
		SELECT
			agent_type,
			CASE
				WHEN status = 'ONLINE' AND last_heartbeat >= NOW() - INTERVAL '1 minute'
				THEN 1
				ELSE 0
			END as online
		FROM agent_status
	`

	rows, err := u.db.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch agent status")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var agentType string
		var online int
		if err := rows.Scan(&agentType, &online); err != nil {
			continue
		}
		SetAgentStatus(agentType, online == 1)
	}

	// Update average confidence by agent type
	query = `
		SELECT
			agent_type,
			AVG(confidence) as avg_confidence
		FROM agent_signals
		WHERE created_at >= NOW() - INTERVAL '5 minutes'
		GROUP BY agent_type
	`

	rows, err = u.db.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch agent confidence")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var agentType string
		var confidence float64
		if err := rows.Scan(&agentType, &confidence); err != nil {
			continue
		}
		AgentSignalConfidence.WithLabelValues(agentType).Set(confidence)
	}
}

// updateDatabaseMetrics updates database connection pool metrics
func (u *Updater) updateDatabaseMetrics() {
	stat := u.db.Stat()
	UpdateDatabaseConnections(stat.AcquiredConns(), stat.IdleConns())
}
