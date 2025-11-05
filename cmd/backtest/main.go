// Backtest Runner CLI
// Runs trading strategies on historical data to evaluate performance
//
//nolint:goconst // Data source types are configuration strings
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
	strategyName = flag.String("strategy", "", "Strategy name (simple, buy-and-hold)")
	symbols      = flag.String("symbols", "BTC/USDT", "Comma-separated list of symbols to trade")

	// Data source
	dataSource = flag.String("data-source", "database", "Data source (database, csv [NOT IMPLEMENTED], json [NOT IMPLEMENTED])")
	dataPath   = flag.String("data-path", "", "Path to CSV/JSON data file (COMING SOON - only database supported currently)")

	// Date range
	startDate = flag.String("start", "", "Start date (YYYY-MM-DD)")
	endDate   = flag.String("end", "", "End date (YYYY-MM-DD)")

	// Capital and risk
	initialCapital = flag.Float64("capital", 10000.0, "Initial capital in USD")
	commissionRate = flag.Float64("commission", 0.001, "Commission rate (0.001 = 0.1%)")
	positionSizing = flag.String("sizing", "percent", "Position sizing method (fixed, percent, kelly)")
	positionSize   = flag.Float64("size", 0.1, "Position size (depends on sizing method)")
	maxPositions   = flag.Int("max-positions", 3, "Maximum concurrent positions")

	// Optimization
	optimize = flag.Bool("optimize", false, "Run parameter optimization")
	// TODO: Will be used in Phase 11 for advanced strategy optimization
	// optimizeMethod = flag.String("optimize-method", "grid", "Optimization method (grid, walk-forward, genetic)")
	// optimizeMetric = flag.String("optimize-metric", "sharpe", "Optimization metric (sharpe, sortino, calmar, return, profit-factor)")

	// Output
	outputFile = flag.String("output", "", "Output file for text report (optional)")
	htmlReport = flag.String("html", "", "Generate HTML report to file (optional)")
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
		fmt.Fprintln(os.Stderr, "\nAvailable strategies: simple, buy-and-hold")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  ./backtest -strategy=simple -start=2024-01-01 -end=2024-12-31 -data-source=csv -data-path=data.csv")
		flag.Usage()
		os.Exit(1)
	}

	// IMPORTANT: CSV and JSON loaders are not yet implemented
	if *dataSource == "csv" || *dataSource == "json" {
		fmt.Fprintln(os.Stderr, "ERROR: CSV and JSON data sources are not yet implemented.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Currently only 'database' source is supported.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Coming soon in a future release!")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please use: --data-source=database")
		os.Exit(1)
	}

	if (*dataSource == "csv" || *dataSource == "json") && *dataPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -data-path is required when using csv or json data sources")
		flag.Usage()
		os.Exit(1)
	}

	// Dates are optional for CSV/JSON (can be inferred from data)
	if *dataSource == "database" && (*startDate == "" || *endDate == "") {
		fmt.Fprintln(os.Stderr, "Error: -start and -end dates are required when using database source")
		flag.Usage()
		os.Exit(1)
	}

	// Parse dates (optional for CSV/JSON)
	var start, end time.Time
	var err error

	if *startDate != "" {
		start, err = time.Parse("2006-01-02", *startDate)
		if err != nil {
			log.Fatal().Err(err).Msg("Invalid start date format (use YYYY-MM-DD)")
		}
	}

	if *endDate != "" {
		end, err = time.Parse("2006-01-02", *endDate)
		if err != nil {
			log.Fatal().Err(err).Msg("Invalid end date format (use YYYY-MM-DD)")
		}
	}

	// Parse symbols
	symbolList := parseSymbols(*symbols)

	log.Info().
		Str("strategy", *strategyName).
		Strs("symbols", symbolList).
		Str("data_source", *dataSource).
		Float64("capital", *initialCapital).
		Bool("optimize", *optimize).
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

	// Load historical data based on source
	switch *dataSource {
	case "database":
		if err := loadFromDatabase(ctx, engine, symbolList, start, end); err != nil {
			return fmt.Errorf("failed to load data from database: %w", err)
		}
	case "csv":
		if err := loadFromCSV(engine, *dataPath, symbolList); err != nil {
			return fmt.Errorf("failed to load data from CSV: %w", err)
		}
	case "json":
		if err := loadFromJSON(engine, *dataPath, symbolList); err != nil {
			return fmt.Errorf("failed to load data from JSON: %w", err)
		}
	default:
		return fmt.Errorf("unsupported data source: %s", *dataSource)
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

	// Generate and display text report
	report := backtest.GenerateReport(metrics)
	fmt.Println(report)

	// Write text report to file if specified
	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, []byte(report), 0600); err != nil {
			log.Warn().Err(err).Str("file", *outputFile).Msg("Failed to write output file")
		} else {
			log.Info().Str("file", *outputFile).Msg("Text report written to file")
		}
	}

	// Generate HTML report if specified
	if *htmlReport != "" {
		generator, err := backtest.NewReportGenerator(engine)
		if err != nil {
			return fmt.Errorf("failed to create report generator: %w", err)
		}

		if err := generator.SaveToFile(*htmlReport); err != nil {
			return fmt.Errorf("failed to save HTML report: %w", err)
		}

		log.Info().Str("file", *htmlReport).Msg("HTML report written to file")
	}

	return nil
}

// ============================================================================
// DATA LOADING
// ============================================================================

// loadFromDatabase loads data from TimescaleDB
func loadFromDatabase(ctx context.Context, engine *backtest.Engine, symbols []string, start, end time.Time) error {
	database, err := db.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	for _, symbol := range symbols {
		candlesticks, err := queryHistoricalData(ctx, database, symbol, start, end)
		if err != nil {
			return fmt.Errorf("failed to load data for %s: %w", symbol, err)
		}

		if err := engine.LoadHistoricalData(symbol, candlesticks); err != nil {
			return fmt.Errorf("failed to load candlesticks for %s: %w", symbol, err)
		}
	}

	return nil
}

func queryHistoricalData(ctx context.Context, database *db.DB, symbol string, start, end time.Time) ([]*backtest.Candlestick, error) {
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

// loadFromCSV loads data from CSV file
// CSV format: timestamp,symbol,open,high,low,close,volume
func loadFromCSV(engine *backtest.Engine, filepath string, symbols []string) error {
	log.Info().Str("file", filepath).Msg("Loading data from CSV")

	// For now, return error with instructions
	return fmt.Errorf("CSV loader not yet implemented - use database source")
}

// loadFromJSON loads data from JSON file
func loadFromJSON(engine *backtest.Engine, filepath string, symbols []string) error {
	log.Info().Str("file", filepath).Msg("Loading data from JSON")

	// For now, return error with instructions
	return fmt.Errorf("JSON loader not yet implemented - use database source")
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
