// Package pathway provides the service layer for pathway enrollment and progress operations.
package pathway

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Service handles pathway business logic.
type Service struct {
	pathwayRepo    repositories.PathwayRepository
	enrollmentRepo repositories.EnrollmentRepository
	logger         *slog.Logger
}

// NewService creates a new pathway service.
func NewService(pathwayRepo repositories.PathwayRepository, enrollmentRepo repositories.EnrollmentRepository, logger *slog.Logger) *Service {
	return &Service{
		pathwayRepo:    pathwayRepo,
		enrollmentRepo: enrollmentRepo,
		logger:         logger,
	}
}

// EnrollResult contains the result of an enrollment operation.
type EnrollResult struct {
	Enrollment *models.PathwayEnrollment
	IsNew      bool
}

// Enroll enrolls a user in a pathway. Returns existing enrollment if already enrolled.
func (s *Service) Enroll(ctx context.Context, userID, pathwayIDOrSlug string) (*EnrollResult, error) {
	if s.pathwayRepo == nil || s.enrollmentRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Resolve pathway ID from slug if necessary
	pathwayID, err := s.resolvePathwayID(ctx, pathwayIDOrSlug)
	if err != nil {
		s.logger.Error("Failed to resolve pathway", "error", err, "pathwayIDOrSlug", pathwayIDOrSlug)
		return nil, err
	}
	if pathwayID == "" {
		return nil, ErrPathwayNotFound
	}

	// Check if already enrolled
	existing, err := s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathwayID)
	if err != nil {
		s.logger.Error("Failed to check existing enrollment", "error", err, "userID", userID, "pathwayID", pathwayID)
		return nil, err
	}
	if existing != nil {
		return &EnrollResult{Enrollment: existing, IsNew: false}, nil
	}

	// Create enrollment
	enrollment := &models.PathwayEnrollment{
		ID:        uuid.New().String(),
		UserID:    userID,
		PathwayID: pathwayID,
		Status:    models.EnrollmentStatusEnrolled,
	}

	if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
		s.logger.Error("Failed to create enrollment", "error", err, "userID", userID, "pathwayID", pathwayID)
		return nil, err
	}

	// Initialize module progress
	if err := s.enrollmentRepo.InitializeModuleProgress(ctx, enrollment.ID, pathwayID); err != nil {
		s.logger.Error("Failed to initialize module progress", "error", err, "enrollmentID", enrollment.ID)
		// Don't fail - enrollment succeeded
	}

	// Reload enrollment with totals
	enrollment, _ = s.enrollmentRepo.GetByID(ctx, enrollment.ID)

	s.logger.Info("User enrolled in pathway",
		"userID", userID,
		"pathwayID", pathwayID,
		"enrollmentID", enrollment.ID,
	)

	return &EnrollResult{Enrollment: enrollment, IsNew: true}, nil
}

// Unenroll removes a user's enrollment from a pathway.
func (s *Service) Unenroll(ctx context.Context, userID, pathwayIDOrSlug string) error {
	if s.pathwayRepo == nil || s.enrollmentRepo == nil {
		return ErrRepoNotAvailable
	}

	// Resolve pathway ID from slug if necessary
	pathwayID, err := s.resolvePathwayID(ctx, pathwayIDOrSlug)
	if err != nil {
		s.logger.Error("Failed to resolve pathway", "error", err, "pathwayIDOrSlug", pathwayIDOrSlug)
		return err
	}
	if pathwayID == "" {
		return ErrPathwayNotFound
	}

	enrollment, err := s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathwayID)
	if err != nil {
		s.logger.Error("Failed to get enrollment", "error", err, "userID", userID, "pathwayID", pathwayID)
		return err
	}
	if enrollment == nil {
		return ErrNotEnrolled
	}

	if err := s.enrollmentRepo.Delete(ctx, enrollment.ID); err != nil {
		s.logger.Error("Failed to delete enrollment", "error", err, "enrollmentID", enrollment.ID)
		return err
	}

	s.logger.Info("User unenrolled from pathway",
		"userID", userID,
		"pathwayID", pathwayID,
		"enrollmentID", enrollment.ID,
	)

	return nil
}

// ManualUnlockResult contains the result of a manual unlock operation.
type ManualUnlockResult struct {
	ModuleID     string `json:"moduleId"`
	EnrollmentID string `json:"enrollmentId"`
	Status       string `json:"status"`
}

