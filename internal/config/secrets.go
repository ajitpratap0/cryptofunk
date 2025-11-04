package config

import (
	"fmt"
	"strings"
	"unicode"
)

// SecretStrength represents the strength level of a secret
type SecretStrength int

const (
	SecretStrengthWeak SecretStrength = iota
	SecretStrengthMedium
	SecretStrengthStrong
)

// Common placeholder values that should never be used
var commonPlaceholders = []string{
	"changeme",
	"changeme_in_production",
	"please_change_me",
	"your_api_key",
	"your_secret",
	"test",
	"test123",
	"password",
	"password123",
	"admin",
	"admin123",
	"secret",
	"secret123",
	"postgres",
	"cryptofunk",
	"cryptofunk_grafana",
	"example",
	"sample",
	"demo",
	"localhost",
	"default",
}

// Common weak passwords (subset - full list would be much larger)
var commonWeakPasswords = []string{
	"123456",
	"password",
	"12345678",
	"qwerty",
	"abc123",
	"monkey",
	"letmein",
	"trustno1",
	"dragon",
	"baseball",
	"iloveyou",
	"master",
	"sunshine",
	"ashley",
	"bailey",
	"passw0rd",
	"shadow",
	"123123",
	"654321",
	"superman",
	"qazwsx",
	"michael",
	"football",
}

// SecretValidationResult contains the result of secret validation
type SecretValidationResult struct {
	IsValid  bool
	Strength SecretStrength
	Errors   []string
	Warnings []string
}

// ValidateSecret validates a secret/password for strength and security
// minLength is the minimum acceptable length
// requireStrong determines if strong passwords are required (typically true for production)
func ValidateSecret(secret string, name string, minLength int, requireStrong bool) SecretValidationResult {
	result := SecretValidationResult{
		IsValid:  true,
		Strength: SecretStrengthStrong,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Check if empty
	if secret == "" {
		result.IsValid = false
		result.Strength = SecretStrengthWeak
		result.Errors = append(result.Errors, fmt.Sprintf("%s cannot be empty", name))
		return result
	}

	// Check for placeholders
	lowerSecret := strings.ToLower(secret)
	for _, placeholder := range commonPlaceholders {
		if lowerSecret == placeholder || strings.Contains(lowerSecret, placeholder) {
			result.IsValid = false
			result.Strength = SecretStrengthWeak
			result.Errors = append(result.Errors, fmt.Sprintf("%s appears to be a placeholder value (%s)", name, placeholder))
			return result
		}
	}

	// Check for common weak passwords
	for _, weak := range commonWeakPasswords {
		if lowerSecret == strings.ToLower(weak) {
			result.IsValid = false
			result.Strength = SecretStrengthWeak
			result.Errors = append(result.Errors, fmt.Sprintf("%s is a commonly known weak password", name))
			return result
		}
	}

	// Check length
	if len(secret) < minLength {
		result.IsValid = false
		result.Strength = SecretStrengthWeak
		result.Errors = append(result.Errors, fmt.Sprintf("%s must be at least %d characters (got %d)", name, minLength, len(secret)))
		return result
	}

	// Analyze character composition for strength
	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range secret {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Count character types
	typesCount := 0
	if hasUpper {
		typesCount++
	}
	if hasLower {
		typesCount++
	}
	if hasNumber {
		typesCount++
	}
	if hasSpecial {
		typesCount++
	}

	// Determine strength based on length and character types
	if len(secret) >= 16 && typesCount >= 3 {
		result.Strength = SecretStrengthStrong
	} else if len(secret) >= 12 && typesCount >= 2 {
		result.Strength = SecretStrengthMedium
	} else {
		result.Strength = SecretStrengthWeak
	}

	// Apply strength requirements
	if requireStrong {
		if result.Strength == SecretStrengthWeak {
			result.IsValid = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s is too weak for production use", name))

			// Add specific improvement suggestions
			if len(secret) < 12 {
				result.Errors = append(result.Errors, "- Use at least 12 characters")
			}
			if typesCount < 3 {
				suggestions := []string{}
				if !hasUpper {
					suggestions = append(suggestions, "uppercase letters")
				}
				if !hasLower {
					suggestions = append(suggestions, "lowercase letters")
				}
				if !hasNumber {
					suggestions = append(suggestions, "numbers")
				}
				if !hasSpecial {
					suggestions = append(suggestions, "special characters")
				}
				result.Errors = append(result.Errors, fmt.Sprintf("- Include at least 3 of: %s", strings.Join(suggestions, ", ")))
			}
		} else if result.Strength == SecretStrengthMedium {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s has medium strength - consider using a stronger secret", name))
		}
	}

	// Check for sequential characters (common weakness)
	if hasSequentialChars(secret) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s contains sequential characters (e.g., 123, abc) - consider using more random values", name))
		if result.Strength == SecretStrengthMedium {
			result.Strength = SecretStrengthWeak
			if requireStrong {
				result.IsValid = false
			}
		}
	}

	// Check for repeated characters
	if hasRepeatedChars(secret, 3) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s contains repeated characters - consider using more varied values", name))
	}

	return result
}

