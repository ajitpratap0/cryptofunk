# Vector Search Alerting Guide

This document provides an overview of the Prometheus alerts configured for vector search performance monitoring in CryptoFunk.

## Overview

Vector search is critical for the LLM decision explainability features, enabling semantic search across historical trading decisions using pgvector. Performance degradation in vector search operations can impact user experience and system observability.

## Alert File Location

- **File**: `/deployments/prometheus/alerts/vector_search_alerts.yml`
- **Loaded by**: Prometheus automatically via `rule_files: ["alerts/*.yml"]` configuration
- **Evaluation Interval**: 15 seconds (as configured in each alert group)

## Alert Groups

### 1. Vector Search Performance Alerts

These alerts monitor the latency and reliability of vector search operations.

#### VectorSearchHighLatency
- **Severity**: Warning
- **Threshold**: P95 latency > 2 seconds for 5 minutes
- **Impact**: Degraded user experience for explainability features
- **Common Causes**:
  - Inefficient pgvector index (IVFFlat needs rebuilding)
  - Large result sets being returned
  - Database resource contention
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#vectorsearchhighlatency)

#### VectorSearchCriticalLatency
- **Severity**: Critical
- **Threshold**: P99 latency > 5 seconds for 2 minutes
- **Impact**: Explainability features effectively broken, user-facing timeouts
- **Common Causes**:
  - Severe database performance issues
  - Long-running queries blocking resources
  - Disk I/O bottlenecks
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#vectorsearchcriticallatency)

#### VectorSearchHighErrorRate
- **Severity**: Warning
- **Threshold**: Error rate > 5% for 5 minutes
- **Impact**: Partial failure of explainability features
- **Common Causes**:
  - pgvector extension issues
  - Invalid embedding dimensions (not 1536)
  - Database connection pool exhaustion
  - NULL embeddings in database
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#vectorsearchhigherrorrate)

#### VectorSearchCriticalErrorRate
- **Severity**: Critical
- **Threshold**: Error rate > 20% for 2 minutes
- **Impact**: Vector search completely broken
- **Common Causes**:
  - Missing pgvector extension
  - Corrupted vector index
  - Database connectivity failure
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#vectorsearchcriticalerrorrate)

#### VectorSearchNoOperations
- **Severity**: Info
- **Threshold**: No operations for 30 minutes
- **Impact**: Potential issue with API endpoints or simply low usage
- **Common Causes**:
  - API deployment broke feature
  - Endpoint routing issue
  - Low user activity (normal during off-hours)
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#vectorsearchnooperations)

#### VectorSearchHighQueryVolume
- **Severity**: Warning
- **Threshold**: > 10 queries/second for 10 minutes
- **Impact**: Potential database overload or abuse
- **Common Causes**:
  - Runaway query loop in application
  - User abuse or API misuse
  - Inefficient code triggering excessive searches
- **Response**: Investigate query patterns, implement rate limiting if needed

#### VectorSearchLowResultCount
- **Severity**: Info
- **Threshold**: Average result count < 1 for 15 minutes
- **Impact**: Poor user experience, possibly empty database
- **Common Causes**:
  - Sparse data in llm_decisions table
  - Overly restrictive similarity threshold
  - Index configuration issues
- **Response**: Check data volume, review similarity thresholds

### 2. Vector Search Infrastructure Alerts

These alerts monitor database health factors that affect vector search performance.

#### DatabaseConnectionPoolExhausted
- **Severity**: Warning
- **Threshold**: >90% of connections in use (9+/10) for 5 minutes
- **Impact**: Query queuing, increased latency for all database operations
- **Common Causes**:
  - Connection leaks in application code
  - Slow queries holding connections
  - Traffic spike
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#databaseconnectionpoolexhausted)

#### DatabaseConnectionPoolCritical
- **Severity**: Critical
- **Threshold**: Zero idle connections for 2 minutes
- **Impact**: All new queries blocked, system effectively frozen
- **Common Causes**:
  - Blocking queries
  - Lock contention
  - Application not releasing connections
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#databaseconnectionpoolcritical)

#### VectorSearchSlowWithHighConnections
- **Severity**: Warning
- **Threshold**: P95 latency >1s AND >8 active connections for 5 minutes
- **Impact**: Database contention affecting performance
- **Common Causes**:
  - Multiple slow queries competing for resources
  - High CPU or I/O wait
  - Lock contention
