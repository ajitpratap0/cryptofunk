package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTradingHoursConfig_Disabled(t *testing.T) {
	cfg := TradingHoursConfig{
		Enabled: false,
		Start:   "09:00",
		End:     "16:00",
	}

	// Should always return true when disabled
	now := time.Date(2024, 1, 15, 3, 0, 0, 0, time.UTC) // 3 AM - would be outside hours if enabled
	assert.True(t, cfg.IsWithinTradingHours(now))
}

func TestTradingHoursConfig_WithinHours(t *testing.T) {
	cfg := TradingHoursConfig{
		Enabled:  true,
		Start:    "09:00",
		End:      "16:00",
		Timezone: "UTC",
	}

	testCases := []struct {
		name     string
		time     time.Time
		expected bool
	}{
		{
			name:     "Start of trading hours",
			time:     time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Middle of trading hours",
			time:     time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Just before end",
			time:     time.Date(2024, 1, 15, 15, 59, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "At end time (exclusive)",
			time:     time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "After trading hours",
			time:     time.Date(2024, 1, 15, 17, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Before trading hours",
			time:     time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Midnight",
			time:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cfg.IsWithinTradingHours(tc.time)
			assert.Equal(t, tc.expected, result, "Time: %v", tc.time)
		})
	}
}

func TestTradingHoursConfig_OvernightTrading(t *testing.T) {
	// Test overnight trading window (e.g., 20:00 - 04:00)
	cfg := TradingHoursConfig{
		Enabled:  true,
		Start:    "20:00",
		End:      "04:00",
		Timezone: "UTC",
	}

	testCases := []struct {
		name     string
		time     time.Time
		expected bool
	}{
		{
			name:     "Start of overnight window",
			time:     time.Date(2024, 1, 15, 20, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "During overnight - late evening",
			time:     time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "During overnight - midnight",
			time:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "During overnight - early morning",
			time:     time.Date(2024, 1, 15, 2, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Just before end of overnight",
			time:     time.Date(2024, 1, 15, 3, 59, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "At end of overnight window",
			time:     time.Date(2024, 1, 15, 4, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "During day (outside overnight window)",
			time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Just before start",
			time:     time.Date(2024, 1, 15, 19, 59, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cfg.IsWithinTradingHours(tc.time)
			assert.Equal(t, tc.expected, result, "Time: %v", tc.time)
		})
	}
}

func TestTradingHoursConfig_Timezone(t *testing.T) {
	cfg := TradingHoursConfig{
		Enabled:  true,
		Start:    "09:00",
		End:      "16:00",
		Timezone: "America/New_York",
	}

	// 14:00 UTC in winter is 09:00 EST (start of trading)
	// Using a date in January to avoid DST complications
	utcTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	assert.True(t, cfg.IsWithinTradingHours(utcTime), "Should be within trading hours when converted to EST")

	// 13:59 UTC in winter is 08:59 EST (before trading)
	utcTime = time.Date(2024, 1, 15, 13, 59, 0, 0, time.UTC)
	assert.False(t, cfg.IsWithinTradingHours(utcTime), "Should be before trading hours when converted to EST")
}

func TestTradingHoursConfig_InvalidTimezone(t *testing.T) {
	cfg := TradingHoursConfig{
		Enabled:  true,
		Start:    "09:00",
		End:      "16:00",
		Timezone: "Invalid/Timezone",
	}

	// Should default to UTC when timezone is invalid
	utcTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	assert.True(t, cfg.IsWithinTradingHours(utcTime))
}

func TestTradingHoursConfig_InvalidTimeFormat(t *testing.T) {
	// Note: Go's time.Parse("15:04", ...) accepts formats like "9:00" (without leading zero)
	// So we use actually unparseable formats to test fail-open behavior
	testCases := []struct {
		name  string
		start string
		end   string
	}{
		{
			name:  "Invalid start format",
			start: "invalid", // Actually unparseable
			end:   "16:00",
		},
		{
			name:  "Invalid end format",
			start: "09:00",
			end:   "notvalid", // Actually unparseable
		},
		{
			name:  "Invalid both",
			start: "invalid",
			end:   "also invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := TradingHoursConfig{
				Enabled:  true,
				Start:    tc.start,
				End:      tc.end,
				Timezone: "UTC",
			}

			// Invalid time format should allow trading (fail-open)
			result := cfg.IsWithinTradingHours(time.Now())
			assert.True(t, result, "Invalid time format should allow trading")
		})
	}
}

func TestTradingHoursConfig_GetLocation(t *testing.T) {
	testCases := []struct {
		name     string
		timezone string
		expected string
	}{
		{
			name:     "Valid timezone",
			timezone: "America/New_York",
			expected: "America/New_York",
		},
		{
			name:     "UTC",
			timezone: "UTC",
			expected: "UTC",
		},
		{
			name:     "Empty timezone",
			timezone: "",
			expected: "UTC",
		},
		{
			name:     "Invalid timezone",
			timezone: "Invalid/Zone",
			expected: "UTC",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := TradingHoursConfig{
				Timezone: tc.timezone,
			}

			loc := cfg.GetLocation()
			assert.Equal(t, tc.expected, loc.String())
		})
	}
}

func TestTradingHoursConfig_EdgeCases(t *testing.T) {
	t.Run("Same start and end time", func(t *testing.T) {
		cfg := TradingHoursConfig{
			Enabled:  true,
			Start:    "12:00",
			End:      "12:00",
			Timezone: "UTC",
		}

		// Same start and end means 24-hour trading (treated as overnight)
		assert.True(t, cfg.IsWithinTradingHours(time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC)))
		assert.True(t, cfg.IsWithinTradingHours(time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC)))
	})
}

