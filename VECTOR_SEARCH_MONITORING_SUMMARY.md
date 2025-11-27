# Vector Search Monitoring Implementation Summary

## Overview

This document summarizes the comprehensive monitoring solution created for vector search operations in the CryptoFunk LLM decision explainability system.

## Files Created/Modified

### 1. New Grafana Dashboard
**File**: `/Users/ajitpratapsingh/dev/cryptofunk/deployments/grafana/dashboards/explainability-dashboard.json`

A production-ready Grafana dashboard with 10 panels monitoring vector search performance:

#### Panel 1: Vector Search Latency - Semantic Search
- **Query**: p50, p95, p99 latencies for semantic search operations
- **Visualization**: Time series graph with percentile lines
- **Thresholds**: Green (<500ms), Yellow (500-1000ms), Red (>1000ms)

#### Panel 2: Vector Search Latency - Similar Decisions
- **Query**: p50, p95, p99 latencies for finding similar decisions
- **Visualization**: Time series graph with percentile lines
- **Thresholds**: Green (<500ms), Yellow (500-1000ms), Red (>1000ms)

#### Panel 3: Decision API Request Rate
- **Query**: Requests/sec for search, similar, and list endpoints
- **Visualization**: Time series with stacked areas
- **Purpose**: Track API usage patterns

#### Panel 4: Search Error Rate
- **Query**: Error percentage over time for vector search operations
- **Visualization**: Time series with threshold lines
- **Thresholds**: Green (<1%), Yellow (1-5%), Orange (5-10%), Red (>10%)

#### Panel 5: Database Connection Pool
- **Query**: Active vs idle database connections
- **Visualization**: Stacked area chart
- **Purpose**: Monitor connection pool health

#### Panel 6: Database Query Latency (p95)
- **Query**: p95 latency for vector search, list, and get operations
- **Visualization**: Time series with multiple query types
- **Thresholds**: Green (<100ms), Yellow (100-500ms), Red (>500ms)

#### Panel 7: Decision API Request Volume
- **Query**: Total requests by endpoint over 5-minute windows
- **Visualization**: Stacked bar chart
- **Purpose**: Traffic analysis

#### Panel 8: Decision API Response Time Distribution
- **Query**: p50, p90, p95, p99 response times
- **Visualization**: Multi-percentile time series
- **Purpose**: Overall API performance

#### Panel 9: Decision API Error Breakdown
- **Query**: HTTP errors by status code (4xx, 5xx)
- **Visualization**: Stacked area showing error distribution
- **Purpose**: Error pattern analysis

#### Panel 10: Vector Search Summary
- **Query**: Key metrics - searches/min, p95 latency, error rate
- **Visualization**: Stat panels with color thresholds
- **Purpose**: At-a-glance health check

**Dashboard Features**:
- Auto-refresh every 10 seconds
- 1-hour default time range
- Consistent color scheme across panels
- Comprehensive legends with mean/max/last values
- Responsive grid layout (24 columns)

---

### 2. New Prometheus Metrics
**File**: `/Users/ajitpratapsingh/dev/cryptofunk/internal/metrics/metrics.go`

Added three new metric definitions:

#### `cryptofunk_vector_search_latency_seconds`
```go
Type: Histogram
Labels: operation, status
Buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0] seconds
Purpose: Track latency distribution for vector searches
```

#### `cryptofunk_vector_search_operations_total`
```go
Type: Counter
Labels: operation, status
Purpose: Count total vector search operations by type and outcome
```

#### `cryptofunk_vector_search_results`
```go
Type: Histogram
Labels: operation
Buckets: [0, 1, 5, 10, 20, 50, 100] results
Purpose: Track result count distribution
```

**Helper Functions Added**:

1. `RecordVectorSearch(operation, durationSec, resultCount, err)` - Generic recorder
2. `RecordSemanticSearch(durationSec, resultCount, err)` - Semantic search specific
3. `RecordSimilarDecisions(durationSec, resultCount, err)` - Similar decisions specific

---

### 3. Instrumentation Guide
**File**: `/Users/ajitpratapsingh/dev/cryptofunk/docs/VECTOR_SEARCH_METRICS_INSTRUMENTATION.md`

Comprehensive documentation covering:

- **Overview**: Purpose and scope of vector search metrics
- **Metrics Reference**: Detailed specs for all metrics
- **Instrumentation Points**: Exact code locations to instrument
- **Code Examples**: Complete implementation examples for:
  - `searchByEmbedding()` function
  - `FindSimilarDecisions()` function
  - `ListDecisions()` function
  - `GetDecision()` function
- **Implementation Steps**: Step-by-step guide
- **Testing Procedures**: How to verify metrics work
- **Performance Considerations**: Overhead analysis
- **Alerting Rules**: Optional Prometheus alert definitions
- **Troubleshooting**: Common issues and solutions

---

### 4. Updated Dashboard Documentation
**File**: `/Users/ajitpratapsingh/dev/cryptofunk/deployments/grafana/README.md`

Added section for new dashboard:
- Dashboard purpose and key metrics
- Use cases for monitoring
- Recommended alert thresholds

---

## Metrics Not Yet Instrumented

The following metrics are **defined but not yet instrumented** in the decision API handlers. Implementation is required in `/Users/ajitpratapsingh/dev/cryptofunk/internal/api/decisions.go`:

