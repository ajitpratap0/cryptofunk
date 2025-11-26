package strategy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ExportFormat specifies the output format for strategy export
type ExportFormat string

const (
	FormatYAML ExportFormat = "yaml"
	FormatJSON ExportFormat = "json"
)

// ExportOptions configures strategy export behavior
type ExportOptions struct {
	// Format specifies the output format (yaml or json)
	Format ExportFormat

	// IncludeMetadata includes full metadata in export
	IncludeMetadata bool

	// PrettyPrint enables indented output
	PrettyPrint bool

	// StripSensitive removes potentially sensitive fields
	StripSensitive bool

	// AddComments adds YAML comments explaining fields (YAML only)
	AddComments bool
}

// DefaultExportOptions returns the default export options
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Format:          FormatYAML,
		IncludeMetadata: true,
		PrettyPrint:     true,
		StripSensitive:  false,
		AddComments:     true,
	}
}

// ImportOptions configures strategy import behavior
type ImportOptions struct {
	// ValidateStrict performs full validation (default: true)
	ValidateStrict bool

	// AllowUnknownFields allows extra fields in input
	AllowUnknownFields bool

	// GenerateNewID generates a new ID for imported strategy
	GenerateNewID bool

	// OverrideMetadata allows specifying new metadata
	OverrideMetadata *StrategyMetadata
}

// DefaultImportOptions returns the default import options
func DefaultImportOptions() ImportOptions {
	return ImportOptions{
		ValidateStrict:     true,
		AllowUnknownFields: true, // Be lenient with extra fields
		GenerateNewID:      true,
	}
}

// Export serializes a strategy configuration to the specified format
func Export(strategy *StrategyConfig, opts ExportOptions) ([]byte, error) {
	if strategy == nil {
		return nil, fmt.Errorf("strategy cannot be nil")
	}

	// Create a copy to avoid modifying the original
	exportStrategy := *strategy

	// Update metadata for export
	if opts.IncludeMetadata {
		exportStrategy.Metadata.UpdatedAt = time.Now()
		if exportStrategy.Metadata.ID == "" {
			exportStrategy.Metadata.ID = uuid.New().String()
		}
		if exportStrategy.Metadata.SchemaVersion == "" {
			exportStrategy.Metadata.SchemaVersion = SchemaVersion
		}
		if exportStrategy.Metadata.Source == "" {
			exportStrategy.Metadata.Source = "export"
		}
	}

	// Strip sensitive data if requested (future extension for API keys, etc.)
	// Currently no sensitive fields in strategy config

	switch opts.Format {
	case FormatYAML:
		return exportToYAML(&exportStrategy, opts)
	case FormatJSON:
		return exportToJSON(&exportStrategy, opts)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", opts.Format)
	}
}

func exportToYAML(strategy *StrategyConfig, opts ExportOptions) ([]byte, error) {
	var buf bytes.Buffer

	if opts.AddComments {
		// Add header comment
		buf.WriteString("# CryptoFunk Strategy Configuration\n")
		buf.WriteString(fmt.Sprintf("# Schema Version: %s\n", strategy.Metadata.SchemaVersion))
		buf.WriteString(fmt.Sprintf("# Exported: %s\n", time.Now().Format(time.RFC3339)))
		buf.WriteString("# Documentation: https://github.com/ajitpratap0/cryptofunk/docs/strategy.md\n")
		buf.WriteString("\n")
	}

	encoder := yaml.NewEncoder(&buf)
	if opts.PrettyPrint {
		encoder.SetIndent(2)
	}

	if err := encoder.Encode(strategy); err != nil {
		return nil, fmt.Errorf("failed to encode strategy to YAML: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
	}

	return buf.Bytes(), nil
}

func exportToJSON(strategy *StrategyConfig, opts ExportOptions) ([]byte, error) {
	var data []byte
	var err error

	if opts.PrettyPrint {
		data, err = json.MarshalIndent(strategy, "", "  ")
	} else {
		data, err = json.Marshal(strategy)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode strategy to JSON: %w", err)
	}

	return data, nil
}

// ExportToFile exports a strategy to a file
func ExportToFile(strategy *StrategyConfig, path string, opts ExportOptions) error {
	// Determine format from file extension if not specified
	if opts.Format == "" {
		ext := filepath.Ext(path)
		switch ext {
		case ".yaml", ".yml":
			opts.Format = FormatYAML
		case ".json":
			opts.Format = FormatJSON
		default:
			opts.Format = FormatYAML
		}
	}

	data, err := Export(strategy, opts)
	if err != nil {
		return fmt.Errorf("failed to export strategy: %w", err)
	}

	// Ensure directory exists with restrictive permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write with restrictive permissions (user read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write strategy file: %w", err)
	}

	return nil
}

