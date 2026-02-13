package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()

	// Create mock backends
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	manager := NewManager(config, emailBackend, slackBackend)

	assert.NotNil(t, manager)
	assert.True(t, manager.IsEnabled())
}

func TestManager_Send(t *testing.T) {
	t.Run("sends to configured channels", func(t *testing.T) {
		// Create test Slack server
		slackCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slackCalled = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create backends
		slackConfig := SlackConfig{
			WebhookURL:         server.URL,
			RateLimitPerMinute: 100,
			RetryAttempts:      1,
		}
		slackBackend, err := NewSlackBackend(slackConfig)
		require.NoError(t, err)

		// Fill rate limiter for test
		slackBackend.rateLimiter <- struct{}{}

		config := ManagerConfig{
			Enabled: true,
			Events: map[NotificationType]EventConfig{
				NotificationTypeTradeExecution: {
					Channels: []ChannelType{ChannelSlack},
					Priority: "high",
					Enabled:  true,
				},
			},
		}

		manager := NewManager(config, nil, slackBackend)

		ctx := context.Background()
		notification := Notification{
			Type:  NotificationTypeTradeExecution,
			Title: "Trade Executed",
			Body:  "BUY 0.5 BTC at $50,000",
		}

		err = manager.Send(ctx, notification)
		require.NoError(t, err)
		assert.True(t, slackCalled)
	})

	t.Run("skips when globally disabled", func(t *testing.T) {
		config := ManagerConfig{
			Enabled: false,
		}

		manager := NewManager(config, nil, nil)

		ctx := context.Background()
		notification := Notification{
			Type:  NotificationTypeTradeExecution,
			Title: "Test",
			Body:  "Test",
		}

		err := manager.Send(ctx, notification)
		require.NoError(t, err)
	})

	t.Run("skips when event type disabled", func(t *testing.T) {
		config := ManagerConfig{
			Enabled: true,
			Events: map[NotificationType]EventConfig{
				NotificationTypeTradeExecution: {
					Channels: []ChannelType{ChannelSlack},
					Priority: "high",
					Enabled:  false,
				},
			},
		}

		manager := NewManager(config, nil, nil)

		ctx := context.Background()
		notification := Notification{
			Type:  NotificationTypeTradeExecution,
			Title: "Test",
			Body:  "Test",
		}

		err := manager.Send(ctx, notification)
		require.NoError(t, err)
	})

	t.Run("uses default channels for unknown event type", func(t *testing.T) {
		slackCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slackCalled = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		slackBackend, err := NewSlackBackend(SlackConfig{
			WebhookURL:         server.URL,
			RateLimitPerMinute: 100,
			RetryAttempts:      1,
		})
		require.NoError(t, err)
		slackBackend.rateLimiter <- struct{}{}

		config := ManagerConfig{
			Enabled:         true,
			Events:          map[NotificationType]EventConfig{},
			DefaultChannels: []ChannelType{ChannelSlack},
		}

		manager := NewManager(config, nil, slackBackend)

		ctx := context.Background()
		notification := Notification{
			Type:  "unknown_type",
			Title: "Test",
			Body:  "Test",
		}

		err = manager.Send(ctx, notification)
		require.NoError(t, err)
		assert.True(t, slackCalled)
	})
}

func TestManager_SendTradeExecuted(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendTradeExecuted(ctx, "order123", "BTCUSDT", "BUY", 0.5, 50000)
	require.NoError(t, err)
}

func TestManager_SendPositionClosed(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendPositionClosed(ctx, "pos123", "BTCUSDT", "LONG", 0.5, 50000, 52000, 1000, 4.0)
	require.NoError(t, err)
}

func TestManager_SendSafetyGuardTriggered(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendSafetyGuardTriggered(ctx, "max_daily_drawdown", "Daily loss exceeded limit", 0.08, 0.05)
	require.NoError(t, err)
}

func TestManager_SendSystemError(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendSystemError(ctx, "orchestrator", "connection_error", "Failed to connect to NATS")
	require.NoError(t, err)
}

func TestManager_SendDailySummary(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendDailySummary(ctx, "2024-01-15", 10, 7, 3, 1500.00, 15.0, 70.0)
	require.NoError(t, err)
}

func TestManager_SendCircuitBreakerTriggered(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendCircuitBreakerTriggered(ctx, "consecutive_losses", 0.6)
	require.NoError(t, err)
}

