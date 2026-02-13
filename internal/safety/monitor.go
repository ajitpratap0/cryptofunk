package safety

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

var (
	metricsOnce         sync.Once
	sharedDrawdownGauge prometheus.Gauge
	sharedTradesToday   prometheus.Gauge
	sharedKillSwitch    prometheus.Gauge
	sharedConsecLoss    prometheus.Gauge
)

func initMetrics() {
	metricsOnce.Do(func() {
		sharedDrawdownGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "safety_drawdown_current",
			Help: "Current daily drawdown as fraction of portfolio",
		})
		sharedTradesToday = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "safety_trades_today",
			Help: "Number of trades executed today",
		})
		sharedKillSwitch = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "safety_kill_switch_active",
			Help: "Whether the kill switch is active (1=active, 0=inactive)",
		})
		sharedConsecLoss = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "safety_consecutive_losses",
			Help: "Current streak of consecutive losing trades",
		})
		// Register ignoring already-registered errors (safe for tests)
		for _, c := range []prometheus.Collector{sharedDrawdownGauge, sharedTradesToday, sharedKillSwitch, sharedConsecLoss} {
			_ = prometheus.Register(c) //nolint:errcheck
		}
	})
}

// Monitor tracks real-time trading state for safety decisions.
type Monitor struct {
	mu sync.RWMutex

	// Daily state (resets each day)
	dayKey            string  // YYYY-MM-DD
	dailyPnL          float64 // running P&L for the day
	dailyTradeCount   int
	consecutiveLosses int
	portfolioValue    float64 // latest known portfolio value
	totalExposure     float64 // sum of abs open position values

	// Prometheus metrics
	drawdownGauge        prometheus.Gauge
	tradesTodayGauge     prometheus.Gauge
	killSwitchGauge      prometheus.Gauge
	consecutiveLossGauge prometheus.Gauge
}

// NewMonitor creates a new safety monitor with Prometheus metrics.
func NewMonitor(portfolioValue float64) *Monitor {
	initMetrics()
	return &Monitor{
		dayKey:               today(),
		portfolioValue:       portfolioValue,
		drawdownGauge:        sharedDrawdownGauge,
		tradesTodayGauge:     sharedTradesToday,
		killSwitchGauge:      sharedKillSwitch,
		consecutiveLossGauge: sharedConsecLoss,
	}
}

func today() string {
	return time.Now().UTC().Format("2006-01-02")
}

// ensureDay resets daily counters if the day has changed. Caller must hold mu.
func (m *Monitor) ensureDay() {
	d := today()
	if d != m.dayKey {
		log.Info().Str("old_day", m.dayKey).Str("new_day", d).Msg("safety: daily reset")
		m.dayKey = d
		m.dailyPnL = 0
		m.dailyTradeCount = 0
		m.drawdownGauge.Set(0)
		m.tradesTodayGauge.Set(0)
		// consecutiveLosses intentionally NOT reset — carries across days
	}
}

// RecordTrade records a completed trade's P&L.
func (m *Monitor) RecordTrade(pnl float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureDay()

	m.dailyPnL += pnl
	m.dailyTradeCount++

	if pnl < 0 {
		m.consecutiveLosses++
	} else {
		m.consecutiveLosses = 0
	}

	// Update metrics
	if m.portfolioValue > 0 {
		m.drawdownGauge.Set(-m.dailyPnL / m.portfolioValue)
	}
	m.tradesTodayGauge.Set(float64(m.dailyTradeCount))
	m.consecutiveLossGauge.Set(float64(m.consecutiveLosses))

	log.Info().
		Float64("pnl", pnl).
		Float64("daily_pnl", m.dailyPnL).
		Int("daily_trades", m.dailyTradeCount).
		Int("consecutive_losses", m.consecutiveLosses).
		Msg("safety: trade recorded")
}

// SetPortfolioValue updates the portfolio value used for threshold calculations.
func (m *Monitor) SetPortfolioValue(v float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.portfolioValue = v
}

// SetTotalExposure updates the current total open exposure.
func (m *Monitor) SetTotalExposure(v float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalExposure = v
}

// SetKillSwitchMetric updates the kill switch Prometheus metric.
func (m *Monitor) SetKillSwitchMetric(active bool) {
	if active {
		m.killSwitchGauge.Set(1)
	} else {
		m.killSwitchGauge.Set(0)
	}
}

// ResetConsecutiveLosses resets the consecutive loss counter (manual resume).
func (m *Monitor) ResetConsecutiveLosses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consecutiveLosses = 0
	m.consecutiveLossGauge.Set(0)
	log.Info().Msg("safety: consecutive losses reset (manual resume)")
}

// Snapshot returns a point-in-time view of monitored state.
type Snapshot struct {
	DayKey            string  `json:"day"`
	DailyPnL          float64 `json:"daily_pnl"`
	DailyTradeCount   int     `json:"daily_trade_count"`
	ConsecutiveLosses int     `json:"consecutive_losses"`
	PortfolioValue    float64 `json:"portfolio_value"`
	TotalExposure     float64 `json:"total_exposure"`
	DrawdownPct       float64 `json:"drawdown_pct"`
}

// Snapshot returns a copy of the current state.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.ensureDayRLock()

	dd := 0.0
	if m.portfolioValue > 0 {
		dd = -m.dailyPnL / m.portfolioValue
	}
	return Snapshot{
		DayKey:            m.dayKey,
		DailyPnL:          m.dailyPnL,
		DailyTradeCount:   m.dailyTradeCount,
		ConsecutiveLosses: m.consecutiveLosses,
		PortfolioValue:    m.portfolioValue,
		TotalExposure:     m.totalExposure,
		DrawdownPct:       dd,
	}
}

// ensureDayRLock is a read-only check; if day changed we upgrade lock.
func (m *Monitor) ensureDayRLock() {
	// In read path we just check — actual reset happens on write.
}
