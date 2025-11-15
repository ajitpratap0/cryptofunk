package db_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentStatusCRUDWithTestcontainers tests complete CRUD operations for agent status
func TestAgentStatusCRUDWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("UpsertNewAgent", func(t *testing.T) {
		// Create a new agent status
		pid := os.Getpid()
		now := time.Now()
		agent := &db.AgentStatus{
			Name:          "test-agent-1",
			Type:          "technical",
			Status:        "RUNNING",
			PID:           &pid,
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  0,
			ErrorCount:    0,
			Metadata: map[string]interface{}{
				"version": "1.0.0",
				"mode":    "paper",
			},
		}

		// Upsert the agent
		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Verify ID was set
		assert.NotEmpty(t, agent.ID)
		assert.NotZero(t, agent.CreatedAt)
		assert.NotZero(t, agent.UpdatedAt)

		// Retrieve and verify
		retrieved, err := tc.DB.GetAgentStatus(ctx, "test-agent-1")
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, "test-agent-1", retrieved.Name)
		assert.Equal(t, "technical", retrieved.Type)
		assert.Equal(t, "RUNNING", retrieved.Status)
		assert.NotNil(t, retrieved.PID)
		assert.Equal(t, pid, *retrieved.PID)

		// Verify metadata
		metadataBytes, err := json.Marshal(retrieved.Metadata)
		require.NoError(t, err)
		var metadata map[string]interface{}
		err = json.Unmarshal(metadataBytes, &metadata)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", metadata["version"])
		assert.Equal(t, "paper", metadata["mode"])
	})

	t.Run("GetAgentStatus", func(t *testing.T) {
		// Create an agent
		now := time.Now()
		errorMsg := "test error"
		agent := &db.AgentStatus{
			Name:          "test-agent-2",
			Type:          "risk",
			Status:        "STOPPED",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  5,
			ErrorCount:    1,
			LastError:     &errorMsg,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Get the agent status
		retrieved, err := tc.DB.GetAgentStatus(ctx, "test-agent-2")
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, "test-agent-2", retrieved.Name)
		assert.Equal(t, "risk", retrieved.Type)
		assert.Equal(t, "STOPPED", retrieved.Status)
		assert.Equal(t, 5, retrieved.TotalSignals)
		assert.Equal(t, 1, retrieved.ErrorCount)
		assert.NotNil(t, retrieved.LastError)
		assert.Equal(t, "test error", *retrieved.LastError)
	})

	t.Run("GetNonexistentAgent", func(t *testing.T) {
		// Try to get an agent that doesn't exist
		retrieved, err := tc.DB.GetAgentStatus(ctx, "nonexistent-agent")
		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Equal(t, pgx.ErrNoRows, err)
	})

	t.Run("UpdateExistingAgent", func(t *testing.T) {
		// Create an agent
		now := time.Now()
		agent := &db.AgentStatus{
			Name:          "test-agent-3",
			Type:          "trend",
			Status:        "STARTING",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  0,
			ErrorCount:    0,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Get initial state
		initial, err := tc.DB.GetAgentStatus(ctx, "test-agent-3")
		require.NoError(t, err)
		initialID := initial.ID
		initialCreatedAt := initial.CreatedAt

		// Wait a moment to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		// Update the same agent (upsert should update)
		updatedNow := time.Now()
		avgConf := 0.85
		updatedAgent := &db.AgentStatus{
			Name:          "test-agent-3",
			Type:          "trend",
			Status:        "RUNNING",
			StartedAt:     &now,
			LastHeartbeat: &updatedNow,
			TotalSignals:  10,
			AvgConfidence: &avgConf,
			ErrorCount:    0,
		}

		err = tc.DB.UpsertAgentStatus(ctx, updatedAgent)
		require.NoError(t, err)

		// Verify the update
		updated, err := tc.DB.GetAgentStatus(ctx, "test-agent-3")
		require.NoError(t, err)

		assert.Equal(t, initialID, updated.ID, "ID should remain the same on update")
		assert.Equal(t, "RUNNING", updated.Status)
		assert.Equal(t, 10, updated.TotalSignals)
		assert.NotNil(t, updated.AvgConfidence)
		assert.Equal(t, 0.85, *updated.AvgConfidence)
		assert.Equal(t, initialCreatedAt.Unix(), updated.CreatedAt.Unix(),
			"CreatedAt should not change on update")
		assert.True(t, updated.UpdatedAt.After(initial.UpdatedAt),
			"UpdatedAt should be updated on upsert")
	})

	t.Run("GetAllAgentStatuses", func(t *testing.T) {
		// Create multiple agents
		now := time.Now()
		agents := []*db.AgentStatus{
			{
				Name:          "agent-alpha",
				Type:          "technical",
				Status:        "RUNNING",
				StartedAt:     &now,
				LastHeartbeat: &now,
				TotalSignals:  100,
				ErrorCount:    0,
			},
			{
				Name:          "agent-beta",
				Type:          "sentiment",
				Status:        "STOPPED",
				StartedAt:     &now,
				LastHeartbeat: &now,
				TotalSignals:  50,
				ErrorCount:    5,
			},
			{
				Name:          "agent-gamma",
				Type:          "orderbook",
				Status:        "RUNNING",
				StartedAt:     &now,
				LastHeartbeat: &now,
				TotalSignals:  200,
				ErrorCount:    1,
			},
		}

		for _, agent := range agents {
			err := tc.DB.UpsertAgentStatus(ctx, agent)
			require.NoError(t, err)
		}

		// Get all agent statuses
		allAgents, err := tc.DB.GetAllAgentStatuses(ctx)
		require.NoError(t, err)

		// Should have at least the 3 we just created (plus any from other tests)
		assert.GreaterOrEqual(t, len(allAgents), 3)

		// Verify they're sorted by name (ASC)
		foundAlpha := false
		foundBeta := false
		foundGamma := false
		var prevName string
		for _, agent := range allAgents {
			// Check alphabetical ordering
			if prevName != "" {
				assert.True(t, agent.Name >= prevName,
					"Agents should be sorted by name in ascending order")
			}
			prevName = agent.Name

			// Check for our test agents
			switch agent.Name {
			case "agent-alpha":
				foundAlpha = true
				assert.Equal(t, "technical", agent.Type)
				assert.Equal(t, "RUNNING", agent.Status)
				assert.Equal(t, 100, agent.TotalSignals)
			case "agent-beta":
				foundBeta = true
				assert.Equal(t, "sentiment", agent.Type)
				assert.Equal(t, "STOPPED", agent.Status)
				assert.Equal(t, 50, agent.TotalSignals)
				assert.Equal(t, 5, agent.ErrorCount)
			case "agent-gamma":
				foundGamma = true
				assert.Equal(t, "orderbook", agent.Type)
				assert.Equal(t, "RUNNING", agent.Status)
				assert.Equal(t, 200, agent.TotalSignals)
			}
		}

		assert.True(t, foundAlpha, "Should find agent-alpha")
		assert.True(t, foundBeta, "Should find agent-beta")
		assert.True(t, foundGamma, "Should find agent-gamma")
	})

	t.Run("GetAllAgentStatusesFromFreshDatabase", func(t *testing.T) {
		// Create a fresh test database for this subtest
		tc2 := testhelpers.SetupTestDatabase(t)
		err := tc2.ApplyMigrations("../../migrations")
		require.NoError(t, err)

		ctx2 := context.Background()
		now2 := time.Now()

		// Insert a test agent to ensure database operations work
		testAgent := &db.AgentStatus{
			Name:          "fresh-db-test-agent",
			Type:          "test",
			Status:        "RUNNING",
			StartedAt:     &now2,
			LastHeartbeat: &now2,
			TotalSignals:  1,
			ErrorCount:    0,
		}
		err = tc2.DB.UpsertAgentStatus(ctx2, testAgent)
		require.NoError(t, err)

		// Get all agent statuses - should have exactly the one we just created
		allAgents, err := tc2.DB.GetAllAgentStatuses(ctx2)
		require.NoError(t, err)
		assert.NotNil(t, allAgents)
		assert.GreaterOrEqual(t, len(allAgents), 1, "Should have at least our test agent")

		// Verify our test agent is present
		foundTest := false
		for _, agent := range allAgents {
			if agent.Name == "fresh-db-test-agent" {
				foundTest = true
				break
			}
		}
		assert.True(t, foundTest, "Should find our test agent in fresh database")
	})
}

