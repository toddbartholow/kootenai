// Package achievements provides achievement and badge awarding functionality
package achievements

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Service handles achievement awarding logic
type Service struct {
	achievementRepo repositories.AchievementRepository
	sessionRepo     repositories.SessionRepository
	labTemplateRepo repositories.LabTemplateRepository
	enrollmentRepo  repositories.EnrollmentRepository
	pathwayRepo     repositories.PathwayRepository
	logger          *slog.Logger
}

// NewService creates a new achievement service
func NewService(
	achievementRepo repositories.AchievementRepository,
	sessionRepo repositories.SessionRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		achievementRepo: achievementRepo,
		sessionRepo:     sessionRepo,
		logger:          logger,
	}
}

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

// CheckAndAwardAchievements checks if a user earned any achievements from a session
func (s *Service) CheckAndAwardAchievements(ctx context.Context, session *models.Session, userID string) ([]*models.UserAchievement, error) {
	// Get all active achievements
	achievements, err := s.achievementRepo.ListAchievements(ctx, repositories.AchievementFilter{
		IsActive: boolPtr(true),
	})
	if err != nil {
		return nil, fmt.Errorf("listing achievements: %w", err)
	}

	var awarded []*models.UserAchievement

	for _, achievement := range achievements {
		// Check if already earned
		existing, err := s.achievementRepo.GetUserAchievement(ctx, userID, achievement.ID)
		if err != nil {
			s.logger.Error("Failed to check existing achievement", "error", err, "achievementId", achievement.ID)
			continue
		}
		if existing != nil {
			// Already earned
			continue
		}

		// Check if criteria met
		met, progress, err := s.checkCriteria(ctx, userID, session, achievement)
		if err != nil {
			s.logger.Error("Failed to check criteria", "error", err, "achievementId", achievement.ID)
			continue
		}

		// Update progress regardless of whether earned
		if progress != nil {
			if err := s.achievementRepo.UpdateProgress(ctx, progress); err != nil {
				s.logger.Error("Failed to update progress", "error", err, "achievementId", achievement.ID)
			}
		}

		if met {
			// Award the achievement
			userAchievement := &models.UserAchievement{
				ID:            fmt.Sprintf("ua-%s", uuid.New().String()),
				UserID:        userID,
				AchievementID: achievement.ID,
				Achievement:   achievement, // Include full achievement details for API response
				EarnedAt:      time.Now(),
				SessionID:     session.ID,
				Progress:      100.0,
				Notified:      false,
			}

			if err := s.achievementRepo.AwardAchievement(ctx, userAchievement); err != nil {
				s.logger.Error("Failed to award achievement", "error", err, "achievementId", achievement.ID)
				continue
			}

			s.logger.Info("Achievement awarded", "userId", userID, "achievementId", achievement.ID, "achievementName", achievement.Name)
			awarded = append(awarded, userAchievement)
		}
	}

	return awarded, nil
}

// checkCriteria checks if achievement criteria is met
func (s *Service) checkCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement) (bool, *models.AchievementProgress, error) {
	criteria := achievement.Criteria

	switch achievement.Type {
	case models.AchievementTypeLabCompletion, models.AchievementTypeMilestone:
		return s.checkLabCompletionCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypePerfectScore:
		return s.checkPerfectScoreCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypeSpeed:
		return s.checkSpeedCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypeStreak:
		return s.checkStreakCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypeCategory:
		return s.checkCategoryCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypeSpecial:
		return s.checkSpecialCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypePathway:
		return s.checkPathwayCriteria(ctx, userID, session, achievement, criteria)

	case models.AchievementTypePathwayComplete:
		return s.checkPathwayCompleteCriteria(ctx, userID, session, achievement, criteria)

	default:
		return false, nil, nil
	}
}

