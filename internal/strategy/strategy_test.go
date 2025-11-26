package strategy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Strategy Types Tests
// =============================================================================

func TestNewDefaultStrategy(t *testing.T) {
	s := NewDefaultStrategy("Test Strategy")

	assert.NotNil(t, s)
	assert.Equal(t, "Test Strategy", s.Metadata.Name)
	assert.Equal(t, SchemaVersion, s.Metadata.SchemaVersion)
	assert.NotEmpty(t, s.Metadata.ID)
	assert.Equal(t, "user", s.Metadata.Source)
	assert.True(t, s.Agents.Enabled.Technical)
	assert.True(t, s.Agents.Enabled.Risk)
	assert.Equal(t, 0.25, s.Agents.Weights.Technical)
}

func TestStrategyConfig_Defaults(t *testing.T) {
	s := NewDefaultStrategy("Test")

	// Risk defaults
	assert.Equal(t, 0.8, s.Risk.MaxPortfolioExposure)
	assert.Equal(t, 0.1, s.Risk.MaxPositionSize)
	assert.Equal(t, 3, s.Risk.MaxPositions)
	assert.Equal(t, 0.02, s.Risk.MaxDailyLoss)
	assert.Equal(t, 0.1, s.Risk.MaxDrawdown)
	assert.True(t, s.Risk.CircuitBreakers.Enabled)

	// Orchestration defaults
	assert.True(t, s.Orchestration.VotingEnabled)
	assert.Equal(t, "weighted_consensus", s.Orchestration.VotingMethod)
	assert.Equal(t, 2, s.Orchestration.MinVotes)
	assert.Equal(t, 0.6, s.Orchestration.Quorum)

	// Indicator defaults
	assert.Equal(t, 14, s.Indicators.RSI.Period)
	assert.Equal(t, 70, s.Indicators.RSI.Overbought)
	assert.Equal(t, 30, s.Indicators.RSI.Oversold)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestStrategyConfig_Validate_Valid(t *testing.T) {
	s := NewDefaultStrategy("Valid Strategy")
	err := s.Validate()
	assert.NoError(t, err)
}

func TestStrategyConfig_Validate_MissingSchemaVersion(t *testing.T) {
	s := NewDefaultStrategy("Test")
	s.Metadata.SchemaVersion = ""

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema_version")
}

func TestStrategyConfig_Validate_InvalidSchemaVersion(t *testing.T) {
	s := NewDefaultStrategy("Test")
	s.Metadata.SchemaVersion = "99.0"

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported schema version")
}

func TestStrategyConfig_Validate_MissingName(t *testing.T) {
	s := NewDefaultStrategy("")

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestStrategyConfig_Validate_NameTooLong(t *testing.T) {
	s := NewDefaultStrategy(strings.Repeat("a", 101))

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "100 characters")
}

func TestStrategyConfig_Validate_InvalidAgentWeight(t *testing.T) {
	s := NewDefaultStrategy("Test")
	s.Agents.Weights.Technical = 1.5 // > 1

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "weight must be between 0 and 1")
}

func TestStrategyConfig_Validate_NoEnabledAgents(t *testing.T) {
	s := NewDefaultStrategy("Test")
	s.Agents.Enabled.Technical = false
	s.Agents.Enabled.Sentiment = false
	s.Agents.Enabled.Trend = false
	s.Agents.Enabled.OrderBook = false
	s.Agents.Enabled.Reversion = false
	s.Agents.Enabled.Arbitrage = false

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one trading agent")
}

