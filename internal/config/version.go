package config

// Version is the canonical version of CryptoFunk
// This should be the single source of truth for all version references
// Can be overwritten at build time using ldflags:
//
//	go build -ldflags "-X github.com/ajitpratap0/cryptofunk/internal/config.Version=$(git describe --tags --always)"
var Version = "dev"

// BuildTime is the time the binary was built (set via ldflags)
var BuildTime = "unknown"

// GitCommit is the git commit hash (set via ldflags)
var GitCommit = "unknown"

// GetVersion returns the current version
func GetVersion() string {
	return Version
}

// GetBuildInfo returns build information as a map
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
}
