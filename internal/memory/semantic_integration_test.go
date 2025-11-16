package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db/testhelpers"
	"github.com/ajitpratap0/cryptofunk/internal/memory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSemanticMemory_Store tests storing knowledge items
func TestSemanticMemory_Store(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Create test knowledge item
	item := &memory.KnowledgeItem{
		Type:       memory.KnowledgeFact,
		Content:    "BTC price tends to rise after halving events",
		Embedding:  generateTestEmbedding(),
		Confidence: 0.85,
		Importance: 0.9,
		Source:     "manual",
		AgentName:  "trend-agent",
		Context:    []byte(`{"market":"bull"}`),
	}

	// Store the item
	err = sm.Store(ctx, item)
	require.NoError(t, err)

	// Verify ID and timestamps were set
	assert.NotEqual(t, uuid.Nil, item.ID)
	assert.False(t, item.CreatedAt.IsZero())
	assert.False(t, item.UpdatedAt.IsZero())
}

// TestSemanticMemory_FindSimilar tests vector similarity search
func TestSemanticMemory_FindSimilar(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store multiple knowledge items
	items := []*memory.KnowledgeItem{
		{
			Type:       memory.KnowledgePattern,
			Content:    "RSI > 70 often leads to price correction",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "pattern_extraction",
			AgentName:  "technical-agent",
		},
		{
			Type:       memory.KnowledgePattern,
			Content:    "MACD crossover signals trend reversal",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.75,
			Importance: 0.8,
			Source:     "pattern_extraction",
			AgentName:  "technical-agent",
		},
		{
			Type:       memory.KnowledgeExperience,
			Content:    "Stop losses at 2% work better than 5%",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.9,
			Importance: 0.85,
			Source:     "backtest",
			AgentName:  "risk-agent",
		},
	}

	for _, item := range items {
		err := sm.Store(ctx, item)
		require.NoError(t, err)
	}

	// Find similar items
	queryEmbedding := generateTestEmbedding()
	similar, err := sm.FindSimilar(ctx, queryEmbedding, 10)
	require.NoError(t, err)

	// Should find all 3 items
	assert.Len(t, similar, 3)
}

// TestSemanticMemory_FindByType tests filtering by knowledge type
func TestSemanticMemory_FindByType(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store items of different types
	types := []memory.KnowledgeType{
		memory.KnowledgeFact,
		memory.KnowledgePattern,
		memory.KnowledgeFact,
		memory.KnowledgeExperience,
		memory.KnowledgeFact,
	}

	for i, knowledgeType := range types {
		item := &memory.KnowledgeItem{
			Type:       knowledgeType,
			Content:    "Test knowledge " + string(knowledgeType),
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  "test-agent",
		}
		err := sm.Store(ctx, item)
		require.NoError(t, err, "Failed to store item %d", i)
	}

	// Find only facts
	facts, err := sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	assert.Len(t, facts, 3)

	for _, fact := range facts {
		assert.Equal(t, memory.KnowledgeFact, fact.Type)
	}

	// Find only patterns
	patterns, err := sm.FindByType(ctx, memory.KnowledgePattern, 10)
	require.NoError(t, err)
	assert.Len(t, patterns, 1)
}

// TestSemanticMemory_FindByAgent tests filtering by agent name
func TestSemanticMemory_FindByAgent(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store items from different agents
	agents := []string{"technical-agent", "risk-agent", "technical-agent", "trend-agent"}
	for _, agentName := range agents {
		item := &memory.KnowledgeItem{
			Type:       memory.KnowledgeFact,
			Content:    "Knowledge from " + agentName,
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  agentName,
		}
		err := sm.Store(ctx, item)
		require.NoError(t, err)
	}

	// Find items from technical-agent
	techItems, err := sm.FindByAgent(ctx, "technical-agent", 10)
	require.NoError(t, err)
	assert.Len(t, techItems, 2)

	for _, item := range techItems {
		assert.Equal(t, "technical-agent", item.AgentName)
	}
}

