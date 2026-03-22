package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket"
)

const serverName = "polymarket"

var serverVersion = config.Version

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	logger := log.With().Str("server", serverName).Logger()
	logger.Info().Msg("Starting Polymarket MCP Server")

	privateKey := os.Getenv("POLYMARKET_PRIVATE_KEY")
	funder := os.Getenv("POLYMARKET_FUNDER_ADDRESS")

	opts := []polymarket.ClientOption{
		polymarket.WithLogger(logger),
	}
	if funder != "" {
		opts = append(opts, polymarket.WithFunder(funder))
	}

	client, err := polymarket.NewClient(privateKey, opts...)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Polymarket client")
	}
	defer client.Close()

	// Auto-derive API creds if private key is set
	if privateKey != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		creds, err := client.CreateOrDeriveAPICreds(ctx)
		cancel()
		if err != nil {
			logger.Warn().Err(err).Msg("Could not derive API creds, L2 endpoints unavailable")
		} else {
			client.SetAPICreds(creds)
			logger.Info().Msg("API credentials derived successfully")
		}
	}

	srv := mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  logger,
	})
	registerTools(srv, client)

	if err := srv.Run(); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}

// registerTools registers all Polymarket tools with the shared MCP server base.
// This enables HTTP transport support via the standard Streamable HTTP handler.
//
// Validation errors (missing required params) return fmt.Errorf so that
// WrapLegacyHandler converts them to IsError=true tool-level errors, which is
// the correct MCP behavior.
func registerTools(srv *mcpserver.Server, client *polymarket.Client) {
	// get_markets
	srv.AddToolRaw(
		mcpserver.NewTool("get_markets", "Get active prediction markets from Polymarket",
			map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return client.GetMarkets(ctx)
		}),
	)

	// get_market
	srv.AddToolRaw(
		mcpserver.NewTool("get_market", "Get details for a specific Polymarket market by condition ID",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"condition_id": map[string]interface{}{"type": "string", "description": "Market condition ID"},
				},
				"required": []string{"condition_id"},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			conditionID, _ := args["condition_id"].(string)
			if conditionID == "" {
				return nil, fmt.Errorf("condition_id is required")
			}
			return client.GetMarket(ctx, conditionID)
		}),
	)

	// get_orderbook
	srv.AddToolRaw(
		mcpserver.NewTool("get_orderbook", "Get the order book for a Polymarket token",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token_id": map[string]interface{}{"type": "string", "description": "Conditional token ID"},
				},
				"required": []string{"token_id"},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			tokenID, _ := args["token_id"].(string)
			if tokenID == "" {
				return nil, fmt.Errorf("token_id is required")
			}
			return client.GetOrderBook(ctx, tokenID)
		}),
	)

	// get_price
	srv.AddToolRaw(
		mcpserver.NewTool("get_price", "Get best bid/ask price for a Polymarket token",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token_id": map[string]interface{}{"type": "string", "description": "Conditional token ID"},
					"side":     map[string]interface{}{"type": "string", "description": "BUY or SELL", "enum": []string{"BUY", "SELL"}},
				},
				"required": []string{"token_id", "side"},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			tokenID, _ := args["token_id"].(string)
			sideStr, _ := args["side"].(string)
			if tokenID == "" || sideStr == "" {
				return nil, fmt.Errorf("token_id and side are required")
			}
			price, err := client.GetPrice(ctx, tokenID, polymarket.Side(sideStr))
			if err != nil {
				return nil, err
			}
			return map[string]string{"price": price}, nil
		}),
	)

	// place_order
	srv.AddToolRaw(
		mcpserver.NewTool("place_order", "Place a limit order on Polymarket",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token_id": map[string]interface{}{"type": "string", "description": "Conditional token ID"},
					"side":     map[string]interface{}{"type": "string", "description": "BUY or SELL", "enum": []string{"BUY", "SELL"}},
					"price":    map[string]interface{}{"type": "number", "description": "Limit price (exclusive 0-1 range)"},
					"size":     map[string]interface{}{"type": "number", "description": "Order size in tokens (must be > 0)"},
				},
				"required": []string{"token_id", "side", "price", "size"},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			tokenID, _ := args["token_id"].(string)
			if tokenID == "" {
				return nil, fmt.Errorf("token_id is required")
			}
			sideStr, _ := args["side"].(string)
			if sideStr != "BUY" && sideStr != "SELL" {
				return nil, fmt.Errorf("side must be BUY or SELL, got %q", sideStr)
			}
			priceVal, ok := args["price"].(float64)
			if !ok {
				return nil, fmt.Errorf("price must be a number")
			}
			if priceVal <= 0 || priceVal >= 1 {
				return nil, fmt.Errorf("price must be between 0 and 1 exclusive, got %v", priceVal)
			}
			sizeVal, ok := args["size"].(float64)
			if !ok {
				return nil, fmt.Errorf("size must be a number")
			}
			if sizeVal <= 0 {
				return nil, fmt.Errorf("size must be greater than 0, got %v", sizeVal)
			}
			// m5: upper bound sanity check — Polymarket tokens are binary
			// contracts with values between 0 and 1. A position size above
			// 1,000,000 tokens is almost certainly a data-entry error and
			// could represent significant unintended financial exposure.
			const maxOrderSize = 1_000_000
			if sizeVal > maxOrderSize {
				return nil, fmt.Errorf("size %v exceeds maximum allowed order size of %v tokens; for intentionally large orders contact Polymarket directly", sizeVal, maxOrderSize)
			}
			return client.CreateOrder(ctx, tokenID, polymarket.Side(sideStr), priceVal, sizeVal)
		}),
	)

	// cancel_order
	srv.AddToolRaw(
		mcpserver.NewTool("cancel_order", "Cancel an open order on Polymarket",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"order_id": map[string]interface{}{"type": "string", "description": "Order ID to cancel"},
				},
				"required": []string{"order_id"},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			orderID, _ := args["order_id"].(string)
			if orderID == "" {
				return nil, fmt.Errorf("order_id is required")
			}
			if err := client.CancelOrder(ctx, orderID); err != nil {
				return nil, err
			}
			return map[string]string{"status": "cancelled", "order_id": orderID}, nil
		}),
	)

	// get_positions
	srv.AddToolRaw(
		mcpserver.NewTool("get_positions", "Get open orders and trade history (positions) on Polymarket",
			map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
		mcpserver.WrapLegacyHandler(func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			orders, err := client.GetOpenOrders(ctx)
			if err != nil {
				return nil, fmt.Errorf("get open orders: %w", err)
			}
			trades, err := client.GetTrades(ctx)
			if err != nil {
				return nil, fmt.Errorf("get trades: %w", err)
			}
			return map[string]interface{}{
				"open_orders": orders,
				"trades":      trades,
			}, nil
		}),
	)
}
