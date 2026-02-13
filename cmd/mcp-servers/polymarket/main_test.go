package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/polymarket"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *MCPServer {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	client, err := polymarket.NewClient("",
		polymarket.WithClobHost(ts.URL),
		polymarket.WithGammaHost(ts.URL),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return &MCPServer{client: client, logger: zerolog.Nop()}
}

func TestHandleRequest_Initialize(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2024-11-05", result["protocolVersion"])
}

func TestHandleRequest_ToolsList(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	tools, ok := result["tools"].([]map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 7, len(tools))

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool["name"].(string)
	}
	assert.Contains(t, names, "get_markets")
	assert.Contains(t, names, "place_order")
	assert.Contains(t, names, "cancel_order")
}

func TestHandleRequest_UnknownMethod(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{JSONRPC: "2.0", ID: 3, Method: "unknown/method"})
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
}

func TestCallTool_GetMarkets(t *testing.T) {
	markets := []polymarket.Market{{ConditionID: "0x1", Question: "Test?"}}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(markets)
	})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_markets","arguments":{}}`),
	})
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestCallTool_GetMarket(t *testing.T) {
	markets := []polymarket.Market{{ConditionID: "0xabc", Question: "Q?"}}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(markets)
	})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_market","arguments":{"condition_id":"0xabc"}}`),
	})
	assert.Nil(t, resp.Error)
}

func TestCallTool_GetMarket_MissingID(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_market","arguments":{}}`),
	})
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "condition_id is required")
}

func TestCallTool_GetOrderbook(t *testing.T) {
	book := polymarket.OrderBook{AssetID: "tok1", Bids: []polymarket.OrderBookLevel{{Price: "0.5", Size: "10"}}}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(book)
	})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 7, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_orderbook","arguments":{"token_id":"tok1"}}`),
	})
	assert.Nil(t, resp.Error)
}

func TestCallTool_GetOrderbook_MissingID(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_orderbook","arguments":{}}`),
	})
	assert.NotNil(t, resp.Error)
}

func TestCallTool_GetPrice(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(polymarket.PriceResponse{Price: "0.65"})
	})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_price","arguments":{"token_id":"tok1","side":"BUY"}}`),
	})
	assert.Nil(t, resp.Error)
}

func TestCallTool_GetPrice_MissingArgs(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_price","arguments":{}}`),
	})
	assert.NotNil(t, resp.Error)
}

func TestCallTool_CancelOrder_MissingID(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: json.RawMessage(`{"name":"cancel_order","arguments":{}}`),
	})
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "order_id is required")
}

func TestCallTool_UnknownTool(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 12, Method: "tools/call",
		Params: json.RawMessage(`{"name":"nonexistent","arguments":{}}`),
	})
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestCallTool_InvalidParams(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := s.handleRequest(&MCPRequest{
		JSONRPC: "2.0", ID: 13, Method: "tools/call",
		Params: json.RawMessage(`invalid json`),
	})
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestToolResult(t *testing.T) {
	result, err := toolResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	m, ok := result.(map[string]interface{})
	require.True(t, ok)
	content, ok := m["content"].([]map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", content[0]["type"])
	assert.Contains(t, content[0]["text"], "key")
}
