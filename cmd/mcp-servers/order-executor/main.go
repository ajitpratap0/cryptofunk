package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/exchange"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging to stderr (stdout is reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Read configuration from environment
	tradingMode := os.Getenv("TRADING_MODE")
	if tradingMode == "" {
		tradingMode = "paper" // Default to paper trading
	}

	binanceAPIKey := os.Getenv("BINANCE_API_KEY")
	binanceSecret := os.Getenv("BINANCE_API_SECRET")
	binanceTestnet := os.Getenv("BINANCE_TESTNET") == "true"

	log.Info().
		Str("mode", tradingMode).
		Bool("testnet", binanceTestnet).
		Msg("Order Executor MCP Server starting...")

	// Initialize database connection
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()

	log.Info().Msg("Database connection established")

	// Create exchange service with configuration
	config := exchange.ServiceConfig{
		Mode:           exchange.TradingMode(tradingMode),
		BinanceAPIKey:  binanceAPIKey,
		BinanceSecret:  binanceSecret,
		BinanceTestnet: binanceTestnet,
	}

	exchangeService, err := exchange.NewService(database, config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create exchange service")
	}

	// Start MCP server with stdio transport
	server := &MCPServer{
		service: exchangeService,
	}

	if err := server.Run(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

// MCPServer handles MCP protocol over stdio
type MCPServer struct {
	service *exchange.Service
}

// Run starts the MCP server
func (s *MCPServer) Run() error {
	log.Info().Msg("MCP server ready, listening on stdio")

	// Read from stdin, process requests, write to stdout
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var request MCPRequest
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" {
				log.Info().Msg("Client disconnected")
				return nil
			}
			log.Error().Err(err).Msg("Failed to decode request")
			continue
		}

		log.Debug().
			Str("method", request.Method).
			Str("tool", request.Params.Name).
			Msg("Received request")

		// Handle request
		response := s.handleRequest(&request)

		// Send response
		if err := encoder.Encode(response); err != nil {
			log.Error().Err(err).Msg("Failed to encode response")
			return err
		}
	}
}

// MCPRequest represents an MCP tool call request
type MCPRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
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

// handleRequest routes the request to the appropriate handler
func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "tools/list":
		resp.Result = s.listTools()
	case "tools/call":
		result, err := s.callTool(req.Params.Name, req.Params.Arguments)
		if err != nil {
			resp.Error = &MCPError{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	return resp
}

// listTools returns the list of available tools
func (s *MCPServer) listTools() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "place_market_order",
				"description": "Place a market order (immediate execution at current market price)",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "Trading pair symbol (e.g., 'BTCUSDT')",
						},
						"side": map[string]interface{}{
							"type":        "string",
							"description": "Order side: 'buy' or 'sell'",
							"enum":        []string{"buy", "sell"},
						},
						"quantity": map[string]interface{}{
							"type":        "number",
							"description": "Order quantity",
						},
					},
					"required": []string{"symbol", "side", "quantity"},
				},
			},
			{
				"name":        "place_limit_order",
				"description": "Place a limit order (executed at specified price or better)",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "Trading pair symbol (e.g., 'BTCUSDT')",
						},
						"side": map[string]interface{}{
							"type":        "string",
							"description": "Order side: 'buy' or 'sell'",
							"enum":        []string{"buy", "sell"},
						},
						"quantity": map[string]interface{}{
							"type":        "number",
							"description": "Order quantity",
						},
						"price": map[string]interface{}{
							"type":        "number",
							"description": "Limit price",
						},
					},
					"required": []string{"symbol", "side", "quantity", "price"},
				},
			},
			{
				"name":        "cancel_order",
				"description": "Cancel an open or pending order",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"order_id": map[string]interface{}{
							"type":        "string",
							"description": "Order ID to cancel",
						},
					},
					"required": []string{"order_id"},
				},
			},
			{
				"name":        "get_order_status",
				"description": "Get current status and details of an order",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"order_id": map[string]interface{}{
							"type":        "string",
							"description": "Order ID to query",
						},
					},
					"required": []string{"order_id"},
				},
			},
			{
				"name":        "start_session",
				"description": "Start a new trading session for paper trading",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "Trading pair symbol (e.g., 'BTCUSDT')",
						},
						"initial_capital": map[string]interface{}{
							"type":        "number",
							"description": "Initial capital for the trading session",
						},
						"config": map[string]interface{}{
							"type":        "object",
							"description": "Optional configuration parameters for the session",
						},
					},
					"required": []string{"symbol", "initial_capital"},
				},
			},
			{
				"name":        "stop_session",
				"description": "Stop the current trading session and retrieve final statistics",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"final_capital": map[string]interface{}{
							"type":        "number",
							"description": "Final capital at session end",
						},
					},
					"required": []string{"final_capital"},
				},
			},
			{
				"name":        "get_session_stats",
				"description": "Get current statistics for the active trading session",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
			},
		},
	}
}

// callTool executes the specified tool
func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "place_market_order":
		return s.service.PlaceMarketOrder(args)
	case "place_limit_order":
		return s.service.PlaceLimitOrder(args)
	case "cancel_order":
		return s.service.CancelOrder(args)
	case "get_order_status":
		return s.service.GetOrderStatus(args)
	case "start_session":
		return s.service.StartSession(args)
	case "stop_session":
		return s.service.StopSession(args)
	case "get_session_stats":
		return s.service.GetSessionStats(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
