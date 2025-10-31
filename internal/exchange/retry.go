package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// RetryConfig configures retry behavior for order operations
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	BackoffFactor  float64       // Multiplier for exponential backoff
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err       error
	Retryable bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types that should be retried
	errStr := err.Error()

	// Network errors
	if contains(errStr, "connection refused") ||
		contains(errStr, "connection reset") ||
		contains(errStr, "timeout") ||
		contains(errStr, "temporary failure") ||
		contains(errStr, "too many requests") ||
		contains(errStr, "rate limit") {
		return true
	}

	// Exchange-specific errors
	if contains(errStr, "EAPI:1015") || // Too many requests (Binance)
		contains(errStr, "EAPI:1003") || // Too many requests (Binance)
		contains(errStr, "-1001") || // Internal error (Binance)
		contains(errStr, "-1021") { // Timestamp for this request is outside of the recvWindow
		return true
	}

	return false
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// WithRetry executes an operation with exponential backoff retry
func WithRetry(ctx context.Context, config RetryConfig, operation RetryableOperation) error {
	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		// Execute operation
		err := operation()
		if err == nil {
			if attempt > 0 {
				log.Info().
					Int("attempt", attempt+1).
					Msg("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			log.Debug().
				Err(err).
				Msg("Error is not retryable, aborting")
			return err
		}

		// Don't sleep after last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Log retry attempt
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", config.MaxRetries+1).
			Dur("backoff", backoff).
			Msg("Operation failed, retrying with backoff")

		// Sleep with backoff
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled during backoff: %w", ctx.Err())
		case <-time.After(backoff):
		}

		// Calculate next backoff (exponential)
		backoff = time.Duration(float64(backoff) * config.BackoffFactor)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// WithRetryable wraps an operation to make it retryable
func WithRetryable(ctx context.Context, config RetryConfig, operation RetryableOperation) error {
	return WithRetry(ctx, config, operation)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
