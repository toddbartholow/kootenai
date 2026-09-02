package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

func TestHandleLogin(t *testing.T) {
	t.Run("auth service not configured", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"email": "test@example.com", "password": "password123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "authentication not configured" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("missing password in production mode", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
			DemoMode:      false, // Production mode
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		body := `{"email": "test@example.com", "password": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "password is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("user not found", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		// Create empty mock user repository
		userRepo := mocks.NewFakeUserRepository()

		srv := newTestServerWithAuthAndUserRepo(t, authSvc, userRepo)

		body := `{"email": "nonexistent@example.com", "password": "Password123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "invalid credentials" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		// Create mock user repository with a test user
		userRepo := mocks.NewFakeUserRepository()
		testPassword := "Password123!"
		passwordHash, err := auth.HashPassword(testPassword)
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}
		userRepo.AddUser(&models.User{
			ID:           "user-123",
			Email:        "test@example.com",
			DisplayName:  "Test User",
			Role:         "student",
			PasswordHash: passwordHash,
			IsActive:     true,
		})

		srv := newTestServerWithAuthAndUserRepo(t, authSvc, userRepo)

		// Use wrong password
		body := `{"email": "test@example.com", "password": "WrongPassword!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "invalid credentials" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("success", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		// Create mock user repository with a test user
		userRepo := mocks.NewFakeUserRepository()
		testPassword := "Password123!"
		passwordHash, err := auth.HashPassword(testPassword)
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}
		userRepo.AddUser(&models.User{
			ID:           "user-123",
			Email:        "test@example.com",
			DisplayName:  "Test User",
			Role:         "student",
			PasswordHash: passwordHash,
			IsActive:     true,
		})

		srv := newTestServerWithAuthAndUserRepo(t, authSvc, userRepo)

		body := `{"email": "test@example.com", "password": "Password123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["token"].(string); !ok {
			t.Error("expected token in response")
		}

		user, ok := response["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user in response")
		}

		if user["email"] != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got '%v'", user["email"])
		}
	})

	// Verifies the full i18n pipeline: the locale middleware reads
	// Accept-Language, attaches a Localizer to the request context, and
	// handleLogin renders the "auth.errors.invalidCredentials" message ID
	// from the English catalog.
	t.Run("localized error with Accept-Language header", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithAuthAndUserRepo(t, authSvc, userRepo)

		body := `{"email": "nobody@example.com", "password": "Password123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Language", "en")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		require.Equal(t, http.StatusUnauthorized, rr.Code)

		var response map[string]string
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&response))
		require.Equal(t, "invalid credentials", response["error"])
	})

	t.Run("invalid request body", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		body := `{"password": "password123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}

		var response ValidationErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Error != "validation failed" {
			t.Errorf("unexpected error: %s", response.Error)
		}

		// Check that email validation error is present
		found := false
		for _, detail := range response.Details {
			if detail.Field == "email" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected email validation error in details")
		}
	})

	t.Run("database error looking up user", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		// Create mock user repository that returns an error
		userRepo := mocks.NewFakeUserRepository()
		userRepo.GetByEmailForAuthErr = errors.New("database connection error")

		srv := newTestServerWithAuthAndUserRepo(t, authSvc, userRepo)

		body := `{"email": "test@example.com", "password": "Password123!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "login failed" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})
}

func TestHandleRefreshToken(t *testing.T) {
	t.Run("auth service not configured", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"token": "some-token"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		// Generate a valid token first
		user := &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
			Name:  "Test User",
			Roles: []string{"student"},
		}
		token, err := authSvc.GenerateToken(user)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		srv := newTestServerWithAuth(t, authSvc)

		body, _ := json.Marshal(map[string]string{"token": token})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["token"].(string); !ok {
			t.Error("expected new token in response")
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		body := `{}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "token is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		body := `{"token": "invalid-token"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})
}

func TestHandleGetCurrentUser(t *testing.T) {
	t.Run("not authenticated", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("authenticated user", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)

		// Generate a valid token
		user := &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
			Name:  "Test User",
			Roles: []string{"student"},
		}
		token, err := authSvc.GenerateToken(user)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		srv := newTestServerWithAuth(t, authSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		userData, ok := response["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user in response")
		}

		if userData["email"] != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got '%v'", userData["email"])
		}
	})
}

