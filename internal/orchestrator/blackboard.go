package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Blackboard is a shared memory system for agent communication
// Implements the Blackboard architectural pattern for multi-agent coordination
type Blackboard struct {
	client *redis.Client
	prefix string // Key prefix for namespacing
}

// MessagePriority represents the priority of a blackboard message
type MessagePriority int

const (
	PriorityLow    MessagePriority = 1
	PriorityNormal MessagePriority = 5
	PriorityHigh   MessagePriority = 10
	PriorityUrgent MessagePriority = 20
)

// BlackboardMessage represents a message on the blackboard
type BlackboardMessage struct {
	ID        uuid.UUID              `json:"id"`
	Topic     string                 `json:"topic"`      // e.g., "signals", "market_data", "decisions"
	AgentName string                 `json:"agent_name"` // Source agent
	Content   json.RawMessage        `json:"content"`    // Message payload
	Priority  MessagePriority        `json:"priority"`
	Tags      []string               `json:"tags"` // For filtering
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"` // Optional TTL
}

// BlackboardConfig configures the blackboard
type BlackboardConfig struct {
	RedisURL      string
	RedisPassword string
	RedisDB       int
	Prefix        string // Key prefix (default: "blackboard:")
}

// DefaultBlackboardConfig returns default configuration
func DefaultBlackboardConfig() BlackboardConfig {
	return BlackboardConfig{
		RedisURL:      "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		Prefix:        "blackboard:",
	}
}

// NewBlackboard creates a new blackboard instance
func NewBlackboard(config BlackboardConfig) (*Blackboard, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	if config.Prefix == "" {
		config.Prefix = "blackboard:"
	}

	log.Info().
		Str("redis_url", config.RedisURL).
		Str("prefix", config.Prefix).
		Msg("Blackboard initialized")

	return &Blackboard{
		client: client,
		prefix: config.Prefix,
	}, nil
}

// Post posts a message to the blackboard
func (bb *Blackboard) Post(ctx context.Context, msg *BlackboardMessage) error {
	// Set defaults
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.Priority == 0 {
		msg.Priority = PriorityNormal
	}

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Store in Redis
	// Key format: blackboard:topic:{topic}:{timestamp}:{id}
	key := fmt.Sprintf("%stopic:%s:%d:%s",
		bb.prefix,
		msg.Topic,
		msg.CreatedAt.UnixNano(),
		msg.ID.String(),
	)

	// Calculate TTL
	var ttl time.Duration
	if msg.ExpiresAt != nil {
		ttl = time.Until(*msg.ExpiresAt)
		if ttl < 0 {
			return fmt.Errorf("message already expired")
		}
	} else {
		ttl = 0 // No expiration
	}

	// Store message
	if err := bb.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	// Add to topic index (sorted set by timestamp for range queries)
	indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, msg.Topic)
	score := float64(msg.CreatedAt.UnixNano())
	if err := bb.client.ZAdd(ctx, indexKey, redis.Z{
		Score:  score,
		Member: key,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to index: %w", err)
	}

	// Add to agent index (for querying by agent)
	agentIndexKey := fmt.Sprintf("%sagent:%s", bb.prefix, msg.AgentName)
	if err := bb.client.ZAdd(ctx, agentIndexKey, redis.Z{
		Score:  score,
		Member: key,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to agent index: %w", err)
	}

	// Publish notification to subscribers
	notifyKey := fmt.Sprintf("%snotify:%s", bb.prefix, msg.Topic)
	notification := map[string]interface{}{
		"message_id": msg.ID.String(),
		"topic":      msg.Topic,
		"agent":      msg.AgentName,
		"priority":   msg.Priority,
		"timestamp":  msg.CreatedAt.Unix(),
	}
	notifyData, _ := json.Marshal(notification)
	if err := bb.client.Publish(ctx, notifyKey, notifyData).Err(); err != nil {
		log.Warn().Err(err).Msg("Failed to publish notification")
		// Don't fail the post if notification fails
	}

	log.Debug().
		Str("message_id", msg.ID.String()).
		Str("topic", msg.Topic).
		Str("agent", msg.AgentName).
		Int("priority", int(msg.Priority)).
		Msg("Posted message to blackboard")

	return nil
}

// GetByTopic retrieves messages for a specific topic
func (bb *Blackboard) GetByTopic(ctx context.Context, topic string, limit int) ([]*BlackboardMessage, error) {
	indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, topic)

	// Get most recent messages (reverse order)
	keys, err := bb.client.ZRevRange(ctx, indexKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query index: %w", err)
	}

	return bb.getMessagesByKeys(ctx, keys)
}

