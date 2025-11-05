package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// FallbackClient provides automatic failover between multiple LLM models
type FallbackClient struct {
	clients        []*Client
	modelNames     []string
	circuitBreaker *CircuitBreaker
}

// FallbackConfig configures the fallback client
type FallbackConfig struct {
	// Primary model configuration
	PrimaryConfig ClientConfig
	PrimaryName   string

	// Fallback model configurations (in order of preference)
	FallbackConfigs []ClientConfig
	FallbackNames   []string

	// Circuit breaker configuration
	CircuitBreakerConfig CircuitBreakerConfig
}

// CircuitBreakerConfig configures the circuit breaker
type CircuitBreakerConfig struct {
	// Threshold for opening circuit (number of consecutive failures)
	FailureThreshold int

	// Threshold for closing circuit (number of consecutive successes)
	SuccessThreshold int

	// Timeout before attempting to close circuit
	Timeout time.Duration

	// Time window for counting failures
	TimeWindow time.Duration
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		TimeWindow:       5 * time.Minute,
	}
}

// NewFallbackClient creates a client with automatic model fallback
func NewFallbackClient(config FallbackConfig) *FallbackClient {
	// Create primary client
	clients := []*Client{NewClient(config.PrimaryConfig)}
	modelNames := []string{config.PrimaryName}

	// Create fallback clients
	for i, fbConfig := range config.FallbackConfigs {
		clients = append(clients, NewClient(fbConfig))
		if i < len(config.FallbackNames) {
			modelNames = append(modelNames, config.FallbackNames[i])
		} else {
			modelNames = append(modelNames, fmt.Sprintf("fallback-%d", i+1))
		}
	}

	// Initialize circuit breaker
	cbConfig := config.CircuitBreakerConfig
	if cbConfig.FailureThreshold == 0 {
		cbConfig = DefaultCircuitBreakerConfig()
	}

	return &FallbackClient{
		clients:        clients,
		modelNames:     modelNames,
		circuitBreaker: NewCircuitBreaker(len(clients), cbConfig),
	}
}

// Complete attempts to get a completion, falling back to other models on failure
func (fc *FallbackClient) Complete(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	var lastErr error

	for i, client := range fc.clients {
		modelName := fc.modelNames[i]

		// Check if circuit is open for this model
		if fc.circuitBreaker.IsOpen(i) {
			log.Warn().
				Str("model", modelName).
				Msg("Circuit breaker open, skipping model")
			continue
		}

		log.Debug().
			Str("model", modelName).
			Int("attempt", i+1).
			Int("total_models", len(fc.clients)).
			Msg("Attempting LLM completion")

		start := time.Now()
		resp, err := client.Complete(ctx, messages)
		duration := time.Since(start)

		if err == nil {
			// Success - record and return
			fc.circuitBreaker.RecordSuccess(i)

			log.Info().
				Str("model", modelName).
				Int("attempt", i+1).
				Dur("duration", duration).
				Msg("LLM completion succeeded")

			return resp, nil
		}

		// Record failure
		fc.circuitBreaker.RecordFailure(i)
		lastErr = err

		log.Warn().
			Err(err).
			Str("model", modelName).
			Int("attempt", i+1).
			Dur("duration", duration).
			Msg("LLM completion failed, trying fallback")

		// Check if error is retryable - if not, try next model immediately
		if llmErr, ok := err.(*LLMError); ok && !llmErr.IsRetryable() {
			log.Debug().
				Str("model", modelName).
				Msg("Non-retryable error, skipping to next model")
			continue
		}
	}

	// All models failed
	return nil, fmt.Errorf("all models failed, last error: %w", lastErr)
}

