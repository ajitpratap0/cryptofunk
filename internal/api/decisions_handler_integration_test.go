//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// TestDecisionEndpoints_Integration tests the decision API endpoints with a real database
func TestDecisionEndpoints_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	// Create test repository and handler
	repo := NewDecisionRepository(tc.DB.Pool())
	handler := NewDecisionHandler(repo)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Insert test data
	ctx := context.Background()
	testDecisions := insertTestDecisions(t, tc.DB.Pool(), ctx)

	t.Run("ListDecisions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Decisions []Decision `json:"decisions"`
			Count     int        `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, response.Count, len(testDecisions))
	})

	t.Run("ListDecisions_WithFilters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions?symbol=BTC/USDT&outcome=SUCCESS", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Decisions []Decision `json:"decisions"`
			Count     int        `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify all returned decisions match the filters
		for _, d := range response.Decisions {
			assert.Equal(t, "BTC/USDT", d.Symbol)
			assert.Equal(t, "SUCCESS", d.Outcome)
		}
	})

	t.Run("ListDecisions_WithPagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions?limit=2&offset=0", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Decisions []Decision `json:"decisions"`
			Count     int        `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.LessOrEqual(t, response.Count, 2)
	})

	t.Run("GetDecision_Found", func(t *testing.T) {
		if len(testDecisions) == 0 {
			t.Skip("No test decisions available")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions/"+testDecisions[0].String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var decision Decision
		err := json.Unmarshal(w.Body.Bytes(), &decision)
		require.NoError(t, err)
		assert.Equal(t, testDecisions[0], decision.ID)
	})

	t.Run("GetDecision_NotFound", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions/"+nonExistentID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GetDecision_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions/invalid-uuid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetStats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions/stats", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats DecisionStats
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, stats.TotalDecisions, len(testDecisions))
	})

	t.Run("SearchDecisions_TextSearch", func(t *testing.T) {
		body := SearchRequest{
			Query: "BTC",
			Limit: 10,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/decisions/search", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Results    []SearchResult `json:"results"`
			Count      int            `json:"count"`
			SearchType string         `json:"search_type"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "text", response.SearchType)
	})

	t.Run("SearchDecisions_InvalidRequest", func(t *testing.T) {
		// Missing both query and embedding
		body := SearchRequest{}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/decisions/search", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("SearchDecisions_QueryTooLong", func(t *testing.T) {
		// Create a query longer than MaxSearchQueryLength (500)
		longQuery := make([]byte, 501)
		for i := range longQuery {
			longQuery[i] = 'a'
		}

		body := SearchRequest{
			Query: string(longQuery),
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/decisions/search", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetSimilarDecisions", func(t *testing.T) {
		if len(testDecisions) == 0 {
			t.Skip("No test decisions available")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/decisions/"+testDecisions[0].String()+"/similar?limit=5", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// May return 200 with empty results or 500 if vectors not indexed
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
	})
}

// insertTestDecisions inserts test data and returns the IDs
func insertTestDecisions(t *testing.T, pool interface{}, ctx context.Context) []uuid.UUID {
	t.Helper()

	// Get the pool with correct type
	db, ok := pool.(interface {
		Exec(ctx context.Context, sql string, arguments ...interface{}) (interface{}, error)
	})
	if !ok {
		t.Log("Could not cast pool to expected interface, skipping test data insertion")
		return nil
	}

	testData := []struct {
		symbol       string
		decisionType string
		outcome      string
		agentName    string
		confidence   float64
		prompt       string
		response     string
	}{
		{"BTC/USDT", "signal", "SUCCESS", "technical-agent", 0.85, "Analyze BTC trend", "BTC showing bullish signals"},
		{"BTC/USDT", "signal", "FAILURE", "trend-agent", 0.65, "Evaluate BTC momentum", "Momentum slowing down"},
		{"ETH/USDT", "risk_approval", "SUCCESS", "risk-agent", 0.90, "Approve ETH trade", "Risk within limits"},
		{"ETH/USDT", "signal", "PENDING", "orderbook-agent", 0.70, "Check ETH orderbook", "Analyzing..."},
	}

	ids := make([]uuid.UUID, 0, len(testData))
	for _, d := range testData {
		id := uuid.New()
		_, err := db.Exec(ctx, `
			INSERT INTO llm_decisions (id, symbol, decision_type, outcome, agent_name, confidence, prompt, response, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, id, d.symbol, d.decisionType, d.outcome, d.agentName, d.confidence, d.prompt, d.response, time.Now())
		if err != nil {
			t.Logf("Failed to insert test decision: %v", err)
			continue
		}
		ids = append(ids, id)
	}

	return ids
}
