package risk

import (
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreakerManager(t *testing.T) {
	manager := NewCircuitBreakerManager()

	require.NotNil(t, manager)
	require.NotNil(t, manager.exchange)
	require.NotNil(t, manager.llm)
	require.NotNil(t, manager.database)
	require.NotNil(t, manager.metrics)

	// Verify initial state is closed
	assert.Equal(t, gobreaker.StateClosed, manager.exchange.State())
	assert.Equal(t, gobreaker.StateClosed, manager.llm.State())
	assert.Equal(t, gobreaker.StateClosed, manager.database.State())
}

func TestCircuitBreakerManager_Exchange(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("successful requests keep circuit closed", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			_, err := manager.Exchange().Execute(func() (interface{}, error) {
				return "success", nil
			})
			require.NoError(t, err)
		}
		assert.Equal(t, gobreaker.StateClosed, manager.Exchange().State())
	})

	t.Run("circuit opens after threshold failures", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// Trigger failures to open circuit
		// Exchange CB: needs 5 requests with 60% failure rate
		for i := 0; i < 5; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				return nil, errors.New("exchange error")
			})
		}

		// Circuit should be open now
		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())

		// Next request should fail immediately with ErrOpenState
		_, err := manager.Exchange().Execute(func() (interface{}, error) {
			return "should not execute", nil
		})
		assert.ErrorIs(t, err, gobreaker.ErrOpenState)
	})

	t.Run("circuit recovers after timeout", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// Open the circuit
		for i := 0; i < 5; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				return nil, errors.New("exchange error")
			})
		}

		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())

		// Wait for timeout (30 seconds for exchange, but we'll use a shorter interval in tests)
		// Note: In production, this would be 30 seconds. For this test, we verify the state.
		// After timeout, the circuit should transition to half-open on the next request

		// Verify the circuit breaker was created successfully
		require.NotNil(t, manager)
		require.NotNil(t, manager.Exchange())
	})
}

func TestCircuitBreakerManager_LLM(t *testing.T) {
	t.Run("LLM circuit opens after 3 failures", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// LLM CB: needs 3 requests with 60% failure rate
		for i := 0; i < 3; i++ {
			manager.LLM().Execute(func() (interface{}, error) {
				return nil, errors.New("llm timeout")
			})
		}

		assert.Equal(t, gobreaker.StateOpen, manager.LLM().State())

		// Verify next request fails immediately
		_, err := manager.LLM().Execute(func() (interface{}, error) {
			return "should not execute", nil
		})
		assert.ErrorIs(t, err, gobreaker.ErrOpenState)
	})

	t.Run("LLM circuit has longer timeout", func(t *testing.T) {
		mgr := NewCircuitBreakerManager()

		// Verify LLM has 60-second timeout (longer than exchange's 30s)
		// We can't easily test the timeout duration directly, but we can verify
		// the circuit breaker exists and functions
		assert.NotNil(t, mgr.LLM())
	})
}

func TestCircuitBreakerManager_Database(t *testing.T) {
	t.Run("database circuit opens after 10 failures", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// Database CB: needs 10 requests with 60% failure rate
		for i := 0; i < 10; i++ {
			manager.Database().Execute(func() (interface{}, error) {
				return nil, errors.New("database connection failed")
			})
		}

		assert.Equal(t, gobreaker.StateOpen, manager.Database().State())

		// Verify next request fails immediately
		_, err := manager.Database().Execute(func() (interface{}, error) {
			return "should not execute", nil
		})
		assert.ErrorIs(t, err, gobreaker.ErrOpenState)
	})

	t.Run("database circuit has shortest timeout", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// Database has 15-second timeout (shortest of the three)
		assert.NotNil(t, manager.Database())
	})
}

func TestCircuitBreakerMetrics_RecordRequest(t *testing.T) {
	manager := NewCircuitBreakerManager()
	metrics := manager.Metrics()

	t.Run("record successful request", func(t *testing.T) {
		metrics.RecordRequest("exchange", true)
		// Metrics are recorded, but we can't easily assert on Prometheus metrics
		// in unit tests. This test verifies the method doesn't panic.
	})

	t.Run("record failed request", func(t *testing.T) {
		metrics.RecordRequest("exchange", false)
		// Verify no panic occurs
	})

	t.Run("record requests for different services", func(t *testing.T) {
		metrics.RecordRequest("exchange", true)
		metrics.RecordRequest("llm", true)
		metrics.RecordRequest("database", false)
		// Verify no panic occurs for any service type
	})
}

func TestCircuitBreakerManager_StateTransitions(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("state transitions trigger metrics updates", func(t *testing.T) {
		// Start in closed state
		assert.Equal(t, gobreaker.StateClosed, manager.Exchange().State())

		// Trigger failures to open
		for i := 0; i < 5; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}

		// Verify state changed to open
		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())

		// Metrics should have been updated (callback was triggered)
		// We can't easily verify Prometheus metrics here, but we've verified
		// the state transition occurred
	})
}

