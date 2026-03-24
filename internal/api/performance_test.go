package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// =============================================================================
// Mock
// =============================================================================

// mockPerformanceRepo implements PerformanceRepository for unit tests.
type mockPerformanceRepo struct {
	pairRows []db.PairPerformance
	pairErr  error
	sessions []*db.TradingSession
	sessErr  error
}

func (m *mockPerformanceRepo) GetPairPerformance(_ context.Context) ([]db.PairPerformance, error) {
	return m.pairRows, m.pairErr
}

func (m *mockPerformanceRepo) ListActiveSessions(_ context.Context) ([]*db.TradingSession, error) {
	return m.sessions, m.sessErr
}

// setupPerformanceRouter builds a minimal Gin router wired to a PerformanceHandler
// backed by the supplied mock repository.
func setupPerformanceRouter(repo PerformanceRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &PerformanceHandler{db: repo}
	v1 := router.Group("/api/v1")
	// Register without auth for unit tests.
	h.RegisterRoutes(v1, gin.HandlerFunc(func(c *gin.Context) { c.Next() }), nil)
	return router
}

// =============================================================================
// TestGetSummaryHandler
// =============================================================================

func TestGetSummaryHandler(t *testing.T) {
	t.Run("empty sessions list returns 200 with all zeros", func(t *testing.T) {
		router := setupPerformanceRouter(&mockPerformanceRepo{
			sessions: []*db.TradingSession{},
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/performance/summary", nil)
		require.NoError(t, err)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

		assert.Equal(t, float64(0), body["total_pnl"])
		assert.Equal(t, float64(0), body["total_trades"])
		assert.Equal(t, float64(0), body["winning_trades"])
		assert.Equal(t, float64(0), body["win_rate"])
		assert.Equal(t, float64(0), body["return_percent"])
		assert.Equal(t, float64(0), body["session_count"])
	})

	t.Run("single session with trades aggregates correctly", func(t *testing.T) {
		session := &db.TradingSession{
			InitialCapital: 10000.0,
			TotalPnL:       500.0,
			TotalTrades:    10,
			WinningTrades:  7,
			LosingTrades:   3,
		}
		router := setupPerformanceRouter(&mockPerformanceRepo{
			sessions: []*db.TradingSession{session},
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/performance/summary", nil)
		require.NoError(t, err)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

		assert.Equal(t, float64(500), body["total_pnl"])
		assert.Equal(t, float64(10), body["total_trades"])
		assert.Equal(t, float64(7), body["winning_trades"])
		assert.Equal(t, float64(3), body["losing_trades"])
		assert.Equal(t, float64(1), body["session_count"])
		assert.Equal(t, "active_sessions_only", body["scope"])

		// win_rate: 7/10 * 100 = 70.0
		assert.InDelta(t, 70.0, body["win_rate"], 0.01)

		// return_percent: 500 / 10000 * 100 = 5.0
		assert.InDelta(t, 5.0, body["return_percent"], 0.01)
	})

	t.Run("zero total initial capital does not divide by zero", func(t *testing.T) {
		// Session with zero InitialCapital — return_percent must remain 0.
		session := &db.TradingSession{
			InitialCapital: 0,
			TotalPnL:       200.0,
			TotalTrades:    4,
			WinningTrades:  2,
		}
		router := setupPerformanceRouter(&mockPerformanceRepo{
			sessions: []*db.TradingSession{session},
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/performance/summary", nil)
		require.NoError(t, err)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

		// return_percent must be 0 (no division by zero)
		assert.Equal(t, float64(0), body["return_percent"])
		// other fields still populated correctly
		assert.Equal(t, float64(200), body["total_pnl"])
		assert.Equal(t, float64(4), body["total_trades"])
	})

	t.Run("multiple sessions aggregate totals", func(t *testing.T) {
		router := setupPerformanceRouter(&mockPerformanceRepo{
			sessions: []*db.TradingSession{
				{InitialCapital: 10000, TotalPnL: 200, TotalTrades: 5, WinningTrades: 3, LosingTrades: 2},
				{InitialCapital: 5000, TotalPnL: -50, TotalTrades: 3, WinningTrades: 1, LosingTrades: 2},
			},
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/performance/summary", nil)
		require.NoError(t, err)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

		// 200 + (-50) = 150
		assert.InDelta(t, 150.0, body["total_pnl"], 0.01)
		assert.Equal(t, float64(2), body["session_count"])
		assert.Equal(t, float64(8), body["total_trades"])   // 5 + 3
		assert.Equal(t, float64(4), body["winning_trades"]) // 3 + 1
		assert.Equal(t, float64(4), body["losing_trades"])  // 2 + 2
		// win_rate: 4/8 * 100 = 50%
		assert.InDelta(t, 50.0, body["win_rate"], 0.01)
		// return_percent uses sum(InitialCapital)=15000 (10000+5000)
		// 150 / 15000 * 100 = 1.0%
		assert.InDelta(t, 1.0, body["return_percent"], 0.01)
		// capital_basis must reflect the sum of all session InitialCapital values
		assert.Equal(t, float64(15000), body["capital_basis"])
	})

	t.Run("database error returns 500", func(t *testing.T) {
		router := setupPerformanceRouter(&mockPerformanceRepo{
			sessErr: errors.New("db unavailable"),
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/performance/summary", nil)
		require.NoError(t, err)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
