package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// KeyManager Struct Tests
// =============================================================================

func TestNewKeyManager(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)
	require.NotNil(t, km)
	assert.NotNil(t, km.db)
}

func TestAPIKeyDetailsStruct(t *testing.T) {
	id := uuid.New()
	rotatedFrom := uuid.New()
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	lastUsed := now.Add(-1 * time.Hour)

	details := APIKeyDetails{
		ID:          id,
		Name:        "Test Key",
		UserID:      "user-123",
		Permissions: []string{"read:decisions", "write:orders"},
		IsActive:    true,
		Revoked:     false,
		CreatedAt:   now,
		ExpiresAt:   &expiresAt,
		LastUsedAt:  &lastUsed,
		RotatedFrom: &rotatedFrom,
	}

	assert.Equal(t, id, details.ID)
	assert.Equal(t, "Test Key", details.Name)
	assert.Equal(t, "user-123", details.UserID)
	assert.Len(t, details.Permissions, 2)
	assert.Contains(t, details.Permissions, "read:decisions")
	assert.True(t, details.IsActive)
	assert.False(t, details.Revoked)
	assert.NotNil(t, details.ExpiresAt)
	assert.NotNil(t, details.LastUsedAt)
	assert.NotNil(t, details.RotatedFrom)
}

func TestCreatedAPIKeyStruct(t *testing.T) {
	id := uuid.New()
	rotatedFrom := uuid.New()
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	created := CreatedAPIKey{
		ID:          id,
		Key:         "test-plaintext-key",
		Name:        "My API Key",
		UserID:      "user-456",
		Permissions: []string{"*"},
		CreatedAt:   now,
		ExpiresAt:   &expiresAt,
		RotatedFrom: &rotatedFrom,
	}

	assert.Equal(t, id, created.ID)
	assert.Equal(t, "test-plaintext-key", created.Key)
	assert.Equal(t, "My API Key", created.Name)
	assert.Equal(t, "user-456", created.UserID)
	assert.Len(t, created.Permissions, 1)
	assert.NotNil(t, created.ExpiresAt)
	assert.NotNil(t, created.RotatedFrom)
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestErrorConstants(t *testing.T) {
	assert.Equal(t, "API key not found", ErrKeyNotFound.Error())
	assert.Equal(t, "API key is revoked", ErrKeyRevoked.Error())
	assert.Equal(t, "API key is inactive", ErrKeyInactive.Error())
	assert.Equal(t, "API key has expired", ErrKeyExpired.Error())
	assert.Equal(t, "expiration must be in the future", ErrInvalidExpiration.Error())
}

// =============================================================================
// CreateAPIKey Tests
// =============================================================================

func TestCreateAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		keyName     string
		permissions []string
		expiresIn   time.Duration
		setupMock   func(mock pgxmock.PgxPoolIface)
		wantErr     bool
		validate    func(t *testing.T, key *CreatedAPIKey)
	}{
		{
			name:        "create key with expiration and permissions",
			userID:      "admin-user",
			keyName:     "Admin Key",
			permissions: []string{"read:decisions", "write:orders"},
			expiresIn:   30 * 24 * time.Hour,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				keyID := uuid.New()
				createdAt := time.Now()
				rows := pgxmock.NewRows([]string{"id", "created_at"}).
					AddRow(keyID, createdAt)
				mock.ExpectQuery("INSERT INTO api_keys").
					WithArgs(
						pgxmock.AnyArg(), // key_hash
						pgxmock.AnyArg(), // hash_algorithm (SEC-009)
						"Admin Key",
						"admin-user",
						pgxmock.AnyArg(), // permissions JSON
						pgxmock.AnyArg(), // expires_at
					).
					WillReturnRows(rows)
			},
			wantErr: false,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.NotEmpty(t, key.Key)
				assert.Len(t, key.Key, 64) // 32 bytes = 64 hex chars
				assert.Equal(t, "Admin Key", key.Name)
				assert.Equal(t, "admin-user", key.UserID)
				assert.Len(t, key.Permissions, 2)
				assert.NotNil(t, key.ExpiresAt)
			},
		},
		{
			name:        "create key without expiration",
			userID:      "regular-user",
			keyName:     "No Expiry Key",
			permissions: []string{"read:decisions"},
			expiresIn:   0,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				keyID := uuid.New()
				createdAt := time.Now()
				rows := pgxmock.NewRows([]string{"id", "created_at"}).
					AddRow(keyID, createdAt)
				mock.ExpectQuery("INSERT INTO api_keys").
					WithArgs(
						pgxmock.AnyArg(), // key_hash
						pgxmock.AnyArg(), // hash_algorithm (SEC-009)
						"No Expiry Key",
						"regular-user",
						pgxmock.AnyArg(), // permissions JSON
						pgxmock.AnyArg(), // expires_at (nil passed as interface{})
					).
					WillReturnRows(rows)
			},
			wantErr: false,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.NotNil(t, key)
				assert.NotEmpty(t, key.Key)
				assert.Equal(t, "No Expiry Key", key.Name)
				assert.Nil(t, key.ExpiresAt)
			},
		},
		{
			name:        "create key with nil permissions uses default",
			userID:      "default-perms-user",
			keyName:     "Default Perms Key",
			permissions: nil,
			expiresIn:   0,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				keyID := uuid.New()
				createdAt := time.Now()
				rows := pgxmock.NewRows([]string{"id", "created_at"}).
					AddRow(keyID, createdAt)
				mock.ExpectQuery("INSERT INTO api_keys").
					WithArgs(
						pgxmock.AnyArg(), // key_hash
						pgxmock.AnyArg(), // hash_algorithm (SEC-009)
						"Default Perms Key",
						"default-perms-user",
						pgxmock.AnyArg(), // permissions JSON (defaults to ["read:decisions"])
						pgxmock.AnyArg(), // expires_at (nil passed as interface{})
					).
					WillReturnRows(rows)
			},
			wantErr: false,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.NotNil(t, key)
				assert.Len(t, key.Permissions, 1)
				assert.Equal(t, "read:decisions", key.Permissions[0])
			},
		},
		{
			name:        "create key database error",
			userID:      "user",
			keyName:     "Fail Key",
			permissions: nil,
			expiresIn:   0,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO api_keys").
					WithArgs(
						pgxmock.AnyArg(), // key_hash
						pgxmock.AnyArg(), // hash_algorithm (SEC-009)
						"Fail Key",
						"user",
						pgxmock.AnyArg(),
						pgxmock.AnyArg(),
					).
					WillReturnError(errors.New("database connection failed"))
			},
			wantErr: true,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.Nil(t, key)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock)

			ctx := context.Background()
			key, err := km.CreateAPIKey(ctx, tt.userID, tt.keyName, tt.permissions, tt.expiresIn)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, key)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// ValidateAPIKey Tests
