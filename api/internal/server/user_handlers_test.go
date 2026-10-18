package server

import (
	"bytes"
	"encoding/json"
	"errors"
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

// -----------------------------------------------------------------------------
// handleListUsers Tests
// -----------------------------------------------------------------------------

func TestHandleListUsers(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("admin - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "user-1",
			Email:       "user1@example.com",
			DisplayName: "User One",
			Role:        "student",
			IsActive:    true,
		})
		userRepo.AddUser(&models.User{
			ID:          "user-2",
			Email:       "user2@example.com",
			DisplayName: "User Two",
			Role:        "instructor",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		users, ok := resp["users"].([]interface{})
		if !ok {
			t.Fatal("expected users array in response")
		}
		// Admin user is also in the list
		if len(users) < 2 {
			t.Errorf("expected at least 2 users, got %d", len(users))
		}
	})

	t.Run("admin with query params", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users?role=student&active=true&search=test&limit=10&offset=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleCreateUser Tests
// -----------------------------------------------------------------------------

func TestHandleCreateUser(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		body := `{"email": "new@example.com", "display_name": "New User", "role": "student"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		body := `{"email": "new@example.com", "display_name": "New User", "role": "student"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		body := `{"email": "new@example.com", "display_name": "New User", "role": "student"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid json - returns 400", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing username - returns 400", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"email": "test@example.com", "displayName": "New User", "role": "student"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("admin creates user - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"username": "newuser", "email": "newuser@example.com", "displayName": "New User", "role": "student"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetUser Tests
// -----------------------------------------------------------------------------

func TestHandleGetUser(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/other-user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("user not found - returns 404", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/nonexistent", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("admin gets user - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Target User",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/target-user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleUpdateUser Tests
// -----------------------------------------------------------------------------

func TestHandleUpdateUser(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		body := `{"display_name": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		body := `{"display_name": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/other-user", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		body := `{"display_name": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid json - returns 400", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "user-123",
			Email:       "user@example.com",
			DisplayName: "User",
			Role:        "student",
			IsActive:    true,
		})
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("user not found - returns 404", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"display_name": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/nonexistent", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("admin updates user - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Original Name",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"display_name": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error on update - returns 500", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Original Name",
			Role:        "student",
			IsActive:    true,
		})
		userRepo.UpdateErr = errors.New("database error")

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"displayName": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid role - returns 400", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Original Name",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"role": "invalid_role"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("email conflict - returns 409", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Original Name",
			Role:        "student",
			IsActive:    true,
		})
		userRepo.AddUser(&models.User{
			ID:          "other-user",
			Email:       "existing@example.com",
			DisplayName: "Other User",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"email": "existing@example.com"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleDeleteUser Tests
// -----------------------------------------------------------------------------

func TestHandleDeleteUser(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/other-user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-123", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("user not found - returns 404", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/nonexistent", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("admin deletes user - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Target User",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/target-user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleUpdateUserRole Tests
// -----------------------------------------------------------------------------

func TestHandleUpdateUserRole(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		body := `{"role": "instructor"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123/role", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		body := `{"role": "instructor"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/other-user/role", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		body := `{"role": "instructor"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123/role", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid role - returns 400", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Target User",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"role": "superadmin"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/role", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("admin updates role - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Target User",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"role": "instructor"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/role", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleUpdateUserStatus Tests
// -----------------------------------------------------------------------------

func TestHandleUpdateUserStatus(t *testing.T) {
	t.Run("no auth - returns 401", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv := newTestServerWithUserRepoAndAuth(t, userRepo)

		body := `{"is_active": false}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("non-admin - returns 403", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		srv, token := newTestServerWithUserRepoAndStudentToken(t, userRepo)

		body := `{"is_active": false}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/other-user/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("no user repo - returns 503", func(t *testing.T) {
		srv, token := newTestServerWithAdminTokenNoUserRepo(t)

		body := `{"is_active": false}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-123/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("admin updates status - success", func(t *testing.T) {
		userRepo := mocks.NewFakeUserRepository()
		userRepo.AddUser(&models.User{
			ID:          "target-user",
			Email:       "target@example.com",
			DisplayName: "Target User",
			Role:        "student",
			IsActive:    true,
		})

		srv, token := newTestServerWithUserRepoAndAdminToken(t, userRepo)

		body := `{"is_active": false}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/target-user/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func newTestServerWithUserRepoAndAuth(t *testing.T, userRepo *mocks.FakeUserRepository) *Server {
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
	return srv
}

func newTestServerWithUserRepoAndStudentToken(t *testing.T, userRepo *mocks.FakeUserRepository) (*Server, string) {
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

	// Generate token for student user
	token, _ := authSvc.GenerateToken(&auth.User{
		ID:    "student-user",
		Email: "student@example.com",
		Name:  "Student User",
		Roles: []string{"student"},
	})

	return srv, token
}

func newTestServerWithUserRepoAndAdminToken(t *testing.T, userRepo *mocks.FakeUserRepository) (*Server, string) {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	// Add admin user to repo
	if userRepo != nil {
		userRepo.AddUser(&models.User{
			ID:          "admin-user",
			Email:       "admin@example.com",
			DisplayName: "Admin User",
			Role:        "admin",
			IsActive:    true,
		})
	}

	opts := []ServerOption{WithAuthService(authSvc)}
	if userRepo != nil {
		opts = append(opts, WithUserRepo(userRepo))
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)

	// Generate token for admin user
	token, _ := authSvc.GenerateToken(&auth.User{
		ID:    "admin-user",
		Email: "admin@example.com",
		Name:  "Admin User",
		Roles: []string{"admin"},
	})

	return srv, token
}

func newTestServerWithAdminTokenNoUserRepo(t *testing.T) (*Server, string) {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithAuthService(authSvc))

	// Generate token for admin user
	token, _ := authSvc.GenerateToken(&auth.User{
		ID:    "admin-user",
		Email: "admin@example.com",
		Name:  "Admin User",
		Roles: []string{"admin"},
	})

	return srv, token
}
