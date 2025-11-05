//nolint:goconst // Message role types are LLM API constants
package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ConversationMemory manages multi-turn conversations with LLMs
// Stores message history, reasoning chains, and context for complex decisions
type ConversationMemory struct {
	id          uuid.UUID
	agentName   string
	messages    []ChatMessage
	metadata    map[string]interface{}
	maxTokens   int
	maxMessages int
	createdAt   time.Time
	updatedAt   time.Time
	mu          sync.RWMutex
}

// ConversationConfig configures a conversation
type ConversationConfig struct {
	AgentName   string
	MaxTokens   int // Maximum total tokens in conversation (default: 4000)
	MaxMessages int // Maximum number of messages to retain (default: 20)
	Metadata    map[string]interface{}
}

// DefaultConversationConfig returns sensible defaults
func DefaultConversationConfig(agentName string) ConversationConfig {
	return ConversationConfig{
		AgentName:   agentName,
		MaxTokens:   4000, // Conservative limit for most models
		MaxMessages: 20,   // Keeps last 20 messages
		Metadata:    make(map[string]interface{}),
	}
}

// NewConversation creates a new conversation with the given configuration
func NewConversation(config ConversationConfig) *ConversationMemory {
	if config.MaxTokens == 0 {
		config.MaxTokens = 4000
	}
	if config.MaxMessages == 0 {
		config.MaxMessages = 20
	}
	if config.Metadata == nil {
		config.Metadata = make(map[string]interface{})
	}

	now := time.Now()
	return &ConversationMemory{
		id:          uuid.New(),
		agentName:   config.AgentName,
		messages:    make([]ChatMessage, 0),
		metadata:    config.Metadata,
		maxTokens:   config.MaxTokens,
		maxMessages: config.MaxMessages,
		createdAt:   now,
		updatedAt:   now,
	}
}

// AddMessage adds a message to the conversation
func (cm *ConversationMemory) AddMessage(role, content string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	message := ChatMessage{
		Role:    role,
		Content: content,
	}

	cm.messages = append(cm.messages, message)
	cm.updatedAt = time.Now()

	// Trim old messages if we exceed max
	if len(cm.messages) > cm.maxMessages {
		// Keep system messages and trim oldest user/assistant messages
		cm.trimMessages()
	}

	log.Debug().
		Str("conversation_id", cm.id.String()).
		Str("agent", cm.agentName).
		Str("role", role).
		Int("total_messages", len(cm.messages)).
		Msg("Added message to conversation")
}

// AddSystemMessage adds a system message to the conversation
func (cm *ConversationMemory) AddSystemMessage(content string) {
	cm.AddMessage("system", content)
}

// AddUserMessage adds a user message to the conversation
func (cm *ConversationMemory) AddUserMessage(content string) {
	cm.AddMessage("user", content)
}

// AddAssistantMessage adds an assistant message to the conversation
func (cm *ConversationMemory) AddAssistantMessage(content string) {
	cm.AddMessage("assistant", content)
}

// AddThought adds an agent's internal reasoning as metadata
// This allows tracking the agent's thought process without sending to LLM
func (cm *ConversationMemory) AddThought(thought string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	thoughts, ok := cm.metadata["thoughts"].([]string)
	if !ok {
		thoughts = make([]string, 0)
	}

	thoughts = append(thoughts, fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), thought))
	cm.metadata["thoughts"] = thoughts

	log.Debug().
		Str("conversation_id", cm.id.String()).
		Str("agent", cm.agentName).
		Str("thought", thought).
		Msg("Added thought to conversation")
}

// GetMessages returns all messages in the conversation
func (cm *ConversationMemory) GetMessages() []ChatMessage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modification
	messagesCopy := make([]ChatMessage, len(cm.messages))
	copy(messagesCopy, cm.messages)
	return messagesCopy
}

