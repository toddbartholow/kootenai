package organizations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Mock team repository for testing
type mockTeamRepoHandler struct {
	teams        map[string]*models.Team
	memberCount  int
	createErr    error
	updateErr    error
	deleteErr    error
	getByIDErr   error
	getBySlugErr error
	listErr      error
}

func (m *mockTeamRepoHandler) Create(ctx context.Context, team *models.Team) error {
	if m.createErr != nil {
		return m.createErr
	}
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()
	m.teams[team.ID] = team
	return nil
}

func (m *mockTeamRepoHandler) GetByID(ctx context.Context, id string) (*models.Team, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	return m.teams[id], nil
}

func (m *mockTeamRepoHandler) GetBySlug(ctx context.Context, orgID, slug string) (*models.Team, error) {
	if m.getBySlugErr != nil {
		return nil, m.getBySlugErr
	}
	for _, team := range m.teams {
		if team.OrganizationID == orgID && team.Slug == slug {
			return team, nil
		}
	}
	return nil, nil
}

func (m *mockTeamRepoHandler) ListByOrganization(ctx context.Context, orgID string, filter repositories.TeamFilter) ([]*models.Team, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*models.Team
	for _, team := range m.teams {
		if team.OrganizationID == orgID {
			if filter.Active == nil || *filter.Active == team.IsActive {
				result = append(result, team)
			}
		}
	}
	return result, nil
}

func (m *mockTeamRepoHandler) Update(ctx context.Context, team *models.Team) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.teams[team.ID] = team
	return nil
}

func (m *mockTeamRepoHandler) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.teams, id)
	return nil
}

func (m *mockTeamRepoHandler) SetActive(ctx context.Context, id string, active bool) error {
	if team, ok := m.teams[id]; ok {
		team.IsActive = active
	}
	return nil
}

func (m *mockTeamRepoHandler) GetMemberCount(ctx context.Context, id string) (int, error) {
	return m.memberCount, nil
}

// Mock team membership repository for testing
type mockTeamMembershipRepoHandler struct {
	memberships          map[string]*models.TeamMembership
	createErr            error
	listByTeamErr        error
	getByTeamAndUserErr  error
	listByTeamAndRoleErr error
	updateRoleErr        error
}

func (m *mockTeamMembershipRepoHandler) Create(ctx context.Context, membership *models.TeamMembership) error {
	if m.createErr != nil {
		return m.createErr
	}
	membership.CreatedAt = time.Now()
	m.memberships[membership.ID] = membership
	return nil
}

func (m *mockTeamMembershipRepoHandler) GetByID(ctx context.Context, id string) (*models.TeamMembership, error) {
	return m.memberships[id], nil
}

func (m *mockTeamMembershipRepoHandler) GetByTeamAndUser(ctx context.Context, teamID, userID string) (*models.TeamMembership, error) {
	if m.getByTeamAndUserErr != nil {
		return nil, m.getByTeamAndUserErr
	}
	for _, mem := range m.memberships {
		if mem.TeamID == teamID && mem.UserID == userID {
			return mem, nil
		}
	}
	return nil, nil
}

