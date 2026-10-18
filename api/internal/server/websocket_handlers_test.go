package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// -----------------------------------------------------------------------------
// extractWebSocketToken Tests
// -----------------------------------------------------------------------------

func TestExtractWebSocketToken(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		authHeader string
		queryToken string
		wantToken  string
	}{
		{
			name:       "bearer token from header",
			authHeader: "Bearer test-jwt-token-123",
			wantToken:  "test-jwt-token-123",
		},
		{
			name:       "raw token from header (no Bearer prefix)",
			authHeader: "raw-token-456",
			wantToken:  "raw-token-456",
		},
		{
			name:       "query parameter token is ignored (removed for security)",
			queryToken: "query-token-789",
			wantToken:  "",
		},
		{
			name:       "header works even with query param present",
			authHeader: "Bearer header-token",
			queryToken: "query-token",
			wantToken:  "header-token",
		},
		{
			name:      "no token",
			wantToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/ws"
			if tt.queryToken != "" {
				url += "?token=" + tt.queryToken
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			got := srv.webSocketMgr.extractWebSocketToken(req)
			if got != tt.wantToken {
				t.Errorf("extractWebSocketToken() = %q, want %q", got, tt.wantToken)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// authenticateWebSocket Tests
// -----------------------------------------------------------------------------

func TestAuthenticateWebSocket_NoAuthService(t *testing.T) {
	// Server without auth service configured
	srv := newTestServer(t)

	tests := []struct {
		name       string
		queryParam string
		wantUserID string
		wantOK     bool
	}{
		{
			name:       "fixed anonymous ID when no userId provided",
			wantUserID: "00000000-0000-0000-0000-000000000001",
			wantOK:     true,
		},
		{
			name:       "ignores userId from query to prevent impersonation",
			queryParam: "userId=user-123",
			wantUserID: "00000000-0000-0000-0000-000000000001",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/ws"
			if tt.queryParam != "" {
				url += "?" + tt.queryParam
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)

			userID, ok := srv.webSocketMgr.authenticateWebSocket(req)
			if ok != tt.wantOK {
				t.Errorf("authenticateWebSocket() ok = %v, want %v", ok, tt.wantOK)
			}
			if userID != tt.wantUserID {
				t.Errorf("authenticateWebSocket() userID = %q, want %q", userID, tt.wantUserID)
			}
		})
	}
}

func TestAuthenticateWebSocket_DemoMode(t *testing.T) {
	// Set up demo mode environment
	os.Setenv("AUTH_DEMO_MODE_CONFIRM", "I_UNDERSTAND_THE_RISKS")
	defer os.Unsetenv("AUTH_DEMO_MODE_CONFIRM")

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "", // Empty is OK in demo mode
		JWTExpiration: time.Hour,
		DemoMode:      true,
	})
	require.NoError(t, err)

	srv := newTestServerWithAuth(t, authSvc)

	// In demo mode, should always return fixed demo user ID
	// regardless of any query params (to prevent impersonation)
	req := httptest.NewRequest(http.MethodGet, "/ws?userId=attacker-id", nil)

	userID, ok := srv.webSocketMgr.authenticateWebSocket(req)
	if !ok {
		t.Error("authenticateWebSocket() should succeed in demo mode")
	}
	// Should use fixed demo user ID, NOT the attacker-provided ID
	expectedDemoUserID := "00000000-0000-0000-0000-000000000001"
	if userID != expectedDemoUserID {
		t.Errorf("authenticateWebSocket() userID = %q, want %q (demo mode should ignore query userId)", userID, expectedDemoUserID)
	}
}

func TestAuthenticateWebSocket_WithAuth(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
		DemoMode:      false,
	})
	require.NoError(t, err)

	srv := newTestServerWithAuth(t, authSvc)

	t.Run("valid token authenticates successfully", func(t *testing.T) {
		// Generate a valid token
		token, err := authSvc.GenerateToken(&auth.User{
			ID:    "user-123",
			Email: "test@example.com",
			Name:  "Test User",
			Roles: []string{"student"},
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		userID, ok := srv.webSocketMgr.authenticateWebSocket(req)
		if !ok {
			t.Error("authenticateWebSocket() should succeed with valid token")
		}
		if userID != "user-123" {
			t.Errorf("authenticateWebSocket() userID = %q, want %q", userID, "user-123")
		}
	})

	t.Run("no token fails authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)

		_, ok := srv.webSocketMgr.authenticateWebSocket(req)
		if ok {
			t.Error("authenticateWebSocket() should fail without token")
		}
	})

	t.Run("invalid token fails authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		_, ok := srv.webSocketMgr.authenticateWebSocket(req)
		if ok {
			t.Error("authenticateWebSocket() should fail with invalid token")
		}
	})

	t.Run("expired token fails authentication", func(t *testing.T) {
		// Create auth service with very short expiration for testing
		shortAuthSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Millisecond, // Very short
			DemoMode:      false,
		})
		require.NoError(t, err)

		token, err := shortAuthSvc.GenerateToken(&auth.User{
			ID:    "user-123",
			Email: "test@example.com",
			Name:  "Test User",
			Roles: []string{"student"},
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		_, ok := srv.webSocketMgr.authenticateWebSocket(req)
		if ok {
			t.Error("authenticateWebSocket() should fail with expired token")
		}
	})

	t.Run("query parameter token is ignored for security", func(t *testing.T) {
		token, err := authSvc.GenerateToken(&auth.User{
			ID:    "user-456",
			Email: "query@example.com",
			Name:  "Query User",
			Roles: []string{"student"},
		})
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/ws?token="+token, nil)

		_, ok := srv.webSocketMgr.authenticateWebSocket(req)
		if ok {
			t.Error("authenticateWebSocket() should fail with token only in query parameter")
		}
	})
}

// -----------------------------------------------------------------------------
// handleWebSocket Tests
// -----------------------------------------------------------------------------

func TestHandleWebSocket_AuthenticationRequired(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
		DemoMode:      false,
	})
	require.NoError(t, err)

	srv := newTestServerWithAuth(t, authSvc)

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})
}

func TestHandleWebSocket_InvalidSession(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
		DemoMode:      false,
	})
	require.NoError(t, err)

	srv := newTestServerWithAuth(t, authSvc)

	token, err := authSvc.GenerateToken(&auth.User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	t.Run("rejects request with invalid sessionId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?sessionId=invalid-session-id", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleWebSocketPod Tests
// -----------------------------------------------------------------------------

func TestHandleWebSocketPod_AuthenticationRequired(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
		DemoMode:      false,
	})
	require.NoError(t, err)

	srv := newTestServerWithAuth(t, authSvc)

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})
}

func TestHandleWebSocketPod_InvalidPod(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
		DemoMode:      false,
	})
	require.NoError(t, err)

	srv := newWSTestServerWithAuth(t, authSvc)

	token, err := authSvc.GenerateToken(&auth.User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	t.Run("rejects request with invalid podId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/invalid-pod-id", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// Test Helpers (unique to this file)
// -----------------------------------------------------------------------------

func newWSTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newWSTestServerWithAuth(t *testing.T, authSvc *auth.Service) *Server {
	t.Helper()
	cfg := DefaultConfig()
	cfg.CORSOrigins = []string{"http://localhost:3000"}

	orch := orchestrator.New(nil, nil, orchestrator.Config{}, orchestrator.WithLogger(newWSTestLogger()))
	wsHub := websocket.NewHub(newWSTestLogger())

	srv, _ := New(cfg, orch, nil, wsHub, newWSTestLogger(),
		WithAuthService(authSvc),
	)
	return srv
}
