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

	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Mock License Repository
// -----------------------------------------------------------------------------

type mockLicenseRepo struct {
	licenses          map[string]*models.License
	orgLicenses       map[string]*models.License // orgID -> license
	keyLicenses       map[string]*models.License // licenseKey -> license
	createErr         error
	getErr            error
	updateErr         error
	deactivateErr     error
	validationUpdated bool
}

func newMockLicenseRepo() *mockLicenseRepo {
	return &mockLicenseRepo{
		licenses:    make(map[string]*models.License),
		orgLicenses: make(map[string]*models.License),
		keyLicenses: make(map[string]*models.License),
	}
}

func (m *mockLicenseRepo) Create(ctx context.Context, license *models.License) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.licenses[license.ID] = license
	m.orgLicenses[license.OrganizationID] = license
	m.keyLicenses[license.LicenseKey] = license
	return nil
}

func (m *mockLicenseRepo) GetByID(ctx context.Context, id string) (*models.License, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.licenses[id], nil
}

func (m *mockLicenseRepo) GetByOrganization(ctx context.Context, orgID string) (*models.License, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.orgLicenses[orgID], nil
}

func (m *mockLicenseRepo) GetByLicenseKey(ctx context.Context, key string) (*models.License, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.keyLicenses[key], nil
}

func (m *mockLicenseRepo) GetActiveByOrganization(ctx context.Context, orgID string) (*models.License, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	license := m.orgLicenses[orgID]
	if license != nil && license.IsActive && license.ValidationStatus == models.LicenseStatusValid {
		return license, nil
	}
	return nil, nil
}

func (m *mockLicenseRepo) Update(ctx context.Context, license *models.License) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.licenses[license.ID] = license
	m.orgLicenses[license.OrganizationID] = license
	return nil
}

func (m *mockLicenseRepo) UpdateValidationStatus(ctx context.Context, id string, status models.LicenseStatus, errorMsg *string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if license, ok := m.licenses[id]; ok {
		license.ValidationStatus = status
		license.ValidationError = errorMsg
		now := time.Now()
		license.LastValidatedAt = &now
		m.validationUpdated = true
	}
	return nil
}

func (m *mockLicenseRepo) Deactivate(ctx context.Context, id string) error {
	if m.deactivateErr != nil {
		return m.deactivateErr
	}
	if license, ok := m.licenses[id]; ok {
		license.IsActive = false
	}
	return nil
}

// Helper to add a license to the mock
func (m *mockLicenseRepo) addLicense(license *models.License) {
	m.licenses[license.ID] = license
	m.orgLicenses[license.OrganizationID] = license
	if license.LicenseKey != "" {
		m.keyLicenses[license.LicenseKey] = license
	}
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func createTestServerWithLicense(t *testing.T, licenseRepo *mockLicenseRepo, orgRepo *mockOrgRepoHandler) *Manager {
	t.Helper()

	cfg := Config{
		Logger: newTestLogger(),
	}
	// Only assign if not nil to avoid interface wrapper around nil pointer
	if licenseRepo != nil {
		cfg.LicenseRepo = licenseRepo
	}
	if orgRepo != nil {
		cfg.OrgRepo = orgRepo
	}

	return NewManager(cfg)
}

func addTenantContextToRequest(r *http.Request, tc *models.TenantContext) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.TenantContextKey{}, tc)
	return r.WithContext(ctx)
}

func createTestTenantContext(role models.OrgRole) *models.TenantContext {
	return &models.TenantContext{
		Organization: &models.Organization{
			ID:       "org-123",
			Name:     "Test Org",
			Slug:     "test-org",
			Edition:  models.EditionCommunity,
			IsActive: true,
		},
		Membership: &models.OrganizationMembership{
			ID:             "mem-123",
			OrganizationID: "org-123",
			UserID:         "user-123",
			Role:           role,
		},
	}
}

