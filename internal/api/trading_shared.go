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
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// UserError is a sentinel type for errors that are safe to surface directly
// to API clients. Only errors explicitly constructed as *UserError will have
// their message forwarded; all other errors (DB errors, internal errors, etc.)
// are replaced with a generic message.
//
// Example:
//
//	return nil, &api.UserError{Message: "symbol is required"}
type UserError struct {
	Message string
}

func (e *UserError) Error() string { return e.Message }

// SanitizeError returns a safe message for API responses using an allowlist
// (sentinel) approach rather than a denylist. Only errors that are explicitly
// typed as *UserError are forwarded to the client; everything else — DB driver
// errors, internal errors, pgx/pgconn messages — returns a generic string.
//
// This eliminates both false negatives (pgx wrapped errors leaking through
// without known keywords) and false positives (legitimate messages suppressed
// by over-broad keyword matching).
//
// Callers should use this wherever an error is returned in a 5xx JSON response:
//
//	c.JSON(http.StatusInternalServerError, gin.H{"error": SanitizeError(err)})
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	var userErr *UserError
	if errors.As(err, &userErr) {
		return userErr.Message // explicitly safe, user-facing message
	}
	// All other error types (DB errors, internal errors) → generic message
	return "internal server error"
}

// ParseSessionID is a shared helper for parsing session IDs in trading requests.
func ParseSessionID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session_id format: %w", err)
	}
	return id, nil
}
