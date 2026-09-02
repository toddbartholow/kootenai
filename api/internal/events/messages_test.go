package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMessageHeader(t *testing.T) {
	before := time.Now().UTC()
	header := NewMessageHeader("test-source")
	after := time.Now().UTC()

	if header.Source != "test-source" {
		t.Errorf("Source = %v, want test-source", header.Source)
	}

	if header.Version != "1.0" {
		t.Errorf("Version = %v, want 1.0", header.Version)
	}

	if header.ID == "" {
		t.Error("ID should not be empty")
	}

	if header.Timestamp.Before(before) || header.Timestamp.After(after) {
		t.Errorf("Timestamp %v not within expected range [%v, %v]", header.Timestamp, before, after)
	}
}

func TestGenerateMessageID(t *testing.T) {
	id1 := generateMessageID()
	id2 := generateMessageID()

	if id1 == "" {
		t.Error("Message ID should not be empty")
	}

	// IDs should be in timestamp format
	if len(id1) < 10 {
		t.Errorf("Message ID %v seems too short", id1)
	}

	// Two consecutive IDs might be the same if called in quick succession,
	// but the format should be consistent
	if len(id1) != len(id2) {
		t.Errorf("Message ID lengths differ: %v vs %v", len(id1), len(id2))
	}
}

func TestParseVMEvent(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
		check   func(*VMEvent) bool
	}{
		{
			name: "valid VM event",
			data: `{
				"id": "evt-123",
				"timestamp": "2024-01-15T10:30:00Z",
				"source": "wazuh",
				"version": "1.0",
				"podId": "pod-456",
				"vmName": "workstation",
				"agentId": "agent-789",
				"eventType": "syscheck",
				"data": {"path": "/etc/passwd"}
			}`,
			wantErr: false,
			check: func(e *VMEvent) bool {
				return e.ID == "evt-123" &&
					e.PodID == "pod-456" &&
					e.VMName == "workstation" &&
					e.AgentID == "agent-789" &&
					e.EventType == "syscheck"
			},
		},
		{
			name:    "invalid JSON",
			data:    `{invalid json`,
			wantErr: true,
			check:   nil,
		},
		{
			name:    "empty data",
			data:    `{}`,
			wantErr: false,
			check: func(e *VMEvent) bool {
				return e.ID == "" && e.PodID == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseVMEvent([]byte(tt.data))

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVMEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.check != nil && !tt.check(event) {
				t.Errorf("ParseVMEvent() returned unexpected values: %+v", event)
			}
		})
	}
}

func TestParseCheckpointUpdate(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
		check   func(*CheckpointUpdate) bool
	}{
		{
			name: "valid checkpoint update",
			data: `{
				"id": "upd-123",
				"podId": "pod-456",
				"sessionId": "session-789",
				"checkpointId": "enable-ufw",
				"action": "passed",
				"points": 10,
				"earnedPoints": 10,
				"sessionEarnedPoints": 50,
				"sessionMaxPoints": 100,
				"sessionPercentage": 50.0
			}`,
			wantErr: false,
			check: func(u *CheckpointUpdate) bool {
				return u.CheckpointID == "enable-ufw" &&
					u.Action == "passed" &&
					u.Points == 10 &&
					u.EarnedPoints == 10 &&
					u.SessionPercentage == 50.0
			},
		},
		{
			name:    "invalid JSON",
			data:    `not json`,
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update, err := ParseCheckpointUpdate([]byte(tt.data))

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCheckpointUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.check != nil && !tt.check(update) {
				t.Errorf("ParseCheckpointUpdate() returned unexpected values: %+v", update)
			}
		})
	}
}

