//go:build integration

package risk

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

// TestCircuitBreakerStateTransitions tests the complete state machine:
// Closed -> Open -> HalfOpen -> Closed/Open
func TestCircuitBreakerStateTransitions(t *testing.T) {
	tests := []struct {
		name              string
		serviceType       string
		minRequests       int
		failureRatio      float64
		timeout           time.Duration
		halfOpenMaxReqs   int
		getCircuitBreaker func(*CircuitBreakerManager) *gobreaker.CircuitBreaker
	}{
		{
			name:              "exchange circuit breaker",
			serviceType:       "exchange",
			minRequests:       ExchangeMinRequests,
			failureRatio:      ExchangeFailureRatio,
			timeout:           ExchangeOpenTimeout,
			halfOpenMaxReqs:   ExchangeHalfOpenMaxReqs,
			getCircuitBreaker: func(m *CircuitBreakerManager) *gobreaker.CircuitBreaker { return m.Exchange() },
		},
		{
			name:              "llm circuit breaker",
			serviceType:       "llm",
			minRequests:       LLMMinRequests,
			failureRatio:      LLMFailureRatio,
			timeout:           LLMOpenTimeout,
			halfOpenMaxReqs:   LLMHalfOpenMaxReqs,
			getCircuitBreaker: func(m *CircuitBreakerManager) *gobreaker.CircuitBreaker { return m.LLM() },
		},
		{
			name:              "database circuit breaker",
			serviceType:       "database",
			minRequests:       DBMinRequests,
			failureRatio:      DBFailureRatio,
			timeout:           DBOpenTimeout,
			halfOpenMaxReqs:   DBHalfOpenMaxReqs,
			getCircuitBreaker: func(m *CircuitBreakerManager) *gobreaker.CircuitBreaker { return m.Database() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewCircuitBreakerManager()
			cb := tt.getCircuitBreaker(manager)

			// Phase 1: Closed state - circuit should be closed initially
			assert.Equal(t, gobreaker.StateClosed, cb.State(), "circuit should start in closed state")

			// Phase 2: Trigger enough failures to open the circuit
			// We need at least minRequests with failureRatio% failures
			failureCount := int(float64(tt.minRequests) * tt.failureRatio)
			if failureCount < tt.minRequests {
				failureCount = tt.minRequests
			}

			for i := 0; i < failureCount; i++ {
				cb.Execute(func() (interface{}, error) {
					return nil, errors.New("simulated failure")
				})
			}

			// Circuit should now be open
			assert.Equal(t, gobreaker.StateOpen, cb.State(), "circuit should be open after failures")

			// Phase 3: Verify requests fail fast while open
			_, err := cb.Execute(func() (interface{}, error) {
				t.Fatal("function should not execute while circuit is open")
				return nil, nil
			})
			assert.ErrorIs(t, err, gobreaker.ErrOpenState, "requests should fail with ErrOpenState while circuit is open")

			// Phase 4: Wait for timeout to transition to half-open
			// For integration tests, we use a small timeout to avoid long waits
			// Note: In production, timeouts are longer (15s-60s)
			time.Sleep(tt.timeout + 100*time.Millisecond)

			// The circuit should transition to half-open on the next request
			// First request after timeout should be allowed through
			transitioned := false
			cb.Execute(func() (interface{}, error) {
				transitioned = true
				// Check state during execution
				state := cb.State()
				assert.True(t, state == gobreaker.StateHalfOpen || state == gobreaker.StateClosed,
					"circuit should be in half-open or closed state during execution")
				return "success", nil
			})
			assert.True(t, transitioned, "circuit should allow test request in half-open state")

			// Phase 5: Successful requests in half-open should close the circuit
			for i := 0; i < tt.halfOpenMaxReqs; i++ {
				_, err := cb.Execute(func() (interface{}, error) {
					return "success", nil
				})
				if err != nil {
					t.Logf("Request %d failed: %v (state: %v)", i, err, cb.State())
				}
			}

			// After successful requests, circuit should be closed again
			assert.Equal(t, gobreaker.StateClosed, cb.State(), "circuit should be closed after successful requests in half-open state")
		})
	}
}

