//go:build integration

package orchestrator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// TestPauseResume tests basic pause and resume functionality
func TestPauseResume(t *testing.T) {
	// Setup test database
	tc := testhelpers.SetupTestDatabase(t)
	defer tc.Cleanup()

	// Apply migrations
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	// Start embedded NATS server for testing
	ns := testhelpers.StartNATSServer(t)
	defer ns.Shutdown()

	// Create orchestrator config
	config := &OrchestratorConfig{
		Name:                "test-orchestrator",
		NATSUrl:             ns.ClientURL(),
		SignalTopic:         "test.signals",
		DecisionTopic:       "test.decisions",
		HeartbeatTopic:      "test.heartbeat",
		StepInterval:        1 * time.Second,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}

	// Create orchestrator
	log := zerolog.Nop()
	orch, err := NewOrchestrator(config, log, tc.DB, 0)
	require.NoError(t, err)

	// Initialize orchestrator
	ctx := context.Background()
	err = orch.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = orch.Shutdown(shutdownCtx)
	}()

	// Test initial state (not paused)
	assert.False(t, orch.IsPaused())

	// Test pause
	err = orch.Pause()
	require.NoError(t, err)
	assert.True(t, orch.IsPaused())

	// Test double pause (should error)
	err = orch.Pause()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already paused")

	// Verify pause state persisted to database
	state, err := tc.DB.GetOrchestratorState(ctx)
	require.NoError(t, err)
	assert.True(t, state.Paused)
	assert.NotNil(t, state.PausedAt)
	assert.NotNil(t, state.PausedBy)
	assert.Equal(t, "api", *state.PausedBy)

	// Test resume
	err = orch.Resume()
	require.NoError(t, err)
	assert.False(t, orch.IsPaused())

	// Test double resume (should error)
	err = orch.Resume()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not paused")

	// Verify resume state persisted to database
	state, err = tc.DB.GetOrchestratorState(ctx)
	require.NoError(t, err)
	assert.False(t, state.Paused)
	assert.NotNil(t, state.ResumedAt)
}

// TestPauseNATSBroadcast tests that pause/resume events are broadcast via NATS
func TestPauseNATSBroadcast(t *testing.T) {
	// Setup test database
	tc := testhelpers.SetupTestDatabase(t)
	defer tc.Cleanup()

	// Apply migrations
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	// Start embedded NATS server
	ns := testhelpers.StartNATSServer(t)
	defer ns.Shutdown()

	// Connect test NATS client
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	// Subscribe to control topic
	controlTopic := "cryptofunk.orchestrator.control"
	eventsChan := make(chan map[string]interface{}, 10)

	sub, err := nc.Subscribe(controlTopic, func(msg *nats.Msg) {
		var event map[string]interface{}
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			t.Errorf("Failed to unmarshal event: %v", err)
			return
		}
		eventsChan <- event
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Flush to ensure subscription is registered
	err = nc.Flush()
	require.NoError(t, err)

	// Create orchestrator
	config := &OrchestratorConfig{
		Name:                "test-orchestrator",
		NATSUrl:             ns.ClientURL(),
		SignalTopic:         "test.signals",
		DecisionTopic:       "test.decisions",
		HeartbeatTopic:      "test.heartbeat",
		StepInterval:        1 * time.Second,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}

	log := zerolog.Nop()
	orch, err := NewOrchestrator(config, log, tc.DB, 0)
	require.NoError(t, err)

	ctx := context.Background()
	err = orch.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = orch.Shutdown(shutdownCtx)
	}()

	// Pause and check for event
	err = orch.Pause()
	require.NoError(t, err)

	// Wait for pause event
	select {
	case event := <-eventsChan:
		assert.Equal(t, "trading_paused", event["event"])
		assert.Equal(t, "manual_pause", event["reason"])
		assert.NotNil(t, event["timestamp"])
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for pause event")
	}

	// Resume and check for event
	err = orch.Resume()
	require.NoError(t, err)

	// Wait for resume event
	select {
	case event := <-eventsChan:
		assert.Equal(t, "trading_resumed", event["event"])
		assert.NotNil(t, event["timestamp"])
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for resume event")
	}
}

