package checkpoint

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Mock repositories for testing

type mockEventRepo struct {
	events []*models.Event
}

func (m *mockEventRepo) Create(ctx context.Context, event *models.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventRepo) GetByID(ctx context.Context, id int64) (*models.Event, error) {
	for _, e := range m.events {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, nil
}

func (m *mockEventRepo) GetByPodID(ctx context.Context, podID string, limit int) ([]*models.Event, error) {
	var result []*models.Event
	for _, e := range m.events {
		if e.PodID == podID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockEventRepo) GetBySessionID(ctx context.Context, sessionID string, limit int) ([]*models.Event, error) {
	var result []*models.Event
	for _, e := range m.events {
		if e.SessionID == sessionID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockEventRepo) GetUnprocessed(ctx context.Context, limit int) ([]*models.Event, error) {
	return nil, nil
}

func (m *mockEventRepo) MarkProcessed(ctx context.Context, id int64, matchedCheckpoints []string) error {
	return nil
}

func (m *mockEventRepo) Query(ctx context.Context, filter repositories.EventFilter) ([]*models.Event, error) {
	return m.events, nil
}

type mockSessionRepo struct {
	sessions map[string]*models.Session
}

func (m *mockSessionRepo) Create(ctx context.Context, session *models.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	return m.sessions[id], nil
}

func (m *mockSessionRepo) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	var result []*models.Session
	for _, s := range m.sessions {
		if s.PodID == podID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSessionRepo) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	var result []*models.Session
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status == "active" {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSessionRepo) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	return nil, nil
}

func (m *mockSessionRepo) Update(ctx context.Context, session *models.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionRepo) End(ctx context.Context, id string) error {
	if s, ok := m.sessions[id]; ok {
		s.Status = "ended"
	}
	return nil
}

func (m *mockSessionRepo) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	return nil
}

func (m *mockSessionRepo) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}

func (m *mockSessionRepo) MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error {
	return nil
}

func (m *mockSessionRepo) GetUserID(ctx context.Context, id string) (string, error) {
	if s, ok := m.sessions[id]; ok {
		return s.UserID, nil
	}
	return "", nil
}

func (m *mockSessionRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	if s, ok := m.sessions[id]; ok {
		return s.UserID == userID, nil
	}
	return false, nil
}

func (m *mockSessionRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}

func (m *mockSessionRepo) ListAll(ctx context.Context) ([]*models.Session, error) {
	var result []*models.Session
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSessionRepo) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockSessionRepo) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionRepo) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return &repositories.UserSessionStats{}, nil
}

func (m *mockSessionRepo) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (m *mockSessionRepo) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

type mockTemplateRepo struct {
	templates map[string]*models.LabTemplateRecord
}

func (m *mockTemplateRepo) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	m.templates[record.ID] = record
	return nil
}

func (m *mockTemplateRepo) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	return m.templates[id], nil
}

func (m *mockTemplateRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	var result []*models.LabTemplateRecord
	for _, id := range ids {
		if t, ok := m.templates[id]; ok {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTemplateRepo) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	for _, t := range m.templates {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockTemplateRepo) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	return nil, nil
}

func (m *mockTemplateRepo) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	m.templates[record.ID] = record
	return nil
}

func (m *mockTemplateRepo) Delete(ctx context.Context, id string) error {
	delete(m.templates, id)
	return nil
}

func (m *mockTemplateRepo) SetActive(ctx context.Context, id string, active bool) error {
	return nil
}
func (m *mockTemplateRepo) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}
func (m *mockTemplateRepo) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}
func (m *mockTemplateRepo) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}
func (m *mockTemplateRepo) CountVersions(_ context.Context, _ string) (int, error) { return 0, nil }

// Helper to create a LabTemplateRecord from a LabTemplate
func createTemplateRecord(id, name string, template *models.LabTemplate) *models.LabTemplateRecord {
	specBytes, _ := json.Marshal(template.Spec)
	checkpointsBytes, _ := json.Marshal(template.Spec.Objectives)

	return &models.LabTemplateRecord{
		ID:          id,
		Name:        name,
		Description: template.Metadata.Description,
		Version:     template.Metadata.Version,
		Spec:        specBytes,
		Checkpoints: checkpointsBytes,
	}
}

