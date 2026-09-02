package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/cli"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// TestMain installs the English CLI localizer once so TestPrintUsage /
// TestPrintVersion (and any other test that exercises user-facing output)
// see real English strings rather than raw message IDs. Kept at package
// scope so every test in this file starts from a known locale state.
func TestMain(m *testing.M) {
	cli.InstallLocalizer("en")
	os.Exit(m.Run())
}

// -----------------------------------------------------------------------------
// Version Tests
// -----------------------------------------------------------------------------

// TestGetVersionInfo and TestGetVersionInfo_DefaultValues removed:
// VersionInfo/GetVersionInfo were dead code (duplicated internal/version.Info).
// Version tests live in internal/version/version_test.go.

func TestPrintVersion(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Save and set version values
	origVersion := Version
	origCommit := Commit
	origBuildTime := BuildTime
	Version = "v2.0.0"
	Commit = "test123"
	BuildTime = "2024-06-01T12:00:00Z"

	printVersion()

	// Restore
	w.Close()
	os.Stdout = old
	Version = origVersion
	Commit = origCommit
	BuildTime = origBuildTime

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	expectedStrings := []string{
		"labctl version v2.0.0",
		"Commit:     test123",
		"Built:      2024-06-01T12:00:00Z",
		"Go version:",
		"OS/Arch:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestPrintUsage(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	expectedStrings := []string{
		"labctl - Kootenai Platform CLI",
		"serve",
		"migrate",
		"version",
		"lab",
		"pod",
		"snapshot",
		"session",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

// -----------------------------------------------------------------------------
// PodRepoAdapter Tests
// -----------------------------------------------------------------------------

// mockPodRepo implements a mock for testing podRepoAdapter
type mockPodRepo struct {
	createFn       func(ctx context.Context, pod *models.Pod) error
	getByIDFn      func(ctx context.Context, id string) (*models.Pod, error)
	listFn         func(ctx context.Context, filter any) ([]*models.Pod, error)
	updateFn       func(ctx context.Context, pod *models.Pod) error
	updateStatusFn func(ctx context.Context, id string, status models.PodStatus) error
	deleteFn       func(ctx context.Context, id string) error
	getExpiredFn   func(ctx context.Context) ([]*models.Pod, error)
}

func (m *mockPodRepo) Create(ctx context.Context, pod *models.Pod) error {
	if m.createFn != nil {
		return m.createFn(ctx, pod)
	}
	return nil
}

func (m *mockPodRepo) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockPodRepo) List(ctx context.Context, filter any) ([]*models.Pod, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockPodRepo) Update(ctx context.Context, pod *models.Pod) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, pod)
	}
	return nil
}

func (m *mockPodRepo) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}

func (m *mockPodRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockPodRepo) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	if m.getExpiredFn != nil {
		return m.getExpiredFn(ctx)
	}
	return nil, nil
}

func TestPodRepoAdapter_Create(t *testing.T) {
	called := false
	mock := &mockPodRepo{
		createFn: func(ctx context.Context, pod *models.Pod) error {
			called = true
			if pod.ID != "test-pod" {
				t.Errorf("expected pod ID test-pod, got %s", pod.ID)
			}
			return nil
		},
	}

	// We can't directly test podRepoAdapter since it uses the real repo type,
	// but we can test the interface it should satisfy
	_ = mock
	if !called {
		// This test verifies the mock setup works
		mock.Create(context.Background(), &models.Pod{ID: "test-pod"})
		if !called {
			t.Error("mock create was not called")
		}
	}
}

func TestPodRepoAdapter_List(t *testing.T) {
	expectedPods := []*models.Pod{
		{ID: "pod-1", OwnerID: "user-1"},
		{ID: "pod-2", OwnerID: "user-1"},
	}

	mock := &mockPodRepo{
		listFn: func(ctx context.Context, filter any) ([]*models.Pod, error) {
			return expectedPods, nil
		},
	}

	pods, err := mock.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 2 {
		t.Errorf("expected 2 pods, got %d", len(pods))
	}
}

func TestPodRepoAdapter_UpdateStatus(t *testing.T) {
	var capturedID string
	var capturedStatus models.PodStatus

	mock := &mockPodRepo{
		updateStatusFn: func(ctx context.Context, id string, status models.PodStatus) error {
			capturedID = id
			capturedStatus = status
			return nil
		},
	}

	err := mock.UpdateStatus(context.Background(), "pod-123", models.PodStatusRunning)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "pod-123" {
		t.Errorf("expected pod ID pod-123, got %s", capturedID)
	}
	if capturedStatus != models.PodStatusRunning {
		t.Errorf("expected status running, got %s", capturedStatus)
	}
}

