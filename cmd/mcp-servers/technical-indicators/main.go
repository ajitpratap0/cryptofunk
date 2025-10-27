package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajitpratap0/cryptofunk/internal/indicators"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging to stderr (stdout is reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Technical Indicators MCP Server starting...")

	// Create indicator service
	indicatorService := indicators.NewService()

	// Start MCP server with stdio transport
	server := &MCPServer{
		service: indicatorService,
	}

	if err := server.Run(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

// MCPServer handles MCP protocol over stdio
type MCPServer struct {
	service *indicators.Service
}

// Run starts the MCP server
func (s *MCPServer) Run() error {
	log.Info().Msg("MCP server ready, listening on stdio")

	// TODO: Phase 2 - Implement actual MCP SDK integration
	// For now, this is a placeholder structure

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
	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	// Handle different MCP methods
	switch req.Method {
	case "tools/list":
		response.Result = s.listTools()
		return response

	case "tools/call":
		result, err := s.callTool(req.Params.Name, req.Params.Arguments)
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

// listTools returns the list of available tools
func (s *MCPServer) listTools() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "calculate_rsi",
				"description": "Calculate Relative Strength Index (RSI) for trend strength analysis",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prices": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of closing prices",
						},
						"period": map[string]interface{}{
							"type":        "number",
							"description": "RSI period (default: 14)",
							"default":     14,
						},
					},
					"required": []string{"prices"},
				},
			},
			{
				"name":        "calculate_macd",
				"description": "Calculate Moving Average Convergence Divergence (MACD) for trend analysis",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prices": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of closing prices",
						},
						"fast_period": map[string]interface{}{
							"type":        "number",
							"description": "Fast EMA period (default: 12)",
							"default":     12,
						},
						"slow_period": map[string]interface{}{
							"type":        "number",
							"description": "Slow EMA period (default: 26)",
							"default":     26,
						},
						"signal_period": map[string]interface{}{
							"type":        "number",
							"description": "Signal line period (default: 9)",
							"default":     9,
						},
					},
					"required": []string{"prices"},
				},
			},
			{
				"name":        "calculate_bollinger_bands",
				"description": "Calculate Bollinger Bands for volatility analysis",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prices": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of closing prices",
						},
						"period": map[string]interface{}{
							"type":        "number",
							"description": "Period for moving average (default: 20)",
							"default":     20,
						},
						"std_dev": map[string]interface{}{
							"type":        "number",
							"description": "Standard deviations (default: 2)",
							"default":     2,
						},
					},
					"required": []string{"prices"},
				},
			},
			{
				"name":        "calculate_ema",
				"description": "Calculate Exponential Moving Average (EMA)",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prices": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of closing prices",
						},
						"period": map[string]interface{}{
							"type":        "number",
							"description": "EMA period",
						},
					},
					"required": []string{"prices", "period"},
				},
			},
			{
				"name":        "calculate_adx",
				"description": "Calculate Average Directional Index (ADX) for trend strength",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"high": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of high prices",
						},
						"low": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of low prices",
						},
						"close": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "number"},
							"description": "Array of closing prices",
						},
						"period": map[string]interface{}{
							"type":        "number",
							"description": "ADX period (default: 14)",
							"default":     14,
						},
					},
					"required": []string{"high", "low", "close"},
				},
			},
		},
	}
}

// callTool executes the requested tool
func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	log.Debug().
		Str("tool", name).
		Interface("args", args).
		Msg("Calling tool")

	switch name {
	case "calculate_rsi":
		return s.service.CalculateRSI(args)
	case "calculate_macd":
		return s.service.CalculateMACD(args)
	case "calculate_bollinger_bands":
		return s.service.CalculateBollingerBands(args)
	case "calculate_ema":
		return s.service.CalculateEMA(args)
	case "calculate_adx":
		return s.service.CalculateADX(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
