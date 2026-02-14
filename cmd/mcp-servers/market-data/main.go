package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/market"
	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
)

const (
	serverName = "market-data"
)

var (
	serverVersion = config.Version
)

// TickerData represents 24h ticker statistics
type TickerData struct {
	Symbol             string `json:"symbol"`
	Price              string `json:"price"`
	PriceChangePercent string `json:"price_change_percent"`
	Volume             string `json:"volume"`
	High24h            string `json:"high_24h"`
	Low24h             string `json:"low_24h"`
	Timestamp          int64  `json:"timestamp"`
}

// MarketDataServer provides market data from multiple sources
type MarketDataServer struct {
	binanceClient   *binance.Client
	coingeckoClient *market.CoinGeckoClient
	logger          zerolog.Logger
	preferCoinGecko bool
}

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	logger := log.With().Str("server", serverName).Logger()
	logger.Info().Msg("Starting Market Data MCP Server")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	var binanceAPIKey, binanceSecretKey string
	if binanceCfg, ok := cfg.Exchanges["binance"]; ok {
		binanceAPIKey = binanceCfg.APIKey
		binanceSecretKey = binanceCfg.SecretKey
	}
	if envKey := os.Getenv("BINANCE_API_KEY"); envKey != "" {
		binanceAPIKey = envKey
	}
	if envSecret := os.Getenv("BINANCE_API_SECRET"); envSecret != "" {
		binanceSecretKey = envSecret
	}

	coingeckoAPIKey := os.Getenv("COINGECKO_API_KEY")

	binanceClient := binance.NewClient(binanceAPIKey, binanceSecretKey)
	binance.UseTestnet = true

	coingeckoClient, err := market.NewCoinGeckoClient(coingeckoAPIKey)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize CoinGecko client, using Binance only")
		coingeckoClient = nil
	}

	preferCoinGecko := coingeckoClient != nil && os.Getenv("PREFER_COINGECKO") != "false"

	service := &MarketDataServer{
		binanceClient:   binanceClient,
		coingeckoClient: coingeckoClient,
		logger:          logger,
		preferCoinGecko: preferCoinGecko,
	}

	srv := mcpserver.New(mcpserver.Config{
		Name:    serverName,
		Version: serverVersion,
		Logger:  logger,
	})

	registerTools(srv, service)

	if err := srv.Run(); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}

func registerTools(srv *mcpserver.Server, service *MarketDataServer) {
	srv.AddToolRaw(
		mcpserver.NewTool("get_price", "Get current price for a cryptocurrency (uses CoinGecko or Binance)", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol":      map[string]interface{}{"type": "string", "description": "Coin ID (e.g., 'bitcoin') for CoinGecko or trading pair (e.g., 'BTCUSDT') for Binance"},
				"vs_currency": map[string]interface{}{"type": "string", "description": "Currency to compare against (default: 'usd')", "default": "usd"},
			},
			"required": []string{"symbol"},
		}),
		mcpserver.WrapLegacyHandler(service.handleGetCurrentPrice),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("get_ticker_24h", "Get 24-hour ticker statistics for a symbol (Binance)", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol": map[string]interface{}{"type": "string", "description": "Trading pair symbol (e.g., BTCUSDT)"},
			},
			"required": []string{"symbol"},
		}),
		mcpserver.WrapLegacyHandler(service.handleGetTicker24h),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("get_order_book", "Get order book depth for a symbol (Binance)", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbol": map[string]interface{}{"type": "string", "description": "Trading pair symbol (e.g., BTCUSDT)"},
				"limit":  map[string]interface{}{"type": "number", "description": "Number of price levels (default: 20)", "default": 20},
			},
			"required": []string{"symbol"},
		}),
		mcpserver.WrapLegacyHandler(service.handleGetOrderbook),
	)

	if service.coingeckoClient != nil {
		srv.AddToolRaw(
			mcpserver.NewTool("get_market_chart", "Get historical market data (prices, market caps, volumes)", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"coin_id": map[string]interface{}{"type": "string", "description": "CoinGecko coin ID (e.g., 'bitcoin', 'ethereum')"},
					"days":    map[string]interface{}{"type": "number", "description": "Number of days of historical data (1, 7, 30, max)", "default": 7},
				},
				"required": []string{"coin_id"},
			}),
			mcpserver.WrapLegacyHandler(service.handleGetMarketChart),
		)

		srv.AddToolRaw(
			mcpserver.NewTool("get_coin_info", "Get detailed information about a cryptocurrency", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"coin_id": map[string]interface{}{"type": "string", "description": "CoinGecko coin ID (e.g., 'bitcoin', 'ethereum')"},
				},
				"required": []string{"coin_id"},
			}),
			mcpserver.WrapLegacyHandler(service.handleGetCoinInfo),
		)
	}
}

