package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

// EventType represents the type of audit event
type EventType string

const (
	// Authentication events
	EventTypeLogin          EventType = "LOGIN"
	EventTypeLogout         EventType = "LOGOUT"
	EventTypeLoginFailed    EventType = "LOGIN_FAILED"
	EventTypePasswordChange EventType = "PASSWORD_CHANGE"

	// Trading control events
	EventTypeTradingStart  EventType = "TRADING_START"
	EventTypeTradingStop   EventType = "TRADING_STOP"
	EventTypeTradingPause  EventType = "TRADING_PAUSE"
	EventTypeTradingResume EventType = "TRADING_RESUME"

	// Order events
	EventTypeOrderPlaced   EventType = "ORDER_PLACED"
	EventTypeOrderCanceled EventType = "ORDER_CANCELED"
	EventTypeOrderFilled   EventType = "ORDER_FILLED"

	// Configuration events
	EventTypeConfigUpdated EventType = "CONFIG_UPDATED"
	EventTypeConfigViewed  EventType = "CONFIG_VIEWED"

	// Strategy events
	EventTypeStrategyUpdated  EventType = "STRATEGY_UPDATED"
	EventTypeStrategyImported EventType = "STRATEGY_IMPORTED"
	EventTypeStrategyExported EventType = "STRATEGY_EXPORTED"
	EventTypeStrategyCloned   EventType = "STRATEGY_CLONED"
	EventTypeStrategyMerged   EventType = "STRATEGY_MERGED"

	// Agent events
	EventTypeAgentStarted EventType = "AGENT_STARTED"
	EventTypeAgentStopped EventType = "AGENT_STOPPED"
	EventTypeAgentFailed  EventType = "AGENT_FAILED"

	// Security events
	EventTypeRateLimitExceeded  EventType = "RATE_LIMIT_EXCEEDED"
	EventTypeUnauthorizedAccess EventType = "UNAUTHORIZED_ACCESS"
	EventTypeInvalidInput       EventType = "INVALID_INPUT"

	// Data access events
	EventTypeDataExport EventType = "DATA_EXPORT"
	EventTypeDataDelete EventType = "DATA_DELETE"
)

// Severity represents the severity level of an audit event
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityError    Severity = "ERROR"
	SeverityCritical Severity = "CRITICAL"
)

