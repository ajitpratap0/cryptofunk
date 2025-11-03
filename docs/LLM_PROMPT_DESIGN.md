# LLM Prompt Design Patterns for CryptoFunk Trading Agents

This document describes the prompt engineering patterns used in CryptoFunk's multi-agent trading system. Each agent type has specialized prompts designed to elicit specific trading insights from LLMs (Claude/GPT-4).

## Table of Contents

1. [Prompt Architecture](#prompt-architecture)
2. [System Prompts by Agent Type](#system-prompts-by-agent-type)
3. [User Prompt Templates](#user-prompt-templates)
4. [JSON Response Format](#json-response-format)
5. [Best Practices](#best-practices)
6. [Example Prompts and Responses](#example-prompts-and-responses)
7. [Testing and Validation](#testing-and-validation)

## Prompt Architecture

All CryptoFunk agent prompts follow a two-part structure:

### 1. System Prompt (Agent Identity and Guidelines)

The system prompt establishes:
- **Agent identity and expertise** (e.g., "technical analysis expert")
- **Core responsibilities** (what decisions the agent makes)
- **Operational guidelines** (risk management rules, decision criteria)
- **Output format requirements** (JSON-only responses)

### 2. User Prompt (Context and Task)

The user prompt provides:
- **Market context** (symbol, price, volume, indicators)
- **Historical decisions** (similar past situations and outcomes)
- **Current positions** (portfolio state for context)
- **Specific task** (e.g., "analyze this signal," "assess this trade")
- **Expected output structure** (JSON schema with required fields)

## System Prompts by Agent Type

### Technical Analysis Agent

**Purpose**: Generate BUY/SELL/HOLD signals based on technical indicators and chart patterns.

**Key Components**:
- Expertise in technical analysis and pattern recognition
- Multi-indicator analysis (RSI, MACD, Bollinger Bands, EMAs)
- Conservative approach when indicators conflict
- Probability-based decision making

**System Prompt** (see `internal/llm/prompts.go:292-310`):
```
You are an expert technical analysis trading agent for cryptocurrency markets.

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

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.
```

### Trend Following Agent

**Purpose**: Identify and capitalize on strong market trends.

**Key Components**:
- Trend direction identification (up/down/sideways)
- Trend strength evaluation (strong/moderate/weak)
- Entry/exit timing optimization
- False breakout avoidance

**System Prompt** (see `internal/llm/prompts.go:312-330`):
```
You are an expert trend-following trading agent for cryptocurrency markets.

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

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.
```

### Mean Reversion Agent

**Purpose**: Identify overbought/oversold conditions and profit from price reversion to mean.

**Key Components**:
- Deviation from mean calculation
- Overbought/oversold detection
- Reversion probability estimation
- Target price determination

**System Prompt** (see `internal/llm/prompts.go:332-350`):
```
You are an expert mean reversion trading agent for cryptocurrency markets.

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

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.
```

### Risk Management Agent

**Purpose**: Evaluate trade proposals, approve/reject them, and determine position sizing.

**Key Components**:
- Risk/reward ratio assessment
- Position sizing calculation
- Stop loss/take profit level setting
- Portfolio exposure evaluation
- **Veto power** over all trades

**System Prompt** (see `internal/llm/prompts.go:352-377`):
```
You are an expert risk management agent for a cryptocurrency trading system.

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

Respond ONLY with valid JSON in the specified format. Do not include explanatory text outside the JSON.
```

## User Prompt Templates

### Technical Analysis Prompt

**Input Context**:
- Symbol (e.g., "BTC/USDT")
- Current price
- 24h price change percentage
- 24h volume
- Technical indicators (RSI, MACD, Bollinger Bands, SMA, EMA, ADX, etc.)

**Template** (see `internal/llm/prompts.go:43-76`):
```
Analyze the following market data for {symbol} and provide a trading signal.

Current Price: ${current_price}
24h Price Change: {price_change_24h}%
24h Volume: ${volume_24h}

Technical Indicators:
  {sorted list of indicators with values}

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
}
```

**Key Design Features**:
1. **Sorted indicators** - Deterministic output (alphabetical order)
2. **Explicit JSON schema** - Reduces parsing errors
3. **Metadata section** - Captures reasoning transparency
4. **Confidence score** - Quantifies conviction level

### Trend Following Prompt

**Input Context**:
- Symbol, current price, 24h change
- Trend indicators (EMA, MACD, ADX, momentum)
- **Historical decisions** - Similar past situations and outcomes

**Template** (see `internal/llm/prompts.go:79-110`):
```
Evaluate the trend for {symbol} and determine if we should enter or exit a position.

Current Price: ${current_price}
24h Price Change: {price_change_24h}%

Trend Indicators:
  {sorted indicators}

{formatted historical decisions - up to 5 most recent}

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
}
```

**Key Design Features**:
1. **Historical context** - Enables learning from past decisions
2. **Trend-specific fields** - Captures strength, direction, and timing
3. **Entry timing guidance** - Prevents premature entries

### Mean Reversion Prompt

**Input Context**:
- Symbol, current price, 24h change
- Mean reversion indicators (RSI, Bollinger Bands, deviation metrics)
- **Current positions** - Portfolio context for sizing

**Template** (see `internal/llm/prompts.go:113-146`):
```
Analyze mean reversion opportunities for {symbol}.

Current Price: ${current_price}
24h Price Change: {price_change_24h}%

Mean Reversion Indicators:
  {sorted indicators}

Current Positions:
  {formatted position details with P&L}

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
}
```

**Key Design Features**:
1. **Position awareness** - Prevents over-concentration
2. **Target price** - Quantifies expected reversion
3. **Time horizon** - Sets realistic expectations

### Risk Assessment Prompt

**Input Context**:
- **Proposed trade signal** (from strategy agent)
- Market context (price, volatility)
- Current positions (portfolio state)
- Portfolio value and risk limits

**Template** (see `internal/llm/prompts.go:148-198`):
```
Evaluate the risk of the following trade proposal and determine if it should be approved.

PROPOSED TRADE:
Symbol: {symbol}
Side: {side}
Signal Confidence: {confidence}
Reasoning: {reasoning}

MARKET CONTEXT:
Current Price: ${current_price}
24h Change: {price_change_24h}%

PORTFOLIO:
Total Value: ${portfolio_value}
Max Position Size: {max_position_size}%

Current Positions:
  {formatted positions with unrealized P&L}

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
}
```

**Key Design Features**:
1. **Trade proposal context** - Full transparency on what's being evaluated
2. **Portfolio risk context** - Enables correlation analysis
3. **Binary approval** - Clear gate-keeping decision
4. **Position sizing output** - Concrete actionable recommendation
5. **Risk mitigation** - Constructive feedback for improvement

## JSON Response Format

All agent responses **must** be valid JSON. The LLM client handles multiple response formats:

### Supported Response Formats

1. **Plain JSON** (preferred):
```json
{"action": "BUY", "confidence": 0.85, "reasoning": "..."}
```

2. **JSON in markdown code block**:
````markdown
```json
{"action": "BUY", "confidence": 0.85, "reasoning": "..."}
```
````

3. **JSON with surrounding text**:
```
Based on the analysis, here is my recommendation:

{"action": "BUY", "confidence": 0.85, "reasoning": "..."}

This signal has high confidence.
```

### Parsing Strategy

The `ParseJSONResponse` method uses a **3-tier fallback** (see `internal/llm/client.go:193-214`):

1. **Extract from markdown** - Try all markdown code block formats
2. **Extract first JSON object** - Use bracket matching to find complete JSON
3. **Try raw content** - Attempt to parse trimmed content directly

This robust parsing handles various LLM response styles gracefully.

### Required Fields by Agent Type

**Technical Analysis**:
```json
{
  "action": "BUY" | "SELL" | "HOLD",           // Required
  "confidence": 0.0-1.0,                       // Required
  "reasoning": "string",                       // Required
  "indicators": { "key": value },              // Optional
  "metadata": { "primary_signal": "..." }      // Optional
}
```

**Trend Following**:
```json
{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": 0.0-1.0,
  "reasoning": "string",
  "metadata": {
    "trend_strength": "STRONG" | "MODERATE" | "WEAK",
    "trend_direction": "UP" | "DOWN" | "SIDEWAYS",
    "entry_timing": "IMMEDIATE" | "WAIT" | "DO_NOT_ENTER"
  }
}
```

**Risk Management**:
```json
{
  "approved": true | false,                    // Required
  "position_size": 0.0-1.0,                    // Required if approved
  "stop_loss": float | null,                   // Recommended
  "take_profit": float | null,                 // Recommended
  "risk_score": 0.0-1.0,                       // Required
  "reasoning": "string",                       // Required
  "concerns": ["string"],                      // Optional
  "recommendations": ["string"]                // Optional
}
```

## Best Practices

### 1. **Deterministic Indicator Formatting**

**Problem**: Map iteration in Go is non-deterministic, causing prompt variations.

**Solution**: Sort map keys alphabetically before formatting (see `internal/llm/prompts.go:207-217`):
```go
keys := make([]string, 0, len(indicators))
for name := range indicators {
    keys = append(keys, name)
}
sort.Strings(keys)

for _, name := range keys {
    lines = append(lines, fmt.Sprintf("  %s: %.4f", name, indicators[name]))
}
```

**Benefit**: Consistent prompts enable reproducible testing and caching.

### 2. **Explicit JSON Schema in Prompt**

**Problem**: LLMs may omit fields or use unexpected formats.

**Solution**: Include complete JSON schema with type annotations in user prompt.

**Example**:
```
Provide your assessment in the following JSON format:
{
  "action": "BUY" | "SELL" | "HOLD",  // Type annotation
  "confidence": 0.0-1.0,               // Range constraint
  "reasoning": "detailed explanation"  // Field description
}
```

### 3. **Historical Context for Learning**

**Problem**: Agents make same mistakes repeatedly.

**Solution**: Include up to 5 recent similar decisions with outcomes in prompt (see `internal/llm/prompts.go:250-279`).

**Format**:
```
Recent Similar Decisions:
  Decision 1:
    Action: BUY (Confidence: 0.85)
    Reasoning: Strong bullish momentum
    Outcome: SUCCESS | P&L: $250.50
    Timestamp: 2024-01-15 14:30
```

**Benefit**: Enables pattern recognition and outcome-based learning.

### 4. **Position Context for Portfolio Awareness**

**Problem**: Agents suggest trades that over-concentrate portfolio.

**Solution**: Include current positions with unrealized P&L (see `internal/llm/prompts.go:221-248`).

**Format**:
```
Current Positions:
  BTC/USDT LONG:
    Entry: $44000.00 | Current: $45000.00 | Qty: 1.0000
    Unrealized P&L: $1000.00 (2.27%)
    Open Duration: 2h 30m
```

**Benefit**: Encourages diversification and correlation awareness.

### 5. **Confidence Scores for Weighted Voting**

**Problem**: All agent signals treated equally, regardless of conviction.

**Solution**: Require confidence scores (0.0-1.0) in all responses.

**Usage**: Orchestrator uses confidence-weighted voting (see Phase 5 implementation).

### 6. **Metadata for Explainability**

**Problem**: "Black box" decisions lack transparency.

**Solution**: Request metadata fields explaining decision factors.

**Example**:
```json
"metadata": {
  "primary_signal": "RSI oversold (28.5)",
  "supporting_signals": ["Bollinger band breach", "Volume spike"],
  "concerns": ["MACD still bearish", "Downtrend intact"]
}
```

**Benefit**: Enables post-trade analysis and debugging.

## Example Prompts and Responses

### Example 1: Technical Analysis - BUY Signal

**User Prompt**:
```
Analyze the following market data for BTC/USDT and provide a trading signal.

Current Price: $45000.50
24h Price Change: 2.50%
24h Volume: $1234567890.50

Technical Indicators:
  ADX: 35.20
  BB_Lower: 43500.00
  BB_Upper: 46500.00
  MACD: 125.30
  RSI: 65.50
  SMA_20: 44800.00

Based on this technical analysis, provide your assessment in the following JSON format:
{...}
```

**Expected Response**:
```json
{
  "action": "BUY",
  "confidence": 0.78,
  "reasoning": "Multiple bullish indicators align: RSI at 65.5 shows strong momentum without being overbought, price is trading above SMA_20 ($44800), and MACD is strongly positive (125.3). ADX at 35.2 confirms trend strength. Price is approaching upper Bollinger Band but not yet overextended. Risk/reward favors entry with stop below SMA_20.",
  "indicators": {
    "RSI": 65.5,
    "MACD": 125.3,
    "ADX": 35.2
  },
  "metadata": {
    "primary_signal": "MACD bullish crossover with strong momentum",
    "supporting_signals": ["RSI momentum", "Price above SMA_20", "Strong ADX trend"],
    "concerns": ["Approaching upper Bollinger Band - watch for reversal"]
  }
}
```

### Example 2: Risk Management - Trade Rejection

**User Prompt**:
```
Evaluate the risk of the following trade proposal and determine if it should be approved.

PROPOSED TRADE:
Symbol: BTC/USDT
Side: BUY
Signal Confidence: 0.85
Reasoning: Strong bullish reversal pattern forming

MARKET CONTEXT:
Current Price: $45000.00
24h Change: -5.20%

PORTFOLIO:
Total Value: $100000.00
Max Position Size: 10.00%

Current Positions:
  ETH/USDT LONG:
    Entry: $3200.00 | Current: $3250.00 | Qty: 5.0000
    Unrealized P&L: $250.00 (1.56%)
    Open Duration: 1d 3h

As the risk manager, evaluate this trade and provide your assessment in JSON format:
{...}
```

**Expected Response**:
```json
{
  "approved": false,
  "position_size": 0.0,
  "stop_loss": null,
  "take_profit": null,
  "risk_score": 0.72,
  "reasoning": "Trade rejected due to high correlation risk and adverse market conditions. BTC and ETH are highly correlated (>0.85), and opening a BTC position while holding ETH would create excessive crypto exposure. Additionally, 24h price decline of -5.2% suggests weak market sentiment. Signal confidence of 0.85 is high, but current market volatility warrants caution.",
  "concerns": [
    "High correlation with existing ETH position (>85%)",
    "Combined crypto exposure would exceed 20% of portfolio",
    "Recent 5.2% decline suggests bearish momentum",
    "No stop loss proposal in original signal",
    "Market volatility elevated"
  ],
  "recommendations": [
    "Wait for market stabilization (price consolidation above $44000)",
    "Consider reducing ETH position first to free up risk budget",
    "Require tighter stop loss if trade is re-proposed",
    "Monitor correlation metrics before entering correlated positions",
    "Consider position size of 5% instead of 10% given current volatility"
  ]
}
```

### Example 3: Mean Reversion - SELL Signal

**User Prompt**:
```
Analyze mean reversion opportunities for SOL/USDT.

Current Price: $125.30
24h Price Change: 15.80%

Mean Reversion Indicators:
  BB_Position: 2.35
  RSI: 82.50
  SMA_50: 110.00
  StdDev: 8.50

Current Positions:
  No open positions

Identify mean reversion opportunities and provide assessment in JSON format:
{...}
```

**Expected Response**:
```json
{
  "action": "SELL",
  "confidence": 0.82,
  "reasoning": "Strong mean reversion setup: Price is 2.35 standard deviations above the 50-day SMA, indicating extreme overbought condition. RSI at 82.5 confirms overbought territory. The 15.8% single-day move suggests parabolic price action likely to revert. Entry for short position is favorable with mean reversion target at SMA_50 ($110), offering 12% downside potential.",
  "metadata": {
    "deviation_from_mean": 2.35,
    "reversion_likelihood": "HIGH",
    "target_price": 110.00,
    "time_horizon": "SHORT"
  }
}
```

## Testing and Validation

### Unit Tests

All prompts have comprehensive unit tests (see `internal/llm/prompts_test.go`):

1. **System prompt tests** - Verify each agent type has correct system prompt
2. **Prompt building tests** - Validate user prompt formatting for each agent
3. **Indicator formatting tests** - Ensure deterministic output (sorted)
4. **Position formatting tests** - Verify P&L calculation display
5. **Historical decision tests** - Check limit to 5 most recent

### Integration Tests

LLM client integration tests (see `internal/llm/client_test.go`):

1. **JSON parsing tests** - Validate 3-tier fallback strategy
2. **Error classification tests** - Verify retryable vs non-retryable errors
3. **Retry logic tests** - Test exponential backoff and circuit breaking
4. **HTTP error handling** - Test 4xx (non-retryable) and 5xx (retryable)

### Manual Testing Checklist

Before deploying new prompts to production:

- [ ] Test with Claude Sonnet 4 (primary model)
- [ ] Test with GPT-4 (fallback model)
- [ ] Validate JSON parsing works for both models
- [ ] Check confidence scores are in 0.0-1.0 range
- [ ] Verify reasoning is detailed and actionable
- [ ] Test edge cases (empty indicators, no positions, conflicting signals)
- [ ] Measure prompt token count (target <1500 tokens)
- [ ] Validate response time <2 seconds (p95)

### A/B Testing

Use the Experiment Manager (Phase 9, T188) to compare prompts:

```go
expManager := llm.NewExperimentManager()

exp, _ := expManager.CreateExperiment(ctx, llm.CreateExperimentRequest{
    Name: "Technical Agent Prompt v2",
    Description: "Testing improved prompt with more explicit JSON schema",
    ControlVariant: llm.ExperimentVariant{
        Name: "Current Prompt",
        ModelConfig: llm.ClientConfig{Model: "claude-sonnet-4-20250514"},
        PromptTemplate: "current_template",
    },
    TreatmentVariant: llm.ExperimentVariant{
        Name: "New Prompt v2",
        ModelConfig: llm.ClientConfig{Model: "claude-sonnet-4-20250514"},
        PromptTemplate: "new_template_v2",
    },
    TrafficSplitPercent: 20, // 20% to treatment, 80% to control
})
```

**Metrics to track**:
- Signal quality (eventual P&L of trades)
- JSON parsing success rate
- Average confidence scores
- Response latency
- Token usage (cost)

## Prompt Evolution and Versioning

### Version Control

Prompts are version-controlled in `internal/llm/prompts.go`:

```go
const technicalAnalysisSystemPrompt = `...` // v1.0
```

**Best practice**: Use git tags for major prompt changes:
```bash
git tag -a prompts-v1.1 -m "Improved technical analysis prompt with risk/reward guidance"
```

### Iteration Process

1. **Identify issue** - Low confidence, poor P&L, parsing errors
2. **Hypothesis** - What prompt change might fix it?
3. **A/B test** - Create experiment with new prompt (20% traffic)
4. **Measure** - Run for 100+ decisions to get statistical significance
5. **Roll out or roll back** - Based on results
6. **Document** - Update this file with findings

### Common Prompt Issues and Fixes

| Issue | Symptom | Fix |
|-------|---------|-----|
| Overfitting to recent trends | High short-term accuracy, poor long-term | Add historical context with diverse outcomes |
| Overconfident signals | Confidence >0.9 for marginal setups | Emphasize uncertainty in system prompt |
| Missing JSON fields | Parsing errors | Add explicit field requirements in schema |
| Verbose reasoning | Excessive token usage | Add "Be concise" to system prompt |
| Conflicting signals | HOLD on clear BUY/SELL setups | Clarify decision criteria and thresholds |

## Conclusion

Effective prompt engineering is critical to CryptoFunk's multi-agent architecture. Key principles:

1. **Clarity** - Explicit schemas and requirements
2. **Consistency** - Deterministic formatting (sorted keys)
3. **Context** - Historical decisions and portfolio state
4. **Explainability** - Metadata and detailed reasoning
5. **Robustness** - Multi-tier JSON parsing
6. **Evolution** - A/B testing and continuous improvement

By following these patterns, agents can make high-quality trading decisions with transparency and accountability.

## Model Fallback and Resilience (T189)

### FallbackClient Architecture

The `FallbackClient` provides automatic failover between multiple LLM models with circuit breaker protection.

**Design**:
```
Primary (Claude Sonnet 4)
    ↓ [fails]
Fallback 1 (GPT-4)
    ↓ [fails]
Fallback 2 (GPT-3.5-turbo)
    ↓ [fails]
Error returned
```

**Usage Example**:
```go
import "github.com/ajitpratap0/cryptofunk/internal/llm"

// Create fallback client with 3 models
fc := llm.NewFallbackClient(llm.FallbackConfig{
    // Primary: Claude Sonnet 4
    PrimaryConfig: llm.ClientConfig{
        Endpoint: "http://localhost:8080/v1/chat/completions",
        Model:    "claude-sonnet-4-20250514",
        Timeout:  30 * time.Second,
    },
    PrimaryName: "claude-sonnet-4",

    // Fallbacks: GPT-4, then GPT-3.5-turbo
    FallbackConfigs: []llm.ClientConfig{
        {
            Endpoint: "http://localhost:8080/v1/chat/completions",
            Model:    "gpt-4",
            Timeout:  30 * time.Second,
        },
        {
            Endpoint: "http://localhost:8080/v1/chat/completions",
            Model:    "gpt-3.5-turbo",
            Timeout:  20 * time.Second,
        },
    },
    FallbackNames: []string{"gpt-4", "gpt-3.5-turbo"},

    // Circuit breaker configuration
    CircuitBreakerConfig: llm.CircuitBreakerConfig{
        FailureThreshold: 5,  // Open after 5 consecutive failures
        SuccessThreshold: 2,  // Close after 2 consecutive successes
        Timeout:          60 * time.Second,  // Try half-open after 60s
        TimeWindow:       5 * time.Minute,   // Track failures in 5min window
    },
})

// Use just like regular client
messages := []llm.ChatMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: userPrompt},
}

resp, err := fc.Complete(ctx, messages)
if err != nil {
    // All models failed
    log.Error().Err(err).Msg("All LLM models failed")
    return err
}

// Success (from whichever model succeeded first)
content := resp.Choices[0].Message.Content
```

### Circuit Breaker Pattern

**States**:
- **CLOSED**: Normal operation, requests flow through
- **OPEN**: Too many failures, requests blocked (fails fast)
- **HALF_OPEN**: Testing recovery, allow one request through

**State Transitions**:
```
CLOSED --[5 failures]--> OPEN
OPEN --[60s timeout]--> HALF_OPEN
HALF_OPEN --[2 successes]--> CLOSED
HALF_OPEN --[1 failure]--> OPEN
```

**Benefits**:
1. **Fail fast**: Don't waste time on consistently failing models
2. **Automatic recovery**: Test if model is back online after timeout
3. **Resource protection**: Prevent cascade failures
4. **Cost reduction**: Skip expensive calls to unavailable models

**Monitoring Circuit Breaker**:
```go
// Get status of all model circuits
statuses := fc.GetCircuitBreakerStatus()

for _, status := range statuses {
    log.Info().
        Int("model_index", status.ModelIndex).
        Str("state", string(status.State)).
        Int("consecutive_failures", status.ConsecutiveFailures).
        Time("last_failure", status.LastFailure).
        Msg("Circuit breaker status")
}

// Manually reset a circuit (e.g., after deployment)
fc.ResetCircuitBreaker(0) // Reset primary model
```

### Fallback Strategies

**1. Non-retryable errors skip immediately**:
```go
// 400 Bad Request - skip to next model immediately
// Don't waste retries on validation errors
```

**2. Retryable errors use exponential backoff**:
```go
// 429 Rate Limit - retry with backoff on same model
// Then try next model if all retries fail
fc.CompleteWithRetry(ctx, messages, 3) // Max 3 retries per model
```

**3. Circuit breaker prevents repeated failures**:
```go
// If model fails 5 times consecutively, circuit opens
// Skip that model for 60 seconds
// Prevents wasting time on broken endpoints
```

### Configuration Recommendations

**Production Configuration**:
```go
llm.CircuitBreakerConfig{
    FailureThreshold: 5,              // Open after 5 failures
    SuccessThreshold: 2,              // Close after 2 successes
    Timeout:          60 * time.Second,  // Test recovery after 1min
    TimeWindow:       5 * time.Minute,   // 5min failure window
}
```

**Development Configuration**:
```go
llm.CircuitBreakerConfig{
    FailureThreshold: 3,              // Open faster in dev
    SuccessThreshold: 1,              // Close faster in dev
    Timeout:          10 * time.Second,  // Test recovery sooner
    TimeWindow:       2 * time.Minute,   // Shorter window
}
```

### Testing Fallback

**Simulate primary failure**:
```bash
# Kill primary model endpoint
curl -X POST http://localhost:8080/admin/kill/claude-sonnet-4

# Requests automatically fallback to GPT-4
# Circuit opens after 5 failures
# All subsequent requests skip primary
```

**Reset circuit after fix**:
```bash
# Fix and restart primary endpoint
curl -X POST http://localhost:8080/admin/start/claude-sonnet-4

# Reset circuit breaker via API
curl -X POST http://localhost:8080/admin/circuit-breaker/reset/0
```

### Metrics to Track

1. **Fallback rate**: % of requests using fallback models
2. **Circuit state changes**: CLOSED → OPEN → HALF_OPEN transitions
3. **Model-specific success rates**: Track each model independently
4. **Cost by model**: Claude vs GPT-4 vs GPT-3.5 usage
5. **Latency by model**: Which model is fastest?

## Using Fallback in Agents

All trading agents (technical, trend, reversion, risk) support automatic model fallback through the `LLMClient` interface.

### Agent Integration Pattern

Agents use the `LLMClient` interface which is implemented by both `Client` and `FallbackClient`:

```go
type LLMClient interface {
    Complete(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
    CompleteWithRetry(ctx context.Context, messages []ChatMessage, maxRetries int) (*ChatResponse, error)
    CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
    ParseJSONResponse(content string, target interface{}) error
}
```

**Agent struct** (example from technical-agent):
```go
type TechnicalAgent struct {
    *agents.BaseAgent

    // LLM client - interface supports both Client and FallbackClient
    llmClient llm.LLMClient
    promptBuilder *llm.PromptBuilder
    useLLM bool
}
```

### Configuration-Driven Fallback

Agents automatically use `FallbackClient` when `llm.fallback_models` is configured in `configs/config.yaml`:

```yaml
llm:
  enabled: true
  endpoint: "http://localhost:8080/v1/chat/completions"
  primary_model: "claude-sonnet-4-20250514"

  # Enable fallback by listing fallback models
  fallback_models:
    - "gpt-4-turbo"
    - "gpt-3.5-turbo"

  # Circuit breaker settings
  circuit_breaker:
    failure_threshold: 5
    success_threshold: 2
    timeout: 60s
    time_window: 5m
```

**Without fallback models**:
- Agents use basic `Client`
- Single model, no automatic failover
- Simpler, lower overhead

**With fallback models**:
- Agents use `FallbackClient`
- Automatic cascading failover (Claude → GPT-4 → GPT-3.5)
- Circuit breaker protection
- Zero code changes required

### Agent Initialization (Automatic)

All agents follow this initialization pattern:

```go
// Initialize LLM client if enabled
var llmClient llm.LLMClient
useLLM := viper.GetBool("llm.enabled")

if useLLM {
    primaryConfig := llm.ClientConfig{
        Endpoint:    viper.GetString("llm.endpoint"),
        APIKey:      viper.GetString("llm.api_key"),
        Model:       viper.GetString("llm.primary_model"),
        Temperature: viper.GetFloat64("llm.temperature"),
        MaxTokens:   viper.GetInt("llm.max_tokens"),
        Timeout:     viper.GetDuration("llm.timeout"),
    }

    // Check if fallback models are configured
    fallbackModels := viper.GetStringSlice("llm.fallback_models")
    if len(fallbackModels) > 0 {
        // Create FallbackClient
        llmClient = llm.NewFallbackClient(fallbackConfig)
        log.Info().
            Str("primary_model", primaryConfig.Model).
            Strs("fallback_models", fallbackModels).
            Msg("LLM fallback client initialized")
    } else {
        // Create basic Client
        llmClient = llm.NewClient(primaryConfig)
        log.Info().Msg("LLM client initialized")
    }
}
```

### Agent Usage (No Changes Needed)

Agents use `llmClient` the same way regardless of whether it's a `Client` or `FallbackClient`:

```go
// Build market context
marketCtx := llm.MarketContext{
    Symbol:       symbol,
    CurrentPrice: currentPrice,
    Indicators:   indicatorMap,
}

// Generate prompt
userPrompt := a.promptBuilder.BuildTechnicalAnalysisPrompt(marketCtx)
systemPrompt := a.promptBuilder.GetSystemPrompt()

// Call LLM (automatically handles fallback if configured)
response, err := a.llmClient.CompleteWithRetry(ctx, []llm.ChatMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: userPrompt},
}, 2)

if err != nil {
    // All models failed (primary + all fallbacks)
    log.Error().Err(err).Msg("LLM completion failed after fallbacks")
    return ruleBasedAnalysis() // Fall back to rule-based
}

// Parse response
var signal llm.Signal
if err := a.llmClient.ParseJSONResponse(response.Choices[0].Message.Content, &signal); err != nil {
    log.Error().Err(err).Msg("Failed to parse LLM response")
    return ruleBasedAnalysis()
}
```

### Benefits for Agents

1. **Zero-downtime resilience**: If Claude is down, automatically use GPT-4
2. **Cost optimization**: Use cheaper fallback models during high-volume periods
3. **Performance**: Fast-fail with circuit breaker prevents wasted retries
4. **Transparent**: Agent code unchanged, purely configuration-driven
5. **Monitoring**: Track which models are being used and their success rates

### Production Deployment

**Recommended configuration for production**:

```yaml
llm:
  enabled: true
  primary_model: "claude-sonnet-4-20250514"  # Best quality
  fallback_models:
    - "gpt-4-turbo"        # High quality fallback
    - "gpt-3.5-turbo"      # Cost-effective backup

  circuit_breaker:
    failure_threshold: 5   # Open circuit after 5 failures
    success_threshold: 2   # Require 2 successes to close
    timeout: 60s           # Test recovery after 60s
    time_window: 5m        # Track failures over 5min window
```

**For high-frequency trading** (minimize latency):
```yaml
llm:
  primary_model: "gpt-3.5-turbo"  # Fastest model
  fallback_models:
    - "claude-sonnet-4-20250514"  # Quality fallback

  circuit_breaker:
    failure_threshold: 3
    timeout: 30s
    time_window: 2m
```

### Testing Agent Fallback

Run integration tests to verify agent behavior with fallback:

```bash
# Run all LLM tests including agent integration
go test -v -race ./internal/llm/...

# Run only agent integration tests
go test -v -race -run TestAgent ./internal/llm/

# Test output shows fallback in action:
# {"level":"warn","model":"claude-sonnet-4","message":"LLM completion failed, trying fallback"}
# {"level":"info","model":"gpt-4","message":"LLM completion succeeded"}
```

---

**Last Updated**: Phase 9 (T187, T189 completion + agent integration)
**Test Coverage**: 71.3% (internal/llm package)
**Related Files**:
- `internal/llm/prompts.go` - Prompt templates
- `internal/llm/prompts_test.go` - Unit tests
- `internal/llm/client.go` - JSON parsing and HTTP client
- `internal/llm/client_test.go` - Integration tests
- `internal/llm/fallback.go` - Fallback client and circuit breaker
- `internal/llm/fallback_test.go` - Fallback and circuit breaker tests
- `internal/llm/interface.go` - LLMClient interface
- `internal/llm/agent_integration_test.go` - Agent integration tests
- `configs/config.yaml` - LLM and fallback configuration
- `cmd/agents/technical-agent/main.go` - Example agent implementation
- `cmd/agents/trend-agent/main.go` - Trend following agent
- `cmd/agents/reversion-agent/main.go` - Mean reversion agent
- `cmd/agents/risk-agent/main.go` - Risk management agent
