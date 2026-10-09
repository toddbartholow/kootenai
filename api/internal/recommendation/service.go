// Package recommendation provides lab and pathway recommendation logic.
package recommendation

import (
	"context"
	"log/slog"
	"sort"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// RecommendationType indicates the reason for a recommendation
type RecommendationType string

const (
	// TypeNextInPathway - next lab in an enrolled pathway
	TypeNextInPathway RecommendationType = "next_in_pathway"
	// TypeContinueProgress - resume a partially completed lab
	TypeContinueProgress RecommendationType = "continue_progress"
	// TypeNewPathway - suggest a new pathway to start
	TypeNewPathway RecommendationType = "new_pathway"
	// TypeSimilarDifficulty - based on completed lab difficulty
	TypeSimilarDifficulty RecommendationType = "similar_difficulty"
	// TypePopular - popular/trending labs
	TypePopular RecommendationType = "popular"
)

// LabRecommendation represents a recommended lab
type LabRecommendation struct {
	LabTemplateID   string             `json:"labTemplateId"`
	LabName         string             `json:"labName"`
	LabSlug         string             `json:"labSlug"`
	LabDescription  string             `json:"labDescription,omitempty"`
	Difficulty      string             `json:"difficulty,omitempty"`
	DurationMinutes int                `json:"durationMinutes,omitempty"`
	MaxPoints       int                `json:"maxPoints,omitempty"`
	Type            RecommendationType `json:"type"`
	Reason          string             `json:"reason"`
	Priority        int                `json:"priority"` // Lower is higher priority

	// Context for pathway-based recommendations
	PathwayID   string `json:"pathwayId,omitempty"`
	PathwayName string `json:"pathwayName,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`
	ModuleName  string `json:"moduleName,omitempty"`
}

// PathwayRecommendation represents a recommended pathway
type PathwayRecommendation struct {
	PathwayID      string             `json:"pathwayId"`
	PathwayName    string             `json:"pathwayName"`
	PathwaySlug    string             `json:"pathwaySlug"`
	Description    string             `json:"description,omitempty"`
	Difficulty     string             `json:"difficulty,omitempty"`
	EstimatedHours int                `json:"estimatedHours,omitempty"`
	ModuleCount    int                `json:"moduleCount,omitempty"`
	LabCount       int                `json:"labCount,omitempty"`
	Type           RecommendationType `json:"type"`
	Reason         string             `json:"reason"`
	Priority       int                `json:"priority"`
	CoverImageURL  string             `json:"coverImageUrl,omitempty"`
	Icon           string             `json:"icon,omitempty"`
}

// RecommendationsResponse contains all recommendations for a user
type RecommendationsResponse struct {
	Labs     []LabRecommendation     `json:"labs"`
	Pathways []PathwayRecommendation `json:"pathways"`
}

// Service provides recommendation functionality
type Service struct {
	enrollmentRepo  repositories.EnrollmentRepository
	pathwayRepo     repositories.PathwayRepository
	labTemplateRepo repositories.LabTemplateRepository
	sessionRepo     repositories.SessionRepository
	logger          *slog.Logger
}

// NewService creates a new recommendation service
func NewService(
	enrollmentRepo repositories.EnrollmentRepository,
	pathwayRepo repositories.PathwayRepository,
	labTemplateRepo repositories.LabTemplateRepository,
	sessionRepo repositories.SessionRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		enrollmentRepo:  enrollmentRepo,
		pathwayRepo:     pathwayRepo,
		labTemplateRepo: labTemplateRepo,
		sessionRepo:     sessionRepo,
		logger:          logger,
	}
}

// GetRecommendations returns personalized recommendations for a user
func (s *Service) GetRecommendations(ctx context.Context, userID string, limit int) (*RecommendationsResponse, error) {
	if limit <= 0 {
		limit = 5
	}

	response := &RecommendationsResponse{
		Labs:     []LabRecommendation{},
		Pathways: []PathwayRecommendation{},
	}

	// Get labs from enrolled pathways
	if s.enrollmentRepo != nil && s.pathwayRepo != nil {
		pathwayLabs, err := s.getNextPathwayLabs(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to get pathway lab recommendations", "error", err, "userId", userID)
		} else {
			response.Labs = append(response.Labs, pathwayLabs...)
		}
	}

	// Get labs the user has started but not completed
	if s.sessionRepo != nil {
		inProgressLabs, err := s.getInProgressLabs(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to get in-progress lab recommendations", "error", err, "userId", userID)
		} else {
			response.Labs = append(response.Labs, inProgressLabs...)
		}
	}

	// Get pathway recommendations
	if s.pathwayRepo != nil && s.enrollmentRepo != nil {
		pathways, err := s.getPathwayRecommendations(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to get pathway recommendations", "error", err, "userId", userID)
		} else {
			response.Pathways = append(response.Pathways, pathways...)
		}
	}

	// Sort by priority
	sort.Slice(response.Labs, func(i, j int) bool {
		return response.Labs[i].Priority < response.Labs[j].Priority
	})
	sort.Slice(response.Pathways, func(i, j int) bool {
		return response.Pathways[i].Priority < response.Pathways[j].Priority
	})

	// Apply limit
	if len(response.Labs) > limit {
		response.Labs = response.Labs[:limit]
	}
	if len(response.Pathways) > limit {
		response.Pathways = response.Pathways[:limit]
	}

	return response, nil
}

