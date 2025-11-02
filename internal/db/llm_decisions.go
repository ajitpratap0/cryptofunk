package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LLMDecision represents a decision made by an LLM
type LLMDecision struct {
	ID               uuid.UUID  `json:"id"`
	SessionID        *uuid.UUID `json:"session_id,omitempty"`
	DecisionType     string     `json:"decision_type"` // 'signal', 'risk_approval', 'position_sizing', etc.
	Symbol           string     `json:"symbol"`
	Prompt           string     `json:"prompt"`
	PromptEmbedding  []float32  `json:"prompt_embedding,omitempty"` // 1536-dim OpenAI embeddings
	Response         string     `json:"response"`
	Model            string     `json:"model"`
	TokensUsed       int        `json:"tokens_used"`
	LatencyMs        int        `json:"latency_ms"`
	Outcome          *string    `json:"outcome,omitempty"`          // 'SUCCESS', 'FAILURE', 'PENDING'
	PnL              *float64   `json:"pnl,omitempty"`              // Profit/Loss if outcome is known
	Context          []byte     `json:"context,omitempty"`          // JSONB - market conditions, indicators, etc.
	AgentName        string     `json:"agent_name"`
	Confidence       float64    `json:"confidence"`
	CreatedAt        time.Time  `json:"created_at"`
}

// InsertLLMDecision records an LLM decision in the database
func (db *DB) InsertLLMDecision(ctx context.Context, decision *LLMDecision) error {
	query := `
		INSERT INTO llm_decisions (
			id, session_id, decision_type, symbol, prompt, prompt_embedding,
			response, model, tokens_used, latency_ms, outcome, pnl,
			context, agent_name, confidence, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16
		)
	`

	_, err := db.pool.Exec(
		ctx,
		query,
		decision.ID,
		decision.SessionID,
		decision.DecisionType,
		decision.Symbol,
		decision.Prompt,
		decision.PromptEmbedding,
		decision.Response,
		decision.Model,
		decision.TokensUsed,
		decision.LatencyMs,
		decision.Outcome,
		decision.PnL,
		decision.Context,
		decision.AgentName,
		decision.Confidence,
		decision.CreatedAt,
	)

	return err
}

// UpdateLLMDecisionOutcome updates the outcome and P&L of a decision
func (db *DB) UpdateLLMDecisionOutcome(ctx context.Context, id uuid.UUID, outcome string, pnl float64) error {
	query := `
		UPDATE llm_decisions
		SET outcome = $2, pnl = $3
		WHERE id = $1
	`

	_, err := db.pool.Exec(ctx, query, id, outcome, pnl)
	return err
}

