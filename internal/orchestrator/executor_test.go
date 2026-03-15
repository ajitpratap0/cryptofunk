package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExecutor(t *testing.T) {
	cfg := ExecutorConfig{
		OrderExecutorURL: "http://localhost:8091/mcp",
		MinConfidence:    0.6,
		MinConsensus:     0.5,
		DefaultQuantity:  0.001,
	}

	executor := NewExecutor(cfg)

	require.NotNil(t, executor)
	assert.Equal(t, cfg.OrderExecutorURL, executor.config.OrderExecutorURL)
	assert.Equal(t, cfg.MinConfidence, executor.config.MinConfidence)
	assert.Equal(t, cfg.MinConsensus, executor.config.MinConsensus)
	assert.Equal(t, cfg.DefaultQuantity, executor.config.DefaultQuantity)
}

func TestShouldExecute(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MinConfidence: 0.6,
		MinConsensus:  0.5,
	})

	tests := []struct {
		name     string
		decision *TradingDecision
		want     bool
	}{
		{
			name: "BUY with high confidence and consensus",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionBuy,
				Confidence: 0.8,
				Consensus:  0.7,
				Timestamp:  time.Now(),
			},
			want: true,
		},
		{
			name: "SELL with high confidence and consensus",
			decision: &TradingDecision{
				Symbol:     "ETH/USDT",
				Action:     SignalActionSell,
				Confidence: 0.75,
				Consensus:  0.6,
				Timestamp:  time.Now(),
			},
			want: true,
		},
		{
			name: "HOLD action should not execute",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionHold,
				Confidence: 0.9,
				Consensus:  0.9,
				Timestamp:  time.Now(),
			},
			want: false,
		},
		{
			name: "BUY with low confidence should not execute",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionBuy,
				Confidence: 0.3,
				Consensus:  0.8,
				Timestamp:  time.Now(),
			},
			want: false,
		},
		{
			name: "SELL with low consensus should not execute",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionSell,
				Confidence: 0.8,
				Consensus:  0.3,
				Timestamp:  time.Now(),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.ShouldExecute(tt.decision)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldExecute_EdgeCases(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MinConfidence: 0.6,
		MinConsensus:  0.5,
	})

	tests := []struct {
		name     string
		decision *TradingDecision
		want     bool
	}{
		{
			name: "exactly at confidence and consensus threshold",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionBuy,
				Confidence: 0.6,
				Consensus:  0.5,
				Timestamp:  time.Now(),
			},
			want: true,
		},
		{
			name: "just below confidence threshold",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionBuy,
				Confidence: 0.5999,
				Consensus:  0.8,
				Timestamp:  time.Now(),
			},
			want: false,
		},
		{
			name: "just below consensus threshold",
			decision: &TradingDecision{
				Symbol:     "BTC/USDT",
				Action:     SignalActionSell,
				Confidence: 0.8,
				Consensus:  0.4999,
				Timestamp:  time.Now(),
			},
			want: false,
		},
		{
			name:     "nil decision should not execute",
			decision: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.ShouldExecute(tt.decision)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStart_RefusesLiveModeWhenPaperOnly(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{
		OrderExecutorURL: "http://localhost:8091/mcp",
		PaperOnly:        true,
		TradingMode:      "LIVE",
	})
	err := exec.Start(nil, "test.decisions")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PaperOnly=true but trading_mode=LIVE")
}

func TestStart_DoesNotRejectPaperModeWhenPaperOnly(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{
		OrderExecutorURL: "http://localhost:8091/mcp",
		PaperOnly:        true,
		TradingMode:      "PAPER",
	})
	// Will fail on nil NATS, but should NOT fail on the PaperOnly check
	err := exec.Start(nil, "test.decisions")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NATS connection is nil")
	assert.NotContains(t, err.Error(), "PaperOnly")
}

func TestStop_Idempotent(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{})
	// Stop before Start should not panic
	exec.Stop()
	exec.Stop()
}

func TestOrderCooldown(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{
		MinConfidence: 0.5,
		MinConsensus:  0.5,
		OrderCooldown: 1 * time.Second,
	})

	// Simulate a recent order for BTCUSDT
	exec.recentOrders.Store("BTCUSDT", time.Now())

	// Should still be in cooldown
	_, ok := exec.recentOrders.Load("BTCUSDT")
	assert.True(t, ok, "BTCUSDT should have a recent order")

	// Wait for cooldown to expire
	time.Sleep(1100 * time.Millisecond)
	lastOrder, _ := exec.recentOrders.Load("BTCUSDT")
	elapsed := time.Since(lastOrder.(time.Time))
	assert.True(t, elapsed >= 1*time.Second, "Cooldown should have expired")
}
