package notifications

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// NotificationCallback is a function type for notification callbacks
type NotificationCallback func(ctx context.Context, notification Notification)

// EventEmitter is an interface for components that can emit notification events
type EventEmitter interface {
	SetNotificationCallback(callback NotificationCallback)
}

// Integration provides notification integration for various components
// It acts as a bridge between system events and the notification manager
type Integration struct {
	manager *Manager
	enabled bool
	mu      sync.RWMutex

	// Rate limiting for specific event types to prevent notification spam
	lastNotification map[NotificationType]time.Time
	cooldowns        map[NotificationType]time.Duration
}

// NewIntegration creates a new notification integration
func NewIntegration(manager *Manager) *Integration {
	return &Integration{
		manager:          manager,
		enabled:          true,
		lastNotification: make(map[NotificationType]time.Time),
		cooldowns: map[NotificationType]time.Duration{
			// Some events should have cooldowns to prevent spam
			NotificationTypePnLAlert:         5 * time.Minute, // P&L alerts every 5 min max
			NotificationTypeConsensusFailure: 1 * time.Minute, // Consensus failures every 1 min max
		},
	}
}

// SetEnabled enables or disables the integration
func (i *Integration) SetEnabled(enabled bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.enabled = enabled
}

// IsEnabled returns whether the integration is enabled
func (i *Integration) IsEnabled() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.enabled
}

// shouldSend checks if we should send this notification based on rate limiting
func (i *Integration) shouldSend(notifType NotificationType) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	cooldown, hasCooldown := i.cooldowns[notifType]
	if !hasCooldown {
		// No cooldown for this type, always send
		i.lastNotification[notifType] = time.Now()
		return true
	}

	lastSent, exists := i.lastNotification[notifType]
	if !exists || time.Since(lastSent) >= cooldown {
		i.lastNotification[notifType] = time.Now()
		return true
	}

	return false
}

// EmitTradeExecuted emits a trade execution notification
func (i *Integration) EmitTradeExecuted(ctx context.Context, orderID, symbol, side string, quantity, price float64) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypeTradeExecution) {
		return
	}

	go func() {
		if err := i.manager.SendTradeExecuted(ctx, orderID, symbol, side, quantity, price); err != nil {
			log.Error().Err(err).
				Str("orderID", orderID).
				Str("symbol", symbol).
				Msg("Failed to send trade execution notification")
		}
	}()
}

// EmitPositionClosed emits a position closed notification
func (i *Integration) EmitPositionClosed(ctx context.Context, positionID, symbol, side string, quantity, entryPrice, exitPrice, pnl, pnlPercent float64) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypePositionClosed) {
		return
	}

	go func() {
		if err := i.manager.SendPositionClosed(ctx, positionID, symbol, side, quantity, entryPrice, exitPrice, pnl, pnlPercent); err != nil {
			log.Error().Err(err).
				Str("positionID", positionID).
				Str("symbol", symbol).
				Msg("Failed to send position closed notification")
		}
	}()
}

// EmitSafetyGuardTriggered emits a safety guard triggered notification
func (i *Integration) EmitSafetyGuardTriggered(ctx context.Context, guardType, reason string, currentValue, threshold float64) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypeSafetyGuard) {
		return
	}

	go func() {
		if err := i.manager.SendSafetyGuardTriggered(ctx, guardType, reason, currentValue, threshold); err != nil {
			log.Error().Err(err).
				Str("guardType", guardType).
				Str("reason", reason).
				Msg("Failed to send safety guard notification")
		}
	}()
}

// EmitSystemError emits a system error notification
func (i *Integration) EmitSystemError(ctx context.Context, component, errorType, errorMsg string) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypeSystemError) {
		return
	}

	go func() {
		if err := i.manager.SendSystemError(ctx, component, errorType, errorMsg); err != nil {
			log.Error().Err(err).
				Str("component", component).
				Str("errorType", errorType).
				Msg("Failed to send system error notification")
		}
	}()
}

// EmitDailySummary emits a daily summary notification
func (i *Integration) EmitDailySummary(ctx context.Context, date string, totalTrades, winningTrades, losingTrades int, totalPnL, totalPnLPercent, winRate float64) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	// Daily summary has no rate limiting
	go func() {
		if err := i.manager.SendDailySummary(ctx, date, totalTrades, winningTrades, losingTrades, totalPnL, totalPnLPercent, winRate); err != nil {
			log.Error().Err(err).
				Str("date", date).
				Msg("Failed to send daily summary notification")
		}
	}()
}

