package agents

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockStarter implements AgentStarter for testing
type mockStarter struct {
	startCalls []string
	stopCalls  []string
	startErr   error
}

func (m *mockStarter) StartAgent(_ context.Context, name string) error {
	m.startCalls = append(m.startCalls, name)
	return m.startErr
}

func (m *mockStarter) StopAgent(_ context.Context, name string) error {
	m.stopCalls = append(m.stopCalls, name)
	return nil
}

func newTestSupervisor(starter AgentStarter) *Supervisor {
	config := SupervisorConfig{
		HeartbeatTopic:     "test.heartbeat",
		HeartbeatTimeout:   5 * time.Second,
		MaxRestartsPerHour: 3,
		InitialBackoff:     100 * time.Millisecond,
		MaxBackoff:         1 * time.Second,
		CheckInterval:      1 * time.Second,
	}
	log := zerolog.Nop()
	s := NewSupervisor(config, starter, nil, log)
	return s
}

func TestSupervisor_Register(t *testing.T) {
	s := newTestSupervisor(nil)
	s.Register("test-agent", "technical")

	state, ok := s.GetAgentState("test-agent")
	if !ok {
		t.Fatal("agent not found after registration")
	}
	if state.Name != "test-agent" {
		t.Fatalf("expected name test-agent, got %s", state.Name)
	}
	if !state.Healthy {
		t.Fatal("expected agent to be healthy after registration")
	}
}

func TestSupervisor_Unregister(t *testing.T) {
	s := newTestSupervisor(nil)
	s.Register("test-agent", "technical")
	s.Unregister("test-agent")

	_, ok := s.GetAgentState("test-agent")
	if ok {
		t.Fatal("agent should not exist after unregister")
	}
}

func TestSupervisor_HeartbeatTimeout(t *testing.T) {
	starter := &mockStarter{}
	s := newTestSupervisor(starter)

	now := time.Now()
	s.nowFn = func() time.Time { return now }
	s.Register("test-agent", "technical")

	// Advance time past heartbeat timeout
	s.nowFn = func() time.Time { return now.Add(10 * time.Second) }
	s.checkAgents()

	state, _ := s.GetAgentState("test-agent")
	if len(starter.startCalls) != 1 {
		t.Fatalf("expected 1 restart call, got %d", len(starter.startCalls))
	}
	if state.RestartCount != 1 {
		t.Fatalf("expected restart count 1, got %d", state.RestartCount)
	}
}

func TestSupervisor_ExponentialBackoff(t *testing.T) {
	starter := &mockStarter{}
	s := newTestSupervisor(starter)

	now := time.Now()
	s.nowFn = func() time.Time { return now }
	s.Register("test-agent", "technical")

	// First timeout + restart
	s.nowFn = func() time.Time { return now.Add(10 * time.Second) }
	s.checkAgents()

	if len(starter.startCalls) != 1 {
		t.Fatalf("expected 1 start call, got %d", len(starter.startCalls))
	}

	// Simulate agent crashing again immediately (heartbeat timeout)
	// But backoff should prevent immediate restart
	s.mu.Lock()
	agent := s.agents["test-agent"]
	agent.Healthy = false
	agent.LastHeartbeat = now // old heartbeat
	agent.LastCrashTime = now.Add(10 * time.Second)
	s.mu.Unlock()

	// Only 50ms later - should NOT restart (backoff is 200ms after doubling from 100ms)
	s.nowFn = func() time.Time { return now.Add(10*time.Second + 50*time.Millisecond) }
	s.checkAgents()

	if len(starter.startCalls) != 1 {
		t.Fatalf("expected still 1 start call (backoff), got %d", len(starter.startCalls))
	}
}

func TestSupervisor_MaxRestartsPerHour(t *testing.T) {
	starter := &mockStarter{}
	s := newTestSupervisor(starter)

	now := time.Now()
	s.nowFn = func() time.Time { return now }
	s.Register("test-agent", "technical")

	// Trigger max restarts
	for i := 0; i < 3; i++ {
		offset := time.Duration(i*20) * time.Second
		s.nowFn = func() time.Time { return now.Add(offset + 10*time.Second) }

		s.mu.Lock()
		agent := s.agents["test-agent"]
		agent.Healthy = false
		agent.LastHeartbeat = now                                                                 // stale
		agent.LastCrashTime = now.Add(offset)                                                     // crash time in the past
		agent.currentBackoff = 1 * time.Millisecond                                               // tiny backoff so restart proceeds
		s.mu.Unlock()

		s.checkAgents()
	}

	if len(starter.startCalls) != 3 {
		t.Fatalf("expected 3 start calls, got %d", len(starter.startCalls))
	}

	// 4th attempt should be blocked by rate limit
	s.nowFn = func() time.Time { return now.Add(70 * time.Second) }
	s.mu.Lock()
	agent := s.agents["test-agent"]
	agent.Healthy = false
	agent.LastHeartbeat = now
	agent.LastCrashTime = now.Add(60 * time.Second)
	agent.currentBackoff = 1 * time.Millisecond
	s.mu.Unlock()

	s.checkAgents()

	if len(starter.startCalls) != 3 {
		t.Fatalf("expected still 3 start calls (rate limited), got %d", len(starter.startCalls))
	}
}

func TestSupervisor_NotifyOnRestart(t *testing.T) {
	starter := &mockStarter{}
	s := newTestSupervisor(starter)

	var notifications []string
	s.SetNotifyFn(func(title, body string) {
		notifications = append(notifications, title+": "+body)
	})

	now := time.Now()
	s.nowFn = func() time.Time { return now }
	s.Register("test-agent", "technical")

	s.nowFn = func() time.Time { return now.Add(10 * time.Second) }
	s.checkAgents()

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
}

func TestSupervisor_GetAllAgentStates(t *testing.T) {
	s := newTestSupervisor(nil)
	s.Register("agent-1", "technical")
	s.Register("agent-2", "sentiment")

	states := s.GetAllAgentStates()
	if len(states) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(states))
	}
}
