package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog/log"
)

// KnowledgeType represents the type of knowledge stored in semantic memory
type KnowledgeType string

const (
	// KnowledgeFact represents factual information (e.g., "BTC price tends to rise after halving events")
	KnowledgeFact KnowledgeType = "fact"

	// KnowledgePattern represents observed patterns (e.g., "When RSI > 70 and volume decreases, price often corrects")
	KnowledgePattern KnowledgeType = "pattern"

	// KnowledgeExperience represents learned experiences (e.g., "Stop losses at 2% work better than 5% for volatile assets")
	KnowledgeExperience KnowledgeType = "experience"

	// KnowledgeStrategy represents strategic knowledge (e.g., "Mean reversion works better in ranging markets")
	KnowledgeStrategy KnowledgeType = "strategy"

	// KnowledgeRisk represents risk-related knowledge (e.g., "Drawdowns > 15% indicate strategy failure")
	KnowledgeRisk KnowledgeType = "risk"
)

// KnowledgeItem represents a piece of knowledge in semantic memory
type KnowledgeItem struct {
	ID uuid.UUID `json:"id"`

	// Knowledge metadata
	Type        KnowledgeType `json:"type"`
	Content     string        `json:"content"`      // Natural language description
	Embedding   []float32     `json:"embedding"`    // 1536-dim vector for similarity search
	Confidence  float64       `json:"confidence"`   // 0.0 to 1.0
	Importance  float64       `json:"importance"`   // 0.0 to 1.0, affects retrieval priority
	AccessCount int           `json:"access_count"` // How many times this knowledge was accessed

	// Provenance (where did this knowledge come from?)
	Source    string     `json:"source"`     // "llm_decision", "manual", "backtest", "pattern_extraction"
	SourceID  *uuid.UUID `json:"source_id"`  // ID of source (decision ID, backtest ID, etc.)
	AgentName string     `json:"agent_name"` // Which agent learned/created this knowledge
	Symbol    *string    `json:"symbol"`     // Associated symbol (if applicable)
	Context   []byte     `json:"context"`    // JSONB - additional context (market conditions, etc.)

	// Validation
	ValidationCount int       `json:"validation_count"` // How many times this knowledge was validated
	SuccessCount    int       `json:"success_count"`    // How many times applying this knowledge succeeded
	FailureCount    int       `json:"failure_count"`    // How many times it failed
	LastValidated   time.Time `json:"last_validated"`

	// Temporal
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at"` // Optional expiration for time-sensitive knowledge
}

// SuccessRate returns the success rate of this knowledge (0.0 to 1.0)
func (k *KnowledgeItem) SuccessRate() float64 {
	total := k.SuccessCount + k.FailureCount
	if total == 0 {
		return 0.0
	}
	return float64(k.SuccessCount) / float64(total)
}

// IsValid checks if the knowledge is still valid (not expired, has good success rate)
func (k *KnowledgeItem) IsValid() bool {
	// Check expiration
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}

	// Check minimum success rate (50%)
	if k.ValidationCount >= 5 && k.SuccessRate() < 0.5 {
		return false
	}

	return true
}

// Age returns how old this knowledge is
func (k *KnowledgeItem) Age() time.Duration {
	return time.Since(k.CreatedAt)
}

// Recency returns a score (0.0 to 1.0) based on how recent the knowledge is
// Newer knowledge gets higher scores
func (k *KnowledgeItem) Recency() float64 {
	age := k.Age()
	days := age.Hours() / 24.0

	// Exponential decay: score = e^(-days/30)
	// Knowledge loses ~63% of its recency value after 30 days
	decayRate := 30.0
	return 1.0 / (1.0 + days/decayRate)
}

