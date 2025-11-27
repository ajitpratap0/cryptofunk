package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

const (
	defaultTimeout       = 30 * time.Second
	defaultRateLimit     = 50 // requests per minute for free tier (CoinGecko allows 10-50 calls/min, we use 50 conservatively)
	defaultMaxRetries    = 3
	defaultRetryDelay    = time.Second
	defaultMaxRetryDelay = 30 * time.Second
	healthCheckTimeout   = 5 * time.Second
	defaultCacheTTL      = 60 * time.Second // Default cache TTL for prices
)

// CoinGeckoClient wraps the CoinGecko MCP server client with rate limiting and retry logic
type CoinGeckoClient struct {
	mcpClient   *mcp.Client
	session     *mcp.ClientSession
	rateLimiter *rate.Limiter
	timeout     time.Duration
	maxRetries  int
	retryDelay  time.Duration
	mu          sync.RWMutex
	connected   bool
	lastError   error              // Last error encountered (protected by mu)
	sfGroup     singleflight.Group // Prevents cache stampede
	cache       *RedisPriceCache   // Optional Redis cache for price data
}

// CoinGeckoClientOptions configures the CoinGecko client
type CoinGeckoClientOptions struct {
	MCPURL             string           // MCP server URL (default: https://mcp.api.coingecko.com/mcp)
	APIKey             string           // Optional API key for CoinGecko Pro tier (currently unused - SDK doesn't support headers yet)
	Timeout            time.Duration    // Request timeout (default: 30s)
	RateLimit          int              // Requests per minute (default: 50 for free tier, increase for Pro)
	MaxRetries         int              // Maximum retry attempts (default: 3)
	RetryDelay         time.Duration    // Initial retry delay with exponential backoff (default: 1s)
	EnableRateLimiting bool             // Enable rate limiting (recommended: true)
	Cache              *RedisPriceCache // Optional Redis cache for price data (default: nil)
}

// NewCoinGeckoClient creates a new CoinGecko MCP client with rate limiting and retry logic.
// The apiKey parameter is reserved for future CoinGecko Pro tier support but is currently
// not used as the MCP SDK doesn't support custom headers yet. Pass an empty string for
// free tier usage.
func NewCoinGeckoClient(apiKey string) (*CoinGeckoClient, error) {
	return NewCoinGeckoClientWithOptions(CoinGeckoClientOptions{
		MCPURL:             "https://mcp.api.coingecko.com/mcp",
		APIKey:             apiKey,
		Timeout:            defaultTimeout,
		RateLimit:          defaultRateLimit,
		MaxRetries:         defaultMaxRetries,
		RetryDelay:         defaultRetryDelay,
		EnableRateLimiting: true,
	})
}

// NewCoinGeckoClientWithOptions creates a new CoinGecko client with custom options
func NewCoinGeckoClientWithOptions(opts CoinGeckoClientOptions) (*CoinGeckoClient, error) {
	log.Info().
		Str("mcp_url", opts.MCPURL).
		Int("rate_limit", opts.RateLimit).
		Bool("has_api_key", opts.APIKey != "").
		Msg("Initializing CoinGecko MCP client")

	// Set defaults
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.RateLimit == 0 {
		opts.RateLimit = defaultRateLimit
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = defaultMaxRetries
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = defaultRetryDelay
	}

	// Create MCP client
	impl := &mcp.Implementation{
		Name:    "cryptofunk-coingecko-client",
		Version: "1.0.0",
	}

	mcpClient := mcp.NewClient(impl, nil)

	// Create rate limiter (token bucket algorithm)
	// Convert requests per minute to requests per second
	rps := float64(opts.RateLimit) / 60.0
	var rateLimiter *rate.Limiter
	if opts.EnableRateLimiting {
		// Burst size should be ~10% of rate limit to prevent stampede
		// For 50 req/min, burst of 5 allows short bursts while maintaining overall rate
		burstSize := opts.RateLimit / 10
		if burstSize < 1 {
			burstSize = 1
		}
		rateLimiter = rate.NewLimiter(rate.Limit(rps), burstSize)
		log.Debug().
			Float64("rate_per_second", rps).
			Int("burst", burstSize).
			Msg("Rate limiter configured")
	}

	client := &CoinGeckoClient{
		mcpClient:   mcpClient,
		rateLimiter: rateLimiter,
		timeout:     opts.Timeout,
		maxRetries:  opts.MaxRetries,
		retryDelay:  opts.RetryDelay,
		connected:   false,
		cache:       opts.Cache, // Optional Redis cache
	}

	// Log cache status
	if client.cache != nil {
		log.Info().Msg("Redis cache enabled for CoinGecko price data")
	} else {
		log.Debug().Msg("Redis cache not configured - using in-memory singleflight only")
	}

	// Connect to CoinGecko MCP server using SSE transport
	if err := client.connect(context.Background(), opts.MCPURL, opts.APIKey); err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	return client, nil
}

