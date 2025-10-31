// Risk Management Agent Unit Tests
package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/risk"
)

// ============================================================================
// TEST HELPERS
// ============================================================================

func createTestRiskAgent() *RiskAgent {
	config := &RiskAgentConfig{
		AgentName:          "test-risk-agent",
		AgentType:          "risk",
		Weight:             1.0,
		MaxPositionSize:    10000.0,
		MaxTotalExposure:   50000.0,
		MaxConcentration:   0.25,
		MaxOpenPositions:   5,
		MaxDrawdownPercent: 20.0,
		MinSharpeRatio:     1.0,
		KellyFraction:      0.25,
		StopLossMultiplier: 2.0,
		RiskFreeRate:       0.03,
	}

	agent := &RiskAgent{
		config:      config,
		riskService: risk.NewService(),
		beliefs: &RiskBeliefs{
			currentPositions:  make([]Position, 0),
			equityCurve:       make([]float64, 0),
			returns:           make([]float64, 0),
			nearLimitSymbols:  make([]string, 0),
			lastUpdate:        time.Now(),
			marketRegime:      "sideways",
			totalExposure:     0,
			openPositionCount: 0,
			currentDrawdown:   0,
			maxDrawdown:       0,
			sharpeRatio:       1.5,
			volatility:        0.02,
			limitsUtilization: 0,
		},
		desires: &RiskDesires{
			protectCapital:          true,
			maintainDiversification: true,
			controlDrawdown:         true,
			optimizeRiskReturn:      true,
			targetSharpe:            config.MinSharpeRatio,
			maxAcceptableDD:         config.MaxDrawdownPercent,
			targetUtilization:       0.80,
		},
		intentions: &RiskIntentions{
			shouldVeto:      false,
			monitorDrawdown: true,
		},
	}

	return agent
}

// ============================================================================
// T119: AGENT CREATION TESTS
// ============================================================================

func TestNewRiskAgent(t *testing.T) {
	config := &RiskAgentConfig{
		AgentName:        "test-agent",
		AgentType:        "risk",
		Weight:           1.0,
		MaxPositionSize:  10000.0,
		MaxTotalExposure: 50000.0,
	}

	agent, err := NewRiskAgent(config, nil, risk.NewService())
	require.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "test-agent", agent.config.AgentName)
	assert.Equal(t, "risk", agent.config.AgentType)
	assert.NotNil(t, agent.beliefs)
	assert.NotNil(t, agent.desires)
	assert.NotNil(t, agent.intentions)
}

func TestRiskAgentInitialization(t *testing.T) {
	agent := createTestRiskAgent()
	assert.NotNil(t, agent)
	assert.Equal(t, 0, agent.beliefs.openPositionCount)
	assert.Equal(t, 0.0, agent.beliefs.totalExposure)
	assert.True(t, agent.desires.protectCapital)
	assert.True(t, agent.desires.controlDrawdown)
}

// ============================================================================
// T120: PORTFOLIO LIMIT CHECKING TESTS
// ============================================================================

func TestCheckPortfolioLimits_WithinLimits(t *testing.T) {
	agent := createTestRiskAgent()

	// Test with small position
	approved, violations := agent.checkPortfolioLimits("BTC/USDT", 5000.0)
	assert.True(t, approved)
	assert.Empty(t, violations)
}

func TestCheckPortfolioLimits_ExceedsPositionSize(t *testing.T) {
	agent := createTestRiskAgent()

	// Test with position exceeding max
	approved, violations := agent.checkPortfolioLimits("BTC/USDT", 15000.0)
	assert.False(t, approved)
	assert.NotEmpty(t, violations)
	assert.Contains(t, violations[0], "Position size")
}

func TestCheckPortfolioLimits_ExceedsTotalExposure(t *testing.T) {
	agent := createTestRiskAgent()

	// Set current exposure high
	agent.beliefs.totalExposure = 45000.0

	// Try to add another 10k (would exceed 50k limit)
	approved, violations := agent.checkPortfolioLimits("ETH/USDT", 10000.0)
	assert.False(t, approved)
	assert.NotEmpty(t, violations)
	assert.Contains(t, violations[0], "Total exposure")
}

func TestCheckPortfolioLimits_ExceedsConcentration(t *testing.T) {
	agent := createTestRiskAgent()

	// Set existing positions
	agent.beliefs.totalExposure = 40000.0
	agent.beliefs.currentPositions = []Position{
		{Symbol: "BTC/USDT", Size: 10000.0},
	}

	// Try to add more BTC (would exceed 25% concentration on 40k exposure)
	// Max concentration: 50000 * 0.25 = 12500
	// Current + new = 10000 + 5000 = 15000 > 12500
	approved, violations := agent.checkPortfolioLimits("BTC/USDT", 5000.0)
	assert.False(t, approved)
	assert.NotEmpty(t, violations)
	assert.Contains(t, violations[0], "concentration")
}

