package api

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// TestParseDuration verifies that parseDuration converts human-friendly duration
// strings into the correct past time.Time value relative to now.
func TestParseDuration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		checkApprox func(t *testing.T, got time.Time)
	}{
		{
			name:  "1h subtracts one hour",
			input: "1h",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().Add(-1 * time.Hour)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected time ~1h ago")
			},
		},
		{
			name:  "24h subtracts 24 hours",
			input: "24h",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().Add(-24 * time.Hour)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected time ~24h ago")
			},
		},
		{
			name:  "7d subtracts 7 days",
			input: "7d",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(0, 0, -7)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected time ~7 days ago")
			},
		},
		{
			name:  "30d subtracts 30 days",
			input: "30d",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(0, 0, -30)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected time ~30 days ago")
			},
		},
		{
			name:  "3m subtracts 3 months",
			input: "3m",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(0, -3, 0)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected time ~3 months ago")
			},
		},
		{
			name:  "1y subtracts 1 year",
			input: "1y",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(-1, 0, 0)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected time ~1 year ago")
			},
		},
		{
			name:  "invalid string falls back to 1 month ago",
			input: "notvalid",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(0, -1, 0)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected default fallback ~1 month ago")
			},
		},
		{
			name:  "empty string falls back to 1 month ago",
			input: "",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(0, -1, 0)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected default fallback ~1 month ago")
			},
		},
		{
			name:  "unknown unit falls back to 1 month ago",
			input: "5z",
			checkApprox: func(t *testing.T, got time.Time) {
				t.Helper()
				want := time.Now().AddDate(0, -1, 0)
				diff := want.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				assert.Less(t, diff, 2*time.Second, "expected default fallback ~1 month ago")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDuration(tc.input)
			assert.False(t, got.IsZero(), "parseDuration should never return zero time")
			tc.checkApprox(t, got)
		})
	}
}

// TestNewRiskHandler verifies that NewRiskHandler returns a non-nil handler with
// the provided config, and that a nil config is handled gracefully.
func TestNewRiskHandler(t *testing.T) {
	t.Run("with explicit config", func(t *testing.T) {
		cfg := &config.RiskConfig{
			MaxDailyLossDollars: 500.0,
			MaxDrawdownPct:      20.0,
			MaxTradeCount:       100,
		}
		h := NewRiskHandler(nil, cfg)
		assert.NotNil(t, h)
		assert.Equal(t, cfg, h.cfg)
		assert.Nil(t, h.db)
		assert.NotNil(t, h.riskService)
	})

	t.Run("with nil config falls back to zero-value RiskConfig", func(t *testing.T) {
		h := NewRiskHandler(nil, nil)
		assert.NotNil(t, h)
		assert.NotNil(t, h.cfg, "cfg should be non-nil even when nil is passed")
		assert.Equal(t, &config.RiskConfig{}, h.cfg)
		assert.NotNil(t, h.riskService)
	})
}

// TestNewTradesHandler verifies that NewTradesHandler returns a non-nil handler.
func TestNewTradesHandler(t *testing.T) {
	h := NewTradesHandler(nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.db)
}

// TestNewPerformanceHandler verifies that NewPerformanceHandler returns a non-nil handler.
func TestNewPerformanceHandler(t *testing.T) {
	h := NewPerformanceHandler(nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.db)
}

func TestBuildBreaker(t *testing.T) {
	tests := []struct {
		name           string
		current        float64
		threshold      float64
		expectedStatus string
	}{
		{"disabled when threshold zero", 100.0, 0.0, "DISABLED"},
		{"disabled when threshold negative", 50.0, -1.0, "DISABLED"},
		{"triggered when current equals threshold", 100.0, 100.0, "TRIGGERED"},
		{"triggered when current exceeds threshold", 110.0, 100.0, "TRIGGERED"},
		{"warning when at 80 percent", 80.0, 100.0, "WARNING"},
		{"warning just above 80 percent", 81.0, 100.0, "WARNING"},
		{"ok when below 80 percent", 79.0, 100.0, "OK"},
		{"ok at zero current", 0.0, 100.0, "OK"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildBreaker("TestBreaker", tc.current, tc.threshold)
			assert.Equal(t, tc.expectedStatus, result["status"])
			assert.Equal(t, "TestBreaker", result["name"])
			assert.Equal(t, tc.threshold, result["threshold"])
			// current is rounded to 2dp
			assert.Equal(t, math.Round(tc.current*100)/100, result["current"])
		})
	}
}
