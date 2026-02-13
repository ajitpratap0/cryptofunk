package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewIntegration(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)

	integration := NewIntegration(manager)

	assert.NotNil(t, integration)
	assert.True(t, integration.IsEnabled())
}

func TestIntegration_SetEnabled(t *testing.T) {
	integration := NewIntegration(nil)

	assert.True(t, integration.IsEnabled())

	integration.SetEnabled(false)
	assert.False(t, integration.IsEnabled())

	integration.SetEnabled(true)
	assert.True(t, integration.IsEnabled())
}

func TestIntegration_EmitTradeExecuted(t *testing.T) {
	// Create test server with proper synchronization
	var mu sync.Mutex
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		called = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slackBackend, _ := NewSlackBackend(SlackConfig{
		WebhookURL:         server.URL,
		RateLimitPerMinute: 100,
		RetryAttempts:      1,
	})
	slackBackend.rateLimiter <- struct{}{}

	config := ManagerConfig{
		Enabled: true,
		Events: map[NotificationType]EventConfig{
			NotificationTypeTradeExecution: {
				Channels: []ChannelType{ChannelSlack},
				Enabled:  true,
			},
		},
	}

	manager := NewManager(config, nil, slackBackend)
	integration := NewIntegration(manager)

	ctx := context.Background()
	integration.EmitTradeExecuted(ctx, "order123", "BTCUSDT", "BUY", 0.5, 50000)

	// Wait for async operation
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	wasCalled := called
	mu.Unlock()
	assert.True(t, wasCalled)
}

func TestIntegration_Cooldowns(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	// Set a very short cooldown for testing
	cooldown := 50 * time.Millisecond
	integration.SetCooldown(NotificationTypePnLAlert, cooldown)

	ctx := context.Background()

	// First emit uses shouldSend internally
	integration.EmitPnLAlert(ctx, "session1", 5.5, 550.00)

	// Wait a bit for async processing
	time.Sleep(10 * time.Millisecond)

	// Immediate second call to shouldSend should return false (cooldown not passed)
	assert.False(t, integration.shouldSend(NotificationTypePnLAlert))

	// Wait for cooldown to pass
	time.Sleep(cooldown + 10*time.Millisecond)

	// Now it should be allowed again
	assert.True(t, integration.shouldSend(NotificationTypePnLAlert))
}

func TestIntegration_CooldownRateLimiting(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	// Set a 100ms cooldown
	cooldown := 100 * time.Millisecond
	integration.SetCooldown(NotificationTypePnLAlert, cooldown)

	// First call should be allowed
	assert.True(t, integration.shouldSend(NotificationTypePnLAlert))

	// Immediate second call should be blocked
	assert.False(t, integration.shouldSend(NotificationTypePnLAlert))

	// Wait for cooldown to pass
	time.Sleep(cooldown + 10*time.Millisecond)

	// Now it should be allowed again
	assert.True(t, integration.shouldSend(NotificationTypePnLAlert))
}

func TestIntegration_DisabledDoesNotSend(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slackBackend, _ := NewSlackBackend(SlackConfig{
		WebhookURL:         server.URL,
		RateLimitPerMinute: 100,
	})

	config := DefaultManagerConfig()
	manager := NewManager(config, nil, slackBackend)
	integration := NewIntegration(manager)

	// Disable integration
	integration.SetEnabled(false)

	ctx := context.Background()
	integration.EmitTradeExecuted(ctx, "order123", "BTCUSDT", "BUY", 0.5, 50000)

	time.Sleep(50 * time.Millisecond)
	assert.False(t, called)
}

func TestIntegration_NilManagerDoesNotPanic(t *testing.T) {
	integration := NewIntegration(nil)

	ctx := context.Background()

	// These should not panic even with nil manager
	assert.NotPanics(t, func() {
		integration.EmitTradeExecuted(ctx, "order123", "BTCUSDT", "BUY", 0.5, 50000)
	})

	assert.NotPanics(t, func() {
		integration.EmitPositionClosed(ctx, "pos123", "BTCUSDT", "LONG", 0.5, 50000, 52000, 1000, 4.0)
	})

	assert.NotPanics(t, func() {
		integration.EmitSafetyGuardTriggered(ctx, "max_drawdown", "exceeded", 0.08, 0.05)
	})

	assert.NotPanics(t, func() {
		integration.EmitSystemError(ctx, "test", "error", "message")
	})

	assert.NotPanics(t, func() {
		integration.EmitDailySummary(ctx, "2024-01-15", 10, 7, 3, 1500, 15.0, 70.0)
	})

	assert.NotPanics(t, func() {
		integration.EmitCircuitBreakerTriggered(ctx, "reason", 0.6)
	})

	assert.NotPanics(t, func() {
		integration.EmitPnLAlert(ctx, "session1", 5.5, 550.00)
	})

	assert.NotPanics(t, func() {
		integration.EmitConsensusFailure(ctx, "BTCUSDT", "reason", 5)
	})
}

func TestIntegration_GetStats(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	ctx := context.Background()
	integration.EmitTradeExecuted(ctx, "order123", "BTCUSDT", "BUY", 0.5, 50000)

	time.Sleep(50 * time.Millisecond)

	stats := integration.GetStats()

	assert.True(t, stats["enabled"].(bool))
	assert.NotNil(t, stats["last_notifications"])
	assert.NotNil(t, stats["cooldowns"])
}

func TestIntegration_ConcurrentEmits(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Emit 50 concurrent notifications of different types
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 5 {
			case 0:
				integration.EmitTradeExecuted(ctx, "order", "BTCUSDT", "BUY", 0.1, 50000)
			case 1:
				integration.EmitPositionClosed(ctx, "pos", "BTCUSDT", "LONG", 0.1, 50000, 51000, 100, 2.0)
			case 2:
				integration.EmitSystemError(ctx, "test", "error", "msg")
			case 3:
				integration.EmitPnLAlert(ctx, "session", 5.0, 500)
			case 4:
				integration.EmitConsensusFailure(ctx, "BTCUSDT", "reason", 3)
			}
		}(i)
	}

	wg.Wait()

	// Should complete without panic
}

func TestIntegration_EmitSafetyGuardTriggered(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	ctx := context.Background()

	// Should not panic
	assert.NotPanics(t, func() {
		integration.EmitSafetyGuardTriggered(ctx, "max_daily_drawdown", "Daily loss exceeded", 0.08, 0.05)
	})
}

func TestIntegration_EmitDailySummary(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	ctx := context.Background()

	// Should not panic
	assert.NotPanics(t, func() {
		integration.EmitDailySummary(ctx, "2024-01-15", 10, 7, 3, 1500.00, 15.0, 70.0)
	})
}

func TestIntegration_CreateSafetyGuardCallback(t *testing.T) {
	emailBackend, _ := NewEmailBackend(EmailConfig{}, nil)
	slackBackend, _ := NewSlackBackend(SlackConfig{})

	config := DefaultManagerConfig()
	manager := NewManager(config, emailBackend, slackBackend)
	integration := NewIntegration(manager)

	callback := integration.CreateSafetyGuardCallback()

	assert.NotNil(t, callback)

	// Test with a mock event (wrong type should not panic)
	assert.NotPanics(t, func() {
		callback(struct{}{})
	})
}
