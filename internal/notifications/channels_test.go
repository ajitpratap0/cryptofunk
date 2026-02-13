package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestTelegramChannelSend(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "test-token",
		ChatID:   "12345",
		BaseURL:  srv.URL,
	})

	event := TradeExecutedEvent("ETHUSDT", "BUY", "trend-agent", 3000.50, 1.5)
	err := ch.Send(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received["chat_id"] != "12345" {
		t.Fatalf("wrong chat_id: %v", received["chat_id"])
	}
	text := received["text"].(string)
	if !strings.Contains(text, "ETHUSDT") {
		t.Fatalf("message should contain symbol: %s", text)
	}
}

func TestTelegramChannelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "bad-token",
		ChatID:   "12345",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), Event{Title: "test", Timestamp: time.Now()})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

type slackPayload struct {
	Text        string `json:"text"`
	Channel     string `json:"channel"`
	Attachments []struct {
		Color  string `json:"color"`
		Title  string `json:"title"`
		Text   string `json:"text"`
		Fields []struct {
			Title string `json:"title"`
			Value string `json:"value"`
		} `json:"fields"`
	} `json:"attachments"`
}

func TestSlackChannelSend(t *testing.T) {
	var received slackPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ch := NewSlackChannel(SlackChannelConfig{WebhookURL: srv.URL})

	event := SafetyAlertEvent("kill_switch", "Drawdown limit hit")
	err := ch.Send(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(received.Attachments))
	}
	att := received.Attachments[0]
	if att.Color != "#9C27B0" { // emergency = purple
		t.Fatalf("wrong color for emergency: %s", att.Color)
	}
	if !strings.Contains(att.Title, "Safety Alert") {
		t.Fatalf("title should contain Safety Alert: %s", att.Title)
	}
}

func TestSlackChannelColors(t *testing.T) {
	tests := []struct {
		priority Priority
		color    string
	}{
		{PriorityInfo, "#36a64f"},
		{PriorityWarning, "#ffcc00"},
		{PriorityCritical, "#ff0000"},
		{PriorityEmergency, "#9C27B0"},
	}
	for _, tt := range tests {
		if got := slackColor(tt.priority); got != tt.color {
			t.Errorf("slackColor(%v) = %s, want %s", tt.priority, got, tt.color)
		}
	}
}

func TestEmailChannelSend(t *testing.T) {
	var sentMsg []byte
	ch := NewEmailChannel(EmailChannelConfig{
		SMTPHost: "localhost",
		SMTPPort: "25",
		From:     "bot@test.com",
		To:       "user@test.com",
		SendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			sentMsg = msg
			return nil
		},
	})

	event := DailySummaryEvent(42, 1234.56, 65.5, "BTCUSDT +500", "ETHUSDT -200")
	err := ch.Send(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgStr := string(sentMsg)
	if !strings.Contains(msgStr, "Daily Summary") {
		t.Fatal("email should contain title")
	}
	if !strings.Contains(msgStr, "text/html") {
		t.Fatal("email should be HTML")
	}
}

func TestEmailChannelBatchMode(t *testing.T) {
	sendCount := 0
	ch := NewEmailChannel(EmailChannelConfig{
		SMTPHost:  "localhost",
		SMTPPort:  "25",
		From:      "bot@test.com",
		To:        "user@test.com",
		BatchMode: true,
		SendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			sendCount++
			return nil
		},
	})

	// These should be batched, not sent immediately
	_ = ch.Send(context.Background(), TradeExecutedEvent("BTC", "BUY", "a", 1, 1))
	_ = ch.Send(context.Background(), TradeExecutedEvent("ETH", "SELL", "a", 2, 2))

	if sendCount != 0 {
		t.Fatal("batch mode should not send immediately")
	}
	if ch.BatchedCount() != 2 {
		t.Fatalf("expected 2 batched, got %d", ch.BatchedCount())
	}

	// Flush digest
	err := ch.FlushDigest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sendCount != 1 {
		t.Fatalf("expected 1 digest email, got %d", sendCount)
	}
	if ch.BatchedCount() != 0 {
		t.Fatal("batch should be cleared after flush")
	}
}

func TestEventCreators(t *testing.T) {
	e := TradeExecutedEvent("BTCUSDT", "BUY", "trend", 50000, 0.5)
	if e.Type != EventTradeExecuted {
		t.Fatal("wrong type")
	}
	if e.Fields["symbol"] != "BTCUSDT" {
		t.Fatal("wrong symbol")
	}

	e2 := PositionClosedEvent("ETHUSDT", -100, 2*time.Hour, "stop-loss")
	if e2.Priority != PriorityWarning {
		t.Fatal("negative P&L should be warning")
	}

	e3 := ErrorAlertEvent("binance-api", "timeout")
	if e3.Priority != PriorityCritical {
		t.Fatal("error should be critical")
	}

	e4 := SafetyAlertEvent("kill_switch", "activated")
	if e4.Priority != PriorityEmergency {
		t.Fatal("safety should be emergency")
	}
}

func TestEscTg(t *testing.T) {
	got := escTg("hello_world*test")
	if got != "hello\\_world\\*test" {
		t.Fatalf("escTg wrong: %s", got)
	}
}
