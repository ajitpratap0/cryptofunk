package alerts

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

// EmailAlerter sends alerts via email
type EmailAlerter struct {
	config     EmailConfig
	recipients []string
}

// EmailConfig holds email alerter configuration
type EmailConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	FromAddress  string
	FromName     string
	UseTLS       bool
	UseStartTLS  bool
	SkipVerify   bool
	AuthRequired bool
}

// NewEmailAlerter creates a new email-based alerter
func NewEmailAlerter(config EmailConfig, recipients []string) (*EmailAlerter, error) {
	if config.Host == "" || config.FromAddress == "" {
		return nil, fmt.Errorf("email host and from_address are required")
	}

	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	log.Info().
		Str("host", config.Host).
		Int("port", config.Port).
		Str("from", config.FromAddress).
		Int("recipient_count", len(recipients)).
		Msg("Email alerter initialized")

	return &EmailAlerter{
		config:     config,
		recipients: recipients,
	}, nil
}

// Send sends an alert via email
func (e *EmailAlerter) Send(ctx context.Context, alert Alert) error {
	if len(e.recipients) == 0 {
		log.Warn().Msg("No email recipients configured, skipping alert")
		return nil
	}

	// Build email content
	subject := e.buildSubject(alert)
	body, err := e.buildBody(alert)
	if err != nil {
		return fmt.Errorf("failed to build email body: %w", err)
	}

	// Send email
	if err := e.sendEmail(ctx, subject, body); err != nil {
		return fmt.Errorf("failed to send email alert: %w", err)
	}

	log.Debug().
		Strs("recipients", e.maskEmails(e.recipients)).
		Str("alert_title", alert.Title).
		Str("severity", string(alert.Severity)).
		Msg("Sent email alert")

	return nil
}

