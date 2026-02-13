package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlackBackend(t *testing.T) {
	t.Run("mock backend when webhook URL missing", func(t *testing.T) {
		config := SlackConfig{}
		backend, err := NewSlackBackend(config)
		require.NoError(t, err)
		assert.True(t, backend.IsMock())
		assert.Equal(t, "slack_mock", backend.Name())
	})

	t.Run("real backend with webhook URL", func(t *testing.T) {
		config := SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		}
		backend, err := NewSlackBackend(config)
		require.NoError(t, err)
		assert.False(t, backend.IsMock())
		assert.Equal(t, "slack", backend.Name())
	})

	t.Run("applies defaults", func(t *testing.T) {
		config := SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		}
		backend, err := NewSlackBackend(config)
		require.NoError(t, err)

		// Check defaults were applied
		assert.Equal(t, 30, backend.config.RateLimitPerMinute)
		assert.Equal(t, 30, backend.config.TimeoutSeconds)
		assert.Equal(t, 3, backend.config.RetryAttempts)
		assert.Equal(t, 2, backend.config.RetryDelaySeconds)
	})
}

func TestSlackBackend_SendMock(t *testing.T) {
	config := SlackConfig{}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)
	assert.True(t, backend.IsMock())

	ctx := context.Background()
	notification := Notification{
		Type:     NotificationTypeTradeExecution,
		Title:    "Trade Executed",
		Body:     "BUY 0.5 BTC at $50,000",
		Data:     map[string]string{"symbol": "BTCUSDT"},
		Priority: "high",
	}

	err = backend.Send(ctx, "", notification)
	require.NoError(t, err)
}

func TestSlackBackend_SendWithMockServer(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SlackConfig{
		WebhookURL:         server.URL,
		RateLimitPerMinute: 100,
		RetryAttempts:      1,
	}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	// Fill rate limiter for test
	backend.rateLimiter <- struct{}{}

	ctx := context.Background()
	notification := Notification{
		Type:     NotificationTypeTradeExecution,
		Title:    "Trade Executed",
		Body:     "BUY 0.5 BTC at $50,000",
		Data:     map[string]string{"symbol": "BTCUSDT"},
		Priority: "high",
	}

	err = backend.Send(ctx, "", notification)
	require.NoError(t, err)
}

func TestSlackBackend_SendWithChannelOverride(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SlackConfig{
		WebhookURL:         server.URL,
		Channel:            "#default-channel",
		RateLimitPerMinute: 100,
		RetryAttempts:      1,
	}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	// Fill rate limiter for test
	backend.rateLimiter <- struct{}{}

	ctx := context.Background()
	notification := Notification{
		Type:  NotificationTypeTradeExecution,
		Title: "Test",
		Body:  "Test body",
	}

	// Send with channel override
	err = backend.Send(ctx, "#override-channel", notification)
	require.NoError(t, err)
}

