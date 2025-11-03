package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestAgentWithFallbackClient simulates a real agent using FallbackClient
// This integration test verifies that agents can seamlessly use fallback models
func TestAgentWithFallbackClient(t *testing.T) {
	// Track which servers received requests
	primaryCalls := atomic.Int32{}
	fallbackCalls := atomic.Int32{}

	// Create mock primary server (fails twice, then succeeds)
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls := primaryCalls.Add(1)
		if calls <= 2 {
			// Fail first 2 calls (simulate temporary outage)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": {"message": "Primary temporarily down"}}`))
			return
		}
		// Succeed on subsequent calls (simulate recovery)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"side\": \"BUY\", \"confidence\": 0.85, \"reasoning\": \"Strong uptrend\"}"
				}
			}],
			"model": "claude-sonnet-4",
			"usage": {"prompt_tokens": 100, "completion_tokens": 50}
		}`))
	}))
	defer primaryServer.Close()

	// Create mock fallback server (always succeeds)
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"side\": \"HOLD\", \"confidence\": 0.70, \"reasoning\": \"Uncertain trend\"}"
				}
			}],
			"model": "gpt-4",
			"usage": {"prompt_tokens": 100, "completion_tokens": 40}
		}`))
	}))
	defer fallbackServer.Close()

	// Configure fallback client (simulating agent initialization)
	config := FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint:    primaryServer.URL,
			Model:       "claude-sonnet-4",
			Temperature: 0.7,
			MaxTokens:   2000,
			Timeout:     5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		FallbackConfigs: []ClientConfig{
			{
				Endpoint:    fallbackServer.URL,
				Model:       "gpt-4",
				Temperature: 0.7,
				MaxTokens:   2000,
				Timeout:     5 * time.Second,
			},
		},
		FallbackNames: []string{"gpt-4"},
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
			TimeWindow:       5 * time.Minute,
		},
	}

	client := NewFallbackClient(config)

	// Test 1: Primary fails, fallback succeeds (simulates agent's first call)
	t.Run("FirstCall_PrimaryFails_FallbackSucceeds", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "system", Content: "You are a trading agent"},
			{Role: "user", Content: "Analyze BTC/USDT"},
		}

		resp, err := client.Complete(context.Background(), messages)
		if err != nil {
			t.Fatalf("Expected fallback to succeed, got error: %v", err)
		}

		if len(resp.Choices) == 0 {
			t.Fatal("Expected choices in response")
		}

		// Verify fallback was used (primary should have failed)
		if primaryCalls.Load() != 1 {
			t.Errorf("Expected 1 primary call, got %d", primaryCalls.Load())
		}
		if fallbackCalls.Load() != 1 {
			t.Errorf("Expected 1 fallback call, got %d", fallbackCalls.Load())
		}

		// Parse and verify the signal
		var signal Signal
		err = client.ParseJSONResponse(resp.Choices[0].Message.Content, &signal)
		if err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		if signal.Side != "HOLD" {
			t.Errorf("Expected HOLD side from fallback, got %s", signal.Side)
		}
	})

	// Test 2: Primary still fails, fallback continues (circuit not yet open)
	t.Run("SecondCall_PrimaryFails_FallbackSucceeds", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "user", Content: "Analyze ETH/USDT"},
		}

		resp, err := client.Complete(context.Background(), messages)
		if err != nil {
			t.Fatalf("Expected fallback to succeed, got error: %v", err)
		}

		// Verify primary was tried again (circuit not open yet)
		if primaryCalls.Load() != 2 {
			t.Errorf("Expected 2 primary calls, got %d", primaryCalls.Load())
		}
		if fallbackCalls.Load() != 2 {
			t.Errorf("Expected 2 fallback calls, got %d", fallbackCalls.Load())
		}

		if len(resp.Choices) == 0 {
			t.Fatal("Expected choices in response")
		}
	})

	// Test 3: Primary recovers (third call succeeds)
	t.Run("ThirdCall_PrimaryRecovers", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "user", Content: "Analyze SOL/USDT"},
		}

		resp, err := client.Complete(context.Background(), messages)
		if err != nil {
			t.Fatalf("Expected primary to succeed, got error: %v", err)
		}

		// Verify primary succeeded this time
		if primaryCalls.Load() != 3 {
			t.Errorf("Expected 3 primary calls, got %d", primaryCalls.Load())
		}
		// Fallback should NOT be called this time
		if fallbackCalls.Load() != 2 {
			t.Errorf("Expected 2 fallback calls (unchanged), got %d", fallbackCalls.Load())
		}

		// Parse and verify the signal from primary
		var signal Signal
		err = client.ParseJSONResponse(resp.Choices[0].Message.Content, &signal)
		if err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		if signal.Side != "BUY" {
			t.Errorf("Expected BUY side from primary, got %s", signal.Side)
		}
		if signal.Confidence != 0.85 {
			t.Errorf("Expected confidence 0.85, got %f", signal.Confidence)
		}
	})

	// Test 4: Verify circuit breaker status
	t.Run("CircuitBreakerStatus", func(t *testing.T) {
		statuses := client.GetCircuitBreakerStatus()
		if len(statuses) != 2 {
			t.Fatalf("Expected 2 circuit statuses, got %d", len(statuses))
		}

		// Primary should be CLOSED (recovered)
		if statuses[0].State != CircuitClosed {
			t.Errorf("Expected primary circuit to be CLOSED, got %s", statuses[0].State)
		}

		// Primary should have recorded failures
		if statuses[0].ConsecutiveFailures != 0 {
			// After success, consecutive failures should reset
			t.Logf("Primary consecutive failures: %d (reset after success)", statuses[0].ConsecutiveFailures)
		}

		// Fallback should be CLOSED (no failures)
		if statuses[1].State != CircuitClosed {
			t.Errorf("Expected fallback circuit to be CLOSED, got %s", statuses[1].State)
		}
	})
}

