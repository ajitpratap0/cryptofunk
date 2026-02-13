package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig_GetDSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "mydb",
		SSLMode:  "disable",
	}
	dsn := cfg.GetDSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=postgres")
	assert.Contains(t, dsn, "dbname=mydb")
}

func TestRedisConfig_GetRedisAddr(t *testing.T) {
	cfg := RedisConfig{Host: "redis.example.com", Port: 6379}
	assert.Equal(t, "redis.example.com:6379", cfg.GetRedisAddr())
}

func TestAPIConfig_GetAPIAddr(t *testing.T) {
	cfg := APIConfig{Host: "0.0.0.0", Port: 8080}
	assert.Equal(t, "0.0.0.0:8080", cfg.GetAPIAddr())
}

func TestAPIConfig_GetOrchestratorURL(t *testing.T) {
	cfg := APIConfig{OrchestratorURL: "http://orch:9090"}
	assert.Equal(t, "http://orch:9090", cfg.GetOrchestratorURL())
}

func TestLLMConfig_GetTimeout(t *testing.T) {
	cfg := LLMConfig{Timeout: 5000}
	assert.Equal(t, 5*time.Second, cfg.GetTimeout())
}

func TestCircuitBreakerSettings_ParseDuration(t *testing.T) {
	s := &CircuitBreakerSettings{}
	d, err := s.ParseDuration("5m")
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)

	d, err = s.ParseDuration("")
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), d)

	_, err = s.ParseDuration("invalid")
	assert.Error(t, err)
}
