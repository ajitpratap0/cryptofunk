package safety

import (
	"context"
	"testing"

	"github.com/ajitpratap0/cryptofunk/internal/exchange"
)

func newTestGuard(portfolio float64) *Guard {
	lc := NewLimitsConfig()
	mon := NewMonitor(portfolio)
	return NewGuard(lc, mon)
}

func makeOrder(symbol string) exchange.PlaceOrderRequest {
	return exchange.PlaceOrderRequest{
		Symbol:   symbol,
		Side:     exchange.OrderSideBuy,
		Type:     exchange.OrderTypeMarket,
		Quantity: 1.0,
	}
}

func TestKillSwitch(t *testing.T) {
	g := newTestGuard(10000)

	// Should pass normally
	if err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 500, ""); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Enable kill switch
	g.EnableKillSwitch()
	if !g.KillSwitchEnabled() {
		t.Fatal("kill switch should be enabled")
	}

	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 500, "")
	if err == nil {
		t.Fatal("expected error with kill switch on")
	}
	rejErr, ok := err.(*ErrOrderRejected)
	if !ok {
		t.Fatalf("expected ErrOrderRejected, got %T", err)
	}
	if rejErr.Reason != ReasonKillSwitch {
		t.Fatalf("expected kill switch reason, got %s", rejErr.Reason)
	}

	// Disable
	g.DisableKillSwitch()
	if err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 500, ""); err != nil {
		t.Fatalf("expected no error after disabling kill switch, got %v", err)
	}
}

func TestMaxDailyDrawdown(t *testing.T) {
	g := newTestGuard(10000)

	// Record losses totalling 5% of portfolio
	g.RecordTrade(-500) // 5% drawdown

	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 100, "")
	if err == nil {
		t.Fatal("expected drawdown rejection")
	}
	rejErr := err.(*ErrOrderRejected)
	if rejErr.Reason != ReasonDailyDrawdown {
		t.Fatalf("expected drawdown reason, got %s", rejErr.Reason)
	}
}

func TestMaxPositionSize(t *testing.T) {
	g := newTestGuard(10000)

	// 10% of 10000 = 1000, try 1500
	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 1500, "")
	if err == nil {
		t.Fatal("expected position size rejection")
	}
	rejErr := err.(*ErrOrderRejected)
	if rejErr.Reason != ReasonPositionSize {
		t.Fatalf("expected position size reason, got %s", rejErr.Reason)
	}

	// 900 should pass
	if err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 900, ""); err != nil {
		t.Fatalf("expected no error for small order, got %v", err)
	}
}

func TestMaxTotalExposure(t *testing.T) {
	g := newTestGuard(10000)

	// Set existing exposure to 4500, order of 600 -> 5100 > 5000 (50%)
	g.SetTotalExposure(4500)

	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 600, "")
	if err == nil {
		t.Fatal("expected total exposure rejection")
	}
	rejErr := err.(*ErrOrderRejected)
	if rejErr.Reason != ReasonTotalExposure {
		t.Fatalf("expected total exposure reason, got %s", rejErr.Reason)
	}

	// 400 should pass (4500+400=4900 < 5000)
	if err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 400, ""); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMaxConsecutiveLosses(t *testing.T) {
	g := newTestGuard(100000) // large portfolio to avoid drawdown trigger

	for i := 0; i < 5; i++ {
		g.RecordTrade(-10) // small losses
	}

	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 100, "")
	if err == nil {
		t.Fatal("expected consecutive losses rejection")
	}
	rejErr := err.(*ErrOrderRejected)
	if rejErr.Reason != ReasonConsecutiveLosses {
		t.Fatalf("expected consecutive losses reason, got %s", rejErr.Reason)
	}

	// Manual resume
	g.ResetConsecutiveLosses()
	if err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 100, ""); err != nil {
		t.Fatalf("expected pass after reset, got %v", err)
	}
}

func TestMaxDailyTrades(t *testing.T) {
	lc := NewLimitsConfig()
	lim := DefaultLimits()
	lim.MaxDailyTrades = 3
	lc.SetGlobal(lim)
	mon := NewMonitor(100000)
	g := NewGuard(lc, mon)

	for i := 0; i < 3; i++ {
		g.RecordTrade(10) // winning trades
	}

	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 100, "")
	if err == nil {
		t.Fatal("expected daily trades rejection")
	}
	rejErr := err.(*ErrOrderRejected)
	if rejErr.Reason != ReasonDailyTrades {
		t.Fatalf("expected daily trades reason, got %s", rejErr.Reason)
	}
}

func TestCombinedRules(t *testing.T) {
	g := newTestGuard(100000)

	// Record 5 consecutive losses (small enough to not trigger drawdown at 100k portfolio)
	for i := 0; i < 5; i++ {
		g.RecordTrade(-100)
	}

	// Should trigger consecutive losses (checked before drawdown threshold hit)
	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 100, "")
	if err == nil {
		t.Fatal("expected rejection")
	}

	// Enable kill switch too — should get kill switch reason (checked first)
	g.EnableKillSwitch()
	err = g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 100, "")
	rejErr := err.(*ErrOrderRejected)
	if rejErr.Reason != ReasonKillSwitch {
		t.Fatalf("kill switch should take priority, got %s", rejErr.Reason)
	}
}

func TestPerAgentLimits(t *testing.T) {
	lc := NewLimitsConfig()
	agentLim := DefaultLimits()
	agentLim.MaxPositionSize = 0.01 // 1% for this agent
	lc.SetAgentLimits("conservative", agentLim)

	mon := NewMonitor(10000)
	g := NewGuard(lc, mon)

	// 200 > 1% of 10000 = 100
	err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 200, "conservative")
	if err == nil {
		t.Fatal("expected rejection for conservative agent")
	}

	// Same order passes for global limits (10% = 1000)
	if err := g.CheckOrder(context.Background(), makeOrder("BTCUSDT"), 200, ""); err != nil {
		t.Fatalf("expected pass for global limits, got %v", err)
	}
}

func TestStatus(t *testing.T) {
	g := newTestGuard(10000)
	g.RecordTrade(-100)
	g.EnableKillSwitch()

	s := g.Status()
	if !s.KillSwitchActive {
		t.Fatal("expected kill switch active in status")
	}
	if s.State.DailyTradeCount != 1 {
		t.Fatalf("expected 1 trade, got %d", s.State.DailyTradeCount)
	}
	if s.State.DailyPnL != -100 {
		t.Fatalf("expected -100 pnl, got %f", s.State.DailyPnL)
	}
}