// GetMessagesForPrompt returns messages formatted for LLM prompts
// Applies token limiting and returns only messages that fit
func (cm *ConversationMemory) GetMessagesForPrompt() []ChatMessage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Estimate tokens (rough approximation: 4 chars = 1 token)
	totalTokens := 0
	validMessages := make([]ChatMessage, 0)

	// Always include system messages first
	systemMessages := make([]ChatMessage, 0)
	otherMessages := make([]ChatMessage, 0)

	for _, msg := range cm.messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Add system messages (always included)
	for _, msg := range systemMessages {
		tokens := estimateTokens(msg.Content)
		totalTokens += tokens
		validMessages = append(validMessages, msg)
	}

	// Collect other messages that fit within token limit (from oldest that fit)
	otherValidMessages := make([]ChatMessage, 0)
	for i := len(otherMessages) - 1; i >= 0; i-- {
		msg := otherMessages[i]
		tokens := estimateTokens(msg.Content)

		if totalTokens+tokens > cm.maxTokens {
			log.Debug().
				Str("conversation_id", cm.id.String()).
				Int("total_tokens", totalTokens).
				Int("max_tokens", cm.maxTokens).
				Msg("Reached token limit, truncating conversation history")
			break
		}

		totalTokens += tokens
		// Collect messages (will reverse order later)
		otherValidMessages = append(otherValidMessages, msg)
	}

	// Reverse otherValidMessages to restore chronological order and append after system messages
	for i := len(otherValidMessages) - 1; i >= 0; i-- {
		validMessages = append(validMessages, otherValidMessages[i])
	}

	return validMessages
}

// GetThoughts returns all agent thoughts
func (cm *ConversationMemory) GetThoughts() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	thoughts, ok := cm.metadata["thoughts"].([]string)
	if !ok {
		return []string{}
	}

	// Return a copy
	thoughtsCopy := make([]string, len(thoughts))
	copy(thoughtsCopy, thoughts)
	return thoughtsCopy
}

// GetMetadata returns conversation metadata
func (cm *ConversationMemory) GetMetadata() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a shallow copy
	metaCopy := make(map[string]interface{})
	for k, v := range cm.metadata {
		metaCopy[k] = v
	}
	return metaCopy
}

// SetMetadata sets a metadata key-value pair
func (cm *ConversationMemory) SetMetadata(key string, value interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.metadata[key] = value
	cm.updatedAt = time.Now()
}

// Clear clears all messages but retains metadata
func (cm *ConversationMemory) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = make([]ChatMessage, 0)
	cm.updatedAt = time.Now()

	log.Info().
		Str("conversation_id", cm.id.String()).
		Str("agent", cm.agentName).
		Msg("Cleared conversation history")
}

// Reset resets the entire conversation including metadata
func (cm *ConversationMemory) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = make([]ChatMessage, 0)
	cm.metadata = make(map[string]interface{})
	cm.updatedAt = time.Now()

	log.Info().
		Str("conversation_id", cm.id.String()).
		Str("agent", cm.agentName).
		Msg("Reset conversation completely")
}

// GetID returns the conversation ID
func (cm *ConversationMemory) GetID() uuid.UUID {
	return cm.id
}

// GetAgentName returns the agent name
func (cm *ConversationMemory) GetAgentName() string {
	return cm.agentName
}

// GetMessageCount returns the number of messages
func (cm *ConversationMemory) GetMessageCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.messages)
}

// GetCreatedAt returns the creation timestamp
func (cm *ConversationMemory) GetCreatedAt() time.Time {
	return cm.createdAt
}

// GetUpdatedAt returns the last update timestamp
func (cm *ConversationMemory) GetUpdatedAt() time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.updatedAt
}

