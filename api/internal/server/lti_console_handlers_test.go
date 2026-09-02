package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

func ltiConsoleTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// -----------------------------------------------------------------------------
// handleLTIConsole Tests
// -----------------------------------------------------------------------------

func TestHandleLTIConsole_MissingParameters(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiConsoleTestLogger(),
	})

	handler := manager.handleLTIConsole()

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing both podId and vmid",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "lti.console.errors.missingPodOrVmid",
		},
		{
			name:           "empty query string",
			queryParams:    "?",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "lti.console.errors.missingPodOrVmid",
		},
		{
			name:           "only node parameter",
			queryParams:    "?node=pve",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "lti.console.errors.missingPodOrVmid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/lti/console"+tt.queryParams, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			body := rec.Body.String()
			if tt.expectedBody != "" && body != tt.expectedBody+"\n" {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

func TestHandleLTIConsole_PodRepoNotConfigured(t *testing.T) {
	// Manager without podRepo configured
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger:        ltiConsoleTestLogger(),
		ConsoleSecret: "test-secret",
		// No PodRepo configured
	})

	handler := manager.handleLTIConsole()

	token := manager.generateConsoleToken("test-pod-123")
	req := httptest.NewRequest(http.MethodGet, "/lti/console?podId=test-pod-123&token="+token, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	// Tests bypass the locale middleware, so Localize returns the raw message ID.
	expectedBody := "lti.console.errors.podRepoNotConfigured\n"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestHandleLTIConsole_PodNotFound(t *testing.T) {
	mockPodRepo := mocks.NewFakePodRepository()
	// Empty pod repo - no pods

	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger:        ltiConsoleTestLogger(),
		PodRepo:       mockPodRepo,
		ConsoleSecret: "test-secret",
	})

	handler := manager.handleLTIConsole()

	token := manager.generateConsoleToken("non-existent-pod")
	req := httptest.NewRequest(http.MethodGet, "/lti/console?podId=non-existent-pod&token="+token, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	expectedBody := "lti.console.errors.podNotFoundOrEmpty\n"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestHandleLTIConsole_PodWithNoVMs(t *testing.T) {
	mockPodRepo := mocks.NewFakePodRepository()
	// Add a pod with no VMs
	mockPodRepo.AddPod(&models.Pod{
		ID:     "pod-no-vms",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{}, // No VMs
	})

	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger:        ltiConsoleTestLogger(),
		PodRepo:       mockPodRepo,
		ConsoleSecret: "test-secret",
	})

	handler := manager.handleLTIConsole()

	token := manager.generateConsoleToken("pod-no-vms")
	req := httptest.NewRequest(http.MethodGet, "/lti/console?podId=pod-no-vms&token="+token, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	expectedBody := "lti.console.errors.podNotFoundOrEmpty\n"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestHandleLTIConsole_PodRepoError(t *testing.T) {
	mockPodRepo := mocks.NewFakePodRepository()
	mockPodRepo.GetByIDErr = &testError{msg: "database connection lost"}

	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger:        ltiConsoleTestLogger(),
		PodRepo:       mockPodRepo,
		ConsoleSecret: "test-secret",
	})

	handler := manager.handleLTIConsole()

	token := manager.generateConsoleToken("some-pod")
	req := httptest.NewRequest(http.MethodGet, "/lti/console?podId=some-pod&token="+token, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// -----------------------------------------------------------------------------
// WebSocket Protocol Detection Tests
// -----------------------------------------------------------------------------

func TestDetermineWebSocketProtocol(t *testing.T) {
	tests := []struct {
		name             string
		tlsEnabled       bool
		forwardedProto   string
		origin           string
		referer          string
		expectedProtocol string
	}{
		{
			name:             "plain HTTP request",
			expectedProtocol: "ws",
		},
		{
			name:             "TLS enabled directly",
			tlsEnabled:       true,
			expectedProtocol: "wss",
		},
		{
			name:             "X-Forwarded-Proto https",
			forwardedProto:   "https",
			expectedProtocol: "wss",
		},
		{
			name:             "X-Forwarded-Proto http",
			forwardedProto:   "http",
			expectedProtocol: "ws",
		},
		{
			name:             "Origin header https",
			origin:           "https://example.com",
			expectedProtocol: "wss",
		},
		{
			name:             "Origin header http",
			origin:           "http://example.com",
			expectedProtocol: "ws",
		},
		{
			name:             "Referer header https",
			referer:          "https://example.com/page",
			expectedProtocol: "wss",
		},
		{
			name:             "Referer header http",
			referer:          "http://example.com/page",
			expectedProtocol: "ws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			protocol := determineWSProtocol(req, tt.tlsEnabled)
			if protocol != tt.expectedProtocol {
				t.Errorf("expected protocol %q, got %q", tt.expectedProtocol, protocol)
			}
		})
	}
}

// determineWSProtocol extracts the WebSocket protocol determination logic for testing
// This mirrors the logic in handleLTIConsole and renderSingleVMConsole
func determineWSProtocol(r *http.Request, tlsEnabled bool) string {
	wsProtocol := "ws"
	if tlsEnabled {
		wsProtocol = "wss"
	} else if r.Header.Get("X-Forwarded-Proto") == "https" {
		wsProtocol = "wss"
	} else if len(r.Header.Get("Origin")) >= 8 && r.Header.Get("Origin")[:8] == "https://" {
		wsProtocol = "wss"
	} else if len(r.Header.Get("Referer")) >= 8 && r.Header.Get("Referer")[:8] == "https://" {
		wsProtocol = "wss"
	}
	return wsProtocol
}

// -----------------------------------------------------------------------------
// renderVNCConsolePage Tests
// -----------------------------------------------------------------------------

func TestRenderVNCConsolePage_BasicRender(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiConsoleTestLogger(),
		Config: canvas.Config{
			ToolIssuer: "http://<INFRA_VM_IP>:8080",
		},
	})

	pod := &models.Pod{
		ID: "test-pod-123",
		VMs: []models.PodVM{
			{Name: "VM1", PlatformID: "100", Node: "pve", Status: "running"},
			{Name: "VM2", PlatformID: "101", Node: "pve", Status: "running"},
		},
	}

	launch := &canvas.LTILaunchRequest{
		Context: canvas.LTIContext{
			Title: "Test Course",
			Label: "TEST101",
		},
		CanvasUser: &canvas.CanvasUser{
			Name: "Test User",
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lti/launch", nil)

	manager.renderVNCConsolePage(rec, req, pod, launch)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("expected Content-Type 'text/html', got %q", contentType)
	}

	body := rec.Body.String()

	// Check that the page contains expected content
	expectedContent := []string{
		"Lab Environment Ready",
		"Test Course",
		"Test User",
		"2 virtual machine(s)",
		"podId=test-pod-123",
	}

	for _, expected := range expectedContent {
		if !ltiConsoleContainsString(body, expected) {
			t.Errorf("expected body to contain %q", expected)
		}
	}
}

