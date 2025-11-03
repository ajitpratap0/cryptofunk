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

	// Use pipeline for atomic operations
	pipe := bb.client.Pipeline()

	// Store message
	pipe.Set(ctx, key, data, ttl)

	// Add to topic index (sorted set by timestamp for range queries)
	indexKey := fmt.Sprintf("%sindex:%s", bb.prefix, msg.Topic)
	score := float64(msg.CreatedAt.UnixNano())
	pipe.ZAdd(ctx, indexKey, redis.Z{
		Score:  score,
		Member: key,
	})

	// Add to agent index (for querying by agent)
	agentIndexKey := fmt.Sprintf("%sagent:%s", bb.prefix, msg.AgentName)
	pipe.ZAdd(ctx, agentIndexKey, redis.Z{
		Score:  score,
		Member: key,
	})

	// Add to priority index (for efficient priority-based queries)
	priorityIndexKey := fmt.Sprintf("%spriority:%s:%d", bb.prefix, msg.Topic, msg.Priority)
	pipe.ZAdd(ctx, priorityIndexKey, redis.Z{
		Score:  score,
		Member: key,
	})

	// Execute pipeline atomically
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to store message and indices: %w", err)
	}

	// Publish notification to subscribers
	notifyKey := fmt.Sprintf("%snotify:%s", bb.prefix, msg.Topic)
	notification := map[string]interface{}{
		"message_id":  msg.ID.String(),
		"message_key": key, // Include key for direct message retrieval
		"topic":       msg.Topic,
		"agent":       msg.AgentName,
		"priority":    msg.Priority,
		"timestamp":   msg.CreatedAt.Unix(),
	}
	notifyData, err := json.Marshal(notification)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal notification")
		// Continue without notification - not fatal
	} else if err := bb.client.Publish(ctx, notifyKey, notifyData).Err(); err != nil {
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
	// Query priority indices for all priorities >= minPriority
	var allKeys []string

	// Iterate through priority levels from highest to lowest
	for priority := PriorityUrgent; priority >= minPriority; priority-- {
		priorityIndexKey := fmt.Sprintf("%spriority:%s:%d", bb.prefix, topic, priority)

		// Get keys from this priority level (most recent first)
		keys, err := bb.client.ZRevRange(ctx, priorityIndexKey, 0, int64(limit-1)).Result()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("failed to query priority index: %w", err)
		}

		allKeys = append(allKeys, keys...)

		// Stop if we have enough keys
		if len(allKeys) >= limit {
			allKeys = allKeys[:limit]
			break
		}
	}

	if len(allKeys) == 0 {
		return []*BlackboardMessage{}, nil
	}

	return bb.getMessagesByKeys(ctx, allKeys)
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
					MessageID  string `json:"message_id"`
					MessageKey string `json:"message_key"`
					Topic      string `json:"topic"`
				}
				if err := json.Unmarshal([]byte(redisMsg.Payload), &notification); err != nil {
					log.Warn().Err(err).Msg("Failed to parse notification")
					continue
				}

				// Fetch message directly using key (more efficient than fetching 100 messages)
				data, err := bb.client.Get(ctx, notification.MessageKey).Result()
				if err != nil {
					log.Warn().
						Err(err).
						Str("message_key", notification.MessageKey).
						Msg("Failed to fetch message")
					continue
				}

				// Unmarshal message
				var msg BlackboardMessage
				if err := json.Unmarshal([]byte(data), &msg); err != nil {
					log.Warn().Err(err).Msg("Failed to parse message")
					continue
				}

				// Send to subscriber
				select {
				case ch <- &msg:
				case <-ctx.Done():
					return
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

	if len(keys) == 0 {
		return nil
	}

	// Fetch messages to get agent names for index cleanup
	messages, err := bb.getMessagesByKeys(ctx, keys)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch messages for agent index cleanup")
		// Continue with deletion even if we can't fetch messages
	}

	// Use pipeline for atomic deletion
	pipe := bb.client.Pipeline()

	// Delete all messages
	pipe.Del(ctx, keys...)

	// Delete topic index
	pipe.Del(ctx, indexKey)

	// Remove from agent indices and collect priorities
	if messages != nil {
		agentKeys := make(map[string]bool)
		priorities := make(map[MessagePriority]bool)
		for _, msg := range messages {
			if msg != nil {
				if msg.AgentName != "" {
					agentKeys[msg.AgentName] = true
				}
				priorities[msg.Priority] = true
			}
		}

		// For each agent, remove all keys from their index
		for agentName := range agentKeys {
			agentIndexKey := fmt.Sprintf("%sagent:%s", bb.prefix, agentName)
			for _, key := range keys {
				pipe.ZRem(ctx, agentIndexKey, key)
			}
		}

		// Remove from priority indices
		for priority := range priorities {
			priorityIndexKey := fmt.Sprintf("%spriority:%s:%d", bb.prefix, topic, priority)
			for _, key := range keys {
				pipe.ZRem(ctx, priorityIndexKey, key)
			}
		}
	} else {
		// If we couldn't fetch messages, clean up all possible priority indices
		for priority := PriorityLow; priority <= PriorityUrgent; priority++ {
			priorityIndexKey := fmt.Sprintf("%spriority:%s:%d", bb.prefix, topic, priority)
			for _, key := range keys {
				pipe.ZRem(ctx, priorityIndexKey, key)
			}
		}
	}

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear topic: %w", err)
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
	topics := make([]string, 0)
	prefixLen := len(bb.prefix) + len("index:")

	// Use SCAN instead of KEYS for non-blocking operation
	iter := bb.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if len(key) > prefixLen {
			topic := key[prefixLen:]
			topics = append(topics, topic)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan topics: %w", err)
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