// =============================================================================

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		plaintextKey string
		setupMock    func(mock pgxmock.PgxPoolIface, keyHash string)
		wantErr      error
		validate     func(t *testing.T, details *APIKeyDetails)
	}{
		{
			name:         "valid active key",
			plaintextKey: "valid-api-key-123",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				keyID := uuid.New()
				now := time.Now()
				expiresAt := now.Add(24 * time.Hour)
				lastUsed := now.Add(-1 * time.Hour)
				permissionsJSON := []byte(`["read:decisions","write:orders"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "My Key", "user-123", permissionsJSON, true, false,
					now, &expiresAt, &lastUsed, nil,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnRows(rows)
			},
			wantErr: nil,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.NotNil(t, details)
				assert.Equal(t, "My Key", details.Name)
				assert.Equal(t, "user-123", details.UserID)
				assert.True(t, details.IsActive)
				assert.False(t, details.Revoked)
				assert.Len(t, details.Permissions, 2)
			},
		},
		{
			name:         "key not found",
			plaintextKey: "nonexistent-key",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnError(pgx.ErrNoRows)
			},
			wantErr: ErrKeyNotFound,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.Nil(t, details)
			},
		},
		{
			name:         "revoked key",
			plaintextKey: "revoked-key",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				keyID := uuid.New()
				now := time.Now()
				permissionsJSON := []byte(`["read:decisions"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "Revoked Key", "user", permissionsJSON, true, true,
					now, nil, nil, nil,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnRows(rows)
			},
			wantErr: ErrKeyRevoked,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.Nil(t, details)
			},
		},
		{
			name:         "inactive key",
			plaintextKey: "inactive-key",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				keyID := uuid.New()
				now := time.Now()
				permissionsJSON := []byte(`["read:decisions"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "Inactive Key", "user", permissionsJSON, false, false,
					now, nil, nil, nil,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnRows(rows)
			},
			wantErr: ErrKeyInactive,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.Nil(t, details)
			},
		},
		{
			name:         "expired key",
			plaintextKey: "expired-key",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				keyID := uuid.New()
				now := time.Now()
				expiredAt := now.Add(-24 * time.Hour) // Expired yesterday
				permissionsJSON := []byte(`["read:decisions"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "Expired Key", "user", permissionsJSON, true, false,
					now, &expiredAt, nil, nil,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnRows(rows)
			},
			wantErr: ErrKeyExpired,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.Nil(t, details)
			},
		},
		{
			name:         "key with no expiration",
			plaintextKey: "no-expiry-key",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				keyID := uuid.New()
				now := time.Now()
				permissionsJSON := []byte(`["admin"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "No Expiry Key", "admin", permissionsJSON, true, false,
					now, nil, nil, nil,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnRows(rows)
			},
			wantErr: nil,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.NotNil(t, details)
				assert.Equal(t, "No Expiry Key", details.Name)
				assert.Nil(t, details.ExpiresAt)
			},
		},
		{
			name:         "database error",
			plaintextKey: "error-key",
			setupMock: func(mock pgxmock.PgxPoolIface, keyHash string) {
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyHash, HashAlgoSHA256).
					WillReturnError(errors.New("connection timeout"))
			},
			wantErr: nil, // Error is wrapped, so we check for non-nil
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.Nil(t, details)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			keyHash := HashAPIKey(tt.plaintextKey)
			tt.setupMock(mock, keyHash)

			ctx := context.Background()
			details, err := km.ValidateAPIKey(ctx, tt.plaintextKey)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else if tt.name == "database error" {
				assert.Error(t, err)
			}

			tt.validate(t, details)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// RevokeAPIKey Tests
// =============================================================================

func TestRevokeAPIKey(t *testing.T) {
	tests := []struct {
		name      string
		keyID     uuid.UUID
		setupMock func(mock pgxmock.PgxPoolIface, keyID uuid.UUID)
		wantErr   error
	}{
		{
			name:  "successfully revoke existing key",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				mock.ExpectExec("UPDATE api_keys").
					WithArgs(keyID).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			wantErr: nil,
		},
		{
			name:  "revoke non-existent key",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				mock.ExpectExec("UPDATE api_keys").
					WithArgs(keyID).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			},
			wantErr: ErrKeyNotFound,
		},
		{
			name:  "revoke already revoked key",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				mock.ExpectExec("UPDATE api_keys").
					WithArgs(keyID).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			},
			wantErr: ErrKeyNotFound,
		},
		{
			name:  "database error during revoke",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				mock.ExpectExec("UPDATE api_keys").
					WithArgs(keyID).
					WillReturnError(errors.New("database unavailable"))
			},
			wantErr: nil, // Wrapped error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock, tt.keyID)

			ctx := context.Background()
			err = km.RevokeAPIKey(ctx, tt.keyID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else if tt.name == "database error during revoke" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// GetAPIKey Tests
// =============================================================================

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name      string
		keyID     uuid.UUID
		setupMock func(mock pgxmock.PgxPoolIface, keyID uuid.UUID)
		wantErr   error
		validate  func(t *testing.T, details *APIKeyDetails)
	}{
		{
			name:  "get existing key",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				now := time.Now()
				expiresAt := now.Add(7 * 24 * time.Hour)
				permissionsJSON := []byte(`["read:decisions","write:orders"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "Test Key", "test-user", permissionsJSON, true, false,
					now, &expiresAt, nil, nil,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyID).
					WillReturnRows(rows)
			},
			wantErr: nil,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.NotNil(t, details)
				assert.Equal(t, "Test Key", details.Name)
				assert.Equal(t, "test-user", details.UserID)
				assert.Len(t, details.Permissions, 2)
			},
		},
		{
			name:  "get non-existent key",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyID).
					WillReturnError(pgx.ErrNoRows)
			},
			wantErr: ErrKeyNotFound,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.Nil(t, details)
			},
		},
		{
			name:  "get key with rotated_from reference",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				now := time.Now()
				rotatedFrom := uuid.New()
				permissionsJSON := []byte(`["admin"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).AddRow(
					keyID, "Rotated Key", "admin", permissionsJSON, true, false,
					now, nil, nil, &rotatedFrom,
				)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(keyID).
					WillReturnRows(rows)
			},
			wantErr: nil,
			validate: func(t *testing.T, details *APIKeyDetails) {
				assert.NotNil(t, details)
				assert.NotNil(t, details.RotatedFrom)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock, tt.keyID)

			ctx := context.Background()
			details, err := km.GetAPIKey(ctx, tt.keyID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, details)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// ListAPIKeys Tests
// =============================================================================

func TestListAPIKeys(t *testing.T) {
	tests := []struct {
		name            string
		userID          string
		includeInactive bool
		setupMock       func(mock pgxmock.PgxPoolIface)
		wantErr         bool
		expectedCount   int
	}{
		{
			name:            "list active keys only",
			userID:          "user-123",
			includeInactive: false,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				permissionsJSON := []byte(`["read:decisions"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).
					AddRow(uuid.New(), "Key 1", "user-123", permissionsJSON, true, false, now, nil, nil, nil).
					AddRow(uuid.New(), "Key 2", "user-123", permissionsJSON, true, false, now, nil, nil, nil)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantErr:       false,
			expectedCount: 2,
		},
		{
			name:            "list all keys including inactive",
			userID:          "user-456",
			includeInactive: true,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				permissionsJSON := []byte(`["read:decisions"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).
					AddRow(uuid.New(), "Active Key", "user-456", permissionsJSON, true, false, now, nil, nil, nil).
					AddRow(uuid.New(), "Inactive Key", "user-456", permissionsJSON, false, false, now, nil, nil, nil).
					AddRow(uuid.New(), "Revoked Key", "user-456", permissionsJSON, false, true, now, nil, nil, nil)

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs("user-456").
					WillReturnRows(rows)
			},
			wantErr:       false,
			expectedCount: 3,
		},
		{
			name:            "list empty result",
			userID:          "no-keys-user",
			includeInactive: false,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				})

				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs("no-keys-user").
					WillReturnRows(rows)
			},
			wantErr:       false,
			expectedCount: 0,
		},
		{
			name:            "database error",
			userID:          "user",
			includeInactive: false,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs("user").
					WillReturnError(errors.New("query failed"))
			},
			wantErr:       true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock)

			ctx := context.Background()
			keys, err := km.ListAPIKeys(ctx, tt.userID, tt.includeInactive)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, keys)
			} else {
				assert.NoError(t, err)
				assert.Len(t, keys, tt.expectedCount)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// RotateAPIKey Tests
// =============================================================================

func TestRotateAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		oldKeyID     uuid.UUID
		newExpiresIn *time.Duration
		setupMock    func(mock pgxmock.PgxPoolIface, oldKeyID uuid.UUID)
		wantErr      error
		validate     func(t *testing.T, key *CreatedAPIKey)
	}{
		{
			name:         "rotate key successfully with new expiration",
			oldKeyID:     uuid.New(),
			newExpiresIn: func() *time.Duration { d := 7 * 24 * time.Hour; return &d }(),
			setupMock: func(mock pgxmock.PgxPoolIface, oldKeyID uuid.UUID) {
				now := time.Now()
				permissionsJSON := []byte(`["read:decisions","write:orders"]`)

				// Begin transaction
				mock.ExpectBegin()

				// Query old key
				oldKeyRows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked", "expires_at",
				}).AddRow(
					oldKeyID, "Original Key", "user-123", permissionsJSON, true, false, nil,
				)
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(oldKeyID).
					WillReturnRows(oldKeyRows)

				// Deactivate old key
				mock.ExpectExec("UPDATE api_keys SET is_active").
					WithArgs(oldKeyID).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))

				// Insert new key
				newKeyID := uuid.New()
				newKeyRows := pgxmock.NewRows([]string{"id", "created_at"}).
					AddRow(newKeyID, now)
				mock.ExpectQuery("INSERT INTO api_keys").
					WithArgs(
						pgxmock.AnyArg(), // key_hash
						pgxmock.AnyArg(), // hash_algorithm (SEC-009)
						"Original Key (rotated)",
						"user-123",
						permissionsJSON,
						pgxmock.AnyArg(), // expires_at
						oldKeyID,
					).
					WillReturnRows(newKeyRows)

				// Commit transaction
				mock.ExpectCommit()
			},
			wantErr: nil,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.NotNil(t, key)
				assert.NotEmpty(t, key.Key)
				assert.Equal(t, "Original Key (rotated)", key.Name)
				assert.NotNil(t, key.RotatedFrom)
				assert.NotNil(t, key.ExpiresAt)
			},
		},
		{
			name:         "rotate key inheriting expiration",
			oldKeyID:     uuid.New(),
			newExpiresIn: nil,
			setupMock: func(mock pgxmock.PgxPoolIface, oldKeyID uuid.UUID) {
				now := time.Now()
				oldExpiry := now.Add(30 * 24 * time.Hour)
				permissionsJSON := []byte(`["admin"]`)

				mock.ExpectBegin()

				oldKeyRows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked", "expires_at",
				}).AddRow(
					oldKeyID, "Admin Key", "admin", permissionsJSON, true, false, &oldExpiry,
				)
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(oldKeyID).
					WillReturnRows(oldKeyRows)

				mock.ExpectExec("UPDATE api_keys SET is_active").
					WithArgs(oldKeyID).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))

				newKeyID := uuid.New()
				newKeyRows := pgxmock.NewRows([]string{"id", "created_at"}).
					AddRow(newKeyID, now)
				mock.ExpectQuery("INSERT INTO api_keys").
					WithArgs(
						pgxmock.AnyArg(), // key_hash
						pgxmock.AnyArg(), // hash_algorithm (SEC-009)
						"Admin Key (rotated)",
						"admin",
						permissionsJSON,
						&oldExpiry, // inherits old expiry
						oldKeyID,
					).
					WillReturnRows(newKeyRows)

				mock.ExpectCommit()
			},
			wantErr: nil,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.NotNil(t, key)
				assert.NotNil(t, key.ExpiresAt)
			},
		},
		{
			name:         "rotate non-existent key",
			oldKeyID:     uuid.New(),
			newExpiresIn: nil,
			setupMock: func(mock pgxmock.PgxPoolIface, oldKeyID uuid.UUID) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(oldKeyID).
					WillReturnError(pgx.ErrNoRows)
				mock.ExpectRollback()
			},
			wantErr: ErrKeyNotFound,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.Nil(t, key)
			},
		},
		{
			name:         "rotate revoked key",
			oldKeyID:     uuid.New(),
			newExpiresIn: nil,
			setupMock: func(mock pgxmock.PgxPoolIface, oldKeyID uuid.UUID) {
				permissionsJSON := []byte(`["read:decisions"]`)

				mock.ExpectBegin()
				oldKeyRows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked", "expires_at",
				}).AddRow(
					oldKeyID, "Revoked Key", "user", permissionsJSON, true, true, nil,
				)
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(oldKeyID).
					WillReturnRows(oldKeyRows)
				mock.ExpectRollback()
			},
			wantErr: ErrKeyRevoked,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.Nil(t, key)
			},
		},
		{
			name:         "rotate inactive key",
			oldKeyID:     uuid.New(),
			newExpiresIn: nil,
			setupMock: func(mock pgxmock.PgxPoolIface, oldKeyID uuid.UUID) {
				permissionsJSON := []byte(`["read:decisions"]`)

				mock.ExpectBegin()
				oldKeyRows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked", "expires_at",
				}).AddRow(
					oldKeyID, "Inactive Key", "user", permissionsJSON, false, false, nil,
				)
				mock.ExpectQuery("SELECT id, name, user_id").
					WithArgs(oldKeyID).
					WillReturnRows(oldKeyRows)
				mock.ExpectRollback()
			},
			wantErr: ErrKeyInactive,
			validate: func(t *testing.T, key *CreatedAPIKey) {
				assert.Nil(t, key)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock, tt.oldKeyID)

			ctx := context.Background()
			key, err := km.RotateAPIKey(ctx, tt.oldKeyID, tt.newExpiresIn)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, key)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// CleanupExpiredKeys Tests
// =============================================================================

func TestCleanupExpiredKeys(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(mock pgxmock.PgxPoolIface)
		wantErr       bool
		expectedCount int64
	}{
		{
			name: "cleanup multiple expired keys",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("UPDATE api_keys").
					WillReturnResult(pgxmock.NewResult("UPDATE", 5))
			},
			wantErr:       false,
			expectedCount: 5,
		},
		{
			name: "cleanup no expired keys",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("UPDATE api_keys").
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			},
			wantErr:       false,
			expectedCount: 0,
		},
		{
			name: "database error during cleanup",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("UPDATE api_keys").
					WillReturnError(errors.New("connection lost"))
			},
			wantErr:       true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock)

			ctx := context.Background()
			count, err := km.CleanupExpiredKeys(ctx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// GetKeyRotationHistory Tests
// =============================================================================

func TestGetKeyRotationHistory(t *testing.T) {
	tests := []struct {
		name          string
		keyID         uuid.UUID
		setupMock     func(mock pgxmock.PgxPoolIface, keyID uuid.UUID)
		wantErr       bool
		expectedCount int
	}{
		{
			name:  "get rotation history with multiple keys",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				now := time.Now()
				oldKeyID := uuid.New()
				olderKeyID := uuid.New()
				permissionsJSON := []byte(`["read:decisions"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).
					AddRow(keyID, "Current Key", "user", permissionsJSON, true, false, now, nil, nil, &oldKeyID).
					AddRow(oldKeyID, "Previous Key", "user", permissionsJSON, false, false, now.Add(-24*time.Hour), nil, nil, &olderKeyID).
					AddRow(olderKeyID, "Original Key", "user", permissionsJSON, false, false, now.Add(-48*time.Hour), nil, nil, nil)

				mock.ExpectQuery("WITH RECURSIVE key_chain AS").
					WithArgs(keyID).
					WillReturnRows(rows)
			},
			wantErr:       false,
			expectedCount: 3,
		},
		{
			name:  "get history for key with no rotation",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				now := time.Now()
				permissionsJSON := []byte(`["admin"]`)

				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				}).
					AddRow(keyID, "Only Key", "admin", permissionsJSON, true, false, now, nil, nil, nil)

				mock.ExpectQuery("WITH RECURSIVE key_chain AS").
					WithArgs(keyID).
					WillReturnRows(rows)
			},
			wantErr:       false,
			expectedCount: 1,
		},
		{
			name:  "get history for non-existent key",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				rows := pgxmock.NewRows([]string{
					"id", "name", "user_id", "permissions", "is_active", "revoked",
					"created_at", "expires_at", "last_used_at", "rotated_from",
				})

				mock.ExpectQuery("WITH RECURSIVE key_chain AS").
					WithArgs(keyID).
					WillReturnRows(rows)
			},
			wantErr:       false,
			expectedCount: 0,
		},
		{
			name:  "database error",
			keyID: uuid.New(),
			setupMock: func(mock pgxmock.PgxPoolIface, keyID uuid.UUID) {
				mock.ExpectQuery("WITH RECURSIVE key_chain AS").
					WithArgs(keyID).
					WillReturnError(errors.New("query timeout"))
			},
			wantErr:       true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			km := NewKeyManager(mock)
			tt.setupMock(mock, tt.keyID)

			ctx := context.Background()
			history, err := km.GetKeyRotationHistory(ctx, tt.keyID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, history)
			} else {
				assert.NoError(t, err)
				assert.Len(t, history, tt.expectedCount)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// =============================================================================
// Cleanup Worker Tests
// =============================================================================

func TestStartStopCleanupWorker(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)

	// Start the worker with a very short interval for testing
	ctx := context.Background()
	km.StartCleanupWorker(ctx, 10*time.Millisecond)

	// The worker should start - give it a moment
	time.Sleep(5 * time.Millisecond)

	// Stop the worker
	km.StopCleanupWorker()

	// The worker should stop gracefully
	// This test primarily verifies no panics or deadlocks occur
}

