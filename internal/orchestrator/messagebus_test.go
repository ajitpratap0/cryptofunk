package orchestrator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTestNATSServer starts an embedded NATS server for testing
func startTestNATSServer(t *testing.T) *server.Server {
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random port
	}

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return ns
}

// setupTestMessageBus creates a test message bus
func setupTestMessageBus(t *testing.T) (*MessageBus, *server.Server) {
	ns := startTestNATSServer(t)

	config := MessageBusConfig{
		NATSURL: ns.ClientURL(),
		Prefix:  "test.agents.",
	}

	mb, err := NewMessageBus(config)
	require.NoError(t, err)
	require.NotNil(t, mb)

	return mb, ns
}

// TestNewMessageBus tests message bus initialization
func TestNewMessageBus(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	config := MessageBusConfig{
		NATSURL: ns.ClientURL(),
		Prefix:  "test.",
	}

	mb, err := NewMessageBus(config)
	require.NoError(t, err)
	require.NotNil(t, mb)
	assert.Equal(t, "test.", mb.prefix)
	assert.True(t, mb.nc.IsConnected())

	_ = mb.Close() // Test cleanup
}

// TestNewMessageBus_DefaultPrefix tests default prefix
func TestNewMessageBus_DefaultPrefix(t *testing.T) {
	ns := startTestNATSServer(t)
	defer ns.Shutdown()

	config := MessageBusConfig{
		NATSURL: ns.ClientURL(),
		Prefix:  "",
	}

	mb, err := NewMessageBus(config)
	require.NoError(t, err)
	assert.Equal(t, "agents.", mb.prefix)

	_ = mb.Close() // Test cleanup
}

// TestAgentMessage_Helpers tests message helper methods
func TestAgentMessage_Helpers(t *testing.T) {
	payload := map[string]interface{}{
		"signal": "BUY",
		"price":  50000.0,
	}

	msg, err := NewAgentMessage("sender", "receiver", "trading", payload)
	require.NoError(t, err)
	assert.Equal(t, "sender", msg.From)
	assert.Equal(t, "receiver", msg.To)
	assert.Equal(t, "trading", msg.Topic)
	assert.Equal(t, 5, msg.Priority)
	assert.NotNil(t, msg.Metadata)

	// Test fluent API
	msg.WithType(MessageTypeRequest).
		WithPriority(9).
		WithTTL(5*time.Minute).
		WithMetadata("exchange", "binance")

	assert.Equal(t, MessageTypeRequest, msg.Type)
	assert.Equal(t, 9, msg.Priority)
	assert.Equal(t, 5*time.Minute, msg.TTL)
	assert.Equal(t, "binance", msg.Metadata["exchange"])
}

