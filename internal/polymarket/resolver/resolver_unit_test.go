package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculatePredictionPnl(t *testing.T) {
	tests := []struct {
		name          string
		predictedProb float64
		marketPrice   float64
		outcome       float64
		expected      float64
	}{
		{
			name:          "predicted > market, outcome YES (win)",
			predictedProb: 0.7,
			marketPrice:   0.5,
			outcome:       1.0,
			expected:      0.5, // outcome - marketPrice = 1.0 - 0.5
		},
		{
			name:          "predicted > market, outcome NO (loss)",
			predictedProb: 0.7,
			marketPrice:   0.5,
			outcome:       0.0,
			expected:      -0.5, // outcome - marketPrice = 0.0 - 0.5
		},
		{
			name:          "predicted < market, outcome NO (win)",
			predictedProb: 0.3,
			marketPrice:   0.5,
			outcome:       0.0,
			expected:      0.5, // (1-outcome) - (1-marketPrice) = 1.0 - 0.5
		},
		{
			name:          "predicted < market, outcome YES (loss)",
			predictedProb: 0.3,
			marketPrice:   0.5,
			outcome:       1.0,
			expected:      -0.5, // (1-outcome) - (1-marketPrice) = 0.0 - 0.5
		},
		{
			name:          "predicted == market, goes to NO branch, outcome NO (win)",
			predictedProb: 0.5,
			marketPrice:   0.5,
			outcome:       0.0,
			expected:      0.5, // (1-0.0) - (1-0.5) = 1.0 - 0.5
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calculatePredictionPnl(tc.predictedProb, tc.marketPrice, tc.outcome)
			assert.InDelta(t, tc.expected, got, 1e-9)
		})
	}
}
