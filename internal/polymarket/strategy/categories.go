package strategy

import "sync"

// CategoryManager manages market categories and their performance-based enablement.
type CategoryManager struct {
	categories map[string]*CategoryConfig
	mu         sync.RWMutex
	tracker    *PerformanceTracker
}

// CategoryConfig holds config for a market category.
type CategoryConfig struct {
	Name               string  `json:"name"`
	Enabled            bool    `json:"enabled"`
	MinResolvedToJudge int     `json:"min_resolved_to_judge"` // min resolved predictions before auto-disable
	MaxBrierScore      float64 `json:"max_brier_score"`       // disable if Brier exceeds this
}

// DefaultCategories returns the default set of categories.
func DefaultCategories() map[string]*CategoryConfig {
	return map[string]*CategoryConfig{
		"crypto":   {Name: "crypto", Enabled: true, MinResolvedToJudge: 10, MaxBrierScore: 0.30},
		"tech":     {Name: "tech", Enabled: true, MinResolvedToJudge: 10, MaxBrierScore: 0.30},
		"politics": {Name: "politics", Enabled: true, MinResolvedToJudge: 10, MaxBrierScore: 0.30},
		"science":  {Name: "science", Enabled: true, MinResolvedToJudge: 10, MaxBrierScore: 0.30},
		"sports":   {Name: "sports", Enabled: true, MinResolvedToJudge: 15, MaxBrierScore: 0.28},
		"other":    {Name: "other", Enabled: true, MinResolvedToJudge: 20, MaxBrierScore: 0.30},
	}
}

// NewCategoryManager creates a new category manager.
func NewCategoryManager(tracker *PerformanceTracker) *CategoryManager {
	return &CategoryManager{
		categories: DefaultCategories(),
		tracker:    tracker,
	}
}

// IsEnabled returns whether a category is currently enabled for trading.
func (cm *CategoryManager) IsEnabled(category string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cat, ok := cm.categories[category]
	if !ok {
		// Unknown categories fall through to "other"
		cat, ok = cm.categories["other"]
		if !ok {
			return true
		}
	}
	return cat.Enabled
}

// GetStats returns category stats from the tracker.
func (cm *CategoryManager) GetStats(category string) *CategoryStats {
	if cm.tracker == nil {
		return nil
	}
	stats := cm.tracker.GetStats()
	if cs, ok := stats.ByCategory[category]; ok {
		return cs
	}
	return nil
}

// RefreshEnabled updates category enablement based on performance data.
func (cm *CategoryManager) RefreshEnabled() {
	if cm.tracker == nil {
		return
	}

	stats := cm.tracker.GetStats()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	for catName, config := range cm.categories {
		catStats, ok := stats.ByCategory[catName]
		if !ok {
			continue
		}
		if catStats.ResolvedCount >= config.MinResolvedToJudge {
			config.Enabled = catStats.BrierScore <= config.MaxBrierScore
		}
	}
}

// SetEnabled manually enables or disables a category.
func (cm *CategoryManager) SetEnabled(category string, enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cat, ok := cm.categories[category]; ok {
		cat.Enabled = enabled
	}
}

// ListCategories returns all category configs.
func (cm *CategoryManager) ListCategories() map[string]*CategoryConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	out := make(map[string]*CategoryConfig, len(cm.categories))
	for k, v := range cm.categories {
		cp := *v
		out[k] = &cp
	}
	return out
}