// GetLLMDecisionsByAgent retrieves recent decisions for a specific agent
func (db *DB) GetLLMDecisionsByAgent(ctx context.Context, agentName string, limit int) ([]*LLMDecision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, prompt,
			response, model, tokens_used, latency_ms, outcome, pnl,
			context, agent_name, confidence, created_at
		FROM llm_decisions
		WHERE agent_name = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.pool.Query(ctx, query, agentName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decisions []*LLMDecision
	for rows.Next() {
		var d LLMDecision
		err := rows.Scan(
			&d.ID,
			&d.SessionID,
			&d.DecisionType,
			&d.Symbol,
			&d.Prompt,
			&d.Response,
			&d.Model,
			&d.TokensUsed,
			&d.LatencyMs,
			&d.Outcome,
			&d.PnL,
			&d.Context,
			&d.AgentName,
			&d.Confidence,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, &d)
	}

	return decisions, rows.Err()
}

// GetLLMDecisionsBySymbol retrieves recent decisions for a specific symbol
func (db *DB) GetLLMDecisionsBySymbol(ctx context.Context, symbol string, limit int) ([]*LLMDecision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, prompt,
			response, model, tokens_used, latency_ms, outcome, pnl,
			context, agent_name, confidence, created_at
		FROM llm_decisions
		WHERE symbol = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.pool.Query(ctx, query, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decisions []*LLMDecision
	for rows.Next() {
		var d LLMDecision
		err := rows.Scan(
			&d.ID,
			&d.SessionID,
			&d.DecisionType,
			&d.Symbol,
			&d.Prompt,
			&d.Response,
			&d.Model,
			&d.TokensUsed,
			&d.LatencyMs,
			&d.Outcome,
			&d.PnL,
			&d.Context,
			&d.AgentName,
			&d.Confidence,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, &d)
	}

	return decisions, rows.Err()
}

// GetSuccessfulLLMDecisions retrieves decisions with positive outcomes for learning
func (db *DB) GetSuccessfulLLMDecisions(ctx context.Context, agentName string, limit int) ([]*LLMDecision, error) {
	query := `
		SELECT
			id, session_id, decision_type, symbol, prompt,
			response, model, tokens_used, latency_ms, outcome, pnl,
			context, agent_name, confidence, created_at
		FROM llm_decisions
		WHERE agent_name = $1
		  AND outcome = 'SUCCESS'
		  AND pnl > 0
		ORDER BY pnl DESC, created_at DESC
		LIMIT $2
	`

	rows, err := db.pool.Query(ctx, query, agentName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decisions []*LLMDecision
	for rows.Next() {
		var d LLMDecision
		err := rows.Scan(
			&d.ID,
			&d.SessionID,
			&d.DecisionType,
			&d.Symbol,
			&d.Prompt,
			&d.Response,
			&d.Model,
			&d.TokensUsed,
			&d.LatencyMs,
			&d.Outcome,
			&d.PnL,
			&d.Context,
			&d.AgentName,
			&d.Confidence,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, &d)
	}

	return decisions, rows.Err()
}

// GetLLMDecisionStats returns statistics about LLM decisions
func (db *DB) GetLLMDecisionStats(ctx context.Context, agentName string, since time.Time) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_decisions,
			COUNT(CASE WHEN outcome = 'SUCCESS' THEN 1 END) as successful,
			COUNT(CASE WHEN outcome = 'FAILURE' THEN 1 END) as failed,
			COUNT(CASE WHEN outcome IS NULL THEN 1 END) as pending,
			AVG(CASE WHEN pnl IS NOT NULL THEN pnl END) as avg_pnl,
			SUM(CASE WHEN pnl IS NOT NULL THEN pnl ELSE 0 END) as total_pnl,
			AVG(latency_ms) as avg_latency_ms,
			AVG(tokens_used) as avg_tokens_used,
			AVG(confidence) as avg_confidence
		FROM llm_decisions
		WHERE agent_name = $1 AND created_at >= $2
	`

	var stats map[string]interface{}
	var totalDecisions, successful, failed, pending int
	var avgPnl, totalPnl, avgLatency, avgTokens, avgConfidence *float64

	err := db.pool.QueryRow(ctx, query, agentName, since).Scan(
		&totalDecisions,
		&successful,
		&failed,
		&pending,
		&avgPnl,
		&totalPnl,
		&avgLatency,
		&avgTokens,
		&avgConfidence,
	)
	if err != nil {
		return nil, err
	}

	stats = map[string]interface{}{
		"total_decisions": totalDecisions,
		"successful":      successful,
		"failed":          failed,
		"pending":         pending,
		"success_rate":    float64(successful) / float64(totalDecisions) * 100.0,
	}

	if avgPnl != nil {
		stats["avg_pnl"] = *avgPnl
	}
	if totalPnl != nil {
		stats["total_pnl"] = *totalPnl
	}
	if avgLatency != nil {
		stats["avg_latency_ms"] = *avgLatency
	}
	if avgTokens != nil {
		stats["avg_tokens_used"] = *avgTokens
	}
	if avgConfidence != nil {
		stats["avg_confidence"] = *avgConfidence
	}

	return stats, nil
}

// FindSimilarDecisions finds decisions with similar market conditions (for T185)
// This uses the context JSONB field to find similar situations
func (db *DB) FindSimilarDecisions(ctx context.Context, symbol string, contextJSON []byte, limit int) ([]*LLMDecision, error) {
	// For now, simple approach: find recent decisions for the same symbol
	// TODO: Implement proper similarity search using pgvector embeddings
	query := `
		SELECT
			id, session_id, decision_type, symbol, prompt,
			response, model, tokens_used, latency_ms, outcome, pnl,
			context, agent_name, confidence, created_at
		FROM llm_decisions
		WHERE symbol = $1
		  AND outcome IS NOT NULL
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.pool.Query(ctx, query, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decisions []*LLMDecision
	for rows.Next() {
		var d LLMDecision
		err := rows.Scan(
			&d.ID,
			&d.SessionID,
			&d.DecisionType,
			&d.Symbol,
			&d.Prompt,
			&d.Response,
			&d.Model,
			&d.TokensUsed,
			&d.LatencyMs,
			&d.Outcome,
			&d.PnL,
			&d.Context,
			&d.AgentName,
			&d.Confidence,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, &d)
	}

	return decisions, rows.Err()
}
