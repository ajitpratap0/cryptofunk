package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMigrationPath(t *testing.T) {
	tests := []struct {
		name        string
		fromVersion string
		toVersion   string
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name:        "same version returns empty path",
			fromVersion: "1.0",
			toVersion:   "1.0",
			wantCount:   0,
			wantErr:     false,
		},
		{
			name:        "newer to older returns empty path",
			fromVersion: "2.0",
			toVersion:   "1.0",
			wantCount:   0,
			wantErr:     false,
		},
		{
			name:        "upgrade from 0.9 to 1.0",
			fromVersion: "0.9",
			toVersion:   "1.0",
			wantCount:   1,
			wantErr:     false,
		},
		{
			name:        "invalid from version",
			fromVersion: "invalid",
			toVersion:   "1.0",
			wantCount:   0,
			wantErr:     true,
			errContains: "invalid from version",
		},
		{
			name:        "invalid to version",
			fromVersion: "1.0",
			toVersion:   "invalid",
			wantCount:   0,
			wantErr:     true,
			errContains: "invalid to version",
		},
		{
			name:        "handles version with .0 suffix",
			fromVersion: "0.9.0",
			toVersion:   "1.0.0",
			wantCount:   1,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetMigrationPath(tt.fromVersion, tt.toVersion)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Len(t, path, tt.wantCount)
		})
	}
}

func TestGetMigrationPath_MigrationOrder(t *testing.T) {
	// When upgrading across multiple versions, migrations should be in order
	path, err := GetMigrationPath("0.9", "1.0")
	require.NoError(t, err)

	if len(path) > 1 {
		// Verify migrations are ordered by FromVersion
		for i := 1; i < len(path); i++ {
			cmp, err := CompareVersions(path[i-1].FromVersion, path[i].FromVersion)
			require.NoError(t, err)
			assert.LessOrEqual(t, cmp, 0, "migrations should be in ascending version order")
		}
	}
}

func TestGetMigrationPath_ReturnsCorrectMigration(t *testing.T) {
	path, err := GetMigrationPath("0.9", "1.0")
	require.NoError(t, err)
	require.Len(t, path, 1)

	assert.Equal(t, "0.9", path[0].FromVersion)
	assert.Equal(t, "1.0", path[0].ToVersion)
	assert.Equal(t, "Add strategy metadata fields", path[0].Name)
	assert.NotNil(t, path[0].Migrate)
}

func TestMigrateFrom09To10(t *testing.T) {
	// Test the 0.9 to 1.0 migration sets defaults correctly
	s := &StrategyConfig{
		Metadata: StrategyMetadata{
			SchemaVersion: "0.9",
			Name:          "test",
		},
		Risk:          RiskSettings{},
		Orchestration: OrchestrationSettings{},
	}

	err := migrateFrom09To10(s)
	require.NoError(t, err)

	// Check defaults were applied
	assert.Equal(t, "migrated", s.Metadata.Source)
	assert.Equal(t, 10.0, s.Risk.MinPositionUSD)
	assert.Equal(t, 100000.0, s.Risk.MaxPositionUSD)
	assert.Equal(t, 0.6, s.Orchestration.MinConfidence)
	assert.Equal(t, 0.5, s.Orchestration.MinConsensus)
}

func TestMigrateFrom09To10_PreservesExistingValues(t *testing.T) {
	// Test that migration preserves existing non-zero values
	s := &StrategyConfig{
		Metadata: StrategyMetadata{
			SchemaVersion: "0.9",
			Name:          "test",
			Source:        "custom-source",
		},
		Risk: RiskSettings{
			MinPositionUSD: 50.0,
			MaxPositionUSD: 50000.0,
		},
		Orchestration: OrchestrationSettings{
			MinConfidence: 0.8,
			MinConsensus:  0.7,
		},
	}

	err := migrateFrom09To10(s)
	require.NoError(t, err)

	// Existing values should be preserved
	assert.Equal(t, "custom-source", s.Metadata.Source)
	assert.Equal(t, 50.0, s.Risk.MinPositionUSD)
	assert.Equal(t, 50000.0, s.Risk.MaxPositionUSD)
	assert.Equal(t, 0.8, s.Orchestration.MinConfidence)
	assert.Equal(t, 0.7, s.Orchestration.MinConsensus)
}

func TestMigrateFrom09To10_HandlesNegativeValues(t *testing.T) {
	// Test that migration handles negative values (treats them as invalid)
	s := &StrategyConfig{
		Metadata: StrategyMetadata{
			SchemaVersion: "0.9",
			Name:          "test",
		},
		Risk: RiskSettings{
			MinPositionUSD: -10.0, // Invalid negative value
			MaxPositionUSD: -100.0,
		},
		Orchestration: OrchestrationSettings{
			MinConfidence: -0.5,
			MinConsensus:  -0.5,
		},
	}

	err := migrateFrom09To10(s)
	require.NoError(t, err)

	// Negative values should be replaced with defaults
	assert.Equal(t, 10.0, s.Risk.MinPositionUSD)
	assert.Equal(t, 100000.0, s.Risk.MaxPositionUSD)
	assert.Equal(t, 0.6, s.Orchestration.MinConfidence)
	assert.Equal(t, 0.5, s.Orchestration.MinConsensus)
}

func TestMigrate_AppliesVersionUpgrade(t *testing.T) {
	s := &StrategyConfig{
		Metadata: StrategyMetadata{
			SchemaVersion: "0.9",
			Name:          "test",
		},
	}

	err := Migrate(s)
	require.NoError(t, err)

	// Should be updated to current version
	assert.Equal(t, SchemaVersion, s.Metadata.SchemaVersion)
}
