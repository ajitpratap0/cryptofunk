package notifications

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

// Slack color constants for message attachments
const (
	colorRed    = "#dc3545"
	colorGreen  = "#28a745"
	colorYellow = "#ffc107"
	colorOrange = "#fd7e14"
	colorCyan   = "#17a2b8"
	colorGray   = "#6c757d"
	colorBlue   = "#007bff"
)

// SlackConfig holds Slack webhook configuration
type SlackConfig struct {
	// WebhookURL is the Slack incoming webhook URL
	WebhookURL string `mapstructure:"webhook_url"`

	// Channel overrides the default channel (optional, use webhook default if empty)
	Channel string `mapstructure:"channel"`

	// Username for the bot (optional)
	Username string `mapstructure:"username"`

	// IconEmoji for the bot (optional, e.g., ":robot_face:")
	IconEmoji string `mapstructure:"icon_emoji"`

	// IconURL for the bot (optional, overrides IconEmoji)
	IconURL string `mapstructure:"icon_url"`

	// Rate limiting
	RateLimitPerMinute int `mapstructure:"rate_limit_per_minute"` // Max messages per minute (0 = unlimited)
	RetryAttempts      int `mapstructure:"retry_attempts"`        // Number of retry attempts on failure
	RetryDelaySeconds  int `mapstructure:"retry_delay_seconds"`   // Delay between retries

	// Timeout for HTTP requests
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

// DefaultSlackConfig returns default Slack configuration
func DefaultSlackConfig() SlackConfig {
	return SlackConfig{
		Username:           "CryptoFunk Bot",
		IconEmoji:          ":chart_with_upwards_trend:",
		RateLimitPerMinute: 30,
		RetryAttempts:      3,
		RetryDelaySeconds:  2,
		TimeoutSeconds:     30,
	}
}

// SlackBackend implements the Backend interface for sending Slack notifications
type SlackBackend struct {
	config      SlackConfig
	httpClient  *http.Client
	mock        bool
	rateLimiter chan struct{}
	stopRefill  chan struct{} // Channel to signal rate limiter goroutine shutdown
}

// SlackMessage represents a Slack webhook message
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
	Blocks      []SlackBlock      `json:"blocks,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color      string       `json:"color,omitempty"`
	Pretext    string       `json:"pretext,omitempty"`
	Title      string       `json:"title,omitempty"`
	TitleLink  string       `json:"title_link,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []SlackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
	MarkdownIn []string     `json:"mrkdwn_in,omitempty"`
}

// SlackField represents a field in a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// SlackBlock represents a Slack Block Kit block
type SlackBlock struct {
	Type     string           `json:"type"`
	Text     *SlackBlockText  `json:"text,omitempty"`
	Elements []SlackElement   `json:"elements,omitempty"`
	Fields   []SlackBlockText `json:"fields,omitempty"`
}

// SlackBlockText represents text in a Slack block
type SlackBlockText struct {
	Type  string `json:"type"` // "plain_text" or "mrkdwn"
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// SlackElement represents an element in a Slack block
type SlackElement struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewSlackBackend creates a new Slack notification backend
// If webhook URL is empty, creates a mock backend
func NewSlackBackend(config SlackConfig) (*SlackBackend, error) {
	// Validate configuration
	if config.WebhookURL == "" {
		log.Warn().Msg("No Slack webhook URL provided, using mock backend")
		return &SlackBackend{
			config: config,
			mock:   true,
		}, nil
	}

	// Set defaults
	if config.RateLimitPerMinute <= 0 {
		config.RateLimitPerMinute = 30
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 30
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelaySeconds <= 0 {
		config.RetryDelaySeconds = 2
	}

	backend := &SlackBackend{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		mock:        false,
		rateLimiter: make(chan struct{}, config.RateLimitPerMinute),
		stopRefill:  make(chan struct{}),
	}

	// Fill rate limiter bucket
	go backend.refillRateLimiter()

	log.Info().
		Str("channel", config.Channel).
		Str("username", config.Username).
		Msg("Initialized Slack notification backend")

	return backend, nil
}

// refillRateLimiter refills the rate limiter bucket every minute
func (s *SlackBackend) refillRateLimiter() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Refill the bucket
		refillLoop:
			for i := 0; i < s.config.RateLimitPerMinute; i++ {
				select {
				case s.rateLimiter <- struct{}{}:
				default:
					// Bucket is full
					break refillLoop
				}
			}
		case <-s.stopRefill:
			return
		}
	}
}

// Send sends a notification via Slack webhook
func (s *SlackBackend) Send(ctx context.Context, deviceToken string, notification Notification) error {
	// deviceToken for Slack backend can be a channel override
	channel := s.config.Channel
	if deviceToken != "" && deviceToken[0] == '#' {
		channel = deviceToken
	}

	if s.mock {
		return s.sendMock(channel, notification)
	}

	// Check rate limit
	select {
	case <-s.rateLimiter:
		// Got a token, proceed
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("slack rate limit exceeded")
	}

	// Build Slack message
	msg := s.buildMessage(channel, notification)

	// Send with retries
	var lastErr error
	for attempt := 1; attempt <= s.config.RetryAttempts; attempt++ {
		if err := s.sendWebhook(ctx, msg); err != nil {
			lastErr = err
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max_attempts", s.config.RetryAttempts).
				Msg("Failed to send Slack message, retrying")

			if attempt < s.config.RetryAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(s.config.RetryDelaySeconds) * time.Second):
					continue
				}
			}
		} else {
			log.Debug().
				Str("channel", channel).
				Str("notification_type", string(notification.Type)).
				Msg("Sent Slack notification")
			return nil
		}
	}

	return fmt.Errorf("failed to send Slack message after %d attempts: %w", s.config.RetryAttempts, lastErr)
}