func (m *mockTeamMembershipRepoHandler) ListByTeam(ctx context.Context, teamID string) ([]*models.TeamMembership, error) {
	if m.listByTeamErr != nil {
		return nil, m.listByTeamErr
	}
	var result []*models.TeamMembership
	for _, mem := range m.memberships {
		if mem.TeamID == teamID {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *mockTeamMembershipRepoHandler) ListByTeamAndRole(ctx context.Context, teamID string, role models.TeamRole) ([]*models.TeamMembership, error) {
	if m.listByTeamAndRoleErr != nil {
		return nil, m.listByTeamAndRoleErr
	}
	var result []*models.TeamMembership
	for _, mem := range m.memberships {
		if mem.TeamID == teamID && mem.Role == role {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *mockTeamMembershipRepoHandler) ListByUser(ctx context.Context, userID string) ([]*models.TeamMembership, error) {
	var result []*models.TeamMembership
	for _, mem := range m.memberships {
		if mem.UserID == userID {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *mockTeamMembershipRepoHandler) Update(ctx context.Context, membership *models.TeamMembership) error {
	m.memberships[membership.ID] = membership
	return nil
}

func (m *mockTeamMembershipRepoHandler) UpdateRole(ctx context.Context, id string, role models.TeamRole) error {
	if m.updateRoleErr != nil {
		return m.updateRoleErr
	}
	if mem, ok := m.memberships[id]; ok {
		mem.Role = role
	}
	return nil
}

func (m *mockTeamMembershipRepoHandler) Delete(ctx context.Context, id string) error {
	delete(m.memberships, id)
	return nil
}

// Test helpers

func createTestServerWithTeams(t *testing.T, teamRepo *mockTeamRepoHandler, teamMembershipRepo *mockTeamMembershipRepoHandler) *Manager {
	t.Helper()

	cfg := Config{
		Logger: newTestLogger(),
	}
	if teamRepo != nil {
		cfg.TeamRepo = teamRepo
	}
	if teamMembershipRepo != nil {
		cfg.TeamMembershipRepo = teamMembershipRepo
	}

	return NewManager(cfg)
}

// Tests

func TestHandleListTeams(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		Edition:  models.EditionProfessional, // Teams enabled
		IsActive: true,
	}

	teamRepo := &mockTeamRepoHandler{
		teams: map[string]*models.Team{
			"team-1": {
				ID:             "team-1",
				OrganizationID: "org-1",
				Name:           "Team Alpha",
				Slug:           "team-alpha",
				IsActive:       true,
			},
			"team-2": {
				ID:             "team-2",
				OrganizationID: "org-1",
				Name:           "Team Beta",
				Slug:           "team-beta",
				IsActive:       true,
			},
		},
		memberCount: 3,
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, nil)

	tests := []struct {
		name       string
		tenantCtx  *models.TenantContext
		wantStatus int
		wantCount  int
	}{
		{
			name: "success",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Edition:      models.EditionProfessional,
				Features:     map[string]bool{"teams": true},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "no tenant context",
			tenantCtx:  nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/teams", nil)
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			w := httptest.NewRecorder()

			orgMgr.handleListTeams()(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				teams, ok := resp["teams"].([]interface{})
				if !ok {
					t.Fatal("expected teams array in response")
				}
				if len(teams) != tt.wantCount {
					t.Errorf("expected %d teams, got %d", tt.wantCount, len(teams))
				}
			}
		})
	}
}

func TestHandleCreateTeam(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		Edition:  models.EditionProfessional,
		IsActive: true,
	}

	teamRepo := &mockTeamRepoHandler{
		teams: make(map[string]*models.Team),
	}
	teamMembershipRepo := &mockTeamMembershipRepoHandler{
		memberships: make(map[string]*models.TeamMembership),
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)
	user := &auth.User{ID: "user-1", Email: "test@example.com"}

	tests := []struct {
		name       string
		tenantCtx  *models.TenantContext
		body       CreateTeamRequest
		wantStatus int
	}{
		{
			name: "success - instructor creates team",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Edition:      models.EditionProfessional,
				Features:     map[string]bool{"teams": true},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleInstructor,
					AcceptedAt: &now,
				},
			},
			body: CreateTeamRequest{
				Name:        "New Team",
				Description: "A new team",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "forbidden - member cannot create team",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Edition:      models.EditionProfessional,
				Features:     map[string]bool{"teams": true},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			body: CreateTeamRequest{
				Name: "New Team",
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "missing name",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Edition:      models.EditionProfessional,
				Features:     map[string]bool{"teams": true},
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleInstructor,
					AcceptedAt: &now,
				},
			},
			body:       CreateTeamRequest{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset teams for each test
			teamRepo.teams = make(map[string]*models.Team)

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			req = addUserToContext(req, user)
			w := httptest.NewRecorder()

			orgMgr.handleCreateTeam()(w, req)

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

	t.Run("no tenant context", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		body, _ := json.Marshal(CreateTeamRequest{Name: "New Team"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
		// No tenant context
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Edition:      models.EditionProfessional,
			Features:     map[string]bool{"teams": true},
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader([]byte("invalid json")))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("slug conflict", func(t *testing.T) {
		teamRepoWithTeam := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"existing-team": {
				ID:             "existing-team",
				OrganizationID: "org-1",
				Name:           "Existing Team",
				Slug:           "existing-slug",
			}},
		}
		orgMgr := createTestServerWithTeams(t, teamRepoWithTeam, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Edition:      models.EditionProfessional,
			Features:     map[string]bool{"teams": true},
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(CreateTeamRequest{Name: "New Team", Slug: "existing-slug"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
		}
	})

	t.Run("database error on create", func(t *testing.T) {
		teamRepoWithErr := &mockTeamRepoHandler{
			teams:     make(map[string]*models.Team),
			createErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepoWithErr, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Edition:      models.EditionProfessional,
			Features:     map[string]bool{"teams": true},
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(CreateTeamRequest{Name: "New Team"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusInternalServerError, w.Body.String())
		}
	})

	t.Run("community edition - teams not enabled", func(t *testing.T) {
		communityOrg := &models.Organization{
			ID:       "org-1",
			Name:     "Test Org",
			Slug:     "test-org",
			Edition:  models.EditionCommunity,
			IsActive: true,
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: communityOrg,
			Edition:      models.EditionCommunity,
			Features:     map[string]bool{}, // No teams feature
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(CreateTeamRequest{Name: "New Team"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusPaymentRequired {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusPaymentRequired, w.Body.String())
		}
	})
}

func TestHandleGetTeam(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		Description:    "Test team",
		IsActive:       true,
	}

	teamRepo := &mockTeamRepoHandler{
		teams:       map[string]*models.Team{"team-1": team},
		memberCount: 5,
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, nil)

	tests := []struct {
		name       string
		teamID     string
		tenantCtx  *models.TenantContext
		wantStatus int
	}{
		{
			name:   "success",
			teamID: "team-1",
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
			name:   "team not found",
			teamID: "team-nonexistent",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "no tenant context",
			teamID:     "team-1",
			tenantCtx:  nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/teams/{teamID}", orgMgr.handleGetTeam())

			req := httptest.NewRequest(http.MethodGet, "/teams/"+tt.teamID, nil)
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if resp["name"] != team.Name {
					t.Errorf("expected name %s, got %s", team.Name, resp["name"])
				}
			}
		})
	}
}

func TestHandleDeleteTeam(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	tests := []struct {
		name       string
		team       *models.Team
		tenantCtx  *models.TenantContext
		wantStatus int
	}{
		{
			name: "success - admin deletes team",
			team: &models.Team{
				ID:             "team-1",
				OrganizationID: "org-1",
				Name:           "Team Alpha",
				Slug:           "team-alpha",
				IsActive:       true,
			},
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
			name: "forbidden - member cannot delete team",
			team: &models.Team{
				ID:             "team-2",
				OrganizationID: "org-1",
				Name:           "Team Beta",
				Slug:           "team-beta",
				IsActive:       true,
			},
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teamRepo := &mockTeamRepoHandler{
				teams: map[string]*models.Team{tt.team.ID: tt.team},
			}
			orgMgr := createTestServerWithTeams(t, teamRepo, nil)

			r := chi.NewRouter()
			r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

			req := httptest.NewRequest(http.MethodDelete, "/teams/"+tt.team.ID, nil)
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

	t.Run("no tenant context", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": {
				ID:             "team-1",
				OrganizationID: "org-1",
				Name:           "Test Team",
			}},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1", nil)
		// No tenant context added
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("team not found", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{}, // No teams
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

		req := httptest.NewRequest(http.MethodDelete, "/teams/nonexistent", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
		}
	})

	t.Run("team from different org", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": {
				ID:             "team-1",
				OrganizationID: "other-org",
				Name:           "Other Org Team",
			}},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
		}
	})

	t.Run("database error on delete", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": {
				ID:             "team-1",
				OrganizationID: "org-1",
				Name:           "Test Team",
			}},
			deleteErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusInternalServerError, w.Body.String())
		}
	})
}

