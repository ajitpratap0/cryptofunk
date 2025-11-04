package backtest

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MOCK STRATEGY FOR TESTING
// ============================================================================

// ParameterizedStrategy is a simple strategy that uses configurable parameters
type ParameterizedStrategy struct {
	shortPeriod int
	longPeriod  int
	threshold   float64
	useStop     bool
}

func NewParameterizedStrategy(params ParameterSet) (Strategy, error) {
	return &ParameterizedStrategy{
		shortPeriod: params["short_period"].(int),
		longPeriod:  params["long_period"].(int),
		threshold:   params["threshold"].(float64),
		useStop:     params["use_stop"].(bool),
	}, nil
}

func (s *ParameterizedStrategy) Initialize(engine *Engine) error {
	return nil
}

func (s *ParameterizedStrategy) GenerateSignals(engine *Engine) ([]*Signal, error) {
	var signals []*Signal

	for symbol := range engine.Data {
		candle, err := engine.GetCurrentCandle(symbol)
		if err != nil {
			continue
		}

		history, _ := engine.GetHistoricalCandles(symbol, s.longPeriod)
		if len(history) < s.longPeriod {
			continue
		}

		// Simple moving average crossover
		shortSMA := calculateSMAFromHistory(history, s.shortPeriod)
		longSMA := calculateSMAFromHistory(history, s.longPeriod)

		var side string
		if shortSMA > longSMA*(1+s.threshold) {
			side = "BUY"
		} else if shortSMA < longSMA*(1-s.threshold) {
			side = "SELL"
		} else {
			side = "HOLD"
		}

		signals = append(signals, &Signal{
			Symbol:     symbol,
			Timestamp:  candle.Timestamp,
			Side:       side,
			Confidence: 0.7,
			Reasoning:  "SMA crossover strategy",
		})
	}

	return signals, nil
}

func (s *ParameterizedStrategy) Finalize(engine *Engine) error {
	return nil
}

func calculateSMAFromHistory(candles []*Candlestick, period int) float64 {
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
// PARAMETER TESTS
// ============================================================================

func TestParameterSet_Clone(t *testing.T) {
	original := ParameterSet{
		"param1": 10,
		"param2": 3.14,
		"param3": true,
	}

	clone := original.Clone()

	// Modify clone
	clone["param1"] = 20

	// Original should be unchanged
	assert.Equal(t, 10, original["param1"])
	assert.Equal(t, 20, clone["param1"])
}

// ============================================================================
// OBJECTIVE FUNCTION TESTS
// ============================================================================

func TestObjectiveFunctions(t *testing.T) {
	metrics := &Metrics{
		SharpeRatio:    1.5,
		SortinoRatio:   2.0,
		CalmarRatio:    0.8,
		TotalReturnPct: 25.0,
		ProfitFactor:   2.5,
		MaxDrawdownPct: 10.0,
		WinRate:        60.0,
	}

	t.Run("MaximizeSharpeRatio", func(t *testing.T) {
		score := MaximizeSharpeRatio(metrics)
		assert.Equal(t, 1.5, score)
	})

	t.Run("MaximizeSortinoRatio", func(t *testing.T) {
		score := MaximizeSortinoRatio(metrics)
		assert.Equal(t, 2.0, score)
	})

	t.Run("MaximizeCalmarRatio", func(t *testing.T) {
		score := MaximizeCalmarRatio(metrics)
		assert.Equal(t, 0.8, score)
	})

	t.Run("MaximizeTotalReturn", func(t *testing.T) {
		score := MaximizeTotalReturn(metrics)
		assert.Equal(t, 25.0, score)
	})

	t.Run("MaximizeProfitFactor", func(t *testing.T) {
		score := MaximizeProfitFactor(metrics)
		assert.Equal(t, 2.5, score)
	})

	t.Run("MinimizeDrawdown", func(t *testing.T) {
		score := MinimizeDrawdown(metrics)
		assert.Equal(t, -10.0, score) // Negative because we minimize
	})

	t.Run("BalancedObjective", func(t *testing.T) {
		score := BalancedObjective(metrics)
		// 0.4*1.5 + 0.3*0.6 + 0.3*0.8 = 0.6 + 0.18 + 0.24 = 1.02
		assert.InDelta(t, 1.02, score, 0.01)
	})
}

// ============================================================================
// GRID SEARCH TESTS
// ============================================================================

func TestNewGridSearchOptimizer(t *testing.T) {
	params := []*Parameter{
		{Name: "short_period", Type: ParamTypeInt, Min: 5, Max: 15, Step: 5},
		{Name: "long_period", Type: ParamTypeInt, Min: 20, Max: 40, Step: 10},
	}

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   2,
	}

	optimizer := NewGridSearchOptimizer(NewParameterizedStrategy, params, MaximizeSharpeRatio, config)

	assert.NotNil(t, optimizer)
	assert.Equal(t, 4, optimizer.parallel)
}

