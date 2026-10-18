package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// mockOrgRepo is a mock organization repository for testing
type mockOrgRepo struct {
	orgs map[string]*models.Organization
}

func (m *mockOrgRepo) Create(ctx context.Context, org *models.Organization) error {
	return nil
}

func (m *mockOrgRepo) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	return m.orgs[id], nil
}

func (m *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	for _, org := range m.orgs {
		if org.Slug == slug {
			return org, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) List(ctx context.Context, filter repositories.OrganizationFilter) ([]*models.Organization, error) {
	return nil, nil
}

func (m *mockOrgRepo) Update(ctx context.Context, org *models.Organization) error {
	return nil
}

func (m *mockOrgRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockOrgRepo) SetActive(ctx context.Context, id string, active bool) error {
	return nil
}

func (m *mockOrgRepo) GetMemberCount(ctx context.Context, id string) (int, error) {
	return 0, nil
}

func (m *mockOrgRepo) GetTeamCount(ctx context.Context, id string) (int, error) {
	return 0, nil
}

func (m *mockOrgRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.Organization, error) {
	return nil, nil
}

func (m *mockOrgRepo) GetMemberCountBatch(ctx context.Context, ids []string) (map[string]int, error) {
	return nil, nil
}

func (m *mockOrgRepo) GetTeamCountBatch(ctx context.Context, ids []string) (map[string]int, error) {
	return nil, nil
}

// mockMembershipRepo is a mock membership repository for testing
type mockMembershipRepo struct {
	memberships map[string]*models.OrganizationMembership
}

func (m *mockMembershipRepo) Create(ctx context.Context, membership *models.OrganizationMembership) error {
	return nil
}

func (m *mockMembershipRepo) GetByID(ctx context.Context, id string) (*models.OrganizationMembership, error) {
	return nil, nil
}

func (m *mockMembershipRepo) GetByOrgAndUser(ctx context.Context, orgID, userID string) (*models.OrganizationMembership, error) {
	key := orgID + ":" + userID
	return m.memberships[key], nil
}

func (m *mockMembershipRepo) ListByOrganization(ctx context.Context, orgID string, filter repositories.MembershipFilter) ([]*models.OrganizationMembership, error) {
	return nil, nil
}

func (m *mockMembershipRepo) ListByUser(ctx context.Context, userID string) ([]*models.OrganizationMembership, error) {
	return nil, nil
}

func (m *mockMembershipRepo) Update(ctx context.Context, membership *models.OrganizationMembership) error {
	return nil
}

func (m *mockMembershipRepo) UpdateRole(ctx context.Context, id string, role models.OrgRole) error {
	return nil
}

func (m *mockMembershipRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockMembershipRepo) AcceptInvitation(ctx context.Context, token string) (*models.OrganizationMembership, error) {
	return nil, nil
}

func (m *mockMembershipRepo) SetPrimary(ctx context.Context, userID, orgID string) error {
	return nil
}

func (m *mockMembershipRepo) GetPrimaryOrganization(ctx context.Context, userID string) (*models.Organization, error) {
	return nil, nil
}

func TestTenantFromContext(t *testing.T) {
	t.Run("returns nil when no tenant in context", func(t *testing.T) {
		ctx := context.Background()
		tc, ok := TenantFromContext(ctx)
		if ok {
			t.Error("expected ok to be false")
		}
		if tc != nil {
			t.Error("expected tenant context to be nil")
		}
	})

	t.Run("returns tenant when present in context", func(t *testing.T) {
		org := &models.Organization{
			ID:      "org-123",
			Name:    "Test Org",
			Edition: models.EditionCommunity,
		}
		tc := &models.TenantContext{
			Organization: org,
			Edition:      models.EditionCommunity,
		}
		ctx := context.WithValue(context.Background(), TenantContextKey{}, tc)

		got, ok := TenantFromContext(ctx)
		if !ok {
			t.Error("expected ok to be true")
		}
		if got == nil {
			t.Fatal("expected tenant context to not be nil")
		}
		if got.Organization.ID != "org-123" {
			t.Errorf("got org ID %s, want org-123", got.Organization.ID)
		}
	})
}

func TestRequireTenant(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("returns 400 when no tenant context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		RequireTenant(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("continues when tenant context present", func(t *testing.T) {
		org := &models.Organization{ID: "org-123"}
		tc := &models.TenantContext{Organization: org}
		ctx := context.WithValue(context.Background(), TenantContextKey{}, tc)

		req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		RequireTenant(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

func TestRequireOrgRole(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	now := func() *interface{} { return nil }()
	_ = now

	tests := []struct {
		name       string
		minRole    models.OrgRole
		membership *models.OrganizationMembership
		wantStatus int
	}{
		{
			name:       "no tenant context",
			minRole:    models.OrgRoleMember,
			membership: nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:    "no membership",
			minRole: models.OrgRoleMember,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleMember,
			},
			wantStatus: http.StatusOK, // Membership will be set in context
		},
		{
			name:    "member accessing member route",
			minRole: models.OrgRoleMember,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleMember,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "member accessing instructor route",
			minRole: models.OrgRoleInstructor,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleMember,
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:    "instructor accessing instructor route",
			minRole: models.OrgRoleInstructor,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleInstructor,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "admin accessing instructor route",
			minRole: models.OrgRoleInstructor,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleAdmin,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "owner accessing admin route",
			minRole: models.OrgRoleAdmin,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleOwner,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "admin accessing owner route",
			minRole: models.OrgRoleOwner,
			membership: &models.OrganizationMembership{
				Role: models.OrgRoleAdmin,
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)

			if tt.membership != nil {
				org := &models.Organization{ID: "org-123"}
				tc := &models.TenantContext{
					Organization: org,
					Membership:   tt.membership,
				}
				req = req.WithContext(context.WithValue(context.Background(), TenantContextKey{}, tc))
			}

			rr := httptest.NewRecorder()
			RequireOrgRole(tt.minRole)(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireFeature(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		featureID  string
		features   map[string]bool
		wantStatus int
	}{
		{
			name:       "no tenant context",
			featureID:  "teams",
			features:   nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "feature enabled",
			featureID:  "teams",
			features:   map[string]bool{"teams": true},
			wantStatus: http.StatusOK,
		},
		{
			name:       "feature disabled",
			featureID:  "teams",
			features:   map[string]bool{"teams": false},
			wantStatus: http.StatusPaymentRequired,
		},
		{
			name:       "feature not present",
			featureID:  "custom_labs",
			features:   map[string]bool{"teams": true},
			wantStatus: http.StatusPaymentRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)

			if tt.features != nil {
				org := &models.Organization{ID: "org-123"}
				tc := &models.TenantContext{
					Organization: org,
					Features:     tt.features,
				}
				req = req.WithContext(context.WithValue(context.Background(), TenantContextKey{}, tc))
			}

			rr := httptest.NewRecorder()
			RequireFeature(tt.featureID)(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireEdition(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		minEdition models.Edition
		edition    models.Edition
		wantStatus int
	}{
		{
			name:       "community meets community",
			minEdition: models.EditionCommunity,
			edition:    models.EditionCommunity,
			wantStatus: http.StatusOK,
		},
		{
			name:       "professional meets community",
			minEdition: models.EditionCommunity,
			edition:    models.EditionProfessional,
			wantStatus: http.StatusOK,
		},
		{
			name:       "community fails professional",
			minEdition: models.EditionProfessional,
			edition:    models.EditionCommunity,
			wantStatus: http.StatusPaymentRequired,
		},
		{
			name:       "enterprise meets enterprise",
			minEdition: models.EditionEnterprise,
			edition:    models.EditionEnterprise,
			wantStatus: http.StatusOK,
		},
		{
			name:       "professional fails enterprise",
			minEdition: models.EditionEnterprise,
			edition:    models.EditionProfessional,
			wantStatus: http.StatusPaymentRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := &models.Organization{ID: "org-123", Edition: tt.edition}
			tc := &models.TenantContext{
				Organization: org,
				Edition:      tt.edition,
			}
			ctx := context.WithValue(context.Background(), TenantContextKey{}, tc)

			req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
			rr := httptest.NewRecorder()

			RequireEdition(tt.minEdition)(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestTenantMiddleware_HeaderResolution(t *testing.T) {
	org := &models.Organization{
		ID:       "org-123",
		Name:     "Test Org",
		Slug:     "test-org",
		Edition:  models.EditionCommunity,
		IsActive: true,
	}

	orgRepo := &mockOrgRepo{
		orgs: map[string]*models.Organization{
			"org-123": org,
		},
	}

	membershipRepo := &mockMembershipRepo{
		memberships: map[string]*models.OrganizationMembership{},
	}

	service := NewTenantService(orgRepo, membershipRepo, nil, DefaultTenantConfig())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := TenantFromContext(r.Context())
		if !ok || tc == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(tc.Organization.ID))
	})

	t.Run("resolves org from X-Organization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Organization", "org-123")
		rr := httptest.NewRecorder()

		service.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
		if rr.Body.String() != "org-123" {
			t.Errorf("got body %s, want org-123", rr.Body.String())
		}
	})

	t.Run("returns 404 for non-existent org", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Organization", "nonexistent")
		rr := httptest.NewRecorder()

		service.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

func TestTenantMiddleware_URLParamResolution(t *testing.T) {
	org := &models.Organization{
		ID:       "org-456",
		Name:     "URL Param Org",
		Slug:     "url-org",
		Edition:  models.EditionProfessional,
		IsActive: true,
	}

	orgRepo := &mockOrgRepo{
		orgs: map[string]*models.Organization{
			"org-456": org,
		},
	}

	membershipRepo := &mockMembershipRepo{
		memberships: map[string]*models.OrganizationMembership{},
	}

	service := NewTenantService(orgRepo, membershipRepo, nil, DefaultTenantConfig())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := TenantFromContext(r.Context())
		if !ok || tc == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(tc.Organization.ID))
	})

	// Create chi router to properly handle URL params
	r := chi.NewRouter()
	r.Route("/organizations/{orgID}", func(r chi.Router) {
		r.Use(service.Middleware)
		r.Get("/", handler)
	})

	t.Run("resolves org from URL param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/organizations/org-456/", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
		if rr.Body.String() != "org-456" {
			t.Errorf("got body %s, want org-456", rr.Body.String())
		}
	})
}

func TestTenantMiddleware_WithAuthenticatedUser(t *testing.T) {
	org := &models.Organization{
		ID:       "org-789",
		Name:     "Auth Test Org",
		Slug:     "auth-org",
		Edition:  models.EditionEnterprise,
		IsActive: true,
	}

	membership := &models.OrganizationMembership{
		ID:             "mem-123",
		OrganizationID: "org-789",
		UserID:         "user-123",
		Role:           models.OrgRoleAdmin,
		AcceptedAt:     ptrTimeNow(),
	}

	orgRepo := &mockOrgRepo{
		orgs: map[string]*models.Organization{
			"org-789": org,
		},
	}

	membershipRepo := &mockMembershipRepo{
		memberships: map[string]*models.OrganizationMembership{
			"org-789:user-123": membership,
		},
	}

	service := NewTenantService(orgRepo, membershipRepo, nil, DefaultTenantConfig())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := TenantFromContext(r.Context())
		if !ok || tc == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if tc.Membership == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("no membership"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(string(tc.Membership.Role)))
	})

	t.Run("loads membership for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Organization", "org-789")

		// Add authenticated user to context
		user := &auth.User{ID: "user-123", Email: "test@example.com"}
		ctx := auth.ContextWithUser(req.Context(), user)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		service.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if rr.Body.String() != string(models.OrgRoleAdmin) {
			t.Errorf("got role %s, want %s", rr.Body.String(), models.OrgRoleAdmin)
		}
	})
}

func TestTenantMiddleware_InactiveOrg(t *testing.T) {
	org := &models.Organization{
		ID:       "org-inactive",
		Name:     "Inactive Org",
		Slug:     "inactive-org",
		Edition:  models.EditionCommunity,
		IsActive: false, // Inactive!
	}

	orgRepo := &mockOrgRepo{
		orgs: map[string]*models.Organization{
			"org-inactive": org,
		},
	}

	service := NewTenantService(orgRepo, nil, nil, DefaultTenantConfig())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("returns 403 for inactive org", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Organization", "org-inactive")
		rr := httptest.NewRecorder()

		service.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}

func TestTenantMiddleware_NonMemberRejected(t *testing.T) {
	org := &models.Organization{
		ID:       "org-secured",
		Name:     "Secured Org",
		Slug:     "secured-org",
		Edition:  models.EditionEnterprise,
		IsActive: true,
	}

	orgRepo := &mockOrgRepo{
		orgs: map[string]*models.Organization{
			"org-secured": org,
		},
	}

	// Empty membership repo - user is NOT a member
	membershipRepo := &mockMembershipRepo{
		memberships: map[string]*models.OrganizationMembership{},
	}

	service := NewTenantService(orgRepo, membershipRepo, nil, DefaultTenantConfig())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("returns 403 for authenticated non-member", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Organization", "org-secured")

		// Add authenticated user who is NOT a member
		user := &auth.User{ID: "user-outsider", Email: "outsider@example.com"}
		ctx := auth.ContextWithUser(req.Context(), user)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		service.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d: %s", rr.Code, http.StatusForbidden, rr.Body.String())
		}
	})
}

func TestTenantMiddleware_PendingMemberRejected(t *testing.T) {
	org := &models.Organization{
		ID:       "org-pending",
		Name:     "Pending Org",
		Slug:     "pending-org",
		Edition:  models.EditionProfessional,
		IsActive: true,
	}

	// Membership exists but AcceptedAt is nil (pending invitation)
	membership := &models.OrganizationMembership{
		ID:             "mem-pending",
		OrganizationID: "org-pending",
		UserID:         "user-pending",
		Role:           models.OrgRoleMember,
		AcceptedAt:     nil, // NOT ACCEPTED YET
	}

	orgRepo := &mockOrgRepo{
		orgs: map[string]*models.Organization{
			"org-pending": org,
		},
	}

	membershipRepo := &mockMembershipRepo{
		memberships: map[string]*models.OrganizationMembership{
			"org-pending:user-pending": membership,
		},
	}

	service := NewTenantService(orgRepo, membershipRepo, nil, DefaultTenantConfig())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("returns 403 for pending member", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Organization", "org-pending")

		// Add user with pending invitation
		user := &auth.User{ID: "user-pending", Email: "pending@example.com"}
		ctx := auth.ContextWithUser(req.Context(), user)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		service.Middleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d: %s", rr.Code, http.StatusForbidden, rr.Body.String())
		}
	})
}

// Helper to create a non-nil time pointer for testing
func ptrTimeNow() *time.Time {
	t := time.Now()
	return &t
}