// RelevanceScore combines multiple factors into a single relevance score
func (k *KnowledgeItem) RelevanceScore() float64 {
	if !k.IsValid() {
		return 0.0
	}

	// Weighted combination of factors
	score := 0.0
	score += k.Confidence * 0.3    // 30% confidence
	score += k.Importance * 0.3    // 30% importance
	score += k.SuccessRate() * 0.2 // 20% success rate
	score += k.Recency() * 0.2     // 20% recency

	return score
}

// SemanticMemory manages knowledge storage and retrieval using vector embeddings
type SemanticMemory struct {
	pool *pgxpool.Pool
}

// NewSemanticMemory creates a new semantic memory instance
func NewSemanticMemory(pool *pgxpool.Pool) *SemanticMemory {
	return &SemanticMemory{
		pool: pool,
	}
}

// NewSemanticMemoryFromDB creates a semantic memory instance from existing DB connection
func NewSemanticMemoryFromDB(database *db.DB) *SemanticMemory {
	return &SemanticMemory{
		pool: database.Pool(),
	}
}

// Store stores a knowledge item in semantic memory
func (sm *SemanticMemory) Store(ctx context.Context, item *KnowledgeItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	item.UpdatedAt = time.Now()

	// Convert []float32 to pgvector.Vector for proper database storage
	var embedding pgvector.Vector
	if item.Embedding != nil {
		embedding = pgvector.NewVector(item.Embedding)
	}

	query := `
		INSERT INTO semantic_memory (
			id, type, content, embedding, confidence, importance, access_count,
			source, source_id, agent_name, symbol, context,
			validation_count, success_count, failure_count, last_validated,
			created_at, updated_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19
		)
		ON CONFLICT (id) DO UPDATE SET
			confidence = EXCLUDED.confidence,
			importance = EXCLUDED.importance,
			access_count = EXCLUDED.access_count,
			validation_count = EXCLUDED.validation_count,
			success_count = EXCLUDED.success_count,
			failure_count = EXCLUDED.failure_count,
			last_validated = EXCLUDED.last_validated,
			updated_at = EXCLUDED.updated_at
	`

	_, err := sm.pool.Exec(
		ctx,
		query,
		item.ID,
		item.Type,
		item.Content,
		embedding,
		item.Confidence,
		item.Importance,
		item.AccessCount,
		item.Source,
		item.SourceID,
		item.AgentName,
		item.Symbol,
		item.Context,
		item.ValidationCount,
		item.SuccessCount,
		item.FailureCount,
		item.LastValidated,
		item.CreatedAt,
		item.UpdatedAt,
		item.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to store knowledge: %w", err)
	}

	log.Debug().
		Str("id", item.ID.String()).
		Str("type", string(item.Type)).
		Str("agent", item.AgentName).
		Msg("Stored knowledge in semantic memory")

	return nil
}

