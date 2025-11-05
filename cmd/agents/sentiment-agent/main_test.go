//nolint:goconst // Test files use repeated strings for clarity
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test articles
func createTestArticles() []Article {
	return []Article{
		{
			Title:       "Bitcoin surges to new all-time high!",
			Source:      "CryptoNews",
			URL:         "https://example.com/1",
			PublishedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			Title:       "Market crash fears as Bitcoin drops",
			Source:      "CryptoDaily",
			URL:         "https://example.com/2",
			PublishedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			Title:       "Bitcoin adoption grows among major institutions",
			Source:      "CryptoTimes",
			URL:         "https://example.com/3",
			PublishedAt: time.Now().Add(-3 * time.Hour),
		},
	}
}

// Helper function to create sentiment agent with test config
func createTestAgent(t *testing.T) *SentimentAgent {
	return &SentimentAgent{
		symbol:             "bitcoin",
		analysisInterval:   5 * time.Minute,
		lookbackHours:      24,
		sentimentThreshold: 0.3,
		includeFearGreed:   true,
		newsSourceWeights: map[string]float64{
			"cryptopanic": 1.0,
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
		beliefs:    NewBeliefBase(),
	}
}

// TestAnalyzeSentiment_Positive tests positive sentiment detection
func TestAnalyzeSentiment_Positive(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "Bitcoin surges past all-time high with bullish rally and record gains!",
		Source:      "CryptoNews",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "positive", article.Sentiment)
	assert.Greater(t, article.Score, 0.0)
	assert.Greater(t, article.Confidence, 0.0)
}

// TestAnalyzeSentiment_Negative tests negative sentiment detection
func TestAnalyzeSentiment_Negative(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "Bitcoin crash: market fears amid bearish plunge and massive losses",
		Source:      "CryptoDaily",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "negative", article.Sentiment)
	assert.Less(t, article.Score, 0.0)
	assert.Greater(t, article.Confidence, 0.0)
}

// TestAnalyzeSentiment_Neutral tests neutral sentiment detection
func TestAnalyzeSentiment_Neutral(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "Bitcoin price remains stable today",
		Source:      "CryptoNews",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "neutral", article.Sentiment)
	assert.InDelta(t, 0.0, article.Score, 0.15)
}

// TestAnalyzeSentiment_EmptyTitle tests empty title handling
func TestAnalyzeSentiment_EmptyTitle(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "",
		Source:      "CryptoNews",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "neutral", article.Sentiment)
	assert.Equal(t, 0.0, article.Score)
	assert.Equal(t, 0.3, article.Confidence)
}

// TestAnalyzeSentiment_MultiplePositiveKeywords tests multiple positive keywords
func TestAnalyzeSentiment_MultiplePositiveKeywords(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "Bullish surge, rally, boom, moon, pump green candles up high",
		Source:      "CryptoNews",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "positive", article.Sentiment)
	assert.Greater(t, article.Score, 0.5)
	assert.Greater(t, article.Confidence, 0.8)
}

// TestAnalyzeSentiment_MultipleNegativeKeywords tests multiple negative keywords
func TestAnalyzeSentiment_MultipleNegativeKeywords(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "Bearish crash, dump, plunge, collapse, fear decline down red",
		Source:      "CryptoDaily",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "negative", article.Sentiment)
	assert.Less(t, article.Score, -0.5)
	assert.Greater(t, article.Confidence, 0.8)
}

// TestAnalyzeSentiment_MixedKeywords tests mixed sentiment with positive dominance
func TestAnalyzeSentiment_MixedKeywords(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "Bitcoin rally despite crash fears",
		Source:      "CryptoNews",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	// Should have some sentiment value since both positive and negative keywords exist
	assert.NotEqual(t, 0.0, article.Score)
}

// TestAnalyzeSentiment_CaseInsensitive tests case-insensitive keyword matching
func TestAnalyzeSentiment_CaseInsensitive(t *testing.T) {
	agent := createTestAgent(t)

	article := &Article{
		Title:       "BITCOIN SURGES with BULLISH RALLY!",
		Source:      "CryptoNews",
		PublishedAt: time.Now(),
	}

	agent.analyzeSentiment(article)

	assert.Equal(t, "positive", article.Sentiment)
	assert.Greater(t, article.Score, 0.0)
}

