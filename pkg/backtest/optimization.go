// Parameter optimization for backtesting strategies
package backtest

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// PARAMETER DEFINITION
// ============================================================================

// Parameter represents a tunable parameter for strategy optimization
type Parameter struct {
	Name   string    `json:"name"`
	Type   ParamType `json:"type"`   // int, float, bool, string
	Min    float64   `json:"min"`    // For numeric types
	Max    float64   `json:"max"`    // For numeric types
	Step   float64   `json:"step"`   // Step size for grid search
	Values []string  `json:"values"` // For string/categorical types
}

// ParamType defines the type of parameter
type ParamType string

const (
	ParamTypeInt    ParamType = "int"
	ParamTypeFloat  ParamType = "float"
	ParamTypeBool   ParamType = "bool"
	ParamTypeString ParamType = "string"
)

// ParameterSet represents a set of parameter values
type ParameterSet map[string]interface{}

// Clone creates a deep copy of the parameter set
func (ps ParameterSet) Clone() ParameterSet {
	clone := make(ParameterSet)
	for k, v := range ps {
		clone[k] = v
	}
	return clone
}

// ============================================================================
// OPTIMIZATION RESULT
// ============================================================================

// OptimizationResult represents the result of a parameter optimization
type OptimizationResult struct {
	Parameters    ParameterSet `json:"parameters"`
	Metrics       *Metrics     `json:"metrics"`
	Score         float64      `json:"score"`         // Fitness score
	Rank          int          `json:"rank"`          // Rank among all results
	IsOutOfSample bool         `json:"is_out_sample"` // Walk-forward out-of-sample flag
}

// OptimizationSummary summarizes an optimization run
type OptimizationSummary struct {
	Method          string                `json:"method"` // grid_search, walk_forward, genetic
	TotalRuns       int                   `json:"total_runs"`
	Duration        time.Duration         `json:"duration"`
	BestResult      *OptimizationResult   `json:"best_result"`
	TopResults      []*OptimizationResult `json:"top_results"` // Top 10 results
	ParameterRanges []*Parameter          `json:"parameter_ranges"`
	ObjectiveMetric string                `json:"objective_metric"` // What we're optimizing
	StartDate       time.Time             `json:"start_date"`
	EndDate         time.Time             `json:"end_date"`
}

// ============================================================================
// OBJECTIVE FUNCTIONS
// ============================================================================

// ObjectiveFunction calculates a fitness score from backtest metrics
type ObjectiveFunction func(*Metrics) float64

// Predefined objective functions
var (
	// MaximizeSharpeRatio optimizes for risk-adjusted returns
	MaximizeSharpeRatio ObjectiveFunction = func(m *Metrics) float64 {
		return m.SharpeRatio
	}

	// MaximizeSortinoRatio optimizes for downside risk-adjusted returns
	MaximizeSortinoRatio ObjectiveFunction = func(m *Metrics) float64 {
		return m.SortinoRatio
	}

	// MaximizeCalmarRatio optimizes for return/max drawdown
	MaximizeCalmarRatio ObjectiveFunction = func(m *Metrics) float64 {
		return m.CalmarRatio
	}

	// MaximizeTotalReturn optimizes for absolute returns
	MaximizeTotalReturn ObjectiveFunction = func(m *Metrics) float64 {
		return m.TotalReturnPct
	}

	// MaximizeProfitFactor optimizes for profit/loss ratio
	MaximizeProfitFactor ObjectiveFunction = func(m *Metrics) float64 {
		return m.ProfitFactor
	}

	// MinimizeDrawdown optimizes for low drawdown
	MinimizeDrawdown ObjectiveFunction = func(m *Metrics) float64 {
		return -m.MaxDrawdownPct // Negative because we minimize
	}

	// BalancedObjective combines multiple metrics
	BalancedObjective ObjectiveFunction = func(m *Metrics) float64 {
		// Weighted combination: 40% Sharpe, 30% Win Rate, 30% Calmar
		sharpe := math.Max(0, m.SharpeRatio)
		winRate := m.WinRate / 100.0
		calmar := math.Max(0, m.CalmarRatio)
		return 0.4*sharpe + 0.3*winRate + 0.3*calmar
	}
)

