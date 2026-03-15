package api

import (
	"context"
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
// the active_agents count. Returns -1 when the orchestrator is unreachable,
// allowing callers to distinguish "zero agents" from "orchestrator unavailable".
func (c *OrchestratorClient) GetActiveAgentCount() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return -1
	}
	resp, err := c.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("url", c.baseURL+"/health").Msg("Failed to reach orchestrator for agent count")
		return -1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("Orchestrator health returned non-200")
		return -1
	}

	var result struct {
		ActiveAgents int `json:"active_agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Debug().Err(err).Msg("Failed to decode orchestrator health response")
		return -1
	}

	return result.ActiveAgents
}

// Pause calls the orchestrator /pause endpoint.
func (c *OrchestratorClient) Pause() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/pause", nil)
	if err != nil {
		return fmt.Errorf("failed to create pause request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/resume", nil)
	if err != nil {
		return fmt.Errorf("failed to create resume request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/status", nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
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
