package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDecisionNotFound is returned when a decision is not found
var ErrDecisionNotFound = errors.New("decision not found")

// FeedbackRating represents the rating type for decision feedback
type FeedbackRating string

const (
	FeedbackPositive FeedbackRating = "positive"
	FeedbackNegative FeedbackRating = "negative"
)

// Constants for feedback queries
const (
	DefaultFeedbackLimit = 50
	MaxFeedbackLimit     = 200
	DefaultReviewLimit   = 20
	MaxReviewLimit       = 100 // Lower limit for review queries (returns full decision data)
	DefaultTrendDays     = 7   // Number of days for recent trend analysis
	orderByCreatedAtDesc = " ORDER BY created_at DESC"
)

// Common feedback tags that users can select
var CommonFeedbackTags = []string{
	"wrong_direction",
	"bad_timing",
	"risk_too_high",
	"risk_too_low",
	"missed_opportunity",
	"good_entry",
	"good_exit",
	"accurate_prediction",
	"unclear_reasoning",
	"helpful_explanation",
}

// FeedbackRepository handles database operations for decision feedback
type FeedbackRepository struct {
	db *pgxpool.Pool
}

// NewFeedbackRepository creates a new feedback repository
func NewFeedbackRepository(db *pgxpool.Pool) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

// Feedback represents a user's feedback on a decision
type Feedback struct {
	ID           uuid.UUID      `json:"id"`
	DecisionID   uuid.UUID      `json:"decision_id"`
	UserID       *uuid.UUID     `json:"user_id,omitempty"`
	Rating       FeedbackRating `json:"rating"`
	Comment      *string        `json:"comment,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	SessionID    *uuid.UUID     `json:"session_id,omitempty"`
	Symbol       *string        `json:"symbol,omitempty"`
	DecisionType *string        `json:"decision_type,omitempty"`
	AgentName    *string        `json:"agent_name,omitempty"`
}

// CreateFeedbackRequest contains the data needed to create feedback
type CreateFeedbackRequest struct {
	DecisionID uuid.UUID      `json:"decision_id" binding:"required"`
	UserID     *uuid.UUID     `json:"user_id,omitempty"`
	Rating     FeedbackRating `json:"rating" binding:"required,oneof=positive negative"`
	Comment    *string        `json:"comment,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
}

// UpdateFeedbackRequest contains the data to update existing feedback
type UpdateFeedbackRequest struct {
	Rating  *FeedbackRating `json:"rating,omitempty"`
	Comment *string         `json:"comment,omitempty"`
	Tags    []string        `json:"tags,omitempty"`
}

// FeedbackFilter contains filtering options for listing feedback
type FeedbackFilter struct {
	DecisionID   *uuid.UUID
	UserID       *uuid.UUID
	Rating       *FeedbackRating
	AgentName    string
	Symbol       string
	DecisionType string
	FromDate     *time.Time
	ToDate       *time.Time
	Limit        int
	Offset       int
}

// FeedbackStats contains aggregated feedback statistics
type FeedbackStats struct {
	TotalFeedback  int                           `json:"total_feedback"`
	PositiveCount  int                           `json:"positive_count"`
	NegativeCount  int                           `json:"negative_count"`
	PositiveRate   float64                       `json:"positive_rate"`
	ByAgent        map[string]AgentFeedbackStats `json:"by_agent,omitempty"`
	ByDecisionType map[string]int                `json:"by_decision_type,omitempty"`
	BySymbol       map[string]int                `json:"by_symbol,omitempty"`
	TopTags        []TagCount                    `json:"top_tags,omitempty"`
	RecentTrend    []DailyFeedback               `json:"recent_trend,omitempty"`
}

// AgentFeedbackStats contains feedback stats for a specific agent
type AgentFeedbackStats struct {
	PositiveCount int     `json:"positive_count"`
	NegativeCount int     `json:"negative_count"`
	Total         int     `json:"total"`
	PositiveRate  float64 `json:"positive_rate"`
}

