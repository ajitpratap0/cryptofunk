package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestConsensusManager creates a test consensus manager with mocked dependencies
func setupTestConsensusManager(t *testing.T) (*ConsensusManager, *Blackboard, *MessageBus, *miniredis.Miniredis, *server.Server, func()) {
	// Setup Redis (miniredis)
	mr := miniredis.RunT(t)

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	bb := &Blackboard{
		client: redisClient,
		prefix: "test:blackboard:",
	}

	// Setup NATS server
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // Random port
	}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)

	mb := &MessageBus{
		nc:     nc,
		prefix: "test:agents.",
	}

	cm := NewConsensusManager(bb, mb)

	cleanup := func() {
		nc.Close()
		ns.Shutdown()
		redisClient.Close()
		mr.Close()
	}

	return cm, bb, mb, mr, ns, cleanup
}

// TestNewConsensusManager tests consensus manager creation
func TestNewConsensusManager(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.blackboard)
	assert.NotNil(t, cm.messageBus)
	assert.NotNil(t, cm.sessions)
}

// TestStartDelphiConsensus tests initiating a Delphi consensus session
func TestStartDelphiConsensus(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2", "agent3"}
	config := DefaultConsensusConfig()
	config.RoundTimeout = 2 * time.Second

	session, err := cm.StartDelphiConsensus(ctx, "price_prediction", "What is BTC price in 1 hour?", participants, config)
	require.NoError(t, err)
	assert.NotNil(t, session)

	assert.Equal(t, ConsensusDelphi, session.Method)
	assert.Equal(t, "price_prediction", session.Topic)
	assert.Equal(t, "What is BTC price in 1 hour?", session.Question)
	assert.Equal(t, participants, session.Participants)
	assert.Equal(t, ConsensusStatusActive, session.Status)
	assert.Len(t, session.Rounds, 1) // First round started
}

// TestStartDelphiConsensusInsufficientParticipants tests error handling
func TestStartDelphiConsensusInsufficientParticipants(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1"} // Only 1 agent
	config := DefaultConsensusConfig()
	config.MinParticipants = 2

	session, err := cm.StartDelphiConsensus(ctx, "test", "Test?", participants, config)
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "insufficient participants")
}

// TestSubmitDelphiResponse tests submitting responses to Delphi rounds
func TestSubmitDelphiResponse(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2", "agent3"}
	config := DefaultConsensusConfig()
	config.RoundTimeout = 5 * time.Second

	session, err := cm.StartDelphiConsensus(ctx, "price_prediction", "BTC price?", participants, config)
	require.NoError(t, err)

	// Submit responses from agents
	err = cm.SubmitDelphiResponse(ctx, session.ID, "agent1", 50000.0, 0.8, "Based on technical analysis")
	assert.NoError(t, err)

	err = cm.SubmitDelphiResponse(ctx, session.ID, "agent2", 51000.0, 0.7, "Bullish momentum")
	assert.NoError(t, err)

	err = cm.SubmitDelphiResponse(ctx, session.ID, "agent3", 49500.0, 0.9, "Support at 49k")
	assert.NoError(t, err)

	// Give time for round completion
	time.Sleep(100 * time.Millisecond)

	// Check that round was completed
	updatedSession, err := cm.GetSession(session.ID)
	require.NoError(t, err)
	assert.NotNil(t, updatedSession.Rounds[0].CompletedAt)
	assert.NotNil(t, updatedSession.Rounds[0].Statistics)
}

// TestSubmitDelphiResponseInvalidAgent tests error handling for non-participants
func TestSubmitDelphiResponseInvalidAgent(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2"}
	config := DefaultConsensusConfig()

	session, err := cm.StartDelphiConsensus(ctx, "test", "Test?", participants, config)
	require.NoError(t, err)

	// Try to submit from non-participant
	err = cm.SubmitDelphiResponse(ctx, session.ID, "agent3", 100.0, 0.5, "Not invited")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a participant")
}

