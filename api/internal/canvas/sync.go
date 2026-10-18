package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// SyncService handles synchronization between Canvas and Kootenai
type SyncService struct {
	config             Config
	orgRepo            repositories.OrganizationRepository
	orgMembershipRepo  repositories.OrganizationMembershipRepository
	teamRepo           repositories.TeamRepository
	teamMembershipRepo repositories.TeamMembershipRepository
	userRepo           repositories.UserRepository
	httpClient         *http.Client
	logger             *slog.Logger
}

// NewSyncService creates a new Canvas sync service
func NewSyncService(
	cfg Config,
	orgRepo repositories.OrganizationRepository,
	orgMembershipRepo repositories.OrganizationMembershipRepository,
	teamRepo repositories.TeamRepository,
	teamMembershipRepo repositories.TeamMembershipRepository,
	userRepo repositories.UserRepository,
	logger *slog.Logger,
) *SyncService {
	return &SyncService{
		config:             cfg,
		orgRepo:            orgRepo,
		orgMembershipRepo:  orgMembershipRepo,
		teamRepo:           teamRepo,
		teamMembershipRepo: teamMembershipRepo,
		userRepo:           userRepo,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		logger: logger,
	}
}

// CanvasCourseInfo contains course information from Canvas API
type CanvasCourseInfo struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	CourseCode       string     `json:"course_code"`
	WorkflowState    string     `json:"workflow_state"`
	AccountID        int64      `json:"account_id"`
	EnrollmentTermID int64      `json:"enrollment_term_id"`
	StartAt          *time.Time `json:"start_at"`
	EndAt            *time.Time `json:"end_at"`
	TotalStudents    int        `json:"total_students"`
}

// CanvasSection contains section information from Canvas API
type CanvasSection struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	CourseID      int64      `json:"course_id"`
	SISSectionID  string     `json:"sis_section_id"`
	IntegrationID string     `json:"integration_id"`
	StartAt       *time.Time `json:"start_at"`
	EndAt         *time.Time `json:"end_at"`
	TotalStudents int        `json:"total_students"`
}

// CanvasEnrollment contains enrollment information from Canvas API
type CanvasEnrollment struct {
	ID              int64           `json:"id"`
	CourseID        int64           `json:"course_id"`
	CourseSectionID int64           `json:"course_section_id"`
	UserID          int64           `json:"user_id"`
	Type            string          `json:"type"` // StudentEnrollment, TeacherEnrollment, TaEnrollment
	Role            string          `json:"role"`
	EnrollmentState string          `json:"enrollment_state"` // active, invited, inactive
	User            *CanvasUserInfo `json:"user,omitempty"`
}

// CanvasUserInfo contains user information from Canvas API
type CanvasUserInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	LoginID   string `json:"login_id,omitempty"`
	SISUserID string `json:"sis_user_id,omitempty"`
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	OrganizationID   string   `json:"organizationId"`
	OrganizationName string   `json:"organizationName"`
	TeamsCreated     int      `json:"teamsCreated"`
	TeamsUpdated     int      `json:"teamsUpdated"`
	MembersAdded     int      `json:"membersAdded"`
	Errors           []string `json:"errors,omitempty"`
}

// GetOrCreateOrganizationForCourse finds or creates an organization for a Canvas course
func (s *SyncService) GetOrCreateOrganizationForCourse(ctx context.Context, courseID, courseName, courseCode string) (*models.Organization, error) {
	// Generate a slug from course code and ID
	slug := generateCourseSlug(courseCode, courseID)

	// Check if organization already exists
	org, err := s.orgRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("checking for existing org: %w", err)
	}

	if org != nil {
		s.logger.Info("Found existing organization for Canvas course",
			"orgId", org.ID,
			"courseId", courseID,
			"slug", slug,
		)
		return org, nil
	}

	// Create new organization for the course
	now := time.Now()
	org = &models.Organization{
		ID:       uuid.New().String(),
		Name:     courseName,
		Slug:     slug,
		Type:     models.OrgTypeEducational,
		Edition:  models.EditionCommunity, // Default to community
		IsActive: true,
		Settings: models.OrgSettings{
			EnableTeams:        true,
			EnableAchievements: true,
			EnableLeaderboards: true,
			DefaultUserRole:    models.OrgRoleMember,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}

	s.logger.Info("Created new organization for Canvas course",
		"orgId", org.ID,
		"courseId", courseID,
		"courseName", courseName,
		"slug", slug,
	)

	return org, nil
}

