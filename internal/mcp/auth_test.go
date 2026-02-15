package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestAuthMiddleware_NoToken_PassThrough(t *testing.T) {
	h := authMiddleware(AuthConfig{}, okHandler())
	req := httptest.NewRequest("GET", "/mcp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	h := authMiddleware(AuthConfig{Token: "secret123"}, okHandler())
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	h := authMiddleware(AuthConfig{Token: "secret123"}, okHandler())
	req := httptest.NewRequest("GET", "/mcp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	h := authMiddleware(AuthConfig{Token: "secret123"}, okHandler())
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_BadFormat(t *testing.T) {
	h := authMiddleware(AuthConfig{Token: "secret123"}, okHandler())
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Basic abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_SkipPaths(t *testing.T) {
	h := authMiddleware(AuthConfig{Token: "secret123", SkipPaths: []string{"/health"}}, okHandler())
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skipped path, got %d", rec.Code)
	}
}
