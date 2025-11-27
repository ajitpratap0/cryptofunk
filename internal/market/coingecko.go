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
	"golang.org/x/time/rate"
)

const (
	defaultTimeout       = 30 * time.Second
	defaultRateLimit     = 50 // requests per minute for free tier
	defaultMaxRetries    = 3
	defaultRetryDelay    = time.Second
	defaultMaxRetryDelay = 30 * time.Second
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
}

// CoinGeckoClientOptions configures the CoinGecko client
type CoinGeckoClientOptions struct {
	MCPURL             string        // MCP server URL
	APIKey             string        // Optional API key for pro tier
	Timeout            time.Duration // Request timeout
	RateLimit          int           // Requests per minute
	MaxRetries         int           // Maximum retry attempts
	RetryDelay         time.Duration // Initial retry delay
	EnableRateLimiting bool          // Enable rate limiting
}

// NewCoinGeckoClient creates a new CoinGecko MCP client with rate limiting and retry logic
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
		rateLimiter = rate.NewLimiter(rate.Limit(rps), opts.RateLimit)
		log.Debug().
			Float64("rate_per_second", rps).
			Int("burst", opts.RateLimit).
			Msg("Rate limiter configured")
	}

	client := &CoinGeckoClient{
		mcpClient:   mcpClient,
		rateLimiter: rateLimiter,
		timeout:     opts.Timeout,
		maxRetries:  opts.MaxRetries,
		retryDelay:  opts.RetryDelay,
		connected:   false,
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
func (c *CoinGeckoClient) GetPrice(ctx context.Context, symbol string, vsCurrency string) (*PriceResult, error) {
	log.Debug().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Msg("Fetching price from CoinGecko MCP")

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
	var result map[string]map[string]float64
	err := c.callToolWithRetry(ctx, "get_price", args, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get price: %w", err)
	}

	// Extract price from nested map structure
	symbolData, ok := result[symbol]
	if !ok {
		return nil, fmt.Errorf("symbol %s not found in response", symbol)
	}

	price, ok := symbolData[vsCurrency]
	if !ok {
		return nil, fmt.Errorf("currency %s not found for symbol %s", vsCurrency, symbol)
	}

	log.Info().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Float64("price", price).
		Msg("Price fetched successfully")

	return &PriceResult{
		Symbol:   symbol,
		Price:    price,
		Currency: vsCurrency,
	}, nil
}

// MarketChart represents historical market data
type MarketChart struct{
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
func (c *CoinGeckoClient) GetMarketChart(ctx context.Context, symbol string, days int) (*MarketChart, error) {
	log.Debug().
		Str("symbol", symbol).
		Int("days", days).
		Msg("Fetching market chart from CoinGecko MCP")

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
	var result struct {
		Prices       [][]float64 `json:"prices"`
		MarketCaps   [][]float64 `json:"market_caps"`
		TotalVolumes [][]float64 `json:"total_volumes"`
	}

	err := c.callToolWithRetry(ctx, "get_market_chart", args, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get market chart: %w", err)
	}

	chart := &MarketChart{
		Prices:       make([]PricePoint, 0, len(result.Prices)),
		MarketCaps:   make([]PricePoint, 0, len(result.MarketCaps)),
		TotalVolumes: make([]PricePoint, 0, len(result.TotalVolumes)),
	}

	// Parse prices array
	for _, point := range result.Prices {
		if len(point) >= 2 {
			chart.Prices = append(chart.Prices, PricePoint{
				Timestamp: time.Unix(0, int64(point[0])*int64(time.Millisecond)),
				Value:     point[1],
			})
		}
	}

	// Parse market caps array
	for _, point := range result.MarketCaps {
		if len(point) >= 2 {
			chart.MarketCaps = append(chart.MarketCaps, PricePoint{
				Timestamp: time.Unix(0, int64(point[0])*int64(time.Millisecond)),
				Value:     point[1],
			})
		}
	}

	// Parse volumes array
	for _, point := range result.TotalVolumes {
		if len(point) >= 2 {
			chart.TotalVolumes = append(chart.TotalVolumes, PricePoint{
				Timestamp: time.Unix(0, int64(point[0])*int64(time.Millisecond)),
				Value:     point[1],
			})
		}
	}

	log.Info().
		Str("symbol", symbol).
		Int("days", days).
		Int("price_points", len(chart.Prices)).
		Int("market_cap_points", len(chart.MarketCaps)).
		Int("volume_points", len(chart.TotalVolumes)).
		Msg("Market chart fetched successfully")

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
func (c *CoinGeckoClient) GetCoinInfo(ctx context.Context, coinID string) (*CoinInfo, error) {
	log.Debug().
		Str("coin_id", coinID).
		Msg("Fetching coin info from CoinGecko MCP")

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
	var result struct {
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

	err := c.callToolWithRetry(ctx, "get_coin_info", args, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get coin info: %w", err)
	}

	// Extract homepage link
	homepage := ""
	if len(result.Links.Homepage) > 0 {
		homepage = result.Links.Homepage[0]
	}

	log.Info().
		Str("coin_id", coinID).
		Str("name", result.Name).
		Str("symbol", result.Symbol).
		Msg("Coin info fetched successfully")

	return &CoinInfo{
		ID:          result.ID,
		Symbol:      result.Symbol,
		Name:        result.Name,
		Description: result.Description.En,
		Links:       map[string]string{"homepage": homepage},
		MarketData:  result.MarketData,
	}, nil
}

// callToolWithRetry calls an MCP tool with exponential backoff retry logic
func (c *CoinGeckoClient) callToolWithRetry(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
	c.mu.RLock()
	session := c.session
	c.mu.RUnlock()

	if session == nil {
		return fmt.Errorf("not connected to MCP server")
	}

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
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
				return ctx.Err()
			}
		}

		// Create request context with timeout
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

		// Success
		log.Debug().
			Str("tool", toolName).
			Int("attempt", attempt+1).
			Msg("MCP tool call succeeded")
		return nil
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", c.maxRetries, lastErr)
}

// waitForRateLimit waits until the rate limiter allows the request
func (c *CoinGeckoClient) waitForRateLimit(ctx context.Context) error {
	if c.rateLimiter == nil {
		return nil // Rate limiting disabled
	}

	if err := c.rateLimiter.Wait(ctx); err != nil {
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

			// Start new candle
			intervalStart = pointIntervalStart
			currentCandle = &Candlestick{
				Timestamp: intervalStart,
				Open:      pricePoint.Value,
				High:      pricePoint.Value,
				Low:       pricePoint.Value,
				Close:     pricePoint.Value,
				Volume:    0.0,
			}

			// Add volume if available
			if i < len(m.TotalVolumes) {
				currentCandle.Volume = m.TotalVolumes[i].Value
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

			// Add volume
			if i < len(m.TotalVolumes) {
				currentCandle.Volume += m.TotalVolumes[i].Value
			}
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

// Health checks if the CoinGecko MCP connection is healthy
func (c *CoinGeckoClient) Health(ctx context.Context) error {
	log.Debug().Msg("Checking CoinGecko MCP health")

	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected to MCP server")
	}

	// Try a simple API call to verify connection
	// Use lightweight ping endpoint if available, or get_price for health check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
