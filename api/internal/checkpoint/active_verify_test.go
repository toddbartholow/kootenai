package checkpoint

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
)

// mockOrchestrator implements orchestrator.Client for testing
type mockOrchestrator struct {
	executeCommandFunc func(ctx context.Context, podID, vmName, command string) (string, error)
}

func (m *mockOrchestrator) ExecuteCommand(ctx context.Context, podID, vmName, command string) (string, error) {
	if m.executeCommandFunc != nil {
		return m.executeCommandFunc(ctx, podID, vmName, command)
	}
	return "", nil
}

// Stub implementations for other interface methods (not used in these tests)
func (m *mockOrchestrator) CreatePod(ctx context.Context, template *models.LabTemplate, templateID, ownerID, ownerName string, _ ...orchestrator.CreatePodOpts) (*models.Pod, error) {
	return nil, nil
}
func (m *mockOrchestrator) GetPod(ctx context.Context, podID string) (*models.Pod, error) {
	return nil, nil
}
func (m *mockOrchestrator) ListPods(ctx context.Context, owner string) ([]*models.Pod, error) {
	return nil, nil
}
func (m *mockOrchestrator) ListPodsWithFilter(ctx context.Context, filter orchestrator.PodFilter) ([]*models.Pod, error) {
	return nil, nil
}
func (m *mockOrchestrator) DestroyPod(ctx context.Context, podID string) error { return nil }
func (m *mockOrchestrator) StartPod(ctx context.Context, podID string) (*models.Pod, error) {
	return nil, nil
}
func (m *mockOrchestrator) StopPod(ctx context.Context, podID string) (*models.Pod, error) {
	return nil, nil
}
func (m *mockOrchestrator) StartVM(ctx context.Context, podID, vmName string) error   { return nil }
func (m *mockOrchestrator) StopVM(ctx context.Context, podID, vmName string) error    { return nil }
func (m *mockOrchestrator) SuspendVM(ctx context.Context, podID, vmName string) error { return nil }
func (m *mockOrchestrator) ResumeVM(ctx context.Context, podID, vmName string) error  { return nil }
func (m *mockOrchestrator) ResetPodVM(ctx context.Context, podID, vmName, snapshotName string) error {
	return nil
}
func (m *mockOrchestrator) CreateVMSnapshot(ctx context.Context, podID, vmName, snapshotName, description string, includeRAM bool) error {
	return nil
}
func (m *mockOrchestrator) DeleteVMSnapshot(ctx context.Context, podID, vmName, snapshotName string) error {
	return nil
}
func (m *mockOrchestrator) ListVMSnapshots(ctx context.Context, podID, vmName string) ([]orchestrator.VMSnapshot, error) {
	return nil, nil
}
func (m *mockOrchestrator) GetVMConsole(ctx context.Context, podID, vmName, consoleType string) (*orchestrator.ConsoleTicket, error) {
	return nil, nil
}

func TestDefaultActiveVerifyConfig(t *testing.T) {
	config := DefaultActiveVerifyConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", config.Timeout)
	}
	if config.RetryCount != 2 {
		t.Errorf("expected RetryCount=2, got %d", config.RetryCount)
	}
	if config.RetryDelay != time.Second {
		t.Errorf("expected RetryDelay=1s, got %v", config.RetryDelay)
	}
	if config.ParallelChecks != 5 {
		t.Errorf("expected ParallelChecks=5, got %d", config.ParallelChecks)
	}
}

func TestNewActiveVerifier(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()
	mock := &mockOrchestrator{}

	verifier := NewActiveVerifier(mock, logger, config)
	if verifier == nil {
		t.Fatal("expected non-nil ActiveVerifier")
	}
}

func TestActiveVerifier_VerifyFileExists(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		commandOutput  string
		expectedResult bool
	}{
		{
			name:           "file exists",
			commandOutput:  "EXISTS\n",
			expectedResult: true,
		},
		{
			name:           "file not found",
			commandOutput:  "NOTFOUND\n",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					return tt.commandOutput, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypeFileExists,
						Target: "test-vm",
						Match: models.TriggerMatch{
							Path: "/etc/test.conf",
						},
					},
				},
			}

			result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.expectedResult {
				t.Errorf("expected Passed=%v, got %v", tt.expectedResult, result.Passed)
			}
			if result.Method != "file_exists" {
				t.Errorf("expected Method=file_exists, got %s", result.Method)
			}
		})
	}
}

