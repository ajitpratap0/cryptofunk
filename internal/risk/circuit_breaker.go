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

	// Metric result labels
	ResultSuccess = "success"
	ResultFailure = "failure"
)

// Circuit breaker thresholds - configurable per service type
const (
	// Exchange circuit breaker settings
	ExchangeMinRequests     = 5                // Minimum requests before tripping
	ExchangeFailureRatio    = 0.6              // Failure ratio threshold (60%)
	ExchangeOpenTimeout     = 30 * time.Second // How long circuit stays open
	ExchangeHalfOpenMaxReqs = 3                // Max requests in half-open state
	ExchangeCountInterval   = 10 * time.Second // Window for counting failures

	// LLM circuit breaker settings (longer timeouts for AI calls)
	LLMMinRequests     = 3                // Minimum requests before tripping
	LLMFailureRatio    = 0.6              // Failure ratio threshold (60%)
	LLMOpenTimeout     = 60 * time.Second // How long circuit stays open (longer for LLM recovery)
	LLMHalfOpenMaxReqs = 2                // Max requests in half-open state
	LLMCountInterval   = 10 * time.Second // Window for counting failures

	// Database circuit breaker settings (faster recovery)
	DBMinRequests     = 10               // Minimum requests before tripping
	DBFailureRatio    = 0.6              // Failure ratio threshold (60%)
	DBOpenTimeout     = 15 * time.Second // How long circuit stays open (quick recovery)
	DBHalfOpenMaxReqs = 5                // Max requests in half-open state
	DBCountInterval   = 10 * time.Second // Window for counting failures
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

// ServiceSettings holds circuit breaker configuration for a single service
type ServiceSettings struct {
	MinRequests     uint32
	FailureRatio    float64
	OpenTimeout     time.Duration
	HalfOpenMaxReqs uint32
	CountInterval   time.Duration
}

// ParseDuration parses a duration string and returns the duration or a default value
func ParseDuration(durationStr string, defaultValue time.Duration) time.Duration {
	if durationStr == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return defaultValue
	}
	return duration
}

// NewCircuitBreakerManager creates a new circuit breaker manager with default settings
// This function maintains backward compatibility for existing code
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return NewCircuitBreakerManagerWithSettings(nil, nil, nil)
}

// NewCircuitBreakerManagerFromConfig creates a circuit breaker manager from config settings
// This is a convenience function that converts config.CircuitBreakerSettings to ServiceSettings
func NewCircuitBreakerManagerFromConfig(exchangeCfg, llmCfg, dbCfg interface{}) *CircuitBreakerManager {
	// Type assertion and conversion for exchange settings
	var exchangeSettings *ServiceSettings
	if cfg, ok := exchangeCfg.(struct {
		MinRequests     uint32
		FailureRatio    float64
		OpenTimeout     string
		HalfOpenMaxReqs uint32
		CountInterval   string
	}); ok {
		exchangeSettings = &ServiceSettings{
			MinRequests:     cfg.MinRequests,
			FailureRatio:    cfg.FailureRatio,
			OpenTimeout:     ParseDuration(cfg.OpenTimeout, ExchangeOpenTimeout),
			HalfOpenMaxReqs: cfg.HalfOpenMaxReqs,
			CountInterval:   ParseDuration(cfg.CountInterval, ExchangeCountInterval),
		}
	}

	// Type assertion and conversion for LLM settings
	var llmSettings *ServiceSettings
	if cfg, ok := llmCfg.(struct {
		MinRequests     uint32
		FailureRatio    float64
		OpenTimeout     string
		HalfOpenMaxReqs uint32
		CountInterval   string
	}); ok {
		llmSettings = &ServiceSettings{
			MinRequests:     cfg.MinRequests,
			FailureRatio:    cfg.FailureRatio,
			OpenTimeout:     ParseDuration(cfg.OpenTimeout, LLMOpenTimeout),
			HalfOpenMaxReqs: cfg.HalfOpenMaxReqs,
			CountInterval:   ParseDuration(cfg.CountInterval, LLMCountInterval),
		}
	}

	// Type assertion and conversion for database settings
	var dbSettings *ServiceSettings
	if cfg, ok := dbCfg.(struct {
		MinRequests     uint32
		FailureRatio    float64
		OpenTimeout     string
		HalfOpenMaxReqs uint32
		CountInterval   string
	}); ok {
		dbSettings = &ServiceSettings{
			MinRequests:     cfg.MinRequests,
			FailureRatio:    cfg.FailureRatio,
			OpenTimeout:     ParseDuration(cfg.OpenTimeout, DBOpenTimeout),
			HalfOpenMaxReqs: cfg.HalfOpenMaxReqs,
			CountInterval:   ParseDuration(cfg.CountInterval, DBCountInterval),
		}
	}

	return NewCircuitBreakerManagerWithSettings(exchangeSettings, llmSettings, dbSettings)
}

