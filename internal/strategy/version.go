package strategy

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// MigrationFunc defines a function that migrates a strategy from one version to another
type MigrationFunc func(*StrategyConfig) error

// migrations maps source version to migration functions
var migrations = map[string]MigrationFunc{
	// Example: "0.9" -> "1.0" migration
	// "0.9": migrateFrom09To10,
}

// Migrate upgrades a strategy configuration to the current schema version
func Migrate(strategy *StrategyConfig) error {
	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	// Already at current version
	if strategy.Metadata.SchemaVersion == SchemaVersion {
		return nil
	}

	// Parse versions for comparison
	current, err := semver.NewVersion(strategy.Metadata.SchemaVersion)
	if err != nil {
		// Try to handle simple version strings
		current, err = semver.NewVersion(strategy.Metadata.SchemaVersion + ".0")
		if err != nil {
			return fmt.Errorf("invalid schema version: %s", strategy.Metadata.SchemaVersion)
		}
	}

	target, err := semver.NewVersion(SchemaVersion)
	if err != nil {
		return fmt.Errorf("invalid target schema version: %s", SchemaVersion)
	}

	// Check if version is newer than supported
	if current.GreaterThan(target) {
		return fmt.Errorf("strategy schema version %s is newer than supported version %s",
			strategy.Metadata.SchemaVersion, SchemaVersion)
	}

	// Apply migrations in order
	for version, migrate := range migrations {
		migrationVersion, err := semver.NewVersion(version)
		if err != nil {
			continue
		}

		// Apply migration if current version is less than migration version
		if current.LessThan(migrationVersion) {
			if err := migrate(strategy); err != nil {
				return fmt.Errorf("migration from %s failed: %w", version, err)
			}
		}
	}

	// Update to current version
	strategy.Metadata.SchemaVersion = SchemaVersion

	return nil
}

// CheckCompatibility checks if a strategy can be migrated to the current version
func CheckCompatibility(strategy *StrategyConfig) error {
	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	if strategy.Metadata.SchemaVersion == "" {
		return fmt.Errorf("missing schema version")
	}

	// Parse versions
	current, err := semver.NewVersion(strategy.Metadata.SchemaVersion)
	if err != nil {
		current, err = semver.NewVersion(strategy.Metadata.SchemaVersion + ".0")
		if err != nil {
			return fmt.Errorf("invalid schema version: %s", strategy.Metadata.SchemaVersion)
		}
	}

	target, err := semver.NewVersion(SchemaVersion)
	if err != nil {
		return fmt.Errorf("invalid target schema version: %s", SchemaVersion)
	}

	// Version is newer than supported
	if current.GreaterThan(target) {
		return fmt.Errorf("strategy requires schema version %s, but only %s is supported",
			strategy.Metadata.SchemaVersion, SchemaVersion)
	}

	// Check if migration path exists for older versions
	if current.LessThan(target) {
		// For now, we support direct migration from any 1.x version
		if current.Major() != target.Major() {
			return fmt.Errorf("no migration path from version %s to %s",
				strategy.Metadata.SchemaVersion, SchemaVersion)
		}
	}

	return nil
}

// GetSchemaVersion returns the current schema version
func GetSchemaVersion() string {
	return SchemaVersion
}

// CompareVersions compares two version strings
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func CompareVersions(a, b string) (int, error) {
	va, err := semver.NewVersion(a)
	if err != nil {
		va, err = semver.NewVersion(a + ".0")
		if err != nil {
			return 0, fmt.Errorf("invalid version: %s", a)
		}
	}

	vb, err := semver.NewVersion(b)
	if err != nil {
		vb, err = semver.NewVersion(b + ".0")
		if err != nil {
			return 0, fmt.Errorf("invalid version: %s", b)
		}
	}

	return va.Compare(vb), nil
}

// IsVersionSupported checks if a schema version is supported
func IsVersionSupported(version string) bool {
	for _, v := range SupportedSchemaVersions {
		if v == version {
			return true
		}
	}

	// Also check using semver comparison for patch versions
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	for _, supported := range SupportedSchemaVersions {
		sv, err := semver.NewVersion(supported)
		if err != nil {
			continue
		}
		// Consider compatible if major.minor match
		if v.Major() == sv.Major() && v.Minor() == sv.Minor() {
			return true
		}
	}

	return false
}

// VersionInfo contains version information for a strategy
type VersionInfo struct {
	SchemaVersion     string `json:"schema_version"`
	StrategyVersion   string `json:"strategy_version,omitempty"`
	IsCompatible      bool   `json:"is_compatible"`
	RequiresMigration bool   `json:"requires_migration"`
	MigrationPath     string `json:"migration_path,omitempty"`
}

// GetVersionInfo returns version information for a strategy
func GetVersionInfo(strategy *StrategyConfig) (*VersionInfo, error) {
	if strategy == nil {
		return nil, fmt.Errorf("strategy cannot be nil")
	}

	info := &VersionInfo{
		SchemaVersion:   strategy.Metadata.SchemaVersion,
		StrategyVersion: strategy.Metadata.Version,
	}

	// Check compatibility
	err := CheckCompatibility(strategy)
	info.IsCompatible = err == nil

	// Check if migration is needed
	if strategy.Metadata.SchemaVersion != SchemaVersion {
		cmp, err := CompareVersions(strategy.Metadata.SchemaVersion, SchemaVersion)
		if err == nil && cmp < 0 {
			info.RequiresMigration = true
			info.MigrationPath = fmt.Sprintf("%s -> %s", strategy.Metadata.SchemaVersion, SchemaVersion)
		}
	}

	return info, nil
}
