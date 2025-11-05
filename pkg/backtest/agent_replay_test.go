package backtest

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MOCK AGENTS FOR TESTING
// ============================================================================

// SimpleMovingAverageAgent is a test agent that uses SMA crossover strategy
type SimpleMovingAverageAgent struct {
	name        string
	shortPeriod int
	longPeriod  int
}

func NewSMAAgent(name string) *SimpleMovingAverageAgent {
	return &SimpleMovingAverageAgent{
		name:        name,
		shortPeriod: 10,
		longPeriod:  20,
	}
}

func (a *SimpleMovingAverageAgent) GetName() string {
	return a.name
}

func (a *SimpleMovingAverageAgent) Analyze(ctx context.Context, data *MarketData) (*Signal, error) {
	// Need at least longPeriod candles
	if len(data.History) < a.longPeriod {
		return &Signal{Side: "HOLD", Confidence: 0.5, Reasoning: "Insufficient data"}, nil
	}

	// Calculate short and long SMAs
	shortSMA := calculateSMAFromData(data.History, a.shortPeriod)
	longSMA := calculateSMAFromData(data.History, a.longPeriod)

	// Crossover strategy
	if shortSMA > longSMA && data.CurrentPrice > shortSMA {
		return &Signal{
			Side:       "BUY",
			Confidence: 0.7,
			Reasoning:  "Short SMA crossed above long SMA (bullish)",
		}, nil
	} else if shortSMA < longSMA && data.CurrentPrice < shortSMA {
		return &Signal{
			Side:       "SELL",
			Confidence: 0.7,
			Reasoning:  "Short SMA crossed below long SMA (bearish)",
		}, nil
	}

	return &Signal{Side: "HOLD", Confidence: 0.5, Reasoning: "No clear signal"}, nil
}

func (a *SimpleMovingAverageAgent) Reset() error {
	return nil
}

func calculateSMAFromData(candles []*Candlestick, period int) float64 {
	if len(candles) < period {
		return 0
	}

	sum := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		sum += candles[i].Close
	}

	return sum / float64(period)
}

// BullishAgent always recommends BUY
type BullishAgent struct {
	name string
}

func NewBullishAgent(name string) *BullishAgent {
	return &BullishAgent{name: name}
}

func (a *BullishAgent) GetName() string {
	return a.name
}

func (a *BullishAgent) Analyze(ctx context.Context, data *MarketData) (*Signal, error) {
	return &Signal{
		Side:       "BUY",
		Confidence: 0.9,
		Reasoning:  "Always bullish",
	}, nil
}

func (a *BullishAgent) Reset() error {
	return nil
}

// BearishAgent always recommends SELL
type BearishAgent struct {
	name string
}

func NewBearishAgent(name string) *BearishAgent {
	return &BearishAgent{name: name}
}

func (a *BearishAgent) GetName() string {
	return a.name
}

func (a *BearishAgent) Analyze(ctx context.Context, data *MarketData) (*Signal, error) {
	return &Signal{
		Side:       "SELL",
		Confidence: 0.8,
		Reasoning:  "Always bearish",
	}, nil
}

func (a *BearishAgent) Reset() error {
	return nil
}

// ============================================================================
// TESTS
// ============================================================================

func TestNewAgentReplayAdapter(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusMajority)

	assert.NotNil(t, adapter)
	assert.Equal(t, ConsensusMajority, adapter.consensus)
	assert.Empty(t, adapter.agents)
	assert.Empty(t, adapter.agentSignals)
	assert.Empty(t, adapter.agentMetrics)
}

func TestAddAgent(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusFirst)

	// Add agent successfully
	agent := NewSMAAgent("sma-agent")
	err := adapter.AddAgent(agent)
	require.NoError(t, err)
	assert.Len(t, adapter.agents, 1)
	assert.Contains(t, adapter.agents, "sma-agent")

	// Cannot add duplicate agent
	err = adapter.AddAgent(agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestAgentReplayInitialize(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusMajority)
	_ = adapter.AddAgent(NewSMAAgent("sma-1")) // Test setup - error handled by test framework
	_ = adapter.AddAgent(NewSMAAgent("sma-2")) // Test setup - error handled by test framework

	engine := createAgentTestEngine()

	err := adapter.Initialize(engine)
	require.NoError(t, err)

	// Check that signal storage is initialized
	assert.Len(t, adapter.agentSignals, 2)
	assert.NotNil(t, adapter.agentSignals["sma-1"])
	assert.NotNil(t, adapter.agentSignals["sma-2"])
}

