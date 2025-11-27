package db_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
)

// Test constants for LLM decision outcomes
const (
	testOutcomeSuccess = "SUCCESS"
	testOutcomeFailure = "FAILURE"
)

// TestLLMDecisionBasicCRUDWithTestcontainers tests core CRUD operations for LLM decisions
func TestLLMDecisionBasicCRUDWithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("InsertLLMDecisionWithAllFields", func(t *testing.T) {
		// Create a session for foreign key
		session := &db.TradingSession{
			Mode:           db.TradingModePaper,
			Symbol:         "BTC/USDT",
			Exchange:       "binance",
			StartedAt:      time.Now(),
			InitialCapital: 10000.0,
		}
		err := tc.DB.CreateSession(ctx, session)
		require.NoError(t, err)

		// Create LLM decision with all fields populated
		outcome := testOutcomeSuccess
		pnl := 150.50
		contextData := map[string]interface{}{
			"market_conditions": map[string]interface{}{
				"volatility": "high",
				"trend":      "bullish",
			},
			"indicators": map[string]interface{}{
				"rsi":  30.5,
				"macd": "bullish",
			},
		}
		contextJSON, err := json.Marshal(contextData)
		require.NoError(t, err)

		decision := &db.LLMDecision{
			ID:           uuid.New(),
			SessionID:    &session.ID,
			DecisionType: "trading_signal",
			Symbol:       "BTC/USDT",
			Prompt:       "Analyze BTC/USDT for entry signal. Current RSI: 30.5, MACD: bullish",
			Response:     "Strong buy signal detected. RSI oversold, MACD bullish crossover, recommend entry at current price.",
			Model:        "claude-3-sonnet",
			TokensUsed:   1500,
			LatencyMs:    250,
			Outcome:      &outcome,
			PnL:          &pnl,
			Context:      contextJSON,
			AgentName:    "technical-agent",
			Confidence:   0.85,
			CreatedAt:    time.Now(),
		}

		err = tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Verify the decision was inserted by querying
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "technical-agent", 10)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(decisions), 1)

		// Find our decision
		var found *db.LLMDecision
		for _, d := range decisions {
			if d.ID == decision.ID {
				found = d
				break
			}
		}
		require.NotNil(t, found, "Should find inserted decision")

		// Verify all fields
		assert.Equal(t, decision.ID, found.ID)
		assert.Equal(t, session.ID, *found.SessionID)
		assert.Equal(t, "trading_signal", found.DecisionType)
		assert.Equal(t, "BTC/USDT", found.Symbol)
		assert.Equal(t, "technical-agent", found.AgentName)
		assert.Equal(t, "claude-3-sonnet", found.Model)
		assert.Equal(t, 1500, found.TokensUsed)
		assert.Equal(t, 250, found.LatencyMs)
		assert.Equal(t, 0.85, found.Confidence)
		assert.NotNil(t, found.Outcome)
		assert.Equal(t, "SUCCESS", *found.Outcome)
		assert.NotNil(t, found.PnL)
		assert.Equal(t, 150.50, *found.PnL)

		// Verify context was stored correctly
		var retrievedContext map[string]interface{}
		err = json.Unmarshal(found.Context, &retrievedContext)
		require.NoError(t, err)
		assert.NotNil(t, retrievedContext["market_conditions"])
		assert.NotNil(t, retrievedContext["indicators"])
	})

	t.Run("InsertLLMDecisionMinimalFields", func(t *testing.T) {
		// Create decision with minimal required fields
		decision := &db.LLMDecision{
			ID:           uuid.New(),
			SessionID:    nil, // Optional
			DecisionType: "risk_approval",
			Symbol:       "ETH/USDT",
			Prompt:       "Approve trade for ETH/USDT",
			Response:     "Trade approved",
			Model:        "gpt-4",
			TokensUsed:   800,
			LatencyMs:    150,
			AgentName:    "risk-agent",
			Confidence:   0.92,
			CreatedAt:    time.Now(),
		}

		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Verify insertion
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "risk-agent", 10)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(decisions), 1)
	})

	t.Run("UpdateLLMDecisionOutcome", func(t *testing.T) {
		// Insert a decision with pending outcome
		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "position_sizing",
			Symbol:       "SOL/USDT",
			Prompt:       "Calculate position size for SOL/USDT",
			Response:     "Position size: 10 SOL",
			Model:        "claude-3-sonnet",
			TokensUsed:   1200,
			LatencyMs:    200,
			AgentName:    "risk-agent",
			Confidence:   0.88,
			CreatedAt:    time.Now(),
		}

		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Update outcome to SUCCESS with P&L
		err = tc.DB.UpdateLLMDecisionOutcome(ctx, decision.ID, testOutcomeSuccess, 75.25)
		require.NoError(t, err)

		// Verify update
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "risk-agent", 10)
		require.NoError(t, err)

		var updated *db.LLMDecision
		for _, d := range decisions {
			if d.ID == decision.ID {
				updated = d
				break
			}
		}
		require.NotNil(t, updated)
		assert.NotNil(t, updated.Outcome)
		assert.Equal(t, "SUCCESS", *updated.Outcome)
		assert.NotNil(t, updated.PnL)
		assert.Equal(t, 75.25, *updated.PnL)
	})

	t.Run("UpdateLLMDecisionOutcomeToFailure", func(t *testing.T) {
		// Insert a decision
		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "trading_signal",
			Symbol:       "ADA/USDT",
			Prompt:       "Analyze ADA/USDT",
			Response:     "Buy signal",
			Model:        "gpt-4",
			TokensUsed:   900,
			LatencyMs:    180,
			AgentName:    "trend-agent",
			Confidence:   0.75,
			CreatedAt:    time.Now(),
		}

		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Update outcome to FAILURE with negative P&L
		err = tc.DB.UpdateLLMDecisionOutcome(ctx, decision.ID, testOutcomeFailure, -50.00)
		require.NoError(t, err)

		// Verify update
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "trend-agent", 10)
		require.NoError(t, err)

		var updated *db.LLMDecision
		for _, d := range decisions {
			if d.ID == decision.ID {
				updated = d
				break
			}
		}
		require.NotNil(t, updated)
		assert.NotNil(t, updated.Outcome)
		assert.Equal(t, "FAILURE", *updated.Outcome)
		assert.NotNil(t, updated.PnL)
		assert.Equal(t, -50.00, *updated.PnL)
	})
}