// EmitCircuitBreakerTriggered emits a circuit breaker triggered notification
func (i *Integration) EmitCircuitBreakerTriggered(ctx context.Context, reason string, threshold float64) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypeCircuitBreaker) {
		return
	}

	go func() {
		if err := i.manager.SendCircuitBreakerTriggered(ctx, reason, threshold); err != nil {
			log.Error().Err(err).
				Str("reason", reason).
				Msg("Failed to send circuit breaker notification")
		}
	}()
}

// EmitPnLAlert emits a P&L alert notification
func (i *Integration) EmitPnLAlert(ctx context.Context, sessionID string, pnlPercent, pnlAmount float64) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypePnLAlert) {
		return
	}

	go func() {
		if err := i.manager.SendPnLAlert(ctx, sessionID, pnlPercent, pnlAmount); err != nil {
			log.Error().Err(err).
				Str("sessionID", sessionID).
				Float64("pnlPercent", pnlPercent).
				Msg("Failed to send P&L alert notification")
		}
	}()
}

// EmitConsensusFailure emits a consensus failure notification
func (i *Integration) EmitConsensusFailure(ctx context.Context, symbol, reason string, agentCount int) {
	if !i.IsEnabled() || i.manager == nil {
		return
	}

	if !i.shouldSend(NotificationTypeConsensusFailure) {
		return
	}

	go func() {
		if err := i.manager.SendConsensusFailure(ctx, symbol, reason, agentCount); err != nil {
			log.Error().Err(err).
				Str("symbol", symbol).
				Str("reason", reason).
				Msg("Failed to send consensus failure notification")
		}
	}()
}

// CreateSafetyGuardCallback creates a callback function for safety guard events
// This can be passed to SafetyGuard.SetMetricsCallback to emit notifications
func (i *Integration) CreateSafetyGuardCallback() func(event interface{}) {
	return func(event interface{}) {
		// Type assertion for SafetyGuardEvent
		type SafetyGuardEvent struct {
			EventType    string    `json:"event_type"`
			Violation    string    `json:"violation"`
			Symbol       string    `json:"symbol,omitempty"`
			OrderValue   float64   `json:"order_value,omitempty"`
			CurrentValue float64   `json:"current_value,omitempty"`
			Threshold    float64   `json:"threshold,omitempty"`
			Timestamp    time.Time `json:"timestamp"`
		}

		sgEvent, ok := event.(SafetyGuardEvent)
		if !ok {
			return
		}

		ctx := context.Background()

		// Emit notifications based on event type
		switch sgEvent.EventType {
		case "circuit_breaker_tripped":
			i.EmitCircuitBreakerTriggered(ctx, string(sgEvent.Violation), sgEvent.Threshold)
		case "order_rejected":
			if sgEvent.Violation != "none" && sgEvent.Violation != "" {
				i.EmitSafetyGuardTriggered(ctx, string(sgEvent.Violation), sgEvent.EventType, sgEvent.CurrentValue, sgEvent.Threshold)
			}
		case "emergency_stop_activated":
			i.EmitSafetyGuardTriggered(ctx, "emergency_stop", "Emergency stop activated", 0, 0)
		}
	}
}

// SetCooldown sets the cooldown duration for a specific notification type
func (i *Integration) SetCooldown(notifType NotificationType, duration time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cooldowns[notifType] = duration
}

// GetStats returns integration statistics
func (i *Integration) GetStats() map[string]interface{} {
	i.mu.RLock()
	defer i.mu.RUnlock()

	lastNotifs := make(map[string]string)
	for notifType, lastTime := range i.lastNotification {
		lastNotifs[string(notifType)] = lastTime.Format(time.RFC3339)
	}

	cooldownsStr := make(map[string]string)
	for notifType, duration := range i.cooldowns {
		cooldownsStr[string(notifType)] = duration.String()
	}

	return map[string]interface{}{
		"enabled":            i.enabled,
		"last_notifications": lastNotifs,
		"cooldowns":          cooldownsStr,
	}
}