// ============================================================================
// STRATEGY FACTORY
// ============================================================================

// StrategyFactory creates a strategy with given parameters
type StrategyFactory func(params ParameterSet) (Strategy, error)

// ============================================================================
// GRID SEARCH OPTIMIZER
// ============================================================================

// GridSearchOptimizer performs exhaustive grid search over parameter space
type GridSearchOptimizer struct {
	factory   StrategyFactory
	params    []*Parameter
	objective ObjectiveFunction
	config    BacktestConfig
	parallel  int // Number of parallel workers
}

// NewGridSearchOptimizer creates a new grid search optimizer
func NewGridSearchOptimizer(factory StrategyFactory, params []*Parameter, objective ObjectiveFunction, config BacktestConfig) *GridSearchOptimizer {
	return &GridSearchOptimizer{
		factory:   factory,
		params:    params,
		objective: objective,
		config:    config,
		parallel:  4, // Default to 4 parallel workers
	}
}

// SetParallelism sets the number of parallel workers
func (opt *GridSearchOptimizer) SetParallelism(n int) {
	opt.parallel = n
}

// Optimize performs grid search optimization
func (opt *GridSearchOptimizer) Optimize(ctx context.Context, data map[string][]*Candlestick) (*OptimizationSummary, error) {
	startTime := time.Now()

	log.Info().
		Int("parameters", len(opt.params)).
		Int("parallel", opt.parallel).
		Msg("Starting grid search optimization")

	// Generate all parameter combinations
	combinations := opt.generateCombinations()
	totalRuns := len(combinations)

	log.Info().
		Int("combinations", totalRuns).
		Msg("Generated parameter combinations")

	// Run backtests in parallel
	results := make([]*OptimizationResult, 0, totalRuns)
	resultsChan := make(chan *OptimizationResult, totalRuns)
	semaphore := make(chan struct{}, opt.parallel)

	var wg sync.WaitGroup

	for i, paramSet := range combinations {
		wg.Add(1)
		go func(idx int, ps ParameterSet) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := opt.runBacktest(ctx, ps, data)
			if result != nil {
				resultsChan <- result
			}

			// Log progress
			if (idx+1)%10 == 0 || idx == totalRuns-1 {
				log.Info().
					Int("completed", idx+1).
					Int("total", totalRuns).
					Msgf("Grid search progress: %.1f%%", float64(idx+1)/float64(totalRuns)*100)
			}
		}(i, paramSet)
	}

	// Wait for all backtests to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}

	// Sort results by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Assign ranks
	for i, result := range results {
		result.Rank = i + 1
	}

	// Create summary
	summary := &OptimizationSummary{
		Method:          "grid_search",
		TotalRuns:       totalRuns,
		Duration:        time.Since(startTime),
		ParameterRanges: opt.params,
		ObjectiveMetric: "custom", // Could be enhanced to track which objective
		BestResult:      results[0],
	}

	// Top 10 results
	topN := 10
	if len(results) < topN {
		topN = len(results)
	}
	summary.TopResults = results[:topN]

	log.Info().
		Int("total_runs", totalRuns).
		Float64("best_score", summary.BestResult.Score).
		Dur("duration", summary.Duration).
		Msg("Grid search optimization complete")

	return summary, nil
}

// generateCombinations generates all parameter combinations
func (opt *GridSearchOptimizer) generateCombinations() []ParameterSet {
	if len(opt.params) == 0 {
		return []ParameterSet{{}}
	}

	// Recursive combination generation
	return opt.generateCombinationsRecursive(0, ParameterSet{})
}

