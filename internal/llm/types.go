package llm

import "time"

// AgentType represents the type of agent
type AgentType string

const (
	AgentTypeTechnical    AgentType = "technical"
	AgentTypeTrend        AgentType = "trend"
	AgentTypeReversion    AgentType = "reversion"
	AgentTypeRisk         AgentType = "risk"
	AgentTypeOrderbook    AgentType = "orderbook"
	AgentTypeSentiment    AgentType = "sentiment"
	AgentTypeOrchestrator AgentType = "orchestrator"
)

// Decision represents an LLM-powered trading decision
type Decision struct {
	Action     string                 `json:"action"`     // "BUY", "SELL", "HOLD", "APPROVE", "REJECT"
	Confidence float64                `json:"confidence"` // 0.0 to 1.0
	Reasoning  string                 `json:"reasoning"`  // Natural language explanation
	Metadata   map[string]interface{} `json:"metadata"`   // Additional context
	Timestamp  time.Time              `json:"timestamp"`
	AgentType  AgentType              `json:"agent_type"`
}

// Signal represents a trading signal from an agent
type Signal struct {
	Symbol     string                 `json:"symbol"`
	Side       string                 `json:"side"`       // "BUY" or "SELL"
	Confidence float64                `json:"confidence"` // 0.0 to 1.0
	Reasoning  string                 `json:"reasoning"`
	Indicators map[string]float64     `json:"indicators,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// RiskAssessment represents a risk evaluation from the risk agent
type RiskAssessment struct {
	Approved        bool                   `json:"approved"`
	PositionSize    float64                `json:"position_size"`
	StopLoss        *float64               `json:"stop_loss,omitempty"`
	TakeProfit      *float64               `json:"take_profit,omitempty"`
	RiskScore       float64                `json:"risk_score"` // 0.0 to 1.0
	Reasoning       string                 `json:"reasoning"`
	Concerns        []string               `json:"concerns,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// MarketContext contains market data for LLM context
type MarketContext struct {
	Symbol         string             `json:"symbol"`
	CurrentPrice   float64            `json:"current_price"`
	PriceChange24h float64            `json:"price_change_24h"`
	Volume24h      float64            `json:"volume_24h"`
	Indicators     map[string]float64 `json:"indicators,omitempty"`
	Timestamp      time.Time          `json:"timestamp"`
}

// PositionContext contains position data for LLM context
type PositionContext struct {
	Symbol         string    `json:"symbol"`
	Side           string    `json:"side"`
	EntryPrice     float64   `json:"entry_price"`
	CurrentPrice   float64   `json:"current_price"`
	Quantity       float64   `json:"quantity"`
	UnrealizedPnL  float64   `json:"unrealized_pnl"`
	RealizedPnL    float64   `json:"realized_pnl"`
	OpenDuration   string    `json:"open_duration"`
	EntryTimestamp time.Time `json:"entry_timestamp"`
}

// HistoricalDecision contains past decision data for learning
type HistoricalDecision struct {
	Action     string    `json:"action"`
	Confidence float64   `json:"confidence"`
	Reasoning  string    `json:"reasoning"`
	Outcome    string    `json:"outcome"` // "SUCCESS", "FAILURE", "NEUTRAL"
	PnL        float64   `json:"pnl"`
	Timestamp  time.Time `json:"timestamp"`
}

// ChatRequest represents a request to the LLM API
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
}

// ChatMessage represents a single message in the chat
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatResponse represents the response from the LLM API
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ErrorResponse represents an error from the LLM API
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
