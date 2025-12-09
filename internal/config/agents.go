package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// AgentConfig holds complete agent system configuration
type AgentConfig struct {
	Global         GlobalAgentConfig        `mapstructure:"global"`
	AnalysisAgents map[string]AnalysisAgent `mapstructure:"analysis_agents"`
	StrategyAgents map[string]StrategyAgent `mapstructure:"strategy_agents"`
	RiskAgent      RiskAgent                `mapstructure:"risk_agent"`
	Orchestration  OrchestrationConfig      `mapstructure:"orchestration"`
	Communication  CommunicationConfig      `mapstructure:"communication"`
	Logging        LoggingConfig            `mapstructure:"logging"`
}

// GlobalAgentConfig contains settings that apply to all agents
type GlobalAgentConfig struct {
	DefaultStepInterval        string  `mapstructure:"default_step_interval"`
	DefaultConfidenceThreshold float64 `mapstructure:"default_confidence_threshold"`
	EnableMetrics              bool    `mapstructure:"enable_metrics"`
	MetricsPort                int     `mapstructure:"metrics_port"`
}

// AnalysisAgent represents an analysis agent configuration
type AnalysisAgent struct {
	Enabled      bool                   `mapstructure:"enabled"`
	Name         string                 `mapstructure:"name"`
	Type         string                 `mapstructure:"type"`
	Version      string                 `mapstructure:"version"`
	MCPServers   []MCPServerConnection  `mapstructure:"mcp_servers"`
	StepInterval string                 `mapstructure:"step_interval"`
	Config       map[string]interface{} `mapstructure:"config"`
}

// StrategyAgent represents a strategy agent configuration
type StrategyAgent struct {
	Enabled      bool                   `mapstructure:"enabled"`
	Name         string                 `mapstructure:"name"`
	Type         string                 `mapstructure:"type"`
	Version      string                 `mapstructure:"version"`
	MCPServers   []MCPServerConnection  `mapstructure:"mcp_servers"`
	StepInterval string                 `mapstructure:"step_interval"`
	Config       map[string]interface{} `mapstructure:"config"`
}

// RiskAgent represents the risk management agent configuration
type RiskAgent struct {
	Enabled      bool                   `mapstructure:"enabled"`
	Name         string                 `mapstructure:"name"`
	Type         string                 `mapstructure:"type"`
	Version      string                 `mapstructure:"version"`
	MCPServers   []MCPServerConnection  `mapstructure:"mcp_servers"`
	StepInterval string                 `mapstructure:"step_interval"`
	Config       map[string]interface{} `mapstructure:"config"`
}

// MCPServerConnection describes how an agent connects to an MCP server
type MCPServerConnection struct {
	Name    string   `mapstructure:"name"`
	Type    string   `mapstructure:"type"`    // "external" or "internal"
	URL     string   `mapstructure:"url"`     // For external servers (e.g., CoinGecko MCP)
	Command string   `mapstructure:"command"` // For internal servers
	Tools   []string `mapstructure:"tools"`   // Tools agent will use
}

// OrchestrationConfig defines how agents coordinate
type OrchestrationConfig struct {
	Voting       VotingConfig       `mapstructure:"voting"`
	LLMReasoning LLMReasoningConfig `mapstructure:"llm_reasoning"`
	Coordination CoordinationConfig `mapstructure:"coordination"`
	Performance  PerformanceConfig  `mapstructure:"performance"`
}

// VotingConfig defines the voting mechanism
type VotingConfig struct {
	Enabled  bool    `mapstructure:"enabled"`
	Method   string  `mapstructure:"method"` // "weighted_consensus" or "majority"
	MinVotes int     `mapstructure:"min_votes"`
	Quorum   float64 `mapstructure:"quorum"`
}

// LLMReasoningConfig defines LLM-based reasoning
type LLMReasoningConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	Model          string  `mapstructure:"model"`
	MaxTokens      int     `mapstructure:"max_tokens"`
	Temperature    float64 `mapstructure:"temperature"`
	PromptTemplate string  `mapstructure:"prompt_template"`
}

// CoordinationConfig defines agent coordination
type CoordinationConfig struct {
	BroadcastSignals bool   `mapstructure:"broadcast_signals"`
	SignalExpiry     string `mapstructure:"signal_expiry"`
	EnableLearning   bool   `mapstructure:"enable_learning"`
}

