package safety

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

// Limits holds all configurable safety limits.
type Limits struct {
	MaxDailyDrawdown     float64 `json:"max_daily_drawdown" yaml:"max_daily_drawdown"`
	MaxPositionSize      float64 `json:"max_position_size" yaml:"max_position_size"`
	MaxTotalExposure     float64 `json:"max_total_exposure" yaml:"max_total_exposure"`
	MaxConsecutiveLosses int     `json:"max_consecutive_losses" yaml:"max_consecutive_losses"`
	MaxDailyTrades       int     `json:"max_daily_trades" yaml:"max_daily_trades"`
}

// DefaultLimits returns the default safety limits.
func DefaultLimits() Limits {
	return Limits{
		MaxDailyDrawdown:     0.05,
		MaxPositionSize:      0.10,
		MaxTotalExposure:     0.50,
		MaxConsecutiveLosses: 5,
		MaxDailyTrades:       50,
	}
}

// LimitsConfig holds global and per-agent limits.
type LimitsConfig struct {
	mu          sync.RWMutex
	global      Limits
	agentLimits map[string]Limits
}

// NewLimitsConfig creates a new LimitsConfig with defaults.
func NewLimitsConfig() *LimitsConfig {
	return &LimitsConfig{
		global:      DefaultLimits(),
		agentLimits: make(map[string]Limits),
	}
}

// Global returns the current global limits.
func (lc *LimitsConfig) Global() Limits {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.global
}

// SetGlobal updates the global limits at runtime.
func (lc *LimitsConfig) SetGlobal(l Limits) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.global = l
}

// ForAgent returns limits for a specific agent, falling back to global.
func (lc *LimitsConfig) ForAgent(agent string) Limits {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	if l, ok := lc.agentLimits[agent]; ok {
		return l
	}
	return lc.global
}

// SetAgentLimits sets per-agent limits.
func (lc *LimitsConfig) SetAgentLimits(agent string, l Limits) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.agentLimits[agent] = l
}

// safetyYAML mirrors the YAML config structure.
type safetyYAML struct {
	Safety struct {
		MaxDailyDrawdown     float64           `yaml:"max_daily_drawdown"`
		MaxPositionSize      float64           `yaml:"max_position_size"`
		MaxTotalExposure     float64           `yaml:"max_total_exposure"`
		MaxConsecutiveLosses int               `yaml:"max_consecutive_losses"`
		MaxDailyTrades       int               `yaml:"max_daily_trades"`
		AgentLimits          map[string]Limits `yaml:"agent_limits"`
	} `yaml:"safety"`
}

// LoadFromFile loads limits from a YAML config file.
func (lc *LimitsConfig) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read safety config: %w", err)
	}

	var cfg safetyYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse safety config: %w", err)
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	s := cfg.Safety
	if s.MaxDailyDrawdown > 0 {
		lc.global.MaxDailyDrawdown = s.MaxDailyDrawdown
	}
	if s.MaxPositionSize > 0 {
		lc.global.MaxPositionSize = s.MaxPositionSize
	}
	if s.MaxTotalExposure > 0 {
		lc.global.MaxTotalExposure = s.MaxTotalExposure
	}
	if s.MaxConsecutiveLosses > 0 {
		lc.global.MaxConsecutiveLosses = s.MaxConsecutiveLosses
	}
	if s.MaxDailyTrades > 0 {
		lc.global.MaxDailyTrades = s.MaxDailyTrades
	}
	for agent, limits := range s.AgentLimits {
		lc.agentLimits[agent] = limits
	}

	return nil
}

// LoadFromEnv overrides limits from environment variables.
func (lc *LimitsConfig) LoadFromEnv() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if v := os.Getenv("SAFETY_MAX_DAILY_DRAWDOWN"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			lc.global.MaxDailyDrawdown = f
		}
	}
	if v := os.Getenv("SAFETY_MAX_POSITION_SIZE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			lc.global.MaxPositionSize = f
		}
	}
	if v := os.Getenv("SAFETY_MAX_TOTAL_EXPOSURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			lc.global.MaxTotalExposure = f
		}
	}
	if v := os.Getenv("SAFETY_MAX_CONSECUTIVE_LOSSES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			lc.global.MaxConsecutiveLosses = i
		}
	}
	if v := os.Getenv("SAFETY_MAX_DAILY_TRADES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			lc.global.MaxDailyTrades = i
		}
	}
}