func TestStartCleanupWorkerReplacesPrevious(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)
	ctx := context.Background()

	// Start first worker
	km.StartCleanupWorker(ctx, 100*time.Millisecond)

	// Start second worker (should replace first)
	km.StartCleanupWorker(ctx, 50*time.Millisecond)

	// Give workers time to initialize
	time.Sleep(10 * time.Millisecond)

	// Stop all workers
	km.StopCleanupWorker()
}

func TestStopCleanupWorkerWithoutStart(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)

	// Should not panic when stopping without starting
	km.StopCleanupWorker()
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestListAPIKeysRowScanError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)

	// Create rows with invalid JSON for permissions
	rows := pgxmock.NewRows([]string{
		"id", "name", "user_id", "permissions", "is_active", "revoked",
		"created_at", "expires_at", "last_used_at", "rotated_from",
	}).AddRow(
		uuid.New(), "Key", "user", []byte(`invalid json`), true, false,
		time.Now(), nil, nil, nil,
	)

	mock.ExpectQuery("SELECT id, name, user_id").
		WithArgs("user").
		WillReturnRows(rows)

	ctx := context.Background()
	keys, err := km.ListAPIKeys(ctx, "user", false)

	assert.Error(t, err)
	assert.Nil(t, keys)
	assert.Contains(t, err.Error(), "failed to parse permissions")
}