// Import deserializes a strategy configuration from bytes
func Import(data []byte, opts ImportOptions) (*StrategyConfig, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty strategy data")
	}

	// Detect format using first non-whitespace character (more efficient than json.Valid)
	var strategy StrategyConfig
	var parseErr error

	// Find first non-whitespace character to detect format
	isJSON := false
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		// JSON typically starts with '{' for objects or '[' for arrays
		isJSON = b == '{' || b == '['
		break
	}

	if isJSON {
		// Try parsing as JSON
		if err := json.Unmarshal(data, &strategy); err != nil {
			// If JSON parsing fails, fall back to YAML (handles edge cases)
			if yamlErr := yaml.Unmarshal(data, &strategy); yamlErr != nil {
				parseErr = fmt.Errorf("failed to parse as JSON (%v) or YAML (%v)", err, yamlErr)
			}
		}
	} else {
		// Try YAML first
		if err := yaml.Unmarshal(data, &strategy); err != nil {
			// Try JSON as fallback (in case whitespace check was wrong)
			if jsonErr := json.Unmarshal(data, &strategy); jsonErr != nil {
				parseErr = fmt.Errorf("failed to parse as YAML (%v) or JSON (%v)", err, jsonErr)
			}
		}
	}

	if parseErr != nil {
		return nil, parseErr
	}

	// Apply import options
	if opts.GenerateNewID {
		strategy.Metadata.ID = uuid.New().String()
	}

	// Override metadata if specified
	if opts.OverrideMetadata != nil {
		if opts.OverrideMetadata.Name != "" {
			strategy.Metadata.Name = opts.OverrideMetadata.Name
		}
		if opts.OverrideMetadata.Description != "" {
			strategy.Metadata.Description = opts.OverrideMetadata.Description
		}
		if opts.OverrideMetadata.Author != "" {
			strategy.Metadata.Author = opts.OverrideMetadata.Author
		}
		if len(opts.OverrideMetadata.Tags) > 0 {
			strategy.Metadata.Tags = opts.OverrideMetadata.Tags
		}
	}

	// Set import timestamp
	strategy.Metadata.UpdatedAt = time.Now()
	if strategy.Metadata.Source == "" {
		strategy.Metadata.Source = "import"
	}

	// Validate the strategy
	if opts.ValidateStrict {
		if err := strategy.Validate(); err != nil {
			return nil, fmt.Errorf("strategy validation failed: %w", err)
		}
	} else {
		if err := strategy.ValidateQuick(); err != nil {
			return nil, fmt.Errorf("strategy validation failed: %w", err)
		}
	}

	return &strategy, nil
}

// ImportFromFile imports a strategy from a file
func ImportFromFile(path string, opts ImportOptions) (*StrategyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read strategy file: %w", err)
	}

	strategy, err := Import(data, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to import strategy from %s: %w", path, err)
	}

	return strategy, nil
}

// ImportFromReader imports a strategy from an io.Reader
func ImportFromReader(r io.Reader, opts ImportOptions) (*StrategyConfig, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read strategy data: %w", err)
	}

	return Import(data, opts)
}