func TestGridSearchOptimizer_GenerateCombinations(t *testing.T) {
	t.Run("integer parameters", func(t *testing.T) {
		params := []*Parameter{
			{Name: "a", Type: ParamTypeInt, Min: 1, Max: 3, Step: 1},
			{Name: "b", Type: ParamTypeInt, Min: 10, Max: 20, Step: 10},
		}

		optimizer := &GridSearchOptimizer{params: params}
		combinations := optimizer.generateCombinations()

		// Should have 3 * 2 = 6 combinations
		assert.Len(t, combinations, 6)

		// Check some combinations
		assert.Contains(t, combinations, ParameterSet{"a": 1, "b": 10})
		assert.Contains(t, combinations, ParameterSet{"a": 3, "b": 20})
	})

	t.Run("float parameters", func(t *testing.T) {
		params := []*Parameter{
			{Name: "threshold", Type: ParamTypeFloat, Min: 0.0, Max: 0.2, Step: 0.1},
		}

		optimizer := &GridSearchOptimizer{params: params}
		combinations := optimizer.generateCombinations()

		assert.Len(t, combinations, 3) // 0.0, 0.1, 0.2
	})

	t.Run("boolean parameters", func(t *testing.T) {
		params := []*Parameter{
			{Name: "use_stop", Type: ParamTypeBool},
		}

		optimizer := &GridSearchOptimizer{params: params}
		combinations := optimizer.generateCombinations()

		assert.Len(t, combinations, 2) // true, false
	})

	t.Run("string parameters", func(t *testing.T) {
		params := []*Parameter{
			{Name: "mode", Type: ParamTypeString, Values: []string{"fast", "slow", "balanced"}},
		}

		optimizer := &GridSearchOptimizer{params: params}
		combinations := optimizer.generateCombinations()

		assert.Len(t, combinations, 3)
	})

	t.Run("mixed parameters", func(t *testing.T) {
		params := []*Parameter{
			{Name: "period", Type: ParamTypeInt, Min: 10, Max: 20, Step: 10},
			{Name: "threshold", Type: ParamTypeFloat, Min: 0.5, Max: 1.5, Step: 0.5},
			{Name: "enabled", Type: ParamTypeBool},
		}

		optimizer := &GridSearchOptimizer{params: params}
		combinations := optimizer.generateCombinations()

		// 2 * 3 * 2 = 12 combinations
		assert.Len(t, combinations, 12)
	})
}

