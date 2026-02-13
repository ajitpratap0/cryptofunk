package agents

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestPauseAndGetters(t *testing.T) {
	log := zerolog.New(os.Stdout)
	config := &AgentConfig{Name: "pause-test", Type: "strategy", Version: "2.0.0"}
	agent := NewBaseAgent(config, log, 0)

	// Test getters
	assert.Equal(t, "pause-test", agent.GetName())
	assert.Equal(t, "strategy", agent.GetType())
	assert.Equal(t, "2.0.0", agent.GetVersion())
	assert.Equal(t, config, agent.GetConfig())

	// Test pause
	assert.False(t, agent.IsPaused())
	assert.False(t, agent.CheckPausedAndSkip())

	agent.pausedMutex.Lock()
	agent.paused = true
	agent.pausedMutex.Unlock()

	assert.True(t, agent.IsPaused())
	assert.True(t, agent.CheckPausedAndSkip())
}
