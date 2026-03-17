//go:build integration

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/api"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/paper"
)

// =============================================================================
// E2E Test Setup
// =============================================================================

// polymarketE2ESuite holds shared state for polymarket user-flow E2E tests.
type polymarketE2ESuite struct {
	tc      *testhelpers.PostgresContainer
	handler *api.PolymarketHandler
	router  *gin.Engine
}

// setupPolymarketE2ESuite creates a containerised Postgres, applies migrations,
// and wires up a PolymarketHandler with routes registered (no rate limiting).
func setupPolymarketE2ESuite(t *testing.T) *polymarketE2ESuite {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	if os.Getenv("SKIP_TESTCONTAINER_TESTS") == "true" {
		t.Skip("Skipping testcontainer test (SKIP_TESTCONTAINER_TESTS=true)")
	}

	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err, "failed to apply migrations")

	handler := api.NewPolymarketHandler(tc.DB)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/")
	// Pass nil for both rate-limiter middlewares — routes are registered without them.
	handler.RegisterRoutesWithRateLimiter(apiGroup, nil, nil)

	return &polymarketE2ESuite{
		tc:      tc,
		handler: handler,
		router:  router,
	}
}

// do issues a request against the suite's router and returns the recorder.
func (s *polymarketE2ESuite) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

// parseE2EBody unmarshals the recorder body into a map.
func parseE2EBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result), "response body: %s", w.Body.String())
	return result
}

// =============================================================================
// Test 1: TestUserFlow_BuyThenSellWithSessionContinuity
// =============================================================================

// TestUserFlow_BuyThenSellWithSessionContinuity validates a full buy→portfolio→sell
// round-trip. The BUY response now includes session_id which is passed to the SELL
// request so both trades share the same paper-engine session.
func TestUserFlow_BuyThenSellWithSessionContinuity(t *testing.T) {
	s := setupPolymarketE2ESuite(t)

	// Step 1: BUY $20 at 0.5 on "flow-market" YES — no session_id provided
	buyReq := map[string]any{
		"action":    "BUY",
		"market_id": "flow-market",
		"question":  "Will X happen?",
		"side":      "YES",
		"amount":    20.0,
		"price":     0.5,
	}
	wBuy := s.do(t, http.MethodPost, "/polymarket/trade", buyReq)
	require.Equal(t, http.StatusOK, wBuy.Code, "BUY response: %s", wBuy.Body.String())

	// Step 2: Assert balance ≈ 80 and session_id is a non-empty UUID string
	buyBody := parseE2EBody(t, wBuy)
	require.Contains(t, buyBody, "balance", "BUY response missing 'balance'")
	require.Contains(t, buyBody, "trade", "BUY response missing 'trade'")
	require.Contains(t, buyBody, "session_id", "BUY response must include session_id so the client can reuse it for SELL")

	balance := buyBody["balance"].(float64)
	assert.InDelta(t, 80.0, balance, 0.01, "balance after $20 BUY should be ≈ $80")

	sessionID, _ := buyBody["session_id"].(string)
	require.NotEmpty(t, sessionID, "session_id must be a non-empty UUID string")

	tradeData := buyBody["trade"].(map[string]any)
	assert.Equal(t, "BUY", tradeData["action"])
	assert.Equal(t, "flow-market", tradeData["market_id"])
	assert.InDelta(t, 20.0, tradeData["amount"].(float64), 0.01)
	assert.InDelta(t, 0.5, tradeData["price"].(float64), 0.001)
	shares := tradeData["shares"].(float64)
	assert.InDelta(t, 40.0, shares, 0.01, "shares = amount/price = 20/0.5 = 40")

	// Step 3: GET /polymarket/portfolio — position_count ≥ 1
	wPortfolio := s.do(t, http.MethodGet, "/polymarket/portfolio", nil)
	require.Equal(t, http.StatusOK, wPortfolio.Code, "portfolio response: %s", wPortfolio.Body.String())

	portfolioBody := parseE2EBody(t, wPortfolio)
	require.Contains(t, portfolioBody, "position_count")
	positionCount := int(portfolioBody["position_count"].(float64))
	assert.GreaterOrEqual(t, positionCount, 1, "portfolio should have at least 1 open position after BUY")

	// Step 4: SELL 40 shares at 0.8 — pass session_id from BUY so same session is reused
	// proceeds = 40 * 0.8 = $32 → balance = 80 + 32 = $112
	sellReq := map[string]any{
		"action":     "SELL",
		"market_id":  "flow-market",
		"side":       "YES",
		"price":      0.8,
		"amount":     32.0, // required by validator (gt=0); shares field takes precedence
		"shares":     40.0,
		"session_id": sessionID,
	}
	wSell := s.do(t, http.MethodPost, "/polymarket/trade", sellReq)
	require.Equal(t, http.StatusOK, wSell.Code, "SELL should succeed when session_id reused: %s", wSell.Body.String())

	// Step 5: Verify balance after sell
	sellBody := parseE2EBody(t, wSell)
	require.Contains(t, sellBody, "balance")
	balanceAfterSell := sellBody["balance"].(float64)
	assert.InDelta(t, 112.0, balanceAfterSell, 0.01, "balance after SELL: $80 + 40*$0.8 = $112")
}

