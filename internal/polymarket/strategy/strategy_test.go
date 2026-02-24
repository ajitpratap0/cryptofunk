package strategy

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/llm"
)

// --- Mock LLM Client ---

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(_ context.Context, _ []llm.ChatMessage) (*llm.ChatResponse, error) {
	return nil, nil
}
func (m *mockLLMClient) CompleteWithRetry(_ context.Context, _ []llm.ChatMessage, _ int) (*llm.ChatResponse, error) {
	return nil, nil
}
func (m *mockLLMClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}
func (m *mockLLMClient) ParseJSONResponse(content string, target interface{}) error {
	return json.Unmarshal([]byte(content), target)
}

// --- Edge Calculation Tests ---

func TestEdgeCalculator_BasicEdge(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.25)

	// LLM says 70%, market says 50% → buy YES, edge = 0.20
	r := ec.Calculate(0.70, 0.50, 0.8)
	if r.Side != SideYes {
		t.Errorf("expected YES, got %s", r.Side)
	}
	if !r.MeetsThreshold {
		t.Error("expected edge to meet threshold")
	}
	if math.Abs(r.Edge-0.20) > 0.001 {
		t.Errorf("expected edge 0.20, got %.4f", r.Edge)
	}
}

func TestEdgeCalculator_BuyNO(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.25)

	// LLM says 30%, market says 60% → buy NO
	r := ec.Calculate(0.30, 0.60, 0.9)
	if r.Side != SideNo {
		t.Errorf("expected NO, got %s", r.Side)
	}
	if !r.MeetsThreshold {
		t.Error("expected edge to meet threshold")
	}
}

func TestEdgeCalculator_NoEdge(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.25)

	// LLM says 52%, market says 50% → edge too small
	r := ec.Calculate(0.52, 0.50, 0.9)
	if r.MeetsThreshold {
		t.Error("expected edge NOT to meet threshold")
	}
}

func TestKellyFraction(t *testing.T) {
	// p=0.7, price=0.5 → b=1.0, kelly = (0.7*1 - 0.3)/1 = 0.4
	k := kellyFraction(0.7, 0.5)
	if math.Abs(k-0.4) > 0.001 {
		t.Errorf("expected kelly 0.4, got %.4f", k)
	}

	// No edge: p=0.5, price=0.5 → kelly=0
	k = kellyFraction(0.5, 0.5)
	if math.Abs(k) > 0.001 {
		t.Errorf("expected kelly ~0, got %.4f", k)
	}

	// Negative edge: p=0.3, price=0.5 → kelly should be 0 (clamped)
	k = kellyFraction(0.3, 0.5)
	if k != 0 {
		t.Errorf("expected kelly 0, got %.4f", k)
	}
}

func TestEdgeCalculator_AdjustedSize(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.25)

	r := ec.Calculate(0.80, 0.50, 1.0)
	// Kelly for p=0.8, price=0.5: b=1, kelly = (0.8-0.2)/1 = 0.6
	// Capped at 0.25, * confidence 1.0 = 0.25
	if math.Abs(r.AdjustedSize-0.25) > 0.001 {
		t.Errorf("expected adjusted size 0.25, got %.4f", r.AdjustedSize)
	}

	// With low confidence
	r = ec.Calculate(0.80, 0.50, 0.5)
	if math.Abs(r.AdjustedSize-0.125) > 0.001 {
		t.Errorf("expected adjusted size 0.125, got %.4f", r.AdjustedSize)
	}
}

// --- Prompt Tests ---

func TestBuildMarketAnalysisPrompt(t *testing.T) {
	market := MarketInfo{
		Question:           "Will Bitcoin reach $100k by end of 2025?",
		ResolutionCriteria: "BTC/USD spot price on any major exchange",
		YesPrice:           0.35,
		NoPrice:            0.65,
		Category:           "crypto",
	}

	prompt := BuildMarketAnalysisPrompt(market, nil)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !contains(prompt, "Will Bitcoin reach $100k") {
		t.Error("prompt should contain market question")
	}
	if !contains(prompt, "35.0%") {
		t.Error("prompt should contain implied probability")
	}
}

func TestBuildMarketAnalysisPromptWithNews(t *testing.T) {
	market := MarketInfo{
		Question:           "Will X happen?",
		ResolutionCriteria: "criteria",
		YesPrice:           0.50,
		NoPrice:            0.50,
		Category:           "tech",
	}

	news := []NewsItem{
		{Title: "Breaking: X is likely", Source: "Reuters", Date: "2025-01-01", Summary: "X seems likely", RelevanceScore: 0.9},
	}

	prompt := BuildMarketAnalysisPrompt(market, news)
	if !contains(prompt, "Breaking: X is likely") {
		t.Error("prompt should contain news")
	}
	if !contains(prompt, "Reuters") {
		t.Error("prompt should contain news source")
	}
}

// --- Tracker Tests ---