// PerformanceConfig defines performance tracking
type PerformanceConfig struct {
	TrackAgentAccuracy bool `mapstructure:"track_agent_accuracy"`
	AdjustWeights      bool `mapstructure:"adjust_weights"`
	MinSampleSize      int  `mapstructure:"min_sample_size"`
}

// CommunicationConfig defines inter-agent communication
type CommunicationConfig struct {
	NATS NATSCommunicationConfig `mapstructure:"nats"`
}

// NATSCommunicationConfig defines NATS topics and retention
type NATSCommunicationConfig struct {
	Topics    NATSTopics    `mapstructure:"topics"`
	Retention NATSRetention `mapstructure:"retention"`
}

// NATSTopics defines topic names for different message types
type NATSTopics struct {
	TechnicalSignals  string `mapstructure:"technical_signals"`
	OrderbookSignals  string `mapstructure:"orderbook_signals"`
	SentimentSignals  string `mapstructure:"sentiment_signals"`
	StrategyDecisions string `mapstructure:"strategy_decisions"`
	TradeProposals    string `mapstructure:"trade_proposals"`
	RiskApprovals     string `mapstructure:"risk_approvals"`
	RiskVetoes        string `mapstructure:"risk_vetoes"`
	AgentHeartbeat    string `mapstructure:"agent_heartbeat"`
	AgentErrors       string `mapstructure:"agent_errors"`
}

// NATSRetention defines message retention policies
type NATSRetention struct {
	Signals   string `mapstructure:"signals"`
	Decisions string `mapstructure:"decisions"`
	Heartbeat string `mapstructure:"heartbeat"`
}

// LoggingConfig defines agent logging settings
type LoggingConfig struct {
	Level       string            `mapstructure:"level"`
	Format      string            `mapstructure:"format"`
	Output      string            `mapstructure:"output"`
	AgentLevels map[string]string `mapstructure:"agent_levels"`
}

// LoadAgentConfig loads agent configuration from file
func LoadAgentConfig(configPath string) (*AgentConfig, error) {
	v := viper.New()

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("agents")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
		v.AddConfigPath("../../configs")
	}

	// Set defaults
	setAgentDefaults(v)

	// Enable environment variable override
	v.SetEnvPrefix("CRYPTOFUNK_AGENT")
	v.AutomaticEnv()

	// Read config
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read agent config: %w", err)
	}

	// Unmarshal into struct
	var cfg AgentConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent config: %w", err)
	}

	return &cfg, nil
}

