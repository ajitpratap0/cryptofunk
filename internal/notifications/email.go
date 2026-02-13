package notifications

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// EmailChannelConfig holds Email channel configuration
type EmailChannelConfig struct {
	SMTPHost  string
	SMTPPort  string
	Username  string
	Password  string
	From      string
	To        string
	SendFunc  func(addr string, a smtp.Auth, from string, to []string, msg []byte) error // for testing
	BatchMode bool // if true, batch notifications for daily digest
}

// EmailChannel sends notifications via SMTP email
type EmailChannel struct {
	config  EmailChannelConfig
	mu      sync.Mutex
	batched []Event
}

// NewEmailChannel creates a new Email notification channel
func NewEmailChannel(cfg EmailChannelConfig) *EmailChannel {
	if cfg.SendFunc == nil {
		cfg.SendFunc = smtp.SendMail
	}
	return &EmailChannel{config: cfg}
}

func (e *EmailChannel) Name() string { return "email" }
func (e *EmailChannel) Close() error { return nil }

// Send delivers a notification event via email
func (e *EmailChannel) Send(ctx context.Context, event Event) error {
	if e.config.BatchMode {
		e.mu.Lock()
		e.batched = append(e.batched, event)
		e.mu.Unlock()
		return nil
	}
	return e.sendEmail(event.Title, e.formatHTML(event))
}

// FlushDigest sends all batched notifications as a single digest email
func (e *EmailChannel) FlushDigest() error {
	e.mu.Lock()
	events := e.batched
	e.batched = nil
	e.mu.Unlock()

	if len(events) == 0 {
		return nil
	}

	body := e.formatDigestHTML(events)
	return e.sendEmail("CryptoFunk Daily Digest", body)
}

// BatchedCount returns the number of batched events (for testing)
func (e *EmailChannel) BatchedCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.batched)
}

func (e *EmailChannel) sendEmail(subject, htmlBody string) error {
	addr := e.config.SMTPHost + ":" + e.config.SMTPPort
	var auth smtp.Auth
	if e.config.Username != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPHost)
	}

	msg := "From: " + e.config.From + "\r\n" +
		"To: " + e.config.To + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" +
		htmlBody

	return e.config.SendFunc(addr, auth, e.config.From, []string{e.config.To}, []byte(msg))
}

func (e *EmailChannel) formatHTML(event Event) string {
	color := priorityColor(event.Priority)
	var fields strings.Builder
	for k, v := range event.Fields {
		fields.WriteString(fmt.Sprintf("<tr><td style='padding:4px 8px;font-weight:bold'>%s</td><td style='padding:4px 8px'>%s</td></tr>", k, v))
	}
	return fmt.Sprintf(`<div style="font-family:sans-serif;max-width:600px;margin:0 auto">
<div style="background:%s;color:white;padding:12px 16px;border-radius:8px 8px 0 0">
<h2 style="margin:0">%s</h2>
<small>%s | %s</small>
</div>
<div style="border:1px solid #ddd;padding:16px;border-radius:0 0 8px 8px">
<p>%s</p>
<table style="width:100%%;border-collapse:collapse">%s</table>
<p style="color:#888;font-size:12px">%s</p>
</div></div>`,
		color, event.Title, event.Priority.String(), string(event.Type),
		event.Message, fields.String(), event.Timestamp.Format(time.RFC1123))
}

func (e *EmailChannel) formatDigestHTML(events []Event) string {
	var sections strings.Builder
	for _, ev := range events {
		sections.WriteString(e.formatHTML(ev))
		sections.WriteString("<hr style='border:none;border-top:1px solid #eee;margin:16px 0'>")
	}
	return fmt.Sprintf(`<div style="font-family:sans-serif;max-width:600px;margin:0 auto">
<h1 style="color:#333">📊 Daily Trading Digest</h1>
<p>%d notifications today</p>
%s
</div>`, len(events), sections.String())
}

func priorityColor(p Priority) string {
	switch p {
	case PriorityInfo:
		return "#2196F3"
	case PriorityWarning:
		return "#FF9800"
	case PriorityCritical:
		return "#F44336"
	case PriorityEmergency:
		return "#9C27B0"
	default:
		return "#607D8B"
	}
}
