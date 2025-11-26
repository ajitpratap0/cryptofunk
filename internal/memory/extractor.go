package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// KnowledgeExtractor extracts knowledge from historical data and stores it in semantic memory
type KnowledgeExtractor struct {
	pool           *pgxpool.Pool
	semanticMemory *SemanticMemory
	embeddingFunc  EmbeddingFunc // Function to generate embeddings
	minConfidence  float64
	minOccurrences int
}

// EmbeddingFunc is a function that generates embeddings for text
type EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

// ExtractionConfig configures the knowledge extraction process
type ExtractionConfig struct {
	MinConfidence  float64 // Minimum confidence to store knowledge (default: 0.5)
	MinOccurrences int     // Minimum pattern occurrences to extract (default: 3)
	EmbeddingFunc  EmbeddingFunc
}

// DefaultExtractionConfig returns sensible defaults
func DefaultExtractionConfig() ExtractionConfig {
	return ExtractionConfig{
		MinConfidence:  0.5,
		MinOccurrences: 3,
		EmbeddingFunc:  nil, // Must be provided
	}
}

// NewKnowledgeExtractor creates a new knowledge extractor
func NewKnowledgeExtractor(pool *pgxpool.Pool, config ExtractionConfig) *KnowledgeExtractor {
	if config.MinConfidence == 0 {
		config.MinConfidence = 0.5
	}
	if config.MinOccurrences == 0 {
		config.MinOccurrences = 3
	}

	return &KnowledgeExtractor{
		pool:           pool,
		semanticMemory: NewSemanticMemory(pool),
		embeddingFunc:  config.EmbeddingFunc,
		minConfidence:  config.MinConfidence,
		minOccurrences: config.MinOccurrences,
	}
}

// NewKnowledgeExtractorFromDB creates an extractor from existing DB connection
func NewKnowledgeExtractorFromDB(database *db.DB, config ExtractionConfig) *KnowledgeExtractor {
	extractor := NewKnowledgeExtractor(database.Pool(), config)
	return extractor
}

// PatternCandidate represents a potential pattern to extract
type PatternCandidate struct {
	Condition    string
	Outcome      string
	Occurrences  int
	SuccessCount int
	FailureCount int
	AvgPnL       float64
	Symbols      []string
	AgentNames   []string
	DecisionIDs  []uuid.UUID
}

// SuccessRate returns the success rate of this pattern
func (pc *PatternCandidate) SuccessRate() float64 {
	total := pc.SuccessCount + pc.FailureCount
	if total == 0 {
		return 0.0
	}
	return float64(pc.SuccessCount) / float64(total)
}

// Confidence returns confidence score based on occurrences and success rate
func (pc *PatternCandidate) Confidence() float64 {
	// More occurrences = higher confidence (up to 10 occurrences)
	occurrenceScore := math.Min(float64(pc.Occurrences)/10.0, 1.0)

	// Success rate contributes to confidence
	successScore := pc.SuccessRate()

	// Weighted combination
	return occurrenceScore*0.4 + successScore*0.6
}

// ExtractFromLLMDecisions analyzes LLM decisions and extracts patterns
func (ke *KnowledgeExtractor) ExtractFromLLMDecisions(ctx context.Context, agentName string, since time.Time) (int, error) {
	log.Info().
		Str("agent", agentName).
		Time("since", since).
		Msg("Starting knowledge extraction from LLM decisions")

	// Get successful decisions
	successfulDecisions, err := ke.getSuccessfulDecisions(ctx, agentName, since)
	if err != nil {
		return 0, fmt.Errorf("failed to get successful decisions: %w", err)
	}

	// Get failed decisions
	failedDecisions, err := ke.getFailedDecisions(ctx, agentName, since)
	if err != nil {
		return 0, fmt.Errorf("failed to get failed decisions: %w", err)
	}

	log.Info().
		Int("successful", len(successfulDecisions)).
		Int("failed", len(failedDecisions)).
		Msg("Retrieved LLM decisions for analysis")

	// Extract patterns from successful decisions
	patterns := ke.identifyPatterns(successfulDecisions, failedDecisions)

	// Store patterns as knowledge
	stored := 0
	for _, pattern := range patterns {
		if pattern.Confidence() >= ke.minConfidence && pattern.Occurrences >= ke.minOccurrences {
			knowledge, err := ke.createKnowledgeFromPattern(ctx, pattern, agentName)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to create knowledge from pattern")
				continue
			}

			if err := ke.semanticMemory.Store(ctx, knowledge); err != nil {
				log.Warn().Err(err).Msg("Failed to store knowledge")
				continue
			}

			stored++
			log.Debug().
				Str("content", knowledge.Content).
				Float64("confidence", knowledge.Confidence).
				Msg("Stored knowledge from pattern")
		}
	}

	log.Info().
		Int("patterns_found", len(patterns)).
		Int("stored", stored).
		Msg("Completed knowledge extraction from LLM decisions")

	return stored, nil
}