// SyncSectionsAsTeams fetches Canvas sections and creates/updates teams
func (s *SyncService) SyncSectionsAsTeams(ctx context.Context, courseID, orgID string) (*SyncResult, error) {
	result := &SyncResult{
		OrganizationID: orgID,
	}

	// Fetch sections from Canvas
	sections, err := s.fetchCourseSections(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("fetching sections: %w", err)
	}

	for _, section := range sections {
		team, created, err := s.syncSectionAsTeam(ctx, orgID, section)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("section %s: %v", section.Name, err))
			continue
		}

		if created {
			result.TeamsCreated++
		} else {
			result.TeamsUpdated++
		}

		// Sync section enrollments as team members
		added, err := s.syncSectionMembers(ctx, courseID, section, team)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("members for section %s: %v", section.Name, err))
		}
		result.MembersAdded += added
	}

	s.logger.Info("Synced Canvas sections as teams",
		"courseId", courseID,
		"orgId", orgID,
		"teamsCreated", result.TeamsCreated,
		"teamsUpdated", result.TeamsUpdated,
		"membersAdded", result.MembersAdded,
	)

	return result, nil
}

// syncSectionAsTeam creates or updates a team for a Canvas section
func (s *SyncService) syncSectionAsTeam(ctx context.Context, orgID string, section CanvasSection) (*models.Team, bool, error) {
	slug := generateSectionSlug(section.Name, section.ID)

	// Check if team exists
	teams, err := s.teamRepo.ListByOrganization(ctx, orgID, repositories.TeamFilter{})
	if err != nil {
		return nil, false, fmt.Errorf("listing teams: %w", err)
	}

	for _, team := range teams {
		if team.Slug == slug {
			// Team exists, update if needed
			return team, false, nil
		}
	}

	// Create new team
	now := time.Now()
	team := &models.Team{
		ID:             uuid.New().String(),
		OrganizationID: orgID,
		Name:           section.Name,
		Slug:           slug,
		Description:    fmt.Sprintf("Canvas Section: %s", section.Name),
		IsActive:       true,
		CreatedAt:      now,
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return nil, false, fmt.Errorf("creating team: %w", err)
	}

	return team, true, nil
}

// syncSectionMembers syncs section enrollments as team members
func (s *SyncService) syncSectionMembers(ctx context.Context, courseID string, section CanvasSection, team *models.Team) (int, error) {
	if s.config.APIAccessToken == "" {
		// Can't sync members without API token
		return 0, nil
	}

	enrollments, err := s.fetchSectionEnrollments(ctx, courseID, section.ID)
	if err != nil {
		return 0, fmt.Errorf("fetching enrollments: %w", err)
	}

	added := 0
	for _, enrollment := range enrollments {
		if enrollment.User == nil || enrollment.EnrollmentState != "active" {
			continue
		}

		// Find or create user
		user, err := s.findOrCreateUser(ctx, enrollment.User)
		if err != nil {
			s.logger.Warn("Failed to find/create user",
				"canvasUserId", enrollment.UserID,
				"error", err,
			)
			continue
		}

		// Determine role based on enrollment type
		role := models.TeamRoleMember
		if enrollment.Type == "TeacherEnrollment" || enrollment.Type == "TaEnrollment" {
			role = models.TeamRoleLead
		}

		// Add to team if not already a member
		_, err = s.teamMembershipRepo.GetByTeamAndUser(ctx, team.ID, user.ID)
		if err == nil {
			// Already a member
			continue
		}

		membership := &models.TeamMembership{
			ID:        uuid.New().String(),
			TeamID:    team.ID,
			UserID:    user.ID,
			Role:      role,
			CreatedAt: time.Now(),
		}

		if err := s.teamMembershipRepo.Create(ctx, membership); err != nil {
			s.logger.Warn("Failed to add team member",
				"teamId", team.ID,
				"userId", user.ID,
				"error", err,
			)
			continue
		}

		added++
	}

	return added, nil
}

