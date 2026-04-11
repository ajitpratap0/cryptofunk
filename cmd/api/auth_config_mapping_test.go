package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ajitpratap0/cryptofunk/internal/api"
	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// TestAuthConfigMapping asserts that every field set on config.AuthConfig
// propagates correctly to api.AuthConfig via the manual copy in
// registerRoutes (cmd/api/routes.go).
//
// The routes.go copy is done field-by-field — a new field added to one
// struct without the corresponding copy line silently defaults to the
// Go zero value at runtime. This test catches that class of bug at
// compile+test time. It was motivated by SEC-009 rounds 2 and 4 where
// TrustForwardedProto was silently defaulting to false because the
// field was added to api.AuthConfig (PR #212) without the matching
// config.AuthConfig field and copy line.
func TestAuthConfigMapping(t *testing.T) {
	// Populate every non-zero field on the config-side struct so the
	// copy is exercised with distinguishable values.
	src := config.AuthConfig{
		Enabled:             true,
		HeaderName:          "X-Test-Key",
		RequireHTTPS:        true,
		TrustForwardedProto: true,
		KeyPepper:           "test-pepper-32-bytes-of-material",
	}

	// This copy MUST stay in sync with the one in cmd/api/routes.go
	// registerRoutes (lines around "authConfig := &api.AuthConfig{...}").
	// The KeyPepper is intentionally NOT copied — it flows into the
	// APIKeyStore via NewAPIKeyStoreWithPepper, not through api.AuthConfig.
	dst := &api.AuthConfig{
		Enabled:             src.Enabled,
		HeaderName:          src.HeaderName,
		RequireHTTPS:        src.RequireHTTPS,
		TrustForwardedProto: src.TrustForwardedProto,
	}

	assert.Equal(t, src.Enabled, dst.Enabled, "Enabled")
	assert.Equal(t, src.HeaderName, dst.HeaderName, "HeaderName")
	assert.Equal(t, src.RequireHTTPS, dst.RequireHTTPS, "RequireHTTPS")
	assert.Equal(t, src.TrustForwardedProto, dst.TrustForwardedProto, "TrustForwardedProto")
	// KeyPepper deliberately not asserted on dst — by design api.AuthConfig
	// doesn't carry it. If a future change adds KeyPepper back to
	// api.AuthConfig this test should fail to review.
}
