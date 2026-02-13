package notifications

import (
	"context"
	"time"
)

// Priority represents notification priority levels
type Priority int

const (
	PriorityInfo      Priority = iota // Informational
	PriorityWarning                   // Warning
	PriorityCritical                  // Critical
	PriorityEmergency                 // Emergency - always delivered
)

func (p Priority) String() string {
	switch p {
	case PriorityInfo:
		return "INFO"
	case PriorityWarning:
		return "WARNING"
	case PriorityCritical:
		return "CRITICAL"
	case PriorityEmergency:
		return "EMERGENCY"
	default:
		return "UNKNOWN"
	}
}

// EventType represents different notification event types
type EventType string

const (
	EventTradeExecuted  EventType = "trade_executed"
	EventPositionClosed EventType = "position_closed"
	EventErrorAlert     EventType = "error_alert"
	EventSafetyAlert    EventType = "safety_alert"
	EventDailySummary   EventType = "daily_summary"
)

// Event represents a notification event to be dispatched
type Event struct {
	Type      EventType         `json:"type"`
	Priority  Priority          `json:"priority"`
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// TradeExecutedEvent creates a trade executed notification event
func TradeExecutedEvent(symbol, side, agent string, price, size float64) Event {
	return Event{
		Type:     EventTradeExecuted,
		Priority: PriorityInfo,
		Title:    "Trade Executed",
		Message:  symbol + " " + side,
		Fields: map[string]string{
			"symbol": symbol,
			"side":   side,
			"price":  formatFloat(price),
			"size":   formatFloatPrec8(size),
			"agent":  agent,
		},
		Timestamp: time.Now(),
	}
}

// PositionClosedEvent creates a position closed notification event
func PositionClosedEvent(symbol string, pnl float64, holdDuration time.Duration, reason string) Event {
	p := PriorityInfo
	if pnl < 0 {
		p = PriorityWarning
	}
	return Event{
		Type:     EventPositionClosed,
		Priority: p,
		Title:    "Position Closed",
		Message:  symbol + " closed",
		Fields: map[string]string{
			"symbol":        symbol,
			"pnl":           formatFloat(pnl),
			"hold_duration": holdDuration.String(),
			"reason":        reason,
		},
		Timestamp: time.Now(),
	}
}

// ErrorAlertEvent creates an error alert notification event
func ErrorAlertEvent(source, message string) Event {
	return Event{
		Type:     EventErrorAlert,
		Priority: PriorityCritical,
		Title:    "Error Alert",
		Message:  message,
		Fields: map[string]string{
			"source": source,
		},
		Timestamp: time.Now(),
	}
}

// SafetyAlertEvent creates a safety alert notification event
func SafetyAlertEvent(alertType, details string) Event {
	return Event{
		Type:     EventSafetyAlert,
		Priority: PriorityEmergency,
		Title:    "Safety Alert: " + alertType,
		Message:  details,
		Fields: map[string]string{
			"alert_type": alertType,
		},
		Timestamp: time.Now(),
	}
}

// DailySummaryEvent creates a daily summary notification event
func DailySummaryEvent(totalTrades int, pnl, winRate float64, bestTrade, worstTrade string) Event {
	return Event{
		Type:     EventDailySummary,
		Priority: PriorityInfo,
		Title:    "Daily Summary",
		Message:  "End of day trading summary",
		Fields: map[string]string{
			"total_trades": formatInt(totalTrades),
			"pnl":          formatFloat(pnl),
			"win_rate":     formatFloat(winRate),
			"best_trade":   bestTrade,
			"worst_trade":  worstTrade,
		},
		Timestamp: time.Now(),
	}
}

// Channel is the interface for notification delivery channels
type Channel interface {
	// Name returns the channel identifier
	Name() string
	// Send delivers a notification event
	Send(ctx context.Context, event Event) error
	// Close cleans up resources
	Close() error
}