// checkLabCompletionCriteria checks if lab completion criteria is met
func (s *Service) checkLabCompletionCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	// Get total labs completed by user
	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
	})
	if err != nil {
		return false, nil, fmt.Errorf("listing sessions: %w", err)
	}

	// Count completed sessions
	completedCount := 0
	for _, sess := range sessions {
		if sess.EndedAt != nil && sess.Passed {
			completedCount++
		}
	}

	progress := &models.AchievementProgress{
		UserID:        userID,
		AchievementID: achievement.ID,
		Current:       completedCount,
		Required:      criteria.TotalLabs,
		Earned:        false,
	}

	if criteria.TotalLabs > 0 {
		progress.Progress = float64(completedCount) / float64(criteria.TotalLabs) * 100
		if progress.Progress > 100 {
			progress.Progress = 100
		}

		if completedCount >= criteria.TotalLabs {
			progress.Earned = true
			now := time.Now()
			progress.EarnedAt = &now
			return true, progress, nil
		}
	}

	return false, progress, nil
}

// checkPerfectScoreCriteria checks if perfect score criteria is met
func (s *Service) checkPerfectScoreCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if !criteria.RequirePerfect {
		return false, nil, nil
	}

	// Check if current session has perfect score
	if session.Percentage >= 100.0 {
		// If TotalLabs requirement exists, check how many perfect scores user has
		if criteria.TotalLabs > 0 {
			sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
				UserID: userID,
			})
			if err != nil {
				return false, nil, fmt.Errorf("listing sessions: %w", err)
			}

			perfectCount := 0
			for _, sess := range sessions {
				if sess.Percentage >= 100.0 {
					perfectCount++
				}
			}

			progress := &models.AchievementProgress{
				UserID:        userID,
				AchievementID: achievement.ID,
				Current:       perfectCount,
				Required:      criteria.TotalLabs,
				Progress:      float64(perfectCount) / float64(criteria.TotalLabs) * 100,
				Earned:        perfectCount >= criteria.TotalLabs,
			}

			if progress.Earned {
				now := time.Now()
				progress.EarnedAt = &now
			}

			return progress.Earned, progress, nil
		}

		// Just need one perfect score
		progress := &models.AchievementProgress{
			UserID:        userID,
			AchievementID: achievement.ID,
			Current:       1,
			Required:      1,
			Progress:      100,
			Earned:        true,
		}
		now := time.Now()
		progress.EarnedAt = &now
		return true, progress, nil
	}

	return false, nil, nil
}

// checkSpeedCriteria checks if speed criteria is met
func (s *Service) checkSpeedCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if criteria.MaxDurationMins <= 0 {
		return false, nil, nil
	}

	if session.EndedAt == nil {
		return false, nil, nil
	}

	duration := session.EndedAt.Sub(session.StartedAt)
	durationMins := int(duration.Minutes())

	if durationMins <= criteria.MaxDurationMins {
		progress := &models.AchievementProgress{
			UserID:        userID,
			AchievementID: achievement.ID,
			Current:       durationMins,
			Required:      criteria.MaxDurationMins,
			Progress:      100,
			Earned:        true,
			Metadata: map[string]any{
				"duration_minutes": durationMins,
			},
		}
		now := time.Now()
		progress.EarnedAt = &now
		return true, progress, nil
	}

	return false, nil, nil
}

// checkStreakCriteria checks if streak criteria is met
func (s *Service) checkStreakCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if criteria.StreakCount <= 0 {
		return false, nil, nil
	}

	// Get user's recent sessions ordered by completion
	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
		Limit:  100, // Look at recent history
	})
	if err != nil {
		return false, nil, fmt.Errorf("listing sessions: %w", err)
	}

	// Count current streak
	streak := 0
	for i := len(sessions) - 1; i >= 0; i-- {
		sess := sessions[i]
		if sess.EndedAt != nil && sess.Passed {
			streak++
		} else if sess.EndedAt != nil {
			// Failed session breaks streak
			break
		}
	}

	progress := &models.AchievementProgress{
		UserID:        userID,
		AchievementID: achievement.ID,
		Current:       streak,
		Required:      criteria.StreakCount,
		Progress:      float64(streak) / float64(criteria.StreakCount) * 100,
		Earned:        streak >= criteria.StreakCount,
		Metadata: map[string]any{
			"current_streak": streak,
		},
	}

	if progress.Progress > 100 {
		progress.Progress = 100
	}

	if progress.Earned {
		now := time.Now()
		progress.EarnedAt = &now
		return true, progress, nil
	}

	return false, progress, nil
}