// Event represents a single audit log event
type Event struct {
	ID        uuid.UUID              `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	EventType EventType              `json:"event_type"`
	Severity  Severity               `json:"severity"`
	UserID    string                 `json:"user_id,omitempty"`       // User/API key if authenticated
	IPAddress string                 `json:"ip_address"`              // Client IP
	UserAgent string                 `json:"user_agent,omitempty"`    // Browser/client info
	Resource  string                 `json:"resource,omitempty"`      // Affected resource (order ID, session ID, etc.)
	Action    string                 `json:"action"`                  // Human-readable action description
	Success   bool                   `json:"success"`                 // Whether action succeeded
	ErrorMsg  string                 `json:"error_message,omitempty"` // Error if failed
	Metadata  map[string]interface{} `json:"metadata,omitempty"`      // Additional context
	RequestID string                 `json:"request_id,omitempty"`    // Request correlation ID
	Duration  int64                  `json:"duration_ms,omitempty"`   // Action duration in ms
}

// Logger handles audit logging operations
type Logger struct {
	db      *pgxpool.Pool
	enabled bool
}

// NewLogger creates a new audit logger
func NewLogger(db *pgxpool.Pool, enabled bool) *Logger {
	return &Logger{
		db:      db,
		enabled: enabled,
	}
}

// Log records an audit event
func (l *Logger) Log(ctx context.Context, event *Event) error {
	if !l.enabled {
		return nil
	}

	start := time.Now()

	// Set defaults
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Log to structured logger for immediate visibility
	logEvent := log.With().
		Str("event_id", event.ID.String()).
		Str("event_type", string(event.EventType)).
		Str("severity", string(event.Severity)).
		Str("user_id", event.UserID).
		Str("ip_address", event.IPAddress).
		Str("resource", event.Resource).
		Str("action", event.Action).
		Bool("success", event.Success).
		Logger()

	if event.ErrorMsg != "" {
		logEvent = logEvent.With().Str("error", event.ErrorMsg).Logger()
	}

	if event.Duration > 0 {
		logEvent = logEvent.With().Int64("duration_ms", event.Duration).Logger()
	}

	// Log at appropriate level
	switch event.Severity {
	case SeverityCritical, SeverityError:
		logEvent.Error().Msg("Audit event")
	case SeverityWarning:
		logEvent.Warn().Msg("Audit event")
	default:
		logEvent.Info().Msg("Audit event")
	}

	// Persist to database if pool is available
	if l.db != nil {
		if err := l.persistEvent(ctx, event); err != nil {
			// Record failure metrics
			durationMs := float64(time.Since(start).Milliseconds())
			metrics.RecordAuditLog(string(event.EventType), false, durationMs)
			metrics.RecordAuditLogFailure("persist_error", string(event.EventType))
			return err
		}
	}

	// Record success metrics
	durationMs := float64(time.Since(start).Milliseconds())
	metrics.RecordAuditLog(string(event.EventType), true, durationMs)

	return nil
}

// persistEvent stores the audit event in the database
func (l *Logger) persistEvent(ctx context.Context, event *Event) error {
	query := `
		INSERT INTO audit_logs (
			id, timestamp, event_type, severity, user_id, ip_address,
			user_agent, resource, action, success, error_message,
			metadata, request_id, duration_ms
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	// Convert metadata to JSON
	var metadataJSON []byte
	var err error
	if event.Metadata != nil {
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal audit event metadata")
			metadataJSON = []byte("{}")
		}
	}

	_, err = l.db.Exec(ctx, query,
		event.ID,
		event.Timestamp,
		event.EventType,
		event.Severity,
		event.UserID,
		event.IPAddress,
		event.UserAgent,
		event.Resource,
		event.Action,
		event.Success,
		event.ErrorMsg,
		metadataJSON,
		event.RequestID,
		event.Duration,
	)

	if err != nil {
		log.Error().Err(err).
			Str("event_id", event.ID.String()).
			Str("event_type", string(event.EventType)).
			Msg("Failed to persist audit event to database")
		return err
	}

	return nil
}