func TestStrategyConfig_Validate_InvalidRiskSettings(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*StrategyConfig)
		errMsg string
	}{
		{
			name: "max_portfolio_exposure > 1",
			modify: func(s *StrategyConfig) {
				s.Risk.MaxPortfolioExposure = 1.5
			},
			errMsg: "max_portfolio_exposure",
		},
		{
			name: "max_daily_loss > 1",
			modify: func(s *StrategyConfig) {
				s.Risk.MaxDailyLoss = 1.5
			},
			errMsg: "max_daily_loss",
		},
		{
			name: "max_positions < 1",
			modify: func(s *StrategyConfig) {
				s.Risk.MaxPositions = 0
			},
			errMsg: "max_positions",
		},
		{
			name: "min_consensus_votes < 1",
			modify: func(s *StrategyConfig) {
				s.Risk.MinConsensusVotes = 0
			},
			errMsg: "min_consensus_votes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewDefaultStrategy("Test")
			tt.modify(s)

			err := s.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestStrategyConfig_Validate_InvalidIndicators(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*StrategyConfig)
		errMsg string
	}{
		{
			name: "RSI period < 1",
			modify: func(s *StrategyConfig) {
				s.Indicators.RSI.Period = 0
			},
			errMsg: "rsi.period",
		},
		{
			name: "RSI overbought < 50",
			modify: func(s *StrategyConfig) {
				s.Indicators.RSI.Overbought = 40
			},
			errMsg: "rsi.overbought",
		},
		{
			name: "RSI oversold >= overbought",
			modify: func(s *StrategyConfig) {
				s.Indicators.RSI.Oversold = 80
				s.Indicators.RSI.Overbought = 70
			},
			errMsg: "rsi.oversold",
		},
		{
			name: "MACD fast >= slow",
			modify: func(s *StrategyConfig) {
				s.Indicators.MACD.FastPeriod = 26
				s.Indicators.MACD.SlowPeriod = 12
			},
			errMsg: "macd.fast_period",
		},
		{
			name: "Bollinger std_dev <= 0",
			modify: func(s *StrategyConfig) {
				s.Indicators.Bollinger.StdDev = 0
			},
			errMsg: "bollinger.std_dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewDefaultStrategy("Test")
			tt.modify(s)

			err := s.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestStrategyConfig_ValidateQuick(t *testing.T) {
	s := NewDefaultStrategy("Test")
	err := s.ValidateQuick()
	assert.NoError(t, err)

	s.Metadata.SchemaVersion = ""
	err = s.ValidateQuick()
	assert.Error(t, err)
}

// =============================================================================
// Export Tests
// =============================================================================

func TestExport_YAML(t *testing.T) {
	s := NewDefaultStrategy("Export Test")

	opts := ExportOptions{
		Format:          FormatYAML,
		IncludeMetadata: true,
		PrettyPrint:     true,
		AddComments:     true,
	}

	data, err := Export(s, opts)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Check it contains YAML content
	assert.Contains(t, string(data), "metadata:")
	assert.Contains(t, string(data), "name: Export Test")
	assert.Contains(t, string(data), "schema_version:")
}

func TestExport_JSON(t *testing.T) {
	s := NewDefaultStrategy("Export Test")

	opts := ExportOptions{
		Format:          FormatJSON,
		IncludeMetadata: true,
		PrettyPrint:     true,
	}

	data, err := Export(s, opts)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Check it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "Export Test", result["metadata"].(map[string]interface{})["name"])
}

func TestExport_NilStrategy(t *testing.T) {
	_, err := Export(nil, DefaultExportOptions())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestExportToFile(t *testing.T) {
	s := NewDefaultStrategy("File Export Test")

	// Create temp directory
	tmpDir := t.TempDir()

	tests := []struct {
		name   string
		file   string
		format ExportFormat
	}{
		{"YAML file", "test.yaml", FormatYAML},
		{"JSON file", "test.json", FormatJSON},
		{"YML extension", "test.yml", FormatYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.file)

			err := ExportToFile(s, path, DefaultExportOptions())
			require.NoError(t, err)

			// Verify file exists
			_, err = os.Stat(path)
			assert.NoError(t, err)

			// Verify content
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.NotEmpty(t, data)
		})
	}
}

// =============================================================================
// Import Tests
// =============================================================================

func TestImport_YAML(t *testing.T) {
	yamlData := `
metadata:
  schema_version: "1.0"
  name: "Imported Strategy"
  description: "Test import"
agents:
  weights:
    technical: 0.3
    orderbook: 0.2
  enabled:
    technical: true
    risk: true
risk:
  max_portfolio_exposure: 0.8
  max_position_size: 0.1
  max_positions: 3
  max_daily_loss: 0.02
  max_drawdown: 0.1
  min_strategy_confidence: 0.7
  min_consensus_votes: 2
  default_stop_loss: 0.02
  default_take_profit: 0.05
orchestration:
  voting_enabled: true
  voting_method: "weighted_consensus"
  min_votes: 2
  quorum: 0.6
  step_interval: "30s"
  max_signal_age: "5m"
  min_consensus: 0.6
  min_confidence: 0.5
indicators:
  rsi:
    period: 14
    overbought: 70
    oversold: 30
  macd:
    fast_period: 12
    slow_period: 26
    signal_period: 9
  bollinger:
    period: 20
    std_dev: 2
  adx:
    period: 14
    trend_threshold: 25
`

	opts := DefaultImportOptions()
	s, err := Import([]byte(yamlData), opts)

	require.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, "Imported Strategy", s.Metadata.Name)
	assert.Equal(t, "Test import", s.Metadata.Description)
	assert.Equal(t, 0.3, s.Agents.Weights.Technical)
	assert.True(t, s.Agents.Enabled.Technical)
}

func TestImport_JSON(t *testing.T) {
	s := NewDefaultStrategy("JSON Test")
	jsonData, err := json.Marshal(s)
	require.NoError(t, err)

	opts := DefaultImportOptions()
	imported, err := Import(jsonData, opts)

	require.NoError(t, err)
	assert.NotNil(t, imported)
	assert.Equal(t, "JSON Test", imported.Metadata.Name)
}

