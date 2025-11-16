package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return "validation errors: " + strings.Join(msgs, "; ")
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator provides validation utilities
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// Errors returns all validation errors
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Required validates that a string is not empty
func (v *Validator) Required(field, value string) {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "is required")
	}
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) {
	if len(value) < min {
		v.AddError(field, fmt.Sprintf("must be at least %d characters", min))
	}
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) {
	if len(value) > max {
		v.AddError(field, fmt.Sprintf("must be at most %d characters", max))
	}
}

// MinValue validates minimum numeric value
func (v *Validator) MinValue(field string, value, min float64) {
	if value < min {
		v.AddError(field, fmt.Sprintf("must be at least %v", min))
	}
}

// MaxValue validates maximum numeric value
func (v *Validator) MaxValue(field string, value, max float64) {
	if value > max {
		v.AddError(field, fmt.Sprintf("must be at most %v", max))
	}
}

// Positive validates that a number is positive
func (v *Validator) Positive(field string, value float64) {
	if value <= 0 {
		v.AddError(field, "must be positive")
	}
}

// NonNegative validates that a number is non-negative
func (v *Validator) NonNegative(field string, value float64) {
	if value < 0 {
		v.AddError(field, "must be non-negative")
	}
}

// OneOf validates that a value is one of the allowed values
func (v *Validator) OneOf(field, value string, allowed []string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.AddError(field, fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")))
}

// Email validates email format
func (v *Validator) Email(field, value string) {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		v.AddError(field, "must be a valid email address")
	}
}

// UUID validates UUID format
func (v *Validator) UUID(field, value string) {
	if _, err := uuid.Parse(value); err != nil {
		v.AddError(field, "must be a valid UUID")
	}
}

// Symbol validates trading pair symbol format (e.g., BTC/USDT)
func (v *Validator) Symbol(field, value string) {
	symbolRegex := regexp.MustCompile(`^[A-Z]{2,10}/[A-Z]{2,10}$`)
	if !symbolRegex.MatchString(value) {
		v.AddError(field, "must be a valid symbol (e.g., BTC/USDT)")
	}
}

// Alphanumeric validates that a string contains only alphanumeric characters
func (v *Validator) Alphanumeric(field, value string) {
	alphanumericRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !alphanumericRegex.MatchString(value) {
		v.AddError(field, "must contain only alphanumeric characters")
	}
}

// NoSpecialChars validates that a string doesn't contain special characters that could be used for injection
func (v *Validator) NoSpecialChars(field, value string) {
	// Disallow characters commonly used in injection attacks
	dangerousChars := []string{"<", ">", "'", "\"", ";", "--", "/*", "*/", "DROP", "SELECT", "INSERT", "UPDATE", "DELETE"}
	upperValue := strings.ToUpper(value)
	for _, char := range dangerousChars {
		if strings.Contains(upperValue, char) {
			v.AddError(field, "contains disallowed characters")
			return
		}
	}
}

// TradingOrderValidator validates trading order parameters
type TradingOrderValidator struct {
	*Validator
}

// NewTradingOrderValidator creates a validator for trading orders
func NewTradingOrderValidator() *TradingOrderValidator {
	return &TradingOrderValidator{
		Validator: NewValidator(),
	}
}

// ValidateOrderSide validates order side (BUY/SELL)
func (v *TradingOrderValidator) ValidateOrderSide(side string) {
	v.Required("side", side)
	if v.HasErrors() {
		return
	}
	v.OneOf("side", side, []string{"BUY", "SELL"})
}

// ValidateOrderType validates order type
func (v *TradingOrderValidator) ValidateOrderType(orderType string) {
	v.Required("type", orderType)
	if v.HasErrors() {
		return
	}
	v.OneOf("type", orderType, []string{"MARKET", "LIMIT", "STOP_LOSS", "STOP_LOSS_LIMIT", "TAKE_PROFIT", "TAKE_PROFIT_LIMIT"})
}

// ValidateQuantity validates order quantity
func (v *TradingOrderValidator) ValidateQuantity(quantity float64) {
	v.Positive("quantity", quantity)
	v.MaxValue("quantity", quantity, 1000000) // Reasonable max to prevent errors
}

