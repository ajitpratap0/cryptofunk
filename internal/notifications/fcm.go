package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
)

// FCMBackend implements the Backend interface using Firebase Cloud Messaging
type FCMBackend struct {
	client *messaging.Client
	mock   bool
}

// NewFCMBackend creates a new FCM backend
// If credentialsPath is empty or file doesn't exist, creates a mock backend
func NewFCMBackend(ctx context.Context, credentialsPath string) (*FCMBackend, error) {
	// Check if credentials file exists
	if credentialsPath == "" {
		log.Warn().Msg("No FCM credentials path provided, using mock backend")
		return &FCMBackend{mock: true}, nil
	}

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		log.Warn().
			Str("credentials_path", credentialsPath).
			Msg("FCM credentials file not found, using mock backend")
		return &FCMBackend{mock: true}, nil
	}

	// Initialize Firebase app
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firebase app: %w", err)
	}

	// Get messaging client
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create messaging client: %w", err)
	}

	log.Info().Msg("Initialized FCM backend")

	return &FCMBackend{
		client: client,
		mock:   false,
	}, nil
}

// Send sends a notification via FCM
func (f *FCMBackend) Send(ctx context.Context, deviceToken string, notification Notification) error {
	if f.mock {
		return f.sendMock(deviceToken, notification)
	}

	// Build FCM message
	msg := &messaging.Message{
		Token: deviceToken,
		Notification: &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Body,
		},
		Data: notification.Data,
	}

	// Set priority if specified
	if notification.Priority == "high" {
		msg.Android = &messaging.AndroidConfig{
			Priority: "high",
		}
		msg.APNS = &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
		}
	}

	// Send message
	response, err := f.client.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send FCM message: %w", err)
	}

	log.Debug().
		Str("message_id", response).
		Str("device_token", maskToken(deviceToken)).
		Str("notification_type", string(notification.Type)).
		Msg("Sent FCM notification")

	return nil
}

// sendMock logs the notification instead of sending it
func (f *FCMBackend) sendMock(deviceToken string, notification Notification) error {
	dataJSON, _ := json.Marshal(notification.Data)

	log.Info().
		Str("backend", "fcm_mock").
		Str("device_token", maskToken(deviceToken)).
		Str("notification_type", string(notification.Type)).
		Str("title", notification.Title).
		Str("body", notification.Body).
		Str("data", string(dataJSON)).
		Str("priority", notification.Priority).
		Msg("Mock FCM notification (not actually sent)")

	return nil
}

// SendMulticast sends a notification to multiple devices
func (f *FCMBackend) SendMulticast(ctx context.Context, deviceTokens []string, notification Notification) (*messaging.BatchResponse, error) {
	if f.mock {
		for _, token := range deviceTokens {
			if err := f.sendMock(token, notification); err != nil {
				return nil, err
			}
		}
		return &messaging.BatchResponse{
			SuccessCount: len(deviceTokens),
		}, nil
	}

	// Build FCM multicast message
	msg := &messaging.MulticastMessage{
		Tokens: deviceTokens,
		Notification: &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Body,
		},
		Data: notification.Data,
	}

	// Set priority if specified
	if notification.Priority == "high" {
		msg.Android = &messaging.AndroidConfig{
			Priority: "high",
		}
		msg.APNS = &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
		}
	}

	// Send multicast message
	response, err := f.client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send FCM multicast: %w", err)
	}

	log.Info().
		Int("success_count", response.SuccessCount).
		Int("failure_count", response.FailureCount).
		Int("total", len(deviceTokens)).
		Str("notification_type", string(notification.Type)).
		Msg("Sent FCM multicast notification")

	return response, nil
}

// Name returns the backend name
func (f *FCMBackend) Name() string {
	if f.mock {
		return "fcm_mock"
	}
	return "fcm"
}

// Close closes the FCM backend
func (f *FCMBackend) Close() error {
	// FCM client doesn't need explicit closing
	log.Debug().Str("backend", f.Name()).Msg("Closed FCM backend")
	return nil
}

// IsMock returns true if this is a mock backend
func (f *FCMBackend) IsMock() bool {
	return f.mock
}

// ValidateToken checks if a device token is valid for FCM
// This is a simple validation - actual validation happens when sending
func ValidateToken(token string) bool {
	// FCM tokens are typically 152-163 characters long
	if len(token) < 100 || len(token) > 200 {
		return false
	}

	// Check for valid characters (alphanumeric, hyphens, underscores, colons)
	for _, ch := range token {
		valid := (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == ':'
		if !valid {
			return false
		}
	}

	return true
}
