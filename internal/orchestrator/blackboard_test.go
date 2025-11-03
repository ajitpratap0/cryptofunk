package orchestrator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestBlackboard creates a test blackboard with miniredis
func setupTestBlackboard(t *testing.T) (*Blackboard, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	bb := &Blackboard{
		client: client,
		prefix: "test:blackboard:",
	}

	return bb, mr
}

// TestNewBlackboard tests blackboard initialization
func TestNewBlackboard(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := BlackboardConfig{
		RedisURL: mr.Addr(),
		Prefix:   "test:",
	}

	bb, err := NewBlackboard(config)
	require.NoError(t, err)
	require.NotNil(t, bb)
	assert.Equal(t, "test:", bb.prefix)

	// Test connection
	ctx := context.Background()
	err = bb.client.Ping(ctx).Err()
	assert.NoError(t, err)

	bb.Close()
}

// TestNewBlackboard_DefaultPrefix tests default prefix
func TestNewBlackboard_DefaultPrefix(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := BlackboardConfig{
		RedisURL: mr.Addr(),
		Prefix:   "",
	}

	bb, err := NewBlackboard(config)
	require.NoError(t, err)
	assert.Equal(t, "blackboard:", bb.prefix)

	bb.Close()
}

// TestBlackboardMessage_Helpers tests message helper methods
func TestBlackboardMessage_Helpers(t *testing.T) {
	content := map[string]interface{}{
		"signal": "BUY",
		"price":  50000.0,
	}

	msg, err := NewMessage("signals", "technical-agent", content)
	require.NoError(t, err)
	assert.Equal(t, "signals", msg.Topic)
	assert.Equal(t, "technical-agent", msg.AgentName)
	assert.Equal(t, PriorityNormal, msg.Priority)
	assert.Empty(t, msg.Tags)
	assert.NotNil(t, msg.Metadata)

	// Test fluent API
	msg.WithPriority(PriorityHigh).
		WithTags("crypto", "BTC").
		WithMetadata("exchange", "binance").
		WithTTL(5 * time.Minute)

	assert.Equal(t, PriorityHigh, msg.Priority)
	assert.Contains(t, msg.Tags, "crypto")
	assert.Contains(t, msg.Tags, "BTC")
	assert.Equal(t, "binance", msg.Metadata["exchange"])
	assert.NotNil(t, msg.ExpiresAt)
	assert.True(t, msg.ExpiresAt.After(time.Now()))
}

// TestPost tests message posting
func TestPost(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	content := map[string]interface{}{
		"signal": "BUY",
		"symbol": "BTC/USDT",
	}

	msg, err := NewMessage("signals", "technical-agent", content)
	require.NoError(t, err)

	err = bb.Post(ctx, msg)
	require.NoError(t, err)

	// Verify message was stored
	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.False(t, msg.CreatedAt.IsZero())

	// Verify message can be retrieved
	messages, err := bb.GetByTopic(ctx, "signals", 10)
	require.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, msg.ID, messages[0].ID)
	assert.Equal(t, "signals", messages[0].Topic)
	assert.Equal(t, "technical-agent", messages[0].AgentName)
}

// TestPost_WithDefaults tests posting with default values
func TestPost_WithDefaults(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	msg := &BlackboardMessage{
		Topic:     "test",
		AgentName: "test-agent",
		Content:   json.RawMessage(`{"test": true}`),
	}

	err := bb.Post(ctx, msg)
	require.NoError(t, err)

	// Check defaults were set
	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.False(t, msg.CreatedAt.IsZero())
	assert.Equal(t, PriorityNormal, msg.Priority)
}