// setAgentDefaults sets default agent configuration values
func setAgentDefaults(v *viper.Viper) {
	// Global defaults
	v.SetDefault("global.default_step_interval", "30s")
	v.SetDefault("global.default_confidence_threshold", 0.7)
	v.SetDefault("global.enable_metrics", true)
	v.SetDefault("global.metrics_port", 9101)

	// Analysis agents - Technical
	v.SetDefault("analysis_agents.technical.enabled", true)
	v.SetDefault("analysis_agents.technical.name", "technical-agent")
	v.SetDefault("analysis_agents.technical.type", "analysis")
	v.SetDefault("analysis_agents.technical.version", "1.0.0")
	v.SetDefault("analysis_agents.technical.step_interval", "30s")

	// Analysis agents - Orderbook
	v.SetDefault("analysis_agents.orderbook.enabled", false)
	v.SetDefault("analysis_agents.orderbook.name", "orderbook-agent")
	v.SetDefault("analysis_agents.orderbook.type", "analysis")
	v.SetDefault("analysis_agents.orderbook.version", "1.0.0")
	v.SetDefault("analysis_agents.orderbook.step_interval", "10s")

	// Analysis agents - Sentiment
	v.SetDefault("analysis_agents.sentiment.enabled", false)
	v.SetDefault("analysis_agents.sentiment.name", "sentiment-agent")
	v.SetDefault("analysis_agents.sentiment.type", "analysis")
	v.SetDefault("analysis_agents.sentiment.version", "1.0.0")
	v.SetDefault("analysis_agents.sentiment.step_interval", "5m")

	// Strategy agents - Trend Following
	v.SetDefault("strategy_agents.trend_following.enabled", true)
	v.SetDefault("strategy_agents.trend_following.name", "trend-agent")
	v.SetDefault("strategy_agents.trend_following.type", "strategy")
	v.SetDefault("strategy_agents.trend_following.version", "1.0.0")
	v.SetDefault("strategy_agents.trend_following.step_interval", "1m")

	// Strategy agents - Mean Reversion
	v.SetDefault("strategy_agents.mean_reversion.enabled", false)
	v.SetDefault("strategy_agents.mean_reversion.name", "reversion-agent")
	v.SetDefault("strategy_agents.mean_reversion.type", "strategy")
	v.SetDefault("strategy_agents.mean_reversion.version", "1.0.0")
	v.SetDefault("strategy_agents.mean_reversion.step_interval", "1m")

	// Strategy agents - Arbitrage
	v.SetDefault("strategy_agents.arbitrage.enabled", false)
	v.SetDefault("strategy_agents.arbitrage.name", "arbitrage-agent")
	v.SetDefault("strategy_agents.arbitrage.type", "strategy")
	v.SetDefault("strategy_agents.arbitrage.version", "1.0.0")
	v.SetDefault("strategy_agents.arbitrage.step_interval", "5s")

	// Risk agent
	v.SetDefault("risk_agent.enabled", true)
	v.SetDefault("risk_agent.name", "risk-agent")
	v.SetDefault("risk_agent.type", "risk")
	v.SetDefault("risk_agent.version", "1.0.0")
	v.SetDefault("risk_agent.step_interval", "10s")

	// Orchestration - Voting
	v.SetDefault("orchestration.voting.enabled", true)
	v.SetDefault("orchestration.voting.method", "weighted_consensus")
	v.SetDefault("orchestration.voting.min_votes", 2)
	v.SetDefault("orchestration.voting.quorum", 0.6)

	// Orchestration - LLM Reasoning
	v.SetDefault("orchestration.llm_reasoning.enabled", true)
	v.SetDefault("orchestration.llm_reasoning.model", "claude-sonnet-4-20250514")
	v.SetDefault("orchestration.llm_reasoning.max_tokens", 2000)
	v.SetDefault("orchestration.llm_reasoning.temperature", 0.7)
	v.SetDefault("orchestration.llm_reasoning.prompt_template", "templates/agent_decision.txt")

	// Orchestration - Coordination
	v.SetDefault("orchestration.coordination.broadcast_signals", true)
	v.SetDefault("orchestration.coordination.signal_expiry", "5m")
	v.SetDefault("orchestration.coordination.enable_learning", false)

	// Orchestration - Performance
	v.SetDefault("orchestration.performance.track_agent_accuracy", true)
	v.SetDefault("orchestration.performance.adjust_weights", false)
	v.SetDefault("orchestration.performance.min_sample_size", 50)

	// Communication - NATS Topics
	v.SetDefault("communication.nats.topics.technical_signals", "agents.analysis.technical")
	v.SetDefault("communication.nats.topics.orderbook_signals", "agents.analysis.orderbook")
	v.SetDefault("communication.nats.topics.sentiment_signals", "agents.analysis.sentiment")
	v.SetDefault("communication.nats.topics.strategy_decisions", "agents.strategy.decisions")
	v.SetDefault("communication.nats.topics.trade_proposals", "agents.strategy.proposals")
	v.SetDefault("communication.nats.topics.risk_approvals", "agents.risk.approvals")
	v.SetDefault("communication.nats.topics.risk_vetoes", "agents.risk.vetoes")
	v.SetDefault("communication.nats.topics.agent_heartbeat", "agents.system.heartbeat")
	v.SetDefault("communication.nats.topics.agent_errors", "agents.system.errors")

	// Communication - NATS Retention
	v.SetDefault("communication.nats.retention.signals", "1h")
	v.SetDefault("communication.nats.retention.decisions", "24h")
	v.SetDefault("communication.nats.retention.heartbeat", "5m")

	// Logging
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stderr")
}

// GetStepIntervalDuration parses step interval string to time.Duration
func (ac *AgentConfig) GetStepIntervalDuration(stepInterval string) (time.Duration, error) {
	return time.ParseDuration(stepInterval)
}

// GetEnabledAnalysisAgents returns list of enabled analysis agents
func (ac *AgentConfig) GetEnabledAnalysisAgents() []string {
	var enabled []string
	for name, agent := range ac.AnalysisAgents {
		if agent.Enabled {
			enabled = append(enabled, name)
		}
	}
	return enabled
}

// GetEnabledStrategyAgents returns list of enabled strategy agents
func (ac *AgentConfig) GetEnabledStrategyAgents() []string {
	var enabled []string
	for name, agent := range ac.StrategyAgents {
		if agent.Enabled {
			enabled = append(enabled, name)
		}
	}
	return enabled
}

// IsRiskAgentEnabled checks if risk agent is enabled
func (ac *AgentConfig) IsRiskAgentEnabled() bool {
	return ac.RiskAgent.Enabled
}