// TagCount represents a tag and its count
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// DailyFeedback represents feedback counts for a single day
type DailyFeedback struct {
	Date          string `json:"date"`
	PositiveCount int    `json:"positive_count"`
	NegativeCount int    `json:"negative_count"`
}

// DecisionNeedingReview represents a decision flagged for review
type DecisionNeedingReview struct {
	DecisionID        uuid.UUID `json:"decision_id"`
	AgentName         *string   `json:"agent_name,omitempty"`
	DecisionType      string    `json:"decision_type"`
	Symbol            string    `json:"symbol"`
	Prompt            string    `json:"prompt"`
	Response          string    `json:"response"`
	Confidence        *float64  `json:"confidence,omitempty"`
	Outcome           *string   `json:"outcome,omitempty"`
	OutcomePnL        *float64  `json:"outcome_pnl,omitempty"`
	DecisionCreatedAt time.Time `json:"decision_created_at"`
	FeedbackCount     int       `json:"feedback_count"`
	NegativeCount     int       `json:"negative_count"`
	PositiveCount     int       `json:"positive_count"`
	Comments          []string  `json:"comments,omitempty"`
	AllTags           []string  `json:"all_tags,omitempty"`
}

// CreateFeedback creates a new feedback entry for a decision.
// Uses a transaction to ensure atomicity between decision lookup and feedback creation.
func (r *FeedbackRepository) CreateFeedback(ctx context.Context, req CreateFeedbackRequest) (*Feedback, error) {
	// Start transaction for atomic decision lookup and feedback creation
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is safe to ignore on commit

	// First, get decision details to denormalize
	var sessionID *uuid.UUID
	var symbol, decisionType, agentName *string

	err = tx.QueryRow(ctx, `
		SELECT session_id, symbol, decision_type, agent_name
		FROM llm_decisions
		WHERE id = $1
	`, req.DecisionID).Scan(&sessionID, &symbol, &decisionType, &agentName)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("%w: %s", ErrDecisionNotFound, req.DecisionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch decision: %w", err)
	}

	// Insert feedback
	query := `
		INSERT INTO decision_feedback (
			decision_id, user_id, rating, comment, tags,
			session_id, symbol, decision_type, agent_name
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	var feedback Feedback
	feedback.DecisionID = req.DecisionID
	feedback.UserID = req.UserID
	feedback.Rating = req.Rating
	feedback.Comment = req.Comment
	feedback.Tags = req.Tags
	feedback.SessionID = sessionID
	feedback.Symbol = symbol
	feedback.DecisionType = decisionType
	feedback.AgentName = agentName

	err = tx.QueryRow(ctx, query,
		req.DecisionID, req.UserID, req.Rating, req.Comment, req.Tags,
		sessionID, symbol, decisionType, agentName,
	).Scan(&feedback.ID, &feedback.CreatedAt, &feedback.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create feedback: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &feedback, nil
}

// GetFeedback retrieves a single feedback entry by ID
func (r *FeedbackRepository) GetFeedback(ctx context.Context, id uuid.UUID) (*Feedback, error) {
	query := `
		SELECT
			id, decision_id, user_id, rating, comment, tags,
			created_at, updated_at, session_id, symbol, decision_type, agent_name
		FROM decision_feedback
		WHERE id = $1
	`

	var f Feedback
	err := r.db.QueryRow(ctx, query, id).Scan(
		&f.ID, &f.DecisionID, &f.UserID, &f.Rating, &f.Comment, &f.Tags,
		&f.CreatedAt, &f.UpdatedAt, &f.SessionID, &f.Symbol, &f.DecisionType, &f.AgentName,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &f, nil
}

// GetFeedbackByDecision retrieves all feedback for a specific decision
func (r *FeedbackRepository) GetFeedbackByDecision(ctx context.Context, decisionID uuid.UUID) ([]Feedback, error) {
	query := `
		SELECT
			id, decision_id, user_id, rating, comment, tags,
			created_at, updated_at, session_id, symbol, decision_type, agent_name
		FROM decision_feedback
		WHERE decision_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, decisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedbacks := make([]Feedback, 0)
	for rows.Next() {
		var f Feedback
		err := rows.Scan(
			&f.ID, &f.DecisionID, &f.UserID, &f.Rating, &f.Comment, &f.Tags,
			&f.CreatedAt, &f.UpdatedAt, &f.SessionID, &f.Symbol, &f.DecisionType, &f.AgentName,
		)
		if err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, f)
	}

	return feedbacks, rows.Err()
}

