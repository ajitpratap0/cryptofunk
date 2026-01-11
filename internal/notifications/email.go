package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// EmailConfig holds SMTP configuration for email notifications
type EmailConfig struct {
	// SMTP server settings
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`

	// Sender information
	FromAddress string `mapstructure:"from_address"`
	FromName    string `mapstructure:"from_name"`

	// Security settings
	UseTLS       bool `mapstructure:"use_tls"`       // Use TLS from start (port 465)
	UseStartTLS  bool `mapstructure:"use_starttls"`  // Use STARTTLS upgrade (port 587)
	SkipVerify   bool `mapstructure:"skip_verify"`   // Skip TLS certificate verification (dev only)
	AuthRequired bool `mapstructure:"auth_required"` // Whether SMTP auth is required

	// Rate limiting
	RateLimitPerMinute int `mapstructure:"rate_limit_per_minute"` // Max emails per minute (0 = unlimited)
	RetryAttempts      int `mapstructure:"retry_attempts"`        // Number of retry attempts on failure
	RetryDelaySeconds  int `mapstructure:"retry_delay_seconds"`   // Delay between retries
}

// DefaultEmailConfig returns default email configuration
func DefaultEmailConfig() EmailConfig {
	return EmailConfig{
		Host:               "localhost",
		Port:               587,
		FromName:           "CryptoFunk Trading",
		UseTLS:             false,
		UseStartTLS:        true,
		SkipVerify:         false,
		AuthRequired:       true,
		RateLimitPerMinute: 30,
		RetryAttempts:      3,
		RetryDelaySeconds:  5,
	}
}

// EmailBackend implements the Backend interface for sending email notifications
type EmailBackend struct {
	config      EmailConfig
	recipients  []string // Default recipients if none specified in notification
	mock        bool
	rateLimiter chan struct{}
}

// NewEmailBackend creates a new email notification backend
// If config is incomplete (missing host/from_address), creates a mock backend
func NewEmailBackend(config EmailConfig, defaultRecipients []string) (*EmailBackend, error) {
	// Validate configuration
	if config.Host == "" || config.FromAddress == "" {
		log.Warn().Msg("Incomplete email configuration, using mock backend")
		return &EmailBackend{
			config:     config,
			recipients: defaultRecipients,
			mock:       true,
		}, nil
	}

	// Set defaults for rate limiter
	if config.RateLimitPerMinute <= 0 {
		config.RateLimitPerMinute = 30
	}

	backend := &EmailBackend{
		config:      config,
		recipients:  defaultRecipients,
		mock:        false,
		rateLimiter: make(chan struct{}, config.RateLimitPerMinute),
	}

	// Fill rate limiter bucket
	go backend.refillRateLimiter()

	log.Info().
		Str("host", config.Host).
		Int("port", config.Port).
		Str("from", config.FromAddress).
		Int("default_recipients", len(defaultRecipients)).
		Msg("Initialized email notification backend")

	return backend, nil
}

// refillRateLimiter refills the rate limiter bucket every minute
func (e *EmailBackend) refillRateLimiter() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Refill the bucket
	refillLoop:
		for i := 0; i < e.config.RateLimitPerMinute; i++ {
			select {
			case e.rateLimiter <- struct{}{}:
			default:
				// Bucket is full
				break refillLoop
			}
		}
	}
}

// Send sends a notification via email
func (e *EmailBackend) Send(ctx context.Context, deviceToken string, notification Notification) error {
	// deviceToken for email backend is the recipient email address
	// If empty, use default recipients
	recipients := e.recipients
	if deviceToken != "" {
		recipients = []string{deviceToken}
	}

	if len(recipients) == 0 {
		log.Warn().Msg("No email recipients configured, skipping notification")
		return nil
	}

	if e.mock {
		return e.sendMock(recipients, notification)
	}

	// Check rate limit
	select {
	case <-e.rateLimiter:
		// Got a token, proceed
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("email rate limit exceeded")
	}

	// Build email content
	subject := e.buildSubject(notification)
	body, err := e.buildBody(notification)
	if err != nil {
		return fmt.Errorf("failed to build email body: %w", err)
	}

	// Send with retries
	var lastErr error
	for attempt := 1; attempt <= e.config.RetryAttempts; attempt++ {
		if err := e.sendEmail(ctx, recipients, subject, body); err != nil {
			lastErr = err
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max_attempts", e.config.RetryAttempts).
				Msg("Failed to send email, retrying")

			if attempt < e.config.RetryAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(e.config.RetryDelaySeconds) * time.Second):
					continue
				}
			}
		} else {
			log.Debug().
				Strs("recipients", maskEmails(recipients)).
				Str("notification_type", string(notification.Type)).
				Msg("Sent email notification")
			return nil
		}
	}

	return fmt.Errorf("failed to send email after %d attempts: %w", e.config.RetryAttempts, lastErr)
}

// sendEmail sends an email using SMTP
func (e *EmailBackend) sendEmail(ctx context.Context, recipients []string, subject, body string) error {
	// Build message
	msg := e.buildMessage(recipients, subject, body)

	// Connect to SMTP server (use net.JoinHostPort for IPv6 compatibility)
	addr := net.JoinHostPort(e.config.Host, strconv.Itoa(e.config.Port))

	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName:         e.config.Host,
		InsecureSkipVerify: e.config.SkipVerify, //nolint:gosec // Configurable for dev environments
	}

	var conn net.Conn
	var err error

	// Use context-aware dialing
	dialCtx, dialCancel := context.WithTimeout(ctx, 30*time.Second)
	defer dialCancel()

	dialer := &net.Dialer{}

	// Connect with appropriate security
	if e.config.UseTLS {
		// Direct TLS connection (port 465)
		tlsDialer := &tls.Dialer{
			NetDialer: dialer,
			Config:    tlsConfig,
		}
		conn, err = tlsDialer.DialContext(dialCtx, "tcp", addr)
	} else {
		// Plain connection (will upgrade with STARTTLS if configured)
		conn, err = dialer.DialContext(dialCtx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, e.config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// STARTTLS if configured and not already using TLS
	if e.config.UseStartTLS && !e.config.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	// Authenticate if required
	if e.config.AuthRequired && e.config.Username != "" && e.config.Password != "" {
		auth := smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(e.config.FromAddress); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", maskEmail(recipient), err)
		}
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// buildMessage builds the raw email message
func (e *EmailBackend) buildMessage(recipients []string, subject, body string) string {
	var msg strings.Builder

	// Headers
	if e.config.FromName != "" {
		msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", e.config.FromName, e.config.FromAddress))
	} else {
		msg.WriteString(fmt.Sprintf("From: %s\r\n", e.config.FromAddress))
	}
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("\r\n")

	// Body
	msg.WriteString(body)

	return msg.String()
}

// buildSubject builds the email subject line
func (e *EmailBackend) buildSubject(notification Notification) string {
	prefix := "[CryptoFunk]"

	switch notification.Type {
	case NotificationTypeTradeExecution:
		prefix = "[CryptoFunk Trade]"
	case NotificationTypePnLAlert:
		prefix = "[CryptoFunk P&L Alert]"
	case NotificationTypeCircuitBreaker:
		prefix = "[CryptoFunk CRITICAL]"
	case NotificationTypeConsensusFailure:
		prefix = "[CryptoFunk Warning]"
	case NotificationTypePositionClosed:
		prefix = "[CryptoFunk Position]"
	case NotificationTypeSafetyGuard:
		prefix = "[CryptoFunk Safety]"
	case NotificationTypeSystemError:
		prefix = "[CryptoFunk ERROR]"
	case NotificationTypeDailySummary:
		prefix = "[CryptoFunk Daily Summary]"
	}

	return fmt.Sprintf("%s %s", prefix, notification.Title)
}

// buildBody builds the HTML email body using templates
func (e *EmailBackend) buildBody(notification Notification) (string, error) {
	tmpl := template.Must(template.New("email").Parse(emailBodyTemplate))

	data := struct {
		Title      string
		Body       string
		Type       NotificationType
		Data       map[string]string
		Priority   string
		Timestamp  string
		FooterText string
		IsHighPrio bool
		TypeLabel  string
		TypeColor  string
		DetailRows []struct{ Key, Value string }
	}{
		Title:      notification.Title,
		Body:       notification.Body,
		Type:       notification.Type,
		Data:       notification.Data,
		Priority:   notification.Priority,
		Timestamp:  time.Now().Format("2006-01-02 15:04:05 MST"),
		FooterText: "This is an automated notification from CryptoFunk Trading System.",
		IsHighPrio: notification.Priority == priorityHigh,
	}

	// Set type-specific styling
	switch notification.Type {
	case NotificationTypeTradeExecution:
		data.TypeLabel = "Trade Executed"
		data.TypeColor = colorGreen
	case NotificationTypePnLAlert:
		data.TypeLabel = "P&L Alert"
		data.TypeColor = colorYellow
	case NotificationTypeCircuitBreaker:
		data.TypeLabel = "Circuit Breaker"
		data.TypeColor = colorRed
	case NotificationTypeConsensusFailure:
		data.TypeLabel = "Consensus Failure"
		data.TypeColor = colorOrange
	case NotificationTypePositionClosed:
		data.TypeLabel = "Position Closed"
		data.TypeColor = colorCyan
	case NotificationTypeSafetyGuard:
		data.TypeLabel = "Safety Guard"
		data.TypeColor = colorRed
	case NotificationTypeSystemError:
		data.TypeLabel = "System Error"
		data.TypeColor = colorRed
	case NotificationTypeDailySummary:
		data.TypeLabel = "Daily Summary"
		data.TypeColor = colorGray
	default:
		data.TypeLabel = "Notification"
		data.TypeColor = colorBlue
	}

	// Convert data map to detail rows
	for key, value := range notification.Data {
		data.DetailRows = append(data.DetailRows, struct{ Key, Value string }{
			Key:   formatDataKey(key),
			Value: value,
		})
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute email template: %w", err)
	}

	return buf.String(), nil
}

// sendMock logs the email instead of sending it
func (e *EmailBackend) sendMock(recipients []string, notification Notification) error {
	log.Info().
		Str("backend", "email_mock").
		Strs("recipients", maskEmails(recipients)).
		Str("notification_type", string(notification.Type)).
		Str("title", notification.Title).
		Str("body", notification.Body).
		Str("priority", notification.Priority).
		Msg("Mock email notification (not actually sent)")

	return nil
}

// Name returns the backend name
func (e *EmailBackend) Name() string {
	if e.mock {
		return "email_mock"
	}
	return "email"
}

// Close closes the email backend
func (e *EmailBackend) Close() error {
	log.Debug().Str("backend", e.Name()).Msg("Closed email backend")
	return nil
}

// IsMock returns true if this is a mock backend
func (e *EmailBackend) IsMock() bool {
	return e.mock
}

// AddRecipient adds a default recipient
func (e *EmailBackend) AddRecipient(email string) {
	for _, r := range e.recipients {
		if r == email {
			return
		}
	}
	e.recipients = append(e.recipients, email)
}

// RemoveRecipient removes a default recipient
func (e *EmailBackend) RemoveRecipient(email string) {
	for i, r := range e.recipients {
		if r == email {
			e.recipients = append(e.recipients[:i], e.recipients[i+1:]...)
			return
		}
	}
}

// GetRecipients returns the list of default recipients
func (e *EmailBackend) GetRecipients() []string {
	return e.recipients
}

// SetRecipients sets the list of default recipients
func (e *EmailBackend) SetRecipients(recipients []string) {
	e.recipients = recipients
}

// Helper functions

// maskEmail masks an email address for logging
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		return "***@" + domain
	}
	return local[:2] + "***@" + domain
}

// maskEmails masks multiple email addresses
func maskEmails(emails []string) []string {
	masked := make([]string, len(emails))
	for i, email := range emails {
		masked[i] = maskEmail(email)
	}
	return masked
}

// formatDataKey formats a data key for display (e.g., "order_id" -> "Order ID")
func formatDataKey(key string) string {
	words := strings.Split(key, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// Email HTML template
const emailBodyTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background-color: {{.TypeColor}};
            color: #ffffff;
            padding: 20px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 24px;
        }
        .type-badge {
            display: inline-block;
            background-color: rgba(255,255,255,0.2);
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            margin-top: 8px;
        }
        .content {
            padding: 24px;
        }
        .title {
            font-size: 20px;
            font-weight: 600;
            margin-bottom: 12px;
            color: #1a1a1a;
        }
        .body-text {
            font-size: 16px;
            color: #4a4a4a;
            margin-bottom: 20px;
        }
        .details {
            background-color: #f8f9fa;
            border-radius: 6px;
            padding: 16px;
            margin-top: 20px;
        }
        .details h3 {
            margin: 0 0 12px 0;
            font-size: 14px;
            color: #666;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .detail-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid #e9ecef;
        }
        .detail-row:last-child {
            border-bottom: none;
        }
        .detail-key {
            font-weight: 500;
            color: #495057;
        }
        .detail-value {
            color: #212529;
            font-family: 'Monaco', 'Menlo', monospace;
        }
        .footer {
            background-color: #f8f9fa;
            padding: 16px 24px;
            text-align: center;
            font-size: 12px;
            color: #6c757d;
        }
        .timestamp {
            color: #868e96;
            font-size: 14px;
            margin-top: 16px;
        }
        {{if .IsHighPrio}}
        .priority-banner {
            background-color: #dc3545;
            color: white;
            text-align: center;
            padding: 8px;
            font-weight: bold;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        {{end}}
    </style>
</head>
<body>
    <div class="container">
        {{if .IsHighPrio}}
        <div class="priority-banner">High Priority Alert</div>
        {{end}}
        <div class="header">
            <h1>CryptoFunk</h1>
            <div class="type-badge">{{.TypeLabel}}</div>
        </div>
        <div class="content">
            <div class="title">{{.Title}}</div>
            <div class="body-text">{{.Body}}</div>
            {{if .DetailRows}}
            <div class="details">
                <h3>Details</h3>
                {{range .DetailRows}}
                <div class="detail-row">
                    <span class="detail-key">{{.Key}}</span>
                    <span class="detail-value">{{.Value}}</span>
                </div>
                {{end}}
            </div>
            {{end}}
            <div class="timestamp">{{.Timestamp}}</div>
        </div>
        <div class="footer">
            {{.FooterText}}
        </div>
    </div>
</body>
</html>`
