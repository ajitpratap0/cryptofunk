package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/llm"
	"github.com/rs/zerolog/log"
)

// Analyzer performs LLM-powered analysis of Polymarket markets.
type Analyzer struct {
	llmClient    llm.LLMClient
	newsFetcher  *NewsFetcher
	edgeCalc     *EdgeCalculator
	tracker      *PerformanceTracker
	categories   *CategoryManager
}

// AnalyzerConfig holds configuration for the Analyzer.
type AnalyzerConfig struct {
	MinEdge         float64 // minimum edge to recommend a trade (default 0.10)
	MaxKellyFrac    float64 // max fraction of Kelly to use (default 0.25 = quarter Kelly)
	ConfidenceFloor float64 // minimum confidence to trade (default 0.3)
}

// DefaultAnalyzerConfig returns sensible defaults.
func DefaultAnalyzerConfig() AnalyzerConfig {
	return AnalyzerConfig{
		MinEdge:         0.10,
		MaxKellyFrac:    0.25,
		ConfidenceFloor: 0.3,
	}
}

// NewAnalyzer creates a new market analyzer.
func NewAnalyzer(client llm.LLMClient, fetcher *NewsFetcher, tracker *PerformanceTracker, categories *CategoryManager, config AnalyzerConfig) *Analyzer {
	return &Analyzer{
		llmClient:   client,
		newsFetcher: fetcher,
		edgeCalc:    NewEdgeCalculator(config.MinEdge, config.MaxKellyFrac),
		tracker:     tracker,
		categories:  categories,
	}
}

// AnalyzeMarket performs a full analysis of a Polymarket market.
func (a *Analyzer) AnalyzeMarket(ctx context.Context, market MarketInfo) (*TradeRecommendation, error) {
	// Check if category is enabled
	if a.categories != nil && !a.categories.IsEnabled(market.Category) {
		return &TradeRecommendation{
			Market:      market,
			ShouldTrade: false,
			Timestamp:   time.Now(),
		}, nil
	}

	// Fetch relevant news
	var newsItems []NewsItem
	if a.newsFetcher != nil {
		var err error
		newsItems, err = a.newsFetcher.FetchNews(ctx, market.Question, market.Category)
		if err != nil {
			log.Warn().Err(err).Str("market", market.ID).Msg("failed to fetch news, proceeding without")
		}
	}

	// Step 1: Get LLM analysis
	analysis, err := a.getLLMAnalysis(ctx, market, newsItems)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Step 2: Calculate edge
	edge := a.edgeCalc.Calculate(analysis.PredictedProbability, market.YesPrice, analysis.Confidence)

	// Step 3: Apply category weighting
	if a.categories != nil {
		catStats := a.categories.GetStats(market.Category)
		if catStats != nil && catStats.ResolvedCount >= 10 {
			// Weight confidence by historical category performance
			edge.AdjustedSize *= categoryConfidenceWeight(catStats)
		}
	}

	rec := &TradeRecommendation{
		Market:          market,
		Analysis:        *analysis,
		Edge:            *edge,
		ShouldTrade:     edge.MeetsThreshold && analysis.Confidence >= 0.3,
		RecommendedSide: edge.Side,
		RecommendedSize: edge.AdjustedSize,
		ExpectedValue:   edge.ExpectedValue,
		Timestamp:       time.Now(),
	}

	// Step 4: Track prediction
	if a.tracker != nil {
		a.tracker.RecordPrediction(Prediction{
			MarketID:      market.ID,
			MarketQuestion: market.Question,
			Category:      market.Category,
			PredictedProb: analysis.PredictedProbability,
			MarketPrice:   market.YesPrice,
			Confidence:    analysis.Confidence,
			Reasoning:     analysis.Reasoning,
			CreatedAt:     time.Now(),
		})
	}

	return rec, nil
}

func (a *Analyzer) getLLMAnalysis(ctx context.Context, market MarketInfo, news []NewsItem) (*AnalysisResult, error) {
	userPrompt := BuildMarketAnalysisPrompt(market, news)

	content, err := a.llmClient.CompleteWithSystem(ctx, predictionMarketSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	var result AnalysisResult
	if err := a.llmClient.ParseJSONResponse(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Clamp values
	result.PredictedProbability = clamp(result.PredictedProbability, 0.01, 0.99)
	result.Confidence = clamp(result.Confidence, 0, 1)

	return &result, nil
}

// DetectEdge performs a follow-up edge detection analysis.
func (a *Analyzer) DetectEdge(ctx context.Context, market MarketInfo, analysis *AnalysisResult) (*EdgeDetectionResult, error) {
	userPrompt := BuildEdgeDetectionPrompt(market, analysis.PredictedProbability, analysis.Reasoning)

	content, err := a.llmClient.CompleteWithSystem(ctx, edgeDetectionSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("edge detection LLM call failed: %w", err)
	}

	var result EdgeDetectionResult
	if err := a.llmClient.ParseJSONResponse(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse edge detection response: %w", err)
	}

	result.AdjustedProbability = clamp(result.AdjustedProbability, 0.01, 0.99)
	result.EdgeMagnitude = clamp(result.EdgeMagnitude, 0, 1)

	return &result, nil
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func categoryConfidenceWeight(stats *CategoryStats) float64 {
	if stats.BrierScore == 0 {
		return 1.0
	}
	// Good Brier score (< 0.25) → weight up to 1.0
	// Bad Brier score (> 0.25) → weight down
	weight := 1.0 - (stats.BrierScore - 0.15) * 2
	return clamp(weight, 0.1, 1.5)
}