// TestPauseBlocksDecisions tests that paused orchestrator doesn't make trading decisions
func TestPauseBlocksDecisions(t *testing.T) {
	// Setup test database
	tc := testhelpers.SetupTestDatabase(t)
	defer tc.Cleanup()

	// Apply migrations
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	// Start embedded NATS server
	ns := testhelpers.StartNATSServer(t)
	defer ns.Shutdown()

	// Connect test NATS client
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	// Subscribe to decision topic to count decisions
	decisionTopic := "test.decisions"
	decisionCount := 0
	decisionMutex := make(chan struct{}, 1)
	decisionMutex <- struct{}{} // Initialize mutex

	sub, err := nc.Subscribe(decisionTopic, func(msg *nats.Msg) {
		<-decisionMutex
		decisionCount++
		decisionMutex <- struct{}{}
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Create orchestrator
	config := &OrchestratorConfig{
		Name:                "test-orchestrator",
		NATSUrl:             ns.ClientURL(),
		SignalTopic:         "test.signals",
		DecisionTopic:       decisionTopic,
		HeartbeatTopic:      "test.heartbeat",
		StepInterval:        100 * time.Millisecond,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}

	log := zerolog.Nop()
	orch, err := NewOrchestrator(config, log, tc.DB, 0)
	require.NoError(t, err)

	ctx := context.Background()
	err = orch.Initialize(ctx)
	require.NoError(t, err)

	// Start orchestrator in background
	orchCtx, orchCancel := context.WithCancel(ctx)
	defer orchCancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- orch.Run(orchCtx)
	}()

	// Let it run for a bit (should generate decisions when not paused)
	time.Sleep(300 * time.Millisecond)

	<-decisionMutex
	decisionsBeforePause := decisionCount
	decisionMutex <- struct{}{}

	t.Logf("Decisions before pause: %d", decisionsBeforePause)
	// Note: May be 0 if no signals were received

	// Pause trading
	err = orch.Pause()
	require.NoError(t, err)

	<-decisionMutex
	decisionCount = 0 // Reset counter
	decisionMutex <- struct{}{}

	// Let it run while paused (should not generate decisions)
	time.Sleep(300 * time.Millisecond)

	<-decisionMutex
	decisionsWhilePaused := decisionCount
	decisionMutex <- struct{}{}

	t.Logf("Decisions while paused: %d", decisionsWhilePaused)
	assert.Equal(t, 0, decisionsWhilePaused, "No decisions should be made while paused")

	// Resume trading
	err = orch.Resume()
	require.NoError(t, err)

	<-decisionMutex
	decisionCount = 0 // Reset counter
	decisionMutex <- struct{}{}

	// Let it run after resume (should generate decisions again)
	time.Sleep(300 * time.Millisecond)

	<-decisionMutex
	decisionsAfterResume := decisionCount
	decisionMutex <- struct{}{}

	t.Logf("Decisions after resume: %d", decisionsAfterResume)
	// Note: May be 0 if no signals were received

	// Stop orchestrator
	orchCancel()

	select {
	case err := <-errChan:
		// Context cancellation is expected
		if err != context.Canceled {
			t.Errorf("Unexpected error from orchestrator: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for orchestrator to stop")
	}

	// Shutdown orchestrator
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = orch.Shutdown(shutdownCtx)
	require.NoError(t, err)
}

// TestStatePersistence tests that pause state survives orchestrator restart
func TestStatePersistence(t *testing.T) {
	// Setup test database
	tc := testhelpers.SetupTestDatabase(t)
	defer tc.Cleanup()

	// Apply migrations
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	// Start embedded NATS server
	ns := testhelpers.StartNATSServer(t)
	defer ns.Shutdown()

	ctx := context.Background()
	log := zerolog.Nop()

	config := &OrchestratorConfig{
		Name:                "test-orchestrator",
		NATSUrl:             ns.ClientURL(),
		SignalTopic:         "test.signals",
		DecisionTopic:       "test.decisions",
		HeartbeatTopic:      "test.heartbeat",
		StepInterval:        1 * time.Second,
		MinConsensus:        0.6,
		MinConfidence:       0.5,
		MaxSignalAge:        5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}

	// Create first orchestrator and pause it
	orch1, err := NewOrchestrator(config, log, tc.DB, 0)
	require.NoError(t, err)

	err = orch1.Initialize(ctx)
	require.NoError(t, err)

	err = orch1.Pause()
	require.NoError(t, err)
	assert.True(t, orch1.IsPaused())

	// Shutdown first orchestrator
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = orch1.Shutdown(shutdownCtx)
	cancel()
	require.NoError(t, err)

	// Create second orchestrator (should load paused state)
	orch2, err := NewOrchestrator(config, log, tc.DB, 0)
	require.NoError(t, err)

	err = orch2.Initialize(ctx)
	require.NoError(t, err)

	// Verify state was loaded as paused
	assert.True(t, orch2.IsPaused(), "Orchestrator should load paused state from database")

	// Shutdown second orchestrator
	shutdownCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = orch2.Shutdown(shutdownCtx)
	cancel()
	require.NoError(t, err)
}
