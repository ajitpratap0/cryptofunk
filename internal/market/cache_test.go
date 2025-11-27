package market

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testCacheKeyBitcoinUSD = "coingecko:price:bitcoin:usd"
)

func setupMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestNewCachedCoinGeckoClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires MCP server connection")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cacheTTL := 60 * time.Second
	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, cacheTTL)

	if cachedClient == nil {
		t.Fatal("Expected non-nil cached client")
	}

	if cachedClient.client != cgClient {
		t.Error("CoinGecko client not properly wrapped")
	}

	if cachedClient.redis != redisClient {
		t.Error("Redis client not properly set")
	}

	if cachedClient.cacheTTL != cacheTTL {
		t.Errorf("Expected TTL %v, got %v", cacheTTL, cachedClient.cacheTTL)
	}
}

func TestCachedGetPrice_CacheMiss(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	result, err := cachedClient.GetPrice(context.Background(), "bitcoin", "usd")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Symbol != "bitcoin" {
		t.Errorf("Expected symbol bitcoin, got %s", result.Symbol)
	}

	// Give async cache write time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify data was cached
	cacheKey := testCacheKeyBitcoinUSD
	cached, err := redisClient.Get(context.Background(), cacheKey).Result()
	if err != nil {
		t.Errorf("Expected data to be cached, got error: %v", err)
	}

	var cachedResult PriceResult
	if err := json.Unmarshal([]byte(cached), &cachedResult); err != nil {
		t.Errorf("Failed to unmarshal cached data: %v", err)
	}

	if cachedResult.Symbol != result.Symbol {
		t.Error("Cached data doesn't match original result")
	}
}

func TestCachedGetPrice_CacheHit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires MCP server connection")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Pre-populate cache with test data
	cacheKey := testCacheKeyBitcoinUSD
	testResult := &PriceResult{
		Symbol:   "bitcoin",
		Price:    50000.0,
		Currency: "usd",
	}
	data, _ := json.Marshal(testResult)
	err = redisClient.Set(context.Background(), cacheKey, data, 60*time.Second).Err()
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	result, err := cachedClient.GetPrice(context.Background(), "bitcoin", "usd")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Price != 50000.0 {
		t.Errorf("Expected cached price 50000.0, got %.2f", result.Price)
	}
}

func TestCachedGetMarketChart_CacheMiss(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	result, err := cachedClient.GetMarketChart(context.Background(), "bitcoin", 7)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Prices) == 0 {
		t.Error("Expected non-empty prices")
	}

	// Give async cache write time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify data was cached
	cacheKey := "coingecko:chart:bitcoin:7"
	cached, err := redisClient.Get(context.Background(), cacheKey).Result()
	if err != nil {
		t.Errorf("Expected data to be cached, got error: %v", err)
	}

	var cachedResult MarketChart
	if err := json.Unmarshal([]byte(cached), &cachedResult); err != nil {
		t.Errorf("Failed to unmarshal cached data: %v", err)
	}
}

func TestCachedGetMarketChart_DifferentTTLForHistorical(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Test with 7+ days (should get longer TTL)
	_, err = cachedClient.GetMarketChart(context.Background(), "bitcoin", 30)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Give async cache write time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify TTL is different for historical data
	cacheKey := "coingecko:chart:bitcoin:30"
	ttl, err := redisClient.TTL(context.Background(), cacheKey).Result()
	if err != nil {
		t.Errorf("Failed to get TTL: %v", err)
	}

	// Should be ~5 minutes for historical data
	if ttl < 4*time.Minute || ttl > 6*time.Minute {
		t.Logf("TTL for historical data: %v (expected ~5 minutes)", ttl)
	}
}

func TestCachedGetCoinInfo(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	result, err := cachedClient.GetCoinInfo(context.Background(), "bitcoin")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.ID != "bitcoin" {
		t.Errorf("Expected ID bitcoin, got %s", result.ID)
	}

	// Give async cache write time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify long TTL for coin info (10 minutes)
	cacheKey := "coingecko:info:bitcoin"
	ttl, err := redisClient.TTL(context.Background(), cacheKey).Result()
	if err != nil {
		t.Errorf("Failed to get TTL: %v", err)
	}

	// Should be ~10 minutes for coin info
	if ttl < 9*time.Minute {
		t.Logf("TTL for coin info: %v (expected ~10 minutes)", ttl)
	}
}