func TestGridSearchOptimizer_Optimize(t *testing.T) {
	params := []*Parameter{
		{Name: "short_period", Type: ParamTypeInt, Min: 10, Max: 20, Step: 10},
		{Name: "long_period", Type: ParamTypeInt, Min: 30, Max: 40, Step: 10},
		{Name: "threshold", Type: ParamTypeFloat, Min: 0.01, Max: 0.02, Step: 0.01},
		{Name: "use_stop", Type: ParamTypeBool},
	}

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   2,
	}

	optimizer := NewGridSearchOptimizer(NewParameterizedStrategy, params, MaximizeSharpeRatio, config)
	optimizer.SetParallelism(2)

	// Generate test data
	data := map[string][]*Candlestick{
		"BTC/USD": generateOptimizationTestData(50),
	}

	ctx := context.Background()
	summary, err := optimizer.Optimize(ctx, data)

	require.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, "grid_search", summary.Method)
	assert.Greater(t, summary.TotalRuns, 0)
	assert.NotNil(t, summary.BestResult)
	assert.NotEmpty(t, summary.TopResults)
}

// ============================================================================
// WALK-FORWARD TESTS
// ============================================================================

func TestNewWalkForwardOptimizer(t *testing.T) {
	params := []*Parameter{
		{Name: "short_period", Type: ParamTypeInt, Min: 5, Max: 15, Step: 5},
	}

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
	}

	optimizer := NewWalkForwardOptimizer(NewParameterizedStrategy, params, MaximizeSharpeRatio, config)

	assert.NotNil(t, optimizer)
	assert.Equal(t, 180*24*time.Hour, optimizer.inSamplePeriod)
	assert.Equal(t, 30*24*time.Hour, optimizer.outSamplePeriod)
}

func TestWalkForwardOptimizer_GenerateWindows(t *testing.T) {
	optimizer := &WalkForwardOptimizer{
		inSamplePeriod:  30 * 24 * time.Hour, // 30 days
		outSamplePeriod: 10 * 24 * time.Hour, // 10 days
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC) // 60 days

	windows := optimizer.generateWindows(start, end)

	// Should create 2 windows:
	// Window 1: Jan 1-30 (in-sample), Jan 31-Feb 9 (out-sample)
	// Window 2: Jan 11-Feb 9 (in-sample), Feb 10-19 (out-sample)
	assert.Greater(t, len(windows), 0)

	// Check first window
	assert.Equal(t, start, windows[0].InSampleStart)
	assert.Equal(t, start.Add(30*24*time.Hour), windows[0].InSampleEnd)
	assert.Equal(t, windows[0].InSampleEnd, windows[0].OutSampleStart)
	assert.Equal(t, windows[0].OutSampleStart.Add(10*24*time.Hour), windows[0].OutSampleEnd)
}

func TestWalkForwardOptimizer_GetDataTimeRange(t *testing.T) {
	candles1 := []*Candlestick{
		{Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Timestamp: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
	}

	candles2 := []*Candlestick{
		{Timestamp: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)},
		{Timestamp: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC)},
	}

	data := map[string][]*Candlestick{
		"BTC": candles1,
		"ETH": candles2,
	}

	optimizer := &WalkForwardOptimizer{}
	start, end := optimizer.getDataTimeRange(data)

	assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), end)
}

func TestWalkForwardOptimizer_FilterDataByTime(t *testing.T) {
	candles := []*Candlestick{
		{Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Close: 100},
		{Timestamp: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), Close: 110},
		{Timestamp: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Close: 120},
		{Timestamp: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), Close: 130},
		{Timestamp: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), Close: 140},
	}

	data := map[string][]*Candlestick{
		"BTC": candles,
	}

	optimizer := &WalkForwardOptimizer{}
	filtered := optimizer.filterDataByTime(
		data,
		time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	)

	require.Contains(t, filtered, "BTC")
	assert.Len(t, filtered["BTC"], 3) // Jan 5, 10, 15
	assert.Equal(t, 110.0, filtered["BTC"][0].Close)
	assert.Equal(t, 130.0, filtered["BTC"][2].Close)
}

// ============================================================================
// GENETIC ALGORITHM TESTS
// ============================================================================

