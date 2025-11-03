package llm

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExperimentManager(t *testing.T) {
	em := NewExperimentManager(nil)
	require.NotNil(t, em)
	assert.NotNil(t, em.experiments)
}

func TestCreateExperiment(t *testing.T) {
	em := NewExperimentManager(nil)

	control := &Variant{
		ID:          "control",
		Name:        "Claude Sonnet 4",
		Model:       "claude-sonnet-4-20250514",
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	variant1 := &Variant{
		ID:          "variant-1",
		Name:        "GPT-4",
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	exp := &Experiment{
		Name:        "Claude vs GPT-4",
		Description: "Compare Claude Sonnet 4 against GPT-4",
		Control:     control,
		Variants:    []*Variant{variant1},
		TrafficSplit: map[string]float64{
			"control":   0.5,
			"variant-1": 0.5,
		},
	}

	err := em.CreateExperiment(exp)
	require.NoError(t, err)
	assert.NotEmpty(t, exp.ID)
	assert.True(t, exp.Active)
	assert.False(t, exp.StartTime.IsZero())
}

func TestCreateExperiment_InvalidTrafficSplit(t *testing.T) {
	em := NewExperimentManager(nil)

	exp := &Experiment{
		Name:    "Invalid Experiment",
		Control: &Variant{ID: "control"},
		TrafficSplit: map[string]float64{
			"control":   0.3,
			"variant-1": 0.5, // Only sums to 0.8
		},
	}

	err := em.CreateExperiment(exp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "traffic split must sum to 1.0")
}

func TestGetExperiment(t *testing.T) {
	em := NewExperimentManager(nil)

	exp := &Experiment{
		ID:      "test-exp-1",
		Name:    "Test Experiment",
		Control: &Variant{ID: "control"},
		TrafficSplit: map[string]float64{
			"control": 1.0,
		},
	}

	err := em.CreateExperiment(exp)
	require.NoError(t, err)

	retrieved, exists := em.GetExperiment("test-exp-1")
	assert.True(t, exists)
	assert.Equal(t, "Test Experiment", retrieved.Name)

	_, exists = em.GetExperiment("nonexistent")
	assert.False(t, exists)
}

func TestListActiveExperiments(t *testing.T) {
	em := NewExperimentManager(nil)

	// Create active experiment
	exp1 := &Experiment{
		ID:      "active-exp",
		Name:    "Active",
		Control: &Variant{ID: "control"},
		TrafficSplit: map[string]float64{
			"control": 1.0,
		},
	}
	err := em.CreateExperiment(exp1)
	require.NoError(t, err)

	// Create inactive experiment
	exp2 := &Experiment{
		ID:      "inactive-exp",
		Name:    "Inactive",
		Active:  false,
		Control: &Variant{ID: "control"},
		TrafficSplit: map[string]float64{
			"control": 1.0,
		},
	}
	em.experiments[exp2.ID] = exp2

	active := em.ListActiveExperiments()
	assert.Len(t, active, 1)
	assert.Equal(t, "active-exp", active[0].ID)
}

func TestStopExperiment(t *testing.T) {
	em := NewExperimentManager(nil)

	exp := &Experiment{
		ID:      "test-exp",
		Name:    "Test",
		Control: &Variant{ID: "control"},
		TrafficSplit: map[string]float64{
			"control": 1.0,
		},
	}
	err := em.CreateExperiment(exp)
	require.NoError(t, err)
	assert.True(t, exp.Active)

	err = em.StopExperiment("test-exp")
	require.NoError(t, err)
	assert.False(t, exp.Active)
	assert.NotNil(t, exp.EndTime)
}

func TestStopExperiment_NotFound(t *testing.T) {
	em := NewExperimentManager(nil)

	err := em.StopExperiment("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment not found")
}

func TestSelectVariant_ConsistentHashing(t *testing.T) {
	em := NewExperimentManager(nil)

	control := &Variant{
		ID:    "control",
		Name:  "Control",
		Model: "claude-sonnet-4",
	}

	variant1 := &Variant{
		ID:    "variant-1",
		Name:  "Variant 1",
		Model: "gpt-4",
	}

	exp := &Experiment{
		ID:       "test-exp",
		Name:     "Test",
		Control:  control,
		Variants: []*Variant{variant1},
		TrafficSplit: map[string]float64{
			"control":   0.5,
			"variant-1": 0.5,
		},
		Active: true,
	}

	err := em.CreateExperiment(exp)
	require.NoError(t, err)

	// Test that same key always gets same variant
	decisionKey := "user123-BTC/USDT-2025-01-01"

	firstSelection, err := em.SelectVariant("test-exp", decisionKey)
	require.NoError(t, err)
	require.NotNil(t, firstSelection)

	// Verify the selection is one of the valid variants
	assert.True(t, firstSelection.ID == "control" || firstSelection.ID == "variant-1",
		"Selected variant should be either control or variant-1")

	// Select 10 more times with same key - should always get the same variant
	for i := 0; i < 10; i++ {
		selection, err := em.SelectVariant("test-exp", decisionKey)
		require.NoError(t, err)
		assert.Equal(t, firstSelection.ID, selection.ID, "Variant should be consistent for same key")
		assert.Equal(t, firstSelection.Model, selection.Model, "Model should match")
	}

	// Different key should potentially get different variant (but consistently)
	differentKey := "user456-ETH/USDT-2025-01-02"
	secondSelection, err := em.SelectVariant("test-exp", differentKey)
	require.NoError(t, err)
	require.NotNil(t, secondSelection)

	// Verify consistency for second key as well
	for i := 0; i < 5; i++ {
		selection, err := em.SelectVariant("test-exp", differentKey)
		require.NoError(t, err)
		assert.Equal(t, secondSelection.ID, selection.ID, "Second key should also be consistent")
	}
}

func TestSelectVariant_TrafficDistribution(t *testing.T) {
	em := NewExperimentManager(nil)

	control := &Variant{
		ID:    "control",
		Name:  "Control",
		Model: "claude-sonnet-4",
	}

	variant1 := &Variant{
		ID:    "variant-1",
		Name:  "Variant 1",
		Model: "gpt-4",
	}

	exp := &Experiment{
		ID:       "test-exp",
		Name:     "Test",
		Control:  control,
		Variants: []*Variant{variant1},
		TrafficSplit: map[string]float64{
			"control":   0.7, // 70% control
			"variant-1": 0.3, // 30% variant
		},
		Active: true,
	}

	err := em.CreateExperiment(exp)
	require.NoError(t, err)

	// Test distribution over many selections
	controlCount := 0
	variantCount := 0
	iterations := 1000

	for i := 0; i < iterations; i++ {
		decisionKey := generateTestKey(i)
		variant, err := em.SelectVariant("test-exp", decisionKey)
		require.NoError(t, err)

		if variant.ID == "control" {
			controlCount++
		} else {
			variantCount++
		}
	}

	// Check distribution is roughly 70/30 (allow 10% margin)
	controlRatio := float64(controlCount) / float64(iterations)
	variantRatio := float64(variantCount) / float64(iterations)

	assert.InDelta(t, 0.7, controlRatio, 0.1, "Control should get ~70% of traffic")
	assert.InDelta(t, 0.3, variantRatio, 0.1, "Variant should get ~30% of traffic")
}

func TestSelectVariant_InactiveExperiment(t *testing.T) {
	em := NewExperimentManager(nil)

	exp := &Experiment{
		ID:      "inactive-exp",
		Name:    "Inactive",
		Active:  false,
		Control: &Variant{ID: "control"},
		TrafficSplit: map[string]float64{
			"control": 1.0,
		},
	}
	em.experiments[exp.ID] = exp

	_, err := em.SelectVariant("inactive-exp", "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestDetermineWinner(t *testing.T) {
	em := NewExperimentManager(nil)

	tests := []struct {
		name           string
		comparison     *ExperimentComparison
		expectedWinner string
		expectSigWin   bool
	}{
		{
			name: "Clear winner with enough data",
			comparison: &ExperimentComparison{
				Control: &ExperimentResult{
					VariantID:      "control",
					TotalDecisions: 50,
					SuccessCount:   30,
					SuccessRate:    60.0,
				},
				Variants: []*ExperimentResult{
					{
						VariantID:      "variant-1",
						TotalDecisions: 50,
						SuccessCount:   40,
						SuccessRate:    80.0,
					},
				},
			},
			expectedWinner: "variant-1",
			expectSigWin:   true,
		},
		{
			name: "Not enough data",
			comparison: &ExperimentComparison{
				Control: &ExperimentResult{
					VariantID:      "control",
					TotalDecisions: 10,
					SuccessRate:    50.0,
				},
				Variants: []*ExperimentResult{
					{
						VariantID:      "variant-1",
						TotalDecisions: 10,
						SuccessRate:    70.0,
					},
				},
			},
			expectedWinner: "",
			expectSigWin:   false,
		},
		{
			name: "Control wins",
			comparison: &ExperimentComparison{
				Control: &ExperimentResult{
					VariantID:      "control",
					TotalDecisions: 50,
					SuccessCount:   45,
					SuccessRate:    90.0,
				},
				Variants: []*ExperimentResult{
					{
						VariantID:      "variant-1",
						TotalDecisions: 50,
						SuccessCount:   40,
						SuccessRate:    80.0,
					},
				},
			},
			expectedWinner: "control",
			expectSigWin:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner := em.determineWinner(tt.comparison)
			assert.Equal(t, tt.expectedWinner, winner)
			assert.Equal(t, tt.expectSigWin, tt.comparison.StatSigWinner)
		})
	}
}

func TestCreateClientFromVariant(t *testing.T) {
	variant := &Variant{
		ID:          "test-variant",
		Name:        "Test",
		Model:       "gpt-4",
		Temperature: 0.8,
		MaxTokens:   1500,
	}

	baseConfig := ClientConfig{
		Endpoint:    "http://localhost:8080/v1/chat/completions",
		APIKey:      "test-key",
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	client := CreateClientFromVariant(variant, baseConfig)
	require.NotNil(t, client)
	assert.Equal(t, "gpt-4", client.model)
	assert.Equal(t, 0.8, client.temperature)
	assert.Equal(t, 1500, client.maxTokens)
}

// Helper function to generate unique test keys
func generateTestKey(i int) string {
	return fmt.Sprintf("test-key-%d-%d", i, time.Now().UnixNano())
}
