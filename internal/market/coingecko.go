package market

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// CoinGeckoClient wraps the CoinGecko MCP server connection
type CoinGeckoClient struct {
	url       string
	transport string
	timeout   time.Duration
	client    *mcp.MCPClient
}

// NewCoinGeckoClient creates a new CoinGecko MCP client
func NewCoinGeckoClient(url string) (*CoinGeckoClient, error) {
	if url == "" {
		return nil, fmt.Errorf("CoinGecko MCP URL is required")
	}

	log.Info().
		Str("url", url).
		Msg("Initializing CoinGecko MCP client")

	// Create MCP client with HTTP streaming transport
	client, err := mcp.NewMCPClient(&mcp.ClientOptions{
		Name:    "cryptofunk-coingecko-client",
		Version: "1.0.0",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// TODO: Connect to HTTP streaming endpoint
	// Note: The MCP Go SDK may need HTTP transport support
	// For now, we'll prepare the structure and log the connection attempt

	return &CoinGeckoClient{
		url:       url,
		transport: "http_streaming",
		timeout:   30 * time.Second,
		client:    client,
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
	log.Info().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Msg("Fetching price from CoinGecko MCP")

	// Call get_price tool via MCP
	result, err := c.client.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_price",
		Arguments: map[string]interface{}{
			"ids":           symbol,
			"vs_currencies": vsCurrency,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from MCP result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from CoinGecko")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type from CoinGecko")
	}

	// Parse JSON result - CoinGecko returns {symbol: {usd: price}}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return nil, fmt.Errorf("failed to parse CoinGecko response: %w", err)
	}

	// Extract price for the symbol
	symbolData, ok := resultMap[symbol].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("symbol data not found for %s", symbol)
	}

	priceInterface, ok := symbolData[vsCurrency]
	if !ok {
		return nil, fmt.Errorf("currency %s not found in response", vsCurrency)
	}

	// Convert price to float64
	var price float64
	switch v := priceInterface.(type) {
	case float64:
		price = v
	case int:
		price = float64(v)
	case string:
		// Try parsing string to float
		var parseErr error
		if price, parseErr = parseFloatString(v); parseErr != nil {
			return nil, fmt.Errorf("failed to parse price: %w", parseErr)
		}
	default:
		return nil, fmt.Errorf("unexpected price type: %T", priceInterface)
	}

	log.Debug().
		Str("symbol", symbol).
		Float64("price", price).
		Str("currency", vsCurrency).
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
	log.Info().
		Str("symbol", symbol).
		Int("days", days).
		Msg("Fetching market chart from CoinGecko MCP")

	// Call get_market_chart tool via MCP
	result, err := c.client.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_market_chart",
		Arguments: map[string]interface{}{
			"id":          symbol,
			"vs_currency": "usd",
			"days":        days,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from MCP result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from CoinGecko")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type from CoinGecko")
	}

	// Parse JSON result
	// CoinGecko returns {prices: [[timestamp, price], ...], market_caps: [[timestamp, cap], ...], total_volumes: [[timestamp, volume], ...]}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return nil, fmt.Errorf("failed to parse CoinGecko response: %w", err)
	}

	chart := &MarketChart{
		Prices:       make([]PricePoint, 0),
		MarketCaps:   make([]PricePoint, 0),
		TotalVolumes: make([]PricePoint, 0),
	}

	// Parse prices array
	if pricesRaw, ok := resultMap["prices"].([]interface{}); ok {
		for _, p := range pricesRaw {
			if point, ok := p.([]interface{}); ok && len(point) >= 2 {
				timestamp := parseTimestamp(point[0])
				value := parseFloat(point[1])
				chart.Prices = append(chart.Prices, PricePoint{
					Timestamp: timestamp,
					Value:     value,
				})
			}
		}
	}

	// Parse market_caps array
	if marketCapsRaw, ok := resultMap["market_caps"].([]interface{}); ok {
		for _, m := range marketCapsRaw {
			if point, ok := m.([]interface{}); ok && len(point) >= 2 {
				timestamp := parseTimestamp(point[0])
				value := parseFloat(point[1])
				chart.MarketCaps = append(chart.MarketCaps, PricePoint{
					Timestamp: timestamp,
					Value:     value,
				})
			}
		}
	}

	// Parse total_volumes array
	if volumesRaw, ok := resultMap["total_volumes"].([]interface{}); ok {
		for _, v := range volumesRaw {
			if point, ok := v.([]interface{}); ok && len(point) >= 2 {
				timestamp := parseTimestamp(point[0])
				value := parseFloat(point[1])
				chart.TotalVolumes = append(chart.TotalVolumes, PricePoint{
					Timestamp: timestamp,
					Value:     value,
				})
			}
		}
	}

	log.Debug().
		Str("symbol", symbol).
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
	log.Info().
		Str("coin_id", coinID).
		Msg("Fetching coin info from CoinGecko MCP")

	// Call get_coin_info tool via MCP
	result, err := c.client.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_coin_info",
		Arguments: map[string]interface{}{
			"id": coinID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from MCP result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result from CoinGecko")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type from CoinGecko")
	}

	// Parse JSON result
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
		return nil, fmt.Errorf("failed to parse CoinGecko response: %w", err)
	}

	coinInfo := &CoinInfo{
		ID:          coinID,
		Symbol:      getString(resultMap, "symbol"),
		Name:        getString(resultMap, "name"),
		Description: getString(resultMap, "description"),
		Links:       make(map[string]string),
		MarketData:  make(map[string]interface{}),
	}

	// Extract links if present
	if linksRaw, ok := resultMap["links"].(map[string]interface{}); ok {
		for k, v := range linksRaw {
			if str, ok := v.(string); ok {
				coinInfo.Links[k] = str
			}
		}
	}

	// Extract market data if present
	if marketDataRaw, ok := resultMap["market_data"].(map[string]interface{}); ok {
		coinInfo.MarketData = marketDataRaw
	}

	log.Debug().
		Str("coin_id", coinID).
		Str("name", coinInfo.Name).
		Msg("Coin info fetched successfully")

	return coinInfo, nil
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