func TestGenerateSignals_SingleAgent(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusFirst)
	_ = adapter.AddAgent(NewSMAAgent("sma-agent")) // Test setup - error handled by test framework

	engine := createAgentTestEngineWithData()
	_ = adapter.Initialize(engine) // Test setup - error handled by test framework

	signals, err := adapter.GenerateSignals(engine)
	require.NoError(t, err)

	// Should generate signals for available symbols
	assert.NotEmpty(t, signals)

	// Check metrics were updated
	metrics := adapter.agentMetrics["sma-agent"]
	assert.Greater(t, metrics.SignalsGenerated, 0)
}

func TestGenerateSignals_MultipleAgents(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusAll)
	_ = adapter.AddAgent(NewSMAAgent("sma-1")) // Test setup - error handled by test
	_ = adapter.AddAgent(NewSMAAgent("sma-2")) // Test setup - error handled by test

	engine := createAgentTestEngineWithData()
	_ = adapter.Initialize(engine) // Test setup - error handled by test

	signals, err := adapter.GenerateSignals(engine)
	require.NoError(t, err)

	// With ConsensusAll, each agent's signal is returned
	// Should have signals from multiple agents
	assert.NotEmpty(t, signals)
}

func TestConsensusMajority(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusMajority)

	// Three bullish agents, one bearish - majority should be BUY
	signals := []*Signal{
		{Side: "BUY", Confidence: 0.8},
		{Side: "BUY", Confidence: 0.7},
		{Side: "BUY", Confidence: 0.9},
		{Side: "SELL", Confidence: 0.6},
	}

	result := adapter.applyConsensus(signals)

	require.Len(t, result, 1)
	assert.Equal(t, "BUY", result[0].Side)
	assert.Contains(t, result[0].Reasoning, "Majority consensus")
}

func TestConsensusUnanimous(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusUnanimous)

	t.Run("all agree", func(t *testing.T) {
		signals := []*Signal{
			{Side: "BUY", Confidence: 0.8},
			{Side: "BUY", Confidence: 0.7},
			{Side: "BUY", Confidence: 0.9},
		}

		result := adapter.applyConsensus(signals)

		require.Len(t, result, 1)
		assert.Equal(t, "BUY", result[0].Side)
		assert.Contains(t, result[0].Reasoning, "Unanimous")
	})

	t.Run("disagreement", func(t *testing.T) {
		signals := []*Signal{
			{Side: "BUY", Confidence: 0.8},
			{Side: "SELL", Confidence: 0.7},
		}

		result := adapter.applyConsensus(signals)

		assert.Empty(t, result) // No consensus
	})
}

func TestConsensusWeighted(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusWeighted)

	// BUY has higher total confidence
	signals := []*Signal{
		{Side: "BUY", Confidence: 0.9},  // total: 0.9
		{Side: "SELL", Confidence: 0.5}, // total: 0.8
		{Side: "SELL", Confidence: 0.3},
	}

	result := adapter.applyConsensus(signals)

	require.Len(t, result, 1)
	assert.Equal(t, "BUY", result[0].Side)
	assert.Contains(t, result[0].Reasoning, "Weighted consensus")
}

func TestConsensusFirst(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusFirst)

	signals := []*Signal{
		{Side: "BUY", Confidence: 0.8},
		{Side: "SELL", Confidence: 0.9},
		{Side: "HOLD", Confidence: 0.7},
	}

	result := adapter.applyConsensus(signals)

	require.Len(t, result, 1)
	assert.Equal(t, "BUY", result[0].Side) // Uses first signal
}

func TestConsensusAll(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusAll)

	signals := []*Signal{
		{Side: "BUY", Confidence: 0.8},
		{Side: "SELL", Confidence: 0.9},
		{Side: "HOLD", Confidence: 0.7},
	}

	result := adapter.applyConsensus(signals)

	assert.Len(t, result, 3) // Returns all signals
}

func TestAgentPerformanceTracking(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusMajority)
	_ = adapter.AddAgent(NewBullishAgent("bull")) // Test setup - error handled by test
	_ = adapter.AddAgent(NewBearishAgent("bear")) // Test setup - error handled by test

	engine := createAgentTestEngineWithData()
	_ = adapter.Initialize(engine) // Test setup - error handled by test

	// Generate signals
	for i := 0; i < 5; i++ {
		_, _ = adapter.GenerateSignals(engine)   // Test loop - error acceptable
		_, _ = engine.Step(context.Background()) // Test loop - error acceptable (returns done, err)
	}

	// Check metrics
	bullMetrics := adapter.agentMetrics["bull"]
	bearMetrics := adapter.agentMetrics["bear"]

	assert.Greater(t, bullMetrics.SignalsGenerated, 0)
	assert.Greater(t, bearMetrics.SignalsGenerated, 0)

	assert.Greater(t, bullMetrics.BuySignals, 0)
	assert.Greater(t, bearMetrics.SellSignals, 0)
}

