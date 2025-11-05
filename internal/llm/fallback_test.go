//nolint:goconst // Test files use repeated strings for clarity
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFallbackClient_SuccessOnPrimary(t *testing.T) {
	// Create test server for primary model (succeeds)
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content": "Primary response"}}],
			"usage": {"total_tokens": 100}
		}`))
	}))
	defer primaryServer.Close()

	// Create test server for fallback model (should not be called)
	var fallbackCalled atomic.Bool
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalled.Store(true)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content": "Fallback response"}}],
			"usage": {"total_tokens": 100}
		}`))
	}))
	defer fallbackServer.Close()

	// Create fallback client
	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: primaryServer.URL,
			Model:    "claude-sonnet-4",
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		FallbackConfigs: []ClientConfig{
			{
				Endpoint: fallbackServer.URL,
				Model:    "gpt-4",
				Timeout:  5 * time.Second,
			},
		},
		FallbackNames: []string{"gpt-4"},
	})

	messages := []ChatMessage{
		{Role: "user", Content: "Test"},
	}

	resp, err := fc.Complete(context.Background(), messages)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if resp.Choices[0].Message.Content != "Primary response" {
		t.Errorf("Expected primary response, got: %s", resp.Choices[0].Message.Content)
	}

	if fallbackCalled.Load() {
		t.Error("Fallback model should not have been called")
	}
}

func TestFallbackClient_FallbackOnPrimaryFailure(t *testing.T) {
	// Create test server for primary model (fails)
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": {"message": "Service unavailable"}}`)) // Test mock response
	}))
	defer primaryServer.Close()

	// Create test server for fallback model (succeeds)
	var fallbackCalled atomic.Bool
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalled.Store(true)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content": "Fallback response"}}],
			"usage": {"total_tokens": 100}
		}`))
	}))
	defer fallbackServer.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: primaryServer.URL,
			Model:    "claude-sonnet-4",
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		FallbackConfigs: []ClientConfig{
			{
				Endpoint: fallbackServer.URL,
				Model:    "gpt-4",
				Timeout:  5 * time.Second,
			},
		},
		FallbackNames: []string{"gpt-4"},
	})

	messages := []ChatMessage{
		{Role: "user", Content: "Test"},
	}

	resp, err := fc.Complete(context.Background(), messages)

	if err != nil {
		t.Fatalf("Expected success after fallback, got error: %v", err)
	}

	if resp.Choices[0].Message.Content != "Fallback response" {
		t.Errorf("Expected fallback response, got: %s", resp.Choices[0].Message.Content)
	}

	if !fallbackCalled.Load() {
		t.Error("Fallback model should have been called")
	}
}

func TestFallbackClient_AllModelsFail(t *testing.T) {
	// Create test servers that all fail
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": {"message": "Primary unavailable"}}`)) // Test mock response
	}))
	defer primaryServer.Close()

	fallback1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": {"message": "Fallback 1 unavailable"}}`)) // Test mock response
	}))
	defer fallback1Server.Close()

	fallback2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": {"message": "Fallback 2 unavailable"}}`))
	}))
	defer fallback2Server.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: primaryServer.URL,
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		FallbackConfigs: []ClientConfig{
			{Endpoint: fallback1Server.URL, Timeout: 5 * time.Second},
			{Endpoint: fallback2Server.URL, Timeout: 5 * time.Second},
		},
		FallbackNames: []string{"gpt-4", "gpt-3.5-turbo"},
	})

	messages := []ChatMessage{
		{Role: "user", Content: "Test"},
	}

	_, err := fc.Complete(context.Background(), messages)

	if err == nil {
		t.Fatal("Expected error when all models fail")
	}

	if err.Error() != "all models failed, last error: LLM API error (status 503): Fallback 2 unavailable" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestFallbackClient_SkipNonRetryableErrors(t *testing.T) {
	// Primary fails with non-retryable error (400)
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer primaryServer.Close()

	// Fallback succeeds
	var fallbackCalled atomic.Bool
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalled.Store(true)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content": "Fallback response"}}],
			"usage": {"total_tokens": 100}
		}`))
	}))
	defer fallbackServer.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: primaryServer.URL,
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		FallbackConfigs: []ClientConfig{
			{Endpoint: fallbackServer.URL, Timeout: 5 * time.Second},
		},
		FallbackNames: []string{"gpt-4"},
	})

	messages := []ChatMessage{
		{Role: "user", Content: "Test"},
	}

	resp, err := fc.Complete(context.Background(), messages)

	if err != nil {
		t.Fatalf("Expected success after fallback, got error: %v", err)
	}

	if !fallbackCalled.Load() {
		t.Error("Should skip to fallback immediately on non-retryable error")
	}

	if resp.Choices[0].Message.Content != "Fallback response" {
		t.Errorf("Expected fallback response, got: %s", resp.Choices[0].Message.Content)
	}
}

func TestFallbackClient_CompleteWithRetry(t *testing.T) {
	// Primary fails once, then succeeds
	var callCount atomic.Int32
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"message": "Rate limited"}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"choices": [{"message": {"content": "Success after retry"}}],
				"usage": {"total_tokens": 100}
			}`))
		}
	}))
	defer primaryServer.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: primaryServer.URL,
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
	})

	messages := []ChatMessage{
		{Role: "user", Content: "Test"},
	}

	resp, err := fc.CompleteWithRetry(context.Background(), messages, 3)

	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}

	if resp.Choices[0].Message.Content != "Success after retry" {
		t.Errorf("Expected retry success response, got: %s", resp.Choices[0].Message.Content)
	}

	if callCount.Load() != 2 {
		t.Errorf("Expected 2 calls (1 failure + 1 success), got %d", callCount.Load())
	}
}

