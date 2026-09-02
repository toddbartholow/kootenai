package wazuh

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// mockRepositories returns nil repositories for testing (service gracefully handles nil repos for agent operations)
func newTestService(apiClient *APIClient) *Service {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	return &Service{
		config: Config{
			AgentGroupPrefix:  "test-",
			EventBufferSize:   100,
			ProcessingWorkers: 1,
		},
		logger:          logger,
		apiClient:       apiClient,
		agentPodMapping: make(map[string]string),
		checkpointCache: make(map[string][]CheckpointMatcher),
		eventChan:       make(chan *Alert, 100),
		stopChan:        make(chan struct{}),
	}
}

func TestService_RegisterAgent_WithAPIClient(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	apiClient := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	svc := newTestService(apiClient)

	ctx := context.Background()
	reg := &AgentRegistration{
		Name:  "test-vm",
		IP:    "192.168.1.100",
		PodID: "pod-123",
	}

	key, err := svc.RegisterAgent(ctx, reg)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	if key.ID == "" {
		t.Error("expected agent ID to be non-empty")
	}
	if key.Key == "" {
		t.Error("expected agent key to be non-empty")
	}

	// Verify agent-pod mapping was created
	svc.agentPodMappingMu.RLock()
	podID, ok := svc.agentPodMapping[key.ID]
	svc.agentPodMappingMu.RUnlock()

	if !ok {
		t.Error("expected agent-pod mapping to be created")
	}
	if podID != "pod-123" {
		t.Errorf("expected pod ID pod-123, got %s", podID)
	}
}

func TestService_RegisterAgent_WithoutAPIClient(t *testing.T) {
	svc := newTestService(nil) // No API client

	ctx := context.Background()
	reg := &AgentRegistration{
		Name:  "test-vm",
		IP:    "192.168.1.100",
		PodID: "pod-456",
	}

	key, err := svc.RegisterAgent(ctx, reg)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Should return a local agent ID
	if key.ID == "" {
		t.Error("expected agent ID to be non-empty")
	}
	if key.Key != "" {
		t.Error("expected agent key to be empty for local-only agent")
	}

	// Local agent IDs should start with "local-"
	if len(key.ID) < 6 || key.ID[:6] != "local-" {
		t.Errorf("expected local agent ID to start with 'local-', got %s", key.ID)
	}

	// Verify agent-pod mapping was created
	svc.agentPodMappingMu.RLock()
	podID, ok := svc.agentPodMapping[key.ID]
	svc.agentPodMappingMu.RUnlock()

	if !ok {
		t.Error("expected agent-pod mapping to be created")
	}
	if podID != "pod-456" {
		t.Errorf("expected pod ID pod-456, got %s", podID)
	}
}

func TestService_UnregisterAgent_WithAPIClient(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	apiClient := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	svc := newTestService(apiClient)

	// Pre-populate the mapping
	svc.agentPodMappingMu.Lock()
	svc.agentPodMapping["001"] = "pod-123"
	svc.agentPodMappingMu.Unlock()

	ctx := context.Background()
	err := svc.UnregisterAgent(ctx, "001")
	if err != nil {
		t.Fatalf("UnregisterAgent failed: %v", err)
	}

	// Verify mapping was removed
	svc.agentPodMappingMu.RLock()
	_, ok := svc.agentPodMapping["001"]
	svc.agentPodMappingMu.RUnlock()

	if ok {
		t.Error("expected agent-pod mapping to be removed")
	}
}

func TestService_UnregisterAgent_LocalAgent(t *testing.T) {
	svc := newTestService(nil) // No API client

	// Pre-populate with a local agent
	localAgentID := "local-pod-789-test-vm"
	svc.agentPodMappingMu.Lock()
	svc.agentPodMapping[localAgentID] = "pod-789"
	svc.agentPodMappingMu.Unlock()

	ctx := context.Background()
	err := svc.UnregisterAgent(ctx, localAgentID)
	if err != nil {
		t.Fatalf("UnregisterAgent failed: %v", err)
	}

	// Verify mapping was removed
	svc.agentPodMappingMu.RLock()
	_, ok := svc.agentPodMapping[localAgentID]
	svc.agentPodMappingMu.RUnlock()

	if ok {
		t.Error("expected agent-pod mapping to be removed")
	}
}

