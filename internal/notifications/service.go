package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service defines the interface for notification operations
type Service interface {
	// SendToUser sends a notification to all enabled devices for a user
	SendToUser(ctx context.Context, userID string, notification Notification) error

	// SendToDevice sends a notification to a specific device
	SendToDevice(ctx context.Context, deviceToken string, notification Notification) error

	// RegisterDevice registers a new device for push notifications
	RegisterDevice(ctx context.Context, userID, deviceToken string, platform Platform) error

	// UnregisterDevice removes a device token
	UnregisterDevice(ctx context.Context, deviceToken string) error

	// GetUserDevices returns all enabled devices for a user
	GetUserDevices(ctx context.Context, userID string) ([]Device, error)

	// UpdatePreferences updates user notification preferences
	UpdatePreferences(ctx context.Context, userID string, prefs Preferences) error

	// GetPreferences returns user notification preferences
	GetPreferences(ctx context.Context, userID string) (Preferences, error)

	// UpdateDeviceLastUsed updates the last used timestamp for a device
	UpdateDeviceLastUsed(ctx context.Context, deviceToken string) error
}

// Backend defines the interface for notification backends (FCM, APNs, etc.)
type Backend interface {
	// Send sends a notification to a device
	Send(ctx context.Context, deviceToken string, notification Notification) error

	// Name returns the backend name
	Name() string

	// Close closes the backend connection
	Close() error
}

// NotificationService implements the Service interface
type NotificationService struct {
	db      *pgxpool.Pool
	backend Backend
}

// NewService creates a new notification service
func NewService(db *pgxpool.Pool, backend Backend) *NotificationService {
	return &NotificationService{
		db:      db,
		backend: backend,
	}
}

// SendToUser sends a notification to all enabled devices for a user
func (s *NotificationService) SendToUser(ctx context.Context, userID string, notification Notification) error {
	// Check user preferences
	prefs, err := s.GetPreferences(ctx, userID)
	if err != nil {
		// If preferences don't exist, use defaults
		if err == sql.ErrNoRows {
			prefs = DefaultPreferences()
		} else {
			return fmt.Errorf("failed to get user preferences: %w", err)
		}
	}

	// Check if this notification type is enabled
	if !prefs.IsEnabled(notification.Type) {
		log.Debug().
			Str("user_id", userID).
			Str("notification_type", string(notification.Type)).
			Msg("Notification type disabled for user")
		return nil
	}

	// Get all enabled devices for the user
	devices, err := s.GetUserDevices(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user devices: %w", err)
	}

	if len(devices) == 0 {
		log.Debug().Str("user_id", userID).Msg("No enabled devices found for user")
		return nil
	}

	// Send to all devices
	var lastErr error
	sentCount := 0
	for _, device := range devices {
		if err := s.sendAndLog(ctx, userID, device.DeviceToken, notification); err != nil {
			log.Error().
				Err(err).
				Str("user_id", userID).
				Str("device_token", maskToken(device.DeviceToken)).
				Msg("Failed to send notification to device")
			lastErr = err
		} else {
			sentCount++
		}
	}

	if sentCount > 0 {
		log.Info().
			Str("user_id", userID).
			Int("sent_count", sentCount).
			Int("total_devices", len(devices)).
			Str("notification_type", string(notification.Type)).
			Msg("Sent notifications to user devices")
	}

	// Return error only if all sends failed
	if sentCount == 0 && lastErr != nil {
		return fmt.Errorf("failed to send to any device: %w", lastErr)
	}

	return nil
}

// SendToDevice sends a notification to a specific device
func (s *NotificationService) SendToDevice(ctx context.Context, deviceToken string, notification Notification) error {
	// Get user ID for this device
	var userID string
	err := s.db.QueryRow(ctx, `
		SELECT user_id FROM user_devices
		WHERE device_token = $1 AND enabled = TRUE
	`, deviceToken).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("device token not found or disabled")
		}
		return fmt.Errorf("failed to query device: %w", err)
	}

	return s.sendAndLog(ctx, userID, deviceToken, notification)
}

