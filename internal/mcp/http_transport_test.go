package mcpserver

import (
	"bytes"
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
	"github.com/stretchr/testify/assert"

	"github.com/ajitpratap0/cryptofunk/internal/testhelpers"
)

// mcpPost sends an MCP JSON-RPC request to the test server and returns the raw
// response body. The caller must close the body.
func mcpPost(t *testing.T, ts *httptest.Server, sessionID string, payload interface{}, extraHeaders ...string) (*http.Response, string) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/mcp", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	for i := 0; i+1 < len(extraHeaders); i += 2 {
		req.Header.Set(extraHeaders[i], extraHeaders[i+1])
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	sid := resp.Header.Get("Mcp-Session-Id")
	return resp, sid
}

// initMCPSession initializes an MCP session and returns the session ID.
func initMCPSession(t *testing.T, ts *httptest.Server, extraHeaders ...string) string {
	t.Helper()

	resp, sid := mcpPost(t, ts, "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
		},
	}, extraHeaders...)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("initialize returned status %d: %s", resp.StatusCode, body)
	}

	data := testhelpers.ReadSSEData(t, resp.Body)
	var rpc struct {
		Result map[string]interface{}    `json:"result"`
		Error  *struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(data, &rpc); err != nil {
		t.Fatalf("failed to parse initialize response: %v\nraw: %s", err, data)
	}
	if rpc.Error != nil {
		t.Fatalf("initialize returned error: %s", rpc.Error.Message)
	}
	if rpc.Result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("unexpected protocolVersion: %v", rpc.Result["protocolVersion"])
	}

	if sid == "" {
		t.Fatal("expected Mcp-Session-Id in initialize response headers")
	}
	return sid
}

// newHTTPTestServer creates a test server with the given tools registered.
func newHTTPTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	t.Setenv("MCP_ALLOW_NO_AUTH", "true")
	logger := zerolog.New(io.Discard)
	srv := New(Config{Name: "test-http", Version: "0.0.1", Logger: logger})

	// Register a simple echo tool
	srv.AddToolRaw(
		NewTool("echo", "Echoes a message", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"msg": map[string]interface{}{"type": "string"},
			},
			"required": []string{"msg"},
		}),
		WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			msg, _ := args["msg"].(string)
			return map[string]interface{}{"echo": msg}, nil
		}),
	)

	// Register an error tool for testing error paths
	srv.AddToolRaw(
		NewTool("fail", "Always returns an error", map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}),
		WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return nil, fmt.Errorf("intentional failure")
		}),
	)

	httpSrv, err := srv.NewHTTPServer(0)
	if err != nil {
		t.Fatalf("NewHTTPServer failed: %v", err)
	}

	ts := httptest.NewServer(httpSrv.Handler)
	t.Cleanup(ts.Close)

	return ts, srv
}

