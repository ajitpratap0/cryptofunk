package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateDatabaseConnections(t *testing.T) {
	// Test updating database connections
	UpdateDatabaseConnections(5, 2)

	// We can't directly assert the metric values as they're global,
	// but we can verify the function doesn't panic
	assert.NotPanics(t, func() {
		UpdateDatabaseConnections(10, 3)
		UpdateDatabaseConnections(0, 0)
		UpdateDatabaseConnections(100, 50)
	})
}

func TestRecordAPIRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		statusCode string
		durationMs float64
	}{
		{
			name:       "GET request success",
			method:     "GET",
			path:       "/api/trades",
			statusCode: "200",
			durationMs: 45.5,
		},
		{
			name:       "POST request created",
			method:     "POST",
			path:       "/api/orders",
			statusCode: "201",
			durationMs: 120.3,
		},
		{
			name:       "GET request not found",
			method:     "GET",
			path:       "/api/unknown",
			statusCode: "404",
			durationMs: 5.2,
		},
		{
			name:       "POST request error",
			method:     "POST",
			path:       "/api/orders",
			statusCode: "500",
			durationMs: 250.8,
		},
		{
			name:       "Zero duration",
			method:     "GET",
			path:       "/health",
			statusCode: "200",
			durationMs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordAPIRequest(tt.method, tt.path, tt.statusCode, tt.durationMs)
			})
		})
	}
}

func TestRecordError(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		component string
	}{
		{
			name:      "database error",
			errorType: "database_timeout",
			component: "order_executor",
		},
		{
			name:      "api error",
			errorType: "invalid_request",
			component: "api",
		},
		{
			name:      "exchange error",
			errorType: "rate_limit",
			component: "binance",
		},
		{
			name:      "agent error",
			errorType: "timeout",
			component: "technical_agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordError(tt.errorType, tt.component)
			})
		})
	}
}

func TestRecordDatabaseQuery(t *testing.T) {
	tests := []struct {
		name       string
		queryType  string
		durationMs float64
	}{
		{
			name:       "SELECT query fast",
			queryType:  "SELECT",
			durationMs: 2.5,
		},
		{
			name:       "INSERT query",
			queryType:  "INSERT",
			durationMs: 15.3,
		},
		{
			name:       "UPDATE query slow",
			queryType:  "UPDATE",
			durationMs: 250.7,
		},
		{
			name:       "DELETE query",
			queryType:  "DELETE",
			durationMs: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordDatabaseQuery(tt.queryType, tt.durationMs)
			})
		})
	}
}

func TestRecordMCPToolCall(t *testing.T) {
	tests := []struct {
		name       string
		toolName   string
		server     string
		durationMs float64
	}{
		{
			name:       "get_price call",
			toolName:   "get_price",
			server:     "market-data",
			durationMs: 25.5,
		},
		{
			name:       "calculate_rsi call",
			toolName:   "calculate_rsi",
			server:     "technical-indicators",
			durationMs: 45.3,
		},
		{
			name:       "place_order call",
			toolName:   "place_order",
			server:     "order-executor",
			durationMs: 150.2,
		},
		{
			name:       "fast call",
			toolName:   "get_cache",
			server:     "cache",
			durationMs: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordMCPToolCall(tt.toolName, tt.server, tt.durationMs)
			})
		})
	}
}

func TestRecordAgentSignal(t *testing.T) {
	tests := []struct {
		name       string
		agentType  string
		signalType string
		confidence float64
	}{
		{
			name:       "technical agent BUY signal high confidence",
			agentType:  "technical",
			signalType: "BUY",
			confidence: 0.85,
		},
		{
			name:       "trend agent SELL signal medium confidence",
			agentType:  "trend",
			signalType: "SELL",
			confidence: 0.65,
		},
		{
			name:       "sentiment agent HOLD signal low confidence",
			agentType:  "sentiment",
			signalType: "HOLD",
			confidence: 0.45,
		},
		{
			name:       "zero confidence",
			agentType:  "test",
			signalType: "NONE",
			confidence: 0.0,
		},
		{
			name:       "max confidence",
			agentType:  "test",
			signalType: "BUY",
			confidence: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordAgentSignal(tt.agentType, tt.signalType, tt.confidence)
			})
		})
	}
}

func TestRecordAgentProcessing(t *testing.T) {
	tests := []struct {
		name       string
		agentType  string
		durationMs float64
	}{
		{
			name:       "technical agent fast processing",
			agentType:  "technical",
			durationMs: 50.5,
		},
		{
			name:       "trend agent medium processing",
			agentType:  "trend",
			durationMs: 250.3,
		},
		{
			name:       "risk agent slow processing",
			agentType:  "risk",
			durationMs: 1500.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordAgentProcessing(tt.agentType, tt.durationMs)
			})
		})
	}
}

func TestSetAgentStatus(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		online    bool
	}{
		{
			name:      "technical agent online",
			agentType: "technical",
			online:    true,
		},
		{
			name:      "trend agent offline",
			agentType: "trend",
			online:    false,
		},
		{
			name:      "risk agent online",
			agentType: "risk",
			online:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				SetAgentStatus(tt.agentType, tt.online)
			})
		})
	}
}

