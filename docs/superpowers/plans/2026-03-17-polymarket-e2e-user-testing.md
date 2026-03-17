# Polymarket E2E User Testing Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose and fix real bugs in the Polymarket paper trading stack via true end-to-end user-flow tests, then add an agent-driven E2E scenario.

**Architecture:** Four independent task areas — (A) session continuity bug in the REST API, (B) session-scoped balance bug in `NewDBPaperEngineWithSession`, (C) agent step E2E with mocked MCP/LLM, (D) performance-endpoint correctness. All testcontainers-backed.

**Tech Stack:** Go 1.24, testcontainers-go, gin, testify, zerolog, zerolog/nop

---

## Known Bugs (confirmed by code review)

| # | File | Bug | Impact |
|---|------|-----|--------|
| B1 | `internal/api/polymarket.go:ExecuteTrade` | No `session_id` returned in response → client can't reuse session for SELL | SELL after BUY always fails with "position not found" in a real user flow |
| B2 | `internal/polymarket/paper/db_engine.go:NewDBPaperEngineWithSession` | `GetPolymarketPortfolioSummary(ctx)` is global (all sessions), not scoped to the requested session | Balance wrong when multiple sessions exist |
| B3 | `internal/api/polymarket.go:GetPortfolio` | Uses global `GetPolymarketPortfolioSummary(ctx)` — fine for now, but no way to query per-session | All sessions mixed together |

---

## File Map

| File | Status | Purpose |
|------|--------|---------|
| `tests/e2e/polymarket_user_flow_test.go` | CREATE | E2E user-flow: BUY via REST → portfolio → SELL via REST with returned session_id |
| `tests/e2e/polymarket_agent_step_test.go` | CREATE | Agent.Step() with mock MCP + mock LLM; verify signals + in-memory trades |
| `internal/api/polymarket.go` | MODIFY:243 | Add `session_id` to ExecuteTrade response |
| `internal/polymarket/paper/db_engine.go` | MODIFY:54 | Fix `NewDBPaperEngineWithSession` to use session-scoped summary |
| `internal/db/polymarket.go` | MODIFY | Add `GetPolymarketPortfolioSummaryBySession(ctx, sessionID)` |

---

## Task A — Fix B1: Return session_id from ExecuteTrade

**Files:**
- Modify: `internal/api/polymarket.go:243`
- Test: `tests/e2e/polymarket_user_flow_test.go`

- [ ] **A1: Write failing test — BUY then SELL reusing session_id**

File: `tests/e2e/polymarket_user_flow_test.go`

