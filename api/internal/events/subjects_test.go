package events

import (
	"testing"
)

func TestBuildEventSubject(t *testing.T) {
	tests := []struct {
		name      string
		podID     string
		vmName    string
		eventType string
		want      string
	}{
		{
			name:      "basic event subject",
			podID:     "pod-123",
			vmName:    "workstation",
			eventType: "syscheck",
			want:      "labs.events.pod-123.workstation.syscheck",
		},
		{
			name:      "audit event",
			podID:     "abc",
			vmName:    "server",
			eventType: "audit",
			want:      "labs.events.abc.server.audit",
		},
		{
			name:      "empty components",
			podID:     "",
			vmName:    "",
			eventType: "",
			want:      "labs.events...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildEventSubject(tt.podID, tt.vmName, tt.eventType)
			if got != tt.want {
				t.Errorf("BuildEventSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCheckpointSubject(t *testing.T) {
	tests := []struct {
		name         string
		podID        string
		checkpointID string
		want         string
	}{
		{
			name:         "basic checkpoint subject",
			podID:        "pod-123",
			checkpointID: "enable-ufw",
			want:         "labs.checkpoints.pod-123.enable-ufw",
		},
		{
			name:         "different checkpoint",
			podID:        "xyz",
			checkpointID: "install-nginx",
			want:         "labs.checkpoints.xyz.install-nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCheckpointSubject(tt.podID, tt.checkpointID)
			if got != tt.want {
				t.Errorf("BuildCheckpointSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSessionSubject(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		action    string
		want      string
	}{
		{
			name:      "session started",
			sessionID: "session-abc",
			action:    SessionActionStarted,
			want:      "labs.sessions.session-abc.started",
		},
		{
			name:      "session ended",
			sessionID: "session-123",
			action:    SessionActionEnded,
			want:      "labs.sessions.session-123.ended",
		},
		{
			name:      "session submitted",
			sessionID: "xyz",
			action:    SessionActionSubmitted,
			want:      "labs.sessions.xyz.submitted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSessionSubject(tt.sessionID, tt.action)
			if got != tt.want {
				t.Errorf("BuildSessionSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildGradeSubject(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      string
	}{
		{
			name:      "basic grade subject",
			sessionID: "session-123",
			want:      "labs.grades.session-123",
		},
		{
			name:      "another session",
			sessionID: "abc-def",
			want:      "labs.grades.abc-def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildGradeSubject(tt.sessionID)
			if got != tt.want {
				t.Errorf("BuildGradeSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPodEventsSubject(t *testing.T) {
	tests := []struct {
		name  string
		podID string
		want  string
	}{
		{
			name:  "pod events wildcard",
			podID: "pod-123",
			want:  "labs.events.pod-123.>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPodEventsSubject(tt.podID)
			if got != tt.want {
				t.Errorf("BuildPodEventsSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPodCheckpointsSubject(t *testing.T) {
	tests := []struct {
		name  string
		podID string
		want  string
	}{
		{
			name:  "pod checkpoints wildcard",
			podID: "pod-456",
			want:  "labs.checkpoints.pod-456.>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPodCheckpointsSubject(tt.podID)
			if got != tt.want {
				t.Errorf("BuildPodCheckpointsSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Verify important constants haven't changed
	if StreamName != "LABS" {
		t.Errorf("StreamName = %v, want LABS", StreamName)
	}

	if SubjectPrefixEvents != "labs.events" {
		t.Errorf("SubjectPrefixEvents = %v, want labs.events", SubjectPrefixEvents)
	}

	if SubjectPrefixCheckpoints != "labs.checkpoints" {
		t.Errorf("SubjectPrefixCheckpoints = %v, want labs.checkpoints", SubjectPrefixCheckpoints)
	}

	if SubjectPrefixSessions != "labs.sessions" {
		t.Errorf("SubjectPrefixSessions = %v, want labs.sessions", SubjectPrefixSessions)
	}

	if SubjectPrefixGrades != "labs.grades" {
		t.Errorf("SubjectPrefixGrades = %v, want labs.grades", SubjectPrefixGrades)
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Verify event types
	eventTypes := []string{
		EventTypeSyscheck,
		EventTypeAudit,
		EventTypePackage,
		EventTypeService,
		EventTypeAuth,
		EventTypeSSH,
		EventTypeFirewall,
		EventTypeNetwork,
		EventTypeUser,
		EventTypeVulnerability,
		EventTypeCustom,
		EventTypeGeneric,
	}

	for _, et := range eventTypes {
		if et == "" {
			t.Error("Event type constant should not be empty")
		}
	}
}

func TestSessionActionConstants(t *testing.T) {
	actions := map[string]string{
		"started":   SessionActionStarted,
		"ended":     SessionActionEnded,
		"paused":    SessionActionPaused,
		"resumed":   SessionActionResumed,
		"reset":     SessionActionReset,
		"submitted": SessionActionSubmitted,
	}

	for expected, got := range actions {
		if got != expected {
			t.Errorf("Session action %v = %v, want %v", expected, got, expected)
		}
	}
}

func TestCheckpointActionConstants(t *testing.T) {
	actions := map[string]string{
		"passed":  CheckpointActionPassed,
		"failed":  CheckpointActionFailed,
		"reset":   CheckpointActionReset,
		"skipped": CheckpointActionSkipped,
	}

	for expected, got := range actions {
		if got != expected {
			t.Errorf("Checkpoint action %v = %v, want %v", expected, got, expected)
		}
	}
}
