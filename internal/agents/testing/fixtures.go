package testing

import (
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestAgentConfig creates a standard test agent configuration
func TestAgentConfig(name, agentType string) *agents.AgentConfig {
	return &agents.AgentConfig{
		Name:         name,
		Type:         agentType,
		Version:      "1.0.0-test",
		MCPServers:   []agents.MCPServerConfig{}, // Empty by default, add as needed
		Config:       make(map[string]interface{}),
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}
}

// AddMockMCPServer adds a mock MCP server configuration to an agent config
func AddMockMCPServer(config *agents.AgentConfig, serverName, command string, args []string) {
	config.MCPServers = append(config.MCPServers, agents.MCPServerConfig{
		Name:    serverName,
		Type:    "internal",
		Command: command,
		Args:    args,
		Env:     make(map[string]string),
	})
}

// MarketDataFixtures provides common market data for testing
type MarketDataFixtures struct{}

// SampleOHLCV returns sample OHLCV data for testing
func (MarketDataFixtures) SampleOHLCV() map[string]interface{} {
	return map[string]interface{}{
		"symbol":    "BTC/USDT",
		"timeframe": "1h",
		"data": [][]interface{}{
			{1704067200000, 42000.0, 42500.0, 41800.0, 42200.0, 1250.5},
			{1704070800000, 42200.0, 42800.0, 42100.0, 42600.0, 1380.2},
			{1704074400000, 42600.0, 43000.0, 42400.0, 42800.0, 1520.8},
		},
	}
}

// SamplePrice returns sample price data
func (MarketDataFixtures) SamplePrice() map[string]interface{} {
	return map[string]interface{}{
		"symbol": "BTC/USDT",
		"price":  42800.0,
		"volume": 1520.8,
		"change": 1.42,
	}
}

// SampleOrderBook returns sample order book data
func (MarketDataFixtures) SampleOrderBook() map[string]interface{} {
	return map[string]interface{}{
		"symbol": "BTC/USDT",
		"bids": [][]interface{}{
			{42795.0, 1.5},
			{42790.0, 2.3},
			{42785.0, 3.1},
		},
		"asks": [][]interface{}{
			{42805.0, 1.2},
			{42810.0, 2.0},
			{42815.0, 2.8},
		},
		"timestamp": time.Now().UnixMilli(),
	}
}

// TechnicalIndicatorFixtures provides technical indicator data for testing
type TechnicalIndicatorFixtures struct{}

// SampleRSI returns sample RSI data
func (TechnicalIndicatorFixtures) SampleRSI() map[string]interface{} {
	return map[string]interface{}{
		"indicator": "rsi",
		"period":    14,
		"values":    []float64{65.2, 68.5, 72.1, 69.8},
		"signal":    "neutral",
	}
}

// SampleMACD returns sample MACD data
func (TechnicalIndicatorFixtures) SampleMACD() map[string]interface{} {
	return map[string]interface{}{
		"indicator": "macd",
		"macd":      []float64{120.5, 135.2, 145.8},
		"signal":    []float64{110.2, 125.8, 140.1},
		"histogram": []float64{10.3, 9.4, 5.7},
		"trend":     "bullish",
	}
}

// SampleBollinger returns sample Bollinger Bands data
func (TechnicalIndicatorFixtures) SampleBollinger() map[string]interface{} {
	return map[string]interface{}{
		"indicator": "bollinger",
		"upper":     []float64{43500.0, 43800.0, 44000.0},
		"middle":    []float64{42500.0, 42700.0, 42900.0},
		"lower":     []float64{41500.0, 41600.0, 41800.0},
		"bandwidth": 4.7,
	}
}

// NewsFixtures provides sentiment/news data for testing
type NewsFixtures struct{}

// SampleNews returns sample news articles
func (NewsFixtures) SampleNews() map[string]interface{} {
	return map[string]interface{}{
		"articles": []map[string]interface{}{
			{
				"title":        "Bitcoin Surges Past $43K",
				"source":       "CryptoNews",
				"sentiment":    "positive",
				"score":        0.75,
				"published_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			},
			{
				"title":        "Market Analysis: Bulls Take Control",
				"source":       "Trading Insights",
				"sentiment":    "positive",
				"score":        0.62,
				"published_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			},
		},
		"overall_sentiment": "positive",
		"confidence":        0.68,
	}
}

// SampleFearGreedIndex returns sample Fear & Greed Index data
func (NewsFixtures) SampleFearGreedIndex() map[string]interface{} {
	return map[string]interface{}{
		"value":          65,
		"classification": "greed",
		"timestamp":      time.Now().Unix(),
	}
}

// RiskFixtures provides risk management data for testing
type RiskFixtures struct{}

// SampleRiskLimits returns sample risk limits
func (RiskFixtures) SampleRiskLimits() map[string]interface{} {
	return map[string]interface{}{
		"max_position_size": 10000.0,
		"max_leverage":      3.0,
		"max_drawdown":      0.15,
		"daily_loss_limit":  1000.0,
	}
}

// SamplePortfolio returns sample portfolio data
func (RiskFixtures) SamplePortfolio() map[string]interface{} {
	return map[string]interface{}{
		"total_value": 50000.0,
		"cash":        20000.0,
		"positions": []map[string]interface{}{
			{
				"symbol":  "BTC/USDT",
				"size":    0.5,
				"value":   21400.0,
				"pnl":     1200.0,
				"pnl_pct": 5.94,
			},
			{
				"symbol":  "ETH/USDT",
				"size":    3.2,
				"value":   8600.0,
				"pnl":     -200.0,
				"pnl_pct": -2.27,
			},
		},
	}
}

// OrderExecutionFixtures provides order execution data for testing
type OrderExecutionFixtures struct{}

// SampleMarketOrder returns sample market order result
func (OrderExecutionFixtures) SampleMarketOrder() map[string]interface{} {
	return map[string]interface{}{
		"order_id": "test-order-12345",
		"symbol":   "BTC/USDT",
		"side":     "buy",
		"type":     "market",
		"quantity": 0.1,
		"status":   "filled",
		"filled":   0.1,
		"price":    42800.0,
		"cost":     4280.0,
	}
}

// SampleLimitOrder returns sample limit order result
func (OrderExecutionFixtures) SampleLimitOrder() map[string]interface{} {
	return map[string]interface{}{
		"order_id": "test-order-67890",
		"symbol":   "BTC/USDT",
		"side":     "sell",
		"type":     "limit",
		"quantity": 0.05,
		"price":    43000.0,
		"status":   "open",
		"filled":   0.0,
	}
}

// CommonTools provides common MCP tool definitions for testing
type CommonTools struct{}

// GetPriceTool returns a standard get_price tool definition
func (CommonTools) GetPriceTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_price",
		Description: "Get current price for a symbol",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol": map[string]interface{}{
					"type":        "string",
					"description": "Trading pair symbol (e.g., BTC/USDT)",
				},
			},
			"required": []string{"symbol"},
		},
	}
}

