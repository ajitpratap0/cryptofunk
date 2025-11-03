package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

// MessageBus provides agent-to-agent communication via NATS
type MessageBus struct {
	nc     *nats.Conn
	prefix string // Subject prefix for namespacing
}

// MessageBusConfig configures the message bus
type MessageBusConfig struct {
	NATSURL string
	Prefix  string // Subject prefix (default: "agents.")
}

// AgentMessage represents a message between agents
type AgentMessage struct {
	ID        uuid.UUID              `json:"id"`
	From      string                 `json:"from"`      // Source agent
	To        string                 `json:"to"`        // Target agent (or "*" for broadcast)
	Type      MessageType            `json:"type"`      // Message type
	Topic     string                 `json:"topic"`     // Message topic/subject
	Payload   json.RawMessage        `json:"payload"`   // Message content
	Metadata  map[string]interface{} `json:"metadata"`  // Additional metadata
	Timestamp time.Time              `json:"timestamp"` // Creation time
	ReplyTo   string                 `json:"reply_to"`  // Reply subject (for request-reply)
	TTL       time.Duration          `json:"ttl"`       // Time-to-live
	Priority  int                    `json:"priority"`  // Message priority (0-9, higher = more important)
}

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeRequest      MessageType = "request"      // Request expecting a reply
	MessageTypeReply        MessageType = "reply"        // Reply to a request
	MessageTypeNotification MessageType = "notification" // One-way notification
	MessageTypeBroadcast    MessageType = "broadcast"    // Broadcast to all agents
	MessageTypeCommand      MessageType = "command"      // Command to execute
	MessageTypeEvent        MessageType = "event"        // Event notification
)

// MessageHandler is a callback for handling received messages
type MessageHandler func(msg *AgentMessage) error

// DefaultMessageBusConfig returns default configuration
func DefaultMessageBusConfig() MessageBusConfig {
	return MessageBusConfig{
		NATSURL: "nats://localhost:4222",
		Prefix:  "agents.",
	}
}

// NewMessageBus creates a new message bus instance
func NewMessageBus(config MessageBusConfig) (*MessageBus, error) {
	// Connect to NATS
	nc, err := nats.Connect(
		config.NATSURL,
		nats.Name("cryptofunk-orchestrator"),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Warn().Err(err).Msg("NATS disconnected")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info().Str("url", nc.ConnectedUrl()).Msg("NATS reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	if config.Prefix == "" {
		config.Prefix = "agents."
	}

	log.Info().
		Str("nats_url", config.NATSURL).
		Str("prefix", config.Prefix).
		Msg("MessageBus initialized")

	return &MessageBus{
		nc:     nc,
		prefix: config.Prefix,
	}, nil
}

// Send sends a message to a specific agent
func (mb *MessageBus) Send(ctx context.Context, msg *AgentMessage) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check connection health
	if !mb.nc.IsConnected() {
		return fmt.Errorf("message bus not connected")
	}

	// Set defaults
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Type == "" {
		msg.Type = MessageTypeNotification
	}

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Construct subject
	// Pattern: agents.{to}.{topic}
	subject := fmt.Sprintf("%s%s.%s", mb.prefix, msg.To, msg.Topic)

	// Publish message
	if err := mb.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Debug().
		Str("message_id", msg.ID.String()).
		Str("from", msg.From).
		Str("to", msg.To).
		Str("type", string(msg.Type)).
		Str("topic", msg.Topic).
		Str("subject", subject).
		Msg("Sent message")

	return nil
}

// Broadcast sends a message to all agents
func (mb *MessageBus) Broadcast(ctx context.Context, msg *AgentMessage) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check connection health
	if !mb.nc.IsConnected() {
		return fmt.Errorf("message bus not connected")
	}

	msg.To = "*" // Broadcast marker
	msg.Type = MessageTypeBroadcast

	// Set defaults
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Broadcast subject
	// Pattern: agents.*.{topic}
	subject := fmt.Sprintf("%s*.%s", mb.prefix, msg.Topic)

	// Publish message
	if err := mb.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	log.Debug().
		Str("message_id", msg.ID.String()).
		Str("from", msg.From).
		Str("topic", msg.Topic).
		Str("subject", subject).
		Msg("Broadcast message")

	return nil
}