func TestService_GetAgentStatus(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	apiClient := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	svc := newTestService(apiClient)

	ctx := context.Background()
	info, err := svc.GetAgentStatus(ctx, "001")
	if err != nil {
		t.Fatalf("GetAgentStatus failed: %v", err)
	}

	if info.ID != "001" {
		t.Errorf("expected agent ID 001, got %s", info.ID)
	}
	if info.Status != "active" {
		t.Errorf("expected status active, got %s", info.Status)
	}
}

func TestService_GetAgentStatus_NoAPIClient(t *testing.T) {
	svc := newTestService(nil)

	ctx := context.Background()
	_, err := svc.GetAgentStatus(ctx, "001")
	if err == nil {
		t.Error("expected error when API client is not configured")
	}
}

func TestService_GetAgentStatus_LocalAgent(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	apiClient := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	svc := newTestService(apiClient)

	ctx := context.Background()
	_, err := svc.GetAgentStatus(ctx, "local-pod-123-test")
	if err == nil {
		t.Error("expected error for local-only agent")
	}
}

func TestService_HealthCheck_WithAPIClient(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	apiClient := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	svc := newTestService(apiClient)

	// Add some test data
	svc.agentPodMappingMu.Lock()
	svc.agentPodMapping["001"] = "pod-1"
	svc.agentPodMapping["002"] = "pod-2"
	svc.agentPodMappingMu.Unlock()

	ctx := context.Background()
	status := svc.HealthCheck(ctx)

	if !status.Healthy {
		t.Errorf("expected healthy status, got error: %s", status.Error)
	}
	if !status.ManagerReachable {
		t.Error("expected manager to be reachable")
	}
	if status.ManagerVersion != "v4.8.0" {
		t.Errorf("expected manager version v4.8.0, got %s", status.ManagerVersion)
	}
	if status.AgentCount != 2 {
		t.Errorf("expected 2 agents, got %d", status.AgentCount)
	}
}

func TestService_HealthCheck_WithoutAPIClient(t *testing.T) {
	svc := newTestService(nil)

	ctx := context.Background()
	status := svc.HealthCheck(ctx)

	// Should still be healthy, just without manager connectivity
	if !status.Healthy {
		t.Errorf("expected healthy status even without API client")
	}
	if status.ManagerReachable {
		t.Error("expected manager to not be reachable without API client")
	}
	if status.Error == "" {
		t.Error("expected error message about manager not being configured")
	}
}

func TestService_MapAgentToPod(t *testing.T) {
	svc := newTestService(nil)

	svc.MapAgentToPod("agent-001", "pod-123")

	svc.agentPodMappingMu.RLock()
	podID, ok := svc.agentPodMapping["agent-001"]
	svc.agentPodMappingMu.RUnlock()

	if !ok {
		t.Error("expected mapping to exist")
	}
	if podID != "pod-123" {
		t.Errorf("expected pod ID pod-123, got %s", podID)
	}
}

func TestService_UnmapAgent(t *testing.T) {
	svc := newTestService(nil)

	// Add a mapping
	svc.agentPodMappingMu.Lock()
	svc.agentPodMapping["agent-001"] = "pod-123"
	svc.agentPodMappingMu.Unlock()

	// Remove it
	svc.UnmapAgent("agent-001")

	svc.agentPodMappingMu.RLock()
	_, ok := svc.agentPodMapping["agent-001"]
	svc.agentPodMappingMu.RUnlock()

	if ok {
		t.Error("expected mapping to be removed")
	}
}

