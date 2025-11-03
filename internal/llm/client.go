package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents an LLM client that communicates with Bifrost gateway
type Client struct {
	endpoint    string
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
	timeout     time.Duration
	httpClient  *http.Client
}

// ClientConfig contains configuration for the LLM client
type ClientConfig struct {
	Endpoint    string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

// NewClient creates a new LLM client
func NewClient(config ClientConfig) *Client {
	if config.Endpoint == "" {
		config.Endpoint = "http://localhost:8080/v1/chat/completions"
	}
	if config.Model == "" {
		config.Model = "claude-sonnet-4-20250514"
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 2000
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Client{
		endpoint:    config.Endpoint,
		apiKey:      config.APIKey,
		model:       config.Model,
		temperature: config.Temperature,
		maxTokens:   config.MaxTokens,
		timeout:     config.Timeout,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Complete sends a chat completion request to the LLM
func (c *Client) Complete(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	request := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	log.Debug().
		Str("endpoint", c.endpoint).
		Str("model", c.model).
		Int("message_count", len(messages)).
		Msg("Sending LLM request")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, classifyHTTPError(resp.StatusCode, string(body))
		}
		return nil, classifyHTTPError(resp.StatusCode, errResp.Error.Message)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debug().
		Str("model", chatResp.Model).
		Int("prompt_tokens", chatResp.Usage.PromptTokens).
		Int("completion_tokens", chatResp.Usage.CompletionTokens).
		Dur("duration", duration).
		Msg("LLM request completed")

	return &chatResp, nil
}

// CompleteWithSystem sends a request with a system message and user message
func (c *Client) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := c.Complete(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}

	return resp.Choices[0].Message.Content, nil
}

// CompleteWithRetry sends a request with retry logic
func (c *Client) CompleteWithRetry(ctx context.Context, messages []ChatMessage, maxRetries int) (*ChatResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Warn().
				Err(lastErr).
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("Retrying LLM request")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.Complete(ctx, messages)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if llmErr, ok := err.(*LLMError); ok && !llmErr.IsRetryable() {
			// Non-retryable error, fail immediately
			return nil, fmt.Errorf("LLM request failed with non-retryable error: %w", err)
		}
	}

	return nil, fmt.Errorf("LLM request failed after %d attempts: %w", maxRetries, lastErr)
}

// ParseJSONResponse parses a JSON response from the LLM
func (c *Client) ParseJSONResponse(content string, target interface{}) error {
	// Try multiple extraction methods in order of preference
	jsonCandidates := []string{
		extractJSONFromMarkdown(content), // Try markdown extraction first
		extractFirstJSONObject(content),  // Try finding first JSON object
		strings.TrimSpace(content),       // Try raw content
	}

	var lastErr error
	for _, candidate := range jsonCandidates {
		if candidate == "" {
			continue
		}
		if err := json.Unmarshal([]byte(candidate), target); err == nil {
			return nil // Success
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed to parse JSON response after multiple attempts: %w", lastErr)
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(content string) string {
	contentBytes := []byte(content)

	// Try all possible markdown code block formats
	patterns := []struct {
		prefix []byte
		offset int
	}{
		{[]byte("```json\n"), 8},
		{[]byte("```json"), 7},
		{[]byte("```\n"), 4},
		{[]byte("```"), 3},
	}

	for _, pattern := range patterns {
		if idx := bytes.Index(contentBytes, pattern.prefix); idx >= 0 {
			start := idx + pattern.offset

			// Find closing ```
			if endIdx := bytes.Index(contentBytes[start:], []byte("```")); endIdx >= 0 {
				end := start + endIdx
				extracted := string(bytes.TrimSpace(contentBytes[start:end]))

				// Validate it looks like JSON before returning
				if len(extracted) > 0 && (extracted[0] == '{' || extracted[0] == '[') {
					return extracted
				}
			}
		}
	}

	return ""
}

// extractFirstJSONObject finds the first complete JSON object or array in the content
func extractFirstJSONObject(content string) string {
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return ""
	}

	// Find first { or [
	startIdx := -1
	isArray := false
	for i, ch := range content {
		if ch == '{' {
			startIdx = i
			break
		} else if ch == '[' {
			startIdx = i
			isArray = true
			break
		}
	}

	if startIdx == -1 {
		return ""
	}

	// Find matching closing bracket
	depth := 0
	openChar := '{'
	closeChar := '}'
	if isArray {
		openChar = '['
		closeChar = ']'
	}

	for i := startIdx; i < len(content); i++ {
		ch := rune(content[i])
		if ch == openChar {
			depth++
		} else if ch == closeChar {
			depth--
			if depth == 0 {
				return content[startIdx : i+1]
			}
		}
	}

	return ""
}

// LLMError represents different types of LLM API errors with retry semantics
type LLMError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("LLM API error (status %d): %s", e.StatusCode, e.Message)
}

// IsRetryable returns whether the error should be retried
func (e *LLMError) IsRetryable() bool {
	return e.Retryable
}

// classifyHTTPError classifies HTTP errors and determines retry strategy
func classifyHTTPError(statusCode int, message string) error {
	retryable := false

	switch {
	case statusCode == http.StatusTooManyRequests: // 429
		retryable = true // Rate limiting - should retry with backoff
	case statusCode >= 500 && statusCode < 600: // 5xx
		retryable = true // Server errors - should retry
	case statusCode == http.StatusBadGateway: // 502
		retryable = true // Bad gateway - should retry
	case statusCode == http.StatusServiceUnavailable: // 503
		retryable = true // Service unavailable - should retry
	case statusCode == http.StatusGatewayTimeout: // 504
		retryable = true // Gateway timeout - should retry
	case statusCode == http.StatusBadRequest: // 400
		retryable = false // Bad request - don't retry
	case statusCode == http.StatusUnauthorized: // 401
		retryable = false // Unauthorized - don't retry
	case statusCode == http.StatusForbidden: // 403
		retryable = false // Forbidden - don't retry
	case statusCode == http.StatusNotFound: // 404
		retryable = false // Not found - don't retry
	default:
		retryable = false // Unknown - don't retry by default
	}

	return &LLMError{
		StatusCode: statusCode,
		Message:    message,
		Retryable:  retryable,
	}
}