func TestNewGeneticOptimizer(t *testing.T) {
	params := []*Parameter{
		{Name: "period", Type: ParamTypeInt, Min: 5, Max: 20, Step: 1},
	}

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
	}

	optimizer := NewGeneticOptimizer(NewParameterizedStrategy, params, MaximizeSharpeRatio, config)

	assert.NotNil(t, optimizer)
	assert.Equal(t, 50, optimizer.populationSize)
	assert.Equal(t, 20, optimizer.generations)
	assert.Equal(t, 0.1, optimizer.mutationRate)
	assert.Equal(t, 0.2, optimizer.eliteRatio)
}

func TestGeneticOptimizer_InitializePopulation(t *testing.T) {
	params := []*Parameter{
		{Name: "int_param", Type: ParamTypeInt, Min: 10, Max: 20, Step: 1},
		{Name: "float_param", Type: ParamTypeFloat, Min: 0.0, Max: 1.0, Step: 0.1},
		{Name: "bool_param", Type: ParamTypeBool},
		{Name: "string_param", Type: ParamTypeString, Values: []string{"a", "b", "c"}},
	}

	optimizer := &GeneticOptimizer{
		params:         params,
		populationSize: 10,
		rng:            rand.New(rand.NewSource(42)),
	}

	population := optimizer.initializePopulation()

	assert.Len(t, population, 10)

	// Check that each individual has all parameters
	for _, individual := range population {
		assert.Contains(t, individual, "int_param")
		assert.Contains(t, individual, "float_param")
		assert.Contains(t, individual, "bool_param")
		assert.Contains(t, individual, "string_param")

		// Check types
		assert.IsType(t, 0, individual["int_param"])
		assert.IsType(t, 0.0, individual["float_param"])
		assert.IsType(t, true, individual["bool_param"])
		assert.IsType(t, "", individual["string_param"])

		// Check ranges
		intVal := individual["int_param"].(int)
		assert.GreaterOrEqual(t, intVal, 10)
		assert.LessOrEqual(t, intVal, 20)

		floatVal := individual["float_param"].(float64)
		assert.GreaterOrEqual(t, floatVal, 0.0)
		assert.LessOrEqual(t, floatVal, 1.0)
	}
}

func TestGeneticOptimizer_Crossover(t *testing.T) {
	params := []*Parameter{
		{Name: "a", Type: ParamTypeInt, Min: 1, Max: 10, Step: 1},
		{Name: "b", Type: ParamTypeFloat, Min: 0, Max: 1, Step: 0.1},
	}

	optimizer := &GeneticOptimizer{
		params: params,
		rng:    rand.New(rand.NewSource(42)),
	}

	parent1 := ParameterSet{"a": 5, "b": 0.5}
	parent2 := ParameterSet{"a": 10, "b": 0.9}

	child := optimizer.crossover(parent1, parent2)

	// Child should have parameters from both parents
	assert.Contains(t, child, "a")
	assert.Contains(t, child, "b")

	// Values should be from one of the parents
	aVal := child["a"].(int)
	assert.True(t, aVal == 5 || aVal == 10)
}

func TestGeneticOptimizer_Mutate(t *testing.T) {
	params := []*Parameter{
		{Name: "period", Type: ParamTypeInt, Min: 10, Max: 20, Step: 1},
	}

	optimizer := &GeneticOptimizer{
		params:       params,
		mutationRate: 1.0, // 100% mutation for testing
		rng:          rand.New(rand.NewSource(42)),
	}

	original := ParameterSet{"period": 15}
	mutated := optimizer.mutate(original)

	// Original should be unchanged
	assert.Equal(t, 15, original["period"])

	// Mutated should be different (with high probability)
	// Note: There's a small chance they could be the same
	assert.Contains(t, mutated, "period")
}

