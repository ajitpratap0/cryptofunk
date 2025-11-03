package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// HotSwapCoordinator manages agent hot-swapping without downtime
type HotSwapCoordinator struct {
	blackboard *Blackboard
	messageBus *MessageBus
	agents     map[string]*AgentRegistration // Agent name -> registration
	swaps      map[uuid.UUID]*SwapSession    // Swap ID -> session
	mu         sync.RWMutex
}

// AgentRegistration tracks registered agents
type AgentRegistration struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"` // e.g., "technical", "trend", "risk"
	Version       string                 `json:"version"`
	Status        AgentStatus            `json:"status"`
	Capabilities  []string               `json:"capabilities"`
	Subscriptions []*Subscription        `json:"-"` // Active message subscriptions
	State         *AgentState            `json:"state,omitempty"`
	Metadata      map[string]interface{} `json:"metadata"`
	RegisteredAt  time.Time              `json:"registered_at"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	MessageCount  int64                  `json:"message_count"`
	ErrorCount    int64                  `json:"error_count"`
}

// AgentStatus represents agent operational status
type AgentStatus string

const (
	AgentStatusActive      AgentStatus = "active"
	AgentStatusPaused      AgentStatus = "paused"
	AgentStatusSwapping    AgentStatus = "swapping"
	AgentStatusTerminating AgentStatus = "terminating"
	AgentStatusOffline     AgentStatus = "offline"
)

// AgentState represents the complete state of an agent
type AgentState struct {
	Memory             map[string]interface{} `json:"memory"`        // Agent's working memory
	PendingTasks       []*AgentTask           `json:"pending_tasks"` // Tasks in queue
	ActiveTask         *AgentTask             `json:"active_task"`   // Currently executing task
	History            []*StateSnapshot       `json:"history"`       // Recent state snapshots
	Configuration      map[string]interface{} `json:"configuration"` // Agent configuration
	PerformanceMetrics *PerformanceMetrics    `json:"performance_metrics"`
	LastUpdated        time.Time              `json:"last_updated"`
}

// AgentTask represents a task assigned to an agent
type AgentTask struct {
	ID          uuid.UUID              `json:"id"`
	Type        string                 `json:"type"`
	Payload     json.RawMessage        `json:"payload"`
	Priority    int                    `json:"priority"`
	Status      TaskStatus             `json:"status"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// TaskStatus represents task execution status
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// PerformanceMetrics tracks agent performance
type PerformanceMetrics struct {
	TotalTasks       int64         `json:"total_tasks"`
	SuccessfulTasks  int64         `json:"successful_tasks"`
	FailedTasks      int64         `json:"failed_tasks"`
	AverageLatency   time.Duration `json:"average_latency"`
	LastTaskDuration time.Duration `json:"last_task_duration"`
	Uptime           time.Duration `json:"uptime"`
	CPUUsage         float64       `json:"cpu_usage,omitempty"`
	MemoryUsage      int64         `json:"memory_usage,omitempty"`
}

// StateSnapshot captures agent state at a point in time
type StateSnapshot struct {
	Timestamp time.Time              `json:"timestamp"`
	State     map[string]interface{} `json:"state"`
	Checksum  string                 `json:"checksum"` // For integrity verification
}