// sendAndLog sends a notification and logs the result
func (s *NotificationService) sendAndLog(ctx context.Context, userID, deviceToken string, notification Notification) error {
	var status NotificationStatus
	var errorMsg string

	// Send via backend
	err := s.backend.Send(ctx, deviceToken, notification)
	if err != nil {
		status = NotificationStatusFailed
		errorMsg = err.Error()
	} else {
		status = NotificationStatusSent
		// Update device last used timestamp
		_ = s.UpdateDeviceLastUsed(ctx, deviceToken)
	}

	// Log notification
	dataJSON, _ := json.Marshal(notification.Data)
	_, logErr := s.db.Exec(ctx, `
		INSERT INTO notification_log (
			user_id, device_token, notification_type, title, body, data, status, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, deviceToken, notification.Type, notification.Title, notification.Body, dataJSON, status, errorMsg)

	if logErr != nil {
		log.Error().Err(logErr).Msg("Failed to log notification")
	}

	return err
}

// RegisterDevice registers a new device for push notifications
func (s *NotificationService) RegisterDevice(ctx context.Context, userID, deviceToken string, platform Platform) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_devices (user_id, device_token, platform)
		VALUES ($1, $2, $3)
		ON CONFLICT (device_token) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    platform = EXCLUDED.platform,
		    enabled = TRUE,
		    last_used_at = CURRENT_TIMESTAMP
	`, userID, deviceToken, platform)

	if err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}

	log.Info().
		Str("user_id", userID).
		Str("platform", string(platform)).
		Str("device_token", maskToken(deviceToken)).
		Msg("Registered device for notifications")

	return nil
}

// UnregisterDevice removes a device token
func (s *NotificationService) UnregisterDevice(ctx context.Context, deviceToken string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE user_devices
		SET enabled = FALSE
		WHERE device_token = $1
	`, deviceToken)

	if err != nil {
		return fmt.Errorf("failed to unregister device: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("device token not found")
	}

	log.Info().
		Str("device_token", maskToken(deviceToken)).
		Msg("Unregistered device")

	return nil
}

// GetUserDevices returns all enabled devices for a user
func (s *NotificationService) GetUserDevices(ctx context.Context, userID string) ([]Device, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, device_token, platform, enabled, created_at, last_used_at
		FROM user_devices
		WHERE user_id = $1 AND enabled = TRUE
		ORDER BY last_used_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		err := rows.Scan(
			&d.ID,
			&d.UserID,
			&d.DeviceToken,
			&d.Platform,
			&d.Enabled,
			&d.CreatedAt,
			&d.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}
		devices = append(devices, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating devices: %w", err)
	}

	return devices, nil
}

// UpdatePreferences updates user notification preferences
func (s *NotificationService) UpdatePreferences(ctx context.Context, userID string, prefs Preferences) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO notification_preferences (
			user_id, trade_executions, pnl_alerts, circuit_breaker, consensus_failures
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET trade_executions = EXCLUDED.trade_executions,
		    pnl_alerts = EXCLUDED.pnl_alerts,
		    circuit_breaker = EXCLUDED.circuit_breaker,
		    consensus_failures = EXCLUDED.consensus_failures
	`, userID, prefs.TradeExecutions, prefs.PnLAlerts, prefs.CircuitBreaker, prefs.ConsensusFailures)

	if err != nil {
		return fmt.Errorf("failed to update preferences: %w", err)
	}

	log.Info().Str("user_id", userID).Msg("Updated notification preferences")

	return nil
}

// GetPreferences returns user notification preferences
func (s *NotificationService) GetPreferences(ctx context.Context, userID string) (Preferences, error) {
	var prefs Preferences
	err := s.db.QueryRow(ctx, `
		SELECT trade_executions, pnl_alerts, circuit_breaker, consensus_failures
		FROM notification_preferences
		WHERE user_id = $1
	`, userID).Scan(
		&prefs.TradeExecutions,
		&prefs.PnLAlerts,
		&prefs.CircuitBreaker,
		&prefs.ConsensusFailures,
	)

	if err != nil {
		return Preferences{}, err
	}

	return prefs, nil
}

// UpdateDeviceLastUsed updates the last used timestamp for a device
func (s *NotificationService) UpdateDeviceLastUsed(ctx context.Context, deviceToken string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE user_devices
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE device_token = $1
	`, deviceToken)

	if err != nil {
		return fmt.Errorf("failed to update device last used: %w", err)
	}

	return nil
}

// Close closes the notification service
func (s *NotificationService) Close() error {
	if s.backend != nil {
		return s.backend.Close()
	}
	return nil
}

// Helper function to mask device tokens in logs
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
