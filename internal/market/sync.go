package market

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// SyncService handles periodic synchronization of market data from CoinGecko to TimescaleDB
type SyncService struct {
	client   *CachedCoinGeckoClient
	db       *sql.DB
	symbols  []string
	interval time.Duration
	stopCh   chan struct{}
}

// NewSyncService creates a new market data sync service
func NewSyncService(client *CachedCoinGeckoClient, db *sql.DB, symbols []string, interval time.Duration) *SyncService {
	return &SyncService{
		client:   client,
		db:       db,
		symbols:  symbols,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic synchronization
func (s *SyncService) Start(ctx context.Context) error {
	log.Info().
		Strs("symbols", s.symbols).
		Dur("interval", s.interval).
		Msg("Starting market data sync service")

	// Do initial sync
	if err := s.syncAll(ctx); err != nil {
		log.Error().Err(err).Msg("Initial sync failed")
	}

	// Start periodic sync
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Market data sync service stopped (context cancelled)")
			return ctx.Err()
		case <-s.stopCh:
			log.Info().Msg("Market data sync service stopped")
			return nil
		case <-ticker.C:
			if err := s.syncAll(ctx); err != nil {
				log.Error().Err(err).Msg("Periodic sync failed")
			}
		}
	}
}

// Stop stops the sync service
func (s *SyncService) Stop() {
	close(s.stopCh)
}

// syncAll synchronizes data for all configured symbols
func (s *SyncService) syncAll(ctx context.Context) error {
	log.Info().Msg("Starting sync for all symbols")
	startTime := time.Now()

	for _, symbol := range s.symbols {
		if err := s.syncSymbol(ctx, symbol); err != nil {
			log.Error().
				Err(err).
				Str("symbol", symbol).
				Msg("Failed to sync symbol")
			// Continue with other symbols even if one fails
			continue
		}
	}

	duration := time.Since(startTime)
	log.Info().
		Dur("duration", duration).
		Int("symbols_count", len(s.symbols)).
		Msg("Completed sync for all symbols")

	return nil
}

// syncSymbol synchronizes historical data for a single symbol
func (s *SyncService) syncSymbol(ctx context.Context, symbol string) error {
	log.Debug().
		Str("symbol", symbol).
		Msg("Syncing symbol data")

	// Get last synced timestamp from database
	lastTimestamp, err := s.getLastTimestamp(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to get last timestamp: %w", err)
	}

	// Calculate how many days to fetch
	days := s.calculateDaysToFetch(lastTimestamp)
	if days == 0 {
		log.Debug().
			Str("symbol", symbol).
			Msg("Symbol is up to date, skipping")
		return nil
	}

	// Fetch market chart from CoinGecko
	log.Debug().
		Str("symbol", symbol).
		Int("days", days).
		Msg("Fetching market chart")

	chart, err := s.client.GetMarketChart(ctx, symbol, days)
	if err != nil {
		return fmt.Errorf("failed to fetch market chart: %w", err)
	}

	// Convert to candlesticks (15-minute intervals)
	candlesticks := chart.ToCandlesticks(15)
	if len(candlesticks) == 0 {
		log.Debug().
			Str("symbol", symbol).
			Msg("No new candlesticks to store")
		return nil
	}

	// Store in database
	if err := s.storeCandlesticks(ctx, symbol, candlesticks); err != nil {
		return fmt.Errorf("failed to store candlesticks: %w", err)
	}

	log.Info().
		Str("symbol", symbol).
		Int("candlesticks_count", len(candlesticks)).
		Msg("Successfully synced symbol data")

	return nil
}

// getLastTimestamp gets the most recent timestamp for a symbol from the database
func (s *SyncService) getLastTimestamp(ctx context.Context, symbol string) (time.Time, error) {
	var lastTimestamp time.Time

	query := `
		SELECT timestamp
		FROM market_data
		WHERE symbol = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`

	err := s.db.QueryRowContext(ctx, query, symbol).Scan(&lastTimestamp)
	if err == sql.ErrNoRows {
		// No data exists, fetch all available data (default to 90 days)
		return time.Now().AddDate(0, 0, -90), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("query failed: %w", err)
	}

	return lastTimestamp, nil
}