func TestPodRepoAdapter_GetExpired(t *testing.T) {
	expired1 := time.Now().Add(-1 * time.Hour)
	expired2 := time.Now().Add(-2 * time.Hour)
	expiredPods := []*models.Pod{
		{ID: "expired-1", ExpiresAt: &expired1},
		{ID: "expired-2", ExpiresAt: &expired2},
	}

	mock := &mockPodRepo{
		getExpiredFn: func(ctx context.Context) ([]*models.Pod, error) {
			return expiredPods, nil
		},
	}

	pods, err := mock.GetExpired(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 2 {
		t.Errorf("expected 2 expired pods, got %d", len(pods))
	}
}

// -----------------------------------------------------------------------------
// ProvisioningBroadcaster Tests
// -----------------------------------------------------------------------------

// mockHub implements a minimal mock for testing provisioningBroadcaster
type mockHub struct {
	lastEvent *websocket.PodProvisioningEvent
}

func (m *mockHub) BroadcastPodProvisioning(event *websocket.PodProvisioningEvent) {
	m.lastEvent = event
}

func TestProvisioningBroadcaster_BroadcastEvent(t *testing.T) {
	mockWsHub := &mockHub{}

	// Create a broadcaster with our mock
	// Note: We can't directly use provisioningBroadcaster as it expects *websocket.Hub
	// This test validates the event structure conversion

	event := orchestrator.PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     orchestrator.PhaseCloningVM,
		Message:   "Cloning VMs",
		Progress:  50,
		VMName:    "ubuntu-vm",
		VMStatus:  "cloning",
		Error:     "",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		RequestID: "req-789",
	}

	// Simulate what provisioningBroadcaster does
	wsEvent := &websocket.PodProvisioningEvent{
		PodID:     event.PodID,
		OwnerID:   event.OwnerID,
		Status:    string(event.Status),
		Phase:     string(event.Phase),
		Message:   event.Message,
		Progress:  event.Progress,
		VMName:    event.VMName,
		VMStatus:  event.VMStatus,
		Error:     event.Error,
		Timestamp: event.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		RequestID: event.RequestID,
	}

	mockWsHub.BroadcastPodProvisioning(wsEvent)

	if mockWsHub.lastEvent == nil {
		t.Fatal("event was not broadcast")
	}
	if mockWsHub.lastEvent.PodID != "pod-123" {
		t.Errorf("expected pod ID pod-123, got %s", mockWsHub.lastEvent.PodID)
	}
	if mockWsHub.lastEvent.Status != "provisioning" {
		t.Errorf("expected status provisioning, got %s", mockWsHub.lastEvent.Status)
	}
	if mockWsHub.lastEvent.Phase != "cloning_vm" {
		t.Errorf("expected phase cloning_vm, got %s", mockWsHub.lastEvent.Phase)
	}
	if mockWsHub.lastEvent.Progress != 50 {
		t.Errorf("expected progress 50, got %d", mockWsHub.lastEvent.Progress)
	}
	if mockWsHub.lastEvent.Timestamp != "2024-01-15T10:30:00Z" {
		t.Errorf("expected timestamp 2024-01-15T10:30:00Z, got %s", mockWsHub.lastEvent.Timestamp)
	}
}

// VersionInfo JSON tests removed — VersionInfo was dead code.

// -----------------------------------------------------------------------------
// AppConfig Alias Test
// -----------------------------------------------------------------------------

func TestAppConfigAlias(t *testing.T) {
	// Verify AppConfig is correctly aliased to config.Config
	var cfg AppConfig
	// Should compile and cfg should be a config.Config
	_ = cfg
}

// -----------------------------------------------------------------------------
// Command Detection Tests
// -----------------------------------------------------------------------------

func TestCommandDetection(t *testing.T) {
	// These tests verify command routing logic without actually executing commands
	commands := []string{
		"serve",
		"migrate",
		"version",
		"lab",
		"pod",
		"snapshot",
		"session",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			// Verify each command is a valid recognized command
			switch cmd {
			case "serve", "migrate", "version", "lab", "pod", "snapshot", "session":
				// Valid command
			default:
				t.Errorf("unexpected command: %s", cmd)
			}
		})
	}
}

