package enterprise

import (
	"context"
	"errors"
	"log/slog"
)

// ErrEnterpriseRequired is returned when an enterprise feature is requested in CE
var ErrEnterpriseRequired = errors.New("this feature requires Kootenai Enterprise Edition")

// communityFeatures provides no-op implementations for CE
type communityFeatures struct{}

// Edition returns "community" for the community edition
func (c *communityFeatures) Edition() string {
	return "community"
}

// IsEnabled always returns false in community edition
func (c *communityFeatures) IsEnabled(feature string) bool {
	return false
}

// GetMultiTenancy returns nil in community edition
func (c *communityFeatures) GetMultiTenancy() MultiTenancyProvider {
	return nil
}

// GetSSOProvider returns nil in community edition
func (c *communityFeatures) GetSSOProvider() SSOProvider {
	return nil
}

// GetSCIMProvider returns nil in community edition
func (c *communityFeatures) GetSCIMProvider() SCIMProvider {
	return nil
}

// GetAuditLogger returns a basic audit logger for CE
func (c *communityFeatures) GetAuditLogger() AuditLogger {
	return &communityAuditLogger{}
}

// GetCloudStackProvider returns nil in community edition
func (c *communityFeatures) GetCloudStackProvider() CloudProvider {
	return nil
}

// GetAnalyticsProvider returns nil in community edition
func (c *communityFeatures) GetAnalyticsProvider() AnalyticsProvider {
	return nil
}

// GetLicense returns community license info
func (c *communityFeatures) GetLicense() LicenseInfo {
	return &communityLicense{}
}

// communityAuditLogger provides basic audit logging for CE
type communityAuditLogger struct{}

func (l *communityAuditLogger) LogEvent(ctx context.Context, event AuditEvent) error {
	// CE: Log via slog but do not persist to audit table
	slog.InfoContext(ctx, "Audit event (CE - not persisted)",
		"action", event.Action,
		"resource_type", event.ResourceType,
		"resource_id", event.ResourceID,
	)
	return nil
}

func (l *communityAuditLogger) QueryEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	return nil, ErrEnterpriseRequired
}

func (l *communityAuditLogger) ExportEvents(ctx context.Context, filter AuditFilter, format string) ([]byte, error) {
	return nil, ErrEnterpriseRequired
}

// communityLicense represents the community edition "license"
type communityLicense struct{}

func (l *communityLicense) IsValid() bool {
	return true // CE is always valid
}

func (l *communityLicense) CustomerName() string {
	return "Community Edition"
}

func (l *communityLicense) ExpiresAt() string {
	return "" // CE never expires
}

func (l *communityLicense) MaxUsers() int {
	return 0 // 0 = unlimited in CE
}

func (l *communityLicense) MaxOrganizations() int {
	return 1 // CE supports single organization only
}

func (l *communityLicense) Features() []string {
	return []string{
		"proxmox",
		"basic_audit",
		"single_org",
		"local_auth",
	}
}
