package telegram

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Bot represents the Telegram bot
type Bot struct {
	api      *tgbotapi.BotAPI
	db       DBPool
	config   *Config
	handlers map[string]CommandHandler
	ctx      context.Context
	cancel   context.CancelFunc
}

// Config holds the bot configuration
type Config struct {
	BotToken        string
	WebhookURL      string
	PollingTimeout  int
	Debug           bool
	OrchestratorURL string // URL for querying orchestrator
}

// CommandHandler is a function that handles a bot command
type CommandHandler func(ctx context.Context, bot *Bot, message *tgbotapi.Message) error

// NewBot creates a new Telegram bot instance
func NewBot(config *Config, db *pgxpool.Pool) (*Bot, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	api, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	api.Debug = config.Debug

	log.Info().
		Str("username", api.Self.UserName).
		Msg("Telegram bot authorized")

	ctx, cancel := context.WithCancel(context.Background())

	bot := &Bot{
		api:      api,
		db:       db,
		config:   config,
		handlers: make(map[string]CommandHandler),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Register default command handlers
	bot.registerDefaultHandlers()

	return bot, nil
}

// registerDefaultHandlers registers all the default bot command handlers
func (b *Bot) registerDefaultHandlers() {
	b.RegisterHandler("start", handleStart)
	b.RegisterHandler("help", handleHelp)
	b.RegisterHandler("status", handleStatus)
	b.RegisterHandler("positions", handlePositions)
	b.RegisterHandler("pl", handlePL)
	b.RegisterHandler("pause", handlePause)
	b.RegisterHandler("resume", handleResume)
	b.RegisterHandler("decisions", handleDecisions)
	b.RegisterHandler("verify", handleVerify)
	b.RegisterHandler("settings", handleSettings)
}

// RegisterHandler registers a command handler
func (b *Bot) RegisterHandler(command string, handler CommandHandler) {
	b.handlers[command] = handler
}

// Start starts the bot in polling or webhook mode
func (b *Bot) Start() error {
	if b.config.WebhookURL != "" {
		return b.startWebhook()
	}
	return b.startPolling()
}

// startPolling starts the bot in polling mode
func (b *Bot) startPolling() error {
	log.Info().Msg("Starting Telegram bot in polling mode")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.config.PollingTimeout

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-b.ctx.Done():
			log.Info().Msg("Telegram bot shutting down")
			return nil
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Handle the message in a goroutine
			go b.handleUpdate(update)
		}
	}
}

// startWebhook starts the bot in webhook mode
func (b *Bot) startWebhook() error {
	log.Info().
		Str("webhook_url", b.config.WebhookURL).
		Msg("Starting Telegram bot in webhook mode")

	webhook, err := tgbotapi.NewWebhook(b.config.WebhookURL)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	_, err = b.api.Request(webhook)
	if err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	info, err := b.api.GetWebhookInfo()
	if err != nil {
		return fmt.Errorf("failed to get webhook info: %w", err)
	}

	if info.LastErrorDate != 0 {
		log.Warn().
			Int("error_date", info.LastErrorDate).
			Str("error_message", info.LastErrorMessage).
			Msg("Telegram webhook has errors")
	}

	updates := b.api.ListenForWebhook("/")

	for update := range updates {
		go b.handleUpdate(update)
	}

	return nil
}

// handleUpdate processes a single update from Telegram
func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	message := update.Message

	// Log the interaction
	b.logMessage(message)

	// Update last interaction time
	if err := b.updateLastInteraction(message.From.ID, message.Chat.ID); err != nil {
		log.Error().
			Err(err).
			Int64("telegram_id", message.From.ID).
			Msg("Failed to update last interaction")
	}

	// Handle commands
	if message.IsCommand() {
		b.handleCommand(message)
		return
	}

	// Handle regular messages (if needed)
	// For now, we'll just acknowledge non-command messages
	msg := tgbotapi.NewMessage(message.Chat.ID, "Please use /help to see available commands.")
	if _, err := b.api.Send(msg); err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
}

