package server

import (
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
)

func TestIsAdminOrInstructor(t *testing.T) {
	tests := []struct {
		name     string
		user     *auth.User
		expected bool
	}{
		{"nil user returns false", nil, false},
		{"empty roles returns false", &auth.User{ID: "1", Roles: []string{}}, false},
		{"student role returns false", &auth.User{ID: "1", Roles: []string{"student"}}, false},
		{"admin role returns true", &auth.User{ID: "1", Roles: []string{"admin"}}, true},
		{"instructor role returns true", &auth.User{ID: "1", Roles: []string{"instructor"}}, true},
		{"mixed roles with admin returns true", &auth.User{ID: "1", Roles: []string{"student", "admin"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAdminOrInstructor(tt.user)
			if result != tt.expected {
				t.Errorf("isAdminOrInstructor() = %v, want %v", result, tt.expected)
			}
		})
	}
}