func TestRecordLLMDecision(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		decisionType string
		durationMs   float64
	}{
		{
			name:         "Claude decision fast",
			model:        "claude-3-sonnet",
			decisionType: "trade_analysis",
			durationMs:   500.5,
		},
		{
			name:         "GPT-4 decision medium",
			model:        "gpt-4",
			decisionType: "risk_assessment",
			durationMs:   1200.3,
		},
		{
			name:         "Claude decision slow",
			model:        "claude-3-opus",
			decisionType: "strategy_generation",
			durationMs:   3500.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordLLMDecision(tt.model, tt.decisionType, tt.durationMs)
			})
		})
	}
}

func TestRecordVotingResult(t *testing.T) {
	tests := []struct {
		name     string
		decision string
	}{
		{
			name:     "BUY decision",
			decision: "BUY",
		},
		{
			name:     "SELL decision",
			decision: "SELL",
		},
		{
			name:     "HOLD decision",
			decision: "HOLD",
		},
		{
			name:     "VETO decision",
			decision: "VETO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordVotingResult(tt.decision)
			})
		})
	}
}

func TestRecordTrade(t *testing.T) {
	tests := []struct {
		name       string
		profitLoss float64
	}{
		{
			name:       "winning trade",
			profitLoss: 150.50,
		},
		{
			name:       "losing trade",
			profitLoss: -75.25,
		},
		{
			name:       "breakeven trade",
			profitLoss: 0.0,
		},
		{
			name:       "large winning trade",
			profitLoss: 1000.00,
		},
		{
			name:       "large losing trade",
			profitLoss: -500.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordTrade(tt.profitLoss)
			})
		})
	}
}

func TestUpdatePositionValue(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		value  float64
	}{
		{
			name:   "BTC position",
			symbol: "BTC/USDT",
			value:  50000.00,
		},
		{
			name:   "ETH position",
			symbol: "ETH/USDT",
			value:  10000.00,
		},
		{
			name:   "zero value position",
			symbol: "DOGE/USDT",
			value:  0.0,
		},
		{
			name:   "small position",
			symbol: "ADA/USDT",
			value:  100.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				UpdatePositionValue(tt.symbol, tt.value)
			})
		})
	}
}

func TestRecordRedisOperation(t *testing.T) {
	tests := []struct {
		name      string
		operation string
	}{
		{
			name:      "GET operation",
			operation: "get",
		},
		{
			name:      "SET operation",
			operation: "set",
		},
		{
			name:      "DEL operation",
			operation: "del",
		},
		{
			name:      "EXISTS operation",
			operation: "exists",
		},
		{
			name:      "EXPIRE operation",
			operation: "expire",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordRedisOperation(tt.operation)
			})
		})
	}
}

func TestUpdateCircuitBreaker(t *testing.T) {
	tests := []struct {
		name        string
		breakerType string
		active      bool
	}{
		{
			name:        "drawdown breaker active",
			breakerType: "max_drawdown",
			active:      true,
		},
		{
			name:        "volatility breaker inactive",
			breakerType: "high_volatility",
			active:      false,
		},
		{
			name:        "order rate breaker active",
			breakerType: "order_rate",
			active:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				UpdateCircuitBreaker(tt.breakerType, tt.active)
			})
		})
	}
}

func TestRecordCircuitBreakerTrip(t *testing.T) {
	tests := []struct {
		name        string
		breakerType string
		reason      string
	}{
		{
			name:        "drawdown trip",
			breakerType: "max_drawdown",
			reason:      "exceeded_threshold",
		},
		{
			name:        "volatility trip",
			breakerType: "high_volatility",
			reason:      "market_unstable",
		},
		{
			name:        "order rate trip",
			breakerType: "order_rate",
			reason:      "too_many_orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordCircuitBreakerTrip(tt.breakerType, tt.reason)
			})
		})
	}
}

func TestRecordExchangeAPICall(t *testing.T) {
	tests := []struct {
		name       string
		exchange   string
		endpoint   string
		durationMs float64
		err        error
	}{
		{
			name:       "successful binance call",
			exchange:   "binance",
			endpoint:   "/api/v3/ticker/price",
			durationMs: 50.5,
			err:        nil,
		},
		{
			name:       "failed coinbase call",
			exchange:   "coinbase",
			endpoint:   "/products",
			durationMs: 250.3,
			err:        assert.AnError,
		},
		{
			name:       "slow kraken call",
			exchange:   "kraken",
			endpoint:   "/0/public/Ticker",
			durationMs: 1500.7,
			err:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordExchangeAPICall(tt.exchange, tt.endpoint, tt.durationMs, tt.err)
			})
		})
	}
}

func TestRecordOrderExecution(t *testing.T) {
	tests := []struct {
		name       string
		durationMs float64
	}{
		{
			name:       "fast execution",
			durationMs: 100.5,
		},
		{
			name:       "medium execution",
			durationMs: 500.3,
		},
		{
			name:       "slow execution",
			durationMs: 2500.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				RecordOrderExecution(tt.durationMs)
			})
		})
	}
}