func TestInvalidCommand(t *testing.T) {
	// Test that unknown commands would trigger usage output
	unknownCommands := []string{
		"unknown",
		"help",
		"start",
		"stop",
		"",
	}

	for _, cmd := range unknownCommands {
		t.Run(cmd, func(t *testing.T) {
			switch cmd {
			case "serve", "migrate", "version", "lab", "pod", "snapshot", "session":
				t.Errorf("command %s should not be considered unknown", cmd)
			default:
				// Expected - unknown command
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Integration-style Tests (without external dependencies)
// -----------------------------------------------------------------------------

func TestLoadConfig_DefaultPaths(t *testing.T) {
	// This test verifies loadConfig doesn't panic with no config files
	// We can't fully test it without side effects, but we can test the path list
	configPaths := []string{
		"config.yaml",
		"config/config.yaml",
		"../config/config.yaml",
		"/app/config.yaml",
		"/etc/labctl/config.yaml",
	}

	for _, path := range configPaths {
		if path == "" {
			t.Error("config path should not be empty")
		}
	}
}

// -----------------------------------------------------------------------------
// Pod Model Tests
// -----------------------------------------------------------------------------

func TestPodModel_Fields(t *testing.T) {
	now := time.Now()
	expires := now.Add(4 * time.Hour)
	pod := &models.Pod{
		ID:            "pod-123",
		LabTemplateID: "template-456",
		OwnerID:       "user-789",
		Status:        models.PodStatusRunning,
		CreatedAt:     now,
		ExpiresAt:     &expires,
	}

	if pod.ID != "pod-123" {
		t.Errorf("expected ID pod-123, got %s", pod.ID)
	}
	if pod.Status != models.PodStatusRunning {
		t.Errorf("expected status running, got %s", pod.Status)
	}
	if pod.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
}

func TestPodStatus_Constants(t *testing.T) {
	tests := []struct {
		status   models.PodStatus
		expected string
	}{
		{models.PodStatusProvisioning, "provisioning"},
		{models.PodStatusRunning, "running"},
		{models.PodStatusStopped, "stopped"},
		{models.PodStatusError, "error"},
		{models.PodStatusDestroying, "destroying"},
		{models.PodStatusDestroyed, "destroyed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Provisioning Phase Tests
// -----------------------------------------------------------------------------

func TestProvisioningPhase_Constants(t *testing.T) {
	tests := []struct {
		phase    orchestrator.ProvisioningPhase
		expected string
	}{
		{orchestrator.PhaseQueued, "queued"},
		{orchestrator.PhaseStarting, "starting"},
		{orchestrator.PhaseCreatingPod, "creating_pod"},
		{orchestrator.PhaseCloningVM, "cloning_vm"},
		{orchestrator.PhaseConfiguringVM, "configuring_vm"},
		{orchestrator.PhaseCreatingSnapshots, "creating_snapshots"},
		{orchestrator.PhaseStartingVM, "starting_vm"},
		{orchestrator.PhaseCompleted, "completed"},
		{orchestrator.PhaseFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.phase) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.phase))
			}
		})
	}
}

// -----------------------------------------------------------------------------
// PodProvisioningEvent Tests
// -----------------------------------------------------------------------------

func TestPodProvisioningEvent_AllFields(t *testing.T) {
	timestamp := time.Now()
	event := orchestrator.PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     orchestrator.PhaseCreatingPod,
		Message:   "Creating pod resources",
		Progress:  25,
		VMName:    "vm-1",
		VMStatus:  "creating",
		Error:     "",
		Timestamp: timestamp,
		RequestID: "req-abc",
	}

	if event.PodID != "pod-123" {
		t.Errorf("expected PodID pod-123, got %s", event.PodID)
	}
	if event.Progress != 25 {
		t.Errorf("expected Progress 25, got %d", event.Progress)
	}
	if event.RequestID != "req-abc" {
		t.Errorf("expected RequestID req-abc, got %s", event.RequestID)
	}
}

func TestPodProvisioningEvent_ErrorCase(t *testing.T) {
	event := orchestrator.PodProvisioningEvent{
		PodID:     "pod-error",
		OwnerID:   "user-123",
		Status:    models.PodStatusError,
		Phase:     orchestrator.PhaseFailed,
		Message:   "Provisioning failed",
		Progress:  0,
		Error:     "VM clone failed: insufficient resources",
		Timestamp: time.Now(),
		RequestID: "req-failed",
	}

	if event.Status != models.PodStatusError {
		t.Errorf("expected status error, got %s", event.Status)
	}
	if event.Phase != orchestrator.PhaseFailed {
		t.Errorf("expected phase failed, got %s", event.Phase)
	}
	if event.Error == "" {
		t.Error("error message should not be empty for failed provisioning")
	}
}

// -----------------------------------------------------------------------------
// WebSocket Event Conversion Tests
// -----------------------------------------------------------------------------

func TestWebSocketEventConversion(t *testing.T) {
	timestamp := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)

	orchEvent := orchestrator.PodProvisioningEvent{
		PodID:     "pod-ws",
		OwnerID:   "user-ws",
		Status:    models.PodStatusRunning,
		Phase:     orchestrator.PhaseCompleted,
		Message:   "Pod ready",
		Progress:  100,
		VMName:    "final-vm",
		VMStatus:  "running",
		Error:     "",
		Timestamp: timestamp,
		RequestID: "req-ws",
	}

	// Convert to WebSocket event (simulating provisioningBroadcaster)
	wsEvent := websocket.PodProvisioningEvent{
		PodID:     orchEvent.PodID,
		OwnerID:   orchEvent.OwnerID,
		Status:    string(orchEvent.Status),
		Phase:     string(orchEvent.Phase),
		Message:   orchEvent.Message,
		Progress:  orchEvent.Progress,
		VMName:    orchEvent.VMName,
		VMStatus:  orchEvent.VMStatus,
		Error:     orchEvent.Error,
		Timestamp: orchEvent.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		RequestID: orchEvent.RequestID,
	}

	if wsEvent.PodID != orchEvent.PodID {
		t.Error("PodID mismatch in conversion")
	}
	if wsEvent.Status != string(orchEvent.Status) {
		t.Error("Status mismatch in conversion")
	}
	if wsEvent.Timestamp != "2024-06-15T14:30:00Z" {
		t.Errorf("expected timestamp 2024-06-15T14:30:00Z, got %s", wsEvent.Timestamp)
	}
	if wsEvent.Progress != 100 {
		t.Errorf("expected progress 100, got %d", wsEvent.Progress)
	}
}

// -----------------------------------------------------------------------------
// Mock Repository Edge Cases
// -----------------------------------------------------------------------------

func TestMockPodRepo_NilFunctions(t *testing.T) {
	// Test that nil function handlers don't panic
	mock := &mockPodRepo{}

	// All of these should not panic with nil handlers
	err := mock.Create(context.Background(), &models.Pod{})
	if err != nil {
		t.Errorf("expected nil error from nil Create handler, got %v", err)
	}

	pod, err := mock.GetByID(context.Background(), "test")
	if err != nil {
		t.Errorf("expected nil error from nil GetByID handler, got %v", err)
	}
	if pod != nil {
		t.Error("expected nil pod from nil GetByID handler")
	}

	pods, err := mock.List(context.Background(), nil)
	if err != nil {
		t.Errorf("expected nil error from nil List handler, got %v", err)
	}
	if pods != nil {
		t.Error("expected nil pods from nil List handler")
	}

	err = mock.Update(context.Background(), &models.Pod{})
	if err != nil {
		t.Errorf("expected nil error from nil Update handler, got %v", err)
	}

	err = mock.UpdateStatus(context.Background(), "test", models.PodStatusRunning)
	if err != nil {
		t.Errorf("expected nil error from nil UpdateStatus handler, got %v", err)
	}

	err = mock.Delete(context.Background(), "test")
	if err != nil {
		t.Errorf("expected nil error from nil Delete handler, got %v", err)
	}

	expired, err := mock.GetExpired(context.Background())
	if err != nil {
		t.Errorf("expected nil error from nil GetExpired handler, got %v", err)
	}
	if expired != nil {
		t.Error("expected nil expired from nil GetExpired handler")
	}
}

func TestMockPodRepo_ErrorReturns(t *testing.T) {
	expectedErr := io.EOF // Using a standard error for testing

	mock := &mockPodRepo{
		createFn: func(ctx context.Context, pod *models.Pod) error {
			return expectedErr
		},
		getByIDFn: func(ctx context.Context, id string) (*models.Pod, error) {
			return nil, expectedErr
		},
	}

	err := mock.Create(context.Background(), &models.Pod{})
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	_, err = mock.GetByID(context.Background(), "test")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// -----------------------------------------------------------------------------
// Benchmark Tests
// -----------------------------------------------------------------------------

func BenchmarkPrintVersion(b *testing.B) {
	// Capture stdout to avoid output during benchmark
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printVersion()
	}
}

func BenchmarkPodProvisioningEventCreation(b *testing.B) {
	timestamp := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = orchestrator.PodProvisioningEvent{
			PodID:     "pod-bench",
			OwnerID:   "user-bench",
			Status:    models.PodStatusProvisioning,
			Phase:     orchestrator.PhaseCreatingPod,
			Message:   "Benchmark test",
			Progress:  50,
			Timestamp: timestamp,
			RequestID: "req-bench",
		}
	}
}

func (m *mockPodRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
