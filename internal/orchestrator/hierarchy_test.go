package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHierarchyManager_CreateMetaAgent(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	config := map[string]interface{}{
		"delegation_policy":  "best_fit",
		"aggregation_policy": "weighted",
	}

	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", config)
	require.NoError(t, err)
	assert.NotNil(t, metaAgent)
	assert.Equal(t, "meta-trader", metaAgent.Name)
	assert.Equal(t, "meta", metaAgent.Type)
	assert.Equal(t, AgentStatusActive, metaAgent.Status)
	assert.NotNil(t, metaAgent.Performance)
	assert.NotNil(t, metaAgent.ResourceLimits)

	// Verify it was registered with hot-swap coordinator
	registered, err := hotSwap.GetAgent("meta-trader")
	require.NoError(t, err)
	assert.Equal(t, "meta-trader", registered.Name)

	// Verify hierarchy node created
	hierarchy := hm.GetHierarchy()
	node, exists := hierarchy["meta-trader"]
	assert.True(t, exists)
	assert.Equal(t, 1, node.Level)
	assert.True(t, node.IsMetaAgent)
}

func TestHierarchyManager_CreateMetaAgent_Duplicate(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, _, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	config := map[string]interface{}{}

	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", config)
	require.NoError(t, err)

	// Try to create duplicate
	_, err = hm.CreateMetaAgent(ctx, "meta-trader", "meta", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestHierarchyManager_AddSubAgent(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent
	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	// Register a sub-agent first
	subAgent := &AgentRegistration{
		Name:         "technical-agent",
		Type:         "technical",
		Version:      "1.0.0",
		Status:       AgentStatusActive,
		Capabilities: []string{"rsi", "macd"},
		State: &AgentState{
			Memory:             make(map[string]interface{}),
			Configuration:      make(map[string]interface{}),
			PerformanceMetrics: &PerformanceMetrics{},
			LastUpdated:        time.Now(),
		},
		Metadata:      make(map[string]interface{}),
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}
	err = hotSwap.RegisterAgent(ctx, subAgent)
	require.NoError(t, err)

	// Add sub-agent to meta-agent
	conditions := []ActivationCondition{
		{Field: "volatility", Operator: ">", Value: 0.02},
	}
	err = hm.AddSubAgent(ctx, "meta-trader", "technical-agent", 0.8, conditions)
	require.NoError(t, err)

	// Verify sub-agent was added
	metaAgent, err := hm.GetMetaAgent("meta-trader")
	require.NoError(t, err)
	assert.Len(t, metaAgent.SubAgents, 1)
	assert.Equal(t, "technical-agent", metaAgent.SubAgents[0].AgentName)
	assert.Equal(t, 0.8, metaAgent.SubAgents[0].Weight)
	assert.Len(t, metaAgent.SubAgents[0].Conditions, 1)

	// Verify hierarchy updated
	hierarchy := hm.GetHierarchy()
	metaNode, exists := hierarchy["meta-trader"]
	assert.True(t, exists)
	assert.Contains(t, metaNode.Children, "technical-agent")

	subNode, exists := hierarchy["technical-agent"]
	assert.True(t, exists)
	assert.Equal(t, "meta-trader", subNode.ParentName)
	assert.Equal(t, 2, subNode.Level)
}

func TestHierarchyManager_AddSubAgent_Duplicate(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent
	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	// Register sub-agent
	subAgent := createTestSubAgent("technical-agent", "technical")
	err = hotSwap.RegisterAgent(ctx, subAgent)
	require.NoError(t, err)

	// Add sub-agent
	err = hm.AddSubAgent(ctx, "meta-trader", "technical-agent", 0.8, nil)
	require.NoError(t, err)

	// Try to add same sub-agent again
	err = hm.AddSubAgent(ctx, "meta-trader", "technical-agent", 0.9, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already added")
}

func TestHierarchyManager_RemoveSubAgent(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Setup meta-agent with sub-agent
	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	subAgent := createTestSubAgent("technical-agent", "technical")
	err = hotSwap.RegisterAgent(ctx, subAgent)
	require.NoError(t, err)

	err = hm.AddSubAgent(ctx, "meta-trader", "technical-agent", 0.8, nil)
	require.NoError(t, err)

	// Remove sub-agent
	err = hm.RemoveSubAgent(ctx, "meta-trader", "technical-agent")
	require.NoError(t, err)

	// Verify removal
	metaAgent, err := hm.GetMetaAgent("meta-trader")
	require.NoError(t, err)
	assert.Len(t, metaAgent.SubAgents, 0)

	// Verify hierarchy updated
	hierarchy := hm.GetHierarchy()
	metaNode, exists := hierarchy["meta-trader"]
	assert.True(t, exists)
	assert.NotContains(t, metaNode.Children, "technical-agent")

	_, exists = hierarchy["technical-agent"]
	assert.False(t, exists)
}

func TestHierarchyManager_AssessSituation(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, _, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent
	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	// Post market data to blackboard
	marketData := map[string]interface{}{
		"volatility":     0.05,
		"trend_strength": 0.8,
		"liquidity":      "high",
		"sentiment":      0.7,
	}
	err = postDataToBlackboard(ctx, blackboard, "market_data", marketData)
	require.NoError(t, err)

	// Assess situation
	situation, err := hm.AssessSituation(ctx, "meta-trader")
	require.NoError(t, err)
	assert.NotNil(t, situation)
	assert.Equal(t, 0.05, situation.Volatility)
	assert.Equal(t, 0.8, situation.TrendStrength)
	assert.Equal(t, "high", situation.LiquidityLevel)
	assert.Equal(t, 0.7, situation.SentimentScore)

	// Verify situation was stored in meta-agent
	metaAgent, err := hm.GetMetaAgent("meta-trader")
	require.NoError(t, err)
	assert.NotNil(t, metaAgent.CurrentSituation)
	assert.Equal(t, 0.05, metaAgent.CurrentSituation.Volatility)
}

func TestHierarchyManager_SelectSubAgents(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent
	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	// Add sub-agents with different conditions
	agents := []struct {
		name       string
		weight     float64
		conditions []ActivationCondition
	}{
		{
			name:   "high-vol-agent",
			weight: 0.9,
			conditions: []ActivationCondition{
				{Field: "volatility", Operator: ">", Value: 0.03},
			},
		},
		{
			name:   "low-vol-agent",
			weight: 0.7,
			conditions: []ActivationCondition{
				{Field: "volatility", Operator: "<=", Value: 0.03},
			},
		},
		{
			name:   "trend-agent",
			weight: 0.8,
			conditions: []ActivationCondition{
				{Field: "trend_strength", Operator: ">", Value: 0.6},
			},
		},
	}

	for _, a := range agents {
		subAgent := createTestSubAgent(a.name, "strategy")
		err = hotSwap.RegisterAgent(ctx, subAgent)
		require.NoError(t, err)

		err = hm.AddSubAgent(ctx, "meta-trader", a.name, a.weight, a.conditions)
		require.NoError(t, err)
	}

	// Create situation with high volatility and strong trend
	situation := &Situation{
		Volatility:       0.05,
		TrendStrength:    0.8,
		MarketConditions: make(map[string]interface{}),
		PortfolioState:   make(map[string]interface{}),
		AssessedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	// Select sub-agents
	selected, err := hm.SelectSubAgents(ctx, "meta-trader", situation)
	require.NoError(t, err)

	// Should select high-vol-agent and trend-agent (not low-vol-agent)
	assert.Len(t, selected, 2)

	selectedNames := make(map[string]bool)
	for _, s := range selected {
		selectedNames[s.AgentName] = true
	}
	assert.True(t, selectedNames["high-vol-agent"])
	assert.True(t, selectedNames["trend-agent"])
	assert.False(t, selectedNames["low-vol-agent"])
}

func TestHierarchyManager_SelectSubAgents_MaxLimit(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with low max active limit
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	metaAgent.ResourceLimits.MaxActiveSubAgents = 2

	// Add 4 sub-agents (all without conditions, so all would be selected)
	for i := 1; i <= 4; i++ {
		name := fmt.Sprintf("agent-%d", i)
		weight := float64(i) * 0.2 // Different weights

		subAgent := createTestSubAgent(name, "strategy")
		err = hotSwap.RegisterAgent(ctx, subAgent)
		require.NoError(t, err)

		err = hm.AddSubAgent(ctx, "meta-trader", name, weight, nil)
		require.NoError(t, err)
	}

	situation := &Situation{
		MarketConditions: make(map[string]interface{}),
		PortfolioState:   make(map[string]interface{}),
		AssessedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	selected, err := hm.SelectSubAgents(ctx, "meta-trader", situation)
	require.NoError(t, err)

	// Should only select top 2 by weight (agent-4: 0.8, agent-3: 0.6)
	assert.Len(t, selected, 2)
	assert.Equal(t, "agent-4", selected[0].AgentName)
	assert.Equal(t, "agent-3", selected[1].AgentName)
}

func TestHierarchyManager_DelegateTask_BestFit(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with BestFit policy
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)
	metaAgent.DelegationPolicy = DelegationBestFit

	// Add sub-agents with different performance
	agents := []struct {
		name     string
		weight   float64
		accuracy float64
		latency  time.Duration
	}{
		{name: "fast-agent", weight: 0.7, accuracy: 0.8, latency: 100 * time.Millisecond},
		{name: "accurate-agent", weight: 0.8, accuracy: 0.95, latency: 500 * time.Millisecond},
		{name: "balanced-agent", weight: 0.9, accuracy: 0.9, latency: 200 * time.Millisecond},
	}

	selectedAgents := []*SubAgentInfo{}
	for _, a := range agents {
		subAgent := createTestSubAgent(a.name, "strategy")
		err = hotSwap.RegisterAgent(ctx, subAgent)
		require.NoError(t, err)

		err = hm.AddSubAgent(ctx, "meta-trader", a.name, a.weight, nil)
		require.NoError(t, err)

		// Get the SubAgentInfo and set performance
		subs, _ := hm.GetSubAgents("meta-trader")
		for _, s := range subs {
			if s.AgentName == a.name {
				s.Performance.AverageAccuracy = a.accuracy
				s.Performance.AverageLatency = a.latency
				s.Performance.TasksAssigned = 10
				s.Performance.TasksCompleted = 9
				selectedAgents = append(selectedAgents, s)
			}
		}
	}

	// Create task
	task := &AgentTask{
		ID:       uuid.New(),
		Type:     "analysis",
		Status:   TaskStatusPending,
		Priority: 5,
		Metadata: make(map[string]interface{}),
	}

	// Delegate task
	allocations, err := hm.DelegateTask(ctx, "meta-trader", task, selectedAgents)
	require.NoError(t, err)

	// Should select balanced-agent (highest weight + good accuracy + acceptable latency)
	require.Len(t, allocations, 1)
	assert.Equal(t, "balanced-agent", allocations[0].SubAgent)
}

func TestHierarchyManager_DelegateTask_All(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with All policy
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)
	metaAgent.DelegationPolicy = DelegationAll

	// Add 3 sub-agents
	selectedAgents := []*SubAgentInfo{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("agent-%d", i)
		subAgent := createTestSubAgent(name, "strategy")
		err = hotSwap.RegisterAgent(ctx, subAgent)
		require.NoError(t, err)

		err = hm.AddSubAgent(ctx, "meta-trader", name, 0.8, nil)
		require.NoError(t, err)

		subs, _ := hm.GetSubAgents("meta-trader")
		for _, s := range subs {
			if s.AgentName == name {
				selectedAgents = append(selectedAgents, s)
			}
		}
	}

	task := &AgentTask{
		ID:       uuid.New(),
		Type:     "analysis",
		Status:   TaskStatusPending,
		Metadata: make(map[string]interface{}),
	}

	// Delegate task
	allocations, err := hm.DelegateTask(ctx, "meta-trader", task, selectedAgents)
	require.NoError(t, err)

	// All 3 agents should receive the task
	assert.Len(t, allocations, 3)
}

func TestHierarchyManager_AggregateResults_Weighted(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with Weighted aggregation
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)
	metaAgent.AggregationPolicy = AggregationWeighted

	// Add sub-agents
	agents := []struct {
		name   string
		weight float64
	}{
		{name: "agent-1", weight: 0.6},
		{name: "agent-2", weight: 0.3},
		{name: "agent-3", weight: 0.1},
	}

	for _, a := range agents {
		subAgent := createTestSubAgent(a.name, "strategy")
		err = hotSwap.RegisterAgent(ctx, subAgent)
		require.NoError(t, err)

		err = hm.AddSubAgent(ctx, "meta-trader", a.name, a.weight, nil)
		require.NoError(t, err)
	}

	// Create results with different confidences
	results := []*SubAgentResult{
		{AgentName: "agent-1", Result: "BUY", Confidence: 0.9, Latency: 100 * time.Millisecond},
		{AgentName: "agent-2", Result: "BUY", Confidence: 0.7, Latency: 150 * time.Millisecond},
		{AgentName: "agent-3", Result: "HOLD", Confidence: 0.5, Latency: 80 * time.Millisecond},
	}

	// Aggregate results
	decision, err := hm.AggregateResults(ctx, "meta-trader", results)
	require.NoError(t, err)
	assert.NotNil(t, decision)

	// Weighted confidence: (0.9*0.6 + 0.7*0.3 + 0.5*0.1) / (0.6+0.3+0.1) = 0.8
	assert.InDelta(t, 0.8, decision.Confidence, 0.01)
	assert.Contains(t, decision.Rationale, "Weighted")

	// Verify decision was stored
	metaAgent, err = hm.GetMetaAgent("meta-trader")
	require.NoError(t, err)
	assert.Len(t, metaAgent.Decisions, 1)

	// Verify performance updated
	assert.Equal(t, int64(1), metaAgent.Performance.TotalDecisions)
	assert.Equal(t, int64(1), metaAgent.Performance.SuccessfulDecisions)
}

func TestHierarchyManager_AggregateResults_Voting(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, _, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with Voting aggregation
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)
	metaAgent.AggregationPolicy = AggregationVoting

	// Create results where 3 vote BUY, 2 vote SELL
	results := []*SubAgentResult{
		{AgentName: "agent-1", Result: "BUY", Confidence: 0.9},
		{AgentName: "agent-2", Result: "BUY", Confidence: 0.8},
		{AgentName: "agent-3", Result: "BUY", Confidence: 0.7},
		{AgentName: "agent-4", Result: "SELL", Confidence: 0.6},
		{AgentName: "agent-5", Result: "SELL", Confidence: 0.5},
	}

	// Aggregate results
	decision, err := hm.AggregateResults(ctx, "meta-trader", results)
	require.NoError(t, err)
	assert.NotNil(t, decision)

	// Majority should be BUY
	assert.Equal(t, "BUY", decision.FinalDecision)
	assert.Equal(t, 0.6, decision.Confidence) // 3/5 = 0.6
	assert.Contains(t, decision.Rationale, "Majority vote")
}

func TestHierarchyManager_AggregateResults_BestScore(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, _, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with BestScore aggregation
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)
	metaAgent.AggregationPolicy = AggregationBestScore

	// Create results with different confidences
	results := []*SubAgentResult{
		{AgentName: "agent-1", Result: "BUY", Confidence: 0.7},
		{AgentName: "agent-2", Result: "SELL", Confidence: 0.9}, // Highest
		{AgentName: "agent-3", Result: "HOLD", Confidence: 0.6},
	}

	// Aggregate results
	decision, err := hm.AggregateResults(ctx, "meta-trader", results)
	require.NoError(t, err)
	assert.NotNil(t, decision)

	// Should select SELL (highest confidence)
	assert.Equal(t, "SELL", decision.FinalDecision)
	assert.Equal(t, 0.9, decision.Confidence)
	assert.Contains(t, decision.Rationale, "agent-2")
}

