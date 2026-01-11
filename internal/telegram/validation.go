package telegram

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a validation error with a user-friendly message
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator provides input validation for Telegram commands
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidSymbolPattern matches valid trading pair symbols
var ValidSymbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,10}(USDT|BTC|ETH|BNB|BUSD|USDC)$`)

// ValidateSymbol validates a trading symbol
func (v *Validator) ValidateSymbol(symbol string) error {
	if symbol == "" {
		return &ValidationError{
			Field:   "symbol",
			Message: "Symbol cannot be empty",
		}
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	if len(symbol) < 3 || len(symbol) > 15 {
		return &ValidationError{
			Field:   "symbol",
			Message: "Symbol must be between 3 and 15 characters",
		}
	}

	if !ValidSymbolPattern.MatchString(symbol) {
		return &ValidationError{
			Field:   "symbol",
			Message: "Invalid symbol format. Use format like BTCUSDT, ETHUSDT, etc.",
		}
	}

	return nil
}

// ValidateCapital validates initial capital amount
func (v *Validator) ValidateCapital(capital float64) error {
	if capital <= 0 {
		return &ValidationError{
			Field:   "capital",
			Message: "Capital must be a positive number",
		}
	}

	if capital < 10 {
		return &ValidationError{
			Field:   "capital",
			Message: "Minimum capital is $10",
		}
	}

	if capital > 10000000 {
		return &ValidationError{
			Field:   "capital",
			Message: "Maximum capital is $10,000,000",
		}
	}

	return nil
}

// ValidateTradingMode validates the trading mode
func (v *Validator) ValidateTradingMode(mode string) error {
	mode = strings.ToUpper(strings.TrimSpace(mode))

	if mode != "PAPER" && mode != "LIVE" {
		return &ValidationError{
			Field:   "mode",
			Message: "Mode must be either PAPER or LIVE",
		}
	}

	return nil
}

// ValidateVerificationCode validates a verification code format
func (v *Validator) ValidateVerificationCode(code string) error {
	if code == "" {
		return &ValidationError{
			Field:   "code",
			Message: "Verification code cannot be empty",
		}
	}

	code = strings.ToUpper(strings.TrimSpace(code))

	if len(code) < 4 || len(code) > 16 {
		return &ValidationError{
			Field:   "code",
			Message: "Verification code must be between 4 and 16 characters",
		}
	}

	// Only alphanumeric characters allowed
	if matched, _ := regexp.MatchString(`^[A-Z0-9]+$`, code); !matched {
		return &ValidationError{
			Field:   "code",
			Message: "Verification code must contain only letters and numbers",
		}
	}

	return nil
}

// ValidateConfirmation validates a confirmation string
func (v *Validator) ValidateConfirmation(confirmation string) bool {
	confirmation = strings.ToUpper(strings.TrimSpace(confirmation))
	return confirmation == "CONFIRM" || confirmation == "YES" || confirmation == "Y"
}

// SanitizeInput sanitizes user input by removing potentially harmful characters
func SanitizeInput(input string) string {
	// Remove any control characters
	result := strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, input)

	// Trim whitespace
	result = strings.TrimSpace(result)

	// Limit length
	if len(result) > 1000 {
		result = result[:1000]
	}

	return result
}

// ParseCapital parses a capital string to float64
func ParseCapital(capitalStr string) (float64, error) {
	capitalStr = strings.TrimSpace(capitalStr)

	// Remove common currency symbols
	capitalStr = strings.TrimPrefix(capitalStr, "$")
	capitalStr = strings.TrimPrefix(capitalStr, "USD")
	capitalStr = strings.TrimSpace(capitalStr)

	// Remove commas (e.g., 1,000 -> 1000)
	capitalStr = strings.ReplaceAll(capitalStr, ",", "")

	var capital float64
	_, err := fmt.Sscanf(capitalStr, "%f", &capital)
	if err != nil {
		return 0, &ValidationError{
			Field:   "capital",
			Message: "Invalid capital format. Please enter a number like 1000 or 1000.50",
		}
	}

	return capital, nil
}

// FormatCurrency formats a float as a currency string
func FormatCurrency(amount float64) string {
	if amount >= 0 {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("-$%.2f", -amount)
}

// FormatPercent formats a float as a percentage string
func FormatPercent(pct float64) string {
	if pct >= 0 {
		return fmt.Sprintf("+%.2f%%", pct)
	}
	return fmt.Sprintf("%.2f%%", pct)
}

// FormatPnL formats P&L with color indication (for Telegram)
func FormatPnL(amount float64) string {
	if amount >= 0 {
		return fmt.Sprintf("+$%.2f", amount)
	}
	return fmt.Sprintf("-$%.2f", -amount)
}
