# LLM Decision Tracking (T183)

## Overview

This document describes the decision history tracking system for LLM-powered agents. The system enables agents to log all decisions to PostgreSQL for learning, explainability, and performance analysis.

## Architecture

```
Agent LLM Decision
        ↓
DecisionTracker.TrackDecision()
        ↓
PostgreSQL (llm_decisions table)
        ↓
Later: Update outcome & P&L
        ↓
Learning & Analytics
```

## Database Schema

The `llm_decisions` table (already created in `migrations/001_initial_schema.sql`):

```sql
CREATE TABLE llm_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id),
    decision_type VARCHAR(50) NOT NULL,  -- 'signal', 'risk_approval', etc.
    symbol VARCHAR(20) NOT NULL,
    prompt TEXT NOT NULL,
    prompt_embedding vector(1536),  -- For semantic search
    response TEXT NOT NULL,
    model VARCHAR(100) NOT NULL,
    tokens_used INTEGER,
    latency_ms INTEGER,
    outcome VARCHAR(20),  -- 'SUCCESS', 'FAILURE', 'PENDING'
    pnl DECIMAL(20, 8),   -- Profit/Loss
    context JSONB,        -- Market conditions, indicators, etc.
    agent_name VARCHAR(100) NOT NULL,
    confidence DECIMAL(5, 4),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

**Indexes**:
- By session, symbol, decision_type, outcome
- **Vector similarity index** for finding similar past decisions

## Components

### 1. Database Layer (`internal/db/llm_decisions.go`)

**Key Functions**:
```go
// Insert a new decision
func (db *DB) InsertLLMDecision(ctx context.Context, decision *LLMDecision) error

// Update outcome when known
func (db *DB) UpdateLLMDecisionOutcome(ctx context.Context, id uuid.UUID, outcome string, pnl float64) error

// Query functions
func (db *DB) GetLLMDecisionsByAgent(ctx context.Context, agentName string, limit int) ([]*LLMDecision, error)
func (db *DB) GetSuccessfulLLMDecisions(ctx context.Context, agentName string, limit int) ([]*LLMDecision, error)
func (db *DB) GetLLMDecisionStats(ctx context.Context, agentName string, since time.Time) (map[string]interface{}, error)

// Find similar past decisions (for T185)
func (db *DB) FindSimilarDecisions(ctx context.Context, symbol string, contextJSON []byte, limit int) ([]*LLMDecision, error)
```

### 2. Decision Tracker (`internal/llm/tracker.go`)

**Main Methods**:
```go
// Generic tracking
func (dt *DecisionTracker) TrackDecision(
    ctx context.Context,
    agentName string,
    decisionType string,
    symbol string,
    prompt string,
    response string,
    model string,
    tokensUsed int,
    latencyMs int,
    confidence float64,
    contextData map[string]interface{},
    sessionID *uuid.UUID,
) (uuid.UUID, error)

// Convenience methods
func (dt *DecisionTracker) TrackSignalDecision(...) (uuid.UUID, error)
func (dt *DecisionTracker) TrackRiskDecision(...) (uuid.UUID, error)

// Update after trade outcome known
func (dt *DecisionTracker) UpdateDecisionOutcome(
    ctx context.Context,
    decisionID uuid.UUID,
    outcome string,
    pnl float64,
) error

// Analytics
func (dt *DecisionTracker) GetDecisionStats(ctx context.Context, agentName string, since time.Time) (map[string]interface{}, error)
```

## Integration Guide

### Option 1: Simple Logging (Agents without DB access)

Agents can log decisions via NATS for a separate service to persist:

```go
// In agent's generateSignalWithLLM method:
decisionData := map[string]interface{}{
    "agent_name":   "technical-agent",
    "decision_type": "signal",
    "symbol":       symbol,
    "prompt":       prompt,
    "response":     response,
    "model":        model,
    "confidence":   confidence,
    "timestamp":    time.Now(),
}

data, _ := json.Marshal(decisionData)
natsConn.Publish("llm.decisions", data)
```

### Option 2: Direct DB Tracking (Agents with DB access)

Agents with database connections can track directly:

```go
// 1. Add to agent struct
type TechnicalAgent struct {
    *agents.BaseAgent
    llmClient       *llm.Client
    decisionTracker *llm.DecisionTracker // Add this
    // ...
}

// 2. Initialize in constructor (if database available)
func NewTechnicalAgent(database *db.DB, ...) (*TechnicalAgent, error) {
    var tracker *llm.DecisionTracker
    if database != nil {
        tracker = llm.NewDecisionTracker(database)
    }

    return &TechnicalAgent{
        // ...
        decisionTracker: tracker,
    }
}

