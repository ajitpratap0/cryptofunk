package llm

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ExperimentManager manages A/B testing experiments for LLMs
type ExperimentManager struct {
	experiments map[string]*Experiment
	tracker     *DecisionTracker
	mu          sync.RWMutex
}

// Experiment defines an A/B test comparing different LLM configurations
type Experiment struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Control     *Variant        `json:"control"`
	Variants    []*Variant      `json:"variants"`
	TrafficSplit map[string]float64 `json:"traffic_split"` // variant_id -> percentage (0-1)
	StartTime   time.Time       `json:"start_time"`
	EndTime     *time.Time      `json:"end_time,omitempty"`
	Active      bool            `json:"active"`
	Tags        []string        `json:"tags,omitempty"`
}

// Variant represents a configuration variant to test
type Variant struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Model       string  `json:"model"`         // e.g., "claude-sonnet-4", "gpt-4"
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	SystemPrompt string `json:"system_prompt,omitempty"` // Optional custom system prompt
	UserPromptTemplate string `json:"user_prompt_template,omitempty"` // Optional template
}

// ExperimentResult contains results for a specific variant
type ExperimentResult struct {
	VariantID       string  `json:"variant_id"`
	VariantName     string  `json:"variant_name"`
	TotalDecisions  int     `json:"total_decisions"`
	SuccessCount    int     `json:"success_count"`
	FailureCount    int     `json:"failure_count"`
	PendingCount    int     `json:"pending_count"`
	SuccessRate     float64 `json:"success_rate"`
	AvgPnL          float64 `json:"avg_pnl"`
	TotalPnL        float64 `json:"total_pnl"`
	AvgLatency      float64 `json:"avg_latency_ms"`
	AvgTokens       float64 `json:"avg_tokens"`
	AvgConfidence   float64 `json:"avg_confidence"`
}

