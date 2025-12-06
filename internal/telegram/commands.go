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
	welcomeText := `Welcome to *CryptoFunk Trading Bot*! 🚀

I'm your AI-powered trading assistant. I can help you monitor your trading sessions, positions, and performance.

*Available Commands:*
/status - Show active sessions and current positions
/positions - List all open positions with P&L
/pl - Show session P&L (realized + unrealized)
/decisions - Show recent agent decisions (last 5)
/pause - Emergency pause trading
/resume - Resume trading after pause
/settings - Manage notification preferences
/help - Show this help message

*First Time Setup:*
To receive alerts and notifications, please verify your account using:
/verify <code>

Get your verification code from the CryptoFunk dashboard.

Happy trading! 📈`

	msg := tgbotapi.NewMessage(message.Chat.ID, welcomeText)
	msg.ParseMode = ParseModeMarkdown

	_, err := bot.api.Send(msg)
	return err
}

// handleHelp handles the /help command
func handleHelp(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	helpText := `*CryptoFunk Trading Bot - Command Reference*

*Monitoring Commands:*
/status - Show active trading sessions and current positions
/positions - List all open positions with detailed P&L
/pl - Show session profit/loss (realized + unrealized)
/decisions - Show the last 5 agent decisions with reasoning

*Control Commands:*
/pause - Emergency pause all trading (positions remain open)
/resume - Resume trading after pause

*Settings Commands:*
/settings - View and manage notification preferences
/verify <code> - Verify your account to receive alerts

*Getting Help:*
/help - Show this help message
/start - Show welcome message

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

	// Build status message
	var sb strings.Builder
	sb.WriteString("*Trading System Status* 📊\n\n")

	// Orchestrator status
	sb.WriteString(fmt.Sprintf("*Orchestrator:* %s\n", status.State))
	if status.IsPaused {
		sb.WriteString("⏸️ *Status:* PAUSED\n")
	} else {
		sb.WriteString("▶️ *Status:* RUNNING\n")
	}
	sb.WriteString(fmt.Sprintf("*Active Agents:* %d\n", status.ActiveAgents))
	sb.WriteString("\n")

	// Active sessions
	if len(sessions) == 0 {
		sb.WriteString("No active trading sessions.\n")
	} else {
		sb.WriteString(fmt.Sprintf("*Active Sessions:* %d\n\n", len(sessions)))
		for i, session := range sessions {
			sb.WriteString(fmt.Sprintf("%d. *%s* (%s)\n", i+1, session.Symbol, session.Mode))
			sb.WriteString(fmt.Sprintf("   Started: %s\n", session.StartedAt.Format("2006-01-02 15:04")))
			sb.WriteString(fmt.Sprintf("   Trades: %d | P&L: %.2f%%\n", session.TotalTrades, session.PnLPercent))
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
			sb.WriteString(fmt.Sprintf("*%d. %s %s*\n", i+1, pos.Side, pos.Symbol))
			sb.WriteString(fmt.Sprintf("   Entry: $%.2f | Current: $%.2f\n", pos.EntryPrice, pos.CurrentPrice))
			sb.WriteString(fmt.Sprintf("   Quantity: %.6f\n", pos.Quantity))
			sb.WriteString(fmt.Sprintf("   P&L: $%.2f (%.2f%%)\n", pos.UnrealizedPnL, pos.PnLPercent))
			if pos.StopLoss > 0 {
				sb.WriteString(fmt.Sprintf("   Stop Loss: $%.2f\n", pos.StopLoss))
			}
			if pos.TakeProfit > 0 {
				sb.WriteString(fmt.Sprintf("   Take Profit: $%.2f\n", pos.TakeProfit))
			}
			sb.WriteString("\n")
			totalPnL += pos.UnrealizedPnL
		}

		sb.WriteString(fmt.Sprintf("*Total Unrealized P&L:* $%.2f\n", totalPnL))
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

	sb.WriteString(fmt.Sprintf("*Realized P&L:* $%.2f\n", pnl.RealizedPnL))
	sb.WriteString(fmt.Sprintf("*Unrealized P&L:* $%.2f\n", pnl.UnrealizedPnL))
	sb.WriteString(fmt.Sprintf("*Total P&L:* $%.2f\n\n", pnl.TotalPnL))

	sb.WriteString(fmt.Sprintf("*Initial Capital:* $%.2f\n", pnl.InitialCapital))
	sb.WriteString(fmt.Sprintf("*Current Value:* $%.2f\n", pnl.CurrentValue))
	sb.WriteString(fmt.Sprintf("*Return:* %.2f%%\n\n", pnl.ReturnPercent))

	sb.WriteString(fmt.Sprintf("*Total Trades:* %d\n", pnl.TotalTrades))
	sb.WriteString(fmt.Sprintf("*Winning Trades:* %d (%.1f%%)\n", pnl.WinningTrades, pnl.WinRate))
	sb.WriteString(fmt.Sprintf("*Losing Trades:* %d (%.1f%%)\n", pnl.LosingTrades, 100-pnl.WinRate))

	if pnl.TotalTrades > 0 {
		sb.WriteString(fmt.Sprintf("\n*Average Win:* $%.2f\n", pnl.AvgWin))
		sb.WriteString(fmt.Sprintf("*Average Loss:* $%.2f\n", pnl.AvgLoss))
		if pnl.AvgLoss != 0 {
			sb.WriteString(fmt.Sprintf("*Win/Loss Ratio:* %.2f\n", pnl.AvgWin/abs(pnl.AvgLoss)))
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
			sb.WriteString(fmt.Sprintf("*%d. %s - %s*\n", i+1, decision.AgentName, decision.Decision))
			sb.WriteString(fmt.Sprintf("   Symbol: %s\n", decision.Symbol))
			sb.WriteString(fmt.Sprintf("   Confidence: %.0f%%\n", decision.Confidence*100))
			sb.WriteString(fmt.Sprintf("   Time: %s\n", decision.CreatedAt.Format("15:04:05")))

			// Truncate reasoning if too long
			reasoning := decision.Reasoning
			if len(reasoning) > 150 {
				reasoning = reasoning[:150] + "..."
			}
			sb.WriteString(fmt.Sprintf("   Reasoning: %s\n", reasoning))
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

	var sb strings.Builder
	sb.WriteString("*Notification Settings* ⚙️\n\n")

	sb.WriteString(fmt.Sprintf("Alerts: %s\n", boolToEmoji(settings.ReceiveAlerts)))
	sb.WriteString(fmt.Sprintf("Trade Notifications: %s\n", boolToEmoji(settings.ReceiveTradeNotifications)))
	sb.WriteString(fmt.Sprintf("Daily Summary: %s\n", boolToEmoji(settings.ReceiveDailySummary)))

	sb.WriteString("\nTo change settings, use the CryptoFunk dashboard.")

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ParseMode = ParseModeMarkdown

	_, err = bot.api.Send(msg)
	return err
}

// handleVerify handles the /verify command
func handleVerify(ctx context.Context, bot *Bot, message *tgbotapi.Message) error {
	args := strings.Fields(message.Text)
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Please provide a verification code: /verify <code>")
		_, err := bot.api.Send(msg)
		return err
	}

	code := strings.ToUpper(args[1])

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
