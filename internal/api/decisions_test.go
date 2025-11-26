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
)

// TestDecisionRepository_ListDecisions tests the list decisions functionality
func TestDecisionRepository_ListDecisions(t *testing.T) {
	// Skip if no database available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires a real database connection
	// In a real test environment, you would use testcontainers or a test database
	t.Skip("Integration test - requires database setup")

	// Example of how this test would work with a database:
	/*
		ctx := context.Background()
		pool, err := pgxpool.New(ctx, "postgres://postgres:postgres@localhost:5432/cryptofunk_test")
		require.NoError(t, err)
		defer pool.Close()

		repo := NewDecisionRepository(pool)

		// Test with empty filter
		filter := DecisionFilter{Limit: 10}
		decisions, err := repo.ListDecisions(ctx, filter)
		assert.NoError(t, err)
		assert.NotNil(t, decisions)

		// Test with symbol filter
		filter = DecisionFilter{Symbol: "BTC/USDT", Limit: 10}
		decisions, err = repo.ListDecisions(ctx, filter)
		assert.NoError(t, err)
	*/
}

// TestDecisionRepository_GetDecision tests getting a single decision
func TestDecisionRepository_GetDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup")
}

// TestDecisionRepository_GetDecisionStats tests statistics aggregation
func TestDecisionRepository_GetDecisionStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup")
}

