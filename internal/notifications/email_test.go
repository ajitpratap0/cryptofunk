package notifications

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmailBackend(t *testing.T) {
	t.Run("mock backend when config incomplete", func(t *testing.T) {
		// Missing host
		config := EmailConfig{
			FromAddress: "test@example.com",
		}
		backend, err := NewEmailBackend(config, nil)
		require.NoError(t, err)
		assert.True(t, backend.IsMock())
		assert.Equal(t, "email_mock", backend.Name())
	})

	t.Run("mock backend when from address missing", func(t *testing.T) {
		config := EmailConfig{
			Host: "smtp.example.com",
		}
		backend, err := NewEmailBackend(config, nil)
		require.NoError(t, err)
		assert.True(t, backend.IsMock())
	})

	t.Run("real backend with complete config", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "test@example.com",
			FromName:    "Test Sender",
		}
		backend, err := NewEmailBackend(config, []string{"recipient@example.com"})
		require.NoError(t, err)
		assert.False(t, backend.IsMock())
		assert.Equal(t, "email", backend.Name())
	})

	t.Run("error when both UseTLS and UseStartTLS enabled", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "test@example.com",
			UseTLS:      true,
			UseStartTLS: true,
		}
		backend, err := NewEmailBackend(config, nil)
		require.Error(t, err)
		assert.Nil(t, backend)
		assert.Contains(t, err.Error(), "cannot enable both UseTLS and UseStartTLS")
	})

	t.Run("UseTLS only is valid", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			Port:        465,
			FromAddress: "test@example.com",
			UseTLS:      true,
			UseStartTLS: false,
		}
		backend, err := NewEmailBackend(config, nil)
		require.NoError(t, err)
		assert.NotNil(t, backend)
		assert.False(t, backend.IsMock())
	})

	t.Run("UseStartTLS only is valid", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "test@example.com",
			UseTLS:      false,
			UseStartTLS: true,
		}
		backend, err := NewEmailBackend(config, nil)
		require.NoError(t, err)
		assert.NotNil(t, backend)
		assert.False(t, backend.IsMock())
	})

	t.Run("SkipVerify logs warning but does not error", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "test@example.com",
			SkipVerify:  true,
		}
		backend, err := NewEmailBackend(config, nil)
		require.NoError(t, err)
		assert.NotNil(t, backend)
		assert.False(t, backend.IsMock())
	})
}

