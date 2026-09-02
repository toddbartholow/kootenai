package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// JWTSecret should be empty to force explicit configuration
	if cfg.JWTSecret != "" {
		t.Error("JWTSecret should be empty by default to force explicit configuration")
	}
	if cfg.JWTExpiration == 0 {
		t.Error("JWTExpiration should have a default value")
	}
	if cfg.DemoMode {
		t.Error("DemoMode should default to false for security")
	}
}

func TestNewServiceErrorsWithEmptySecret(t *testing.T) {
	cfg := Config{
		JWTSecret:     "",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false,
	}
	_, err := NewService(cfg)
	require.Error(t, err)
}

func TestNewServiceErrorsWithDemoModeWithoutConfirmation(t *testing.T) {
	originalEnv := os.Getenv("ENV")
	originalConfirm := os.Getenv("AUTH_DEMO_MODE_CONFIRM")
	os.Setenv("ENV", "development")
	os.Unsetenv("AUTH_DEMO_MODE_CONFIRM")
	defer os.Setenv("ENV", originalEnv)
	defer func() {
		if originalConfirm != "" {
			os.Setenv("AUTH_DEMO_MODE_CONFIRM", originalConfirm)
		}
	}()

	cfg := Config{
		JWTSecret:     "",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      true,
	}
	_, err := NewService(cfg)
	require.Error(t, err)
}

func TestNewServiceAllowsEmptySecretInDemoMode(t *testing.T) {
	// Ensure we're not in "production" environment for this test
	originalEnv := os.Getenv("ENV")
	originalConfirm := os.Getenv("AUTH_DEMO_MODE_CONFIRM")
	os.Setenv("ENV", "development")
	os.Setenv("AUTH_DEMO_MODE_CONFIRM", "I_UNDERSTAND_THE_RISKS")
	defer os.Setenv("ENV", originalEnv)
	defer func() {
		if originalConfirm != "" {
			os.Setenv("AUTH_DEMO_MODE_CONFIRM", originalConfirm)
		} else {
			os.Unsetenv("AUTH_DEMO_MODE_CONFIRM")
		}
	}()

	cfg := Config{
		JWTSecret:     "",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      true,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)
	if svc == nil {
		t.Error("expected service to be created")
	}
}

func TestNewServiceErrorsWithDemoModeInProduction(t *testing.T) {
	originalEnv := os.Getenv("ENV")
	os.Setenv("ENV", "production")
	defer os.Setenv("ENV", originalEnv)

	cfg := Config{
		JWTSecret:     "",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      true,
	}
	_, err := NewService(cfg)
	require.Error(t, err)
}

func TestNewServiceErrorsWithShortSecretInProduction(t *testing.T) {
	originalEnv := os.Getenv("ENV")
	os.Setenv("ENV", "production")
	defer os.Setenv("ENV", originalEnv)

	cfg := Config{
		JWTSecret:     "short-secret", // Less than 32 characters
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false,
	}
	_, err := NewService(cfg)
	require.Error(t, err)
}

func TestNewServiceErrorsOnShortSecretWithoutOverride(t *testing.T) {
	originalEnv := os.Getenv("ENV")
	originalOverride := os.Getenv("ALLOW_WEAK_JWT_SECRET")
	os.Setenv("ENV", "development")
	os.Unsetenv("ALLOW_WEAK_JWT_SECRET")
	defer os.Setenv("ENV", originalEnv)
	defer func() {
		if originalOverride != "" {
			os.Setenv("ALLOW_WEAK_JWT_SECRET", originalOverride)
		}
	}()

	cfg := Config{
		JWTSecret:     "short-secret", // Less than 32 characters
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false,
	}
	_, err := NewService(cfg)
	require.Error(t, err)
}

func TestNewServiceAllowsShortSecretWithOverride(t *testing.T) {
	originalEnv := os.Getenv("ENV")
	originalOverride := os.Getenv("ALLOW_WEAK_JWT_SECRET")
	os.Setenv("ENV", "development")
	os.Setenv("ALLOW_WEAK_JWT_SECRET", "true")
	defer os.Setenv("ENV", originalEnv)
	defer func() {
		if originalOverride != "" {
			os.Setenv("ALLOW_WEAK_JWT_SECRET", originalOverride)
		} else {
			os.Unsetenv("ALLOW_WEAK_JWT_SECRET")
		}
	}()

	cfg := Config{
		JWTSecret:     "short-secret", // Less than 32 characters
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false,
	}
	svc, err := NewService(cfg) // Should warn but not error with override
	require.NoError(t, err)
	if svc == nil {
		t.Error("expected service to be created")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	user := &User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}

	// Generate token
	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Validate token
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("expected UserID %s, got %s", user.ID, claims.UserID)
	}
	if claims.Email != user.Email {
		t.Errorf("expected Email %s, got %s", user.Email, claims.Email)
	}
	if claims.Name != user.Name {
		t.Errorf("expected Name %s, got %s", user.Name, claims.Name)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "student" {
		t.Errorf("expected Roles [student], got %v", claims.Roles)
	}
}

