package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/market"
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

const (
	serverName = "market-data"
)

var (
	serverVersion = config.Version
)

// TickerData represents 24h ticker statistics
type TickerData struct {
	Symbol             string `json:"symbol"`
	Price              string `json:"price"`
	PriceChangePercent string `json:"price_change_percent"`
	Volume             string `json:"volume"`
	High24h            string `json:"high_24h"`
	Low24h             string `json:"low_24h"`
	Timestamp          int64  `json:"timestamp"`
}

// MCPRequest represents an MCP tool call request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPServer handles MCP protocol over stdio
type MCPServer struct {
	service *MarketDataServer
}

// MarketDataServer provides market data from multiple sources
type MarketDataServer struct {
	binanceClient   *binance.Client
	coingeckoClient *market.CoinGeckoClient
	logger          zerolog.Logger
	preferCoinGecko bool // Use CoinGecko as primary source
}

func main() {
	// Configure logging to stderr (stdout reserved for MCP protocol)
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	logger := log.With().Str("server", serverName).Logger()
	logger.Info().Msg("Starting Market Data MCP Server")

	// Load configuration (includes Vault secrets if enabled)
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Start metrics server on port 9201
	metricsServer := metrics.NewServer(9201, logger)
	if err := metricsServer.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start metrics server")
	}
	logger.Info().Msg("Metrics server started on :9201")

	// Get API keys from configuration (Vault or env vars)
	var binanceAPIKey, binanceSecretKey string
	if binanceCfg, ok := cfg.Exchanges["binance"]; ok {
		binanceAPIKey = binanceCfg.APIKey
		binanceSecretKey = binanceCfg.SecretKey
	}

	// Allow environment variable override for development
	if envKey := os.Getenv("BINANCE_API_KEY"); envKey != "" {
		binanceAPIKey = envKey
	}
	if envSecret := os.Getenv("BINANCE_API_SECRET"); envSecret != "" {
		binanceSecretKey = envSecret
	}

	coingeckoAPIKey := os.Getenv("COINGECKO_API_KEY") // Optional for free tier

	logger.Info().
		Bool("vault_enabled", config.GetVaultConfigFromEnv().Enabled).
		Msg("Configuration loaded successfully")

	// Initialize Binance client (testnet)
	binanceClient := binance.NewClient(binanceAPIKey, binanceSecretKey)
	binance.UseTestnet = true

	// Initialize CoinGecko client
	coingeckoClient, err := market.NewCoinGeckoClient(coingeckoAPIKey)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize CoinGecko client, using Binance only")
		coingeckoClient = nil
	} else {
		logger.Info().Msg("CoinGecko client initialized successfully")
	}

	// Prefer CoinGecko if available (more comprehensive data)
	preferCoinGecko := coingeckoClient != nil && os.Getenv("PREFER_COINGECKO") != "false"

	// Create market data service
	marketDataService := &MarketDataServer{
		binanceClient:   binanceClient,
		coingeckoClient: coingeckoClient,
		logger:          logger,
		preferCoinGecko: preferCoinGecko,
	}

	// Create MCP server
	mcpServer := &MCPServer{
		service: marketDataService,
	}

	logger.Info().Msg("Market Data MCP Server ready, listening on stdio")

	// Run MCP server
	if err := mcpServer.Run(); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}

// Run starts the MCP server with stdio transport
func (s *MCPServer) Run() error {
	s.service.logger.Info().Msg("MCP server ready, listening on stdio")

	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var request MCPRequest
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" {
				s.service.logger.Info().Msg("Client disconnected")
				return nil
			}
			s.service.logger.Error().Err(err).Msg("Failed to decode request")
			continue
		}

		s.service.logger.Debug().
			Str("method", request.Method).
			Int("id", request.ID).
			Msg("Received MCP request")

		response := s.handleRequest(&request)

		if err := encoder.Encode(response); err != nil {
			s.service.logger.Error().Err(err).Msg("Failed to encode response")
			return err
		}
	}
}

