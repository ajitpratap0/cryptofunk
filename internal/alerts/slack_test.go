package alerts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlackAlerter(t *testing.T) {
	t.Run("error when webhook URL missing", func(t *testing.T) {
		config := SlackConfig{}
		_, err := NewSlackAlerter(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhook URL")
	})

	t.Run("success with webhook URL", func(t *testing.T) {
		config := SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		}
		alerter, err := NewSlackAlerter(config)
		require.NoError(t, err)
		assert.NotNil(t, alerter)
	})

	t.Run("applies defaults", func(t *testing.T) {
		config := SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		}
		alerter, err := NewSlackAlerter(config)
		require.NoError(t, err)

		assert.Equal(t, 30, alerter.config.TimeoutSeconds)
		assert.Equal(t, "CryptoFunk Alerts", alerter.config.Username)
		assert.Equal(t, ":rotating_light:", alerter.config.IconEmoji)
	})
}

func TestSlackAlerter_Send(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SlackConfig{
		WebhookURL: server.URL,
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	ctx := context.Background()
	alert := Alert{
		Title:     "Test Alert",
		Message:   "This is a test message",
		Severity:  SeverityWarning,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	err = alerter.Send(ctx, alert)
	require.NoError(t, err)
}

func TestSlackAlerter_SendFailure(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	config := SlackConfig{
		WebhookURL: server.URL,
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	ctx := context.Background()
	alert := Alert{
		Title:    "Test",
		Message:  "Test message",
		Severity: SeverityInfo,
	}

	err = alerter.Send(ctx, alert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSlackAlerter_Channel(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		Channel:    "#initial-channel",
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	t.Run("get channel", func(t *testing.T) {
		assert.Equal(t, "#initial-channel", alerter.GetChannel())
	})

	t.Run("set channel", func(t *testing.T) {
		alerter.SetChannel("#new-channel")
		assert.Equal(t, "#new-channel", alerter.GetChannel())
	})
}

func TestSlackAlerter_GetColorForSeverity(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	tests := []struct {
		severity      Severity
		expectedColor string
	}{
		{SeverityCritical, "#dc3545"},
		{SeverityWarning, "#ffc107"},
		{SeverityInfo, "#17a2b8"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			color := alerter.getColorForSeverity(tt.severity)
			assert.Equal(t, tt.expectedColor, color)
		})
	}
}

func TestSlackAlerter_GetEmojiForSeverity(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	tests := []struct {
		severity      Severity
		expectedEmoji string
	}{
		{SeverityCritical, ":rotating_light:"},
		{SeverityWarning, ":warning:"},
		{SeverityInfo, ":information_source:"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			emoji := alerter.getEmojiForSeverity(tt.severity)
			assert.Equal(t, tt.expectedEmoji, emoji)
		})
	}
}

func TestSlackAlerter_BuildMessage(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
		Username:   "Test Bot",
		IconEmoji:  ":robot_face:",
		Channel:    "#test-channel",
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	alert := Alert{
		Title:     "Test Alert",
		Message:   "This is a test message",
		Severity:  SeverityCritical,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	msg := alerter.buildMessage(alert)

	assert.Equal(t, "Test Bot", msg.Username)
	assert.Equal(t, ":robot_face:", msg.IconEmoji)
	assert.Equal(t, "#test-channel", msg.Channel)
	assert.Len(t, msg.Attachments, 1)
	assert.GreaterOrEqual(t, len(msg.Blocks), 4)

	attachment := msg.Attachments[0]
	assert.Equal(t, "#dc3545", attachment.Color) // Critical color
	assert.Equal(t, "Test Alert", attachment.Title)
	assert.Contains(t, attachment.Pretext, "CRITICAL")
	assert.Len(t, attachment.Fields, 2) // Two metadata fields
}

func TestSlackAlerter_BuildBlocks(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	alert := Alert{
		Title:     "Warning Alert",
		Message:   "Something needs attention",
		Severity:  SeverityWarning,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"component": "trading-engine",
		},
	}

	blocks := alerter.buildBlocks(alert)

	// Should have header, body section, fields section, divider, and context
	assert.GreaterOrEqual(t, len(blocks), 4)

	// Check header block
	assert.Equal(t, "header", blocks[0].Type)
	assert.Contains(t, blocks[0].Text.Text, "Warning Alert")
	assert.Contains(t, blocks[0].Text.Text, ":warning:")

	// Check body section
	assert.Equal(t, "section", blocks[1].Type)
	assert.Equal(t, "Something needs attention", blocks[1].Text.Text)
}

func TestSlackAlerter_BuildMessageNoCriticalPretext(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
	}
	alerter, err := NewSlackAlerter(config)
	require.NoError(t, err)

	// Non-critical alert should not have pretext
	alert := Alert{
		Title:    "Info Alert",
		Message:  "Just informational",
		Severity: SeverityInfo,
	}

	msg := alerter.buildMessage(alert)

	assert.Empty(t, msg.Attachments[0].Pretext)
}
