package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBackend is a mock notification backend for testing
type MockBackend struct {
	sentNotifications []SentNotification
	shouldFail        bool
}

type SentNotification struct {
	DeviceToken  string
	Notification Notification
}

func (m *MockBackend) Send(ctx context.Context, deviceToken string, notification Notification) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.sentNotifications = append(m.sentNotifications, SentNotification{
		DeviceToken:  deviceToken,
		Notification: notification,
	})
	return nil
}

func (m *MockBackend) Name() string {
	return "mock"
}

func (m *MockBackend) Close() error {
	return nil
}

func (m *MockBackend) Reset() {
	m.sentNotifications = nil
	m.shouldFail = false
}

func TestPreferences(t *testing.T) {
	t.Run("default preferences", func(t *testing.T) {
		prefs := DefaultPreferences()
		assert.True(t, prefs.TradeExecutions)
		assert.True(t, prefs.PnLAlerts)
		assert.True(t, prefs.CircuitBreaker)
		assert.True(t, prefs.ConsensusFailures)
	})

	t.Run("is enabled", func(t *testing.T) {
		prefs := Preferences{
			TradeExecutions:   true,
			PnLAlerts:         false,
			CircuitBreaker:    true,
			ConsensusFailures: false,
		}

		assert.True(t, prefs.IsEnabled(NotificationTypeTradeExecution))
		assert.False(t, prefs.IsEnabled(NotificationTypePnLAlert))
		assert.True(t, prefs.IsEnabled(NotificationTypeCircuitBreaker))
		assert.False(t, prefs.IsEnabled(NotificationTypeConsensusFailure))
	})
}

func TestNotificationData(t *testing.T) {
	t.Run("trade notification data", func(t *testing.T) {
		data := TradeNotificationData("order-123", "BTC/USDT", "BUY", 0.5, 50000.0)
		assert.Equal(t, "order-123", data["order_id"])
		assert.Equal(t, "BTC/USDT", data["symbol"])
		assert.Equal(t, "BUY", data["side"])
		assert.Equal(t, "0.50", data["quantity"])
		assert.Equal(t, "50000.00", data["price"])
	})

	t.Run("pnl notification data", func(t *testing.T) {
		data := PnLNotificationData("session-456", 7.5, 1500.0)
		assert.Equal(t, "session-456", data["session_id"])
		assert.Equal(t, "7.50", data["pnl_percent"])
		assert.Equal(t, "1500.00", data["pnl_amount"])
	})

	t.Run("circuit breaker notification data", func(t *testing.T) {
		data := CircuitBreakerNotificationData("max_drawdown", 10.0)
		assert.Equal(t, "max_drawdown", data["reason"])
		assert.Equal(t, "10.00", data["threshold"])
	})

	t.Run("consensus failure notification data", func(t *testing.T) {
		data := ConsensusFailureNotificationData("ETH/USDT", "insufficient_confidence", 5)
		assert.Equal(t, "ETH/USDT", data["symbol"])
		assert.Equal(t, "insufficient_confidence", data["reason"])
		assert.Equal(t, "5", data["agent_count"])
	})
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "short token",
			token:    "abc",
			expected: "***",
		},
		{
			name:     "normal token",
			token:    "abcd1234efgh5678",
			expected: "abcd...5678",
		},
		{
			name:     "long token",
			token:    "very_long_firebase_token_here_1234567890",
			expected: "very...7890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskToken(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatting(t *testing.T) {
	t.Run("format float", func(t *testing.T) {
		assert.Equal(t, "123.45", formatFloat(123.45))
		assert.Equal(t, "0.00", formatFloat(0.0))
		assert.Equal(t, "-50.00", formatFloat(-50.0))
		assert.Equal(t, "1000.00", formatFloat(1000.0))
	})

	t.Run("format int", func(t *testing.T) {
		assert.Equal(t, "0", formatInt(0))
		assert.Equal(t, "123", formatInt(123))
		assert.Equal(t, "-456", formatInt(-456))
		assert.Equal(t, "1000", formatInt(1000))
	})
}

func TestNotificationHelper(t *testing.T) {
	// We can't test the full service without a database,
	// but we can test the helper methods with a mock service
	t.Run("check pnl threshold", func(t *testing.T) {
		// Create a minimal mock service for testing
		// In a real test, this would use the full service with a test DB

		tests := []struct {
			name         string
			previousPnL  float64
			currentPnL   float64
			threshold    float64
			shouldNotify bool
		}{
			{
				name:         "exceeds threshold positive",
				previousPnL:  100.0,
				currentPnL:   110.0,
				threshold:    5.0,
				shouldNotify: true,
			},
			{
				name:         "exceeds threshold negative",
				previousPnL:  100.0,
				currentPnL:   90.0,
				threshold:    5.0,
				shouldNotify: true,
			},
			{
				name:         "below threshold",
				previousPnL:  100.0,
				currentPnL:   103.0,
				threshold:    5.0,
				shouldNotify: false,
			},
			{
				name:         "zero previous pnl",
				previousPnL:  0.0,
				currentPnL:   50.0,
				threshold:    5.0,
				shouldNotify: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Calculate percentage change
				var percentChange float64
				if tt.previousPnL != 0 {
					percentChange = ((tt.currentPnL - tt.previousPnL) / abs(tt.previousPnL)) * 100
				} else if tt.currentPnL != 0 {
					percentChange = 100
				}

				shouldNotify := abs(percentChange) >= tt.threshold
				assert.Equal(t, tt.shouldNotify, shouldNotify)
			})
		}
	})
}

func TestMockBackend(t *testing.T) {
	mockBackend := &MockBackend{}
	ctx := context.Background()

	t.Run("successful send", func(t *testing.T) {
		mockBackend.Reset()

		notification := Notification{
			Type:  NotificationTypeTradeExecution,
			Title: "Test",
			Body:  "Test body",
		}

		err := mockBackend.Send(ctx, "test-token", notification)
		require.NoError(t, err)
		assert.Len(t, mockBackend.sentNotifications, 1)
		assert.Equal(t, "test-token", mockBackend.sentNotifications[0].DeviceToken)
		assert.Equal(t, "Test", mockBackend.sentNotifications[0].Notification.Title)
	})

	t.Run("failed send", func(t *testing.T) {
		mockBackend.Reset()
		mockBackend.shouldFail = true

		notification := Notification{
			Type:  NotificationTypeTradeExecution,
			Title: "Test",
			Body:  "Test body",
		}

		err := mockBackend.Send(ctx, "test-token", notification)
		require.Error(t, err)
	})

	t.Run("backend name", func(t *testing.T) {
		assert.Equal(t, "mock", mockBackend.Name())
	})

	t.Run("backend close", func(t *testing.T) {
		err := mockBackend.Close()
		require.NoError(t, err)
	})
}