func (opt *GridSearchOptimizer) generateCombinationsRecursive(paramIdx int, current ParameterSet) []ParameterSet {
	if paramIdx >= len(opt.params) {
		return []ParameterSet{current.Clone()}
	}

	param := opt.params[paramIdx]
	var combinations []ParameterSet

	switch param.Type {
	case ParamTypeInt:
		for v := param.Min; v <= param.Max; v += param.Step {
			newSet := current.Clone()
			newSet[param.Name] = int(v)
			combinations = append(combinations, opt.generateCombinationsRecursive(paramIdx+1, newSet)...)
		}

	case ParamTypeFloat:
		for v := param.Min; v <= param.Max; v += param.Step {
			newSet := current.Clone()
			newSet[param.Name] = v
			combinations = append(combinations, opt.generateCombinationsRecursive(paramIdx+1, newSet)...)
		}

	case ParamTypeBool:
		for _, v := range []bool{false, true} {
			newSet := current.Clone()
			newSet[param.Name] = v
			combinations = append(combinations, opt.generateCombinationsRecursive(paramIdx+1, newSet)...)
		}

	case ParamTypeString:
		for _, v := range param.Values {
			newSet := current.Clone()
			newSet[param.Name] = v
			combinations = append(combinations, opt.generateCombinationsRecursive(paramIdx+1, newSet)...)
		}
	}

	return combinations
}

// runBacktest runs a single backtest with given parameters
func (opt *GridSearchOptimizer) runBacktest(ctx context.Context, params ParameterSet, data map[string][]*Candlestick) *OptimizationResult {
	// Create strategy with parameters
	strategy, err := opt.factory(params)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create strategy")
		return nil
	}

	// Create engine
	engine := NewEngine(opt.config)

	// Load data
	for symbol, candles := range data {
		_ = engine.LoadHistoricalData(symbol, candles) // Optimization run - error logged elsewhere
	}

	// Run backtest
	if err := engine.Run(ctx, strategy); err != nil {
		log.Warn().Err(err).Msg("Backtest failed")
		return nil
	}

	// Calculate metrics
	metrics, err := CalculateMetrics(engine)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate metrics")
		return nil
	}

	// Calculate score
	score := opt.objective(metrics)

	return &OptimizationResult{
		Parameters: params,
		Metrics:    metrics,
		Score:      score,
	}
}

// ============================================================================
// WALK-FORWARD OPTIMIZER
// ============================================================================

// WalkForwardOptimizer performs walk-forward analysis
type WalkForwardOptimizer struct {
	factory         StrategyFactory
	params          []*Parameter
	objective       ObjectiveFunction
	config          BacktestConfig
	inSamplePeriod  time.Duration // e.g., 180 days
	outSamplePeriod time.Duration // e.g., 30 days
	parallel        int
}

// NewWalkForwardOptimizer creates a new walk-forward optimizer
func NewWalkForwardOptimizer(factory StrategyFactory, params []*Parameter, objective ObjectiveFunction, config BacktestConfig) *WalkForwardOptimizer {
	return &WalkForwardOptimizer{
		factory:         factory,
		params:          params,
		objective:       objective,
		config:          config,
		inSamplePeriod:  180 * 24 * time.Hour, // 6 months in-sample
		outSamplePeriod: 30 * 24 * time.Hour,  // 1 month out-of-sample
		parallel:        4,
	}
}

// SetPeriods sets the in-sample and out-of-sample periods
func (opt *WalkForwardOptimizer) SetPeriods(inSample, outSample time.Duration) {
	opt.inSamplePeriod = inSample
	opt.outSamplePeriod = outSample
}

