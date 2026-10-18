package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// adminUser creates an admin user for tests
func adminUser() *auth.User {
	return &auth.User{
		ID:    "admin-123",
		Email: "admin@example.com",
		Roles: []string{"admin"},
	}
}

func TestHandleListLabsWithRepo(t *testing.T) {
	t.Run("returns labs", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:          "lab-1",
			Name:        "basic-lab",
			Description: "A basic lab",
			Version:     "1.0.0",
			Platform:    "proxmox",
			MaxPoints:   100,
			IsActive:    true,
			Visibility:  models.LabVisibilityGlobal,
		})
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:          "lab-2",
			Name:        "advanced-lab",
			Description: "An advanced lab",
			Version:     "2.0.0",
			Platform:    "cloudstack",
			MaxPoints:   200,
			IsActive:    true,
			Visibility:  models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count, ok := response["count"].(float64)
		if !ok {
			t.Fatal("expected count in response")
		}

		if int(count) != 2 {
			t.Errorf("expected count 2, got %d", int(count))
		}

		labs, ok := response["labs"].([]interface{})
		if !ok {
			t.Fatal("expected labs array in response")
		}

		if len(labs) != 2 {
			t.Errorf("expected 2 labs, got %d", len(labs))
		}
	})

	t.Run("empty list", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count := response["count"].(float64)
		if int(count) != 0 {
			t.Errorf("expected count 0, got %d", int(count))
		}
	})

	t.Run("filter by platform", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-1",
			Name:       "proxmox-lab",
			Platform:   "proxmox",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-2",
			Name:       "cloudstack-lab",
			Platform:   "cloudstack",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs?platform=proxmox", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count := response["count"].(float64)
		if int(count) != 1 {
			t.Errorf("expected count 1, got %d", int(count))
		}
	})

	t.Run("database error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.ListErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("admin with all=true bypasses org filter", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-1",
			Name:       "global-lab",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs?all=true", nil)
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("user with org gets org scoped labs", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		orgID := "org-123"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:             "lab-1",
			Name:           "org-lab",
			IsActive:       true,
			Visibility:     models.LabVisibilityOrganization,
			OrganizationID: &orgID,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:                    "user-1",
			Email:                 "user@org.com",
			Roles:                 []string{"student"},
			DefaultOrganizationID: "org-123",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("filter by visibility", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-1",
			Name:       "global-lab",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs?visibility=global", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("lab with organization ID in response", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		orgID := "org-456"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:             "lab-1",
			Name:           "org-lab",
			IsActive:       true,
			Visibility:     models.LabVisibilityGlobal,
			OrganizationID: &orgID,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		labs := response["labs"].([]interface{})
		if len(labs) > 0 {
			lab := labs[0].(map[string]interface{})
			if lab["organizationId"] != "org-456" {
				t.Errorf("expected organizationId 'org-456', got '%v'", lab["organizationId"])
			}
		}
	})
}

