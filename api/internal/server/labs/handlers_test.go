package labs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestCanAccessLab(t *testing.T) {
	orgID := "org-123"
	userID := "user-456"
	otherUserID := "user-789"
	otherOrgID := "org-other"

	tests := []struct {
		name     string
		user     *auth.User
		lab      *models.LabTemplateRecord
		expected bool
	}{
		{
			name: "global visibility - anyone can access",
			user: nil,
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityGlobal,
			},
			expected: true,
		},
		{
			name: "global visibility - logged in user can access",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityGlobal,
			},
			expected: true,
		},
		{
			name: "organization visibility - no user context",
			user: nil,
			lab: &models.LabTemplateRecord{
				Visibility:     models.LabVisibilityOrganization,
				OrganizationID: &orgID,
			},
			expected: false,
		},
		{
			name: "organization visibility - user in same org",
			user: &auth.User{ID: userID, DefaultOrganizationID: orgID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility:     models.LabVisibilityOrganization,
				OrganizationID: &orgID,
			},
			expected: true,
		},
		{
			name: "organization visibility - user in different org",
			user: &auth.User{ID: userID, DefaultOrganizationID: otherOrgID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility:     models.LabVisibilityOrganization,
				OrganizationID: &orgID,
			},
			expected: false,
		},
		{
			name: "organization visibility - admin can access any org",
			user: &auth.User{ID: userID, DefaultOrganizationID: otherOrgID, Roles: []string{"admin"}},
			lab: &models.LabTemplateRecord{
				Visibility:     models.LabVisibilityOrganization,
				OrganizationID: &orgID,
			},
			expected: true,
		},
		{
			name: "organization visibility - instructor can access any org",
			user: &auth.User{ID: userID, DefaultOrganizationID: otherOrgID, Roles: []string{"instructor"}},
			lab: &models.LabTemplateRecord{
				Visibility:     models.LabVisibilityOrganization,
				OrganizationID: &orgID,
			},
			expected: true,
		},
		{
			name: "private visibility - no user context",
			user: nil,
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityPrivate,
				CreatedBy:  &userID,
			},
			expected: false,
		},
		{
			name: "private visibility - creator can access",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityPrivate,
				CreatedBy:  &userID,
			},
			expected: true,
		},
		{
			name: "private visibility - non-creator cannot access",
			user: &auth.User{ID: otherUserID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityPrivate,
				CreatedBy:  &userID,
			},
			expected: false,
		},
		{
			name: "private visibility - admin can access",
			user: &auth.User{ID: otherUserID, Roles: []string{"admin"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityPrivate,
				CreatedBy:  &userID,
			},
			expected: true,
		},
		{
			name: "private visibility - nil createdBy",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityPrivate,
				CreatedBy:  nil,
			},
			expected: false,
		},
		{
			name: "organization visibility - nil organizationID",
			user: &auth.User{ID: userID, DefaultOrganizationID: orgID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility:     models.LabVisibilityOrganization,
				OrganizationID: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanAccessLab(tt.user, tt.lab)
			if result != tt.expected {
				t.Errorf("CanAccessLab() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanModifyLab(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"

	tests := []struct {
		name     string
		user     *auth.User
		lab      *models.LabTemplateRecord
		expected bool
	}{
		{
			name: "no user context",
			user: nil,
			lab: &models.LabTemplateRecord{
				CreatedBy: &userID,
			},
			expected: false,
		},
		{
			name: "admin can modify any lab",
			user: &auth.User{ID: otherUserID, Roles: []string{"admin"}},
			lab: &models.LabTemplateRecord{
				CreatedBy: &userID,
			},
			expected: true,
		},
		{
			name: "instructor can modify any lab",
			user: &auth.User{ID: otherUserID, Roles: []string{"instructor"}},
			lab: &models.LabTemplateRecord{
				CreatedBy: &userID,
			},
			expected: true,
		},
		{
			name: "creator can modify own lab",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				CreatedBy: &userID,
			},
			expected: true,
		},
		{
			name: "non-creator cannot modify lab",
			user: &auth.User{ID: otherUserID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				CreatedBy: &userID,
			},
			expected: false,
		},
		{
			name: "nil createdBy - regular user cannot modify",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				CreatedBy: nil,
			},
			expected: false,
		},
		{
			name: "nil createdBy - admin can still modify",
			user: &auth.User{ID: userID, Roles: []string{"admin"}},
			lab: &models.LabTemplateRecord{
				CreatedBy: nil,
			},
			expected: true,
		},
		{
			name: "creator can modify global visibility lab",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityGlobal,
				CreatedBy:  &userID,
			},
			expected: true,
		},
		{
			name: "creator can modify organization visibility lab",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityOrganization,
				CreatedBy:  &userID,
			},
			expected: true,
		},
		{
			name: "creator can modify private visibility lab",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			lab: &models.LabTemplateRecord{
				Visibility: models.LabVisibilityPrivate,
				CreatedBy:  &userID,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanModifyLab(tt.user, tt.lab)
			if result != tt.expected {
				t.Errorf("CanModifyLab() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateLabSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "my lab",
			expected: "my-lab",
		},
		{
			name:     "uppercase conversion",
			input:    "My Lab Name",
			expected: "my-lab-name",
		},
		{
			name:     "numbers preserved",
			input:    "Lab 101",
			expected: "lab-101",
		},
		{
			name:     "special characters removed",
			input:    "Lab @ Special # Characters!",
			expected: "lab-special-characters",
		},
		{
			name:     "consecutive spaces",
			input:    "Lab  With   Spaces",
			expected: "lab-with-spaces",
		},
		{
			name:     "leading trailing spaces",
			input:    "  Lab Name  ",
			expected: "lab-name",
		},
		{
			name:     "underscores removed",
			input:    "lab_with_underscores",
			expected: "labwithunderscores",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    "@#$%^&*()",
			expected: "",
		},
		{
			name:     "mixed alphanumeric and special",
			input:    "Lab-v2.0-beta!",
			expected: "lab-v20-beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateLabSlug(tt.input)
			if result != tt.expected {
				t.Errorf("generateLabSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseAndValidateSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid JSON spec",
			spec: `{
				"apiVersion": "v1",
				"kind": "LabTemplate",
				"metadata": {"name": "test-lab"},
				"spec": {}
			}`,
			wantErr: false,
		},
		{
			name: `valid YAML spec`,
			spec: `
apiVersion: v1
kind: LabTemplate
metadata:
  name: test-lab
spec: {}
`,
			wantErr: false,
		},
		{
			name:        "invalid YAML/JSON",
			spec:        "not valid: {yaml: or json",
			wantErr:     true,
			errContains: "must be valid YAML or JSON",
		},
		{
			name: "missing apiVersion",
			spec: `{
				"kind": "LabTemplate",
				"metadata": {"name": "test-lab"},
				"spec": {}
			}`,
			wantErr:     true,
			errContains: "apiVersion is required",
		},
		{
			name: "missing metadata.name",
			spec: `{
				"apiVersion": "v1",
				"kind": "LabTemplate",
				"metadata": {},
				"spec": {}
			}`,
			wantErr:     true,
			errContains: "metadata.name is required",
		},
		{
			name: "spec with objectives/checkpoints",
			spec: `{
				"apiVersion": "v1",
				"kind": "LabTemplate",
				"metadata": {"name": "test-lab"},
				"spec": {
					"objectives": [
						{"id": "obj-1", "description": "First objective", "points": 10}
					]
				}
			}`,
			wantErr: false,
		},
		{
			name:        "completely empty spec",
			spec:        "",
			wantErr:     true,
			errContains: "apiVersion is required",
		},
		{
			name:        "empty JSON object",
			spec:        "{}",
			wantErr:     true,
			errContains: "apiVersion is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specJSON, checkpointsJSON, err := parseAndValidateSpec(tt.spec)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseAndValidateSpec() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseAndValidateSpec() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parseAndValidateSpec() unexpected error: %v", err)
				return
			}

			if specJSON == nil {
				t.Error("parseAndValidateSpec() returned nil specJSON")
			}

			// For spec with objectives, checkpointsJSON should not be nil
			if strings.Contains(tt.spec, "objectives") && checkpointsJSON == nil {
				t.Error("parseAndValidateSpec() should return checkpointsJSON for spec with objectives")
			}
		})
	}
}

// Verify JSON marshaling of exported types.
func TestCreateLabTemplateRequestJSON(t *testing.T) {
	body := `{"name":"test","version":"1.0","platform":"proxmox","spec":"x"}`
	var req CreateLabTemplateRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Name != "test" {
		t.Errorf("Name = %q, want %q", req.Name, "test")
	}
}