// handleRequest routes MCP requests to appropriate handlers
func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	startTime := time.Now()

	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	defer func() {
		// TODO: Add MCP request metrics when they are defined in internal/metrics
		// status := "success"
		// if response.Error != nil {
		// 	status = "error"
		// }
		// metrics.MCPRequestsTotal.WithLabelValues(serverName, req.Method, status).Inc()
		// metrics.MCPRequestDuration.WithLabelValues(serverName, req.Method).Observe(time.Since(startTime).Seconds())
		_ = startTime // Suppress unused variable warning
	}()

	switch req.Method {
	case "initialize":
		response.Result = s.handleInitialize(req.Params)
		return response

	case "tools/list":
		response.Result = s.listTools()
		return response

	case "tools/call":
		var toolParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &toolParams); err != nil {
			response.Error = &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			}
			return response
		}

		result, err := s.callTool(toolParams.Name, toolParams.Arguments)
		if err != nil {
			response.Error = &MCPError{
				Code:    -32000,
				Message: err.Error(),
			}
		} else {
			response.Result = result
		}
		return response

	default:
		response.Error = &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
		return response
	}
}

// handleInitialize responds to MCP initialize request
func (s *MCPServer) handleInitialize(params json.RawMessage) interface{} {
	s.service.logger.Info().Msg("Handling initialize request")

	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]interface{}{
			"name":    serverName,
			"version": serverVersion,
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}
}

// listTools returns available MCP tools
func (s *MCPServer) listTools() interface{} {
	tools := []map[string]interface{}{
		{
			"name":        "get_price",
			"description": "Get current price for a cryptocurrency (uses CoinGecko or Binance)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Coin ID (e.g., 'bitcoin') for CoinGecko or trading pair (e.g., 'BTCUSDT') for Binance",
					},
					"vs_currency": map[string]interface{}{
						"type":        "string",
						"description": "Currency to compare against (default: 'usd')",
						"default":     "usd",
					},
				},
				"required": []string{"symbol"},
			},
		},
		{
			"name":        "get_ticker_24h",
			"description": "Get 24-hour ticker statistics for a symbol (Binance)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Trading pair symbol (e.g., BTCUSDT)",
					},
				},
				"required": []string{"symbol"},
			},
		},
		{
			"name":        "get_order_book",
			"description": "Get order book depth for a symbol (Binance)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Trading pair symbol (e.g., BTCUSDT)",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Number of price levels (default: 20)",
						"default":     20,
					},
				},
				"required": []string{"symbol"},
			},
		},
	}

	// Add CoinGecko-specific tools if client is available
	if s.service.coingeckoClient != nil {
		tools = append(tools, []map[string]interface{}{
			{
				"name":        "get_market_chart",
				"description": "Get historical market data (prices, market caps, volumes)",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"coin_id": map[string]interface{}{
							"type":        "string",
							"description": "CoinGecko coin ID (e.g., 'bitcoin', 'ethereum')",
						},
						"days": map[string]interface{}{
							"type":        "number",
							"description": "Number of days of historical data (1, 7, 30, max)",
							"default":     7,
						},
					},
					"required": []string{"coin_id"},
				},
			},
			{
				"name":        "get_coin_info",
				"description": "Get detailed information about a cryptocurrency",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"coin_id": map[string]interface{}{
							"type":        "string",
							"description": "CoinGecko coin ID (e.g., 'bitcoin', 'ethereum')",
						},
					},
					"required": []string{"coin_id"},
				},
			},
		}...)
	}

	return map[string]interface{}{
		"tools": tools,
	}
}

