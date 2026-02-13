package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramChannel_Name(t *testing.T) {
	ch := NewTelegramChannel(TelegramChannelConfig{BotToken: "tok", ChatID: "123"})
	assert.Equal(t, "telegram", ch.Name())
}

func TestTelegramChannel_Close(t *testing.T) {
	ch := NewTelegramChannel(TelegramChannelConfig{BotToken: "tok", ChatID: "123"})
	assert.NoError(t, ch.Close())
}

func TestTelegramChannel_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "testtoken",
		ChatID:   "12345",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), TradeExecutedEvent("BTCUSDT", "BUY", "agent1", 50000, 0.1))
	require.NoError(t, err)
}

func TestTelegramChannel_Send_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "testtoken",
		ChatID:   "12345",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), TradeExecutedEvent("BTCUSDT", "BUY", "agent1", 50000, 0.1))
	assert.Error(t, err)
}

func TestTelegramChannel_FormatPositionClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "tok",
		ChatID:   "123",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), PositionClosedEvent("ETHUSDT", -50.0, 2*time.Hour, "stop_loss"))
	assert.NoError(t, err)
}

func TestTelegramChannel_FormatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "tok",
		ChatID:   "123",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), ErrorAlertEvent("exchange", "connection timeout"))
	assert.NoError(t, err)
}

func TestTelegramChannel_FormatSafety(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "tok",
		ChatID:   "123",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), SafetyAlertEvent("kill_switch", "trading halted"))
	assert.NoError(t, err)
}

func TestTelegramChannel_FormatDailySummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ch := NewTelegramChannel(TelegramChannelConfig{
		BotToken: "tok",
		ChatID:   "123",
		BaseURL:  srv.URL,
	})

	err := ch.Send(context.Background(), DailySummaryEvent(42, 1500.50, 65.0, "BTCUSDT +500", "ETHUSDT -200"))
	assert.NoError(t, err)
}