func TestSlackBackend_SendFailure(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	config := SlackConfig{
		WebhookURL:         server.URL,
		RateLimitPerMinute: 100,
		RetryAttempts:      1,
		RetryDelaySeconds:  0, // No delay for tests
	}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	// Fill rate limiter for test
	backend.rateLimiter <- struct{}{}

	ctx := context.Background()
	notification := Notification{
		Type:  NotificationTypeTradeExecution,
		Title: "Test",
		Body:  "Test body",
	}

	err = backend.Send(ctx, "", notification)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSlackBackend_Channel(t *testing.T) {
	config := SlackConfig{}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	t.Run("get channel", func(t *testing.T) {
		assert.Empty(t, backend.GetChannel())
	})

	t.Run("set channel", func(t *testing.T) {
		backend.SetChannel("#new-channel")
		assert.Equal(t, "#new-channel", backend.GetChannel())
	})
}

func TestSlackBackend_GetColorForType(t *testing.T) {
	config := SlackConfig{}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	tests := []struct {
		notifType     NotificationType
		expectedColor string
	}{
		{NotificationTypeTradeExecution, "#28a745"},
		{NotificationTypePnLAlert, "#ffc107"},
		{NotificationTypeCircuitBreaker, "#dc3545"},
		{NotificationTypeConsensusFailure, "#fd7e14"},
		{NotificationTypePositionClosed, "#17a2b8"},
		{NotificationTypeSafetyGuard, "#dc3545"},
		{NotificationTypeSystemError, "#dc3545"},
		{NotificationTypeDailySummary, "#6c757d"},
	}

	for _, tt := range tests {
		t.Run(string(tt.notifType), func(t *testing.T) {
			color := backend.getColorForType(tt.notifType)
			assert.Equal(t, tt.expectedColor, color)
		})
	}
}

func TestSlackBackend_GetEmojiForType(t *testing.T) {
	config := SlackConfig{}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	tests := []struct {
		notifType     NotificationType
		expectedEmoji string
	}{
		{NotificationTypeTradeExecution, ":chart_with_upwards_trend:"},
		{NotificationTypePnLAlert, ":moneybag:"},
		{NotificationTypeCircuitBreaker, ":rotating_light:"},
		{NotificationTypeConsensusFailure, ":warning:"},
		{NotificationTypePositionClosed, ":checkered_flag:"},
		{NotificationTypeSafetyGuard, ":shield:"},
		{NotificationTypeSystemError, ":x:"},
		{NotificationTypeDailySummary, ":clipboard:"},
	}

	for _, tt := range tests {
		t.Run(string(tt.notifType), func(t *testing.T) {
			emoji := backend.getEmojiForType(tt.notifType)
			assert.Equal(t, tt.expectedEmoji, emoji)
		})
	}
}

func TestSlackBackend_BuildMessage(t *testing.T) {
	config := SlackConfig{
		Username:  "Test Bot",
		IconEmoji: ":robot_face:",
		Channel:   "#test-channel",
	}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	notification := Notification{
		Type:     NotificationTypeTradeExecution,
		Title:    "Trade Executed",
		Body:     "BUY 0.5 BTC at $50,000",
		Data:     map[string]string{"symbol": "BTCUSDT", "side": "BUY"},
		Priority: "high",
	}

	msg := backend.buildMessage("#override", notification)

	assert.Equal(t, "Test Bot", msg.Username)
	assert.Equal(t, ":robot_face:", msg.IconEmoji)
	assert.Equal(t, "#override", msg.Channel) // Override takes precedence
	assert.Len(t, msg.Attachments, 1)
	assert.GreaterOrEqual(t, len(msg.Blocks), 4) // header, body, fields, divider, context

	attachment := msg.Attachments[0]
	assert.Equal(t, "#28a745", attachment.Color) // Trade execution color
	assert.Equal(t, "Trade Executed", attachment.Title)
	assert.Contains(t, attachment.Pretext, "HIGH PRIORITY")
}

func TestSlackBackend_BuildBlocks(t *testing.T) {
	config := SlackConfig{}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	notification := Notification{
		Type:     NotificationTypePnLAlert,
		Title:    "P&L Alert",
		Body:     "Significant gain detected",
		Data:     map[string]string{"pnl_percent": "5.5", "pnl_amount": "550.00"},
		Priority: "normal",
	}

	blocks := backend.buildBlocks(notification)

	// Should have header, body section, fields section, divider, and context
	assert.GreaterOrEqual(t, len(blocks), 4)

	// Check header block
	assert.Equal(t, "header", blocks[0].Type)
	assert.Contains(t, blocks[0].Text.Text, "P&L Alert")

	// Check body section
	assert.Equal(t, "section", blocks[1].Type)
	assert.Equal(t, "Significant gain detected", blocks[1].Text.Text)
}

func TestSlackBackend_Close(t *testing.T) {
	config := SlackConfig{}
	backend, err := NewSlackBackend(config)
	require.NoError(t, err)

	err = backend.Close()
	assert.NoError(t, err)
}

func TestDefaultSlackConfig(t *testing.T) {
	config := DefaultSlackConfig()

	assert.Equal(t, "CryptoFunk Bot", config.Username)
	assert.Equal(t, ":chart_with_upwards_trend:", config.IconEmoji)
	assert.Equal(t, 30, config.RateLimitPerMinute)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 2, config.RetryDelaySeconds)
	assert.Equal(t, 30, config.TimeoutSeconds)
}
