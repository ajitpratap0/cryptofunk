# Load Tests - Quick Start Guide

Quick reference for running load tests on CryptoFunk vector search endpoints.

## Prerequisites

```bash
# 1. Start services
task docker-up

# 2. Run migrations
task db-migrate

# 3. Start API
task run-api
```

## Run Tests

### Option 1: Go Tests (Recommended)

```bash
# All tests
go test -v ./tests/load/

# Benchmarks only
go test -v -bench=. ./tests/load/

# Concurrency tests only
go test -v -run=Concurrency ./tests/load/

# Stress test
go test -v -run=Stress ./tests/load/
```

### Option 2: Shell Script

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Run tests
./scripts/load-test-vector-search.sh
```

## Expected Results

### Good Performance
- Average latency: <1s
- P95 latency: <3s
- P99 latency: <5s
- Success rate: >98%
- Throughput: >15 req/s

### Warning Signs
- Average latency: 1-2s
- P95 latency: 3-5s
- Success rate: 95-98%
- Throughput: 10-15 req/s

### Critical Issues
- Average latency: >2s
- P95 latency: >5s
- Success rate: <95%
- Throughput: <10 req/s

## Troubleshooting

### High Latency
```sql
-- Update statistics
ANALYZE llm_decisions;

-- Check slow queries
SELECT query, mean_exec_time FROM pg_stat_statements
WHERE query LIKE '%llm_decisions%' ORDER BY mean_exec_time DESC LIMIT 10;
```

### High Error Rate
```sql
-- Check connections
SELECT count(*) FROM pg_stat_activity WHERE datname='cryptofunk';

-- Check locks
SELECT * FROM pg_locks WHERE NOT granted;
```

### Index Issues
```sql
-- Check index usage
SELECT indexname, idx_scan FROM pg_stat_user_indexes
WHERE tablename = 'llm_decisions';

-- Rebuild if needed
REINDEX INDEX idx_llm_decisions_embedding;
```

## More Information

- Full guide: `tests/load/README.md`
- Technical report: `docs/PGVECTOR_INDEX_AND_LOAD_TESTS.md`
- Summary: `docs/VECTOR_SEARCH_VERIFICATION_SUMMARY.md`
