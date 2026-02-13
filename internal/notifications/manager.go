package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Channel represents a notification channel
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSlack Channel = "slack"
)

// Outcome and priority constants
const (
	outcomeProfit  = "profit"
	outcomeLoss    = "loss"
	outcomeGain    = "gain"
	priorityHigh   = "high"
	priorityNormal = "normal"
)

// EventConfig maps event types to channels and priority settings
type EventConfig struct {
	Channels []Channel `mapstructure:"channels" json:"channels"`
	Priority string    `mapstructure:"priority" json:"priority"` // "high" or "normal"
	Enabled  bool      `mapstructure:"enabled" json:"enabled"`
}

// ManagerConfig holds configuration for the notification manager
type ManagerConfig struct {
	Enabled bool `mapstructure:"enabled" json:"enabled"`

	// Event routing configuration
	Events map[NotificationType]EventConfig `mapstructure:"events" json:"events"`

	// Default channels if event is not explicitly configured
	DefaultChannels []Channel `mapstructure:"default_channels" json:"default_channels"`
}

// DefaultManagerConfig returns sensible defaults for notification routing
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Enabled: true,
		Events: map[NotificationType]EventConfig{
			NotificationTypeTradeExecution: {
				Channels: []Channel{ChannelEmail, ChannelSlack},
				Priority: priorityHigh,
				Enabled:  true,
			},
			NotificationTypePositionClosed: {
				Channels: []Channel{ChannelSlack},
				Priority: "normal",
				Enabled:  true,
			},
			NotificationTypeSafetyGuard: {
				Channels: []Channel{ChannelEmail, ChannelSlack},
				Priority: priorityHigh,
				Enabled:  true,
			},
			NotificationTypeSystemError: {
				Channels: []Channel{ChannelEmail, ChannelSlack},
				Priority: priorityHigh,
				Enabled:  true,
			},
			NotificationTypeDailySummary: {
				Channels: []Channel{ChannelEmail},
				Priority: "normal",
				Enabled:  true,
			},
			NotificationTypePnLAlert: {
				Channels: []Channel{ChannelSlack},
				Priority: priorityHigh,
				Enabled:  true,
			},
			NotificationTypeCircuitBreaker: {
				Channels: []Channel{ChannelEmail, ChannelSlack},
				Priority: priorityHigh,
				Enabled:  true,
			},
			NotificationTypeConsensusFailure: {
				Channels: []Channel{ChannelSlack},
				Priority: "normal",
				Enabled:  true,
			},
		},
		DefaultChannels: []Channel{ChannelSlack},
	}
}

// Manager routes notifications to appropriate channels based on event type
type Manager struct {
	config ManagerConfig

	// Backends
	emailBackend *EmailBackend
	slackBackend *SlackBackend

	// Stats
	mu             sync.RWMutex
	sentCount      map[NotificationType]int64
	failedCount    map[NotificationType]int64
	lastSentByType map[NotificationType]time.Time
}

// NewManager creates a new notification manager
func NewManager(config ManagerConfig, emailBackend *EmailBackend, slackBackend *SlackBackend) *Manager {
	return &Manager{
		config:         config,
		emailBackend:   emailBackend,
		slackBackend:   slackBackend,
		sentCount:      make(map[NotificationType]int64),
		failedCount:    make(map[NotificationType]int64),
		lastSentByType: make(map[NotificationType]time.Time),
	}
}

// Send routes a notification to the appropriate channels based on configuration
func (m *Manager) Send(ctx context.Context, notification Notification) error {
	if !m.config.Enabled {
		log.Debug().
			Str("notification_type", string(notification.Type)).
			Msg("Notifications disabled globally, skipping")
		return nil
	}

	// Get event configuration
	eventConfig, exists := m.config.Events[notification.Type]
	if !exists {
		// Use default channels
		eventConfig = EventConfig{
			Channels: m.config.DefaultChannels,
			Priority: "normal",
			Enabled:  true,
		}
	}

	if !eventConfig.Enabled {
		log.Debug().
			Str("notification_type", string(notification.Type)).
			Msg("Notification type disabled, skipping")
		return nil
	}

	// Set priority from config if not explicitly set
	if notification.Priority == "" {
		notification.Priority = eventConfig.Priority
	}

	// Send to each configured channel
	var lastErr error
	successCount := 0

	for _, channel := range eventConfig.Channels {
		var err error

		switch channel {
		case ChannelEmail:
			err = m.sendEmail(ctx, notification)
		case ChannelSlack:
			err = m.sendSlack(ctx, notification)
		default:
			log.Warn().
				Str("channel", string(channel)).
				Msg("Unknown notification channel, skipping")
			continue
		}

		if err != nil {
			log.Error().
				Err(err).
				Str("channel", string(channel)).
				Str("notification_type", string(notification.Type)).
				Msg("Failed to send notification")
			lastErr = err
			m.recordFailure(notification.Type)
		} else {
			successCount++
			m.recordSuccess(notification.Type)
		}
	}

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("failed to send notification to any channel: %w", lastErr)
	}

	log.Info().
		Str("notification_type", string(notification.Type)).
		Int("channels_sent", successCount).
		Int("channels_total", len(eventConfig.Channels)).
		Msg("Notification sent")

	return nil
}