// CompleteWithRetry attempts completion with retries on each model before fallback
func (fc *FallbackClient) CompleteWithRetry(ctx context.Context, messages []ChatMessage, maxRetries int) (*ChatResponse, error) {
	var lastErr error

	for i, client := range fc.clients {
		modelName := fc.modelNames[i]

		// Check if circuit is open for this model
		if fc.circuitBreaker.IsOpen(i) {
			log.Warn().
				Str("model", modelName).
				Msg("Circuit breaker open, skipping model")
			continue
		}

		log.Debug().
			Str("model", modelName).
			Int("model_index", i+1).
			Int("total_models", len(fc.clients)).
			Int("max_retries", maxRetries).
			Msg("Attempting LLM completion with retries")

		start := time.Now()
		resp, err := client.CompleteWithRetry(ctx, messages, maxRetries)
		duration := time.Since(start)

		if err == nil {
			// Success - record and return
			fc.circuitBreaker.RecordSuccess(i)

			log.Info().
				Str("model", modelName).
				Int("model_index", i+1).
				Dur("duration", duration).
				Msg("LLM completion with retry succeeded")

			return resp, nil
		}

		// Record failure
		fc.circuitBreaker.RecordFailure(i)
		lastErr = err

		log.Warn().
			Err(err).
			Str("model", modelName).
			Int("model_index", i+1).
			Dur("duration", duration).
			Msg("LLM completion with retry failed, trying fallback")
	}

	// All models failed
	return nil, fmt.Errorf("all models failed after retries, last error: %w", lastErr)
}

// CompleteWithSystem is a convenience method for system + user prompts with fallback
func (fc *FallbackClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := fc.Complete(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}

	return resp.Choices[0].Message.Content, nil
}

// ParseJSONResponse parses a JSON response from the LLM
// Delegates to the primary client's JSON parsing logic
func (fc *FallbackClient) ParseJSONResponse(content string, target interface{}) error {
	if len(fc.clients) == 0 {
		return fmt.Errorf("no clients available for JSON parsing")
	}
	// Use primary client's JSON parsing logic
	return fc.clients[0].ParseJSONResponse(content, target)
}

// GetCircuitBreakerStatus returns the status of all model circuit breakers
func (fc *FallbackClient) GetCircuitBreakerStatus() []CircuitBreakerStatus {
	return fc.circuitBreaker.GetAllStatus()
}

// ResetCircuitBreaker resets the circuit breaker for a specific model
func (fc *FallbackClient) ResetCircuitBreaker(modelIndex int) error {
	if modelIndex < 0 || modelIndex >= len(fc.clients) {
		return fmt.Errorf("invalid model index: %d", modelIndex)
	}
	fc.circuitBreaker.Reset(modelIndex)
	log.Info().
		Str("model", fc.modelNames[modelIndex]).
		Int("model_index", modelIndex).
		Msg("Circuit breaker reset")
	return nil
}

// CircuitBreaker implements the circuit breaker pattern for multiple models
type CircuitBreaker struct {
	models []modelCircuit
	config CircuitBreakerConfig
	mu     sync.RWMutex
}

type modelCircuit struct {
	state                CircuitState
	consecutiveFails     int
	consecutiveSuccesses int
	lastFailure          time.Time
	lastSuccess          time.Time
	openedAt             time.Time
	failures             []time.Time // Sliding window of failures
}

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	CircuitClosed   CircuitState = "CLOSED"    // Normal operation
	CircuitOpen     CircuitState = "OPEN"      // Blocking requests
	CircuitHalfOpen CircuitState = "HALF_OPEN" // Testing recovery
)

// CircuitBreakerStatus represents the status of a single model's circuit
type CircuitBreakerStatus struct {
	ModelIndex           int
	State                CircuitState
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastFailure          time.Time
	LastSuccess          time.Time
	OpenedAt             time.Time
	RecentFailureCount   int // Failures in time window
}

