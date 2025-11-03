package memory

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPolicy_SuccessRate tests policy success rate calculation
func TestPolicy_SuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expectedRate float64
	}{
		{
			name:         "No applications",
			successCount: 0,
			failureCount: 0,
			expectedRate: 0.0,
		},
		{
			name:         "All successful",
			successCount: 20,
			failureCount: 0,
			expectedRate: 1.0,
		},
		{
			name:         "All failed",
			successCount: 0,
			failureCount: 20,
			expectedRate: 0.0,
		},
		{
			name:         "Mixed results - 70% win rate",
			successCount: 14,
			failureCount: 6,
			expectedRate: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &Policy{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			assert.Equal(t, tt.expectedRate, policy.SuccessRate())
		})
	}
}

// TestPolicy_IsPerforming tests policy performance evaluation
func TestPolicy_IsPerforming(t *testing.T) {
	tests := []struct {
		name     string
		policy   *Policy
		expected bool
	}{
		{
			name: "New policy (not enough data)",
			policy: &Policy{
				TimesApplied: 3,
				SuccessCount: 1,
				FailureCount: 2,
				AvgPnL:       -5.0,
			},
			expected: true, // Give it a chance
		},
		{
			name: "Well-performing policy",
			policy: &Policy{
				TimesApplied: 20,
				SuccessCount: 14,
				FailureCount: 6,
				AvgPnL:       25.5,
			},
			expected: true,
		},
		{
			name: "Low win rate policy",
			policy: &Policy{
				TimesApplied: 20,
				SuccessCount: 8,
				FailureCount: 12,
				AvgPnL:       10.0,
			},
			expected: false, // < 50% win rate
		},
		{
			name: "Unprofitable policy",
			policy: &Policy{
				TimesApplied: 20,
				SuccessCount: 12,
				FailureCount: 8,
				AvgPnL:       -5.0,
			},
			expected: false, // Negative average P&L
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.policy.IsPerforming())
		})
	}
}

// TestSkill_SkillSuccessRate tests skill success rate calculation
func TestSkill_SkillSuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expectedRate float64
	}{
		{
			name:         "No uses",
			successCount: 0,
			failureCount: 0,
			expectedRate: 0.0,
		},
		{
			name:         "All successful",
			successCount: 50,
			failureCount: 0,
			expectedRate: 1.0,
		},
		{
			name:         "Mixed - 80% success",
			successCount: 80,
			failureCount: 20,
			expectedRate: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill := &Skill{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			assert.Equal(t, tt.expectedRate, skill.SkillSuccessRate())
		})
	}
}

// TestSkill_IsProficient tests skill proficiency evaluation
func TestSkill_IsProficient(t *testing.T) {
	tests := []struct {
		name     string
		skill    *Skill
		expected bool
	}{
		{
			name: "New skill (still learning)",
			skill: &Skill{
				TimesUsed:    8,
				SuccessCount: 5,
				FailureCount: 3,
				Proficiency:  0.5,
			},
			expected: true, // Still learning
		},
		{
			name: "Proficient skill",
			skill: &Skill{
				TimesUsed:    50,
				SuccessCount: 45,
				FailureCount: 5,
				Proficiency:  0.85,
			},
			expected: true,
		},
		{
			name: "Low success rate",
			skill: &Skill{
				TimesUsed:    50,
				SuccessCount: 30,
				FailureCount: 20,
				Proficiency:  0.8,
			},
			expected: false, // < 70% success rate
		},
		{
			name: "Low proficiency",
			skill: &Skill{
				TimesUsed:    50,
				SuccessCount: 40,
				FailureCount: 10,
				Proficiency:  0.55,
			},
			expected: false, // < 0.6 proficiency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.skill.IsProficient())
		})
	}
}

// TestProceduralMemory_StorePolicy tests policy storage (requires database)
func TestProceduralMemory_StorePolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup with procedural_memory tables")
}

// TestProceduralMemory_RecordPolicyApplication tests recording policy usage (requires database)
func TestProceduralMemory_RecordPolicyApplication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup with procedural_memory tables")
}

// TestProceduralMemory_StoreSkill tests skill storage (requires database)
func TestProceduralMemory_StoreSkill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup with procedural_memory tables")
}

// TestCreatePolicyConditions tests policy condition creation
func TestCreatePolicyConditions(t *testing.T) {
	data := map[string]interface{}{
		"rsi":          map[string]float64{"min": 40, "max": 70},
		"volume_ratio": 1.5,
		"trend":        "bullish",
	}

	conditions, err := CreatePolicyConditions(data)
	require.NoError(t, err)
	assert.NotNil(t, conditions)
	assert.Contains(t, string(conditions), "rsi")
	assert.Contains(t, string(conditions), "bullish")
}