func TestEmailBackend_SendMock(t *testing.T) {
	config := EmailConfig{}
	backend, err := NewEmailBackend(config, []string{"test@example.com"})
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

func TestEmailBackend_Recipients(t *testing.T) {
	config := EmailConfig{}
	backend, err := NewEmailBackend(config, []string{"user1@example.com"})
	require.NoError(t, err)

	t.Run("get recipients", func(t *testing.T) {
		recipients := backend.GetRecipients()
		assert.Equal(t, []string{"user1@example.com"}, recipients)
	})

	t.Run("add recipient", func(t *testing.T) {
		backend.AddRecipient("user2@example.com")
		recipients := backend.GetRecipients()
		assert.Len(t, recipients, 2)
		assert.Contains(t, recipients, "user2@example.com")
	})

	t.Run("add duplicate recipient", func(t *testing.T) {
		backend.AddRecipient("user1@example.com")
		recipients := backend.GetRecipients()
		assert.Len(t, recipients, 2) // Should not add duplicate
	})

	t.Run("remove recipient", func(t *testing.T) {
		backend.RemoveRecipient("user2@example.com")
		recipients := backend.GetRecipients()
		assert.Len(t, recipients, 1)
		assert.NotContains(t, recipients, "user2@example.com")
	})

	t.Run("set recipients", func(t *testing.T) {
		newRecipients := []string{"new1@example.com", "new2@example.com"}
		backend.SetRecipients(newRecipients)
		recipients := backend.GetRecipients()
		assert.Equal(t, newRecipients, recipients)
	})
}

func TestEmailBackend_NoRecipients(t *testing.T) {
	config := EmailConfig{}
	backend, err := NewEmailBackend(config, nil)
	require.NoError(t, err)

	ctx := context.Background()
	notification := Notification{
		Type:  NotificationTypeTradeExecution,
		Title: "Test",
		Body:  "Test body",
	}

	// Should not error, just skip
	err = backend.Send(ctx, "", notification)
	require.NoError(t, err)
}

func TestEmailBackend_BuildSubject(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	backend, err := NewEmailBackend(config, nil)
	require.NoError(t, err)

	tests := []struct {
		notifType      NotificationType
		title          string
		expectedPrefix string
	}{
		{NotificationTypeTradeExecution, "BUY BTC", "[CryptoFunk Trade]"},
		{NotificationTypePnLAlert, "Profit Alert", "[CryptoFunk P&L Alert]"},
		{NotificationTypeCircuitBreaker, "Trading Halted", "[CryptoFunk CRITICAL]"},
		{NotificationTypeConsensusFailure, "No Consensus", "[CryptoFunk Warning]"},
		{NotificationTypePositionClosed, "Position Closed", "[CryptoFunk Position]"},
		{NotificationTypeSafetyGuard, "Safety Triggered", "[CryptoFunk Safety]"},
		{NotificationTypeSystemError, "Error", "[CryptoFunk ERROR]"},
		{NotificationTypeDailySummary, "Summary", "[CryptoFunk Daily Summary]"},
	}

	for _, tt := range tests {
		t.Run(string(tt.notifType), func(t *testing.T) {
			notification := Notification{
				Type:  tt.notifType,
				Title: tt.title,
			}
			subject := backend.buildSubject(notification)
			assert.True(t, strings.HasPrefix(subject, tt.expectedPrefix),
				"expected subject to start with %s, got %s", tt.expectedPrefix, subject)
			assert.Contains(t, subject, tt.title)
		})
	}
}

func TestEmailBackend_BuildBody(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	backend, err := NewEmailBackend(config, nil)
	require.NoError(t, err)

	notification := Notification{
		Type:     NotificationTypeTradeExecution,
		Title:    "Trade Executed",
		Body:     "BUY 0.5 BTC at $50,000",
		Data:     map[string]string{"symbol": "BTCUSDT", "side": "BUY"},
		Priority: "high",
	}

	body, err := backend.buildBody(notification)
	require.NoError(t, err)

	// Check HTML content
	assert.Contains(t, body, "Trade Executed")
	assert.Contains(t, body, "BUY 0.5 BTC at $50,000")
	assert.Contains(t, body, "CryptoFunk")
	assert.Contains(t, body, "priority-banner") // High priority banner class
}

func TestEmailBackend_Close(t *testing.T) {
	config := EmailConfig{}
	backend, err := NewEmailBackend(config, nil)
	require.NoError(t, err)

	err = backend.Close()
	assert.NoError(t, err)
}

func TestDefaultEmailConfig(t *testing.T) {
	config := DefaultEmailConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 587, config.Port)
	assert.Equal(t, "CryptoFunk Trading", config.FromName)
	assert.False(t, config.UseTLS)
	assert.True(t, config.UseStartTLS)
	assert.False(t, config.SkipVerify)
	assert.True(t, config.AuthRequired)
	assert.Equal(t, 30, config.RateLimitPerMinute)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 5, config.RetryDelaySeconds)
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{
			name:     "normal email",
			email:    "test@example.com",
			expected: "te***@example.com",
		},
		{
			name:     "short local part",
			email:    "ab@example.com",
			expected: "***@example.com",
		},
		{
			name:     "invalid email",
			email:    "notanemail",
			expected: "***",
		},
		{
			name:     "long local part",
			email:    "verylongemail@example.com",
			expected: "ve***@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskEmails(t *testing.T) {
	emails := []string{"test@example.com", "user@domain.com"}
	masked := maskEmails(emails)

	assert.Len(t, masked, 2)
	assert.Equal(t, "te***@example.com", masked[0])
	assert.Equal(t, "us***@domain.com", masked[1])
}

func TestFormatDataKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"order_id", "Order Id"},
		{"pnl_percent", "Pnl Percent"},
		{"symbol", "Symbol"},
		{"total_pnl", "Total Pnl"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := formatDataKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}