func TestCheckPortfolioLimits_MaxOpenPositions(t *testing.T) {
	agent := createTestRiskAgent()

	// Set open positions to max
	agent.beliefs.openPositionCount = 5

	// Try to add another position
	approved, violations := agent.checkPortfolioLimits("SOL/USDT", 5000.0)
	assert.False(t, approved)
	assert.NotEmpty(t, violations)
	assert.Contains(t, violations[0], "maximum")
	assert.Contains(t, violations[0], "open positions")
}

func TestCheckPortfolioLimits_DrawdownExceeded(t *testing.T) {
	agent := createTestRiskAgent()

	// Set drawdown above limit
	agent.beliefs.currentDrawdown = 25.0 // Exceeds 20% limit

	// Note: Drawdown is now checked separately in evaluateProposal,
	// not in checkPortfolioLimits. This test verifies that portfolio
	// limits check doesn't block the trade for other reasons.
	approved, violations := agent.checkPortfolioLimits("BTC/USDT", 5000.0)

	// Should pass portfolio limits (drawdown checked elsewhere)
	assert.True(t, approved)
	assert.Empty(t, violations)
}

// ============================================================================
// T121: KELLY CRITERION POSITION SIZING TESTS
// ============================================================================

func TestCalculateOptimalSize_BasicCalculation(t *testing.T) {
	agent := createTestRiskAgent()

	// Test with default win rate
	optimalSize := agent.calculateOptimalSize("BTC/USDT", 0.8)

	// Should return a positive size
	assert.Greater(t, optimalSize, 0.0)

	// Should be less than max position size
	assert.LessOrEqual(t, optimalSize, agent.config.MaxPositionSize)

	// Should be reasonable (e.g., not more than 10% of max exposure)
	maxExpectedSize := agent.config.MaxTotalExposure * 0.10
	assert.LessOrEqual(t, optimalSize, maxExpectedSize)
}

func TestCalculateOptimalSize_HighConfidence(t *testing.T) {
	agent := createTestRiskAgent()

	lowConfSize := agent.calculateOptimalSize("BTC/USDT", 0.5)
	highConfSize := agent.calculateOptimalSize("BTC/USDT", 0.9)

	// Higher confidence should result in larger position
	assert.Greater(t, highConfSize, lowConfSize)
}

func TestCalculateOptimalSize_CappedAtMaxPositionSize(t *testing.T) {
	agent := createTestRiskAgent()

	// Even with high confidence, should cap at max position size
	optimalSize := agent.calculateOptimalSize("BTC/USDT", 1.0)
	assert.LessOrEqual(t, optimalSize, agent.config.MaxPositionSize)
}

// ============================================================================
// T122: STOP-LOSS CALCULATION TESTS
// ============================================================================

func TestCalculateStopLoss_BuyPosition(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.volatility = 0.02 // 2%

	entryPrice := 50000.0
	stopLoss := agent.calculateStopLoss(entryPrice, "BUY")

	// Stop loss should be below entry for BUY
	assert.Less(t, stopLoss, entryPrice)

	// Stop loss distance should be volatility * multiplier
	expectedDistance := entryPrice * 0.02 * agent.config.StopLossMultiplier // 2000
	expectedStopLoss := entryPrice - expectedDistance
	assert.InDelta(t, expectedStopLoss, stopLoss, 1.0)
}

func TestCalculateStopLoss_SellPosition(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.volatility = 0.02 // 2%

	entryPrice := 50000.0
	stopLoss := agent.calculateStopLoss(entryPrice, "SELL")

	// Stop loss should be above entry for SELL
	assert.Greater(t, stopLoss, entryPrice)

	// Stop loss distance should be volatility * multiplier
	expectedDistance := entryPrice * 0.02 * agent.config.StopLossMultiplier
	expectedStopLoss := entryPrice + expectedDistance
	assert.InDelta(t, expectedStopLoss, stopLoss, 1.0)
}

func TestCalculateStopLoss_DefaultVolatility(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.volatility = 0 // Unknown volatility

	entryPrice := 50000.0
	stopLoss := agent.calculateStopLoss(entryPrice, "BUY")

	// Should use default 2% volatility
	expectedDistance := entryPrice * 0.02 * agent.config.StopLossMultiplier
	expectedStopLoss := entryPrice - expectedDistance
	assert.InDelta(t, expectedStopLoss, stopLoss, 1.0)
}

// ============================================================================
// T123: VETO LOGIC TESTS
// ============================================================================

func TestEvaluateProposal_ApproveNormalTrade(t *testing.T) {
	agent := createTestRiskAgent()

	intentions := agent.evaluateProposal("BTC/USDT", "BUY", 5000.0, 0.8)

	assert.False(t, intentions.shouldVeto)
	assert.GreaterOrEqual(t, intentions.confidenceScore, 0.7)
	assert.Greater(t, intentions.recommendedSize, 0.0)
}