func TestPerformanceTracker_RecordAndResolve(t *testing.T) {
	pt := NewPerformanceTracker()

	id := pt.RecordPrediction(Prediction{
		MarketID:      "m1",
		Category:      "crypto",
		PredictedProb: 0.8,
		MarketPrice:   0.5,
		Confidence:    0.9,
	})

	if id == "" {
		t.Fatal("should return an ID")
	}

	ok := pt.ResolvePrediction(id, 1.0)
	if !ok {
		t.Fatal("should find and resolve prediction")
	}

	stats := pt.GetStats()
	if stats.TotalPredictions != 1 {
		t.Errorf("expected 1 prediction, got %d", stats.TotalPredictions)
	}
	if stats.ResolvedCount != 1 {
		t.Errorf("expected 1 resolved, got %d", stats.ResolvedCount)
	}
	if stats.Accuracy != 1.0 {
		t.Errorf("expected accuracy 1.0, got %.2f", stats.Accuracy)
	}
}

func TestPerformanceTracker_BrierScore(t *testing.T) {
	pt := NewPerformanceTracker()

	// Perfect prediction: predicted 0.9, outcome 1.0 → Brier = (0.9-1)^2 = 0.01
	id := pt.RecordPrediction(Prediction{
		MarketID:      "m1",
		Category:      "crypto",
		PredictedProb: 0.9,
		MarketPrice:   0.5,
	})
	pt.ResolvePrediction(id, 1.0)

	stats := pt.GetStats()
	if math.Abs(stats.BrierScore-0.01) > 0.001 {
		t.Errorf("expected Brier 0.01, got %.4f", stats.BrierScore)
	}
}

// --- Category Tests ---

func TestCategoryManager_IsEnabled(t *testing.T) {
	cm := NewCategoryManager(nil)
	if !cm.IsEnabled("crypto") {
		t.Error("crypto should be enabled by default")
	}
	cm.SetEnabled("crypto", false)
	if cm.IsEnabled("crypto") {
		t.Error("crypto should be disabled")
	}
}

func TestCategoryManager_AutoDisable(t *testing.T) {
	pt := NewPerformanceTracker()
	cm := NewCategoryManager(pt)

	// Add 15 bad predictions for sports
	for i := 0; i < 15; i++ {
		id := pt.RecordPrediction(Prediction{
			MarketID:      "m",
			Category:      "sports",
			PredictedProb: 0.9, // predicted high
			MarketPrice:   0.5,
		})
		pt.ResolvePrediction(id, 0.0) // but outcome was 0 → Brier = 0.81
	}

	cm.RefreshEnabled()
	if cm.IsEnabled("sports") {
		t.Error("sports should be auto-disabled with terrible Brier score")
	}
}

// --- Analyzer Integration Test (with mock LLM) ---

func TestAnalyzer_AnalyzeMarket(t *testing.T) {
	mockResp := `{
		"predicted_probability": 0.75,
		"confidence": 0.8,
		"reasoning": "Strong evidence supports YES",
		"key_factors": ["factor1", "factor2"],
		"information_gaps": ["gap1"],
		"market_assessment": "UNDERPRICED"
	}`

	client := &mockLLMClient{response: mockResp}
	tracker := NewPerformanceTracker()
	categories := NewCategoryManager(tracker)
	config := DefaultAnalyzerConfig()

	analyzer := NewAnalyzer(client, nil, tracker, categories, config)

	market := MarketInfo{
		ID:                 "test-market",
		Question:           "Will BTC hit 100k?",
		ResolutionCriteria: "spot price",
		YesPrice:           0.50,
		NoPrice:            0.50,
		Category:           "crypto",
		EndDate:            time.Now().Add(30 * 24 * time.Hour),
	}

	rec, err := analyzer.AnalyzeMarket(context.Background(), market)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !rec.ShouldTrade {
		t.Error("should recommend trading with 25% edge")
	}
	if rec.RecommendedSide != SideYes {
		t.Errorf("expected YES, got %s", rec.RecommendedSide)
	}
	if rec.Analysis.PredictedProbability != 0.75 {
		t.Errorf("expected prob 0.75, got %.2f", rec.Analysis.PredictedProbability)
	}

	// Check prediction was tracked
	preds := tracker.GetPredictions()
	if len(preds) != 1 {
		t.Errorf("expected 1 tracked prediction, got %d", len(preds))
	}
}

// --- News Tests ---

func TestExtractSearchTerms(t *testing.T) {
	q := "Will the United States approve a Bitcoin ETF before 2026?"
	terms := extractSearchTerms(q)
	if terms == "" {
		t.Fatal("should extract terms")
	}
	if contains(terms, "will") || contains(terms, "the") {
		t.Error("should filter stop words")
	}
	if !contains(terms, "united") || !contains(terms, "bitcoin") {
		t.Error("should keep meaningful words")
	}
}

func TestScoreRelevance(t *testing.T) {
	item := NewsItem{Title: "Bitcoin ETF approval likely says SEC"}
	score := scoreRelevance(item, "Will Bitcoin ETF be approved?")
	if score < 0.3 {
		t.Errorf("expected high relevance, got %.2f", score)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
