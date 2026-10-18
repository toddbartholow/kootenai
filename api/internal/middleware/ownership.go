// Package middleware provides HTTP middleware for the Kootenai API
package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// OwnershipChecker is a function that checks if a user has access to a resource
type OwnershipChecker func(ctx context.Context, resourceID string, user *auth.User) (bool, error)

// ResourceOwnershipConfig holds configuration for ownership validation
type ResourceOwnershipConfig struct {
	// ResourceParam is the URL parameter name for the resource ID (e.g., "podID", "sessionID")
	ResourceParam string
	// Checker is the function that validates ownership
	Checker OwnershipChecker
	// AllowAdmin allows users with admin role to bypass ownership check
	AllowAdmin bool
	// AllowInstructor allows users with instructor role to bypass ownership check
	AllowInstructor bool
}

// RequireOwnership creates middleware that validates resource ownership
// It uses the authenticated user from context and validates they own the resource
func RequireOwnership(cfg ResourceOwnershipConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get authenticated user
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				writeOwnershipError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check for admin/instructor bypass
			if cfg.AllowAdmin && containsRole(user.Roles, "admin") {
				next.ServeHTTP(w, r)
				return
			}
			if cfg.AllowInstructor && containsRole(user.Roles, "instructor") {
				next.ServeHTTP(w, r)
				return
			}

			// Get resource ID from URL
			resourceID := chi.URLParam(r, cfg.ResourceParam)
			if resourceID == "" {
				writeOwnershipError(w, http.StatusBadRequest, "Resource ID required")
				return
			}

			// Check ownership
			hasAccess, err := cfg.Checker(ctx, resourceID, user)
			if err != nil {
				writeOwnershipError(w, http.StatusInternalServerError, "Failed to verify ownership")
				return
			}

			if !hasAccess {
				writeOwnershipError(w, http.StatusForbidden, "Access denied: you do not own this resource")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OwnershipService provides resource ownership checking functionality
type OwnershipService struct {
	podChecker     OwnershipChecker
	sessionChecker OwnershipChecker
}

// NewOwnershipService creates a new ownership service
func NewOwnershipService(podChecker, sessionChecker OwnershipChecker) *OwnershipService {
	return &OwnershipService{
		podChecker:     podChecker,
		sessionChecker: sessionChecker,
	}
}

// PodOwnershipMiddleware returns middleware for validating pod ownership
func (s *OwnershipService) PodOwnershipMiddleware() func(http.Handler) http.Handler {
	return RequireOwnership(ResourceOwnershipConfig{
		ResourceParam:   "podID",
		Checker:         s.podChecker,
		AllowAdmin:      true,
		AllowInstructor: true, // Instructors can view student pods
	})
}

// SessionOwnershipMiddleware returns middleware for validating session ownership
func (s *OwnershipService) SessionOwnershipMiddleware() func(http.Handler) http.Handler {
	return RequireOwnership(ResourceOwnershipConfig{
		ResourceParam:   "sessionID",
		Checker:         s.sessionChecker,
		AllowAdmin:      true,
		AllowInstructor: true, // Instructors can view student sessions
	})
}

// StrictPodOwnershipMiddleware returns middleware that only allows the owner (no instructor bypass)
func (s *OwnershipService) StrictPodOwnershipMiddleware() func(http.Handler) http.Handler {
	return RequireOwnership(ResourceOwnershipConfig{
		ResourceParam: "podID",
		Checker:       s.podChecker,
		AllowAdmin:    true,
		// No instructor bypass for destructive operations
	})
}

// containsRole checks if a role is present in the list
func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// writeOwnershipError writes a JSON error response
func writeOwnershipError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// OwnershipContext holds ownership information for a request
type OwnershipContext struct {
	ResourceType  string
	ResourceID    string
	OwnerID       string
	RequestUserID string
	IsOwner       bool
	HasAccess     bool
	AccessReason  string // "owner", "admin", "instructor", "team_member", "org_member"
}

// OwnershipContextKey is the context key for ownership context
type OwnershipContextKey struct{}

// OwnershipFromContext retrieves ownership context from request context
func OwnershipFromContext(ctx context.Context) (*OwnershipContext, bool) {
	oc, ok := ctx.Value(OwnershipContextKey{}).(*OwnershipContext)
	return oc, ok
}

// ContextWithOwnership adds ownership context to a context
func ContextWithOwnership(ctx context.Context, oc *OwnershipContext) context.Context {
	return context.WithValue(ctx, OwnershipContextKey{}, oc)
}

// ValidateAndEnrichOwnership creates middleware that validates ownership and adds context
// This is useful when handlers need to know ownership details beyond just allow/deny
func ValidateAndEnrichOwnership(cfg ResourceOwnershipConfig, getOwnerID func(ctx context.Context, resourceID string) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get authenticated user
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				writeOwnershipError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Get resource ID from URL
			resourceID := chi.URLParam(r, cfg.ResourceParam)
			if resourceID == "" {
				writeOwnershipError(w, http.StatusBadRequest, "Resource ID required")
				return
			}

			// Get owner ID
			ownerID, err := getOwnerID(ctx, resourceID)
			if err != nil {
				writeOwnershipError(w, http.StatusInternalServerError, "Failed to get resource owner")
				return
			}

			// Build ownership context
			oc := &OwnershipContext{
				ResourceType:  cfg.ResourceParam,
				ResourceID:    resourceID,
				OwnerID:       ownerID,
				RequestUserID: user.ID,
				IsOwner:       user.ID == ownerID,
			}

			// Determine access
			if oc.IsOwner {
				oc.HasAccess = true
				oc.AccessReason = "owner"
			} else if cfg.AllowAdmin && containsRole(user.Roles, "admin") {
				oc.HasAccess = true
				oc.AccessReason = "admin"
			} else if cfg.AllowInstructor && containsRole(user.Roles, "instructor") {
				oc.HasAccess = true
				oc.AccessReason = "instructor"
			}

			if !oc.HasAccess {
				writeOwnershipError(w, http.StatusForbidden, "Access denied: you do not own this resource")
				return
			}

			// Add ownership context and continue
			ctx = ContextWithOwnership(ctx, oc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OrganizationResourceChecker creates an ownership checker that validates
// both direct ownership and organization membership
func OrganizationResourceChecker(
	directChecker OwnershipChecker,
	getResourceOrg func(ctx context.Context, resourceID string) (*string, error),
	membershipChecker func(ctx context.Context, orgID, userID string) (bool, models.OrgRole, error),
) OwnershipChecker {
	return func(ctx context.Context, resourceID string, user *auth.User) (bool, error) {
		// First check direct ownership
		isOwner, err := directChecker(ctx, resourceID, user)
		if err != nil {
			return false, err
		}
		if isOwner {
			return true, nil
		}

		// Check organization membership
		orgID, err := getResourceOrg(ctx, resourceID)
		if err != nil {
			return false, err
		}
		if orgID == nil {
			return false, nil // No org, no additional access
		}

		isMember, role, err := membershipChecker(ctx, *orgID, user.ID)
		if err != nil {
			return false, err
		}

		// Only instructors and above can access other users' resources in their org
		return isMember && role >= models.OrgRoleInstructor, nil
	}
}
