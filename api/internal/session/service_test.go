package session

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Helper to create test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewService(t *testing.T) {
	logger := testLogger()

	// NewService accepts nil for testing basic creation
	svc := NewService(nil, nil, logger)

	assert.NotNil(t, svc)
	assert.Equal(t, logger, svc.logger)
	assert.Nil(t, svc.sessionRepo)
	assert.Nil(t, svc.checkpointRepo)
}

func TestService_BuilderPattern(t *testing.T) {
	logger := testLogger()

	svc := NewService(nil, nil, logger)

	// Test that builder methods return the service for chaining
	result := svc.WithLabTemplateRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithEnrollmentRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithPathwayRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithGradeSyncRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithEvaluator(nil)
	assert.Same(t, svc, result)

	result = svc.WithAchievementService(nil)
	assert.Same(t, svc, result)

	result = svc.WithNATSClient(nil)
	assert.Same(t, svc, result)

	result = svc.WithRedisService(nil)
	assert.Same(t, svc, result)
}

func TestService_Submit_NoSessionRepo(t *testing.T) {
	logger := testLogger()

	// Service with nil session repo
	svc := &Service{
		sessionRepo: nil,
		logger:      logger,
	}

	result, err := svc.Submit(t.Context(), "session-123")

	assert.ErrorIs(t, err, ErrSessionRepoNotAvailable)
	assert.Nil(t, result)
}

func TestCheckpointResult_Format(t *testing.T) {
	now := time.Now()
	result := CheckpointResult{
		ID:           "cp-1",
		Description:  "Test checkpoint",
		Points:       10,
		EarnedPoints: 10,
		Passed:       true,
		CompletedAt:  &now,
	}

	assert.Equal(t, "cp-1", result.ID)
	assert.Equal(t, "Test checkpoint", result.Description)
	assert.Equal(t, 10, result.Points)
	assert.Equal(t, 10, result.EarnedPoints)
	assert.True(t, result.Passed)
	assert.NotNil(t, result.CompletedAt)
}

func TestSubmitResult_Format(t *testing.T) {
	now := time.Now()
	result := SubmitResult{
		SessionID:           "session-123",
		Status:              "graded",
		EarnedPoints:        80,
		MaxPoints:           100,
		Percentage:          80.0,
		Passed:              true,
		PassThreshold:       70,
		Checkpoints:         []CheckpointResult{},
		Achievements:        nil,
		AchievementsPending: false,
		SubmittedAt:         now,
	}

	assert.Equal(t, "session-123", result.SessionID)
	assert.Equal(t, "graded", result.Status)
	assert.Equal(t, 80, result.EarnedPoints)
	assert.Equal(t, 100, result.MaxPoints)
	assert.Equal(t, 80.0, result.Percentage)
	assert.True(t, result.Passed)
	assert.Equal(t, 70, result.PassThreshold)
	assert.Empty(t, result.Checkpoints)
	assert.Nil(t, result.Achievements)
	assert.False(t, result.AchievementsPending)
}

func TestProgressResult_Format(t *testing.T) {
	result := ProgressResult{
		SessionID:    "session-123",
		EarnedPoints: 50,
		MaxPoints:    100,
		Percentage:   50.0,
		Checkpoints:  []CheckpointResult{},
	}

	assert.Equal(t, "session-123", result.SessionID)
	assert.Equal(t, 50, result.EarnedPoints)
	assert.Equal(t, 100, result.MaxPoints)
	assert.Equal(t, 50.0, result.Percentage)
	assert.Empty(t, result.Checkpoints)
}

