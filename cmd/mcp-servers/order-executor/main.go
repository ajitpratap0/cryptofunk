package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/exchange"
	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
)

const (
	serverName    = "order-executor"
	serverVersion = "1.0.0"

	// MCP Tool Names
	toolPlaceMarketOrder = "place_market_order"
	toolPlaceLimitOrder  = "place_limit_order"
	toolCancelOrder      = "cancel_order"
	toolGetOrderStatus   = "get_order_status"
	toolStartSession     = "start_session"
	toolStopSession      = "stop_session"
	toolGetSessionStats  = "get_session_stats"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Info().Msg("Order Executor MCP Server starting...")

	cfg, err := config.Load("")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	tradingMode := os.Getenv("TRADING_MODE")
	if tradingMode == "" {
		tradingMode = cfg.Trading.Mode
	}

	var binanceAPIKey, binanceSecret string
	var binanceTestnet bool

	if binanceCfg, ok := cfg.Exchanges["binance"]; ok {
		binanceAPIKey = binanceCfg.APIKey
		binanceSecret = binanceCfg.SecretKey
		binanceTestnet = binanceCfg.Testnet
	}
	if envKey := os.Getenv("BINANCE_API_KEY"); envKey != "" {
		binanceAPIKey = envKey
	}
	if envSecret := os.Getenv("BINANCE_API_SECRET"); envSecret != "" {
		binanceSecret = envSecret
	}
	if os.Getenv("BINANCE_TESTNET") == "true" {
		binanceTestnet = true
	}

	logger := log.With().Str("server", serverName).Logger()

	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()

	exchangeConfig := exchange.ServiceConfig{
		Mode:           exchange.TradingMode(tradingMode),
		BinanceAPIKey:  binanceAPIKey,
		BinanceSecret:  binanceSecret,
		BinanceTestnet: binanceTestnet,
	}

	exchangeService, err := exchange.NewService(database, exchangeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create exchange service")
	}

	srv := mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  logger,
	})

	registerTools(srv, exchangeService)

	if err := srv.Run(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func registerTools(srv *mcpserver.Server, service *exchange.Service) {
	srv.AddToolRaw(
		mcpserver.NewTool("place_market_order", "Place a market order (immediate execution at current market price)", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol":   map[string]interface{}{"type": "string", "description": "Trading pair symbol (e.g., 'BTCUSDT')"},
				"side":     map[string]interface{}{"type": "string", "description": "Order side: 'buy' or 'sell'", "enum": []string{"buy", "sell"}},
				"quantity": map[string]interface{}{"type": "number", "description": "Order quantity"},
			},
			"required": []string{"symbol", "side", "quantity"},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.PlaceMarketOrder(ctx, args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("place_limit_order", "Place a limit order (executed at specified price or better)", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol":   map[string]interface{}{"type": "string", "description": "Trading pair symbol (e.g., 'BTCUSDT')"},
				"side":     map[string]interface{}{"type": "string", "description": "Order side: 'buy' or 'sell'", "enum": []string{"buy", "sell"}},
				"quantity": map[string]interface{}{"type": "number", "description": "Order quantity"},
				"price":    map[string]interface{}{"type": "number", "description": "Limit price"},
			},
			"required": []string{"symbol", "side", "quantity", "price"},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.PlaceLimitOrder(ctx, args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("cancel_order", "Cancel an open or pending order", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"order_id": map[string]interface{}{"type": "string", "description": "Order ID to cancel"},
			},
			"required": []string{"order_id"},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.CancelOrder(ctx, args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("get_order_status", "Get current status and details of an order", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"order_id": map[string]interface{}{"type": "string", "description": "Order ID to query"},
			},
			"required": []string{"order_id"},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.GetOrderStatus(ctx, args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("start_session", "Start a new trading session for paper trading", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol":          map[string]interface{}{"type": "string", "description": "Trading pair symbol (e.g., 'BTCUSDT')"},
				"initial_capital": map[string]interface{}{"type": "number", "description": "Initial capital for the trading session"},
				"config":          map[string]interface{}{"type": "object", "description": "Optional configuration parameters for the session"},
			},
			"required": []string{"symbol", "initial_capital"},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.StartSession(ctx, args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("stop_session", "Stop the current trading session and retrieve final statistics", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"final_capital": map[string]interface{}{"type": "number", "description": "Final capital at session end"},
			},
			"required": []string{"final_capital"},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.StopSession(ctx, args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("get_session_stats", "Get current statistics for the active trading session", map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			return service.GetSessionStats(ctx, args)
		}),
	)
}

// Suppress unused import warning for fmt
var _ = fmt.Sprintf