// ValidatePrice validates order price (for limit orders)
func (v *TradingOrderValidator) ValidatePrice(price float64, required bool) {
	if required {
		v.Positive("price", price)
	} else if price != 0 {
		v.Positive("price", price)
	}
	if price > 0 {
		v.MaxValue("price", price, 10000000) // Reasonable max
	}
}

// ValidateStopPrice validates stop price (for stop orders)
func (v *TradingOrderValidator) ValidateStopPrice(stopPrice float64, required bool) {
	if required {
		v.Positive("stop_price", stopPrice)
	} else if stopPrice != 0 {
		v.Positive("stop_price", stopPrice)
	}
	if stopPrice > 0 {
		v.MaxValue("stop_price", stopPrice, 10000000)
	}
}

// TradingSessionValidator validates trading session parameters
type TradingSessionValidator struct {
	*Validator
}

// NewTradingSessionValidator creates a validator for trading sessions
func NewTradingSessionValidator() *TradingSessionValidator {
	return &TradingSessionValidator{
		Validator: NewValidator(),
	}
}

// ValidateMode validates trading mode
func (v *TradingSessionValidator) ValidateMode(mode string) {
	v.Required("mode", mode)
	if v.HasErrors() {
		return
	}
	v.OneOf("mode", mode, []string{"PAPER", "LIVE"})
}

// ValidateExchange validates exchange name
func (v *TradingSessionValidator) ValidateExchange(exchange string) {
	v.Required("exchange", exchange)
	if v.HasErrors() {
		return
	}
	v.MinLength("exchange", exchange, 2)
	v.MaxLength("exchange", exchange, 50)
	v.Alphanumeric("exchange", strings.ToLower(exchange))
}

// ValidateInitialCapital validates initial capital amount
func (v *TradingSessionValidator) ValidateInitialCapital(capital float64) {
	v.Positive("initial_capital", capital)
	v.MinValue("initial_capital", capital, 100)     // Minimum $100
	v.MaxValue("initial_capital", capital, 10000000) // Max $10M
}

// ConfigValidator validates configuration updates
type ConfigValidator struct {
	*Validator
}

// NewConfigValidator creates a validator for configuration
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		Validator: NewValidator(),
	}
}

// ValidateRiskSettings validates risk management settings
func (v *ConfigValidator) ValidateRiskSettings(maxPositionSize, maxDailyLoss, maxDrawdown, minConfidence float64) {
	if maxPositionSize != 0 {
		v.Positive("max_position_size", maxPositionSize)
		v.MaxValue("max_position_size", maxPositionSize, 1000000)
	}

	if maxDailyLoss != 0 {
		v.Positive("max_daily_loss", maxDailyLoss)
		v.MaxValue("max_daily_loss", maxDailyLoss, 100) // Max 100% daily loss
	}

	if maxDrawdown != 0 {
		v.Positive("max_drawdown", maxDrawdown)
		v.MaxValue("max_drawdown", maxDrawdown, 100) // Max 100% drawdown
	}

	if minConfidence != 0 {
		v.MinValue("min_confidence", minConfidence, 0)
		v.MaxValue("min_confidence", minConfidence, 1) // 0-1 range
	}
}

// ValidateStopLossTakeProfit validates stop loss and take profit percentages
func (v *ConfigValidator) ValidateStopLossTakeProfit(stopLoss, takeProfit float64) {
	if stopLoss != 0 {
		v.Positive("default_stop_loss", stopLoss)
		v.MaxValue("default_stop_loss", stopLoss, 100) // Max 100%
	}

	if takeProfit != 0 {
		v.Positive("default_take_profit", takeProfit)
		v.MaxValue("default_take_profit", takeProfit, 1000) // Max 1000%
	}
}

// SanitizeInput sanitizes user input to prevent injection attacks
func SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	// Limit length to prevent DoS
	if len(input) > 10000 {
		input = input[:10000]
	}

	return input
}

// SanitizeSymbol sanitizes and normalizes a trading symbol
func SanitizeSymbol(symbol string) string {
	// Convert to uppercase
	symbol = strings.ToUpper(symbol)

	// Remove whitespace
	symbol = strings.ReplaceAll(symbol, " ", "")

	// Ensure it contains a slash
	if !strings.Contains(symbol, "/") {
		// Try to split at common positions
		if len(symbol) >= 6 {
			symbol = symbol[:len(symbol)/2] + "/" + symbol[len(symbol)/2:]
		}
	}

	return symbol
}
