# Test Coverage Improvement Plan

**Branch**: `feature/test-coverage-improvements`
**Goal**: Increase overall test coverage from ~40% to >60% using testcontainers for integration tests
**Status**: 📋 Planning Phase Complete

---

## Current State Analysis

### Coverage by Package (Baseline)

#### 🔴 Critical - Low Coverage (< 30%)
- `internal/db`: **8.4%** ← Migration tests fail, needs immediate attention
- `internal/technical-agent`: **22.1%** ← Core trading logic
- `internal/audit`: **26.1%** ← Security critical
- `internal/risk-agent`: **30.0%** ← Risk management

#### 🟡 Moderate - Needs Improvement (30-50%)
- `internal/memory`: 32.4%
- `internal/metrics`: 32.3%
- `internal/trend-agent`: 31.8%
- `internal/arbitrage-agent`: 39.2%
- `internal/reversion-agent`: 40.0%
- `internal/market`: 41.5%
- `internal/exchange`: 44.2%
- `internal/agents`: 45.5%

#### 🟢 Good - Maintain (> 80%)
- `internal/indicators`: 95.6% ✅
- `internal/alerts`: 91.2% ✅
- `internal/validation`: 84.5% ✅
- `pkg/backtest`: 82.9% ✅
- `internal/risk`: 81.6% ✅

---

## Immediate Blockers

### 1. Database Integration Test Failures ❌

**Status**: 3+ testcontainer tests failing due to migration issue
**Error**: `cannot create a unique index without the column "timestamp" (used in partitioning)`
**File**: `migrations/005_audit_logs.sql`
**Impact**: Blocks all database integration testing

**Fix Required**:
```sql
-- Current (FAILING):
CREATE UNIQUE INDEX idx_audit_logs_unique ON audit_logs (id, timestamp);

-- Should be (with timestamp in partition key):
CREATE INDEX idx_audit_logs_id ON audit_logs (id);
-- OR fix the partition definition to not partition by timestamp
```

**Priority**: P0 - Must fix before any other DB tests

### 2. API Integration Tests (7 failing out of 15) ❌

**Status**: Tests exist and use testcontainers, but 46% are failing
**Cause**: Multiple issues - response validation, error handling, route registration
**Impact**: Unknown coverage (tests fail before measurement)
**Details**: See API_TEST_ANALYSIS.md for comprehensive breakdown

**Action**: Fix individual test failures or defer to separate sprint (8-12 hour effort)

---

## Strategic Improvement Plan

### Phase 1: Fix Blockers (Priority: P0)

**Estimated Time**: 2-4 hours

#### Task 1.1: Fix Migration Issue
- [ ] Investigate 005_audit_logs.sql partitioning
- [ ] Fix unique index on partitioned table
- [ ] Verify all migrations apply successfully
- [ ] Re-run testcontainer tests

#### Task 1.2: Verify Database Tests Pass
- [ ] Run `go test ./internal/db -v`
- [ ] Ensure all testcontainer tests pass
- [ ] Document any remaining failures

**Success Criteria**: All DB integration tests pass

---

### Phase 2: Database Coverage (8.4% → 60%)

**Estimated Time**: 6-8 hours

#### Untested DB Methods (High Value)

**Sessions** (Partially tested):
- [x] CreateSession
- [x] GetSession
- [x] StopSession
- [x] UpdateSessionStats
- [ ] ListActiveSessions ← Has testcontainer test but commented unit test
- [ ] GetSessionsBySymbol ← Has testcontainer test but commented unit test

**Orders** (Needs expansion):
- [ ] InsertOrder
- [ ] GetOrder
- [ ] GetOrderByID
- [ ] UpdateOrderStatus
- [ ] GetOrdersBySession
- [ ] GetOrdersByStatus
- [ ] GetOrdersBySymbol
- [ ] GetRecentOrders

**Positions** (Needs expansion):
- [ ] CreatePosition
- [ ] UpdatePosition
- [ ] ClosePosition
- [ ] PartialClosePosition
- [ ] UpdatePositionQuantity
- [ ] UpdatePositionAveraging
- [ ] UpdateUnrealizedPnL
- [ ] GetPosition
- [ ] GetPositionsBySession
- [ ] GetPositionBySymbolAndSession
- [ ] GetAllOpenPositions
- [ ] GetLatestPositionBySymbol

**Trades**:
- [ ] InsertTrade
- [ ] GetTradesByOrderID

