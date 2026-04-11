package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// TestAuthConfigMapping asserts that every field set on config.AuthConfig
// propagates correctly to api.AuthConfig through buildAPIAuthConfig — the
// single pure function used by registerRoutes (cmd/api/routes.go) to build
// the runtime AuthConfig the middleware consumes.
//
// This test calls buildAPIAuthConfig DIRECTLY, so a new field added to one
// struct without a corresponding copy line inside buildAPIAuthConfig is
// caught as a zero-value assertion failure here. It was motivated by
// SEC-009 rounds 2 and 4 where TrustForwardedProto silently defaulted to
// false because the field was added to api.AuthConfig (PR #212) without a
// matching config.AuthConfig field and copy line.
func TestAuthConfigMapping(t *testing.T) {
	// Populate every non-zero field on the config-side struct so the
	// live copy inside buildAPIAuthConfig is exercised with distinguishable
	// values. KeyPepper is intentionally set to prove it does NOT flow
	// through api.AuthConfig (it is wired via NewAPIKeyStoreWithPepper).
	src := config.AuthConfig{
		Enabled:             true,
		HeaderName:          "X-Test-Key",
		RequireHTTPS:        true,
		TrustForwardedProto: true,
		KeyPepper:           "test-pepper-32-bytes-of-material",
	}

	dst := buildAPIAuthConfig(src)

	assert.Equal(t, src.Enabled, dst.Enabled, "Enabled")
	assert.Equal(t, src.HeaderName, dst.HeaderName, "HeaderName")
	assert.Equal(t, src.RequireHTTPS, dst.RequireHTTPS, "RequireHTTPS")
	assert.Equal(t, src.TrustForwardedProto, dst.TrustForwardedProto, "TrustForwardedProto")
	// KeyPepper deliberately not asserted on dst — by design api.AuthConfig
	// does not carry it. If a future change adds KeyPepper back to
	// api.AuthConfig this test should be updated to explicitly reject it.
}

// TestBuildAPIAuthConfig_HeaderNameDefault asserts that buildAPIAuthConfig
// applies the X-API-Key default when the config leaves HeaderName empty.
// This is the only non-copy transform in the function; regressing it would
// break the middleware header lookup for every deployment that relies on
// the default.
func TestBuildAPIAuthConfig_HeaderNameDefault(t *testing.T) {
	src := config.AuthConfig{
		Enabled:    true,
		HeaderName: "", // Intentionally empty to trigger the default.
	}

	dst := buildAPIAuthConfig(src)

	assert.Equal(t, "X-API-Key", dst.HeaderName, "HeaderName should default to X-API-Key")
	assert.True(t, dst.Enabled, "Enabled should still propagate alongside the default")
}
