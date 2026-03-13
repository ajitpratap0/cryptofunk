package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

const (
	emojiGreen = "🟢"
	emojiRed   = "🔴"
)

// Trade represents a completed trade
type Trade struct {
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"`
	EntryAt   time.Time `json:"entry_at"`
	ExitAt    time.Time `json:"exit_at"`
	Entry     float64   `json:"entry_price"`
	Exit      float64   `json:"exit_price"`
	Quantity  float64   `json:"quantity"`
	PnL       float64   `json:"pnl"`
	PnLPct    float64   `json:"pnl_percent"`
	SessionID string    `json:"session_id"`
}

// BalanceInfo holds portfolio balance information
type BalanceInfo struct {
	TotalBalance    float64 `json:"total_balance"`
	AvailableMargin float64 `json:"available_margin"`
	UsedMargin      float64 `json:"used_margin"`
	TotalPnL        float64 `json:"total_pnl"`
	TotalPnLPct     float64 `json:"total_pnl_percent"`
	InitialCapital  float64 `json:"initial_capital"`
}

// AgentInfo holds information about a trading agent
type AgentInfo struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Symbol    string    `json:"symbol"`
	LastRun   time.Time `json:"last_run"`
	Decisions int       `json:"decisions"`
}

// Signal holds a trading signal from an agent
type Signal struct {
	AgentName  string    `json:"agent_name"`
	Symbol     string    `json:"symbol"`
	Signal     string    `json:"signal"`
	Confidence float64   `json:"confidence"`
	Price      float64   `json:"price"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleTrades handles the /trades command - recent trade history
func handleTrades(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	trades, err := getRecentTrades(ctx, bot, 10)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("*Recent Trades* 📜\n\n")

	if len(trades) == 0 {
		sb.WriteString("No recent trades found.\n")
	} else {
		totalPnL := 0.0
		for i, t := range trades {
			emoji := emojiGreen
			if t.PnL < 0 {
				emoji = emojiRed
			}
			fmt.Fprintf(&sb, "%s *%d. %s %s*\n", emoji, i+1, t.Side, t.Symbol)
			fmt.Fprintf(&sb, "   Entry: $%.2f → Exit: $%.2f\n", t.Entry, t.Exit)
			fmt.Fprintf(&sb, "   Qty: %.6f\n", t.Quantity)
			fmt.Fprintf(&sb, "   P&L: $%.2f (%.2f%%)\n", t.PnL, t.PnLPct)
			fmt.Fprintf(&sb, "   %s\n\n", t.ExitAt.Format("2006-01-02 15:04:05"))
			totalPnL += t.PnL
		}
		fmt.Fprintf(&sb, "*Total P&L (shown):* $%.2f\n", totalPnL)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown
	_, err = bot.api.Send(msg)
	return err
}

// handleBalance handles the /balance command
func handleBalance(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	balance, err := getBalance(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	emoji := "📈"
	if balance.TotalPnL < 0 {
		emoji = "📉"
	}

	var sb strings.Builder
	sb.WriteString("*Portfolio Balance* 💰\n\n")
	fmt.Fprintf(&sb, "*Total Balance:* $%.2f\n", balance.TotalBalance)
	fmt.Fprintf(&sb, "*Available Margin:* $%.2f\n", balance.AvailableMargin)
	fmt.Fprintf(&sb, "*Used Margin:* $%.2f\n\n", balance.UsedMargin)
	fmt.Fprintf(&sb, "*Initial Capital:* $%.2f\n", balance.InitialCapital)
	fmt.Fprintf(&sb, "%s *Total P&L:* $%.2f (%.2f%%)\n", emoji, balance.TotalPnL, balance.TotalPnLPct)

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown
	_, err = bot.api.Send(msg)
	return err
}

// handleStartTrade handles the /starttrade command with confirmation
func handleStartTrade(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	text := "⚠️ *Enable Live Trading?*\n\nThis will activate all trading strategies. Are you sure?"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "confirm_starttrade"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_starttrade"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = ParseModeMarkdown
	msg.ReplyMarkup = keyboard
	_, err = bot.api.Send(msg)
	return err
}

// handleStopTrade handles the /stoptrade command with confirmation
func handleStopTrade(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	text := "🛑 *Emergency Stop Trading?*\n\nThis will pause ALL trading immediately. Open positions will remain but no new trades will execute."

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛑 Stop All", "confirm_stoptrade"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_stoptrade"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = ParseModeMarkdown
	msg.ReplyMarkup = keyboard
	_, err = bot.api.Send(msg)
	return err
}

// handleAgents handles the /agents command
func handleAgents(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	agents, err := getAgents(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to get agents: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("*Trading Agents* 🤖\n\n")

	if len(agents) == 0 {
		sb.WriteString("No agents configured.\n")
	} else {
		for i, a := range agents {
			statusEmoji := "⏹️"
			switch a.Status {
			case "running":
				statusEmoji = emojiGreen
			case "stopped":
				statusEmoji = emojiRed
			case "error":
				statusEmoji = "⚠️"
			case "paused":
				statusEmoji = "⏸️"
			}
			fmt.Fprintf(&sb, "%s *%d. %s*\n", statusEmoji, i+1, a.Name)
			fmt.Fprintf(&sb, "   Status: %s | Symbol: %s\n", a.Status, a.Symbol)
			fmt.Fprintf(&sb, "   Decisions: %d\n", a.Decisions)
			if !a.LastRun.IsZero() {
				fmt.Fprintf(&sb, "   Last run: %s\n", a.LastRun.Format("15:04:05"))
			}
			sb.WriteString("\n")
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown
	_, err = bot.api.Send(msg)
	return err
}

// handlePrice handles the /price <symbol> command
func handlePrice(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	args := strings.Fields(message.Text)
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Usage: /price <symbol>\nExample: /price BTCUSDT")
		_, err := bot.api.Send(msg)
		return err
	}

	symbol := strings.ToUpper(args[1])
	price, err := getSymbolPrice(ctx, bot, symbol)
	if err != nil {
		return fmt.Errorf("failed to get price for %s: %w", symbol, err)
	}

	text := fmt.Sprintf("💲 *%s*\n\nPrice: $%.2f\nUpdated: %s", symbol, price.Price, price.UpdatedAt.Format("15:04:05"))

	if price.Change24h != 0 {
		changeEmoji := "📈"
		if price.Change24h < 0 {
			changeEmoji = "📉"
		}
		text += fmt.Sprintf("\n%s 24h Change: %.2f%%", changeEmoji, price.Change24h)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = ParseModeMarkdown
	_, err = bot.api.Send(msg)
	return err
}

// handleSignals handles the /signals command
func handleSignals(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	signals, err := getLatestSignals(ctx, bot, 10)
	if err != nil {
		return fmt.Errorf("failed to get signals: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("*Latest Trading Signals* 📡\n\n")

	if len(signals) == 0 {
		sb.WriteString("No recent signals.\n")
	} else {
		for i, s := range signals {
			emoji := "⬜"
			switch strings.ToUpper(s.Signal) {
			case "BUY":
				emoji = "🟢"
			case "SELL":
				emoji = emojiRed
			case "HOLD":
				emoji = "🟡"
			}
			fmt.Fprintf(&sb, "%s *%d. %s — %s %s*\n", emoji, i+1, s.AgentName, s.Signal, s.Symbol)
			fmt.Fprintf(&sb, "   Confidence: %.0f%% | Price: $%.2f\n", s.Confidence*100, s.Price)
			fmt.Fprintf(&sb, "   %s\n\n", s.CreatedAt.Format("15:04:05"))
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown
	_, err = bot.api.Send(msg)
	return err
}

// Callback handlers for inline keyboards

func handleConfirmStartTrade(ctx context.Context, bot *Bot, callback *tgbotapi.CallbackQuery) error {
	if err := sendOrchestratorCommand(ctx, bot, "resume"); err != nil {
		log.Error().Err(err).Msg("Failed to start trading via orchestrator")
		text := "❌ Failed to enable trading. Please try again."
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, text)
		_, err = bot.api.Send(msg)
		return err
	}

	// Edit the original message to show confirmation
	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
		"✅ *Live Trading Enabled*\n\nAll strategies are now active. Use /status to monitor.")
	edit.ParseMode = ParseModeMarkdown
	_, err := bot.api.Send(edit)
	return err
}

func handleCancelStartTrade(ctx context.Context, bot *Bot, callback *tgbotapi.CallbackQuery) error {
	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
		"❌ Start trading cancelled.")
	_, err := bot.api.Send(edit)
	return err
}

func handleConfirmStopTrade(ctx context.Context, bot *Bot, callback *tgbotapi.CallbackQuery) error {
	if err := sendOrchestratorCommand(ctx, bot, "pause"); err != nil {
		log.Error().Err(err).Msg("Failed to stop trading via orchestrator")
		text := "❌ Failed to stop trading. Please try again or contact support."
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, text)
		_, err = bot.api.Send(msg)
		return err
	}

	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
		"🛑 *All Trading Stopped*\n\nNo new trades will be executed. Open positions remain.\n\nUse /starttrade to resume.")
	edit.ParseMode = ParseModeMarkdown
	_, err := bot.api.Send(edit)
	return err
}

func handleCancelStopTrade(ctx context.Context, bot *Bot, callback *tgbotapi.CallbackQuery) error {
	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
		"Trading continues as normal. ✅")
	_, err := bot.api.Send(edit)
	return err
}

// PriceInfo holds current price data
type PriceInfo struct {
	Price     float64   `json:"price"`
	Change24h float64   `json:"change_24h"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Data access functions

func getRecentTrades(ctx context.Context, bot *Bot, limit int) ([]Trade, error) {
	query := `
		SELECT
			symbol, side::text, entry_price, exit_price, quantity,
			pnl, COALESCE((pnl / NULLIF(entry_price * quantity, 0)) * 100, 0) as pnl_percent,
			opened_at, closed_at, session_id::text
		FROM trades
		WHERE closed_at IS NOT NULL
		ORDER BY closed_at DESC
		LIMIT $1
	`

	rows, err := bot.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		err := rows.Scan(&t.Symbol, &t.Side, &t.Entry, &t.Exit, &t.Quantity,
			&t.PnL, &t.PnLPct, &t.EntryAt, &t.ExitAt, &t.SessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade: %w", err)
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

func getBalance(ctx context.Context, bot *Bot) (*BalanceInfo, error) {
	query := `
		SELECT
			COALESCE(SUM(initial_capital), 0) as initial_capital,
			COALESCE(SUM(initial_capital + total_pnl), 0) as total_balance,
			COALESCE(SUM(total_pnl), 0) as total_pnl
		FROM trading_sessions
		WHERE stopped_at IS NULL
	`

	var balance BalanceInfo
	err := bot.db.QueryRow(ctx, query).Scan(&balance.InitialCapital, &balance.TotalBalance, &balance.TotalPnL)
	if err != nil {
		return nil, fmt.Errorf("failed to query balance: %w", err)
	}

	// Get unrealized P&L from positions for margin calculation
	positions, err := getOpenPositions(ctx, bot)
	if err != nil {
		return nil, err
	}

	for _, pos := range positions {
		balance.UsedMargin += pos.EntryPrice * pos.Quantity
		balance.TotalPnL += pos.UnrealizedPnL
	}

	balance.TotalBalance += balance.TotalPnL - balance.TotalPnL // already included
	balance.AvailableMargin = balance.TotalBalance - balance.UsedMargin
	if balance.InitialCapital > 0 {
		balance.TotalPnLPct = (balance.TotalPnL / balance.InitialCapital) * 100
	}

	return &balance, nil
}

func getAgents(ctx context.Context, bot *Bot) ([]AgentInfo, error) {
	url := fmt.Sprintf("%s/api/v1/agents", bot.config.OrchestratorURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Fallback to DB if orchestrator unavailable
		return getAgentsFromDB(ctx, bot)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return getAgentsFromDB(ctx, bot)
	}

	var agents []AgentInfo
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func getAgentsFromDB(ctx context.Context, bot *Bot) ([]AgentInfo, error) {
	query := `
		SELECT
			agent_name,
			COUNT(*) as decisions,
			MAX(created_at) as last_run
		FROM llm_decisions
		GROUP BY agent_name
		ORDER BY last_run DESC
	`

	rows, err := bot.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []AgentInfo
	for rows.Next() {
		var a AgentInfo
		err := rows.Scan(&a.Name, &a.Decisions, &a.LastRun)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}
		a.Status = "unknown"
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func getSymbolPrice(ctx context.Context, bot *Bot, symbol string) (*PriceInfo, error) {
	// Try orchestrator/market data first
	url := fmt.Sprintf("%s/api/v1/price/%s", bot.config.OrchestratorURL, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var price PriceInfo
		if err := json.NewDecoder(resp.Body).Decode(&price); err == nil {
			return &price, nil
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback: get latest candlestick from DB
	query := `
		SELECT close, open_time
		FROM candlesticks
		WHERE symbol = $1
		ORDER BY open_time DESC
		LIMIT 1
	`

	var price PriceInfo
	err = bot.db.QueryRow(ctx, query, symbol).Scan(&price.Price, &price.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("no price data found for %s", symbol)
	}

	// Try to get 24h change
	query24h := `
		SELECT close
		FROM candlesticks
		WHERE symbol = $1
		AND open_time <= $2 - INTERVAL '24 hours'
		ORDER BY open_time DESC
		LIMIT 1
	`
	var oldPrice float64
	if err := bot.db.QueryRow(ctx, query24h, symbol, price.UpdatedAt).Scan(&oldPrice); err == nil && oldPrice > 0 {
		price.Change24h = ((price.Price - oldPrice) / oldPrice) * 100
	}

	return &price, nil
}

func getLatestSignals(ctx context.Context, bot *Bot, limit int) ([]Signal, error) {
	query := `
		SELECT
			agent_name,
			symbol,
			decision,
			confidence,
			COALESCE(
				(SELECT close FROM candlesticks WHERE symbol = d.symbol ORDER BY open_time DESC LIMIT 1),
				0
			) as price,
			created_at
		FROM llm_decisions d
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := bot.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query signals: %w", err)
	}
	defer rows.Close()

	var signals []Signal
	for rows.Next() {
		var s Signal
		err := rows.Scan(&s.AgentName, &s.Symbol, &s.Signal, &s.Confidence, &s.Price, &s.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan signal: %w", err)
		}
		signals = append(signals, s)
	}
	return signals, rows.Err()
}
