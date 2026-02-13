package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

const (
	// DefaultMaxRestartsPerHour is the max number of restarts allowed per hour before giving up
	DefaultMaxRestartsPerHour = 5
	// DefaultHeartbeatTimeout is how long to wait without a heartbeat before considering an agent dead
	DefaultHeartbeatTimeout = 90 * time.Second
	// DefaultMaxBackoff is the maximum backoff duration between restart attempts
	DefaultMaxBackoff = 5 * time.Minute
	// DefaultInitialBackoff is the starting backoff duration
	DefaultInitialBackoff = 1 * time.Second
)

// SupervisorConfig configures the agent supervisor
type SupervisorConfig struct {
	HeartbeatTopic     string        `json:"heartbeat_topic" yaml:"heartbeat_topic"`
	HeartbeatTimeout   time.Duration `json:"heartbeat_timeout" yaml:"heartbeat_timeout"`
	MaxRestartsPerHour int           `json:"max_restarts_per_hour" yaml:"max_restarts_per_hour"`
	InitialBackoff     time.Duration `json:"initial_backoff" yaml:"initial_backoff"`
	MaxBackoff         time.Duration `json:"max_backoff" yaml:"max_backoff"`
	CheckInterval      time.Duration `json:"check_interval" yaml:"check_interval"`
}

// DefaultSupervisorConfig returns sensible defaults
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		HeartbeatTopic:     "agents.heartbeat",
		HeartbeatTimeout:   DefaultHeartbeatTimeout,
		MaxRestartsPerHour: DefaultMaxRestartsPerHour,
		InitialBackoff:     DefaultInitialBackoff,
		MaxBackoff:         DefaultMaxBackoff,
		CheckInterval:      30 * time.Second,
	}
}

// AgentStarter is the interface that the supervisor uses to start/stop agents
type AgentStarter interface {
	StartAgent(ctx context.Context, name string) error
	StopAgent(ctx context.Context, name string) error
}

// SupervisedAgent tracks the state of a supervised agent
type SupervisedAgent struct {
	Name           string
	Type           string
	Healthy        bool
	LastHeartbeat  time.Time
	LastCrashTime  time.Time
	LastCrashError string
	RestartCount   int
	StartedAt      time.Time
	// restartTimes tracks restart timestamps within the last hour for rate limiting
	restartTimes []time.Time
	// currentBackoff is the current backoff duration for exponential backoff
	currentBackoff time.Duration
}

// SupervisorMetrics holds Prometheus metrics for the supervisor
type SupervisorMetrics struct {
	RestartsTotal prometheus.Counter
	UptimeSeconds *prometheus.GaugeVec
	AgentHealthy  *prometheus.GaugeVec
}

// Supervisor monitors agent health and restarts crashed agents
type Supervisor struct {
	config   SupervisorConfig
	starter  AgentStarter
	natsConn *nats.Conn
	natsSub  *nats.Subscription
	log      zerolog.Logger
	metrics  *SupervisorMetrics

	mu     sync.RWMutex
	agents map[string]*SupervisedAgent

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// notifyFn is an optional callback for notifications (e.g., push notifications)
	notifyFn func(title, body string)

	// nowFn for testing time
	nowFn func() time.Time
}

var (
	supervisorMetrics     *SupervisorMetrics
	supervisorMetricsOnce sync.Once
)

func getSupervisorMetrics() *SupervisorMetrics {
	supervisorMetricsOnce.Do(func() {
		supervisorMetrics = &SupervisorMetrics{
			RestartsTotal: promauto.NewCounter(prometheus.CounterOpts{
				Name: "agent_restarts_total",
				Help: "Total number of agent restarts",
			}),
			UptimeSeconds: promauto.NewGaugeVec(prometheus.GaugeOpts{
				Name: "agent_uptime_seconds",
				Help: "Agent uptime in seconds",
			}, []string{"agent"}),
			AgentHealthy: promauto.NewGaugeVec(prometheus.GaugeOpts{
				Name: "agent_healthy",
				Help: "Whether agent is healthy (1) or not (0)",
			}, []string{"agent"}),
		}
	})
	return supervisorMetrics
}