// -----------------------------------------------------------------------------
// handleUpdatePreferredLocale Tests
// -----------------------------------------------------------------------------

func TestHandleUpdatePreferredLocale(t *testing.T) {
	newAuthenticatedServer := func(t *testing.T) (*Server, string, *mocks.FakeUserRepository) {
		t.Helper()
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
		})
		require.NoError(t, err)
		userRepo := mocks.NewFakeUserRepository()
		// Seed a user so UpdatePreferredLocale can find them.
		require.NoError(t, userRepo.Create(context.Background(), &models.User{
			ID:       "user-123",
			Username: "test",
		}))
		user := &auth.User{ID: "user-123", Email: "test@example.com", Name: "Test", Roles: []string{"student"}}
		token, err := authSvc.GenerateToken(user)
		require.NoError(t, err)
		srv := newTestServerWithAuthAndUserRepo(t, authSvc, userRepo)
		return srv, token, userRepo
	}

	t.Run("sets a supported locale", func(t *testing.T) {
		srv, token, userRepo := newAuthenticatedServer(t)
		body := strings.NewReader(`{"preferredLocale":"es"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/me/preferred-locale", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		require.Equal(t, http.StatusNoContent, rr.Code, "body=%s", rr.Body.String())
		got, err := userRepo.GetPreferredLocale(context.Background(), "user-123")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, "es", *got)
	})

	t.Run("clears preference on null", func(t *testing.T) {
		srv, token, userRepo := newAuthenticatedServer(t)
		// Seed an existing preference.
		require.NoError(t, userRepo.UpdatePreferredLocale(context.Background(), "user-123", strPtr("es")))

		body := strings.NewReader(`{"preferredLocale":null}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/me/preferred-locale", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		require.Equal(t, http.StatusNoContent, rr.Code)
		got, err := userRepo.GetPreferredLocale(context.Background(), "user-123")
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("rejects unsupported locale", func(t *testing.T) {
		srv, token, _ := newAuthenticatedServer(t)
		body := strings.NewReader(`{"preferredLocale":"xx"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/me/preferred-locale", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("rejects unauthenticated", func(t *testing.T) {
		srv, _, _ := newAuthenticatedServer(t)
		body := strings.NewReader(`{"preferredLocale":"es"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/me/preferred-locale", body)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// -----------------------------------------------------------------------------
// handleDemoLogin Tests
// -----------------------------------------------------------------------------

func TestHandleDemoLogin(t *testing.T) {
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

	t.Run("success - generates token for demo user", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
			DemoMode:      true,
		})
		require.NoError(t, err)

		authMgr := NewAuthManager(AuthManagerConfig{
			AuthService: authSvc,
			Logger:      newTestLogger(),
		})

		// Test direct handler invocation
		rr := httptest.NewRecorder()
		authMgr.handleDemoLogin(rr, "demo@example.com")

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["token"].(string); !ok {
			t.Error("expected token in response")
		}

		user, ok := response["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user in response")
		}

		if user["email"] != "demo@example.com" {
			t.Errorf("expected email 'demo@example.com', got '%v'", user["email"])
		}

		if user["name"] != "Demo User" {
			t.Errorf("expected name 'Demo User', got '%v'", user["name"])
		}

		// Check mustChangePassword is false for demo
		if response["mustChangePassword"] != false {
			t.Error("expected mustChangePassword to be false for demo login")
		}
	})

	t.Run("demo login via API in demo mode", func(t *testing.T) {
		authSvc, err := auth.NewService(auth.Config{
			JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
			JWTExpiration: time.Hour,
			DemoMode:      true,
		})
		require.NoError(t, err)

		srv := newTestServerWithAuth(t, authSvc)

		// In demo mode, any login should work
		body := `{"email": "demo@example.com", "password": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should succeed in demo mode
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d in demo mode, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

// newTestServerWithAuth creates a test server with an auth service
func newTestServerWithAuth(t *testing.T, authSvc *auth.Service) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithAuthService(authSvc))
	return srv
}

// newTestServerWithAuthAndUserRepo creates a test server with an auth service and user repository
func newTestServerWithAuthAndUserRepo(t *testing.T, authSvc *auth.Service, userRepo repositories.UserRepository) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithAuthService(authSvc), WithUserRepo(userRepo))
	return srv
}

// newTestServerWithCookieMode creates a test server with cookie auth enabled
func newTestServerWithCookieMode(t *testing.T, authSvc *auth.Service, userRepo repositories.UserRepository) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())
	cookieCfg := auth.CookieConfig{
		Enabled:  true,
		Name:     auth.DefaultCookieName,
		CSRFName: auth.DefaultCSRFName,
		MaxAge:   time.Hour,
	}
	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(),
		WithAuthService(authSvc),
		WithUserRepo(userRepo),
		WithCookieConfig(cookieCfg),
	)
	return srv
}

// ---------------------------------------------------------------------------
// Cookie-mode handler tests (ADR-0002 Phase B)
// ---------------------------------------------------------------------------

func TestLoginSetsCookie(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	userRepo := mocks.NewFakeUserRepository()
	pw, err := auth.HashPassword("Password123!")
	require.NoError(t, err)
	userRepo.AddUser(&models.User{
		ID: "user-1", Email: "test@example.com", DisplayName: "Test",
		Role: "student", PasswordHash: pw, IsActive: true,
	})

	srv := newTestServerWithCookieMode(t, authSvc, userRepo)

	body := `{"email":"test@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Verify Set-Cookie headers
	cookies := rr.Result().Cookies()
	var authCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case auth.DefaultCookieName:
			authCookie = c
		case auth.DefaultCSRFName:
			csrfCookie = c
		}
	}

	if authCookie == nil {
		t.Fatal("auth cookie not set on login")
	}
	if !authCookie.HttpOnly {
		t.Error("auth cookie must be HttpOnly")
	}
	if authCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", authCookie.SameSite)
	}

	if csrfCookie == nil {
		t.Fatal("CSRF cookie not set on login")
	}
	if csrfCookie.HttpOnly {
		t.Error("CSRF cookie must NOT be HttpOnly")
	}
	if len(csrfCookie.Value) != 64 {
		t.Errorf("CSRF token length = %d, want 64", len(csrfCookie.Value))
	}

	// JSON body must still contain the token (backwards compat)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	if _, ok := resp["token"]; !ok {
		t.Error("JSON body must still contain 'token' for backwards compat")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	userRepo := mocks.NewFakeUserRepository()
	srv := newTestServerWithCookieMode(t, authSvc, userRepo)

	token, err := authSvc.GenerateToken(&auth.User{
		ID: "user-1", Email: "test@example.com", Name: "Test", Roles: []string{"student"},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Verify cookies are cleared (MaxAge = -1 or 0)
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.DefaultCookieName || c.Name == auth.DefaultCSRFName {
			if c.MaxAge > 0 {
				t.Errorf("cookie %q should be cleared (MaxAge=%d)", c.Name, c.MaxAge)
			}
		}
	}
}

func TestLogoutFromCookie(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	userRepo := mocks.NewFakeUserRepository()
	srv := newTestServerWithCookieMode(t, authSvc, userRepo)

	token, err := authSvc.GenerateToken(&auth.User{
		ID: "user-1", Email: "test@example.com", Name: "Test", Roles: []string{"student"},
	})
	require.NoError(t, err)

	// Logout using cookie instead of Bearer header
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.DefaultCookieName, Value: token})
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestRefreshRotatesCookie(t *testing.T) {
	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	userRepo := mocks.NewFakeUserRepository()
	srv := newTestServerWithCookieMode(t, authSvc, userRepo)

	token, err := authSvc.GenerateToken(&auth.User{
		ID: "user-1", Email: "test@example.com", Name: "Test", Roles: []string{"student"},
	})
	require.NoError(t, err)

	// Refresh via cookie (empty body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.DefaultCookieName, Value: token})
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Verify new auth cookie was set
	var foundNewCookie bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.DefaultCookieName && c.Value != "" && c.MaxAge > 0 {
			foundNewCookie = true
			if c.Value == token {
				t.Error("refreshed cookie should have a new token value")
			}
		}
	}
	if !foundNewCookie {
		t.Error("refresh should set a new auth cookie")
	}
}