// TestSemanticMemory_GetMostRelevant tests relevance-based retrieval
func TestSemanticMemory_GetMostRelevant(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store items with different relevance scores
	now := time.Now()
	items := []*memory.KnowledgeItem{
		{
			Type:            memory.KnowledgeFact,
			Content:         "Old low-confidence knowledge",
			Embedding:       generateTestEmbedding(),
			Confidence:      0.3,
			Importance:      0.4,
			Source:          "test",
			AgentName:       "test-agent",
			ValidationCount: 10,
			SuccessCount:    3,
			FailureCount:    7,
			CreatedAt:       now.Add(-60 * 24 * time.Hour), // 60 days old
		},
		{
			Type:            memory.KnowledgeFact,
			Content:         "Recent high-confidence knowledge",
			Embedding:       generateTestEmbedding(),
			Confidence:      0.95,
			Importance:      0.9,
			Source:          "test",
			AgentName:       "test-agent",
			ValidationCount: 20,
			SuccessCount:    18,
			FailureCount:    2,
			CreatedAt:       now.Add(-1 * 24 * time.Hour), // 1 day old
		},
		{
			Type:            memory.KnowledgeFact,
			Content:         "Medium relevance knowledge",
			Embedding:       generateTestEmbedding(),
			Confidence:      0.7,
			Importance:      0.6,
			Source:          "test",
			AgentName:       "test-agent",
			ValidationCount: 10,
			SuccessCount:    7,
			FailureCount:    3,
			CreatedAt:       now.Add(-15 * 24 * time.Hour), // 15 days old
		},
	}

	for _, item := range items {
		item.UpdatedAt = item.CreatedAt
		err := sm.Store(ctx, item)
		require.NoError(t, err)
	}

	// Get most relevant (should prioritize high confidence + recency + success rate)
	relevant, err := sm.GetMostRelevant(ctx, 10)
	require.NoError(t, err)
	require.Len(t, relevant, 3)

	// First item should be the recent high-confidence one
	assert.Equal(t, "Recent high-confidence knowledge", relevant[0].Content)
}

// TestSemanticMemory_RecordAccess tests access tracking
func TestSemanticMemory_RecordAccess(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store an item
	item := &memory.KnowledgeItem{
		Type:       memory.KnowledgeFact,
		Content:    "Test knowledge",
		Embedding:  generateTestEmbedding(),
		Confidence: 0.8,
		Importance: 0.7,
		Source:     "test",
		AgentName:  "test-agent",
	}
	err = sm.Store(ctx, item)
	require.NoError(t, err)

	initialAccessCount := item.AccessCount

	// Record access
	err = sm.RecordAccess(ctx, item.ID)
	require.NoError(t, err)

	// Retrieve and verify access count increased
	items, err := sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, initialAccessCount+1, items[0].AccessCount)
}

// TestSemanticMemory_RecordValidation tests validation tracking
func TestSemanticMemory_RecordValidation(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store an item
	item := &memory.KnowledgeItem{
		Type:       memory.KnowledgePattern,
		Content:    "Test pattern",
		Embedding:  generateTestEmbedding(),
		Confidence: 0.8,
		Importance: 0.7,
		Source:     "test",
		AgentName:  "test-agent",
	}
	err = sm.Store(ctx, item)
	require.NoError(t, err)

	// Record successful validation
	err = sm.RecordValidation(ctx, item.ID, true)
	require.NoError(t, err)

	// Verify counts
	items, err := sm.FindByType(ctx, memory.KnowledgePattern, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 1, items[0].ValidationCount)
	assert.Equal(t, 1, items[0].SuccessCount)
	assert.Equal(t, 0, items[0].FailureCount)

	// Record failed validation
	err = sm.RecordValidation(ctx, item.ID, false)
	require.NoError(t, err)

	// Verify counts updated
	items, err = sm.FindByType(ctx, memory.KnowledgePattern, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 2, items[0].ValidationCount)
	assert.Equal(t, 1, items[0].SuccessCount)
	assert.Equal(t, 1, items[0].FailureCount)
}

// TestSemanticMemory_UpdateConfidence tests confidence updates
func TestSemanticMemory_UpdateConfidence(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store an item
	item := &memory.KnowledgeItem{
		Type:       memory.KnowledgeFact,
		Content:    "Test fact",
		Embedding:  generateTestEmbedding(),
		Confidence: 0.5,
		Importance: 0.7,
		Source:     "test",
		AgentName:  "test-agent",
	}
	err = sm.Store(ctx, item)
	require.NoError(t, err)

	// Update confidence
	newConfidence := 0.85
	err = sm.UpdateConfidence(ctx, item.ID, newConfidence)
	require.NoError(t, err)

	// Verify update
	items, err := sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, newConfidence, items[0].Confidence)
}

// TestSemanticMemory_Delete tests deleting knowledge
func TestSemanticMemory_Delete(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store two items
	item1 := &memory.KnowledgeItem{
		Type:       memory.KnowledgeFact,
		Content:    "Fact 1",
		Embedding:  generateTestEmbedding(),
		Confidence: 0.8,
		Importance: 0.7,
		Source:     "test",
		AgentName:  "test-agent",
	}
	err = sm.Store(ctx, item1)
	require.NoError(t, err)

	item2 := &memory.KnowledgeItem{
		Type:       memory.KnowledgeFact,
		Content:    "Fact 2",
		Embedding:  generateTestEmbedding(),
		Confidence: 0.8,
		Importance: 0.7,
		Source:     "test",
		AgentName:  "test-agent",
	}
	err = sm.Store(ctx, item2)
	require.NoError(t, err)

	// Verify both exist
	items, err := sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Delete first item
	err = sm.Delete(ctx, item1.ID)
	require.NoError(t, err)

	// Verify only one remains
	items, err = sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, item2.ID, items[0].ID)
}

