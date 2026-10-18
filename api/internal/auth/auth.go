// Package auth provides simple JWT-based authentication
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Config holds authentication configuration
type Config struct {
	JWTSecret     string        `yaml:"jwt_secret"`
	JWTExpiration time.Duration `yaml:"jwt_expiration"`
	DemoMode      bool          `yaml:"demo_mode"`   // Explicitly enable demo mode (no auth required)
	DemoRole      string        `yaml:"demo_role"`   // Role for demo user (default: "student" for safety)
	CookieMode    bool          `yaml:"cookie_mode"` // Enable HttpOnly cookie auth (ADR-0002)
}

// DefaultConfig returns a default auth configuration
// Note: JWTSecret is intentionally empty to force explicit configuration
func DefaultConfig() Config {
	return Config{
		JWTSecret:     "", // Must be set via environment variable or config file
		JWTExpiration: 24 * time.Hour,
	}
}

// User represents an authenticated user
type User struct {
	ID                    string   `json:"id"`
	Email                 string   `json:"email"`
	Name                  string   `json:"name"`
	Roles                 []string `json:"roles"`
	IssuedAt              int64    `json:"iat"`
	DefaultOrganizationID string   `json:"defaultOrgId,omitempty"`
}

// Claims represents JWT claims
type Claims struct {
	UserID                string   `json:"uid"`
	Email                 string   `json:"email"`
	Name                  string   `json:"name"`
	Roles                 []string `json:"roles"`
	DefaultOrganizationID string   `json:"org,omitempty"`
	OrgRole               string   `json:"orgRole,omitempty"`
	jwt.RegisteredClaims
}

// TokenID returns the JWT ID (jti claim) for blacklisting purposes
func (c *Claims) TokenID() string {
	return c.RegisteredClaims.ID
}