// FindSimilar finds knowledge items similar to the given embedding
func (sm *SemanticMemory) FindSimilar(ctx context.Context, embedding []float32, limit int, filters ...Filter) ([]*KnowledgeItem, error) {
	if len(embedding) != 1536 {
		return nil, fmt.Errorf("embedding must be 1536 dimensions, got %d", len(embedding))
	}

	// Convert []float32 to pgvector.Vector
	vec := pgvector.NewVector(embedding)

	// Build WHERE clause from filters
	whereClause := "WHERE embedding IS NOT NULL"
	args := []interface{}{vec, limit}
	argIndex := 3 // Start from $3 (after embedding and limit)

	for _, filter := range filters {
		clause, filterArgs := filter.SQL(argIndex)
		if clause != "" {
			whereClause += " AND " + clause
			args = append(args, filterArgs...)
			argIndex += len(filterArgs)
		}
	}

	query := fmt.Sprintf(`
		SELECT
			id, type, content, embedding, confidence, importance, access_count,
			source, source_id, agent_name, symbol, context,
			validation_count, success_count, failure_count, last_validated,
			created_at, updated_at, expires_at,
			embedding <=> $1 as distance
		FROM semantic_memory
		%s
		ORDER BY embedding <=> $1
		LIMIT $2
	`, whereClause)

	rows, err := sm.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query similar knowledge: %w", err)
	}
	defer rows.Close()

	var items []*KnowledgeItem
	for rows.Next() {
		var item KnowledgeItem
		var distance float32
		var lastValidated *time.Time
		var embeddingVec *pgvector.Vector

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Content,
			&embeddingVec,
			&item.Confidence,
			&item.Importance,
			&item.AccessCount,
			&item.Source,
			&item.SourceID,
			&item.AgentName,
			&item.Symbol,
			&item.Context,
			&item.ValidationCount,
			&item.SuccessCount,
			&item.FailureCount,
			&lastValidated,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ExpiresAt,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan knowledge item: %w", err)
		}

		// Convert pgvector.Vector to []float32 (handle NULL)
		if embeddingVec != nil {
			item.Embedding = embeddingVec.Slice()
		}

		if lastValidated != nil {
			item.LastValidated = *lastValidated
		}

		// Record access (best effort telemetry)
		go func(id uuid.UUID) {
			_ = sm.RecordAccess(context.Background(), id)
		}(item.ID)

		items = append(items, &item)
	}

	log.Debug().
		Int("count", len(items)).
		Int("limit", limit).
		Msg("Found similar knowledge items")

	return items, rows.Err()
}

// FindByType retrieves knowledge items of a specific type
func (sm *SemanticMemory) FindByType(ctx context.Context, knowledgeType KnowledgeType, limit int) ([]*KnowledgeItem, error) {
	query := `
		SELECT
			id, type, content, embedding, confidence, importance, access_count,
			source, source_id, agent_name, symbol, context,
			validation_count, success_count, failure_count, last_validated,
			created_at, updated_at, expires_at
		FROM semantic_memory
		WHERE type = $1
		ORDER BY importance DESC, created_at DESC
		LIMIT $2
	`

	rows, err := sm.pool.Query(ctx, query, knowledgeType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query knowledge by type: %w", err)
	}
	defer rows.Close()

	return sm.scanKnowledgeItems(rows)
}

// FindByAgent retrieves knowledge learned by a specific agent
func (sm *SemanticMemory) FindByAgent(ctx context.Context, agentName string, limit int) ([]*KnowledgeItem, error) {
	query := `
		SELECT
			id, type, content, embedding, confidence, importance, access_count,
			source, source_id, agent_name, symbol, context,
			validation_count, success_count, failure_count, last_validated,
			created_at, updated_at, expires_at
		FROM semantic_memory
		WHERE agent_name = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := sm.pool.Query(ctx, query, agentName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query knowledge by agent: %w", err)
	}
	defer rows.Close()

	return sm.scanKnowledgeItems(rows)
}

// GetMostRelevant retrieves the most relevant knowledge items based on multiple criteria
func (sm *SemanticMemory) GetMostRelevant(ctx context.Context, limit int, filters ...Filter) ([]*KnowledgeItem, error) {
	// Build WHERE clause
	whereClause := "WHERE TRUE"
	args := []interface{}{limit}
	argIndex := 2

	for _, filter := range filters {
		clause, filterArgs := filter.SQL(argIndex)
		if clause != "" {
			whereClause += " AND " + clause
			args = append(args, filterArgs...)
			argIndex += len(filterArgs)
		}
	}

	query := fmt.Sprintf(`
		SELECT
			id, type, content, embedding, confidence, importance, access_count,
			source, source_id, agent_name, symbol, context,
			validation_count, success_count, failure_count, last_validated,
			created_at, updated_at, expires_at
		FROM semantic_memory
		%s
		ORDER BY importance DESC, confidence DESC, created_at DESC
		LIMIT $1
	`, whereClause)

	rows, err := sm.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query relevant knowledge: %w", err)
	}
	defer rows.Close()

	return sm.scanKnowledgeItems(rows)
}

// RecordAccess increments the access count for a knowledge item
func (sm *SemanticMemory) RecordAccess(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE semantic_memory
		SET access_count = access_count + 1,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := sm.pool.Exec(ctx, query, id)
	return err
}

// RecordValidation records a validation attempt (success or failure)
func (sm *SemanticMemory) RecordValidation(ctx context.Context, id uuid.UUID, success bool) error {
	var query string
	if success {
		query = `
			UPDATE semantic_memory
			SET validation_count = validation_count + 1,
			    success_count = success_count + 1,
			    last_validated = NOW(),
			    updated_at = NOW()
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE semantic_memory
			SET validation_count = validation_count + 1,
			    failure_count = failure_count + 1,
			    last_validated = NOW(),
			    updated_at = NOW()
			WHERE id = $1
		`
	}

	_, err := sm.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to record validation: %w", err)
	}

	log.Debug().
		Str("id", id.String()).
		Bool("success", success).
		Msg("Recorded knowledge validation")

	return nil
}

// UpdateConfidence updates the confidence level of a knowledge item
func (sm *SemanticMemory) UpdateConfidence(ctx context.Context, id uuid.UUID, confidence float64) error {
	if confidence < 0.0 || confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %f", confidence)
	}

	query := `
		UPDATE semantic_memory
		SET confidence = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := sm.pool.Exec(ctx, query, id, confidence)
	return err
}

