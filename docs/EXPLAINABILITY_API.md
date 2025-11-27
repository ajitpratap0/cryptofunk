# Explainability Dashboard API

## Overview

The Explainability Dashboard API provides endpoints for viewing, analyzing, and searching LLM trading decisions in the CryptoFunk system. This API enables transparency into AI-driven trading decisions, including prompts, responses, confidence scores, outcomes, and performance metrics.

The API leverages PostgreSQL full-text search and pgvector for semantic similarity matching, providing both text-based and vector-based search capabilities.

## Base URL

- **Development**: `http://localhost:8080`
- **Production**: Configure via `API_HOST` and `API_PORT` environment variables

## Authentication

Authentication is **optional by default** (disabled in development) but can be enabled via configuration.

### Enabling Authentication

To enable API key authentication:

1. Run migration `009_api_keys.sql` to create the api_keys table
2. Set `api.auth.enabled: true` in `configs/config.yaml`
3. Create API keys using PostgreSQL function: `SELECT create_api_key('username', 'key_name', '["*"]'::jsonb);`

### Authentication Headers

When authentication is enabled, provide your API key using one of:

- **X-API-Key header** (recommended):
  ```
  X-API-Key: your-api-key-here
  ```

- **Authorization Bearer token**:
  ```
  Authorization: Bearer your-api-key-here
  ```

### Authentication Behavior

- **Anonymous Access**: Allowed when auth is disabled (default for development)
- **Authenticated Access**: Provides enhanced audit logging and user tracking
- **Failed Auth**: Returns `401 Unauthorized` if invalid key is provided

## Rate Limiting

All endpoints are rate-limited to prevent abuse:

| Endpoint Type | Limit | Window | Applies To |
|--------------|-------|--------|-----------|
| Global | 100 requests | 1 minute | All endpoints |
| Read | 60 requests | 1 minute | List/Get operations |
| Search | 20 requests | 1 minute | Search/Similar operations |

### Rate Limit Headers

All responses include rate limit information:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1735302000
```

When rate limit is exceeded:

```
HTTP/1.1 429 Too Many Requests
Retry-After: 30

{
  "error": "rate limit exceeded",
  "message": "Maximum 60 requests per 1m0s allowed",
  "retry_after": 30
}
```

## Endpoints

### 1. List Decisions

Retrieve LLM decisions with optional filtering and pagination.

**Endpoint**: `GET /api/v1/decisions`

**Query Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Filter by trading symbol (e.g., `BTC/USDT`) |
| `decision_type` | string | No | Filter by decision type (e.g., `signal`, `risk_approval`) |
| `outcome` | string | No | Filter by outcome (`SUCCESS`, `FAILURE`, `PENDING`) |
| `model` | string | No | Filter by LLM model (e.g., `claude-sonnet-4`, `gpt-4-turbo`) |
| `from_date` | string | No | Filter from date (RFC3339 format: `2025-01-01T00:00:00Z`) |
| `to_date` | string | No | Filter to date (RFC3339 format) |
| `limit` | integer | No | Max results (default: 50, max: 500) |
| `offset` | integer | No | Pagination offset (default: 0) |

**Example Request**:

```bash
curl -X GET "http://localhost:8080/api/v1/decisions?limit=10&symbol=BTC/USDT&outcome=SUCCESS" \
  -H "X-API-Key: your-api-key-here"
```

**Example Response**:

```json
{
  "decisions": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "session_id": "789e4567-e89b-12d3-a456-426614174111",
      "decision_type": "signal",
      "symbol": "BTC/USDT",
      "agent_name": "Technical Agent",
      "prompt": "Analyze BTC/USDT technical indicators...",
      "response": "Based on RSI(14)=35.2 and MACD crossover...",
      "model": "claude-sonnet-4-5-20250929",
      "tokens_used": 1245,
      "latency_ms": 850,
      "confidence": 0.85,
      "outcome": "SUCCESS",
      "pnl": 125.50,
      "created_at": "2025-01-15T10:30:00Z"
    }
  ],
  "count": 1,
  "filter": {
    "symbol": "BTC/USDT",
    "outcome": "SUCCESS",
    "limit": 10,
    "offset": 0
  }
}
```

**Status Codes**:

- `200 OK`: Success
- `400 Bad Request`: Invalid parameters (e.g., malformed date)
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Database error

---

### 2. Get Single Decision

Retrieve detailed information about a specific decision.

**Endpoint**: `GET /api/v1/decisions/:id`

**Path Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | UUID | Yes | Decision ID |

**Example Request**:

```bash
curl -X GET "http://localhost:8080/api/v1/decisions/123e4567-e89b-12d3-a456-426614174000" \
  -H "X-API-Key: your-api-key-here"