// sendWebhook sends a message to the Slack webhook
func (s *SlackBackend) sendWebhook(ctx context.Context, msg SlackMessage) error {
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
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildMessage builds a Slack message from a notification
func (s *SlackBackend) buildMessage(channel string, notification Notification) SlackMessage {
	msg := SlackMessage{
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		IconURL:   s.config.IconURL,
	}

	if channel != "" {
		msg.Channel = channel
	}

	// Determine color based on notification type
	color := s.getColorForType(notification.Type)

	// Build attachment with fields
	attachment := SlackAttachment{
		Color:      color,
		Title:      notification.Title,
		Text:       notification.Body,
		Footer:     "CryptoFunk Trading System",
		FooterIcon: "https://example.com/cryptofunk-icon.png", // Replace with actual icon URL
		Timestamp:  time.Now().Unix(),
		MarkdownIn: []string{"text", "fields"},
	}

	// Add data fields
	for key, value := range notification.Data {
		attachment.Fields = append(attachment.Fields, SlackField{
			Title: formatDataKey(key),
			Value: value,
			Short: len(value) < 20, // Short fields if value is small
		})
	}

	// Add priority indicator
	if notification.Priority == priorityHigh {
		attachment.Pretext = ":rotating_light: *HIGH PRIORITY ALERT*"
	}

	msg.Attachments = []SlackAttachment{attachment}

	// Add contextual blocks for rich formatting
	msg.Blocks = s.buildBlocks(notification)

	return msg
}

// buildBlocks builds Slack Block Kit blocks for rich formatting
func (s *SlackBackend) buildBlocks(notification Notification) []SlackBlock {
	blocks := []SlackBlock{
		// Header
		{
			Type: "header",
			Text: &SlackBlockText{
				Type:  "plain_text",
				Text:  s.getEmojiForType(notification.Type) + " " + notification.Title,
				Emoji: true,
			},
		},
		// Body section
		{
			Type: "section",
			Text: &SlackBlockText{
				Type: "mrkdwn",
				Text: notification.Body,
			},
		},
	}

	// Add fields section if there's data
	if len(notification.Data) > 0 {
		fields := make([]SlackBlockText, 0, len(notification.Data))
		for key, value := range notification.Data {
			fields = append(fields, SlackBlockText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:*\n%s", formatDataKey(key), value),
			})
		}

		blocks = append(blocks, SlackBlock{
			Type:   "section",
			Fields: fields,
		})
	}

	// Add divider
	blocks = append(blocks, SlackBlock{
		Type: "divider",
	})

	// Add context
	blocks = append(blocks, SlackBlock{
		Type: "context",
		Elements: []SlackElement{
			{
				Type: "mrkdwn",
				Text: fmt.Sprintf("_%s_ | CryptoFunk Trading System", time.Now().Format("2006-01-02 15:04:05 MST")),
			},
		},
	})

	return blocks
}

// getColorForType returns the attachment color for a notification type
func (s *SlackBackend) getColorForType(notifType NotificationType) string {
	switch notifType {
	case NotificationTypeTradeExecution:
		return colorGreen
	case NotificationTypePnLAlert:
		return colorYellow
	case NotificationTypeCircuitBreaker:
		return colorRed
	case NotificationTypeConsensusFailure:
		return colorOrange
	case NotificationTypePositionClosed:
		return colorCyan
	case NotificationTypeSafetyGuard:
		return colorRed
	case NotificationTypeSystemError:
		return colorRed
	case NotificationTypeDailySummary:
		return colorGray
	default:
		return colorBlue
	}
}

// getEmojiForType returns an emoji for a notification type
func (s *SlackBackend) getEmojiForType(notifType NotificationType) string {
	switch notifType {
	case NotificationTypeTradeExecution:
		return ":chart_with_upwards_trend:"
	case NotificationTypePnLAlert:
		return ":moneybag:"
	case NotificationTypeCircuitBreaker:
		return ":rotating_light:"
	case NotificationTypeConsensusFailure:
		return ":warning:"
	case NotificationTypePositionClosed:
		return ":checkered_flag:"
	case NotificationTypeSafetyGuard:
		return ":shield:"
	case NotificationTypeSystemError:
		return ":x:"
	case NotificationTypeDailySummary:
		return ":clipboard:"
	default:
		return ":bell:"
	}
}

// sendMock logs the Slack message instead of sending it
func (s *SlackBackend) sendMock(channel string, notification Notification) error {
	log.Info().
		Str("backend", "slack_mock").
		Str("channel", channel).
		Str("notification_type", string(notification.Type)).
		Str("title", notification.Title).
		Str("body", notification.Body).
		Str("priority", notification.Priority).
		Msg("Mock Slack notification (not actually sent)")

	return nil
}

// Name returns the backend name
func (s *SlackBackend) Name() string {
	if s.mock {
		return "slack_mock"
	}
	return "slack"
}

// Close closes the Slack backend
func (s *SlackBackend) Close() error {
	// Stop the rate limiter goroutine if running (non-mock backends)
	if s.stopRefill != nil {
		close(s.stopRefill)
	}
	log.Debug().Str("backend", s.Name()).Msg("Closed Slack backend")
	return nil
}

// IsMock returns true if this is a mock backend
func (s *SlackBackend) IsMock() bool {
	return s.mock
}

// SetChannel sets the default channel
func (s *SlackBackend) SetChannel(channel string) {
	s.config.Channel = channel
}

// GetChannel returns the default channel
func (s *SlackBackend) GetChannel() string {
	return s.config.Channel
}