func TestImport_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"invalid YAML", "::invalid::"},
		{"missing required fields", "metadata:\n  name: test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Import([]byte(tt.data), DefaultImportOptions())
			assert.Error(t, err)
		})
	}
}

func TestImport_GenerateNewID(t *testing.T) {
	s := NewDefaultStrategy("Test")
	originalID := s.Metadata.ID

	data, err := Export(s, DefaultExportOptions())
	require.NoError(t, err)

	// Import with new ID generation
	opts := DefaultImportOptions()
	opts.GenerateNewID = true

	imported, err := Import(data, opts)
	require.NoError(t, err)
	assert.NotEqual(t, originalID, imported.Metadata.ID)
}

func TestImport_OverrideMetadata(t *testing.T) {
	s := NewDefaultStrategy("Original")
	data, err := Export(s, DefaultExportOptions())
	require.NoError(t, err)

	opts := DefaultImportOptions()
	opts.OverrideMetadata = &StrategyMetadata{
		Name:        "Overridden",
		Description: "New description",
		Tags:        []string{"tag1", "tag2"},
	}

	imported, err := Import(data, opts)
	require.NoError(t, err)
	assert.Equal(t, "Overridden", imported.Metadata.Name)
	assert.Equal(t, "New description", imported.Metadata.Description)
	assert.Equal(t, []string{"tag1", "tag2"}, imported.Metadata.Tags)
}

func TestImportFromFile(t *testing.T) {
	s := NewDefaultStrategy("File Test")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "strategy.yaml")

	err := ExportToFile(s, path, DefaultExportOptions())
	require.NoError(t, err)

	imported, err := ImportFromFile(path, DefaultImportOptions())
	require.NoError(t, err)
	assert.Equal(t, "File Test", imported.Metadata.Name)
}

// =============================================================================
// Clone Tests
// =============================================================================

func TestClone(t *testing.T) {
	original := NewDefaultStrategy("Original")
	original.Metadata.Description = "Original description"
	original.Agents.Weights.Technical = 0.5

	cloned, err := Clone(original)
	require.NoError(t, err)

	// Check it's a deep copy
	assert.NotEqual(t, original.Metadata.ID, cloned.Metadata.ID)
	assert.Equal(t, original.Metadata.Name, cloned.Metadata.Name)
	assert.Equal(t, "clone", cloned.Metadata.Source)

	// Modify original and verify clone is unaffected
	original.Agents.Weights.Technical = 0.9
	assert.Equal(t, 0.5, cloned.Agents.Weights.Technical)
}

func TestClone_Nil(t *testing.T) {
	_, err := Clone(nil)
	assert.Error(t, err)
}

// =============================================================================
// Merge Tests
// =============================================================================

func TestMerge(t *testing.T) {
	base := NewDefaultStrategy("Base")
	base.Agents.Weights.Technical = 0.2

	override := &StrategyConfig{
		Metadata: StrategyMetadata{
			Name:        "Override",
			Description: "Override description",
		},
		Agents: AgentConfig{
			Weights: AgentWeights{
				Technical: 0.5, // Override
				Sentiment: 0.3, // Override
			},
			Enabled: EnabledAgents{
				Technical: true,
				Sentiment: true,
				Risk:      true,
			},
		},
	}

	merged, err := Merge(base, override)
	require.NoError(t, err)

	assert.Equal(t, "Override", merged.Metadata.Name)
	assert.Equal(t, "Override description", merged.Metadata.Description)
	assert.Equal(t, 0.5, merged.Agents.Weights.Technical)
	assert.Equal(t, 0.3, merged.Agents.Weights.Sentiment)
	assert.Equal(t, "merge", merged.Metadata.Source)
}

func TestMerge_NilBase(t *testing.T) {
	_, err := Merge(nil, NewDefaultStrategy("Override"))
	assert.Error(t, err)
}

func TestMerge_NilOverride(t *testing.T) {
	base := NewDefaultStrategy("Base")
	merged, err := Merge(base, nil)
	require.NoError(t, err)
	assert.Equal(t, "Base", merged.Metadata.Name)
}

// =============================================================================
// Version Tests
// =============================================================================

func TestGetSchemaVersion(t *testing.T) {
	assert.Equal(t, "1.0", GetSchemaVersion())
}

func TestIsVersionSupported(t *testing.T) {
	assert.True(t, IsVersionSupported("1.0"))
	assert.False(t, IsVersionSupported("99.0"))
}

func TestCheckCompatibility(t *testing.T) {
	s := NewDefaultStrategy("Test")
	err := CheckCompatibility(s)
	assert.NoError(t, err)

	// Future version
	s.Metadata.SchemaVersion = "99.0"
	err = CheckCompatibility(s)
	assert.Error(t, err)
}

