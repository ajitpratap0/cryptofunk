package agents

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// startTestNATSServer starts an embedded NATS server for testing
func startTestNATSServer(t *testing.T) (*server.Server, string) {
	t.Helper()
	opts := &server.Options{
		Port: -1, // Random port
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS server: %v", err)
	}
	go ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return ns, ns.ClientURL()
}

func TestNewHeartbeatPublisher(t *testing.T) {
	log := zerolog.Nop()
	config := DefaultHeartbeatConfig()

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)

	if publisher == nil {
		t.Fatal("NewHeartbeatPublisher returned nil")
	}
	if publisher.agentName != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", publisher.agentName)
	}
	if publisher.agentType != "technical" {
		t.Errorf("Expected agent type 'technical', got '%s'", publisher.agentType)
	}
	if publisher.IsRunning() {
		t.Error("Publisher should not be running initially")
	}
}

func TestDefaultHeartbeatConfig(t *testing.T) {
	config := DefaultHeartbeatConfig()

	if config.Interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", config.Interval)
	}
	if config.Topic != "agents.heartbeat" {
		t.Errorf("Expected topic 'agents.heartbeat', got '%s'", config.Topic)
	}
}

func TestHeartbeatPublisher_StartWithoutNATS(t *testing.T) {
	log := zerolog.Nop()
	config := DefaultHeartbeatConfig()

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)

	// Start without NATS connection - should not panic
	publisher.Start()

	// Should not be running without NATS
	if publisher.IsRunning() {
		t.Error("Publisher should not be running without NATS connection")
	}
}

func TestHeartbeatPublisher_StartStop(t *testing.T) {
	ns, natsURL := startTestNATSServer(t)
	defer ns.Shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log := zerolog.Nop()
	config := HeartbeatConfig{
		Interval: 100 * time.Millisecond,
		Topic:    "test.heartbeat",
	}

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)
	publisher.SetNATSConn(nc)

	// Subscribe to heartbeat topic to verify messages
	var received []HeartbeatMessage
	var mu sync.Mutex

	sub, err := nc.Subscribe(config.Topic, func(msg *nats.Msg) {
		var hb HeartbeatMessage
		if err := json.Unmarshal(msg.Data, &hb); err == nil {
			mu.Lock()
			received = append(received, hb)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Start publisher
	publisher.Start()

	if !publisher.IsRunning() {
		t.Error("Publisher should be running after Start()")
	}

	// Wait for at least 2 heartbeats (immediate + 1 interval)
	time.Sleep(250 * time.Millisecond)

	// Stop publisher
	publisher.Stop()

	// Wait a bit for stop to take effect
	time.Sleep(50 * time.Millisecond)

	if publisher.IsRunning() {
		t.Error("Publisher should not be running after Stop()")
	}

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count < 2 {
		t.Errorf("Expected at least 2 heartbeats, got %d", count)
	}

	// Verify heartbeat content
	mu.Lock()
	if count > 0 {
		hb := received[0]
		if hb.AgentName != "test-agent" {
			t.Errorf("Expected agent name 'test-agent', got '%s'", hb.AgentName)
		}
		if hb.AgentType != "technical" {
			t.Errorf("Expected agent type 'technical', got '%s'", hb.AgentType)
		}
		if hb.Status != "healthy" {
			t.Errorf("Expected status 'healthy', got '%s'", hb.Status)
		}
		if hb.Timestamp.IsZero() {
			t.Error("Timestamp should not be zero")
		}
	}
	mu.Unlock()
}

func TestHeartbeatPublisher_DoubleStart(t *testing.T) {
	ns, natsURL := startTestNATSServer(t)
	defer ns.Shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log := zerolog.Nop()
	config := HeartbeatConfig{
		Interval: 1 * time.Second,
		Topic:    "test.heartbeat",
	}

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)
	publisher.SetNATSConn(nc)

	// Start twice - should not panic
	publisher.Start()
	publisher.Start()

	if !publisher.IsRunning() {
		t.Error("Publisher should be running")
	}

	publisher.Stop()
}

func TestHeartbeatPublisher_PublishNow(t *testing.T) {
	ns, natsURL := startTestNATSServer(t)
	defer ns.Shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log := zerolog.Nop()
	config := HeartbeatConfig{
		Interval: 1 * time.Hour, // Long interval to ensure no automatic publishing
		Topic:    "test.heartbeat",
	}

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)
	publisher.SetNATSConn(nc)

	var received bool
	var mu sync.Mutex

	sub, err := nc.Subscribe(config.Topic, func(msg *nats.Msg) {
		mu.Lock()
		received = true
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Manually publish (without starting the publisher loop)
	publisher.PublishNow()

	// Wait for message
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !received {
		t.Error("Expected to receive heartbeat from PublishNow()")
	}
}

func TestHeartbeatPublisher_PublishWithStatus(t *testing.T) {
	ns, natsURL := startTestNATSServer(t)
	defer ns.Shutdown()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log := zerolog.Nop()
	config := HeartbeatConfig{
		Interval: 1 * time.Hour,
		Topic:    "test.heartbeat",
	}

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)
	publisher.SetNATSConn(nc)

	var receivedStatus string
	var mu sync.Mutex

	sub, err := nc.Subscribe(config.Topic, func(msg *nats.Msg) {
		var hb HeartbeatMessage
		if err := json.Unmarshal(msg.Data, &hb); err == nil {
			mu.Lock()
			receivedStatus = hb.Status
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Publish with custom status
	publisher.PublishWithStatus("shutting_down")

	// Wait for message
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if receivedStatus != "shutting_down" {
		t.Errorf("Expected status 'shutting_down', got '%s'", receivedStatus)
	}
}

func TestHeartbeatPublisher_PublishWithoutNATS(t *testing.T) {
	log := zerolog.Nop()
	config := DefaultHeartbeatConfig()

	publisher := NewHeartbeatPublisher("test-agent", "technical", config, log)

	// These should not panic without NATS connection
	publisher.PublishNow()
	publisher.PublishWithStatus("test")
}
