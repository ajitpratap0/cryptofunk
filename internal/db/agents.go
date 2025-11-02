package db

import (
	"context"
	"time"
)

// AgentStatus represents an agent's status
type AgentStatus struct {
	Name       string    `db:"name" json:"name"`
	Status     string    `db:"status" json:"status"`
	LastSeenAt time.Time `db:"last_seen_at" json:"last_seen_at"`
	IsHealthy  bool      `db:"is_healthy" json:"is_healthy"`
	Metadata   any       `db:"metadata" json:"metadata,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// GetAgentStatus retrieves a specific agent's status
func (db *DB) GetAgentStatus(ctx context.Context, name string) (*AgentStatus, error) {
	query := `
		SELECT name, status, last_seen_at, is_healthy, metadata, created_at, updated_at
		FROM agent_status
		WHERE name = $1
	`

	var agent AgentStatus
	err := db.pool.QueryRow(ctx, query, name).Scan(
		&agent.Name,
		&agent.Status,
		&agent.LastSeenAt,
		&agent.IsHealthy,
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
		SELECT name, status, last_seen_at, is_healthy, metadata, created_at, updated_at
		FROM agent_status
		ORDER BY name ASC
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
			&agent.Name,
			&agent.Status,
			&agent.LastSeenAt,
			&agent.IsHealthy,
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
		INSERT INTO agent_status (name, status, last_seen_at, is_healthy, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET
			status = EXCLUDED.status,
			last_seen_at = EXCLUDED.last_seen_at,
			is_healthy = EXCLUDED.is_healthy,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`

	_, err := db.pool.Exec(ctx, query,
		agent.Name,
		agent.Status,
		agent.LastSeenAt,
		agent.IsHealthy,
		agent.Metadata,
	)

	return err
}
