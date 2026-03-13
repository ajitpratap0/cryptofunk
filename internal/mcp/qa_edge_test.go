// qa_edge_test.go — QA edge case tests for PR #60 (fix/mcp-streamable-http-v2)
//
// Covers:
//   - MCP_RATE_LIMIT garbage input ("abc,xyz")
//   - MCP_RATE_LIMIT zero values ("0,0")
//   - Empty Authorization header when token is required
//   - Concurrent /mcp initialize requests (10 goroutines)
//   - Rate limiter actually blocks after burst exhausted
//   - Partial rate limit config (only rps, no burst)
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newEdgeTestServer creates a minimal httptest.Server backed by a real MCP stack.
func newEdgeTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	t.Setenv("MCP_ALLOW_NO_AUTH", "true")
	logger := zerolog.New(io.Discard)
	srv := New(Config{Name: "qa-edge", Version: "0.0.1", Logger: logger})

	srv.AddToolRaw(
		NewTool("ping", "Echo tool", map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}),
		WrapLegacyHandler(func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return fmt.Sprintf("pong: %v", args), nil
		}),
	)

	httpSrv, err := srv.NewHTTPServer(0)
	if err != nil {
		t.Fatalf("NewHTTPServer: %v", err)
	}
	ts := httptest.NewServer(httpSrv.Handler)
	t.Cleanup(ts.Close)
	return ts
}

// postMCP sends a JSON-RPC POST to /mcp with optional extra headers.
func postMCP(ts *httptest.Server, payload interface{}, headers map[string]string) (*http.Response, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, ts.URL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return ts.Client().Do(req)
}

func initPayload() map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "qa-edge", "version": "1.0"},
		},
	}
}

// ─── Edge case 1: MCP_RATE_LIMIT = "abc,xyz" ─────────────────────────────────

func TestQA_ParseRateLimit_GarbageInput(t *testing.T) {
	lim := parseRateLimit("abc,xyz")
	if lim == nil {
		t.Fatal("parseRateLimit returned nil for garbage input")
	}
	if lim.Limit() != rate.Limit(100) {
		t.Errorf("rps: want 100, got %v", lim.Limit())
	}
	if lim.Burst() != 200 {
		t.Errorf("burst: want 200, got %v", lim.Burst())
	}
	t.Logf("PASS: 'abc,xyz' → safe default (rps=100, burst=200)")
}

// ─── Edge case 2: MCP_RATE_LIMIT = "0,0" ─────────────────────────────────────

func TestQA_ParseRateLimit_ZeroValues(t *testing.T) {
	lim := parseRateLimit("0,0")
	if lim == nil {
		t.Fatal("parseRateLimit returned nil for '0,0'")
	}
	if lim.Limit() != rate.Limit(100) {
		t.Errorf("rps: want 100, got %v", lim.Limit())
	}
	if lim.Burst() != 200 {
		t.Errorf("burst: want 200, got %v", lim.Burst())
	}
	t.Logf("PASS: '0,0' → safe default (rps=100, burst=200)")
}

func TestQA_ParseRateLimit_NegativeValues(t *testing.T) {
	lim := parseRateLimit("-5,-10")
	if lim.Limit() != rate.Limit(100) || lim.Burst() != 200 {
		t.Errorf("negative input: want rps=100/burst=200, got rps=%v/burst=%v",
			lim.Limit(), lim.Burst())
	}
	t.Logf("PASS: '-5,-10' → safe default (rps=100, burst=200)")
}

func TestQA_ParseRateLimit_OnlyRPS(t *testing.T) {
	lim := parseRateLimit("50")
	if lim.Limit() != rate.Limit(50) {
		t.Errorf("rps: want 50, got %v", lim.Limit())
	}
	if lim.Burst() != 200 { // burst untouched
		t.Errorf("burst: want 200, got %v", lim.Burst())
	}
	t.Logf("PASS: '50' → rps=50, burst=200 (default)")
}

// ─── Edge case 3: Empty / invalid Authorization header ────────────────────────

func TestQA_AuthMiddleware_EmptyHeader_Returns401(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := authMiddleware(AuthConfig{Token: "secret"}, inner)

	cases := []struct {
		name       string
		header     string // "" means omit
		wantStatus int
	}{
		{"no header", "", 401},
		{"bearer empty", "Bearer ", 401},
		{"wrong scheme", "Basic dXNlcjpwYXNz", 401},
		{"wrong token", "Bearer wrong-secret", 401},
		{"valid token", "Bearer secret", 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Errorf("auth=%q → got %d, want %d (body: %s)",
					tc.header, rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
	t.Logf("PASS: all auth header edge cases return expected status codes")
}

// ─── Edge case 4: Concurrent /mcp initialize (10 goroutines) ─────────────────

func TestQA_ConcurrentMCPInitialize(t *testing.T) {
	ts := newEdgeTestServer(t)

	const n = 10
	var (
		wg      sync.WaitGroup
		success atomic.Int32
		fail    atomic.Int32
	)

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			resp, err := postMCP(ts, initPayload(), nil)
			if err != nil {
				t.Logf("goroutine %d: request error: %v", idx, err)
				fail.Add(1)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				success.Add(1)
			} else {
				body, _ := io.ReadAll(resp.Body)
				t.Logf("goroutine %d: status %d body=%s", idx, resp.StatusCode, body)
				fail.Add(1)
			}
		}(i)
	}
	wg.Wait()

	t.Logf("concurrent initialize: %d/%d succeeded", success.Load(), n)
	if success.Load() == 0 {
		t.Error("FAIL: no concurrent initialize requests succeeded")
	}
	if fail.Load() > 0 {
		t.Errorf("FAIL: %d/%d concurrent requests failed", fail.Load(), n)
	} else {
		t.Logf("PASS: all %d concurrent /mcp initialize requests succeeded", n)
	}
}

// ─── Edge case 5: Rate limiter actually enforces the limit ───────────────────

func TestQA_RateLimiter_BlocksAfterBurst(t *testing.T) {
	// Tight limiter: 0.0001 rps, burst=1 — second request should always be 429
	lim := rate.NewLimiter(rate.Limit(0.0001), 1)
	called := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(inner, lim)

	// First: should pass (burst available)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", nil))
	if rr1.Code != http.StatusOK {
		t.Errorf("first request: want 200, got %d", rr1.Code)
	}

	// Second (immediate): burst exhausted → 429
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", nil))
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: want 429, got %d (rate limiter not blocking)", rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429 response")
	}

	var errBody struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(rr2.Body.Bytes())).Decode(&errBody); err != nil || errBody.Error == "" {
		t.Errorf("429 body should be JSON with 'error' field, got: %s", rr2.Body.String())
	}

	t.Logf("PASS: rate limiter blocks with 429+Retry-After+JSON body after burst exhausted")
}

// ─── Edge case 6: Verify mcp.Tool type (type-check for NewTool return) ───────

func TestQA_NewTool_TypeIsCorrect(t *testing.T) {
	tool := NewTool("test-tool", "desc", nil)
	if _, ok := interface{}(tool).(*mcp.Tool); !ok {
		t.Errorf("NewTool should return *mcp.Tool")
	}
	if tool.Name != "test-tool" {
		t.Errorf("tool name: want 'test-tool', got %q", tool.Name)
	}
	t.Logf("PASS: NewTool returns well-formed *mcp.Tool")
}