// Optimize performs walk-forward optimization
func (opt *WalkForwardOptimizer) Optimize(ctx context.Context, data map[string][]*Candlestick) (*OptimizationSummary, error) {
	startTime := time.Now()

	log.Info().
		Dur("in_sample", opt.inSamplePeriod).
		Dur("out_sample", opt.outSamplePeriod).
		Msg("Starting walk-forward optimization")

	// Get time range from data
	startDate, endDate := opt.getDataTimeRange(data)
	log.Info().
		Time("start", startDate).
		Time("end", endDate).
		Msg("Data time range")

	// Generate walk-forward windows
	windows := opt.generateWindows(startDate, endDate)
	log.Info().
		Int("windows", len(windows)).
		Msg("Generated walk-forward windows")

	var allResults []*OptimizationResult
	var bestParams ParameterSet

	// For each window: optimize on in-sample, test on out-of-sample
	for i, window := range windows {
		log.Info().
			Int("window", i+1).
			Int("total", len(windows)).
			Time("train_start", window.InSampleStart).
			Time("train_end", window.InSampleEnd).
			Time("test_start", window.OutSampleStart).
			Time("test_end", window.OutSampleEnd).
			Msg("Processing walk-forward window")

		// Split data into in-sample and out-of-sample
		inSampleData := opt.filterDataByTime(data, window.InSampleStart, window.InSampleEnd)
		outSampleData := opt.filterDataByTime(data, window.OutSampleStart, window.OutSampleEnd)

		// Optimize on in-sample data (grid search)
		gridOpt := NewGridSearchOptimizer(opt.factory, opt.params, opt.objective, opt.config)
		gridOpt.SetParallelism(opt.parallel)

		summary, err := gridOpt.Optimize(ctx, inSampleData)
		if err != nil {
			log.Warn().Err(err).Int("window", i+1).Msg("In-sample optimization failed")
			continue
		}

		bestParams = summary.BestResult.Parameters

		// Test on out-of-sample data
		outResult := opt.runBacktest(ctx, bestParams, outSampleData)
		if outResult != nil {
			outResult.IsOutOfSample = true
			allResults = append(allResults, outResult)

			log.Info().
				Int("window", i+1).
				Float64("in_sample_score", summary.BestResult.Score).
				Float64("out_sample_score", outResult.Score).
				Msg("Walk-forward window complete")
		}
	}

	// Sort by out-of-sample score
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	for i, result := range allResults {
		result.Rank = i + 1
	}

	summary := &OptimizationSummary{
		Method:          "walk_forward",
		TotalRuns:       len(allResults),
		Duration:        time.Since(startTime),
		ParameterRanges: opt.params,
		BestResult:      allResults[0],
		StartDate:       startDate,
		EndDate:         endDate,
	}

	topN := 10
	if len(allResults) < topN {
		topN = len(allResults)
	}
	summary.TopResults = allResults[:topN]

	log.Info().
		Int("windows", len(windows)).
		Float64("best_score", summary.BestResult.Score).
		Dur("duration", summary.Duration).
		Msg("Walk-forward optimization complete")

	return summary, nil
}

// WalkForwardWindow represents a training/testing window
type WalkForwardWindow struct {
	InSampleStart  time.Time
	InSampleEnd    time.Time
	OutSampleStart time.Time
	OutSampleEnd   time.Time
}

// generateWindows creates overlapping walk-forward windows
func (opt *WalkForwardOptimizer) generateWindows(start, end time.Time) []WalkForwardWindow {
	var windows []WalkForwardWindow

	currentStart := start
	for {
		inSampleEnd := currentStart.Add(opt.inSamplePeriod)
		outSampleStart := inSampleEnd
		outSampleEnd := outSampleStart.Add(opt.outSamplePeriod)

		if outSampleEnd.After(end) {
			break
		}

		windows = append(windows, WalkForwardWindow{
			InSampleStart:  currentStart,
			InSampleEnd:    inSampleEnd,
			OutSampleStart: outSampleStart,
			OutSampleEnd:   outSampleEnd,
		})

		// Move window forward by out-of-sample period (anchored walk-forward)
		currentStart = currentStart.Add(opt.outSamplePeriod)
	}

	return windows
}

