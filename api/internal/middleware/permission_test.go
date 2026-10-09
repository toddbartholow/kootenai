package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// MockRBACRepository implements a mock RBAC repository for testing
type MockRBACRepository struct {
	permissions   map[string][]string        // userID -> permissions
	hasPermResult map[string]bool            // "userID:permissionID" -> result
	checkResults  map[string]map[string]bool // userID -> permissionID -> result
}

func NewMockRBACRepository() *MockRBACRepository {
	return &MockRBACRepository{
		permissions:   make(map[string][]string),
		hasPermResult: make(map[string]bool),
		checkResults:  make(map[string]map[string]bool),
	}
}

func (m *MockRBACRepository) SetUserPermissions(userID string, permissions []string) {
	m.permissions[userID] = permissions
}

func (m *MockRBACRepository) SetHasPermission(userID, permissionID string, result bool) {
	key := userID + ":" + permissionID
	m.hasPermResult[key] = result
}

func (m *MockRBACRepository) GetPermission(ctx context.Context, id string) (*models.Permission, error) {
	return nil, nil
}

func (m *MockRBACRepository) ListPermissions(ctx context.Context, resource string) ([]*models.Permission, error) {
	return nil, nil
}

func (m *MockRBACRepository) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}

func (m *MockRBACRepository) GetRole(ctx context.Context, id string) (*models.Role, error) {
	return nil, nil
}

func (m *MockRBACRepository) GetRoleBySlug(ctx context.Context, slug string) (*models.Role, error) {
	return nil, nil
}

func (m *MockRBACRepository) GetRoleWithPermissions(ctx context.Context, id string) (*models.Role, error) {
	return nil, nil
}

func (m *MockRBACRepository) ListRoles(ctx context.Context, orgID *string, includeSystem bool) ([]*models.Role, error) {
	return nil, nil
}

func (m *MockRBACRepository) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}

func (m *MockRBACRepository) DeleteRole(ctx context.Context, id string) error {
	return nil
}

func (m *MockRBACRepository) SetRolePermissionSets(ctx context.Context, roleID string, permissionSetIDs []string) error {
	return nil
}

func (m *MockRBACRepository) SetRolePermissions(ctx context.Context, roleID string, permissions []models.RolePermission) error {
	return nil
}

func (m *MockRBACRepository) GetRolePermissions(ctx context.Context, roleID string) ([]string, error) {
	return nil, nil
}

func (m *MockRBACRepository) AssignRole(ctx context.Context, userRole *models.UserRole) error {
	return nil
}

func (m *MockRBACRepository) RevokeRole(ctx context.Context, userID, roleID string, orgID *string) error {
	return nil
}

func (m *MockRBACRepository) GetUserRoles(ctx context.Context, userID string, orgID *string) ([]*models.UserRole, error) {
	return nil, nil
}

func (m *MockRBACRepository) BulkAssignRole(ctx context.Context, userIDs []string, roleID string, orgID *string, grantedBy *string) error {
	return nil
}

func (m *MockRBACRepository) BulkRevokeRole(ctx context.Context, userIDs []string, roleID string, orgID *string) error {
	return nil
}

func (m *MockRBACRepository) HasPermission(ctx context.Context, userID, permissionID string, orgID *string) (bool, error) {
	key := userID + ":" + permissionID
	return m.hasPermResult[key], nil
}

func (m *MockRBACRepository) GetUserPermissions(ctx context.Context, userID string, orgID *string) ([]string, error) {
	return m.permissions[userID], nil
}

func (m *MockRBACRepository) CheckPermissions(ctx context.Context, userID string, permissionIDs []string, orgID *string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, permID := range permissionIDs {
		key := userID + ":" + permID
		result[permID] = m.hasPermResult[key]
	}
	return result, nil
}

func (m *MockRBACRepository) GetUsersWithPermission(ctx context.Context, permissionID string, orgID *string) ([]string, error) {
	return nil, nil
}

func (m *MockRBACRepository) CleanupExpiredRoles(ctx context.Context) (int64, error) {
	return 0, nil
}

// Test helpers
func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

