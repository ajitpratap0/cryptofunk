# Decision Explainability API

This document describes the Decision Explainability API endpoints for CryptoFunk. These endpoints provide transparency and auditability for all LLM-powered trading decisions.

## Overview

The Decision Explainability API allows you to:
- Query historical LLM decisions with rich filtering
- View detailed decision context including prompts, responses, and outcomes
- Find similar market situations using vector similarity search
- Analyze decision performance with aggregated statistics

All endpoints are available under `/api/v1/decisions`.

---

## Endpoints

### 1. List Decisions

List LLM decisions with optional filtering and pagination.

**Endpoint:** `GET /api/v1/decisions`

**Query Parameters:**

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| `symbol` | string | No | Filter by trading symbol | `BTC/USDT` |
| `decision_type` | string | No | Filter by decision type | `signal`, `risk_approval`, `position_sizing` |
| `outcome` | string | No | Filter by outcome | `SUCCESS`, `FAILURE`, `PENDING` |
| `model` | string | No | Filter by LLM model | `claude-sonnet-4`, `gpt-4-turbo` |
| `from_date` | string | No | Filter from date (RFC3339) | `2024-01-01T00:00:00Z` |
| `to_date` | string | No | Filter to date (RFC3339) | `2024-12-31T23:59:59Z` |
| `limit` | integer | No | Max results (default: 50, max: 500) | `100` |
| `offset` | integer | No | Pagination offset (default: 0) | `50` |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/decisions?symbol=BTC/USDT&decision_type=signal&limit=10"
```

**Example Response:**

```json
{
  "decisions": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "session_id": "650e8400-e29b-41d4-a716-446655440001",
      "decision_type": "signal",
      "symbol": "BTC/USDT",
      "prompt": "Analyze BTC/USDT with RSI=72, MACD bullish crossover, volume spike",
      "response": "{\"action\": \"BUY\", \"confidence\": 0.85, \"reasoning\": \"Strong bullish signals with RSI approaching overbought but MACD confirming momentum\"}",
      "model": "claude-sonnet-4",
      "tokens_used": 450,
      "latency_ms": 1200,
      "confidence": 0.85,
      "outcome": "SUCCESS",
      "pnl": 250.50,
      "created_at": "2024-11-03T10:30:00Z"
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440002",
      "session_id": "650e8400-e29b-41d4-a716-446655440001",
      "decision_type": "signal",
      "symbol": "BTC/USDT",
      "prompt": "Analyze BTC/USDT with RSI=35, MACD bearish, declining volume",
      "response": "{\"action\": \"HOLD\", \"confidence\": 0.65, \"reasoning\": \"Oversold conditions but weak momentum, wait for confirmation\"}",
      "model": "claude-sonnet-4",
      "tokens_used": 380,
      "latency_ms": 950,
      "confidence": 0.65,
      "outcome": "SUCCESS",
      "pnl": 0.00,
      "created_at": "2024-11-03T11:45:00Z"
    }
  ],
  "count": 2,
  "filter": {
    "symbol": "BTC/USDT",
    "decision_type": "signal",
    "outcome": "",
    "model": "",
    "from_date": null,
    "to_date": null,
    "limit": 10,
    "offset": 0
  }
}
```

**Response Fields:**

- `decisions`: Array of decision objects
- `count`: Number of decisions returned
- `filter`: Echo of applied filters

**HTTP Status Codes:**

- `200 OK`: Success
- `500 Internal Server Error`: Database error

---

### 2. Get Decision Details

Retrieve detailed information for a specific decision by ID.

**Endpoint:** `GET /api/v1/decisions/:id`

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | UUID | Yes | Decision ID |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000"
```

