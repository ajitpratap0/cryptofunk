package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConversation(t *testing.T) {
	config := ConversationConfig{
		AgentName:   "test-agent",
		MaxTokens:   2000,
		MaxMessages: 10,
	}

	conv := NewConversation(config)

	assert.NotNil(t, conv)
	assert.NotEqual(t, "", conv.GetID().String())
	assert.Equal(t, "test-agent", conv.GetAgentName())
	assert.Equal(t, 0, conv.GetMessageCount())
	assert.NotZero(t, conv.GetCreatedAt())
}

func TestDefaultConversationConfig(t *testing.T) {
	config := DefaultConversationConfig("my-agent")

	assert.Equal(t, "my-agent", config.AgentName)
	assert.Equal(t, 4000, config.MaxTokens)
	assert.Equal(t, 20, config.MaxMessages)
	assert.NotNil(t, config.Metadata)
}

func TestAddMessage(t *testing.T) {
	conv := NewConversation(DefaultConversationConfig("test"))

	conv.AddSystemMessage("You are a helpful assistant")
	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi there!")

	messages := conv.GetMessages()
	require.Len(t, messages, 3)

	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "You are a helpful assistant", messages[0].Content)

	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "Hello", messages[1].Content)

	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "Hi there!", messages[2].Content)
}

func TestAddThought(t *testing.T) {
	conv := NewConversation(DefaultConversationConfig("test"))

	conv.AddThought("Analyzing market conditions")
	conv.AddThought("Trend is bullish")
	conv.AddThought("Confidence is high")

	thoughts := conv.GetThoughts()
	require.Len(t, thoughts, 3)

	// Thoughts should contain timestamps
	assert.Contains(t, thoughts[0], "Analyzing market conditions")
	assert.Contains(t, thoughts[1], "Trend is bullish")
	assert.Contains(t, thoughts[2], "Confidence is high")
}

func TestMessageTrimming(t *testing.T) {
	config := ConversationConfig{
		AgentName:   "test",
		MaxMessages: 5,
		MaxTokens:   10000,
	}
	conv := NewConversation(config)

	// Add system message
	conv.AddSystemMessage("System prompt")

	// Add more messages than the limit
	for i := 0; i < 10; i++ {
		conv.AddUserMessage("User message")
		conv.AddAssistantMessage("Assistant message")
	}

	messages := conv.GetMessages()

	// Should have trimmed to max (5 messages total, including 1 system)
	assert.LessOrEqual(t, len(messages), 5)

	// System message should still be present
	hasSystem := false
	for _, msg := range messages {
		if msg.Role == "system" {
			hasSystem = true
			break
		}
	}
	assert.True(t, hasSystem, "System message should be retained")
}

func TestGetMessagesForPrompt(t *testing.T) {
	config := ConversationConfig{
		AgentName:   "test",
		MaxTokens:   100, // Very small to test token limiting
		MaxMessages: 20,
	}
	conv := NewConversation(config)

	conv.AddSystemMessage("Short system")

	// Add messages that will exceed token limit
	for i := 0; i < 10; i++ {
		conv.AddUserMessage("This is a user message that has some length to it")
		conv.AddAssistantMessage("This is an assistant response with similar length")
	}

	promptMessages := conv.GetMessagesForPrompt()

	// Should have fewer messages due to token limiting
	assert.Less(t, len(promptMessages), conv.GetMessageCount())

	// System message should still be included
	assert.Equal(t, "system", promptMessages[0].Role)
}

func TestGetMessagesForPrompt_SystemAlwaysIncluded(t *testing.T) {
	config := ConversationConfig{
		AgentName:   "test",
		MaxTokens:   50,
		MaxMessages: 20,
	}
	conv := NewConversation(config)

	// Add a moderately long system message
	conv.AddSystemMessage("You are a trading assistant with knowledge of technical analysis")

	// Add many user/assistant messages
	for i := 0; i < 5; i++ {
		conv.AddUserMessage("What should I do?")
		conv.AddAssistantMessage("Analyze the market")
	}

	promptMessages := conv.GetMessagesForPrompt()

	// System message should always be first
	require.Greater(t, len(promptMessages), 0)
	assert.Equal(t, "system", promptMessages[0].Role)
}

func TestMetadata(t *testing.T) {
	conv := NewConversation(DefaultConversationConfig("test"))

	conv.SetMetadata("symbol", "BTC/USDT")
	conv.SetMetadata("strategy", "trend-following")
	conv.SetMetadata("confidence", 0.85)

	metadata := conv.GetMetadata()

	assert.Equal(t, "BTC/USDT", metadata["symbol"])
	assert.Equal(t, "trend-following", metadata["strategy"])
	assert.Equal(t, 0.85, metadata["confidence"])
}

func TestClear(t *testing.T) {
	conv := NewConversation(DefaultConversationConfig("test"))

	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi")
	conv.SetMetadata("key", "value")

	assert.Equal(t, 2, conv.GetMessageCount())

	conv.Clear()

	assert.Equal(t, 0, conv.GetMessageCount())
	// Metadata should be retained
	assert.Equal(t, "value", conv.GetMetadata()["key"])
}

func TestReset(t *testing.T) {
	conv := NewConversation(DefaultConversationConfig("test"))

	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi")
	conv.SetMetadata("key", "value")

	conv.Reset()

	assert.Equal(t, 0, conv.GetMessageCount())
	assert.Empty(t, conv.GetMetadata())
}