func TestHandleListTeamMembers(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	t.Run("success", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}

		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				},
				"tm-2": {
					ID:     "tm-2",
					TeamID: "team-1",
					UserID: "user-2",
					Role:   models.TeamRoleMember,
				},
			},
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		}

		r := chi.NewRouter()
		r.Get("/teams/{teamID}/members", orgMgr.handleListTeamMembers())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1/members", nil)
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
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
	})

	t.Run("no tenant context", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}/members", orgMgr.handleListTeamMembers())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1/members", nil)
		// No tenant context
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("team not found", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: make(map[string]*models.Team), // No teams
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		}

		r := chi.NewRouter()
		r.Get("/teams/{teamID}/members", orgMgr.handleListTeamMembers())

		req := httptest.NewRequest(http.MethodGet, "/teams/nonexistent/members", nil)
		req = addTenantToContext(req, tenantCtx)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
		}
	})
}

func TestHandleAddTeamMember(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	t.Run("success - team lead adds member", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}

		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				},
			},
		}

		// Also need org membership repo to verify user is org member
		orgMembershipRepo := &mockMembershipRepoHandler{
			memberships: map[string]*models.OrganizationMembership{
				"om-1": {
					ID:             "om-1",
					OrganizationID: "org-1",
					UserID:         "user-2",
					Role:           models.OrgRoleMember,
					AcceptedAt:     &now,
				},
			},
		}

		orgMgr := NewManager(Config{
			TeamRepo:           teamRepo,
			TeamMembershipRepo: teamMembershipRepo,
			OrgMembershipRepo:  orgMembershipRepo,
			Logger:             newTestLogger(),
		})

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{
			UserID: "user-2",
			Role:   models.TeamRoleMember,
		})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember, // Not org-level instructor, but team lead
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
		}
	})

	t.Run("no tenant context", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		// No tenant context
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("team not found", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: make(map[string]*models.Team), // No teams
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/nonexistent/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
		}
	})

	t.Run("forbidden - non-lead non-instructor", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership), // user-1 is not a team lead
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember, // Not instructor
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-3"}) // Not a team lead
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {ID: "tm-1", TeamID: "team-1", UserID: "user-1", Role: models.TeamRoleLead},
			},
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader([]byte(`{invalid`)))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("missing userId", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {ID: "tm-1", TeamID: "team-1", UserID: "user-1", Role: models.TeamRoleLead},
			},
		}

		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: ""}) // Empty userId
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
}

