package risk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// SafetyGuardViolation represents a type of safety guard violation
type SafetyGuardViolation string

const (
	ViolationNone                SafetyGuardViolation = "none"
	ViolationMaxDailyDrawdown    SafetyGuardViolation = "max_daily_drawdown"
	ViolationMaxPositionSize     SafetyGuardViolation = "max_position_size"
	ViolationMaxTotalExposure    SafetyGuardViolation = "max_total_exposure"
	ViolationMaxDailyTrades      SafetyGuardViolation = "max_daily_trades"
	ViolationConsecutiveLosses   SafetyGuardViolation = "consecutive_losses"
	ViolationMaxOrderValue       SafetyGuardViolation = "max_order_value"
	ViolationMinOrderInterval    SafetyGuardViolation = "min_order_interval"
	ViolationCircuitBreakerOpen  SafetyGuardViolation = "circuit_breaker_open"
	ViolationLargeTradeUnconfirm SafetyGuardViolation = "large_trade_unconfirmed"
	ViolationOutsideTradingHours SafetyGuardViolation = "outside_trading_hours"
	ViolationEmergencyStop       SafetyGuardViolation = "emergency_stop"
)

// SafetyCheckResult represents the result of a safety check
type SafetyCheckResult struct {
	Allowed        bool                 `json:"allowed"`
	Violation      SafetyGuardViolation `json:"violation,omitempty"`
	Message        string               `json:"message,omitempty"`
	CurrentValue   float64              `json:"current_value,omitempty"`
	ThresholdValue float64              `json:"threshold_value,omitempty"`
}

// TradingStats holds current trading statistics for safety checks
type TradingStats struct {
	DailyPnL             float64   `json:"daily_pnl"`
	DailyPnLPercent      float64   `json:"daily_pnl_percent"`
	DailyTradeCount      int       `json:"daily_trade_count"`
	ConsecutiveLosses    int       `json:"consecutive_losses"`
	TotalExposure        float64   `json:"total_exposure"`
	TotalExposurePercent float64   `json:"total_exposure_percent"`
	Capital              float64   `json:"capital"`
	LastOrderTime        time.Time `json:"last_order_time"`
	EmergencyStopActive  bool      `json:"emergency_stop_active"`
	EmergencyStopReason  string    `json:"emergency_stop_reason,omitempty"`
	EmergencyStopTime    time.Time `json:"emergency_stop_time,omitempty"`
}

// OrderRequest represents an order to be validated
type OrderRequest struct {
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`       // "buy" or "sell"
	OrderType   string  `json:"order_type"` // "market" or "limit"
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price,omitempty"` // For limit orders
	OrderValue  float64 `json:"order_value"`     // Total value of the order
	IsConfirmed bool    `json:"is_confirmed"`    // For large trade confirmation
}

// SafetyGuard provides live trading safety mechanisms
type SafetyGuard struct {
	config config.SafetyGuardConfig
	mu     sync.RWMutex

	// Trading state
	dailyPnL              float64
	dailyStartCapital     float64
	dailyTradeCount       int
	consecutiveLosses     int
	lastOrderTime         time.Time
	circuitBreakerTripped bool
	circuitBreakerUntil   time.Time
	lastDayReset          time.Time

	// Emergency stop state
	emergencyStopActive bool
	emergencyStopReason string
	emergencyStopTime   time.Time

	// Position tracking
	positions     map[string]float64 // symbol -> position value
	totalExposure float64

	// Metrics callback (optional, set via SetMetricsCallback)
	metricsCallback func(event SafetyGuardEvent)
}

// SafetyGuardEvent represents a safety guard event for metrics
type SafetyGuardEvent struct {
	EventType    string               `json:"event_type"`
	Violation    SafetyGuardViolation `json:"violation"`
	Symbol       string               `json:"symbol,omitempty"`
	OrderValue   float64              `json:"order_value,omitempty"`
	CurrentValue float64              `json:"current_value,omitempty"`
	Threshold    float64              `json:"threshold,omitempty"`
	Timestamp    time.Time            `json:"timestamp"`
}

// NewSafetyGuard creates a new SafetyGuard instance
func NewSafetyGuard(cfg config.SafetyGuardConfig) *SafetyGuard {
	return &SafetyGuard{
		config:       cfg,
		positions:    make(map[string]float64),
		lastDayReset: time.Now().Truncate(24 * time.Hour),
	}
}

