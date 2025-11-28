# Strategy Import/Export

CryptoFunk provides comprehensive strategy import/export functionality, allowing you to save, share, and version control your trading strategies.

## Overview

The strategy import/export system allows you to:

- **Export** your current strategy configuration to YAML or JSON
- **Import** strategies from files or API requests
- **Validate** strategy configurations before applying them
- **Clone** existing strategies with modifications
- **Merge** strategies to combine configurations
- **Version** strategies with schema compatibility checking

## Strategy Format

Strategies are defined in YAML or JSON format with the following structure:

```yaml
metadata:
  schema_version: "1.0"
  name: "My Trading Strategy"
  description: "Conservative trend following"
  tags: ["trend-following", "conservative"]

agents:
  weights:
    technical: 0.25
    trend: 0.40
  enabled:
    technical: true
    trend: true
    risk: true

risk:
  max_portfolio_exposure: 0.50
  max_position_size: 0.10
  default_stop_loss: 0.02
  default_take_profit: 0.04

orchestration:
  voting_enabled: true
  voting_method: "weighted_consensus"

indicators:
  rsi:
    period: 14
    overbought: 70
    oversold: 30
```

See `configs/strategy-schema.yaml` for complete documentation of all available fields.

## API Endpoints

### Export Current Strategy

**GET** `/api/v1/strategies/export?format=yaml`

Query parameters:
- `format`: Export format (`yaml` or `json`, default: `yaml`)

**Response**: Strategy file download

**Example**:
```bash
curl -O http://localhost:8081/api/v1/strategies/export?format=yaml
```

### Export with Custom Options

**POST** `/api/v1/strategies/export`

**Request body**:
```json
{
  "format": "yaml",
  "include_comments": true,
  "pretty_print": true
}
```

**Response**: Strategy file download

### Import Strategy

**POST** `/api/v1/strategies/import`

**Multipart form upload**:
```bash
curl -X POST http://localhost:8081/api/v1/strategies/import \
  -F "file=@my-strategy.yaml" \
  -F "apply_now=true"
```

**JSON request**:
```bash
curl -X POST http://localhost:8081/api/v1/strategies/import \
  -H "Content-Type: application/json" \
  -d '{
    "data": "metadata:\n  schema_version: \"1.0\"\n  name: \"Test Strategy\"\n...",
    "apply_now": true,
    "validate_strict": true
  }'
```

Form/JSON fields:
- `file`: Strategy file (multipart form only)
- `data`: Strategy content as string (JSON only)
- `apply_now`: Apply strategy immediately (default: `false`)
- `validate_strict`: Perform full validation (default: `true`)

**Response**:
```json
{
  "strategy": { ... },
  "applied": true
}
```

### Validate Strategy

**POST** `/api/v1/strategies/validate`

**Request**:
```json
{
  "data": "metadata:\n  schema_version: \"1.0\"\n...",
  "strict": true
}
```

**Response**:
```json
{
  "valid": true,
  "name": "My Strategy",
  "schema_version": "1.0",
  "version_info": {
    "schema_version": "1.0",
    "is_compatible": true,
    "requires_migration": false
  }
}
```

### Get Current Strategy

**GET** `/api/v1/strategies/current`

**Response**:
```json
{
  "metadata": {
    "schema_version": "1.0",
    "name": "Current Strategy",
    ...
  },
  "agents": { ... },
  "risk": { ... },
  ...
}
```

### Update Current Strategy

**PUT** `/api/v1/strategies/current`

**Request**: Full strategy configuration (JSON)

**Response**: Updated strategy

### Clone Strategy

**POST** `/api/v1/strategies/clone`

**Request**:
```json
{
  "name": "My Strategy Copy",
  "description": "Modified version of original"
}
```

**Response**: Cloned strategy with new ID

### Merge Strategies

**POST** `/api/v1/strategies/merge`

**Request**:
```json
{
  "base": { ... },
  "override": {
    "agents": {
      "weights": {
        "technical": 0.35
      }
    }
  },
  "apply": true
}
```

**Response**:
```json
{
  "strategy": { ... },
  "applied": true
}
```

### Get Version Info

**GET** `/api/v1/strategies/version`

**Response**:
```json
{
  "current_schema_version": "1.0",
  "supported_versions": ["1.0"],
  "strategy": {
    "schema_version": "1.0",
    "is_compatible": true,
    "requires_migration": false
  }
}
```

### Get Schema Info

**GET** `/api/v1/strategies/schema`

**Response**: Schema documentation with required and optional fields

## Validation Rules

### Required Fields

- `metadata.schema_version`: Must be a supported version ("1.0")
- `metadata.name`: Strategy name (max 100 characters)
- At least one agent enabled (besides risk)
- All risk parameters must be present

### Agent Weights

- All weights must be between 0 and 1
- Enabled agents should have non-zero weights
- Weights don't need to sum to 1.0 (weighted voting handles normalization)

### Risk Parameters

- `max_position_size` <= `max_portfolio_exposure`
- `max_daily_loss` <= `max_drawdown`
- `default_stop_loss` < `default_take_profit`
- Circuit breaker `drawdown_halt` <= `max_drawdown`
- All percentages between 0-1
- Position counts >= 1

### Indicator Settings

- RSI: `oversold` < `overbought` (both 0-100)
- MACD: `fast_period` < `slow_period`
- All periods >= 1
- Bollinger `std_dev` > 0

### Orchestration

- `voting_method`: "weighted_consensus" or "majority"
- `step_interval` and `max_signal_age`: valid duration strings (e.g., "30s", "5m", "1h")
- Quorum, consensus, confidence: 0-1
- LLM temperature: 0-2

