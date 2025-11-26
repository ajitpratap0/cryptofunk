package api

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Constants for decision queries
const (
	// EmbeddingDimension is the dimension of OpenAI text-embedding-ada-002 vectors
	EmbeddingDimension = 1536

	// DefaultSearchLimit is the default number of search results
	DefaultSearchLimit = 20
	// MaxSearchLimit is the maximum number of search results
	MaxSearchLimit = 100

	// DefaultListLimit is the default number of list results
	DefaultListLimit = 50
	// MaxListLimit is the maximum number of list results
	MaxListLimit = 500

	// DefaultSimilarLimit is the default number of similar decisions to return
	DefaultSimilarLimit = 10
	// MaxSimilarLimit is the maximum number of similar decisions to return
	MaxSimilarLimit = 50
)

// DecisionRepository handles database operations for LLM decisions
type DecisionRepository struct {
	db *pgxpool.Pool
}

// NewDecisionRepository creates a new decision repository
func NewDecisionRepository(db *pgxpool.Pool) *DecisionRepository {
	return &DecisionRepository{db: db}
}

// Decision represents an LLM decision record
type Decision struct {
	ID           uuid.UUID  `json:"id"`
	SessionID    *uuid.UUID `json:"session_id,omitempty"`
	DecisionType string     `json:"decision_type"`
	Symbol       string     `json:"symbol"`
	AgentName    *string    `json:"agent_name,omitempty"`
	Prompt       string     `json:"prompt"`
	Response     string     `json:"response"`
	Model        string     `json:"model"`
	TokensUsed   *int       `json:"tokens_used,omitempty"`
	LatencyMs    *int       `json:"latency_ms,omitempty"`
	Confidence   *float64   `json:"confidence,omitempty"`
	Outcome      *string    `json:"outcome,omitempty"`
	PnL          *float64   `json:"pnl,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// DecisionFilter contains filtering options for listing decisions
type DecisionFilter struct {
	Symbol       string
	DecisionType string
	Outcome      string
	Model        string
	FromDate     *time.Time
	ToDate       *time.Time
	Limit        int
	Offset       int
}

// DecisionStats contains aggregated statistics
type DecisionStats struct {
	TotalDecisions int            `json:"total_decisions"`
	ByType         map[string]int `json:"by_type"`
	ByOutcome      map[string]int `json:"by_outcome"`
	ByModel        map[string]int `json:"by_model"`
	AvgConfidence  float64        `json:"avg_confidence"`
	AvgLatencyMs   float64        `json:"avg_latency_ms"`
	AvgTokensUsed  float64        `json:"avg_tokens_used"`
	SuccessRate    float64        `json:"success_rate"`
	TotalPnL       float64        `json:"total_pnl"`
	AvgPnL         float64        `json:"avg_pnl"`
}

// ListDecisions retrieves decisions with optional filtering
func (r *DecisionRepository) ListDecisions(ctx context.Context, filter DecisionFilter) ([]Decision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, agent_name, prompt, response,
			model, tokens_used, latency_ms, confidence, outcome, outcome_pnl,
			created_at
		FROM llm_decisions
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argPos := 1

	// Add filters
	if filter.Symbol != "" {
		query += ` AND symbol = $` + itoa(argPos)
		args = append(args, filter.Symbol)
		argPos++
	}
	if filter.DecisionType != "" {
		query += ` AND decision_type = $` + itoa(argPos)
		args = append(args, filter.DecisionType)
		argPos++
	}
	if filter.Outcome != "" {
		query += ` AND outcome = $` + itoa(argPos)
		args = append(args, filter.Outcome)
		argPos++
	}
	if filter.Model != "" {
		query += ` AND model = $` + itoa(argPos)
		args = append(args, filter.Model)
		argPos++
	}
	if filter.FromDate != nil {
		query += ` AND created_at >= $` + itoa(argPos)
		args = append(args, *filter.FromDate)
		argPos++
	}
	if filter.ToDate != nil {
		query += ` AND created_at <= $` + itoa(argPos)
		args = append(args, *filter.ToDate)
		argPos++
	}

	// Order and pagination
	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += ` LIMIT $` + itoa(argPos)
		args = append(args, filter.Limit)
		argPos++
	}
	if filter.Offset > 0 {
		query += ` OFFSET $` + itoa(argPos)
		args = append(args, filter.Offset)
		// argPos++ - removed: ineffectual assignment (last use)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := make([]Decision, 0, filter.Limit)
	for rows.Next() {
		var d Decision
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.AgentName,
			&d.Prompt, &d.Response, &d.Model, &d.TokensUsed,
			&d.LatencyMs, &d.Confidence, &d.Outcome, &d.PnL,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}

	return decisions, rows.Err()
}

// GetDecision retrieves a single decision by ID
func (r *DecisionRepository) GetDecision(ctx context.Context, id uuid.UUID) (*Decision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, agent_name, prompt, response,
			model, tokens_used, latency_ms, confidence, outcome, outcome_pnl,
			created_at
		FROM llm_decisions
		WHERE id = $1
	`

	var d Decision
	err := r.db.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.AgentName,
		&d.Prompt, &d.Response, &d.Model, &d.TokensUsed,
		&d.LatencyMs, &d.Confidence, &d.Outcome, &d.PnL,
		&d.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// GetDecisionStats retrieves aggregated statistics
func (r *DecisionRepository) GetDecisionStats(ctx context.Context, filter DecisionFilter) (*DecisionStats, error) {
	query := `
		SELECT
			COUNT(*) as total,
			AVG(COALESCE(confidence, 0)) as avg_confidence,
			AVG(COALESCE(latency_ms, 0)) as avg_latency,
			AVG(COALESCE(tokens_used, 0)) as avg_tokens,
			SUM(CASE WHEN outcome = 'SUCCESS' THEN 1 ELSE 0 END)::FLOAT /
				NULLIF(COUNT(CASE WHEN outcome IS NOT NULL THEN 1 END), 0) as success_rate,
			SUM(COALESCE(outcome_pnl, 0)) as total_pnl,
			AVG(COALESCE(outcome_pnl, 0)) as avg_pnl
		FROM llm_decisions
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argPos := 1

	// Add filters (same as ListDecisions)
	if filter.Symbol != "" {
		query += ` AND symbol = $` + itoa(argPos)
		args = append(args, filter.Symbol)
		argPos++
	}
	if filter.DecisionType != "" {
		query += ` AND decision_type = $` + itoa(argPos)
		args = append(args, filter.DecisionType)
		argPos++
	}
	if filter.FromDate != nil {
		query += ` AND created_at >= $` + itoa(argPos)
		args = append(args, *filter.FromDate)
		argPos++
	}
	if filter.ToDate != nil {
		query += ` AND created_at <= $` + itoa(argPos)
		args = append(args, *filter.ToDate)
		// argPos++ - removed: ineffectual assignment (last use)
	}

	var stats DecisionStats
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&stats.TotalDecisions,
		&stats.AvgConfidence,
		&stats.AvgLatencyMs,
		&stats.AvgTokensUsed,
		&stats.SuccessRate,
		&stats.TotalPnL,
		&stats.AvgPnL,
	)
	if err != nil {
		return nil, err
	}

	// Get breakdown by type
	stats.ByType, err = r.getCountsByField(ctx, "decision_type", filter)
	if err != nil {
		return nil, err
	}

	// Get breakdown by outcome
	stats.ByOutcome, err = r.getCountsByField(ctx, "outcome", filter)
	if err != nil {
		return nil, err
	}

	// Get breakdown by model
	stats.ByModel, err = r.getCountsByField(ctx, "model", filter)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// allowedGroupByFields defines the whitelist of fields that can be used in GROUP BY
var allowedGroupByFields = map[string]bool{
	"decision_type": true,
	"outcome":       true,
	"model":         true,
}

// getCountsByField gets count breakdown by a specific field
func (r *DecisionRepository) getCountsByField(ctx context.Context, field string, filter DecisionFilter) (map[string]int, error) {
	// Validate field name to prevent SQL injection
	if !allowedGroupByFields[field] {
		return nil, fmt.Errorf("invalid field name: %s", field)
	}

	query := `
		SELECT ` + field + `, COUNT(*)
		FROM llm_decisions
		WHERE ` + field + ` IS NOT NULL
	`
	args := make([]interface{}, 0)
	argPos := 1

	// Add filters
	if filter.Symbol != "" {
		query += ` AND symbol = $` + itoa(argPos)
		args = append(args, filter.Symbol)
		argPos++
	}
	if filter.DecisionType != "" && field != "decision_type" {
		query += ` AND decision_type = $` + itoa(argPos)
		args = append(args, filter.DecisionType)
		argPos++
	}
	if filter.FromDate != nil {
		query += ` AND created_at >= $` + itoa(argPos)
		args = append(args, *filter.FromDate)
		argPos++
	}
	if filter.ToDate != nil {
		query += ` AND created_at <= $` + itoa(argPos)
		args = append(args, *filter.ToDate)
		// argPos++ - removed: ineffectual assignment (last use)
	}

	query += ` GROUP BY ` + field

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		result[key] = count
	}

	return result, rows.Err()
}

// FindSimilarDecisions finds decisions with similar prompts using vector similarity
func (r *DecisionRepository) FindSimilarDecisions(ctx context.Context, id uuid.UUID, limit int) ([]Decision, error) {
	query := `
		WITH target AS (
			SELECT prompt_embedding
			FROM llm_decisions
			WHERE id = $1 AND prompt_embedding IS NOT NULL
		)
		SELECT
			d.id, d.session_id, d.decision_type, d.symbol, d.agent_name, d.prompt, d.response,
			d.model, d.tokens_used, d.latency_ms, d.confidence, d.outcome, d.outcome_pnl,
			d.created_at,
			d.prompt_embedding <=> t.prompt_embedding as distance
		FROM llm_decisions d, target t
		WHERE d.id != $1
			AND d.prompt_embedding IS NOT NULL
		ORDER BY d.prompt_embedding <=> t.prompt_embedding
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := make([]Decision, 0, limit)
	for rows.Next() {
		var d Decision
		var distance float64
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.AgentName,
			&d.Prompt, &d.Response, &d.Model, &d.TokensUsed,
			&d.LatencyMs, &d.Confidence, &d.Outcome, &d.PnL,
			&d.CreatedAt, &distance,
		)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}

	return decisions, rows.Err()
}