// callTool executes the requested tool
func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	startTime := time.Now()

	s.service.logger.Debug().
		Str("tool", name).
		Interface("args", args).
		Msg("Calling tool")

	ctx := context.Background()

	var result interface{}
	var err error

	switch name {
	case "get_price":
		result, err = s.service.handleGetCurrentPrice(ctx, args)
	case "get_ticker_24h":
		result, err = s.service.handleGetTicker24h(ctx, args)
	case "get_order_book":
		result, err = s.service.handleGetOrderbook(ctx, args)
	case "get_market_chart":
		result, err = s.service.handleGetMarketChart(ctx, args)
	case "get_coin_info":
		result, err = s.service.handleGetCoinInfo(ctx, args)
	default:
		err = fmt.Errorf("unknown tool: %s", name)
	}

	// Record metrics
	// TODO: Add MCPToolCallsTotal metric when defined in internal/metrics
	// status := "success"
	// if err != nil {
	// 	status = "error"
	// }
	// metrics.MCPToolCallsTotal.WithLabelValues(serverName, name, status).Inc()
	metrics.MCPToolCallDuration.WithLabelValues(name, serverName).Observe(time.Since(startTime).Seconds())

	return result, err
}

// handleGetCurrentPrice gets current price for a symbol
// Uses CoinGecko by default if available, falls back to Binance
func (s *MarketDataServer) handleGetCurrentPrice(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	vsCurrency := "usd"
	if vs, ok := args["vs_currency"].(string); ok {
		vsCurrency = vs
	}

	s.logger.Debug().
		Str("symbol", symbol).
		Str("vs_currency", vsCurrency).
		Bool("prefer_coingecko", s.preferCoinGecko).
		Msg("Getting current price")

	// Try CoinGecko first if preferred and available
	if s.preferCoinGecko && s.coingeckoClient != nil {
		priceResult, err := s.coingeckoClient.GetPrice(ctx, symbol, vsCurrency)
		if err == nil {
			result := map[string]interface{}{
				"symbol":    priceResult.Symbol,
				"price":     fmt.Sprintf("%.2f", priceResult.Price),
				"currency":  priceResult.Currency,
				"timestamp": time.Now().Unix(),
				"source":    "coingecko",
			}

			s.logger.Info().
				Str("symbol", symbol).
				Float64("price", priceResult.Price).
				Str("source", "coingecko").
				Msg("Price retrieved from CoinGecko")

			return result, nil
		}

		s.logger.Warn().
			Err(err).
			Str("symbol", symbol).
			Msg("CoinGecko price fetch failed, falling back to Binance")
	}

	// Fallback to Binance
	prices, err := s.binanceClient.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get price from Binance")
		return nil, fmt.Errorf("failed to get price: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data for symbol: %s", symbol)
	}

	result := map[string]interface{}{
		"symbol":    prices[0].Symbol,
		"price":     prices[0].Price,
		"timestamp": time.Now().Unix(),
		"source":    "binance",
	}

	s.logger.Info().
		Str("symbol", symbol).
		Str("price", prices[0].Price).
		Str("source", "binance").
		Msg("Price retrieved from Binance")

	return result, nil
}

// handleGetTicker24h gets 24-hour ticker for a symbol
func (s *MarketDataServer) handleGetTicker24h(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	s.logger.Debug().Str("symbol", symbol).Msg("Getting 24h ticker")

	ticker, err := s.binanceClient.NewListPriceChangeStatsService().Symbol(symbol).Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get ticker")
		return nil, fmt.Errorf("failed to get ticker: %w", err)
	}

	if len(ticker) == 0 {
		return nil, fmt.Errorf("no ticker data for symbol: %s", symbol)
	}

	t := ticker[0]
	result := TickerData{
		Symbol:             t.Symbol,
		Price:              t.LastPrice,
		PriceChangePercent: t.PriceChangePercent,
		Volume:             t.Volume,
		High24h:            t.HighPrice,
		Low24h:             t.LowPrice,
		Timestamp:          time.Now().Unix(),
	}

	s.logger.Info().
		Str("symbol", symbol).
		Str("price", t.LastPrice).
		Str("change_percent", t.PriceChangePercent).
		Msg("Ticker retrieved")

	return result, nil
}