func (s *MarketDataServer) handleGetCurrentPrice(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	vsCurrency := "usd"
	if vs, ok := args["vs_currency"].(string); ok {
		vsCurrency = vs
	}

	if s.preferCoinGecko && s.coingeckoClient != nil {
		priceResult, err := s.coingeckoClient.GetPrice(ctx, symbol, vsCurrency)
		if err == nil {
			return map[string]interface{}{
				"symbol":    priceResult.Symbol,
				"price":     fmt.Sprintf("%.2f", priceResult.Price),
				"currency":  priceResult.Currency,
				"timestamp": time.Now().Unix(),
				"source":    "coingecko",
			}, nil
		}
		s.logger.Warn().Err(err).Str("symbol", symbol).Msg("CoinGecko failed, falling back to Binance")
	}

	prices, err := s.binanceClient.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get price: %w", err)
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data for symbol: %s", symbol)
	}

	return map[string]interface{}{
		"symbol":    prices[0].Symbol,
		"price":     prices[0].Price,
		"timestamp": time.Now().Unix(),
		"source":    "binance",
	}, nil
}

func (s *MarketDataServer) handleGetTicker24h(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	ticker, err := s.binanceClient.NewListPriceChangeStatsService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %w", err)
	}
	if len(ticker) == 0 {
		return nil, fmt.Errorf("no ticker data for symbol: %s", symbol)
	}

	t := ticker[0]
	return TickerData{
		Symbol:             t.Symbol,
		Price:              t.LastPrice,
		PriceChangePercent: t.PriceChangePercent,
		Volume:             t.Volume,
		High24h:            t.HighPrice,
		Low24h:             t.LowPrice,
		Timestamp:          time.Now().Unix(),
	}, nil
}

func (s *MarketDataServer) handleGetOrderbook(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol must be a string")
	}

	limit := 20
	if l, ok := args["limit"]; ok {
		if lNum, ok := l.(float64); ok {
			limit = int(lNum)
		} else if lStr, ok := l.(string); ok {
			if parsed, err := strconv.Atoi(lStr); err == nil {
				limit = parsed
			}
		}
	}

	orderbook, err := s.binanceClient.NewDepthService().Symbol(symbol).Limit(limit).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get orderbook: %w", err)
	}

	return map[string]interface{}{
		"symbol":    symbol,
		"bids":      orderbook.Bids,
		"asks":      orderbook.Asks,
		"timestamp": time.Now().Unix(),
	}, nil
}

func (s *MarketDataServer) handleGetMarketChart(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.coingeckoClient == nil {
		return nil, fmt.Errorf("CoinGecko client not available")
	}

	coinID, ok := args["coin_id"].(string)
	if !ok {
		return nil, fmt.Errorf("coin_id must be a string")
	}

	days := 7
	if d, ok := args["days"]; ok {
		if dNum, ok := d.(float64); ok {
			days = int(dNum)
		} else if dStr, ok := d.(string); ok {
			if parsed, err := strconv.Atoi(dStr); err == nil {
				days = parsed
			}
		}
	}

	chart, err := s.coingeckoClient.GetMarketChart(ctx, coinID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get market chart: %w", err)
	}

	return map[string]interface{}{
		"coin_id":           coinID,
		"days":              days,
		"price_points":      len(chart.Prices),
		"market_cap_points": len(chart.MarketCaps),
		"volume_points":     len(chart.TotalVolumes),
		"prices":            chart.Prices,
		"market_caps":       chart.MarketCaps,
		"total_volumes":     chart.TotalVolumes,
		"timestamp":         time.Now().Unix(),
	}, nil
}

func (s *MarketDataServer) handleGetCoinInfo(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.coingeckoClient == nil {
		return nil, fmt.Errorf("CoinGecko client not available")
	}

	coinID, ok := args["coin_id"].(string)
	if !ok {
		return nil, fmt.Errorf("coin_id must be a string")
	}

	info, err := s.coingeckoClient.GetCoinInfo(ctx, coinID)
	if err != nil {
		return nil, fmt.Errorf("failed to get coin info: %w", err)
	}

	return map[string]interface{}{
		"id":          info.ID,
		"symbol":      info.Symbol,
		"name":        info.Name,
		"description": info.Description,
		"links":       info.Links,
		"market_data": info.MarketData,
		"timestamp":   time.Now().Unix(),
	}, nil
}
