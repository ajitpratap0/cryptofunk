package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	DefaultWSURL   = "wss://ws-subscriptions-clob.polymarket.com/ws"
	pingInterval   = 30 * time.Second
	reconnectDelay = 5 * time.Second
	maxReconnects  = 10
)

// WSHandler handles incoming WebSocket messages
type WSHandler func(msgType string, data json.RawMessage)

// WSClient manages a WebSocket connection to Polymarket
type WSClient struct {
	url     string
	conn    *websocket.Conn
	handler WSHandler
	logger  zerolog.Logger

	subscriptions []wsSubscription
	mu            sync.Mutex
	writeMu       sync.Mutex
	done          chan struct{}
	reconnects    int
}

type wsSubscription struct {
	Channel string   `json:"channel"`
	Assets  []string `json:"assets,omitempty"`
	Market  string   `json:"market,omitempty"`
}

type wsCommand struct {
	Type    string   `json:"type"`
	Channel string   `json:"channel,omitempty"`
	Assets  []string `json:"assets_ids,omitempty"`
	Market  string   `json:"market,omitempty"`
	Auth    *wsAuth  `json:"auth,omitempty"`
}

type wsAuth struct {
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// NewWSClient creates a new WebSocket client
func NewWSClient(opts ...WSClientOption) *WSClient {
	ws := &WSClient{
		url:    DefaultWSURL,
		done:   make(chan struct{}),
		logger: zerolog.Nop(),
	}
	for _, opt := range opts {
		opt(ws)
	}
	return ws
}

// WSClientOption configures the WS client
type WSClientOption func(*WSClient)

// WithWSURL sets a custom WS URL
func WithWSURL(url string) WSClientOption {
	return func(ws *WSClient) { ws.url = url }
}

// WithWSLogger sets the logger
func WithWSLogger(logger zerolog.Logger) WSClientOption {
	return func(ws *WSClient) { ws.logger = logger }
}

// WithWSHandler sets the message handler
func WithWSHandler(h WSHandler) WSClientOption {
	return func(ws *WSClient) { ws.handler = h }
}

// Connect establishes the WebSocket connection
func (ws *WSClient) Connect(ctx context.Context) error {
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, ws.url, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	ws.mu.Lock()
	ws.conn = conn
	ws.mu.Unlock()
	ws.reconnects = 0
	ws.mu.Unlock()

	// Re-subscribe existing channels
	ws.mu.Lock()
	subs := make([]wsSubscription, len(ws.subscriptions))
	copy(subs, ws.subscriptions)
	ws.mu.Unlock()

	for _, sub := range subs {
		if err := ws.sendSubscribe(sub); err != nil {
			ws.logger.Error().Err(err).Str("channel", sub.Channel).Msg("resubscribe failed")
		}
	}

	go ws.readLoop(ctx)
	go ws.pingLoop(ctx)

	return nil
}

// SubscribeMarket subscribes to orderbook updates for assets
func (ws *WSClient) SubscribeMarket(assets []string) error {
	sub := wsSubscription{Channel: "market", Assets: assets}
	ws.mu.Lock()
	ws.subscriptions = append(ws.subscriptions, sub)
	ws.mu.Unlock()
	return ws.sendSubscribe(sub)
}

// SubscribeUser subscribes to user order/trade updates (requires auth)
func (ws *WSClient) SubscribeUser(market string, creds *APICreds) error {
	sub := wsSubscription{Channel: "user", Market: market}
	ws.mu.Lock()
	ws.subscriptions = append(ws.subscriptions, sub)
	ws.mu.Unlock()

	cmd := wsCommand{
		Type:    "subscribe",
		Channel: "user",
		Market:  market,
		Auth: &wsAuth{
			APIKey:     creds.APIKey,
			Secret:     creds.Secret,
			Passphrase: creds.Passphrase,
		},
	}
	ws.writeMu.Lock()
	err := ws.conn.WriteJSON(cmd)
	ws.writeMu.Unlock()
	return err
}

func (ws *WSClient) sendSubscribe(sub wsSubscription) error {
	if ws.conn == nil {
		return fmt.Errorf("not connected")
	}
	cmd := wsCommand{
		Type:    "subscribe",
		Channel: sub.Channel,
		Assets:  sub.Assets,
		Market:  sub.Market,
	}
	ws.writeMu.Lock()
	err := ws.conn.WriteJSON(cmd)
	ws.writeMu.Unlock()
	return err
}

// Close closes the WebSocket connection
func (ws *WSClient) Close() error {
	close(ws.done)
	if ws.conn != nil {
		return ws.conn.Close()
	}
	return nil
}

func (ws *WSClient) readLoop(ctx context.Context) {
	for {
		select {
		case <-ws.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			ws.logger.Error().Err(err).Msg("ws read error")
			ws.tryReconnect(ctx)
			return
		}

		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			ws.logger.Error().Err(err).Msg("ws unmarshal error")
			continue
		}

		if ws.handler != nil {
			ws.handler(msg.Type, msg.Data)
		}
	}
}

func (ws *WSClient) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ws.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ws.conn != nil {
				ws.writeMu.Lock()
				err := ws.conn.WriteMessage(websocket.PingMessage, nil)
				ws.writeMu.Unlock()
				if err != nil {
					ws.logger.Error().Err(err).Msg("ws ping failed")
				}
			}
		}
	}
}

func (ws *WSClient) tryReconnect(ctx context.Context) {
	for attempt := 1; attempt <= maxReconnects; attempt++ {
		select {
		case <-ws.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		shift := attempt - 1
		if shift < 0 {
			shift = 0
		}
		backoff := reconnectDelay * time.Duration(1<<uint(shift)) //nolint:gosec // G115 - shift is bounded [0, maxReconnects]
		if backoff > 2*time.Minute {
			backoff = 2 * time.Minute
		}

		ws.logger.Info().Int("attempt", attempt).Dur("backoff", backoff).Msg("reconnecting ws")
		time.Sleep(backoff)

		ws.reconnects = attempt
		if err := ws.Connect(ctx); err != nil {
			ws.logger.Error().Err(err).Int("attempt", attempt).Msg("reconnect failed")
			continue
		}
		return // success
	}
	ws.logger.Error().Msg("max reconnects reached, giving up")
	ws.Close()
}
