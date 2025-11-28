# Backtest API Documentation (T312)

This document describes the Strategy Backtesting API endpoints for the CryptoFunk project.

## Overview

The Backtest API allows users to test trading strategies against historical data before deploying them in live trading. All backtest jobs run asynchronously, allowing users to queue multiple backtests and retrieve results when complete.

## API Endpoints

### 1. POST /api/v1/backtest/run

Start a new backtest job (async).

**Request Body:**
```json
{
  "name": "BTC Trend Test",
  "start_date": "2024-01-01",
  "end_date": "2024-06-01",
  "symbols": ["BTC/USDT"],
  "initial_capital": 10000,
  "strategy": {
    "type": "trend_following",
    "parameters": {
      "period": 20,
      "threshold": 0.02
    }
  },
  "parameter_grid": {
    "period": [10, 20, 50],
    "threshold": [0.01, 0.02, 0.05]
  }
}
```

**Response (202 Accepted):**
```json
{
  "id": "uuid",
  "status": "pending",
  "message": "Backtest job created successfully. Use GET /api/v1/backtest/:id to check status."
}
```

**Validation Rules:**
- `name` (required): User-friendly name for the backtest
- `start_date` (required): Start date in YYYY-MM-DD format
- `end_date` (required): End date in YYYY-MM-DD format (must be after start_date)
- `symbols` (required): Array of trading symbols (at least 1)
- `initial_capital` (required): Initial capital amount (must be > 0)
- `strategy` (required): Strategy configuration object
- `parameter_grid` (optional): Parameter grid for optimization

### 2. GET /api/v1/backtest/:id

Get backtest status and results.

**Response (200 OK):**

**Pending/Running:**
```json
{
  "id": "uuid",
  "name": "BTC Trend Test",
  "status": "running",
  "start_date": "2024-01-01T00:00:00Z",
  "end_date": "2024-06-01T00:00:00Z",
  "symbols": ["BTC/USDT"],
  "initial_capital": 10000,
  "created_at": "2024-11-28T10:00:00Z",
  "started_at": "2024-11-28T10:00:01Z"
}
```

**Completed:**
```json
{
  "id": "uuid",
  "name": "BTC Trend Test",
  "status": "completed",
  "start_date": "2024-01-01T00:00:00Z",
  "end_date": "2024-06-01T00:00:00Z",
  "symbols": ["BTC/USDT"],
  "initial_capital": 10000,
  "created_at": "2024-11-28T10:00:00Z",
  "started_at": "2024-11-28T10:00:01Z",
  "completed_at": "2024-11-28T10:05:30Z",
  "results": {
    "total_return_pct": 15.2,
    "sharpe_ratio": 1.8,
    "max_drawdown_pct": -8.5,
    "win_rate": 0.62,
    "total_trades": 45,
    "profit_factor": 2.5,
    "sortino_ratio": 2.1,
    "calmar_ratio": 1.9,
    "expectancy": 125.5,
    "winning_trades": 28,
    "losing_trades": 17,
    "average_win": 350.0,
    "average_loss": -180.0,
    "largest_win": 1200.0,
    "largest_loss": -450.0,
    "equity_curve": [
      {"date": "2024-01-01", "value": 10000},
      {"date": "2024-01-02", "value": 10150},
      ...
    ],
    "trades": [
      {
        "symbol": "BTC/USDT",
        "side": "LONG",
        "entry_time": "2024-01-01T00:00:00Z",
        "exit_time": "2024-01-05T00:00:00Z",
        "entry_price": 42000,
        "exit_price": 43500,
        "quantity": 0.1,
        "pnl": 150.0,
        "pnl_pct": 3.57,
        "commission": 4.2,
        "holding_time": "4d 0h 0m"
      },
      ...
    ]
  }
}
```

**Failed:**
```json
{
  "id": "uuid",
  "name": "BTC Trend Test",
  "status": "failed",
  "error_message": "Insufficient historical data for BTC/USDT",
  "created_at": "2024-11-28T10:00:00Z",
  "started_at": "2024-11-28T10:00:01Z",
  "completed_at": "2024-11-28T10:00:15Z"
}
```

**Error Response (404 Not Found):**
```json
{
  "error": "Backtest job not found",
  "job_id": "invalid-uuid"
}
```

### 3. GET /api/v1/backtest

List user's backtests (paginated).

**Query Parameters:**
- `limit` (optional, default: 20): Number of results per page (1-100)
- `offset` (optional, default: 0): Offset for pagination

**Response (200 OK):**
```json
{
  "backtests": [
    {
      "id": "uuid",
      "name": "BTC Trend Test",
      "status": "completed",
      "start_date": "2024-01-01T00:00:00Z",
      "end_date": "2024-06-01T00:00:00Z",
      "symbols": ["BTC/USDT"],
      "initial_capital": 10000,
      "results": {
        "total_return_pct": 15.2,
        "sharpe_ratio": 1.8,
        "max_drawdown_pct": -8.5,
        "win_rate": 0.62,
        "total_trades": 45
      },
      "created_at": "2024-11-28T10:00:00Z",
      "completed_at": "2024-11-28T10:05:30Z"
    },
    ...
  ],
  "total": 10,
  "limit": 20,
  "offset": 0,
  "has_more": false
}
```

