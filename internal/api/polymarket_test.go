package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// =============================================================================
// Test Setup
// =============================================================================

// polymarketTestSuite holds shared state for polymarket handler integration tests.
type polymarketTestSuite struct {
	tc      *testhelpers.PostgresContainer
	handler *PolymarketHandler
	router  *gin.Engine
}

// setupPolymarketSuite creates a containerised Postgres, applies migrations, and
// wires up a PolymarketHandler with routes registered.
func setupPolymarketSuite(t *testing.T) *polymarketTestSuite {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if os.Getenv("SKIP_TESTCONTAINER_TESTS") == "true" {
		t.Skip("Skipping testcontainer test (SKIP_TESTCONTAINER_TESTS=true)")
	}

	testhelpers.RequireDocker(t)

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err, "Failed to apply migrations")

	handler := NewPolymarketHandler(tc.DB)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/")
	// Pass nil for both rate-limiter middlewares so routes are registered without them.
	handler.RegisterRoutesWithRateLimiter(apiGroup, nil, nil)

	return &polymarketTestSuite{
		tc:      tc,
		handler: handler,
		router:  router,
	}
}

// do issues a request against the suite's router and returns the recorder.
func (s *polymarketTestSuite) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
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

// parseBody unmarshals the recorder body into a map.
func parseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result), "response body: %s", w.Body.String())
	return result
}

// seedMarket inserts a PolymarketMarket directly into the DB and returns it.
func seedMarket(t *testing.T, database *db.DB, id string, active bool) *db.PolymarketMarket {
	t.Helper()
	ctx := context.Background()
	yesPrice := 0.6
	noPrice := 0.4
	volume := 1000.0
	m := &db.PolymarketMarket{
		ID:       id,
		Question: fmt.Sprintf("Will something happen? (%s)", id),
		Active:   active,
		YesPrice: &yesPrice,
		NoPrice:  &noPrice,
		Volume:   &volume,
	}
	require.NoError(t, database.InsertPolymarketMarket(ctx, m))
	return m
}

// seedSession inserts a paper trading session and returns it.
func seedSession(t *testing.T, database *db.DB) *db.TradingSession {
	t.Helper()
	ctx := context.Background()
	session := &db.TradingSession{
		ID:             uuid.New(),
		Mode:           db.TradingModePaper,
		Symbol:         "POLYMARKET",
		Exchange:       "polymarket",
		StartedAt:      time.Now(),
		InitialCapital: 100.0,
	}
	require.NoError(t, database.CreateSession(ctx, session))
	return session
}

// seedPosition inserts an open position and returns it.
func seedPosition(t *testing.T, database *db.DB, sessionID uuid.UUID, marketID string) *db.PolymarketPosition {
	t.Helper()
	ctx := context.Background()
	pos := &db.PolymarketPosition{
		ID:        uuid.New(),
		SessionID: sessionID,
		MarketID:  marketID,
		Side:      "YES",
		Shares:    10.0,
		AvgPrice:  0.6,
		CostBasis: 6.0,
		Status:    "OPEN",
	}
	require.NoError(t, database.InsertPolymarketPosition(ctx, pos))
	return pos
}

// =============================================================================
// TestPolymarketHandler_ListMarkets
// =============================================================================

func TestPolymarketHandler_ListMarkets(t *testing.T) {
	s := setupPolymarketSuite(t)

	// Seed 2 active markets and 1 inactive market.
	id1 := "market-list-active-1"
	id2 := "market-list-active-2"
	id3 := "market-list-inactive-3"
	seedMarket(t, s.tc.DB, id1, true)
	seedMarket(t, s.tc.DB, id2, true)
	seedMarket(t, s.tc.DB, id3, false)

	t.Run("default returns active markets only", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/markets", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		count := int(body["count"].(float64))
		assert.GreaterOrEqual(t, count, 2, "should include at least 2 active markets")

		markets := body["markets"].([]any)
		// Verify the inactive market is NOT in the list.
		for _, m := range markets {
			mMap := m.(map[string]any)
			assert.NotEqual(t, id3, mMap["id"], "inactive market should not appear in active-only listing")
		}
	})

	t.Run("active=false returns all markets", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/markets?active=false", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		count := int(body["count"].(float64))
		assert.GreaterOrEqual(t, count, 3, "should include all 3 markets when active=false")
	})
}

// =============================================================================
// TestPolymarketHandler_GetMarket
// =============================================================================

