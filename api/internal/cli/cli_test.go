package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// TestMain installs the English CLI localizer once so tests that assert
// on `validateTemplate` output (via T()) see real English strings rather
// than raw message IDs. Sibling pattern to cmd/labctl/main_test.go.
func TestMain(m *testing.M) {
	InstallLocalizer("en")
	os.Exit(m.Run())
}

// -----------------------------------------------------------------------------
// Config Tests
// -----------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.APIBaseURL != "http://localhost:8080" {
		t.Errorf("expected APIBaseURL 'http://localhost:8080', got %q", cfg.APIBaseURL)
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", cfg.Timeout)
	}
}

// -----------------------------------------------------------------------------
// Client Tests
// -----------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	cfg := Config{
		APIBaseURL: "http://test:9000",
		Timeout:    10 * time.Second,
	}

	client := NewClient(cfg)

	if client.baseURL != cfg.APIBaseURL {
		t.Errorf("expected baseURL %q, got %q", cfg.APIBaseURL, client.baseURL)
	}

	if client.httpClient.Timeout != cfg.Timeout {
		t.Errorf("expected timeout %v, got %v", cfg.Timeout, client.httpClient.Timeout)
	}
}

func TestClient_Get_Success(t *testing.T) {
	expected := map[string]string{"status": "ok"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/test" {
			t.Errorf("expected path /test, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	var result map[string]string
	err := client.Get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", result["status"])
	}
}

func TestClient_Get_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	var result map[string]string
	err := client.Get(context.Background(), "/notfound", &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "API error (404): not found"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestClient_Post_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		if body["name"] != "test" {
			t.Errorf("expected name 'test', got %q", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	var result map[string]string
	err := client.Post(context.Background(), "/create", map[string]string{"name": "test"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["id"] != "123" {
		t.Errorf("expected id '123', got %q", result["id"])
	}
}

func TestClient_Delete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	var result map[string]string
	err := client.Delete(context.Background(), "/item/1", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %q", result["status"])
	}
}

func TestClient_Health_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected path /health, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Health_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// -----------------------------------------------------------------------------
// Helper Function Tests
// -----------------------------------------------------------------------------

func TestTruncateID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc", "abc"},
		{"123456789012", "123456789012"},
		{"1234567890123", "123456789012"},
		{"abcdefghijklmnopqrstuvwxyz", "abcdefghijkl"},
		{"", ""},
	}

	for _, tt := range tests {
		result := truncateID(tt.input)
		if result != tt.expected {
			t.Errorf("truncateID(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"running", "● running"},
		{"active", "● active"},
		{"provisioning", "◐ provisioning"},
		{"creating", "◐ creating"},
		{"stopped", "○ stopped"},
		{"error", "✗ error"},
		{"failed", "✗ failed"},
		{"destroying", "◌ destroying"},
		{"destroyed", "⊘ destroyed"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := statusIcon(tt.status)
		if result != tt.expected {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{5 * time.Minute, "5m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h 0m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{23*time.Hour + 59*time.Minute, "23h 59m"},
		{24 * time.Hour, "1d 0h"},
		{48*time.Hour + 12*time.Hour, "2d 12h"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
		}
	}
}

func TestCheckpointIcon(t *testing.T) {
	tests := []struct {
		status   models.CheckpointStatus
		expected string
	}{
		{models.CheckpointStatusPassed, "✓"},
		{models.CheckpointStatusFailed, "✗"},
		{models.CheckpointStatusSkipped, "⊘"},
		{models.CheckpointStatusPartial, "◐"},
		{models.CheckpointStatusPending, "○"},
		{"unknown", "○"},
	}

	for _, tt := range tests {
		result := checkpointIcon(tt.status)
		if result != tt.expected {
			t.Errorf("checkpointIcon(%q) = %q, want %q", tt.status, result, tt.expected)
		}
	}
}

// -----------------------------------------------------------------------------
// PodCommands Tests
// -----------------------------------------------------------------------------

func TestNewPodCommands(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewPodCommands(client)

	if cmds.client != client {
		t.Error("expected client to be set")
	}
}

func TestPodCommands_List_Success(t *testing.T) {
	now := time.Now()
	expires := now.Add(1 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pods" {
			t.Errorf("expected path /api/v1/pods, got %s", r.URL.Path)
		}

		response := PodListResponse{
			Pods: []*models.Pod{
				{
					ID:          "pod-123",
					LabTemplate: "linux-basics",
					Owner:       "user1",
					Status:      models.PodStatusRunning,
					Platform:    models.PlatformProxmox,
					VMs:         []models.PodVM{{Name: "vm1"}},
					CreatedAt:   now,
					ExpiresAt:   &expires,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.List(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_List_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := PodListResponse{Pods: []*models.Pod{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.List(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_List_WithOwnerFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("owner") != "user1" {
			t.Errorf("expected owner=user1, got %s", r.URL.Query().Get("owner"))
		}

		response := PodListResponse{Pods: []*models.Pod{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.List(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_Create_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/pods" {
			t.Errorf("expected path /api/v1/pods, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["labTemplate"] != "linux-basics" {
			t.Errorf("expected labTemplate 'linux-basics', got %v", body["labTemplate"])
		}
		if body["owner"] != "user1" {
			t.Errorf("expected owner 'user1', got %v", body["owner"])
		}

		response := map[string]string{
			"id":     "pod-123",
			"status": "provisioning",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.Create(context.Background(), "linux-basics", "user1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_Destroy_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		response := map[string]string{"status": "destroying"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.Destroy(context.Background(), "pod-123", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_Destroy_Force(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("force") != "true" {
			t.Errorf("expected force=true query param")
		}

		response := map[string]string{"status": "destroying"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.Destroy(context.Background(), "pod-123", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_Reset_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := map[string]string{"status": "resetting"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.Reset(context.Background(), "pod-123", "clean")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_Status_Success(t *testing.T) {
	now := time.Now()
	expires := now.Add(1 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pod := models.Pod{
			ID:          "pod-123",
			LabTemplate: "linux-basics",
			Owner:       "user1",
			Status:      models.PodStatusRunning,
			Platform:    models.PlatformProxmox,
			CreatedAt:   now,
			ExpiresAt:   &expires,
			VMs: []models.PodVM{
				{Name: "vm1", Status: "running", IPAddress: "10.0.0.1", PlatformID: "vm-1234567890123"},
			},
			Networks: []models.PodNetwork{
				{Name: "net1", VLAN: 100, Subnet: "10.0.0.0/24"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pod)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.Status(context.Background(), "pod-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// SessionCommands Tests
// -----------------------------------------------------------------------------

func TestNewSessionCommands(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewSessionCommands(client)

	if cmds.client != client {
		t.Error("expected client to be set")
	}
}

func TestSessionCommands_List_Success(t *testing.T) {
	now := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sessions" {
			t.Errorf("expected path /api/v1/sessions, got %s", r.URL.Path)
		}

		response := SessionListResponse{
			Sessions: []SessionInfo{
				{
					ID:           "session-123",
					PodID:        "pod-123",
					UserID:       "user1",
					LabTemplate:  "linux-basics",
					EarnedPoints: 50,
					MaxPoints:    100,
					Percentage:   50.0,
					StartedAt:    now,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_List_ActiveOnly(t *testing.T) {
	now := time.Now()
	ended := now.Add(-1 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SessionListResponse{
			Sessions: []SessionInfo{
				{
					ID:        "session-active",
					UserID:    "user1",
					StartedAt: now,
				},
				{
					ID:        "session-ended",
					UserID:    "user1",
					StartedAt: now.Add(-2 * time.Hour),
					EndedAt:   &ended,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	// Test with active=true should filter out ended sessions
	err := cmds.List(context.Background(), "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_Start_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		if body["podId"] != "pod-123" {
			t.Errorf("expected podId 'pod-123', got %q", body["podId"])
		}

		response := map[string]string{
			"sessionId": "session-123",
			"status":    "active",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.Start(context.Background(), "pod-123", "user1", "linux-basics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_End_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := map[string]string{"status": "ended"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.End(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_Status_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ProgressResponse{
			SessionID:    "session-123",
			EarnedPoints: 75,
			MaxPoints:    100,
			Percentage:   75.0,
			Checkpoints: []models.CheckpointState{
				{CheckpointID: "cp1", Status: models.CheckpointStatusPassed, Points: 50, EarnedPoints: 50},
				{CheckpointID: "cp2", Status: models.CheckpointStatusPending, Points: 50, EarnedPoints: 0},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.Status(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_Progress_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ProgressResponse{
			SessionID:    "session-123",
			EarnedPoints: 80,
			MaxPoints:    100,
			Percentage:   80.0,
			Checkpoints: []models.CheckpointState{
				{CheckpointID: "cp1", Status: models.CheckpointStatusPassed},
				{CheckpointID: "cp2", Status: models.CheckpointStatusPending},
				{CheckpointID: "cp3", Status: models.CheckpointStatusFailed},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.Progress(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_Checkpoints_Success(t *testing.T) {
	now := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			Checkpoints []models.CheckpointState `json:"checkpoints"`
		}{
			Checkpoints: []models.CheckpointState{
				{
					CheckpointID: "cp1",
					Status:       models.CheckpointStatusPassed,
					Points:       50,
					EarnedPoints: 50,
					AttemptCount: 1,
					PassedAt:     &now,
				},
				{
					CheckpointID: "cp2",
					Status:       models.CheckpointStatusPending,
					Points:       50,
					EarnedPoints: 0,
					AttemptCount: 0,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.Checkpoints(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionCommands_Checkpoints_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			Checkpoints []models.CheckpointState `json:"checkpoints"`
		}{
			Checkpoints: []models.CheckpointState{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSessionCommands(client)

	err := cmds.Checkpoints(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Error Handling Tests
// -----------------------------------------------------------------------------

func TestClient_Request_MarshalError(t *testing.T) {
	client := NewClient(Config{APIBaseURL: "http://localhost"})

	// Create an unmarshalable value (function cannot be JSON marshaled)
	err := client.Post(context.Background(), "/test", func() {}, nil)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
}

func TestClient_Request_ErrorResponse_NonJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})

	var result map[string]string
	err := client.Get(context.Background(), "/error", &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Error should contain the raw response body
	if err.Error() != "API error (500): Internal Server Error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// LabCommands Tests
// -----------------------------------------------------------------------------

func TestNewLabCommands(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, "/tmp/templates")

	if cmds.client != client {
		t.Error("expected client to be set")
	}
	if cmds.templatesDir != "/tmp/templates" {
		t.Errorf("expected templatesDir '/tmp/templates', got %q", cmds.templatesDir)
	}
}

func TestLabCommands_List_EmptyDir(t *testing.T) {
	// Create a temp directory with no templates
	dir := t.TempDir()

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLabCommands_List_WithTemplates(t *testing.T) {
	// Create a temp directory with a template file
	dir := t.TempDir()

	template := `apiVersion: v1
kind: LabTemplate
metadata:
  name: test-lab
  description: A test lab
  difficulty: beginner
  duration: 60m
spec:
  platform: proxmox
  vms:
    - name: server
      template: ubuntu-22.04
  network:
    segments:
      - name: internal
        subnet: 10.0.0.0/24
  objectives:
    - id: obj-1
      description: Test objective
      points: 10
`
	err := os.WriteFile(filepath.Join(dir, "test-lab.yaml"), []byte(template), 0644)
	if err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err = cmds.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLabCommands_Show_Success(t *testing.T) {
	dir := t.TempDir()

	template := `apiVersion: v1
kind: LabTemplate
metadata:
  name: test-lab
  description: A test lab for unit testing
  difficulty: beginner
  duration: 60m
  version: "1.0.0"
  author: Test Author
  tags:
    - testing
    - example
spec:
  platform: proxmox
  vms:
    - name: server
      template: ubuntu-22.04
      resources:
        cpu: 2
        memory: 4096
  network:
    segments:
      - name: internal
        subnet: 10.0.0.0/24
        gateway: 10.0.0.1
        dhcp: true
  objectives:
    - id: obj-1
      description: First objective
      points: 10
      hint: Try running the command
`
	err := os.WriteFile(filepath.Join(dir, "test-lab.yaml"), []byte(template), 0644)
	if err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err = cmds.Show(context.Background(), "test-lab")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLabCommands_Show_NotFound(t *testing.T) {
	dir := t.TempDir()

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.Show(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent lab")
	}
}

func TestLabCommands_Validate_Success(t *testing.T) {
	dir := t.TempDir()

	// Create a valid template file
	templateContent := `metadata:
  name: valid-lab
  description: A valid lab template
spec:
  platform: proxmox
  vms:
    - name: server
      template: ubuntu-22.04
      resources:
        cpu: 2
        memory: 4096
      networks:
        - segment: internal
  network:
    segments:
      - name: internal
        vlan: 10
        cidr: 10.0.0.0/24
  objectives:
    - id: obj1
      title: First objective
      points: 10
      triggers:
        - type: file_exists
          target: server
          params:
            path: /tmp/test
`
	templatePath := filepath.Join(dir, "valid.yaml")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.Validate(context.Background(), templatePath)
	if err != nil {
		t.Fatalf("expected no error for valid template, got: %v", err)
	}
}

func TestLabCommands_Validate_InvalidYAML(t *testing.T) {
	dir := t.TempDir()

	// Create invalid YAML file
	templatePath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(templatePath, []byte("not: valid: yaml:"), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.Validate(context.Background(), templatePath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLabCommands_Validate_FileNotFound(t *testing.T) {
	dir := t.TempDir()

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.Validate(context.Background(), "/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLabCommands_Validate_MissingName(t *testing.T) {
	dir := t.TempDir()

	templateContent := `metadata:
  description: Missing name
spec:
  platform: proxmox
  vms:
    - name: server
      template: ubuntu-22.04
      resources:
        cpu: 2
        memory: 4096
`
	templatePath := filepath.Join(dir, "missing-name.yaml")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.Validate(context.Background(), templatePath)
	if err == nil {
		t.Fatal("expected validation error for missing name")
	}
}

func TestLabCommands_Validate_NoVMs(t *testing.T) {
	dir := t.TempDir()

	templateContent := `metadata:
  name: no-vms
spec:
  platform: proxmox
  vms: []
`
	templatePath := filepath.Join(dir, "no-vms.yaml")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, dir)

	err := cmds.Validate(context.Background(), templatePath)
	if err == nil {
		t.Fatal("expected validation error for no VMs")
	}
}

func TestLabCommands_ValidateTemplate_DuplicateVMNames(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "duplicate VM name") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected duplicate VM name error")
	}
}

func TestLabCommands_ValidateTemplate_VMMissingName(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "name is required") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected VM name required error")
	}
}

func TestLabCommands_ValidateTemplate_VMInvalidResources(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 0, Memory: 0}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	cpuError := false
	memError := false
	for _, e := range errors {
		if strings.Contains(e, "cpu must be positive") {
			cpuError = true
		}
		if strings.Contains(e, "memory must be positive") {
			memError = true
		}
	}
	if !cpuError {
		t.Error("expected CPU must be positive error")
	}
	if !memError {
		t.Error("expected memory must be positive error")
	}
}

func TestLabCommands_ValidateTemplate_DuplicateNetworkSegment(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal"},
					{Name: "internal"},
				},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "duplicate network segment") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected duplicate network segment error")
	}
}

func TestLabCommands_ValidateTemplate_UnknownNetworkSegment(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{
					Name:      "server",
					Template:  "ubuntu",
					Resources: models.ResourceSpec{CPU: 1, Memory: 1024},
					Networks:  []models.VMNetworkSpec{{Segment: "nonexistent"}},
				},
			},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal"},
				},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "unknown network segment") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected unknown network segment error")
	}
}

func TestLabCommands_ValidateTemplate_DuplicateCheckpointID(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Points: 10, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}},
				{ID: "obj1", Points: 10, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "duplicate checkpoint ID") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected duplicate checkpoint ID error")
	}
}

func TestLabCommands_ValidateTemplate_NegativePoints(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Points: -10, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "points cannot be negative") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected points cannot be negative error")
	}
}

func TestLabCommands_ValidateTemplate_UnknownDependency(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Points: 10, DependsOn: []string{"nonexistent"}, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "depends on unknown checkpoint") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected depends on unknown checkpoint error")
	}
}

func TestLabCommands_ValidateTemplate_TriggerMissingType(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Points: 10, Triggers: []models.CheckpointTrigger{{Target: "server"}}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "type is required") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected trigger type required error")
	}
}

func TestLabCommands_ValidateTemplate_TriggerUnknownVM(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Points: 10, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "nonexistent"}}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "references unknown VM") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected references unknown VM error")
	}
}

func TestLabCommands_ValidateTemplate_ValidForwardDependency(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	// Forward dependency (obj1 depends on obj2 which is defined later)
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Points: 10, DependsOn: []string{"obj2"}, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}},
				{ID: "obj2", Points: 10, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	for _, e := range errors {
		if strings.Contains(e, "depends on unknown checkpoint") {
			t.Errorf("unexpected error for forward dependency: %s", e)
		}
	}
}

func TestLabCommands_ValidateTemplate_MissingNetworkSegmentName(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{VLAN: 10, Subnet: "10.0.0.0/24"}, // Missing name
				},
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "network segment name is required") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected network segment name required error")
	}
}

func TestLabCommands_ValidateTemplate_MissingVMTemplate(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}}, // Missing template
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "template is required") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected VM template required error")
	}
}

func TestLabCommands_ValidateTemplate_MissingCheckpointID(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewLabCommands(client, t.TempDir())

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test"},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu", Resources: models.ResourceSpec{CPU: 1, Memory: 1024}},
			},
			Objectives: []models.Checkpoint{
				{Points: 10, Triggers: []models.CheckpointTrigger{{Type: "file_exists", Target: "server"}}}, // Missing ID
			},
		},
	}

	errors := cmds.validateTemplate(template)
	found := false
	for _, e := range errors {
		if strings.Contains(e, "id is required") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected checkpoint ID required error")
	}
}

// -----------------------------------------------------------------------------
// SnapshotCommands Tests
// -----------------------------------------------------------------------------

func TestNewSnapshotCommands(t *testing.T) {
	client := NewClient(DefaultConfig())
	cmds := NewSnapshotCommands(client)

	if cmds.client != client {
		t.Error("expected client to be set")
	}
}

func TestSnapshotCommands_List_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pods/pod-123/vms/vm1/snapshots" {
			t.Errorf("expected path /api/v1/pods/pod-123/vms/vm1/snapshots, got %s", r.URL.Path)
		}

		response := SnapshotListResponse{
			PodID:  "pod-123",
			VMName: "vm1",
			Snapshots: []SnapshotInfo{
				{Name: "clean", Description: "Initial clean state"},
				{Name: "configured", Description: "After configuration", Parent: "clean"},
			},
			Count: 2,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSnapshotCommands(client)

	err := cmds.List(context.Background(), "pod-123", "vm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSnapshotCommands_List_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SnapshotListResponse{
			PodID:     "pod-123",
			VMName:    "vm1",
			Snapshots: []SnapshotInfo{},
			Count:     0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSnapshotCommands(client)

	err := cmds.List(context.Background(), "pod-123", "vm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSnapshotCommands_Create_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["name"] != "my-snapshot" {
			t.Errorf("expected name 'my-snapshot', got %v", body["name"])
		}
		if body["description"] != "My test snapshot" {
			t.Errorf("expected description 'My test snapshot', got %v", body["description"])
		}

		response := map[string]string{
			"status":   "creating",
			"podId":    "pod-123",
			"vmName":   "vm1",
			"snapshot": "my-snapshot",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSnapshotCommands(client)

	err := cmds.Create(context.Background(), "pod-123", "vm1", "my-snapshot", "My test snapshot", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSnapshotCommands_Revert_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := map[string]string{
			"status":   "resetting",
			"vmName":   "vm1",
			"snapshot": "clean",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSnapshotCommands(client)

	err := cmds.Revert(context.Background(), "pod-123", "vm1", "clean")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSnapshotCommands_Delete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		response := map[string]string{
			"status":   "deleted",
			"podId":    "pod-123",
			"vmName":   "vm1",
			"snapshot": "old-snapshot",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewSnapshotCommands(client)

	err := cmds.Delete(context.Background(), "pod-123", "vm1", "old-snapshot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// PodCommands Additional Tests
// -----------------------------------------------------------------------------

func TestPodCommands_ResetVM_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/api/v1/pods/pod-123/vms/vm1/reset" {
			t.Errorf("expected path /api/v1/pods/pod-123/vms/vm1/reset, got %s", r.URL.Path)
		}

		response := map[string]string{"status": "resetting"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.ResetVM(context.Background(), "pod-123", "vm1", "clean")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodCommands_ResetVM_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "VM not found"})
	}))
	defer server.Close()

	client := NewClient(Config{APIBaseURL: server.URL})
	cmds := NewPodCommands(client)

	err := cmds.ResetVM(context.Background(), "pod-123", "nonexistent", "clean")
	if err == nil {
		t.Fatal("expected error for nonexistent VM")
	}
}
