package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/canvas"
)

func ltiDeeplinkTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// -----------------------------------------------------------------------------
// handleListLTITemplates Tests
// -----------------------------------------------------------------------------

func TestHandleListLTITemplates_LTINotConfigured_WithAuthContext(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
		// No LTI assignment repo configured
	})

	handler := manager.handleListLTITemplates()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lti/templates", nil)
	// Set auth context with org ID to verify org scoping path doesn't panic
	ctx := auth.ContextWithUser(req.Context(), &auth.User{
		ID:                    "user-123",
		DefaultOrganizationID: "org-456",
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Still returns service unavailable since no LTI repo is configured
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleListLTITemplates_LTINotConfigured(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
		// No LTI assignment repo configured
	})

	handler := manager.handleListLTITemplates()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lti/templates", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	// Check error response body
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "lti.errors.notConfigured" {
		t.Errorf("expected error 'lti.errors.notConfigured', got %q", response["error"])
	}
}

// -----------------------------------------------------------------------------
// handleDeepLinkSubmit Tests
// -----------------------------------------------------------------------------

func TestHandleDeepLinkSubmit_InvalidRequestBody(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	handler := manager.handleDeepLinkSubmit()

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lti/deep-link/submit", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "error.invalidRequestBody" {
		t.Errorf("expected error 'error.invalidRequestBody', got %q", response["error"])
	}
}

func TestHandleDeepLinkSubmit_MissingRequiredFields(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	handler := manager.handleDeepLinkSubmit()

	tests := []struct {
		name          string
		request       DeepLinkTemplateRequest
		expectedError string
	}{
		{
			name:          "missing templateId",
			request:       DeepLinkTemplateRequest{ReturnURL: "http://example.com", CourseID: "123"},
			expectedError: "ltiDeeplink.errors.templateIdRequired",
		},
		{
			name:          "missing returnUrl",
			request:       DeepLinkTemplateRequest{TemplateID: "tmpl-1", CourseID: "123"},
			expectedError: "ltiDeeplink.errors.returnUrlRequired",
		},
		{
			name:          "missing courseId",
			request:       DeepLinkTemplateRequest{TemplateID: "tmpl-1", ReturnURL: "http://example.com"},
			expectedError: "ltiDeeplink.errors.courseIdRequired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/lti/deep-link/submit", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}

			var response map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if response["error"] != tt.expectedError {
				t.Errorf("expected error %q, got %q", tt.expectedError, response["error"])
			}
		})
	}
}

func TestHandleDeepLinkSubmit_DeepLinkingNotConfigured(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
		// No deep linking service configured
	})

	handler := manager.handleDeepLinkSubmit()

	request := DeepLinkTemplateRequest{
		TemplateID: "tmpl-1",
		ReturnURL:  "http://example.com/return",
		CourseID:   "course-123",
	}
	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lti/deep-link/submit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "ltiDeeplink.errors.notConfigured" {
		t.Errorf("expected error 'ltiDeeplink.errors.notConfigured', got %q", response["error"])
	}
}

// -----------------------------------------------------------------------------
// handleLTISelectPage Tests
// -----------------------------------------------------------------------------

func TestHandleLTISelectPage_BasicRender(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	handler := manager.handleLTISelectPage()

	req := httptest.NewRequest(http.MethodGet, "/lti/select?return_url=http://canvas.test/return&deployment_id=deploy-1&course_id=course-123&course_name=Test+Course&issuer=http://canvas.test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type 'text/html; charset=utf-8', got %q", contentType)
	}

	body := rec.Body.String()

	// Check that the page contains expected content
	expectedContent := []string{
		"Select Lab Assignment",
		"Course: Test Course",
		"http://canvas.test/return",
		"deploy-1",
		"course-123",
		"http://canvas.test",
	}

	for _, expected := range expectedContent {
		if !ltiDeeplinkContainsString(body, expected) {
			t.Errorf("expected body to contain %q", expected)
		}
	}
}

