package safety

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/exchange"
)

// RejectionReason describes why an order was rejected.
type RejectionReason string

const (
	ReasonKillSwitch        RejectionReason = "kill_switch_active"
	ReasonDailyDrawdown     RejectionReason = "max_daily_drawdown_exceeded"
	ReasonPositionSize      RejectionReason = "max_position_size_exceeded"
	ReasonTotalExposure     RejectionReason = "max_total_exposure_exceeded"
	ReasonConsecutiveLosses RejectionReason = "max_consecutive_losses_reached"
	ReasonDailyTrades       RejectionReason = "max_daily_trades_reached"
)

// ErrOrderRejected is returned when a safety check blocks an order.
type ErrOrderRejected struct {
	Reason  RejectionReason
	Message string
}

func (e *ErrOrderRejected) Error() string {
	return fmt.Sprintf("order rejected [%s]: %s", e.Reason, e.Message)
}

// Guard is the central safety guard that validates orders before execution.
type Guard struct {
	mu         sync.RWMutex
	limits     *LimitsConfig
	monitor    *Monitor
	killSwitch bool
}

// NewGuard creates a new safety guard.
func NewGuard(limits *LimitsConfig, monitor *Monitor) *Guard {
	return &Guard{
		limits:  limits,
		monitor: monitor,
	}
}

// KillSwitchEnabled returns whether the kill switch is active.
func (g *Guard) KillSwitchEnabled() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.killSwitch
}

// EnableKillSwitch activates the kill switch.
func (g *Guard) EnableKillSwitch() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.killSwitch = true
	g.monitor.SetKillSwitchMetric(true)
	log.Warn().Msg("safety: KILL SWITCH ENABLED — all trading halted")
}

// DisableKillSwitch deactivates the kill switch.
func (g *Guard) DisableKillSwitch() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.killSwitch = false
	g.monitor.SetKillSwitchMetric(false)
	log.Warn().Msg("safety: kill switch disabled — trading resumed")
}

// CheckOrder validates an order against all safety rules.
// agent is optional (empty string uses global limits).
// orderValue is the notional value of the order (price * quantity).
func (g *Guard) CheckOrder(_ context.Context, req exchange.PlaceOrderRequest, orderValue float64, agent string) error {
	g.mu.RLock()
	ks := g.killSwitch
	g.mu.RUnlock()

	// 1. Kill switch
	if ks {
		log.Warn().Str("symbol", req.Symbol).Msg("safety: order blocked by kill switch")
		return &ErrOrderRejected{Reason: ReasonKillSwitch, Message: "kill switch is active"}
	}

	var lim Limits
	if agent != "" {
		lim = g.limits.ForAgent(agent)
	} else {
		lim = g.limits.Global()
	}

	snap := g.monitor.Snapshot()

	// 2. Daily drawdown
	if snap.PortfolioValue > 0 && snap.DrawdownPct >= lim.MaxDailyDrawdown {
		log.Warn().
			Float64("drawdown", snap.DrawdownPct).
			Float64("limit", lim.MaxDailyDrawdown).
			Msg("safety: order blocked by daily drawdown")
		return &ErrOrderRejected{
			Reason:  ReasonDailyDrawdown,
			Message: fmt.Sprintf("daily drawdown %.2f%% >= limit %.2f%%", snap.DrawdownPct*100, lim.MaxDailyDrawdown*100),
		}
	}

	// 3. Position size
	if snap.PortfolioValue > 0 {
		maxValue := snap.PortfolioValue * lim.MaxPositionSize
		if orderValue > maxValue {
			log.Warn().
				Float64("order_value", orderValue).
				Float64("max_value", maxValue).
				Msg("safety: order blocked by position size")
			return &ErrOrderRejected{
				Reason:  ReasonPositionSize,
				Message: fmt.Sprintf("order value %.2f > max %.2f (%.0f%% of portfolio)", orderValue, maxValue, lim.MaxPositionSize*100),
			}
		}
	}

	// 4. Total exposure
	if snap.PortfolioValue > 0 {
		maxExposure := snap.PortfolioValue * lim.MaxTotalExposure
		if snap.TotalExposure+orderValue > maxExposure {
			log.Warn().
				Float64("current_exposure", snap.TotalExposure).
				Float64("order_value", orderValue).
				Float64("max_exposure", maxExposure).
				Msg("safety: order blocked by total exposure")
			return &ErrOrderRejected{
				Reason:  ReasonTotalExposure,
				Message: fmt.Sprintf("total exposure %.2f + order %.2f > max %.2f", snap.TotalExposure, orderValue, maxExposure),
			}
		}
	}

	// 5. Consecutive losses
	if snap.ConsecutiveLosses >= lim.MaxConsecutiveLosses {
		log.Warn().
			Int("consecutive_losses", snap.ConsecutiveLosses).
			Int("limit", lim.MaxConsecutiveLosses).
			Msg("safety: order blocked by consecutive losses")
		return &ErrOrderRejected{
			Reason:  ReasonConsecutiveLosses,
			Message: fmt.Sprintf("%d consecutive losses >= limit %d (manual resume required)", snap.ConsecutiveLosses, lim.MaxConsecutiveLosses),
		}
	}

	// 6. Daily trade count
	if snap.DailyTradeCount >= lim.MaxDailyTrades {
		log.Warn().
			Int("trades_today", snap.DailyTradeCount).
			Int("limit", lim.MaxDailyTrades).
			Msg("safety: order blocked by daily trade limit")
		return &ErrOrderRejected{
			Reason:  ReasonDailyTrades,
			Message: fmt.Sprintf("%d trades today >= limit %d", snap.DailyTradeCount, lim.MaxDailyTrades),
		}
	}

	return nil
}

// Status returns the current safety status for API reporting.
type Status struct {
	KillSwitchActive bool     `json:"kill_switch_active"`
	Limits           Limits   `json:"limits"`
	State            Snapshot `json:"state"`
}

// Status returns the current safety status.
func (g *Guard) Status() Status {
	g.mu.RLock()
	ks := g.killSwitch
	g.mu.RUnlock()

	return Status{
		KillSwitchActive: ks,
		Limits:           g.limits.Global(),
		State:            g.monitor.Snapshot(),
	}
}

// RecordTrade delegates to the monitor.
func (g *Guard) RecordTrade(pnl float64) {
	g.monitor.RecordTrade(pnl)
}

// ResetConsecutiveLosses delegates to the monitor (manual resume).
func (g *Guard) ResetConsecutiveLosses() {
	g.monitor.ResetConsecutiveLosses()
}

// SetPortfolioValue delegates to the monitor.
func (g *Guard) SetPortfolioValue(v float64) {
	g.monitor.SetPortfolioValue(v)
}

// SetTotalExposure delegates to the monitor.
func (g *Guard) SetTotalExposure(v float64) {
	g.monitor.SetTotalExposure(v)
}

// LimitsConfig returns the underlying limits config for runtime adjustment.
func (g *Guard) LimitsConfig() *LimitsConfig {
	return g.limits
}
