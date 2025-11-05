package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAgentConfig(t *testing.T) {
	// Load config from default location
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test global config
	assert.Equal(t, "30s", cfg.Global.DefaultStepInterval)
	assert.Equal(t, 0.7, cfg.Global.DefaultConfidenceThreshold)
	assert.True(t, cfg.Global.EnableMetrics)
	assert.Equal(t, 9101, cfg.Global.MetricsPort)
}

func TestAnalysisAgentConfig(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test technical agent
	technicalAgent, ok := cfg.AnalysisAgents["technical"]
	require.True(t, ok, "Technical agent should exist in config")
	assert.True(t, technicalAgent.Enabled)
	assert.Equal(t, "technical-agent", technicalAgent.Name)
	assert.Equal(t, "analysis", technicalAgent.Type)
	assert.Equal(t, "1.0.0", technicalAgent.Version)
	assert.Equal(t, "30s", technicalAgent.StepInterval)
	assert.NotNil(t, technicalAgent.Config)

	// Test MCP server connections
	require.Len(t, technicalAgent.MCPServers, 2)
	assert.Equal(t, "coingecko", technicalAgent.MCPServers[0].Name)
	assert.Equal(t, "external", technicalAgent.MCPServers[0].Type)
	assert.Equal(t, "technical_indicators", technicalAgent.MCPServers[1].Name)
	assert.Equal(t, "internal", technicalAgent.MCPServers[1].Type)
	assert.Equal(t, "./bin/technical-indicators-server", technicalAgent.MCPServers[1].Command)

	// Test orderbook agent (should be disabled)
	orderbookAgent, ok := cfg.AnalysisAgents["orderbook"]
	require.True(t, ok)
	assert.False(t, orderbookAgent.Enabled)
	assert.Equal(t, "orderbook-agent", orderbookAgent.Name)
	assert.Equal(t, "10s", orderbookAgent.StepInterval)

	// Test sentiment agent (now enabled as Phase 3.4 is complete)
	sentimentAgent, ok := cfg.AnalysisAgents["sentiment"]
	require.True(t, ok)
	assert.True(t, sentimentAgent.Enabled)
	assert.Equal(t, "sentiment-agent", sentimentAgent.Name)
	assert.Equal(t, "5m", sentimentAgent.StepInterval)
}

func TestStrategyAgentConfig(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test trend following agent (key changed from "trend_following" to "trend")
	trendAgent, ok := cfg.StrategyAgents["trend"]
	require.True(t, ok)
	assert.True(t, trendAgent.Enabled)
	assert.Equal(t, "trend-follower", trendAgent.Name)
	assert.Equal(t, "strategy", trendAgent.Type)
	assert.Equal(t, "1.0.0", trendAgent.Version)
	assert.Equal(t, "5m", trendAgent.StepInterval)
	assert.NotNil(t, trendAgent.Config)

	// Verify strategy-specific config (symbols instead of "strategy" field)
	symbols := trendAgent.Config["symbols"]
	assert.NotNil(t, symbols)

	// Test MCP server connections
	require.Len(t, trendAgent.MCPServers, 2)
	assert.Equal(t, "coingecko", trendAgent.MCPServers[0].Name)
	assert.Equal(t, "technical_indicators", trendAgent.MCPServers[1].Name)

	// Test mean reversion agent (should be disabled)
	reversionAgent, ok := cfg.StrategyAgents["mean_reversion"]
	require.True(t, ok)
	assert.False(t, reversionAgent.Enabled)
	assert.Equal(t, "reversion-agent", reversionAgent.Name)

	// Test arbitrage agent (should be disabled)
	arbitrageAgent, ok := cfg.StrategyAgents["arbitrage"]
	require.True(t, ok)
	assert.False(t, arbitrageAgent.Enabled)
	assert.Equal(t, "arbitrage-agent", arbitrageAgent.Name)
	assert.Equal(t, "5s", arbitrageAgent.StepInterval)
}

func TestRiskAgentConfig(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test risk agent
	riskAgent := cfg.RiskAgent
	assert.True(t, riskAgent.Enabled)
	assert.Equal(t, "risk-agent", riskAgent.Name)
	assert.Equal(t, "risk", riskAgent.Type)
	assert.Equal(t, "1.0.0", riskAgent.Version)
	assert.Equal(t, "10s", riskAgent.StepInterval)
	assert.NotNil(t, riskAgent.Config)

	// Test MCP server connections
	require.Len(t, riskAgent.MCPServers, 2)
	assert.Equal(t, "risk_analyzer", riskAgent.MCPServers[0].Name)
	assert.Equal(t, "internal", riskAgent.MCPServers[0].Type)
	assert.Equal(t, "./bin/risk-analyzer-server", riskAgent.MCPServers[0].Command)
	assert.Equal(t, "order_executor", riskAgent.MCPServers[1].Name)

	// Verify risk-specific config
	vetoPower := riskAgent.Config["veto_power"]
	assert.Equal(t, true, vetoPower)
}

