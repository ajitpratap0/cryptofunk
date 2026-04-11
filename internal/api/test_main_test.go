package api

import (
	"os"
	"testing"
)

// TestMain provides a package-level setup/teardown hook. Today it only
// runs the test suite directly, but the file exists so future test
// authors have a single place to reset package-level state between
// runs — notably inFlightRehashes and asyncKeyOpsWG from the SEC-009
// HMAC pepper rollout (#123).
//
// Neither of those currently needs per-run reset because every
// existing test uses fresh uuid.New() key IDs, so inFlightRehashes
// never has a key collision across subtests, and asyncKeyOpsWG is
// drained explicitly via waitForRehashesForTest() before mock.Close().
// But if a future test constructs a fixed UUID as a fixture, this is
// where the reset hook belongs:
//
//	func TestMain(m *testing.M) {
//	    code := m.Run()
//	    inFlightRehashes.Range(func(k, _ any) bool {
//	        inFlightRehashes.Delete(k)
//	        return true
//	    })
//	    os.Exit(code)
//	}
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