func TestEvaluateProposal_VetoOnPortfolioLimits(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.openPositionCount = 5 // At max

	intentions := agent.evaluateProposal("SOL/USDT", "BUY", 5000.0, 0.8)

	assert.True(t, intentions.shouldVeto)
	assert.Contains(t, intentions.vetoReason, "Portfolio limits")
	assert.Greater(t, intentions.confidenceScore, 0.9)
}

func TestEvaluateProposal_VetoOnDrawdownLimit(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.currentDrawdown = 22.0 // Exceeds 20% limit

	intentions := agent.evaluateProposal("BTC/USDT", "BUY", 5000.0, 0.8)

	assert.True(t, intentions.shouldVeto)
	assert.Contains(t, intentions.vetoReason, "drawdown")
	assert.Contains(t, intentions.vetoReason, "circuit breaker")
	assert.GreaterOrEqual(t, intentions.confidenceScore, 0.95)
}

func TestEvaluateProposal_VetoOnApproachingDrawdown(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.currentDrawdown = 17.0 // 85% of 20% limit

	intentions := agent.evaluateProposal("BTC/USDT", "BUY", 5000.0, 0.8)

	assert.True(t, intentions.shouldVeto)
	assert.Contains(t, intentions.vetoReason, "Approaching maximum drawdown")
	assert.Greater(t, intentions.confidenceScore, 0.8)
}

func TestEvaluateProposal_VetoOnHighVolatilityAndUtilization(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.volatility = 0.05        // 5% volatility (high)
	agent.beliefs.limitsUtilization = 0.90 // 90% utilization (high)

	intentions := agent.evaluateProposal("BTC/USDT", "BUY", 5000.0, 0.8)

	assert.True(t, intentions.shouldVeto)
	assert.Contains(t, intentions.vetoReason, "High volatility")
	assert.Contains(t, intentions.vetoReason, "high utilization")
}

func TestEvaluateProposal_RecommendSmallerSize(t *testing.T) {
	agent := createTestRiskAgent()

	// Request size larger than optimal but within max limits
	// Optimal size is ~2000, so request 4000 (2x optimal)
	intentions := agent.evaluateProposal("BTC/USDT", "BUY", 4000.0, 0.8)

	// Should not veto but recommend smaller size
	assert.False(t, intentions.shouldVeto)
	assert.Less(t, intentions.recommendedSize, 4000.0)
	assert.Contains(t, intentions.vetoReason, "Recommended size")
	assert.Contains(t, intentions.vetoReason, "Kelly Criterion")
}

func TestEvaluateProposal_VetoOnConcentration(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.totalExposure = 40000.0
	agent.beliefs.currentPositions = []Position{
		{Symbol: "BTC/USDT", Size: 9000.0},
	}

	// Try to add more BTC (approaching 25% concentration limit)
	intentions := agent.evaluateProposal("BTC/USDT", "BUY", 3000.0, 0.8)

	assert.True(t, intentions.shouldVeto)
	assert.Contains(t, intentions.vetoReason, "concentration")
	assert.Contains(t, intentions.vetoReason, "diversification")
}

func TestEvaluateProposal_NoVetoOnSell(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.currentDrawdown = 17.0 // Would veto BUY

	// SELL should not be vetoed even with high drawdown
	intentions := agent.evaluateProposal("BTC/USDT", "SELL", 5000.0, 0.8)

	assert.False(t, intentions.shouldVeto)
}

// ============================================================================
// T124: RISK ASSESSMENT TESTS
// ============================================================================

func TestAssessRisk_ApprovalFlow(t *testing.T) {
	agent := createTestRiskAgent()

	signal, confidence, reasoning := agent.assessRisk("BTC/USDT", "BUY")

	assert.Equal(t, "BUY", signal)
	assert.GreaterOrEqual(t, confidence, 0.7)
	assert.Contains(t, reasoning, "RISK MANAGEMENT APPROVAL")
	assert.Contains(t, reasoning, "PORTFOLIO HEALTH")
	assert.Contains(t, reasoning, "KELLY CRITERION")
	assert.Equal(t, int64(1), agent.approvalCount)
}

func TestAssessRisk_VetoFlow(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.currentDrawdown = 22.0 // Exceeds limit

	signal, confidence, reasoning := agent.assessRisk("BTC/USDT", "BUY")

	assert.Equal(t, "HOLD", signal)
	assert.Greater(t, confidence, 0.9)
	assert.Contains(t, reasoning, "RISK MANAGEMENT VETO")
	assert.Contains(t, reasoning, "drawdown")
	assert.Contains(t, reasoning, "circuit breaker")
	assert.Equal(t, int64(1), agent.vetoCount)
}

