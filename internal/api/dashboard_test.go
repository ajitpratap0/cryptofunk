package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// =============================================================================
// Mock Repository
// =============================================================================

// MockDashboardRepository implements DashboardRepositoryInterface for testing
type MockDashboardRepository struct {
	ListActiveSessionsFunc        func(ctx context.Context) ([]*db.TradingSession, error)
	GetSessionFunc                func(ctx context.Context, sessionID uuid.UUID) (*db.TradingSession, error)
	CreateSessionFunc             func(ctx context.Context, session *db.TradingSession) error
	StopSessionFunc               func(ctx context.Context, sessionID uuid.UUID, finalCapital float64) error
	GetAllOpenPositionsFunc       func(ctx context.Context) ([]*db.Position, error)
	GetClosedFeesBySessionIDsFunc func(ctx context.Context, sessionIDs []uuid.UUID) (float64, error)
	GetPositionsBySessionFunc     func(ctx context.Context, sessionID uuid.UUID) ([]*db.Position, error)
	PingFunc                      func(ctx context.Context) error
	IsTradingPausedFunc           func(ctx context.Context) (bool, error)
	GetAllAgentStatusesFunc       func(ctx context.Context) ([]*db.AgentStatus, error)
}

func (m *MockDashboardRepository) ListActiveSessions(ctx context.Context) ([]*db.TradingSession, error) {
	if m.ListActiveSessionsFunc != nil {
		return m.ListActiveSessionsFunc(ctx)
	}
	return []*db.TradingSession{}, nil
}

func (m *MockDashboardRepository) GetSession(ctx context.Context, sessionID uuid.UUID) (*db.TradingSession, error) {
	if m.GetSessionFunc != nil {
		return m.GetSessionFunc(ctx, sessionID)
	}
	return nil, errors.New("session not found")
}

func (m *MockDashboardRepository) CreateSession(ctx context.Context, session *db.TradingSession) error {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(ctx, session)
	}
	session.ID = uuid.New()
	return nil
}

func (m *MockDashboardRepository) StopSession(ctx context.Context, sessionID uuid.UUID, finalCapital float64) error {
	if m.StopSessionFunc != nil {
		return m.StopSessionFunc(ctx, sessionID, finalCapital)
	}
	return nil
}

func (m *MockDashboardRepository) GetAllOpenPositions(ctx context.Context) ([]*db.Position, error) {
	if m.GetAllOpenPositionsFunc != nil {
		return m.GetAllOpenPositionsFunc(ctx)
	}
	return []*db.Position{}, nil
}

func (m *MockDashboardRepository) GetClosedFeesBySessionIDs(ctx context.Context, sessionIDs []uuid.UUID) (float64, error) {
	if m.GetClosedFeesBySessionIDsFunc != nil {
		return m.GetClosedFeesBySessionIDsFunc(ctx, sessionIDs)
	}
	return 0, nil
}

func (m *MockDashboardRepository) GetPositionsBySession(ctx context.Context, sessionID uuid.UUID) ([]*db.Position, error) {
	if m.GetPositionsBySessionFunc != nil {
		return m.GetPositionsBySessionFunc(ctx, sessionID)
	}
	return []*db.Position{}, nil
}

func (m *MockDashboardRepository) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

func (m *MockDashboardRepository) IsTradingPaused(ctx context.Context) (bool, error) {
	if m.IsTradingPausedFunc != nil {
		return m.IsTradingPausedFunc(ctx)
	}
	return false, nil
}

func (m *MockDashboardRepository) GetAllAgentStatuses(ctx context.Context) ([]*db.AgentStatus, error) {
	if m.GetAllAgentStatusesFunc != nil {
		return m.GetAllAgentStatusesFunc(ctx)
	}
	return []*db.AgentStatus{}, nil
}

// =============================================================================
// Mock Orchestrator
// =============================================================================

// MockOrchestrator implements OrchestratorInterface for testing
type MockOrchestrator struct {
	PauseFunc               func() error
	ResumeFunc              func() error
	IsPausedFunc            func() bool
	GetActiveAgentCountFunc func() int
}

func (m *MockOrchestrator) Pause() error {
	if m.PauseFunc != nil {
		return m.PauseFunc()
	}
	return nil
}

