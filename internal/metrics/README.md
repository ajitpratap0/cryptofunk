# CryptoFunk Metrics Package

This package provides comprehensive Prometheus metrics instrumentation for the entire CryptoFunk trading system.

## Overview

The metrics package exposes 50+ metrics across three main categories:
- **Trading Performance**: P&L, win rate, positions, returns, Sharpe ratio
- **System Health**: Database, Redis, API latency, errors, NATS messaging
- **Agent Activity**: Signals, confidence, LLM decisions, voting results

## Architecture

### Components

1. **metrics.go** - Metric definitions and helper functions
2. **updater.go** - Periodic database metrics updater
3. **handler.go** - HTTP endpoint handler
4. **middleware.go** - HTTP request instrumentation
5. **redis.go** - Redis operation instrumentation
6. **server.go** - Metrics HTTP server

### Metrics Endpoint

All services expose metrics on `/metrics` endpoint:
- **Orchestrator**: http://localhost:8081/metrics
- **API Server**: http://localhost:8080/metrics

## Integration Guide

### 1. API Server Integration

The API server should start the metrics updater to periodically fetch trading metrics from the database.

```go
// cmd/api/main.go

import (
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

func main() {
	// ... existing setup code ...

	// Create API server with database
	apiServer := &APIServer{
		db:     database,
		// ... other fields ...
	}

	// Start metrics updater (updates every 30 seconds)
	metricsUpdater := metrics.NewUpdater(database.Pool, 30*time.Second)
	go metricsUpdater.Start(ctx)
	defer metricsUpdater.Stop()

	// Apply metrics middleware to Gin router
	apiServer.router.Use(func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := float64(time.Since(start).Milliseconds())
		statusCode := strconv.Itoa(c.Writer.Status())
		metrics.RecordAPIRequest(c.Request.Method, c.FullPath(), statusCode, duration)
	})

	// Register metrics endpoint
	apiServer.router.GET("/metrics", gin.WrapH(metrics.Handler()))

	// ... rest of setup ...
}
```

### 2. Exchange Integration

Instrument exchange API calls to track latency and errors.

```go
// internal/exchange/service.go

import (
	"time"
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
)

func (s *Service) placeOrder(ctx context.Context, order *Order) error {
	start := time.Now()

	// Place order via exchange API
	err := s.exchange.CreateOrder(order)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	metrics.RecordExchangeAPICall("binance", "create_order", duration, err)

	if err != nil {
		metrics.RecordError("exchange_api", "order_executor")
		return err
	}

	// Record order execution
	metrics.RecordOrderExecution(duration)

	return nil
}
```

### 3. Database Query Instrumentation

Wrap database queries to track execution time.

```go
// internal/db/queries.go

import (
	"time"
	"github.com/ajitpratapsingh/dev/cryptofunk/internal/metrics"
)

func (db *DB) GetPositions(ctx context.Context) ([]Position, error) {
	start := time.Now()
	defer func() {
		duration := float64(time.Since(start).Milliseconds())
		metrics.RecordDatabaseQuery("select_positions", duration)
	}()

	// Execute query
	rows, err := db.pool.Query(ctx, "SELECT * FROM positions WHERE status = 'OPEN'")
	// ... rest of implementation
}
```

### 4. Redis Integration

Use the instrumented Redis client to automatically track operations.

```go
// internal/market/cache.go

import (
	"github.com/ajitpratap0/cryptofunk/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	redis *metrics.RedisMetrics
}

func NewCache(client *redis.Client) *Cache {
	return &Cache{
		redis: metrics.NewRedisMetrics(client),
	}
}

func (c *Cache) GetPrice(ctx context.Context, symbol string) (float64, error) {
	// Automatically records GET operation and updates hit rate
	val, err := c.redis.Get(ctx, "price:"+symbol)
	if err != nil {
		return 0, err
	}

	// Parse and return price
	price, _ := strconv.ParseFloat(val, 64)
	return price, nil
}

func (c *Cache) SetPrice(ctx context.Context, symbol string, price float64, ttl time.Duration) error {
	// Automatically records SET operation
	return c.redis.Set(ctx, "price:"+symbol, price, ttl)
}
```

### 5. Agent Signal Recording

Record agent signals with confidence levels.

```go
// cmd/agents/technical-agent/main.go

import (
	"github.com/ajitpratapsingh/cryptofunk/internal/metrics"
)

func (a *TechnicalAgent) generateSignal(ctx context.Context) (*Signal, error) {
	start := time.Now()

	// Generate signal using indicators
	signal := &Signal{
		Type:       "BUY",
		Confidence: 0.85,
		// ... other fields ...
	}

	// Record agent signal with confidence
	metrics.RecordAgentSignal("technical", signal.Type, signal.Confidence)

	// Record processing duration
	duration := float64(time.Since(start).Milliseconds())
	metrics.RecordAgentProcessing("technical", duration)

	return signal, nil
}
```