// SetMetricsCallback sets a callback function for metrics recording
func (sg *SafetyGuard) SetMetricsCallback(callback func(SafetyGuardEvent)) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.metricsCallback = callback
}

// SetCapital sets the current capital for percentage calculations
func (sg *SafetyGuard) SetCapital(capital float64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.dailyStartCapital = capital
}

// UpdatePosition updates position tracking for a symbol
func (sg *SafetyGuard) UpdatePosition(symbol string, value float64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	oldValue := sg.positions[symbol]
	if value <= 0 {
		delete(sg.positions, symbol)
	} else {
		sg.positions[symbol] = value
	}

	// Recalculate total exposure
	sg.totalExposure -= oldValue
	if value > 0 {
		sg.totalExposure += value
	}

	log.Debug().
		Str("symbol", symbol).
		Float64("old_value", oldValue).
		Float64("new_value", value).
		Float64("total_exposure", sg.totalExposure).
		Msg("Position updated")
}

// RecordTrade records a completed trade for tracking
func (sg *SafetyGuard) RecordTrade(pnl float64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.checkDayReset()

	sg.dailyPnL += pnl
	sg.dailyTradeCount++

	if pnl < 0 {
		sg.consecutiveLosses++
	} else {
		sg.consecutiveLosses = 0
	}

	log.Debug().
		Float64("pnl", pnl).
		Float64("daily_pnl", sg.dailyPnL).
		Int("daily_trades", sg.dailyTradeCount).
		Int("consecutive_losses", sg.consecutiveLosses).
		Msg("Trade recorded")

	// Check if consecutive losses trigger circuit breaker
	if sg.consecutiveLosses >= sg.config.MaxConsecutiveLosses && sg.config.MaxConsecutiveLosses > 0 {
		sg.tripCircuitBreaker(ViolationConsecutiveLosses)
	}

	// Check if daily drawdown triggers circuit breaker
	if sg.dailyStartCapital > 0 {
		drawdownPercent := -sg.dailyPnL / sg.dailyStartCapital
		if drawdownPercent >= sg.config.MaxDailyDrawdown && sg.config.MaxDailyDrawdown > 0 {
			sg.tripCircuitBreaker(ViolationMaxDailyDrawdown)
		}
	}
}

// ValidateOrder checks if an order is allowed based on safety guards
func (sg *SafetyGuard) ValidateOrder(ctx context.Context, req OrderRequest) SafetyCheckResult {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Check if safety guards are enabled
	if !sg.config.Enabled {
		return SafetyCheckResult{Allowed: true}
	}

	sg.checkDayReset()

	// Check emergency stop first (highest priority)
	if result := sg.checkEmergencyStop(); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check circuit breaker
	if result := sg.checkCircuitBreaker(); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check trading hours
	if result := sg.checkTradingHours(); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check daily drawdown
	if result := sg.checkDailyDrawdown(); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check max order value
	if result := sg.checkMaxOrderValue(req.OrderValue); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check max position size
	if result := sg.checkMaxPositionSize(req.OrderValue); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check max total exposure
	if result := sg.checkMaxTotalExposure(req.OrderValue); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check max daily trades
	if result := sg.checkMaxDailyTrades(); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check min order interval
	if result := sg.checkMinOrderInterval(); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// Check large trade confirmation
	if result := sg.checkLargeTradeConfirmation(req); !result.Allowed {
		sg.recordEvent("order_rejected", result.Violation, req.Symbol, req.OrderValue, result.CurrentValue, result.ThresholdValue)
		return result
	}

	// All checks passed
	sg.recordEvent("order_allowed", ViolationNone, req.Symbol, req.OrderValue, 0, 0)
	return SafetyCheckResult{Allowed: true}
}

// RecordOrderPlaced should be called after an order is successfully placed
func (sg *SafetyGuard) RecordOrderPlaced() {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.lastOrderTime = time.Now()
}

// GetStats returns current trading statistics
func (sg *SafetyGuard) GetStats() TradingStats {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	sg.checkDayResetRLocked()

	dailyPnLPercent := 0.0
	totalExposurePercent := 0.0
	if sg.dailyStartCapital > 0 {
		dailyPnLPercent = sg.dailyPnL / sg.dailyStartCapital
		totalExposurePercent = sg.totalExposure / sg.dailyStartCapital
	}

	return TradingStats{
		DailyPnL:             sg.dailyPnL,
		DailyPnLPercent:      dailyPnLPercent,
		DailyTradeCount:      sg.dailyTradeCount,
		ConsecutiveLosses:    sg.consecutiveLosses,
		TotalExposure:        sg.totalExposure,
		TotalExposurePercent: totalExposurePercent,
		Capital:              sg.dailyStartCapital,
		LastOrderTime:        sg.lastOrderTime,
		EmergencyStopActive:  sg.emergencyStopActive,
		EmergencyStopReason:  sg.emergencyStopReason,
		EmergencyStopTime:    sg.emergencyStopTime,
	}
}

