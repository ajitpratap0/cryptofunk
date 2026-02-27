// Package testhelpers provides shared test utilities for CryptoFunk test files.
// This package is intended to be imported only from _test.go files.
package testhelpers

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// ReadSSEData reads a single SSE response from an HTTP response body and
// returns the JSON data field. For the MCP Streamable HTTP transport,
// responses are delivered as SSE events in the format:
//
//	event: message
//	id: <event-id>
//	data: <json-payload>
//
// Calls t.Fatal if no data line is found, to catch test bugs early.
// This function is extracted here to avoid duplication across test packages
// (m6: was readSSEData in internal/mcp and readSSELine in cmd/mcp-servers/polymarket).
func ReadSSEData(t *testing.T, body io.Reader) json.RawMessage {
	t.Helper()
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			return json.RawMessage(data)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading SSE response: %v", err)
	}
	t.Fatal("no SSE data line found in response")
	return nil
}
