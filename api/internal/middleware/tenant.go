// Package middleware provides HTTP middleware for the Kootenai API
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// TenantContextKey is the context key for tenant context
type TenantContextKey struct{}

// TenantMiddlewareConfig holds configuration for tenant middleware
type TenantMiddlewareConfig struct {
	// DefaultOrgID is the fallback organization ID when none is specified
	DefaultOrgID string

	// RequireOrg determines if requests must have an organization context
	// If false, requests without org context are allowed (for backward compatibility)
	RequireOrg bool

	// HeaderName is the header used to specify organization (default: X-Organization)
	HeaderName string
}

// DefaultTenantConfig returns default tenant middleware configuration
func DefaultTenantConfig() TenantMiddlewareConfig {
	return TenantMiddlewareConfig{
		DefaultOrgID: "",
		RequireOrg:   false,
		HeaderName:   "X-Organization",
	}
}

// TenantService provides tenant resolution functionality
type TenantService struct {
	orgRepo        repositories.OrganizationRepository
	membershipRepo repositories.OrganizationMembershipRepository
	featureRepo    repositories.FeatureRepository
	config         TenantMiddlewareConfig
}

// NewTenantService creates a new tenant service
func NewTenantService(
	orgRepo repositories.OrganizationRepository,
	membershipRepo repositories.OrganizationMembershipRepository,
	featureRepo repositories.FeatureRepository,
	config TenantMiddlewareConfig,
) *TenantService {
	return &TenantService{
		orgRepo:        orgRepo,
		membershipRepo: membershipRepo,
		featureRepo:    featureRepo,
		config:         config,
	}
}