// ExperimentComparison compares results across variants
type ExperimentComparison struct {
	ExperimentID    string              `json:"experiment_id"`
	ExperimentName  string              `json:"experiment_name"`
	Control         *ExperimentResult   `json:"control"`
	Variants        []*ExperimentResult `json:"variants"`
	WinningVariant  string              `json:"winning_variant,omitempty"`
	StatSigWinner   bool                `json:"statistically_significant"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

// NewExperimentManager creates a new experiment manager
func NewExperimentManager(tracker *DecisionTracker) *ExperimentManager {
	return &ExperimentManager{
		experiments: make(map[string]*Experiment),
		tracker:     tracker,
	}
}

// CreateExperiment creates and registers a new experiment
func (em *ExperimentManager) CreateExperiment(exp *Experiment) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if exp.ID == "" {
		exp.ID = uuid.New().String()
	}

	// Validate traffic split sums to 1.0
	totalTraffic := 0.0
	for _, pct := range exp.TrafficSplit {
		totalTraffic += pct
	}
	if totalTraffic < 0.99 || totalTraffic > 1.01 { // Allow small floating point errors
		return fmt.Errorf("traffic split must sum to 1.0, got %.2f", totalTraffic)
	}

	exp.Active = true
	if exp.StartTime.IsZero() {
		exp.StartTime = time.Now()
	}

	em.experiments[exp.ID] = exp

	log.Info().
		Str("experiment_id", exp.ID).
		Str("experiment_name", exp.Name).
		Msg("Created A/B experiment")

	return nil
}

// GetExperiment retrieves an experiment by ID
func (em *ExperimentManager) GetExperiment(experimentID string) (*Experiment, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	exp, exists := em.experiments[experimentID]
	return exp, exists
}

// ListActiveExperiments returns all active experiments
func (em *ExperimentManager) ListActiveExperiments() []*Experiment {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var active []*Experiment
	for _, exp := range em.experiments {
		if exp.Active && (exp.EndTime == nil || exp.EndTime.After(time.Now())) {
			active = append(active, exp)
		}
	}
	return active
}

// StopExperiment stops an active experiment
func (em *ExperimentManager) StopExperiment(experimentID string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	exp, exists := em.experiments[experimentID]
	if !exists {
		return fmt.Errorf("experiment not found: %s", experimentID)
	}

	exp.Active = false
	now := time.Now()
	exp.EndTime = &now

	log.Info().
		Str("experiment_id", exp.ID).
		Str("experiment_name", exp.Name).
		Msg("Stopped A/B experiment")

	return nil
}

// SelectVariant selects a variant based on traffic split
// Uses consistent hashing to ensure same decisionKey always gets same variant
func (em *ExperimentManager) SelectVariant(experimentID string, decisionKey string) (*Variant, error) {
	exp, exists := em.GetExperiment(experimentID)
	if !exists {
		return nil, fmt.Errorf("experiment not found: %s", experimentID)
	}

	if !exp.Active {
		return nil, fmt.Errorf("experiment is not active: %s", experimentID)
	}

	// Use consistent hashing based on decision key
	// This ensures same key always gets same variant (important for learning)
	hash := md5.Sum([]byte(decisionKey))
	hashInt := binary.BigEndian.Uint64(hash[:8])
	selection := float64(hashInt%10000) / 10000.0 // Convert to 0-1 range

	// Build ordered list of variants (sorted by ID for consistency)
	// Maps have undefined iteration order in Go, so we must sort
	type variantWithTraffic struct {
		id      string
		traffic float64
	}
	var orderedVariants []variantWithTraffic
	for variantID, traffic := range exp.TrafficSplit {
		orderedVariants = append(orderedVariants, variantWithTraffic{variantID, traffic})
	}
	// Sort by variant ID to ensure consistent ordering
	sort.Slice(orderedVariants, func(i, j int) bool {
		return orderedVariants[i].id < orderedVariants[j].id
	})

	// Select variant based on traffic split with consistent ordering
	cumulative := 0.0
	for _, vt := range orderedVariants {
		cumulative += vt.traffic
		if selection < cumulative {
			// Find the variant
			if exp.Control != nil && exp.Control.ID == vt.id {
				return exp.Control, nil
			}
			for _, v := range exp.Variants {
				if v.ID == vt.id {
					return v, nil
				}
			}
		}
	}

	// Fallback to control
	return exp.Control, nil
}

// TrackExperimentDecision tracks a decision made as part of an experiment
func (em *ExperimentManager) TrackExperimentDecision(
	ctx context.Context,
	experimentID string,
	variant *Variant,
	agentName string,
	decisionType string,
	symbol string,
	prompt string,
	response string,
	tokensUsed int,
	latencyMs int,
	confidence float64,
	contextData map[string]interface{},
	sessionID *uuid.UUID,
) (uuid.UUID, error) {
	if em.tracker == nil {
		return uuid.Nil, fmt.Errorf("decision tracker not configured")
	}

	// Add experiment metadata to context
	if contextData == nil {
		contextData = make(map[string]interface{})
	}
	contextData["experiment_id"] = experimentID
	contextData["variant_id"] = variant.ID
	contextData["variant_name"] = variant.Name

	// Track with the variant's model
	decisionID, err := em.tracker.TrackDecision(
		ctx,
		agentName,
		decisionType,
		symbol,
		prompt,
		response,
		variant.Model,
		tokensUsed,
		latencyMs,
		confidence,
		contextData,
		sessionID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to track experiment decision: %w", err)
	}

	log.Debug().
		Str("experiment_id", experimentID).
		Str("variant_id", variant.ID).
		Str("variant_name", variant.Name).
		Str("decision_id", decisionID.String()).
		Msg("Tracked experiment decision")

	return decisionID, nil
}

// GetExperimentResults retrieves aggregated results for an experiment
func (em *ExperimentManager) GetExperimentResults(
	ctx context.Context,
	experimentID string,
	database *db.DB,
) (*ExperimentComparison, error) {
	exp, exists := em.GetExperiment(experimentID)
	if !exists {
		return nil, fmt.Errorf("experiment not found: %s", experimentID)
	}

	comparison := &ExperimentComparison{
		ExperimentID:   exp.ID,
		ExperimentName: exp.Name,
		GeneratedAt:    time.Now(),
	}

	// Get results for control
	if exp.Control != nil {
		result, err := em.getVariantResults(ctx, database, experimentID, exp.Control, exp.StartTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get control results: %w", err)
		}
		comparison.Control = result
	}

	// Get results for all variants
	for _, variant := range exp.Variants {
		result, err := em.getVariantResults(ctx, database, experimentID, variant, exp.StartTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get variant %s results: %w", variant.ID, err)
		}
		comparison.Variants = append(comparison.Variants, result)
	}

	// Determine winner (simple heuristic: best success rate with enough data)
	comparison.WinningVariant = em.determineWinner(comparison)

	return comparison, nil
}

// getVariantResults retrieves results for a specific variant
func (em *ExperimentManager) getVariantResults(
	ctx context.Context,
	database *db.DB,
	experimentID string,
	variant *Variant,
	since time.Time,
) (*ExperimentResult, error) {
	// Query decisions for this variant
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN outcome = 'SUCCESS' THEN 1 END) as success,
			COUNT(CASE WHEN outcome = 'FAILURE' THEN 1 END) as failure,
			COUNT(CASE WHEN outcome IS NULL OR outcome = 'PENDING' THEN 1 END) as pending,
			AVG(CASE WHEN pnl IS NOT NULL THEN pnl END) as avg_pnl,
			SUM(CASE WHEN pnl IS NOT NULL THEN pnl ELSE 0 END) as total_pnl,
			AVG(latency_ms) as avg_latency,
			AVG(tokens_used) as avg_tokens,
			AVG(confidence) as avg_confidence
		FROM llm_decisions
		WHERE model = $1
		  AND created_at >= $2
		  AND context @> $3::jsonb
	`

	contextFilter := map[string]interface{}{
		"experiment_id": experimentID,
		"variant_id":    variant.ID,
	}
	contextJSON, _ := json.Marshal(contextFilter)

	var total, success, failure, pending int
	var avgPnL, totalPnL, avgLatency, avgTokens, avgConfidence *float64

	err := database.Pool().QueryRow(ctx, query, variant.Model, since, contextJSON).Scan(
		&total,
		&success,
		&failure,
		&pending,
		&avgPnL,
		&totalPnL,
		&avgLatency,
		&avgTokens,
		&avgConfidence,
	)
	if err != nil {
		return nil, err
	}

	result := &ExperimentResult{
		VariantID:      variant.ID,
		VariantName:    variant.Name,
		TotalDecisions: total,
		SuccessCount:   success,
		FailureCount:   failure,
		PendingCount:   pending,
	}

	if total > 0 {
		result.SuccessRate = float64(success) / float64(total) * 100.0
	}
	if avgPnL != nil {
		result.AvgPnL = *avgPnL
	}
	if totalPnL != nil {
		result.TotalPnL = *totalPnL
	}
	if avgLatency != nil {
		result.AvgLatency = *avgLatency
	}
	if avgTokens != nil {
		result.AvgTokens = *avgTokens
	}
	if avgConfidence != nil {
		result.AvgConfidence = *avgConfidence
	}

	return result, nil
}