- **Response**: See [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md#vectorsearchslowwithhighconnections)

## Metrics Reference

### Primary Metrics Used

1. **cryptofunk_vector_search_latency_seconds**
   - Type: Histogram
   - Labels: `operation`, `status`
   - Buckets: 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0 seconds
   - Operations: `semantic_search`, `similar_decisions`, `embedding_lookup`

2. **cryptofunk_vector_search_operations_total**
   - Type: Counter
   - Labels: `operation`, `status`
   - Status values: `success`, `error`

3. **cryptofunk_vector_search_results**
   - Type: Histogram
   - Labels: `operation`
   - Buckets: 0, 1, 5, 10, 20, 50, 100 results

4. **cryptofunk_database_connections_active**
   - Type: Gauge
   - Tracks number of active database connections (max: 10)

5. **cryptofunk_database_connections_idle**
   - Type: Gauge
   - Tracks number of idle database connections

## Threshold Tuning

### When to Adjust Thresholds

Alert thresholds are based on typical production workloads. Consider adjusting if:

1. **False Positives**: Alerts fire frequently but no real issue exists
   - Increase latency thresholds or time windows
   - Adjust error rate percentages

2. **False Negatives**: Issues occur without alerts firing
   - Decrease latency thresholds
   - Shorten time windows for critical alerts

3. **Workload Changes**: System usage patterns change significantly
   - Review and update all thresholds
   - Adjust based on new baseline metrics

### Recommended Adjustments

Edit `/deployments/prometheus/alerts/vector_search_alerts.yml` and update the `expr` field:

```yaml
# Example: Increase latency threshold from 2s to 3s
- alert: VectorSearchHighLatency
  expr: histogram_quantile(0.95, rate(cryptofunk_vector_search_latency_seconds_bucket[5m])) > 3  # Changed from 2
```

After editing, reload Prometheus configuration:
```bash
curl -X POST http://prometheus-service:9090/-/reload
```

## Testing Alerts

### Manual Alert Testing

1. **Test latency alerts** (simulate slow queries):
   ```sql
   -- Connect to database
   kubectl exec -it -n cryptofunk deployment/postgres -- \
     psql -U postgres -d cryptofunk

   -- Run a slow vector search
   SELECT id, prompt, response,
          prompt_embedding <=> '[0.1,0.2,...]'::vector as distance
   FROM llm_decisions
   ORDER BY distance
   LIMIT 1000;  -- Large limit to slow it down
   ```

2. **Test error alerts** (simulate failures):
   ```bash
   # Temporarily break pgvector
   kubectl exec -it -n cryptofunk deployment/postgres -- \
     psql -U postgres -d cryptofunk -c "DROP EXTENSION vector CASCADE;"

   # Make API calls to trigger errors
   curl -X POST http://api-service:8080/api/v1/decisions/similar \
     -H "Content-Type: application/json" \
     -d '{"query": "test", "limit": 5}'

   # Restore extension
   kubectl exec -it -n cryptofunk deployment/postgres -- \
     psql -U postgres -d cryptofunk -c "CREATE EXTENSION vector;"
   ```

3. **Check alert status**:
   ```bash
   # View pending/firing alerts
   curl http://prometheus-service:9090/api/v1/alerts | jq '.data.alerts[] | select(.labels.alertname | contains("VectorSearch"))'
   ```

## Grafana Dashboards

Vector search metrics are visualized in the Explainability Dashboard:

- **Dashboard**: `Explainability Dashboard` (ID: 16)
- **Panels**:
  - Vector Search Latency (P50, P95, P99)
  - Vector Search Operations Rate
  - Vector Search Error Rate
  - Search Results Distribution

Access at: `http://grafana-service:3000/d/explainability`

## Integration with AlertManager

Alerts are routed through AlertManager based on severity:

- **Critical alerts**:
  - Slack: `#cryptofunk-critical` (with @channel)
  - Email: `oncall@example.com`
  - PagerDuty: If configured

- **Warning alerts**:
  - Slack: `#cryptofunk-alerts`
  - Email: `team@example.com`

- **Info alerts**:
  - Slack: `#cryptofunk-alerts`
  - No email notification

Configuration: `/deployments/prometheus/alertmanager.yml`

## Maintenance Windows

To silence alerts during maintenance:

```bash
# Silence vector search alerts for 2 hours
amtool silence add \
  alertname=~"VectorSearch.*|DatabaseConnectionPool.*" \
  --duration=2h \
  --comment="Vector search index rebuild maintenance"

# List active silences
amtool silence query

# Remove silence early
amtool silence expire <silence-id>
```

## Troubleshooting

### Alerts Not Firing

1. **Check Prometheus is scraping metrics**:
   ```bash
   curl http://prometheus-service:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="api")'
   ```

2. **Verify metrics exist**:
   ```bash
   curl -G http://prometheus-service:9090/api/v1/query \
     --data-urlencode 'query=cryptofunk_vector_search_latency_seconds_bucket' | jq
   ```

3. **Check rule evaluation**:
   ```bash
   curl http://prometheus-service:9090/api/v1/rules | jq '.data.groups[] | select(.name=="vector_search_performance")'
   ```

### Alerts Firing Incorrectly

1. **Check current metric values**:
   ```bash
   # Check actual latency
   curl -G http://prometheus-service:9090/api/v1/query \
     --data-urlencode 'query=histogram_quantile(0.95, rate(cryptofunk_vector_search_latency_seconds_bucket[5m]))' | jq
   ```

2. **Review alert history**:
   ```bash
   curl -G http://prometheus-service:9090/api/v1/query \
     --data-urlencode 'query=ALERTS{alertname="VectorSearchHighLatency"}[1h]' | jq
   ```

## Related Documentation

- **Alert Runbook**: [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md) - Detailed response procedures
- **Metrics Instrumentation**: [VECTOR_SEARCH_METRICS_INSTRUMENTATION.md](./VECTOR_SEARCH_METRICS_INSTRUMENTATION.md)
- **Architecture**: [CLAUDE.md](../CLAUDE.md) - System architecture and database details
- **API Documentation**: API handlers in `cmd/api/handlers/` for implementation details

## Support

For questions or issues with vector search alerts:

1. Check the [ALERT_RUNBOOK.md](./ALERT_RUNBOOK.md) for response procedures
2. Review Grafana dashboards for metric visualizations
3. Consult #cryptofunk-ops Slack channel
4. Escalate to on-call DBA for critical database issues