func TestValidateAPIKeyInvalidPermissionsJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)
	plaintextKey := "test-key"
	keyHash := HashAPIKey(plaintextKey)

	rows := pgxmock.NewRows([]string{
		"id", "name", "user_id", "permissions", "is_active", "revoked",
		"created_at", "expires_at", "last_used_at", "rotated_from",
	}).AddRow(
		uuid.New(), "Key", "user", []byte(`not valid json`), true, false,
		time.Now(), nil, nil, nil,
	)

	mock.ExpectQuery("SELECT id, name, user_id").
		WithArgs(keyHash, HashAlgoSHA256).
		WillReturnRows(rows)

	ctx := context.Background()
	details, err := km.ValidateAPIKey(ctx, plaintextKey)

	assert.Error(t, err)
	assert.Nil(t, details)
	assert.Contains(t, err.Error(), "failed to parse permissions")
}

// TestKeyManager_ValidateAPIKey_HMAC_SEC009 exercises the pepper-aware
// ValidateAPIKey path in keys.go without testcontainers. Asserts that:
//  1. When a pepper is configured, the validator probes the HMAC hash first
//     and matches a key stored with hash_algorithm='hmac-sha256'.
//  2. When the HMAC probe misses (legacy row), the validator falls back
//     to the SHA-256 probe and still returns the key.
//  3. A non-ErrNoRows DB error from the first probe is propagated instead
//     of silently falling through to the legacy probe — this is the
//     critical security property introduced by the round-1 fix.
func TestKeyManager_ValidateAPIKey_HMAC_SEC009(t *testing.T) {
	const pepper = "unit-test-pepper"

	t.Run("HMAC probe hits first", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		km := NewKeyManagerWithPepper(mock, pepper)

		plaintext := "api-key-hmac-hit"
		hmacHash := MustHashAPIKeyHMAC(pepper, plaintext)

		rows := pgxmock.NewRows([]string{
			"id", "name", "user_id", "permissions", "is_active", "revoked",
			"created_at", "expires_at", "last_used_at", "rotated_from",
		}).AddRow(
			uuid.New(), "HMAC Key", "user", []byte(`["read"]`), true, false,
			time.Now(), nil, nil, nil,
		)
		// Expect the HMAC probe — and NOT the legacy probe.
		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnRows(rows)

		details, err := km.ValidateAPIKey(context.Background(), plaintext)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, "HMAC Key", details.Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("HMAC probe misses, SHA-256 fallback hits", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		km := NewKeyManagerWithPepper(mock, pepper)

		plaintext := "api-key-legacy"
		hmacHash := MustHashAPIKeyHMAC(pepper, plaintext)
		legacyHash := HashAPIKey(plaintext)

		// First probe (HMAC) returns ErrNoRows.
		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnError(pgx.ErrNoRows)

		// Second probe (SHA-256) returns the key row.
		rows := pgxmock.NewRows([]string{
			"id", "name", "user_id", "permissions", "is_active", "revoked",
			"created_at", "expires_at", "last_used_at", "rotated_from",
		}).AddRow(
			uuid.New(), "Legacy Key", "user", []byte(`["read"]`), true, false,
			time.Now(), nil, nil, nil,
		)
		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(legacyHash, HashAlgoSHA256).
			WillReturnRows(rows)

		details, err := km.ValidateAPIKey(context.Background(), plaintext)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, "Legacy Key", details.Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("HMAC probe DB error propagates, no fallback", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		km := NewKeyManagerWithPepper(mock, pepper)

		plaintext := "api-key-transient"
		hmacHash := MustHashAPIKeyHMAC(pepper, plaintext)

		// HMAC probe returns a connection error — NOT ErrNoRows. The
		// validator must propagate this instead of falling through to
		// the legacy probe (which would defeat the HMAC guarantee for
		// transient DB failures).
		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnError(errors.New("connection refused"))

		details, err := km.ValidateAPIKey(context.Background(), plaintext)
		require.Error(t, err)
		assert.Nil(t, details)
		assert.Contains(t, err.Error(), "connection refused")
		// Critically: the mock should have NO unmet expectations — the
		// legacy probe must not have been attempted.
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetKeyRotationHistoryInvalidPermissionsJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)
	keyID := uuid.New()

	rows := pgxmock.NewRows([]string{
		"id", "name", "user_id", "permissions", "is_active", "revoked",
		"created_at", "expires_at", "last_used_at", "rotated_from",
	}).AddRow(
		keyID, "Key", "user", []byte(`{invalid`), true, false,
		time.Now(), nil, nil, nil,
	)

	mock.ExpectQuery("WITH RECURSIVE key_chain AS").
		WithArgs(keyID).
		WillReturnRows(rows)

	ctx := context.Background()
	history, err := km.GetKeyRotationHistory(ctx, keyID)

	assert.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "failed to parse permissions")
}

func TestGetAPIKeyInvalidPermissionsJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	km := NewKeyManager(mock)
	keyID := uuid.New()

	rows := pgxmock.NewRows([]string{
		"id", "name", "user_id", "permissions", "is_active", "revoked",
		"created_at", "expires_at", "last_used_at", "rotated_from",
	}).AddRow(
		keyID, "Key", "user", []byte(`["missing quote]`), true, false,
		time.Now(), nil, nil, nil,
	)

	mock.ExpectQuery("SELECT id, name, user_id").
		WithArgs(keyID).
		WillReturnRows(rows)

	ctx := context.Background()
	details, err := km.GetAPIKey(ctx, keyID)

	assert.Error(t, err)
	assert.Nil(t, details)
	assert.Contains(t, err.Error(), "failed to parse permissions")
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkHashAPIKey(b *testing.B) {
	key := "benchmark-api-key-1234567890"
	for i := 0; i < b.N; i++ {
		HashAPIKey(key)
	}
}

func BenchmarkValidateAPIKey(b *testing.B) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		b.Fatal(err)
	}
	defer mock.Close()

	km := NewKeyManager(mock)
	plaintextKey := "benchmark-key"
	keyHash := HashAPIKey(plaintextKey)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		keyID := uuid.New()
		now := time.Now()
		permissionsJSON := []byte(`["read:decisions"]`)

		rows := pgxmock.NewRows([]string{
			"id", "name", "user_id", "permissions", "is_active", "revoked",
			"created_at", "expires_at", "last_used_at", "rotated_from",
		}).AddRow(
			keyID, "Key", "user", permissionsJSON, true, false,
			now, nil, nil, nil,
		)

		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(keyHash, HashAlgoSHA256).
			WillReturnRows(rows)
		b.StartTimer()

		_, _ = km.ValidateAPIKey(ctx, plaintextKey)
	}
}

