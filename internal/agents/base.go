// Package agents provides base infrastructure for AI trading agents
package agents

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

// MCPServerConfig holds configuration for a single MCP server
type MCPServerConfig struct {
	Name    string            `json:"name" yaml:"name"`       // Server identifier (e.g., "coingecko", "technical_indicators")
	Type    string            `json:"type" yaml:"type"`       // "internal" (stdio) or "external" (HTTP)
	Command string            `json:"command" yaml:"command"` // Command to start internal server
	Args    []string          `json:"args" yaml:"args"`       // Arguments for internal server command
	Env     map[string]string `json:"env" yaml:"env"`         // Environment variables for internal server
	URL     string            `json:"url" yaml:"url"`         // URL for external HTTP server
}

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	// Identity
	Name    string `json:"name" yaml:"name"`
	Type    string `json:"type" yaml:"type"`
	Version string `json:"version" yaml:"version"`

	// MCP Server Connections (multiple servers supported)
	MCPServers []MCPServerConfig `json:"mcp_servers" yaml:"mcp_servers"`

	// Agent-specific configuration
	Config map[string]interface{} `json:"config" yaml:"config"`

	// Behavior
	StepInterval time.Duration `json:"step_interval" yaml:"step_interval"` // Time between decision cycles
	Enabled      bool          `json:"enabled" yaml:"enabled"`
}

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	// Identity
	name      string
	agentType string
	version   string

	// MCP Client and Sessions (multiple servers supported)
	mcpClient   *mcp.Client                   // Single client instance for creating connections
	mcpSessions map[string]*mcp.ClientSession // One session per MCP server
	config      *AgentConfig

	// State
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Logger
	log zerolog.Logger

	// Metrics
	metrics       *AgentMetrics
	metricsServer *metrics.Server
}

// AgentMetrics holds Prometheus metrics for an agent
type AgentMetrics struct {
	StepsTotal      prometheus.Counter
	StepDuration    prometheus.Histogram
	MCPCallsTotal   prometheus.Counter
	MCPErrorsTotal  prometheus.Counter
	MCPCallDuration prometheus.Histogram
	AgentStatus     prometheus.Gauge
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(config *AgentConfig, log zerolog.Logger, metricsPort int) *BaseAgent {
	// Create metrics
	agentMetrics := &AgentMetrics{
		StepsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("agent_%s_steps_total", config.Name),
			Help: fmt.Sprintf("Total number of decision steps for agent %s", config.Name),
		}),
		StepDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    fmt.Sprintf("agent_%s_step_duration_seconds", config.Name),
			Help:    fmt.Sprintf("Duration of decision steps for agent %s", config.Name),
			Buckets: prometheus.DefBuckets,
		}),
		MCPCallsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("agent_%s_mcp_calls_total", config.Name),
			Help: fmt.Sprintf("Total MCP calls for agent %s", config.Name),
		}),
		MCPErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("agent_%s_mcp_errors_total", config.Name),
			Help: fmt.Sprintf("Total MCP errors for agent %s", config.Name),
		}),
		MCPCallDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    fmt.Sprintf("agent_%s_mcp_call_duration_seconds", config.Name),
			Help:    fmt.Sprintf("Duration of MCP calls for agent %s", config.Name),
			Buckets: prometheus.DefBuckets,
		}),
		AgentStatus: promauto.NewGauge(prometheus.GaugeOpts{
			Name: fmt.Sprintf("agent_%s_status", config.Name),
			Help: fmt.Sprintf("Status of agent %s (1=running, 0=stopped)", config.Name),
		}),
	}

	// Create logger for agent
	agentLog := log.With().Str("agent", config.Name).Str("type", config.Type).Logger()

	// Create metrics server
	metricsServer := metrics.NewServer(metricsPort, agentLog)

	// Create single MCP client instance for creating connections
	mcpClient := mcp.NewClient(
		&mcp.Implementation{
			Name:    config.Name,
			Version: config.Version,
		},
		nil, // ClientOptions - nil if no handlers needed
	)

	return &BaseAgent{
		name:          config.Name,
		agentType:     config.Type,
		version:       config.Version,
		mcpClient:     mcpClient,                           // Single client for creating connections
		mcpSessions:   make(map[string]*mcp.ClientSession), // Initialize sessions map
		config:        config,
		log:           agentLog,
		metrics:       agentMetrics,
		metricsServer: metricsServer,
	}
}

