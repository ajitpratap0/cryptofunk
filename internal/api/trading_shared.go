// Package api - trading_shared.go documents the relationship between the two
// trading control route groups and provides shared validation helpers.
//
// Route groups:
//
//	/api/v1/trade/*              (cmd/api/main.go) — calls orchestrator via HTTP
//	/api/v1/dashboard/trading/*  (internal/api/dashboard.go) — uses direct Go interface
//
// Both are kept for backward compatibility. They differ in orchestrator integration
// (HTTP vs direct), but share the same DB operations. New clients should prefer
// the /api/v1/dashboard/trading/* routes.
package api

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// SanitizeError prevents internal database and driver error strings from being
// leaked to HTTP clients (#116). Raw pgx / postgres error messages can reveal
// table names, column names, and query fragments that aid attackers.
//
// Callers should use this wherever an error is returned in a 5xx JSON response:
//
//	c.JSON(http.StatusInternalServerError, gin.H{"error": SanitizeError(err)})
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Detect well-known DB driver prefixes / substrings
	dbPrefixes := []string{
		"ERROR:", "pgx:", "pgconn:", "pq:", "SQLSTATE",
		"db:", "sql:", "exec", "query",
	}
	for _, prefix := range dbPrefixes {
		if strings.Contains(msg, prefix) {
			return "internal server error"
		}
	}
	return msg
}

// ParseSessionID is a shared helper for parsing session IDs in trading requests.
func ParseSessionID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session_id format: %w", err)
	}
	return id, nil
}
