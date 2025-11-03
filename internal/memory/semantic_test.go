package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKnowledgeItem_SuccessRate tests success rate calculation
func TestKnowledgeItem_SuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failureCount int
		expectedRate float64
	}{
		{
			name:         "No validations",
			successCount: 0,
			failureCount: 0,
			expectedRate: 0.0,
		},
		{
			name:         "All successful",
			successCount: 10,
			failureCount: 0,
			expectedRate: 1.0,
		},
		{
			name:         "All failed",
			successCount: 0,
			failureCount: 10,
			expectedRate: 0.0,
		},
		{
			name:         "Mixed results",
			successCount: 7,
			failureCount: 3,
			expectedRate: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &KnowledgeItem{
				SuccessCount: tt.successCount,
				FailureCount: tt.failureCount,
			}
			assert.Equal(t, tt.expectedRate, item.SuccessRate())
		})
	}
}

// TestKnowledgeItem_IsValid tests knowledge validation logic
func TestKnowledgeItem_IsValid(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		item     *KnowledgeItem
		expected bool
	}{
		{
			name: "Valid knowledge",
			item: &KnowledgeItem{
				ValidationCount: 10,
				SuccessCount:    8,
				FailureCount:    2,
				CreatedAt:       now.Add(-24 * time.Hour),
			},
			expected: true,
		},
		{
			name: "Expired knowledge",
			item: &KnowledgeItem{
				ValidationCount: 10,
				SuccessCount:    8,
				FailureCount:    2,
				CreatedAt:       now.Add(-24 * time.Hour),
				ExpiresAt:       ptrTime(now.Add(-1 * time.Hour)),
			},
			expected: false,
		},
		{
			name: "Low success rate",
			item: &KnowledgeItem{
				ValidationCount: 10,
				SuccessCount:    3,
				FailureCount:    7,
				CreatedAt:       now.Add(-24 * time.Hour),
			},
			expected: false,
		},
		{
			name: "Not enough validations",
			item: &KnowledgeItem{
				ValidationCount: 3,
				SuccessCount:    1,
				FailureCount:    2,
				CreatedAt:       now.Add(-24 * time.Hour),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.item.IsValid())
		})
	}
}

// TestKnowledgeItem_Recency tests recency score calculation
func TestKnowledgeItem_Recency(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		minScore  float64
		maxScore  float64
	}{
		{
			name:      "Very recent (1 hour ago)",
			createdAt: now.Add(-1 * time.Hour),
			minScore:  0.95,
			maxScore:  1.0,
		},
		{
			name:      "Recent (1 day ago)",
			createdAt: now.Add(-24 * time.Hour),
			minScore:  0.9,
			maxScore:  1.0,
		},
		{
			name:      "Moderate (7 days ago)",
			createdAt: now.Add(-7 * 24 * time.Hour),
			minScore:  0.7,
			maxScore:  0.9,
		},
		{
			name:      "Old (30 days ago)",
			createdAt: now.Add(-30 * 24 * time.Hour),
			minScore:  0.4,
			maxScore:  0.6,
		},
		{
			name:      "Very old (90 days ago)",
			createdAt: now.Add(-90 * 24 * time.Hour),
			minScore:  0.1,
			maxScore:  0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &KnowledgeItem{
				CreatedAt: tt.createdAt,
			}
			score := item.Recency()
			assert.GreaterOrEqual(t, score, tt.minScore, "Recency score should be >= %f", tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore, "Recency score should be <= %f", tt.maxScore)
		})
	}
}

// TestKnowledgeItem_RelevanceScore tests combined relevance scoring
func TestKnowledgeItem_RelevanceScore(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		item     *KnowledgeItem
		minScore float64
		maxScore float64
	}{
		{
			name: "High quality, recent",
			item: &KnowledgeItem{
				Confidence:      0.9,
				Importance:      0.9,
				ValidationCount: 10,
				SuccessCount:    9,
				FailureCount:    1,
				CreatedAt:       now.Add(-24 * time.Hour),
			},
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name: "Medium quality, old",
			item: &KnowledgeItem{
				Confidence:      0.6,
				Importance:      0.6,
				ValidationCount: 10,
				SuccessCount:    6,
				FailureCount:    4,
				CreatedAt:       now.Add(-60 * 24 * time.Hour),
			},
			minScore: 0.3,
			maxScore: 0.6,
		},
		{
			name: "Invalid knowledge",
			item: &KnowledgeItem{
				Confidence:      0.5,
				Importance:      0.5,
				ValidationCount: 10,
				SuccessCount:    2,
				FailureCount:    8,
				CreatedAt:       now.Add(-30 * 24 * time.Hour),
			},
			minScore: 0.0,
			maxScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := tt.item.RelevanceScore()
			assert.GreaterOrEqual(t, score, tt.minScore, "Relevance score should be >= %f", tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore, "Relevance score should be <= %f", tt.maxScore)
		})
	}
}

// TestSemanticMemory_Store tests storing knowledge (requires database)
func TestSemanticMemory_Store(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup with semantic_memory table")

	// Example of how this test would work with a database:
	/*
		ctx := context.Background()
		pool := setupTestDB(t)
		defer pool.Close()

		sm := NewSemanticMemory(pool)

		item := &KnowledgeItem{
			Type:       KnowledgePattern,
			Content:    "Test pattern",
			Embedding:  make([]float32, 1536),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  "test-agent",
		}

		err := sm.Store(ctx, item)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, item.ID)
	*/
}