```

**Example Response**:

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "session_id": "789e4567-e89b-12d3-a456-426614174111",
  "decision_type": "signal",
  "symbol": "BTC/USDT",
  "agent_name": "Technical Agent",
  "prompt": "Analyze BTC/USDT with the following market context:\n\nPrice: $42,150.00\nRSI(14): 35.2\nMACD: Bullish crossover detected\nVolume: Above 20-day MA\nBollinger Bands: Price near lower band\n\nProvide trading signal with confidence and reasoning.",
  "response": "**Signal: BUY**\n\nConfidence: 0.85\n\nReasoning:\n1. RSI(14) at 35.2 indicates oversold conditions\n2. MACD bullish crossover suggests momentum shift\n3. Price near lower Bollinger Band presents good entry\n4. Volume confirmation supports move\n\nRecommended entry: $42,100-$42,200\nStop loss: $41,500 (-1.5%)\nTake profit: $43,800 (+4.0%)",
  "model": "claude-sonnet-4-5-20250929",
  "tokens_used": 1245,
  "latency_ms": 850,
  "confidence": 0.85,
  "outcome": "SUCCESS",
  "pnl": 125.50,
  "created_at": "2025-01-15T10:30:00Z"
}
```

**Status Codes**:

- `200 OK`: Success
- `400 Bad Request`: Invalid UUID format
- `404 Not Found`: Decision not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Database error

---

### 3. Get Similar Decisions

Find decisions with similar market context using vector similarity search (pgvector).

**Endpoint**: `GET /api/v1/decisions/:id/similar`

**Path Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | UUID | Yes | Decision ID to find similar decisions for |

**Query Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | integer | No | Number of similar decisions to return (default: 10, max: 50) |

**Example Request**:

```bash
curl -X GET "http://localhost:8080/api/v1/decisions/123e4567-e89b-12d3-a456-426614174000/similar?limit=5" \
  -H "X-API-Key: your-api-key-here"
```

**Example Response**:

```json
{
  "decision_id": "123e4567-e89b-12d3-a456-426614174000",
  "similar": [
    {
      "id": "987e4567-e89b-12d3-a456-426614174222",
      "decision_type": "signal",
      "symbol": "BTC/USDT",
      "agent_name": "Technical Agent",
      "prompt": "Analyze BTC/USDT technical indicators (RSI: 36.8, MACD: bullish)...",
      "response": "Signal: BUY with 0.82 confidence...",
      "model": "claude-sonnet-4-5-20250929",
      "confidence": 0.82,
      "outcome": "SUCCESS",
      "pnl": 98.25,
      "created_at": "2025-01-14T15:20:00Z"
    }
  ],
  "count": 5
}
```

**Status Codes**:

- `200 OK`: Success
- `400 Bad Request`: Invalid UUID or limit parameter
- `429 Too Many Requests`: Rate limit exceeded (search limits apply)
- `500 Internal Server Error`: Database error

**Notes**:

- Uses pgvector cosine similarity on prompt embeddings (1536-dimensional vectors)
- Only returns decisions that have embeddings (generated when decision is created)
- Results are ordered by similarity (most similar first)
- This is a computationally expensive operation, hence stricter rate limits

---

### 4. Get Statistics

Retrieve aggregated decision statistics with optional filtering.

**Endpoint**: `GET /api/v1/decisions/stats`

**Query Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Filter by trading symbol |
| `decision_type` | string | No | Filter by decision type |
| `from_date` | string | No | Filter from date (RFC3339 format) |
| `to_date` | string | No | Filter to date (RFC3339 format) |

