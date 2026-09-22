// Package repositories provides database access layer implementations.
// Test doubles for these interfaces live in
// internal/testutil/mocks/repositories.go as hand-written Fake*Repository
// types (state-machine fakes). A previous gomock-generated mock file
// (mock_repositories.go) was removed in favor of the fakes — every test
// uses the Fake* variant; mixing two mock styles was a maintenance tax.
package repositories

import (
	"context"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Repository Interfaces
// -----------------------------------------------------------------------------

// LabTemplateRepository defines operations for lab templates
type LabTemplateRepository interface {
	Create(ctx context.Context, record *models.LabTemplateRecord) error
	GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error)
	GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) // Batch fetch to avoid N+1 queries
	GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error)
	List(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error)
	Update(ctx context.Context, record *models.LabTemplateRecord) error
	Delete(ctx context.Context, id string) error
	SetActive(ctx context.Context, id string, active bool) error
	// Version history
	CreateVersion(ctx context.Context, version *models.LabTemplateVersion) error
	ListVersions(ctx context.Context, templateID string, limit, offset int) ([]*models.LabTemplateVersion, error)
	GetVersionByNumber(ctx context.Context, templateID string, versionNumber int) (*models.LabTemplateVersion, error)
	CountVersions(ctx context.Context, templateID string) (int, error)
}

// PodRepository defines operations for pods
type PodRepository interface {
	Create(ctx context.Context, pod *models.Pod) error
	GetByID(ctx context.Context, id string) (*models.Pod, error)
	List(ctx context.Context, filter PodFilter) ([]*models.Pod, error)
	Update(ctx context.Context, pod *models.Pod) error
	UpdateStatus(ctx context.Context, id string, status models.PodStatus) error
	Delete(ctx context.Context, id string) error
	GetExpired(ctx context.Context) ([]*models.Pod, error)
	// CountActive returns the number of pods that still hold hypervisor
	// resources, i.e. every status except destroyed. A destroying pod is
	// mid-teardown and still has VMs on the hypervisor.
	CountActive(ctx context.Context) (int64, error)
	// Ownership methods
	GetOwnerID(ctx context.Context, id string) (string, error)
	IsOwner(ctx context.Context, id, userID string) (bool, error)
	GetOrganizationID(ctx context.Context, id string) (*string, error)
}

// SessionRepository defines operations for lab sessions
type SessionRepository interface {
	Create(ctx context.Context, session *models.Session) error
	GetByID(ctx context.Context, id string) (*models.Session, error)
	GetByPodID(ctx context.Context, podID string) ([]*models.Session, error)
	GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error)
	List(ctx context.Context, filter SessionFilter) ([]*models.Session, error)
	Update(ctx context.Context, session *models.Session) error
	End(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error
	MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error
	MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error
	// CountActive returns the number of sessions that have not ended.
	CountActive(ctx context.Context) (int64, error)
	// Ownership methods
	GetUserID(ctx context.Context, id string) (string, error)
	IsOwner(ctx context.Context, id, userID string) (bool, error)
	GetOrganizationID(ctx context.Context, id string) (*string, error)
	// Cleanup
	EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error)
	DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error)
	// Analytics
	ListAll(ctx context.Context) ([]*models.Session, error)
	CountCompletedLabsByUser(ctx context.Context) (map[string]int, error)
	GetUserStats(ctx context.Context, userID string) (*UserSessionStats, error)
	ListWithLabNames(ctx context.Context, filter SessionFilter) ([]*SessionWithLabName, error)
}

// CheckpointProgressRepository defines operations for checkpoint progress
type CheckpointProgressRepository interface {
	Create(ctx context.Context, progress *models.CheckpointProgress) error
	GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointProgress, error)
	GetBySessionAndCheckpoint(ctx context.Context, sessionID, checkpointID string) (*models.CheckpointProgress, error)
	Update(ctx context.Context, progress *models.CheckpointProgress) error
	MarkPassed(ctx context.Context, sessionID, checkpointID string, triggerEventID *string) error
	MarkFailed(ctx context.Context, sessionID, checkpointID string) error
	ResetForSession(ctx context.Context, sessionID string) error
}

