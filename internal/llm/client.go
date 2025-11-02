package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
			return nil, fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("LLM API error: %s", errResp.Error.Message)
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
	}

	return nil, fmt.Errorf("LLM request failed after %d attempts: %w", maxRetries, lastErr)
}

// ParseJSONResponse parses a JSON response from the LLM
func (c *Client) ParseJSONResponse(content string, target interface{}) error {
	// Try to extract JSON from markdown code blocks if present
	content = extractJSONFromMarkdown(content)

	if err := json.Unmarshal([]byte(content), target); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(content string) string {
	// Look for ```json ... ``` or ``` ... ```
	start := -1
	end := -1

	// Find ```json or ```
	contentBytes := []byte(content)
	if idx := bytes.Index(contentBytes, []byte("```json")); idx >= 0 {
		start = idx + 7
	} else if idx := bytes.Index(contentBytes, []byte("```")); idx >= 0 {
		start = idx + 3
	}

	if start >= 0 {
		// Find closing ```
		if idx := bytes.Index(contentBytes[start:], []byte("```")); idx >= 0 {
			end = start + idx
			content = content[start:end]
		}
	}

	return string(bytes.TrimSpace([]byte(content)))
}