// TestSemanticMemory_FindSimilar tests similarity search (requires database)
func TestSemanticMemory_FindSimilar(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup with semantic_memory table")
}

// TestSemanticMemory_RecordValidation tests validation recording (requires database)
func TestSemanticMemory_RecordValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test - requires database setup with semantic_memory table")
}

// TestFilters tests query filter construction
func TestTypeFilter(t *testing.T) {
	filter := TypeFilter{Type: KnowledgePattern}
	clause, args := filter.SQL(3)

	assert.Equal(t, "type = $3", clause)
	require.Len(t, args, 1)
	assert.Equal(t, KnowledgePattern, args[0])
}

func TestAgentFilter(t *testing.T) {
	filter := AgentFilter{AgentName: "technical-agent"}
	clause, args := filter.SQL(5)

	assert.Equal(t, "agent_name = $5", clause)
	require.Len(t, args, 1)
	assert.Equal(t, "technical-agent", args[0])
}

func TestSymbolFilter(t *testing.T) {
	filter := SymbolFilter{Symbol: "BTC/USDT"}
	clause, args := filter.SQL(2)

	assert.Equal(t, "symbol = $2", clause)
	require.Len(t, args, 1)
	assert.Equal(t, "BTC/USDT", args[0])
}

func TestMinConfidenceFilter(t *testing.T) {
	filter := MinConfidenceFilter{MinConfidence: 0.7}
	clause, args := filter.SQL(4)

	assert.Equal(t, "confidence >= $4", clause)
	require.Len(t, args, 1)
	assert.Equal(t, 0.7, args[0])
}

func TestValidOnlyFilter(t *testing.T) {
	filter := ValidOnlyFilter{}
	clause, args := filter.SQL(1)

	assert.Contains(t, clause, "expires_at")
	assert.Contains(t, clause, "validation_count")
	assert.Contains(t, clause, "success_count")
	assert.Nil(t, args)
}

// TestCreateKnowledgeContext tests context creation
func TestCreateKnowledgeContext(t *testing.T) {
	data := map[string]interface{}{
		"indicators": map[string]float64{
			"rsi":  75.5,
			"macd": -0.5,
		},
		"market_condition": "overbought",
	}

	context, err := CreateKnowledgeContext(data)
	require.NoError(t, err)
	assert.NotNil(t, context)
	assert.Contains(t, string(context), "rsi")
	assert.Contains(t, string(context), "75.5")
}

// TestKnowledgeTypes validates knowledge type constants
func TestKnowledgeTypes(t *testing.T) {
	assert.Equal(t, KnowledgeType("fact"), KnowledgeFact)
	assert.Equal(t, KnowledgeType("pattern"), KnowledgePattern)
	assert.Equal(t, KnowledgeType("experience"), KnowledgeExperience)
	assert.Equal(t, KnowledgeType("strategy"), KnowledgeStrategy)
	assert.Equal(t, KnowledgeType("risk"), KnowledgeRisk)
}

// Mock SemanticMemory for unit testing without database
type MockSemanticMemory struct {
	StoreFunc            func(ctx context.Context, item *KnowledgeItem) error
	FindSimilarFunc      func(ctx context.Context, embedding []float32, limit int, filters ...Filter) ([]*KnowledgeItem, error)
	RecordValidationFunc func(ctx context.Context, id uuid.UUID, success bool) error
}

func (m *MockSemanticMemory) Store(ctx context.Context, item *KnowledgeItem) error {
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, item)
	}
	return nil
}

func (m *MockSemanticMemory) FindSimilar(ctx context.Context, embedding []float32, limit int, filters ...Filter) ([]*KnowledgeItem, error) {
	if m.FindSimilarFunc != nil {
		return m.FindSimilarFunc(ctx, embedding, limit, filters...)
	}
	return []*KnowledgeItem{}, nil
}

func (m *MockSemanticMemory) RecordValidation(ctx context.Context, id uuid.UUID, success bool) error {
	if m.RecordValidationFunc != nil {
		return m.RecordValidationFunc(ctx, id, success)
	}
	return nil
}

// Example of using MockSemanticMemory in tests
func TestMockSemanticMemory(t *testing.T) {
	mock := &MockSemanticMemory{
		StoreFunc: func(ctx context.Context, item *KnowledgeItem) error {
			item.ID = uuid.New()
			return nil
		},
		FindSimilarFunc: func(ctx context.Context, embedding []float32, limit int, filters ...Filter) ([]*KnowledgeItem, error) {
			return []*KnowledgeItem{
				{
					ID:         uuid.New(),
					Type:       KnowledgePattern,
					Content:    "Test pattern",
					Confidence: 0.8,
				},
			}, nil
		},
	}

	ctx := context.Background()

	// Test Store
	item := &KnowledgeItem{
		Type:    KnowledgePattern,
		Content: "New pattern",
	}
	err := mock.Store(ctx, item)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, item.ID)

	// Test FindSimilar
	embedding := make([]float32, 1536)
	results, err := mock.FindSimilar(ctx, embedding, 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, KnowledgePattern, results[0].Type)
}

// Helper function
func ptrTime(t time.Time) *time.Time {
	return &t
}
