package strategy

import "math"

// EdgeCalculator computes trading edges and position sizes.
type EdgeCalculator struct {
	minEdge      float64 // minimum absolute edge to trade
	maxKellyFrac float64 // cap on Kelly fraction (e.g. 0.25 for quarter-Kelly)
}

// NewEdgeCalculator creates a new edge calculator.
func NewEdgeCalculator(minEdge, maxKellyFrac float64) *EdgeCalculator {
	if minEdge <= 0 {
		minEdge = 0.10
	}
	if maxKellyFrac <= 0 {
		maxKellyFrac = 0.25
	}
	return &EdgeCalculator{
		minEdge:      minEdge,
		maxKellyFrac: maxKellyFrac,
	}
}

// Calculate computes edge, EV, and Kelly sizing for a market.
// predictedProb is the LLM's estimated true probability (0-1).
// marketPrice is the current YES token price (0-1).
// confidence is the LLM's self-reported confidence (0-1).
func (ec *EdgeCalculator) Calculate(predictedProb, marketPrice, confidence float64) *EdgeResult {
	result := &EdgeResult{
		PredictedProb: predictedProb,
		MarketPrice:   marketPrice,
	}

	edge := predictedProb - marketPrice
	result.Edge = edge

	absEdge := math.Abs(edge)
	result.MeetsThreshold = absEdge >= ec.minEdge

	if edge > 0 {
		// LLM thinks YES is underpriced → buy YES
		result.Side = "YES"
		result.ExpectedValue = (predictedProb * (1.0 / marketPrice)) - 1.0
		result.KellyFraction = kellyFraction(predictedProb, marketPrice)
	} else if edge < 0 {
		// LLM thinks NO is underpriced → buy NO
		noProb := 1.0 - predictedProb
		noPrice := 1.0 - marketPrice
		result.Side = "NO"
		result.ExpectedValue = (noProb * (1.0 / noPrice)) - 1.0
		result.KellyFraction = kellyFraction(noProb, noPrice)
	} else {
		result.Side = "NO_TRADE"
		result.KellyFraction = 0
		result.ExpectedValue = 0
	}

	// Cap Kelly and adjust by confidence
	kelly := math.Min(result.KellyFraction, ec.maxKellyFrac)
	result.AdjustedSize = math.Max(0, kelly*confidence)

	return result
}

// kellyFraction computes the Kelly criterion fraction.
// p = probability of winning, price = cost of the bet.
// Payout on win = 1/price - 1 (you pay `price`, receive 1 if correct).
// Kelly = (p * b - q) / b where b = net odds = (1-price)/price, q = 1-p.
func kellyFraction(p, price float64) float64 {
	if price <= 0 || price >= 1 || p <= 0 || p >= 1 {
		return 0
	}
	b := (1 - price) / price // net odds
	q := 1 - p
	kelly := (p*b - q) / b
	if kelly < 0 {
		return 0
	}
	return kelly
}