// 3. Track in generateSignalWithLLM
func (a *TechnicalAgent) generateSignalWithLLM(ctx context.Context, ...) (*TechnicalSignal, error) {
    startTime := time.Now()

    // Call LLM
    response, err := a.llmClient.CompleteWithRetry(ctx, messages, 2)
    latencyMs := int(time.Since(startTime).Milliseconds())

    // Parse response
    var llmSignal llm.Signal
    a.llmClient.ParseJSONResponse(response.Choices[0].Message.Content, &llmSignal)

    // Track decision (if tracker available)
    if a.decisionTracker != nil {
        contextData := map[string]interface{}{
            "indicators": indicatorMap,
            "current_price": currentPrice,
        }

        decisionID, err := a.decisionTracker.TrackSignalDecision(
            ctx,
            a.GetName(),
            &llmSignal,
            systemPrompt + "\n\n" + userPrompt,
            response.Choices[0].Message.Content,
            response.Model,
            response.Usage.CompletionTokens,
            latencyMs,
            marketCtx,
            nil, // sessionID (if available)
        )

        if err != nil {
            log.Warn().Err(err).Msg("Failed to track decision")
        } else {
            // Store decision ID for later outcome update
            signal.DecisionID = decisionID
        }
    }

    return signal, nil
}

// 4. Update outcome after trade resolves
func (a *TechnicalAgent) updateTradeOutcome(ctx context.Context, decisionID uuid.UUID, success bool, pnl float64) {
    if a.decisionTracker == nil {
        return
    }

    outcome := "SUCCESS"
    if !success {
        outcome = "FAILURE"
    }

    err := a.decisionTracker.UpdateDecisionOutcome(ctx, decisionID, outcome, pnl)
    if err != nil {
        log.Error().Err(err).Msg("Failed to update decision outcome")
    }
}
```

## Analytics & Learning

### Get Decision Stats

```go
// Get agent performance over last 24 hours
stats, err := tracker.GetDecisionStats(
    ctx,
    "technical-agent",
    time.Now().Add(-24*time.Hour),
)

// stats contains:
// - total_decisions: 145
// - successful: 87
// - failed: 43
// - pending: 15
// - success_rate: 60.0%
// - avg_pnl: 125.50
// - total_pnl: 5,400.00
// - avg_latency_ms: 1850
// - avg_tokens_used: 1250
// - avg_confidence: 0.78
```

### Find Successful Patterns

```go
// Get top 10 successful decisions for learning
successfulDecisions, err := tracker.GetSuccessfulDecisions(ctx, "technical-agent", 10)

for _, decision := range successfulDecisions {
    fmt.Printf("Successful trade on %s: P&L $%.2f\n", decision.Symbol, *decision.PnL)
    fmt.Printf("  Reasoning: %s\n", decision.Response)
    fmt.Printf("  Context: %s\n", decision.Context)
}
```

### Find Similar Past Decisions (T185)

```go
// Find decisions with similar market conditions
currentContext := map[string]interface{}{
    "rsi": 62.5,
    "macd": 125.45,
    "trend": "uptrend",
}

similarDecisions, err := tracker.FindSimilarDecisions(ctx, "BTC/USDT", currentContext, 5)

// Use in prompt:
// "In similar situations (RSI ~62, uptrend), we previously decided:
//  - 3 BUY signals with avg P&L of $450
//  - 1 HOLD signal
//  - Success rate: 75%"
```

## Benefits

1. **Explainability**: Every LLM decision logged with full context
2. **Learning**: Analyze successful vs failed decisions
3. **Performance Tracking**: Monitor LLM latency, costs, accuracy
4. **A/B Testing**: Compare different models (Claude vs GPT-4)
5. **Similar Situations**: Learn from past outcomes
6. **Debugging**: Reproduce exact decision conditions
7. **Audit Trail**: Complete history for compliance

## Performance Considerations

### Async Tracking

To avoid blocking agent decisions:

```go
// Track asynchronously
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    tracker.TrackDecision(ctx, ...)
}()
```

### Batch Inserts (Future Optimization)

For high-frequency agents, batch insertions:

```go
type DecisionBuffer struct {
    decisions []*db.LLMDecision
    mutex     sync.Mutex
}

func (buf *DecisionBuffer) Flush(ctx context.Context, db *db.DB) error {
    buf.mutex.Lock()
    defer buf.mutex.Unlock()

    // Batch insert all buffered decisions
    // ... implementation
}
```

## Monitoring

### Key Metrics to Track

```sql
-- Decision volume by agent
SELECT agent_name, COUNT(*), AVG(confidence)
FROM llm_decisions
WHERE created_at > NOW() - INTERVAL '1 hour'
GROUP BY agent_name;

