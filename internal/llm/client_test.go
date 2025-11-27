//nolint:goconst // Test files use repeated strings for clarity
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Complete(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		wantError     bool
		wantRetryable bool
	}{
		{
			name:       "Successful response",
			statusCode: http.StatusOK,
			responseBody: `{
				"id": "test-123",
				"model": "claude-sonnet-4-20250514",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "{\"action\": \"BUY\", \"confidence\": 0.85}"
					}
				}],
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 50,
					"total_tokens": 150
				}
			}`,
			wantError: false,
		},
		{
			name:       "Rate limit error (retryable)",
			statusCode: http.StatusTooManyRequests,
			responseBody: `{
				"error": {
					"message": "Rate limit exceeded",
					"type": "rate_limit_error"
				}
			}`,
			wantError:     true,
			wantRetryable: true,
		},
		{
			name:       "Server error (retryable)",
			statusCode: http.StatusInternalServerError,
			responseBody: `{
				"error": {
					"message": "Internal server error",
					"type": "server_error"
				}
			}`,
			wantError:     true,
			wantRetryable: true,
		},
		{
			name:       "Bad request (non-retryable)",
			statusCode: http.StatusBadRequest,
			responseBody: `{
				"error": {
					"message": "Invalid request format",
					"type": "invalid_request_error"
				}
			}`,
			wantError:     true,
			wantRetryable: false,
		},
		{
			name:       "Unauthorized (non-retryable)",
			statusCode: http.StatusUnauthorized,
			responseBody: `{
				"error": {
					"message": "Invalid API key",
					"type": "authentication_error"
				}
			}`,
			wantError:     true,
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody)) // Test mock response
			}))
			defer server.Close()

			client := NewClient(ClientConfig{
				Endpoint: server.URL,
				APIKey:   "test-key",
				Timeout:  5 * time.Second,
			})

			messages := []ChatMessage{
				{Role: "user", Content: "Test message"},
			}

			resp, err := client.Complete(context.Background(), messages)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}

				// Check if error is properly classified
				if llmErr, ok := err.(*LLMError); ok {
					if llmErr.IsRetryable() != tt.wantRetryable {
						t.Errorf("Expected retryable=%v, got %v", tt.wantRetryable, llmErr.IsRetryable())
					}
				} else {
					t.Error("Expected LLMError type")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("Expected non-nil response")
				}
			}
		})
	}
}

func TestClient_CompleteWithRetry(t *testing.T) {
	tests := []struct {
		name          string
		attempts      []int // Status codes for each attempt
		maxRetries    int
		expectSuccess bool
	}{
		{
			name:          "Success on first attempt",
			attempts:      []int{http.StatusOK},
			maxRetries:    3,
			expectSuccess: true,
		},
		{
			name:          "Success after retry",
			attempts:      []int{http.StatusTooManyRequests, http.StatusOK},
			maxRetries:    3,
			expectSuccess: true,
		},
		{
			name:          "Fail after max retries",
			attempts:      []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests},
			maxRetries:    2,
			expectSuccess: false,
		},
		{
			name:          "Non-retryable error fails immediately",
			attempts:      []int{http.StatusBadRequest},
			maxRetries:    3,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				statusCode := tt.attempts[attemptCount]
				if attemptCount < len(tt.attempts)-1 {
					attemptCount++
				}

				w.WriteHeader(statusCode)
				if statusCode == http.StatusOK {
					_, _ = w.Write([]byte(`{
						"choices": [{
							"message": {"content": "test"}
						}],
						"usage": {"total_tokens": 100}
					}`))
				} else {
					_, _ = w.Write([]byte(`{
						"error": {"message": "Error"}
					}`))
				}
			}))
			defer server.Close()

			client := NewClient(ClientConfig{
				Endpoint: server.URL,
				Timeout:  5 * time.Second,
			})

			messages := []ChatMessage{
				{Role: "user", Content: "Test"},
			}

			_, err := client.CompleteWithRetry(context.Background(), messages, tt.maxRetries)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			}
		})
	}
}

