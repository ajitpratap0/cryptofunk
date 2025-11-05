package testing_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/agents/testing"
	"github.com/nats-io/nats.go"
)

// ExampleMockOrchestrator demonstrates basic usage of the MockOrchestrator
func ExampleMockOrchestrator() {
	// Connect to NATS
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		fmt.Printf("Failed to connect to NATS: %v\n", err)
		return
	}
	defer nc.Close()

	// Create mock orchestrator
	mo, err := testing.NewMockOrchestrator(testing.MockOrchestratorConfig{
		NATSConn: nc,
		Topics: []string{
			"agents.analysis.technical",
			"agents.analysis.orderbook",
		},
	})
	if err != nil {
		fmt.Printf("Failed to create mock orchestrator: %v\n", err)
		return
	}

	// Start listening for signals
	ctx := context.Background()
	if err := mo.Start(ctx); err != nil {
		fmt.Printf("Failed to start mock orchestrator: %v\n", err)
		return
	}
	defer func() { _ = mo.Stop() }() // Example cleanup

	// Simulate agent publishing a signal
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
	_ = nc.Publish("agents.analysis.technical", data) // Example - error acceptable

	// Wait for signal to be received
	if err := mo.WaitForSignals(1, 2*time.Second); err != nil {
		fmt.Printf("Failed to receive signal: %v\n", err)
		return
	}

	// Get received signals
	signals := mo.GetReceivedSignals()
	fmt.Printf("Received %d signal(s)\n", len(signals))

	// Wait for decision
	decision, err := mo.WaitForDecision(2 * time.Second)
	if err != nil {
		fmt.Printf("Failed to get decision: %v\n", err)
		return
	}

	fmt.Printf("Decision: %s %s (confidence: %.2f)\n",
		decision.Action, decision.Symbol, decision.Confidence)

	// Example output when NATS is available:
	// Received 1 signal(s)
	// Decision: BUY BTC/USDT (confidence: 0.85)
}

// ExampleMockOrchestrator_customDecisionPolicy demonstrates custom decision-making
func ExampleMockOrchestrator_customDecisionPolicy() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		fmt.Printf("Failed to connect to NATS: %v\n", err)
		return
	}
	defer nc.Close()

	// Custom policy that requires >0.90 confidence for BUY
	customPolicy := func(signals []testing.ReceivedSignal) *testing.Decision {
		if len(signals) == 0 {
			return nil
		}

		// Extract signal data
		var totalConfidence float64
		var lastSymbol string

		for _, sig := range signals {
			if gs, ok := sig.Signal.(struct {
				Symbol     string  `json:"symbol"`
				Signal     string  `json:"signal"`
				Confidence float64 `json:"confidence"`
			}); ok {
				totalConfidence += gs.Confidence
				lastSymbol = gs.Symbol
			}
		}

		avgConfidence := totalConfidence / float64(len(signals))

		// High confidence threshold
		if avgConfidence > 0.90 {
			return &testing.Decision{
				Action:     "BUY",
				Symbol:     lastSymbol,
				Confidence: avgConfidence,
				Reasoning:  "High confidence threshold met",
			}
		}

		return &testing.Decision{
			Action:     "HOLD",
			Symbol:     lastSymbol,
			Confidence: avgConfidence,
			Reasoning:  "Confidence threshold not met",
		}
	}

	// Create orchestrator with custom policy
	mo, err := testing.NewMockOrchestrator(testing.MockOrchestratorConfig{
		NATSConn:       nc,
		Topics:         []string{"agents.analysis.technical"},
		DecisionPolicy: customPolicy,
	})
	if err != nil {
		fmt.Printf("Failed to create mock orchestrator: %v\n", err)
		return
	}

	ctx := context.Background()
	if err := mo.Start(ctx); err != nil {
		fmt.Printf("Failed to start: %v\n", err)
		return
	}
	defer func() { _ = mo.Stop() }() // Example cleanup

	// Publish low confidence signal
	signal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{
		Symbol:     "ETH/USDT",
		Signal:     "BUY",
		Confidence: 0.80, // Below 0.90 threshold
	}

	data, _ := json.Marshal(signal)
	_ = nc.Publish("agents.analysis.technical", data) // Example - error acceptable

	_ = mo.WaitForSignals(1, 2*time.Second) // Example - timeout acceptable
	decision, _ := mo.WaitForDecision(2 * time.Second)

	fmt.Printf("Decision: %s (reasoning: %s)\n",
		decision.Action, decision.Reasoning)

	// Example output when NATS is available:
	// Decision: HOLD (reasoning: Confidence threshold not met)
}