func TestParseSessionEvent(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
		check   func(*SessionEvent) bool
	}{
		{
			name: "valid session event",
			data: `{
				"sessionId": "session-123",
				"podId": "pod-456",
				"userId": "user-789",
				"action": "started",
				"labTemplate": "linux-basics",
				"earnedPoints": 0,
				"maxPoints": 100
			}`,
			wantErr: false,
			check: func(e *SessionEvent) bool {
				return e.SessionID == "session-123" &&
					e.Action == "started" &&
					e.LabTemplate == "linux-basics" &&
					e.MaxPoints == 100
			},
		},
		{
			name:    "invalid JSON",
			data:    `[not an object]`,
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseSessionEvent([]byte(tt.data))

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSessionEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.check != nil && !tt.check(event) {
				t.Errorf("ParseSessionEvent() returned unexpected values: %+v", event)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name    string
		msg     any
		wantErr bool
	}{
		{
			name: "checkpoint update",
			msg: &CheckpointUpdate{
				MessageHeader: MessageHeader{
					ID:      "test-id",
					Source:  "test",
					Version: "1.0",
				},
				PodID:        "pod-123",
				SessionID:    "session-456",
				CheckpointID: "checkpoint-789",
				Action:       CheckpointActionPassed,
				Points:       10,
			},
			wantErr: false,
		},
		{
			name: "session event",
			msg: &SessionEvent{
				MessageHeader: MessageHeader{
					ID:      "test-id",
					Source:  "test",
					Version: "1.0",
				},
				SessionID: "session-123",
				Action:    SessionActionStarted,
			},
			wantErr: false,
		},
		{
			name: "grade update",
			msg: &GradeUpdate{
				SessionID:    "session-123",
				EarnedPoints: 80,
				MaxPoints:    100,
				Percentage:   80.0,
				Passed:       true,
				RequiresSync: true,
			},
			wantErr: false,
		},
		{
			name:    "simple map",
			msg:     map[string]string{"key": "value"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ToJSON(tt.msg)

			if (err != nil) != tt.wantErr {
				t.Errorf("ToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify it's valid JSON by unmarshaling
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Errorf("ToJSON() produced invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestWebSocketMessage(t *testing.T) {
	msg := WebSocketMessage{
		Type:    "checkpoint",
		Subject: "labs.checkpoints.pod-123.enable-ufw",
		Payload: json.RawMessage(`{"status": "passed"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal WebSocketMessage: %v", err)
	}

	var decoded WebSocketMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal WebSocketMessage: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, msg.Type)
	}

	if decoded.Subject != msg.Subject {
		t.Errorf("Subject = %v, want %v", decoded.Subject, msg.Subject)
	}
}

func TestCheckpointBroadcast(t *testing.T) {
	now := time.Now()
	broadcast := CheckpointBroadcast{
		PodID:               "pod-123",
		SessionID:           "session-456",
		CheckpointID:        "enable-firewall",
		EarnedPoints:        10,
		Points:              10,
		SessionEarnedPoints: 50,
		SessionMaxPoints:    100,
		SessionPercentage:   50.0,
		SessionPassed:       false,
		Timestamp:           now,
	}

	data, err := json.Marshal(broadcast)
	if err != nil {
		t.Fatalf("Failed to marshal CheckpointBroadcast: %v", err)
	}

	var decoded CheckpointBroadcast
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CheckpointBroadcast: %v", err)
	}

	if decoded.CheckpointID != broadcast.CheckpointID {
		t.Errorf("CheckpointID = %v, want %v", decoded.CheckpointID, broadcast.CheckpointID)
	}

	if decoded.SessionPercentage != broadcast.SessionPercentage {
		t.Errorf("SessionPercentage = %v, want %v", decoded.SessionPercentage, broadcast.SessionPercentage)
	}
}

func TestGradeUpdate(t *testing.T) {
	syncedAt := time.Now()
	update := GradeUpdate{
		MessageHeader:      NewMessageHeader("grade-sync"),
		SessionID:          "session-123",
		UserID:             "user-456",
		EarnedPoints:       85,
		MaxPoints:          100,
		Percentage:         85.0,
		Passed:             true,
		CanvasAssignmentID: "canvas-assign-789",
		CanvasCourseID:     "canvas-course-101",
		CanvasUserID:       "canvas-user-202",
		RequiresSync:       true,
		SyncedAt:           &syncedAt,
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("Failed to marshal GradeUpdate: %v", err)
	}

	var decoded GradeUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal GradeUpdate: %v", err)
	}

	if decoded.SessionID != update.SessionID {
		t.Errorf("SessionID = %v, want %v", decoded.SessionID, update.SessionID)
	}

	if decoded.Percentage != update.Percentage {
		t.Errorf("Percentage = %v, want %v", decoded.Percentage, update.Percentage)
	}

	if decoded.RequiresSync != update.RequiresSync {
		t.Errorf("RequiresSync = %v, want %v", decoded.RequiresSync, update.RequiresSync)
	}
}
