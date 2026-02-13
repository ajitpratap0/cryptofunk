package circuitbreaker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.NotNil(t, cfg)
	assert.Len(t, cfg.CircuitBreakers, 4)
	assert.Contains(t, cfg.CircuitBreakers, "exchange")
	assert.Contains(t, cfg.CircuitBreakers, "llm")
	assert.Contains(t, cfg.CircuitBreakers, "database")
	assert.Contains(t, cfg.CircuitBreakers, "coingecko")
}

func TestNewRegistry(t *testing.T) {
	cfg := DefaultConfig()
	r := NewRegistry(cfg)
	require.NotNil(t, r)

	names := r.Names()
	assert.Len(t, names, 4)

	assert.NotNil(t, r.Get("exchange"))
	assert.NotNil(t, r.Get("llm"))
	assert.NotNil(t, r.Get("database"))
	assert.NotNil(t, r.Get("coingecko"))
	assert.Nil(t, r.Get("nonexistent"))
}

func TestRegistry_Execute(t *testing.T) {
	r := NewRegistry(DefaultConfig())

	// Successful execution
	result, err := r.Execute("exchange", func() (interface{}, error) {
		return "ok", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "ok", result)

	// Pass-through for unknown breaker
	result, err = r.Execute("unknown", func() (interface{}, error) {
		return "passthrough", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "passthrough", result)
}

func TestRegistry_Settings(t *testing.T) {
	r := NewRegistry(DefaultConfig())

	s, ok := r.Settings("exchange")
	assert.True(t, ok)
	assert.Equal(t, 5, s.MaxFailures)
	assert.Equal(t, "60s", s.Timeout)

	_, ok = r.Settings("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_CircuitTrips(t *testing.T) {
	cfg := &Config{
		CircuitBreakers: map[string]BreakerSettings{
			"test": {
				MaxFailures:      3,
				FailureRatio:     0.5,
				Timeout:          "1s",
				HalfOpenRequests: 1,
				CountInterval:    "10s",
			},
		},
	}
	r := NewRegistry(cfg)

	errFail := errors.New("fail")

	// Generate enough failures to trip
	for i := 0; i < 5; i++ {
		_, _ = r.Execute("test", func() (interface{}, error) {
			return nil, errFail
		})
	}

	// Circuit should be open now
	cb := r.Get("test")
	require.NotNil(t, cb)
	// State should be open after enough failures
	assert.NotEqual(t, "closed", cb.State().String(), "circuit should have tripped")
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestFindAndLoadConfig_Defaults(t *testing.T) {
	// In test context, config file may not be at expected paths - should return defaults
	cfg, err := FindAndLoadConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.CircuitBreakers)
}

func TestParseDuration(t *testing.T) {
	assert.Equal(t, 30*1e9, float64(parseDuration("30s", 0)))
	assert.Equal(t, 5*1e9, float64(parseDuration("", 5e9)))
	assert.Equal(t, 5*1e9, float64(parseDuration("invalid", 5e9)))
}
