package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// mockEventRepository implements repositories.EventRepository for testing
type mockEventRepository struct {
	events            []*models.Event
	queryErr          error
	getByPodIDErr     error
	getBySessionIDErr error
}

func (r *mockEventRepository) Create(ctx context.Context, event *models.Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *mockEventRepository) GetByID(ctx context.Context, id int64) (*models.Event, error) {
	for _, e := range r.events {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, nil
}

func (r *mockEventRepository) GetByPodID(ctx context.Context, podID string, limit int) ([]*models.Event, error) {
	if r.getByPodIDErr != nil {
		return nil, r.getByPodIDErr
	}
	var result []*models.Event
	for _, e := range r.events {
		if e.PodID == podID {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *mockEventRepository) GetBySessionID(ctx context.Context, sessionID string, limit int) ([]*models.Event, error) {
	if r.getBySessionIDErr != nil {
		return nil, r.getBySessionIDErr
	}
	var result []*models.Event
	for _, e := range r.events {
		if e.SessionID == sessionID {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *mockEventRepository) GetUnprocessed(ctx context.Context, limit int) ([]*models.Event, error) {
	var result []*models.Event
	for _, e := range r.events {
		if !e.Processed {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *mockEventRepository) MarkProcessed(ctx context.Context, id int64, matchedCheckpoints []string) error {
	for _, e := range r.events {
		if e.ID == id {
			e.Processed = true
			e.MatchedCheckpoints = matchedCheckpoints
			break
		}
	}
	return nil
}

func (r *mockEventRepository) Query(ctx context.Context, filter repositories.EventFilter) ([]*models.Event, error) {
	if r.queryErr != nil {
		return nil, r.queryErr
	}
	return r.events, nil
}

func TestHandleWazuhWebhook(t *testing.T) {
	t.Run("valid alert", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		// Sample Wazuh alert format
		alert := map[string]interface{}{
			"id":        "12345",
			"timestamp": "2024-01-15T10:30:00.000Z",
			"rule": map[string]interface{}{
				"id":          "550",
				"description": "File added to the system",
				"level":       7,
			},
			"agent": map[string]interface{}{
				"id":   "001",
				"name": "lab-webserver",
				"ip":   "192.168.1.100",
			},
			"data": map[string]interface{}{
				"syscheck": map[string]interface{}{
					"path": "/etc/nginx/nginx.conf",
				},
			},
			"location": "syscheck",
		}

		body, _ := json.Marshal(alert)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "received" {
			t.Errorf("expected status 'received', got '%v'", response["status"])
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", nil)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("alert with session evaluates checkpoints", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Register a template with a file checkpoint
		template := &models.LabTemplate{
			Metadata: models.LabMetadata{Name: "nginx-lab"},
			Spec: models.LabSpec{
				Objectives: []models.Checkpoint{
					{
						ID:          "cp-nginx-config",
						Description: "Configure nginx",
						Points:      10,
						Triggers: []models.CheckpointTrigger{
							{
								Type: models.TriggerTypeFileExists,
								Match: models.TriggerMatch{
									Path: "/etc/nginx/nginx.conf",
								},
							},
						},
					},
				},
			},
		}
		eval.RegisterTemplate(template)
		eval.StartSession("session-123", "pod-456", "user-789", "nginx-lab")

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		// Alert with pod/session labels
		alert := map[string]interface{}{
			"id":        "12345",
			"timestamp": "2024-01-15T10:30:00.000Z",
			"rule": map[string]interface{}{
				"id":          "550",
				"description": "File added to the system",
				"level":       7,
			},
			"agent": map[string]interface{}{
				"id":   "001",
				"name": "pod-456-webserver",
				"ip":   "192.168.1.100",
				"labels": map[string]interface{}{
					"session_id": "session-123",
					"pod_id":     "pod-456",
				},
			},
			"data": map[string]interface{}{
				"syscheck": map[string]interface{}{
					"path": "/etc/nginx/nginx.conf",
				},
			},
			"location": "syscheck",
		}

		body, _ := json.Marshal(alert)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh?sessionId=session-123", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "received" {
			t.Errorf("expected status 'received', got '%v'", response["status"])
		}
	})

	t.Run("stores event with event repo", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		eventRepo := &mockEventRepository{}

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		alert := map[string]interface{}{
			"id":        "12345",
			"timestamp": "2024-01-15T10:30:00.000Z",
			"rule": map[string]interface{}{
				"id":          "550",
				"description": "File added to the system",
				"level":       7,
			},
			"agent": map[string]interface{}{
				"id":   "001",
				"name": "a1b2c3d4-e5f6-7890-abcd-ef1234567890-linux-vm",
				"ip":   "192.168.1.100",
				"labels": map[string]interface{}{
					"pod_id":  "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
					"vm_name": "linux-vm",
				},
			},
			"data": map[string]interface{}{
				"syscheck": map[string]interface{}{
					"path": "/etc/test.conf",
				},
			},
			"location": "syscheck",
		}

		body, _ := json.Marshal(alert)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		// Verify event was stored (requires pod ID extracted from agent name)
		if len(eventRepo.events) != 1 {
			t.Errorf("expected 1 event stored, got %d", len(eventRepo.events))
		}
	})

	t.Run("wrapped alert format", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		// Wrapped format from integration script
		wrappedAlert := map[string]interface{}{
			"alert": map[string]interface{}{
				"id":        "67890",
				"timestamp": "2024-01-15T11:00:00.000Z",
				"rule": map[string]interface{}{
					"id":          "550",
					"description": "File modified",
					"level":       5,
				},
				"agent": map[string]interface{}{
					"id":   "002",
					"name": "wrapped-agent",
					"ip":   "192.168.1.101",
				},
				"data": map[string]interface{}{
					"syscheck": map[string]interface{}{
						"path": "/etc/wrapped.conf",
					},
				},
				"location": "syscheck",
			},
		}

		body, _ := json.Marshal(wrappedAlert)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("finds session by pod ID from agent name", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		// Create session repo with a session
		sessionRepo := &mockSessionRepoForWebhook{
			sessions: []*models.Session{
				{
					ID:            "session-from-pod",
					UserID:        "user-1",
					PodID:         "ad78a3b9-235c-4213-8a91-b40c35953707",
					LabTemplateID: "lab-1",
				},
			},
		}

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithSessionRepo(sessionRepo))

		// Agent name contains UUID pod ID
		alert := map[string]interface{}{
			"id":        "12345",
			"timestamp": "2024-01-15T10:30:00.000Z",
			"rule": map[string]interface{}{
				"id":          "550",
				"description": "File added",
				"level":       7,
			},
			"agent": map[string]interface{}{
				"id":   "001",
				"name": "ad78a3b9-235c-4213-8a91-b40c35953707-linux-vm",
				"ip":   "192.168.1.100",
			},
			"data": map[string]interface{}{
				"syscheck": map[string]interface{}{
					"path": "/etc/test.conf",
				},
			},
			"location": "syscheck",
		}

		body, _ := json.Marshal(alert)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("no session warning", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())

		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		// Alert with agent name that doesn't have a valid UUID
		alert := map[string]interface{}{
			"id":        "12345",
			"timestamp": "2024-01-15T10:30:00.000Z",
			"rule": map[string]interface{}{
				"id":          "550",
				"description": "File added",
				"level":       7,
			},
			"agent": map[string]interface{}{
				"id":   "001",
				"name": "short-name", // No valid UUID
				"ip":   "192.168.1.100",
			},
			"data": map[string]interface{}{
				"syscheck": map[string]interface{}{
					"path": "/etc/test.conf",
				},
			},
			"location": "syscheck",
		}

		body, _ := json.Marshal(alert)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events/wazuh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should still succeed, just with no session
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

// mockSessionRepoForWebhook provides session lookup for webhook tests
type mockSessionRepoForWebhook struct {
	sessions []*models.Session
}

func (r *mockSessionRepoForWebhook) Create(ctx context.Context, session *models.Session) error {
	return nil
}

func (r *mockSessionRepoForWebhook) GetByID(ctx context.Context, id string) (*models.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (r *mockSessionRepoForWebhook) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	var result []*models.Session
	for _, s := range r.sessions {
		if s.PodID == podID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *mockSessionRepoForWebhook) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepoForWebhook) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	if filter.PodID != "" {
		var result []*models.Session
		for _, s := range r.sessions {
			if s.PodID == filter.PodID {
				result = append(result, s)
			}
		}
		return result, nil
	}
	return r.sessions, nil
}

func (r *mockSessionRepoForWebhook) ListAll(ctx context.Context) ([]*models.Session, error) {
	return r.sessions, nil
}

func (r *mockSessionRepoForWebhook) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForWebhook) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForWebhook) Update(ctx context.Context, session *models.Session) error {
	return nil
}

func (r *mockSessionRepoForWebhook) End(ctx context.Context, id string) error {
	return nil
}

func (r *mockSessionRepoForWebhook) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *mockSessionRepoForWebhook) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	return nil
}

func (r *mockSessionRepoForWebhook) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}

func (r *mockSessionRepoForWebhook) MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error {
	return nil
}

func (r *mockSessionRepoForWebhook) GetUserID(ctx context.Context, id string) (string, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s.UserID, nil
		}
	}
	return "", nil
}

