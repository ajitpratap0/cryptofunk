package agents

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultShutdownConfig verifies the default shutdown configuration values
func TestDefaultShutdownConfig(t *testing.T) {
	config := DefaultShutdownConfig()

	assert.Equal(t, DefaultShutdownTimeout, config.Timeout)
	assert.Equal(t, DefaultNATSDrainTimeout, config.NATSDrainTimeout)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 5*time.Second, config.NATSDrainTimeout)
}

// TestShutdownConfigValues verifies the default constant values
func TestShutdownConfigValues(t *testing.T) {
	assert.Equal(t, 10*time.Second, DefaultShutdownTimeout)
	assert.Equal(t, 5*time.Second, DefaultNATSDrainTimeout)
}

// TestBaseAgentShutdownConfig tests the shutdown config getter and setter
func TestBaseAgentShutdownConfig(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "shutdown-config-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0) // 0 disables metrics server
	require.NotNil(t, agent)

	// Verify default shutdown config is set
	shutdownConfig := agent.GetShutdownConfig()
	assert.Equal(t, DefaultShutdownTimeout, shutdownConfig.Timeout)
	assert.Equal(t, DefaultNATSDrainTimeout, shutdownConfig.NATSDrainTimeout)

	// Test setting custom shutdown config
	customConfig := ShutdownConfig{
		Timeout:          30 * time.Second,
		NATSDrainTimeout: 10 * time.Second,
	}
	agent.SetShutdownConfig(customConfig)

	updatedConfig := agent.GetShutdownConfig()
	assert.Equal(t, 30*time.Second, updatedConfig.Timeout)
	assert.Equal(t, 10*time.Second, updatedConfig.NATSDrainTimeout)
}

