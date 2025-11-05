# Test Coverage Improvement Plan

**Task**: T264 [P1] - Increase test coverage to >80%
**Current Status**: <20% overall coverage, 9 packages with failures
**Target**: >80% coverage, all tests passing
**Estimate**: 40 hours remaining

## Current Test Status (Before Fixes)

### ✅ Passing Packages (Good Coverage)
- `pkg/backtest` - 82.9% ✅
- `cmd/mcp-servers/risk-analyzer` - 80.0% ✅
- `tests/e2e` - 72.0%
- `internal/orchestrator` - 77.7%
- `internal/llm` - 76.2%
- `cmd/agents/orderbook-agent` - 66.8%
- `cmd/agents/sentiment-agent` - 62.5%
- `cmd/mcp-servers/technical-indicators` - 56.0%
- `internal/agents/testing` - 54.0%

### ❌ Packages with Failures

#### 1. Build Failures (P0 - Blocks Everything)
**internal/market** - Build failed
- Syntax error in coingecko.go (unclosed comment block)
- Missing types: CoinInfo, Candlestick
- Missing methods: GetCoinInfo(), Health(), ToCandlesticks()
- MCP SDK API mismatch (old API usage)

**cmd/test-mcp-client** - Build failed
- Dependency issues

#### 2. Test Failures (P1 - Need Fixes)
**cmd/agents/risk-agent** - 1 failure
- `TestCalculateOptimalSize_BasicCalculation` failing

**cmd/api** - 8 failures, 0% coverage
- `TestPauseTrading_Success`
- `TestPauseTrading_InvalidSessionID`
- `TestPauseTrading_NonExistentSession`
- `TestResumeTrading_Success`
- `TestOrchestratorFailure`
- `TestRateLimiting`
- `TestConcurrentPauseResume`
- `TestOrchestratorRetry`

**cmd/mcp-servers/market-data** - Build/test failures
- Dependency on internal/market fixes

**cmd/mcp-servers/order-executor** - 1 failure
- `TestOrderExecutorServer_ListTools`

**internal/config** - 3 failures, 90.4% coverage
- `TestAnalysisAgentConfig`
- `TestStrategyAgentConfig`
- `TestGetEnabledAgents`

**internal/llm** - 2 failures (but 76.2% coverage)
- `TestGetMessagesForPrompt`
- `TestGetMessagesForPrompt_SystemAlwaysIncluded`

#### 3. Low Coverage (P2 - Add Tests)
- `internal/metrics` - 8.9% (needs integration tests)
- `internal/db` - 4.9% (needs more query tests)
- `internal/api` - 0.0% (handler tests needed)
- `internal/agents` - 13.2% (base agent tests)
- `cmd/agents/technical-agent` - 22.1%
- `internal/memory` - 27.3%
- `internal/exchange` - 30.4%
- `cmd/agents/trend-agent` - 32.6%
- `cmd/agents/arbitrage-agent` - 39.2%
- `cmd/agents/reversion-agent` - 40.2%

---

## Fix Strategy (Phased Approach)

### Phase 1: Fix Build Failures (4-6 hours) ⚠️ IN PROGRESS

**Goal**: All packages compile

**Tasks**:
1. ✅ Fix metrics/updater.go syntax error (DONE)
2. ⚠️ Fix internal/market/coingecko.go:
   - Remove unclosed comment block (line 175+)
   - Add missing CoinInfo type
   - Add missing Candlestick type
   - Implement GetCoinInfo() stub
   - Implement Health() stub
   - Add ToCandlesticks() method to MarketChart
   - Update to new MCP SDK API (or use mocks)
3. Fix cmd/test-mcp-client dependencies
4. Verify: `go build ./...` succeeds

**Status**: 20% complete (1/5 tasks done)

---

### Phase 2: Fix Critical Test Failures (8-12 hours)

**Goal**: All existing tests pass

#### 2.1 Fix cmd/api Tests (4 hours)
All 8 tests in `trading_control_test.go` are failing. Likely issues:
- Mock orchestrator not properly configured
- HTTP client timeout/connection issues
- Test setup/teardown problems

**Action**:
```bash
cd cmd/api
go test -v -run TestPauseTrading_Success
# Analyze failure reason
# Fix mock/setup issues
# Repeat for each test
```

#### 2.2 Fix cmd/agents/risk-agent Test (1 hour)
`TestCalculateOptimalSize_BasicCalculation` failing.

**Action**:
```bash
go test -v ./cmd/agents/risk-agent -run TestCalculateOptimalSize
# Check Kelly Criterion calculation
# Verify test expectations
```

#### 2.3 Fix internal/config Tests (2 hours)
3 tests failing despite 90.4% coverage - likely assertion issues.

**Action**:
```bash
go test -v ./internal/config -run TestAnalysisAgentConfig
# Fix agent config loading
# Verify YAML parsing
```

#### 2.4 Fix internal/llm Tests (1 hour)
2 tests failing with 76.2% coverage - edge case issues.

**Action**:
```bash
go test -v ./internal/llm -run TestGetMessagesForPrompt
# Fix message formatting
# Verify system message inclusion
```

#### 2.5 Fix MCP Server Tests (2 hours)
- market-data server (depends on Phase 1)
- order-executor server (ListTools test)

---

### Phase 3: Increase Coverage to >80% (20-24 hours)

**Strategy**: Focus on high-impact, low-coverage packages first

#### 3.1 Add API Handler Tests (6 hours)
**Target**: internal/api 0% → 80%

```go
// Test plan:
- TestHealthEndpoint
- TestPositionsHandler
- TestOrdersHandler
- TestTradesHandler
- TestSessionsHandler
- TestMetricsHandler
- Test error cases
- Test middleware (auth, logging, metrics)
```

