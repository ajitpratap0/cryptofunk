package telegram

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const (
	// ParseModeMarkdown is the constant for Markdown parse mode in Telegram messages
	ParseModeMarkdown = "Markdown"
)

// CallbackHandler is a function that handles an inline keyboard callback
type CallbackHandler func(ctx context.Context, bot *Bot, callback *tgbotapi.CallbackQuery) error

// Bot represents the Telegram bot
type Bot struct {
	api              *tgbotapi.BotAPI
	db               DBPool
	config           *Config
	handlers         map[string]CommandHandler
	callbackHandlers map[string]CallbackHandler
	ctx              context.Context
	cancel           context.CancelFunc
	rateLimiter      *RateLimiter
	startTime        time.Time
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

// NewBot creates a new Telegram bot instance.
// Deprecated: Use NewBotWithContext for proper context propagation and graceful shutdown.
func NewBot(config *Config, db *pgxpool.Pool) (*Bot, error) {
	return NewBotWithContext(context.Background(), config, db)
}

// NewBotWithContext creates a new Telegram bot instance with a parent context for lifecycle management.
func NewBotWithContext(parentCtx context.Context, config *Config, db *pgxpool.Pool) (*Bot, error) {
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

	ctx, cancel := context.WithCancel(parentCtx)

	bot := &Bot{
		api:              api,
		db:               db,
		config:           config,
		handlers:         make(map[string]CommandHandler),
		callbackHandlers: make(map[string]CallbackHandler),
		ctx:              ctx,
		cancel:           cancel,
		rateLimiter:      NewRateLimiter(nil), // Use default rate limiter config
		startTime:        time.Now(),
	}

	// Register default command handlers
	bot.registerDefaultHandlers()

	return bot, nil
}

// registerDefaultHandlers registers all the default bot command handlers
func (b *Bot) registerDefaultHandlers() {
	// Core commands
	b.RegisterHandler("start", handleStart)
	b.RegisterHandler("help", handleHelp)
	b.RegisterHandler("status", handleStatus)
	b.RegisterHandler("positions", handlePositions)
	b.RegisterHandler("trades", handleTrades)
	b.RegisterHandler("balance", handleBalance)

	// Trading control
	b.RegisterHandler("starttrade", handleStartTrade)
	b.RegisterHandler("stoptrade", handleStopTrade)
	b.RegisterHandler("agents", handleAgents)

	// Market info
	b.RegisterHandler("price", handlePrice)
	b.RegisterHandler("signals", handleSignals)

	// Settings & legacy
	b.RegisterHandler("settings", handleSettings)
	b.RegisterHandler("pl", handlePL)
	b.RegisterHandler("pause", handlePause)
	b.RegisterHandler("resume", handleResume)
	b.RegisterHandler("decisions", handleDecisions)
	b.RegisterHandler("verify", handleVerify)
	b.RegisterHandler("settings", handleSettings)
	// New TC-001 commands
	b.RegisterHandler("balance", handleBalance)
	b.RegisterHandler("stop", handleStop)
	b.RegisterHandler("startsession", handleStartSession)

	// Inline keyboard callback handlers
	b.RegisterCallbackHandler("confirm_starttrade", handleConfirmStartTrade)
	b.RegisterCallbackHandler("cancel_starttrade", handleCancelStartTrade)
	b.RegisterCallbackHandler("confirm_stoptrade", handleConfirmStopTrade)
	b.RegisterCallbackHandler("cancel_stoptrade", handleCancelStopTrade)
}

// RegisterHandler registers a command handler
func (b *Bot) RegisterHandler(command string, handler CommandHandler) {
	b.handlers[command] = handler
}

// RegisterCallbackHandler registers a callback query handler
func (b *Bot) RegisterCallbackHandler(data string, handler CallbackHandler) {
	b.callbackHandlers[data] = handler
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
			if update.Message == nil && update.CallbackQuery == nil {
				continue
			}

			// Handle the update in a goroutine
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
	// Handle callback queries from inline keyboards
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(update.CallbackQuery)
		return
	}

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
		// Rate limiting
		if !b.rateLimiter.Allow(message.From.ID) {
			msg := tgbotapi.NewMessage(message.Chat.ID, "⚠️ Rate limit exceeded. Please wait a moment before sending more commands.")
			if _, err := b.api.Send(msg); err != nil {
				log.Error().Err(err).Msg("Failed to send rate limit message")
			}
			return
		}
		b.handleCommand(message)
		return
	}

	// Handle regular messages (if needed)
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

	// Check rate limit (skip for help command to always allow users to get help)
	if command != "help" && b.rateLimiter != nil && !b.rateLimiter.Allow(message.From.ID) {
		waitTime := b.rateLimiter.GetWaitTime(message.From.ID)
		rateLimitMsg := fmt.Sprintf("You're sending commands too quickly. Please wait %v before trying again.", waitTime.Round(time.Second))
		msg := tgbotapi.NewMessage(message.Chat.ID, rateLimitMsg)
		if _, err := b.api.Send(msg); err != nil {
			log.Error().Err(err).Msg("Failed to send rate limit message")
		}
		log.Warn().
			Int64("telegram_id", message.From.ID).
			Str("command", command).
			Dur("wait_time", waitTime).
			Msg("Rate limited user")
		return
	}

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

// handleCallbackQuery processes inline keyboard callback queries
func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	data := callback.Data

	log.Info().
		Str("data", data).
		Int64("telegram_id", callback.From.ID).
		Msg("Received callback query")

	handler, exists := b.callbackHandlers[data]
	if !exists {
		answer := tgbotapi.NewCallback(callback.ID, "Unknown action")
		if _, err := b.api.Request(answer); err != nil {
			log.Error().Err(err).Msg("Failed to answer callback query")
		}
		return
	}

	ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
	defer cancel()

	// Answer the callback to remove loading state
	answer := tgbotapi.NewCallback(callback.ID, "")
	if _, err := b.api.Request(answer); err != nil {
		log.Error().Err(err).Msg("Failed to answer callback query")
	}

	if err := handler(ctx, b, callback); err != nil {
		log.Error().Err(err).Str("data", data).Msg("Callback handler failed")
	}
}

// GetStartTime returns when the bot was started
func (b *Bot) GetStartTime() time.Time {
	return b.startTime
}

// SendMessage sends a text message to a chat
func (b *Bot) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = ParseModeMarkdown

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
	if b.rateLimiter != nil {
		b.rateLimiter.Stop()
	}
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

	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
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

	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
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

	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	_, err := b.db.Exec(ctx, query, telegramID, chatID)
	return err
}
