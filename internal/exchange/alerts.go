package exchange

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "CRITICAL" // System-breaking errors requiring immediate attention
	AlertSeverityWarning  AlertSeverity = "WARNING"  // Important errors that should be investigated
	AlertSeverityInfo     AlertSeverity = "INFO"     // Informational alerts for tracking
)

// AlertCategory represents the category of an alert
type AlertCategory string

const (
	AlertCategoryOrderPlacement AlertCategory = "ORDER_PLACEMENT"
	AlertCategoryOrderCancel    AlertCategory = "ORDER_CANCEL"
	AlertCategoryOrderQuery     AlertCategory = "ORDER_QUERY"
	AlertCategoryPosition       AlertCategory = "POSITION"
	AlertCategorySession        AlertCategory = "SESSION"
	AlertCategoryDatabase       AlertCategory = "DATABASE"
	AlertCategoryExchange       AlertCategory = "EXCHANGE"
	AlertCategoryRateLimit      AlertCategory = "RATE_LIMIT"
	AlertCategoryNetwork        AlertCategory = "NETWORK"
)

// Alert represents an error alert with structured data
type Alert struct {
	Severity  AlertSeverity          `json:"severity"`
	Category  AlertCategory          `json:"category"`
	Message   string                 `json:"message"`
	Error     error                  `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// AlertManager handles error alerting and logging
type AlertManager struct {
	// In the future, this could include integrations with:
	// - Prometheus for metrics
	// - PagerDuty for on-call alerts
	// - Slack/Discord for team notifications
	// - Email for critical failures
	// - NATS for event streaming
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{}
}

// SendAlert logs and potentially forwards an alert
func (am *AlertManager) SendAlert(ctx context.Context, alert Alert) {
	// Set timestamp if not provided
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	// Build structured log event
	logEvent := log.With().
		Str("severity", string(alert.Severity)).
		Str("category", string(alert.Category)).
		Time("timestamp", alert.Timestamp)

	// Add context fields
	if alert.Context != nil {
		for key, value := range alert.Context {
			logEvent = logEvent.Interface(key, value)
		}
	}

	// Add error if present
	if alert.Error != nil {
		logEvent = logEvent.Err(alert.Error)
	}

	logger := logEvent.Logger()

	// Log based on severity
	switch alert.Severity {
	case AlertSeverityCritical:
		logger.Error().Msg(alert.Message)
		// Future: Send to PagerDuty, trigger immediate notification
		// Future: Increment Prometheus critical_alerts counter
	case AlertSeverityWarning:
		logger.Warn().Msg(alert.Message)
		// Future: Send to Slack, log to metrics
		// Future: Increment Prometheus warning_alerts counter
	case AlertSeverityInfo:
		logger.Info().Msg(alert.Message)
		// Future: Log to metrics only
		// Future: Increment Prometheus info_alerts counter
	default:
		logger.Error().Msg(alert.Message)
	}

	// Future: Publish to NATS for event streaming
	// Future: Store in time-series database for analysis
	// Future: Trigger automated responses (e.g., circuit breakers)
}

// Helper functions for common alert scenarios

// AlertOrderPlacementFailed creates an alert for order placement failures
func AlertOrderPlacementFailed(err error, symbol string, side OrderSide, quantity float64, orderType OrderType) Alert {
	severity := AlertSeverityCritical
	// Downgrade to warning if it's a validation error
	if IsRetryable(err) {
		severity = AlertSeverityWarning
	}

	return Alert{
		Severity: severity,
		Category: AlertCategoryOrderPlacement,
		Message:  "Failed to place order",
		Error:    err,
		Context: map[string]interface{}{
			"symbol":     symbol,
			"side":       string(side),
			"quantity":   quantity,
			"order_type": string(orderType),
		},
	}
}

// AlertOrderCancellationFailed creates an alert for order cancellation failures
func AlertOrderCancellationFailed(err error, orderID string) Alert {
	severity := AlertSeverityWarning
	// Upgrade to critical if it's a non-retryable error
	if !IsRetryable(err) {
		severity = AlertSeverityCritical
	}

	return Alert{
		Severity: severity,
		Category: AlertCategoryOrderCancel,
		Message:  "Failed to cancel order",
		Error:    err,
		Context: map[string]interface{}{
			"order_id": orderID,
		},
	}
}

// AlertOrderQueryFailed creates an alert for order query failures
func AlertOrderQueryFailed(err error, orderID string) Alert {
	return Alert{
		Severity: AlertSeverityWarning,
		Category: AlertCategoryOrderQuery,
		Message:  "Failed to query order status",
		Error:    err,
		Context: map[string]interface{}{
			"order_id": orderID,
		},
	}
}

// AlertPositionUpdateFailed creates an alert for position update failures
func AlertPositionUpdateFailed(err error, symbol string, operation string) Alert {
	return Alert{
		Severity: AlertSeverityCritical,
		Category: AlertCategoryPosition,
		Message:  "Failed to update position",
		Error:    err,
		Context: map[string]interface{}{
			"symbol":    symbol,
			"operation": operation,
		},
	}
}

// AlertDatabaseError creates an alert for database errors
func AlertDatabaseError(err error, operation string, details map[string]interface{}) Alert {
	return Alert{
		Severity: AlertSeverityCritical,
		Category: AlertCategoryDatabase,
		Message:  "Database operation failed",
		Error:    err,
		Context: map[string]interface{}{
			"operation": operation,
			"details":   details,
		},
	}
}

// AlertRateLimitExceeded creates an alert for rate limit errors
func AlertRateLimitExceeded(err error, endpoint string) Alert {
	return Alert{
		Severity: AlertSeverityWarning,
		Category: AlertCategoryRateLimit,
		Message:  "Rate limit exceeded",
		Error:    err,
		Context: map[string]interface{}{
			"endpoint": endpoint,
		},
	}
}

// AlertNetworkError creates an alert for network errors
func AlertNetworkError(err error, operation string) Alert {
	return Alert{
		Severity: AlertSeverityWarning,
		Category: AlertCategoryNetwork,
		Message:  "Network error occurred",
		Error:    err,
		Context: map[string]interface{}{
			"operation": operation,
		},
	}
}

// AlertSessionCreationFailed creates an alert for session creation failures
func AlertSessionCreationFailed(err error, symbol string, initialCapital float64) Alert {
	return Alert{
		Severity: AlertSeverityCritical,
		Category: AlertCategorySession,
		Message:  "Failed to create trading session",
		Error:    err,
		Context: map[string]interface{}{
			"symbol":          symbol,
			"initial_capital": initialCapital,
		},
	}
}

// AlertExchangeConnectionFailed creates an alert for exchange connection failures
func AlertExchangeConnectionFailed(err error, exchange string) Alert {
	return Alert{
		Severity: AlertSeverityCritical,
		Category: AlertCategoryExchange,
		Message:  "Failed to connect to exchange",
		Error:    err,
		Context: map[string]interface{}{
			"exchange": exchange,
		},
	}
}
