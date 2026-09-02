package email

import (
	"bytes"
	"context"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.SMTPPort != 587 {
		t.Errorf("expected port 587, got %d", config.SMTPPort)
	}
	if !config.LogOnly {
		t.Error("expected LogOnly to be true by default")
	}
	if config.AppName != "Kootenai" {
		t.Errorf("expected AppName 'Kootenai', got %s", config.AppName)
	}
}

func TestSMTPSender_LogOnlyMode(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"test@example.com"},
		Subject: "Test Subject",
		Body:    "Test Body",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error in log-only mode: %v", err)
	}
}

func TestSMTPSender_Disabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"test@example.com"},
		Subject: "Test Subject",
		Body:    "Test Body",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error when disabled: %v", err)
	}
}

// ctxWithBundle returns a context carrying an English localizer backed by the
// embedded catalogs so template resolution works in tests.
func ctxWithBundle(t *testing.T) context.Context {
	t.Helper()
	bundle := appi18n.MustNewBundle()
	l := i18n.NewLocalizer(bundle, "en")
	return appi18n.WithLocalizer(context.Background(), l)
}

func TestPasswordResetTemplate(t *testing.T) {
	data := PasswordResetData{
		Heading:        "Password Reset Request",
		Greeting:       "Hi John Doe,",
		Body:           "We received a request to reset your password. Click the button below to create a new password:",
		ButtonLabel:    "Reset Password",
		ExpiresIn:      "This link will expire in 1 hour.",
		IgnoreNotice:   "If you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.",
		LinkFallback:   "If the button doesn't work, copy and paste this link into your browser:",
		SupportContact: "Need help? Contact us at support@example.com",
		ResetLink:      "https://example.com/reset?token=abc123",
		AppName:        "Kootenai",
		SupportEmail:   "support@example.com",
	}

	var buf bytes.Buffer
	err := PasswordResetTemplate.Execute(&buf, data)
	if err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("John Doe")) {
		t.Error("expected user name in output")
	}
	if !bytes.Contains([]byte(output), []byte("https://example.com/reset?token=abc123")) {
		t.Error("expected reset link in output")
	}
	if !bytes.Contains([]byte(output), []byte("1 hour")) {
		t.Error("expected expiration time in output")
	}
	if !bytes.Contains([]byte(output), []byte("support@example.com")) {
		t.Error("expected support email in output")
	}
}

func TestPasswordResetPlainTemplate(t *testing.T) {
	data := PasswordResetPlainData{
		Heading:         "Password Reset Request",
		Greeting:        "Hi Jane Doe,",
		PlainBody:       "We received a request to reset your password for your Kootenai account.",
		PlainLinkPrompt: "To reset your password, visit the following link:",
		ResetLink:       "https://example.com/reset?token=xyz789",
		ExpiresIn:       "This link will expire in 30 minutes.",
		IgnoreNotice:    "If you didn't request a password reset, you can safely ignore this email.",
		SupportContact:  "",
		SupportEmail:    "",
	}

	var buf bytes.Buffer
	err := PasswordResetPlainTemplate.Execute(&buf, data)
	if err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("Jane Doe")) {
		t.Error("expected user name in output")
	}
	if !bytes.Contains([]byte(output), []byte("https://example.com/reset?token=xyz789")) {
		t.Error("expected reset link in output")
	}
}