**LLM Decisions** (Partially tested):
- [x] InsertLLMDecision
- [x] FindSimilarDecisions
- [x] UpdateLLMDecisionOutcome
- [ ] GetLLMDecisionsByAgent
- [ ] GetLLMDecisionsBySymbol
- [ ] GetLLMDecisionStats
- [ ] GetSuccessfulLLMDecisions

**Agent Status** (Has integration tests):
- [x] UpsertAgentStatus
- [x] GetAgentStatus
- [x] GetAllAgentStatuses

**Test Strategy**:
1. Use existing testcontainer pattern from `testhelpers/testcontainers.go`
2. Create helper functions for common setups
3. Test both success and error cases
4. Test edge cases (NULL values, constraints, etc.)

**Example Template**:
```go
func TestInsertOrder_Integration(t *testing.T) {
    db, cleanup := testhelpers.SetupTestDB(t)
    defer cleanup()

    ctx := context.Background()

    // Create test session first
    session := createTestSession(t, db)

    // Test order insertion
    order := &Order{
        SessionID: session.ID,
        Symbol:    "BTCUSDT",
        Side:      OrderSideBuy,
        Type:      OrderTypeMarket,
        Quantity:  0.1,
        Status:    OrderStatusNew,
    }

    err := db.InsertOrder(ctx, order)
    require.NoError(t, err)
    assert.NotEqual(t, uuid.Nil, order.ID)
}
```

---

### Phase 3: Agent Coverage (22-45% → 60%)

**Estimated Time**: 8-12 hours

#### Technical Agent (22.1% → 60%)
**Missing Tests**:
- RSI calculation and signal generation
- MACD crossover detection
- Bollinger Band breakout logic
- Multi-indicator consensus logic
- Error handling for missing market data

**Strategy**:
```go
// Test RSI overbought/oversold signals
func TestTechnicalAgent_RSI_Signals(t *testing.T) {
    tests := []struct {
        name      string
        rsiValue  float64
        expected  Signal
    }{
        {"Oversold", 25.0, SignalBuy},
        {"Neutral", 50.0, SignalNone},
        {"Overbought", 75.0, SignalSell},
    }
    // ...
}
```

#### Risk Agent (30% → 60%)
**Missing Tests**:
- Kelly Criterion edge cases (already done in pkg/backtest)
- Circuit breaker triggers
- Position size calculation with various win rates
- Risk limit enforcement
- Drawdown tracking

#### Strategy Agents (31-40% → 60%)
**Trend Agent**:
- Pattern detection (higher highs, lower lows)
- Trend strength calculation
- Entry/exit signal generation

**Reversion Agent**:
- Mean reversion identification
- Bollinger Band mean reversion
- Z-score calculations

**Arbitrage Agent**:
- Price differential detection
- Fee-adjusted profit calculation
- Multi-exchange scenarios

---

### Phase 4: API Integration Tests (0% → 60%)

**Estimated Time**: 6-8 hours

**Current Issue**: 30+ tests failing due to HTTP mocking

**Solution**: Convert to testcontainers pattern

**Test Coverage Needed**:

#### Health & Status Endpoints
- [ ] `GET /health` - Basic health check
- [ ] `GET /readiness` - Kubernetes readiness
- [ ] `GET /status` - Orchestrator status
- [ ] `GET /config` - Configuration retrieval

#### Trading Endpoints
- [ ] `POST /api/v1/trading/start` - Start trading
- [ ] `POST /api/v1/trading/stop` - Stop trading
- [ ] `POST /api/v1/trading/pause` - Pause trading
- [ ] `POST /api/v1/trading/resume` - Resume trading

#### Order Endpoints
- [ ] `POST /api/v1/orders` - Place order
- [ ] `GET /api/v1/orders` - List orders
- [ ] `GET /api/v1/orders/:id` - Get order
- [ ] `DELETE /api/v1/orders/:id` - Cancel order

#### Position Endpoints
- [ ] `GET /api/v1/positions` - List positions
- [ ] `GET /api/v1/positions/:id` - Get position
- [ ] `POST /api/v1/positions/:id/close` - Close position

#### Agent Endpoints
- [ ] `GET /api/v1/agents` - List agents
- [ ] `GET /api/v1/agents/:id` - Get agent status