// TestCircuitBreakerConcurrentLoad tests circuit breaker behavior under concurrent load
func TestCircuitBreakerConcurrentLoad(t *testing.T) {
	tests := []struct {
		name          string
		numGoroutines int
		requestsPerGo int
		failureRate   float64
		serviceType   string
	}{
		{
			name:          "low concurrency with high success rate",
			numGoroutines: 10,
			requestsPerGo: 100,
			failureRate:   0.1, // 10% failures
			serviceType:   "exchange",
		},
		{
			name:          "high concurrency with high failure rate",
			numGoroutines: 50,
			requestsPerGo: 50,
			failureRate:   0.7, // 70% failures - should trip circuit
			serviceType:   "exchange",
		},
		{
			name:          "moderate concurrency with moderate failures",
			numGoroutines: 20,
			requestsPerGo: 75,
			failureRate:   0.5, // 50% failures - below threshold
			serviceType:   "llm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh manager for each test to avoid state pollution
			manager := NewCircuitBreakerManager()
			var cb *gobreaker.CircuitBreaker

			if tt.serviceType == "llm" {
				cb = manager.LLM()
			} else if tt.serviceType == "database" {
				cb = manager.Database()
			} else {
				cb = manager.Exchange()
			}

			var wg sync.WaitGroup
			var successCount, failureCount, openStateCount atomic.Int64

			// Launch concurrent goroutines making requests
			for i := 0; i < tt.numGoroutines; i++ {
				wg.Add(1)
				go func(goroutineID int) {
					defer wg.Done()

					for j := 0; j < tt.requestsPerGo; j++ {
						shouldFail := (float64(j) / float64(tt.requestsPerGo)) < tt.failureRate

						_, err := cb.Execute(func() (interface{}, error) {
							// Simulate some work
							time.Sleep(time.Millisecond)

							if shouldFail {
								return nil, fmt.Errorf("simulated failure from goroutine %d", goroutineID)
							}
							return fmt.Sprintf("success-%d-%d", goroutineID, j), nil
						})

						if err == nil {
							successCount.Add(1)
						} else if errors.Is(err, gobreaker.ErrOpenState) {
							openStateCount.Add(1)
						} else {
							failureCount.Add(1)
						}
					}
				}(i)
			}

			// Wait for all goroutines to complete
			wg.Wait()

			totalRequests := int64(tt.numGoroutines * tt.requestsPerGo)
			success := successCount.Load()
			failures := failureCount.Load()
			openState := openStateCount.Load()

			t.Logf("Total requests: %d, Success: %d, Failures: %d, Open state rejections: %d",
				totalRequests, success, failures, openState)
			t.Logf("Final circuit state: %v", cb.State())

			// Verify we didn't lose any requests
			assert.Equal(t, totalRequests, success+failures+openState,
				"all requests should be accounted for")

			// Verify behavior based on failure rate
			// Note: Due to concurrent execution, the actual failure rate experienced
			// by the circuit breaker may differ from the expected rate, so we check
			// the final state rather than making strict assertions about open state count
			if tt.failureRate >= ExchangeFailureRatio {
				// High failure rate should trip the circuit
				assert.Greater(t, openState, int64(0),
					"circuit should have opened and rejected some requests")
				t.Logf("Circuit opened as expected with %.0f%% failure rate", tt.failureRate*100)
			} else {
				// Low failure rate may or may not trip depending on timing
				// Just verify we processed all requests
				t.Logf("Circuit state: %v with %.0f%% expected failure rate", cb.State(), tt.failureRate*100)
			}
		})
	}
}

// TestCircuitBreakerRaceConditions tests for race conditions during state transitions
func TestCircuitBreakerRaceConditions(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("concurrent requests during state transitions", func(t *testing.T) {
		// This test runs with -race flag in integration tests
		var wg sync.WaitGroup
		numGoroutines := 100

		// Phase 1: Hammer with failures to open circuit
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				manager.Exchange().Execute(func() (interface{}, error) {
					return nil, errors.New("failure")
				})
			}()
		}
		wg.Wait()

		// Phase 2: Concurrent requests while circuit is open/transitioning
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Mix of successes and failures
				manager.Exchange().Execute(func() (interface{}, error) {
					if id%2 == 0 {
						return "success", nil
					}
					return nil, errors.New("failure")
				})
			}(i)
		}
		wg.Wait()

		// If we reach here without race detector complaints, test passes
		t.Log("No race conditions detected during concurrent state transitions")
	})

	t.Run("concurrent metric updates", func(t *testing.T) {
		var wg sync.WaitGroup
		metrics := manager.Metrics()

		// Hammer metrics recording from multiple goroutines
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					metrics.RecordRequest("exchange", id%2 == 0)
					metrics.RecordRequest("llm", id%3 == 0)
					metrics.RecordRequest("database", id%5 == 0)
				}
			}(i)
		}
		wg.Wait()

		t.Log("No race conditions detected during concurrent metric updates")
	})
}