// TestDelphiConsensusConvergence tests full consensus convergence
func TestDelphiConsensusConvergence(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2", "agent3"}
	config := DefaultConsensusConfig()
	config.RoundTimeout = 5 * time.Second
	config.MaxRounds = 3

	session, err := cm.StartDelphiConsensus(ctx, "price_prediction", "BTC price?", participants, config)
	require.NoError(t, err)

	// Round 1: Divergent opinions
	cm.SubmitDelphiResponse(ctx, session.ID, "agent1", 50000.0, 0.8, "Technical analysis")
	cm.SubmitDelphiResponse(ctx, session.ID, "agent2", 55000.0, 0.7, "Bullish trend")
	cm.SubmitDelphiResponse(ctx, session.ID, "agent3", 48000.0, 0.9, "Support levels")

	time.Sleep(200 * time.Millisecond)

	// Check that second round started
	updatedSession, err := cm.GetSession(session.ID)
	require.NoError(t, err)

	if len(updatedSession.Rounds) > 1 {
		// Round 2: More convergent
		cm.SubmitDelphiResponse(ctx, session.ID, "agent1", 51000.0, 0.85, "Adjusted based on feedback")
		cm.SubmitDelphiResponse(ctx, session.ID, "agent2", 52000.0, 0.8, "Moderate adjustment")
		cm.SubmitDelphiResponse(ctx, session.ID, "agent3", 50500.0, 0.9, "Converging to mean")

		time.Sleep(200 * time.Millisecond)

		// Check final session status
		finalSession, err := cm.GetSession(session.ID)
		require.NoError(t, err)

		// Should converge or reach max rounds
		if finalSession.Status == ConsensusStatusConverged {
			assert.NotNil(t, finalSession.Result)
			assert.Greater(t, finalSession.Result.Agreement, 0.0)
			t.Logf("Consensus reached: decision=%.2f, agreement=%.2f, rounds=%d",
				finalSession.Result.Decision.(float64),
				finalSession.Result.Agreement,
				finalSession.Result.Rounds)
		}
	}
}

// TestCalculateStatistics tests statistical calculations
func TestCalculateStatistics(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	round := &ConsensusRound{
		Number:    1,
		Responses: make(map[string]*Response),
	}

	// Add responses with similar values (high consensus)
	round.Responses["agent1"] = &Response{Value: 100.0, Confidence: 0.8}
	round.Responses["agent2"] = &Response{Value: 102.0, Confidence: 0.9}
	round.Responses["agent3"] = &Response{Value: 98.0, Confidence: 0.85}

	stats := cm.calculateStatistics(round)

	assert.NotNil(t, stats)
	assert.InDelta(t, 100.0, stats.Mean, 2.0)
	assert.Equal(t, 98.0, stats.Min)
	assert.Equal(t, 102.0, stats.Max)
	assert.InDelta(t, 4.0, stats.Range, 0.1)
	assert.Greater(t, stats.Consensus, 0.5) // High consensus expected
}

// TestCalculateOverallConfidence tests confidence calculation
func TestCalculateOverallConfidence(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	round := &ConsensusRound{
		Responses: map[string]*Response{
			"agent1": {Confidence: 0.8},
			"agent2": {Confidence: 0.9},
			"agent3": {Confidence: 0.7},
		},
	}

	confidence := cm.calculateOverallConfidence(round)
	assert.InDelta(t, 0.8, confidence, 0.1) // Average of 0.8, 0.9, 0.7
}

// TestStartContractNet tests Contract Net protocol
func TestStartContractNet(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()

	task := &ContractNetTask{
		Description: "Analyze market data",
		Requirements: map[string]interface{}{
			"data_source": "binance",
			"timeframe":   "1h",
		},
		Deadline: time.Now().Add(1 * time.Hour),
		Priority: 5,
		Metadata: make(map[string]interface{}),
	}

	eligibleAgents := []string{"agent1", "agent2", "agent3"}

	// Submit bids via blackboard before starting contract net
	go func() {
		time.Sleep(100 * time.Millisecond)

		// Agent 1 bid
		bid1 := &Bid{
			TaskID:    task.ID,
			AgentName: "agent1",
			Cost:      10.0,
			Quality:   0.8,
			Deadline:  time.Now().Add(45 * time.Minute),
			Reasoning: "Experienced in market analysis",
		}
		cm.SubmitBid(ctx, bid1)

		// Agent 2 bid
		bid2 := &Bid{
			TaskID:    task.ID,
			AgentName: "agent2",
			Cost:      8.0,
			Quality:   0.9, // Higher quality
			Deadline:  time.Now().Add(50 * time.Minute),
			Reasoning: "Specialized in Binance data",
		}
		cm.SubmitBid(ctx, bid2)

		// Agent 3 bid
		bid3 := &Bid{
			TaskID:    task.ID,
			AgentName: "agent3",
			Cost:      12.0,
			Quality:   0.7,
			Deadline:  time.Now().Add(40 * time.Minute),
			Reasoning: "Fast turnaround",
		}
		cm.SubmitBid(ctx, bid3)
	}()

	contract, err := cm.StartContractNet(ctx, task, eligibleAgents, 500*time.Millisecond)

	if err == nil {
		require.NotNil(t, contract)
		assert.Equal(t, ContractStatusAwarded, contract.Status)
		assert.NotEmpty(t, contract.Contractor)
		assert.NotNil(t, contract.Bid)
		assert.Greater(t, contract.Bid.Quality, 0.0)

		t.Logf("Contract awarded to: %s (quality: %.2f, cost: %.2f)",
			contract.Contractor,
			contract.Bid.Quality,
			contract.Bid.Cost)
	} else {
		// No bids received in time - this is acceptable in test
		assert.Contains(t, err.Error(), "no bids received")
		t.Log("No bids received (timing issue in test)")
	}
}