**Template**:
```go
func TestAPI_PlaceOrder_Integration(t *testing.T) {
    // Setup: DB + Redis + API server
    db, dbCleanup := testhelpers.SetupTestDB(t)
    defer dbCleanup()

    redis, redisCleanup := testhelpers.SetupTestRedis(t)
    defer redisCleanup()

    // Start API server
    api := startTestAPIServer(t, db, redis)
    defer api.Close()

    // Test request
    reqBody := `{"symbol":"BTCUSDT","side":"buy","quantity":0.1}`
    resp := httpPost(t, api.URL+"/api/v1/orders", reqBody)

    assert.Equal(t, http.StatusOK, resp.StatusCode)
    // Verify order in database
    // ...
}
```

---

### Phase 5: Supporting Packages (Various → +20%)

**Estimated Time**: 4-6 hours

#### Market Package (41.5% → 60%)
- [ ] CoinGecko API integration tests (with mock server)
- [ ] Cache hit/miss scenarios
- [ ] Redis integration for caching
- [ ] Error handling and retries

#### Exchange Package (44.2% → 65%)
- [ ] Multi-exchange fee calculations (already added in Phase 14)
- [ ] Slippage simulation tests
- [ ] Order execution flow tests
- [ ] Position manager with various scenarios

#### Audit Package (26.1% → 60%)
- [ ] Audit log creation and retrieval
- [ ] Log filtering and search
- [ ] Retention policy tests
- [ ] Performance under load

#### Memory Package (32.4% → 60%)
- [ ] Semantic memory storage/retrieval
- [ ] Procedural memory pattern matching
- [ ] Vector similarity search
- [ ] Memory cleanup and retention

---

## Implementation Priority

### Sprint 1: Unblock & Stabilize (Days 1-2) ✅ COMPLETE
1. ✅ Fix migration issue (005_audit_logs.sql) - Changed to composite PRIMARY KEY (id, timestamp)
2. ✅ Verify all testcontainer tests pass - All 8 tests now passing
3. ✅ Document baseline coverage

**Result**: Fixing the migration blocker allowed all existing testcontainer tests to run successfully, immediately improving DB coverage from 8.4% to **57.9%** (nearly meeting 60% target!)

### Sprint 2: Database Foundation (Days 3-4) - PAUSED
1. ~~Complete order CRUD integration tests~~ - Existing tests cover most methods
2. ~~Complete position CRUD integration tests~~ - Existing tests cover most methods
3. ~~Complete trade integration tests~~ - Existing tests cover most methods
4. ✅ Target: DB coverage 8.4% → 57.9% (nearly 60%)

**Note**: With 57.9% coverage achieved by fixing the blocker, additional order/position tests may not be immediately needed. Focus should shift to other low-coverage packages.

### Sprint 3: Agent Testing (Days 5-7)
1. Technical agent comprehensive tests
2. Risk agent comprehensive tests
3. Strategy agent tests (trend, reversion, arbitrage)
4. Target: Agent coverage 22-45% → 60%

### Sprint 4: API Integration (Days 8-9) - UPDATED AFTER ANALYSIS
**Status**: Tests already use testcontainers, need debugging not conversion

1. Fix 7 failing tests (response validation, error handling, route panic)
2. Add missing endpoint tests (order placement, cancellation, position close)
3. Test authentication/authorization (partially done)
4. Test rate limiting (exists but panics - needs fix)
5. Target: API coverage unknown → 70%
6. **Estimated Effort**: 8-12 hours (see API_TEST_ANALYSIS.md)

**Recommendation**: Consider deferring API work to separate sprint after audit/memory/metrics packages

### Sprint 5: Polish & Optimize (Day 10)
1. Supporting package tests (market, exchange, audit, memory)
2. Performance testing
3. Documentation updates
4. Final coverage verification

---

## Testing Infrastructure

### Testcontainers Setup (Already Available)

**Location**: `internal/db/testhelpers/testcontainers.go`

**Available Helpers**:
```go
func SetupTestDB(t *testing.T) (*db.DB, func())
func SetupTestDBWithMigrations(t *testing.T, migrations ...string) (*db.DB, func())
```

**Usage Pattern**:
```go
func TestSomething_Integration(t *testing.T) {
    db, cleanup := testhelpers.SetupTestDB(t)
    defer cleanup()

    // Test logic here
}
```

### Additional Helpers Needed

**Redis Testcontainer**:
```go
func SetupTestRedis(t *testing.T) (*redis.Client, func())
```

**NATS Testcontainer**:
```go
func SetupTestNATS(t *testing.T) (*nats.Conn, func())
```

**API Server Helper**:
```go
func StartTestAPIServer(t *testing.T, db *db.DB, redisClient *redis.Client) (*httptest.Server, func())
```