// NewCircuitBreakerManagerWithSettings creates a new circuit breaker manager with Prometheus metrics
// If settings are nil, defaults to the constants defined above
func NewCircuitBreakerManagerWithSettings(exchangeSettings, llmSettings, dbSettings *ServiceSettings) *CircuitBreakerManager {
	// Register metrics only once using sync.Once for thread safety
	initMetrics()

	metrics := globalMetrics

	manager := &CircuitBreakerManager{
		metrics: metrics,
	}

	// Use defaults if settings not provided
	if exchangeSettings == nil {
		exchangeSettings = &ServiceSettings{
			MinRequests:     ExchangeMinRequests,
			FailureRatio:    ExchangeFailureRatio,
			OpenTimeout:     ExchangeOpenTimeout,
			HalfOpenMaxReqs: ExchangeHalfOpenMaxReqs,
			CountInterval:   ExchangeCountInterval,
		}
	}
	if llmSettings == nil {
		llmSettings = &ServiceSettings{
			MinRequests:     LLMMinRequests,
			FailureRatio:    LLMFailureRatio,
			OpenTimeout:     LLMOpenTimeout,
			HalfOpenMaxReqs: LLMHalfOpenMaxReqs,
			CountInterval:   LLMCountInterval,
		}
	}
	if dbSettings == nil {
		dbSettings = &ServiceSettings{
			MinRequests:     DBMinRequests,
			FailureRatio:    DBFailureRatio,
			OpenTimeout:     DBOpenTimeout,
			HalfOpenMaxReqs: DBHalfOpenMaxReqs,
			CountInterval:   DBCountInterval,
		}
	}

	// Exchange circuit breaker: configurable thresholds
	manager.exchange = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "exchange",
		MaxRequests: exchangeSettings.HalfOpenMaxReqs,
		Interval:    exchangeSettings.CountInterval,
		Timeout:     exchangeSettings.OpenTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= exchangeSettings.MinRequests && failureRatio >= exchangeSettings.FailureRatio
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			manager.updateMetrics("exchange", to)
		},
	})

	// LLM circuit breaker: longer timeouts for AI model calls
	manager.llm = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "llm",
		MaxRequests: llmSettings.HalfOpenMaxReqs,
		Interval:    llmSettings.CountInterval,
		Timeout:     llmSettings.OpenTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= llmSettings.MinRequests && failureRatio >= llmSettings.FailureRatio
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			manager.updateMetrics("llm", to)
		},
	})

	// Database circuit breaker: quick recovery for DB connections
	manager.database = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "database",
		MaxRequests: dbSettings.HalfOpenMaxReqs,
		Interval:    dbSettings.CountInterval,
		Timeout:     dbSettings.OpenTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= dbSettings.MinRequests && failureRatio >= dbSettings.FailureRatio
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
	result := ResultSuccess
	if !success {
		result = ResultFailure
		m.failures.WithLabelValues(service).Inc()
	}
	m.requests.WithLabelValues(service, result).Inc()
}

// Metrics returns the metrics instance for manual recording
func (m *CircuitBreakerManager) Metrics() *CircuitBreakerMetrics {
	return m.metrics
}
