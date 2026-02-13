package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
)

// slackColor returns the Slack attachment color for a given priority
func slackColor(p Priority) string {
	switch p {
	case PriorityInfo:
		return "#36a64f"
	case PriorityWarning:
		return "#ffcc00"
	case PriorityCritical:
		return "#ff0000"
	case PriorityEmergency:
		return "#9C27B0"
	default:
		return "#cccccc"
	}
}

// SlackChannelConfig holds config for a Slack notification channel
type SlackChannelConfig struct {
	WebhookURL string
}

// slackChannel implements Channel for Slack webhook delivery
type slackChannel struct {
	config SlackChannelConfig
	client *http.Client
}

// NewSlackChannel creates a new Slack Channel implementation
func NewSlackChannel(cfg SlackChannelConfig) Channel {
	return &slackChannel{
		config: cfg,
		client: &http.Client{},
	}
}

func (s *slackChannel) Name() string { return "slack" }

func (s *slackChannel) Send(ctx context.Context, event Event) error {
	color := slackColor(event.Priority)

	// Build fields
	fields := make([]map[string]interface{}, 0, len(event.Fields))
	for k, v := range event.Fields {
		fields = append(fields, map[string]interface{}{
			"title": k,
			"value": v,
			"short": true,
		})
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  event.Title,
				"text":   event.Message,
				"fields": fields,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *slackChannel) Close() error { return nil }

// EmailChannelConfig holds config for an email notification channel
type EmailChannelConfig struct {
	SMTPHost  string
	SMTPPort  string
	From      string
	To        string
	Password  string
	BatchMode bool
	SendFunc  func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// emailChannel implements Channel for email delivery
type emailChannel struct {
	config  EmailChannelConfig
	mu      sync.Mutex
	batched []Event
}

// NewEmailChannel creates a new email Channel implementation
func NewEmailChannel(cfg EmailChannelConfig) *emailChannel {
	if cfg.SendFunc == nil {
		cfg.SendFunc = smtp.SendMail
	}
	return &emailChannel{config: cfg}
}

func (e *emailChannel) Name() string { return string(ChannelEmail) }

func (e *emailChannel) Send(ctx context.Context, event Event) error {
	if e.config.BatchMode {
		e.mu.Lock()
		e.batched = append(e.batched, event)
		e.mu.Unlock()
		return nil
	}
	return e.sendOne(event)
}

func (e *emailChannel) sendOne(event Event) error {
	body := e.buildHTML([]Event{event})
	msg := e.buildRawEmail(event.Title, body)
	addr := e.config.SMTPHost + ":" + e.config.SMTPPort
	return e.config.SendFunc(addr, nil, e.config.From, []string{e.config.To}, msg)
}

func (e *emailChannel) buildRawEmail(subject, htmlBody string) []byte {
	var buf bytes.Buffer
	buf.WriteString("From: " + e.config.From + "\r\n")
	buf.WriteString("To: " + e.config.To + "\r\n")
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	return buf.Bytes()
}

func (e *emailChannel) buildHTML(events []Event) string {
	tmpl := template.Must(template.New("digest").Parse(`<html><body>
{{range .}}<h2>{{.Title}}</h2><p>{{.Message}}</p>
{{range $k, $v := .Fields}}<b>{{$k}}:</b> {{$v}}<br/>{{end}}<hr/>
{{end}}</body></html>`))
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, events)
	return buf.String()
}

// BatchedCount returns the number of batched events
func (e *emailChannel) BatchedCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.batched)
}

// FlushDigest sends all batched events as a single digest email
func (e *emailChannel) FlushDigest() error {
	e.mu.Lock()
	events := e.batched
	e.batched = nil
	e.mu.Unlock()

	if len(events) == 0 {
		return nil
	}

	titles := make([]string, len(events))
	for i, ev := range events {
		titles[i] = ev.Title
	}
	subject := "Digest: " + strings.Join(titles, ", ")
	body := e.buildHTML(events)
	msg := e.buildRawEmail(subject, body)
	addr := e.config.SMTPHost + ":" + e.config.SMTPPort
	return e.config.SendFunc(addr, nil, e.config.From, []string{e.config.To}, msg)
}

func (e *emailChannel) Close() error { return nil }
