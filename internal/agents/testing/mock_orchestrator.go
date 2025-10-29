// Package testing provides utilities for testing trading agents
package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

// MockOrchestrator simulates an orchestrator for testing agent signal publishing
type MockOrchestrator struct {
	mu sync.RWMutex

	// NATS connection
	natsConn      *nats.Conn
	subscriptions []*nats.Subscription

	// Signal recording
	signals []ReceivedSignal

	// Decision simulation
	decisions      []Decision
	decisionPolicy DecisionPolicy

	// Configuration
	topics []string
}

// ReceivedSignal represents a signal received from an agent
type ReceivedSignal struct {
	Topic     string
	Signal    interface{} // TechnicalSignal, OrderBookSignal, or SentimentSignal
	RawData   []byte
	Timestamp time.Time
}

// Decision represents a simulated orchestrator decision
type Decision struct {
	Action     string // BUY, SELL, HOLD
	Symbol     string
	Confidence float64
	Reasoning  string
	Timestamp  time.Time
	BasedOn    []string // Topics that contributed to decision
}

// DecisionPolicy determines how the mock orchestrator makes decisions
type DecisionPolicy func(signals []ReceivedSignal) *Decision

// MockOrchestratorConfig configures the mock orchestrator
type MockOrchestratorConfig struct {
	NATSConn       *nats.Conn
	Topics         []string
	DecisionPolicy DecisionPolicy
}

// NewMockOrchestrator creates a new mock orchestrator
func NewMockOrchestrator(config MockOrchestratorConfig) (*MockOrchestrator, error) {
	if config.NATSConn == nil {
		return nil, fmt.Errorf("NATS connection is required")
	}

	if len(config.Topics) == 0 {
		// Default to standard agent topics
		config.Topics = []string{
			"agents.analysis.technical",
			"agents.analysis.orderbook",
			"agents.analysis.sentiment",
		}
	}

	if config.DecisionPolicy == nil {
		// Default to simple majority voting policy
		config.DecisionPolicy = DefaultDecisionPolicy
	}

	return &MockOrchestrator{
		natsConn:       config.NATSConn,
		topics:         config.Topics,
		signals:        make([]ReceivedSignal, 0),
		decisions:      make([]Decision, 0),
		subscriptions:  make([]*nats.Subscription, 0),
		decisionPolicy: config.DecisionPolicy,
	}, nil
}

// Start begins listening for agent signals
func (m *MockOrchestrator) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Subscribe to all configured topics
	for _, topic := range m.topics {
		sub, err := m.natsConn.Subscribe(topic, m.handleSignal)
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", topic, err)
		}
		m.subscriptions = append(m.subscriptions, sub)
		log.Info().Str("topic", topic).Msg("Mock orchestrator subscribed to topic")
	}

	return nil
}

// Stop unsubscribes from all topics and cleans up
func (m *MockOrchestrator) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sub := range m.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			log.Warn().Err(err).Msg("Failed to unsubscribe")
		}
	}

	m.subscriptions = nil
	log.Info().Msg("Mock orchestrator stopped")
	return nil
}

// handleSignal processes incoming signals from agents
func (m *MockOrchestrator) handleSignal(msg *nats.Msg) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the signal
	signal := ReceivedSignal{
		Topic:     msg.Subject,
		RawData:   msg.Data,
		Timestamp: time.Now(),
	}

	// Try to unmarshal as a generic signal to extract common fields
	var genericSignal struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal(msg.Data, &genericSignal); err != nil {
		log.Warn().Err(err).Str("topic", msg.Subject).Msg("Failed to unmarshal signal")
		return
	}

	signal.Signal = genericSignal
	m.signals = append(m.signals, signal)

	log.Debug().
		Str("topic", msg.Subject).
		Str("symbol", genericSignal.Symbol).
		Str("signal", genericSignal.Signal).
		Float64("confidence", genericSignal.Confidence).
		Msg("Mock orchestrator received signal")

	// Trigger decision-making based on accumulated signals
	m.makeDecision()
}

// makeDecision simulates orchestrator decision-making
func (m *MockOrchestrator) makeDecision() {
	// Only make decisions if we have signals
	if len(m.signals) == 0 {
		return
	}

	// Use the configured decision policy
	decision := m.decisionPolicy(m.signals)
	if decision != nil {
		m.decisions = append(m.decisions, *decision)
		log.Info().
			Str("action", decision.Action).
			Str("symbol", decision.Symbol).
			Float64("confidence", decision.Confidence).
			Msg("Mock orchestrator made decision")
	}
}

