// Package session provides session business logic separated from HTTP concerns
package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
	natsclient "github.com/toddbartholow/kootenai/api/internal/nats"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
)

// Service handles session business logic
type Service struct {
	sessionRepo     repositories.SessionRepository
	checkpointRepo  repositories.CheckpointProgressRepository
	labTemplateRepo repositories.LabTemplateRepository
	enrollmentRepo  repositories.EnrollmentRepository
	pathwayRepo     repositories.PathwayRepository
	gradeSyncRepo   repositories.GradeSyncRepository
	evaluator       *checkpoint.Evaluator
	achievementSvc  *achievements.Service
	natsClient      *natsclient.Client
	redisService    *redisclient.Service
	logger          *slog.Logger
}

// NewService creates a new session service with required dependencies
func NewService(
	sessionRepo repositories.SessionRepository,
	checkpointRepo repositories.CheckpointProgressRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		sessionRepo:    sessionRepo,
		checkpointRepo: checkpointRepo,
		logger:         logger,
	}
}

// Builder pattern for optional dependencies

// WithLabTemplateRepo adds a lab template repository to the service
func (s *Service) WithLabTemplateRepo(repo repositories.LabTemplateRepository) *Service {
	s.labTemplateRepo = repo
	return s
}

// WithEnrollmentRepo adds an enrollment repository to the service
func (s *Service) WithEnrollmentRepo(repo repositories.EnrollmentRepository) *Service {
	s.enrollmentRepo = repo
	return s
}

// WithPathwayRepo adds a pathway repository to the service
func (s *Service) WithPathwayRepo(repo repositories.PathwayRepository) *Service {
	s.pathwayRepo = repo
	return s
}

// WithGradeSyncRepo adds a grade sync repository to the service
func (s *Service) WithGradeSyncRepo(repo repositories.GradeSyncRepository) *Service {
	s.gradeSyncRepo = repo
	return s
}

// WithEvaluator adds a checkpoint evaluator to the service
func (s *Service) WithEvaluator(eval *checkpoint.Evaluator) *Service {
	s.evaluator = eval
	return s
}

// WithAchievementService adds an achievement service to the service
func (s *Service) WithAchievementService(svc *achievements.Service) *Service {
	s.achievementSvc = svc
	return s
}

// WithNATSClient adds a NATS client to the service
func (s *Service) WithNATSClient(client *natsclient.Client) *Service {
	s.natsClient = client
	return s
}

// WithRedisService adds a Redis service to the service
func (s *Service) WithRedisService(svc *redisclient.Service) *Service {
	s.redisService = svc
	return s
}

// Result types

// SubmitResult contains the result of submitting a session
type SubmitResult struct {
	SessionID           string                    `json:"sessionId"`
	Status              string                    `json:"status"`
	EarnedPoints        int                       `json:"earnedPoints"`
	MaxPoints           int                       `json:"maxPoints"`
	Percentage          float64                   `json:"percentage"`
	Passed              bool                      `json:"passed"`
	PassThreshold       int                       `json:"passThreshold"`
	Checkpoints         []CheckpointResult        `json:"checkpoints"`
	Achievements        []*models.UserAchievement `json:"achievements,omitempty"`
	AchievementsPending bool                      `json:"achievementsPending,omitempty"`
	ModuleCompletion    *ModuleCompletion         `json:"moduleCompletion,omitempty"`
	SubmittedAt         time.Time                 `json:"submittedAt"`
}