func TestSafetyGuardConfig_GetCooldownPeriod(t *testing.T) {
	testCases := []struct {
		name           string
		cooldownPeriod string
		expected       time.Duration
	}{
		{
			name:           "Valid duration",
			cooldownPeriod: "30m",
			expected:       30 * time.Minute,
		},
		{
			name:           "One hour",
			cooldownPeriod: "1h",
			expected:       time.Hour,
		},
		{
			name:           "Invalid duration",
			cooldownPeriod: "invalid",
			expected:       time.Hour, // Default
		},
		{
			name:           "Empty",
			cooldownPeriod: "",
			expected:       time.Hour, // Default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := SafetyGuardConfig{
				CooldownPeriod: tc.cooldownPeriod,
			}

			result := cfg.GetCooldownPeriod()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafetyGuardConfig_GetMinOrderInterval(t *testing.T) {
	testCases := []struct {
		name             string
		minOrderInterval string
		expected         time.Duration
	}{
		{
			name:             "Valid duration",
			minOrderInterval: "10s",
			expected:         10 * time.Second,
		},
		{
			name:             "One minute",
			minOrderInterval: "1m",
			expected:         time.Minute,
		},
		{
			name:             "Invalid duration",
			minOrderInterval: "invalid",
			expected:         5 * time.Second, // Default
		},
		{
			name:             "Empty",
			minOrderInterval: "",
			expected:         5 * time.Second, // Default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := SafetyGuardConfig{
				MinOrderInterval: tc.minOrderInterval,
			}

			result := cfg.GetMinOrderInterval()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSafetyGuardConfig_IsLargeOrder(t *testing.T) {
	cfg := SafetyGuardConfig{
		LargeTradeThreshold: 0.05, // 5%
	}

	testCases := []struct {
		name       string
		orderValue float64
		capital    float64
		expected   bool
	}{
		{
			name:       "Large order (10% of capital)",
			orderValue: 1000.0,
			capital:    10000.0,
			expected:   true,
		},
		{
			name:       "Exactly at threshold (5%)",
			orderValue: 500.0,
			capital:    10000.0,
			expected:   true,
		},
		{
			name:       "Small order (1%)",
			orderValue: 100.0,
			capital:    10000.0,
			expected:   false,
		},
		{
			name:       "Zero capital",
			orderValue: 100.0,
			capital:    0,
			expected:   false,
		},
		{
			name:       "Zero threshold",
			orderValue: 100.0,
			capital:    10000.0,
			expected:   false, // Will fail because we need to test with 0 threshold
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testCfg := cfg
			if tc.name == "Zero threshold" {
				testCfg.LargeTradeThreshold = 0
			}
			result := testCfg.IsLargeOrder(tc.orderValue, tc.capital)
			assert.Equal(t, tc.expected, result)
		})
	}
}