// TestStreamableHTTP_HealthEndpoint verifies the /health endpoint is reachable without auth.
// By default, health returns only {"status":"ok"}. Verbose mode adds server info.
func TestStreamableHTTP_HealthEndpoint(t *testing.T) {
	ts, _ := newHTTPTestServer(t)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

// TestStreamableHTTP_CORSPreflight verifies CORS preflight OPTIONS requests work.
func TestStreamableHTTP_CORSPreflight(t *testing.T) {
	ts, _ := newHTTPTestServer(t)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodOptions, ts.URL+"/mcp", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS Allow-Origin header")
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("missing POST in CORS Allow-Methods: %s", resp.Header.Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Mcp-Session-Id") {
		t.Fatalf("missing Mcp-Session-Id in CORS Allow-Headers: %s", resp.Header.Get("Access-Control-Allow-Headers"))
	}
	if resp.Header.Get("Access-Control-Expose-Headers") == "" {
		t.Fatal("missing Access-Control-Expose-Headers")
	}
}

// TestStreamableHTTP_FullMCPSessionFlow exercises the complete MCP session lifecycle:
// initialize → notifications/initialized → tools/list → tools/call → session cleanup.
func TestStreamableHTTP_FullMCPSessionFlow(t *testing.T) {
	ts, _ := newHTTPTestServer(t)

	// Step 1: Initialize and get session ID
	sid := initMCPSession(t, ts)
	t.Logf("Session ID: %s", sid)

	// Step 2: Send notifications/initialized (no response expected per MCP spec)
	notifResp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	defer notifResp.Body.Close()
	// 200 or 202 are both acceptable for notifications
	if notifResp.StatusCode != http.StatusOK && notifResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(notifResp.Body)
		t.Logf("notifications/initialized status %d body: %s", notifResp.StatusCode, body)
	}

	// Step 3: tools/list
	toolsResp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	defer toolsResp.Body.Close()

	if toolsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(toolsResp.Body)
		t.Fatalf("tools/list returned %d: %s", toolsResp.StatusCode, body)
	}

	listData := testhelpers.ReadSSEData(t, toolsResp.Body)
	var listRPC struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(listData, &listRPC); err != nil {
		t.Fatalf("failed to parse tools/list response: %v\nraw: %s", err, listData)
	}
	if listRPC.Error != nil {
		t.Fatalf("tools/list returned error: %s", listRPC.Error.Message)
	}
	if len(listRPC.Result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(listRPC.Result.Tools))
	}

	// Verify tool names
	toolNames := make(map[string]bool)
	for _, tool := range listRPC.Result.Tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["echo"] || !toolNames["fail"] {
		t.Fatalf("unexpected tool names: %v", toolNames)
	}

	// Step 4: tools/call (successful)
	callResp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "echo",
			"arguments": map[string]interface{}{"msg": "hello world"},
		},
	})
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(callResp.Body)
		t.Fatalf("tools/call returned %d: %s", callResp.StatusCode, body)
	}

	callData := testhelpers.ReadSSEData(t, callResp.Body)
	var callRPC struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(callData, &callRPC); err != nil {
		t.Fatalf("failed to parse tools/call response: %v\nraw: %s", err, callData)
	}
	if callRPC.Error != nil {
		t.Fatalf("tools/call returned RPC error: %s", callRPC.Error.Message)
	}
	if callRPC.Result.IsError {
		t.Fatalf("tools/call returned tool error: %+v", callRPC.Result.Content)
	}
	if len(callRPC.Result.Content) == 0 {
		t.Fatal("expected content in tools/call result")
	}
	var echoResult map[string]interface{}
	if err := json.Unmarshal([]byte(callRPC.Result.Content[0].Text), &echoResult); err != nil {
		t.Fatalf("failed to parse echo result text: %v", err)
	}
	if echoResult["echo"] != "hello world" {
		t.Fatalf("unexpected echo result: %v", echoResult)
	}

	// Step 5: tools/call (tool error - IsError=true, not RPC error)
	failResp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "fail",
			"arguments": map[string]interface{}{},
		},
	})
	defer failResp.Body.Close()

	failData := testhelpers.ReadSSEData(t, failResp.Body)
	var failRPC struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(failData, &failRPC); err != nil {
		t.Fatalf("failed to parse fail tool response: %v\nraw: %s", err, failData)
	}
	if failRPC.Error != nil {
		t.Fatalf("expected tool-level error (IsError), not RPC error: %s", failRPC.Error.Message)
	}
	if !failRPC.Result.IsError {
		t.Fatalf("expected IsError=true for fail tool, got result: %+v", failRPC.Result)
	}
	if len(failRPC.Result.Content) == 0 || !strings.Contains(failRPC.Result.Content[0].Text, "intentional failure") {
		t.Fatalf("unexpected error message: %+v", failRPC.Result.Content)
	}

	// Step 6: Session cleanup via DELETE
	delReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/mcp", nil)
	delReq.Header.Set("Mcp-Session-Id", sid)
	delResp, err := ts.Client().Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /mcp failed: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(delResp.Body)
		t.Fatalf("DELETE /mcp returned %d: %s", delResp.StatusCode, body)
	}
}

