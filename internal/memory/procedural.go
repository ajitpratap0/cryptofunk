//nolint:goconst // SQL fragments used in query building
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// PolicyType represents the type of policy stored in procedural memory
type PolicyType string

const (
	// PolicyEntry defines when and how to enter a position
	PolicyEntry PolicyType = "entry"

	// PolicyExit defines when and how to exit a position
	PolicyExit PolicyType = "exit"

	// PolicySizing defines how to size positions
	PolicySizing PolicyType = "sizing"

	// PolicyRisk defines risk management rules
	PolicyRisk PolicyType = "risk"

	// PolicyHedging defines hedging strategies
	PolicyHedging PolicyType = "hedging"

	// PolicyRebalancing defines portfolio rebalancing rules
	PolicyRebalancing PolicyType = "rebalancing"
)

// SkillType represents types of agent skills
type SkillType string

const (
	// SkillTechnicalAnalysis represents technical analysis capability
	SkillTechnicalAnalysis SkillType = "technical_analysis"

	// SkillOrderBookAnalysis represents order book analysis capability
	SkillOrderBookAnalysis SkillType = "orderbook_analysis"

	// SkillSentimentAnalysis represents sentiment analysis capability
	SkillSentimentAnalysis SkillType = "sentiment_analysis"

	// SkillTrendFollowing represents trend following strategy
	SkillTrendFollowing SkillType = "trend_following"

	// SkillMeanReversion represents mean reversion strategy
	SkillMeanReversion SkillType = "mean_reversion"

	// SkillRiskManagement represents risk management capability
	SkillRiskManagement SkillType = "risk_management"
)

// Policy represents a learned trading policy or rule
type Policy struct {
	ID uuid.UUID `json:"id"`

	// Policy metadata
	Type        PolicyType `json:"type"`
	Name        string     `json:"name"`        // Human-readable name
	Description string     `json:"description"` // What this policy does

	// Policy definition
	Conditions []byte `json:"conditions"` // JSONB - when to apply this policy
	Actions    []byte `json:"actions"`    // JSONB - what actions to take
	Parameters []byte `json:"parameters"` // JSONB - configurable parameters

	// Performance tracking
	TimesApplied int     `json:"times_applied"` // How many times this policy was used
	SuccessCount int     `json:"success_count"` // How many times it succeeded
	FailureCount int     `json:"failure_count"` // How many times it failed
	AvgPnL       float64 `json:"avg_pnl"`       // Average P&L when applied
	TotalPnL     float64 `json:"total_pnl"`     // Cumulative P&L
	Sharpe       float64 `json:"sharpe"`        // Sharpe ratio
	MaxDrawdown  float64 `json:"max_drawdown"`  // Maximum drawdown
	WinRate      float64 `json:"win_rate"`      // Win rate (0.0 to 1.0)

	// Learning metadata
	AgentName   string     `json:"agent_name"`   // Which agent learned this policy
	Symbol      *string    `json:"symbol"`       // Associated symbol (if specific)
	LearnedFrom string     `json:"learned_from"` // Source: "backtest", "live_trading", "manual"
	SourceID    *uuid.UUID `json:"source_id"`    // ID of source (backtest ID, session ID, etc.)
	Confidence  float64    `json:"confidence"`   // Confidence in this policy (0.0 to 1.0)
	IsActive    bool       `json:"is_active"`    // Whether this policy is currently active
	Priority    int        `json:"priority"`     // Priority when multiple policies match (higher = more priority)

	// Temporal
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastApplied  *time.Time `json:"last_applied"`
	LastModified *time.Time `json:"last_modified"`
}

// SuccessRate returns the success rate of this policy
func (p *Policy) SuccessRate() float64 {
	total := p.SuccessCount + p.FailureCount
	if total == 0 {
		return 0.0
	}
	return float64(p.SuccessCount) / float64(total)
}