// ExampleMockOrchestrator_multipleAgents demonstrates aggregating signals from multiple agents
func ExampleMockOrchestrator_multipleAgents() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		fmt.Printf("Failed to connect to NATS: %v\n", err)
		return
	}
	defer nc.Close()

	mo, err := testing.NewMockOrchestrator(testing.MockOrchestratorConfig{
		NATSConn: nc,
		Topics: []string{
			"agents.analysis.technical",
			"agents.analysis.orderbook",
			"agents.analysis.sentiment",
		},
	})
	if err != nil {
		fmt.Printf("Failed to create mock orchestrator: %v\n", err)
		return
	}

	ctx := context.Background()
	_ = mo.Start(ctx)                // Example - error logged
	defer func() { _ = mo.Stop() }() // Example cleanup

	// Technical agent signal
	techSignal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{Symbol: "BTC/USDT", Signal: "BUY", Confidence: 0.85}
	techData, _ := json.Marshal(techSignal)
	_ = nc.Publish("agents.analysis.technical", techData) // Example - error acceptable

	// Order book agent signal
	obSignal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{Symbol: "BTC/USDT", Signal: "BUY", Confidence: 0.80}
	obData, _ := json.Marshal(obSignal)
	_ = nc.Publish("agents.analysis.orderbook", obData) // Example - error acceptable

	// Sentiment agent signal
	sentSignal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{Symbol: "BTC/USDT", Signal: "BULLISH", Confidence: 0.75}
	sentData, _ := json.Marshal(sentSignal)
	_ = nc.Publish("agents.analysis.sentiment", sentData) // Example - error acceptable

	// Wait for all signals
	_ = mo.WaitForSignals(3, 2*time.Second) // Example - timeout acceptable

	// Check signal counts per topic
	fmt.Printf("Technical signals: %d\n", mo.GetSignalCount("agents.analysis.technical"))
	fmt.Printf("Order book signals: %d\n", mo.GetSignalCount("agents.analysis.orderbook"))
	fmt.Printf("Sentiment signals: %d\n", mo.GetSignalCount("agents.analysis.sentiment"))

	// Get decision based on all signals
	decision, _ := mo.WaitForDecision(2 * time.Second)
	fmt.Printf("Final decision: %s (confidence: %.2f)\n",
		decision.Action, decision.Confidence)

	// Example output when NATS is available:
	// Technical signals: 1
	// Order book signals: 1
	// Sentiment signals: 1
	// Final decision: BUY (confidence: 0.80)
}

// ExampleMockOrchestrator_testing demonstrates using MockOrchestrator in tests
func ExampleMockOrchestrator_testing() {
	// This example shows how to use MockOrchestrator to test that agents
	// properly publish signals to NATS

	nc, _ := nats.Connect(nats.DefaultURL)
	defer nc.Close()

	mo, _ := testing.NewMockOrchestrator(testing.MockOrchestratorConfig{
		NATSConn: nc,
		Topics:   []string{"agents.analysis.technical"},
	})

	ctx := context.Background()
	_ = mo.Start(ctx)                // Example - error logged
	defer func() { _ = mo.Stop() }() // Example cleanup

	// Simulate agent publishing
	signal := struct {
		Symbol     string  `json:"symbol"`
		Signal     string  `json:"signal"`
		Confidence float64 `json:"confidence"`
	}{Symbol: "BTC/USDT", Signal: "BUY", Confidence: 0.90}

	data, _ := json.Marshal(signal)
	_ = nc.Publish("agents.analysis.technical", data) // Example - error acceptable

	// Verify signal was received
	if err := mo.WaitForSignals(1, 2*time.Second); err != nil {
		fmt.Printf("Test failed: signal not received\n")
		return
	}

	// Verify signal content
	lastSignal := mo.GetLastSignal("agents.analysis.technical")
	if lastSignal != nil {
		fmt.Println("Test passed: signal received and recorded")
	}

	// Verify decision was made
	decision := mo.GetLastDecision()
	if decision != nil && decision.Action == "BUY" {
		fmt.Println("Test passed: correct decision made")
	}

	// Example output when NATS is available:
	// Test passed: signal received and recorded
	// Test passed: correct decision made
}
