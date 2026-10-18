// Package auth provides authentication and security services
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// SecurityEventType defines types of security events to audit
type SecurityEventType string

const (
	// Authentication events
	EventLoginSuccess       SecurityEventType = "auth.login.success"
	EventLoginFailed        SecurityEventType = "auth.login.failed"
	EventLoginRateLimited   SecurityEventType = "auth.login.rate_limited"
	EventLogout             SecurityEventType = "auth.logout"
	EventTokenRefresh       SecurityEventType = "auth.token.refresh"        // #nosec G101 -- event type constant, not a credential
	EventTokenRefreshFailed SecurityEventType = "auth.token.refresh_failed" // #nosec G101 -- event type constant, not a credential

	// Password events
	EventPasswordChanged          SecurityEventType = "auth.password.changed"
	EventPasswordChangeFailed     SecurityEventType = "auth.password.change_failed"
	EventPasswordResetRequest     SecurityEventType = "auth.password.reset_request"
	EventPasswordResetComplete    SecurityEventType = "auth.password.reset_complete"
	EventPasswordResetFailed      SecurityEventType = "auth.password.reset_failed"
	EventPasswordResetRateLimited SecurityEventType = "auth.password.rate_limited"

	// Account events
	EventAccountCreated  SecurityEventType = "account.created"
	EventAccountLocked   SecurityEventType = "account.locked"
	EventAccountUnlocked SecurityEventType = "account.unlocked"
	EventAccountDeleted  SecurityEventType = "account.deleted"
	EventRoleChanged     SecurityEventType = "account.role_changed"

	// Access events
	EventUnauthorizedAccess SecurityEventType = "access.unauthorized"
	EventForbiddenAccess    SecurityEventType = "access.forbidden"
	EventSuspiciousActivity SecurityEventType = "access.suspicious"

	// Configuration events
	EventDemoModeEnabled SecurityEventType = "config.demo_mode_enabled"
	EventConfigChanged   SecurityEventType = "config.changed"
)

// SecurityAuditService provides security event logging
type SecurityAuditService struct {
	auditRepo repositories.AuditLogRepository
	logger    *slog.Logger
}

// NewSecurityAuditService creates a new security audit service
func NewSecurityAuditService(auditRepo repositories.AuditLogRepository, logger *slog.Logger) *SecurityAuditService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SecurityAuditService{
		auditRepo: auditRepo,
		logger:    logger,
	}
}