// Initialize sets up the agent and connects to MCP servers
func (a *BaseAgent) Initialize(ctx context.Context) error {
	a.log.Info().Msg("Initializing agent")

	// Create cancellable context
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Connect to all configured MCP servers
	if err := a.connectMCPServers(); err != nil {
		return fmt.Errorf("failed to connect to MCP servers: %w", err)
	}

	// Initialize all MCP connections (send initialize requests)
	if err := a.initializeMCPConnections(); err != nil {
		// Close all sessions on failure
		for name, session := range a.mcpSessions {
			if err := session.Close(); err != nil {
				a.log.Error().Err(err).Str("server", name).Msg("Failed to close session during cleanup")
			}
		}
		return fmt.Errorf("failed to initialize MCP connections: %w", err)
	}

	// Start metrics server
	if a.metricsServer != nil {
		if err := a.metricsServer.Start(); err != nil {
			a.log.Error().Err(err).Msg("Failed to start metrics server")
			// Don't fail agent initialization if metrics server fails
		} else {
			a.log.Info().Msg("Metrics server started successfully")
		}
	}

	a.metrics.AgentStatus.Set(1)
	a.log.Info().Msg("Agent initialized successfully")
	return nil
}

// connectMCPServers connects to all configured MCP servers
func (a *BaseAgent) connectMCPServers() error {
	a.log.Info().Int("server_count", len(a.config.MCPServers)).Msg("Connecting to MCP servers")

	for _, serverConfig := range a.config.MCPServers {
		a.log.Info().
			Str("name", serverConfig.Name).
			Str("type", serverConfig.Type).
			Msg("Connecting to MCP server")

		var session *mcp.ClientSession
		var err error

		// Create appropriate session based on server type
		switch serverConfig.Type {
		case "internal":
			// Internal server: spawn process with stdio transport
			session, err = a.createStdioClient(a.ctx, serverConfig)
			if err != nil {
				return fmt.Errorf("failed to create stdio session for %s: %w", serverConfig.Name, err)
			}

		case "external":
			// External server: HTTP streaming transport
			session, err = a.createHTTPClient(a.ctx, serverConfig)
			if err != nil {
				return fmt.Errorf("failed to create HTTP session for %s: %w", serverConfig.Name, err)
			}

		default:
			return fmt.Errorf("unknown server type %s for %s", serverConfig.Type, serverConfig.Name)
		}

		// Store session in map
		a.mcpSessions[serverConfig.Name] = session

		a.log.Info().Str("name", serverConfig.Name).Msg("MCP server connected")
	}

	return nil
}

// createStdioClient creates an MCP session with stdio transport for internal servers
func (a *BaseAgent) createStdioClient(ctx context.Context, config MCPServerConfig) (*mcp.ClientSession, error) {
	// Create command transport (spawns process with exec.Command)
	cmd := exec.Command(config.Command, config.Args...)
	// Convert env map to KEY=value slice format
	for key, val := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
	}
	transport := &mcp.CommandTransport{Command: cmd}

	// Create session using the BaseAgent's client instance
	session, err := a.mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return session, nil
}

// createHTTPClient creates an MCP session with HTTP streaming transport for external servers
func (a *BaseAgent) createHTTPClient(ctx context.Context, config MCPServerConfig) (*mcp.ClientSession, error) {
	// Create SSE client transport for HTTP
	transport := &mcp.SSEClientTransport{Endpoint: config.URL}

	// Create session using the BaseAgent's client instance
	session, err := a.mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return session, nil
}

