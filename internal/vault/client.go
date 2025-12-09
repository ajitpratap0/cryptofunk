// Package vault provides a client for HashiCorp Vault integration.
// It enables CryptoFunk services to retrieve secrets from Vault.
package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents a Vault client for retrieving secrets.
type Client struct {
	address    string
	token      string
	httpClient *http.Client
	cache      map[string]*cachedSecret
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
}

// cachedSecret holds a cached secret with expiry.
type cachedSecret struct {
	data      map[string]interface{}
	expiresAt time.Time
}

// SecretData represents the data portion of a KV v2 secret.
type SecretData struct {
	Data     map[string]interface{} `json:"data"`
	Metadata struct {
		CreatedTime  string `json:"created_time"`
		Version      int    `json:"version"`
		Destroyed    bool   `json:"destroyed"`
		DeletionTime string `json:"deletion_time"`
	} `json:"metadata"`
}

// SecretResponse represents a Vault KV v2 secret response.
type SecretResponse struct {
	RequestID     string      `json:"request_id"`
	LeaseID       string      `json:"lease_id"`
	Renewable     bool        `json:"renewable"`
	LeaseDuration int         `json:"lease_duration"`
	Data          *SecretData `json:"data"`
	Errors        []string    `json:"errors"`
}

// Config holds Vault client configuration.
type Config struct {
	Address  string        // Vault server address (default: http://localhost:8200)
	Token    string        // Vault token for authentication
	CacheTTL time.Duration // How long to cache secrets (default: 5 minutes)
	Timeout  time.Duration // HTTP client timeout (default: 30 seconds)
}

// NewClient creates a new Vault client.
func NewClient(cfg Config) (*Client, error) {
	// Use environment variables as defaults
	if cfg.Address == "" {
		cfg.Address = os.Getenv("VAULT_ADDR")
		if cfg.Address == "" {
			cfg.Address = "http://localhost:8200"
		}
	}

	if cfg.Token == "" {
		cfg.Token = os.Getenv("VAULT_TOKEN")
		if cfg.Token == "" {
			cfg.Token = os.Getenv("VAULT_DEV_TOKEN")
		}
	}

	if cfg.Token == "" {
		return nil, fmt.Errorf("vault token is required (set VAULT_TOKEN or VAULT_DEV_TOKEN)")
	}

	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		address: cfg.Address,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		cache:    make(map[string]*cachedSecret),
		cacheTTL: cfg.CacheTTL,
	}, nil
}

// GetSecret retrieves a secret from Vault KV v2 engine.
// path should be the logical path like "cryptofunk/data/database"
func (c *Client) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	// Check cache first
	if cached := c.getCached(path); cached != nil {
		return cached, nil
	}

	// Build the URL for KV v2 (note: data is in the path for KV v2)
	url := fmt.Sprintf("%s/v1/%s", c.address, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secret from vault: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(body))
	}

	var secretResp SecretResponse
	if err := json.Unmarshal(body, &secretResp); err != nil {
		return nil, fmt.Errorf("failed to parse secret response: %w", err)
	}

	if len(secretResp.Errors) > 0 {
		return nil, fmt.Errorf("vault errors: %v", secretResp.Errors)
	}

	if secretResp.Data == nil || secretResp.Data.Data == nil {
		return nil, fmt.Errorf("secret not found at path: %s", path)
	}

	// Cache the result
	c.setCached(path, secretResp.Data.Data)

	return secretResp.Data.Data, nil
}

// GetSecretString retrieves a specific string value from a secret.
func (c *Client) GetSecretString(ctx context.Context, path, key string) (string, error) {
	data, err := c.GetSecret(ctx, path)
	if err != nil {
		return "", err
	}

	value, ok := data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret at %s", key, path)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("key %q is not a string at %s", key, path)
	}

	return strValue, nil
}

// getCached retrieves a secret from cache if not expired.
func (c *Client) getCached(path string) map[string]interface{} {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	cached, ok := c.cache[path]
	if !ok {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		return nil
	}

	return cached.data
}

// setCached stores a secret in cache.
func (c *Client) setCached(path string, data map[string]interface{}) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache[path] = &cachedSecret{
		data:      data,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
}

// ClearCache clears the secret cache.
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache = make(map[string]*cachedSecret)
}

// Health checks if Vault is healthy.
func (c *Client) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/sys/health", c.address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault health check failed: %w", err)
	}
	defer resp.Body.Close()

	// 200 = initialized, unsealed, active
	// 429 = unsealed, standby
	// 472 = disaster recovery mode replication secondary and active
	// 473 = performance standby
	// 501 = not initialized
	// 503 = sealed
	if resp.StatusCode >= 500 {
		return fmt.Errorf("vault is not healthy: status %d", resp.StatusCode)
	}

	return nil
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

