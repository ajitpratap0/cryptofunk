package alerts

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

// TelegramAlerter sends alerts via Telegram bot
type TelegramAlerter struct {
	api     *tgbotapi.BotAPI
	chatIDs []int64
}

// NewTelegramAlerter creates a new Telegram-based alerter
// botToken: Telegram bot API token
// chatIDs: List of chat IDs to send alerts to
func NewTelegramAlerter(botToken string, chatIDs []int64) (*TelegramAlerter, error) {
	if botToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	api, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	log.Info().
		Str("bot_username", api.Self.UserName).
		Int("chat_count", len(chatIDs)).
		Msg("Telegram alerter initialized")

	return &TelegramAlerter{
		api:     api,
		chatIDs: chatIDs,
	}, nil
}

// Send sends an alert via Telegram
func (t *TelegramAlerter) Send(ctx context.Context, alert Alert) error {
	if len(t.chatIDs) == 0 {
		log.Warn().Msg("No Telegram chat IDs configured, skipping alert")
		return nil
	}

	// Format the alert message
	message := t.formatAlert(alert)

	// Send to all configured chat IDs
	var lastErr error
	successCount := 0

	for _, chatID := range t.chatIDs {
		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = "Markdown"

		_, err := t.api.Send(msg)
		if err != nil {
			log.Error().
				Err(err).
				Int64("chat_id", chatID).
				Str("alert_title", alert.Title).
				Msg("Failed to send Telegram alert")
			lastErr = err
			continue
		}

		successCount++
	}

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("failed to send alert to any chat: %w", lastErr)
	}

	log.Debug().
		Int("success_count", successCount).
		Int("total_chats", len(t.chatIDs)).
		Str("alert_title", alert.Title).
		Msg("Telegram alert sent")

	return nil
}

// formatAlert formats an alert for Telegram
func (t *TelegramAlerter) formatAlert(alert Alert) string {
	var emoji string
	switch alert.Severity {
	case SeverityCritical:
		emoji = "🚨"
	case SeverityWarning:
		emoji = "⚠️"
	case SeverityInfo:
		emoji = "ℹ️"
	default:
		emoji = "📢"
	}

	// Build message
	message := fmt.Sprintf("%s *%s*\n\n%s", emoji, alert.Title, alert.Message)

	// Add metadata if present
	if len(alert.Metadata) > 0 {
		message += "\n\n*Details:*"
		for key, value := range alert.Metadata {
			message += fmt.Sprintf("\n• %s: `%v`", key, value)
		}
	}

	// Add timestamp
	message += fmt.Sprintf("\n\n_Time: %s_", alert.Timestamp.Format("2006-01-02 15:04:05"))

	return message
}

// AddChatID adds a chat ID to the alerter
func (t *TelegramAlerter) AddChatID(chatID int64) {
	// Check if chat ID already exists
	for _, id := range t.chatIDs {
		if id == chatID {
			return
		}
	}
	t.chatIDs = append(t.chatIDs, chatID)
	log.Info().
		Int64("chat_id", chatID).
		Msg("Added chat ID to Telegram alerter")
}

// RemoveChatID removes a chat ID from the alerter
func (t *TelegramAlerter) RemoveChatID(chatID int64) {
	for i, id := range t.chatIDs {
		if id == chatID {
			t.chatIDs = append(t.chatIDs[:i], t.chatIDs[i+1:]...)
			log.Info().
				Int64("chat_id", chatID).
				Msg("Removed chat ID from Telegram alerter")
			return
		}
	}
}

// GetChatIDs returns the list of configured chat IDs
func (t *TelegramAlerter) GetChatIDs() []int64 {
	return t.chatIDs
}

// SetChatIDs sets the list of chat IDs
func (t *TelegramAlerter) SetChatIDs(chatIDs []int64) {
	t.chatIDs = chatIDs
	log.Info().
		Int("chat_count", len(chatIDs)).
		Msg("Updated Telegram alerter chat IDs")
}
