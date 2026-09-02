package checkpoint

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewEvaluator(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	if eval == nil {
		t.Fatal("NewEvaluator returned nil")
	}

	if eval.templates == nil {
		t.Error("templates map should be initialized")
	}

	if eval.sessions == nil {
		t.Error("sessions map should be initialized")
	}
}

func TestRegisterTemplate(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name: "test-lab",
		},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:          "checkpoint-1",
					Description: "Enable firewall",
					Points:      10,
				},
				{
					ID:          "checkpoint-2",
					Description: "Install nginx",
					Points:      15,
				},
			},
		},
	}

	err := eval.RegisterTemplate(template)
	if err != nil {
		t.Fatalf("RegisterTemplate() error = %v", err)
	}

	// Verify template was registered
	eval.mu.RLock()
	lc, ok := eval.templates["test-lab"]
	eval.mu.RUnlock()

	if !ok {
		t.Fatal("Template was not registered")
	}

	if lc.MaxPoints != 25 {
		t.Errorf("MaxPoints = %v, want 25", lc.MaxPoints)
	}

	if len(lc.Checkpoints) != 2 {
		t.Errorf("Checkpoint count = %v, want 2", len(lc.Checkpoints))
	}
}

func TestRegisterTemplateNoCheckpoints(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name: "empty-lab",
		},
		Spec: models.LabSpec{},
	}

	err := eval.RegisterTemplate(template)
	if err != nil {
		t.Fatalf("RegisterTemplate() with no checkpoints should not error: %v", err)
	}

	eval.mu.RLock()
	_, ok := eval.templates["empty-lab"]
	eval.mu.RUnlock()

	if ok {
		t.Error("Template with no checkpoints should not be registered")
	}
}

func TestStartSession(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	// Register template first
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp-1", Points: 10},
				{ID: "cp-2", Points: 20},
			},
		},
	}
	eval.RegisterTemplate(template)

	// Start session
	err := eval.StartSession("session-123", "pod-456", "user-789", "test-lab")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Verify session state
	state, err := eval.GetSessionProgress("session-123")
	if err != nil {
		t.Fatalf("GetSessionProgress() error = %v", err)
	}

	if state.SessionID != "session-123" {
		t.Errorf("SessionID = %v, want session-123", state.SessionID)
	}

	if state.MaxPoints != 30 {
		t.Errorf("MaxPoints = %v, want 30", state.MaxPoints)
	}

	if len(state.Checkpoints) != 2 {
		t.Errorf("Checkpoint count = %v, want 2", len(state.Checkpoints))
	}

	// Verify all checkpoints start as pending
	for id, cp := range state.Checkpoints {
		if cp.Status != models.CheckpointStatusPending {
			t.Errorf("Checkpoint %v status = %v, want pending", id, cp.Status)
		}
	}
}

func TestStartSessionUnknownTemplate(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	err := eval.StartSession("session-123", "pod-456", "user-789", "unknown-template")
	if err == nil {
		t.Error("StartSession() should error for unknown template")
	}
}

func TestEndSession(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{{ID: "cp-1", Points: 10}},
		},
	}
	eval.RegisterTemplate(template)
	eval.StartSession("session-123", "pod-456", "user-789", "test-lab")

	// Verify session exists
	_, err := eval.GetSessionProgress("session-123")
	if err != nil {
		t.Fatalf("Session should exist before EndSession")
	}

	// End session
	eval.EndSession("session-123")

	// Verify session is gone
	_, err = eval.GetSessionProgress("session-123")
	if err == nil {
		t.Error("Session should not exist after EndSession")
	}
}

func TestGetCheckpointStatus(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp-1", Points: 10},
				{ID: "cp-2", Points: 20},
			},
		},
	}
	eval.RegisterTemplate(template)
	eval.StartSession("session-123", "pod-456", "user-789", "test-lab")

	// Get existing checkpoint
	progress, err := eval.GetCheckpointStatus("session-123", "cp-1")
	if err != nil {
		t.Fatalf("GetCheckpointStatus() error = %v", err)
	}

	if progress.CheckpointID != "cp-1" {
		t.Errorf("CheckpointID = %v, want cp-1", progress.CheckpointID)
	}

	if progress.Points != 10 {
		t.Errorf("Points = %v, want 10", progress.Points)
	}

	// Try non-existent checkpoint
	_, err = eval.GetCheckpointStatus("session-123", "nonexistent")
	if err == nil {
		t.Error("GetCheckpointStatus() should error for unknown checkpoint")
	}

	// Try non-existent session
	_, err = eval.GetCheckpointStatus("unknown-session", "cp-1")
	if err == nil {
		t.Error("GetCheckpointStatus() should error for unknown session")
	}
}

func TestDependenciesMet(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	session := &SessionState{
		Checkpoints: map[string]*CheckpointProgress{
			"cp-1": {Status: models.CheckpointStatusPassed},
			"cp-2": {Status: models.CheckpointStatusPending},
			"cp-3": {Status: models.CheckpointStatusFailed},
		},
	}

	tests := []struct {
		name       string
		checkpoint *models.Checkpoint
		want       bool
	}{
		{
			name:       "no dependencies",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: nil},
			want:       true,
		},
		{
			name:       "empty dependencies",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{}},
			want:       true,
		},
		{
			name:       "dependency passed",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{"cp-1"}},
			want:       true,
		},
		{
			name:       "dependency pending",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{"cp-2"}},
			want:       false,
		},
		{
			name:       "dependency failed",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{"cp-3"}},
			want:       false,
		},
		{
			name:       "multiple dependencies all passed",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{"cp-1"}},
			want:       true,
		},
		{
			name:       "multiple dependencies one pending",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{"cp-1", "cp-2"}},
			want:       false,
		},
		{
			name:       "unknown dependency",
			checkpoint: &models.Checkpoint{ID: "test", DependsOn: []string{"unknown"}},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.dependenciesMet(session, tt.checkpoint)
			if got != tt.want {
				t.Errorf("dependenciesMet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTriggerTypeMatchesEvent(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		triggerType models.TriggerType
		eventType   string
		want        bool
	}{
		{models.TriggerTypeFileExists, events.EventTypeSyscheck, true},
		{models.TriggerTypeFileContent, events.EventTypeSyscheck, true},
		{models.TriggerTypeFileDeleted, events.EventTypeSyscheck, true},
		{models.TriggerTypePackage, events.EventTypePackage, true},
		{models.TriggerTypeService, events.EventTypeService, true},
		{models.TriggerTypeCommandExecuted, events.EventTypeAudit, true},
		{models.TriggerTypeUserCreated, events.EventTypeUser, true},
		{models.TriggerTypeUserCreated, events.EventTypeAuth, true},
		{models.TriggerTypePermission, events.EventTypeSyscheck, true},
		{models.TriggerTypeNetwork, events.EventTypeNetwork, true},

		// Mismatches
		{models.TriggerTypeFileExists, events.EventTypeAudit, false},
		{models.TriggerTypePackage, events.EventTypeSyscheck, false},
		{models.TriggerTypeService, events.EventTypePackage, false},
		{models.TriggerTypeCustom, events.EventTypeSyscheck, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.triggerType)+"_"+tt.eventType, func(t *testing.T) {
			got := eval.triggerTypeMatchesEvent(tt.triggerType, tt.eventType)
			if got != tt.want {
				t.Errorf("triggerTypeMatchesEvent(%v, %v) = %v, want %v",
					tt.triggerType, tt.eventType, got, tt.want)
			}
		})
	}
}

func TestEvaluateFileMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.SyscheckData
		want    bool
	}{
		{
			name: "file exists - added",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileExists,
				Match: models.TriggerMatch{Path: "/etc/test.conf"},
			},
			data: models.SyscheckData{Path: "/etc/test.conf", Event: "added"},
			want: true,
		},
		{
			name: "file exists - modified",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileExists,
				Match: models.TriggerMatch{Path: "/etc/test.conf"},
			},
			data: models.SyscheckData{Path: "/etc/test.conf", Event: "modified"},
			want: true,
		},
		{
			name: "file exists - deleted (should fail)",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileExists,
				Match: models.TriggerMatch{Path: "/etc/test.conf"},
			},
			data: models.SyscheckData{Path: "/etc/test.conf", Event: "deleted"},
			want: false,
		},
		{
			name: "file deleted",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileDeleted,
				Match: models.TriggerMatch{Path: "/tmp/bad.txt"},
			},
			data: models.SyscheckData{Path: "/tmp/bad.txt", Event: "deleted"},
			want: true,
		},
		{
			name: "file content contains",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileContent,
				Match: models.TriggerMatch{Path: "/etc/ssh/sshd_config", Contains: "PermitRootLogin no"},
			},
			data: models.SyscheckData{
				Path:       "/etc/ssh/sshd_config",
				Event:      "modified",
				NewContent: "# SSH config\nPermitRootLogin no\nPasswordAuthentication yes",
			},
			want: true,
		},
		{
			name: "file content regex",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileContent,
				Match: models.TriggerMatch{Path: "/etc/nginx/nginx.conf", Regex: `server_name\s+example\.com`},
			},
			data: models.SyscheckData{
				Path:       "/etc/nginx/nginx.conf",
				Event:      "modified",
				NewContent: "server {\n  server_name example.com;\n}",
			},
			want: true,
		},
		{
			name: "path mismatch",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFileExists,
				Match: models.TriggerMatch{Path: "/etc/file1.conf"},
			},
			data: models.SyscheckData{Path: "/etc/file2.conf", Event: "added"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{
				Data: dataJSON,
			}

			got := eval.evaluateFileMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateFileMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluatePackageMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.PackageData
		want    bool
	}{
		{
			name: "package installed",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypePackage,
				Match: models.TriggerMatch{Package: "nginx", State: "installed"},
			},
			data: models.PackageData{Package: "nginx", Action: "install"},
			want: true,
		},
		{
			name: "package upgraded counts as installed",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypePackage,
				Match: models.TriggerMatch{Package: "apache2", State: "installed"},
			},
			data: models.PackageData{Package: "apache2", Action: "upgrade"},
			want: true,
		},
		{
			name: "package removed",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypePackage,
				Match: models.TriggerMatch{Package: "telnet", State: "removed"},
			},
			data: models.PackageData{Package: "telnet", Action: "remove"},
			want: true,
		},
		{
			name: "wrong package",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypePackage,
				Match: models.TriggerMatch{Package: "nginx", State: "installed"},
			},
			data: models.PackageData{Package: "apache2", Action: "install"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluatePackageMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluatePackageMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateServiceMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.ServiceData
		want    bool
	}{
		{
			name: "service active",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeService,
				Match: models.TriggerMatch{Name: "nginx", State: "active"},
			},
			data: models.ServiceData{Unit: "nginx", State: "active"},
			want: true,
		},
		{
			name: "service with .service suffix",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeService,
				Match: models.TriggerMatch{Name: "ssh", State: "active"},
			},
			data: models.ServiceData{Unit: "ssh.service", State: "active"},
			want: true,
		},
		{
			name: "wrong service",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeService,
				Match: models.TriggerMatch{Name: "nginx", State: "active"},
			},
			data: models.ServiceData{Unit: "apache2", State: "active"},
			want: false,
		},
		{
			name: "wrong state",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeService,
				Match: models.TriggerMatch{Name: "nginx", State: "active"},
			},
			data: models.ServiceData{Unit: "nginx", State: "inactive"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateServiceMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateServiceMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateCommandMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.AuditData
		want    bool
	}{
		{
			name: "command pattern match",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCommandExecuted,
				Match: models.TriggerMatch{Pattern: `ufw\s+enable`},
			},
			data: models.AuditData{Command: "ufw enable"},
			want: true,
		},
		{
			name: "exe pattern match",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCommandExecuted,
				Match: models.TriggerMatch{Pattern: `/usr/bin/systemctl`},
			},
			data: models.AuditData{Exe: "/usr/bin/systemctl"},
			want: true,
		},
		{
			name: "user match",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCommandExecuted,
				Match: models.TriggerMatch{Pattern: ".*", User: "root"},
			},
			data: models.AuditData{Command: "some command", User: "root"},
			want: true,
		},
		{
			name: "wrong user",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCommandExecuted,
				Match: models.TriggerMatch{Pattern: ".*", User: "root"},
			},
			data: models.AuditData{Command: "some command", User: "student"},
			want: false,
		},
		{
			name: "pattern no match",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCommandExecuted,
				Match: models.TriggerMatch{Pattern: `apt\s+install\s+nginx`},
			},
			data: models.AuditData{Command: "apt install apache2"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateCommandMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateCommandMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateUserMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.UserData
		want    bool
	}{
		{
			name: "user created",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeUserCreated,
				Match: models.TriggerMatch{Username: "testuser"},
			},
			data: models.UserData{Username: "testuser", Action: "created"},
			want: true,
		},
		{
			name: "wrong action",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeUserCreated,
				Match: models.TriggerMatch{Username: "testuser"},
			},
			data: models.UserData{Username: "testuser", Action: "deleted"},
			want: false,
		},
		{
			name: "wrong user",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeUserCreated,
				Match: models.TriggerMatch{Username: "testuser"},
			},
			data: models.UserData{Username: "otheruser", Action: "created"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateUserMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateUserMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckpointComplete(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name       string
		progress   *CheckpointProgress
		checkpoint *models.Checkpoint
		want       bool
	}{
		{
			name: "no triggers met",
			progress: &CheckpointProgress{
				TriggersMet: map[int]bool{},
			},
			checkpoint: &models.Checkpoint{},
			want:       false,
		},
		{
			name: "one trigger met",
			progress: &CheckpointProgress{
				TriggersMet: map[int]bool{0: true},
			},
			checkpoint: &models.Checkpoint{},
			want:       true,
		},
		{
			name: "multiple triggers met",
			progress: &CheckpointProgress{
				TriggersMet: map[int]bool{0: true, 1: true, 2: true},
			},
			checkpoint: &models.Checkpoint{},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.checkpointComplete(tt.progress, tt.checkpoint)
			if got != tt.want {
				t.Errorf("checkpointComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Tests for new system metric trigger types

func TestEvaluateDiskUsageMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.DiskUsageData
		want    bool
	}{
		{
			name: "disk usage above threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					MountPoint:   "/",
					ThresholdPct: 80.0,
					Operator:     ">=",
				},
			},
			data: models.DiskUsageData{MountPoint: "/", UsedPercent: 85.5},
			want: true,
		},
		{
			name: "disk usage below threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					MountPoint:   "/home",
					ThresholdPct: 90.0,
					Operator:     "<",
				},
			},
			data: models.DiskUsageData{MountPoint: "/home", UsedPercent: 50.0},
			want: true,
		},
		{
			name: "wrong mount point",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					MountPoint:   "/var",
					ThresholdPct: 80.0,
					Operator:     ">=",
				},
			},
			data: models.DiskUsageData{MountPoint: "/home", UsedPercent: 90.0},
			want: false,
		},
		{
			name: "disk usage exactly at threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					MountPoint:   "/",
					ThresholdPct: 50.0,
					Operator:     "==",
				},
			},
			data: models.DiskUsageData{MountPoint: "/", UsedPercent: 50.0},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateDiskUsageMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateDiskUsageMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateCPULoadMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.CPULoadData
		want    bool
	}{
		{
			name: "load1 above threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdValue: 2.0,
					Operator:       ">",
				},
			},
			data: models.CPULoadData{Load1: 3.5, Load5: 2.0, Load15: 1.5},
			want: true,
		},
		{
			name: "load below threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdValue: 1.0,
					Operator:       "<",
				},
			},
			data: models.CPULoadData{Load1: 0.5, Load5: 0.3, Load15: 0.2},
			want: true,
		},
		{
			name: "load not above threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdValue: 5.0,
					Operator:       ">",
				},
			},
			data: models.CPULoadData{Load1: 2.0, Load5: 1.5, Load15: 1.0},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateCPULoadMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateCPULoadMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateMemoryUsageMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.MemoryUsageData
		want    bool
	}{
		{
			name: "memory usage above threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeMemoryUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80.0,
					Operator:     ">=",
				},
			},
			data: models.MemoryUsageData{UsedPercent: 85.0, TotalBytes: 8000000000, UsedBytes: 6800000000},
			want: true,
		},
		{
			name: "memory usage below threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeMemoryUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 50.0,
					Operator:     "<=",
				},
			},
			data: models.MemoryUsageData{UsedPercent: 40.0, TotalBytes: 8000000000, UsedBytes: 3200000000},
			want: true,
		},
		{
			name: "memory usage not at threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeMemoryUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 90.0,
					Operator:     ">",
				},
			},
			data: models.MemoryUsageData{UsedPercent: 70.0},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateMemoryUsageMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateMemoryUsageMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateProcessMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.ProcessData
		want    bool
	}{
		{
			name: "process running",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "nginx",
					State:       "running",
				},
			},
			data: models.ProcessData{Name: "nginx", State: "running", PID: 1234},
			want: true,
		},
		{
			name: "process pattern match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					Pattern: "apache.*",
					State:   "running",
				},
			},
			data: models.ProcessData{Name: "apache2", State: "running", PID: 5678},
			want: true,
		},
		{
			name: "process not running",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "nginx",
					State:       "running",
				},
			},
			data: models.ProcessData{Name: "nginx", State: "stopped", PID: 0},
			want: false,
		},
		{
			name: "wrong process",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "nginx",
					State:       "running",
				},
			},
			data: models.ProcessData{Name: "apache2", State: "running", PID: 1234},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateProcessMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateProcessMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluatePortListenMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.PortListenData
		want    bool
	}{
		{
			name: "port listening",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port:     80,
					Protocol: "tcp",
					State:    "listening",
				},
			},
			data: models.PortListenData{Port: 80, Protocol: "tcp", State: "listening", Process: "nginx"},
			want: true,
		},
		{
			name: "port not listening",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port:     443,
					Protocol: "tcp",
					State:    "listening",
				},
			},
			data: models.PortListenData{Port: 443, Protocol: "tcp", State: "closed"},
			want: false,
		},
		{
			name: "wrong port",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port:     80,
					Protocol: "tcp",
					State:    "listening",
				},
			},
			data: models.PortListenData{Port: 8080, Protocol: "tcp", State: "listening"},
			want: false,
		},
		{
			name: "UDP port listening",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port:     53,
					Protocol: "udp",
					State:    "listening",
				},
			},
			data: models.PortListenData{Port: 53, Protocol: "udp", State: "listening", Process: "named"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluatePortListenMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluatePortListenMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateCronJobMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.CronJobData
		want    bool
	}{
		{
			name: "cron job exists",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Pattern: "backup",
					User:    "root",
				},
			},
			data: models.CronJobData{
				User:    "root",
				Command: "/usr/local/bin/backup.sh",
				Status:  "created",
			},
			want: true,
		},
		{
			name: "cron job with schedule pattern",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Pattern: `0 \* \* \* \*`,
				},
			},
			data: models.CronJobData{
				Schedule: "0 * * * *",
				Command:  "/usr/bin/cleanup",
				Status:   "created",
			},
			want: true,
		},
		{
			name: "wrong user",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					User: "admin",
				},
			},
			data: models.CronJobData{User: "root", Command: "/backup.sh"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateCronJobMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateCronJobMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateFirewallMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name    string
		trigger *models.CheckpointTrigger
		data    models.FirewallRuleData
		want    bool
	}{
		{
			name: "firewall rule exists - allow SSH",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Port:     22,
					Protocol: "tcp",
					Action:   "allow",
				},
			},
			data: models.FirewallRuleData{
				Chain:    "INPUT",
				Action:   "ACCEPT",
				Protocol: "tcp",
				DestPort: 22,
			},
			want: true,
		},
		{
			name: "firewall rule with source",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Port:        80,
					Protocol:    "tcp",
					Action:      "allow",
					Destination: "192.168.1.0/24",
				},
			},
			data: models.FirewallRuleData{
				Action:   "ACCEPT",
				Protocol: "tcp",
				DestPort: 80,
				SourceIP: "192.168.1.0/24",
			},
			want: true,
		},
		{
			name: "firewall deny rule",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Port:     23,
					Protocol: "tcp",
					Action:   "deny",
				},
			},
			data: models.FirewallRuleData{
				Action:   "DROP",
				Protocol: "tcp",
				DestPort: 23,
			},
			want: true,
		},
		{
			name: "wrong action",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Port:     22,
					Protocol: "tcp",
					Action:   "deny",
				},
			},
			data: models.FirewallRuleData{
				Action:   "ACCEPT",
				Protocol: "tcp",
				DestPort: 22,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataJSON, _ := json.Marshal(tt.data)
			event := &events.VMEvent{Data: dataJSON}

			got := eval.evaluateFirewallMatch(tt.trigger, event)
			if got != tt.want {
				t.Errorf("evaluateFirewallMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckThreshold(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name      string
		value     float64
		threshold float64
		operator  string
		want      bool
	}{
		{"greater than true", 10.0, 5.0, ">", true},
		{"greater than false", 3.0, 5.0, ">", false},
		{"greater or equal true", 5.0, 5.0, ">=", true},
		{"greater or equal false", 4.0, 5.0, ">=", false},
		{"less than true", 3.0, 5.0, "<", true},
		{"less than false", 7.0, 5.0, "<", false},
		{"less or equal true", 5.0, 5.0, "<=", true},
		{"less or equal false", 6.0, 5.0, "<=", false},
		{"equal true", 5.0, 5.0, "==", true},
		{"equal false", 5.1, 5.0, "==", false},
		{"not equal true", 5.1, 5.0, "!=", true},
		{"not equal false", 5.0, 5.0, "!=", false},
		{"default greater or equal", 10.0, 5.0, "", true},
		{"unknown operator", 10.0, 5.0, "??", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eval.checkThreshold(tt.value, tt.threshold, tt.operator)
			if got != tt.want {
				t.Errorf("checkThreshold(%v, %v, %q) = %v, want %v",
					tt.value, tt.threshold, tt.operator, got, tt.want)
			}
		})
	}
}

