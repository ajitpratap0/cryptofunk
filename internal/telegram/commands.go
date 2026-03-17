package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleStart handles the /start command
func handleStart(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	welcomeText := `Welcome to *CryptoFunk Trading Bot*!

I'm your AI-powered trading assistant. Monitor positions, control trading, and track performance — all from Telegram.

*Available Commands:*

*Monitoring:*
/status - Show active sessions and current positions
/positions - List all open positions with P&L
/balance - Show current account balance
/pl - Show session P&L (realized + unrealized)
/decisions - Show recent agent decisions (last 5)

*Trading Control:*
/startsession - Start a new trading session
/stop - Stop current trading session
/pause - Emergency pause trading
/resume - Resume trading after pause

*Settings:*
/settings - Manage notification preferences
/verify <code> - Verify your account
/help - Show detailed help

*🎮 Trading Control:*
/starttrade — Enable live trading
/stoptrade — Emergency stop all trading
/agents — List all agents & status

*📈 Market Info:*
/price <symbol> — Current price (e.g. /price BTCUSDT)
/signals — Latest trading signals

*⚙️ Settings:*
/settings — Current configuration
/help — All commands

*First Time?* Verify with /verify <code> from your dashboard.

Happy trading!`

	msg := tgbotapi.NewMessage(message.Chat.ID, welcomeText)
	msg.ParseMode = ParseModeMarkdown

	_, err := bot.api.Send(msg)
	return err
}

// handleHelp handles the /help command
func handleHelp(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	helpText := `*CryptoFunk Trading Bot — Command Reference* 📖

*Monitoring Commands:*
/status - Show active trading sessions and current positions
/positions - List all open positions with detailed P&L
/balance - Show current account balance and P&L summary
/pl - Show session profit/loss (realized + unrealized)
/decisions - Show the last 5 agent decisions with reasoning

*Trading Control Commands:*
/startsession <symbol> <capital> [mode] - Start a new trading session
  Example: /startsession BTCUSDT 1000 PAPER
/stop - Stop all active trading sessions (requires confirmation)
/pause - Emergency pause all trading (positions remain open)
/resume - Resume trading after pause

*📈 Market Info:*
/price <symbol> — Current price (e.g. /price BTCUSDT)
/signals — Latest trading signals from agents

*⚙️ Settings & Account:*
/settings — Current config (risk limits, max position)
/verify <code> — Link your Telegram account
/help — This help message

*Examples:*
- Start paper trading: /startsession BTCUSDT 1000
- Start live trading: /startsession ETHUSDT 5000 LIVE
- Stop trading: /stop CONFIRM
- Check balance: /balance

For more information, visit the CryptoFunk dashboard.`

	msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
	msg.ParseMode = ParseModeMarkdown

	_, err := bot.api.Send(msg)
	return err
}

