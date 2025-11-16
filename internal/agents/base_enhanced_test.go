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

func TestBaseAgent_GetterMethods(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "test-getter-agent",
		Type:         "technical",
		Version:      "2.5.1",
		StepInterval: 1 * time.Second,
		Enabled:      true,
		Config: map[string]interface{}{
			"symbol":     "BTC/USDT",
			"threshold":  0.75,
			"indicators": []string{"RSI", "MACD", "BB"},
		},
	}

	agent := NewBaseAgent(config, log, 9201)
	require.NotNil(t, agent)

	t.Run("GetName", func(t *testing.T) {
		assert.Equal(t, "test-getter-agent", agent.GetName())
	})

	t.Run("GetType", func(t *testing.T) {
		assert.Equal(t, "technical", agent.GetType())
	})

	t.Run("GetVersion", func(t *testing.T) {
		assert.Equal(t, "2.5.1", agent.GetVersion())
	})

	t.Run("GetConfig", func(t *testing.T) {
		cfg := agent.GetConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "test-getter-agent", cfg.Name)
		assert.Equal(t, "technical", cfg.Type)
		assert.Equal(t, "2.5.1", cfg.Version)
		assert.Equal(t, time.Second, cfg.StepInterval)
		assert.True(t, cfg.Enabled)

		// Verify nested config
		assert.Equal(t, "BTC/USDT", cfg.Config["symbol"])
		assert.Equal(t, 0.75, cfg.Config["threshold"])
		indicators, ok := cfg.Config["indicators"].([]string)
		require.True(t, ok)
		assert.Len(t, indicators, 3)
		assert.Contains(t, indicators, "RSI")
		assert.Contains(t, indicators, "MACD")
		assert.Contains(t, indicators, "BB")
	})
}

func TestBaseAgent_StepExecution(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "step-test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 100 * time.Millisecond,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 9202)
	require.NotNil(t, agent)

	ctx := context.Background()

	t.Run("SingleStep", func(t *testing.T) {
		err := agent.Step(ctx)
		assert.NoError(t, err)
	})

	t.Run("MultipleSteps", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := agent.Step(ctx)
			assert.NoError(t, err, "Step %d should not error", i)
		}
	})

	t.Run("StepWithCanceledContext", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		// Step should still execute without error (doesn't check context)
		err := agent.Step(canceledCtx)
		assert.NoError(t, err)
	})
}

