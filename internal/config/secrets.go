package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	vault "github.com/hashicorp/vault/api"
	"github.com/rs/zerolog/log"
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
		switch result.Strength {
		case SecretStrengthWeak:
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
		case SecretStrengthMedium:
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

// ================================================
// HashiCorp Vault Integration
// ================================================

// VaultConfig holds Vault connection configuration
type VaultConfig struct {
	Enabled    bool   // Enable Vault integration
	Address    string // Vault server address (e.g., "https://vault.example.com:8200")
	Token      string // Vault authentication token (from VAULT_TOKEN env var or K8s service account)
	AuthMethod string // Authentication method: "token", "kubernetes", "approle"
	MountPath  string // Secrets mount path (default: "secret/")
	SecretPath string // Base path for CryptoFunk secrets (e.g., "cryptofunk/production")
	Namespace  string // Vault namespace (for Vault Enterprise)
}

// VaultClient wraps HashiCorp Vault client for secrets management
type VaultClient struct {
	client *vault.Client
	config VaultConfig
}

// NewVaultClient creates a new Vault client from configuration
func NewVaultClient(cfg VaultConfig) (*VaultClient, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("vault is not enabled in configuration")
	}

	// Create Vault config
	vaultCfg := vault.DefaultConfig()
	vaultCfg.Address = cfg.Address

	// Create Vault client
	client, err := vault.NewClient(vaultCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Set namespace if specified (Vault Enterprise feature)
	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	// Authenticate based on method
	switch cfg.AuthMethod {
	case "token", "":
		// Use token authentication
		if cfg.Token == "" {
			// Try to get token from environment
			cfg.Token = os.Getenv("VAULT_TOKEN")
		}
		if cfg.Token == "" {
			return nil, fmt.Errorf("VAULT_TOKEN not set for token authentication")
		}
		client.SetToken(cfg.Token)

	case "kubernetes":
		// Kubernetes service account authentication
		if err := authenticateKubernetes(client, cfg); err != nil {
			return nil, fmt.Errorf("kubernetes authentication failed: %w", err)
		}

	case "approle":
		// AppRole authentication
		if err := authenticateAppRole(client, cfg); err != nil {
			return nil, fmt.Errorf("AppRole authentication failed: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported Vault auth method: %s", cfg.AuthMethod)
	}

	log.Info().
		Str("address", cfg.Address).
		Str("auth_method", cfg.AuthMethod).
		Str("mount_path", cfg.MountPath).
		Str("secret_path", cfg.SecretPath).
		Msg("Vault client initialized successfully")

	return &VaultClient{
		client: client,
		config: cfg,
	}, nil
}

// GetSecret retrieves a secret from Vault
// path is relative to the configured SecretPath
func (vc *VaultClient) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	// Construct full path
	fullPath := fmt.Sprintf("%s/data/%s/%s", vc.config.MountPath, vc.config.SecretPath, path)

	log.Debug().Str("path", fullPath).Msg("Reading secret from Vault")

	// Read secret
	secret, err := vc.client.Logical().ReadWithContext(ctx, fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret from Vault: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found at path: %s", fullPath)
	}

	// For KV v2, secrets are nested under "data" key
	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		return data, nil
	}

	// For KV v1, return data directly
	return secret.Data, nil
}

// GetSecretString retrieves a single string value from Vault
func (vc *VaultClient) GetSecretString(ctx context.Context, path string, key string) (string, error) {
	data, err := vc.GetSecret(ctx, path)
	if err != nil {
		return "", err
	}

	value, ok := data[key].(string)
	if !ok {
		return "", fmt.Errorf("secret key '%s' not found or not a string at path: %s", key, path)
	}

	return value, nil
}

// LoadSecretsFromVault loads all secrets from Vault into configuration
func LoadSecretsFromVault(ctx context.Context, cfg *Config, vaultCfg VaultConfig) error {
	if !vaultCfg.Enabled {
		log.Info().Msg("Vault integration disabled - using environment variables for secrets")
		return nil
	}

	log.Info().Msg("Loading secrets from HashiCorp Vault...")

	// Create Vault client
	vaultClient, err := NewVaultClient(vaultCfg)
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Load database secrets
	if err := loadDatabaseSecrets(ctx, vaultClient, cfg); err != nil {
		log.Warn().Err(err).Msg("Failed to load database secrets from Vault")
		// Continue - may be using env vars
	}

	// Load Redis secrets
	if err := loadRedisSecrets(ctx, vaultClient, cfg); err != nil {
		log.Warn().Err(err).Msg("Failed to load Redis secrets from Vault")
	}

	// Load exchange API keys
	if err := loadExchangeSecrets(ctx, vaultClient, cfg); err != nil {
		log.Warn().Err(err).Msg("Failed to load exchange secrets from Vault")
	}

	// Load LLM API keys (for Bifrost)
	if err := loadLLMSecrets(ctx, vaultClient, cfg); err != nil {
		log.Warn().Err(err).Msg("Failed to load LLM secrets from Vault")
	}

	log.Info().Msg("Secrets loaded from Vault successfully")
	return nil
}

// loadDatabaseSecrets loads database credentials from Vault
func loadDatabaseSecrets(ctx context.Context, vc *VaultClient, cfg *Config) error {
	secrets, err := vc.GetSecret(ctx, "database")
	if err != nil {
		return err
	}

	if password, ok := secrets["password"].(string); ok && password != "" {
		cfg.Database.Password = password
		log.Info().Msg("✓ Loaded database password from Vault")
	}

	if user, ok := secrets["user"].(string); ok && user != "" {
		cfg.Database.User = user
	}

	return nil
}