func TestService_FormatCheckpoints(t *testing.T) {
	logger := testLogger()
	svc := &Service{logger: logger}

	now := time.Now()
	checkpoints := []checkpoint.CheckpointWithDescription{
		{
			CheckpointID: "cp-1",
			Description:  "First checkpoint",
			Status:       models.CheckpointStatusPassed,
			Points:       10,
			EarnedPoints: 10,
			PassedAt:     &now,
		},
		{
			CheckpointID: "cp-2",
			Description:  "Second checkpoint",
			Status:       models.CheckpointStatusPending,
			Points:       20,
			EarnedPoints: 0,
			PassedAt:     nil,
		},
	}

	result := svc.formatCheckpoints(checkpoints)

	assert.Len(t, result, 2)

	// First checkpoint - passed
	assert.Equal(t, "cp-1", result[0].ID)
	assert.Equal(t, "First checkpoint", result[0].Description)
	assert.True(t, result[0].Passed)
	assert.Equal(t, 10, result[0].EarnedPoints)
	assert.NotNil(t, result[0].CompletedAt)

	// Second checkpoint - pending
	assert.Equal(t, "cp-2", result[1].ID)
	assert.Equal(t, "Second checkpoint", result[1].Description)
	assert.False(t, result[1].Passed)
	assert.Equal(t, 0, result[1].EarnedPoints)
	assert.Nil(t, result[1].CompletedAt)
}

func TestService_FormatCheckpoints_Empty(t *testing.T) {
	logger := testLogger()
	svc := &Service{logger: logger}

	result := svc.formatCheckpoints(nil)

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestErrors(t *testing.T) {
	// Test that error messages are meaningful
	assert.Equal(t, "session not found", ErrSessionNotFound.Error())
	assert.Equal(t, "session already submitted", ErrSessionAlreadySubmitted.Error())
	assert.Equal(t, "session already ended", ErrSessionAlreadyEnded.Error())
	assert.Equal(t, "forbidden", ErrForbidden.Error())
	assert.Equal(t, "progress data unavailable", ErrProgressUnavailable.Error())
	assert.Equal(t, "invalid request parameters", ErrInvalidRequest.Error())
	assert.Equal(t, "session repository not available", ErrSessionRepoNotAvailable.Error())
}

// -----------------------------------------------------------------------------
// Additional Coverage Tests
// -----------------------------------------------------------------------------

func TestService_GetByID_NoSessionRepo(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		sessionRepo: nil,
		logger:      logger,
	}

	result, err := svc.GetByID(t.Context(), "session-123")

	assert.ErrorIs(t, err, ErrSessionRepoNotAvailable)
	assert.Nil(t, result)
}

func TestService_GetProgress_NoEvaluator(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		evaluator: nil,
		logger:    logger,
	}

	result, err := svc.GetProgress(t.Context(), "session-123")

	assert.ErrorIs(t, err, ErrProgressUnavailable)
	assert.Nil(t, result)
}

func TestService_persistCheckpointProgress_NilRepo(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		checkpointRepo: nil,
		logger:         logger,
	}

	progress := &checkpoint.SessionState{
		Checkpoints: map[string]*checkpoint.CheckpointProgress{
			"cp-1": {CheckpointID: "cp-1", Status: models.CheckpointStatusPassed},
		},
	}

	// Should return nil (no-op) when repo is nil
	err := svc.persistCheckpointProgress(t.Context(), "session-123", progress)
	assert.NoError(t, err)
}

func TestService_queueGradeSync_NoCanvasAssignment(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		logger: logger,
	}

	session := &models.Session{
		ID:                 "session-123",
		CanvasAssignmentID: "", // No Canvas assignment
	}

	// Should return nil (no-op) when no Canvas assignment
	err := svc.queueGradeSync(t.Context(), session, 80, 100, 80.0)
	assert.NoError(t, err)
}

func TestService_queueGradeSync_NoGradeSyncRepo(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		gradeSyncRepo: nil,
		logger:        logger,
	}

	session := &models.Session{
		ID:                 "session-123",
		CanvasAssignmentID: "canvas-assignment-456",
	}

	// Should return nil (no-op) when repo is nil
	err := svc.queueGradeSync(t.Context(), session, 80, 100, 80.0)
	assert.NoError(t, err)
}

func TestService_updatePathwayProgress_NilEnrollmentID(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		logger: logger,
	}

	session := &models.Session{
		ID:           "session-123",
		EnrollmentID: nil, // No enrollment
	}

	// Should return nil (no-op) when no enrollment
	mc, err := svc.updatePathwayProgress(t.Context(), session, "session-123", 80, true)
	assert.NoError(t, err)
	assert.Nil(t, mc)
}

