package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
)

// EmailConfig holds SMTP configuration.
type EmailConfig struct {
	Host       string   // SMTP server host
	Port       int      // SMTP server port (465 for implicit TLS, 587 for STARTTLS)
	Username   string   // SMTP username (optional)
	Password   string   // SMTP password (optional)
	From       string   // From address
	Recipients []string // Email recipients
}

// Validate validates the email configuration.
func (c *EmailConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if c.Port == 0 {
		return fmt.Errorf("SMTP port is required")
	}
	if c.From == "" {
		return fmt.Errorf("from address is required")
	}
	if len(c.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	return nil
}

// EmailNotifier sends alerts via email.
type EmailNotifier struct {
	config    EmailConfig
	templates *Templates
}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier(config EmailConfig) (*EmailNotifier, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid email config: %w", err)
	}

	templates, err := LoadTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return &EmailNotifier{
		config:    config,
		templates: templates,
	}, nil
}

// Name returns "email".
func (e *EmailNotifier) Name() string {
	return "email"
}

// Send sends an alert to all configured recipients.
func (e *EmailNotifier) Send(ctx context.Context, alert *alerting.Alert) error {
	// Convert alert to template data
	data := AlertToTemplateData(alert)

	// Render templates
	htmlBody, err := e.templates.RenderHTML(data)
	if err != nil {
		return fmt.Errorf("failed to render HTML template: %w", err)
	}

	plainBody, err := e.templates.RenderPlain(data)
	if err != nil {
		return fmt.Errorf("failed to render plain template: %w", err)
	}

	// Build subject
	subject := fmt.Sprintf("[%s] BlazeLog Alert: %s", strings.ToUpper(string(alert.Severity)), alert.RuleName)

	// Build message
	msg := e.buildMIMEMessage(subject, plainBody, htmlBody)

	// Send email
	return e.sendMail(ctx, msg)
}

// Close is a no-op for email notifier.
func (e *EmailNotifier) Close() error {
	return nil
}

// buildMIMEMessage builds a MIME multipart message with HTML and plain text.
func (e *EmailNotifier) buildMIMEMessage(subject, plainBody, htmlBody string) []byte {
	boundary := fmt.Sprintf("----=_Part_%d", time.Now().UnixNano())

	var msg strings.Builder

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", e.config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.config.Recipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	msg.WriteString("\r\n")

	// Plain text part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(plainBody)
	msg.WriteString("\r\n")

	// HTML part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)
	msg.WriteString("\r\n")

	// End boundary
	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return []byte(msg.String())
}

// sendMail sends the email via SMTP.
func (e *EmailNotifier) sendMail(ctx context.Context, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", e.config.Host, e.config.Port)

	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName: e.config.Host,
	}

	var client *smtp.Client
	var err error

	// Try to connect based on port
	if e.config.Port == 465 {
		// Implicit TLS (SMTPS)
		client, err = e.connectImplicitTLS(addr, tlsConfig)
	} else {
		// STARTTLS (port 587 or 25)
		client, err = e.connectSTARTTLS(ctx, addr, tlsConfig)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Authenticate if credentials provided
	if e.config.Username != "" && e.config.Password != "" {
		auth := smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(e.extractEmail(e.config.From)); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, rcpt := range e.config.Recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", rcpt, err)
		}
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data: %w", err)
	}

	return client.Quit()
}

// connectImplicitTLS connects using implicit TLS (port 465).
func (e *EmailNotifier) connectImplicitTLS(addr string, tlsConfig *tls.Config) (*smtp.Client, error) {
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return smtp.NewClient(conn, e.config.Host)
}

// connectSTARTTLS connects using STARTTLS (port 587 or 25).
func (e *EmailNotifier) connectSTARTTLS(ctx context.Context, addr string, tlsConfig *tls.Config) (*smtp.Client, error) {
	// Use context-aware dialer
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	client, err := smtp.NewClient(conn, e.config.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Try STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("STARTTLS failed: %w", err)
		}
	}

	return client, nil
}

// extractEmail extracts the email address from a "Name <email>" format.
func (e *EmailNotifier) extractEmail(addr string) string {
	if start := strings.Index(addr, "<"); start != -1 {
		if end := strings.Index(addr, ">"); end != -1 {
			return addr[start+1 : end]
		}
	}
	return addr
}