// sendEmail sends notification via email backend
func (m *Manager) sendEmail(ctx context.Context, notification Notification) error {
	if m.emailBackend == nil {
		log.Debug().Msg("Email backend not configured, skipping")
		return nil
	}

	// Use empty device token to send to default recipients
	return m.emailBackend.Send(ctx, "", notification)
}

// sendSlack sends notification via Slack backend
func (m *Manager) sendSlack(ctx context.Context, notification Notification) error {
	if m.slackBackend == nil {
		log.Debug().Msg("Slack backend not configured, skipping")
		return nil
	}

	// Use empty device token to send to default channel
	return m.slackBackend.Send(ctx, "", notification)
}

// recordSuccess records a successful notification
func (m *Manager) recordSuccess(notifType NotificationType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentCount[notifType]++
	m.lastSentByType[notifType] = time.Now()
}

// recordFailure records a failed notification
func (m *Manager) recordFailure(notifType NotificationType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedCount[notifType]++
}

// GetStats returns notification statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSent := int64(0)
	totalFailed := int64(0)

	sentByType := make(map[string]int64)
	failedByType := make(map[string]int64)
	lastSentByType := make(map[string]string)

	for notifType, count := range m.sentCount {
		sentByType[string(notifType)] = count
		totalSent += count
	}

	for notifType, count := range m.failedCount {
		failedByType[string(notifType)] = count
		totalFailed += count
	}

	for notifType, t := range m.lastSentByType {
		lastSentByType[string(notifType)] = t.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"enabled":           m.config.Enabled,
		"total_sent":        totalSent,
		"total_failed":      totalFailed,
		"sent_by_type":      sentByType,
		"failed_by_type":    failedByType,
		"last_sent_by_type": lastSentByType,
		"email_configured":  m.emailBackend != nil && !m.emailBackend.IsMock(),
		"slack_configured":  m.slackBackend != nil && !m.slackBackend.IsMock(),
	}
}

