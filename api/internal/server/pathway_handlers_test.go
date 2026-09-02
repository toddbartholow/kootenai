package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// -----------------------------------------------------------------------------
// List Pathways Tests
// -----------------------------------------------------------------------------

func TestHandleListPathways(t *testing.T) {
	t.Run("returns pathways", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:          "pathway-1",
			Name:        "Network Fundamentals",
			Slug:        "network-fundamentals",
			Description: "Learn networking basics",
			Difficulty:  "beginner",
			Status:      models.PathwayStatusPublished,
		})
		pathwayRepo.AddPathway(&models.Pathway{
			ID:          "pathway-2",
			Name:        "Advanced Security",
			Slug:        "advanced-security",
			Description: "Advanced security topics",
			Difficulty:  "advanced",
			Status:      models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways", nil)
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
	})

	t.Run("empty list", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways", nil)
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

	t.Run("filter by difficulty", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:         "pathway-1",
			Name:       "Beginner Path",
			Slug:       "beginner-path",
			Difficulty: "beginner",
			Status:     models.PathwayStatusPublished,
		})
		pathwayRepo.AddPathway(&models.Pathway{
			ID:         "pathway-2",
			Name:       "Advanced Path",
			Slug:       "advanced-path",
			Difficulty: "advanced",
			Status:     models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways?difficulty=beginner", nil)
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
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.ListErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("filter featured pathways", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:         "pathway-1",
			Name:       "Featured Path",
			Slug:       "featured-path",
			Status:     models.PathwayStatusPublished,
			IsFeatured: true,
		})
		pathwayRepo.AddPathway(&models.Pathway{
			ID:         "pathway-2",
			Name:       "Regular Path",
			Slug:       "regular-path",
			Status:     models.PathwayStatusPublished,
			IsFeatured: false,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways?featured=true", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("with limit and offset", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:     "pathway-1",
			Name:   "Path 1",
			Slug:   "path-1",
			Status: models.PathwayStatusPublished,
		})
		pathwayRepo.AddPathway(&models.Pathway{
			ID:     "pathway-2",
			Name:   "Path 2",
			Slug:   "path-2",
			Status: models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways?limit=10&offset=0", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("with search query", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:     "pathway-1",
			Name:   "Network Fundamentals",
			Slug:   "network-fundamentals",
			Status: models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways?search=network", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("with visibility filter", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:         "pathway-1",
			Name:       "Global Path",
			Slug:       "global-path",
			Status:     models.PathwayStatusPublished,
			Visibility: models.LabVisibilityGlobal,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways?visibility=global", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Get Pathway Tests
// -----------------------------------------------------------------------------

func TestHandleGetPathway(t *testing.T) {
	t.Run("get by ID", func(t *testing.T) {
		pathwayID := "00000000-0000-0000-0000-000000000123"
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:          pathwayID,
			Name:        "Test Pathway",
			Slug:        "test-pathway",
			Description: "A test pathway",
			Difficulty:  "intermediate",
			Status:      models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/"+pathwayID, nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["id"] != pathwayID {
			t.Errorf("expected id '%s', got '%v'", pathwayID, response["id"])
		}

		if response["name"] != "Test Pathway" {
			t.Errorf("expected name 'Test Pathway', got '%v'", response["name"])
		}
	})

	t.Run("get by slug", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:          "pathway-456",
			Name:        "Slug Pathway",
			Slug:        "slug-pathway",
			Description: "Accessed via slug",
			Status:      models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/slug-pathway", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["slug"] != "slug-pathway" {
			t.Errorf("expected slug 'slug-pathway', got '%v'", response["slug"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.GetWithModulesErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		// Use a UUID format to trigger the ID-based lookup path
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/00000000-0000-0000-0000-000000000001", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Create Pathway Tests
// -----------------------------------------------------------------------------

func TestHandleCreatePathway(t *testing.T) {
	t.Run("creates pathway successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name":        "New Pathway",
			"description": "A new learning pathway",
			"difficulty":  "beginner",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["name"] != "New Pathway" {
			t.Errorf("expected name 'New Pathway', got '%v'", response["name"])
		}

		if response["slug"] != "new-pathway" {
			t.Errorf("expected auto-generated slug 'new-pathway', got '%v'", response["slug"])
		}

		if response["status"] != "draft" {
			t.Errorf("expected status 'draft', got '%v'", response["status"])
		}
	})

	t.Run("missing name", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"description": "No name provided",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("duplicate slug", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "existing-pathway",
			Name: "Existing Pathway",
			Slug: "existing-pathway",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "Existing Pathway", // Will generate same slug
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.CreateErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "Test Pathway",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Update Pathway Tests
// -----------------------------------------------------------------------------

func TestHandleUpdatePathway(t *testing.T) {
	t.Run("updates pathway successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:          "pathway-123",
			Name:        "Original Name",
			Slug:        "original-name",
			Description: "Original description",
			Status:      models.PathwayStatusDraft,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		newName := "Updated Name"
		body := map[string]interface{}{
			"name": newName,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/pathways/pathway-123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["name"] != newName {
			t.Errorf("expected name '%s', got '%v'", newName, response["name"])
		}
	})

	t.Run("pathway not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "New Name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/pathways/nonexistent", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "Test",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/pathways/pathway-123", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("database error on update", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "Test Pathway",
		})
		pathwayRepo.UpdateErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "Updated Name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/pathways/pathway-123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("database error on get", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "Updated Name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/pathways/pathway-123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Delete Pathway Tests
// -----------------------------------------------------------------------------

func TestHandleDeletePathway(t *testing.T) {
	t.Run("deletes pathway successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "To Delete",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/pathway-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("pathway not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("database error on delete", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "Test",
		})
		pathwayRepo.DeleteErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/pathway-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("database error on get", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/pathway-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Publish/Archive Pathway Tests
// -----------------------------------------------------------------------------

func TestHandlePublishPathway(t *testing.T) {
	t.Run("publishes pathway successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:     "pathway-123",
			Name:   "Draft Pathway",
			Status: models.PathwayStatusDraft,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/pathway-123/publish", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.UpdateStatusErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/pathway-123/publish", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleArchivePathway(t *testing.T) {
	t.Run("archives pathway successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:     "pathway-123",
			Name:   "Published Pathway",
			Status: models.PathwayStatusPublished,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/pathway-123/archive", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.UpdateStatusErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/pathway-123/archive", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Get Pathway Stats Tests
// -----------------------------------------------------------------------------

func TestHandleGetPathwayStats(t *testing.T) {
	t.Run("returns stats", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "Stats Pathway",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/pathway-123/stats", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["pathwayId"] != "pathway-123" {
			t.Errorf("expected pathwayId 'pathway-123', got '%v'", response["pathwayId"])
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.GetStatsErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/pathway-123/stats", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Module Handlers Tests
// -----------------------------------------------------------------------------

func TestHandleListModules(t *testing.T) {
	t.Run("returns modules", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "Test Pathway",
		})
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-1",
			PathwayID: "pathway-123",
			Name:      "Module 1",
			Slug:      "module-1",
		})
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-2",
			PathwayID: "pathway-123",
			Name:      "Module 2",
			Slug:      "module-2",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/pathway-123/modules", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count := response["count"].(float64)
		if int(count) != 2 {
			t.Errorf("expected count 2, got %d", int(count))
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.ListModulesErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pathways/pathway-123/modules", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleCreateModule(t *testing.T) {
	t.Run("creates module successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   "pathway-123",
			Name: "Test Pathway",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name":        "New Module",
			"description": "A new module",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/pathway-123/modules", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["name"] != "New Module" {
			t.Errorf("expected name 'New Module', got '%v'", response["name"])
		}

		if response["pathwayId"] != "pathway-123" {
			t.Errorf("expected pathwayId 'pathway-123', got '%v'", response["pathwayId"])
		}
	})

	t.Run("missing name", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"description": "No name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/pathway-123/modules", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

func TestHandleUpdateModule(t *testing.T) {
	t.Run("updates module successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-123",
			PathwayID: "pathway-1",
			Name:      "Original Name",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		newName := "Updated Name"
		body := map[string]interface{}{
			"name": newName,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/modules/module-123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("module not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "New Name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/modules/nonexistent", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-123",
			PathwayID: "pathway-1",
			Name:      "Test",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/modules/module-123", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("database error on update", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-123",
			PathwayID: "pathway-1",
			Name:      "Test",
		})
		pathwayRepo.UpdateModuleErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "Updated Name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/modules/module-123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("database error on get", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.GetModuleByIDErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := map[string]interface{}{
			"name": "Updated Name",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/modules/module-123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleDeleteModule(t *testing.T) {
	t.Run("deletes module successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-123",
			PathwayID: "pathway-1",
			Name:      "To Delete",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/modules/module-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.DeleteModuleErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/modules/module-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleAddLabToModule(t *testing.T) {
	t.Run("adds lab to module successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-123",
			PathwayID: "pathway-1",
			Name:      "Test Module",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := `{"labTemplateId": "lab-template-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/modules/module-123/labs", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := `{invalid json`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/modules/module-123/labs", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing labTemplateId", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := `{"displayOrder": 1}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/modules/module-123/labs", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("with explicit displayOrder", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddModule(&models.PathwayModule{
			ID:        "module-123",
			PathwayID: "pathway-1",
			Name:      "Test Module",
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := `{"labTemplateId": "lab-template-1", "displayOrder": 5, "isRequired": false}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/modules/module-123/labs", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["displayOrder"].(float64) != 5 {
			t.Errorf("expected displayOrder 5, got %v", response["displayOrder"])
		}
		if response["isRequired"].(bool) != false {
			t.Errorf("expected isRequired false, got %v", response["isRequired"])
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddLabToModuleErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		body := `{"labTemplateId": "lab-template-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/modules/module-123/labs", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleRemoveLabFromModule(t *testing.T) {
	t.Run("removes lab from module successfully", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/modules/module-123/labs/lab-template-1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.RemoveLabFromModuleErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/modules/module-123/labs/lab-template-1", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Enrollment Handlers Tests
// -----------------------------------------------------------------------------

func TestHandleEnrollInPathway(t *testing.T) {
	t.Run("enrolls user successfully", func(t *testing.T) {
		pathwayID := "00000000-0000-0000-0000-000000000123"
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:     pathwayID,
			Name:   "Test Pathway",
			Slug:   "test-pathway",
			Status: models.PathwayStatusPublished,
		})

		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/"+pathwayID+"/enroll", nil)
		// Add user to context
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}
	})

	t.Run("already enrolled returns existing", func(t *testing.T) {
		pathwayID := "00000000-0000-0000-0000-000000000123"
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   pathwayID,
			Name: "Test Pathway",
			Slug: "test-pathway",
		})

		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.AddEnrollment(&models.PathwayEnrollment{
			ID:        "enrollment-existing",
			UserID:    "user-123",
			PathwayID: pathwayID,
			Status:    models.EnrollmentStatusEnrolled,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/"+pathwayID+"/enroll", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["id"] != "enrollment-existing" {
			t.Errorf("expected existing enrollment id, got '%v'", response["id"])
		}
	})

	t.Run("pathway not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/nonexistent/enroll", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("unauthorized without user context", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pathways/00000000-0000-0000-0000-000000000123/enroll", nil)
		// No user context added
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})
}

func TestHandleUnenrollFromPathway(t *testing.T) {
	t.Run("unenrolls user successfully", func(t *testing.T) {
		pathwayID := "00000000-0000-0000-0000-000000000123"
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   pathwayID,
			Name: "Test Pathway",
			Slug: "test-pathway",
		})

		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.AddEnrollment(&models.PathwayEnrollment{
			ID:        "enrollment-123",
			UserID:    "user-123",
			PathwayID: pathwayID,
			Status:    models.EnrollmentStatusEnrolled,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/"+pathwayID+"/enroll", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("not enrolled", func(t *testing.T) {
		pathwayID := "00000000-0000-0000-0000-000000000123"
		pathwayRepo := mocks.NewFakePathwayRepository()
		pathwayRepo.AddPathway(&models.Pathway{
			ID:   pathwayID,
			Name: "Test Pathway",
			Slug: "test-pathway",
		})

		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/"+pathwayID+"/enroll", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/pathway-123/enroll", nil)
		// No user context
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("pathway not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pathways/nonexistent/enroll", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

func TestHandleListEnrollments(t *testing.T) {
	t.Run("returns user enrollments", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.AddEnrollment(&models.PathwayEnrollment{
			ID:        "enrollment-1",
			UserID:    "user-123",
			PathwayID: "pathway-1",
			Status:    models.EnrollmentStatusEnrolled,
		})
		enrollmentRepo.AddEnrollment(&models.PathwayEnrollment{
			ID:        "enrollment-2",
			UserID:    "user-123",
			PathwayID: "pathway-2",
			Status:    models.EnrollmentStatusCompleted,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count := response["count"].(float64)
		if int(count) != 2 {
			t.Errorf("expected count 2, got %d", int(count))
		}
	})

	t.Run("unauthorized without user", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.ListErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleGetEnrollment(t *testing.T) {
	t.Run("returns enrollment", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.AddEnrollment(&models.PathwayEnrollment{
			ID:        "enrollment-123",
			UserID:    "user-123",
			PathwayID: "pathway-1",
			Status:    models.EnrollmentStatusEnrolled,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments/enrollment-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["id"] != "enrollment-123" {
			t.Errorf("expected id 'enrollment-123', got '%v'", response["id"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.GetByIDErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments/enrollment-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandleGetEnrollmentProgress(t *testing.T) {
	t.Run("returns enrollment with progress", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.AddEnrollment(&models.PathwayEnrollment{
			ID:               "enrollment-123",
			UserID:           "user-123",
			PathwayID:        "pathway-1",
			Status:           models.EnrollmentStatusInProgress,
			CompletedModules: 2,
			TotalModules:     5,
			EarnedPoints:     150,
			MaxPoints:        500,
			Percentage:       30.0,
		})

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments/enrollment-123/progress", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["completedModules"].(float64) != 2 {
			t.Errorf("expected completedModules 2, got '%v'", response["completedModules"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments/nonexistent/progress", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("database error", func(t *testing.T) {
		pathwayRepo := mocks.NewFakePathwayRepository()
		enrollmentRepo := mocks.NewFakeEnrollmentRepository()
		enrollmentRepo.GetWithProgressErr = errors.New("database error")

		srv := newTestServerWithPathwayRepo(t, pathwayRepo, enrollmentRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments/enrollment-123/progress", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Utility Function Tests
// -----------------------------------------------------------------------------

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		wantID string
		wantOK bool
	}{
		{
			name:   "no user in context",
			ctx:    context.Background(),
			wantID: "",
			wantOK: false,
		},
		{
			name: "user in context",
			ctx: auth.ContextWithUser(context.Background(), &auth.User{
				ID:    "user-123",
				Email: "test@example.com",
			}),
			wantID: "user-123",
			wantOK: true,
		},
		{
			name:   "nil user in context",
			ctx:    auth.ContextWithUser(context.Background(), nil),
			wantID: "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := getUserIDFromContext(tt.ctx)
			if gotID != tt.wantID {
				t.Errorf("getUserIDFromContext() ID = %v, want %v", gotID, tt.wantID)
			}
			if gotOK != tt.wantOK {
				t.Errorf("getUserIDFromContext() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

// newTestServerWithPathwayRepo creates a test server with pathway and enrollment repositories
func newTestServerWithPathwayRepo(t *testing.T, pathwayRepo *mocks.FakePathwayRepository, enrollmentRepo *mocks.FakeEnrollmentRepository) *Server {
	t.Helper()
	cfg := DefaultConfig()
	cfg.CORSOrigins = []string{"http://localhost:3000"}
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	opts := []ServerOption{WithPathwayRepo(pathwayRepo)}
	if enrollmentRepo != nil {
		opts = append(opts, WithEnrollmentRepo(enrollmentRepo))
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)
	return srv
}