// TestSend tests sending a message to a specific agent
func TestSend(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Subscribe to receive messages
	var receivedMsg *AgentMessage
	var wg sync.WaitGroup
	wg.Add(1)

	sub, err := mb.Subscribe("receiver", "trading", func(msg *AgentMessage) error {
		receivedMsg = msg
		wg.Done()
		return nil
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send message
	payload := map[string]string{"signal": "BUY"}
	msg, err := NewAgentMessage("sender", "receiver", "trading", payload)
	require.NoError(t, err)

	err = mb.Send(ctx, msg)
	require.NoError(t, err)

	// Wait for message
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Verify message
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "sender", receivedMsg.From)
	assert.Equal(t, "receiver", receivedMsg.To)
	assert.Equal(t, "trading", receivedMsg.Topic)
}

// TestBroadcast tests broadcasting to all agents
func TestBroadcast(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Subscribe multiple agents to broadcasts
	var receivedCount sync.WaitGroup
	receivedCount.Add(3)

	agents := []string{"agent1", "agent2", "agent3"}
	for range agents {
		sub, err := mb.SubscribeBroadcasts("announcements", func(msg *AgentMessage) error {
			receivedCount.Done()
			return nil
		})
		require.NoError(t, err)
		defer func() { _ = sub.Unsubscribe() }() // Test cleanup
	}

	// Give subscriptions time to establish
	time.Sleep(100 * time.Millisecond)

	// Broadcast message
	msg, err := NewAgentMessage("orchestrator", "*", "announcements", map[string]string{
		"message": "System update",
	})
	require.NoError(t, err)

	err = mb.Broadcast(ctx, msg)
	require.NoError(t, err)

	// Wait for all agents to receive
	done := make(chan struct{})
	go func() {
		receivedCount.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - all 3 agents received the broadcast
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for broadcasts")
	}
}

// TestRequest tests request-reply pattern
func TestRequest(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Set up responder
	sub, err := mb.Subscribe("responder", "query", func(msg *AgentMessage) error {
		// Send reply using mb.Reply
		return mb.Reply(context.Background(), msg, map[string]string{"result": "success"})
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Send request
	request, err := NewAgentMessage("requester", "responder", "query", map[string]string{
		"question": "What is the price?",
	})
	require.NoError(t, err)

	reply, err := mb.Request(ctx, request, 2*time.Second)
	require.NoError(t, err)
	require.NotNil(t, reply)
	assert.Equal(t, MessageTypeReply, reply.Type)
	assert.Equal(t, "responder", reply.From)
}

// TestRequest_Timeout tests request timeout
func TestRequest_Timeout(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// No responder - should timeout
	request, err := NewAgentMessage("requester", "nonexistent", "query", map[string]string{
		"question": "Anyone there?",
	})
	require.NoError(t, err)

	_, err = mb.Request(ctx, request, 100*time.Millisecond)
	assert.Error(t, err)
	// Error can be timeout or no responders
	assert.True(t, err.Error() == "request failed: nats: timeout" ||
		err.Error() == "request failed: nats: no responders available for request")
}

// TestMessageBusSubscribe tests subscribing to specific topic
func TestMessageBusSubscribe(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Track received messages
	var receivedMessages []*AgentMessage
	var mu sync.Mutex

	sub, err := mb.Subscribe("receiver", "trading", func(msg *AgentMessage) error {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send multiple messages
	for i := 0; i < 3; i++ {
		msg, _ := NewAgentMessage("sender", "receiver", "trading", map[string]int{"count": i})
		_ = mb.Send(ctx, msg) // Benchmark - error acceptable
	}

	// Send message to different topic (should not be received)
	wrongMsg, _ := NewAgentMessage("sender", "receiver", "market_data", map[string]string{"data": "test"})
	_ = mb.Send(ctx, wrongMsg) // Test setup - error expected

	// Wait for messages
	time.Sleep(200 * time.Millisecond)

	// Verify only trading messages were received
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedMessages, 3)
	for _, msg := range receivedMessages {
		assert.Equal(t, "trading", msg.Topic)
	}
}

// TestSubscribeAll tests subscribing to all topics for an agent
func TestSubscribeAll(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Track received messages
	var receivedMessages []*AgentMessage
	var mu sync.Mutex

	sub, err := mb.SubscribeAll("receiver", func(msg *AgentMessage) error {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send messages to different topics
	topics := []string{"trading", "market_data", "alerts"}
	for _, topic := range topics {
		msg, _ := NewAgentMessage("sender", "receiver", topic, map[string]string{"topic": topic})
		_ = mb.Send(ctx, msg) // Benchmark - error acceptable
	}

	// Send message to different agent (should not be received)
	wrongMsg, _ := NewAgentMessage("sender", "other-agent", "trading", map[string]string{"data": "test"})
	_ = mb.Send(ctx, wrongMsg) // Test setup - error expected

	// Wait for messages
	time.Sleep(200 * time.Millisecond)

	// Verify all messages to receiver were received
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedMessages, 3)
}

// TestMessageTTL tests message TTL expiration
func TestMessageTTL(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Track received messages
	var receivedCount int
	var mu sync.Mutex

	sub, err := mb.Subscribe("receiver", "trading", func(msg *AgentMessage) error {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send message with short TTL
	msg, _ := NewAgentMessage("sender", "receiver", "trading", map[string]string{"data": "test"})
	msg.WithTTL(100 * time.Millisecond)
	msg.Timestamp = time.Now().Add(-200 * time.Millisecond) // Already expired

	_ = mb.Send(ctx, msg)

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Message should have been filtered out due to TTL
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 0, receivedCount)
}

// TestMessageBusPriorities tests message priority levels
func TestMessageBusPriorities(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Track received priorities
	var priorities []int
	var mu sync.Mutex

	sub, err := mb.Subscribe("receiver", "trading", func(msg *AgentMessage) error {
		mu.Lock()
		priorities = append(priorities, msg.Priority)
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send messages with different priorities
	testPriorities := []int{0, 5, 9}
	for _, priority := range testPriorities {
		msg, _ := NewAgentMessage("sender", "receiver", "trading", map[string]int{"priority": priority})
		msg.WithPriority(priority)
		_ = mb.Send(ctx, msg) // Benchmark - error acceptable
	}

	// Wait for messages
	time.Sleep(200 * time.Millisecond)

	// Verify priorities
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, testPriorities, priorities)
}

// TestMessageTypes tests different message types
func TestMessageTypes(t *testing.T) {
	types := []MessageType{
		MessageTypeRequest,
		MessageTypeReply,
		MessageTypeNotification,
		MessageTypeBroadcast,
		MessageTypeCommand,
		MessageTypeEvent,
	}

	for _, msgType := range types {
		msg, _ := NewAgentMessage("sender", "receiver", "test", map[string]string{"test": "data"})
		msg.WithType(msgType)
		assert.Equal(t, msgType, msg.Type)
	}
}

// TestSubscription_IsValid tests subscription validity
func TestSubscription_IsValid(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	sub, err := mb.Subscribe("agent", "topic", func(msg *AgentMessage) error {
		return nil
	})
	require.NoError(t, err)

	// Should be valid initially
	assert.True(t, sub.IsValid())

	// Should be invalid after unsubscribe
	_ = sub.Unsubscribe() // Test cleanup
	assert.False(t, sub.IsValid())
}

// TestMessageBusGetStats tests statistics retrieval
func TestMessageBusGetStats(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	stats := mb.GetStats()

	assert.NotNil(t, stats)
	assert.Equal(t, true, stats["connected"])
	assert.NotNil(t, stats["status"])
	assert.NotNil(t, stats["connected_url"])
}

// TestDefaultMessageBusConfig tests default configuration
func TestDefaultMessageBusConfig(t *testing.T) {
	config := DefaultMessageBusConfig()

	assert.Equal(t, "nats://localhost:4222", config.NATSURL)
	assert.Equal(t, "agents.", config.Prefix)
}

// TestErrorHandling tests error handling in message handler
func TestErrorHandling(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Subscribe with error-throwing handler
	sub, err := mb.Subscribe("receiver", "trading", func(msg *AgentMessage) error {
		return assert.AnError // Simulate handler error
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send message
	msg, _ := NewAgentMessage("sender", "receiver", "trading", map[string]string{"test": "data"})
	err = mb.Send(ctx, msg)
	require.NoError(t, err)

	// Handler error should be logged but not prevent message sending
	time.Sleep(100 * time.Millisecond)
}

// TestConcurrentMessaging tests concurrent message sending
func TestConcurrentMessaging(t *testing.T) {
	mb, ns := setupTestMessageBus(t)
	defer ns.Shutdown()
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()

	// Track received messages
	var receivedCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	expectedCount := 100
	wg.Add(expectedCount)

	sub, err := mb.Subscribe("receiver", "trading", func(msg *AgentMessage) error {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		wg.Done()
		return nil
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }() // Test cleanup

	// Send many messages concurrently
	var sendWg sync.WaitGroup
	for i := 0; i < expectedCount; i++ {
		sendWg.Add(1)
		go func(n int) {
			defer sendWg.Done()
			msg, _ := NewAgentMessage("sender", "receiver", "trading", map[string]int{"n": n})
			_ = mb.Send(ctx, msg) // Benchmark - error acceptable
		}(i)
	}

	// Wait for all sends to complete
	sendWg.Wait()

	// Wait for all receives
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for concurrent messages")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, expectedCount, receivedCount)
}

// Benchmark tests

func BenchmarkSend(b *testing.B) {
	ns := startTestNATSServer(&testing.T{})
	defer ns.Shutdown()

	config := MessageBusConfig{
		NATSURL: ns.ClientURL(),
		Prefix:  "bench.",
	}

	mb, _ := NewMessageBus(config)
	defer func() { _ = mb.Close() }() // Test cleanup

	ctx := context.Background()
	msg, _ := NewAgentMessage("sender", "receiver", "test", map[string]string{"data": "benchmark"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mb.Send(ctx, msg) // Benchmark - error acceptable
	}
}

func BenchmarkRequest(b *testing.B) {
	ns := startTestNATSServer(&testing.T{})
	defer ns.Shutdown()

	config := MessageBusConfig{
		NATSURL: ns.ClientURL(),
		Prefix:  "bench.",
	}

	mb, _ := NewMessageBus(config)
	defer func() { _ = mb.Close() }() // Test cleanup

	// Set up responder
	_, _ = mb.Subscribe("responder", "query", func(msg *AgentMessage) error { // Test subscription
		return mb.Reply(context.Background(), msg, map[string]string{"result": "ok"})
	})

	ctx := context.Background()
	request, _ := NewAgentMessage("sender", "responder", "query", map[string]string{"q": "test"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mb.Request(ctx, request, 1*time.Second) // Benchmark - error acceptable
	}
}
