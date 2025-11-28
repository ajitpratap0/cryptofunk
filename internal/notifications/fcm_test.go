package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFCMBackend(t *testing.T) {
	ctx := context.Background()

	t.Run("empty credentials path uses mock", func(t *testing.T) {
		backend, err := NewFCMBackend(ctx, "")
		require.NoError(t, err)
		assert.NotNil(t, backend)
		assert.True(t, backend.IsMock())
		assert.Equal(t, "fcm_mock", backend.Name())
	})

	t.Run("non-existent credentials path uses mock", func(t *testing.T) {
		backend, err := NewFCMBackend(ctx, "/nonexistent/path/credentials.json")
		require.NoError(t, err)
		assert.NotNil(t, backend)
		assert.True(t, backend.IsMock())
		assert.Equal(t, "fcm_mock", backend.Name())
	})
}

func TestFCMBackendMock(t *testing.T) {
	ctx := context.Background()
	backend, err := NewFCMBackend(ctx, "")
	require.NoError(t, err)
	require.True(t, backend.IsMock())

	t.Run("send notification", func(t *testing.T) {
		notification := Notification{
			Type:     NotificationTypeTradeExecution,
			Title:    "Trade Executed",
			Body:     "BTC/USDT order filled",
			Data:     TradeNotificationData("order-123", "BTC/USDT", "BUY", 0.5, 50000.0),
			Priority: "high",
		}

		err := backend.Send(ctx, "mock-device-token", notification)
		require.NoError(t, err)
	})

	t.Run("send multicast notification", func(t *testing.T) {
		notification := Notification{
			Type:     NotificationTypePnLAlert,
			Title:    "P&L Alert",
			Body:     "Significant gain detected",
			Data:     PnLNotificationData("session-123", 7.5, 1500.0),
			Priority: "high",
		}

		deviceTokens := []string{"token1", "token2", "token3"}
		response, err := backend.SendMulticast(ctx, deviceTokens, notification)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, len(deviceTokens), response.SuccessCount)
	})

	t.Run("backend name", func(t *testing.T) {
		assert.Equal(t, "fcm_mock", backend.Name())
	})

	t.Run("close backend", func(t *testing.T) {
		err := backend.Close()
		require.NoError(t, err)
	})
}

func TestFCMValidateToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{
			name:  "valid FCM token",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1xd2VydHl1aW9wYXNkZmdoamtsenhjdmJubXF3ZXJ0eXVpb3Bhc2RmZ2hqa2x6eGN2Ym5tcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: true,
		},
		{
			name:  "valid FCM token with hyphens",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0tcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0tcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0tcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: true,
		},
		{
			name:  "valid FCM token with underscores",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: true,
		},
		{
			name:  "valid FCM token with colons",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm06cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm06cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm06cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: true,
		},
		{
			name:  "too short",
			token: "short",
			valid: false,
		},
		{
			name:  "too long",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm1fcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: false,
		},
		{
			name:  "invalid characters",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0@cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0jcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0kcXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: false,
		},
		{
			name:  "spaces not allowed",
			token: "cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0 cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0 cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0 cXdlcnR5dWlvcGFzZGZnaGprbHp4Y3Zibm0",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateToken(tt.token)
			assert.Equal(t, tt.valid, result, "token: %s", tt.token)
		})
	}
}