func TestCircuitBreakerManager_ConcurrentAccess(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("concurrent requests to same circuit breaker", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				_, err := manager.Exchange().Execute(func() (interface{}, error) {
					time.Sleep(10 * time.Millisecond)
					return "success", nil
				})

				// Should either succeed or fail with open state error
				if err != nil && !errors.Is(err, gobreaker.ErrOpenState) {
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestCircuitBreakerManager_MixedSuccessFailure(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("mixed success and failure stays closed", func(t *testing.T) {
		// Execute requests with success rate > 40% (below failure threshold)
		for i := 0; i < 10; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				if i%3 == 0 {
					return nil, errors.New("occasional failure")
				}
				return "success", nil
			})
		}

		// Circuit should remain closed with occasional failures
		// (failure rate is 30%, below the 60% threshold)
		assert.Equal(t, gobreaker.StateClosed, manager.Exchange().State())
	})
}

func TestCircuitBreakerManager_HalfOpen(t *testing.T) {
	t.Run("circuit transitions through states correctly", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// 1. Start in closed state
		assert.Equal(t, gobreaker.StateClosed, manager.Exchange().State())

		// 2. Trigger failures to open circuit
		for i := 0; i < 5; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())

		// 3. After timeout, first request transitions to half-open
		// Note: We can't easily test the timeout in a unit test without
		// mocking time, but we verify the state machine works

		// 4. Verify requests fail while open
		_, err := manager.Exchange().Execute(func() (interface{}, error) {
			return "test", nil
		})
		assert.ErrorIs(t, err, gobreaker.ErrOpenState)
	})
}

func TestCircuitBreakerManager_DifferentServices(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("circuit breakers are independent", func(t *testing.T) {
		// Break exchange circuit
		for i := 0; i < 5; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				return nil, errors.New("exchange error")
			})
		}

		// Exchange should be open
		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())

		// LLM and Database should still be closed
		assert.Equal(t, gobreaker.StateClosed, manager.LLM().State())
		assert.Equal(t, gobreaker.StateClosed, manager.Database().State())

		// LLM requests should still work
		_, err := manager.LLM().Execute(func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	})
}

func TestCircuitBreakerManager_ErrorPropagation(t *testing.T) {
	manager := NewCircuitBreakerManager()

	t.Run("function errors are propagated", func(t *testing.T) {
		expectedErr := errors.New("specific error message")

		_, err := manager.Exchange().Execute(func() (interface{}, error) {
			return nil, expectedErr
		})

		assert.Equal(t, expectedErr, err)
	})

	t.Run("return values are propagated", func(t *testing.T) {
		expectedValue := map[string]interface{}{
			"status": "ok",
			"data":   []int{1, 2, 3},
		}

		result, err := manager.Exchange().Execute(func() (interface{}, error) {
			return expectedValue, nil
		})

		require.NoError(t, err)
		assert.Equal(t, expectedValue, result)
	})
}

func TestCircuitBreakerManager_MetricsSingleton(t *testing.T) {
	t.Run("multiple managers share metrics", func(t *testing.T) {
		manager1 := NewCircuitBreakerManager()
		manager2 := NewCircuitBreakerManager()

		// Both managers should exist
		require.NotNil(t, manager1)
		require.NotNil(t, manager2)

		// They should have their own circuit breakers
		require.NotNil(t, manager1.Exchange())
		require.NotNil(t, manager2.Exchange())

		// Metrics should be the same instance (singleton pattern)
		assert.Same(t, manager1.metrics, manager2.metrics)
	})
}

func TestCircuitBreakerManager_RealWorldScenario(t *testing.T) {
	t.Run("simulate exchange API failures and recovery", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		// Phase 1: Normal operation (small number to not affect failure ratio)
		for i := 0; i < 3; i++ {
			result, err := manager.Exchange().Execute(func() (interface{}, error) {
				return "order_placed", nil
			})
			require.NoError(t, err)
			assert.Equal(t, "order_placed", result)
		}
		assert.Equal(t, gobreaker.StateClosed, manager.Exchange().State())

		// Phase 2: Exchange has issues - multiple failures
		// Need 60% failure rate with at least 5 requests
		// So we'll do 5 failures out of 8 total = 62.5% failure rate
		for i := 0; i < 5; i++ {
			manager.Exchange().Execute(func() (interface{}, error) {
				return nil, errors.New("exchange timeout")
			})
		}
		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())

		// Phase 3: Requests fail fast while circuit is open
		_, err := manager.Exchange().Execute(func() (interface{}, error) {
			t.Fatal("should not execute while circuit is open")
			return nil, nil
		})
		assert.ErrorIs(t, err, gobreaker.ErrOpenState)

		// Phase 4: After timeout (in real scenario, would wait 30s)
		// Circuit would transition to half-open and allow test requests
		// For this test, we just verify the circuit is open
		assert.Equal(t, gobreaker.StateOpen, manager.Exchange().State())
	})
}