// SwapSession represents an active hot-swap operation
type SwapSession struct {
	ID            uuid.UUID     `json:"id"`
	OldAgentName  string        `json:"old_agent_name"`
	NewAgentName  string        `json:"new_agent_name"`
	Status        SwapStatus    `json:"status"`
	StateSnapshot *AgentState   `json:"state_snapshot"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	Duration      time.Duration `json:"duration"`
	Error         string        `json:"error,omitempty"`
	Steps         []*SwapStep   `json:"steps"`
}

// SwapStatus represents hot-swap status
type SwapStatus string

const (
	SwapStatusInitiating     SwapStatus = "initiating"
	SwapStatusCapturingState SwapStatus = "capturing_state"
	SwapStatusPausingOld     SwapStatus = "pausing_old"
	SwapStatusTransferring   SwapStatus = "transferring"
	SwapStatusStartingNew    SwapStatus = "starting_new"
	SwapStatusVerifying      SwapStatus = "verifying"
	SwapStatusCompleted      SwapStatus = "completed"
	SwapStatusFailed         SwapStatus = "failed"
	SwapStatusRolledBack     SwapStatus = "rolled_back"
)

// SwapStep represents a step in the hot-swap process
type SwapStep struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

// NewHotSwapCoordinator creates a new hot-swap coordinator
func NewHotSwapCoordinator(blackboard *Blackboard, messageBus *MessageBus) *HotSwapCoordinator {
	return &HotSwapCoordinator{
		blackboard: blackboard,
		messageBus: messageBus,
		agents:     make(map[string]*AgentRegistration),
		swaps:      make(map[uuid.UUID]*SwapSession),
	}
}

// RegisterAgent registers an agent with the coordinator
func (hsc *HotSwapCoordinator) RegisterAgent(ctx context.Context, agent *AgentRegistration) error {
	hsc.mu.Lock()
	defer hsc.mu.Unlock()

	if agent.Name == "" {
		return fmt.Errorf("agent name required")
	}

	// Initialize default values
	if agent.State == nil {
		agent.State = &AgentState{
			Memory:             make(map[string]interface{}),
			PendingTasks:       []*AgentTask{},
			History:            []*StateSnapshot{},
			Configuration:      make(map[string]interface{}),
			PerformanceMetrics: &PerformanceMetrics{},
			LastUpdated:        time.Now(),
		}
	}
	if agent.Metadata == nil {
		agent.Metadata = make(map[string]interface{})
	}
	if agent.Subscriptions == nil {
		agent.Subscriptions = []*Subscription{}
	}

	agent.Status = AgentStatusActive
	agent.RegisteredAt = time.Now()
	agent.LastHeartbeat = time.Now()

	hsc.agents[agent.Name] = agent

	log.Info().
		Str("agent", agent.Name).
		Str("type", agent.Type).
		Str("version", agent.Version).
		Msg("Agent registered")

	// Post to blackboard
	msg, _ := NewMessage("agent_registry", "orchestrator", agent)
	msg.WithMetadata("event", "agent_registered")
	if err := hsc.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post agent registration to blackboard")
	}

	return nil
}

// UnregisterAgent removes an agent from the coordinator
func (hsc *HotSwapCoordinator) UnregisterAgent(ctx context.Context, agentName string) error {
	hsc.mu.Lock()
	defer hsc.mu.Unlock()

	agent, exists := hsc.agents[agentName]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentName)
	}

	// Unsubscribe from all topics
	for _, sub := range agent.Subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			log.Warn().Err(err).Str("agent", agentName).Msg("Failed to unsubscribe")
		}
	}

	delete(hsc.agents, agentName)

	log.Info().Str("agent", agentName).Msg("Agent unregistered")

	// Post to blackboard
	msg, _ := NewMessage("agent_registry", "orchestrator", map[string]interface{}{
		"agent": agentName,
		"event": "agent_unregistered",
	})
	if err := hsc.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post agent unregistration to blackboard")
	}

	return nil
}

// SwapAgent performs a hot-swap of an agent
func (hsc *HotSwapCoordinator) SwapAgent(ctx context.Context, oldAgentName, newAgentName string, newAgentConfig map[string]interface{}) (*SwapSession, error) {
	hsc.mu.Lock()

	oldAgent, exists := hsc.agents[oldAgentName]
	if !exists {
		hsc.mu.Unlock()
		return nil, fmt.Errorf("old agent not found: %s", oldAgentName)
	}

	// Create swap session
	session := &SwapSession{
		ID:           uuid.New(),
		OldAgentName: oldAgentName,
		NewAgentName: newAgentName,
		Status:       SwapStatusInitiating,
		StartedAt:    time.Now(),
		Steps:        []*SwapStep{},
	}

	hsc.swaps[session.ID] = session
	hsc.mu.Unlock()

	log.Info().
		Str("swap_id", session.ID.String()).
		Str("old_agent", oldAgentName).
		Str("new_agent", newAgentName).
		Msg("Starting agent hot-swap")

	// Step 1: Capture state
	if err := hsc.captureState(ctx, session, oldAgent); err != nil {
		hsc.failSwap(session, err)
		return session, err
	}

	// Step 2: Pause old agent
	if err := hsc.pauseAgent(ctx, session, oldAgent); err != nil {
		hsc.failSwap(session, err)
		return session, err
	}

	// Step 3: Register new agent
	newAgent := &AgentRegistration{
		Name:          newAgentName,
		Type:          oldAgent.Type,
		Version:       fmt.Sprintf("%s-swap", oldAgent.Version),
		Capabilities:  oldAgent.Capabilities,
		State:         session.StateSnapshot, // Transfer state
		Metadata:      newAgentConfig,
		Subscriptions: []*Subscription{},
	}

	if err := hsc.transferState(ctx, session, newAgent); err != nil {
		hsc.failSwap(session, err)
		// Attempt rollback
		hsc.resumeAgent(ctx, oldAgent)
		return session, err
	}

	// Step 4: Start new agent
	if err := hsc.startNewAgent(ctx, session, newAgent); err != nil {
		hsc.failSwap(session, err)
		hsc.resumeAgent(ctx, oldAgent)
		return session, err
	}

	// Step 5: Verify new agent
	if err := hsc.verifyAgent(ctx, session, newAgent); err != nil {
		hsc.failSwap(session, err)
		hsc.resumeAgent(ctx, oldAgent)
		return session, err
	}

	// Step 6: Terminate old agent
	if err := hsc.terminateAgent(ctx, session, oldAgent); err != nil {
		log.Warn().Err(err).Msg("Failed to terminate old agent (non-fatal)")
	}

	// Complete swap
	hsc.completeSwap(session)

	log.Info().
		Str("swap_id", session.ID.String()).
		Dur("duration", session.Duration).
		Msg("Agent hot-swap completed successfully")

	return session, nil
}

// captureState captures the current state of an agent
func (hsc *HotSwapCoordinator) captureState(ctx context.Context, session *SwapSession, agent *AgentRegistration) error {
	step := hsc.addStep(session, "capture_state")
	session.Status = SwapStatusCapturingState

	// Deep copy the agent state
	stateJSON, err := json.Marshal(agent.State)
	if err != nil {
		return fmt.Errorf("failed to serialize agent state: %w", err)
	}

	var stateCopy AgentState
	if err := json.Unmarshal(stateJSON, &stateCopy); err != nil {
		return fmt.Errorf("failed to deserialize agent state: %w", err)
	}

	// Add current snapshot to history
	snapshot := &StateSnapshot{
		Timestamp: time.Now(),
		State: map[string]interface{}{
			"memory":        agent.State.Memory,
			"pending_tasks": len(agent.State.PendingTasks),
			"active_task":   agent.State.ActiveTask != nil,
		},
	}
	stateCopy.History = append(stateCopy.History, snapshot)

	session.StateSnapshot = &stateCopy

	hsc.completeStep(step)

	log.Debug().
		Str("swap_id", session.ID.String()).
		Int("pending_tasks", len(stateCopy.PendingTasks)).
		Msg("State captured")

	return nil
}

// pauseAgent pauses an agent's operations
func (hsc *HotSwapCoordinator) pauseAgent(ctx context.Context, session *SwapSession, agent *AgentRegistration) error {
	step := hsc.addStep(session, "pause_agent")
	session.Status = SwapStatusPausingOld

	hsc.mu.Lock()
	agent.Status = AgentStatusPaused
	hsc.mu.Unlock()

	// Send pause notification to agent
	msg, _ := NewAgentMessage("orchestrator", agent.Name, "control", map[string]interface{}{
		"command": "pause",
		"swap_id": session.ID.String(),
		"reason":  "hot_swap",
	})
	if err := hsc.messageBus.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send pause message: %w", err)
	}

	// Wait briefly for agent to pause
	time.Sleep(100 * time.Millisecond)

	hsc.completeStep(step)

	log.Debug().
		Str("swap_id", session.ID.String()).
		Str("agent", agent.Name).
		Msg("Agent paused")

	return nil
}

// transferState transfers state to the new agent
func (hsc *HotSwapCoordinator) transferState(ctx context.Context, session *SwapSession, newAgent *AgentRegistration) error {
	step := hsc.addStep(session, "transfer_state")
	session.Status = SwapStatusTransferring

	// Register new agent with transferred state
	if err := hsc.RegisterAgent(ctx, newAgent); err != nil {
		return fmt.Errorf("failed to register new agent: %w", err)
	}

	// Post state transfer notification to blackboard
	msg, _ := NewMessage("agent_swaps", "orchestrator", map[string]interface{}{
		"swap_id":    session.ID.String(),
		"old_agent":  session.OldAgentName,
		"new_agent":  session.NewAgentName,
		"state_size": len(session.StateSnapshot.PendingTasks),
	})
	if err := hsc.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post state transfer to blackboard")
	}

	hsc.completeStep(step)

	log.Debug().
		Str("swap_id", session.ID.String()).
		Str("new_agent", newAgent.Name).
		Msg("State transferred")

	return nil
}

// startNewAgent starts the new agent
func (hsc *HotSwapCoordinator) startNewAgent(ctx context.Context, session *SwapSession, newAgent *AgentRegistration) error {
	step := hsc.addStep(session, "start_new_agent")
	session.Status = SwapStatusStartingNew

	// Send start notification to new agent
	msg, _ := NewAgentMessage("orchestrator", newAgent.Name, "control", map[string]interface{}{
		"command":           "start",
		"swap_id":           session.ID.String(),
		"transferred_state": session.StateSnapshot,
	})
	if err := hsc.messageBus.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}

	// Wait for agent to initialize
	time.Sleep(200 * time.Millisecond)

	hsc.completeStep(step)

	log.Debug().
		Str("swap_id", session.ID.String()).
		Str("agent", newAgent.Name).
		Msg("New agent started")

	return nil
}

// verifyAgent verifies the new agent is working correctly
func (hsc *HotSwapCoordinator) verifyAgent(ctx context.Context, session *SwapSession, newAgent *AgentRegistration) error {
	step := hsc.addStep(session, "verify_agent")
	session.Status = SwapStatusVerifying

	hsc.mu.RLock()
	agent, exists := hsc.agents[newAgent.Name]
	hsc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("new agent not found in registry")
	}

	if agent.Status != AgentStatusActive {
		return fmt.Errorf("new agent not active: %s", agent.Status)
	}

	// Send ping to verify responsiveness
	msg, _ := NewAgentMessage("orchestrator", newAgent.Name, "control", map[string]interface{}{
		"command": "ping",
		"swap_id": session.ID.String(),
	})

	// Request with timeout
	reply, err := hsc.messageBus.Request(ctx, msg, 2*time.Second)
	if err != nil {
		return fmt.Errorf("new agent not responsive: %w", err)
	}

	log.Debug().
		Str("swap_id", session.ID.String()).
		Str("reply_id", reply.ID.String()).
		Msg("New agent verified")

	hsc.completeStep(step)
	return nil
}

// terminateAgent terminates the old agent
func (hsc *HotSwapCoordinator) terminateAgent(ctx context.Context, session *SwapSession, agent *AgentRegistration) error {
	step := hsc.addStep(session, "terminate_old_agent")

	hsc.mu.Lock()
	agent.Status = AgentStatusTerminating
	hsc.mu.Unlock()

	// Send termination notification
	msg, _ := NewAgentMessage("orchestrator", agent.Name, "control", map[string]interface{}{
		"command": "terminate",
		"swap_id": session.ID.String(),
		"reason":  "replaced",
	})
	if err := hsc.messageBus.Send(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to send terminate message")
	}

	// Unregister old agent
	if err := hsc.UnregisterAgent(ctx, agent.Name); err != nil {
		log.Warn().Err(err).Msg("Failed to unregister old agent")
	}

	hsc.completeStep(step)

	log.Debug().
		Str("swap_id", session.ID.String()).
		Str("agent", agent.Name).
		Msg("Old agent terminated")

	return nil
}

// resumeAgent resumes a paused agent (for rollback)
func (hsc *HotSwapCoordinator) resumeAgent(ctx context.Context, agent *AgentRegistration) error {
	hsc.mu.Lock()
	agent.Status = AgentStatusActive
	hsc.mu.Unlock()

	msg, _ := NewAgentMessage("orchestrator", agent.Name, "control", map[string]interface{}{
		"command": "resume",
		"reason":  "swap_failed",
	})
	return hsc.messageBus.Send(ctx, msg)
}

// Helper methods for swap session management

func (hsc *HotSwapCoordinator) addStep(session *SwapSession, name string) *SwapStep {
	step := &SwapStep{
		Name:      name,
		Status:    "running",
		StartedAt: time.Now(),
	}
	session.Steps = append(session.Steps, step)
	return step
}

func (hsc *HotSwapCoordinator) completeStep(step *SwapStep) {
	now := time.Now()
	step.CompletedAt = &now
	step.Duration = now.Sub(step.StartedAt)
	step.Status = "completed"
}

func (hsc *HotSwapCoordinator) failSwap(session *SwapSession, err error) {
	session.Status = SwapStatusFailed
	session.Error = err.Error()
	now := time.Now()
	session.CompletedAt = &now
	session.Duration = now.Sub(session.StartedAt)

	log.Error().
		Err(err).
		Str("swap_id", session.ID.String()).
		Msg("Agent hot-swap failed")
}

func (hsc *HotSwapCoordinator) completeSwap(session *SwapSession) {
	session.Status = SwapStatusCompleted
	now := time.Now()
	session.CompletedAt = &now
	session.Duration = now.Sub(session.StartedAt)
}

// GetAgent retrieves agent registration
func (hsc *HotSwapCoordinator) GetAgent(agentName string) (*AgentRegistration, error) {
	hsc.mu.RLock()
	defer hsc.mu.RUnlock()

	agent, exists := hsc.agents[agentName]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentName)
	}

	return agent, nil
}

// ListAgents returns all registered agents
func (hsc *HotSwapCoordinator) ListAgents() []*AgentRegistration {
	hsc.mu.RLock()
	defer hsc.mu.RUnlock()

	agents := make([]*AgentRegistration, 0, len(hsc.agents))
	for _, agent := range hsc.agents {
		agents = append(agents, agent)
	}

	return agents
}

// GetSwapSession retrieves a swap session
func (hsc *HotSwapCoordinator) GetSwapSession(swapID uuid.UUID) (*SwapSession, error) {
	hsc.mu.RLock()
	defer hsc.mu.RUnlock()

	session, exists := hsc.swaps[swapID]
	if !exists {
		return nil, fmt.Errorf("swap session not found: %s", swapID)
	}

	return session, nil
}

// ListSwapSessions returns all swap sessions
func (hsc *HotSwapCoordinator) ListSwapSessions() []*SwapSession {
	hsc.mu.RLock()
	defer hsc.mu.RUnlock()

	sessions := make([]*SwapSession, 0, len(hsc.swaps))
	for _, session := range hsc.swaps {
		sessions = append(sessions, session)
	}

	return sessions
}

// UpdateAgentHeartbeat updates the last heartbeat timestamp
func (hsc *HotSwapCoordinator) UpdateAgentHeartbeat(agentName string) error {
	hsc.mu.Lock()
	defer hsc.mu.Unlock()

	agent, exists := hsc.agents[agentName]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentName)
	}

	agent.LastHeartbeat = time.Now()
	return nil
}

// UpdateAgentState updates agent state
func (hsc *HotSwapCoordinator) UpdateAgentState(agentName string, state *AgentState) error {
	hsc.mu.Lock()
	defer hsc.mu.Unlock()

	agent, exists := hsc.agents[agentName]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentName)
	}

	state.LastUpdated = time.Now()
	agent.State = state
	return nil
}