// TestCircuitBreakerMetricsAccuracy tests that Prometheus metrics are updated correctly
func TestCircuitBreakerMetricsAccuracy(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
		successes   int
		failures    int
	}{
		{
			name:        "mostly successful requests",
			serviceType: "exchange",
			successes:   90,
			failures:    10,
		},
		{
			name:        "mostly failed requests",
			serviceType: "llm",
			successes:   30,
			failures:    70,
		},
		{
			name:        "equal success and failure",
			serviceType: "database",
			successes:   50,
			failures:    50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewCircuitBreakerManager()
			metrics := manager.Metrics()

			// Record requests
			for i := 0; i < tt.successes; i++ {
				metrics.RecordRequest(tt.serviceType, true)
			}
			for i := 0; i < tt.failures; i++ {
				metrics.RecordRequest(tt.serviceType, false)
			}

			// Verify success counter
			successMetric := metrics.requests.WithLabelValues(tt.serviceType, ResultSuccess)
			successCount := testutil.ToFloat64(successMetric)
			assert.GreaterOrEqual(t, successCount, float64(tt.successes),
				"success metric should be at least the number of successes recorded")

			// Verify failure counter
			failureMetric := metrics.requests.WithLabelValues(tt.serviceType, ResultFailure)
			failureCount := testutil.ToFloat64(failureMetric)
			assert.GreaterOrEqual(t, failureCount, float64(tt.failures),
				"failure metric should be at least the number of failures recorded")

			// Verify failure counter matches failures total
			failureTotalMetric := metrics.failures.WithLabelValues(tt.serviceType)
			failureTotalCount := testutil.ToFloat64(failureTotalMetric)
			assert.GreaterOrEqual(t, failureTotalCount, float64(tt.failures),
				"failure total metric should be at least the number of failures recorded")
		})
	}
}

// TestCircuitBreakerMetricsStateTransitions verifies metrics are updated during state changes
func TestCircuitBreakerMetricsStateTransitions(t *testing.T) {
	manager := NewCircuitBreakerManager()
	metrics := manager.Metrics()

	// Get initial state metric value
	stateMetric := metrics.state.WithLabelValues("exchange")
	initialState := testutil.ToFloat64(stateMetric)
	assert.Equal(t, float64(0), initialState, "initial state should be closed (0)")

	// Trigger failures to open circuit
	for i := 0; i < ExchangeMinRequests; i++ {
		manager.Exchange().Execute(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Verify state metric changed to open (1)
	openState := testutil.ToFloat64(stateMetric)
	assert.Equal(t, float64(1), openState, "state should be open (1) after failures")

	// Wait for timeout to transition to half-open
	time.Sleep(ExchangeOpenTimeout + 100*time.Millisecond)

	// Trigger a request to transition to half-open
	manager.Exchange().Execute(func() (interface{}, error) {
		return "success", nil
	})

	// State should now be half-open (2) or closed (0) if it recovered
	halfOpenState := testutil.ToFloat64(stateMetric)
	assert.True(t, halfOpenState == 0 || halfOpenState == 2,
		"state should be half-open (2) or closed (0) after timeout")
}

// TestCircuitBreakerHalfOpenBehavior tests the half-open state specifically
func TestCircuitBreakerHalfOpenBehavior(t *testing.T) {
	t.Run("successful requests in half-open close circuit", func(t *testing.T) {
		manager := NewCircuitBreakerManager()
		cb := manager.Exchange()

		// Open the circuit
		for i := 0; i < ExchangeMinRequests; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		assert.Equal(t, gobreaker.StateOpen, cb.State())

		// Wait for timeout
		time.Sleep(ExchangeOpenTimeout + 100*time.Millisecond)

		// Send successful requests to close circuit
		for i := 0; i < ExchangeHalfOpenMaxReqs; i++ {
			result, err := cb.Execute(func() (interface{}, error) {
				return fmt.Sprintf("success-%d", i), nil
			})
			if err == nil {
				assert.NotNil(t, result)
			}
		}

		// Give circuit time to process and transition
		time.Sleep(100 * time.Millisecond)

		// Circuit should be closed now
		assert.Equal(t, gobreaker.StateClosed, cb.State())
	})

	t.Run("failed requests in half-open reopen circuit", func(t *testing.T) {
		manager := NewCircuitBreakerManager()
		cb := manager.Exchange()

		// Open the circuit
		for i := 0; i < ExchangeMinRequests; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		assert.Equal(t, gobreaker.StateOpen, cb.State())

		// Wait for timeout
		time.Sleep(ExchangeOpenTimeout + 100*time.Millisecond)

		// Send a failed request in half-open state
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("failure in half-open")
		})

		// Circuit should be open again
		assert.Equal(t, gobreaker.StateOpen, cb.State())
	})
}

