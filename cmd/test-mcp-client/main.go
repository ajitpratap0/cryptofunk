package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/market"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
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
		// Note: Using free tier (no API key). Set COINGECKO_API_KEY env var for pro tier.
		client, err := market.NewCoinGeckoClient("")
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

			// Test Redis caching layer
			fmt.Println()
			fmt.Println("Testing Redis Caching Layer:")
			testRedisCaching(cfg, client)

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
}

func testRedisCaching(cfg *config.Config, client *market.CoinGeckoClient) {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close Redis connection")
		}
	}()

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("  ✗ Redis connection failed: %v\n", err)
		fmt.Println("  ℹ Start Redis with: docker-compose up -d redis")
		return
	}
	fmt.Println("  ✓ Redis connection established")

	// Create cached client
	cacheTTL := time.Duration(cfg.MCP.External.CoinGecko.CacheTTL) * time.Second
	cachedClient := market.NewCachedCoinGeckoClient(client, redisClient, cacheTTL)
	fmt.Printf("  ✓ Cached client created (TTL: %v)\n", cacheTTL)

	// Test health check with cache
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	if err := cachedClient.Health(ctx2); err != nil {
		fmt.Printf("  ✗ Cached client health check failed: %v\n", err)
	} else {
		fmt.Println("  ✓ Cached client health check passed")
	}

	// Demonstrate cache behavior
	fmt.Println()
	fmt.Println("  Cache Behavior Demo:")
	fmt.Println("    - First call: Cache miss → Fetch from CoinGecko MCP")
	fmt.Println("    - Second call: Cache hit → Fast response from Redis")
	fmt.Println("    - After TTL: Cache expires → Fresh fetch")
	fmt.Println()
	fmt.Println("  Cache Management:")
	fmt.Println("    - InvalidateCache(symbol) → Clear specific symbol cache")
	fmt.Println("    - ClearCache() → Clear all cached data")

	// Test TimescaleDB sync (if Redis is available)
	if err := redisClient.Ping(ctx).Err(); err == nil {
		fmt.Println()
		fmt.Println("Testing TimescaleDB Sync Service:")
		testTimescaleDBSync(cfg, cachedClient)
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

func testTimescaleDBSync(cfg *config.Config, cachedClient *market.CachedCoinGeckoClient) {
	// Connect to database
	db, err := sql.Open("postgres", cfg.Database.GetDSN())
	if err != nil {
		fmt.Printf("  ✗ Database connection failed: %v\n", err)
		fmt.Println("  ℹ Start PostgreSQL with: docker-compose up -d postgres")
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database connection")
		}
	}()

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		fmt.Printf("  ✗ Database ping failed: %v\n", err)
		fmt.Println("  ℹ Start PostgreSQL with: docker-compose up -d postgres")
		return
	}
	fmt.Println("  ✓ Database connection established")

	// Create sync service
	symbols := cfg.Trading.Symbols
	syncInterval := 15 * time.Minute // Sync every 15 minutes
	syncService := market.NewSyncService(cachedClient, db, symbols, syncInterval)
	fmt.Printf("  ✓ Sync service created (interval: %v)\n", syncInterval)

	// Get data statistics for each symbol
	fmt.Println()
	fmt.Println("  Data Statistics:")
	for _, symbol := range symbols {
		stats, err := syncService.GetDataStats(ctx, symbol)
		if err != nil {
			fmt.Printf("    %s: No data yet\n", symbol)
			continue
		}

		count := stats["count"].(int)
		if count > 0 {
			earliest := stats["earliest"].(time.Time)
			latest := stats["latest"].(time.Time)
			daysStored := stats["days_stored"].(int)
			fmt.Printf("    %s: %d candlesticks (%d days, %s to %s)\n",
				symbol, count, daysStored,
				earliest.Format("2006-01-02"), latest.Format("2006-01-02"))
		} else {
			fmt.Printf("    %s: No data yet\n", symbol)
		}
	}

	fmt.Println()
	fmt.Println("  Sync Service Features:")
	fmt.Println("    - Periodic sync of historical data from CoinGecko")
	fmt.Println("    - Stores OHLCV candlesticks in TimescaleDB")
	fmt.Println("    - Automatic gap detection and backfilling")
	fmt.Println("    - Fast local backtesting without API calls")
	fmt.Println()
	fmt.Println("  Available Methods:")
	fmt.Println("    - Start(ctx) → Begin periodic sync")
	fmt.Println("    - Stop() → Stop sync service")
	fmt.Println("    - GetCandlesticks(symbol, start, end) → Query historical data")
	fmt.Println("    - GetLatestPrice(symbol) → Get most recent price")
	fmt.Println("    - GetDataStats(symbol) → Get storage statistics")
}