// Close closes all backends
func (m *Manager) Close() error {
	var lastErr error

	if m.emailBackend != nil {
		if err := m.emailBackend.Close(); err != nil {
			lastErr = err
		}
	}

	if m.slackBackend != nil {
		if err := m.slackBackend.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// IsEnabled returns whether notifications are enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// IsEventEnabled returns whether a specific event type is enabled
func (m *Manager) IsEventEnabled(notifType NotificationType) bool {
	if !m.config.Enabled {
		return false
	}

	eventConfig, exists := m.config.Events[notifType]
	if !exists {
		return true // Default to enabled if not explicitly configured
	}

	return eventConfig.Enabled
}

// GetChannelsForEvent returns the channels configured for a specific event type
func (m *Manager) GetChannelsForEvent(notifType NotificationType) []Channel {
	eventConfig, exists := m.config.Events[notifType]
	if !exists {
		return m.config.DefaultChannels
	}

	return eventConfig.Channels
}

// Helper methods for sending specific notification types

// SendTradeExecuted sends a trade execution notification
func (m *Manager) SendTradeExecuted(ctx context.Context, orderID, symbol, side string, quantity, price float64) error {
	notification := Notification{
		Type:  NotificationTypeTradeExecution,
		Title: fmt.Sprintf("Trade Executed: %s %s", side, symbol),
		Body:  fmt.Sprintf("Executed %s order for %.8f %s at $%.2f", side, quantity, symbol, price),
		Data:  TradeNotificationData(orderID, symbol, side, quantity, price),
	}

	return m.Send(ctx, notification)
}

// SendPositionClosed sends a position closed notification
func (m *Manager) SendPositionClosed(ctx context.Context, positionID, symbol, side string, quantity, entryPrice, exitPrice, pnl, pnlPercent float64) error {
	outcome := outcomeProfit
	if pnl < 0 {
		outcome = outcomeLoss
	}

	notification := Notification{
		Type:  NotificationTypePositionClosed,
		Title: fmt.Sprintf("Position Closed: %s %s", side, symbol),
		Body:  fmt.Sprintf("Closed %s position with %s of $%.2f (%.2f%%)", symbol, outcome, pnl, pnlPercent),
		Data:  PositionClosedNotificationData(positionID, symbol, side, quantity, entryPrice, exitPrice, pnl, pnlPercent),
	}

	return m.Send(ctx, notification)
}

// SendSafetyGuardTriggered sends a safety guard triggered notification
func (m *Manager) SendSafetyGuardTriggered(ctx context.Context, guardType, reason string, currentValue, threshold float64) error {
	notification := Notification{
		Type:     NotificationTypeSafetyGuard,
		Title:    fmt.Sprintf("Safety Guard Triggered: %s", guardType),
		Body:     fmt.Sprintf("Trading halted: %s (current: %.2f%%, threshold: %.2f%%)", reason, currentValue*100, threshold*100),
		Data:     SafetyGuardNotificationData(guardType, reason, currentValue, threshold),
		Priority: priorityHigh,
	}

	return m.Send(ctx, notification)
}

// SendSystemError sends a system error notification
func (m *Manager) SendSystemError(ctx context.Context, component, errorType, errorMsg string) error {
	notification := Notification{
		Type:     NotificationTypeSystemError,
		Title:    fmt.Sprintf("System Error: %s", component),
		Body:     fmt.Sprintf("Error in %s: %s - %s", component, errorType, errorMsg),
		Data:     SystemErrorNotificationData(component, errorType, errorMsg),
		Priority: priorityHigh,
	}

	return m.Send(ctx, notification)
}

// SendDailySummary sends a daily summary notification
func (m *Manager) SendDailySummary(ctx context.Context, date string, totalTrades, winningTrades, losingTrades int, totalPnL, totalPnLPercent, winRate float64) error {
	outcome := outcomeProfit
	if totalPnL < 0 {
		outcome = outcomeLoss
	}

	notification := Notification{
		Type:  NotificationTypeDailySummary,
		Title: fmt.Sprintf("Daily Summary: %s", date),
		Body: fmt.Sprintf("Trades: %d (W: %d, L: %d) | Total %s: $%.2f (%.2f%%) | Win Rate: %.1f%%",
			totalTrades, winningTrades, losingTrades, outcome, totalPnL, totalPnLPercent, winRate),
		Data: DailySummaryNotificationData(date, totalTrades, winningTrades, losingTrades, totalPnL, totalPnLPercent, winRate),
	}

	return m.Send(ctx, notification)
}

// SendCircuitBreakerTriggered sends a circuit breaker triggered notification
func (m *Manager) SendCircuitBreakerTriggered(ctx context.Context, reason string, threshold float64) error {
	notification := Notification{
		Type:     NotificationTypeCircuitBreaker,
		Title:    "Circuit Breaker Triggered",
		Body:     fmt.Sprintf("Trading halted: %s (threshold: %.2f%%)", reason, threshold*100),
		Data:     CircuitBreakerNotificationData(reason, threshold),
		Priority: priorityHigh,
	}

	return m.Send(ctx, notification)
}

// SendConsensusFailure sends a consensus failure notification
func (m *Manager) SendConsensusFailure(ctx context.Context, symbol, reason string, agentCount int) error {
	notification := Notification{
		Type:  NotificationTypeConsensusFailure,
		Title: fmt.Sprintf("Consensus Failure: %s", symbol),
		Body:  fmt.Sprintf("Failed to reach consensus for %s: %s (%d agents)", symbol, reason, agentCount),
		Data:  ConsensusFailureNotificationData(symbol, reason, agentCount),
	}

	return m.Send(ctx, notification)
}

// SendPnLAlert sends a P&L alert notification
func (m *Manager) SendPnLAlert(ctx context.Context, sessionID string, pnlPercent, pnlAmount float64) error {
	direction := outcomeGain
	if pnlPercent < 0 {
		direction = outcomeLoss
	}

	notification := Notification{
		Type:     NotificationTypePnLAlert,
		Title:    fmt.Sprintf("P&L Alert: %.2f%%", pnlPercent),
		Body:     fmt.Sprintf("Significant %s detected: %.2f%% ($%.2f)", direction, pnlPercent, pnlAmount),
		Data:     PnLNotificationData(sessionID, pnlPercent, pnlAmount),
		Priority: priorityHigh,
	}

	return m.Send(ctx, notification)
}