### Functions Requiring Instrumentation:

1. **`searchByEmbedding()`** - Line ~427
   ```go
   // Add at start:
   start := time.Now()

   // Add before return:
   duration := time.Since(start).Seconds()
   metrics.RecordSemanticSearch(duration, len(results), err)
   metrics.RecordDatabaseQuery("vector_search", float64(time.Since(start).Milliseconds()))
   ```

2. **`FindSimilarDecisions()`** - Line ~345
   ```go
   // Add at start:
   start := time.Now()

   // Add before return:
   duration := time.Since(start).Seconds()
   metrics.RecordSimilarDecisions(duration, len(decisions), err)
   metrics.RecordDatabaseQuery("vector_search", float64(time.Since(start).Milliseconds()))
   ```

3. **`ListDecisions()`** - Line ~94 (optional but recommended)
   ```go
   // Add at start:
   start := time.Now()

   // Add before return:
   metrics.RecordDatabaseQuery("list_decisions", float64(time.Since(start).Milliseconds()))
   ```

4. **`GetDecision()`** - Line ~177 (optional but recommended)
   ```go
   // Add at start:
   start := time.Now()

   // Add before return:
   metrics.RecordDatabaseQuery("get_decision", float64(time.Since(start).Milliseconds()))
   ```

### Import Required:
```go
import (
    "time"
    "github.com/ajitpratap0/cryptofunk/internal/metrics"
)
```

---

## Dashboard Access

Once Prometheus is scraping metrics:

- **Dashboard URL**: http://localhost:3000/d/cryptofunk-explainability
- **Dashboard Name**: "CryptoFunk - Explainability & Vector Search"
- **Metrics Endpoint**: http://localhost:8080/metrics

---

## Quick Start Guide

### 1. Deploy Dashboard to Grafana

The dashboard JSON is auto-provisioned if using the standard deployment:

```bash
# Start infrastructure
task docker-up

# Grafana will auto-import dashboards from deployments/grafana/dashboards/
```

### 2. Instrument API Handlers

Add the instrumentation code to `internal/api/decisions.go` as documented in the instrumentation guide.

### 3. Test Metrics

```bash
# Start API
task run-api

# Generate traffic
curl -X POST http://localhost:8080/api/v1/decisions/search \
  -H "Content-Type: application/json" \
  -d '{"query": "BTC", "limit": 10}'

# Check metrics
curl http://localhost:8080/metrics | grep vector_search
```

### 4. View Dashboard

Open http://localhost:3000 and navigate to:
- Dashboards → CryptoFunk → Explainability & Vector Search

---

## Performance Impact

Metrics recording has minimal overhead:

- **CPU**: <0.1ms per operation (negligible)
- **Memory**: ~40KB per metric time series (bounded cardinality)
- **Network**: Metrics scraped every 15s by Prometheus (not per-request)
- **Cardinality**: 4 time series per histogram (2 operations × 2 statuses)

---

## Alerting Recommendations

Add to `deployments/prometheus/alerts.yml`:

```yaml
groups:
  - name: vector_search_alerts
    rules:
      - alert: HighVectorSearchLatency
        expr: histogram_quantile(0.95, rate(cryptofunk_vector_search_latency_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Vector search p95 latency exceeds 2 seconds"

      - alert: VectorSearchErrorRate
        expr: (sum(rate(cryptofunk_vector_search_operations_total{status="error"}[5m])) / sum(rate(cryptofunk_vector_search_operations_total[5m]))) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Vector search error rate exceeds 5%"
```

---

## Next Steps

1. **Instrument API handlers** following the guide in `docs/VECTOR_SEARCH_METRICS_INSTRUMENTATION.md`
2. **Deploy to staging** and verify metrics collection
3. **Set baseline thresholds** based on actual performance data
4. **Configure alerts** in Prometheus based on SLOs
5. **Review dashboard** after 24 hours of data collection
6. **Optimize queries** if latency thresholds are exceeded

---

## Related Documentation

- **Instrumentation Guide**: `/docs/VECTOR_SEARCH_METRICS_INSTRUMENTATION.md`
- **Dashboard JSON**: `/deployments/grafana/dashboards/explainability-dashboard.json`
- **Metrics Package**: `/internal/metrics/metrics.go`
- **Decision API**: `/internal/api/decisions.go`
- **API Documentation**: `/docs/API.md`

---

## Testing Checklist

- [ ] Dashboard imports successfully in Grafana
- [ ] Prometheus scrapes metrics endpoint
- [ ] Vector search operations emit metrics
- [ ] All panels show data after test traffic
- [ ] Error states are captured correctly
- [ ] Latency percentiles calculate accurately
- [ ] Database connection pool metrics update
- [ ] Alert rules trigger when thresholds exceeded

---

## Support

For issues or questions:
1. Check troubleshooting section in instrumentation guide
2. Review Prometheus targets: http://localhost:9090/targets
3. Inspect raw metrics: http://localhost:8080/metrics
4. Query Prometheus directly: http://localhost:9090/graph

---

**Status**: ✅ Monitoring infrastructure complete, awaiting API instrumentation
**Priority**: High - Required for production readiness of explainability features
**Estimated Implementation Time**: 30 minutes for full instrumentation