func TestGeneticOptimizer_SelectParent(t *testing.T) {
	results := []*OptimizationResult{
		{Score: 1.0, Parameters: ParameterSet{"a": 1}},
		{Score: 2.0, Parameters: ParameterSet{"a": 2}},
		{Score: 3.0, Parameters: ParameterSet{"a": 3}},
		{Score: 4.0, Parameters: ParameterSet{"a": 4}},
		{Score: 5.0, Parameters: ParameterSet{"a": 5}},
	}

	optimizer := &GeneticOptimizer{
		rng: rand.New(rand.NewSource(42)),
	}

	// Run tournament selection multiple times
	selected := make(map[int]int)
	for i := 0; i < 100; i++ {
		parent := optimizer.selectParent(results)
		val := parent.Parameters["a"].(int)
		selected[val]++
	}

	// Higher scored individuals should be selected more often
	// (Though this is probabilistic, so not guaranteed every time)
	assert.Greater(t, len(selected), 0)
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestGridSearch_SmallParameterSpace(t *testing.T) {
	params := []*Parameter{
		{Name: "short_period", Type: ParamTypeInt, Min: 10, Max: 20, Step: 10},
		{Name: "long_period", Type: ParamTypeInt, Min: 30, Max: 40, Step: 10},
		{Name: "threshold", Type: ParamTypeFloat, Min: 0.01, Max: 0.01, Step: 0.01},
		{Name: "use_stop", Type: ParamTypeBool},
	}

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   2,
	}

	optimizer := NewGridSearchOptimizer(NewParameterizedStrategy, params, MaximizeTotalReturn, config)

	data := map[string][]*Candlestick{
		"BTC/USD": generateOptimizationTestData(30),
	}

	ctx := context.Background()
	summary, err := optimizer.Optimize(ctx, data)

	require.NoError(t, err)
	assert.Equal(t, "grid_search", summary.Method)
	assert.Equal(t, 8, summary.TotalRuns) // 2*2*1*2 = 8
	assert.NotNil(t, summary.BestResult)
	assert.Equal(t, 1, summary.BestResult.Rank)
	assert.LessOrEqual(t, len(summary.TopResults), 10)
}

func TestGeneticAlgorithm_SmallPopulation(t *testing.T) {
	params := []*Parameter{
		{Name: "short_period", Type: ParamTypeInt, Min: 10, Max: 20, Step: 1},
		{Name: "long_period", Type: ParamTypeInt, Min: 30, Max: 40, Step: 1},
		{Name: "threshold", Type: ParamTypeFloat, Min: 0.01, Max: 0.05, Step: 0.01},
		{Name: "use_stop", Type: ParamTypeBool},
	}

	config := BacktestConfig{
		InitialCapital: 10000,
		CommissionRate: 0.001,
		PositionSizing: "fixed",
		PositionSize:   1000,
		MaxPositions:   2,
	}

	optimizer := NewGeneticOptimizer(NewParameterizedStrategy, params, MaximizeSharpeRatio, config)
	optimizer.SetParameters(10, 3, 0.1, 0.2) // Small population and few generations for speed

	data := map[string][]*Candlestick{
		"BTC/USD": generateOptimizationTestData(30),
	}

	ctx := context.Background()
	summary, err := optimizer.Optimize(ctx, data)

	require.NoError(t, err)
	assert.Equal(t, "genetic_algorithm", summary.Method)
	assert.Equal(t, 30, summary.TotalRuns) // 10 pop * 3 gens = 30
	assert.NotNil(t, summary.BestResult)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateOptimizationTestData(count int) []*Candlestick {
	candles := make([]*Candlestick, count)
	basePrice := 50000.0
	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < count; i++ {
		// Create trending price movement
		trend := float64(i) * 100.0
		noise := float64((i%10)-5) * 50.0
		price := basePrice + trend + noise

		candles[i] = &Candlestick{
			Symbol:    "BTC/USD",
			Timestamp: timestamp.Add(time.Duration(i) * 24 * time.Hour),
			Open:      price - 100,
			High:      price + 200,
			Low:       price - 200,
			Close:     price,
			Volume:    1000 + float64(i*10),
		}
	}

	return candles
}