### 6. MCP Tool Call Instrumentation

Track MCP tool call duration for performance monitoring.

```go
// cmd/mcp-servers/market-data/main.go

import (
	"time"
	"github.com/ajitpratapsingh/cryptofunk/internal/metrics"
)

func (s *MCPServer) handleToolCall(method string, params map[string]interface{}) interface{} {
	start := time.Now()

	// Execute tool
	result := s.executeToolMethod(method, params)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	metrics.RecordMCPToolCall(method, "market-data", duration)

	return result
}
```

### 7. Trading Metrics Recording

Record completed trades to track P&L.

```go
// internal/exchange/execution.go

import (
	"github.com/ajitpratapsingh/cryptofunk/internal/metrics"
)

func (e *Executor) closeTrade(ctx context.Context, trade *Trade) error {
	// Calculate P&L
	pnl := trade.ExitPrice*trade.Quantity - trade.EntryPrice*trade.Quantity

	// Record trade metrics
	metrics.RecordTrade(pnl)

	// Update position value
	metrics.UpdatePositionValue(trade.Symbol, 0) // Position closed

	return nil
}
```

### 8. LLM Decision Recording

Track LLM API calls and decisions.

```go
// internal/llm/client.go

import (
	"time"
	"github.com/ajitpratapsingh/cryptofunk/internal/metrics"
)

func (c *Client) MakeDecision(ctx context.Context, prompt string) (*Decision, error) {
	start := time.Now()

	// Call LLM API
	response, err := c.api.Complete(ctx, prompt)

	// Record metrics
	duration := float64(time.Since(start).Milliseconds())
	metrics.RecordLLMDecision(c.model, "trading", duration)

	if err != nil {
		metrics.RecordError("llm_api", "llm_client")
		return nil, err
	}

	// Parse decision
	decision := parseDecision(response)
	return decision, nil
}
```

### 9. Voting Result Recording

Track orchestrator voting outcomes.

```go
// internal/orchestrator/voting.go

import (
	"github.com/ajitpratapsingh/cryptofunk/internal/metrics"
)

func (o *Orchestrator) executeVote(ctx context.Context, signals []AgentSignal) (*Decision, error) {
	// Calculate weighted vote
	decision := o.calculateConsensus(signals)

	// Record voting result
	if decision.Approved {
		metrics.RecordVotingResult("APPROVED")
	} else {
		metrics.RecordVotingResult("REJECTED")
	}

	return decision, nil
}
```

### 10. Circuit Breaker Monitoring

Track circuit breaker status and trips.

```go
// internal/risk/circuit_breaker.go

import (
	"github.com/ajitpratapsingh/cryptofunk/internal/metrics"
)

func (cb *CircuitBreaker) Trip(reason string) {
	cb.active = true

	// Update metrics
	metrics.UpdateCircuitBreaker(cb.breakerType, true)
	metrics.RecordCircuitBreakerTrip(cb.breakerType, reason)

	log.Warn().
		Str("breaker", cb.breakerType).
		Str("reason", reason).
		Msg("Circuit breaker tripped")
}

func (cb *CircuitBreaker) Reset() {
	cb.active = false
	metrics.UpdateCircuitBreaker(cb.breakerType, false)
}
```

## Metric Categories

### Trading Performance Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `cryptofunk_total_pnl` | Gauge | Total profit/loss in USD |
| `cryptofunk_win_rate` | Gauge | Win rate (0.0-1.0) |
| `cryptofunk_open_positions` | Gauge | Number of open positions |
| `cryptofunk_total_trades` | Counter | Total trades executed |
| `cryptofunk_current_drawdown` | Gauge | Current drawdown ratio |
| `cryptofunk_risk_reward_ratio` | Gauge | Average risk/reward ratio |
| `cryptofunk_sharpe_ratio` | Gauge | Sharpe ratio (risk-adjusted returns) |
| `cryptofunk_daily_return` | Gauge | Daily return ratio |
| `cryptofunk_weekly_return` | Gauge | Weekly return ratio |
| `cryptofunk_monthly_return` | Gauge | Monthly return ratio |

### System Health Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `cryptofunk_database_connections_active` | Gauge | Active DB connections |
| `cryptofunk_database_connections_idle` | Gauge | Idle DB connections |
| `cryptofunk_redis_cache_hit_rate` | Gauge | Redis cache hit rate (0.0-1.0) |
| `cryptofunk_api_request_duration_ms` | Histogram | API request duration |
| `cryptofunk_http_requests_total` | Counter | Total HTTP requests |
| `cryptofunk_errors_total` | Counter | Total errors by type |
| `cryptofunk_database_query_duration_ms` | Histogram | DB query duration |
| `cryptofunk_nats_messages_published_total` | Counter | NATS messages published |
| `cryptofunk_nats_messages_received_total` | Counter | NATS messages received |