**Example Request**:

```bash
curl -X GET "http://localhost:8080/api/v1/decisions/stats?symbol=BTC/USDT&from_date=2025-01-01T00:00:00Z" \
  -H "X-API-Key: your-api-key-here"
```

**Example Response**:

```json
{
  "total_decisions": 1523,
  "by_type": {
    "signal": 892,
    "risk_approval": 425,
    "market_analysis": 206
  },
  "by_outcome": {
    "SUCCESS": 945,
    "FAILURE": 312,
    "PENDING": 266
  },
  "by_model": {
    "claude-sonnet-4-5-20250929": 1245,
    "gpt-4-turbo": 278
  },
  "avg_confidence": 0.78,
  "avg_latency_ms": 723.5,
  "avg_tokens_used": 1156.8,
  "success_rate": 0.75,
  "total_pnl": 12543.75,
  "avg_pnl": 13.28
}
```

**Status Codes**:

- `200 OK`: Success
- `400 Bad Request`: Invalid date format
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Database error

---

### 5. Search Decisions

Search decisions using text or semantic search.

**Endpoint**: `POST /api/v1/decisions/search`

**Request Body**:

```json
{
  "query": "oversold RSI bullish crossover",
  "embedding": [0.123, 0.456, ...],
  "symbol": "BTC/USDT",
  "from_date": "2025-01-01T00:00:00Z",
  "to_date": "2025-01-31T23:59:59Z",
  "limit": 20
}
```

**Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Conditional | Text search query (max 500 chars). Required if `embedding` not provided |
| `embedding` | float[] | Conditional | Pre-computed embedding vector (1536 dimensions, OpenAI format). Required if `query` not provided |
| `symbol` | string | No | Filter by trading symbol |
| `from_date` | string | No | Filter from date (RFC3339 format) |
| `to_date` | string | No | Filter to date (RFC3339 format) |
| `limit` | integer | No | Max results (default: 20, max: 100) |

**Search Behavior**:

- If `embedding` is provided (1536-dim vector): Uses **semantic search** via pgvector
- If only `query` is provided: Uses **PostgreSQL full-text search**
- If full-text search fails or returns no results: Falls back to **ILIKE pattern matching**

**Example Request (Text Search)**:

```bash
curl -X POST "http://localhost:8080/api/v1/decisions/search" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "query": "oversold RSI bullish MACD",
    "symbol": "BTC/USDT",
    "limit": 10
  }'
```

**Example Request (Semantic Search)**:

```bash
curl -X POST "http://localhost:8080/api/v1/decisions/search" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "embedding": [0.123, 0.456, -0.789, ...],
    "limit": 10
  }'
```

**Example Response**:

```json
{
  "results": [
    {
      "decision": {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "decision_type": "signal",
        "symbol": "BTC/USDT",
        "agent_name": "Technical Agent",
        "prompt": "Analyze BTC/USDT: RSI(14)=35.2, MACD bullish crossover...",
        "response": "Signal: BUY with 0.85 confidence...",
        "model": "claude-sonnet-4-5-20250929",
        "confidence": 0.85,
        "outcome": "SUCCESS",
        "pnl": 125.50,
        "created_at": "2025-01-15T10:30:00Z"
      },
      "score": 0.92
    }
  ],
  "count": 1,
  "search_type": "semantic",
  "query": ""
}
```

**Status Codes**:

- `200 OK`: Success
- `400 Bad Request`: Invalid request (missing query/embedding, invalid embedding dimension, query too long)
- `429 Too Many Requests`: Rate limit exceeded (search limits apply)
- `500 Internal Server Error`: Database or search error

**Notes**:

- Semantic search requires embeddings to be pre-computed and stored in the database
- Text search uses PostgreSQL's full-text search with English language stemming
- Relevance scores range from 0-1 (higher = more relevant)
- Search is case-insensitive
- This is a computationally expensive operation, hence stricter rate limits (20 req/min)

---

## Data Models

### Decision Object

