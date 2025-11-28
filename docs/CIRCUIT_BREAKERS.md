# Circuit Breakers Implementation

## Overview

CryptoFunk implements a comprehensive circuit breaker system using `github.com/sony/gobreaker` to prevent cascading failures and improve system resilience. Circuit breakers are integrated into all critical external dependencies: Exchange APIs, LLM Gateway, and Database connections.

## Architecture

### Circuit Breaker Manager

Located in `internal/risk/circuit_breaker.go`, the `CircuitBreakerManager` manages three independent circuit breakers:

1. **Exchange API Circuit Breaker**
   - Threshold: 5 requests with 60% failure rate
   - Open timeout: 30 seconds
   - Max requests in half-open: 3

2. **LLM Gateway Circuit Breaker**
   - Threshold: 3 requests with 60% failure rate
   - Open timeout: 60 seconds (longer for LLM recovery)
   - Max requests in half-open: 2

3. **Database Circuit Breaker**
   - Threshold: 10 requests with 60% failure rate
   - Open timeout: 15 seconds (shortest for quick recovery)
   - Max requests in half-open: 5

### States

Circuit breakers transition through three states:

1. **Closed** (0): Normal operation, all requests pass through
2. **Open** (1): Service is failing, requests fail immediately with `ErrOpenState`
3. **Half-Open** (2): Testing recovery, limited requests allowed

### Prometheus Metrics

The circuit breaker system exposes three Prometheus metrics:

```go
// State gauge (0=closed, 1=open, 2=half-open)
circuit_breaker_state{service="exchange|llm|database"}

// Request counter by result
circuit_breaker_requests_total{service="exchange|llm|database",result="success|failure"}

// Failure counter
circuit_breaker_failures_total{service="exchange|llm|database"}
```

## Integration

### Exchange Service

The exchange service automatically wraps API calls with circuit breaker protection:

```go
// internal/exchange/service.go
func (s *Service) PlaceMarketOrder(args map[string]interface{}) (interface{}, error) {
    // Place order through circuit breaker
    cbResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
        return s.exchange.PlaceOrder(ctx, req)
    })

    if err != nil {
        if err == gobreaker.ErrOpenState {
            s.circuitBreaker.Metrics().RecordRequest("exchange", false)
            return nil, fmt.Errorf("exchange circuit breaker is open, system unavailable")
        }
        s.circuitBreaker.Metrics().RecordRequest("exchange", false)
        return nil, fmt.Errorf("failed to place order: %w", err)
    }

    s.circuitBreaker.Metrics().RecordRequest("exchange", true)
    return cbResult.(*PlaceOrderResponse), nil
}
```

### LLM Client

The LLM client wraps all API calls with circuit breaker protection:

```go
// internal/llm/client.go
func (c *Client) Complete(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
    // Wrap the LLM call with circuit breaker
    result, err := c.circuitBreaker.LLM().Execute(func() (interface{}, error) {
        return c.doComplete(ctx, messages)
    })

    if err != nil {
        if err == gobreaker.ErrOpenState {
            c.circuitBreaker.Metrics().RecordRequest("llm", false)
            return nil, fmt.Errorf("LLM circuit breaker is open, service unavailable")
        }
        c.circuitBreaker.Metrics().RecordRequest("llm", false)
        return nil, err
    }

    c.circuitBreaker.Metrics().RecordRequest("llm", true)
    return result.(*ChatResponse), nil
}
```

### Database Operations

The database package provides a helper method for circuit breaker protection:

```go
// internal/db/db.go
func (db *DB) ExecuteWithCircuitBreaker(operation func() (interface{}, error)) (interface{}, error) {
    result, err := db.circuitBreaker.Database().Execute(operation)
    if err != nil {
        if err == gobreaker.ErrOpenState {
            db.circuitBreaker.Metrics().RecordRequest("database", false)
            return nil, fmt.Errorf("database circuit breaker is open, service unavailable")
        }
        db.circuitBreaker.Metrics().RecordRequest("database", false)
        return nil, err
    }

    db.circuitBreaker.Metrics().RecordRequest("database", true)
    return result, nil
}
```

Usage example:

```go
result, err := db.ExecuteWithCircuitBreaker(func() (interface{}, error) {
    var session TradingSession
    err := db.Pool().QueryRow(ctx, query, args...).Scan(&session)
    return &session, err
})
```

### Orchestrator

The orchestrator has access to circuit breakers for all components:

```go
// internal/orchestrator/orchestrator.go
type Orchestrator struct {
    circuitBreaker *risk.CircuitBreakerManager
    // ... other fields
}

// Access circuit breaker for external use
func (o *Orchestrator) GetCircuitBreaker() *risk.CircuitBreakerManager {
    return o.circuitBreaker
}
```

## Usage Examples

### Basic Usage

```go
manager := risk.NewCircuitBreakerManager()

// Execute operation with circuit breaker
result, err := manager.Exchange().Execute(func() (interface{}, error) {
    // Your operation here
    return fetchMarketData()
})

if err == gobreaker.ErrOpenState {
    // Circuit is open, fail fast
    log.Error().Msg("Circuit breaker is open")
    return
}
```

### Sharing Circuit Breakers

Components can share circuit breaker instances:

