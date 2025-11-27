package risk

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sony/gobreaker"
)

// Circuit breaker states for Prometheus metrics
const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half_open"
)

// CircuitBreakerManager manages circuit breakers for different service types
type CircuitBreakerManager struct {
	exchange *gobreaker.CircuitBreaker
	llm      *gobreaker.CircuitBreaker
	database *gobreaker.CircuitBreaker
	metrics  *CircuitBreakerMetrics
}

// CircuitBreakerMetrics holds Prometheus metrics for circuit breakers
type CircuitBreakerMetrics struct {
	state    *prometheus.GaugeVec
	requests *prometheus.CounterVec
	failures *prometheus.CounterVec
}

var (
	// Global metrics instance (singleton)
	globalMetrics *CircuitBreakerMetrics
	metricsOnce   sync.Once
)

// initMetrics initializes the global metrics instance exactly once in a thread-safe manner
func initMetrics() {
	metricsOnce.Do(func() {
		globalMetrics = &CircuitBreakerMetrics{
			state: promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "circuit_breaker_state",
					Help: "Circuit breaker state (0=closed, 1=open, 2=half_open)",
				},
				[]string{"service"},
			),
			requests: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "circuit_breaker_requests_total",
					Help: "Total number of requests through circuit breaker",
				},
				[]string{"service", "result"},
			),
			failures: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "circuit_breaker_failures_total",
					Help: "Total number of failures tracked by circuit breaker",
				},
				[]string{"service"},
			),
		}
	})
}

// NewCircuitBreakerManager creates a new circuit breaker manager with Prometheus metrics
func NewCircuitBreakerManager() *CircuitBreakerManager {
	// Register metrics only once using sync.Once for thread safety
	initMetrics()

	metrics := globalMetrics

	manager := &CircuitBreakerManager{
		metrics: metrics,
	}

	// Exchange circuit breaker: 5 failures → open for 30 seconds
	manager.exchange = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "exchange",
		MaxRequests: 3,                // Max requests in half-open state
		Interval:    10 * time.Second, // Window for counting failures
		Timeout:     30 * time.Second, // How long to stay open before half-open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			manager.updateMetrics("exchange", to)
		},
	})

	// LLM circuit breaker: 3 failures → open for 60 seconds
	manager.llm = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "llm",
		MaxRequests: 2,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second, // Longer timeout for LLM
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			manager.updateMetrics("llm", to)
		},
	})

	// Database circuit breaker: 10 failures → open for 15 seconds
	manager.database = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "database",
		MaxRequests: 5,
		Interval:    10 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			manager.updateMetrics("database", to)
		},
	})

	// Initialize metrics
	manager.updateMetrics("exchange", manager.exchange.State())
	manager.updateMetrics("llm", manager.llm.State())
	manager.updateMetrics("database", manager.database.State())

	return manager
}

// NewPassthroughCircuitBreakerManager creates a circuit breaker manager that never trips.
// This is useful for testing scenarios where you want to test other components without
// the circuit breaker interfering.
func NewPassthroughCircuitBreakerManager() *CircuitBreakerManager {
	// Register metrics only once using sync.Once for thread safety
	initMetrics()

	metrics := globalMetrics

	manager := &CircuitBreakerManager{
		metrics: metrics,
	}

	// Passthrough circuit breaker - never trips
	neverTrip := func(counts gobreaker.Counts) bool {
		return false // Never trip
	}

	manager.exchange = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "exchange_passthrough",
		MaxRequests: 1000,
		Interval:    0,
		Timeout:     1 * time.Millisecond,
		ReadyToTrip: neverTrip,
	})

	manager.llm = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "llm_passthrough",
		MaxRequests: 1000,
		Interval:    0,
		Timeout:     1 * time.Millisecond,
		ReadyToTrip: neverTrip,
	})

	manager.database = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "database_passthrough",
		MaxRequests: 1000,
		Interval:    0,
		Timeout:     1 * time.Millisecond,
		ReadyToTrip: neverTrip,
	})

	return manager
}

// Exchange returns the exchange circuit breaker
func (m *CircuitBreakerManager) Exchange() *gobreaker.CircuitBreaker {
	return m.exchange
}

// LLM returns the LLM circuit breaker
func (m *CircuitBreakerManager) LLM() *gobreaker.CircuitBreaker {
	return m.llm
}

// Database returns the database circuit breaker
func (m *CircuitBreakerManager) Database() *gobreaker.CircuitBreaker {
	return m.database
}

// updateMetrics updates Prometheus metrics for a circuit breaker state change
func (m *CircuitBreakerManager) updateMetrics(service string, state gobreaker.State) {
	var stateValue float64
	switch state {
	case gobreaker.StateClosed:
		stateValue = 0
	case gobreaker.StateOpen:
		stateValue = 1
	case gobreaker.StateHalfOpen:
		stateValue = 2
	}
	m.metrics.state.WithLabelValues(service).Set(stateValue)
}

// RecordRequest records a request result for metrics
func (m *CircuitBreakerMetrics) RecordRequest(service string, success bool) {
	result := "success"
	if !success {
		result = "failure"
		m.failures.WithLabelValues(service).Inc()
	}
	m.requests.WithLabelValues(service, result).Inc()
}

// Metrics returns the metrics instance for manual recording
func (m *CircuitBreakerManager) Metrics() *CircuitBreakerMetrics {
	return m.metrics
}
