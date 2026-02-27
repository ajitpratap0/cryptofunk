package notifications

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
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
		Priority: priorityHigh,
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendPnLAlert sends a P&L alert notification
func (h *NotificationHelper) SendPnLAlert(ctx context.Context, userID, sessionID string, pnlPercent, pnlAmount float64) error {
	direction := outcomeGain
	if pnlPercent < 0 {
		direction = outcomeLoss
	}

	notification := Notification{
		Type:     NotificationTypePnLAlert,
		Title:    fmt.Sprintf("P&L Alert: %s%%", formatFloat(pnlPercent)),
		Body:     fmt.Sprintf("Significant %s detected: %s%% (%s)", direction, formatFloat(pnlPercent), formatFloat(pnlAmount)),
		Data:     PnLNotificationData(sessionID, pnlPercent, pnlAmount),
		Priority: priorityHigh,
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
		Priority: priorityHigh,
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
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to send trade notification")
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

// SendPositionClosed sends a position closed notification
func (h *NotificationHelper) SendPositionClosed(ctx context.Context, userID, positionID, symbol, side string, quantity, entryPrice, exitPrice, pnl, pnlPercent float64) error {
	outcome := "profit"
	if pnl < 0 {
		outcome = "loss"
	}

	notification := Notification{
		Type:     NotificationTypePositionClosed,
		Title:    fmt.Sprintf("Position Closed: %s %s", side, symbol),
		Body:     fmt.Sprintf("Closed %s %s with %s of %s (%s%%)", formatFloat(quantity), symbol, outcome, formatFloat(pnl), formatFloat(pnlPercent)),
		Data:     PositionClosedNotificationData(positionID, symbol, side, quantity, entryPrice, exitPrice, pnl, pnlPercent),
		Priority: "normal",
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendSafetyGuardTriggered sends a safety guard triggered notification
func (h *NotificationHelper) SendSafetyGuardTriggered(ctx context.Context, userID, guardType, reason string, currentValue, threshold float64) error {
	notification := Notification{
		Type:     NotificationTypeSafetyGuard,
		Title:    fmt.Sprintf("Safety Guard Triggered: %s", guardType),
		Body:     fmt.Sprintf("Trading halted due to %s: %s (current: %s, threshold: %s)", guardType, reason, formatFloat(currentValue), formatFloat(threshold)),
		Data:     SafetyGuardNotificationData(guardType, reason, currentValue, threshold),
		Priority: priorityHigh,
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendSystemError sends a system error notification
func (h *NotificationHelper) SendSystemError(ctx context.Context, userID, component, errorType, errorMsg string) error {
	notification := Notification{
		Type:     NotificationTypeSystemError,
		Title:    fmt.Sprintf("System Error: %s", component),
		Body:     fmt.Sprintf("Error in %s: %s - %s", component, errorType, errorMsg),
		Data:     SystemErrorNotificationData(component, errorType, errorMsg),
		Priority: priorityHigh,
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// SendDailySummary sends a daily summary notification
func (h *NotificationHelper) SendDailySummary(ctx context.Context, userID, date string, totalTrades, winningTrades, losingTrades int, totalPnL, totalPnLPercent, winRate float64) error {
	outcome := "profit"
	if totalPnL < 0 {
		outcome = "loss"
	}

	notification := Notification{
		Type:  NotificationTypeDailySummary,
		Title: fmt.Sprintf("Daily Summary: %s", date),
		Body: fmt.Sprintf("Trades: %d (W: %d, L: %d) | P&L: %s (%s%%) | Win Rate: %s%%",
			totalTrades, winningTrades, losingTrades, formatFloat(totalPnL), outcome, formatFloat(winRate)),
		Data:     DailySummaryNotificationData(date, totalTrades, winningTrades, losingTrades, totalPnL, totalPnLPercent, winRate),
		Priority: "normal",
	}

	return h.service.SendToUser(ctx, userID, notification)
}

// BulkSendPositionClosed sends position closed notifications to multiple users
func (h *NotificationHelper) BulkSendPositionClosed(ctx context.Context, userIDs []string, positionID, symbol, side string, quantity, entryPrice, exitPrice, pnl, pnlPercent float64) error {
	for _, userID := range userIDs {
		if err := h.SendPositionClosed(ctx, userID, positionID, symbol, side, quantity, entryPrice, exitPrice, pnl, pnlPercent); err != nil {
			// Log error but continue sending to other users
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to send position closed notification")
		}
	}
	return nil
}

// BulkSendSafetyGuard sends safety guard notifications to multiple users
func (h *NotificationHelper) BulkSendSafetyGuard(ctx context.Context, userIDs []string, guardType, reason string, currentValue, threshold float64) error {
	for _, userID := range userIDs {
		if err := h.SendSafetyGuardTriggered(ctx, userID, guardType, reason, currentValue, threshold); err != nil {
			// Log error but continue sending to other users
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to send safety guard notification")
		}
	}
	return nil
}

// BulkSendSystemError sends system error notifications to multiple users (typically admins)
func (h *NotificationHelper) BulkSendSystemError(ctx context.Context, userIDs []string, component, errorType, errorMsg string) error {
	for _, userID := range userIDs {
		if err := h.SendSystemError(ctx, userID, component, errorType, errorMsg); err != nil {
			// Log error but continue sending to other users
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to send system error notification")
		}
	}
	return nil
}

// BulkSendDailySummary sends daily summary notifications to multiple users
func (h *NotificationHelper) BulkSendDailySummary(ctx context.Context, userIDs []string, date string, totalTrades, winningTrades, losingTrades int, totalPnL, totalPnLPercent, winRate float64) error {
	for _, userID := range userIDs {
		if err := h.SendDailySummary(ctx, userID, date, totalTrades, winningTrades, losingTrades, totalPnL, totalPnLPercent, winRate); err != nil {
			// Log error but continue sending to other users
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to send daily summary notification")
		}
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
