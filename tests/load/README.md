# Load Tests for CryptoFunk

This directory contains load testing tools for the CryptoFunk trading system, with a focus on vector search and database-intensive operations.

## Overview

Load tests help ensure that the system can handle expected traffic volumes and identify performance bottlenecks before they impact production.

## Test Files

### `vector_search_load_test.go`

Comprehensive Go-based load tests for vector search endpoints using the standard `testing` package.

**Key Features:**
- Benchmark tests for semantic search (with embeddings)
- Benchmark tests for text-only search
- Benchmark tests for similar decisions endpoint
- Concurrency tests with configurable workers
- Stress tests with high concurrency
- Detailed latency metrics (average, P95, P99)

**Test Types:**

1. **BenchmarkSemanticSearch**: Tests search with 1536-dimensional embedding vectors
2. **BenchmarkTextSearch**: Tests PostgreSQL full-text search
3. **BenchmarkSimilarDecisions**: Tests pgvector similarity queries
4. **TestVectorSearchConcurrency**: Tests 10 concurrent workers (default)
5. **TestSimilarDecisionsConcurrency**: Tests concurrent similar decision queries
6. **TestVectorSearchStress**: Stress test with 50 concurrent workers

## Running Load Tests

### Prerequisites

```bash
# 1. Start infrastructure
task docker-up

# 2. Run database migrations
task db-migrate

# 3. Start API server
task run-api

# 4. (Optional) Populate test data
# You'll need some decisions in the database for meaningful tests
```

### Using Go Tests

```bash
# Run all load tests
go test -v ./tests/load/

# Run specific benchmark
go test -v -bench=BenchmarkSemanticSearch ./tests/load/

# Run all benchmarks
go test -v -bench=. ./tests/load/

# Run concurrency tests (skipped in short mode)
go test -v -run=TestVectorSearchConcurrency ./tests/load/

# Run stress test
go test -v -run=TestVectorSearchStress ./tests/load/

# With custom iterations
go test -v -bench=. -benchtime=100x ./tests/load/
```

### Using Shell Script (with `hey`)

```bash
# Install hey (HTTP load testing tool)
go install github.com/rakyll/hey@latest

# Run load tests against local API
./scripts/load-test-vector-search.sh

# Run against custom API URL
./scripts/load-test-vector-search.sh http://staging.example.com:8080

# Run with custom parameters (edit script)
REQUESTS=200 CONCURRENCY=20 ./scripts/load-test-vector-search.sh
```

## Understanding Results

### Go Benchmark Output

```
BenchmarkSemanticSearch-8    100    12345678 ns/op
```

- `100`: Number of iterations
- `12345678 ns/op`: Nanoseconds per operation (12.3ms in this example)

### Concurrency Test Output

```
Concurrency Test Results:
  Total requests: 100
  Concurrency: 10
  Success: 98 (98.00%)
  Errors: 2 (2.00%)
  Total duration: 5.2s
  Throughput: 19.23 req/s
  Avg latency: 520ms
  P95 latency: 890ms
  P99 latency: 1.2s
```

**What to look for:**
- **Success rate**: Should be >95% for healthy system
- **Error rate**: Should be <5%
- **Avg latency**: Should be <2s for search endpoints
- **P95 latency**: Should be <5s
- **P99 latency**: Should be <8s for expensive operations

### Hey Tool Output

```
Summary:
  Total:        5.0234 secs
  Slowest:      0.8901 secs
  Fastest:      0.1234 secs
  Average:      0.4567 secs
  Requests/sec: 19.91

Response time histogram:
  0.123 [1]     |
  0.200 [15]    |■■■■■
  0.300 [45]    |■■■■■■■■■■■■■■■
  ...

Status code distribution:
  [200] 100 responses
```

## Performance Targets

Based on the codebase's query timeouts and expected usage:

| Metric | Target | Max Acceptable |
|--------|--------|----------------|
| Text Search (avg) | <500ms | <2s |
| Vector Search (avg) | <1s | <3s |
| Similar Decisions (avg) | <1.5s | <3s |
| P95 Latency | <3s | <5s |
| P99 Latency | <5s | <8s |
| Success Rate | >98% | >95% |
| Throughput (10 workers) | >15 req/s | >10 req/s |