// getDataTimeRange extracts start and end times from data
func (opt *WalkForwardOptimizer) getDataTimeRange(data map[string][]*Candlestick) (time.Time, time.Time) {
	var start, end time.Time

	for _, candles := range data {
		if len(candles) == 0 {
			continue
		}

		if start.IsZero() || candles[0].Timestamp.Before(start) {
			start = candles[0].Timestamp
		}

		if end.IsZero() || candles[len(candles)-1].Timestamp.After(end) {
			end = candles[len(candles)-1].Timestamp
		}
	}

	return start, end
}

// filterDataByTime filters candlesticks by time range
func (opt *WalkForwardOptimizer) filterDataByTime(data map[string][]*Candlestick, start, end time.Time) map[string][]*Candlestick {
	filtered := make(map[string][]*Candlestick)

	for symbol, candles := range data {
		var filteredCandles []*Candlestick
		for _, candle := range candles {
			if !candle.Timestamp.Before(start) && !candle.Timestamp.After(end) {
				filteredCandles = append(filteredCandles, candle)
			}
		}
		if len(filteredCandles) > 0 {
			filtered[symbol] = filteredCandles
		}
	}

	return filtered
}

// runBacktest runs a single backtest (same as GridSearchOptimizer)
func (opt *WalkForwardOptimizer) runBacktest(ctx context.Context, params ParameterSet, data map[string][]*Candlestick) *OptimizationResult {
	strategy, err := opt.factory(params)
	if err != nil {
		return nil
	}

	engine := NewEngine(opt.config)
	for symbol, candles := range data {
		_ = engine.LoadHistoricalData(symbol, candles) // Optimization run - error logged elsewhere
	}

	if err := engine.Run(ctx, strategy); err != nil {
		return nil
	}

	metrics, err := CalculateMetrics(engine)
	if err != nil {
		return nil
	}

	return &OptimizationResult{
		Parameters: params,
		Metrics:    metrics,
		Score:      opt.objective(metrics),
	}
}

// ============================================================================
// GENETIC ALGORITHM OPTIMIZER
// ============================================================================

// GeneticOptimizer performs genetic algorithm optimization
type GeneticOptimizer struct {
	factory        StrategyFactory
	params         []*Parameter
	objective      ObjectiveFunction
	config         BacktestConfig
	populationSize int
	generations    int
	mutationRate   float64
	eliteRatio     float64 // Percentage of elite individuals to keep
	parallel       int
	rng            *rand.Rand
	seed           int64 // Random seed for reproducibility (0 = use time-based seed)
}

// NewGeneticOptimizer creates a new genetic algorithm optimizer
// Random seed is initialized with current time for non-deterministic behavior
// Use SetSeed() to set a specific seed for reproducible results
func NewGeneticOptimizer(factory StrategyFactory, params []*Parameter, objective ObjectiveFunction, config BacktestConfig) *GeneticOptimizer {
	seed := time.Now().UnixNano()
	return &GeneticOptimizer{
		factory:        factory,
		params:         params,
		objective:      objective,
		config:         config,
		populationSize: 50,
		generations:    20,
		mutationRate:   0.1,
		eliteRatio:     0.2, // Keep top 20%
		parallel:       4,
		rng:            rand.New(rand.NewSource(seed)), // #nosec G404 -- Non-cryptographic use: genetic algorithm needs reproducible randomness for backtesting
		seed:           seed,
	}
}

// SetParameters configures genetic algorithm parameters
func (opt *GeneticOptimizer) SetParameters(popSize, gens int, mutRate, eliteRatio float64) {
	opt.populationSize = popSize
	opt.generations = gens
	opt.mutationRate = mutRate
	opt.eliteRatio = eliteRatio
}

