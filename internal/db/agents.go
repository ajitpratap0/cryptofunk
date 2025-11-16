package db

import (
	"context"
	"time"
)

// AgentStatus represents an agent's status
type AgentStatus struct {
	ID            string     `db:"id" json:"id"`
	Name          string     `db:"agent_name" json:"agent_name"`
	Type          string     `db:"agent_type" json:"agent_type"`
	Status        string     `db:"status" json:"status"`
	PID           *int       `db:"pid" json:"pid,omitempty"`
	StartedAt     *time.Time `db:"started_at" json:"started_at,omitempty"`
	LastHeartbeat *time.Time `db:"last_heartbeat" json:"last_heartbeat,omitempty"`
	TotalSignals  int        `db:"total_signals" json:"total_signals"`
	AvgConfidence *float64   `db:"avg_confidence" json:"avg_confidence,omitempty"`
	ErrorCount    int        `db:"error_count" json:"error_count"`
	LastError     *string    `db:"last_error" json:"last_error,omitempty"`
	Config        any        `db:"config" json:"config,omitempty"`
	Metadata      any        `db:"metadata" json:"metadata,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

// GetAgentStatus retrieves a specific agent's status
func (db *DB) GetAgentStatus(ctx context.Context, name string) (*AgentStatus, error) {
	query := `
		SELECT id, agent_name, agent_type, status, pid, started_at, last_heartbeat,
		       total_signals, avg_confidence, error_count, last_error, config, metadata,
		       created_at, updated_at
		FROM agent_status
		WHERE agent_name = $1
	`

	var agent AgentStatus
	err := db.pool.QueryRow(ctx, query, name).Scan(
		&agent.ID,
		&agent.Name,
		&agent.Type,
		&agent.Status,
		&agent.PID,
		&agent.StartedAt,
		&agent.LastHeartbeat,
		&agent.TotalSignals,
		&agent.AvgConfidence,
		&agent.ErrorCount,
		&agent.LastError,
		&agent.Config,
		&agent.Metadata,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &agent, nil
}

// GetAllAgentStatuses retrieves all agents' statuses
func (db *DB) GetAllAgentStatuses(ctx context.Context) ([]*AgentStatus, error) {
	query := `
		SELECT id, agent_name, agent_type, status, pid, started_at, last_heartbeat,
		       total_signals, avg_confidence, error_count, last_error, config, metadata,
		       created_at, updated_at
		FROM agent_status
		ORDER BY agent_name ASC
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*AgentStatus
	for rows.Next() {
		var agent AgentStatus
		err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&agent.Type,
			&agent.Status,
			&agent.PID,
			&agent.StartedAt,
			&agent.LastHeartbeat,
			&agent.TotalSignals,
			&agent.AvgConfidence,
			&agent.ErrorCount,
			&agent.LastError,
			&agent.Config,
			&agent.Metadata,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		agents = append(agents, &agent)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return agents, nil
}

// UpsertAgentStatus inserts or updates an agent's status
func (db *DB) UpsertAgentStatus(ctx context.Context, agent *AgentStatus) error {
	query := `
		INSERT INTO agent_status (
			agent_name, agent_type, status, pid, started_at, last_heartbeat,
			total_signals, avg_confidence, error_count, last_error, config, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (agent_name) DO UPDATE SET
			agent_type = EXCLUDED.agent_type,
			status = EXCLUDED.status,
			pid = EXCLUDED.pid,
			started_at = EXCLUDED.started_at,
			last_heartbeat = EXCLUDED.last_heartbeat,
			total_signals = EXCLUDED.total_signals,
			avg_confidence = EXCLUDED.avg_confidence,
			error_count = EXCLUDED.error_count,
			last_error = EXCLUDED.last_error,
			config = EXCLUDED.config,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.pool.QueryRow(ctx, query,
		agent.Name,
		agent.Type,
		agent.Status,
		agent.PID,
		agent.StartedAt,
		agent.LastHeartbeat,
		agent.TotalSignals,
		agent.AvgConfidence,
		agent.ErrorCount,
		agent.LastError,
		agent.Config,
		agent.Metadata,
	).Scan(&agent.ID, &agent.CreatedAt, &agent.UpdatedAt)

	return err
}
