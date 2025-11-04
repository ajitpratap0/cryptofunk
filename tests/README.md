# CryptoFunk Test Suite

This directory contains all automated tests for the CryptoFunk AI Trading Platform.

## Directory Structure

```
tests/
├── unit/              # Unit tests (isolated, fast)
├── integration/       # Integration tests (with dependencies)
├── e2e/              # End-to-end tests (full system)
├── fixtures/          # Test data and fixtures
└── README.md         # This file
```

## Test Categories

### Unit Tests (`tests/unit/`)

**Purpose**: Test individual functions and methods in isolation.

**Characteristics**:
- Fast execution (< 1ms per test)
- No external dependencies (databases, APIs, file system)
- Use mocks and stubs
- High coverage of edge cases

**Examples**:
- Testing indicator calculations
- Testing configuration parsing
- Testing utility functions
- Testing business logic

**Run unit tests**:
```bash
task test-unit
# or
go test ./tests/unit/... -v
```

### Integration Tests (`tests/integration/`)

**Purpose**: Test interactions between components.

**Characteristics**:
- Medium execution time (10ms-100ms per test)
- May use databases, Redis, NATS
- Test component integration
- Use testcontainers or docker-compose

**Examples**:
- Testing database queries
- Testing MCP server communication
- Testing agent coordination
- Testing API endpoints with database

**Run integration tests**:
```bash
task test-integration
# or
go test ./tests/integration/... -v
```

**Requirements**:
- Docker must be running (for testcontainers)
- PostgreSQL, Redis, NATS services available

### End-to-End Tests (`tests/e2e/`)

**Purpose**: Test complete user workflows and system behavior.

**Characteristics**:
- Slow execution (1s-30s per test)
- Use full system stack
- Test real-world scenarios
- Verify system-level behavior

**Examples**:
- Complete trading cycle (market data → decision → execution)
- Paper trading workflow
- Agent coordination and consensus
- Error recovery and circuit breakers
- Position management and P&L calculation

**Run E2E tests**:
```bash
task test-e2e
# or
go test ./tests/e2e/... -v -timeout=5m
```

**Requirements**:
- All infrastructure services running (task dev)
- Database migrations applied
- All MCP servers available

### Test Fixtures (`tests/fixtures/`)

**Purpose**: Shared test data and helper utilities.

**Contents**:
- Sample candlestick data
- Mock API responses
- Configuration files for testing
- Helper functions
- Test database schemas

**Example usage**:
```go
import "github.com/ajitpratapsingh/cryptofunk/tests/fixtures"

candlesticks := fixtures.LoadCandlesticks("BTC-1h-sample.json")
mockResponse := fixtures.LoadMockResponse("coingecko-btc-price.json")
```

## Running Tests

### Run All Tests
```bash
# Run all tests with coverage
task test

# Run with race detector
task test-race

# Run with verbose output
go test ./... -v

# Run with coverage report
task test-coverage
```

### Run Specific Test
```bash
# Run a specific test function
go test -v -run TestPlaceMarketOrder ./internal/exchange/

# Run tests in a specific package
go test -v ./internal/orchestrator/...

# Run tests matching a pattern
go test -v -run ".*Integration" ./...
```

### Watch Mode
```bash
# Automatically run tests on file changes
task test-watch
```

### Benchmarks
```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkConsensusDecision ./internal/orchestrator/
```

## Test Organization Guidelines

### When to Write Unit Tests

- For pure functions with clear inputs/outputs
- For business logic without external dependencies
- For utility functions and helpers
- For data transformations
- For validation logic

### When to Write Integration Tests

- When testing database queries
- When testing MCP protocol communication
- When testing API endpoints
- When testing multi-component interactions
- When testing external service integrations

### When to Write E2E Tests

- For critical user workflows
- For trading scenarios (paper and live)
- For disaster recovery scenarios
- For performance under load
- For system-level behavior verification

## Test Best Practices

### 1. Test Naming

```go
// Good: Describes what is being tested and expected outcome
func TestPlaceMarketOrder_WithValidParams_ReturnsOrder(t *testing.T)
func TestCalculateRSI_WithInsufficientData_ReturnsError(t *testing.T)

// Bad: Unclear what is being tested
func TestOrder(t *testing.T)
func TestRSI(t *testing.T)
```

### 2. Table-Driven Tests

```go
func TestCalculateRSI(t *testing.T) {
    tests := []struct {
        name      string
        prices    []float64
        period    int
        wantRSI   float64
        wantError bool
    }{
        {"insufficient data", []float64{100, 101}, 14, 0, true},
        {"oversold", []float64{...}, 14, 25.5, false},
        {"overbought", []float64{...}, 14, 75.2, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            rsi, err := CalculateRSI(tt.prices, tt.period)
            // assertions...
        })
    }
}
```

### 3. Setup and Teardown

```go
func TestMain(m *testing.M) {
    // Setup: Start testcontainers, load fixtures
    setup()

    // Run tests
    code := m.Run()

    // Teardown: Clean up resources
    teardown()

    os.Exit(code)
}
```

### 4. Use testify Assertions

