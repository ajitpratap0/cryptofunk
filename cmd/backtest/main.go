// Backtest Runner CLI
// Runs trading strategies on historical data to evaluate performance
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/pkg/backtest"
)

// ============================================================================
// CLI FLAGS
// ============================================================================

var (
	// Strategy parameters
	strategyName = flag.String("strategy", "", "Strategy name (trend, reversion, arbitrage)")
	symbols      = flag.String("symbols", "BTC/USDT", "Comma-separated list of symbols to trade")

	// Date range
	startDate = flag.String("start", "", "Start date (YYYY-MM-DD)")
	endDate   = flag.String("end", "", "End date (YYYY-MM-DD)")

	// Capital and risk
	initialCapital = flag.Float64("capital", 10000.0, "Initial capital in USD")
	commissionRate = flag.Float64("commission", 0.001, "Commission rate (0.001 = 0.1%)")
	positionSizing = flag.String("sizing", "percent", "Position sizing method (fixed, percent, kelly)")
	positionSize   = flag.Float64("size", 0.1, "Position size (depends on sizing method)")
	maxPositions   = flag.Int("max-positions", 3, "Maximum concurrent positions")

	// Output
	outputFile = flag.String("output", "", "Output file for results (optional)")
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
)

// ============================================================================
// MAIN
// ============================================================================

func main() {
	// Parse flags
	flag.Parse()

	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if *verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Validate required flags
	if *strategyName == "" {
		fmt.Fprintln(os.Stderr, "Error: -strategy flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *startDate == "" || *endDate == "" {
		fmt.Fprintln(os.Stderr, "Error: -start and -end dates are required")
		flag.Usage()
		os.Exit(1)
	}

	// Parse dates
	start, err := time.Parse("2006-01-02", *startDate)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid start date format (use YYYY-MM-DD)")
	}

	end, err := time.Parse("2006-01-02", *endDate)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid end date format (use YYYY-MM-DD)")
	}

	// Parse symbols
	symbolList := parseSymbols(*symbols)

	log.Info().
		Str("strategy", *strategyName).
		Strs("symbols", symbolList).
		Str("start", start.Format("2006-01-02")).
		Str("end", end.Format("2006-01-02")).
		Float64("capital", *initialCapital).
		Msg("Starting backtest")

	// Run backtest
	ctx := context.Background()
	if err := runBacktest(ctx, start, end, symbolList); err != nil {
		log.Fatal().Err(err).Msg("Backtest failed")
	}

	log.Info().Msg("Backtest completed successfully")
}

// ============================================================================
// BACKTEST EXECUTION
// ============================================================================

func runBacktest(ctx context.Context, start, end time.Time, symbolList []string) error {
	// Connect to database
	database, err := db.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// Create backtest configuration
	config := backtest.BacktestConfig{
		InitialCapital: *initialCapital,
		CommissionRate: *commissionRate,
		PositionSizing: *positionSizing,
		PositionSize:   *positionSize,
		MaxPositions:   *maxPositions,
		StartDate:      start,
		EndDate:        end,
		Symbols:        symbolList,
	}

	// Create backtest engine
	engine := backtest.NewEngine(config)

	// Load historical data
	for _, symbol := range symbolList {
		candlesticks, err := loadHistoricalData(ctx, database, symbol, start, end)
		if err != nil {
			return fmt.Errorf("failed to load data for %s: %w", symbol, err)
		}

		if err := engine.LoadHistoricalData(symbol, candlesticks); err != nil {
			return fmt.Errorf("failed to load candlesticks for %s: %w", symbol, err)
		}
	}

	// Create strategy
	strategy, err := createStrategy(*strategyName)
	if err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// Run backtest
	if err := engine.Run(ctx, strategy); err != nil {
		return fmt.Errorf("backtest execution failed: %w", err)
	}

	// Calculate metrics
	metrics, err := backtest.CalculateMetrics(engine)
	if err != nil {
		return fmt.Errorf("failed to calculate metrics: %w", err)
	}

	// Generate report
	report := backtest.GenerateReport(metrics)

	// Display report
	fmt.Println(report)

	// Write to output file if specified
	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, []byte(report), 0644); err != nil {
			log.Warn().Err(err).Str("file", *outputFile).Msg("Failed to write output file")
		} else {
			log.Info().Str("file", *outputFile).Msg("Report written to file")
		}
	}

	return nil
}

// ============================================================================
// DATA LOADING
// ============================================================================