func TestActiveVerifier_VerifyFileContent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		contains       string
		regex          string
		fileContent    string
		expectedResult bool
	}{
		{
			name:           "contains match",
			contains:       "Hello World",
			fileContent:    "This is Hello World content",
			expectedResult: true,
		},
		{
			name:           "contains no match",
			contains:       "Goodbye",
			fileContent:    "This is Hello World content",
			expectedResult: false,
		},
		{
			name:           "regex match",
			regex:          `port\s*=\s*\d+`,
			fileContent:    "port = 8080",
			expectedResult: true,
		},
		{
			name:           "regex no match",
			regex:          `port\s*=\s*\d+`,
			fileContent:    "hostname = localhost",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					callCount++
					if callCount == 1 {
						// First call is file exists check
						return "EXISTS\n", nil
					}
					// Second call is cat file
					return tt.fileContent, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypeFileContent,
						Target: "test-vm",
						Match: models.TriggerMatch{
							Path:     "/etc/config",
							Contains: tt.contains,
							Regex:    tt.regex,
						},
					},
				},
			}

			result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.expectedResult {
				t.Errorf("expected Passed=%v, got %v", tt.expectedResult, result.Passed)
			}
		})
	}
}

func TestActiveVerifier_VerifyServiceState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		serviceOutput  string
		expectedState  string
		expectedResult bool
	}{
		{
			name:           "service active",
			serviceOutput:  "active\n",
			expectedState:  "active",
			expectedResult: true,
		},
		{
			name:           "service running (maps to active)",
			serviceOutput:  "active\n",
			expectedState:  "running",
			expectedResult: true,
		},
		{
			name:           "service inactive",
			serviceOutput:  "inactive\n",
			expectedState:  "stopped",
			expectedResult: true,
		},
		{
			name:           "service state mismatch",
			serviceOutput:  "inactive\n",
			expectedState:  "active",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					return tt.serviceOutput, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypeService,
						Target: "test-vm",
						Match: models.TriggerMatch{
							Name:  "nginx",
							State: tt.expectedState,
						},
					},
				},
			}

			result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.expectedResult {
				t.Errorf("expected Passed=%v, got %v", tt.expectedResult, result.Passed)
			}
		})
	}
}

func TestActiveVerifier_VerifyPackageInstalled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		commandOutput  string
		expectedResult bool
	}{
		{
			name:           "package installed",
			commandOutput:  "INSTALLED\n",
			expectedResult: true,
		},
		{
			name:           "package not found",
			commandOutput:  "NOTFOUND\n",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					return tt.commandOutput, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypePackage,
						Target: "test-vm",
						Match: models.TriggerMatch{
							Package: "vim",
						},
					},
				},
			}

			result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.expectedResult {
				t.Errorf("expected Passed=%v, got %v", tt.expectedResult, result.Passed)
			}
		})
	}
}

func TestActiveVerifier_VerifyUserExists(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		commandOutput  string
		expectedResult bool
	}{
		{
			name:           "user exists",
			commandOutput:  "EXISTS\n",
			expectedResult: true,
		},
		{
			name:           "user not found",
			commandOutput:  "NOTFOUND\n",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					return tt.commandOutput, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypeUserCreated,
						Target: "test-vm",
						Match: models.TriggerMatch{
							User: "testuser",
						},
					},
				},
			}

			result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.expectedResult {
				t.Errorf("expected Passed=%v, got %v", tt.expectedResult, result.Passed)
			}
		})
	}
}