// getNextPathwayLabs finds the next labs to complete in enrolled pathways
func (s *Service) getNextPathwayLabs(ctx context.Context, userID string) ([]LabRecommendation, error) {
	var recommendations []LabRecommendation

	// Get user's active enrollments
	enrollments, err := s.enrollmentRepo.List(ctx, models.EnrollmentListOptions{
		UserID: userID,
		Status: models.EnrollmentStatusInProgress,
		Limit:  10,
	})
	if err != nil {
		return nil, err
	}

	// Also get newly enrolled (not yet started)
	newEnrollments, err := s.enrollmentRepo.List(ctx, models.EnrollmentListOptions{
		UserID: userID,
		Status: models.EnrollmentStatusEnrolled,
		Limit:  5,
	})
	if err == nil {
		enrollments = append(enrollments, newEnrollments...)
	}

	for _, enrollment := range enrollments {
		// Get enrollment with progress
		enrollmentWithProgress, err := s.enrollmentRepo.GetWithProgress(ctx, enrollment.ID)
		if err != nil {
			continue
		}

		// Get pathway details
		pathway, err := s.pathwayRepo.GetWithModules(ctx, enrollment.PathwayID)
		if err != nil || pathway == nil {
			continue
		}

		// Find next unlocked lab in each module
		for _, moduleProgress := range enrollmentWithProgress.ModuleProgress {
			// Skip locked or completed modules
			if moduleProgress.Status == models.ModuleStatusLocked ||
				moduleProgress.Status == models.ModuleStatusCompleted {
				continue
			}

			// Find the module info
			var module *models.PathwayModule
			for _, m := range pathway.Modules {
				if m.ID == moduleProgress.ModuleID {
					module = m
					break
				}
			}
			if module == nil {
				continue
			}

			// Find first incomplete lab in this module
			for _, labProgress := range moduleProgress.LabProgress {
				if !labProgress.Passed {
					// This is a candidate
					var labInfo *models.ModuleLab
					for _, lab := range module.Labs {
						if lab.LabTemplateID == labProgress.LabTemplateID {
							labInfo = lab
							break
						}
					}
					if labInfo != nil {
						rec := LabRecommendation{
							LabTemplateID:   labProgress.LabTemplateID,
							LabName:         labInfo.LabName,
							LabDescription:  labInfo.LabDescription,
							Difficulty:      labInfo.LabDifficulty,
							DurationMinutes: labInfo.LabDuration,
							MaxPoints:       labInfo.LabMaxPoints,
							Type:            TypeNextInPathway,
							Reason:          "Continue your progress in " + pathway.Name,
							Priority:        1, // High priority
							PathwayID:       pathway.ID,
							PathwayName:     pathway.Name,
							ModuleID:        module.ID,
							ModuleName:      module.Name,
						}
						recommendations = append(recommendations, rec)
						break // Only one lab per module
					}
				}
			}
		}
	}

	return recommendations, nil
}

// getInProgressLabs finds labs the user has started but not completed
func (s *Service) getInProgressLabs(ctx context.Context, userID string) ([]LabRecommendation, error) {
	var recommendations []LabRecommendation

	// Get active sessions (not yet submitted)
	active := true
	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
		Active: &active,
		Limit:  5,
	})
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.Status == "active" || session.Status == "paused" {
			rec := LabRecommendation{
				LabTemplateID: session.LabTemplateID,
				Type:          TypeContinueProgress,
				Reason:        "Continue where you left off",
				Priority:      0, // Highest priority - active session
			}

			// Try to get lab template details
			if s.labTemplateRepo != nil {
				template, err := s.labTemplateRepo.GetByID(ctx, session.LabTemplateID)
				if err == nil && template != nil {
					rec.LabName = template.Name
					rec.LabSlug = template.Slug
					rec.LabDescription = template.Description
					rec.Difficulty = template.Difficulty
					rec.DurationMinutes = template.DurationMinutes
					rec.MaxPoints = template.MaxPoints
				}
			}

			recommendations = append(recommendations, rec)
		}
	}

	return recommendations, nil
}