## Programmatic Usage

### Go Code

```go
import "github.com/ajitpratap0/cryptofunk/internal/strategy"

// Create a new strategy
s := strategy.NewDefaultStrategy("My Strategy")

// Export to YAML file
opts := strategy.DefaultExportOptions()
err := strategy.ExportToFile(s, "my-strategy.yaml", opts)

// Import from file
importOpts := strategy.DefaultImportOptions()
loaded, err := strategy.ImportFromFile("my-strategy.yaml", importOpts)

// Validate
err = loaded.Validate()

// Clone
cloned, err := strategy.Clone(loaded)

// Merge
merged, err := strategy.Merge(base, override)
```

### Command Line

Export current strategy:
```bash
curl http://localhost:8081/api/v1/strategies/export > my-strategy.yaml
```

Import and apply:
```bash
curl -X POST http://localhost:8081/api/v1/strategies/import \
  -F "file=@my-strategy.yaml" \
  -F "apply_now=true"
```

Validate before applying:
```bash
# First validate
curl -X POST http://localhost:8081/api/v1/strategies/validate \
  -H "Content-Type: application/json" \
  -d "{\"data\": \"$(cat my-strategy.yaml)\", \"strict\": true}"

# Then import if valid
curl -X POST http://localhost:8081/api/v1/strategies/import \
  -F "file=@my-strategy.yaml" \
  -F "apply_now=true"
```

## Example Strategies

See `configs/examples/` for pre-built strategy templates:

- **conservative.yaml**: Low-risk, tight stops, high confidence requirements
- **aggressive.yaml**: Higher risk, looser stops, more positions
- **paper-trading.yaml**: Safe defaults for testing
- **trend-following-example.yaml**: Complete example with all fields documented

## Version Compatibility

### Schema Versions

Current schema version: **1.0**

Supported versions: **1.0**

### Migration

Strategies from older schema versions are automatically migrated on import:

```go
// Import automatically migrates to current version
s, err := strategy.ImportFromFile("old-strategy.yaml", opts)
// s.Metadata.SchemaVersion is now "1.0"
```

Check compatibility:
```go
err := strategy.CheckCompatibility(oldStrategy)
if err != nil {
    // Strategy is incompatible
}
```

Get migration path:
```go
path, err := strategy.GetMigrationPath("0.9", "1.0")
// Returns list of migrations to apply
```

## Security Considerations

### File Upload Security

- **Size Limits**: Max 10MB, min 50 bytes
- **Extension Whitelist**: Only `.yaml`, `.yml`, `.json` allowed
- **MIME Type Checking**: Defense-in-depth validation
- **Binary File Detection**: Reject non-text files via magic number checking

### Validation

All imported strategies undergo:
1. Schema validation (required fields, data types)
2. Business rule validation (ranges, cross-field constraints)
3. Format validation (YAML/JSON parsing)

### Permissions

Consider adding authentication for strategy import/export endpoints in production:

```go
// Example: Add authentication middleware
strategies.POST("/import", authMiddleware, h.ImportStrategy)
```

## Best Practices

### Strategy Development

1. **Start with a template**: Use example strategies as starting points
2. **Validate frequently**: Use the validation endpoint during development
3. **Test in paper mode**: Always test new strategies in paper trading first
4. **Version control**: Store strategies in git with meaningful commit messages
5. **Document changes**: Use description and tags fields extensively

### Production Usage

1. **Backup before changes**: Export current strategy before importing new ones
2. **Review before applying**: Use `apply_now: false` to review first
3. **Monitor after changes**: Watch performance metrics after strategy updates
4. **Use strict validation**: Always enable `validate_strict` for production imports
5. **Audit trail**: The system logs all strategy changes when audit logging is enabled

### Organization

```
strategies/
├── production/
│   ├── btc-trend-v2.yaml
│   └── eth-reversion-v1.yaml
├── testing/
│   ├── experimental-arb.yaml
│   └── high-freq-test.yaml
└── archive/
    ├── btc-trend-v1.yaml  (deprecated)
    └── old-balanced.yaml  (deprecated)
```

## Troubleshooting

### Import Fails

**Error**: "Strategy validation failed"

- Check that all required fields are present
- Verify all values are within valid ranges
- Ensure agent weights are between 0-1
- Check that enabled agents have non-zero weights

**Error**: "File too large" / "File too small"

- File must be between 50 bytes and 10MB
- Check file wasn't corrupted during transfer

**Error**: "Invalid file extension"

- Only `.yaml`, `.yml`, and `.json` files are accepted

### Validation Errors

See specific field in error message:
```json
{
  "error": "Strategy validation failed",
  "details": "validation failed: risk.max_daily_loss: max daily loss must be between 0 and 1"
}
```

Fix the field mentioned in the error and try again.

### Export Issues

**Error**: "No strategy configured"

- Import or create a strategy first
- Use `/api/v1/strategies/current` to check current strategy

## Related Documentation

- [Strategy Schema](../configs/strategy-schema.yaml) - Complete field reference
- [Example Strategies](../configs/examples/) - Pre-built templates
- [API Documentation](./API.md) - Complete API reference
- [Risk Management](./RISK_MANAGEMENT.md) - Risk configuration details

## Support

For issues or questions:
- Check validation error messages for specific field issues
- Review example strategies in `configs/examples/`
- Consult schema documentation in `configs/strategy-schema.yaml`
- Open an issue on GitHub with example strategy file that fails