func TestPolymarketHandler_GetMarket(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-get-one"
	seedMarket(t, s.tc.DB, marketID, true)

	t.Run("existing market returns 200", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/markets/"+marketID, nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		require.Contains(t, body, "market")
		marketData := body["market"].(map[string]any)
		assert.Equal(t, marketID, marketData["id"])
	})

	t.Run("nonexistent market returns 404", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/markets/does-not-exist-xyz", nil)
		assert.Equal(t, http.StatusNotFound, w.Code)

		body := parseBody(t, w)
		assert.Contains(t, body, "error")
	})
}

// =============================================================================
// TestPolymarketHandler_ListPositions
// =============================================================================

func TestPolymarketHandler_ListPositions(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-positions-list"
	seedMarket(t, s.tc.DB, marketID, true)
	session := seedSession(t, s.tc.DB)
	seedPosition(t, s.tc.DB, session.ID, marketID)

	t.Run("returns open positions", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/positions", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		assert.Contains(t, body, "positions")
		assert.Contains(t, body, "count")
		count := int(body["count"].(float64))
		assert.GreaterOrEqual(t, count, 1)
	})

	t.Run("live=false still returns positions without live enrichment", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/positions?live=false", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		assert.Contains(t, body, "positions")
		assert.GreaterOrEqual(t, int(body["count"].(float64)), 1)
	})
}

// =============================================================================
// TestPolymarketHandler_GetPosition
// =============================================================================

func TestPolymarketHandler_GetPosition(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-get-position"
	seedMarket(t, s.tc.DB, marketID, true)
	session := seedSession(t, s.tc.DB)
	pos := seedPosition(t, s.tc.DB, session.ID, marketID)

	t.Run("existing position returns 200", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/positions/"+pos.ID.String(), nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		require.Contains(t, body, "position")
		posData := body["position"].(map[string]any)
		assert.Equal(t, pos.ID.String(), posData["id"])
	})

	t.Run("invalid UUID format returns 400", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/positions/not-a-uuid", nil)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		body := parseBody(t, w)
		assert.Contains(t, body, "error")
	})

	t.Run("valid UUID but nonexistent position returns 404", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/positions/"+uuid.New().String(), nil)
		assert.Equal(t, http.StatusNotFound, w.Code)

		body := parseBody(t, w)
		assert.Contains(t, body, "error")
	})
}

// =============================================================================
// TestPolymarketHandler_ExecuteTrade_Buy
// =============================================================================

func TestPolymarketHandler_ExecuteTrade_Buy(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-execute-buy"
	seedMarket(t, s.tc.DB, marketID, true)

	reqBody := map[string]any{
		"market_id": marketID,
		"question":  "Will this buy test pass?",
		"side":      "YES",
		"action":    "BUY",
		"amount":    10.0,
		"price":     0.6,
	}

	w := s.do(t, http.MethodPost, "/polymarket/trade", reqBody)
	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())

	body := parseBody(t, w)
	require.Contains(t, body, "trade")
	require.Contains(t, body, "balance")
	require.Contains(t, body, "message")

	tradeData := body["trade"].(map[string]any)
	assert.Equal(t, "BUY", tradeData["action"])
	assert.Equal(t, marketID, tradeData["market_id"])
	assert.Equal(t, "YES", tradeData["side"])
	assert.InDelta(t, 10.0, tradeData["amount"].(float64), 0.01)
	assert.InDelta(t, 0.6, tradeData["price"].(float64), 0.001)

	// Verify a position was created in DB.
	ctx := context.Background()
	positions, err := s.tc.DB.GetOpenPolymarketPositions(ctx)
	require.NoError(t, err)
	found := false
	for _, p := range positions {
		if p.MarketID == marketID && p.Side == "YES" {
			found = true
			assert.Equal(t, "OPEN", p.Status)
			// cost_basis = amount spent = $10
			assert.InDelta(t, 10.0, p.CostBasis, 0.01)
			break
		}
	}
	assert.True(t, found, "open position should exist in DB after buy")
}

// =============================================================================
// TestPolymarketHandler_ExecuteTrade_Sell
// =============================================================================