// sendEmail sends an email via SMTP
func (e *EmailAlerter) sendEmail(ctx context.Context, subject, body string) error {
	msg := e.buildMessage(subject, body)
	// Use net.JoinHostPort for IPv6 compatibility
	addr := net.JoinHostPort(e.config.Host, strconv.Itoa(e.config.Port))

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

	if e.config.UseTLS {
		tlsDialer := &tls.Dialer{
			NetDialer: dialer,
			Config:    tlsConfig,
		}
		conn, err = tlsDialer.DialContext(dialCtx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(dialCtx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	if e.config.UseStartTLS && !e.config.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	if e.config.AuthRequired && e.config.Username != "" && e.config.Password != "" {
		auth := smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	if err := client.Mail(e.config.FromAddress); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, recipient := range e.recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient: %w", err)
		}
	}

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
func (e *EmailAlerter) buildMessage(subject, body string) string {
	var msg strings.Builder

	if e.config.FromName != "" {
		fmt.Fprintf(&msg, "From: %s <%s>\r\n", e.config.FromName, e.config.FromAddress)
	} else {
		fmt.Fprintf(&msg, "From: %s\r\n", e.config.FromAddress)
	}
	fmt.Fprintf(&msg, "To: %s\r\n", strings.Join(e.recipients, ", "))
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	fmt.Fprintf(&msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}

// buildSubject builds the email subject line
func (e *EmailAlerter) buildSubject(alert Alert) string {
	var prefix string
	switch alert.Severity {
	case SeverityCritical:
		prefix = "[CRITICAL]"
	case SeverityWarning:
		prefix = "[WARNING]"
	case SeverityInfo:
		prefix = "[INFO]"
	default:
		prefix = "[ALERT]"
	}

	return fmt.Sprintf("%s CryptoFunk: %s", prefix, alert.Title)
}

// buildBody builds the HTML email body
func (e *EmailAlerter) buildBody(alert Alert) (string, error) {
	tmpl := template.Must(template.New("alert").Parse(alertEmailTemplate))

	color := "#007bff" // Default blue
	switch alert.Severity {
	case SeverityCritical:
		color = "#dc3545" // Red
	case SeverityWarning:
		color = "#ffc107" // Yellow
	case SeverityInfo:
		color = "#17a2b8" // Cyan
	}

	data := struct {
		Title     string
		Message   string
		Severity  string
		Timestamp string
		Metadata  map[string]interface{}
		Color     string
	}{
		Title:     alert.Title,
		Message:   alert.Message,
		Severity:  string(alert.Severity),
		Timestamp: alert.Timestamp.Format("2006-01-02 15:04:05 MST"),
		Metadata:  alert.Metadata,
		Color:     color,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute email template: %w", err)
	}

	return buf.String(), nil
}

// maskEmails masks email addresses for logging
func (e *EmailAlerter) maskEmails(emails []string) []string {
	masked := make([]string, len(emails))
	for i, email := range emails {
		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			masked[i] = "***"
			continue
		}
		local := parts[0]
		domain := parts[1]
		if len(local) <= 2 {
			masked[i] = "***@" + domain
		} else {
			masked[i] = local[:2] + "***@" + domain
		}
	}
	return masked
}

// AddRecipient adds a recipient to the alerter
func (e *EmailAlerter) AddRecipient(email string) {
	for _, r := range e.recipients {
		if r == email {
			return
		}
	}
	e.recipients = append(e.recipients, email)
	log.Info().
		Str("email", e.maskEmails([]string{email})[0]).
		Msg("Added email recipient to alerter")
}

// RemoveRecipient removes a recipient from the alerter
func (e *EmailAlerter) RemoveRecipient(email string) {
	for i, r := range e.recipients {
		if r == email {
			e.recipients = append(e.recipients[:i], e.recipients[i+1:]...)
			log.Info().
				Str("email", e.maskEmails([]string{email})[0]).
				Msg("Removed email recipient from alerter")
			return
		}
	}
}

// GetRecipients returns the list of recipients
func (e *EmailAlerter) GetRecipients() []string {
	return e.recipients
}

// SetRecipients sets the list of recipients
func (e *EmailAlerter) SetRecipients(recipients []string) {
	e.recipients = recipients
	log.Info().
		Int("recipient_count", len(recipients)).
		Msg("Updated email alerter recipients")
}

// HTML template for alert emails
const alertEmailTemplate = `<!DOCTYPE html>
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
            background-color: {{.Color}};
            color: #ffffff;
            padding: 20px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 20px;
        }
        .severity-badge {
            display: inline-block;
            background-color: rgba(255,255,255,0.2);
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            margin-top: 8px;
            text-transform: uppercase;
        }
        .content {
            padding: 24px;
        }
        .title {
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 12px;
            color: #1a1a1a;
        }
        .message {
            font-size: 16px;
            color: #4a4a4a;
            margin-bottom: 20px;
            white-space: pre-wrap;
        }
        .metadata {
            background-color: #f8f9fa;
            border-radius: 6px;
            padding: 16px;
            margin-top: 20px;
        }
        .metadata h3 {
            margin: 0 0 12px 0;
            font-size: 14px;
            color: #666;
            text-transform: uppercase;
        }
        .metadata-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid #e9ecef;
        }
        .metadata-row:last-child {
            border-bottom: none;
        }
        .metadata-key {
            font-weight: 500;
            color: #495057;
        }
        .metadata-value {
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
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>CryptoFunk Alert</h1>
            <div class="severity-badge">{{.Severity}}</div>
        </div>
        <div class="content">
            <div class="title">{{.Title}}</div>
            <div class="message">{{.Message}}</div>
            {{if .Metadata}}
            <div class="metadata">
                <h3>Details</h3>
                {{range $key, $value := .Metadata}}
                <div class="metadata-row">
                    <span class="metadata-key">{{$key}}</span>
                    <span class="metadata-value">{{$value}}</span>
                </div>
                {{end}}
            </div>
            {{end}}
            <div class="timestamp">{{.Timestamp}}</div>
        </div>
        <div class="footer">
            This is an automated alert from CryptoFunk Trading System.
        </div>
    </div>
</body>
</html>`