**Example Response:**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "session_id": "650e8400-e29b-41d4-a716-446655440001",
  "decision_type": "signal",
  "symbol": "BTC/USDT",
  "prompt": "Analyze BTC/USDT with RSI=72, MACD bullish crossover, volume spike. Market context: Strong buying pressure, order book depth 2.5:1 bid/ask ratio.",
  "response": "{\"action\": \"BUY\", \"confidence\": 0.85, \"reasoning\": \"Strong bullish signals across multiple indicators. RSI approaching overbought but MACD confirming momentum. Volume spike suggests institutional interest. Recommend entry at current price with tight stop-loss.\", \"entry_price\": 42500, \"stop_loss\": 42000, \"take_profit\": 44000}",
  "model": "claude-sonnet-4",
  "tokens_used": 450,
  "latency_ms": 1200,
  "confidence": 0.85,
  "outcome": "SUCCESS",
  "pnl": 250.50,
  "created_at": "2024-11-03T10:30:00Z"
}
```

**HTTP Status Codes:**

- `200 OK`: Decision found
- `400 Bad Request`: Invalid UUID format
- `404 Not Found`: Decision not found
- `500 Internal Server Error`: Database error

---

### 3. Find Similar Decisions

Find decisions with similar market context using vector similarity search.

**Endpoint:** `GET /api/v1/decisions/:id/similar`

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | UUID | Yes | Reference decision ID |

**Query Parameters:**

| Parameter | Type | Required | Description | Default | Max |
|-----------|------|----------|-------------|---------|-----|
| `limit` | integer | No | Number of similar decisions | 10 | 50 |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000/similar?limit=5"
```

**Example Response:**

```json
{
  "decision_id": "550e8400-e29b-41d4-a716-446655440000",
  "similar": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440003",
      "session_id": "650e8400-e29b-41d4-a716-446655440001",
      "decision_type": "signal",
      "symbol": "BTC/USDT",
      "prompt": "Analyze BTC/USDT with RSI=70, MACD bullish, volume increasing",
      "response": "{\"action\": \"BUY\", \"confidence\": 0.82, \"reasoning\": \"Similar bullish setup with strong momentum\"}",
      "model": "claude-sonnet-4",
      "tokens_used": 420,
      "latency_ms": 1100,
      "confidence": 0.82,
      "outcome": "SUCCESS",
      "pnl": 180.25,
      "created_at": "2024-11-02T14:20:00Z"
    },
    {
      "id": "880e8400-e29b-41d4-a716-446655440004",
      "session_id": "650e8400-e29b-41d4-a716-446655440002",
      "decision_type": "signal",
      "symbol": "ETH/USDT",
      "prompt": "Analyze ETH/USDT with RSI=71, MACD bullish crossover, high volume",
      "response": "{\"action\": \"BUY\", \"confidence\": 0.80, \"reasoning\": \"Strong bullish signals, similar to BTC setup\"}",
      "model": "gpt-4-turbo",
      "tokens_used": 380,
      "latency_ms": 1050,
      "confidence": 0.80,
      "outcome": "SUCCESS",
      "pnl": 320.75,
      "created_at": "2024-11-01T09:15:00Z"
    }
  ],
  "count": 2
}
```

**How It Works:**

Uses pgvector cosine similarity search on prompt embeddings (1536-dimensional OpenAI embeddings) to find decisions made in similar market contexts. This helps answer questions like:
- "What happened in similar market conditions?"
- "How did the model perform in comparable situations?"
- "What was the reasoning in similar setups?"

**HTTP Status Codes:**

- `200 OK`: Success (returns empty array if no similar decisions found)
- `400 Bad Request`: Invalid UUID format
- `500 Internal Server Error`: Database error

---

### 4. Get Decision Statistics

Get aggregated statistics across decisions with optional filtering.

**Endpoint:** `GET /api/v1/decisions/stats`

