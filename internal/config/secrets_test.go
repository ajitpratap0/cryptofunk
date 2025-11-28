package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSecret_Empty(t *testing.T) {
	result := ValidateSecret("", "test_secret", 12, true)
	assert.False(t, result.IsValid)
	assert.Equal(t, SecretStrengthWeak, result.Strength)
	assert.Contains(t, result.Errors[0], "cannot be empty")
}

func TestValidateSecret_Placeholders(t *testing.T) {
	placeholders := []string{
		"changeme",
		"CHANGEME",
		"please_change_me",
		"your_api_key",
		"test123",
		"password",
		"admin123",
	}

	for _, placeholder := range placeholders {
		t.Run(placeholder, func(t *testing.T) {
			result := ValidateSecret(placeholder, "test_secret", 12, true)
			assert.False(t, result.IsValid)
			assert.Equal(t, SecretStrengthWeak, result.Strength)
			assert.NotEmpty(t, result.Errors)
		})
	}
}

func TestValidateSecret_CommonWeakPasswords(t *testing.T) {
	weakPasswords := []string{
		"123456",
		"12345678",
		"qwerty",
		"letmein",
	}

	for _, weak := range weakPasswords {
		t.Run(weak, func(t *testing.T) {
			result := ValidateSecret(weak, "test_secret", 12, true)
			assert.False(t, result.IsValid)
			assert.Equal(t, SecretStrengthWeak, result.Strength)
			// Should contain either "weak password" or "placeholder" (both are caught)
			assert.NotEmpty(t, result.Errors)
		})
	}
}

func TestValidateSecret_TooShort(t *testing.T) {
	result := ValidateSecret("short", "test_secret", 12, true)
	assert.False(t, result.IsValid)
	assert.Contains(t, result.Errors[0], "at least 12 characters")
}

func TestValidateSecret_WeakStrength(t *testing.T) {
	// Only lowercase, meets length but weak composition
	result := ValidateSecret("abcdefghijkl", "test_secret", 12, true)
	assert.False(t, result.IsValid)
	assert.Equal(t, SecretStrengthWeak, result.Strength)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateSecret_MediumStrength(t *testing.T) {
	// 12 chars, 2 types (lowercase + numbers) - no sequential chars
	result := ValidateSecret("h7j2p9k4m6q8", "test_secret", 12, false)
	assert.True(t, result.IsValid)
	assert.Equal(t, SecretStrengthMedium, result.Strength)
}

func TestValidateSecret_StrongPassword(t *testing.T) {
	strongPasswords := []string{
		"MyP@ssw0rd12345!",       // 16 chars, 4 types
		"Tr0ng_P@ssw0rd_2024",    // 19 chars, 4 types
		"Secure!Database#Pass99", // 22 chars, 4 types
		"aB3$fG7*jK9@mN2pQr",     // 18 chars, 4 types
	}

	for _, strong := range strongPasswords {
		t.Run(strong, func(t *testing.T) {
			result := ValidateSecret(strong, "test_secret", 12, true)
			assert.True(t, result.IsValid, "Password should be valid: %v", result.Errors)
			assert.Equal(t, SecretStrengthStrong, result.Strength)
			assert.Empty(t, result.Errors)
		})
	}
}

func TestValidateSecret_SequentialChars(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hasWarn  bool
	}{
		{"sequential numbers", "MyPass123word", true},
		{"sequential letters", "MyPassabcword", true},
		{"no sequential", "MyP@ssw0rd!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecret(tt.password, "test_secret", 12, false)
			if tt.hasWarn {
				assert.NotEmpty(t, result.Warnings)
				assert.Contains(t, result.Warnings[0], "sequential")
			}
		})
	}
}

func TestValidateSecret_RepeatedChars(t *testing.T) {
	result := ValidateSecret("MyPaaassword", "test_secret", 12, false)
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "repeated")
}

func TestValidateSecret_NotRequireStrong(t *testing.T) {
	// Weak password but requireStrong=false
	result := ValidateSecret("simplepass", "test_secret", 8, false)
	assert.True(t, result.IsValid) // Should be valid when not requiring strong
	assert.Equal(t, SecretStrengthWeak, result.Strength)
}