func TestRenderVNCConsolePage_FallbackCourseName(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiConsoleTestLogger(),
		Config: canvas.Config{
			ToolIssuer: "http://<INFRA_VM_IP>:8080",
		},
	})

	pod := &models.Pod{
		ID:  "test-pod-123",
		VMs: []models.PodVM{{Name: "VM1", PlatformID: "100"}},
	}

	// Launch with no Title but has Label
	launch := &canvas.LTILaunchRequest{
		Context: canvas.LTIContext{
			Title: "", // Empty title
			Label: "FALLBACK101",
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lti/launch", nil)

	manager.renderVNCConsolePage(rec, req, pod, launch)

	body := rec.Body.String()
	if !ltiConsoleContainsString(body, "FALLBACK101") {
		t.Error("expected body to contain fallback course label 'FALLBACK101'")
	}
}

func TestRenderVNCConsolePage_NoUser(t *testing.T) {
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiConsoleTestLogger(),
		Config: canvas.Config{
			ToolIssuer: "http://<INFRA_VM_IP>:8080",
		},
	})

	pod := &models.Pod{
		ID:  "test-pod-123",
		VMs: []models.PodVM{{Name: "VM1", PlatformID: "100"}},
	}

	// Launch with no user
	launch := &canvas.LTILaunchRequest{
		Context: canvas.LTIContext{
			Title: "Test Course",
		},
		CanvasUser: nil, // No user
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lti/launch", nil)

	// Should not panic
	manager.renderVNCConsolePage(rec, req, pod, launch)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRenderVNCConsolePage_MissingToolIssuer(t *testing.T) {
	// Manager with no ToolIssuer configured — should return an error
	manager, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: ltiConsoleTestLogger(),
		Config: canvas.Config{
			ToolIssuer: "", // Empty - should fail
		},
	})

	pod := &models.Pod{
		ID:  "test-pod-123",
		VMs: []models.PodVM{{Name: "VM1", PlatformID: "100"}},
	}

	launch := &canvas.LTILaunchRequest{
		Context: canvas.LTIContext{Title: "Test"},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lti/launch", nil)

	manager.renderVNCConsolePage(rec, req, pod, launch)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 when ToolIssuer is empty, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// ltiConsoleContainsString checks if haystack contains needle
// Named uniquely to avoid conflicts with other test helpers in this package
func ltiConsoleContainsString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
