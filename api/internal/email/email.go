package email

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

// Sender interface defines email sending operations
type Sender interface {
	Send(ctx context.Context, msg *Message) error
}

// Message represents an email message
type Message struct {
	To       []string
	From     string
	Subject  string
	Body     string
	HTMLBody string
	ReplyTo  string
}

// Config holds email configuration
type Config struct {
	// SMTP settings
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// TLS settings
	UseTLS     bool
	SkipVerify bool

	// App settings
	AppName      string
	AppURL       string
	SupportEmail string

	// Feature flags
	Enabled bool
	LogOnly bool // If true, just log emails instead of sending (for development)
}

// DefaultConfig returns sensible defaults for email configuration
func DefaultConfig() Config {
	return Config{
		SMTPPort: 587,
		UseTLS:   true,
		AppName:  "Kootenai",
		LogOnly:  true, // Default to log-only for development safety
	}
}

// SMTPSender implements Sender using SMTP
type SMTPSender struct {
	config Config
	logger *slog.Logger
}

// NewSMTPSender creates a new SMTP email sender
func NewSMTPSender(config Config, logger *slog.Logger) *SMTPSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &SMTPSender{
		config: config,
		logger: logger,
	}
}

// Send sends an email message
func (s *SMTPSender) Send(ctx context.Context, msg *Message) error {
	if !s.config.Enabled {
		s.logger.Debug("email sending disabled", "to", msg.To, "subject", msg.Subject)
		return nil
	}

	// Log-only mode for development
	if s.config.LogOnly {
		s.logger.Info("email would be sent (log-only mode)",
			"to", msg.To,
			"from", msg.From,
			"subject", msg.Subject,
			"bodyPreview", truncate(msg.Body, 200),
		)
		return nil
	}

	from := msg.From
	if from == "" {
		from = s.config.SMTPFrom
	}

	// Build email headers and body
	var emailBody bytes.Buffer
	emailBody.WriteString(fmt.Sprintf("From: %s\r\n", from))
	emailBody.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	emailBody.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))

	if msg.ReplyTo != "" {
		emailBody.WriteString(fmt.Sprintf("Reply-To: %s\r\n", msg.ReplyTo))
	}

	// MIME headers for HTML emails
	if msg.HTMLBody != "" {
		emailBody.WriteString("MIME-Version: 1.0\r\n")
		emailBody.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		emailBody.WriteString("\r\n")
		emailBody.WriteString(msg.HTMLBody)
	} else {
		emailBody.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		emailBody.WriteString("\r\n")
		emailBody.WriteString(msg.Body)
	}

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	var auth smtp.Auth
	if s.config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	}

	err := smtp.SendMail(addr, auth, from, msg.To, emailBody.Bytes())
	if err != nil {
		s.logger.Error("failed to send email", "error", err, "to", msg.To, "subject", msg.Subject)
		return fmt.Errorf("sending email: %w", err)
	}

	s.logger.Info("email sent successfully", "to", msg.To, "subject", msg.Subject)
	return nil
}

// truncate truncates a string to maxLen with ellipsis
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// localizedPasswordResetStrings holds pre-resolved i18n strings for email templates.
type localizedPasswordResetStrings struct {
	Subject         string
	Heading         string
	Greeting        string
	Body            string
	ButtonLabel     string
	ExpiresIn       string
	IgnoreNotice    string
	LinkFallback    string
	SupportContact  string
	PlainBody       string
	PlainLinkPrompt string
}

// resolvePasswordResetStrings resolves all email template strings from the
// request-scoped localizer. Falls back to English (the message ID) on miss.
func resolvePasswordResetStrings(ctx context.Context, userName, expiresIn, appName, supportEmail string) localizedPasswordResetStrings {
	greeting := appi18n.Localize(ctx, "email.passwordReset.greetingAnonymous", nil)
	if userName != "" {
		greeting = appi18n.Localize(ctx, "email.passwordReset.greeting", map[string]any{"UserName": userName})
	}

	return localizedPasswordResetStrings{
		Subject:         appi18n.Localize(ctx, "email.passwordReset.subject", map[string]any{"AppName": appName}),
		Heading:         appi18n.Localize(ctx, "email.passwordReset.heading", nil),
		Greeting:        greeting,
		Body:            appi18n.Localize(ctx, "email.passwordReset.body", nil),
		ButtonLabel:     appi18n.Localize(ctx, "email.passwordReset.buttonLabel", nil),
		ExpiresIn:       appi18n.Localize(ctx, "email.passwordReset.expiresIn", map[string]any{"ExpiresIn": expiresIn}),
		IgnoreNotice:    appi18n.Localize(ctx, "email.passwordReset.ignoreNotice", nil),
		LinkFallback:    appi18n.Localize(ctx, "email.passwordReset.linkFallback", nil),
		SupportContact:  appi18n.Localize(ctx, "email.passwordReset.supportContact", map[string]any{"SupportEmail": supportEmail}),
		PlainBody:       appi18n.Localize(ctx, "email.passwordReset.plain.body", map[string]any{"AppName": appName}),
		PlainLinkPrompt: appi18n.Localize(ctx, "email.passwordReset.plain.linkPrompt", nil),
	}
}

