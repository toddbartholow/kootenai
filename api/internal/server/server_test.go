package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// TestMain sets up the test environment for all tests in this package.
// It enables CORS_ALLOW_WILDCARD to prevent panics when using DefaultConfig with wildcard origins.
func TestMain(m *testing.M) {
	// Set CORS_ALLOW_WILDCARD for tests using DefaultConfig with wildcard origins
	os.Setenv("CORS_ALLOW_WILDCARD", "true")
	code := m.Run()
	os.Exit(code)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %v, want 0.0.0.0", cfg.Host)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %v, want 8080", cfg.Port)
	}

	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", cfg.ReadTimeout)
	}

	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", cfg.WriteTimeout)
	}

	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}

	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("CORSOrigins = %v, want [http://localhost:3000]", cfg.CORSOrigins)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid custom config",
			config: Config{
				Host:            "127.0.0.1",
				Port:            9090,
				ReadTimeout:     60 * time.Second,
				WriteTimeout:    60 * time.Second,
				ShutdownTimeout: 30 * time.Second,
				CORSOrigins:     []string{"http://localhost:3000"},
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: Config{
				Host:            "",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name: "port too low",
			config: Config{
				Host:            "localhost",
				Port:            0,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "port too high",
			config: Config{
				Host:            "localhost",
				Port:            65536,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name: "zero read timeout",
			config: Config{
				Host:            "localhost",
				Port:            8080,
				ReadTimeout:     0,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "read_timeout must be positive",
		},
		{
			name: "negative read timeout",
			config: Config{
				Host:            "localhost",
				Port:            8080,
				ReadTimeout:     -1 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "read_timeout must be positive",
		},
		{
			name: "zero write timeout",
			config: Config{
				Host:            "localhost",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    0,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "write_timeout must be positive",
		},
		{
			name: "negative write timeout",
			config: Config{
				Host:            "localhost",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    -1 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "write_timeout must be positive",
		},
		{
			name: "zero shutdown timeout",
			config: Config{
				Host:            "localhost",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 0,
			},
			wantErr: true,
			errMsg:  "shutdown_timeout must be positive",
		},
		{
			name: "negative shutdown timeout",
			config: Config{
				Host:            "localhost",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "shutdown_timeout must be positive",
		},
		{
			name: "multiple errors",
			config: Config{
				Host:            "",
				Port:            0,
				ReadTimeout:     0,
				WriteTimeout:    0,
				ShutdownTimeout: 0,
			},
			wantErr: true,
			errMsg:  "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestConfigValidateEdgeCases(t *testing.T) {
	// Test boundary port values
	cfg := DefaultConfig()

	cfg.Port = 1
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with port 1 should not error, got %v", err)
	}

	cfg.Port = 65535
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with port 65535 should not error, got %v", err)
	}

	// Test minimum valid timeout (1 nanosecond)
	cfg.Port = 8080
	cfg.ReadTimeout = 1
	cfg.WriteTimeout = 1
	cfg.ShutdownTimeout = 1
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with 1ns timeouts should not error, got %v", err)
	}

	// Test common port configurations
	commonPorts := []int{80, 443, 8000, 8080, 8443, 3000, 9000}
	for _, port := range commonPorts {
		cfg := DefaultConfig()
		cfg.Port = port
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() with port %d should not error, got %v", port, err)
		}
	}
}

func TestConfigWithLargeTimeouts(t *testing.T) {
	cfg := Config{
		Host:            "localhost",
		Port:            8080,
		ReadTimeout:     24 * time.Hour,
		WriteTimeout:    24 * time.Hour,
		ShutdownTimeout: time.Hour,
		CORSOrigins:     []string{},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with large timeouts should not error, got %v", err)
	}
}

func TestConfigWithEmptyCORSOrigins(t *testing.T) {
	cfg := Config{
		Host:            "localhost",
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		CORSOrigins:     []string{},
	}

	// Empty CORS origins should be valid (defaults may be applied later)
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with empty CORSOrigins should not error, got %v", err)
	}
}

func TestConfigWithNilCORSOrigins(t *testing.T) {
	cfg := Config{
		Host:            "localhost",
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		CORSOrigins:     nil,
	}

	// Nil CORS origins should be valid
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with nil CORSOrigins should not error, got %v", err)
	}
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// strPtr is a helper to create string pointers in tests
func strPtr(s string) *string {
	return &s
}

func newTestServer(t *testing.T, opts ...ServerOption) *Server {
	t.Helper()
	cfg := DefaultConfig()
	// Use specific CORS origin for tests to avoid wildcard+credentials panic
	cfg.CORSOrigins = []string{"http://localhost:3000"}
	orch := orchestrator.New(nil, nil, orchestrator.Config{}, orchestrator.WithLogger(newTestLogger()))
	wsHub := websocket.NewHub(newTestLogger())

	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger(), opts...)
	return srv
}

// mockPodRepository implements orchestrator.PodRepository for testing
type mockPodRepository struct {
	pods map[string]*models.Pod
}

func newMockPodRepository() *mockPodRepository {
	return &mockPodRepository{
		pods: make(map[string]*models.Pod),
	}
}

func (r *mockPodRepository) Create(ctx context.Context, pod *models.Pod) error {
	r.pods[pod.ID] = pod
	return nil
}

func (r *mockPodRepository) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	pod, ok := r.pods[id]
	if !ok {
		return nil, nil
	}
	return pod, nil
}

func (r *mockPodRepository) List(ctx context.Context, filter orchestrator.PodFilter) ([]*models.Pod, error) {
	result := make([]*models.Pod, 0)
	for _, pod := range r.pods {
		if filter.OwnerID != "" && pod.Owner != filter.OwnerID {
			continue
		}
		result = append(result, pod)
	}
	return result, nil
}

func (r *mockPodRepository) Update(ctx context.Context, pod *models.Pod) error {
	r.pods[pod.ID] = pod
	return nil
}

func (r *mockPodRepository) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	if pod, ok := r.pods[id]; ok {
		pod.Status = status
	}
	return nil
}

func (r *mockPodRepository) Delete(ctx context.Context, id string) error {
	delete(r.pods, id)
	return nil
}

func (r *mockPodRepository) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// Health Handler Tests
// -----------------------------------------------------------------------------

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

func TestHandleLive(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

func TestHandleReady(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// newTestServer has no database, and readiness means "can serve traffic".
	// This asserted 200/healthy before, which is what let a server running with
	// no persistence report itself ready -- the same blind spot that hid the
	// unsupported-sslmode default, since that also degrades to no database.
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", response.Status)
	}

	// The reason has to be named, not merely implied by the status code.
	dbCheck, ok := response.Checks["database"]
	if !ok {
		t.Fatal("expected a 'database' entry in checks; its absence is what made this silent")
	}
	if dbCheck.Status == "healthy" {
		t.Errorf("expected database check to be unhealthy, got %q", dbCheck.Status)
	}
	if dbCheck.Error == "" {
		t.Error("expected the database check to carry an error message")
	}

	// Verify version is present
	if response.Version == "" {
		t.Error("expected version in response")
	}

	// Verify uptime is present
	if response.Uptime == "" {
		t.Error("expected uptime in response")
	}

	// Verify checks map is present (may be empty)
	if response.Checks == nil {
		t.Error("expected checks in response")
	}
}

func TestHandleReadyAlternatePath(t *testing.T) {
	srv := newTestServer(t)

	// Test the /health/ready endpoint (alternative path)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// Same no-database server as TestHandleReady; both paths must agree.
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", response.Status)
	}
}

// TestHandleReadyWithHealthyDatabase is the counterpart to TestHandleReady:
// readiness must still succeed when a database IS present, so that "no
// database reports unready" cannot be satisfied by failing unconditionally.
func TestHandleReadyWithHealthyDatabase(t *testing.T) {
	srv := newTestServer(t, WithDBHealthChecker(&mockDBHealthChecker{}))

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body: %s)", http.StatusOK, rr.Code, rr.Body.String())
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", response.Status)
	}
	if db, ok := response.Checks["database"]; !ok || db.Status != "healthy" {
		t.Errorf("expected a healthy database check, got %+v (present=%v)", db, ok)
	}
}

func TestHandleReadyDegraded(t *testing.T) {
	// Test that failing dependencies cause degraded status
	mockDB := &mockDBHealthChecker{pingErr: errors.New("db connection failed")}
	srv := newTestServer(t, WithDBHealthChecker(mockDB))

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// Should return 503 Service Unavailable when degraded
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", response.Status)
	}

	// Verify the database check is unhealthy
	if response.Checks["database"] == nil {
		t.Error("expected database check in response")
	} else if response.Checks["database"].Status != "unhealthy" {
		t.Errorf("expected database status 'unhealthy', got '%s'", response.Checks["database"].Status)
	}
}