// SetSeed sets a specific random seed for reproducible results
// This is useful for testing and debugging. If not called, a time-based seed is used.
func (opt *GeneticOptimizer) SetSeed(seed int64) {
	opt.seed = seed
	opt.rng = rand.New(rand.NewSource(seed)) // #nosec G404 -- Non-cryptographic use: genetic algorithm needs reproducible randomness for backtesting
}

// Optimize performs genetic algorithm optimization
func (opt *GeneticOptimizer) Optimize(ctx context.Context, data map[string][]*Candlestick) (*OptimizationSummary, error) {
	startTime := time.Now()

	log.Info().
		Int("population", opt.populationSize).
		Int("generations", opt.generations).
		Float64("mutation_rate", opt.mutationRate).
		Msg("Starting genetic algorithm optimization")

	// Initialize population
	population := opt.initializePopulation()

	var allResults []*OptimizationResult
	var bestResult *OptimizationResult

	// Evolution loop
	for gen := 0; gen < opt.generations; gen++ {
		log.Info().
			Int("generation", gen+1).
			Int("total", opt.generations).
			Msg("Evolving generation")

		// Evaluate fitness
		evaluated := opt.evaluatePopulation(ctx, population, data)
		allResults = append(allResults, evaluated...)

		// Sort by fitness
		sort.Slice(evaluated, func(i, j int) bool {
			return evaluated[i].Score > evaluated[j].Score
		})

		// Track best
		if bestResult == nil || evaluated[0].Score > bestResult.Score {
			bestResult = evaluated[0]
		}

		log.Info().
			Int("generation", gen+1).
			Float64("best_score", evaluated[0].Score).
			Float64("worst_score", evaluated[len(evaluated)-1].Score).
			Float64("avg_score", opt.averageScore(evaluated)).
			Msg("Generation complete")

		// Early stopping if last generation
		if gen == opt.generations-1 {
			break
		}

		// Selection and reproduction
		eliteCount := int(float64(opt.populationSize) * opt.eliteRatio)
		elite := evaluated[:eliteCount]

		// Create next generation
		nextGen := make([]ParameterSet, 0, opt.populationSize)

		// Keep elite
		for _, result := range elite {
			nextGen = append(nextGen, result.Parameters.Clone())
		}

		// Crossover and mutation
		for len(nextGen) < opt.populationSize {
			parent1 := opt.selectParent(evaluated)
			parent2 := opt.selectParent(evaluated)

			child := opt.crossover(parent1.Parameters, parent2.Parameters)
			child = opt.mutate(child)

			nextGen = append(nextGen, child)
		}

		population = nextGen
	}

	// Sort all results
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	for i, result := range allResults {
		result.Rank = i + 1
	}

	summary := &OptimizationSummary{
		Method:          "genetic_algorithm",
		TotalRuns:       len(allResults),
		Duration:        time.Since(startTime),
		ParameterRanges: opt.params,
		BestResult:      bestResult,
	}

	topN := 10
	if len(allResults) < topN {
		topN = len(allResults)
	}
	summary.TopResults = allResults[:topN]

	log.Info().
		Int("total_evaluations", len(allResults)).
		Float64("best_score", bestResult.Score).
		Dur("duration", summary.Duration).
		Msg("Genetic algorithm optimization complete")

	return summary, nil
}

// initializePopulation creates random initial population
func (opt *GeneticOptimizer) initializePopulation() []ParameterSet {
	population := make([]ParameterSet, opt.populationSize)

	for i := 0; i < opt.populationSize; i++ {
		individual := make(ParameterSet)

		for _, param := range opt.params {
			switch param.Type {
			case ParamTypeInt:
				min := int(param.Min)
				max := int(param.Max)
				individual[param.Name] = min + opt.rng.Intn(max-min+1)

			case ParamTypeFloat:
				individual[param.Name] = param.Min + opt.rng.Float64()*(param.Max-param.Min)

			case ParamTypeBool:
				individual[param.Name] = opt.rng.Float64() < 0.5

			case ParamTypeString:
				individual[param.Name] = param.Values[opt.rng.Intn(len(param.Values))]
			}
		}

		population[i] = individual
	}

	return population
}

