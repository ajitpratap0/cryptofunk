package llm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// DecisionTracker tracks LLM decisions to the database for learning and explainability
type DecisionTracker struct {
	database *db.DB
}

// NewDecisionTracker creates a new decision tracker
func NewDecisionTracker(database *db.DB) *DecisionTracker {
	return &DecisionTracker{
		database: database,
	}
}

// TrackDecision records an LLM decision in the database
func (dt *DecisionTracker) TrackDecision(
	ctx context.Context,
	agentName string,
	decisionType string,
	symbol string,
	prompt string,
	response string,
	model string,
	tokensUsed int,
	latencyMs int,
	confidence float64,
	contextData map[string]interface{},
	sessionID *uuid.UUID,
) (uuid.UUID, error) {
	// Marshal context to JSONB
	contextJSON, err := json.Marshal(contextData)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal context, continuing without it")
		contextJSON = []byte("{}")
	}

	decision := &db.LLMDecision{
		ID:           uuid.New(),
		SessionID:    sessionID,
		DecisionType: decisionType,
		Symbol:       symbol,
		Prompt:       prompt,
		Response:     response,
		Model:        model,
		TokensUsed:   tokensUsed,
		LatencyMs:    latencyMs,
		Context:      contextJSON,
		AgentName:    agentName,
		Confidence:   confidence,
		CreatedAt:    time.Now(),
	}

	err = dt.database.InsertLLMDecision(ctx, decision)
	if err != nil {
		log.Error().Err(err).Msg("Failed to track LLM decision")
		return uuid.Nil, err
	}

	log.Debug().
		Str("agent", agentName).
		Str("symbol", symbol).
		Str("decision_type", decisionType).
		Float64("confidence", confidence).
		Int("tokens", tokensUsed).
		Int("latency_ms", latencyMs).
		Msg("LLM decision tracked")

	return decision.ID, nil
}

// UpdateDecisionOutcome updates the outcome and P&L of a tracked decision
func (dt *DecisionTracker) UpdateDecisionOutcome(
	ctx context.Context,
	decisionID uuid.UUID,
	outcome string, // "SUCCESS", "FAILURE", "PENDING"
	pnl float64,
) error {
	err := dt.database.UpdateLLMDecisionOutcome(ctx, decisionID, outcome, pnl)
	if err != nil {
		log.Error().Err(err).Str("decision_id", decisionID.String()).Msg("Failed to update decision outcome")
		return err
	}

	log.Info().
		Str("decision_id", decisionID.String()).
		Str("outcome", outcome).
		Float64("pnl", pnl).
		Msg("Decision outcome updated")

	return nil
}

// GetRecentDecisions retrieves recent decisions for an agent
func (dt *DecisionTracker) GetRecentDecisions(ctx context.Context, agentName string, limit int) ([]*db.LLMDecision, error) {
	return dt.database.GetLLMDecisionsByAgent(ctx, agentName, limit)
}

// GetSuccessfulDecisions retrieves successful decisions for learning
func (dt *DecisionTracker) GetSuccessfulDecisions(ctx context.Context, agentName string, limit int) ([]*db.LLMDecision, error) {
	return dt.database.GetSuccessfulLLMDecisions(ctx, agentName, limit)
}

// GetDecisionStats retrieves statistics about agent decisions
func (dt *DecisionTracker) GetDecisionStats(ctx context.Context, agentName string, since time.Time) (map[string]interface{}, error) {
	return dt.database.GetLLMDecisionStats(ctx, agentName, since)
}

// FindSimilarDecisions finds past decisions with similar market conditions
func (dt *DecisionTracker) FindSimilarDecisions(ctx context.Context, symbol string, contextData map[string]interface{}, limit int) ([]*db.LLMDecision, error) {
	contextJSON, err := json.Marshal(contextData)
	if err != nil {
		return nil, err
	}

	return dt.database.FindSimilarDecisions(ctx, symbol, contextJSON, limit)
}

// TrackSignalDecision is a convenience method for tracking signal decisions
func (dt *DecisionTracker) TrackSignalDecision(
	ctx context.Context,
	agentName string,
	signal *Signal,
	prompt string,
	response string,
	model string,
	tokensUsed int,
	latencyMs int,
	marketContext MarketContext,
	sessionID *uuid.UUID,
) (uuid.UUID, error) {
	// Build context data
	contextData := map[string]interface{}{
		"current_price":    marketContext.CurrentPrice,
		"price_change_24h": marketContext.PriceChange24h,
		"volume_24h":       marketContext.Volume24h,
		"indicators":       marketContext.Indicators,
		"timestamp":        marketContext.Timestamp,
	}

	return dt.TrackDecision(
		ctx,
		agentName,
		"signal",
		signal.Symbol,
		prompt,
		response,
		model,
		tokensUsed,
		latencyMs,
		signal.Confidence,
		contextData,
		sessionID,
	)
}

// TrackRiskDecision is a convenience method for tracking risk assessment decisions
func (dt *DecisionTracker) TrackRiskDecision(
	ctx context.Context,
	agentName string,
	symbol string,
	approved bool,
	prompt string,
	response string,
	model string,
	tokensUsed int,
	latencyMs int,
	confidence float64,
	contextData map[string]interface{},
	sessionID *uuid.UUID,
) (uuid.UUID, error) {
	decisionType := "risk_approval"
	if !approved {
		decisionType = "risk_veto"
	}

	return dt.TrackDecision(
		ctx,
		agentName,
		decisionType,
		symbol,
		prompt,
		response,
		model,
		tokensUsed,
		latencyMs,
		confidence,
		contextData,
		sessionID,
	)
}