### 4. DELETE /api/v1/backtest/:id

Delete a backtest job.

**Response (200 OK):**
```json
{
  "message": "Backtest job deleted successfully",
  "job_id": "uuid"
}
```

**Error Response (403 Forbidden):**
```json
{
  "error": "You don't have permission to delete this backtest job"
}
```

### 5. POST /api/v1/backtest/:id/cancel

Cancel a running backtest job.

**Response (200 OK):**
```json
{
  "message": "Backtest job cancelled successfully",
  "job_id": "uuid",
  "status": "cancelled"
}
```

**Error Response (409 Conflict):**
```json
{
  "error": "Cannot cancel backtest job",
  "details": "Job is not in pending or running state",
  "status": "completed"
}
```

## Job Status Flow

```
pending → running → completed
                 → failed
                 → cancelled
```

- **pending**: Job created, waiting to execute
- **running**: Job is currently executing
- **completed**: Job finished successfully with results
- **failed**: Job encountered an error
- **cancelled**: Job was cancelled by user

## Performance Metrics

The backtest results include the following performance metrics:

### Return Metrics
- **total_return_pct**: Total return percentage
- **profit_factor**: Ratio of total profit to total loss
- **expectancy**: Expected value per trade in dollars

### Risk Metrics
- **sharpe_ratio**: Risk-adjusted return (higher is better)
- **sortino_ratio**: Downside risk-adjusted return
- **calmar_ratio**: CAGR divided by maximum drawdown
- **max_drawdown_pct**: Maximum peak-to-trough decline

### Trade Statistics
- **total_trades**: Total number of trades executed
- **winning_trades**: Number of profitable trades
- **losing_trades**: Number of losing trades
- **win_rate**: Percentage of winning trades (0-1)
- **average_win**: Average profit per winning trade
- **average_loss**: Average loss per losing trade
- **largest_win**: Largest single trade profit
- **largest_loss**: Largest single trade loss

## Rate Limiting

Backtest endpoints are rate-limited to prevent resource exhaustion:

- **Read operations** (GET): Higher limits (60 requests/minute)
- **Write operations** (POST, DELETE): Lower limits (10 requests/minute)

## Implementation Notes

### Database Schema

Backtest jobs are stored in the `backtest_jobs` table with the following key fields:

- Job configuration (name, dates, symbols, capital, strategy)
- Status tracking (pending → running → completed/failed/cancelled)
- Results (JSONB field with complete backtest output)
- Denormalized metrics for quick querying (sharpe_ratio, total_return_pct, etc.)

### Async Execution

Backtest jobs are designed for async execution:

1. **Job Creation**: POST /api/v1/backtest/run creates job in `pending` state
2. **Worker Processing**: Background worker picks up pending jobs (not yet implemented)
3. **Status Updates**: Worker updates status to `running` → `completed`/`failed`
4. **Result Retrieval**: GET /api/v1/backtest/:id returns current status and results

**Note**: The actual backtest execution worker is not yet implemented. Jobs will remain in `pending` state until a worker is added.

### Future Enhancements

- **Worker Queue**: Implement background worker using NATS or Redis Queue
- **Progress Updates**: Real-time progress updates via WebSocket
- **Parameter Optimization**: Grid search and Bayesian optimization
- **Comparison Tool**: Compare multiple backtest results side-by-side
- **Export Results**: Export to CSV, PDF, or HTML report

## Example Usage

### Run a Simple Backtest

```bash
# Create backtest job
curl -X POST http://localhost:8080/api/v1/backtest/run \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My First Backtest",
    "start_date": "2024-01-01",
    "end_date": "2024-06-01",
    "symbols": ["BTC/USDT"],
    "initial_capital": 10000,
    "strategy": {
      "type": "trend_following",
      "parameters": {
        "period": 20
      }
    }
  }'

# Response:
# {
#   "id": "a1b2c3d4-...",
#   "status": "pending",
#   "message": "Backtest job created successfully..."
# }

# Check status
curl http://localhost:8080/api/v1/backtest/a1b2c3d4-...

# List all backtests
curl http://localhost:8080/api/v1/backtest?limit=10&offset=0
```

### Cancel a Running Backtest

```bash
curl -X POST http://localhost:8080/api/v1/backtest/a1b2c3d4-.../cancel
```

### Delete a Completed Backtest

```bash
curl -X DELETE http://localhost:8080/api/v1/backtest/a1b2c3d4-...
```

## Integration with Frontend (web/explainability/)

The frontend can use these endpoints to:

1. **Submit Backtests**: Form to configure and submit backtest jobs
2. **Monitor Progress**: Poll GET /api/v1/backtest/:id for status updates
3. **Display Results**: Visualize equity curve, trade log, and metrics
4. **Compare Strategies**: Run multiple backtests and compare results

## Related Documentation

- **T310**: Strategy Import/Export API (provides strategy configuration format)
- **T307**: Decision Explainability Dashboard (similar UI patterns)
- **pkg/backtest/**: Backtest engine implementation
- **migrations/011_backtest_jobs.sql**: Database schema

## Migration

To enable the Backtest API, run the database migration:

```bash
task db-migrate
```

This will create the `backtest_jobs` table with appropriate indexes.