func TestHandleRemoveTeamMember(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	tests := []struct {
		name        string
		userID      string
		tenantCtx   *models.TenantContext
		currentUser string
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
			currentUser: "user-1",
			wantStatus:  http.StatusOK,
		},
		{
			name:   "success - user removes self",
			userID: "user-3",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			currentUser: "user-3",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "no tenant context",
			userID:      "user-3",
			tenantCtx:   nil,
			currentUser: "user-1",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:   "forbidden - non-admin non-lead cannot remove other member",
			userID: "user-3",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			currentUser: "user-4", // Not admin, not team lead, not self
			wantStatus:  http.StatusForbidden,
		},
		{
			name:   "success - team lead removes member",
			userID: "user-3",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember, // Not org admin
					AcceptedAt: &now,
				},
			},
			currentUser: "user-1", // Is team lead
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh repos for each test
			teamRepo := &mockTeamRepoHandler{
				teams: map[string]*models.Team{"team-1": team},
			}

			teamMembershipRepo := &mockTeamMembershipRepoHandler{
				memberships: map[string]*models.TeamMembership{
					"tm-1": {
						ID:     "tm-1",
						TeamID: "team-1",
						UserID: "user-1",
						Role:   models.TeamRoleLead,
					},
					"tm-2": {
						ID:     "tm-2",
						TeamID: "team-1",
						UserID: "user-2",
						Role:   models.TeamRoleLead, // Second lead
					},
					"tm-3": {
						ID:     "tm-3",
						TeamID: "team-1",
						UserID: "user-3",
						Role:   models.TeamRoleMember,
					},
				},
			}

			orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

			r := chi.NewRouter()
			r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

			req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/"+tt.userID, nil)
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			req = addUserToContext(req, &auth.User{ID: tt.currentUser})
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleRemoveTeamMember_TeamNotFound(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	// Team repo with no teams
	teamRepo := &mockTeamRepoHandler{
		teams: make(map[string]*models.Team),
	}

	teamMembershipRepo := &mockTeamMembershipRepoHandler{
		memberships: make(map[string]*models.TeamMembership),
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

	r := chi.NewRouter()
	r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

	tenantCtx := &models.TenantContext{
		Organization: org,
		Membership: &models.OrganizationMembership{
			Role:       models.OrgRoleAdmin,
			AcceptedAt: &now,
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/teams/nonexistent/members/user-1", nil)
	req = addTenantToContext(req, tenantCtx)
	req = addUserToContext(req, &auth.User{ID: "admin-user"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleRemoveTeamMember_MemberNotFound(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	teamRepo := &mockTeamRepoHandler{
		teams: map[string]*models.Team{"team-1": team},
	}

	// No membership for the user we're trying to remove
	teamMembershipRepo := &mockTeamMembershipRepoHandler{
		memberships: make(map[string]*models.TeamMembership),
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

	r := chi.NewRouter()
	r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

	tenantCtx := &models.TenantContext{
		Organization: org,
		Membership: &models.OrganizationMembership{
			Role:       models.OrgRoleAdmin,
			AcceptedAt: &now,
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/nonexistent-user", nil)
	req = addTenantToContext(req, tenantCtx)
	req = addUserToContext(req, &auth.User{ID: "admin-user"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleRemoveTeamMember_CannotRemoveLastLead(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	teamRepo := &mockTeamRepoHandler{
		teams: map[string]*models.Team{"team-1": team},
	}

	// Only one lead in the team
	teamMembershipRepo := &mockTeamMembershipRepoHandler{
		memberships: map[string]*models.TeamMembership{
			"tm-1": {
				ID:     "tm-1",
				TeamID: "team-1",
				UserID: "user-1",
				Role:   models.TeamRoleLead, // Only lead
			},
		},
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

	r := chi.NewRouter()
	r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

	tenantCtx := &models.TenantContext{
		Organization: org,
		Membership: &models.OrganizationMembership{
			Role:       models.OrgRoleAdmin,
			AcceptedAt: &now,
		},
	}

	// Try to remove the only lead
	req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/user-1", nil)
	req = addTenantToContext(req, tenantCtx)
	req = addUserToContext(req, &auth.User{ID: "admin-user"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandleUpdateTeam(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		Description:    "Original description",
		IsActive:       true,
	}

	tests := []struct {
		name       string
		tenantCtx  *models.TenantContext
		body       UpdateTeamRequest
		isTeamLead bool
		wantStatus int
	}{
		{
			name: "success - admin updates team",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			body: UpdateTeamRequest{
				Name:        stringPtr("Updated Team Name"),
				Description: stringPtr("Updated description"),
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "success - team lead updates team",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember, // Not org admin
					AcceptedAt: &now,
				},
			},
			body: UpdateTeamRequest{
				Name: stringPtr("Lead Updated Name"),
			},
			isTeamLead: true,
			wantStatus: http.StatusOK,
		},
		{
			name: "forbidden - member cannot update team",
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			body: UpdateTeamRequest{
				Name: stringPtr("Should Not Update"),
			},
			isTeamLead: false,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no tenant context",
			tenantCtx:  nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh team copy for each test
			teamCopy := *team
			teamRepo := &mockTeamRepoHandler{
				teams: map[string]*models.Team{team.ID: &teamCopy},
			}

			// Create membership repo with optional team lead membership
			memberships := make(map[string]*models.TeamMembership)
			if tt.isTeamLead {
				memberships["tm-lead"] = &models.TeamMembership{
					ID:     "tm-lead",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				}
			}
			teamMembershipRepo := &mockTeamMembershipRepoHandler{
				memberships: memberships,
			}

			orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

			r := chi.NewRouter()
			r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			req = addUserToContext(req, &auth.User{ID: "user-1"})
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK && tt.body.Name != nil {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if resp["name"] != *tt.body.Name {
					t.Errorf("expected name %s, got %s", *tt.body.Name, resp["name"])
				}
			}
		})
	}

	t.Run("team not found", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{}, // No teams
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		body, _ := json.Marshal(UpdateTeamRequest{Name: stringPtr("New Name")})
		req := httptest.NewRequest(http.MethodPut, "/teams/nonexistent", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader([]byte("invalid json")))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("database error on update", func(t *testing.T) {
		teamCopy := *team
		teamRepo := &mockTeamRepoHandler{
			teams:     map[string]*models.Team{"team-1": &teamCopy},
			updateErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		body, _ := json.Marshal(UpdateTeamRequest{Name: stringPtr("New Name")})
		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d: %s", w.Code, http.StatusInternalServerError, w.Body.String())
		}
	})
}

func TestHandleUpdateTeamMember(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	tests := []struct {
		name        string
		targetUser  string
		newRole     models.TeamRole
		tenantCtx   *models.TenantContext
		currentUser string
		isTeamLead  bool
		wantStatus  int
	}{
		{
			name:       "success - admin updates member role",
			targetUser: "user-3",
			newRole:    models.TeamRoleLead,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			currentUser: "user-1",
			wantStatus:  http.StatusOK,
		},
		{
			name:       "success - team lead updates member role",
			targetUser: "user-3",
			newRole:    models.TeamRoleLead,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			currentUser: "user-1",
			isTeamLead:  true,
			wantStatus:  http.StatusOK,
		},
		{
			name:       "forbidden - regular member cannot update roles",
			targetUser: "user-3",
			newRole:    models.TeamRoleLead,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleMember,
					AcceptedAt: &now,
				},
			},
			currentUser: "user-4",
			isTeamLead:  false,
			wantStatus:  http.StatusForbidden,
		},
		{
			name:       "not found - member not in team",
			targetUser: "user-nonexistent",
			newRole:    models.TeamRoleLead,
			tenantCtx: &models.TenantContext{
				Organization: org,
				Membership: &models.OrganizationMembership{
					Role:       models.OrgRoleAdmin,
					AcceptedAt: &now,
				},
			},
			currentUser: "user-1",
			wantStatus:  http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teamRepo := &mockTeamRepoHandler{
				teams: map[string]*models.Team{team.ID: team},
			}

			// Set up memberships with multiple leads to avoid "cannot demote last lead" issue
			memberships := map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				},
				"tm-2": {
					ID:     "tm-2",
					TeamID: "team-1",
					UserID: "user-2",
					Role:   models.TeamRoleLead, // Second lead
				},
				"tm-3": {
					ID:     "tm-3",
					TeamID: "team-1",
					UserID: "user-3",
					Role:   models.TeamRoleMember,
				},
			}
			teamMembershipRepo := &mockTeamMembershipRepoHandler{
				memberships: memberships,
			}

			orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

			r := chi.NewRouter()
			r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

			body, _ := json.Marshal(UpdateTeamMemberRequest{Role: tt.newRole})
			req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/"+tt.targetUser, bytes.NewReader(body))
			if tt.tenantCtx != nil {
				req = addTenantToContext(req, tt.tenantCtx)
			}
			req = addUserToContext(req, &auth.User{ID: tt.currentUser})
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleUpdateTeamMember_CannotDemoteLastLead(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	teamRepo := &mockTeamRepoHandler{
		teams: map[string]*models.Team{team.ID: team},
	}

	// Only one lead
	teamMembershipRepo := &mockTeamMembershipRepoHandler{
		memberships: map[string]*models.TeamMembership{
			"tm-1": {
				ID:     "tm-1",
				TeamID: "team-1",
				UserID: "user-1",
				Role:   models.TeamRoleLead, // Only lead
			},
			"tm-2": {
				ID:     "tm-2",
				TeamID: "team-1",
				UserID: "user-2",
				Role:   models.TeamRoleMember,
			},
		},
	}

	orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

	tenantCtx := &models.TenantContext{
		Organization: org,
		Membership: &models.OrganizationMembership{
			Role:       models.OrgRoleAdmin,
			AcceptedAt: &now,
		},
	}

	r := chi.NewRouter()
	r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

	// Try to demote the only lead
	body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleMember})
	req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
	req = addTenantToContext(req, tenantCtx)
	req = addUserToContext(req, &auth.User{ID: "user-admin"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Additional error path tests for coverage

func TestHandleGetTeam_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	t.Run("database error on GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}", orgMgr.handleGetTeam())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}", orgMgr.handleGetTeam())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("team belongs to different org", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{
				"team-1": {
					ID:             "team-1",
					OrganizationID: "other-org",
					Name:           "Other Org Team",
				},
			},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}", orgMgr.handleGetTeam())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})

	t.Run("with user membership", func(t *testing.T) {
		team := &models.Team{
			ID:             "team-1",
			OrganizationID: "org-1",
			Name:           "Team Alpha",
			Slug:           "team-alpha",
			IsActive:       true,
		}
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				},
			},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}", orgMgr.handleGetTeam())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if _, ok := resp["membership"]; !ok {
			t.Error("expected membership in response")
		}
	})
}

func TestHandleListTeams_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/teams", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		orgMgr.handleListTeams()(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on list", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:   map[string]*models.Team{},
			listErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/teams", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		orgMgr.handleListTeams()(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("with active_only filter", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{
				"team-1": {
					ID:             "team-1",
					OrganizationID: "org-1",
					Name:           "Active Team",
					IsActive:       true,
				},
				"team-2": {
					ID:             "team-2",
					OrganizationID: "org-1",
					Name:           "Inactive Team",
					IsActive:       false,
				},
			},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/teams?active_only=true", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		orgMgr.handleListTeams()(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		teams, ok := resp["teams"].([]any)
		if !ok {
			t.Fatal("expected teams array in response")
		}
		if len(teams) != 1 {
			t.Errorf("expected 1 active team, got %d", len(teams))
		}
	})
}

func TestHandleListTeamMembers_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		IsActive:       true,
	}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}/members", orgMgr.handleListTeamMembers())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1/members", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on team GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}/members", orgMgr.handleListTeamMembers())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1/members", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("database error on ListByTeam", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships:   make(map[string]*models.TeamMembership),
			listByTeamErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Get("/teams/{teamID}/members", orgMgr.handleListTeamMembers())

		req := httptest.NewRequest(http.MethodGet, "/teams/team-1/members", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleMember,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})
}