```go
//go:build integration

package e2e

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/ajitpratapsingh/cryptofunk/internal/api"
    "github.com/ajitpratapsingh/cryptofunk/internal/db/testhelpers"
)

func TestUserFlow_BuyThenSellWithSession(t *testing.T) {
    testhelpers.RequireDocker(t)
    tc := testhelpers.SetupTestDatabase(t)
    require.NoError(t, tc.ApplyMigrations("../../migrations"))
    database := tc.DB

    gin.SetMode(gin.TestMode)
    router := gin.New()
    h := api.NewPolymarketHandler(database)
    pg := router.Group("/polymarket")
    h.RegisterRoutes(pg)

    // Step 1: BUY — no session_id provided
    buyBody := `{"action":"BUY","market_id":"test-market","question":"Will X happen?","side":"YES","amount":20,"price":0.5}`
    req1 := httptest.NewRequestWithContext(t.Context(), "POST", "/polymarket/trade", bytes.NewBufferString(buyBody))
    req1.Header.Set("Content-Type", "application/json")
    w1 := httptest.NewRecorder()
    router.ServeHTTP(w1, req1)
    require.Equal(t, http.StatusOK, w1.Code, "BUY should succeed: %s", w1.Body)

    var buyResp map[string]any
    require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &buyResp))

    // session_id MUST be present in BUY response
    sessionID, ok := buyResp["session_id"].(string)
    require.True(t, ok && sessionID != "", "BUY response must include session_id, got: %v", buyResp)

    balanceAfterBuy, _ := buyResp["balance"].(float64)
    assert.InDelta(t, 80.0, balanceAfterBuy, 0.01, "balance after $20 buy should be $80")

    // Step 2: GET /portfolio — should show 1 open position
    req2 := httptest.NewRequestWithContext(t.Context(), "GET", "/polymarket/portfolio", nil)
    w2 := httptest.NewRecorder()
    router.ServeHTTP(w2, req2)
    require.Equal(t, http.StatusOK, w2.Code)

    var portfolio map[string]any
    require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &portfolio))
    posCount, _ := portfolio["position_count"].(float64)
    assert.Equal(t, 1.0, posCount, "portfolio should show 1 open position")

    // Step 3: SELL using returned session_id (40 shares at 0.8)
    sellBody := `{"action":"SELL","market_id":"test-market","side":"YES","price":0.8,"shares":40,"session_id":"` + sessionID + `"}`
    req3 := httptest.NewRequestWithContext(t.Context(), "POST", "/polymarket/trade", bytes.NewBufferString(sellBody))
    req3.Header.Set("Content-Type", "application/json")
    w3 := httptest.NewRecorder()
    router.ServeHTTP(w3, req3)
    require.Equal(t, http.StatusOK, w3.Code, "SELL should succeed: %s", w3.Body)

    var sellResp map[string]any
    require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &sellResp))
    // sell proceeds = 40 * 0.8 = 32; balance = 80 + 32 = 112
    balanceAfterSell, _ := sellResp["balance"].(float64)
    assert.InDelta(t, 112.0, balanceAfterSell, 0.01, "balance after sell: $80 + 40*$0.8 = $112")

    // Step 4: GET /performance — verify P&L
    req4 := httptest.NewRequestWithContext(t.Context(), "GET", "/polymarket/performance", nil)
    w4 := httptest.NewRecorder()
    router.ServeHTTP(w4, req4)
    require.Equal(t, http.StatusOK, w4.Code, "performance endpoint should return 200")
}
```

- [ ] **A2: Run test, confirm it fails with "session_id missing" or SELL fails**

```bash
cd /Users/ajitpratapsingh/dev/cryptofunk
go test -v -tags=integration -run TestUserFlow_BuyThenSellWithSession ./tests/e2e/... 2>&1
```
Expected: FAIL — either `session_id` not in BUY response, or SELL returns 400 "no position for test-market YES"

- [ ] **A3: Fix — add `session_id` to ExecuteTrade response in `internal/api/polymarket.go`**

Replace:
```go
c.JSON(http.StatusOK, gin.H{
    "trade":   trade,
    "balance": engine.GetBalance(),
    "message": "Trade executed successfully",
})
```
With:
```go
c.JSON(http.StatusOK, gin.H{
    "trade":      trade,
    "balance":    engine.GetBalance(),
    "session_id": engine.SessionID().String(),
    "message":    "Trade executed successfully",
})
```

Also expose `SessionID()` on `DBPaperEngine`:
```go
// SessionID returns the session UUID for this engine instance.
func (e *DBPaperEngine) SessionID() uuid.UUID {
    return e.sessionID
}
```

- [ ] **A4: Run test, confirm it passes**

```bash
go test -v -tags=integration -run TestUserFlow_BuyThenSellWithSession ./tests/e2e/... 2>&1
```
Expected: PASS

- [ ] **A5: Also add `TestUserFlow_SellWithoutSession_Fails`** — verify that a SELL without session_id on a new session correctly returns an error (not a silent no-op)

```go
func TestUserFlow_SellWithoutSession_Fails(t *testing.T) {
    // setup ... (same testcontainers boilerplate)
    // POST SELL without session_id when no positions exist
    sellBody := `{"action":"SELL","market_id":"ghost-market","side":"YES","price":0.8,"shares":40}`
    req := httptest.NewRequestWithContext(t.Context(), "POST", "/polymarket/trade", bytes.NewBufferString(sellBody))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusBadRequest, w.Code, "SELL with no position should return 400")
}
```

- [ ] **A6: Commit**

```bash
git add internal/api/polymarket.go internal/polymarket/paper/db_engine.go tests/e2e/polymarket_user_flow_test.go
git commit -m "fix: return session_id from ExecuteTrade and add UserFlow E2E tests"
```