func TestValidateProductionSecrets(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectError bool
		errorField  string
	}{
		{
			name: "valid production secrets",
			cfg: &Config{
				App: AppConfig{Environment: "production"},
				Database: DatabaseConfig{
					Password: "MyStr0ng_P@ssw0rd!",
				},
				Redis: RedisConfig{
					Password: "RedisStr0ng_P@ss!",
				},
				Exchanges: map[string]ExchangeConfig{
					"binance": {
						APIKey:    "bI9nX4pQ2vL7mR5wK8zF3g",
						SecretKey: "sK9tY4qP2hL7nR5wJ8zC3m",
					},
				},
			},
			expectError: false,
		},
		{
			name: "weak database password",
			cfg: &Config{
				App: AppConfig{Environment: "production"},
				Database: DatabaseConfig{
					Password: "weak",
				},
			},
			expectError: true,
			errorField:  "database.password",
		},
		{
			name: "placeholder database password",
			cfg: &Config{
				App: AppConfig{Environment: "production"},
				Database: DatabaseConfig{
					Password: "changeme",
				},
			},
			expectError: true,
			errorField:  "database.password",
		},
		{
			name: "weak redis password",
			cfg: &Config{
				App: AppConfig{Environment: "production"},
				Database: DatabaseConfig{
					Password: "MyStr0ng_P@ssw0rd!",
				},
				Redis: RedisConfig{
					Password: "123456",
				},
			},
			expectError: true,
			errorField:  "redis.password",
		},
		{
			name: "placeholder exchange key",
			cfg: &Config{
				App: AppConfig{Environment: "production"},
				Database: DatabaseConfig{
					Password: "MyStr0ng_P@ssw0rd!",
				},
				Exchanges: map[string]ExchangeConfig{
					"binance": {
						APIKey:    "test",
						SecretKey: "sK9tY4qP2hL7nR5wJ8zC3m",
					},
				},
			},
			expectError: true,
			errorField:  "exchanges.binance.api_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateProductionSecrets(tt.cfg)
			if tt.expectError {
				assert.NotEmpty(t, errors)
				found := false
				for _, err := range errors {
					if err.Field == tt.errorField {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error for field %s", tt.errorField)
			} else {
				assert.Empty(t, errors)
			}
		})
	}
}

func TestHasSequentialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", true},
		{"123abc", true},
		{"def456", true},
		{"random123", true},
		{"xyz789", true},
		{"AbC123", true},  // Case-insensitive
		{"a1b2c3", false}, // Not sequential
		{"random", false},
		{"135", false}, // Not sequential
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := hasSequentialChars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasRepeatedChars(t *testing.T) {
	tests := []struct {
		input    string
		n        int
		expected bool
	}{
		{"aaa", 3, true},
		{"aaab", 3, true},
		{"baaa", 3, true},
		{"aabb", 3, false},
		{"abcabc", 3, false},
		{"aaaa", 3, true},
		{"111", 3, true},
		{"1122", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := hasRepeatedChars(tt.input, tt.n)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSecretStrengthDescription(t *testing.T) {
	assert.Equal(t, "Weak", GetSecretStrengthDescription(SecretStrengthWeak))
	assert.Equal(t, "Medium", GetSecretStrengthDescription(SecretStrengthMedium))
	assert.Equal(t, "Strong", GetSecretStrengthDescription(SecretStrengthStrong))
}

func TestValidateSecret_CharacterComposition(t *testing.T) {
	tests := []struct {
		name             string
		password         string
		expectedStrength SecretStrength
		minLength        int
		requireStrong    bool
		expectValid      bool
	}{
		{
			name:             "only lowercase",
			password:         "abcdefghijklmnop",
			expectedStrength: SecretStrengthWeak,
			minLength:        12,
			requireStrong:    true,
			expectValid:      false,
		},
		{
			name:             "lowercase + numbers",
			password:         "h7j2p9k4m6q8",
			expectedStrength: SecretStrengthMedium,
			minLength:        12,
			requireStrong:    false, // Medium is acceptable when not requiring strong
			expectValid:      true,
		},
		{
			name:             "lowercase + uppercase + numbers",
			password:         "H7J2P9K4M6Q8",
			expectedStrength: SecretStrengthMedium,
			minLength:        12,
			requireStrong:    false, // Medium is acceptable when not requiring strong
			expectValid:      true,
		},
		{
			name:             "all four types, short",
			password:         "Ab1!cdef",
			expectedStrength: SecretStrengthWeak,
			minLength:        12,
			requireStrong:    true,
			expectValid:      false,
		},
		{
			name:             "all four types, long",
			password:         "Ab1!cdefghijklmn",
			expectedStrength: SecretStrengthStrong,
			minLength:        12,
			requireStrong:    true,
			expectValid:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecret(tt.password, "test", tt.minLength, tt.requireStrong)
			assert.Equal(t, tt.expectedStrength, result.Strength)
			assert.Equal(t, tt.expectValid, result.IsValid)
		})
	}
}

func TestValidateSecret_KeyboardPatterns(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectValid bool
		pattern     string
	}{
		// Top row patterns
		{
			name:        "qwerty pattern",
			password:    "qwerty123",
			expectValid: false,
			pattern:     "qwerty",
		},
		{
			name:        "qwertyuiop pattern",
			password:    "MyPass_qwertyuiop",
			expectValid: false,
			pattern:     "qwertyuiop",
		},
		{
			name:        "reversed qwerty",
			password:    "ytrewq123",
			expectValid: false,
			pattern:     "ytrewq",
		},
		{
			name:        "QWERTY uppercase",
			password:    "QWERTY123!",
			expectValid: false,
			pattern:     "qwerty",
		},
		// Middle row patterns
		{
			name:        "asdfgh pattern",
			password:    "asdfgh789",
			expectValid: false,
			pattern:     "asdfgh",
		},
		{
			name:        "asdfghjkl pattern",
			password:    "Pass_asdfghjkl",
			expectValid: false,
			pattern:     "asdfghjkl",
		},
		{
			name:        "reversed asdfgh",
			password:    "hgfdsa456",
			expectValid: false,
			pattern:     "hgfdsa",
		},
		{
			name:        "lkjhgfdsa pattern",
			password:    "lkjhgfdsa123",
			expectValid: false,
			pattern:     "lkjhgfdsa",
		},
		// Bottom row patterns
		{
			name:        "zxcvbn pattern",
			password:    "zxcvbn123",
			expectValid: false,
			pattern:     "zxcvbn",
		},
		{
			name:        "zxcvbnm pattern",
			password:    "zxcvbnm456",
			expectValid: false,
			pattern:     "zxcvbnm",
		},
		{
			name:        "reversed zxcvbn",
			password:    "nbvcxz789",
			expectValid: false,
			pattern:     "nbvcxz",
		},
		{
			name:        "mnbvcxz pattern",
			password:    "mnbvcxz123",
			expectValid: false,
			pattern:     "mnbvcxz",
		},
		// Number sequences
		{
			name:        "12345678 pattern",
			password:    "Pass12345678",
			expectValid: false,
			pattern:     "12345678",
		},
		{
			name:        "123456789 pattern",
			password:    "123456789!",
			expectValid: false,
			pattern:     "123456789",
		},
		{
			name:        "reversed 87654321",
			password:    "Pass87654321",
			expectValid: false,
			pattern:     "87654321",
		},
		{
			name:        "reversed 987654321",
			password:    "987654321!",
			expectValid: false,
			pattern:     "987654321",
		},
		// Diagonal patterns
		{
			name:        "qazwsx pattern",
			password:    "qazwsx123",
			expectValid: false,
			pattern:     "qazwsx",
		},
		{
			name:        "1qaz2wsx pattern",
			password:    "1qaz2wsx!",
			expectValid: false,
			pattern:     "1qaz2wsx",
		},
		// Valid passwords without keyboard patterns
		{
			name:        "no keyboard pattern 1",
			password:    "MyStr0ng_P@ssw0rd!",
			expectValid: true,
			pattern:     "",
		},
		{
			name:        "no keyboard pattern 2",
			password:    "aB3$fG7*jK9@mN2pQr",
			expectValid: true,
			pattern:     "",
		},
		{
			name:        "no keyboard pattern 3",
			password:    "R@nd0m!Secure#Pass99",
			expectValid: true,
			pattern:     "",
		},
		// Edge cases - keyboard pattern embedded
		{
			name:        "embedded qwerty",
			password:    "Prefixqwertysuffix",
			expectValid: false,
			pattern:     "qwerty",
		},
		{
			name:        "embedded asdfgh",
			password:    "X_asdfgh_Y",
			expectValid: false,
			pattern:     "asdfgh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecret(tt.password, "test_secret", 8, true)
			assert.Equal(t, tt.expectValid, result.IsValid, "Password: %s, Errors: %v", tt.password, result.Errors)
			if !tt.expectValid && tt.pattern != "" {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(err, "keyboard pattern") && strings.Contains(err, tt.pattern) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected keyboard pattern error for '%s', got errors: %v", tt.pattern, result.Errors)
			}
		})
	}
}

func TestContainsKeyboardPattern(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedPattern string
	}{
		{"qwerty found", "qwerty123", "qwerty"},
		{"qwertyuiop found", "mypassqwertyuiop", "qwertyuiop"},
		{"asdfgh found", "asdfgh456", "asdfgh"},
		{"asdfghjkl found", "testasdfghjkl", "asdfghjkl"},
		{"zxcvbn found", "zxcvbn789", "zxcvbn"},
		{"zxcvbnm found", "zxcvbnm123", "zxcvbnm"},
		{"reversed ytrewq found", "ytrewq", "ytrewq"},
		{"reversed hgfdsa found", "hgfdsa", "hgfdsa"},
		{"reversed nbvcxz found", "nbvcxz", "nbvcxz"},
		{"12345678 found", "pass12345678", "12345678"},
		{"123456789 found", "123456789x", "123456789"},
		{"reversed 87654321 found", "87654321", "87654321"},
		{"qazwsx found", "qazwsx", "qazwsx"},
		{"no pattern", "R@nd0m!Pass", ""},
		{"no pattern with numbers", "P@ss12W0rd34", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsKeyboardPattern(strings.ToLower(tt.input))
			assert.Equal(t, tt.expectedPattern, result)
		})
	}
}
