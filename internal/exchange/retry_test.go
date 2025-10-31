package exchange

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetryConfig tests retry configuration
func TestRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialBackoff)
	assert.Equal(t, 5*time.Second, config.MaxBackoff)
	assert.Equal(t, 2.0, config.BackoffFactor)
}

// TestIsRetryable tests error categorization
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "connection refused error",
			err:       fmt.Errorf("connection refused"),
			retryable: true,
		},
		{
			name:      "connection reset error",
			err:       fmt.Errorf("connection reset by peer"),
			retryable: true,
		},
		{
			name:      "timeout error",
			err:       fmt.Errorf("request timeout exceeded"),
			retryable: true,
		},
		{
			name:      "rate limit error",
			err:       fmt.Errorf("rate limit exceeded - too many requests"),
			retryable: true,
		},
		{
			name:      "binance rate limit error",
			err:       fmt.Errorf("EAPI:1015 - Too many requests"),
			retryable: true,
		},
		{
			name:      "binance internal error",
			err:       fmt.Errorf("API error -1001: Internal server error"),
			retryable: true,
		},
		{
			name:      "validation error",
			err:       fmt.Errorf("invalid parameter: quantity must be positive"),
			retryable: false,
		},
		{
			name:      "generic error",
			err:       fmt.Errorf("some other error"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			assert.Equal(t, tt.retryable, result, "Error retryability mismatch")
		})
	}
}

// TestWithRetry_Success tests successful operation without retries
func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := WithRetry(ctx, config, operation)
	require.NoError(t, err)
	assert.Equal(t, 1, attempts, "Should succeed on first attempt")
}

// TestWithRetry_RetryableErrorEventualSuccess tests retry with eventual success
func TestWithRetry_RetryableErrorEventualSuccess(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("connection timeout")
		}
		return nil
	}

	startTime := time.Now()
	err := WithRetry(ctx, config, operation)
	duration := time.Since(startTime)

	require.NoError(t, err)
	assert.Equal(t, 3, attempts, "Should succeed on third attempt")
	// Should have backoff delays: 10ms + 20ms = 30ms minimum
	assert.Greater(t, duration, 30*time.Millisecond, "Should have backoff delays")
}

// TestWithRetry_NonRetryableError tests immediate failure on non-retryable error
func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	attempts := 0
	expectedErr := fmt.Errorf("invalid parameter")
	operation := func() error {
		attempts++
		return expectedErr
	}

	err := WithRetry(ctx, config, operation)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err, "Should return the same error")
	assert.Equal(t, 1, attempts, "Should not retry non-retryable errors")
}

// TestWithRetry_MaxRetriesExceeded tests failure after max retries
func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	operation := func() error {
		attempts++
		return fmt.Errorf("connection refused")
	}

	err := WithRetry(ctx, config, operation)
	require.Error(t, err)
	assert.Equal(t, 3, attempts, "Should attempt 3 times (initial + 2 retries)")
	assert.Contains(t, err.Error(), "operation failed after 3 attempts")
}

// TestWithRetry_ContextCancellation tests cancellation during retry
func TestWithRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts == 2 {
			// Cancel context on second attempt
			cancel()
		}
		return fmt.Errorf("rate limit exceeded")
	}

	err := WithRetry(ctx, config, operation)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation cancelled")
	assert.LessOrEqual(t, attempts, 3, "Should stop retrying after context cancellation")
}

// TestWithRetry_ExponentialBackoff tests backoff duration increases
func TestWithRetry_ExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	operation := func() error {
		attempts++
		return fmt.Errorf("timeout")
	}

	startTime := time.Now()
	err := WithRetry(ctx, config, operation)
	duration := time.Since(startTime)

	require.Error(t, err)
	assert.Equal(t, 4, attempts, "Should attempt 4 times")
	// Expected backoff: 10ms + 20ms + 40ms = 70ms minimum
	assert.Greater(t, duration, 70*time.Millisecond, "Should have exponential backoff")
}

// TestWithRetry_MaxBackoffLimit tests backoff cap
func TestWithRetry_MaxBackoffLimit(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     150 * time.Millisecond, // Cap at 150ms
		BackoffFactor:  3.0,                    // Fast growth
	}

	attempts := 0
	operation := func() error {
		attempts++
		return fmt.Errorf("connection reset")
	}

	startTime := time.Now()
	err := WithRetry(ctx, config, operation)
	duration := time.Since(startTime)

	require.Error(t, err)
	assert.Equal(t, 6, attempts, "Should attempt 6 times")
	// With capping: 100ms + 150ms + 150ms + 150ms + 150ms = 700ms minimum
	// Without capping: 100ms + 300ms + 900ms + 2700ms + 8100ms >> 700ms
	assert.Greater(t, duration, 700*time.Millisecond, "Should respect max backoff")
	assert.Less(t, duration, 1500*time.Millisecond, "Should not exceed max backoff significantly")
}

// BenchmarkWithRetry_NoRetries benchmarks successful operation
func BenchmarkWithRetry_NoRetries(b *testing.B) {
	ctx := context.Background()
	config := DefaultRetryConfig()

	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithRetry(ctx, config, operation)
	}
}

// BenchmarkWithRetry_WithRetries benchmarks operation with retries
func BenchmarkWithRetry_WithRetries(b *testing.B) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 2 {
				return fmt.Errorf("timeout")
			}
			return nil
		}
		_ = WithRetry(ctx, config, operation)
	}
}