func TestTriggerTypeMatchesEventNewTypes(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		triggerType models.TriggerType
		eventType   string
		want        bool
	}{
		// New trigger types
		{models.TriggerTypeDiskUsage, events.EventTypeDiskUsage, true},
		{models.TriggerTypeCPULoad, events.EventTypeCPULoad, true},
		{models.TriggerTypeMemoryUsage, events.EventTypeMemoryUsage, true},
		{models.TriggerTypeProcessRunning, events.EventTypeProcess, true},
		{models.TriggerTypePortListening, events.EventTypePortListen, true},
		{models.TriggerTypeCronJob, events.EventTypeCron, true},
		{models.TriggerTypeFirewallRule, events.EventTypeFirewall, true},

		// Mismatches
		{models.TriggerTypeDiskUsage, events.EventTypeCPULoad, false},
		{models.TriggerTypeCPULoad, events.EventTypeMemoryUsage, false},
		{models.TriggerTypeProcessRunning, events.EventTypePortListen, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.triggerType)+"_"+tt.eventType, func(t *testing.T) {
			got := eval.triggerTypeMatchesEvent(tt.triggerType, tt.eventType)
			if got != tt.want {
				t.Errorf("triggerTypeMatchesEvent(%v, %v) = %v, want %v",
					tt.triggerType, tt.eventType, got, tt.want)
			}
		})
	}
}