-- Success rate by agent
SELECT
    agent_name,
    COUNT(*) as total,
    SUM(CASE WHEN outcome = 'SUCCESS' THEN 1 ELSE 0 END) as successes,
    AVG(CASE WHEN outcome = 'SUCCESS' THEN 1.0 ELSE 0.0 END) * 100 as success_rate,
    AVG(pnl) as avg_pnl,
    SUM(pnl) as total_pnl
FROM llm_decisions
WHERE created_at > NOW() - INTERVAL '24 hours'
  AND outcome IS NOT NULL
GROUP BY agent_name;

-- Average latency by model
SELECT
    model,
    COUNT(*) as decisions,
    AVG(latency_ms) as avg_latency,
    AVG(tokens_used) as avg_tokens,
    MAX(latency_ms) as max_latency
FROM llm_decisions
WHERE created_at > NOW() - INTERVAL '1 hour'
GROUP BY model
ORDER BY avg_latency DESC;

-- Confidence vs Outcome correlation
SELECT
    CASE
        WHEN confidence >= 0.9 THEN 'Very High (>0.9)'
        WHEN confidence >= 0.75 THEN 'High (0.75-0.9)'
        WHEN confidence >= 0.6 THEN 'Medium (0.6-0.75)'
        ELSE 'Low (<0.6)'
    END as confidence_bucket,
    COUNT(*) as total,
    AVG(CASE WHEN outcome = 'SUCCESS' THEN 1.0 ELSE 0.0 END) * 100 as success_rate,
    AVG(pnl) as avg_pnl
FROM llm_decisions
WHERE outcome IS NOT NULL
GROUP BY confidence_bucket
ORDER BY confidence_bucket;
```

## Testing

```go
func TestDecisionTracking(t *testing.T) {
    // Setup
    database := setupTestDatabase(t)
    tracker := llm.NewDecisionTracker(database)

    // Track a decision
    decisionID, err := tracker.TrackSignalDecision(
        context.Background(),
        "test-agent",
        &llm.Signal{
            Symbol: "BTC/USDT",
            Side: "BUY",
            Confidence: 0.85,
            Reasoning: "Test decision",
        },
        "Test prompt",
        "Test response",
        "claude-sonnet-4",
        1250,
        1850,
        llm.MarketContext{CurrentPrice: 50000},
        nil,
    )
    require.NoError(t, err)
    require.NotEqual(t, uuid.Nil, decisionID)

    // Update outcome
    err = tracker.UpdateDecisionOutcome(context.Background(), decisionID, "SUCCESS", 250.50)
    require.NoError(t, err)

    // Verify
    decisions, err := tracker.GetRecentDecisions(context.Background(), "test-agent", 10)
    require.NoError(t, err)
    require.Len(t, decisions, 1)
    require.Equal(t, "SUCCESS", *decisions[0].Outcome)
    require.Equal(t, 250.50, *decisions[0].PnL)
}
```

## Roadmap

### Completed (T183)
- ✅ Database schema with pgvector support
- ✅ Database layer (`internal/db/llm_decisions.go`)
- ✅ Decision tracker (`internal/llm/tracker.go`)
- ✅ Query functions for analytics
- ✅ Update outcome mechanism
- ✅ Statistics aggregation

### Upcoming
- [ ] **T184**: Context builder for better prompts
- [ ] **T185**: Similar situations retrieval using vector search
- [ ] **T188**: A/B testing framework
- [ ] Async tracking for performance
- [ ] Batch insert optimization
- [ ] Grafana dashboards for decision analytics
- [ ] Alerting on low success rates
- [ ] Automatic model selection based on past performance

## Example: Complete Flow

```go
// 1. Agent makes LLM decision
decisionID, _ := tracker.TrackSignalDecision(ctx, ...)

// 2. Trade is executed based on decision
order := executeOrder(signal)

// 3. Trade resolves (seconds/minutes/hours later)
if order.Status == "FILLED" {
    pnl := calculatePnL(order)
    outcome := "SUCCESS"
    if pnl < 0 {
        outcome = "FAILURE"
    }

    // 4. Update decision outcome
    tracker.UpdateDecisionOutcome(ctx, decisionID, outcome, pnl)
}

// 5. Analytics (daily/weekly)
stats, _ := tracker.GetDecisionStats(ctx, "technical-agent", time.Now().Add(-7*24*time.Hour))
log.Info().
    Int("total_decisions", stats["total_decisions"].(int)).
    Float64("success_rate", stats["success_rate"].(float64)).
    Float64("total_pnl", stats["total_pnl"].(float64)).
    Msg("Weekly agent performance")
```

---

**Status**: ✅ **T183 Complete** - Infrastructure ready for decision tracking
**Files Created**:
- `internal/db/llm_decisions.go` (300+ lines)
- `internal/llm/tracker.go` (200+ lines)
- `docs/LLM_DECISION_TRACKING.md` (this file)

**Next Steps**: Integrate into agents (shown above) and implement T184-T185 for enhanced learning