// ManualUnlock manually unlocks a module for an enrollment (admin/instructor only).
func (s *Service) ManualUnlock(ctx context.Context, enrollmentID, moduleID string) (*ManualUnlockResult, error) {
	if s.pathwayRepo == nil || s.enrollmentRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Verify enrollment exists
	enrollment, err := s.enrollmentRepo.GetByID(ctx, enrollmentID)
	if err != nil {
		s.logger.Error("Failed to get enrollment", "error", err, "enrollmentID", enrollmentID)
		return nil, err
	}
	if enrollment == nil {
		return nil, ErrEnrollmentNotFound
	}

	// Verify module exists and belongs to the pathway
	module, err := s.pathwayRepo.GetModuleByID(ctx, moduleID)
	if err != nil {
		s.logger.Error("Failed to get module", "error", err, "moduleID", moduleID)
		return nil, err
	}
	if module == nil {
		return nil, ErrModuleNotFound
	}
	if module.PathwayID != enrollment.PathwayID {
		return nil, ErrModuleNotInPathway
	}

	// Unlock the module
	if err := s.enrollmentRepo.UnlockModule(ctx, enrollmentID, moduleID); err != nil {
		s.logger.Error("Failed to unlock module", "error", err, "enrollmentID", enrollmentID, "moduleID", moduleID)
		return nil, err
	}

	s.logger.Info("Module manually unlocked",
		"enrollmentID", enrollmentID,
		"moduleID", moduleID,
	)

	return &ManualUnlockResult{
		ModuleID:     moduleID,
		EnrollmentID: enrollmentID,
		Status:       "unlocked",
	}, nil
}

// UnlockRequirement describes what's needed to unlock a module.
type UnlockRequirement struct {
	ModuleID   string `json:"moduleId"`
	ModuleName string `json:"moduleName"`
	Status     string `json:"status"`
	Required   bool   `json:"required"`
}

// UnlockRequirementsResult contains the unlock requirements for a module.
type UnlockRequirementsResult struct {
	ModuleID      string              `json:"moduleId"`
	ModuleName    string              `json:"moduleName"`
	UnlockType    models.UnlockType   `json:"unlockType"`
	CurrentStatus string              `json:"currentStatus"`
	Requirements  []UnlockRequirement `json:"requirements"`
	Message       string              `json:"message"`
}

// GetUnlockRequirements returns what's needed to unlock a specific module.
func (s *Service) GetUnlockRequirements(ctx context.Context, enrollmentID, moduleID string) (*UnlockRequirementsResult, error) {
	if s.pathwayRepo == nil || s.enrollmentRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Get enrollment with progress
	enrollment, err := s.enrollmentRepo.GetWithProgress(ctx, enrollmentID)
	if err != nil {
		s.logger.Error("Failed to get enrollment", "error", err, "enrollmentID", enrollmentID)
		return nil, err
	}
	if enrollment == nil {
		return nil, ErrEnrollmentNotFound
	}

	// Get target module
	targetModule, err := s.pathwayRepo.GetModuleByID(ctx, moduleID)
	if err != nil {
		s.logger.Error("Failed to get module", "error", err, "moduleID", moduleID)
		return nil, err
	}
	if targetModule == nil {
		return nil, ErrModuleNotFound
	}

	// Get all modules in the pathway
	modules, err := s.pathwayRepo.ListModules(ctx, enrollment.PathwayID)
	if err != nil {
		s.logger.Error("Failed to list modules", "error", err, "pathwayID", enrollment.PathwayID)
		return nil, err
	}

	// Build a map of module progress
	progressMap := make(map[string]*models.ModuleProgress)
	for _, mp := range enrollment.ModuleProgress {
		progressMap[mp.ModuleID] = mp
	}

	// Get current status
	currentProgress := progressMap[moduleID]
	currentStatus := "locked"
	if currentProgress != nil {
		currentStatus = string(currentProgress.Status)
	}

	// Determine requirements based on unlock type
	var requirements []UnlockRequirement
	var message string

	switch targetModule.UnlockType {
	case models.UnlockTypeSequential:
		// Need the immediately previous module to be completed
		for _, mod := range modules {
			if mod.DisplayOrder == targetModule.DisplayOrder-1 {
				status := "locked"
				if p := progressMap[mod.ID]; p != nil {
					status = string(p.Status)
				}
				requirements = append(requirements, UnlockRequirement{
					ModuleID:   mod.ID,
					ModuleName: mod.Name,
					Status:     status,
					Required:   true,
				})
				if status == "completed" {
					message = "Previous module is complete. Module should unlock automatically."
				} else {
					message = "Complete the previous module to unlock this one."
				}
				break
			}
		}
		if len(requirements) == 0 {
			message = "This is the first module - no requirements."
		}

	case models.UnlockTypeAllPrevious:
		// Need all previous modules to be completed
		allComplete := true
		for _, mod := range modules {
			if mod.DisplayOrder < targetModule.DisplayOrder {
				status := "locked"
				if p := progressMap[mod.ID]; p != nil {
					status = string(p.Status)
				}
				if status != "completed" {
					allComplete = false
				}
				requirements = append(requirements, UnlockRequirement{
					ModuleID:   mod.ID,
					ModuleName: mod.Name,
					Status:     status,
					Required:   true,
				})
			}
		}
		if allComplete {
			message = "All previous modules are complete. Module should unlock automatically."
		} else {
			message = "Complete all previous modules to unlock this one."
		}

	case models.UnlockTypeManual:
		message = "This module requires instructor approval to unlock."

	case models.UnlockTypeAlways:
		message = "This module is always available."
	}

	return &UnlockRequirementsResult{
		ModuleID:      moduleID,
		ModuleName:    targetModule.Name,
		UnlockType:    targetModule.UnlockType,
		CurrentStatus: currentStatus,
		Requirements:  requirements,
		Message:       message,
	}, nil
}