func TestNewReplayService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	service := NewReplayService(logger, evaluator, nil, nil, nil)

	if service == nil {
		t.Fatal("NewReplayService returned nil")
	}

	if service.evaluator == nil {
		t.Error("evaluator should be set")
	}
}

func TestReplayService_ReplaySession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	// Create mock template
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:          "install-nginx",
					Description: "Install nginx web server",
					Points:      10,
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypePackage,
							Target: "webserver",
							Match:  models.TriggerMatch{Package: "nginx", State: "installed"},
						},
					},
				},
				{
					ID:          "start-nginx",
					Description: "Start nginx service",
					Points:      5,
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypeService,
							Target: "webserver",
							Match:  models.TriggerMatch{Name: "nginx", State: "active"},
						},
					},
				},
			},
		},
	}

	// Create mock session
	session := &models.Session{
		ID:            "session-123",
		PodID:         "pod-456",
		UserID:        "user-789",
		LabTemplateID: "template-001",
		Status:        "active",
	}

	// Create mock events
	packageData, _ := json.Marshal(models.PackageData{Package: "nginx", Action: "install"})
	serviceData, _ := json.Marshal(models.ServiceData{Unit: "nginx", State: "active"})

	events := []*models.Event{
		{
			ID:        1,
			Timestamp: time.Now().Add(-2 * time.Minute),
			PodID:     "pod-456",
			SessionID: "session-123",
			VMName:    "webserver",
			EventType: "package",
			Data:      packageData,
		},
		{
			ID:        2,
			Timestamp: time.Now().Add(-1 * time.Minute),
			PodID:     "pod-456",
			SessionID: "session-123",
			VMName:    "webserver",
			EventType: "service",
			Data:      serviceData,
		},
	}

	// Create mock repos
	eventRepo := &mockEventRepo{events: events}
	sessionRepo := &mockSessionRepo{sessions: map[string]*models.Session{"session-123": session}}
	templateRecord := createTemplateRecord("template-001", "test-lab", template)
	templateRepo := &mockTemplateRepo{templates: map[string]*models.LabTemplateRecord{"template-001": templateRecord}}

	service := NewReplayService(logger, evaluator, eventRepo, sessionRepo, templateRepo)

	// Replay session
	result, err := service.ReplaySession(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("ReplaySession error: %v", err)
	}

	if result.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", result.SessionID)
	}

	if result.TotalEvents != 2 {
		t.Errorf("expected 2 total events, got %d", result.TotalEvents)
	}

	if result.ProcessedEvents != 2 {
		t.Errorf("expected 2 processed events, got %d", result.ProcessedEvents)
	}

	// Check checkpoint results
	if len(result.Checkpoints) != 2 {
		t.Errorf("expected 2 checkpoints, got %d", len(result.Checkpoints))
	}

	installNginx := result.Checkpoints["install-nginx"]
	if installNginx == nil {
		t.Fatal("install-nginx checkpoint result not found")
	}

	if installNginx.Status != "passed" {
		t.Errorf("expected install-nginx status 'passed', got %s", installNginx.Status)
	}

	startNginx := result.Checkpoints["start-nginx"]
	if startNginx == nil {
		t.Fatal("start-nginx checkpoint result not found")
	}

	if startNginx.Status != "passed" {
		t.Errorf("expected start-nginx status 'passed', got %s", startNginx.Status)
	}

	// Check timeline
	if len(result.Timeline) != 2 {
		t.Errorf("expected 2 timeline entries, got %d", len(result.Timeline))
	}
}