func TestMigrate_AlreadyCurrent(t *testing.T) {
	s := NewDefaultStrategy("Test")
	err := Migrate(s)
	assert.NoError(t, err)
	assert.Equal(t, SchemaVersion, s.Metadata.SchemaVersion)
}

func TestMigrate_Nil(t *testing.T) {
	err := Migrate(nil)
	assert.Error(t, err)
}

func TestGetVersionInfo(t *testing.T) {
	s := NewDefaultStrategy("Test")
	s.Metadata.Version = "1.2.3"

	info, err := GetVersionInfo(s)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion, info.SchemaVersion)
	assert.Equal(t, "1.2.3", info.StrategyVersion)
	assert.True(t, info.IsCompatible)
	assert.False(t, info.RequiresMigration)
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "2.0", -1},
		{"2.0", "1.0", 1},
		{"1.0.0", "1.0.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result, err := CompareVersions(tt.a, tt.b)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Round-Trip Tests
// =============================================================================

func TestRoundTrip_YAMLExportImport(t *testing.T) {
	original := NewDefaultStrategy("Round Trip Test")
	original.Metadata.Description = "Testing round trip"
	original.Metadata.Tags = []string{"test", "roundtrip"}
	original.Agents.Weights.Technical = 0.35
	original.Risk.MaxDailyLoss = 0.03

	// Export
	opts := ExportOptions{
		Format:          FormatYAML,
		IncludeMetadata: true,
		PrettyPrint:     true,
	}
	data, err := Export(original, opts)
	require.NoError(t, err)

	// Import
	importOpts := DefaultImportOptions()
	importOpts.GenerateNewID = false // Keep same ID for comparison

	imported, err := Import(data, importOpts)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original.Metadata.Name, imported.Metadata.Name)
	assert.Equal(t, original.Metadata.Description, imported.Metadata.Description)
	assert.Equal(t, original.Agents.Weights.Technical, imported.Agents.Weights.Technical)
	assert.Equal(t, original.Risk.MaxDailyLoss, imported.Risk.MaxDailyLoss)
}

func TestRoundTrip_JSONExportImport(t *testing.T) {
	original := NewDefaultStrategy("JSON Round Trip")
	original.Agents.Trend = &TrendAgentConfig{
		StepInterval:  "5m",
		FastEMAPeriod: 9,
		SlowEMAPeriod: 21,
	}

	// Export as JSON
	opts := ExportOptions{
		Format:          FormatJSON,
		IncludeMetadata: true,
	}
	data, err := Export(original, opts)
	require.NoError(t, err)

	// Import
	imported, err := Import(data, DefaultImportOptions())
	require.NoError(t, err)

	assert.Equal(t, original.Metadata.Name, imported.Metadata.Name)
	require.NotNil(t, imported.Agents.Trend)
	assert.Equal(t, 9, imported.Agents.Trend.FastEMAPeriod)
	assert.Equal(t, 21, imported.Agents.Trend.SlowEMAPeriod)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkExport_YAML(b *testing.B) {
	s := NewDefaultStrategy("Benchmark")
	opts := DefaultExportOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Export(s, opts)
	}
}

func BenchmarkExport_JSON(b *testing.B) {
	s := NewDefaultStrategy("Benchmark")
	opts := ExportOptions{Format: FormatJSON}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Export(s, opts)
	}
}

func BenchmarkImport_YAML(b *testing.B) {
	s := NewDefaultStrategy("Benchmark")
	data, _ := Export(s, DefaultExportOptions())
	opts := DefaultImportOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Import(data, opts)
	}
}

func BenchmarkValidate(b *testing.B) {
	s := NewDefaultStrategy("Benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Validate()
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestValidationErrors_Multiple(t *testing.T) {
	s := NewDefaultStrategy("")
	s.Metadata.SchemaVersion = ""
	s.Agents.Weights.Technical = 2.0
	s.Risk.MaxDailyLoss = 2.0

	err := s.Validate()
	require.Error(t, err)

	// Should contain multiple errors
	errStr := err.Error()
	assert.Contains(t, errStr, "schema_version")
	assert.Contains(t, errStr, "name")
}

func TestStrategyMetadata_Timestamps(t *testing.T) {
	s := NewDefaultStrategy("Test")

	// CreatedAt should be set
	assert.False(t, s.Metadata.CreatedAt.IsZero())

	// Sleep briefly and update
	time.Sleep(10 * time.Millisecond)
	s.Metadata.UpdatedAt = time.Now()

	assert.True(t, s.Metadata.UpdatedAt.After(s.Metadata.CreatedAt))
}

func TestStrategyMetadata_UUID(t *testing.T) {
	s := NewDefaultStrategy("Test")

	// ID should be a valid UUID
	_, err := uuid.Parse(s.Metadata.ID)
	assert.NoError(t, err)
}