// determineWinner determines the winning variant based on results
// Simple heuristic: highest success rate with at least 30 decisions
func (em *ExperimentManager) determineWinner(comparison *ExperimentComparison) string {
	minDecisions := 30
	bestSuccessRate := 0.0
	winner := ""

	// Check control
	if comparison.Control != nil && comparison.Control.TotalDecisions >= minDecisions {
		if comparison.Control.SuccessRate > bestSuccessRate {
			bestSuccessRate = comparison.Control.SuccessRate
			winner = comparison.Control.VariantID
		}
	}

	// Check variants
	for _, variant := range comparison.Variants {
		if variant.TotalDecisions >= minDecisions {
			if variant.SuccessRate > bestSuccessRate {
				bestSuccessRate = variant.SuccessRate
				winner = variant.VariantID
			}
		}
	}

	// Mark as statistically significant if we have enough data
	// Very simplified - real implementation would use proper statistical tests
	totalDecisions := 0
	if comparison.Control != nil {
		totalDecisions += comparison.Control.TotalDecisions
	}
	for _, v := range comparison.Variants {
		totalDecisions += v.TotalDecisions
	}
	comparison.StatSigWinner = totalDecisions >= 100

	return winner
}

// CreateClientFromVariant creates an LLM client configured for a variant
func CreateClientFromVariant(variant *Variant, baseConfig ClientConfig) *Client {
	config := baseConfig
	config.Model = variant.Model
	config.Temperature = variant.Temperature
	if variant.MaxTokens > 0 {
		config.MaxTokens = variant.MaxTokens
	}

	return NewClient(config)
}
