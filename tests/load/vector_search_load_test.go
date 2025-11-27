package load

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	// Default API base URL for load tests
	defaultAPIURL = "http://localhost:8080/api/v1"

	// Test parameters
	defaultConcurrency = 10
	defaultIterations  = 100
)

// TestConfig holds configuration for load tests
type TestConfig struct {
	APIURL      string
	Concurrency int
	Iterations  int
}

// SearchRequest matches the API search request structure
type SearchRequest struct {
	Query     string     `json:"query"`
	Embedding []float32  `json:"embedding,omitempty"`
	Symbol    string     `json:"symbol,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	FromDate  *time.Time `json:"from_date,omitempty"`
	ToDate    *time.Time `json:"to_date,omitempty"`
}

// SearchResponse matches the API search response structure
type SearchResponse struct {
	Results    []interface{} `json:"results"`
	Count      int           `json:"count"`
	SearchType string        `json:"search_type"`
	Query      string        `json:"query"`
}

// SimilarResponse matches the API similar decisions response structure
type SimilarResponse struct {
	DecisionID uuid.UUID     `json:"decision_id"`
	Similar    []interface{} `json:"similar"`
	Count      int           `json:"count"`
}

// BenchmarkSemanticSearch benchmarks the semantic search endpoint
// This test requires valid embedding vectors to be meaningful
func BenchmarkSemanticSearch(b *testing.B) {
	config := getTestConfig()
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Prepare test embedding (1536 dimensions - OpenAI text-embedding-ada-002 format)
	embedding := generateTestEmbedding()

	searchReq := SearchRequest{
		Query:     "BTC bullish signal RSI oversold",
		Embedding: embedding,
		Limit:     10,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := performSearch(client, config.APIURL, searchReq)
		if err != nil {
			b.Errorf("Search request failed: %v", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			b.Errorf("Search returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
		}
		resp.Body.Close()
	}
}

// BenchmarkTextSearch benchmarks text-only search (no embedding)
func BenchmarkTextSearch(b *testing.B) {
	config := getTestConfig()
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	searchQueries := []string{
		"BTC bullish signal",
		"ETH bearish trend",
		"RSI oversold MACD crossover",
		"risk management stop loss",
		"position sizing kelly criterion",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		query := searchQueries[i%len(searchQueries)]
		searchReq := SearchRequest{
			Query: query,
			Limit: 20,
		}

		resp, err := performSearch(client, config.APIURL, searchReq)
		if err != nil {
			b.Errorf("Search request failed: %v", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			b.Errorf("Search returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
		}
		resp.Body.Close()
	}
}

// BenchmarkSimilarDecisions benchmarks the similar decisions endpoint
// Note: This requires a valid decision ID to exist in the database
func BenchmarkSimilarDecisions(b *testing.B) {
	config := getTestConfig()
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// First, get a valid decision ID from the database
	decisionID, err := getValidDecisionID(client, config.APIURL)
	if err != nil {
		b.Skipf("Skipping benchmark: %v", err)
		return
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := performSimilarQuery(client, config.APIURL, decisionID, 10)
		if err != nil {
			b.Errorf("Similar decisions request failed: %v", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			b.Errorf("Similar decisions returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
		}
		resp.Body.Close()
	}
}

// TestVectorSearchConcurrency tests concurrent vector search requests
func TestVectorSearchConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	config := getTestConfig()
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	concurrency := config.Concurrency
	iterations := config.Iterations

	// Prepare test data
	embedding := generateTestEmbedding()
	searchReq := SearchRequest{
		Query:     "BTC trading signal analysis",
		Embedding: embedding,
		Limit:     10,
	}

	// Track metrics
	var (
		successCount int
		errorCount   int
		mu           sync.Mutex
		latencies    []time.Duration
	)

	// Create worker pool
	var wg sync.WaitGroup
	work := make(chan int, iterations)

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for range work {
				start := time.Now()
				resp, err := performSearch(client, config.APIURL, searchReq)
				duration := time.Since(start)

				mu.Lock()
				latencies = append(latencies, duration)
				if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
					errorCount++
					if resp != nil {
						resp.Body.Close()
					}
				} else {
					successCount++
					resp.Body.Close()
				}
				mu.Unlock()
			}
		}(i)
	}

	// Send work
	testStart := time.Now()
	for i := 0; i < iterations; i++ {
		work <- i
	}
	close(work)

	// Wait for completion
	wg.Wait()
	totalDuration := time.Since(testStart)

	// Calculate statistics
	require.Greater(t, successCount, 0, "No successful requests")
	require.Less(t, errorCount, iterations/10, "More than 10%% error rate")

	avgLatency, p95Latency, p99Latency := calculateLatencyPercentiles(latencies)

	// Log results
	t.Logf("Concurrency Test Results:")
	t.Logf("  Total requests: %d", iterations)
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Success: %d (%.2f%%)", successCount, float64(successCount)/float64(iterations)*100)
	t.Logf("  Errors: %d (%.2f%%)", errorCount, float64(errorCount)/float64(iterations)*100)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Throughput: %.2f req/s", float64(iterations)/totalDuration.Seconds())
	t.Logf("  Avg latency: %v", avgLatency)
	t.Logf("  P95 latency: %v", p95Latency)
	t.Logf("  P99 latency: %v", p99Latency)

	// Assert performance requirements
	require.Less(t, avgLatency, 2*time.Second, "Average latency too high")
	require.Less(t, p95Latency, 5*time.Second, "P95 latency too high")
}

// TestSimilarDecisionsConcurrency tests concurrent similar decisions requests
func TestSimilarDecisionsConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	config := getTestConfig()
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get a valid decision ID
	decisionID, err := getValidDecisionID(client, config.APIURL)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}

	concurrency := config.Concurrency
	iterations := config.Iterations / 2 // Fewer iterations since this is more expensive

	// Track metrics
	var (
		successCount int
		errorCount   int
		mu           sync.Mutex
		latencies    []time.Duration
	)

	// Create worker pool
	var wg sync.WaitGroup
	work := make(chan int, iterations)

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for range work {
				start := time.Now()
				resp, err := performSimilarQuery(client, config.APIURL, decisionID, 10)
				duration := time.Since(start)

				mu.Lock()
				latencies = append(latencies, duration)
				if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
					errorCount++
					if resp != nil {
						resp.Body.Close()
					}
				} else {
					successCount++
					resp.Body.Close()
				}
				mu.Unlock()
			}
		}(i)
	}

	// Send work
	testStart := time.Now()
	for i := 0; i < iterations; i++ {
		work <- i
	}
	close(work)

	// Wait for completion
	wg.Wait()
	totalDuration := time.Since(testStart)

	// Calculate statistics
	require.Greater(t, successCount, 0, "No successful requests")
	require.Less(t, errorCount, iterations/10, "More than 10%% error rate")

	avgLatency, p95Latency, p99Latency := calculateLatencyPercentiles(latencies)

	// Log results
	t.Logf("Similar Decisions Concurrency Test Results:")
	t.Logf("  Total requests: %d", iterations)
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Success: %d (%.2f%%)", successCount, float64(successCount)/float64(iterations)*100)
	t.Logf("  Errors: %d (%.2f%%)", errorCount, float64(errorCount)/float64(iterations)*100)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Throughput: %.2f req/s", float64(iterations)/totalDuration.Seconds())
	t.Logf("  Avg latency: %v", avgLatency)
	t.Logf("  P95 latency: %v", p95Latency)
	t.Logf("  P99 latency: %v", p99Latency)

	// Assert performance requirements (more lenient for expensive operations)
	require.Less(t, avgLatency, 3*time.Second, "Average latency too high")
	require.Less(t, p95Latency, 8*time.Second, "P95 latency too high")
}

// TestVectorSearchStress performs a stress test with higher concurrency
func TestVectorSearchStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := getTestConfig()
	config.Concurrency = 50 // Higher concurrency for stress test
	config.Iterations = 500

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Prepare various test queries
	testQueries := []SearchRequest{
		{Query: "BTC bullish momentum", Limit: 10},
		{Query: "ETH bearish reversal", Limit: 20},
		{Query: "RSI overbought MACD bearish", Limit: 15},
		{Query: "support resistance breakout", Limit: 10},
		{Query: "volume surge price action", Limit: 10},
	}

	// Track metrics
	var (
		successCount int
		errorCount   int
		mu           sync.Mutex
		latencies    []time.Duration
	)

	// Create worker pool
	var wg sync.WaitGroup
	work := make(chan int, config.Iterations)

	// Start workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for reqNum := range work {
				// Rotate through different queries
				searchReq := testQueries[reqNum%len(testQueries)]

				start := time.Now()
				resp, err := performSearch(client, config.APIURL, searchReq)
				duration := time.Since(start)

				mu.Lock()
				latencies = append(latencies, duration)
				if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
					errorCount++
					if resp != nil {
						resp.Body.Close()
					}
				} else {
					successCount++
					resp.Body.Close()
				}
				mu.Unlock()
			}
		}(i)
	}

	// Send work
	testStart := time.Now()
	for i := 0; i < config.Iterations; i++ {
		work <- i
	}
	close(work)

	// Wait for completion
	wg.Wait()
	totalDuration := time.Since(testStart)

	// Calculate statistics
	require.Greater(t, successCount, 0, "No successful requests")
	require.Less(t, errorCount, config.Iterations/5, "More than 20%% error rate in stress test")

	avgLatency, p95Latency, p99Latency := calculateLatencyPercentiles(latencies)

	// Log results
	t.Logf("Stress Test Results:")
	t.Logf("  Total requests: %d", config.Iterations)
	t.Logf("  Concurrency: %d", config.Concurrency)
	t.Logf("  Success: %d (%.2f%%)", successCount, float64(successCount)/float64(config.Iterations)*100)
	t.Logf("  Errors: %d (%.2f%%)", errorCount, float64(errorCount)/float64(config.Iterations)*100)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Throughput: %.2f req/s", float64(config.Iterations)/totalDuration.Seconds())
	t.Logf("  Avg latency: %v", avgLatency)
	t.Logf("  P95 latency: %v", p95Latency)
	t.Logf("  P99 latency: %v", p99Latency)

	// More lenient assertions for stress test
	require.Less(t, avgLatency, 5*time.Second, "Average latency too high under stress")
}

// Helper functions

func getTestConfig() TestConfig {
	// TODO: Read from environment variables or config file
	return TestConfig{
		APIURL:      defaultAPIURL,
		Concurrency: defaultConcurrency,
		Iterations:  defaultIterations,
	}
}

func generateTestEmbedding() []float32 {
	// Generate random normalized embedding vector (1536 dimensions)
	embedding := make([]float32, 1536)
	var sum float32
	for i := range embedding {
		embedding[i] = rand.Float32()*2 - 1 // Random between -1 and 1
		sum += embedding[i] * embedding[i]
	}
	// Normalize
	magnitude := float32(1.0) / float32(sum)
	for i := range embedding {
		embedding[i] *= magnitude
	}
	return embedding
}

func performSearch(client *http.Client, apiURL string, searchReq SearchRequest) (*http.Response, error) {
	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL+"/decisions/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func performSimilarQuery(client *http.Client, apiURL string, decisionID uuid.UUID, limit int) (*http.Response, error) {
	url := fmt.Sprintf("%s/decisions/%s/similar?limit=%d", apiURL, decisionID.String(), limit)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return client.Do(req)
}

func getValidDecisionID(client *http.Client, apiURL string) (uuid.UUID, error) {
	// Try to get the first decision from the list endpoint
	req, err := http.NewRequest(http.MethodGet, apiURL+"/decisions?limit=1", nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to list decisions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, fmt.Errorf("list decisions returned status %d", resp.StatusCode)
	}

	var response struct {
		Decisions []struct {
			ID uuid.UUID `json:"id"`
		} `json:"decisions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return uuid.Nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Decisions) == 0 {
		return uuid.Nil, fmt.Errorf("no decisions found in database")
	}

	return response.Decisions[0].ID, nil
}

func calculateLatencyPercentiles(latencies []time.Duration) (avg, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate average
	var sum time.Duration
	for _, l := range sorted {
		sum += l
	}
	avg = sum / time.Duration(len(sorted))

	// Calculate percentiles
	p95Index := int(float64(len(sorted)) * 0.95)
	p99Index := int(float64(len(sorted)) * 0.99)

	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	if p99Index >= len(sorted) {
		p99Index = len(sorted) - 1
	}

	p95 = sorted[p95Index]
	p99 = sorted[p99Index]

	return
}
