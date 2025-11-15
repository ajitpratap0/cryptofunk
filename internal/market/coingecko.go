package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	coinGeckoAPIBase = "https://api.coingecko.com/api/v3"
	defaultTimeout   = 30 * time.Second
)

// CoinGeckoClient wraps the CoinGecko REST API
type CoinGeckoClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

// NewCoinGeckoClient creates a new CoinGecko REST API client
func NewCoinGeckoClient(apiKey string) (*CoinGeckoClient, error) {
	log.Info().Msg("Initializing CoinGecko REST API client")

	return &CoinGeckoClient{
		baseURL: coinGeckoAPIBase,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		timeout: defaultTimeout,
	}, nil
}

// PriceResult represents the response from get_price tool
type PriceResult struct {
	Symbol   string
	Price    float64
	Currency string
}

// GetPrice fetches the current price for a cryptocurrency
func (c *CoinGeckoClient) GetPrice(ctx context.Context, symbol string, vsCurrency string) (*PriceResult, error) {
	log.Debug().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Msg("Fetching price from CoinGecko API")

	// Build request URL: /simple/price?ids=bitcoin&vs_currencies=usd
	params := url.Values{}
	params.Add("ids", symbol)
	params.Add("vs_currencies", vsCurrency)
	if c.apiKey != "" {
		params.Add("x_cg_pro_api_key", c.apiKey)
	}

	reqURL := fmt.Sprintf("%s/simple/price?%s", c.baseURL, params.Encode())

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Best effort close
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response: {symbol: {currency: price}}
	// Example: {"bitcoin": {"usd": 45000.50}}
	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract price
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

// GetMarketChart fetches historical market data
func (c *CoinGeckoClient) GetMarketChart(ctx context.Context, symbol string, days int) (*MarketChart, error) {
	log.Debug().
		Str("symbol", symbol).
		Int("days", days).
		Msg("Fetching market chart from CoinGecko API")

	// Build request URL: /coins/{id}/market_chart?vs_currency=usd&days=7
	params := url.Values{}
	params.Add("vs_currency", "usd")
	params.Add("days", strconv.Itoa(days))
	if c.apiKey != "" {
		params.Add("x_cg_pro_api_key", c.apiKey)
	}

	reqURL := fmt.Sprintf("%s/coins/%s/market_chart?%s", c.baseURL, symbol, params.Encode())

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Best effort close
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	// CoinGecko returns {prices: [[timestamp_ms, price], ...], market_caps: [[timestamp_ms, cap], ...], total_volumes: [[timestamp_ms, volume], ...]}
	var result struct {
		Prices       [][]float64 `json:"prices"`
		MarketCaps   [][]float64 `json:"market_caps"`
		TotalVolumes [][]float64 `json:"total_volumes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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

// GetCoinInfo fetches detailed information about a cryptocurrency
func (c *CoinGeckoClient) GetCoinInfo(ctx context.Context, coinID string) (*CoinInfo, error) {
	log.Debug().
		Str("coin_id", coinID).
		Msg("Fetching coin info from CoinGecko API")

	// Build request URL: /coins/{id}?localization=false&tickers=false&community_data=false&developer_data=false
	params := url.Values{}
	params.Add("localization", "false")
	params.Add("tickers", "false")
	params.Add("community_data", "false")
	params.Add("developer_data", "false")
	if c.apiKey != "" {
		params.Add("x_cg_pro_api_key", c.apiKey)
	}

	reqURL := fmt.Sprintf("%s/coins/%s?%s", c.baseURL, coinID, params.Encode())

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Best effort close
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
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

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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
		// Start new candle if needed
		if currentCandle == nil || pricePoint.Timestamp.Sub(intervalStart) >= intervalDuration {
			// Save previous candle if exists
			if currentCandle != nil {
				candlesticks = append(candlesticks, *currentCandle)
			}

			// Start new candle
			intervalStart = pricePoint.Timestamp.Truncate(intervalDuration)
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

// Health checks if the CoinGecko API connection is healthy
func (c *CoinGeckoClient) Health(ctx context.Context) error {
	log.Debug().Msg("Checking CoinGecko API health")

	if c.httpClient == nil {
		return fmt.Errorf("HTTP client not initialized")
	}

	// Try a simple API call to verify connection
	// Use lightweight ping endpoint if available, or get_price for health check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	log.Debug().Msg("CoinGecko API health check passed")
	return nil
}

// Close closes the HTTP client and releases resources
func (c *CoinGeckoClient) Close() error {
	if c.httpClient != nil {
		// Close idle connections
		c.httpClient.CloseIdleConnections()
		log.Info().Msg("Closed CoinGecko API client")
	}
	return nil
}