// connect establishes connection to the MCP server
func (c *CoinGeckoClient) connect(ctx context.Context, mcpURL, apiKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Create HTTP client with API key header if provided
	httpClient := &http.Client{
		Timeout: c.timeout,
	}

	// Create SSE transport for CoinGecko MCP
	transport := &mcp.SSEClientTransport{
		Endpoint:   mcpURL,
		HTTPClient: httpClient,
	}

	// TODO: Add support for API key headers when SDK supports it
	// For now, CoinGecko MCP free tier should work without API key
	if apiKey != "" {
		log.Debug().Msg("API key support via headers not yet implemented in SDK")
	}

	// Connect to server
	session, err := c.mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	c.session = session
	c.connected = true

	log.Info().
		Str("mcp_url", mcpURL).
		Msg("Connected to CoinGecko MCP server")

	return nil
}

// PriceResult represents the response from get_price tool
type PriceResult struct {
	Symbol   string
	Price    float64
	Currency string
}

// GetPrice fetches the current price for a cryptocurrency using MCP
// Uses Redis cache (if available) and singleflight to prevent cache stampede from concurrent requests
func (c *CoinGeckoClient) GetPrice(ctx context.Context, symbol string, vsCurrency string) (*PriceResult, error) {
	log.Debug().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Msg("Fetching price from CoinGecko MCP")

	// Check Redis cache first (if available)
	if c.cache != nil {
		if price, found := c.cache.Get(ctx, symbol, vsCurrency); found {
			log.Debug().
				Str("symbol", symbol).
				Str("vs_currency", vsCurrency).
				Float64("price", price).
				Msg("Redis cache hit for price")
			return &PriceResult{
				Symbol:   symbol,
				Price:    price,
				Currency: vsCurrency,
			}, nil
		}
		log.Debug().
			Str("symbol", symbol).
			Str("vs_currency", vsCurrency).
			Msg("Redis cache miss - fetching from MCP")
	}

	// Use singleflight to deduplicate concurrent requests for same symbol+currency
	key := fmt.Sprintf("price:%s:%s", symbol, vsCurrency)
	result, err, shared := c.sfGroup.Do(key, func() (interface{}, error) {
		// Apply rate limiting
		if err := c.waitForRateLimit(ctx); err != nil {
			return nil, err
		}

		// Prepare MCP tool call arguments
		args := map[string]interface{}{
			"ids":           symbol,
			"vs_currencies": vsCurrency,
		}

		// Call MCP tool with retry logic
		var apiResult map[string]map[string]float64
		err := c.callToolWithRetry(ctx, "get_price", args, &apiResult)
		if err != nil {
			return nil, fmt.Errorf("failed to get price: %w", err)
		}

		// Extract price from nested map structure
		symbolData, ok := apiResult[symbol]
		if !ok {
			return nil, fmt.Errorf("symbol %s not found in response", symbol)
		}

		price, ok := symbolData[vsCurrency]
		if !ok {
			return nil, fmt.Errorf("currency %s not found for symbol %s", vsCurrency, symbol)
		}

		priceResult := &PriceResult{
			Symbol:   symbol,
			Price:    price,
			Currency: vsCurrency,
		}

		// Store in Redis cache if available (fire-and-forget, don't fail on cache errors)
		if c.cache != nil {
			if cacheErr := c.cache.Set(ctx, symbol, vsCurrency, price); cacheErr != nil {
				log.Warn().
					Err(cacheErr).
					Str("symbol", symbol).
					Str("vs_currency", vsCurrency).
					Msg("Failed to cache price in Redis - continuing anyway")
			}
		}

		return priceResult, nil
	})

	if err != nil {
		return nil, err
	}

	if shared {
		log.Debug().
			Str("symbol", symbol).
			Str("vs_currency", vsCurrency).
			Msg("Price request deduped via singleflight")
	} else {
		log.Info().
			Str("symbol", symbol).
			Str("vs_currency", vsCurrency).
			Float64("price", result.(*PriceResult).Price).
			Msg("Price fetched successfully")
	}

	return result.(*PriceResult), nil
}

// MarketChart represents historical market data
type MarketChart struct {
	Prices       []PricePoint
	MarketCaps   []PricePoint
	TotalVolumes []PricePoint
}

