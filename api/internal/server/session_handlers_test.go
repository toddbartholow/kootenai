package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

func TestHandleListSessions(t *testing.T) {
	t.Run("no session repo returns empty list", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		sessions, ok := response["sessions"].([]interface{})
		if !ok {
			t.Fatal("expected sessions array in response")
		}

		if len(sessions) != 0 {
			t.Errorf("expected empty sessions array, got %d items", len(sessions))
		}
	})

	t.Run("with session repo returns sessions", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-1",
		})
		sessionRepo.AddSession(&models.Session{
			ID:     "session-2",
			PodID:  "pod-2",
			UserID: "user-2",
		})

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response ListSessionsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Sessions) != 2 {
			t.Errorf("expected 2 sessions, got %d", len(response.Sessions))
		}
	})

	t.Run("filter by userId", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-1",
		})
		sessionRepo.AddSession(&models.Session{
			ID:     "session-2",
			PodID:  "pod-2",
			UserID: "user-2",
		})

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?userId=user-1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response ListSessionsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(response.Sessions))
		}
	})

	t.Run("filter active sessions", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		now := time.Now()
		sessionRepo.AddSession(&models.Session{
			ID:      "session-active",
			PodID:   "pod-1",
			UserID:  "user-1",
			EndedAt: nil, // Active
		})
		sessionRepo.AddSession(&models.Session{
			ID:      "session-ended",
			PodID:   "pod-2",
			UserID:  "user-1",
			EndedAt: &now, // Ended
		})

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?active=true", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response ListSessionsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Sessions) != 1 {
			t.Errorf("expected 1 session (only active), got %d", len(response.Sessions))
		}
	})

	t.Run("database error", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.ListErr = errMockDB

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("non-admin filters to own sessions", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-1",
		})
		sessionRepo.AddSession(&models.Session{
			ID:     "session-2",
			PodID:  "pod-2",
			UserID: "user-2",
		})

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
		// Add non-admin user to context
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-1",
			Email: "user1@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response ListSessionsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Non-admin should only see their own sessions
		if len(response.Sessions) != 1 {
			t.Errorf("expected 1 session (own only), got %d", len(response.Sessions))
		}
	})

	t.Run("non-admin with org can query org sessions", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-1",
		})

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?userId=user-2", nil)
		// Non-admin user with organization context
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:                    "user-1",
			Email:                 "user1@example.com",
			Roles:                 []string{"student"},
			DefaultOrganizationID: "org-123",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("admin with org filters by default", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-1",
		})

		srv := newTestServerWithSessionRepo(t, sessionRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
		// Admin with organization
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:                    "admin-1",
			Email:                 "admin@example.com",
			Roles:                 []string{"admin"},
			DefaultOrganizationID: "org-123",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestHandleCreateSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Register a template so the session can be started
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{
				Name: "basic-lab",
			},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{
						ID:          "cp-1",
						Description: "Test checkpoint description",
						Points:      10,
					},
				},
			},
		}
		eval.RegisterTemplate(template)

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		body := `{"podId": "pod-1", "userId": "user-1", "labTemplate": "basic-lab"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "started" {
			t.Errorf("expected status 'started', got '%v'", response["status"])
		}

		if _, ok := response["sessionId"].(string); !ok {
			t.Error("expected sessionId in response")
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing podId", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"userId": "user-1", "labTemplate": "basic-lab"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
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

		if response["error"] != "podId is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("missing userId", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"podId": "pod-1", "labTemplate": "basic-lab"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
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

		if response["error"] != "userId is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("missing labTemplate", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"podId": "pod-1", "userId": "user-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
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

		if response["error"] != "labTemplate is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("enrollmentId without moduleId", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"podId": "pod-1", "userId": "user-1", "labTemplate": "basic-lab", "enrollmentId": "enroll-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
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

		if response["error"] != "both enrollmentId and moduleId must be provided together" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("moduleId without enrollmentId", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"podId": "pod-1", "userId": "user-1", "labTemplate": "basic-lab", "moduleId": "mod-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
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

		if response["error"] != "both enrollmentId and moduleId must be provided together" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("evaluator start session error", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())
		// Don't register template - will cause StartSession to fail

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		body := `{"podId": "pod-1", "userId": "user-1", "labTemplate": "unknown-template"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("success with authenticated user sets org", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Register a template
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		body := `{"podId": "pod-1", "userId": "user-1", "labTemplate": "basic-lab"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		// Add auth context with organization
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:                    "user-1",
			Email:                 "user1@example.com",
			Roles:                 []string{"student"},
			DefaultOrganizationID: "org-123",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		// Verify session was created with org
		sessions, _ := sessionRepo.ListAll(context.Background())
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(sessions))
		}
		if sessions[0].OrganizationID == nil || *sessions[0].OrganizationID != "org-123" {
			t.Errorf("expected session organization to be 'org-123', got %v", sessions[0].OrganizationID)
		}
	})
}

func TestHandleGetSession(t *testing.T) {
	t.Run("session from database", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:           "session-1",
			PodID:        "pod-1",
			UserID:       "user-1",
			EarnedPoints: 50,
			MaxPoints:    100,
		})
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-1", Email: "user1@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["id"] != "session-1" {
			t.Errorf("expected id 'session-1', got '%v'", response["id"])
		}
	})

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied - other user session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-owner",
		})
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "other-user",
			Email: "other@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("admin can access any session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-owner",
		})
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "admin-user",
			Email: "admin@example.com",
			Roles: []string{"admin"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error - falls back to evaluator", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.GetByIDErr = errors.New("database error")

		eval := checkpoint.NewEvaluator(newTestLogger())
		// Register template and start session in evaluator
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-1", "pod-1", "user-1", "basic-lab")

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should fall back to evaluator and succeed
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleEndSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:        "session-1",
			PodID:     "pod-1",
			UserID:    "user-1",
			MaxPoints: 100,
		})
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Register template first
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		// Start session in evaluator so it can be ended
		eval.StartSession("session-1", "pod-1", "user-1", "basic-lab")

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/end", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-1", Email: "user1@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response SessionSubmitResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.SessionID != "session-1" {
			t.Errorf("expected sessionId 'session-1', got '%v'", response.SessionID)
		}
	})

	t.Run("access denied - other user session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-owner",
		})
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/end", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "other-user",
			Email: "other@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error on get session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.GetByIDErr = errors.New("database error")
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/end", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-1",
			Email: "user1@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleGetProgress(t *testing.T) {
	t.Run("session with progress", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Register template first
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		// Start session in evaluator
		eval.StartSession("session-1", "pod-1", "user-1", "basic-lab")

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/progress", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["sessionId"] != "session-1" {
			t.Errorf("expected sessionId 'session-1', got '%v'", response["sessionId"])
		}
	})

	t.Run("session not found", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, nil, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/progress", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied - other user session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			UserID: "user-owner",
			PodID:  "pod-1",
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/progress", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "other-user",
			Email: "other@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("admin can access any session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			UserID: "user-owner",
			PodID:  "pod-1",
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		// Register template and start session
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-1", "pod-1", "user-owner", "basic-lab")

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/progress", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "admin-user",
			Email: "admin@example.com",
			Roles: []string{"admin"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database fallback - session in database only", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:           "db-session-1",
			UserID:       "user-1",
			PodID:        "pod-1",
			MaxPoints:    100,
			EarnedPoints: 75,
			Percentage:   75.0,
			Passed:       true,
		})

		// Evaluator has no knowledge of this session (simulates restart scenario)
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/db-session-1/progress", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-1",
			Email: "user1@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["earnedPoints"] != float64(75) {
			t.Errorf("expected earnedPoints 75, got %v", response["earnedPoints"])
		}
	})

	t.Run("database fallback - database error", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.GetByIDErr = errors.New("database connection error")

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/db-session-1/progress", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleGetCheckpoints(t *testing.T) {
	t.Run("session with checkpoints", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Register template first
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-1", "pod-1", "user-1", "basic-lab")

		srv := newTestServerWithEvaluator(t, nil, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/checkpoints", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["checkpoints"]; !ok {
			t.Error("expected checkpoints in response")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, nil, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/checkpoints", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied - different user", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:            "session-other",
			PodID:         "pod-other",
			UserID:        "owner-user",
			LabTemplateID: "template-1",
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Test", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-other", "pod-other", "owner-user", "basic-lab")

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-other/checkpoints", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "different-user",
			Email: "different@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleSubmitSession(t *testing.T) {
	t.Run("success - submit session with passed grade", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:               "session-1",
			PodID:            "pod-1",
			UserID:           "user-1",
			LabTemplateID:    "template-1",
			MaxPoints:        100,
			PassingThreshold: 70,
			Metadata:         make(map[string]string),
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Task 1", Points: 50},
					{ID: "cp-2", Description: "Task 2", Points: 50},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-1", "pod-1", "user-1", "basic-lab")

		// Simulate completing some checkpoints (80 points = 80%)
		progress, _ := eval.GetSessionProgress("session-1")
		progress.EarnedPoints = 80

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/submit", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-1", Email: "user1@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "graded" {
			t.Errorf("expected status 'graded', got '%v'", response["status"])
		}

		if response["passed"] != true {
			t.Errorf("expected passed to be true, got %v", response["passed"])
		}

		earnedPoints, ok := response["earnedPoints"].(float64)
		if !ok || earnedPoints != 80 {
			t.Errorf("expected earnedPoints 80, got %v", response["earnedPoints"])
		}

		percentage, ok := response["percentage"].(float64)
		if !ok || percentage != 80.0 {
			t.Errorf("expected percentage 80.0, got %v", response["percentage"])
		}
	})

	t.Run("success - submit session with failed grade", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:               "session-2",
			PodID:            "pod-2",
			UserID:           "user-2",
			LabTemplateID:    "template-1",
			MaxPoints:        100,
			PassingThreshold: 70,
			Metadata:         make(map[string]string),
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "basic-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Description: "Task 1", Points: 50},
					{ID: "cp-2", Description: "Task 2", Points: 50},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-2", "pod-2", "user-2", "basic-lab")

		// Simulate completing fewer checkpoints (50 points = 50% < 70%)
		progress, _ := eval.GetSessionProgress("session-2")
		progress.EarnedPoints = 50

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-2/submit", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-2", Email: "user2@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["passed"] != false {
			t.Errorf("expected passed to be false, got %v", response["passed"])
		}
	})

	t.Run("error - no session repository", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, nil, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/submit", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("error - session not found", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())

		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/nonexistent/submit", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("error - session already submitted", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:            "session-submitted",
			PodID:         "pod-1",
			UserID:        "user-1",
			LabTemplateID: "template-1",
			MaxPoints:     100,
			Metadata: map[string]string{
				"status": "submitted",
			},
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-submitted/submit", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-1", Email: "user1@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "session already submitted" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("error - progress not found", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:            "session-no-progress",
			PodID:         "pod-1",
			UserID:        "user-1",
			LabTemplateID: "template-1",
			MaxPoints:     100,
			Metadata:      make(map[string]string),
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-no-progress/submit", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-1", Email: "user1@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("error - access denied", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:            "session-1",
			PodID:         "pod-1",
			UserID:        "user-owner",
			LabTemplateID: "template-1",
			MaxPoints:     100,
			Metadata:      make(map[string]string),
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/session-1/submit", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "other-user",
			Email: "other@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleDeleteSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-to-delete",
			PodID:  "pod-1",
			UserID: "user-1",
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/session-to-delete", nil)
		// Add auth context as session owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-1", Email: "user1@example.com"})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "deleted" {
			t.Errorf("expected status 'deleted', got '%s'", response["status"])
		}

		// Verify session was deleted
		session, _ := sessionRepo.GetByID(context.TODO(), "session-to-delete")
		if session != nil {
			t.Error("expected session to be deleted")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no session repository", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, nil, eval)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/any-session", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("database error on get", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.GetByIDErr = errMockDB

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/session-1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("access denied - other user session", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-1",
			PodID:  "pod-1",
			UserID: "user-owner",
		})

		eval := checkpoint.NewEvaluator(newTestLogger())
		srv := newTestServerWithEvaluator(t, sessionRepo, eval)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/session-1", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "other-user",
			Email: "other@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})
}

// Access control tests (canAccessSession, canModifySession) are now in the
// sessions/ sub-package: sessions/handlers_test.go.

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

var errMockDB = errors.New("mock database error")

// newTestServerWithSessionRepo creates a test server with a session repository
func newTestServerWithSessionRepo(t *testing.T, sessionRepo *mocks.FakeSessionRepository) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithSessionRepo(sessionRepo))
	return srv
}

// newTestServerWithEvaluator creates a test server with a session repository and evaluator
func newTestServerWithEvaluator(t *testing.T, sessionRepo *mocks.FakeSessionRepository, eval *checkpoint.Evaluator) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())

	var opts []ServerOption
	if sessionRepo != nil {
		opts = append(opts, WithSessionRepo(sessionRepo))
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)
	return srv
}
