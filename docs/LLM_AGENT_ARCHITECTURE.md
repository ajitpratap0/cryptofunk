# CryptoFunk - LLM-Powered Agent Architecture

**Version:** 2.0 (LLM-First MVP)
**Date:** 2025-10-27
**Status:** Active Design

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [LLM-First Architecture Philosophy](#2-llm-first-architecture-philosophy)
3. [Bifrost Gateway Integration](#3-bifrost-gateway-integration)
4. [Agent Types & Prompts](#4-agent-types--prompts)
5. [Context Building](#5-context-building)
6. [Decision Flow](#6-decision-flow)
7. [Explainability](#7-explainability)
8. [Memory & Learning](#8-memory--learning)
9. [Error Handling & Fallbacks](#9-error-handling--fallbacks)
10. [Performance Optimization](#10-performance-optimization)
11. [Migration Path to Custom Models](#11-migration-path-to-custom-models)

> **Note**: This document describes the **LLM-powered MVP** architecture. For future custom RL model architecture, see [AGENT_ARCHITECTURE_FUTURE.md](AGENT_ARCHITECTURE_FUTURE.md)

---

## 1. Executive Summary

### 1.1 Overview

CryptoFunk uses **Large Language Models (LLMs)** as the reasoning engine for all trading agents. Instead of training custom reinforcement learning models from scratch, we leverage Claude Sonnet 4 and GPT-4 Turbo via the Bifrost gateway for immediate sophisticated reasoning capabilities.

### 1.2 Key Design Decisions

**Why LLM-First for MVP?**

1. **Speed to Market**: 9.5 weeks vs 12+ weeks with custom models
2. **Sophisticated Reasoning**: Claude/GPT-4 provide better reasoning than MVP-stage custom models
3. **Natural Language Explainability**: Every decision includes human-readable reasoning
4. **No Training Data Required**: Can start trading immediately without historical training
5. **Data Collection**: Use LLM-powered MVP to collect high-quality trading data for future custom models

**Trade-offs Accepted**:

- API costs (~$0.01-0.10 per decision, reduced 90% with caching)
- External dependency on LLM providers (mitigated with automatic failover)
- Latency (<100ms with Bifrost, acceptable for trading)

### 1.3 Architecture Principles

- **Single Responsibility**: Each agent has one job and one LLM prompt template
- **Composability**: Agents combine via MCP for complex reasoning
- **Explainability**: All decisions include natural language reasoning
- **Resilience**: Automatic failover between LLM providers
- **Cost Optimization**: Semantic caching reduces costs by 90%

---

## 2. LLM-First Architecture Philosophy

### 2.1 The LLM as Agent Brain

In traditional agent architectures, agents have:
- **Perception**: Sensors to gather data
- **Reasoning**: Decision-making logic (rule-based or learned)
- **Action**: Actuators to execute decisions

In our LLM-powered architecture:
- **Perception**: MCP tools fetch market data, indicators
- **Reasoning**: **LLM provides sophisticated natural language reasoning**
- **Action**: MCP tools execute orders

```
┌─────────────────────────────────────────────┐
│           Agent Decision Cycle               │
│                                              │
│  1. PERCEIVE (MCP Tools)                    │
│     ↓                                        │
│  2. BUILD CONTEXT (Format for LLM)          │
│     ↓                                        │
│  3. REASON (LLM via Bifrost)                │
│     - Claude analyzes market conditions     │
│     - Generates decision with reasoning     │
│     ↓                                        │
│  4. PARSE RESPONSE (JSON extraction)        │
│     ↓                                        │
│  5. ACT (MCP Tools)                         │
│     ↓                                        │
│  6. LEARN (Store decision + outcome)        │
└─────────────────────────────────────────────┘
```

### 2.2 Hybrid Intelligence

Our agents use **hybrid intelligence**:

1. **Structural Intelligence (Code)**:
   - Data fetching via CCXT
   - Technical indicators via cinar/indicator
   - Position sizing calculations
   - Risk limits enforcement

2. **Reasoning Intelligence (LLM)**:
   - Market condition interpretation
   - Pattern recognition
   - Strategy selection
   - Risk assessment
   - Natural language explanation

**Example**: Trend Following Agent
```go
// 1. Structural Intelligence: Fetch data (code)
prices := fetchPrices("BTCUSDT", "1h", 100)
ema50 := indicator.EMA(prices, 50)
ema200 := indicator.EMA(prices, 200)
adx := indicator.ADX(prices, 14)

// 2. Build context (code)
context := fmt.Sprintf(`
Market: BTCUSDT
EMA50: %.2f
EMA200: %.2f
ADX: %.2f
Recent Prices: %v
`, ema50, ema200, adx, prices[len(prices)-10:])

// 3. Reasoning Intelligence: LLM decides (Claude/GPT-4)
decision := callLLM(trendFollowingPrompt, context)

// decision.action: "BUY" | "SELL" | "HOLD"
// decision.confidence: 0.85
// decision.reasoning: "EMA50 crossed above EMA200 (golden cross)..."
```

### 2.3 Benefits Over Custom Models (MVP Stage)

| Aspect | Custom RL Model | LLM-Powered |
|--------|----------------|-------------|
| **Time to Deploy** | 8+ weeks (data + training) | 2-3 days (prompts) |
| **Initial Quality** | Poor (needs data) | Excellent (pre-trained) |
| **Explainability** | None (black box) | Excellent (natural language) |
| **Debugging** | Difficult | Easy (read reasoning) |
| **Iteration Speed** | Slow (retrain) | Fast (update prompts) |
| **Generalization** | Limited | Strong |
| **Cost** | Training compute | API calls (~$0.01/decision, cached 90%) |

---

## 3. Bifrost Gateway Integration

### 3.1 Why Bifrost?

**Bifrost** is a unified LLM gateway that provides:

- **Single API**: OpenAI-compatible API for Claude, GPT-4, Gemini
- **Automatic Failover**: If Claude is down, use GPT-4 automatically
- **Semantic Caching**: 90% cost reduction for repeated prompts
- **Ultra-Low Latency**: <100µs overhead at 5k RPS
- **Production Ready**: 50x faster than LiteLLM

### 3.2 Architecture

```
┌──────────────────────────────────────────────┐
│            Trading Agents                    │
│  (Technical, Trend, Reversion, Risk)         │
└──────────────┬───────────────────────────────┘
               │
               │ HTTP POST: /v1/chat/completions
               │ (OpenAI-compatible API)
               ↓
┌──────────────────────────────────────────────┐
│         BIFROST LLM GATEWAY                  │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Routing Logic                         │ │
│  │  - Primary: Claude Sonnet 4            │ │
│  │  - Fallback: GPT-4 Turbo               │ │
│  │  - Backup: Gemini Pro                  │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Semantic Cache (Redis)                │ │
│  │  - Cache similar prompts               │ │
│  │  - 90% hit rate → 90% cost savings     │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  Observability                         │ │
│  │  - Latency per provider                │ │
│  │  - Cost tracking                       │ │
│  │  - Cache hit rates                     │ │
│  └────────────────────────────────────────┘ │
└──────────────┬───────────────────────────────┘
               │
        ┌──────┴──────┬──────────┐
        │             │          │
        ↓             ↓          ↓
  ┌─────────┐  ┌──────────┐  ┌────────┐
  │ Claude  │  │  GPT-4   │  │ Gemini │
  │ Sonnet  │  │  Turbo   │  │  Pro   │
  └─────────┘  └──────────┘  └────────┘
```

### 3.3 Configuration

**docker-compose.yml**:
```yaml
services:
  bifrost:
    image: maximhq/bifrost:latest
    ports:
      - "8080:8080"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - GOOGLE_API_KEY=${GOOGLE_API_KEY}
    volumes:
      - ./configs/bifrost.yaml:/etc/bifrost/config.yaml
```

**configs/bifrost.yaml**:
```yaml
providers:
  - name: claude
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    priority: 1  # Primary
    models:
      - claude-sonnet-4-20250514
    rate_limit: 1000  # requests per minute
    timeout: 30s

  - name: openai
    type: openai
    api_key: ${OPENAI_API_KEY}
    priority: 2  # Fallback
    models:
      - gpt-4-turbo
    rate_limit: 500
    timeout: 30s

  - name: gemini
    type: google
    api_key: ${GOOGLE_API_KEY}
    priority: 3  # Backup
    models:
      - gemini-pro
    rate_limit: 500
    timeout: 30s

routing:
  strategy: failover
  retry_attempts: 2
  retry_delay: 1s

cache:
  enabled: true
  backend: redis
  redis_url: redis://localhost:6379
  ttl: 3600  # 1 hour
  similarity_threshold: 0.95  # Cache hit threshold

observability:
  metrics_enabled: true
  metrics_port: 9090
  log_level: info
```

### 3.4 Go Client Implementation

**internal/llm/client.go**:
```go
package llm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    BaseURL    string
    HTTPClient *http.Client
}

type ChatRequest struct {
    Model       string          `json:"model"`
    Messages    []Message       `json:"messages"`
    Temperature float64         `json:"temperature"`
    MaxTokens   int             `json:"max_tokens"`
}

type Message struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant"
    Content string `json:"content"`
}

type ChatResponse struct {
    ID      string   `json:"id"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

type Choice struct {
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

func NewClient(baseURL string) *Client {
    return &Client{
        BaseURL: baseURL,
        HTTPClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *Client) Chat(req ChatRequest) (*ChatResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    httpReq, err := http.NewRequest("POST",
        c.BaseURL+"/v1/chat/completions",
        bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.HTTPClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("LLM API error: %s", resp.Status)
    }

    var chatResp ChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &chatResp, nil
}
```

---

## 4. Agent Types & Prompts

### 4.1 Technical Analysis Agent

**Purpose**: Analyze technical indicators and generate signals

**Prompt Template**:
```go
const TechnicalAnalysisPrompt = `You are an expert technical analyst for cryptocurrency trading.

Your task is to analyze the provided technical indicators and market data, then provide a trading signal.

## Current Market Data

Symbol: {{.Symbol}}
Timeframe: {{.Timeframe}}
Current Price: ${{.CurrentPrice}}

## Technical Indicators

RSI(14): {{.RSI}}
MACD: {{.MACD.Value}} (Signal: {{.MACD.Signal}}, Histogram: {{.MACD.Histogram}})
Bollinger Bands: Upper={{.BB.Upper}}, Middle={{.BB.Middle}}, Lower={{.BB.Lower}}
EMA(50): {{.EMA50}}
EMA(200): {{.EMA200}}
ADX(14): {{.ADX}}
Volume(24h): {{.Volume24h}}

## Recent Price Action

{{.RecentPrices}}

## Task

Analyze these indicators and provide a trading signal. Respond in JSON format:

{
  "signal": "BUY" | "SELL" | "HOLD",
  "confidence": 0.0-1.0,
  "reasoning": "Detailed explanation of your analysis",
  "key_factors": ["factor1", "factor2", "factor3"],
  "risk_level": "LOW" | "MEDIUM" | "HIGH"
}

## Analysis Guidelines

1. RSI < 30 suggests oversold, RSI > 70 suggests overbought
2. MACD crossover indicates trend change
3. Price touching Bollinger Bands indicates volatility
4. EMA crossovers (golden cross/death cross) are significant
5. ADX > 25 indicates strong trend
6. Consider multiple timeframes for confirmation
7. Volume confirms price movements

Provide your analysis:`
```

**Example Usage**:
```go
package agents

import (
    "bytes"
    "text/template"
    "cryptofunk/internal/llm"
)

type TechnicalAgent struct {
    llmClient *llm.Client
    template  *template.Template
}

type TechnicalContext struct {
    Symbol       string
    Timeframe    string
    CurrentPrice float64
    RSI          float64
    MACD         MACDData
    BB           BollingerBands
    EMA50        float64
    EMA200       float64
    ADX          float64
    Volume24h    float64
    RecentPrices []float64
}

type TechnicalSignal struct {
    Signal     string   `json:"signal"`
    Confidence float64  `json:"confidence"`
    Reasoning  string   `json:"reasoning"`
    KeyFactors []string `json:"key_factors"`
    RiskLevel  string   `json:"risk_level"`
}

func (a *TechnicalAgent) Analyze(ctx TechnicalContext) (*TechnicalSignal, error) {
    // 1. Build prompt from template
    var prompt bytes.Buffer
    if err := a.template.Execute(&prompt, ctx); err != nil {
        return nil, err
    }

    // 2. Call LLM via Bifrost
    resp, err := a.llmClient.Chat(llm.ChatRequest{
        Model:       "claude-sonnet-4-20250514",
        Temperature: 0.7,
        MaxTokens:   2000,
        Messages: []llm.Message{
            {Role: "user", Content: prompt.String()},
        },
    })
    if err != nil {
        return nil, err
    }

    // 3. Parse JSON response
    var signal TechnicalSignal
    content := resp.Choices[0].Message.Content
    if err := json.Unmarshal([]byte(content), &signal); err != nil {
        return nil, fmt.Errorf("parse LLM response: %w", err)
    }

    return &signal, nil
}
```

### 4.2 Trend Following Agent

**Purpose**: Identify and trade trends using EMA crossovers and momentum

**Prompt Template**:
```go
const TrendFollowingPrompt = `You are an expert trend-following trader specializing in cryptocurrency markets.

Your strategy focuses on identifying strong trends and riding them with proper risk management.

## Current Market Data

Symbol: {{.Symbol}}
Timeframe: {{.Timeframe}}
Current Price: ${{.CurrentPrice}}

## Trend Indicators

EMA(50): {{.EMA50}}
EMA(200): {{.EMA200}}
EMA Relationship: {{.EMARelationship}}  # "bullish" | "bearish" | "neutral"
ADX(14): {{.ADX}}
Trend Strength: {{.TrendStrength}}  # "strong" | "weak" | "none"

## Recent Crossovers

{{.RecentCrossovers}}

## Current Position

{{if .HasPosition}}
Position: {{.PositionSide}} ({{.PositionSize}} {{.Symbol}})
Entry Price: ${{.EntryPrice}}
Current P&L: {{.PnLPercent}}%
{{else}}
No current position
{{end}}

## Task

Evaluate the trend and decide on the next action. Respond in JSON format:

{
  "action": "BUY" | "SELL" | "HOLD" | "CLOSE",
  "confidence": 0.0-1.0,
  "reasoning": "Detailed explanation focusing on trend analysis",
  "entry_price": null or number,
  "stop_loss": null or number,
  "take_profit": null or number,
  "trend_phase": "EMERGING" | "ESTABLISHED" | "MATURE" | "EXHAUSTED"
}

## Trend Following Rules

1. **Entry**: Only enter when EMA50 > EMA200 (bullish) and ADX > 25
2. **Exit**: Close when EMA50 crosses below EMA200 or stop-loss hit
3. **Stop Loss**: Place 2-3% below recent swing low
4. **Take Profit**: Trail stop as trend continues
5. **Trend Phases**:
   - EMERGING: Just crossed, low momentum
   - ESTABLISHED: Strong momentum, ADX rising
   - MATURE: High momentum, ADX > 40
   - EXHAUSTED: Momentum divergence, potential reversal

Provide your analysis:`
```

### 4.3 Mean Reversion Agent

**Purpose**: Trade oversold/overbought conditions in ranging markets

**Prompt Template**:
```go
const MeanReversionPrompt = `You are an expert mean reversion trader specializing in cryptocurrency markets.

Your strategy focuses on identifying extreme price deviations and trading the return to the mean.

## Current Market Data

Symbol: {{.Symbol}}
Timeframe: {{.Timeframe}}
Current Price: ${{.CurrentPrice}}

## Mean Reversion Indicators

RSI(14): {{.RSI}}
Bollinger Bands:
  - Upper: ${{.BB.Upper}}
  - Middle: ${{.BB.Middle}}
  - Lower: ${{.BB.Lower}}
  - %B: {{.BB.PercentB}}  # 0 = at lower band, 1 = at upper band
  - Width: {{.BB.Width}}  # Volatility measure

Market Regime: {{.MarketRegime}}  # "RANGING" | "TRENDING"
ADX(14): {{.ADX}}  # < 25 indicates ranging market

## Price Position

Distance from Mean: {{.DistanceFromMean}}%
Standard Deviations from Mean: {{.StdDevsFromMean}}

## Task

Evaluate mean reversion opportunity. Respond in JSON format:

{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": 0.0-1.0,
  "reasoning": "Detailed explanation of reversion setup",
  "target_profit": "1-3% quick profit target",
  "stop_loss": "Tight stop beyond extreme",
  "timeframe": "SHORT" | "MEDIUM",  # Expected time to reversion
  "risk_reward": number  # Risk/reward ratio
}

## Mean Reversion Rules

1. **Only Trade in Ranging Markets**: ADX < 25
2. **Entry Signals**:
   - RSI < 30 AND price < Lower BB → BUY
   - RSI > 70 AND price > Upper BB → SELL (short)
3. **Exit**: Price returns to middle BB or hits stop
4. **Stop Loss**: Tight, just beyond the extreme
5. **Target**: 1-3% quick profit, don't be greedy
6. **Avoid**: Strong trends (ADX > 25)

Provide your analysis:`
```

### 4.4 Risk Management Agent

**Purpose**: Final approval for all trades with position sizing and risk assessment

**Prompt Template**:
```go
const RiskManagementPrompt = `You are the Chief Risk Officer for a cryptocurrency trading operation.

Your responsibility is to evaluate every trade proposal and approve/reject based on risk management principles.

## Proposed Trade

Symbol: {{.Symbol}}
Action: {{.ProposedAction}}
Size: {{.ProposedSize}} {{.Symbol}}
Entry Price: ${{.EntryPrice}}
Stop Loss: ${{.StopLoss}}
Take Profit: ${{.TakeProfit}}
Confidence: {{.Confidence}}

## Portfolio State

Total Capital: ${{.TotalCapital}}
Available Capital: ${{.AvailableCapital}}
Current Positions: {{.PositionCount}}
Total Exposure: {{.TotalExposure}}% of capital
Current P&L: {{.CurrentPnL}}%
Max Drawdown (30d): {{.MaxDrawdown}}%

## Risk Metrics

Position Size: {{.PositionSizePercent}}% of capital
Risk Per Trade: {{.RiskPerTrade}}% of capital
Portfolio Risk: {{.PortfolioRisk}}% of capital
Correlation with existing positions: {{.Correlation}}

## Market Conditions

Volatility (30d): {{.Volatility}}
Market Regime: {{.MarketRegime}}
Recent Win Rate: {{.WinRate}}%

## Risk Limits

- Max Position Size: {{.MaxPositionSize}}% of capital
- Max Risk Per Trade: {{.MaxRiskPerTrade}}% of capital
- Max Portfolio Risk: {{.MaxPortfolioRisk}}% of capital
- Max Drawdown: {{.MaxDrawdownLimit}}%

## Task

Evaluate this trade and respond in JSON format:

{
  "approved": true | false,
  "adjusted_size": null or number,  # If size should be reduced
  "reasoning": "Detailed risk assessment",
  "risk_score": 0.0-1.0,  # 0 = very safe, 1 = very risky
  "recommendations": ["recommendation1", "recommendation2"],
  "warnings": ["warning1", "warning2"]  # If any concerns
}

## Risk Management Principles

1. **Never risk more than 2% of capital on a single trade**
2. **Portfolio risk should not exceed 10% of capital**
3. **Reduce position size if:**
   - Volatility is high
   - Recent drawdown > 10%
   - Low confidence signal
   - High correlation with existing positions
4. **Reject trade if:**
   - Exceeds risk limits
   - Insufficient capital
   - Stop loss too far (risk > 5%)
   - Recent large losses
5. **Position Sizing**: Use Kelly Criterion with fractional Kelly (25-50%)

Provide your risk assessment:`
```

---

## 5. Context Building

### 5.1 Context Builder Pattern

```go
package llm

import (
    "fmt"
    "strings"
    "time"
)

type ContextBuilder struct {
    sections []string
}

func NewContextBuilder() *ContextBuilder {
    return &ContextBuilder{
        sections: make([]string, 0),
    }
}

func (cb *ContextBuilder) AddSection(title, content string) {
    section := fmt.Sprintf("## %s\n\n%s", title, content)
    cb.sections = append(cb.sections, section)
}

func (cb *ContextBuilder) AddTable(title string, headers []string, rows [][]string) {
    var table strings.Builder
    table.WriteString(fmt.Sprintf("## %s\n\n", title))

    // Headers
    table.WriteString("| " + strings.Join(headers, " | ") + " |\n")
    table.WriteString("|" + strings.Repeat(" --- |", len(headers)) + "\n")

    // Rows
    for _, row := range rows {
        table.WriteString("| " + strings.Join(row, " | ") + " |\n")
    }

    cb.sections = append(cb.sections, table.String())
}

func (cb *ContextBuilder) AddTimeSeries(title string, data []TimeSeriesPoint) {
    var content strings.Builder
    content.WriteString(fmt.Sprintf("## %s\n\n", title))

    for _, point := range data {
        content.WriteString(fmt.Sprintf("- %s: $%.2f\n",
            point.Time.Format("15:04"), point.Value))
    }

    cb.sections = append(cb.sections, content.String())
}

func (cb *ContextBuilder) AddRecentDecisions(decisions []Decision) {
    var content strings.Builder
    content.WriteString("## Recent Decisions (Last 10)\n\n")

    for i, d := range decisions {
        outcome := ""
        if d.Outcome != nil {
            outcome = fmt.Sprintf(" → P&L: %.2f%%", d.Outcome.PnLPercent)
        }
        content.WriteString(fmt.Sprintf("%d. %s %s at $%.2f (Confidence: %.2f)%s\n",
            i+1, d.Action, d.Symbol, d.Price, d.Confidence, outcome))
    }

    cb.sections = append(cb.sections, content.String())
}

func (cb *ContextBuilder) Build() string {
    return strings.Join(cb.sections, "\n\n")
}

type TimeSeriesPoint struct {
    Time  time.Time
    Value float64
}

type Decision struct {
    Symbol     string
    Action     string
    Price      float64
    Confidence float64
    Outcome    *Outcome
}

type Outcome struct {
    PnLPercent float64
}
```

### 5.2 Example: Building Context for Technical Agent

```go
func (a *TechnicalAgent) buildContext(symbol string) (string, error) {
    cb := llm.NewContextBuilder()

    // 1. Current market state
    price, err := a.dataClient.GetCurrentPrice(symbol)
    if err != nil {
        return "", err
    }

    cb.AddSection("Current Market", fmt.Sprintf(`
Symbol: %s
Current Price: $%.2f
Timestamp: %s
`, symbol, price, time.Now().Format(time.RFC3339)))

    // 2. Technical indicators
    indicators, err := a.calculateIndicators(symbol)
    if err != nil {
        return "", err
    }

    cb.AddTable("Technical Indicators",
        []string{"Indicator", "Value", "Signal"},
        [][]string{
            {"RSI(14)", fmt.Sprintf("%.2f", indicators.RSI), indicators.RSISignal},
            {"MACD", fmt.Sprintf("%.2f", indicators.MACD), indicators.MACDSignal},
            {"ADX(14)", fmt.Sprintf("%.2f", indicators.ADX), indicators.ADXSignal},
        },
    )

    // 3. Recent price action
    prices, err := a.dataClient.GetRecentPrices(symbol, 10)
    if err != nil {
        return "", err
    }

    points := make([]llm.TimeSeriesPoint, len(prices))
    for i, p := range prices {
        points[i] = llm.TimeSeriesPoint{
            Time:  p.Timestamp,
            Value: p.Close,
        }
    }
    cb.AddTimeSeries("Recent Prices (Last 10 candles)", points)

    // 4. Past decisions (for learning)
    decisions, err := a.getRecentDecisions(symbol, 10)
    if err == nil && len(decisions) > 0 {
        cb.AddRecentDecisions(decisions)
    }

    return cb.Build(), nil
}
```

---

## 6. Decision Flow

### 6.1 Multi-Agent Decision Pipeline

```
┌─────────────────────────────────────────────────────┐
│                 Market Event                        │
│          (Price update, new candle, etc.)           │
└──────────────────┬──────────────────────────────────┘
                   │
                   ↓
┌──────────────────────────────────────────────────────┐
│         ANALYSIS AGENTS (Parallel)                   │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────┐ │
│  │  Technical   │  │  Order Book  │  │ Sentiment │ │
│  │  Analysis    │  │  Analysis    │  │  Analysis │ │
│  │  Agent       │  │  Agent       │  │  Agent    │ │
│  │  (LLM)       │  │  (LLM)       │  │  (LLM)    │ │
│  └──────┬───────┘  └──────┬───────┘  └─────┬─────┘ │
│         │                 │                 │       │
│         └─────────────────┼─────────────────┘       │
└───────────────────────────┼─────────────────────────┘
                            │
                            ↓ Signals with reasoning
┌──────────────────────────────────────────────────────┐
│              MCP ORCHESTRATOR                        │
│                                                      │
│  1. Collect signals from all analysis agents        │
│  2. Aggregate confidence scores                     │
│  3. Check for consensus (e.g., 2/3 agents agree)    │
│  4. If consensus → forward to strategy agents       │
└──────────────────────────────────────────────────────┘
                            │
                            ↓ Aggregated analysis
┌──────────────────────────────────────────────────────┐
│         STRATEGY AGENTS (Parallel)                   │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐                │
│  │  Trend       │  │  Mean        │                │
│  │  Following   │  │  Reversion   │                │
│  │  Agent       │  │  Agent       │                │
│  │  (LLM)       │  │  (LLM)       │                │
│  └──────┬───────┘  └──────┬───────┘                │
│         │                 │                         │
│         └─────────────────┘                         │
└──────────────────┬────────────────────────────────────┘
                   │
                   ↓ Trade proposals
┌──────────────────────────────────────────────────────┐
│           MCP ORCHESTRATOR                           │
│                                                      │
│  1. Select best trade proposal                      │
│     - Highest confidence                            │
│     - Best risk/reward                              │
│  2. Forward to risk agent for approval              │
└──────────────────────────────────────────────────────┘
                   │
                   ↓ Trade proposal
┌──────────────────────────────────────────────────────┐
│              RISK AGENT (LLM)                        │
│                                                      │
│  1. Evaluate position size                          │
│  2. Check risk limits                               │
│  3. Assess portfolio risk                           │
│  4. APPROVE or REJECT with detailed reasoning       │
└──────────────────┬────────────────────────────────────┘
                   │
                   ↓ Approved trade
┌──────────────────────────────────────────────────────┐
│            EXECUTION AGENT                           │
│                                                      │
│  1. Place order via CCXT                            │
│  2. Monitor fill status                             │
│  3. Store trade in database                         │
│  4. Send notification                               │
└──────────────────────────────────────────────────────┘
```

### 6.2 Orchestrator Implementation

```go
package orchestrator

import (
    "context"
    "fmt"
    "sync"
)

type Orchestrator struct {
    analysisAgents  []Agent
    strategyAgents  []Agent
    riskAgent       Agent
    executionAgent  Agent
    consensusThreshold float64
}

type AgentDecision struct {
    AgentName  string
    Signal     string  // "BUY" | "SELL" | "HOLD"
    Confidence float64
    Reasoning  string
}

func (o *Orchestrator) ProcessMarketEvent(ctx context.Context, event MarketEvent) error {
    // 1. Run analysis agents in parallel
    analysisDecisions := o.runAnalysisAgentsParallel(ctx, event)

    // 2. Check for consensus
    consensus, aggregated := o.checkConsensus(analysisDecisions)
    if !consensus {
        return nil  // No consensus, do nothing
    }

    // 3. Run strategy agents with aggregated analysis
    strategyDecisions := o.runStrategyAgentsParallel(ctx, aggregated)

    // 4. Select best strategy
    bestStrategy := o.selectBestStrategy(strategyDecisions)
    if bestStrategy == nil {
        return nil  // No strong strategy signal
    }

    // 5. Risk approval
    riskDecision := o.riskAgent.Evaluate(ctx, *bestStrategy)
    if !riskDecision.Approved {
        log.Info("Trade rejected by risk agent: %s", riskDecision.Reasoning)
        return nil
    }

    // 6. Execute trade
    return o.executionAgent.Execute(ctx, *bestStrategy, riskDecision)
}

func (o *Orchestrator) runAnalysisAgentsParallel(ctx context.Context, event MarketEvent) []AgentDecision {
    var wg sync.WaitGroup
    decisions := make([]AgentDecision, len(o.analysisAgents))

    for i, agent := range o.analysisAgents {
        wg.Add(1)
        go func(idx int, ag Agent) {
            defer wg.Done()
            decision, err := ag.Analyze(ctx, event)
            if err != nil {
                log.Error("Agent %s failed: %v", ag.Name(), err)
                return
            }
            decisions[idx] = decision
        }(i, agent)
    }

    wg.Wait()
    return decisions
}

func (o *Orchestrator) checkConsensus(decisions []AgentDecision) (bool, AggregatedAnalysis) {
    // Count votes for each signal
    votes := make(map[string]int)
    totalConfidence := make(map[string]float64)

    for _, d := range decisions {
        votes[d.Signal]++
        totalConfidence[d.Signal] += d.Confidence
    }

    // Find majority signal
    var majoritySignal string
    var maxVotes int
    for signal, count := range votes {
        if count > maxVotes {
            maxVotes = count
            majoritySignal = signal
        }
    }

    // Check if meets consensus threshold
    consensusRatio := float64(maxVotes) / float64(len(decisions))
    if consensusRatio < o.consensusThreshold {
        return false, AggregatedAnalysis{}
    }

    // Calculate average confidence for majority signal
    avgConfidence := totalConfidence[majoritySignal] / float64(maxVotes)

    return true, AggregatedAnalysis{
        Signal:     majoritySignal,
        Confidence: avgConfidence,
        Decisions:  decisions,
    }
}
```

---

## 7. Explainability

### 7.1 Decision Logging

Every decision is logged with full reasoning:

```go
type DecisionLog struct {
    ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    Symbol       string    `json:"symbol"`
    AgentName    string    `json:"agent_name"`
    AgentType    string    `json:"agent_type"`  // "analysis" | "strategy" | "risk"

    // Decision
    Action       string    `json:"action"`      // "BUY" | "SELL" | "HOLD"
    Confidence   float64   `json:"confidence"`

    // LLM Details
    LLMProvider  string    `json:"llm_provider"` // "claude" | "gpt-4"
    LLMModel     string    `json:"llm_model"`
    PromptTokens int       `json:"prompt_tokens"`
    ResponseTokens int     `json:"response_tokens"`
    Latency      int       `json:"latency_ms"`

    // Reasoning (Natural Language)
    Reasoning    string    `json:"reasoning"`
    KeyFactors   []string  `json:"key_factors"`

    // Context
    MarketData   json.RawMessage `json:"market_data"`
    Indicators   json.RawMessage `json:"indicators"`

    // Outcome (filled later)
    Outcome      *DecisionOutcome `json:"outcome,omitempty"`
}

type DecisionOutcome struct {
    Executed     bool      `json:"executed"`
    ExecutedAt   time.Time `json:"executed_at,omitempty"`
    EntryPrice   float64   `json:"entry_price,omitempty"`
    ExitPrice    float64   `json:"exit_price,omitempty"`
    PnL          float64   `json:"pnl,omitempty"`
    PnLPercent   float64   `json:"pnl_percent,omitempty"`
    HoldTime     int       `json:"hold_time_seconds,omitempty"`
}
```

### 7.2 Explainability Dashboard

```
┌─────────────────────────────────────────────────────────┐
│              Decision Explainability View               │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Decision ID: dec_2025_10_27_123456                    │
│  Timestamp: 2025-10-27 12:34:56 UTC                    │
│  Symbol: BTCUSDT                                       │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │ DECISION: BUY                                     │ │
│  │ Confidence: 0.85 (High)                           │ │
│  │ Agent: Trend Following Agent                      │ │
│  │ LLM: Claude Sonnet 4                              │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  REASONING:                                            │
│  ┌───────────────────────────────────────────────────┐ │
│  │ The market is showing a strong bullish trend with │ │
│  │ multiple confirming signals:                      │ │
│  │                                                   │ │
│  │ 1. Golden Cross: EMA(50) crossed above EMA(200)  │ │
│  │    3 days ago and has been holding above since.  │ │
│  │                                                   │ │
│  │ 2. Strong Momentum: ADX is at 32, indicating a   │ │
│  │    strong trending market (above our threshold   │ │
│  │    of 25).                                        │ │
│  │                                                   │ │
│  │ 3. Volume Confirmation: Trading volume is 25%    │ │
│  │    above the 30-day average, confirming the      │ │
│  │    strength of the move.                         │ │
│  │                                                   │ │
│  │ 4. No Divergence: Price and RSI are both making  │ │
│  │    higher highs, indicating healthy momentum.    │ │
│  │                                                   │ │
│  │ Risk/Reward: Favorable at 1:3 with stop loss at  │ │
│  │ $41,200 and target at $45,000.                   │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  KEY FACTORS:                                          │
│  • Golden Cross (EMA50 > EMA200)                       │
│  • High ADX (32)                                       │
│  • Volume confirmation (+25%)                          │
│  • No bearish divergence                               │
│                                                         │
│  MARKET CONTEXT:                                       │
│  Current Price: $42,150                                │
│  EMA(50): $41,800                                      │
│  EMA(200): $40,500                                     │
│  RSI: 62 (Neutral)                                     │
│  ADX: 32 (Strong Trend)                                │
│                                                         │
│  RISK ASSESSMENT (from Risk Agent):                    │
│  ✓ Approved                                            │
│  Position Size: 2.5% of portfolio ($2,500)             │
│  Risk: 1.8% of portfolio ($450)                        │
│  Risk/Reward: 1:3                                      │
│                                                         │
│  OUTCOME:                                              │
│  Status: EXECUTED                                      │
│  Entry: $42,150 @ 2025-10-27 12:35:00                 │
│  Current P&L: +3.2% ($80)                              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 8. Memory & Learning

### 8.1 Decision History Storage

```sql
-- Store all decisions with outcomes for learning
CREATE TABLE agent_decisions (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    agent_name VARCHAR(100) NOT NULL,
    agent_type VARCHAR(50) NOT NULL,

    -- Decision
    action VARCHAR(10) NOT NULL,
    confidence DECIMAL(3,2) NOT NULL,

    -- LLM tracking
    llm_provider VARCHAR(50) NOT NULL,
    llm_model VARCHAR(100) NOT NULL,
    prompt_tokens INTEGER NOT NULL,
    response_tokens INTEGER NOT NULL,
    latency_ms INTEGER NOT NULL,
    cost_usd DECIMAL(10,6),

    -- Reasoning
    reasoning TEXT NOT NULL,
    key_factors TEXT[] NOT NULL,

    -- Context (JSONB for querying)
    market_data JSONB NOT NULL,
    indicators JSONB NOT NULL,

    -- Outcome (filled when position closes)
    executed BOOLEAN DEFAULT FALSE,
    executed_at TIMESTAMPTZ,
    entry_price DECIMAL(20,8),
    exit_price DECIMAL(20,8),
    pnl DECIMAL(20,8),
    pnl_percent DECIMAL(10,4),
    hold_time_seconds INTEGER
);

-- Index for fast lookups
CREATE INDEX idx_decisions_timestamp ON agent_decisions(timestamp DESC);
CREATE INDEX idx_decisions_symbol_agent ON agent_decisions(symbol, agent_name);
CREATE INDEX idx_decisions_outcome ON agent_decisions(executed, pnl_percent);

-- Convert to TimescaleDB hypertable for efficient time-series queries
SELECT create_hypertable('agent_decisions', 'timestamp');
```

### 8.2 Learning from Past Decisions

```go
package memory

import (
    "context"
    "database/sql"
    "fmt"
)

type DecisionMemory struct {
    db *sql.DB
}

// GetSimilarDecisions retrieves past decisions in similar market conditions
func (dm *DecisionMemory) GetSimilarDecisions(ctx context.Context,
    symbol string, agentName string, limit int) ([]DecisionLog, error) {

    query := `
        SELECT id, timestamp, action, confidence, reasoning,
               key_factors, pnl_percent, market_data, indicators
        FROM agent_decisions
        WHERE symbol = $1
          AND agent_name = $2
          AND executed = TRUE
          AND pnl_percent IS NOT NULL
        ORDER BY timestamp DESC
        LIMIT $3
    `

    rows, err := dm.db.QueryContext(ctx, query, symbol, agentName, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var decisions []DecisionLog
    for rows.Next() {
        var d DecisionLog
        if err := rows.Scan(&d.ID, &d.Timestamp, &d.Action, &d.Confidence,
            &d.Reasoning, &d.KeyFactors, &d.Outcome.PnLPercent,
            &d.MarketData, &d.Indicators); err != nil {
            return nil, err
        }
        decisions = append(decisions, d)
    }

    return decisions, nil
}

// GetPerformanceStats calculates agent performance metrics
func (dm *DecisionMemory) GetPerformanceStats(ctx context.Context,
    agentName string, days int) (*PerformanceStats, error) {

    query := `
        SELECT
            COUNT(*) as total_decisions,
            COUNT(CASE WHEN executed THEN 1 END) as executed_trades,
            AVG(CASE WHEN pnl_percent > 0 THEN 1.0 ELSE 0.0 END) as win_rate,
            AVG(pnl_percent) as avg_pnl_percent,
            MAX(pnl_percent) as max_win,
            MIN(pnl_percent) as max_loss,
            AVG(confidence) as avg_confidence,
            SUM(cost_usd) as total_llm_cost
        FROM agent_decisions
        WHERE agent_name = $1
          AND timestamp > NOW() - INTERVAL '$2 days'
    `

    var stats PerformanceStats
    err := dm.db.QueryRowContext(ctx, query, agentName, days).Scan(
        &stats.TotalDecisions,
        &stats.ExecutedTrades,
        &stats.WinRate,
        &stats.AvgPnLPercent,
        &stats.MaxWin,
        &stats.MaxLoss,
        &stats.AvgConfidence,
        &stats.TotalLLMCost,
    )

    return &stats, err
}

type PerformanceStats struct {
    TotalDecisions int
    ExecutedTrades int
    WinRate        float64
    AvgPnLPercent  float64
    MaxWin         float64
    MaxLoss        float64
    AvgConfidence  float64
    TotalLLMCost   float64
}
```

### 8.3 Context Enhancement with Past Decisions

```go
// Add performance context to LLM prompts
func (a *TechnicalAgent) enhanceContextWithMemory(ctx string) (string, error) {
    // Get recent performance
    stats, err := a.memory.GetPerformanceStats(context.Background(),
        "technical-agent", 30)
    if err != nil {
        return ctx, nil  // Non-fatal, continue without stats
    }

    // Get similar past decisions
    similar, err := a.memory.GetSimilarDecisions(context.Background(),
        a.symbol, "technical-agent", 5)
    if err != nil {
        return ctx, nil
    }

    // Build memory section
    memoryContext := fmt.Sprintf(`

## Your Recent Performance (Last 30 days)

Win Rate: %.1f%%
Average P&L: %.2f%%
Total Trades: %d

## Similar Past Decisions

`, stats.WinRate*100, stats.AvgPnLPercent, stats.ExecutedTrades)

    for i, d := range similar {
        memoryContext += fmt.Sprintf(`
%d. %s at $%.2f (Confidence: %.2f)
   Outcome: %.2f%% P&L
   Reasoning: %s

`, i+1, d.Action, d.EntryPrice, d.Confidence,
   d.Outcome.PnLPercent, d.Reasoning)
    }

    memoryContext += `
Consider these past decisions when making your current analysis. Learn from what worked and what didn't.
`

    return ctx + memoryContext, nil
}
```

---

## 9. Error Handling & Fallbacks

### 9.1 Multi-Layer Fallback Strategy

```
┌─────────────────────────────────────────┐
│     Agent Makes Decision Request        │
└──────────────┬──────────────────────────┘
               │
               ↓
┌──────────────────────────────────────────┐
│  Layer 1: Bifrost (Automatic)           │
│  - Try Claude Sonnet 4                  │
│  - If fail → GPT-4 Turbo                │
│  - If fail → Gemini Pro                 │
│  - Timeout: 30s per attempt             │
└──────────────┬──────────────────────────┘
               │
               ↓ All LLM providers failed
┌──────────────────────────────────────────┐
│  Layer 2: Cached Response               │
│  - Check Redis for similar prompt       │
│  - If found → use cached decision       │
│  - Mark as "cached" in logs             │
└──────────────┬──────────────────────────┘
               │
               ↓ No cache hit
┌──────────────────────────────────────────┐
│  Layer 3: Rule-Based Fallback          │
│  - Use simple technical rules           │
│  - e.g., RSI < 30 → BUY                 │
│  - Low confidence (0.3)                 │
│  - Mark as "fallback" in logs           │
└──────────────┬──────────────────────────┘
               │
               ↓ Rules can't decide
┌──────────────────────────────────────────┐
│  Layer 4: Do Nothing                    │
│  - Return HOLD signal                   │
│  - Log error and alert                  │
│  - Continue monitoring                  │
└──────────────────────────────────────────┘
```

### 9.2 Implementation

```go
package agents

import (
    "context"
    "errors"
    "time"
)

type AgentWithFallback struct {
    llmClient     *llm.Client
    cache         *redis.Client
    fallbackRules *RuleEngine
    alerter       *Alerter
}

func (a *AgentWithFallback) MakeDecision(ctx context.Context) (*Decision, error) {
    // Layer 1: Try LLM via Bifrost (automatic multi-provider failover)
    decision, err := a.tryLLM(ctx)
    if err == nil {
        return decision, nil
    }

    log.Warn("LLM decision failed: %v, trying fallbacks", err)

    // Layer 2: Check cache
    cachedDecision, err := a.tryCache(ctx)
    if err == nil {
        cachedDecision.Source = "cache"
        cachedDecision.Confidence *= 0.8  // Reduce confidence for cached
        log.Info("Using cached decision")
        return cachedDecision, nil
    }

    // Layer 3: Rule-based fallback
    ruleDecision, err := a.tryRules(ctx)
    if err == nil {
        ruleDecision.Source = "rules"
        ruleDecision.Confidence = 0.3  // Low confidence for rules
        log.Info("Using rule-based fallback")
        a.alerter.Send("Agent using rule-based fallback")
        return ruleDecision, nil
    }

    // Layer 4: Do nothing
    log.Error("All fallbacks failed, returning HOLD")
    a.alerter.SendUrgent("Agent cannot make decisions - all systems down")

    return &Decision{
        Action:     "HOLD",
        Confidence: 0.0,
        Reasoning:  "All decision systems unavailable",
        Source:     "emergency",
    }, nil
}

func (a *AgentWithFallback) tryLLM(ctx context.Context) (*Decision, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Bifrost handles multi-provider failover automatically
    resp, err := a.llmClient.Chat(llm.ChatRequest{
        Model:       "claude-sonnet-4-20250514",  // Bifrost will failover if needed
        Messages:    a.buildMessages(),
        Temperature: 0.7,
        MaxTokens:   2000,
    })

    if err != nil {
        return nil, fmt.Errorf("LLM request failed: %w", err)
    }

    decision, err := a.parseResponse(resp)
    if err != nil {
        return nil, fmt.Errorf("parse LLM response: %w", err)
    }

    decision.Source = "llm:" + resp.Model
    return decision, nil
}

func (a *AgentWithFallback) tryCache(ctx context.Context) (*Decision, error) {
    // Generate cache key from current context
    contextKey := a.generateContextKey()

    // Look for similar cached decisions
    cached, err := a.cache.Get(ctx, "decision:"+contextKey).Result()
    if err != nil {
        return nil, err
    }

    var decision Decision
    if err := json.Unmarshal([]byte(cached), &decision); err != nil {
        return nil, err
    }

    return &decision, nil
}

func (a *AgentWithFallback) tryRules(ctx context.Context) (*Decision, error) {
    // Simple rule-based logic as last resort
    indicators, err := a.getIndicators()
    if err != nil {
        return nil, err
    }

    // Basic RSI strategy
    if indicators.RSI < 30 {
        return &Decision{
            Action:     "BUY",
            Confidence: 0.3,
            Reasoning:  "Fallback rule: RSI oversold (< 30)",
        }, nil
    }

    if indicators.RSI > 70 {
        return &Decision{
            Action:     "SELL",
            Confidence: 0.3,
            Reasoning:  "Fallback rule: RSI overbought (> 70)",
        }, nil
    }

    return nil, errors.New("no rule triggered")
}
```

---

## 10. Performance Optimization

### 10.1 Semantic Caching

**How it works**:

1. Generate embedding of prompt
2. Search for similar past prompts (cosine similarity > 0.95)
3. If found → return cached response (skip LLM call)
4. If not found → call LLM and cache response

**Benefits**:
- 90% cost reduction (cached responses are free)
- 100x faster (no LLM latency)
- Consistent responses for similar situations

**Example**:
```
Prompt 1: "Analyze BTCUSDT: RSI=32, MACD=100, EMA50=42000, EMA200=41000"
Prompt 2: "Analyze BTCUSDT: RSI=33, MACD=105, EMA50=42050, EMA200=41020"

Similarity: 0.97 → Cache hit! Return cached response
```

### 10.2 Prompt Optimization

**Strategies**:

1. **Token Reduction**:
   - Remove unnecessary context
   - Use abbreviations for repeated terms
   - Compress numeric data (round to 2 decimals)

2. **Structured Output**:
   - Request JSON format
   - Reduces parsing errors
   - Faster response parsing

3. **Temperature Tuning**:
   - Lower temp (0.5-0.7) for consistent decisions
   - Higher temp (0.8-0.9) for creative analysis

4. **Max Tokens**:
   - Set appropriate limits (1000-2000 tokens)
   - Faster responses
   - Lower costs

### 10.3 Parallel Agent Execution

```go
// Run analysis agents in parallel for speed
func (o *Orchestrator) runAnalysisAgentsParallel(ctx context.Context, event MarketEvent) []AgentDecision {
    // Use goroutines + channels for parallel execution
    decisions := make(chan AgentDecision, len(o.analysisAgents))

    for _, agent := range o.analysisAgents {
        go func(ag Agent) {
            decision, err := ag.Analyze(ctx, event)
            if err != nil {
                log.Error("Agent %s failed: %v", ag.Name(), err)
                return
            }
            decisions <- decision
        }(agent)
    }

    // Collect results with timeout
    timeout := time.After(10 * time.Second)
    result := make([]AgentDecision, 0, len(o.analysisAgents))

    for i := 0; i < len(o.analysisAgents); i++ {
        select {
        case d := <-decisions:
            result = append(result, d)
        case <-timeout:
            log.Warn("Agent timeout, proceeding with partial results")
            break
        }
    }

    return result
}
```

### 10.4 Cost Tracking

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    LLMCostsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_costs_total_usd",
            Help: "Total LLM API costs in USD",
        },
        []string{"provider", "model", "agent"},
    )

    LLMLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "llm_latency_seconds",
            Help:    "LLM API latency",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
        },
        []string{"provider", "model"},
    )

    LLMCacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_cache_hits_total",
            Help: "Number of cache hits",
        },
        []string{"agent"},
    )
)

// Track costs after each LLM call
func trackCosts(provider, model, agent string, inputTokens, outputTokens int) {
    // Pricing (approximate)
    var costPerInputToken, costPerOutputToken float64

    switch {
    case strings.Contains(model, "claude-sonnet-4"):
        costPerInputToken = 0.000003   // $3 per 1M tokens
        costPerOutputToken = 0.000015  // $15 per 1M tokens
    case strings.Contains(model, "gpt-4-turbo"):
        costPerInputToken = 0.000010   // $10 per 1M tokens
        costPerOutputToken = 0.000030  // $30 per 1M tokens
    case strings.Contains(model, "gemini-pro"):
        costPerInputToken = 0.0000005  // $0.50 per 1M tokens
        costPerOutputToken = 0.0000015 // $1.50 per 1M tokens
    }

    cost := float64(inputTokens)*costPerInputToken + float64(outputTokens)*costPerOutputToken

    LLMCostsTotal.WithLabelValues(provider, model, agent).Add(cost)
}
```

---

## 11. Migration Path to Custom Models

### 11.1 Hybrid Approach (Post-MVP)

Once we've collected sufficient trading data (3-6 months), we can train custom models while keeping LLMs:

```
┌──────────────────────────────────────────┐
│         Hybrid Agent Architecture        │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  Strategy Selection (LLM)          │ │
│  │  - Analyze market regime           │ │
│  │  - Select best strategy            │ │
│  │  - Provide reasoning               │ │
│  └─────────────┬──────────────────────┘ │
│                │                         │
│                ↓                         │
│    ┌───────────────────────┐            │
│    │                       │            │
│    ↓                       ↓            │
│  ┌─────────────┐    ┌──────────────┐   │
│  │  Custom RL  │    │   LLM-Based  │   │
│  │  Model      │    │   Strategy   │   │
│  │  (Fast)     │    │   (Smart)    │   │
│  └──────┬──────┘    └──────┬───────┘   │
│         │                  │            │
│         └──────────┬───────┘            │
│                    │                    │
│                    ↓                    │
│         ┌────────────────────┐         │
│         │  Ensemble Decision │         │
│         └────────────────────┘         │
└──────────────────────────────────────────┘
```

**Benefits**:
- **Speed**: Custom RL models for execution (microseconds)
- **Reasoning**: LLM for complex analysis and explanation
- **Cost**: RL models are free after training
- **Explainability**: LLM provides reasoning for ensemble decisions

### 11.2 Data Collection Strategy

While running LLM-powered MVP:

1. **Log everything**:
   - Market conditions
   - LLM decisions and reasoning
   - Execution details
   - Outcomes (P&L, hold time, etc.)

2. **Build training dataset**:
   - State: Market indicators, price action
   - Action: BUY/SELL/HOLD with size
   - Reward: P&L percentage
   - Expert labels: LLM reasoning as supervision

3. **Metrics to collect**:
   - 10,000+ decisions (3-6 months)
   - Various market regimes (trending, ranging, volatile)
   - Win rate, Sharpe ratio, max drawdown

### 11.3 Model Training Pipeline

```python
# Train custom RL model using FinRL + collected data

from finrl import FinRLTrainer
import pandas as pd

# Load collected data
decisions = pd.read_sql("SELECT * FROM agent_decisions WHERE executed = TRUE", conn)

# Prepare training data
states = prepare_states(decisions)
actions = decisions['action'].values
rewards = decisions['pnl_percent'].values

# Train PPO model
trainer = FinRLTrainer(
    algorithm="PPO",
    state_dim=len(states[0]),
    action_dim=3,  # BUY, SELL, HOLD
)

model = trainer.train(
    states=states,
    actions=actions,
    rewards=rewards,
    episodes=10000,
)

# Export to ONNX for Go inference
import torch.onnx
torch.onnx.export(
    model,
    sample_input,
    "models/custom_rl_model.onnx",
)
```

### 11.4 Timeline

- **Weeks 1-10**: LLM-powered MVP development
- **Weeks 11-26**: Run MVP in paper trading, collect data
- **Weeks 27-30**: Train custom RL models on collected data
- **Weeks 31+**: Hybrid deployment (RL + LLM)

---

## Conclusion

This LLM-powered agent architecture provides:

✅ **Fast MVP**: 9.5 weeks vs 12+ weeks with custom models
✅ **Sophisticated Reasoning**: Claude/GPT-4 quality from day one
✅ **Explainability**: Natural language reasoning for all decisions
✅ **Resilience**: Automatic failover between LLM providers
✅ **Cost Optimization**: 90% cost reduction with semantic caching
✅ **Migration Path**: Clear path to custom models with collected data

**Next Steps**:
1. Implement Bifrost gateway deployment
2. Create prompt templates for each agent
3. Build context builder and response parser
4. Deploy in paper trading mode
5. Collect data for 3-6 months
6. Train custom models (future phase)

---

## References

- [Bifrost Documentation](https://docs.getbifrost.ai)
- [Anthropic Claude API](https://docs.anthropic.com)
- [OpenAI GPT-4 API](https://platform.openai.com/docs)
- [MCP Specification](https://modelcontextprotocol.io)
- [CCXT Documentation](https://docs.ccxt.com)
- [FinRL Framework](https://github.com/AI4Finance-Foundation/FinRL)