// SearchRequest contains parameters for searching decisions
type SearchRequest struct {
	Query     string     `json:"query"`               // Text search query
	Embedding []float32  `json:"embedding,omitempty"` // Pre-computed embedding vector (1536 dim)
	Symbol    string     `json:"symbol,omitempty"`    // Filter by symbol
	FromDate  *time.Time `json:"from_date,omitempty"` // Filter by date range
	ToDate    *time.Time `json:"to_date,omitempty"`
	Limit     int        `json:"limit,omitempty"` // Max results (default 20, max 100)
}

// SearchResult represents a search result with relevance score
type SearchResult struct {
	Decision Decision `json:"decision"`
	Score    float64  `json:"score"` // Relevance score (0-1)
}

// SearchDecisions performs semantic search on decisions
// If embedding is provided, uses pgvector similarity search
// Otherwise, falls back to text search on prompt and response fields
func (r *DecisionRepository) SearchDecisions(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// Set default limit
	if req.Limit <= 0 {
		req.Limit = DefaultSearchLimit
	}
	if req.Limit > MaxSearchLimit {
		req.Limit = MaxSearchLimit
	}

	// If embedding is provided, use vector similarity search
	if len(req.Embedding) == EmbeddingDimension {
		return r.searchByEmbedding(ctx, req)
	}

	// Otherwise, use text search
	return r.searchByText(ctx, req)
}

