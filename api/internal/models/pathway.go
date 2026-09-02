package models

import (
	"encoding/json"
	"time"
)

// PathwayStatus represents the publication state of a pathway
type PathwayStatus string

const (
	PathwayStatusDraft     PathwayStatus = "draft"
	PathwayStatusPublished PathwayStatus = "published"
	PathwayStatusArchived  PathwayStatus = "archived"
)

// EnrollmentStatus represents a user's enrollment state
type EnrollmentStatus string

const (
	EnrollmentStatusEnrolled   EnrollmentStatus = "enrolled"
	EnrollmentStatusInProgress EnrollmentStatus = "in_progress"
	EnrollmentStatusCompleted  EnrollmentStatus = "completed"
	EnrollmentStatusAbandoned  EnrollmentStatus = "abandoned"
)

// ModuleProgressStatus represents the state of module progress
type ModuleProgressStatus string

const (
	ModuleStatusLocked     ModuleProgressStatus = "locked"
	ModuleStatusUnlocked   ModuleProgressStatus = "unlocked"
	ModuleStatusInProgress ModuleProgressStatus = "in_progress"
	ModuleStatusCompleted  ModuleProgressStatus = "completed"
)

// UnlockType represents how a module unlocks
type UnlockType string

const (
	UnlockTypeSequential  UnlockType = "sequential"
	UnlockTypeAllPrevious UnlockType = "all_previous"
	UnlockTypeManual      UnlockType = "manual"
	UnlockTypeAlways      UnlockType = "always"
)

// Pathway represents a learning track containing modules of labs
type Pathway struct {
	ID               string          `json:"id" db:"id"`
	Name             string          `json:"name" db:"name"`
	Slug             string          `json:"slug" db:"slug"`
	Description      string          `json:"description,omitempty" db:"description"`
	ShortDescription string          `json:"shortDescription,omitempty" db:"short_description"`
	Difficulty       string          `json:"difficulty,omitempty" db:"difficulty"`
	EstimatedHours   int             `json:"estimatedHours,omitempty" db:"estimated_hours"`
	DisplayOrder     int             `json:"displayOrder" db:"display_order"`
	Status           PathwayStatus   `json:"status" db:"status"`
	IsFeatured       bool            `json:"isFeatured" db:"is_featured"`
	OrganizationID   *string         `json:"organizationId,omitempty" db:"organization_id"`
	Visibility       LabVisibility   `json:"visibility" db:"visibility"`
	CreatedBy        *string         `json:"createdBy,omitempty" db:"created_by"`
	Prerequisites    json.RawMessage `json:"prerequisites,omitempty" db:"prerequisites"`
	Tags             []string        `json:"tags,omitempty" db:"tags"`
	Icon             string          `json:"icon,omitempty" db:"icon"`
	Color            string          `json:"color,omitempty" db:"color"`
	CoverImageURL    string          `json:"coverImageUrl,omitempty" db:"cover_image_url"`
	CreatedAt        time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time       `json:"updatedAt" db:"updated_at"`

	// Computed/joined fields
	ModuleCount     int `json:"moduleCount,omitempty" db:"module_count"`
	LabCount        int `json:"labCount,omitempty" db:"lab_count"`
	TotalPoints     int `json:"totalPoints,omitempty" db:"total_points"`
	TotalDuration   int `json:"totalDurationMinutes,omitempty" db:"total_duration_minutes"`
	EnrollmentCount int `json:"enrollmentCount,omitempty" db:"enrollment_count"`
	CompletionCount int `json:"completionCount,omitempty" db:"completion_count"`

	// Nested data
	Modules []*PathwayModule `json:"modules,omitempty" db:"-"`
}

// PathwayModule represents a grouped section within a pathway
type PathwayModule struct {
	ID                string     `json:"id" db:"id"`
	PathwayID         string     `json:"pathwayId" db:"pathway_id"`
	Name              string     `json:"name" db:"name"`
	Slug              string     `json:"slug" db:"slug"`
	Description       string     `json:"description,omitempty" db:"description"`
	DisplayOrder      int        `json:"displayOrder" db:"display_order"`
	UnlockType        UnlockType `json:"unlockType" db:"unlock_type"`
	RequiredModuleIDs []string   `json:"requiredModuleIds,omitempty" db:"required_module_ids"`
	IsActive          bool       `json:"isActive" db:"is_active"`
	Icon              string     `json:"icon,omitempty" db:"icon"`
	EstimatedMinutes  int        `json:"estimatedMinutes,omitempty" db:"estimated_minutes"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`

	// Computed fields
	LabCount    int `json:"labCount,omitempty" db:"lab_count"`
	TotalPoints int `json:"totalPoints,omitempty" db:"total_points"`

	// Nested data
	Labs []*ModuleLab `json:"labs,omitempty" db:"-"`
}