---

## Task B — Fix B2: Session-scoped balance in NewDBPaperEngineWithSession

**Files:**
- Modify: `internal/db/polymarket.go`
- Modify: `internal/polymarket/paper/db_engine.go:54`
- Test: `tests/e2e/polymarket_user_flow_test.go` (add subtest)

- [ ] **B1: Write failing test — two sessions, verify second session's balance is independent**

Add to `tests/e2e/polymarket_user_flow_test.go`:

```go
func TestUserFlow_SessionBalanceIsolation(t *testing.T) {
    testhelpers.RequireDocker(t)
    tc := testhelpers.SetupTestDatabase(t)
    require.NoError(t, tc.ApplyMigrations("../../migrations"))
    database := tc.DB

    // Session 1: buy $60 of positions
    engine1, err := paper.NewDBPaperEngine(database)
    require.NoError(t, err)
    _, err = engine1.Buy("market-A", "Q?", paper.YES, 60.0, 0.6)
    require.NoError(t, err)
    assert.InDelta(t, 40.0, engine1.GetBalance(), 0.01, "engine1: 100-60=40")

    // Session 2: brand new engine, recreate from session ID
    engine2, err := paper.NewDBPaperEngine(database)
    require.NoError(t, err)
    _, err = engine2.Buy("market-B", "Q2?", paper.YES, 30.0, 0.3)
    require.NoError(t, err)
    assert.InDelta(t, 70.0, engine2.GetBalance(), 0.01, "engine2: 100-30=70")

    // Reconstruct engine1 from its session ID — balance MUST be 40 (not 40-30=10 due to global summary)
    engine1Reloaded, err := paper.NewDBPaperEngineWithSession(database, engine1.SessionID())
    require.NoError(t, err)
    assert.InDelta(t, 40.0, engine1Reloaded.GetBalance(), 0.01,
        "reloaded engine1 should still show $40, not polluted by engine2's positions")
}
```

- [ ] **B2: Run test, confirm balance is wrong (shows $10 instead of $40)**

```bash
go test -v -tags=integration -run TestUserFlow_SessionBalanceIsolation ./tests/e2e/... 2>&1
```
Expected: FAIL — `engine1Reloaded.GetBalance()` returns 10.0 (100 - 60 - 30 = 10) instead of 40.0

- [ ] **B3: Add `GetPolymarketPortfolioSummaryBySession` to `internal/db/polymarket.go`**

```go
// PolymarketSessionSummary is the session-scoped version of PolymarketPortfolioSummary.
type PolymarketSessionSummary struct {
    PositionCount int
    TotalCostBasis float64
}

// GetPolymarketPortfolioSummaryBySession returns portfolio summary for a single session.
func (db *DB) GetPolymarketPortfolioSummaryBySession(ctx context.Context, sessionID uuid.UUID) (*PolymarketSessionSummary, error) {
    query := `
        SELECT
            COUNT(*) as position_count,
            COALESCE(SUM(cost_basis), 0) as total_cost_basis
        FROM polymarket_positions
        WHERE session_id = $1 AND status = 'OPEN'
    `
    row := db.pool.QueryRow(ctx, query, sessionID)
    var s PolymarketSessionSummary
    if err := row.Scan(&s.PositionCount, &s.TotalCostBasis); err != nil {
        return nil, fmt.Errorf("GetPolymarketPortfolioSummaryBySession: %w", err)
    }
    return &s, nil
}
```

- [ ] **B4: Fix `NewDBPaperEngineWithSession` to use session-scoped summary**

In `internal/polymarket/paper/db_engine.go`, replace:
```go
summary, err := database.GetPolymarketPortfolioSummary(ctx)
if err != nil {
    return nil, fmt.Errorf("failed to get portfolio summary: %w", err)
}
balance := session.InitialCapital - summary.TotalCostBasis
```

With:
```go
summary, err := database.GetPolymarketPortfolioSummaryBySession(ctx, sessionID)
if err != nil {
    return nil, fmt.Errorf("failed to get session portfolio summary: %w", err)
}
balance := session.InitialCapital - summary.TotalCostBasis
```

- [ ] **B5: Run test, confirm it passes**