// calculateDaysToFetch calculates how many days of data to fetch based on last timestamp
func (s *SyncService) calculateDaysToFetch(lastTimestamp time.Time) int {
	now := time.Now()
	duration := now.Sub(lastTimestamp)

	days := int(duration.Hours() / 24)

	// Cap at 90 days (CoinGecko API limit for detailed data)
	if days > 90 {
		days = 90
	}

	// Don't fetch if we're less than 1 hour behind
	if duration < time.Hour {
		return 0
	}

	// Minimum 1 day
	if days < 1 {
		days = 1
	}

	return days
}

// storeCandlesticks stores candlestick data in TimescaleDB
func (s *SyncService) storeCandlesticks(ctx context.Context, symbol string, candlesticks []Candlestick) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // Rollback on error - commit overrides if successful

	// Prepare insert statement with ON CONFLICT to handle duplicates
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO market_data (
			timestamp, symbol, exchange, interval,
			open, high, low, close, volume
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (timestamp, symbol, exchange, interval)
		DO UPDATE SET
			open = EXCLUDED.open,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }() // Statement cleanup

	// Insert/update each candlestick
	for _, candle := range candlesticks {
		_, err := stmt.ExecContext(ctx,
			candle.Timestamp,
			symbol,
			"coingecko", // Exchange source
			"1d",        // Daily interval (CoinGecko provides daily data)
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
			candle.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert candlestick: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetCandlesticks retrieves candlesticks from the database for backtesting
func (s *SyncService) GetCandlesticks(ctx context.Context, symbol string, start, end time.Time) ([]Candlestick, error) {
	query := `
		SELECT timestamp, open, high, low, close, volume
		FROM market_data
		WHERE symbol = $1
			AND timestamp >= $2
			AND timestamp <= $3
			AND exchange = 'coingecko'
			AND interval = '1d'
		ORDER BY timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, symbol, start, end)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }() // Rows cleanup

	var candlesticks []Candlestick
	for rows.Next() {
		var c Candlestick
		if err := rows.Scan(&c.Timestamp, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		candlesticks = append(candlesticks, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	log.Debug().
		Str("symbol", symbol).
		Time("start", start).
		Time("end", end).
		Int("count", len(candlesticks)).
		Msg("Retrieved candlesticks from database")

	return candlesticks, nil
}

// GetLatestPrice retrieves the most recent price for a symbol
func (s *SyncService) GetLatestPrice(ctx context.Context, symbol string) (*Candlestick, error) {
	query := `
		SELECT timestamp, open, high, low, close, volume
		FROM market_data
		WHERE symbol = $1
			AND exchange = 'coingecko'
			AND interval = '1d'
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var c Candlestick
	err := s.db.QueryRowContext(ctx, query, symbol).Scan(
		&c.Timestamp, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no data found for symbol: %s", symbol)
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &c, nil
}

// GetDataStats returns statistics about stored data for a symbol
func (s *SyncService) GetDataStats(ctx context.Context, symbol string) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as count,
			MIN(timestamp) as earliest,
			MAX(timestamp) as latest,
			AVG(volume) as avg_volume
		FROM market_data
		WHERE symbol = $1
			AND exchange = 'coingecko'
			AND interval = '1d'
	`

	var count int
	var earliest, latest time.Time
	var avgVolume float64

	err := s.db.QueryRowContext(ctx, query, symbol).Scan(&count, &earliest, &latest, &avgVolume)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	stats := map[string]interface{}{
		"count":       count,
		"earliest":    earliest,
		"latest":      latest,
		"avg_volume":  avgVolume,
		"days_stored": int(latest.Sub(earliest).Hours() / 24),
	}

	return stats, nil
}