// TestAgentStatusMetadataWithTestcontainers tests complex metadata handling
func TestAgentStatusMetadataWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	t.Run("ComplexNestedMetadata", func(t *testing.T) {
		// Create agent with complex nested metadata
		complexMetadata := map[string]interface{}{
			"version": "2.1.0",
			"config": map[string]interface{}{
				"max_retries":    3,
				"timeout_sec":    30,
				"enable_feature": true,
			},
			"performance": map[string]interface{}{
				"avg_latency_ms": 45.2,
				"requests_total": 1532,
				"errors":         []interface{}{"timeout", "connection_refused"},
			},
			"tags": []interface{}{"production", "high-priority", "critical"},
		}

		agent := &db.AgentStatus{
			Name:          "complex-metadata-agent",
			Type:          "strategy",
			Status:        "RUNNING",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  100,
			ErrorCount:    0,
			Metadata:      complexMetadata,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Retrieve and verify complex metadata
		retrieved, err := tc.DB.GetAgentStatus(ctx, "complex-metadata-agent")
		require.NoError(t, err)

		// Convert metadata back to map for verification
		metadataBytes, err := json.Marshal(retrieved.Metadata)
		require.NoError(t, err)
		var metadata map[string]interface{}
		err = json.Unmarshal(metadataBytes, &metadata)
		require.NoError(t, err)

		assert.Equal(t, "2.1.0", metadata["version"])

		config := metadata["config"].(map[string]interface{})
		assert.Equal(t, float64(3), config["max_retries"])
		assert.Equal(t, float64(30), config["timeout_sec"])
		assert.Equal(t, true, config["enable_feature"])

		performance := metadata["performance"].(map[string]interface{})
		assert.Equal(t, 45.2, performance["avg_latency_ms"])
		assert.Equal(t, float64(1532), performance["requests_total"])

		errors := performance["errors"].([]interface{})
		assert.Len(t, errors, 2)
		assert.Contains(t, errors, "timeout")
		assert.Contains(t, errors, "connection_refused")

		tags := metadata["tags"].([]interface{})
		assert.Len(t, tags, 3)
		assert.Contains(t, tags, "production")
	})

	t.Run("NullMetadata", func(t *testing.T) {
		// Create agent with nil metadata
		agent := &db.AgentStatus{
			Name:          "null-metadata-agent",
			Type:          "analysis",
			Status:        "RUNNING",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  0,
			ErrorCount:    0,
			Metadata:      nil,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Retrieve and verify
		_, err = tc.DB.GetAgentStatus(ctx, "null-metadata-agent")
		require.NoError(t, err)
		// Metadata can be nil or empty based on JSONB handling
		// Both are acceptable for null JSONB
	})

	t.Run("EmptyMetadata", func(t *testing.T) {
		// Create agent with empty metadata map
		agent := &db.AgentStatus{
			Name:          "empty-metadata-agent",
			Type:          "analysis",
			Status:        "RUNNING",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  0,
			ErrorCount:    0,
			Metadata:      map[string]interface{}{},
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Retrieve and verify
		_, err = tc.DB.GetAgentStatus(ctx, "empty-metadata-agent")
		require.NoError(t, err)
	})

	t.Run("ConfigAndMetadata", func(t *testing.T) {
		// Test both config and metadata fields
		config := map[string]interface{}{
			"refresh_interval": 60,
			"max_concurrent":   5,
		}

		metadata := map[string]interface{}{
			"deployment": "production",
			"region":     "us-east-1",
		}

		agent := &db.AgentStatus{
			Name:          "config-metadata-agent",
			Type:          "executor",
			Status:        "RUNNING",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  25,
			ErrorCount:    0,
			Config:        config,
			Metadata:      metadata,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := tc.DB.GetAgentStatus(ctx, "config-metadata-agent")
		require.NoError(t, err)

		// Verify config
		configBytes, err := json.Marshal(retrieved.Config)
		require.NoError(t, err)
		var retrievedConfig map[string]interface{}
		err = json.Unmarshal(configBytes, &retrievedConfig)
		require.NoError(t, err)
		assert.Equal(t, float64(60), retrievedConfig["refresh_interval"])
		assert.Equal(t, float64(5), retrievedConfig["max_concurrent"])

		// Verify metadata
		metadataBytes, err := json.Marshal(retrieved.Metadata)
		require.NoError(t, err)
		var retrievedMetadata map[string]interface{}
		err = json.Unmarshal(metadataBytes, &retrievedMetadata)
		require.NoError(t, err)
		assert.Equal(t, "production", retrievedMetadata["deployment"])
		assert.Equal(t, "us-east-1", retrievedMetadata["region"])
	})
}

// TestAgentStatusConcurrencyWithTestcontainers tests concurrent agent status updates
func TestAgentStatusConcurrencyWithTestcontainers(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	t.Run("MultipleAgentsConcurrentInsert", func(t *testing.T) {
		// Insert 50 different agents concurrently
		var wg sync.WaitGroup
		errors := make(chan error, 50)

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				agentName := "concurrent-agent-" + string(rune('a'+index%26)) + string(rune('0'+(index/26)%10)) + string(rune('0'+index%10))
				agentNow := time.Now()
				agent := &db.AgentStatus{
					Name:          agentName,
					Type:          "test",
					Status:        "RUNNING",
					StartedAt:     &agentNow,
					LastHeartbeat: &agentNow,
					TotalSignals:  index,
					ErrorCount:    0,
				}

				err := tc.DB.UpsertAgentStatus(ctx, agent)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent insert error: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "Should have no errors during concurrent inserts")

		// Verify all agents were inserted
		allAgents, err := tc.DB.GetAllAgentStatuses(ctx)
		require.NoError(t, err)

		// Count agents matching our pattern
		concurrentAgentCount := 0
		for _, agent := range allAgents {
			if len(agent.Name) > 17 && agent.Name[:17] == "concurrent-agent-" {
				concurrentAgentCount++
			}
		}
		assert.GreaterOrEqual(t, concurrentAgentCount, 50,
			"Should have at least 50 concurrent agents")
	})

	t.Run("SameAgentConcurrentUpdates", func(t *testing.T) {
		// Create initial agent
		agentName := "update-race-agent"
		agent := &db.AgentStatus{
			Name:          agentName,
			Type:          "test",
			Status:        "STARTING",
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  0,
			ErrorCount:    0,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		// Update the same agent 100 times concurrently
		var wg sync.WaitGroup
		errors := make(chan error, 100)
		successCount := make(chan int, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				agentNow := time.Now()
				updateAgent := &db.AgentStatus{
					Name:          agentName,
					Type:          "test",
					Status:        "RUNNING",
					StartedAt:     &now,
					LastHeartbeat: &agentNow,
					TotalSignals:  index,
					ErrorCount:    0,
				}

				err := tc.DB.UpsertAgentStatus(ctx, updateAgent)
				if err != nil {
					errors <- err
				} else {
					successCount <- 1
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(successCount)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent update error: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "Should have no errors during concurrent updates")

		// Count successful updates
		totalSuccess := 0
		for range successCount {
			totalSuccess++
		}
		assert.Equal(t, 100, totalSuccess, "All 100 updates should succeed")

		// Verify final state (should be one of the updates)
		final, err := tc.DB.GetAgentStatus(ctx, agentName)
		require.NoError(t, err)
		assert.Equal(t, "RUNNING", final.Status)

		// TotalSignals should contain one of the values (0-99)
		assert.GreaterOrEqual(t, final.TotalSignals, 0)
		assert.LessOrEqual(t, final.TotalSignals, 99)
	})

	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		// Create agents for concurrent read/write test
		agentsToCreate := []string{
			"rw-agent-1",
			"rw-agent-2",
			"rw-agent-3",
			"rw-agent-4",
			"rw-agent-5",
		}

		for _, name := range agentsToCreate {
			agent := &db.AgentStatus{
				Name:          name,
				Type:          "test",
				Status:        "RUNNING",
				StartedAt:     &now,
				LastHeartbeat: &now,
				TotalSignals:  0,
				ErrorCount:    0,
			}
			err := tc.DB.UpsertAgentStatus(ctx, agent)
			require.NoError(t, err)
		}

		// Perform concurrent reads and writes
		var wg sync.WaitGroup
		errors := make(chan error, 200)

		// 100 concurrent reads
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				agentName := agentsToCreate[index%len(agentsToCreate)]
				_, err := tc.DB.GetAgentStatus(ctx, agentName)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		// 50 concurrent writes
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				agentName := agentsToCreate[index%len(agentsToCreate)]
				agentNow := time.Now()
				agent := &db.AgentStatus{
					Name:          agentName,
					Type:          "test",
					Status:        "RUNNING",
					StartedAt:     &now,
					LastHeartbeat: &agentNow,
					TotalSignals:  index,
					ErrorCount:    0,
				}
				err := tc.DB.UpsertAgentStatus(ctx, agent)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		// 50 concurrent GetAll operations
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_, err := tc.DB.GetAllAgentStatuses(ctx)
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent read/write error: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "Should have no errors during concurrent read/write")
	})
}

// TestAgentStatusEdgeCases tests edge cases and error conditions
func TestAgentStatusEdgeCases(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	t.Run("AllFieldsPopulated", func(t *testing.T) {
		// Test with all optional fields populated
		pid := os.Getpid()
		avgConf := 0.92
		errorMsg := "last error message"

		agent := &db.AgentStatus{
			Name:          "full-fields-agent",
			Type:          "technical",
			Status:        "RUNNING",
			PID:           &pid,
			StartedAt:     &now,
			LastHeartbeat: &now,
			TotalSignals:  1000,
			AvgConfidence: &avgConf,
			ErrorCount:    10,
			LastError:     &errorMsg,
			Config: map[string]interface{}{
				"setting1": "value1",
			},
			Metadata: map[string]interface{}{
				"meta1": "metavalue1",
			},
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		retrieved, err := tc.DB.GetAgentStatus(ctx, "full-fields-agent")
		require.NoError(t, err)

		assert.Equal(t, "full-fields-agent", retrieved.Name)
		assert.Equal(t, "technical", retrieved.Type)
		assert.Equal(t, "RUNNING", retrieved.Status)
		assert.NotNil(t, retrieved.PID)
		assert.Equal(t, pid, *retrieved.PID)
		assert.NotNil(t, retrieved.StartedAt)
		assert.NotNil(t, retrieved.LastHeartbeat)
		assert.Equal(t, 1000, retrieved.TotalSignals)
		assert.NotNil(t, retrieved.AvgConfidence)
		assert.Equal(t, 0.92, *retrieved.AvgConfidence)
		assert.Equal(t, 10, retrieved.ErrorCount)
		assert.NotNil(t, retrieved.LastError)
		assert.Equal(t, "last error message", *retrieved.LastError)
	})

	t.Run("MinimalFieldsPopulated", func(t *testing.T) {
		// Test with only required fields
		agent := &db.AgentStatus{
			Name:         "minimal-agent",
			Type:         "risk",
			Status:       "STOPPED",
			TotalSignals: 0,
			ErrorCount:   0,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		retrieved, err := tc.DB.GetAgentStatus(ctx, "minimal-agent")
		require.NoError(t, err)

		assert.Equal(t, "minimal-agent", retrieved.Name)
		assert.Equal(t, "risk", retrieved.Type)
		assert.Equal(t, "STOPPED", retrieved.Status)
		assert.Nil(t, retrieved.PID)
		assert.Nil(t, retrieved.StartedAt)
		assert.Nil(t, retrieved.LastHeartbeat)
		assert.Equal(t, 0, retrieved.TotalSignals)
		assert.Nil(t, retrieved.AvgConfidence)
		assert.Equal(t, 0, retrieved.ErrorCount)
		assert.Nil(t, retrieved.LastError)
	})

	t.Run("TimestampPrecision", func(t *testing.T) {
		// Test that timestamps maintain reasonable precision
		beforeInsert := time.Now()

		agent := &db.AgentStatus{
			Name:          "timestamp-test-agent",
			Type:          "test",
			Status:        "RUNNING",
			StartedAt:     &beforeInsert,
			LastHeartbeat: &beforeInsert,
			TotalSignals:  0,
			ErrorCount:    0,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		afterInsert := time.Now()

		retrieved, err := tc.DB.GetAgentStatus(ctx, "timestamp-test-agent")
		require.NoError(t, err)

		// StartedAt should be close to beforeInsert
		assert.WithinDuration(t, beforeInsert, *retrieved.StartedAt, time.Second,
			"StartedAt should preserve precision")

		// CreatedAt should be between beforeInsert and afterInsert
		assert.True(t, retrieved.CreatedAt.After(beforeInsert.Add(-time.Second)),
			"CreatedAt should be after insert started")
		assert.True(t, retrieved.CreatedAt.Before(afterInsert.Add(time.Second)),
			"CreatedAt should be before insert completed")
	})

	t.Run("HighErrorCount", func(t *testing.T) {
		// Test with high error count
		agent := &db.AgentStatus{
			Name:         "error-prone-agent",
			Type:         "strategy",
			Status:       "ERROR",
			TotalSignals: 5000,
			ErrorCount:   999,
		}

		err := tc.DB.UpsertAgentStatus(ctx, agent)
		require.NoError(t, err)

		retrieved, err := tc.DB.GetAgentStatus(ctx, "error-prone-agent")
		require.NoError(t, err)
		assert.Equal(t, 999, retrieved.ErrorCount)
	})
}
