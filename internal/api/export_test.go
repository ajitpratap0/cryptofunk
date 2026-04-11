package api

// Test-only helpers and TestMain hook for the api package.
//
// This file uses the `_test.go` suffix so none of its symbols are
// compiled into the production binary — they exist only when `go test`
// builds the test binary. Anything that reaches into package-level
// state solely for test coordination (draining async goroutines,
// resetting the rehash dedup map) belongs here, not in auth_middleware.go.

import (
	"os"
	"testing"
)

// waitForRehashesForTest blocks until every async key-op goroutine
// spawned by runAsyncKeyOps has returned. Used by pgxmock-based unit
// tests to prevent the async UPDATE from racing mock.Close() — the
// test drains the WaitGroup first, asserts ExpectationsWereMet, then
// closes the mock.
//
// This is the test-only counterpart to the production DrainAsyncKeyOps
// helper in auth_middleware.go. They deliberately share the
// asyncKeyOpsWG package-level var rather than taking separate
// arguments, so a new goroutine path added to runAsyncKeyOps is
// covered by both the shutdown drain and the test drain automatically.
func waitForRehashesForTest() {
	asyncKeyOpsWG.Wait()
}

// TestMain provides a package-level setup/teardown hook for the api
// package test binary. After running the suite it clears the
// process-global inFlightRehashes sync.Map so a future test that
// reuses a fixed UUID across subtests can't silently suppress a
// rehash in an unrelated run. Current tests use uuid.New() per
// subtest so the reset is defensive, but wiring it up now prevents
// a class of future flakes as the pattern grows.
func TestMain(m *testing.M) {
	code := m.Run()
	inFlightRehashes.Range(func(k, _ any) bool {
		inFlightRehashes.Delete(k)
		return true
	})
	os.Exit(code)
}
