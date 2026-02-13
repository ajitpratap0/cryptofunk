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

	// Unwrap CallToolResult into the legacy format
	data, _ := json.Marshal(jResp.Result)
	var wrapper struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if json.Unmarshal(data, &wrapper) == nil && len(wrapper.Content) > 0 {
		if wrapper.IsError {
			return nil, fmt.Errorf("%s", wrapper.Content[0].Text)
		}
		var parsed interface{}
		if json.Unmarshal([]byte(wrapper.Content[0].Text), &parsed) == nil {
			return parsed, nil
		}
		return wrapper.Content[0].Text, nil
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

	// For tools/call responses, unwrap MCP CallToolResult format back to
	// the legacy format expected by tests.
	if resp.Error == nil && req.Method == "tools/call" {
		if result, ok := resp.Result.(map[string]interface{}); ok {
			// Check for tool-level errors (IsError flag)
			if isErr, _ := result["isError"].(bool); isErr {
				errMsg := "tool error"
				if text := extractMCPContentText(result); text != "" {
					errMsg = text
				}
				resp.Error = &MCPError{Code: -32603, Message: errMsg}
				resp.Result = nil
				return resp
			}

			// Unwrap successful content: parse the JSON text back into a map/value
			if text := extractMCPContentText(result); text != "" {
				var parsed interface{}
				if err := json.Unmarshal([]byte(text), &parsed); err == nil {
					resp.Result = parsed
				}
			}
		}
	}

	return resp
}

// extractMCPContentText extracts the text from MCP content array format:
// {"content": [{"text": "...", "type": "text"}]}
func extractMCPContentText(result map[string]interface{}) string {
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return ""
	}
	first, ok := content[0].(map[string]interface{})
	if !ok {
		return ""
	}
	text, _ := first["text"].(string)
	return text
}
