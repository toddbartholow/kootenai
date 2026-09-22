package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// mockJetStreamMsg implements jetstream.Msg for testing
type mockJetStreamMsg struct {
	data []byte
}

func (m *mockJetStreamMsg) Data() []byte                              { return m.data }
func (m *mockJetStreamMsg) Headers() nats.Header                      { return nil }
func (m *mockJetStreamMsg) Subject() string                           { return "" }
func (m *mockJetStreamMsg) Reply() string                             { return "" }
func (m *mockJetStreamMsg) Ack() error                                { return nil }
func (m *mockJetStreamMsg) DoubleAck(ctx context.Context) error       { return nil }
func (m *mockJetStreamMsg) Nak() error                                { return nil }
func (m *mockJetStreamMsg) NakWithDelay(delay time.Duration) error    { return nil }
func (m *mockJetStreamMsg) InProgress() error                         { return nil }
func (m *mockJetStreamMsg) Term() error                               { return nil }
func (m *mockJetStreamMsg) TermWithReason(reason string) error        { return nil }
func (m *mockJetStreamMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }

// mockSessionRepo implements SessionRepository for testing
type mockSessionRepo struct {
	sessions map[string]*models.Session
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessions: make(map[string]*models.Session),
	}
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
		if s.UserID == userID && s.EndedAt == nil {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSessionRepo) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	var result []*models.Session
	for _, s := range m.sessions {
		if filter.UserID != "" && s.UserID != filter.UserID {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSessionRepo) Update(ctx context.Context, session *models.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionRepo) End(ctx context.Context, id string) error {
	if s, ok := m.sessions[id]; ok {
		now := time.Now()
		s.EndedAt = &now
	}
	return nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionRepo) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	if s, ok := m.sessions[id]; ok {
		s.EarnedPoints = earnedPoints
		s.Passed = passed
	}
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

func (m *mockSessionRepo) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return &repositories.UserSessionStats{}, nil
}

func (m *mockSessionRepo) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (m *mockSessionRepo) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

// mockEventRepo implements EventRepository for testing
type mockEventRepo struct {
	events      map[int64]*models.Event
	unprocessed []*models.Event
}

func newMockEventRepo() *mockEventRepo {
	return &mockEventRepo{
		events:      make(map[int64]*models.Event),
		unprocessed: make([]*models.Event, 0),
	}
}

func (m *mockEventRepo) Create(ctx context.Context, event *models.Event) error {
	m.events[event.ID] = event
	return nil
}

func (m *mockEventRepo) GetByID(ctx context.Context, id int64) (*models.Event, error) {
	return m.events[id], nil
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
	if limit > len(m.unprocessed) {
		limit = len(m.unprocessed)
	}
	return m.unprocessed[:limit], nil
}

func (m *mockEventRepo) MarkProcessed(ctx context.Context, id int64, matchedCheckpoints []string) error {
	if e, ok := m.events[id]; ok {
		e.Processed = true
		e.MatchedCheckpoints = matchedCheckpoints
	}
	return nil
}

func (m *mockEventRepo) Query(ctx context.Context, filter repositories.EventFilter) ([]*models.Event, error) {
	return nil, nil
}

func TestNewWorkerPool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	if wp == nil {
		t.Fatal("Expected worker pool to be created")
	}

	if wp.sessionRepo == nil {
		t.Error("Expected session repo to be set")
	}

	if wp.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestWorkerPoolWithEventRepo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger).
		WithEventRepo(eventRepo)

	if wp.eventRepo == nil {
		t.Error("Expected event repo to be set")
	}
}

func TestWorkerPoolStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	stats := wp.Stats()

	if !stats.Running {
		t.Error("Expected worker pool to be running")
	}

	if stats.ConsumersActive != 0 {
		t.Errorf("Expected 0 consumers, got %d", stats.ConsumersActive)
	}

	if stats.JobsProcessed != 0 {
		t.Errorf("Expected 0 jobs processed, got %d", stats.JobsProcessed)
	}
}

