# Test Fixtures

This directory contains shared test data and helper utilities used across unit, integration, and E2E tests.

## Directory Structure

```
fixtures/
├── candlesticks/      # Sample OHLCV data
├── mock_responses/    # Mock API responses
├── configs/           # Test configuration files
├── helpers.go         # Test helper functions
└── README.md          # This file
```

## Usage

### Loading Fixtures

```go
import "github.com/ajitpratapsingh/cryptofunk/tests/fixtures"

// Load sample candlestick data
candlesticks := fixtures.LoadCandlesticks("BTC-1h-sample.json")

// Load mock API response
mockResponse := fixtures.LoadMockResponse("coingecko-btc-price.json")

// Use test database
db := fixtures.NewTestDB(t)
defer db.Close()
```

## Available Fixtures

### Candlestick Data

Sample OHLCV data for various timeframes and scenarios:

- `BTC-1h-bullish.json` - Bullish trend with increasing volume
- `BTC-1h-bearish.json` - Bearish trend with panic selling
- `BTC-1h-sideways.json` - Range-bound price action
- `BTC-1h-volatile.json` - High volatility period
- `ETH-5m-sample.json` - 5-minute Ethereum data

### Mock API Responses

Pre-recorded API responses for testing:

- `coingecko-btc-price.json` - Bitcoin price response
- `coingecko-market-data.json` - Market data response
- `binance-order-response.json` - Order placement response
- `binance-account-balance.json` - Account balance response

### Test Configurations

Configuration files for different testing scenarios:

- `paper-trading.yaml` - Paper trading configuration
- `conservative.yaml` - Conservative risk parameters
- `aggressive.yaml` - Aggressive risk parameters
- `test-minimal.yaml` - Minimal configuration for unit tests

## Creating New Fixtures

### 1. Candlestick Data

```json
{
  "symbol": "BTC/USDT",
  "interval": "1h",
  "data": [
    {
      "open_time": "2025-01-01T00:00:00Z",
      "open": 45000.0,
      "high": 45500.0,
      "low": 44800.0,
      "close": 45200.0,
      "volume": 1500.5
    }
  ]
}
```

### 2. Mock API Response

```json
{
  "request": {
    "method": "GET",
    "url": "/api/v3/price",
    "params": {"symbol": "BTCUSDT"}
  },
  "response": {
    "status": 200,
    "body": {
      "symbol": "BTCUSDT",
      "price": "45123.45"
    }
  }
}
```

### 3. Test Configuration

```yaml
# fixtures/configs/test-minimal.yaml
trading_mode: PAPER
database:
  url: "postgresql://test:test@localhost:5432/cryptofunk_test"
risk:
  max_position_size: 0.1
  max_drawdown: 0.05
```

## Helper Functions

Common test utilities available in `helpers.go`:

```go
// Database helpers
func NewTestDB(t *testing.T) *sql.DB
func SeedDatabase(t *testing.T, db *sql.DB)
func CleanDatabase(t *testing.T, db *sql.DB)

// Time helpers
func FixedTime() time.Time  // Returns consistent timestamp for tests
func MockClock() Clock      // Returns mock clock for time-dependent tests

// Mock helpers
func NewMockExchange(t *testing.T) *MockExchange
func NewMockLLMClient(t *testing.T) *MockLLMClient
func NewMockMCPServer(t *testing.T) *MockMCPServer

// Data helpers
func GenerateCandlesticks(count int, pattern string) []Candlestick
func GenerateTrades(count int) []Trade
func GeneratePositions(count int) []Position
```

## Best Practices

1. **Keep fixtures small**: Focus on edge cases, don't replicate entire datasets
2. **Version fixtures**: Add version or date to fixture names if they change
3. **Document unusual data**: Add comments explaining non-obvious test data
4. **Avoid hardcoded paths**: Use relative paths and path helpers
5. **Clean up after tests**: Ensure tests don't leave residual state

## Adding New Fixtures

1. Create fixture file in appropriate subdirectory
2. Add documentation here
3. Add helper function if needed
4. Reference in test using fixtures package
5. Commit fixture with descriptive name

---

**Last Updated**: 2025-11-04
**Task**: T260 - Create /tests directory structure