// Request sends a request and waits for a reply
func (mb *MessageBus) Request(ctx context.Context, msg *AgentMessage, timeout time.Duration) (*AgentMessage, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check connection health
	if !mb.nc.IsConnected() {
		return nil, fmt.Errorf("message bus not connected")
	}

	msg.Type = MessageTypeRequest

	// Set defaults
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Construct subject
	subject := fmt.Sprintf("%s%s.%s", mb.prefix, msg.To, msg.Topic)

	// Send request and wait for reply
	natsMsg, err := mb.nc.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse reply
	var reply AgentMessage
	if err := json.Unmarshal(natsMsg.Data, &reply); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reply: %w", err)
	}

	log.Debug().
		Str("request_id", msg.ID.String()).
		Str("reply_id", reply.ID.String()).
		Str("from", msg.From).
		Str("to", msg.To).
		Str("topic", msg.Topic).
		Dur("duration", time.Since(msg.Timestamp)).
		Msg("Request completed")

	return &reply, nil
}

// createSubscriptionHandler creates a common message handler for subscriptions
func (mb *MessageBus) createSubscriptionHandler(agentName string, handler MessageHandler) func(*nats.Msg) {
	return func(natsMsg *nats.Msg) {
		// Parse message
		var msg AgentMessage
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			log.Warn().Err(err).Msg("Failed to unmarshal message")
			return
		}

		// Set reply address from NATS message (for request-reply pattern)
		if natsMsg.Reply != "" {
			msg.ReplyTo = natsMsg.Reply
		}

		// Check TTL
		if msg.TTL > 0 && time.Since(msg.Timestamp) > msg.TTL {
			log.Debug().
				Str("message_id", msg.ID.String()).
				Dur("age", time.Since(msg.Timestamp)).
				Dur("ttl", msg.TTL).
				Msg("Message expired, skipping")
			return
		}

		// Handle message
		if err := handler(&msg); err != nil {
			mb.handleSubscriptionError(&msg, agentName, natsMsg, err)
			return
		}

		log.Debug().
			Str("message_id", msg.ID.String()).
			Str("from", msg.From).
			Str("to", msg.To).
			Str("topic", msg.Topic).
			Msg("Message handled successfully")
	}
}

// handleSubscriptionError handles errors from message handlers
func (mb *MessageBus) handleSubscriptionError(msg *AgentMessage, agentName string, natsMsg *nats.Msg, handlerErr error) {
	log.Error().
		Err(handlerErr).
		Str("message_id", msg.ID.String()).
		Str("from", msg.From).
		Str("to", msg.To).
		Str("topic", msg.Topic).
		Msg("Message handler error")

	// If this is a request, send error reply
	if msg.Type == MessageTypeRequest && natsMsg.Reply != "" {
		errorReply := &AgentMessage{
			ID:        uuid.New(),
			From:      agentName,
			To:        msg.From,
			Type:      MessageTypeReply,
			Topic:     msg.Topic,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"error":      handlerErr.Error(),
				"request_id": msg.ID.String(),
			},
		}
		replyData, err := json.Marshal(errorReply)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal error reply")
			return
		}
		if err := natsMsg.Respond(replyData); err != nil {
			log.Error().Err(err).Msg("Failed to send error reply")
		}
	}
}

// Subscribe subscribes to messages for a specific agent and topic
func (mb *MessageBus) Subscribe(agentName, topic string, handler MessageHandler) (*Subscription, error) {
	// Subject pattern: agents.{agentName}.{topic}
	subject := fmt.Sprintf("%s%s.%s", mb.prefix, agentName, topic)

	// Subscribe with common handler
	sub, err := mb.nc.Subscribe(subject, mb.createSubscriptionHandler(agentName, handler))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Info().
		Str("agent", agentName).
		Str("topic", topic).
		Str("subject", subject).
		Msg("Subscribed to messages")

	return &Subscription{
		sub:       sub,
		agentName: agentName,
		topic:     topic,
		subject:   subject,
	}, nil
}

// SubscribeAll subscribes to all messages for a specific agent
func (mb *MessageBus) SubscribeAll(agentName string, handler MessageHandler) (*Subscription, error) {
	// Subject pattern: agents.{agentName}.>
	subject := fmt.Sprintf("%s%s.>", mb.prefix, agentName)

	// Subscribe with common handler
	sub, err := mb.nc.Subscribe(subject, mb.createSubscriptionHandler(agentName, handler))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Info().
		Str("agent", agentName).
		Str("subject", subject).
		Msg("Subscribed to all messages")

	return &Subscription{
		sub:       sub,
		agentName: agentName,
		topic:     "*",
		subject:   subject,
	}, nil
}

