package backtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateJob tests the validateJob method without a DB connection.
func TestValidateJob(t *testing.T) {
	m := &JobManager{} // nil db is fine — validateJob doesn't touch the DB

	validJob := func() *BacktestJob {
		return &BacktestJob{
			Name:           "test",
			StartDate:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:        time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			Symbols:        []string{"BTCUSDT"},
			InitialCapital: 10000.0,
			StrategyConfig: map[string]interface{}{"type": "trend"},
		}
	}

	t.Run("valid job passes", func(t *testing.T) {
		require.NoError(t, m.validateJob(validJob()))
	})

	t.Run("empty name", func(t *testing.T) {
		j := validJob()
		j.Name = ""
		assert.Error(t, m.validateJob(j))
	})

	t.Run("end before start", func(t *testing.T) {
		j := validJob()
		j.EndDate = j.StartDate.Add(-time.Hour)
		assert.Error(t, m.validateJob(j))
	})

	t.Run("equal start and end", func(t *testing.T) {
		j := validJob()
		j.EndDate = j.StartDate
		assert.Error(t, m.validateJob(j))
	})

	t.Run("no symbols", func(t *testing.T) {
		j := validJob()
		j.Symbols = nil
		assert.Error(t, m.validateJob(j))
	})

	t.Run("zero capital", func(t *testing.T) {
		j := validJob()
		j.InitialCapital = 0
		assert.Error(t, m.validateJob(j))
	})

	t.Run("negative capital", func(t *testing.T) {
		j := validJob()
		j.InitialCapital = -1
		assert.Error(t, m.validateJob(j))
	})

	t.Run("empty strategy config", func(t *testing.T) {
		j := validJob()
		j.StrategyConfig = nil
		assert.Error(t, m.validateJob(j))
	})
}

func TestGetValue(t *testing.T) {
	v := 3.14
	assert.Equal(t, 3.14, getValue(&v))
	assert.Equal(t, 0.0, getValue(nil))
}

func TestGetIntValue(t *testing.T) {
	n := 42
	assert.Equal(t, 42, getIntValue(&n))
	assert.Equal(t, 0, getIntValue(nil))
}
