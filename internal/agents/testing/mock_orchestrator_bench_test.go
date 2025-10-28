package testing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agenttest "github.com/ajitpratap0/cryptofunk/internal/agents/testing"
	"github.com/nats-io/nats.go"
)

// BenchmarkMockOrchestrator_SignalProcessing measures signal processing latency
func BenchmarkMockOrchestrator_SignalProcessing(b *testing.B) {
	// Skip if NATS not available
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		b.Skip("NATS not available - skipping benchmark")
	}
	defer nc.Close()

	// Create mock orchestrator
	mo, err := agenttest.NewMockOrchestrator(agenttest.MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"bench.signal.processing"},
	})
	if err != nil {
		b.Fatalf("Failed to create mock orchestrator: %v", err)
	}

	ctx := context.Background()
	if err := mo.Start(ctx); err != nil {
		b.Fatalf("Failed to start mock orchestrator: %v", err)
	}
	defer mo.Stop()

	// Prepare test signal
	signal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
	}
	data, _ := json.Marshal(signal)

	// Reset timer before benchmark loop
	b.ResetTimer()

	// Benchmark signal processing
	for i := 0; i < b.N; i++ {
		nc.Publish("bench.signal.processing", data)
	}

	// Wait for all signals to be processed
	b.StopTimer()
	mo.WaitForSignals(b.N, 10*time.Second)
}

// BenchmarkMockOrchestrator_DecisionMaking measures decision-making latency
func BenchmarkMockOrchestrator_DecisionMaking(b *testing.B) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		b.Skip("NATS not available - skipping benchmark")
	}
	defer nc.Close()

	mo, err := agenttest.NewMockOrchestrator(agenttest.MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"bench.decision.making"},
	})
	if err != nil {
		b.Fatalf("Failed to create mock orchestrator: %v", err)
	}

	ctx := context.Background()
	mo.Start(ctx)
	defer mo.Stop()

	signal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
	}
	data, _ := json.Marshal(signal)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mo.Reset()
		nc.Publish("bench.decision.making", data)
		mo.WaitForDecision(5 * time.Second)
	}
}

// BenchmarkMockOrchestrator_MultipleSignals measures performance with multiple signals
func BenchmarkMockOrchestrator_MultipleSignals(b *testing.B) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		b.Skip("NATS not available - skipping benchmark")
	}
	defer nc.Close()

	mo, err := agenttest.NewMockOrchestrator(agenttest.MockOrchestratorConfig{
		NATSConn: nc,
		Topics: []string{
			"bench.multi.technical",
			"bench.multi.orderbook",
			"bench.multi.sentiment",
		},
	})
	if err != nil {
		b.Fatalf("Failed to create mock orchestrator: %v", err)
	}

	ctx := context.Background()
	mo.Start(ctx)
	defer mo.Stop()

	// Prepare different signals
	signals := []struct {
		topic string
		data  []byte
	}{
		{
			topic: "bench.multi.technical",
			data: func() []byte {
				s := struct {
					Symbol     string  `json:"symbol"`
					Signal     string  `json:"signal"`
					Confidence float64 `json:"confidence"`
				}{"BTC/USDT", "BUY", 0.85}
				d, _ := json.Marshal(s)
				return d
			}(),
		},
		{
			topic: "bench.multi.orderbook",
			data: func() []byte {
				s := struct {
					Symbol     string  `json:"symbol"`
					Signal     string  `json:"signal"`
					Confidence float64 `json:"confidence"`
				}{"BTC/USDT", "BUY", 0.80}
				d, _ := json.Marshal(s)
				return d
			}(),
		},
		{
			topic: "bench.multi.sentiment",
			data: func() []byte {
				s := struct {
					Symbol     string  `json:"symbol"`
					Signal     string  `json:"signal"`
					Confidence float64 `json:"confidence"`
				}{"BTC/USDT", "BULLISH", 0.75}
				d, _ := json.Marshal(s)
				return d
			}(),
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mo.Reset()
		for _, sig := range signals {
			nc.Publish(sig.topic, sig.data)
		}
		mo.WaitForSignals(3, 5*time.Second)
	}
}

// BenchmarkMockOrchestrator_ConcurrentSignals measures concurrent signal handling
func BenchmarkMockOrchestrator_ConcurrentSignals(b *testing.B) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		b.Skip("NATS not available - skipping benchmark")
	}
	defer nc.Close()

	mo, err := agenttest.NewMockOrchestrator(agenttest.MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"bench.concurrent"},
	})
	if err != nil {
		b.Fatalf("Failed to create mock orchestrator: %v", err)
	}

	ctx := context.Background()
	mo.Start(ctx)
	defer mo.Stop()

	signal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
	}
	data, _ := json.Marshal(signal)

	b.ResetTimer()

	// Run concurrent signal publishing
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			nc.Publish("bench.concurrent", data)
		}
	})

	b.StopTimer()
}

// BenchmarkDefaultDecisionPolicy measures decision policy performance
func BenchmarkDefaultDecisionPolicy(b *testing.B) {
	// Prepare test signals
	signals := []agenttest.ReceivedSignal{
		{
			Topic: "agents.analysis.technical",
			Signal: struct {
				Symbol     string  `json:"symbol"`
				Signal     string  `json:"signal"`
				Confidence float64 `json:"confidence"`
			}{
				Symbol:     "BTC/USDT",
				Signal:     "BUY",
				Confidence: 0.85,
			},
			Timestamp: time.Now(),
		},
		{
			Topic: "agents.analysis.orderbook",
			Signal: struct {
				Symbol     string  `json:"symbol"`
				Signal     string  `json:"signal"`
				Confidence float64 `json:"confidence"`
			}{
				Symbol:     "BTC/USDT",
				Signal:     "BUY",
				Confidence: 0.80,
			},
			Timestamp: time.Now(),
		},
		{
			Topic: "agents.analysis.sentiment",
			Signal: struct {
				Symbol     string  `json:"symbol"`
				Signal     string  `json:"signal"`
				Confidence float64 `json:"confidence"`
			}{
				Symbol:     "BTC/USDT",
				Signal:     "BULLISH",
				Confidence: 0.75,
			},
			Timestamp: time.Now(),
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		agenttest.DefaultDecisionPolicy(signals)
	}
}

// BenchmarkMockOrchestrator_GettersUnderLoad measures getter performance under concurrent load
func BenchmarkMockOrchestrator_GettersUnderLoad(b *testing.B) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		b.Skip("NATS not available - skipping benchmark")
	}
	defer nc.Close()

	mo, err := agenttest.NewMockOrchestrator(agenttest.MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"bench.getters"},
	})
	if err != nil {
		b.Fatalf("Failed to create mock orchestrator: %v", err)
	}

	ctx := context.Background()
	mo.Start(ctx)
	defer mo.Stop()

	// Populate with some signals
	signal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{
		Symbol:     "BTC/USDT",
		Signal:     "BUY",
		Confidence: 0.85,
	}
	data, _ := json.Marshal(signal)

	for i := 0; i < 100; i++ {
		nc.Publish("bench.getters", data)
	}
	mo.WaitForSignals(100, 5*time.Second)

	b.ResetTimer()

	// Benchmark concurrent getter access
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mo.GetReceivedSignals()
			mo.GetDecisions()
			mo.GetSignalCount("bench.getters")
			mo.GetLastSignal("bench.getters")
			mo.GetLastDecision()
		}
	})
}