func TestWorkerPoolStatsAfterJobs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Simulate job completions
	wp.recordJobSuccess(100 * time.Millisecond)
	wp.recordJobSuccess(200 * time.Millisecond)
	wp.recordJobFailure()

	stats := wp.Stats()

	if stats.JobsProcessed != 2 {
		t.Errorf("Expected 2 jobs processed, got %d", stats.JobsProcessed)
	}

	if stats.JobsFailed != 1 {
		t.Errorf("Expected 1 job failed, got %d", stats.JobsFailed)
	}

	if stats.AverageLatencyMs != 150 {
		t.Errorf("Expected average latency 150ms, got %d", stats.AverageLatencyMs)
	}
}

func TestWorkerPoolStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Stop should not panic
	wp.Stop()

	stats := wp.Stats()
	if stats.Running {
		t.Error("Expected worker pool to be stopped")
	}
}

func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()

	if cfg.AchievementWorkers != 2 {
		t.Errorf("Expected 2 achievement workers, got %d", cfg.AchievementWorkers)
	}

	if cfg.EventWorkers != 2 {
		t.Errorf("Expected 2 event workers, got %d", cfg.EventWorkers)
	}
}

// mockAchievementRepo for testing - implements AchievementRepository
type mockAchievementRepo struct{}

func (m *mockAchievementRepo) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return nil
}

func (m *mockAchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) ListAchievements(ctx context.Context, filter repositories.AchievementFilter) ([]*models.Achievement, error) {
	return []*models.Achievement{}, nil
}

func (m *mockAchievementRepo) UpdateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return nil
}

func (m *mockAchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	return nil
}

func (m *mockAchievementRepo) AwardAchievement(ctx context.Context, ua *models.UserAchievement) error {
	return nil
}

func (m *mockAchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	return &models.UserAchievementSummary{}, nil
}

func (m *mockAchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	return nil
}

func (m *mockAchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	return nil
}

func (m *mockAchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	return nil, nil
}

func (m *mockAchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	return nil, nil
}

func (m *mockAchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	return nil, nil
}

func (m *mockAchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	return &models.Leaderboard{
		Entries:    []*models.LeaderboardEntry{},
		TotalUsers: 0,
		UpdatedAt:  time.Now(),
		TimeRange:  timeRange,
	}, nil
}

func TestWorkerPoolStartWithoutNATS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	achievementRepo := &mockAchievementRepo{}
	achievementSvc := achievements.NewService(achievementRepo, sessionRepo, logger)

	wp := NewWorkerPool(nil, achievementSvc, sessionRepo, logger)

	// Start without NATS should succeed (workers just won't start)
	err := wp.Start(DefaultWorkerConfig())
	if err != nil {
		t.Errorf("Expected no error starting without NATS, got: %v", err)
	}

	// Should have no consumers since NATS is nil
	stats := wp.Stats()
	if stats.ConsumersActive != 0 {
		t.Errorf("Expected 0 consumers without NATS, got %d", stats.ConsumersActive)
	}

	wp.Stop()
}

func TestWorkerPoolWithCheckpointEvaluator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Test WithCheckpointEvaluator returns the same pool (fluent API)
	result := wp.WithCheckpointEvaluator(nil)
	if result != wp {
		t.Error("Expected WithCheckpointEvaluator to return the same worker pool")
	}
}

func TestWorkerPoolStartWithZeroWorkers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Start with zero workers should succeed
	cfg := WorkerConfig{
		AchievementWorkers: 0,
		EventWorkers:       0,
	}
	err := wp.Start(cfg)
	if err != nil {
		t.Errorf("Expected no error starting with zero workers, got: %v", err)
	}

	stats := wp.Stats()
	if stats.ConsumersActive != 0 {
		t.Errorf("Expected 0 consumers with zero workers, got %d", stats.ConsumersActive)
	}

	wp.Stop()
}

func TestWorkerPoolRecordJobSuccessConcurrent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Simulate concurrent job completions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			wp.recordJobSuccess(100 * time.Millisecond)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := wp.Stats()
	if stats.JobsProcessed != 10 {
		t.Errorf("Expected 10 jobs processed, got %d", stats.JobsProcessed)
	}
}

func TestWorkerPoolRecordJobFailureConcurrent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Simulate concurrent job failures
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			wp.recordJobFailure()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	stats := wp.Stats()
	if stats.JobsFailed != 5 {
		t.Errorf("Expected 5 jobs failed, got %d", stats.JobsFailed)
	}
}

