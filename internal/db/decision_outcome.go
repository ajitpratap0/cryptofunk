package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DecisionWithOutcome represents a decision joined with its outcome data.
type DecisionWithOutcome struct {
	LLMDecision
	OrderID    *uuid.UUID `json:"order_id,omitempty"`
	PositionID *uuid.UUID `json:"position_id,omitempty"`
	OrderSide  *string    `json:"order_side,omitempty"`
	OrderPrice *float64   `json:"order_price,omitempty"`
	PosPnl     *float64   `json:"position_pnl,omitempty"`
}

// AgentAnalytics holds per-agent decision statistics.
type AgentAnalytics struct {
	AgentName      string  `json:"agent_name"`
	TotalDecisions int     `json:"total_decisions"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	Pending        int     `json:"pending"`
	WinRate        float64 `json:"win_rate"`
	AvgConfidence  float64 `json:"avg_confidence"`
	AvgPnl         float64 `json:"avg_pnl"`
	TotalPnl       float64 `json:"total_pnl"`
}

// ConfidenceBucket holds calibration data for a confidence range.
type ConfidenceBucket struct {
	BucketLow  float64 `json:"bucket_low"`
	BucketHigh float64 `json:"bucket_high"`
	Count      int     `json:"count"`
	WinRate    float64 `json:"win_rate"`
	AvgPnl     float64 `json:"avg_pnl"`
}

// DecisionAnalytics holds overall decision analytics.
type DecisionAnalytics struct {
	TotalDecisions        int                `json:"total_decisions"`
	ResolvedDecisions     int                `json:"resolved_decisions"`
	OverallWinRate        float64            `json:"overall_win_rate"`
	OverallAvgPnl         float64            `json:"overall_avg_pnl"`
	ByAgent               []AgentAnalytics   `json:"by_agent"`
	ConfidenceCalibration []ConfidenceBucket `json:"confidence_calibration"`
}

// LinkDecisionToOrder links a decision to an order via the decision_id column.
func (db *DB) LinkDecisionToOrder(ctx context.Context, decisionID uuid.UUID, orderID uuid.UUID) error {
	query := `UPDATE orders SET decision_id = $1 WHERE id = $2`
	result, err := db.pool.Exec(ctx, query, decisionID, orderID)
	if err != nil {
		return fmt.Errorf("failed to link decision to order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found: %s", orderID)
	}
	return nil
}

// LinkDecisionToPosition links a decision to a position.
func (db *DB) LinkDecisionToPosition(ctx context.Context, decisionID uuid.UUID, positionID uuid.UUID) error {
	query := `UPDATE positions SET decision_id = $1 WHERE id = $2`
	result, err := db.pool.Exec(ctx, query, decisionID, positionID)
	if err != nil {
		return fmt.Errorf("failed to link decision to position: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("position not found: %s", positionID)
	}
	return nil
}

// UpdateDecisionOutcome updates the outcome and P&L of a decision.
func (db *DB) UpdateDecisionOutcome(ctx context.Context, decisionID uuid.UUID, outcome string, pnl float64) error {
	query := `UPDATE llm_decisions SET outcome = $2, outcome_pnl = $3 WHERE id = $1`
	result, err := db.pool.Exec(ctx, query, decisionID, outcome, pnl)
	if err != nil {
		return fmt.Errorf("failed to update decision outcome: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("decision not found: %s", decisionID)
	}
	return nil
}

// GetDecisionsWithOutcomes returns recent decisions joined with their linked orders/positions.
func (db *DB) GetDecisionsWithOutcomes(ctx context.Context, limit int) ([]DecisionWithOutcome, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT
			d.id, d.session_id, d.decision_type, d.symbol, d.prompt,
			d.response, d.model, d.tokens_used, d.latency_ms, d.outcome, d.outcome_pnl,
			d.context, d.agent_name, d.confidence, d.created_at,
			o.id AS order_id, o.side AS order_side, o.price AS order_price,
			p.id AS position_id, p.realized_pnl AS position_pnl
		FROM llm_decisions d
		LEFT JOIN orders o ON o.decision_id = d.id
		LEFT JOIN positions p ON p.decision_id = d.id
		ORDER BY d.created_at DESC
		LIMIT $1
	`

	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get decisions with outcomes: %w", err)
	}
	defer rows.Close()

	var results []DecisionWithOutcome
	for rows.Next() {
		var dwo DecisionWithOutcome
		var orderSide *string
		err := rows.Scan(
			&dwo.ID, &dwo.SessionID, &dwo.DecisionType, &dwo.Symbol, &dwo.Prompt,
			&dwo.Response, &dwo.Model, &dwo.TokensUsed, &dwo.LatencyMs, &dwo.Outcome, &dwo.PnL,
			&dwo.Context, &dwo.AgentName, &dwo.Confidence, &dwo.CreatedAt,
			&dwo.OrderID, &orderSide, &dwo.OrderPrice,
			&dwo.PositionID, &dwo.PosPnl,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan decision with outcome: %w", err)
		}
		dwo.OrderSide = orderSide
		results = append(results, dwo)
	}
	return results, rows.Err()
}