func TestService_updatePathwayProgress_NilModuleID(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		logger: logger,
	}

	enrollmentID := "enrollment-123"
	session := &models.Session{
		ID:           "session-123",
		EnrollmentID: &enrollmentID,
		ModuleID:     nil, // No module
	}

	// Should return nil (no-op) when no module
	mc, err := svc.updatePathwayProgress(t.Context(), session, "session-123", 80, true)
	assert.NoError(t, err)
	assert.Nil(t, mc)
}

func TestService_updatePathwayProgress_NilEnrollmentRepo(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		enrollmentRepo: nil,
		logger:         logger,
	}

	enrollmentID := "enrollment-123"
	moduleID := "module-456"
	session := &models.Session{
		ID:           "session-123",
		EnrollmentID: &enrollmentID,
		ModuleID:     &moduleID,
	}

	// Should return nil (no-op) when repo is nil
	mc, err := svc.updatePathwayProgress(t.Context(), session, "session-123", 80, true)
	assert.NoError(t, err)
	assert.Nil(t, mc)
}

func TestService_processAchievements_NilService(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		achievementSvc: nil,
		logger:         logger,
	}

	session := &models.Session{
		ID:     "session-123",
		UserID: "user-456",
	}

	achievements, pending := svc.processAchievements(t.Context(), session, "session-123", 80, 100, 80.0)

	assert.Nil(t, achievements)
	assert.False(t, pending)
}

func TestService_invalidateCaches_NilRedisService(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		redisService: nil,
		logger:       logger,
	}

	session := &models.Session{
		ID:     "session-123",
		UserID: "user-456",
	}

	// Should not panic when Redis is nil
	svc.invalidateCaches(t.Context(), session)
}

func TestSubmitResult_WithAchievements(t *testing.T) {
	now := time.Now()
	achievements := []*models.UserAchievement{
		{
			ID:            "ua-1",
			UserID:        "user-123",
			AchievementID: "ach-1",
			EarnedAt:      now,
		},
	}

	result := SubmitResult{
		SessionID:           "session-123",
		Status:              "graded",
		EarnedPoints:        100,
		MaxPoints:           100,
		Percentage:          100.0,
		Passed:              true,
		PassThreshold:       70,
		Checkpoints:         []CheckpointResult{},
		Achievements:        achievements,
		AchievementsPending: false,
		SubmittedAt:         now,
	}

	assert.Len(t, result.Achievements, 1)
	assert.Equal(t, "ua-1", result.Achievements[0].ID)
}

func TestSubmitResult_AchievementsPending(t *testing.T) {
	now := time.Now()

	result := SubmitResult{
		SessionID:           "session-123",
		Status:              "graded",
		EarnedPoints:        100,
		MaxPoints:           100,
		Percentage:          100.0,
		Passed:              true,
		PassThreshold:       70,
		Checkpoints:         []CheckpointResult{},
		Achievements:        nil,
		AchievementsPending: true,
		SubmittedAt:         now,
	}

	assert.True(t, result.AchievementsPending)
	assert.Nil(t, result.Achievements)
}

func TestProgressResult_ZeroMaxPoints(t *testing.T) {
	result := ProgressResult{
		SessionID:    "session-123",
		EarnedPoints: 0,
		MaxPoints:    0,
		Percentage:   0.0,
		Checkpoints:  []CheckpointResult{},
	}

	assert.Equal(t, 0, result.MaxPoints)
	assert.Equal(t, 0.0, result.Percentage)
}

func TestCheckpointResult_NotPassed(t *testing.T) {
	result := CheckpointResult{
		ID:           "cp-1",
		Description:  "Test checkpoint",
		Points:       10,
		EarnedPoints: 0,
		Passed:       false,
		CompletedAt:  nil,
	}

	assert.False(t, result.Passed)
	assert.Equal(t, 0, result.EarnedPoints)
	assert.Nil(t, result.CompletedAt)
}