// =============================================================================
// Test 2: TestUserFlow_SellWithoutPosition_Returns400
// =============================================================================

// TestUserFlow_SellWithoutPosition_Returns400 verifies that attempting to sell
// a position that does not exist returns HTTP 400.
func TestUserFlow_SellWithoutPosition_Returns400(t *testing.T) {
	s := setupPolymarketE2ESuite(t)

	sellReq := map[string]any{
		"action":    "SELL",
		"market_id": "ghost-market",
		"side":      "YES",
		"amount":    8.0, // amount > 0 required by validation; shares derived from amount/price
		"price":     0.8,
		"shares":    10.0,
	}
	w := s.do(t, http.MethodPost, "/polymarket/trade", sellReq)
	assert.Equal(t, http.StatusBadRequest, w.Code, "SELL on non-existent position should return 400; body: %s", w.Body.String())

	body := parseE2EBody(t, w)
	assert.Contains(t, body, "error")
}

// =============================================================================
// Test 3: TestUserFlow_SessionBalanceIsolation
// =============================================================================

// TestUserFlow_SessionBalanceIsolation confirms that two concurrent paper-engine
// sessions maintain independent balances, and that reconstructing an engine from
// a session ID restores the correct per-session balance (not polluted by other sessions).
// NewDBPaperEngineWithSession uses GetPolymarketPortfolioSummaryBySession which is
// scoped to the requested session only.
func TestUserFlow_SessionBalanceIsolation(t *testing.T) {
	database := func() *testhelpers.PostgresContainer {
		if testing.Short() {
			t.Skip("Skipping E2E test in short mode")
		}
		if os.Getenv("SKIP_TESTCONTAINER_TESTS") == "true" {
			t.Skip("Skipping testcontainer test (SKIP_TESTCONTAINER_TESTS=true)")
		}
		testhelpers.RequireDocker(t)
		tc := testhelpers.SetupTestDatabase(t)
		err := tc.ApplyMigrations("../../migrations")
		require.NoError(t, err)
		return tc
	}()

	// engine1: buy $60 at 0.6 on "iso-market" YES
	engine1, err := paper.NewDBPaperEngine(database.DB)
	require.NoError(t, err)

	_, err = engine1.Buy("iso-market", "Isolation test market?", paper.YES, 60.0, 0.6)
	require.NoError(t, err)
	assert.InDelta(t, 40.0, engine1.GetBalance(), 0.01, "engine1 balance after $60 BUY should be ≈ $40")

	session1ID := engine1.SessionID()

	// engine2: new session — buy $30 at 0.3 on "iso-market2" YES
	engine2, err := paper.NewDBPaperEngine(database.DB)
	require.NoError(t, err)

	_, err = engine2.Buy("iso-market2", "Isolation test market 2?", paper.YES, 30.0, 0.3)
	require.NoError(t, err)
	assert.InDelta(t, 70.0, engine2.GetBalance(), 0.01, "engine2 balance after $30 BUY should be ≈ $70")

	// Verify engine balances are independent of each other.
	assert.NotEqual(t, engine1.GetBalance(), engine2.GetBalance(),
		"engine1 and engine2 should have different balances")

	// Reconstruct engine1 from its session ID: balance must be session-scoped (≈ $40),
	// not $10 (which would be 100 - 60 - 30 = global across both sessions).
	engine1r, err := paper.NewDBPaperEngineWithSession(database.DB, session1ID)
	require.NoError(t, err)

	assert.InDelta(t, 40.0, engine1r.GetBalance(), 0.01,
		"reconstructed engine1 balance should be ≈ $40 (session-scoped), not polluted by engine2's $30 cost basis")

	portfolio1r := engine1r.GetPortfolio()
	require.NotNil(t, portfolio1r)

	// The reconstructed portfolio's positions must be session1's only.
	pos, ok := portfolio1r.Positions["iso-market:YES"]
	require.True(t, ok, "reconstructed engine1 portfolio should contain iso-market:YES position")
	assert.InDelta(t, 60.0/0.6, pos.Shares, 0.01, "iso-market:YES should have ≈ 100 shares")

	// engine2's position must NOT appear in the reconstructed engine1 portfolio.
	_, hasEngine2Pos := portfolio1r.Positions["iso-market2:YES"]
	assert.False(t, hasEngine2Pos, "engine2's position must not appear in engine1's reconstructed portfolio")
}