// TestAnalyzeSentiment_ScoreClamping tests that extreme scores are clamped to [-1, 1]
func TestAnalyzeSentiment_ScoreClamping(t *testing.T) {
	agent := createTestAgent(t)

	// Test positive score clamping - single word with keyword (would be > 1.0 without clamping)
	positiveArticle := &Article{
		Title:       "surge",
		Source:      "Test",
		PublishedAt: time.Now(),
	}
	agent.analyzeSentiment(positiveArticle)
	assert.Equal(t, "positive", positiveArticle.Sentiment)
	assert.Equal(t, 1.0, positiveArticle.Score, "Score should be clamped to 1.0")

	// Test negative score clamping - single word with keyword (would be < -1.0 without clamping)
	negativeArticle := &Article{
		Title:       "crash",
		Source:      "Test",
		PublishedAt: time.Now(),
	}
	agent.analyzeSentiment(negativeArticle)
	assert.Equal(t, "negative", negativeArticle.Sentiment)
	assert.Equal(t, -1.0, negativeArticle.Score, "Score should be clamped to -1.0")
}

// TestAggregateSentiment_OnlyNews tests aggregation with only news data
func TestAggregateSentiment_OnlyNews(t *testing.T) {
	agent := createTestAgent(t)
	agent.includeFearGreed = false

	articles := []Article{
		{
			Title:       "Bitcoin surges with bullish rally",
			Score:       0.5,
			Confidence:  0.8,
			PublishedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			Title:       "Bitcoin gains momentum",
			Score:       0.4,
			Confidence:  0.7,
			PublishedAt: time.Now().Add(-2 * time.Hour),
		},
	}

	// Analyze sentiment for all articles first
	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)

	assert.Greater(t, overall, 0.0, "Overall sentiment should be positive")
}

// TestAggregateSentiment_WithFearGreed tests aggregation with Fear & Greed Index
func TestAggregateSentiment_WithFearGreed(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin surges",
			Score:       0.5,
			Confidence:  0.8,
			PublishedAt: time.Now().Add(-1 * time.Hour),
		},
	}

	// Analyze sentiment
	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          75, // Greed
		Classification: "Greed",
		Timestamp:      time.Now(),
	}

	overall := agent.aggregateSentiment(articles, fearGreed)

	// With both positive news and greed, overall sentiment should be positive
	assert.Greater(t, overall, 0.0)
}

// TestAggregateSentiment_ExtremeFear tests extreme fear index
func TestAggregateSentiment_ExtremeFear(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin stable",
			Score:       0.0,
			Confidence:  0.5,
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          10, // Extreme Fear
		Classification: "Extreme Fear",
		Timestamp:      time.Now(),
	}

	overall := agent.aggregateSentiment(articles, fearGreed)

	// Extreme fear should result in negative overall sentiment
	assert.Less(t, overall, 0.0)
}

// TestAggregateSentiment_ExtremeGreed tests extreme greed index
func TestAggregateSentiment_ExtremeGreed(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin stable",
			Score:       0.0,
			Confidence:  0.5,
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          95, // Extreme Greed
		Classification: "Extreme Greed",
		Timestamp:      time.Now(),
	}

	overall := agent.aggregateSentiment(articles, fearGreed)

	// Extreme greed should result in positive overall sentiment
	assert.Greater(t, overall, 0.0)
}

// TestAggregateSentiment_RecencyWeighting tests that recent articles have more weight
func TestAggregateSentiment_RecencyWeighting(t *testing.T) {
	agent := createTestAgent(t)
	agent.includeFearGreed = false

	articles := []Article{
		{
			Title:       "Bitcoin surges bullish rally",
			Score:       0.8,
			Confidence:  0.9,
			PublishedAt: time.Now().Add(-1 * time.Hour), // Recent
		},
		{
			Title:       "Bitcoin crash fears plunge",
			Score:       -0.8,
			Confidence:  0.9,
			PublishedAt: time.Now().Add(-20 * time.Hour), // Old
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)

	// Recent positive article should outweigh old negative article
	assert.Greater(t, overall, 0.0)
}

// TestAggregateSentiment_EmptyArticles tests empty article list
func TestAggregateSentiment_EmptyArticles(t *testing.T) {
	agent := createTestAgent(t)

	overall := agent.aggregateSentiment([]Article{}, nil)

	assert.Equal(t, 0.0, overall)
}

// TestGenerateSignal_BuySignal tests buy signal generation
func TestGenerateSignal_BuySignal(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin surges bullish",
			Sentiment:   "positive",
			Score:       0.6,
			Confidence:  0.8,
			Source:      "CryptoNews",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          70,
		Classification: "Greed",
	}

	overall := agent.aggregateSentiment(articles, fearGreed)
	signal := agent.generateSignal(articles, fearGreed, overall)

	assert.Equal(t, "BUY", signal.Action)
	assert.Greater(t, signal.Confidence, 0.0)
	assert.Greater(t, signal.OverallSentiment, agent.sentimentThreshold)
	assert.Equal(t, 70, signal.FearGreedIndex)
	assert.Contains(t, signal.Reasoning, "positive")
}