func TestService_InvalidateCheckpointCache(t *testing.T) {
	svc := newTestService(nil)

	// Add a cache entry
	svc.checkpointCacheMu.Lock()
	svc.checkpointCache["template-1"] = []CheckpointMatcher{
		{CheckpointID: "cp-1"},
	}
	svc.checkpointCacheMu.Unlock()

	// Invalidate it
	svc.InvalidateCheckpointCache("template-1")

	svc.checkpointCacheMu.RLock()
	_, ok := svc.checkpointCache["template-1"]
	svc.checkpointCacheMu.RUnlock()

	if ok {
		t.Error("expected cache entry to be invalidated")
	}
}

// -----------------------------------------------------------------------------
// Service Lifecycle Tests
// -----------------------------------------------------------------------------

func TestService_StartStop(t *testing.T) {
	svc := newTestService(nil)

	// Start should not panic
	svc.Start()

	// Verify workers are running by checking context is not done
	select {
	case <-svc.stopChan:
		t.Error("expected stop channel to be open")
	default:
		// Expected - channel is open
	}

	// Stop should not panic
	svc.Stop()

	// Verify workers stopped by checking stop channel is closed
	select {
	case <-svc.stopChan:
		// Expected - channel is closed
	default:
		t.Error("expected stop channel to be closed after Stop()")
	}
}

