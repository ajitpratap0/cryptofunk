package notifications

import (
	"time"
)

// NotificationType represents different types of notifications
type NotificationType string

const (
	NotificationTypeTradeExecution   NotificationType = "trade_execution"
	NotificationTypePnLAlert         NotificationType = "pnl_alert"
	NotificationTypeCircuitBreaker   NotificationType = "circuit_breaker"
	NotificationTypeConsensusFailure NotificationType = "consensus_failure"
)

// Platform represents the device platform
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
)

// Notification represents a push notification to be sent
type Notification struct {
	Type     NotificationType  `json:"type"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Data     map[string]string `json:"data,omitempty"`
	Priority string            `json:"priority,omitempty"` // "high" or "normal"
}

// Preferences represents user notification preferences
type Preferences struct {
	TradeExecutions   bool `json:"trade_executions"`
	PnLAlerts         bool `json:"pnl_alerts"`
	CircuitBreaker    bool `json:"circuit_breaker"`
	ConsensusFailures bool `json:"consensus_failures"`
}

// DefaultPreferences returns the default notification preferences
func DefaultPreferences() Preferences {
	return Preferences{
		TradeExecutions:   true,
		PnLAlerts:         true,
		CircuitBreaker:    true,
		ConsensusFailures: true,
	}
}

// IsEnabled checks if a specific notification type is enabled
func (p Preferences) IsEnabled(notifType NotificationType) bool {
	switch notifType {
	case NotificationTypeTradeExecution:
		return p.TradeExecutions
	case NotificationTypePnLAlert:
		return p.PnLAlerts
	case NotificationTypeCircuitBreaker:
		return p.CircuitBreaker
	case NotificationTypeConsensusFailure:
		return p.ConsensusFailures
	default:
		return false
	}
}

// Device represents a user device for push notifications
type Device struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	DeviceToken string    `json:"device_token"`
	Platform    Platform  `json:"platform"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

// NotificationLog represents a logged notification
type NotificationLog struct {
	ID               string            `json:"id"`
	UserID           string            `json:"user_id"`
	DeviceToken      string            `json:"device_token,omitempty"`
	NotificationType NotificationType  `json:"notification_type"`
	Title            string            `json:"title"`
	Body             string            `json:"body"`
	Data             map[string]string `json:"data,omitempty"`
	Status           string            `json:"status"` // pending, sent, failed
	ErrorMessage     string            `json:"error_message,omitempty"`
	SentAt           time.Time         `json:"sent_at"`
}

// NotificationStatus represents the status of a sent notification
type NotificationStatus string

const (
	NotificationStatusPending NotificationStatus = "pending"
	NotificationStatusSent    NotificationStatus = "sent"
	NotificationStatusFailed  NotificationStatus = "failed"
)

// TradeNotificationData creates data payload for trade execution notifications
func TradeNotificationData(orderID, symbol, side string, quantity, price float64) map[string]string {
	return map[string]string{
		"order_id": orderID,
		"symbol":   symbol,
		"side":     side,
		"quantity": formatFloat(quantity),
		"price":    formatFloat(price),
	}
}

// PnLNotificationData creates data payload for P&L alerts
func PnLNotificationData(sessionID string, pnlPercent, pnlAmount float64) map[string]string {
	return map[string]string{
		"session_id":  sessionID,
		"pnl_percent": formatFloat(pnlPercent),
		"pnl_amount":  formatFloat(pnlAmount),
	}
}

// CircuitBreakerNotificationData creates data payload for circuit breaker alerts
func CircuitBreakerNotificationData(reason string, threshold float64) map[string]string {
	return map[string]string{
		"reason":    reason,
		"threshold": formatFloat(threshold),
	}
}

// ConsensusFailureNotificationData creates data payload for consensus failure alerts
func ConsensusFailureNotificationData(symbol, reason string, agentCount int) map[string]string {
	return map[string]string{
		"symbol":      symbol,
		"reason":      reason,
		"agent_count": formatInt(agentCount),
	}
}

// Helper functions for formatting
func formatFloat(f float64) string {
	return formatFloatPrec(f, 2)
}

func formatFloatPrec(f float64, precision int) string {
	switch precision {
	case 2:
		return formatFloatPrec2(f)
	case 8:
		return formatFloatPrec8(f)
	default:
		return formatFloatPrec2(f)
	}
}

func formatFloatPrec2(f float64) string {
	s := ""
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}

	whole := int64(f)
	frac := int64((f-float64(whole))*100 + 0.5)
	if frac >= 100 {
		whole++
		frac -= 100
	}

	s = sign + formatInt(int(whole)) + "."
	if frac < 10 {
		s += "0"
	}
	s += formatInt(int(frac))
	return s
}

func formatFloatPrec8(f float64) string {
	if f == float64(int64(f)) {
		return formatInt(int(f))
	}
	s := ""
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}

	whole := int64(f)
	frac := int64((f-float64(whole))*100000000 + 0.5)
	if frac >= 100000000 {
		whole++
		frac -= 100000000
	}

	s = sign + formatInt(int(whole)) + "."
	fracStr := formatInt(int(frac))
	for len(fracStr) < 8 {
		fracStr = "0" + fracStr
	}
	s += fracStr
	return s
}

func formatInt(i int) string {
	if i == 0 {
		return "0"
	}

	s := ""
	neg := i < 0
	if neg {
		i = -i
	}

	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}

	if neg {
		s = "-" + s
	}
	return s
}