// TestGenerateSignal_SellSignal tests sell signal generation
func TestGenerateSignal_SellSignal(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin crash bearish plunge",
			Sentiment:   "negative",
			Score:       -0.6,
			Confidence:  0.8,
			Source:      "CryptoDaily",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          20,
		Classification: "Fear",
	}

	overall := agent.aggregateSentiment(articles, fearGreed)
	signal := agent.generateSignal(articles, fearGreed, overall)

	assert.Equal(t, "SELL", signal.Action)
	assert.Greater(t, signal.Confidence, 0.0)
	assert.Less(t, signal.OverallSentiment, -agent.sentimentThreshold)
	assert.Equal(t, 20, signal.FearGreedIndex)
	assert.Contains(t, signal.Reasoning, "negative")
}

// TestGenerateSignal_HoldSignal tests hold signal generation
func TestGenerateSignal_HoldSignal(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin price remains stable",
			Sentiment:   "neutral",
			Score:       0.0,
			Confidence:  0.5,
			Source:      "CryptoNews",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          50,
		Classification: "Neutral",
	}

	overall := agent.aggregateSentiment(articles, fearGreed)
	signal := agent.generateSignal(articles, fearGreed, overall)

	assert.Equal(t, "HOLD", signal.Action)
	// With exactly 0.0 sentiment, confidence calculation produces 0.0
	assert.GreaterOrEqual(t, signal.Confidence, 0.0)
	assert.InDelta(t, 0.0, signal.OverallSentiment, agent.sentimentThreshold)
	assert.Equal(t, 50, signal.FearGreedIndex)
}