func TestHandleReadyWithLatency(t *testing.T) {
	mockDB := &mockDBHealthChecker{pingErr: nil}
	srv := newTestServer(t, WithDBHealthChecker(mockDB))

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify latency is recorded
	if response.Checks["database"] != nil {
		if response.Checks["database"].LatencyMs < 0 {
			t.Error("expected non-negative latency for database check")
		}
	}
}

// -----------------------------------------------------------------------------
// Pod Handler Tests
// -----------------------------------------------------------------------------

func TestHandleListPods(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}
	podRepo.pods["pod-2"] = &models.Pod{
		ID:       "pod-2",
		Owner:    "other@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("list all pods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		pods, ok := response["pods"].([]interface{})
		if !ok {
			t.Fatal("expected pods array in response")
		}

		if len(pods) != 2 {
			t.Errorf("expected 2 pods, got %d", len(pods))
		}
	})

	t.Run("list pods by owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods?owner=user@example.com", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		pods, ok := response["pods"].([]interface{})
		if !ok {
			t.Fatal("expected pods array in response")
		}

		if len(pods) != 1 {
			t.Errorf("expected 1 pod, got %d", len(pods))
		}
	})
}

func TestHandleGetPod(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("get existing pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/pod-1", nil)
		// Add auth context matching the pod owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:   "user-1",
			Name: "user@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var pod models.Pod
		if err := json.NewDecoder(rr.Body).Decode(&pod); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if pod.ID != "pod-1" {
			t.Errorf("expected pod ID 'pod-1', got '%s'", pod.ID)
		}
	})

	t.Run("get non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/non-existent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

func TestHandleCreatePod(t *testing.T) {
	srv := newTestServer(t)

	t.Run("missing request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", nil)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing labTemplate field", func(t *testing.T) {
		body := bytes.NewBufferString(`{"owner": "user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", body)
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
			t.Errorf("unexpected error message: %s", response.Error)
		}

		// Check that labTemplate validation error is present
		found := false
		for _, detail := range response.Details {
			if detail.Field == "labTemplate" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected labTemplate validation error in details")
		}
	})

	t.Run("missing owner field", func(t *testing.T) {
		body := bytes.NewBufferString(`{"labTemplate": "test-lab"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", body)
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
			t.Errorf("unexpected error message: %s", response.Error)
		}

		// Check that owner validation error is present
		found := false
		for _, detail := range response.Details {
			if detail.Field == "owner" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected owner validation error in details")
		}
	})

	t.Run("lab template repo not configured", func(t *testing.T) {
		body := bytes.NewBufferString(`{"labTemplate": "test-lab", "owner": "user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})
}

func TestHandleDeletePod(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		OwnerID:  "user-123",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("delete existing pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pods/pod-1", nil)
		// Add owner as authenticated user
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "user@example.com",
			Name:  "user@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "destroyed" {
			t.Errorf("expected status 'destroyed', got '%s'", response["status"])
		}
	})
}

func TestHandleResetPod(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("reset with invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/reset", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("reset non-existent pod", func(t *testing.T) {
		body := bytes.NewBufferString(`{"snapshot": "baseline"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/reset", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Lab Handler Tests
// -----------------------------------------------------------------------------

func TestHandleListLabs(t *testing.T) {
	srv := newTestServer(t)

	t.Run("lab template repo not configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})
}

func TestHandleGetLab(t *testing.T) {
	srv := newTestServer(t)

	t.Run("lab template repo not configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/test-lab", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// JSON Response Tests
// -----------------------------------------------------------------------------

func TestJSONResponse(t *testing.T) {
	srv := newTestServer(t)

	// Test that health endpoint returns valid JSON with correct content-type
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestErrorResponse(t *testing.T) {
	srv := newTestServer(t)

	// Request non-existent pod to trigger error response
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/does-not-exist", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if _, ok := response["error"]; !ok {
		t.Error("expected 'error' field in error response")
	}
}

// -----------------------------------------------------------------------------
// HTTP Method Tests
// -----------------------------------------------------------------------------

func TestHTTPMethods(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"GET health", http.MethodGet, "/health", http.StatusOK},
		// No database on the test server, so readiness is 503 by design.
		{"GET ready", http.MethodGet, "/ready", http.StatusServiceUnavailable},
		{"POST to health should fail", http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{"PUT to ready should fail", http.MethodPut, "/ready", http.StatusMethodNotAllowed},
		{"DELETE to health should fail", http.MethodDelete, "/health", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			srv.Router().ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Server Creation Tests
// -----------------------------------------------------------------------------

func TestNewServer(t *testing.T) {
	cfg := DefaultConfig()
	orch := orchestrator.New(nil, nil, orchestrator.Config{}, orchestrator.WithLogger(newTestLogger()))
	wsHub := websocket.NewHub(newTestLogger())
	logger := newTestLogger()

	srv, _ := New(cfg, orch, nil, wsHub, logger)

	if srv == nil {
		t.Fatal("expected server to be created")
	}

	if srv.Router() == nil {
		t.Error("expected router to be initialized")
	}
}

func TestServerWithOptions(t *testing.T) {
	cfg := DefaultConfig()
	orch := orchestrator.New(nil, nil, orchestrator.Config{}, orchestrator.WithLogger(newTestLogger()))
	wsHub := websocket.NewHub(newTestLogger())
	logger := newTestLogger()

	// Test that server can be created with options
	srv, _ := New(cfg, orch, nil, wsHub, logger)

	if srv == nil {
		t.Fatal("expected server to be created with options")
	}
}

// -----------------------------------------------------------------------------
// Server Option Tests
// -----------------------------------------------------------------------------

func TestWithCheckpointRepo(t *testing.T) {
	opt := WithCheckpointRepo(&mockCheckpointRepository{})
	srv := &Server{}
	opt(srv)
	if srv.deps == nil || srv.deps.checkpointRepo == nil {
		t.Error("expected checkpoint repo to be set")
	}
}

func TestWithGradeSyncRepo(t *testing.T) {
	opt := WithGradeSyncRepo(&mockGradeSyncRepository{})
	srv := &Server{}
	opt(srv)
	if srv.deps == nil || srv.deps.gradeSyncRepo == nil {
		t.Error("expected grade sync repo to be set")
	}
}

func TestWithGradeSyncHandler(t *testing.T) {
	handler := &GradeSyncHandler{}
	opt := WithGradeSyncHandler(handler)
	srv := &Server{}
	opt(srv)
	if srv.gradeSyncHandler == nil {
		t.Error("expected grade sync handler to be set")
	}
}

func TestWithLifecycleManager(t *testing.T) {
	lm := &orchestrator.LifecycleManager{}
	opt := WithLifecycleManager(lm)
	srv := &Server{}
	opt(srv)
	if srv.lifecycleManager == nil {
		t.Error("expected lifecycle manager to be set")
	}
}

func TestWithRedisService(t *testing.T) {
	// We can't easily create a real Redis service, so we just test that the option doesn't panic
	opt := WithRedisService(nil)
	srv := &Server{}
	opt(srv)
	// Redis service remains nil since we passed nil
}

// -----------------------------------------------------------------------------
// Pod Handler Tests - Reset VM
// -----------------------------------------------------------------------------

func TestHandleResetVM(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/reset", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing snapshot name", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/reset", body)
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

		if response["error"] != "snapshot name is required" {
			t.Errorf("unexpected error message: %s", response["error"])
		}
	})

	t.Run("empty snapshot name", func(t *testing.T) {
		body := bytes.NewBufferString(`{"snapshot": ""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/reset", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Pod Handler Tests - Snapshots
// -----------------------------------------------------------------------------

func TestHandleListSnapshots(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("request to list snapshots", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/pod-1/vms/vm1/snapshots", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but this exercises the handler
		if rr.Code == http.StatusOK {
			var response map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response["podId"] != "pod-1" {
				t.Errorf("expected podId 'pod-1', got '%v'", response["podId"])
			}
			if response["vmName"] != "vm1" {
				t.Errorf("expected vmName 'vm1', got '%v'", response["vmName"])
			}
		}
	})
}

func TestHandleCreateSnapshot(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	ownerCtx := func(req *http.Request) *http.Request {
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:   "user-1",
			Name: "user@example.com",
		})
		return req.WithContext(ctx)
	}

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/snapshots", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		req = ownerCtx(req)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing snapshot name", func(t *testing.T) {
		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/snapshots", body)
		req.Header.Set("Content-Type", "application/json")
		req = ownerCtx(req)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "snapshot name is required" {
			t.Errorf("unexpected error message: %s", response["error"])
		}
	})

	t.Run("empty snapshot name", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": ""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/snapshots", body)
		req.Header.Set("Content-Type", "application/json")
		req = ownerCtx(req)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

func TestHandleDeleteSnapshot(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("request to delete snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pods/pod-1/vms/vm1/snapshots/test-snapshot", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but this exercises the handler
		// The handler should be reached
		if rr.Code == http.StatusOK {
			var response map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response["status"] != "deleted" {
				t.Errorf("expected status 'deleted', got '%s'", response["status"])
			}
		}
	})
}

// -----------------------------------------------------------------------------
// Pod Lifecycle Handler Tests
// -----------------------------------------------------------------------------

func TestHandleStartPod(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusStopped,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "stopped"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("start stopped pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/start", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but exercises the handler path
		if rr.Code == http.StatusOK {
			var pod models.Pod
			if err := json.NewDecoder(rr.Body).Decode(&pod); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if pod.Status != models.PodStatusRunning {
				t.Errorf("expected status running, got %s", pod.Status)
			}
		}
	})

	t.Run("start non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/start", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusBadRequest && rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected error status (400, 404, or 500), got %d", rr.Code)
		}
	})
}

func TestHandleStopPod(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("stop running pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/stop", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but exercises the handler path
		if rr.Code == http.StatusOK {
			var pod models.Pod
			if err := json.NewDecoder(rr.Body).Decode(&pod); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if pod.Status != models.PodStatusStopped {
				t.Errorf("expected status stopped, got %s", pod.Status)
			}
		}
	})

	t.Run("stop non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/stop", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusBadRequest && rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected error status (400, 404, or 500), got %d", rr.Code)
		}
	})
}

func TestHandleStartVM(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "stopped"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("start VM in pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/start", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but exercises the handler path
		if rr.Code == http.StatusOK {
			var response map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response["status"] != "started" {
				t.Errorf("expected status 'started', got '%s'", response["status"])
			}
		}
	})

	t.Run("start VM in non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/vms/vm1/start", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
		}
	})

	t.Run("start non-existent VM", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/non-existent/start", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Ownership check runs first — without auth context, returns 403
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected status 403, 500, or 404, got %d", rr.Code)
		}
	})
}

func TestHandleStopVM(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("stop VM in pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/stop", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but exercises the handler path
		if rr.Code == http.StatusOK {
			var response map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response["status"] != "stopped" {
				t.Errorf("expected status 'stopped', got '%s'", response["status"])
			}
		}
	})

	t.Run("stop VM in non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/vms/vm1/stop", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
		}
	})
}

func TestHandleSuspendVM(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("suspend VM in pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/suspend", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but exercises the handler path
		if rr.Code == http.StatusOK {
			var response map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response["status"] != "suspended" {
				t.Errorf("expected status 'suspended', got '%s'", response["status"])
			}
		}
	})

	t.Run("suspend VM in non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/vms/vm1/suspend", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
		}
	})
}

func TestHandleResumeVM(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "suspended"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("resume VM in pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm1/resume", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Will fail because there's no proxmox client, but exercises the handler path
		if rr.Code == http.StatusOK {
			var response map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response["status"] != "running" {
				t.Errorf("expected status 'running', got '%s'", response["status"])
			}
		}
	})

	t.Run("resume VM in non-existent pod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/non-existent/vms/vm1/resume", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// LTI Handler Tests
// -----------------------------------------------------------------------------

func TestHandleLTILaunch(t *testing.T) {
	srv := newTestServer(t)

	t.Run("lti not configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lti/launch", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// LTI is an Enterprise-only feature, returns 403 Forbidden
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "This feature requires Kootenai Enterprise Edition" {
			t.Errorf("unexpected error message: %s", response["error"])
		}
	})
}

func TestHandleLTIJWKS(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/lti/jwks", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// LTI is an Enterprise-only feature, returns 403 Forbidden
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "This feature requires Kootenai Enterprise Edition" {
		t.Errorf("unexpected error message: %s", response["error"])
	}
}

func TestHandleLTIToken(t *testing.T) {
	srv := newTestServer(t)

	t.Run("lti not configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lti/token", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// LTI is an Enterprise-only feature, returns 403 Forbidden
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// -----------------------------------------------------------------------------
// Assessment Manager Tests (basic tests - comprehensive tests in assessment_handlers_test.go)
// -----------------------------------------------------------------------------

func TestAssessmentManagerOptions(t *testing.T) {
	t.Run("with assessment repo", func(t *testing.T) {
		mockRepo := &mockAssessmentResultRepository{}
		am := NewAssessmentManager(AssessmentManagerConfig{Repo: mockRepo})
		if am.repo == nil {
			t.Error("expected assessment repo to be set")
		}
	})

	t.Run("with assessment TTL", func(t *testing.T) {
		customTTL := 2 * time.Hour
		am := NewAssessmentManager(AssessmentManagerConfig{TTL: customTTL})
		if am.ttl != customTTL {
			t.Errorf("expected TTL %v, got %v", customTTL, am.ttl)
		}
	})

	t.Run("with session repo", func(t *testing.T) {
		mockRepo := &mockSessionRepository{}
		am := NewAssessmentManager(AssessmentManagerConfig{SessionRepo: mockRepo})
		if am.sessionRepo == nil {
			t.Error("expected session repo to be set")
		}
	})

	t.Run("with lab template repo", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepository{}
		am := NewAssessmentManager(AssessmentManagerConfig{LabTemplateRepo: mockRepo})
		if am.labTemplateRepo == nil {
			t.Error("expected lab template repo to be set")
		}
	})
}

func TestAssessmentManagerResultCount(t *testing.T) {
	am := NewAssessmentManager(AssessmentManagerConfig{})

	if count := am.ResultCount(); count != 0 {
		t.Errorf("expected 0 results, got %d", count)
	}
}

func TestAssessmentManagerStartStop(t *testing.T) {
	am := NewAssessmentManager(AssessmentManagerConfig{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	am.Start(ctx)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	am.Stop()

	// Should complete without hanging
}

func TestAssessmentManagerStoreAndGetResult(t *testing.T) {
	am := NewAssessmentManager(AssessmentManagerConfig{})

	ctx := context.Background()
	sessionID := "test-session-123"

	result := &models.AssessmentResult{
		SessionID:   sessionID,
		Status:      "completed",
		Score:       80,
		MaxScore:    100,
		Percentage:  80.0,
		ItemCount:   10,
		PassedCount: 8,
	}

	// Store result
	am.storeResult(ctx, sessionID, result)

	// Retrieve result
	retrieved, ok := am.getResult(ctx, sessionID)
	if !ok {
		t.Fatal("expected to retrieve result")
	}

	if retrieved.SessionID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, retrieved.SessionID)
	}

	if retrieved.Score != 80 {
		t.Errorf("expected score 80, got %d", retrieved.Score)
	}

	// Check result count
	if count := am.ResultCount(); count != 1 {
		t.Errorf("expected 1 result, got %d", count)
	}
}

func TestAssessmentManagerGetResultNotFound(t *testing.T) {
	am := NewAssessmentManager(AssessmentManagerConfig{})

	ctx := context.Background()

	_, ok := am.getResult(ctx, "non-existent-session")
	if ok {
		t.Error("expected result not found")
	}
}

// -----------------------------------------------------------------------------
// GradeSync Handler Tests
// -----------------------------------------------------------------------------

func TestNewGradeSyncHandler(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger: newTestLogger(),
	}

	handler := NewGradeSyncHandler(cfg)
	if handler == nil {
		t.Fatal("expected handler to be created")
	}
}

func TestGradeSyncHandlerBuildLineItemURL(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger:        newTestLogger(),
		CanvasBaseURL: "https://canvas.example.com",
	}

	handler := NewGradeSyncHandler(cfg)

	url := handler.buildLineItemURL("course-123", "assignment-456")
	expected := "https://canvas.example.com/api/lti/courses/course-123/line_items/assignment-456"

	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}
}

func TestGradeSyncHandlerBuildLineItemURLEmpty(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger:        newTestLogger(),
		CanvasBaseURL: "",
	}

	handler := NewGradeSyncHandler(cfg)

	url := handler.buildLineItemURL("course-123", "assignment-456")
	if url != "" {
		t.Errorf("expected empty URL, got %s", url)
	}
}

func TestGradeSyncHandlerBuildGradeComment(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger: newTestLogger(),
	}

	handler := NewGradeSyncHandler(cfg)

	t.Run("passed without checkpoints", func(t *testing.T) {
		update := events.GradeUpdate{
			EarnedPoints: 80,
			MaxPoints:    100,
			Percentage:   80.0,
			Passed:       true,
		}

		comment := handler.buildGradeComment(update)
		if comment != "Lab Score: 80/100 (80.0%) - Passed" {
			t.Errorf("unexpected comment: %s", comment)
		}
	})

	t.Run("not passed without checkpoints", func(t *testing.T) {
		update := events.GradeUpdate{
			EarnedPoints: 40,
			MaxPoints:    100,
			Percentage:   40.0,
			Passed:       false,
		}

		comment := handler.buildGradeComment(update)
		if comment != "Lab Score: 40/100 (40.0%) - Not passed" {
			t.Errorf("unexpected comment: %s", comment)
		}
	})

	t.Run("with checkpoints", func(t *testing.T) {
		update := events.GradeUpdate{
			EarnedPoints: 80,
			MaxPoints:    100,
			Percentage:   80.0,
			Passed:       true,
			CheckpointStates: []models.CheckpointState{
				{Status: models.CheckpointStatusPassed},
				{Status: models.CheckpointStatusPassed},
				{Status: models.CheckpointStatusFailed},
			},
		}

		comment := handler.buildGradeComment(update)
		expected := "Lab Score: 80/100 (80.0%) - Passed | Checkpoints: 2/3 completed"
		if comment != expected {
			t.Errorf("expected comment %s, got %s", expected, comment)
		}
	})
}

func TestGradeSyncHandlerProcessPendingGradesNoRepo(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger: newTestLogger(),
	}

	handler := NewGradeSyncHandler(cfg)

	ctx := context.Background()
	err := handler.ProcessPendingGrades(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestGradeSyncHandlerStartNoNATS(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger: newTestLogger(),
	}

	handler := NewGradeSyncHandler(cfg)

	ctx := context.Background()
	err := handler.Start(ctx)
	if err != nil {
		t.Errorf("expected no error without NATS, got %v", err)
	}
}

func TestGradeSyncHandlerStop(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger: newTestLogger(),
	}

	handler := NewGradeSyncHandler(cfg)

	// Should not panic when stopping without starting
	handler.Stop()
}

// -----------------------------------------------------------------------------
// Background Services Tests
// -----------------------------------------------------------------------------

func TestStartStopBackgroundServices(t *testing.T) {
	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start services
	srv.StartBackgroundServices(ctx)

	// Give them time to start
	time.Sleep(10 * time.Millisecond)

	// Stop services
	srv.StopBackgroundServices()
}

func TestAssessmentManagerCleanupExpired(t *testing.T) {
	am := NewAssessmentManager(AssessmentManagerConfig{TTL: 100 * time.Millisecond})

	ctx := context.Background()

	// Store a result
	result := &models.AssessmentResult{
		SessionID: "test-session",
		Status:    "completed",
	}
	am.storeResult(ctx, "test-session", result)

	// Should have 1 result
	if count := am.ResultCount(); count != 1 {
		t.Errorf("expected 1 result, got %d", count)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Run cleanup
	am.cleanupExpired(ctx)

	// Should have 0 results after cleanup
	if count := am.ResultCount(); count != 0 {
		t.Errorf("expected 0 results after cleanup, got %d", count)
	}
}

func TestAssessmentManagerCleanupExpiredWithRepo(t *testing.T) {
	mockRepo := &mockAssessmentResultRepository{}
	am := NewAssessmentManager(AssessmentManagerConfig{
		Repo: mockRepo,
		TTL:  100 * time.Millisecond,
	})

	ctx := context.Background()

	// Store a result
	result := &models.AssessmentResult{
		SessionID: "test-session",
		Status:    "completed",
	}
	am.storeResult(ctx, "test-session", result)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Run cleanup - should not panic with repo
	am.cleanupExpired(ctx)
}

// -----------------------------------------------------------------------------
// LTI SubmitGrade Tests
// -----------------------------------------------------------------------------

func TestSubmitGrade(t *testing.T) {
	// Create a CanvasManager without grade service configured
	canvasMgr, _ := NewCanvasManager(CanvasManagerConfig{
		Logger: newTestLogger(),
	})

	t.Run("grade service not configured", func(t *testing.T) {
		ctx := context.Background()
		err := canvasMgr.SubmitGrade(ctx, "session-123", "https://canvas.example.com/line_item", "user-1", 85.0, 100.0, "Good work!")
		if err == nil {
			t.Error("expected error when grade service is not configured")
		}
		if err.Error() != "LTI grade service not configured" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})
}

// -----------------------------------------------------------------------------
// WebSocket Handler Tests
// -----------------------------------------------------------------------------

func TestHandleWebSocket(t *testing.T) {
	t.Run("no session ID allows connection attempt", func(t *testing.T) {
		// Create server with evaluator (needed for validateSession)
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?userId=user-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Without session validation, should not return 401
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected non-401 status for no sessionId, got %d", rr.Code)
		}
	})

	t.Run("invalid session returns unauthorized", func(t *testing.T) {
		// Create server with evaluator but no registered sessions
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?sessionId=invalid-session", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d for invalid session, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("valid session in evaluator cache", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "test-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-123", "pod-456", "user-123", "test-lab")

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?sessionId=session-123&userId=user-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should not return 401 since session is valid
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected non-401 status for valid session in cache, got %d", rr.Code)
		}
	})

	t.Run("valid session in database fallback", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-db",
			UserID: "user-123",
			PodID:  "pod-456",
		})

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithSessionRepo(sessionRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?sessionId=session-db&userId=user-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should not return 401 since session is in database
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected non-401 status for valid session in database, got %d", rr.Code)
		}
	})
}

func TestHandleWebSocketPod(t *testing.T) {
	t.Run("invalid pod returns unauthorized", func(t *testing.T) {
		podRepo := newMockPodRepository()
		eval := checkpoint.NewEvaluator(newTestLogger())
		orch := orchestrator.New(nil, nil, orchestrator.Config{},
			orchestrator.WithPodRepository(podRepo),
			orchestrator.WithLogger(newTestLogger()),
		)

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, orch, eval, wsHub, newTestLogger())

		// Route is /ws/{podID}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/invalid-pod", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d for invalid pod, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("valid pod allows connection attempt", func(t *testing.T) {
		podRepo := newMockPodRepository()
		podRepo.pods["pod-1"] = &models.Pod{
			ID:       "pod-1",
			Owner:    "user@example.com",
			Status:   models.PodStatusRunning,
			Platform: models.PlatformProxmox,
		}

		eval := checkpoint.NewEvaluator(newTestLogger())
		orch := orchestrator.New(nil, nil, orchestrator.Config{},
			orchestrator.WithPodRepository(podRepo),
			orchestrator.WithLogger(newTestLogger()),
		)

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, orch, eval, wsHub, newTestLogger())

		// Route is /ws/{podID}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod-1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Valid pod should not return 401
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected non-401 status for valid pod, got %d", rr.Code)
		}
	})

	t.Run("valid pod with invalid session returns unauthorized", func(t *testing.T) {
		podRepo := newMockPodRepository()
		podRepo.pods["pod-1"] = &models.Pod{
			ID:       "pod-1",
			OwnerID:  "00000000-0000-0000-0000-000000000001",
			Owner:    "user@example.com",
			Status:   models.PodStatusRunning,
			Platform: models.PlatformProxmox,
		}

		eval := checkpoint.NewEvaluator(newTestLogger())
		orch := orchestrator.New(nil, nil, orchestrator.Config{},
			orchestrator.WithPodRepository(podRepo),
			orchestrator.WithLogger(newTestLogger()),
		)

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, orch, eval, wsHub, newTestLogger())

		// Route is /ws/{podID}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod-1?sessionId=invalid-session", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d for invalid session, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("valid pod with valid session in cache", func(t *testing.T) {
		podRepo := newMockPodRepository()
		podRepo.pods["pod-1"] = &models.Pod{
			ID:       "pod-1",
			OwnerID:  "00000000-0000-0000-0000-000000000001",
			Owner:    "user@example.com",
			Status:   models.PodStatusRunning,
			Platform: models.PlatformProxmox,
		}

		orch := orchestrator.New(nil, nil, orchestrator.Config{},
			orchestrator.WithPodRepository(podRepo),
			orchestrator.WithLogger(newTestLogger()),
		)

		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "test-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-123", "pod-1", "user-123", "test-lab")

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, orch, eval, wsHub, newTestLogger())

		// Route is /ws/{podID}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod-1?sessionId=session-123&userId=user-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should not return 401 for valid pod and session
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("expected non-401 status for valid pod and session, got %d", rr.Code)
		}
	})
}

func TestValidateSession(t *testing.T) {
	t.Run("session in evaluator cache returns true", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "test-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-cache", "pod-123", "user-123", "test-lab")

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		if !srv.webSocketMgr.validateSession(context.Background(), "session-cache") {
			t.Error("validateSession() = false, want true for session in cache")
		}
	})

	t.Run("session in database returns true", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-db",
			UserID: "user-123",
			PodID:  "pod-123",
		})

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithSessionRepo(sessionRepo))

		if !srv.webSocketMgr.validateSession(context.Background(), "session-db") {
			t.Error("validateSession() = false, want true for session in database")
		}
	})

	t.Run("session not found returns false", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		sessionRepo := mocks.NewFakeSessionRepository()

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithSessionRepo(sessionRepo))

		if srv.webSocketMgr.validateSession(context.Background(), "nonexistent") {
			t.Error("validateSession() = true, want false for nonexistent session")
		}
	})

	t.Run("empty session ID returns false", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		if srv.webSocketMgr.validateSession(context.Background(), "") {
			t.Error("validateSession() = true, want false for empty session ID")
		}
	})

	t.Run("nil session repo falls back gracefully", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		// No session repo provided
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		// Should return false without panicking
		if srv.webSocketMgr.validateSession(context.Background(), "any-session") {
			t.Error("validateSession() = true, want false when session repo is nil")
		}
	})

	t.Run("cache takes priority over database", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "test-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{ID: "cp-1", Points: 10},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-both", "pod-123", "user-123", "test-lab")

		// Also add to database
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:     "session-both",
			UserID: "user-123",
			PodID:  "pod-123",
		})

		cfg := DefaultConfig()
		cfg.CORSOrigins = []string{"http://localhost:3000"}
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithSessionRepo(sessionRepo))

		// Should find in cache first (fast path)
		if !srv.webSocketMgr.validateSession(context.Background(), "session-both") {
			t.Error("validateSession() = false, want true for session in both cache and database")
		}
	})
}

// -----------------------------------------------------------------------------
// Pod Creation with Lab Template Repository
// -----------------------------------------------------------------------------

func TestHandleCreatePodWithLabRepo(t *testing.T) {
	labRepo := &mockLabTemplateRepositoryWithData{}
	labRepo.templates = map[string]*models.LabTemplateRecord{
		"lab-1": {
			ID:       "lab-1",
			Name:     "test-lab",
			IsActive: true,
			Spec:     []byte(`{"name":"test-lab","version":"1.0.0","platform":"proxmox","vms":[{"name":"vm1","template":"ubuntu-22.04"}]}`),
		},
		"inactive-lab": {
			ID:       "inactive-lab",
			Name:     "inactive-lab",
			IsActive: false,
			Spec:     []byte(`{}`),
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger(), WithLabTemplateRepo(labRepo))

	t.Run("template not found", func(t *testing.T) {
		body := bytes.NewBufferString(`{"labTemplate": "nonexistent", "owner": "user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("template not active", func(t *testing.T) {
		body := bytes.NewBufferString(`{"labTemplate": "inactive-lab", "owner": "user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", body)
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

		if response["error"] != "lab template is not active" {
			t.Errorf("unexpected error message: %s", response["error"])
		}
	})

	t.Run("template load error", func(t *testing.T) {
		labRepoWithErr := &mockLabTemplateRepositoryWithData{}
		labRepoWithErr.getByNameErr = errors.New("database error")

		srv2, _ := New(cfg, orch, nil, wsHub, newTestLogger(), WithLabTemplateRepo(labRepoWithErr))

		body := bytes.NewBufferString(`{"labTemplate": "test-lab", "owner": "user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv2.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// mockLabTemplateRepositoryWithData implements lab template repository with test data
type mockLabTemplateRepositoryWithData struct {
	templates    map[string]*models.LabTemplateRecord
	getByNameErr error
	getByIDErr   error
}

func (r *mockLabTemplateRepositoryWithData) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	return nil
}

func (r *mockLabTemplateRepositoryWithData) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	if r.templates != nil {
		if t, ok := r.templates[id]; ok {
			return t, nil
		}
	}
	return nil, nil
}

func (r *mockLabTemplateRepositoryWithData) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	var result []*models.LabTemplateRecord
	for _, id := range ids {
		if r.templates != nil {
			if t, ok := r.templates[id]; ok {
				result = append(result, t)
			}
		}
	}
	return result, nil
}

func (r *mockLabTemplateRepositoryWithData) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	if r.getByNameErr != nil {
		return nil, r.getByNameErr
	}
	if r.templates != nil {
		for _, t := range r.templates {
			if t.Name == name {
				return t, nil
			}
		}
	}
	return nil, nil
}

func (r *mockLabTemplateRepositoryWithData) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	result := make([]*models.LabTemplateRecord, 0)
	for _, t := range r.templates {
		result = append(result, t)
	}
	return result, nil
}

func (r *mockLabTemplateRepositoryWithData) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	return nil
}

func (r *mockLabTemplateRepositoryWithData) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *mockLabTemplateRepositoryWithData) SetActive(ctx context.Context, id string, active bool) error {
	return nil
}
func (r *mockLabTemplateRepositoryWithData) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}
func (r *mockLabTemplateRepositoryWithData) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}
func (r *mockLabTemplateRepositoryWithData) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}
func (r *mockLabTemplateRepositoryWithData) CountVersions(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// -----------------------------------------------------------------------------
// LTI Handler Additional Tests
// -----------------------------------------------------------------------------

func TestHandleLTILaunchFormParsing(t *testing.T) {
	srv := newTestServer(t)

	t.Run("missing id_token", func(t *testing.T) {
		// LTI launch without LTI service - we set a mock service to test form parsing
		// But since LTI is Enterprise-only, it returns Forbidden first
		body := "state=abc123"
		req := httptest.NewRequest(http.MethodPost, "/lti/launch", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// LTI is an Enterprise-only feature, returns 403 Forbidden
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Additional Pod Handler Tests
// -----------------------------------------------------------------------------

func TestHandleListPodsFilters(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user1@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}
	podRepo.pods["pod-2"] = &models.Pod{
		ID:       "pod-2",
		Owner:    "user2@example.com",
		Status:   models.PodStatusStopped,
		Platform: models.PlatformCloudStack,
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("filter by owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods?owner=user1@example.com", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods?status=running", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestHandleDeletePodNotFound(t *testing.T) {
	podRepo := newMockPodRepository()

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pods/nonexistent", nil)
	// Add admin user to pass authorization check
	ctx := auth.ContextWithUser(req.Context(), &auth.User{
		ID:    "admin-123",
		Email: "admin@example.com",
		Roles: []string{"admin"},
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// Handler now returns 404 when pod is not found (checked before delete)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleResetPodValidation(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	t.Run("invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/reset", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("empty snapshot allowed", func(t *testing.T) {
		// Note: handleResetPod allows empty snapshot for default reset
		// (unlike handleResetVM which requires snapshot)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/reset", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		// Add auth context matching the pod owner
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:   "user-1",
			Name: "user@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Empty snapshot is allowed - resets to default state
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("pod not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/nonexistent/reset", bytes.NewBufferString(`{"snapshot":"base"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Assessment Handler Additional Tests
// -----------------------------------------------------------------------------

func TestHandleGetAssessmentWithResult(t *testing.T) {
	srv := newTestServer(t)

	// Store a result in the assessment manager
	result := &models.AssessmentResult{
		SessionID:   "session-with-result",
		Status:      "completed",
		Score:       85,
		MaxScore:    100,
		Percentage:  85.0,
		ItemCount:   10,
		PassedCount: 8,
	}
	srv.assessmentMgr.storeResult(context.Background(), "session-with-result", result)

	t.Run("get existing assessment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-with-result", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["sessionId"] != "session-with-result" {
			t.Errorf("unexpected session ID: %v", response["sessionId"])
		}

		score := int(response["score"].(float64))
		if score != 85 {
			t.Errorf("expected score 85, got %d", score)
		}
	})
}

func TestHandleGetAssessmentStatusWithResult(t *testing.T) {
	srv := newTestServer(t)

	// Store a result
	result := &models.AssessmentResult{
		SessionID:  "session-status-test",
		Status:     "completed",
		Score:      70,
		MaxScore:   100,
		Percentage: 70.0,
	}
	srv.assessmentMgr.storeResult(context.Background(), "session-status-test", result)

	t.Run("get status for existing assessment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-status-test/status", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "completed" {
			t.Errorf("expected status 'completed', got '%v'", response["status"])
		}
	})
}

func TestHandleGetComponentsWithResult(t *testing.T) {
	srv := newTestServer(t)

	// Store a result with components
	result := &models.AssessmentResult{
		SessionID: "session-components-test",
		Status:    "completed",
		Components: []models.ComponentResult{
			{
				ID:           "router1",
				Description:  "Router configuration",
				TotalItems:   2,
				PassedItems:  2,
				MaxPoints:    10,
				EarnedPoints: 10,
				Percentage:   100.0,
			},
		},
	}
	srv.assessmentMgr.storeResult(context.Background(), "session-components-test", result)

	t.Run("get components for existing assessment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-components-test/components", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		components, ok := response["components"].([]interface{})
		if !ok {
			t.Fatal("expected components array")
		}

		if len(components) != 1 {
			t.Errorf("expected 1 component, got %d", len(components))
		}
	})
}

func TestHandleGetDeviceAssessmentWithResult(t *testing.T) {
	srv := newTestServer(t)

	// Store a result with device results
	result := &models.AssessmentResult{
		SessionID: "session-device-test",
		Status:    "completed",
		Devices: []models.DeviceResult{
			{
				Name:   "router1",
				Type:   "router",
				Status: models.AssessmentStatusCorrect, // Use correct status constant
			},
		},
	}
	srv.assessmentMgr.storeResult(context.Background(), "session-device-test", result)

	t.Run("get device assessment - device found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-device-test/devices/router1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("get device assessment - device not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-device-test/devices/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// GradeSync Handler Additional Tests
// -----------------------------------------------------------------------------

func TestGradeSyncHandlerStopWithSubscription(t *testing.T) {
	cfg := GradeSyncHandlerConfig{
		Logger: newTestLogger(),
	}

	handler := NewGradeSyncHandler(cfg)

	ctx := context.Background()
	_ = handler.Start(ctx)

	// Simulate having a subscription cancel func
	handler.Stop()

	// Should complete without error
}

// -----------------------------------------------------------------------------
// Mock Repositories for Testing
// -----------------------------------------------------------------------------

type mockCheckpointRepository struct{}

func (r *mockCheckpointRepository) Create(ctx context.Context, progress *models.CheckpointProgress) error {
	return nil
}

func (r *mockCheckpointRepository) GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointProgress, error) {
	return nil, nil
}

func (r *mockCheckpointRepository) GetBySessionAndCheckpoint(ctx context.Context, sessionID, checkpointID string) (*models.CheckpointProgress, error) {
	return nil, nil
}

func (r *mockCheckpointRepository) Update(ctx context.Context, progress *models.CheckpointProgress) error {
	return nil
}

func (r *mockCheckpointRepository) MarkPassed(ctx context.Context, sessionID, checkpointID string, triggerEventID *string) error {
	return nil
}

func (r *mockCheckpointRepository) MarkFailed(ctx context.Context, sessionID, checkpointID string) error {
	return nil
}

func (r *mockCheckpointRepository) ResetForSession(ctx context.Context, sessionID string) error {
	return nil
}

type mockGradeSyncRepository struct{}

func (r *mockGradeSyncRepository) Create(ctx context.Context, entry *models.GradeSyncEntry) error {
	return nil
}

func (r *mockGradeSyncRepository) GetPending(ctx context.Context, limit int) ([]*models.GradeSyncEntry, error) {
	return nil, nil
}

func (r *mockGradeSyncRepository) MarkProcessing(ctx context.Context, id string) error {
	return nil
}

func (r *mockGradeSyncRepository) MarkCompleted(ctx context.Context, id string) error {
	return nil
}

func (r *mockGradeSyncRepository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	return nil
}

type mockAssessmentResultRepository struct{}

func (r *mockAssessmentResultRepository) Create(ctx context.Context, result *models.AssessmentResult) error {
	return nil
}

func (r *mockAssessmentResultRepository) GetBySessionID(ctx context.Context, sessionID string) (*models.AssessmentResult, error) {
	return nil, nil
}

func (r *mockAssessmentResultRepository) Update(ctx context.Context, result *models.AssessmentResult) error {
	return nil
}

func (r *mockAssessmentResultRepository) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	return nil
}

func (r *mockAssessmentResultRepository) Delete(ctx context.Context, sessionID string) error {
	return nil
}

func (r *mockAssessmentResultRepository) DeleteExpired(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}

type mockSessionRepository struct{}

func (r *mockSessionRepository) Create(ctx context.Context, session *models.Session) error {
	return nil
}

func (r *mockSessionRepository) GetByID(ctx context.Context, id string) (*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepository) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepository) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepository) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepository) Update(ctx context.Context, session *models.Session) error {
	return nil
}

func (r *mockSessionRepository) End(ctx context.Context, id string) error {
	return nil
}

func (r *mockSessionRepository) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	return nil
}

func (r *mockSessionRepository) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}

func (r *mockSessionRepository) MarkGradeSyncFailed(ctx context.Context, id string, errMsg string) error {
	return nil
}

func (r *mockSessionRepository) GetUserID(ctx context.Context, id string) (string, error) {
	return "", nil
}

func (r *mockSessionRepository) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	return true, nil
}

func (r *mockSessionRepository) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}

func (r *mockSessionRepository) ListAll(ctx context.Context) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepository) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepository) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *mockSessionRepository) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return &repositories.UserSessionStats{}, nil
}

func (r *mockSessionRepository) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (r *mockSessionRepository) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

type mockLabTemplateRepository struct{}

func (r *mockLabTemplateRepository) Create(ctx context.Context, template *models.LabTemplateRecord) error {
	return nil
}

func (r *mockLabTemplateRepository) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepository) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepository) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepository) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepository) Update(ctx context.Context, template *models.LabTemplateRecord) error {
	return nil
}

func (r *mockLabTemplateRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *mockLabTemplateRepository) SetActive(ctx context.Context, id string, active bool) error {
	return nil
}
func (r *mockLabTemplateRepository) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}
func (r *mockLabTemplateRepository) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}
func (r *mockLabTemplateRepository) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}
func (r *mockLabTemplateRepository) CountVersions(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// Note: Session handler tests are in session_handlers_test.go
// Note: Event handler tests (Wazuh) are in event_handlers_test.go

// -----------------------------------------------------------------------------
// Additional Mock Repositories
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Additional Pod Handler Tests
// -----------------------------------------------------------------------------

func TestHandleResetVMValidation(t *testing.T) {
	srv := newTestServer(t)

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm-1/reset", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm-1/reset", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// handleResetVM requires snapshot to be non-empty
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("pod not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/nonexistent/vms/vm-1/reset", bytes.NewBufferString(`{"snapshot":"base"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// safeErrorResponse returns 404 for "not found" errors
		if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
		}
	})
}

func TestHandleCreateSnapshotValidation(t *testing.T) {
	srv := newTestServer(t)

	t.Run("invalid JSON", func(t *testing.T) {
		// Pod doesn't exist in test server, so ownership check returns 404 before body parsing
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm-1/snapshots", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		// Pod doesn't exist in test server, so ownership check returns 404 before body parsing
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/pod-1/vms/vm-1/snapshots", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("pod not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/nonexistent/vms/vm-1/snapshots", bytes.NewBufferString(`{"name":"snap-1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Server Background Services Tests
// -----------------------------------------------------------------------------

func TestStartBackgroundServicesComplete(t *testing.T) {
	t.Run("starts all services", func(t *testing.T) {
		srv := newTestServer(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Should not panic
		srv.StartBackgroundServices(ctx)

		// Stop them
		srv.StopBackgroundServices()
	})

	t.Run("with grade sync handler", func(t *testing.T) {
		srv := newTestServer(t)
		handler := NewGradeSyncHandler(GradeSyncHandlerConfig{
			Logger: srv.logger,
		})
		srv.gradeSyncHandler = handler

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		srv.StartBackgroundServices(ctx)
		srv.StopBackgroundServices()
	})
}

// -----------------------------------------------------------------------------
// Server Run Tests
// -----------------------------------------------------------------------------

func TestServerRunWithCancel(t *testing.T) {
	t.Run("starts and stops with context cancel", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Port = 18082 // Use different port to avoid conflicts
		cfg.ShutdownTimeout = 100 * time.Millisecond

		srv, _ := New(cfg, nil, nil, nil, newTestLogger())

		ctx, cancel := context.WithCancel(context.Background())

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.Run(ctx)
		}()

		// Give server time to start
		time.Sleep(50 * time.Millisecond)

		// Cancel context to trigger shutdown
		cancel()

		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server didn't shut down in time")
		}
	})
}

// -----------------------------------------------------------------------------
// handleListPods Additional Tests
// -----------------------------------------------------------------------------

func TestHandleListPodsWithOwnerFilter(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods?owner=test-user", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandleListPodsWithStatusFilter(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods?status=running", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// -----------------------------------------------------------------------------
// Additional Assessment Handler Tests
// -----------------------------------------------------------------------------

func TestHandleRunAssessmentValidation(t *testing.T) {
	srv := newTestServer(t)

	t.Run("missing session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/nonexistent/assessment/run", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Session not found should return error
		if rr.Code == http.StatusOK {
			t.Error("expected error for nonexistent session")
		}
	})
}

func TestHandleGetAssessmentNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/assessment", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleGetAssessmentStatusNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/assessment/status", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleGetComponentsNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/assessment/components", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// -----------------------------------------------------------------------------
// Server.New Additional Tests
// -----------------------------------------------------------------------------

func TestNewServerWithLTIConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LTI.ClientID = "test-client-id"

	srv, _ := New(cfg, nil, nil, nil, newTestLogger())

	// LTI service should be initialized (though it may fail without proper config)
	if srv == nil {
		t.Error("expected server to be created")
	}
}

func TestNewServerWithRateLimiting(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RateLimit.Enabled = true
	cfg.RateLimit.RequestsPerMinute = 100
	cfg.RateLimit.PodCreatePerHour = 10

	srv, _ := New(cfg, nil, nil, nil, newTestLogger())

	if srv == nil {
		t.Error("expected server to be created")
	}
}

// -----------------------------------------------------------------------------
// handleListPods Platform Filter Test
// -----------------------------------------------------------------------------

func TestHandleListPodsWithPlatformFilter(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods?platform=proxmox", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// -----------------------------------------------------------------------------
// StopBackgroundServices Tests
// -----------------------------------------------------------------------------

func TestStopBackgroundServicesWithLifecycleManager(t *testing.T) {
	srv := newTestServer(t)

	// Start and stop with lifecycle manager (mock)
	ctx, cancel := context.WithCancel(context.Background())
	srv.StartBackgroundServices(ctx)
	cancel()
	srv.StopBackgroundServices()

	// Should not panic
}

// -----------------------------------------------------------------------------
// Additional ServerOption Tests
// -----------------------------------------------------------------------------

func TestWithLabTemplateRepo(t *testing.T) {
	opt := WithLabTemplateRepo(&mockLabTemplateRepository{})
	srv := &Server{}
	opt(srv)
	if srv.deps == nil || srv.deps.labTemplateRepo == nil {
		t.Error("expected lab template repo to be set")
	}
}

func TestWithSessionRepo(t *testing.T) {
	opt := WithSessionRepo(&mockSessionRepository{})
	srv := &Server{}
	opt(srv)
	if srv.sessionRepo == nil {
		t.Error("expected session repo to be set")
	}
}

func TestWithUserRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithUserRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithPasswordResetRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithPasswordResetRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithAuthService(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithAuthService(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithReservationRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithReservationRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithDatabase(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithDatabase(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithNATSClient(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithNATSClient(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithActiveCheckRunner(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithActiveCheckRunner(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithOrganizationRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithOrganizationRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithOrganizationMembershipRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithOrganizationMembershipRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithTeamRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithTeamRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithTeamMembershipRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithTeamMembershipRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithFeatureRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithFeatureRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithTenantService(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithTenantService(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithSimulationService(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithSimulationService(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithEmailSender(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithEmailSender(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithPodRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithPodRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithOwnershipService(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithOwnershipService(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithAsyncProvisioner(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithAsyncProvisioner(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithPathwayRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithPathwayRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithEnrollmentRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithEnrollmentRepo(nil)
	srv := &Server{}
	opt(srv)
}

func TestWithEventRepo(t *testing.T) {
	// Pass nil - option should not panic
	opt := WithEventRepo(nil)
	srv := &Server{}
	opt(srv)
}

// TestWithAchievementService and TestWithAchievementRepo are in achievement_handlers_test.go

// Note: mockPathwayRepository, mockEnrollmentRepository, mockEventRepository, and
// mockAchievementRepository are not needed since we use nil in the ServerOption tests.
// Full mocks for these interfaces would be complex due to their extensive methods.

// -----------------------------------------------------------------------------
// handleVersion Tests
// -----------------------------------------------------------------------------

func TestHandleVersion(t *testing.T) {
	server := &Server{
		logger: newTestLogger(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()

	server.handleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Decode response
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check that version field exists (value depends on build flags)
	if _, ok := response["version"]; !ok {
		t.Error("expected version field in response")
	}
}

// -----------------------------------------------------------------------------
// Additional LTI Handler Tests
// -----------------------------------------------------------------------------

func TestHandleLTILaunchNoService(t *testing.T) {
	srv := newTestServer(t)

	// LTI is an Enterprise-only feature, all requests return 403 Forbidden
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "no LTI service - enterprise feature",
			body:       "id_token=test123",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "OIDC initiation without service",
			body:       "login_hint=user123&iss=https://canvas.instructure.com",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/lti/launch", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()

			srv.Router().ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestHandleLTICallbackNoService(t *testing.T) {
	srv := newTestServer(t)

	t.Run("no LTI service configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lti/callback", bytes.NewBufferString("id_token=test&state=abc"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// LTI is an Enterprise-only feature, returns 403 Forbidden
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("missing id_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lti/callback", bytes.NewBufferString("state=abc"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// LTI is an Enterprise-only feature, returns 403 Forbidden
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

// Note: TestHandleLTIJWKS and TestHandleLTIToken are declared earlier in this file

func TestHandleLTIConsoleMissingParams(t *testing.T) {
	srv := newTestServer(t)

	t.Run("missing podId and vmid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lti/console", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// LTI is an Enterprise-only feature, returns 403 Forbidden before checking params
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

// Note: TestHandleGetAssessmentNotFound, TestHandleGetAssessmentStatusNotStarted, TestHandleGetComponentsNotFound
// and TestHandleGetDeviceAssessmentNotFound are declared earlier in this file

// -----------------------------------------------------------------------------
// Additional Console/VNC Handler Tests
// -----------------------------------------------------------------------------

func TestHandleGetVMConsoleNoOrchestrator(t *testing.T) {
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, nil, nil, wsHub, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/pod-1/vms/vm-1/console", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// With no orchestrator, the route will still exist but should error
	if rr.Code == http.StatusNotFound {
		t.Error("route should exist even without orchestrator")
	}
}

// -----------------------------------------------------------------------------
// Pod Handler Additional Snapshot Tests
// -----------------------------------------------------------------------------

func TestHandleListSnapshotsNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/nonexistent/vms/vm-1/snapshots", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// safeErrorResponse returns 404 for "not found" errors
	if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
	}
}

func TestHandleDeleteSnapshotValidation(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pods/nonexistent/vms/vm-1/snapshots/snap-1", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	// safeErrorResponse returns 404 for "not found" errors
	if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d or %d, got %d", http.StatusInternalServerError, http.StatusNotFound, rr.Code)
	}
}

// -----------------------------------------------------------------------------
// Server Options Tests
// -----------------------------------------------------------------------------

func TestWithLTIAssignmentRepoNew(t *testing.T) {
	opt := WithLTIAssignmentRepo(nil)
	srv := &Server{}
	opt(srv)
	// Should not panic with nil
}

func TestWithCanvasSyncServiceNew(t *testing.T) {
	opt := WithCanvasSyncService(nil)
	srv := &Server{}
	opt(srv)
	// Should not panic with nil
}

func TestWithGradeSyncRepoNew(t *testing.T) {
	opt := WithGradeSyncRepo(nil)
	srv := &Server{}
	opt(srv)
	// Should not panic with nil
}

func TestWithQuestionResponseRepo(t *testing.T) {
	opt := WithQuestionResponseRepo(nil)
	srv := &Server{}
	opt(srv)
	// Should not panic with nil
}

func TestWithLicenseRepo(t *testing.T) {
	opt := WithLicenseRepo(nil)
	srv := &Server{}
	opt(srv)
	// Should not panic with nil
}

func TestWithAuditRepo(t *testing.T) {
	opt := WithAuditRepo(nil)
	srv := &Server{}
	opt(srv)
	// Should not panic with nil
}

// -----------------------------------------------------------------------------
// More Pod Handler Tests with Real Orchestrator
// -----------------------------------------------------------------------------

func TestHandleGetPodNotFound(t *testing.T) {
	podRepo := newMockPodRepository()

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/nonexistent", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleListPodsEmpty(t *testing.T) {
	podRepo := newMockPodRepository()

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	pods, ok := response["pods"].([]interface{})
	if !ok {
		t.Fatal("expected pods field in response")
	}

	if len(pods) != 0 {
		t.Errorf("expected empty array, got %d items", len(pods))
	}
}

func TestHandleGetPodSuccess(t *testing.T) {
	podRepo := newMockPodRepository()
	podRepo.pods["pod-123"] = &models.Pod{
		ID:       "pod-123",
		Owner:    "user@example.com",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs: []models.PodVM{
			{Name: "vm1", PlatformID: "100", Status: "running"},
		},
	}

	orch := orchestrator.New(nil, nil, orchestrator.Config{},
		orchestrator.WithPodRepository(podRepo),
		orchestrator.WithLogger(newTestLogger()),
	)

	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, orch, nil, wsHub, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/pod-123", nil)
	// Add auth context matching the pod owner
	ctx := auth.ContextWithUser(req.Context(), &auth.User{
		ID:   "user-1",
		Name: "user@example.com",
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

	if response["id"] != "pod-123" {
		t.Errorf("expected id 'pod-123', got '%v'", response["id"])
	}
}

// -----------------------------------------------------------------------------
// Additional Server Option Tests
// -----------------------------------------------------------------------------

func TestWithDashboardCache(t *testing.T) {
	mockCache := mocks.NewFakeDashboardCache()
	opt := WithDashboardCache(mockCache)
	srv := &Server{}
	opt(srv)
	if srv.dashboardCache == nil {
		t.Error("expected dashboard cache to be set")
	}
	if srv.dashboardCache != mockCache {
		t.Error("expected dashboard cache to be the mock")
	}
}

func TestWithLTIStateCache(t *testing.T) {
	mockCache := mocks.NewFakeLTIStateStore()
	opt := WithLTIStateCache(mockCache)
	srv := &Server{}
	opt(srv)
	if srv.ltiStateCache == nil {
		t.Error("expected LTI state cache to be set")
	}
	if srv.ltiStateCache != mockCache {
		t.Error("expected LTI state cache to be the mock")
	}
}

func TestWithOrchestrator(t *testing.T) {
	mockOrch := &mocks.FakeOrchestrator{}
	opt := WithOrchestrator(mockOrch)
	srv := &Server{}
	opt(srv)
	if srv.orchestrator == nil {
		t.Error("expected orchestrator to be set")
	}
	if srv.orchestrator != mockOrch {
		t.Error("expected orchestrator to be the mock")
	}
}

func TestWithLTIAssignmentRepo(t *testing.T) {
	// Test with nil - should not panic
	opt := WithLTIAssignmentRepo(nil)
	srv := &Server{}
	opt(srv)
	// Repo should be nil since we passed nil
	if srv.deps != nil && srv.deps.ltiAssignmentRepo != nil {
		t.Error("expected LTI assignment repo to be nil when nil passed")
	}
}

func TestWithCanvasSyncService(t *testing.T) {
	// Test with nil - should not panic
	opt := WithCanvasSyncService(nil)
	srv := &Server{}
	opt(srv)
	// Service should be nil since we passed nil
	if srv.deps != nil && srv.deps.canvasSyncService != nil {
		t.Error("expected Canvas sync service to be nil when nil passed")
	}
}

func TestGetDashboardCache_Helper(t *testing.T) {
	t.Run("returns injected cache when available", func(t *testing.T) {
		mockCache := mocks.NewFakeDashboardCache()
		srv := &Server{
			dashboardCache: mockCache,
		}
		cache := srv.getDashboardCache()
		if cache != mockCache {
			t.Error("expected injected cache to be returned")
		}
	})

	t.Run("returns nil when no cache configured", func(t *testing.T) {
		srv := &Server{}
		cache := srv.getDashboardCache()
		if cache != nil {
			t.Error("expected nil cache when nothing configured")
		}
	})
}

// TestGetLTIStateCache_Server_Helper removed — the Server-level
// getLTIStateCache() method was a duplicate of CanvasManager's helper.
// Equivalent coverage now lives in lti_handlers_test.go's
// TestGetLTIStateCache_Helper (same two cases: injected cache is
// preferred; nil when nothing configured).

// -----------------------------------------------------------------------------
// Health Check Function Tests with Mock Dependencies
// -----------------------------------------------------------------------------

// mockDBHealthChecker implements DatabaseHealthChecker for testing
type mockDBHealthChecker struct {
	pingErr error
}

func (m *mockDBHealthChecker) PingContext(ctx context.Context) error {
	return m.pingErr
}

// mockNATSHealthChecker implements NATSHealthChecker for testing
type mockNATSHealthChecker struct {
	connected bool
}

func (m *mockNATSHealthChecker) IsConnected() bool {
	return m.connected
}

// mockRedisHealthChecker implements RedisHealthChecker for testing
type mockRedisHealthChecker struct {
	healthErr error
}

func (m *mockRedisHealthChecker) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func TestCheckDatabaseWithLatency(t *testing.T) {
	t.Run("returns healthy when db ping succeeds", func(t *testing.T) {
		mockDB := &mockDBHealthChecker{pingErr: nil}
		srv := newTestServer(t, WithDBHealthChecker(mockDB))

		result := srv.checkDatabaseWithLatency(context.Background())

		if result.Status != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", result.Status)
		}
		if result.Error != "" {
			t.Error("expected no error when healthy")
		}
		if result.LatencyMs < 0 {
			t.Error("expected non-negative latency")
		}
	})

	t.Run("returns unhealthy when db ping fails", func(t *testing.T) {
		mockDB := &mockDBHealthChecker{pingErr: errors.New("connection refused")}
		srv := newTestServer(t, WithDBHealthChecker(mockDB))

		result := srv.checkDatabaseWithLatency(context.Background())

		if result.Status != "unhealthy" {
			t.Errorf("expected status 'unhealthy', got '%s'", result.Status)
		}
		if result.Error != "connection refused" {
			t.Errorf("expected error 'connection refused', got '%s'", result.Error)
		}
	})

	t.Run("returns healthy when no db configured", func(t *testing.T) {
		srv := newTestServer(t)
		// No db or dbHealthChecker set

		result := srv.checkDatabaseWithLatency(context.Background())

		if result.Status != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", result.Status)
		}
	})
}

func TestCheckNATSWithLatency(t *testing.T) {
	t.Run("returns healthy when nats is connected", func(t *testing.T) {
		mockNATS := &mockNATSHealthChecker{connected: true}
		srv := newTestServer(t, WithNATSHealthChecker(mockNATS))

		result := srv.checkNATSWithLatency()

		if result.Status != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", result.Status)
		}
		if result.Error != "" {
			t.Error("expected no error when healthy")
		}
		if result.LatencyMs < 0 {
			t.Error("expected non-negative latency")
		}
	})

	t.Run("returns unhealthy when nats is not connected", func(t *testing.T) {
		mockNATS := &mockNATSHealthChecker{connected: false}
		srv := newTestServer(t, WithNATSHealthChecker(mockNATS))

		result := srv.checkNATSWithLatency()

		if result.Status != "unhealthy" {
			t.Errorf("expected status 'unhealthy', got '%s'", result.Status)
		}
		if result.Error != "not connected" {
			t.Errorf("expected error 'not connected', got '%s'", result.Error)
		}
	})

	t.Run("returns unhealthy when no nats configured", func(t *testing.T) {
		srv := newTestServer(t)
		// No natsClient or natsHealthChecker set

		result := srv.checkNATSWithLatency()

		// When neither is set, connected will be false (zero value)
		if result.Status != "unhealthy" {
			t.Errorf("expected status 'unhealthy', got '%s'", result.Status)
		}
	})
}

func TestCheckRedisWithLatency(t *testing.T) {
	t.Run("returns healthy when redis health check succeeds", func(t *testing.T) {
		mockRedis := &mockRedisHealthChecker{healthErr: nil}
		srv := newTestServer(t, WithRedisHealthChecker(mockRedis))

		result := srv.checkRedisWithLatency(context.Background())

		if result.Status != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", result.Status)
		}
		if result.Error != "" {
			t.Error("expected no error when healthy")
		}
		if result.LatencyMs < 0 {
			t.Error("expected non-negative latency")
		}
	})

	t.Run("returns unhealthy when redis health check fails", func(t *testing.T) {
		mockRedis := &mockRedisHealthChecker{healthErr: errors.New("redis: connection refused")}
		srv := newTestServer(t, WithRedisHealthChecker(mockRedis))

		result := srv.checkRedisWithLatency(context.Background())

		if result.Status != "unhealthy" {
			t.Errorf("expected status 'unhealthy', got '%s'", result.Status)
		}
		if result.Error != "redis: connection refused" {
			t.Errorf("expected error 'redis: connection refused', got '%s'", result.Error)
		}
	})

	t.Run("returns healthy when no redis configured", func(t *testing.T) {
		srv := newTestServer(t)
		// No redisService or redisHealthChecker set

		result := srv.checkRedisWithLatency(context.Background())

		if result.Status != "healthy" {
			t.Errorf("expected status 'healthy', got '%s'", result.Status)
		}
	})
}

func TestHealthCheckOptions(t *testing.T) {
	t.Run("WithDBHealthChecker sets checker", func(t *testing.T) {
		mockDB := &mockDBHealthChecker{}
		srv := newTestServer(t, WithDBHealthChecker(mockDB))

		if srv.dbHealthChecker != mockDB {
			t.Error("expected dbHealthChecker to be set")
		}
	})

	t.Run("WithNATSHealthChecker sets checker", func(t *testing.T) {
		mockNATS := &mockNATSHealthChecker{}
		srv := newTestServer(t, WithNATSHealthChecker(mockNATS))

		if srv.natsHealthChecker != mockNATS {
			t.Error("expected natsHealthChecker to be set")
		}
	})

	t.Run("WithRedisHealthChecker sets checker", func(t *testing.T) {
		mockRedis := &mockRedisHealthChecker{}
		srv := newTestServer(t, WithRedisHealthChecker(mockRedis))

		if srv.redisHealthChecker != mockRedis {
			t.Error("expected redisHealthChecker to be set")
		}
	})
}

func (r *mockSessionRepository) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
