package llm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PromptBuilder builds prompts for different agent types
type PromptBuilder struct {
	agentType AgentType
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder(agentType AgentType) *PromptBuilder {
	return &PromptBuilder{
		agentType: agentType,
	}
}

// GetSystemPrompt returns the system prompt for the agent type
func (pb *PromptBuilder) GetSystemPrompt() string {
	switch pb.agentType {
	case AgentTypeTechnical:
		return technicalAnalysisSystemPrompt
	case AgentTypeTrend:
		return trendFollowingSystemPrompt
	case AgentTypeReversion:
		return meanReversionSystemPrompt
	case AgentTypeRisk:
		return riskManagementSystemPrompt
	case AgentTypeOrderbook:
		return orderbookAnalysisSystemPrompt
	case AgentTypeSentiment:
		return sentimentAnalysisSystemPrompt
	default:
		return defaultSystemPrompt
	}
}

// BuildTechnicalAnalysisPrompt builds a prompt for technical analysis
func (pb *PromptBuilder) BuildTechnicalAnalysisPrompt(ctx MarketContext) string {
	indicators := formatIndicators(ctx.Indicators)

	return fmt.Sprintf(`Analyze the following market data for %s and provide a trading signal.

Current Price: $%.2f
24h Price Change: %.2f%%
24h Volume: $%.2f

Technical Indicators:
%s

Based on this technical analysis, provide your assessment in the following JSON format:
{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": 0.0-1.0,
  "reasoning": "detailed explanation of your analysis",
  "indicators": {
    "key_indicator_1": value,
    "key_indicator_2": value
  },
  "metadata": {
    "primary_signal": "which indicator influenced decision most",
    "supporting_signals": ["list", "of", "supporting", "indicators"],
    "concerns": ["any", "concerns", "or", "conflicting", "signals"]
  }
}`,
		ctx.Symbol,
		ctx.CurrentPrice,
		ctx.PriceChange24h,
		ctx.Volume24h,
		indicators,
	)
}

// BuildTrendFollowingPrompt builds a prompt for trend following
func (pb *PromptBuilder) BuildTrendFollowingPrompt(ctx MarketContext, historicalData []HistoricalDecision) string {
	indicators := formatIndicators(ctx.Indicators)
	history := formatHistoricalDecisions(historicalData)

	return fmt.Sprintf(`Evaluate the trend for %s and determine if we should enter or exit a position.

Current Price: $%.2f
24h Price Change: %.2f%%

Trend Indicators:
%s

%s

Provide your trend-following assessment in JSON format:
{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": 0.0-1.0,
  "reasoning": "detailed trend analysis",
  "metadata": {
    "trend_strength": "STRONG" | "MODERATE" | "WEAK",
    "trend_direction": "UP" | "DOWN" | "SIDEWAYS",
    "entry_timing": "IMMEDIATE" | "WAIT" | "DO_NOT_ENTER"
  }
}`,
		ctx.Symbol,
		ctx.CurrentPrice,
		ctx.PriceChange24h,
		indicators,
		history,
	)
}

// BuildMeanReversionPrompt builds a prompt for mean reversion
func (pb *PromptBuilder) BuildMeanReversionPrompt(ctx MarketContext, positions []PositionContext) string {
	indicators := formatIndicators(ctx.Indicators)
	positionsData := formatPositions(positions)

	return fmt.Sprintf(`Analyze mean reversion opportunities for %s.

Current Price: $%.2f
24h Price Change: %.2f%%

Mean Reversion Indicators:
%s

Current Positions:
%s

Identify mean reversion opportunities and provide assessment in JSON format:
{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": 0.0-1.0,
  "reasoning": "mean reversion analysis",
  "metadata": {
    "deviation_from_mean": float,
    "reversion_likelihood": "HIGH" | "MEDIUM" | "LOW",
    "target_price": float,
    "time_horizon": "SHORT" | "MEDIUM" | "LONG"
  }
}`,
		ctx.Symbol,
		ctx.CurrentPrice,
		ctx.PriceChange24h,
		indicators,
		positionsData,
	)
}

// BuildRiskAssessmentPrompt builds a prompt for risk assessment
func (pb *PromptBuilder) BuildRiskAssessmentPrompt(
	signal Signal,
	ctx MarketContext,
	positions []PositionContext,
	portfolioValue float64,
	maxPositionSize float64,
) string {
	positionsData := formatPositions(positions)

	return fmt.Sprintf(`Evaluate the risk of the following trade proposal and determine if it should be approved.

PROPOSED TRADE:
Symbol: %s
Side: %s
Signal Confidence: %.2f
Reasoning: %s

MARKET CONTEXT:
Current Price: $%.2f
24h Change: %.2f%%

PORTFOLIO:
Total Value: $%.2f
Max Position Size: %.2f%%

Current Positions:
%s

As the risk manager, evaluate this trade and provide your assessment in JSON format:
{
  "approved": true | false,
  "position_size": float (recommended position size as fraction of portfolio, 0.0-1.0),
  "stop_loss": float (recommended stop loss price) | null,
  "take_profit": float (recommended take profit price) | null,
  "risk_score": 0.0-1.0 (0 = low risk, 1 = high risk),
  "reasoning": "detailed risk assessment",
  "concerns": ["list", "of", "risk", "concerns"],
  "recommendations": ["list", "of", "risk", "mitigation", "recommendations"]
}`,
		signal.Symbol,
		signal.Side,
		signal.Confidence,
		signal.Reasoning,
		ctx.CurrentPrice,
		ctx.PriceChange24h,
		portfolioValue,
		maxPositionSize*100,
		positionsData,
	)
}

// Helper functions

func formatIndicators(indicators map[string]float64) string {
	if len(indicators) == 0 {
		return "No indicators available"
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(indicators))
	for name := range indicators {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	var lines []string
	for _, name := range keys {
		lines = append(lines, fmt.Sprintf("  %s: %.4f", name, indicators[name]))
	}
	return strings.Join(lines, "\n")
}

func formatPositions(positions []PositionContext) string {
	if len(positions) == 0 {
		return "No open positions"
	}

	var lines []string
	for _, pos := range positions {
		pnlPercent := ((pos.CurrentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		if pos.Side == "SHORT" {
			pnlPercent = -pnlPercent
		}

		lines = append(lines, fmt.Sprintf(`  %s %s:
    Entry: $%.2f | Current: $%.2f | Qty: %.4f
    Unrealized P&L: $%.2f (%.2f%%)
    Open Duration: %s`,
			pos.Symbol,
			pos.Side,
			pos.EntryPrice,
			pos.CurrentPrice,
			pos.Quantity,
			pos.UnrealizedPnL,
			pnlPercent,
			pos.OpenDuration,
		))
	}
	return strings.Join(lines, "\n\n")
}

func formatHistoricalDecisions(decisions []HistoricalDecision) string {
	if len(decisions) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "Recent Similar Decisions:")

	for i, decision := range decisions {
		if i >= 5 { // Limit to 5 most recent
			break
		}

		lines = append(lines, fmt.Sprintf(`  Decision %d:
    Action: %s (Confidence: %.2f)
    Reasoning: %s
    Outcome: %s | P&L: $%.2f
    Timestamp: %s`,
			i+1,
			decision.Action,
			decision.Confidence,
			decision.Reasoning,
			decision.Outcome,
			decision.PnL,
			decision.Timestamp.Format("2006-01-02 15:04"),
		))
	}

	return strings.Join(lines, "\n\n")
}

// FormatContextAsJSON formats context as JSON for structured prompts
func FormatContextAsJSON(data interface{}) string {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// System prompts for each agent type

const technicalAnalysisSystemPrompt = `You are an expert technical analysis trading agent for cryptocurrency markets.

Your role is to analyze technical indicators and chart patterns to generate trading signals.

Key responsibilities:
- Analyze price action, volume, and technical indicators
- Identify support and resistance levels
- Recognize chart patterns (head and shoulders, triangles, etc.)
- Evaluate momentum and trend strength
- Generate BUY, SELL, or HOLD signals with confidence scores

Guidelines:
- Always provide detailed reasoning for your decisions
- Consider multiple indicators before making a decision
- Be conservative when indicators conflict
- Acknowledge uncertainty and conflicting signals
- Focus on probability and risk/reward ratios

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`

const trendFollowingSystemPrompt = `You are an expert trend-following trading agent for cryptocurrency markets.

Your role is to identify and capitalize on strong market trends.

Key responsibilities:
- Identify trend direction (up, down, sideways)
- Evaluate trend strength and momentum
- Determine optimal entry and exit points
- Avoid false breakouts and whipsaws
- Ride trends while protecting profits

Guidelines:
- The trend is your friend - follow strong trends
- Wait for confirmation before entering
- Use trailing stops to protect profits
- Cut losses quickly on trend reversals
- Be patient and disciplined

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`

const meanReversionSystemPrompt = `You are an expert mean reversion trading agent for cryptocurrency markets.

Your role is to identify when prices have deviated significantly from their mean and are likely to revert.

Key responsibilities:
- Calculate price deviation from mean
- Identify overbought and oversold conditions
- Estimate reversion probability and timing
- Determine target prices for mean reversion
- Manage risk on failed reversions

Guidelines:
- Mean reversion works best in range-bound markets
- Avoid trading against strong trends
- Use statistical measures (Bollinger Bands, RSI, etc.)
- Set realistic profit targets
- Always use stop losses

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`

const riskManagementSystemPrompt = `You are an expert risk management agent for a cryptocurrency trading system.

Your role is to evaluate proposed trades and ensure they align with risk management rules.

Key responsibilities:
- Assess risk/reward ratio of proposed trades
- Determine appropriate position sizing
- Set stop loss and take profit levels
- Evaluate portfolio exposure and concentration
- Approve or reject trades based on risk criteria

Risk Management Rules:
- Maximum position size per trade
- Maximum portfolio exposure
- Diversification requirements
- Stop loss mandatory on all positions
- Risk no more than specified percentage per trade

Guidelines:
- Be conservative - err on the side of caution
- Preserve capital is the top priority
- Consider correlation between positions
- Evaluate market conditions (volatility, liquidity)
- Provide clear reasoning for rejections

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`

const orderbookAnalysisSystemPrompt = `You are an expert order book analysis agent for cryptocurrency markets.

Your role is to analyze order book depth and flow to identify trading opportunities.

Key responsibilities:
- Analyze bid/ask spread and depth
- Identify large orders (walls) and their impact
- Detect order flow imbalances
- Assess liquidity and market impact
- Generate signals based on order book dynamics

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`

const sentimentAnalysisSystemPrompt = `You are an expert sentiment analysis agent for cryptocurrency markets.

Your role is to analyze market sentiment from various sources and generate trading signals.

Key responsibilities:
- Analyze social media sentiment
- Evaluate news and media coverage
- Assess market fear and greed
- Identify sentiment shifts
- Generate signals based on sentiment analysis

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`

const defaultSystemPrompt = `You are an AI trading agent for cryptocurrency markets.

Provide trading signals based on the data provided.

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.`