// PricePoint represents a single data point in time
type PricePoint struct {
	Timestamp time.Time
	Value     float64
}

// GetMarketChart fetches historical market data using MCP
// Uses singleflight to prevent cache stampede from concurrent requests
func (c *CoinGeckoClient) GetMarketChart(ctx context.Context, symbol string, days int) (*MarketChart, error) {
	log.Debug().
		Str("symbol", symbol).
		Int("days", days).
		Msg("Fetching market chart from CoinGecko MCP")

	// Use singleflight to deduplicate concurrent requests for same symbol+days
	key := fmt.Sprintf("chart:%s:%d", symbol, days)
	result, err, shared := c.sfGroup.Do(key, func() (interface{}, error) {
		// Apply rate limiting
		if err := c.waitForRateLimit(ctx); err != nil {
			return nil, err
		}

		// Prepare MCP tool call arguments
		args := map[string]interface{}{
			"coin_id":     symbol,
			"vs_currency": "usd",
			"days":        days,
		}

		// Call MCP tool with retry logic
		var apiResult struct {
			Prices       [][]float64 `json:"prices"`
			MarketCaps   [][]float64 `json:"market_caps"`
			TotalVolumes [][]float64 `json:"total_volumes"`
		}

		err := c.callToolWithRetry(ctx, "get_market_chart", args, &apiResult)
		if err != nil {
			return nil, fmt.Errorf("failed to get market chart: %w", err)
		}

		chart := &MarketChart{
			Prices:       make([]PricePoint, 0, len(apiResult.Prices)),
			MarketCaps:   make([]PricePoint, 0, len(apiResult.MarketCaps)),
			TotalVolumes: make([]PricePoint, 0, len(apiResult.TotalVolumes)),
		}

		// Parse prices array
		for _, point := range apiResult.Prices {
			if len(point) >= 2 {
				chart.Prices = append(chart.Prices, PricePoint{
					Timestamp: time.Unix(0, int64(point[0])*int64(time.Millisecond)),
					Value:     point[1],
				})
			}
		}

		// Parse market caps array
		for _, point := range apiResult.MarketCaps {
			if len(point) >= 2 {
				chart.MarketCaps = append(chart.MarketCaps, PricePoint{
					Timestamp: time.Unix(0, int64(point[0])*int64(time.Millisecond)),
					Value:     point[1],
				})
			}
		}

		// Parse volumes array
		for _, point := range apiResult.TotalVolumes {
			if len(point) >= 2 {
				chart.TotalVolumes = append(chart.TotalVolumes, PricePoint{
					Timestamp: time.Unix(0, int64(point[0])*int64(time.Millisecond)),
					Value:     point[1],
				})
			}
		}

		return chart, nil
	})

	if err != nil {
		return nil, err
	}

	chart := result.(*MarketChart)
	if shared {
		log.Debug().
			Str("symbol", symbol).
			Int("days", days).
			Msg("Market chart request deduped via singleflight")
	} else {
		log.Info().
			Str("symbol", symbol).
			Int("days", days).
			Int("price_points", len(chart.Prices)).
			Int("market_cap_points", len(chart.MarketCaps)).
			Int("volume_points", len(chart.TotalVolumes)).
			Msg("Market chart fetched successfully")
	}

	return chart, nil
}

// CoinInfo represents detailed coin information
type CoinInfo struct {
	ID          string
	Symbol      string
	Name        string
	Description string
	Links       map[string]string
	MarketData  map[string]interface{}
}