// hasSequentialChars checks if the string contains sequential characters
func hasSequentialChars(s string) bool {
	// Check for sequential numbers
	for i := 0; i < len(s)-2; i++ {
		if unicode.IsDigit(rune(s[i])) && unicode.IsDigit(rune(s[i+1])) && unicode.IsDigit(rune(s[i+2])) {
			if (s[i+1] == s[i]+1) && (s[i+2] == s[i]+2) {
				return true
			}
		}
	}

	// Check for sequential letters
	lower := strings.ToLower(s)
	for i := 0; i < len(lower)-2; i++ {
		if (lower[i+1] == lower[i]+1) && (lower[i+2] == lower[i]+2) {
			return true
		}
	}

	return false
}

// hasRepeatedChars checks if the string has the same character repeated n times
func hasRepeatedChars(s string, n int) bool {
	if len(s) < n {
		return false
	}

	for i := 0; i < len(s)-n+1; i++ {
		allSame := true
		for j := 1; j < n; j++ {
			if s[i+j] != s[i] {
				allSame = false
				break
			}
		}
		if allSame {
			return true
		}
	}

	return false
}

// ValidateProductionSecrets validates all secrets for production use
// Returns validation errors if any secrets are weak or invalid
func ValidateProductionSecrets(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	// Minimum length for production secrets
	const minProductionLength = 12

	// Validate database password
	if cfg.Database.Password != "" {
		result := ValidateSecret(cfg.Database.Password, "Database password", minProductionLength, true)
		if !result.IsValid {
			for _, err := range result.Errors {
				errors = append(errors, ValidationError{
					Field:   "database.password",
					Message: err,
				})
			}
		}
	}

	// Validate Redis password (if set)
	if cfg.Redis.Password != "" {
		result := ValidateSecret(cfg.Redis.Password, "Redis password", minProductionLength, true)
		if !result.IsValid {
			for _, err := range result.Errors {
				errors = append(errors, ValidationError{
					Field:   "redis.password",
					Message: err,
				})
			}
		}
	}

	// Validate exchange secrets
	for exchangeName, exchangeConfig := range cfg.Exchanges {
		// API Key validation (less strict - exchange-generated keys may not follow password rules)
		if exchangeConfig.APIKey != "" {
			result := ValidateSecret(exchangeConfig.APIKey, fmt.Sprintf("%s API key", exchangeName), 10, false)
			if !result.IsValid {
				for _, err := range result.Errors {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("exchanges.%s.api_key", exchangeName),
						Message: err,
					})
				}
			}
		}

		// Secret Key validation (less strict - exchange-generated secrets may not follow password rules)
		if exchangeConfig.SecretKey != "" {
			result := ValidateSecret(exchangeConfig.SecretKey, fmt.Sprintf("%s secret key", exchangeName), 10, false)
			if !result.IsValid {
				for _, err := range result.Errors {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("exchanges.%s.secret_key", exchangeName),
						Message: err,
					})
				}
			}
		}
	}

	// Note: LLM API keys are typically long random strings from the provider
	// We don't validate their strength, just check for placeholders
	// This is already handled in validateEnvironmentRequirements

	return errors
}

// GetSecretStrengthDescription returns a human-readable description of secret strength
func GetSecretStrengthDescription(strength SecretStrength) string {
	switch strength {
	case SecretStrengthWeak:
		return "Weak"
	case SecretStrengthMedium:
		return "Medium"
	case SecretStrengthStrong:
		return "Strong"
	default:
		return "Unknown"
	}
}
