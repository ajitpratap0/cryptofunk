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
		Name:         "config-test",
		Type:         "test",
		Version:      "2.0.0",
		StepInterval: 5 * time.Second,
		Enabled:      true,
		MCPServers: []MCPServerConfig{
			{
				Name:    "test-server",
				Type:    "internal",
				Command: "node",
				Args:    []string{"server.js"},
				Env: map[string]string{
					"API_KEY": "test-key",
				},
			},
		},
		Config: map[string]interface{}{
			"threshold": 0.8,
			"symbols":   []string{"BTC/USDT", "ETH/USDT"},
		},
	}

	assert.Equal(t, "config-test", config.Name)
	assert.Equal(t, "test", config.Type)
	assert.Equal(t, "2.0.0", config.Version)
	assert.Equal(t, 5*time.Second, config.StepInterval)
	assert.True(t, config.Enabled)
	assert.NotNil(t, config.Config)
	assert.Equal(t, 0.8, config.Config["threshold"])
	assert.Len(t, config.MCPServers, 1)
	assert.Equal(t, "test-server", config.MCPServers[0].Name)
	assert.Equal(t, "internal", config.MCPServers[0].Type)
	assert.Equal(t, "node", config.MCPServers[0].Command)
	assert.Len(t, config.MCPServers[0].Args, 1)
	assert.Equal(t, "server.js", config.MCPServers[0].Args[0])
	assert.Equal(t, "test-key", config.MCPServers[0].Env["API_KEY"])
}
