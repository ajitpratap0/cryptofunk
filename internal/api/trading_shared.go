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

	"github.com/google/uuid"
)

// ParseSessionID is a shared helper for parsing session IDs in trading requests.
func ParseSessionID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session_id format: %w", err)
	}
	return id, nil
}