// ExtractFromTradingResults analyzes trading results and extracts experiences
func (ke *KnowledgeExtractor) ExtractFromTradingResults(ctx context.Context, agentName string, since time.Time) (int, error) {
	log.Info().
		Str("agent", agentName).
		Time("since", since).
		Msg("Starting knowledge extraction from trading results")

	// Get trading sessions
	query := `
		SELECT
			id, strategy, symbol, entry_price, exit_price, pnl, pnl_percent,
			entry_time, exit_time, notes
		FROM trading_sessions
		WHERE agent_name = $1
		  AND created_at >= $2
		  AND exit_time IS NOT NULL
		  AND pnl IS NOT NULL
		ORDER BY created_at DESC
	`

	rows, err := ke.pool.Query(ctx, query, agentName, since)
	if err != nil {
		return 0, fmt.Errorf("failed to query trading sessions: %w", err)
	}
	defer rows.Close()

	type tradingSession struct {
		ID         uuid.UUID
		Strategy   string
		Symbol     string
		EntryPrice float64
		ExitPrice  *float64
		PnL        *float64
		PnLPercent *float64
		EntryTime  time.Time
		ExitTime   *time.Time
		Notes      *string
	}

	var sessions []tradingSession
	for rows.Next() {
		var s tradingSession
		err := rows.Scan(
			&s.ID, &s.Strategy, &s.Symbol, &s.EntryPrice, &s.ExitPrice,
			&s.PnL, &s.PnLPercent, &s.EntryTime, &s.ExitTime, &s.Notes,
		)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	log.Info().
		Int("sessions", len(sessions)).
		Msg("Retrieved trading sessions for analysis")

	// Extract experiences
	experiences := ke.extractExperiences(sessions)

	// Store experiences as knowledge
	stored := 0
	for _, exp := range experiences {
		knowledge, err := ke.createKnowledgeFromExperience(ctx, exp, agentName)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create knowledge from experience")
			continue
		}

		if err := ke.semanticMemory.Store(ctx, knowledge); err != nil {
			log.Warn().Err(err).Msg("Failed to store knowledge")
			continue
		}

		stored++
	}

	log.Info().
		Int("experiences_found", len(experiences)).
		Int("stored", stored).
		Msg("Completed knowledge extraction from trading results")

	return stored, nil
}

// ExtractFactsFromMarketData analyzes market data to extract factual knowledge
func (ke *KnowledgeExtractor) ExtractFactsFromMarketData(ctx context.Context, symbol string, since time.Time) (int, error) {
	log.Info().
		Str("symbol", symbol).
		Time("since", since).
		Msg("Starting fact extraction from market data")

	// Analyze price volatility patterns
	volatilityFacts := ke.analyzeVolatilityPatterns(ctx, symbol, since)

	// Analyze volume patterns
	volumeFacts := ke.analyzeVolumePatterns(ctx, symbol, since)

	// Combine all facts
	allFacts := append(volatilityFacts, volumeFacts...)

	// Store facts as knowledge
	stored := 0
	for _, fact := range allFacts {
		knowledge, err := ke.createKnowledgeFromFact(ctx, fact, symbol)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create knowledge from fact")
			continue
		}

		if err := ke.semanticMemory.Store(ctx, knowledge); err != nil {
			log.Warn().Err(err).Msg("Failed to store knowledge")
			continue
		}

		stored++
	}

	log.Info().
		Int("facts_found", len(allFacts)).
		Int("stored", stored).
		Msg("Completed fact extraction from market data")

	return stored, nil
}

// Helper methods

func (ke *KnowledgeExtractor) getSuccessfulDecisions(ctx context.Context, agentName string, since time.Time) ([]*db.LLMDecision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, prompt, response,
			model, tokens_used, latency_ms, outcome, pnl, context,
			agent_name, confidence, created_at
		FROM llm_decisions
		WHERE agent_name = $1
		  AND created_at >= $2
		  AND outcome = 'SUCCESS'
		  AND pnl > 0
		ORDER BY created_at DESC
		LIMIT 1000
	`

	rows, err := ke.pool.Query(ctx, query, agentName, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ke.scanLLMDecisions(rows)
}

func (ke *KnowledgeExtractor) getFailedDecisions(ctx context.Context, agentName string, since time.Time) ([]*db.LLMDecision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, prompt, response,
			model, tokens_used, latency_ms, outcome, pnl, context,
			agent_name, confidence, created_at
		FROM llm_decisions
		WHERE agent_name = $1
		  AND created_at >= $2
		  AND outcome = 'FAILURE'
		ORDER BY created_at DESC
		LIMIT 1000
	`

	rows, err := ke.pool.Query(ctx, query, agentName, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ke.scanLLMDecisions(rows)
}

func (ke *KnowledgeExtractor) scanLLMDecisions(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*db.LLMDecision, error) {
	var decisions []*db.LLMDecision

	for rows.Next() {
		var d db.LLMDecision
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.Prompt,
			&d.Response, &d.Model, &d.TokensUsed, &d.LatencyMs,
			&d.Outcome, &d.PnL, &d.Context, &d.AgentName, &d.Confidence,
			&d.CreatedAt,
		)
		if err != nil {
			continue
		}
		decisions = append(decisions, &d)
	}

	return decisions, rows.Err()
}