// checkCategoryCriteria checks if category criteria is met
func (s *Service) checkCategoryCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if criteria.LabCategory == "" {
		return false, nil, nil
	}

	// Need lab template repository to look up templates by category
	if s.labTemplateRepo == nil {
		s.logger.Debug("Lab template repository not configured, skipping category achievement check")
		return false, nil, nil
	}

	// Get all active templates in this category
	active := true
	templates, err := s.labTemplateRepo.List(ctx, repositories.LabTemplateFilter{
		Category: criteria.LabCategory,
		Active:   &active,
	})
	if err != nil {
		return false, nil, fmt.Errorf("listing templates by category: %w", err)
	}

	if len(templates) == 0 {
		// No templates in this category
		return false, nil, nil
	}

	// Get user's completed sessions
	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
	})
	if err != nil {
		return false, nil, fmt.Errorf("listing user sessions: %w", err)
	}

	// Build set of completed template IDs
	completedTemplates := make(map[string]bool)
	for _, sess := range sessions {
		if sess.EndedAt != nil && sess.Passed {
			completedTemplates[sess.LabTemplateID] = true
		}
	}

	// Count how many category templates have been completed
	completedInCategory := 0
	for _, tmpl := range templates {
		if completedTemplates[tmpl.ID] {
			completedInCategory++
		}
	}

	totalInCategory := len(templates)
	progress := &models.AchievementProgress{
		UserID:        userID,
		AchievementID: achievement.ID,
		Current:       completedInCategory,
		Required:      totalInCategory,
		Progress:      float64(completedInCategory) / float64(totalInCategory) * 100,
		Earned:        completedInCategory >= totalInCategory,
		Metadata: map[string]any{
			"category":           criteria.LabCategory,
			"completedTemplates": completedInCategory,
			"totalTemplates":     totalInCategory,
		},
	}

	if progress.Earned {
		now := time.Now()
		progress.EarnedAt = &now
		return true, progress, nil
	}

	return false, progress, nil
}

// checkSpecialCriteria checks if special criteria is met
func (s *Service) checkSpecialCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if criteria.CustomRule == "" {
		return false, nil, nil
	}

	switch criteria.CustomRule {
	case "early_morning":
		// Completed before 6 AM
		if session.EndedAt != nil {
			hour := session.EndedAt.Hour()
			if hour < 6 {
				progress := &models.AchievementProgress{
					UserID:        userID,
					AchievementID: achievement.ID,
					Progress:      100,
					Earned:        true,
					Metadata: map[string]any{
						"completed_hour": hour,
					},
				}
				now := time.Now()
				progress.EarnedAt = &now
				return true, progress, nil
			}
		}

	case "late_night":
		// Completed after 10 PM
		if session.EndedAt != nil {
			hour := session.EndedAt.Hour()
			if hour >= 22 {
				progress := &models.AchievementProgress{
					UserID:        userID,
					AchievementID: achievement.ID,
					Progress:      100,
					Earned:        true,
					Metadata: map[string]any{
						"completed_hour": hour,
					},
				}
				now := time.Now()
				progress.EarnedAt = &now
				return true, progress, nil
			}
		}
	}

	return false, nil, nil
}