// IsCircuitBreakerTripped returns whether the circuit breaker is currently open
func (sg *SafetyGuard) IsCircuitBreakerTripped() bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	if !sg.circuitBreakerTripped {
		return false
	}

	// Check if cooldown period has passed
	if time.Now().After(sg.circuitBreakerUntil) {
		return false
	}

	return true
}

// ResetCircuitBreaker manually resets the circuit breaker
func (sg *SafetyGuard) ResetCircuitBreaker() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.circuitBreakerTripped = false
	sg.circuitBreakerUntil = time.Time{}
	sg.consecutiveLosses = 0

	log.Info().Msg("Safety guard circuit breaker manually reset")
	sg.recordEvent("circuit_breaker_reset", ViolationNone, "", 0, 0, 0)
}

// ResetDailyCounters resets daily counters (called at start of new trading day)
func (sg *SafetyGuard) ResetDailyCounters(newCapital float64) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.dailyPnL = 0
	sg.dailyTradeCount = 0
	sg.dailyStartCapital = newCapital
	sg.lastDayReset = time.Now().Truncate(24 * time.Hour)

	log.Info().Float64("capital", newCapital).Msg("Daily counters reset")
	sg.recordEvent("daily_reset", ViolationNone, "", 0, 0, 0)
}

// === Internal helper methods ===

func (sg *SafetyGuard) checkDayReset() {
	today := time.Now().Truncate(24 * time.Hour)
	if today.After(sg.lastDayReset) {
		sg.dailyPnL = 0
		sg.dailyTradeCount = 0
		sg.lastDayReset = today
		log.Debug().Msg("Daily counters auto-reset")
	}
}

func (sg *SafetyGuard) checkDayResetRLocked() {
	// This is a read-only check for GetStats - actual reset happens in write-locked methods
}

func (sg *SafetyGuard) tripCircuitBreaker(reason SafetyGuardViolation) {
	sg.circuitBreakerTripped = true
	sg.circuitBreakerUntil = time.Now().Add(sg.config.GetCooldownPeriod())

	log.Warn().
		Str("reason", string(reason)).
		Time("until", sg.circuitBreakerUntil).
		Msg("Safety guard circuit breaker tripped")

	sg.recordEvent("circuit_breaker_tripped", reason, "", 0, 0, 0)
}

func (sg *SafetyGuard) recordEvent(eventType string, violation SafetyGuardViolation, symbol string, orderValue, currentValue, threshold float64) {
	if sg.metricsCallback != nil {
		sg.metricsCallback(SafetyGuardEvent{
			EventType:    eventType,
			Violation:    violation,
			Symbol:       symbol,
			OrderValue:   orderValue,
			CurrentValue: currentValue,
			Threshold:    threshold,
			Timestamp:    time.Now(),
		})
	}
}

func (sg *SafetyGuard) checkCircuitBreaker() SafetyCheckResult {
	if !sg.circuitBreakerTripped {
		return SafetyCheckResult{Allowed: true}
	}

	if time.Now().After(sg.circuitBreakerUntil) {
		// Cooldown period has passed, reset circuit breaker
		sg.circuitBreakerTripped = false
		sg.circuitBreakerUntil = time.Time{}
		log.Info().Msg("Circuit breaker cooldown period ended, trading resumed")
		return SafetyCheckResult{Allowed: true}
	}

	remaining := time.Until(sg.circuitBreakerUntil)
	return SafetyCheckResult{
		Allowed:   false,
		Violation: ViolationCircuitBreakerOpen,
		Message:   fmt.Sprintf("Circuit breaker is open, trading halted for %v", remaining.Round(time.Second)),
	}
}

