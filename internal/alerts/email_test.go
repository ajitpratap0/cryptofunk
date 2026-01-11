package alerts

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmailAlerter(t *testing.T) {
	t.Run("error when host missing", func(t *testing.T) {
		config := EmailConfig{
			FromAddress: "test@example.com",
		}
		_, err := NewEmailAlerter(config, []string{"recipient@example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host")
	})

	t.Run("error when from address missing", func(t *testing.T) {
		config := EmailConfig{
			Host: "smtp.example.com",
		}
		_, err := NewEmailAlerter(config, []string{"recipient@example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_address")
	})

	t.Run("error when no recipients", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			FromAddress: "test@example.com",
		}
		_, err := NewEmailAlerter(config, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient")
	})

	t.Run("success with valid config", func(t *testing.T) {
		config := EmailConfig{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "test@example.com",
		}
		alerter, err := NewEmailAlerter(config, []string{"recipient@example.com"})
		require.NoError(t, err)
		assert.NotNil(t, alerter)
	})
}

func TestEmailAlerter_Recipients(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	alerter, err := NewEmailAlerter(config, []string{"user1@example.com"})
	require.NoError(t, err)

	t.Run("get recipients", func(t *testing.T) {
		recipients := alerter.GetRecipients()
		assert.Equal(t, []string{"user1@example.com"}, recipients)
	})

	t.Run("add recipient", func(t *testing.T) {
		alerter.AddRecipient("user2@example.com")
		recipients := alerter.GetRecipients()
		assert.Len(t, recipients, 2)
		assert.Contains(t, recipients, "user2@example.com")
	})

	t.Run("add duplicate recipient", func(t *testing.T) {
		alerter.AddRecipient("user1@example.com")
		recipients := alerter.GetRecipients()
		assert.Len(t, recipients, 2) // Should not add duplicate
	})

	t.Run("remove recipient", func(t *testing.T) {
		alerter.RemoveRecipient("user2@example.com")
		recipients := alerter.GetRecipients()
		assert.Len(t, recipients, 1)
		assert.NotContains(t, recipients, "user2@example.com")
	})

	t.Run("set recipients", func(t *testing.T) {
		newRecipients := []string{"new1@example.com", "new2@example.com"}
		alerter.SetRecipients(newRecipients)
		recipients := alerter.GetRecipients()
		assert.Equal(t, newRecipients, recipients)
	})
}

func TestEmailAlerter_BuildSubject(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	alerter, err := NewEmailAlerter(config, []string{"recipient@example.com"})
	require.NoError(t, err)

	tests := []struct {
		severity       Severity
		title          string
		expectedPrefix string
	}{
		{SeverityCritical, "Trading Halted", "[CRITICAL]"},
		{SeverityWarning, "Warning Alert", "[WARNING]"},
		{SeverityInfo, "Info Update", "[INFO]"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			alert := Alert{
				Title:    tt.title,
				Severity: tt.severity,
			}
			subject := alerter.buildSubject(alert)
			assert.True(t, strings.HasPrefix(subject, tt.expectedPrefix),
				"expected subject to start with %s, got %s", tt.expectedPrefix, subject)
			assert.Contains(t, subject, tt.title)
		})
	}
}

func TestEmailAlerter_BuildBody(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	alerter, err := NewEmailAlerter(config, []string{"recipient@example.com"})
	require.NoError(t, err)

	alert := Alert{
		Title:     "Test Alert",
		Message:   "This is a test alert message",
		Severity:  SeverityCritical,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"symbol": "BTCUSDT",
			"price":  50000.0,
		},
	}

	body, err := alerter.buildBody(alert)
	require.NoError(t, err)

	// Check HTML content
	assert.Contains(t, body, "Test Alert")
	assert.Contains(t, body, "This is a test alert message")
	assert.Contains(t, body, "CRITICAL")
	assert.Contains(t, body, "CryptoFunk")
	assert.Contains(t, body, "#dc3545") // Critical color
}

func TestEmailAlerter_MaskEmails(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	alerter, err := NewEmailAlerter(config, []string{"recipient@example.com"})
	require.NoError(t, err)

	tests := []struct {
		email    string
		expected string
	}{
		{"test@example.com", "te***@example.com"},
		{"ab@example.com", "***@example.com"},
		{"notanemail", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			masked := alerter.maskEmails([]string{tt.email})
			assert.Equal(t, tt.expected, masked[0])
		})
	}
}

func TestEmailAlerter_SendNoRecipients(t *testing.T) {
	config := EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}
	alerter, err := NewEmailAlerter(config, []string{"initial@example.com"})
	require.NoError(t, err)

	// Clear recipients
	alerter.SetRecipients(nil)

	ctx := context.Background()
	alert := Alert{
		Title:    "Test",
		Message:  "Test message",
		Severity: SeverityInfo,
	}

	// Should not error, just skip
	err = alerter.Send(ctx, alert)
	require.NoError(t, err)
}