// ModuleLab represents a lab within a module (junction table data + lab info)
type ModuleLab struct {
	ID                    string `json:"id" db:"id"`
	ModuleID              string `json:"moduleId" db:"module_id"`
	LabTemplateID         string `json:"labTemplateId" db:"lab_template_id"`
	DisplayOrder          int    `json:"displayOrder" db:"display_order"`
	IsRequired            bool   `json:"isRequired" db:"is_required"`
	PassThresholdOverride *int   `json:"passThresholdOverride,omitempty" db:"pass_threshold_override"`

	// Joined from lab_templates
	LabName        string   `json:"labName,omitempty" db:"lab_name"`
	LabDescription string   `json:"labDescription,omitempty" db:"lab_description"`
	LabDifficulty  string   `json:"labDifficulty,omitempty" db:"lab_difficulty"`
	LabDuration    int      `json:"labDurationMinutes,omitempty" db:"lab_duration_minutes"`
	LabMaxPoints   int      `json:"labMaxPoints,omitempty" db:"lab_max_points"`
	LabPlatform    Platform `json:"labPlatform,omitempty" db:"lab_platform"`
}

// PathwayEnrollment represents a user's enrollment in a pathway
type PathwayEnrollment struct {
	ID                string           `json:"id" db:"id"`
	UserID            string           `json:"userId" db:"user_id"`
	PathwayID         string           `json:"pathwayId" db:"pathway_id"`
	Status            EnrollmentStatus `json:"status" db:"status"`
	CompletedModules  int              `json:"completedModules" db:"completed_modules"`
	TotalModules      int              `json:"totalModules" db:"total_modules"`
	EarnedPoints      int              `json:"earnedPoints" db:"earned_points"`
	MaxPoints         int              `json:"maxPoints" db:"max_points"`
	Percentage        float64          `json:"percentage" db:"percentage"`
	EnrolledAt        time.Time        `json:"enrolledAt" db:"enrolled_at"`
	StartedAt         *time.Time       `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt       *time.Time       `json:"completedAt,omitempty" db:"completed_at"`
	LastActivityAt    *time.Time       `json:"lastActivityAt,omitempty" db:"last_activity_at"`
	OrganizationID    *string          `json:"organizationId,omitempty" db:"organization_id"`
	CertificateIssued bool             `json:"certificateIssued" db:"certificate_issued"`
	CertificateURL    string           `json:"certificateUrl,omitempty" db:"certificate_url"`

	// Joined data
	Pathway        *Pathway          `json:"pathway,omitempty" db:"-"`
	User           *User             `json:"user,omitempty" db:"-"`
	ModuleProgress []*ModuleProgress `json:"moduleProgress,omitempty" db:"-"`
}

// IsCompleted returns true if the enrollment is completed
func (e *PathwayEnrollment) IsCompleted() bool {
	return e.Status == EnrollmentStatusCompleted && e.CompletedAt != nil
}

// ProgressPercentage returns the completion percentage (0-100)
func (e *PathwayEnrollment) ProgressPercentage() float64 {
	if e.TotalModules == 0 {
		return 0
	}
	return float64(e.CompletedModules) / float64(e.TotalModules) * 100
}

// ModuleProgress represents a user's progress within a specific module
type ModuleProgress struct {
	ID            string               `json:"id" db:"id"`
	EnrollmentID  string               `json:"enrollmentId" db:"enrollment_id"`
	ModuleID      string               `json:"moduleId" db:"module_id"`
	Status        ModuleProgressStatus `json:"status" db:"status"`
	CompletedLabs int                  `json:"completedLabs" db:"completed_labs"`
	TotalLabs     int                  `json:"totalLabs" db:"total_labs"`
	EarnedPoints  int                  `json:"earnedPoints" db:"earned_points"`
	MaxPoints     int                  `json:"maxPoints" db:"max_points"`
	UnlockedAt    *time.Time           `json:"unlockedAt,omitempty" db:"unlocked_at"`
	StartedAt     *time.Time           `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt   *time.Time           `json:"completedAt,omitempty" db:"completed_at"`

	// Joined data
	Module      *PathwayModule `json:"module,omitempty" db:"-"`
	LabProgress []*LabProgress `json:"labProgress,omitempty" db:"-"`
}