// TestStreamableHTTP_UnknownMethod verifies that unknown methods return -32601.
func TestStreamableHTTP_UnknownMethod(t *testing.T) {
	ts, _ := newHTTPTestServer(t)
	sid := initMCPSession(t, ts)

	resp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      99,
		"method":  "no/such/method",
		"params":  map[string]interface{}{},
	})
	defer resp.Body.Close()

	// MCP SDK may return the error either as HTTP 4xx or as SSE data with error field
	if resp.StatusCode == http.StatusOK {
		data := testhelpers.ReadSSEData(t, resp.Body)
		var rpc struct {
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &rpc); err != nil {
			t.Fatalf("failed to parse unknown method response: %v\nraw: %s", err, data)
		}
		if rpc.Error == nil {
			t.Fatal("expected JSON-RPC error for unknown method, got nil")
		}
		t.Logf("Unknown method returned expected error: code=%d msg=%s", rpc.Error.Code, rpc.Error.Message)
	} else if resp.StatusCode >= 400 {
		t.Logf("Unknown method returned HTTP %d (expected)", resp.StatusCode)
	} else {
		t.Fatalf("unexpected status %d for unknown method", resp.StatusCode)
	}
}

// TestStreamableHTTP_UnknownTool verifies that calling an unknown tool returns an error.
func TestStreamableHTTP_UnknownTool(t *testing.T) {
	ts, _ := newHTTPTestServer(t)
	sid := initMCPSession(t, ts)

	resp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "nonexistent_tool",
			"arguments": map[string]interface{}{},
		},
	})
	defer resp.Body.Close()

	data := testhelpers.ReadSSEData(t, resp.Body)
	var rpc struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &rpc); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, data)
	}
	if rpc.Error == nil {
		t.Fatalf("expected error for unknown tool, got successful response")
	}
	t.Logf("Unknown tool error: code=%d msg=%s", rpc.Error.Code, rpc.Error.Message)
}

// TestStreamableHTTP_SessionRequired verifies that requests without a session ID
// fail before session initialization.
func TestStreamableHTTP_SessionRequired(t *testing.T) {
	ts, _ := newHTTPTestServer(t)

	// Try to call tools/list without initializing first (no session ID)
	resp, _ := mcpPost(t, ts, "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	defer resp.Body.Close()

	// Per MCP spec the server must return an error for requests without a session
	data := testhelpers.ReadSSEData(t, resp.Body)
	var rpc struct {
		Error *struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(data, &rpc); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, data)
	}
	if rpc.Error == nil {
		t.Fatalf("expected error for uninitialized session, got successful response")
	}
	t.Logf("Uninitialized session error: %s", rpc.Error.Message)
}

// TestRateLimiting verifies that the rate limiter returns 429 when the limit is exceeded.
//
// NOTE (m4): Do NOT add t.Parallel() to this test. It uses t.Setenv to configure
// MCP_RATE_LIMIT, which mutates the global environment. Although parseRateLimit
// now accepts the env value as a parameter (making it directly unit-testable),
// the full-stack integration test path through wrapMiddleware still reads
// os.Getenv("MCP_RATE_LIMIT") at server construction time. Running in parallel
// with other tests that create servers would create a race on the env variable.
func TestRateLimiting(t *testing.T) {
	// Explicit guard: if this test is ever refactored to run in parallel,
	// replace t.Setenv with: limiter := parseRateLimit("1,1") and inject it directly.
	t.Setenv("MCP_RATE_LIMIT", "1,1")

	ts, _ := newHTTPTestServer(t)

	var got429 bool
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/mcp", nil)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("expected at least one 429 response from rate limiter")
	}
}