```bash
go test -v -tags=integration -run TestUserFlow_SessionBalanceIsolation ./tests/e2e/... 2>&1
```
Expected: PASS

- [ ] **B6: Run all integration tests to catch regressions**

```bash
go test -v -tags=integration ./tests/integration/... ./tests/e2e/... 2>&1 | grep -E "PASS|FAIL|---"
```
Expected: All PASS

- [ ] **B7: Commit**

```bash
git add internal/db/polymarket.go internal/polymarket/paper/db_engine.go tests/e2e/polymarket_user_flow_test.go
git commit -m "fix: session-scoped balance in NewDBPaperEngineWithSession"
```

---

## Task C — Agent Step E2E with Mock MCP and LLM

**Files:**
- Create: `tests/e2e/polymarket_agent_step_test.go`

This test exercises the agent's `Step()` method end-to-end with:
- Embedded NATS server (same pattern as orchestrator integration test)
- Mock Polymarket MCP server (HTTP mock)
- Mock Bifrost LLM (HTTP mock returning structured analysis)
- In-memory paper engine

- [ ] **C1: Write the agent step test**

File: `tests/e2e/polymarket_agent_step_test.go`

```go
//go:build integration

package e2e

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    natsserver "github.com/nats-io/nats-server/v2/server"
    "github.com/nats-io/nats.go"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/rs/zerolog"

    "github.com/ajitpratap0/cryptofunk/internal/agents"
    "github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

func TestPolymarketAgent_StepGeneratesSignal(t *testing.T) {
    testhelpers.RequireDocker(t)
    tc := testhelpers.SetupTestDatabase(t)
    require.NoError(t, tc.ApplyMigrations("../../migrations"))

    // 1. Start embedded NATS
    ns, err := startEmbeddedNATS(t)
    require.NoError(t, err)
    defer ns.Shutdown()

    nc, err := nats.Connect(ns.ClientURL())
    require.NoError(t, err)
    defer nc.Close()

    signalChan := make(chan []byte, 10)
    _, err = nc.Subscribe("cryptofunk.agent.signals", func(msg *nats.Msg) {
        signalChan <- msg.Data
    })
    require.NoError(t, err)

    // 2. Mock Polymarket MCP/API server
    mockMarkets := []map[string]any{
        {
            "condition_id": "cond-btc-100k",
            "question":     "Will BTC exceed $100k by Q2 2026?",
            "active":       true,
            "yes_price":    0.55,
            "no_price":     0.45,
            "volume":       2500000.0,
            "end_date_iso": time.Now().Add(72 * time.Hour).Format(time.RFC3339),
            "tokens": []map[string]any{
                {"token_id": "yes-tok", "outcome": "YES", "price": 0.55},
                {"token_id": "no-tok", "outcome": "NO", "price": 0.45},
            },
        },
    }
    mockMarketsJSON, _ := json.Marshal(mockMarkets)

    mockMCPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write(mockMarketsJSON)
    }))
    defer mockMCPServer.Close()

    // 3. Mock Bifrost LLM server (returns structured analysis)
    llmResponse := map[string]any{
        "choices": []map[string]any{
            {"message": map[string]any{
                "content": `{"prediction":"YES","confidence":0.78,"fair_price":0.62,"reasoning":"Strong bullish momentum"}`,
            }},
        },
    }
    llmJSON, _ := json.Marshal(llmResponse)
    mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write(llmJSON)
    }))
    defer mockLLM.Close()

    // 4. Configure agent via viper overrides
    t.Setenv("CRYPTOFUNK_COMMUNICATION_NATS_URL", ns.ClientURL())
    t.Setenv("CRYPTOFUNK_STRATEGY_AGENTS_POLYMARKET_MAX_POSITION_SIZE", "10")
    t.Setenv("CRYPTOFUNK_STRATEGY_AGENTS_POLYMARKET_CONFIDENCE_THRESHOLD", "0.7")
    t.Setenv("CRYPTOFUNK_STRATEGY_AGENTS_POLYMARKET_MIN_EDGE", "0.05")

    // 5. Create and initialize agent (with paper engine, no execute mode)
    logger := zerolog.Nop()
    agentConfig := &agents.Config{
        Name:   "polymarket-test-agent",
        NATSUrl: ns.ClientURL(),
    }
    agent, err := NewPolymarketAgentForTest(agentConfig, tc.DB, mockMCPServer.URL, mockLLM.URL, logger)
    require.NoError(t, err)

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    require.NoError(t, agent.Initialize(ctx))

    // 6. Run one Step
    require.NoError(t, agent.Step(ctx))

    // 7. Verify a signal was published to NATS
    select {
    case sigData := <-signalChan:
        var sig map[string]any
        require.NoError(t, json.Unmarshal(sigData, &sig))
        assert.Equal(t, "polymarket-test-agent", sig["agent_name"])
        tradeSignal, ok := sig["trade_signal"].(map[string]any)
        require.True(t, ok, "signal should contain trade_signal")
        assert.Contains(t, []string{"BUY_YES", "BUY_NO"}, tradeSignal["signal"],
            "signal should be BUY_YES or BUY_NO given 7% edge")
        t.Logf("Signal published: %+v", tradeSignal)
    case <-time.After(10 * time.Second):
        t.Fatal("No signal published within 10 seconds")
    }
}

// startEmbeddedNATS starts an embedded NATS server for tests.
func startEmbeddedNATS(t *testing.T) (*natsserver.Server, error) {
    t.Helper()
    opts := &natsserver.Options{
        Host:           "127.0.0.1",
        Port:           -1, // random port
        NoLog:          true,
        NoSigs:         true,
        MaxControlLine: 4096,
    }
    ns, err := natsserver.NewServer(opts)
    if err != nil {
        return nil, err
    }
    go ns.Start()
    if !ns.ReadyForConnections(5 * time.Second) {
        return nil, fmt.Errorf("NATS server not ready")
    }
    t.Cleanup(ns.Shutdown)
    return ns, nil
}
```