// TestDecisionFilter validates filter struct
func TestDecisionFilter(t *testing.T) {
	now := time.Now()
	filter := DecisionFilter{
		Symbol:       "BTC/USDT",
		DecisionType: "signal",
		Outcome:      "SUCCESS",
		Model:        "claude-sonnet-4",
		FromDate:     &now,
		ToDate:       &now,
		Limit:        50,
		Offset:       0,
	}

	assert.Equal(t, "BTC/USDT", filter.Symbol)
	assert.Equal(t, "signal", filter.DecisionType)
	assert.Equal(t, "SUCCESS", filter.Outcome)
	assert.Equal(t, "claude-sonnet-4", filter.Model)
	assert.Equal(t, 50, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
	assert.NotNil(t, filter.FromDate)
	assert.NotNil(t, filter.ToDate)
}

// TestDecision validates Decision struct
func TestDecision(t *testing.T) {
	id := uuid.New()
	sessionID := uuid.New()
	tokens := 100
	latency := 150
	confidence := 0.85
	outcome := "SUCCESS"
	pnl := 250.50

	decision := Decision{
		ID:           id,
		SessionID:    &sessionID,
		DecisionType: "signal",
		Symbol:       "ETH/USDT",
		Prompt:       "Analyze ETH/USDT",
		Response:     `{"side": "BUY", "confidence": 0.85}`,
		Model:        "gpt-4",
		TokensUsed:   &tokens,
		LatencyMs:    &latency,
		Confidence:   &confidence,
		Outcome:      &outcome,
		PnL:          &pnl,
		CreatedAt:    time.Now(),
	}

	assert.Equal(t, id, decision.ID)
	assert.Equal(t, &sessionID, decision.SessionID)
	assert.Equal(t, "signal", decision.DecisionType)
	assert.Equal(t, "ETH/USDT", decision.Symbol)
	assert.Equal(t, 100, *decision.TokensUsed)
	assert.Equal(t, 150, *decision.LatencyMs)
	assert.Equal(t, 0.85, *decision.Confidence)
	assert.Equal(t, "SUCCESS", *decision.Outcome)
	assert.Equal(t, 250.50, *decision.PnL)
}

// TestDecisionStats validates DecisionStats struct
func TestDecisionStats(t *testing.T) {
	stats := DecisionStats{
		TotalDecisions: 100,
		ByType: map[string]int{
			"signal":        60,
			"risk_approval": 40,
		},
		ByOutcome: map[string]int{
			"SUCCESS": 70,
			"FAILURE": 30,
		},
		ByModel: map[string]int{
			"claude-sonnet-4": 50,
			"gpt-4":           30,
			"gpt-3.5-turbo":   20,
		},
		AvgConfidence: 0.75,
		AvgLatencyMs:  120.5,
		AvgTokensUsed: 150.0,
		SuccessRate:   0.70,
		TotalPnL:      5000.00,
		AvgPnL:        71.43,
	}

	assert.Equal(t, 100, stats.TotalDecisions)
	assert.Equal(t, 60, stats.ByType["signal"])
	assert.Equal(t, 70, stats.ByOutcome["SUCCESS"])
	assert.Equal(t, 50, stats.ByModel["claude-sonnet-4"])
	assert.Equal(t, 0.75, stats.AvgConfidence)
	assert.Equal(t, 0.70, stats.SuccessRate)
}

// Benchmark DecisionRepository operations (requires database)
func BenchmarkDecisionRepository_ListDecisions(b *testing.B) {
	b.Skip("Benchmark - requires database setup")

	/*
		ctx := context.Background()
		pool, _ := pgxpool.New(ctx, "postgres://postgres:postgres@localhost:5432/cryptofunk_test")
		defer pool.Close()

		repo := NewDecisionRepository(pool)
		filter := DecisionFilter{Limit: 100}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = repo.ListDecisions(ctx, filter)
		}
	*/
}

// MockDecisionRepository for unit testing handlers without database
type MockDecisionRepository struct {
	ListDecisionsFunc        func(ctx context.Context, filter DecisionFilter) ([]Decision, error)
	GetDecisionFunc          func(ctx context.Context, id uuid.UUID) (*Decision, error)
	GetDecisionStatsFunc     func(ctx context.Context, filter DecisionFilter) (*DecisionStats, error)
	FindSimilarDecisionsFunc func(ctx context.Context, id uuid.UUID, limit int) ([]Decision, error)
	SearchDecisionsFunc      func(ctx context.Context, req SearchRequest) ([]SearchResult, error)
}

func (m *MockDecisionRepository) ListDecisions(ctx context.Context, filter DecisionFilter) ([]Decision, error) {
	if m.ListDecisionsFunc != nil {
		return m.ListDecisionsFunc(ctx, filter)
	}
	return []Decision{}, nil
}

func (m *MockDecisionRepository) GetDecision(ctx context.Context, id uuid.UUID) (*Decision, error) {
	if m.GetDecisionFunc != nil {
		return m.GetDecisionFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockDecisionRepository) GetDecisionStats(ctx context.Context, filter DecisionFilter) (*DecisionStats, error) {
	if m.GetDecisionStatsFunc != nil {
		return m.GetDecisionStatsFunc(ctx, filter)
	}
	return &DecisionStats{}, nil
}

func (m *MockDecisionRepository) FindSimilarDecisions(ctx context.Context, id uuid.UUID, limit int) ([]Decision, error) {
	if m.FindSimilarDecisionsFunc != nil {
		return m.FindSimilarDecisionsFunc(ctx, id, limit)
	}
	return []Decision{}, nil
}

func (m *MockDecisionRepository) SearchDecisions(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	if m.SearchDecisionsFunc != nil {
		return m.SearchDecisionsFunc(ctx, req)
	}
	return []SearchResult{}, nil
}

// Example of how to use MockDecisionRepository in tests
func TestMockRepository(t *testing.T) {
	mock := &MockDecisionRepository{
		ListDecisionsFunc: func(ctx context.Context, filter DecisionFilter) ([]Decision, error) {
			return []Decision{
				{
					ID:           uuid.New(),
					DecisionType: "signal",
					Symbol:       "BTC/USDT",
					Model:        "claude-sonnet-4",
				},
			}, nil
		},
	}

	ctx := context.Background()
	decisions, err := mock.ListDecisions(ctx, DecisionFilter{})

	require.NoError(t, err)
	assert.Len(t, decisions, 1)
	assert.Equal(t, "signal", decisions[0].DecisionType)
	assert.Equal(t, "BTC/USDT", decisions[0].Symbol)
}

// =============================================================================
// Handler Tests with Mock Repository
// =============================================================================

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestRouter creates a router with the decision handler using a mock repository
func setupTestRouter(mock *MockDecisionRepository) *gin.Engine {
	router := gin.New()
	handler := &DecisionHandler{repo: mock}
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)
	return router
}

// TestHandlerListDecisions tests the list decisions endpoint
func TestHandlerListDecisions(t *testing.T) {
	testDecisions := []Decision{
		{
			ID:           uuid.New(),
			DecisionType: "signal",
			Symbol:       "BTC/USDT",
			Model:        "claude-sonnet-4",
			Prompt:       "Analyze BTC/USDT",
			Response:     `{"action": "BUY"}`,
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			DecisionType: "risk_approval",
			Symbol:       "ETH/USDT",
			Model:        "gpt-4",
			Prompt:       "Evaluate risk",
			Response:     `{"approved": true}`,
			CreatedAt:    time.Now(),
		},
	}

	mock := &MockDecisionRepository{
		ListDecisionsFunc: func(ctx context.Context, filter DecisionFilter) ([]Decision, error) {
			// Filter by symbol if provided
			if filter.Symbol != "" {
				filtered := make([]Decision, 0)
				for _, d := range testDecisions {
					if d.Symbol == filter.Symbol {
						filtered = append(filtered, d)
					}
				}
				return filtered, nil
			}
			return testDecisions, nil
		},
	}

	router := setupTestRouter(mock)

	t.Run("list all decisions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		decisions := response["decisions"].([]interface{})
		assert.Len(t, decisions, 2)
		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("list decisions with symbol filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions?symbol=BTC/USDT", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		decisions := response["decisions"].([]interface{})
		assert.Len(t, decisions, 1)
	})

	t.Run("list decisions with pagination", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions?limit=1&offset=0", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify filter is returned
		filter := response["filter"].(map[string]interface{})
		assert.Equal(t, float64(1), filter["Limit"])
	})
}