**Query Parameters:**

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| `symbol` | string | No | Filter by symbol | `BTC/USDT` |
| `decision_type` | string | No | Filter by decision type | `signal` |
| `from_date` | string | No | Filter from date (RFC3339) | `2024-01-01T00:00:00Z` |
| `to_date` | string | No | Filter to date (RFC3339) | `2024-12-31T23:59:59Z` |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/decisions/stats?symbol=BTC/USDT&decision_type=signal"
```

**Example Response:**

```json
{
  "total_decisions": 1250,
  "by_type": {
    "signal": 850,
    "risk_approval": 300,
    "position_sizing": 100
  },
  "by_outcome": {
    "SUCCESS": 875,
    "FAILURE": 250,
    "PENDING": 125
  },
  "by_model": {
    "claude-sonnet-4": 700,
    "gpt-4-turbo": 400,
    "gpt-3.5-turbo": 150
  },
  "avg_confidence": 0.7524,
  "avg_latency_ms": 1150.5,
  "avg_tokens_used": 425.8,
  "success_rate": 0.7777,
  "total_pnl": 12500.75,
  "avg_pnl": 14.29
}
```

**Response Fields:**

- `total_decisions`: Total number of decisions matching filters
- `by_type`: Breakdown by decision type
- `by_outcome`: Breakdown by outcome (SUCCESS/FAILURE/PENDING)
- `by_model`: Breakdown by LLM model used
- `avg_confidence`: Average confidence score (0-1)
- `avg_latency_ms`: Average response time in milliseconds
- `avg_tokens_used`: Average tokens consumed per decision
- `success_rate`: Ratio of successful outcomes (0-1)
- `total_pnl`: Total profit/loss across all decisions
- `avg_pnl`: Average profit/loss per decision

**HTTP Status Codes:**

- `200 OK`: Success
- `500 Internal Server Error`: Database error

---

## Decision Types

Common decision types tracked in the system:

| Type | Description | Example Use |
|------|-------------|-------------|
| `signal` | Trading signal generation | Buy/Sell/Hold recommendations |
| `risk_approval` | Risk management decisions | Position size approval, risk checks |
| `position_sizing` | Position size calculations | Kelly criterion, portfolio allocation |
| `market_analysis` | Market condition analysis | Trend identification, volatility assessment |

---

## Outcome Values

Possible outcome values:

| Outcome | Description |
|---------|-------------|
| `SUCCESS` | Decision led to profitable result |
| `FAILURE` | Decision led to loss |
| `PENDING` | Decision outcome not yet determined |
| `CANCELED` | Decision was canceled before execution |

---

## Model Names

Common LLM models used:

| Model | Description |
|-------|-------------|
| `claude-sonnet-4` | Anthropic Claude Sonnet 4 |
| `claude-sonnet-4-5-20250929` | Anthropic Claude Sonnet 4.5 |
| `gpt-4-turbo` | OpenAI GPT-4 Turbo |
| `gpt-4o` | OpenAI GPT-4o |
| `gpt-3.5-turbo` | OpenAI GPT-3.5 Turbo |

---

## Use Cases

### 1. Audit Trail

Query all decisions for a trading session:

```bash
curl "http://localhost:8080/api/v1/decisions?from_date=2024-11-03T00:00:00Z&to_date=2024-11-03T23:59:59Z"
```

### 2. Model Performance Comparison

Get statistics for each model:

```bash
# Claude performance
curl "http://localhost:8080/api/v1/decisions/stats?model=claude-sonnet-4"

# GPT-4 performance
curl "http://localhost:8080/api/v1/decisions/stats?model=gpt-4-turbo"
```

### 3. Decision Explainability

For any decision, view the complete context:

```bash
# Get decision details
curl "http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000"

