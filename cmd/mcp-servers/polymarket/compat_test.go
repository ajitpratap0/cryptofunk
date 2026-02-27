// compat_test.go provides HTTP transport integration tests for the polymarket
// MCP server. It validates that the registerTools function correctly bridges
// the Polymarket client to the shared MCP server base for Streamable HTTP.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket"
	"github.com/ajitpratap0/cryptofunk/internal/testhelpers"
)

// newHTTPPolyServer creates a test Polymarket MCP server using the shared base
// and the given mock HTTP handler for the Polymarket API.
func newHTTPPolyServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *mcpserver.Server) {
	t.Helper()

	apiServer := httptest.NewServer(handler)
	t.Cleanup(apiServer.Close)

	client, err := polymarket.NewClient("",
		polymarket.WithClobHost(apiServer.URL),
		polymarket.WithGammaHost(apiServer.URL),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	srv := mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  zerolog.Nop(),
	})
	registerTools(srv, client)

	httpSrv, err := srv.NewHTTPServer(0)
	require.NoError(t, err)

	ts := httptest.NewServer(httpSrv.Handler)
	t.Cleanup(ts.Close)

	return ts, srv
}

// mcpHTTPPost posts a JSON-RPC request to the MCP endpoint and returns the response.
func mcpHTTPPost(t *testing.T, ts *httptest.Server, sessionID string, payload interface{}) (*http.Response, string) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/mcp", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	return resp, resp.Header.Get("Mcp-Session-Id")
}

// initHTTPSession initializes an MCP session over HTTP and returns the session ID.
func initHTTPSession(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp, sid := mcpHTTPPost(t, ts, "", map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
		},
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, sid, "expected Mcp-Session-Id in response")

	data := testhelpers.ReadSSEData(t, resp.Body)
	var rpc struct {
		Result map[string]interface{} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(data, &rpc))
	assert.Equal(t, "2024-11-05", rpc.Result["protocolVersion"])
	return sid
}

// TestHTTP_PolymarketToolsRegistered verifies that all 7 polymarket tools appear
// in the tools/list response when using the shared HTTP base server.
func TestHTTP_PolymarketToolsRegistered(t *testing.T) {
	ts, _ := newHTTPPolyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]")) //nolint:errcheck
	})

	sid := initHTTPSession(t, ts)

	resp, _ := mcpHTTPPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list",
		"params": map[string]interface{}{},
	})
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	data := testhelpers.ReadSSEData(t, resp.Body)
	var listRPC struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(data, &listRPC))

	expectedTools := []string{
		"get_markets", "get_market", "get_orderbook",
		"get_price", "place_order", "cancel_order", "get_positions",
	}
	assert.Equal(t, len(expectedTools), len(listRPC.Result.Tools),
		"expected %d tools, got %d", len(expectedTools), len(listRPC.Result.Tools))

	toolNames := make(map[string]bool)
	for _, tool := range listRPC.Result.Tools {
		toolNames[tool.Name] = true
	}
	for _, name := range expectedTools {
		assert.True(t, toolNames[name], "missing tool: %s", name)
	}
}

// TestHTTP_GetMarketsOverHTTP verifies that the get_markets tool works via HTTP transport.
func TestHTTP_GetMarketsOverHTTP(t *testing.T) {
	mockMarkets := `[{"question":"Will BTC hit 100k?","active":true}]`

	ts, _ := newHTTPPolyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockMarkets)) //nolint:errcheck
	})

	sid := initHTTPSession(t, ts)

	resp, _ := mcpHTTPPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]interface{}{
			"name":      "get_markets",
			"arguments": map[string]interface{}{},
		},
	})
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	data := testhelpers.ReadSSEData(t, resp.Body)
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
	require.NoError(t, json.Unmarshal(data, &callRPC))

	if callRPC.Error != nil {
		t.Fatalf("RPC error: %s", callRPC.Error.Message)
	}
	assert.False(t, callRPC.Result.IsError, "expected no error, got: %+v", callRPC.Result.Content)
	require.NotEmpty(t, callRPC.Result.Content)
	assert.Contains(t, callRPC.Result.Content[0].Text, "BTC hit 100k")
}

// TestHTTP_MissingRequiredArg verifies that missing required arguments produce
// a proper ToolError (IsError=true) rather than a panic.
func TestHTTP_MissingRequiredArg(t *testing.T) {
	ts, _ := newHTTPPolyServer(t, func(w http.ResponseWriter, r *http.Request) {})

	sid := initHTTPSession(t, ts)

	// Call get_market without condition_id
	resp, _ := mcpHTTPPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0", "id": 4, "method": "tools/call",
		"params": map[string]interface{}{
			"name":      "get_market",
			"arguments": map[string]interface{}{},
		},
	})
	defer resp.Body.Close()

	data := testhelpers.ReadSSEData(t, resp.Body)
	var callRPC struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(data, &callRPC))
	assert.True(t, callRPC.Result.IsError, "expected IsError=true for missing condition_id")
	require.NotEmpty(t, callRPC.Result.Content)
	assert.Contains(t, callRPC.Result.Content[0].Text, "condition_id is required")
}

// TestHTTP_PolymarketHealthCheck verifies the health endpoint is accessible.
func TestHTTP_PolymarketHealthCheck(t *testing.T) {
	ts, _ := newHTTPPolyServer(t, func(w http.ResponseWriter, r *http.Request) {})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `"status":"ok"`)
}