// GetOHLCVTool returns a standard get_ohlcv tool definition
func (CommonTools) GetOHLCVTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_ohlcv",
		Description: "Get OHLCV candlestick data",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol": map[string]interface{}{
					"type":        "string",
					"description": "Trading pair symbol",
				},
				"timeframe": map[string]interface{}{
					"type":        "string",
					"description": "Timeframe (e.g., 1h, 4h, 1d)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of candles to return",
				},
			},
			"required": []string{"symbol", "timeframe"},
		},
	}
}

// CalculateRSITool returns a standard calculate_rsi tool definition
func (CommonTools) CalculateRSITool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "calculate_rsi",
		Description: "Calculate RSI indicator",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prices": map[string]interface{}{
					"type":        "array",
					"description": "Array of price values",
				},
				"period": map[string]interface{}{
					"type":        "integer",
					"description": "RSI period (default 14)",
				},
			},
			"required": []string{"prices"},
		},
	}
}

// PlaceOrderTool returns a standard place_order tool definition
func (CommonTools) PlaceOrderTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "place_market_order",
		Description: "Place a market order",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol": map[string]interface{}{
					"type":        "string",
					"description": "Trading pair symbol",
				},
				"side": map[string]interface{}{
					"type":        "string",
					"description": "Order side (buy or sell)",
					"enum":        []string{"buy", "sell"},
				},
				"quantity": map[string]interface{}{
					"type":        "number",
					"description": "Order quantity",
				},
			},
			"required": []string{"symbol", "side", "quantity"},
		},
	}
}
