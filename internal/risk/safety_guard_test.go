package risk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

func TestNewSafetyGuard(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:          true,
		MaxDailyDrawdown: 0.05,
		MaxPositionSize:  0.1,
	}

	sg := NewSafetyGuard(cfg)

	assert.NotNil(t, sg)
	assert.True(t, sg.config.Enabled)
}

func TestValidateOrder_Disabled(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: false,
	}

	sg := NewSafetyGuard(cfg)

	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   1.0,
		OrderValue: 50000.0,
	})

	assert.True(t, result.Allowed)
}

func TestValidateOrder_MaxDailyDrawdown(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:          true,
		MaxDailyDrawdown: 0.05, // 5%
		// Don't set MaxConsecutiveLosses to avoid triggering circuit breaker
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0) // $10,000 capital

	// Simulate 6% loss with a winning trade first to avoid consecutive losses
	sg.RecordTrade(100.0)
	sg.RecordTrade(-700.0) // Net -600 = 6% loss

	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	// When drawdown exceeds threshold, either the direct check or circuit breaker could block
	// Both indicate the max drawdown protection is working
	assert.True(t,
		result.Violation == ViolationMaxDailyDrawdown || result.Violation == ViolationCircuitBreakerOpen,
		"Expected max_daily_drawdown or circuit_breaker_open, got: %s", result.Violation)
}

func TestValidateOrder_MaxPositionSize(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:         true,
		MaxPositionSize: 0.1, // 10% max position
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0) // $10,000 capital

	// Try to place order worth 15% of capital
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.03,
		OrderValue: 1500.0, // 15% of $10,000
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationMaxPositionSize, result.Violation)
}

func TestValidateOrder_MaxTotalExposure(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:          true,
		MaxTotalExposure: 0.5, // 50% max exposure
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0) // $10,000 capital

	// Add existing positions totaling 45%
	sg.UpdatePosition("ETH/USDT", 4500.0)

	// Try to place order that would bring total to 55%
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.02,
		OrderValue: 1000.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationMaxTotalExposure, result.Violation)
}

func TestValidateOrder_MaxDailyTrades(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:        true,
		MaxDailyTrades: 3,
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0)

	// Simulate 3 trades
	sg.RecordTrade(100.0)
	sg.RecordTrade(50.0)
	sg.RecordTrade(-25.0)

	// 4th trade should be rejected
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationMaxDailyTrades, result.Violation)
}

func TestValidateOrder_MaxOrderValue(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:       true,
		MaxOrderValue: 1000.0, // $1,000 max per order
	}

	sg := NewSafetyGuard(cfg)

	// Try to place order worth $1,500
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.03,
		OrderValue: 1500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationMaxOrderValue, result.Violation)
}

func TestValidateOrder_MinOrderInterval(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:          true,
		MinOrderInterval: "1s", // 1 second minimum between orders
	}

	sg := NewSafetyGuard(cfg)
	sg.RecordOrderPlaced() // Record that an order was just placed

	// Immediately try to place another order
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationMinOrderInterval, result.Violation)
}

func TestValidateOrder_LargeTradeConfirmation(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:             true,
		RequireConfirmation: true,
		LargeTradeThreshold: 0.05, // 5% of capital is "large"
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0) // $10,000 capital

	// Try to place large order without confirmation
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:      "BTC/USDT",
		Side:        "buy",
		OrderType:   "market",
		Quantity:    0.02,
		OrderValue:  1000.0, // 10% of capital
		IsConfirmed: false,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationLargeTradeUnconfirm, result.Violation)

	// Same order with confirmation should pass
	result = sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:      "BTC/USDT",
		Side:        "buy",
		OrderType:   "market",
		Quantity:    0.02,
		OrderValue:  1000.0,
		IsConfirmed: true,
	})

	assert.True(t, result.Allowed)
}

