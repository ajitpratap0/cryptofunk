package notifications

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockService implements Service for testing helpers
type mockService struct {
	lastUserID       string
	lastNotification Notification
	err              error
}

func (m *mockService) SendToUser(ctx context.Context, userID string, n Notification) error {
	m.lastUserID = userID
	m.lastNotification = n
	return m.err
}
func (m *mockService) SendToDevice(ctx context.Context, token string, n Notification) error {
	return m.err
}
func (m *mockService) RegisterDevice(ctx context.Context, userID, token string, p Platform) error {
	return m.err
}
func (m *mockService) UnregisterDevice(ctx context.Context, token string) error { return m.err }
func (m *mockService) GetUserDevices(ctx context.Context, userID string) ([]Device, error) {
	return nil, m.err
}
func (m *mockService) UpdatePreferences(ctx context.Context, userID string, p Preferences) error {
	return m.err
}
func (m *mockService) GetPreferences(ctx context.Context, userID string) (Preferences, error) {
	return Preferences{}, m.err
}
func (m *mockService) UpdateDeviceLastUsed(ctx context.Context, token string) error { return m.err }

func TestHelper_SendTradeExecution(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendTradeExecution(context.Background(), "user1", "order1", "BTCUSDT", "BUY", 0.5, 50000)
	require.NoError(t, err)
	assert.Equal(t, "user1", svc.lastUserID)
	assert.Equal(t, NotificationTypeTradeExecution, svc.lastNotification.Type)
}

func TestHelper_SendPnLAlert(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendPnLAlert(context.Background(), "user1", "sess1", -5.5, -550)
	require.NoError(t, err)
	assert.Contains(t, svc.lastNotification.Title, "P&L Alert")
}

func TestHelper_SendPnLAlert_Gain(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendPnLAlert(context.Background(), "user1", "sess1", 10.0, 1000)
	require.NoError(t, err)
	assert.Contains(t, svc.lastNotification.Body, "gain")
}

func TestHelper_SendCircuitBreakerAlert(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendCircuitBreakerAlert(context.Background(), "user1", "drawdown exceeded", 5.0)
	require.NoError(t, err)
	assert.Equal(t, NotificationTypeCircuitBreaker, svc.lastNotification.Type)
}

func TestHelper_SendConsensusFailure(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.SendConsensusFailure(context.Background(), "user1", "BTCUSDT", "disagreement", 3)
	require.NoError(t, err)
	assert.Equal(t, NotificationTypeConsensusFailure, svc.lastNotification.Type)
}

func TestHelper_BulkSendTradeExecution(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.BulkSendTradeExecution(context.Background(), []string{"u1", "u2"}, "o1", "BTCUSDT", "SELL", 1.0, 40000)
	require.NoError(t, err)
}

func TestHelper_BulkSendTradeExecution_WithError(t *testing.T) {
	svc := &mockService{err: errors.New("send failed")}
	h := NewHelper(svc)
	// Should not return error even when individual sends fail
	err := h.BulkSendTradeExecution(context.Background(), []string{"u1", "u2"}, "o1", "BTCUSDT", "SELL", 1.0, 40000)
	require.NoError(t, err)
}

func TestHelper_CheckPnLThresholdAndNotify_BelowThreshold(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.CheckPnLThresholdAndNotify(context.Background(), "user1", "sess1", 1000, 1005, 10.0)
	require.NoError(t, err)
	assert.Empty(t, svc.lastUserID) // no notification sent
}

func TestHelper_CheckPnLThresholdAndNotify_AboveThreshold(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.CheckPnLThresholdAndNotify(context.Background(), "user1", "sess1", 1000, 1200, 10.0)
	require.NoError(t, err)
	assert.Equal(t, "user1", svc.lastUserID)
}

func TestHelper_CheckPnLThresholdAndNotify_FromZero(t *testing.T) {
	svc := &mockService{}
	h := NewHelper(svc)
	err := h.CheckPnLThresholdAndNotify(context.Background(), "user1", "sess1", 0, 100, 10.0)
	require.NoError(t, err)
	assert.Equal(t, "user1", svc.lastUserID) // 100% change from zero
}
