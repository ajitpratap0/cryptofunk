// Package agents provides base infrastructure for AI trading agents
package agents

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// HeartbeatConfig holds configuration for heartbeat publishing
type HeartbeatConfig struct {
	// Interval between heartbeat messages (default: 30 seconds)
	Interval time.Duration
	// Topic is the NATS topic to publish heartbeats to (e.g., "agents.heartbeat")
	Topic string
}

// DefaultHeartbeatConfig returns the default heartbeat configuration
func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Interval: 30 * time.Second,
		Topic:    "agents.heartbeat",
	}
}

// HeartbeatMessage represents a heartbeat message published by agents
type HeartbeatMessage struct {
	AgentName string    `json:"agent_name"`
	AgentType string    `json:"agent_type"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// HeartbeatPublisher handles periodic heartbeat publishing for agents
type HeartbeatPublisher struct {
	natsConn  *nats.Conn
	config    HeartbeatConfig
	agentName string
	agentType string
	log       zerolog.Logger
	stopChan  chan struct{}
	running   atomic.Bool
}

// NewHeartbeatPublisher creates a new heartbeat publisher
// The natsConn can be nil initially and set later with SetNATSConn
func NewHeartbeatPublisher(agentName, agentType string, config HeartbeatConfig, log zerolog.Logger) *HeartbeatPublisher {
	return &HeartbeatPublisher{
		config:    config,
		agentName: agentName,
		agentType: agentType,
		log:       log.With().Str("component", "heartbeat").Logger(),
		stopChan:  make(chan struct{}),
	}
}

// SetNATSConn sets the NATS connection for the heartbeat publisher
// This allows setting the connection after initialization
func (h *HeartbeatPublisher) SetNATSConn(conn *nats.Conn) {
	h.natsConn = conn
}

// Start begins publishing heartbeat messages at the configured interval
// The goroutine will publish immediately on start, then at the configured interval
func (h *HeartbeatPublisher) Start() {
	if h.running.Load() {
		h.log.Warn().Msg("Heartbeat publisher already running")
		return
	}
	if h.natsConn == nil {
		h.log.Warn().Msg("Cannot start heartbeat publisher: NATS connection not set")
		return
	}

	h.running.Store(true)
	ticker := time.NewTicker(h.config.Interval)

	go func() {
		// Publish immediately on start
		h.publish()

		for {
			select {
			case <-ticker.C:
				h.publish()
			case <-h.stopChan:
				ticker.Stop()
				h.running.Store(false)
				h.log.Info().Str("topic", h.config.Topic).Msg("Heartbeat publishing stopped")
				return
			}
		}
	}()

	h.log.Info().
		Str("topic", h.config.Topic).
		Dur("interval", h.config.Interval).
		Msg("Heartbeat publishing started")
}

// publish sends a single heartbeat message
func (h *HeartbeatPublisher) publish() {
	if h.natsConn == nil {
		h.log.Warn().Msg("Cannot publish heartbeat: NATS connection not set")
		return
	}

	heartbeat := HeartbeatMessage{
		AgentName: h.agentName,
		AgentType: h.agentType,
		Timestamp: time.Now(),
		Status:    "healthy",
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to marshal heartbeat")
		return
	}

	if err := h.natsConn.Publish(h.config.Topic, data); err != nil {
		h.log.Error().Err(err).Msg("Failed to publish heartbeat")
		return
	}

	h.log.Debug().Str("topic", h.config.Topic).Msg("Heartbeat published")
}

// Stop stops the heartbeat publisher
func (h *HeartbeatPublisher) Stop() {
	if !h.running.Load() {
		return
	}
	close(h.stopChan)
}

// IsRunning returns whether the heartbeat publisher is currently running
func (h *HeartbeatPublisher) IsRunning() bool {
	return h.running.Load()
}

// PublishNow immediately publishes a heartbeat message (useful for status updates)
func (h *HeartbeatPublisher) PublishNow() {
	h.publish()
}

// PublishWithStatus publishes a heartbeat with a custom status
func (h *HeartbeatPublisher) PublishWithStatus(status string) {
	if h.natsConn == nil {
		h.log.Warn().Msg("Cannot publish heartbeat: NATS connection not set")
		return
	}

	heartbeat := HeartbeatMessage{
		AgentName: h.agentName,
		AgentType: h.agentType,
		Timestamp: time.Now(),
		Status:    status,
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to marshal heartbeat")
		return
	}

	if err := h.natsConn.Publish(h.config.Topic, data); err != nil {
		h.log.Error().Err(err).Msg("Failed to publish heartbeat")
		return
	}

	h.log.Debug().
		Str("topic", h.config.Topic).
		Str("status", status).
		Msg("Heartbeat with status published")
}
