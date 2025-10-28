package agents

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBaseAgent(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
		Config: map[string]interface{}{
			"test_param": "test_value",
		},
	}

	agent := NewBaseAgent(config, log, 9101)

	assert.NotNil(t, agent)
	assert.Equal(t, "test-agent", agent.GetName())
	assert.Equal(t, "test", agent.GetType())
	assert.Equal(t, "1.0.0", agent.GetVersion())
	assert.NotNil(t, agent.GetConfig())
	assert.NotNil(t, agent.metrics)
	assert.NotNil(t, agent.metrics.StepsTotal)
	assert.NotNil(t, agent.metrics.StepDuration)
	assert.NotNil(t, agent.metrics.MCPCallsTotal)
	assert.NotNil(t, agent.metrics.MCPErrorsTotal)
	assert.NotNil(t, agent.metrics.AgentStatus)
	assert.NotNil(t, agent.metricsServer)
}

func TestBaseAgentLifecycle(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "lifecycle-test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 100 * time.Millisecond,
		Enabled:      true,
		// No MCP server configured for this test
	}

	agent := NewBaseAgent(config, log, 9102)
	require.NotNil(t, agent)

	// Test that agent can be created and basic methods work
	assert.Equal(t, "lifecycle-test-agent", agent.GetName())
	assert.Equal(t, "test", agent.GetType())
	assert.Equal(t, "1.0.0", agent.GetVersion())

	// Test Step method (should not error even without MCP server)
	ctx := context.Background()
	err := agent.Step(ctx)
	assert.NoError(t, err)
}

func TestAgentConfig(t *testing.T) {
	config := &AgentConfig{
		Name:          "config-test",
		Type:          "test",
		Version:       "2.0.0",
		MCPServerCmd:  "node",
		MCPServerArgs: []string{"server.js"},
		MCPServerEnv: map[string]string{
			"API_KEY": "test-key",
		},
		StepInterval: 5 * time.Second,
		Enabled:      true,
		Config: map[string]interface{}{
			"threshold": 0.8,
			"symbols":   []string{"BTC/USDT", "ETH/USDT"},
		},
	}

	assert.Equal(t, "config-test", config.Name)
	assert.Equal(t, "test", config.Type)
	assert.Equal(t, "2.0.0", config.Version)
	assert.Equal(t, "node", config.MCPServerCmd)
	assert.Len(t, config.MCPServerArgs, 1)
	assert.Equal(t, "server.js", config.MCPServerArgs[0])
	assert.Equal(t, "test-key", config.MCPServerEnv["API_KEY"])
	assert.Equal(t, 5*time.Second, config.StepInterval)
	assert.True(t, config.Enabled)
	assert.NotNil(t, config.Config)
	assert.Equal(t, 0.8, config.Config["threshold"])
}

func TestMCPRequest(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  nil,
	}

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, 1, req.ID)
	assert.Equal(t, "tools/list", req.Method)
	assert.Nil(t, req.Params)
}

func TestMCPResponse(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  []byte(`{"tools": []}`),
		Error:   nil,
	}

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)
}

func TestMCPError(t *testing.T) {
	mcpErr := MCPError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]interface{}{"detail": "Missing required field"},
	}

	assert.Equal(t, -32600, mcpErr.Code)
	assert.Equal(t, "Invalid Request", mcpErr.Message)
	assert.NotNil(t, mcpErr.Data)
}
