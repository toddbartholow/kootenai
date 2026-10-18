// Package canvas provides Canvas LMS LTI 1.3 integration
package canvas

import (
	"crypto/rsa"
	"time"
)

// Config holds Canvas LTI configuration
type Config struct {
	// Canvas instance URL (e.g., https://canvas.instructure.com)
	CanvasURL string `yaml:"canvas_url"`

	// LTI 1.3 configuration
	ClientID     string `yaml:"client_id"`
	DeploymentID string `yaml:"deployment_id"`

	// Platform endpoints (auto-discovered from Canvas)
	AuthorizationURL string `yaml:"authorization_url"`
	TokenURL         string `yaml:"token_url"`
	JWKsURL          string `yaml:"jwks_url"`

	// Tool configuration
	ToolIssuer     string `yaml:"tool_issuer"`      // Our platform URL
	ToolPublicKey  string `yaml:"tool_public_key"`  // PEM encoded
	ToolPrivateKey string `yaml:"tool_private_key"` // PEM encoded

	// API access (for grade passback)
	APIAccessToken string `yaml:"api_access_token"` // Optional: for direct API access

	// Timeouts
	TokenExpiry    time.Duration `yaml:"token_expiry"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

// DefaultConfig returns default Canvas configuration
func DefaultConfig() Config {
	return Config{
		TokenExpiry:    time.Hour,
		RequestTimeout: 30 * time.Second,
	}
}

// Credentials holds parsed cryptographic credentials
type Credentials struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// -----------------------------------------------------------------------------
// LTI 1.3 Message Types
// -----------------------------------------------------------------------------

// LTILaunchRequest represents an LTI 1.3 resource link launch request
type LTILaunchRequest struct {
	// Standard OIDC claims
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
	Nonce    string `json:"nonce"`

	// LTI claims
	MessageType        string             `json:"https://purl.imsglobal.org/spec/lti/claim/message_type"`
	Version            string             `json:"https://purl.imsglobal.org/spec/lti/claim/version"`
	DeploymentID       string             `json:"https://purl.imsglobal.org/spec/lti/claim/deployment_id"`
	TargetLinkURI      string             `json:"https://purl.imsglobal.org/spec/lti/claim/target_link_uri"`
	ResourceLink       ResourceLink       `json:"https://purl.imsglobal.org/spec/lti/claim/resource_link"`
	Roles              []string           `json:"https://purl.imsglobal.org/spec/lti/claim/roles"`
	Context            LTIContext         `json:"https://purl.imsglobal.org/spec/lti/claim/context,omitempty"`
	LaunchPresentation LaunchPresentation `json:"https://purl.imsglobal.org/spec/lti/claim/launch_presentation,omitempty"`
	Custom             map[string]string  `json:"https://purl.imsglobal.org/spec/lti/claim/custom,omitempty"`

	// Assignment and Grade Services (AGS)
	AGS *AGSClaim `json:"https://purl.imsglobal.org/spec/lti-ags/claim/endpoint,omitempty"`

	// Names and Roles Provisioning Services (NRPS)
	NRPS *NRPSClaim `json:"https://purl.imsglobal.org/spec/lti-nrps/claim/namesroleservice,omitempty"`

	// Canvas-specific claims
	CanvasUser   *CanvasUser   `json:"https://canvas.instructure.com/lti/user,omitempty"`
	CanvasCourse *CanvasCourse `json:"https://canvas.instructure.com/lti/course,omitempty"`
}

// ResourceLink contains resource link information
type ResourceLink struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// LTIContext contains context/course information
type LTIContext struct {
	ID    string   `json:"id"`
	Label string   `json:"label,omitempty"`
	Title string   `json:"title,omitempty"`
	Type  []string `json:"type,omitempty"`
}

// LaunchPresentation contains launch presentation settings
type LaunchPresentation struct {
	DocumentTarget string `json:"document_target,omitempty"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
	ReturnURL      string `json:"return_url,omitempty"`
	Locale         string `json:"locale,omitempty"`
}

// AGSClaim contains Assignment and Grade Services endpoint info
type AGSClaim struct {
	Scope     []string `json:"scope"`
	LineItem  string   `json:"lineitem,omitempty"`  // Specific line item URL
	LineItems string   `json:"lineitems,omitempty"` // Line items container URL
}

// NRPSClaim contains Names and Roles Provisioning Services endpoint info
type NRPSClaim struct {
	ContextMembershipsURL string   `json:"context_memberships_url"`
	ServiceVersions       []string `json:"service_versions,omitempty"`
}

// CanvasUser contains Canvas-specific user information
type CanvasUser struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	SortableName  string `json:"sortable_name,omitempty"`
	ShortName     string `json:"short_name,omitempty"`
	SISUserID     string `json:"sis_user_id,omitempty"`
	SISImportID   string `json:"sis_import_id,omitempty"`
	IntegrationID string `json:"integration_id,omitempty"`
	LoginID       string `json:"login_id,omitempty"`
	Email         string `json:"email,omitempty"`
}

// CanvasCourse contains Canvas-specific course information
type CanvasCourse struct {
	ID            string `json:"id"`
	SISCourseID   string `json:"sis_course_id,omitempty"`
	CourseCode    string `json:"course_code,omitempty"`
	WorkflowState string `json:"workflow_state,omitempty"`
}

// -----------------------------------------------------------------------------
// Grade Passback Types
// -----------------------------------------------------------------------------

// Score represents a grade submission to Canvas AGS
type Score struct {
	UserID           string    `json:"userId"`
	ScoreGiven       float64   `json:"scoreGiven,omitempty"`
	ScoreMaximum     float64   `json:"scoreMaximum,omitempty"`
	Comment          string    `json:"comment,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	ActivityProgress string    `json:"activityProgress"` // Initialized, Started, InProgress, Submitted, Completed
	GradingProgress  string    `json:"gradingProgress"`  // FullyGraded, Pending, PendingManual, Failed, NotReady
}

// LineItem represents a Canvas gradebook column
type LineItem struct {
	ID             string     `json:"id,omitempty"`
	ScoreMaximum   float64    `json:"scoreMaximum"`
	Label          string     `json:"label"`
	Tag            string     `json:"tag,omitempty"`
	ResourceID     string     `json:"resourceId,omitempty"`
	ResourceLinkID string     `json:"resourceLinkId,omitempty"`
	StartDateTime  *time.Time `json:"startDateTime,omitempty"`
	EndDateTime    *time.Time `json:"endDateTime,omitempty"`
}

// LTI role constants
const (
	RoleInstructor        = "http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor"
	RoleLearner           = "http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"
	RoleTeachingAssistant = "http://purl.imsglobal.org/vocab/lis/v2/membership#TeachingAssistant"
	RoleContentDeveloper  = "http://purl.imsglobal.org/vocab/lis/v2/membership#ContentDeveloper"
	RoleAdministrator     = "http://purl.imsglobal.org/vocab/lis/v2/institution/person#Administrator"
)

// Activity progress constants
const (
	ActivityProgressInitialized = "Initialized"
	ActivityProgressStarted     = "Started"
	ActivityProgressInProgress  = "InProgress"
	ActivityProgressSubmitted   = "Submitted"
	ActivityProgressCompleted   = "Completed"
)

// Grading progress constants
const (
	GradingProgressFullyGraded   = "FullyGraded"
	GradingProgressPending       = "Pending"
	GradingProgressPendingManual = "PendingManual"
	GradingProgressFailed        = "Failed"
	GradingProgressNotReady      = "NotReady"
)
