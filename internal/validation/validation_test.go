package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator_Required(t *testing.T) {
	v := NewValidator()

	v.Required("field", "")
	assert.True(t, v.HasErrors())
	assert.Equal(t, "field", v.Errors()[0].Field)
	assert.Contains(t, v.Errors()[0].Message, "required")

	v = NewValidator()
	v.Required("field", "  ")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.Required("field", "value")
	assert.False(t, v.HasErrors())
}

func TestValidator_MinLength(t *testing.T) {
	v := NewValidator()

	v.MinLength("field", "ab", 3)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.MinLength("field", "abc", 3)
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.MinLength("field", "abcd", 3)
	assert.False(t, v.HasErrors())
}

func TestValidator_MaxLength(t *testing.T) {
	v := NewValidator()

	v.MaxLength("field", "abcd", 3)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.MaxLength("field", "abc", 3)
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.MaxLength("field", "ab", 3)
	assert.False(t, v.HasErrors())
}

func TestValidator_MinValue(t *testing.T) {
	v := NewValidator()

	v.MinValue("field", 5.0, 10.0)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.MinValue("field", 10.0, 10.0)
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.MinValue("field", 15.0, 10.0)
	assert.False(t, v.HasErrors())
}

func TestValidator_MaxValue(t *testing.T) {
	v := NewValidator()

	v.MaxValue("field", 15.0, 10.0)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.MaxValue("field", 10.0, 10.0)
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.MaxValue("field", 5.0, 10.0)
	assert.False(t, v.HasErrors())
}

func TestValidator_Positive(t *testing.T) {
	v := NewValidator()

	v.Positive("field", -1.0)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.Positive("field", 0.0)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.Positive("field", 1.0)
	assert.False(t, v.HasErrors())
}

func TestValidator_NonNegative(t *testing.T) {
	v := NewValidator()

	v.NonNegative("field", -1.0)
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.NonNegative("field", 0.0)
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.NonNegative("field", 1.0)
	assert.False(t, v.HasErrors())
}

func TestValidator_OneOf(t *testing.T) {
	v := NewValidator()

	v.OneOf("field", "invalid", []string{"a", "b", "c"})
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.OneOf("field", "b", []string{"a", "b", "c"})
	assert.False(t, v.HasErrors())
}

func TestValidator_Email(t *testing.T) {
	v := NewValidator()

	v.Email("field", "invalid")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.Email("field", "user@example.com")
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.Email("field", "user.name+tag@example.co.uk")
	assert.False(t, v.HasErrors())
}

func TestValidator_UUID(t *testing.T) {
	v := NewValidator()

	v.UUID("field", "invalid")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.UUID("field", "550e8400-e29b-41d4-a716-446655440000")
	assert.False(t, v.HasErrors())
}

func TestValidator_Symbol(t *testing.T) {
	v := NewValidator()

	v.Symbol("field", "invalid")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.Symbol("field", "BTC/USDT")
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.Symbol("field", "ETH/BTC")
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.Symbol("field", "btc/usdt") // lowercase should fail
	assert.True(t, v.HasErrors())
}

func TestValidator_Alphanumeric(t *testing.T) {
	v := NewValidator()

	v.Alphanumeric("field", "abc123")
	assert.False(t, v.HasErrors())

	v = NewValidator()
	v.Alphanumeric("field", "abc-123")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.Alphanumeric("field", "abc 123")
	assert.True(t, v.HasErrors())
}

func TestValidator_NoSpecialChars(t *testing.T) {
	v := NewValidator()

	v.NoSpecialChars("field", "normal text 123")
	assert.False(t, v.HasErrors())

	// SQL injection attempts
	v = NewValidator()
	v.NoSpecialChars("field", "'; DROP TABLE users; --")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.NoSpecialChars("field", "<script>alert('xss')</script>")
	assert.True(t, v.HasErrors())

	v = NewValidator()
	v.NoSpecialChars("field", "SELECT * FROM users")
	assert.True(t, v.HasErrors())
}