func TestService_ProcessAlert_BufferFull(t *testing.T) {
	svc := newTestService(nil)
	svc.eventChan = make(chan *Alert, 1) // Small buffer for testing

	// Fill the buffer
	alert1 := &Alert{Agent: Agent{ID: "001"}}
	_ = svc.ProcessAlert(context.Background(), alert1)

	// Second alert should still succeed (processed synchronously when buffer full)
	// but will fail due to missing repos - that's expected, we're just testing the flow
	alert2 := &Alert{Agent: Agent{ID: "002"}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = svc.ProcessAlert(ctx, alert2) // Ignore error - we're testing the buffer behavior
}

func TestService_ProcessAlert_ContextCancelled(t *testing.T) {
	svc := newTestService(nil)
	svc.eventChan = make(chan *Alert) // Unbuffered channel

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	alert := &Alert{Agent: Agent{ID: "001"}}
	err := svc.ProcessAlert(ctx, alert)
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}

// -----------------------------------------------------------------------------
// Alert Conversion Tests
// -----------------------------------------------------------------------------

func TestService_AlertToEvent(t *testing.T) {
	svc := newTestService(nil)

	tests := []struct {
		name         string
		alert        *Alert
		expectedType string
	}{
		{
			name: "regular alert",
			alert: &Alert{
				ID:        "alert-123",
				Timestamp: "2024-01-15T10:30:00Z",
				Agent: Agent{
					ID:   "001",
					Name: "test-vm",
				},
				Rule: Rule{
					ID:          "550",
					Description: "Test rule",
					Level:       5,
				},
			},
			expectedType: "wazuh_alert",
		},
		{
			name: "FIM alert - file added",
			alert: &Alert{
				ID:        "alert-456",
				Timestamp: "2024-01-15T10:30:00Z",
				Agent: Agent{
					ID:   "001",
					Name: "test-vm",
				},
				Rule: Rule{
					ID:          "554",
					Description: "File added",
					Level:       5,
				},
				Syscheck: &Syscheck{
					Path:  "/tmp/newfile.txt",
					Event: "added",
				},
			},
			expectedType: "fim_added",
		},
		{
			name: "FIM alert - file deleted",
			alert: &Alert{
				ID:        "alert-789",
				Timestamp: "",
				Agent: Agent{
					ID:   "001",
					Name: "test-vm",
				},
				Rule: Rule{
					ID:          "553",
					Description: "File deleted",
					Level:       7,
				},
				Syscheck: &Syscheck{
					Path:  "/tmp/oldfile.txt",
					Event: "deleted",
				},
			},
			expectedType: "fim_deleted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := svc.alertToEvent(tt.alert, "pod-123", "session-456")
			if err != nil {
				t.Fatalf("alertToEvent failed: %v", err)
			}

			if event.EventType != tt.expectedType {
				t.Errorf("expected event type %s, got %s", tt.expectedType, event.EventType)
			}
			if event.PodID != "pod-123" {
				t.Errorf("expected pod ID pod-123, got %s", event.PodID)
			}
			if event.SessionID != "session-456" {
				t.Errorf("expected session ID session-456, got %s", event.SessionID)
			}
			if event.Source != "wazuh" {
				t.Errorf("expected source wazuh, got %s", event.Source)
			}
			if event.WazuhAlertID != tt.alert.ID {
				t.Errorf("expected wazuh alert ID %s, got %s", tt.alert.ID, event.WazuhAlertID)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Checkpoint Matcher Tests
// -----------------------------------------------------------------------------

func TestService_AlertMatchesMatcher_RuleIDs(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		Rule: Rule{
			ID:    "550",
			Level: 5,
		},
	}

	// Should match when rule ID is in list
	matcher := &CheckpointMatcher{
		RuleIDs: []string{"549", "550", "551"},
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match rule ID")
	}

	// Should not match when rule ID is not in list
	matcher = &CheckpointMatcher{
		RuleIDs: []string{"100", "200"},
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match rule ID")
	}
}

func TestService_AlertMatchesMatcher_RuleGroups(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		Rule: Rule{
			Groups: []string{"syscheck", "fim"},
		},
	}

	// Should match when group is present
	matcher := &CheckpointMatcher{
		RuleGroups: []string{"fim"},
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match rule group")
	}

	// Should not match when group is not present
	matcher = &CheckpointMatcher{
		RuleGroups: []string{"authentication"},
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match rule group")
	}
}

func TestService_AlertMatchesMatcher_MinLevel(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		Rule: Rule{
			Level: 7,
		},
	}

	// Should match when level is >= min
	matcher := &CheckpointMatcher{
		MinLevel: 5,
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match min level")
	}

	// Should not match when level is < min
	matcher = &CheckpointMatcher{
		MinLevel: 10,
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match min level")
	}
}

func TestService_AlertMatchesMatcher_FilePaths(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		Syscheck: &Syscheck{
			Path:  "/etc/passwd",
			Event: "modified",
		},
	}

	// Should match exact path
	matcher := &CheckpointMatcher{
		FilePaths: []string{"/etc/passwd"},
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match file path")
	}

	// Should match path prefix
	alert.Syscheck.Path = "/etc/nginx/nginx.conf"
	matcher = &CheckpointMatcher{
		FilePaths: []string{"/etc/nginx"},
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match file path prefix")
	}

	// Should not match unrelated path
	matcher = &CheckpointMatcher{
		FilePaths: []string{"/var/log"},
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match file path")
	}
}

func TestService_AlertMatchesMatcher_FileEvents(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		Syscheck: &Syscheck{
			Path:  "/tmp/test.txt",
			Event: "added",
		},
	}

	// Should match when event type matches
	matcher := &CheckpointMatcher{
		FileEvents: []string{"added"},
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match file event")
	}

	// Should not match when event type doesn't match
	matcher = &CheckpointMatcher{
		FileEvents: []string{"deleted"},
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match file event")
	}
}

func TestService_AlertMatchesMatcher_LogMatch(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		FullLog: "User admin logged in from 192.168.1.100",
	}

	// Should match regex
	matcher := &CheckpointMatcher{
		LogMatch: "User .* logged in",
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match log pattern")
	}

	// Should not match when pattern doesn't match
	matcher = &CheckpointMatcher{
		LogMatch: "User .* logged out",
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match log pattern")
	}
}

func TestService_AlertMatchesMatcher_RequireAll(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{
		Rule: Rule{
			ID:     "550",
			Level:  7,
			Groups: []string{"fim"},
		},
		Syscheck: &Syscheck{
			Path:  "/etc/passwd",
			Event: "modified",
		},
	}

	// RequireAll=true: should match only if ALL conditions match
	matcher := &CheckpointMatcher{
		RuleIDs:    []string{"550"},
		MinLevel:   5,
		FilePaths:  []string{"/etc"},
		RequireAll: true,
	}
	if !svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to match all conditions")
	}

	// RequireAll=true with one failing condition
	matcher = &CheckpointMatcher{
		RuleIDs:    []string{"550"},
		MinLevel:   10, // Too high
		RequireAll: true,
	}
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected alert to not match when one condition fails with RequireAll")
	}
}