// TestLLMDecisionQueryMethodsWithTestcontainers tests query and filter operations
func TestLLMDecisionQueryMethodsWithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	// Set up test data - create multiple decisions for different agents and symbols
	testData := []struct {
		agentName  string
		symbol     string
		outcome    string
		pnl        float64
		model      string
		tokens     int
		latency    int
		confidence float64
	}{
		{"technical-agent", "BTC/USDT", "SUCCESS", 100.0, "claude-3-sonnet", 1500, 250, 0.85},
		{"technical-agent", "BTC/USDT", "SUCCESS", 200.0, "claude-3-sonnet", 1400, 240, 0.90},
		{"technical-agent", "ETH/USDT", "FAILURE", -50.0, "claude-3-sonnet", 1600, 260, 0.70},
		{"technical-agent", "ETH/USDT", "SUCCESS", 150.0, "claude-3-sonnet", 1550, 255, 0.88},
		{"risk-agent", "BTC/USDT", "SUCCESS", 75.0, "gpt-4", 1000, 180, 0.92},
		{"risk-agent", "SOL/USDT", "FAILURE", -25.0, "gpt-4", 1100, 190, 0.80},
		{"trend-agent", "BTC/USDT", "SUCCESS", 120.0, "claude-3-sonnet", 1300, 220, 0.87},
		{"trend-agent", "ADA/USDT", "PENDING", 0.0, "gpt-4", 1200, 200, 0.75},
	}

	for _, td := range testData {
		outcome := td.outcome
		pnl := td.pnl

		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "trading_signal",
			Symbol:       td.symbol,
			Prompt:       "Analyze " + td.symbol,
			Response:     "Signal generated",
			Model:        td.model,
			TokensUsed:   td.tokens,
			LatencyMs:    td.latency,
			AgentName:    td.agentName,
			Confidence:   td.confidence,
			CreatedAt:    time.Now(),
		}

		if outcome != "PENDING" {
			decision.Outcome = &outcome
			decision.PnL = &pnl
		}

		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	t.Run("GetLLMDecisionsByAgent", func(t *testing.T) {
		// Query technical-agent decisions
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "technical-agent", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(decisions), 4, "Should have at least 4 technical-agent decisions")

		// Verify all returned decisions are for technical-agent
		for _, d := range decisions {
			assert.Equal(t, "technical-agent", d.AgentName)
		}

		// Verify they're ordered by created_at DESC (most recent first)
		for i := 1; i < len(decisions); i++ {
			assert.True(t, decisions[i-1].CreatedAt.After(decisions[i].CreatedAt) ||
				decisions[i-1].CreatedAt.Equal(decisions[i].CreatedAt),
				"Decisions should be ordered by created_at DESC")
		}
	})

	t.Run("GetLLMDecisionsByAgentWithLimit", func(t *testing.T) {
		// Query with limit of 2
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "technical-agent", 2)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(decisions), 2, "Should respect limit")
	})

	t.Run("GetLLMDecisionsBySymbol", func(t *testing.T) {
		// Query BTC/USDT decisions
		decisions, err := tc.DB.GetLLMDecisionsBySymbol(ctx, "BTC/USDT", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(decisions), 4, "Should have at least 4 BTC/USDT decisions")

		// Verify all returned decisions are for BTC/USDT
		for _, d := range decisions {
			assert.Equal(t, "BTC/USDT", d.Symbol)
		}

		// Verify ordering
		for i := 1; i < len(decisions); i++ {
			assert.True(t, decisions[i-1].CreatedAt.After(decisions[i].CreatedAt) ||
				decisions[i-1].CreatedAt.Equal(decisions[i].CreatedAt),
				"Decisions should be ordered by created_at DESC")
		}
	})

	t.Run("GetLLMDecisionsBySymbolETH", func(t *testing.T) {
		// Query ETH/USDT decisions
		decisions, err := tc.DB.GetLLMDecisionsBySymbol(ctx, "ETH/USDT", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(decisions), 2, "Should have at least 2 ETH/USDT decisions")

		for _, d := range decisions {
			assert.Equal(t, "ETH/USDT", d.Symbol)
		}
	})

	t.Run("GetSuccessfulLLMDecisions", func(t *testing.T) {
		// Query successful decisions for technical-agent
		decisions, err := tc.DB.GetSuccessfulLLMDecisions(ctx, "technical-agent", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(decisions), 3, "Should have at least 3 successful decisions")

		// Verify all are successful with positive P&L
		for _, d := range decisions {
			assert.Equal(t, "technical-agent", d.AgentName)
			assert.NotNil(t, d.Outcome)
			assert.Equal(t, "SUCCESS", *d.Outcome)
			assert.NotNil(t, d.PnL)
			assert.Greater(t, *d.PnL, 0.0)
		}

		// Verify they're ordered by PnL DESC (most profitable first)
		for i := 1; i < len(decisions); i++ {
			assert.True(t, *decisions[i-1].PnL >= *decisions[i].PnL,
				"Successful decisions should be ordered by PnL DESC")
		}
	})

	t.Run("GetSuccessfulLLMDecisionsForRiskAgent", func(t *testing.T) {
		// Query successful decisions for risk-agent
		decisions, err := tc.DB.GetSuccessfulLLMDecisions(ctx, "risk-agent", 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(decisions), 1, "Should have at least 1 successful decision")

		for _, d := range decisions {
			assert.Equal(t, "risk-agent", d.AgentName)
			assert.NotNil(t, d.Outcome)
			assert.Equal(t, "SUCCESS", *d.Outcome)
		}
	})

	t.Run("GetLLMDecisionStats", func(t *testing.T) {
		// Get stats for technical-agent (all time)
		since := time.Now().Add(-24 * time.Hour)
		stats, err := tc.DB.GetLLMDecisionStats(ctx, "technical-agent", since)
		require.NoError(t, err)
		require.NotNil(t, stats)

		// Verify stats structure
		assert.Contains(t, stats, "total_decisions")
		assert.Contains(t, stats, "successful")
		assert.Contains(t, stats, "failed")
		assert.Contains(t, stats, "pending")
		assert.Contains(t, stats, "success_rate")

		// Verify counts
		totalDecisions := stats["total_decisions"].(int)
		successful := stats["successful"].(int)
		failed := stats["failed"].(int)

		assert.GreaterOrEqual(t, totalDecisions, 4, "Should have at least 4 total decisions")
		assert.GreaterOrEqual(t, successful, 3, "Should have at least 3 successful decisions")
		assert.GreaterOrEqual(t, failed, 1, "Should have at least 1 failed decision")

		// Verify success rate calculation
		successRate := stats["success_rate"].(float64)
		expectedRate := float64(successful) / float64(totalDecisions) * 100.0
		assert.InDelta(t, expectedRate, successRate, 0.01, "Success rate should be calculated correctly")

		// Verify P&L stats
		if avgPnL, ok := stats["avg_pnl"]; ok {
			assert.IsType(t, float64(0), avgPnL)
			// Average should be positive for technical-agent (more wins than losses)
			assert.Greater(t, avgPnL.(float64), 0.0)
		}

		if totalPnL, ok := stats["total_pnl"]; ok {
			assert.IsType(t, float64(0), totalPnL)
			// Total should be positive (100 + 200 + 150 - 50 = 400)
			assert.Greater(t, totalPnL.(float64), 0.0)
		}

		// Verify latency and token stats
		if avgLatency, ok := stats["avg_latency_ms"]; ok {
			assert.IsType(t, float64(0), avgLatency)
			assert.Greater(t, avgLatency.(float64), 0.0)
		}

		if avgTokens, ok := stats["avg_tokens_used"]; ok {
			assert.IsType(t, float64(0), avgTokens)
			assert.Greater(t, avgTokens.(float64), 0.0)
		}

		if avgConfidence, ok := stats["avg_confidence"]; ok {
			assert.IsType(t, float64(0), avgConfidence)
			assert.Greater(t, avgConfidence.(float64), 0.0)
			assert.LessOrEqual(t, avgConfidence.(float64), 1.0)
		}
	})

	t.Run("GetLLMDecisionStatsForRiskAgent", func(t *testing.T) {
		// Get stats for risk-agent
		since := time.Now().Add(-24 * time.Hour)
		stats, err := tc.DB.GetLLMDecisionStats(ctx, "risk-agent", since)
		require.NoError(t, err)
		require.NotNil(t, stats)

		totalDecisions := stats["total_decisions"].(int)
		successful := stats["successful"].(int)
		failed := stats["failed"].(int)

		assert.GreaterOrEqual(t, totalDecisions, 2)
		assert.GreaterOrEqual(t, successful, 1)
		assert.GreaterOrEqual(t, failed, 1)

		// Success rate should be 50% (1 success, 1 failure)
		successRate := stats["success_rate"].(float64)
		assert.InDelta(t, 50.0, successRate, 1.0)
	})

	t.Run("GetLLMDecisionStatsWithNarrowTimeWindow", func(t *testing.T) {
		// Query with future time (should return zero stats)
		since := time.Now().Add(1 * time.Hour)
		stats, err := tc.DB.GetLLMDecisionStats(ctx, "technical-agent", since)
		require.NoError(t, err)
		require.NotNil(t, stats)

		totalDecisions := stats["total_decisions"].(int)
		assert.Equal(t, 0, totalDecisions, "Should have no decisions in future time window")
	})
}