func (ke *KnowledgeExtractor) identifyPatterns(successful, failed []*db.LLMDecision) []*PatternCandidate {
	patterns := make(map[string]*PatternCandidate)

	// Analyze successful decisions
	for _, decision := range successful {
		conditions := ke.extractConditions(decision)
		for _, condition := range conditions {
			key := fmt.Sprintf("%s:SUCCESS", condition)
			if _, exists := patterns[key]; !exists {
				patterns[key] = &PatternCandidate{
					Condition:   condition,
					Outcome:     "typically leads to profitable trades",
					Symbols:     []string{},
					AgentNames:  []string{},
					DecisionIDs: []uuid.UUID{},
				}
			}
			p := patterns[key]
			p.Occurrences++
			p.SuccessCount++
			if decision.PnL != nil {
				p.AvgPnL = (p.AvgPnL*float64(p.Occurrences-1) + *decision.PnL) / float64(p.Occurrences)
			}
			p.Symbols = appendUnique(p.Symbols, decision.Symbol)
			p.AgentNames = appendUnique(p.AgentNames, decision.AgentName)
			p.DecisionIDs = append(p.DecisionIDs, decision.ID)
		}
	}

	// Analyze failed decisions
	for _, decision := range failed {
		conditions := ke.extractConditions(decision)
		for _, condition := range conditions {
			// Check if we have a success pattern for this condition
			successKey := fmt.Sprintf("%s:SUCCESS", condition)
			if p, exists := patterns[successKey]; exists {
				p.FailureCount++
			} else {
				// Create failure pattern
				failKey := fmt.Sprintf("%s:FAILURE", condition)
				if _, exists := patterns[failKey]; !exists {
					patterns[failKey] = &PatternCandidate{
						Condition:   condition,
						Outcome:     "often leads to losses",
						Symbols:     []string{},
						AgentNames:  []string{},
						DecisionIDs: []uuid.UUID{},
					}
				}
				p := patterns[failKey]
				p.Occurrences++
				p.FailureCount++
				if decision.PnL != nil {
					p.AvgPnL = (p.AvgPnL*float64(p.Occurrences-1) + *decision.PnL) / float64(p.Occurrences)
				}
				p.Symbols = appendUnique(p.Symbols, decision.Symbol)
				p.AgentNames = appendUnique(p.AgentNames, decision.AgentName)
				p.DecisionIDs = append(p.DecisionIDs, decision.ID)
			}
		}
	}

	// Convert map to slice
	result := make([]*PatternCandidate, 0, len(patterns))
	for _, p := range patterns {
		result = append(result, p)
	}

	return result
}

func (ke *KnowledgeExtractor) extractConditions(decision *db.LLMDecision) []string {
	var conditions []string

	// Parse context JSONB to extract indicators and conditions
	if len(decision.Context) == 0 {
		return conditions
	}

	var context map[string]interface{}
	if err := json.Unmarshal(decision.Context, &context); err != nil {
		return conditions
	}

	// Extract indicator-based conditions
	if indicators, ok := context["indicators"].(map[string]interface{}); ok {
		for name, value := range indicators {
			condition := formatIndicatorCondition(name, value)
			if condition != "" {
				conditions = append(conditions, condition)
			}
		}
	}

	// Extract market condition
	if marketCondition, ok := context["market_condition"].(string); ok {
		conditions = append(conditions, fmt.Sprintf("market condition is %s", marketCondition))
	}

	return conditions
}

func formatIndicatorCondition(name string, value interface{}) string {
	switch v := value.(type) {
	case float64:
		// Create range-based conditions
		switch name {
		case "rsi":
			if v >= 70 {
				return "RSI exceeds 70 (overbought)"
			} else if v <= 30 {
				return "RSI below 30 (oversold)"
			}
		case "macd":
			if v > 0 {
				return "MACD is positive (bullish)"
			} else if v < 0 {
				return "MACD is negative (bearish)"
			}
		}
		return fmt.Sprintf("%s is %.2f", name, v)
	case bool:
		return fmt.Sprintf("%s is %v", name, v)
	case string:
		return fmt.Sprintf("%s is %s", name, v)
	}
	return ""
}

