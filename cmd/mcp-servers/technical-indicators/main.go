package main

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/indicators"
	mcpserver "github.com/ajitpratap0/cryptofunk/internal/mcp"
)

const (
	serverName = "technical-indicators"
)

var serverVersion = config.Version

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Info().Msg("Technical Indicators MCP Server starting...")

	indicatorService := indicators.NewService()
	logger := log.With().Str("server", serverName).Logger()

	srv := mcpserver.New(mcpserver.Config{
		Name:        serverName,
		Version:     serverVersion,
		Logger:      logger,
		MetricsPort: 9113,
	})

	registerTools(srv, indicatorService)

	if err := srv.Run(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func registerTools(srv *mcpserver.Server, service *indicators.Service) {
	srv.AddToolRaw(
		mcpserver.NewTool("calculate_rsi", "Calculate Relative Strength Index (RSI) for trend strength analysis", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prices": map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of closing prices"},
				"period": map[string]interface{}{"type": "number", "description": "RSI period (default: 14)", "default": 14},
			},
			"required": []string{"prices"},
		}),
		mcpserver.WrapLegacyHandler(func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return service.CalculateRSI(args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_macd", "Calculate Moving Average Convergence Divergence (MACD) for trend analysis", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prices":        map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of closing prices"},
				"fast_period":   map[string]interface{}{"type": "number", "description": "Fast EMA period (default: 12)", "default": 12},
				"slow_period":   map[string]interface{}{"type": "number", "description": "Slow EMA period (default: 26)", "default": 26},
				"signal_period": map[string]interface{}{"type": "number", "description": "Signal line period (default: 9)", "default": 9},
			},
			"required": []string{"prices"},
		}),
		mcpserver.WrapLegacyHandler(func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return service.CalculateMACD(args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_bollinger_bands", "Calculate Bollinger Bands for volatility analysis", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prices":  map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of closing prices"},
				"period":  map[string]interface{}{"type": "number", "description": "Period for moving average (default: 20)", "default": 20},
				"std_dev": map[string]interface{}{"type": "number", "description": "Standard deviations (default: 2)", "default": 2},
			},
			"required": []string{"prices"},
		}),
		mcpserver.WrapLegacyHandler(func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return service.CalculateBollingerBands(args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_ema", "Calculate Exponential Moving Average (EMA)", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prices": map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of closing prices"},
				"period": map[string]interface{}{"type": "number", "description": "EMA period"},
			},
			"required": []string{"prices", "period"},
		}),
		mcpserver.WrapLegacyHandler(func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return service.CalculateEMA(args)
		}),
	)

	srv.AddToolRaw(
		mcpserver.NewTool("calculate_adx", "Calculate Average Directional Index (ADX) for trend strength", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"high":   map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of high prices"},
				"low":    map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of low prices"},
				"close":  map[string]interface{}{"type": "array", "items": map[string]string{"type": "number"}, "description": "Array of closing prices"},
				"period": map[string]interface{}{"type": "number", "description": "ADX period (default: 14)", "default": 14},
			},
			"required": []string{"high", "low", "close"},
		}),
		mcpserver.WrapLegacyHandler(func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return service.CalculateADX(args)
		}),
	)
}