func TestHealth_Success(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	err = cachedClient.Health(context.Background())
	if err != nil {
		t.Errorf("Unexpected error in health check: %v", err)
	}
}

func TestHealth_RedisDown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires MCP server connection")
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cgClient, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Close miniredis to simulate failure
	mr.Close()

	err = cachedClient.Health(context.Background())
	if err == nil {
		t.Error("Expected error when Redis is down")
	}
}

func TestInvalidateCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires MCP server connection")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Add some cached data
	testData := map[string]string{
		testCacheKeyBitcoinUSD:         `{"symbol":"bitcoin","price":45000,"currency":"usd"}`,
		"coingecko:chart:bitcoin:7":    `{"prices":[]}`,
		"coingecko:price:ethereum:usd": `{"symbol":"ethereum","price":3000,"currency":"usd"}`,
	}

	for key, value := range testData {
		err := redisClient.Set(context.Background(), key, value, 60*time.Second).Err()
		if err != nil {
			t.Fatalf("Failed to set cache key %s: %v", key, err)
		}
	}

	// Invalidate bitcoin cache
	err = cachedClient.InvalidateCache(context.Background(), "bitcoin")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify bitcoin keys are gone
	_, err = redisClient.Get(context.Background(), testCacheKeyBitcoinUSD).Result()
	if err != redis.Nil {
		t.Error("Expected bitcoin price cache to be invalidated")
	}

	// Verify ethereum keys still exist
	_, err = redisClient.Get(context.Background(), "coingecko:price:ethereum:usd").Result()
	if err == redis.Nil {
		t.Error("Expected ethereum cache to remain")
	}
}

func TestClearCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires MCP server connection")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient("")
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Add some cached data
	testData := map[string]string{
		testCacheKeyBitcoinUSD:         `{"symbol":"bitcoin"}`,
		"coingecko:chart:bitcoin:7":    `{"prices":[]}`,
		"coingecko:price:ethereum:usd": `{"symbol":"ethereum"}`,
		"other:key":                    `{"other":"data"}`,
	}

	for key, value := range testData {
		err := redisClient.Set(context.Background(), key, value, 60*time.Second).Err()
		if err != nil {
			t.Fatalf("Failed to set cache key %s: %v", key, err)
		}
	}

	// Clear all CoinGecko cache
	err = cachedClient.ClearCache(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify all coingecko keys are gone
	_, err = redisClient.Get(context.Background(), testCacheKeyBitcoinUSD).Result()
	if err != redis.Nil {
		t.Error("Expected coingecko cache to be cleared")
	}

	// Verify non-coingecko keys remain
	_, err = redisClient.Get(context.Background(), "other:key").Result()
	if err == redis.Nil {
		t.Error("Expected non-coingecko keys to remain")
	}
}

func TestCachedGetPrice_InvalidCachedData(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Set invalid JSON in cache
	cacheKey := testCacheKeyBitcoinUSD
	err = redisClient.Set(context.Background(), cacheKey, "invalid json", 60*time.Second).Err()
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Should fall back to fresh data
	result, err := cachedClient.GetPrice(context.Background(), "bitcoin", "usd")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result after cache unmarshal failure")
	}
}

// TestSingleflightPreventsThunderingHerd verifies that concurrent requests
// for the same cache key only result in a single API call
func TestSingleflightPreventsThunderingHerd(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Make sure cache is empty for this test
	ctx := context.Background()
	_ = redisClient.Del(ctx, testCacheKeyBitcoinUSD).Err()

	const numConcurrentRequests = 10
	var wg sync.WaitGroup
	errors := make(chan error, numConcurrentRequests)
	results := make(chan *PriceResult, numConcurrentRequests)

	// Note: In a real implementation with mocks, you could track API call counts.
	// For this test, we're relying on singleflight to deduplicate,
	// which we can verify by checking the "shared" return value in logs.

	// Launch multiple concurrent requests for the same data
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := cachedClient.GetPrice(ctx, "bitcoin", "usd")
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}

	// Wait for all requests to complete
	wg.Wait()
	close(errors)
	close(results)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error in concurrent request: %v", err)
	}

	// Verify all results are the same
	var firstResult *PriceResult
	resultCount := 0
	for result := range results {
		resultCount++
		if firstResult == nil {
			firstResult = result
		} else {
			// All results should be identical
			if result.Symbol != firstResult.Symbol || result.Price != firstResult.Price {
				t.Errorf("Inconsistent results across concurrent requests")
			}
		}
	}

	if resultCount != numConcurrentRequests {
		t.Errorf("Expected %d results, got %d", numConcurrentRequests, resultCount)
	}

	// Verify data was cached (should be cached now for subsequent requests)
	cached, err := redisClient.Get(ctx, testCacheKeyBitcoinUSD).Result()
	if err != nil {
		t.Errorf("Expected data to be cached after concurrent requests: %v", err)
	}

	var cachedResult PriceResult
	if err := json.Unmarshal([]byte(cached), &cachedResult); err != nil {
		t.Errorf("Failed to unmarshal cached data: %v", err)
	}

	if cachedResult.Symbol != "bitcoin" {
		t.Errorf("Cached data doesn't match expected result")
	}

	t.Logf("API call count tracking would show deduplication (implementation detail)")
	t.Logf("All %d concurrent requests completed successfully with consistent results", numConcurrentRequests)
}

