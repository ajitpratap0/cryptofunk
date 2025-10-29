package testing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test signal structures matching agent formats
type TestTechnicalSignal struct {
	Symbol     string  `json:"symbol"`
	Signal     string  `json:"signal"`
	Confidence float64 `json:"confidence"`
	RSI        float64 `json:"rsi"`
	MACD       string  `json:"macd"`
	Reasoning  string  `json:"reasoning"`
}

type TestOrderBookSignal struct {
	Symbol      string  `json:"symbol"`
	Signal      string  `json:"signal"`
	Confidence  float64 `json:"confidence"`
	Imbalance   float64 `json:"imbalance"`
	BidPressure float64 `json:"bid_pressure"`
	AskPressure float64 `json:"ask_pressure"`
	Reasoning   string  `json:"reasoning"`
}

type TestSentimentSignal struct {
	Symbol         string  `json:"symbol"`
	Signal         string  `json:"signal"`
	Confidence     float64 `json:"confidence"`
	SentimentScore float64 `json:"sentiment_score"`
	Reasoning      string  `json:"reasoning"`
}

func setupNATS(t *testing.T) *nats.Conn {
	t.Helper()

	// Connect to NATS (assumes NATS is running on localhost:4222)
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS not available: %v", err)
	}

	return nc
}

func TestNewMockOrchestrator(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	tests := []struct {
		name        string
		config      MockOrchestratorConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with default topics",
			config: MockOrchestratorConfig{
				NATSConn: nc,
			},
			wantErr: false,
		},
		{
			name: "valid config with custom topics",
			config: MockOrchestratorConfig{
				NATSConn: nc,
				Topics:   []string{"test.topic.1", "test.topic.2"},
			},
			wantErr: false,
		},
		{
			name: "valid config with custom decision policy",
			config: MockOrchestratorConfig{
				NATSConn: nc,
				DecisionPolicy: func(signals []ReceivedSignal) *Decision {
					return &Decision{Action: "CUSTOM"}
				},
			},
			wantErr: false,
		},
		{
			name: "missing NATS connection",
			config: MockOrchestratorConfig{
				Topics: []string{"test.topic"},
			},
			wantErr:     true,
			errContains: "NATS connection is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mo, err := NewMockOrchestrator(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, mo)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mo)
				assert.NotNil(t, mo.decisionPolicy)
				assert.NotEmpty(t, mo.topics)
			}
		})
	}
}

func TestMockOrchestrator_StartStop(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"test.start.stop"},
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Start should succeed
	err = mo.Start(ctx)
	assert.NoError(t, err)
	assert.Len(t, mo.subscriptions, 1)

	// Stop should succeed
	err = mo.Stop()
	assert.NoError(t, err)
	assert.Nil(t, mo.subscriptions)
}

func TestMockOrchestrator_ReceiveSignal(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topic := "test.receive.signal"
	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{topic},
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish a test signal
	signal := TestTechnicalSignal{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
		RSI:        35.5,
		MACD:       "BULLISH",
		Reasoning:  "RSI oversold, MACD bullish crossover",
	}

	data, err := json.Marshal(signal)
	require.NoError(t, err)

	err = nc.Publish(topic, data)
	require.NoError(t, err)

	// Wait for signal to be received
	err = mo.WaitForSignals(1, 2*time.Second)
	require.NoError(t, err)

	// Verify signal was recorded
	signals := mo.GetReceivedSignals()
	assert.Len(t, signals, 1)
	assert.Equal(t, topic, signals[0].Topic)
	assert.Equal(t, data, signals[0].RawData)

	// Verify signal count
	count := mo.GetSignalCount(topic)
	assert.Equal(t, 1, count)

	// Verify last signal
	lastSignal := mo.GetLastSignal(topic)
	require.NotNil(t, lastSignal)
	assert.Equal(t, topic, lastSignal.Topic)
}