// SubscribeBroadcasts subscribes to broadcast messages for a specific topic
func (mb *MessageBus) SubscribeBroadcasts(topic string, handler MessageHandler) (*Subscription, error) {
	// Subject pattern: agents.*.{topic}
	subject := fmt.Sprintf("%s*.%s", mb.prefix, topic)

	// Subscribe with common handler (empty agentName for broadcasts)
	sub, err := mb.nc.Subscribe(subject, mb.createSubscriptionHandler("", handler))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to broadcasts: %w", err)
	}

	log.Info().
		Str("topic", topic).
		Str("subject", subject).
		Msg("Subscribed to broadcast messages")

	return &Subscription{
		sub:     sub,
		topic:   topic,
		subject: subject,
	}, nil
}

// Reply sends a reply to a request message
func (mb *MessageBus) Reply(ctx context.Context, originalMsg *AgentMessage, replyPayload interface{}) error {
	if originalMsg.ReplyTo == "" {
		return fmt.Errorf("original message has no reply address")
	}

	payloadJSON, err := json.Marshal(replyPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal reply payload: %w", err)
	}

	reply := &AgentMessage{
		ID:        uuid.New(),
		From:      originalMsg.To, // Reply from the original recipient
		To:        originalMsg.From,
		Type:      MessageTypeReply,
		Topic:     originalMsg.Topic,
		Payload:   payloadJSON,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"request_id": originalMsg.ID.String(),
		},
	}

	data, err := json.Marshal(reply)
	if err != nil {
		return fmt.Errorf("failed to marshal reply: %w", err)
	}

	if err := mb.nc.Publish(originalMsg.ReplyTo, data); err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	log.Debug().
		Str("reply_id", reply.ID.String()).
		Str("request_id", originalMsg.ID.String()).
		Str("from", reply.From).
		Str("to", reply.To).
		Msg("Sent reply")

	return nil
}

// GetStats returns message bus statistics
func (mb *MessageBus) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if mb.nc != nil {
		stats["connected"] = mb.nc.IsConnected()
		stats["status"] = mb.nc.Status().String()
		stats["servers"] = mb.nc.Servers()
		stats["connected_url"] = mb.nc.ConnectedUrl()
		stats["in_msgs"] = mb.nc.Stats().InMsgs
		stats["out_msgs"] = mb.nc.Stats().OutMsgs
		stats["in_bytes"] = mb.nc.Stats().InBytes
		stats["out_bytes"] = mb.nc.Stats().OutBytes
		stats["reconnects"] = mb.nc.Stats().Reconnects
	}

	return stats
}

// Close closes the message bus connection
func (mb *MessageBus) Close() error {
	if mb.nc != nil {
		mb.nc.Close()
		log.Info().Msg("MessageBus closed")
	}
	return nil
}

// Subscription represents an active subscription
type Subscription struct {
	sub       *nats.Subscription
	agentName string
	topic     string
	subject   string
}

// Unsubscribe unsubscribes from the subscription
func (s *Subscription) Unsubscribe() error {
	if err := s.sub.Unsubscribe(); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	log.Info().
		Str("agent", s.agentName).
		Str("topic", s.topic).
		Str("subject", s.subject).
		Msg("Unsubscribed from messages")

	return nil
}

// IsValid returns whether the subscription is still active
func (s *Subscription) IsValid() bool {
	return s.sub.IsValid()
}

// Helper functions for creating messages

// NewAgentMessage creates a new agent message
func NewAgentMessage(from, to, topic string, payload interface{}) (*AgentMessage, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return &AgentMessage{
		From:     from,
		To:       to,
		Topic:    topic,
		Payload:  payloadJSON,
		Metadata: make(map[string]interface{}),
		Priority: 5, // Normal priority
	}, nil
}

// WithType sets the message type
func (m *AgentMessage) WithType(msgType MessageType) *AgentMessage {
	m.Type = msgType
	return m
}

// WithPriority sets the message priority
func (m *AgentMessage) WithPriority(priority int) *AgentMessage {
	m.Priority = priority
	return m
}

// WithTTL sets the message TTL
func (m *AgentMessage) WithTTL(ttl time.Duration) *AgentMessage {
	m.TTL = ttl
	return m
}

// WithMetadata adds metadata to the message
func (m *AgentMessage) WithMetadata(key string, value interface{}) *AgentMessage {
	m.Metadata[key] = value
	return m
}
