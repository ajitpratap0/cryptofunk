// Package backtest provides a backtesting framework for trading strategies
package backtest

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// Candlestick represents OHLCV data for a time period
type Candlestick struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// Signal represents a trading signal from an agent
type Signal struct {
	Timestamp  time.Time              `json:"timestamp"`
	Symbol     string                 `json:"symbol"`
	Side       string                 `json:"side"`       // "BUY", "SELL", "HOLD"
	Confidence float64                `json:"confidence"` // 0.0 to 1.0
	Reasoning  string                 `json:"reasoning"`
	Agent      string                 `json:"agent"` // Which agent generated this signal
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Trade represents an executed trade
type Trade struct {
	ID         int       `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"` // "BUY", "SELL"
	Quantity   float64   `json:"quantity"`
	Price      float64   `json:"price"`
	Commission float64   `json:"commission"`
	Value      float64   `json:"value"` // price * quantity
	Signal     *Signal   `json:"signal,omitempty"`
}

// Position represents an open trading position
type Position struct {
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"` // "LONG", "SHORT"
	EntryTime    time.Time `json:"entry_time"`
	EntryPrice   float64   `json:"entry_price"`
	Quantity     float64   `json:"quantity"`
	CurrentPrice float64   `json:"current_price"`
	UnrealizedPL float64   `json:"unrealized_pl"`
	Commission   float64   `json:"commission"`
}

// ClosedPosition represents a closed position with P&L
type ClosedPosition struct {
	Symbol      string        `json:"symbol"`
	Side        string        `json:"side"`
	EntryTime   time.Time     `json:"entry_time"`
	ExitTime    time.Time     `json:"exit_time"`
	EntryPrice  float64       `json:"entry_price"`
	ExitPrice   float64       `json:"exit_price"`
	Quantity    float64       `json:"quantity"`
	RealizedPL  float64       `json:"realized_pl"`
	ReturnPct   float64       `json:"return_pct"`
	HoldingTime time.Duration `json:"holding_time"`
	Commission  float64       `json:"commission"`
}

// EquityPoint represents portfolio equity at a point in time
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Equity    float64   `json:"equity"`
	Cash      float64   `json:"cash"`
	Holdings  float64   `json:"holdings"`
}

// ============================================================================
// BACKTEST ENGINE
// ============================================================================