### Agent Activity Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `cryptofunk_active_agents` | Gauge | Number of active agents |
| `cryptofunk_agent_signals_total` | Counter | Agent signals by type |
| `cryptofunk_agent_signal_confidence` | Gauge | Agent confidence (0.0-1.0) |
| `cryptofunk_agent_status` | Gauge | Agent online status (1=online) |
| `cryptofunk_agent_processing_duration_ms` | Histogram | Agent processing time |
| `cryptofunk_llm_decisions_total` | Counter | LLM decisions by model |
| `cryptofunk_llm_request_duration_ms` | Histogram | LLM request duration |
| `cryptofunk_voting_results_total` | Counter | Voting results (approved/rejected) |

## Grafana Dashboards

The following pre-configured dashboards are available in `deployments/grafana/dashboards/`:

1. **Trading Performance** (`trading-performance.json`)
   - P&L tracking
   - Win rate and position metrics
   - Drawdown monitoring
   - Return analysis
   - Sharpe ratio

2. **System Health** (`system-health.json`)
   - Service status
   - Database and Redis monitoring
   - API latency and errors
   - NATS messaging
   - MCP tool performance

3. **Agent Activity** (`agent-activity.json`)
   - Agent status and signals
   - Confidence levels
   - LLM decisions
   - Voting results
   - Processing latency

## Query Examples

### PromQL Queries

**Average P&L over last hour:**
```promql
avg_over_time(cryptofunk_total_pnl[1h])
```

**API error rate (5min):**
```promql
rate(cryptofunk_errors_total{component="api"}[5m])
```

**Agent signal rate by type:**
```promql
sum by(signal_type) (rate(cryptofunk_agent_signals_total[5m]))
```

**P95 API latency:**
```promql
histogram_quantile(0.95, rate(cryptofunk_api_request_duration_ms_bucket[5m]))
```

**Redis cache hit rate:**
```promql
cryptofunk_redis_cache_hit_rate
```

## Testing

### Local Testing

Start all services with Docker Compose:
```bash
cd deployments
docker-compose up -d
```

Access metrics:
- Orchestrator: http://localhost:8081/metrics
- API: http://localhost:8080/metrics
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin / password from .env)

### Manual Metric Testing

```bash
# Check if metrics are exposed
curl http://localhost:8081/metrics | grep cryptofunk_

# Query specific metric
curl http://localhost:8081/metrics | grep cryptofunk_total_pnl

# Check Prometheus targets
open http://localhost:9090/targets
```

## Performance Considerations

1. **Metrics Updater**: Runs every 30 seconds by default. Adjust interval based on load:
   ```go
   updater := metrics.NewUpdater(db, 60*time.Second) // 1 minute
   ```

2. **Histogram Buckets**: Pre-defined buckets for latency metrics. Adjust if needed:
   ```go
   Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000}
   ```

3. **Redis Stats**: Cache hit rate is calculated incrementally. Reset periodically if needed:
   ```go
   redisMetrics.ResetStats()
   ```

4. **Label Cardinality**: Avoid high-cardinality labels (e.g., user IDs, timestamps). Use low-cardinality labels like agent_type, symbol, status.

## Troubleshooting

### No metrics appearing

1. Check metrics endpoint is accessible:
   ```bash
   curl http://localhost:8081/metrics
   ```

2. Verify Prometheus is scraping:
   ```bash
   curl http://localhost:9090/api/v1/targets
   ```

3. Check for registration errors in logs

### Duplicate metric registration

Metrics are registered globally on import. Avoid importing metrics package multiple times in test files. Use sync.Once pattern if needed.

### Database metrics not updating

1. Verify metrics updater is started:
   ```go
   go metricsUpdater.Start(ctx)
   ```

2. Check database connectivity
3. Review updater logs for errors

### High memory usage

1. Reduce metrics updater frequency
2. Limit histogram bucket count
3. Review label cardinality

## Best Practices

1. **Always record errors**: Use `metrics.RecordError()` whenever an error occurs
2. **Wrap operations with timing**: Use `defer` for clean metric recording
3. **Use appropriate metric types**:
   - Counter: Monotonically increasing values (requests, errors)
   - Gauge: Values that can go up/down (connections, positions)
   - Histogram: Durations and sizes with percentiles
4. **Keep labels low-cardinality**: Max 10 values per label
5. **Document custom metrics**: Add clear descriptions
6. **Test metrics in development**: Verify metrics appear before production

## Future Enhancements

- [ ] Add custom alerting rules
- [ ] Implement metric aggregation service
- [ ] Add distributed tracing integration
- [ ] Create metric export to TimescaleDB
- [ ] Add anomaly detection on key metrics
- [ ] Implement SLO/SLI tracking
