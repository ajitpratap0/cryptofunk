//nolint:all // Test compatibility shim
package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/indicators"
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
	service *indicators.Service
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