func TestBaseAgent_CallMCPTool_Errors(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "mcp-test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 9203)
	require.NotNil(t, agent)

	ctx := context.Background()

	t.Run("ServerNotFound", func(t *testing.T) {
		result, err := agent.CallMCPTool(ctx, "nonexistent-server", "test_tool", map[string]interface{}{
			"param": "value",
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "MCP server nonexistent-server not found")
	})

	t.Run("EmptyServerName", func(t *testing.T) {
		result, err := agent.CallMCPTool(ctx, "", "test_tool", map[string]interface{}{})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("NilArguments", func(t *testing.T) {
		result, err := agent.CallMCPTool(ctx, "missing-server", "test_tool", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestBaseAgent_ListMCPTools_Errors(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "list-tools-test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 9204)
	require.NotNil(t, agent)

	ctx := context.Background()

	t.Run("ServerNotFound", func(t *testing.T) {
		result, err := agent.ListMCPTools(ctx, "nonexistent-server")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "MCP server nonexistent-server not found")
	})

	t.Run("EmptyServerName", func(t *testing.T) {
		result, err := agent.ListMCPTools(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestBaseAgent_RunLoop(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "run-loop-test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 50 * time.Millisecond, // Fast interval for testing
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 9205)
	require.NotNil(t, agent)

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Initialize agent context
		agent.ctx, agent.cancel = context.WithCancel(context.Background())

		// Run should stop when context is canceled
		err := agent.Run(ctx)

		// Should return context.DeadlineExceeded or context.Canceled
		assert.Error(t, err)
		assert.True(t, err == context.DeadlineExceeded || err == context.Canceled)
	})

	t.Run("InternalContextCancellation", func(t *testing.T) {
		// Create agent with its own cancellable context
		agent.ctx, agent.cancel = context.WithCancel(context.Background())

		// Start Run in goroutine
		done := make(chan error, 1)
		go func() {
			done <- agent.Run(context.Background())
		}()

		// Cancel after a short delay
		time.Sleep(100 * time.Millisecond)
		agent.cancel()

		// Wait for Run to finish
		select {
		case err := <-done:
			assert.Error(t, err)
			assert.Equal(t, context.Canceled, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not stop after context cancellation")
		}
	})
}

func TestBaseAgent_Shutdown(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	t.Run("ShutdownWithoutInitialize", func(t *testing.T) {
		config := &AgentConfig{
			Name:         "shutdown-test-1",
			Type:         "test",
			Version:      "1.0.0",
			StepInterval: 1 * time.Second,
			Enabled:      true,
		}

		agent := NewBaseAgent(config, log, 9206)
		require.NotNil(t, agent)

		ctx := context.Background()

		// Shutdown should work even without Initialize
		err := agent.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithContext", func(t *testing.T) {
		config := &AgentConfig{
			Name:         "shutdown-test-2",
			Type:         "test",
			Version:      "1.0.0",
			StepInterval: 1 * time.Second,
			Enabled:      true,
		}

		agent := NewBaseAgent(config, log, 9207)
		require.NotNil(t, agent)

		// Create agent context
		agent.ctx, agent.cancel = context.WithCancel(context.Background())

		ctx := context.Background()
		err := agent.Shutdown(ctx)
		assert.NoError(t, err)

		// Verify cancel was called
		select {
		case <-agent.ctx.Done():
			// Expected - context was canceled
		default:
			t.Error("Agent context should be canceled after Shutdown")
		}
	})

	t.Run("ShutdownWithTimeout", func(t *testing.T) {
		config := &AgentConfig{
			Name:         "shutdown-test-3",
			Type:         "test",
			Version:      "1.0.0",
			StepInterval: 1 * time.Second,
			Enabled:      true,
		}

		agent := NewBaseAgent(config, log, 9208)
		require.NotNil(t, agent)

		// Create agent context
		agent.ctx, agent.cancel = context.WithCancel(context.Background())

		// Use a timeout context for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := agent.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestBaseAgent_MetricsInitialization(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "metrics-test-agent",
		Type:         "technical",
		Version:      "1.2.3",
		StepInterval: 1 * time.Second,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 9209)
	require.NotNil(t, agent)

	t.Run("AllMetricsInitialized", func(t *testing.T) {
		require.NotNil(t, agent.metrics)
		assert.NotNil(t, agent.metrics.StepsTotal, "StepsTotal should be initialized")
		assert.NotNil(t, agent.metrics.StepDuration, "StepDuration should be initialized")
		assert.NotNil(t, agent.metrics.MCPCallsTotal, "MCPCallsTotal should be initialized")
		assert.NotNil(t, agent.metrics.MCPErrorsTotal, "MCPErrorsTotal should be initialized")
		assert.NotNil(t, agent.metrics.MCPCallDuration, "MCPCallDuration should be initialized")
		assert.NotNil(t, agent.metrics.AgentStatus, "AgentStatus should be initialized")
	})

	t.Run("MetricsServerInitialized", func(t *testing.T) {
		assert.NotNil(t, agent.metricsServer, "Metrics server should be initialized")
	})
}

func TestAgentConfig_Validation(t *testing.T) {
	t.Run("MinimalConfig", func(t *testing.T) {
		config := &AgentConfig{
			Name:    "minimal",
			Type:    "test",
			Version: "1.0.0",
		}

		assert.Equal(t, "minimal", config.Name)
		assert.Equal(t, "test", config.Type)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Nil(t, config.MCPServers)
		assert.Nil(t, config.Config)
		assert.Equal(t, time.Duration(0), config.StepInterval)
		assert.False(t, config.Enabled)
	})

	t.Run("FullConfig", func(t *testing.T) {
		config := &AgentConfig{
			Name:         "full-config",
			Type:         "strategy",
			Version:      "2.0.0",
			StepInterval: 5 * time.Second,
			Enabled:      true,
			MCPServers: []MCPServerConfig{
				{
					Name:    "server1",
					Type:    "internal",
					Command: "bin/server1",
					Args:    []string{"--config", "config.json"},
					Env: map[string]string{
						"API_KEY":   "key123",
						"LOG_LEVEL": "debug",
					},
				},
				{
					Name: "server2",
					Type: "external",
					URL:  "http://localhost:8080/mcp",
				},
			},
			Config: map[string]interface{}{
				"strategy":  "mean_reversion",
				"threshold": 2.0,
				"lookback":  20,
				"pairs": []string{
					"BTC/USDT",
					"ETH/USDT",
					"SOL/USDT",
				},
			},
		}

		assert.Equal(t, "full-config", config.Name)
		assert.Equal(t, "strategy", config.Type)
		assert.Equal(t, "2.0.0", config.Version)
		assert.Equal(t, 5*time.Second, config.StepInterval)
		assert.True(t, config.Enabled)

		// Verify MCP servers
		require.Len(t, config.MCPServers, 2)

		// Server 1
		assert.Equal(t, "server1", config.MCPServers[0].Name)
		assert.Equal(t, "internal", config.MCPServers[0].Type)
		assert.Equal(t, "bin/server1", config.MCPServers[0].Command)
		assert.Len(t, config.MCPServers[0].Args, 2)
		assert.Equal(t, "--config", config.MCPServers[0].Args[0])
		assert.Equal(t, "config.json", config.MCPServers[0].Args[1])
		assert.Len(t, config.MCPServers[0].Env, 2)
		assert.Equal(t, "key123", config.MCPServers[0].Env["API_KEY"])
		assert.Equal(t, "debug", config.MCPServers[0].Env["LOG_LEVEL"])

		// Server 2
		assert.Equal(t, "server2", config.MCPServers[1].Name)
		assert.Equal(t, "external", config.MCPServers[1].Type)
		assert.Equal(t, "http://localhost:8080/mcp", config.MCPServers[1].URL)

		// Verify config map
		assert.Equal(t, "mean_reversion", config.Config["strategy"])
		assert.Equal(t, 2.0, config.Config["threshold"])
		assert.Equal(t, 20, config.Config["lookback"])

		pairs, ok := config.Config["pairs"].([]string)
		require.True(t, ok)
		assert.Len(t, pairs, 3)
		assert.Contains(t, pairs, "BTC/USDT")
		assert.Contains(t, pairs, "ETH/USDT")
		assert.Contains(t, pairs, "SOL/USDT")
	})
}

func TestMCPServerConfig_Types(t *testing.T) {
	t.Run("InternalServerConfig", func(t *testing.T) {
		config := MCPServerConfig{
			Name:    "internal-server",
			Type:    "internal",
			Command: "node",
			Args:    []string{"server.js", "--port", "3000"},
			Env: map[string]string{
				"NODE_ENV": "development",
				"PORT":     "3000",
			},
		}

		assert.Equal(t, "internal-server", config.Name)
		assert.Equal(t, "internal", config.Type)
		assert.Equal(t, "node", config.Command)
		assert.Len(t, config.Args, 3)
		assert.Equal(t, "server.js", config.Args[0])
		assert.Equal(t, "--port", config.Args[1])
		assert.Equal(t, "3000", config.Args[2])
		assert.Len(t, config.Env, 2)
		assert.Equal(t, "development", config.Env["NODE_ENV"])
		assert.Equal(t, "3000", config.Env["PORT"])
	})

	t.Run("ExternalServerConfig", func(t *testing.T) {
		config := MCPServerConfig{
			Name: "external-server",
			Type: "external",
			URL:  "https://api.example.com/mcp/v1",
		}

		assert.Equal(t, "external-server", config.Name)
		assert.Equal(t, "external", config.Type)
		assert.Equal(t, "https://api.example.com/mcp/v1", config.URL)
		assert.Empty(t, config.Command)
		assert.Nil(t, config.Args)
		assert.Nil(t, config.Env)
	})
}

func TestBaseAgent_ConcurrentSteps(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	config := &AgentConfig{
		Name:         "concurrent-test-agent",
		Type:         "test",
		Version:      "1.0.0",
		StepInterval: 10 * time.Millisecond,
		Enabled:      true,
	}

	agent := NewBaseAgent(config, log, 9210)
	require.NotNil(t, agent)

	ctx := context.Background()

	// Run multiple steps concurrently
	const numGoroutines = 50
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				err := agent.Step(ctx)
				assert.NoError(t, err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent steps timed out")
		}
	}
}