// GetCoinInfo fetches detailed information about a cryptocurrency using MCP
// Uses singleflight to prevent cache stampede from concurrent requests
func (c *CoinGeckoClient) GetCoinInfo(ctx context.Context, coinID string) (*CoinInfo, error) {
	log.Debug().
		Str("coin_id", coinID).
		Msg("Fetching coin info from CoinGecko MCP")

	// Use singleflight to deduplicate concurrent requests for same coin
	key := fmt.Sprintf("info:%s", coinID)
	result, err, shared := c.sfGroup.Do(key, func() (interface{}, error) {
		// Apply rate limiting
		if err := c.waitForRateLimit(ctx); err != nil {
			return nil, err
		}

		// Prepare MCP tool call arguments
		args := map[string]interface{}{
			"coin_id":        coinID,
			"localization":   false,
			"tickers":        false,
			"community_data": false,
			"developer_data": false,
		}

		// Call MCP tool with retry logic
		var apiResult struct {
			ID          string `json:"id"`
			Symbol      string `json:"symbol"`
			Name        string `json:"name"`
			Description struct {
				En string `json:"en"`
			} `json:"description"`
			Links struct {
				Homepage []string `json:"homepage"`
			} `json:"links"`
			MarketData map[string]interface{} `json:"market_data"`
		}

		err := c.callToolWithRetry(ctx, "get_coin_info", args, &apiResult)
		if err != nil {
			return nil, fmt.Errorf("failed to get coin info: %w", err)
		}

		// Extract homepage link
		homepage := ""
		if len(apiResult.Links.Homepage) > 0 {
			homepage = apiResult.Links.Homepage[0]
		}

		return &CoinInfo{
			ID:          apiResult.ID,
			Symbol:      apiResult.Symbol,
			Name:        apiResult.Name,
			Description: apiResult.Description.En,
			Links:       map[string]string{"homepage": homepage},
			MarketData:  apiResult.MarketData,
		}, nil
	})

	if err != nil {
		return nil, err
	}

	coinInfo := result.(*CoinInfo)
	if shared {
		log.Debug().
			Str("coin_id", coinID).
			Msg("Coin info request deduped via singleflight")
	} else {
		log.Info().
			Str("coin_id", coinID).
			Str("name", coinInfo.Name).
			Str("symbol", coinInfo.Symbol).
			Msg("Coin info fetched successfully")
	}

	return coinInfo, nil
}

// callToolWithRetry calls an MCP tool with exponential backoff retry logic
func (c *CoinGeckoClient) callToolWithRetry(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
	c.mu.RLock()
	session := c.session
	c.mu.RUnlock()

	if session == nil {
		err := fmt.Errorf("not connected to MCP server")
		c.setLastError(err)
		return err
	}

	var lastErr error

	// Fix: Use < instead of <= to get exactly maxRetries attempts
	// With maxRetries=3, this gives attempts 0, 1, 2 (3 total attempts)
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(math.Pow(2, float64(attempt-1))) * c.retryDelay
			if backoffDelay > defaultMaxRetryDelay {
				backoffDelay = defaultMaxRetryDelay
			}

			log.Debug().
				Str("tool", toolName).
				Int("attempt", attempt).
				Dur("backoff", backoffDelay).
				Msg("Retrying MCP tool call after backoff")

			select {
			case <-time.After(backoffDelay):
			case <-ctx.Done():
				err := ctx.Err()
				c.setLastError(err)
				return err
			}
		}

		// Fix: Create context with timeout and ensure proper cleanup with defer
		// This prevents context leak even if panic occurs before cancel() is called
		reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()

		// Call MCP tool
		params := &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		}

		response, err := session.CallTool(reqCtx, params)

		if err != nil {
			lastErr = fmt.Errorf("attempt %d failed: %w", attempt+1, err)
			log.Warn().
				Err(err).
				Str("tool", toolName).
				Int("attempt", attempt+1).
				Msg("MCP tool call failed")
			continue
		}

		// Parse response content
		if len(response.Content) == 0 {
			lastErr = fmt.Errorf("attempt %d: empty response content", attempt+1)
			log.Warn().
				Str("tool", toolName).
				Int("attempt", attempt+1).
				Msg("Empty MCP response")
			continue
		}

		// Extract text content
		content := response.Content[0]
		textContent, ok := content.(*mcp.TextContent)
		if !ok {
			lastErr = fmt.Errorf("attempt %d: unexpected content type", attempt+1)
			continue
		}

		// Unmarshal JSON response
		if err := json.Unmarshal([]byte(textContent.Text), result); err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to unmarshal response: %w", attempt+1, err)
			log.Warn().
				Err(err).
				Str("tool", toolName).
				Int("attempt", attempt+1).
				Msg("Failed to unmarshal MCP response")
			continue
		}

		// Success - clear last error
		c.setLastError(nil)
		log.Debug().
			Str("tool", toolName).
			Int("attempt", attempt+1).
			Msg("MCP tool call succeeded")
		return nil
	}

	finalErr := fmt.Errorf("max retries (%d) exceeded: %w", c.maxRetries, lastErr)
	c.setLastError(finalErr)
	return finalErr
}

// setLastError safely sets the last error with mutex protection
func (c *CoinGeckoClient) setLastError(err error) {
	c.mu.Lock()
	c.lastError = err
	c.mu.Unlock()
}

