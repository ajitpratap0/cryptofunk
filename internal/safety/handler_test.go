package safety

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
)

func setupRouter() (*gin.Engine, *Guard) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lc := NewLimitsConfig()
	mon := NewMonitor(10000)
	g := NewGuard(lc, mon)
	rg := r.Group("/api")
	RegisterRoutes(rg, g)
	return r, g
}

func TestHandleStatus(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/safety/status", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp Status
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.KillSwitchActive)
}

func TestHandleEnableKillSwitch(t *testing.T) {
	r, g := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/api/safety/killswitch", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, g.KillSwitchEnabled())
}

func TestHandleDisableKillSwitch(t *testing.T) {
	r, g := setupRouter()
	g.EnableKillSwitch()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/safety/killswitch", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, g.KillSwitchEnabled())
}

func TestHandleResume(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/api/safety/resume", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleUpdateLimits(t *testing.T) {
	r, g := setupRouter()

	lim := Limits{
		MaxDailyDrawdown:     0.10,
		MaxPositionSize:      0.20,
		MaxTotalExposure:     0.80,
		MaxConsecutiveLosses: 10,
		MaxDailyTrades:       100,
	}
	body, _ := json.Marshal(lim)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/safety/limits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0.10, g.LimitsConfig().Global().MaxDailyDrawdown)
}

func TestHandleUpdateLimits_BadJSON(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/safety/limits", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrOrderRejected_Error(t *testing.T) {
	err := &ErrOrderRejected{Reason: ReasonKillSwitch, Message: "kill switch is active"}
	assert.Contains(t, err.Error(), "kill_switch_active")
	assert.Contains(t, err.Error(), "kill switch is active")
}

func TestGuard_SetPortfolioValue(t *testing.T) {
	lc := NewLimitsConfig()
	mon := NewMonitor(1000)
	g := NewGuard(lc, mon)
	g.SetPortfolioValue(5000)
	assert.Equal(t, 5000.0, g.Status().State.PortfolioValue)
}

func TestGuard_LimitsConfig(t *testing.T) {
	lc := NewLimitsConfig()
	mon := NewMonitor(1000)
	g := NewGuard(lc, mon)
	assert.Equal(t, lc, g.LimitsConfig())
}