func TestEvaluateEvent(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	// Track callback invocations
	var callbackCalled bool
	var lastUpdate *events.CheckpointUpdate
	eval.SetCheckpointUpdateCallback(func(ctx context.Context, update *events.CheckpointUpdate) error {
		callbackCalled = true
		lastUpdate = update
		return nil
	})

	// Register template with checkpoint
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:     "install-nginx",
					Points: 10,
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypePackage,
							Target: "webserver",
							Match:  models.TriggerMatch{Package: "nginx", State: "installed"},
						},
					},
				},
			},
		},
	}
	eval.RegisterTemplate(template)
	eval.StartSession("session-123", "pod-456", "user-789", "test-lab")

	// Create matching event
	pkgData, _ := json.Marshal(models.PackageData{Package: "nginx", Action: "install"})
	event := &events.VMEvent{
		MessageHeader: events.MessageHeader{
			ID: "event-001",
		},
		VMName:    "webserver",
		EventType: events.EventTypePackage,
		Data:      pkgData,
	}

	// Evaluate
	passed, err := eval.EvaluateEvent(context.Background(), event, "session-123")
	if err != nil {
		t.Fatalf("EvaluateEvent() error = %v", err)
	}

	if len(passed) != 1 || passed[0] != "install-nginx" {
		t.Errorf("EvaluateEvent() passed = %v, want [install-nginx]", passed)
	}

	if !callbackCalled {
		t.Error("Callback should have been called")
	}

	if lastUpdate == nil {
		t.Fatal("lastUpdate should not be nil")
	}

	if lastUpdate.CheckpointID != "install-nginx" {
		t.Errorf("Update CheckpointID = %v, want install-nginx", lastUpdate.CheckpointID)
	}

	if lastUpdate.EarnedPoints != 10 {
		t.Errorf("Update EarnedPoints = %v, want 10", lastUpdate.EarnedPoints)
	}

	// Verify session state
	state, _ := eval.GetSessionProgress("session-123")
	if state.EarnedPoints != 10 {
		t.Errorf("Session EarnedPoints = %v, want 10", state.EarnedPoints)
	}
}

func TestEvaluator_GetTemplateCheckpoints(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	// Register a template with checkpoints
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-template"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp1", Description: "First checkpoint", Points: 10},
				{ID: "cp2", Description: "Second checkpoint", Points: 20},
				{ID: "cp3", Description: "Third checkpoint", Points: 30},
			},
		},
	}
	eval.RegisterTemplate(template)

	// Get checkpoints for registered template
	got := eval.GetTemplateCheckpoints("test-template")
	if len(got) != 3 {
		t.Fatalf("expected 3 checkpoints, got %d", len(got))
	}

	// Verify checkpoint data - they should be returned in order
	foundCp1 := false
	foundCp2 := false
	foundCp3 := false
	for _, cp := range got {
		switch cp.ID {
		case "cp1":
			foundCp1 = true
			if cp.Points != 10 {
				t.Errorf("cp1 points mismatch: got %d, want 10", cp.Points)
			}
		case "cp2":
			foundCp2 = true
			if cp.Points != 20 {
				t.Errorf("cp2 points mismatch: got %d, want 20", cp.Points)
			}
		case "cp3":
			foundCp3 = true
			if cp.Points != 30 {
				t.Errorf("cp3 points mismatch: got %d, want 30", cp.Points)
			}
		}
	}
	if !foundCp1 || !foundCp2 || !foundCp3 {
		t.Errorf("missing checkpoints: cp1=%v, cp2=%v, cp3=%v", foundCp1, foundCp2, foundCp3)
	}

	// Get checkpoints for non-existent template
	got = eval.GetTemplateCheckpoints("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent template, got %+v", got)
	}
}