// TestCircuitBreakerLoadSustained tests sustained load over time
func TestCircuitBreakerLoadSustained(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sustained load test in short mode")
	}

	manager := NewCircuitBreakerManager()
	cb := manager.Exchange()

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	var totalRequests, successCount, failureCount atomic.Int64

	// Start workers generating constant load
	numWorkers := 20
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					totalRequests.Add(1)

					// 30% failure rate - below threshold
					shouldFail := (workerID+int(totalRequests.Load()))%10 < 3

					_, err := cb.Execute(func() (interface{}, error) {
						if shouldFail {
							return nil, errors.New("transient failure")
						}
						return "success", nil
					})

					if err == nil {
						successCount.Add(1)
					} else {
						failureCount.Add(1)
					}
				}
			}
		}(i)
	}

	// Run for 2 seconds
	time.Sleep(2 * time.Second)
	close(stopChan)
	wg.Wait()

	total := totalRequests.Load()
	success := successCount.Load()
	failures := failureCount.Load()

	t.Logf("Sustained load results: Total=%d, Success=%d, Failures=%d, State=%v",
		total, success, failures, cb.State())

	// With 30% failure rate (below 60% threshold), circuit should stay closed
	assert.Equal(t, gobreaker.StateClosed, cb.State(),
		"circuit should remain closed under sustained load with acceptable failure rate")
	assert.Greater(t, total, int64(1000), "should have processed many requests")
}

// TestCircuitBreakerIndependence tests that different service circuits are independent
func TestCircuitBreakerIndependence(t *testing.T) {
	manager := NewCircuitBreakerManager()

	// Open exchange circuit
	for i := 0; i < ExchangeMinRequests; i++ {
		manager.Exchange().Execute(func() (interface{}, error) {
			return nil, errors.New("exchange failure")
		})
	}

	// Open LLM circuit
	for i := 0; i < LLMMinRequests; i++ {
		manager.LLM().Execute(func() (interface{}, error) {
			return nil, errors.New("llm failure")
		})
	}

	// Verify both are open
	assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())
	assert.Equal(t, gobreaker.StateOpen, manager.LLM().State())

	// Database should still be closed
	assert.Equal(t, gobreaker.StateClosed, manager.Database().State())

	// Database requests should still work
	result, err := manager.Database().Execute(func() (interface{}, error) {
		return "database works", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "database works", result)
}

// TestCircuitBreakerRecoveryUnderLoad tests recovery behavior under sustained load
func TestCircuitBreakerRecoveryUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping recovery test in short mode")
	}

	manager := NewCircuitBreakerManager()
	cb := manager.Exchange()

	var wg sync.WaitGroup
	var phase atomic.Int32 // 0=breaking, 1=recovering

	// Phase 1: Break the circuit with high failure rate
	phase.Store(0)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cb.Execute(func() (interface{}, error) {
					return nil, errors.New("failure")
				})
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, gobreaker.StateOpen, cb.State(), "circuit should be open after failures")

	// Phase 2: Wait for timeout and recover with successful requests
	time.Sleep(ExchangeOpenTimeout + 100*time.Millisecond)
	phase.Store(1)

	var recoverySuccesses, recoveryFailures atomic.Int64

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, err := cb.Execute(func() (interface{}, error) {
					return "success", nil
				})
				if err == nil {
					recoverySuccesses.Add(1)
				} else {
					recoveryFailures.Add(1)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	t.Logf("Recovery phase: Successes=%d, Failures=%d, Final state=%v",
		recoverySuccesses.Load(), recoveryFailures.Load(), cb.State())

	// After successful requests, circuit should eventually close
	assert.True(t, cb.State() == gobreaker.StateClosed || cb.State() == gobreaker.StateHalfOpen,
		"circuit should be closed or half-open after recovery attempts")
}

// BenchmarkCircuitBreakerThroughput benchmarks circuit breaker throughput
func BenchmarkCircuitBreakerThroughput(b *testing.B) {
	manager := NewCircuitBreakerManager()
	cb := manager.Exchange()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cb.Execute(func() (interface{}, error) {
				// Simulate very fast operation
				i++
				return i, nil
			})
		}
	})
}

// BenchmarkCircuitBreakerWithMetrics benchmarks circuit breaker with metrics recording
func BenchmarkCircuitBreakerWithMetrics(b *testing.B) {
	manager := NewCircuitBreakerManager()
	cb := manager.Exchange()
	metrics := manager.Metrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, err := cb.Execute(func() (interface{}, error) {
				i++
				return i, nil
			})
			metrics.RecordRequest("exchange", err == nil)
		}
	})
}