// Clone creates a deep copy of a strategy
func Clone(strategy *StrategyConfig) (*StrategyConfig, error) {
	if strategy == nil {
		return nil, fmt.Errorf("strategy cannot be nil")
	}

	// Use JSON marshaling for deep copy
	data, err := json.Marshal(strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal strategy: %w", err)
	}

	var clone StrategyConfig
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategy: %w", err)
	}

	// Generate new ID for clone
	clone.Metadata.ID = uuid.New().String()
	clone.Metadata.CreatedAt = time.Now()
	clone.Metadata.UpdatedAt = time.Now()
	clone.Metadata.Source = "clone"

	return &clone, nil
}

// Merge merges two strategies, with the second taking precedence for non-zero values.
//
// IMPORTANT: Due to Go's zero value semantics, this function cannot distinguish between
// "field not specified" and "field explicitly set to zero". This means:
//   - To set a numeric value to 0, you must use the base strategy's defaults
//   - Override values of 0 are treated as "not specified" and won't override the base
//   - For boolean fields like VotingEnabled/LLMReasoningEnabled, the override always takes effect
//
// If you need to explicitly set values to zero, modify the base strategy directly or
// use Import with a complete strategy configuration.
func Merge(base, override *StrategyConfig) (*StrategyConfig, error) {
	if base == nil {
		return nil, fmt.Errorf("base strategy cannot be nil")
	}

	// Clone the base first
	result, err := Clone(base)
	if err != nil {
		return nil, fmt.Errorf("failed to clone base strategy: %w", err)
	}

	if override == nil {
		return result, nil
	}

	// Merge metadata
	if override.Metadata.Name != "" {
		result.Metadata.Name = override.Metadata.Name
	}
	if override.Metadata.Description != "" {
		result.Metadata.Description = override.Metadata.Description
	}
	if len(override.Metadata.Tags) > 0 {
		result.Metadata.Tags = override.Metadata.Tags
	}

	// Merge agent weights (only non-zero values)
	if override.Agents.Weights.Technical > 0 {
		result.Agents.Weights.Technical = override.Agents.Weights.Technical
	}
	if override.Agents.Weights.OrderBook > 0 {
		result.Agents.Weights.OrderBook = override.Agents.Weights.OrderBook
	}
	if override.Agents.Weights.Sentiment > 0 {
		result.Agents.Weights.Sentiment = override.Agents.Weights.Sentiment
	}
	if override.Agents.Weights.Trend > 0 {
		result.Agents.Weights.Trend = override.Agents.Weights.Trend
	}
	if override.Agents.Weights.Reversion > 0 {
		result.Agents.Weights.Reversion = override.Agents.Weights.Reversion
	}
	if override.Agents.Weights.Arbitrage > 0 {
		result.Agents.Weights.Arbitrage = override.Agents.Weights.Arbitrage
	}

	// Merge enabled agents (always take override if defined)
	result.Agents.Enabled = override.Agents.Enabled

	// Merge agent-specific configs if provided
	if override.Agents.Technical != nil {
		result.Agents.Technical = override.Agents.Technical
	}
	if override.Agents.Sentiment != nil {
		result.Agents.Sentiment = override.Agents.Sentiment
	}
	if override.Agents.OrderBook != nil {
		result.Agents.OrderBook = override.Agents.OrderBook
	}
	if override.Agents.Trend != nil {
		result.Agents.Trend = override.Agents.Trend
	}
	if override.Agents.Reversion != nil {
		result.Agents.Reversion = override.Agents.Reversion
	}
	if override.Agents.Arbitrage != nil {
		result.Agents.Arbitrage = override.Agents.Arbitrage
	}

	// Risk settings - merge non-zero values
	mergeRiskSettings(&result.Risk, &override.Risk)

	// Orchestration - merge non-zero values
	mergeOrchestration(&result.Orchestration, &override.Orchestration)

	// Indicators - merge non-zero values
	mergeIndicators(&result.Indicators, &override.Indicators)

	result.Metadata.UpdatedAt = time.Now()
	result.Metadata.Source = "merge"

	return result, nil
}