// IsPerforming checks if the policy is performing well
func (p *Policy) IsPerforming() bool {
	// Need at least 5 applications to judge
	if p.TimesApplied < 5 {
		return true // Give it a chance
	}

	// Check success rate (should be > 50%)
	if p.SuccessRate() < 0.5 {
		return false
	}

	// Check if profitable
	if p.AvgPnL < 0 {
		return false
	}

	return true
}

// Skill represents an agent's learned capability or skill
type Skill struct {
	ID uuid.UUID `json:"id"`

	// Skill metadata
	Type        SkillType `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`

	// Skill definition
	Implementation []byte `json:"implementation"` // JSONB - how to execute this skill
	Parameters     []byte `json:"parameters"`     // JSONB - configurable parameters
	Prerequisites  []byte `json:"prerequisites"`  // JSONB - required conditions/resources

	// Performance tracking
	TimesUsed    int     `json:"times_used"`    // How many times this skill was used
	SuccessCount int     `json:"success_count"` // Successful executions
	FailureCount int     `json:"failure_count"` // Failed executions
	AvgDuration  float64 `json:"avg_duration"`  // Average execution duration (ms)
	AvgAccuracy  float64 `json:"avg_accuracy"`  // Average accuracy (0.0 to 1.0)

	// Learning metadata
	AgentName   string     `json:"agent_name"`   // Which agent has this skill
	LearnedFrom string     `json:"learned_from"` // Source: "training", "observation", "manual"
	SourceID    *uuid.UUID `json:"source_id"`    // ID of source
	Proficiency float64    `json:"proficiency"`  // Proficiency level (0.0 to 1.0)
	IsActive    bool       `json:"is_active"`    // Whether this skill is active

	// Temporal
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastUsed  *time.Time `json:"last_used"`
}

// SkillSuccessRate returns the success rate of this skill
func (s *Skill) SkillSuccessRate() float64 {
	total := s.SuccessCount + s.FailureCount
	if total == 0 {
		return 0.0
	}
	return float64(s.SuccessCount) / float64(total)
}

// IsProficient checks if the agent is proficient in this skill
func (s *Skill) IsProficient() bool {
	if s.TimesUsed < 10 {
		return true // Still learning
	}

	// Check success rate
	if s.SkillSuccessRate() < 0.7 {
		return false
	}

	// Check proficiency level
	if s.Proficiency < 0.6 {
		return false
	}

	return true
}

// ProceduralMemory manages policies and skills
type ProceduralMemory struct {
	pool *pgxpool.Pool
}

// NewProceduralMemory creates a new procedural memory instance
func NewProceduralMemory(pool *pgxpool.Pool) *ProceduralMemory {
	return &ProceduralMemory{
		pool: pool,
	}
}

