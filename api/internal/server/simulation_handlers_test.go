package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/simulation"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// mockSimulationService implements SimulationService for testing
type mockSimulationService struct {
	students         []*models.User
	createdStudent   *models.User
	progress         map[string]interface{}
	simulationResult *simulation.SimulationResult
	listErr          error
	createErr        error
	simulateErr      error
	getProgressErr   error
	resetErr         error
}

func (m *mockSimulationService) ListTestStudents(ctx context.Context) ([]*models.User, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.students, nil
}

func (m *mockSimulationService) CreateTestStudent(ctx context.Context, name, email string) (*models.User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createdStudent != nil {
		return m.createdStudent, nil
	}
	user := &models.User{
		ID:          "test-student-" + name,
		DisplayName: name,
		Email:       email,
		CreatedAt:   time.Now(),
	}
	m.students = append(m.students, user)
	return user, nil
}

func (m *mockSimulationService) SimulatePathwayProgression(ctx context.Context, userID, pathwayID string, config simulation.SimulationConfig) (*simulation.SimulationResult, error) {
	if m.simulateErr != nil {
		return nil, m.simulateErr
	}
	if m.simulationResult != nil {
		return m.simulationResult, nil
	}
	return &simulation.SimulationResult{
		StudentID:        userID,
		StudentName:      "Test Student",
		PathwayID:        pathwayID,
		PathwayName:      "Test Pathway",
		EnrollmentID:     "enrollment-123",
		Status:           "completed",
		ModulesCompleted: 3,
		TotalModules:     3,
		EarnedPoints:     100,
		MaxPoints:        100,
		Percentage:       100.0,
	}, nil
}

func (m *mockSimulationService) GetStudentProgress(ctx context.Context, userID string) (map[string]interface{}, error) {
	if m.getProgressErr != nil {
		return nil, m.getProgressErr
	}
	if m.progress != nil {
		return m.progress, nil
	}
	return map[string]interface{}{
		"user":         map[string]string{"id": userID, "name": "Test", "email": "test@example.com"},
		"enrollments":  []interface{}{},
		"achievements": []interface{}{},
		"labSummary":   map[string]interface{}{"totalLabs": 0, "passedLabs": 0},
	}, nil
}

func (m *mockSimulationService) ResetStudentProgress(ctx context.Context, userID string) error {
	return m.resetErr
}

// simTestLogger creates a test logger for simulation tests
func simTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newSimMgrForTest creates a SimulationManager with the given mock (nil-safe).
func newSimMgrForTest(mock SimulationService) *SimulationManager {
	return NewSimulationManager(SimulationManagerConfig{
		SimulationService: mock,
		Logger:            simTestLogger(),
	})
}

// -----------------------------------------------------------------------------
// handleListTestStudents Tests
// -----------------------------------------------------------------------------