// NewCircuitBreaker creates a new circuit breaker for N models
func NewCircuitBreaker(numModels int, config CircuitBreakerConfig) *CircuitBreaker {
	models := make([]modelCircuit, numModels)
	for i := range models {
		models[i] = modelCircuit{
			state:    CircuitClosed,
			failures: make([]time.Time, 0),
		}
	}

	return &CircuitBreaker{
		models: models,
		config: config,
	}
}

// IsOpen checks if the circuit is open for a given model
func (cb *CircuitBreaker) IsOpen(modelIndex int) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if modelIndex < 0 || modelIndex >= len(cb.models) {
		return true // Safe default
	}

	circuit := &cb.models[modelIndex]

	switch circuit.state {
	case CircuitOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(circuit.openedAt) >= cb.config.Timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			circuit.state = CircuitHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return false
		}
		return true
	case CircuitHalfOpen:
		return false // Allow one request through
	case CircuitClosed:
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess(modelIndex int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if modelIndex < 0 || modelIndex >= len(cb.models) {
		return
	}

	circuit := &cb.models[modelIndex]
	circuit.consecutiveSuccesses++
	circuit.consecutiveFails = 0
	circuit.lastSuccess = time.Now()

	// Transition from half-open to closed after success threshold
	if circuit.state == CircuitHalfOpen && circuit.consecutiveSuccesses >= cb.config.SuccessThreshold {
		circuit.state = CircuitClosed
		circuit.consecutiveSuccesses = 0
		log.Info().
			Int("model_index", modelIndex).
			Msg("Circuit breaker closed after successful recovery")
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure(modelIndex int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if modelIndex < 0 || modelIndex >= len(cb.models) {
		return
	}

	circuit := &cb.models[modelIndex]
	now := time.Now()

	circuit.consecutiveFails++
	circuit.consecutiveSuccesses = 0
	circuit.lastFailure = now
	circuit.failures = append(circuit.failures, now)

	// Clean up old failures outside time window
	cutoff := now.Add(-cb.config.TimeWindow)
	validFailures := make([]time.Time, 0)
	for _, failTime := range circuit.failures {
		if failTime.After(cutoff) {
			validFailures = append(validFailures, failTime)
		}
	}
	circuit.failures = validFailures

	// Check if we should open the circuit
	switch circuit.state {
	case CircuitClosed:
		if circuit.consecutiveFails >= cb.config.FailureThreshold {
			circuit.state = CircuitOpen
			circuit.openedAt = now
			log.Warn().
				Int("model_index", modelIndex).
				Int("consecutive_failures", circuit.consecutiveFails).
				Msg("Circuit breaker opened due to failures")
		}
	case CircuitHalfOpen:
		// Failure in half-open state - go back to open
		circuit.state = CircuitOpen
		circuit.openedAt = now
		log.Warn().
			Int("model_index", modelIndex).
			Msg("Circuit breaker re-opened after failed recovery attempt")
	}
}

// Reset resets the circuit breaker for a specific model
func (cb *CircuitBreaker) Reset(modelIndex int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if modelIndex < 0 || modelIndex >= len(cb.models) {
		return
	}

	circuit := &cb.models[modelIndex]
	circuit.state = CircuitClosed
	circuit.consecutiveFails = 0
	circuit.consecutiveSuccesses = 0
	circuit.failures = make([]time.Time, 0)
}

// GetAllStatus returns the status of all circuits
func (cb *CircuitBreaker) GetAllStatus() []CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	statuses := make([]CircuitBreakerStatus, len(cb.models))
	for i, circuit := range cb.models {
		statuses[i] = CircuitBreakerStatus{
			ModelIndex:           i,
			State:                circuit.state,
			ConsecutiveFailures:  circuit.consecutiveFails,
			ConsecutiveSuccesses: circuit.consecutiveSuccesses,
			LastFailure:          circuit.lastFailure,
			LastSuccess:          circuit.lastSuccess,
			OpenedAt:             circuit.openedAt,
			RecentFailureCount:   len(circuit.failures),
		}
	}

	return statuses
}
