package models

import (
	"encoding/json"
	"time"
)

// Edition represents the product edition
type Edition string

const (
	EditionCommunity    Edition = "community"
	EditionProfessional Edition = "professional"
	EditionEnterprise   Edition = "enterprise"
)

// OrgType represents the type of organization
type OrgType string

const (
	OrgTypeStandard    OrgType = "standard"
	OrgTypeEducational OrgType = "educational"
	OrgTypeEnterprise  OrgType = "enterprise"
)

// OrgRole represents a user's role within an organization
type OrgRole string

const (
	OrgRoleOwner      OrgRole = "owner"
	OrgRoleAdmin      OrgRole = "admin"
	OrgRoleInstructor OrgRole = "instructor"
	OrgRoleMember     OrgRole = "member"
)

// OrgRolePriority returns the priority of a role (higher = more permissions)
func (r OrgRole) Priority() int {
	switch r {
	case OrgRoleOwner:
		return 100
	case OrgRoleAdmin:
		return 80
	case OrgRoleInstructor:
		return 50
	case OrgRoleMember:
		return 10
	default:
		return 0
	}
}

// HasPermission checks if this role has at least the permissions of minRole
func (r OrgRole) HasPermission(minRole OrgRole) bool {
	return r.Priority() >= minRole.Priority()
}

// TeamRole represents a user's role within a team
type TeamRole string

const (
	TeamRoleLead   TeamRole = "lead"
	TeamRoleMember TeamRole = "member"
)

// LabVisibility represents the visibility of a lab template
type LabVisibility string

const (
	LabVisibilityGlobal       LabVisibility = "global"
	LabVisibilityOrganization LabVisibility = "organization"
	LabVisibilityPrivate      LabVisibility = "private"
)

// LicenseStatus represents the validation status of a license
type LicenseStatus string

const (
	LicenseStatusPending LicenseStatus = "pending"
	LicenseStatusValid   LicenseStatus = "valid"
	LicenseStatusExpired LicenseStatus = "expired"
	LicenseStatusRevoked LicenseStatus = "revoked"
)

// Organization represents a tenant in the multi-tenant system
type Organization struct {
	ID                string          `json:"id" db:"id"`
	Name              string          `json:"name" db:"name"`
	Slug              string          `json:"slug" db:"slug"`
	Type              OrgType         `json:"type" db:"type"`
	Edition           Edition         `json:"edition" db:"edition"`
	LicenseKey        string          `json:"-" db:"license_key"` // Never expose
	LicenseExpiresAt  *time.Time      `json:"licenseExpiresAt,omitempty" db:"license_expires_at"`
	Settings          OrgSettings     `json:"settings" db:"settings"`
	MaxUsers          *int            `json:"maxUsers,omitempty" db:"max_users"`
	MaxConcurrentPods *int            `json:"maxConcurrentPods,omitempty" db:"max_concurrent_pods"`
	MaxStorageGB      *int            `json:"maxStorageGb,omitempty" db:"max_storage_gb"`
	LogoURL           string          `json:"logoUrl,omitempty" db:"logo_url"`
	ContactEmail      string          `json:"contactEmail,omitempty" db:"contact_email"`
	IsActive          bool            `json:"isActive" db:"is_active"`
	CreatedAt         time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time       `json:"updatedAt" db:"updated_at"`
	Metadata          json.RawMessage `json:"metadata,omitempty" db:"metadata"`

	// Computed fields (not stored in DB)
	MemberCount int `json:"memberCount,omitempty" db:"-"`
	TeamCount   int `json:"teamCount,omitempty" db:"-"`
}

// OrgSettings holds organization-specific configuration
type OrgSettings struct {
	AllowPublicSignup     bool     `json:"allowPublicSignup"`
	RequireApproval       bool     `json:"requireApproval"`
	DefaultUserRole       OrgRole  `json:"defaultUserRole"`
	AllowedDomains        []string `json:"allowedDomains,omitempty"`
	EnableTeams           bool     `json:"enableTeams"`
	EnableAchievements    bool     `json:"enableAchievements"`
	EnableLeaderboards    bool     `json:"enableLeaderboards"`
	MaxSessionDurationMin int      `json:"maxSessionDurationMins,omitempty"`
	RetentionDays         int      `json:"retentionDays,omitempty"`
}

// Scan implements sql.Scanner for OrgSettings
func (s *OrgSettings) Scan(value any) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// Team represents a group within an organization
type Team struct {
	ID              string          `json:"id" db:"id"`
	OrganizationID  string          `json:"organizationId" db:"organization_id"`
	Name            string          `json:"name" db:"name"`
	Slug            string          `json:"slug" db:"slug"`
	Description     string          `json:"description,omitempty" db:"description"`
	ParentTeamID    *string         `json:"parentTeamId,omitempty" db:"parent_team_id"`
	CanvasSectionID *string         `json:"canvasSectionId,omitempty" db:"canvas_section_id"`
	Settings        json.RawMessage `json:"settings,omitempty" db:"settings"`
	IsActive        bool            `json:"isActive" db:"is_active"`
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" db:"updated_at"`

	// Computed fields
	MemberCount int `json:"memberCount,omitempty" db:"-"`
}

