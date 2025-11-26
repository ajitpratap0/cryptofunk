package strategy

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// MigrationFunc defines a function that migrates a strategy from one version to another
type MigrationFunc func(*StrategyConfig) error

// Migration represents a single schema migration
type Migration struct {
	FromVersion string        // Source version
	ToVersion   string        // Target version
	Name        string        // Human-readable name
	Migrate     MigrationFunc // Migration function
}

// registeredMigrations holds all registered migrations in order
var registeredMigrations []Migration

// migrations maps source version to migration functions (legacy support)
var migrations = map[string]MigrationFunc{
	// Example: "0.9" -> "1.0" migration
	// "0.9": migrateFrom09To10,
}

func init() {
	// Register all migrations in order
	registerMigrations()
}

// registerMigrations sets up all known migrations.
//
// Migration Infrastructure Design:
// - Migrations are registered in chronological order (oldest to newest)
// - Each migration transforms a strategy from one schema version to the next
// - The Migrate function applies migrations sequentially based on version comparison
// - GetMigrationPath can be used to preview which migrations will be applied
//
// To add a new migration:
// 1. Add a new Migration struct to registeredMigrations below
// 2. Implement the migration function (e.g., migrateFrom10To11)
// 3. Update SchemaVersion constant to the new version
//
// Note: The legacy 'migrations' map is maintained for backward compatibility
// but new code should use registeredMigrations and GetMigrationPath.
func registerMigrations() {
	registeredMigrations = []Migration{
		{
			FromVersion: "0.9",
			ToVersion:   "1.0",
			Name:        "Add strategy metadata fields",
			Migrate:     migrateFrom09To10,
		},
		// Future migrations can be added here:
		// {
		//     FromVersion: "1.0",
		//     ToVersion:   "1.1",
		//     Name:        "Add new indicator configurations",
		//     Migrate:     migrateFrom10To11,
		// },
	}

	// Validate migrations at initialization time to catch configuration errors early.
	// Using panic() instead of log.Fatal() for two reasons:
	// 1. Testability: panic can be recovered in tests, while log.Fatal calls os.Exit(1)
	// 2. Deferred cleanup: panic runs deferred functions, log.Fatal does not
	// This is a deliberate fail-fast pattern - invalid migration config should crash at startup.
	for _, m := range registeredMigrations {
		if _, err := semver.NewVersion(m.FromVersion); err != nil {
			panic(fmt.Sprintf("invalid FromVersion %q in migration %q: %v", m.FromVersion, m.Name, err))
		}
		if _, err := semver.NewVersion(m.ToVersion); err != nil {
			panic(fmt.Sprintf("invalid ToVersion %q in migration %q: %v", m.ToVersion, m.Name, err))
		}
	}

	// Validate migration path continuity - ensure no gaps in the migration chain
	if len(registeredMigrations) > 1 {
		for i := 1; i < len(registeredMigrations); i++ {
			prevTo := registeredMigrations[i-1].ToVersion
			currFrom := registeredMigrations[i].FromVersion
			if prevTo != currFrom {
				panic(fmt.Sprintf("migration gap detected: %q ends at %s but %q starts at %s",
					registeredMigrations[i-1].Name, prevTo, registeredMigrations[i].Name, currFrom))
			}
		}
	}

	// Also populate legacy map for backward compatibility
	for _, m := range registeredMigrations {
		migrations[m.FromVersion] = m.Migrate
	}
}

// migrateFrom09To10 migrates strategy from schema version 0.9 to 1.0
func migrateFrom09To10(s *StrategyConfig) error {
	// Set default values for new fields introduced in 1.0
	if s.Metadata.Source == "" {
		s.Metadata.Source = "migrated"
	}

	// Ensure risk parameters have valid defaults (check for zero or negative values)
	if s.Risk.MinPositionUSD <= 0 {
		s.Risk.MinPositionUSD = 10.0
	}
	if s.Risk.MaxPositionUSD <= 0 {
		s.Risk.MaxPositionUSD = 100000.0
	}

	// Ensure orchestration has valid defaults (check for zero or negative values)
	if s.Orchestration.MinConfidence <= 0 {
		s.Orchestration.MinConfidence = 0.6
	}
	if s.Orchestration.MinConsensus <= 0 {
		s.Orchestration.MinConsensus = 0.5
	}

	return nil
}

// GetMigrationPath returns the list of migrations needed to upgrade from one version to another
func GetMigrationPath(fromVersion, toVersion string) ([]Migration, error) {
	from, err := semver.NewVersion(fromVersion)
	if err != nil {
		from, err = semver.NewVersion(fromVersion + ".0")
		if err != nil {
			return nil, fmt.Errorf("invalid from version: %s", fromVersion)
		}
	}

	to, err := semver.NewVersion(toVersion)
	if err != nil {
		to, err = semver.NewVersion(toVersion + ".0")
		if err != nil {
			return nil, fmt.Errorf("invalid to version: %s", toVersion)
		}
	}

	if from.GreaterThan(to) || from.Equal(to) {
		return nil, nil // No migrations needed
	}

	// Find applicable migrations
	// Note: Migration versions are validated at init() time, so semver.NewVersion
	// will not fail here. We use MustParse since validation already passed.
	var path []Migration
	for _, m := range registeredMigrations {
		// These are guaranteed valid by registerMigrations() validation
		migFrom := semver.MustParse(m.FromVersion)
		migTo := semver.MustParse(m.ToVersion)

		// Include migration if it falls within our upgrade range:
		// - Migration starts at or after our source version (migFrom >= from)
		// - Migration ends at or before our target version (migTo <= to)
		startsAtOrAfterSource := migFrom.GreaterThan(from) || migFrom.Equal(from)
		endsAtOrBeforeTarget := migTo.LessThan(to) || migTo.Equal(to)
		if startsAtOrAfterSource && endsAtOrBeforeTarget {
			path = append(path, m)
		}
	}

	// Sort migrations by version
	sort.Slice(path, func(i, j int) bool {
		vi := semver.MustParse(path[i].FromVersion)
		vj := semver.MustParse(path[j].FromVersion)
		return vi.LessThan(vj)
	})

	return path, nil
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
	// Note: Migration versions are validated at init() time via registerMigrations()
	for version, migrate := range migrations {
		// Guaranteed valid by registerMigrations() validation
		migrationVersion := semver.MustParse(version)

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