// EventRepository defines operations for events
type EventRepository interface {
	Create(ctx context.Context, event *models.Event) error
	GetByID(ctx context.Context, id int64) (*models.Event, error)
	GetByPodID(ctx context.Context, podID string, limit int) ([]*models.Event, error)
	GetBySessionID(ctx context.Context, sessionID string, limit int) ([]*models.Event, error)
	GetUnprocessed(ctx context.Context, limit int) ([]*models.Event, error)
	MarkProcessed(ctx context.Context, id int64, matchedCheckpoints []string) error
	Query(ctx context.Context, filter EventFilter) ([]*models.Event, error)
}

// UserRepository defines operations for users
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByIDs(ctx context.Context, ids []string) ([]*models.User, error) // Batch fetch to avoid N+1 queries
	GetByExternalID(ctx context.Context, externalID string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetOrCreateByUsername(ctx context.Context, username string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLastLogin(ctx context.Context, id string) error
	List(ctx context.Context, opts UserListOptions) ([]*models.User, int, error)
	Delete(ctx context.Context, id string) error
	// Password management
	GetByEmailForAuth(ctx context.Context, email string) (*models.User, error)
	GetByIDWithPassword(ctx context.Context, id string) (*models.User, error)
	UpdatePassword(ctx context.Context, id, passwordHash string, mustChange bool) error
	ClearMustChangePassword(ctx context.Context, id string) error
	// Locale preference
	GetPreferredLocale(ctx context.Context, id string) (*string, error)
	UpdatePreferredLocale(ctx context.Context, id string, locale *string) error
}

// GradeSyncRepository defines operations for grade sync queue
type GradeSyncRepository interface {
	Create(ctx context.Context, entry *models.GradeSyncEntry) error
	GetPending(ctx context.Context, limit int) ([]*models.GradeSyncEntry, error)
	MarkProcessing(ctx context.Context, id string) error
	MarkCompleted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errorMsg string) error
}

// AuditLogRepository defines operations for audit logs
type AuditLogRepository interface {
	Create(ctx context.Context, entry *models.AuditEntry) error
	Query(ctx context.Context, filter AuditFilter) ([]*models.AuditEntry, error)
}

// AssessmentResultRepository defines operations for assessment results
type AssessmentResultRepository interface {
	Create(ctx context.Context, result *models.AssessmentResult) error
	GetBySessionID(ctx context.Context, sessionID string) (*models.AssessmentResult, error)
	Update(ctx context.Context, result *models.AssessmentResult) error
	UpdateStatus(ctx context.Context, sessionID string, status string) error
	Delete(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context, olderThan time.Time) (int64, error)
}

// ReservationRepository defines operations for reservations
type ReservationRepository interface {
	Create(ctx context.Context, reservation *models.Reservation) error
	GetByID(ctx context.Context, id string) (*models.Reservation, error)
	GetByUserID(ctx context.Context, userID string, filter ReservationFilter) ([]*models.Reservation, error)
	List(ctx context.Context, filter ReservationFilter) ([]*models.Reservation, error)
	Update(ctx context.Context, reservation *models.Reservation) error
	UpdateStatus(ctx context.Context, id string, status models.ReservationStatus) error
	Cancel(ctx context.Context, id string, reason string) error
	SetPodID(ctx context.Context, id string, podID string) error
	GetOverlapping(ctx context.Context, startTime, endTime time.Time, excludeID string) ([]*models.Reservation, error)
	GetUpcoming(ctx context.Context, withinMinutes int) ([]*models.Reservation, error)
	GetExpired(ctx context.Context) ([]*models.Reservation, error)
}

// The RBAC surface is split into focused interfaces so callers can depend on
// the narrowest surface they need (easier mocking, clearer intent). The
// composite RBACRepository is kept as an alias-of-embedding so existing
// concrete repositories and callers don't need to change.

// PermissionReader reads the catalog of defined permissions.
type PermissionReader interface {
	GetPermission(ctx context.Context, id string) (*models.Permission, error)
	ListPermissions(ctx context.Context, resource string) ([]*models.Permission, error)
}