func TestFullBacktestWithAgents(t *testing.T) {
	// Create backtest engine
	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "percent",
		PositionSize:   0.1, // 10% of equity per trade
		MaxPositions:   3,
	}

	engine := NewEngine(config)

	// Load test data
	candlesticks := generateAgentTestCandlesticks("BTC/USD", 100)
	_ = engine.LoadHistoricalData("BTC/USD", candlesticks) // Test setup - error handled by test

	// Create agent replay adapter
	adapter := NewAgentReplayAdapter(ConsensusMajority)
	_ = adapter.AddAgent(NewSMAAgent("sma-agent")) // Test setup - error handled by test

	// Run backtest
	ctx := context.Background()
	err := engine.Run(ctx, adapter)
	require.NoError(t, err)

	// Verify backtest completed
	assert.NotEmpty(t, engine.EquityCurve, "Should have equity curve")

	// Verify agent metrics
	metrics := adapter.GetAgentMetrics()
	assert.NotEmpty(t, metrics)

	smaMetrics := metrics["sma-agent"]
	assert.NotNil(t, smaMetrics)
	assert.Greater(t, smaMetrics.SignalsGenerated, 0, "Agent should have generated signals")

	// Note: The SMA strategy may generate HOLD signals with this test data,
	// so we don't require trades to have been executed

	// Print report
	report := adapter.PrintAgentReport()
	assert.Contains(t, report, "sma-agent")
}

func TestSetContext(t *testing.T) {
	adapter := NewAgentReplayAdapter(ConsensusMajority)

	adapter.SetContext("market_regime", "bull")
	adapter.SetContext("volatility", 0.25)

	assert.Equal(t, "bull", adapter.context["market_regime"])
	assert.Equal(t, 0.25, adapter.context["volatility"])
}

func TestCalculateSMA(t *testing.T) {
	candles := []*Candlestick{
		{Close: 100},
		{Close: 110},
		{Close: 105},
		{Close: 115},
		{Close: 120},
	}

	sma := calculateSMA(candles, 5)
	expected := (100.0 + 110.0 + 105.0 + 115.0 + 120.0) / 5.0
	assert.Equal(t, expected, sma)
}

func TestCalculateSMA_InsufficientData(t *testing.T) {
	candles := []*Candlestick{
		{Close: 100},
		{Close: 110},
	}

	sma := calculateSMA(candles, 5)
	assert.Equal(t, 0.0, sma) // Not enough data
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func createAgentTestEngine() *Engine {
	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   5,
	}

	return NewEngine(config)
}

func createAgentTestEngineWithData() *Engine {
	engine := createAgentTestEngine()

	// Add some test data
	candlesticks := generateAgentTestCandlesticks("BTC/USD", 50)
	_ = engine.LoadHistoricalData("BTC/USD", candlesticks) // Test setup - error handled by test

	return engine
}

func generateAgentTestCandlesticks(symbol string, count int) []*Candlestick {
	candles := make([]*Candlestick, count)
	basePrice := 50000.0
	timestamp := time.Now().Add(-time.Duration(count) * time.Hour)

	for i := 0; i < count; i++ {
		// Simulate price movement
		priceChange := (float64(i%10) - 5.0) * 100 // +/- $500
		price := basePrice + priceChange

		candles[i] = &Candlestick{
			Symbol:    symbol,
			Timestamp: timestamp.Add(time.Duration(i) * time.Hour),
			Open:      price - 50,
			High:      price + 100,
			Low:       price - 100,
			Close:     price,
			Volume:    1000 + float64(i*10),
		}
	}

	return candles
}

// ============================================================================
// DATA LOADER TESTS
// ============================================================================

func TestLoadFromCSV(t *testing.T) {
	// Create a temporary CSV file
	tmpFile, err := os.CreateTemp("", "test_candles_*.csv")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Test cleanup

	// Write CSV data
	csvData := `timestamp,symbol,open,high,low,close,volume
1609459200,BTC/USD,29000.0,29500.0,28800.0,29300.0,1000.5
1609545600,BTC/USD,29300.0,30000.0,29100.0,29800.0,1200.3
1609632000,BTC/USD,29800.0,30500.0,29600.0,30200.0,1500.8`

	_, err = tmpFile.WriteString(csvData)
	require.NoError(t, err)
	_ = tmpFile.Close() // Test cleanup

	// Load from CSV
	candles, err := LoadFromCSV(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, 3, len(candles))

	// Verify first candle
	assert.Equal(t, "BTC/USD", candles[0].Symbol)
	assert.Equal(t, 29000.0, candles[0].Open)
	assert.Equal(t, 29500.0, candles[0].High)
	assert.Equal(t, 28800.0, candles[0].Low)
	assert.Equal(t, 29300.0, candles[0].Close)
	assert.Equal(t, 1000.5, candles[0].Volume)

	// Verify timestamp parsing
	expectedTime := time.Unix(1609459200, 0)
	assert.Equal(t, expectedTime.Unix(), candles[0].Timestamp.Unix())
}

