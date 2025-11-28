package notifications

import (
	"context"
	"fmt"
)

// NotificationHelper provides convenient methods for sending common notifications
type NotificationHelper struct {
	service Service
}

// NewHelper creates a new notification helper
func NewHelper(service Service) *NotificationHelper {
	return &NotificationHelper{
		service: service,
	}
}

// SendTradeExecution sends a trade execution notification
func (h *NotificationHelper) SendTradeExecution(ctx context.Context, userID, orderID, symbol, side string, quantity, price float64) error {
	notification := Notification{
		Type:     NotificationTypeTradeExecution,
		Title:    fmt.Sprintf("Trade Executed: %s %s", side, symbol),
		Body:     fmt.Sprintf("Executed %s %s at %s", formatFloat(quantity), symbol, formatFloat(price)),
		Data:     TradeNotificationData(orderID, symbol, side, quantity, price),
		Priority: "high",
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendPnLAlert sends a P&L alert notification
func (h *NotificationHelper) SendPnLAlert(ctx context.Context, userID, sessionID string, pnlPercent, pnlAmount float64) error {
	direction := "gain"
	if pnlPercent < 0 {
		direction = "loss"
	}

	notification := Notification{
		Type:     NotificationTypePnLAlert,
		Title:    fmt.Sprintf("P&L Alert: %s%%", formatFloat(pnlPercent)),
		Body:     fmt.Sprintf("Significant %s detected: %s%% (%s)", direction, formatFloat(pnlPercent), formatFloat(pnlAmount)),
		Data:     PnLNotificationData(sessionID, pnlPercent, pnlAmount),
		Priority: "high",
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendCircuitBreakerAlert sends a circuit breaker notification
func (h *NotificationHelper) SendCircuitBreakerAlert(ctx context.Context, userID, reason string, threshold float64) error {
	notification := Notification{
		Type:     NotificationTypeCircuitBreaker,
		Title:    "Trading Halted - Circuit Breaker Triggered",
		Body:     fmt.Sprintf("Trading halted: %s (threshold: %s%%)", reason, formatFloat(threshold)),
		Data:     CircuitBreakerNotificationData(reason, threshold),
		Priority: "high",
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendConsensusFailure sends a consensus failure notification
func (h *NotificationHelper) SendConsensusFailure(ctx context.Context, userID, symbol, reason string, agentCount int) error {
	notification := Notification{
		Type:     NotificationTypeConsensusFailure,
		Title:    fmt.Sprintf("Consensus Failure: %s", symbol),
		Body:     fmt.Sprintf("Failed to reach consensus for %s: %s (%d agents)", symbol, reason, agentCount),
		Data:     ConsensusFailureNotificationData(symbol, reason, agentCount),
		Priority: "normal",
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// BulkSendTradeExecution sends trade execution notifications to multiple users
func (h *NotificationHelper) BulkSendTradeExecution(ctx context.Context, userIDs []string, orderID, symbol, side string, quantity, price float64) error {
	for _, userID := range userIDs {
		if err := h.SendTradeExecution(ctx, userID, orderID, symbol, side, quantity, price); err != nil {
			// Log error but continue sending to other users
			fmt.Printf("Failed to send trade notification to user %s: %v\n", userID, err)
		}
	}
	return nil
}

// CheckPnLThresholdAndNotify checks if P&L change exceeds threshold and sends notification
func (h *NotificationHelper) CheckPnLThresholdAndNotify(ctx context.Context, userID, sessionID string, previousPnL, currentPnL float64, threshold float64) error {
	// Calculate percentage change
	var percentChange float64
	if previousPnL != 0 {
		percentChange = ((currentPnL - previousPnL) / abs(previousPnL)) * 100
	} else if currentPnL != 0 {
		percentChange = 100 // 100% change from zero
	}

	// Check if threshold is exceeded
	if abs(percentChange) >= threshold {
		return h.SendPnLAlert(ctx, userID, sessionID, percentChange, currentPnL-previousPnL)
	}

	return nil
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
