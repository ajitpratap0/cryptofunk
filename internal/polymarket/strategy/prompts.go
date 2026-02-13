package strategy

import "fmt"

const predictionMarketSystemPrompt = `You are an expert prediction market analyst. Your job is to estimate TRUE probabilities of events based on available information. Be calibrated — if you say 70%, it should happen 70% of the time.

You analyze prediction markets by:
1. Evaluating all available evidence objectively
2. Considering base rates and reference classes
3. Updating probabilities based on new information
4. Identifying information the market might be missing or overweighting
5. Being honest about uncertainty — don't pretend to know more than you do

You MUST respond with valid JSON only. No explanatory text outside the JSON.`

const edgeDetectionSystemPrompt = `You are an expert prediction market edge detector. Your job is to find mispricings in prediction markets by comparing your probability estimate to the market price.

Focus on:
1. Information asymmetry — what do you know that traders might not?
2. Recency bias — is the market overreacting to recent news?
3. Anchoring — is the market stuck on an old price?
4. Narrative bias — is the market pricing a story rather than evidence?
5. Low liquidity — thin markets are more likely to be mispriced

You MUST respond with valid JSON only.`

// BuildMarketAnalysisPrompt constructs the user prompt for analyzing a market.
func BuildMarketAnalysisPrompt(market MarketInfo, newsItems []NewsItem) string {
	newsSection := "No recent news available."
	if len(newsItems) > 0 {
		newsSection = "Recent relevant news:\n"
		for i, item := range newsItems {
			if i >= 10 {
				break
			}
			newsSection += fmt.Sprintf("  %d. [%s] %s (Source: %s, Relevance: %.0f%%)\n     %s\n",
				i+1, item.Date, item.Title, item.Source, item.RelevanceScore*100, item.Summary)
		}
	}

	return fmt.Sprintf(`Analyze this prediction market and estimate the TRUE probability of the outcome.

MARKET QUESTION: %s

RESOLUTION CRITERIA: %s

CURRENT PRICES:
  YES: $%.4f (implied probability: %.1f%%)
  NO:  $%.4f (implied probability: %.1f%%)

%s

CATEGORY: %s

Provide your analysis in the following JSON format:
{
  "predicted_probability": 0.0-1.0,
  "confidence": 0.0-1.0,
  "reasoning": "detailed explanation of your probability estimate",
  "key_factors": ["factor1", "factor2", "factor3"],
  "information_gaps": ["what you'd want to know but don't"],
  "market_assessment": "UNDERPRICED" | "OVERPRICED" | "FAIR"
}`,
		market.Question,
		market.ResolutionCriteria,
		market.YesPrice, market.YesPrice*100,
		market.NoPrice, market.NoPrice*100,
		newsSection,
		market.Category,
	)
}

// BuildEdgeDetectionPrompt constructs a prompt specifically for edge detection.
func BuildEdgeDetectionPrompt(market MarketInfo, predictedProb float64, reasoning string) string {
	return fmt.Sprintf(`The market prices this event at %.1f%%. My initial analysis estimates the true probability at %.1f%%.

MARKET: %s

INITIAL REASONING: %s

CURRENT YES PRICE: $%.4f

Do you believe the true probability is meaningfully different from the market price? What information might the market be missing?

Respond in JSON:
{
  "adjusted_probability": 0.0-1.0,
  "edge_exists": true | false,
  "edge_direction": "BUY_YES" | "BUY_NO" | "NO_TRADE",
  "edge_magnitude": 0.0-1.0,
  "reasoning": "why the market is wrong or right",
  "risk_factors": ["risks that could invalidate this edge"],
  "time_sensitivity": "HIGH" | "MEDIUM" | "LOW"
}`,
		market.YesPrice*100,
		predictedProb*100,
		market.Question,
		reasoning,
		market.YesPrice,
	)
}