func TestHandleGetLabWithRepo(t *testing.T) {
	t.Run("get by ID", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:          "lab-123",
			Name:        "test-lab",
			Description: "A test lab",
			Version:     "1.0.0",
			Platform:    "proxmox",
			MaxPoints:   100,
			IsActive:    true,
			Visibility:  models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["id"] != "lab-123" {
			t.Errorf("expected id 'lab-123', got '%v'", response["id"])
		}

		if response["name"] != "test-lab" {
			t.Errorf("expected name 'test-lab', got '%v'", response["name"])
		}
	})

	t.Run("get by name", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:          "lab-456",
			Name:        "nginx-lab",
			Description: "Nginx configuration lab",
			Platform:    "proxmox",
			IsActive:    true,
			Visibility:  models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/nginx-lab", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["name"] != "nginx-lab" {
			t.Errorf("expected name 'nginx-lab', got '%v'", response["name"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("include spec", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-789",
			Name:       "spec-lab",
			Spec:       []byte(`{"vms": [{"name": "vm1"}]}`),
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-789?include_spec=true", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["spec"]; !ok {
			t.Error("expected spec in response when include_spec=true")
		}
	})

	t.Run("database error on getByID", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("database error on getByName", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByNameErr = errors.New("database error")
		// No template found by ID, so it falls back to GetByName which errors

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/some-name", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("access denied - private lab without user", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-private",
			Name:       "private-lab",
			Visibility: models.LabVisibilityPrivate,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-private", nil)
		// No auth context - anonymous access
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("access denied - org lab for different org", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		orgID := "org-123"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:             "lab-org",
			Name:           "org-lab",
			Visibility:     models.LabVisibilityOrganization,
			OrganizationID: &orgID,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-org", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:                    "user-1",
			Email:                 "user@other-org.com",
			Roles:                 []string{"student"},
			DefaultOrganizationID: "different-org", // Different org
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("includes organization ID in response", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		orgID := "org-456"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:             "lab-with-org",
			Name:           "org-lab",
			Visibility:     models.LabVisibilityGlobal, // Global so anyone can access
			OrganizationID: &orgID,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-with-org", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["organizationId"] != "org-456" {
			t.Errorf("expected organizationId 'org-456', got '%v'", response["organizationId"])
		}
	})
}

func TestHandleGetLabInstructions(t *testing.T) {
	t.Run("returns instructions", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Platform:   "proxmox",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
			Spec:       []byte(`{"metadata":{"name":"test-lab","instructions":{"overview":"Test overview","learning_objectives":["obj1","obj2"],"steps":[{"title":"Step 1","content":"Do something"}]}},"spec":{"vms":[]}}`),
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-123/instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["labId"] != "lab-123" {
			t.Errorf("expected labId 'lab-123', got '%v'", response["labId"])
		}
	})

	t.Run("lab not found", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/nonexistent/instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no lab repo configured", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-123/instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-123/instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("empty labID parameter", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		// Direct call to handler with empty parameter
		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs//instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		// Should fail validation
		if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 404 or 400, got %d", rr.Code)
		}
	})

	t.Run("database error on getByName", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByNameErr = errors.New("database error")
		// No template found by ID, so it falls back to GetByName which errors

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/some-name/instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("access denied - private lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-private",
			Name:       "private-lab",
			Visibility: models.LabVisibilityPrivate,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-private/instructions", nil)
		// No auth context - anonymous access
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("parse error - invalid spec", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-invalid",
			Name:       "invalid-spec-lab",
			Visibility: models.LabVisibilityGlobal,
			Spec:       []byte(`{invalid json`), // Invalid JSON/YAML
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labs/lab-invalid/instructions", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleDeleteLab(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-to-delete",
			Name:       "delete-me",
			Platform:   "proxmox",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/labs/lab-to-delete", nil)
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		// Verify it's deleted
		record, _ := labRepo.GetByID(context.TODO(), "lab-to-delete")
		if record != nil {
			t.Error("expected lab to be deleted")
		}
	})

	t.Run("lab not found", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/labs/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no lab repo configured", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/labs/lab-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("delete error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "error-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		labRepo.DeleteErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/labs/lab-123", nil)
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("access denied - non-owner", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		otherUser := "other-user"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "owned-lab",
			CreatedBy:  &otherUser,
			Visibility: models.LabVisibilityPrivate,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/labs/lab-123", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "non-owner",
			Email: "nonowner@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error on get", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/labs/lab-123", nil)
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleSetLabActive(t *testing.T) {
	t.Run("activate lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "inactive-lab",
			Platform:   "proxmox",
			IsActive:   false,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"isActive": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123/active", strings.NewReader(body))
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		// Verify it's activated
		record, _ := labRepo.GetByID(context.TODO(), "lab-123")
		if record != nil && !record.IsActive {
			t.Error("expected lab to be active")
		}
	})

	t.Run("deactivate lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-456",
			Name:       "active-lab",
			Platform:   "proxmox",
			IsActive:   true,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"isActive": false}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-456/active", strings.NewReader(body))
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		// Verify it's deactivated
		record, _ := labRepo.GetByID(context.TODO(), "lab-456")
		if record != nil && record.IsActive {
			t.Error("expected lab to be inactive")
		}
	})

	t.Run("lab not found", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"isActive": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/nonexistent/active", strings.NewReader(body))
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no lab repo configured", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"isActive": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123/active", strings.NewReader(body))
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("set active error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "error-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		labRepo.SetActiveErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"isActive": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123/active", strings.NewReader(body))
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		body := `invalid json`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123/active", strings.NewReader(body))
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("non-admin cannot modify lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"isActive": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123/active", strings.NewReader(body))
		// Add non-admin user context
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "user@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("database error on getByID", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"isActive": true}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123/active", strings.NewReader(body))
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

