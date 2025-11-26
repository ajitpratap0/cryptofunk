package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Bounded cardinality constants for metric labels.
// These ensure metrics don't have unbounded label values which can cause memory issues.
const (
	// Circuit breaker reasons (bounded set)
	ReasonMaxDrawdown    = "max_drawdown"
	ReasonHighVolatility = "high_volatility"
	ReasonRateLimit      = "rate_limit"
	ReasonManualHalt     = "manual_halt"
	ReasonOther          = "other"

	// Strategy validation failure reasons (bounded set)
	ValidationReasonSchemaInvalid   = "schema_invalid"
	ValidationReasonFieldMissing    = "field_missing"
	ValidationReasonValueOutOfRange = "value_out_of_range"
	ValidationReasonIncompatible    = "incompatible"
	ValidationReasonOther           = "other"

	// Exchange API error categories (bounded set)
	ExchangeErrorTimeout     = "timeout"
	ExchangeErrorRateLimit   = "rate_limit"
	ExchangeErrorAuth        = "authentication"
	ExchangeErrorNetwork     = "network"
	ExchangeErrorInvalidReq  = "invalid_request"
	ExchangeErrorServerError = "server_error"
	ExchangeErrorOther       = "other"
)

// NormalizeCircuitBreakerReason maps arbitrary reasons to bounded set
func NormalizeCircuitBreakerReason(reason string) string {
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "drawdown"):
		return ReasonMaxDrawdown
	case strings.Contains(lower, "volatility"):
		return ReasonHighVolatility
	case strings.Contains(lower, "rate") || strings.Contains(lower, "limit"):
		return ReasonRateLimit
	case strings.Contains(lower, "manual") || strings.Contains(lower, "halt"):
		return ReasonManualHalt
	default:
		return ReasonOther
	}
}

// NormalizeValidationReason maps arbitrary validation failures to bounded set
func NormalizeValidationReason(reason string) string {
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "schema") || strings.Contains(lower, "version"):
		return ValidationReasonSchemaInvalid
	case strings.Contains(lower, "missing") || strings.Contains(lower, "required"):
		return ValidationReasonFieldMissing
	case strings.Contains(lower, "range") || strings.Contains(lower, "value") || strings.Contains(lower, "invalid"):
		return ValidationReasonValueOutOfRange
	case strings.Contains(lower, "compatible") || strings.Contains(lower, "migration"):
		return ValidationReasonIncompatible
	default:
		return ValidationReasonOther
	}
}

// NormalizeExchangeError maps arbitrary error messages to bounded set
func NormalizeExchangeError(err error) string {
	if err == nil {
		return ""
	}
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
		return ExchangeErrorTimeout
	case strings.Contains(errStr, "rate") || strings.Contains(errStr, "429"):
		return ExchangeErrorRateLimit
	case strings.Contains(errStr, "auth") || strings.Contains(errStr, "401") || strings.Contains(errStr, "403"):
		return ExchangeErrorAuth
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection"):
		return ExchangeErrorNetwork
	case strings.Contains(errStr, "400") || strings.Contains(errStr, "invalid"):
		return ExchangeErrorInvalidReq
	case strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503"):
		return ExchangeErrorServerError
	default:
		return ExchangeErrorOther
	}
}