---

## Success Metrics

### Coverage Targets

| Package | Current | Target | Improvement |
|---------|---------|--------|-------------|
| `internal/db` | 8.4% | 60% | +51.6% |
| `internal/technical-agent` | 22.1% | 60% | +37.9% |
| `internal/audit` | 26.1% | 60% | +33.9% |
| `internal/risk-agent` | 30% | 60% | +30% |
| `cmd/api` | 0% | 60% | +60% |
| **Overall** | **~40%** | **>60%** | **+20%** |

### Quality Gates

- [ ] All integration tests use testcontainers
- [ ] 0 failing tests
- [ ] All P0 packages > 60% coverage
- [ ] CI/CD pipeline passes
- [ ] No regression in existing tests
- [ ] Performance benchmarks pass

---

## Risk Mitigation

### Risk 1: Migration Issue Persists
**Mitigation**:
- Document issue thoroughly
- Create workaround migrations
- Escalate to database expert if needed

### Risk 2: Testcontainer Setup Complexity
**Mitigation**:
- Use existing patterns from internal/db
- Create reusable helper functions
- Document setup process

### Risk 3: Time Overrun
**Mitigation**:
- Prioritize P0 packages (db, agents, API)
- Accept lower coverage on P2 packages if needed
- Can split into multiple PRs

---

## Sprint Progress Update

### ✅ Sprint 1: Complete (Database Foundation)
**Status**: EXCEEDED EXPECTATIONS
- ✅ Fixed migration blocker (005_audit_logs.sql)
- ✅ All 14 testcontainer integration tests passing
- ✅ Coverage improved 8.4% → 57.9% (+49.5%)
- ✅ Comprehensive documentation created
- ✅ PR #17 created and merged

**Time**: 1 hour (vs 6-8 hours estimated for writing new tests)
**ROI**: ⭐⭐⭐⭐⭐ Exceptional

### 📊 Sprint 2: API Analysis (Complete)
**Status**: ANALYSIS COMPLETE - DEFER TO FUTURE SPRINT
- ✅ Analyzed all API test files
- ✅ Identified 7 failing tests (46% failure rate)
- ✅ Documented issues in API_TEST_ANALYSIS.md
- ✅ Estimated fix effort: 8-12 hours

**Key Finding**: Unlike database (single blocker), API requires individual debugging of 7+ issues

**Decision**: Defer API test fixes to dedicated sprint, focus on easier wins next

### 🎯 Next Recommended Sprints

**Sprint 3: Audit Package** (Recommended Next)
- **Current**: 26.1% coverage
- **Target**: 60%
- **Effort**: 4-6 hours
- **Method**: Testcontainers pattern (proven success)
- **Priority**: High (security/compliance critical)
- **ROI**: ⭐⭐⭐⭐ High

**Sprint 4: Memory Package**
- **Current**: 32.4% coverage
- **Target**: 60%
- **Effort**: 6-8 hours
- **Method**: Testcontainers + semantic search tests
- **ROI**: ⭐⭐⭐ Medium

**Sprint 5: Metrics Package**
- **Current**: 32.3% coverage
- **Target**: 60%
- **Effort**: 4-6 hours
- **Method**: Integration tests with Prometheus
- **ROI**: ⭐⭐⭐ Medium

**Sprint 6: API Package** (Defer to later)
- **Current**: Unknown (tests failing)
- **Target**: 70%
- **Effort**: 8-12 hours
- **Method**: Debug 7 failing tests + add missing endpoints
- **ROI**: ⭐⭐⭐ Medium (important but time-consuming)

## Next Steps

### Immediate (Recommended)
1. ✅ **Complete**: Database migration fix
2. ✅ **Complete**: API test analysis
3. **Next**: Choose sprint focus - audit, memory, or metrics package
4. **Future**: API test fixes in dedicated sprint

### Original Next Steps (Reference)
1. ~~**Fix Migration**: Resolve 005_audit_logs.sql issue~~ ✅ DONE
2. ~~**Verify Baseline**: Ensure all existing tests pass~~ ✅ DONE (for DB)
3. ~~**Begin Sprint 1**: Database coverage improvements~~ ✅ COMPLETE
4. **Regular Updates**: Commit incrementally with clear messages ✅ Ongoing
5. **Create PR**: When coverage targets met ✅ PR #17 created

---

**Document Version**: 1.0
**Last Updated**: 2025-11-16
**Owner**: Development Team
**Status**: 📋 READY FOR IMPLEMENTATION
