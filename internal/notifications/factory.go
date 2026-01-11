package notifications

import (
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

// NewManagerFromConfig creates a NotificationManager from the application config
func NewManagerFromConfig(cfg *config.Config) (*Manager, error) {
	notifCfg := cfg.Notifications

	// Create email backend if enabled
	var emailBackend *EmailBackend
	if notifCfg.Email.Enabled {
		emailConfig := EmailConfig{
			Host:               notifCfg.Email.Host,
			Port:               notifCfg.Email.Port,
			Username:           notifCfg.Email.Username,
			Password:           notifCfg.Email.Password,
			FromAddress:        notifCfg.Email.FromAddress,
			FromName:           notifCfg.Email.FromName,
			UseTLS:             notifCfg.Email.UseTLS,
			UseStartTLS:        notifCfg.Email.UseStartTLS,
			SkipVerify:         notifCfg.Email.SkipVerify,
			AuthRequired:       notifCfg.Email.AuthRequired,
			RateLimitPerMinute: notifCfg.Email.RateLimitPerMinute,
			RetryAttempts:      notifCfg.Email.RetryAttempts,
			RetryDelaySeconds:  notifCfg.Email.RetryDelaySeconds,
		}

		var err error
		emailBackend, err = NewEmailBackend(emailConfig, notifCfg.Email.Recipients)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create email backend, notifications may not work")
		} else {
			if emailBackend.IsMock() {
				log.Info().Msg("Email notifications configured in mock mode (incomplete configuration)")
			} else {
				log.Info().
					Str("host", emailConfig.Host).
					Int("port", emailConfig.Port).
					Int("recipients", len(notifCfg.Email.Recipients)).
					Msg("Email notifications enabled")
			}
		}
	} else {
		log.Info().Msg("Email notifications disabled")
	}

	// Create Slack backend if enabled
	var slackBackend *SlackBackend
	if notifCfg.Slack.Enabled {
		slackConfig := SlackConfig{
			WebhookURL:         notifCfg.Slack.WebhookURL,
			Channel:            notifCfg.Slack.Channel,
			Username:           notifCfg.Slack.Username,
			IconEmoji:          notifCfg.Slack.IconEmoji,
			IconURL:            notifCfg.Slack.IconURL,
			RateLimitPerMinute: notifCfg.Slack.RateLimitPerMinute,
			RetryAttempts:      notifCfg.Slack.RetryAttempts,
			RetryDelaySeconds:  notifCfg.Slack.RetryDelaySeconds,
			TimeoutSeconds:     notifCfg.Slack.TimeoutSeconds,
		}

		var err error
		slackBackend, err = NewSlackBackend(slackConfig)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create Slack backend, notifications may not work")
		} else {
			if slackBackend.IsMock() {
				log.Info().Msg("Slack notifications configured in mock mode (no webhook URL)")
			} else {
				log.Info().
					Str("channel", slackConfig.Channel).
					Str("username", slackConfig.Username).
					Msg("Slack notifications enabled")
			}
		}
	} else {
		log.Info().Msg("Slack notifications disabled")
	}

	// Build manager config from config events
	managerConfig := buildManagerConfig(notifCfg)

	manager := NewManager(managerConfig, emailBackend, slackBackend)

	log.Info().
		Bool("enabled", managerConfig.Enabled).
		Bool("email_ready", emailBackend != nil && !emailBackend.IsMock()).
		Bool("slack_ready", slackBackend != nil && !slackBackend.IsMock()).
		Int("event_configs", len(managerConfig.Events)).
		Msg("Notification manager initialized")

	return manager, nil
}

// buildManagerConfig converts config.NotificationsConfig to ManagerConfig
func buildManagerConfig(notifCfg config.NotificationsConfig) ManagerConfig {
	managerConfig := ManagerConfig{
		Enabled:         notifCfg.Enabled,
		Events:          make(map[NotificationType]EventConfig),
		DefaultChannels: []Channel{ChannelSlack}, // Default to Slack if not configured
	}

	// Map config event names to NotificationTypes
	eventMapping := map[string]NotificationType{
		"trade_executed":    NotificationTypeTradeExecution,
		"position_closed":   NotificationTypePositionClosed,
		"safety_guard":      NotificationTypeSafetyGuard,
		"system_error":      NotificationTypeSystemError,
		"daily_summary":     NotificationTypeDailySummary,
		"pnl_alert":         NotificationTypePnLAlert,
		"circuit_breaker":   NotificationTypeCircuitBreaker,
		"consensus_failure": NotificationTypeConsensusFailure,
	}

	// Convert config events to manager events
	for eventName, eventCfg := range notifCfg.Events {
		notifType, exists := eventMapping[eventName]
		if !exists {
			log.Warn().
				Str("event_name", eventName).
				Msg("Unknown notification event type in config, skipping")
			continue
		}

		// Convert channel strings to Channel type
		var channels []Channel
		for _, ch := range eventCfg.Channels {
			switch ch {
			case "email":
				channels = append(channels, ChannelEmail)
			case "slack":
				channels = append(channels, ChannelSlack)
			default:
				log.Warn().
					Str("channel", ch).
					Str("event", eventName).
					Msg("Unknown notification channel in config, skipping")
			}
		}

		managerConfig.Events[notifType] = EventConfig{
			Enabled:  eventCfg.Enabled,
			Channels: channels,
			Priority: eventCfg.Priority,
		}
	}

	// If no events configured, use defaults
	if len(managerConfig.Events) == 0 {
		log.Info().Msg("No notification events configured, using defaults")
		managerConfig = DefaultManagerConfig()
		managerConfig.Enabled = notifCfg.Enabled
	}

	return managerConfig
}