// CheckpointResult contains checkpoint progress details
type CheckpointResult struct {
	ID           string     `json:"id"`
	Description  string     `json:"description"`
	Points       int        `json:"points"`
	EarnedPoints int        `json:"earnedPoints"`
	Passed       bool       `json:"passed"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

// ModuleCompletion contains info about pathway progress after lab submission
type ModuleCompletion struct {
	ModuleID           string `json:"moduleId"`
	ModuleName         string `json:"moduleName"`
	PathwayID          string `json:"pathwayId"`
	PathwayName        string `json:"pathwayName"`
	UnlockedModuleName string `json:"unlockedModuleName,omitempty"`
	PathwayCompleted   bool   `json:"pathwayCompleted"`
}

// ProgressResult contains current session progress
type ProgressResult struct {
	SessionID    string             `json:"sessionId"`
	EarnedPoints int                `json:"earnedPoints"`
	MaxPoints    int                `json:"maxPoints"`
	Percentage   float64            `json:"percentage"`
	Checkpoints  []CheckpointResult `json:"checkpoints"`
}

// Submit submits a session and calculates the final grade
// This method extracts the business logic from handleSubmitSession
func (s *Service) Submit(ctx context.Context, sessionID string) (*SubmitResult, error) {
	if s.sessionRepo == nil {
		return nil, ErrSessionRepoNotAvailable
	}

	// Get session from database
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		s.logger.Error("Failed to get session", "error", err, "sessionId", sessionID)
		return nil, fmt.Errorf("getting session: %w", err)
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	// Check if already submitted
	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	if session.Metadata["status"] == "submitted" {
		return nil, ErrSessionAlreadySubmitted
	}

	// Get checkpoint progress from evaluator
	if s.evaluator == nil {
		return nil, ErrProgressUnavailable
	}

	progress, err := s.evaluator.GetSessionProgress(sessionID)
	if err != nil {
		s.logger.Error("Failed to get progress", "error", err, "sessionId", sessionID)
		return nil, fmt.Errorf("getting progress: %w", err)
	}

	// Get checkpoint progress with descriptions for the response
	checkpointsWithDesc, err := s.evaluator.GetSessionProgressWithDescriptions(sessionID)
	if err != nil {
		s.logger.Warn("Failed to get checkpoint descriptions", "error", err, "sessionId", sessionID)
		// Continue without descriptions - they're nice to have but not required
	}

	// Calculate final grade
	earnedPoints := progress.EarnedPoints
	maxPoints := progress.MaxPoints
	percentage := 0.0
	if maxPoints > 0 {
		percentage = float64(earnedPoints) / float64(maxPoints) * 100
	}

	// Determine if passed (using 70% threshold or from session passing_threshold)
	passThreshold := 70
	if session.PassingThreshold > 0 {
		passThreshold = session.PassingThreshold
	}
	passed := percentage >= float64(passThreshold)

	// Update session with final grade and submitted status
	session.EarnedPoints = earnedPoints
	session.Percentage = percentage
	session.Passed = passed
	session.Metadata["status"] = "submitted"

	if err := s.sessionRepo.Update(ctx, session); err != nil {
		s.logger.Error("Failed to update session", "error", err, "sessionId", sessionID)
		return nil, fmt.Errorf("updating session: %w", err)
	}

	// Persist checkpoint progress to database
	if err := s.persistCheckpointProgress(ctx, sessionID, progress); err != nil {
		s.logger.Error("Failed to persist checkpoint progress", "error", err, "sessionId", sessionID)
		// Don't fail the submission, just log the error
	}

	// Queue for Canvas grade sync
	if err := s.queueGradeSync(ctx, session, earnedPoints, maxPoints, percentage); err != nil {
		s.logger.Error("Failed to queue grade sync", "error", err, "sessionId", sessionID)
		// Don't fail the submission, just log the error
	}

	s.logger.Info("Session submitted", "sessionId", sessionID, "earnedPoints", earnedPoints, "maxPoints", maxPoints, "passed", passed)

	// Update pathway progress and get completion info
	var moduleCompletion *ModuleCompletion
	if mc, err := s.updatePathwayProgress(ctx, session, sessionID, earnedPoints, passed); err != nil {
		s.logger.Error("Failed to update pathway progress", "error", err, "sessionId", sessionID)
		// Don't fail the submission, just log the error
	} else {
		moduleCompletion = mc
	}

	// Check and award achievements
	var newAchievements []*models.UserAchievement
	var achievementsPending bool
	if passed {
		newAchievements, achievementsPending = s.processAchievements(ctx, session, sessionID, earnedPoints, maxPoints, percentage)
	}

	// Invalidate caches
	s.invalidateCaches(ctx, session)

	// Build result
	result := &SubmitResult{
		SessionID:           sessionID,
		Status:              "graded",
		EarnedPoints:        earnedPoints,
		MaxPoints:           maxPoints,
		Percentage:          percentage,
		Passed:              passed,
		PassThreshold:       passThreshold,
		Checkpoints:         s.formatCheckpoints(checkpointsWithDesc),
		Achievements:        newAchievements,
		AchievementsPending: achievementsPending,
		ModuleCompletion:    moduleCompletion,
		SubmittedAt:         time.Now(),
	}

	return result, nil
}

// persistCheckpointProgress saves checkpoint progress to the database
func (s *Service) persistCheckpointProgress(ctx context.Context, sessionID string, progress *checkpoint.SessionState) error {
	if s.checkpointRepo == nil {
		return nil
	}

	for _, cp := range progress.Checkpoints {
		existing, err := s.checkpointRepo.GetBySessionAndCheckpoint(ctx, sessionID, cp.CheckpointID)
		if err != nil {
			s.logger.Error("Failed to check checkpoint progress", "error", err)
			continue
		}
		if existing == nil {
			// Create new checkpoint progress record
			cpProgress := &models.CheckpointProgress{
				ID:           uuid.New().String(),
				SessionID:    sessionID,
				CheckpointID: cp.CheckpointID,
				Status:       cp.Status,
				Points:       cp.Points,
				EarnedPoints: cp.EarnedPoints,
				PassedAt:     cp.PassedAt,
				AttemptCount: 1, // First attempt
			}
			if err := s.checkpointRepo.Create(ctx, cpProgress); err != nil {
				s.logger.Error("Failed to create checkpoint progress", "error", err)
			}
		}
	}

	return nil
}

// queueGradeSync queues the grade for Canvas sync if applicable
func (s *Service) queueGradeSync(ctx context.Context, session *models.Session, earnedPoints, maxPoints int, percentage float64) error {
	if session.CanvasAssignmentID == "" || s.gradeSyncRepo == nil {
		return nil
	}

	syncEntry := &models.GradeSyncEntry{
		ID:                 fmt.Sprintf("sync-%s", uuid.New().String()),
		SessionID:          session.ID,
		Status:             "pending",
		EarnedPoints:       earnedPoints,
		MaxPoints:          maxPoints,
		Percentage:         percentage,
		CanvasCourseID:     session.CanvasCourseID,
		CanvasAssignmentID: session.CanvasAssignmentID,
		CanvasUserID:       session.CanvasUserID,
	}

	if err := s.gradeSyncRepo.Create(ctx, syncEntry); err != nil {
		return fmt.Errorf("creating grade sync entry: %w", err)
	}

	s.logger.Info("Queued grade sync", "sessionId", session.ID, "syncId", syncEntry.ID)
	return nil
}

// updatePathwayProgress updates pathway progress if the session is part of an enrollment
// Returns module completion info if the lab is part of a pathway
func (s *Service) updatePathwayProgress(ctx context.Context, session *models.Session, sessionID string, earnedPoints int, passed bool) (*ModuleCompletion, error) {
	if session.EnrollmentID == nil || session.ModuleID == nil || s.enrollmentRepo == nil {
		return nil, nil
	}

	// Record this lab attempt in pathway progress
	if err := s.enrollmentRepo.RecordLabAttempt(ctx,
		*session.EnrollmentID,
		*session.ModuleID,
		session.LabTemplateID,
		sessionID,
		earnedPoints,
		passed,
	); err != nil {
		return nil, fmt.Errorf("recording lab attempt: %w", err)
	}

	s.logger.Info("Recorded lab attempt for pathway", "sessionId", sessionID, "enrollmentId", *session.EnrollmentID, "moduleId", *session.ModuleID)

	// Recalculate module progress (this triggers DB cascade to enrollment)
	if err := s.enrollmentRepo.RecalculateModuleProgress(ctx, *session.EnrollmentID, *session.ModuleID); err != nil {
		s.logger.Warn("Failed to recalculate module progress", "error", err, "enrollmentId", *session.EnrollmentID, "moduleId", *session.ModuleID)
	}

	// Recalculate overall enrollment progress
	if err := s.enrollmentRepo.RecalculateEnrollmentProgress(ctx, *session.EnrollmentID); err != nil {
		s.logger.Warn("Failed to recalculate enrollment progress", "error", err, "enrollmentId", *session.EnrollmentID)
	}

	// Check if module completion unlocks any new modules
	unlockedModules, err := s.enrollmentRepo.UnlockNextModules(ctx, *session.EnrollmentID, *session.ModuleID)
	if err != nil {
		s.logger.Warn("Failed to unlock next modules", "error", err, "enrollmentId", *session.EnrollmentID, "moduleId", *session.ModuleID)
	} else if len(unlockedModules) > 0 {
		s.logger.Info("Unlocked next modules", "enrollmentId", *session.EnrollmentID, "unlockedModules", unlockedModules)
	}

	// Build module completion info for the response
	completion := s.buildModuleCompletion(ctx, session, unlockedModules)

	return completion, nil
}

// buildModuleCompletion creates completion info from session and unlocked modules
func (s *Service) buildModuleCompletion(ctx context.Context, session *models.Session, unlockedModules []string) *ModuleCompletion {
	if session.EnrollmentID == nil || session.ModuleID == nil {
		return nil
	}

	completion := &ModuleCompletion{
		ModuleID: *session.ModuleID,
	}

	// Get module info
	if s.pathwayRepo != nil {
		module, err := s.pathwayRepo.GetModuleByID(ctx, *session.ModuleID)
		if err != nil {
			s.logger.Warn("Failed to get module info", "error", err, "moduleId", *session.ModuleID)
		} else if module != nil {
			completion.ModuleName = module.Name
			completion.PathwayID = module.PathwayID

			// Get pathway info
			pathway, err := s.pathwayRepo.GetByID(ctx, module.PathwayID)
			if err != nil {
				s.logger.Warn("Failed to get pathway info", "error", err, "pathwayId", module.PathwayID)
			} else if pathway != nil {
				completion.PathwayName = pathway.Name
			}
		}

		// Get unlocked module name if any
		if len(unlockedModules) > 0 {
			unlockedModule, err := s.pathwayRepo.GetModuleByID(ctx, unlockedModules[0])
			if err != nil {
				s.logger.Warn("Failed to get unlocked module info", "error", err, "moduleId", unlockedModules[0])
			} else if unlockedModule != nil {
				completion.UnlockedModuleName = unlockedModule.Name
			}
		}
	}

	// Check if pathway is completed
	if s.enrollmentRepo != nil {
		enrollment, err := s.enrollmentRepo.GetByID(ctx, *session.EnrollmentID)
		if err != nil {
			s.logger.Warn("Failed to get enrollment", "error", err, "enrollmentId", *session.EnrollmentID)
		} else if enrollment != nil {
			completion.PathwayCompleted = enrollment.Status == models.EnrollmentStatusCompleted
		}
	}

	return completion
}

// processAchievements checks and awards achievements for a submitted session
func (s *Service) processAchievements(ctx context.Context, session *models.Session, sessionID string, earnedPoints, maxPoints int, percentage float64) ([]*models.UserAchievement, bool) {
	if s.achievementSvc == nil {
		return nil, false
	}

	var newAchievements []*models.UserAchievement
	var achievementsPending bool

	// Try async processing first if NATS is connected
	if s.natsClient != nil && s.natsClient.IsConnected() {
		job := events.NewAchievementJob(
			sessionID,
			session.UserID,
			session.LabTemplateID,
			session.Passed,
			percentage,
			earnedPoints,
			maxPoints,
		)
		job.EnrollmentID = session.EnrollmentID
		job.ModuleID = session.ModuleID

		if err := s.natsClient.PublishAchievementJob(ctx, job); err != nil {
			s.logger.Warn("Failed to queue achievement job, falling back to sync", "error", err, "sessionId", sessionID)
			// Fall back to synchronous evaluation
			achievements, err := s.achievementSvc.CheckAndAwardAchievements(ctx, session, session.UserID)
			if err != nil {
				s.logger.Error("Failed to check achievements", "error", err, "sessionId", sessionID)
			} else if len(achievements) > 0 {
				s.logger.Info("Achievements awarded (sync fallback)", "sessionId", sessionID, "count", len(achievements))
				newAchievements = achievements
			}
		} else {
			s.logger.Info("Achievement evaluation queued", "sessionId", sessionID, "jobId", job.ID)
			achievementsPending = true
		}
	} else {
		// No NATS - synchronous evaluation
		achievements, err := s.achievementSvc.CheckAndAwardAchievements(ctx, session, session.UserID)
		if err != nil {
			s.logger.Error("Failed to check achievements", "error", err, "sessionId", sessionID)
		} else if len(achievements) > 0 {
			s.logger.Info("Achievements awarded", "sessionId", sessionID, "count", len(achievements))
			newAchievements = achievements
		}
	}

	return newAchievements, achievementsPending
}

// invalidateCaches invalidates relevant caches after session submission
func (s *Service) invalidateCaches(ctx context.Context, session *models.Session) {
	if s.redisService == nil {
		return
	}

	// Invalidate dashboard cache for this user since session data changed
	if s.redisService.Dashboard != nil {
		if err := s.redisService.Dashboard.InvalidateUserDashboard(ctx, session.UserID); err != nil {
			s.logger.Warn("Failed to invalidate dashboard cache", "error", err, "userId", session.UserID)
		}
		// Also invalidate leaderboard since scores may have changed
		if err := s.redisService.Dashboard.InvalidateLeaderboard(ctx); err != nil {
			s.logger.Warn("Failed to invalidate leaderboard cache", "error", err)
		}
	}

	// Invalidate pathway cache if session was part of an enrollment
	if session.EnrollmentID != nil && s.redisService.Pathways != nil {
		if err := s.redisService.Pathways.InvalidateEnrollmentProgress(ctx, *session.EnrollmentID); err != nil {
			s.logger.Warn("Failed to invalidate enrollment progress cache", "error", err, "enrollmentId", *session.EnrollmentID)
		}
		if err := s.redisService.Pathways.InvalidateUserEnrollments(ctx, session.UserID); err != nil {
			s.logger.Warn("Failed to invalidate user enrollments cache", "error", err, "userId", session.UserID)
		}
	}
}

// formatCheckpoints converts checkpoint progress to result format
func (s *Service) formatCheckpoints(checkpoints []checkpoint.CheckpointWithDescription) []CheckpointResult {
	result := make([]CheckpointResult, 0, len(checkpoints))
	for _, cp := range checkpoints {
		result = append(result, CheckpointResult{
			ID:           cp.CheckpointID,
			Description:  cp.Description,
			Points:       cp.Points,
			EarnedPoints: cp.EarnedPoints,
			Passed:       cp.Status == models.CheckpointStatusPassed,
			CompletedAt:  cp.PassedAt,
		})
	}
	return result
}

// GetByID retrieves a session by ID
func (s *Service) GetByID(ctx context.Context, sessionID string) (*models.Session, error) {
	if s.sessionRepo == nil {
		return nil, ErrSessionRepoNotAvailable
	}

	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// GetProgress retrieves current progress for a session
func (s *Service) GetProgress(ctx context.Context, sessionID string) (*ProgressResult, error) {
	if s.evaluator == nil {
		return nil, ErrProgressUnavailable
	}

	progress, err := s.evaluator.GetSessionProgress(sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting progress: %w", err)
	}

	checkpointsWithDesc, err := s.evaluator.GetSessionProgressWithDescriptions(sessionID)
	if err != nil {
		s.logger.Warn("Failed to get checkpoint descriptions", "error", err, "sessionId", sessionID)
	}

	percentage := 0.0
	if progress.MaxPoints > 0 {
		percentage = float64(progress.EarnedPoints) / float64(progress.MaxPoints) * 100
	}

	return &ProgressResult{
		SessionID:    sessionID,
		EarnedPoints: progress.EarnedPoints,
		MaxPoints:    progress.MaxPoints,
		Percentage:   percentage,
		Checkpoints:  s.formatCheckpoints(checkpointsWithDesc),
	}, nil
}