// Delete removes a knowledge item from semantic memory
func (sm *SemanticMemory) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM semantic_memory WHERE id = $1`
	_, err := sm.pool.Exec(ctx, query, id)
	return err
}

// PruneExpired removes expired knowledge items
func (sm *SemanticMemory) PruneExpired(ctx context.Context) (int, error) {
	query := `
		DELETE FROM semantic_memory
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
	`

	result, err := sm.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prune expired knowledge: %w", err)
	}

	count := result.RowsAffected()
	if count > 0 {
		log.Info().
			Int64("count", count).
			Msg("Pruned expired knowledge items")
	}

	return int(count), nil
}

// PruneLowQuality removes knowledge items with low success rates
func (sm *SemanticMemory) PruneLowQuality(ctx context.Context, minValidations int, minSuccessRate float64) (int, error) {
	query := `
		DELETE FROM semantic_memory
		WHERE validation_count >= $1
		  AND (success_count::float / NULLIF(validation_count, 0)) < $2
	`

	result, err := sm.pool.Exec(ctx, query, minValidations, minSuccessRate)
	if err != nil {
		return 0, fmt.Errorf("failed to prune low-quality knowledge: %w", err)
	}

	count := result.RowsAffected()
	if count > 0 {
		log.Info().
			Int64("count", count).
			Float64("min_success_rate", minSuccessRate).
			Msg("Pruned low-quality knowledge items")
	}

	return int(count), nil
}

// GetStats returns statistics about semantic memory
func (sm *SemanticMemory) GetStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_items,
			COUNT(CASE WHEN type = 'fact' THEN 1 END) as facts,
			COUNT(CASE WHEN type = 'pattern' THEN 1 END) as patterns,
			COUNT(CASE WHEN type = 'experience' THEN 1 END) as experiences,
			COUNT(CASE WHEN type = 'strategy' THEN 1 END) as strategies,
			COUNT(CASE WHEN type = 'risk' THEN 1 END) as risk_knowledge,
			AVG(confidence) as avg_confidence,
			AVG(importance) as avg_importance,
			AVG(CASE WHEN validation_count > 0 THEN success_count::float / validation_count END) as avg_success_rate,
			SUM(access_count) as total_accesses
		FROM semantic_memory
	`

	var stats struct {
		TotalItems     int64
		Facts          int64
		Patterns       int64
		Experiences    int64
		Strategies     int64
		RiskKnowledge  int64
		AvgConfidence  *float64
		AvgImportance  *float64
		AvgSuccessRate *float64
		TotalAccesses  int64
	}

	err := sm.pool.QueryRow(ctx, query).Scan(
		&stats.TotalItems,
		&stats.Facts,
		&stats.Patterns,
		&stats.Experiences,
		&stats.Strategies,
		&stats.RiskKnowledge,
		&stats.AvgConfidence,
		&stats.AvgImportance,
		&stats.AvgSuccessRate,
		&stats.TotalAccesses,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	result := map[string]interface{}{
		"total_items":    stats.TotalItems,
		"facts":          stats.Facts,
		"patterns":       stats.Patterns,
		"experiences":    stats.Experiences,
		"strategies":     stats.Strategies,
		"risk_knowledge": stats.RiskKnowledge,
		"total_accesses": stats.TotalAccesses,
	}

	if stats.AvgConfidence != nil {
		result["avg_confidence"] = *stats.AvgConfidence
	}
	if stats.AvgImportance != nil {
		result["avg_importance"] = *stats.AvgImportance
	}
	if stats.AvgSuccessRate != nil {
		result["avg_success_rate"] = *stats.AvgSuccessRate
	}

	return result, nil
}