// Engine is the main backtesting engine
type Engine struct {
	// Configuration
	InitialCapital float64 `json:"initial_capital"`
	CommissionRate float64 `json:"commission_rate"` // e.g., 0.001 for 0.1%
	PositionSizing string  `json:"position_sizing"` // "fixed", "percent", "kelly"
	PositionSize   float64 `json:"position_size"`   // Amount per trade
	MaxPositions   int     `json:"max_positions"`   // Maximum concurrent positions

	// State
	Cash            float64              `json:"cash"`
	Positions       map[string]*Position `json:"positions"` // symbol -> position
	Trades          []*Trade             `json:"trades"`
	ClosedPositions []*ClosedPosition    `json:"closed_positions"`
	EquityCurve     []*EquityPoint       `json:"equity_curve"`

	// Historical data
	Data         map[string][]*Candlestick `json:"-"` // symbol -> candlesticks
	CurrentIndex map[string]int            `json:"-"` // symbol -> current index

	// Statistics (calculated during backtest)
	TotalTrades    int     `json:"total_trades"`
	WinningTrades  int     `json:"winning_trades"`
	LosingTrades   int     `json:"losing_trades"`
	TotalProfit    float64 `json:"total_profit"`
	TotalLoss      float64 `json:"total_loss"`
	MaxDrawdown    float64 `json:"max_drawdown"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	PeakEquity     float64 `json:"peak_equity"`
}

// NewEngine creates a new backtesting engine
func NewEngine(config BacktestConfig) *Engine {
	return &Engine{
		InitialCapital:  config.InitialCapital,
		CommissionRate:  config.CommissionRate,
		PositionSizing:  config.PositionSizing,
		PositionSize:    config.PositionSize,
		MaxPositions:    config.MaxPositions,
		Cash:            config.InitialCapital,
		Positions:       make(map[string]*Position),
		Trades:          []*Trade{},
		ClosedPositions: []*ClosedPosition{},
		EquityCurve:     []*EquityPoint{},
		Data:            make(map[string][]*Candlestick),
		CurrentIndex:    make(map[string]int),
		PeakEquity:      config.InitialCapital,
	}
}

// BacktestConfig holds configuration for a backtest
type BacktestConfig struct {
	InitialCapital float64
	CommissionRate float64
	PositionSizing string // "fixed", "percent", "kelly"
	PositionSize   float64
	MaxPositions   int
	StartDate      time.Time
	EndDate        time.Time
	Symbols        []string
}

// ============================================================================
// DATA LOADING
// ============================================================================

// LoadHistoricalData loads candlestick data for backtesting
func (e *Engine) LoadHistoricalData(symbol string, candlesticks []*Candlestick) error {
	if len(candlesticks) == 0 {
		return fmt.Errorf("no candlesticks provided for symbol %s", symbol)
	}

	// Sort by timestamp ascending
	sort.Slice(candlesticks, func(i, j int) bool {
		return candlesticks[i].Timestamp.Before(candlesticks[j].Timestamp)
	})

	e.Data[symbol] = candlesticks
	e.CurrentIndex[symbol] = 0

	log.Info().
		Str("symbol", symbol).
		Int("candles", len(candlesticks)).
		Time("start", candlesticks[0].Timestamp).
		Time("end", candlesticks[len(candlesticks)-1].Timestamp).
		Msg("Loaded historical data for backtesting")

	return nil
}

// GetCurrentCandle returns the current candlestick for a symbol
func (e *Engine) GetCurrentCandle(symbol string) (*Candlestick, error) {
	candles, exists := e.Data[symbol]
	if !exists {
		return nil, fmt.Errorf("no data loaded for symbol %s", symbol)
	}

	index := e.CurrentIndex[symbol]
	if index >= len(candles) {
		return nil, fmt.Errorf("no more data for symbol %s", symbol)
	}

	return candles[index], nil
}

// GetHistoricalCandles returns N candlesticks before current index
func (e *Engine) GetHistoricalCandles(symbol string, lookback int) ([]*Candlestick, error) {
	candles, exists := e.Data[symbol]
	if !exists {
		return nil, fmt.Errorf("no data loaded for symbol %s", symbol)
	}

	currentIndex := e.CurrentIndex[symbol]
	if currentIndex == 0 {
		return []*Candlestick{}, nil
	}

	startIndex := currentIndex - lookback
	if startIndex < 0 {
		startIndex = 0
	}

	return candles[startIndex:currentIndex], nil
}

// ============================================================================
// TIME-STEP SIMULATION
// ============================================================================

// Step advances the backtest by one time step
func (e *Engine) Step(ctx context.Context) (bool, error) {
	// Check if we have more data
	hasMore := false
	for symbol := range e.Data {
		if e.CurrentIndex[symbol] < len(e.Data[symbol]) {
			hasMore = true
			break
		}
	}

	if !hasMore {
		return false, nil // Backtest complete
	}

	// Get current timestamp (earliest timestamp across all symbols)
	var currentTime time.Time
	for symbol, candles := range e.Data {
		index := e.CurrentIndex[symbol]
		if index < len(candles) {
			candleTime := candles[index].Timestamp
			if currentTime.IsZero() || candleTime.Before(currentTime) {
				currentTime = candleTime
			}
		}
	}

	// Update current prices for all positions
	for symbol, position := range e.Positions {
		candle, err := e.GetCurrentCandle(symbol)
		if err == nil {
			position.CurrentPrice = candle.Close
			position.UnrealizedPL = e.calculateUnrealizedPL(position)
		}
	}

	// Record equity point
	e.recordEquityPoint(currentTime)

	// Advance indices for symbols at current time
	for symbol, candles := range e.Data {
		index := e.CurrentIndex[symbol]
		if index < len(candles) && !candles[index].Timestamp.After(currentTime) {
			e.CurrentIndex[symbol]++
		}
	}

	return true, nil
}

// ============================================================================
// ORDER EXECUTION
// ============================================================================

// ExecuteSignal executes a trading signal
func (e *Engine) ExecuteSignal(signal *Signal) error {
	// Get current candle for the symbol
	candle, err := e.GetCurrentCandle(signal.Symbol)
	if err != nil {
		return fmt.Errorf("cannot execute signal: %w", err)
	}

	// Use close price for execution
	price := candle.Close

	switch signal.Side {
	case "BUY":
		return e.executeBuy(signal, price, candle.Timestamp)
	case "SELL":
		return e.executeSell(signal, price, candle.Timestamp)
	case "HOLD":
		// No action needed
		return nil
	default:
		return fmt.Errorf("unknown signal side: %s", signal.Side)
	}
}

// executeBuy executes a buy order
func (e *Engine) executeBuy(signal *Signal, price float64, timestamp time.Time) error {
	// Check if we already have a position
	if _, exists := e.Positions[signal.Symbol]; exists {
		log.Debug().Str("symbol", signal.Symbol).Msg("Already have position, skipping buy")
		return nil
	}

	// Check max positions limit
	if len(e.Positions) >= e.MaxPositions {
		log.Debug().Int("max", e.MaxPositions).Msg("Max positions reached, skipping buy")
		return nil
	}

	// Calculate position size
	quantity := e.calculatePositionSize(price)
	if quantity <= 0 {
		return fmt.Errorf("invalid quantity: %f", quantity)
	}

	value := price * quantity
	commission := value * e.CommissionRate
	totalCost := value + commission

	// Check if we have enough cash
	if e.Cash < totalCost {
		log.Debug().
			Float64("cash", e.Cash).
			Float64("needed", totalCost).
			Msg("Insufficient cash, skipping buy")
		return nil
	}

	// Execute trade
	trade := &Trade{
		ID:         len(e.Trades) + 1,
		Timestamp:  timestamp,
		Symbol:     signal.Symbol,
		Side:       "BUY",
		Quantity:   quantity,
		Price:      price,
		Commission: commission,
		Value:      value,
		Signal:     signal,
	}

	// Open position
	position := &Position{
		Symbol:       signal.Symbol,
		Side:         "LONG",
		EntryTime:    timestamp,
		EntryPrice:   price,
		Quantity:     quantity,
		CurrentPrice: price,
		UnrealizedPL: 0,
		Commission:   commission,
	}

	// Update state
	e.Cash -= totalCost
	e.Positions[signal.Symbol] = position
	e.Trades = append(e.Trades, trade)
	e.TotalTrades++

	log.Info().
		Str("symbol", signal.Symbol).
		Float64("price", price).
		Float64("quantity", quantity).
		Float64("value", value).
		Float64("commission", commission).
		Msg("Executed BUY")

	return nil
}

// executeSell executes a sell order
func (e *Engine) executeSell(signal *Signal, price float64, timestamp time.Time) error {
	// Check if we have a position to close
	position, exists := e.Positions[signal.Symbol]
	if !exists {
		log.Debug().Str("symbol", signal.Symbol).Msg("No position to close, skipping sell")
		return nil
	}

	// Calculate values
	quantity := position.Quantity
	value := price * quantity
	commission := value * e.CommissionRate
	totalProceeds := value - commission

	// Execute trade
	trade := &Trade{
		ID:         len(e.Trades) + 1,
		Timestamp:  timestamp,
		Symbol:     signal.Symbol,
		Side:       "SELL",
		Quantity:   quantity,
		Price:      price,
		Commission: commission,
		Value:      value,
		Signal:     signal,
	}

	// Calculate P&L
	entryValue := position.EntryPrice * quantity
	totalCommissions := position.Commission + commission
	realizedPL := totalProceeds - entryValue - position.Commission
	returnPct := (realizedPL / entryValue) * 100.0

	// Close position
	closedPosition := &ClosedPosition{
		Symbol:      signal.Symbol,
		Side:        position.Side,
		EntryTime:   position.EntryTime,
		ExitTime:    timestamp,
		EntryPrice:  position.EntryPrice,
		ExitPrice:   price,
		Quantity:    quantity,
		RealizedPL:  realizedPL,
		ReturnPct:   returnPct,
		HoldingTime: timestamp.Sub(position.EntryTime),
		Commission:  totalCommissions,
	}

	// Update statistics
	if realizedPL > 0 {
		e.WinningTrades++
		e.TotalProfit += realizedPL
	} else {
		e.LosingTrades++
		e.TotalLoss += realizedPL
	}

	// Update state
	e.Cash += totalProceeds
	delete(e.Positions, signal.Symbol)
	e.Trades = append(e.Trades, trade)
	e.ClosedPositions = append(e.ClosedPositions, closedPosition)

	log.Info().
		Str("symbol", signal.Symbol).
		Float64("price", price).
		Float64("quantity", quantity).
		Float64("pl", realizedPL).
		Float64("return_pct", returnPct).
		Msg("Executed SELL")

	return nil
}

// ============================================================================
// POSITION SIZING
// ============================================================================

// calculatePositionSize calculates the quantity to buy based on position sizing method
func (e *Engine) calculatePositionSize(price float64) float64 {
	switch e.PositionSizing {
	case "fixed":
		// Fixed dollar amount per trade
		return e.PositionSize / price

	case "percent":
		// Percentage of current equity
		equity := e.GetCurrentEquity()
		dollarAmount := equity * e.PositionSize // e.g., 0.1 for 10%
		return dollarAmount / price

	case "kelly":
		// Kelly Criterion (simplified)
		// For now, use fixed percentage
		// TODO: Implement proper Kelly Criterion with win rate and average win/loss
		equity := e.GetCurrentEquity()
		dollarAmount := equity * 0.02 // 2% of equity
		return dollarAmount / price

	default:
		// Default to fixed $1000 per trade
		return 1000.0 / price
	}
}

// ============================================================================
// EQUITY CALCULATIONS
// ============================================================================

// GetCurrentEquity returns current portfolio equity (cash + unrealized P&L)
func (e *Engine) GetCurrentEquity() float64 {
	equity := e.Cash

	for _, position := range e.Positions {
		equity += position.CurrentPrice * position.Quantity
	}

	return equity
}

// calculateUnrealizedPL calculates unrealized P&L for a position
func (e *Engine) calculateUnrealizedPL(position *Position) float64 {
	currentValue := position.CurrentPrice * position.Quantity
	entryValue := position.EntryPrice * position.Quantity
	return currentValue - entryValue - position.Commission
}

// recordEquityPoint records current equity in the equity curve
func (e *Engine) recordEquityPoint(timestamp time.Time) {
	equity := e.GetCurrentEquity()
	holdings := equity - e.Cash

	point := &EquityPoint{
		Timestamp: timestamp,
		Equity:    equity,
		Cash:      e.Cash,
		Holdings:  holdings,
	}

	e.EquityCurve = append(e.EquityCurve, point)

	// Update peak equity and drawdown
	if equity > e.PeakEquity {
		e.PeakEquity = equity
	}

	drawdown := e.PeakEquity - equity
	drawdownPct := (drawdown / e.PeakEquity) * 100.0

	if drawdown > e.MaxDrawdown {
		e.MaxDrawdown = drawdown
		e.MaxDrawdownPct = drawdownPct
	}
}

// ============================================================================
// BACKTEST EXECUTION
// ============================================================================

// Run executes the complete backtest
func (e *Engine) Run(ctx context.Context, strategy Strategy) error {
	log.Info().
		Float64("initial_capital", e.InitialCapital).
		Float64("commission_rate", e.CommissionRate*100).
		Str("position_sizing", e.PositionSizing).
		Int("max_positions", e.MaxPositions).
		Msg("Starting backtest")

	// Initialize strategy
	if err := strategy.Initialize(e); err != nil {
		return fmt.Errorf("failed to initialize strategy: %w", err)
	}

	// Main backtest loop
	stepCount := 0
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Advance one time step
		hasMore, err := e.Step(ctx)
		if err != nil {
			return fmt.Errorf("step error: %w", err)
		}

		if !hasMore {
			break // Backtest complete
		}

		stepCount++

		// Generate signals from strategy
		signals, err := strategy.GenerateSignals(e)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to generate signals")
			continue
		}

		// Execute signals
		for _, signal := range signals {
			if err := e.ExecuteSignal(signal); err != nil {
				log.Warn().
					Err(err).
					Str("symbol", signal.Symbol).
					Str("side", signal.Side).
					Msg("Failed to execute signal")
			}
		}

		// Log progress every 1000 steps
		if stepCount%1000 == 0 {
			equity := e.GetCurrentEquity()
			log.Debug().
				Int("step", stepCount).
				Float64("equity", equity).
				Int("positions", len(e.Positions)).
				Int("trades", e.TotalTrades).
				Msg("Backtest progress")
		}
	}

	// Close all remaining positions at the end
	e.closeAllPositions()

	// Finalize strategy
	if err := strategy.Finalize(e); err != nil {
		log.Warn().Err(err).Msg("Failed to finalize strategy")
	}

	log.Info().
		Int("steps", stepCount).
		Int("trades", e.TotalTrades).
		Float64("final_equity", e.GetCurrentEquity()).
		Msg("Backtest complete")

	return nil
}

// closeAllPositions closes all open positions at the end of backtest
func (e *Engine) closeAllPositions() {
	for symbol, position := range e.Positions {
		// Create a SELL signal
		signal := &Signal{
			Timestamp:  position.EntryTime, // Will be updated to current time
			Symbol:     symbol,
			Side:       "SELL",
			Confidence: 1.0,
			Reasoning:  "End of backtest - closing position",
			Agent:      "backtest_engine",
		}

		// Get current price
		candle, err := e.GetCurrentCandle(symbol)
		if err != nil {
			log.Warn().
				Err(err).
				Str("symbol", symbol).
				Msg("Failed to get current candle for position close")
			continue
		}

		if err := e.executeSell(signal, candle.Close, candle.Timestamp); err != nil {
			log.Warn().
				Err(err).
				Str("symbol", symbol).
				Msg("Failed to close position at end of backtest")
		}
	}
}

// ============================================================================
// STRATEGY INTERFACE
// ============================================================================

// Strategy is the interface that trading strategies must implement
type Strategy interface {
	// Initialize is called before the backtest starts
	Initialize(engine *Engine) error

	// GenerateSignals generates trading signals at each time step
	GenerateSignals(engine *Engine) ([]*Signal, error)

	// Finalize is called after the backtest ends
	Finalize(engine *Engine) error
}
