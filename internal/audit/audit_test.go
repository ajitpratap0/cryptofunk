package audit

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEvent_Defaults(t *testing.T) {
	event := &Event{
		EventType: EventTypeTradingStart,
		Severity:  SeverityInfo,
		IPAddress: "192.168.1.1",
		Action:    "Start trading session",
		Success:   true,
	}

	// ID and timestamp should be set by the logger
	assert.Equal(t, uuid.Nil, event.ID)
	assert.True(t, event.Timestamp.IsZero())
}

func TestLogger_LogWithoutDatabase(t *testing.T) {
	// Create logger without database connection
	logger := NewLogger(nil, true)

	event := &Event{
		EventType: EventTypeTradingStart,
		Severity:  SeverityInfo,
		UserID:    "user123",
		IPAddress: "192.168.1.1",
		Action:    "Start trading session",
		Success:   true,
	}

	// Should not error even without database
	err := logger.Log(context.Background(), event)
	assert.NoError(t, err)

	// ID and timestamp should be set
	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.False(t, event.Timestamp.IsZero())
}

func TestLogger_Disabled(t *testing.T) {
	// Create disabled logger
	logger := NewLogger(nil, false)

	event := &Event{
		EventType: EventTypeTradingStart,
		Severity:  SeverityInfo,
		IPAddress: "192.168.1.1",
		Action:    "Start trading session",
		Success:   true,
	}

	// Should be no-op when disabled
	err := logger.Log(context.Background(), event)
	assert.NoError(t, err)
}

func TestLogger_LogTradingAction(t *testing.T) {
	logger := NewLogger(nil, true)

	err := logger.LogTradingAction(
		context.Background(),
		EventTypeTradingStart,
		"user123",
		"192.168.1.1",
		"session-456",
		true,
		"",
	)

	assert.NoError(t, err)
}

func TestLogger_LogOrderAction(t *testing.T) {
	logger := NewLogger(nil, true)

	metadata := map[string]interface{}{
		"symbol":   "BTC/USDT",
		"quantity": 0.1,
		"price":    50000.0,
	}

	err := logger.LogOrderAction(
		context.Background(),
		EventTypeOrderPlaced,
		"user123",
		"192.168.1.1",
		"order-789",
		metadata,
		true,
		"",
	)

	assert.NoError(t, err)
}

func TestLogger_LogSecurityEvent(t *testing.T) {
	logger := NewLogger(nil, true)

	metadata := map[string]interface{}{
		"attempts": 5,
		"endpoint": "/api/v1/trade/start",
	}

	err := logger.LogSecurityEvent(
		context.Background(),
		EventTypeRateLimitExceeded,
		"",
		"192.168.1.1",
		"/api/v1/trade/start",
		"Rate limit exceeded",
		metadata,
	)

	assert.NoError(t, err)
}

func TestLogger_LogConfigChange(t *testing.T) {
	logger := NewLogger(nil, true)

	err := logger.LogConfigChange(
		context.Background(),
		"admin",
		"192.168.1.1",
		"max_position_size",
		1000.0,
		2000.0,
		true,
		"",
	)

	assert.NoError(t, err)
}

func TestQueryFilters(t *testing.T) {
	filters := &QueryFilters{
		EventType: EventTypeTradingStart,
		UserID:    "user123",
		IPAddress: "192.168.1.1",
		StartTime: time.Now().Add(-24 * time.Hour),
		EndTime:   time.Now(),
		Success:   boolPtr(true),
		Limit:     100,
	}

	assert.Equal(t, EventTypeTradingStart, filters.EventType)
	assert.Equal(t, "user123", filters.UserID)
	assert.Equal(t, "192.168.1.1", filters.IPAddress)
	assert.NotNil(t, filters.Success)
	assert.True(t, *filters.Success)
	assert.Equal(t, 100, filters.Limit)
}

func TestEventTypes(t *testing.T) {
	// Test that event types are unique strings
	types := []EventType{
		EventTypeLogin,
		EventTypeLogout,
		EventTypeLoginFailed,
		EventTypeTradingStart,
		EventTypeTradingStop,
		EventTypeOrderPlaced,
		EventTypeConfigUpdated,
		EventTypeRateLimitExceeded,
	}

	seen := make(map[EventType]bool)
	for _, et := range types {
		assert.False(t, seen[et], "Duplicate event type: %s", et)
		assert.NotEmpty(t, string(et), "Event type should not be empty")
		seen[et] = true
	}
}

func TestSeverityLevels(t *testing.T) {
	// Test severity levels
	severities := []Severity{
		SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityCritical,
	}

	for _, s := range severities {
		assert.NotEmpty(t, string(s), "Severity should not be empty")
	}
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
