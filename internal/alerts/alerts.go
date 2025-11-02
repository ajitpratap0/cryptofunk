package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Severity levels for alerts
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// Alert represents an alert message
type Alert struct {
	Title     string
	Message   string
	Severity  Severity
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// Alerter defines the interface for sending alerts
type Alerter interface {
	Send(ctx context.Context, alert Alert) error
}

// Manager manages multiple alert channels
type Manager struct {
	alerters []Alerter
}

// NewManager creates a new alert manager
func NewManager(alerters ...Alerter) *Manager {
	return &Manager{
		alerters: alerters,
	}
}

// Send sends an alert to all configured alerters
func (m *Manager) Send(ctx context.Context, alert Alert) error {
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	var lastErr error
	for _, alerter := range m.alerters {
		if err := alerter.Send(ctx, alert); err != nil {
			log.Error().
				Err(err).
				Str("title", alert.Title).
				Msg("Failed to send alert")
			lastErr = err
		}
	}

	return lastErr
}

// SendCritical is a convenience method for sending critical alerts
func (m *Manager) SendCritical(ctx context.Context, title, message string, metadata map[string]interface{}) error {
	return m.Send(ctx, Alert{
		Title:    title,
		Message:  message,
		Severity: SeverityCritical,
		Metadata: metadata,
	})
}

// SendWarning is a convenience method for sending warning alerts
func (m *Manager) SendWarning(ctx context.Context, title, message string, metadata map[string]interface{}) error {
	return m.Send(ctx, Alert{
		Title:    title,
		Message:  message,
		Severity: SeverityWarning,
		Metadata: metadata,
	})
}

// SendInfo is a convenience method for sending info alerts
func (m *Manager) SendInfo(ctx context.Context, title, message string, metadata map[string]interface{}) error {
	return m.Send(ctx, Alert{
		Title:    title,
		Message:  message,
		Severity: SeverityInfo,
		Metadata: metadata,
	})
}

// LogAlerter logs alerts using zerolog
type LogAlerter struct{}

// NewLogAlerter creates a new log-based alerter
func NewLogAlerter() *LogAlerter {
	return &LogAlerter{}
}

// Send sends an alert by logging it
func (l *LogAlerter) Send(ctx context.Context, alert Alert) error {
	event := log.Log()

	// Set log level based on severity
	switch alert.Severity {
	case SeverityCritical:
		event = log.Error()
	case SeverityWarning:
		event = log.Warn()
	case SeverityInfo:
		event = log.Info()
	}

	// Add metadata fields
	if alert.Metadata != nil {
		for key, value := range alert.Metadata {
			event = event.Interface(key, value)
		}
	}

	event.
		Str("alert_title", alert.Title).
		Str("alert_severity", string(alert.Severity)).
		Time("alert_time", alert.Timestamp).
		Msg(fmt.Sprintf("🚨 ALERT: %s", alert.Message))

	return nil
}

// ConsoleAlerter prints alerts to console with prominent formatting
type ConsoleAlerter struct{}

// NewConsoleAlerter creates a new console-based alerter
func NewConsoleAlerter() *ConsoleAlerter {
	return &ConsoleAlerter{}
}

// Send sends an alert by printing to console
func (c *ConsoleAlerter) Send(ctx context.Context, alert Alert) error {
	banner := ""
	switch alert.Severity {
	case SeverityCritical:
		banner = "🚨🚨🚨 CRITICAL ALERT 🚨🚨🚨"
	case SeverityWarning:
		banner = "⚠️  WARNING ALERT ⚠️"
	case SeverityInfo:
		banner = "ℹ️  INFO ALERT ℹ️"
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(banner)
	fmt.Println("========================================")
	fmt.Printf("Title: %s\n", alert.Title)
	fmt.Printf("Message: %s\n", alert.Message)
	fmt.Printf("Severity: %s\n", alert.Severity)
	fmt.Printf("Time: %s\n", alert.Timestamp.Format(time.RFC3339))

	if alert.Metadata != nil && len(alert.Metadata) > 0 {
		fmt.Println("Metadata:")
		for key, value := range alert.Metadata {
			fmt.Printf("  - %s: %v\n", key, value)
		}
	}

	fmt.Println("========================================")
	fmt.Println()

	return nil
}

// Default global alert manager (can be replaced with custom configuration)
var defaultManager *Manager

func init() {
	// Initialize with log and console alerters by default
	defaultManager = NewManager(
		NewLogAlerter(),
		NewConsoleAlerter(),
	)
}

// GetDefaultManager returns the default alert manager
func GetDefaultManager() *Manager {
	return defaultManager
}

// SetDefaultManager sets the default alert manager
func SetDefaultManager(manager *Manager) {
	defaultManager = manager
}

// Helper functions for common alerts

// AlertOrderFailed sends an alert for order placement failure
func AlertOrderFailed(ctx context.Context, symbol, side string, quantity float64, err error) {
	defaultManager.SendCritical(ctx, "Order Placement Failed", fmt.Sprintf(
		"Failed to place %s order for %s: %v", side, symbol, err,
	), map[string]interface{}{
		"symbol":   symbol,
		"side":     side,
		"quantity": quantity,
		"error":    err.Error(),
	})
}

// AlertOrderCancelFailed sends an alert for order cancellation failure
func AlertOrderCancelFailed(ctx context.Context, orderID, symbol string, err error) {
	defaultManager.SendWarning(ctx, "Order Cancellation Failed", fmt.Sprintf(
		"Failed to cancel order %s for %s: %v", orderID, symbol, err,
	), map[string]interface{}{
		"order_id": orderID,
		"symbol":   symbol,
		"error":    err.Error(),
	})
}

// AlertConnectionError sends an alert for exchange connection issues
func AlertConnectionError(ctx context.Context, exchange string, err error) {
	defaultManager.SendCritical(ctx, "Exchange Connection Error", fmt.Sprintf(
		"Lost connection to %s: %v", exchange, err,
	), map[string]interface{}{
		"exchange": exchange,
		"error":    err.Error(),
	})
}

// AlertPositionRisk sends an alert for position risk violations
func AlertPositionRisk(ctx context.Context, symbol string, riskLevel float64, reason string) {
	defaultManager.SendCritical(ctx, "Position Risk Alert", fmt.Sprintf(
		"High risk detected for %s position: %s (risk level: %.2f)", symbol, reason, riskLevel,
	), map[string]interface{}{
		"symbol":     symbol,
		"risk_level": riskLevel,
		"reason":     reason,
	})
}

// AlertSystemError sends an alert for critical system errors
func AlertSystemError(ctx context.Context, component string, err error) {
	defaultManager.SendCritical(ctx, "System Error", fmt.Sprintf(
		"Critical error in %s: %v", component, err,
	), map[string]interface{}{
		"component": component,
		"error":     err.Error(),
	})
}