// handleCommand processes a command message
func (b *Bot) handleCommand(message *tgbotapi.Message) {
	command := message.Command()

	log.Info().
		Str("command", command).
		Int64("telegram_id", message.From.ID).
		Str("username", message.From.UserName).
		Msg("Received command")

	handler, exists := b.handlers[command]
	if !exists {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Unknown command. Use /help to see available commands.")
		if _, err := b.api.Send(msg); err != nil {
			log.Error().Err(err).Msg("Failed to send unknown command message")
		}
		return
	}

	// Execute the handler
	ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
	defer cancel()

	if err := handler(ctx, b, message); err != nil {
		log.Error().
			Err(err).
			Str("command", command).
			Int64("telegram_id", message.From.ID).
			Msg("Command handler failed")

		// Send error message to user
		errorMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Error executing command: %v", err))
		if _, sendErr := b.api.Send(errorMsg); sendErr != nil {
			log.Error().Err(sendErr).Msg("Failed to send error message")
		}

		// Log to database
		b.logMessageWithError(message, command, err)
	}
}

// SendMessage sends a text message to a chat
func (b *Bot) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, err := b.api.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendAlert sends an alert message with appropriate formatting
func (b *Bot) SendAlert(chatID int64, title, message, severity string) error {
	var emoji string
	switch severity {
	case "CRITICAL":
		emoji = "🚨"
	case "WARNING":
		emoji = "⚠️"
	case "INFO":
		emoji = "ℹ️"
	default:
		emoji = "📢"
	}

	text := fmt.Sprintf("%s *%s*\n\n%s", emoji, title, message)

	return b.SendMessage(chatID, text)
}

// Stop stops the bot gracefully
func (b *Bot) Stop() {
	log.Info().Msg("Stopping Telegram bot")
	b.cancel()
	b.api.StopReceivingUpdates()
}

// GetDB returns the database connection pool
func (b *Bot) GetDB() DBPool {
	return b.db
}

// GetAPI returns the Telegram bot API instance
func (b *Bot) GetAPI() *tgbotapi.BotAPI {
	return b.api
}

// GetConfig returns the bot configuration
func (b *Bot) GetConfig() *Config {
	return b.config
}

// logMessage logs a message to the database
func (b *Bot) logMessage(message *tgbotapi.Message) {
	query := `
		INSERT INTO telegram_messages (telegram_user_id, message_id, chat_id, command, text)
		SELECT id, $1, $2, $3, $4
		FROM telegram_users
		WHERE telegram_id = $5
	`

	command := ""
	if message.IsCommand() {
		command = message.Command()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.db.Exec(ctx, query,
		message.MessageID,
		message.Chat.ID,
		command,
		message.Text,
		message.From.ID,
	)

	if err != nil {
		log.Warn().
			Err(err).
			Int64("telegram_id", message.From.ID).
			Msg("Failed to log message (user might not be registered)")
	}
}

// logMessageWithError logs a message with an error to the database
func (b *Bot) logMessageWithError(message *tgbotapi.Message, command string, cmdErr error) {
	query := `
		INSERT INTO telegram_messages (telegram_user_id, message_id, chat_id, command, text, success, error_message)
		SELECT id, $1, $2, $3, $4, $5, $6
		FROM telegram_users
		WHERE telegram_id = $7
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.db.Exec(ctx, query,
		message.MessageID,
		message.Chat.ID,
		command,
		message.Text,
		false,
		cmdErr.Error(),
		message.From.ID,
	)

	if err != nil {
		log.Error().
			Err(err).
			Int64("telegram_id", message.From.ID).
			Msg("Failed to log message error")
	}
}

// updateLastInteraction updates the last interaction timestamp for a user
func (b *Bot) updateLastInteraction(telegramID int64, chatID int64) error {
	query := `
		INSERT INTO telegram_users (telegram_id, chat_id, last_interaction_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (telegram_id)
		DO UPDATE SET
			last_interaction_at = CURRENT_TIMESTAMP,
			chat_id = EXCLUDED.chat_id
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.db.Exec(ctx, query, telegramID, chatID)
	return err
}