func TestConcurrency(t *testing.T) {
	conv := NewConversation(DefaultConversationConfig("test"))

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			conv.AddUserMessage("Message")
			conv.AddThought("Thought")
			conv.SetMetadata("key", n)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have all messages
	assert.Equal(t, 10, conv.GetMessageCount())
	assert.Equal(t, 10, len(conv.GetThoughts()))
}

func TestConversationManager(t *testing.T) {
	manager := NewConversationManager()

	config1 := DefaultConversationConfig("agent1")
	conv1 := manager.CreateConversation(config1)

	config2 := DefaultConversationConfig("agent2")
	conv2 := manager.CreateConversation(config2)

	// Retrieve by agent name
	retrieved1, err := manager.GetConversation("agent1")
	require.NoError(t, err)
	assert.Equal(t, conv1.GetID(), retrieved1.GetID())

	// Retrieve by conversation ID
	retrieved2, err := manager.GetConversation(conv2.GetID().String())
	require.NoError(t, err)
	assert.Equal(t, conv2.GetID(), retrieved2.GetID())

	// List all
	conversations := manager.ListConversations()
	assert.Len(t, conversations, 2)
}

func TestConversationManager_GetOrCreate(t *testing.T) {
	manager := NewConversationManager()

	// First call creates
	conv1 := manager.GetOrCreateConversation("agent1")
	assert.NotNil(t, conv1)
	assert.Equal(t, "agent1", conv1.GetAgentName())

	// Second call retrieves existing
	conv2 := manager.GetOrCreateConversation("agent1")
	assert.Equal(t, conv1.GetID(), conv2.GetID())
}

func TestConversationManager_Remove(t *testing.T) {
	manager := NewConversationManager()

	config := DefaultConversationConfig("test")
	conv := manager.CreateConversation(config)

	// Should exist
	_, err := manager.GetConversation("test")
	require.NoError(t, err)

	// Remove
	err = manager.RemoveConversation("test")
	require.NoError(t, err)

	// Should not exist anymore
	_, err = manager.GetConversation("test")
	assert.Error(t, err)

	// Should also not be accessible by ID
	_, err = manager.GetConversation(conv.GetID().String())
	assert.Error(t, err)
}

func TestCompleteWithConversation(t *testing.T) {
	// Create mock client
	mockClient := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
			resp := &ChatResponse{
				ID:      "test-response",
				Model:   "claude-sonnet-4",
				Created: time.Now().Unix(),
			}
			resp.Choices = []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "This is a test response",
					},
					FinishReason: "stop",
				},
			}
			resp.Usage.PromptTokens = 10
			resp.Usage.CompletionTokens = 5
			resp.Usage.TotalTokens = 15
			return resp, nil
		},
	}

	conv := NewConversation(DefaultConversationConfig("test"))
	conv.AddSystemMessage("You are a helpful assistant")

	ctx := context.Background()
	resp, err := CompleteWithConversation(ctx, mockClient, conv, "Hello!")

	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Conversation should have user message and assistant response
	messages := conv.GetMessages()
	require.Len(t, messages, 3) // system + user + assistant

	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "Hello!", messages[1].Content)
	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "This is a test response", messages[2].Content)
}

func TestCompleteWithConversation_MultiTurn(t *testing.T) {
	mockClient := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
			// Echo the last user message
			var lastUserMsg string
			for _, msg := range messages {
				if msg.Role == "user" {
					lastUserMsg = msg.Content
				}
			}

			resp := &ChatResponse{
				ID:      "test-response",
				Model:   "claude-sonnet-4",
				Created: time.Now().Unix(),
			}
			resp.Choices = []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Response to: " + lastUserMsg,
					},
				},
			}
			return resp, nil
		},
	}

	conv := NewConversation(DefaultConversationConfig("test"))
	conv.AddSystemMessage("You are a helpful assistant")

	ctx := context.Background()

	// Turn 1
	resp1, err := CompleteWithConversation(ctx, mockClient, conv, "First question")
	require.NoError(t, err)
	assert.Contains(t, resp1.Choices[0].Message.Content, "First question")

	// Turn 2
	resp2, err := CompleteWithConversation(ctx, mockClient, conv, "Second question")
	require.NoError(t, err)
	assert.Contains(t, resp2.Choices[0].Message.Content, "Second question")

	// Turn 3
	resp3, err := CompleteWithConversation(ctx, mockClient, conv, "Third question")
	require.NoError(t, err)
	assert.Contains(t, resp3.Choices[0].Message.Content, "Third question")

	// Conversation should have full history
	messages := conv.GetMessages()
	assert.Equal(t, 7, len(messages)) // 1 system + 3 user + 3 assistant
}

// MockLLMClient for testing
type MockLLMClient struct {
	CompleteFunc func(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
}

func (m *MockLLMClient) Complete(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, messages)
	}
	return nil, nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	resp, err := m.Complete(ctx, messages)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

func (m *MockLLMClient) CompleteWithRetry(ctx context.Context, messages []ChatMessage, maxRetries int) (*ChatResponse, error) {
	return m.Complete(ctx, messages)
}

func (m *MockLLMClient) ParseJSONResponse(content string, target interface{}) error {
	return nil
}