func TestRequirePermission_NoAuth(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequirePermission("pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}

func TestRequirePermission_AdminBypasses(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequirePermission("pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	// Add admin user to context
	user := &auth.User{
		ID:    "admin-user",
		Email: "admin@example.com",
		Roles: []string{"admin"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Admin should bypass RBAC, got %d", rec.Code)
	}
}

func TestRequirePermission_HasPermission(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.create", true)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequirePermission("pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("User with permission should be allowed, got %d", rec.Code)
	}
}

func TestRequirePermission_NoPermission(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.create", false)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequirePermission("pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("User without permission should be denied, got %d", rec.Code)
	}
}

func TestRequirePermission_WithCachedPermissions(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	// Don't set permission in repo - we'll use cached permissions
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequirePermission("pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)

	// Add cached permissions to context
	cached := &CachedPermissions{
		UserID: "user-123",
		Permissions: map[string]bool{
			"pods.create": true,
			"pods.read":   true,
		},
	}
	ctx = context.WithValue(ctx, PermissionContextKey{}, cached)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("User with cached permission should be allowed, got %d", rec.Code)
	}
}

func TestRequireAnyPermission_HasOne(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.read", true)
	mockRepo.SetHasPermission("user-123", "pods.create", false)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequireAnyPermission("pods.read", "pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("User with any matching permission should be allowed, got %d", rec.Code)
	}
}

func TestRequireAnyPermission_HasNone(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.read", false)
	mockRepo.SetHasPermission("user-123", "pods.create", false)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequireAnyPermission("pods.read", "pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("User without any matching permission should be denied, got %d", rec.Code)
	}
}

func TestRequireAllPermissions_HasAll(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.read", true)
	mockRepo.SetHasPermission("user-123", "pods.create", true)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequireAllPermissions("pods.read", "pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("User with all permissions should be allowed, got %d", rec.Code)
	}
}

func TestRequireAllPermissions_MissingOne(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.read", true)
	mockRepo.SetHasPermission("user-123", "pods.create", false)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequireAllPermissions("pods.read", "pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("User missing one permission should be denied, got %d", rec.Code)
	}
}

func TestLoadPermissions(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetUserPermissions("user-123", []string{"pods.read", "pods.create", "labs.read"})
	service := NewPermissionService(mockRepo, nil)

	// Handler that checks cached permissions
	var cachedPerms *CachedPermissions
	checkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		cachedPerms, ok = PermissionsFromContext(r.Context())
		if !ok {
			t.Error("Expected cached permissions in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := service.LoadPermissions(checkHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if cachedPerms == nil {
		t.Fatal("Expected cached permissions")
	}
	if len(cachedPerms.Permissions) != 3 {
		t.Errorf("Expected 3 permissions, got %d", len(cachedPerms.Permissions))
	}
	if !cachedPerms.Permissions["pods.read"] {
		t.Error("Expected pods.read permission")
	}
}

func TestCheckPermission_Direct(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	mockRepo.SetHasPermission("user-123", "pods.create", true)
	service := NewPermissionService(mockRepo, nil)

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)

	result := service.CheckPermission(ctx, "pods.create")
	if !result {
		t.Error("Expected permission check to return true")
	}
}

func TestCheckPermission_AdminBypass(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	// Don't set any permission - admin should bypass
	service := NewPermissionService(mockRepo, nil)

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "admin-user",
		Email: "admin@example.com",
		Roles: []string{"admin"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)

	result := service.CheckPermission(ctx, "any.permission")
	if !result {
		t.Error("Admin should bypass permission checks")
	}
}

func TestRequirePermission_WithTenantContext(t *testing.T) {
	mockRepo := NewMockRBACRepository()
	// Permission is granted only with org context
	mockRepo.SetHasPermission("user-123", "pods.create", true)
	service := NewPermissionService(mockRepo, nil)

	handler := service.RequirePermission("pods.create")(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	user := &auth.User{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"student"},
	}
	ctx := auth.ContextWithUser(req.Context(), user)

	// Add tenant context
	orgID := "org-456"
	tenantCtx := &models.TenantContext{
		Organization: &models.Organization{ID: orgID},
	}
	ctx = context.WithValue(ctx, TenantContextKey{}, tenantCtx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("User with permission in org context should be allowed, got %d", rec.Code)
	}
}