func mergeRiskSettings(base, override *RiskSettings) {
	if override.MaxPortfolioExposure > 0 {
		base.MaxPortfolioExposure = override.MaxPortfolioExposure
	}
	if override.MaxPositionSize > 0 {
		base.MaxPositionSize = override.MaxPositionSize
	}
	if override.MaxPositions > 0 {
		base.MaxPositions = override.MaxPositions
	}
	if override.MaxDailyLoss > 0 {
		base.MaxDailyLoss = override.MaxDailyLoss
	}
	if override.MaxDrawdown > 0 {
		base.MaxDrawdown = override.MaxDrawdown
	}
	if override.MinStrategyConfidence > 0 {
		base.MinStrategyConfidence = override.MinStrategyConfidence
	}
	if override.MinConsensusVotes > 0 {
		base.MinConsensusVotes = override.MinConsensusVotes
	}
	if override.DefaultStopLoss > 0 {
		base.DefaultStopLoss = override.DefaultStopLoss
	}
	if override.DefaultTakeProfit > 0 {
		base.DefaultTakeProfit = override.DefaultTakeProfit
	}
	if override.KellyFraction > 0 {
		base.KellyFraction = override.KellyFraction
	}
	if override.MinPositionUSD > 0 {
		base.MinPositionUSD = override.MinPositionUSD
	}
	if override.MaxPositionUSD > 0 {
		base.MaxPositionUSD = override.MaxPositionUSD
	}
	// Circuit breakers - merge if any field is explicitly set
	// Check for Enabled OR any non-zero values in the circuit breaker config
	if override.CircuitBreakers.Enabled ||
		override.CircuitBreakers.MaxTradesPerHour > 0 ||
		override.CircuitBreakers.MaxLossesPerDay > 0 ||
		override.CircuitBreakers.VolatilityThreshold > 0 ||
		override.CircuitBreakers.DrawdownHalt > 0 {
		base.CircuitBreakers = override.CircuitBreakers
	}
}

func mergeOrchestration(base, override *OrchestrationSettings) {
	if override.VotingMethod != "" {
		base.VotingMethod = override.VotingMethod
	}
	if override.MinVotes > 0 {
		base.MinVotes = override.MinVotes
	}
	if override.Quorum > 0 {
		base.Quorum = override.Quorum
	}
	if override.StepInterval != "" {
		base.StepInterval = override.StepInterval
	}
	if override.MaxSignalAge != "" {
		base.MaxSignalAge = override.MaxSignalAge
	}
	if override.MinConsensus > 0 {
		base.MinConsensus = override.MinConsensus
	}
	if override.MinConfidence > 0 {
		base.MinConfidence = override.MinConfidence
	}
	if override.LLMTemperature > 0 {
		base.LLMTemperature = override.LLMTemperature
	}
	base.VotingEnabled = override.VotingEnabled
	base.LLMReasoningEnabled = override.LLMReasoningEnabled
}

func mergeIndicators(base, override *IndicatorSettings) {
	// RSI
	if override.RSI.Period > 0 {
		base.RSI.Period = override.RSI.Period
	}
	if override.RSI.Overbought > 0 {
		base.RSI.Overbought = override.RSI.Overbought
	}
	if override.RSI.Oversold > 0 {
		base.RSI.Oversold = override.RSI.Oversold
	}

	// MACD
	if override.MACD.FastPeriod > 0 {
		base.MACD.FastPeriod = override.MACD.FastPeriod
	}
	if override.MACD.SlowPeriod > 0 {
		base.MACD.SlowPeriod = override.MACD.SlowPeriod
	}
	if override.MACD.SignalPeriod > 0 {
		base.MACD.SignalPeriod = override.MACD.SignalPeriod
	}

	// Bollinger
	if override.Bollinger.Period > 0 {
		base.Bollinger.Period = override.Bollinger.Period
	}
	if override.Bollinger.StdDev > 0 {
		base.Bollinger.StdDev = override.Bollinger.StdDev
	}

	// EMA
	if len(override.EMA.Periods) > 0 {
		base.EMA.Periods = override.EMA.Periods
	}

	// ADX
	if override.ADX.Period > 0 {
		base.ADX.Period = override.ADX.Period
	}
	if override.ADX.TrendThreshold > 0 {
		base.ADX.TrendThreshold = override.ADX.TrendThreshold
	}
}