// NewSupervisor creates a new agent supervisor
func NewSupervisor(config SupervisorConfig, starter AgentStarter, natsConn *nats.Conn, log zerolog.Logger) *Supervisor {
	return &Supervisor{
		config:   config,
		starter:  starter,
		natsConn: natsConn,
		log:      log.With().Str("component", "supervisor").Logger(),
		metrics:  getSupervisorMetrics(),
		agents:   make(map[string]*SupervisedAgent),
		nowFn:    time.Now,
	}
}

// SetNotifyFn sets a notification callback
func (s *Supervisor) SetNotifyFn(fn func(title, body string)) {
	s.notifyFn = fn
}

// Register adds an agent to be supervised
func (s *Supervisor) Register(name, agentType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFn()
	s.agents[name] = &SupervisedAgent{
		Name:           name,
		Type:           agentType,
		Healthy:        true,
		LastHeartbeat:  now,
		StartedAt:      now,
		currentBackoff: s.config.InitialBackoff,
	}
	s.metrics.AgentHealthy.WithLabelValues(name).Set(1)
	s.log.Info().Str("agent", name).Str("type", agentType).Msg("Agent registered with supervisor")
}

// Unregister removes an agent from supervision
func (s *Supervisor) Unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, name)
	s.log.Info().Str("agent", name).Msg("Agent unregistered from supervisor")
}

// Start begins the supervisor's monitoring loop
func (s *Supervisor) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Subscribe to heartbeat messages
	if s.natsConn != nil {
		sub, err := s.natsConn.Subscribe(s.config.HeartbeatTopic, s.handleHeartbeat)
		if err != nil {
			return fmt.Errorf("subscribe to heartbeat topic: %w", err)
		}
		s.natsSub = sub
	}

	// Start health check loop
	s.wg.Add(1)
	go s.healthCheckLoop()

	s.log.Info().
		Dur("check_interval", s.config.CheckInterval).
		Dur("heartbeat_timeout", s.config.HeartbeatTimeout).
		Int("max_restarts_per_hour", s.config.MaxRestartsPerHour).
		Msg("Supervisor started")

	return nil
}

// Stop gracefully stops the supervisor
func (s *Supervisor) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.natsSub != nil {
		_ = s.natsSub.Unsubscribe()
	}
	s.wg.Wait()
	s.log.Info().Msg("Supervisor stopped")
}

// handleHeartbeat processes heartbeat messages from agents
func (s *Supervisor) handleHeartbeat(msg *nats.Msg) {
	var hb HeartbeatMessage
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		s.log.Error().Err(err).Msg("Failed to unmarshal heartbeat")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[hb.AgentName]
	if !ok {
		// Unknown agent, auto-register
		agent = &SupervisedAgent{
			Name:           hb.AgentName,
			Type:           hb.AgentType,
			currentBackoff: s.config.InitialBackoff,
		}
		s.agents[hb.AgentName] = agent
	}

	now := s.nowFn()
	agent.LastHeartbeat = now
	agent.Healthy = true
	s.metrics.AgentHealthy.WithLabelValues(hb.AgentName).Set(1)

	// Update uptime
	if !agent.StartedAt.IsZero() {
		s.metrics.UptimeSeconds.WithLabelValues(hb.AgentName).Set(now.Sub(agent.StartedAt).Seconds())
	}
}

// healthCheckLoop periodically checks agent health
func (s *Supervisor) healthCheckLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAgents()
		}
	}
}

