package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryOrchestratorStatus(t *testing.T) {
	expected := OrchestratorStatus{
		State:        "running",
		IsPaused:     false,
		ActiveAgents: 5,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/orchestrator/status", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
	status, err := queryOrchestratorStatus(t.Context(), bot)
	require.NoError(t, err)
	assert.Equal(t, "running", status.State)
	assert.False(t, status.IsPaused)
	assert.Equal(t, 5, status.ActiveAgents)
}

func TestQueryOrchestratorStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
	status, err := queryOrchestratorStatus(t.Context(), bot)
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "500")
}

func TestSendOrchestratorCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"pause", "pause"},
		{"resume", "resume"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/orchestrator/"+tt.command, r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
			err := sendOrchestratorCommand(t.Context(), bot, tt.command)
			assert.NoError(t, err)
		})
	}
}

func TestSendOrchestratorCommand_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
	err := sendOrchestratorCommand(t.Context(), bot, "pause")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestGetSystemConfig(t *testing.T) {
	expected := SystemConfig{
		MaxPositionSize:  5000.0,
		MaxDailyLoss:     500.0,
		MaxOpenPositions: 3,
		StopLossPercent:  2.5,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
	config, err := getSystemConfig(t.Context(), bot)
	require.NoError(t, err)
	assert.Equal(t, 5000.0, config.MaxPositionSize)
	assert.Equal(t, 500.0, config.MaxDailyLoss)
	assert.Equal(t, 3, config.MaxOpenPositions)
	assert.Equal(t, 2.5, config.StopLossPercent)
}

func TestGetSystemConfig_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
	config, err := getSystemConfig(t.Context(), bot)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestGetAgents_FromHTTP(t *testing.T) {
	agents := []AgentInfo{
		{Name: "technical-agent", Status: "running", Symbol: "BTCUSDT", Decisions: 42},
		{Name: "trend-agent", Status: "stopped", Symbol: "ETHUSDT", Decisions: 10},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/agents", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	}))
	defer server.Close()

	bot := &Bot{config: &Config{OrchestratorURL: server.URL}}
	result, err := getAgents(t.Context(), bot)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "technical-agent", result[0].Name)
	assert.Equal(t, "running", result[0].Status)
}
