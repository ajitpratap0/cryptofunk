package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockService implements Service for testing
type mockService struct {
	sentNotifications []Notification
	sendErr           error
}

func (m *mockService) SendToUser(_ context.Context, _ string, n Notification) error {
	m.sentNotifications = append(m.sentNotifications, n)
	return m.sendErr
}
func (m *mockService) SendToDevice(_ context.Context, _ string, n Notification) error {
	m.sentNotifications = append(m.sentNotifications, n)
	return m.sendErr
}
func (m *mockService) RegisterDevice(_ context.Context, _, _ string, _ Platform) error { return nil }
func (m *mockService) UnregisterDevice(_ context.Context, _ string) error              { return nil }
func (m *mockService) GetUserDevices(_ context.Context, _ string) ([]Device, error) {
	return nil, nil
}
func (m *mockService) GetPreferences(_ context.Context, _ string) (Preferences, error) {
	return DefaultPreferences(), nil
}
func (m *mockService) UpdatePreferences(_ context.Context, _ string, _ Preferences) error {
	return nil
}
func (m *mockService) UpdateDeviceLastUsed(_ context.Context, _ string) error { return nil }

func TestNotificationHelper_SendTradeExecution(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendTradeExecution(context.Background(), "user1", "order1", "BTCUSDT", "BUY", 0.5, 45000)
	require.NoError(t, err)
	require.Len(t, svc.sentNotifications, 1)
	assert.Equal(t, NotificationTypeTradeExecution, svc.sentNotifications[0].Type)
	assert.Contains(t, svc.sentNotifications[0].Title, "BUY")
	assert.Contains(t, svc.sentNotifications[0].Title, "BTCUSDT")
}

func TestNotificationHelper_SendPnLAlert_Gain(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendPnLAlert(context.Background(), "user1", "sess1", 5.5, 1000)
	require.NoError(t, err)
	require.Len(t, svc.sentNotifications, 1)
	assert.Contains(t, svc.sentNotifications[0].Body, "gain")
}

func TestNotificationHelper_SendPnLAlert_Loss(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendPnLAlert(context.Background(), "user1", "sess1", -3.2, -500)
	require.NoError(t, err)
	assert.Contains(t, svc.sentNotifications[0].Body, "loss")
}

func TestNotificationHelper_SendCircuitBreakerAlert(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendCircuitBreakerAlert(context.Background(), "user1", "max drawdown", 5.0)
	require.NoError(t, err)
	assert.Equal(t, NotificationTypeCircuitBreaker, svc.sentNotifications[0].Type)
	assert.Contains(t, svc.sentNotifications[0].Title, "Circuit Breaker")
}

func TestNotificationHelper_SendConsensusFailure(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendConsensusFailure(context.Background(), "user1", "ETHUSDT", "disagreement", 5)
	require.NoError(t, err)
	assert.Equal(t, NotificationTypeConsensusFailure, svc.sentNotifications[0].Type)
}

func TestNotificationHelper_BulkSendTradeExecution(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.BulkSendTradeExecution(context.Background(), []string{"u1", "u2", "u3"}, "o1", "BTC", "BUY", 1, 50000)
	require.NoError(t, err)
	assert.Len(t, svc.sentNotifications, 3)
}

func TestNotificationHelper_CheckPnLThresholdAndNotify_Exceeded(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.CheckPnLThresholdAndNotify(context.Background(), "user1", "sess1", 1000, 1100, 5.0)
	require.NoError(t, err)
	assert.Len(t, svc.sentNotifications, 1) // 10% change > 5% threshold
}

func TestNotificationHelper_CheckPnLThresholdAndNotify_NotExceeded(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.CheckPnLThresholdAndNotify(context.Background(), "user1", "sess1", 1000, 1010, 5.0)
	require.NoError(t, err)
	assert.Len(t, svc.sentNotifications, 0) // 1% change < 5% threshold
}

func TestNotificationHelper_CheckPnLThresholdAndNotify_FromZero(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.CheckPnLThresholdAndNotify(context.Background(), "user1", "sess1", 0, 100, 5.0)
	require.NoError(t, err)
	assert.Len(t, svc.sentNotifications, 1) // 100% change from zero
}

func TestPreferences_IsEnabled(t *testing.T) {
	p := DefaultPreferences()
	assert.True(t, p.IsEnabled(NotificationTypeTradeExecution))
	assert.True(t, p.IsEnabled(NotificationTypePnLAlert))
	assert.True(t, p.IsEnabled(NotificationTypeCircuitBreaker))
	assert.True(t, p.IsEnabled(NotificationTypeConsensusFailure))
	assert.False(t, p.IsEnabled("unknown"))

	p.TradeExecutions = false
	assert.False(t, p.IsEnabled(NotificationTypeTradeExecution))
}

func TestTradeNotificationData(t *testing.T) {
	data := TradeNotificationData("o1", "BTC", "BUY", 1.5, 50000)
	assert.Equal(t, "o1", data["order_id"])
	assert.Equal(t, "BTC", data["symbol"])
	assert.Equal(t, "BUY", data["side"])
}

func TestFormatFloat(t *testing.T) {
	assert.Equal(t, "0.50", formatFloat(0.5))
	assert.Equal(t, "100.00", formatFloat(100.0))
	assert.Equal(t, "-5.25", formatFloat(-5.25))
}

func TestFormatInt(t *testing.T) {
	assert.Equal(t, "0", formatInt(0))
	assert.Equal(t, "42", formatInt(42))
	assert.Equal(t, "-7", formatInt(-7))
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 5.0, abs(5.0))
	assert.Equal(t, 5.0, abs(-5.0))
	assert.Equal(t, 0.0, abs(0.0))
}