// loadRedisSecrets loads Redis credentials from Vault
func loadRedisSecrets(ctx context.Context, vc *VaultClient, cfg *Config) error {
	secrets, err := vc.GetSecret(ctx, "redis")
	if err != nil {
		return err
	}

	if password, ok := secrets["password"].(string); ok && password != "" {
		cfg.Redis.Password = password
		log.Info().Msg("✓ Loaded Redis password from Vault")
	}

	return nil
}

// loadExchangeSecrets loads exchange API keys from Vault
func loadExchangeSecrets(ctx context.Context, vc *VaultClient, cfg *Config) error {
	for exchangeName := range cfg.Exchanges {
		path := fmt.Sprintf("exchanges/%s", exchangeName)
		secrets, err := vc.GetSecret(ctx, path)
		if err != nil {
			log.Warn().Str("exchange", exchangeName).Err(err).Msg("Failed to load exchange secrets")
			continue
		}

		exchangeConfig := cfg.Exchanges[exchangeName]

		if apiKey, ok := secrets["api_key"].(string); ok && apiKey != "" {
			exchangeConfig.APIKey = apiKey
		}

		if secretKey, ok := secrets["secret_key"].(string); ok && secretKey != "" {
			exchangeConfig.SecretKey = secretKey
		}

		cfg.Exchanges[exchangeName] = exchangeConfig
		log.Info().Str("exchange", exchangeName).Msg("✓ Loaded exchange API keys from Vault")
	}

	return nil
}

// loadLLMSecrets loads LLM API keys from Vault (for Bifrost)
func loadLLMSecrets(ctx context.Context, vc *VaultClient, cfg *Config) error {
	secrets, err := vc.GetSecret(ctx, "llm")
	if err != nil {
		return err
	}

	// These are typically used by Bifrost, set as env vars
	if anthropicKey, ok := secrets["anthropic_api_key"].(string); ok && anthropicKey != "" {
		if err := os.Setenv("ANTHROPIC_API_KEY", anthropicKey); err != nil {
			log.Warn().Err(err).Msg("Failed to set ANTHROPIC_API_KEY environment variable")
		} else {
			log.Info().Msg("✓ Loaded Anthropic API key from Vault")
		}
	}

	if openaiKey, ok := secrets["openai_api_key"].(string); ok && openaiKey != "" {
		if err := os.Setenv("OPENAI_API_KEY", openaiKey); err != nil {
			log.Warn().Err(err).Msg("Failed to set OPENAI_API_KEY environment variable")
		} else {
			log.Info().Msg("✓ Loaded OpenAI API key from Vault")
		}
	}

	if geminiKey, ok := secrets["gemini_api_key"].(string); ok && geminiKey != "" {
		if err := os.Setenv("GEMINI_API_KEY", geminiKey); err != nil {
			log.Warn().Err(err).Msg("Failed to set GEMINI_API_KEY environment variable")
		} else {
			log.Info().Msg("✓ Loaded Gemini API key from Vault")
		}
	}

	return nil
}

// authenticateKubernetes performs Kubernetes service account authentication
func authenticateKubernetes(client *vault.Client, cfg VaultConfig) error {
	// Read service account JWT token
	jwtPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	jwt, err := os.ReadFile(jwtPath)
	if err != nil {
		return fmt.Errorf("failed to read service account token: %w", err)
	}

	// Get role from env var or config
	role := os.Getenv("VAULT_K8S_ROLE")
	if role == "" {
		role = "cryptofunk" // Default role
	}

	// Authenticate
	data := map[string]interface{}{
		"jwt":  string(jwt),
		"role": role,
	}

	secret, err := client.Logical().Write("auth/kubernetes/login", data)
	if err != nil {
		return fmt.Errorf("failed to login with Kubernetes auth: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("kubernetes authentication returned no token")
	}

	// Set client token
	client.SetToken(secret.Auth.ClientToken)

	log.Info().
		Str("role", role).
		Msg("Authenticated to Vault using Kubernetes service account")

	return nil
}

// authenticateAppRole performs AppRole authentication
func authenticateAppRole(client *vault.Client, cfg VaultConfig) error {
	roleID := os.Getenv("VAULT_ROLE_ID")
	secretID := os.Getenv("VAULT_SECRET_ID")

	if roleID == "" || secretID == "" {
		return fmt.Errorf("VAULT_ROLE_ID and VAULT_SECRET_ID must be set for AppRole authentication")
	}

	data := map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	secret, err := client.Logical().Write("auth/approle/login", data)
	if err != nil {
		return fmt.Errorf("failed to login with AppRole: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("AppRole authentication returned no token")
	}

	// Set client token
	client.SetToken(secret.Auth.ClientToken)

	log.Info().Msg("Authenticated to Vault using AppRole")

	return nil
}

// GetVaultConfigFromEnv creates VaultConfig from environment variables
func GetVaultConfigFromEnv() VaultConfig {
	enabled := os.Getenv("VAULT_ENABLED") == "true"
	if !enabled {
		return VaultConfig{Enabled: false}
	}

	return VaultConfig{
		Enabled:    true,
		Address:    getEnvOrDefault("VAULT_ADDR", "http://localhost:8200"),
		Token:      os.Getenv("VAULT_TOKEN"),
		AuthMethod: getEnvOrDefault("VAULT_AUTH_METHOD", "token"),
		MountPath:  getEnvOrDefault("VAULT_MOUNT_PATH", "secret"),
		SecretPath: getEnvOrDefault("VAULT_SECRET_PATH", "cryptofunk/production"),
		Namespace:  os.Getenv("VAULT_NAMESPACE"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
