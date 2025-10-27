package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	serverName    = "market-data"
	serverVersion = "0.1.0"
)

// MarketDataServer implements MCP server for market data
type MarketDataServer struct {
	binanceClient *binance.Client
	logger        zerolog.Logger
}

// TickerData represents ticker information
type TickerData struct {
	Symbol             string `json:"symbol"`
	Price              string `json:"price"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	High24h            string `json:"high24h"`
	Low24h             string `json:"low24h"`
	Timestamp          int64  `json:"timestamp"`
}

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	logger := log.With().Str("server", serverName).Logger()
	logger.Info().Msg("Starting Market Data MCP Server")

	// Get API keys from environment
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	// Initialize Binance client (testnet)
	binanceClient := binance.NewClient(apiKey, secretKey)
	binance.UseTestnet = true // Use testnet for development

	// Create market data server
	_ = &MarketDataServer{
		binanceClient: binanceClient,
		logger:        logger,
	}

	// Create MCP server
	// Note: Full MCP integration will be implemented in Phase 2
	// For Phase 1, we're setting up the basic structure
	logger.Info().Msg("MCP Server structure ready")
	logger.Info().Msg("Note: Full MCP integration to be completed in Phase 2")

	// TODO: Implement full MCP server with mcp.NewServer and proper handlers
	// This placeholder ensures the project compiles and infrastructure is ready

	logger.Info().Msg("Market Data Server initialized successfully")
	logger.Info().Msg("Phase 1 complete - Infrastructure and basic structure ready")

	// Keep the server running for demonstration
	select {}
}

// registerTools registers MCP tools
// TODO: Update to use mcp.Server API in Phase 2
func (s *MarketDataServer) registerTools(srv interface{}) error {
	// Tool: get_current_price
	// TODO: Implement with mcp.Server.Handle() in Phase 2
	_ = srv
	/*
		var err error
		err := srv.AddTool(
			"get_current_price",
			"Get current price for a trading symbol",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Trading pair symbol (e.g., BTCUSDT, ETHUSDT)",
					},
				},
				"required": []string{"symbol"},
			},
			s.handleGetCurrentPrice,
		)
		if err != nil {
			return fmt.Errorf("failed to add get_current_price tool: %w", err)
		}

		// Tool: get_ticker_24h
		err = srv.AddTool(
			"get_ticker_24h",
			"Get 24-hour ticker statistics for a symbol",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Trading pair symbol (e.g., BTCUSDT)",
					},
				},
				"required": []string{"symbol"},
			},
			s.handleGetTicker24h,
		)
		if err != nil {
			return fmt.Errorf("failed to add get_ticker_24h tool: %w", err)
		}

		// Tool: get_orderbook
		err = srv.AddTool(
			"get_orderbook",
			"Get order book depth for a symbol",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "Trading pair symbol (e.g., BTCUSDT)",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Depth limit (5, 10, 20, 50, 100, 500, 1000)",
						"default":     20,
					},
				},
				"required": []string{"symbol"},
			},
			s.handleGetOrderbook,
		)
		if err != nil {
			return fmt.Errorf("failed to add get_orderbook tool: %w", err)
		}
	*/

	s.logger.Info().Msg("Tools registration structure ready (full implementation in Phase 2)")
	return nil
}

// registerResources registers MCP resources
// TODO: Update to use mcp.Server API in Phase 2
func (s *MarketDataServer) registerResources(srv interface{}) error {
	// Resource: market://ticker/{symbol}
	// TODO: Implement with mcp.Server in Phase 2
	_ = srv
	/*
		var err error
		err := srv.AddResourceTemplate(
			"market://ticker/{symbol}",
			"Real-time ticker data for a trading symbol",
			"application/json",
			s.handleTickerResource,
		)
		if err != nil {
			return fmt.Errorf("failed to add ticker resource: %w", err)
		}
	*/

	s.logger.Info().Msg("Resources registration structure ready (full implementation in Phase 2)")
	return nil
}

// Tool handlers

func (s *MarketDataServer) handleGetCurrentPrice(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	s.logger.Debug().Str("symbol", symbol).Msg("Getting current price")

	// Get price from Binance
	prices, err := s.binanceClient.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get price")
		return nil, fmt.Errorf("failed to get price: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data for symbol: %s", symbol)
	}

	result := map[string]interface{}{
		"symbol":    prices[0].Symbol,
		"price":     prices[0].Price,
		"timestamp": time.Now().Unix(),
	}

	s.logger.Info().
		Str("symbol", symbol).
		Str("price", prices[0].Price).
		Msg("Price retrieved")

	return result, nil
}

func (s *MarketDataServer) handleGetTicker24h(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	s.logger.Debug().Str("symbol", symbol).Msg("Getting 24h ticker")

	// Get 24h ticker from Binance
	ticker, err := s.binanceClient.NewListPriceChangeStatsService().Symbol(symbol).Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get ticker")
		return nil, fmt.Errorf("failed to get ticker: %w", err)
	}

	if len(ticker) == 0 {
		return nil, fmt.Errorf("no ticker data for symbol: %s", symbol)
	}

	t := ticker[0]
	result := TickerData{
		Symbol:             t.Symbol,
		Price:              t.LastPrice,
		PriceChangePercent: t.PriceChangePercent,
		Volume:             t.Volume,
		High24h:            t.HighPrice,
		Low24h:             t.LowPrice,
		Timestamp:          time.Now().Unix(),
	}

	s.logger.Info().
		Str("symbol", symbol).
		Str("price", t.LastPrice).
		Str("change_percent", t.PriceChangePercent).
		Msg("Ticker retrieved")

	return result, nil
}

func (s *MarketDataServer) handleGetOrderbook(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	limit := 20 // default
	if l, ok := args["limit"]; ok {
		if lNum, ok := l.(float64); ok {
			limit = int(lNum)
		} else if lStr, ok := l.(string); ok {
			if parsed, err := strconv.Atoi(lStr); err == nil {
				limit = parsed
			}
		}
	}

	s.logger.Debug().
		Str("symbol", symbol).
		Int("limit", limit).
		Msg("Getting orderbook")

	// Get orderbook from Binance
	orderbook, err := s.binanceClient.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to get orderbook")
		return nil, fmt.Errorf("failed to get orderbook: %w", err)
	}

	result := map[string]interface{}{
		"symbol":    symbol,
		"bids":      orderbook.Bids,
		"asks":      orderbook.Asks,
		"timestamp": time.Now().Unix(),
	}

	s.logger.Info().
		Str("symbol", symbol).
		Int("bids", len(orderbook.Bids)).
		Int("asks", len(orderbook.Asks)).
		Msg("Orderbook retrieved")

	return result, nil
}

// Resource handlers

func (s *MarketDataServer) handleTickerResource(ctx context.Context, uri string, params map[string]string) (string, string, error) {
	symbol, ok := params["symbol"]
	if !ok {
		return "", "", fmt.Errorf("symbol parameter required")
	}

	s.logger.Debug().Str("symbol", symbol).Msg("Handling ticker resource")

	// Get ticker data (reuse tool handler logic)
	result, err := s.handleGetTicker24h(ctx, map[string]interface{}{"symbol": symbol})
	if err != nil {
		return "", "", err
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal ticker data: %w", err)
	}

	return string(data), "application/json", nil
}