// OrganizationMembership represents a user's membership in an organization
type OrganizationMembership struct {
	ID              string     `json:"id" db:"id"`
	OrganizationID  string     `json:"organizationId" db:"organization_id"`
	UserID          string     `json:"userId" db:"user_id"`
	Role            OrgRole    `json:"role" db:"role"`
	IsPrimary       bool       `json:"isPrimary" db:"is_primary"`
	InvitedBy       *string    `json:"invitedBy,omitempty" db:"invited_by"`
	InvitationToken *string    `json:"-" db:"invitation_token"` // Never expose
	InvitedAt       time.Time  `json:"invitedAt" db:"invited_at"`
	AcceptedAt      *time.Time `json:"acceptedAt,omitempty" db:"accepted_at"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time  `json:"updatedAt" db:"updated_at"`

	// Joined data
	Organization *Organization `json:"organization,omitempty" db:"-"`
	User         *User         `json:"user,omitempty" db:"-"`
}

// IsPending returns true if the membership invitation hasn't been accepted
func (m *OrganizationMembership) IsPending() bool {
	return m.AcceptedAt == nil
}

// TeamMembership represents a user's membership in a team
type TeamMembership struct {
	ID        string    `json:"id" db:"id"`
	TeamID    string    `json:"teamId" db:"team_id"`
	UserID    string    `json:"userId" db:"user_id"`
	Role      TeamRole  `json:"role" db:"role"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`

	// Joined data
	Team *Team `json:"team,omitempty" db:"-"`
	User *User `json:"user,omitempty" db:"-"`
}

// FeatureFlag represents a gated feature
type FeatureFlag struct {
	ID              string          `json:"id" db:"id"`
	Name            string          `json:"name" db:"name"`
	Description     string          `json:"description,omitempty" db:"description"`
	Editions        []Edition       `json:"editions" db:"editions"`
	IsGlobal        bool            `json:"isGlobal" db:"is_global"`
	DefaultSettings json.RawMessage `json:"defaultSettings,omitempty" db:"default_settings"`
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
}

// OrganizationFeature represents an organization-specific feature override
type OrganizationFeature struct {
	OrganizationID string          `json:"organizationId" db:"organization_id"`
	FeatureID      string          `json:"featureId" db:"feature_id"`
	Enabled        bool            `json:"enabled" db:"enabled"`
	ExpiresAt      *time.Time      `json:"expiresAt,omitempty" db:"expires_at"`
	Settings       json.RawMessage `json:"settings,omitempty" db:"settings"`
	GrantedBy      *string         `json:"grantedBy,omitempty" db:"granted_by"`
	GrantedAt      time.Time       `json:"grantedAt" db:"granted_at"`
}

// IsExpired returns true if the feature override has expired
func (f *OrganizationFeature) IsExpired() bool {
	if f.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*f.ExpiresAt)
}

// License represents a product license
type License struct {
	ID               string          `json:"id" db:"id"`
	OrganizationID   string          `json:"organizationId" db:"organization_id"`
	LicenseKey       string          `json:"-" db:"license_key"` // Never expose full key
	Edition          Edition         `json:"edition" db:"edition"`
	IssuedAt         time.Time       `json:"issuedAt" db:"issued_at"`
	ExpiresAt        *time.Time      `json:"expiresAt,omitempty" db:"expires_at"`
	MaxUsers         *int            `json:"maxUsers,omitempty" db:"max_users"`
	MaxPods          *int            `json:"maxPods,omitempty" db:"max_pods"`
	MaxStorageGB     *int            `json:"maxStorageGb,omitempty" db:"max_storage_gb"`
	Features         []string        `json:"features,omitempty" db:"features"`
	IsActive         bool            `json:"isActive" db:"is_active"`
	ValidationStatus LicenseStatus   `json:"validationStatus" db:"validation_status"`
	LastValidatedAt  *time.Time      `json:"lastValidatedAt,omitempty" db:"last_validated_at"`
	ValidationError  *string         `json:"validationError,omitempty" db:"validation_error"`
	CreatedAt        time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time       `json:"updatedAt" db:"updated_at"`
	Metadata         json.RawMessage `json:"metadata,omitempty" db:"metadata"`
}

// IsValid returns true if the license is active and not expired
func (l *License) IsValid() bool {
	if !l.IsActive {
		return false
	}
	if l.ValidationStatus != LicenseStatusValid {
		return false
	}
	if l.ExpiresAt != nil && time.Now().After(*l.ExpiresAt) {
		return false
	}
	return true
}

// MaskedKey returns a masked version of the license key for display
func (l *License) MaskedKey() string {
	if len(l.LicenseKey) < 8 {
		return "****"
	}
	return l.LicenseKey[:4] + "..." + l.LicenseKey[len(l.LicenseKey)-4:]
}

// TenantContext holds the current request's tenant context
type TenantContext struct {
	Organization *Organization
	Team         *Team
	Membership   *OrganizationMembership
	Features     map[string]bool
	Edition      Edition
}

// HasFeature checks if a feature is enabled in this context
func (tc *TenantContext) HasFeature(featureID string) bool {
	if tc.Features == nil {
		return false
	}
	return tc.Features[featureID]
}

// HasRole checks if the user has at least the specified role
func (tc *TenantContext) HasRole(minRole OrgRole) bool {
	if tc.Membership == nil {
		return false
	}
	return tc.Membership.Role.HasPermission(minRole)
}

// IsOwner returns true if the user is the organization owner
func (tc *TenantContext) IsOwner() bool {
	return tc.HasRole(OrgRoleOwner)
}

// IsAdmin returns true if the user is an admin or higher
func (tc *TenantContext) IsAdmin() bool {
	return tc.HasRole(OrgRoleAdmin)
}

// IsInstructor returns true if the user is an instructor or higher
func (tc *TenantContext) IsInstructor() bool {
	return tc.HasRole(OrgRoleInstructor)
}
