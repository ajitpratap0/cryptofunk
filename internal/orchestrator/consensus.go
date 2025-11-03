package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ConsensusMethod represents the type of consensus mechanism
type ConsensusMethod string

const (
	ConsensusDelphi      ConsensusMethod = "delphi"       // Iterative expert consensus
	ConsensusContractNet ConsensusMethod = "contract_net" // Task allocation through bidding
	ConsensusVoting      ConsensusMethod = "voting"       // Simple majority voting
	ConsensusWeighted    ConsensusMethod = "weighted"     // Weighted voting by confidence
)

// ConsensusManager coordinates consensus mechanisms between agents
type ConsensusManager struct {
	blackboard *Blackboard
	messageBus *MessageBus
	config     *ConsensusConfig
	sessions   map[uuid.UUID]*ConsensusSession
	timeoutSem chan struct{} // Semaphore for limiting concurrent timeout handlers
	mu         sync.RWMutex
}

// ConsensusSession represents an active consensus session
type ConsensusSession struct {
	ID           uuid.UUID              `json:"id"`
	Method       ConsensusMethod        `json:"method"`
	Topic        string                 `json:"topic"`
	Question     string                 `json:"question"`
	Participants []string               `json:"participants"` // Agent names
	Rounds       []*ConsensusRound      `json:"rounds"`
	Status       ConsensusStatus        `json:"status"`
	Result       *ConsensusResult       `json:"result,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
}

// ConsensusRound represents one iteration of the consensus process
type ConsensusRound struct {
	Number      int                  `json:"number"`
	Responses   map[string]*Response `json:"responses"` // Agent name -> Response
	Statistics  *RoundStatistics     `json:"statistics"`
	Feedback    string               `json:"feedback,omitempty"` // For Delphi method
	StartedAt   time.Time            `json:"started_at"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
}

// Response represents an agent's response in a consensus round
type Response struct {
	AgentName  string                 `json:"agent_name"`
	Value      interface{}            `json:"value"`      // Numeric value or decision
	Confidence float64                `json:"confidence"` // 0.0-1.0
	Reasoning  string                 `json:"reasoning"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
}

// RoundStatistics provides statistical summary of a round
type RoundStatistics struct {
	Mean        float64 `json:"mean"`
	Median      float64 `json:"median"`
	StdDev      float64 `json:"std_dev"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Range       float64 `json:"range"`
	Consensus   float64 `json:"consensus"`   // Measure of agreement (0.0-1.0)
	Convergence bool    `json:"convergence"` // Whether consensus is reached
}

// ConsensusStatus represents the state of a consensus session
type ConsensusStatus string

const (
	ConsensusStatusPending   ConsensusStatus = "pending"
	ConsensusStatusActive    ConsensusStatus = "active"
	ConsensusStatusConverged ConsensusStatus = "converged"
	ConsensusStatusFailed    ConsensusStatus = "failed"
	ConsensusStatusExpired   ConsensusStatus = "expired"
)

