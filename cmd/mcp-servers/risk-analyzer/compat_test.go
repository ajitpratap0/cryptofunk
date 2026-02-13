//nolint:goconst // Test compatibility shim
package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/rs/zerolog"

	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
)

// MCPRequest is the old request type kept for test compatibility.
type MCPRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
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

// MCPServer is a test-compat shim that delegates to the shared server.
type MCPServer struct {
	srv *mcpserver.Server
}

func newTestCompatServer() *MCPServer {
	logger := zerolog.New(io.Discard)
	srv := mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  logger,
	})
	registerTools(srv)
	return &MCPServer{srv: srv}
}

func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	if s.srv == nil {
		// Auto-init for tests that use &MCPServer{}
		logger := zerolog.New(io.Discard)
		s.srv = mcpserver.New(mcpserver.Config{
			Name:    serverName,
			Version: serverVersion,
			Logger:  logger,
		})
		registerTools(s.srv)
	}

	ctx := context.Background()
	params, _ := json.Marshal(req.Params)
	jResp := s.srv.HandleJSONRPC(ctx, req.Method, params, req.ID)

	// Marshal and unmarshal to get plain map types (tests expect map[string]interface{})
	data, _ := json.Marshal(jResp)
	resp := &MCPResponse{}
	_ = json.Unmarshal(data, resp)
	resp.ID = req.ID
	return resp
}