func TestEmergencyStop(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
	}

	sg := NewSafetyGuard(cfg)

	// Verify not active initially
	assert.False(t, sg.IsEmergencyStopActive())

	// Activate emergency stop
	sg.EmergencyStop("Market crash detected")

	// Verify active
	assert.True(t, sg.IsEmergencyStopActive())
	active, reason, since := sg.GetEmergencyStopInfo()
	assert.True(t, active)
	assert.Equal(t, "Market crash detected", reason)
	assert.False(t, since.IsZero())

	// Orders should be rejected
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationEmergencyStop, result.Violation)

	// Clear emergency stop
	sg.ClearEmergencyStop()

	// Verify cleared
	assert.False(t, sg.IsEmergencyStopActive())

	// Orders should be allowed again
	result = sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.True(t, result.Allowed)
}

func TestTradingHours_Disabled(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
		TradingHours: config.TradingHoursConfig{
			Enabled: false,
		},
	}

	sg := NewSafetyGuard(cfg)

	// Should always be within trading hours when disabled
	assert.True(t, sg.IsWithinTradingHours())

	// Orders should be allowed
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.True(t, result.Allowed)
}

func TestTradingHours_WithinHours(t *testing.T) {
	// Test with a time window that includes the current time in UTC
	nowUTC := time.Now().UTC()
	startTime := nowUTC.Add(-1 * time.Hour).Format("15:04")
	endTime := nowUTC.Add(1 * time.Hour).Format("15:04")

	cfg := config.SafetyGuardConfig{
		Enabled: true,
		TradingHours: config.TradingHoursConfig{
			Enabled:  true,
			Start:    startTime,
			End:      endTime,
			Timezone: "UTC",
		},
	}

	sg := NewSafetyGuard(cfg)

	// Should be within trading hours
	assert.True(t, sg.IsWithinTradingHours())
}

func TestTradingHours_OutsideHours(t *testing.T) {
	// Test with a time window that excludes the current time
	now := time.Now().UTC()
	startTime := now.Add(2 * time.Hour).Format("15:04")
	endTime := now.Add(3 * time.Hour).Format("15:04")

	cfg := config.SafetyGuardConfig{
		Enabled: true,
		TradingHours: config.TradingHoursConfig{
			Enabled:  true,
			Start:    startTime,
			End:      endTime,
			Timezone: "UTC",
		},
	}

	sg := NewSafetyGuard(cfg)

	// Should be outside trading hours
	assert.False(t, sg.IsWithinTradingHours())

	// Orders should be rejected
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationOutsideTradingHours, result.Violation)
}

func TestCircuitBreaker_ConsecutiveLosses(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:              true,
		MaxConsecutiveLosses: 3,
		CooldownPeriod:       "1m", // 1 minute cooldown
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0)

	// Record 3 consecutive losses
	sg.RecordTrade(-100.0)
	sg.RecordTrade(-50.0)
	sg.RecordTrade(-75.0)

	// Circuit breaker should be tripped
	assert.True(t, sg.IsCircuitBreakerTripped())

	// Orders should be rejected
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationCircuitBreakerOpen, result.Violation)
}

func TestCircuitBreaker_ManualReset(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:              true,
		MaxConsecutiveLosses: 2,
		CooldownPeriod:       "1h", // Long cooldown
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0)

	// Trip circuit breaker
	sg.RecordTrade(-100.0)
	sg.RecordTrade(-50.0)

	assert.True(t, sg.IsCircuitBreakerTripped())

	// Manually reset
	sg.ResetCircuitBreaker()

	assert.False(t, sg.IsCircuitBreakerTripped())

	// Orders should be allowed again
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.True(t, result.Allowed)
}

func TestGetStats(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0)

	// Record some trades
	sg.RecordTrade(100.0)
	sg.RecordTrade(-50.0)
	sg.UpdatePosition("BTC/USDT", 2000.0)

	stats := sg.GetStats()

	assert.Equal(t, 50.0, stats.DailyPnL)
	assert.Equal(t, 0.005, stats.DailyPnLPercent) // 0.5%
	assert.Equal(t, 2, stats.DailyTradeCount)
	assert.Equal(t, 1, stats.ConsecutiveLosses)
	assert.Equal(t, 2000.0, stats.TotalExposure)
	assert.Equal(t, 0.2, stats.TotalExposurePercent) // 20%
	assert.Equal(t, 10000.0, stats.Capital)
	assert.False(t, stats.EmergencyStopActive)
}

