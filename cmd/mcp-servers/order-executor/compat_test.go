//nolint:goconst // Test compatibility shim
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/exchange"
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
	service *exchange.Service
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
	registerTools(s.srv, s.service)
}

func (s *MCPServer) listTools() interface{} {
	s.ensureInit()
	ctx := context.Background()
	jResp := s.srv.HandleJSONRPC(ctx, "tools/list", nil, 0)
	return jResp.Result
}

func (s *MCPServer) callTool(name string, args map[string]interface{}) (interface{}, error) {
	s.ensureInit()
	ctx := context.Background()
	params, _ := json.Marshal(map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	jResp := s.srv.HandleJSONRPC(ctx, "tools/call", params, 0)
	if jResp.Error != nil {
		return nil, fmt.Errorf("%s", jResp.Error.Message)
	}
	return jResp.Result, nil
}

func (s *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	s.ensureInit()

	ctx := context.Background()
	params, _ := json.Marshal(req.Params)
	jResp := s.srv.HandleJSONRPC(ctx, req.Method, params, req.ID)

	data, _ := json.Marshal(jResp)
	resp := &MCPResponse{}
	_ = json.Unmarshal(data, resp)
	resp.ID = req.ID
	return resp
}