// Middleware creates an HTTP middleware that resolves tenant context
// It extracts organization context from (in order of priority):
// 1. URL path parameter (orgID or organizationID)
// 2. X-Organization header (or custom header)
// 3. Subdomain (for SaaS mode)
// 4. User's primary organization
// 5. Default organization (if configured)
func (s *TenantService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get authenticated user
		user, hasUser := auth.UserFromContext(ctx)

		// Try to resolve organization ID from various sources
		orgID := s.resolveOrgID(r, user)

		// If no org found and required, return error
		if orgID == "" && s.config.RequireOrg {
			writeJSONError(w, http.StatusBadRequest, "Organization context required")
			return
		}

		// If no org found and not required, continue without tenant context
		if orgID == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Load organization
		org, err := s.orgRepo.GetByID(ctx, orgID)
		if err != nil {
			slog.Warn("Tenant middleware: failed to load organization", "orgID", orgID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to load organization")
			return
		}
		if org == nil {
			// Try by slug
			org, err = s.orgRepo.GetBySlug(ctx, orgID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "Failed to load organization")
				return
			}
		}
		if org == nil {
			writeJSONError(w, http.StatusNotFound, "Organization not found")
			return
		}

		// Check if organization is active
		if !org.IsActive {
			writeJSONError(w, http.StatusForbidden, "Organization is inactive")
			return
		}

		// Build tenant context
		tenantCtx := &models.TenantContext{
			Organization: org,
			Edition:      org.Edition,
			Features:     make(map[string]bool),
		}

		// If user is authenticated, validate their membership
		if hasUser && s.membershipRepo != nil {
			membership, err := s.membershipRepo.GetByOrgAndUser(ctx, org.ID, user.ID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "Failed to load membership")
				return
			}

			// User MUST be an accepted member to access this organization
			if membership == nil || membership.AcceptedAt == nil {
				writeJSONError(w, http.StatusForbidden, "Not a member of this organization")
				return
			}
			tenantCtx.Membership = membership
		}

		// Load enabled features for the organization
		if s.featureRepo != nil {
			features, err := s.featureRepo.GetOrganizationFeatures(ctx, org.ID)
			if err == nil {
				tenantCtx.Features = features
			}
		}

		// Add tenant context to request context
		ctx = context.WithValue(ctx, TenantContextKey{}, tenantCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveOrgID attempts to resolve organization ID from various sources
func (s *TenantService) resolveOrgID(r *http.Request, user *auth.User) string {
	// 1. URL path parameter
	if orgID := chi.URLParam(r, "orgID"); orgID != "" {
		return orgID
	}
	if orgID := chi.URLParam(r, "organizationID"); orgID != "" {
		return orgID
	}

	// 2. Header
	headerName := s.config.HeaderName
	if headerName == "" {
		headerName = "X-Organization"
	}
	if orgID := r.Header.Get(headerName); orgID != "" {
		return orgID
	}

	// 3. Subdomain (for SaaS mode)
	// Format: org-slug.example.com
	// Skip when host is an IP address (contains only digits and dots before port).
	host := r.Host
	if colonIdx := strings.LastIndex(host, ":"); colonIdx > 0 {
		host = host[:colonIdx] // strip port
	}
	if !isIPAddress(host) {
		if idx := strings.Index(host, "."); idx > 0 {
			subdomain := host[:idx]
			// Skip common subdomains
			if subdomain != "www" && subdomain != "api" && subdomain != "app" {
				return subdomain
			}
		}
	}

	// 4. User's primary/default organization
	if user != nil && s.membershipRepo != nil {
		primaryOrg, err := s.membershipRepo.GetPrimaryOrganization(r.Context(), user.ID)
		if err == nil && primaryOrg != nil {
			return primaryOrg.ID
		}
	}

	// 5. Default organization
	if s.config.DefaultOrgID != "" {
		return s.config.DefaultOrgID
	}

	return ""
}

// TenantFromContext retrieves the tenant context from the request context
func TenantFromContext(ctx context.Context) (*models.TenantContext, bool) {
	tc, ok := ctx.Value(TenantContextKey{}).(*models.TenantContext)
	return tc, ok
}

// RequireTenant creates middleware that requires a valid tenant context
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := TenantFromContext(r.Context())
		if !ok || tc == nil || tc.Organization == nil {
			writeJSONError(w, http.StatusBadRequest, "Organization context required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOrgRole creates middleware that requires a minimum organization role
func RequireOrgRole(minRole models.OrgRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, ok := TenantFromContext(r.Context())
			if !ok || tc == nil {
				writeJSONError(w, http.StatusBadRequest, "Organization context required")
				return
			}

			if tc.Membership == nil {
				writeJSONError(w, http.StatusForbidden, "Organization membership required")
				return
			}

			if !tc.HasRole(minRole) {
				writeJSONError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOrgOwner is a convenience middleware requiring owner role
func RequireOrgOwner(next http.Handler) http.Handler {
	return RequireOrgRole(models.OrgRoleOwner)(next)
}

// RequireOrgAdmin is a convenience middleware requiring admin or higher role
func RequireOrgAdmin(next http.Handler) http.Handler {
	return RequireOrgRole(models.OrgRoleAdmin)(next)
}

// RequireOrgInstructor is a convenience middleware requiring instructor or higher role
func RequireOrgInstructor(next http.Handler) http.Handler {
	return RequireOrgRole(models.OrgRoleInstructor)(next)
}

// RequireOrgMember is a convenience middleware requiring any member role
func RequireOrgMember(next http.Handler) http.Handler {
	return RequireOrgRole(models.OrgRoleMember)(next)
}

// RequireFeature creates middleware that requires a specific feature to be enabled
func RequireFeature(featureID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, ok := TenantFromContext(r.Context())
			if !ok || tc == nil {
				writeJSONError(w, http.StatusBadRequest, "Organization context required")
				return
			}

			if !tc.HasFeature(featureID) {
				// Return 402 Payment Required for feature gating
				writeJSONError(w, http.StatusPaymentRequired, "Feature not available in your edition")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyFeature creates middleware that requires at least one of the features to be enabled
func RequireAnyFeature(featureIDs ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, ok := TenantFromContext(r.Context())
			if !ok || tc == nil {
				writeJSONError(w, http.StatusBadRequest, "Organization context required")
				return
			}

			// Check if any of the features is enabled
			for _, featureID := range featureIDs {
				if tc.HasFeature(featureID) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Return 402 Payment Required for feature gating
			writeJSONError(w, http.StatusPaymentRequired, "Feature not available in your edition")
		})
	}
}

// RequireAllFeatures creates middleware that requires all specified features to be enabled
func RequireAllFeatures(featureIDs ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, ok := TenantFromContext(r.Context())
			if !ok || tc == nil {
				writeJSONError(w, http.StatusBadRequest, "Organization context required")
				return
			}

			// Check if all features are enabled
			for _, featureID := range featureIDs {
				if !tc.HasFeature(featureID) {
					writeJSONError(w, http.StatusPaymentRequired, "Feature not available in your edition")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireEdition creates middleware that requires a minimum edition level
func RequireEdition(minEdition models.Edition) func(http.Handler) http.Handler {
	editionPriority := map[models.Edition]int{
		models.EditionCommunity:    1,
		models.EditionProfessional: 2,
		models.EditionEnterprise:   3,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, ok := TenantFromContext(r.Context())
			if !ok || tc == nil {
				writeJSONError(w, http.StatusBadRequest, "Organization context required")
				return
			}

			if editionPriority[tc.Edition] < editionPriority[minEdition] {
				writeJSONError(w, http.StatusPaymentRequired, "Feature requires "+string(minEdition)+" edition or higher")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// isIPAddress returns true if the host string looks like an IPv4 or IPv6
// address (contains only digits/dots/colons). This prevents the subdomain
// resolver from treating "192" in "192.0.2.10" as an org slug.
func isIPAddress(host string) bool {
	for _, c := range host {
		if (c < '0' || c > '9') && c != '.' && c != ':' && c != '[' && c != ']' {
			return false
		}
	}
	return len(host) > 0
}
