package config

// Version is the canonical version of CryptoFunk
// This should be the single source of truth for all version references
const Version = "1.0.0"

// GetVersion returns the current version
func GetVersion() string {
	return Version
}