func TestMockOrchestrator_MultipleSignals(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topics := []string{
		"agents.analysis.technical",
		"agents.analysis.orderbook",
		"agents.analysis.sentiment",
	}

	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   topics,
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish technical signal
	techSignal := TestTechnicalSignal{
		Symbol:     "ETH/USDT",
		Signal:     "BUY",
		Confidence: 0.80,
		RSI:        30.0,
		MACD:       "BULLISH",
		Reasoning:  "Strong technical indicators",
	}
	techData, _ := json.Marshal(techSignal)
	err = nc.Publish(topics[0], techData)
	require.NoError(t, err)

	// Publish orderbook signal
	obSignal := TestOrderBookSignal{
		Symbol:      "ETH/USDT",
		Signal:      "BUY",
		Confidence:  0.75,
		Imbalance:   0.65,
		BidPressure: 0.70,
		AskPressure: 0.30,
		Reasoning:   "Strong buy-side pressure",
	}
	obData, _ := json.Marshal(obSignal)
	err = nc.Publish(topics[1], obData)
	require.NoError(t, err)

	// Publish sentiment signal
	sentSignal := TestSentimentSignal{
		Symbol:         "ETH/USDT",
		Signal:         "BULLISH",
		Confidence:     0.70,
		SentimentScore: 0.75,
		Reasoning:      "Positive market sentiment",
	}
	sentData, _ := json.Marshal(sentSignal)
	err = nc.Publish(topics[2], sentData)
	require.NoError(t, err)

	// Wait for all signals
	err = mo.WaitForSignals(3, 2*time.Second)
	require.NoError(t, err)

	// Verify all signals received
	signals := mo.GetReceivedSignals()
	assert.Len(t, signals, 3)

	// Verify signal counts per topic
	assert.Equal(t, 1, mo.GetSignalCount(topics[0]))
	assert.Equal(t, 1, mo.GetSignalCount(topics[1]))
	assert.Equal(t, 1, mo.GetSignalCount(topics[2]))
}

func TestMockOrchestrator_DecisionMaking(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topic := "test.decision.making"
	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{topic},
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish a BUY signal
	signal := TestTechnicalSignal{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.90,
		RSI:        25.0,
		MACD:       "BULLISH",
		Reasoning:  "Strong buy signal",
	}

	data, err := json.Marshal(signal)
	require.NoError(t, err)

	err = nc.Publish(topic, data)
	require.NoError(t, err)

	// Wait for decision
	decision, err := mo.WaitForDecision(2 * time.Second)
	require.NoError(t, err)
	require.NotNil(t, decision)

	// Verify decision
	assert.Equal(t, "BUY", decision.Action)
	assert.Equal(t, "BTC/USDT", decision.Symbol)
	assert.Greater(t, decision.Confidence, 0.0)
	assert.NotEmpty(t, decision.Reasoning)
	assert.Contains(t, decision.BasedOn, topic)
}

func TestMockOrchestrator_MajorityVoting(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topics := []string{
		"agents.analysis.technical",
		"agents.analysis.orderbook",
		"agents.analysis.sentiment",
	}

	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   topics,
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish 2 BUY signals and 1 SELL signal
	buySignal1 := TestTechnicalSignal{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
		Reasoning:  "Technical buy",
	}
	data1, _ := json.Marshal(buySignal1)
	nc.Publish(topics[0], data1)

	buySignal2 := TestOrderBookSignal{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.80,
		Reasoning:  "Orderbook buy",
	}
	data2, _ := json.Marshal(buySignal2)
	nc.Publish(topics[1], data2)

	sellSignal := TestSentimentSignal{
		Symbol:     "BTC/USDT",
		Signal:     "SELL",
		Confidence: 0.60,
		Reasoning:  "Sentiment sell",
	}
	data3, _ := json.Marshal(sellSignal)
	nc.Publish(topics[2], data3)

	// Wait for decision
	decision, err := mo.WaitForDecision(2 * time.Second)
	require.NoError(t, err)
	require.NotNil(t, decision)

	// Should decide BUY due to majority voting (0.85 + 0.80 > 0.60)
	assert.Equal(t, "BUY", decision.Action)
}

func TestMockOrchestrator_CustomDecisionPolicy(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topic := "test.custom.policy"

	// Custom policy that always returns HOLD
	customPolicy := func(signals []ReceivedSignal) *Decision {
		if len(signals) == 0 {
			return nil
		}
		return &Decision{
			Action:     "HOLD",
			Symbol:     "CUSTOM",
			Confidence: 1.0,
			Reasoning:  "Custom policy always holds",
		}
	}

	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn:       nc,
		Topics:         []string{topic},
		DecisionPolicy: customPolicy,
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish a BUY signal (should be ignored by custom policy)
	signal := TestTechnicalSignal{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.95,
		Reasoning:  "Strong buy",
	}
	data, _ := json.Marshal(signal)
	nc.Publish(topic, data)

	// Wait for decision
	decision, err := mo.WaitForDecision(2 * time.Second)
	require.NoError(t, err)
	require.NotNil(t, decision)

	// Should use custom policy
	assert.Equal(t, "HOLD", decision.Action)
	assert.Equal(t, "CUSTOM", decision.Symbol)
	assert.Equal(t, 1.0, decision.Confidence)
	assert.Equal(t, "Custom policy always holds", decision.Reasoning)
}