// RoleReader reads role definitions without mutating them.
type RoleReader interface {
	GetRole(ctx context.Context, id string) (*models.Role, error)
	GetRoleBySlug(ctx context.Context, slug string) (*models.Role, error)
	GetRoleWithPermissions(ctx context.Context, id string) (*models.Role, error)
	ListRoles(ctx context.Context, orgID *string, includeSystem bool) ([]*models.Role, error)
	GetRolePermissions(ctx context.Context, roleID string) ([]string, error)
}

// RoleWriter mutates role definitions and their permission assignments.
type RoleWriter interface {
	CreateRole(ctx context.Context, role *models.Role) error
	UpdateRole(ctx context.Context, role *models.Role) error
	DeleteRole(ctx context.Context, id string) error
	SetRolePermissionSets(ctx context.Context, roleID string, permissionSetIDs []string) error
	SetRolePermissions(ctx context.Context, roleID string, permissions []models.RolePermission) error
}

// UserRoleManager assigns/revokes roles to users.
type UserRoleManager interface {
	AssignRole(ctx context.Context, userRole *models.UserRole) error
	RevokeRole(ctx context.Context, userID, roleID string, orgID *string) error
	GetUserRoles(ctx context.Context, userID string, orgID *string) ([]*models.UserRole, error)
	BulkAssignRole(ctx context.Context, userIDs []string, roleID string, orgID *string, grantedBy *string) error
	BulkRevokeRole(ctx context.Context, userIDs []string, roleID string, orgID *string) error
	CleanupExpiredRoles(ctx context.Context) (int64, error)
}

// PermissionChecker answers "can this user do X?". This is the surface that
// request-path authorization middleware needs; it is intentionally narrow so
// middleware can be unit-tested without standing up a full RBAC repo.
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID, permissionID string, orgID *string) (bool, error)
	GetUserPermissions(ctx context.Context, userID string, orgID *string) ([]string, error)
	CheckPermissions(ctx context.Context, userID string, permissionIDs []string, orgID *string) (map[string]bool, error)
	GetUsersWithPermission(ctx context.Context, permissionID string, orgID *string) ([]string, error)
}

// RBACRepository is the composite surface implemented by the concrete
// Postgres-backed RBAC repository. New code should accept the narrowest
// sub-interface it actually uses rather than this composite.
type RBACRepository interface {
	PermissionReader
	RoleReader
	RoleWriter
	UserRoleManager
	PermissionChecker
}

// -----------------------------------------------------------------------------
// Filter Types
// -----------------------------------------------------------------------------

// LabTemplateFilter defines filters for lab template queries
type LabTemplateFilter struct {
	Platform       string
	Category       string
	Active         *bool
	OrganizationID string               // Filter by organization
	Visibility     models.LabVisibility // Filter by visibility level
	CreatedBy      string               // Filter by creator user ID
	IncludeGlobal  bool                 // Include global templates when filtering by org
	MaxEdition     models.Edition       // Filter by maximum edition (includes this and lower)
	Limit          int
	Offset         int
}

// PodFilter defines filters for pod queries
type PodFilter struct {
	OwnerID        string
	TemplateID     string
	Status         models.PodStatus
	Platform       string
	OrganizationID string // Filter by organization
	TeamID         string // Filter by team
	Limit          int
	Offset         int
}

// SessionFilter defines filters for session queries
type SessionFilter struct {
	UserID             string
	PodID              string
	TemplateID         string
	CanvasAssignmentID string
	Active             *bool
	OrganizationID     string // Filter by organization
	TeamID             string // Filter by team
	Limit              int
	Offset             int
}

