// Package api provides API key lifecycle management for the CryptoFunk trading system.
//
// This file implements TB-006: API Key Expiration and Rotation
//
// Key features:
//   - Create API keys with optional expiration dates
//   - Rotate API keys (create new key, deactivate old one)
//   - Validate API keys checking expiration and active status
//   - Background cleanup job to deactivate expired keys
//   - Revoke API keys
//
// Usage:
//
//	keyManager := api.NewKeyManager(dbPool)
//
//	// Create a key with 30-day expiration
//	key, err := keyManager.CreateAPIKey(ctx, "admin", "My API Key", nil, 30*24*time.Hour)
//
//	// Rotate an existing key
//	newKey, err := keyManager.RotateAPIKey(ctx, oldKeyID, nil)
//
//	// Start background cleanup (runs every hour)
//	keyManager.StartCleanupWorker(ctx, time.Hour)
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// KeyManagerDB is an interface for database operations used by KeyManager.
// This allows for mocking in tests while supporting both pgxpool.Pool and pgxmock.
type KeyManagerDB interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// KeyManager handles API key lifecycle operations
type KeyManager struct {
	db            KeyManagerDB
	cleanupCancel context.CancelFunc
	cleanupWG     sync.WaitGroup
	mu            sync.Mutex
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager(db KeyManagerDB) *KeyManager {
	return &KeyManager{
		db: db,
	}
}

// NewKeyManagerWithPool creates a new KeyManager with a pgxpool.Pool
// This is a convenience function for production code.
func NewKeyManagerWithPool(db *pgxpool.Pool) *KeyManager {
	return &KeyManager{
		db: db,
	}
}

// APIKeyDetails represents the full details of an API key
type APIKeyDetails struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	IsActive    bool       `json:"is_active"`
	Revoked     bool       `json:"revoked"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RotatedFrom *uuid.UUID `json:"rotated_from,omitempty"`
}

// CreatedAPIKey represents a newly created API key with the plaintext key
// The plaintext key is only available at creation time
type CreatedAPIKey struct {
	ID          uuid.UUID  `json:"id"`
	Key         string     `json:"key"` // Plaintext key - only shown once
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RotatedFrom *uuid.UUID `json:"rotated_from,omitempty"`
}

// Common errors
var (
	ErrKeyNotFound       = errors.New("API key not found")
	ErrKeyRevoked        = errors.New("API key is revoked")
	ErrKeyInactive       = errors.New("API key is inactive")
	ErrKeyExpired        = errors.New("API key has expired")
	ErrInvalidExpiration = errors.New("expiration must be in the future")
)

// CreateAPIKey creates a new API key with optional expiration
//
// Parameters:
//   - userID: Owner identifier for the key
//   - name: Human-readable name for the key
//   - permissions: Slice of permission strings (nil defaults to ["read:decisions"])
//   - expiresIn: Duration until key expires (0 = no expiration)
//
// Returns the created key including the plaintext key (only available at creation)
func (km *KeyManager) CreateAPIKey(
	ctx context.Context,
	userID string,
	name string,
	permissions []string,
	expiresIn time.Duration,
) (*CreatedAPIKey, error) {
	// Generate a secure random API key (32 bytes = 64 hex chars)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	plaintextKey := hex.EncodeToString(keyBytes)

	// Hash the key for storage
	keyHash := HashAPIKey(plaintextKey)

	// Default permissions if not specified
	if permissions == nil {
		permissions = []string{"read:decisions"}
	}

	// Calculate expiration time
	var expiresAt *time.Time
	if expiresIn > 0 {
		exp := time.Now().Add(expiresIn)
		expiresAt = &exp
	}

	// Marshal permissions to JSON
	permissionsJSON, err := json.Marshal(permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal permissions: %w", err)
	}

	// Insert the key
	query := `
		INSERT INTO api_keys (key_hash, name, user_id, permissions, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		RETURNING id, created_at
	`

	var id uuid.UUID
	var createdAt time.Time

	err = km.db.QueryRow(ctx, query, keyHash, name, userID, permissionsJSON, expiresAt).
		Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert API key: %w", err)
	}

	log.Info().
		Str("key_id", id.String()).
		Str("user_id", userID).
		Str("name", name).
		Bool("has_expiration", expiresAt != nil).
		Msg("API key created")

	return &CreatedAPIKey{
		ID:          id,
		Key:         plaintextKey,
		Name:        name,
		UserID:      userID,
		Permissions: permissions,
		CreatedAt:   createdAt,
		ExpiresAt:   expiresAt,
	}, nil
}

// RotateAPIKey creates a new API key from an existing one and deactivates the old key
//
// Parameters:
//   - oldKeyID: UUID of the key to rotate
//   - newExpiresIn: Duration until new key expires (nil = inherit from old key)
//
// Returns the new key including the plaintext key (only available at creation)
func (km *KeyManager) RotateAPIKey(
	ctx context.Context,
	oldKeyID uuid.UUID,
	newExpiresIn *time.Duration,
) (*CreatedAPIKey, error) {
	// Start a transaction
	tx, err := km.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Get the old key details
	var oldKey APIKeyDetails
	var permissionsJSON []byte

	query := `
		SELECT id, name, user_id, permissions, is_active, revoked, expires_at
		FROM api_keys
		WHERE id = $1
		FOR UPDATE
	`

	err = tx.QueryRow(ctx, query, oldKeyID).Scan(
		&oldKey.ID,
		&oldKey.Name,
		&oldKey.UserID,
		&permissionsJSON,
		&oldKey.IsActive,
		&oldKey.Revoked,
		&oldKey.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to get old key: %w", err)
	}

	// Validate old key state
	if oldKey.Revoked {
		return nil, ErrKeyRevoked
	}
	if !oldKey.IsActive {
		return nil, ErrKeyInactive
	}

	// Parse permissions
	if err := json.Unmarshal(permissionsJSON, &oldKey.Permissions); err != nil {
		return nil, fmt.Errorf("failed to parse permissions: %w", err)
	}

	// Generate new key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	plaintextKey := hex.EncodeToString(keyBytes)
	keyHash := HashAPIKey(plaintextKey)

	// Calculate new expiration
	var newExpiresAt *time.Time
	if newExpiresIn != nil {
		exp := time.Now().Add(*newExpiresIn)
		newExpiresAt = &exp
	} else {
		newExpiresAt = oldKey.ExpiresAt
	}

	// Create new key name
	newName := oldKey.Name + " (rotated)"

	// Deactivate old key
	_, err = tx.Exec(ctx, "UPDATE api_keys SET is_active = FALSE WHERE id = $1", oldKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate old key: %w", err)
	}

	// Insert new key
	insertQuery := `
		INSERT INTO api_keys (key_hash, name, user_id, permissions, expires_at, rotated_from, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, TRUE)
		RETURNING id, created_at
	`

	var newID uuid.UUID
	var createdAt time.Time

	err = tx.QueryRow(ctx, insertQuery, keyHash, newName, oldKey.UserID, permissionsJSON, newExpiresAt, oldKeyID).
		Scan(&newID, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert new key: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("old_key_id", oldKeyID.String()).
		Str("new_key_id", newID.String()).
		Str("user_id", oldKey.UserID).
		Msg("API key rotated")

	return &CreatedAPIKey{
		ID:          newID,
		Key:         plaintextKey,
		Name:        newName,
		UserID:      oldKey.UserID,
		Permissions: oldKey.Permissions,
		CreatedAt:   createdAt,
		ExpiresAt:   newExpiresAt,
		RotatedFrom: &oldKeyID,
	}, nil
}

// ValidateAPIKey checks if an API key is valid
// This is a more comprehensive check than the basic auth middleware
//
// Returns the key details if valid, or an appropriate error
func (km *KeyManager) ValidateAPIKey(ctx context.Context, plaintextKey string) (*APIKeyDetails, error) {
	keyHash := HashAPIKey(plaintextKey)

	query := `
		SELECT id, name, user_id, permissions, is_active, revoked,
		       created_at, expires_at, last_used_at, rotated_from
		FROM api_keys
		WHERE key_hash = $1
	`

	var key APIKeyDetails
	var permissionsJSON []byte

	err := km.db.QueryRow(ctx, query, keyHash).Scan(
		&key.ID,
		&key.Name,
		&key.UserID,
		&permissionsJSON,
		&key.IsActive,
		&key.Revoked,
		&key.CreatedAt,
		&key.ExpiresAt,
		&key.LastUsedAt,
		&key.RotatedFrom,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to query key: %w", err)
	}

	// Parse permissions
	if err := json.Unmarshal(permissionsJSON, &key.Permissions); err != nil {
		return nil, fmt.Errorf("failed to parse permissions: %w", err)
	}

	// Check key state
	if key.Revoked {
		return nil, ErrKeyRevoked
	}
	if !key.IsActive {
		return nil, ErrKeyInactive
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, ErrKeyExpired
	}

	// Update last used timestamp asynchronously
	// Use a detached context since this is fire-and-forget, but add safety checks
	go func(keyID uuid.UUID) { //nolint:gosec
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Only update if key is still active to prevent race with rotation/revocation
		_, err := km.db.Exec(updateCtx, "UPDATE api_keys SET last_used_at = NOW() WHERE id = $1 AND is_active = TRUE", keyID)
		if err != nil {
			log.Debug().Err(err).Str("key_id", keyID.String()).Msg("Failed to update last_used_at")
		}
	}(key.ID)

	return &key, nil
}

// RevokeAPIKey revokes an API key, preventing further use
func (km *KeyManager) RevokeAPIKey(ctx context.Context, keyID uuid.UUID) error {
	query := `
		UPDATE api_keys
		SET revoked = TRUE, revoked_at = NOW(), is_active = FALSE
		WHERE id = $1 AND revoked = FALSE
	`

	result, err := km.db.Exec(ctx, query, keyID)
	if err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrKeyNotFound
	}

	log.Info().
		Str("key_id", keyID.String()).
		Msg("API key revoked")

	return nil
}

// GetAPIKey retrieves details about an API key by ID
func (km *KeyManager) GetAPIKey(ctx context.Context, keyID uuid.UUID) (*APIKeyDetails, error) {
	query := `
		SELECT id, name, user_id, permissions, is_active, revoked,
		       created_at, expires_at, last_used_at, rotated_from
		FROM api_keys
		WHERE id = $1
	`

	var key APIKeyDetails
	var permissionsJSON []byte

	err := km.db.QueryRow(ctx, query, keyID).Scan(
		&key.ID,
		&key.Name,
		&key.UserID,
		&permissionsJSON,
		&key.IsActive,
		&key.Revoked,
		&key.CreatedAt,
		&key.ExpiresAt,
		&key.LastUsedAt,
		&key.RotatedFrom,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to query key: %w", err)
	}

	// Parse permissions
	if err := json.Unmarshal(permissionsJSON, &key.Permissions); err != nil {
		return nil, fmt.Errorf("failed to parse permissions: %w", err)
	}

	return &key, nil
}

// ListAPIKeys retrieves all API keys for a user
func (km *KeyManager) ListAPIKeys(ctx context.Context, userID string, includeInactive bool) ([]*APIKeyDetails, error) {
	query := `
		SELECT id, name, user_id, permissions, is_active, revoked,
		       created_at, expires_at, last_used_at, rotated_from
		FROM api_keys
		WHERE user_id = $1
	`

	if !includeInactive {
		query += " AND is_active = TRUE AND revoked = FALSE"
	}

	query += " ORDER BY created_at DESC"

	rows, err := km.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKeyDetails
	for rows.Next() {
		var key APIKeyDetails
		var permissionsJSON []byte

		err := rows.Scan(
			&key.ID,
			&key.Name,
			&key.UserID,
			&permissionsJSON,
			&key.IsActive,
			&key.Revoked,
			&key.CreatedAt,
			&key.ExpiresAt,
			&key.LastUsedAt,
			&key.RotatedFrom,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}

		if err := json.Unmarshal(permissionsJSON, &key.Permissions); err != nil {
			return nil, fmt.Errorf("failed to parse permissions: %w", err)
		}

		keys = append(keys, &key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating keys: %w", err)
	}

	return keys, nil
}

// CleanupExpiredKeys deactivates all expired keys
// Returns the number of keys deactivated
func (km *KeyManager) CleanupExpiredKeys(ctx context.Context) (int64, error) {
	query := `
		UPDATE api_keys
		SET is_active = FALSE
		WHERE expires_at IS NOT NULL
		  AND expires_at < NOW()
		  AND is_active = TRUE
		  AND revoked = FALSE
	`

	result, err := km.db.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired keys: %w", err)
	}

	count := result.RowsAffected()
	if count > 0 {
		log.Info().
			Int64("count", count).
			Msg("Deactivated expired API keys")
	}

	return count, nil
}

// StartCleanupWorker starts a background worker that periodically cleans up expired keys
func (km *KeyManager) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Cancel any existing worker
	if km.cleanupCancel != nil {
		km.cleanupCancel()
	}

	// Create new context for the worker
	workerCtx, cancel := context.WithCancel(ctx)
	km.cleanupCancel = cancel

	km.cleanupWG.Add(1)
	go func() {
		defer km.cleanupWG.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Info().
			Dur("interval", interval).
			Msg("API key cleanup worker started")

		for {
			select {
			case <-workerCtx.Done():
				log.Info().Msg("API key cleanup worker stopped")
				return
			case <-ticker.C:
				count, err := km.CleanupExpiredKeys(workerCtx)
				if err != nil {
					log.Error().Err(err).Msg("Failed to cleanup expired API keys")
				} else if count > 0 {
					log.Debug().
						Int64("count", count).
						Msg("Cleanup worker deactivated expired keys")
				}
			}
		}
	}()
}

// StopCleanupWorker stops the background cleanup worker
func (km *KeyManager) StopCleanupWorker() {
	km.mu.Lock()
	if km.cleanupCancel != nil {
		km.cleanupCancel()
		km.cleanupCancel = nil
	}
	km.mu.Unlock()

	// Wait for worker to stop
	km.cleanupWG.Wait()
}

// GetKeyRotationHistory retrieves the rotation history for a key
func (km *KeyManager) GetKeyRotationHistory(ctx context.Context, keyID uuid.UUID) ([]*APIKeyDetails, error) {
	query := `
		WITH RECURSIVE key_chain AS (
			SELECT id, name, user_id, permissions, is_active, revoked,
			       created_at, expires_at, last_used_at, rotated_from,
			       1 as depth
			FROM api_keys
			WHERE id = $1

			UNION ALL

			SELECT k.id, k.name, k.user_id, k.permissions, k.is_active, k.revoked,
			       k.created_at, k.expires_at, k.last_used_at, k.rotated_from,
			       kc.depth + 1
			FROM api_keys k
			JOIN key_chain kc ON k.id = kc.rotated_from
			WHERE kc.depth < 10
		)
		SELECT id, name, user_id, permissions, is_active, revoked,
		       created_at, expires_at, last_used_at, rotated_from
		FROM key_chain
		ORDER BY depth ASC
	`

	rows, err := km.db.Query(ctx, query, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to query key history: %w", err)
	}
	defer rows.Close()

	var keys []*APIKeyDetails
	for rows.Next() {
		var key APIKeyDetails
		var permissionsJSON []byte

		err := rows.Scan(
			&key.ID,
			&key.Name,
			&key.UserID,
			&permissionsJSON,
			&key.IsActive,
			&key.Revoked,
			&key.CreatedAt,
			&key.ExpiresAt,
			&key.LastUsedAt,
			&key.RotatedFrom,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}

		if err := json.Unmarshal(permissionsJSON, &key.Permissions); err != nil {
			return nil, fmt.Errorf("failed to parse permissions: %w", err)
		}

		keys = append(keys, &key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating keys: %w", err)
	}

	return keys, nil
}
