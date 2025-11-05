//nolint:goconst // Trading signals are domain-specific strings
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// ContextBuilder builds rich context for LLM prompts with token limiting
type ContextBuilder struct {
	tracker        *DecisionTracker
	maxTokens      int // Maximum tokens for context (default 4000)
	agentName      string
	includeHistory bool // Include past decisions
}

// ContextBuilderConfig configures the context builder
type ContextBuilderConfig struct {
	MaxTokens      int
	AgentName      string
	IncludeHistory bool
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(tracker *DecisionTracker, config ContextBuilderConfig) *ContextBuilder {
	if config.MaxTokens == 0 {
		config.MaxTokens = 4000 // Default max context tokens
	}

	return &ContextBuilder{
		tracker:        tracker,
		maxTokens:      config.MaxTokens,
		agentName:      config.AgentName,
		includeHistory: config.IncludeHistory,
	}
}

// EnhancedMarketContext includes historical and portfolio data
type EnhancedMarketContext struct {
	CurrentMarket     MarketContext        `json:"current_market"`
	Positions         []PositionContext    `json:"positions,omitempty"`
	RecentDecisions   []HistoricalDecision `json:"recent_decisions,omitempty"`
	SimilarSituations []HistoricalDecision `json:"similar_situations,omitempty"`
	PortfolioSummary  *PortfolioSummary    `json:"portfolio_summary,omitempty"`
	MarketRegime      string               `json:"market_regime,omitempty"`
}

// PortfolioSummary provides high-level portfolio metrics
type PortfolioSummary struct {
	TotalValue    float64 `json:"total_value"`
	TotalPnL      float64 `json:"total_pnl"`
	OpenPositions int     `json:"open_positions"`
	DayPnL        float64 `json:"day_pnl"`
	WeekPnL       float64 `json:"week_pnl"`
	SuccessRate   float64 `json:"success_rate"`   // Last 24h
	AvgConfidence float64 `json:"avg_confidence"` // Last 24h
}

// BuildContext creates an enhanced context for LLM prompts
func (cb *ContextBuilder) BuildContext(
	ctx context.Context,
	market MarketContext,
	positions []PositionContext,
	portfolioSummary *PortfolioSummary,
) (*EnhancedMarketContext, error) {
	enhanced := &EnhancedMarketContext{
		CurrentMarket:    market,
		Positions:        positions,
		PortfolioSummary: portfolioSummary,
	}

	// Add historical decisions if enabled and tracker available
	if cb.includeHistory && cb.tracker != nil {
		// Get recent decisions (last 10)
		decisions, err := cb.tracker.GetRecentDecisions(ctx, cb.agentName, 10)
		if err == nil && len(decisions) > 0 {
			enhanced.RecentDecisions = cb.convertToHistoricalDecisions(decisions)
		}

		// Get similar situations (if symbol provided)
		if market.Symbol != "" {
			contextData := map[string]interface{}{
				"current_price": market.CurrentPrice,
				"indicators":    market.Indicators,
			}
			similar, err := cb.tracker.FindSimilarDecisions(ctx, market.Symbol, contextData, 5)
			if err == nil && len(similar) > 0 {
				enhanced.SimilarSituations = cb.convertToHistoricalDecisions(similar)
			}
		}
	}

	return enhanced, nil
}

// FormatContextForPrompt formats the context as a string for LLM prompts
func (cb *ContextBuilder) FormatContextForPrompt(enhanced *EnhancedMarketContext) string {
	var parts []string

	// 1. Current Market Conditions
	parts = append(parts, "## Current Market Conditions\n")
	parts = append(parts, fmt.Sprintf("Symbol: %s\n", enhanced.CurrentMarket.Symbol))
	parts = append(parts, fmt.Sprintf("Current Price: $%.2f\n", enhanced.CurrentMarket.CurrentPrice))

	if enhanced.CurrentMarket.PriceChange24h != 0 {
		parts = append(parts, fmt.Sprintf("24h Change: %.2f%%\n", enhanced.CurrentMarket.PriceChange24h))
	}
	if enhanced.CurrentMarket.Volume24h != 0 {
		parts = append(parts, fmt.Sprintf("24h Volume: $%.2f\n", enhanced.CurrentMarket.Volume24h))
	}

	// Indicators
	if len(enhanced.CurrentMarket.Indicators) > 0 {
		parts = append(parts, "\nTechnical Indicators:\n")
		for name, value := range enhanced.CurrentMarket.Indicators {
			parts = append(parts, fmt.Sprintf("  %s: %.4f\n", name, value))
		}
	}

	// 2. Portfolio Summary
	if enhanced.PortfolioSummary != nil {
		parts = append(parts, "\n## Portfolio Summary\n")
		ps := enhanced.PortfolioSummary
		parts = append(parts, fmt.Sprintf("Total Value: $%.2f\n", ps.TotalValue))
		parts = append(parts, fmt.Sprintf("Total P&L: $%.2f\n", ps.TotalPnL))
		parts = append(parts, fmt.Sprintf("Open Positions: %d\n", ps.OpenPositions))

		if ps.DayPnL != 0 {
			parts = append(parts, fmt.Sprintf("Today's P&L: $%.2f\n", ps.DayPnL))
		}
		if ps.WeekPnL != 0 {
			parts = append(parts, fmt.Sprintf("Week P&L: $%.2f\n", ps.WeekPnL))
		}
		if ps.SuccessRate > 0 {
			parts = append(parts, fmt.Sprintf("Recent Success Rate: %.1f%%\n", ps.SuccessRate))
		}
	}

	// 3. Current Positions
	if len(enhanced.Positions) > 0 {
		parts = append(parts, "\n## Current Positions\n")
		for i, pos := range enhanced.Positions {
			if i >= 5 { // Limit to 5 positions to save tokens
				parts = append(parts, fmt.Sprintf("... and %d more positions\n", len(enhanced.Positions)-5))
				break
			}
			pnlPercent := ((pos.CurrentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
			if pos.Side == "SHORT" {
				pnlPercent = -pnlPercent
			}
			parts = append(parts, fmt.Sprintf("%d. %s %s: Entry $%.2f → Current $%.2f (%.2f%% P&L, %s old)\n",
				i+1, pos.Symbol, pos.Side, pos.EntryPrice, pos.CurrentPrice, pnlPercent, pos.OpenDuration))
		}
	}

	// 4. Similar Past Situations (most important for learning)
	if len(enhanced.SimilarSituations) > 0 {
		parts = append(parts, "\n## Similar Past Situations\n")
		parts = append(parts, "In similar market conditions, this agent previously decided:\n\n")

		successCount := 0
		failureCount := 0
		totalPnL := 0.0

		for i, decision := range enhanced.SimilarSituations {
			if i >= 3 { // Show top 3 similar situations
				break
			}

			outcome := "PENDING"
			pnlStr := ""
			switch decision.Outcome {
			case "SUCCESS":
				successCount++
				outcome = "✓ SUCCESS"
				if decision.PnL != 0 {
					totalPnL += decision.PnL
					pnlStr = fmt.Sprintf(" (P&L: $%.2f)", decision.PnL)
				}
			case "FAILURE":
				failureCount++
				outcome = "✗ FAILURE"
				if decision.PnL != 0 {
					totalPnL += decision.PnL
					pnlStr = fmt.Sprintf(" (P&L: $%.2f)", decision.PnL)
				}
			}

			parts = append(parts, fmt.Sprintf("%d. Action: %s → %s%s\n", i+1, decision.Action, outcome, pnlStr))
			if decision.Reasoning != "" {
				// Truncate reasoning if too long
				reasoning := decision.Reasoning
				if len(reasoning) > 200 {
					reasoning = reasoning[:200] + "..."
				}
				parts = append(parts, fmt.Sprintf("   Reasoning: %s\n", reasoning))
			}
		}

		// Summary
		if successCount > 0 || failureCount > 0 {
			successRate := float64(successCount) / float64(successCount+failureCount) * 100
			parts = append(parts, fmt.Sprintf("\nSimilar Situations Summary: %d successes, %d failures (%.1f%% success rate)\n",
				successCount, failureCount, successRate))
			if totalPnL != 0 {
				parts = append(parts, fmt.Sprintf("Average P&L: $%.2f\n", totalPnL/float64(successCount+failureCount)))
			}
		}
	}

	// 5. Recent Decisions (condensed to save tokens)
	if len(enhanced.RecentDecisions) > 0 {
		parts = append(parts, "\n## Recent Decision History\n")

		// Show only last 5, condensed format
		recentCount := len(enhanced.RecentDecisions)
		if recentCount > 5 {
			recentCount = 5
		}

		for i := 0; i < recentCount; i++ {
			decision := enhanced.RecentDecisions[i]
			var outcome string
			switch decision.Outcome {
			case "SUCCESS":
				outcome = "✓"
			case "FAILURE":
				outcome = "✗"
			default:
				outcome = "⋯"
			}

			// Very condensed format
			parts = append(parts, fmt.Sprintf("- %s %s (conf: %.2f) %s\n",
				decision.Timestamp.Format("15:04"), decision.Action, decision.Confidence, outcome))
		}
	}

	// Join all parts
	context := strings.Join(parts, "")

	// Check token count and truncate if necessary
	tokens := cb.estimateTokens(context)
	if tokens > cb.maxTokens {
		context = cb.truncateToTokenLimit(context, cb.maxTokens)
	}

	return context
}

// FormatLearningContext creates a learning-focused context (for T185)
func (cb *ContextBuilder) FormatLearningContext(
	ctx context.Context,
	symbol string,
	currentIndicators map[string]float64,
) (string, error) {
	if cb.tracker == nil {
		return "", nil
	}

	// Get successful decisions for this symbol
	successful, err := cb.tracker.GetSuccessfulDecisions(ctx, cb.agentName, 10)
	if err != nil || len(successful) == 0 {
		return "", err
	}

	var parts []string
	parts = append(parts, "## What We've Learned\n\n")
	parts = append(parts, fmt.Sprintf("Based on %d successful past decisions for %s:\n\n", len(successful), symbol))

	totalPnL := 0.0
	patterns := make(map[string]int)

	for i, decision := range successful {
		if i >= 5 { // Top 5 wins
			break
		}

		// Extract pattern from context if available
		if len(decision.Context) > 0 {
			var contextData map[string]interface{}
			_ = json.Unmarshal(decision.Context, &contextData) // Best effort context extraction

			// Try to identify pattern
			if indicators, ok := contextData["indicators"].(map[string]interface{}); ok {
				// Analyze indicator patterns
				// This is simplified - real implementation would be more sophisticated
				for name := range indicators {
					patterns[name]++
				}
			}
		}

		if decision.PnL != nil {
			totalPnL += *decision.PnL
			parts = append(parts, fmt.Sprintf("%d. P&L: $%.2f (Confidence: %.2f)\n",
				i+1, *decision.PnL, decision.Confidence))
		}
	}

	avgPnL := totalPnL / float64(len(successful))
	parts = append(parts, fmt.Sprintf("\nAverage winning trade: $%.2f\n", avgPnL))

	// Most common patterns
	if len(patterns) > 0 {
		parts = append(parts, "\nMost relevant indicators in winning trades: ")
		count := 0
		for indicator := range patterns {
			if count >= 3 {
				break
			}
			parts = append(parts, indicator)
			if count < 2 {
				parts = append(parts, ", ")
			}
			count++
		}
		parts = append(parts, "\n")
	}

	return strings.Join(parts, ""), nil
}

// estimateTokens provides a rough token count estimate
// Rule of thumb: 1 token ≈ 4 characters for English text
func (cb *ContextBuilder) estimateTokens(text string) int {
	// Simple estimation: ~4 chars per token
	return len(text) / 4
}

// truncateToTokenLimit truncates text to fit within token limit
func (cb *ContextBuilder) truncateToTokenLimit(text string, maxTokens int) string {
	maxChars := maxTokens * 4 // Conservative estimate

	if len(text) <= maxChars {
		return text
	}

	// Truncate and add indicator
	truncated := text[:maxChars-50] // Leave room for message
	truncated += "\n\n[Context truncated to fit token limit]\n"

	return truncated
}

// convertToHistoricalDecisions converts database decisions to HistoricalDecision format
func (cb *ContextBuilder) convertToHistoricalDecisions(decisions []*db.LLMDecision) []HistoricalDecision {
	historical := make([]HistoricalDecision, 0, len(decisions))

	for _, d := range decisions {
		hd := HistoricalDecision{
			Timestamp:  d.CreatedAt,
			Confidence: d.Confidence,
		}

		// Parse response to get action
		// For simplicity, try to extract from response
		// In practice, might want to store action separately
		if strings.Contains(strings.ToUpper(d.Response), "\"ACTION\":\"BUY\"") ||
			strings.Contains(strings.ToUpper(d.Response), "\"SIDE\":\"BUY\"") {
			hd.Action = "BUY"
		} else if strings.Contains(strings.ToUpper(d.Response), "\"ACTION\":\"SELL\"") ||
			strings.Contains(strings.ToUpper(d.Response), "\"SIDE\":\"SELL\"") {
			hd.Action = "SELL"
		} else {
			hd.Action = "HOLD"
		}

		// Parse reasoning (first 200 chars of response)
		if len(d.Response) > 0 {
			// Try to extract reasoning from JSON response
			var responseData map[string]interface{}
			if err := json.Unmarshal([]byte(d.Response), &responseData); err == nil {
				if reasoning, ok := responseData["reasoning"].(string); ok {
					hd.Reasoning = reasoning
				}
			}

			// Fallback to truncated response
			if hd.Reasoning == "" {
				hd.Reasoning = d.Response
				if len(hd.Reasoning) > 200 {
					hd.Reasoning = hd.Reasoning[:200] + "..."
				}
			}
		}

		// Outcome
		if d.Outcome != nil {
			hd.Outcome = *d.Outcome
		} else {
			hd.Outcome = "PENDING"
		}

		// P&L
		if d.PnL != nil {
			hd.PnL = *d.PnL
		}

		historical = append(historical, hd)
	}

	return historical
}

// GetContextStats returns statistics about the context
func (cb *ContextBuilder) GetContextStats(enhanced *EnhancedMarketContext) map[string]interface{} {
	formatted := cb.FormatContextForPrompt(enhanced)

	return map[string]interface{}{
		"estimated_tokens": cb.estimateTokens(formatted),
		"char_count":       len(formatted),
		"has_history":      len(enhanced.RecentDecisions) > 0,
		"has_similar":      len(enhanced.SimilarSituations) > 0,
		"position_count":   len(enhanced.Positions),
		"decision_count":   len(enhanced.RecentDecisions),
		"similar_count":    len(enhanced.SimilarSituations),
	}
}

// BuildMinimalContext creates a minimal context when tokens are very limited
func (cb *ContextBuilder) BuildMinimalContext(market MarketContext) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Symbol: %s | Price: $%.2f", market.Symbol, market.CurrentPrice))

	if market.PriceChange24h != 0 {
		parts = append(parts, fmt.Sprintf(" | 24h: %.2f%%", market.PriceChange24h))
	}

	// Top 3 indicators only
	if len(market.Indicators) > 0 {
		parts = append(parts, " | ")
		count := 0
		for name, value := range market.Indicators {
			if count >= 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s: %.2f ", name, value))
			count++
		}
	}

	return strings.Join(parts, "")
}