// RemainingTTL returns the remaining time until the token expires
func (c *Claims) RemainingTTL() time.Duration {
	if c.ExpiresAt == nil {
		return 0
	}
	remaining := time.Until(c.ExpiresAt.Time)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// TokenBlacklistChecker checks if a token has been revoked.
// This interface enables token revocation without creating a dependency on the redis package.
type TokenBlacklistChecker interface {
	// IsBlacklisted checks if a token hash is in the blacklist
	IsBlacklisted(ctx context.Context, tokenHash string) (bool, error)
}

// Service handles authentication operations
type Service struct {
	config         Config
	tokenBlacklist TokenBlacklistChecker
}

// ServiceOption configures the auth service
type ServiceOption func(*Service)

// WithTokenBlacklist sets the token blacklist checker for validating revoked tokens
func WithTokenBlacklist(checker TokenBlacklistChecker) ServiceOption {
	return func(s *Service) {
		s.tokenBlacklist = checker
	}
}

// NewService creates a new auth service.
// Returns an error if:
//   - JWTSecret is empty and DemoMode is not enabled (security vulnerability)
//   - DemoMode is enabled in a production environment (ENV=production)
//   - JWTSecret is less than 32 characters without explicit override
func NewService(cfg Config, opts ...ServiceOption) (*Service, error) {
	logger := slog.Default()

	if cfg.JWTSecret == "" && !cfg.DemoMode {
		return nil, fmt.Errorf("SECURITY: JWT_SECRET must be set (minimum 32 characters recommended). Set AUTH_DEMO_MODE=true only for local development")
	}
	if cfg.DemoMode {
		env := strings.ToLower(os.Getenv("ENV"))
		if env == "production" || env == "prod" {
			return nil, fmt.Errorf("SECURITY: Demo mode cannot be enabled in production. Set ENV to something other than 'production' or disable AUTH_DEMO_MODE")
		}
		// Require explicit confirmation to enable demo mode
		confirm := os.Getenv("AUTH_DEMO_MODE_CONFIRM")
		if confirm != "I_UNDERSTAND_THE_RISKS" {
			return nil, fmt.Errorf("SECURITY: Demo mode requires explicit confirmation. " +
				"Set AUTH_DEMO_MODE_CONFIRM=I_UNDERSTAND_THE_RISKS to enable. " +
				"This mode bypasses authentication and should only be used for local development")
		}
		// Default to "student" role for safety (not "admin")
		if cfg.DemoRole == "" {
			cfg.DemoRole = "student"
		}
		// Log at ERROR level to ensure visibility - this is a security-relevant warning
		logger.Error("SECURITY WARNING: Demo mode is enabled",
			slog.String("warning", "All requests will be authenticated as 'demo-user'"),
			slog.String("demo_role", cfg.DemoRole),
			slog.String("action", "Do not use in production!"),
			slog.String("env", env),
		)
	}
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		env := strings.ToLower(os.Getenv("ENV"))
		// Always enforce minimum 32-character JWT secret for security
		// Only allow shorter secrets with explicit override in non-production environments
		if env == "production" || env == "prod" {
			return nil, fmt.Errorf("SECURITY: JWT_SECRET must be at least 32 characters in production")
		}
		// In development/test, require explicit acknowledgment of weak secret
		if os.Getenv("ALLOW_WEAK_JWT_SECRET") != "true" {
			return nil, fmt.Errorf("SECURITY: JWT_SECRET must be at least 32 characters (current: %d). "+
				"Set ALLOW_WEAK_JWT_SECRET=true to override in development (NOT recommended)", len(cfg.JWTSecret))
		}
		logger.Warn("SECURITY WARNING: Running with weak JWT_SECRET (development override active)",
			slog.Int("current_length", len(cfg.JWTSecret)),
			slog.Int("recommended_length", 32),
			slog.String("warning", "Do not deploy to production with this configuration"),
		)
	}
	s := &Service{config: cfg}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// IsDemoMode returns whether demo mode is enabled
func (s *Service) IsDemoMode() bool {
	return s.config.DemoMode
}

// DemoRole returns the role for demo users (defaults to "student" for safety)
func (s *Service) DemoRole() string {
	if s.config.DemoRole == "" {
		return "student"
	}
	return s.config.DemoRole
}

// GenerateToken generates a JWT token for a user
func (s *Service) GenerateToken(user *User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:                user.ID,
		Email:                 user.Email,
		Name:                  user.Name,
		Roles:                 user.Roles,
		DefaultOrganizationID: user.DefaultOrganizationID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // JTI for token blacklisting
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWTExpiration)),
			Issuer:    "kootenai-platform",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// HashToken creates a SHA-256 hash of a token for blacklist storage.
// This avoids storing the full token in Redis.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GenerateTokenWithOrgContext generates a JWT token for a user with organization context
func (s *Service) GenerateTokenWithOrgContext(user *User, orgID, orgRole string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:                user.ID,
		Email:                 user.Email,
		Name:                  user.Name,
		Roles:                 user.Roles,
		DefaultOrganizationID: orgID,
		OrgRole:               orgRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // JTI for token blacklisting
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWTExpiration)),
			Issuer:    "kootenai-platform",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// ValidateToken validates a JWT token and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// AuthSource indicates how the request was authenticated.
type AuthSource string

const (
	AuthSourceBearer AuthSource = "bearer"
	AuthSourceCookie AuthSource = "cookie"
	AuthSourceDemo   AuthSource = "demo"
)

// Context keys
type contextKey string

const (
	userContextKey       contextKey = "user"
	authSourceContextKey contextKey = "auth_source"
)

// UserFromContext retrieves the user from context
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// ContextWithUser adds a user to the context
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// AuthSourceFromContext returns how the current request was authenticated.
// Returns empty string if no auth source was recorded.
func AuthSourceFromContext(ctx context.Context) AuthSource {
	src, _ := ctx.Value(authSourceContextKey).(AuthSource)
	return src
}

func contextWithAuthSource(ctx context.Context, src AuthSource) context.Context {
	return context.WithValue(ctx, authSourceContextKey, src)
}