func TestOrchestrationConfig(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test voting config
	voting := cfg.Orchestration.Voting
	assert.True(t, voting.Enabled)
	assert.Equal(t, "weighted_consensus", voting.Method)
	assert.Equal(t, 2, voting.MinVotes)
	assert.Equal(t, 0.6, voting.Quorum)

	// Test LLM reasoning config
	llm := cfg.Orchestration.LLMReasoning
	assert.True(t, llm.Enabled)
	assert.Equal(t, "claude-sonnet-4-20250514", llm.Model)
	assert.Equal(t, 2000, llm.MaxTokens)
	assert.Equal(t, 0.7, llm.Temperature)
	assert.Equal(t, "templates/agent_decision.txt", llm.PromptTemplate)

	// Test coordination config
	coord := cfg.Orchestration.Coordination
	assert.True(t, coord.BroadcastSignals)
	assert.Equal(t, "5m", coord.SignalExpiry)
	assert.False(t, coord.EnableLearning)

	// Test performance config
	perf := cfg.Orchestration.Performance
	assert.True(t, perf.TrackAgentAccuracy)
	assert.False(t, perf.AdjustWeights)
	assert.Equal(t, 50, perf.MinSampleSize)
}

func TestCommunicationConfig(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test NATS topics
	topics := cfg.Communication.NATS.Topics
	assert.Equal(t, "agents.analysis.technical", topics.TechnicalSignals)
	assert.Equal(t, "agents.analysis.orderbook", topics.OrderbookSignals)
	assert.Equal(t, "agents.analysis.sentiment", topics.SentimentSignals)
	assert.Equal(t, "agents.strategy.decisions", topics.StrategyDecisions)
	assert.Equal(t, "agents.strategy.proposals", topics.TradeProposals)
	assert.Equal(t, "agents.risk.approvals", topics.RiskApprovals)
	assert.Equal(t, "agents.risk.vetoes", topics.RiskVetoes)
	assert.Equal(t, "agents.system.heartbeat", topics.AgentHeartbeat)
	assert.Equal(t, "agents.system.errors", topics.AgentErrors)

	// Test NATS retention
	retention := cfg.Communication.NATS.Retention
	assert.Equal(t, "1h", retention.Signals)
	assert.Equal(t, "24h", retention.Decisions)
	assert.Equal(t, "5m", retention.Heartbeat)
}

func TestLoggingConfig(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test logging config
	logging := cfg.Logging
	assert.Equal(t, "info", logging.Level)
	assert.Equal(t, "json", logging.Format)
	assert.Equal(t, "stderr", logging.Output)

	// Test agent-specific log levels
	assert.Equal(t, "debug", logging.AgentLevels["technical-agent"])
	assert.Equal(t, "info", logging.AgentLevels["risk-agent"])
	assert.Equal(t, "info", logging.AgentLevels["trend-agent"])
}

func TestGetStepIntervalDuration(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test parsing step intervals
	duration, err := cfg.GetStepIntervalDuration("30s")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, duration)

	duration, err = cfg.GetStepIntervalDuration("1m")
	require.NoError(t, err)
	assert.Equal(t, 1*time.Minute, duration)

	duration, err = cfg.GetStepIntervalDuration("5m")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, duration)

	duration, err = cfg.GetStepIntervalDuration("10s")
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, duration)
}

func TestGetEnabledAgents(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test enabled analysis agents
	enabledAnalysis := cfg.GetEnabledAnalysisAgents()
	assert.Contains(t, enabledAnalysis, "technical")
	assert.NotContains(t, enabledAnalysis, "orderbook") // disabled
	assert.Contains(t, enabledAnalysis, "sentiment")    // enabled (Phase 3.4 complete)

	// Test enabled strategy agents (key changed from "trend_following" to "trend")
	enabledStrategy := cfg.GetEnabledStrategyAgents()
	assert.Contains(t, enabledStrategy, "trend")
	assert.NotContains(t, enabledStrategy, "mean_reversion") // disabled
	assert.NotContains(t, enabledStrategy, "arbitrage")      // disabled

	// Test risk agent
	assert.True(t, cfg.IsRiskAgentEnabled())
}

func TestMCPServerTools(t *testing.T) {
	cfg, err := LoadAgentConfig("../../configs/agents.yaml")
	require.NoError(t, err)

	// Test technical agent tools
	technicalAgent := cfg.AnalysisAgents["technical"]
	coingeckoServer := technicalAgent.MCPServers[0]
	assert.Contains(t, coingeckoServer.Tools, "get_price")
	assert.Contains(t, coingeckoServer.Tools, "get_market_chart")

	techIndicatorsServer := technicalAgent.MCPServers[1]
	assert.Contains(t, techIndicatorsServer.Tools, "calculate_rsi")
	assert.Contains(t, techIndicatorsServer.Tools, "calculate_macd")
	assert.Contains(t, techIndicatorsServer.Tools, "calculate_bollinger_bands")
	assert.Contains(t, techIndicatorsServer.Tools, "calculate_ema")
	assert.Contains(t, techIndicatorsServer.Tools, "calculate_adx")

	// Test risk agent tools
	riskAgent := cfg.RiskAgent
	riskAnalyzerServer := riskAgent.MCPServers[0]
	assert.Contains(t, riskAnalyzerServer.Tools, "calculate_position_size")
	assert.Contains(t, riskAnalyzerServer.Tools, "calculate_var")
	assert.Contains(t, riskAnalyzerServer.Tools, "check_portfolio_limits")
	assert.Contains(t, riskAnalyzerServer.Tools, "calculate_sharpe")
	assert.Contains(t, riskAnalyzerServer.Tools, "calculate_drawdown")
}