// scanKnowledgeItems is a helper to scan knowledge items from query results
func (sm *SemanticMemory) scanKnowledgeItems(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*KnowledgeItem, error) {
	var items []*KnowledgeItem

	for rows.Next() {
		var item KnowledgeItem
		var lastValidated *time.Time
		var embeddingVec *pgvector.Vector

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Content,
			&embeddingVec,
			&item.Confidence,
			&item.Importance,
			&item.AccessCount,
			&item.Source,
			&item.SourceID,
			&item.AgentName,
			&item.Symbol,
			&item.Context,
			&item.ValidationCount,
			&item.SuccessCount,
			&item.FailureCount,
			&lastValidated,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan knowledge item: %w", err)
		}

		// Convert pgvector.Vector to []float32 (handle NULL)
		if embeddingVec != nil {
			item.Embedding = embeddingVec.Slice()
		}

		if lastValidated != nil {
			item.LastValidated = *lastValidated
		}

		items = append(items, &item)
	}

	return items, rows.Err()
}

// Filter represents a query filter for semantic memory
type Filter interface {
	SQL(argIndex int) (clause string, args []interface{})
}

// TypeFilter filters by knowledge type
type TypeFilter struct {
	Type KnowledgeType
}

func (f TypeFilter) SQL(argIndex int) (string, []interface{}) {
	return fmt.Sprintf("type = $%d", argIndex), []interface{}{f.Type}
}

// AgentFilter filters by agent name
type AgentFilter struct {
	AgentName string
}

func (f AgentFilter) SQL(argIndex int) (string, []interface{}) {
	return fmt.Sprintf("agent_name = $%d", argIndex), []interface{}{f.AgentName}
}

// SymbolFilter filters by symbol
type SymbolFilter struct {
	Symbol string
}

func (f SymbolFilter) SQL(argIndex int) (string, []interface{}) {
	return fmt.Sprintf("symbol = $%d", argIndex), []interface{}{f.Symbol}
}

// MinConfidenceFilter filters by minimum confidence
type MinConfidenceFilter struct {
	MinConfidence float64
}

func (f MinConfidenceFilter) SQL(argIndex int) (string, []interface{}) {
	return fmt.Sprintf("confidence >= $%d", argIndex), []interface{}{f.MinConfidence}
}

// ValidOnlyFilter filters to only valid knowledge (not expired, good success rate)
type ValidOnlyFilter struct{}

func (f ValidOnlyFilter) SQL(argIndex int) (string, []interface{}) {
	return "(expires_at IS NULL OR expires_at > NOW()) AND (validation_count < 5 OR (success_count::float / NULLIF(validation_count, 0)) >= 0.5)", nil
}

// Helper function to create context JSON
func CreateKnowledgeContext(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}