func (m *MockOrchestrator) Resume() error {
	if m.ResumeFunc != nil {
		return m.ResumeFunc()
	}
	return nil
}

func (m *MockOrchestrator) IsPaused() bool {
	if m.IsPausedFunc != nil {
		return m.IsPausedFunc()
	}
	return false
}

func (m *MockOrchestrator) GetActiveAgentCount() int {
	if m.GetActiveAgentCountFunc != nil {
		return m.GetActiveAgentCountFunc()
	}
	return 0
}

// =============================================================================
// Test Setup
// =============================================================================

func setupDashboardTestRouter(mock *MockDashboardRepository, orch *MockOrchestrator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := NewDashboardHandler(mock)
	if orch != nil {
		handler.SetOrchestrator(orch)
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)
	return router
}

// =============================================================================
// Type Tests
// =============================================================================

func TestDashboardDataTypes(t *testing.T) {
	t.Run("DashboardData struct", func(t *testing.T) {
		sessionID := uuid.New()
		data := DashboardData{
			TradingStatus: TradingStatusInfo{
				IsActive:       true,
				IsPaused:       false,
				Mode:           "PAPER",
				ActiveSessions: 1,
				CurrentSession: &sessionID,
			},
			PositionSummary: PositionSummaryInfo{
				OpenPositions:   5,
				TotalExposure:   10000.0,
				LongPositions:   3,
				ShortPositions:  2,
				TotalUnrealized: 500.0,
			},
			PnLSummary: PnLSummaryInfo{
				TotalPnL:       1500.0,
				RealizedPnL:    1000.0,
				UnrealizedPnL:  500.0,
				TotalTrades:    20,
				WinningTrades:  15,
				LosingTrades:   5,
				WinRate:        75.0,
				MaxDrawdown:    0.05,
				InitialCapital: 10000.0,
				CurrentCapital: 11500.0,
			},
			SystemStatus: SystemStatusInfo{
				Status:       "healthy",
				Uptime:       "1h30m",
				DatabaseOK:   true,
				ActiveAgents: 5,
				Version:      "1.0.0",
			},
			Timestamp: time.Now(),
		}

		assert.True(t, data.TradingStatus.IsActive)
		assert.Equal(t, "PAPER", data.TradingStatus.Mode)
		assert.Equal(t, 5, data.PositionSummary.OpenPositions)
		assert.Equal(t, 1500.0, data.PnLSummary.TotalPnL)
		assert.Equal(t, "healthy", data.SystemStatus.Status)
	})

	t.Run("PositionDetails struct", func(t *testing.T) {
		posID := uuid.New()
		sessionID := uuid.New()
		stopLoss := 45000.0
		takeProfit := 55000.0
		unrealizedPnL := 250.0
		entryReason := "Technical breakout"

		details := PositionDetails{
			ID:            posID,
			SessionID:     &sessionID,
			Symbol:        "BTC/USDT",
			Exchange:      "binance",
			Side:          "LONG",
			EntryPrice:    50000.0,
			Quantity:      0.1,
			EntryTime:     time.Now(),
			StopLoss:      &stopLoss,
			TakeProfit:    &takeProfit,
			UnrealizedPnL: &unrealizedPnL,
			Fees:          5.0,
			EntryReason:   &entryReason,
			Duration:      "2h30m",
		}

		assert.Equal(t, posID, details.ID)
		assert.Equal(t, "BTC/USDT", details.Symbol)
		assert.Equal(t, "LONG", details.Side)
		assert.Equal(t, 50000.0, details.EntryPrice)
		assert.Equal(t, 45000.0, *details.StopLoss)
	})

	t.Run("TradingResponse struct", func(t *testing.T) {
		sessionID := uuid.New()
		response := TradingResponse{
			Success:   true,
			Message:   "Trading started successfully",
			SessionID: &sessionID,
			Action:    "start",
			Timestamp: time.Now(),
		}

		assert.True(t, response.Success)
		assert.Equal(t, "start", response.Action)
		assert.NotNil(t, response.SessionID)
	})
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestGetDashboard(t *testing.T) {
	testSessionID := uuid.New()

	mock := &MockDashboardRepository{
		ListActiveSessionsFunc: func(ctx context.Context) ([]*db.TradingSession, error) {
			return []*db.TradingSession{
				{
					ID:             testSessionID,
					Mode:           db.TradingModePaper,
					Symbol:         "BTC/USDT",
					InitialCapital: 10000.0,
					TotalPnL:       500.0,
					TotalTrades:    10,
					WinningTrades:  7,
					LosingTrades:   3,
				},
			}, nil
		},
		GetAllOpenPositionsFunc: func(ctx context.Context) ([]*db.Position, error) {
			unrealizedPnL := 100.0
			return []*db.Position{
				{
					ID:            uuid.New(),
					Symbol:        "BTC/USDT",
					Side:          db.PositionSideLong,
					EntryPrice:    50000.0,
					Quantity:      0.1,
					UnrealizedPnL: &unrealizedPnL,
				},
			}, nil
		},
		PingFunc: func(ctx context.Context) error {
			return nil
		},
		IsTradingPausedFunc: func(ctx context.Context) (bool, error) {
			return false, nil
		},
		GetAllAgentStatusesFunc: func(ctx context.Context) ([]*db.AgentStatus, error) {
			return []*db.AgentStatus{
				{Name: "technical-agent", Status: "HEALTHY"},
				{Name: "risk-agent", Status: "HEALTHY"},
			}, nil
		},
	}

	router := setupDashboardTestRouter(mock, nil)

	t.Run("get dashboard success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var dashboard DashboardData
		err := json.Unmarshal(w.Body.Bytes(), &dashboard)
		require.NoError(t, err)

		assert.True(t, dashboard.TradingStatus.IsActive)
		assert.Equal(t, 1, dashboard.TradingStatus.ActiveSessions)
		assert.Equal(t, "PAPER", dashboard.TradingStatus.Mode)
		assert.Equal(t, 1, dashboard.PositionSummary.OpenPositions)
		assert.Equal(t, "healthy", dashboard.SystemStatus.Status)
		assert.True(t, dashboard.SystemStatus.DatabaseOK)
	})

	t.Run("get dashboard with database error", func(t *testing.T) {
		failingMock := &MockDashboardRepository{
			ListActiveSessionsFunc: func(ctx context.Context) ([]*db.TradingSession, error) {
				return nil, errors.New("database error")
			},
		}

		failingRouter := setupDashboardTestRouter(failingMock, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard", nil)
		failingRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestGetPositions(t *testing.T) {
	testSessionID := uuid.New()
	unrealizedPnL := 150.0

	testPositions := []*db.Position{
		{
			ID:            uuid.New(),
			SessionID:     &testSessionID,
			Symbol:        "BTC/USDT",
			Exchange:      "binance",
			Side:          db.PositionSideLong,
			EntryPrice:    50000.0,
			Quantity:      0.1,
			EntryTime:     time.Now().Add(-2 * time.Hour),
			UnrealizedPnL: &unrealizedPnL,
			Fees:          2.5,
		},
		{
			ID:         uuid.New(),
			SessionID:  &testSessionID,
			Symbol:     "ETH/USDT",
			Exchange:   "binance",
			Side:       db.PositionSideShort,
			EntryPrice: 3000.0,
			Quantity:   1.0,
			EntryTime:  time.Now().Add(-1 * time.Hour),
			Fees:       1.5,
		},
	}

	mock := &MockDashboardRepository{
		GetAllOpenPositionsFunc: func(ctx context.Context) ([]*db.Position, error) {
			return testPositions, nil
		},
		GetPositionsBySessionFunc: func(ctx context.Context, sessionID uuid.UUID) ([]*db.Position, error) {
			if sessionID == testSessionID {
				return testPositions, nil
			}
			return []*db.Position{}, nil
		},
	}

	router := setupDashboardTestRouter(mock, nil)

	t.Run("get all positions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/positions", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		positions := response["positions"].([]interface{})
		assert.Len(t, positions, 2)
		assert.Equal(t, float64(2), response["count"])

		summary := response["summary"].(map[string]interface{})
		assert.Equal(t, float64(2), summary["open_positions"])
		assert.Equal(t, float64(1), summary["long_positions"])
		assert.Equal(t, float64(1), summary["short_positions"])
	})

	t.Run("get positions by session", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/positions?session_id="+testSessionID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("get positions with invalid session ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/positions?session_id=invalid-uuid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetPnL(t *testing.T) {
	testSessionID := uuid.New()

	closedFeesCalled := false
	mock := &MockDashboardRepository{
		ListActiveSessionsFunc: func(ctx context.Context) ([]*db.TradingSession, error) {
			return []*db.TradingSession{
				{
					ID:             testSessionID,
					Mode:           db.TradingModePaper,
					Symbol:         "BTC/USDT",
					InitialCapital: 10000.0,
					TotalPnL:       500.0,
					TotalTrades:    10,
					WinningTrades:  7,
					LosingTrades:   3,
					MaxDrawdown:    0.03,
				},
			}, nil
		},
		GetSessionFunc: func(ctx context.Context, sessionID uuid.UUID) (*db.TradingSession, error) {
			if sessionID == testSessionID {
				return &db.TradingSession{
					ID:             testSessionID,
					Mode:           db.TradingModePaper,
					Symbol:         "BTC/USDT",
					InitialCapital: 10000.0,
					TotalPnL:       500.0,
					TotalTrades:    10,
					WinningTrades:  7,
					LosingTrades:   3,
					MaxDrawdown:    0.03,
				}, nil
			}
			return nil, errors.New("session not found")
		},
		GetAllOpenPositionsFunc: func(ctx context.Context) ([]*db.Position, error) {
			unrealizedPnL := 100.0
			realizedPnL := 200.0
			return []*db.Position{
				{
					ID:            uuid.New(),
					UnrealizedPnL: &unrealizedPnL,
					RealizedPnL:   &realizedPnL,
					Fees:          5.0,
				},
			}, nil
		},
		GetClosedFeesBySessionIDsFunc: func(ctx context.Context, sessionIDs []uuid.UUID) (float64, error) {
			closedFeesCalled = true
			return 10.0, nil
		},
		GetPositionsBySessionFunc: func(ctx context.Context, sessionID uuid.UUID) ([]*db.Position, error) {
			unrealizedPnL := 100.0
			return []*db.Position{
				{
					ID:            uuid.New(),
					UnrealizedPnL: &unrealizedPnL,
					Fees:          5.0,
				},
			}, nil
		},
	}

	router := setupDashboardTestRouter(mock, nil)

	t.Run("get aggregate P&L", func(t *testing.T) {
		closedFeesCalled = false
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/pnl", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var pnl PnLSummaryInfo
		err := json.Unmarshal(w.Body.Bytes(), &pnl)
		require.NoError(t, err)

		assert.Equal(t, 500.0, pnl.TotalPnL)
		assert.Equal(t, 10, pnl.TotalTrades)
		assert.Equal(t, 7, pnl.WinningTrades)
		assert.Equal(t, 70.0, pnl.WinRate)
		assert.Equal(t, 10000.0, pnl.InitialCapital)

		// RealizedPnL must equal TotalPnL (both represent session-level realized P&L).
		assert.Equal(t, pnl.TotalPnL, pnl.RealizedPnL, "RealizedPnL should equal TotalPnL")

		// CurrentCapital must be InitialCapital + TotalPnL + UnrealizedPnL.
		assert.Equal(t, pnl.InitialCapital+pnl.TotalPnL+pnl.UnrealizedPnL, pnl.CurrentCapital,
			"CurrentCapital should equal InitialCapital + TotalPnL + UnrealizedPnL")

		// GetClosedFeesBySessionIDs must have been called to scope fees to active sessions.
		assert.True(t, closedFeesCalled, "GetClosedFeesBySessionIDs should have been called")
	})

	t.Run("get session-specific P&L", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/pnl?session_id="+testSessionID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var pnl PnLSummaryInfo
		err := json.Unmarshal(w.Body.Bytes(), &pnl)
		require.NoError(t, err)
		assert.Equal(t, 10000.0, pnl.InitialCapital)
	})

	t.Run("get P&L with invalid session ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/pnl?session_id=invalid-uuid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("get P&L for non-existent session", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/pnl?session_id="+uuid.New().String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetStatus(t *testing.T) {
	mock := &MockDashboardRepository{
		PingFunc: func(ctx context.Context) error {
			return nil
		},
		GetAllAgentStatusesFunc: func(ctx context.Context) ([]*db.AgentStatus, error) {
			return []*db.AgentStatus{
				{Name: "technical-agent", Status: "HEALTHY"},
				{Name: "risk-agent", Status: "HEALTHY"},
				{Name: "sentiment-agent", Status: "DEGRADED"},
			}, nil
		},
	}

	orch := &MockOrchestrator{
		GetActiveAgentCountFunc: func() int {
			return 3
		},
	}

	router := setupDashboardTestRouter(mock, orch)

	t.Run("get status with healthy system", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var status SystemStatusInfo
		err := json.Unmarshal(w.Body.Bytes(), &status)
		require.NoError(t, err)

		assert.Equal(t, "healthy", status.Status)
		assert.True(t, status.DatabaseOK)
		assert.Equal(t, 3, status.ActiveAgents)
	})

	t.Run("get status with database error", func(t *testing.T) {
		failingMock := &MockDashboardRepository{
			PingFunc: func(ctx context.Context) error {
				return errors.New("connection refused")
			},
			GetAllAgentStatusesFunc: func(ctx context.Context) ([]*db.AgentStatus, error) {
				return []*db.AgentStatus{}, nil
			},
		}

		failingRouter := setupDashboardTestRouter(failingMock, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/status", nil)
		failingRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var status SystemStatusInfo
		err := json.Unmarshal(w.Body.Bytes(), &status)
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", status.Status)
		assert.False(t, status.DatabaseOK)
	})
}

func TestStartTrading(t *testing.T) {
	mock := &MockDashboardRepository{
		CreateSessionFunc: func(ctx context.Context, session *db.TradingSession) error {
			session.ID = uuid.New()
			return nil
		},
	}

	router := setupDashboardTestRouter(mock, nil)

	t.Run("start trading with valid request", func(t *testing.T) {
		body := map[string]interface{}{
			"symbol":          "BTC/USDT",
			"initial_capital": 10000.0,
			"mode":            "PAPER",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/start", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response TradingResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "start", response.Action)
		assert.NotNil(t, response.SessionID)
	})

	t.Run("start trading with default mode", func(t *testing.T) {
		body := map[string]interface{}{
			"symbol":          "ETH/USDT",
			"initial_capital": 5000.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/start", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("start trading with missing symbol", func(t *testing.T) {
		body := map[string]interface{}{
			"initial_capital": 10000.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/start", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("start trading with invalid capital", func(t *testing.T) {
		body := map[string]interface{}{
			"symbol":          "BTC/USDT",
			"initial_capital": -100.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/start", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("start trading with database error", func(t *testing.T) {
		failingMock := &MockDashboardRepository{
			CreateSessionFunc: func(ctx context.Context, session *db.TradingSession) error {
				return errors.New("database error")
			},
		}

		failingRouter := setupDashboardTestRouter(failingMock, nil)

		body := map[string]interface{}{
			"symbol":          "BTC/USDT",
			"initial_capital": 10000.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/start", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		failingRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestStopTrading(t *testing.T) {
	testSessionID := uuid.New()

	mock := &MockDashboardRepository{
		GetSessionFunc: func(ctx context.Context, sessionID uuid.UUID) (*db.TradingSession, error) {
			if sessionID == testSessionID {
				return &db.TradingSession{
					ID:             testSessionID,
					Mode:           db.TradingModePaper,
					Symbol:         "BTC/USDT",
					InitialCapital: 10000.0,
				}, nil
			}
			return nil, errors.New("session not found")
		},
		StopSessionFunc: func(ctx context.Context, sessionID uuid.UUID, finalCapital float64) error {
			if sessionID == testSessionID {
				return nil
			}
			return errors.New("session not found")
		},
	}

	router := setupDashboardTestRouter(mock, nil)

	t.Run("stop trading with valid request", func(t *testing.T) {
		body := map[string]interface{}{
			"session_id":    testSessionID.String(),
			"final_capital": 10500.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/stop", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response TradingResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "stop", response.Action)
	})

	t.Run("stop trading with invalid session ID format", func(t *testing.T) {
		body := map[string]interface{}{
			"session_id":    "invalid-uuid",
			"final_capital": 10000.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/stop", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("stop trading with non-existent session", func(t *testing.T) {
		body := map[string]interface{}{
			"session_id":    uuid.New().String(),
			"final_capital": 10000.0,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/stop", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestPauseTrading(t *testing.T) {
	mock := &MockDashboardRepository{}

	t.Run("pause trading with orchestrator", func(t *testing.T) {
		orch := &MockOrchestrator{
			PauseFunc: func() error {
				return nil
			},
		}

		router := setupDashboardTestRouter(mock, orch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/pause", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response TradingResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "pause", response.Action)
	})

	t.Run("pause trading without orchestrator", func(t *testing.T) {
		router := setupDashboardTestRouter(mock, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/pause", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("pause trading when already paused", func(t *testing.T) {
		orch := &MockOrchestrator{
			PauseFunc: func() error {
				return errors.New("trading is already paused")
			},
		}

		router := setupDashboardTestRouter(mock, orch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/pause", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestResumeTrading(t *testing.T) {
	mock := &MockDashboardRepository{}

	t.Run("resume trading with orchestrator", func(t *testing.T) {
		orch := &MockOrchestrator{
			ResumeFunc: func() error {
				return nil
			},
		}

		router := setupDashboardTestRouter(mock, orch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/resume", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response TradingResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "resume", response.Action)
	})

	t.Run("resume trading without orchestrator", func(t *testing.T) {
		router := setupDashboardTestRouter(mock, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/resume", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("resume trading when not paused", func(t *testing.T) {
		orch := &MockOrchestrator{
			ResumeFunc: func() error {
				return errors.New("trading is not paused")
			},
		}

		router := setupDashboardTestRouter(mock, orch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboard/trading/resume", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 45 * time.Second,
			expected: "45s",
		},
		{
			name:     "minutes",
			duration: 30 * time.Minute,
			expected: "30m0s",
		},
		{
			name:     "hours",
			duration: 2*time.Hour + 30*time.Minute,
			expected: "3h0m0s", // Rounded to hour
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.NotEmpty(t, result)
		})
	}
}

func TestCalculatePositionSummary(t *testing.T) {
	handler := NewDashboardHandler(nil)

	unrealizedPnL := 100.0
	positions := []*db.Position{
		{
			ID:            uuid.New(),
			Side:          db.PositionSideLong,
			EntryPrice:    50000.0,
			Quantity:      0.1,
			UnrealizedPnL: &unrealizedPnL,
		},
		{
			ID:         uuid.New(),
			Side:       db.PositionSideShort,
			EntryPrice: 3000.0,
			Quantity:   1.0,
		},
		{
			ID:         uuid.New(),
			Side:       db.PositionSideLong,
			EntryPrice: 100.0,
			Quantity:   10.0,
		},
	}

	summary := handler.calculatePositionSummary(positions)

	assert.Equal(t, 3, summary.OpenPositions)
	assert.Equal(t, 2, summary.LongPositions)
	assert.Equal(t, 1, summary.ShortPositions)
	assert.Equal(t, 100.0, summary.TotalUnrealized)

	// Total exposure = 50000*0.1 + 3000*1 + 100*10 = 5000 + 3000 + 1000 = 9000
	assert.Equal(t, 9000.0, summary.TotalExposure)
}

func TestNewDashboardHandler(t *testing.T) {
	mock := &MockDashboardRepository{}

	handler := NewDashboardHandler(mock)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.repo)
	assert.Equal(t, "1.0.0", handler.version)
}

func TestNewDashboardHandlerWithOrchestrator(t *testing.T) {
	mock := &MockDashboardRepository{}
	orch := &MockOrchestrator{}

	handler := NewDashboardHandlerWithOrchestrator(mock, orch, "2.0.0")
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.repo)
	assert.NotNil(t, handler.orchestrator)
	assert.Equal(t, "2.0.0", handler.version)
}

func TestSetOrchestrator(t *testing.T) {
	mock := &MockDashboardRepository{}
	handler := NewDashboardHandler(mock)

	assert.Nil(t, handler.orchestrator)

	orch := &MockOrchestrator{}
	handler.SetOrchestrator(orch)

	assert.NotNil(t, handler.orchestrator)
}
