package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestCloningCoordinator creates a test cloning coordinator
func setupTestCloningCoordinator(t *testing.T) (*CloningCoordinator, *HotSwapCoordinator, *Blackboard, *MessageBus, func()) {
	// Setup Redis
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	bb := &Blackboard{
		client: redisClient,
		prefix: "test:blackboard:",
	}

	// Setup NATS
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)

	mb := &MessageBus{
		nc:     nc,
		prefix: "test:agents.",
	}

	hsc := NewHotSwapCoordinator(bb, mb)
	cc := NewCloningCoordinator(hsc, bb, mb)

	cleanup := func() {
		nc.Close()
		ns.Shutdown()
		redisClient.Close()
		mr.Close()
	}

	return cc, hsc, bb, mb, cleanup
}

// TestNewCloningCoordinator tests coordinator creation
func TestNewCloningCoordinator(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	assert.NotNil(t, cc)
	assert.NotNil(t, cc.hotSwap)
	assert.NotNil(t, cc.blackboard)
	assert.NotNil(t, cc.messageBus)
	assert.NotNil(t, cc.experiments)
}

// TestCloneAgent tests agent cloning
func TestCloneAgent(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	// Register source agent
	sourceAgent := &AgentRegistration{
		Name:         "source-agent",
		Type:         "technical",
		Version:      "1.0.0",
		Capabilities: []string{"analyze", "predict"},
		State: &AgentState{
			Memory: map[string]interface{}{
				"last_price": 50000.0,
			},
			Configuration: map[string]interface{}{
				"threshold": 0.8,
			},
			PendingTasks: []*AgentTask{},
			PerformanceMetrics: &PerformanceMetrics{
				TotalTasks: 100,
			},
		},
	}

	err := hsc.RegisterAgent(ctx, sourceAgent)
	require.NoError(t, err)

	// Clone agent
	cloneConfig := &CloneConfig{
		SourceAgent:  "source-agent",
		CloneName:    "cloned-agent",
		InheritState: true,
		ConfigOverrides: map[string]interface{}{
			"threshold": 0.9, // Override configuration
		},
		Metadata: map[string]interface{}{
			"purpose": "testing",
		},
	}

	clone, err := cc.CloneAgent(ctx, cloneConfig)
	require.NoError(t, err)
	assert.NotNil(t, clone)

	// Verify clone
	assert.Equal(t, "cloned-agent", clone.Name)
	assert.Equal(t, "technical", clone.Type)
	assert.Equal(t, "1.0.0-clone", clone.Version)
	assert.Equal(t, sourceAgent.Capabilities, clone.Capabilities)

	// Verify state inheritance
	assert.NotNil(t, clone.State)
	assert.Equal(t, 50000.0, clone.State.Memory["last_price"])

	// Verify configuration override
	assert.Equal(t, 0.9, clone.State.Configuration["threshold"])

	// Verify metadata
	assert.Equal(t, "testing", clone.Metadata["purpose"])
}

// TestCloneAgentWithoutStateInheritance tests cloning without state
func TestCloneAgentWithoutStateInheritance(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	sourceAgent := &AgentRegistration{
		Name: "source-agent",
		Type: "technical",
		State: &AgentState{
			Memory: map[string]interface{}{
				"data": "test",
			},
		},
	}

	hsc.RegisterAgent(ctx, sourceAgent)

	cloneConfig := &CloneConfig{
		SourceAgent:  "source-agent",
		CloneName:    "clone-no-state",
		InheritState: false,
	}

	clone, err := cc.CloneAgent(ctx, cloneConfig)
	require.NoError(t, err)

	// State should be initialized but empty
	assert.NotNil(t, clone.State)
	assert.Empty(t, clone.State.Memory)
}