// PasswordResetData holds data for password reset email template rendering.
type PasswordResetData struct {
	// Localized strings
	Heading        string
	Greeting       string
	Body           string
	ButtonLabel    string
	ExpiresIn      string
	IgnoreNotice   string
	LinkFallback   string
	SupportContact string

	// Dynamic data
	ResetLink    string
	AppName      string
	SupportEmail string
}

// PasswordResetTemplate is the HTML template for password reset emails.
// All user-facing strings come from the localized PasswordResetData fields.
var PasswordResetTemplate = template.Must(template.New("password_reset").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; border-radius: 8px 8px 0 0;">
        <h1 style="color: white; margin: 0; font-size: 24px;">{{.AppName}}</h1>
    </div>
    <div style="background: #ffffff; padding: 30px; border: 1px solid #e0e0e0; border-top: none; border-radius: 0 0 8px 8px;">
        <h2 style="color: #333; margin-top: 0;">{{.Heading}}</h2>
        <p>{{.Greeting}}</p>
        <p>{{.Body}}</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.ResetLink}}" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 14px 28px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block;">{{.ButtonLabel}}</a>
        </div>
        <p style="color: #666; font-size: 14px;">{{.ExpiresIn}}</p>
        <p style="color: #666; font-size: 14px;">{{.IgnoreNotice}}</p>
        <hr style="border: none; border-top: 1px solid #e0e0e0; margin: 30px 0;">
        <p style="color: #999; font-size: 12px;">{{.LinkFallback}}</p>
        <p style="color: #667eea; font-size: 12px; word-break: break-all;">{{.ResetLink}}</p>
        {{if .SupportEmail}}
        <p style="color: #999; font-size: 12px;">{{.SupportContact}}</p>
        {{end}}
    </div>
</body>
</html>
`))

// PasswordResetPlainData holds data for the plain text password reset template.
type PasswordResetPlainData struct {
	Heading         string
	Greeting        string
	PlainBody       string
	PlainLinkPrompt string
	ResetLink       string
	ExpiresIn       string
	IgnoreNotice    string
	SupportContact  string
	SupportEmail    string
}

// PasswordResetPlainTemplate is the plain text template for password reset emails.
var PasswordResetPlainTemplate = template.Must(template.New("password_reset_plain").Parse(`
{{.Heading}}

{{.Greeting}}

{{.PlainBody}}

{{.PlainLinkPrompt}}
{{.ResetLink}}

{{.ExpiresIn}}

{{.IgnoreNotice}}

{{if .SupportEmail}}{{.SupportContact}}{{end}}
`))

// SendPasswordResetEmail sends a localized password reset email. The localizer
// is read from ctx (set by the locale middleware from Accept-Language).
func (s *SMTPSender) SendPasswordResetEmail(ctx context.Context, email, userName, resetToken string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.config.AppURL, resetToken)
	expiresIn := "1 hour"

	strs := resolvePasswordResetStrings(ctx, userName, expiresIn, s.config.AppName, s.config.SupportEmail)

	// Render HTML template
	htmlData := PasswordResetData{
		Heading:        strs.Heading,
		Greeting:       strs.Greeting,
		Body:           strs.Body,
		ButtonLabel:    strs.ButtonLabel,
		ExpiresIn:      strs.ExpiresIn,
		IgnoreNotice:   strs.IgnoreNotice,
		LinkFallback:   strs.LinkFallback,
		SupportContact: strs.SupportContact,
		ResetLink:      resetLink,
		AppName:        s.config.AppName,
		SupportEmail:   s.config.SupportEmail,
	}

	var htmlBody bytes.Buffer
	if err := PasswordResetTemplate.Execute(&htmlBody, htmlData); err != nil {
		return fmt.Errorf("rendering email template: %w", err)
	}

	// Render plain text template
	plainData := PasswordResetPlainData{
		Heading:         strs.Heading,
		Greeting:        strs.Greeting,
		PlainBody:       strs.PlainBody,
		PlainLinkPrompt: strs.PlainLinkPrompt,
		ResetLink:       resetLink,
		ExpiresIn:       strs.ExpiresIn,
		IgnoreNotice:    strs.IgnoreNotice,
		SupportContact:  strs.SupportContact,
		SupportEmail:    s.config.SupportEmail,
	}

	var plainBody bytes.Buffer
	if err := PasswordResetPlainTemplate.Execute(&plainBody, plainData); err != nil {
		return fmt.Errorf("rendering plain template: %w", err)
	}

	msg := &Message{
		To:       []string{email},
		Subject:  strs.Subject,
		Body:     plainBody.String(),
		HTMLBody: htmlBody.String(),
	}

	return s.Send(ctx, msg)
}

// NoopSender is a sender that does nothing (for testing)
type NoopSender struct{}

// Send does nothing
func (n *NoopSender) Send(ctx context.Context, msg *Message) error {
	return nil
}

// SendPasswordResetEmail does nothing (for testing)
func (n *NoopSender) SendPasswordResetEmail(ctx context.Context, email, userName, resetToken string) error {
	return nil
}
