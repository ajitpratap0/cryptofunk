// Package mcpserver provides a shared MCP server base that supports both
// stdio and Streamable HTTP transports.
package mcpserver

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// DefaultPorts maps server names to their default HTTP ports.
var DefaultPorts = map[string]int{
	"market-data":          8090,
	"order-executor":       8091,
	"risk-analyzer":        8092,
	"technical-indicators": 8093,
	"polymarket":           8094,
}

// ToolDef defines a tool to register with the server.
type ToolDef struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

// Config holds server configuration.
type Config struct {
	Name    string
	Version string
	// MetricsPort is the port for the Prometheus metrics endpoint.
	// If 0, metrics are disabled.
	MetricsPort int
	Logger      zerolog.Logger
}

// Server wraps mcp.Server with transport selection and metrics.
type Server struct {
	config   Config
	mcpSrv   *mcp.Server
	logger   zerolog.Logger
	tools    []ToolDef
	registry *prometheus.Registry
	toolCall *prometheus.CounterVec
	toolDur  *prometheus.HistogramVec
}

// New creates a new MCP server with the given config.
func New(cfg Config) *Server {
	impl := &mcp.Implementation{
		Name:    cfg.Name,
		Version: cfg.Version,
	}
	mcpSrv := mcp.NewServer(impl, nil)

	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	s := &Server{
		config: cfg,
		mcpSrv: mcpSrv,
		logger: cfg.Logger,
		registry: reg,
		toolCall: factory.NewCounterVec(prometheus.CounterOpts{
			Name:        "mcp_tool_calls_total",
			Help:        "Total MCP tool calls",
			ConstLabels: prometheus.Labels{"server": cfg.Name},
		}, []string{"tool", "status"}),
		toolDur: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "mcp_tool_duration_seconds",
			Help:        "MCP tool call duration",
			ConstLabels: prometheus.Labels{"server": cfg.Name},
			Buckets:     prometheus.DefBuckets,
		}, []string{"tool"}),
	}

	return s
}

// AddTool registers a tool with the server.
// The handler is wrapped with metrics instrumentation.
func (s *Server) AddTool(def ToolDef) {
	s.tools = append(s.tools, def)

	// Wrap handler with metrics
	name := def.Tool.Name
	wrapped := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		result, err := def.Handler(ctx, req)
		dur := time.Since(start).Seconds()
		s.toolDur.WithLabelValues(name).Observe(dur)
		status := "success"
		if err != nil {
			status = "error"
		} else if result != nil && result.IsError {
			status = "error"
		}
		s.toolCall.WithLabelValues(name, status).Inc()
		return result, err
	}

	s.mcpSrv.AddTool(def.Tool, wrapped)
}

// AddToolRaw registers a tool using raw mcp.Tool and mcp.ToolHandler directly.
func (s *Server) AddToolRaw(tool *mcp.Tool, handler mcp.ToolHandler) {
	s.AddTool(ToolDef{Tool: tool, Handler: handler})
}

// MCPServer returns the underlying mcp.Server for advanced usage.
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpSrv
}

// Run parses CLI flags and starts the server with the selected transport.
func (s *Server) Run() error {
	transport := flag.String("transport", "stdio", "Transport mode: stdio or http")
	port := flag.Int("port", DefaultPorts[s.config.Name], "HTTP port (only used with --transport http)")
	metricsPort := flag.Int("metrics-port", s.config.MetricsPort, "Prometheus metrics port (0 to disable)")
	flag.Parse()

	// Start metrics server if configured
	if *metricsPort > 0 {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
			addr := fmt.Sprintf(":%d", *metricsPort)
			s.logger.Info().Str("addr", addr).Msg("Starting metrics server")
			if err := http.ListenAndServe(addr, mux); err != nil {
				s.logger.Error().Err(err).Msg("Metrics server failed")
			}
		}()
	}

	switch *transport {
	case "stdio":
		return s.runStdio()
	case "http":
		return s.runHTTP(*port)
	default:
		return fmt.Errorf("unknown transport: %s (use stdio or http)", *transport)
	}
}

// RunWithArgs starts the server with explicit args (useful for testing).
func (s *Server) RunWithArgs(transport string, port int) error {
	switch transport {
	case "stdio":
		return s.runStdio()
	case "http":
		return s.runHTTP(port)
	default:
		return fmt.Errorf("unknown transport: %s", transport)
	}
}

// runStdio runs the server using stdio transport (stdin/stdout).
func (s *Server) runStdio() error {
	s.logger.Info().Msg("MCP server starting with stdio transport")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		s.logger.Info().Msg("Shutdown signal received")
		cancel()
	}()

	// For stdio, we use the raw JSON-RPC protocol like before
	// since the SDK's stdio transport is client-side only
	return s.runStdioLoop(ctx)
}

// runStdioLoop implements the stdio JSON-RPC loop compatible with the existing protocol.
func (s *Server) runStdioLoop(ctx context.Context) error {
	s.logger.Info().Msg("MCP server ready, listening on stdio")

	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err.Error() == "EOF" {
				s.logger.Info().Msg("Client disconnected")
				return nil
			}
			s.logger.Error().Err(err).Msg("Failed to decode request")
			continue
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			s.logger.Error().Err(err).Msg("Failed to parse request")
			continue
		}

		s.logger.Debug().Str("method", req.Method).Msg("Received request")

		resp := s.HandleJSONRPC(ctx, req.Method, req.Params, req.ID)
		if err := encoder.Encode(resp); err != nil {
			s.logger.Error().Err(err).Msg("Failed to encode response")
			return err
		}
	}
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id"`
	Result  interface{}  `json:"result,omitempty"`
	Error   *JSONRPCErr  `json:"error,omitempty"`
}

// JSONRPCErr is a JSON-RPC 2.0 error object.
type JSONRPCErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HandleJSONRPC dispatches a JSON-RPC request to the appropriate handler.
// Exported for testing compatibility.
func (s *Server) HandleJSONRPC(ctx context.Context, method string, params json.RawMessage, id any) *JSONRPCResponse {
	resp := &JSONRPCResponse{JSONRPC: "2.0", ID: id}

	switch method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    s.config.Name,
				"version": s.config.Version,
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		}

	case "tools/list":
		tools := make([]map[string]interface{}, 0, len(s.tools))
		for _, t := range s.tools {
			toolMap := map[string]interface{}{
				"name":        t.Tool.Name,
				"description": t.Tool.Description,
				"inputSchema": t.Tool.InputSchema,
			}
			tools = append(tools, toolMap)
		}
		resp.Result = map[string]interface{}{"tools": tools}

	case "tools/call":
		var toolParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(params, &toolParams); err != nil {
			resp.Error = &JSONRPCErr{Code: -32602, Message: fmt.Sprintf("Invalid params: %v", err)}
			return resp
		}

		// Find and call the tool handler
		var handler mcp.ToolHandler
		for _, t := range s.tools {
			if t.Tool.Name == toolParams.Name {
				handler = t.Handler
				break
			}
		}
		if handler == nil {
			resp.Error = &JSONRPCErr{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", toolParams.Name)}
			return resp
		}

		// Build a CallToolRequest
		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      toolParams.Name,
				Arguments: toolParams.Arguments,
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			resp.Error = &JSONRPCErr{Code: -32000, Message: err.Error()}
		} else {
			resp.Result = result
		}

	case "notifications/initialized":
		// Client notification, no response needed but we return empty result
		resp.Result = map[string]interface{}{}

	default:
		resp.Error = &JSONRPCErr{Code: -32601, Message: fmt.Sprintf("Method not found: %s", method)}
	}

	return resp
}
