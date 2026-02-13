package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ManagerConfig holds configuration for the notification manager
type ManagerConfig struct {
	MaxPerHour       int           `yaml:"max_per_hour"`
	DedupCooldown    time.Duration `yaml:"dedup_cooldown"`
	MinPriority      Priority      `yaml:"min_priority"`
	QuietHoursStart  string        `yaml:"quiet_hours_start"`  // "HH:MM"
	QuietHoursEnd    string        `yaml:"quiet_hours_end"`    // "HH:MM"
	QuietHoursTimezone string      `yaml:"quiet_hours_timezone"`
}

// DefaultManagerConfig returns sensible defaults
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxPerHour:         60,
		DedupCooldown:      5 * time.Minute,
		MinPriority:        PriorityInfo,
		QuietHoursStart:    "23:00",
		QuietHoursEnd:      "07:00",
		QuietHoursTimezone: "Asia/Calcutta",
	}
}

// rateBucket tracks sends per channel per hour
type rateBucket struct {
	timestamps []time.Time
}

// Manager is the central notification dispatcher
type Manager struct {
	mu         sync.RWMutex
	channels   map[string]Channel
	config     ManagerConfig
	rateLimits map[string]*rateBucket // key: channel name
	dedupCache map[string]time.Time   // key: event hash -> last sent
}

// NewManager creates a new notification manager
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		channels:   make(map[string]Channel),
		config:     cfg,
		rateLimits: make(map[string]*rateBucket),
		dedupCache: make(map[string]time.Time),
	}
}

// RegisterChannel adds a channel to the manager
func (m *Manager) RegisterChannel(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.Name()] = ch
	m.rateLimits[ch.Name()] = &rateBucket{}
	log.Info().Str("channel", ch.Name()).Msg("Registered notification channel")
}

// Notify dispatches an event to all registered channels
func (m *Manager) Notify(ctx context.Context, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.mu.RLock()
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.RUnlock()

	if len(channels) == 0 {
		return nil
	}

	// Check priority filter
	if event.Priority < m.config.MinPriority {
		return nil
	}

	// Check quiet hours (emergency bypasses)
	if event.Priority < PriorityEmergency && m.isQuietHours() {
		log.Debug().Str("event", string(event.Type)).Msg("Suppressed during quiet hours")
		return nil
	}

	// Check dedup
	if m.isDuplicate(event) {
		log.Debug().Str("event", string(event.Type)).Msg("Deduplicated notification")
		return nil
	}

	var lastErr error
	for _, ch := range channels {
		if !m.checkRateLimit(ch.Name()) {
			log.Warn().Str("channel", ch.Name()).Msg("Rate limit exceeded, skipping")
			continue
		}
		if err := ch.Send(ctx, event); err != nil {
			log.Error().Err(err).Str("channel", ch.Name()).Msg("Failed to send notification")
			lastErr = err
		} else {
			m.recordSend(ch.Name())
		}
	}

	m.recordDedup(event)
	return lastErr
}

// Close shuts down all channels
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var lastErr error
	for name, ch := range m.channels {
		if err := ch.Close(); err != nil {
			log.Error().Err(err).Str("channel", name).Msg("Error closing channel")
			lastErr = err
		}
	}
	return lastErr
}

// eventHash produces a dedup key for an event
func eventHash(e Event) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s:%s:%s", e.Type, e.Title, e.Message)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (m *Manager) isDuplicate(e Event) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := eventHash(e)
	if lastSent, ok := m.dedupCache[key]; ok {
		return time.Since(lastSent) < m.config.DedupCooldown
	}
	return false
}

func (m *Manager) recordDedup(e Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dedupCache[eventHash(e)] = time.Now()
}

func (m *Manager) checkRateLimit(channelName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bucket, ok := m.rateLimits[channelName]
	if !ok {
		return true
	}
	cutoff := time.Now().Add(-time.Hour)
	count := 0
	for _, ts := range bucket.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}
	return count < m.config.MaxPerHour
}

func (m *Manager) recordSend(channelName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := m.rateLimits[channelName]
	if bucket == nil {
		bucket = &rateBucket{}
		m.rateLimits[channelName] = bucket
	}
	now := time.Now()
	cutoff := now.Add(-time.Hour)
	// Prune old entries
	fresh := bucket.timestamps[:0]
	for _, ts := range bucket.timestamps {
		if ts.After(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	bucket.timestamps = append(fresh, now)
}

// IsQuietHours checks if current time is within quiet hours
func (m *Manager) isQuietHours() bool {
	if m.config.QuietHoursStart == "" || m.config.QuietHoursEnd == "" {
		return false
	}
	loc, err := time.LoadLocation(m.config.QuietHoursTimezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	startH, startM := parseHHMM(m.config.QuietHoursStart)
	endH, endM := parseHHMM(m.config.QuietHoursEnd)

	nowMin := now.Hour()*60 + now.Minute()
	startMin := startH*60 + startM
	endMin := endH*60 + endM

	if startMin <= endMin {
		return nowMin >= startMin && nowMin < endMin
	}
	// Wraps midnight
	return nowMin >= startMin || nowMin < endMin
}

func parseHHMM(s string) (int, int) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h, m
}