// initializeMCPConnections verifies MCP connections are initialized
// Note: With SDK v1.0.0, Connect() handles initialization automatically
func (a *BaseAgent) initializeMCPConnections() error {
	a.log.Info().Msg("Verifying MCP connections")

	for name, session := range a.mcpSessions {
		// Get initialization result from the session
		initResult := session.InitializeResult()

		a.log.Debug().
			Str("server", name).
			Str("server_name", initResult.ServerInfo.Name).
			Str("server_version", initResult.ServerInfo.Version).
			Msg("MCP server connection verified")
	}

	return nil
}

// Run starts the agent's main loop
func (a *BaseAgent) Run(ctx context.Context) error {
	a.log.Info().Msg("Starting agent run loop")

	ticker := time.NewTicker(a.config.StepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.log.Info().Msg("Agent run loop stopped by context")
			return ctx.Err()
		case <-a.ctx.Done():
			a.log.Info().Msg("Agent run loop stopped by internal context")
			return a.ctx.Err()
		case <-ticker.C:
			if err := a.Step(ctx); err != nil {
				a.log.Error().Err(err).Msg("Error in agent step")
				// Continue running despite errors
			}
		}
	}
}

// Step performs a single decision cycle
func (a *BaseAgent) Step(ctx context.Context) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		a.metrics.StepDuration.Observe(duration.Seconds())
		a.metrics.StepsTotal.Inc()
	}()

	a.log.Debug().Msg("Executing agent step")

	// Subclasses will override this method with actual decision logic
	// For now, this is a placeholder that agents can extend

	return nil
}

// Shutdown gracefully stops the agent
func (a *BaseAgent) Shutdown(ctx context.Context) error {
	a.log.Info().Msg("Shutting down agent")

	// Cancel internal context
	if a.cancel != nil {
		a.cancel()
	}

	// Close all MCP sessions
	for name, session := range a.mcpSessions {
		if err := session.Close(); err != nil {
			a.log.Error().Err(err).Str("server", name).Msg("Error closing MCP session")
		} else {
			a.log.Debug().Str("server", name).Msg("MCP session closed successfully")
		}
	}

	// Shutdown metrics server
	if a.metricsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.metricsServer.Shutdown(shutdownCtx); err != nil {
			a.log.Error().Err(err).Msg("Error shutting down metrics server")
		} else {
			a.log.Info().Msg("Metrics server shutdown successfully")
		}
	}

	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.log.Info().Msg("Agent shutdown complete")
	case <-ctx.Done():
		a.log.Warn().Msg("Agent shutdown timeout")
		return ctx.Err()
	}

	a.metrics.AgentStatus.Set(0)
	return nil
}

// CallMCPTool calls a tool on a specific MCP server
func (a *BaseAgent) CallMCPTool(ctx context.Context, serverName string, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		a.metrics.MCPCallDuration.Observe(duration.Seconds())
		a.metrics.MCPCallsTotal.Inc()
	}()

	// Get session for the specified server
	session, ok := a.mcpSessions[serverName]
	if !ok {
		a.metrics.MCPErrorsTotal.Inc()
		return nil, fmt.Errorf("MCP server %s not found", serverName)
	}

	// Call tool on session
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		a.metrics.MCPErrorsTotal.Inc()
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	return result, nil
}

// ListMCPTools lists available tools from a specific MCP server
func (a *BaseAgent) ListMCPTools(ctx context.Context, serverName string) (*mcp.ListToolsResult, error) {
	// Get session for the specified server
	session, ok := a.mcpSessions[serverName]
	if !ok {
		return nil, fmt.Errorf("MCP server %s not found", serverName)
	}

	// List tools from session
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return result, nil
}

// GetConfig returns the agent's configuration
func (a *BaseAgent) GetConfig() *AgentConfig {
	return a.config
}

// GetName returns the agent's name
func (a *BaseAgent) GetName() string {
	return a.name
}

// GetType returns the agent's type
func (a *BaseAgent) GetType() string {
	return a.agentType
}

// GetVersion returns the agent's version
func (a *BaseAgent) GetVersion() string {
	return a.version
}
