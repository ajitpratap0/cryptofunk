package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

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

		// Marshal result to JSON text content
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", marshalErr)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
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