// handleGetOrderbook gets order book for a symbol
func (s *MarketDataServer) handleGetOrderbook(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	limit := 20
	if l, ok := args["limit"]; ok {
		if lNum, ok := l.(float64); ok {
			limit = int(lNum)
		} else if lStr, ok := l.(string); ok {
			if parsed, err := strconv.Atoi(lStr); err == nil {
				limit = parsed
			}
		}
	}

	s.logger.Debug().
		Str("symbol", symbol).
		Int("limit", limit).
		Msg("Getting orderbook")

	orderbook, err := s.binanceClient.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get orderbook")
		return nil, fmt.Errorf("failed to get orderbook: %w", err)
	}

	result := map[string]interface{}{
		"symbol":    symbol,
		"bids":      orderbook.Bids,
		"asks":      orderbook.Asks,
		"timestamp": time.Now().Unix(),
	}

	s.logger.Info().
		Str("symbol", symbol).
		Int("bids", len(orderbook.Bids)).
		Int("asks", len(orderbook.Asks)).
		Msg("Orderbook retrieved")

	return result, nil
}

// handleGetMarketChart gets historical market data from CoinGecko
func (s *MarketDataServer) handleGetMarketChart(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.coingeckoClient == nil {
		return nil, fmt.Errorf("CoinGecko client not available")
	}

	coinID, ok := args["coin_id"].(string)
	if !ok {
		return nil, fmt.Errorf("coin_id must be a string")
	}

	days := 7
	if d, ok := args["days"]; ok {
		if dNum, ok := d.(float64); ok {
			days = int(dNum)
		} else if dStr, ok := d.(string); ok {
			if parsed, err := strconv.Atoi(dStr); err == nil {
				days = parsed
			}
		}
	}

	s.logger.Debug().
		Str("coin_id", coinID).
		Int("days", days).
		Msg("Getting market chart from CoinGecko")

	chart, err := s.coingeckoClient.GetMarketChart(ctx, coinID, days)
	if err != nil {
		s.logger.Error().Err(err).Str("coin_id", coinID).Msg("Failed to get market chart")
		return nil, fmt.Errorf("failed to get market chart: %w", err)
	}

	result := map[string]interface{}{
		"coin_id":           coinID,
		"days":              days,
		"price_points":      len(chart.Prices),
		"market_cap_points": len(chart.MarketCaps),
		"volume_points":     len(chart.TotalVolumes),
		"prices":            chart.Prices,
		"market_caps":       chart.MarketCaps,
		"total_volumes":     chart.TotalVolumes,
		"timestamp":         time.Now().Unix(),
	}

	s.logger.Info().
		Str("coin_id", coinID).
		Int("days", days).
		Int("price_points", len(chart.Prices)).
		Msg("Market chart retrieved from CoinGecko")

	return result, nil
}

// handleGetCoinInfo gets detailed coin information from CoinGecko
func (s *MarketDataServer) handleGetCoinInfo(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.coingeckoClient == nil {
		return nil, fmt.Errorf("CoinGecko client not available")
	}

	coinID, ok := args["coin_id"].(string)
	if !ok {
		return nil, fmt.Errorf("coin_id must be a string")
	}

	s.logger.Debug().
		Str("coin_id", coinID).
		Msg("Getting coin info from CoinGecko")

	info, err := s.coingeckoClient.GetCoinInfo(ctx, coinID)
	if err != nil {
		s.logger.Error().Err(err).Str("coin_id", coinID).Msg("Failed to get coin info")
		return nil, fmt.Errorf("failed to get coin info: %w", err)
	}

	result := map[string]interface{}{
		"id":          info.ID,
		"symbol":      info.Symbol,
		"name":        info.Name,
		"description": info.Description,
		"links":       info.Links,
		"market_data": info.MarketData,
		"timestamp":   time.Now().Unix(),
	}

	s.logger.Info().
		Str("coin_id", coinID).
		Str("name", info.Name).
		Str("symbol", info.Symbol).
		Msg("Coin info retrieved from CoinGecko")

	return result, nil
}
