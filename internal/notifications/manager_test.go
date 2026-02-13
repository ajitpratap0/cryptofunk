package notifications

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockChannel implements Channel for testing
type mockChannel struct {
	name   string
	mu     sync.Mutex
	events []Event
	err    error
}

func newMockChannel(name string) *mockChannel {
	return &mockChannel{name: name}
}

func (m *mockChannel) Name() string { return m.name }
func (m *mockChannel) Close() error { return nil }
func (m *mockChannel) Send(_ context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return m.err
}
func (m *mockChannel) sentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestManagerNotify(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.QuietHoursStart = ""
	cfg.QuietHoursEnd = ""
	mgr := NewManager(cfg)

	ch := newMockChannel("test")
	mgr.RegisterChannel(ch)

	event := TradeExecutedEvent("BTCUSDT", "BUY", "trend-agent", 50000, 0.1)
	if err := mgr.Notify(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ch.sentCount() != 1 {
		t.Fatalf("expected 1 event sent, got %d", ch.sentCount())
	}
}

func TestManagerDeduplication(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.QuietHoursStart = ""
	cfg.QuietHoursEnd = ""
	cfg.DedupCooldown = 1 * time.Hour
	mgr := NewManager(cfg)

	ch := newMockChannel("test")
	mgr.RegisterChannel(ch)

	event := ErrorAlertEvent("api", "connection timeout")

	_ = mgr.Notify(context.Background(), event)
	_ = mgr.Notify(context.Background(), event) // duplicate

	if ch.sentCount() != 1 {
		t.Fatalf("expected 1 (deduplicated), got %d", ch.sentCount())
	}
}

func TestManagerRateLimit(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MaxPerHour = 3
	cfg.DedupCooldown = 0
	cfg.QuietHoursStart = ""
	cfg.QuietHoursEnd = ""
	mgr := NewManager(cfg)

	ch := newMockChannel("test")
	mgr.RegisterChannel(ch)

	for i := 0; i < 5; i++ {
		event := Event{
			Type:      EventTradeExecuted,
			Priority:  PriorityInfo,
			Title:     "Trade",
			Message:   formatInt(i),
			Timestamp: time.Now(),
		}
		_ = mgr.Notify(context.Background(), event)
	}

	if ch.sentCount() > 3 {
		t.Fatalf("rate limit exceeded: got %d sends, expected max 3", ch.sentCount())
	}
}

func TestManagerPriorityFilter(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MinPriority = PriorityWarning
	cfg.QuietHoursStart = ""
	cfg.QuietHoursEnd = ""
	mgr := NewManager(cfg)

	ch := newMockChannel("test")
	mgr.RegisterChannel(ch)

	info := Event{Type: EventTradeExecuted, Priority: PriorityInfo, Title: "t", Message: "m", Timestamp: time.Now()}
	warn := Event{Type: EventErrorAlert, Priority: PriorityWarning, Title: "t", Message: "m2", Timestamp: time.Now()}

	_ = mgr.Notify(context.Background(), info)
	_ = mgr.Notify(context.Background(), warn)

	if ch.sentCount() != 1 {
		t.Fatalf("expected 1 (warning only), got %d", ch.sentCount())
	}
}

func TestManagerQuietHours(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.DedupCooldown = 0
	// Set quiet hours to cover all 24h
	cfg.QuietHoursStart = "00:00"
	cfg.QuietHoursEnd = "23:59"
	cfg.QuietHoursTimezone = "UTC"
	mgr := NewManager(cfg)

	ch := newMockChannel("test")
	mgr.RegisterChannel(ch)

	// Info event should be suppressed
	info := Event{Type: EventTradeExecuted, Priority: PriorityInfo, Title: "t", Message: "m", Timestamp: time.Now()}
	_ = mgr.Notify(context.Background(), info)

	// Emergency should bypass quiet hours
	emergency := Event{Type: EventSafetyAlert, Priority: PriorityEmergency, Title: "e", Message: "m2", Timestamp: time.Now()}
	_ = mgr.Notify(context.Background(), emergency)

	if ch.sentCount() != 1 {
		t.Fatalf("expected 1 (emergency only), got %d", ch.sentCount())
	}
}

func TestMultipleChannels(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.QuietHoursStart = ""
	cfg.QuietHoursEnd = ""
	mgr := NewManager(cfg)

	ch1 := newMockChannel("ch1")
	ch2 := newMockChannel("ch2")
	mgr.RegisterChannel(ch1)
	mgr.RegisterChannel(ch2)

	event := SafetyAlertEvent("kill_switch", "Maximum drawdown exceeded")
	_ = mgr.Notify(context.Background(), event)

	if ch1.sentCount() != 1 || ch2.sentCount() != 1 {
		t.Fatalf("expected both channels to receive event: ch1=%d ch2=%d", ch1.sentCount(), ch2.sentCount())
	}
}

func TestParseHHMM(t *testing.T) {
	h, m := parseHHMM("23:00")
	if h != 23 || m != 0 {
		t.Fatalf("expected 23:00, got %d:%d", h, m)
	}
	h, m = parseHHMM("07:30")
	if h != 7 || m != 30 {
		t.Fatalf("expected 7:30, got %d:%d", h, m)
	}
}
