package organizations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Mock repositories for testing
type mockOrgRepoHandler struct {
	orgs        map[string]*models.Organization
	memberCount int
	teamCount   int
	createErr   error
	updateErr   error
	deleteErr   error
}

func (m *mockOrgRepoHandler) Create(ctx context.Context, org *models.Organization) error {
	if m.createErr != nil {
		return m.createErr
	}
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()
	m.orgs[org.ID] = org
	return nil
}

func (m *mockOrgRepoHandler) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	return m.orgs[id], nil
}

func (m *mockOrgRepoHandler) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	for _, org := range m.orgs {
		if org.Slug == slug {
			return org, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepoHandler) List(ctx context.Context, filter repositories.OrganizationFilter) ([]*models.Organization, error) {
	var result []*models.Organization
	for _, org := range m.orgs {
		result = append(result, org)
	}
	return result, nil
}

func (m *mockOrgRepoHandler) Update(ctx context.Context, org *models.Organization) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.orgs[org.ID] = org
	return nil
}

func (m *mockOrgRepoHandler) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.orgs, id)
	return nil
}

func (m *mockOrgRepoHandler) SetActive(ctx context.Context, id string, active bool) error {
	if org, ok := m.orgs[id]; ok {
		org.IsActive = active
	}
	return nil
}

func (m *mockOrgRepoHandler) GetMemberCount(ctx context.Context, id string) (int, error) {
	return m.memberCount, nil
}

func (m *mockOrgRepoHandler) GetTeamCount(ctx context.Context, id string) (int, error) {
	return m.teamCount, nil
}

func (m *mockOrgRepoHandler) GetByIDs(ctx context.Context, ids []string) ([]*models.Organization, error) {
	var result []*models.Organization
	for _, id := range ids {
		if org, ok := m.orgs[id]; ok {
			result = append(result, org)
		}
	}
	return result, nil
}

func (m *mockOrgRepoHandler) GetMemberCountBatch(ctx context.Context, ids []string) (map[string]int, error) {
	counts := make(map[string]int, len(ids))
	for _, id := range ids {
		counts[id] = m.memberCount
	}
	return counts, nil
}

func (m *mockOrgRepoHandler) GetTeamCountBatch(ctx context.Context, ids []string) (map[string]int, error) {
	counts := make(map[string]int, len(ids))
	for _, id := range ids {
		counts[id] = m.teamCount
	}
	return counts, nil
}

type mockMembershipRepoHandler struct {
	memberships map[string]*models.OrganizationMembership
	createErr   error
}

func (m *mockMembershipRepoHandler) Create(ctx context.Context, membership *models.OrganizationMembership) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.memberships[membership.ID] = membership
	return nil
}

func (m *mockMembershipRepoHandler) GetByID(ctx context.Context, id string) (*models.OrganizationMembership, error) {
	return m.memberships[id], nil
}

func (m *mockMembershipRepoHandler) GetByOrgAndUser(ctx context.Context, orgID, userID string) (*models.OrganizationMembership, error) {
	for _, mem := range m.memberships {
		if mem.OrganizationID == orgID && mem.UserID == userID {
			return mem, nil
		}
	}
	return nil, nil
}

func (m *mockMembershipRepoHandler) ListByOrganization(ctx context.Context, orgID string, filter repositories.MembershipFilter) ([]*models.OrganizationMembership, error) {
	var result []*models.OrganizationMembership
	for _, mem := range m.memberships {
		if mem.OrganizationID == orgID {
			if filter.IncludePending || mem.AcceptedAt != nil {
				result = append(result, mem)
			}
		}
	}
	return result, nil
}

