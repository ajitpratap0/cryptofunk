package strategy

import (
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PerformanceTracker tracks predictions and computes performance metrics.
// This is an in-memory implementation; a PostgreSQL-backed version can be added later.
type PerformanceTracker struct {
	predictions []Prediction
	mu          sync.RWMutex
}

// NewPerformanceTracker creates a new tracker.
func NewPerformanceTracker() *PerformanceTracker {
	return &PerformanceTracker{
		predictions: make([]Prediction, 0, 256),
	}
}

// RecordPrediction stores a new prediction.
func (pt *PerformanceTracker) RecordPrediction(p Prediction) string {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	pt.predictions = append(pt.predictions, p)
	return p.ID
}

// ResolvePrediction marks a prediction as resolved.
func (pt *PerformanceTracker) ResolvePrediction(id string, outcome float64) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for i := range pt.predictions {
		if pt.predictions[i].ID == id {
			pt.predictions[i].Outcome = &outcome
			pt.predictions[i].Resolved = true
			now := time.Now()
			pt.predictions[i].ResolvedAt = &now

			// Calculate P&L (simplified: assume $1 bet at market price)
			if outcome == 1 {
				pt.predictions[i].PnL = (1.0 / pt.predictions[i].MarketPrice) - 1.0
			} else {
				pt.predictions[i].PnL = -1.0
			}
			return true
		}
	}
	return false
}

// GetStats returns aggregate performance statistics.
func (pt *PerformanceTracker) GetStats() *PerformanceStats {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	stats := &PerformanceStats{
		ByCategory: make(map[string]*CategoryStats),
	}

	stats.TotalPredictions = len(pt.predictions)

	var brierSum float64
	var correct int

	for _, p := range pt.predictions {
		// Ensure category entry exists
		cat, ok := stats.ByCategory[p.Category]
		if !ok {
			cat = &CategoryStats{Category: p.Category, Enabled: true}
			stats.ByCategory[p.Category] = cat
		}
		cat.TotalPredictions++

		if !p.Resolved || p.Outcome == nil {
			continue
		}

		stats.ResolvedCount++
		cat.ResolvedCount++
		stats.TotalPnL += p.PnL
		cat.TotalPnL += p.PnL

		outcome := *p.Outcome
		brierContrib := math.Pow(p.PredictedProb-outcome, 2)
		brierSum += brierContrib
		cat.BrierScore += brierContrib

		// Accuracy: correct if predicted > 0.5 and outcome=1, or predicted < 0.5 and outcome=0
		if (p.PredictedProb > 0.5 && outcome == 1) || (p.PredictedProb < 0.5 && outcome == 0) {
			correct++
			cat.Accuracy++
		}
	}

	if stats.ResolvedCount > 0 {
		stats.BrierScore = brierSum / float64(stats.ResolvedCount)
		stats.Accuracy = float64(correct) / float64(stats.ResolvedCount)
	}

	for _, cat := range stats.ByCategory {
		if cat.ResolvedCount > 0 {
			cat.BrierScore /= float64(cat.ResolvedCount)
			cat.Accuracy /= float64(cat.ResolvedCount)
		}
	}

	return stats
}

// GetPredictions returns all predictions (for export/debugging).
func (pt *PerformanceTracker) GetPredictions() []Prediction {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	out := make([]Prediction, len(pt.predictions))
	copy(out, pt.predictions)
	return out
}
