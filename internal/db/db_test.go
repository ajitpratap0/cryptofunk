package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database connection
// Skips test if DATABASE_URL is not set
func setupTestDB(t *testing.T) (*DB, func()) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("Skipping database test: DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := New(ctx)
	if err != nil {
		t.Skipf("Skipping database test: failed to connect: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestNew(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	assert.NotNil(t, db)
	assert.NotNil(t, db.Pool())
}

func TestClose(t *testing.T) {
	db, _ := setupTestDB(t)

	// Close doesn't return error
	db.Close()
}

func TestPing(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.Ping(ctx)
	assert.NoError(t, err)
}

func TestPool(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	pool := db.Pool()
	assert.NotNil(t, pool)
}

func TestHealth(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.Health(ctx)
	assert.NoError(t, err)
}

// TestGetAgentStatus tests retrieving agent status
func TestGetAgentStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	agentName := "test-agent-" + uuid.New().String()[:8]
	now := time.Now()

	// First, upsert a status
	err := db.UpsertAgentStatus(ctx, &AgentStatus{
		Name:          agentName,
		Type:          "technical",
		Status:        "RUNNING",
		StartedAt:     &now,
		LastHeartbeat: &now,
		TotalSignals:  0,
		ErrorCount:    0,
		Metadata: map[string]interface{}{
			"version": "1.0.0",
			"config":  map[string]interface{}{"test": "value"},
			"metrics": map[string]interface{}{"uptime": 100},
		},
	})
	require.NoError(t, err)

	// Now get it
	status, err := db.GetAgentStatus(ctx, agentName)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, agentName, status.Name)
	assert.Equal(t, "RUNNING", status.Status)
	assert.Equal(t, "technical", status.Type)
}

func TestGetAgentStatus_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	status, err := db.GetAgentStatus(ctx, "non-existent-agent")

	// Should return error for not found
	assert.Error(t, err)
	assert.Nil(t, status)
}

func TestGetAllAgentStatuses(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Insert multiple agents
	agent1 := "test-agent-1-" + uuid.New().String()[:8]
	agent2 := "test-agent-2-" + uuid.New().String()[:8]

	err := db.UpsertAgentStatus(ctx, &AgentStatus{
		Name:          agent1,
		Type:          "technical",
		Status:        "RUNNING",
		StartedAt:     &now,
		LastHeartbeat: &now,
		TotalSignals:  0,
		ErrorCount:    0,
		Metadata: map[string]interface{}{
			"version": "1.0.0",
		},
	})
	require.NoError(t, err)

	err = db.UpsertAgentStatus(ctx, &AgentStatus{
		Name:          agent2,
		Type:          "strategy",
		Status:        "RUNNING",
		StartedAt:     &now,
		LastHeartbeat: &now,
		TotalSignals:  0,
		ErrorCount:    0,
		Metadata: map[string]interface{}{
			"version": "1.0.0",
		},
	})
	require.NoError(t, err)

	// Get all statuses
	statuses, err := db.GetAllAgentStatuses(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(statuses), 2)

	// Verify our test agents are in the list
	foundAgent1 := false
	foundAgent2 := false
	for _, status := range statuses {
		if status.Name == agent1 {
			foundAgent1 = true
		}
		if status.Name == agent2 {
			foundAgent2 = true
		}
	}
	assert.True(t, foundAgent1, "Should find agent1")
	assert.True(t, foundAgent2, "Should find agent2")
}

func TestUpsertAgentStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	agentName := "test-agent-" + uuid.New().String()[:8]
	now := time.Now()

	// Insert
	err := db.UpsertAgentStatus(ctx, &AgentStatus{
		Name:          agentName,
		Type:          "technical",
		Status:        "STARTING",
		StartedAt:     &now,
		LastHeartbeat: &now,
		TotalSignals:  0,
		ErrorCount:    0,
		Metadata: map[string]interface{}{
			"version": "1.0.0",
		},
	})
	require.NoError(t, err)

	// Verify insert
	status, err := db.GetAgentStatus(ctx, agentName)
	require.NoError(t, err)
	assert.Equal(t, "STARTING", status.Status)

	// Update
	updatedNow := time.Now()
	err = db.UpsertAgentStatus(ctx, &AgentStatus{
		Name:          agentName,
		Type:          "technical",
		Status:        "RUNNING",
		StartedAt:     &now,
		LastHeartbeat: &updatedNow,
		TotalSignals:  10,
		ErrorCount:    0,
		Metadata: map[string]interface{}{
			"version": "1.0.0",
		},
	})
	require.NoError(t, err)

	// Verify update
	status, err = db.GetAgentStatus(ctx, agentName)
	require.NoError(t, err)
	assert.Equal(t, "RUNNING", status.Status)
	assert.Equal(t, 10, status.TotalSignals)
}

func TestUpsertAgentStatus_WithMetadata(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	agentName := "test-agent-" + uuid.New().String()[:8]
	now := time.Now()

	metadata := map[string]interface{}{
		"version": "1.0.0",
		"config": map[string]interface{}{
			"symbol":    "BTC/USDT",
			"timeframe": "1h",
			"indicators": map[string]interface{}{
				"RSI": 14,
				"MACD": map[string]int{
					"fast": 12,
					"slow": 26,
				},
			},
		},
		"metrics": map[string]interface{}{
			"uptime_seconds":    3600,
			"signals_generated": 42,
			"accuracy":          0.85,
		},
	}

	err := db.UpsertAgentStatus(ctx, &AgentStatus{
		Name:          agentName,
		Type:          "technical",
		Status:        "RUNNING",
		StartedAt:     &now,
		LastHeartbeat: &now,
		TotalSignals:  42,
		ErrorCount:    0,
		Metadata:      metadata,
	})
	require.NoError(t, err)

	// Verify
	status, err := db.GetAgentStatus(ctx, agentName)
	require.NoError(t, err)
	assert.NotNil(t, status.Metadata)

	// Check specific values from metadata
	metadataMap, ok := status.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1.0.0", metadataMap["version"])
}