// handleStatus handles the /status command
func handleStatus(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	// Query orchestrator status
	status, err := queryOrchestratorStatus(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to query orchestrator status: %w", err)
	}

	// Query active sessions from database
	sessions, err := getActiveSessions(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to get active sessions: %w", err)
	}

	// Get last trade time
	lastTrade, _ := getLastTradeTime(ctx, bot)

	// Build status message
	var sb strings.Builder
	sb.WriteString("*Trading System Status* 📊\n\n")

	// Orchestrator status
	fmt.Fprintf(&sb, "*Orchestrator:* %s\n", status.State)
	if status.IsPaused {
		sb.WriteString("⏸️ *Status:* PAUSED\n")
	} else {
		sb.WriteString("▶️ *Status:* RUNNING\n")
	}
	fmt.Fprintf(&sb, "*Active Agents:* %d\n", status.ActiveAgents)

	// Uptime
	uptime := time.Since(bot.GetStartTime())
	fmt.Fprintf(&sb, "*Bot Uptime:* %s\n", formatDuration(uptime))

	// Last trade
	if lastTrade != nil {
		fmt.Fprintf(&sb, "*Last Trade:* %s\n", lastTrade.Format("2006-01-02 15:04:05"))
	}
	sb.WriteString("\n")

	// Active sessions
	if len(sessions) == 0 {
		sb.WriteString("No active trading sessions.\n")
	} else {
		fmt.Fprintf(&sb, "*Active Sessions:* %d\n\n", len(sessions))
		for i, session := range sessions {
			fmt.Fprintf(&sb, "%d. *%s* (%s)\n", i+1, session.Symbol, session.Mode)
			fmt.Fprintf(&sb, "   Started: %s\n", session.StartedAt.Format("2006-01-02 15:04"))
			fmt.Fprintf(&sb, "   Trades: %d | P&L: %.2f%%\n", session.TotalTrades, session.PnLPercent)
			sb.WriteString("\n")
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handlePositions handles the /positions command
func handlePositions(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	positions, err := getOpenPositions(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("*Open Positions* 💼\n\n")

	if len(positions) == 0 {
		sb.WriteString("No open positions.\n")
	} else {
		totalPnL := 0.0
		for i, pos := range positions {
			fmt.Fprintf(&sb, "*%d. %s %s*\n", i+1, pos.Side, pos.Symbol)
			fmt.Fprintf(&sb, "   Entry: $%.2f | Current: $%.2f\n", pos.EntryPrice, pos.CurrentPrice)
			fmt.Fprintf(&sb, "   Quantity: %.6f\n", pos.Quantity)
			fmt.Fprintf(&sb, "   P&L: $%.2f (%.2f%%)\n", pos.UnrealizedPnL, pos.PnLPercent)
			if pos.StopLoss > 0 {
				fmt.Fprintf(&sb, "   Stop Loss: $%.2f\n", pos.StopLoss)
			}
			if pos.TakeProfit > 0 {
				fmt.Fprintf(&sb, "   Take Profit: $%.2f\n", pos.TakeProfit)
			}
			sb.WriteString("\n")
			totalPnL += pos.UnrealizedPnL
		}

		fmt.Fprintf(&sb, "*Total Unrealized P&L:* $%.2f\n", totalPnL)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handlePL handles the /pl command
func handlePL(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	pnl, err := getSessionPnL(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to get P&L: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("*Profit & Loss Report* 💰\n\n")

	fmt.Fprintf(&sb, "*Realized P&L:* $%.2f\n", pnl.RealizedPnL)
	fmt.Fprintf(&sb, "*Unrealized P&L:* $%.2f\n", pnl.UnrealizedPnL)
	fmt.Fprintf(&sb, "*Total P&L:* $%.2f\n\n", pnl.TotalPnL)

	fmt.Fprintf(&sb, "*Initial Capital:* $%.2f\n", pnl.InitialCapital)
	fmt.Fprintf(&sb, "*Current Value:* $%.2f\n", pnl.CurrentValue)
	fmt.Fprintf(&sb, "*Return:* %.2f%%\n\n", pnl.ReturnPercent)

	fmt.Fprintf(&sb, "*Total Trades:* %d\n", pnl.TotalTrades)
	fmt.Fprintf(&sb, "*Winning Trades:* %d (%.1f%%)\n", pnl.WinningTrades, pnl.WinRate)
	fmt.Fprintf(&sb, "*Losing Trades:* %d (%.1f%%)\n", pnl.LosingTrades, 100-pnl.WinRate)

	if pnl.TotalTrades > 0 {
		fmt.Fprintf(&sb, "\n*Average Win:* $%.2f\n", pnl.AvgWin)
		fmt.Fprintf(&sb, "*Average Loss:* $%.2f\n", pnl.AvgLoss)
		if pnl.AvgLoss != 0 {
			fmt.Fprintf(&sb, "*Win/Loss Ratio:* %.2f\n", pnl.AvgWin/abs(pnl.AvgLoss))
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handlePause handles the /pause command
func handlePause(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	// Send pause command to orchestrator
	if err := sendOrchestratorCommand(ctx, bot, "pause"); err != nil {
		return fmt.Errorf("failed to pause orchestrator: %w", err)
	}

	responseText := `⏸️ *Trading Paused*

All trading has been paused. Current positions remain open but no new trades will be executed.

Use /resume to resume trading.
Use /positions to check your open positions.`

	msg := tgbotapi.NewMessage(message.Chat.ID, responseText)
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handleResume handles the /resume command
func handleResume(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	// Send resume command to orchestrator
	if err := sendOrchestratorCommand(ctx, bot, "resume"); err != nil {
		return fmt.Errorf("failed to resume orchestrator: %w", err)
	}

	responseText := `▶️ *Trading Resumed*

Trading has been resumed. The system is now actively monitoring markets and executing strategies.

Use /status to check the system status.`

	msg := tgbotapi.NewMessage(message.Chat.ID, responseText)
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handleDecisions handles the /decisions command
func handleDecisions(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	decisions, err := getRecentDecisions(ctx, bot, 5)
	if err != nil {
		return fmt.Errorf("failed to get decisions: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("*Recent Agent Decisions* 🤖\n\n")

	if len(decisions) == 0 {
		sb.WriteString("No recent decisions found.\n")
	} else {
		for i, decision := range decisions {
			fmt.Fprintf(&sb, "*%d. %s - %s*\n", i+1, decision.AgentName, decision.Decision)
			fmt.Fprintf(&sb, "   Symbol: %s\n", decision.Symbol)
			fmt.Fprintf(&sb, "   Confidence: %.0f%%\n", decision.Confidence*100)
			fmt.Fprintf(&sb, "   Time: %s\n", decision.CreatedAt.Format("15:04:05"))

			// Truncate reasoning if too long
			reasoning := decision.Reasoning
			if len(reasoning) > 150 {
				reasoning = reasoning[:150] + "..."
			}
			fmt.Fprintf(&sb, "   Reasoning: %s\n", reasoning)
			sb.WriteString("\n")
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handleSettings handles the /settings command
func handleSettings(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	settings, err := getUserSettings(ctx, bot, message.From.ID)
	if err != nil {
		return fmt.Errorf("failed to get user settings: %w", err)
	}

	// Try to get system config from orchestrator
	sysConfig, _ := getSystemConfig(ctx, bot)

	var sb strings.Builder
	sb.WriteString("*Settings & Configuration* ⚙️\n\n")

	// System config
	if sysConfig != nil {
		sb.WriteString("*Risk Limits:*\n")
		fmt.Fprintf(&sb, "  Max Position Size: $%.2f\n", sysConfig.MaxPositionSize)
		fmt.Fprintf(&sb, "  Max Daily Loss: $%.2f\n", sysConfig.MaxDailyLoss)
		fmt.Fprintf(&sb, "  Max Open Positions: %d\n", sysConfig.MaxOpenPositions)
		fmt.Fprintf(&sb, "  Stop Loss %%: %.1f%%\n\n", sysConfig.StopLossPercent)
	}

	// Notification settings
	sb.WriteString("*Notifications:*\n")
	fmt.Fprintf(&sb, "  Alerts: %s\n", boolToEmoji(settings.ReceiveAlerts))
	fmt.Fprintf(&sb, "  Trade Notifications: %s\n", boolToEmoji(settings.ReceiveTradeNotifications))
	fmt.Fprintf(&sb, "  Daily Summary: %s\n", boolToEmoji(settings.ReceiveDailySummary))

	sb.WriteString("\nTo change settings, use the CryptoFunk dashboard.")

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// SystemConfig holds system-wide configuration
type SystemConfig struct {
	MaxPositionSize  float64 `json:"max_position_size"`
	MaxDailyLoss     float64 `json:"max_daily_loss"`
	MaxOpenPositions int     `json:"max_open_positions"`
	StopLossPercent  float64 `json:"stop_loss_percent"`
}

func getSystemConfig(ctx context.Context, bot *Bot) (*SystemConfig, error) {
	url := fmt.Sprintf("%s/api/v1/config", bot.config.OrchestratorURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("config endpoint returned %d", resp.StatusCode)
	}

	var config SystemConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// handleVerify handles the /verify command
func handleVerify(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	args := strings.Fields(SanitizeInput(message.Text))
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Please provide a verification code: /verify <code>")
		_, err := bot.api.Send(msg)
		return err
	}

	code := strings.ToUpper(args[1])

	// Validate the verification code format
	validator := NewValidator()
	if err := validator.ValidateVerificationCode(code); err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Invalid verification code format: %s", err.(*ValidationError).Message))
		_, sendErr := bot.api.Send(msg)
		return sendErr
	}

	// Verify the code
	verified, err := verifyUser(ctx, bot, message.From.ID, message.Chat.ID, code, message.From)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if !verified {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Invalid or expired verification code. Please get a new code from the dashboard.")
		_, err = bot.api.Send(msg)
		return err
	}

	successText := `✅ *Verification Successful!*

Your Telegram account has been linked to CryptoFunk.

You will now receive:
- Trading alerts and notifications
- Daily performance summaries
- System status updates

Use /settings to manage your notification preferences.
Use /help to see all available commands.`

	msg := tgbotapi.NewMessage(message.Chat.ID, successText)
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// Helper functions

func sendVerificationRequired(bot *Bot, chatID int64) error {
	text := `🔒 *Verification Required*

Please verify your account to use this command.

1. Go to the CryptoFunk dashboard
2. Generate a verification code
3. Use: /verify <code>

If you need help, use /help`

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = ParseModeMarkdown

	_, err := bot.api.Send(msg)
	return err
}

func boolToEmoji(b bool) string {
	if b {
		return "✅ Enabled"
	}
	return "❌ Disabled"
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func getLastTradeTime(ctx context.Context, bot *Bot) (*time.Time, error) {
	query := `SELECT closed_at FROM trades WHERE closed_at IS NOT NULL ORDER BY closed_at DESC LIMIT 1`
	var t time.Time
	err := bot.db.QueryRow(ctx, query).Scan(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// API response structures

type OrchestratorStatus struct {
	State        string `json:"state"`
	IsPaused     bool   `json:"is_paused"`
	ActiveAgents int    `json:"active_agents"`
}

type SessionInfo struct {
	Symbol      string    `json:"symbol"`
	Mode        string    `json:"mode"`
	StartedAt   time.Time `json:"started_at"`
	TotalTrades int       `json:"total_trades"`
	PnLPercent  float64   `json:"pnl_percent"`
}

type Position struct {
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	EntryPrice    float64 `json:"entry_price"`
	CurrentPrice  float64 `json:"current_price"`
	Quantity      float64 `json:"quantity"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	PnLPercent    float64 `json:"pnl_percent"`
	StopLoss      float64 `json:"stop_loss"`
	TakeProfit    float64 `json:"take_profit"`
}

type PnLReport struct {
	RealizedPnL    float64 `json:"realized_pnl"`
	UnrealizedPnL  float64 `json:"unrealized_pnl"`
	TotalPnL       float64 `json:"total_pnl"`
	InitialCapital float64 `json:"initial_capital"`
	CurrentValue   float64 `json:"current_value"`
	ReturnPercent  float64 `json:"return_percent"`
	TotalTrades    int     `json:"total_trades"`
	WinningTrades  int     `json:"winning_trades"`
	LosingTrades   int     `json:"losing_trades"`
	WinRate        float64 `json:"win_rate"`
	AvgWin         float64 `json:"avg_win"`
	AvgLoss        float64 `json:"avg_loss"`
}

type Decision struct {
	AgentName  string    `json:"agent_name"`
	Decision   string    `json:"decision"`
	Symbol     string    `json:"symbol"`
	Confidence float64   `json:"confidence"`
	Reasoning  string    `json:"reasoning"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserSettings struct {
	ReceiveAlerts             bool `json:"receive_alerts"`
	ReceiveTradeNotifications bool `json:"receive_trade_notifications"`
	ReceiveDailySummary       bool `json:"receive_daily_summary"`
}

// AccountBalance represents account balance information
type AccountBalance struct {
	TotalBalance     float64 `json:"total_balance"`
	AvailableBalance float64 `json:"available_balance"`
	InPositions      float64 `json:"in_positions"`
	TotalPnL         float64 `json:"total_pnl"`
	TodayPnL         float64 `json:"today_pnl"`
	Currency         string  `json:"currency"`
}

// SessionStartRequest represents a request to start a trading session
type SessionStartRequest struct {
	Symbol         string  `json:"symbol"`
	InitialCapital float64 `json:"initial_capital"`
	Mode           string  `json:"mode"` // PAPER or LIVE
}

// SessionStopRequest represents a request to stop a trading session
type SessionStopRequest struct {
	SessionID    string  `json:"session_id"`
	FinalCapital float64 `json:"final_capital"`
}

// queryOrchestratorStatus queries the orchestrator for its current status
func queryOrchestratorStatus(ctx context.Context, bot *Bot) (*OrchestratorStatus, error) {
	url := fmt.Sprintf("%s/api/v1/orchestrator/status", bot.config.OrchestratorURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("orchestrator returned status %d: %s", resp.StatusCode, body)
	}

	var status OrchestratorStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// sendOrchestratorCommand sends a command to the orchestrator
func sendOrchestratorCommand(ctx context.Context, bot *Bot, command string) error {
	url := fmt.Sprintf("%s/api/v1/orchestrator/%s", bot.config.OrchestratorURL, command)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("orchestrator returned status %d: %s", resp.StatusCode, body)
	}

	return nil
}

// handleStop handles the /stop command
func handleStop(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	// Check for confirmation argument
	args := strings.Fields(message.Text)
	hasConfirmation := len(args) > 1 && strings.ToUpper(args[1]) == "CONFIRM"

	// Get active sessions
	sessions, err := getActiveSessions(ctx, bot)
	if err != nil {
		return fmt.Errorf("failed to get active sessions: %w", err)
	}

	if len(sessions) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "No active trading sessions to stop.")
		_, err = bot.api.Send(msg)
		return err
	}

	if !hasConfirmation {
		// Show confirmation prompt
		var sb strings.Builder
		sb.WriteString("*Stop Trading Session*\n\n")
		sb.WriteString("Are you sure you want to stop all active trading sessions?\n\n")

		sb.WriteString("*Active Sessions:*\n")
		for i, session := range sessions {
			fmt.Fprintf(&sb, "%d. %s (%s) - P&L: %.2f%%\n", i+1, session.Symbol, session.Mode, session.PnLPercent)
		}

		sb.WriteString("\nTo confirm, type: `/stop CONFIRM`\n")
		sb.WriteString("\n*Warning:* This will stop all trading. Open positions will remain but no new trades will be executed.")

		msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
		msg.ParseMode = ParseModeMarkdown

		_, err = bot.api.Send(msg)
		return err
	}

	// User confirmed, stop all sessions
	stoppedCount := 0
	var stopErrors []string

	for _, session := range sessions {
		if err := stopTradingSession(ctx, bot, session.Symbol); err != nil {
			stopErrors = append(stopErrors, fmt.Sprintf("%s: %v", session.Symbol, err))
		} else {
			stoppedCount++
		}
	}

	var sb strings.Builder
	if stoppedCount > 0 {
		fmt.Fprintf(&sb, "*Trading Stopped*\n\nStopped %d session(s).\n", stoppedCount)
	}

	if len(stopErrors) > 0 {
		sb.WriteString("\n*Errors:*\n")
		for _, e := range stopErrors {
			fmt.Fprintf(&sb, "- %s\n", e)
		}
	}

	sb.WriteString("\nUse /status to check system status.\nUse /positions to check remaining open positions.")

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handleStartSession handles the /startsession command to start a new trading session
func handleStartSession(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	// Check if user is verified
	verified, err := isUserVerified(ctx, bot, message.From.ID)
	if err != nil {
		return err
	}
	if !verified {
		return sendVerificationRequired(bot, message.Chat.ID)
	}

	// Parse arguments: /startsession [symbol] [capital] [mode]
	args := strings.Fields(SanitizeInput(message.Text))

	if len(args) < 3 {
		helpText := `*Start Trading Session*

Usage: /startsession <symbol> <capital> [mode]

Parameters:
- *symbol*: Trading pair (e.g., BTCUSDT, ETHUSDT)
- *capital*: Initial capital in USD (min: $10, max: $10,000,000)
- *mode*: Optional, PAPER (default) or LIVE

Examples:
/startsession BTCUSDT 1000
/startsession ETHUSDT 500 PAPER
/startsession BTCUSDT 10000 LIVE

*Warning:* LIVE mode uses real money. Use PAPER mode for testing.`

		msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
		msg.ParseMode = ParseModeMarkdown

		_, err = bot.api.Send(msg)
		return err
	}

	symbol := strings.ToUpper(args[1])
	capitalStr := args[2]
	mode := "PAPER"
	if len(args) > 3 {
		mode = strings.ToUpper(args[3])
	}

	// Validate inputs using the validator
	validator := NewValidator()

	// Validate symbol
	if err := validator.ValidateSymbol(symbol); err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Invalid symbol: %s\n\nSupported formats: BTCUSDT, ETHUSDT, SOLUSDT, etc.", err.(*ValidationError).Message))
		_, sendErr := bot.api.Send(msg)
		return sendErr
	}

	// Validate mode
	if err := validator.ValidateTradingMode(mode); err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Invalid mode. Use PAPER or LIVE.")
		_, sendErr := bot.api.Send(msg)
		return sendErr
	}

	// Parse and validate capital
	capital, parseErr := ParseCapital(capitalStr)
	if parseErr != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Invalid capital: %s", parseErr.Error()))
		_, sendErr := bot.api.Send(msg)
		return sendErr
	}

	if err := validator.ValidateCapital(capital); err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Invalid capital: %s", err.(*ValidationError).Message))
		_, sendErr := bot.api.Send(msg)
		return sendErr
	}

	// Confirmation for LIVE mode
	hasConfirmation := len(args) > 4 && validator.ValidateConfirmation(args[4])
	if mode == "LIVE" && !hasConfirmation {
		confirmText := fmt.Sprintf(`*LIVE Trading Confirmation Required*

You are about to start LIVE trading with real money!

*Symbol:* %s
*Capital:* %s
*Mode:* LIVE

To confirm, type:
/startsession %s %.2f LIVE CONFIRM

*Warning:* LIVE trading involves real financial risk.`, symbol, FormatCurrency(capital), symbol, capital)

		msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
		msg.ParseMode = ParseModeMarkdown

		_, err = bot.api.Send(msg)
		return err
	}

	// Start the session
	sessionID, err := startTradingSession(ctx, bot, symbol, capital, mode)
	if err != nil {
		return fmt.Errorf("failed to start trading session: %w", err)
	}

	successText := fmt.Sprintf(`*Trading Session Started*

*Session ID:* %s
*Symbol:* %s
*Capital:* %s
*Mode:* %s

Use /status to monitor your session.
Use /positions to view open positions.
Use /stop to stop trading.`, sessionID, symbol, FormatCurrency(capital), mode)

	msg := tgbotapi.NewMessage(message.Chat.ID, successText)
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}
