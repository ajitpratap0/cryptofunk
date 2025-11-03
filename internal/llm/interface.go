package llm

import "context"

// LLMClient defines the interface for LLM clients (both basic and fallback)
// This allows agents to use either Client or FallbackClient transparently
type LLMClient interface {
	// Complete sends a chat completion request with the given messages
	Complete(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)

	// CompleteWithRetry attempts completion with retries on transient failures
	CompleteWithRetry(ctx context.Context, messages []ChatMessage, maxRetries int) (*ChatResponse, error)

	// CompleteWithSystem is a convenience method for system + user prompts
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// ParseJSONResponse extracts and parses JSON from LLM response content
	ParseJSONResponse(content string, target interface{}) error
}

// Ensure Client implements LLMClient interface
var _ LLMClient = (*Client)(nil)

// Ensure FallbackClient implements LLMClient interface
var _ LLMClient = (*FallbackClient)(nil)
