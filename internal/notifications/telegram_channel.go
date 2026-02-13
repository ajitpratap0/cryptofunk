package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramChannelConfig holds Telegram channel configuration
type TelegramChannelConfig struct {
	BotToken   string
	ChatID     string
	HTTPClient *http.Client
	BaseURL    string // override for testing
}

// TelegramChannel sends notifications via Telegram Bot API
type TelegramChannel struct {
	config TelegramChannelConfig
	client *http.Client
}

// NewTelegramChannel creates a new Telegram notification channel
func NewTelegramChannel(cfg TelegramChannelConfig) *TelegramChannel {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.telegram.org"
	}
	return &TelegramChannel{config: cfg, client: client}
}

func (t *TelegramChannel) Name() string { return "telegram" }
func (t *TelegramChannel) Close() error { return nil }

// Send formats and sends a notification event via Telegram
func (t *TelegramChannel) Send(ctx context.Context, event Event) error {
	text := t.formatMessage(event)

	payload := map[string]interface{}{
		"chat_id":    t.config.ChatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.config.BaseURL, t.config.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}

func (t *TelegramChannel) formatMessage(event Event) string {
	var icon string
	switch event.Priority {
	case PriorityInfo:
		icon = "ℹ️"
	case PriorityWarning:
		icon = "⚠️"
	case PriorityCritical:
		icon = "🔴"
	case PriorityEmergency:
		icon = "🚨"
	}

	msg := icon + " *" + escTg(event.Title) + "*\n"

	switch event.Type {
	case EventTradeExecuted:
		msg += t.formatTrade(event)
	case EventPositionClosed:
		msg += t.formatPositionClosed(event)
	case EventErrorAlert:
		msg += t.formatError(event)
	case EventSafetyAlert:
		msg += t.formatSafety(event)
	case EventDailySummary:
		msg += t.formatDailySummary(event)
	default:
		msg += escTg(event.Message)
	}

	msg += "\n🕐 " + escTg(event.Timestamp.Format("15:04:05"))
	return msg
}

func (t *TelegramChannel) formatTrade(e Event) string {
	f := e.Fields
	side := f["side"]
	emoji := "🟢"
	if side == "SELL" {
		emoji = "🔴"
	}
	return fmt.Sprintf("%s %s %s\n💰 Price: %s\n📊 Size: %s\n🤖 Agent: %s",
		emoji, escTg(side), escTg(f["symbol"]),
		escTg(f["price"]), escTg(f["size"]), escTg(f["agent"]))
}

func (t *TelegramChannel) formatPositionClosed(e Event) string {
	f := e.Fields
	return fmt.Sprintf("📈 %s\n💵 P&L: %s\n⏱ Duration: %s\n📝 Reason: %s",
		escTg(f["symbol"]), escTg(f["pnl"]),
		escTg(f["hold_duration"]), escTg(f["reason"]))
}

func (t *TelegramChannel) formatError(e Event) string {
	return fmt.Sprintf("⚙️ Source: %s\n📝 %s",
		escTg(e.Fields["source"]), escTg(e.Message))
}

func (t *TelegramChannel) formatSafety(e Event) string {
	return fmt.Sprintf("🛡 Type: %s\n📝 %s",
		escTg(e.Fields["alert_type"]), escTg(e.Message))
}

func (t *TelegramChannel) formatDailySummary(e Event) string {
	f := e.Fields
	return fmt.Sprintf("📊 Total Trades: %s\n💵 P&L: %s\n🎯 Win Rate: %s%%\n🏆 Best: %s\n💀 Worst: %s",
		escTg(f["total_trades"]), escTg(f["pnl"]),
		escTg(f["win_rate"]), escTg(f["best_trade"]), escTg(f["worst_trade"]))
}

// escTg escapes special characters for Telegram MarkdownV2
func escTg(s string) string {
	special := []byte{'_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!'}
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		for _, sp := range special {
			if c == sp {
				buf.WriteByte('\\')
				break
			}
		}
		buf.WriteByte(c)
	}
	return buf.String()
}