// BenchmarkValidateAPIKey_HMACLegacyFallback measures the migration-window
// cost per validation: every request probes the HMAC hash first, misses
// (because the row is still sha256), then hits the legacy probe. Two DB
// round-trips per validation until the opportunistic rehash drains the
// fleet. This benchmark exists to make that cost observable in CI before
// the migration completes so operators can size accordingly.
func BenchmarkValidateAPIKey_HMACLegacyFallback(b *testing.B) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		b.Fatal(err)
	}
	defer mock.Close()
	// A successful validation fires two async UPDATEs: the last_used_at
	// refresh and the opportunistic rehash. MatchExpectationsInOrder(false)
	// lets those async Exec calls land on the expected matchers in any
	// order alongside the synchronous probe queries.
	mock.MatchExpectationsInOrder(false)

	const pepper = "bench-pepper-32-bytes-of-material"
	km := NewKeyManagerWithPepper(mock, pepper)
	plaintextKey := "benchmark-legacy-key"
	hmacHash := MustHashAPIKeyHMAC(pepper, plaintextKey)
	legacyHash := HashAPIKey(plaintextKey)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		keyID := uuid.New()
		now := time.Now()
		permissionsJSON := []byte(`["read:decisions"]`)

		// First probe: HMAC hash — miss (legacy row).
		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(hmacHash, HashAlgoHMACSHA256).
			WillReturnError(pgx.ErrNoRows)

		// Second probe: SHA-256 — hit, returns the row.
		rows := pgxmock.NewRows([]string{
			"id", "name", "user_id", "permissions", "is_active", "revoked",
			"created_at", "expires_at", "last_used_at", "rotated_from",
		}).AddRow(
			keyID, "Key", "user", permissionsJSON, true, false,
			now, nil, nil, nil,
		)
		mock.ExpectQuery("SELECT id, name, user_id").
			WithArgs(legacyHash, HashAlgoSHA256).
			WillReturnRows(rows)

		// Async last_used_at update + async rehash — both are
		// fire-and-forget; we accept them with a permissive matcher so
		// they don't log "unexpected call" noise during the benchmark.
		mock.ExpectExec("UPDATE api_keys").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("UPDATE api_keys").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		b.StartTimer()
		_, _ = km.ValidateAPIKey(ctx, plaintextKey)
	}
}