// StorePolicy stores a policy in procedural memory
func (pm *ProceduralMemory) StorePolicy(ctx context.Context, policy *Policy) error {
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	policy.UpdatedAt = time.Now()

	query := `
		INSERT INTO procedural_memory_policies (
			id, type, name, description, conditions, actions, parameters,
			times_applied, success_count, failure_count, avg_pnl, total_pnl,
			sharpe, max_drawdown, win_rate,
			agent_name, symbol, learned_from, source_id, confidence,
			is_active, priority,
			created_at, updated_at, last_applied, last_modified
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15,
			$16, $17, $18, $19, $20,
			$21, $22,
			$23, $24, $25, $26
		)
		ON CONFLICT (id) DO UPDATE SET
			times_applied = EXCLUDED.times_applied,
			success_count = EXCLUDED.success_count,
			failure_count = EXCLUDED.failure_count,
			avg_pnl = EXCLUDED.avg_pnl,
			total_pnl = EXCLUDED.total_pnl,
			sharpe = EXCLUDED.sharpe,
			max_drawdown = EXCLUDED.max_drawdown,
			win_rate = EXCLUDED.win_rate,
			confidence = EXCLUDED.confidence,
			is_active = EXCLUDED.is_active,
			priority = EXCLUDED.priority,
			updated_at = EXCLUDED.updated_at,
			last_applied = EXCLUDED.last_applied,
			last_modified = EXCLUDED.last_modified
	`

	_, err := pm.pool.Exec(
		ctx,
		query,
		policy.ID,
		policy.Type,
		policy.Name,
		policy.Description,
		policy.Conditions,
		policy.Actions,
		policy.Parameters,
		policy.TimesApplied,
		policy.SuccessCount,
		policy.FailureCount,
		policy.AvgPnL,
		policy.TotalPnL,
		policy.Sharpe,
		policy.MaxDrawdown,
		policy.WinRate,
		policy.AgentName,
		policy.Symbol,
		policy.LearnedFrom,
		policy.SourceID,
		policy.Confidence,
		policy.IsActive,
		policy.Priority,
		policy.CreatedAt,
		policy.UpdatedAt,
		policy.LastApplied,
		policy.LastModified,
	)

	if err != nil {
		return fmt.Errorf("failed to store policy: %w", err)
	}

	log.Debug().
		Str("id", policy.ID.String()).
		Str("type", string(policy.Type)).
		Str("name", policy.Name).
		Msg("Stored policy in procedural memory")

	return nil
}

// GetPoliciesByType retrieves policies of a specific type
func (pm *ProceduralMemory) GetPoliciesByType(ctx context.Context, policyType PolicyType, activeOnly bool) ([]*Policy, error) {
	query := `
		SELECT
			id, type, name, description, conditions, actions, parameters,
			times_applied, success_count, failure_count, avg_pnl, total_pnl,
			sharpe, max_drawdown, win_rate,
			agent_name, symbol, learned_from, source_id, confidence,
			is_active, priority,
			created_at, updated_at, last_applied, last_modified
		FROM procedural_memory_policies
		WHERE type = $1
	`

	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY priority DESC, confidence DESC, created_at DESC"

	rows, err := pm.pool.Query(ctx, query, policyType)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies: %w", err)
	}
	defer rows.Close()

	return pm.scanPolicies(rows)
}

// GetPoliciesByAgent retrieves policies for a specific agent
func (pm *ProceduralMemory) GetPoliciesByAgent(ctx context.Context, agentName string, activeOnly bool) ([]*Policy, error) {
	query := `
		SELECT
			id, type, name, description, conditions, actions, parameters,
			times_applied, success_count, failure_count, avg_pnl, total_pnl,
			sharpe, max_drawdown, win_rate,
			agent_name, symbol, learned_from, source_id, confidence,
			is_active, priority,
			created_at, updated_at, last_applied, last_modified
		FROM procedural_memory_policies
		WHERE agent_name = $1
	`

	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY priority DESC, created_at DESC"

	rows, err := pm.pool.Query(ctx, query, agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies by agent: %w", err)
	}
	defer rows.Close()

	return pm.scanPolicies(rows)
}

// GetBestPolicies retrieves the best performing policies
func (pm *ProceduralMemory) GetBestPolicies(ctx context.Context, limit int) ([]*Policy, error) {
	query := `
		SELECT
			id, type, name, description, conditions, actions, parameters,
			times_applied, success_count, failure_count, avg_pnl, total_pnl,
			sharpe, max_drawdown, win_rate,
			agent_name, symbol, learned_from, source_id, confidence,
			is_active, priority,
			created_at, updated_at, last_applied, last_modified
		FROM procedural_memory_policies
		WHERE is_active = true
		  AND times_applied >= 5
		ORDER BY sharpe DESC, avg_pnl DESC
		LIMIT $1
	`

	rows, err := pm.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query best policies: %w", err)
	}
	defer rows.Close()

	return pm.scanPolicies(rows)
}

