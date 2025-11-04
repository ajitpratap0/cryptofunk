# CryptoFunk API Documentation

**Version:** 1.0.0
**Base URL:** `http://localhost:8080`
**Protocol:** REST + WebSocket

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [REST API Endpoints](#rest-api-endpoints)
  - [Health & Status](#health--status)
  - [Agents](#agents)
  - [Positions](#positions)
  - [Orders](#orders)
  - [Trading Control](#trading-control)
  - [Configuration](#configuration)
  - [Decision Explainability](#decision-explainability)
- [WebSocket API](#websocket-api)
- [Data Models](#data-models)
- [Examples](#examples)

---

## Overview

The CryptoFunk API provides a comprehensive interface for monitoring and controlling the multi-agent trading system. It consists of:

1. **REST API**: Query and manage system resources (positions, orders, agents, etc.)
2. **WebSocket API**: Real-time updates and event streaming

**Features:**
- Real-time position and order tracking
- Agent status monitoring
- Trading session management
- Decision explainability (view LLM reasoning)
- Configuration management
- WebSocket streaming for live updates

**Default Port:** `8080` (configurable via `API_PORT` environment variable)

---

## Authentication

**Current Status:** ⚠️ Not yet implemented (Phase 10, Task T222-T223)

**Planned Authentication Methods:**
- JWT tokens for session-based authentication
- API key authentication for programmatic access
- Role-based access control (RBAC) for multi-user environments

**Headers (Future):**
```http
Authorization: Bearer <JWT_TOKEN>
X-API-Key: <API_KEY>
```

---

## Error Handling

### Error Response Format

All errors follow a consistent JSON format:

```json
{
  "error": "Brief error message",
  "details": "Detailed error information (optional)",
  "field": "Field name for validation errors (optional)"
}
```

### HTTP Status Codes

| Code | Meaning | Usage |
|------|---------|-------|
| 200 | OK | Successful GET request |
| 201 | Created | Successful POST request (resource created) |
| 400 | Bad Request | Invalid request body or parameters |
| 404 | Not Found | Resource not found |
| 500 | Internal Server Error | Server-side error |
| 503 | Service Unavailable | Database or service unavailable |

### Common Error Examples

```json
// Invalid UUID format
{
  "error": "invalid order_id format"
}

// Validation error
{
  "error": "invalid request body",
  "details": "Price is required for limit orders"
}

// Not found
{
  "error": "order not found",
  "order_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

---

## Rate Limiting

**Current Status:** ⚠️ Not yet implemented

**Planned Limits:**
- 100 requests per minute for general endpoints
- 10 requests per minute for write operations (orders, config updates)
- WebSocket: 1 connection per client (multiple tabs require separate connections)

---

## REST API Endpoints

### Health & Status

#### `GET /` - Root Endpoint

Get API information.

**Response:**
```json
{
  "name": "CryptoFunk Trading API",
  "version": "1.0.0",
  "status": "running"
}
```

#### `GET /api/v1/health` - Health Check

Check API and database health. Used by Kubernetes liveness probes.

**Response (Healthy):**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h15m30s"
}
```

**Response (Unhealthy - 503):**
```json
{
  "status": "unhealthy",
  "error": "database connection failed",
  "version": "1.0.0"
}
```

#### `GET /api/v1/status` - System Status

Get detailed system status including all components.

**Response:**
```json
{
  "status": "operational",
  "version": "1.0.0",
  "uptime": "2h15m30s",
  "components": {
    "database": "healthy",
    "api": "healthy",
    "websocket": "healthy"
  },
  "websocket": {
    "connected_clients": 3
  }
}
```

---

### Agents

#### `GET /api/v1/agents` - List All Agents

Get status of all registered trading agents.

**Response:**
```json
{
  "agents": [
    {
      "name": "technical-agent",
      "status": "active",
      "last_seen_at": "2025-01-15T10:30:00Z",
      "is_healthy": true,
      "metadata": {
        "version": "1.0.0",
        "type": "analysis"
      }
    },
    {
      "name": "trend-agent",
      "status": "active",
      "last_seen_at": "2025-01-15T10:30:05Z",
      "is_healthy": true,
      "metadata": {
        "version": "1.0.0",
        "type": "strategy"
      }
    }
  ],
  "count": 2
}
```

#### `GET /api/v1/agents/:name` - Get Agent Details

Get detailed information about a specific agent.

**Path Parameters:**
- `name` (string, required): Agent name (e.g., "technical-agent")

**Response:**
```json
{
  "agent": {
    "name": "technical-agent",
    "status": "active",
    "last_seen_at": "2025-01-15T10:30:00Z",
    "is_healthy": true,
    "metadata": {
      "version": "1.0.0",
      "type": "analysis",
      "signals_generated": 42,
      "last_signal_at": "2025-01-15T10:29:45Z"
    }
  }
}
```

**Errors:**
- `404`: Agent not found

#### `GET /api/v1/agents/:name/status` - Get Agent Status

Get health status of a specific agent (lightweight version).

**Path Parameters:**
- `name` (string, required): Agent name

**Response:**
```json
{
  "name": "technical-agent",
  "status": "active",
  "last_seen_at": "2025-01-15T10:30:00Z",
  "healthy": true
}
```

---

### Positions

#### `GET /api/v1/positions` - List Positions

Get all open positions, optionally filtered by trading session.

**Query Parameters:**
- `session_id` (UUID, optional): Filter by trading session

**Response:**
```json
{
  "positions": [
    {
      "id": "pos-123",
      "session_id": "session-456",
      "symbol": "BTC/USDT",
      "exchange": "PAPER",
      "side": "LONG",
      "entry_price": 42000.00,
      "exit_price": null,
      "quantity": 0.1,
      "entry_time": "2025-01-15T10:00:00Z",
      "exit_time": null,
      "stop_loss": 40000.00,
      "take_profit": 45000.00,
      "realized_pnl": 0.0,
      "unrealized_pnl": 150.50,
      "fees": 4.20,
      "entry_reason": "Technical indicator confluence: RSI oversold + MACD bullish crossover",
      "exit_reason": null
    }
  ],
  "count": 1
}
```

#### `GET /api/v1/positions/:symbol` - Get Position by Symbol

Get the latest position for a specific symbol.

**Path Parameters:**
- `symbol` (string, required): Trading pair symbol (e.g., "BTC/USDT")

**Query Parameters:**
- `session_id` (UUID, optional): Filter by trading session

**Response:**
```json
{
  "position": {
    "id": "pos-123",
    "session_id": "session-456",
    "symbol": "BTC/USDT",
    "exchange": "PAPER",
    "side": "LONG",
    "entry_price": 42000.00,
    "quantity": 0.1,
    "unrealized_pnl": 150.50
  }
}
```

**Errors:**
- `404`: Position not found for symbol

---

### Orders

#### `GET /api/v1/orders` - List Orders

Get orders with optional filtering.

**Query Parameters:**
- `session_id` (UUID, optional): Filter by trading session
- `symbol` (string, optional): Filter by symbol (e.g., "BTC/USDT")
- `status` (string, optional): Filter by status (`NEW`, `FILLED`, `CANCELED`, etc.)

**Response:**
```json
{
  "orders": [
    {
      "id": "order-789",
      "session_id": "session-456",
      "position_id": "pos-123",
      "exchange_order_id": "EXG-12345",
      "symbol": "BTC/USDT",
      "exchange": "PAPER",
      "side": "BUY",
      "type": "MARKET",
      "status": "FILLED",
      "price": null,
      "stop_price": null,
      "quantity": 0.1,
      "executed_quantity": 0.1,
      "executed_quote_quantity": 4200.00,
      "time_in_force": "GTC",
      "placed_at": "2025-01-15T10:00:00Z",
      "filled_at": "2025-01-15T10:00:02Z",
      "canceled_at": null,
      "error_message": null
    }
  ],
  "count": 1
}
```

#### `GET /api/v1/orders/:id` - Get Order Details

Get detailed information about a specific order.

**Path Parameters:**
- `id` (UUID, required): Order ID

**Response:**
```json
{
  "order": {
    "id": "order-789",
    "symbol": "BTC/USDT",
    "side": "BUY",
    "type": "MARKET",
    "status": "FILLED",
    "quantity": 0.1,
    "executed_quantity": 0.1,
    "placed_at": "2025-01-15T10:00:00Z",
    "filled_at": "2025-01-15T10:00:02Z"
  }
}
```

**Errors:**
- `400`: Invalid order ID format
- `404`: Order not found

#### `POST /api/v1/orders` - Place Order

Create a new order (manual trading).

**Request Body:**
```json
{
  "symbol": "BTC/USDT",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 0.1,
  "price": null
}
```

**Request Body (Limit Order):**
```json
{
  "symbol": "BTC/USDT",
  "side": "BUY",
  "type": "LIMIT",
  "quantity": 0.1,
  "price": 41500.00
}
```

**Validations:**
- `symbol` (required): Trading pair
- `side` (required): `BUY` or `SELL`
- `type` (required): `MARKET` or `LIMIT`
- `quantity` (required, >0): Order quantity
- `price` (required for LIMIT orders): Limit price

**Response (201 Created):**
```json
{
  "order": {
    "id": "order-new-123",
    "symbol": "BTC/USDT",
    "side": "BUY",
    "type": "MARKET",
    "status": "NEW",
    "quantity": 0.1,
    "placed_at": "2025-01-15T10:30:00Z"
  },
  "message": "Order created successfully"
}
```

**Errors:**
- `400`: Invalid request body or missing required fields

#### `DELETE /api/v1/orders/:id` - Cancel Order

Cancel a pending order.

**Path Parameters:**
- `id` (UUID, required): Order ID

**Response:**
```json
{
  "order": {
    "id": "order-789",
    "status": "CANCELED",
    "canceled_at": "2025-01-15T10:35:00Z"
  },
  "message": "Order cancelled successfully"
}
```

**Errors:**
- `400`: Order cannot be cancelled (already filled/canceled)
- `404`: Order not found

---

### Trading Control

#### `POST /api/v1/trade/start` - Start Trading

Start a new trading session.

**Request Body:**
```json
{
  "symbol": "BTC/USDT",
  "initial_capital": 10000.00,
  "mode": "PAPER"
}
```

**Fields:**
- `symbol` (required): Trading pair
- `initial_capital` (required, >0): Starting capital in quote currency
- `mode` (optional): `PAPER` (default) or `LIVE`

**Response:**
```json
{
  "message": "Trading started successfully",
  "session_id": "123e4567-e89b-12d3-a456-426614174000",
  "symbol": "BTC/USDT",
  "mode": "PAPER",
  "started_at": "2025-01-15T10:00:00Z"
}
```

#### `POST /api/v1/trade/stop` - Stop Trading

Stop an active trading session.

**Request Body:**
```json
{
  "session_id": "123e4567-e89b-12d3-a456-426614174000",
  "final_capital": 10500.00
}
```

**Fields:**
- `session_id` (required): Trading session ID
- `final_capital` (required, >=0): Final capital after closing all positions

**Response:**
```json
{
  "message": "Trading stopped successfully",
  "session_id": "123e4567-e89b-12d3-a456-426614174000",
  "final_capital": 10500.00,
  "total_pnl": 500.00,
  "total_trades": 15,
  "stopped_at": "2025-01-15T12:00:00Z"
}
```

#### `POST /api/v1/trade/pause` - Pause Trading

Pause an active trading session.

**Request Body:**
```json
{
  "session_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

**Response:**
```json
{
  "message": "Trading paused successfully",
  "session_id": "123e4567-e89b-12d3-a456-426614174000",
  "symbol": "BTC/USDT",
  "note": "Pause logic to be implemented in orchestrator"
}
```

**Note:** ⚠️ Pause functionality is a TODO (Task T257). Currently returns success but does not actually pause the orchestrator.

---

### Configuration

#### `GET /api/v1/config` - Get Configuration

Get current system configuration (sanitized, no API keys or secrets).

**Response:**
```json
{
  "config": {
    "app": {
      "name": "cryptofunk",
      "version": "1.0.0",
      "environment": "development",
      "log_level": "info"
    },
    "trading": {
      "mode": "paper",
      "symbols": ["BTC/USDT", "ETH/USDT"],
      "exchange": "PAPER",
      "initial_capital": 10000.00,
      "max_positions": 3,
      "default_quantity": 0.1
    },
    "risk": {
      "max_position_size": 0.2,
      "max_daily_loss": 0.05,
      "max_drawdown": 0.15,
      "default_stop_loss": 0.02,
      "default_take_profit": 0.05,
      "llm_approval_required": true,
      "min_confidence": 0.7
    },
    "llm": {
      "gateway": "bifrost",
      "primary_model": "claude-sonnet-4",
      "fallback_model": "gpt-4-turbo",
      "temperature": 0.7,
      "max_tokens": 2000,
      "enable_caching": true
    }
  }
}
```

#### `PATCH /api/v1/config` - Update Configuration

Update runtime configuration (in-memory only, resets on restart).

**Updatable Fields:**
- Trading: `trading_mode`, `initial_capital`, `max_positions`
- Risk: `max_position_size`, `max_daily_loss`, `max_drawdown`, `default_stop_loss`, `default_take_profit`, `min_confidence`, `llm_approval_required`
- LLM: `llm_temperature`, `llm_max_tokens`

**Request Body:**
```json
{
  "trading_mode": "paper",
  "max_position_size": 0.15,
  "default_stop_loss": 0.03,
  "llm_temperature": 0.8
}
```

**Response:**
```json
{
  "message": "Configuration updated successfully",
  "updates": {
    "trading_mode": "paper",
    "max_position_size": 0.15,
    "default_stop_loss": 0.03,
    "llm_temperature": 0.8
  },
  "note": "Changes are in-memory only and will reset on server restart"
}
```

**Errors:**
- `400`: Validation failed (invalid values)

---

### Decision Explainability

View LLM decision history and reasoning for transparency and learning.

#### `GET /api/v1/decisions` - List Decisions

List LLM decisions with filtering and pagination.

**Query Parameters:**
- `symbol` (string, optional): Filter by symbol (e.g., "BTC/USDT")
- `decision_type` (string, optional): Filter by type (`signal`, `risk_approval`, etc.)
- `outcome` (string, optional): Filter by outcome (`SUCCESS`, `FAILURE`, `PENDING`)
- `model` (string, optional): Filter by model (e.g., "claude-sonnet-4", "gpt-4-turbo")
- `from_date` (RFC3339, optional): Filter from date (e.g., "2025-01-15T00:00:00Z")
- `to_date` (RFC3339, optional): Filter to date
- `limit` (int, optional): Results limit (default 50, max 500)
- `offset` (int, optional): Pagination offset (default 0)

**Response:**
```json
{
  "decisions": [
    {
      "id": "dec-123",
      "agent_name": "trend-agent",
      "symbol": "BTC/USDT",
      "decision_type": "signal",
      "decision": "BUY",
      "confidence": 0.85,
      "reasoning": "Strong uptrend confirmed by:\n1. RSI at 35 (oversold)\n2. MACD bullish crossover\n3. Price above 50-day EMA\n4. Volume increasing on up days",
      "context": {
        "price": 42000.00,
        "rsi": 35.2,
        "macd": 120.5,
        "volume_24h": 1500000000
      },
      "prompt_tokens": 1250,
      "completion_tokens": 180,
      "model": "claude-sonnet-4",
      "outcome": "SUCCESS",
      "actual_pnl": 250.50,
      "created_at": "2025-01-15T10:00:00Z"
    }
  ],
  "count": 1,
  "filter": {
    "symbol": "BTC/USDT",
    "limit": 50,
    "offset": 0
  }
}
```

#### `GET /api/v1/decisions/:id` - Get Decision Details

Get detailed information about a specific LLM decision.

**Path Parameters:**
- `id` (UUID, required): Decision ID

**Response:**
```json
{
  "id": "dec-123",
  "agent_name": "trend-agent",
  "symbol": "BTC/USDT",
  "decision_type": "signal",
  "decision": "BUY",
  "confidence": 0.85,
  "reasoning": "Strong uptrend confirmed...",
  "context": {
    "price": 42000.00,
    "indicators": {...}
  },
  "prompt": "Full LLM prompt text...",
  "response": "Full LLM response text...",
  "prompt_embedding": [0.123, -0.456, ...],
  "model": "claude-sonnet-4",
  "outcome": "SUCCESS",
  "actual_pnl": 250.50,
  "created_at": "2025-01-15T10:00:00Z"
}
```

**Errors:**
- `400`: Invalid decision ID
- `404`: Decision not found

#### `GET /api/v1/decisions/:id/similar` - Find Similar Decisions

Find decisions with similar market context using vector similarity search (pgvector).

**Path Parameters:**
- `id` (UUID, required): Reference decision ID

**Query Parameters:**
- `limit` (int, optional): Number of results (default 10, max 50)

**Response:**
```json
{
  "decision_id": "dec-123",
  "similar": [
    {
      "id": "dec-456",
      "symbol": "BTC/USDT",
      "decision": "BUY",
      "confidence": 0.82,
      "outcome": "SUCCESS",
      "similarity": 0.94,
      "created_at": "2025-01-14T15:30:00Z"
    },
    {
      "id": "dec-789",
      "symbol": "BTC/USDT",
      "decision": "BUY",
      "confidence": 0.78,
      "outcome": "FAILURE",
      "similarity": 0.89,
      "created_at": "2025-01-13T09:15:00Z"
    }
  ],
  "count": 2
}
```

**Use Case:** Learn from past decisions in similar market conditions.

#### `GET /api/v1/decisions/stats` - Decision Statistics

Get aggregated statistics for LLM decisions.

**Query Parameters:**
- `symbol` (string, optional): Filter by symbol
- `decision_type` (string, optional): Filter by type
- `from_date` (RFC3339, optional): Filter from date
- `to_date` (RFC3339, optional): Filter to date

**Response:**
```json
{
  "total_decisions": 342,
  "by_outcome": {
    "SUCCESS": 198,
    "FAILURE": 87,
    "PENDING": 57
  },
  "by_decision_type": {
    "signal": 280,
    "risk_approval": 62
  },
  "avg_confidence": 0.76,
  "success_rate": 0.69,
  "avg_pnl": 125.30,
  "by_model": {
    "claude-sonnet-4": 310,
    "gpt-4-turbo": 32
  },
  "total_cost": 45.20,
  "avg_latency_ms": 1250
}
```

---

## WebSocket API

### Connection

**Endpoint:** `ws://localhost:8080/api/v1/ws`

**Protocol:** WebSocket (RFC 6455)

### Connection Example

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onopen = () => {
  console.log('Connected to CryptoFunk WebSocket');
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message);
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected from WebSocket');
};
```

### Message Format

All WebSocket messages follow this structure:

```json
{
  "type": "message_type",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    // Type-specific data
  }
}
```

### Message Types

#### 1. `position_update` - Position Updates

Broadcast when a position is opened, updated, or closed.

```json
{
  "type": "position_update",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "position_id": "pos-123",
    "session_id": "session-456",
    "symbol": "BTC/USDT",
    "side": "LONG",
    "entry_price": 42000.00,
    "quantity": 0.1,
    "unrealized_pnl": 150.50,
    "realized_pnl": 0.0
  }
}
```

#### 2. `order_update` - Order Updates

Broadcast when an order status changes (NEW → FILLED, CANCELED, etc.).

```json
{
  "type": "order_update",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "order_id": "order-789",
    "symbol": "BTC/USDT",
    "side": "BUY",
    "type": "MARKET",
    "status": "FILLED",
    "quantity": 0.1,
    "executed_quantity": 0.1,
    "filled_at": "2025-01-15T10:30:00Z"
  }
}
```

#### 3. `trade` - Trade Notifications

Broadcast when an order is filled (individual fill).

```json
{
  "type": "trade",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "trade_id": "trade-321",
    "order_id": "order-789",
    "symbol": "BTC/USDT",
    "side": "BUY",
    "price": 42000.00,
    "quantity": 0.1,
    "commission": 4.20,
    "executed_at": "2025-01-15T10:30:00Z"
  }
}
```

#### 4. `agent_status` - Agent Status Changes

Broadcast when an agent's status changes (active, inactive, error, etc.).

```json
{
  "type": "agent_status",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "name": "technical-agent",
    "status": "active",
    "last_seen_at": "2025-01-15T10:30:00Z",
    "is_healthy": true,
    "metadata": {
      "signals_generated": 42
    }
  }
}
```

#### 5. `system_status` - System Events

Broadcast for system-wide events (trading started/stopped, errors, etc.).

```json
{
  "type": "system_status",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "status": "trading_started",
    "message": "Trading session started",
    "metadata": {
      "session_id": "session-456",
      "symbol": "BTC/USDT",
      "mode": "PAPER"
    }
  }
}
```

#### 6. `error` - Error Messages

Broadcast when errors occur.

```json
{
  "type": "error",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "error": "Failed to execute order",
    "details": "Insufficient balance",
    "severity": "high"
  }
}
```

#### 7. `ping` / `pong` - Heartbeat

Client can send ping to check connection health.

**Client sends:**
```json
{
  "type": "ping",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {}
}
```

**Server responds:**
```json
{
  "type": "pong",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {}
}
```

### WebSocket Configuration

- **Write Timeout:** 10 seconds
- **Pong Timeout:** 60 seconds
- **Ping Interval:** 54 seconds
- **Max Message Size:** 512 bytes (client → server)
- **Send Buffer:** 256 messages

### Connection Management

The server automatically:
- Sends ping messages every 54 seconds
- Closes stale connections after 60 seconds without pong
- Broadcasts to all connected clients simultaneously
- Handles graceful disconnection and cleanup

---

## Data Models

### Order

```typescript
{
  id: UUID,
  session_id: UUID | null,
  position_id: UUID | null,
  exchange_order_id: string | null,
  symbol: string,
  exchange: string,
  side: "BUY" | "SELL",
  type: "MARKET" | "LIMIT" | "STOP_LOSS" | "STOP_LOSS_LIMIT" | "TAKE_PROFIT" | "TAKE_PROFIT_LIMIT",
  status: "NEW" | "PARTIALLY_FILLED" | "FILLED" | "CANCELED" | "REJECTED" | "EXPIRED",
  price: number | null,
  stop_price: number | null,
  quantity: number,
  executed_quantity: number,
  executed_quote_quantity: number,
  time_in_force: "GTC" | "IOC" | "FOK",
  placed_at: timestamp,
  filled_at: timestamp | null,
  canceled_at: timestamp | null,
  expired_at: timestamp | null,
  error_message: string | null,
  created_at: timestamp,
  updated_at: timestamp
}
```

### Position

```typescript
{
  id: UUID,
  session_id: UUID,
  symbol: string,
  exchange: string,
  side: "LONG" | "SHORT",
  entry_price: number,
  exit_price: number | null,
  quantity: number,
  entry_time: timestamp,
  exit_time: timestamp | null,
  stop_loss: number | null,
  take_profit: number | null,
  realized_pnl: number,
  unrealized_pnl: number,
  fees: number,
  entry_reason: string | null,
  exit_reason: string | null,
  created_at: timestamp,
  updated_at: timestamp
}
```

### Agent Status

```typescript
{
  name: string,
  status: "active" | "inactive" | "error",
  last_seen_at: timestamp,
  is_healthy: boolean,
  metadata: object
}
```

### Trading Session

```typescript
{
  id: UUID,
  mode: "PAPER" | "LIVE",
  symbol: string,
  exchange: string,
  started_at: timestamp,
  stopped_at: timestamp | null,
  initial_capital: number,
  final_capital: number,
  total_pnl: number,
  total_trades: number,
  winning_trades: number,
  losing_trades: number,
  max_drawdown: number,
  sharpe_ratio: number,
  created_at: timestamp,
  updated_at: timestamp
}
```

---

## Examples

### Example 1: Start Paper Trading and Monitor via WebSocket

```javascript
// 1. Start trading session
const startResponse = await fetch('http://localhost:8080/api/v1/trade/start', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    symbol: 'BTC/USDT',
    initial_capital: 10000.00,
    mode: 'PAPER'
  })
});