// Trading Performance Metrics
var (
	// Total P&L
	TotalPnL = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_total_pnl",
		Help: "Total profit and loss in USD",
	})

	// Win rate (0.0 to 1.0)
	WinRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_win_rate",
		Help: "Win rate as a ratio (0.0 to 1.0)",
	})

	// Open positions
	OpenPositions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_open_positions",
		Help: "Number of currently open positions",
	})

	// Total trades
	TotalTrades = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cryptofunk_total_trades",
		Help: "Total number of trades executed",
	})

	// Current drawdown
	CurrentDrawdown = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_current_drawdown",
		Help: "Current drawdown as a ratio (0.0 to 1.0)",
	})

	// Max drawdown threshold
	MaxDrawdownThreshold = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_max_drawdown_threshold",
		Help: "Maximum allowed drawdown threshold",
	})

	// Position value by symbol
	PositionValueBySymbol = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cryptofunk_position_value_by_symbol",
		Help: "Position value in USD by trading symbol",
	}, []string{"symbol"})

	// Risk/reward ratio
	RiskRewardRatio = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_risk_reward_ratio",
		Help: "Average risk/reward ratio",
	})

	// Winning trades value
	WinningTradesValue = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cryptofunk_winning_trades_value",
		Help: "Total value of winning trades in USD",
	})

	// Losing trades value
	LosingTradesValue = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cryptofunk_losing_trades_value",
		Help: "Total value (absolute) of losing trades in USD",
	})

	// Daily return
	DailyReturn = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_daily_return",
		Help: "Daily return as a ratio",
	})

	// Weekly return
	WeeklyReturn = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_weekly_return",
		Help: "Weekly return as a ratio",
	})

	// Monthly return
	MonthlyReturn = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_monthly_return",
		Help: "Monthly return as a ratio",
	})

	// Sharpe ratio
	SharpeRatio = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_sharpe_ratio",
		Help: "Sharpe ratio (risk-adjusted return)",
	})
)

// System Health Metrics
var (
	// Active trading sessions
	ActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_active_sessions",
		Help: "Number of currently active trading sessions",
	})

	// Orchestrator latency
	OrchestratorLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cryptofunk_orchestrator_latency_ms",
		Help:    "Orchestrator decision latency in milliseconds",
		Buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000},
	})

	// Database connections
	DatabaseConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_database_connections_active",
		Help: "Number of active database connections",
	})

	DatabaseConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_database_connections_idle",
		Help: "Number of idle database connections",
	})

	// Redis cache hit rate
	RedisCacheHitRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_redis_cache_hit_rate",
		Help: "Redis cache hit rate as a ratio (0.0 to 1.0)",
	})

	// Redis operations
	RedisOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_redis_operations_total",
		Help: "Total number of Redis operations by type",
	}, []string{"operation"})

	// API request duration
	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cryptofunk_api_request_duration_ms",
		Help:    "API request duration in milliseconds",
		Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
	}, []string{"method", "path", "status_code"})

	// HTTP requests
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status_code"})

	// Errors
	Errors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_errors_total",
		Help: "Total number of errors by type",
	}, []string{"type", "component"})

	// Database query duration
	DatabaseQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cryptofunk_database_query_duration_ms",
		Help:    "Database query duration in milliseconds",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{"query_type"})

	// NATS messages
	NATSMessagesPublished = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cryptofunk_nats_messages_published_total",
		Help: "Total number of NATS messages published",
	})

	NATSMessagesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cryptofunk_nats_messages_received_total",
		Help: "Total number of NATS messages received",
	})

	// MCP tool call duration
	MCPToolCallDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cryptofunk_mcp_tool_call_duration_ms",
		Help:    "MCP tool call duration in milliseconds",
		Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500},
	}, []string{"tool_name", "server"})
)

// Agent Activity Metrics
var (
	// Active agents
	ActiveAgents = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cryptofunk_active_agents",
		Help: "Number of currently active agents",
	})

	// Agent signals
	AgentSignals = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_agent_signals_total",
		Help: "Total number of agent signals by type",
	}, []string{"agent_type", "signal_type"})

	// Agent signal confidence
	AgentSignalConfidence = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cryptofunk_agent_signal_confidence",
		Help: "Agent signal confidence level (0.0 to 1.0)",
	}, []string{"agent_type"})

	// Agent signals by status
	AgentSignalsByStatus = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_agent_signals_by_status_total",
		Help: "Total agent signals by status",
	}, []string{"status"})

	// Agent status (1 = online, 0 = offline)
	AgentStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cryptofunk_agent_status",
		Help: "Agent status (1 = online, 0 = offline)",
	}, []string{"agent_type"})

	// Agent processing duration
	AgentProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cryptofunk_agent_processing_duration_ms",
		Help:    "Agent processing duration in milliseconds",
		Buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000},
	}, []string{"agent_type"})

	// LLM decisions
	LLMDecisions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_llm_decisions_total",
		Help: "Total number of LLM decisions",
	}, []string{"model", "decision_type"})

	// LLM request duration
	LLMRequestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cryptofunk_llm_request_duration_ms",
		Help:    "LLM request duration in milliseconds",
		Buckets: []float64{100, 250, 500, 1000, 2500, 5000, 10000},
	})

	// Voting results
	VotingResults = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_voting_results_total",
		Help: "Total voting results by decision",
	}, []string{"decision"})
)