// RecordPolicyApplication records that a policy was applied
func (pm *ProceduralMemory) RecordPolicyApplication(ctx context.Context, id uuid.UUID, success bool, pnl float64) error {
	now := time.Now()

	var query string
	if success {
		query = `
			UPDATE procedural_memory_policies
			SET times_applied = times_applied + 1,
			    success_count = success_count + 1,
			    total_pnl = total_pnl + $2,
			    avg_pnl = (total_pnl + $2) / (times_applied + 1),
			    win_rate = (success_count + 1)::float / (times_applied + 1),
			    last_applied = $3,
			    updated_at = $3
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE procedural_memory_policies
			SET times_applied = times_applied + 1,
			    failure_count = failure_count + 1,
			    total_pnl = total_pnl + $2,
			    avg_pnl = (total_pnl + $2) / (times_applied + 1),
			    win_rate = success_count::float / (times_applied + 1),
			    last_applied = $3,
			    updated_at = $3
			WHERE id = $1
		`
	}

	_, err := pm.pool.Exec(ctx, query, id, pnl, now)
	if err != nil {
		return fmt.Errorf("failed to record policy application: %w", err)
	}

	log.Debug().
		Str("id", id.String()).
		Bool("success", success).
		Float64("pnl", pnl).
		Msg("Recorded policy application")

	return nil
}

// DeactivatePolicy deactivates a policy
func (pm *ProceduralMemory) DeactivatePolicy(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE procedural_memory_policies
		SET is_active = false,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := pm.pool.Exec(ctx, query, id)
	return err
}

// StoreSkill stores a skill in procedural memory
func (pm *ProceduralMemory) StoreSkill(ctx context.Context, skill *Skill) error {
	if skill.ID == uuid.Nil {
		skill.ID = uuid.New()
	}
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = time.Now()
	}
	skill.UpdatedAt = time.Now()

	query := `
		INSERT INTO procedural_memory_skills (
			id, type, name, description, implementation, parameters, prerequisites,
			times_used, success_count, failure_count, avg_duration, avg_accuracy,
			agent_name, learned_from, source_id, proficiency, is_active,
			created_at, updated_at, last_used
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			$18, $19, $20
		)
		ON CONFLICT (id) DO UPDATE SET
			times_used = EXCLUDED.times_used,
			success_count = EXCLUDED.success_count,
			failure_count = EXCLUDED.failure_count,
			avg_duration = EXCLUDED.avg_duration,
			avg_accuracy = EXCLUDED.avg_accuracy,
			proficiency = EXCLUDED.proficiency,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at,
			last_used = EXCLUDED.last_used
	`

	_, err := pm.pool.Exec(
		ctx,
		query,
		skill.ID,
		skill.Type,
		skill.Name,
		skill.Description,
		skill.Implementation,
		skill.Parameters,
		skill.Prerequisites,
		skill.TimesUsed,
		skill.SuccessCount,
		skill.FailureCount,
		skill.AvgDuration,
		skill.AvgAccuracy,
		skill.AgentName,
		skill.LearnedFrom,
		skill.SourceID,
		skill.Proficiency,
		skill.IsActive,
		skill.CreatedAt,
		skill.UpdatedAt,
		skill.LastUsed,
	)

	if err != nil {
		return fmt.Errorf("failed to store skill: %w", err)
	}

	log.Debug().
		Str("id", skill.ID.String()).
		Str("type", string(skill.Type)).
		Str("name", skill.Name).
		Msg("Stored skill in procedural memory")

	return nil
}

// GetSkillsByAgent retrieves skills for a specific agent
func (pm *ProceduralMemory) GetSkillsByAgent(ctx context.Context, agentName string, activeOnly bool) ([]*Skill, error) {
	query := `
		SELECT
			id, type, name, description, implementation, parameters, prerequisites,
			times_used, success_count, failure_count, avg_duration, avg_accuracy,
			agent_name, learned_from, source_id, proficiency, is_active,
			created_at, updated_at, last_used
		FROM procedural_memory_skills
		WHERE agent_name = $1
	`

	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY proficiency DESC, created_at DESC"

	rows, err := pm.pool.Query(ctx, query, agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to query skills: %w", err)
	}
	defer rows.Close()

	return pm.scanSkills(rows)
}