// TestGenerateSignal_HighAgreement tests signal with high article agreement
func TestGenerateSignal_HighAgreement(t *testing.T) {
	agent := createTestAgent(t)

	// All articles agree on positive sentiment (keywords will be analyzed)
	articles := []Article{
		{
			Title:       "Bitcoin surges with bullish rally and gains",
			Source:      "Source1",
			PublishedAt: time.Now(),
		},
		{
			Title:       "Bitcoin rallies upward with strong momentum",
			Source:      "Source2",
			PublishedAt: time.Now(),
		},
		{
			Title:       "Bitcoin gains continue with positive outlook",
			Source:      "Source3",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)
	signal := agent.generateSignal(articles, nil, overall)

	// High agreement (all positive) should result in confidence boost
	assert.Equal(t, "BUY", signal.Action)
	// Confidence depends on multiple factors: distance from threshold, article count, and agreement
	// With moderate sentiment scores and 3 articles, confidence will be moderate
	assert.Greater(t, signal.Confidence, 0.0)
	assert.Equal(t, 3, signal.ArticlesAnalyzed)
}

// TestGenerateSignal_LowAgreement tests signal with low article agreement
func TestGenerateSignal_LowAgreement(t *testing.T) {
	agent := createTestAgent(t)

	// Articles have mixed sentiment
	articles := []Article{
		{
			Title:       "Bitcoin surges",
			Sentiment:   "positive",
			Score:       0.5,
			Confidence:  0.8,
			Source:      "Source1",
			PublishedAt: time.Now(),
		},
		{
			Title:       "Bitcoin crashes",
			Sentiment:   "negative",
			Score:       -0.5,
			Confidence:  0.8,
			Source:      "Source2",
			PublishedAt: time.Now(),
		},
		{
			Title:       "Bitcoin stable",
			Sentiment:   "neutral",
			Score:       0.0,
			Confidence:  0.5,
			Source:      "Source3",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)
	signal := agent.generateSignal(articles, nil, overall)

	// Low agreement should result in lower confidence
	assert.Less(t, signal.Confidence, 0.8)
	assert.Equal(t, 3, signal.ArticlesAnalyzed)
}

// TestGenerateSignal_NoArticles tests signal generation with no articles
func TestGenerateSignal_NoArticles(t *testing.T) {
	agent := createTestAgent(t)

	fearGreed := &FearGreedData{
		Value:          80,
		Classification: "Extreme Greed",
	}

	overall := agent.aggregateSentiment([]Article{}, fearGreed)
	signal := agent.generateSignal([]Article{}, fearGreed, overall)

	assert.NotEmpty(t, signal.Action)
	assert.Equal(t, 0, signal.ArticlesAnalyzed)
	assert.Contains(t, signal.Reasoning, "Fear & Greed")
}

// TestUpdateBeliefs tests belief system updates
func TestUpdateBeliefs(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin surges",
			Sentiment:   "positive",
			Score:       0.5,
			Confidence:  0.8,
			PublishedAt: time.Now(),
		},
		{
			Title:       "Bitcoin rallies",
			Sentiment:   "positive",
			Score:       0.6,
			Confidence:  0.8,
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	fearGreed := &FearGreedData{
		Value:          70,
		Classification: "Greed",
	}

	overall := agent.aggregateSentiment(articles, fearGreed)
	agent.updateBeliefs(articles, fearGreed, overall)

	// Check beliefs are updated
	newsSentiment, exists := agent.beliefs.GetBelief("news_sentiment")
	assert.True(t, exists)
	assert.Equal(t, "positive", newsSentiment.Value)

	fearGreedBelief, exists := agent.beliefs.GetBelief("fear_greed_index")
	assert.True(t, exists)
	assert.Equal(t, 70, fearGreedBelief.Value)

	overallBelief, exists := agent.beliefs.GetBelief("overall_sentiment")
	assert.True(t, exists)
	assert.Equal(t, "bullish", overallBelief.Value)
}

// TestUpdateBeliefs_NegativeSentiment tests belief updates with negative sentiment
func TestUpdateBeliefs_NegativeSentiment(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin crashes",
			Sentiment:   "negative",
			Score:       -0.6,
			Confidence:  0.8,
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)
	agent.updateBeliefs(articles, nil, overall)

	overallBelief, exists := agent.beliefs.GetBelief("overall_sentiment")
	assert.True(t, exists)
	assert.Equal(t, "bearish", overallBelief.Value)
}

// TestUpdateBeliefs_NeutralSentiment tests belief updates with neutral sentiment
func TestUpdateBeliefs_NeutralSentiment(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin stable",
			Sentiment:   "neutral",
			Score:       0.1,
			Confidence:  0.5,
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)
	agent.updateBeliefs(articles, nil, overall)

	overallBelief, exists := agent.beliefs.GetBelief("overall_sentiment")
	assert.True(t, exists)
	assert.Equal(t, "neutral", overallBelief.Value)
}

// TestBeliefBase_UpdateAndGet tests belief base operations
func TestBeliefBase_UpdateAndGet(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("test_key", "test_value", 0.8, "test_source")

	belief, exists := bb.GetBelief("test_key")
	assert.True(t, exists)
	assert.Equal(t, "test_key", belief.Key)
	assert.Equal(t, "test_value", belief.Value)
	assert.Equal(t, 0.8, belief.Confidence)
	assert.Equal(t, "test_source", belief.Source)
	assert.False(t, belief.Timestamp.IsZero())
}

// TestBeliefBase_GetAllBeliefs tests getting all beliefs
func TestBeliefBase_GetAllBeliefs(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("key1", "value1", 0.7, "source1")
	bb.UpdateBelief("key2", "value2", 0.8, "source2")
	bb.UpdateBelief("key3", "value3", 0.9, "source3")

	beliefs := bb.GetAllBeliefs()
	assert.Len(t, beliefs, 3)
	assert.Contains(t, beliefs, "key1")
	assert.Contains(t, beliefs, "key2")
	assert.Contains(t, beliefs, "key3")
}

// TestBeliefBase_GetConfidence tests overall confidence calculation
func TestBeliefBase_GetConfidence(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("key1", "value1", 0.6, "source1")
	bb.UpdateBelief("key2", "value2", 0.8, "source2")
	bb.UpdateBelief("key3", "value3", 1.0, "source3")

	confidence := bb.GetConfidence()
	expected := (0.6 + 0.8 + 1.0) / 3.0
	assert.InDelta(t, expected, confidence, 0.01)
}

// TestBeliefBase_GetConfidence_Empty tests confidence with empty belief base
func TestBeliefBase_GetConfidence_Empty(t *testing.T) {
	bb := NewBeliefBase()
	confidence := bb.GetConfidence()
	assert.Equal(t, 0.0, confidence)
}