func TestWorkerPoolStatsZeroLatency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Stats with no jobs should have zero average latency
	stats := wp.Stats()
	if stats.AverageLatencyMs != 0 {
		t.Errorf("Expected 0 average latency with no jobs, got %d", stats.AverageLatencyMs)
	}
}

func TestWorkerPoolStartWithAchievementServiceNoNATS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	achievementRepo := &mockAchievementRepo{}
	achievementSvc := achievements.NewService(achievementRepo, sessionRepo, logger)

	// Even with achievement service configured, no consumers without NATS
	wp := NewWorkerPool(nil, achievementSvc, sessionRepo, logger)

	err := wp.Start(WorkerConfig{AchievementWorkers: 2, EventWorkers: 0})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	stats := wp.Stats()
	if stats.ConsumersActive != 0 {
		t.Errorf("Expected 0 consumers without NATS, got %d", stats.ConsumersActive)
	}

	wp.Stop()
}

func TestWorkerPoolStartWithEventRepoNoNATS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	// Even with event repo configured, no consumers without NATS
	wp := NewWorkerPool(nil, nil, sessionRepo, logger).WithEventRepo(eventRepo)

	err := wp.Start(WorkerConfig{AchievementWorkers: 0, EventWorkers: 2})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	stats := wp.Stats()
	if stats.ConsumersActive != 0 {
		t.Errorf("Expected 0 consumers without NATS, got %d", stats.ConsumersActive)
	}

	wp.Stop()
}

func TestWorkerPoolDoubleStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Double stop should not panic
	wp.Stop()
	wp.Stop()

	stats := wp.Stats()
	if stats.Running {
		t.Error("Expected worker pool to be stopped after double stop")
	}
}

func TestWorkerPoolStatsRunningAfterCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger)

	// Initially running
	stats := wp.Stats()
	if !stats.Running {
		t.Error("Expected worker pool to be running initially")
	}

	// After cancel
	wp.cancel()

	stats = wp.Stats()
	if stats.Running {
		t.Error("Expected worker pool to not be running after cancel")
	}
}

// -----------------------------------------------------------------------------
// Achievement Job Handler Tests
// -----------------------------------------------------------------------------

func TestHandleAchievementJobInvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	achievementRepo := &mockAchievementRepo{}
	achievementSvc := achievements.NewService(achievementRepo, sessionRepo, logger)

	wp := NewWorkerPool(nil, achievementSvc, sessionRepo, logger)

	// Invalid JSON should return error
	msg := &mockJetStreamMsg{data: []byte("invalid json")}
	err := wp.handleAchievementJob(context.Background(), msg)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleAchievementJobSessionNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	achievementRepo := &mockAchievementRepo{}
	achievementSvc := achievements.NewService(achievementRepo, sessionRepo, logger)

	wp := NewWorkerPool(nil, achievementSvc, sessionRepo, logger)

	// Create valid job but session doesn't exist
	job := events.AchievementEvaluationJob{
		SessionID: "non-existent-session",
		UserID:    "user-123",
	}
	jobData, _ := json.Marshal(job)
	msg := &mockJetStreamMsg{data: jobData}

	// Should return nil (session not found is not an error that should be retried)
	err := wp.handleAchievementJob(context.Background(), msg)
	if err != nil {
		t.Errorf("Expected nil for non-existent session, got: %v", err)
	}
}

func TestHandleAchievementJobSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	achievementRepo := &mockAchievementRepo{}
	achievementSvc := achievements.NewService(achievementRepo, sessionRepo, logger)

	// Add a session
	session := &models.Session{
		ID:            "session-123",
		UserID:        "user-123",
		LabTemplateID: "lab-1",
		PodID:         "pod-1",
		EarnedPoints:  100,
		MaxPoints:     100,
		Passed:        true,
	}
	sessionRepo.Create(context.Background(), session)

	wp := NewWorkerPool(nil, achievementSvc, sessionRepo, logger)

	// Create valid job
	job := events.AchievementEvaluationJob{
		SessionID: "session-123",
		UserID:    "user-123",
	}
	jobData, _ := json.Marshal(job)
	msg := &mockJetStreamMsg{data: jobData}

	err := wp.handleAchievementJob(context.Background(), msg)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Event Job Handler Tests