func TestFallbackClient_CompleteWithSystem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request has system and user messages
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req) // Test mock - decode error handled by test assertions

		if len(req.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("Expected first message to be system, got %s", req.Messages[0].Role)
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("Expected second message to be user, got %s", req.Messages[1].Role)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"content": "System response"}}],
			"usage": {"total_tokens": 100}
		}`))
	}))
	defer server.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
	})

	content, err := fc.CompleteWithSystem(
		context.Background(),
		"You are a helpful assistant",
		"Test question",
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if content != "System response" {
		t.Errorf("Expected 'System response', got %q", content)
	}
}

func TestCircuitBreaker_OpenAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(1, CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		TimeWindow:       5 * time.Minute,
	})

	// Initially closed
	if cb.IsOpen(0) {
		t.Error("Circuit should start closed")
	}

	// Record failures
	cb.RecordFailure(0)
	if cb.IsOpen(0) {
		t.Error("Circuit should not be open after 1 failure")
	}

	cb.RecordFailure(0)
	if cb.IsOpen(0) {
		t.Error("Circuit should not be open after 2 failures")
	}

	cb.RecordFailure(0)
	if !cb.IsOpen(0) {
		t.Error("Circuit should be open after 3 failures")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		TimeWindow:       5 * time.Minute,
	})

	// Open the circuit
	cb.RecordFailure(0)
	cb.RecordFailure(0)

	if !cb.IsOpen(0) {
		t.Fatal("Circuit should be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open and allow request
	if cb.IsOpen(0) {
		t.Error("Circuit should be half-open (allow request) after timeout")
	}
}

func TestCircuitBreaker_CloseAfterSuccessInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		TimeWindow:       5 * time.Minute,
	})

	// Open the circuit
	cb.RecordFailure(0)
	cb.RecordFailure(0)

	// Wait for half-open
	time.Sleep(150 * time.Millisecond)
	cb.IsOpen(0) // Trigger state transition

	// Record successes to close
	cb.RecordSuccess(0)
	status := cb.GetAllStatus()[0]
	if status.State != CircuitHalfOpen {
		t.Error("Should still be half-open after 1 success (need 2)")
	}

	cb.RecordSuccess(0)
	status = cb.GetAllStatus()[0]
	if status.State != CircuitClosed {
		t.Errorf("Should be closed after 2 successes, got state: %s", status.State)
	}
}

func TestCircuitBreaker_ReopenOnFailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		TimeWindow:       5 * time.Minute,
	})

	// Open the circuit
	cb.RecordFailure(0)
	cb.RecordFailure(0)

	// Wait for half-open
	time.Sleep(150 * time.Millisecond)
	cb.IsOpen(0) // Trigger transition

	// Failure in half-open should reopen
	cb.RecordFailure(0)
	status := cb.GetAllStatus()[0]
	if status.State != CircuitOpen {
		t.Errorf("Should reopen after failure in half-open, got state: %s", status.State)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(1, CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		TimeWindow:       5 * time.Minute,
	})

	// Open the circuit
	cb.RecordFailure(0)
	cb.RecordFailure(0)

	if !cb.IsOpen(0) {
		t.Fatal("Circuit should be open")
	}

	// Reset
	cb.Reset(0)

	if cb.IsOpen(0) {
		t.Error("Circuit should be closed after reset")
	}

	status := cb.GetAllStatus()[0]
	if status.ConsecutiveFailures != 0 {
		t.Errorf("Consecutive failures should be 0 after reset, got %d", status.ConsecutiveFailures)
	}
}

func TestCircuitBreaker_MultipleModels(t *testing.T) {
	cb := NewCircuitBreaker(3, CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		TimeWindow:       5 * time.Minute,
	})

	// Open circuit for model 0
	cb.RecordFailure(0)
	cb.RecordFailure(0)

	// Model 0 should be open
	if !cb.IsOpen(0) {
		t.Error("Model 0 circuit should be open")
	}

	// Models 1 and 2 should be closed
	if cb.IsOpen(1) {
		t.Error("Model 1 circuit should be closed")
	}
	if cb.IsOpen(2) {
		t.Error("Model 2 circuit should be closed")
	}

	// Record failure on model 1
	cb.RecordFailure(1)

	// Model 1 should still be closed (only 1 failure)
	if cb.IsOpen(1) {
		t.Error("Model 1 circuit should still be closed")
	}

	statuses := cb.GetAllStatus()
	if len(statuses) != 3 {
		t.Errorf("Expected 3 status entries, got %d", len(statuses))
	}
}

func TestCircuitBreakerStatus(t *testing.T) {
	cb := NewCircuitBreaker(2, CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		TimeWindow:       5 * time.Minute,
	})

	// Record some activity
	cb.RecordFailure(0)
	cb.RecordFailure(0)
	cb.RecordSuccess(1)

	statuses := cb.GetAllStatus()

	// Check model 0 status
	if statuses[0].ConsecutiveFailures != 2 {
		t.Errorf("Expected 2 consecutive failures for model 0, got %d", statuses[0].ConsecutiveFailures)
	}
	if statuses[0].State != CircuitClosed {
		t.Errorf("Expected model 0 to be closed (need 3 failures), got %s", statuses[0].State)
	}

	// Check model 1 status
	if statuses[1].ConsecutiveSuccesses != 1 {
		t.Errorf("Expected 1 consecutive success for model 1, got %d", statuses[1].ConsecutiveSuccesses)
	}
	if statuses[1].State != CircuitClosed {
		t.Errorf("Expected model 1 to be closed, got %s", statuses[1].State)
	}
}

func TestFallbackClient_GetCircuitBreakerStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": {"message": "Unavailable"}}`))
	}))
	defer server.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		FallbackConfigs: []ClientConfig{
			{Endpoint: server.URL, Timeout: 5 * time.Second},
		},
		FallbackNames: []string{"gpt-4"},
	})

	// Trigger some failures
	_, _ = fc.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "Test"}}) // Test circuit breaker - error expected

	statuses := fc.GetCircuitBreakerStatus()

	if len(statuses) != 2 {
		t.Errorf("Expected 2 circuit statuses, got %d", len(statuses))
	}

	// Both models should have recorded failures
	if statuses[0].ConsecutiveFailures == 0 {
		t.Error("Expected model 0 to have recorded failures")
	}
	if statuses[1].ConsecutiveFailures == 0 {
		t.Error("Expected model 1 to have recorded failures")
	}
}