// TestBaseAgentShutdownWithoutNATS tests shutdown when NATS is not connected
func TestBaseAgentShutdownWithoutNATS(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "shutdown-no-nats-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0)
	require.NotNil(t, agent)

	// Initialize the agent's context
	ctx := context.Background()
	err := agent.Initialize(ctx)
	require.NoError(t, err)

	// Shutdown should complete without error even without NATS
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = agent.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// TestBaseAgentShutdownWithTimeout tests that shutdown respects timeout
func TestBaseAgentShutdownWithTimeout(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "shutdown-timeout-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0)
	require.NotNil(t, agent)

	// Initialize
	ctx := context.Background()
	err := agent.Initialize(ctx)
	require.NoError(t, err)

	// Add an in-flight operation that will block
	agent.AddInFlightOperation()

	// Start shutdown with a short timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Shutdown should timeout since we have a blocking in-flight operation
	err = agent.Shutdown(shutdownCtx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Clean up - mark operation as done
	agent.DoneInFlightOperation()
}

// TestBaseAgentInFlightOperations tests the in-flight operation tracking
func TestBaseAgentInFlightOperations(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "in-flight-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0)
	require.NotNil(t, agent)

	// Initialize
	ctx := context.Background()
	err := agent.Initialize(ctx)
	require.NoError(t, err)

	// Test adding and completing operations
	var completed sync.WaitGroup
	completed.Add(3)

	for i := 0; i < 3; i++ {
		agent.AddInFlightOperation()
		go func() {
			defer agent.DoneInFlightOperation()
			time.Sleep(50 * time.Millisecond)
			completed.Done()
		}()
	}

	// Wait for all operations to complete
	completed.Wait()

	// Shutdown should complete quickly since all operations are done
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = agent.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// TestBaseAgentShutdownWithNATSDrain tests shutdown with a real NATS connection
func TestBaseAgentShutdownWithNATSDrain(t *testing.T) {
	// Skip if NATS server cannot be started (integration test)
	if testing.Short() {
		t.Skip("Skipping NATS integration test in short mode")
	}

	// Start embedded NATS server
	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1, // Random available port
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Skipf("Could not start NATS server: %v", err)
	}

	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "nats-drain-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0)
	require.NotNil(t, agent)

	// Initialize
	ctx := context.Background()
	err = agent.Initialize(ctx)
	require.NoError(t, err)

	// Connect to NATS
	natsURL := ns.ClientURL()
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)

	// Setup control subscription using the public method
	err = agent.SetupControlSubscription(natsURL, "test.control")
	require.NoError(t, err)

	// Verify NATS is connected
	assert.NotNil(t, agent.GetNATSConnection())

	// Close the standalone connection we opened
	nc.Close()

	// Shutdown should drain NATS properly
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = agent.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// TestHeartbeatPublisherShutdown tests that heartbeat publisher stops gracefully
func TestHeartbeatPublisherShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping NATS integration test in short mode")
	}

	// Start embedded NATS server
	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1,
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Skipf("Could not start NATS server: %v", err)
	}

	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Connect to NATS
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	// Create heartbeat publisher with short interval
	heartbeatConfig := HeartbeatConfig{
		Interval: 100 * time.Millisecond,
		Topic:    "test.heartbeat",
	}

	publisher := NewHeartbeatPublisher("test-agent", "test", heartbeatConfig, log)
	publisher.SetNATSConn(nc)

	// Track received heartbeats
	var heartbeatCount int
	var mu sync.Mutex

	sub, err := nc.Subscribe("test.heartbeat", func(msg *nats.Msg) {
		mu.Lock()
		heartbeatCount++
		mu.Unlock()
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Start publisher
	publisher.Start()
	assert.True(t, publisher.IsRunning())

	// Wait for some heartbeats
	time.Sleep(350 * time.Millisecond)

	// Stop publisher
	publisher.Stop()

	// Give time for stop to complete
	time.Sleep(50 * time.Millisecond)

	assert.False(t, publisher.IsRunning())

	// Verify we received some heartbeats
	mu.Lock()
	count := heartbeatCount
	mu.Unlock()

	assert.GreaterOrEqual(t, count, 2, "Should have received at least 2 heartbeats")
}

// TestShutdownSequence tests the full shutdown sequence order
func TestShutdownSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping NATS integration test in short mode")
	}

	// Start embedded NATS server
	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1,
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Skipf("Could not start NATS server: %v", err)
	}

	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "sequence-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0)
	require.NotNil(t, agent)

	// Initialize
	ctx := context.Background()
	err = agent.Initialize(ctx)
	require.NoError(t, err)

	// Setup NATS control subscription
	err = agent.SetupControlSubscription(ns.ClientURL(), "test.control")
	require.NoError(t, err)

	// Add some in-flight operations that complete quickly
	for i := 0; i < 3; i++ {
		agent.AddInFlightOperation()
		go func() {
			defer agent.DoneInFlightOperation()
			time.Sleep(10 * time.Millisecond)
		}()
	}

	// Perform shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startTime := time.Now()
	err = agent.Shutdown(shutdownCtx)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	// Shutdown should complete relatively quickly (well under timeout)
	assert.Less(t, duration, 5*time.Second)
}

// TestConcurrentShutdown tests that shutdown handles concurrent operations correctly
func TestConcurrentShutdown(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "concurrent-shutdown-test",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 0)
	require.NotNil(t, agent)

	// Initialize
	ctx := context.Background()
	err := agent.Initialize(ctx)
	require.NoError(t, err)

	// Start multiple operations concurrently
	var wg sync.WaitGroup
	operationCount := 10

	for i := 0; i < operationCount; i++ {
		wg.Add(1)
		agent.AddInFlightOperation()

		go func(id int) {
			defer wg.Done()
			defer agent.DoneInFlightOperation()

			// Simulate varying operation durations
			time.Sleep(time.Duration(10*(id+1)) * time.Millisecond)
		}(i)
	}

	// Wait for all operations to start
	time.Sleep(5 * time.Millisecond)

	// Perform shutdown - it should wait for all operations
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = agent.Shutdown(shutdownCtx)
	assert.NoError(t, err)

	// All operations should have completed
	wg.Wait()
}