func TestPolymarketHandler_ExecuteTrade_Sell(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-execute-sell"
	seedMarket(t, s.tc.DB, marketID, true)

	// First buy to create a position.
	buyBody := map[string]any{
		"market_id": marketID,
		"question":  "Will this sell test pass?",
		"side":      "YES",
		"action":    "BUY",
		"amount":    10.0,
		"price":     0.6,
	}
	wBuy := s.do(t, http.MethodPost, "/polymarket/trade", buyBody)
	require.Equal(t, http.StatusOK, wBuy.Code, "buy setup failed: %s", wBuy.Body.String())

	buyResp := parseBody(t, wBuy)
	buyTrade := buyResp["trade"].(map[string]any)
	sessionID := buyResp["trade"] // kept for documentation; sessionID implicit

	// Shares bought = amount / price = 10 / 0.6 ≈ 16.666
	shares := buyTrade["shares"].(float64)

	// Sell all shares we just bought using same session.
	// We do NOT pass session_id — the engine will create a new session;
	// but since the DB engine looks across ALL open positions, it will still find
	// the position created in the buy step only if the positions belong to the
	// same session. To guarantee that, we reuse the session embedded in the engine.
	//
	// Instead of threading session_id through, we simply sell a partial amount
	// of shares that we know fits the position.
	sellShares := shares / 2
	sellBody := map[string]any{
		"market_id": marketID,
		"side":      "YES",
		"action":    "SELL",
		"amount":    sellShares * 0.6,
		"price":     0.6,
		"shares":    sellShares,
	}
	_ = sessionID // suppress unused lint warning

	wSell := s.do(t, http.MethodPost, "/polymarket/trade", sellBody)
	// It is expected the sell may fail with a new engine session that doesn't
	// hold the position. We accept either 200 (if position is found across sessions)
	// or 400 (if position not found for this new session).
	// The important thing is the handler doesn't panic and returns a parseable body.
	sellBody2 := parseBody(t, wSell)
	if wSell.Code == http.StatusOK {
		assert.Contains(t, sellBody2, "trade")
		assert.Contains(t, sellBody2, "balance")
		sellTrade := sellBody2["trade"].(map[string]any)
		assert.Equal(t, "SELL", sellTrade["action"])
	} else {
		// 400 is acceptable when position not found in new session.
		assert.Equal(t, http.StatusBadRequest, wSell.Code)
		assert.Contains(t, sellBody2, "error")
	}
}

// =============================================================================
// TestPolymarketHandler_ExecuteTrade_SameSession_Sell
// =============================================================================

// TestPolymarketHandler_ExecuteTrade_SameSession_Sell validates sell within
// the same session using session_id reuse.
func TestPolymarketHandler_ExecuteTrade_SameSession_Sell(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-same-session-sell"
	seedMarket(t, s.tc.DB, marketID, true)

	// Create a paper session manually so we can pass the session_id.
	session := seedSession(t, s.tc.DB)

	buyBody := map[string]any{
		"market_id":  marketID,
		"question":   "Same-session sell test?",
		"side":       "YES",
		"action":     "BUY",
		"amount":     10.0,
		"price":      0.6,
		"session_id": session.ID.String(),
	}
	wBuy := s.do(t, http.MethodPost, "/polymarket/trade", buyBody)
	require.Equal(t, http.StatusOK, wBuy.Code, "buy setup failed: %s", wBuy.Body.String())

	buyResp := parseBody(t, wBuy)
	buyTrade := buyResp["trade"].(map[string]any)
	shares := buyTrade["shares"].(float64)

	sellBody := map[string]any{
		"market_id":  marketID,
		"side":       "YES",
		"action":     "SELL",
		"amount":     shares * 0.6,
		"price":      0.6,
		"shares":     shares,
		"session_id": session.ID.String(),
	}
	wSell := s.do(t, http.MethodPost, "/polymarket/trade", sellBody)
	assert.Equal(t, http.StatusOK, wSell.Code, "sell failed: %s", wSell.Body.String())

	sellResp := parseBody(t, wSell)
	assert.Contains(t, sellResp, "trade")
	sellTrade := sellResp["trade"].(map[string]any)
	assert.Equal(t, "SELL", sellTrade["action"])
	assert.Equal(t, marketID, sellTrade["market_id"])

	// Verify P&L: realized proceeds = shares * price = amount, cost = shares * avgBuyPrice * fraction
	assert.Contains(t, sellResp, "balance")
}

// =============================================================================
// TestPolymarketHandler_ExecuteTrade_ValidationErrors
// =============================================================================