func TestHandleLTISelectPage_EmptyParameters(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	handler := manager.handleLTISelectPage()

	// Request with no query parameters
	req := httptest.NewRequest(http.MethodGet, "/lti/select", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should still render the page (with empty values)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// -----------------------------------------------------------------------------
// getOrCreateUserFromLaunch Tests
// -----------------------------------------------------------------------------

func TestGetOrCreateUserFromLaunch_NilCanvasUser(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	launch := &canvas.LTILaunchRequest{
		CanvasUser: nil,
	}

	user := manager.getOrCreateUserFromLaunch(context.Background(), launch)

	if user != nil {
		t.Error("expected nil user when CanvasUser is nil")
	}
}

func TestGetOrCreateUserFromLaunch_StudentRole(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	launch := &canvas.LTILaunchRequest{
		Subject: "canvas-user-123",
		CanvasUser: &canvas.CanvasUser{
			ID:      "123",
			Name:    "Test Student",
			Email:   "student@test.edu",
			LoginID: "student@test.edu",
		},
		Roles: []string{
			"http://purl.imsglobal.org/vocab/lis/v2/institution/person#Student",
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner",
		},
	}

	user := manager.getOrCreateUserFromLaunch(context.Background(), launch)

	if user == nil {
		t.Fatal("expected non-nil user")
	}

	if user.ID != "canvas-user-123" {
		t.Errorf("expected ID 'canvas-user-123', got %q", user.ID)
	}

	if user.Email != "student@test.edu" {
		t.Errorf("expected email 'student@test.edu', got %q", user.Email)
	}

	if user.Name != "Test Student" {
		t.Errorf("expected name 'Test Student', got %q", user.Name)
	}

	// Should have student role (not instructor)
	if !slices.Contains(user.Roles, "student") {
		t.Errorf("expected user to have 'student' role, got %v", user.Roles)
	}
}

func TestGetOrCreateUserFromLaunch_InstructorRole(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	launch := &canvas.LTILaunchRequest{
		Subject: "canvas-instructor-456",
		CanvasUser: &canvas.CanvasUser{
			ID:      "456",
			Name:    "Test Instructor",
			Email:   "instructor@test.edu",
			LoginID: "instructor@test.edu",
		},
		Roles: []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor",
			"http://purl.imsglobal.org/vocab/lis/v2/institution/person#Instructor",
		},
	}

	user := manager.getOrCreateUserFromLaunch(context.Background(), launch)

	if user == nil {
		t.Fatal("expected non-nil user")
	}

	// Should have instructor role
	if !slices.Contains(user.Roles, "instructor") {
		t.Errorf("expected user to have 'instructor' role, got %v", user.Roles)
	}
}

func TestGetOrCreateUserFromLaunch_FallbackToLoginID(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	launch := &canvas.LTILaunchRequest{
		Subject: "canvas-user-789",
		CanvasUser: &canvas.CanvasUser{
			ID:      "789",
			Name:    "Test User",
			Email:   "", // Empty email
			LoginID: "testuser",
		},
		Roles: []string{},
	}

	user := manager.getOrCreateUserFromLaunch(context.Background(), launch)

	if user == nil {
		t.Fatal("expected non-nil user")
	}

	// Should use LoginID as email when email is empty
	if user.Email != "testuser" {
		t.Errorf("expected email to fall back to 'testuser', got %q", user.Email)
	}
}

// -----------------------------------------------------------------------------
// handleDeepLinkingRequest Tests
// -----------------------------------------------------------------------------

func TestHandleDeepLinkingRequest_NotInstructor(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiDeeplinkTestLogger(),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lti/launch", nil)

	// LTI launch with student role (not instructor)
	launch := &canvas.LTILaunchRequest{
		Subject: "canvas-user-123",
		Context: canvas.LTIContext{
			ID:    "course-123",
			Title: "Test Course",
		},
		Roles: []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner",
		},
	}

	// Create a simple JWT for testing
	idToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwczovL3B1cmwuaW1zZ2xvYmFsLm9yZy9zcGVjL2x0aS1kbC9jbGFpbS9kZWVwX2xpbmtpbmdfc2V0dGluZ3MiOnsiZGVlcF9saW5rX3JldHVybl91cmwiOiJodHRwOi8vZXhhbXBsZS5jb20vcmV0dXJuIiwiYWNjZXB0X3R5cGVzIjpbImx0aVJlc291cmNlTGluayJdfX0.sig"

	manager.handleDeepLinkingRequest(rec, req, launch, idToken)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "ltiDeeplink.errors.instructorRequired" {
		t.Errorf("expected error 'ltiDeeplink.errors.instructorRequired', got %q", response["error"])
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// ltiDeeplinkContainsString checks if haystack contains needle
func ltiDeeplinkContainsString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