func TestActiveVerifier_VerifyFilePermissions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		statOutput     string
		expectedMode   string
		expectedOwner  string
		expectedResult bool
	}{
		{
			name:           "correct permissions and owner",
			statOutput:     "644 root\n",
			expectedMode:   "644",
			expectedOwner:  "root",
			expectedResult: true,
		},
		{
			name:           "wrong mode",
			statOutput:     "755 root\n",
			expectedMode:   "644",
			expectedOwner:  "root",
			expectedResult: false,
		},
		{
			name:           "wrong owner",
			statOutput:     "644 nobody\n",
			expectedMode:   "644",
			expectedOwner:  "root",
			expectedResult: false,
		},
		{
			name:           "mode only check",
			statOutput:     "600 anyuser\n",
			expectedMode:   "600",
			expectedOwner:  "",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					return tt.statOutput, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypePermission,
						Target: "test-vm",
						Match: models.TriggerMatch{
							Path:  "/etc/shadow",
							Mode:  tt.expectedMode,
							Owner: tt.expectedOwner,
						},
					},
				},
			}

			result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.expectedResult {
				t.Errorf("expected Passed=%v, got %v", tt.expectedResult, result.Passed)
			}
		})
	}
}

func TestActiveVerifier_NoTargetVM(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()
	mock := &mockOrchestrator{}

	verifier := NewActiveVerifier(mock, logger, config)

	// Checkpoint with no target VM
	checkpoint := &models.Checkpoint{
		ID: "test-cp",
		Triggers: []models.CheckpointTrigger{
			{
				Type: models.TriggerTypeFileExists,
				// No Target set
				Match: models.TriggerMatch{
					Path: "/etc/test.conf",
				},
			},
		},
	}

	result, err := verifier.VerifyCheckpoint(context.Background(), "pod-1", checkpoint)
	if err == nil {
		t.Error("expected error for checkpoint with no target VM")
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestActiveVerifier_CrossVerify(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	tests := []struct {
		name           string
		passedByEvent  bool
		activeResult   string
		expectedPassed bool
		expectMismatch bool
	}{
		{
			name:           "both pass",
			passedByEvent:  true,
			activeResult:   "EXISTS\n",
			expectedPassed: true,
			expectMismatch: false,
		},
		{
			name:           "both fail",
			passedByEvent:  false,
			activeResult:   "NOTFOUND\n",
			expectedPassed: false,
			expectMismatch: false,
		},
		{
			name:           "event passed but active failed",
			passedByEvent:  true,
			activeResult:   "NOTFOUND\n",
			expectedPassed: false, // Trust active
			expectMismatch: true,
		},
		{
			name:           "event failed but active passed",
			passedByEvent:  false,
			activeResult:   "EXISTS\n",
			expectedPassed: true, // Trust active
			expectMismatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockOrchestrator{
				executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
					return tt.activeResult, nil
				},
			}

			verifier := NewActiveVerifier(mock, logger, config)

			checkpoint := &models.Checkpoint{
				ID: "test-cp",
				Triggers: []models.CheckpointTrigger{
					{
						Type:   models.TriggerTypeFileExists,
						Target: "test-vm",
						Match: models.TriggerMatch{
							Path: "/etc/test.conf",
						},
					},
				},
			}

			result, passed, err := verifier.CrossVerify(context.Background(), "pod-1", checkpoint, tt.passedByEvent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if passed != tt.expectedPassed {
				t.Errorf("expected passed=%v, got %v", tt.expectedPassed, passed)
			}
			if tt.expectMismatch {
				if result.Details["verification_mismatch"] != true {
					t.Error("expected verification_mismatch in details")
				}
			}
		})
	}
}

func TestActiveVerifier_VerifyAllCheckpoints(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()
	config.ParallelChecks = 2

	mock := &mockOrchestrator{
		executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
			return "EXISTS\n", nil
		},
	}

	verifier := NewActiveVerifier(mock, logger, config)

	checkpoints := []models.Checkpoint{
		{
			ID: "cp-1",
			Triggers: []models.CheckpointTrigger{
				{Type: models.TriggerTypeFileExists, Target: "vm-1", Match: models.TriggerMatch{Path: "/file1"}},
			},
		},
		{
			ID: "cp-2",
			Triggers: []models.CheckpointTrigger{
				{Type: models.TriggerTypeFileExists, Target: "vm-1", Match: models.TriggerMatch{Path: "/file2"}},
			},
		},
		{
			ID: "cp-3",
			Triggers: []models.CheckpointTrigger{
				{Type: models.TriggerTypeFileExists, Target: "vm-1", Match: models.TriggerMatch{Path: "/file3"}},
			},
		},
	}

	results, err := verifier.VerifyAllCheckpoints(context.Background(), "pod-1", checkpoints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// All should pass
	for _, result := range results {
		if !result.Passed {
			t.Errorf("expected all checkpoints to pass, but %s failed", result.CheckpointID)
		}
	}
}

func TestActiveVerifier_GenerateReport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultActiveVerifyConfig()

	callCount := 0
	mock := &mockOrchestrator{
		executeCommandFunc: func(ctx context.Context, podID, vmName, command string) (string, error) {
			callCount++
			if callCount%2 == 0 {
				return "NOTFOUND\n", nil
			}
			return "EXISTS\n", nil
		},
	}

	verifier := NewActiveVerifier(mock, logger, config)

	checkpoints := []models.Checkpoint{
		{
			ID: "cp-1",
			Triggers: []models.CheckpointTrigger{
				{Type: models.TriggerTypeFileExists, Target: "vm-1", Match: models.TriggerMatch{Path: "/file1"}},
			},
		},
		{
			ID: "cp-2",
			Triggers: []models.CheckpointTrigger{
				{Type: models.TriggerTypeFileExists, Target: "vm-1", Match: models.TriggerMatch{Path: "/file2"}},
			},
		},
	}

	passedCheckpoints := map[string]bool{
		"cp-1": true,
		"cp-2": false,
	}

	report, err := verifier.GenerateReport(context.Background(), "session-1", "pod-1", checkpoints, passedCheckpoints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.SessionID != "session-1" {
		t.Errorf("expected SessionID=session-1, got %s", report.SessionID)
	}
	if report.TotalCheckpoints != 2 {
		t.Errorf("expected TotalCheckpoints=2, got %d", report.TotalCheckpoints)
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(report.Results))
	}
}