// TestSemanticMemory_PruneExpired tests pruning expired knowledge
func TestSemanticMemory_PruneExpired(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	// Store items with different expiration
	items := []*memory.KnowledgeItem{
		{
			Type:       memory.KnowledgeFact,
			Content:    "Expired knowledge",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  "test-agent",
			ExpiresAt:  &pastTime, // Already expired
		},
		{
			Type:       memory.KnowledgeFact,
			Content:    "Valid knowledge",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  "test-agent",
			ExpiresAt:  &futureTime, // Still valid
		},
		{
			Type:       memory.KnowledgeFact,
			Content:    "No expiration",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  "test-agent",
			ExpiresAt:  nil, // Never expires
		},
	}

	for _, item := range items {
		err := sm.Store(ctx, item)
		require.NoError(t, err)
	}

	// Prune expired
	deleted, err := sm.PruneExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted) // Should delete 1 expired item

	// Verify only valid items remain
	remaining, err := sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	assert.Len(t, remaining, 2)
}

// TestSemanticMemory_PruneLowQuality tests pruning low quality knowledge
func TestSemanticMemory_PruneLowQuality(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store items with different quality (success rates)
	items := []*memory.KnowledgeItem{
		{
			Type:            memory.KnowledgeFact,
			Content:         "Low quality knowledge",
			Embedding:       generateTestEmbedding(),
			Confidence:      0.8,
			Importance:      0.7,
			Source:          "test",
			AgentName:       "test-agent",
			ValidationCount: 10,
			SuccessCount:    2,  // 20% success rate
			FailureCount:    8,  // Low quality
		},
		{
			Type:            memory.KnowledgeFact,
			Content:         "High quality knowledge",
			Embedding:       generateTestEmbedding(),
			Confidence:      0.8,
			Importance:      0.7,
			Source:          "test",
			AgentName:       "test-agent",
			ValidationCount: 10,
			SuccessCount:    9,  // 90% success rate
			FailureCount:    1,  // High quality
		},
	}

	for _, item := range items {
		err := sm.Store(ctx, item)
		require.NoError(t, err)
	}

	// Prune low quality (minValidations=5, minSuccessRate=0.5)
	deleted, err := sm.PruneLowQuality(ctx, 5, 0.5)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted) // Should delete 1 low quality item

	// Verify only high quality remains
	remaining, err := sm.FindByType(ctx, memory.KnowledgeFact, 10)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "High quality knowledge", remaining[0].Content)
}

// TestSemanticMemory_GetStats tests statistics retrieval
func TestSemanticMemory_GetStats(t *testing.T) {
	tc := testhelpers.SetupTestDatabase(t)
	err := tc.ApplyMigrations("../../migrations")
	require.NoError(t, err)

	ctx := context.Background()
	sm := memory.NewSemanticMemory(tc.DB.Pool())

	// Store items of different types
	types := []memory.KnowledgeType{
		memory.KnowledgeFact,
		memory.KnowledgeFact,
		memory.KnowledgePattern,
		memory.KnowledgeExperience,
	}

	for _, knowledgeType := range types {
		item := &memory.KnowledgeItem{
			Type:       knowledgeType,
			Content:    "Test",
			Embedding:  generateTestEmbedding(),
			Confidence: 0.8,
			Importance: 0.7,
			Source:     "test",
			AgentName:  "test-agent",
		}
		err := sm.Store(ctx, item)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := sm.GetStats(ctx)
	require.NoError(t, err)

	// Verify counts (GetStats returns map[string]interface{})
	totalItems, ok := stats["total_items"].(int64)
	require.True(t, ok)
	assert.Equal(t, int64(4), totalItems)

	facts, ok := stats["facts"].(int64)
	require.True(t, ok)
	assert.Equal(t, int64(2), facts)

	patterns, ok := stats["patterns"].(int64)
	require.True(t, ok)
	assert.Equal(t, int64(1), patterns)

	experiences, ok := stats["experiences"].(int64)
	require.True(t, ok)
	assert.Equal(t, int64(1), experiences)
}

// Helper function to generate test embeddings (1536 dimensions)
func generateTestEmbedding() []float32 {
	embedding := make([]float32, 1536)
	for i := range embedding {
		// Generate varied test values to create unique embeddings
		embedding[i] = float32(i) / 1536.0
	}
	return embedding
}

// Helper to format embedding as pgvector string
func formatEmbeddingForPgvector(embedding []float32) string {
	if len(embedding) == 0 {
		return "[]"
	}

	result := "["
	for i, val := range embedding {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", val)
	}
	result += "]"
	return result
}