// =============================================================================
// Test 4: TestUserFlow_PerformanceEndpoint
// =============================================================================

// TestUserFlow_PerformanceEndpoint verifies the /polymarket/performance endpoint
// returns valid JSON with the expected field names.
//
// Note: GetPerformance aggregates PolymarketPredictions (not paper trades). It
// calculates win_rate as a percentage (0–100) and total_pnl from resolved
// predictions. Paper trade P&L is NOT reflected here unless predictions are
// explicitly resolved via ResolvePrediction. This test validates the endpoint
// shape and that it handles an empty-predictions state gracefully.
func TestUserFlow_PerformanceEndpoint(t *testing.T) {
	s := setupPolymarketE2ESuite(t)

	// GET /polymarket/performance with no predictions seeded — should return zeros.
	w := s.do(t, http.MethodGet, "/polymarket/performance", nil)
	require.Equal(t, http.StatusOK, w.Code, "performance response: %s", w.Body.String())

	body := parseE2EBody(t, w)

	// Verify all expected fields are present.
	assert.Contains(t, body, "total_predictions", "performance response missing 'total_predictions'")
	assert.Contains(t, body, "resolved", "performance response missing 'resolved'")
	assert.Contains(t, body, "correct", "performance response missing 'correct'")
	assert.Contains(t, body, "win_rate", "performance response missing 'win_rate'")
	assert.Contains(t, body, "brier_score", "performance response missing 'brier_score'")
	assert.Contains(t, body, "total_pnl", "performance response missing 'total_pnl'")
	assert.Contains(t, body, "by_category", "performance response missing 'by_category'")
	assert.Contains(t, body, "timestamp", "performance response missing 'timestamp'")

	// With no predictions: win_rate and total_pnl should be zero.
	winRate := body["win_rate"].(float64)
	assert.InDelta(t, 0.0, winRate, 0.001, "win_rate should be 0 with no resolved predictions")

	totalPnl := body["total_pnl"].(float64)
	assert.InDelta(t, 0.0, totalPnl, 0.001, "total_pnl should be 0 with no resolved predictions")
}