// ListFeedback retrieves feedback with optional filtering
func (r *FeedbackRepository) ListFeedback(ctx context.Context, filter FeedbackFilter) ([]Feedback, error) {
	query := `
		SELECT
			id, decision_id, user_id, rating, comment, tags,
			created_at, updated_at, session_id, symbol, decision_type, agent_name
		FROM decision_feedback
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argPos := 1

	if filter.DecisionID != nil {
		query += fmt.Sprintf(" AND decision_id = $%d", argPos)
		args = append(args, *filter.DecisionID)
		argPos++
	}
	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argPos)
		args = append(args, *filter.UserID)
		argPos++
	}
	if filter.Rating != nil {
		query += fmt.Sprintf(" AND rating = $%d", argPos)
		args = append(args, *filter.Rating)
		argPos++
	}
	if filter.AgentName != "" {
		query += fmt.Sprintf(" AND agent_name = $%d", argPos)
		args = append(args, filter.AgentName)
		argPos++
	}
	if filter.Symbol != "" {
		query += fmt.Sprintf(" AND symbol = $%d", argPos)
		args = append(args, filter.Symbol)
		argPos++
	}
	if filter.DecisionType != "" {
		query += fmt.Sprintf(" AND decision_type = $%d", argPos)
		args = append(args, filter.DecisionType)
		argPos++
	}
	if filter.FromDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argPos)
		args = append(args, *filter.FromDate)
		argPos++
	}
	if filter.ToDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argPos)
		args = append(args, *filter.ToDate)
		argPos++
	}

	query += orderByCreatedAtDesc

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedbacks := make([]Feedback, 0, filter.Limit)
	for rows.Next() {
		var f Feedback
		err := rows.Scan(
			&f.ID, &f.DecisionID, &f.UserID, &f.Rating, &f.Comment, &f.Tags,
			&f.CreatedAt, &f.UpdatedAt, &f.SessionID, &f.Symbol, &f.DecisionType, &f.AgentName,
		)
		if err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, f)
	}

	return feedbacks, rows.Err()
}

// UpdateFeedback updates an existing feedback entry.
// Note: updated_at is automatically set by the database trigger (decision_feedback_updated_at).
func (r *FeedbackRepository) UpdateFeedback(ctx context.Context, id uuid.UUID, req UpdateFeedbackRequest) (*Feedback, error) {
	query := `UPDATE decision_feedback SET `
	args := make([]interface{}, 0)
	argPos := 1
	updates := make([]string, 0)

	if req.Rating != nil {
		updates = append(updates, fmt.Sprintf("rating = $%d", argPos))
		args = append(args, *req.Rating)
		argPos++
	}
	if req.Comment != nil {
		updates = append(updates, fmt.Sprintf("comment = $%d", argPos))
		args = append(args, *req.Comment)
		argPos++
	}
	if req.Tags != nil {
		updates = append(updates, fmt.Sprintf("tags = $%d", argPos))
		args = append(args, req.Tags)
		argPos++
	}

	if len(updates) == 0 {
		return r.GetFeedback(ctx, id)
	}

	query += updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += fmt.Sprintf(" WHERE id = $%d RETURNING id, decision_id, user_id, rating, comment, tags, created_at, updated_at, session_id, symbol, decision_type, agent_name", argPos)
	args = append(args, id)

	var f Feedback
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&f.ID, &f.DecisionID, &f.UserID, &f.Rating, &f.Comment, &f.Tags,
		&f.CreatedAt, &f.UpdatedAt, &f.SessionID, &f.Symbol, &f.DecisionType, &f.AgentName,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &f, nil
}

// DeleteFeedback deletes a feedback entry
func (r *FeedbackRepository) DeleteFeedback(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM decision_feedback WHERE id = $1", id)
	return err
}

// GetFeedbackStats retrieves aggregated feedback statistics
func (r *FeedbackRepository) GetFeedbackStats(ctx context.Context, filter FeedbackFilter) (*FeedbackStats, error) {
	// Overall stats
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE rating = 'positive') as positive_count,
			COUNT(*) FILTER (WHERE rating = 'negative') as negative_count
		FROM decision_feedback
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argPos := 1

	if filter.AgentName != "" {
		query += fmt.Sprintf(" AND agent_name = $%d", argPos)
		args = append(args, filter.AgentName)
		argPos++
	}
	if filter.Symbol != "" {
		query += fmt.Sprintf(" AND symbol = $%d", argPos)
		args = append(args, filter.Symbol)
		argPos++
	}
	if filter.FromDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argPos)
		args = append(args, *filter.FromDate)
		argPos++
	}
	if filter.ToDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argPos)
		args = append(args, *filter.ToDate)
	}

	var stats FeedbackStats
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&stats.TotalFeedback, &stats.PositiveCount, &stats.NegativeCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get overall stats: %w", err)
	}

	if stats.TotalFeedback > 0 {
		stats.PositiveRate = float64(stats.PositiveCount) / float64(stats.TotalFeedback) * 100
	}

	// Stats by agent
	stats.ByAgent, err = r.getStatsByAgent(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Stats by decision type
	stats.ByDecisionType, err = r.getCountsByField(ctx, "decision_type", filter)
	if err != nil {
		return nil, err
	}

	// Stats by symbol
	stats.BySymbol, err = r.getCountsByField(ctx, "symbol", filter)
	if err != nil {
		return nil, err
	}

	// Top tags
	stats.TopTags, err = r.getTopTags(ctx, filter, 10)
	if err != nil {
		return nil, err
	}

	// Recent trend
	stats.RecentTrend, err = r.getDailyTrend(ctx, DefaultTrendDays)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// getStatsByAgent returns feedback stats grouped by agent
func (r *FeedbackRepository) getStatsByAgent(ctx context.Context, filter FeedbackFilter) (map[string]AgentFeedbackStats, error) {
	query := `
		SELECT
			agent_name,
			COUNT(*) FILTER (WHERE rating = 'positive') as positive,
			COUNT(*) FILTER (WHERE rating = 'negative') as negative,
			COUNT(*) as total
		FROM decision_feedback
		WHERE agent_name IS NOT NULL
		GROUP BY agent_name
		ORDER BY total DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]AgentFeedbackStats)
	for rows.Next() {
		var agentName string
		var stats AgentFeedbackStats
		if err := rows.Scan(&agentName, &stats.PositiveCount, &stats.NegativeCount, &stats.Total); err != nil {
			return nil, err
		}
		if stats.Total > 0 {
			stats.PositiveRate = float64(stats.PositiveCount) / float64(stats.Total) * 100
		}
		result[agentName] = stats
	}

	return result, rows.Err()
}

