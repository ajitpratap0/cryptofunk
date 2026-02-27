package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

func newTestServer() *Server {
	logger := zerolog.New(io.Discard)
	srv := New(Config{
		Name:    "test-server",
		Version: "0.1.0",
		Logger:  logger,
	})
	return srv
}

func TestToolRegistration(t *testing.T) {
	srv := newTestServer()

	srv.AddToolRaw(
		NewTool("echo", "Echoes input", map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"msg": map[string]interface{}{"type": "string"}},
		}),
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "hello"}},
			}, nil
		},
	)

	if len(srv.tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(srv.tools))
	}
	if srv.tools[0].Tool.Name != "echo" {
		t.Fatalf("expected tool name 'echo', got %q", srv.tools[0].Tool.Name)
	}
}

func TestHTTPHandler(t *testing.T) {
	srv := newTestServer()

	srv.AddToolRaw(
		NewTool("ping", "Returns pong", map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}),
		WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"reply": "pong"}, nil
		}),
	)

	httpSrv, err := srv.NewHTTPServer(0)
	if err != nil {
		t.Fatal(err)
	}

	// Test health endpoint
	ts := httptest.NewServer(httpSrv.Handler)
	defer ts.Close()

	healthReq, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(healthReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("health check returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %s", body)
	}

	// Test CORS headers
	req, _ := http.NewRequestWithContext(context.Background(), "OPTIONS", ts.URL+"/mcp", nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}

func TestWrapLegacyHandler(t *testing.T) {
	// Test error handling
	handler := WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, fmt.Errorf("something broke")
	})

	result, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test",
			Arguments: json.RawMessage(`{}`),
		},
	})
	if err != nil {
		t.Fatalf("WrapLegacyHandler should not return error, got: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	if result.Content[0].(*mcp.TextContent).Text != "something broke" {
		t.Fatalf("unexpected error text: %s", result.Content[0].(*mcp.TextContent).Text)
	}
}