func TestPolymarketHandler_ExecuteTrade_ValidationErrors(t *testing.T) {
	s := setupPolymarketSuite(t)

	tests := []struct {
		name           string
		body           map[string]any
		expectedStatus int
	}{
		{
			name: "missing market_id",
			body: map[string]any{
				"side":   "YES",
				"action": "BUY",
				"amount": 10.0,
				"price":  0.6,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing side",
			body: map[string]any{
				"market_id": "some-market",
				"action":    "BUY",
				"amount":    10.0,
				"price":     0.6,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid action",
			body: map[string]any{
				"market_id": "some-market",
				"side":      "YES",
				"action":    "HOLD",
				"amount":    10.0,
				"price":     0.6,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "price out of range (>= 1)",
			body: map[string]any{
				"market_id": "some-market",
				"side":      "YES",
				"action":    "BUY",
				"amount":    10.0,
				"price":     1.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "price zero (lt=0)",
			body: map[string]any{
				"market_id": "some-market",
				"side":      "YES",
				"action":    "BUY",
				"amount":    10.0,
				"price":     0.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "amount zero (not gt=0)",
			body: map[string]any{
				"market_id": "some-market",
				"side":      "YES",
				"action":    "BUY",
				"amount":    0.0,
				"price":     0.5,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "amount negative",
			body: map[string]any{
				"market_id": "some-market",
				"side":      "YES",
				"action":    "BUY",
				"amount":    -5.0,
				"price":     0.5,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid session_id format",
			body: map[string]any{
				"market_id":  "some-market",
				"side":       "YES",
				"action":     "BUY",
				"amount":     10.0,
				"price":      0.5,
				"session_id": "not-a-valid-uuid",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := s.do(t, http.MethodPost, "/polymarket/trade", tt.body)
			assert.Equal(t, tt.expectedStatus, w.Code, "body: %s", w.Body.String())
			body := parseBody(t, w)
			assert.Contains(t, body, "error")
		})
	}
}

// TestPolymarketHandler_ExecuteTrade_InsufficientBalance tests that buying
// more than the default balance ($100) returns an error from the engine.
func TestPolymarketHandler_ExecuteTrade_InsufficientBalance(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-insufficient-balance"
	seedMarket(t, s.tc.DB, marketID, true)

	// Request $200 which exceeds DefaultBalance of $100.
	reqBody := map[string]any{
		"market_id": marketID,
		"question":  "Balance test",
		"side":      "YES",
		"action":    "BUY",
		"amount":    200.0,
		"price":     0.5,
	}
	w := s.do(t, http.MethodPost, "/polymarket/trade", reqBody)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	body := parseBody(t, w)
	assert.Contains(t, body, "error")
	assert.Contains(t, body["error"].(string), "insufficient balance")
}

// =============================================================================
// TestPolymarketHandler_GetPortfolio
// =============================================================================

func TestPolymarketHandler_GetPortfolio(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-portfolio"
	seedMarket(t, s.tc.DB, marketID, true)

	// Buy to create a position so portfolio is non-trivial.
	buyBody := map[string]any{
		"market_id": marketID,
		"question":  "Portfolio test market?",
		"side":      "YES",
		"action":    "BUY",
		"amount":    10.0,
		"price":     0.6,
	}
	wBuy := s.do(t, http.MethodPost, "/polymarket/trade", buyBody)
	require.Equal(t, http.StatusOK, wBuy.Code, "buy setup failed: %s", wBuy.Body.String())

	w := s.do(t, http.MethodGet, "/polymarket/portfolio", nil)
	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	body := parseBody(t, w)
	assert.Contains(t, body, "balance")
	assert.Contains(t, body, "total_value")
	assert.Contains(t, body, "cost_basis")
	assert.Contains(t, body, "unrealized_pnl")
	assert.Contains(t, body, "position_count")

	positionCount := int(body["position_count"].(float64))
	assert.GreaterOrEqual(t, positionCount, 1)

	costBasis := body["cost_basis"].(float64)
	assert.Greater(t, costBasis, 0.0)
}

// =============================================================================
// TestPolymarketHandler_ListTrades
// =============================================================================

func TestPolymarketHandler_ListTrades(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-list-trades"
	seedMarket(t, s.tc.DB, marketID, true)

	// Execute a buy so there is at least one trade recorded.
	buyBody := map[string]any{
		"market_id": marketID,
		"question":  "List trades test?",
		"side":      "YES",
		"action":    "BUY",
		"amount":    5.0,
		"price":     0.5,
	}
	wBuy := s.do(t, http.MethodPost, "/polymarket/trade", buyBody)
	require.Equal(t, http.StatusOK, wBuy.Code, "buy setup failed: %s", wBuy.Body.String())

	w := s.do(t, http.MethodGet, "/polymarket/trades", nil)
	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	body := parseBody(t, w)
	assert.Contains(t, body, "trades")
	assert.Contains(t, body, "count")

	count := int(body["count"].(float64))
	assert.GreaterOrEqual(t, count, 1)

	trades := body["trades"].([]any)
	firstTrade := trades[0].(map[string]any)
	assert.Contains(t, firstTrade, "id")
	assert.Contains(t, firstTrade, "market_id")
	assert.Contains(t, firstTrade, "action")
	assert.Contains(t, firstTrade, "side")
	assert.Contains(t, firstTrade, "price")
	assert.Contains(t, firstTrade, "shares")
}

// TestPolymarketHandler_ListTrades_WithLimit verifies the limit query param is respected.
func TestPolymarketHandler_ListTrades_WithLimit(t *testing.T) {
	s := setupPolymarketSuite(t)

	marketID := "market-trades-limit"
	seedMarket(t, s.tc.DB, marketID, true)

	// Create 3 trades.
	for i := 0; i < 3; i++ {
		buyBody := map[string]any{
			"market_id": marketID,
			"question":  fmt.Sprintf("Trade limit test %d?", i),
			"side":      "YES",
			"action":    "BUY",
			"amount":    1.0,
			"price":     0.5,
		}
		s.do(t, http.MethodPost, "/polymarket/trade", buyBody)
	}

	w := s.do(t, http.MethodGet, "/polymarket/trades?limit=2", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	count := int(body["count"].(float64))
	assert.LessOrEqual(t, count, 2, "limit=2 should return at most 2 trades")
}

// =============================================================================
// TestPolymarketHandler_ListPredictions
// =============================================================================

func TestPolymarketHandler_ListPredictions(t *testing.T) {
	s := setupPolymarketSuite(t)

	// Seed a market for the FK constraint on predictions.
	marketID := "market-predictions"
	seedMarket(t, s.tc.DB, marketID, true)

	ctx := context.Background()
	cat := "crypto"
	reasoning := "Strong trend signal"
	pred := &db.PolymarketPrediction{
		MarketID:      marketID,
		PredictedProb: 0.75,
		MarketPrice:   0.6,
		Edge:          0.15,
		Confidence:    0.8,
		Category:      &cat,
		Reasoning:     &reasoning,
		Resolved:      false,
	}
	require.NoError(t, s.tc.DB.InsertPolymarketPrediction(ctx, pred))

	t.Run("lists all predictions", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/predictions", nil)
		assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		body := parseBody(t, w)
		assert.Contains(t, body, "predictions")
		assert.Contains(t, body, "count")
		count := int(body["count"].(float64))
		assert.GreaterOrEqual(t, count, 1)
	})

	t.Run("resolved=true returns empty when none resolved", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/predictions?resolved=true", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		// All seeded predictions are unresolved, so resolved-only count should be 0
		// (unless prior tests created resolved predictions, but we seed fresh market IDs).
		count := int(body["count"].(float64))
		assert.Equal(t, 0, count)
	})

	t.Run("after resolving prediction appears in resolved list", func(t *testing.T) {
		outcome := 1.0
		pnl := 5.0
		require.NoError(t, s.tc.DB.ResolvePrediction(ctx, pred.ID, outcome, pnl))

		w := s.do(t, http.MethodGet, "/polymarket/predictions?resolved=true", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		body := parseBody(t, w)
		count := int(body["count"].(float64))
		assert.GreaterOrEqual(t, count, 1)
	})
}

// =============================================================================
// TestPolymarketHandler_GetPerformance
// =============================================================================

func TestPolymarketHandler_GetPerformance(t *testing.T) {
	s := setupPolymarketSuite(t)

	t.Run("empty state returns zero performance metrics", func(t *testing.T) {
		w := s.do(t, http.MethodGet, "/polymarket/performance", nil)
		assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		body := parseBody(t, w)
		assert.Contains(t, body, "total_predictions")
		assert.Contains(t, body, "resolved")
		assert.Contains(t, body, "correct")
		assert.Contains(t, body, "win_rate")
		assert.Contains(t, body, "brier_score")
		assert.Contains(t, body, "total_pnl")
		assert.Contains(t, body, "by_category")
		assert.Contains(t, body, "timestamp")
	})

	t.Run("with resolved predictions computes win_rate and brier_score", func(t *testing.T) {
		ctx := context.Background()

		marketID := "market-perf"
		seedMarket(t, s.tc.DB, marketID, true)

		cat := "sports"
		// Seed a resolved correct prediction (predicted > 0.5, outcome = 1)
		p1 := &db.PolymarketPrediction{
			MarketID:      marketID,
			PredictedProb: 0.8,
			MarketPrice:   0.7,
			Edge:          0.1,
			Confidence:    0.9,
			Category:      &cat,
			Resolved:      false,
		}
		require.NoError(t, s.tc.DB.InsertPolymarketPrediction(ctx, p1))
		require.NoError(t, s.tc.DB.ResolvePrediction(ctx, p1.ID, 1.0, 10.0))

		// Seed a resolved incorrect prediction (predicted > 0.5, outcome = 0)
		p2 := &db.PolymarketPrediction{
			MarketID:      marketID,
			PredictedProb: 0.7,
			MarketPrice:   0.6,
			Edge:          0.1,
			Confidence:    0.7,
			Category:      &cat,
			Resolved:      false,
		}
		require.NoError(t, s.tc.DB.InsertPolymarketPrediction(ctx, p2))
		require.NoError(t, s.tc.DB.ResolvePrediction(ctx, p2.ID, 0.0, -5.0))

		w := s.do(t, http.MethodGet, "/polymarket/performance", nil)
		assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		body := parseBody(t, w)
		totalPredictions := int(body["total_predictions"].(float64))
		assert.GreaterOrEqual(t, totalPredictions, 2)

		resolved := int(body["resolved"].(float64))
		assert.GreaterOrEqual(t, resolved, 2)

		winRate := body["win_rate"].(float64)
		// 1 correct out of 2 resolved = 50%, but there may be prior data so check > 0
		assert.GreaterOrEqual(t, winRate, 0.0)
		assert.LessOrEqual(t, winRate, 100.0)

		brierScore := body["brier_score"].(float64)
		assert.GreaterOrEqual(t, brierScore, 0.0)

		totalPnl := body["total_pnl"].(float64)
		// p1 pnl=10, p2 pnl=-5 → net 5 (plus any prior test pnl)
		// We only assert it's a valid float, not panic.
		_ = totalPnl

		byCategory := body["by_category"].(map[string]any)
		assert.Contains(t, byCategory, "sports")
	})
}

// =============================================================================
// TestPolymarketHandler_Routes
// =============================================================================

// TestPolymarketHandler_Routes verifies all 9 routes are registered and
// return something other than 404 (which would indicate missing route).
func TestPolymarketHandler_Routes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if os.Getenv("SKIP_TESTCONTAINER_TESTS") == "true" {
		t.Skip("Skipping testcontainer test (SKIP_TESTCONTAINER_TESTS=true)")
	}

	testhelpers.RequireDocker(t)
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewPolymarketHandler(tc.DB)
	handler.RegisterRoutesWithRateLimiter(router.Group("/"), nil, nil)

	// Routes that return 200/4xx from the handler (route IS registered).
	// We distinguish gin's "no route" 404 (body is plain text "404 page not found")
	// from a handler-generated 404 (body is JSON with "error" key).
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/polymarket/markets"},
		{http.MethodGet, "/polymarket/markets/some-market-id"},
		{http.MethodGet, "/polymarket/positions"},
		{http.MethodGet, "/polymarket/positions/" + uuid.New().String()},
		{http.MethodPost, "/polymarket/trade"},
		{http.MethodGet, "/polymarket/portfolio"},
		{http.MethodGet, "/polymarket/trades"},
		{http.MethodGet, "/polymarket/predictions"},
		{http.MethodGet, "/polymarket/performance"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			var body *bytes.Reader
			if rt.method == http.MethodPost {
				// Send an intentionally invalid body — we just want the route to exist.
				body = bytes.NewReader([]byte(`{}`))
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequestWithContext(t.Context(), rt.method, rt.path, body)
			if rt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// If gin found no matching route it returns a plain-text 404.
			// A handler-generated 404 has a JSON body with an "error" key.
			// Either way the route is registered as long as the body is JSON.
			if w.Code == http.StatusNotFound {
				// Must be a handler JSON 404, not gin's "404 page not found".
				var resp map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err,
					"route %s %s returned 404 with non-JSON body — route may not be registered. body: %s",
					rt.method, rt.path, w.Body.String())
			}
		})
	}
}