```go
import "github.com/stretchr/testify/assert"

func TestOrder(t *testing.T) {
    order, err := PlaceOrder(...)

    assert.NoError(t, err)
    assert.NotNil(t, order)
    assert.Equal(t, "BUY", order.Side)
    assert.Greater(t, order.Quantity, 0.0)
}
```

### 5. Mock External Dependencies

```go
// Use interfaces for easy mocking
type ExchangeClient interface {
    PlaceOrder(ctx context.Context, order Order) (*OrderResponse, error)
}

// In tests, use mock implementation
type MockExchangeClient struct {
    PlaceOrderFunc func(ctx context.Context, order Order) (*OrderResponse, error)
}

func (m *MockExchangeClient) PlaceOrder(ctx context.Context, order Order) (*OrderResponse, error) {
    return m.PlaceOrderFunc(ctx, order)
}
```

### 6. Test Error Cases

```go
func TestPlaceOrder_ErrorCases(t *testing.T) {
    tests := []struct {
        name      string
        setup     func() ExchangeClient
        wantError string
    }{
        {
            name: "insufficient balance",
            setup: func() ExchangeClient {
                return &MockClient{Error: ErrInsufficientBalance}
            },
            wantError: "insufficient balance",
        },
        {
            name: "invalid symbol",
            setup: func() ExchangeClient {
                return &MockClient{Error: ErrInvalidSymbol}
            },
            wantError: "invalid symbol",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client := tt.setup()
            _, err := client.PlaceOrder(...)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tt.wantError)
        })
    }
}
```

## Coverage Requirements

**Target**: >80% overall coverage

**Minimum Requirements**:
- Critical trading logic: >90%
- Business logic: >80%
- API endpoints: >75%
- Utilities: >70%

**Check coverage**:
```bash
# Generate coverage report
task test-coverage

# View coverage in browser
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Coverage by package** (current baseline):
```
internal/orchestrator:      ~75%
internal/exchange:          ~65%
internal/risk:              ~60%
internal/agents:            ~70%
internal/llm:               ~55%
cmd/agents/*:               ~40%
cmd/mcp-servers/*:          ~45%
```

## Continuous Integration

All tests run automatically on:
- Pull request creation
- Push to develop/main branches
- Manual workflow dispatch

**CI Pipeline** (`.github/workflows/ci.yml`):
1. Lint (golangci-lint)
2. Unit tests (fast feedback)
3. Integration tests (with service containers)
4. E2E tests (full stack)
5. Coverage check (fail if <40%)
6. Build verification

**Quality Gates**:
- All tests must pass
- Coverage must not decrease
- No new linting errors
- Code review required

## Performance Baselines

See `internal/agents/testing/PERFORMANCE_BASELINES.md` for detailed benchmarks.

**Key Metrics**:
- Agent decision time: <100ms
- MCP tool call latency: <50ms
- Database query time: <10ms
- API endpoint response: <100ms

## Troubleshooting

### Tests Timing Out

**Issue**: E2E tests timeout after 2 minutes

**Solutions**:
1. Increase timeout: `go test -timeout=5m`
2. Check if services are running: `task docker-status`
3. Check logs: `task docker-logs`

### Database Connection Errors

**Issue**: Tests fail with "connection refused"

**Solutions**:
1. Start infrastructure: `task docker-up`
2. Run migrations: `task db-migrate`
3. Check database health: `task db-status`

### Race Detector Failures

**Issue**: Tests fail with race detector enabled

**Solutions**:
1. Fix data races in code
2. Use proper locking mechanisms
3. Avoid shared mutable state

### Flaky Tests

**Issue**: Tests pass/fail non-deterministically

**Solutions**:
1. Add explicit waits instead of sleeps
2. Use proper synchronization
3. Isolate test data
4. Reset state between tests

## Adding New Tests

### 1. Choose Test Type

Ask yourself:
- Does this test external dependencies? → Integration or E2E
- Is this a pure function? → Unit
- Does this test a complete workflow? → E2E

### 2. Create Test File

```bash
# Unit test
touch tests/unit/my_feature_test.go

# Integration test
touch tests/integration/my_integration_test.go

# E2E test
touch tests/e2e/my_workflow_test.go
```

### 3. Write Test

```go
package unit

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestMyFeature(t *testing.T) {
    // Arrange
    input := setupInput()

    // Act
    result, err := MyFeature(input)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### 4. Run Test

```bash
# Run your new test
go test -v -run TestMyFeature ./tests/unit/

# Run with coverage
go test -cover -run TestMyFeature ./tests/unit/
```

### 5. Add to CI

Tests in `tests/` directories are automatically picked up by CI.

## Resources

- **Go Testing**: https://go.dev/doc/tutorial/add-a-test
- **Testify**: https://github.com/stretchr/testify
- **Testcontainers**: https://golang.testcontainers.org/
- **Table-Driven Tests**: https://go.dev/wiki/TableDrivenTests
- **Code Coverage**: https://go.dev/blog/cover

## Contributing

When adding features, please include:
1. Unit tests for business logic
2. Integration tests for external interactions
3. E2E tests for user workflows (if applicable)
4. Update this README if adding new patterns

See [CONTRIBUTING.md](../CONTRIBUTING.md) for general contribution guidelines.

---

**Last Updated**: 2025-11-04
**Phase**: 10 - Production Readiness
**Task**: T260 - Create /tests directory structure