func TestFallbackClient_ResetCircuitBreaker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		PrimaryName: "claude-sonnet-4",
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 1,
			Timeout:          1 * time.Hour, // Long timeout
			TimeWindow:       5 * time.Minute,
		},
	})

	// Open the circuit
	_, _ = fc.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "Test"}}) // Test circuit breaker - error expected

	statuses := fc.GetCircuitBreakerStatus()
	if statuses[0].State != CircuitOpen {
		t.Fatal("Circuit should be open")
	}

	// Reset
	err := fc.ResetCircuitBreaker(0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	statuses = fc.GetCircuitBreakerStatus()
	if statuses[0].State != CircuitClosed {
		t.Errorf("Circuit should be closed after reset, got %s", statuses[0].State)
	}
}

func TestFallbackClient_InvalidModelIndexReset(t *testing.T) {
	fc := NewFallbackClient(FallbackConfig{
		PrimaryConfig: ClientConfig{Timeout: 5 * time.Second},
		PrimaryName:   "test",
	})

	err := fc.ResetCircuitBreaker(10) // Invalid index
	if err == nil {
		t.Error("Expected error for invalid model index")
	}

	err = fc.ResetCircuitBreaker(-1) // Invalid index
	if err == nil {
		t.Error("Expected error for negative model index")
	}
}