func (r *mockSessionRepoForWebhook) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s.UserID == userID, nil
		}
	}
	return false, nil
}

func (r *mockSessionRepoForWebhook) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}

func (r *mockSessionRepoForWebhook) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return nil, nil
}

func (r *mockSessionRepoForWebhook) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (r *mockSessionRepoForWebhook) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// isValidUUID Tests
// -----------------------------------------------------------------------------

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid UUID lowercase",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: true,
		},
		{
			name:     "valid UUID uppercase",
			input:    "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			expected: true,
		},
		{
			name:     "valid UUID mixed case",
			input:    "a1B2c3D4-e5F6-7890-AbCd-EF1234567890",
			expected: true,
		},
		{
			name:     "all zeros",
			input:    "00000000-0000-0000-0000-000000000000",
			expected: true,
		},
		{
			name:     "all f's",
			input:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
			expected: true,
		},
		{
			name:     "too short",
			input:    "a1b2c3d4-e5f6-7890-abcd",
			expected: false,
		},
		{
			name:     "too long",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890x",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "missing first dash",
			input:    "a1b2c3d4e5f6-7890-abcd-ef1234567890",
			expected: false,
		},
		{
			name:     "missing second dash",
			input:    "a1b2c3d4-e5f67890-abcd-ef1234567890",
			expected: false,
		},
		{
			name:     "missing third dash",
			input:    "a1b2c3d4-e5f6-7890abcd-ef1234567890",
			expected: false,
		},
		{
			name:     "missing fourth dash",
			input:    "a1b2c3d4-e5f6-7890-abcdef1234567890",
			expected: false,
		},
		{
			name:     "invalid character g",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef123456789g",
			expected: false,
		},
		{
			name:     "invalid character space",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef123456789 ",
			expected: false,
		},
		{
			name:     "dashes in wrong places",
			input:    "-1b2c3d4-e5f6-7890-abcd-ef123456789a",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUUID(tt.input)
			if result != tt.expected {
				t.Errorf("isValidUUID(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// extractPodIDFromAgentName Tests
// -----------------------------------------------------------------------------

func TestExtractPodIDFromAgentName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid pod ID with VM name",
			input:    "ad78a3b9-235c-4213-8a91-b40c35953707-linux-vm",
			expected: "ad78a3b9-235c-4213-8a91-b40c35953707",
		},
		{
			name:     "just UUID",
			input:    "ad78a3b9-235c-4213-8a91-b40c35953707",
			expected: "ad78a3b9-235c-4213-8a91-b40c35953707",
		},
		{
			name:     "UUID with simple suffix",
			input:    "ad78a3b9-235c-4213-8a91-b40c35953707-vm1",
			expected: "ad78a3b9-235c-4213-8a91-b40c35953707",
		},
		{
			name:     "too short",
			input:    "short-name",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid UUID at start",
			input:    "not-a-valid-uuid-but-is-36-chars-xx",
			expected: "",
		},
		{
			name:     "35 characters",
			input:    "d78a3b9-235c-4213-8a91-b40c35953707",
			expected: "",
		},
		{
			name:     "UUID with uppercase",
			input:    "AD78A3B9-235C-4213-8A91-B40C35953707-vm-name",
			expected: "AD78A3B9-235C-4213-8A91-B40C35953707",
		},
		{
			name:     "all zeros UUID",
			input:    "00000000-0000-0000-0000-000000000000-test",
			expected: "00000000-0000-0000-0000-000000000000",
		},
		{
			name:     "UUID embedded in longer name with multiple hyphens",
			input:    "ad78a3b9-235c-4213-8a91-b40c35953707-vm-with-multiple-hyphens",
			expected: "ad78a3b9-235c-4213-8a91-b40c35953707",
		},
		{
			name:     "exactly 36 chars but invalid format",
			input:    "123456789012345678901234567890123456",
			expected: "",
		},
		{
			name:     "36 chars with wrong structure",
			input:    "xxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
			expected: "",
		},
		{
			name:     "long invalid name without UUID",
			input:    "this-is-a-long-string-but-no-valid-uuid-here-at-all",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPodIDFromAgentName(tt.input)
			if result != tt.expected {
				t.Errorf("extractPodIDFromAgentName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Events API Handler Tests
// -----------------------------------------------------------------------------

func TestHandleListEvents(t *testing.T) {
	t.Run("without event repo returns service unavailable", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("with event repo returns events", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{
					ID:            1,
					Timestamp:     time.Now(),
					PodID:         "pod-123",
					SessionID:     "session-456",
					VMName:        "server",
					EventType:     "syscheck",
					WazuhRuleID:   "550",
					WazuhLevel:    7,
					WazuhRuleDesc: "File added",
					Processed:     true,
				},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response EventsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Events) != 1 {
			t.Errorf("expected 1 event, got %d", len(response.Events))
		}
	})

	t.Run("with pagination parameters", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, EventType: "syscheck"},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events?limit=10&offset=5", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response EventsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Limit != 10 {
			t.Errorf("expected limit 10, got %d", response.Limit)
		}
		if response.Offset != 5 {
			t.Errorf("expected offset 5, got %d", response.Offset)
		}
	})

	t.Run("returns 500 on query error", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			queryErr: errors.New("database error"),
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	t.Run("filters by time range", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, EventType: "syscheck", Timestamp: time.Now()},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		start := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		end := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events?start="+start+"&end="+end, nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleEventStats(t *testing.T) {
	t.Run("without event repo returns empty stats", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/stats", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response EventStatsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.EventsByType == nil {
			t.Error("expected EventsByType to be initialized")
		}
	})

	t.Run("with event repo returns stats", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, EventType: "syscheck"},
				{ID: 2, EventType: "syscheck"},
				{ID: 3, EventType: "agent"},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/stats", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleSessionEvents(t *testing.T) {
	t.Run("without event repo returns service unavailable", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/session/session-123", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("with event repo returns session events", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, SessionID: "session-123", EventType: "syscheck"},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/session/session-123", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("returns 500 on database error", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			getBySessionIDErr: errors.New("database error"),
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/session/session-123", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	t.Run("respects custom limit parameter", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, SessionID: "session-123", EventType: "syscheck"},
				{ID: 2, SessionID: "session-123", EventType: "audit"},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/session/session-123?limit=1", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response EventsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Limit != 1 {
			t.Errorf("expected limit 1, got %d", response.Limit)
		}
	})
}

func TestHandlePodEvents(t *testing.T) {
	t.Run("without event repo returns service unavailable", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())
		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/pod/pod-123", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("with event repo returns pod events", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, PodID: "pod-123", EventType: "syscheck"},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/pod/pod-123", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("returns 500 on database error", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			getByPodIDErr: errors.New("database error"),
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/pod/pod-123", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	t.Run("respects custom limit parameter", func(t *testing.T) {
		eval := checkpoint.NewEvaluator(newTestLogger())
		cfg := DefaultConfig()
		wsHub := websocket.NewHub(newTestLogger())

		eventRepo := &mockEventRepository{
			events: []*models.Event{
				{ID: 1, PodID: "pod-123", EventType: "syscheck"},
				{ID: 2, PodID: "pod-123", EventType: "audit"},
			},
		}

		srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithEventRepo(eventRepo))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/events/pod/pod-123?limit=1", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response EventsResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Limit != 1 {
			t.Errorf("expected limit 1, got %d", response.Limit)
		}
	})
}

func (r *mockSessionRepoForWebhook) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