// IsCompleted returns true if the module is completed
func (m *ModuleProgress) IsCompleted() bool {
	return m.Status == ModuleStatusCompleted && m.CompletedAt != nil
}

// ProgressPercentage returns the lab completion percentage (0-100)
func (m *ModuleProgress) ProgressPercentage() float64 {
	if m.TotalLabs == 0 {
		return 0
	}
	return float64(m.CompletedLabs) / float64(m.TotalLabs) * 100
}

// LabProgress represents a user's progress on a specific lab within a pathway
type LabProgress struct {
	ID             string     `json:"id" db:"id"`
	EnrollmentID   string     `json:"enrollmentId" db:"enrollment_id"`
	ModuleID       string     `json:"moduleId" db:"module_id"`
	LabTemplateID  string     `json:"labTemplateId" db:"lab_template_id"`
	BestSessionID  *string    `json:"bestSessionId,omitempty" db:"best_session_id"`
	AttemptCount   int        `json:"attemptCount" db:"attempt_count"`
	BestScore      int        `json:"bestScore" db:"best_score"`
	MaxPoints      int        `json:"maxPoints" db:"max_points"`
	Passed         bool       `json:"passed" db:"passed"`
	FirstAttemptAt *time.Time `json:"firstAttemptAt,omitempty" db:"first_attempt_at"`
	CompletedAt    *time.Time `json:"completedAt,omitempty" db:"completed_at"`

	// Joined data
	LabTemplate *LabTemplateRecord `json:"labTemplate,omitempty" db:"-"`
	BestSession *Session           `json:"bestSession,omitempty" db:"-"`
}

// ScorePercentage returns the best score as a percentage (0-100)
func (l *LabProgress) ScorePercentage() float64 {
	if l.MaxPoints == 0 {
		return 0
	}
	return float64(l.BestScore) / float64(l.MaxPoints) * 100
}

// PathwayStats holds computed statistics for a pathway
type PathwayStats struct {
	PathwayID            string  `json:"pathwayId" db:"id"`
	ModuleCount          int     `json:"moduleCount" db:"module_count"`
	LabCount             int     `json:"labCount" db:"lab_count"`
	TotalPoints          int     `json:"totalPoints" db:"total_points"`
	TotalDurationMinutes int     `json:"totalDurationMinutes" db:"total_duration_minutes"`
	EnrollmentCount      int     `json:"enrollmentCount" db:"enrollment_count"`
	CompletionCount      int     `json:"completionCount" db:"completion_count"`
	CompletionRate       float64 `json:"completionRate,omitempty"`
	AverageScore         float64 `json:"averageScore,omitempty"`
}

// ComputeCompletionRate calculates the completion rate
func (s *PathwayStats) ComputeCompletionRate() {
	if s.EnrollmentCount == 0 {
		s.CompletionRate = 0
		return
	}
	s.CompletionRate = float64(s.CompletionCount) / float64(s.EnrollmentCount) * 100
}

// PathwayWithProgress combines a pathway with user-specific progress
type PathwayWithProgress struct {
	Pathway    `json:",inline"`
	Enrollment *PathwayEnrollment `json:"enrollment,omitempty"`
}

// ModuleWithProgress combines a module with user-specific progress
type ModuleWithProgress struct {
	PathwayModule `json:",inline"`
	Progress      *ModuleProgress `json:"progress,omitempty"`
}

// LabWithProgress combines a lab with user-specific progress
type LabWithProgress struct {
	ModuleLab `json:",inline"`
	Progress  *LabProgress `json:"progress,omitempty"`
}