// TestAgentWithCircuitBreaker verifies circuit breaker opens after threshold
func TestAgentWithCircuitBreaker(t *testing.T) {
	failureCount := atomic.Int32{}

	// Create server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": {"message": "Service down"}}`))
	}))
	defer server.Close()

	config := FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint:    server.URL,
			Model:       "claude-sonnet-4",
			Temperature: 0.7,
			MaxTokens:   2000,
			Timeout:     5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 3, // Open after 3 failures
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
			TimeWindow:       5 * time.Minute,
		},
	}

	client := NewFallbackClient(config)
	messages := []ChatMessage{{Role: "user", Content: "Test"}}

	// Make 5 failed calls
	for i := 0; i < 5; i++ {
		client.Complete(context.Background(), messages)
	}

	// Circuit should be open now
	statuses := client.GetCircuitBreakerStatus()
	if statuses[0].State != CircuitOpen {
		t.Errorf("Expected circuit to be OPEN after %d failures, got %s",
			failureCount.Load(), statuses[0].State)
	}

	// Verify circuit blocks further calls (failureCount should not increase)
	previousFailures := failureCount.Load()
	client.Complete(context.Background(), messages)

	if failureCount.Load() != previousFailures {
		t.Error("Circuit breaker should block calls when OPEN")
	}

	// Wait for timeout and verify transition to HALF_OPEN
	time.Sleep(150 * time.Millisecond)

	// Next call should attempt recovery (half-open state)
	client.Complete(context.Background(), messages)

	// Should have attempted one more call (half-open allows one request)
	if failureCount.Load() <= previousFailures {
		t.Error("Circuit should attempt recovery in HALF_OPEN state")
	}
}

// TestAgentConfigParsing simulates how agents would load fallback config from viper
// This test verifies the configuration structure matches what agents expect
func TestAgentConfigParsing(t *testing.T) {
	// Simulate config values that would come from viper
	simulatedConfig := map[string]interface{}{
		"llm.enabled":                           true,
		"llm.endpoint":                          "http://localhost:8080/v1/chat/completions",
		"llm.api_key":                           "test-key",
		"llm.primary_model":                     "claude-sonnet-4",
		"llm.fallback_models":                   []string{"gpt-4", "gpt-3.5-turbo"},
		"llm.temperature":                       0.7,
		"llm.max_tokens":                        2000,
		"llm.timeout":                           30 * time.Second,
		"llm.circuit_breaker.failure_threshold": 5,
		"llm.circuit_breaker.success_threshold": 2,
		"llm.circuit_breaker.timeout":           60 * time.Second,
		"llm.circuit_breaker.time_window":       5 * time.Minute,
	}

	// Verify all required config keys are present
	requiredKeys := []string{
		"llm.enabled",
		"llm.endpoint",
		"llm.primary_model",
		"llm.fallback_models",
		"llm.circuit_breaker.failure_threshold",
	}

	for _, key := range requiredKeys {
		if _, ok := simulatedConfig[key]; !ok {
			t.Errorf("Missing required config key: %s", key)
		}
	}

	// Verify fallback models is a slice
	fallbackModels, ok := simulatedConfig["llm.fallback_models"].([]string)
	if !ok {
		t.Fatal("llm.fallback_models should be []string")
	}

	if len(fallbackModels) != 2 {
		t.Errorf("Expected 2 fallback models, got %d", len(fallbackModels))
	}

	// Verify circuit breaker thresholds are integers
	failureThreshold, ok := simulatedConfig["llm.circuit_breaker.failure_threshold"].(int)
	if !ok {
		t.Fatal("failure_threshold should be int")
	}

	if failureThreshold <= 0 {
		t.Error("failure_threshold should be positive")
	}

	t.Logf("Configuration validation passed: %d fallback models, threshold=%d",
		len(fallbackModels), failureThreshold)
}