// EventFilter defines filters for event queries
type EventFilter struct {
	PodID     string
	SessionID string
	VMName    string
	EventType string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// AuditFilter defines filters for audit log queries
type AuditFilter struct {
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

// ReservationFilter defines filters for reservation queries
type ReservationFilter struct {
	UserID        string
	LabTemplateID string
	Status        models.ReservationStatus
	StartAfter    *time.Time
	StartBefore   *time.Time
	Limit         int
	Offset        int
}

// -----------------------------------------------------------------------------
// Multi-Tenancy Interfaces
// -----------------------------------------------------------------------------

// OrganizationRepository defines operations for organizations
type OrganizationRepository interface {
	Create(ctx context.Context, org *models.Organization) error
	GetByID(ctx context.Context, id string) (*models.Organization, error)
	GetByIDs(ctx context.Context, ids []string) ([]*models.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*models.Organization, error)
	List(ctx context.Context, filter OrganizationFilter) ([]*models.Organization, error)
	Update(ctx context.Context, org *models.Organization) error
	Delete(ctx context.Context, id string) error
	SetActive(ctx context.Context, id string, active bool) error
	GetMemberCount(ctx context.Context, id string) (int, error)
	GetTeamCount(ctx context.Context, id string) (int, error)
	GetMemberCountBatch(ctx context.Context, ids []string) (map[string]int, error)
	GetTeamCountBatch(ctx context.Context, ids []string) (map[string]int, error)
}

// OrganizationMembershipRepository defines operations for organization memberships
type OrganizationMembershipRepository interface {
	Create(ctx context.Context, membership *models.OrganizationMembership) error
	GetByID(ctx context.Context, id string) (*models.OrganizationMembership, error)
	GetByOrgAndUser(ctx context.Context, orgID, userID string) (*models.OrganizationMembership, error)
	ListByOrganization(ctx context.Context, orgID string, filter MembershipFilter) ([]*models.OrganizationMembership, error)
	ListByUser(ctx context.Context, userID string) ([]*models.OrganizationMembership, error)
	Update(ctx context.Context, membership *models.OrganizationMembership) error
	UpdateRole(ctx context.Context, id string, role models.OrgRole) error
	Delete(ctx context.Context, id string) error
	AcceptInvitation(ctx context.Context, token string) (*models.OrganizationMembership, error)
	SetPrimary(ctx context.Context, userID, orgID string) error
	GetPrimaryOrganization(ctx context.Context, userID string) (*models.Organization, error)
}

// TeamRepository defines operations for teams
type TeamRepository interface {
	Create(ctx context.Context, team *models.Team) error
	GetByID(ctx context.Context, id string) (*models.Team, error)
	GetBySlug(ctx context.Context, orgID, slug string) (*models.Team, error)
	ListByOrganization(ctx context.Context, orgID string, filter TeamFilter) ([]*models.Team, error)
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id string) error
	SetActive(ctx context.Context, id string, active bool) error
	GetMemberCount(ctx context.Context, id string) (int, error)
}

// TeamMembershipRepository defines operations for team memberships
type TeamMembershipRepository interface {
	Create(ctx context.Context, membership *models.TeamMembership) error
	GetByID(ctx context.Context, id string) (*models.TeamMembership, error)
	GetByTeamAndUser(ctx context.Context, teamID, userID string) (*models.TeamMembership, error)
	ListByTeam(ctx context.Context, teamID string) ([]*models.TeamMembership, error)
	ListByTeamAndRole(ctx context.Context, teamID string, role models.TeamRole) ([]*models.TeamMembership, error)
	ListByUser(ctx context.Context, userID string) ([]*models.TeamMembership, error)
	Update(ctx context.Context, membership *models.TeamMembership) error
	UpdateRole(ctx context.Context, id string, role models.TeamRole) error
	Delete(ctx context.Context, id string) error
}

// FeatureRepository defines operations for feature flags
type FeatureRepository interface {
	GetByID(ctx context.Context, id string) (*models.FeatureFlag, error)
	List(ctx context.Context) ([]*models.FeatureFlag, error)
	ListByEdition(ctx context.Context, edition models.Edition) ([]*models.FeatureFlag, error)
	IsFeatureEnabled(ctx context.Context, orgID, featureID string) (bool, error)
	GetOrganizationFeatures(ctx context.Context, orgID string) (map[string]bool, error)
	SetOrganizationFeature(ctx context.Context, feature *models.OrganizationFeature) error
	RemoveOrganizationFeature(ctx context.Context, orgID, featureID string) error
}

// LicenseRepository defines operations for licenses
type LicenseRepository interface {
	Create(ctx context.Context, license *models.License) error
	GetByID(ctx context.Context, id string) (*models.License, error)
	GetByOrganization(ctx context.Context, orgID string) (*models.License, error)
	GetByLicenseKey(ctx context.Context, key string) (*models.License, error)
	GetActiveByOrganization(ctx context.Context, orgID string) (*models.License, error)
	Update(ctx context.Context, license *models.License) error
	UpdateValidationStatus(ctx context.Context, id string, status models.LicenseStatus, errorMsg *string) error
	Deactivate(ctx context.Context, id string) error
}

// -----------------------------------------------------------------------------
// Multi-Tenancy Filter Types
// -----------------------------------------------------------------------------

// OrganizationFilter defines filters for organization queries
type OrganizationFilter struct {
	Type    models.OrgType
	Edition models.Edition
	Active  *bool
	Limit   int
	Offset  int
}

// MembershipFilter defines filters for membership queries
type MembershipFilter struct {
	Role           models.OrgRole
	IncludePending bool
	Limit          int
	Offset         int
}

// TeamFilter defines filters for team queries
type TeamFilter struct {
	ParentTeamID *string
	Active       *bool
	Limit        int
	Offset       int
}

// -----------------------------------------------------------------------------
// Password Reset Token Repository
// -----------------------------------------------------------------------------

// PasswordResetTokenRepository defines operations for password reset tokens
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *models.PasswordResetToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// QuestionResponseRepository defines operations for question responses
type QuestionResponseRepository interface {
	Create(ctx context.Context, response *models.QuestionResponse) error
	GetBySessionID(ctx context.Context, sessionID string) ([]*models.QuestionResponse, error)
	GetBySessionAndQuestion(ctx context.Context, sessionID, questionID string) (*models.QuestionResponse, error)
	Update(ctx context.Context, response *models.QuestionResponse) error
	MarkCorrect(ctx context.Context, sessionID, questionID string, earnedPoints int, feedback string) error
	MarkIncorrect(ctx context.Context, sessionID, questionID, feedback string) error
	ShowHintLevel(ctx context.Context, sessionID, questionID string, level, penalty int) error
	GetHintLevel(ctx context.Context, sessionID, questionID string) (level int, penalty int, err error)
	GetSessionQuestionStats(ctx context.Context, sessionID string) (earned int, total int, err error)
	InitializeForSession(ctx context.Context, sessionID string, questions []models.Question) error
}

// CheckpointHintRepository defines operations for checkpoint hint progress tracking
type CheckpointHintRepository interface {
	GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointHintProgress, error)
	GetHintLevel(ctx context.Context, sessionID, checkpointID string) (level int, penalty int, err error)
	ShowHintLevel(ctx context.Context, sessionID, checkpointID string, level, penalty int) error
	Create(ctx context.Context, progress *models.CheckpointHintProgress) error
}

// LTIAssignmentRepository defines the interface for LTI assignment persistence.
type LTIAssignmentRepository interface {
	Create(ctx context.Context, assignment *models.LTIAssignment) error
	GetByID(ctx context.Context, id string) (*models.LTIAssignment, error)
	GetByResourceLink(ctx context.Context, courseID, resourceLinkID string) (*models.LTIAssignment, error)
	GetByCourseID(ctx context.Context, courseID string) ([]*models.LTIAssignment, error)
	GetByTemplateID(ctx context.Context, templateID string) ([]*models.LTIAssignment, error)
	Update(ctx context.Context, assignment *models.LTIAssignment) error
	Delete(ctx context.Context, id string) error
	CountByCourse(ctx context.Context, courseID string) (int, error)
	GetWithTemplateByResourceLink(ctx context.Context, courseID, resourceLinkID string) (*LTIAssignmentWithTemplate, error)
	ListActiveTemplates(ctx context.Context, orgID *string) ([]*models.LTITemplateInfo, error)
	Exists(ctx context.Context, courseID, resourceLinkID string) (bool, error)
}
