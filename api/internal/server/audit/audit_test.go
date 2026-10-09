package audit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Mock Audit Repository
// -----------------------------------------------------------------------------

type mockAuditRepo struct {
	entries   []*models.AuditEntry
	queryErr  error
	createErr error
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{
		entries: []*models.AuditEntry{},
	}
}

func (r *mockAuditRepo) Create(ctx context.Context, entry *models.AuditEntry) error {
	if r.createErr != nil {
		return r.createErr
	}
	entry.ID = int64(len(r.entries) + 1)
	entry.Timestamp = time.Now()
	r.entries = append(r.entries, entry)
	return nil
}

func (r *mockAuditRepo) Query(ctx context.Context, filter repositories.AuditFilter) ([]*models.AuditEntry, error) {
	if r.queryErr != nil {
		return nil, r.queryErr
	}

	var result []*models.AuditEntry
	for _, e := range r.entries {
		// Apply filters
		if filter.ActorID != "" && e.ActorID != filter.ActorID {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.ResourceType != "" && e.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != "" && e.ResourceID != filter.ResourceID {
			continue
		}
		if filter.StartTime != nil && e.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && e.Timestamp.After(*filter.EndTime) {
			continue
		}
		result = append(result, e)
	}

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	} else if filter.Offset >= len(result) {
		result = []*models.AuditEntry{}
	}

	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

// -----------------------------------------------------------------------------
// Mock Response Writer
// -----------------------------------------------------------------------------

type mockResponseWriter struct {
	lastStatus int
	lastData   any
}

func (rw *mockResponseWriter) JSONResponse(w http.ResponseWriter, status int, data any) {
	rw.lastStatus = status
	rw.lastData = data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (rw *mockResponseWriter) ErrorResponse(w http.ResponseWriter, status int, message string) {
	rw.JSONResponse(w, status, map[string]string{"error": message})
}

func (rw *mockResponseWriter) LocalizedErrorResponse(_ context.Context, w http.ResponseWriter, status int, messageID string, _ map[string]any) {
	// Tests don't carry a localizer; fall back to writing the message ID so
	// assertions on lastStatus / lastData stay deterministic.
	rw.ErrorResponse(w, status, messageID)
}

// -----------------------------------------------------------------------------
// Test Helper: Create Tenant Context for Audit Tests
// -----------------------------------------------------------------------------

func createAuditTenantContext(orgID string) *models.TenantContext {
	return &models.TenantContext{
		Organization: &models.Organization{
			ID:       orgID,
			Name:     "Test Organization",
			Slug:     "test-org",
			Edition:  models.EditionProfessional,
			IsActive: true,
		},
		Membership: &models.OrganizationMembership{
			OrganizationID: orgID,
			UserID:         "user-123",
			Role:           models.OrgRoleAdmin,
		},
		Features: map[string]bool{
			"audit": true,
		},
	}
}

func withAuditTenantContext(req *http.Request, tc *models.TenantContext) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.TenantContextKey{}, tc)
	return req.WithContext(ctx)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// -----------------------------------------------------------------------------
// GetClientIP Tests
// -----------------------------------------------------------------------------

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "remote addr only",
			remoteAddr: "192.168.1.100:54321",
			headers:    nil,
			expected:   "192.168.1.100",
		},
		{
			name:       "remote addr without port",
			remoteAddr: "192.168.1.100",
			headers:    nil,
			expected:   "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For ignored (spoofable)",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			expected: "10.0.0.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.100",
			},
			expected: "192.168.1.100",
		},
		{
			name:       "X-Real-IP takes precedence over X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
				"X-Real-IP":       "10.10.10.10",
			},
			expected: "10.10.10.10",
		},
		{
			name:       "IPv6 address",
			remoteAddr: "[::1]:54321",
			headers:    nil,
			expected:   "[::1]",
		},
		{
			name:       "empty X-Forwarded-For falls back to RemoteAddr",
			remoteAddr: "192.168.1.100:54321",
			headers: map[string]string{
				"X-Forwarded-For": "",
			},
			expected: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := GetClientIP(req)
			if result != tt.expected {
				t.Errorf("GetClientIP() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// ListResponse Tests
// -----------------------------------------------------------------------------

func TestListResponse(t *testing.T) {
	response := ListResponse{
		Entries:    nil,
		TotalCount: 0,
		Limit:      50,
		Offset:     0,
	}

	if response.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", response.Limit)
	}
	if response.TotalCount != 0 {
		t.Errorf("expected total count 0, got %d", response.TotalCount)
	}
}

// -----------------------------------------------------------------------------
// ExportRequest Tests
// -----------------------------------------------------------------------------

func TestExportRequest(t *testing.T) {
	req := ExportRequest{
		Format: "json",
	}

	if req.Format != "json" {
		t.Errorf("expected format 'json', got %q", req.Format)
	}
	if req.StartTime != nil {
		t.Error("expected nil StartTime")
	}
	if req.EndTime != nil {
		t.Error("expected nil EndTime")
	}
}

// -----------------------------------------------------------------------------
// HandleList Tests
// -----------------------------------------------------------------------------

func TestHandleList(t *testing.T) {
	t.Run("forbidden without tenant context", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("service unavailable without audit repo", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}
	})

	t.Run("success returns entries", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		mockRepo.entries = []*models.AuditEntry{
			{
				ID:           1,
				Timestamp:    time.Now(),
				ActorID:      "user-123",
				ActorType:    "user",
				Action:       "user.create",
				ResourceType: "user",
				ResourceID:   "user-456",
			},
			{
				ID:           2,
				Timestamp:    time.Now(),
				ActorID:      "user-123",
				ActorType:    "user",
				Action:       "user.update",
				ResourceType: "user",
				ResourceID:   "user-456",
			},
		}
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid start_time format returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?start_time=invalid", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("invalid end_time format returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?end_time=invalid", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("invalid limit returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?limit=invalid", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("invalid offset returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?offset=invalid", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("repo query error returns 500", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		mockRepo.queryErr = context.DeadlineExceeded
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("limit is capped at 500", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?limit=1000", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var response ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response.Limit != 500 {
			t.Errorf("expected limit to be capped at 500, got %d", response.Limit)
		}
	})

	t.Run("negative limit returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?limit=-1", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("negative offset returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/?offset=-1", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleList(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// HandleGet Tests
// -----------------------------------------------------------------------------

func TestHandleGet(t *testing.T) {
	t.Run("forbidden without tenant context", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/entry-123", nil)
		rec := httptest.NewRecorder()

		svc.HandleGet(rw)(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("service unavailable without audit repo", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/entry-123", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleGet(rw)(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}
	})

	t.Run("not found when entry doesn't exist", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/entry-123", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleGet(rw)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("repo query error returns 500", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		mockRepo.queryErr = context.DeadlineExceeded
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		req := httptest.NewRequest(http.MethodGet, "/entry-123", nil)
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleGet(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// HandleExport Tests
// -----------------------------------------------------------------------------

func TestHandleExport(t *testing.T) {
	t.Run("forbidden without tenant context", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())

		body := strings.NewReader(`{"format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("service unavailable without audit repo", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())
		tc := createAuditTenantContext("org-123")

		body := strings.NewReader(`{"format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}
	})

	t.Run("invalid request body returns error", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		body := strings.NewReader(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("success returns exported entries", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		mockRepo.entries = []*models.AuditEntry{
			{
				ID:           1,
				Timestamp:    time.Now(),
				ActorID:      "user-123",
				ActorType:    "user",
				Action:       "user.create",
				ResourceType: "user",
				ResourceID:   "user-456",
			},
		}
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		body := strings.NewReader(`{"format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("repo query error returns 500", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		mockRepo.queryErr = context.DeadlineExceeded
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		body := strings.NewReader(`{"format": "json"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("csv format returns CSV with entries", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		ts := time.Now()
		mockRepo.entries = []*models.AuditEntry{
			{
				ID:           1,
				Timestamp:    ts,
				ActorID:      "user-123",
				ActorType:    "user",
				Action:       "user.create",
				ResourceType: "user",
				ResourceID:   "user-456",
				IPAddress:    "10.0.0.1",
				UserAgent:    "TestAgent/1.0",
			},
		}
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		body := strings.NewReader(`{"format": "csv"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "text/csv" {
			t.Errorf("expected Content-Type 'text/csv', got %q", contentType)
		}

		disposition := rec.Header().Get("Content-Disposition")
		if disposition != "attachment; filename=audit-export.csv" {
			t.Errorf("expected Content-Disposition attachment, got %q", disposition)
		}

		csvBody := rec.Body.String()
		if !strings.Contains(csvBody, "id,timestamp,actorId") {
			t.Error("expected CSV header row")
		}
		if !strings.Contains(csvBody, "user-123") {
			t.Error("expected CSV to contain actor ID")
		}
		if !strings.Contains(csvBody, "user.create") {
			t.Error("expected CSV to contain action")
		}
	})

	t.Run("csv format with empty results", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		rw := &mockResponseWriter{}
		svc := NewService(mockRepo, newTestLogger())
		tc := createAuditTenantContext("org-123")

		body := strings.NewReader(`{"format": "csv"}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		req = withAuditTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleExport(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		csvBody := rec.Body.String()
		// Should still have the header row
		if !strings.Contains(csvBody, "id,timestamp,actorId") {
			t.Error("expected CSV header row even with no entries")
		}

		// Count lines: header + newline only
		lines := strings.Split(strings.TrimSpace(csvBody), "\n")
		if len(lines) != 1 {
			t.Errorf("expected 1 line (header only), got %d", len(lines))
		}
	})
}

// -----------------------------------------------------------------------------
// LogAction Tests
// -----------------------------------------------------------------------------

func TestLogAction(t *testing.T) {
	t.Run("no-op when repo is nil", func(t *testing.T) {
		svc := NewService(nil, newTestLogger())
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		// Should not panic
		svc.LogAction(req, "test.action", "test_resource", "resource-123", map[string]any{
			"key": "value",
		})
	})

	t.Run("creates audit entry successfully", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		svc := NewService(mockRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", "Test-Agent/1.0")
		req.RemoteAddr = "192.168.1.100:54321"

		svc.LogAction(req, "user.create", "user", "user-123", map[string]any{
			"email": "test@example.com",
		})

		if len(mockRepo.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(mockRepo.entries))
		}

		entry := mockRepo.entries[0]
		if entry.Action != "user.create" {
			t.Errorf("expected action 'user.create', got '%s'", entry.Action)
		}
		if entry.ResourceType != "user" {
			t.Errorf("expected resource type 'user', got '%s'", entry.ResourceType)
		}
		if entry.ResourceID != "user-123" {
			t.Errorf("expected resource ID 'user-123', got '%s'", entry.ResourceID)
		}
		if entry.IPAddress != "192.168.1.100" {
			t.Errorf("expected IP '192.168.1.100', got '%s'", entry.IPAddress)
		}
		if entry.UserAgent != "Test-Agent/1.0" {
			t.Errorf("expected User-Agent 'Test-Agent/1.0', got '%s'", entry.UserAgent)
		}
	})

	t.Run("handles create error gracefully", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		mockRepo.createErr = context.DeadlineExceeded
		svc := NewService(mockRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		// Should not panic
		svc.LogAction(req, "user.delete", "user", "user-123", nil)

		// Entry should not be added due to error
		if len(mockRepo.entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(mockRepo.entries))
		}
	})

	t.Run("handles nil details", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		svc := NewService(mockRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		svc.LogAction(req, "user.delete", "user", "user-123", nil)

		if len(mockRepo.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(mockRepo.entries))
		}

		// Details should be nil/empty
		if mockRepo.entries[0].Details != nil {
			t.Errorf("expected nil details, got %v", mockRepo.entries[0].Details)
		}
	})

	t.Run("extracts IP from X-Real-IP header (ignores X-Forwarded-For)", func(t *testing.T) {
		mockRepo := newMockAuditRepo()
		svc := NewService(mockRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
		req.Header.Set("X-Real-IP", "10.0.0.1")
		req.RemoteAddr = "10.0.0.1:54321"

		svc.LogAction(req, "user.create", "user", "user-123", nil)

		if len(mockRepo.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(mockRepo.entries))
		}

		// Should use X-Real-IP, not X-Forwarded-For
		if mockRepo.entries[0].IPAddress != "10.0.0.1" {
			t.Errorf("expected IP '10.0.0.1', got '%s'", mockRepo.entries[0].IPAddress)
		}
	})
}

// Unused variable to quiet linter for os import
var _ = os.Stdout