// newTestServerWithLabRepo creates a test server with a lab template repository
func newTestServerWithLabRepo(t *testing.T, labRepo *mocks.FakeLabTemplateRepository) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), WithLabTemplateRepo(labRepo))
	return srv
}

func TestHandleCreateLab(t *testing.T) {
	validSpec := `apiVersion: labs/v1
metadata:
  name: test-lab
  description: Test lab
  version: 1.0.0
  duration: 60
spec:
  topology:
    vms: []
  objectives: []`

	t.Run("no lab repo configured", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"name": "test-lab", "version": "1.0.0", "platform": "proxmox", "spec": "test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"version": "1.0.0", "platform": "proxmox", "spec": "test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing version", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "platform": "proxmox", "spec": "test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing platform", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "version": "1.0.0", "spec": "test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing spec", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "version": "1.0.0", "platform": "proxmox"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("invalid spec", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "version": "1.0.0", "platform": "proxmox", "spec": "invalid yaml ]["}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("non-admin cannot create global lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "version": "1.0.0", "platform": "proxmox", "spec": "` + strings.ReplaceAll(validSpec, "\n", "\\n") + `", "visibility": "global"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add non-admin user
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "user@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("success - admin creates lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "version": "1.0.0", "platform": "proxmox", "spec": "` + strings.ReplaceAll(validSpec, "\n", "\\n") + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add admin user
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}
	})

	t.Run("database create error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.CreateErr = errors.New("database error")
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "test-lab", "version": "1.0.0", "platform": "proxmox", "spec": "` + strings.ReplaceAll(validSpec, "\n", "\\n") + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/labs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add admin user
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleUpdateLab(t *testing.T) {
	t.Run("no lab repo configured", func(t *testing.T) {
		srv := newTestServer(t)

		body := `{"name": "updated-name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("lab not found", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "updated-name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/nonexistent", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied - not owner", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		otherUser := "other-user"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			CreatedBy:  &otherUser,
			Visibility: models.LabVisibilityPrivate,
		})
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "updated-name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add non-admin user who doesn't own the lab
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "different-user",
			Email: "different@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		srv := newTestServerWithLabRepo(t, labRepo)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("non-admin cannot set global visibility", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		userId := "user-123"
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			CreatedBy:  &userId,
			Visibility: models.LabVisibilityPrivate,
		})
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"visibility": "global"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add non-admin user who owns the lab
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    userId,
			Email: "user@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("success - admin updates lab", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "updated-name", "description": "Updated description"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database update error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		labRepo.UpdateErr = errors.New("database error")
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "updated-name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Add admin context
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("database get error", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.GetByIDErr = errors.New("database error")
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"name": "updated-name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("update with invalid spec", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{"spec": "invalid yaml {"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("update all fields", func(t *testing.T) {
		labRepo := mocks.NewFakeLabTemplateRepository()
		labRepo.AddTemplate(&models.LabTemplateRecord{
			ID:         "lab-123",
			Name:       "test-lab",
			Visibility: models.LabVisibilityGlobal,
		})
		srv := newTestServerWithLabRepo(t, labRepo)

		body := `{
			"name": "updated-lab",
			"description": "Updated description",
			"version": "2.0.0",
			"platform": "cloudstack",
			"durationMinutes": 120,
			"difficulty": "advanced",
			"category": "security",
			"tags": ["new", "tags"],
			"maxPoints": 200,
			"passThreshold": 80,
			"isActive": false,
			"visibility": "organization"
		}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/labs/lab-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.ContextWithUser(req.Context(), adminUser())
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}
