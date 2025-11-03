package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// CloningCoordinator manages agent cloning and A/B testing
type CloningCoordinator struct {
	hotSwap     *HotSwapCoordinator
	blackboard  *Blackboard
	messageBus  *MessageBus
	experiments map[uuid.UUID]*ABTestExperiment // Experiment ID -> experiment
	mu          sync.RWMutex
}

// ABTestExperiment represents an A/B testing experiment
type ABTestExperiment struct {
	ID              uuid.UUID                  `json:"id"`
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	ControlAgent    string                     `json:"control_agent"`  // Original agent
	VariantAgents   []string                   `json:"variant_agents"` // Cloned variants
	Status          ExperimentStatus           `json:"status"`
	Configuration   *ExperimentConfig          `json:"configuration"`
	Metrics         map[string]*VariantMetrics `json:"metrics"` // Agent name -> metrics
	Winner          string                     `json:"winner,omitempty"`
	WinningStrategy string                     `json:"winning_strategy,omitempty"`
	StartedAt       time.Time                  `json:"started_at"`
	CompletedAt     *time.Time                 `json:"completed_at,omitempty"`
	Duration        time.Duration              `json:"duration"`
	Results         *ExperimentResults         `json:"results,omitempty"`
}

// ExperimentStatus represents experiment status
type ExperimentStatus string

const (
	ExperimentStatusSetup     ExperimentStatus = "setup"
	ExperimentStatusRunning   ExperimentStatus = "running"
	ExperimentStatusCompleted ExperimentStatus = "completed"
	ExperimentStatusFailed    ExperimentStatus = "failed"
	ExperimentStatusCancelled ExperimentStatus = "cancelled"
)

// ExperimentConfig configures an A/B test experiment
type ExperimentConfig struct {
	Duration          time.Duration          `json:"duration"`            // Experiment duration
	MinSamples        int                    `json:"min_samples"`         // Minimum samples needed
	SignificanceLevel float64                `json:"significance_level"`  // Statistical significance (e.g., 0.95)
	TrafficSplit      map[string]float64     `json:"traffic_split"`       // Agent -> % of traffic (0.0-1.0)
	ComparisonMetric  string                 `json:"comparison_metric"`   // Primary metric to compare
	VariantConfigs    map[string]interface{} `json:"variant_configs"`     // Variant-specific configs
	AutoSelectWinner  bool                   `json:"auto_select_winner"`  // Automatically promote winner
	RollbackOnFailure bool                   `json:"rollback_on_failure"` // Rollback if variant underperforms
}

// VariantMetrics tracks performance metrics for a variant
type VariantMetrics struct {
	AgentName          string             `json:"agent_name"`
	TotalRequests      int64              `json:"total_requests"`
	SuccessfulRequests int64              `json:"successful_requests"`
	FailedRequests     int64              `json:"failed_requests"`
	AverageLatency     time.Duration      `json:"average_latency"`
	P50Latency         time.Duration      `json:"p50_latency"`
	P95Latency         time.Duration      `json:"p95_latency"`
	P99Latency         time.Duration      `json:"p99_latency"`
	ErrorRate          float64            `json:"error_rate"`
	Throughput         float64            `json:"throughput"` // Requests per second
	CustomMetrics      map[string]float64 `json:"custom_metrics"`
	Samples            []*MetricSample    `json:"samples"`
	LastUpdated        time.Time          `json:"last_updated"`
}

