package safety

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromFile(t *testing.T) {
	yaml := `
safety:
  max_daily_drawdown: 0.08
  max_position_size: 0.15
  max_total_exposure: 0.60
  max_consecutive_losses: 8
  max_daily_trades: 75
  agent_limits:
    agent1:
      max_daily_drawdown: 0.03
      max_position_size: 0.05
`
	dir := t.TempDir()
	path := filepath.Join(dir, "safety.yml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	lc := NewLimitsConfig()
	require.NoError(t, lc.LoadFromFile(path))

	g := lc.Global()
	assert.Equal(t, 0.08, g.MaxDailyDrawdown)
	assert.Equal(t, 0.15, g.MaxPositionSize)
	assert.Equal(t, 0.60, g.MaxTotalExposure)
	assert.Equal(t, 8, g.MaxConsecutiveLosses)
	assert.Equal(t, 75, g.MaxDailyTrades)

	a := lc.ForAgent("agent1")
	assert.Equal(t, 0.03, a.MaxDailyDrawdown)
	assert.Equal(t, 0.05, a.MaxPositionSize)
}

func TestLoadFromFile_NotFound(t *testing.T) {
	lc := NewLimitsConfig()
	err := lc.LoadFromFile("/nonexistent/path.yml")
	assert.Error(t, err)
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	require.NoError(t, os.WriteFile(path, []byte(":::invalid"), 0644))

	lc := NewLimitsConfig()
	err := lc.LoadFromFile(path)
	assert.Error(t, err)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("SAFETY_MAX_DAILY_DRAWDOWN", "0.12")
	t.Setenv("SAFETY_MAX_POSITION_SIZE", "0.25")
	t.Setenv("SAFETY_MAX_TOTAL_EXPOSURE", "0.70")
	t.Setenv("SAFETY_MAX_CONSECUTIVE_LOSSES", "12")
	t.Setenv("SAFETY_MAX_DAILY_TRADES", "200")

	lc := NewLimitsConfig()
	lc.LoadFromEnv()

	g := lc.Global()
	assert.Equal(t, 0.12, g.MaxDailyDrawdown)
	assert.Equal(t, 0.25, g.MaxPositionSize)
	assert.Equal(t, 0.70, g.MaxTotalExposure)
	assert.Equal(t, 12, g.MaxConsecutiveLosses)
	assert.Equal(t, 200, g.MaxDailyTrades)
}

func TestLoadFromEnv_InvalidValues(t *testing.T) {
	t.Setenv("SAFETY_MAX_DAILY_DRAWDOWN", "notanumber")
	t.Setenv("SAFETY_MAX_CONSECUTIVE_LOSSES", "notanint")

	lc := NewLimitsConfig()
	defaults := lc.Global()
	lc.LoadFromEnv()

	// Should keep defaults when parsing fails
	assert.Equal(t, defaults.MaxDailyDrawdown, lc.Global().MaxDailyDrawdown)
	assert.Equal(t, defaults.MaxConsecutiveLosses, lc.Global().MaxConsecutiveLosses)
}