func (m *mockMembershipRepoHandler) ListByUser(ctx context.Context, userID string) ([]*models.OrganizationMembership, error) {
	var result []*models.OrganizationMembership
	for _, mem := range m.memberships {
		if mem.UserID == userID {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *mockMembershipRepoHandler) Update(ctx context.Context, membership *models.OrganizationMembership) error {
	m.memberships[membership.ID] = membership
	return nil
}

func (m *mockMembershipRepoHandler) UpdateRole(ctx context.Context, id string, role models.OrgRole) error {
	if mem, ok := m.memberships[id]; ok {
		mem.Role = role
	}
	return nil
}

func (m *mockMembershipRepoHandler) Delete(ctx context.Context, id string) error {
	delete(m.memberships, id)
	return nil
}

func (m *mockMembershipRepoHandler) AcceptInvitation(ctx context.Context, token string) (*models.OrganizationMembership, error) {
	for _, mem := range m.memberships {
		if mem.InvitationToken != nil && *mem.InvitationToken == token {
			now := time.Now()
			mem.AcceptedAt = &now
			return mem, nil
		}
	}
	return nil, nil
}

func (m *mockMembershipRepoHandler) SetPrimary(ctx context.Context, userID, orgID string) error {
	return nil
}

func (m *mockMembershipRepoHandler) GetPrimaryOrganization(ctx context.Context, userID string) (*models.Organization, error) {
	return nil, nil
}

// Test helpers

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createTestServerWithOrgs(t *testing.T, orgRepo *mockOrgRepoHandler, membershipRepo *mockMembershipRepoHandler) *Manager {
	t.Helper()

	cfg := Config{
		Logger: newTestLogger(),
	}
	if orgRepo != nil {
		cfg.OrgRepo = orgRepo
	}
	if membershipRepo != nil {
		cfg.OrgMembershipRepo = membershipRepo
	}

	return NewManager(cfg)
}

func addUserToContext(r *http.Request, user *auth.User) *http.Request {
	ctx := auth.ContextWithUser(r.Context(), user)
	return r.WithContext(ctx)
}

func addTenantToContext(r *http.Request, tc *models.TenantContext) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.TenantContextKey{}, tc)
	return r.WithContext(ctx)
}

// Tests

func TestHandleListOrganizations(t *testing.T) {
	now := time.Now()
	orgRepo := &mockOrgRepoHandler{
		orgs: map[string]*models.Organization{
			"org-1": {
				ID:       "org-1",
				Name:     "Test Org",
				Slug:     "test-org",
				Type:     models.OrgTypeStandard,
				Edition:  models.EditionCommunity,
				IsActive: true,
			},
		},
	}
	membershipRepo := &mockMembershipRepoHandler{
		memberships: map[string]*models.OrganizationMembership{
			"mem-1": {
				ID:             "mem-1",
				OrganizationID: "org-1",
				UserID:         "user-1",
				Role:           models.OrgRoleMember,
				IsPrimary:      true,
				AcceptedAt:     &now,
			},
		},
	}

	orgMgr := createTestServerWithOrgs(t, orgRepo, membershipRepo)

	tests := []struct {
		name       string
		user       *auth.User
		wantStatus int
	}{
		{
			name:       "success",
			user:       &auth.User{ID: "user-1", Email: "test@example.com"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "no user context",
			user:       nil,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations", nil)
			if tt.user != nil {
				req = addUserToContext(req, tt.user)
			}
			w := httptest.NewRecorder()

			orgMgr.handleListOrganizations()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				orgs, ok := resp["organizations"].([]interface{})
				if !ok {
					t.Fatal("expected organizations array in response")
				}
				if len(orgs) != 1 {
					t.Errorf("expected 1 organization, got %d", len(orgs))
				}
			}
		})
	}
}

func TestHandleCreateOrganization(t *testing.T) {
	orgRepo := &mockOrgRepoHandler{
		orgs: make(map[string]*models.Organization),
	}
	membershipRepo := &mockMembershipRepoHandler{
		memberships: make(map[string]*models.OrganizationMembership),
	}

	orgMgr := createTestServerWithOrgs(t, orgRepo, membershipRepo)
	user := &auth.User{ID: "user-1", Email: "test@example.com"}

	tests := []struct {
		name       string
		body       CreateOrganizationRequest
		wantStatus int
	}{
		{
			name: "success",
			body: CreateOrganizationRequest{
				Name: "New Organization",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "success with slug",
			body: CreateOrganizationRequest{
				Name: "Another Org",
				Slug: "another-org",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: CreateOrganizationRequest{
				Slug: "no-name",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
			req = addUserToContext(req, user)
			w := httptest.NewRecorder()

			orgMgr.handleCreateOrganization()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusCreated {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if resp["name"] != tt.body.Name {
					t.Errorf("expected name %s, got %s", tt.body.Name, resp["name"])
				}
			}
		})
	}
}

func TestHandleGetOrganization(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		Type:     models.OrgTypeStandard,
		Edition:  models.EditionCommunity,
		IsActive: true,
	}

	orgRepo := &mockOrgRepoHandler{
		orgs:        map[string]*models.Organization{"org-1": org},
		memberCount: 5,
		teamCount:   2,
	}

	orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

	tests := []struct {
		name       string
		tenantCtx  *models.TenantContext
		wantStatus int
	}{
		{
			name: "success",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "no tenant context",
			tenantCtx:  nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1", nil)
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			w := httptest.NewRecorder()

			orgMgr.handleGetOrganization()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if resp["name"] != org.Name {
					t.Errorf("expected name %s, got %s", org.Name, resp["name"])
				}
				if int(resp["memberCount"].(float64)) != 5 {
					t.Errorf("expected memberCount 5, got %v", resp["memberCount"])
				}
			}
		})
	}
}