func TestTradingOrderValidator_ValidateOrderSide(t *testing.T) {
	v := NewTradingOrderValidator()

	v.ValidateOrderSide("")
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateOrderSide("INVALID")
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateOrderSide("BUY")
	assert.False(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateOrderSide("SELL")
	assert.False(t, v.HasErrors())
}

func TestTradingOrderValidator_ValidateOrderType(t *testing.T) {
	v := NewTradingOrderValidator()

	v.ValidateOrderType("")
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateOrderType("INVALID")
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateOrderType("MARKET")
	assert.False(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateOrderType("LIMIT")
	assert.False(t, v.HasErrors())
}

func TestTradingOrderValidator_ValidateQuantity(t *testing.T) {
	v := NewTradingOrderValidator()

	v.ValidateQuantity(0)
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateQuantity(-1)
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateQuantity(2000000) // Exceeds max
	assert.True(t, v.HasErrors())

	v = NewTradingOrderValidator()
	v.ValidateQuantity(0.01)
	assert.False(t, v.HasErrors())
}

func TestTradingSessionValidator_ValidateMode(t *testing.T) {
	v := NewTradingSessionValidator()

	v.ValidateMode("")
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateMode("INVALID")
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateMode("PAPER")
	assert.False(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateMode("LIVE")
	assert.False(t, v.HasErrors())
}

func TestTradingSessionValidator_ValidateExchange(t *testing.T) {
	v := NewTradingSessionValidator()

	v.ValidateExchange("")
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateExchange("a") // Too short
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateExchange("binance")
	assert.False(t, v.HasErrors())
}

func TestTradingSessionValidator_ValidateInitialCapital(t *testing.T) {
	v := NewTradingSessionValidator()

	v.ValidateInitialCapital(0)
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateInitialCapital(50) // Below minimum
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateInitialCapital(20000000) // Above maximum
	assert.True(t, v.HasErrors())

	v = NewTradingSessionValidator()
	v.ValidateInitialCapital(10000)
	assert.False(t, v.HasErrors())
}

func TestConfigValidator_ValidateRiskSettings(t *testing.T) {
	v := NewConfigValidator()

	v.ValidateRiskSettings(-1, 0, 0, 0) // Negative position size
	assert.True(t, v.HasErrors())

	v = NewConfigValidator()
	v.ValidateRiskSettings(0, 150, 0, 0) // Daily loss > 100%
	assert.True(t, v.HasErrors())

	v = NewConfigValidator()
	v.ValidateRiskSettings(0, 0, 150, 0) // Drawdown > 100%
	assert.True(t, v.HasErrors())

	v = NewConfigValidator()
	v.ValidateRiskSettings(0, 0, 0, 1.5) // Confidence > 1
	assert.True(t, v.HasErrors())

	v = NewConfigValidator()
	v.ValidateRiskSettings(1000, 5, 10, 0.7)
	assert.False(t, v.HasErrors())
}

func TestSanitizeInput(t *testing.T) {
	// Test null byte removal
	input := "test\x00value"
	sanitized := SanitizeInput(input)
	assert.Equal(t, "testvalue", sanitized)

	// Test whitespace trimming
	input = "  test  "
	sanitized = SanitizeInput(input)
	assert.Equal(t, "test", sanitized)

	// Test length limiting
	longInput := make([]byte, 15000)
	for i := range longInput {
		longInput[i] = 'a'
	}
	input = string(longInput)
	sanitized = SanitizeInput(input)
	assert.Equal(t, 10000, len(sanitized))
}

func TestSanitizeSymbol(t *testing.T) {
	// Test lowercase conversion
	symbol := "btc/usdt"
	sanitized := SanitizeSymbol(symbol)
	assert.Equal(t, "BTC/USDT", sanitized)

	// Test whitespace removal
	symbol = "BTC / USDT"
	sanitized = SanitizeSymbol(symbol)
	assert.Equal(t, "BTC/USDT", sanitized)

	// Test auto-splitting (simple case)
	symbol = "BTCUSDT"
	sanitized = SanitizeSymbol(symbol)
	assert.Contains(t, sanitized, "/")
}

func TestValidationErrors(t *testing.T) {
	errors := ValidationErrors{}
	assert.False(t, errors.HasErrors())
	assert.Equal(t, "", errors.Error())

	errors = ValidationErrors{
		ValidationError{Field: "field1", Message: "error1"},
	}
	assert.True(t, errors.HasErrors())
	assert.Contains(t, errors.Error(), "field1")

	errors = ValidationErrors{
		ValidationError{Field: "field1", Message: "error1"},
		ValidationError{Field: "field2", Message: "error2"},
	}
	assert.True(t, errors.HasErrors())
	assert.Contains(t, errors.Error(), "field1")
	assert.Contains(t, errors.Error(), "field2")
}