func TestEvaluator_MarkCheckpointPassed(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	// Register template
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-template"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp1", Description: "First checkpoint", Points: 10},
				{ID: "cp2", Description: "Second checkpoint", Points: 20, DependsOn: []string{"cp1"}},
			},
		},
	}
	eval.RegisterTemplate(template)

	// Start session
	eval.StartSession("session-123", "pod-456", "user-789", "test-template")

	// Mark first checkpoint as passed
	err := eval.MarkCheckpointPassed(context.Background(), "session-123", "cp1")
	if err != nil {
		t.Fatalf("MarkCheckpointPassed error: %v", err)
	}

	// Verify it was marked
	progress, err := eval.GetCheckpointStatus("session-123", "cp1")
	if err != nil {
		t.Fatalf("GetCheckpointStatus error: %v", err)
	}
	if progress.Status != models.CheckpointStatusPassed {
		t.Errorf("expected status 'passed', got %s", progress.Status)
	}

	// Verify points were added
	sessionProgress, _ := eval.GetSessionProgress("session-123")
	if sessionProgress.EarnedPoints != 10 {
		t.Errorf("expected 10 earned points, got %d", sessionProgress.EarnedPoints)
	}

	// Test marking non-existent checkpoint
	err = eval.MarkCheckpointPassed(context.Background(), "session-123", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent checkpoint")
	}

	// Test with non-existent session
	err = eval.MarkCheckpointPassed(context.Background(), "nonexistent-session", "cp1")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestEvaluator_GetSessionProgressWithDescriptions(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	// Register template with checkpoints
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-template"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp1", Description: "First checkpoint", Points: 10, Hint: "This is a hint"},
				{ID: "cp2", Description: "Second checkpoint", Points: 20},
			},
		},
	}
	eval.RegisterTemplate(template)

	// Start session
	eval.StartSession("session-123", "pod-456", "user-789", "test-template")

	// Get progress with descriptions
	checkpointsWithDesc, err := eval.GetSessionProgressWithDescriptions("session-123")
	if err != nil {
		t.Fatalf("GetSessionProgressWithDescriptions error: %v", err)
	}

	if len(checkpointsWithDesc) != 2 {
		t.Fatalf("expected 2 checkpoint details, got %d", len(checkpointsWithDesc))
	}

	// Find the checkpoints by ID
	var cp1, cp2 *CheckpointWithDescription
	for i := range checkpointsWithDesc {
		if checkpointsWithDesc[i].CheckpointID == "cp1" {
			cp1 = &checkpointsWithDesc[i]
		}
		if checkpointsWithDesc[i].CheckpointID == "cp2" {
			cp2 = &checkpointsWithDesc[i]
		}
	}

	if cp1 == nil {
		t.Fatal("cp1 not found in results")
	}
	if cp1.Description != "First checkpoint" {
		t.Errorf("expected description 'First checkpoint', got %s", cp1.Description)
	}
	if cp1.Hint != "This is a hint" {
		t.Errorf("expected hint, got %s", cp1.Hint)
	}
	if cp1.Status != models.CheckpointStatusPending {
		t.Errorf("expected status 'pending', got %s", cp1.Status)
	}

	if cp2 == nil {
		t.Fatal("cp2 not found in results")
	}
	if cp2.Description != "Second checkpoint" {
		t.Errorf("expected description 'Second checkpoint', got %s", cp2.Description)
	}

	// Test with non-existent session
	_, err = eval.GetSessionProgressWithDescriptions("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

// Test evaluatePermissionMatch
func TestEvaluator_EvaluatePermissionMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "permission change success with path and permission match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:       "/etc/passwd",
					Permission: "644",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:    "/etc/passwd",
					Event:   "modified",
					NewPerm: "644",
				}),
			},
			expected: true,
		},
		{
			name: "permission change with glob path match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path: "/etc/*",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/hosts",
					Event: "attributes",
				}),
			},
			expected: true,
		},
		{
			name: "path mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path: "/var/log/syslog",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/passwd",
					Event: "modified",
				}),
			},
			expected: false,
		},
		{
			name: "wrong event type - not permission change",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path: "/etc/passwd",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/passwd",
					Event: "deleted",
				}),
			},
			expected: false,
		},
		{
			name: "permission mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:       "/etc/passwd",
					Permission: "600",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:    "/etc/passwd",
					Event:   "modified",
					NewPerm: "644",
				}),
			},
			expected: false,
		},
		{
			name: "owner check with NewUser field",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:  "/etc/shadow",
					Owner: "root",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:    "/etc/shadow",
					Event:   "attributes",
					NewUser: "root",
				}),
			},
			expected: true,
		},
		{
			name: "owner check failure",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:  "/etc/shadow",
					Owner: "root",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:    "/etc/shadow",
					Event:   "attributes",
					NewUser: "nobody",
				}),
			},
			expected: false,
		},
		{
			name: "group check with NewGroup field",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:  "/etc/shadow",
					Group: "shadow",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:     "/etc/shadow",
					Event:    "attributes",
					NewGroup: "shadow",
				}),
			},
			expected: true,
		},
		{
			name: "group check failure",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:  "/etc/shadow",
					Group: "shadow",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:     "/etc/shadow",
					Event:    "attributes",
					NewGroup: "root",
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON data",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path: "/etc/passwd",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data:      []byte("invalid json"),
			},
			expected: false,
		},
		{
			name: "fallback to Permissions field when NewPerm empty",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:       "/etc/hosts",
					Permission: "644",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:        "/etc/hosts",
					Event:       "modified",
					NewPerm:     "",
					Permissions: "644",
				}),
			},
			expected: true,
		},
		{
			name: "fallback to Owner field when NewUser empty",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:  "/var/log/syslog",
					Owner: "syslog",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:    "/var/log/syslog",
					Event:   "modified",
					NewUser: "",
					Owner:   "syslog",
				}),
			},
			expected: true,
		},
		{
			name: "fallback to Group field when NewGroup empty",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePermission,
				Match: models.TriggerMatch{
					Path:  "/var/log/syslog",
					Group: "adm",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:     "/var/log/syslog",
					Event:    "modified",
					NewGroup: "",
					Group:    "adm",
				}),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluatePermissionMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluatePermissionMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateNetworkMatch
func TestEvaluator_EvaluateNetworkMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "network connection match - all fields",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Protocol:    "tcp",
					Port:        443,
					State:       "ESTABLISHED",
					Destination: "192.168.1.100",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					Protocol:   "tcp",
					RemotePort: 443,
					State:      "ESTABLISHED",
					RemoteAddr: "192.168.1.100",
				}),
			},
			expected: true,
		},
		{
			name: "protocol match only - case insensitive",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Protocol: "TCP",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					Protocol: "tcp",
				}),
			},
			expected: true,
		},
		{
			name: "protocol mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Protocol: "udp",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					Protocol: "tcp",
				}),
			},
			expected: false,
		},
		{
			name: "port match - local port",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Port: 8080,
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					LocalPort: 8080,
				}),
			},
			expected: true,
		},
		{
			name: "port match - remote port",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Port: 443,
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					RemotePort: 443,
				}),
			},
			expected: true,
		},
		{
			name: "port mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Port: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					LocalPort:  8080,
					RemotePort: 443,
				}),
			},
			expected: false,
		},
		{
			name: "state match - case insensitive",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					State: "listen",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					State: "LISTEN",
				}),
			},
			expected: true,
		},
		{
			name: "state mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					State: "ESTABLISHED",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					State: "TIME_WAIT",
				}),
			},
			expected: false,
		},
		{
			name: "destination address match using Destination field",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Destination: "10.0.0.1",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					RemoteAddr: "10.0.0.1",
				}),
			},
			expected: true,
		},
		{
			name: "destination address match using Address field",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Address: "192.168.1.50",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					RemoteAddr: "192.168.1.50",
				}),
			},
			expected: true,
		},
		{
			name: "destination prefix match with wildcard",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Destination: "192.168.1.*",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					RemoteAddr: "192.168.1.100",
				}),
			},
			expected: true,
		},
		{
			name: "destination mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Destination: "10.0.0.1",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data: mustMarshal(models.NetworkData{
					RemoteAddr: "192.168.1.1",
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON data",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeNetwork,
				Match: models.TriggerMatch{
					Protocol: "tcp",
				},
			},
			event: &events.VMEvent{
				EventType: "network",
				Data:      []byte("invalid json"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateNetworkMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateNetworkMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluatePortListenMatch
func TestEvaluator_EvaluatePortListenMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "port listening match - all fields",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port:          80,
					Protocol:      "tcp",
					ListenAddress: "0.0.0.0",
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Port:     80,
					Protocol: "tcp",
					Address:  "0.0.0.0",
					State:    "LISTEN",
				}),
			},
			expected: true,
		},
		{
			name: "port mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port: 443,
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Port:  80,
					State: "LISTEN",
				}),
			},
			expected: false,
		},
		{
			name: "protocol mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Protocol: "udp",
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Protocol: "tcp",
					State:    "LISTEN",
				}),
			},
			expected: false,
		},
		{
			name: "address mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					ListenAddress: "127.0.0.1",
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Address: "0.0.0.0",
					State:   "LISTEN",
				}),
			},
			expected: false,
		},
		{
			name: "0.0.0.0 listen address matches anything",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					ListenAddress: "0.0.0.0",
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Address: "192.168.1.1",
					State:   "LISTEN",
				}),
			},
			expected: true,
		},
		{
			name: "state match specified",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					State: "LISTEN",
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					State: "listen",
				}),
			},
			expected: true,
		},
		{
			name: "state mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					State: "ESTABLISHED",
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					State: "LISTEN",
				}),
			},
			expected: false,
		},
		{
			name: "no state specified - requires listen/listening",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Port:  80,
					State: "ESTABLISHED",
				}),
			},
			expected: false,
		},
		{
			name: "listening state variant",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data: mustMarshal(models.PortListenData{
					Port:  80,
					State: "listening",
				}),
			},
			expected: true,
		},
		{
			name: "invalid JSON data",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypePortListening,
				Match: models.TriggerMatch{
					Port: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "port_listen",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluatePortListenMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluatePortListenMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateCronJobMatch
func TestEvaluator_EvaluateCronJobMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "cron job match - all fields",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Schedule: "0 * * * *",
					Command:  "backup\\.sh",
					User:     "root",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Schedule: "0 * * * *",
					Command:  "/usr/local/bin/backup.sh",
					User:     "root",
					Status:   "active",
				}),
			},
			expected: true,
		},
		{
			name: "schedule mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Schedule: "*/5 * * * *",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Schedule: "0 * * * *",
					Status:   "active",
				}),
			},
			expected: false,
		},
		{
			name: "command regex mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Command: "backup\\.sh$",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Command: "/usr/bin/cleanup",
					Status:  "active",
				}),
			},
			expected: false,
		},
		{
			name: "user mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					User: "root",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					User:   "www-data",
					Status: "active",
				}),
			},
			expected: false,
		},
		{
			name: "pattern match on command",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Pattern: ".*cleanup.*",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Command: "/usr/bin/cleanup-logs",
					Status:  "active",
				}),
			},
			expected: true,
		},
		{
			name: "pattern match on schedule",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Pattern: "^0 \\* \\* \\* \\*$",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Command:  "echo test",
					Schedule: "0 * * * *",
					Status:   "active",
				}),
			},
			expected: true,
		},
		{
			name: "pattern mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCronJob,
				Match: models.TriggerMatch{
					Pattern: "nomatch",
				},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Command:  "backup",
					Schedule: "0 * * * *",
					Status:   "active",
				}),
			},
			expected: false,
		},
		{
			name: "status enabled accepted",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCronJob,
				Match: models.TriggerMatch{},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Status: "enabled",
				}),
			},
			expected: true,
		},
		{
			name: "status created accepted",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCronJob,
				Match: models.TriggerMatch{},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Status: "created",
				}),
			},
			expected: true,
		},
		{
			name: "status disabled rejected",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCronJob,
				Match: models.TriggerMatch{},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Status: "disabled",
				}),
			},
			expected: false,
		},
		{
			name: "empty status accepted",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCronJob,
				Match: models.TriggerMatch{},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data: mustMarshal(models.CronJobData{
					Status: "",
				}),
			},
			expected: true,
		},
		{
			name: "invalid JSON data",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeCronJob,
				Match: models.TriggerMatch{},
			},
			event: &events.VMEvent{
				EventType: "cron",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateCronJobMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateCronJobMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateFirewallMatch
func TestEvaluator_EvaluateFirewallMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "firewall match - all fields",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Chain:       "INPUT",
					Action:      "allow",
					Protocol:    "tcp",
					Port:        22,
					Destination: "10.0.0.0/8",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Chain:    "INPUT",
					Action:   "ACCEPT",
					Protocol: "tcp",
					DestPort: 22,
					SourceIP: "10.0.0.0/8",
				}),
			},
			expected: true,
		},
		{
			name: "chain mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Chain: "INPUT",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Chain: "OUTPUT",
				}),
			},
			expected: false,
		},
		{
			name: "action accept - allow alias",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Action: "allow",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Action: "accept",
				}),
			},
			expected: true,
		},
		{
			name: "action deny - drop alias",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Action: "deny",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Action: "drop",
				}),
			},
			expected: true,
		},
		{
			name: "action reject",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Action: "reject",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Action: "reject",
				}),
			},
			expected: true,
		},
		{
			name: "action mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Action: "allow",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Action: "drop",
				}),
			},
			expected: false,
		},
		{
			name: "protocol mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Protocol: "udp",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Protocol: "tcp",
				}),
			},
			expected: false,
		},
		{
			name: "port mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Port: 443,
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					DestPort: 80,
				}),
			},
			expected: false,
		},
		{
			name: "destination match on DestIP",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Destination: "192.168.1.0/24",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					DestIP: "192.168.1.0/24",
				}),
			},
			expected: true,
		},
		{
			name: "destination mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Destination: "10.0.0.0/8",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					SourceIP: "192.168.0.0/16",
					DestIP:   "172.16.0.0/12",
				}),
			},
			expected: false,
		},
		{
			name: "interface match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Interface: "eth0",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Interface: "eth0",
				}),
			},
			expected: true,
		},
		{
			name: "interface mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Interface: "eth0",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Interface: "eth1",
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON data",
			trigger: &models.CheckpointTrigger{
				Type:  models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
		{
			name: "case insensitive chain match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFirewallRule,
				Match: models.TriggerMatch{
					Chain: "input",
				},
			},
			event: &events.VMEvent{
				EventType: "firewall",
				Data: mustMarshal(models.FirewallRuleData{
					Chain: "INPUT",
				}),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateFirewallMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateFirewallMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateMatch via switch statement coverage
func TestEvaluator_EvaluateMatch_SwitchCoverage(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	// Test unsupported trigger type falls through to default
	trigger := &models.CheckpointTrigger{
		Type:  "unsupported_type",
		Match: models.TriggerMatch{},
	}
	event := &events.VMEvent{
		EventType: "test",
		Data:      []byte("{}"),
	}

	result := eval.evaluateMatch(trigger, event)
	if result != false {
		t.Error("expected false for unsupported trigger type")
	}
}

// Helper function to marshal JSON for test events
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// Test evaluateFileMatch comprehensively
func TestEvaluator_EvaluateFileMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "file_exists - exact path match - added event",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/etc/hosts",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/hosts",
					Event: "added",
				}),
			},
			expected: true,
		},
		{
			name: "file_exists - exact path match - modified event",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/etc/hosts",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/hosts",
					Event: "modified",
				}),
			},
			expected: true,
		},
		{
			name: "file_exists - exact path match - deleted event not matched",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/etc/hosts",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/hosts",
					Event: "deleted",
				}),
			},
			expected: false,
		},
		{
			name: "file_exists - glob path with wildcard",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/etc/nginx/*",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/nginx/nginx.conf",
					Event: "added",
				}),
			},
			expected: true,
		},
		{
			name: "file_exists - directory existence via child file",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/home/user/projects",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/home/user/projects/file.txt",
					Event: "added",
				}),
			},
			expected: true,
		},
		{
			name: "file_exists - path mismatch no wildcard",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/var/log/syslog",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/hosts",
					Event: "added",
				}),
			},
			expected: false,
		},
		{
			name: "file_deleted - exact match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileDeleted,
				Match: models.TriggerMatch{
					Path: "/tmp/tempfile",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/tmp/tempfile",
					Event: "deleted",
				}),
			},
			expected: true,
		},
		{
			name: "file_deleted - wrong event type",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileDeleted,
				Match: models.TriggerMatch{
					Path: "/tmp/tempfile",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/tmp/tempfile",
					Event: "added",
				}),
			},
			expected: false,
		},
		{
			name: "file_content - contains match in NewContent",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:     "/etc/nginx/nginx.conf",
					Contains: "server_name",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:       "/etc/nginx/nginx.conf",
					Event:      "modified",
					NewContent: "http { server { server_name localhost; } }",
				}),
			},
			expected: true,
		},
		{
			name: "file_content - contains match in Diff",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:     "/etc/config.yml",
					Contains: "debug: true",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/config.yml",
					Event: "modified",
					Diff:  "+debug: true\n-debug: false",
				}),
			},
			expected: true,
		},
		{
			name: "file_content - contains no match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:     "/etc/config.yml",
					Contains: "not_found",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:       "/etc/config.yml",
					Event:      "modified",
					NewContent: "some content",
					Diff:       "some diff",
				}),
			},
			expected: false,
		},
		{
			name: "file_content - regex match in NewContent",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:  "/etc/passwd",
					Regex: "^newuser:.*:1000:",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:       "/etc/passwd",
					Event:      "modified",
					NewContent: "newuser:x:1000:1000:New User:/home/newuser:/bin/bash",
				}),
			},
			expected: true,
		},
		{
			name: "file_content - regex match in Diff",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:  "/etc/hosts",
					Regex: `192\.168\.1\.\d+\s+myserver`,
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/hosts",
					Event: "modified",
					Diff:  "+192.168.1.100 myserver",
				}),
			},
			expected: true,
		},
		{
			name: "file_content - regex no match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:  "/etc/hosts",
					Regex: `10\.0\.0\.\d+`,
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:       "/etc/hosts",
					Event:      "modified",
					NewContent: "192.168.1.1 localhost",
				}),
			},
			expected: false,
		},
		{
			name: "file_content - invalid regex",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path:  "/etc/hosts",
					Regex: "[invalid(regex",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:       "/etc/hosts",
					Event:      "modified",
					NewContent: "content",
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON data",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileExists,
				Match: models.TriggerMatch{
					Path: "/etc/hosts",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data:      []byte("invalid json"),
			},
			expected: false,
		},
		{
			name: "file_content - no path specified",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Contains: "test",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:       "/any/path",
					Event:      "modified",
					NewContent: "test content",
				}),
			},
			expected: true,
		},
		{
			name: "file_content - no match criteria",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileContent,
				Match: models.TriggerMatch{
					Path: "/etc/file",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/etc/file",
					Event: "modified",
				}),
			},
			expected: false,
		},
		{
			name: "file_deleted - wildcard path match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeFileDeleted,
				Match: models.TriggerMatch{
					Path: "/tmp/test/*",
				},
			},
			event: &events.VMEvent{
				EventType: "syscheck",
				Data: mustMarshal(models.SyscheckData{
					Path:  "/tmp/test/file.txt",
					Event: "deleted",
				}),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateFileMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateFileMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateProcessMatch
func TestEvaluator_EvaluateProcessMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "process running - name match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "nginx",
				},
			},
			event: &events.VMEvent{
				EventType: "process",
				Data: mustMarshal(models.ProcessData{
					Name:  "nginx",
					State: "running",
				}),
			},
			expected: true,
		},
		{
			name: "process running - command line match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "python",
				},
			},
			event: &events.VMEvent{
				EventType: "process",
				Data: mustMarshal(models.ProcessData{
					Name:        "python3",
					CommandLine: "/usr/bin/python app.py",
					State:       "sleeping",
				}),
			},
			expected: true,
		},
		{
			name: "process name mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "apache",
				},
			},
			event: &events.VMEvent{
				EventType: "process",
				Data: mustMarshal(models.ProcessData{
					Name:  "nginx",
					State: "running",
				}),
			},
			expected: false,
		},
		{
			name: "process not running - zombie state",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "test",
				},
			},
			event: &events.VMEvent{
				EventType: "process",
				Data: mustMarshal(models.ProcessData{
					Name:  "test",
					State: "zombie",
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeProcessRunning,
				Match: models.TriggerMatch{
					ProcessName: "test",
				},
			},
			event: &events.VMEvent{
				EventType: "process",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateProcessMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateProcessMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateDiskUsageMatch
func TestEvaluator_EvaluateDiskUsageMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "disk usage exceeds threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "disk_usage",
				Data: mustMarshal(models.DiskUsageData{
					UsedPercent: 85,
				}),
			},
			expected: true,
		},
		{
			name: "disk usage below threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "disk_usage",
				Data: mustMarshal(models.DiskUsageData{
					UsedPercent: 50,
				}),
			},
			expected: false,
		},
		{
			name: "mount point match",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					MountPoint:   "/data",
					ThresholdPct: 90,
				},
			},
			event: &events.VMEvent{
				EventType: "disk_usage",
				Data: mustMarshal(models.DiskUsageData{
					MountPoint:  "/data",
					UsedPercent: 95,
				}),
			},
			expected: true,
		},
		{
			name: "mount point mismatch",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					MountPoint:   "/data",
					ThresholdPct: 90,
				},
			},
			event: &events.VMEvent{
				EventType: "disk_usage",
				Data: mustMarshal(models.DiskUsageData{
					MountPoint:  "/home",
					UsedPercent: 95,
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeDiskUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "disk_usage",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateDiskUsageMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateDiskUsageMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateCPULoadMatch
func TestEvaluator_EvaluateCPULoadMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "CPU load exceeds threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdPct: 70,
				},
			},
			event: &events.VMEvent{
				EventType: "cpu_load",
				Data: mustMarshal(models.CPULoadData{
					CPUPercent: 85,
				}),
			},
			expected: true,
		},
		{
			name: "CPU load below threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdPct: 70,
				},
			},
			event: &events.VMEvent{
				EventType: "cpu_load",
				Data: mustMarshal(models.CPULoadData{
					CPUPercent: 50,
				}),
			},
			expected: false,
		},
		{
			name: "load average threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdValue: 2.0,
				},
			},
			event: &events.VMEvent{
				EventType: "cpu_load",
				Data: mustMarshal(models.CPULoadData{
					Load1: 2.5,
				}),
			},
			expected: true,
		},
		{
			name: "invalid JSON",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeCPULoad,
				Match: models.TriggerMatch{
					ThresholdPct: 70,
				},
			},
			event: &events.VMEvent{
				EventType: "cpu_load",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateCPULoadMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateCPULoadMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test evaluateMemoryUsageMatch
func TestEvaluator_EvaluateMemoryUsageMatch(t *testing.T) {
	logger := newTestLogger()
	eval := NewEvaluator(logger)

	tests := []struct {
		name     string
		trigger  *models.CheckpointTrigger
		event    *events.VMEvent
		expected bool
	}{
		{
			name: "memory usage exceeds threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeMemoryUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "memory_usage",
				Data: mustMarshal(models.MemoryUsageData{
					UsedPercent: 90,
				}),
			},
			expected: true,
		},
		{
			name: "memory usage below threshold",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeMemoryUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "memory_usage",
				Data: mustMarshal(models.MemoryUsageData{
					UsedPercent: 50,
				}),
			},
			expected: false,
		},
		{
			name: "invalid JSON",
			trigger: &models.CheckpointTrigger{
				Type: models.TriggerTypeMemoryUsage,
				Match: models.TriggerMatch{
					ThresholdPct: 80,
				},
			},
			event: &events.VMEvent{
				EventType: "memory_usage",
				Data:      []byte("invalid"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.evaluateMemoryUsageMatch(tt.trigger, tt.event)
			if result != tt.expected {
				t.Errorf("evaluateMemoryUsageMatch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizePermission(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"bare octal", "755", "0755"},
		{"leading zero", "0755", "0755"},
		{"stat mode file", "100755", "0755"},
		{"stat mode dir", "040755", "0755"},
		{"symbolic", "rwxr-xr-x", "0755"},
		{"symbolic with type", "-rwxr-xr-x", "0755"},
		{"symbolic dir", "drwxr-xr-x", "0755"},
		{"symbolic 644", "rw-r--r--", "0644"},
		{"symbolic 600", "rw-------", "0600"},
		{"symbolic none", "---------", "0000"},
		{"symbolic all", "rwxrwxrwx", "0777"},

		// Special bits must survive: a setuid binary is not the same
		// permission as the plain file, and a security course grades exactly
		// that distinction.
		{"setuid octal", "4755", "4755"},
		{"setuid symbolic", "rwsr-xr-x", "4755"},
		{"setuid stat mode", "104755", "4755"},
		{"setgid octal", "2755", "2755"},
		{"setgid symbolic", "rwxr-sr-x", "2755"},
		{"sticky octal", "1777", "1777"},
		{"sticky symbolic", "rwxrwxrwt", "1777"},
		{"sticky stat mode", "041777", "1777"},

		// Uppercase means the special bit is set and execute is NOT.
		{"setuid without execute", "rwSr--r--", "4644"},
		{"sticky without execute", "rwxrwxrwT", "1776"},

		{"whitespace trimmed", "  755  ", "0755"},
		{"empty", "", ""},
		{"unrecognised passes through", "not-a-mode", "not-a-mode"},
		{"bogus symbolic char passes through", "rwxr-xr-z", "rwxr-xr-z"},
		{"non-octal digit passes through", "899", "899"},
		{"sticky in user slot rejected", "rwtr-xr-x", "rwtr-xr-x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePermission(tt.in); got != tt.want {
				t.Errorf("normalizePermission(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestPermissionSpecialBitsDoNotCollapse guards the regression that the first
// version of the normaliser introduced: it dropped setuid/setgid/sticky, so a
// checkpoint asking for "755" credited a setuid-root binary.
func TestPermissionSpecialBitsDoNotCollapse(t *testing.T) {
	pairs := [][2]string{
		{"755", "4755"},
		{"755", "rwsr-xr-x"},
		{"777", "1777"},
		{"777", "rwxrwxrwt"},
		{"644", "rwSr--r--"},
	}
	for _, p := range pairs {
		if normalizePermission(p[0]) == normalizePermission(p[1]) {
			t.Errorf("%q and %q must not compare equal (special bits differ)", p[0], p[1])
		}
	}
}

// TestPermissionMatchThroughEvaluator drives the real matching path rather than
// the helper. Without this the normaliser could be reverted to a literal
// comparison and every other test would still pass.
func TestPermissionMatchThroughEvaluator(t *testing.T) {
	tests := []struct {
		name      string
		template  string // what the lab template asks for
		reported  string // what the agent reports in new_perm
		wantMatch bool
	}{
		{"same spelling", "644", "644", true},
		{"template octal, agent symbolic", "755", "rwxr-xr-x", true},
		{"template symbolic, agent octal", "rwxr-xr-x", "755", true},
		{"template bare, agent stat mode", "755", "100755", true},
		{"leading zero either side", "0644", "644", true},
		{"different mode", "755", "rw-r--r--", false},
		{"setuid must not match plain", "755", "4755", false},
	}
	e := NewEvaluator(slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(models.SyscheckData{
				Path: "/home/student/script.sh", Event: "modified", NewPerm: tt.reported,
			})
			if err != nil {
				t.Fatal(err)
			}
			trigger := &models.CheckpointTrigger{
				Type:   models.TriggerTypePermission,
				Target: "vm",
				Match:  models.TriggerMatch{Path: "/home/student/script.sh", Permission: tt.template},
			}
			got := e.evaluateMatch(trigger, &events.VMEvent{Data: data})
			if got != tt.wantMatch {
				t.Errorf("template %q vs reported %q: match = %v, want %v",
					tt.template, tt.reported, got, tt.wantMatch)
			}
		})
	}
}
