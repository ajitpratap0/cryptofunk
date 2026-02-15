package polymarket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func TestNewWSClient_Defaults(t *testing.T) {
	ws := NewWSClient()
	assert.Equal(t, DefaultWSURL, ws.url)
	assert.Nil(t, ws.handler)
	assert.NotNil(t, ws.done)
}

func TestNewWSClient_Options(t *testing.T) {
	var called bool
	handler := func(string, json.RawMessage) { called = true }
	logger := zerolog.Nop()

	ws := NewWSClient(
		WithWSURL("wss://custom.example.com/ws"),
		WithWSHandler(handler),
		WithWSLogger(logger),
	)
	assert.Equal(t, "wss://custom.example.com/ws", ws.url)
	assert.NotNil(t, ws.handler)
	// invoke handler to confirm it's set
	ws.handler("test", nil)
	assert.True(t, called)
}

func TestWSClient_ConnectAndReceive(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			close(done)
		}
	})

	msgReceived := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a message
		msg := map[string]interface{}{
			"type": "price_change",
			"data": map[string]string{"asset_id": "tok1", "price": "0.65"},
		}
		conn.WriteJSON(msg)

		// Keep connection alive until test signals done
		<-done
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var mu sync.Mutex
	var received []string

	ws := NewWSClient(
		WithWSURL(wsURL),
		WithWSHandler(func(msgType string, data json.RawMessage) {
			mu.Lock()
			received = append(received, msgType)
			mu.Unlock()
			select {
			case msgReceived <- struct{}{}:
			default:
			}
		}),
		WithWSLogger(zerolog.Nop()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)

	// Wait for message via channel
	select {
	case <-msgReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	mu.Lock()
	assert.Contains(t, received, "price_change")
	mu.Unlock()

	close(done) // unblock server handler first
	err = ws.Close()
	assert.NoError(t, err)
}

func TestWSClient_SubscribeMarket(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			close(done)
		}
	})

	cmdReceived := make(chan wsCommand, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		var cmd wsCommand
		conn.ReadJSON(&cmd)
		cmdReceived <- cmd
		<-done
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws := NewWSClient(WithWSURL(wsURL), WithWSLogger(zerolog.Nop()))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)

	err = ws.SubscribeMarket([]string{"asset1", "asset2"})
	require.NoError(t, err)

	select {
	case receivedCmd := <-cmdReceived:
		assert.Equal(t, "subscribe", receivedCmd.Type)
		assert.Equal(t, "market", receivedCmd.Channel)
		assert.Equal(t, []string{"asset1", "asset2"}, receivedCmd.Assets)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subscribe command")
	}

	close(done)
	ws.Close()
}

func TestWSClient_SubscribeUser(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			close(done)
		}
	})

	cmdReceived := make(chan wsCommand, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var cmd wsCommand
		conn.ReadJSON(&cmd)
		cmdReceived <- cmd
		<-done
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws := NewWSClient(WithWSURL(wsURL), WithWSLogger(zerolog.Nop()))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)

	creds := &APICreds{APIKey: "key", Secret: "secret", Passphrase: "pass"}
	err = ws.SubscribeUser("market1", creds)
	require.NoError(t, err)

	select {
	case receivedCmd := <-cmdReceived:
		assert.Equal(t, "subscribe", receivedCmd.Type)
		assert.Equal(t, "user", receivedCmd.Channel)
		assert.NotNil(t, receivedCmd.Auth)
		assert.Equal(t, "key", receivedCmd.Auth.APIKey)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subscribe command")
	}

	close(done)
	ws.Close()
}

func TestWSClient_SubscribeMarket_NotConnected(t *testing.T) {
	ws := NewWSClient()
	err := ws.SubscribeMarket([]string{"asset1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestWSClient_CloseWithoutConnect(t *testing.T) {
	ws := NewWSClient()
	err := ws.Close()
	assert.NoError(t, err)
}