// SecurityEventDetails holds common security event details
type SecurityEventDetails struct {
	Event     SecurityEventType `json:"event"`
	UserID    string            `json:"userId,omitempty"`
	Username  string            `json:"username,omitempty"`
	Email     string            `json:"email,omitempty"` // Will be hashed for privacy
	IPAddress string            `json:"ipAddress,omitempty"`
	UserAgent string            `json:"userAgent,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Extra     map[string]any    `json:"extra,omitempty"`
	RequestID string            `json:"requestId,omitempty"`
}

// LogSecurityEvent logs a security event to both audit log and structured logger
func (s *SecurityAuditService) LogSecurityEvent(ctx context.Context, details SecurityEventDetails) {
	// Always log to structured logger for immediate visibility
	logLevel := s.getLogLevel(details.Event)
	attrs := []any{
		slog.String("event", string(details.Event)),
		slog.String("ip", details.IPAddress),
	}
	if details.UserID != "" {
		attrs = append(attrs, slog.String("userId", details.UserID))
	}
	if details.Username != "" {
		attrs = append(attrs, slog.String("username", details.Username))
	}
	if details.Reason != "" {
		attrs = append(attrs, slog.String("reason", details.Reason))
	}
	if details.RequestID != "" {
		attrs = append(attrs, slog.String("requestId", details.RequestID))
	}

	// Log with appropriate level
	switch logLevel {
	case slog.LevelError:
		s.logger.Error("Security event", attrs...)
	case slog.LevelWarn:
		s.logger.Warn("Security event", attrs...)
	default:
		s.logger.Info("Security event", attrs...)
	}

	// Also persist to audit log if repository is available
	if s.auditRepo != nil {
		s.persistAuditEntry(ctx, details)
	}
}

// getLogLevel returns the appropriate log level for an event type
func (s *SecurityAuditService) getLogLevel(event SecurityEventType) slog.Level {
	switch event {
	case EventLoginFailed, EventLoginRateLimited, EventPasswordChangeFailed,
		EventPasswordResetFailed, EventPasswordResetRateLimited,
		EventUnauthorizedAccess, EventForbiddenAccess, EventSuspiciousActivity,
		EventAccountLocked:
		return slog.LevelWarn
	case EventDemoModeEnabled:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// persistAuditEntry saves the event to the database audit log
func (s *SecurityAuditService) persistAuditEntry(ctx context.Context, details SecurityEventDetails) {
	// Hash email for privacy if present
	emailForLog := ""
	if details.Email != "" {
		emailForLog = hashEmail(details.Email)
	}

	// Build details JSON
	extra := details.Extra
	if extra == nil {
		extra = make(map[string]any)
	}
	extra["event_type"] = string(details.Event)
	if details.Reason != "" {
		extra["reason"] = details.Reason
	}
	if emailForLog != "" {
		extra["email_hash"] = emailForLog
	}
	if details.RequestID != "" {
		extra["request_id"] = details.RequestID
	}

	detailsJSON, _ := json.Marshal(extra)

	actorID := details.UserID
	if actorID == "" {
		actorID = "anonymous"
	}

	entry := &models.AuditEntry{
		ActorID:      actorID,
		ActorType:    "user",
		Action:       string(details.Event),
		ResourceType: "security",
		ResourceID:   details.UserID,
		Details:      detailsJSON,
		IPAddress:    details.IPAddress,
		UserAgent:    details.UserAgent,
	}

	if err := s.auditRepo.Create(ctx, entry); err != nil {
		s.logger.Error("Failed to persist security audit entry",
			slog.String("event", string(details.Event)),
			slog.Any("error", err),
		)
	}
}

// LogLoginSuccess logs a successful login
func (s *SecurityAuditService) LogLoginSuccess(ctx context.Context, userID, username, ip, userAgent string) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventLoginSuccess,
		UserID:    userID,
		Username:  username,
		IPAddress: ip,
		UserAgent: userAgent,
	})
}

// LogLoginFailed logs a failed login attempt
func (s *SecurityAuditService) LogLoginFailed(ctx context.Context, username, email, ip, userAgent, reason string) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventLoginFailed,
		Username:  username,
		Email:     email,
		IPAddress: ip,
		UserAgent: userAgent,
		Reason:    reason,
	})
}

// LogLoginRateLimited logs a rate-limited login attempt
func (s *SecurityAuditService) LogLoginRateLimited(ctx context.Context, ip, email string, retryAfter time.Duration) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventLoginRateLimited,
		Email:     email,
		IPAddress: ip,
		Reason:    "Rate limit exceeded",
		Extra: map[string]any{
			"retry_after_seconds": retryAfter.Seconds(),
		},
	})
}

// LogPasswordResetRequest logs a password reset request
func (s *SecurityAuditService) LogPasswordResetRequest(ctx context.Context, email, ip, userAgent string) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventPasswordResetRequest,
		Email:     email,
		IPAddress: ip,
		UserAgent: userAgent,
	})
}

// LogPasswordResetRateLimited logs a rate-limited password reset attempt
func (s *SecurityAuditService) LogPasswordResetRateLimited(ctx context.Context, ip, email string, retryAfter time.Duration) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventPasswordResetRateLimited,
		Email:     email,
		IPAddress: ip,
		Reason:    "Rate limit exceeded",
		Extra: map[string]any{
			"retry_after_seconds": retryAfter.Seconds(),
		},
	})
}

// LogUnauthorizedAccess logs an unauthorized access attempt
func (s *SecurityAuditService) LogUnauthorizedAccess(ctx context.Context, ip, userAgent, path, method string) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventUnauthorizedAccess,
		IPAddress: ip,
		UserAgent: userAgent,
		Extra: map[string]any{
			"path":   path,
			"method": method,
		},
	})
}

// LogForbiddenAccess logs a forbidden access attempt
func (s *SecurityAuditService) LogForbiddenAccess(ctx context.Context, userID, ip, userAgent, path, method, reason string) {
	s.LogSecurityEvent(ctx, SecurityEventDetails{
		Event:     EventForbiddenAccess,
		UserID:    userID,
		IPAddress: ip,
		UserAgent: userAgent,
		Reason:    reason,
		Extra: map[string]any{
			"path":   path,
			"method": method,
		},
	})
}

// LogFromRequest extracts common details from an HTTP request
func (s *SecurityAuditService) LogFromRequest(r *http.Request, event SecurityEventType, userID string, extra map[string]any) {
	s.LogSecurityEvent(r.Context(), SecurityEventDetails{
		Event:     event,
		UserID:    userID,
		IPAddress: GetRealIPFromRequest(r),
		UserAgent: r.UserAgent(),
		RequestID: r.Header.Get("X-Request-ID"),
		Extra:     extra,
	})
}

// GetRealIPFromRequest extracts the real client IP from a request.
// Prefers X-Real-IP (set by nginx, harder to spoof) over RemoteAddr.
// X-Forwarded-For is NOT used as it can be trivially spoofed by clients.
func GetRealIPFromRequest(r *http.Request) string {
	// Prefer X-Real-IP header (set by nginx, single value)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fallback to RemoteAddr (strip port)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		if strings.Contains(addr, "[") {
			if bracketEnd := strings.Index(addr, "]"); bracketEnd != -1 {
				return addr[1:bracketEnd]
			}
		}
		return addr[:idx]
	}
	return addr
}

// hashEmail creates a SHA-256 hash of an email for privacy-preserving logging
func hashEmail(email string) string {
	// Normalize email to lowercase before hashing
	email = strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(email))
	// Return first 12 characters of hex (enough for correlation, not reversible)
	return hex.EncodeToString(hash[:])[:12]
}

// HashForLog returns a privacy-preserving hash of sensitive data for logging
func HashForLog(data string) string {
	if data == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:12]
}