// waitForRateLimit waits until the rate limiter allows the request
func (c *CoinGeckoClient) waitForRateLimit(ctx context.Context) error {
	if c.rateLimiter == nil {
		return nil // Rate limiting disabled
	}

	// Check if we would need to wait (for logging purposes)
	reservation := c.rateLimiter.Reserve()
	delay := reservation.Delay()

	if delay > 0 {
		log.Debug().
			Dur("wait_duration", delay).
			Msg("Rate limit reached, waiting before API call")
	}

	// Cancel the reservation since we'll use Wait() instead
	reservation.Cancel()

	// Wait for rate limiter to allow the request
	if err := c.rateLimiter.Wait(ctx); err != nil {
		log.Warn().
			Err(err).
			Msg("Rate limit wait interrupted")
		return fmt.Errorf("rate limit wait failed: %w", err)
	}

	return nil
}

// ToCandlesticks converts MarketChart data to OHLCV candlesticks
// Groups price points into time intervals
func (m *MarketChart) ToCandlesticks(intervalMinutes int) []Candlestick {
	if len(m.Prices) == 0 {
		return []Candlestick{}
	}

	candlesticks := make([]Candlestick, 0)
	intervalDuration := time.Duration(intervalMinutes) * time.Minute

	// Group prices by interval
	var currentCandle *Candlestick
	var intervalStart time.Time

	for i, pricePoint := range m.Prices {
		// Determine which interval this price point belongs to
		pointIntervalStart := pricePoint.Timestamp.Truncate(intervalDuration)

		// Start new candle if needed (when we move to a new interval)
		if currentCandle == nil || !pointIntervalStart.Equal(intervalStart) {
			// Save previous candle if exists
			if currentCandle != nil {
				candlesticks = append(candlesticks, *currentCandle)
			}

			// Start new candle with volume initialized to 0
			intervalStart = pointIntervalStart
			currentCandle = &Candlestick{
				Timestamp: intervalStart,
				Open:      pricePoint.Value,
				High:      pricePoint.Value,
				Low:       pricePoint.Value,
				Close:     pricePoint.Value,
				Volume:    0.0, // Always start at 0, then aggregate below
			}
		} else {
			// Update current candle
			if pricePoint.Value > currentCandle.High {
				currentCandle.High = pricePoint.Value
			}
			if pricePoint.Value < currentCandle.Low {
				currentCandle.Low = pricePoint.Value
			}
			currentCandle.Close = pricePoint.Value
		}

		// Fix: Always aggregate volume for ALL price points (including first)
		// This ensures consistent behavior regardless of whether it's a new or existing candle
		if i < len(m.TotalVolumes) {
			currentCandle.Volume += m.TotalVolumes[i].Value
		}
	}

	// Append last candle
	if currentCandle != nil {
		candlesticks = append(candlesticks, *currentCandle)
	}

	return candlesticks
}

// Candlestick represents OHLCV data for a time period
type Candlestick struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// MarshalJSON implements json.Marshaler for Candlestick
func (c *Candlestick) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"timestamp": c.Timestamp.Unix(),
		"open":      c.Open,
		"high":      c.High,
		"low":       c.Low,
		"close":     c.Close,
		"volume":    c.Volume,
	})
}

// SetCache sets or updates the Redis cache for the client
// This allows cache to be configured after client initialization
func (c *CoinGeckoClient) SetCache(cache *RedisPriceCache) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = cache
	if cache != nil {
		log.Info().Msg("Redis cache configured for CoinGecko client")
	} else {
		log.Info().Msg("Redis cache removed from CoinGecko client")
	}
}

// GetCache returns the current Redis cache (if any)
func (c *CoinGeckoClient) GetCache() *RedisPriceCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

// Health checks if the CoinGecko MCP connection is healthy
func (c *CoinGeckoClient) Health(ctx context.Context) error {
	log.Debug().Msg("Checking CoinGecko MCP health")

	c.mu.RLock()
	connected := c.connected
	lastErr := c.lastError
	cache := c.cache
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected to MCP server")
	}

	// If there was a recent error, report it
	if lastErr != nil {
		return fmt.Errorf("recent error detected: %w", lastErr)
	}

	// Check Redis cache health if available (non-blocking)
	if cache != nil {
		if err := cache.Health(ctx); err != nil {
			log.Warn().
				Err(err).
				Msg("Redis cache health check failed - cache may be unavailable")
			// Don't fail the overall health check, just warn
		}
	}

	// Try a simple API call to verify connection
	// Use lightweight ping endpoint if available, or get_price for health check
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	_, err := c.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	log.Debug().Msg("CoinGecko MCP health check passed")
	return nil
}

// Close closes the MCP client and releases resources
func (c *CoinGeckoClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			log.Warn().Err(err).Msg("Error closing MCP session")
		}
		c.session = nil
	}

	c.connected = false
	log.Info().Msg("Closed CoinGecko MCP client")

	return nil
}
