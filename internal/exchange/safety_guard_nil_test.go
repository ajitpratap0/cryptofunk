package exchange

// Tests for service safety-guard methods when the guard is initialized but disabled
// (i.e., NewServicePaper — SafetyGuardConfig{Enabled: false}).
//
// Note: safetyGuard is always non-nil; the nil branch in each method is dead code
// because NewService always calls risk.NewSafetyGuard(config.SafetyGuard).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPaperServiceNoSafety(t *testing.T) *Service {
	t.Helper()
	s := NewServicePaper(nil) // nil DB, no safety guard config → guard initialized but disabled
	return s
}

func TestGetSafetyGuardStats_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.GetSafetyGuardStats(context.Background(), nil)
	require.NoError(t, err)
	m := result.(map[string]interface{})
	// Guard is non-nil, so enabled is always true in the non-nil branch
	assert.Equal(t, true, m["enabled"])
	assert.Equal(t, float64(0), m["daily_pnl"])
	assert.Equal(t, int(0), m["daily_trade_count"])
}

func TestSetSafetyGuardCapital_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.SetSafetyGuardCapital(context.Background(), map[string]interface{}{"capital": 1000.0})
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["success"])
	assert.Equal(t, 1000.0, m["capital"])
}

func TestRecordTradePnL_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.RecordTradePnL(context.Background(), map[string]interface{}{"pnl": 50.0})
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["success"])
	assert.Equal(t, 50.0, m["pnl_recorded"])
}

func TestResetSafetyGuardCircuitBreaker_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.ResetSafetyGuardCircuitBreaker(context.Background(), nil)
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["success"])
}

func TestResetDailyCounters_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	// nil args → capital extraction fails: "capital is required"
	_, err := s.ResetDailyCounters(context.Background(), nil)
	assert.Error(t, err)

	// valid call succeeds
	result, err := s.ResetDailyCounters(context.Background(), map[string]interface{}{"capital": 5000.0})
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["success"])
	assert.Equal(t, 5000.0, m["new_capital"])
}

func TestEmergencyStop_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.EmergencyStop(context.Background(), map[string]interface{}{"reason": "test"})
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["success"])
	assert.Equal(t, "test", m["reason"])
}

func TestClearEmergencyStop_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.ClearEmergencyStop(context.Background(), nil)
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, true, m["success"])
	assert.Equal(t, false, m["was_active"])
}

func TestGetEmergencyStopStatus_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.GetEmergencyStopStatus(context.Background(), nil)
	require.NoError(t, err)
	m := result.(map[string]interface{})
	assert.Equal(t, false, m["active"])
}

func TestGetTradingHoursStatus_NoGuard(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	result, err := s.GetTradingHoursStatus(context.Background(), nil)
	require.NoError(t, err)
	m := result.(map[string]interface{})
	// TradingHours.Enabled is false in zero config
	assert.Equal(t, false, m["enabled"])
	// IsWithinTradingHours returns true when disabled
	assert.Equal(t, true, m["within_hours"])
}

func TestSetMarketPrice_Service(t *testing.T) {
	s := newPaperServiceNoSafety(t)
	// SetMarketPrice delegates to the mock exchange — should not panic
	s.SetMarketPrice("BTCUSDT", 45000.0)
	price := s.exchange.GetMarketPrice("BTCUSDT")
	assert.Equal(t, 45000.0, price)
}