// GetUnresolvedDecisions returns decisions linked to orders/positions that have no outcome yet.
func (db *DB) GetUnresolvedDecisions(ctx context.Context) ([]LLMDecision, error) {
	query := `
		SELECT
			d.id, d.session_id, d.decision_type, d.symbol, d.prompt,
			d.response, d.model, d.tokens_used, d.latency_ms, d.outcome, d.outcome_pnl,
			d.context, d.agent_name, d.confidence, d.created_at
		FROM llm_decisions d
		WHERE d.outcome IS NULL
		  AND (
		    EXISTS (SELECT 1 FROM orders o WHERE o.decision_id = d.id)
		    OR EXISTS (SELECT 1 FROM positions p WHERE p.decision_id = d.id)
		  )
		ORDER BY d.created_at DESC
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get unresolved decisions: %w", err)
	}
	defer rows.Close()

	var decisions []LLMDecision
	for rows.Next() {
		var d LLMDecision
		err := rows.Scan(
			&d.ID, &d.SessionID, &d.DecisionType, &d.Symbol, &d.Prompt,
			&d.Response, &d.Model, &d.TokensUsed, &d.LatencyMs, &d.Outcome, &d.PnL,
			&d.Context, &d.AgentName, &d.Confidence, &d.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unresolved decision: %w", err)
		}
		decisions = append(decisions, d)
	}
	return decisions, rows.Err()
}

// GetDecisionAnalytics returns aggregated analytics about decision accuracy.
func (db *DB) GetDecisionAnalytics(ctx context.Context, since time.Time) (*DecisionAnalytics, error) {
	// Per-agent stats
	agentQuery := `
		SELECT
			agent_name,
			COUNT(*) AS total,
			COUNT(CASE WHEN outcome = 'SUCCESS' THEN 1 END) AS wins,
			COUNT(CASE WHEN outcome = 'FAILURE' THEN 1 END) AS losses,
			COUNT(CASE WHEN outcome IS NULL THEN 1 END) AS pending,
			AVG(confidence) AS avg_confidence,
			AVG(CASE WHEN outcome_pnl IS NOT NULL THEN outcome_pnl ELSE 0 END) AS avg_pnl,
			SUM(CASE WHEN outcome_pnl IS NOT NULL THEN outcome_pnl ELSE 0 END) AS total_pnl
		FROM llm_decisions
		WHERE created_at >= $1
		GROUP BY agent_name
		ORDER BY total DESC
	`

	rows, err := db.pool.Query(ctx, agentQuery, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent analytics: %w", err)
	}
	defer rows.Close()

	analytics := &DecisionAnalytics{}
	var totalAll, resolvedAll, winsAll int

	for rows.Next() {
		var a AgentAnalytics
		err := rows.Scan(&a.AgentName, &a.TotalDecisions, &a.Wins, &a.Losses, &a.Pending, &a.AvgConfidence, &a.AvgPnl, &a.TotalPnl)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent analytics: %w", err)
		}
		resolved := a.Wins + a.Losses
		if resolved > 0 {
			a.WinRate = float64(a.Wins) / float64(resolved) * 100
		}
		analytics.ByAgent = append(analytics.ByAgent, a)
		totalAll += a.TotalDecisions
		resolvedAll += resolved
		winsAll += a.Wins
	}

	analytics.TotalDecisions = totalAll
	analytics.ResolvedDecisions = resolvedAll
	if resolvedAll > 0 {
		analytics.OverallWinRate = float64(winsAll) / float64(resolvedAll) * 100
	}

	// Confidence calibration (buckets of 0.1)
	calQuery := `
		SELECT
			FLOOR(confidence * 10) / 10 AS bucket_low,
			COUNT(*) AS cnt,
			COUNT(CASE WHEN outcome = 'SUCCESS' THEN 1 END) AS wins,
			AVG(CASE WHEN outcome_pnl IS NOT NULL THEN outcome_pnl ELSE 0 END) AS avg_pnl
		FROM llm_decisions
		WHERE created_at >= $1 AND outcome IS NOT NULL
		GROUP BY bucket_low
		ORDER BY bucket_low
	`

	calRows, err := db.pool.Query(ctx, calQuery, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get calibration: %w", err)
	}
	defer calRows.Close()

	for calRows.Next() {
		var b ConfidenceBucket
		var wins int
		err := calRows.Scan(&b.BucketLow, &b.Count, &wins, &b.AvgPnl)
		if err != nil {
			return nil, fmt.Errorf("failed to scan calibration bucket: %w", err)
		}
		b.BucketHigh = b.BucketLow + 0.1
		if b.Count > 0 {
			b.WinRate = float64(wins) / float64(b.Count) * 100
		}
		analytics.ConfidenceCalibration = append(analytics.ConfidenceCalibration, b)
	}

	// Compute overall avg pnl
	if resolvedAll > 0 {
		var totalPnlAll float64
		for _, a := range analytics.ByAgent {
			totalPnlAll += a.TotalPnl
		}
		analytics.OverallAvgPnl = totalPnlAll / float64(resolvedAll)
	}

	return analytics, nil
}

// ResolveDecisionsFromPositions finds unresolved decisions linked to closed positions and resolves them.
func (db *DB) ResolveDecisionsFromPositions(ctx context.Context) (int, error) {
	query := `
		UPDATE llm_decisions d
		SET outcome = CASE WHEN p.realized_pnl >= 0 THEN 'SUCCESS' ELSE 'FAILURE' END,
		    outcome_pnl = p.realized_pnl
		FROM positions p
		WHERE p.decision_id = d.id
		  AND d.outcome IS NULL
		  AND p.exit_time IS NOT NULL
		  AND p.realized_pnl IS NOT NULL
	`
	result, err := db.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve decisions from positions: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// GetUnresolvedPolymarketPredictions returns predictions that haven't been resolved yet.
func (db *DB) GetUnresolvedPolymarketPredictions(ctx context.Context) ([]*PolymarketPrediction, error) {
	query := `
		SELECT id, market_id, predicted_prob, market_price, edge, confidence, category, reasoning, outcome, resolved, pnl, created_at, resolved_at
		FROM polymarket_predictions
		WHERE resolved = FALSE
		ORDER BY created_at ASC
	`
	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get unresolved predictions: %w", err)
	}
	defer rows.Close()

	var predictions []*PolymarketPrediction
	for rows.Next() {
		var p PolymarketPrediction
		if err := rows.Scan(&p.ID, &p.MarketID, &p.PredictedProb, &p.MarketPrice, &p.Edge, &p.Confidence, &p.Category, &p.Reasoning, &p.Outcome, &p.Resolved, &p.Pnl, &p.CreatedAt, &p.ResolvedAt); err != nil {
			return nil, fmt.Errorf("failed to scan prediction: %w", err)
		}
		predictions = append(predictions, &p)
	}
	return predictions, rows.Err()
}