func TestHierarchyManager_AggregateResults_Consensus(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, _, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Create meta-agent with Consensus aggregation
	metaAgent, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)
	metaAgent.AggregationPolicy = AggregationConsensus

	// Test with consensus
	results := []*SubAgentResult{
		{AgentName: "agent-1", Result: "BUY", Confidence: 0.9},
		{AgentName: "agent-2", Result: "BUY", Confidence: 0.8},
		{AgentName: "agent-3", Result: "BUY", Confidence: 0.7},
	}

	decision, err := hm.AggregateResults(ctx, "meta-trader", results)
	require.NoError(t, err)
	assert.NotNil(t, decision)

	assert.Equal(t, "BUY", decision.FinalDecision)
	assert.Equal(t, 1.0, decision.Confidence)
	assert.Contains(t, decision.Rationale, "consensus")

	// Test without consensus
	results = []*SubAgentResult{
		{AgentName: "agent-1", Result: "BUY", Confidence: 0.9},
		{AgentName: "agent-2", Result: "SELL", Confidence: 0.8},
		{AgentName: "agent-3", Result: "BUY", Confidence: 0.7},
	}

	decision, err = hm.AggregateResults(ctx, "meta-trader", results)
	require.NoError(t, err)
	assert.NotNil(t, decision)

	assert.Equal(t, 0.0, decision.Confidence)
	assert.Contains(t, decision.Rationale, "No consensus")
}