// TestCreatePolicyActions tests policy action creation
func TestCreatePolicyActions(t *testing.T) {
	data := map[string]interface{}{
		"action":        "enter_long",
		"position_size": 0.1,
		"stop_loss":     0.02,
	}

	actions, err := CreatePolicyActions(data)
	require.NoError(t, err)
	assert.NotNil(t, actions)
	assert.Contains(t, string(actions), "enter_long")
	assert.Contains(t, string(actions), "0.02")
}

// TestPolicyTypes validates policy type constants
func TestPolicyTypes(t *testing.T) {
	assert.Equal(t, PolicyType("entry"), PolicyEntry)
	assert.Equal(t, PolicyType("exit"), PolicyExit)
	assert.Equal(t, PolicyType("sizing"), PolicySizing)
	assert.Equal(t, PolicyType("risk"), PolicyRisk)
	assert.Equal(t, PolicyType("hedging"), PolicyHedging)
	assert.Equal(t, PolicyType("rebalancing"), PolicyRebalancing)
}

// TestSkillTypes validates skill type constants
func TestSkillTypes(t *testing.T) {
	assert.Equal(t, SkillType("technical_analysis"), SkillTechnicalAnalysis)
	assert.Equal(t, SkillType("orderbook_analysis"), SkillOrderBookAnalysis)
	assert.Equal(t, SkillType("sentiment_analysis"), SkillSentimentAnalysis)
	assert.Equal(t, SkillType("trend_following"), SkillTrendFollowing)
	assert.Equal(t, SkillType("mean_reversion"), SkillMeanReversion)
	assert.Equal(t, SkillType("risk_management"), SkillRiskManagement)
}

// Mock ProceduralMemory for unit testing without database
type MockProceduralMemory struct {
	StorePolicyFunc             func(ctx context.Context, policy *Policy) error
	GetPoliciesByTypeFunc       func(ctx context.Context, policyType PolicyType, activeOnly bool) ([]*Policy, error)
	RecordPolicyApplicationFunc func(ctx context.Context, id uuid.UUID, success bool, pnl float64) error
	StoreSkillFunc              func(ctx context.Context, skill *Skill) error
}

func (m *MockProceduralMemory) StorePolicy(ctx context.Context, policy *Policy) error {
	if m.StorePolicyFunc != nil {
		return m.StorePolicyFunc(ctx, policy)
	}
	return nil
}

func (m *MockProceduralMemory) GetPoliciesByType(ctx context.Context, policyType PolicyType, activeOnly bool) ([]*Policy, error) {
	if m.GetPoliciesByTypeFunc != nil {
		return m.GetPoliciesByTypeFunc(ctx, policyType, activeOnly)
	}
	return []*Policy{}, nil
}

func (m *MockProceduralMemory) RecordPolicyApplication(ctx context.Context, id uuid.UUID, success bool, pnl float64) error {
	if m.RecordPolicyApplicationFunc != nil {
		return m.RecordPolicyApplicationFunc(ctx, id, success, pnl)
	}
	return nil
}

func (m *MockProceduralMemory) StoreSkill(ctx context.Context, skill *Skill) error {
	if m.StoreSkillFunc != nil {
		return m.StoreSkillFunc(ctx, skill)
	}
	return nil
}

// Example of using MockProceduralMemory in tests
func TestMockProceduralMemory(t *testing.T) {
	mock := &MockProceduralMemory{
		StorePolicyFunc: func(ctx context.Context, policy *Policy) error {
			policy.ID = uuid.New()
			return nil
		},
		GetPoliciesByTypeFunc: func(ctx context.Context, policyType PolicyType, activeOnly bool) ([]*Policy, error) {
			return []*Policy{
				{
					ID:         uuid.New(),
					Type:       PolicyEntry,
					Name:       "Test Entry Policy",
					Confidence: 0.8,
					IsActive:   true,
				},
			}, nil
		},
	}

	ctx := context.Background()

	// Test StorePolicy
	policy := &Policy{
		Type: PolicyEntry,
		Name: "New Entry Policy",
	}
	err := mock.StorePolicy(ctx, policy)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, policy.ID)

	// Test GetPoliciesByType
	policies, err := mock.GetPoliciesByType(ctx, PolicyEntry, true)
	require.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, PolicyEntry, policies[0].Type)
	assert.True(t, policies[0].IsActive)
}