func TestReplayService_ReplaySessionNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	sessionRepo := &mockSessionRepo{sessions: make(map[string]*models.Session)}
	service := NewReplayService(logger, evaluator, nil, sessionRepo, nil)

	_, err := service.ReplaySession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestReplayService_ReplayEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	// Create mock template
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:          "install-nginx",
					Description: "Install nginx",
					Points:      10,
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

	session := &models.Session{
		ID:            "session-123",
		PodID:         "pod-456",
		UserID:        "user-789",
		LabTemplateID: "template-001",
		Status:        "active",
	}

	packageData, _ := json.Marshal(models.PackageData{Package: "nginx", Action: "install"})
	event := &models.Event{
		ID:        1,
		Timestamp: time.Now(),
		PodID:     "pod-456",
		SessionID: "session-123",
		VMName:    "webserver",
		EventType: "package",
		Data:      packageData,
	}

	eventRepo := &mockEventRepo{events: []*models.Event{event}}
	sessionRepo := &mockSessionRepo{sessions: map[string]*models.Session{"session-123": session}}
	templateRecord := createTemplateRecord("template-001", "test-lab", template)
	templateRepo := &mockTemplateRepo{templates: map[string]*models.LabTemplateRecord{"template-001": templateRecord}}

	service := NewReplayService(logger, evaluator, eventRepo, sessionRepo, templateRepo)

	detail, err := service.ReplayEvent(context.Background(), "session-123", 1)
	if err != nil {
		t.Fatalf("ReplayEvent error: %v", err)
	}

	if detail.EventID != 1 {
		t.Errorf("expected event ID 1, got %d", detail.EventID)
	}

	if detail.VMName != "webserver" {
		t.Errorf("expected VM name 'webserver', got %s", detail.VMName)
	}

	if detail.EventType != "package" {
		t.Errorf("expected event type 'package', got %s", detail.EventType)
	}

	if !detail.Matched {
		t.Error("expected event to be matched")
	}

	if len(detail.MatchedCheckpoints) != 1 || detail.MatchedCheckpoints[0] != "install-nginx" {
		t.Errorf("expected matched checkpoints [install-nginx], got %v", detail.MatchedCheckpoints)
	}
}

func TestReplayService_CompareReplays(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)
	service := NewReplayService(logger, evaluator, nil, nil, nil)

	original := &ReplayResult{
		SessionID:   "session-123",
		TotalEvents: 10,
		Checkpoints: map[string]*CheckpointReplayResult{
			"cp-1": {CheckpointID: "cp-1", Status: "passed"},
			"cp-2": {CheckpointID: "cp-2", Status: "pending"},
		},
	}

	replay := &ReplayResult{
		SessionID:   "session-123",
		TotalEvents: 12,
		Checkpoints: map[string]*CheckpointReplayResult{
			"cp-1": {CheckpointID: "cp-1", Status: "passed"},
			"cp-2": {CheckpointID: "cp-2", Status: "passed"},
		},
	}

	comparison := service.CompareReplays(original, replay)

	if comparison.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", comparison.SessionID)
	}

	if comparison.OriginalEvents != 10 {
		t.Errorf("expected 10 original events, got %d", comparison.OriginalEvents)
	}

	if comparison.ReplayedEvents != 12 {
		t.Errorf("expected 12 replayed events, got %d", comparison.ReplayedEvents)
	}

	if comparison.EventsDiff != 2 {
		t.Errorf("expected events diff 2, got %d", comparison.EventsDiff)
	}

	if !comparison.HasDifferences {
		t.Error("expected HasDifferences to be true")
	}

	// Check that cp-2 shows as different
	foundDiff := false
	for _, diff := range comparison.CheckpointDiffs {
		if diff.CheckpointID == "cp-2" && diff.Different {
			foundDiff = true
			break
		}
	}
	if !foundDiff {
		t.Error("expected cp-2 to be marked as different")
	}
}

