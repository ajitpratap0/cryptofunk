package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator_ValidateSymbol(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{"valid BTCUSDT", "BTCUSDT", false},
		{"valid ETHUSDT", "ETHUSDT", false},
		{"valid BNBUSDT", "BNBUSDT", false},
		{"valid SOLUSDT", "SOLUSDT", false},
		{"valid ETHBTC", "ETHBTC", false},
		{"valid lowercase", "btcusdt", false},
		{"valid mixed case", "BtcUsdt", false},
		{"empty", "", true},
		{"too short", "BT", true},
		{"invalid suffix", "BTCXYZ", true},
		{"invalid characters", "BTC$USDT", true},
		{"spaces", "BTC USDT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateSymbol(tt.symbol)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateCapital(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		capital float64
		wantErr bool
	}{
		{"valid 1000", 1000.0, false},
		{"valid 100", 100.0, false},
		{"valid 10", 10.0, false},
		{"valid max", 10000000.0, false},
		{"zero", 0.0, true},
		{"negative", -100.0, true},
		{"too small", 5.0, true},
		{"too large", 10000001.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCapital(tt.capital)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateTradingMode(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"PAPER uppercase", "PAPER", false},
		{"LIVE uppercase", "LIVE", false},
		{"paper lowercase", "paper", false},
		{"live lowercase", "live", false},
		{"Paper mixed", "Paper", false},
		{"invalid mode", "TEST", true},
		{"empty", "", true},
		{"sandbox", "SANDBOX", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateTradingMode(tt.mode)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateVerificationCode(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid short", "ABC123", false},
		{"valid long", "ABCD1234EFGH5678", false},
		{"valid min length", "ABCD", false},
		{"empty", "", true},
		{"too short", "ABC", true},
		{"too long", "ABCDEFGHIJKLMNOPQ", true},
		{"special chars", "ABC-123", true},
		{"spaces", "ABC 123", true},
		{"lowercase", "abc123", false}, // Gets uppercased
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateVerificationCode(tt.code)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateConfirmation(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name         string
		confirmation string
		want         bool
	}{
		{"CONFIRM uppercase", "CONFIRM", true},
		{"confirm lowercase", "confirm", true},
		{"YES uppercase", "YES", true},
		{"yes lowercase", "yes", true},
		{"Y uppercase", "Y", true},
		{"y lowercase", "y", true},
		{"no", "NO", false},
		{"n", "n", false},
		{"empty", "", false},
		{"random", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.ValidateConfirmation(tt.confirmation)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal text", "Hello World", "Hello World"},
		{"with leading spaces", "  Hello", "Hello"},
		{"with trailing spaces", "Hello  ", "Hello"},
		{"with control chars", "Hello\x00World", "HelloWorld"},
		{"with newlines", "Hello\nWorld", "HelloWorld"}, // Newlines (code 10) are removed as control chars < 32
		{"very long input", string(make([]byte, 1500)), ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeInput(tt.input)
			if tt.name == "very long input" {
				assert.LessOrEqual(t, len(result), 1000)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseCapital(t *testing.T) {
	tests := []struct {
		name       string
		capitalStr string
		expected   float64
		wantErr    bool
	}{
		{"plain number", "1000", 1000.0, false},
		{"with decimal", "1000.50", 1000.50, false},
		{"with dollar sign", "$1000", 1000.0, false},
		{"with USD prefix", "USD1000", 1000.0, false},
		{"with commas", "1,000", 1000.0, false},
		{"with spaces", "  1000  ", 1000.0, false},
		{"complex format", "$1,000.50", 1000.50, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCapital(tt.capitalStr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expected, result, 0.01)
			}
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		amount   float64
		expected string
	}{
		{1000.0, "$1000.00"},
		{1000.50, "$1000.50"},
		{0.0, "$0.00"},
		{-100.0, "-$100.00"},
		{-1000.50, "-$1000.50"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCurrency(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		pct      float64
		expected string
	}{
		{5.0, "+5.00%"},
		{0.0, "+0.00%"},
		{-5.0, "-5.00%"},
		{100.0, "+100.00%"},
		{-100.0, "-100.00%"},
		{0.5, "+0.50%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatPercent(tt.pct)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPnL(t *testing.T) {
	tests := []struct {
		amount   float64
		expected string
	}{
		{100.0, "+$100.00"},
		{0.0, "+$0.00"},
		{-100.0, "-$100.00"},
		{1000.50, "+$1000.50"},
		{-1000.50, "-$1000.50"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatPnL(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "symbol",
		Message: "Invalid format",
	}

	assert.Equal(t, "symbol: Invalid format", err.Error())
	assert.Contains(t, err.Error(), "symbol")
	assert.Contains(t, err.Error(), "Invalid format")
}