func TestValidateInvalidToken(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "malformed token",
			token: "not.a.valid.token",
		},
		{
			name:  "wrong signature",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ValidateToken(tt.token)
			if err == nil {
				t.Error("expected error for invalid token")
			}
		})
	}
}

func TestMiddlewareWithDemoModeEnabled(t *testing.T) {
	// Set required confirmation for demo mode
	originalConfirm := os.Getenv("AUTH_DEMO_MODE_CONFIRM")
	os.Setenv("AUTH_DEMO_MODE_CONFIRM", "I_UNDERSTAND_THE_RISKS")
	defer func() {
		if originalConfirm != "" {
			os.Setenv("AUTH_DEMO_MODE_CONFIRM", originalConfirm)
		} else {
			os.Unsetenv("AUTH_DEMO_MODE_CONFIRM")
		}
	}()

	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      true, // Demo mode enabled
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("expected user in context")
			http.Error(w, "no user", http.StatusInternalServerError)
			return
		}
		// Demo user now uses a valid UUID for database compatibility
		expectedDemoID := "00000000-0000-0000-0000-000000000001"
		if user.ID != expectedDemoID {
			t.Errorf("expected %s, got %s", expectedDemoID, user.ID)
		}
		// Demo user should have "student" role by default (not "admin")
		if len(user.Roles) != 1 || user.Roles[0] != "student" {
			t.Errorf("expected demo user to have 'student' role, got %v", user.Roles)
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Request without Authorization header
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestMiddlewareWithDemoModeDisabled(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false, // Demo mode disabled (default)
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without Authorization header should fail
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMiddlewareWithValidToken(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	user := &User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("expected user in context")
			http.Error(w, "no user", http.StatusInternalServerError)
			return
		}
		if u.ID != user.ID {
			t.Errorf("expected user ID %s, got %s", user.ID, u.ID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestMiddlewareWithInvalidAuthHeader(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
		DemoMode:      false,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		authHeader string
	}{
		{
			name:       "missing Bearer prefix",
			authHeader: "some-token",
		},
		{
			name:       "wrong prefix",
			authHeader: "Basic some-token",
		},
		{
			name:       "empty Bearer",
			authHeader: "Bearer ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})
	}
}

func TestRequireAuth(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	handler := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("no auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("with valid token", func(t *testing.T) {
		user := &User{ID: "user-1", Email: "test@example.com", Name: "Test", Roles: []string{"student"}}
		token, _ := svc.GenerateToken(user)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestRequireRole(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	adminHandler := svc.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("user without required role", func(t *testing.T) {
		user := &User{ID: "user-1", Email: "test@example.com", Name: "Test", Roles: []string{"student"}}
		token, _ := svc.GenerateToken(user)

		// First authenticate via Middleware
		handler := svc.Middleware(adminHandler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("user with required role", func(t *testing.T) {
		user := &User{ID: "user-1", Email: "admin@example.com", Name: "Admin", Roles: []string{"admin"}}
		token, _ := svc.GenerateToken(user)

		handler := svc.Middleware(adminHandler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestUserContext(t *testing.T) {
	user := &User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := ContextWithUser(req.Context(), user)

	retrievedUser, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("expected to retrieve user from context")
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, retrievedUser.ID)
	}
	if retrievedUser.Email != user.Email {
		t.Errorf("expected Email %s, got %s", user.Email, retrievedUser.Email)
	}
}

func TestUserFromContextNotSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, ok := UserFromContext(req.Context())
	if ok {
		t.Error("expected no user in context")
	}
}

func TestHashToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"simple token", "abc123"},
		{"JWT-like token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"},
		{"empty token", ""},
		{"unicode token", "日本語トークン"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := HashToken(tt.token)
			hash2 := HashToken(tt.token)

			// Hash should be deterministic
			if hash1 != hash2 {
				t.Error("HashToken should return the same hash for the same input")
			}

			// Hash should be 64 hex characters (SHA-256 = 256 bits = 32 bytes = 64 hex chars)
			if len(hash1) != 64 {
				t.Errorf("HashToken should return 64 hex chars, got %d", len(hash1))
			}

			// Hash should be different for different inputs
			if tt.token != "" {
				differentHash := HashToken(tt.token + "x")
				if hash1 == differentHash {
					t.Error("HashToken should return different hashes for different inputs")
				}
			}
		})
	}
}

func TestClaimsTokenID(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	user := &User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}

	// Generate token
	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Validate and check TokenID
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	tokenID := claims.TokenID()
	if tokenID == "" {
		t.Error("TokenID() should return a non-empty JTI")
	}

	// Generate another token - should have different JTI
	token2, _ := svc.GenerateToken(user)
	claims2, _ := svc.ValidateToken(token2)
	tokenID2 := claims2.TokenID()

	if tokenID == tokenID2 {
		t.Error("Different tokens should have different JTIs")
	}
}

func TestClaimsRemainingTTL(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	user := &User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}

	// Generate token
	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Validate and check RemainingTTL
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	ttl := claims.RemainingTTL()

	// TTL should be close to the configured expiration (1 hour)
	// Allow for some variance due to test execution time
	if ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Errorf("RemainingTTL() = %v, expected ~1 hour", ttl)
	}
}

func TestClaimsRemainingTTL_Expired(t *testing.T) {
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: -1 * time.Hour, // Negative expiration for expired token
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)

	user := &User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}

	// Generate already-expired token
	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Validation should fail for expired token
	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken should fail for expired token")
	}
}