// Health checks if the CoinGecko MCP connection is healthy
func (c *CoinGeckoClient) Health(ctx context.Context) error {
	log.Debug().Str("url", c.url).Msg("Checking CoinGecko MCP health")

	if c.url == "" {
		return fmt.Errorf("CoinGecko MCP URL not configured")
	}

	if c.client == nil {
		return fmt.Errorf("CoinGecko MCP client not initialized")
	}

	// Try a simple tool call to verify connection
	// Use a lightweight tool like get_price for health check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.GetPrice(ctx, "bitcoin", "usd")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	log.Debug().Msg("CoinGecko MCP health check passed")
	return nil
}

// Close closes the MCP client connection
func (c *CoinGeckoClient) Close() error {
	if c.client != nil {
		// Note: MCP SDK may not have explicit Close method
		// This is here for future cleanup if needed
		log.Info().Msg("Closing CoinGecko MCP client")
	}
	return nil
}

// Helper functions

// parseTimestamp converts various timestamp formats to time.Time
func parseTimestamp(v interface{}) time.Time {
	switch t := v.(type) {
	case float64:
		// Unix timestamp in milliseconds
		return time.Unix(0, int64(t)*int64(time.Millisecond))
	case int64:
		return time.Unix(0, t*int64(time.Millisecond))
	case int:
		return time.Unix(0, int64(t)*int64(time.Millisecond))
	default:
		return time.Now()
	}
}

// parseFloat converts various numeric types to float64
func parseFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		// Try parsing string
		if f, err := parseFloatString(n); err == nil {
			return f
		}
	}
	return 0.0
}

// parseFloatString parses a string to float64
func parseFloatString(s string) (float64, error) {
	var f float64
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return 0, fmt.Errorf("failed to parse float from string: %w", err)
	}
	return f, nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}
