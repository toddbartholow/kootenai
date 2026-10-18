package sessions

import (
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// strPtr is a helper to create string pointers
func strPtr(s string) *string {
	return &s
}

func TestCanAccessSession(t *testing.T) {
	tests := []struct {
		name     string
		user     *auth.User
		session  *models.Session
		expected bool
	}{
		{
			name:     "nil user",
			user:     nil,
			session:  &models.Session{UserID: "user-1"},
			expected: false,
		},
		{
			name:     "owner can access own session",
			user:     &auth.User{ID: "user-1", Email: "user@example.com"},
			session:  &models.Session{UserID: "user-1"},
			expected: true,
		},
		{
			name:     "admin can access any session",
			user:     &auth.User{ID: "admin-1", Email: "admin@example.com", Roles: []string{"admin"}},
			session:  &models.Session{UserID: "user-1"},
			expected: true,
		},
		{
			name: "org member can access org session",
			user: &auth.User{ID: "user-2", Email: "user2@example.com", DefaultOrganizationID: "org-1"},
			session: &models.Session{
				UserID:         "user-1",
				OrganizationID: strPtr("org-1"),
			},
			expected: true,
		},
		{
			name: "org member cannot access other org session",
			user: &auth.User{ID: "user-2", Email: "user2@example.com", DefaultOrganizationID: "org-2"},
			session: &models.Session{
				UserID:         "user-1",
				OrganizationID: strPtr("org-1"),
			},
			expected: false,
		},
		{
			name:     "different user cannot access",
			user:     &auth.User{ID: "user-2", Email: "user2@example.com"},
			session:  &models.Session{UserID: "user-1"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canAccessSession(tt.user, tt.session)
			if result != tt.expected {
				t.Errorf("canAccessSession() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanModifySession(t *testing.T) {
	tests := []struct {
		name     string
		user     *auth.User
		session  *models.Session
		expected bool
	}{
		{
			name:     "nil user",
			user:     nil,
			session:  &models.Session{UserID: "user-1"},
			expected: false,
		},
		{
			name:     "owner can modify own session",
			user:     &auth.User{ID: "user-1", Email: "user@example.com"},
			session:  &models.Session{UserID: "user-1"},
			expected: true,
		},
		{
			name:     "admin can modify any session",
			user:     &auth.User{ID: "admin-1", Email: "admin@example.com", Roles: []string{"admin"}},
			session:  &models.Session{UserID: "user-1"},
			expected: true,
		},
		{
			name:     "non-owner cannot modify",
			user:     &auth.User{ID: "user-2", Email: "user2@example.com"},
			session:  &models.Session{UserID: "user-1"},
			expected: false,
		},
		{
			name: "org member cannot modify other user session",
			user: &auth.User{ID: "user-2", Email: "user2@example.com", DefaultOrganizationID: "org-1"},
			session: &models.Session{
				UserID:         "user-1",
				OrganizationID: strPtr("org-1"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canModifySession(tt.user, tt.session)
			if result != tt.expected {
				t.Errorf("canModifySession() = %v, want %v", result, tt.expected)
			}
		})
	}
}
