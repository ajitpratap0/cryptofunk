package risk

import (
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

// SafetyGuardMetricsRecorder provides a metrics callback for SafetyGuard
// that integrates with the centralized metrics package
type SafetyGuardMetricsRecorder struct{}

// NewSafetyGuardMetricsRecorder creates a new metrics recorder
func NewSafetyGuardMetricsRecorder() *SafetyGuardMetricsRecorder {
	return &SafetyGuardMetricsRecorder{}
}

// RecordEvent records a safety guard event to Prometheus metrics
func (r *SafetyGuardMetricsRecorder) RecordEvent(event SafetyGuardEvent) {
	switch event.EventType {
	case "order_rejected":
		metrics.RecordSafetyGuardOrderRejected(string(event.Violation), event.Symbol)
	case "order_allowed":
		metrics.RecordSafetyGuardOrderAllowed()
	case "circuit_breaker_tripped":
		metrics.UpdateSafetyGuardCircuitBreaker(true, 0)
		metrics.RecordSafetyGuardViolation(string(event.Violation))
	case "circuit_breaker_reset":
		metrics.UpdateSafetyGuardCircuitBreaker(false, 0)
	case "daily_reset":
		// Reset daily metrics
		metrics.UpdateSafetyGuardStats(0, 0, 0, 0)
	case "emergency_stop_activated":
		metrics.UpdateSafetyGuardEmergencyStop(true)
		metrics.RecordSafetyGuardViolation(string(event.Violation))
	case "emergency_stop_cleared":
		metrics.UpdateSafetyGuardEmergencyStop(false)
	}
}

// UpdateStats updates the safety guard statistics in Prometheus
func (r *SafetyGuardMetricsRecorder) UpdateStats(stats TradingStats, isCircuitBreakerTripped bool, cooldownUntil time.Time) {
	// Calculate daily drawdown as a positive ratio
	dailyDrawdown := 0.0
	if stats.Capital > 0 && stats.DailyPnL < 0 {
		dailyDrawdown = -stats.DailyPnL / stats.Capital
	}

	// Calculate total exposure as a ratio
	totalExposureRatio := 0.0
	if stats.Capital > 0 {
		totalExposureRatio = stats.TotalExposure / stats.Capital
	}

	metrics.UpdateSafetyGuardStats(
		dailyDrawdown,
		stats.DailyTradeCount,
		stats.ConsecutiveLosses,
		totalExposureRatio,
	)

	// Update circuit breaker status
	if isCircuitBreakerTripped {
		cooldownSeconds := time.Until(cooldownUntil).Seconds()
		if cooldownSeconds < 0 {
			cooldownSeconds = 0
		}
		metrics.UpdateSafetyGuardCircuitBreaker(true, cooldownSeconds)
	} else {
		metrics.UpdateSafetyGuardCircuitBreaker(false, 0)
	}

	// Update emergency stop status
	metrics.UpdateSafetyGuardEmergencyStop(stats.EmergencyStopActive)
}

// UpdateTradingHoursStatus updates the trading hours status metric
func (r *SafetyGuardMetricsRecorder) UpdateTradingHoursStatus(withinHours bool) {
	metrics.UpdateSafetyGuardTradingHours(withinHours)
}

// SetEnabled updates the safety guard enabled status metric
func (r *SafetyGuardMetricsRecorder) SetEnabled(enabled bool) {
	metrics.UpdateSafetyGuardStatus(enabled)
}

// SetupMetricsCallback configures a SafetyGuard instance with a metrics callback
func SetupMetricsCallback(sg *SafetyGuard) {
	recorder := NewSafetyGuardMetricsRecorder()
	recorder.SetEnabled(sg.config.Enabled)

	sg.SetMetricsCallback(func(event SafetyGuardEvent) {
		recorder.RecordEvent(event)
	})
}
