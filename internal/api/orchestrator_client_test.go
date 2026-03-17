package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetActiveAgentCount_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","active_agents":5,"nats":"connected"}`))
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	count := client.GetActiveAgentCount()
	assert.Equal(t, 5, count)
}

func TestGetActiveAgentCount_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	count := client.GetActiveAgentCount()
	assert.Equal(t, -1, count)
}

func TestGetActiveAgentCount_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	count := client.GetActiveAgentCount()
	assert.Equal(t, -1, count)
}

func TestGetActiveAgentCount_NetworkFailure(t *testing.T) {
	client := NewOrchestratorClient("http://localhost:99999")
	count := client.GetActiveAgentCount()
	assert.Equal(t, -1, count)
}

func TestIsPaused_True(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/status", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"running","paused":true}`))
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	assert.True(t, client.IsPaused())
}

func TestIsPaused_False(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"running","paused":false}`))
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	assert.False(t, client.IsPaused())
}

func TestIsPaused_NetworkFailure(t *testing.T) {
	client := NewOrchestratorClient("http://localhost:99999")
	assert.False(t, client.IsPaused())
}

func TestPause_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/pause", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	assert.NoError(t, client.Pause())
}

func TestPause_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	assert.Error(t, client.Pause())
}

func TestResume_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/resume", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewOrchestratorClient(server.URL)
	assert.NoError(t, client.Resume())
}
