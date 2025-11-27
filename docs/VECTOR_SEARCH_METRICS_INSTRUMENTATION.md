# Vector Search Metrics Instrumentation Guide

This document describes how to instrument the decision API handlers with vector search metrics for monitoring via the Explainability Dashboard.

## Overview

The new vector search metrics track the performance and reliability of pgvector-based semantic search operations in the LLM decision explainability system.

## Metrics Added

### 1. `cryptofunk_vector_search_latency_seconds`
- **Type**: Histogram
- **Labels**: `operation`, `status`
- **Description**: Latency of vector search operations in seconds
- **Buckets**: 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0 seconds
- **Operations**:
  - `semantic_search` - POST /api/v1/decisions/search with embedding
  - `similar_decisions` - GET /api/v1/decisions/:id/similar

### 2. `cryptofunk_vector_search_operations_total`
- **Type**: Counter
- **Labels**: `operation`, `status`
- **Description**: Total number of vector search operations
- **Status values**: `success`, `error`

### 3. `cryptofunk_vector_search_results`
- **Type**: Histogram
- **Labels**: `operation`
- **Description**: Number of results returned by vector search operations
- **Buckets**: 0, 1, 5, 10, 20, 50, 100 results

## Instrumentation Points

### Location: `internal/api/decisions.go`

The following functions should be instrumented:

#### 1. `SearchDecisions` (Semantic Search)

**Function**: `(r *DecisionRepository) searchByEmbedding(ctx context.Context, req SearchRequest)`

Add timing and metrics recording:

```go
func (r *DecisionRepository) searchByEmbedding(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	start := time.Now()

	query := `
		SELECT
			id, session_id, decision_type, symbol, agent_name, prompt, response,
			model, tokens_used, latency_ms, confidence, outcome, outcome_pnl,
			created_at,
			1 - (prompt_embedding <=> $1::vector) as similarity
		FROM llm_decisions
		WHERE prompt_embedding IS NOT NULL
	`
	// ... rest of query building and execution ...

	// Record metrics
	duration := time.Since(start).Seconds()
	metrics.RecordSemanticSearch(duration, len(results), err)

	// Also record database query metrics for visibility
	metrics.RecordDatabaseQuery("vector_search", float64(time.Since(start).Milliseconds()))

	return results, err
}
```

#### 2. `FindSimilarDecisions`

**Function**: `(r *DecisionRepository) FindSimilarDecisions(ctx context.Context, id uuid.UUID, limit int)`

Add timing and metrics recording:

```go
func (r *DecisionRepository) FindSimilarDecisions(ctx context.Context, id uuid.UUID, limit int) ([]Decision, error) {
	start := time.Now()

	query := `
		WITH target AS (
			SELECT prompt_embedding
			FROM llm_decisions
			WHERE id = $1 AND prompt_embedding IS NOT NULL
		)
		SELECT
			d.id, d.session_id, d.decision_type, d.symbol, d.agent_name, d.prompt, d.response,
			d.model, d.tokens_used, d.latency_ms, d.confidence, d.outcome, d.outcome_pnl,
			d.created_at,
			d.prompt_embedding <=> t.prompt_embedding as distance
		FROM llm_decisions d, target t
		WHERE d.id != $1
			AND d.prompt_embedding IS NOT NULL
		ORDER BY d.prompt_embedding <=> t.prompt_embedding
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, id, limit)
	// ... rest of query processing ...

	// Record metrics
	duration := time.Since(start).Seconds()
	metrics.RecordSimilarDecisions(duration, len(decisions), err)

	// Also record database query metrics
	metrics.RecordDatabaseQuery("vector_search", float64(time.Since(start).Milliseconds()))

	return decisions, err
}
```

### Additional Instrumentation for List/Get Operations

For completeness, also instrument the standard query operations:

```go
func (r *DecisionRepository) ListDecisions(ctx context.Context, filter DecisionFilter) ([]Decision, error) {
	start := time.Now()

	// ... existing query logic ...

	// Record database query metrics
	metrics.RecordDatabaseQuery("list_decisions", float64(time.Since(start).Milliseconds()))

	return decisions, rows.Err()
}

