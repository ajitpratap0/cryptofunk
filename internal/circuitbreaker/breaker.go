// Package circuitbreaker provides a simple circuit breaker wrapper around sony/gobreaker.
// It offers a lightweight Wrap helper for protecting external service calls with
// preconfigured settings (5 consecutive failures to trip, 60s recovery window).
package circuitbreaker

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"
)

const (
	// DefaultMaxFailures is the number of consecutive failures before the breaker trips.
	DefaultMaxFailures = 5
	// DefaultTimeout is how long the breaker stays open before moving to half-open.
	DefaultTimeout = 60 * time.Second
	// DefaultHalfOpenMaxReqs is the number of requests allowed in half-open state.
	DefaultHalfOpenMaxReqs = 1
)

var (
	mu       sync.Mutex
	breakers = make(map[string]*gobreaker.CircuitBreaker)
)

// Get returns (or lazily creates) a named circuit breaker with default settings.
func Get(name string) *gobreaker.CircuitBreaker {
	mu.Lock()
	defer mu.Unlock()

	if cb, ok := breakers[name]; ok {
		return cb
	}

	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: DefaultHalfOpenMaxReqs,
		Timeout:     DefaultTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= DefaultMaxFailures
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Warn().
				Str("breaker", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("circuit breaker state change")
		},
	})

	breakers[name] = cb
	return cb
}

// Wrap executes fn inside the named circuit breaker.
// If the breaker is open, it returns immediately with an error.
// Failures from fn count toward tripping the breaker.
func Wrap(name string, fn func() error) error {
	cb := Get(name)
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, fn()
	})
	if err != nil {
		return fmt.Errorf("circuit breaker %s: %w", name, err)
	}
	return nil
}

// WrapWithResult executes fn inside the named circuit breaker and returns its result.
func WrapWithResult[T any](name string, fn func() (T, error)) (T, error) {
	cb := Get(name)
	result, err := cb.Execute(func() (interface{}, error) {
		return fn()
	})
	if err != nil {
		var zero T
		return zero, fmt.Errorf("circuit breaker %s: %w", name, err)
	}
	return result.(T), nil
}