// searchByEmbedding performs vector similarity search using pgvector
func (r *DecisionRepository) searchByEmbedding(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, agent_name, prompt, response,
			model, tokens_used, latency_ms, confidence, outcome, outcome_pnl,
			created_at,
			1 - (prompt_embedding <=> $1::vector) as similarity
		FROM llm_decisions
		WHERE prompt_embedding IS NOT NULL
	`
	args := []interface{}{req.Embedding}
	argPos := 2

	// Add filters
	if req.Symbol != "" {
		query += ` AND symbol = $` + itoa(argPos)
		args = append(args, req.Symbol)
		argPos++
	}
	if req.FromDate != nil {
		query += ` AND created_at >= $` + itoa(argPos)
		args = append(args, *req.FromDate)
		argPos++
	}
	if req.ToDate != nil {
		query += ` AND created_at <= $` + itoa(argPos)
		args = append(args, *req.ToDate)
		argPos++
	}

	query += ` ORDER BY prompt_embedding <=> $1::vector`
	query += ` LIMIT $` + itoa(argPos)
	args = append(args, req.Limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]SearchResult, 0, req.Limit)
	for rows.Next() {
		var d Decision
		var similarity float64
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.AgentName,
			&d.Prompt, &d.Response, &d.Model, &d.TokensUsed,
			&d.LatencyMs, &d.Confidence, &d.Outcome, &d.PnL,
			&d.CreatedAt, &similarity,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			Decision: d,
			Score:    similarity,
		})
	}

	return results, rows.Err()
}

// searchByText performs text-based search using PostgreSQL full-text search
func (r *DecisionRepository) searchByText(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, agent_name, prompt, response,
			model, tokens_used, latency_ms, confidence, outcome, outcome_pnl,
			created_at,
			ts_rank(
				to_tsvector('english', COALESCE(prompt, '') || ' ' || COALESCE(response, '')),
				plainto_tsquery('english', $1)
			) as rank
		FROM llm_decisions
		WHERE to_tsvector('english', COALESCE(prompt, '') || ' ' || COALESCE(response, ''))
			@@ plainto_tsquery('english', $1)
	`
	args := []interface{}{req.Query}
	argPos := 2

	// Add filters
	if req.Symbol != "" {
		query += ` AND symbol = $` + itoa(argPos)
		args = append(args, req.Symbol)
		argPos++
	}
	if req.FromDate != nil {
		query += ` AND created_at >= $` + itoa(argPos)
		args = append(args, *req.FromDate)
		argPos++
	}
	if req.ToDate != nil {
		query += ` AND created_at <= $` + itoa(argPos)
		args = append(args, *req.ToDate)
		argPos++
	}

	query += ` ORDER BY rank DESC`
	query += ` LIMIT $` + itoa(argPos)
	args = append(args, req.Limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		// Log the error for debugging, then fall back to ILIKE search
		log.Warn().Err(err).Str("query", req.Query).Msg("Full-text search failed, falling back to ILIKE")
		return r.searchByILike(ctx, req)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, req.Limit)
	for rows.Next() {
		var d Decision
		var rank float64
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.AgentName,
			&d.Prompt, &d.Response, &d.Model, &d.TokensUsed,
			&d.LatencyMs, &d.Confidence, &d.Outcome, &d.PnL,
			&d.CreatedAt, &rank,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			Decision: d,
			Score:    rank,
		})
	}

	// If no results from full-text search, try ILIKE
	if len(results) == 0 && req.Query != "" {
		return r.searchByILike(ctx, req)
	}

	return results, rows.Err()
}

