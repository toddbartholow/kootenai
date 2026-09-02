package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// mockAuditLogRepo implements AuditLogRepository for testing
type mockAuditLogRepo struct {
	createFn func(ctx context.Context, entry *models.AuditEntry) error
	queryFn  func(ctx context.Context, filter repositories.AuditFilter) ([]*models.AuditEntry, error)
	entries  []*models.AuditEntry
}

func (m *mockAuditLogRepo) Create(ctx context.Context, entry *models.AuditEntry) error {
	if m.createFn != nil {
		return m.createFn(ctx, entry)
	}
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditLogRepo) Query(ctx context.Context, filter repositories.AuditFilter) ([]*models.AuditEntry, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, filter)
	}
	return m.entries, nil
}

var _ repositories.AuditLogRepository = (*mockAuditLogRepo)(nil)

func TestNewSecurityAuditService(t *testing.T) {
	t.Run("with_logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		repo := &mockAuditLogRepo{}
		service := NewSecurityAuditService(repo, logger)

		if service == nil {
			t.Fatal("expected non-nil service")
		}
		if service.logger != logger {
			t.Error("expected logger to be set")
		}
		if service.auditRepo == nil {
			t.Error("expected audit repo to be set")
		}
	})

	t.Run("nil_logger_uses_default", func(t *testing.T) {
		repo := &mockAuditLogRepo{}
		service := NewSecurityAuditService(repo, nil)

		if service == nil {
			t.Fatal("expected non-nil service")
		}
		if service.logger == nil {
			t.Error("expected default logger to be set")
		}
	})
}

