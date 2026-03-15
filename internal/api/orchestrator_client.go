package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// OrchestratorClient is an HTTP client that implements OrchestratorInterface
// by calling the orchestrator's REST endpoints. This is used by the API server
// (which runs as a separate process) to query orchestrator state.
type OrchestratorClient struct {
	baseURL string
	client  *http.Client
}

// NewOrchestratorClient creates a new OrchestratorClient that talks to the
// orchestrator at the given base URL (e.g., "http://localhost:8081").
func NewOrchestratorClient(baseURL string) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetActiveAgentCount calls the orchestrator /health endpoint and returns
// the active_agents count. Returns 0 on any error (graceful degradation).
func (c *OrchestratorClient) GetActiveAgentCount() int {
	resp, err := c.client.Get(c.baseURL + "/health")
	if err != nil {
		log.Debug().Err(err).Str("url", c.baseURL+"/health").Msg("Failed to reach orchestrator for agent count")
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("Orchestrator health returned non-200")
		return 0
	}

	var result struct {
		ActiveAgents int `json:"active_agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Debug().Err(err).Msg("Failed to decode orchestrator health response")
		return 0
	}

	return result.ActiveAgents
}

// Pause calls the orchestrator /pause endpoint.
func (c *OrchestratorClient) Pause() error {
	resp, err := c.client.Post(c.baseURL+"/pause", "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to call orchestrator pause: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("orchestrator pause returned status %d", resp.StatusCode)
	}
	return nil
}

// Resume calls the orchestrator /resume endpoint.
func (c *OrchestratorClient) Resume() error {
	resp, err := c.client.Post(c.baseURL+"/resume", "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to call orchestrator resume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("orchestrator resume returned status %d", resp.StatusCode)
	}
	return nil
}

// IsPaused calls the orchestrator /api/v1/status endpoint and returns the paused state.
// Returns false on any error (graceful degradation).
func (c *OrchestratorClient) IsPaused() bool {
	resp, err := c.client.Get(c.baseURL + "/api/v1/status")
	if err != nil {
		log.Debug().Err(err).Msg("Failed to reach orchestrator for pause state")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return result.Paused
}