func TestAssessRisk_CountersIncrement(t *testing.T) {
	agent := createTestRiskAgent()

	// Approval
	agent.assessRisk("BTC/USDT", "BUY")
	assert.Equal(t, int64(1), agent.approvalCount)
	assert.Equal(t, int64(1), agent.totalDecisions)

	// Veto
	agent.beliefs.currentDrawdown = 22.0
	agent.assessRisk("ETH/USDT", "BUY")
	assert.Equal(t, int64(1), agent.vetoCount)
	assert.Equal(t, int64(2), agent.totalDecisions)
}

func TestBuildVetoReasoning(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.currentDrawdown = 18.0
	agent.beliefs.totalExposure = 30000.0
	agent.beliefs.openPositionCount = 3

	intentions := &RiskIntentions{
		shouldVeto:      true,
		vetoReason:      "Test veto reason",
		confidenceScore: 0.95,
	}

	reasoning := buildVetoReasoning(intentions, agent.beliefs, agent.config)

	assert.Contains(t, reasoning, "RISK MANAGEMENT VETO")
	assert.Contains(t, reasoning, "Test veto reason")
	assert.Contains(t, reasoning, "CURRENT PORTFOLIO STATE")
	assert.Contains(t, reasoning, "Open Positions: 3")
	assert.Contains(t, reasoning, "RISK METRICS")
	assert.Contains(t, reasoning, "RECOMMENDATION: HOLD")
}

func TestBuildApprovalReasoning(t *testing.T) {
	agent := createTestRiskAgent()

	intentions := &RiskIntentions{
		shouldVeto:      false,
		recommendedSize: 5000.0,
		stopLossLevel:   48000.0,
		confidenceScore: 0.90,
	}

	reasoning := buildApprovalReasoning(intentions, agent.beliefs, agent.config)

	assert.Contains(t, reasoning, "RISK MANAGEMENT APPROVAL")
	assert.Contains(t, reasoning, "PORTFOLIO HEALTH")
	assert.Contains(t, reasoning, "KELLY CRITERION")
	assert.Contains(t, reasoning, "Recommended Size:")
	assert.Contains(t, reasoning, "Stop Loss Level:")
	assert.Contains(t, reasoning, "RECOMMENDATION: PROCEED")
}

// ============================================================================
// BELIEF SYSTEM TESTS
// ============================================================================

func TestGetSymbolExposure(t *testing.T) {
	agent := createTestRiskAgent()
	agent.beliefs.currentPositions = []Position{
		{Symbol: "BTC/USDT", Size: 5000.0},
		{Symbol: "BTC/USDT", Size: 3000.0},
		{Symbol: "ETH/USDT", Size: 2000.0},
	}

	btcExposure := agent.getSymbolExposure("BTC/USDT")
	assert.Equal(t, 8000.0, btcExposure)

	ethExposure := agent.getSymbolExposure("ETH/USDT")
	assert.Equal(t, 2000.0, ethExposure)

	solExposure := agent.getSymbolExposure("SOL/USDT")
	assert.Equal(t, 0.0, solExposure)
}

func TestAssessMarketConditions(t *testing.T) {
	agent := createTestRiskAgent()

	agent.assessMarketConditions()

	assert.NotEmpty(t, agent.beliefs.marketRegime)
	assert.Greater(t, agent.beliefs.volatility, 0.0)
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestRiskAgent_CompleteWorkflow(t *testing.T) {
	agent := createTestRiskAgent()

	// Scenario 1: Normal trading conditions - should approve
	signal1, conf1, reason1 := agent.assessRisk("BTC/USDT", "BUY")
	assert.Equal(t, "BUY", signal1)
	assert.GreaterOrEqual(t, conf1, 0.7)
	assert.Contains(t, reason1, "APPROVAL")

	// Scenario 2: High drawdown - should veto BUY
	agent.beliefs.currentDrawdown = 22.0
	signal2, conf2, reason2 := agent.assessRisk("ETH/USDT", "BUY")
	assert.Equal(t, "HOLD", signal2)
	assert.GreaterOrEqual(t, conf2, 0.9)
	assert.Contains(t, reason2, "VETO")

	// Scenario 3: SELL in high drawdown - should allow (to reduce exposure)
	signal3, conf3, reason3 := agent.assessRisk("BTC/USDT", "SELL")
	assert.Equal(t, "SELL", signal3)
	assert.GreaterOrEqual(t, conf3, 0.7)
	assert.Contains(t, reason3, "APPROVAL")

	// Verify counters
	assert.Equal(t, int64(2), agent.approvalCount) // BUY + SELL
	assert.Equal(t, int64(1), agent.vetoCount)     // ETH BUY
	assert.Equal(t, int64(3), agent.totalDecisions)
}