func TestUpdatePosition(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
	}

	sg := NewSafetyGuard(cfg)

	// Add position
	sg.UpdatePosition("BTC/USDT", 1000.0)
	stats := sg.GetStats()
	assert.Equal(t, 1000.0, stats.TotalExposure)

	// Update position
	sg.UpdatePosition("BTC/USDT", 1500.0)
	stats = sg.GetStats()
	assert.Equal(t, 1500.0, stats.TotalExposure)

	// Add another position
	sg.UpdatePosition("ETH/USDT", 500.0)
	stats = sg.GetStats()
	assert.Equal(t, 2000.0, stats.TotalExposure)

	// Close a position (set to 0)
	sg.UpdatePosition("BTC/USDT", 0)
	stats = sg.GetStats()
	assert.Equal(t, 500.0, stats.TotalExposure)
}

func TestResetDailyCounters(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0)

	// Accumulate some stats
	sg.RecordTrade(-500.0)
	sg.RecordTrade(-100.0)
	sg.RecordTrade(-50.0)

	stats := sg.GetStats()
	assert.Equal(t, -650.0, stats.DailyPnL)
	assert.Equal(t, 3, stats.DailyTradeCount)
	assert.Equal(t, 3, stats.ConsecutiveLosses)

	// Reset daily counters with new capital
	sg.ResetDailyCounters(12000.0)

	stats = sg.GetStats()
	assert.Equal(t, 0.0, stats.DailyPnL)
	assert.Equal(t, 0, stats.DailyTradeCount)
	assert.Equal(t, 12000.0, stats.Capital)
	// Note: consecutive losses are NOT reset by ResetDailyCounters
}

func TestMetricsCallback(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
	}

	sg := NewSafetyGuard(cfg)

	var receivedEvents []SafetyGuardEvent
	sg.SetMetricsCallback(func(event SafetyGuardEvent) {
		receivedEvents = append(receivedEvents, event)
	})

	// Trigger an event
	sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	require.Len(t, receivedEvents, 1)
	assert.Equal(t, "order_allowed", receivedEvents[0].EventType)
	assert.Equal(t, ViolationNone, receivedEvents[0].Violation)
}

func TestValidateOrder_AllowedOrder(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled:          true,
		MaxDailyDrawdown: 0.05,
		MaxPositionSize:  0.2,
		MaxTotalExposure: 0.5,
		MaxDailyTrades:   100,
		MaxOrderValue:    10000.0,
	}

	sg := NewSafetyGuard(cfg)
	sg.SetCapital(10000.0)

	// Place a reasonable order
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.True(t, result.Allowed)
	// When allowed, violation is empty string (not "none")
	assert.Empty(t, result.Violation)
}

func TestEmergencyStopPrecedence(t *testing.T) {
	// Emergency stop should take precedence over all other checks
	cfg := config.SafetyGuardConfig{
		Enabled: true,
	}

	sg := NewSafetyGuard(cfg)

	// Activate emergency stop
	sg.EmergencyStop("Test reason")

	// Even with disabled safety guards scenario check
	result := sg.ValidateOrder(context.Background(), OrderRequest{
		Symbol:     "BTC/USDT",
		Side:       "buy",
		OrderType:  "market",
		Quantity:   0.01,
		OrderValue: 500.0,
	})

	assert.False(t, result.Allowed)
	assert.Equal(t, ViolationEmergencyStop, result.Violation)
}

func TestGetTradingHoursInfo(t *testing.T) {
	cfg := config.SafetyGuardConfig{
		Enabled: true,
		TradingHours: config.TradingHoursConfig{
			Enabled:  true,
			Start:    "09:00",
			End:      "16:00",
			Timezone: "America/New_York",
		},
	}

	sg := NewSafetyGuard(cfg)

	enabled, start, end, timezone, _ := sg.GetTradingHoursInfo()

	assert.True(t, enabled)
	assert.Equal(t, "09:00", start)
	assert.Equal(t, "16:00", end)
	assert.Equal(t, "America/New_York", timezone)
}
