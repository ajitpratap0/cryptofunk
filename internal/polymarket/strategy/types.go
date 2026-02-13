package strategy

import "time"

// MarketInfo represents a Polymarket market for analysis.
type MarketInfo struct {
	ID                 string    `json:"id"`
	Question           string    `json:"question"`
	ResolutionCriteria string    `json:"resolution_criteria"`
	YesPrice           float64   `json:"yes_price"` // 0-1
	NoPrice            float64   `json:"no_price"`  // 0-1
	Volume             float64   `json:"volume"`
	Liquidity          float64   `json:"liquidity"`
	Category           string    `json:"category"`
	EndDate            time.Time `json:"end_date"`
}

// AnalysisResult is the parsed output from the LLM market analysis.
type AnalysisResult struct {
	PredictedProbability float64  `json:"predicted_probability"`
	Confidence           float64  `json:"confidence"`
	Reasoning            string   `json:"reasoning"`
	KeyFactors           []string `json:"key_factors"`
	InformationGaps      []string `json:"information_gaps"`
	MarketAssessment     string   `json:"market_assessment"`
}

// EdgeDetectionResult is the parsed output from edge detection.
type EdgeDetectionResult struct {
	AdjustedProbability float64  `json:"adjusted_probability"`
	EdgeExists          bool     `json:"edge_exists"`
	EdgeDirection       string   `json:"edge_direction"` // BUY_YES, BUY_NO, NO_TRADE
	EdgeMagnitude       float64  `json:"edge_magnitude"`
	Reasoning           string   `json:"reasoning"`
	RiskFactors         []string `json:"risk_factors"`
	TimeSensitivity     string   `json:"time_sensitivity"`
}

// TradeRecommendation is the final output combining analysis and edge calculation.
type TradeRecommendation struct {
	Market          MarketInfo           `json:"market"`
	Analysis        AnalysisResult       `json:"analysis"`
	Edge            EdgeResult           `json:"edge"`
	EdgeDetection   *EdgeDetectionResult `json:"edge_detection,omitempty"`
	ShouldTrade     bool                 `json:"should_trade"`
	RecommendedSide string               `json:"recommended_side"` // YES or NO
	RecommendedSize float64              `json:"recommended_size"` // fraction of bankroll
	ExpectedValue   float64              `json:"expected_value"`
	Timestamp       time.Time            `json:"timestamp"`
}

// NewsItem represents a fetched news article relevant to a market.
type NewsItem struct {
	Title          string  `json:"title"`
	Source         string  `json:"source"`
	Date           string  `json:"date"`
	Summary        string  `json:"summary"`
	URL            string  `json:"url"`
	RelevanceScore float64 `json:"relevance_score"` // 0-1
}

// EdgeResult contains the calculated edge and position sizing.
type EdgeResult struct {
	PredictedProb  float64 `json:"predicted_prob"`
	MarketPrice    float64 `json:"market_price"`
	Edge           float64 `json:"edge"` // predicted - market
	ExpectedValue  float64 `json:"expected_value"`
	KellyFraction  float64 `json:"kelly_fraction"`
	AdjustedSize   float64 `json:"adjusted_size"` // kelly * confidence adjustment
	Side           string  `json:"side"`          // YES or NO
	MeetsThreshold bool    `json:"meets_threshold"`
}

// Prediction tracks a prediction for performance measurement.
type Prediction struct {
	ID             string     `json:"id"`
	MarketID       string     `json:"market_id"`
	MarketQuestion string     `json:"market_question"`
	Category       string     `json:"category"`
	PredictedProb  float64    `json:"predicted_prob"`
	MarketPrice    float64    `json:"market_price"`
	Confidence     float64    `json:"confidence"`
	Reasoning      string     `json:"reasoning"`
	Outcome        *float64   `json:"outcome"` // nil if unresolved, 0 or 1
	PnL            float64    `json:"pnl"`
	Resolved       bool       `json:"resolved"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}

// PerformanceStats holds aggregate performance metrics.
type PerformanceStats struct {
	TotalPredictions int                       `json:"total_predictions"`
	ResolvedCount    int                       `json:"resolved_count"`
	Accuracy         float64                   `json:"accuracy"`
	BrierScore       float64                   `json:"brier_score"`
	TotalPnL         float64                   `json:"total_pnl"`
	ByCategory       map[string]*CategoryStats `json:"by_category"`
}

// CategoryStats holds per-category performance.
type CategoryStats struct {
	Category         string  `json:"category"`
	TotalPredictions int     `json:"total_predictions"`
	ResolvedCount    int     `json:"resolved_count"`
	Accuracy         float64 `json:"accuracy"`
	BrierScore       float64 `json:"brier_score"`
	TotalPnL         float64 `json:"total_pnl"`
	Enabled          bool    `json:"enabled"`
}