// TestCloneAgentSourceNotFound tests error handling
func TestCloneAgentSourceNotFound(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	cloneConfig := &CloneConfig{
		SourceAgent: "nonexistent",
		CloneName:   "clone",
	}

	_, err := cc.CloneAgent(ctx, cloneConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source agent not found")
}

// TestStartABTest tests A/B test experiment creation
func TestStartABTest(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	// Register control agent
	controlAgent := &AgentRegistration{
		Name: "control-agent",
		Type: "technical",
		State: &AgentState{
			Memory:             make(map[string]interface{}),
			Configuration:      make(map[string]interface{}),
			PendingTasks:       []*AgentTask{},
			PerformanceMetrics: &PerformanceMetrics{},
		},
	}

	hsc.RegisterAgent(ctx, controlAgent)

	// Start A/B test
	config := &ExperimentConfig{
		Duration:          1 * time.Minute,
		MinSamples:        100,
		SignificanceLevel: 0.95,
		ComparisonMetric:  "latency",
		AutoSelectWinner:  false,
	}

	experiment, err := cc.StartABTest(ctx, "Latency Test", "control-agent", 2, config)
	require.NoError(t, err)
	assert.NotNil(t, experiment)

	// Verify experiment
	assert.Equal(t, "Latency Test", experiment.Name)
	assert.Equal(t, "control-agent", experiment.ControlAgent)
	assert.Len(t, experiment.VariantAgents, 2)
	assert.Equal(t, ExperimentStatusRunning, experiment.Status)

	// Verify variants created
	variant1, err := hsc.GetAgent("control-agent-variant-1")
	require.NoError(t, err)
	assert.NotNil(t, variant1)

	variant2, err := hsc.GetAgent("control-agent-variant-2")
	require.NoError(t, err)
	assert.NotNil(t, variant2)

	// Verify metrics initialized
	assert.Len(t, experiment.Metrics, 3) // control + 2 variants
	assert.NotNil(t, experiment.Metrics["control-agent"])
	assert.NotNil(t, experiment.Metrics["control-agent-variant-1"])
	assert.NotNil(t, experiment.Metrics["control-agent-variant-2"])

	// Verify traffic split
	assert.NotNil(t, config.TrafficSplit)
	assert.InDelta(t, 0.333, config.TrafficSplit["control-agent"], 0.01)
}

// TestStartABTestControlNotFound tests error handling
func TestStartABTestControlNotFound(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	config := &ExperimentConfig{
		Duration:   1 * time.Minute,
		MinSamples: 100,
	}

	_, err := cc.StartABTest(ctx, "Test", "nonexistent", 2, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control agent not found")
}

// TestRecordMetric tests metric recording
func TestRecordMetric(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	// Setup experiment
	controlAgent := &AgentRegistration{
		Name:  "control",
		Type:  "technical",
		State: &AgentState{Memory: make(map[string]interface{}), Configuration: make(map[string]interface{}), PendingTasks: []*AgentTask{}, PerformanceMetrics: &PerformanceMetrics{}},
	}
	hsc.RegisterAgent(ctx, controlAgent)

	config := &ExperimentConfig{
		Duration:   1 * time.Minute,
		MinSamples: 10,
	}

	experiment, err := cc.StartABTest(ctx, "Test", "control", 1, config)
	require.NoError(t, err)

	// Record metrics
	for i := 0; i < 10; i++ {
		latency := time.Duration(100+i) * time.Millisecond
		err := cc.RecordMetric(ctx, experiment.ID, "control", latency, true, nil)
		require.NoError(t, err)
	}

	// Verify metrics updated
	metrics := experiment.Metrics["control"]
	assert.Equal(t, int64(10), metrics.TotalRequests)
	assert.Equal(t, int64(10), metrics.SuccessfulRequests)
	assert.Equal(t, int64(0), metrics.FailedRequests)
	assert.Greater(t, metrics.AverageLatency, time.Duration(0))
}

// TestRecordMetricExperimentNotFound tests error handling
func TestRecordMetricExperimentNotFound(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	err := cc.RecordMetric(ctx, uuid.New(), "agent", 100*time.Millisecond, true, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment not found")
}

// TestUpdateAggregateMetrics tests aggregate metric calculation
func TestUpdateAggregateMetrics(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	metrics := &VariantMetrics{
		AgentName:     "test",
		Samples:       []*MetricSample{},
		CustomMetrics: make(map[string]float64),
	}

	// Add samples
	samples := []struct {
		latency time.Duration
		success bool
	}{
		{100 * time.Millisecond, true},
		{120 * time.Millisecond, true},
		{110 * time.Millisecond, true},
		{150 * time.Millisecond, false},
		{105 * time.Millisecond, true},
	}

	for _, s := range samples {
		sample := &MetricSample{
			Timestamp: time.Now(),
			Latency:   s.latency,
			Success:   s.success,
		}
		metrics.Samples = append(metrics.Samples, sample)
		metrics.TotalRequests++
		if s.success {
			metrics.SuccessfulRequests++
		} else {
			metrics.FailedRequests++
		}
	}

	// Update aggregates
	cc.updateAggregateMetrics(metrics)

	// Verify calculations
	assert.Equal(t, int64(5), metrics.TotalRequests)
	assert.Equal(t, int64(4), metrics.SuccessfulRequests)
	assert.Equal(t, int64(1), metrics.FailedRequests)
	assert.InDelta(t, 0.2, metrics.ErrorRate, 0.01)
	assert.Greater(t, metrics.AverageLatency, time.Duration(0))
}

// TestCompareVariants tests variant comparison logic
func TestCompareVariants(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	controlMetrics := &VariantMetrics{
		AgentName:      "control",
		AverageLatency: 200 * time.Millisecond,
		ErrorRate:      0.1,   // 10% error rate
		Throughput:     100.0, // 100 req/s
	}

	variantMetrics := &VariantMetrics{
		AgentName:      "variant",
		AverageLatency: 150 * time.Millisecond, // 25% improvement
		ErrorRate:      0.05,                   // 5% error rate (50% reduction)
		Throughput:     120.0,                  // 20% improvement
	}

	comparison := cc.compareVariants(controlMetrics, variantMetrics)

	assert.Equal(t, "variant", comparison.VariantName)
	assert.InDelta(t, 25.0, comparison.LatencyImprovement, 1.0)
	assert.InDelta(t, -5.0, comparison.ErrorRateChange, 1.0)
	assert.InDelta(t, 20.0, comparison.ThroughputChange, 1.0)
	assert.Greater(t, comparison.OverallScore, 50.0)
	assert.True(t, comparison.BetterThanControl)
}

// TestAnalyzeResults tests result analysis
func TestAnalyzeResults(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	experiment := &ABTestExperiment{
		ID:            uuid.New(),
		ControlAgent:  "control",
		VariantAgents: []string{"variant-1", "variant-2"},
		Configuration: &ExperimentConfig{
			SignificanceLevel: 0.95,
		},
		Metrics: map[string]*VariantMetrics{
			"control": {
				AgentName:      "control",
				AverageLatency: 200 * time.Millisecond,
				ErrorRate:      0.1,
				Throughput:     100.0,
			},
			"variant-1": {
				AgentName:      "variant-1",
				AverageLatency: 150 * time.Millisecond, // Better
				ErrorRate:      0.05,
				Throughput:     120.0,
			},
			"variant-2": {
				AgentName:      "variant-2",
				AverageLatency: 250 * time.Millisecond, // Worse
				ErrorRate:      0.15,
				Throughput:     90.0,
			},
		},
	}

	results := cc.analyzeResults(experiment)

	assert.NotNil(t, results)
	assert.Equal(t, "variant-1", results.Winner)
	assert.NotNil(t, results.WinnerMetrics)
	assert.Equal(t, "variant-1", results.WinnerMetrics.AgentName)
	assert.Len(t, results.Comparison, 2)
	assert.NotEmpty(t, results.Recommendation)
	assert.NotEmpty(t, results.Summary)
}

// TestGetExperiment tests experiment retrieval
func TestGetExperiment(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	controlAgent := &AgentRegistration{
		Name:  "control",
		Type:  "technical",
		State: &AgentState{Memory: make(map[string]interface{}), Configuration: make(map[string]interface{}), PendingTasks: []*AgentTask{}, PerformanceMetrics: &PerformanceMetrics{}},
	}
	hsc.RegisterAgent(ctx, controlAgent)

	config := &ExperimentConfig{
		Duration:   1 * time.Minute,
		MinSamples: 10,
	}

	experiment, err := cc.StartABTest(ctx, "Test", "control", 1, config)
	require.NoError(t, err)

	retrieved, err := cc.GetExperiment(experiment.ID)
	require.NoError(t, err)
	assert.Equal(t, experiment.ID, retrieved.ID)
	assert.Equal(t, experiment.Name, retrieved.Name)
}

// TestGetExperimentNotFound tests error handling
func TestGetExperimentNotFound(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	_, err := cc.GetExperiment(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment not found")
}

// TestListExperiments tests listing experiments
func TestListExperiments(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	controlAgent := &AgentRegistration{
		Name:  "control",
		Type:  "technical",
		State: &AgentState{Memory: make(map[string]interface{}), Configuration: make(map[string]interface{}), PendingTasks: []*AgentTask{}, PerformanceMetrics: &PerformanceMetrics{}},
	}
	hsc.RegisterAgent(ctx, controlAgent)

	config := &ExperimentConfig{
		Duration:   1 * time.Minute,
		MinSamples: 10,
	}

	cc.StartABTest(ctx, "Test1", "control", 1, config)
	cc.StartABTest(ctx, "Test2", "control", 1, config)

	experiments := cc.ListExperiments()
	assert.GreaterOrEqual(t, len(experiments), 2)
}

// TestCancelExperiment tests experiment cancellation
func TestCancelExperiment(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	controlAgent := &AgentRegistration{
		Name:  "control",
		Type:  "technical",
		State: &AgentState{Memory: make(map[string]interface{}), Configuration: make(map[string]interface{}), PendingTasks: []*AgentTask{}, PerformanceMetrics: &PerformanceMetrics{}},
	}
	hsc.RegisterAgent(ctx, controlAgent)

	config := &ExperimentConfig{
		Duration:   1 * time.Minute,
		MinSamples: 10,
	}

	experiment, err := cc.StartABTest(ctx, "Test", "control", 2, config)
	require.NoError(t, err)

	// Cancel experiment
	err = cc.CancelExperiment(ctx, experiment.ID)
	require.NoError(t, err)

	// Verify status
	cancelled, _ := cc.GetExperiment(experiment.ID)
	assert.Equal(t, ExperimentStatusCancelled, cancelled.Status)
	assert.NotNil(t, cancelled.CompletedAt)

	// Verify variants cleaned up
	_, err = hsc.GetAgent("control-variant-1")
	assert.Error(t, err)
}

// TestCancelExperimentNotRunning tests error handling
func TestCancelExperimentNotRunning(t *testing.T) {
	cc, hsc, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	ctx := context.Background()

	controlAgent := &AgentRegistration{
		Name:  "control",
		Type:  "technical",
		State: &AgentState{Memory: make(map[string]interface{}), Configuration: make(map[string]interface{}), PendingTasks: []*AgentTask{}, PerformanceMetrics: &PerformanceMetrics{}},
	}
	hsc.RegisterAgent(ctx, controlAgent)

	config := &ExperimentConfig{
		Duration:   1 * time.Minute,
		MinSamples: 10,
	}

	experiment, err := cc.StartABTest(ctx, "Test", "control", 1, config)
	require.NoError(t, err)

	// Cancel once
	cc.CancelExperiment(ctx, experiment.ID)

	// Try to cancel again
	err = cc.CancelExperiment(ctx, experiment.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment not running")
}

// TestHasMinimumSamples tests minimum sample checking
func TestHasMinimumSamples(t *testing.T) {
	cc, _, _, _, cleanup := setupTestCloningCoordinator(t)
	defer cleanup()

	experiment := &ABTestExperiment{
		Configuration: &ExperimentConfig{
			MinSamples: 10,
		},
		Metrics: map[string]*VariantMetrics{
			"control": {
				TotalRequests: 15,
			},
			"variant": {
				TotalRequests: 12,
			},
		},
	}

	// Both have minimum samples
	assert.True(t, cc.hasMinimumSamples(experiment))

	// One doesn't have minimum
	experiment.Metrics["variant"].TotalRequests = 5
	assert.False(t, cc.hasMinimumSamples(experiment))
}