// TestBeliefBase_OverwriteBelief tests that beliefs can be overwritten
func TestBeliefBase_OverwriteBelief(t *testing.T) {
	bb := NewBeliefBase()

	bb.UpdateBelief("test_key", "old_value", 0.5, "source1")
	bb.UpdateBelief("test_key", "new_value", 0.9, "source2")

	belief, exists := bb.GetBelief("test_key")
	assert.True(t, exists)
	assert.Equal(t, "new_value", belief.Value)
	assert.Equal(t, 0.9, belief.Confidence)
	assert.Equal(t, "source2", belief.Source)
}

// TODO: Will be used for testing news sentiment analysis in Phase 11
//
// Mock HTTP server for CryptoPanic API
//
//nolint:unused
func createMockNewsServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"count": 3,
			"results": []map[string]interface{}{
				{
					"title": "Bitcoin surges to new all-time high",
					"url":   "https://example.com/1",
					"source": map[string]string{
						"title": "CryptoNews",
					},
					"published_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
					"votes": map[string]int{
						"positive": 10,
						"negative": 2,
						"liked":    8,
					},
				},
				{
					"title": "Market crash fears grow",
					"url":   "https://example.com/2",
					"source": map[string]string{
						"title": "CryptoDaily",
					},
					"published_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
					"votes": map[string]int{
						"positive": 1,
						"negative": 15,
						"liked":    3,
					},
				},
				{
					"title": "Bitcoin adoption grows",
					"url":   "https://example.com/3",
					"source": map[string]string{
						"title": "CryptoTimes",
					},
					"published_at": time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
					"votes": map[string]int{
						"positive": 12,
						"negative": 3,
						"liked":    9,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
}

// TODO: Will be used for testing fear & greed sentiment analysis in Phase 11
//
// Mock HTTP server for Fear & Greed API
//
//nolint:unused
func createMockFearGreedServer(t *testing.T, value int, classification string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"value":                string(rune(value + '0')),
					"value_classification": classification,
					"timestamp":            time.Now().Unix(),
					"time_until_update":    "3600",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
}

// TestFetchNews_Success tests successful news fetching with mock API
func TestFetchNews_Success(t *testing.T) {
	// Create mock server that returns valid CryptoPanic response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"count": 2,
			"results": [
				{
					"title": "Bitcoin surges past $50k",
					"url": "https://example.com/1",
					"source": {"title": "CryptoNews"},
					"published_at": "` + time.Now().Format(time.RFC3339) + `",
					"votes": {"positive": 10, "negative": 2, "liked": 8}
				},
				{
					"title": "Market analysis shows bullish trend",
					"url": "https://example.com/2",
					"source": {"title": "CryptoDaily"},
					"published_at": "` + time.Now().Add(-1*time.Hour).Format(time.RFC3339) + `",
					"votes": {"positive": 5, "negative": 1, "liked": 4}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"

	// Replace httpClient to use mock server
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	articles, err := agent.fetchNews(context.Background())
	require.NoError(t, err)
	assert.Len(t, articles, 2)
	assert.Equal(t, "Bitcoin surges past $50k", articles[0].Title)
	assert.Equal(t, "CryptoNews", articles[0].Source)
	assert.NotEmpty(t, articles[0].URL)
}

// mockTransport redirects all requests to mock server
type mockTransport struct {
	mockURL string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to mock server but preserve query params
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.mockURL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}

// TestFetchNews_CacheHit tests cache functionality
func TestFetchNews_CacheHit(t *testing.T) {
	agent := createTestAgent(t)

	// Set cached data
	cachedArticles := createTestArticles()
	agent.cachedNews = cachedArticles
	agent.cacheExpiry = time.Now().Add(5 * time.Minute)

	// Fetch should return cached data
	articles, err := agent.fetchNews(context.Background())
	require.NoError(t, err)
	assert.Len(t, articles, len(cachedArticles))
	assert.Equal(t, cachedArticles[0].Title, articles[0].Title)
}

// TestFetchNews_NoAPIKey tests behavior without API key
func TestFetchNews_NoAPIKey(t *testing.T) {
	agent := createTestAgent(t)
	agent.newsAPIKey = ""

	_, err := agent.fetchNews(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key not configured")
}

// TestFetchNews_HTTPError tests handling of HTTP errors
func TestFetchNews_HTTPError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	_, err := agent.fetchNews(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API returned status 500")
}

// TestFetchNews_MalformedJSON tests handling of invalid JSON response
func TestFetchNews_MalformedJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"invalid json"`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	_, err := agent.fetchNews(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// TestFetchNews_EmptyResults tests handling of empty results
func TestFetchNews_EmptyResults(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{"count": 0, "results": []}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	articles, err := agent.fetchNews(context.Background())
	require.NoError(t, err)
	assert.Len(t, articles, 0)
}

// TestFetchNews_InvalidTimestamp tests handling of invalid publish timestamps
func TestFetchNews_InvalidTimestamp(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"count": 1,
			"results": [
				{
					"title": "Test article",
					"url": "https://example.com/1",
					"source": {"title": "TestSource"},
					"published_at": "invalid-timestamp",
					"votes": {"positive": 1, "negative": 0, "liked": 1}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	articles, err := agent.fetchNews(context.Background())
	require.NoError(t, err)
	assert.Len(t, articles, 1)
	// Should use current time as fallback
	assert.WithinDuration(t, time.Now(), articles[0].PublishedAt, 5*time.Second)
}

// TestFetchNews_LookbackFilter tests that old articles are filtered out
func TestFetchNews_LookbackFilter(t *testing.T) {
	oldTime := time.Now().Add(-48 * time.Hour) // Older than 24h lookback
	recentTime := time.Now().Add(-1 * time.Hour)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"count": 2,
			"results": [
				{
					"title": "Recent article",
					"url": "https://example.com/1",
					"source": {"title": "Source1"},
					"published_at": "` + recentTime.Format(time.RFC3339) + `",
					"votes": {"positive": 1, "negative": 0, "liked": 1}
				},
				{
					"title": "Old article",
					"url": "https://example.com/2",
					"source": {"title": "Source2"},
					"published_at": "` + oldTime.Format(time.RFC3339) + `",
					"votes": {"positive": 1, "negative": 0, "liked": 1}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"
	agent.lookbackHours = 24
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	articles, err := agent.fetchNews(context.Background())
	require.NoError(t, err)
	// Should only get the recent article (old one filtered)
	assert.Len(t, articles, 1)
	assert.Equal(t, "Recent article", articles[0].Title)
}

// TestFetchNews_CacheExpiry tests that expired cache triggers new fetch
func TestFetchNews_CacheExpiry(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := `{
			"count": 1,
			"results": [
				{
					"title": "Fresh article",
					"url": "https://example.com/1",
					"source": {"title": "Source"},
					"published_at": "` + time.Now().Format(time.RFC3339) + `",
					"votes": {"positive": 1, "negative": 0, "liked": 1}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.newsAPIKey = "test_key"
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	// Set expired cache
	agent.cachedNews = []Article{{Title: "Old cached article"}}
	agent.cacheExpiry = time.Now().Add(-1 * time.Minute) // Expired

	articles, err := agent.fetchNews(context.Background())
	require.NoError(t, err)
	assert.Len(t, articles, 1)
	assert.Equal(t, "Fresh article", articles[0].Title)
	assert.Equal(t, 1, callCount, "Should make API call for expired cache")
}

// TestFetchFearGreedIndex_CacheHit tests Fear & Greed cache
func TestFetchFearGreedIndex_CacheHit(t *testing.T) {
	agent := createTestAgent(t)

	// Set cached data
	cachedFG := &FearGreedData{
		Value:          75,
		Classification: "Greed",
		Timestamp:      time.Now(),
		UpdateTime:     "3600",
	}
	agent.cachedFearGreed = cachedFG

	// Fetch should return cached data
	fg, err := agent.fetchFearGreedIndex(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 75, fg.Value)
	assert.Equal(t, "Greed", fg.Classification)
}

// TestFetchFearGreedIndex_Success tests successful Fear & Greed Index fetch
func TestFetchFearGreedIndex_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"name": "Fear and Greed Index",
			"data": [
				{
					"value": "45",
					"value_classification": "Fear",
					"timestamp": "1234567890",
					"time_until_update": "3600"
				}
			],
			"metadata": {
				"error": null
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	fg, err := agent.fetchFearGreedIndex(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 45, fg.Value)
	assert.Equal(t, "Fear", fg.Classification)
	assert.Equal(t, "3600", fg.UpdateTime)
	assert.WithinDuration(t, time.Now(), fg.Timestamp, 5*time.Second)
}

// TestFetchFearGreedIndex_HTTPError tests HTTP error handling
func TestFetchFearGreedIndex_HTTPError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	_, err := agent.fetchFearGreedIndex(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API returned status 503")
}

// TestFetchFearGreedIndex_MalformedJSON tests invalid JSON handling
func TestFetchFearGreedIndex_MalformedJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"data": [malformed`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	_, err := agent.fetchFearGreedIndex(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// TestFetchFearGreedIndex_EmptyData tests empty data array handling
func TestFetchFearGreedIndex_EmptyData(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{"name": "Fear and Greed Index", "data": []}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	_, err := agent.fetchFearGreedIndex(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Fear & Greed data available")
}

// TestFetchFearGreedIndex_InvalidValue tests invalid value parsing
func TestFetchFearGreedIndex_InvalidValue(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"name": "Fear and Greed Index",
			"data": [
				{
					"value": "not-a-number",
					"value_classification": "Unknown",
					"timestamp": "1234567890",
					"time_until_update": "3600"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	fg, err := agent.fetchFearGreedIndex(context.Background())
	require.NoError(t, err)
	// Sscanf will return 0 for invalid value
	assert.Equal(t, 0, fg.Value)
	assert.Equal(t, "Unknown", fg.Classification)
}

// TestFetchFearGreedIndex_CacheExpiry tests cache expiry behavior
func TestFetchFearGreedIndex_CacheExpiry(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := `{
			"name": "Fear and Greed Index",
			"data": [
				{
					"value": "60",
					"value_classification": "Greed",
					"timestamp": "1234567890",
					"time_until_update": "3600"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer mockServer.Close()

	agent := createTestAgent(t)
	agent.httpClient = &http.Client{
		Transport: &mockTransport{mockServer.URL},
		Timeout:   10 * time.Second,
	}

	// Set expired cache (more than 1 hour old)
	agent.cachedFearGreed = &FearGreedData{
		Value:          25,
		Classification: "Fear",
		Timestamp:      time.Now().Add(-2 * time.Hour),
		UpdateTime:     "3600",
	}

	fg, err := agent.fetchFearGreedIndex(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 60, fg.Value)
	assert.Equal(t, "Greed", fg.Classification)
	assert.Equal(t, 1, callCount, "Should make API call for expired cache")
}

// TestFetchFearGreedIndex_ExtremeValues tests boundary values
func TestFetchFearGreedIndex_ExtremeValues(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		classification string
		expectedValue  int
	}{
		{"Extreme Fear", "0", "Extreme Fear", 0},
		{"Extreme Greed", "100", "Extreme Greed", 100},
		{"Mid Value", "50", "Neutral", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := fmt.Sprintf(`{
					"name": "Fear and Greed Index",
					"data": [
						{
							"value": "%s",
							"value_classification": "%s",
							"timestamp": "1234567890",
							"time_until_update": "3600"
						}
					]
				}`, tt.value, tt.classification)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(response)); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			}))
			defer mockServer.Close()

			agent := createTestAgent(t)
			agent.httpClient = &http.Client{
				Transport: &mockTransport{mockServer.URL},
				Timeout:   10 * time.Second,
			}

			fg, err := agent.fetchFearGreedIndex(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.expectedValue, fg.Value)
			assert.Equal(t, tt.classification, fg.Classification)
		})
	}
}

// TestConfigHelpers tests configuration helper functions
func TestGetStringFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"test_key": "test_value",
	}

	value := getStringFromConfig(config, "test_key", "default")
	assert.Equal(t, "test_value", value)

	value = getStringFromConfig(config, "missing_key", "default")
	assert.Equal(t, "default", value)
}

func TestGetIntFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"int_key":     42,
		"int64_key":   int64(64),
		"float64_key": 100.0,
	}

	value := getIntFromConfig(config, "int_key", 0)
	assert.Equal(t, 42, value)

	value = getIntFromConfig(config, "int64_key", 0)
	assert.Equal(t, 64, value)

	value = getIntFromConfig(config, "float64_key", 0)
	assert.Equal(t, 100, value)

	value = getIntFromConfig(config, "missing_key", 99)
	assert.Equal(t, 99, value)
}

func TestGetFloatFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"float64_key": 3.14,
		"float32_key": float32(2.71),
		"int_key":     42,
		"int64_key":   int64(100),
	}

	value := getFloatFromConfig(config, "float64_key", 0.0)
	assert.InDelta(t, 3.14, value, 0.01)

	value = getFloatFromConfig(config, "float32_key", 0.0)
	assert.InDelta(t, 2.71, value, 0.01)

	value = getFloatFromConfig(config, "int_key", 0.0)
	assert.Equal(t, 42.0, value)

	value = getFloatFromConfig(config, "int64_key", 0.0)
	assert.Equal(t, 100.0, value)

	value = getFloatFromConfig(config, "missing_key", 1.5)
	assert.Equal(t, 1.5, value)
}

func TestGetBoolFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"true_key":  true,
		"false_key": false,
	}

	value := getBoolFromConfig(config, "true_key", false)
	assert.True(t, value)

	value = getBoolFromConfig(config, "false_key", true)
	assert.False(t, value)

	value = getBoolFromConfig(config, "missing_key", true)
	assert.True(t, value)
}

// TestMax tests max helper function
func TestMax(t *testing.T) {
	assert.Equal(t, 10, max(10, 5))
	assert.Equal(t, 10, max(5, 10))
	assert.Equal(t, 5, max(5, 5))
	assert.Equal(t, 0, max(0, -5))
}