func TestReplayService_GetCheckpointDiagnostics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	// Create mock template with a checkpoint that won't pass
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:          "install-nginx",
					Description: "Install nginx",
					Points:      10,
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

	session := &models.Session{
		ID:            "session-123",
		PodID:         "pod-456",
		UserID:        "user-789",
		LabTemplateID: "template-001",
		Status:        "active",
	}

	// No matching events - checkpoint should remain pending
	eventRepo := &mockEventRepo{events: []*models.Event{}}
	sessionRepo := &mockSessionRepo{sessions: map[string]*models.Session{"session-123": session}}
	templateRecord := createTemplateRecord("template-001", "test-lab", template)
	templateRepo := &mockTemplateRepo{templates: map[string]*models.LabTemplateRecord{"template-001": templateRecord}}

	service := NewReplayService(logger, evaluator, eventRepo, sessionRepo, templateRepo)

	diagnostics, err := service.GetCheckpointDiagnostics(context.Background(), "session-123", "install-nginx")
	if err != nil {
		t.Fatalf("GetCheckpointDiagnostics error: %v", err)
	}

	if diagnostics.CheckpointID != "install-nginx" {
		t.Errorf("expected checkpoint ID 'install-nginx', got %s", diagnostics.CheckpointID)
	}

	if diagnostics.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", diagnostics.Status)
	}

	if diagnostics.TotalTriggers != 1 {
		t.Errorf("expected 1 total trigger, got %d", diagnostics.TotalTriggers)
	}

	if diagnostics.MetTriggers != 0 {
		t.Errorf("expected 0 met triggers, got %d", diagnostics.MetTriggers)
	}

	if len(diagnostics.Recommendations) == 0 {
		t.Error("expected recommendations for unmet triggers")
	}
}

func TestTriggerMatchToMap(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)
	service := NewReplayService(logger, evaluator, nil, nil, nil)

	match := models.TriggerMatch{
		Path:         "/etc/nginx/nginx.conf",
		Contains:     "server_name",
		Package:      "nginx",
		State:        "installed",
		Port:         80,
		ThresholdPct: 80.0,
		Operator:     ">=",
	}

	result := service.triggerMatchToMap(match)

	if result["path"] != "/etc/nginx/nginx.conf" {
		t.Errorf("expected path '/etc/nginx/nginx.conf', got %v", result["path"])
	}

	if result["contains"] != "server_name" {
		t.Errorf("expected contains 'server_name', got %v", result["contains"])
	}

	if result["package"] != "nginx" {
		t.Errorf("expected package 'nginx', got %v", result["package"])
	}

	if result["port"] != 80 {
		t.Errorf("expected port 80, got %v", result["port"])
	}

	if result["thresholdPct"] != 80.0 {
		t.Errorf("expected thresholdPct 80.0, got %v", result["thresholdPct"])
	}
}

func TestReplayResult_Structure(t *testing.T) {
	result := &ReplayResult{
		SessionID:       "session-123",
		LabTemplate:     "test-lab",
		TotalEvents:     100,
		ProcessedEvents: 100,
		MatchedEvents:   50,
		Checkpoints:     make(map[string]*CheckpointReplayResult),
		Timeline:        make([]*EventReplayDetail, 0),
		Duration:        5 * time.Second,
	}

	if result.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", result.SessionID)
	}

	if result.TotalEvents != 100 {
		t.Errorf("expected 100 total events, got %d", result.TotalEvents)
	}

	if result.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", result.Duration)
	}
}

func TestCheckpointDiagnostics_Structure(t *testing.T) {
	diag := &CheckpointDiagnostics{
		CheckpointID:    "cp-1",
		Status:          "pending",
		TotalTriggers:   3,
		MetTriggers:     1,
		Triggers:        make([]TriggerReplayDetail, 3),
		RelevantEvents:  make([]*EventReplayDetail, 0),
		Recommendations: []string{"Try doing X", "Check Y"},
	}

	if diag.CheckpointID != "cp-1" {
		t.Errorf("expected checkpoint ID 'cp-1', got %s", diag.CheckpointID)
	}

	if diag.TotalTriggers != 3 {
		t.Errorf("expected 3 total triggers, got %d", diag.TotalTriggers)
	}

	if len(diag.Recommendations) != 2 {
		t.Errorf("expected 2 recommendations, got %d", len(diag.Recommendations))
	}
}

func (m *mockSessionRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