// CreatePathwayRequest represents the request to create a pathway
type CreatePathwayRequest struct {
	Name             string        `json:"name" validate:"required,min=2,max=255"`
	Slug             string        `json:"slug,omitempty" validate:"omitempty,slug,max=128"`
	Description      string        `json:"description,omitempty"`
	ShortDescription string        `json:"shortDescription,omitempty" validate:"max=500"`
	Difficulty       string        `json:"difficulty,omitempty" validate:"omitempty,oneof=beginner intermediate advanced mixed"`
	EstimatedHours   int           `json:"estimatedHours,omitempty" validate:"omitempty,min=1"`
	Tags             []string      `json:"tags,omitempty"`
	Icon             string        `json:"icon,omitempty"`
	Color            string        `json:"color,omitempty"`
	CoverImageURL    string        `json:"coverImageUrl,omitempty" validate:"omitempty,url"`
	Visibility       LabVisibility `json:"visibility,omitempty"`
}

// UpdatePathwayRequest represents the request to update a pathway
type UpdatePathwayRequest struct {
	Name             *string        `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description      *string        `json:"description,omitempty"`
	ShortDescription *string        `json:"shortDescription,omitempty" validate:"omitempty,max=500"`
	Difficulty       *string        `json:"difficulty,omitempty" validate:"omitempty,oneof=beginner intermediate advanced mixed"`
	EstimatedHours   *int           `json:"estimatedHours,omitempty" validate:"omitempty,min=1"`
	DisplayOrder     *int           `json:"displayOrder,omitempty"`
	Status           *PathwayStatus `json:"status,omitempty"`
	IsFeatured       *bool          `json:"isFeatured,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Icon             *string        `json:"icon,omitempty"`
	Color            *string        `json:"color,omitempty"`
	CoverImageURL    *string        `json:"coverImageUrl,omitempty" validate:"omitempty,url"`
	Visibility       *LabVisibility `json:"visibility,omitempty"`
}

// CreateModuleRequest represents the request to create a pathway module
type CreateModuleRequest struct {
	Name             string     `json:"name" validate:"required,min=2,max=255"`
	Slug             string     `json:"slug,omitempty" validate:"omitempty,slug,max=128"`
	Description      string     `json:"description,omitempty"`
	UnlockType       UnlockType `json:"unlockType,omitempty"`
	Icon             string     `json:"icon,omitempty"`
	EstimatedMinutes int        `json:"estimatedMinutes,omitempty" validate:"omitempty,min=1"`
}

// UpdateModuleRequest represents the request to update a pathway module
type UpdateModuleRequest struct {
	Name             *string     `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description      *string     `json:"description,omitempty"`
	DisplayOrder     *int        `json:"displayOrder,omitempty"`
	UnlockType       *UnlockType `json:"unlockType,omitempty"`
	IsActive         *bool       `json:"isActive,omitempty"`
	Icon             *string     `json:"icon,omitempty"`
	EstimatedMinutes *int        `json:"estimatedMinutes,omitempty" validate:"omitempty,min=1"`
}

// AddLabToModuleRequest represents the request to add a lab to a module
type AddLabToModuleRequest struct {
	LabTemplateID         string `json:"labTemplateId" validate:"required,uuid"`
	DisplayOrder          *int   `json:"displayOrder,omitempty"`
	IsRequired            *bool  `json:"isRequired,omitempty"`
	PassThresholdOverride *int   `json:"passThresholdOverride,omitempty" validate:"omitempty,min=0,max=100"`
}

// EnrollRequest represents the request to enroll in a pathway
type EnrollRequest struct {
	PathwayID string `json:"pathwayId" validate:"required,uuid"`
}

// PathwayListOptions represents options for listing pathways
type PathwayListOptions struct {
	Status         PathwayStatus `json:"status,omitempty"`
	Visibility     LabVisibility `json:"visibility,omitempty"`
	Difficulty     string        `json:"difficulty,omitempty"`
	IsFeatured     *bool         `json:"isFeatured,omitempty"`
	OrganizationID *string       `json:"organizationId,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
	Search         string        `json:"search,omitempty"`
	IncludeStats   bool          `json:"includeStats,omitempty"`
	Limit          int           `json:"limit,omitempty"`
	Offset         int           `json:"offset,omitempty"`
}

// EnrollmentListOptions represents options for listing enrollments
type EnrollmentListOptions struct {
	UserID         string           `json:"userId,omitempty"`
	PathwayID      string           `json:"pathwayId,omitempty"`
	Status         EnrollmentStatus `json:"status,omitempty"`
	OrganizationID *string          `json:"organizationId,omitempty"`
	Limit          int              `json:"limit,omitempty"`
	Offset         int              `json:"offset,omitempty"`
}