func TestMockOrchestrator_Reset(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topic := "test.reset"
	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{topic},
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish a signal
	signal := TestTechnicalSignal{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
		Reasoning:  "Test signal",
	}
	data, _ := json.Marshal(signal)
	nc.Publish(topic, data)

	// Wait for signal and decision
	mo.WaitForSignals(1, 2*time.Second)
	mo.WaitForDecision(2 * time.Second)

	// Verify data exists
	assert.NotEmpty(t, mo.GetReceivedSignals())
	assert.NotEmpty(t, mo.GetDecisions())

	// Reset
	mo.Reset()

	// Verify data cleared
	assert.Empty(t, mo.GetReceivedSignals())
	assert.Empty(t, mo.GetDecisions())
	assert.Equal(t, 0, mo.GetSignalCount(topic))
	assert.Nil(t, mo.GetLastSignal(topic))
	assert.Nil(t, mo.GetLastDecision())
}

func TestMockOrchestrator_GettersThreadSafety(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	topic := "test.thread.safety"
	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{topic},
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Publish multiple signals concurrently
	for i := 0; i < 10; i++ {
		signal := TestTechnicalSignal{
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.85,
			Reasoning:  "Concurrent signal",
		}
		data, _ := json.Marshal(signal)
		go nc.Publish(topic, data)
	}

	// Wait for signals
	mo.WaitForSignals(10, 5*time.Second)

	// Concurrent reads should not panic
	done := make(chan bool, 4)
	for i := 0; i < 4; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic during concurrent read: %v", r)
				}
				done <- true
			}()

			// Multiple concurrent reads
			for j := 0; j < 100; j++ {
				mo.GetReceivedSignals()
				mo.GetDecisions()
				mo.GetSignalCount(topic)
				mo.GetLastSignal(topic)
				mo.GetLastDecision()
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestMockOrchestrator_WaitTimeout(t *testing.T) {
	nc := setupNATS(t)
	defer nc.Close()

	mo, err := NewMockOrchestrator(MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"test.timeout"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mo.Start(ctx)
	require.NoError(t, err)
	defer mo.Stop()

	// Wait for signals that won't arrive
	err = mo.WaitForSignals(10, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")

	// Wait for decision that won't happen
	decision, err := mo.WaitForDecision(100 * time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Nil(t, decision)
}

func TestDefaultDecisionPolicy_EmptySignals(t *testing.T) {
	decision := DefaultDecisionPolicy(nil)
	assert.Nil(t, decision)

	decision = DefaultDecisionPolicy([]ReceivedSignal{})
	assert.Nil(t, decision)
}

func TestDefaultDecisionPolicy_BuySignal(t *testing.T) {
	signal := ReceivedSignal{
		Topic: "test.topic",
		Signal: struct {
			Symbol     string  `json:"symbol"`
			Signal     string  `json:"signal"`
			Confidence float64 `json:"confidence"`
		}{
			Symbol:     "BTC/USDT",
			Signal:     "BUY",
			Confidence: 0.90,
		},
	}

	decision := DefaultDecisionPolicy([]ReceivedSignal{signal})
	require.NotNil(t, decision)
	assert.Equal(t, "BUY", decision.Action)
	assert.Equal(t, "BTC/USDT", decision.Symbol)
	assert.Greater(t, decision.Confidence, 0.0)
}

func TestDefaultDecisionPolicy_SellSignal(t *testing.T) {
	signal := ReceivedSignal{
		Topic: "test.topic",
		Signal: struct {
			Symbol     string  `json:"symbol"`
			Signal     string  `json:"signal"`
			Confidence float64 `json:"confidence"`
		}{
			Symbol:     "ETH/USDT",
			Signal:     "SELL",
			Confidence: 0.85,
		},
	}

	decision := DefaultDecisionPolicy([]ReceivedSignal{signal})
	require.NotNil(t, decision)
	assert.Equal(t, "SELL", decision.Action)
	assert.Equal(t, "ETH/USDT", decision.Symbol)
}
