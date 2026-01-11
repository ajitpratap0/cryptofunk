package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// SlackAlerter sends alerts via Slack webhook
type SlackAlerter struct {
	config     SlackConfig
	httpClient *http.Client
}

// SlackConfig holds Slack alerter configuration
type SlackConfig struct {
	WebhookURL     string
	Channel        string // Optional channel override
	Username       string
	IconEmoji      string
	IconURL        string
	TimeoutSeconds int
}

// NewSlackAlerter creates a new Slack-based alerter
func NewSlackAlerter(config SlackConfig) (*SlackAlerter, error) {
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("slack webhook URL is required")
	}

	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 30
	}

	if config.Username == "" {
		config.Username = "CryptoFunk Alerts"
	}

	if config.IconEmoji == "" {
		config.IconEmoji = ":rotating_light:"
	}

	log.Info().
		Str("channel", config.Channel).
		Str("username", config.Username).
		Msg("Slack alerter initialized")

	return &SlackAlerter{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		},
	}, nil
}

// Send sends an alert via Slack webhook
func (s *SlackAlerter) Send(ctx context.Context, alert Alert) error {
	msg := s.buildMessage(alert)

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Debug().
		Str("channel", s.config.Channel).
		Str("alert_title", alert.Title).
		Str("severity", string(alert.Severity)).
		Msg("Sent Slack alert")

	return nil
}

// slackMessage represents a Slack webhook message
type slackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
	Blocks      []slackBlock      `json:"blocks,omitempty"`
}

// slackAttachment represents a Slack message attachment
type slackAttachment struct {
	Color      string       `json:"color,omitempty"`
	Pretext    string       `json:"pretext,omitempty"`
	Title      string       `json:"title,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []slackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
	MarkdownIn []string     `json:"mrkdwn_in,omitempty"`
}

// slackField represents a field in a Slack attachment
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// slackBlock represents a Slack Block Kit block
type slackBlock struct {
	Type     string         `json:"type"`
	Text     *slackText     `json:"text,omitempty"`
	Elements []slackElement `json:"elements,omitempty"`
	Fields   []slackText    `json:"fields,omitempty"`
}

// slackText represents text in a Slack block
type slackText struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// slackElement represents an element in a Slack block
type slackElement struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// buildMessage builds a Slack message from an alert
func (s *SlackAlerter) buildMessage(alert Alert) slackMessage {
	msg := slackMessage{
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		IconURL:   s.config.IconURL,
	}

	if s.config.Channel != "" {
		msg.Channel = s.config.Channel
	}

	// Determine color based on severity
	color := s.getColorForSeverity(alert.Severity)

	// Build attachment
	attachment := slackAttachment{
		Color:      color,
		Title:      alert.Title,
		Text:       alert.Message,
		Footer:     "CryptoFunk Trading System",
		Timestamp:  alert.Timestamp.Unix(),
		MarkdownIn: []string{"text", "fields"},
	}

	// Add metadata fields
	for key, value := range alert.Metadata {
		attachment.Fields = append(attachment.Fields, slackField{
			Title: key,
			Value: fmt.Sprintf("%v", value),
			Short: true,
		})
	}

	// Add severity pretext for critical alerts
	if alert.Severity == SeverityCritical {
		attachment.Pretext = ":rotating_light: *CRITICAL ALERT*"
	}

	msg.Attachments = []slackAttachment{attachment}

	// Build rich blocks
	msg.Blocks = s.buildBlocks(alert)

	return msg
}

// buildBlocks builds Slack Block Kit blocks
func (s *SlackAlerter) buildBlocks(alert Alert) []slackBlock {
	emoji := s.getEmojiForSeverity(alert.Severity)

	blocks := []slackBlock{
		// Header
		{
			Type: "header",
			Text: &slackText{
				Type:  "plain_text",
				Text:  emoji + " " + alert.Title,
				Emoji: true,
			},
		},
		// Message section
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: alert.Message,
			},
		},
	}

	// Add metadata fields section
	if len(alert.Metadata) > 0 {
		fields := make([]slackText, 0, len(alert.Metadata))
		for key, value := range alert.Metadata {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:*\n%v", key, value),
			})
		}

		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fields,
		})
	}

	// Add divider
	blocks = append(blocks, slackBlock{
		Type: "divider",
	})

	// Add context
	blocks = append(blocks, slackBlock{
		Type: "context",
		Elements: []slackElement{
			{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Severity:* %s | *Time:* %s", alert.Severity, alert.Timestamp.Format("2006-01-02 15:04:05 MST")),
			},
		},
	})

	return blocks
}

// getColorForSeverity returns the color for a severity level
func (s *SlackAlerter) getColorForSeverity(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "#dc3545" // Red
	case SeverityWarning:
		return "#ffc107" // Yellow
	case SeverityInfo:
		return "#17a2b8" // Cyan
	default:
		return "#007bff" // Blue
	}
}

// getEmojiForSeverity returns an emoji for a severity level
func (s *SlackAlerter) getEmojiForSeverity(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return ":rotating_light:"
	case SeverityWarning:
		return ":warning:"
	case SeverityInfo:
		return ":information_source:"
	default:
		return ":bell:"
	}
}

// SetChannel sets the default channel
func (s *SlackAlerter) SetChannel(channel string) {
	s.config.Channel = channel
}

// GetChannel returns the default channel
func (s *SlackAlerter) GetChannel() string {
	return s.config.Channel
}