// getPathwayRecommendations suggests new pathways based on user's history
func (s *Service) getPathwayRecommendations(ctx context.Context, userID string) ([]PathwayRecommendation, error) {
	var recommendations []PathwayRecommendation

	// Get pathways user is NOT enrolled in
	allPathways, err := s.pathwayRepo.List(ctx, models.PathwayListOptions{
		Status:       models.PathwayStatusPublished,
		IncludeStats: true,
		Limit:        20,
	})
	if err != nil {
		return nil, err
	}

	// Get user's enrollments to filter out
	enrollments, err := s.enrollmentRepo.List(ctx, models.EnrollmentListOptions{
		UserID: userID,
		Limit:  100,
	})
	if err != nil {
		return nil, err
	}

	enrolledPathways := make(map[string]bool)
	for _, e := range enrollments {
		enrolledPathways[e.PathwayID] = true
	}

	// Find pathways user isn't enrolled in
	priority := 5
	for _, pathway := range allPathways {
		if enrolledPathways[pathway.ID] {
			continue
		}

		rec := PathwayRecommendation{
			PathwayID:      pathway.ID,
			PathwayName:    pathway.Name,
			PathwaySlug:    pathway.Slug,
			Description:    pathway.ShortDescription,
			Difficulty:     pathway.Difficulty,
			EstimatedHours: pathway.EstimatedHours,
			ModuleCount:    pathway.ModuleCount,
			LabCount:       pathway.LabCount,
			CoverImageURL:  pathway.CoverImageURL,
			Icon:           pathway.Icon,
			Type:           TypeNewPathway,
			Priority:       priority,
		}

		// Adjust priority and reason based on pathway properties
		if pathway.IsFeatured {
			rec.Priority = 3
			rec.Reason = "Featured pathway"
		} else if pathway.EnrollmentCount > 10 {
			rec.Priority = 4
			rec.Reason = "Popular with other students"
			rec.Type = TypePopular
		} else {
			rec.Reason = "Expand your skills"
		}

		recommendations = append(recommendations, rec)
		priority++
	}

	return recommendations, nil
}

// GetNextLabInPathway returns the next lab to complete in a specific pathway
func (s *Service) GetNextLabInPathway(ctx context.Context, userID, pathwayID string) (*LabRecommendation, error) {
	if s.enrollmentRepo == nil || s.pathwayRepo == nil {
		return nil, nil
	}

	// Get enrollment
	enrollment, err := s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathwayID)
	if err != nil || enrollment == nil {
		return nil, err
	}

	// Get enrollment with progress
	enrollmentWithProgress, err := s.enrollmentRepo.GetWithProgress(ctx, enrollment.ID)
	if err != nil {
		return nil, err
	}

	// Get pathway with modules
	pathway, err := s.pathwayRepo.GetWithModules(ctx, pathwayID)
	if err != nil || pathway == nil {
		return nil, err
	}

	// Find first incomplete lab in first unlocked/in-progress module
	for _, moduleProgress := range enrollmentWithProgress.ModuleProgress {
		if moduleProgress.Status == models.ModuleStatusLocked ||
			moduleProgress.Status == models.ModuleStatusCompleted {
			continue
		}

		var module *models.PathwayModule
		for _, m := range pathway.Modules {
			if m.ID == moduleProgress.ModuleID {
				module = m
				break
			}
		}
		if module == nil {
			continue
		}

		for _, labProgress := range moduleProgress.LabProgress {
			if !labProgress.Passed {
				var labInfo *models.ModuleLab
				for _, lab := range module.Labs {
					if lab.LabTemplateID == labProgress.LabTemplateID {
						labInfo = lab
						break
					}
				}
				if labInfo != nil {
					return &LabRecommendation{
						LabTemplateID:   labProgress.LabTemplateID,
						LabName:         labInfo.LabName,
						LabDescription:  labInfo.LabDescription,
						Difficulty:      labInfo.LabDifficulty,
						DurationMinutes: labInfo.LabDuration,
						MaxPoints:       labInfo.LabMaxPoints,
						Type:            TypeNextInPathway,
						Reason:          "Next lab in " + module.Name,
						Priority:        1,
						PathwayID:       pathway.ID,
						PathwayName:     pathway.Name,
						ModuleID:        module.ID,
						ModuleName:      module.Name,
					}, nil
				}
			}
		}
	}

	return nil, nil
}