// GetByTopicRange retrieves messages within a time range
func (bb *Blackboard) GetByTopicRange(ctx context.Context, topic string, start, end time.Time, limit int) ([]*BlackboardMessage, error) {
	indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, topic)

	// Query by score (timestamp) range
	keys, err := bb.client.ZRevRangeByScore(ctx, indexKey, &redis.ZRangeBy{
		Min:    fmt.Sprintf("%d", start.UnixNano()),
		Max:    fmt.Sprintf("%d", end.UnixNano()),
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query time range: %w", err)
	}

	return bb.getMessagesByKeys(ctx, keys)
}

// GetByAgent retrieves messages posted by a specific agent
func (bb *Blackboard) GetByAgent(ctx context.Context, agentName string, limit int) ([]*BlackboardMessage, error) {
	agentIndexKey := fmt.Sprintf("%sagent:%s", bb.prefix, agentName)

	keys, err := bb.client.ZRevRange(ctx, agentIndexKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query agent index: %w", err)
	}

	return bb.getMessagesByKeys(ctx, keys)
}

// GetByPriority retrieves messages above a certain priority
func (bb *Blackboard) GetByPriority(ctx context.Context, topic string, minPriority MessagePriority, limit int) ([]*BlackboardMessage, error) {
	messages, err := bb.GetByTopic(ctx, topic, limit*2) // Fetch more to filter
	if err != nil {
		return nil, err
	}

	// Filter by priority
	var filtered []*BlackboardMessage
	for _, msg := range messages {
		if msg.Priority >= minPriority {
			filtered = append(filtered, msg)
			if len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

// Subscribe subscribes to topic notifications
func (bb *Blackboard) Subscribe(ctx context.Context, topic string) (<-chan *BlackboardMessage, error) {
	notifyKey := fmt.Sprintf("%snotify:%s", bb.prefix, topic)

	pubsub := bb.client.Subscribe(ctx, notifyKey)

	// Test subscription
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	ch := make(chan *BlackboardMessage, 100)

	go func() {
		defer close(ch)
		defer pubsub.Close()

		msgChan := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case redisMsg := <-msgChan:
				if redisMsg == nil {
					return
				}

				// Parse notification
				var notification struct {
					MessageID string `json:"message_id"`
					Topic     string `json:"topic"`
				}
				if err := json.Unmarshal([]byte(redisMsg.Payload), &notification); err != nil {
					log.Warn().Err(err).Msg("Failed to parse notification")
					continue
				}

				// Fetch full message
				messages, err := bb.GetByTopic(ctx, notification.Topic, 100)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to fetch message")
					continue
				}

				// Find the message by ID
				for _, msg := range messages {
					if msg.ID.String() == notification.MessageID {
						select {
						case ch <- msg:
						case <-ctx.Done():
							return
						}
						break
					}
				}
			}
		}
	}()

	log.Info().
		Str("topic", topic).
		Msg("Subscribed to blackboard topic")

	return ch, nil
}

// Clear removes all messages for a topic
func (bb *Blackboard) Clear(ctx context.Context, topic string) error {
	indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, topic)

	// Get all keys
	keys, err := bb.client.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	// Delete all messages
	if len(keys) > 0 {
		if err := bb.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}
	}

	// Delete index
	if err := bb.client.Del(ctx, indexKey).Err(); err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}

	log.Info().
		Str("topic", topic).
		Int("messages_deleted", len(keys)).
		Msg("Cleared blackboard topic")

	return nil
}

