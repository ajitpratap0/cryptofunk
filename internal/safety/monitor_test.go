package safety

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonitor_SetPortfolioValue(t *testing.T) {
	m := NewMonitor(1000)
	m.SetPortfolioValue(5000)
	snap := m.Snapshot()
	assert.Equal(t, 5000.0, snap.PortfolioValue)
}

func TestMonitor_EnsureDayRLock(t *testing.T) {
	m := NewMonitor(1000)
	// ensureDayRLock is a no-op but should not panic
	snap := m.Snapshot()
	assert.NotEmpty(t, snap.DayKey)
}

func TestMonitor_RecordTrade_Drawdown(t *testing.T) {
	m := NewMonitor(10000)
	m.RecordTrade(-500) // loss
	snap := m.Snapshot()
	assert.Equal(t, -500.0, snap.DailyPnL)
	assert.Equal(t, 1, snap.DailyTradeCount)
	assert.Equal(t, 1, snap.ConsecutiveLosses)
	assert.InDelta(t, 0.05, snap.DrawdownPct, 0.001)
}

func TestMonitor_ResetConsecutiveLosses(t *testing.T) {
	m := NewMonitor(10000)
	m.RecordTrade(-100)
	m.RecordTrade(-100)
	assert.Equal(t, 2, m.Snapshot().ConsecutiveLosses)
	m.ResetConsecutiveLosses()
	assert.Equal(t, 0, m.Snapshot().ConsecutiveLosses)
}

func TestMonitor_SetTotalExposure(t *testing.T) {
	m := NewMonitor(10000)
	m.SetTotalExposure(3000)
	assert.Equal(t, 3000.0, m.Snapshot().TotalExposure)
}

func TestMonitor_SetKillSwitchMetric(t *testing.T) {
	m := NewMonitor(10000)
	// Should not panic
	m.SetKillSwitchMetric(true)
	m.SetKillSwitchMetric(false)
}

func TestMonitor_WinResetsConsecutiveLosses(t *testing.T) {
	m := NewMonitor(10000)
	m.RecordTrade(-100)
	m.RecordTrade(-100)
	assert.Equal(t, 2, m.Snapshot().ConsecutiveLosses)
	m.RecordTrade(200) // win
	assert.Equal(t, 0, m.Snapshot().ConsecutiveLosses)
}

func TestMonitor_SnapshotDrawdown(t *testing.T) {
	m := NewMonitor(0) // zero portfolio
	snap := m.Snapshot()
	assert.Equal(t, 0.0, snap.DrawdownPct)
}