func TestHandleUpdateTeamMember_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		IsActive:       true,
	}

	t.Run("no tenant context", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleLead})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		// No tenant context
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
	})

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleLead})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on team GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleLead})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("team belongs to different org", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{
				"team-1": {
					ID:             "team-1",
					OrganizationID: "other-org",
					Name:           "Other Org Team",
				},
			},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleLead})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader([]byte("{invalid")))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
		}
	})

	t.Run("error getting membership", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships:         make(map[string]*models.TeamMembership),
			getByTeamAndUserErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleLead})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("error listing leads when demoting", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				},
			},
			listByTeamAndRoleErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleMember})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("error updating role", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleMember,
				},
			},
			updateRoleErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Patch("/teams/{teamID}/members/{userID}", orgMgr.handleUpdateTeamMember())

		body, _ := json.Marshal(UpdateTeamMemberRequest{Role: models.TeamRoleLead})
		req := httptest.NewRequest(http.MethodPatch, "/teams/team-1/members/user-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})
}

func TestHandleCreateTeam_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		Edition:  models.EditionProfessional,
		IsActive: true,
	}
	user := &auth.User{ID: "user-1", Email: "test@example.com"}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Edition:      models.EditionProfessional,
			Features:     map[string]bool{"teams": true},
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(CreateTeamRequest{Name: "New Team"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("error checking slug", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:        map[string]*models.Team{},
			getBySlugErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		tenantCtx := &models.TenantContext{
			Organization: org,
			Edition:      models.EditionProfessional,
			Features:     map[string]bool{"teams": true},
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		}

		body, _ := json.Marshal(CreateTeamRequest{Name: "New Team"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-1/teams", bytes.NewReader(body))
		req = addTenantToContext(req, tenantCtx)
		req = addUserToContext(req, user)
		w := httptest.NewRecorder()

		orgMgr.handleCreateTeam()(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})
}

func TestHandleDeleteTeam_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}", orgMgr.handleDeleteTeam())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})
}

func TestHandleUpdateTeam_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		Slug:           "team-alpha",
		IsActive:       true,
	}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		body, _ := json.Marshal(UpdateTeamRequest{Name: stringPtr("New Name")})
		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		body, _ := json.Marshal(UpdateTeamRequest{Name: stringPtr("New Name")})
		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("slug conflict on update", func(t *testing.T) {
		existingTeam := &models.Team{
			ID:             "team-2",
			OrganizationID: "org-1",
			Name:           "Existing Team",
			Slug:           "existing-slug",
			IsActive:       true,
		}
		teamCopy := *team
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{
				"team-1": &teamCopy,
				"team-2": existingTeam,
			},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		body, _ := json.Marshal(UpdateTeamRequest{Slug: stringPtr("existing-slug")})
		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
		}
	})

	t.Run("error checking slug on update", func(t *testing.T) {
		teamCopy := *team
		teamRepo := &mockTeamRepoHandler{
			teams:        map[string]*models.Team{"team-1": &teamCopy},
			getBySlugErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		body, _ := json.Marshal(UpdateTeamRequest{Slug: stringPtr("new-slug")})
		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("update isActive field", func(t *testing.T) {
		teamCopy := *team
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": &teamCopy},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, nil)

		r := chi.NewRouter()
		r.Put("/teams/{teamID}", orgMgr.handleUpdateTeam())

		inactive := false
		body, _ := json.Marshal(UpdateTeamRequest{IsActive: &inactive})
		req := httptest.NewRequest(http.MethodPut, "/teams/team-1", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["isActive"] != false {
			t.Errorf("expected isActive false, got %v", resp["isActive"])
		}
	})
}

func TestHandleAddTeamMember_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		IsActive:       true,
	}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on team GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("already a team member", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-2",
					Role:   models.TeamRoleMember,
				},
			},
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
		}
	})

	t.Run("error creating membership", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
			createErr:   errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"})
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("default role when not specified", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Post("/teams/{teamID}/members", orgMgr.handleAddTeamMember())

		body, _ := json.Marshal(AddTeamMemberRequest{UserID: "user-2"}) // No role specified
		req := httptest.NewRequest(http.MethodPost, "/teams/team-1/members", bytes.NewReader(body))
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleInstructor,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "user-1"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["role"] != string(models.TeamRoleMember) {
			t.Errorf("expected role %s, got %v", models.TeamRoleMember, resp["role"])
		}
	})
}