func TestHandleListTestStudents(t *testing.T) {
	t.Run("simulation service not configured", func(t *testing.T) {
		mgr := newSimMgrForTest(nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students", nil)
		rr := httptest.NewRecorder()

		mgr.handleListTestStudents(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.serviceNotConfigured") {
			t.Errorf("expected error message about simulation service, got %q", rr.Body.String())
		}
	})

	t.Run("list error", func(t *testing.T) {
		mock := &mockSimulationService{
			listErr: errors.New("database error"),
		}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students", nil)
		rr := httptest.NewRecorder()

		mgr.handleListTestStudents(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.listTestStudentsFailed") {
			t.Errorf("expected error message, got %q", rr.Body.String())
		}
	})

	t.Run("success with students", func(t *testing.T) {
		mock := &mockSimulationService{
			students: []*models.User{
				{ID: "user-1", DisplayName: "Test Student 1", Email: "test1@example.com"},
				{ID: "user-2", DisplayName: "Test Student 2", Email: "test2@example.com"},
			},
		}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students", nil)
		rr := httptest.NewRecorder()

		mgr.handleListTestStudents(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"count":2`) {
			t.Errorf("expected count:2, got %q", rr.Body.String())
		}
	})

	t.Run("success with empty list", func(t *testing.T) {
		mock := &mockSimulationService{
			students: []*models.User{},
		}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students", nil)
		rr := httptest.NewRecorder()

		mgr.handleListTestStudents(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"count":0`) {
			t.Errorf("expected count:0, got %q", rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleCreateTestStudent Tests
// -----------------------------------------------------------------------------

func TestHandleCreateTestStudent(t *testing.T) {
	t.Run("simulation service not configured", func(t *testing.T) {
		mgr := newSimMgrForTest(nil)

		body := `{"name": "Test", "email": "test@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleCreateTestStudent(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", strings.NewReader(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleCreateTestStudent(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "error.invalidRequestBody") {
			t.Errorf("expected invalid request body error, got %q", rr.Body.String())
		}
	})

	t.Run("missing name", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		body := `{"name": "", "email": "test@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleCreateTestStudent(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.nameRequired") {
			t.Errorf("expected name required error, got %q", rr.Body.String())
		}
	})

	t.Run("missing email", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		body := `{"name": "Test Student", "email": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleCreateTestStudent(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.emailRequired") {
			t.Errorf("expected email required error, got %q", rr.Body.String())
		}
	})

	t.Run("create error", func(t *testing.T) {
		mock := &mockSimulationService{
			createErr: errors.New("database error"),
		}
		mgr := newSimMgrForTest(mock)

		body := `{"name": "Test Student", "email": "test@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleCreateTestStudent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.createTestStudentFailed") {
			t.Errorf("expected create error message, got %q", rr.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		mock := &mockSimulationService{
			createdStudent: &models.User{
				ID:          "user-123",
				DisplayName: "Test Student",
				Email:       "test@example.com",
				Role:        "student",
				IsActive:    true,
				CreatedAt:   time.Now(),
			},
		}
		mgr := newSimMgrForTest(mock)

		body := `{"name": "Test Student", "email": "test@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleCreateTestStudent(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"id":"user-123"`) {
			t.Errorf("expected user ID in response, got %q", rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleSimulateProgress Tests
// -----------------------------------------------------------------------------

func TestHandleSimulateProgress(t *testing.T) {
	t.Run("simulation service not configured", func(t *testing.T) {
		mgr := newSimMgrForTest(nil)

		body := `{"userId": "user-1", "pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing userId", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		body := `{"userId": "", "pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.userIdRequired") {
			t.Errorf("expected userId required error, got %q", rr.Body.String())
		}
	})

	t.Run("missing pathwayId", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		body := `{"userId": "user-1", "pathwayId": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.pathwayIdRequired") {
			t.Errorf("expected pathwayId required error, got %q", rr.Body.String())
		}
	})

	t.Run("simulation error", func(t *testing.T) {
		mock := &mockSimulationService{
			simulateErr: errors.New("simulation failed"),
		}
		mgr := newSimMgrForTest(mock)

		body := `{"userId": "user-1", "pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("success with default profile", func(t *testing.T) {
		mock := &mockSimulationService{
			simulationResult: &simulation.SimulationResult{
				StudentID:        "user-1",
				StudentName:      "Test Student",
				PathwayID:        "pathway-1",
				PathwayName:      "Test Pathway",
				Status:           "completed",
				ModulesCompleted: 5,
				TotalModules:     5,
				Percentage:       100,
			},
		}
		mgr := newSimMgrForTest(mock)

		body := `{"userId": "user-1", "pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"status":"completed"`) {
			t.Errorf("expected completed status, got %q", rr.Body.String())
		}
	})

	t.Run("success with custom profile", func(t *testing.T) {
		mock := &mockSimulationService{
			simulationResult: &simulation.SimulationResult{
				StudentID:  "user-1",
				PathwayID:  "pathway-1",
				Status:     "in_progress",
				Percentage: 75,
			},
		}
		mgr := newSimMgrForTest(mock)

		body := `{"userId": "user-1", "pathwayId": "pathway-1", "studentProfile": "struggling", "speed": 100, "modulesToComplete": 3}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleSimulateProgress(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"status":"in_progress"`) {
			t.Errorf("expected in_progress status, got %q", rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetStudentProgress Tests
// -----------------------------------------------------------------------------

func TestHandleGetStudentProgress(t *testing.T) {
	t.Run("simulation service not configured", func(t *testing.T) {
		mgr := newSimMgrForTest(nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students/user-1/progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleGetStudentProgress(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("missing userID", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students//progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleGetStudentProgress(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.userIDRequired") {
			t.Errorf("expected userID required error, got %q", rr.Body.String())
		}
	})

	t.Run("get progress error", func(t *testing.T) {
		mock := &mockSimulationService{
			getProgressErr: errors.New("database error"),
		}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students/user-1/progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleGetStudentProgress(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.getStudentProgressFailed") {
			t.Errorf("expected error message, got %q", rr.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		mock := &mockSimulationService{
			progress: map[string]interface{}{
				"user": map[string]string{
					"id":    "user-1",
					"name":  "Test Student",
					"email": "test@example.com",
				},
				"enrollments":  []interface{}{},
				"achievements": []interface{}{},
				"labSummary": map[string]interface{}{
					"totalLabs":   10,
					"passedLabs":  8,
					"totalPoints": 800,
					"maxPoints":   1000,
				},
			},
		}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students/user-1/progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleGetStudentProgress(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "totalLabs") {
			t.Errorf("expected totalLabs in response, got %q", rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleResetStudentProgress Tests
// -----------------------------------------------------------------------------

func TestHandleResetStudentProgress(t *testing.T) {
	t.Run("simulation service not configured", func(t *testing.T) {
		mgr := newSimMgrForTest(nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/simulation/students/user-1/progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleResetStudentProgress(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("missing userID", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/simulation/students//progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleResetStudentProgress(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.userIDRequired") {
			t.Errorf("expected userID required error, got %q", rr.Body.String())
		}
	})

	t.Run("reset error", func(t *testing.T) {
		mock := &mockSimulationService{
			resetErr: errors.New("database error"),
		}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/simulation/students/user-1/progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleResetStudentProgress(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.resetStudentProgressFailed") {
			t.Errorf("expected error message, got %q", rr.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/simulation/students/user-1/progress", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		mgr.handleResetStudentProgress(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Student progress reset successfully") {
			t.Errorf("expected success message, got %q", rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleQuickSimulation Tests
// -----------------------------------------------------------------------------

func TestHandleQuickSimulation(t *testing.T) {
	t.Run("simulation service not configured", func(t *testing.T) {
		mgr := newSimMgrForTest(nil)

		body := `{"pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing pathwayId", func(t *testing.T) {
		mock := &mockSimulationService{}
		mgr := newSimMgrForTest(mock)

		body := `{"pathwayId": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.pathwayIdRequired") {
			t.Errorf("expected pathwayId required error, got %q", rr.Body.String())
		}
	})

	t.Run("create student error", func(t *testing.T) {
		mock := &mockSimulationService{
			createErr: errors.New("database error"),
		}
		mgr := newSimMgrForTest(mock)

		body := `{"pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "simulation.errors.createTestStudentFailed") {
			t.Errorf("expected create error message, got %q", rr.Body.String())
		}
	})

	t.Run("simulation error", func(t *testing.T) {
		mock := &mockSimulationService{
			createdStudent: &models.User{
				ID:          "user-123",
				DisplayName: "Test Student",
				Email:       "test@simulation.local",
			},
			simulateErr: errors.New("simulation failed"),
		}
		mgr := newSimMgrForTest(mock)

		body := `{"pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("success with default profile", func(t *testing.T) {
		mock := &mockSimulationService{
			createdStudent: &models.User{
				ID:          "user-123",
				DisplayName: "Test Student",
				Email:       "test@simulation.local",
			},
			simulationResult: &simulation.SimulationResult{
				StudentID:  "user-123",
				PathwayID:  "pathway-1",
				Status:     "completed",
				Percentage: 95,
			},
		}
		mgr := newSimMgrForTest(mock)

		body := `{"pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"status":"completed"`) {
			t.Errorf("expected completed status, got %q", rr.Body.String())
		}
	})

	t.Run("success with custom profile", func(t *testing.T) {
		mock := &mockSimulationService{
			createdStudent: &models.User{
				ID:          "user-456",
				DisplayName: "Test Student",
				Email:       "test@simulation.local",
			},
			simulationResult: &simulation.SimulationResult{
				StudentID:  "user-456",
				PathwayID:  "pathway-1",
				Status:     "in_progress",
				Percentage: 60,
			},
		}
		mgr := newSimMgrForTest(mock)

		body := `{"pathwayId": "pathway-1", "studentProfile": "struggling"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mgr.handleQuickSimulation(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"studentId":"user-456"`) {
			t.Errorf("expected user-456 in response, got %q", rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// Integration Tests with Router
// -----------------------------------------------------------------------------

// newTestServerWithSimulationMock creates a test server with an injected mock simulation service
func newTestServerWithSimulationMock(t *testing.T, mock SimulationService) (*Server, string) {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	eval := checkpoint.NewEvaluator(newTestLogger())

	authSvc, err := auth.NewService(auth.Config{
		JWTSecret:     "test-secret-key-for-testing-minimum-32-chars",
		JWTExpiration: time.Hour,
	})
	require.NoError(t, err)

	opts := []ServerOption{
		WithAuthService(authSvc),
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)

	// Inject the mock simulation service into the manager (since the manager
	// is already constructed during New, we must set it on the manager directly).
	if mock != nil {
		srv.simulationMgr.simulationService = mock
	}

	// Generate token for an admin user (for simulation access)
	user := &auth.User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"admin"},
	}
	token, err := authSvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	return srv, token
}

func TestSimulationHandlersIntegration(t *testing.T) {
	t.Run("list students through router", func(t *testing.T) {
		mock := &mockSimulationService{
			students: []*models.User{
				{ID: "student-1", DisplayName: "Test Student 1"},
			},
		}
		srv, token := newTestServerWithSimulationMock(t, mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "student-1") {
			t.Errorf("expected student-1 in response, got %q", rr.Body.String())
		}
	})

	t.Run("create student through router", func(t *testing.T) {
		mock := &mockSimulationService{
			createdStudent: &models.User{
				ID:          "new-student",
				DisplayName: "New Student",
				Email:       "new@example.com",
			},
		}
		srv, token := newTestServerWithSimulationMock(t, mock)

		body := `{"name": "New Student", "email": "new@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/students", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}
	})

	t.Run("simulate progress through router", func(t *testing.T) {
		mock := &mockSimulationService{
			simulationResult: &simulation.SimulationResult{
				StudentID: "user-1",
				PathwayID: "pathway-1",
				Status:    "completed",
			},
		}
		srv, token := newTestServerWithSimulationMock(t, mock)

		body := `{"userId": "user-1", "pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/run", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("quick simulation through router", func(t *testing.T) {
		mock := &mockSimulationService{
			createdStudent: &models.User{
				ID:    "quick-student",
				Email: "quick@simulation.local",
			},
			simulationResult: &simulation.SimulationResult{
				StudentID: "quick-student",
				Status:    "completed",
			},
		}
		srv, token := newTestServerWithSimulationMock(t, mock)

		body := `{"pathwayId": "pathway-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/simulation/quick", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("get student progress through router", func(t *testing.T) {
		mock := &mockSimulationService{
			progress: map[string]interface{}{
				"user": map[string]string{"id": "user-1"},
			},
		}
		srv, token := newTestServerWithSimulationMock(t, mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/simulation/students/user-1/progress", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("reset student progress through router", func(t *testing.T) {
		mock := &mockSimulationService{}
		srv, token := newTestServerWithSimulationMock(t, mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/simulation/students/user-1/progress", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}