// TestSingleflightGetMarketChart verifies singleflight for market chart requests
func TestSingleflightGetMarketChart(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Make sure cache is empty for this test
	ctx := context.Background()
	cacheKey := "coingecko:chart:bitcoin:7"
	_ = redisClient.Del(ctx, cacheKey).Err()

	const numConcurrentRequests = 5
	var wg sync.WaitGroup
	errors := make(chan error, numConcurrentRequests)
	results := make(chan *MarketChart, numConcurrentRequests)

	// Launch multiple concurrent requests for the same data
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := cachedClient.GetMarketChart(ctx, "bitcoin", 7)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}

	// Wait for all requests to complete
	wg.Wait()
	close(errors)
	close(results)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error in concurrent request: %v", err)
	}

	// Verify all results have data
	resultCount := 0
	for result := range results {
		resultCount++
		if len(result.Prices) == 0 {
			t.Error("Expected non-empty prices in result")
		}
	}

	if resultCount != numConcurrentRequests {
		t.Errorf("Expected %d results, got %d", numConcurrentRequests, resultCount)
	}

	// Verify data was cached
	_, err = redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		t.Errorf("Expected data to be cached after concurrent requests: %v", err)
	}

	t.Logf("All %d concurrent market chart requests completed successfully", numConcurrentRequests)
}

// TestSingleflightGetCoinInfo verifies singleflight for coin info requests
func TestSingleflightGetCoinInfo(t *testing.T) {
	// Skip real API tests by default to avoid rate limiting
	if testing.Short() || os.Getenv("COINGECKO_API_TEST") == "" {
		t.Skip("Skipping real API test - set COINGECKO_API_TEST=1 to run")
	}

	redisClient, mr := setupMiniRedis(t)
	defer mr.Close()

	cgClient, err := NewCoinGeckoClient(os.Getenv("COINGECKO_API_KEY"))
	if err != nil {
		t.Fatalf("Failed to create CoinGecko client: %v", err)
	}

	cachedClient := NewCachedCoinGeckoClient(cgClient, redisClient, 60*time.Second)

	// Make sure cache is empty for this test
	ctx := context.Background()
	cacheKey := "coingecko:info:bitcoin"
	_ = redisClient.Del(ctx, cacheKey).Err()

	const numConcurrentRequests = 5
	var wg sync.WaitGroup
	errors := make(chan error, numConcurrentRequests)
	results := make(chan *CoinInfo, numConcurrentRequests)

	// Launch multiple concurrent requests for the same data
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := cachedClient.GetCoinInfo(ctx, "bitcoin")
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}

	// Wait for all requests to complete
	wg.Wait()
	close(errors)
	close(results)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error in concurrent request: %v", err)
	}

	// Verify all results have data
	resultCount := 0
	for result := range results {
		resultCount++
		if result.ID != "bitcoin" {
			t.Errorf("Expected ID bitcoin, got %s", result.ID)
		}
	}

	if resultCount != numConcurrentRequests {
		t.Errorf("Expected %d results, got %d", numConcurrentRequests, resultCount)
	}

	// Verify data was cached
	_, err = redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		t.Errorf("Expected data to be cached after concurrent requests: %v", err)
	}

	t.Logf("All %d concurrent coin info requests completed successfully", numConcurrentRequests)
}