// MetricSample represents a single metric sample
type MetricSample struct {
	Timestamp time.Time              `json:"timestamp"`
	Latency   time.Duration          `json:"latency"`
	Success   bool                   `json:"success"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ExperimentResults contains final experiment results
type ExperimentResults struct {
	Winner          string                       `json:"winner"`
	WinnerMetrics   *VariantMetrics              `json:"winner_metrics"`
	Comparison      map[string]*ComparisonResult `json:"comparison"` // Agent -> comparison vs control
	StatSignificant bool                         `json:"stat_significant"`
	ConfidenceLevel float64                      `json:"confidence_level"`
	Recommendation  string                       `json:"recommendation"`
	Summary         string                       `json:"summary"`
}

// ComparisonResult compares a variant against the control
type ComparisonResult struct {
	VariantName        string  `json:"variant_name"`
	LatencyImprovement float64 `json:"latency_improvement"` // % improvement (positive = better)
	ErrorRateChange    float64 `json:"error_rate_change"`   // % change (negative = better)
	ThroughputChange   float64 `json:"throughput_change"`   // % change (positive = better)
	OverallScore       float64 `json:"overall_score"`       // Weighted score (0-100)
	BetterThanControl  bool    `json:"better_than_control"`
}

// CloneConfig configures agent cloning
type CloneConfig struct {
	SourceAgent     string                 `json:"source_agent"`
	CloneName       string                 `json:"clone_name"`
	InheritState    bool                   `json:"inherit_state"`    // Inherit source agent's state
	ConfigOverrides map[string]interface{} `json:"config_overrides"` // Override configuration
	Metadata        map[string]interface{} `json:"metadata"`
}

// NewCloningCoordinator creates a new cloning coordinator
func NewCloningCoordinator(hotSwap *HotSwapCoordinator, blackboard *Blackboard, messageBus *MessageBus) *CloningCoordinator {
	return &CloningCoordinator{
		hotSwap:     hotSwap,
		blackboard:  blackboard,
		messageBus:  messageBus,
		experiments: make(map[uuid.UUID]*ABTestExperiment),
	}
}

// CloneAgent creates a clone of an existing agent
func (cc *CloningCoordinator) CloneAgent(ctx context.Context, config *CloneConfig) (*AgentRegistration, error) {
	// Get source agent
	sourceAgent, err := cc.hotSwap.GetAgent(config.SourceAgent)
	if err != nil {
		return nil, fmt.Errorf("source agent not found: %w", err)
	}

	// Create clone
	clone := &AgentRegistration{
		Name:         config.CloneName,
		Type:         sourceAgent.Type,
		Version:      fmt.Sprintf("%s-clone", sourceAgent.Version),
		Capabilities: append([]string{}, sourceAgent.Capabilities...), // Deep copy
		Metadata:     config.Metadata,
	}

	// Inherit state if requested
	if config.InheritState && sourceAgent.State != nil {
		// Deep copy state
		stateJSON, err := json.Marshal(sourceAgent.State)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize source state: %w", err)
		}

		var clonedState AgentState
		if err := json.Unmarshal(stateJSON, &clonedState); err != nil {
			return nil, fmt.Errorf("failed to deserialize state: %w", err)
		}

		// Apply configuration overrides
		if config.ConfigOverrides != nil {
			for key, value := range config.ConfigOverrides {
				clonedState.Configuration[key] = value
			}
		}

		clonedState.LastUpdated = time.Now()
		clone.State = &clonedState
	}

	// Register clone
	if err := cc.hotSwap.RegisterAgent(ctx, clone); err != nil {
		return nil, fmt.Errorf("failed to register clone: %w", err)
	}

	log.Info().
		Str("source", config.SourceAgent).
		Str("clone", config.CloneName).
		Bool("inherited_state", config.InheritState).
		Msg("Agent cloned successfully")

	// Post to blackboard
	msg, _ := NewMessage("agent_clones", "orchestrator", map[string]interface{}{
		"source": config.SourceAgent,
		"clone":  config.CloneName,
		"config": config,
	})
	if err := cc.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post clone event to blackboard")
	}

	return clone, nil
}

// StartABTest initiates an A/B testing experiment
func (cc *CloningCoordinator) StartABTest(ctx context.Context, name, controlAgent string, numVariants int, config *ExperimentConfig) (*ABTestExperiment, error) {
	// Validate control agent exists
	_, err := cc.hotSwap.GetAgent(controlAgent)
	if err != nil {
		return nil, fmt.Errorf("control agent not found: %w", err)
	}

	// Create experiment
	experiment := &ABTestExperiment{
		ID:            uuid.New(),
		Name:          name,
		Description:   fmt.Sprintf("A/B test: %s with %d variants", controlAgent, numVariants),
		ControlAgent:  controlAgent,
		VariantAgents: []string{},
		Status:        ExperimentStatusSetup,
		Configuration: config,
		Metrics:       make(map[string]*VariantMetrics),
		StartedAt:     time.Now(),
	}

	// Initialize control metrics
	experiment.Metrics[controlAgent] = &VariantMetrics{
		AgentName:     controlAgent,
		CustomMetrics: make(map[string]float64),
		Samples:       []*MetricSample{},
		LastUpdated:   time.Now(),
	}

	// Create variant clones
	for i := 0; i < numVariants; i++ {
		variantName := fmt.Sprintf("%s-variant-%d", controlAgent, i+1)

		// Get variant-specific config if provided
		variantConfig := make(map[string]interface{})
		if config.VariantConfigs != nil {
			if cfg, exists := config.VariantConfigs[variantName]; exists {
				if cfgMap, ok := cfg.(map[string]interface{}); ok {
					variantConfig = cfgMap
				}
			}
		}

		cloneConfig := &CloneConfig{
			SourceAgent:     controlAgent,
			CloneName:       variantName,
			InheritState:    true,
			ConfigOverrides: variantConfig,
			Metadata: map[string]interface{}{
				"experiment_id": experiment.ID.String(),
				"variant_index": i,
			},
		}

		clone, err := cc.CloneAgent(ctx, cloneConfig)
		if err != nil {
			// Cleanup already created clones
			for _, v := range experiment.VariantAgents {
				cc.hotSwap.UnregisterAgent(ctx, v)
			}
			return nil, fmt.Errorf("failed to create variant %d: %w", i+1, err)
		}

		experiment.VariantAgents = append(experiment.VariantAgents, clone.Name)

		// Initialize variant metrics
		experiment.Metrics[clone.Name] = &VariantMetrics{
			AgentName:     clone.Name,
			CustomMetrics: make(map[string]float64),
			Samples:       []*MetricSample{},
			LastUpdated:   time.Now(),
		}
	}

	// Calculate default traffic split if not provided
	if config.TrafficSplit == nil {
		config.TrafficSplit = make(map[string]float64)
		splitPerAgent := 1.0 / float64(numVariants+1) // +1 for control

		config.TrafficSplit[controlAgent] = splitPerAgent
		for _, variant := range experiment.VariantAgents {
			config.TrafficSplit[variant] = splitPerAgent
		}
	}

	// Save experiment
	cc.mu.Lock()
	cc.experiments[experiment.ID] = experiment
	cc.mu.Unlock()

	// Start experiment
	experiment.Status = ExperimentStatusRunning

	log.Info().
		Str("experiment_id", experiment.ID.String()).
		Str("name", name).
		Str("control", controlAgent).
		Int("variants", numVariants).
		Msg("A/B test experiment started")

	// Post to blackboard
	msg, _ := NewMessage("ab_tests", "orchestrator", experiment)
	msg.WithMetadata("event", "experiment_started")
	if err := cc.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post experiment to blackboard")
	}

	// Start monitoring goroutine
	go cc.monitorExperiment(ctx, experiment)

	return experiment, nil
}

// RecordMetric records a metric sample for an agent in an experiment
func (cc *CloningCoordinator) RecordMetric(ctx context.Context, experimentID uuid.UUID, agentName string, latency time.Duration, success bool, metadata map[string]interface{}) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	experiment, exists := cc.experiments[experimentID]
	if !exists {
		return fmt.Errorf("experiment not found: %s", experimentID)
	}

	metrics, exists := experiment.Metrics[agentName]
	if !exists {
		return fmt.Errorf("agent not part of experiment: %s", agentName)
	}

	// Record sample
	sample := &MetricSample{
		Timestamp: time.Now(),
		Latency:   latency,
		Success:   success,
		Metadata:  metadata,
	}

	metrics.Samples = append(metrics.Samples, sample)
	metrics.TotalRequests++

	if success {
		metrics.SuccessfulRequests++
	} else {
		metrics.FailedRequests++
	}

	// Update aggregate metrics
	cc.updateAggregateMetrics(metrics)

	return nil
}

// updateAggregateMetrics recalculates aggregate metrics from samples
func (cc *CloningCoordinator) updateAggregateMetrics(metrics *VariantMetrics) {
	if len(metrics.Samples) == 0 {
		return
	}

	// Calculate average latency
	totalLatency := time.Duration(0)
	latencies := make([]time.Duration, 0, len(metrics.Samples))

	for _, sample := range metrics.Samples {
		totalLatency += sample.Latency
		latencies = append(latencies, sample.Latency)
	}

	metrics.AverageLatency = totalLatency / time.Duration(len(metrics.Samples))

	// Calculate percentiles (simple implementation)
	// For production, use a proper percentile algorithm
	if len(latencies) > 0 {
		// Sort latencies for percentile calculation
		// Using simple indexing for approximation
		p50Index := int(float64(len(latencies)) * 0.5)
		p95Index := int(float64(len(latencies)) * 0.95)
		p99Index := int(float64(len(latencies)) * 0.99)

		if p50Index < len(latencies) {
			metrics.P50Latency = latencies[p50Index]
		}
		if p95Index < len(latencies) {
			metrics.P95Latency = latencies[p95Index]
		}
		if p99Index < len(latencies) {
			metrics.P99Latency = latencies[p99Index]
		}
	}

	// Calculate error rate
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	}

	// Calculate throughput (requests per second)
	if len(metrics.Samples) > 1 {
		firstSample := metrics.Samples[0]
		lastSample := metrics.Samples[len(metrics.Samples)-1]
		duration := lastSample.Timestamp.Sub(firstSample.Timestamp).Seconds()
		if duration > 0 {
			metrics.Throughput = float64(len(metrics.Samples)) / duration
		}
	}

	metrics.LastUpdated = time.Now()
}

// monitorExperiment monitors an experiment and auto-completes when ready
func (cc *CloningCoordinator) monitorExperiment(ctx context.Context, experiment *ABTestExperiment) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(experiment.Configuration.Duration)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			cc.completeExperiment(ctx, experiment)
			return
		case <-ticker.C:
			// Check if minimum samples reached
			cc.mu.RLock()
			minSamplesReached := cc.hasMinimumSamples(experiment)
			cc.mu.RUnlock()

			if minSamplesReached && experiment.Configuration.AutoSelectWinner {
				cc.completeExperiment(ctx, experiment)
				return
			}
		}
	}
}

// hasMinimumSamples checks if all variants have minimum samples
func (cc *CloningCoordinator) hasMinimumSamples(experiment *ABTestExperiment) bool {
	for _, metrics := range experiment.Metrics {
		if metrics.TotalRequests < int64(experiment.Configuration.MinSamples) {
			return false
		}
	}
	return true
}

// completeExperiment finalizes an experiment and selects a winner
func (cc *CloningCoordinator) completeExperiment(ctx context.Context, experiment *ABTestExperiment) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if experiment.Status != ExperimentStatusRunning {
		return
	}

	experiment.Status = ExperimentStatusCompleted
	now := time.Now()
	experiment.CompletedAt = &now
	experiment.Duration = now.Sub(experiment.StartedAt)

	// Analyze results
	results := cc.analyzeResults(experiment)
	experiment.Results = results
	experiment.Winner = results.Winner

	log.Info().
		Str("experiment_id", experiment.ID.String()).
		Str("winner", results.Winner).
		Float64("confidence", results.ConfidenceLevel).
		Dur("duration", experiment.Duration).
		Msg("A/B test experiment completed")

	// Post results to blackboard
	msg, _ := NewMessage("ab_tests", "orchestrator", results)
	msg.WithMetadata("experiment_id", experiment.ID.String())
	msg.WithMetadata("event", "experiment_completed")
	if err := cc.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post experiment results to blackboard")
	}

	// Auto-select winner if configured
	if experiment.Configuration.AutoSelectWinner && results.StatSignificant {
		if err := cc.promoteWinner(ctx, experiment); err != nil {
			log.Error().Err(err).Msg("Failed to promote winner")
		}
	}
}

// analyzeResults analyzes experiment results and determines winner
func (cc *CloningCoordinator) analyzeResults(experiment *ABTestExperiment) *ExperimentResults {
	controlMetrics := experiment.Metrics[experiment.ControlAgent]

	results := &ExperimentResults{
		Comparison: make(map[string]*ComparisonResult),
	}

	bestScore := 0.0
	bestAgent := experiment.ControlAgent

	// Compare each variant against control
	for agentName, metrics := range experiment.Metrics {
		if agentName == experiment.ControlAgent {
			continue
		}

		comparison := cc.compareVariants(controlMetrics, metrics)
		results.Comparison[agentName] = comparison

		if comparison.OverallScore > bestScore {
			bestScore = comparison.OverallScore
			bestAgent = agentName
		}
	}

	results.Winner = bestAgent
	results.WinnerMetrics = experiment.Metrics[bestAgent]

	// Determine statistical significance (simplified)
	// In production, use proper statistical tests (t-test, etc.)
	results.StatSignificant = bestScore > 60.0 // Simple threshold
	results.ConfidenceLevel = experiment.Configuration.SignificanceLevel

	// Generate recommendation
	if bestAgent == experiment.ControlAgent {
		results.Recommendation = "Keep control agent - no variants showed significant improvement"
		results.Summary = fmt.Sprintf("Control agent performed best with score %.2f", bestScore)
	} else {
		bestComparison := results.Comparison[bestAgent]
		results.Recommendation = fmt.Sprintf("Promote %s - showed %.2f%% latency improvement", bestAgent, bestComparison.LatencyImprovement)
		results.Summary = fmt.Sprintf("%s won with overall score %.2f (%.2f%% better latency, %.2f%% lower error rate)",
			bestAgent, bestScore, bestComparison.LatencyImprovement, -bestComparison.ErrorRateChange)
	}

	return results
}

// compareVariants compares a variant against control
func (cc *CloningCoordinator) compareVariants(control, variant *VariantMetrics) *ComparisonResult {
	comparison := &ComparisonResult{
		VariantName: variant.AgentName,
	}

	// Calculate latency improvement (positive = better)
	if control.AverageLatency > 0 {
		comparison.LatencyImprovement = ((float64(control.AverageLatency) - float64(variant.AverageLatency)) / float64(control.AverageLatency)) * 100
	}

	// Calculate error rate change (negative = better)
	comparison.ErrorRateChange = (variant.ErrorRate - control.ErrorRate) * 100

	// Calculate throughput change (positive = better)
	if control.Throughput > 0 {
		comparison.ThroughputChange = ((variant.Throughput - control.Throughput) / control.Throughput) * 100
	}

	// Calculate overall score (0-100, weighted)
	// Weights: latency 50%, error rate 30%, throughput 20%
	latencyScore := 50.0 + (comparison.LatencyImprovement * 0.5)
	errorScore := 30.0 - (comparison.ErrorRateChange * 0.3)
	throughputScore := 20.0 + (comparison.ThroughputChange * 0.2)

	comparison.OverallScore = latencyScore + errorScore + throughputScore

	// Clamp to 0-100
	if comparison.OverallScore < 0 {
		comparison.OverallScore = 0
	}
	if comparison.OverallScore > 100 {
		comparison.OverallScore = 100
	}

	comparison.BetterThanControl = comparison.OverallScore > 50.0

	return comparison
}

// promoteWinner promotes the winning variant
func (cc *CloningCoordinator) promoteWinner(ctx context.Context, experiment *ABTestExperiment) error {
	if experiment.Winner == experiment.ControlAgent {
		log.Info().
			Str("experiment_id", experiment.ID.String()).
			Msg("Control agent remains - no promotion needed")
		return nil
	}

	// Hot-swap control with winner
	_, err := cc.hotSwap.SwapAgent(ctx, experiment.ControlAgent, experiment.Winner, map[string]interface{}{
		"promoted_from_experiment": experiment.ID.String(),
		"previous_agent":           experiment.ControlAgent,
	})

	if err != nil {
		return fmt.Errorf("failed to promote winner: %w", err)
	}

	log.Info().
		Str("experiment_id", experiment.ID.String()).
		Str("winner", experiment.Winner).
		Str("replaced", experiment.ControlAgent).
		Msg("Winner promoted successfully")

	// Cleanup non-winning variants
	for _, variant := range experiment.VariantAgents {
		if variant != experiment.Winner {
			if err := cc.hotSwap.UnregisterAgent(ctx, variant); err != nil {
				log.Warn().Err(err).Str("variant", variant).Msg("Failed to cleanup variant")
			}
		}
	}

	return nil
}

// GetExperiment retrieves an experiment by ID
func (cc *CloningCoordinator) GetExperiment(experimentID uuid.UUID) (*ABTestExperiment, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	experiment, exists := cc.experiments[experimentID]
	if !exists {
		return nil, fmt.Errorf("experiment not found: %s", experimentID)
	}

	return experiment, nil
}

// ListExperiments returns all experiments
func (cc *CloningCoordinator) ListExperiments() []*ABTestExperiment {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	experiments := make([]*ABTestExperiment, 0, len(cc.experiments))
	for _, exp := range cc.experiments {
		experiments = append(experiments, exp)
	}

	return experiments
}

// CancelExperiment cancels a running experiment
func (cc *CloningCoordinator) CancelExperiment(ctx context.Context, experimentID uuid.UUID) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	experiment, exists := cc.experiments[experimentID]
	if !exists {
		return fmt.Errorf("experiment not found: %s", experimentID)
	}

	if experiment.Status != ExperimentStatusRunning {
		return fmt.Errorf("experiment not running: %s", experiment.Status)
	}

	experiment.Status = ExperimentStatusCancelled
	now := time.Now()
	experiment.CompletedAt = &now
	experiment.Duration = now.Sub(experiment.StartedAt)

	// Cleanup variants
	for _, variant := range experiment.VariantAgents {
		if err := cc.hotSwap.UnregisterAgent(ctx, variant); err != nil {
			log.Warn().Err(err).Str("variant", variant).Msg("Failed to cleanup variant")
		}
	}

	log.Info().
		Str("experiment_id", experimentID.String()).
		Msg("Experiment cancelled")

	return nil
}
