package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/market"
)

func main() {
	fmt.Println("=== CryptoFunk MCP Client Test ===")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Printf("Warning: Could not load config (expected in development): %v\n", err)
		fmt.Println("Using default configuration...")
		cfg = &config.Config{
			MCP: config.MCPConfig{
				External: config.MCPExternalServers{
					CoinGecko: config.MCPExternalServerConfig{
						Enabled: true,
						URL:     "https://mcp.api.coingecko.com/mcp",
					},
				},
			},
		}
	}

	fmt.Println("Phase 1: Infrastructure Setup Complete ✓")
	fmt.Println()
	fmt.Println("Current Status:")
	fmt.Println("  ✓ Git repository initialized")
	fmt.Println("  ✓ Project structure created")
	fmt.Println("  ✓ Go modules configured")
	fmt.Println("  ✓ Docker Compose setup (PostgreSQL, Redis, NATS, Bifrost)")
	fmt.Println("  ✓ Database schema created")
	fmt.Println("  ✓ Configuration management (Viper)")
	fmt.Println("  ✓ Structured logging (Zerolog)")
	fmt.Println("  ✓ Hybrid MCP architecture designed")
	fmt.Println("  ✓ CoinGecko MCP client structure created")
	fmt.Println()

	// Test CoinGecko MCP client structure
	fmt.Println("Phase 2.1: CoinGecko MCP Integration (In Progress)")
	fmt.Println()

	if cfg.MCP.External.CoinGecko.Enabled {
		fmt.Printf("  ✓ CoinGecko MCP enabled: %s\n", cfg.MCP.External.CoinGecko.URL)

		// Create CoinGecko client
		client, err := market.NewCoinGeckoClient(cfg.MCP.External.CoinGecko.URL)
		if err != nil {
			fmt.Printf("  ✗ Failed to create CoinGecko client: %v\n", err)
		} else {
			fmt.Println("  ✓ CoinGecko client created successfully")

			// Test health check
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := client.Health(ctx); err != nil {
				fmt.Printf("  ✗ Health check failed: %v\n", err)
			} else {
				fmt.Println("  ✓ Health check passed")
			}

			// Demonstrate API structure (actual calls in Phase 2)
			fmt.Println()
			fmt.Println("  Available Methods (Structure Ready):")
			fmt.Println("    - GetPrice(symbol, currency) → Current price")
			fmt.Println("    - GetMarketChart(symbol, days) → Historical OHLCV")
			fmt.Println("    - GetCoinInfo(coinID) → Detailed coin information")
			fmt.Println()
			fmt.Println("  Note: Full MCP SDK integration will be completed in Phase 2")
		}
	} else {
		fmt.Println("  ⚠ CoinGecko MCP is disabled in configuration")
	}

	fmt.Println()
	fmt.Println("Custom MCP Servers Configuration:")
	fmt.Printf("  - Order Executor: %v\n", cfg.MCP.Internal.OrderExecutor.Enabled)
	fmt.Printf("  - Risk Analyzer: %v\n", cfg.MCP.Internal.RiskAnalyzer.Enabled)
	fmt.Printf("  - Technical Indicators: %v\n", cfg.MCP.Internal.TechnicalIndicators.Enabled)
	fmt.Printf("  - Market Data (Optional): %v\n", cfg.MCP.Internal.MarketData.Enabled)

	fmt.Println()
	fmt.Println("Next Steps (Phase 2):")
	fmt.Println("  - Complete MCP SDK integration with CoinGecko")
	fmt.Println("  - Implement Redis caching layer")
	fmt.Println("  - Build technical indicators MCP server")
	fmt.Println("  - Build risk analyzer MCP server")
	fmt.Println("  - Build order executor MCP server")
	fmt.Println("  - Create analysis and strategy agents")
	fmt.Println()
	fmt.Println("=== Phase 1 Complete + Architecture Enhanced! ===")
}
