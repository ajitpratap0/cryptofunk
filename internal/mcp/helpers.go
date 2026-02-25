package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WrapLegacyHandler wraps a legacy tool handler that takes map[string]interface{} args
// and returns (interface{}, error) into an mcp.ToolHandler.
func WrapLegacyHandler(fn func(ctx context.Context, args map[string]interface{}) (interface{}, error)) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
		}
		if args == nil {
			args = make(map[string]interface{})
		}

		result, err := fn(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil
		}

		// Marshal result to JSON text content.
		// Use a custom approach to handle special float values (Inf, NaN)
		// that standard json.Marshal cannot encode.
		sanitized := sanitizeForJSON(result)
		data, marshalErr := json.Marshal(sanitized)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", marshalErr)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}

// sanitizeForJSON recursively replaces Inf/NaN float values with JSON-safe
// representations so json.Marshal won't fail.
func sanitizeForJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case float64:
		if math.IsInf(val, 1) {
			return "Infinity"
		} else if math.IsInf(val, -1) {
			return "-Infinity"
		} else if math.IsNaN(val) {
			return "NaN"
		}
		return val
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			out[k] = sanitizeForJSON(v2)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, v2 := range val {
			out[i] = sanitizeForJSON(v2)
		}
		return out
	default:
		return v
	}
}

// NewTool creates an mcp.Tool with a raw JSON Schema input schema.
func NewTool(name, description string, inputSchema map[string]interface{}) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
}

// ToolError returns a CallToolResult with IsError=true and the given message.
// Use this to return tool-level errors (as opposed to JSON-RPC errors).
func ToolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// ToolJSON marshals data to JSON and returns it as a text content result.
// Returns a ToolError result if marshalling fails.
func ToolJSON(data interface{}) (*mcp.CallToolResult, error) {
	sanitized := sanitizeForJSON(data)
	b, err := json.Marshal(sanitized)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil
}

// ParseArgs extracts tool call arguments from a CallToolRequest as a
// map[string]interface{}. Returns an empty map if arguments are nil or invalid.
func ParseArgs(req *mcp.CallToolRequest) map[string]interface{} {
	if req.Params == nil || req.Params.Arguments == nil {
		return make(map[string]interface{})
	}
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}