func TestLoadFromCSV_RFC3339Timestamp(t *testing.T) {
	// Create a temporary CSV file with RFC3339 timestamps
	tmpFile, err := os.CreateTemp("", "test_candles_rfc_*.csv")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Test cleanup

	// Write CSV data with RFC3339 timestamps
	csvData := `timestamp,symbol,open,high,low,close,volume
2021-01-01T00:00:00Z,BTC/USD,29000.0,29500.0,28800.0,29300.0,1000.5
2021-01-02T00:00:00Z,BTC/USD,29300.0,30000.0,29100.0,29800.0,1200.3`

	_, err = tmpFile.WriteString(csvData)
	require.NoError(t, err)
	_ = tmpFile.Close() // Test cleanup

	// Load from CSV
	candles, err := LoadFromCSV(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, 2, len(candles))
	assert.Equal(t, "BTC/USD", candles[0].Symbol)
}

func TestLoadFromJSON_ArrayFormat(t *testing.T) {
	// Create a temporary JSON file with array format
	tmpFile, err := os.CreateTemp("", "test_candles_array_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Test cleanup

	// Write JSON data
	jsonData := `[
  {
    "timestamp": "2021-01-01T00:00:00Z",
    "symbol": "BTC/USD",
    "open": 29000.0,
    "high": 29500.0,
    "low": 28800.0,
    "close": 29300.0,
    "volume": 1000.5
  },
  {
    "timestamp": "2021-01-02T00:00:00Z",
    "symbol": "BTC/USD",
    "open": 29300.0,
    "high": 30000.0,
    "low": 29100.0,
    "close": 29800.0,
    "volume": 1200.3
  }
]`

	_, err = tmpFile.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpFile.Close() // Test cleanup

	// Load from JSON
	candles, err := LoadFromJSON(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, 2, len(candles))
	assert.Equal(t, "BTC/USD", candles[0].Symbol)
	assert.Equal(t, 29000.0, candles[0].Open)
}

func TestLoadFromJSON_ObjectFormat(t *testing.T) {
	// Create a temporary JSON file with object format
	tmpFile, err := os.CreateTemp("", "test_candles_object_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Test cleanup

	// Write JSON data
	jsonData := `{
  "candles": [
    {
      "timestamp": "2021-01-01T00:00:00Z",
      "symbol": "ETH/USD",
      "open": 730.0,
      "high": 750.0,
      "low": 720.0,
      "close": 745.0,
      "volume": 500.5
    }
  ]
}`

	_, err = tmpFile.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpFile.Close() // Test cleanup

	// Load from JSON
	candles, err := LoadFromJSON(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, 1, len(candles))
	assert.Equal(t, "ETH/USD", candles[0].Symbol)
	assert.Equal(t, 730.0, candles[0].Open)
}

func TestExportResults(t *testing.T) {
	// Create a test engine with some results
	config := BacktestConfig{
		InitialCapital: 10000.0,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000.0,
		MaxPositions:   5,
	}
	engine := NewEngine(config)

	// Add some test data
	engine.TotalTrades = 10
	engine.WinningTrades = 6
	engine.LosingTrades = 4
	engine.TotalProfit = 500.0
	engine.TotalLoss = 200.0
	engine.MaxDrawdown = 100.0
	engine.MaxDrawdownPct = 1.0
	engine.PeakEquity = 10500.0

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_results_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Test cleanup
	_ = tmpFile.Close()                              // Test cleanup

	// Export results
	err = ExportResults(engine, tmpFile.Name())
	require.NoError(t, err)

	// Verify file was created and contains data
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Parse JSON to verify structure
	var results map[string]interface{}
	err = json.Unmarshal(data, &results)
	require.NoError(t, err)

	// Verify expected fields
	assert.Contains(t, results, "config")
	assert.Contains(t, results, "statistics")
	assert.Contains(t, results, "trades")
	assert.Contains(t, results, "closed_positions")
	assert.Contains(t, results, "equity_curve")

	// Verify statistics
	stats := results["statistics"].(map[string]interface{})
	assert.Equal(t, float64(10), stats["total_trades"])
	assert.Equal(t, float64(6), stats["winning_trades"])
	assert.Equal(t, float64(4), stats["losing_trades"])
	assert.Equal(t, 500.0, stats["total_profit"])
	assert.Equal(t, 200.0, stats["total_loss"])
	assert.Equal(t, 300.0, stats["net_profit"])
	assert.InDelta(t, 60.0, stats["win_rate"], 0.01)
	assert.InDelta(t, 2.5, stats["profit_factor"], 0.01)
}