func TestClient_ParseJSONResponse(t *testing.T) {
	type TestResponse struct {
		Action     string  `json:"action"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	tests := []struct {
		name      string
		content   string
		wantError bool
		checkData func(*testing.T, TestResponse)
	}{
		{
			name:      "Plain JSON",
			content:   `{"action": "BUY", "confidence": 0.85, "reasoning": "Strong bullish momentum"}`,
			wantError: false,
			checkData: func(t *testing.T, r TestResponse) {
				if r.Action != "BUY" {
					t.Errorf("Expected action BUY, got %s", r.Action)
				}
				if r.Confidence != 0.85 {
					t.Errorf("Expected confidence 0.85, got %f", r.Confidence)
				}
			},
		},
		{
			name: "JSON in markdown code block",
			content: "```json\n" +
				`{"action": "SELL", "confidence": 0.75, "reasoning": "Overbought conditions"}` + "\n" +
				"```",
			wantError: false,
			checkData: func(t *testing.T, r TestResponse) {
				if r.Action != "SELL" {
					t.Errorf("Expected action SELL, got %s", r.Action)
				}
			},
		},
		{
			name: "JSON in code block without language",
			content: "```\n" +
				`{"action": "HOLD", "confidence": 0.60, "reasoning": "Mixed signals"}` + "\n" +
				"```",
			wantError: false,
			checkData: func(t *testing.T, r TestResponse) {
				if r.Action != "HOLD" {
					t.Errorf("Expected action HOLD, got %s", r.Action)
				}
			},
		},
		{
			name: "JSON with surrounding text",
			content: "Based on the analysis, here is my recommendation:\n\n" +
				`{"action": "BUY", "confidence": 0.90, "reasoning": "Breakout confirmed"}` + "\n\n" +
				"This signal has high confidence.",
			wantError: false,
			checkData: func(t *testing.T, r TestResponse) {
				if r.Action != "BUY" {
					t.Errorf("Expected action BUY, got %s", r.Action)
				}
				if r.Confidence != 0.90 {
					t.Errorf("Expected confidence 0.90, got %f", r.Confidence)
				}
			},
		},
		{
			name: "Nested JSON objects with extra text",
			content: "Analysis complete:\n" +
				`{"action": "SELL", "confidence": 0.80, "reasoning": "Bearish divergence", "metadata": {"strength": "high"}}` + "\n" +
				"Additional notes...",
			wantError: false,
			checkData: func(t *testing.T, r TestResponse) {
				if r.Action != "SELL" {
					t.Errorf("Expected action SELL, got %s", r.Action)
				}
			},
		},
		{
			name:      "Invalid JSON",
			content:   `{action: invalid, no quotes}`,
			wantError: true,
		},
		{
			name:      "No JSON content",
			content:   "This is just plain text with no JSON",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(ClientConfig{})

			var result TestResponse
			err := client.ParseJSONResponse(tt.content, &result)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.checkData != nil {
					tt.checkData(t, result)
				}
			}
		})
	}
}

func TestExtractJSONFromMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "JSON in ```json block",
			content: "```json\n{\"test\": \"value\"}\n```",
			want:    `{"test": "value"}`,
		},
		{
			name:    "JSON in ``` block",
			content: "```\n{\"test\": \"value\"}\n```",
			want:    `{"test": "value"}`,
		},
		{
			name:    "No code block",
			content: "{\"test\": \"value\"}",
			want:    "",
		},
		{
			name:    "Array in code block",
			content: "```json\n[{\"a\": 1}, {\"b\": 2}]\n```",
			want:    `[{"a": 1}, {"b": 2}]`,
		},
		{
			name:    "Non-JSON content in code block",
			content: "```json\nNot valid JSON\n```",
			want:    "", // Returns empty because it doesn't start with { or [
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONFromMarkdown(tt.content)
			// Normalize whitespace for comparison
			gotNorm := strings.TrimSpace(strings.ReplaceAll(got, " ", ""))
			wantNorm := strings.TrimSpace(strings.ReplaceAll(tt.want, " ", ""))

			if gotNorm != wantNorm {
				t.Errorf("extractJSONFromMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractFirstJSONObject(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Simple object",
			content: `{"test": "value"}`,
			want:    `{"test": "value"}`,
		},
		{
			name:    "Object with surrounding text",
			content: `Some text before {"test": "value"} and after`,
			want:    `{"test": "value"}`,
		},
		{
			name:    "Nested object",
			content: `{"outer": {"inner": "value"}}`,
			want:    `{"outer": {"inner": "value"}}`,
		},
		{
			name:    "Array",
			content: `[{"a": 1}, {"b": 2}]`,
			want:    `[{"a": 1}, {"b": 2}]`,
		},
		{
			name:    "Multiple objects (returns first)",
			content: `{"first": 1} {"second": 2}`,
			want:    `{"first": 1}`,
		},
		{
			name:    "No JSON",
			content: `Just plain text`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFirstJSONObject(tt.content)
			// Normalize whitespace
			gotNorm := strings.TrimSpace(strings.ReplaceAll(got, " ", ""))
			wantNorm := strings.TrimSpace(strings.ReplaceAll(tt.want, " ", ""))

			if gotNorm != wantNorm {
				t.Errorf("extractFirstJSONObject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLLMError(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		message       string
		wantRetryable bool
	}{
		{"Rate limit", 429, "Too many requests", true},
		{"Server error", 500, "Internal error", true},
		{"Bad gateway", 502, "Bad gateway", true},
		{"Service unavailable", 503, "Service unavailable", true},
		{"Gateway timeout", 504, "Timeout", true},
		{"Bad request", 400, "Invalid input", false},
		{"Unauthorized", 401, "Invalid API key", false},
		{"Forbidden", 403, "Access denied", false},
		{"Not found", 404, "Not found", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyHTTPError(tt.statusCode, tt.message)

			llmErr, ok := err.(*LLMError)
			if !ok {
				t.Fatal("Expected *LLMError type")
			}

			if llmErr.StatusCode != tt.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.statusCode, llmErr.StatusCode)
			}

			if llmErr.Message != tt.message {
				t.Errorf("Expected message %q, got %q", tt.message, llmErr.Message)
			}

			if llmErr.IsRetryable() != tt.wantRetryable {
				t.Errorf("Expected retryable=%v, got %v", tt.wantRetryable, llmErr.IsRetryable())
			}

			// Test Error() method
			errMsg := llmErr.Error()
			if !strings.Contains(errMsg, tt.message) {
				t.Errorf("Error message should contain %q, got: %s", tt.message, errMsg)
			}
		})
	}
}

func TestClient_CompleteWithSystem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
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
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Test response"
				}
			}],
			"usage": {"total_tokens": 100}
		}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		Endpoint: server.URL,
		Timeout:  5 * time.Second,
	})

	content, err := client.CompleteWithSystem(
		context.Background(),
		"You are a helpful assistant",
		"What is the weather?",
	)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if content != "Test response" {
		t.Errorf("Expected 'Test response', got %q", content)
	}
}

func TestClient_DefaultConfig(t *testing.T) {
	client := NewClient(ClientConfig{})

	if client.endpoint == "" {
		t.Error("Expected default endpoint to be set")
	}
	if client.model == "" {
		t.Error("Expected default model to be set")
	}
	if client.temperature == 0 {
		t.Error("Expected default temperature to be set")
	}
	if client.maxTokens == 0 {
		t.Error("Expected default maxTokens to be set")
	}
	if client.timeout == 0 {
		t.Error("Expected default timeout to be set")
	}

	// Verify default values
	expectedEndpoint := "http://localhost:8080/v1/chat/completions"
	if client.endpoint != expectedEndpoint {
		t.Errorf("Expected endpoint %q, got %q", expectedEndpoint, client.endpoint)
	}

	expectedModel := "claude-sonnet-4-20250514"
	if client.model != expectedModel {
		t.Errorf("Expected model %q, got %q", expectedModel, client.model)
	}

	if client.temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", client.temperature)
	}

	if client.maxTokens != 2000 {
		t.Errorf("Expected maxTokens 2000, got %d", client.maxTokens)
	}

	if client.timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.timeout)
	}
}