func (sg *SafetyGuard) checkDailyDrawdown() SafetyCheckResult {
	if sg.config.MaxDailyDrawdown <= 0 || sg.dailyStartCapital <= 0 {
		return SafetyCheckResult{Allowed: true}
	}

	currentDrawdown := -sg.dailyPnL / sg.dailyStartCapital
	if currentDrawdown >= sg.config.MaxDailyDrawdown {
		return SafetyCheckResult{
			Allowed:        false,
			Violation:      ViolationMaxDailyDrawdown,
			Message:        fmt.Sprintf("Daily drawdown %.2f%% exceeds maximum %.2f%%", currentDrawdown*100, sg.config.MaxDailyDrawdown*100),
			CurrentValue:   currentDrawdown,
			ThresholdValue: sg.config.MaxDailyDrawdown,
		}
	}

	return SafetyCheckResult{Allowed: true}
}

func (sg *SafetyGuard) checkMaxOrderValue(orderValue float64) SafetyCheckResult {
	if sg.config.MaxOrderValue <= 0 {
		return SafetyCheckResult{Allowed: true}
	}

	if orderValue > sg.config.MaxOrderValue {
		return SafetyCheckResult{
			Allowed:        false,
			Violation:      ViolationMaxOrderValue,
			Message:        fmt.Sprintf("Order value %.2f exceeds maximum %.2f", orderValue, sg.config.MaxOrderValue),
			CurrentValue:   orderValue,
			ThresholdValue: sg.config.MaxOrderValue,
		}
	}

	return SafetyCheckResult{Allowed: true}
}

func (sg *SafetyGuard) checkMaxPositionSize(orderValue float64) SafetyCheckResult {
	if sg.config.MaxPositionSize <= 0 || sg.dailyStartCapital <= 0 {
		return SafetyCheckResult{Allowed: true}
	}

	maxPositionValue := sg.dailyStartCapital * sg.config.MaxPositionSize
	if orderValue > maxPositionValue {
		return SafetyCheckResult{
			Allowed:        false,
			Violation:      ViolationMaxPositionSize,
			Message:        fmt.Sprintf("Order value %.2f exceeds max position size %.2f (%.1f%% of capital)", orderValue, maxPositionValue, sg.config.MaxPositionSize*100),
			CurrentValue:   orderValue,
			ThresholdValue: maxPositionValue,
		}
	}

	return SafetyCheckResult{Allowed: true}
}

func (sg *SafetyGuard) checkMaxTotalExposure(additionalExposure float64) SafetyCheckResult {
	if sg.config.MaxTotalExposure <= 0 || sg.dailyStartCapital <= 0 {
		return SafetyCheckResult{Allowed: true}
	}

	maxExposure := sg.dailyStartCapital * sg.config.MaxTotalExposure
	newTotalExposure := sg.totalExposure + additionalExposure

	if newTotalExposure > maxExposure {
		return SafetyCheckResult{
			Allowed:        false,
			Violation:      ViolationMaxTotalExposure,
			Message:        fmt.Sprintf("Total exposure %.2f would exceed maximum %.2f (%.1f%% of capital)", newTotalExposure, maxExposure, sg.config.MaxTotalExposure*100),
			CurrentValue:   newTotalExposure,
			ThresholdValue: maxExposure,
		}
	}

	return SafetyCheckResult{Allowed: true}
}

func (sg *SafetyGuard) checkMaxDailyTrades() SafetyCheckResult {
	if sg.config.MaxDailyTrades <= 0 {
		return SafetyCheckResult{Allowed: true}
	}

	if sg.dailyTradeCount >= sg.config.MaxDailyTrades {
		return SafetyCheckResult{
			Allowed:        false,
			Violation:      ViolationMaxDailyTrades,
			Message:        fmt.Sprintf("Daily trade count %d has reached maximum %d", sg.dailyTradeCount, sg.config.MaxDailyTrades),
			CurrentValue:   float64(sg.dailyTradeCount),
			ThresholdValue: float64(sg.config.MaxDailyTrades),
		}
	}

	return SafetyCheckResult{Allowed: true}
}

func (sg *SafetyGuard) checkMinOrderInterval() SafetyCheckResult {
	minInterval := sg.config.GetMinOrderInterval()
	if minInterval <= 0 {
		return SafetyCheckResult{Allowed: true}
	}

	if sg.lastOrderTime.IsZero() {
		return SafetyCheckResult{Allowed: true}
	}

	elapsed := time.Since(sg.lastOrderTime)
	if elapsed < minInterval {
		remaining := minInterval - elapsed
		return SafetyCheckResult{
			Allowed:        false,
			Violation:      ViolationMinOrderInterval,
			Message:        fmt.Sprintf("Order too soon, wait %v (min interval: %v)", remaining.Round(time.Millisecond), minInterval),
			CurrentValue:   elapsed.Seconds(),
			ThresholdValue: minInterval.Seconds(),
		}
	}

	return SafetyCheckResult{Allowed: true}
}