// Query retrieves audit events based on filters
func (l *Logger) Query(ctx context.Context, filters *QueryFilters) ([]Event, error) {
	if l.db == nil {
		return nil, nil
	}

	query := `
		SELECT
			id, timestamp, event_type, severity, user_id, ip_address,
			user_agent, resource, action, success, error_message,
			metadata, request_id, duration_ms
		FROM audit_logs
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	// Apply filters
	if filters.EventType != "" {
		query += ` AND event_type = $` + string(rune('0'+argPos))
		args = append(args, filters.EventType)
		argPos++
	}

	if filters.UserID != "" {
		query += ` AND user_id = $` + string(rune('0'+argPos))
		args = append(args, filters.UserID)
		argPos++
	}

	if filters.IPAddress != "" {
		query += ` AND ip_address = $` + string(rune('0'+argPos))
		args = append(args, filters.IPAddress)
		argPos++
	}

	if !filters.StartTime.IsZero() {
		query += ` AND timestamp >= $` + string(rune('0'+argPos))
		args = append(args, filters.StartTime)
		argPos++
	}

	if !filters.EndTime.IsZero() {
		query += ` AND timestamp <= $` + string(rune('0'+argPos))
		args = append(args, filters.EndTime)
		argPos++
	}

	if filters.Success != nil {
		query += ` AND success = $` + string(rune('0'+argPos))
		args = append(args, *filters.Success)
		argPos++
	}

	// Order by timestamp descending
	query += ` ORDER BY timestamp DESC`

	// Apply limit
	if filters.Limit > 0 {
		query += ` LIMIT $` + string(rune('0'+argPos))
		args = append(args, filters.Limit)
	}

	rows, err := l.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var event Event
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.Timestamp,
			&event.EventType,
			&event.Severity,
			&event.UserID,
			&event.IPAddress,
			&event.UserAgent,
			&event.Resource,
			&event.Action,
			&event.Success,
			&event.ErrorMsg,
			&metadataJSON,
			&event.RequestID,
			&event.Duration,
		)
		if err != nil {
			return nil, err
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				log.Warn().Err(err).Msg("Failed to unmarshal audit event metadata")
			}
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// QueryFilters defines filters for querying audit events
type QueryFilters struct {
	EventType EventType
	UserID    string
	IPAddress string
	StartTime time.Time
	EndTime   time.Time
	Success   *bool
	Limit     int
}

// Helper functions for common audit events

// LogTradingAction logs a trading control action (start/stop/pause/resume)
func (l *Logger) LogTradingAction(ctx context.Context, eventType EventType, userID, ipAddress, sessionID string, success bool, errorMsg string) error {
	return l.Log(ctx, &Event{
		EventType: eventType,
		Severity:  SeverityInfo,
		UserID:    userID,
		IPAddress: ipAddress,
		Resource:  sessionID,
		Action:    string(eventType),
		Success:   success,
		ErrorMsg:  errorMsg,
	})
}

// LogOrderAction logs an order-related action (place/cancel/fill)
func (l *Logger) LogOrderAction(ctx context.Context, eventType EventType, userID, ipAddress, orderID string, metadata map[string]interface{}, success bool, errorMsg string) error {
	severity := SeverityInfo
	if !success {
		severity = SeverityWarning
	}

	return l.Log(ctx, &Event{
		EventType: eventType,
		Severity:  severity,
		UserID:    userID,
		IPAddress: ipAddress,
		Resource:  orderID,
		Action:    string(eventType),
		Success:   success,
		ErrorMsg:  errorMsg,
		Metadata:  metadata,
	})
}

// LogSecurityEvent logs a security-related event (rate limit, unauthorized access, etc.)
func (l *Logger) LogSecurityEvent(ctx context.Context, eventType EventType, userID, ipAddress, resource, action string, metadata map[string]interface{}) error {
	return l.Log(ctx, &Event{
		EventType: eventType,
		Severity:  SeverityWarning,
		UserID:    userID,
		IPAddress: ipAddress,
		Resource:  resource,
		Action:    action,
		Success:   false,
		Metadata:  metadata,
	})
}

// LogConfigChange logs a configuration change
func (l *Logger) LogConfigChange(ctx context.Context, userID, ipAddress, configKey string, oldValue, newValue interface{}, success bool, errorMsg string) error {
	metadata := map[string]interface{}{
		"config_key": configKey,
		"old_value":  oldValue,
		"new_value":  newValue,
	}

	severity := SeverityInfo
	if !success {
		severity = SeverityError
	}

	return l.Log(ctx, &Event{
		EventType: EventTypeConfigUpdated,
		Severity:  severity,
		UserID:    userID,
		IPAddress: ipAddress,
		Resource:  configKey,
		Action:    "Configuration updated",
		Success:   success,
		ErrorMsg:  errorMsg,
		Metadata:  metadata,
	})
}

// LogStrategyChange logs a strategy modification event
func (l *Logger) LogStrategyChange(ctx context.Context, eventType EventType, userID, ipAddress, strategyID, strategyName string, metadata map[string]interface{}, success bool, errorMsg string) error {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["strategy_id"] = strategyID
	metadata["strategy_name"] = strategyName

	severity := SeverityInfo
	if !success {
		severity = SeverityError
	}

	action := "Strategy operation"
	switch eventType {
	case EventTypeStrategyUpdated:
		action = "Strategy updated"
	case EventTypeStrategyImported:
		action = "Strategy imported"
	case EventTypeStrategyExported:
		action = "Strategy exported"
	case EventTypeStrategyCloned:
		action = "Strategy cloned"
	case EventTypeStrategyMerged:
		action = "Strategies merged"
	}

	return l.Log(ctx, &Event{
		EventType: eventType,
		Severity:  severity,
		UserID:    userID,
		IPAddress: ipAddress,
		Resource:  strategyID,
		Action:    action,
		Success:   success,
		ErrorMsg:  errorMsg,
		Metadata:  metadata,
	})
}