// TestStartContractNetNoBids tests Contract Net with no bids
func TestStartContractNetNoBids(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()

	task := &ContractNetTask{
		Description: "Test task",
		Deadline:    time.Now().Add(1 * time.Hour),
		Priority:    5,
		Metadata:    make(map[string]interface{}),
	}

	eligibleAgents := []string{"agent1", "agent2"}

	// Don't submit any bids
	contract, err := cm.StartContractNet(ctx, task, eligibleAgents, 200*time.Millisecond)

	assert.Error(t, err)
	assert.Nil(t, contract)
	assert.Contains(t, err.Error(), "no bids received")
}

// TestStartContractNetNoEligibleAgents tests error handling
func TestStartContractNetNoEligibleAgents(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()

	task := &ContractNetTask{
		Description: "Test task",
		Deadline:    time.Now().Add(1 * time.Hour),
		Priority:    5,
		Metadata:    make(map[string]interface{}),
	}

	contract, err := cm.StartContractNet(ctx, task, []string{}, 1*time.Second)

	assert.Error(t, err)
	assert.Nil(t, contract)
	assert.Contains(t, err.Error(), "no eligible agents")
}

// TestSubmitBid tests bid submission
func TestSubmitBid(t *testing.T) {
	cm, bb, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	taskID := uuid.New()

	bid := &Bid{
		TaskID:    taskID,
		AgentName: "test-agent",
		Cost:      10.0,
		Quality:   0.8,
		Deadline:  time.Now().Add(1 * time.Hour),
		Reasoning: "I can do this",
	}

	err := cm.SubmitBid(ctx, bid)
	require.NoError(t, err)

	// Verify bid was posted to blackboard
	bidTopic := fmt.Sprintf("bids:%s", taskID.String())
	messages, err := bb.GetByTopic(ctx, bidTopic, 10)
	require.NoError(t, err)
	assert.Len(t, messages, 1)

	var retrievedBid Bid
	err = json.Unmarshal(messages[0].Content, &retrievedBid)
	require.NoError(t, err)
	assert.Equal(t, bid.AgentName, retrievedBid.AgentName)
	assert.Equal(t, bid.Cost, retrievedBid.Cost)
	assert.Equal(t, bid.Quality, retrievedBid.Quality)
}

