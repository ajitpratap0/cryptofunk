// Package analyzer provides LLM-based market analysis for Polymarket.
package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/news"
)

// Analysis is the output of the LLM analyzer.
type Analysis struct {
	MarketID      string  `json:"market_id"`
	Question      string  `json:"question"`
	CurrentYes    float64 `json:"current_yes_price"`
	CurrentNo     float64 `json:"current_no_price"`
	PredictedProb float64 `json:"predicted_probability"`
	Confidence    float64 `json:"confidence"`
	Reasoning     string  `json:"reasoning"`
	Action        string  `json:"action"` // BUY YES, BUY NO, SKIP
	Edge          float64 `json:"edge"`   // predicted - market price
	Articles      int     `json:"articles_analyzed"`
}

// Analyze runs LLM analysis on a market.
func Analyze(market *gamma.Market) (*Analysis, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	// Fetch news
	articles, _ := news.FetchNews(market.Question, 5)

	prompt := buildPrompt(market, articles)

	resp, err := callClaude(apiKey, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	analysis := parseResponse(resp, market)
	analysis.Articles = len(articles)
	return analysis, nil
}

func buildPrompt(market *gamma.Market, articles []news.Article) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`You are a prediction market analyst. Analyze this Polymarket question and estimate the TRUE probability of YES.

Market Question: %s
Description: %s
Current YES price: %.2f (market implied probability)
Current NO price: %.2f
End Date: %s
Volume: $%.0f

`, market.Question, market.Description, market.OutcomeYesPrice, market.OutcomeNoPrice, market.EndDate, market.Volume))

	if len(articles) > 0 {
		sb.WriteString("Recent relevant news:\n")
		for i, a := range articles {
			sb.WriteString(fmt.Sprintf("%d. %s (%s)\n   %s\n", i+1, a.Title, a.PubDate, a.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`Respond in EXACTLY this JSON format (no markdown, no extra text):
{
  "predicted_probability": 0.XX,
  "confidence": 0.XX,
  "reasoning": "brief explanation",
  "action": "BUY YES" or "BUY NO" or "SKIP"
}

Rules:
- predicted_probability: your estimate of YES happening (0.0-1.0)
- confidence: how confident you are in your estimate (0.0-1.0)
- action: BUY YES if predicted_prob > current_yes + 0.10, BUY NO if predicted_prob < current_yes - 0.10, else SKIP
- Be calibrated. Don't be overconfident.`)

	return sb.String()
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func callClaude(apiKey, prompt string) (string, error) {
	reqBody := claudeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(respBody))
	}

	var cr claudeResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return "", err
	}
	if len(cr.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return cr.Content[0].Text, nil
}

func parseResponse(resp string, market *gamma.Market) *Analysis {
	analysis := &Analysis{
		MarketID:   market.ConditionID,
		Question:   market.Question,
		CurrentYes: market.OutcomeYesPrice,
		CurrentNo:  market.OutcomeNoPrice,
		Action:     "SKIP",
	}

	// Try to extract JSON from response
	text := resp
	if idx := strings.Index(text, "{"); idx >= 0 {
		if end := strings.LastIndex(text, "}"); end >= idx {
			text = text[idx : end+1]
		}
	}

	var parsed struct {
		PredictedProb float64 `json:"predicted_probability"`
		Confidence    float64 `json:"confidence"`
		Reasoning     string  `json:"reasoning"`
		Action        string  `json:"action"`
	}

	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		analysis.PredictedProb = parsed.PredictedProb
		analysis.Confidence = parsed.Confidence
		analysis.Reasoning = parsed.Reasoning
		analysis.Action = parsed.Action
		analysis.Edge = parsed.PredictedProb - market.OutcomeYesPrice
	}

	return analysis
}