// TestContentTypeEnforcement verifies that POST to /mcp with wrong Content-Type returns 415.
func TestContentTypeEnforcement(t *testing.T) {
	ts, _ := newHTTPTestServer(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnsupportedMediaType {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 415, got %d: %s", resp.StatusCode, respBody)
	}
}

// TestStreamableHTTP_MultipleTools verifies that multiple tools can be registered
// and all appear in tools/list.
func TestStreamableHTTP_MultipleTools(t *testing.T) {
	t.Setenv("MCP_ALLOW_NO_AUTH", "true")
	logger := zerolog.New(io.Discard)
	srv := New(Config{Name: "multi-tool-test", Version: "0.0.1", Logger: logger})

	toolNames := []string{"alpha", "beta", "gamma", "delta"}
	for _, name := range toolNames {
		toolName := name // capture for closure
		srv.AddToolRaw(
			NewTool(toolName, "Tool "+toolName, map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: toolName}},
				}, nil
			},
		)
	}

	httpSrv, err := srv.NewHTTPServer(0)
	if err != nil {
		t.Fatalf("NewHTTPServer: %v", err)
	}
	ts := httptest.NewServer(httpSrv.Handler)
	t.Cleanup(ts.Close)

	sid := initMCPSession(t, ts)

	resp, _ := mcpPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0", "id": 2,
		"method": "tools/list", "params": map[string]interface{}{},
	})
	t.Cleanup(func() { resp.Body.Close() })

	data := testhelpers.ReadSSEData(t, resp.Body)
	var listRPC struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &listRPC); err != nil {
		t.Fatalf("parse tools/list: %v\nraw: %s", err, data)
	}
	if len(listRPC.Result.Tools) != len(toolNames) {
		t.Fatalf("expected %d tools, got %d", len(toolNames), len(listRPC.Result.Tools))
	}
}

// TestPanicRecoveryMiddleware verifies that the panicRecoveryMiddleware:
// 1. Returns HTTP 500 with a JSON-RPC -32603 body when the inner handler panics.
// 2. Does not affect non-panicking handlers (pass-through).
// 3. Continues to serve subsequent requests after a panic is recovered.
func TestPanicRecoveryMiddleware(t *testing.T) {
	logger := zerolog.New(io.Discard)
	srv := New(Config{Name: "panic-test", Version: "0.0.1", Logger: logger})

	// Build a panicRecoveryMiddleware-wrapped handler directly without HTTP server.
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	t.Run("panicking handler returns 500 JSON-RPC error", func(t *testing.T) {
		wrapped := srv.panicRecoveryMiddleware(panicHandler)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)

		assert.NotPanics(t, func() { wrapped.ServeHTTP(rec, req) })

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"code":-32603`) {
			t.Errorf("expected JSON-RPC -32603 error body, got: %s", body)
		}
		if !strings.Contains(body, `"id":null`) {
			t.Errorf("expected id:null in error body, got: %s", body)
		}
	})

	t.Run("non-panicking handler passes through unchanged", func(t *testing.T) {
		wrapped := srv.panicRecoveryMiddleware(okHandler)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("server continues serving after panic recovery", func(t *testing.T) {
		wrapped := srv.panicRecoveryMiddleware(panicHandler)

		// First request — panics
		rec1 := httptest.NewRecorder()
		assert.NotPanics(t, func() {
			wrapped.ServeHTTP(rec1, httptest.NewRequest(http.MethodPost, "/mcp", nil))
		})
		if rec1.Code != http.StatusInternalServerError {
			t.Errorf("first request: expected 500, got %d", rec1.Code)
		}

		// Second request — also panics but is also recovered
		rec2 := httptest.NewRecorder()
		assert.NotPanics(t, func() {
			wrapped.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/mcp", nil))
		})
		if rec2.Code != http.StatusInternalServerError {
			t.Errorf("second request: expected 500, got %d", rec2.Code)
		}
	})
}
