// Shared helper functions for E2E tests
package e2e

import (
	"encoding/json"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

// startEmbeddedNATS starts an embedded NATS server for testing
func startEmbeddedNATS(t *testing.T) *natsserver.Server {
	t.Helper()
	opts := &natsserver.Options{
		Host:           "127.0.0.1",
		Port:           -1, // Random port
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
	}
	ns, err := natsserver.NewServer(opts)
	require.NoError(t, err)

	go ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(4 * time.Second) {
		t.Fatal("NATS server did not start in time")
	}

	return ns
}

// sendHeartbeat sends an agent heartbeat message via NATS
func sendHeartbeat(t *testing.T, nc *nats.Conn, topic, agentName, agentType string, weight float64) {
	t.Helper()
	heartbeat := map[string]interface{}{
		"agent_name":       agentName,
		"agent_type":       agentType,
		"weight":           weight,
		"timestamp":        time.Now().Format(time.RFC3339),
		"status":           "HEALTHY",
		"enabled":          true,
		"performance_data": map[string]interface{}{},
	}
	data, err := json.Marshal(heartbeat)
	require.NoError(t, err)
	err = nc.Publish(topic, data)
	require.NoError(t, err)
}

// TODO: Will be used in Phase 10 E2E tests for map-based signal testing
//
// sendSignalMap sends a trading signal via NATS (as map)
//
//nolint:unused
func sendSignalMap(t *testing.T, nc *nats.Conn, topic, agentName, action string, confidence float64) {
	t.Helper()
	signal := map[string]interface{}{
		"agent_name": agentName,
		"action":     action,
		"confidence": confidence,
		"timestamp":  time.Now().Unix(),
	}
	data, err := json.Marshal(signal)
	require.NoError(t, err)
	err = nc.Publish(topic, data)
	require.NoError(t, err)
}

// sendSignal sends a trading signal via NATS (generic version)
func sendSignal(t *testing.T, nc *nats.Conn, topic string, signal interface{}) {
	t.Helper()
	data, err := json.Marshal(signal)
	require.NoError(t, err)
	err = nc.Publish(topic, data)
	require.NoError(t, err)
}