func (sg *SafetyGuard) checkLargeTradeConfirmation(req OrderRequest) SafetyCheckResult {
	if !sg.config.RequireConfirmation {
		return SafetyCheckResult{Allowed: true}
	}

	if !sg.config.IsLargeOrder(req.OrderValue, sg.dailyStartCapital) {
		return SafetyCheckResult{Allowed: true}
	}

	if req.IsConfirmed {
		return SafetyCheckResult{Allowed: true}
	}

	threshold := sg.dailyStartCapital * sg.config.LargeTradeThreshold
	return SafetyCheckResult{
		Allowed:        false,
		Violation:      ViolationLargeTradeUnconfirm,
		Message:        fmt.Sprintf("Large trade (%.2f) requires confirmation (threshold: %.2f)", req.OrderValue, threshold),
		CurrentValue:   req.OrderValue,
		ThresholdValue: threshold,
	}
}

func (sg *SafetyGuard) checkEmergencyStop() SafetyCheckResult {
	if !sg.emergencyStopActive {
		return SafetyCheckResult{Allowed: true}
	}

	return SafetyCheckResult{
		Allowed:   false,
		Violation: ViolationEmergencyStop,
		Message:   fmt.Sprintf("Emergency stop active: %s (since %v)", sg.emergencyStopReason, sg.emergencyStopTime.Format(time.RFC3339)),
	}
}

func (sg *SafetyGuard) checkTradingHours() SafetyCheckResult {
	if !sg.config.TradingHours.Enabled {
		return SafetyCheckResult{Allowed: true}
	}

	if sg.config.TradingHours.IsWithinTradingHours(time.Now()) {
		return SafetyCheckResult{Allowed: true}
	}

	loc := sg.config.TradingHours.GetLocation()
	return SafetyCheckResult{
		Allowed:   false,
		Violation: ViolationOutsideTradingHours,
		Message: fmt.Sprintf("Outside trading hours (%s - %s %s)",
			sg.config.TradingHours.Start,
			sg.config.TradingHours.End,
			loc.String()),
	}
}

// === Emergency Stop Public Methods ===

// EmergencyStop immediately halts all trading with a reason
func (sg *SafetyGuard) EmergencyStop(reason string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.emergencyStopActive = true
	sg.emergencyStopReason = reason
	sg.emergencyStopTime = time.Now()

	log.Warn().
		Str("reason", reason).
		Msg("EMERGENCY STOP ACTIVATED - All trading halted")

	sg.recordEvent("emergency_stop_activated", ViolationEmergencyStop, "", 0, 0, 0)
}

// ClearEmergencyStop clears the emergency stop state
func (sg *SafetyGuard) ClearEmergencyStop() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if !sg.emergencyStopActive {
		return
	}

	duration := time.Since(sg.emergencyStopTime)
	reason := sg.emergencyStopReason

	sg.emergencyStopActive = false
	sg.emergencyStopReason = ""
	sg.emergencyStopTime = time.Time{}

	log.Info().
		Str("previous_reason", reason).
		Dur("duration", duration).
		Msg("Emergency stop cleared - Trading can resume")

	sg.recordEvent("emergency_stop_cleared", ViolationNone, "", 0, 0, 0)
}

// IsEmergencyStopActive returns whether emergency stop is currently active
func (sg *SafetyGuard) IsEmergencyStopActive() bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return sg.emergencyStopActive
}

// GetEmergencyStopInfo returns information about the current emergency stop
func (sg *SafetyGuard) GetEmergencyStopInfo() (active bool, reason string, since time.Time) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return sg.emergencyStopActive, sg.emergencyStopReason, sg.emergencyStopTime
}

// === Trading Hours Public Methods ===

// IsWithinTradingHours checks if trading is currently allowed based on time restrictions
func (sg *SafetyGuard) IsWithinTradingHours() bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	if !sg.config.TradingHours.Enabled {
		return true
	}

	return sg.config.TradingHours.IsWithinTradingHours(time.Now())
}

// GetTradingHoursInfo returns information about trading hours configuration
func (sg *SafetyGuard) GetTradingHoursInfo() (enabled bool, start, end, timezone string, withinHours bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	cfg := sg.config.TradingHours
	return cfg.Enabled, cfg.Start, cfg.End, cfg.Timezone, cfg.IsWithinTradingHours(time.Now())
}
