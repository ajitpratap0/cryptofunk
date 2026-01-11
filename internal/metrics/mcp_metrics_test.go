package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeMCPMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{
			name:     "initialize method",
			method:   "initialize",
			expected: MCPMethodInitialize,
		},
		{
			name:     "tools/list method",
			method:   "tools/list",
			expected: MCPMethodToolsList,
		},
		{
			name:     "tools/call method",
			method:   "tools/call",
			expected: MCPMethodToolsCall,
		},
		{
			name:     "unknown method maps to other",
			method:   "unknown",
			expected: MCPMethodOther,
		},
		{
			name:     "empty method maps to other",
			method:   "",
			expected: MCPMethodOther,
		},
		{
			name:     "resources/list maps to other",
			method:   "resources/list",
			expected: MCPMethodOther,
		},
		{
			name:     "prompts/list maps to other",
			method:   "prompts/list",
			expected: MCPMethodOther,
		},
		{
			name:     "case sensitive - INITIALIZE maps to other",
			method:   "INITIALIZE",
			expected: MCPMethodOther,
		},
		{
			name:     "case sensitive - Tools/List maps to other",
			method:   "Tools/List",
			expected: MCPMethodOther,
		},
		{
			name:     "arbitrary string maps to other",
			method:   "some_random_method",
			expected: MCPMethodOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMCPMethod(tt.method)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeMCPErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected string
	}{
		// Standard JSON-RPC error codes
		{
			name:     "parse error -32700",
			code:     -32700,
			expected: MCPErrorTypeInvalidRequest,
		},
		{
			name:     "invalid request -32600",
			code:     -32600,
			expected: MCPErrorTypeInvalidRequest,
		},
		{
			name:     "method not found -32601",
			code:     -32601,
			expected: MCPErrorTypeMethodNotFound,
		},
		{
			name:     "invalid params -32602",
			code:     -32602,
			expected: MCPErrorTypeInvalidParams,
		},
		{
			name:     "internal error -32603",
			code:     -32603,
			expected: MCPErrorTypeInternalError,
		},
		// Server error range (-32000 to -32099)
		{
			name:     "server error lower bound -32099",
			code:     -32099,
			expected: MCPErrorTypeToolExecution,
		},
		{
			name:     "server error upper bound -32000",
			code:     -32000,
			expected: MCPErrorTypeToolExecution,
		},
		{
			name:     "server error middle -32050",
			code:     -32050,
			expected: MCPErrorTypeToolExecution,
		},
		// Other/Unknown error codes
		{
			name:     "positive error code maps to other",
			code:     1,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "zero error code maps to other",
			code:     0,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "large positive code maps to other",
			code:     99999,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "outside server range -31999 maps to other",
			code:     -31999,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "outside server range -32100 maps to other",
			code:     -32100,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "negative code outside reserved range maps to other",
			code:     -1,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "large negative code maps to other",
			code:     -99999,
			expected: MCPErrorTypeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMCPErrorCode(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecordMCPRequest(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		method   string
		duration time.Duration
		success  bool
	}{
		{
			name:     "successful initialize request",
			server:   "market-data",
			method:   "initialize",
			duration: 50 * time.Millisecond,
			success:  true,
		},
		{
			name:     "failed tools/list request",
			server:   "technical-indicators",
			method:   "tools/list",
			duration: 100 * time.Millisecond,
			success:  false,
		},
		{
			name:     "successful tools/call request",
			server:   "order-executor",
			method:   "tools/call",
			duration: 250 * time.Millisecond,
			success:  true,
		},
		{
			name:     "unknown method request",
			server:   "risk-analyzer",
			method:   "custom/method",
			duration: 10 * time.Millisecond,
			success:  true,
		},
		{
			name:     "zero duration request",
			server:   "test-server",
			method:   "initialize",
			duration: 0,
			success:  true,
		},
		{
			name:     "very long duration request",
			server:   "slow-server",
			method:   "tools/call",
			duration: 5 * time.Second,
			success:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordMCPRequest(tt.server, tt.method, tt.duration, tt.success)
			})
		})
	}
}

func TestRecordMCPError(t *testing.T) {
	tests := []struct {
		name      string
		server    string
		operation string
		errorCode int
	}{
		{
			name:      "parse error",
			server:    "market-data",
			operation: "initialize",
			errorCode: -32700,
		},
		{
			name:      "method not found error",
			server:    "technical-indicators",
			operation: "tools/list",
			errorCode: -32601,
		},
		{
			name:      "invalid params error",
			server:    "order-executor",
			operation: "tools/call",
			errorCode: -32602,
		},
		{
			name:      "internal error",
			server:    "risk-analyzer",
			operation: "analyze",
			errorCode: -32603,
		},
		{
			name:      "tool execution error",
			server:    "order-executor",
			operation: "place_order",
			errorCode: -32050,
		},
		{
			name:      "unknown error code",
			server:    "test-server",
			operation: "test_operation",
			errorCode: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordMCPError(tt.server, tt.operation, tt.errorCode)
			})
		})
	}
}