// -----------------------------------------------------------------------------

func TestHandleEventJobInvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger).WithEventRepo(eventRepo)

	// Invalid JSON should return error and record failure
	msg := &mockJetStreamMsg{data: []byte("invalid json")}
	err := wp.handleEventJob(context.Background(), msg)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	stats := wp.Stats()
	if stats.JobsFailed != 1 {
		t.Errorf("Expected 1 job failed, got %d", stats.JobsFailed)
	}
}

func TestHandleEventJobWithEventIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	// Add an event
	event := &models.Event{
		ID:        1,
		PodID:     "pod-123",
		SessionID: "session-123",
		VMName:    "vm-1",
		EventType: "file_created",
		Data:      json.RawMessage(`{"path": "/tmp/test"}`),
		Processed: false,
	}
	eventRepo.events[1] = event

	wp := NewWorkerPool(nil, nil, sessionRepo, logger).WithEventRepo(eventRepo)

	// Create job with specific event IDs
	job := events.EventProcessingJob{
		PodID:     "pod-123",
		SessionID: "session-123",
		EventIDs:  []int64{1},
	}
	jobData, _ := json.Marshal(job)
	msg := &mockJetStreamMsg{data: jobData}

	err := wp.handleEventJob(context.Background(), msg)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Event should be marked as processed
	if !eventRepo.events[1].Processed {
		t.Error("Expected event to be marked as processed")
	}

	stats := wp.Stats()
	if stats.JobsProcessed != 1 {
		t.Errorf("Expected 1 job processed, got %d", stats.JobsProcessed)
	}
}

func TestHandleEventJobWithNonExistentEventID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger).WithEventRepo(eventRepo)

	// Create job with non-existent event ID
	job := events.EventProcessingJob{
		PodID:    "pod-123",
		EventIDs: []int64{999},
	}
	jobData, _ := json.Marshal(job)
	msg := &mockJetStreamMsg{data: jobData}

	// Should succeed (non-existent events are skipped)
	err := wp.handleEventJob(context.Background(), msg)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestHandleEventJobWithUnprocessedEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	// Add unprocessed events
	event1 := &models.Event{
		ID:        1,
		PodID:     "pod-123",
		SessionID: "session-123",
		EventType: "file_created",
		Processed: false,
	}
	event2 := &models.Event{
		ID:        2,
		PodID:     "pod-456", // Different pod
		SessionID: "session-456",
		EventType: "command_executed",
		Processed: false,
	}
	eventRepo.events[1] = event1
	eventRepo.events[2] = event2
	eventRepo.unprocessed = []*models.Event{event1, event2}

	wp := NewWorkerPool(nil, nil, sessionRepo, logger).WithEventRepo(eventRepo)

	// Create job without event IDs (process unprocessed batch) but with pod filter
	job := events.EventProcessingJob{
		PodID:     "pod-123",
		BatchSize: 10,
	}
	jobData, _ := json.Marshal(job)
	msg := &mockJetStreamMsg{data: jobData}

	err := wp.handleEventJob(context.Background(), msg)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Only event1 should be processed (matches pod filter)
	if !eventRepo.events[1].Processed {
		t.Error("Expected event1 to be marked as processed")
	}
	// event2 should not be processed (different pod)
	if eventRepo.events[2].Processed {
		t.Error("Expected event2 to NOT be processed (different pod)")
	}
}

func TestHandleEventJobEmptyBatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sessionRepo := newMockSessionRepo()
	eventRepo := newMockEventRepo()

	wp := NewWorkerPool(nil, nil, sessionRepo, logger).WithEventRepo(eventRepo)

	// Create job without event IDs and empty unprocessed queue
	job := events.EventProcessingJob{
		PodID:     "pod-123",
		BatchSize: 10,
	}
	jobData, _ := json.Marshal(job)
	msg := &mockJetStreamMsg{data: jobData}

	err := wp.handleEventJob(context.Background(), msg)
	if err != nil {
		t.Errorf("Expected no error for empty batch, got: %v", err)
	}

	stats := wp.Stats()
	if stats.JobsProcessed != 1 {
		t.Errorf("Expected 1 job processed, got %d", stats.JobsProcessed)
	}
}

func (m *mockSessionRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