func TestHandleRemoveTeamMember_ErrorPaths(t *testing.T) {
	now := time.Now()
	org := &models.Organization{
		ID:       "org-1",
		Name:     "Test Org",
		Slug:     "test-org",
		IsActive: true,
	}

	team := &models.Team{
		ID:             "team-1",
		OrganizationID: "org-1",
		Name:           "Team Alpha",
		IsActive:       true,
	}

	t.Run("team repo not configured", func(t *testing.T) {
		orgMgr := createTestServerWithTeams(t, nil, nil)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/user-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "admin-user"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
		}
	})

	t.Run("database error on team GetByID", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams:      map[string]*models.Team{},
			getByIDErr: errors.New("database error"),
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: make(map[string]*models.TeamMembership),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/user-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "admin-user"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("error getting membership", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships:         make(map[string]*models.TeamMembership),
			getByTeamAndUserErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/user-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "admin-user"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})

	t.Run("error listing leads when removing lead", func(t *testing.T) {
		teamRepo := &mockTeamRepoHandler{
			teams: map[string]*models.Team{"team-1": team},
		}
		teamMembershipRepo := &mockTeamMembershipRepoHandler{
			memberships: map[string]*models.TeamMembership{
				"tm-1": {
					ID:     "tm-1",
					TeamID: "team-1",
					UserID: "user-1",
					Role:   models.TeamRoleLead,
				},
			},
			listByTeamAndRoleErr: errors.New("database error"),
		}
		orgMgr := createTestServerWithTeams(t, teamRepo, teamMembershipRepo)

		r := chi.NewRouter()
		r.Delete("/teams/{teamID}/members/{userID}", orgMgr.handleRemoveTeamMember())

		req := httptest.NewRequest(http.MethodDelete, "/teams/team-1/members/user-1", nil)
		req = addTenantToContext(req, &models.TenantContext{
			Organization: org,
			Membership: &models.OrganizationMembership{
				Role:       models.OrgRoleAdmin,
				AcceptedAt: &now,
			},
		})
		req = addUserToContext(req, &auth.User{ID: "admin-user"})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
		}
	})
}