type Experience struct {
	Description string
	SuccessRate float64
	AvgPnL      float64
	Occurrences int
	Symbol      string
}

func (ke *KnowledgeExtractor) extractExperiences(sessions interface{}) []*Experience {
	// For now, return empty - this would analyze trading sessions
	// to extract lessons like "stop losses at 2% work better than 5%"
	// TODO: Analyze sessions to find patterns like:
	// - Optimal stop loss percentages
	// - Best entry/exit timing
	// - Symbol-specific strategies
	return []*Experience{}
}

type Fact struct {
	Statement  string
	Confidence float64
	Source     string
}

func (ke *KnowledgeExtractor) analyzeVolatilityPatterns(ctx context.Context, symbol string, since time.Time) []*Fact {
	// Placeholder - would analyze candlestick data
	return []*Fact{}
}

func (ke *KnowledgeExtractor) analyzeVolumePatterns(ctx context.Context, symbol string, since time.Time) []*Fact {
	// Placeholder - would analyze volume patterns
	return []*Fact{}
}

func (ke *KnowledgeExtractor) createKnowledgeFromPattern(ctx context.Context, pattern *PatternCandidate, agentName string) (*KnowledgeItem, error) {
	// Generate natural language description
	content := fmt.Sprintf("When %s, this %s (observed %d times, %.1f%% success rate, avg P&L: $%.2f)",
		pattern.Condition,
		pattern.Outcome,
		pattern.Occurrences,
		pattern.SuccessRate()*100,
		pattern.AvgPnL,
	)

	// Generate embedding if function provided
	var embedding []float32
	if ke.embeddingFunc != nil {
		var err error
		embedding, err = ke.embeddingFunc(ctx, content)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}
	}

	// Create context
	contextData := map[string]interface{}{
		"condition":    pattern.Condition,
		"outcome":      pattern.Outcome,
		"occurrences":  pattern.Occurrences,
		"success_rate": pattern.SuccessRate(),
		"avg_pnl":      pattern.AvgPnL,
		"symbols":      pattern.Symbols,
		"decision_ids": pattern.DecisionIDs,
	}
	contextJSON, _ := json.Marshal(contextData)

	// Determine symbol (if pattern is specific to one)
	var symbol *string
	if len(pattern.Symbols) == 1 {
		symbol = &pattern.Symbols[0]
	}

	knowledge := &KnowledgeItem{
		Type:            KnowledgePattern,
		Content:         content,
		Embedding:       embedding,
		Confidence:      pattern.Confidence(),
		Importance:      calculateImportance(pattern),
		Source:          "pattern_extraction",
		AgentName:       agentName,
		Symbol:          symbol,
		Context:         contextJSON,
		ValidationCount: pattern.Occurrences,
		SuccessCount:    pattern.SuccessCount,
		FailureCount:    pattern.FailureCount,
	}

	return knowledge, nil
}

func (ke *KnowledgeExtractor) createKnowledgeFromExperience(ctx context.Context, exp *Experience, agentName string) (*KnowledgeItem, error) {
	// Generate embedding if function provided
	var embedding []float32
	if ke.embeddingFunc != nil {
		var err error
		embedding, err = ke.embeddingFunc(ctx, exp.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}
	}

	knowledge := &KnowledgeItem{
		Type:       KnowledgeExperience,
		Content:    exp.Description,
		Embedding:  embedding,
		Confidence: exp.SuccessRate,
		Importance: 0.7,
		Source:     "trading_results",
		AgentName:  agentName,
	}

	return knowledge, nil
}

func (ke *KnowledgeExtractor) createKnowledgeFromFact(ctx context.Context, fact *Fact, symbol string) (*KnowledgeItem, error) {
	// Generate embedding if function provided
	var embedding []float32
	if ke.embeddingFunc != nil {
		var err error
		embedding, err = ke.embeddingFunc(ctx, fact.Statement)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}
	}

	knowledge := &KnowledgeItem{
		Type:       KnowledgeFact,
		Content:    fact.Statement,
		Embedding:  embedding,
		Confidence: fact.Confidence,
		Importance: 0.6,
		Source:     "market_data_analysis",
		Symbol:     &symbol,
	}

	return knowledge, nil
}

func calculateImportance(pattern *PatternCandidate) float64 {
	// Higher P&L and more occurrences = more important
	occurrenceScore := math.Min(float64(pattern.Occurrences)/20.0, 1.0)
	pnlScore := math.Min(math.Abs(pattern.AvgPnL)/100.0, 1.0)

	return occurrenceScore*0.5 + pnlScore*0.5
}

func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}