// TestLLMDecisionConcurrencyWithTestcontainers tests concurrent operations
func TestLLMDecisionConcurrencyWithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("MultipleAgentsConcurrentInsert", func(t *testing.T) {
		// Insert 50 decisions from different agents concurrently
		var wg sync.WaitGroup
		errors := make(chan error, 50)
		agentNames := []string{"agent-1", "agent-2", "agent-3", "agent-4", "agent-5"}
		symbols := []string{"BTC/USDT", "ETH/USDT", "SOL/USDT"}

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				agentName := agentNames[index%len(agentNames)]
				symbol := symbols[index%len(symbols)]

				decision := &db.LLMDecision{
					ID:           uuid.New(),
					DecisionType: "trading_signal",
					Symbol:       symbol,
					Prompt:       "Analyze market",
					Response:     "Signal generated",
					Model:        "claude-3-sonnet",
					TokensUsed:   1000 + index,
					LatencyMs:    200 + index,
					AgentName:    agentName,
					Confidence:   0.75 + float64(index%20)*0.01,
					CreatedAt:    time.Now(),
				}

				err := tc.DB.InsertLLMDecision(ctx, decision)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent insert error: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "Should have no errors during concurrent inserts")

		// Verify all decisions were inserted
		for _, agentName := range agentNames {
			decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, agentName, 100)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(decisions), 10, "Each agent should have at least 10 decisions")
		}
	})

	t.Run("ConcurrentUpdates", func(t *testing.T) {
		// Create 20 decisions
		decisionIDs := make([]uuid.UUID, 20)
		for i := 0; i < 20; i++ {
			decision := &db.LLMDecision{
				ID:           uuid.New(),
				DecisionType: "position_sizing",
				Symbol:       "BTC/USDT",
				Prompt:       "Calculate position",
				Response:     "Position calculated",
				Model:        "gpt-4",
				TokensUsed:   1000,
				LatencyMs:    200,
				AgentName:    "update-test-agent",
				Confidence:   0.80,
				CreatedAt:    time.Now(),
			}
			err := tc.DB.InsertLLMDecision(ctx, decision)
			require.NoError(t, err)
			decisionIDs[i] = decision.ID
		}

		// Update all 20 decisions concurrently
		var wg sync.WaitGroup
		errors := make(chan error, 20)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				outcome := testOutcomeSuccess
				if index%2 == 0 {
					outcome = testOutcomeFailure
				}
				pnl := float64(index * 10)
				if outcome == testOutcomeFailure {
					pnl = -pnl
				}

				err := tc.DB.UpdateLLMDecisionOutcome(ctx, decisionIDs[index], outcome, pnl)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent update error: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "Should have no errors during concurrent updates")

		// Verify all decisions were updated
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "update-test-agent", 100)
		require.NoError(t, err)

		successCount := 0
		failureCount := 0
		for _, d := range decisions {
			if d.Outcome != nil {
				switch *d.Outcome {
				case testOutcomeSuccess:
					successCount++
				case testOutcomeFailure:
					failureCount++
				}
			}
		}

		assert.Equal(t, 10, successCount, "Should have 10 successful decisions")
		assert.Equal(t, 10, failureCount, "Should have 10 failed decisions")
	})

	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		// Create some initial data
		for i := 0; i < 10; i++ {
			decision := &db.LLMDecision{
				ID:           uuid.New(),
				DecisionType: "risk_approval",
				Symbol:       "ETH/USDT",
				Prompt:       "Approve trade",
				Response:     "Approved",
				Model:        "claude-3-sonnet",
				TokensUsed:   800,
				LatencyMs:    150,
				AgentName:    "rw-test-agent",
				Confidence:   0.85,
				CreatedAt:    time.Now(),
			}
			err := tc.DB.InsertLLMDecision(ctx, decision)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		errors := make(chan error, 200)

		// 50 concurrent reads (GetLLMDecisionsByAgent)
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_, err := tc.DB.GetLLMDecisionsByAgent(ctx, "rw-test-agent", 10)
				if err != nil {
					errors <- err
				}
			}()
		}

		// 50 concurrent reads (GetLLMDecisionsBySymbol)
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_, err := tc.DB.GetLLMDecisionsBySymbol(ctx, "ETH/USDT", 10)
				if err != nil {
					errors <- err
				}
			}()
		}

		// 50 concurrent writes
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				decision := &db.LLMDecision{
					ID:           uuid.New(),
					DecisionType: "trading_signal",
					Symbol:       "ETH/USDT",
					Prompt:       "Concurrent test",
					Response:     "Test response",
					Model:        "gpt-4",
					TokensUsed:   900,
					LatencyMs:    170,
					AgentName:    "rw-test-agent",
					Confidence:   0.82,
					CreatedAt:    time.Now(),
				}

				err := tc.DB.InsertLLMDecision(ctx, decision)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		// 50 concurrent stats queries
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				since := time.Now().Add(-1 * time.Hour)
				_, err := tc.DB.GetLLMDecisionStats(ctx, "rw-test-agent", since)
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent read/write error: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "Should have no errors during concurrent read/write")
	})
}