// ConsensusResult is the final outcome of consensus
type ConsensusResult struct {
	Decision   interface{}            `json:"decision"`
	Confidence float64                `json:"confidence"`
	Agreement  float64                `json:"agreement"` // Percentage of agents in agreement
	Rounds     int                    `json:"rounds"`
	Duration   time.Duration          `json:"duration"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ConsensusConfig configures consensus behavior
type ConsensusConfig struct {
	MaxRounds             int           `json:"max_rounds"`              // Maximum iterations per session
	ConvergenceThreshold  float64       `json:"convergence_threshold"`   // Agreement threshold (0.0-1.0)
	RoundTimeout          time.Duration `json:"round_timeout"`           // Time to wait for responses
	SessionTTL            time.Duration `json:"session_ttl"`             // Session expiration
	MinParticipants       int           `json:"min_participants"`        // Minimum agents required
	MaxActiveSessions     int           `json:"max_active_sessions"`     // Maximum concurrent sessions (resource limit)
	MaxParticipants       int           `json:"max_participants"`        // Maximum participants per session (resource limit)
	MaxConcurrentTimeouts int           `json:"max_concurrent_timeouts"` // Maximum concurrent timeout handlers (resource limit)
}

// DefaultConsensusConfig returns default configuration
func DefaultConsensusConfig() ConsensusConfig {
	return ConsensusConfig{
		MaxRounds:             5,
		ConvergenceThreshold:  0.8, // 80% agreement
		RoundTimeout:          30 * time.Second,
		SessionTTL:            5 * time.Minute,
		MinParticipants:       2,
		MaxActiveSessions:     10,  // Resource limit: prevent DoS
		MaxParticipants:       50,  // Resource limit: prevent exhaustion
		MaxConcurrentTimeouts: 100, // Resource limit: prevent goroutine exhaustion
	}
}

// NewConsensusManager creates a new consensus manager with default config
func NewConsensusManager(blackboard *Blackboard, messageBus *MessageBus) *ConsensusManager {
	config := DefaultConsensusConfig()
	return NewConsensusManagerWithConfig(blackboard, messageBus, &config)
}

// NewConsensusManagerWithConfig creates a new consensus manager with custom config
func NewConsensusManagerWithConfig(blackboard *Blackboard, messageBus *MessageBus, config *ConsensusConfig) *ConsensusManager {
	if config == nil {
		defaultConfig := DefaultConsensusConfig()
		config = &defaultConfig
	}
	return &ConsensusManager{
		blackboard: blackboard,
		messageBus: messageBus,
		config:     config,
		sessions:   make(map[uuid.UUID]*ConsensusSession),
		timeoutSem: make(chan struct{}, config.MaxConcurrentTimeouts),
	}
}

// StartDelphiConsensus initiates a Delphi method consensus session
func (cm *ConsensusManager) StartDelphiConsensus(ctx context.Context, topic, question string, participants []string, config ConsensusConfig) (*ConsensusSession, error) {
	// Resource limit checks
	cm.mu.RLock()
	activeCount := cm.countActiveSessions()
	cm.mu.RUnlock()

	if activeCount >= cm.config.MaxActiveSessions {
		return nil, fmt.Errorf("max active sessions reached: %d/%d", activeCount, cm.config.MaxActiveSessions)
	}

	if len(participants) > cm.config.MaxParticipants {
		return nil, fmt.Errorf("too many participants: got %d, max %d", len(participants), cm.config.MaxParticipants)
	}

	if len(participants) < config.MinParticipants {
		return nil, fmt.Errorf("insufficient participants: got %d, need %d", len(participants), config.MinParticipants)
	}

	session := &ConsensusSession{
		ID:           uuid.New(),
		Method:       ConsensusDelphi,
		Topic:        topic,
		Question:     question,
		Participants: participants,
		Rounds:       []*ConsensusRound{},
		Status:       ConsensusStatusPending,
		Metadata: map[string]interface{}{
			"config": config,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(config.SessionTTL),
	}

	cm.mu.Lock()
	cm.sessions[session.ID] = session
	cm.mu.Unlock()

	log.Info().
		Str("session_id", session.ID.String()).
		Str("method", string(ConsensusDelphi)).
		Str("topic", topic).
		Int("participants", len(participants)).
		Msg("Started Delphi consensus session")

	// Start first round
	if err := cm.startDelphiRound(ctx, session, config); err != nil {
		session.Status = ConsensusStatusFailed
		return nil, fmt.Errorf("failed to start first round: %w", err)
	}

	return session, nil
}

// startDelphiRound initiates a new round of Delphi consensus
func (cm *ConsensusManager) startDelphiRound(ctx context.Context, session *ConsensusSession, config ConsensusConfig) error {
	roundNum := len(session.Rounds) + 1

	round := &ConsensusRound{
		Number:    roundNum,
		Responses: make(map[string]*Response),
		StartedAt: time.Now(),
	}

	// Add feedback from previous round (if any)
	if roundNum > 1 {
		prevRound := session.Rounds[roundNum-2]
		if prevRound.Statistics != nil {
			round.Feedback = fmt.Sprintf(
				"Previous round results: Mean=%.2f, Median=%.2f, StdDev=%.2f, Consensus=%.2f%%",
				prevRound.Statistics.Mean,
				prevRound.Statistics.Median,
				prevRound.Statistics.StdDev,
				prevRound.Statistics.Consensus*100,
			)
		}
	}

	session.Rounds = append(session.Rounds, round)
	session.Status = ConsensusStatusActive
	session.UpdatedAt = time.Now()

	// Broadcast question to participants via MessageBus
	for _, agentName := range session.Participants {
		msg, err := NewAgentMessage("orchestrator", agentName, "consensus_request", map[string]interface{}{
			"session_id": session.ID.String(),
			"round":      roundNum,
			"question":   session.Question,
			"feedback":   round.Feedback,
			"topic":      session.Topic,
		})
		if err != nil {
			log.Warn().Err(err).Str("agent", agentName).Msg("Failed to create consensus request message")
			continue
		}

		if err := cm.messageBus.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("agent", agentName).Msg("Failed to send consensus request")
		}
	}

	log.Debug().
		Str("session_id", session.ID.String()).
		Int("round", roundNum).
		Str("feedback", round.Feedback).
		Msg("Started Delphi round")

	// Start timeout timer
	go cm.roundTimeoutHandler(ctx, session.ID, roundNum, config.RoundTimeout)

	return nil
}

// SubmitDelphiResponse processes an agent's response to a Delphi round
func (cm *ConsensusManager) SubmitDelphiResponse(ctx context.Context, sessionID uuid.UUID, agentName string, value float64, confidence float64, reasoning string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	session, exists := cm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != ConsensusStatusActive {
		return fmt.Errorf("session not active: %s", session.Status)
	}

	// Get current round
	if len(session.Rounds) == 0 {
		return fmt.Errorf("no active round")
	}
	currentRound := session.Rounds[len(session.Rounds)-1]

	// Check if agent is a participant
	isParticipant := false
	for _, p := range session.Participants {
		if p == agentName {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		return fmt.Errorf("agent not a participant: %s", agentName)
	}

	// Record response
	response := &Response{
		AgentName:  agentName,
		Value:      value,
		Confidence: confidence,
		Reasoning:  reasoning,
		Metadata:   make(map[string]interface{}),
		Timestamp:  time.Now(),
	}
	currentRound.Responses[agentName] = response

	log.Debug().
		Str("session_id", sessionID.String()).
		Int("round", currentRound.Number).
		Str("agent", agentName).
		Float64("value", value).
		Float64("confidence", confidence).
		Msg("Received Delphi response")

	// Check if all participants have responded
	if len(currentRound.Responses) == len(session.Participants) {
		config := session.Metadata["config"].(ConsensusConfig)
		cm.completeDelphiRound(ctx, session, currentRound, config)
	}

	return nil
}

// completeDelphiRound finalizes a round and checks for convergence
func (cm *ConsensusManager) completeDelphiRound(ctx context.Context, session *ConsensusSession, round *ConsensusRound, config ConsensusConfig) {
	now := time.Now()
	round.CompletedAt = &now

	// Calculate statistics
	round.Statistics = cm.calculateStatistics(round)

	session.UpdatedAt = time.Now()

	log.Info().
		Str("session_id", session.ID.String()).
		Int("round", round.Number).
		Float64("mean", round.Statistics.Mean).
		Float64("consensus", round.Statistics.Consensus).
		Bool("converged", round.Statistics.Convergence).
		Msg("Completed Delphi round")

	// Check for convergence
	if round.Statistics.Convergence || round.Number >= config.MaxRounds {
		cm.finalizeDelphiConsensus(session, round)
	} else {
		// Start next round
		if err := cm.startDelphiRound(ctx, session, config); err != nil {
			log.Error().Err(err).Msg("Failed to start next round")
			session.Status = ConsensusStatusFailed
		}
	}
}

// finalizeDelphiConsensus completes the consensus session
func (cm *ConsensusManager) finalizeDelphiConsensus(session *ConsensusSession, finalRound *ConsensusRound) {
	session.Status = ConsensusStatusConverged
	session.Result = &ConsensusResult{
		Decision:   finalRound.Statistics.Mean,
		Confidence: cm.calculateOverallConfidence(finalRound),
		Agreement:  finalRound.Statistics.Consensus,
		Rounds:     len(session.Rounds),
		Duration:   time.Since(session.CreatedAt),
		Metadata: map[string]interface{}{
			"final_statistics": finalRound.Statistics,
		},
	}
	session.UpdatedAt = time.Now()

	log.Info().
		Str("session_id", session.ID.String()).
		Int("rounds", session.Result.Rounds).
		Float64("decision", session.Result.Decision.(float64)).
		Float64("agreement", session.Result.Agreement).
		Dur("duration", session.Result.Duration).
		Msg("Delphi consensus reached")

	// Post result to blackboard
	msg, _ := NewMessage(session.Topic, "orchestrator", session.Result)
	msg.WithMetadata("consensus_session_id", session.ID.String())
	msg.WithMetadata("consensus_method", string(ConsensusDelphi))
	if err := cm.blackboard.Post(context.Background(), msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post consensus result to blackboard")
	}
}

// calculateStatistics computes statistical measures for a round
func (cm *ConsensusManager) calculateStatistics(round *ConsensusRound) *RoundStatistics {
	values := make([]float64, 0, len(round.Responses))
	for _, resp := range round.Responses {
		if v, ok := resp.Value.(float64); ok {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		return &RoundStatistics{}
	}

	sort.Float64s(values)

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate median
	median := 0.0
	if len(values)%2 == 0 {
		median = (values[len(values)/2-1] + values[len(values)/2]) / 2
	} else {
		median = values[len(values)/2]
	}

	// Calculate standard deviation
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	stdDev := math.Sqrt(variance / float64(len(values)))

	// Calculate consensus score (inverse of coefficient of variation)
	consensus := 0.0
	if mean != 0 {
		cv := stdDev / math.Abs(mean) // Coefficient of variation
		consensus = math.Max(0, 1.0-cv)
	} else if stdDev == 0 {
		consensus = 1.0 // Perfect consensus at zero
	}

	// Check convergence (consensus above threshold)
	convergence := consensus >= 0.8 // 80% threshold

	return &RoundStatistics{
		Mean:        mean,
		Median:      median,
		StdDev:      stdDev,
		Min:         values[0],
		Max:         values[len(values)-1],
		Range:       values[len(values)-1] - values[0],
		Consensus:   consensus,
		Convergence: convergence,
	}
}

// calculateOverallConfidence computes weighted average confidence
func (cm *ConsensusManager) calculateOverallConfidence(round *ConsensusRound) float64 {
	if len(round.Responses) == 0 {
		return 0.0
	}

	totalConfidence := 0.0
	for _, resp := range round.Responses {
		totalConfidence += resp.Confidence
	}

	return totalConfidence / float64(len(round.Responses))
}

// roundTimeoutHandler handles round timeout
func (cm *ConsensusManager) roundTimeoutHandler(ctx context.Context, sessionID uuid.UUID, roundNum int, timeout time.Duration) {
	// Acquire semaphore to limit concurrent timeout handlers (prevent goroutine exhaustion)
	select {
	case cm.timeoutSem <- struct{}{}:
		// Semaphore acquired
		defer func() { <-cm.timeoutSem }() // Release on exit
	case <-ctx.Done():
		// Context cancelled before acquiring semaphore
		return
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		cm.mu.Lock()
		defer cm.mu.Unlock()

		session, exists := cm.sessions[sessionID]
		if !exists || session.Status != ConsensusStatusActive {
			return
		}

		if len(session.Rounds) < roundNum {
			return
		}

		round := session.Rounds[roundNum-1]
		if round.CompletedAt != nil {
			return // Round already completed
		}

		log.Warn().
			Str("session_id", sessionID.String()).
			Int("round", roundNum).
			Int("responses", len(round.Responses)).
			Int("expected", len(session.Participants)).
			Msg("Round timeout - proceeding with partial responses")

		// Proceed with available responses if we have minimum
		config := session.Metadata["config"].(ConsensusConfig)
		if len(round.Responses) >= config.MinParticipants {
			cm.completeDelphiRound(ctx, session, round, config)
		} else {
			session.Status = ConsensusStatusFailed
			log.Error().
				Str("session_id", sessionID.String()).
				Int("round", roundNum).
				Msg("Insufficient responses for consensus")
		}
	}
}

// GetSession retrieves a consensus session by ID
func (cm *ConsensusManager) GetSession(sessionID uuid.UUID) (*ConsensusSession, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	session, exists := cm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListSessions returns all active consensus sessions
func (cm *ConsensusManager) ListSessions() []*ConsensusSession {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	sessions := make([]*ConsensusSession, 0, len(cm.sessions))
	for _, session := range cm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// CleanupExpiredSessions removes expired sessions
func (cm *ConsensusManager) CleanupExpiredSessions() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, session := range cm.sessions {
		if now.After(session.ExpiresAt) {
			if session.Status == ConsensusStatusActive || session.Status == ConsensusStatusPending {
				session.Status = ConsensusStatusExpired
			}
			delete(cm.sessions, id)
			removed++
		}
	}

	if removed > 0 {
		log.Info().Int("removed", removed).Msg("Cleaned up expired consensus sessions")
	}

	return removed
}

// Contract Net Protocol Implementation

// ContractNetTask represents a task to be allocated via Contract Net
type ContractNetTask struct {
	ID           uuid.UUID              `json:"id"`
	Description  string                 `json:"description"`
	Requirements map[string]interface{} `json:"requirements"` // Task requirements
	Deadline     time.Time              `json:"deadline"`
	Priority     int                    `json:"priority"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Bid represents a contractor's bid for a task
type Bid struct {
	TaskID       uuid.UUID              `json:"task_id"`
	AgentName    string                 `json:"agent_name"`
	Cost         float64                `json:"cost"`         // Estimated cost/effort
	Quality      float64                `json:"quality"`      // Expected quality (0.0-1.0)
	Deadline     time.Time              `json:"deadline"`     // Estimated completion time
	Capabilities map[string]interface{} `json:"capabilities"` // Agent capabilities
	Reasoning    string                 `json:"reasoning"`
	Timestamp    time.Time              `json:"timestamp"`
}

// Contract represents an awarded contract
type Contract struct {
	ID          uuid.UUID              `json:"id"`
	TaskID      uuid.UUID              `json:"task_id"`
	Task        *ContractNetTask       `json:"task"`
	Contractor  string                 `json:"contractor"`
	Bid         *Bid                   `json:"bid"`
	Status      ContractStatus         `json:"status"`
	Result      interface{}            `json:"result,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ContractStatus represents contract execution status
type ContractStatus string

const (
	ContractStatusAwarded    ContractStatus = "awarded"
	ContractStatusInProgress ContractStatus = "in_progress"
	ContractStatusCompleted  ContractStatus = "completed"
	ContractStatusFailed     ContractStatus = "failed"
	ContractStatusCancelled  ContractStatus = "cancelled"
)

// StartContractNet initiates a Contract Net protocol session for task allocation
func (cm *ConsensusManager) StartContractNet(ctx context.Context, task *ContractNetTask, eligibleAgents []string, bidTimeout time.Duration) (*Contract, error) {
	if len(eligibleAgents) == 0 {
		return nil, fmt.Errorf("no eligible agents for task")
	}

	// Resource limit check: prevent excessive participants
	if len(eligibleAgents) > cm.config.MaxParticipants {
		return nil, fmt.Errorf("too many eligible agents: got %d, max %d", len(eligibleAgents), cm.config.MaxParticipants)
	}

	task.ID = uuid.New()

	log.Info().
		Str("task_id", task.ID.String()).
		Str("description", task.Description).
		Int("eligible_agents", len(eligibleAgents)).
		Msg("Starting Contract Net protocol")

	// Announce task to eligible agents via MessageBus
	for _, agentName := range eligibleAgents {
		msg, err := NewAgentMessage("orchestrator", agentName, "task_announcement", map[string]interface{}{
			"task_id":      task.ID.String(),
			"description":  task.Description,
			"requirements": task.Requirements,
			"deadline":     task.Deadline.Format(time.RFC3339),
			"priority":     task.Priority,
		})
		if err != nil {
			log.Warn().Err(err).Str("agent", agentName).Msg("Failed to create task announcement")
			continue
		}

		if err := cm.messageBus.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("agent", agentName).Msg("Failed to send task announcement")
		}
	}

	// Wait for bids
	bids := cm.collectBids(ctx, task.ID, bidTimeout)

	if len(bids) == 0 {
		log.Warn().Str("task_id", task.ID.String()).Msg("No bids received for task")
		return nil, fmt.Errorf("no bids received for task")
	}

	// Select best bid
	bestBid := cm.selectBestBid(bids, task)

	// Award contract
	contract := &Contract{
		ID:         uuid.New(),
		TaskID:     task.ID,
		Task:       task,
		Contractor: bestBid.AgentName,
		Bid:        bestBid,
		Status:     ContractStatusAwarded,
		StartedAt:  time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	// Notify winning bidder
	awardMsg, _ := NewAgentMessage("orchestrator", bestBid.AgentName, "contract_awarded", map[string]interface{}{
		"contract_id": contract.ID.String(),
		"task_id":     task.ID.String(),
		"task":        task,
	})
	if err := cm.messageBus.Send(ctx, awardMsg); err != nil {
		log.Error().Err(err).Str("agent", bestBid.AgentName).Msg("Failed to notify contract award")
		return nil, fmt.Errorf("failed to notify contract award: %w", err)
	}

	// Notify losing bidders
	for _, bid := range bids {
		if bid.AgentName != bestBid.AgentName {
			rejectMsg, _ := NewAgentMessage("orchestrator", bid.AgentName, "bid_rejected", map[string]interface{}{
				"task_id": task.ID.String(),
				"reason":  "Another agent selected",
			})
			cm.messageBus.Send(ctx, rejectMsg)
		}
	}

	log.Info().
		Str("contract_id", contract.ID.String()).
		Str("task_id", task.ID.String()).
		Str("contractor", contract.Contractor).
		Int("bids_received", len(bids)).
		Msg("Contract awarded")

	// Post to blackboard
	msg, _ := NewMessage("contracts", "orchestrator", contract)
	msg.WithMetadata("task_id", task.ID.String())
	msg.WithMetadata("contractor", contract.Contractor)
	if err := cm.blackboard.Post(ctx, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to post contract to blackboard")
	}

	return contract, nil
}

// collectBids waits for bids from agents
func (cm *ConsensusManager) collectBids(ctx context.Context, taskID uuid.UUID, timeout time.Duration) []*Bid {
	bids := make([]*Bid, 0)
	bidChan := make(chan *Bid, 10)

	// Subscribe to bid responses via blackboard
	bidTopic := fmt.Sprintf("bids:%s", taskID.String())
	bidsMsgChan, err := cm.blackboard.Subscribe(ctx, bidTopic)
	if err != nil {
		log.Error().Err(err).Msg("Failed to subscribe to bids")
		return bids
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Collect bids until timeout
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-bidsMsgChan:
				if msg == nil {
					return
				}
				var bid Bid
				if err := json.Unmarshal(msg.Content, &bid); err != nil {
					log.Warn().Err(err).Msg("Failed to unmarshal bid")
					continue
				}
				bidChan <- &bid
			}
		}
	}()

	for {
		select {
		case <-timer.C:
			close(bidChan)
			// Drain remaining bids
			for bid := range bidChan {
				bids = append(bids, bid)
			}
			return bids
		case bid := <-bidChan:
			if bid != nil {
				bids = append(bids, bid)
				log.Debug().
					Str("task_id", taskID.String()).
					Str("agent", bid.AgentName).
					Float64("cost", bid.Cost).
					Float64("quality", bid.Quality).
					Msg("Received bid")
			}
		}
	}
}

// selectBestBid selects the best bid based on cost, quality, and deadline
func (cm *ConsensusManager) selectBestBid(bids []*Bid, task *ContractNetTask) *Bid {
	if len(bids) == 0 {
		return nil
	}

	if len(bids) == 1 {
		return bids[0]
	}

	// Score each bid (higher is better)
	type scoredBid struct {
		bid   *Bid
		score float64
	}

	scored := make([]scoredBid, 0, len(bids))

	for _, bid := range bids {
		// Normalize cost (inverse, since lower is better)
		maxCost := 0.0
		for _, b := range bids {
			if b.Cost > maxCost {
				maxCost = b.Cost
			}
		}
		costScore := 1.0
		if maxCost > 0 {
			costScore = 1.0 - (bid.Cost / maxCost)
		}

		// Quality score (higher is better)
		qualityScore := bid.Quality

		// Deadline score (earlier is better)
		deadlineScore := 0.0
		if bid.Deadline.Before(task.Deadline) {
			// Give bonus for completing before deadline
			timeBuffer := task.Deadline.Sub(bid.Deadline)
			deadlineScore = math.Min(1.0, timeBuffer.Seconds()/3600.0) // Max 1 hour buffer = 1.0 score
		}

		// Weighted combination (cost: 30%, quality: 50%, deadline: 20%)
		totalScore := (costScore * 0.3) + (qualityScore * 0.5) + (deadlineScore * 0.2)

		scored = append(scored, scoredBid{bid: bid, score: totalScore})

		log.Debug().
			Str("agent", bid.AgentName).
			Float64("cost_score", costScore).
			Float64("quality_score", qualityScore).
			Float64("deadline_score", deadlineScore).
			Float64("total_score", totalScore).
			Msg("Bid evaluation")
	}

	// Sort by score (descending)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	bestBid := scored[0].bid

	log.Info().
		Str("selected_agent", bestBid.AgentName).
		Float64("score", scored[0].score).
		Float64("cost", bestBid.Cost).
		Float64("quality", bestBid.Quality).
		Msg("Selected best bid")

	return bestBid
}

// SubmitBid allows an agent to submit a bid for a task
func (cm *ConsensusManager) SubmitBid(ctx context.Context, bid *Bid) error {
	// Validate bid
	if bid.AgentName == "" {
		return fmt.Errorf("agent name required")
	}
	if bid.Quality < 0 || bid.Quality > 1 {
		return fmt.Errorf("quality must be between 0 and 1")
	}

	bid.Timestamp = time.Now()

	// Post bid to blackboard
	bidTopic := fmt.Sprintf("bids:%s", bid.TaskID.String())
	msg, err := NewMessage(bidTopic, bid.AgentName, bid)
	if err != nil {
		return fmt.Errorf("failed to create bid message: %w", err)
	}

	if err := cm.blackboard.Post(ctx, msg); err != nil {
		return fmt.Errorf("failed to post bid: %w", err)
	}

	log.Debug().
		Str("task_id", bid.TaskID.String()).
		Str("agent", bid.AgentName).
		Float64("cost", bid.Cost).
		Float64("quality", bid.Quality).
		Msg("Submitted bid")

	return nil
}

// MarshalJSON custom marshaling for numeric response values
func (r *Response) MarshalJSON() ([]byte, error) {
	type Alias Response
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// countActiveSessions counts non-expired, non-failed sessions (must be called with lock held)
func (cm *ConsensusManager) countActiveSessions() int {
	count := 0
	now := time.Now()
	for _, session := range cm.sessions {
		if session.Status != ConsensusStatusFailed &&
			session.Status != ConsensusStatusExpired &&
			session.ExpiresAt.After(now) {
			count++
		}
	}
	return count
}
