# CryptoFunk Agent Architecture - Comprehensive Design

**Version:** 2.0
**Date:** 2025-10-27
**Status:** Design Review

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Agent Architecture Foundations](#2-agent-architecture-foundations)
3. [Memory Systems](#3-memory-systems)
4. [Agent Communication & Coordination](#4-agent-communication--coordination)
5. [Decision-Making Framework](#5-decision-making-framework)
6. [Learning & Adaptation](#6-learning--adaptation)
7. [Detailed Agent Specifications](#7-detailed-agent-specifications)
8. [Implementation Architecture](#8-implementation-architecture)
9. [Advanced Features](#9-advanced-features)

> **Note**: For detailed implementation tasks and roadmap, see [TASKS.md](../TASKS.md)

---

## 1. Executive Summary

### 1.1 What Are Agents in CryptoFunk?

Agents are **autonomous, intelligent entities** that:
- **Perceive** the trading environment (market data, portfolio state, news)
- **Reason** about optimal actions (buy, sell, hold)
- **Act** through the orchestrator and execution layer
- **Learn** from past experiences to improve performance
- **Collaborate** with other agents through MCP protocol

### 1.2 Core Design Philosophy

Our agent architecture combines **three paradigms**:

1. **BDI (Belief-Desire-Intention)** - for goal-oriented reasoning
2. **Reinforcement Learning** - for adaptive decision-making
3. **MCP Protocol** - for standardized communication

### 1.3 Key Innovations

✅ **Hybrid Architecture**: Combines symbolic reasoning (BDI) with learning (RL)
✅ **Multi-Layer Memory**: Short-term, episodic, semantic, and procedural memory
✅ **MCP-Native**: All agents are MCP clients with standardized interfaces
✅ **Reflective Learning**: Agents analyze past performance and adapt
✅ **Consensus Mechanism**: Multi-agent voting with confidence weighting
✅ **Risk-Aware**: Integrated risk management at every decision point

---

## 2. Agent Architecture Foundations

### 2.1 Three-Layer Architecture

```
┌─────────────────────────────────────────────────────────┐
│               COGNITIVE LAYER (BDI)                     │
│  Beliefs: Market understanding, agent knowledge         │
│  Desires: Trading goals, profit targets                 │
│  Intentions: Planned actions, commitments               │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│          LEARNING LAYER (Reinforcement Learning)        │
│  State: Market conditions, portfolio, indicators        │
│  Policy: Strategy for action selection                  │
│  Value Function: Expected returns estimation            │
│  Experience Replay: Learning from past trades           │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│         COMMUNICATION LAYER (MCP Protocol)              │
│  MCP Client: Connect to servers (tools/resources)       │
│  Message Passing: Coordination with orchestrator        │
│  Tool Invocation: Call technical indicators, data       │
└─────────────────────────────────────────────────────────┘
```

### 2.2 BDI (Belief-Desire-Intention) Model

#### Why BDI for Trading?

BDI architecture provides a **transparent, interpretable** framework for agent reasoning:

- **Beliefs** = Agent's understanding of the market
- **Desires** = Trading objectives (profit, risk minimization)
- **Intentions** = Committed actions to execute

This makes agent behavior **explainable** and **debuggable** - critical for financial applications.

#### BDI Components in CryptoFunk

```go
type BDIAgent struct {
    // Beliefs: Agent's view of the world
    beliefs *BeliefBase

    // Desires: Goals and objectives
    desires []Desire

    // Intentions: Committed action plans
    intentions []Intention

    // Reasoning engine
    reasoner *Reasoner
}

type BeliefBase struct {
    // Market beliefs
    marketTrend      string        // bullish/bearish/neutral
    volatility       float64       // current volatility assessment
    liquidity        string        // high/medium/low
    sentiment        float64       // -1 (bearish) to +1 (bullish)

    // Technical beliefs
    indicators       map[string]float64  // RSI, MACD, etc.
    support          float64       // support level
    resistance       float64       // resistance level

    // Self-beliefs
    confidence       float64       // agent's confidence in beliefs
    expertise        string        // agent's domain expertise
    pastPerformance  PerformanceMetrics

    // Social beliefs (from other agents)
    consensusView    string        // what other agents believe
    disagreementLevel float64      // level of disagreement
}

type Desire struct {
    goal        string     // "maximize_profit", "minimize_risk"
    priority    int        // importance (1-10)
    deadline    time.Time  // when to achieve
    constraints []Constraint
}

type Intention struct {
    action      string     // "place_buy_order", "analyze_trend"
    target      string     // symbol, price
    rationale   string     // why this action
    commitment  float64    // strength of commitment (0-1)
    prerequisites []string // what must be true first
}
```

#### BDI Reasoning Cycle

```
1. PERCEIVE
   ↓
   Update Beliefs (from market data, other agents)
   ↓
2. DELIBERATE
   ↓
   Generate Options (possible actions)
   ↓
   Filter by Desires (align with goals)
   ↓
   Select Intentions (commit to actions)
   ↓
3. EXECUTE
   ↓
   Perform Committed Actions
   ↓
4. REFLECT
   ↓
   Evaluate Outcomes
   ↓
   Update Beliefs
   ↓
   (Cycle repeats)
```

### 2.3 Reinforcement Learning Layer

#### MDP (Markov Decision Process) Formulation

Trading is modeled as an MDP:

**State (S)**:
```go
type TradingState struct {
    // Market state
    prices        []float64  // recent price history
    volumes       []float64  // volume history
    indicators    map[string]float64  // RSI, MACD, etc.
    orderBook     OrderBookSnapshot

    // Portfolio state
    position      Position   // current holdings
    cash          float64    // available cash
    unrealizedPnL float64

    // Temporal context
    timeOfDay     int        // hour of day
    dayOfWeek     int
    volatility    float64

    // Agent state
    recentActions []Action   // last N actions
    performance   float64    // recent win rate
}
```

**Action (A)**:
```go
type Action struct {
    actionType   string   // "BUY", "SELL", "HOLD"
    symbol       string
    quantity     float64  // or percentage
    orderType    string   // "MARKET", "LIMIT"
    limitPrice   float64  // if LIMIT order
    stopLoss     float64
    takeProfit   float64
}
```

**Reward (R)**:
```go
// Reward function combines multiple objectives
reward = α * profitReward +         // immediate profit
         β * riskAdjustedReturn +   // Sharpe ratio
         γ * drawdownPenalty +      // penalty for drawdown
         δ * actionConsistency      // penalty for erratic behavior
```

**Transition (P)**:
- Probabilistic state transitions based on market dynamics
- Considers impact of agent's actions on market (small for crypto)

#### Policy Architecture

```go
type Policy interface {
    // Select action given state
    SelectAction(state TradingState) (Action, float64) // action, confidence

    // Update policy based on experience
    Update(experience Experience)

    // Evaluate action value
    Evaluate(state TradingState, action Action) float64
}

// Deep Q-Network (DQN) Policy
type DQNPolicy struct {
    network      *NeuralNetwork
    targetNetwork *NeuralNetwork
    replayBuffer *ExperienceReplay
    optimizer    Optimizer
}

// Actor-Critic Policy (more advanced)
type ActorCriticPolicy struct {
    actor  *NeuralNetwork  // policy network
    critic *NeuralNetwork  // value network
    memory *Memory
}
```

#### Experience Replay

```go
type Experience struct {
    state      TradingState
    action     Action
    reward     float64
    nextState  TradingState
    done       bool
    metadata   map[string]interface{} // additional context
}

type ExperienceReplay struct {
    buffer     []Experience
    capacity   int
    position   int
    prioritized bool  // prioritized experience replay
}

// Sample experiences for learning
func (er *ExperienceReplay) Sample(batchSize int) []Experience {
    // Prioritize experiences with high temporal-difference error
    // Or recent important events (big wins/losses)
}
```

### 2.4 Hybrid BDI-RL Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    AGENT DECISION LOOP                  │
└─────────────────────────────────────────────────────────┘

1. PERCEPTION
   ├─ Receive market data (MCP resources)
   ├─ Update Beliefs (BDI)
   └─ Construct State (RL)

2. REASONING
   ├─ BDI: Generate goal-oriented options
   │   └─ "Should I trade now based on my goals?"
   │
   └─ RL: Evaluate options with learned policy
       └─ "What's the expected value of each option?"

3. DECISION
   ├─ Combine BDI rationale + RL value estimates
   ├─ Select Intention (BDI) + Action (RL)
   └─ Compute Confidence

4. EXECUTION
   ├─ Commit to Intention (BDI)
   └─ Execute Action (MCP tool calls)

5. REFLECTION
   ├─ Observe outcome
   ├─ Update Beliefs (BDI)
   ├─ Store Experience (RL)
   └─ Learn from result
```

---

## 3. Memory Systems

### 3.1 Memory Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                  AGENT MEMORY SYSTEM                    │
├─────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────┐  │
│  │         SHORT-TERM MEMORY (Working Memory)       │  │
│  │  - Current session context                       │  │
│  │  - Active market data                            │  │
│  │  - Immediate observations                        │  │
│  │  Capacity: ~100 items, Duration: 1 session       │  │
│  └──────────────────────────────────────────────────┘  │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐  │
│  │         EPISODIC MEMORY (Event Memory)          │  │
│  │  - Trading episodes (sequences of actions)       │  │
│  │  - Successful/failed trades                      │  │
│  │  - Market events (crashes, rallies)              │  │
│  │  Storage: PostgreSQL, Capacity: Unlimited        │  │
│  └──────────────────────────────────────────────────┘  │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐  │
│  │         SEMANTIC MEMORY (Knowledge Base)         │  │
│  │  - Market knowledge (patterns, relationships)    │  │
│  │  - Strategy knowledge                            │  │
│  │  - Indicator interpretations                     │  │
│  │  Storage: Vector DB (embeddings)                 │  │
│  └──────────────────────────────────────────────────┘  │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐  │
│  │         PROCEDURAL MEMORY (Skills)               │  │
│  │  - Learned policies (neural networks)            │  │
│  │  - Action strategies                             │  │
│  │  - Optimization techniques                       │  │
│  │  Storage: Model files (.pt, .h5)                 │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### 3.2 Short-Term Memory (Working Memory)

**Purpose**: Hold immediate context for current decision-making

```go
type ShortTermMemory struct {
    // Current session context
    sessionID       string
    startTime       time.Time

    // Recent observations
    recentPrices    *RingBuffer  // last 100 prices
    recentVolumes   *RingBuffer
    recentSignals   *RingBuffer

    // Active beliefs
    currentBeliefs  *BeliefBase

    // Attention mechanism
    focus           []string      // what agent is focusing on

    // Capacity limit
    maxSize         int
}

// Ring buffer for efficient memory management
type RingBuffer struct {
    data     []interface{}
    capacity int
    head     int
    tail     int
}
```

**Characteristics**:
- **Volatile**: Cleared at end of session
- **Fast**: In-memory (Go maps/slices)
- **Limited capacity**: ~100 items
- **Real-time**: Updated every decision cycle

### 3.3 Episodic Memory (Experience Memory)

**Purpose**: Store specific trading episodes for learning and reflection

```go
type EpisodicMemory struct {
    db *postgres.DB
}

type Episode struct {
    id          string
    agentName   string
    timestamp   time.Time

    // Episode content
    initialState TradingState
    actions      []Action
    states       []TradingState
    rewards      []float64
    finalState   TradingState

    // Episode metadata
    totalReward  float64
    duration     time.Duration
    success      bool
    category     string  // "winning_trade", "losing_trade", "market_crash"

    // Importance weighting
    importance   float64  // for prioritized replay

    // Context
    marketConditions string
    annotations      []string  // human or agent annotations
}
```

**Storage Strategy**:
```sql
-- Episodes table
CREATE TABLE episodes (
    id UUID PRIMARY KEY,
    agent_name VARCHAR(100),
    timestamp TIMESTAMPTZ,
    initial_state JSONB,
    actions JSONB,
    states JSONB,
    rewards FLOAT[],
    final_state JSONB,
    total_reward FLOAT,
    duration INTERVAL,
    success BOOLEAN,
    category VARCHAR(50),
    importance FLOAT,
    market_conditions VARCHAR(50),
    annotations TEXT[]
);

-- Index for efficient retrieval
CREATE INDEX idx_episodes_agent_time ON episodes (agent_name, timestamp DESC);
CREATE INDEX idx_episodes_success ON episodes (success, importance DESC);
CREATE INDEX idx_episodes_category ON episodes (category);
```

**Retrieval Mechanisms**:

1. **Recent Episodes**: Last N episodes
2. **Similar Situations**: Retrieve episodes with similar market conditions
3. **Successful Episodes**: High-reward episodes for imitation
4. **Failure Analysis**: Low-reward episodes to avoid mistakes

```go
func (em *EpisodicMemory) RetrieveSimilar(
    currentState TradingState,
    limit int,
) ([]Episode, error) {
    // Find episodes with similar:
    // - Market trend
    // - Volatility
    // - Indicators
    // - Time of day

    similarity := calculateSimilarity(currentState, episode.initialState)
    // Return top-k most similar
}
```

### 3.4 Semantic Memory (Knowledge Base)

**Purpose**: Store abstract knowledge about markets, patterns, strategies

```go
type SemanticMemory struct {
    vectorDB VectorDatabase  // Pinecone, Weaviate, or pgvector
}

type Knowledge struct {
    id          string
    knowledgeType string  // "pattern", "strategy", "relationship"

    // Content
    description string
    embedding   []float64  // vector representation

    // Relationships
    relatedTo   []string   // IDs of related knowledge

    // Confidence
    confidence  float64
    evidence    []string   // supporting episodes

    // Metadata
    createdAt   time.Time
    updatedAt   time.Time
    useCount    int
}
```

**Knowledge Types**:

1. **Market Patterns**
   ```go
   "When RSI > 70 and MACD crosses down, price typically drops 3-5% within 24h"
   ```

2. **Strategy Knowledge**
   ```go
   "Trend following works best in high-volatility, directional markets"
   ```

3. **Causal Relationships**
   ```go
   "News about regulation → bearish sentiment → price drop"
   ```

4. **Meta-Knowledge**
   ```go
   "My technical agent is more accurate during US trading hours"
   ```

**Retrieval**:
```go
func (sm *SemanticMemory) Query(question string, topK int) ([]Knowledge, error) {
    // 1. Embed question
    questionEmbedding := embed(question)

    // 2. Vector similarity search
    results := sm.vectorDB.Search(questionEmbedding, topK)

    // 3. Rank by relevance + confidence
    ranked := rankByRelevanceAndConfidence(results)

    return ranked, nil
}

// Example usage
relevantKnowledge := sm.Query(
    "What happens when RSI is overbought in a downtrend?",
    topK=5,
)
```

### 3.5 Procedural Memory (Learned Skills)

**Purpose**: Store learned action policies and procedures

```go
type ProceduralMemory struct {
    policies     map[string]Policy
    skills       map[string]Skill
    modelStorage *ModelStorage
}

type Skill struct {
    name        string
    description string
    policyFile  string       // path to saved model
    version     int
    performance PerformanceMetrics

    // When to use this skill
    conditions  []Condition
}

// Example: Learned entry timing skill
entryTimingSkill := Skill{
    name:        "optimal_entry_timing",
    description: "Learned policy for timing entries in trending markets",
    policyFile:  "models/entry_timing_v3.pt",
    conditions: []Condition{
        {"market_trend", "==", "bullish"},
        {"volatility", "<", 0.03},
    },
}
```

**Skill Learning**:
```go
// Train new skill
func (pm *ProceduralMemory) LearnSkill(
    skillName string,
    trainingData []Episode,
) error {
    // 1. Initialize policy network
    policy := NewDQNPolicy()

    // 2. Train on episodes
    for _, episode := range trainingData {
        policy.Update(episode)
    }

    // 3. Evaluate performance
    performance := evaluatePolicy(policy, validationSet)

    // 4. Save if better than existing
    if performance > existingPerformance {
        pm.SaveSkill(skillName, policy, performance)
    }

    return nil
}
```

### 3.6 Memory Consolidation & Intelligent Decay

**Challenge**: Agents can't remember everything forever

**Solution**: Intelligent memory management

```go
type MemoryManager struct {
    stm *ShortTermMemory
    em  *EpisodicMemory
    sm  *SemanticMemory
    pm  *ProceduralMemory
}

// Periodically consolidate memories
func (mm *MemoryManager) Consolidate() {
    // 1. Short-term → Episodic
    //    Move completed episodes from STM to EM
    episodes := mm.stm.ExtractEpisodes()
    mm.em.Store(episodes)

    // 2. Episodic → Semantic
    //    Extract patterns from episodes
    patterns := mm.em.ExtractPatterns()
    mm.sm.Store(patterns)

    // 3. Decay old memories
    mm.DecayMemories()
}

func (mm *MemoryManager) DecayMemories() {
    // Composite score for each memory
    for _, episode := range mm.em.GetAll() {
        score := calculateMemoryScore(episode)

        if score < threshold {
            // Option 1: Delete
            mm.em.Delete(episode.id)

            // Option 2: Archive (compress)
            mm.em.Archive(episode.id)
        }
    }
}

func calculateMemoryScore(episode Episode) float64 {
    // Factors:
    // - Recency: Recent memories more important
    // - Relevance: Frequently accessed memories
    // - Utility: Memories that led to good outcomes
    // - Uniqueness: Rare situations more important

    recency := 1.0 / timeSince(episode.timestamp)
    relevance := float64(episode.accessCount)
    utility := episode.totalReward
    uniqueness := 1.0 / similarEpisodeCount(episode)

    return α*recency + β*relevance + γ*utility + δ*uniqueness
}
```

---

## 4. Agent Communication & Coordination

### 4.1 Communication Patterns

#### 4.1.1 Agent-to-Server (MCP Tools & Resources)

```go
// Agent invokes MCP tools
resp, err := agent.mcpClient.CallTool(ctx, &mcp.CallToolRequest{
    Params: mcp.CallToolRequestParams{
        Name: "calculate_rsi",
        Arguments: map[string]any{
            "symbol": "BTCUSDT",
            "period": 14,
        },
    },
})
```

#### 4.1.2 Agent-to-Orchestrator (Signals & Decisions)

```go
// Agent publishes signal to orchestrator
signal := Signal{
    AgentName:  "technical-agent",
    Symbol:     "BTCUSDT",
    Signal:     "BUY",
    Confidence: 0.85,
    Rationale:  "RSI oversold + MACD bullish crossover",
    Timestamp:  time.Now(),
}

orchestrator.ReceiveSignal(signal)
```

#### 4.1.3 Agent-to-Agent (Peer Communication)

```go
// Direct agent communication via message bus
type AgentMessage struct {
    from    string
    to      string
    msgType string  // "request", "inform", "query"
    content interface{}
}

// Example: Trend agent asks technical agent for confirmation
msg := AgentMessage{
    from:    "trend-agent",
    to:      "technical-agent",
    msgType: "query",
    content: QueryMessage{
        question: "Do you see bullish confirmation on BTCUSDT?",
        context:  currentState,
    },
}

messageBus.Send(msg)
```

### 4.2 Coordination Mechanisms

#### 4.2.1 Blackboard Pattern

```
┌─────────────────────────────────────────┐
│           BLACKBOARD (Redis)            │
│                                         │
│  Current Market State                   │
│  Agent Signals                          │
│  Shared Knowledge                       │
│  Coordination State                     │
└─────────────────────────────────────────┘
         ▲  │  ▲  │  ▲  │
         │  ▼  │  ▼  │  ▼
    ┌────┴──┐┌┴────┐┌┴────┐
    │Agent 1││Agent2││Agent3│
    └───────┘└─────┘└─────┘
```

```go
type Blackboard struct {
    redis *redis.Client
}

// Write to blackboard
func (b *Blackboard) Post(key string, value interface{}) error {
    return b.redis.Set(ctx, key, value, expiration)
}

// Read from blackboard
func (b *Blackboard) Read(key string) (interface{}, error) {
    return b.redis.Get(ctx, key)
}

// Subscribe to blackboard changes
func (b *Blackboard) Subscribe(pattern string, handler func(msg)) {
    pubsub := b.redis.Subscribe(ctx, pattern)
    for msg := range pubsub.Channel() {
        handler(msg)
    }
}
```

**Usage**:
```go
// Agent posts belief
blackboard.Post("beliefs:technical-agent:BTCUSDT", beliefs)

// Other agents can read
beliefs := blackboard.Read("beliefs:technical-agent:BTCUSDT")

// Subscribe to all belief updates
blackboard.Subscribe("beliefs:*:BTCUSDT", func(msg) {
    // React to belief updates from other agents
})
```

#### 4.2.2 Contract Net Protocol

For task allocation among agents:

```go
// 1. Orchestrator announces task
task := Task{
    description: "Analyze BTCUSDT for next 1 hour",
    deadline:    time.Now().Add(5 * time.Minute),
    requirements: []string{"technical_analysis"},
}

orchestrator.AnnounceTask(task)

// 2. Agents bid
bid := Bid{
    agentName:  "technical-agent",
    confidence: 0.9,
    cost:       5.0,  // computational cost
    time:       2 * time.Minute,
}

orchestrator.SubmitBid(task.id, bid)

// 3. Orchestrator awards task
winner := orchestrator.SelectBestBid(task.id)
orchestrator.AwardTask(task.id, winner)

// 4. Winner executes and reports
result := winner.ExecuteTask(task)
orchestrator.ReceiveResult(task.id, result)
```

#### 4.2.3 Consensus Mechanisms

**Weighted Voting** (already in ARCHITECTURE.md)

**Delphi Method** (iterative consensus):
```go
// Round 1: Initial opinions
round1 := collectOpinions(agents)

// Round 2: Share aggregated results, ask for revision
aggregated := aggregate(round1)
round2 := collectRevisedOpinions(agents, aggregated)

// Round 3: Final consensus
consensus := aggregate(round2)
```

---

## 5. Decision-Making Framework

### 5.1 Decision Process Flow

```
INPUT: Market State + Portfolio State
  │
  ▼
┌─────────────────────────────────────┐
│  STAGE 1: PERCEPTION & BELIEF       │
│  - Update beliefs from observations │
│  - Assess market conditions         │
│  - Update confidence levels         │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│  STAGE 2: OPTION GENERATION         │
│  - BDI: Generate goal-aligned options│
│  - RL: Sample actions from policy   │
│  - Constraints: Filter by rules     │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│  STAGE 3: EVALUATION                │
│  - RL: Estimate Q-values            │
│  - BDI: Check intention viability   │
│  - Risk: Assess risk/reward         │
│  - Memory: Retrieve similar cases   │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│  STAGE 4: SELECTION                 │
│  - Rank options by composite score  │
│  - Apply confidence threshold       │
│  - Select best action               │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│  STAGE 5: COMMITMENT                │
│  - Form intention (BDI)             │
│  - Allocate resources               │
│  - Set monitoring triggers          │
└─────────────────┬───────────────────┘
                  │
                  ▼
OUTPUT: Decision (Action + Confidence + Rationale)
```

### 5.2 Multi-Criteria Decision Making

```go
type DecisionCriteria struct {
    expectedReturn  float64
    riskAdjusted    float64
    timeHorizon     time.Duration
    confidence      float64
    alignment       float64  // alignment with goals
    feasibility     float64
}

func (agent *Agent) EvaluateOption(option Action) float64 {
    criteria := DecisionCriteria{
        expectedReturn: agent.rl.EstimateReturn(option),
        riskAdjusted:   agent.risk.AdjustForRisk(option),
        confidence:     agent.beliefs.GetConfidence(),
        alignment:      agent.bdi.CheckAlignment(option),
        feasibility:    agent.CheckFeasibility(option),
    }

    // Weighted score
    score := w1*criteria.expectedReturn +
             w2*criteria.riskAdjusted +
             w3*criteria.confidence +
             w4*criteria.alignment +
             w5*criteria.feasibility

    return score
}
```

### 5.3 Confidence Calibration

```go
type ConfidenceEstimator struct {
    historicalAccuracy map[float64]float64  // confidence -> actual accuracy
}

// Calibrate confidence based on historical performance
func (ce *ConfidenceEstimator) Calibrate(confidence float64) float64 {
    // If agent says 80% confident, what's actual success rate?
    actualAccuracy := ce.historicalAccuracy[confidence]

    // Adjust confidence to match reality
    calibrated := (confidence + actualAccuracy) / 2

    return calibrated
}
```

---

## 6. Learning & Adaptation

### 6.1 Online Learning

```go
// After each trade execution
func (agent *Agent) Learn(experience Experience) {
    // 1. Store in episodic memory
    agent.memory.episodic.Store(experience)

    // 2. Update RL policy
    agent.rl.policy.Update(experience)

    // 3. Update beliefs
    agent.bdi.UpdateBeliefs(experience.outcome)

    // 4. Extract knowledge
    if experience.isSignificant() {
        knowledge := agent.ExtractKnowledge(experience)
        agent.memory.semantic.Store(knowledge)
    }
}
```

### 6.2 Batch Learning (Nightly)

```go
// Run overnight to improve policies
func (agent *Agent) BatchLearn() {
    // 1. Retrieve all experiences from past week
    experiences := agent.memory.episodic.GetRecent(7 * 24 * time.Hour)

    // 2. Train policy with mini-batch gradient descent
    for epoch := 0; epoch < numEpochs; epoch++ {
        for _, batch := range BatchIterator(experiences, batchSize) {
            loss := agent.rl.policy.TrainBatch(batch)
            log.Info().Float64("loss", loss).Msg("Training")
        }
    }

    // 3. Evaluate on validation set
    performance := agent.Evaluate(validationSet)

    // 4. Save if improved
    if performance > agent.bestPerformance {
        agent.rl.policy.Save(fmt.Sprintf("models/%s_v%d.pt", agent.name, version))
        agent.bestPerformance = performance
    }
}
```

### 6.3 Meta-Learning (Learning to Learn)

```go
// Agent learns which strategies work in which conditions
type MetaLearner struct {
    conditionStrategyMap map[string]string  // condition -> best strategy
}

func (ml *MetaLearner) SelectStrategy(marketConditions string) string {
    // Learn which strategy works best for current conditions
    return ml.conditionStrategyMap[marketConditions]
}

// Update after trading session
func (ml *MetaLearner) UpdateStrategyPerformance(
    conditions string,
    strategy string,
    performance float64,
) {
    // If this strategy performed well in these conditions, remember it
    if performance > threshold {
        ml.conditionStrategyMap[conditions] = strategy
    }
}
```

### 6.4 Reflection & Self-Improvement

```go
// Periodic self-evaluation
func (agent *Agent) Reflect() {
    // 1. Analyze recent performance
    recentEpisodes := agent.memory.episodic.GetRecent(24 * time.Hour)

    winRate := calculateWinRate(recentEpisodes)
    avgReturn := calculateAvgReturn(recentEpisodes)
    sharpeRatio := calculateSharpe(recentEpisodes)

    // 2. Identify weaknesses
    weaknesses := agent.IdentifyWeaknesses(recentEpisodes)
    // e.g., "Poor performance in high volatility"

    // 3. Adjust strategy
    for _, weakness := range weaknesses {
        agent.AddressWeakness(weakness)
    }

    // 4. Update self-beliefs
    agent.beliefs.UpdateSelfAssessment(winRate, avgReturn)
}

func (agent *Agent) IdentifyWeaknesses(episodes []Episode) []string {
    var weaknesses []string

    // Check performance in different conditions
    perfByCondition := agent.GroupByCondition(episodes)

    for condition, episodes := range perfByCondition {
        performance := calculatePerformance(episodes)

        if performance < threshold {
            weaknesses = append(weaknesses, condition)
        }
    }

    return weaknesses
}
```

---

## 7. Detailed Agent Specifications

### 7.1 Technical Analysis Agent

**Role**: Analyze price action and technical indicators

**BDI Configuration**:
```go
beliefs := BeliefBase{
    marketTrend:    "bullish",
    confidence:     0.85,
    indicators: map[string]float64{
        "RSI":        32.5,  // oversold
        "MACD":       positive,
        "BB_position": "lower_band",
    },
}

desires := []Desire{
    {goal: "identify_entry_signals", priority: 10},
    {goal: "detect_trend_changes", priority: 9},
    {goal: "avoid_false_signals", priority: 8},
}

intentions := []Intention{
    {
        action:     "signal_buy",
        rationale:  "RSI oversold + MACD bullish cross + price at lower BB",
        commitment: 0.85,
    },
}
```

**RL State Space**:
```go
state := TradingState{
    prices:     last100Prices,
    volumes:    last100Volumes,
    indicators: map[string]float64{
        "RSI":     32.5,
        "MACD":    0.15,
        "signal":  0.12,
        "BB_upper": 45000,
        "BB_lower": 42000,
        "EMA_20":   43500,
        "EMA_50":   43000,
    },
}
```

**Action Space**:
```go
actions := []string{
    "signal_strong_buy",
    "signal_buy",
    "signal_neutral",
    "signal_sell",
    "signal_strong_sell",
}
```

**Reward Function**:
```go
// Reward based on signal accuracy
reward = 1.0 if signal_correct else -0.5
// + bonus for early detection
// + penalty for whipsaw (frequent reversals)
```

**Memory Usage**:
- **STM**: Current candlesticks, recent indicator values
- **Episodic**: Past signal episodes (correct/incorrect)
- **Semantic**: Pattern knowledge ("double bottom → reversal")
- **Procedural**: Indicator interpretation policies

**MCP Integration**:
```go
// Connects to:
// - Market Data Server (candlesticks)
// - Technical Indicators Server (RSI, MACD, BB)

// Tools used:
tools := []string{
    "get_candlesticks",
    "calculate_rsi",
    "calculate_macd",
    "calculate_bollinger_bands",
    "detect_patterns",
}
```

**Implementation**:
```go
type TechnicalAnalysisAgent struct {
    *BaseAgent

    // Technical expertise
    indicators    *TechnicalIndicators
    patternDetector *PatternDetector

    // Specialization
    expertise     []string  // ["RSI", "MACD", "patterns"]
    timeframe     string    // "1h", "4h", "1d"
}

func (ta *TechnicalAnalysisAgent) Analyze(symbol string) Signal {
    // 1. Gather data
    candles := ta.mcpClient.GetCandlesticks(symbol, ta.timeframe, 100)

    // 2. Calculate indicators
    rsi := ta.indicators.RSI(candles, 14)
    macd, signal, hist := ta.indicators.MACD(candles)
    bb := ta.indicators.BollingerBands(candles, 20, 2)

    // 3. Update beliefs
    ta.beliefs.indicators["RSI"] = rsi
    ta.beliefs.indicators["MACD"] = macd
    ta.beliefs.UpdateTrend(candles)

    // 4. Detect patterns
    patterns := ta.patternDetector.Detect(candles)

    // 5. BDI reasoning
    options := ta.GenerateOptions(ta.beliefs)
    bestOption := ta.SelectBestOption(options)

    // 6. RL policy evaluation
    state := ta.ConstructState(candles, ta.beliefs)
    action, confidence := ta.rl.policy.SelectAction(state)

    // 7. Combine BDI + RL
    signal := ta.CombineDecisions(bestOption, action, confidence)

    return signal
}
```

### 7.2 Sentiment Analysis Agent

**Role**: Analyze news, social media, and market sentiment

**BDI Configuration**:
```go
beliefs := BeliefBase{
    sentiment:       0.65,  // bullish
    newsImpact:      "positive",
    socialMood:      "optimistic",
    fearGreedIndex:  75,    // greed
    confidence:      0.70,
}

desires := []Desire{
    {goal: "identify_sentiment_shifts", priority: 10},
    {goal: "predict_sentiment_impact", priority: 9},
    {goal: "detect_manipulation", priority: 7},
}
```

**Data Sources**:
- News APIs (CryptoPanic, NewsAPI)
- Twitter/X sentiment
- Reddit sentiment (r/cryptocurrency)
- Fear & Greed Index
- On-chain sentiment (Santiment)

**NLP Pipeline**:
```go
type SentimentPipeline struct {
    tokenizer    *Tokenizer
    sentimentModel *SentimentModel  // BERT-based
    entityExtractor *EntityExtractor
    impactPredictor *ImpactPredictor
}

func (sp *SentimentPipeline) AnalyzeNews(articles []Article) SentimentScore {
    var scores []float64

    for _, article := range articles {
        // 1. Extract entities
        entities := sp.entityExtractor.Extract(article.text)
        // e.g., "Bitcoin", "SEC", "regulation"

        // 2. Sentiment analysis
        sentiment := sp.sentimentModel.Predict(article.text)
        // -1 (very negative) to +1 (very positive)

        // 3. Weight by source credibility
        weighted := sentiment * article.sourceCredibility

        // 4. Weight by recency
        timeFactor := 1.0 / (1.0 + hoursSince(article.publishedAt))
        weighted *= timeFactor

        scores = append(scores, weighted)
    }

    aggregated := average(scores)
    return SentimentScore{value: aggregated, confidence: calculateConfidence(scores)}
}
```

**Signal Generation**:
```go
func (sa *SentimentAnalysisAgent) GenerateSignal(symbol string) Signal {
    // 1. Fetch recent news
    news := sa.fetchNews(symbol, 24*time.Hour)

    // 2. Analyze sentiment
    newsSentiment := sa.pipeline.AnalyzeNews(news)

    // 3. Social media sentiment
    tweets := sa.fetchTweets(symbol, 24*time.Hour)
    socialSentiment := sa.pipeline.AnalyzeSocial(tweets)

    // 4. Fear & Greed index
    fgIndex := sa.fetchFearGreedIndex()

    // 5. Aggregate sentiments
    overall := sa.AggregateSentiments(newsSentiment, socialSentiment, fgIndex)

    // 6. Convert to signal
    signal := sa.SentimentToSignal(overall)

    return signal
}
```

### 7.3 Order Book Analysis Agent

**Role**: Analyze order book depth, liquidity, and imbalances

**BDI Configuration**:
```go
beliefs := BeliefBase{
    liquidity:       "high",
    bidAskImbalance: 1.5,   // more buyers
    depthSkew:       "bid",
    largOrders:      []Order{...},  // whale watching
}

desires := []Desire{
    {goal: "predict_short_term_moves", priority: 10},
    {goal: "identify_support_resistance", priority: 9},
    {goal: "detect_manipulation", priority: 8},
}
```

**Analysis Techniques**:

1. **Bid-Ask Imbalance**:
```go
func (oba *OrderBookAgent) CalculateImbalance(orderbook OrderBook) float64 {
    bidVolume := sum(orderbook.bids[:10])  // top 10 levels
    askVolume := sum(orderbook.asks[:10])

    imbalance := bidVolume / askVolume
    // > 1: buying pressure
    // < 1: selling pressure

    return imbalance
}
```

2. **Depth Analysis**:
```go
func (oba *OrderBookAgent) AnalyzeDepth(orderbook OrderBook) DepthAnalysis {
    // Calculate cumulative volume at different price levels
    bidDepth := cumulativeVolume(orderbook.bids, 0.01)  // 1% below
    askDepth := cumulativeVolume(orderbook.asks, 0.01)  // 1% above

    return DepthAnalysis{
        support:    bidDepth > askDepth,
        depthRatio: bidDepth / askDepth,
    }
}
```

3. **Large Order Detection**:
```go
func (oba *OrderBookAgent) DetectLargeOrders(orderbook OrderBook) []LargeOrder {
    var largeOrders []LargeOrder

    avgSize := calculateAverageOrderSize(orderbook)

    for _, order := range orderbook.AllOrders() {
        if order.size > 10*avgSize {  // 10x average
            largeOrders = append(largeOrders, LargeOrder{
                side:  order.side,
                price: order.price,
                size:  order.size,
                impact: estimateImpact(order),
            })
        }
    }

    return largeOrders
}
```

4. **Spoofing Detection**:
```go
func (oba *OrderBookAgent) DetectSpoofing(history []OrderBookSnapshot) bool {
    // Look for large orders that disappear quickly
    for i := 1; i < len(history); i++ {
        prev := history[i-1]
        curr := history[i]

        // Find orders that vanished
        vanished := prev.findMissingOrders(curr)

        for _, order := range vanished {
            if order.size > threshold && order.duration < 5*time.Second {
                // Likely spoofing
                return true
            }
        }
    }

    return false
}
```

### 7.4 Trend Following Agent

**Role**: Identify and trade with established trends

**BDI Configuration**:
```go
beliefs := BeliefBase{
    marketTrend:    "uptrend",
    trendStrength:  0.85,
    trendDuration:  48 * time.Hour,
    confidence:     0.90,
}

desires := []Desire{
    {goal: "ride_trend", priority: 10},
    {goal: "enter_early", priority: 8},
    {goal: "exit_before_reversal", priority: 9},
}
```

**Strategy**:
```go
type TrendFollowingStrategy struct {
    // EMA crossover parameters
    fastEMA    int  // 20
    slowEMA    int  // 50

    // Trend confirmation
    adx        *ADXIndicator
    adxThreshold float64  // 25

    // Exit strategy
    trailingStop float64  // 2%
}

func (tfs *TrendFollowingStrategy) GenerateSignal(candles []Candlestick) Signal {
    // 1. Calculate EMAs
    fastEMA := calculateEMA(candles, tfs.fastEMA)
    slowEMA := calculateEMA(candles, tfs.slowEMA)

    // 2. Check for crossover
    crossover := detectCrossover(fastEMA, slowEMA)

    // 3. Confirm trend strength with ADX
    adx := tfs.adx.Calculate(candles)
    trendStrong := adx > tfs.adxThreshold

    // 4. Generate signal
    if crossover == "bullish" && trendStrong {
        return Signal{
            action:     "BUY",
            confidence: adx / 100,  // normalize ADX
            stopLoss:   currentPrice * (1 - tfs.trailingStop),
        }
    }

    if crossover == "bearish" && trendStrong {
        return Signal{
            action:     "SELL",
            confidence: adx / 100,
        }
    }

    return Signal{action: "HOLD"}
}
```

**Learning Enhancement**:
```go
// Learn optimal EMA periods for different markets
func (tfa *TrendFollowingAgent) OptimizeParameters() {
    episodes := tfa.memory.episodic.GetAll()

    // Group by market conditions
    byCondition := groupBy(episodes, "market_volatility")

    for condition, eps := range byCondition {
        // Find best EMA periods for this condition
        bestParams := tfa.GridSearch(eps, parameterSpace)

        // Store in semantic memory
        tfa.memory.semantic.Store(Knowledge{
            description: fmt.Sprintf("Best EMA params for %s: %v", condition, bestParams),
            confidence:  calculateConfidence(eps),
        })
    }
}
```

### 7.5 Mean Reversion Agent

**Role**: Trade oversold/overbought conditions

**BDI Configuration**:
```go
beliefs := BeliefBase{
    priceDeviation: -2.5,  // -2.5 std devs from mean
    rsi:            28,    // oversold
    marketRegime:   "ranging",
    confidence:     0.80,
}

desires := []Desire{
    {goal: "profit_from_reversions", priority: 10},
    {goal: "quick_exits", priority: 9},
    {goal: "avoid_trending_markets", priority: 10},
}
```

**Strategy**:
```go
type MeanReversionStrategy struct {
    lookback      int      // 20 periods
    stdDevs       float64  // 2.0
    rsiPeriod     int      // 14
    rsiOversold   float64  // 30
    rsiOverbought float64  // 70

    // Quick exit
    profitTarget  float64  // 1.5%
    stopLoss      float64  // 1.0%
}

func (mrs *MeanReversionStrategy) GenerateSignal(candles []Candlestick) Signal {
    // 1. Calculate mean and std dev
    prices := extractPrices(candles)
    mean := average(prices)
    stdDev := standardDeviation(prices)

    currentPrice := prices[len(prices)-1]

    // 2. Calculate z-score
    zScore := (currentPrice - mean) / stdDev

    // 3. Calculate RSI
    rsi := calculateRSI(prices, mrs.rsiPeriod)

    // 4. Check market regime (avoid trends)
    isRanging := mrs.detectRangingMarket(candles)

    if !isRanging {
        return Signal{action: "HOLD", rationale: "trending market"}
    }

    // 5. Mean reversion signals
    if zScore < -mrs.stdDevs && rsi < mrs.rsiOversold {
        return Signal{
            action:       "BUY",
            confidence:   0.8,
            takeProfit:   currentPrice * (1 + mrs.profitTarget),
            stopLoss:     currentPrice * (1 - mrs.stopLoss),
            rationale:    "oversold mean reversion",
        }
    }

    if zScore > mrs.stdDevs && rsi > mrs.rsiOverbought {
        return Signal{
            action:       "SELL",
            confidence:   0.8,
            takeProfit:   currentPrice * (1 - mrs.profitTarget),
            stopLoss:     currentPrice * (1 + mrs.stopLoss),
            rationale:    "overbought mean reversion",
        }
    }

    return Signal{action: "HOLD"}
}

func (mrs *MeanReversionStrategy) detectRangingMarket(candles []Candlestick) bool {
    // Use ADX: low ADX = ranging, high ADX = trending
    adx := calculateADX(candles, 14)
    return adx < 20  // weak trend = ranging
}
```

### 7.6 Risk Management Agent

**Role**: Final approval, position sizing, risk controls

**BDI Configuration**:
```go
beliefs := BeliefBase{
    portfolioRisk:     0.15,  // 15% of portfolio at risk
    currentDrawdown:   0.08,  // 8% drawdown
    winRate:           0.58,  // recent win rate
    volatility:        "high",
    confidence:        0.95,
}

desires := []Desire{
    {goal: "protect_capital", priority: 10},
    {goal: "optimize_risk_reward", priority: 9},
    {goal: "prevent_ruin", priority: 10},
}
```

**Risk Assessment**:
```go
type RiskAssessment struct {
    // Portfolio risk
    totalExposure    float64
    concentrationRisk float64
    correlationRisk  float64

    // Trade risk
    positionSize     float64
    stopLossDistance float64
    riskRewardRatio  float64

    // Market risk
    volatility       float64
    liquidityRisk    float64

    // Overall
    approved         bool
    rejectionReason  string
}

func (rma *RiskManagementAgent) AssessTrade(decision Decision) RiskAssessment {
    assessment := RiskAssessment{}

    // 1. Check portfolio limits
    if rma.checkPortfolioLimits() == false {
        assessment.approved = false
        assessment.rejectionReason = "portfolio limit exceeded"
        return assessment
    }

    // 2. Calculate position size (Kelly Criterion)
    positionSize := rma.CalculatePositionSize(decision)
    assessment.positionSize = positionSize

    // 3. Check risk/reward ratio
    rr := (decision.takeProfit - decision.entryPrice) /
          (decision.entryPrice - decision.stopLoss)
    assessment.riskRewardRatio = rr

    if rr < 2.0 {  // require at least 2:1
        assessment.approved = false
        assessment.rejectionReason = "insufficient risk/reward ratio"
        return assessment
    }

    // 4. Check circuit breakers
    if rma.circuitBreakersTripped() {
        assessment.approved = false
        assessment.rejectionReason = "circuit breaker active"
        return assessment
    }

    // 5. All checks passed
    assessment.approved = true
    return assessment
}
```

**Position Sizing (Kelly Criterion)**:
```go
func (rma *RiskManagementAgent) CalculatePositionSize(decision Decision) float64 {
    // Kelly Criterion: f* = (bp - q) / b
    // f* = fraction of capital
    // b = odds received (win/loss ratio)
    // p = probability of winning
    // q = probability of losing (1 - p)

    // Estimate from historical performance
    winRate := rma.GetRecentWinRate()
    avgWin := rma.GetAvgWin()
    avgLoss := rma.GetAvgLoss()

    b := avgWin / avgLoss
    p := winRate
    q := 1 - p

    kelly := (b*p - q) / b

    // Apply fractional Kelly for safety
    fractionalKelly := kelly * 0.5  // half-Kelly

    // Cap at max position size
    maxSize := 0.10  // 10% of portfolio
    positionSize := math.Min(fractionalKelly, maxSize)

    // Ensure positive
    if positionSize < 0 {
        positionSize = 0
    }

    return positionSize
}
```

**Circuit Breakers**:
```go
type CircuitBreakers struct {
    maxDailyLoss     float64  // -5%
    maxDrawdown      float64  // -15%
    maxOrdersPerMin  int      // 10
    volatilityLimit  float64  // 5%
}

func (cb *CircuitBreakers) Check(state PortfolioState) (bool, string) {
    // Daily loss check
    if state.dailyPnL < cb.maxDailyLoss {
        return true, "daily loss limit"
    }

    // Drawdown check
    if state.currentDrawdown > cb.maxDrawdown {
        return true, "max drawdown exceeded"
    }

    // Order rate check
    if state.ordersLastMinute > cb.maxOrdersPerMin {
        return true, "order rate limit"
    }

    // Volatility check
    if state.currentVolatility > cb.volatilityLimit {
        return true, "excessive volatility"
    }

    return false, ""
}
```

---

## 8. Implementation Architecture

### 8.1 Base Agent Class

```go
// BaseAgent provides common functionality
type BaseAgent struct {
    // Identity
    name        string
    agentType   string
    version     string

    // BDI components
    beliefs     *BeliefBase
    desires     []Desire
    intentions  []Intention
    reasoner    *BDIReasoner

    // RL components
    rl          *RLComponent
    policy      Policy

    // Memory
    memory      *MemorySystem

    // Communication
    mcpClient   *mcp.Client
    messageBus  *MessageBus
    blackboard  *Blackboard

    // Configuration
    config      *AgentConfig

    // Lifecycle
    state       AgentState
    lastUpdate  time.Time

    // Monitoring
    metrics     *AgentMetrics
    logger      *zerolog.Logger
}

type AgentConfig struct {
    updateInterval  time.Duration
    mcpServers      []string
    learningRate    float64
    memorySize      int
    confidence      float64
}

type AgentState string

const (
    StateInitializing AgentState = "initializing"
    StateActive       AgentState = "active"
    StateIdle         AgentState = "idle"
    StatePaused       AgentState = "paused"
    StateError        AgentState = "error"
)
```

### 8.2 Agent Lifecycle

```go
func (agent *BaseAgent) Initialize() error {
    // 1. Connect to MCP servers
    if err := agent.ConnectMCPServers(); err != nil {
        return err
    }

    // 2. Load memory
    if err := agent.memory.Load(); err != nil {
        return err
    }

    // 3. Load learned policies
    if err := agent.rl.LoadPolicy(); err != nil {
        return err
    }

    // 4. Initialize beliefs
    agent.beliefs.Initialize()

    // 5. Subscribe to events
    agent.SubscribeToEvents()

    agent.state = StateActive
    return nil
}

func (agent *BaseAgent) Run(ctx context.Context) error {
    ticker := time.NewTicker(agent.config.updateInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Main agent loop
            if err := agent.Step(ctx); err != nil {
                agent.logger.Error().Err(err).Msg("Agent step failed")
            }

        case <-ctx.Done():
            agent.Shutdown()
            return nil
        }
    }
}

func (agent *BaseAgent) Step(ctx context.Context) error {
    // 1. Perceive
    observations := agent.Perceive(ctx)

    // 2. Update beliefs
    agent.UpdateBeliefs(observations)

    // 3. Deliberate
    options := agent.Deliberate()

    // 4. Decide
    decision := agent.Decide(options)

    // 5. Act
    if decision.ShouldAct() {
        result := agent.Act(ctx, decision)

        // 6. Learn
        agent.Learn(result)
    }

    // 7. Update metrics
    agent.UpdateMetrics()

    return nil
}

func (agent *BaseAgent) Shutdown() error {
    // 1. Save state
    agent.memory.Save()

    // 2. Save learned policies
    agent.rl.SavePolicy()

    // 3. Close connections
    agent.mcpClient.Close()

    agent.state = StateIdle
    return nil
}
```

---

## 9. Advanced Features

### 9.1 Agent Hot-Swapping

```go
// Replace agent without stopping system
func (orchestrator *Orchestrator) SwapAgent(
    oldAgentName string,
    newAgent Agent,
) error {
    // 1. Transfer ongoing tasks
    tasks := orchestrator.GetAgentTasks(oldAgentName)

    // 2. Pause old agent
    orchestrator.PauseAgent(oldAgentName)

    // 3. Initialize new agent
    if err := newAgent.Initialize(); err != nil {
        return err
    }

    // 4. Transfer memory/state
    newAgent.ImportMemory(oldAgent.memory)

    // 5. Activate new agent
    orchestrator.RegisterAgent(newAgent)

    // 6. Reassign tasks
    orchestrator.AssignTasks(newAgent, tasks)

    // 7. Remove old agent
    orchestrator.RemoveAgent(oldAgentName)

    return nil
}
```

### 9.2 Agent Cloning & A/B Testing

```go
// Create agent variant for testing
func (agent *BaseAgent) Clone(variant string) *BaseAgent {
    clone := &BaseAgent{
        name:       agent.name + "_" + variant,
        agentType:  agent.agentType,
        beliefs:    agent.beliefs.Copy(),
        memory:     agent.memory.Copy(),
        rl:         agent.rl.Copy(),
        config:     agent.config.Copy(),
    }

    return clone
}

// A/B test: Original vs. New strategy
func (system *TradingSystem) ABTest() {
    controlAgent := system.GetAgent("trend-agent")
    variantAgent := controlAgent.Clone("variant_aggressive")

    // Modify variant
    variantAgent.config.riskTolerance = 1.5  // more aggressive

    // Run both in parallel (paper trading)
    results := system.RunParallel(
        controlAgent,
        variantAgent,
        duration=30*24*time.Hour,  // 30 days
    )

    // Compare performance
    if results.variant.SharpeRatio > results.control.SharpeRatio {
        system.SwapAgent("trend-agent", variantAgent)
    }
}
```

### 9.3 Hierarchical Multi-Agent System

```go
// Meta-agent that coordinates other agents
type MetaAgent struct {
    *BaseAgent
    subAgents []Agent
}

func (ma *MetaAgent) Coordinate() {
    // 1. Assess current situation
    situation := ma.AssessSituation()

    // 2. Select appropriate sub-agents
    selectedAgents := ma.SelectAgents(situation)
    // e.g., "high volatility" → activate scalping agents

    // 3. Allocate resources
    ma.AllocateResources(selectedAgents)

    // 4. Aggregate results
    results := ma.CollectResults(selectedAgents)

    // 5. Meta-decision
    finalDecision := ma.MetaDecide(results)

    return finalDecision
}
```

---

## Appendix A: Research References

1. **BDI Architecture**
   - Rao & Georgeff (1995): "BDI Agents: From Theory to Practice"
   - TwinMarket (2025): "Behavioral and Social Simulation for Financial Markets"

2. **Reinforcement Learning**
   - Multi-agent RL for trading (2024): Various ScienceDirect papers
   - HAT: HyperEdge AI Trader framework (2025)

3. **Memory Systems**
   - LLM Agent Memory (2025): Short-term, episodic, semantic
   - Experience replay challenges and solutions

4. **Agentic Design Patterns**
   - Reflection, Tool Use, Planning patterns (2025)
   - Microsoft & Google agent architecture guides

---

**End of Agent Architecture Document**

**Status**: Ready for review
**Next**: Review and approve before implementation
