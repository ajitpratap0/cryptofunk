// Package circuitbreaker provides a centralized circuit breaker registry
// that loads configuration from configs/circuit_breakers.yaml.
package circuitbreaker

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"gopkg.in/yaml.v3"
)

// BreakerSettings holds configuration for a single circuit breaker.
type BreakerSettings struct {
	MaxFailures      int     `yaml:"max_failures"`
	FailureRatio     float64 `yaml:"failure_ratio"`
	Timeout          string  `yaml:"timeout"`
	HalfOpenRequests int     `yaml:"half_open_requests"`
	CountInterval    string  `yaml:"count_interval"`
}

// Config is the top-level circuit breaker configuration file structure.
type Config struct {
	CircuitBreakers map[string]BreakerSettings `yaml:"circuit_breakers"`
}

// Registry holds named circuit breakers loaded from config.
type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*gobreaker.CircuitBreaker
	config   Config
}

// DefaultConfigPaths returns the standard search paths for the config file.
func DefaultConfigPaths() []string {
	return []string{
		"configs/circuit_breakers.yaml",
		"../../configs/circuit_breakers.yaml",
		"../../../configs/circuit_breakers.yaml",
	}
}

// LoadConfig reads and parses a circuit breaker config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- config path from trusted source
	if err != nil {
		return nil, fmt.Errorf("read circuit breaker config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse circuit breaker config: %w", err)
	}
	return &cfg, nil
}

// FindAndLoadConfig searches default paths for the config file.
func FindAndLoadConfig() (*Config, error) {
	for _, p := range DefaultConfigPaths() {
		cfg, err := LoadConfig(p)
		if err == nil {
			return cfg, nil
		}
	}
	// Return defaults if no config file found
	return DefaultConfig(), nil
}

// DefaultConfig returns hardcoded defaults matching configs/circuit_breakers.yaml.
func DefaultConfig() *Config {
	return &Config{
		CircuitBreakers: map[string]BreakerSettings{
			"exchange": {
				MaxFailures:      5,
				FailureRatio:     0.6,
				Timeout:          "60s",
				HalfOpenRequests: 2,
				CountInterval:    "10s",
			},
			"llm": {
				MaxFailures:      3,
				FailureRatio:     0.6,
				Timeout:          "30s",
				HalfOpenRequests: 2,
				CountInterval:    "10s",
			},
			"database": {
				MaxFailures:      5,
				FailureRatio:     0.6,
				Timeout:          "15s",
				HalfOpenRequests: 5,
				CountInterval:    "10s",
			},
			"coingecko": {
				MaxFailures:      5,
				FailureRatio:     0.6,
				Timeout:          "30s",
				HalfOpenRequests: 2,
				CountInterval:    "10s",
			},
		},
	}
}

// NewRegistry creates a registry from config, building all named breakers.
func NewRegistry(cfg *Config) *Registry {
	r := &Registry{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		config:   *cfg,
	}
	for name, settings := range cfg.CircuitBreakers {
		r.breakers[name] = newBreaker(name, settings)
	}
	return r
}

// NewRegistryFromFile loads config and creates a registry.
func NewRegistryFromFile(path string) (*Registry, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return NewRegistry(cfg), nil
}

// NewDefaultRegistry auto-discovers config and creates a registry.
func NewDefaultRegistry() *Registry {
	cfg, _ := FindAndLoadConfig()
	return NewRegistry(cfg)
}

// Get returns a named circuit breaker. Returns nil if not found.
func (r *Registry) Get(name string) *gobreaker.CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.breakers[name]
}

// Execute runs fn through the named circuit breaker.
func (r *Registry) Execute(name string, fn func() (interface{}, error)) (interface{}, error) {
	cb := r.Get(name)
	if cb == nil {
		return fn() // no breaker configured, pass through
	}
	return cb.Execute(fn)
}

// Settings returns the config for a named breaker.
func (r *Registry) Settings(name string) (BreakerSettings, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.config.CircuitBreakers[name]
	return s, ok
}

// Names returns all configured breaker names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.breakers))
	for n := range r.breakers {
		names = append(names, n)
	}
	return names
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func newBreaker(name string, s BreakerSettings) *gobreaker.CircuitBreaker {
	timeout := parseDuration(s.Timeout, 30*time.Second)
	interval := parseDuration(s.CountInterval, 10*time.Second)
	minReqs := uint32(5)
	if s.MaxFailures > 0 && s.MaxFailures <= int(^uint32(0)) {
		minReqs = uint32(s.MaxFailures) //nolint:gosec // G115 - bounds checked above
	}
	halfOpen := uint32(2)
	if s.HalfOpenRequests > 0 && s.HalfOpenRequests <= int(^uint32(0)) {
		halfOpen = uint32(s.HalfOpenRequests) //nolint:gosec // G115 - bounds checked above
	}
	ratio := s.FailureRatio
	if ratio <= 0 {
		ratio = 0.6
	}

	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: halfOpen,
		Interval:    interval,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < minReqs {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= ratio
		},
	})
}