func TestHandleUpdateOrganization(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		Type:     models.OrgTypeStandard,
		Edition:  models.EditionCommunity,
		IsActive: true,
	}

	orgRepo := &mockOrgRepoHandler{
		orgs: map[string]*models.Organization{"org-1": org},
	}

	orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

	tests := []struct {
		name       string
		tenantCtx  *models.TenantContext
		body       UpdateOrganizationRequest
		wantStatus int
	}{
		{
			name: "success - admin update",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			body:       UpdateOrganizationRequest{Name: stringPtr("Updated Name")},
			wantStatus: http.StatusOK,
		},
		{
			name: "forbidden - member cannot update",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			body:       UpdateOrganizationRequest{Name: stringPtr("Updated Name")},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no tenant context",
			tenantCtx:  nil,
			body:       UpdateOrganizationRequest{Name: stringPtr("Updated Name")},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/org-1", bytes.NewReader(body))
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			w := httptest.NewRecorder()

			orgMgr.handleUpdateOrganization()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleDeleteOrganization(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		org        *models.Organization
		tenantCtx  *models.TenantContext
		wantStatus int
	}{
		{
			name: "success - owner delete",
			org: &models.Organization{
				ID:       "org-1",
				Name:     "Test Org",
				Slug:     "test-org",
				IsActive: true,
			},
			tenantCtx: &models.TenantContext{
				Organization: &models.Organization{ID: "org-1", Slug: "test-org"},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleOwner,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "forbidden - admin cannot delete",
			org: &models.Organization{
				ID:       "org-2",
				Name:     "Test Org 2",
				Slug:     "test-org-2",
				IsActive: true,
			},
			tenantCtx: &models.TenantContext{
				Organization: &models.Organization{ID: "org-2", Slug: "test-org-2"},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "forbidden - cannot delete system org",
			org: &models.Organization{
				ID:       "org-3",
				Name:     "System",
				Slug:     "system",
				IsActive: true,
			},
			tenantCtx: &models.TenantContext{
				Organization: &models.Organization{ID: "org-3", Slug: "system"},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleOwner,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgRepo := &mockOrgRepoHandler{
				orgs: map[string]*models.Organization{tt.org.ID: tt.org},
			}
			orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/"+tt.org.ID, nil)
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			w := httptest.NewRecorder()

			orgMgr.handleDeleteOrganization()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleListMembers(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	membershipRepo := &mockMembershipRepoHandler{
		memberships: map[string]*models.OrganizationMembership{
			"mem-1": {
				ID:             "mem-1",
				OrganizationID: "org-1",
				UserID:         "user-1",
				Role:           models.OrgRoleOwner,
				AcceptedAt:     &now,
			},
			"mem-2": {
				ID:             "mem-2",
				OrganizationID: "org-1",
				UserID:         "user-2",
				Role:           models.OrgRoleMember,
				AcceptedAt:     &now,
			},
		},
	}

	orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

	tenantCtx := &models.TenantContext{
		Organization: org,
		Membership: &models.OrganizationMembership{
			Role:       models.OrgRoleMember,
			AcceptedAt: &now,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/members", nil)
	req = addTenantToContext(req, tenantCtx)
	w := httptest.NewRecorder()

	orgMgr.handleListMembers()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	members, ok := resp["members"].([]interface{})
	if !ok {
		t.Fatal("expected members array in response")
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestHandleInviteMember(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	membershipRepo := &mockMembershipRepoHandler{
		memberships: make(map[string]*models.OrganizationMembership),
	}

	orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

	tests := []struct {
		name       string
		tenantCtx  *models.TenantContext
		body       InviteMemberRequest
		wantStatus int
	}{
		{
			name: "success - admin invites member",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			body: InviteMemberRequest{
				UserID: "user-2",
				Role:   models.OrgRoleMember,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "forbidden - member cannot invite",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			body: InviteMemberRequest{
				UserID: "user-3",
				Role:   models.OrgRoleMember,
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "forbidden - admin cannot invite admin",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			body: InviteMemberRequest{
				UserID: "user-4",
				Role:   models.OrgRoleAdmin,
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset memberships for each test
			membershipRepo.memberships = make(map[string]*models.OrganizationMembership)

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/members", bytes.NewReader(body))
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			req = addUserToContext(req, &auth.User{ID: "inviter-1"})
			w := httptest.NewRecorder()

			orgMgr.handleInviteMember()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleAcceptInvitation(t *testing.T) {
	token := "valid-token"
	membershipRepo := &mockMembershipRepoHandler{
		memberships: map[string]*models.OrganizationMembership{
			"mem-1": {
				ID:              "mem-1",
				OrganizationID:  "org-1",
				UserID:          "user-1",
				Role:            models.OrgRoleMember,
				InvitationToken: &token,
			},
		},
	}

	orgRepo := &mockOrgRepoHandler{
		orgs: map[string]*models.Organization{
			"org-1": {
				ID:   "org-1",
				Name: "Test Org",
				Slug: "test-org",
			},
		},
	}

	orgMgr := createTestServerWithOrgs(t, orgRepo, membershipRepo)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{
			name:       "success",
			token:      "valid-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid token",
			token:      "invalid-token",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up chi URL params
			r := chi.NewRouter()
			r.Post("/invitations/{token}/accept", orgMgr.handleAcceptInvitation())

			req := httptest.NewRequest(http.MethodPost, "/invitations/"+tt.token+"/accept", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}

	t.Run("error when organization not found", func(t *testing.T) {
		missingOrgToken := "missing-org-token"
		localMembershipRepo := &mockMembershipRepoHandler{
			memberships: map[string]*models.OrganizationMembership{
				"mem-2": {
					ID:              "mem-2",
					OrganizationID:  "nonexistent-org",
					UserID:          "user-2",
					Role:            models.OrgRoleMember,
					InvitationToken: &missingOrgToken,
				},
			},
		}

		localOrgRepo := &mockOrgRepoHandler{
			orgs: map[string]*models.Organization{}, // No orgs
		}

		localOrgMgr := createTestServerWithOrgs(t, localOrgRepo, localMembershipRepo)

		r := chi.NewRouter()
		r.Post("/invitations/{token}/accept", localOrgMgr.handleAcceptInvitation())

		req := httptest.NewRequest(http.MethodPost, "/invitations/missing-org-token/accept", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusInternalServerError, w.Body.String())
		}
	})
}

func TestHandleUpdateMember(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	tests := []struct {
		name       string
		userID     string
		newRole    models.OrgRole
		tenantCtx  *models.TenantContext
		wantStatus int
	}{
		{
			name:    "success - admin updates member role",
			userID:  "user-2",
			newRole: models.OrgRoleInstructor,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "forbidden - member cannot update roles",
			userID:  "user-2",
			newRole: models.OrgRoleInstructor,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no tenant context",
			userID:     "user-2",
			newRole:    models.OrgRoleInstructor,
			tenantCtx:  nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:    "member not found",
			userID:  "nonexistent-user",
			newRole: models.OrgRoleInstructor,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			membershipRepo := &mockMembershipRepoHandler{
				memberships: map[string]*models.OrganizationMembership{
					"m-1": {
						ID:             "m-1",
						OrganizationID: "org-1",
						UserID:         "user-2",
						Role:           models.OrgRoleMember,
						AcceptedAt:     &now,
					},
				},
			}

			orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

			r := chi.NewRouter()
			r.Patch("/organizations/{orgID}/members/{userID}", orgMgr.handleUpdateMember())

			body, _ := json.Marshal(UpdateMemberRequest{Role: tt.newRole})
			req := httptest.NewRequest(http.MethodPatch, "/organizations/org-1/members/"+tt.userID, bytes.NewReader(body))
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleRemoveMember(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	tests := []struct {
		name        string
		userID      string
		tenantCtx   *models.TenantContext
		memberships map[string]*models.OrganizationMembership
		wantStatus  int
	}{
		{
			name:   "success - admin removes member",
			userID: "user-3",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			memberships: map[string]*models.OrganizationMembership{
				"m-1": {
					ID:             "m-1",
					OrganizationID: "org-1",
					UserID:         "user-1",
					Role:           models.OrgRoleOwner,
					AcceptedAt:     &now,
				},
				"m-2": {
					ID:             "m-2",
					OrganizationID: "org-1",
					UserID:         "user-2",
					Role:           models.OrgRoleOwner,
					AcceptedAt:     &now,
				},
				"m-3": {
					ID:             "m-3",
					OrganizationID: "org-1",
					UserID:         "user-3",
					Role:           models.OrgRoleMember,
					AcceptedAt:     &now,
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "forbidden - member cannot remove members",
			userID: "user-3",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			memberships: map[string]*models.OrganizationMembership{
				"m-3": {
					ID:             "m-3",
					OrganizationID: "org-1",
					UserID:         "user-3",
					Role:           models.OrgRoleMember,
				},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no tenant context",
			userID:     "user-3",
			tenantCtx:  nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "member not found",
			userID: "nonexistent-user",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			memberships: map[string]*models.OrganizationMembership{},
			wantStatus:  http.StatusNotFound,
		},
		{
			name:   "cannot remove last owner",
			userID: "user-1",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleOwner,
					AcceptedAt: &now,
				},
			},
			memberships: map[string]*models.OrganizationMembership{
				"m-1": {
					ID:             "m-1",
					OrganizationID: "org-1",
					UserID:         "user-1",
					Role:           models.OrgRoleOwner,
					AcceptedAt:     &now,
				},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			membershipRepo := &mockMembershipRepoHandler{
				memberships: tt.memberships,
			}
			if membershipRepo.memberships == nil {
				membershipRepo.memberships = make(map[string]*models.OrganizationMembership)
			}

			orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

			r := chi.NewRouter()
			r.Delete("/organizations/{orgID}/members/{userID}", orgMgr.handleRemoveMember())

			req := httptest.NewRequest(http.MethodDelete, "/organizations/org-1/members/"+tt.userID, nil)
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			req = addUserToContext(req, &auth.User{ID: "admin-user"})
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Test Organization", "test-organization"},
		{"My Cool Lab", "my-cool-lab"},
		{"UPPERCASE", "uppercase"},
		{"with  spaces", "with--spaces"},
		{"Special!@#$Characters", "specialcharacters"},
		{"123 Numbers", "123-numbers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateSlug(tt.name)
			if got != tt.want {
				t.Errorf("generateSlug(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

// -----------------------------------------------------------------------------
// Additional Error Path Tests
// -----------------------------------------------------------------------------

func TestHandleCreateOrganization_ErrorPaths(t *testing.T) {
	t.Run("no user context", func(t *testing.T) {
		orgRepo := &mockOrgRepoHandler{orgs: make(map[string]*models.Organization)}
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, orgRepo, membershipRepo)

		body, _ := json.Marshal(CreateOrganizationRequest{Name: "Test Org"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
		// No user context added
		w := httptest.NewRecorder()

		orgMgr.handleCreateOrganization()(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		orgRepo := &mockOrgRepoHandler{orgs: make(map[string]*models.Organization)}
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, orgRepo, membershipRepo)
		user := &auth.User{ID: "user-1", Email: "test@example.com"}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader([]byte("invalid json")))
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateOrganization()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("slug conflict", func(t *testing.T) {
		orgRepo := &mockOrgRepoHandler{
			orgs: map[string]*models.Organization{
				"existing-org": {
					ID:   "existing-org",
					Name: "Existing Org",
					Slug: "test-org",
				},
			},
		}
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, orgRepo, membershipRepo)
		user := &auth.User{ID: "user-1", Email: "test@example.com"}

		body, _ := json.Marshal(CreateOrganizationRequest{
			Name: "Test Org", // Will generate slug "test-org" which conflicts
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateOrganization()(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
		}
	})
}

func TestHandleGetOrganization_ErrorPaths(t *testing.T) {
	t.Run("no organization in tenant context", func(t *testing.T) {
		orgRepo := &mockOrgRepoHandler{orgs: make(map[string]*models.Organization)}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		// Tenant context with nil organization
		tenantCtx := &models.TenantContext{
			Organization: nil,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1", nil)
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleGetOrganization()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("includes limits for admin", func(t *testing.T) {
		now := time.Now()
		maxUsers := 100
		maxPods := 10
		maxStorage := 50
		org := &models.Organization{
			ID:                "org-1",
			Name:              "Test Org",
			Slug:              "test-org",
			Type:              models.OrgTypeStandard,
			Edition:           models.EditionProfessional,
			IsActive:          true,
			MaxUsers:          &maxUsers,
			MaxConcurrentPods: &maxPods,
			MaxStorageGB:      &maxStorage,
		}

		orgRepo := &mockOrgRepoHandler{
			orgs:        map[string]*models.Organization{"org-1": org},
			memberCount: 5,
			teamCount:   2,
		}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1", nil)
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleGetOrganization()(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// Admin should see limits
		limits, ok := resp["limits"].(map[string]interface{})
		if !ok {
			t.Fatal("expected limits object in response for admin")
		}
		if limits["maxUsers"].(float64) != 100 {
			t.Errorf("expected maxUsers 100, got %v", limits["maxUsers"])
		}
	})
}

func TestHandleUpdateOrganization_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{ID: "org-1", Name: "Test", Slug: "test"}

	t.Run("invalid JSON body", func(t *testing.T) {
		orgRepo := &mockOrgRepoHandler{orgs: map[string]*models.Organization{"org-1": org}}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/org-1", bytes.NewReader([]byte("invalid json")))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleUpdateOrganization()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("owner can update any field", func(t *testing.T) {
		orgRepo := &mockOrgRepoHandler{orgs: map[string]*models.Organization{"org-1": org}}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleOwner,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(UpdateOrganizationRequest{Name: stringPtr("Owner Updated")})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/org-1", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleUpdateOrganization()(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("slug conflict - returns 409", func(t *testing.T) {
		org1 := &models.Organization{ID: "org-1", Name: "Org 1", Slug: "org-one"}
		org2 := &models.Organization{ID: "org-2", Name: "Org 2", Slug: "org-two"}
		orgRepo := &mockOrgRepoHandler{orgs: map[string]*models.Organization{"org-1": org1, "org-2": org2}}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: org1,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(UpdateOrganizationRequest{Slug: stringPtr("org-two")})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/org-1", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleUpdateOrganization()(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
		}
	})

	t.Run("non-owner cannot deactivate - returns 403", func(t *testing.T) {
		orgCopy := *org
		orgRepo := &mockOrgRepoHandler{orgs: map[string]*models.Organization{"org-1": &orgCopy}}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: &orgCopy,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin, // Not owner
				AcceptedAt: &now,
			},
		}

		isActive := false
		body, _ := json.Marshal(UpdateOrganizationRequest{IsActive: &isActive})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/org-1", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleUpdateOrganization()(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
		}
	})

	t.Run("database error on update - returns 500", func(t *testing.T) {
		orgCopy := *org
		orgRepo := &mockOrgRepoHandler{
			orgs:      map[string]*models.Organization{"org-1": &orgCopy},
			updateErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: &orgCopy,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(UpdateOrganizationRequest{Name: stringPtr("Updated Name")})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/org-1", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		orgMgr.handleUpdateOrganization()(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusInternalServerError, w.Body.String())
		}
	})
}

func TestHandleDeleteOrganization_ErrorPaths(t *testing.T) {
	t.Run("no tenant context", func(t *testing.T) {
		org := &models.Organization{ID: "org-1", Name: "Test", Slug: "test"}
		orgRepo := &mockOrgRepoHandler{orgs: map[string]*models.Organization{"org-1": org}}
		orgMgr := createTestServerWithOrgs(t, orgRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/org-1", nil)
		// No tenant context
		w := httptest.NewRecorder()

		orgMgr.handleDeleteOrganization()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestHandleListMembers_ErrorPaths(t *testing.T) {
	t.Run("no tenant context", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/members", nil)
		// No tenant context
		w := httptest.NewRecorder()

		orgMgr.handleListMembers()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestHandleInviteMember_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{ID: "org-1", Name: "Test"}

	t.Run("no tenant context", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)
		user := &auth.User{ID: "inviter-1"}

		body, _ := json.Marshal(InviteMemberRequest{UserID: "user-2", Role: models.OrgRoleMember})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/members", bytes.NewReader(body))
		// No tenant context
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleInviteMember()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)
		user := &auth.User{ID: "inviter-1"}

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/members", bytes.NewReader([]byte("invalid")))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleInviteMember()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing userID", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)
		user := &auth.User{ID: "inviter-1"}

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(InviteMemberRequest{Role: models.OrgRoleMember}) // Missing userID
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleInviteMember()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("owner can invite any role", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{memberships: make(map[string]*models.OrganizationMembership)}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)
		user := &auth.User{ID: "inviter-1"}

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleOwner,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(InviteMemberRequest{UserID: "user-2", Role: models.OrgRoleAdmin})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleInviteMember()(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
		}
	})

	t.Run("user already member", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{
			memberships: map[string]*models.OrganizationMembership{
				"existing-mem": {
					ID:             "existing-mem",
					OrganizationID: "org-1",
					UserID:         "user-2", // Already a member
					Role:           models.OrgRoleMember,
					AcceptedAt:     &now,
				},
			},
		}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)
		user := &auth.User{ID: "inviter-1"}

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(InviteMemberRequest{UserID: "user-2", Role: models.OrgRoleMember})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleInviteMember()(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
		}
	})
}

func TestHandleUpdateMember_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{ID: "org-1", Name: "Test"}

	t.Run("invalid JSON body", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{
			memberships: map[string]*models.OrganizationMembership{
				"m-1": {
					ID:             "m-1",
					OrganizationID: "org-1",
					UserID:         "user-2",
					Role:           models.OrgRoleMember,
					AcceptedAt:     &now,
				},
			},
		}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		r := chi.NewRouter()
		r.Patch("/organizations/{orgID}/members/{userID}", orgMgr.handleUpdateMember())

		req := httptest.NewRequest(http.MethodPatch, "/organizations/org-1/members/user-2", bytes.NewReader([]byte("invalid")))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("cannot change owner role", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{
			memberships: map[string]*models.OrganizationMembership{
				"m-1": {
					ID:             "m-1",
					OrganizationID: "org-1",
					UserID:         "user-owner",
					Role:           models.OrgRoleOwner,
					AcceptedAt:     &now,
				},
			},
		}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleOwner,
				AcceptedAt: &now,
			},
		}

		r := chi.NewRouter()
		r.Patch("/organizations/{orgID}/members/{userID}", orgMgr.handleUpdateMember())

		body, _ := json.Marshal(UpdateMemberRequest{Role: models.OrgRoleAdmin})
		req := httptest.NewRequest(http.MethodPatch, "/organizations/org-1/members/user-owner", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// API returns 400 Bad Request with "cannot demote last owner" message
		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
}

func TestHandleRemoveMember_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{ID: "org-1", Name: "Test"}

	t.Run("admin cannot remove another admin", func(t *testing.T) {
		membershipRepo := &mockMembershipRepoHandler{
			memberships: map[string]*models.OrganizationMembership{
				"m-1": {
					ID:             "m-1",
					OrganizationID: "org-1",
					UserID:         "user-owner",
					Role:           models.OrgRoleOwner,
					AcceptedAt:     &now,
				},
				"m-2": {
					ID:             "m-2",
					OrganizationID: "org-1",
					UserID:         "other-admin",
					Role:           models.OrgRoleAdmin,
					AcceptedAt:     &now,
				},
			},
		}
		orgMgr := createTestServerWithOrgs(t, nil, membershipRepo)
		user := &auth.User{ID: "current-admin"}

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		}

		r := chi.NewRouter()
		r.Delete("/organizations/{orgID}/members/{userID}", orgMgr.handleRemoveMember())

		req := httptest.NewRequest(http.MethodDelete, "/organizations/org-1/members/other-admin", nil)
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
		}
	})
}