func TestRecordMCPErrorWithType(t *testing.T) {
	tests := []struct {
		name      string
		server    string
		operation string
		errorType string
	}{
		{
			name:      "timeout error",
			server:    "market-data",
			operation: "get_price",
			errorType: MCPErrorTypeTimeout,
		},
		{
			name:      "connection error",
			server:    "technical-indicators",
			operation: "calculate_rsi",
			errorType: MCPErrorTypeConnectionError,
		},
		{
			name:      "tool not found error",
			server:    "order-executor",
			operation: "unknown_tool",
			errorType: MCPErrorTypeToolNotFound,
		},
		{
			name:      "tool execution error",
			server:    "risk-analyzer",
			operation: "assess_risk",
			errorType: MCPErrorTypeToolExecution,
		},
		{
			name:      "other error type",
			server:    "test-server",
			operation: "test",
			errorType: MCPErrorTypeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordMCPErrorWithType(tt.server, tt.operation, tt.errorType)
			})
		})
	}
}

func TestSetMCPActiveConnections(t *testing.T) {
	tests := []struct {
		name   string
		server string
		count  int
	}{
		{
			name:   "zero connections",
			server: "market-data",
			count:  0,
		},
		{
			name:   "single connection",
			server: "technical-indicators",
			count:  1,
		},
		{
			name:   "multiple connections",
			server: "order-executor",
			count:  10,
		},
		{
			name:   "large number of connections",
			server: "risk-analyzer",
			count:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				SetMCPActiveConnections(tt.server, tt.count)
			})
		})
	}
}

func TestIncrementDecrementMCPActiveConnections(t *testing.T) {
	tests := []struct {
		name   string
		server string
	}{
		{
			name:   "market-data server",
			server: "market-data",
		},
		{
			name:   "technical-indicators server",
			server: "technical-indicators",
		},
		{
			name:   "order-executor server",
			server: "order-executor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				IncrementMCPActiveConnections(tt.server)
			})
			assert.NotPanics(t, func() {
				DecrementMCPActiveConnections(tt.server)
			})
		})
	}
}

func TestMCPRequestTimer(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		method  string
		success bool
	}{
		{
			name:    "successful initialize request timer",
			server:  "market-data",
			method:  "initialize",
			success: true,
		},
		{
			name:    "failed tools/list request timer",
			server:  "technical-indicators",
			method:  "tools/list",
			success: false,
		},
		{
			name:    "successful tools/call request timer",
			server:  "order-executor",
			method:  "tools/call",
			success: true,
		},
		{
			name:    "unknown method request timer",
			server:  "risk-analyzer",
			method:  "custom/method",
			success: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timer := NewMCPRequestTimer(tt.server, tt.method)
			assert.NotNil(t, timer)
			assert.Equal(t, tt.server, timer.server)
			assert.Equal(t, tt.method, timer.method)
			assert.False(t, timer.startTime.IsZero())

			// Simulate some processing time
			time.Sleep(1 * time.Millisecond)

			// Record should not panic
			assert.NotPanics(t, func() {
				timer.Record(tt.success)
			})
		})
	}
}

func TestMCPToolTimer(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		toolName string
		success  bool
	}{
		{
			name:     "successful get_price tool timer",
			server:   "market-data",
			toolName: "get_price",
			success:  true,
		},
		{
			name:     "failed calculate_rsi tool timer",
			server:   "technical-indicators",
			toolName: "calculate_rsi",
			success:  false,
		},
		{
			name:     "successful place_order tool timer",
			server:   "order-executor",
			toolName: "place_order",
			success:  true,
		},
		{
			name:     "failed assess_risk tool timer",
			server:   "risk-analyzer",
			toolName: "assess_risk",
			success:  false,
		},
		{
			name:     "tool with long name",
			server:   "test-server",
			toolName: "very_long_tool_name_that_might_be_used_in_production",
			success:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timer := NewMCPToolTimer(tt.server, tt.toolName)
			assert.NotNil(t, timer)
			assert.Equal(t, tt.server, timer.server)
			assert.Equal(t, tt.toolName, timer.toolName)
			assert.False(t, timer.startTime.IsZero())

			// Simulate some processing time
			time.Sleep(1 * time.Millisecond)

			// Record should not panic
			assert.NotPanics(t, func() {
				timer.Record(tt.success)
			})
		})
	}
}