// GetReceivedSignals returns all recorded signals
func (m *MockOrchestrator) GetReceivedSignals() []ReceivedSignal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return copy to prevent modification
	signals := make([]ReceivedSignal, len(m.signals))
	copy(signals, m.signals)
	return signals
}

// GetDecisions returns all simulated decisions
func (m *MockOrchestrator) GetDecisions() []Decision {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return copy to prevent modification
	decisions := make([]Decision, len(m.decisions))
	copy(decisions, m.decisions)
	return decisions
}

// GetSignalCount returns the number of signals received from a specific topic
func (m *MockOrchestrator) GetSignalCount(topic string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, signal := range m.signals {
		if signal.Topic == topic {
			count++
		}
	}
	return count
}

// GetLastSignal returns the last signal received from a specific topic
func (m *MockOrchestrator) GetLastSignal(topic string) *ReceivedSignal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := len(m.signals) - 1; i >= 0; i-- {
		if m.signals[i].Topic == topic {
			signal := m.signals[i]
			return &signal
		}
	}
	return nil
}

// GetLastDecision returns the most recent decision
func (m *MockOrchestrator) GetLastDecision() *Decision {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.decisions) == 0 {
		return nil
	}

	decision := m.decisions[len(m.decisions)-1]
	return &decision
}

// Reset clears all recorded signals and decisions
func (m *MockOrchestrator) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.signals = make([]ReceivedSignal, 0)
	m.decisions = make([]Decision, 0)
	log.Info().Msg("Mock orchestrator reset")
}

// WaitForSignals waits for at least n signals to be received or timeout
func (m *MockOrchestrator) WaitForSignals(n int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		m.mu.RLock()
		count := len(m.signals)
		m.mu.RUnlock()

		if count >= n {
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for %d signals (received %d)", n, len(m.signals))
}

// WaitForDecision waits for at least one decision to be made or timeout
func (m *MockOrchestrator) WaitForDecision(timeout time.Duration) (*Decision, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		m.mu.RLock()
		count := len(m.decisions)
		m.mu.RUnlock()

		if count > 0 {
			return m.GetLastDecision(), nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for decision")
}

// DefaultDecisionPolicy is a simple majority voting policy
func DefaultDecisionPolicy(signals []ReceivedSignal) *Decision {
	if len(signals) == 0 {
		return nil
	}

	// Extract generic signals
	type GenericSignal struct {
		Symbol     string
		Signal     string
		Confidence float64
	}

	var genericSignals []GenericSignal
	symbolMap := make(map[string]bool)

	for _, signal := range signals {
		if gs, ok := signal.Signal.(struct {
			Symbol     string  `json:"symbol"`
			Signal     string  `json:"signal"`
			Confidence float64 `json:"confidence"`
		}); ok {
			genericSignals = append(genericSignals, GenericSignal{
				Symbol:     gs.Symbol,
				Signal:     gs.Signal,
				Confidence: gs.Confidence,
			})
			symbolMap[gs.Symbol] = true
		}
	}

	if len(genericSignals) == 0 {
		return nil
	}

	// For simplicity, use the most recent signal's symbol
	symbol := genericSignals[len(genericSignals)-1].Symbol

	// Count weighted votes
	buyWeight := 0.0
	sellWeight := 0.0
	holdWeight := 0.0
	topics := make([]string, 0)

	for i, signal := range signals {
		gs := genericSignals[i]
		topics = append(topics, signal.Topic)

		switch gs.Signal {
		case "BUY", "BULLISH", "LONG":
			buyWeight += gs.Confidence
		case "SELL", "BEARISH", "SHORT":
			sellWeight += gs.Confidence
		default:
			holdWeight += gs.Confidence
		}
	}

	// Determine action based on highest weighted vote
	action := "HOLD"
	confidence := holdWeight
	reasoning := "Neutral signals"

	if buyWeight > sellWeight && buyWeight > holdWeight {
		action = "BUY"
		confidence = buyWeight / float64(len(genericSignals))
		reasoning = "Majority bullish signals"
	} else if sellWeight > buyWeight && sellWeight > holdWeight {
		action = "SELL"
		confidence = sellWeight / float64(len(genericSignals))
		reasoning = "Majority bearish signals"
	}

	return &Decision{
		Action:     action,
		Symbol:     symbol,
		Confidence: confidence,
		Reasoning:  reasoning,
		Timestamp:  time.Now(),
		BasedOn:    topics,
	}
}