```go
// Create shared circuit breaker
cb := risk.NewCircuitBreakerManager()

// Use in LLM client
llmClient := llm.NewClient(llm.ClientConfig{
    CircuitBreaker: cb,
    // ... other config
})

// Use in database
database.SetCircuitBreaker(cb)
```

### Monitoring State

```go
manager := risk.NewCircuitBreakerManager()

// Check circuit breaker state
state := manager.Exchange().State()
switch state {
case gobreaker.StateClosed:
    log.Info().Msg("Circuit is closed - healthy")
case gobreaker.StateOpen:
    log.Warn().Msg("Circuit is open - failing")
case gobreaker.StateHalfOpen:
    log.Info().Msg("Circuit is half-open - testing recovery")
}
```

## Testing

Comprehensive unit tests are in `internal/risk/circuit_breaker_test.go`:

```bash
# Run circuit breaker tests
go test -v -run TestCircuitBreaker ./internal/risk/

# Run all risk package tests
go test -v ./internal/risk/
```

### Test Coverage

- Circuit breaker initialization
- State transitions (closed → open → half-open)
- Threshold behavior (failure rates)
- Concurrent access safety
- Metrics recording
- Error propagation
- Independent service isolation
- Real-world scenarios

## Operational Guidelines

### When Circuit Opens

When a circuit breaker opens:

1. **Alert**: Monitor `circuit_breaker_state` metric
2. **Investigate**: Check service health (exchange API, LLM gateway, database)
3. **Wait**: Circuit automatically attempts recovery after timeout
4. **Monitor**: Watch `circuit_breaker_requests_total` for recovery

### Tuning Parameters

**Recommended Method: Configuration File**

Circuit breaker thresholds are now fully configurable via `configs/config.yaml`:

```yaml
risk:
  circuit_breaker:
    # Exchange Circuit Breaker
    exchange:
      min_requests: 5           # Minimum requests before circuit can trip
      failure_ratio: 0.6        # Trip after 60% failure rate
      open_timeout: "30s"       # Wait 30s before attempting recovery
      half_open_max_reqs: 3     # Test with 3 requests in half-open state
      count_interval: "10s"     # Count failures over 10s window

    # LLM Circuit Breaker
    llm:
      min_requests: 3
      failure_ratio: 0.6
      open_timeout: "60s"       # Longer for AI services
      half_open_max_reqs: 2
      count_interval: "10s"

    # Database Circuit Breaker
    database:
      min_requests: 10
      failure_ratio: 0.6
      open_timeout: "15s"       # Quick recovery for DB
      half_open_max_reqs: 5
      count_interval: "10s"
```

**Programmatic Usage:**

```go
// Create with custom settings
exchangeSettings := &risk.ServiceSettings{
    MinRequests:     10,
    FailureRatio:    0.7,
    OpenTimeout:     60 * time.Second,
    HalfOpenMaxReqs: 5,
    CountInterval:   20 * time.Second,
}

manager := risk.NewCircuitBreakerManagerWithSettings(
    exchangeSettings, // Exchange settings
    nil,              // LLM settings (use defaults)
    nil,              // DB settings (use defaults)
)
```

**Default Behavior:**

If no configuration is provided, the system uses the default constants defined in `internal/risk/circuit_breaker.go`. This maintains backward compatibility with existing code.

### Monitoring Queries

```promql
# Circuit breaker state by service
circuit_breaker_state{service="exchange"}

# Request success rate
rate(circuit_breaker_requests_total{result="success"}[5m])
  /
rate(circuit_breaker_requests_total[5m])

# Circuit open events (state transition 0→1)
changes(circuit_breaker_state{service="exchange"}[1h]) > 0

# Total failures in last hour
increase(circuit_breaker_failures_total[1h])
```

### Grafana Dashboard

Create alerts for:

1. Circuit breaker opens: `circuit_breaker_state > 0`
2. High failure rate: `rate(circuit_breaker_failures_total[5m]) > 0.1`
3. Frequent trips: `changes(circuit_breaker_state[1h]) > 3`

## Benefits

1. **Fail Fast**: Prevents cascading failures by quickly detecting and isolating failing services
2. **Automatic Recovery**: Circuits automatically test recovery after timeout
3. **Resource Protection**: Prevents resource exhaustion from repeated failed calls
4. **Observability**: Comprehensive Prometheus metrics for monitoring
5. **Independent Services**: Each service has its own circuit breaker with appropriate thresholds

## Design Decisions

### Why Different Timeouts?

- **Exchange (30s)**: Balance between responsiveness and allowing exchange APIs time to recover
- **LLM (60s)**: Longer timeout as LLM gateway failures may indicate upstream issues requiring more recovery time
- **Database (15s)**: Shortest timeout as database issues are usually quick to resolve or indicate serious problems requiring fast failover

### Why Failure Ratio vs Fixed Count?

Using failure ratio (60%) instead of fixed counts prevents false positives during low traffic periods and ensures circuit opens only when there's a genuine pattern of failures.

### Singleton Metrics

Prometheus metrics use singleton pattern to prevent duplicate registration errors when creating multiple circuit breaker instances (common in tests and multi-component scenarios).

## References

- [sony/gobreaker Documentation](https://github.com/sony/gobreaker)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Prometheus Metrics Best Practices](https://prometheus.io/docs/practices/naming/)
