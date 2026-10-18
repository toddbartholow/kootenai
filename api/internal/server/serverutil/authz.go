package serverutil

import (
	"net/http"
	"slices"

	"github.com/toddbartholow/kootenai/api/internal/auth"
)

// HasRole checks if the user has a specific role.
func HasRole(user *auth.User, role string) bool {
	if user == nil {
		return false
	}
	return slices.Contains(user.Roles, role)
}

// IsAdminUser checks if the user has the admin role.
func IsAdminUser(user *auth.User) bool {
	return HasRole(user, "admin")
}

// IsAdminOrInstructor checks if the user has admin or instructor role.
func IsAdminOrInstructor(user *auth.User) bool {
	return HasRole(user, "admin") || HasRole(user, "instructor")
}

// IsAdminRequest checks if the request is from an admin user.
func IsAdminRequest(r *http.Request) bool {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		return false
	}
	return IsAdminUser(user)
}
