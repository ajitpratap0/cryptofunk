package config

import "testing"

func TestGetAgentMetricsPort(t *testing.T) {
	tests := []struct {
		name      string
		agentName string
		expected  int
	}{
		{"technical-agent", "technical-agent", MetricsPortTechnicalAgent},
		{"trend-agent", "trend-agent", MetricsPortTrendAgent},
		{"sentiment-agent", "sentiment-agent", MetricsPortSentimentAgent},
		{"orderbook-agent", "orderbook-agent", MetricsPortOrderbookAgent},
		{"reversion-agent", "reversion-agent", MetricsPortReversionAgent},
		{"arbitrage-agent", "arbitrage-agent", MetricsPortArbitrageAgent},
		{"risk-agent", "risk-agent", MetricsPortRiskAgent},
		{"unknown-agent returns 0", "unknown-agent", 0},
		{"empty name returns 0", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAgentMetricsPort(tt.agentName)
			if got != tt.expected {
				t.Errorf("GetAgentMetricsPort(%q) = %d, want %d", tt.agentName, got, tt.expected)
			}
		})
	}
}

func TestAgentMetricsPorts(t *testing.T) {
	// Verify all expected agents are in the map
	expectedAgents := []string{
		"technical-agent", "trend-agent", "sentiment-agent",
		"orderbook-agent", "reversion-agent", "arbitrage-agent", "risk-agent",
	}

	for _, agent := range expectedAgents {
		if _, ok := AgentMetricsPorts[agent]; !ok {
			t.Errorf("AgentMetricsPorts missing expected agent: %s", agent)
		}
	}

	// Verify we have exactly 7 agents
	if len(AgentMetricsPorts) != 7 {
		t.Errorf("AgentMetricsPorts has %d agents, expected 7", len(AgentMetricsPorts))
	}
}

func TestAgentMetricsPortsValues(t *testing.T) {
	// Verify that each agent has a unique port and the port is in the expected range
	tests := []struct {
		agentName    string
		expectedPort int
	}{
		{"technical-agent", 9101},
		{"trend-agent", 9102},
		{"sentiment-agent", 9104},
		{"orderbook-agent", 9105},
		{"reversion-agent", 9106},
		{"arbitrage-agent", 9107},
		{"risk-agent", 9108},
	}

	seenPorts := make(map[int]string)

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			port := AgentMetricsPorts[tt.agentName]

			// Verify the port matches the expected value
			if port != tt.expectedPort {
				t.Errorf("AgentMetricsPorts[%q] = %d, want %d", tt.agentName, port, tt.expectedPort)
			}

			// Verify the port is in the valid Prometheus metrics range (9100-9199)
			if port < 9100 || port > 9199 {
				t.Errorf("AgentMetricsPorts[%q] = %d, port should be in range 9100-9199", tt.agentName, port)
			}

			// Verify each agent has a unique port
			if existingAgent, exists := seenPorts[port]; exists {
				t.Errorf("Port %d is used by both %q and %q", port, existingAgent, tt.agentName)
			}
			seenPorts[port] = tt.agentName
		})
	}
}

func TestAgentMetricsPortsConsistency(t *testing.T) {
	// Verify that GetAgentMetricsPort returns the same values as direct map access
	for agentName, expectedPort := range AgentMetricsPorts {
		t.Run(agentName, func(t *testing.T) {
			got := GetAgentMetricsPort(agentName)
			if got != expectedPort {
				t.Errorf("GetAgentMetricsPort(%q) = %d, but AgentMetricsPorts[%q] = %d",
					agentName, got, agentName, expectedPort)
			}
		})
	}
}
