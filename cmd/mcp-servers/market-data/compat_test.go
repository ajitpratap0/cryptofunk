//nolint:all // Test compatibility shim for old test structure
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/rs/zerolog"

	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
)

// MCPRequest is the old request type kept for test compatibility.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// MCPResponse is the old response type kept for test compatibility.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError is the old error type kept for test compatibility.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPServer is a test-compat shim.
type MCPServer struct {
	service *MarketDataServer
	srv     *mcpserver.Server
}

func (s *MCPServer) ensureInit() {
	if s.srv != nil {
		return
	}
	logger := zerolog.New(io.Discard)
	s.srv = mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  logger,
	})
	if s.service != nil {
		registerTools(s.srv, s.service)
	}
}

func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	s.ensureInit()
	ctx := context.Background()
	jResp := s.srv.HandleJSONRPC(ctx, req.Method, req.Params, req.ID)
	data, _ := json.Marshal(jResp)
	resp := &MCPResponse{}
	_ = json.Unmarshal(data, resp)
	resp.ID = req.ID
	return resp
}

func (s *MCPServer) handleInitialize(_ json.RawMessage) interface{} {
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

func (s *MCPServer) listTools() interface{} {
	s.ensureInit()
	ctx := context.Background()
	jResp := s.srv.HandleJSONRPC(ctx, "tools/list", nil, 0)
	return jResp.Result
}

func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	ctx := context.Background()
	switch name {
	case "get_price":
		return s.service.handleGetCurrentPrice(ctx, args)
	case "get_ticker_24h":
		return s.service.handleGetTicker24h(ctx, args)
	case "get_order_book":
		return s.service.handleGetOrderbook(ctx, args)
	case "get_market_chart":
		return s.service.handleGetMarketChart(ctx, args)
	case "get_coin_info":
		return s.service.handleGetCoinInfo(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
