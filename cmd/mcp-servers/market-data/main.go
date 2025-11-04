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
)

const (
	serverName    = "market-data"
	serverVersion = "1.0.0"
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

// MarketDataServer provides market data from Binance
type MarketDataServer struct {
	binanceClient *binance.Client
	logger        zerolog.Logger
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

	// Get API keys from environment
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_API_SECRET")

	// Initialize Binance client (testnet)
	binanceClient := binance.NewClient(apiKey, secretKey)
	binance.UseTestnet = true

	// Create market data service
	marketDataService := &MarketDataServer{
		binanceClient: binanceClient,
		logger:        logger,
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
	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

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
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "get_price",
				"description": "Get current price for a trading symbol",
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
				"name":        "get_ticker_24h",
				"description": "Get 24-hour ticker statistics for a symbol",
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
				"description": "Get order book depth for a symbol",
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
		},
	}
}

// callTool executes the requested tool
func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	s.service.logger.Debug().
		Str("tool", name).
		Interface("args", args).
		Msg("Calling tool")

	ctx := context.Background()

	switch name {
	case "get_price":
		return s.service.handleGetCurrentPrice(ctx, args)
	case "get_ticker_24h":
		return s.service.handleGetTicker24h(ctx, args)
	case "get_order_book":
		return s.service.handleGetOrderbook(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// handleGetCurrentPrice gets current price for a symbol
func (s *MarketDataServer) handleGetCurrentPrice(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	s.logger.Debug().Str("symbol", symbol).Msg("Getting current price")

	prices, err := s.binanceClient.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get price")
		return nil, fmt.Errorf("failed to get price: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data for symbol: %s", symbol)
	}

	result := map[string]interface{}{
		"symbol":    prices[0].Symbol,
		"price":     prices[0].Price,
		"timestamp": time.Now().Unix(),
	}

	s.logger.Info().
		Str("symbol", symbol).
		Str("price", prices[0].Price).
		Msg("Price retrieved")

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
