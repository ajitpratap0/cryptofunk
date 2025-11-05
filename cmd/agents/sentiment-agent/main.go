// Sentiment Analysis Agent
// Generates trading signals based on news sentiment and Fear & Greed Index
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/ajitpratap0/cryptofunk/internal/agents"
)

// SentimentAgent analyzes news and social sentiment for trading signals
type SentimentAgent struct {
	*agents.BaseAgent

	// NATS connection for signal publishing
	natsConn  *nats.Conn
	natsTopic string

	// HTTP client for API calls
	httpClient *http.Client

	// Configuration
	symbol             string
	newsAPIKey         string
	analysisInterval   time.Duration
	lookbackHours      int
	sentimentThreshold float64
	includeFearGreed   bool
	newsSourceWeights  map[string]float64

	// Cached data (with TTL)
	cachedNews      []Article
	cachedFearGreed *FearGreedData
	cacheExpiry     time.Time
	cacheMutex      sync.RWMutex

	// BDI belief system
	beliefs *BeliefBase
}

// Article represents a news article
type Article struct {
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Sentiment   string    `json:"sentiment"`  // positive, negative, neutral
	Confidence  float64   `json:"confidence"` // 0.0 to 1.0
	Score       float64   `json:"score"`      // -1.0 to 1.0 (negative to positive)
}

// FearGreedData represents the Crypto Fear & Greed Index
type FearGreedData struct {
	Value          int       `json:"value"`                // 0-100
	Classification string    `json:"value_classification"` // Extreme Fear, Fear, Neutral, Greed, Extreme Greed
	Timestamp      time.Time `json:"timestamp"`
	UpdateTime     string    `json:"time_until_update"`
}

// SentimentSignal represents a trading signal based on sentiment analysis
type SentimentSignal struct {
	Timestamp        time.Time `json:"timestamp"`
	Symbol           string    `json:"symbol"`
	Action           string    `json:"action"`            // BUY, SELL, HOLD
	Confidence       float64   `json:"confidence"`        // 0.0 to 1.0
	NewsSentiment    float64   `json:"news_sentiment"`    // -1 to 1
	FearGreedIndex   int       `json:"fear_greed_index"`  // 0-100
	OverallSentiment float64   `json:"overall_sentiment"` // -1 to 1
	ArticlesAnalyzed int       `json:"articles_analyzed"`
	Reasoning        string    `json:"reasoning"`
	ArticlesSources  []string  `json:"articles_sources"`
}

// Belief represents a single belief in the agent's belief base
type Belief struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Confidence float64     `json:"confidence"`
	Timestamp  time.Time   `json:"timestamp"`
	Source     string      `json:"source"`
}

// BeliefBase represents the agent's beliefs about sentiment
type BeliefBase struct {
	beliefs map[string]*Belief
	mutex   sync.RWMutex
}

// NewBeliefBase creates a new belief base
func NewBeliefBase() *BeliefBase {
	return &BeliefBase{
		beliefs: make(map[string]*Belief),
	}
}

// UpdateBelief updates or creates a belief
func (bb *BeliefBase) UpdateBelief(key string, value interface{}, confidence float64, source string) {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	bb.beliefs[key] = &Belief{
		Key:        key,
		Value:      value,
		Confidence: confidence,
		Timestamp:  time.Now(),
		Source:     source,
	}
}

// GetBelief retrieves a belief by key
func (bb *BeliefBase) GetBelief(key string) (*Belief, bool) {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	belief, exists := bb.beliefs[key]
	return belief, exists
}

// GetAllBeliefs returns a copy of all beliefs
func (bb *BeliefBase) GetAllBeliefs() map[string]*Belief {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	beliefs := make(map[string]*Belief, len(bb.beliefs))
	for k, v := range bb.beliefs {
		beliefs[k] = v
	}
	return beliefs
}

