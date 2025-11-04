package backtest

import (
	"context"
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
	adapter.AddAgent(NewSMAAgent("sma-1"))
	adapter.AddAgent(NewSMAAgent("sma-2"))

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
	adapter.AddAgent(NewSMAAgent("sma-agent"))

	engine := createAgentTestEngineWithData()
	adapter.Initialize(engine)

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
	adapter.AddAgent(NewSMAAgent("sma-1"))
	adapter.AddAgent(NewSMAAgent("sma-2"))

	engine := createAgentTestEngineWithData()
	adapter.Initialize(engine)

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
	adapter.AddAgent(NewBullishAgent("bull"))
	adapter.AddAgent(NewBearishAgent("bear"))

	engine := createAgentTestEngineWithData()
	adapter.Initialize(engine)

	// Generate signals
	for i := 0; i < 5; i++ {
		adapter.GenerateSignals(engine)
		engine.Step(context.Background())
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
	engine.LoadHistoricalData("BTC/USD", candlesticks)

	// Create agent replay adapter
	adapter := NewAgentReplayAdapter(ConsensusMajority)
	adapter.AddAgent(NewSMAAgent("sma-agent"))

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
	engine.LoadHistoricalData("BTC/USD", candlesticks)

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