func createTestLicense(orgID string) *models.License {
	now := time.Now()
	expires := now.AddDate(1, 0, 0)
	maxUsers := 100
	maxPods := 20
	maxStorage := 100
	return &models.License{
		ID:               "lic-123",
		OrganizationID:   orgID,
		LicenseKey:       "PRO-1234-5678-9012-3456",
		Edition:          models.EditionProfessional,
		IssuedAt:         now,
		ExpiresAt:        &expires,
		MaxUsers:         &maxUsers,
		MaxPods:          &maxPods,
		MaxStorageGB:     &maxStorage,
		Features:         []string{"teams", "labs.custom"},
		IsActive:         true,
		ValidationStatus: models.LicenseStatusValid,
		LastValidatedAt:  &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// -----------------------------------------------------------------------------
// handleGetLicense Tests
// -----------------------------------------------------------------------------

func TestHandleGetLicense(t *testing.T) {
	t.Run("error - no tenant context", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)

		req := httptest.NewRequest(http.MethodGet, "/organizations/org-123/license", nil)
		rr := httptest.NewRecorder()

		orgMgr.handleGetLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - not admin", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleMember)

		req := httptest.NewRequest(http.MethodGet, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleGetLicense()(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("error - no license repo", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, nil, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodGet, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleGetLicense()(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("error - repo error", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		licenseRepo.getErr = errors.New("database error")
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodGet, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleGetLicense()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("error - no active license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodGet, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleGetLicense()(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("success - returns license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		license := createTestLicense("org-123")
		licenseRepo.addLicense(license)
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodGet, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleGetLicense()(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp LicenseResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.ID != "lic-123" {
			t.Errorf("expected license ID 'lic-123', got '%s'", resp.ID)
		}
		if resp.Edition != models.EditionProfessional {
			t.Errorf("expected edition Professional, got %s", resp.Edition)
		}
		if !resp.IsValid {
			t.Error("expected license to be valid")
		}
	})
}

// -----------------------------------------------------------------------------
// handleActivateLicense Tests
// -----------------------------------------------------------------------------

func TestHandleActivateLicense(t *testing.T) {
	t.Run("error - no tenant context", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)

		body := bytes.NewBufferString(`{"licenseKey": "PRO-1234-5678-9012-3456"}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - not owner", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleAdmin) // Admin is not enough

		body := bytes.NewBufferString(`{"licenseKey": "PRO-1234-5678-9012-3456"}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("error - invalid request body", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleOwner)

		body := bytes.NewBufferString(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - empty license key", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleOwner)

		body := bytes.NewBufferString(`{"licenseKey": ""}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - license key in use by another org", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		existingLicense := createTestLicense("other-org")
		existingLicense.LicenseKey = "PRO-USED-KEY1-2345-6789"
		licenseRepo.addLicense(existingLicense)

		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleOwner)

		body := bytes.NewBufferString(`{"licenseKey": "PRO-USED-KEY1-2345-6789"}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, rr.Code)
		}
	})

	t.Run("error - invalid license key format", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleOwner)

		body := bytes.NewBufferString(`{"licenseKey": "short"}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("success - activates license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		orgRepo := &mockOrgRepoHandler{
			orgs: map[string]*models.Organization{
				"org-123": {
					ID:       "org-123",
					Name:     "Test Org",
					Edition:  models.EditionCommunity,
					IsActive: true,
				},
			},
		}
		orgMgr := createTestServerWithLicense(t, licenseRepo, orgRepo)
		tc := createTestTenantContext(models.OrgRoleOwner)

		body := bytes.NewBufferString(`{"licenseKey": "PRO-1234-5678-9012-3456"}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var resp LicenseResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Edition != models.EditionProfessional {
			t.Errorf("expected edition Professional, got %s", resp.Edition)
		}
		if !resp.IsActive {
			t.Error("expected license to be active")
		}
	})

	t.Run("success - enterprise license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleOwner)

		body := bytes.NewBufferString(`{"licenseKey": "ENT-1234-5678-9012-3456"}`)
		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license", body)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleActivateLicense()(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var resp LicenseResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Edition != models.EditionEnterprise {
			t.Errorf("expected edition Enterprise, got %s", resp.Edition)
		}
	})
}

// -----------------------------------------------------------------------------
// handleDeactivateLicense Tests
// -----------------------------------------------------------------------------

func TestHandleDeactivateLicense(t *testing.T) {
	t.Run("error - no tenant context", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)

		req := httptest.NewRequest(http.MethodDelete, "/organizations/org-123/license", nil)
		rr := httptest.NewRecorder()

		orgMgr.handleDeactivateLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - not owner", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodDelete, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleDeactivateLicense()(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("error - no active license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleOwner)

		req := httptest.NewRequest(http.MethodDelete, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleDeactivateLicense()(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("success - deactivates license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		license := createTestLicense("org-123")
		licenseRepo.addLicense(license)

		orgRepo := &mockOrgRepoHandler{
			orgs: map[string]*models.Organization{
				"org-123": {
					ID:       "org-123",
					Name:     "Test Org",
					Edition:  models.EditionProfessional,
					IsActive: true,
				},
			},
		}
		orgMgr := createTestServerWithLicense(t, licenseRepo, orgRepo)
		tc := createTestTenantContext(models.OrgRoleOwner)

		req := httptest.NewRequest(http.MethodDelete, "/organizations/org-123/license", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleDeactivateLicense()(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		// Verify license was deactivated
		if license.IsActive {
			t.Error("expected license to be deactivated")
		}
	})
}

// -----------------------------------------------------------------------------
// handleValidateLicense Tests
// -----------------------------------------------------------------------------

func TestHandleValidateLicense(t *testing.T) {
	t.Run("error - no tenant context", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)

		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license/validate", nil)
		rr := httptest.NewRecorder()

		orgMgr.handleValidateLicense()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - not admin", func(t *testing.T) {
		orgMgr := createTestServerWithLicense(t, newMockLicenseRepo(), nil)
		tc := createTestTenantContext(models.OrgRoleMember)

		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license/validate", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleValidateLicense()(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("error - no active license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license/validate", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleValidateLicense()(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("success - validates active license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		license := createTestLicense("org-123")
		licenseRepo.addLicense(license)
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license/validate", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleValidateLicense()(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		if !licenseRepo.validationUpdated {
			t.Error("expected validation status to be updated")
		}

		var resp LicenseResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.ValidationStatus != models.LicenseStatusValid {
			t.Errorf("expected status Valid, got %s", resp.ValidationStatus)
		}
	})

	t.Run("success - detects expired license", func(t *testing.T) {
		licenseRepo := newMockLicenseRepo()
		license := createTestLicense("org-123")
		expired := time.Now().AddDate(-1, 0, 0) // 1 year ago
		license.ExpiresAt = &expired
		licenseRepo.addLicense(license)
		orgMgr := createTestServerWithLicense(t, licenseRepo, nil)
		tc := createTestTenantContext(models.OrgRoleAdmin)

		req := httptest.NewRequest(http.MethodPost, "/organizations/org-123/license/validate", nil)
		req = addTenantContextToRequest(req, tc)
		rr := httptest.NewRecorder()

		orgMgr.handleValidateLicense()(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp LicenseResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.ValidationStatus != models.LicenseStatusExpired {
			t.Errorf("expected status Expired, got %s", resp.ValidationStatus)
		}
	})
}

// -----------------------------------------------------------------------------
// Helper Function Tests
// -----------------------------------------------------------------------------

func TestValidateLicenseKey(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantErr     bool
		wantEdition models.Edition
	}{
		{
			name:    "error - key too short",
			key:     "short",
			wantErr: true,
		},
		{
			name:        "success - enterprise key",
			key:         "ENT-1234-5678-9012",
			wantErr:     false,
			wantEdition: models.EditionEnterprise,
		},
		{
			name:        "success - professional key",
			key:         "PRO-1234-5678-9012",
			wantErr:     false,
			wantEdition: models.EditionProfessional,
		},
		{
			name:        "success - unknown prefix defaults to professional",
			key:         "XXX-1234-5678-9012",
			wantErr:     false,
			wantEdition: models.EditionProfessional,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := validateLicenseKey(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Edition != tt.wantEdition {
				t.Errorf("expected edition %s, got %s", tt.wantEdition, info.Edition)
			}
		})
	}
}

func TestGenerateLicenseKey(t *testing.T) {
	tests := []struct {
		edition    models.Edition
		wantPrefix string
	}{
		{models.EditionEnterprise, "ENT-"},
		{models.EditionProfessional, "PRO-"},
		{models.EditionCommunity, "COM-"},
	}

	for _, tt := range tests {
		t.Run(string(tt.edition), func(t *testing.T) {
			key := generateLicenseKey(tt.edition)
			if len(key) < 20 {
				t.Errorf("key too short: %s", key)
			}
			if key[:4] != tt.wantPrefix {
				t.Errorf("expected prefix %s, got %s", tt.wantPrefix, key[:4])
			}
		})
	}
}

func TestToLicenseResponse(t *testing.T) {
	license := createTestLicense("org-123")
	resp := toLicenseResponse(license)

	if resp.ID != license.ID {
		t.Errorf("expected ID %s, got %s", license.ID, resp.ID)
	}
	if resp.Edition != license.Edition {
		t.Errorf("expected edition %s, got %s", license.Edition, resp.Edition)
	}
	if resp.MaskedKey != license.MaskedKey() {
		t.Errorf("expected masked key %s, got %s", license.MaskedKey(), resp.MaskedKey)
	}
	if !resp.IsValid {
		t.Error("expected license to be valid")
	}
	if resp.DaysRemaining == nil {
		t.Error("expected DaysRemaining to be set")
	}
	if *resp.DaysRemaining < 360 {
		t.Errorf("expected ~365 days remaining, got %d", *resp.DaysRemaining)
	}
}
