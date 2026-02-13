package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/polymarket"
)

const serverName = "polymarket"

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
	client *polymarket.Client
	logger zerolog.Logger
}

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	logger := log.With().Str("server", serverName).Logger()
	logger.Info().Msg("Starting Polymarket MCP Server")

	privateKey := os.Getenv("POLYMARKET_PRIVATE_KEY")
	funder := os.Getenv("POLYMARKET_FUNDER_ADDRESS")

	opts := []polymarket.ClientOption{
		polymarket.WithLogger(logger),
	}
	if funder != "" {
		opts = append(opts, polymarket.WithFunder(funder))
	}

	client, err := polymarket.NewClient(privateKey, opts...)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Polymarket client")
	}
	defer client.Close()

	// Auto-derive API creds if private key is set
	if privateKey != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		creds, err := client.CreateOrDeriveAPICreds(ctx)
		cancel()
		if err != nil {
			logger.Warn().Err(err).Msg("Could not derive API creds, L2 endpoints unavailable")
		} else {
			client.SetAPICreds(creds)
			logger.Info().Msg("API credentials derived successfully")
		}
	}

	server := &MCPServer{client: client, logger: logger}
	logger.Info().Msg("Polymarket MCP Server ready, listening on stdio")

	if err := server.Run(); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}

// Run starts the MCP server with stdio transport
func (s *MCPServer) Run() error {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var request MCPRequest
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" {
				s.logger.Info().Msg("Client disconnected")
				return nil
			}
			s.logger.Error().Err(err).Msg("Failed to decode request")
			continue
		}

		response := s.handleRequest(&request)
		if err := encoder.Encode(response); err != nil {
			s.logger.Error().Err(err).Msg("Failed to encode response")
			return err
		}
	}
}

func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	response := &MCPResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		response.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]interface{}{"name": serverName, "version": "1.0.0"},
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
		}
	case "tools/list":
		response.Result = s.listTools()
	case "tools/call":
		var toolParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &toolParams); err != nil {
			response.Error = &MCPError{Code: -32602, Message: fmt.Sprintf("Invalid params: %v", err)}
			return response
		}
		result, err := s.callTool(toolParams.Name, toolParams.Arguments)
		if err != nil {
			response.Error = &MCPError{Code: -32000, Message: err.Error()}
		} else {
			response.Result = result
		}
	default:
		response.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}
	return response
}

func (s *MCPServer) listTools() interface{} {
	tools := []map[string]interface{}{
		{
			"name":        "get_markets",
			"description": "Get active prediction markets from Polymarket",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "get_market",
			"description": "Get details for a specific Polymarket market by condition ID",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"condition_id": map[string]interface{}{"type": "string", "description": "Market condition ID"},
				},
				"required": []string{"condition_id"},
			},
		},
		{
			"name":        "get_orderbook",
			"description": "Get the order book for a Polymarket token",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token_id": map[string]interface{}{"type": "string", "description": "Conditional token ID"},
				},
				"required": []string{"token_id"},
			},
		},
		{
			"name":        "get_price",
			"description": "Get best bid/ask price for a Polymarket token",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token_id": map[string]interface{}{"type": "string", "description": "Conditional token ID"},
					"side":     map[string]interface{}{"type": "string", "description": "BUY or SELL", "enum": []string{"BUY", "SELL"}},
				},
				"required": []string{"token_id", "side"},
			},
		},
		{
			"name":        "place_order",
			"description": "Place a limit order on Polymarket",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token_id": map[string]interface{}{"type": "string", "description": "Conditional token ID"},
					"side":     map[string]interface{}{"type": "string", "description": "BUY or SELL"},
					"price":    map[string]interface{}{"type": "number", "description": "Limit price (0-1)"},
					"size":     map[string]interface{}{"type": "number", "description": "Order size in tokens"},
				},
				"required": []string{"token_id", "side", "price", "size"},
			},
		},
		{
			"name":        "cancel_order",
			"description": "Cancel an open order on Polymarket",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"order_id": map[string]interface{}{"type": "string", "description": "Order ID to cancel"},
				},
				"required": []string{"order_id"},
			},
		},
		{
			"name":        "get_positions",
			"description": "Get open orders and trade history (positions) on Polymarket",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
	return map[string]interface{}{"tools": tools}
}

func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	switch name {
	case "get_markets":
		markets, err := s.client.GetMarkets(ctx)
		if err != nil {
			return nil, err
		}
		return toolResult(markets)

	case "get_market":
		conditionID, _ := args["condition_id"].(string)
		if conditionID == "" {
			return nil, fmt.Errorf("condition_id is required")
		}
		market, err := s.client.GetMarket(ctx, conditionID)
		if err != nil {
			return nil, err
		}
		return toolResult(market)

	case "get_orderbook":
		tokenID, _ := args["token_id"].(string)
		if tokenID == "" {
			return nil, fmt.Errorf("token_id is required")
		}
		book, err := s.client.GetOrderBook(ctx, tokenID)
		if err != nil {
			return nil, err
		}
		return toolResult(book)

	case "get_price":
		tokenID, _ := args["token_id"].(string)
		sideStr, _ := args["side"].(string)
		if tokenID == "" || sideStr == "" {
			return nil, fmt.Errorf("token_id and side are required")
		}
		price, err := s.client.GetPrice(ctx, tokenID, polymarket.Side(sideStr))
		if err != nil {
			return nil, err
		}
		return toolResult(map[string]string{"price": price})

	case "place_order":
		tokenID, _ := args["token_id"].(string)
		sideStr, _ := args["side"].(string)
		priceVal, _ := args["price"].(float64)
		sizeVal, _ := args["size"].(float64)
		if tokenID == "" || sideStr == "" {
			return nil, fmt.Errorf("token_id and side are required")
		}
		if priceVal == 0 {
			if ps, ok := args["price"].(string); ok {
				priceVal, _ = strconv.ParseFloat(ps, 64)
			}
		}
		if sizeVal == 0 {
			if ss, ok := args["size"].(string); ok {
				sizeVal, _ = strconv.ParseFloat(ss, 64)
			}
		}
		resp, err := s.client.CreateOrder(ctx, tokenID, polymarket.Side(sideStr), priceVal, sizeVal)
		if err != nil {
			return nil, err
		}
		return toolResult(resp)

	case "cancel_order":
		orderID, _ := args["order_id"].(string)
		if orderID == "" {
			return nil, fmt.Errorf("order_id is required")
		}
		if err := s.client.CancelOrder(ctx, orderID); err != nil {
			return nil, err
		}
		return toolResult(map[string]string{"status": "cancelled", "order_id": orderID})

	case "get_positions":
		orders, err := s.client.GetOpenOrders(ctx)
		if err != nil {
			return nil, fmt.Errorf("get open orders: %w", err)
		}
		trades, err := s.client.GetTrades(ctx)
		if err != nil {
			return nil, fmt.Errorf("get trades: %w", err)
		}
		return toolResult(map[string]interface{}{
			"open_orders": orders,
			"trades":      trades,
		})

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func toolResult(data interface{}) (interface{}, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(b)},
		},
	}, nil
}