// Middleware creates an authentication middleware.
// If DemoMode is enabled in config, allows unauthenticated access with a demo user.
// Otherwise, requires valid JWT authentication via Bearer header.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if s.config.DemoMode {
				demoUser := &User{
					ID:    "00000000-0000-0000-0000-000000000001",
					Email: "demo@example.com",
					Name:  "Demo User",
					Roles: []string{s.DemoRole()},
				}
				ctx := ContextWithUser(r.Context(), demoUser)
				ctx = contextWithAuthSource(ctx, AuthSourceDemo)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		user, authErr := s.validateAndResolveUser(r.Context(), parts[1])
		if authErr != nil {
			writeAuthError(w, authErr)
			return
		}

		ctx := ContextWithUser(r.Context(), user)
		ctx = contextWithAuthSource(ctx, AuthSourceBearer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CookieOrBearerMiddleware creates an authentication middleware that checks
// for an auth cookie first, then falls back to the Authorization: Bearer header.
// When auth comes from a cookie, the CSRF middleware (wired separately) will
// enforce the double-submit token on mutating requests.
func (s *Service) CookieOrBearerMiddleware(cookieCfg CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenString string
			var source AuthSource

			// 1. Try cookie
			if cookieCfg.Enabled {
				if t := TokenFromCookie(r, cookieCfg); t != "" {
					tokenString = t
					source = AuthSourceCookie
				}
			}

			// 2. Fall back to Bearer header
			if tokenString == "" {
				if t, ok := bearerTokenFromHeader(r); ok {
					tokenString = t
					source = AuthSourceBearer
				}
			}

			// 3. Demo fallback
			if tokenString == "" {
				if s.config.DemoMode {
					demoUser := &User{
						ID:    "00000000-0000-0000-0000-000000000001",
						Email: "demo@example.com",
						Name:  "Demo User",
						Roles: []string{s.DemoRole()},
					}
					ctx := ContextWithUser(r.Context(), demoUser)
					ctx = contextWithAuthSource(ctx, AuthSourceDemo)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			user, authErr := s.validateAndResolveUser(r.Context(), tokenString)
			if authErr != nil {
				writeAuthError(w, authErr)
				return
			}

			ctx := ContextWithUser(r.Context(), user)
			ctx = contextWithAuthSource(ctx, source)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth creates a middleware that requires authentication (no demo fallback)
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		user, authErr := s.validateAndResolveUser(r.Context(), parts[1])
		if authErr != nil {
			writeAuthError(w, authErr)
			return
		}

		ctx := ContextWithUser(r.Context(), user)
		ctx = contextWithAuthSource(ctx, AuthSourceBearer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateAndResolveUser validates a token string and returns the User, or an
// authError describing the failure.
func (s *Service) validateAndResolveUser(ctx context.Context, tokenString string) (*User, *authError) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, &authError{msg: "Invalid token", status: http.StatusUnauthorized}
	}

	if s.tokenBlacklist != nil {
		tokenHash := HashToken(tokenString)
		blacklisted, blErr := s.tokenBlacklist.IsBlacklisted(ctx, tokenHash)
		if blErr != nil {
			slog.Error("Token blacklist check failed, rejecting request", "error", blErr)
			return nil, &authError{msg: "Authentication service unavailable", status: http.StatusServiceUnavailable}
		}
		if blacklisted {
			return nil, &authError{msg: "Token has been revoked", status: http.StatusUnauthorized}
		}
	}

	return &User{
		ID:                    claims.UserID,
		Email:                 claims.Email,
		Name:                  claims.Name,
		Roles:                 claims.Roles,
		DefaultOrganizationID: claims.DefaultOrganizationID,
	}, nil
}

type authError struct {
	msg    string
	status int
}

func writeAuthError(w http.ResponseWriter, e *authError) {
	http.Error(w, e.msg, e.status)
}

func bearerTokenFromHeader(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	parts := strings.Split(h, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	return parts[1], true
}

// RequireRole creates a middleware that requires a specific role
func (s *Service) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, r := range user.Roles {
				if r == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