// mockBlacklist implements TokenBlacklistChecker for testing
type mockBlacklist struct {
	blacklisted bool
	err         error
}

func (m *mockBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return m.blacklisted, m.err
}

func newBlacklistTestService(t *testing.T, bl TokenBlacklistChecker) (*Service, string) {
	t.Helper()
	cfg := Config{
		JWTSecret:     "test-secret-key-that-is-at-least-32-chars",
		JWTExpiration: 1 * time.Hour,
	}
	svc, err := NewService(cfg, WithTokenBlacklist(bl))
	require.NoError(t, err)
	user := &User{ID: "user-1", Email: "test@example.com", Name: "Test", Roles: []string{"student"}}
	token, _ := svc.GenerateToken(user)
	return svc, token
}

func TestMiddleware_BlacklistError_Returns503(t *testing.T) {
	bl := &mockBlacklist{err: errors.New("redis connection refused")}
	svc, token := newBlacklistTestService(t, bl)

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when blacklist check fails, got %d", rr.Code)
	}
}

func TestMiddleware_BlacklistRevoked_Returns401(t *testing.T) {
	bl := &mockBlacklist{blacklisted: true}
	svc, token := newBlacklistTestService(t, bl)

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked token, got %d", rr.Code)
	}
}

func TestMiddleware_BlacklistNotRevoked_PassesThrough(t *testing.T) {
	bl := &mockBlacklist{blacklisted: false}
	svc, token := newBlacklistTestService(t, bl)

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for non-revoked token, got %d", rr.Code)
	}
}

func TestRequireAuth_BlacklistError_Returns503(t *testing.T) {
	bl := &mockBlacklist{err: errors.New("redis timeout")}
	svc, token := newBlacklistTestService(t, bl)

	handler := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when blacklist check fails, got %d", rr.Code)
	}
}

func TestRequireAuth_BlacklistRevoked_Returns401(t *testing.T) {
	bl := &mockBlacklist{blacklisted: true}
	svc, token := newBlacklistTestService(t, bl)

	handler := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked token, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// CookieOrBearerMiddleware tests
// ---------------------------------------------------------------------------

func newCookieTestService(t *testing.T) (*Service, string) {
	t.Helper()
	svc, err := NewService(Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.GenerateToken(&User{
		ID:    "user-1",
		Email: "test@example.com",
		Name:  "Test",
		Roles: []string{"student"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, token
}

func TestCookieOrBearerMiddleware_CookieAuth(t *testing.T) {
	svc, token := newCookieTestService(t)
	cfg := CookieConfig{Enabled: true, Name: DefaultCookieName}

	var capturedSource AuthSource
	handler := svc.CookieOrBearerMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSource = AuthSourceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedSource != AuthSourceCookie {
		t.Errorf("auth source = %q, want %q", capturedSource, AuthSourceCookie)
	}
}

func TestCookieOrBearerMiddleware_BearerFallback(t *testing.T) {
	svc, token := newCookieTestService(t)
	cfg := CookieConfig{Enabled: true, Name: DefaultCookieName}

	var capturedSource AuthSource
	handler := svc.CookieOrBearerMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSource = AuthSourceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedSource != AuthSourceBearer {
		t.Errorf("auth source = %q, want %q", capturedSource, AuthSourceBearer)
	}
}

func TestCookieOrBearerMiddleware_CookiePreferredOverBearer(t *testing.T) {
	svc, token := newCookieTestService(t)
	cfg := CookieConfig{Enabled: true, Name: DefaultCookieName}

	var capturedSource AuthSource
	handler := svc.CookieOrBearerMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSource = AuthSourceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: token})
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedSource != AuthSourceCookie {
		t.Errorf("auth source = %q, want cookie (preferred), got %q", capturedSource, AuthSourceCookie)
	}
}

func TestCookieOrBearerMiddleware_NoAuth(t *testing.T) {
	svc, _ := newCookieTestService(t)
	cfg := CookieConfig{Enabled: true, Name: DefaultCookieName}

	handler := svc.CookieOrBearerMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCookieOrBearerMiddleware_InvalidCookieToken(t *testing.T) {
	svc, _ := newCookieTestService(t)
	cfg := CookieConfig{Enabled: true, Name: DefaultCookieName}

	handler := svc.CookieOrBearerMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: "invalid-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid cookie token, got %d", rr.Code)
	}
}
