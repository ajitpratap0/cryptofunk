// Package config provides configuration management for CryptoFunk.
// This file centralizes all port constants to avoid duplication and ensure consistency.
package config

// ============================================================================
// CENTRALIZED PORT CONFIGURATION
// ============================================================================
//
// This file defines all ports used by CryptoFunk services.
// Update this file when adding new services or changing port assignments.
//
// Port Allocation Strategy:
//   8080-8099: API servers and web services
//   8200-8299: Infrastructure services (Vault, etc.)
//   9100-9199: Prometheus metrics endpoints
//
// ============================================================================

// API and Web Service Ports
const (
	// APIServerPort is the port for the main REST API server.
	APIServerPort = 8080

	// OrchestratorPort is the port for the orchestrator HTTP server.
	OrchestratorPort = 8081

	// WebSocketPort is the port for WebSocket connections (uses same as API).
	WebSocketPort = APIServerPort
)

// Infrastructure Service Ports
const (
	// VaultPort is the default port for HashiCorp Vault.
	VaultPort = 8200

	// PostgresPort is the default port for PostgreSQL.
	PostgresPort = 5432

	// RedisPort is the default port for Redis.
	RedisPort = 6379

	// NATSPort is the default port for NATS messaging.
	NATSPort = 4222
)

// Prometheus Metrics Ports for Trading Agents
// Each agent gets a unique port for metrics scraping.
const (
	// MetricsPortTechnicalAgent is the metrics port for the technical analysis agent.
	MetricsPortTechnicalAgent = 9101

	// MetricsPortTrendAgent is the metrics port for the trend following agent.
	MetricsPortTrendAgent = 9102

	// MetricsPortSentimentAgent is the metrics port for the sentiment analysis agent.
	// Note: Port 9103 was skipped to maintain gap, 9104 is used.
	MetricsPortSentimentAgent = 9104

	// MetricsPortOrderbookAgent is the metrics port for the orderbook analysis agent.
	MetricsPortOrderbookAgent = 9105

	// MetricsPortReversionAgent is the metrics port for the mean reversion agent.
	MetricsPortReversionAgent = 9106

	// MetricsPortArbitrageAgent is the metrics port for the arbitrage agent.
	MetricsPortArbitrageAgent = 9107

	// MetricsPortRiskAgent is the metrics port for the risk management agent.
	MetricsPortRiskAgent = 9108

	// MetricsPortOrchestrator is the metrics port for the orchestrator.
	// Note: Orchestrator serves metrics on its main HTTP port.
	MetricsPortOrchestrator = OrchestratorPort
)

// Monitoring Service Ports
const (
	// PrometheusPort is the default port for Prometheus.
	PrometheusPort = 9090

	// GrafanaPort is the default port for Grafana.
	GrafanaPort = 3000

	// NATSExporterPort is the port for the NATS Prometheus exporter.
	NATSExporterPort = 7777
)

// AgentMetricsPorts provides a mapping of agent names to their metrics ports.
// This is useful for Prometheus configuration and health checks.
var AgentMetricsPorts = map[string]int{
	"technical-agent": MetricsPortTechnicalAgent,
	"trend-agent":     MetricsPortTrendAgent,
	"sentiment-agent": MetricsPortSentimentAgent,
	"orderbook-agent": MetricsPortOrderbookAgent,
	"reversion-agent": MetricsPortReversionAgent,
	"arbitrage-agent": MetricsPortArbitrageAgent,
	"risk-agent":      MetricsPortRiskAgent,
}

// GetAgentMetricsPort returns the metrics port for a given agent name.
// Returns 0 if the agent is not found.
func GetAgentMetricsPort(agentName string) int {
	if port, ok := AgentMetricsPorts[agentName]; ok {
		return port
	}
	return 0
}
