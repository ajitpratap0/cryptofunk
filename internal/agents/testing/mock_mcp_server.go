// Package testing provides utilities for testing trading agents
package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MockMCPServer simulates an MCP server for testing agents
type MockMCPServer struct {
	mu sync.RWMutex

	// Server configuration
	name    string
	version string

	// Tools registry
	tools map[string]*mcp.Tool

	// Tool call handlers
	handlers map[string]ToolHandler

	// Call history for verification
	calls []ToolCall
}

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx context.Context, arguments map[string]interface{}) (interface{}, error)

// ToolCall represents a recorded tool call for testing
type ToolCall struct {
	ToolName  string
	Arguments map[string]interface{}
	Result    interface{}
	Error     error
}

// NewMockMCPServer creates a new mock MCP server
func NewMockMCPServer(name, version string) *MockMCPServer {
	return &MockMCPServer{
		name:     name,
		version:  version,
		tools:    make(map[string]*mcp.Tool),
		handlers: make(map[string]ToolHandler),
		calls:    make([]ToolCall, 0),
	}
}

// RegisterTool registers a tool with its handler
func (m *MockMCPServer) RegisterTool(tool *mcp.Tool, handler ToolHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tools[tool.Name] = tool
	m.handlers[tool.Name] = handler
}

// CallTool simulates calling a tool on the server
func (m *MockMCPServer) CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if tool exists
	_, ok := m.tools[params.Name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found", params.Name)
	}

	// Get handler
	handler, ok := m.handlers[params.Name]
	if !ok {
		return nil, fmt.Errorf("no handler for tool %s", params.Name)
	}

	// Type assert arguments to map[string]interface{}
	args, ok := params.Arguments.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}, got %T", params.Arguments)
	}

	// Call handler
	result, err := handler(ctx, args)

	// Record call
	m.calls = append(m.calls, ToolCall{
		ToolName:  params.Name,
		Arguments: args,
		Result:    result,
		Error:     err,
	})

	if err != nil {
		return nil, err
	}

	// Convert result to MCP response format
	contentJSON, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", jsonErr)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(contentJSON),
			},
		},
	}, nil
}

// ListTools returns all registered tools
func (m *MockMCPServer) ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]*mcp.Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}

	return &mcp.ListToolsResult{
		Tools: tools,
	}, nil
}

// GetCalls returns all recorded tool calls
func (m *MockMCPServer) GetCalls() []ToolCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return copy to prevent modification
	calls := make([]ToolCall, len(m.calls))
	copy(calls, m.calls)
	return calls
}

// GetCallCount returns the number of times a tool was called
func (m *MockMCPServer) GetCallCount(toolName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, call := range m.calls {
		if call.ToolName == toolName {
			count++
		}
	}
	return count
}

// GetLastCall returns the last call for a specific tool
func (m *MockMCPServer) GetLastCall(toolName string) *ToolCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := len(m.calls) - 1; i >= 0; i-- {
		if m.calls[i].ToolName == toolName {
			call := m.calls[i]
			return &call
		}
	}
	return nil
}

// Reset clears all recorded calls
func (m *MockMCPServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = make([]ToolCall, 0)
}

// GetName returns the server name
func (m *MockMCPServer) GetName() string {
	return m.name
}

// GetVersion returns the server version
func (m *MockMCPServer) GetVersion() string {
	return m.version
}