// trimMessages removes oldest messages while keeping system messages
// Call with lock held
func (cm *ConversationMemory) trimMessages() {
	systemMessages := make([]ChatMessage, 0)
	otherMessages := make([]ChatMessage, 0)

	for _, msg := range cm.messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Keep most recent non-system messages
	maxOther := cm.maxMessages - len(systemMessages)
	if maxOther < 1 {
		maxOther = 1 // Always keep at least 1 non-system message
	}

	if len(otherMessages) > maxOther {
		startIdx := len(otherMessages) - maxOther
		otherMessages = otherMessages[startIdx:]
	}

	// Rebuild messages array
	cm.messages = make([]ChatMessage, 0, len(systemMessages)+len(otherMessages))
	cm.messages = append(cm.messages, systemMessages...)
	cm.messages = append(cm.messages, otherMessages...)

	log.Debug().
		Str("conversation_id", cm.id.String()).
		Int("system_messages", len(systemMessages)).
		Int("other_messages", len(otherMessages)).
		Msg("Trimmed conversation messages")
}

// estimateTokens provides rough token estimation (4 chars ≈ 1 token)
func estimateTokens(text string) int {
	return len(text) / 4
}

// ConversationManager manages multiple conversations for different agents/sessions
type ConversationManager struct {
	conversations map[string]*ConversationMemory // Key: agent_name or conversation_id
	mu            sync.RWMutex
}

// NewConversationManager creates a new conversation manager
func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		conversations: make(map[string]*ConversationMemory),
	}
}

// CreateConversation creates and stores a new conversation
func (cm *ConversationManager) CreateConversation(config ConversationConfig) *ConversationMemory {
	conv := NewConversation(config)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Store by both agent name and conversation ID
	cm.conversations[config.AgentName] = conv
	cm.conversations[conv.id.String()] = conv

	log.Info().
		Str("conversation_id", conv.id.String()).
		Str("agent", config.AgentName).
		Msg("Created new conversation")

	return conv
}

// GetConversation retrieves a conversation by agent name or ID
func (cm *ConversationManager) GetConversation(key string) (*ConversationMemory, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conv, exists := cm.conversations[key]
	if !exists {
		return nil, fmt.Errorf("conversation not found for key: %s", key)
	}

	return conv, nil
}

// GetOrCreateConversation gets existing or creates new conversation for agent
func (cm *ConversationManager) GetOrCreateConversation(agentName string) *ConversationMemory {
	cm.mu.RLock()
	conv, exists := cm.conversations[agentName]
	cm.mu.RUnlock()

	if exists {
		return conv
	}

	// Create new conversation
	config := DefaultConversationConfig(agentName)
	return cm.CreateConversation(config)
}

// RemoveConversation removes a conversation from the manager
func (cm *ConversationManager) RemoveConversation(key string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conv, exists := cm.conversations[key]
	if !exists {
		return fmt.Errorf("conversation not found for key: %s", key)
	}

	// Remove both agent name and ID references
	delete(cm.conversations, conv.agentName)
	delete(cm.conversations, conv.id.String())

	log.Info().
		Str("conversation_id", conv.id.String()).
		Str("agent", conv.agentName).
		Msg("Removed conversation")

	return nil
}

// ListConversations returns all active conversations
func (cm *ConversationManager) ListConversations() []*ConversationMemory {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Use map to deduplicate (same conversation stored under agent name and ID)
	seen := make(map[uuid.UUID]bool)
	conversations := make([]*ConversationMemory, 0)

	for _, conv := range cm.conversations {
		if !seen[conv.id] {
			seen[conv.id] = true
			conversations = append(conversations, conv)
		}
	}

	return conversations
}

// CompleteWithConversation sends messages with conversation context
// This is a helper method that integrates with existing LLM clients
func CompleteWithConversation(ctx context.Context, client LLMClient, conv *ConversationMemory, userMessage string) (*ChatResponse, error) {
	// Add user message to conversation
	conv.AddUserMessage(userMessage)

	// Get messages for prompt (with token limiting)
	messages := conv.GetMessagesForPrompt()

	// Send to LLM
	resp, err := client.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Add assistant response to conversation
	if len(resp.Choices) > 0 {
		conv.AddAssistantMessage(resp.Choices[0].Message.Content)
	}

	return resp, nil
}
