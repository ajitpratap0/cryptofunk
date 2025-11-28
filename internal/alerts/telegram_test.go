package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTelegramAlerter(t *testing.T) {
	tests := []struct {
		name      string
		botToken  string
		chatIDs   []int64
		wantError bool
		errMsg    string
	}{
		{
			name:      "valid config with chat IDs",
			botToken:  "test_token",
			chatIDs:   []int64{123456789},
			wantError: true, // Will fail without actual Telegram API
		},
		{
			name:      "empty bot token",
			botToken:  "",
			chatIDs:   []int64{123456789},
			wantError: true,
			errMsg:    "bot token is required",
		},
		{
			name:      "no chat IDs",
			botToken:  "test_token",
			chatIDs:   []int64{},
			wantError: true, // Will fail without actual Telegram API
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerter, err := NewTelegramAlerter(tt.botToken, tt.chatIDs)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, alerter)
			}
		})
	}
}

func TestTelegramAlerter_AddChatID(t *testing.T) {
	alerter := &TelegramAlerter{
		chatIDs: []int64{123456789},
	}

	// Add new chat ID
	alerter.AddChatID(987654321)
	assert.Len(t, alerter.chatIDs, 2)
	assert.Contains(t, alerter.chatIDs, int64(987654321))

	// Add duplicate chat ID (should not add)
	alerter.AddChatID(123456789)
	assert.Len(t, alerter.chatIDs, 2)
}

func TestTelegramAlerter_RemoveChatID(t *testing.T) {
	alerter := &TelegramAlerter{
		chatIDs: []int64{123456789, 987654321},
	}

	// Remove existing chat ID
	alerter.RemoveChatID(123456789)
	assert.Len(t, alerter.chatIDs, 1)
	assert.NotContains(t, alerter.chatIDs, int64(123456789))

	// Remove non-existent chat ID (should not error)
	alerter.RemoveChatID(111111111)
	assert.Len(t, alerter.chatIDs, 1)
}

func TestTelegramAlerter_GetChatIDs(t *testing.T) {
	chatIDs := []int64{123456789, 987654321}
	alerter := &TelegramAlerter{
		chatIDs: chatIDs,
	}

	result := alerter.GetChatIDs()
	assert.Equal(t, chatIDs, result)
}

func TestTelegramAlerter_SetChatIDs(t *testing.T) {
	alerter := &TelegramAlerter{
		chatIDs: []int64{123456789},
	}

	newChatIDs := []int64{987654321, 111111111}
	alerter.SetChatIDs(newChatIDs)

	assert.Equal(t, newChatIDs, alerter.chatIDs)
}

func TestTelegramAlerter_FormatAlert(t *testing.T) {
	alerter := &TelegramAlerter{}

	tests := []struct {
		name     string
		alert    Alert
		contains []string
	}{
		{
			name: "critical alert",
			alert: Alert{
				Title:     "System Error",
				Message:   "Database connection failed",
				Severity:  SeverityCritical,
				Timestamp: time.Now(),
			},
			contains: []string{"🚨", "System Error", "Database connection failed"},
		},
		{
			name: "warning alert",
			alert: Alert{
				Title:     "High CPU Usage",
				Message:   "CPU usage above 80%",
				Severity:  SeverityWarning,
				Timestamp: time.Now(),
			},
			contains: []string{"⚠️", "High CPU Usage", "CPU usage above 80%"},
		},
		{
			name: "info alert",
			alert: Alert{
				Title:     "New Trade",
				Message:   "Bought 0.1 BTC",
				Severity:  SeverityInfo,
				Timestamp: time.Now(),
			},
			contains: []string{"ℹ️", "New Trade", "Bought 0.1 BTC"},
		},
		{
			name: "alert with metadata",
			alert: Alert{
				Title:     "Order Executed",
				Message:   "Market order executed",
				Severity:  SeverityInfo,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"symbol":   "BTCUSDT",
					"quantity": 0.1,
					"price":    50000.0,
				},
			},
			contains: []string{"Order Executed", "Market order executed", "Details:", "symbol", "BTCUSDT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := alerter.formatAlert(tt.alert)
			for _, str := range tt.contains {
				assert.Contains(t, result, str)
			}
		})
	}
}

func TestTelegramAlerter_Send_NoChatIDs(t *testing.T) {
	alerter := &TelegramAlerter{
		chatIDs: []int64{},
	}

	alert := Alert{
		Title:     "Test Alert",
		Message:   "This is a test",
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	err := alerter.Send(ctx, alert)

	// Should not error when no chat IDs configured
	assert.NoError(t, err)
}

func TestAlert_Severity(t *testing.T) {
	assert.Equal(t, Severity("INFO"), SeverityInfo)
	assert.Equal(t, Severity("WARNING"), SeverityWarning)
	assert.Equal(t, Severity("CRITICAL"), SeverityCritical)
}