// GetConfidence returns overall confidence (average of all beliefs)
func (bb *BeliefBase) GetConfidence() float64 {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	if len(bb.beliefs) == 0 {
		return 0.0
	}

	var total float64
	for _, belief := range bb.beliefs {
		total += belief.Confidence
	}
	return total / float64(len(bb.beliefs))
}

// Positive and negative keywords for sentiment analysis
var (
	positiveKeywords = []string{
		"bullish", "surge", "rally", "gain", "rise", "breakthrough",
		"moon", "pump", "green", "up", "soar", "boom", "high",
		"growth", "increase", "profit", "record", "all-time-high",
		"ath", "adoption", "upgrade", "partnership", "integration",
	}

	negativeKeywords = []string{
		"bearish", "crash", "drop", "fall", "decline", "concern",
		"dump", "red", "down", "plunge", "collapse", "fear",
		"loss", "decrease", "risk", "warning", "ban", "hack",
		"scam", "fraud", "regulation", "investigation", "lawsuit",
	}
)

// NewSentimentAgent creates a new sentiment analysis agent
func NewSentimentAgent(config *agents.AgentConfig, log zerolog.Logger, metricsPort int) (*SentimentAgent, error) {
	baseAgent := agents.NewBaseAgent(config, log, metricsPort)

	// Extract configuration
	agentConfig := config.Config

	// Read NATS configuration
	natsURL := viper.GetString("nats.url")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	natsTopic := viper.GetString("communication.nats.topics.sentiment_signals")
	if natsTopic == "" {
		natsTopic = "agents.analysis.sentiment"
	}

	// Connect to NATS
	log.Info().Str("url", natsURL).Str("topic", natsTopic).Msg("Connecting to NATS")
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info().Msg("Successfully connected to NATS")

	// Extract sentiment-specific config
	symbol := getStringFromConfig(agentConfig, "symbol", "bitcoin")
	newsAPIKey := os.Getenv("CRYPTOPANIC_API_KEY")
	if newsAPIKey == "" {
		log.Warn().Msg("CRYPTOPANIC_API_KEY not set, news analysis will be limited")
	}

	analysisIntervalStr := getStringFromConfig(agentConfig, "analysis_interval", "300s")
	analysisInterval, err := time.ParseDuration(analysisIntervalStr)
	if err != nil {
		log.Warn().Err(err).Str("interval", analysisIntervalStr).Msg("Invalid analysis_interval, using default 5m")
		analysisInterval = 5 * time.Minute
	}

	lookbackHours := getIntFromConfig(agentConfig, "lookback_hours", 24)
	sentimentThreshold := getFloatFromConfig(agentConfig, "sentiment_threshold", 0.3)
	includeFearGreed := getBoolFromConfig(agentConfig, "include_fear_greed", true)

	// Parse news source weights
	newsSourceWeights := make(map[string]float64)
	if sources, ok := agentConfig["news_sources"].([]interface{}); ok {
		for _, src := range sources {
			if srcMap, ok := src.(map[string]interface{}); ok {
				name := getStringFromConfig(srcMap, "name", "")
				weight := getFloatFromConfig(srcMap, "weight", 1.0)
				if name != "" {
					newsSourceWeights[name] = weight
				}
			}
		}
	}
	if len(newsSourceWeights) == 0 {
		newsSourceWeights["cryptopanic"] = 1.0
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &SentimentAgent{
		BaseAgent:          baseAgent,
		natsConn:           nc,
		natsTopic:          natsTopic,
		httpClient:         httpClient,
		symbol:             symbol,
		newsAPIKey:         newsAPIKey,
		analysisInterval:   analysisInterval,
		lookbackHours:      lookbackHours,
		sentimentThreshold: sentimentThreshold,
		includeFearGreed:   includeFearGreed,
		newsSourceWeights:  newsSourceWeights,
		beliefs:            NewBeliefBase(),
	}, nil
}

// Step performs a single sentiment analysis cycle
func (a *SentimentAgent) Step(ctx context.Context) error {
	// Call parent Step to handle metrics
	if err := a.BaseAgent.Step(ctx); err != nil {
		return err
	}

	log.Debug().Msg("Executing sentiment analysis step")

	// Step 1: Fetch news articles
	articles, err := a.fetchNews(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch news")
		// Don't fail - continue with cached data if available
		if len(a.cachedNews) == 0 {
			return fmt.Errorf("no news data available: %w", err)
		}
		articles = a.cachedNews
	}

	log.Info().Int("count", len(articles)).Msg("Fetched news articles")

	// Step 2: Fetch Fear & Greed Index (if enabled)
	var fearGreed *FearGreedData
	if a.includeFearGreed {
		fearGreed, err = a.fetchFearGreedIndex(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to fetch Fear & Greed Index")
			// Use cached data if available
			fearGreed = a.cachedFearGreed
		}
	}

	// Step 3: Analyze sentiment of articles
	for i := range articles {
		a.analyzeSentiment(&articles[i])
	}

	// Step 4: Aggregate sentiment from all sources
	overallSentiment := a.aggregateSentiment(articles, fearGreed)

	// Step 5: Update beliefs
	a.updateBeliefs(articles, fearGreed, overallSentiment)

	// Step 6: Generate trading signal
	signal := a.generateSignal(articles, fearGreed, overallSentiment)

	log.Info().
		Str("action", signal.Action).
		Float64("confidence", signal.Confidence).
		Float64("sentiment", signal.OverallSentiment).
		Str("reasoning", signal.Reasoning).
		Msg("Sentiment signal generated")

	// Step 7: Publish signal to NATS
	if err := a.publishSignal(ctx, signal); err != nil {
		log.Error().Err(err).Msg("Failed to publish signal to NATS")
	}

	return nil
}

// fetchNews fetches recent news articles from CryptoPanic
func (a *SentimentAgent) fetchNews(ctx context.Context) ([]Article, error) {
	// Check cache first (5 minute TTL)
	a.cacheMutex.RLock()
	if time.Now().Before(a.cacheExpiry) && len(a.cachedNews) > 0 {
		articles := a.cachedNews
		a.cacheMutex.RUnlock()
		log.Debug().Int("count", len(articles)).Msg("Using cached news articles")
		return articles, nil
	}
	a.cacheMutex.RUnlock()

	if a.newsAPIKey == "" {
		return nil, fmt.Errorf("CryptoPanic API key not configured")
	}

	// CryptoPanic API endpoint
	url := fmt.Sprintf("https://cryptopanic.com/api/v1/posts/?auth_token=%s&currencies=%s&kind=news",
		a.newsAPIKey, a.symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Error().Err(cerr).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse CryptoPanic response
	var result struct {
		Count   int `json:"count"`
		Results []struct {
			Title  string `json:"title"`
			URL    string `json:"url"`
			Source struct {
				Title string `json:"title"`
			} `json:"source"`
			PublishedAt string `json:"published_at"`
			Votes       struct {
				Positive int `json:"positive"`
				Negative int `json:"negative"`
				Liked    int `json:"liked"`
			} `json:"votes"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to Article structs
	articles := make([]Article, 0, len(result.Results))
	cutoffTime := time.Now().Add(-time.Duration(a.lookbackHours) * time.Hour)

	for _, item := range result.Results {
		publishedAt, err := time.Parse(time.RFC3339, item.PublishedAt)
		if err != nil {
			log.Warn().Err(err).Str("timestamp", item.PublishedAt).Msg("Failed to parse publish time")
			publishedAt = time.Now()
		}

		// Filter by lookback window
		if publishedAt.Before(cutoffTime) {
			continue
		}

		article := Article{
			Title:       item.Title,
			Source:      item.Source.Title,
			URL:         item.URL,
			PublishedAt: publishedAt,
		}

		articles = append(articles, article)
	}

	// Update cache
	a.cacheMutex.Lock()
	a.cachedNews = articles
	a.cacheExpiry = time.Now().Add(5 * time.Minute)
	a.cacheMutex.Unlock()

	log.Debug().Int("count", len(articles)).Msg("Fetched articles from CryptoPanic")
	return articles, nil
}

// fetchFearGreedIndex fetches the Crypto Fear & Greed Index
func (a *SentimentAgent) fetchFearGreedIndex(ctx context.Context) (*FearGreedData, error) {
	// Check cache (1 hour TTL for F&G index)
	a.cacheMutex.RLock()
	if a.cachedFearGreed != nil && time.Since(a.cachedFearGreed.Timestamp) < time.Hour {
		fg := a.cachedFearGreed
		a.cacheMutex.RUnlock()
		log.Debug().Int("value", fg.Value).Str("classification", fg.Classification).Msg("Using cached Fear & Greed Index")
		return fg, nil
	}
	a.cacheMutex.RUnlock()

	// Alternative.me API endpoint
	url := "https://api.alternative.me/fng/?limit=1"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Error().Err(cerr).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Data []struct {
			Value           string `json:"value"`
			Classification  string `json:"value_classification"`
			Timestamp       string `json:"timestamp"`
			TimeUntilUpdate string `json:"time_until_update"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no Fear & Greed data available")
	}

	// Parse value
	var value int
	if _, err := fmt.Sscanf(result.Data[0].Value, "%d", &value); err != nil {
		return nil, fmt.Errorf("failed to parse fear & greed value: %w", err)
	}

	fearGreed := &FearGreedData{
		Value:          value,
		Classification: result.Data[0].Classification,
		Timestamp:      time.Now(),
		UpdateTime:     result.Data[0].TimeUntilUpdate,
	}

	// Update cache
	a.cacheMutex.Lock()
	a.cachedFearGreed = fearGreed
	a.cacheMutex.Unlock()

	log.Debug().Int("value", value).Str("classification", fearGreed.Classification).Msg("Fetched Fear & Greed Index")
	return fearGreed, nil
}

// analyzeSentiment analyzes sentiment of a single article using keyword matching
func (a *SentimentAgent) analyzeSentiment(article *Article) {
	// Convert title to lowercase for matching
	titleLower := strings.ToLower(article.Title)

	// Count positive and negative keywords
	positiveCount := 0
	negativeCount := 0
	totalWords := len(strings.Fields(article.Title))

	for _, keyword := range positiveKeywords {
		if strings.Contains(titleLower, keyword) {
			positiveCount++
		}
	}

	for _, keyword := range negativeKeywords {
		if strings.Contains(titleLower, keyword) {
			negativeCount++
		}
	}

	// Calculate sentiment score (-1 to 1)
	if totalWords == 0 {
		article.Score = 0
		article.Sentiment = "neutral"
		article.Confidence = 0.3
		return
	}

	// Score based on keyword balance
	keywordDiff := positiveCount - negativeCount
	article.Score = float64(keywordDiff) / float64(totalWords)

	// Clamp to -1 to 1 range
	if article.Score > 1.0 {
		article.Score = 1.0
	} else if article.Score < -1.0 {
		article.Score = -1.0
	}

	// Classify sentiment
	if article.Score > 0.1 {
		article.Sentiment = "positive"
		article.Confidence = math.Min(math.Abs(article.Score)*2, 1.0)
	} else if article.Score < -0.1 {
		article.Sentiment = "negative"
		article.Confidence = math.Min(math.Abs(article.Score)*2, 1.0)
	} else {
		article.Sentiment = "neutral"
		article.Confidence = 0.5
	}

	log.Debug().
		Str("title", article.Title).
		Str("sentiment", article.Sentiment).
		Float64("score", article.Score).
		Float64("confidence", article.Confidence).
		Msg("Analyzed article sentiment")
}

// aggregateSentiment combines news sentiment and Fear & Greed Index
func (a *SentimentAgent) aggregateSentiment(articles []Article, fearGreed *FearGreedData) float64 {
	if len(articles) == 0 && fearGreed == nil {
		return 0.0
	}

	// Calculate weighted news sentiment
	var newsSentiment float64
	var totalWeight float64

	for _, article := range articles {
		// Weight by recency (exponential decay)
		hoursOld := time.Since(article.PublishedAt).Hours()
		recencyWeight := math.Exp(-hoursOld / 24.0) // Half-life of 24 hours

		// Weight by source credibility
		sourceWeight := a.newsSourceWeights["cryptopanic"]
		if sw, ok := a.newsSourceWeights[strings.ToLower(article.Source)]; ok {
			sourceWeight = sw
		}

		// Weight by article confidence
		weight := recencyWeight * sourceWeight * article.Confidence

		newsSentiment += article.Score * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		newsSentiment /= totalWeight
	}

	// If no Fear & Greed Index, return news sentiment only
	if fearGreed == nil || !a.includeFearGreed {
		return newsSentiment
	}

	// Normalize Fear & Greed Index (0-100 to -1 to 1)
	// 0 = Extreme Fear (-1), 50 = Neutral (0), 100 = Extreme Greed (1)
	fearGreedNorm := (float64(fearGreed.Value) - 50.0) / 50.0

	// Combine with configured weights (news: 60%, F&G: 40%)
	newsWeight := 0.60
	fearGreedWeight := 0.40

	overall := (newsSentiment * newsWeight) + (fearGreedNorm * fearGreedWeight)

	log.Debug().
		Float64("news_sentiment", newsSentiment).
		Float64("fear_greed_norm", fearGreedNorm).
		Float64("overall", overall).
		Msg("Aggregated sentiment")

	return overall
}

// updateBeliefs updates the agent's beliefs based on sentiment analysis
func (a *SentimentAgent) updateBeliefs(articles []Article, fearGreed *FearGreedData, overall float64) {
	log.Debug().Msg("Updating agent beliefs from sentiment analysis")

	// Update news sentiment belief
	if len(articles) > 0 {
		positiveCount := 0
		negativeCount := 0
		for _, article := range articles {
			if article.Sentiment == "positive" {
				positiveCount++
			} else if article.Sentiment == "negative" {
				negativeCount++
			}
		}

		var dominantSentiment string
		var confidence float64
		if positiveCount > negativeCount {
			dominantSentiment = "positive"
			confidence = float64(positiveCount) / float64(len(articles))
		} else if negativeCount > positiveCount {
			dominantSentiment = "negative"
			confidence = float64(negativeCount) / float64(len(articles))
		} else {
			dominantSentiment = "neutral"
			confidence = 0.5
		}

		a.beliefs.UpdateBelief("news_sentiment", dominantSentiment, confidence, "news_analysis")
	}

	// Update Fear & Greed belief
	if fearGreed != nil {
		a.beliefs.UpdateBelief("fear_greed_index", fearGreed.Value, 1.0, "fear_greed_api")
		a.beliefs.UpdateBelief("market_emotion", fearGreed.Classification, 1.0, "fear_greed_api")
	}

	// Update overall sentiment belief
	var sentimentClass string
	if overall > 0.3 {
		sentimentClass = "bullish"
	} else if overall < -0.3 {
		sentimentClass = "bearish"
	} else {
		sentimentClass = "neutral"
	}

	confidence := math.Min(math.Abs(overall), 1.0)
	a.beliefs.UpdateBelief("overall_sentiment", sentimentClass, confidence, "aggregated")

	log.Debug().
		Str("sentiment", sentimentClass).
		Float64("confidence", confidence).
		Float64("overall_confidence", a.beliefs.GetConfidence()).
		Int("belief_count", len(a.beliefs.GetAllBeliefs())).
		Msg("Beliefs updated successfully")
}

// generateSignal converts sentiment to trading signal
func (a *SentimentAgent) generateSignal(articles []Article, fearGreed *FearGreedData, overall float64) *SentimentSignal {
	// Determine action based on sentiment threshold
	var action string
	var confidence float64

	if overall > a.sentimentThreshold {
		action = "BUY"
		// Confidence increases with distance from threshold
		confidence = math.Min((overall-a.sentimentThreshold)/(1.0-a.sentimentThreshold), 1.0)
	} else if overall < -a.sentimentThreshold {
		action = "SELL"
		confidence = math.Min((math.Abs(overall)-a.sentimentThreshold)/(1.0-a.sentimentThreshold), 1.0)
	} else {
		action = "HOLD"
		// Confidence in HOLD is inverse of distance from thresholds
		distanceFromThreshold := math.Min(
			math.Abs(overall-a.sentimentThreshold),
			math.Abs(overall+a.sentimentThreshold),
		)
		confidence = 1.0 - (distanceFromThreshold / a.sentimentThreshold)
	}

	// Adjust confidence based on number of articles and agreement
	if len(articles) > 0 {
		// More articles = higher confidence (up to 20 articles)
		articleFactor := math.Min(float64(len(articles))/20.0, 1.0)
		confidence *= (0.7 + 0.3*articleFactor)

		// Agreement factor: how much do articles agree?
		positiveCount := 0
		negativeCount := 0
		for _, article := range articles {
			if article.Sentiment == "positive" {
				positiveCount++
			} else if article.Sentiment == "negative" {
				negativeCount++
			}
		}
		agreement := float64(max(positiveCount, negativeCount)) / float64(len(articles))
		confidence *= agreement
	}

	// Build reasoning
	var reasoningParts []string

	// News sentiment reasoning
	if len(articles) > 0 {
		positiveCount := 0
		negativeCount := 0
		for _, article := range articles {
			if article.Sentiment == "positive" {
				positiveCount++
			} else if article.Sentiment == "negative" {
				negativeCount++
			}
		}

		newsScore := 0.0
		for _, article := range articles {
			newsScore += article.Score
		}
		newsScore /= float64(len(articles))

		reasoningParts = append(reasoningParts,
			fmt.Sprintf("News sentiment: %d positive, %d negative out of %d articles (score: %.2f)",
				positiveCount, negativeCount, len(articles), newsScore))
	}

	// Fear & Greed reasoning
	var fearGreedValue int
	if fearGreed != nil {
		fearGreedValue = fearGreed.Value
		reasoningParts = append(reasoningParts,
			fmt.Sprintf("Fear & Greed Index: %d (%s)", fearGreed.Value, fearGreed.Classification))
	}

	// Overall sentiment reasoning
	reasoningParts = append(reasoningParts,
		fmt.Sprintf("Overall sentiment: %.2f → %s", overall, action))

	reasoning := strings.Join(reasoningParts, "; ")

	// Collect article sources
	sources := make([]string, 0, len(articles))
	seenSources := make(map[string]bool)
	for _, article := range articles {
		if !seenSources[article.Source] {
			sources = append(sources, article.Source)
			seenSources[article.Source] = true
		}
	}

	return &SentimentSignal{
		Timestamp:        time.Now(),
		Symbol:           a.symbol,
		Action:           action,
		Confidence:       confidence,
		NewsSentiment:    overall,
		FearGreedIndex:   fearGreedValue,
		OverallSentiment: overall,
		ArticlesAnalyzed: len(articles),
		Reasoning:        reasoning,
		ArticlesSources:  sources,
	}
}

// publishSignal publishes a sentiment signal to NATS
func (a *SentimentAgent) publishSignal(ctx context.Context, signal *SentimentSignal) error {
	// Marshal signal to JSON
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	// Publish to NATS
	if err := a.natsConn.Publish(a.natsTopic, data); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	log.Debug().
		Str("topic", a.natsTopic).
		Str("symbol", signal.Symbol).
		Str("action", signal.Action).
		Float64("confidence", signal.Confidence).
		Msg("Signal published to NATS")

	return nil
}

// Configuration helper functions

func getStringFromConfig(config map[string]interface{}, key string, defaultVal string) string {
	if val, ok := config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultVal
}

func getIntFromConfig(config map[string]interface{}, key string, defaultVal int) int {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

func getFloatFromConfig(config map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

func getBoolFromConfig(config map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := config[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	// Configure logging to stderr (stdout reserved for MCP protocol)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	viper.SetConfigName("agents")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../../../configs") // From cmd/agents/sentiment-agent/

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	// Extract sentiment agent configuration
	var agentConfig agents.AgentConfig

	// Get sentiment agent config from analysis_agents.sentiment
	sentimentConfig := viper.Sub("analysis_agents.sentiment")
	if sentimentConfig == nil {
		log.Fatal().Msg("Sentiment agent configuration not found in agents.yaml")
	}

	agentConfig.Name = sentimentConfig.GetString("name")
	agentConfig.Type = sentimentConfig.GetString("type")
	agentConfig.Version = sentimentConfig.GetString("version")
	agentConfig.Enabled = sentimentConfig.GetBool("enabled")

	// Parse step interval
	stepIntervalStr := sentimentConfig.GetString("step_interval")
	stepInterval, err := time.ParseDuration(stepIntervalStr)
	if err != nil {
		log.Fatal().Err(err).Str("interval", stepIntervalStr).Msg("Invalid step_interval")
	}
	agentConfig.StepInterval = stepInterval

	// Get agent-specific config
	agentConfig.Config = sentimentConfig.Get("config").(map[string]interface{})

	// Get metrics port from global config
	metricsPort := viper.GetInt("global.metrics_port")
	if metricsPort == 0 {
		metricsPort = 9103 // Default port for sentiment agent
	}

	// Get MCP server configurations (if any)
	mcpServers := sentimentConfig.Get("mcp_servers")
	if mcpServers != nil {
		log.Debug().Interface("mcp_servers", mcpServers).Msg("MCP servers configured")

		// Parse MCP server list into MCPServerConfig structs
		if servers, ok := mcpServers.([]interface{}); ok {
			agentConfig.MCPServers = make([]agents.MCPServerConfig, 0, len(servers))
			for _, srv := range servers {
				if server, ok := srv.(map[string]interface{}); ok {
					serverConfig := agents.MCPServerConfig{
						Name: server["name"].(string),
						Type: server["type"].(string),
					}

					// Set fields based on server type
					if serverConfig.Type == "internal" {
						if cmd, ok := server["command"].(string); ok {
							serverConfig.Command = cmd
						}
						if args, ok := server["args"].([]interface{}); ok {
							serverConfig.Args = make([]string, len(args))
							for i, arg := range args {
								serverConfig.Args[i] = arg.(string)
							}
						}
						if env, ok := server["env"].(map[string]interface{}); ok {
							serverConfig.Env = make(map[string]string, len(env))
							for k, v := range env {
								serverConfig.Env[k] = v.(string)
							}
						}
					} else if serverConfig.Type == "external" {
						if url, ok := server["url"].(string); ok {
							serverConfig.URL = url
						}
					}

					agentConfig.MCPServers = append(agentConfig.MCPServers, serverConfig)
					log.Info().
						Str("name", serverConfig.Name).
						Str("type", serverConfig.Type).
						Msg("Configured MCP server")
				}
			}
		}
	}

	// Create agent
	agent, err := NewSentimentAgent(&agentConfig, log.Logger, metricsPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create sentiment agent")
	}

	log.Info().
		Str("name", agentConfig.Name).
		Str("type", agentConfig.Type).
		Str("version", agentConfig.Version).
		Dur("step_interval", agentConfig.StepInterval).
		Int("metrics_port", metricsPort).
		Msg("Starting sentiment analysis agent")

	// Initialize agent
	ctx := context.Background()
	if err := agent.Initialize(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize agent")
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run agent in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Run(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case err := <-errChan:
		if err != nil {
			log.Error().Err(err).Msg("Agent run error")
		}
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := agent.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Sentiment analysis agent shutdown complete")
}