// GetDatabaseConfig retrieves database configuration from Vault.
// Uses the 'cryptofunk' KV v2 mount with path cryptofunk/data/database
func (c *Client) GetDatabaseConfig(ctx context.Context) (*DatabaseConfig, error) {
	data, err := c.GetSecret(ctx, "cryptofunk/data/database")
	if err != nil {
		return nil, fmt.Errorf("failed to get database secret: %w", err)
	}

	cfg := &DatabaseConfig{}

	if v, ok := data["host"].(string); ok {
		cfg.Host = v
	}
	if v, ok := data["port"].(string); ok {
		cfg.Port = v
	}
	if v, ok := data["database"].(string); ok {
		cfg.Database = v
	}
	// Support both 'username' and 'user' field names
	if v, ok := data["username"].(string); ok {
		cfg.Username = v
	} else if v, ok := data["user"].(string); ok {
		cfg.Username = v
	}
	if v, ok := data["password"].(string); ok {
		cfg.Password = v
	}
	if v, ok := data["sslmode"].(string); ok {
		cfg.SSLMode = v
	}

	return cfg, nil
}

// ConnectionString returns a PostgreSQL connection string.
func (cfg *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode,
	)
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

// GetRedisConfig retrieves Redis configuration from Vault.
func (c *Client) GetRedisConfig(ctx context.Context) (*RedisConfig, error) {
	data, err := c.GetSecret(ctx, "cryptofunk/data/redis")
	if err != nil {
		return nil, fmt.Errorf("failed to get redis secret: %w", err)
	}

	cfg := &RedisConfig{}

	if v, ok := data["host"].(string); ok {
		cfg.Host = v
	}
	if v, ok := data["port"].(string); ok {
		cfg.Port = v
	}
	if v, ok := data["password"].(string); ok {
		cfg.Password = v
	}

	return cfg, nil
}

// Address returns Redis address in host:port format.
func (cfg *RedisConfig) Address() string {
	return fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
}

// LLMConfig holds LLM API keys.
type LLMConfig struct {
	AnthropicAPIKey string
	OpenAIAPIKey    string
	GeminiAPIKey    string
}

// GetLLMConfig retrieves LLM API keys from Vault.
func (c *Client) GetLLMConfig(ctx context.Context) (*LLMConfig, error) {
	data, err := c.GetSecret(ctx, "cryptofunk/data/llm")
	if err != nil {
		return nil, fmt.Errorf("failed to get llm secret: %w", err)
	}

	cfg := &LLMConfig{}

	if v, ok := data["anthropic_api_key"].(string); ok {
		cfg.AnthropicAPIKey = v
	}
	if v, ok := data["openai_api_key"].(string); ok {
		cfg.OpenAIAPIKey = v
	}
	if v, ok := data["gemini_api_key"].(string); ok {
		cfg.GeminiAPIKey = v
	}

	return cfg, nil
}

// ExchangeConfig holds exchange API credentials.
type ExchangeConfig struct {
	BinanceAPIKey    string
	BinanceAPISecret string
	CoinGeckoAPIKey  string
}

// GetExchangeConfig retrieves exchange API credentials from Vault.
func (c *Client) GetExchangeConfig(ctx context.Context) (*ExchangeConfig, error) {
	data, err := c.GetSecret(ctx, "cryptofunk/data/exchanges")
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange secret: %w", err)
	}

	cfg := &ExchangeConfig{}

	if v, ok := data["binance_api_key"].(string); ok {
		cfg.BinanceAPIKey = v
	}
	if v, ok := data["binance_api_secret"].(string); ok {
		cfg.BinanceAPISecret = v
	}
	if v, ok := data["coingecko_api_key"].(string); ok {
		cfg.CoinGeckoAPIKey = v
	}

	return cfg, nil
}

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Secret string
}

// GetJWTConfig retrieves JWT secret from Vault.
func (c *Client) GetJWTConfig(ctx context.Context) (*JWTConfig, error) {
	data, err := c.GetSecret(ctx, "cryptofunk/data/jwt")
	if err != nil {
		return nil, fmt.Errorf("failed to get jwt secret: %w", err)
	}

	cfg := &JWTConfig{}

	if v, ok := data["secret"].(string); ok {
		cfg.Secret = v
	}

	return cfg, nil
}

// NATSConfig holds NATS configuration.
type NATSConfig struct {
	URL string
}

// GetNATSConfig retrieves NATS configuration from Vault.
func (c *Client) GetNATSConfig(ctx context.Context) (*NATSConfig, error) {
	data, err := c.GetSecret(ctx, "cryptofunk/data/nats")
	if err != nil {
		return nil, fmt.Errorf("failed to get nats secret: %w", err)
	}

	cfg := &NATSConfig{}

	if v, ok := data["url"].(string); ok {
		cfg.URL = v
	}

	return cfg, nil
}

// MustNewClient creates a new Vault client or panics.
func MustNewClient(cfg Config) *Client {
	client, err := NewClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Vault client")
	}
	return client
}

// NewClientFromEnv creates a new Vault client using environment variables.
func NewClientFromEnv() (*Client, error) {
	return NewClient(Config{})
}