// RecordSkillUsage records that a skill was used
func (pm *ProceduralMemory) RecordSkillUsage(ctx context.Context, id uuid.UUID, success bool, duration float64, accuracy float64) error {
	now := time.Now()

	var query string
	if success {
		query = `
			UPDATE procedural_memory_skills
			SET times_used = times_used + 1,
			    success_count = success_count + 1,
			    avg_duration = (avg_duration * times_used + $2) / (times_used + 1),
			    avg_accuracy = (avg_accuracy * times_used + $3) / (times_used + 1),
			    proficiency = LEAST(1.0, proficiency + 0.01),
			    last_used = $4,
			    updated_at = $4
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE procedural_memory_skills
			SET times_used = times_used + 1,
			    failure_count = failure_count + 1,
			    avg_duration = (avg_duration * times_used + $2) / (times_used + 1),
			    proficiency = GREATEST(0.0, proficiency - 0.02),
			    last_used = $4,
			    updated_at = $4
			WHERE id = $1
		`
	}

	_, err := pm.pool.Exec(ctx, query, id, duration, accuracy, now)
	if err != nil {
		return fmt.Errorf("failed to record skill usage: %w", err)
	}

	log.Debug().
		Str("id", id.String()).
		Bool("success", success).
		Float64("duration", duration).
		Msg("Recorded skill usage")

	return nil
}

// Helper functions for scanning database results

func (pm *ProceduralMemory) scanPolicies(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*Policy, error) {
	var policies []*Policy

	for rows.Next() {
		var p Policy
		var lastApplied, lastModified *time.Time

		err := rows.Scan(
			&p.ID,
			&p.Type,
			&p.Name,
			&p.Description,
			&p.Conditions,
			&p.Actions,
			&p.Parameters,
			&p.TimesApplied,
			&p.SuccessCount,
			&p.FailureCount,
			&p.AvgPnL,
			&p.TotalPnL,
			&p.Sharpe,
			&p.MaxDrawdown,
			&p.WinRate,
			&p.AgentName,
			&p.Symbol,
			&p.LearnedFrom,
			&p.SourceID,
			&p.Confidence,
			&p.IsActive,
			&p.Priority,
			&p.CreatedAt,
			&p.UpdatedAt,
			&lastApplied,
			&lastModified,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}

		p.LastApplied = lastApplied
		p.LastModified = lastModified

		policies = append(policies, &p)
	}

	return policies, rows.Err()
}

func (pm *ProceduralMemory) scanSkills(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*Skill, error) {
	var skills []*Skill

	for rows.Next() {
		var s Skill
		var lastUsed *time.Time

		err := rows.Scan(
			&s.ID,
			&s.Type,
			&s.Name,
			&s.Description,
			&s.Implementation,
			&s.Parameters,
			&s.Prerequisites,
			&s.TimesUsed,
			&s.SuccessCount,
			&s.FailureCount,
			&s.AvgDuration,
			&s.AvgAccuracy,
			&s.AgentName,
			&s.LearnedFrom,
			&s.SourceID,
			&s.Proficiency,
			&s.IsActive,
			&s.CreatedAt,
			&s.UpdatedAt,
			&lastUsed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}

		s.LastUsed = lastUsed

		skills = append(skills, &s)
	}

	return skills, rows.Err()
}

// Helper functions for creating policy/skill structures

// CreatePolicyConditions creates JSONB conditions for a policy
func CreatePolicyConditions(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// CreatePolicyActions creates JSONB actions for a policy
func CreatePolicyActions(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// CreatePolicyParameters creates JSONB parameters for a policy
func CreatePolicyParameters(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// CreateSkillImplementation creates JSONB implementation for a skill
func CreateSkillImplementation(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}