> **Note:** `NewPolymarketAgentForTest` is a test-helper constructor that wires the agent with mock server URLs (instead of reading from viper config). It will need to be created in a `_test.go` helper file or within the agent package as a test export.

- [ ] **C2: Run test to see what's missing/failing**

```bash
go test -v -tags=integration -run TestPolymarketAgent_StepGeneratesSignal ./tests/e2e/... 2>&1
```

Expected: FAIL — `NewPolymarketAgentForTest` does not exist yet

- [ ] **C3: Add test helper constructor to agent package**

Create `cmd/agents/polymarket-agent/agent_test_export_test.go` (or use build tags):

```go
//go:build integration
// +build integration

package main

import (
    "github.com/ajitpratap0/cryptofunk/internal/db"
    "github.com/rs/zerolog"
)

// NewPolymarketAgentForTest constructs a PolymarketAgent wired to mock servers.
func NewPolymarketAgentForTest(db *db.DB, polymarketURL, llmURL string, log zerolog.Logger) (*PolymarketAgent, error) {
    a := &PolymarketAgent{
        beliefs:             NewBeliefBase(),
        positions:           make(map[string]*Position),
        maxPositionSize:     10.0,
        maxTotalExposure:    50.0,
        confidenceThreshold: 0.70,
        minEdge:             0.05,
        profitTarget:        0.15,
        stopLoss:            0.10,
        minLiquidity:        1000.0,
        // polymarketAPIURL and llmURL would be stored and used in fetchMarkets/analyzeMarket
    }
    return a, nil
}
```

> **Note:** If the agent tightly couples to viper config inside `analyzeMarket` / `fetchMarkets`, you may need to inject mock HTTP clients instead. Investigate during implementation — use `t.Setenv` to override viper env vars as a simpler approach.

- [ ] **C4: Fix compilation errors and re-run until test passes**

```bash
go test -v -tags=integration -run TestPolymarketAgent_StepGeneratesSignal ./tests/e2e/... 2>&1
```
Expected: PASS

- [ ] **C5: Commit**

```bash
git add tests/e2e/polymarket_agent_step_test.go cmd/agents/polymarket-agent/
git commit -m "test: agent Step E2E with mock MCP and LLM"
```

---

## Task D — Performance Endpoint + Closed-Position P&L Validation

