package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MCP Server Metrics
// These metrics track MCP protocol operations across all MCP servers
var (
	// MCPRequestsTotal counts total MCP requests by server, method, and status
	MCPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_mcp_requests_total",
		Help: "Total number of MCP requests by server, method, and status",
	}, []string{"server", "method", "status"})

	// MCPRequestDuration tracks MCP request duration by server and method
	MCPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cryptofunk_mcp_request_duration_seconds",
		Help:    "MCP request duration in seconds by server and method",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	}, []string{"server", "method"})

	// MCPToolCallsTotal counts total MCP tool calls by server, tool name, and status
	MCPToolCallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_mcp_tool_calls_total",
		Help: "Total number of MCP tool calls by server, tool name, and status",
	}, []string{"server", "tool", "status"})

	// MCPErrorsTotal counts total MCP errors by server, method/tool, and error type
	MCPErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_mcp_errors_total",
		Help: "Total number of MCP errors by server, operation, and error type",
	}, []string{"server", "operation", "error_type"})

	// MCPActiveConnections tracks active MCP connections per server
	MCPActiveConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cryptofunk_mcp_active_connections",
		Help: "Number of active MCP connections per server",
	}, []string{"server"})
)

// MCP Error Types - bounded set for cardinality control
const (
	MCPErrorTypeInvalidRequest  = "invalid_request"
	MCPErrorTypeMethodNotFound  = "method_not_found"
	MCPErrorTypeInvalidParams   = "invalid_params"
	MCPErrorTypeInternalError   = "internal_error"
	MCPErrorTypeToolNotFound    = "tool_not_found"
	MCPErrorTypeToolExecution   = "tool_execution"
	MCPErrorTypeTimeout         = "timeout"
	MCPErrorTypeConnectionError = "connection"
	MCPErrorTypeOther           = "other"
)

// MCP Method names - bounded set for cardinality control
const (
	MCPMethodInitialize = "initialize"
	MCPMethodToolsList  = "tools/list"
	MCPMethodToolsCall  = "tools/call"
	MCPMethodOther      = "other"
)

// NormalizeMCPMethod maps MCP methods to bounded set
func NormalizeMCPMethod(method string) string {
	switch method {
	case "initialize":
		return MCPMethodInitialize
	case "tools/list":
		return MCPMethodToolsList
	case "tools/call":
		return MCPMethodToolsCall
	default:
		return MCPMethodOther
	}
}

// NormalizeMCPErrorCode maps JSON-RPC error codes to error types
func NormalizeMCPErrorCode(code int) string {
	switch {
	case code == -32700:
		return MCPErrorTypeInvalidRequest // Parse error
	case code == -32600:
		return MCPErrorTypeInvalidRequest // Invalid Request
	case code == -32601:
		return MCPErrorTypeMethodNotFound // Method not found
	case code == -32602:
		return MCPErrorTypeInvalidParams // Invalid params
	case code == -32603:
		return MCPErrorTypeInternalError // Internal error
	case code >= -32099 && code <= -32000:
		return MCPErrorTypeToolExecution // Server error (reserved for implementation-defined errors)
	default:
		return MCPErrorTypeOther
	}
}

// RecordMCPRequest records an MCP request with duration and status
func RecordMCPRequest(server, method string, duration time.Duration, success bool) {
	normalizedMethod := NormalizeMCPMethod(method)
	status := StatusSuccess
	if !success {
		status = StatusError
	}

	MCPRequestsTotal.WithLabelValues(server, normalizedMethod, status).Inc()
	MCPRequestDuration.WithLabelValues(server, normalizedMethod).Observe(duration.Seconds())
}

// RecordMCPToolCall records an MCP tool call with status
func RecordMCPToolCall(server, toolName string, duration time.Duration, success bool) {
	status := StatusSuccess
	if !success {
		status = StatusError
	}

	MCPToolCallsTotal.WithLabelValues(server, toolName, status).Inc()
	// Also record to the existing MCPToolCallDuration metric for backward compatibility
	MCPToolCallDuration.WithLabelValues(toolName, server).Observe(duration.Seconds() * 1000) // Convert to ms for existing metric
}

// RecordMCPError records an MCP error
func RecordMCPError(server, operation string, errorCode int) {
	errorType := NormalizeMCPErrorCode(errorCode)
	MCPErrorsTotal.WithLabelValues(server, operation, errorType).Inc()
}

// RecordMCPErrorWithType records an MCP error with a specific error type
func RecordMCPErrorWithType(server, operation, errorType string) {
	MCPErrorsTotal.WithLabelValues(server, operation, errorType).Inc()
}

// SetMCPActiveConnections sets the number of active connections for a server
func SetMCPActiveConnections(server string, count int) {
	MCPActiveConnections.WithLabelValues(server).Set(float64(count))
}

// IncrementMCPActiveConnections increments the active connection count
func IncrementMCPActiveConnections(server string) {
	MCPActiveConnections.WithLabelValues(server).Inc()
}

// DecrementMCPActiveConnections decrements the active connection count
func DecrementMCPActiveConnections(server string) {
	MCPActiveConnections.WithLabelValues(server).Dec()
}

// MCPRequestTimer is a helper for timing MCP requests
type MCPRequestTimer struct {
	server    string
	method    string
	startTime time.Time
}

// NewMCPRequestTimer creates a new timer for an MCP request
func NewMCPRequestTimer(server, method string) *MCPRequestTimer {
	return &MCPRequestTimer{
		server:    server,
		method:    method,
		startTime: time.Now(),
	}
}

// Record records the request metrics
func (t *MCPRequestTimer) Record(success bool) {
	RecordMCPRequest(t.server, t.method, time.Since(t.startTime), success)
}

// MCPToolTimer is a helper for timing MCP tool calls
type MCPToolTimer struct {
	server    string
	toolName  string
	startTime time.Time
}

// NewMCPToolTimer creates a new timer for an MCP tool call
func NewMCPToolTimer(server, toolName string) *MCPToolTimer {
	return &MCPToolTimer{
		server:    server,
		toolName:  toolName,
		startTime: time.Now(),
	}
}

// Record records the tool call metrics
func (t *MCPToolTimer) Record(success bool) {
	RecordMCPToolCall(t.server, t.toolName, time.Since(t.startTime), success)
}