func TestManager_SendConsensusFailure(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendConsensusFailure(ctx, "BTCUSDT", "insufficient_confidence", 5)
	require.NoError(t, err)
}

func TestManager_SendPnLAlert(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	err := manager.SendPnLAlert(ctx, "session123", 5.5, 550.00)
	require.NoError(t, err)
}

func TestManager_GetStats(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{
		Host:        "smtp.example.com",
		FromAddress: "test@example.com",
	}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
	})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	// Send some notifications to generate stats
	ctx := context.Background()
	_ = manager.SendTradeExecuted(ctx, "order1", "BTCUSDT", "BUY", 0.1, 50000)

	stats := manager.GetStats()

	assert.True(t, stats["enabled"].(bool))
	assert.NotNil(t, stats["total_sent"])
	assert.NotNil(t, stats["total_failed"])
}

func TestManager_IsEventEnabled(t *testing.T) {
	config := ManagerConfig{
		Enabled: true,
		Events: map[NotificationType]EventConfig{
			NotificationTypeTradeExecution: {
				Enabled: true,
			},
			NotificationTypePnLAlert: {
				Enabled: false,
			},
		},
	}

	manager := NewManager(config, nil, nil)

	assert.True(t, manager.IsEventEnabled(NotificationTypeTradeExecution))
	assert.False(t, manager.IsEventEnabled(NotificationTypePnLAlert))
	assert.True(t, manager.IsEventEnabled(NotificationTypeDailySummary)) // Not configured, defaults to true
}

func TestManager_GetChannelsForEvent(t *testing.T) {
	config := ManagerConfig{
		Enabled: true,
		Events: map[NotificationType]EventConfig{
			NotificationTypeTradeExecution: {
				Channels: []ChannelType{ChannelEmail, ChannelSlack},
			},
			NotificationTypeDailySummary: {
				Channels: []ChannelType{ChannelEmail},
			},
		},
		DefaultChannels: []ChannelType{ChannelSlack},
	}

	manager := NewManager(config, nil, nil)

	tradeChannels := manager.GetChannelsForEvent(NotificationTypeTradeExecution)
	assert.Contains(t, tradeChannels, ChannelEmail)
	assert.Contains(t, tradeChannels, ChannelSlack)

	summaryChannels := manager.GetChannelsForEvent(NotificationTypeDailySummary)
	assert.Contains(t, summaryChannels, ChannelEmail)
	assert.NotContains(t, summaryChannels, ChannelSlack)

	unknownChannels := manager.GetChannelsForEvent("unknown_type")
	assert.Contains(t, unknownChannels, ChannelSlack) // Default
}

func TestManager_Close(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	err := manager.Close()
	assert.NoError(t, err)
}

func TestManager_ConcurrentSends(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Send 50 concurrent notifications
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = manager.SendTradeExecuted(ctx, "order"+string(rune(i)), "BTCUSDT", "BUY", 0.1, 50000)
		}(i)
	}

	wg.Wait()

	stats := manager.GetStats()
	assert.NotNil(t, stats["total_sent"])
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	assert.True(t, config.Enabled)
	assert.NotEmpty(t, config.Events)
	assert.NotEmpty(t, config.DefaultChannels)

	// Check trade_execution config
	tradeConfig, exists := config.Events[NotificationTypeTradeExecution]
	assert.True(t, exists)
	assert.True(t, tradeConfig.Enabled)
	assert.Equal(t, "high", tradeConfig.Priority)
	assert.Contains(t, tradeConfig.Channels, ChannelEmail)
	assert.Contains(t, tradeConfig.Channels, ChannelSlack)

	// Check daily_summary config (email only)
	summaryConfig, exists := config.Events[NotificationTypeDailySummary]
	assert.True(t, exists)
	assert.True(t, summaryConfig.Enabled)
	assert.Equal(t, "normal", summaryConfig.Priority)
	assert.Contains(t, summaryConfig.Channels, ChannelEmail)
	assert.NotContains(t, summaryConfig.Channels, ChannelSlack)

	// Check safety_guard config (critical)
	safetyConfig, exists := config.Events[NotificationTypeSafetyGuard]
	assert.True(t, exists)
	assert.True(t, safetyConfig.Enabled)
	assert.Equal(t, "high", safetyConfig.Priority)
}
