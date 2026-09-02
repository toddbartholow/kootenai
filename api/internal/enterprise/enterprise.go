// Package enterprise provides extension points for Kootenai Enterprise Edition.
// Community Edition provides no-op implementations of these interfaces.
// Enterprise Edition replaces them with full implementations via init().
package enterprise

import (
	"context"
	"net/http"
)

// Features defines enterprise-only capabilities.
// CE provides no-op implementations; EE replaces at init.
type Features interface {
	// Edition returns "community" or "enterprise"
	Edition() string

	// IsEnabled checks if a specific enterprise feature is available
	IsEnabled(feature string) bool

	// Multi-tenancy
	GetMultiTenancy() MultiTenancyProvider

	// SSO and identity
	GetSSOProvider() SSOProvider
	GetSCIMProvider() SCIMProvider

	// Advanced audit logging
	GetAuditLogger() AuditLogger

	// CloudStack integration
	GetCloudStackProvider() CloudProvider

	// Analytics
	GetAnalyticsProvider() AnalyticsProvider

	// License information
	GetLicense() LicenseInfo
}

// MultiTenancyProvider handles multi-organization features
type MultiTenancyProvider interface {
	// ListOrganizations returns all organizations (EE: with tenant isolation)
	ListOrganizations(ctx context.Context) ([]Organization, error)

	// CreateOrganization creates a new organization with resource quotas
	CreateOrganization(ctx context.Context, org Organization) error

	// GetOrganizationQuotas returns resource quotas for an organization
	GetOrganizationQuotas(ctx context.Context, orgID string) (*ResourceQuotas, error)

	// SetOrganizationQuotas sets resource quotas for an organization
	SetOrganizationQuotas(ctx context.Context, orgID string, quotas ResourceQuotas) error
}

// SSOProvider handles SAML/OIDC authentication
type SSOProvider interface {
	// HandleSAMLLogin processes SAML authentication requests
	HandleSAMLLogin(w http.ResponseWriter, r *http.Request)

	// HandleOIDCLogin processes OIDC authentication requests
	HandleOIDCLogin(w http.ResponseWriter, r *http.Request)

	// HandleOIDCCallback processes OIDC callback
	HandleOIDCCallback(w http.ResponseWriter, r *http.Request)

	// ConfigureSAML sets up SAML for an organization
	ConfigureSAML(ctx context.Context, orgID string, config SAMLConfig) error

	// ConfigureOIDC sets up OIDC for an organization
	ConfigureOIDC(ctx context.Context, orgID string, config OIDCConfig) error
}

// SCIMProvider handles SCIM user provisioning
type SCIMProvider interface {
	// HandleSCIMRequest processes SCIM API requests
	HandleSCIMRequest(w http.ResponseWriter, r *http.Request)

	// SyncUsers synchronizes users from identity provider
	SyncUsers(ctx context.Context, orgID string) error
}

// AuditLogger provides compliance-grade audit logging
type AuditLogger interface {
	// LogEvent records an audit event with full context
	LogEvent(ctx context.Context, event AuditEvent) error

	// QueryEvents searches audit logs with filters
	QueryEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)

	// ExportEvents exports audit logs for compliance
	ExportEvents(ctx context.Context, filter AuditFilter, format string) ([]byte, error)
}

// CloudProvider handles CloudStack integration
type CloudProvider interface {
	// CreateVM creates a VM in CloudStack
	CreateVM(ctx context.Context, spec VMSpec) (string, error)

	// DeleteVM deletes a VM in CloudStack
	DeleteVM(ctx context.Context, vmID string) error

	// ListVMs lists VMs in CloudStack
	ListVMs(ctx context.Context, filter VMFilter) ([]VM, error)

	// CreateNetwork creates a network in CloudStack
	CreateNetwork(ctx context.Context, spec NetworkSpec) (string, error)
}

// AnalyticsProvider handles advanced analytics
type AnalyticsProvider interface {
	// TrackEvent records an analytics event
	TrackEvent(ctx context.Context, event AnalyticsEvent) error

	// GetDashboard returns analytics dashboard data
	GetDashboard(ctx context.Context, orgID string, timeRange TimeRange) (*Dashboard, error)

	// GenerateReport generates a custom analytics report
	GenerateReport(ctx context.Context, config ReportConfig) ([]byte, error)
}

// LicenseInfo contains license details
type LicenseInfo interface {
	// IsValid returns whether the license is valid
	IsValid() bool

	// CustomerName returns the licensed customer name
	CustomerName() string

	// ExpiresAt returns license expiration time
	ExpiresAt() string

	// MaxUsers returns maximum licensed users (0 = unlimited)
	MaxUsers() int

	// MaxOrganizations returns maximum licensed organizations (0 = unlimited)
	MaxOrganizations() int

	// Features returns list of licensed features
	Features() []string
}

// Default is the global enterprise features instance.
// CE sets this to communityFeatures; EE replaces it in init().
var Default Features = &communityFeatures{}

// Feature constants for IsEnabled checks
const (
	FeatureMultiTenancy    = "multi_tenancy"
	FeatureSSO             = "sso"
	FeatureSCIM            = "scim"
	FeatureCloudStack      = "cloudstack"
	FeatureAdvancedAudit   = "advanced_audit"
	FeatureAdvancedQuotas  = "advanced_quotas"
	FeatureCustomAnalytics = "custom_analytics"
	FeatureHACluster       = "ha_cluster"
	FeatureLabLibrary      = "lab_library"
	FeatureLTI             = "lti"          // LTI 1.3 LMS integration (Canvas, Moodle, Blackboard, etc.)
	FeatureAIClassroom     = "ai_classroom" // AI classroom simulation with virtual students
)