// evaluatePopulation evaluates fitness of all individuals
func (opt *GeneticOptimizer) evaluatePopulation(ctx context.Context, population []ParameterSet, data map[string][]*Candlestick) []*OptimizationResult {
	results := make([]*OptimizationResult, len(population))
	resultsChan := make(chan struct {
		idx    int
		result *OptimizationResult
	}, len(population))

	semaphore := make(chan struct{}, opt.parallel)
	var wg sync.WaitGroup

	for i, params := range population {
		wg.Add(1)
		go func(idx int, ps ParameterSet) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := opt.runBacktest(ctx, ps, data)
			resultsChan <- struct {
				idx    int
				result *OptimizationResult
			}{idx, result}
		}(i, params)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for res := range resultsChan {
		results[res.idx] = res.result
	}

	return results
}

// selectParent selects a parent using tournament selection
func (opt *GeneticOptimizer) selectParent(population []*OptimizationResult) *OptimizationResult {
	tournamentSize := 3
	best := population[opt.rng.Intn(len(population))]

	for i := 1; i < tournamentSize; i++ {
		contestant := population[opt.rng.Intn(len(population))]
		if contestant.Score > best.Score {
			best = contestant
		}
	}

	return best
}

// crossover performs uniform crossover
func (opt *GeneticOptimizer) crossover(parent1, parent2 ParameterSet) ParameterSet {
	child := make(ParameterSet)

	for _, param := range opt.params {
		if opt.rng.Float64() < 0.5 {
			child[param.Name] = parent1[param.Name]
		} else {
			child[param.Name] = parent2[param.Name]
		}
	}

	return child
}

// mutate performs mutation on an individual
func (opt *GeneticOptimizer) mutate(individual ParameterSet) ParameterSet {
	mutated := individual.Clone()

	for _, param := range opt.params {
		if opt.rng.Float64() < opt.mutationRate {
			switch param.Type {
			case ParamTypeInt:
				min := int(param.Min)
				max := int(param.Max)
				mutated[param.Name] = min + opt.rng.Intn(max-min+1)

			case ParamTypeFloat:
				mutated[param.Name] = param.Min + opt.rng.Float64()*(param.Max-param.Min)

			case ParamTypeBool:
				mutated[param.Name] = opt.rng.Float64() < 0.5

			case ParamTypeString:
				mutated[param.Name] = param.Values[opt.rng.Intn(len(param.Values))]
			}
		}
	}

	return mutated
}

// runBacktest runs a backtest (same as other optimizers)
func (opt *GeneticOptimizer) runBacktest(ctx context.Context, params ParameterSet, data map[string][]*Candlestick) *OptimizationResult {
	strategy, err := opt.factory(params)
	if err != nil {
		return &OptimizationResult{Parameters: params, Score: -math.Inf(1)}
	}

	engine := NewEngine(opt.config)
	for symbol, candles := range data {
		_ = engine.LoadHistoricalData(symbol, candles) // Optimization run - error logged elsewhere
	}

	if err := engine.Run(ctx, strategy); err != nil {
		return &OptimizationResult{Parameters: params, Score: -math.Inf(1)}
	}

	metrics, err := CalculateMetrics(engine)
	if err != nil {
		return &OptimizationResult{Parameters: params, Score: -math.Inf(1)}
	}

	return &OptimizationResult{
		Parameters: params,
		Metrics:    metrics,
		Score:      opt.objective(metrics),
	}
}

// averageScore calculates average fitness score
func (opt *GeneticOptimizer) averageScore(results []*OptimizationResult) float64 {
	if len(results) == 0 {
		return 0
	}

	sum := 0.0
	for _, r := range results {
		sum += r.Score
	}

	return sum / float64(len(results))
}