// RecordLabCompletionResult contains the result of recording a lab completion.
type RecordLabCompletionResult struct {
	EnrollmentID    string   `json:"enrollmentId"`
	ModuleID        string   `json:"moduleId"`
	LabTemplateID   string   `json:"labTemplateId"`
	UnlockedModules []string `json:"unlockedModules,omitempty"`
}

// RecordLabCompletion records a lab attempt and updates pathway progress.
// This is called after a session is submitted.
func (s *Service) RecordLabCompletion(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) (*RecordLabCompletionResult, error) {
	if s.enrollmentRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Record the lab attempt
	if err := s.enrollmentRepo.RecordLabAttempt(ctx, enrollmentID, moduleID, labTemplateID, sessionID, score, passed); err != nil {
		s.logger.Error("Failed to record lab attempt",
			"error", err,
			"enrollmentID", enrollmentID,
			"moduleID", moduleID,
			"labTemplateID", labTemplateID,
		)
		return nil, err
	}

	// Recalculate module progress
	if err := s.enrollmentRepo.RecalculateModuleProgress(ctx, enrollmentID, moduleID); err != nil {
		s.logger.Error("Failed to recalculate module progress",
			"error", err,
			"enrollmentID", enrollmentID,
			"moduleID", moduleID,
		)
		// Don't fail - continue with other updates
	}

	// Recalculate enrollment progress
	if err := s.enrollmentRepo.RecalculateEnrollmentProgress(ctx, enrollmentID); err != nil {
		s.logger.Error("Failed to recalculate enrollment progress",
			"error", err,
			"enrollmentID", enrollmentID,
		)
		// Don't fail - continue with other updates
	}

	result := &RecordLabCompletionResult{
		EnrollmentID:  enrollmentID,
		ModuleID:      moduleID,
		LabTemplateID: labTemplateID,
	}

	// If passed, check for module completion and unlock next modules
	if passed {
		// Check if module is now completed
		moduleProgress, err := s.enrollmentRepo.GetModuleProgress(ctx, enrollmentID, moduleID)
		if err != nil {
			s.logger.Error("Failed to get module progress", "error", err, "enrollmentID", enrollmentID, "moduleID", moduleID)
		} else if moduleProgress != nil && moduleProgress.Status == models.ModuleStatusCompleted {
			// Unlock next modules
			unlockedIDs, err := s.enrollmentRepo.UnlockNextModules(ctx, enrollmentID, moduleID)
			if err != nil {
				s.logger.Error("Failed to unlock next modules", "error", err, "enrollmentID", enrollmentID, "moduleID", moduleID)
			} else if len(unlockedIDs) > 0 {
				result.UnlockedModules = unlockedIDs
				s.logger.Info("Modules unlocked after completion",
					"enrollmentID", enrollmentID,
					"completedModuleID", moduleID,
					"unlockedModules", unlockedIDs,
				)
			}
		}
	}

	return result, nil
}

// GetEnrollmentByUserAndPathway gets an enrollment for a user and pathway.
func (s *Service) GetEnrollmentByUserAndPathway(ctx context.Context, userID, pathwayIDOrSlug string) (*models.PathwayEnrollment, error) {
	if s.pathwayRepo == nil || s.enrollmentRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Resolve pathway ID from slug if necessary
	pathwayID, err := s.resolvePathwayID(ctx, pathwayIDOrSlug)
	if err != nil {
		return nil, err
	}
	if pathwayID == "" {
		return nil, ErrPathwayNotFound
	}

	return s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathwayID)
}

// resolvePathwayID resolves a pathway ID from either an ID or a slug.
func (s *Service) resolvePathwayID(ctx context.Context, idOrSlug string) (string, error) {
	// Try as ID first
	pathway, err := s.pathwayRepo.GetByID(ctx, idOrSlug)
	if err != nil {
		return "", err
	}
	if pathway != nil {
		return pathway.ID, nil
	}

	// Try as slug
	pathway, err = s.pathwayRepo.GetBySlug(ctx, idOrSlug)
	if err != nil {
		return "", err
	}
	if pathway != nil {
		return pathway.ID, nil
	}

	return "", nil
}