```json
{
  "id": "UUID",
  "session_id": "UUID | null",
  "decision_type": "string",
  "symbol": "string",
  "agent_name": "string | null",
  "prompt": "string",
  "response": "string",
  "model": "string",
  "tokens_used": "integer | null",
  "latency_ms": "integer | null",
  "confidence": "float | null",
  "outcome": "string | null",
  "pnl": "float | null",
  "created_at": "RFC3339 timestamp"
}
```

### Decision Filter

```json
{
  "symbol": "string",
  "decision_type": "string",
  "outcome": "string",
  "model": "string",
  "from_date": "RFC3339 timestamp",
  "to_date": "RFC3339 timestamp",
  "limit": "integer",
  "offset": "integer"
}
```

### Decision Stats

```json
{
  "total_decisions": "integer",
  "by_type": {
    "type_name": "integer"
  },
  "by_outcome": {
    "outcome_name": "integer"
  },
  "by_model": {
    "model_name": "integer"
  },
  "avg_confidence": "float",
  "avg_latency_ms": "float",
  "avg_tokens_used": "float",
  "success_rate": "float",
  "total_pnl": "float",
  "avg_pnl": "float"
}
```

### Search Result

```json
{
  "decision": "Decision object",
  "score": "float (0-1)"
}
```

---

## Error Responses

All error responses follow a consistent format:

```json
{
  "error": "error_type",
  "message": "Human-readable error message",
  "details": "Optional detailed error information"
}
```

### Common Error Codes

| Status Code | Error Type | Description |
|------------|------------|-------------|
| `400` | Bad Request | Invalid parameters, malformed request |
| `401` | Unauthorized | Missing or invalid API key (when auth enabled) |
| `403` | Forbidden | HTTPS required, insufficient permissions |
| `404` | Not Found | Decision not found |
| `429` | Too Many Requests | Rate limit exceeded |
| `500` | Internal Server Error | Database error, server error |

### Example Error Response

```json
{
  "error": "Invalid from_date format, must be RFC3339"
}
```

---

## Best Practices

### Pagination

For large result sets, use pagination to avoid timeouts:

```bash
# First page
curl "http://localhost:8080/api/v1/decisions?limit=50&offset=0"

# Second page
curl "http://localhost:8080/api/v1/decisions?limit=50&offset=50"

# Third page
curl "http://localhost:8080/api/v1/decisions?limit=50&offset=100"
```

### Date Filtering

Always use RFC3339 format for dates:

```bash
# Correct
curl "http://localhost:8080/api/v1/decisions?from_date=2025-01-01T00:00:00Z"

# Also correct (with timezone)
curl "http://localhost:8080/api/v1/decisions?from_date=2025-01-01T00:00:00-05:00"
```

### Search Performance

For optimal search performance:

1. **Text Search**: Keep queries under 100 characters for best performance
2. **Semantic Search**: Pre-compute embeddings using OpenAI's text-embedding-ada-002 model
3. **Filtering**: Combine search with date/symbol filters to narrow results
4. **Caching**: Cache frequently accessed decisions on the client side

### Rate Limit Handling

Handle rate limits gracefully:

```python
import requests
import time

def make_request_with_retry(url, max_retries=3):
    for attempt in range(max_retries):
        response = requests.get(url)

        if response.status_code == 429:
            retry_after = int(response.headers.get('Retry-After', 60))
            print(f"Rate limited. Retrying after {retry_after}s...")
            time.sleep(retry_after)
            continue

        return response

    raise Exception("Max retries exceeded")
```

---

## Examples

### Example 1: Find Recent Failed Decisions

```bash
curl -X GET "http://localhost:8080/api/v1/decisions?outcome=FAILURE&limit=10" \
  -H "X-API-Key: your-api-key-here"
```

### Example 2: Analyze Performance by Model

```bash
# Get overall stats
curl -X GET "http://localhost:8080/api/v1/decisions/stats" \
  -H "X-API-Key: your-api-key-here"

# Get stats for specific model
curl -X GET "http://localhost:8080/api/v1/decisions?model=claude-sonnet-4-5-20250929&limit=100" \
  -H "X-API-Key: your-api-key-here"
```

### Example 3: Find Similar Successful Decisions

