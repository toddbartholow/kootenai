package enterprise

import "time"

// Organization represents a tenant organization (EE feature)
type Organization struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Slug      string      `json:"slug"`
	Settings  OrgSettings `json:"settings"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// OrgSettings contains organization-specific settings
type OrgSettings struct {
	SSOEnabled  bool   `json:"sso_enabled"`
	SSOProvider string `json:"sso_provider"` // "saml" or "oidc"
	DefaultRole string `json:"default_role"`
	AllowSignup bool   `json:"allow_signup"`
}

// ResourceQuotas defines resource limits for an organization
type ResourceQuotas struct {
	MaxUsers      int `json:"max_users"`
	MaxPods       int `json:"max_pods"`
	MaxConcurrent int `json:"max_concurrent_pods"`
	MaxCPUCores   int `json:"max_cpu_cores"`
	MaxMemoryGB   int `json:"max_memory_gb"`
	MaxStorageGB  int `json:"max_storage_gb"`
}

// SAMLConfig contains SAML SSO configuration
type SAMLConfig struct {
	EntityID         string            `json:"entity_id"`
	SSOURL           string            `json:"sso_url"`
	Certificate      string            `json:"certificate"`
	AttributeMapping map[string]string `json:"attribute_mapping"`
}

// OIDCConfig contains OIDC SSO configuration
type OIDCConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Issuer       string   `json:"issuer"`
	Scopes       []string `json:"scopes"`
}

// AuditEvent represents a compliance audit log entry
type AuditEvent struct {
	ID           string         `json:"id"`
	Timestamp    time.Time      `json:"timestamp"`
	ActorID      string         `json:"actor_id"`
	ActorEmail   string         `json:"actor_email"`
	ActorIP      string         `json:"actor_ip"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	OrgID        string         `json:"org_id"`
	Details      map[string]any `json:"details"`
	Result       string         `json:"result"` // "success" or "failure"
}

// AuditFilter defines filters for querying audit logs
type AuditFilter struct {
	OrgID        string    `json:"org_id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Limit        int       `json:"limit"`
	Offset       int       `json:"offset"`
}

// VMSpec defines a CloudStack VM specification
type VMSpec struct {
	Name            string            `json:"name"`
	TemplateID      string            `json:"template_id"`
	ServiceOffering string            `json:"service_offering"`
	NetworkID       string            `json:"network_id"`
	ZoneID          string            `json:"zone_id"`
	Tags            map[string]string `json:"tags"`
}

// VMFilter defines filters for listing CloudStack VMs
type VMFilter struct {
	OrgID  string `json:"org_id"`
	PodID  string `json:"pod_id"`
	State  string `json:"state"`
	ZoneID string `json:"zone_id"`
}

// VM represents a CloudStack virtual machine
type VM struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	State     string    `json:"state"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

// NetworkSpec defines a CloudStack network specification
type NetworkSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ZoneID      string `json:"zone_id"`
	CIDR        string `json:"cidr"`
	Gateway     string `json:"gateway"`
}

// AnalyticsEvent represents an analytics tracking event
type AnalyticsEvent struct {
	Name       string         `json:"name"`
	Timestamp  time.Time      `json:"timestamp"`
	UserID     string         `json:"user_id"`
	OrgID      string         `json:"org_id"`
	Properties map[string]any `json:"properties"`
}

// TimeRange defines a time range for analytics queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Dashboard contains analytics dashboard data
type Dashboard struct {
	TotalUsers     int              `json:"total_users"`
	ActiveUsers    int              `json:"active_users"`
	TotalPods      int              `json:"total_pods"`
	ActivePods     int              `json:"active_pods"`
	CompletionRate float64          `json:"completion_rate"`
	AvgSessionTime float64          `json:"avg_session_time_minutes"`
	TopLabs        []LabUsage       `json:"top_labs"`
	UsageOverTime  []UsageDataPoint `json:"usage_over_time"`
	CustomMetrics  map[string]any   `json:"custom_metrics"`
}

// LabUsage represents lab usage statistics
type LabUsage struct {
	LabID       string  `json:"lab_id"`
	LabName     string  `json:"lab_name"`
	TotalStarts int     `json:"total_starts"`
	Completions int     `json:"completions"`
	AvgTime     float64 `json:"avg_time_minutes"`
}

// UsageDataPoint represents a single data point in usage charts
type UsageDataPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	ActiveUsers int       `json:"active_users"`
	ActivePods  int       `json:"active_pods"`
}

// ReportConfig defines custom report generation parameters
type ReportConfig struct {
	OrgID      string            `json:"org_id"`
	ReportType string            `json:"report_type"`
	TimeRange  TimeRange         `json:"time_range"`
	Format     string            `json:"format"` // "pdf", "csv", "json"
	Filters    map[string]string `json:"filters"`
}