const { session_id } = await startResponse.json();
console.log('Trading started, session:', session_id);

// 2. Connect to WebSocket for real-time updates
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);

  switch (message.type) {
    case 'order_update':
      console.log('Order update:', message.data);
      break;
    case 'position_update':
      console.log('Position update:', message.data);
      updatePositionUI(message.data);
      break;
    case 'agent_status':
      console.log('Agent status:', message.data);
      break;
  }
};

// 3. Query positions periodically
setInterval(async () => {
  const positions = await fetch(`http://localhost:8080/api/v1/positions?session_id=${session_id}`);
  const data = await positions.json();
  console.log('Current positions:', data.positions);
}, 5000);
```

### Example 2: Place Manual Order

```javascript
const orderResponse = await fetch('http://localhost:8080/api/v1/orders', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    symbol: 'BTC/USDT',
    side: 'BUY',
    type: 'LIMIT',
    quantity: 0.1,
    price: 41500.00
  })
});

const { order } = await orderResponse.json();
console.log('Order placed:', order.id);

// Listen for order fill on WebSocket
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  if (message.type === 'order_update' && message.data.order_id === order.id) {
    console.log('Order status:', message.data.status);
    if (message.data.status === 'FILLED') {
      console.log('Order filled at:', message.data.filled_at);
    }
  }
};
```

### Example 3: View Decision Reasoning

```javascript
// Get recent decisions
const decisionsResponse = await fetch('http://localhost:8080/api/v1/decisions?limit=10');
const { decisions } = await decisionsResponse.json();