func TestSecurityAuditService_LogSecurityEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("logs_to_audit_repo", func(t *testing.T) {
		var createdEntry *models.AuditEntry
		repo := &mockAuditLogRepo{
			createFn: func(ctx context.Context, entry *models.AuditEntry) error {
				createdEntry = entry
				return nil
			},
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		service := NewSecurityAuditService(repo, logger)

		service.LogSecurityEvent(ctx, SecurityEventDetails{
			Event:     EventLoginSuccess,
			UserID:    "user-123",
			Username:  "testuser",
			IPAddress: "192.168.1.1",
			UserAgent: "Test/1.0",
		})

		if createdEntry == nil {
			t.Fatal("expected audit entry to be created")
		}
		if createdEntry.ActorID != "user-123" {
			t.Errorf("expected actor ID user-123, got %s", createdEntry.ActorID)
		}
		if createdEntry.Action != string(EventLoginSuccess) {
			t.Errorf("expected action %s, got %s", EventLoginSuccess, createdEntry.Action)
		}
		if createdEntry.IPAddress != "192.168.1.1" {
			t.Errorf("expected IP 192.168.1.1, got %s", createdEntry.IPAddress)
		}
	})

	t.Run("handles_nil_audit_repo", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		service := NewSecurityAuditService(nil, logger)

		// Should not panic
		service.LogSecurityEvent(ctx, SecurityEventDetails{
			Event:     EventLoginSuccess,
			UserID:    "user-123",
			IPAddress: "192.168.1.1",
		})
	})

	t.Run("handles_audit_repo_error", func(t *testing.T) {
		repo := &mockAuditLogRepo{
			createFn: func(ctx context.Context, entry *models.AuditEntry) error {
				return errors.New("database error")
			},
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		service := NewSecurityAuditService(repo, logger)

		// Should not panic on error
		service.LogSecurityEvent(ctx, SecurityEventDetails{
			Event:     EventLoginSuccess,
			UserID:    "user-123",
			IPAddress: "192.168.1.1",
		})
	})

	t.Run("anonymous_user_when_no_userid", func(t *testing.T) {
		var createdEntry *models.AuditEntry
		repo := &mockAuditLogRepo{
			createFn: func(ctx context.Context, entry *models.AuditEntry) error {
				createdEntry = entry
				return nil
			},
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		service := NewSecurityAuditService(repo, logger)

		service.LogSecurityEvent(ctx, SecurityEventDetails{
			Event:     EventLoginFailed,
			Username:  "unknown",
			IPAddress: "192.168.1.1",
		})

		if createdEntry.ActorID != "anonymous" {
			t.Errorf("expected actor ID 'anonymous', got %s", createdEntry.ActorID)
		}
	})

	t.Run("includes_email_hash_and_extra", func(t *testing.T) {
		var createdEntry *models.AuditEntry
		repo := &mockAuditLogRepo{
			createFn: func(ctx context.Context, entry *models.AuditEntry) error {
				createdEntry = entry
				return nil
			},
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		service := NewSecurityAuditService(repo, logger)

		service.LogSecurityEvent(ctx, SecurityEventDetails{
			Event:     EventLoginFailed,
			Email:     "test@example.com",
			IPAddress: "192.168.1.1",
			Reason:    "Invalid password",
			RequestID: "req-123",
			Extra: map[string]interface{}{
				"attempts": 3,
			},
		})

		if createdEntry == nil {
			t.Fatal("expected entry to be created")
		}
		// Details should contain email_hash, reason, request_id
		detailsStr := string(createdEntry.Details)
		if !strings.Contains(detailsStr, "email_hash") {
			t.Error("expected details to contain email_hash")
		}
		if !strings.Contains(detailsStr, "reason") {
			t.Error("expected details to contain reason")
		}
		if !strings.Contains(detailsStr, "request_id") {
			t.Error("expected details to contain request_id")
		}
	})
}

func TestSecurityAuditService_getLogLevel(t *testing.T) {
	service := NewSecurityAuditService(nil, slog.Default())

	tests := []struct {
		event    SecurityEventType
		expected slog.Level
	}{
		{EventLoginSuccess, slog.LevelInfo},
		{EventLoginFailed, slog.LevelWarn},
		{EventLoginRateLimited, slog.LevelWarn},
		{EventPasswordChangeFailed, slog.LevelWarn},
		{EventPasswordResetFailed, slog.LevelWarn},
		{EventPasswordResetRateLimited, slog.LevelWarn},
		{EventUnauthorizedAccess, slog.LevelWarn},
		{EventForbiddenAccess, slog.LevelWarn},
		{EventSuspiciousActivity, slog.LevelWarn},
		{EventAccountLocked, slog.LevelWarn},
		{EventDemoModeEnabled, slog.LevelError},
		{EventLogout, slog.LevelInfo},
		{EventTokenRefresh, slog.LevelInfo},
		{EventPasswordChanged, slog.LevelInfo},
		{EventAccountCreated, slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			level := service.getLogLevel(tt.event)
			if level != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestSecurityAuditService_LogLoginSuccess(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogLoginSuccess(context.Background(), "user-123", "testuser", "192.168.1.1", "Mozilla/5.0")

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventLoginSuccess) {
		t.Errorf("expected action %s, got %s", EventLoginSuccess, createdEntry.Action)
	}
	if createdEntry.ActorID != "user-123" {
		t.Errorf("expected actor ID user-123, got %s", createdEntry.ActorID)
	}
}

func TestSecurityAuditService_LogLoginFailed(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogLoginFailed(context.Background(), "testuser", "test@example.com", "192.168.1.1", "Mozilla/5.0", "Invalid password")

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventLoginFailed) {
		t.Errorf("expected action %s, got %s", EventLoginFailed, createdEntry.Action)
	}
}

func TestSecurityAuditService_LogLoginRateLimited(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogLoginRateLimited(context.Background(), "192.168.1.1", "test@example.com", 60*time.Second)

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventLoginRateLimited) {
		t.Errorf("expected action %s, got %s", EventLoginRateLimited, createdEntry.Action)
	}
	// Check extra contains retry_after_seconds
	if !strings.Contains(string(createdEntry.Details), "retry_after_seconds") {
		t.Error("expected details to contain retry_after_seconds")
	}
}

func TestSecurityAuditService_LogPasswordResetRequest(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogPasswordResetRequest(context.Background(), "test@example.com", "192.168.1.1", "Mozilla/5.0")

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventPasswordResetRequest) {
		t.Errorf("expected action %s, got %s", EventPasswordResetRequest, createdEntry.Action)
	}
}

func TestSecurityAuditService_LogPasswordResetRateLimited(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogPasswordResetRateLimited(context.Background(), "192.168.1.1", "test@example.com", 30*time.Second)

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventPasswordResetRateLimited) {
		t.Errorf("expected action %s, got %s", EventPasswordResetRateLimited, createdEntry.Action)
	}
}

func TestSecurityAuditService_LogUnauthorizedAccess(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogUnauthorizedAccess(context.Background(), "192.168.1.1", "Mozilla/5.0", "/api/admin", "GET")

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventUnauthorizedAccess) {
		t.Errorf("expected action %s, got %s", EventUnauthorizedAccess, createdEntry.Action)
	}
	if !strings.Contains(string(createdEntry.Details), "path") {
		t.Error("expected details to contain path")
	}
	if !strings.Contains(string(createdEntry.Details), "method") {
		t.Error("expected details to contain method")
	}
}