// Circuit Breaker Metrics
var (
	// Circuit breaker status (1 = active, 0 = inactive)
	CircuitBreakerStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cryptofunk_circuit_breaker_status",
		Help: "Circuit breaker status (1 = active/tripped, 0 = inactive)",
	}, []string{"breaker_type"})

	// Circuit breaker trips
	CircuitBreakerTrips = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_circuit_breaker_trips_total",
		Help: "Total number of circuit breaker trips",
	}, []string{"breaker_type", "reason"})
)

// Audit Metrics
var (
	// Audit log operations
	AuditLogOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_audit_log_operations_total",
		Help: "Total number of audit log operations by event type and status",
	}, []string{"event_type", "status"})

	// Audit log failures
	AuditLogFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_audit_log_failures_total",
		Help: "Total number of audit log failures by error type",
	}, []string{"error_type", "event_type"})

	// Audit log latency
	AuditLogLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cryptofunk_audit_log_latency_ms",
		Help:    "Audit log operation latency in milliseconds",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500},
	})

	// Strategy operations metrics
	StrategyOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_strategy_operations_total",
		Help: "Total number of strategy operations by type and status",
	}, []string{"operation", "status"})

	// Strategy validation failures
	StrategyValidationFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_strategy_validation_failures_total",
		Help: "Total number of strategy validation failures by reason",
	}, []string{"reason"})
)

// Exchange Metrics
var (
	// Exchange API latency
	ExchangeAPILatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cryptofunk_exchange_api_latency_ms",
		Help:    "Exchange API latency in milliseconds",
		Buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000},
	}, []string{"exchange", "endpoint"})

	// Exchange API errors
	ExchangeAPIErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cryptofunk_exchange_api_errors_total",
		Help: "Total exchange API errors",
	}, []string{"exchange", "error_type"})

	// Order execution latency
	OrderExecutionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cryptofunk_order_execution_latency_ms",
		Help:    "Order execution latency in milliseconds",
		Buckets: []float64{100, 250, 500, 1000, 2500, 5000},
	})
)

// Helper functions to update metrics

// UpdateDatabaseConnections updates database connection metrics
func UpdateDatabaseConnections(active, idle int32) {
	DatabaseConnectionsActive.Set(float64(active))
	DatabaseConnectionsIdle.Set(float64(idle))
}

// RecordAPIRequest records an API request with duration
func RecordAPIRequest(method, path, statusCode string, durationMs float64) {
	APIRequestDuration.WithLabelValues(method, path, statusCode).Observe(durationMs)
	HTTPRequests.WithLabelValues(method, path, statusCode).Inc()
}

// RecordError records an error
func RecordError(errorType, component string) {
	Errors.WithLabelValues(errorType, component).Inc()
}

// RecordDatabaseQuery records a database query
func RecordDatabaseQuery(queryType string, durationMs float64) {
	DatabaseQueryDuration.WithLabelValues(queryType).Observe(durationMs)
}

// RecordMCPToolCall records an MCP tool call
func RecordMCPToolCall(toolName, server string, durationMs float64) {
	MCPToolCallDuration.WithLabelValues(toolName, server).Observe(durationMs)
}