// TestLLMDecisionEdgeCases tests edge cases and special scenarios
func TestLLMDecisionEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("EmptyAgentQuery", func(t *testing.T) {
		// Query non-existent agent
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "non-existent-agent", 10)
		require.NoError(t, err)
		assert.Empty(t, decisions, "Should return empty array for non-existent agent")
	})

	t.Run("EmptySymbolQuery", func(t *testing.T) {
		// Query non-existent symbol
		decisions, err := tc.DB.GetLLMDecisionsBySymbol(ctx, "NONEXISTENT/USDT", 10)
		require.NoError(t, err)
		assert.Empty(t, decisions, "Should return empty array for non-existent symbol")
	})

	t.Run("NoSuccessfulDecisions", func(t *testing.T) {
		// Create agent with only failures
		outcome := "FAILURE"
		pnl := -10.0
		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "trading_signal",
			Symbol:       "TEST/USDT",
			Prompt:       "Test",
			Response:     "Response",
			Model:        "claude-3-sonnet",
			TokensUsed:   1000,
			LatencyMs:    200,
			Outcome:      &outcome,
			PnL:          &pnl,
			AgentName:    "failure-agent",
			Confidence:   0.70,
			CreatedAt:    time.Now(),
		}
		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Query successful decisions
		decisions, err := tc.DB.GetSuccessfulLLMDecisions(ctx, "failure-agent", 10)
		require.NoError(t, err)
		assert.Empty(t, decisions, "Should return empty for agent with no successes")
	})

	t.Run("StatsForAgentWithNoPnL", func(t *testing.T) {
		// Create decisions without outcomes
		for i := 0; i < 3; i++ {
			decision := &db.LLMDecision{
				ID:           uuid.New(),
				DecisionType: "analysis",
				Symbol:       "BTC/USDT",
				Prompt:       "Analyze",
				Response:     "Analysis complete",
				Model:        "gpt-4",
				TokensUsed:   1000,
				LatencyMs:    200,
				AgentName:    "pending-agent",
				Confidence:   0.80,
				CreatedAt:    time.Now(),
			}
			err := tc.DB.InsertLLMDecision(ctx, decision)
			require.NoError(t, err)
		}

		// Get stats
		since := time.Now().Add(-1 * time.Hour)
		stats, err := tc.DB.GetLLMDecisionStats(ctx, "pending-agent", since)
		require.NoError(t, err)

		// Should have stats but no P&L
		assert.Equal(t, 3, stats["total_decisions"])
		assert.Equal(t, 0, stats["successful"])
		assert.Equal(t, 0, stats["failed"])
		assert.Equal(t, 3, stats["pending"])
		assert.Equal(t, 0.0, stats["success_rate"])

		// avg_pnl and total_pnl should be present but zero or not included
		if avgPnL, ok := stats["avg_pnl"]; ok {
			assert.Equal(t, 0.0, avgPnL)
		}
	})

	t.Run("VeryLongPromptAndResponse", func(t *testing.T) {
		// Test with very long strings
		longPrompt := ""
		for i := 0; i < 1000; i++ {
			longPrompt += "This is a very long prompt with lots of market analysis data. "
		}

		longResponse := ""
		for i := 0; i < 1000; i++ {
			longResponse += "This is a very detailed response with comprehensive trading recommendations. "
		}

		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "comprehensive_analysis",
			Symbol:       "BTC/USDT",
			Prompt:       longPrompt,
			Response:     longResponse,
			Model:        "claude-3-sonnet",
			TokensUsed:   50000,
			LatencyMs:    5000,
			AgentName:    "verbose-agent",
			Confidence:   0.95,
			CreatedAt:    time.Now(),
		}

		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Verify retrieval
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "verbose-agent", 10)
		require.NoError(t, err)
		require.Len(t, decisions, 1)
		assert.Equal(t, longPrompt, decisions[0].Prompt)
		assert.Equal(t, longResponse, decisions[0].Response)
	})

	t.Run("ComplexContextJSON", func(t *testing.T) {
		// Test with deeply nested JSON context
		complexContext := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"indicators": map[string]interface{}{
							"rsi":  []float64{30.5, 31.2, 32.1},
							"macd": map[string]float64{"value": 0.5, "signal": 0.3, "histogram": 0.2},
						},
						"patterns": []string{"bullish_engulfing", "hammer", "morning_star"},
					},
				},
			},
			"metadata": map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"version":   "2.0",
			},
		}

		contextJSON, err := json.Marshal(complexContext)
		require.NoError(t, err)

		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "pattern_analysis",
			Symbol:       "ETH/USDT",
			Prompt:       "Analyze patterns",
			Response:     "Patterns identified",
			Model:        "claude-3-sonnet",
			TokensUsed:   2000,
			LatencyMs:    300,
			Context:      contextJSON,
			AgentName:    "pattern-agent",
			Confidence:   0.88,
			CreatedAt:    time.Now(),
		}

		err = tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Verify retrieval and context integrity
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "pattern-agent", 10)
		require.NoError(t, err)
		require.Len(t, decisions, 1)

		var retrievedContext map[string]interface{}
		err = json.Unmarshal(decisions[0].Context, &retrievedContext)
		require.NoError(t, err)
		assert.NotNil(t, retrievedContext["level1"])
		assert.NotNil(t, retrievedContext["metadata"])
	})

	t.Run("ZeroAndNegativeValues", func(t *testing.T) {
		outcome := "SUCCESS"
		pnl := 0.0 // Break-even trade

		decision := &db.LLMDecision{
			ID:           uuid.New(),
			DecisionType: "trading_signal",
			Symbol:       "BTC/USDT",
			Prompt:       "Analyze",
			Response:     "Break-even",
			Model:        "gpt-4",
			TokensUsed:   0, // Edge case: zero tokens
			LatencyMs:    0, // Edge case: zero latency
			Outcome:      &outcome,
			PnL:          &pnl,
			AgentName:    "zero-agent",
			Confidence:   0.00, // Edge case: zero confidence
			CreatedAt:    time.Now(),
		}

		err := tc.DB.InsertLLMDecision(ctx, decision)
		require.NoError(t, err)

		// Verify retrieval
		decisions, err := tc.DB.GetLLMDecisionsByAgent(ctx, "zero-agent", 10)
		require.NoError(t, err)
		require.Len(t, decisions, 1)
		assert.Equal(t, 0, decisions[0].TokensUsed)
		assert.Equal(t, 0, decisions[0].LatencyMs)
		assert.Equal(t, 0.0, decisions[0].Confidence)
	})
}
