package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/audit"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditLogger_PersistEvent tests that audit events are persisted to the database
func TestAuditLogger_PersistEvent(t *testing.T) {
	// Setup testcontainer database
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Create audit logger with database
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create test event
	event := &audit.Event{
		EventType: audit.EventTypeTradingStart,
		Severity:  audit.SeverityInfo,
		UserID:    "user123",
		IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		Resource:  "session-456",
		Action:    "Start trading session",
		Success:   true,
		RequestID: "req-789",
		Duration:  150,
		Metadata: map[string]interface{}{
			"symbol":  "BTC/USDT",
			"mode":    "LIVE",
			"balance": 10000.0,
		},
	}

	// Log the event
	err = logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify event was persisted by querying it back
	filters := &audit.QueryFilters{
		UserID: "user123",
		Limit:  10,
	}

	events, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Verify all fields were persisted correctly
	retrieved := events[0]
	assert.Equal(t, event.ID, retrieved.ID)
	assert.Equal(t, event.EventType, retrieved.EventType)
	assert.Equal(t, event.Severity, retrieved.Severity)
	assert.Equal(t, event.UserID, retrieved.UserID)
	assert.Equal(t, event.IPAddress, retrieved.IPAddress)
	assert.Equal(t, event.UserAgent, retrieved.UserAgent)
	assert.Equal(t, event.Resource, retrieved.Resource)
	assert.Equal(t, event.Action, retrieved.Action)
	assert.Equal(t, event.Success, retrieved.Success)
	assert.Equal(t, event.RequestID, retrieved.RequestID)
	assert.Equal(t, event.Duration, retrieved.Duration)

	// Verify metadata was persisted correctly
	assert.NotNil(t, retrieved.Metadata)
	assert.Equal(t, "BTC/USDT", retrieved.Metadata["symbol"])
	assert.Equal(t, "LIVE", retrieved.Metadata["mode"])
	assert.Equal(t, 10000.0, retrieved.Metadata["balance"])
}

