package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPRequest_JSONMarshaling tests MCP request serialization
func TestMCPRequest_JSONMarshaling(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
	}
	req.Params.Name = toolPlaceMarketOrder
	req.Params.Arguments = map[string]interface{}{
		"symbol":   "BTCUSDT",
		"side":     "buy",
		"quantity": 0.1,
	}

	// Marshal to JSON
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// Unmarshal back
	var decoded MCPRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, 1, decoded.ID)
	assert.Equal(t, "tools/call", decoded.Method)
	assert.Equal(t, toolPlaceMarketOrder, decoded.Params.Name)
	assert.Equal(t, "BTCUSDT", decoded.Params.Arguments["symbol"])
	assert.Equal(t, "buy", decoded.Params.Arguments["side"])
	assert.Equal(t, 0.1, decoded.Params.Arguments["quantity"])
}

// TestMCPResponse_JSONMarshaling tests MCP response serialization
func TestMCPResponse_JSONMarshaling(t *testing.T) {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]interface{}{
			"order_id": "abc123",
			"status":   "FILLED",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// Unmarshal back
	var decoded MCPResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, 1, decoded.ID)
	assert.Nil(t, decoded.Error)
	assert.NotNil(t, decoded.Result)

	result, ok := decoded.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "abc123", result["order_id"])
	assert.Equal(t, "FILLED", result["status"])
}

// TestMCPError_JSONMarshaling tests MCP error serialization
func TestMCPError_JSONMarshaling(t *testing.T) {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &MCPError{
			Code:    -32601,
			Message: "Method not found",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// Unmarshal back
	var decoded MCPResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, 1, decoded.ID)
	assert.Nil(t, decoded.Result)
	assert.NotNil(t, decoded.Error)
	assert.Equal(t, -32601, decoded.Error.Code)
	assert.Equal(t, "Method not found", decoded.Error.Message)
}

// TestListTools tests the listTools function
func TestListTools(t *testing.T) {
	server := &MCPServer{
		service: nil, // Not needed for listTools
	}

	result := server.listTools()
	require.NotNil(t, result)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := resultMap["tools"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 7)

	// Verify all expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		name, ok := tool["name"].(string)
		require.True(t, ok)
		toolNames[name] = true

		// Verify tool has required fields
		assert.NotEmpty(t, tool["description"])
		assert.NotNil(t, tool["inputSchema"])
	}

	expectedTools := []string{
		toolPlaceMarketOrder,
		toolPlaceLimitOrder,
		toolCancelOrder,
		toolGetOrderStatus,
		toolStartSession,
		toolStopSession,
		toolGetSessionStats,
	}

	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Expected tool %s to be in list", expected)
	}
}

// TestHandleRequest_UnknownMethod tests handling of unknown methods
func TestHandleRequest_UnknownMethod(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
	}

	resp := server.handleRequest(req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Result)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

// TestHandleRequest_ListTools tests tools/list method
func TestHandleRequest_ListTools(t *testing.T) {
	server := &MCPServer{
		service: nil,
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp := server.handleRequest(req)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	resultMap, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := resultMap["tools"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 7)
}

// TestMCPRequestStructure tests the MCP request structure
func TestMCPRequestStructure(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      123,
		Method:  "tools/call",
	}
	req.Params.Name = "test_tool"
	req.Params.Arguments = map[string]interface{}{
		"param1": "value1",
		"param2": 42,
		"param3": true,
	}

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, 123, req.ID)
	assert.Equal(t, "tools/call", req.Method)
	assert.Equal(t, "test_tool", req.Params.Name)
	assert.Equal(t, "value1", req.Params.Arguments["param1"])
	assert.Equal(t, 42, req.Params.Arguments["param2"])
	assert.Equal(t, true, req.Params.Arguments["param3"])
}

// TestMCPResponseStructure tests the MCP response structure
func TestMCPResponseStructure(t *testing.T) {
	// Success response
	successResp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "success",
	}

	assert.Equal(t, "2.0", successResp.JSONRPC)
	assert.Equal(t, 1, successResp.ID)
	assert.Equal(t, "success", successResp.Result)
	assert.Nil(t, successResp.Error)

	// Error response
	errorResp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      2,
		Error: &MCPError{
			Code:    -32700,
			Message: "Parse error",
		},
	}

	assert.Equal(t, "2.0", errorResp.JSONRPC)
	assert.Equal(t, 2, errorResp.ID)
	assert.Nil(t, errorResp.Result)
	assert.NotNil(t, errorResp.Error)
	assert.Equal(t, -32700, errorResp.Error.Code)
	assert.Equal(t, "Parse error", errorResp.Error.Message)
}

// TestListTools_ToolSchemas tests that all tools have valid schemas
func TestListTools_ToolSchemas(t *testing.T) {
	server := &MCPServer{}
	result := server.listTools()

	resultMap := result.(map[string]interface{})
	tools := resultMap["tools"].([]map[string]interface{})

	for _, tool := range tools {
		name := tool["name"].(string)
		t.Run(name, func(t *testing.T) {
			// Verify tool has description
			description, ok := tool["description"].(string)
			assert.True(t, ok, "Tool %s should have description", name)
			assert.NotEmpty(t, description, "Tool %s description should not be empty", name)

			// Verify tool has inputSchema
			schema, ok := tool["inputSchema"].(map[string]interface{})
			assert.True(t, ok, "Tool %s should have inputSchema", name)

			// Verify schema has type
			schemaType, ok := schema["type"].(string)
			assert.True(t, ok, "Tool %s schema should have type", name)
			assert.Equal(t, "object", schemaType, "Tool %s schema type should be object", name)

			// Verify schema has properties
			properties, ok := schema["properties"].(map[string]interface{})
			assert.True(t, ok, "Tool %s schema should have properties", name)

			// Verify schema has required array
			required, ok := schema["required"].([]string)
			assert.True(t, ok, "Tool %s schema should have required array", name)

			// For tools with required fields, verify they exist in properties
			for _, reqField := range required {
				_, exists := properties[reqField]
				assert.True(t, exists, "Tool %s required field %s should exist in properties", name, reqField)
			}
		})
	}
}

// TestToolCount tests that we have the expected number of tools
func TestToolCount(t *testing.T) {
	server := &MCPServer{}
	result := server.listTools()

	resultMap := result.(map[string]interface{})
	tools := resultMap["tools"].([]map[string]interface{})

	// We expect exactly 7 tools
	assert.Equal(t, 7, len(tools), "Should have exactly 7 tools defined")
}

// TestMCPErrorCodes tests standard MCP error codes
func TestMCPErrorCodes(t *testing.T) {
	tests := []struct {
		name         string
		code         int
		expectedType string
	}{
		{"Parse error", -32700, "Parse error"},
		{"Invalid request", -32600, "Invalid request"},
		{"Method not found", -32601, "Method not found"},
		{"Invalid params", -32602, "Invalid params"},
		{"Internal error", -32603, "Internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &MCPError{
				Code:    tt.code,
				Message: tt.expectedType,
			}

			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.expectedType, err.Message)
		})
	}
}