func loadHistoricalData(ctx context.Context, database *db.DB, symbol string, start, end time.Time) ([]*backtest.Candlestick, error) {
	query := `
		SELECT
			timestamp,
			open,
			high,
			low,
			close,
			volume
		FROM candlesticks
		WHERE symbol = $1
			AND timestamp >= $2
			AND timestamp <= $3
		ORDER BY timestamp ASC
	`

	rows, err := database.Pool().Query(ctx, query, symbol, start, end)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var candlesticks []*backtest.Candlestick
	for rows.Next() {
		candle := &backtest.Candlestick{
			Symbol: symbol,
		}

		if err := rows.Scan(
			&candle.Timestamp,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		candlesticks = append(candlesticks, candle)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	log.Info().
		Str("symbol", symbol).
		Int("candles", len(candlesticks)).
		Msg("Loaded historical data from database")

	return candlesticks, nil
}

// ============================================================================
// STRATEGY FACTORY
// ============================================================================

func createStrategy(name string) (backtest.Strategy, error) {
	switch strings.ToLower(name) {
	case "simple":
		return &SimpleStrategy{}, nil
	case "buy-and-hold":
		return &BuyAndHoldStrategy{}, nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s (available: simple, buy-and-hold)", name)
	}
}

// ============================================================================
// EXAMPLE STRATEGIES
// ============================================================================

// SimpleStrategy is a basic example strategy
type SimpleStrategy struct {
	symbols []string
	bought  map[string]bool
}

func (s *SimpleStrategy) Initialize(engine *backtest.Engine) error {
	s.symbols = []string{}
	for symbol := range engine.Data {
		s.symbols = append(s.symbols, symbol)
	}
	s.bought = make(map[string]bool)

	log.Info().Strs("symbols", s.symbols).Msg("Initialized SimpleStrategy")
	return nil
}

func (s *SimpleStrategy) GenerateSignals(engine *backtest.Engine) ([]*backtest.Signal, error) {
	var signals []*backtest.Signal

	for _, symbol := range s.symbols {
		// Get current candle
		candle, err := engine.GetCurrentCandle(symbol)
		if err != nil {
			continue
		}

		// Simple logic: Buy if we haven't bought yet, sell after holding for 10 steps
		if !s.bought[symbol] && len(engine.Positions) < engine.MaxPositions {
			signals = append(signals, &backtest.Signal{
				Timestamp:  candle.Timestamp,
				Symbol:     symbol,
				Side:       "BUY",
				Confidence: 0.8,
				Reasoning:  "Simple strategy buy signal",
				Agent:      "simple_strategy",
			})
			s.bought[symbol] = true
		} else if position, exists := engine.Positions[symbol]; exists {
			// Check if we've held for long enough
			holdingTime := candle.Timestamp.Sub(position.EntryTime)
			if holdingTime > 10*time.Hour { // Example: hold for 10 hours
				signals = append(signals, &backtest.Signal{
					Timestamp:  candle.Timestamp,
					Symbol:     symbol,
					Side:       "SELL",
					Confidence: 0.8,
					Reasoning:  "Simple strategy sell signal (holding time exceeded)",
					Agent:      "simple_strategy",
				})
				s.bought[symbol] = false
			}
		}
	}

	return signals, nil
}

func (s *SimpleStrategy) Finalize(engine *backtest.Engine) error {
	log.Info().Msg("Finalized SimpleStrategy")
	return nil
}

// BuyAndHoldStrategy simply buys at the beginning and holds
type BuyAndHoldStrategy struct {
	bought bool
}

func (s *BuyAndHoldStrategy) Initialize(engine *backtest.Engine) error {
	s.bought = false
	log.Info().Msg("Initialized BuyAndHoldStrategy")
	return nil
}

func (s *BuyAndHoldStrategy) GenerateSignals(engine *backtest.Engine) ([]*backtest.Signal, error) {
	if s.bought {
		return nil, nil // No more signals after initial buy
	}

	var signals []*backtest.Signal

	// Buy all symbols at the start
	for symbol := range engine.Data {
		candle, err := engine.GetCurrentCandle(symbol)
		if err != nil {
			continue
		}

		signals = append(signals, &backtest.Signal{
			Timestamp:  candle.Timestamp,
			Symbol:     symbol,
			Side:       "BUY",
			Confidence: 1.0,
			Reasoning:  "Buy and hold strategy - initial purchase",
			Agent:      "buy_and_hold",
		})
	}

	s.bought = true
	return signals, nil
}

func (s *BuyAndHoldStrategy) Finalize(engine *backtest.Engine) error {
	log.Info().Msg("Finalized BuyAndHoldStrategy")
	return nil
}

// ============================================================================
// UTILITIES
// ============================================================================

func parseSymbols(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