func (r *DecisionRepository) GetDecision(ctx context.Context, id uuid.UUID) (*Decision, error) {
	start := time.Now()

	// ... existing query logic ...

	// Record database query metrics
	metrics.RecordDatabaseQuery("get_decision", float64(time.Since(start).Milliseconds()))

	return &d, nil
}
```

## Implementation Steps

1. **Import metrics package** in `internal/api/decisions.go`:
   ```go
   import (
       "time"
       "github.com/ajitpratap0/cryptofunk/internal/metrics"
   )
   ```

2. **Add timing variables** at the start of instrumented functions:
   ```go
   start := time.Now()
   ```

3. **Record metrics before returning** (ensure all return paths are covered):
   ```go
   duration := time.Since(start).Seconds()
   metrics.RecordSemanticSearch(duration, len(results), err)
   ```

4. **Handle error cases** - record metrics even on error:
   ```go
   if err != nil {
       duration := time.Since(start).Seconds()
       metrics.RecordSemanticSearch(duration, 0, err)
       return nil, err
   }
   ```

## Dashboard Access

After instrumentation is complete, the metrics will be available in the Grafana dashboard:

**Dashboard**: CryptoFunk - Explainability & Vector Search
**Path**: `deployments/grafana/dashboards/explainability-dashboard.json`
**URL**: http://localhost:3000/d/cryptofunk-explainability

### Dashboard Panels

1. **Vector Search Latency - Semantic Search**: p50, p95, p99 latencies
2. **Vector Search Latency - Similar Decisions**: p50, p95, p99 latencies
3. **Decision API Request Rate**: Requests/sec by endpoint
4. **Search Error Rate**: Error percentage over time
5. **Database Connection Pool**: Active vs idle connections
6. **Database Query Latency**: Query performance by type
7. **Decision API Request Volume**: Total requests per endpoint
8. **Decision API Response Time Distribution**: p50, p90, p95, p99
9. **Decision API Error Breakdown**: HTTP errors by status code
10. **Vector Search Summary**: Key metrics at a glance

## Testing

After implementing instrumentation:

1. **Start the API server**:
   ```bash
   task run-api
   ```

2. **Generate test traffic**:
   ```bash
   # Semantic search
   curl -X POST http://localhost:8080/api/v1/decisions/search \
     -H "Content-Type: application/json" \
     -d '{"query": "BTC", "limit": 10}'

   # Similar decisions (replace {id} with actual decision ID)
   curl http://localhost:8080/api/v1/decisions/{id}/similar?limit=10
   ```

3. **View metrics**:
   ```bash
   curl http://localhost:8080/metrics | grep vector_search
   ```

4. **Expected output**:
   ```
   # HELP cryptofunk_vector_search_latency_seconds Latency of vector search operations in seconds
   # TYPE cryptofunk_vector_search_latency_seconds histogram
   cryptofunk_vector_search_latency_seconds_bucket{operation="semantic_search",status="success",le="0.01"} 0
   cryptofunk_vector_search_latency_seconds_bucket{operation="semantic_search",status="success",le="0.05"} 0
   cryptofunk_vector_search_latency_seconds_bucket{operation="semantic_search",status="success",le="0.1"} 5
   cryptofunk_vector_search_latency_seconds_bucket{operation="semantic_search",status="success",le="0.25"} 10
   ...
   ```

## Performance Considerations

1. **Minimal Overhead**: Metrics recording adds negligible overhead (<1ms per operation)
2. **Async Recording**: Prometheus client library handles metric updates asynchronously
3. **Cardinality**: Label cardinality is bounded (2 operations × 2 statuses = 4 time series)
4. **Memory**: Each histogram uses ~40KB per unique label combination

## Alerting Rules (Optional)

Add Prometheus alerting rules in `deployments/prometheus/alerts.yml`:

```yaml
groups:
  - name: vector_search
    rules:
      - alert: HighVectorSearchLatency
        expr: histogram_quantile(0.95, rate(cryptofunk_vector_search_latency_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High vector search latency detected"
          description: "p95 vector search latency is {{ $value }}s (threshold: 2s)"

      - alert: VectorSearchErrorRate
        expr: (sum(rate(cryptofunk_vector_search_operations_total{status="error"}[5m])) / sum(rate(cryptofunk_vector_search_operations_total[5m]))) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High vector search error rate"
          description: "Error rate is {{ $value | humanizePercentage }}"
```

## Troubleshooting

### Metrics Not Appearing

1. **Check imports**: Ensure `metrics` package is imported
2. **Verify registration**: Metrics are auto-registered via `promauto.NewHistogramVec`
3. **Check endpoint**: Visit http://localhost:8080/metrics
4. **Test queries**: Run actual vector searches to generate data

### Dashboard Shows No Data

1. **Verify Prometheus scraping**: Check Prometheus targets at http://localhost:9090/targets
2. **Check time range**: Ensure dashboard time range includes recent data
3. **Query Prometheus directly**: Test queries in Prometheus console
4. **Verify datasource**: Ensure Grafana datasource is configured correctly

### High Latency

If vector search latency is high (>1s):

1. **Check index status**:
   ```sql
   SELECT * FROM pg_indexes WHERE tablename = 'llm_decisions';
   ```

2. **Verify pgvector extension**:
   ```sql
   SELECT * FROM pg_extension WHERE extname = 'vector';
   ```

3. **Check index type** (should be IVFFlat or HNSW):
   ```sql
   SELECT indexname, indexdef FROM pg_indexes
   WHERE tablename = 'llm_decisions' AND indexdef LIKE '%vector%';
   ```

4. **Monitor connection pool**: High latency may indicate connection pool exhaustion

## Related Documentation

- **Grafana Dashboard**: `/deployments/grafana/dashboards/explainability-dashboard.json`
- **Metrics Package**: `/internal/metrics/metrics.go`
- **Decision API**: `/internal/api/decisions.go`
- **API Endpoints**: `/docs/API.md`
- **Database Schema**: `/migrations/004_llm_decisions_enhancement.sql`