// searchByILike performs simple ILIKE pattern matching as fallback
func (r *DecisionRepository) searchByILike(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	pattern := "%" + req.Query + "%"
	query := `
		SELECT
			id, session_id, decision_type, symbol, agent_name, prompt, response,
			model, tokens_used, latency_ms, confidence, outcome, outcome_pnl,
			created_at
		FROM llm_decisions
		WHERE (prompt ILIKE $1 OR response ILIKE $1)
	`
	args := []interface{}{pattern}
	argPos := 2

	// Add filters
	if req.Symbol != "" {
		query += ` AND symbol = $` + itoa(argPos)
		args = append(args, req.Symbol)
		argPos++
	}
	if req.FromDate != nil {
		query += ` AND created_at >= $` + itoa(argPos)
		args = append(args, *req.FromDate)
		argPos++
	}
	if req.ToDate != nil {
		query += ` AND created_at <= $` + itoa(argPos)
		args = append(args, *req.ToDate)
		argPos++
	}

	query += ` ORDER BY created_at DESC`
	query += ` LIMIT $` + itoa(argPos)
	args = append(args, req.Limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]SearchResult, 0, req.Limit)
	for rows.Next() {
		var d Decision
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.AgentName,
			&d.Prompt, &d.Response, &d.Model, &d.TokensUsed,
			&d.LatencyMs, &d.Confidence, &d.Outcome, &d.PnL,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			Decision: d,
			Score:    0.5, // Arbitrary score for ILIKE matches
		})
	}

	return results, rows.Err()
}

// Helper function to convert int to string for query building
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