// TestAuditLogger_PersistEventWithDefaults tests that ID and timestamp are auto-generated
func TestAuditLogger_PersistEventWithDefaults(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create event without ID or timestamp
	event := &audit.Event{
		EventType: audit.EventTypeTradingStop,
		Severity:  audit.SeverityInfo,
		IPAddress: "192.168.1.2",
		Action:    "Stop trading session",
		Success:   true,
	}

	// Verify defaults are not set yet
	assert.Equal(t, uuid.Nil, event.ID)
	assert.True(t, event.Timestamp.IsZero())

	// Log the event
	err = logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify defaults were set
	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.False(t, event.Timestamp.IsZero())

	// Verify persistence
	events, err := logger.Query(ctx, &audit.QueryFilters{Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, event.ID, events[0].ID)
}

// TestAuditLogger_QueryByEventType tests filtering by event type
func TestAuditLogger_QueryByEventType(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create multiple events of different types
	events := []*audit.Event{
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Start trading",
			Success:   true,
		},
		{
			EventType: audit.EventTypeTradingStop,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Stop trading",
			Success:   true,
		},
		{
			EventType: audit.EventTypeOrderPlaced,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Place order",
			Success:   true,
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.2",
			Action:    "Another start",
			Success:   true,
		},
	}

	for _, event := range events {
		err := logger.Log(ctx, event)
		require.NoError(t, err)
	}

	// Query for only TradingStart events
	filters := &audit.QueryFilters{
		EventType: audit.EventTypeTradingStart,
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Verify all results are TradingStart
	for _, result := range results {
		assert.Equal(t, audit.EventTypeTradingStart, result.EventType)
	}
}

// TestAuditLogger_QueryByUserID tests filtering by user ID
func TestAuditLogger_QueryByUserID(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create events for different users
	users := []string{"alice", "bob", "alice", "charlie", "alice"}
	for _, userID := range users {
		err := logger.Log(ctx, &audit.Event{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			UserID:    userID,
			IPAddress: "192.168.1.1",
			Action:    "Trading action",
			Success:   true,
		})
		require.NoError(t, err)
	}

	// Query for alice's events
	filters := &audit.QueryFilters{
		UserID: "alice",
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	for _, result := range results {
		assert.Equal(t, "alice", result.UserID)
	}
}

// TestAuditLogger_QueryByIPAddress tests filtering by IP address
func TestAuditLogger_QueryByIPAddress(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create events from different IPs
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.1", "10.0.0.1"}
	for _, ip := range ips {
		err := logger.Log(ctx, &audit.Event{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: ip,
			Action:    "Trading action",
			Success:   true,
		})
		require.NoError(t, err)
	}

	// Query for events from 192.168.1.1
	filters := &audit.QueryFilters{
		IPAddress: "192.168.1.1",
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	for _, result := range results {
		assert.Equal(t, "192.168.1.1", result.IPAddress)
	}
}

// TestAuditLogger_QueryByTimeRange tests filtering by time range
func TestAuditLogger_QueryByTimeRange(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	// Create events at different times
	events := []*audit.Event{
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Old event",
			Success:   true,
			Timestamp: twoDaysAgo,
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Yesterday event",
			Success:   true,
			Timestamp: yesterday,
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Today event",
			Success:   true,
			Timestamp: now,
		},
	}

	for _, event := range events {
		err := logger.Log(ctx, event)
		require.NoError(t, err)
	}

	// Query for events in the last 36 hours
	filters := &audit.QueryFilters{
		StartTime: now.Add(-36 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 2) // Should get yesterday and today, not two days ago
}

// TestAuditLogger_QueryBySuccess tests filtering by success/failure
func TestAuditLogger_QueryBySuccess(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create mix of successful and failed events
	successes := []bool{true, false, true, true, false}
	for _, success := range successes {
		errorMsg := ""
		if !success {
			errorMsg = "Operation failed"
		}
		err := logger.Log(ctx, &audit.Event{
			EventType: audit.EventTypeOrderPlaced,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Place order",
			Success:   success,
			ErrorMsg:  errorMsg,
		})
		require.NoError(t, err)
	}

	// Query for only successful events
	successFilter := true
	filters := &audit.QueryFilters{
		Success: &successFilter,
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	for _, result := range results {
		assert.True(t, result.Success)
		assert.Empty(t, result.ErrorMsg)
	}

	// Query for only failed events
	failureFilter := false
	filters = &audit.QueryFilters{
		Success: &failureFilter,
	}

	results, err = logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	for _, result := range results {
		assert.False(t, result.Success)
		assert.Equal(t, "Operation failed", result.ErrorMsg)
	}
}

// TestAuditLogger_QueryWithLimit tests query result limiting
func TestAuditLogger_QueryWithLimit(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create 10 events
	for i := 0; i < 10; i++ {
		err := logger.Log(ctx, &audit.Event{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Trading action",
			Success:   true,
		})
		require.NoError(t, err)
	}

	// Query with limit of 5
	filters := &audit.QueryFilters{
		Limit: 5,
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

// TestAuditLogger_QueryMultipleFilters tests combining multiple filters
func TestAuditLogger_QueryMultipleFilters(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	now := time.Now()

	// Create diverse set of events
	events := []*audit.Event{
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			UserID:    "alice",
			IPAddress: "192.168.1.1",
			Action:    "Start trading",
			Success:   true,
			Timestamp: now,
		},
		{
			EventType: audit.EventTypeTradingStop,
			Severity:  audit.SeverityInfo,
			UserID:    "alice",
			IPAddress: "192.168.1.1",
			Action:    "Stop trading",
			Success:   true,
			Timestamp: now,
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			UserID:    "bob",
			IPAddress: "192.168.1.1",
			Action:    "Start trading",
			Success:   true,
			Timestamp: now,
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			UserID:    "alice",
			IPAddress: "192.168.1.2",
			Action:    "Start trading",
			Success:   true,
			Timestamp: now,
		},
	}

	for _, event := range events {
		err := logger.Log(ctx, event)
		require.NoError(t, err)
	}

	// Query with multiple filters: EventType=TradingStart, UserID=alice, IPAddress=192.168.1.1
	filters := &audit.QueryFilters{
		EventType: audit.EventTypeTradingStart,
		UserID:    "alice",
		IPAddress: "192.168.1.1",
	}

	results, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	assert.Len(t, results, 1) // Only first event matches all filters

	result := results[0]
	assert.Equal(t, audit.EventTypeTradingStart, result.EventType)
	assert.Equal(t, "alice", result.UserID)
	assert.Equal(t, "192.168.1.1", result.IPAddress)
}

// TestAuditLogger_LogTradingAction_Integration tests the helper function with DB
func TestAuditLogger_LogTradingAction_Integration(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Log trading action
	err = logger.LogTradingAction(
		ctx,
		audit.EventTypeTradingStart,
		"user123",
		"192.168.1.1",
		"session-456",
		true,
		"",
	)
	require.NoError(t, err)

	// Verify it was persisted
	filters := &audit.QueryFilters{
		EventType: audit.EventTypeTradingStart,
	}

	events, err := logger.Query(ctx, filters)
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, audit.EventTypeTradingStart, event.EventType)
	assert.Equal(t, audit.SeverityInfo, event.Severity)
	assert.Equal(t, "user123", event.UserID)
	assert.Equal(t, "192.168.1.1", event.IPAddress)
	assert.Equal(t, "session-456", event.Resource)
	assert.True(t, event.Success)
}

// TestAuditLogger_LogOrderAction_Integration tests order logging with DB
func TestAuditLogger_LogOrderAction_Integration(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	metadata := map[string]interface{}{
		"symbol":   "BTC/USDT",
		"quantity": 0.5,
		"price":    45000.0,
	}

	// Log order action
	err = logger.LogOrderAction(
		ctx,
		audit.EventTypeOrderPlaced,
		"trader1",
		"10.0.0.1",
		"order-123",
		metadata,
		true,
		"",
	)
	require.NoError(t, err)

	// Verify metadata was persisted correctly
	events, err := logger.Query(ctx, &audit.QueryFilters{Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, "order-123", event.Resource)
	assert.NotNil(t, event.Metadata)
	assert.Equal(t, "BTC/USDT", event.Metadata["symbol"])
	assert.Equal(t, 0.5, event.Metadata["quantity"])
	assert.Equal(t, 45000.0, event.Metadata["price"])
}

// TestAuditLogger_LogSecurityEvent_Integration tests security event logging
func TestAuditLogger_LogSecurityEvent_Integration(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	metadata := map[string]interface{}{
		"attempts": 5,
		"endpoint": "/api/v1/orders",
	}

	// Log security event
	err = logger.LogSecurityEvent(
		ctx,
		audit.EventTypeRateLimitExceeded,
		"",
		"192.168.1.100",
		"/api/v1/orders",
		"Rate limit exceeded",
		metadata,
	)
	require.NoError(t, err)

	// Verify event was logged with warning severity
	events, err := logger.Query(ctx, &audit.QueryFilters{Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, audit.EventTypeRateLimitExceeded, event.EventType)
	assert.Equal(t, audit.SeverityWarning, event.Severity)
	assert.False(t, event.Success) // Security events are failures
	assert.Equal(t, "192.168.1.100", event.IPAddress)
}

// TestAuditLogger_LogConfigChange_Integration tests config change logging
func TestAuditLogger_LogConfigChange_Integration(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Log successful config change
	err = logger.LogConfigChange(
		ctx,
		"admin",
		"192.168.1.5",
		"max_position_size",
		1000.0,
		2000.0,
		true,
		"",
	)
	require.NoError(t, err)

	// Verify metadata contains old and new values
	events, err := logger.Query(ctx, &audit.QueryFilters{Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, audit.EventTypeConfigUpdated, event.EventType)
	assert.Equal(t, "max_position_size", event.Resource)
	assert.True(t, event.Success)
	assert.NotNil(t, event.Metadata)
	assert.Equal(t, "max_position_size", event.Metadata["config_key"])
	assert.Equal(t, 1000.0, event.Metadata["old_value"])
	assert.Equal(t, 2000.0, event.Metadata["new_value"])
}

// TestAuditLogger_QueryOrdering tests that events are returned in descending timestamp order
func TestAuditLogger_QueryOrdering(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	logger := audit.NewLogger(tc.DB.Pool(), true)

	// Create events with explicit timestamps
	now := time.Now()
	events := []*audit.Event{
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "First",
			Success:   true,
			Timestamp: now.Add(-3 * time.Minute),
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Second",
			Success:   true,
			Timestamp: now.Add(-2 * time.Minute),
		},
		{
			EventType: audit.EventTypeTradingStart,
			Severity:  audit.SeverityInfo,
			IPAddress: "192.168.1.1",
			Action:    "Third",
			Success:   true,
			Timestamp: now.Add(-1 * time.Minute),
		},
	}

	for _, event := range events {
		err := logger.Log(ctx, event)
		require.NoError(t, err)
	}

	// Query all events
	results, err := logger.Query(ctx, &audit.QueryFilters{})
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify descending order (most recent first)
	assert.Equal(t, "Third", results[0].Action)
	assert.Equal(t, "Second", results[1].Action)
	assert.Equal(t, "First", results[2].Action)
}