// allowedFeedbackFields defines the whitelist of fields for GROUP BY
var allowedFeedbackFields = map[string]bool{
	"decision_type": true,
	"symbol":        true,
	"agent_name":    true,
}

// getCountsByField gets feedback count by a specific field
func (r *FeedbackRepository) getCountsByField(ctx context.Context, field string, filter FeedbackFilter) (map[string]int, error) {
	if !allowedFeedbackFields[field] {
		return nil, fmt.Errorf("invalid field name: %s", field)
	}

	query := fmt.Sprintf(`
		SELECT %s, COUNT(*)
		FROM decision_feedback
		WHERE %s IS NOT NULL
		GROUP BY %s
		ORDER BY COUNT(*) DESC
		LIMIT 20
	`, field, field, field)

	rows, err := r.db.Query(ctx, query)
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

// getTopTags returns the most frequently used tags
func (r *FeedbackRepository) getTopTags(ctx context.Context, filter FeedbackFilter, limit int) ([]TagCount, error) {
	query := `
		SELECT tag, COUNT(*) as count
		FROM decision_feedback, unnest(tags) as tag
		GROUP BY tag
		ORDER BY count DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]TagCount, 0, limit)
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tc)
	}

	return tags, rows.Err()
}

// getDailyTrend returns daily feedback counts for the past N days
func (r *FeedbackRepository) getDailyTrend(ctx context.Context, days int) ([]DailyFeedback, error) {
	query := `
		SELECT
			DATE(created_at) as date,
			COUNT(*) FILTER (WHERE rating = 'positive') as positive,
			COUNT(*) FILTER (WHERE rating = 'negative') as negative
		FROM decision_feedback
		WHERE created_at >= NOW() - INTERVAL '1 day' * $1
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`

	rows, err := r.db.Query(ctx, query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trend := make([]DailyFeedback, 0, days)
	for rows.Next() {
		var df DailyFeedback
		var date time.Time
		if err := rows.Scan(&date, &df.PositiveCount, &df.NegativeCount); err != nil {
			return nil, err
		}
		df.Date = date.Format("2006-01-02")
		trend = append(trend, df)
	}

	return trend, rows.Err()
}

// GetDecisionsNeedingReview returns decisions with multiple negative ratings
func (r *FeedbackRepository) GetDecisionsNeedingReview(ctx context.Context, limit int) ([]DecisionNeedingReview, error) {
	if limit <= 0 {
		limit = DefaultReviewLimit
	}
	if limit > MaxReviewLimit {
		limit = MaxReviewLimit
	}

	query := `
		SELECT
			d.id as decision_id,
			d.agent_name,
			d.decision_type,
			d.symbol,
			d.prompt,
			d.response,
			d.confidence,
			d.outcome,
			d.outcome_pnl,
			d.created_at as decision_created_at,
			COUNT(f.id) as feedback_count,
			COUNT(*) FILTER (WHERE f.rating = 'negative') as negative_count,
			COUNT(*) FILTER (WHERE f.rating = 'positive') as positive_count,
			ARRAY_AGG(f.comment) FILTER (WHERE f.comment IS NOT NULL) as comments,
			ARRAY_AGG(DISTINCT tag) FILTER (WHERE tag IS NOT NULL) as all_tags
		FROM llm_decisions d
		JOIN decision_feedback f ON d.id = f.decision_id
		LEFT JOIN LATERAL unnest(f.tags) AS tag ON true
		GROUP BY d.id, d.agent_name, d.decision_type, d.symbol, d.prompt, d.response,
				 d.confidence, d.outcome, d.outcome_pnl, d.created_at
		HAVING COUNT(*) FILTER (WHERE f.rating = 'negative') >= 2
		ORDER BY negative_count DESC, d.created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := make([]DecisionNeedingReview, 0, limit)
	for rows.Next() {
		var d DecisionNeedingReview
		if err := rows.Scan(
			&d.DecisionID, &d.AgentName, &d.DecisionType, &d.Symbol, &d.Prompt, &d.Response,
			&d.Confidence, &d.Outcome, &d.OutcomePnL, &d.DecisionCreatedAt,
			&d.FeedbackCount, &d.NegativeCount, &d.PositiveCount, &d.Comments, &d.AllTags,
		); err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}

	return decisions, rows.Err()
}

// RefreshStatsView refreshes the materialized view for feedback statistics
func (r *FeedbackRepository) RefreshStatsView(ctx context.Context) error {
	_, err := r.db.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY decision_feedback_stats")
	return err
}