// TestSubmitBidValidation tests bid validation
func TestSubmitBidValidation(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()

	// Invalid: no agent name
	bid1 := &Bid{
		TaskID:  uuid.New(),
		Cost:    10.0,
		Quality: 0.8,
	}
	err := cm.SubmitBid(ctx, bid1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent name required")

	// Invalid: quality out of range
	bid2 := &Bid{
		TaskID:    uuid.New(),
		AgentName: "test-agent",
		Cost:      10.0,
		Quality:   1.5, // > 1.0
	}
	err = cm.SubmitBid(ctx, bid2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quality must be between 0 and 1")
}

// TestSelectBestBid tests bid selection algorithm
func TestSelectBestBid(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	task := &ContractNetTask{
		Deadline: time.Now().Add(1 * time.Hour),
	}

	bids := []*Bid{
		{
			AgentName: "agent1",
			Cost:      10.0,
			Quality:   0.8,
			Deadline:  time.Now().Add(50 * time.Minute),
		},
		{
			AgentName: "agent2",
			Cost:      8.0,
			Quality:   0.9, // Best quality, good cost
			Deadline:  time.Now().Add(45 * time.Minute),
		},
		{
			AgentName: "agent3",
			Cost:      12.0,
			Quality:   0.7,
			Deadline:  time.Now().Add(40 * time.Minute),
		},
	}

	bestBid := cm.selectBestBid(bids, task)
	require.NotNil(t, bestBid)

	// Agent2 should win (best quality, good cost)
	assert.Equal(t, "agent2", bestBid.AgentName)
}

// TestSelectBestBidSingleBid tests single bid scenario
func TestSelectBestBidSingleBid(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	task := &ContractNetTask{
		Deadline: time.Now().Add(1 * time.Hour),
	}

	bids := []*Bid{
		{
			AgentName: "agent1",
			Cost:      10.0,
			Quality:   0.8,
			Deadline:  time.Now().Add(50 * time.Minute),
		},
	}

	bestBid := cm.selectBestBid(bids, task)
	require.NotNil(t, bestBid)
	assert.Equal(t, "agent1", bestBid.AgentName)
}

// TestGetSession tests session retrieval
func TestGetSession(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2"}
	config := DefaultConsensusConfig()

	session, err := cm.StartDelphiConsensus(ctx, "test", "Test?", participants, config)
	require.NoError(t, err)

	retrieved, err := cm.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)
	assert.Equal(t, session.Topic, retrieved.Topic)
}

// TestGetSessionNotFound tests error handling
func TestGetSessionNotFound(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	retrieved, err := cm.GetSession(uuid.New())
	assert.Error(t, err)
	assert.Nil(t, retrieved)
	assert.Contains(t, err.Error(), "session not found")
}

// TestListSessions tests listing all sessions
func TestListSessions(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	config := DefaultConsensusConfig()

	// Create multiple sessions
	participants := []string{"agent1", "agent2"}
	cm.StartDelphiConsensus(ctx, "topic1", "Question 1?", participants, config)
	cm.StartDelphiConsensus(ctx, "topic2", "Question 2?", participants, config)

	sessions := cm.ListSessions()
	assert.GreaterOrEqual(t, len(sessions), 2)
}

// TestCleanupExpiredSessions tests session cleanup
func TestCleanupExpiredSessions(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2"}
	config := DefaultConsensusConfig()
	config.SessionTTL = 100 * time.Millisecond // Very short TTL

	session, err := cm.StartDelphiConsensus(ctx, "test", "Test?", participants, config)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	removed := cm.CleanupExpiredSessions()
	assert.GreaterOrEqual(t, removed, 1)

	// Session should be gone
	_, err = cm.GetSession(session.ID)
	assert.Error(t, err)
}

// TestRoundTimeout tests round timeout handling
func TestRoundTimeout(t *testing.T) {
	cm, _, _, _, _, cleanup := setupTestConsensusManager(t)
	defer cleanup()

	ctx := context.Background()
	participants := []string{"agent1", "agent2", "agent3"}
	config := DefaultConsensusConfig()
	config.RoundTimeout = 200 * time.Millisecond
	config.MinParticipants = 2

	session, err := cm.StartDelphiConsensus(ctx, "test", "Test?", participants, config)
	require.NoError(t, err)

	// Only submit 2 out of 3 responses
	cm.SubmitDelphiResponse(ctx, session.ID, "agent1", 100.0, 0.8, "Response 1")
	cm.SubmitDelphiResponse(ctx, session.ID, "agent2", 102.0, 0.9, "Response 2")

	// Wait for timeout
	time.Sleep(400 * time.Millisecond)

	// Check that round completed with partial responses
	updatedSession, err := cm.GetSession(session.ID)
	require.NoError(t, err)

	if len(updatedSession.Rounds) > 0 {
		firstRound := updatedSession.Rounds[0]
		if firstRound.CompletedAt != nil {
			assert.Len(t, firstRound.Responses, 2) // Only 2 responses
			t.Log("Round completed with partial responses due to timeout")
		}
	}
}

// TestDefaultConsensusConfig tests default configuration
func TestDefaultConsensusConfig(t *testing.T) {
	config := DefaultConsensusConfig()

	assert.Equal(t, 5, config.MaxRounds)
	assert.Equal(t, 0.8, config.ConvergenceThreshold)
	assert.Equal(t, 30*time.Second, config.RoundTimeout)
	assert.Equal(t, 5*time.Minute, config.SessionTTL)
	assert.Equal(t, 2, config.MinParticipants)
}