// checkPathwayCriteria checks if pathway module criteria is met (complete N modules in pathway)
func (s *Service) checkPathwayCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if s.enrollmentRepo == nil || s.pathwayRepo == nil {
		s.logger.Debug("Enrollment or pathway repository not configured, skipping pathway achievement check")
		return false, nil, nil
	}

	// Handle pathway_modules type from criteria.Type field
	if criteria.Type != "pathway_modules" && criteria.ModulesRequired <= 0 {
		return false, nil, nil
	}

	// Get pathway by slug or ID
	var pathway *models.Pathway
	var err error
	if criteria.PathwaySlug != "" {
		pathway, err = s.pathwayRepo.GetBySlug(ctx, criteria.PathwaySlug)
	} else if criteria.PathwayID != "" {
		pathway, err = s.pathwayRepo.GetByID(ctx, criteria.PathwayID)
	}
	if err != nil || pathway == nil {
		s.logger.Debug("Pathway not found for achievement", "achievementId", achievement.ID)
		return false, nil, nil
	}

	// Get user's enrollment in this pathway
	enrollment, err := s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathway.ID)
	if err != nil || enrollment == nil {
		// Not enrolled in this pathway
		return false, nil, nil
	}

	requiredModules := criteria.ModulesRequired
	if requiredModules <= 0 {
		requiredModules = enrollment.TotalModules
	}

	progress := &models.AchievementProgress{
		UserID:        userID,
		AchievementID: achievement.ID,
		Current:       enrollment.CompletedModules,
		Required:      requiredModules,
		Progress:      float64(enrollment.CompletedModules) / float64(requiredModules) * 100,
		Earned:        enrollment.CompletedModules >= requiredModules,
		Metadata: map[string]any{
			"pathway_id":        pathway.ID,
			"pathway_name":      pathway.Name,
			"completed_modules": enrollment.CompletedModules,
			"required_modules":  requiredModules,
		},
	}

	if progress.Progress > 100 {
		progress.Progress = 100
	}

	if progress.Earned {
		now := time.Now()
		progress.EarnedAt = &now
		return true, progress, nil
	}

	return false, progress, nil
}

// checkPathwayCompleteCriteria checks if pathway completion criteria is met
func (s *Service) checkPathwayCompleteCriteria(ctx context.Context, userID string, session *models.Session, achievement *models.Achievement, criteria models.AchievementCriteria) (bool, *models.AchievementProgress, error) {
	if s.enrollmentRepo == nil || s.pathwayRepo == nil {
		s.logger.Debug("Enrollment or pathway repository not configured, skipping pathway completion achievement check")
		return false, nil, nil
	}

	// Handle pathway_complete type
	if criteria.Type != "pathway_complete" && criteria.PathwayID == "" && criteria.PathwaySlug == "" {
		return false, nil, nil
	}

	// Get pathway by slug or ID
	var pathway *models.Pathway
	var err error
	if criteria.PathwaySlug != "" {
		pathway, err = s.pathwayRepo.GetBySlug(ctx, criteria.PathwaySlug)
	} else if criteria.PathwayID != "" {
		pathway, err = s.pathwayRepo.GetByID(ctx, criteria.PathwayID)
	}
	if err != nil || pathway == nil {
		s.logger.Debug("Pathway not found for achievement", "achievementId", achievement.ID)
		return false, nil, nil
	}

	// Get user's enrollment in this pathway
	enrollment, err := s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathway.ID)
	if err != nil || enrollment == nil {
		// Not enrolled in this pathway
		return false, nil, nil
	}

	isComplete := enrollment.Status == models.EnrollmentStatusCompleted ||
		(enrollment.TotalModules > 0 && enrollment.CompletedModules >= enrollment.TotalModules)

	progress := &models.AchievementProgress{
		UserID:        userID,
		AchievementID: achievement.ID,
		Current:       enrollment.CompletedModules,
		Required:      enrollment.TotalModules,
		Progress:      enrollment.Percentage,
		Earned:        isComplete,
		Metadata: map[string]any{
			"pathway_id":        pathway.ID,
			"pathway_name":      pathway.Name,
			"completed_modules": enrollment.CompletedModules,
			"total_modules":     enrollment.TotalModules,
			"status":            string(enrollment.Status),
		},
	}

	if progress.Earned {
		now := time.Now()
		progress.EarnedAt = &now
		return true, progress, nil
	}

	return false, progress, nil
}

// GetUserAchievements retrieves all achievements for a user with progress
func (s *Service) GetUserAchievements(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	return s.achievementRepo.ListAchievementsWithProgress(ctx, userID)
}

// GetUserSummary retrieves achievement summary for a user
func (s *Service) GetUserSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	return s.achievementRepo.GetUserAchievementSummary(ctx, userID)
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}