// checkAgents checks all supervised agents and restarts unhealthy ones
func (s *Supervisor) checkAgents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFn()

	for name, agent := range s.agents {
		// Check if heartbeat has timed out
		if now.Sub(agent.LastHeartbeat) > s.config.HeartbeatTimeout {
			if agent.Healthy {
				agent.Healthy = false
				agent.LastCrashTime = now
				agent.LastCrashError = "heartbeat timeout"
				s.metrics.AgentHealthy.WithLabelValues(name).Set(0)
				s.log.Warn().
					Str("agent", name).
					Time("last_heartbeat", agent.LastHeartbeat).
					Msg("Agent heartbeat timeout detected")
			}

			// Attempt restart
			s.tryRestart(name, agent, now)
		}

		// Update uptime metric for healthy agents
		if agent.Healthy && !agent.StartedAt.IsZero() {
			s.metrics.UptimeSeconds.WithLabelValues(name).Set(now.Sub(agent.StartedAt).Seconds())
		}
	}
}

// tryRestart attempts to restart an agent with exponential backoff and rate limiting
func (s *Supervisor) tryRestart(name string, agent *SupervisedAgent, now time.Time) {
	// Rate limit: count restarts in the last hour
	cutoff := now.Add(-1 * time.Hour)
	var recentRestarts []time.Time
	for _, t := range agent.restartTimes {
		if t.After(cutoff) {
			recentRestarts = append(recentRestarts, t)
		}
	}
	agent.restartTimes = recentRestarts

	if len(recentRestarts) >= s.config.MaxRestartsPerHour {
		s.log.Error().
			Str("agent", name).
			Int("restarts_in_hour", len(recentRestarts)).
			Msg("Max restarts per hour exceeded, giving up")

		if s.notifyFn != nil {
			s.notifyFn(
				"Agent Supervisor Alert",
				fmt.Sprintf("Agent %s has crashed %d times in the last hour. Automatic restart disabled.", name, len(recentRestarts)),
			)
		}
		return
	}

	// Check backoff: don't restart too soon (skip backoff check on first detection)
	if agent.RestartCount > 0 && !agent.LastCrashTime.IsZero() && now.Sub(agent.LastCrashTime) < agent.currentBackoff {
		s.log.Debug().
			Str("agent", name).
			Dur("backoff", agent.currentBackoff).
			Msg("Waiting for backoff before restart")
		return
	}

	// Attempt restart
	s.log.Info().
		Str("agent", name).
		Int("attempt", agent.RestartCount+1).
		Dur("backoff", agent.currentBackoff).
		Msg("Attempting to restart agent")

	if s.starter != nil {
		if err := s.starter.StartAgent(s.ctx, name); err != nil {
			s.log.Error().Err(err).Str("agent", name).Msg("Failed to restart agent")
			agent.LastCrashError = fmt.Sprintf("restart failed: %v", err)
		} else {
			s.log.Info().Str("agent", name).Msg("Agent restarted successfully")
			agent.Healthy = true
			agent.StartedAt = now
			agent.LastHeartbeat = now
			s.metrics.AgentHealthy.WithLabelValues(name).Set(1)
		}
	}

	// Update restart tracking
	agent.RestartCount++
	agent.restartTimes = append(agent.restartTimes, now)
	s.metrics.RestartsTotal.Inc()

	// Exponential backoff
	agent.currentBackoff *= 2
	if agent.currentBackoff > s.config.MaxBackoff {
		agent.currentBackoff = s.config.MaxBackoff
	}

	if s.notifyFn != nil {
		s.notifyFn(
			"Agent Restarted",
			fmt.Sprintf("Agent %s was restarted (attempt %d). Reason: %s", name, agent.RestartCount, agent.LastCrashError),
		)
	}
}

// GetAgentState returns a copy of the supervised agent state (for monitoring)
func (s *Supervisor) GetAgentState(name string) (*SupervisedAgent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[name]
	if !ok {
		return nil, false
	}
	// Return a copy
	cp := *agent
	return &cp, true
}

// GetAllAgentStates returns a copy of all supervised agent states
func (s *Supervisor) GetAllAgentStates() map[string]*SupervisedAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*SupervisedAgent, len(s.agents))
	for k, v := range s.agents {
		cp := *v
		result[k] = &cp
	}
	return result
}
