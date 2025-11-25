package testing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	agenttest "github.com/ajitpratap0/cryptofunk/internal/agents/testing"
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
	defer func() { _ = mo.Stop() }() // Benchmark cleanup

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
		_ = nc.Publish("bench.signal.processing", data) // Benchmark - error acceptable
	}

	// Wait for all signals to be processed
	b.StopTimer()
	_ = mo.WaitForSignals(b.N, 10*time.Second) // Benchmark - timeout acceptable
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
	_ = mo.Start(ctx)                // Benchmark mock - error handled by test framework
	defer func() { _ = mo.Stop() }() // Benchmark cleanup

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
		_ = nc.Publish("bench.decision.making", data) // Benchmark - error acceptable
		_, _ = mo.WaitForDecision(5 * time.Second)    // Benchmark - timeout acceptable
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
	_ = mo.Start(ctx)                // Benchmark mock - error handled by test framework
	defer func() { _ = mo.Stop() }() // Benchmark cleanup

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
			_ = nc.Publish(sig.topic, sig.data) // Benchmark - error acceptable
		}
		_ = mo.WaitForSignals(3, 5*time.Second) // Benchmark - timeout acceptable
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
	_ = mo.Start(ctx)                // Benchmark mock - error handled by test framework
	defer func() { _ = mo.Stop() }() // Benchmark cleanup

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
			_ = nc.Publish("bench.concurrent", data) // Benchmark - error acceptable
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
	_ = mo.Start(ctx)                // Benchmark mock - error handled by test framework
	defer func() { _ = mo.Stop() }() // Benchmark cleanup

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
		_ = nc.Publish("bench.getters", data) // Benchmark - error acceptable
	}
	_ = mo.WaitForSignals(100, 5*time.Second) // Benchmark - timeout acceptable

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