# Find similar historical situations
curl "http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000/similar"
```

### 4. Performance Analysis

Analyze performance by symbol:

```bash
curl "http://localhost:8080/api/v1/decisions/stats?symbol=BTC/USDT"
```

### 5. Debugging Failed Decisions

Find all failed decisions to analyze:

```bash
curl "http://localhost:8080/api/v1/decisions?outcome=FAILURE&limit=50"
```

---

## Integration with Frontend

### Example: Display Decision with Explanation

```javascript
async function showDecisionExplanation(decisionId) {
  // Get decision details
  const response = await fetch(`/api/v1/decisions/${decisionId}`);
  const decision = await response.json();

  // Parse LLM response
  const llmResponse = JSON.parse(decision.response);

  // Display
  console.log('Decision:', llmResponse.action);
  console.log('Confidence:', decision.confidence);
  console.log('Reasoning:', llmResponse.reasoning);
  console.log('Model:', decision.model);
  console.log('Outcome:', decision.outcome);
  console.log('P&L:', decision.pnl);
}
```

### Example: Find Similar Past Situations

```javascript
async function findSimilarSituations(decisionId) {
  const response = await fetch(`/api/v1/decisions/${decisionId}/similar?limit=10`);
  const data = await response.json();

  console.log(`Found ${data.count} similar situations:`);
  data.similar.forEach(sim => {
    const llmResponse = JSON.parse(sim.response);
    console.log(`- ${sim.symbol}: ${llmResponse.action} (confidence: ${sim.confidence}, outcome: ${sim.outcome})`);
  });
}
```

### Example: Display Model Performance Dashboard

```javascript
async function showModelStats(model) {
  const response = await fetch(`/api/v1/decisions/stats?model=${model}`);
  const stats = await response.json();

  console.log(`${model} Performance:`);
  console.log(`- Total Decisions: ${stats.total_decisions}`);
  console.log(`- Success Rate: ${(stats.success_rate * 100).toFixed(2)}%`);
  console.log(`- Avg Confidence: ${stats.avg_confidence.toFixed(4)}`);
  console.log(`- Avg Latency: ${stats.avg_latency_ms.toFixed(0)}ms`);
  console.log(`- Total P&L: $${stats.total_pnl.toFixed(2)}`);
  console.log(`- Avg P&L: $${stats.avg_pnl.toFixed(2)}`);
}
```

---

## Database Schema

Decisions are stored in the `llm_decisions` table:

```sql
CREATE TABLE llm_decisions (
    id UUID PRIMARY KEY,
    session_id UUID REFERENCES trading_sessions(id),
    decision_type VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    prompt TEXT NOT NULL,
    prompt_embedding vector(1536),  -- For semantic search
    response TEXT NOT NULL,
    model VARCHAR(100) NOT NULL,
    tokens_used INTEGER,
    latency_ms INTEGER,
    confidence DECIMAL(5,4),
    outcome VARCHAR(20),
    pnl DECIMAL(20,8),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

**Indexes:**
- `idx_llm_decisions_session`: Session + created_at (for session queries)
- `idx_llm_decisions_symbol`: Symbol + created_at (for symbol queries)
- `idx_llm_decisions_type`: Decision type + created_at (for type queries)
- `idx_llm_decisions_outcome`: Outcome (partial index for outcome queries)
- `idx_llm_decisions_embedding`: IVFFlat vector index for similarity search

---

## Performance Considerations

1. **Pagination**: Use `limit` and `offset` for large result sets
2. **Filtering**: Apply filters to reduce result size
3. **Vector Search**: Similarity search is computationally expensive, keep `limit` reasonable (≤50)
4. **Caching**: Consider caching statistics queries at the application layer
5. **Indexes**: All common query patterns are indexed for fast retrieval

---

## Error Handling

All endpoints follow consistent error response format:

```json
{
  "error": "Human-readable error message",
  "details": "Technical error details (in development mode)"
}
```

**Common Error Codes:**

- `400 Bad Request`: Invalid parameters (e.g., malformed UUID)
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Database or server error

---

## Testing

### Manual Testing with curl

```bash
# List recent decisions
curl http://localhost:8080/api/v1/decisions?limit=5

# Get specific decision
curl http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000

# Find similar decisions
curl http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000/similar

# Get overall statistics
curl http://localhost:8080/api/v1/decisions/stats
```

### Testing with jq (pretty print)

```bash
curl -s http://localhost:8080/api/v1/decisions | jq .
```

### Integration Tests

See `internal/api/decisions_test.go` for unit and integration tests.

---

## Future Enhancements

Potential future improvements:

1. **Real-time Updates**: WebSocket endpoint for streaming decisions
2. **Advanced Analytics**: Time-series aggregations, rolling statistics
3. **Comparison Tool**: Side-by-side model comparison
4. **Export**: CSV/Excel export for offline analysis
5. **Annotations**: Add user notes/tags to decisions
6. **Replay**: Replay decision context to re-evaluate with current model

---

**Version**: 1.0
**Last Updated**: 2025-11-03
**Implemented In**: Phase 9 LLM Integration (T190)