**Files:**
- Test: `tests/e2e/polymarket_user_flow_test.go` (add subtests)

- [ ] **D1: Write test for GetPerformance endpoint accuracy**

```go
func TestUserFlow_PerformanceEndpoint(t *testing.T) {
    testhelpers.RequireDocker(t)
    tc := testhelpers.SetupTestDatabase(t)
    require.NoError(t, tc.ApplyMigrations("../../migrations"))
    database := tc.DB
    // ... router setup ...

    // Create two sessions: one profitable, one unprofitable
    engine, err := paper.NewDBPaperEngine(database)
    require.NoError(t, err)

    // Buy 200 shares at 0.5 ($100 cost)
    buyTrade, err := engine.Buy("perf-market", "Q?", paper.YES, 100.0, 0.5)
    require.NoError(t, err)
    require.InDelta(t, 200.0, buyTrade.Shares, 0.01)

    // Sell all 200 shares at 0.8 (proceeds = $160)
    _, err = engine.Sell("perf-market", paper.YES, 200.0, 0.8)
    require.NoError(t, err)

    // GET /polymarket/performance
    req := httptest.NewRequestWithContext(t.Context(), "GET", "/polymarket/performance", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    require.Equal(t, http.StatusOK, w.Code, "performance: %s", w.Body)

    var perfResp map[string]any
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &perfResp))

    // Verify P&L = $160 - $100 = $60
    totalPnl, _ := perfResp["total_realized_pnl"].(float64)
    assert.InDelta(t, 60.0, totalPnl, 0.01, "total realized PnL should be $60")

    winRate, _ := perfResp["win_rate"].(float64)
    assert.InDelta(t, 1.0, winRate, 0.01, "all closed positions were profitable → win rate 100%")
}
```

- [ ] **D2: Run to confirm passes or reveal P&L calculation bugs**

```bash
go test -v -tags=integration -run TestUserFlow_PerformanceEndpoint ./tests/e2e/... 2>&1
```

- [ ] **D3: Fix any P&L calculation bugs found in GetPerformance handler**

Look at `internal/api/polymarket.go:GetPerformance` — it likely queries `polymarket_positions WHERE status = 'CLOSED'` and sums `realized_pnl`. Ensure the query matches what `ClosePolymarketPosition` stores.

- [ ] **D4: Commit**

```bash
git add internal/api/polymarket.go tests/e2e/polymarket_user_flow_test.go
git commit -m "test: performance endpoint E2E + P&L accuracy validation"
```

---

## Task E — Run Full Suite + Lint + PR

- [ ] **E1: Run all integration tests**

```bash
go test -v -race -tags=integration ./tests/integration/... ./tests/e2e/... 2>&1 | grep -E "PASS|FAIL|---"
```
Expected: All PASS

- [ ] **E2: Lint**

```bash
golangci-lint run --new-from-rev origin/main ./... 2>&1
```
Expected: 0 issues

- [ ] **E3: Create PR**

```bash
git push origin HEAD
gh pr create --title "test: Polymarket E2E user testing + session bugs" \
  --body "$(cat <<'EOF'
## Summary
- Adds true E2E user-flow tests (BUY → portfolio → SELL with session reuse)
- Fixes B1: Returns `session_id` from ExecuteTrade so clients can reuse sessions
- Fixes B2: Session-scoped balance in `NewDBPaperEngineWithSession`
- Adds agent Step E2E test with mock MCP + mock LLM
- Validates performance endpoint P&L accuracy

## Test plan
- [ ] All integration tests pass (`go test -v -race -tags=integration ./tests/...`)
- [ ] BUY → SELL user flow works with session continuity
- [ ] Session balance isolation verified
- [ ] Agent generates signal when edge > 5%

🤖 Generated with Claude Code
EOF
)"
```

---

## Execution Order (for parallel agents)

Tasks A and B are independent and can run in parallel.
Task C is independent from A and B.
Task D depends on Task A (needs the test file to exist).
Task E depends on A, B, C, D all complete.

```
A (fix session_id response) ──┐
B (fix session balance)       ├──→ E (full suite + PR)
C (agent step E2E)           ─┘
D (performance endpoint) ─ after A ──┘
```
