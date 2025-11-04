# Backtest CLI

Command-line tool for running backtests on historical trading data.

## Usage

```bash
./backtest [options]
```

## Required Flags

- `-strategy` - Strategy name (simple, buy-and-hold)

### For Database Source
- `-start` - Start date (YYYY-MM-DD)
- `-end` - End date (YYYY-MM-DD)

### For CSV/JSON Source
- `-data-path` - Path to CSV/JSON data file

## Optional Flags

### Data Source
- `-data-source` - Data source: database, csv, json (default: database)
- `-symbols` - Comma-separated list of symbols (default: BTC/USDT)

### Capital & Risk
- `-capital` - Initial capital in USD (default: 10000)
- `-commission` - Commission rate, 0.001 = 0.1% (default: 0.001)
- `-sizing` - Position sizing method: fixed, percent, kelly (default: percent)
- `-size` - Position size, depends on sizing method (default: 0.1)
- `-max-positions` - Maximum concurrent positions (default: 3)

### Optimization
- `-optimize` - Run parameter optimization (default: false)
- `-optimize-method` - Optimization method: grid, walk-forward, genetic (default: grid)
- `-optimize-metric` - Optimization metric: sharpe, sortino, calmar, return, profit-factor (default: sharpe)

### Output
- `-output` - Output file for text report (optional)
- `-html` - Generate HTML report to file (optional)
- `-verbose` - Enable verbose logging (default: false)

## Examples

### Basic Backtest from Database

```bash
./backtest \
  -strategy=simple \
  -start=2024-01-01 \
  -end=2024-12-31 \
  -capital=10000 \
  -commission=0.001
```

### Backtest with HTML Report

```bash
./backtest \
  -strategy=buy-and-hold \
  -start=2024-01-01 \
  -end=2024-12-31 \
  -symbols="BTC/USDT,ETH/USDT" \
  -html=report.html \
  -output=results.txt
```

### Backtest with Custom Position Sizing

```bash
./backtest \
  -strategy=simple \
  -start=2024-01-01 \
  -end=2024-12-31 \
  -sizing=percent \
  -size=0.15 \
  -max-positions=5
```

### CSV Data Source (Coming Soon)

```bash
./backtest \
  -strategy=simple \
  -data-source=csv \
  -data-path=historical_data.csv \
  -html=report.html
```

## Strategies

### simple
Basic example strategy that:
- Buys when no position is held and max positions not reached
- Sells after holding for 10 hours

### buy-and-hold
Simple buy-and-hold strategy:
- Buys all symbols at the beginning
- Holds until the end of the backtest period

## Output

The CLI generates:
1. **Console Output**: Summary of backtest results
2. **Text Report** (if `-output` specified): Detailed performance metrics
3. **HTML Report** (if `-html` specified): Interactive charts and visualizations

## Report Metrics

- **Returns**: Total Return, CAGR, Annualized Return
- **Risk**: Max Drawdown, Volatility, Sharpe Ratio, Sortino Ratio
- **Trades**: Total Trades, Win Rate, Profit Factor, Average Win/Loss
- **Time**: Average/Median/Max/Min Holding Time

## Data Requirements

### Database Source
Requires TimescaleDB with `candlesticks` table:
```sql
CREATE TABLE candlesticks (
    symbol TEXT,
    timestamp TIMESTAMPTZ,
    open DOUBLE PRECISION,
    high DOUBLE PRECISION,
    low DOUBLE PRECISION,
    close DOUBLE PRECISION,
    volume DOUBLE PRECISION
);
```

### CSV Format (Coming Soon)
```
timestamp,symbol,open,high,low,close,volume
2024-01-01T00:00:00Z,BTC/USDT,45000.0,45500.0,44800.0,45200.0,1000.5
```

### JSON Format (Coming Soon)
```json
[
  {
    "timestamp": "2024-01-01T00:00:00Z",
    "symbol": "BTC/USDT",
    "open": 45000.0,
    "high": 45500.0,
    "low": 44800.0,
    "close": 45200.0,
    "volume": 1000.5
  }
]
```

## Building

```bash
go build -o bin/backtest ./cmd/backtest
```

## Testing

```bash
go test ./cmd/backtest/...
```