// TestPost_WithExpiration tests posting with TTL
func TestPost_WithExpiration(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	msg, err := NewMessage("test", "agent", map[string]string{"key": "value"})
	require.NoError(t, err)

	msg.WithTTL(1 * time.Second)

	err = bb.Post(ctx, msg)
	require.NoError(t, err)

	// Fast-forward time in miniredis
	mr.FastForward(2 * time.Second)

	// Message should be expired
	messages, err := bb.GetByTopic(ctx, "test", 10)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

// TestGetByTopic tests retrieving messages by topic
func TestGetByTopic(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post multiple messages to different topics
	for i := 0; i < 5; i++ {
		msg, err := NewMessage("signals", "agent", map[string]int{"value": i})
		require.NoError(t, err)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	for i := 0; i < 3; i++ {
		msg, err := NewMessage("market_data", "agent", map[string]int{"value": i})
		require.NoError(t, err)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
	}

	// Get messages by topic
	signalMessages, err := bb.GetByTopic(ctx, "signals", 10)
	require.NoError(t, err)
	assert.Len(t, signalMessages, 5)

	marketMessages, err := bb.GetByTopic(ctx, "market_data", 10)
	require.NoError(t, err)
	assert.Len(t, marketMessages, 3)

	// Test limit
	limitedMessages, err := bb.GetByTopic(ctx, "signals", 3)
	require.NoError(t, err)
	assert.Len(t, limitedMessages, 3)

	// Messages should be in reverse chronological order (most recent first)
	for i := 0; i < len(limitedMessages)-1; i++ {
		assert.True(t, limitedMessages[i].CreatedAt.After(limitedMessages[i+1].CreatedAt) ||
			limitedMessages[i].CreatedAt.Equal(limitedMessages[i+1].CreatedAt))
	}
}

// TestGetByTopicRange tests time-based message retrieval
func TestGetByTopicRange(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	now := time.Now()

	// Post messages at different times
	msg1, _ := NewMessage("test", "agent", map[string]string{"time": "old"})
	msg1.CreatedAt = now.Add(-2 * time.Hour)
	bb.Post(ctx, msg1)

	msg2, _ := NewMessage("test", "agent", map[string]string{"time": "recent"})
	msg2.CreatedAt = now.Add(-30 * time.Minute)
	bb.Post(ctx, msg2)

	msg3, _ := NewMessage("test", "agent", map[string]string{"time": "new"})
	msg3.CreatedAt = now
	bb.Post(ctx, msg3)

	// Query for messages in the last hour
	messages, err := bb.GetByTopicRange(ctx, "test", now.Add(-1*time.Hour), now.Add(1*time.Minute), 10)
	require.NoError(t, err)
	assert.Len(t, messages, 2) // msg2 and msg3
}

// TestGetByAgent tests retrieving messages by agent
func TestGetByAgent(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post messages from different agents
	for i := 0; i < 3; i++ {
		msg, err := NewMessage("signals", "technical-agent", map[string]int{"value": i})
		require.NoError(t, err)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		msg, err := NewMessage("signals", "sentiment-agent", map[string]int{"value": i})
		require.NoError(t, err)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
	}

	// Get messages by agent
	technicalMessages, err := bb.GetByAgent(ctx, "technical-agent", 10)
	require.NoError(t, err)
	assert.Len(t, technicalMessages, 3)

	sentimentMessages, err := bb.GetByAgent(ctx, "sentiment-agent", 10)
	require.NoError(t, err)
	assert.Len(t, sentimentMessages, 2)
}

// TestGetByPriority tests priority filtering
func TestGetByPriority(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post messages with different priorities
	priorities := []MessagePriority{
		PriorityLow,
		PriorityNormal,
		PriorityHigh,
		PriorityUrgent,
		PriorityNormal,
	}

	for i, priority := range priorities {
		msg, err := NewMessage("signals", "agent", map[string]int{"value": i})
		require.NoError(t, err)
		msg.WithPriority(priority)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	// Get high priority messages (High and Urgent)
	highPriorityMessages, err := bb.GetByPriority(ctx, "signals", PriorityHigh, 10)
	require.NoError(t, err)
	assert.Len(t, highPriorityMessages, 2)

	for _, msg := range highPriorityMessages {
		assert.GreaterOrEqual(t, msg.Priority, PriorityHigh)
	}
}

// TestSubscribe tests pub/sub functionality
func TestSubscribe(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Subscribe to a topic
	msgChan, err := bb.Subscribe(ctx, "signals")
	require.NoError(t, err)

	// Post a message in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		msg, _ := NewMessage("signals", "test-agent", map[string]string{"test": "data"})
		bb.Post(context.Background(), msg)
	}()

	// Wait for notification
	select {
	case receivedMsg := <-msgChan:
		assert.NotNil(t, receivedMsg)
		assert.Equal(t, "signals", receivedMsg.Topic)
		assert.Equal(t, "test-agent", receivedMsg.AgentName)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestClear tests clearing topic messages
func TestClear(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post several messages
	for i := 0; i < 5; i++ {
		msg, err := NewMessage("test", "agent", map[string]int{"value": i})
		require.NoError(t, err)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
	}

	// Verify messages exist
	messages, err := bb.GetByTopic(ctx, "test", 10)
	require.NoError(t, err)
	assert.Len(t, messages, 5)

	// Clear topic
	err = bb.Clear(ctx, "test")
	require.NoError(t, err)

	// Verify messages are gone
	messages, err = bb.GetByTopic(ctx, "test", 10)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

// TestClearExpired tests expired message cleanup
func TestClearExpired(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post message with short TTL
	msg1, _ := NewMessage("test", "agent", map[string]string{"msg": "1"})
	msg1.WithTTL(1 * time.Second)
	bb.Post(ctx, msg1)

	// Post message without TTL
	msg2, _ := NewMessage("test", "agent", map[string]string{"msg": "2"})
	bb.Post(ctx, msg2)

	// Fast-forward time
	mr.FastForward(2 * time.Second)

	// Clear expired
	deleted, err := bb.ClearExpired(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Verify only non-expired message remains
	messages, err := bb.GetByTopic(ctx, "test", 10)
	require.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, msg2.ID, messages[0].ID)
}

// TestGetTopics tests topic listing
func TestGetTopics(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post messages to different topics
	topics := []string{"signals", "market_data", "decisions"}
	for _, topic := range topics {
		msg, err := NewMessage(topic, "agent", map[string]string{"test": "data"})
		require.NoError(t, err)
		err = bb.Post(ctx, msg)
		require.NoError(t, err)
	}

	// Get all topics
	retrievedTopics, err := bb.GetTopics(ctx)
	require.NoError(t, err)
	assert.Len(t, retrievedTopics, 3)

	for _, topic := range topics {
		assert.Contains(t, retrievedTopics, topic)
	}
}

// TestGetStats tests statistics retrieval
func TestGetStats(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	// Post messages to different topics
	for i := 0; i < 5; i++ {
		msg, _ := NewMessage("signals", "agent", map[string]int{"value": i})
		bb.Post(ctx, msg)
	}

	for i := 0; i < 3; i++ {
		msg, _ := NewMessage("market_data", "agent", map[string]int{"value": i})
		bb.Post(ctx, msg)
	}

	// Get statistics
	stats, err := bb.GetStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 2, stats["topics"])
	assert.Equal(t, 8, stats["total_messages"])

	topicStats := stats["by_topic"].(map[string]int)
	assert.Equal(t, 5, topicStats["signals"])
	assert.Equal(t, 3, topicStats["market_data"])
}

// TestMessagePriorities tests priority constants
func TestMessagePriorities(t *testing.T) {
	assert.Equal(t, MessagePriority(1), PriorityLow)
	assert.Equal(t, MessagePriority(5), PriorityNormal)
	assert.Equal(t, MessagePriority(10), PriorityHigh)
	assert.Equal(t, MessagePriority(20), PriorityUrgent)

	// Verify ordering
	assert.Less(t, PriorityLow, PriorityNormal)
	assert.Less(t, PriorityNormal, PriorityHigh)
	assert.Less(t, PriorityHigh, PriorityUrgent)
}

// TestGetMessagesByKeys_EmptyKeys tests edge case
func TestGetMessagesByKeys_EmptyKeys(t *testing.T) {
	bb, mr := setupTestBlackboard(t)
	defer mr.Close()
	defer bb.Close()

	ctx := context.Background()

	messages, err := bb.getMessagesByKeys(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, messages)
}

// TestDefaultBlackboardConfig tests default configuration
func TestDefaultBlackboardConfig(t *testing.T) {
	config := DefaultBlackboardConfig()

	assert.Equal(t, "localhost:6379", config.RedisURL)
	assert.Equal(t, "", config.RedisPassword)
	assert.Equal(t, 0, config.RedisDB)
	assert.Equal(t, "blackboard:", config.Prefix)
}

// Benchmark tests

func BenchmarkPost(b *testing.B) {
	mr := miniredis.NewMiniRedis()
	mr.Start()
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	bb := &Blackboard{
		client: client,
		prefix: "bench:",
	}
	defer bb.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg, _ := NewMessage("test", "agent", map[string]int{"value": i})
		bb.Post(ctx, msg)
	}
}

func BenchmarkGetByTopic(b *testing.B) {
	mr := miniredis.NewMiniRedis()
	mr.Start()
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	bb := &Blackboard{
		client: client,
		prefix: "bench:",
	}
	defer bb.Close()

	ctx := context.Background()

	// Populate with test data
	for i := 0; i < 100; i++ {
		msg, _ := NewMessage("test", "agent", map[string]int{"value": i})
		bb.Post(ctx, msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb.GetByTopic(ctx, "test", 10)
	}
}