func TestVerificationReport_ToJSON(t *testing.T) {
	report := &VerificationReport{
		SessionID:        "session-1",
		PodID:            "pod-1",
		Timestamp:        time.Now(),
		TotalCheckpoints: 3,
		PassedCount:      2,
		FailedCount:      1,
		SkippedCount:     0,
		Mismatches:       1,
		Duration:         5 * time.Second,
		Results:          []*VerificationResult{},
	}

	jsonData, err := report.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if parsed["session_id"] != "session-1" {
		t.Error("session_id not found in JSON")
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/etc/passwd", "/etc/passwd"},
		{"/etc/passwd;rm -rf /", "/etc/passwdrm-rf/"},
		{"/path|cat /etc/shadow", "/pathcat/etc/shadow"},
		{"/file`whoami`", "/filewhoami"},
		{"/normal/path/to/file.txt", "/normal/path/to/file.txt"},
		{"$(cat /etc/passwd)", "cat/etc/passwd"},
		{"/path with spaces", "/pathwithspaces"},
		{"/path\x00null", "/pathnull"},
	}

	for _, tt := range tests {
		result := sanitizePath(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeServiceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"nginx", "nginx"},
		{"nginx.service", "nginx.service"},
		{"my-service", "my-service"},
		{"user@1000.service", "user@1000.service"},
		{"service;rm -rf /", "servicerm-rf"},
		{"$(whoami)", "whoami"},
	}

	for _, tt := range tests {
		result := sanitizeServiceName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeServiceName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizePackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"vim", "vim"},
		{"libssl1.1", "libssl1.1"},
		{"g++", "g++"},
		{"package;rm -rf /", "packagerm-rf"},
	}

	for _, tt := range tests {
		result := sanitizePackageName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizePackageName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"testuser", "testuser"},
		{"test_user", "test_user"},
		{"test-user", "test-user"},
		{"user;rm -rf /", "userrm-rf"},
		{"$(whoami)", "whoami"},
	}

	for _, tt := range tests {
		result := sanitizeUsername(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeUsername(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