```bash
# First, get a successful decision ID
DECISION_ID="123e4567-e89b-12d3-a456-426614174000"

# Find similar decisions
curl -X GET "http://localhost:8080/api/v1/decisions/${DECISION_ID}/similar?limit=10" \
  -H "X-API-Key: your-api-key-here"
```

### Example 4: Search for Specific Strategy Patterns

```bash
curl -X POST "http://localhost:8080/api/v1/decisions/search" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "query": "mean reversion strategy support resistance",
    "symbol": "BTC/USDT",
    "from_date": "2025-01-01T00:00:00Z",
    "limit": 20
  }'
```

### Example 5: Get Trading Performance for Date Range

```bash
curl -X GET "http://localhost:8080/api/v1/decisions/stats?from_date=2025-01-01T00:00:00Z&to_date=2025-01-31T23:59:59Z" \
  -H "X-API-Key: your-api-key-here"
```

---

## Python Client Example

```python
import requests
from typing import Optional, List, Dict

class CryptoFunkClient:
    def __init__(self, base_url: str = "http://localhost:8080", api_key: Optional[str] = None):
        self.base_url = base_url
        self.headers = {}
        if api_key:
            self.headers["X-API-Key"] = api_key

    def list_decisions(
        self,
        symbol: Optional[str] = None,
        outcome: Optional[str] = None,
        limit: int = 50,
        offset: int = 0
    ) -> Dict:
        """List decisions with filtering."""
        params = {"limit": limit, "offset": offset}
        if symbol:
            params["symbol"] = symbol
        if outcome:
            params["outcome"] = outcome

        response = requests.get(
            f"{self.base_url}/api/v1/decisions",
            params=params,
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

    def get_decision(self, decision_id: str) -> Dict:
        """Get single decision by ID."""
        response = requests.get(
            f"{self.base_url}/api/v1/decisions/{decision_id}",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

    def search_decisions(self, query: str, limit: int = 20) -> Dict:
        """Search decisions by text query."""
        response = requests.post(
            f"{self.base_url}/api/v1/decisions/search",
            json={"query": query, "limit": limit},
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

    def get_stats(self, symbol: Optional[str] = None) -> Dict:
        """Get decision statistics."""
        params = {}
        if symbol:
            params["symbol"] = symbol

        response = requests.get(
            f"{self.base_url}/api/v1/decisions/stats",
            params=params,
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

# Usage
client = CryptoFunkClient(api_key="your-api-key")

# Get recent decisions
decisions = client.list_decisions(symbol="BTC/USDT", limit=10)
print(f"Found {decisions['count']} decisions")

# Search for patterns
results = client.search_decisions("oversold RSI bullish")
for result in results["results"]:
    print(f"Score: {result['score']:.2f} - {result['decision']['id']}")

# Get performance stats
stats = client.get_stats(symbol="BTC/USDT")
print(f"Success rate: {stats['success_rate']:.2%}")
print(f"Average P&L: ${stats['avg_pnl']:.2f}")
```

---

## Configuration

### Enable Authentication

Edit `configs/config.yaml`:

```yaml
api:
  auth:
    enabled: true
    header_name: "X-API-Key"
    require_https: true
```

### Adjust Rate Limits

Rate limits are defined in `cmd/api/middleware.go` via `DefaultRateLimiterConfig()`:

```go
// Read endpoints: 60 requests per minute (allow monitoring)
ReadMaxRequests: 60,
ReadWindow:      time.Minute,

// Search endpoints: 20 requests per minute (vector search is expensive)
SearchMaxRequests: 20,
SearchWindow:      time.Minute,
```

---

## Additional Resources

- **Database Schema**: See `migrations/004_llm_decisions_enhancement.sql`
- **Audit Logging**: All decision accesses are logged when authentication is enabled
- **Monitoring**: Metrics available at `/metrics` endpoint (Prometheus format)
- **WebSocket Updates**: Real-time decision broadcasts via `/api/v1/ws`

---

## Support

For issues or questions:

1. Check logs: `docker-compose logs -f api`
2. Verify database: `task db-status`
3. Test health: `curl http://localhost:8080/health`
4. Review troubleshooting: `docs/TROUBLESHOOTING.md`