// View reasoning for each decision
decisions.forEach(decision => {
  console.log(`\nDecision: ${decision.decision} (confidence: ${decision.confidence})`);
  console.log(`Reasoning: ${decision.reasoning}`);
  console.log(`Outcome: ${decision.outcome}, P&L: $${decision.actual_pnl}`);
});

// Find similar past decisions
const similarResponse = await fetch(`http://localhost:8080/api/v1/decisions/${decisions[0].id}/similar?limit=5`);
const { similar } = await similarResponse.json();

console.log('\nSimilar past decisions:');
similar.forEach(s => {
  console.log(`- ${s.decision} (similarity: ${s.similarity}, outcome: ${s.outcome})`);
});
```

### Example 4: Update Risk Configuration

```javascript
const configResponse = await fetch('http://localhost:8080/api/v1/config', {
  method: 'PATCH',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    max_position_size: 0.15,
    default_stop_loss: 0.03,
    default_take_profit: 0.06,
    min_confidence: 0.75
  })
});

const { updates } = await configResponse.json();
console.log('Configuration updated:', updates);
```

---

## Future Enhancements

**Planned for Phase 10:**
- OpenAPI/Swagger specification (Task T224)
- JWT authentication (Task T222)
- Role-based authorization (Task T223)
- Rate limiting
- API versioning (v2)
- GraphQL endpoint (optional)

---

## Support

For issues, questions, or feature requests:
- GitHub Issues: https://github.com/ajitpratap0/cryptofunk/issues
- Documentation: See README.md, CLAUDE.md
- Contributing: See CONTRIBUTING.md

---

**Last Updated:** 2025-01-15
**API Version:** 1.0.0