#### 3.2 Add Database Tests (4 hours)
**Target**: internal/db 4.9% → 80%

```go
// Test plan:
- Test connection pooling
- Test query execution
- Test transaction handling
- Test error scenarios
- Test migration functions
```

#### 3.3 Add Metrics Tests (3 hours)
**Target**: internal/metrics 8.9% → 80%

```go
// Test plan:
- Test metric registration
- Test metric updates
- Test database updater
- Test Redis wrapper
- Test HTTP middleware
```

#### 3.4 Add Agent Tests (5 hours)
**Target**: Various agents 20-40% → 80%

- Technical agent: 22.1% → 80%
- Trend agent: 32.6% → 80%
- Arbitrage agent: 39.2% → 80%
- Reversion agent: 40.2% → 80%

```go
// Test plan per agent:
- Test signal generation
- Test confidence calculation
- Test error handling
- Test NATS communication
- Test LLM integration
```

#### 3.5 Add Exchange Tests (3 hours)
**Target**: internal/exchange 30.4% → 80%

```go
// Test plan:
- Test mock exchange operations
- Test order placement
- Test position management
- Test P&L calculation
- Test slippage simulation
```

#### 3.6 Add Memory Tests (3 hours)
**Target**: internal/memory 27.3% → 80%

```go
// Test plan:
- Test memory storage/retrieval
- Test semantic search
- Test conversation history
- Test memory consolidation
```

---

## Testing Best Practices

### 1. Use Table-Driven Tests
```go
func TestCalculateOptimalSize(t *testing.T) {
	tests := []struct {
		name        string
		equity      float64
		winRate     float64
		riskReward  float64
		expected    float64
	}{
		{"basic", 10000, 0.6, 2.0, 0.1},
		{"high winrate", 10000, 0.8, 2.0, 0.3},
		// ... more cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateOptimalSize(tt.equity, tt.winRate, tt.riskReward)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}
```

### 2. Use Test Fixtures
```go
// fixtures/test_config.yaml
// fixtures/test_data.sql
// Use in tests for consistent data
```

### 3. Mock External Dependencies
```go
type mockNATSConn struct {
	PublishFunc func(string, []byte) error
}

func (m *mockNATSConn) Publish(subj string, data []byte) error {
	if m.PublishFunc != nil {
		return m.PublishFunc(subj, data)
	}
	return nil
}
```

### 4. Use testcontainers for Integration Tests
```go
func TestWithPostgres(t *testing.T) {
	ctx := context.Background()
	postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:15",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_PASSWORD": "test",
			},
		},
		Started: true,
	})
	// ... use container
}
```

### 5. Measure Coverage
```bash
# Run with coverage
go test -coverprofile=coverage.out ./...

# View coverage by package
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

---

## Progress Tracking

### Completed (This Session)
- [x] Fixed metrics/updater.go syntax error (unclosed string literal)
- [x] Updated CoinGeckoClient to use new MCP SDK API
- [x] Added mock data returns for GetPrice and GetMarketChart
- [x] Started cleanup of coingecko.go commented code

### In Progress
- [ ] Complete coingecko.go cleanup (remove commented code)
- [ ] Add missing types (CoinInfo, Candlestick)
- [ ] Add missing methods (GetCoinInfo, Health, ToCandlesticks)

### Not Started
- [ ] Fix cmd/api test failures (8 tests)
- [ ] Fix cmd/agents/risk-agent test failure
- [ ] Fix internal/config test failures (3 tests)
- [ ] Fix internal/llm test failures (2 tests)
- [ ] Fix MCP server test failures
- [ ] Add tests for low-coverage packages

---

## Estimated Timeline

| Phase | Tasks | Hours | Dependencies |
|-------|-------|-------|--------------|
| Phase 1: Build Fixes | 5 tasks | 4-6 | None |
| Phase 2: Test Fixes | 5 sub-phases | 8-12 | Phase 1 complete |
| Phase 3: Coverage | 6 sub-phases | 20-24 | Phase 2 complete |
| **Total** | **16 groups** | **32-42** | Sequential |

**Current Progress**: ~10% (Phase 1 - 20% complete)
**Remaining Work**: ~36 hours
**Original Estimate**: 40 hours ✅ On track

---

## Quick Reference Commands

```bash
# Run all tests with coverage
go test -v -cover ./... 2>&1 | tee test-results.txt

# Run specific package
go test -v -cover ./internal/api

# Run specific test
go test -v -run TestPauseTrading_Success ./cmd/api

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Check for race conditions
go test -race ./...

# Run with timeout
go test -timeout 30s ./...

# Parallel execution
go test -p 4 ./...
```

---

## Success Criteria

### Minimum (Production-Ready)
- ✅ All packages build successfully
- ✅ All existing tests pass
- ✅ Overall coverage >80%
- ✅ Critical packages (orchestrator, exchange, risk) >85%
- ✅ No race conditions detected

### Stretch Goals
- Coverage >85% overall
- All packages >75% coverage
- Benchmark tests added
- Performance regression tests
- Mutation testing >70%

---

## Notes

1. **Build failures block everything** - Must fix Phase 1 first
2. **internal/market is complex** - May need significant refactoring for MCP SDK update
3. **cmd/api tests all failing** - Likely systematic issue (mock setup)
4. **Some packages have high coverage but failing tests** - Likely assertion/expectation issues, easy to fix
5. **Low coverage packages** - Need new tests, time-consuming but straightforward

---

**Last Updated**: 2025-11-05
**Next Milestone**: Complete Phase 1 (build fixes)
**Blocker**: internal/market/coingecko.go cleanup (320+ lines of commented code)