// TestHandlerGetDecision tests getting a single decision
func TestHandlerGetDecision(t *testing.T) {
	testID := uuid.New()
	confidence := 0.85
	testDecision := &Decision{
		ID:           testID,
		DecisionType: "signal",
		Symbol:       "BTC/USDT",
		Model:        "claude-sonnet-4",
		Confidence:   &confidence,
		Prompt:       "Test prompt",
		Response:     "Test response",
		CreatedAt:    time.Now(),
	}

	mock := &MockDecisionRepository{
		GetDecisionFunc: func(ctx context.Context, id uuid.UUID) (*Decision, error) {
			if id == testID {
				return testDecision, nil
			}
			return nil, nil // Not found
		},
	}

	router := setupTestRouter(mock)

	t.Run("get existing decision", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/"+testID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var decision Decision
		err := json.Unmarshal(w.Body.Bytes(), &decision)
		require.NoError(t, err)

		assert.Equal(t, testID, decision.ID)
		assert.Equal(t, "BTC/USDT", decision.Symbol)
		assert.Equal(t, 0.85, *decision.Confidence)
	})

	t.Run("get non-existent decision", func(t *testing.T) {
		w := httptest.NewRecorder()
		nonExistentID := uuid.New()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/"+nonExistentID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("get decision with invalid ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/invalid-uuid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestHandlerGetStats tests the statistics endpoint
func TestHandlerGetStats(t *testing.T) {
	mock := &MockDecisionRepository{
		GetDecisionStatsFunc: func(ctx context.Context, filter DecisionFilter) (*DecisionStats, error) {
			return &DecisionStats{
				TotalDecisions: 100,
				ByType: map[string]int{
					"signal":        60,
					"risk_approval": 40,
				},
				ByOutcome: map[string]int{
					"SUCCESS": 70,
					"FAILURE": 30,
				},
				ByModel: map[string]int{
					"claude-sonnet-4": 50,
					"gpt-4":           50,
				},
				AvgConfidence: 0.75,
				AvgLatencyMs:  120.5,
				AvgTokensUsed: 150,
				SuccessRate:   0.70,
				TotalPnL:      5000.0,
				AvgPnL:        71.43,
			}, nil
		},
	}

	router := setupTestRouter(mock)

	t.Run("get stats without filters", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats DecisionStats
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)

		assert.Equal(t, 100, stats.TotalDecisions)
		assert.Equal(t, 0.75, stats.AvgConfidence)
		assert.Equal(t, 0.70, stats.SuccessRate)
		assert.Equal(t, 60, stats.ByType["signal"])
	})

	t.Run("get stats with symbol filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/stats?symbol=BTC/USDT", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestHandlerSearchDecisions tests the search endpoint
func TestHandlerSearchDecisions(t *testing.T) {
	mock := &MockDecisionRepository{}
	router := setupTestRouter(mock)

	t.Run("search with text query", func(t *testing.T) {
		body := SearchRequest{
			Query: "buy signal BTC",
			Limit: 10,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "text", response["search_type"])
	})

	t.Run("search with missing query and embedding", func(t *testing.T) {
		body := SearchRequest{
			Limit: 10,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("search with invalid embedding dimension", func(t *testing.T) {
		body := SearchRequest{
			Embedding: make([]float32, 100), // Wrong dimension
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Invalid embedding dimension")
	})

	t.Run("search with invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/search", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("search with query too long", func(t *testing.T) {
		// Create a query longer than MaxSearchQueryLength (500)
		longQuery := make([]byte, 501)
		for i := range longQuery {
			longQuery[i] = 'a'
		}
		body := SearchRequest{
			Query: string(longQuery),
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Query too long")
	})
}

// TestHandlerSimilarDecisions tests the similar decisions endpoint
func TestHandlerSimilarDecisions(t *testing.T) {
	testID := uuid.New()

	mock := &MockDecisionRepository{
		FindSimilarDecisionsFunc: func(ctx context.Context, id uuid.UUID, limit int) ([]Decision, error) {
			if id == testID {
				return []Decision{
					{
						ID:           uuid.New(),
						DecisionType: "signal",
						Symbol:       "BTC/USDT",
						Model:        "claude-sonnet-4",
					},
					{
						ID:           uuid.New(),
						DecisionType: "signal",
						Symbol:       "ETH/USDT",
						Model:        "gpt-4",
					},
				}, nil
			}
			return []Decision{}, nil
		},
	}

	router := setupTestRouter(mock)

	t.Run("get similar decisions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/"+testID.String()+"/similar", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		similar := response["similar"].([]interface{})
		assert.Len(t, similar, 2)
		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("get similar with limit", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/"+testID.String()+"/similar?limit=5", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestSearchRequest validates SearchRequest struct
func TestSearchRequest(t *testing.T) {
	now := time.Now()
	req := SearchRequest{
		Query:     "buy signal",
		Embedding: make([]float32, 1536),
		Symbol:    "BTC/USDT",
		FromDate:  &now,
		ToDate:    &now,
		Limit:     20,
	}

	assert.Equal(t, "buy signal", req.Query)
	assert.Len(t, req.Embedding, 1536)
	assert.Equal(t, "BTC/USDT", req.Symbol)
	assert.Equal(t, 20, req.Limit)
}

// TestSearchResult validates SearchResult struct
func TestSearchResult(t *testing.T) {
	result := SearchResult{
		Decision: Decision{
			ID:     uuid.New(),
			Symbol: "BTC/USDT",
		},
		Score: 0.95,
	}

	assert.Equal(t, 0.95, result.Score)
	assert.Equal(t, "BTC/USDT", result.Decision.Symbol)
}