func TestService_AlertMatchesMatcher_NoConditions(t *testing.T) {
	svc := newTestService(nil)

	alert := &Alert{}
	matcher := &CheckpointMatcher{}

	// Empty matcher should not match anything
	if svc.alertMatchesMatcher(alert, matcher) {
		t.Error("expected empty matcher to not match")
	}
}

// -----------------------------------------------------------------------------
// Build Matchers Tests
// -----------------------------------------------------------------------------

func TestService_BuildMatchers(t *testing.T) {
	svc := newTestService(nil)

	checkpoints := []models.Checkpoint{
		{
			ID: "cp-file-exists",
			Triggers: []models.CheckpointTrigger{
				{
					Type: models.TriggerTypeFileExists,
					Match: models.TriggerMatch{
						Path: "/tmp/testfile.txt",
					},
				},
			},
		},
		{
			ID: "cp-file-deleted",
			Triggers: []models.CheckpointTrigger{
				{
					Type: models.TriggerTypeFileDeleted,
					Match: models.TriggerMatch{
						Path: "/tmp/oldfile.txt",
					},
				},
			},
		},
		{
			ID: "cp-service",
			Triggers: []models.CheckpointTrigger{
				{
					Type: models.TriggerTypeService,
				},
			},
		},
		{
			ID: "cp-command",
			Triggers: []models.CheckpointTrigger{
				{
					Type: models.TriggerTypeCommandExecuted,
					Match: models.TriggerMatch{
						Pattern: "apt.*install",
					},
				},
			},
		},
	}

	matchers := svc.buildMatchers(checkpoints)

	if len(matchers) != 4 {
		t.Errorf("expected 4 matchers, got %d", len(matchers))
	}

	// Check file exists matcher
	var fileExistsMatcher *CheckpointMatcher
	for i := range matchers {
		if matchers[i].CheckpointID == "cp-file-exists" {
			fileExistsMatcher = &matchers[i]
			break
		}
	}
	if fileExistsMatcher == nil {
		t.Fatal("expected to find file exists matcher")
	}
	if len(fileExistsMatcher.FilePaths) != 1 || fileExistsMatcher.FilePaths[0] != "/tmp/testfile.txt" {
		t.Error("expected file path to be set")
	}
	if len(fileExistsMatcher.FileEvents) != 1 || fileExistsMatcher.FileEvents[0] != "added" {
		t.Error("expected file event 'added' for TriggerTypeFileExists")
	}
}

// -----------------------------------------------------------------------------
// getPodForAgent Tests
// -----------------------------------------------------------------------------

func TestService_GetPodForAgent_FromMapping(t *testing.T) {
	svc := newTestService(nil)

	// Add a mapping
	svc.agentPodMappingMu.Lock()
	svc.agentPodMapping["agent-001"] = "pod-123"
	svc.agentPodMappingMu.Unlock()

	ctx := context.Background()
	podID, err := svc.getPodForAgent(ctx, "agent-001")
	if err != nil {
		t.Fatalf("getPodForAgent failed: %v", err)
	}
	if podID != "pod-123" {
		t.Errorf("expected pod ID pod-123, got %s", podID)
	}
}

func TestService_GetPodForAgent_NoMapping(t *testing.T) {
	svc := newTestService(nil)

	ctx := context.Background()
	podID, err := svc.getPodForAgent(ctx, "unknown-agent")
	if err != nil {
		t.Fatalf("getPodForAgent failed: %v", err)
	}
	if podID != "" {
		t.Errorf("expected empty pod ID for unknown agent, got %s", podID)
	}
}
