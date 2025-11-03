// Agent replay mode for backtesting with trading agents
package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// AGENT REPLAY ADAPTER
// ============================================================================

// AgentReplayAdapter adapts trading agents to work with the backtest engine
type AgentReplayAdapter struct {
	agents       map[string]Agent // agent name -> agent instance
	agentSignals map[string][]*Signal
	// agent name -> signals generated
	agentMetrics map[string]*AgentPerformance // agent name -> performance metrics
	consensus    ConsensusStrategy            // How to combine signals from multiple agents
	context      map[string]interface{}       // Shared context for agents
}

// Agent represents a trading agent that can generate signals
type Agent interface {
	// GetName returns the agent's unique identifier
	GetName() string

	// Analyze receives market data and returns a trading signal
	Analyze(ctx context.Context, data *MarketData) (*Signal, error)

	// Reset resets the agent's internal state (for multi-run backtests)
	Reset() error
}

// MarketData represents market data available to an agent at a point in time
type MarketData struct {
	Timestamp    time.Time         `json:"timestamp"`
	Symbol       string            `json:"symbol"`
	CurrentPrice float64           `json:"current_price"`
	OHLCV        *Candlestick      `json:"current_candle"`
	History      []*Candlestick    `json:"historical_candles"`
	Indicators   map[string]float64 `json:"indicators,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// AgentPerformance tracks individual agent performance during backtest
type AgentPerformance struct {
	AgentName        string  `json:"agent_name"`
	SignalsGenerated int     `json:"signals_generated"`
	BuySignals       int     `json:"buy_signals"`
	SellSignals      int     `json:"sell_signals"`
	HoldSignals      int     `json:"hold_signals"`
	SignalsExecuted  int     `json:"signals_executed"` // Signals that resulted in trades
	AvgConfidence    float64 `json:"average_confidence"`
	CorrectSignals   int     `json:"correct_signals"`   // Signals that led to profitable trades
	IncorrectSignals int     `json:"incorrect_signals"` // Signals that led to losing trades
	Accuracy         float64 `json:"accuracy"`          // Percentage of correct signals
}

// ConsensusStrategy defines how signals from multiple agents are combined
type ConsensusStrategy string

const (
	ConsensusMajority  ConsensusStrategy = "majority"  // Follow majority vote
	ConsensusUnanimous ConsensusStrategy = "unanimous" // All agents must agree
	ConsensusWeighted  ConsensusStrategy = "weighted"  // Weight by agent confidence
	ConsensusFirst     ConsensusStrategy = "first"     // Use first agent only
	ConsensusAll       ConsensusStrategy = "all"       // Execute all agent signals independently
)

// NewAgentReplayAdapter creates a new agent replay adapter
func NewAgentReplayAdapter(consensus ConsensusStrategy) *AgentReplayAdapter {
	return &AgentReplayAdapter{
		agents:       make(map[string]Agent),
		agentSignals: make(map[string][]*Signal),
		agentMetrics: make(map[string]*AgentPerformance),
		consensus:    consensus,
		context:      make(map[string]interface{}),
	}
}

// AddAgent registers an agent for replay
func (a *AgentReplayAdapter) AddAgent(agent Agent) error {
	name := agent.GetName()
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	if _, exists := a.agents[name]; exists {
		return fmt.Errorf("agent %s already registered", name)
	}

	a.agents[name] = agent
	a.agentMetrics[name] = &AgentPerformance{
		AgentName: name,
	}

	log.Info().
		Str("agent", name).
		Str("consensus", string(a.consensus)).
		Msg("Registered agent for replay")

	return nil
}

// SetContext sets shared context data for all agents
func (a *AgentReplayAdapter) SetContext(key string, value interface{}) {
	a.context[key] = value
}

// Initialize implements the Strategy interface for the backtest engine
func (a *AgentReplayAdapter) Initialize(engine *Engine) error {
	log.Info().
		Int("agents", len(a.agents)).
		Str("consensus", string(a.consensus)).
		Msg("Initializing agent replay mode")

	// Reset all agents
	for name, agent := range a.agents {
		if err := agent.Reset(); err != nil {
			log.Warn().
				Err(err).
				Str("agent", name).
				Msg("Failed to reset agent")
		}
	}

	// Initialize agent signal storage
	for name := range a.agents {
		a.agentSignals[name] = make([]*Signal, 0)
	}

	return nil
}

// GenerateSignals implements the Strategy interface for the backtest engine
func (a *AgentReplayAdapter) GenerateSignals(engine *Engine) ([]*Signal, error) {
	var allSignals []*Signal

	// For each symbol with data
	for symbol := range engine.Data {
		// Get current candle
		currentCandle, err := engine.GetCurrentCandle(symbol)
		if err != nil {
			continue // No more data for this symbol
		}

		// Get historical candles (lookback 100 periods)
		historicalCandles, err := engine.GetHistoricalCandles(symbol, 100)
		if err != nil {
			historicalCandles = []*Candlestick{}
		}

		// Prepare market data for agents
		marketData := &MarketData{
			Timestamp:    currentCandle.Timestamp,
			Symbol:       symbol,
			CurrentPrice: currentCandle.Close,
			OHLCV:        currentCandle,
			History:      historicalCandles,
			Indicators:   make(map[string]float64),
			Context:      a.context,
		}

		// Calculate basic indicators (agents can compute their own too)
		if len(historicalCandles) >= 20 {
			marketData.Indicators["sma_20"] = calculateSMA(append(historicalCandles, currentCandle), 20)
		}

		// Get signals from all agents
		agentSignals := make([]*Signal, 0)
		ctx := context.Background()

		for name, agent := range a.agents {
			signal, err := agent.Analyze(ctx, marketData)
			if err != nil {
				log.Warn().
					Err(err).
					Str("agent", name).
					Str("symbol", symbol).
					Msg("Agent analysis failed")
				continue
			}

			if signal != nil {
				// Ensure signal has required fields
				signal.Symbol = symbol
				signal.Timestamp = currentCandle.Timestamp
				signal.Agent = name

				// Track signal
				agentSignals = append(agentSignals, signal)
				a.agentSignals[name] = append(a.agentSignals[name], signal)

				// Update agent metrics
				metrics := a.agentMetrics[name]
				metrics.SignalsGenerated++
				metrics.AvgConfidence = (metrics.AvgConfidence*float64(metrics.SignalsGenerated-1) + signal.Confidence) / float64(metrics.SignalsGenerated)

				switch signal.Side {
				case "BUY":
					metrics.BuySignals++
				case "SELL":
					metrics.SellSignals++
				case "HOLD":
					metrics.HoldSignals++
				}
			}
		}

		// Apply consensus strategy to combine agent signals
		finalSignals := a.applyConsensus(agentSignals)
		allSignals = append(allSignals, finalSignals...)
	}

	return allSignals, nil
}

// Finalize implements the Strategy interface for the backtest engine
func (a *AgentReplayAdapter) Finalize(engine *Engine) error {
	log.Info().Msg("Finalizing agent replay - calculating agent performance")

	// Calculate final agent metrics
	for name, metrics := range a.agentMetrics {
		// Calculate accuracy based on closed positions
		correctSignals, incorrectSignals := a.calculateSignalAccuracy(name, engine.ClosedPositions)
		metrics.CorrectSignals = correctSignals
		metrics.IncorrectSignals = incorrectSignals

		totalEvaluated := correctSignals + incorrectSignals
		if totalEvaluated > 0 {
			metrics.Accuracy = (float64(correctSignals) / float64(totalEvaluated)) * 100.0
		}

		log.Info().
			Str("agent", name).
			Int("signals", metrics.SignalsGenerated).
			Float64("confidence", metrics.AvgConfidence).
			Float64("accuracy", metrics.Accuracy).
			Msg("Agent performance")
	}

	return nil
}

// applyConsensus combines signals from multiple agents based on consensus strategy
func (a *AgentReplayAdapter) applyConsensus(signals []*Signal) []*Signal {
	if len(signals) == 0 {
		return []*Signal{}
	}

	switch a.consensus {
	case ConsensusFirst:
		// Use only the first agent's signal
		return signals[:1]

	case ConsensusAll:
		// Execute all signals independently
		return signals

	case ConsensusMajority:
		// Count votes for each action
		votes := make(map[string]int)
		for _, signal := range signals {
			votes[signal.Side]++
		}

		// Find majority
		maxVotes := 0
		majority := "HOLD"
		for side, count := range votes {
			if count > maxVotes {
				maxVotes = count
				majority = side
			}
		}

		// Return a signal with the majority vote and average confidence
		if majority != "HOLD" {
			avgConfidence := 0.0
			count := 0
			for _, signal := range signals {
				if signal.Side == majority {
					avgConfidence += signal.Confidence
					count++
				}
			}
			avgConfidence /= float64(count)

			return []*Signal{{
				Side:       majority,
				Confidence: avgConfidence,
				Reasoning:  fmt.Sprintf("Majority consensus: %d/%d agents voted %s", maxVotes, len(signals), majority),
				Agent:      "consensus",
			}}
		}
		return []*Signal{}

	case ConsensusUnanimous:
		// All agents must agree
		if len(signals) == 0 {
			return []*Signal{}
		}

		firstSide := signals[0].Side
		for _, signal := range signals {
			if signal.Side != firstSide {
				return []*Signal{} // No consensus
			}
		}

		// All agree - return combined signal
		avgConfidence := 0.0
		for _, signal := range signals {
			avgConfidence += signal.Confidence
		}
		avgConfidence /= float64(len(signals))

		return []*Signal{{
			Side:       firstSide,
			Confidence: avgConfidence,
			Reasoning:  fmt.Sprintf("Unanimous consensus: all %d agents voted %s", len(signals), firstSide),
			Agent:      "consensus",
		}}

	case ConsensusWeighted:
		// Weight by confidence
		weightedVotes := make(map[string]float64)
		for _, signal := range signals {
			weightedVotes[signal.Side] += signal.Confidence
		}

		// Find highest weighted vote
		maxWeight := 0.0
		bestSide := "HOLD"
		for side, weight := range weightedVotes {
			if weight > maxWeight {
				maxWeight = weight
				bestSide = side
			}
		}

		if bestSide != "HOLD" {
			totalConfidence := 0.0
			for _, signal := range signals {
				totalConfidence += signal.Confidence
			}

			return []*Signal{{
				Side:       bestSide,
				Confidence: maxWeight / float64(len(signals)),
				Reasoning:  fmt.Sprintf("Weighted consensus: %s with %.2f total confidence", bestSide, maxWeight),
				Agent:      "consensus",
			}}
		}
		return []*Signal{}

	default:
		return signals
	}
}

// calculateSignalAccuracy compares agent signals to actual trade outcomes
func (a *AgentReplayAdapter) calculateSignalAccuracy(agentName string, closedPositions []*ClosedPosition) (correct, incorrect int) {
	signals := a.agentSignals[agentName]

	for _, pos := range closedPositions {
		// Find the signal that led to this position
		var entrySignal *Signal
		for _, sig := range signals {
			if sig.Symbol == pos.Symbol &&
				sig.Side == "BUY" &&
				sig.Timestamp.Before(pos.EntryTime.Add(1*time.Minute)) &&
				sig.Timestamp.After(pos.EntryTime.Add(-1*time.Minute)) {
				entrySignal = sig
				break
			}
		}

		if entrySignal != nil {
			if pos.RealizedPL > 0 {
				correct++
			} else {
				incorrect++
			}
		}
	}

	return correct, incorrect
}

// GetAgentMetrics returns performance metrics for all agents
func (a *AgentReplayAdapter) GetAgentMetrics() map[string]*AgentPerformance {
	return a.agentMetrics
}

// PrintAgentReport prints a summary of agent performance
func (a *AgentReplayAdapter) PrintAgentReport() string {
	report := "\n"
	report += "================================================================================\n"
	report += "AGENT REPLAY PERFORMANCE REPORT\n"
	report += "================================================================================\n\n"

	for name, metrics := range a.agentMetrics {
		report += fmt.Sprintf("Agent: %s\n", name)
		report += fmt.Sprintf("  Signals Generated:  %d\n", metrics.SignalsGenerated)
		report += fmt.Sprintf("    - BUY signals:    %d\n", metrics.BuySignals)
		report += fmt.Sprintf("    - SELL signals:   %d\n", metrics.SellSignals)
		report += fmt.Sprintf("    - HOLD signals:   %d\n", metrics.HoldSignals)
		report += fmt.Sprintf("  Signals Executed:   %d\n", metrics.SignalsExecuted)
		report += fmt.Sprintf("  Average Confidence: %.2f%%\n", metrics.AvgConfidence*100)
		report += fmt.Sprintf("  Accuracy:           %.2f%% (%d correct, %d incorrect)\n",
			metrics.Accuracy, metrics.CorrectSignals, metrics.IncorrectSignals)
		report += "\n"
	}

	report += "================================================================================\n"

	return report
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// calculateSMA calculates Simple Moving Average
func calculateSMA(candles []*Candlestick, period int) float64 {
	if len(candles) < period {
		return 0
	}

	sum := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		sum += candles[i].Close
	}

	return sum / float64(period)
}

// ============================================================================
// DATA LOADER FROM DATABASE
// ============================================================================

// HistoricalDataLoader loads historical candlestick data from database
type HistoricalDataLoader struct {
	// Database connection would go here
	// For now, this is a placeholder for the database integration
}

// LoadFromDatabase loads historical data for backtesting from TimescaleDB
// This is a placeholder - actual implementation would query the database
func (h *HistoricalDataLoader) LoadFromDatabase(symbol, exchange, interval string, startDate, endDate time.Time) ([]*Candlestick, error) {
	// TODO: Implement actual database query
	// Query would look like:
	// SELECT symbol, open_time as timestamp, open, high, low, close, volume
	// FROM candlesticks
	// WHERE symbol = $1 AND exchange = $2 AND interval = $3
	//   AND open_time >= $4 AND open_time <= $5
	// ORDER BY open_time ASC

	return nil, fmt.Errorf("database loader not yet implemented - use CSV loader instead")
}

// LoadFromCSV loads historical data from a CSV file
func LoadFromCSV(filepath string) ([]*Candlestick, error) {
	// TODO: Implement CSV loader for historical data
	// CSV format: timestamp,symbol,open,high,low,close,volume
	return nil, fmt.Errorf("CSV loader not yet implemented")
}

// LoadFromJSON loads historical data from a JSON file
func LoadFromJSON(filepath string) ([]*Candlestick, error) {
	// TODO: Implement JSON loader for historical data
	return nil, fmt.Errorf("JSON loader not yet implemented")
}

// ExportResults exports backtest results to JSON
func ExportResults(engine *Engine, filepath string) error {
	results := map[string]interface{}{
		"config": map[string]interface{}{
			"initial_capital": engine.InitialCapital,
			"commission_rate": engine.CommissionRate,
			"position_sizing": engine.PositionSizing,
			"max_positions":   engine.MaxPositions,
		},
		"trades":           engine.Trades,
		"closed_positions": engine.ClosedPositions,
		"equity_curve":     engine.EquityCurve,
		"statistics": map[string]interface{}{
			"total_trades":     engine.TotalTrades,
			"winning_trades":   engine.WinningTrades,
			"losing_trades":    engine.LosingTrades,
			"total_profit":     engine.TotalProfit,
			"total_loss":       engine.TotalLoss,
			"max_drawdown":     engine.MaxDrawdown,
			"max_drawdown_pct": engine.MaxDrawdownPct,
			"peak_equity":      engine.PeakEquity,
		},
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	// TODO: Write to file
	_ = data
	return fmt.Errorf("file export not yet implemented")
}