// findOrCreateUser finds an existing user or creates one from Canvas user info
func (s *SyncService) findOrCreateUser(ctx context.Context, canvasUser *CanvasUserInfo) (*models.User, error) {
	// Try to find by email
	email := canvasUser.Email
	if email == "" {
		email = canvasUser.LoginID
	}
	if email == "" {
		return nil, fmt.Errorf("no email or login_id for Canvas user %d", canvasUser.ID)
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	if user != nil {
		return user, nil
	}

	// Create new user
	now := time.Now()
	user = &models.User{
		ID:          uuid.New().String(),
		Email:       email,
		DisplayName: canvasUser.Name,
		Role:        "student",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	s.logger.Info("Created user from Canvas",
		"userId", user.ID,
		"email", email,
		"canvasUserId", canvasUser.ID,
	)

	return user, nil
}

// fetchCourseSections fetches all sections for a course from Canvas API
func (s *SyncService) fetchCourseSections(ctx context.Context, courseID string) ([]CanvasSection, error) {
	if s.config.APIAccessToken == "" {
		return nil, fmt.Errorf("API access token not configured")
	}

	url := fmt.Sprintf("%s/api/v1/courses/%s/sections", s.config.CanvasURL, courseID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.APIAccessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var sections []CanvasSection
	if err := json.NewDecoder(resp.Body).Decode(&sections); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return sections, nil
}

// fetchSectionEnrollments fetches enrollments for a section
func (s *SyncService) fetchSectionEnrollments(ctx context.Context, courseID string, sectionID int64) ([]CanvasEnrollment, error) {
	if s.config.APIAccessToken == "" {
		return nil, fmt.Errorf("API access token not configured")
	}

	url := fmt.Sprintf("%s/api/v1/courses/%s/enrollments?course_section_id=%d&include[]=user",
		s.config.CanvasURL, courseID, sectionID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.APIAccessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var enrollments []CanvasEnrollment
	if err := json.NewDecoder(resp.Body).Decode(&enrollments); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return enrollments, nil
}

// FetchCourseInfo fetches course information from Canvas API
func (s *SyncService) FetchCourseInfo(ctx context.Context, courseID string) (*CanvasCourseInfo, error) {
	if s.config.APIAccessToken == "" {
		return nil, fmt.Errorf("API access token not configured")
	}

	url := fmt.Sprintf("%s/api/v1/courses/%s", s.config.CanvasURL, courseID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.APIAccessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var course CanvasCourseInfo
	if err := json.NewDecoder(resp.Body).Decode(&course); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &course, nil
}

// generateCourseSlug creates a URL-safe slug for a Canvas course
func generateCourseSlug(courseCode, courseID string) string {
	slug := strings.ToLower(courseCode)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove any non-alphanumeric characters except hyphens
	result := strings.Builder{}
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	// Append course ID to ensure uniqueness
	return fmt.Sprintf("canvas-%s-%s", result.String(), courseID)
}

// generateSectionSlug creates a URL-safe slug for a Canvas section
func generateSectionSlug(sectionName string, sectionID int64) string {
	slug := strings.ToLower(sectionName)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	result := strings.Builder{}
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return fmt.Sprintf("section-%s-%d", result.String(), sectionID)
}