func TestHierarchyManager_UpdateSubAgentWeight(t *testing.T) {
	ctx := context.Background()
	blackboard, messageBus, hotSwap, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	// Setup meta-agent with sub-agent
	_, err := hm.CreateMetaAgent(ctx, "meta-trader", "meta", map[string]interface{}{})
	require.NoError(t, err)

	subAgent := createTestSubAgent("technical-agent", "technical")
	err = hotSwap.RegisterAgent(ctx, subAgent)
	require.NoError(t, err)

	err = hm.AddSubAgent(ctx, "meta-trader", "technical-agent", 0.7, nil)
	require.NoError(t, err)

	// Update weight
	err = hm.UpdateSubAgentWeight("meta-trader", "technical-agent", 0.95)
	require.NoError(t, err)

	// Verify weight updated
	subs, err := hm.GetSubAgents("meta-trader")
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, 0.95, subs[0].Weight)
}

func TestHierarchyManager_ConditionEvaluation(t *testing.T) {
	blackboard, messageBus, _, hm := setupHierarchyTest(t)
	defer cleanupHierarchy(blackboard, messageBus)

	situation := &Situation{
		Volatility:       0.05,
		TrendStrength:    0.8,
		LiquidityLevel:   "high",
		SentimentScore:   0.7,
		TimeOfDay:        14, // 2 PM
		MarketConditions: map[string]interface{}{"price": 50000.0},
		PortfolioState:   map[string]interface{}{"position_size": 1.5},
		AssessedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	tests := []struct {
		name      string
		condition ActivationCondition
		expected  bool
	}{
		{
			name:      "volatility greater than",
			condition: ActivationCondition{Field: "volatility", Operator: ">", Value: 0.03},
			expected:  true,
		},
		{
			name:      "volatility less than",
			condition: ActivationCondition{Field: "volatility", Operator: "<", Value: 0.03},
			expected:  false,
		},
		{
			name:      "trend strength equals",
			condition: ActivationCondition{Field: "trend_strength", Operator: ">=", Value: 0.8},
			expected:  true,
		},
		{
			name:      "liquidity equals",
			condition: ActivationCondition{Field: "liquidity_level", Operator: "==", Value: "high"},
			expected:  true,
		},
		{
			name:      "liquidity not equals",
			condition: ActivationCondition{Field: "liquidity_level", Operator: "!=", Value: "low"},
			expected:  true,
		},
		{
			name:      "time of day in range",
			condition: ActivationCondition{Field: "time_of_day", Operator: ">", Value: 9},
			expected:  true,
		},
		{
			name:      "custom field from market conditions",
			condition: ActivationCondition{Field: "price", Operator: ">", Value: 45000.0},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hm.evaluateCondition(tt.condition, situation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions

func setupHierarchyTest(t *testing.T) (*Blackboard, *MessageBus, *HotSwapCoordinator, *HierarchyManager) {
	t.Helper()

	blackboard, _ := setupTestBlackboard(t)
	messageBus, _ := setupTestMessageBus(t)
	hotSwap := NewHotSwapCoordinator(blackboard, messageBus)
	hm := NewHierarchyManager(hotSwap, blackboard, messageBus)

	return blackboard, messageBus, hotSwap, hm
}

func cleanupHierarchy(blackboard *Blackboard, messageBus *MessageBus) {
	if blackboard != nil {
		_ = blackboard.Close() // Test cleanup
	}
	if messageBus != nil {
		_ = messageBus.Close() // Test cleanup
	}
}

func createTestSubAgent(name, agentType string) *AgentRegistration {
	return &AgentRegistration{
		Name:         name,
		Type:         agentType,
		Version:      "1.0.0",
		Status:       AgentStatusActive,
		Capabilities: []string{"analysis", "trading"},
		State: &AgentState{
			Memory:             make(map[string]interface{}),
			Configuration:      make(map[string]interface{}),
			PerformanceMetrics: &PerformanceMetrics{},
			LastUpdated:        time.Now(),
		},
		Metadata:      make(map[string]interface{}),
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}
}

func postDataToBlackboard(ctx context.Context, blackboard *Blackboard, topic string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &BlackboardMessage{
		ID:        uuid.New(),
		Topic:     topic,
		AgentName: "test",
		Content:   jsonData,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
	}

	return blackboard.Post(ctx, msg)
}
