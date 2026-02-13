package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackChannelConfig holds Slack webhook configuration
type SlackChannelConfig struct {
	WebhookURL string
	HTTPClient *http.Client
}

// SlackChannel sends notifications via Slack webhook
type SlackChannel struct {
	config SlackChannelConfig
	client *http.Client
}

// NewSlackChannel creates a new Slack notification channel
func NewSlackChannel(cfg SlackChannelConfig) *SlackChannel {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &SlackChannel{config: cfg, client: client}
}

func (s *SlackChannel) Name() string { return "slack" }
func (s *SlackChannel) Close() error { return nil }

// Send delivers a notification event via Slack webhook
func (s *SlackChannel) Send(ctx context.Context, event Event) error {
	payload := s.buildPayload(event)
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}

type slackPayload struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color    string       `json:"color"`
	Title    string       `json:"title"`
	Text     string       `json:"text"`
	Fields   []slackField `json:"fields,omitempty"`
	Footer   string       `json:"footer"`
	Ts       int64        `json:"ts"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func (s *SlackChannel) buildPayload(event Event) slackPayload {
	var fields []slackField
	for k, v := range event.Fields {
		fields = append(fields, slackField{Title: k, Value: v, Short: true})
	}

	return slackPayload{
		Attachments: []slackAttachment{
			{
				Color:  slackColor(event.Priority),
				Title:  event.Priority.String() + ": " + event.Title,
				Text:   event.Message,
				Fields: fields,
				Footer: "CryptoFunk | " + string(event.Type),
				Ts:     event.Timestamp.Unix(),
			},
		},
	}
}

func slackColor(p Priority) string {
	switch p {
	case PriorityInfo:
		return "#36a64f" // green
	case PriorityWarning:
		return "#ffcc00" // yellow
	case PriorityCritical:
		return "#ff0000" // red
	case PriorityEmergency:
		return "#9C27B0" // purple
	default:
		return "#808080"
	}
}
