package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// HierarchyManager manages hierarchical agent structures
type HierarchyManager struct {
	hotSwap    *HotSwapCoordinator
	blackboard *Blackboard
	messageBus *MessageBus
	metaAgents map[string]*MetaAgent     // Meta-agent name -> meta-agent
	hierarchy  map[string]*HierarchyNode // Agent name -> hierarchy info
	mu         sync.RWMutex
}

// MetaAgent represents a parent agent that coordinates sub-agents
type MetaAgent struct {
	*AgentRegistration
	SubAgents         []*SubAgentInfo        `json:"sub_agents"`
	DelegationPolicy  DelegationPolicy       `json:"delegation_policy"`
	AggregationPolicy AggregationPolicy      `json:"aggregation_policy"`
	ResourceLimits    *ResourceLimits        `json:"resource_limits"`
	CurrentSituation  *Situation             `json:"current_situation,omitempty"`
	Decisions         []*MetaDecision        `json:"decisions"`
	Performance       *MetaAgentPerformance  `json:"performance"`
	Config            map[string]interface{} `json:"config"`
}

// SubAgentInfo tracks information about a sub-agent
type SubAgentInfo struct {
	AgentName    string                 `json:"agent_name"`
	AgentType    string                 `json:"agent_type"`
	Active       bool                   `json:"active"`
	Weight       float64                `json:"weight"` // Weighting for aggregation
	Capabilities []string               `json:"capabilities"`
	Conditions   []ActivationCondition  `json:"conditions"` // When to activate
	Performance  *SubAgentPerformance   `json:"performance"`
	AddedAt      time.Time              `json:"added_at"`
	LastActive   *time.Time             `json:"last_active,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// DelegationPolicy defines how tasks are delegated to sub-agents
type DelegationPolicy string

const (
	DelegationRoundRobin DelegationPolicy = "round_robin" // Distribute evenly
	DelegationWeighted   DelegationPolicy = "weighted"    // Delegate based on weights
	DelegationBestFit    DelegationPolicy = "best_fit"    // Match task to most capable agent
	DelegationAll        DelegationPolicy = "all"         // All agents handle all tasks
	DelegationAuction    DelegationPolicy = "auction"     // Agents bid for tasks
)

// AggregationPolicy defines how sub-agent results are combined
type AggregationPolicy string

const (
	AggregationVoting    AggregationPolicy = "voting"     // Majority vote
	AggregationWeighted  AggregationPolicy = "weighted"   // Weighted average
	AggregationConsensus AggregationPolicy = "consensus"  // Require consensus
	AggregationBestScore AggregationPolicy = "best_score" // Highest scoring result
	AggregationEnsemble  AggregationPolicy = "ensemble"   // Combine all results
)

// ActivationCondition defines when a sub-agent should be activated
type ActivationCondition struct {
	Field    string      `json:"field"`    // e.g., "market_volatility", "trend_strength"
	Operator string      `json:"operator"` // "==", "!=", ">", "<", ">=", "<="
	Value    interface{} `json:"value"`
}

// ResourceLimits constrains meta-agent resource usage
type ResourceLimits struct {
	MaxActiveSubAgents int           `json:"max_active_sub_agents"`
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
	TaskTimeout        time.Duration `json:"task_timeout"`
	MemoryLimit        int64         `json:"memory_limit"` // bytes
	CPULimit           float64       `json:"cpu_limit"`    // percentage
}

// Situation represents the current trading situation
type Situation struct {
	MarketConditions map[string]interface{} `json:"market_conditions"`
	PortfolioState   map[string]interface{} `json:"portfolio_state"`
	Volatility       float64                `json:"volatility"`
	TrendStrength    float64                `json:"trend_strength"`
	LiquidityLevel   string                 `json:"liquidity_level"` // "high", "medium", "low"
	SentimentScore   float64                `json:"sentiment_score"`
	TimeOfDay        int                    `json:"time_of_day"` // hour
	AssessedAt       time.Time              `json:"assessed_at"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// MetaDecision represents a decision made by the meta-agent
type MetaDecision struct {
	ID               uuid.UUID              `json:"id"`
	SubAgentResults  []*SubAgentResult      `json:"sub_agent_results"`
	AggregatedResult interface{}            `json:"aggregated_result"`
	FinalDecision    interface{}            `json:"final_decision"`
	Confidence       float64                `json:"confidence"`
	Rationale        string                 `json:"rationale"`
	DecidedAt        time.Time              `json:"decided_at"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// SubAgentResult represents a result from a sub-agent
type SubAgentResult struct {
	AgentName  string                 `json:"agent_name"`
	Result     interface{}            `json:"result"`
	Confidence float64                `json:"confidence"`
	Latency    time.Duration          `json:"latency"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// MetaAgentPerformance tracks meta-agent performance metrics
type MetaAgentPerformance struct {
	TotalDecisions      int64            `json:"total_decisions"`
	SuccessfulDecisions int64            `json:"successful_decisions"`
	FailedDecisions     int64            `json:"failed_decisions"`
	AverageConfidence   float64          `json:"average_confidence"`
	AverageLatency      time.Duration    `json:"average_latency"`
	SubAgentActivations map[string]int64 `json:"sub_agent_activations"` // Agent name -> count
	LastUpdated         time.Time        `json:"last_updated"`
}

// SubAgentPerformance tracks sub-agent performance under meta-agent
type SubAgentPerformance struct {
	TasksAssigned   int64         `json:"tasks_assigned"`
	TasksCompleted  int64         `json:"tasks_completed"`
	TasksFailed     int64         `json:"tasks_failed"`
	AverageLatency  time.Duration `json:"average_latency"`
	AverageAccuracy float64       `json:"average_accuracy"`
	LastTaskAt      *time.Time    `json:"last_task_at,omitempty"`
}

// HierarchyNode represents a node in the agent hierarchy
type HierarchyNode struct {
	AgentName   string    `json:"agent_name"`
	Level       int       `json:"level"`       // 0 = root, 1 = meta-agent, 2 = sub-agent
	ParentName  string    `json:"parent_name"` // Empty for root
	Children    []string  `json:"children"`    // Child agent names
	IsMetaAgent bool      `json:"is_meta_agent"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskAllocation represents a task allocated to a sub-agent
type TaskAllocation struct {
	TaskID      uuid.UUID              `json:"task_id"`
	MetaAgent   string                 `json:"meta_agent"`
	SubAgent    string                 `json:"sub_agent"`
	Task        *AgentTask             `json:"task"`
	AllocatedAt time.Time              `json:"allocated_at"`
	Status      TaskStatus             `json:"status"`
	Result      *SubAgentResult        `json:"result,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewHierarchyManager creates a new hierarchy manager
func NewHierarchyManager(hotSwap *HotSwapCoordinator, blackboard *Blackboard, messageBus *MessageBus) *HierarchyManager {
	return &HierarchyManager{
		hotSwap:    hotSwap,
		blackboard: blackboard,
		messageBus: messageBus,
		metaAgents: make(map[string]*MetaAgent),
		hierarchy:  make(map[string]*HierarchyNode),
	}
}

// CreateMetaAgent creates a new meta-agent
func (hm *HierarchyManager) CreateMetaAgent(ctx context.Context, name, agentType string, config map[string]interface{}) (*MetaAgent, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Check if already exists
	if _, exists := hm.metaAgents[name]; exists {
		return nil, fmt.Errorf("meta-agent %s already exists", name)
	}

	// Create base agent registration
	registration := &AgentRegistration{
		Name:         name,
		Type:         agentType,
		Version:      "1.0.0",
		Status:       AgentStatusActive,
		Capabilities: []string{"coordination", "aggregation", "delegation"},
		State: &AgentState{
			Memory:             make(map[string]interface{}),
			PendingTasks:       []*AgentTask{},
			Configuration:      config,
			PerformanceMetrics: &PerformanceMetrics{},
			LastUpdated:        time.Now(),
		},
		Metadata:      make(map[string]interface{}),
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	// Register with hot-swap coordinator
	if err := hm.hotSwap.RegisterAgent(ctx, registration); err != nil {
		return nil, fmt.Errorf("failed to register meta-agent: %w", err)
	}

	// Create meta-agent
	metaAgent := &MetaAgent{
		AgentRegistration: registration,
		SubAgents:         []*SubAgentInfo{},
		DelegationPolicy:  DelegationBestFit,
		AggregationPolicy: AggregationWeighted,
		ResourceLimits: &ResourceLimits{
			MaxActiveSubAgents: 10,
			MaxConcurrentTasks: 100,
			TaskTimeout:        5 * time.Minute,
		},
		Decisions: []*MetaDecision{},
		Performance: &MetaAgentPerformance{
			SubAgentActivations: make(map[string]int64),
			LastUpdated:         time.Now(),
		},
		Config: config,
	}

	hm.metaAgents[name] = metaAgent

	// Add to hierarchy
	hm.hierarchy[name] = &HierarchyNode{
		AgentName:   name,
		Level:       1, // Meta-agent level
		ParentName:  "",
		Children:    []string{},
		IsMetaAgent: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	log.Info().Str("meta_agent", name).Msg("Created meta-agent")
	return metaAgent, nil
}

// AddSubAgent adds a sub-agent to a meta-agent
func (hm *HierarchyManager) AddSubAgent(ctx context.Context, metaAgentName, subAgentName string, weight float64, conditions []ActivationCondition) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	metaAgent, exists := hm.metaAgents[metaAgentName]
	if !exists {
		return fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	// Get sub-agent registration
	subAgent, err := hm.hotSwap.GetAgent(subAgentName)
	if err != nil {
		return fmt.Errorf("sub-agent %s not found: %w", subAgentName, err)
	}

	// Check if already added
	for _, sa := range metaAgent.SubAgents {
		if sa.AgentName == subAgentName {
			return fmt.Errorf("sub-agent %s already added to meta-agent %s", subAgentName, metaAgentName)
		}
	}

	// Add sub-agent
	subAgentInfo := &SubAgentInfo{
		AgentName:    subAgentName,
		AgentType:    subAgent.Type,
		Active:       true,
		Weight:       weight,
		Capabilities: subAgent.Capabilities,
		Conditions:   conditions,
		Performance: &SubAgentPerformance{
			AverageAccuracy: 1.0, // Start optimistic
		},
		AddedAt:  time.Now(),
		Metadata: make(map[string]interface{}),
	}

	metaAgent.SubAgents = append(metaAgent.SubAgents, subAgentInfo)

	// Update hierarchy
	if node, exists := hm.hierarchy[metaAgentName]; exists {
		node.Children = append(node.Children, subAgentName)
		node.UpdatedAt = time.Now()
	}

	// Create hierarchy node for sub-agent if not exists
	if _, exists := hm.hierarchy[subAgentName]; !exists {
		hm.hierarchy[subAgentName] = &HierarchyNode{
			AgentName:   subAgentName,
			Level:       2, // Sub-agent level
			ParentName:  metaAgentName,
			Children:    []string{},
			IsMetaAgent: false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}

	log.Info().
		Str("meta_agent", metaAgentName).
		Str("sub_agent", subAgentName).
		Float64("weight", weight).
		Msg("Added sub-agent to meta-agent")

	return nil
}

// RemoveSubAgent removes a sub-agent from a meta-agent
func (hm *HierarchyManager) RemoveSubAgent(ctx context.Context, metaAgentName, subAgentName string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	metaAgent, exists := hm.metaAgents[metaAgentName]
	if !exists {
		return fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	// Find and remove sub-agent
	found := false
	newSubAgents := []*SubAgentInfo{}
	for _, sa := range metaAgent.SubAgents {
		if sa.AgentName != subAgentName {
			newSubAgents = append(newSubAgents, sa)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("sub-agent %s not found in meta-agent %s", subAgentName, metaAgentName)
	}

	metaAgent.SubAgents = newSubAgents

	// Update hierarchy
	if node, exists := hm.hierarchy[metaAgentName]; exists {
		children := []string{}
		for _, child := range node.Children {
			if child != subAgentName {
				children = append(children, child)
			}
		}
		node.Children = children
		node.UpdatedAt = time.Now()
	}

	// Remove hierarchy node for sub-agent
	delete(hm.hierarchy, subAgentName)

	log.Info().
		Str("meta_agent", metaAgentName).
		Str("sub_agent", subAgentName).
		Msg("Removed sub-agent from meta-agent")

	return nil
}

// AssessSituation analyzes the current situation
func (hm *HierarchyManager) AssessSituation(ctx context.Context, metaAgentName string) (*Situation, error) {
	hm.mu.RLock()
	metaAgent, exists := hm.metaAgents[metaAgentName]
	hm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	// Gather situation data from blackboard
	situation := &Situation{
		MarketConditions: make(map[string]interface{}),
		PortfolioState:   make(map[string]interface{}),
		AssessedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	// Read market data from blackboard
	if messages, err := hm.blackboard.GetByTopic(ctx, "market_data", 1); err == nil && len(messages) > 0 {
		if err := json.Unmarshal(messages[0].Content, &situation.MarketConditions); err == nil {
			// Extract specific metrics
			if vol, ok := situation.MarketConditions["volatility"].(float64); ok {
				situation.Volatility = vol
			}
			if trend, ok := situation.MarketConditions["trend_strength"].(float64); ok {
				situation.TrendStrength = trend
			}
			if liq, ok := situation.MarketConditions["liquidity"].(string); ok {
				situation.LiquidityLevel = liq
			}
			if sent, ok := situation.MarketConditions["sentiment"].(float64); ok {
				situation.SentimentScore = sent
			}
		}
	}

	// Read portfolio state
	if messages, err := hm.blackboard.GetByTopic(ctx, "portfolio_state", 1); err == nil && len(messages) > 0 {
		json.Unmarshal(messages[0].Content, &situation.PortfolioState)
	}

	// Time of day
	situation.TimeOfDay = time.Now().Hour()

	// Update meta-agent's current situation
	hm.mu.Lock()
	metaAgent.CurrentSituation = situation
	hm.mu.Unlock()

	log.Debug().
		Str("meta_agent", metaAgentName).
		Float64("volatility", situation.Volatility).
		Float64("trend_strength", situation.TrendStrength).
		Msg("Assessed situation")

	return situation, nil
}

// SelectSubAgents selects appropriate sub-agents based on situation
func (hm *HierarchyManager) SelectSubAgents(ctx context.Context, metaAgentName string, situation *Situation) ([]*SubAgentInfo, error) {
	hm.mu.RLock()
	metaAgent, exists := hm.metaAgents[metaAgentName]
	hm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	selected := []*SubAgentInfo{}

	for _, subAgent := range metaAgent.SubAgents {
		if hm.evaluateConditions(subAgent.Conditions, situation) {
			selected = append(selected, subAgent)
		}
	}

	// Limit to max active sub-agents
	if len(selected) > metaAgent.ResourceLimits.MaxActiveSubAgents {
		// Sort by weight (descending)
		sort.Slice(selected, func(i, j int) bool {
			return selected[i].Weight > selected[j].Weight
		})
		selected = selected[:metaAgent.ResourceLimits.MaxActiveSubAgents]
	}

	log.Debug().
		Str("meta_agent", metaAgentName).
		Int("selected_count", len(selected)).
		Int("total_sub_agents", len(metaAgent.SubAgents)).
		Msg("Selected sub-agents")

	return selected, nil
}

// evaluateConditions checks if conditions are met
func (hm *HierarchyManager) evaluateConditions(conditions []ActivationCondition, situation *Situation) bool {
	if len(conditions) == 0 {
		return true // No conditions = always active
	}

	for _, cond := range conditions {
		if !hm.evaluateCondition(cond, situation) {
			return false // All conditions must be true
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (hm *HierarchyManager) evaluateCondition(cond ActivationCondition, situation *Situation) bool {
	var actualValue interface{}

	// Map field to situation value
	switch cond.Field {
	case "volatility":
		actualValue = situation.Volatility
	case "trend_strength":
		actualValue = situation.TrendStrength
	case "liquidity_level":
		actualValue = situation.LiquidityLevel
	case "sentiment_score":
		actualValue = situation.SentimentScore
	case "time_of_day":
		actualValue = situation.TimeOfDay
	default:
		// Check in market conditions or portfolio state
		if val, ok := situation.MarketConditions[cond.Field]; ok {
			actualValue = val
		} else if val, ok := situation.PortfolioState[cond.Field]; ok {
			actualValue = val
		} else {
			return false // Field not found
		}
	}

	return hm.compareValues(actualValue, cond.Operator, cond.Value)
}

// compareValues compares two values based on operator
func (hm *HierarchyManager) compareValues(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "==":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "!=":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case ">":
		return hm.numericCompare(actual, expected) > 0
	case ">=":
		return hm.numericCompare(actual, expected) >= 0
	case "<":
		return hm.numericCompare(actual, expected) < 0
	case "<=":
		return hm.numericCompare(actual, expected) <= 0
	default:
		return false
	}
}

// numericCompare compares numeric values
func (hm *HierarchyManager) numericCompare(a, b interface{}) int {
	aFloat := hm.toFloat64(a)
	bFloat := hm.toFloat64(b)

	if aFloat > bFloat {
		return 1
	} else if aFloat < bFloat {
		return -1
	}
	return 0
}

// toFloat64 converts interface{} to float64
func (hm *HierarchyManager) toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	default:
		return 0
	}
}

// DelegateTask delegates a task to sub-agents based on policy
func (hm *HierarchyManager) DelegateTask(ctx context.Context, metaAgentName string, task *AgentTask, selectedAgents []*SubAgentInfo) ([]*TaskAllocation, error) {
	hm.mu.RLock()
	metaAgent, exists := hm.metaAgents[metaAgentName]
	hm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	allocations := []*TaskAllocation{}

	switch metaAgent.DelegationPolicy {
	case DelegationAll:
		// All selected agents handle task
		for _, subAgent := range selectedAgents {
			allocation := &TaskAllocation{
				TaskID:      uuid.New(),
				MetaAgent:   metaAgentName,
				SubAgent:    subAgent.AgentName,
				Task:        task,
				AllocatedAt: time.Now(),
				Status:      TaskStatusPending,
				Metadata:    make(map[string]interface{}),
			}
			allocations = append(allocations, allocation)
		}

	case DelegationBestFit:
		// Select best agent based on capabilities and performance
		bestAgent := hm.selectBestAgent(selectedAgents, task)
		if bestAgent != nil {
			allocation := &TaskAllocation{
				TaskID:      uuid.New(),
				MetaAgent:   metaAgentName,
				SubAgent:    bestAgent.AgentName,
				Task:        task,
				AllocatedAt: time.Now(),
				Status:      TaskStatusPending,
				Metadata:    make(map[string]interface{}),
			}
			allocations = append(allocations, allocation)
		}

	case DelegationWeighted:
		// Probabilistic selection based on weights
		selectedAgent := hm.weightedSelection(selectedAgents)
		if selectedAgent != nil {
			allocation := &TaskAllocation{
				TaskID:      uuid.New(),
				MetaAgent:   metaAgentName,
				SubAgent:    selectedAgent.AgentName,
				Task:        task,
				AllocatedAt: time.Now(),
				Status:      TaskStatusPending,
				Metadata:    make(map[string]interface{}),
			}
			allocations = append(allocations, allocation)
		}

	case DelegationRoundRobin:
		// Simple round-robin (just select first available)
		if len(selectedAgents) > 0 {
			allocation := &TaskAllocation{
				TaskID:      uuid.New(),
				MetaAgent:   metaAgentName,
				SubAgent:    selectedAgents[0].AgentName,
				Task:        task,
				AllocatedAt: time.Now(),
				Status:      TaskStatusPending,
				Metadata:    make(map[string]interface{}),
			}
			allocations = append(allocations, allocation)
		}
	}

	log.Debug().
		Str("meta_agent", metaAgentName).
		Str("policy", string(metaAgent.DelegationPolicy)).
		Int("allocations", len(allocations)).
		Msg("Delegated task")

	return allocations, nil
}

// selectBestAgent selects the best sub-agent for a task
func (hm *HierarchyManager) selectBestAgent(agents []*SubAgentInfo, task *AgentTask) *SubAgentInfo {
	if len(agents) == 0 {
		return nil
	}

	// Score each agent
	bestAgent := agents[0]
	bestScore := hm.scoreAgent(bestAgent, task)

	for _, agent := range agents[1:] {
		score := hm.scoreAgent(agent, task)
		if score > bestScore {
			bestScore = score
			bestAgent = agent
		}
	}

	return bestAgent
}

// scoreAgent calculates a score for agent suitability
func (hm *HierarchyManager) scoreAgent(agent *SubAgentInfo, task *AgentTask) float64 {
	// Base score from weight
	score := agent.Weight

	// Boost for high accuracy
	score += agent.Performance.AverageAccuracy * 0.3

	// Penalty for high latency
	if agent.Performance.AverageLatency > 1*time.Second {
		score -= 0.2
	}

	// Boost for low failure rate
	if agent.Performance.TasksAssigned > 0 {
		successRate := float64(agent.Performance.TasksCompleted) / float64(agent.Performance.TasksAssigned)
		score += successRate * 0.2
	}

	return score
}

// weightedSelection selects an agent probabilistically based on weights
func (hm *HierarchyManager) weightedSelection(agents []*SubAgentInfo) *SubAgentInfo {
	if len(agents) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0.0
	for _, agent := range agents {
		totalWeight += agent.Weight
	}

	// For simplicity, just select the highest weighted agent
	// In production, this would use random weighted selection
	bestAgent := agents[0]
	for _, agent := range agents[1:] {
		if agent.Weight > bestAgent.Weight {
			bestAgent = agent
		}
	}

	return bestAgent
}

// AggregateResults aggregates results from sub-agents
func (hm *HierarchyManager) AggregateResults(ctx context.Context, metaAgentName string, results []*SubAgentResult) (*MetaDecision, error) {
	hm.mu.RLock()
	metaAgent, exists := hm.metaAgents[metaAgentName]
	hm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	decision := &MetaDecision{
		ID:              uuid.New(),
		SubAgentResults: results,
		DecidedAt:       time.Now(),
		Metadata:        make(map[string]interface{}),
	}

	switch metaAgent.AggregationPolicy {
	case AggregationWeighted:
		// Weighted average of confidences
		totalWeight := 0.0
		weightedSum := 0.0
		for _, result := range results {
			weight := hm.getSubAgentWeight(metaAgent, result.AgentName)
			totalWeight += weight
			weightedSum += result.Confidence * weight
		}
		if totalWeight > 0 {
			decision.Confidence = weightedSum / totalWeight
		}
		decision.Rationale = "Weighted aggregation of sub-agent results"

	case AggregationVoting:
		// Majority vote (count results)
		votes := make(map[string]int)
		for _, result := range results {
			key := fmt.Sprintf("%v", result.Result)
			votes[key]++
		}

		// Find majority
		maxVotes := 0
		var majority string
		for key, count := range votes {
			if count > maxVotes {
				maxVotes = count
				majority = key
			}
		}

		decision.FinalDecision = majority
		decision.Confidence = float64(maxVotes) / float64(len(results))
		decision.Rationale = fmt.Sprintf("Majority vote: %d/%d agents agreed", maxVotes, len(results))

	case AggregationBestScore:
		// Select result with highest confidence
		var best *SubAgentResult
		for _, result := range results {
			if best == nil || result.Confidence > best.Confidence {
				best = result
			}
		}
		if best != nil {
			decision.FinalDecision = best.Result
			decision.Confidence = best.Confidence
			decision.Rationale = fmt.Sprintf("Best result from %s", best.AgentName)
		}

	case AggregationConsensus:
		// Require all agents to agree
		if len(results) == 0 {
			decision.Confidence = 0
			decision.Rationale = "No results to aggregate"
		} else {
			firstResult := fmt.Sprintf("%v", results[0].Result)
			consensus := true
			for _, result := range results[1:] {
				if fmt.Sprintf("%v", result.Result) != firstResult {
					consensus = false
					break
				}
			}
			if consensus {
				decision.FinalDecision = results[0].Result
				decision.Confidence = 1.0
				decision.Rationale = "Full consensus achieved"
			} else {
				decision.Confidence = 0
				decision.Rationale = "No consensus"
			}
		}

	case AggregationEnsemble:
		// Combine all results (simple average for numeric results)
		decision.AggregatedResult = results
		avgConfidence := 0.0
		for _, result := range results {
			avgConfidence += result.Confidence
		}
		if len(results) > 0 {
			decision.Confidence = avgConfidence / float64(len(results))
		}
		decision.Rationale = "Ensemble of all sub-agent results"
	}

	// Store decision
	hm.mu.Lock()
	metaAgent.Decisions = append(metaAgent.Decisions, decision)
	hm.mu.Unlock()

	// Update performance metrics
	hm.updateMetaAgentPerformance(metaAgent, decision)

	log.Info().
		Str("meta_agent", metaAgentName).
		Str("policy", string(metaAgent.AggregationPolicy)).
		Float64("confidence", decision.Confidence).
		Int("results_count", len(results)).
		Msg("Aggregated results")

	return decision, nil
}

// getSubAgentWeight retrieves the weight of a sub-agent
func (hm *HierarchyManager) getSubAgentWeight(metaAgent *MetaAgent, agentName string) float64 {
	for _, sa := range metaAgent.SubAgents {
		if sa.AgentName == agentName {
			return sa.Weight
		}
	}
	return 1.0 // Default weight
}

// updateMetaAgentPerformance updates performance metrics
func (hm *HierarchyManager) updateMetaAgentPerformance(metaAgent *MetaAgent, decision *MetaDecision) {
	metaAgent.Performance.TotalDecisions++

	if decision.Confidence > 0.5 {
		metaAgent.Performance.SuccessfulDecisions++
	} else {
		metaAgent.Performance.FailedDecisions++
	}

	// Update average confidence
	total := metaAgent.Performance.TotalDecisions
	metaAgent.Performance.AverageConfidence =
		(metaAgent.Performance.AverageConfidence*float64(total-1) + decision.Confidence) / float64(total)

	metaAgent.Performance.LastUpdated = time.Now()
}

// GetMetaAgent retrieves a meta-agent
func (hm *HierarchyManager) GetMetaAgent(name string) (*MetaAgent, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	metaAgent, exists := hm.metaAgents[name]
	if !exists {
		return nil, fmt.Errorf("meta-agent %s not found", name)
	}

	return metaAgent, nil
}

// GetHierarchy retrieves the complete hierarchy
func (hm *HierarchyManager) GetHierarchy() map[string]*HierarchyNode {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	// Create a copy
	hierarchy := make(map[string]*HierarchyNode)
	for k, v := range hm.hierarchy {
		hierarchy[k] = v
	}

	return hierarchy
}

// GetSubAgents retrieves all sub-agents for a meta-agent
func (hm *HierarchyManager) GetSubAgents(metaAgentName string) ([]*SubAgentInfo, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	metaAgent, exists := hm.metaAgents[metaAgentName]
	if !exists {
		return nil, fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	return metaAgent.SubAgents, nil
}

// UpdateSubAgentWeight updates the weight of a sub-agent
func (hm *HierarchyManager) UpdateSubAgentWeight(metaAgentName, subAgentName string, newWeight float64) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	metaAgent, exists := hm.metaAgents[metaAgentName]
	if !exists {
		return fmt.Errorf("meta-agent %s not found", metaAgentName)
	}

	for _, sa := range metaAgent.SubAgents {
		if sa.AgentName == subAgentName {
			sa.Weight = newWeight
			log.Info().
				Str("meta_agent", metaAgentName).
				Str("sub_agent", subAgentName).
				Float64("new_weight", newWeight).
				Msg("Updated sub-agent weight")
			return nil
		}
	}

	return fmt.Errorf("sub-agent %s not found in meta-agent %s", subAgentName, metaAgentName)
}