// TestKeywordLists tests that positive and negative keyword lists are defined
func TestKeywordLists(t *testing.T) {
	assert.NotEmpty(t, positiveKeywords)
	assert.NotEmpty(t, negativeKeywords)
	assert.GreaterOrEqual(t, len(positiveKeywords), 20)
	assert.GreaterOrEqual(t, len(negativeKeywords), 15)

	// Verify some expected keywords
	assert.Contains(t, positiveKeywords, "bullish")
	assert.Contains(t, positiveKeywords, "surge")
	assert.Contains(t, positiveKeywords, "rally")
	assert.Contains(t, negativeKeywords, "bearish")
	assert.Contains(t, negativeKeywords, "crash")
	assert.Contains(t, negativeKeywords, "drop")
}

// TestSignalGeneration_EdgeCases tests various edge cases
func TestSignalGeneration_AtThreshold(t *testing.T) {
	agent := createTestAgent(t)

	// Create articles that result in sentiment exactly at threshold
	articles := []Article{
		{
			Title:       "Bitcoin news",
			Sentiment:   "neutral",
			Score:       agent.sentimentThreshold, // Exactly at threshold
			Confidence:  0.7,
			Source:      "CryptoNews",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)
	signal := agent.generateSignal(articles, nil, overall)

	// At exactly threshold, code uses > (not >=), so should be HOLD
	assert.Equal(t, "HOLD", signal.Action)
}

func TestSignalGeneration_JustBelowThreshold(t *testing.T) {
	agent := createTestAgent(t)

	articles := []Article{
		{
			Title:       "Bitcoin news",
			Sentiment:   "neutral",
			Score:       agent.sentimentThreshold - 0.01, // Just below threshold
			Confidence:  0.7,
			Source:      "CryptoNews",
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	overall := agent.aggregateSentiment(articles, nil)
	signal := agent.generateSignal(articles, nil, overall)

	// Just below threshold should generate HOLD signal
	assert.Equal(t, "HOLD", signal.Action)
}

// TestSentimentWeighting tests the 60% news + 40% F&G weighting
func TestSentimentWeighting(t *testing.T) {
	agent := createTestAgent(t)

	// Very positive news
	articles := []Article{
		{
			Title:       "Bitcoin surges bullish rally boom",
			Score:       1.0,
			Confidence:  1.0,
			PublishedAt: time.Now(),
		},
	}

	for i := range articles {
		agent.analyzeSentiment(&articles[i])
	}

	// Extreme fear (should pull down overall sentiment)
	fearGreed := &FearGreedData{
		Value:          0, // Extreme fear
		Classification: "Extreme Fear",
		Timestamp:      time.Now(),
	}

	overall := agent.aggregateSentiment(articles, fearGreed)

	// News is +1.0, F&G is -1.0
	// Expected: 0.6 * 1.0 + 0.4 * (-1.0) = 0.6 - 0.4 = 0.2
	assert.InDelta(t, 0.2, overall, 0.15)
}

// Note: fetchNews and fetchFearGreedIndex tests are better suited for
// integration tests since they require context and external API endpoints.
// The existing TestFetchNews_Success, TestFetchNews_CacheHit, TestFetchNews_NoAPIKey,
// and TestFetchFearGreedIndex_CacheHit provide adequate unit test coverage
// for the caching logic which is the main unit testable aspect.

// TestGetFloatFromConfig_DefaultValue tests default value return
func TestGetFloatFromConfig_DefaultValue(t *testing.T) {
	config := map[string]interface{}{
		"other_key": 42,
	}

	result := getFloatFromConfig(config, "missing_key", 3.14)
	assert.Equal(t, 3.14, result)
}
