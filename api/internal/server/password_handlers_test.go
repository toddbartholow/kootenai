package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

func TestHandleChangePassword(t *testing.T) {
	t.Run("not authenticated", func(t *testing.T) {
		srv := newTestServerWithUserRepo(t, nil)

		body := `{"currentPassword": "OldPass123", "newPassword": "NewPass123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/password/change", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("missing current password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		body := `{"newPassword": "NewPass123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/password/change", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response["error"] != "current password is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("missing new password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		body := `{"currentPassword": "OldPass123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/password/change", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response["error"] != "new password is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("wrong current password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		// Hash a known password
		hash, _ := auth.HashPassword("CorrectPass123")
		userRepo.AddUser(&models.User{
			ID:           "user-123",
			Email:        "test@example.com",
			PasswordHash: hash,
		})
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		body := `{"currentPassword": "WrongPass123", "newPassword": "NewPass123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/password/change", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response["error"] != "current password is incorrect" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("weak new password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		hash, _ := auth.HashPassword("CorrectPass123")
		userRepo.AddUser(&models.User{
			ID:           "user-123",
			Email:        "test@example.com",
			PasswordHash: hash,
		})
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		// New password lacks uppercase
		body := `{"currentPassword": "CorrectPass123", "newPassword": "weakpassword1"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/password/change", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		hash, _ := auth.HashPassword("CorrectPass123")
		userRepo.AddUser(&models.User{
			ID:           "user-123",
			Email:        "test@example.com",
			PasswordHash: hash,
		})
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		body := `{"currentPassword": "CorrectPass123", "newPassword": "NewSecurePass123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/password/change", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response["message"] != "password updated successfully" {
			t.Errorf("unexpected message: %s", response["message"])
		}
	})
}

func TestHandleAdminResetPassword(t *testing.T) {
	t.Run("not authenticated", func(t *testing.T) {
		srv := newTestServerWithUserRepo(t, nil)

		body := `{"generatePassword": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-456/password/reset", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		// Create server with non-admin user
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		body := `{"generatePassword": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password/reset", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithAuthAndAdmin(t, userRepo)

		body := `{"generatePassword": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/nonexistent/password/reset", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rr.Code, rr.Body.String())
		}
	})

	t.Run("neither password nor generatePassword provided", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		srv, token := newTestServerWithAuthAndAdmin(t, userRepo)

		body := `{}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password/reset", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response["error"] != "either password or generatePassword is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("success with generated password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		srv, token := newTestServerWithAuthAndAdmin(t, userRepo)

		body := `{"generatePassword": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password/reset", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
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

		if response["message"] != "password reset successfully" {
			t.Errorf("unexpected message: %v", response["message"])
		}
		if response["mustChangePassword"] != true {
			t.Errorf("expected mustChangePassword to be true")
		}
		if _, ok := response["temporaryPassword"].(string); !ok {
			t.Error("expected temporaryPassword in response")
		}
	})

	t.Run("success with specific password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		srv, token := newTestServerWithAuthAndAdmin(t, userRepo)

		body := `{"password": "NewPassword123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password/reset", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
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

		if response["message"] != "password reset successfully" {
			t.Errorf("unexpected message: %v", response["message"])
		}
		// Should not return password when manually set
		if _, ok := response["temporaryPassword"]; ok {
			t.Error("should not return temporaryPassword when password is manually set")
		}
	})
}

func TestHandleAdminSetPassword(t *testing.T) {
	t.Run("not authenticated", func(t *testing.T) {
		srv := newTestServerWithUserRepo(t, nil)

		body := `{"password": "NewPassword123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-456/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		srv, token := newTestServerWithAuthAndUser(t, userRepo)

		body := `{"password": "NewPassword123"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("missing password", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		srv, token := newTestServerWithAuthAndAdmin(t, userRepo)

		body := `{}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:    "target-user",
			Email: "target@example.com",
		})
		srv, token := newTestServerWithAuthAndAdmin(t, userRepo)

		body := `{"password": "NewPassword123", "mustChangePassword": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
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

		if response["message"] != "password set successfully" {
			t.Errorf("unexpected message: %v", response["message"])
		}
		if response["mustChangePassword"] != true {
			t.Errorf("expected mustChangePassword to be true")
		}
	})
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

// newTestServerWithUserRepo creates a test server with a user repository
func newTestServerWithUserRepo(t *testing.T, userRepo *mocks.FakeUserRepository) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	opts := []ServerOption{}
	if userRepo != nil {
		opts = append(opts, WithUserRepo(userRepo))
	}

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)
	opts = append(opts, WithAuthService(authSvc))

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)
	return srv
}

// newTestServerWithAuthAndUser creates a test server with auth and a regular user token
func newTestServerWithAuthAndUser(t *testing.T, userRepo *mocks.FakeUserRepository) (*Server, string) {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	opts := []ServerOption{WithAuthService(authSvc)}
	if userRepo != nil {
		opts = append(opts, WithUserRepo(userRepo))
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)

	// Generate token for a regular user
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

	return srv, token
}

// newTestServerWithAuthAndAdmin creates a test server with auth and an admin user token
func newTestServerWithAuthAndAdmin(t *testing.T, userRepo *mocks.FakeUserRepository) (*Server, string) {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	opts := []ServerOption{WithAuthService(authSvc)}
	if userRepo != nil {
		opts = append(opts, WithUserRepo(userRepo))
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)

	// Generate token for an admin user
	user := &auth.User{
		ID:    "admin-123",
		Email: "admin@example.com",
		Name:  "Admin User",
		Roles: []string{"admin"},
	}
	token, err := authSvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	return srv, token
}