// RecordAgentSignal records an agent signal
func RecordAgentSignal(agentType, signalType string, confidence float64) {
	AgentSignals.WithLabelValues(agentType, signalType).Inc()
	AgentSignalConfidence.WithLabelValues(agentType).Set(confidence)
}

// RecordAgentProcessing records agent processing duration
func RecordAgentProcessing(agentType string, durationMs float64) {
	AgentProcessingDuration.WithLabelValues(agentType).Observe(durationMs)
}

// SetAgentStatus sets agent online/offline status
func SetAgentStatus(agentType string, online bool) {
	status := 0.0
	if online {
		status = 1.0
	}
	AgentStatus.WithLabelValues(agentType).Set(status)
}

// RecordLLMDecision records an LLM decision
func RecordLLMDecision(model, decisionType string, durationMs float64) {
	LLMDecisions.WithLabelValues(model, decisionType).Inc()
	LLMRequestDuration.Observe(durationMs)
}

// RecordVotingResult records a voting result
func RecordVotingResult(decision string) {
	VotingResults.WithLabelValues(decision).Inc()
}

// RecordTrade records a completed trade
func RecordTrade(profitLoss float64) {
	TotalTrades.Inc()
	if profitLoss > 0 {
		WinningTradesValue.Add(profitLoss)
	} else {
		LosingTradesValue.Add(-profitLoss) // Store absolute value
	}
}

// UpdatePositionValue updates position value for a symbol
func UpdatePositionValue(symbol string, value float64) {
	PositionValueBySymbol.WithLabelValues(symbol).Set(value)
}

// RecordRedisOperation records a Redis operation
func RecordRedisOperation(operation string) {
	RedisOperations.WithLabelValues(operation).Inc()
}

// UpdateCircuitBreaker updates circuit breaker status
func UpdateCircuitBreaker(breakerType string, active bool) {
	status := 0.0
	if active {
		status = 1.0
	}
	CircuitBreakerStatus.WithLabelValues(breakerType).Set(status)
}

// RecordCircuitBreakerTrip records a circuit breaker trip with normalized reason
func RecordCircuitBreakerTrip(breakerType, reason string) {
	normalizedReason := NormalizeCircuitBreakerReason(reason)
	CircuitBreakerTrips.WithLabelValues(breakerType, normalizedReason).Inc()
}

// RecordExchangeAPICall records an exchange API call with normalized error category
func RecordExchangeAPICall(exchange, endpoint string, durationMs float64, err error) {
	ExchangeAPILatency.WithLabelValues(exchange, endpoint).Observe(durationMs)
	if err != nil {
		errorCategory := NormalizeExchangeError(err)
		ExchangeAPIErrors.WithLabelValues(exchange, errorCategory).Inc()
	}
}

// RecordOrderExecution records order execution latency
func RecordOrderExecution(durationMs float64) {
	OrderExecutionLatency.Observe(durationMs)
}

// UpdateActiveSessions updates the number of active trading sessions
func UpdateActiveSessions(count int) {
	ActiveSessions.Set(float64(count))
}

// RecordOrchestratorLatency records orchestrator decision latency
func RecordOrchestratorLatency(durationMs float64) {
	OrchestratorLatency.Observe(durationMs)
}

// RecordAuditLog records an audit log operation
func RecordAuditLog(eventType string, success bool, durationMs float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	AuditLogOperations.WithLabelValues(eventType, status).Inc()
	AuditLogLatency.Observe(durationMs)
}

// RecordAuditLogFailure records an audit log failure with error type
func RecordAuditLogFailure(errorType, eventType string) {
	AuditLogFailures.WithLabelValues(errorType, eventType).Inc()
}

// RecordStrategyOperation records a strategy operation
func RecordStrategyOperation(operation string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	StrategyOperations.WithLabelValues(operation, status).Inc()
}

// RecordStrategyValidationFailure records a strategy validation failure with normalized reason
func RecordStrategyValidationFailure(reason string) {
	normalizedReason := NormalizeValidationReason(reason)
	StrategyValidationFailures.WithLabelValues(normalizedReason).Inc()
}