func TestMCPTimerStartTimeCapture(t *testing.T) {
	// Test that timer captures start time correctly
	before := time.Now()
	requestTimer := NewMCPRequestTimer("test-server", "initialize")
	toolTimer := NewMCPToolTimer("test-server", "test_tool")
	after := time.Now()

	// Request timer start time should be between before and after
	assert.True(t, !requestTimer.startTime.Before(before), "request timer start time should not be before test start")
	assert.True(t, !requestTimer.startTime.After(after), "request timer start time should not be after test end")

	// Tool timer start time should be between before and after
	assert.True(t, !toolTimer.startTime.Before(before), "tool timer start time should not be before test start")
	assert.True(t, !toolTimer.startTime.After(after), "tool timer start time should not be after test end")
}

func TestMCPErrorTypeConstants(t *testing.T) {
	// Verify error type constants are properly defined
	assert.Equal(t, "invalid_request", MCPErrorTypeInvalidRequest)
	assert.Equal(t, "method_not_found", MCPErrorTypeMethodNotFound)
	assert.Equal(t, "invalid_params", MCPErrorTypeInvalidParams)
	assert.Equal(t, "internal_error", MCPErrorTypeInternalError)
	assert.Equal(t, "tool_not_found", MCPErrorTypeToolNotFound)
	assert.Equal(t, "tool_execution", MCPErrorTypeToolExecution)
	assert.Equal(t, "timeout", MCPErrorTypeTimeout)
	assert.Equal(t, "connection", MCPErrorTypeConnectionError)
	assert.Equal(t, "other", MCPErrorTypeOther)
}

func TestMCPMethodConstants(t *testing.T) {
	// Verify method constants are properly defined
	assert.Equal(t, "initialize", MCPMethodInitialize)
	assert.Equal(t, "tools/list", MCPMethodToolsList)
	assert.Equal(t, "tools/call", MCPMethodToolsCall)
	assert.Equal(t, "other", MCPMethodOther)
}

func TestMCPMetricsIntegration(t *testing.T) {
	// Test a full workflow using the metrics
	t.Run("complete request workflow", func(t *testing.T) {
		server := "integration-test-server"

		// Simulate connection establishment
		assert.NotPanics(t, func() {
			IncrementMCPActiveConnections(server)
		})

		// Simulate initialize request
		timer := NewMCPRequestTimer(server, "initialize")
		time.Sleep(1 * time.Millisecond)
		assert.NotPanics(t, func() {
			timer.Record(true)
		})

		// Simulate tools/list request
		timer = NewMCPRequestTimer(server, "tools/list")
		time.Sleep(1 * time.Millisecond)
		assert.NotPanics(t, func() {
			timer.Record(true)
		})

		// Simulate successful tool call
		toolTimer := NewMCPToolTimer(server, "get_data")
		time.Sleep(1 * time.Millisecond)
		assert.NotPanics(t, func() {
			toolTimer.Record(true)
		})

		// Simulate failed tool call with error
		toolTimer = NewMCPToolTimer(server, "process_data")
		time.Sleep(1 * time.Millisecond)
		assert.NotPanics(t, func() {
			toolTimer.Record(false)
			RecordMCPError(server, "process_data", -32603)
		})

		// Simulate connection teardown
		assert.NotPanics(t, func() {
			DecrementMCPActiveConnections(server)
		})
	})
}

func TestNormalizeMCPMethodBoundaryConditions(t *testing.T) {
	// Test with special characters
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{
			name:     "method with spaces",
			method:   "tools / list",
			expected: MCPMethodOther,
		},
		{
			name:     "method with tabs",
			method:   "tools\tlist",
			expected: MCPMethodOther,
		},
		{
			name:     "method with newline",
			method:   "tools\nlist",
			expected: MCPMethodOther,
		},
		{
			name:     "method with special chars",
			method:   "tools/list!@#$",
			expected: MCPMethodOther,
		},
		{
			name:     "unicode method",
			method:   "工具/列表",
			expected: MCPMethodOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMCPMethod(tt.method)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeMCPErrorCodeBoundaryConditions(t *testing.T) {
	// Test boundary conditions for error code ranges
	tests := []struct {
		name     string
		code     int
		expected string
	}{
		{
			name:     "just above server error range",
			code:     -31999,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "just below server error range",
			code:     -32100,
			expected: MCPErrorTypeOther,
		},
		{
			name:     "max int",
			code:     int(^uint(0) >> 1),
			expected: MCPErrorTypeOther,
		},
		{
			name:     "min int",
			code:     -int(^uint(0)>>1) - 1,
			expected: MCPErrorTypeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMCPErrorCode(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}