func TestNoopSender(t *testing.T) {
	sender := &NoopSender{}

	msg := &Message{
		To:      []string{"test@example.com"},
		Subject: "Test",
		Body:    "Test",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error from NoopSender: %v", err)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly 10", 10, "exactly 10"},
		{"this is a longer string", 10, "this is..."},
		{"hello", 5, "hello"},
		{"hello world", 5, "he..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestSMTPSender_SendPasswordResetEmail(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true
	config.AppURL = "https://kootenai.example.com"
	config.AppName = "Kootenai"

	sender := NewSMTPSender(config, nil)

	ctx := ctxWithBundle(t)
	err := sender.SendPasswordResetEmail(ctx, "user@example.com", "Test User", "abc123token")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSMTPSender_SendPasswordResetEmail_Localized(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true
	config.AppURL = "https://kootenai.example.com"
	config.AppName = "Kootenai"
	config.SupportEmail = "help@example.com"

	sender := NewSMTPSender(config, nil)

	// Test with Spanish localizer
	bundle := appi18n.MustNewBundle()
	l := i18n.NewLocalizer(bundle, "es")
	ctx := appi18n.WithLocalizer(context.Background(), l)

	err := sender.SendPasswordResetEmail(ctx, "usuario@example.com", "María", "token123")
	if err != nil {
		t.Errorf("unexpected error with Spanish locale: %v", err)
	}
}

func TestSMTPSender_SendPasswordResetEmail_NoLocalizer(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true
	config.AppURL = "https://kootenai.example.com"
	config.AppName = "Kootenai"

	sender := NewSMTPSender(config, nil)

	// No localizer on context — should fall back to message IDs gracefully
	err := sender.SendPasswordResetEmail(context.Background(), "user@example.com", "Test", "token")
	if err != nil {
		t.Errorf("unexpected error without localizer: %v", err)
	}
}

func TestResolvePasswordResetStrings_English(t *testing.T) {
	ctx := ctxWithBundle(t)

	strs := resolvePasswordResetStrings(ctx, "Alice", "1 hour", "Kootenai", "support@example.com")

	if strs.Subject != "[Kootenai] Password Reset Request" {
		t.Errorf("unexpected subject: %s", strs.Subject)
	}
	if strs.Heading != "Password Reset Request" {
		t.Errorf("unexpected heading: %s", strs.Heading)
	}
	if strs.Greeting != "Hi Alice," {
		t.Errorf("unexpected greeting: %s", strs.Greeting)
	}
	if strs.ButtonLabel != "Reset Password" {
		t.Errorf("unexpected button label: %s", strs.ButtonLabel)
	}
}

func TestResolvePasswordResetStrings_Anonymous(t *testing.T) {
	ctx := ctxWithBundle(t)

	strs := resolvePasswordResetStrings(ctx, "", "1 hour", "Kootenai", "")

	if strs.Greeting != "Hi," {
		t.Errorf("expected anonymous greeting, got: %s", strs.Greeting)
	}
}

func TestSMTPSender_SendWithHTMLBody(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:       []string{"test@example.com"},
		Subject:  "Test HTML Email",
		Body:     "Plain text fallback",
		HTMLBody: "<html><body><h1>Hello</h1></body></html>",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error with HTML body: %v", err)
	}
}

func TestSMTPSender_SendWithReplyTo(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"recipient@example.com"},
		Subject: "Test with Reply-To",
		Body:    "Test body",
		ReplyTo: "noreply@example.com",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error with ReplyTo: %v", err)
	}
}

func TestSMTPSender_SendWithDefaultFrom(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true
	config.SMTPFrom = "system@example.com"

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"recipient@example.com"},
		Subject: "Test with default From",
		Body:    "Test body",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error with default From: %v", err)
	}
}

func TestSMTPSender_SendWithCustomFrom(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true
	config.SMTPFrom = "system@example.com"

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"recipient@example.com"},
		From:    "custom@example.com",
		Subject: "Test with custom From",
		Body:    "Test body",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error with custom From: %v", err)
	}
}

func TestSMTPSender_SendActualFails(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = false
	config.SMTPHost = "localhost"
	config.SMTPPort = 12345
	config.SMTPFrom = "test@example.com"

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"recipient@example.com"},
		Subject: "Test Send Failure",
		Body:    "Test body",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error when SMTP server is unavailable")
	}
}

func TestSMTPSender_SendWithAuth(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = false
	config.SMTPHost = "localhost"
	config.SMTPPort = 12345
	config.SMTPUsername = "testuser"
	config.SMTPPassword = "testpass"
	config.SMTPFrom = "test@example.com"

	sender := NewSMTPSender(config, nil)

	msg := &Message{
		To:      []string{"recipient@example.com"},
		Subject: "Test with Auth",
		Body:    "Test body",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error when SMTP server is unavailable")
	}
}

func TestSMTPSender_LongBodyTruncation(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true

	sender := NewSMTPSender(config, nil)

	longBody := make([]byte, 500)
	for i := range longBody {
		longBody[i] = 'a'
	}

	msg := &Message{
		To:      []string{"test@example.com"},
		Subject: "Test Long Body",
		Body:    string(longBody),
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error with long body: %v", err)
	}
}

func TestNewSMTPSenderWithNilLogger(t *testing.T) {
	config := DefaultConfig()
	sender := NewSMTPSender(config, nil)

	if sender == nil {
		t.Fatal("expected sender to be created")
	}
	if sender.logger == nil {
		t.Error("expected logger to default to slog.Default()")
	}
}

func TestSMTPSender_SendPasswordResetEmailWithEmptyUserName(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.LogOnly = true
	config.AppURL = "https://kootenai.example.com"
	config.AppName = "Kootenai"

	sender := NewSMTPSender(config, nil)

	ctx := ctxWithBundle(t)
	err := sender.SendPasswordResetEmail(ctx, "user@example.com", "", "token123")
	if err != nil {
		t.Errorf("unexpected error with empty user name: %v", err)
	}
}