## Database Index Configuration

The vector search performance heavily depends on the pgvector index configuration.

### Current Configuration

From `migrations/001_initial_schema.sql`:

```sql
CREATE INDEX idx_llm_decisions_embedding ON llm_decisions
USING ivfflat (prompt_embedding vector_cosine_ops)
WITH (lists = 100);
```

**Index Type**: IVFFlat (Inverted File with Flat quantization)
**Distance Metric**: Cosine similarity (`<=>` operator)
**Lists Parameter**: 100

### Understanding IVFFlat Parameters

**Lists**: Number of clusters for inverted file index
- **Smaller values** (50-100): Faster inserts, slower queries
- **Larger values** (200-500): Slower inserts, faster queries
- **Rule of thumb**: `lists = rows / 1000` (capped between 10-1000)

### When to Tune

Monitor these metrics to decide if tuning is needed:

1. **Query latency** consistently >2s
2. **Index scan cost** is high in `EXPLAIN ANALYZE`
3. **Database size** changes significantly (10x growth)

### Tuning Recommendations

If you have **<10,000 decisions**:
```sql
-- Keep current configuration
-- lists = 100 is appropriate
```

If you have **10,000-100,000 decisions**:
```sql
-- Consider increasing lists
DROP INDEX idx_llm_decisions_embedding;
CREATE INDEX idx_llm_decisions_embedding ON llm_decisions
USING ivfflat (prompt_embedding vector_cosine_ops)
WITH (lists = 200);
```

If you have **>100,000 decisions**:
```sql
-- Consider HNSW index for better performance
DROP INDEX idx_llm_decisions_embedding;
CREATE INDEX idx_llm_decisions_embedding ON llm_decisions
USING hnsw (prompt_embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

### Monitoring Index Performance

```sql
-- Check index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename = 'llm_decisions';

-- Check index size
SELECT
    pg_size_pretty(pg_relation_size('idx_llm_decisions_embedding')) as index_size;

-- Analyze query performance
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM llm_decisions
ORDER BY prompt_embedding <=> '[0.1, 0.2, ...]'::vector
LIMIT 10;
```

## Troubleshooting

### High Error Rates

**Symptoms**: >10% errors during load tests

**Possible causes:**
1. Database connection pool exhausted
2. Query timeouts (default 30s for regular, 60s for vector)
3. Memory issues
4. Insufficient database resources

**Solutions:**
```bash
# Check database connections
psql -h localhost -U postgres -d cryptofunk \
  -c "SELECT count(*) FROM pg_stat_activity WHERE datname='cryptofunk';"

# Increase connection pool (in config)
# internal/db/db.go: MaxConns: 10 -> 20

# Check memory usage
docker stats
```

### Slow Queries

**Symptoms**: P95 >5s consistently

**Investigate:**
```sql
-- Enable query logging
ALTER DATABASE cryptofunk SET log_min_duration_statement = 1000;

-- Check slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Analyze specific query
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT ... [slow query here]
```

### Index Not Being Used

**Symptoms**: Sequential scans in EXPLAIN output

**Check:**
```sql
-- Update statistics
ANALYZE llm_decisions;

-- Verify index exists
\d llm_decisions

-- Force index usage (for testing)
SET enable_seqscan = off;
```

## Integration with CI/CD

Add load tests to your CI pipeline:

```yaml
# .github/workflows/load-test.yml
name: Load Tests
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4

      - name: Start services
        run: docker-compose up -d

      - name: Wait for services
        run: ./scripts/wait-for-services.sh

      - name: Run load tests
        run: go test -v -timeout 30m ./tests/load/

      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: load-test-results
          path: tests/load/*.log
```

## Further Reading

- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [PostgreSQL Performance Tuning](https://wiki.postgresql.org/wiki/Performance_Optimization)
- [Go Benchmark Best Practices](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [Hey Load Testing Tool](https://github.com/rakyll/hey)

## Contributing

When adding new load tests:

1. Follow the existing naming convention: `Test*Concurrency` or `Benchmark*`
2. Include metrics collection (latency, throughput, error rate)
3. Set reasonable assertions (not too strict, not too lenient)
4. Document expected performance characteristics
5. Update this README with new tests