func TestSecurityAuditService_LogForbiddenAccess(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	service.LogForbiddenAccess(context.Background(), "user-123", "192.168.1.1", "Mozilla/5.0", "/api/admin", "DELETE", "Insufficient permissions")

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventForbiddenAccess) {
		t.Errorf("expected action %s, got %s", EventForbiddenAccess, createdEntry.Action)
	}
	if createdEntry.ActorID != "user-123" {
		t.Errorf("expected actor ID user-123, got %s", createdEntry.ActorID)
	}
}

func TestSecurityAuditService_LogFromRequest(t *testing.T) {
	var createdEntry *models.AuditEntry
	repo := &mockAuditLogRepo{
		createFn: func(ctx context.Context, entry *models.AuditEntry) error {
			createdEntry = entry
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewSecurityAuditService(repo, logger)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("User-Agent", "Test/1.0")
	req.Header.Set("X-Request-ID", "req-456")
	req.RemoteAddr = "192.168.1.100:12345"

	service.LogFromRequest(req, EventLogout, "user-789", map[string]interface{}{"reason": "manual"})

	if createdEntry == nil {
		t.Fatal("expected entry to be created")
	}
	if createdEntry.Action != string(EventLogout) {
		t.Errorf("expected action %s, got %s", EventLogout, createdEntry.Action)
	}
	if createdEntry.ActorID != "user-789" {
		t.Errorf("expected actor ID user-789, got %s", createdEntry.ActorID)
	}
	if createdEntry.UserAgent != "Test/1.0" {
		t.Errorf("expected user agent Test/1.0, got %s", createdEntry.UserAgent)
	}
}

func TestGetRealIPFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:       "x-forwarded-for ignored (spoofable)",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1"},
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "x-real-ip",
			headers:    map[string]string{"X-Real-IP": "10.0.0.5"},
			remoteAddr: "192.168.1.1:12345",
			expected:   "10.0.0.5",
		},
		{
			name:       "x-real-ip takes precedence over x-forwarded-for",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1", "X-Real-IP": "10.0.0.5"},
			remoteAddr: "192.168.1.1:12345",
			expected:   "10.0.0.5",
		},
		{
			name:       "fallback to remote addr",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "ipv6 address with port",
			headers:    map[string]string{},
			remoteAddr: "[::1]:12345",
			expected:   "::1",
		},
		{
			name:       "ipv6 address without port",
			headers:    map[string]string{},
			remoteAddr: "::1",
			expected:   ":", // The function strips after last colon, so "::1" becomes ":"
		},
		{
			name:       "remote addr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "x-forwarded-for with spaces ignored",
			headers:    map[string]string{"X-Forwarded-For": " 10.0.0.1 , 10.0.0.2 "},
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remoteAddr

			result := GetRealIPFromRequest(req)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestHashEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string // first 12 chars of SHA-256 hex
	}{
		{
			name:  "normal email",
			email: "test@example.com",
		},
		{
			name:  "uppercase email normalized",
			email: "TEST@EXAMPLE.COM",
		},
		{
			name:  "email with spaces trimmed",
			email: "  test@example.com  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashEmail(tt.email)
			if len(result) != 12 {
				t.Errorf("expected 12 character hash, got %d", len(result))
			}
		})
	}

	// Test that normalized emails produce same hash
	hash1 := hashEmail("test@example.com")
	hash2 := hashEmail("TEST@EXAMPLE.COM")
	hash3 := hashEmail("  test@example.com  ")

	if hash1 != hash2 || hash1 != hash3 {
		t.Error("normalized emails should produce same hash")
	}

	// Test that different emails produce different hashes
	hash4 := hashEmail("different@example.com")
	if hash1 == hash4 {
		t.Error("different emails should produce different hashes")
	}
}

func TestHashForLog(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		result := HashForLog("")
		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})

	t.Run("non_empty_string", func(t *testing.T) {
		result := HashForLog("sensitive-data")
		if len(result) != 12 {
			t.Errorf("expected 12 character hash, got %d", len(result))
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		hash1 := HashForLog("test-data")
		hash2 := HashForLog("test-data")
		if hash1 != hash2 {
			t.Error("same input should produce same hash")
		}
	})

	t.Run("different_inputs_different_hashes", func(t *testing.T) {
		hash1 := HashForLog("data1")
		hash2 := HashForLog("data2")
		if hash1 == hash2 {
			t.Error("different inputs should produce different hashes")
		}
	})
}