// ClearExpired removes expired messages
func (bb *Blackboard) ClearExpired(ctx context.Context, topic string) (int, error) {
	indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, topic)

	// Get all keys up to now
	now := time.Now()
	keys, err := bb.client.ZRangeByScore(ctx, indexKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now.UnixNano()),
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to query expired: %w", err)
	}

	deleted := 0
	for _, key := range keys {
		// Check if key exists (may have been deleted by TTL)
		exists, err := bb.client.Exists(ctx, key).Result()
		if err != nil {
			continue
		}
		if exists == 0 {
			// Remove from index
			bb.client.ZRem(ctx, indexKey, key)
			deleted++
		}
	}

	if deleted > 0 {
		log.Info().
			Str("topic", topic).
			Int("deleted", deleted).
			Msg("Cleared expired messages")
	}

	return deleted, nil
}

// GetTopics returns all active topics
func (bb *Blackboard) GetTopics(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("%sindex:*", bb.prefix)
	keys, err := bb.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}

	// Extract topic names from keys
	topics := make([]string, 0, len(keys))
	prefixLen := len(bb.prefix) + len("index:")
	for _, key := range keys {
		if len(key) > prefixLen {
			topic := key[prefixLen:]
			topics = append(topics, topic)
		}
	}

	return topics, nil
}

// GetStats returns statistics about the blackboard
func (bb *Blackboard) GetStats(ctx context.Context) (map[string]interface{}, error) {
	topics, err := bb.GetTopics(ctx)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	stats["topics"] = len(topics)

	topicStats := make(map[string]int)
	totalMessages := 0
	for _, topic := range topics {
		indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, topic)
		count, err := bb.client.ZCard(ctx, indexKey).Result()
		if err == nil {
			topicStats[topic] = int(count)
			totalMessages += int(count)
		}
	}

	stats["total_messages"] = totalMessages
	stats["by_topic"] = topicStats

	return stats, nil
}

// Close closes the blackboard connection
func (bb *Blackboard) Close() error {
	return bb.client.Close()
}

// Helper methods

func (bb *Blackboard) getMessagesByKeys(ctx context.Context, keys []string) ([]*BlackboardMessage, error) {
	if len(keys) == 0 {
		return []*BlackboardMessage{}, nil
	}

	// Fetch all messages
	results, err := bb.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	messages := make([]*BlackboardMessage, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}

		data, ok := result.(string)
		if !ok {
			continue
		}

		var msg BlackboardMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			log.Warn().Err(err).Msg("Failed to unmarshal message")
			continue
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

// Helper functions for creating messages

// NewMessage creates a new blackboard message
func NewMessage(topic, agentName string, content interface{}) (*BlackboardMessage, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	return &BlackboardMessage{
		Topic:     topic,
		AgentName: agentName,
		Content:   contentJSON,
		Priority:  PriorityNormal,
		Tags:      []string{},
		Metadata:  make(map[string]interface{}),
	}, nil
}

// WithPriority sets the message priority
func (msg *BlackboardMessage) WithPriority(priority MessagePriority) *BlackboardMessage {
	msg.Priority = priority
	return msg
}

// WithTTL sets the message expiration
func (msg *BlackboardMessage) WithTTL(ttl time.Duration) *BlackboardMessage {
	expiresAt := time.Now().Add(ttl)
	msg.ExpiresAt = &expiresAt
	return msg
}

// WithTags adds tags to the message
func (msg *BlackboardMessage) WithTags(tags ...string) *BlackboardMessage {
	msg.Tags = append(msg.Tags, tags...)
	return msg
}

// WithMetadata adds metadata to the message
func (msg *BlackboardMessage) WithMetadata(key string, value interface{}) *BlackboardMessage {
	msg.Metadata[key] = value
	return msg
}
