package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/simulation"
)

// ---------------------------------------------------------------------------
// SimulationManager
// ---------------------------------------------------------------------------

// SimulationManagerConfig holds configuration for creating a SimulationManager.
type SimulationManagerConfig struct {
	SimulationService  SimulationService
	ClassroomRouteHook func(r chi.Router) // optional: wires classroom sub-routes
	Logger             *slog.Logger
}

// SimulationManager owns all simulation HTTP handlers.
type SimulationManager struct {
	responder          *httputil.Responder
	simulationService  SimulationService
	classroomRouteHook func(r chi.Router)
	logger             *slog.Logger
}

// NewSimulationManager creates a SimulationManager from the given config.
func NewSimulationManager(cfg SimulationManagerConfig) *SimulationManager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &SimulationManager{
		responder:          httputil.NewResponder(logger),
		simulationService:  cfg.SimulationService,
		classroomRouteHook: cfg.ClassroomRouteHook,
		logger:             logger,
	}
}

// SetupRoutes registers simulation routes on the given router.
func (m *SimulationManager) SetupRoutes(r chi.Router) {
	r.Route("/simulation", func(r chi.Router) {
		// Test student management
		r.Get("/students", m.handleListTestStudents)
		r.Post("/students", m.handleCreateTestStudent)
		r.Get("/students/{userID}/progress", m.handleGetStudentProgress)
		r.Delete("/students/{userID}/progress", m.handleResetStudentProgress)

		// Simulation execution
		r.Post("/run", m.handleSimulateProgress)
		r.Post("/quick", m.handleQuickSimulation)

		// Classroom simulation (enterprise only)
		if m.classroomRouteHook != nil {
			m.classroomRouteHook(r)
		}
	})
}

// -----------------------------------------------------------------------------
// Simulation Handlers (Admin/Development only)
// -----------------------------------------------------------------------------

// CreateTestStudentRequest is the request body for creating a test student
type CreateTestStudentRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SimulateProgressRequest is the request body for simulating pathway progress
type SimulateProgressRequest struct {
	UserID            string                    `json:"userId"`
	PathwayID         string                    `json:"pathwayId"`
	StudentProfile    simulation.StudentProfile `json:"studentProfile"`
	Speed             int                       `json:"speed"`
	ModulesToComplete int                       `json:"modulesToComplete"`
}

func (m *SimulationManager) handleListTestStudents(w http.ResponseWriter, r *http.Request) {
	if m.simulationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "simulation.errors.serviceNotConfigured", nil)
		return
	}

	students, err := m.simulationService.ListTestStudents(r.Context())
	if err != nil {
		m.logger.Error("Failed to list test students", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "simulation.errors.listTestStudentsFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"students": students,
		"count":    len(students),
	})
}

func (m *SimulationManager) handleCreateTestStudent(w http.ResponseWriter, r *http.Request) {
	if m.simulationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "simulation.errors.serviceNotConfigured", nil)
		return
	}

	var req CreateTestStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return
	}

	if req.Name == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.nameRequired", nil)
		return
	}
	if req.Email == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.emailRequired", nil)
		return
	}

	student, err := m.simulationService.CreateTestStudent(r.Context(), req.Name, req.Email)
	if err != nil {
		m.logger.Error("Failed to create test student", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "simulation.errors.createTestStudentFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusCreated, student)
}

func (m *SimulationManager) handleSimulateProgress(w http.ResponseWriter, r *http.Request) {
	if m.simulationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "simulation.errors.serviceNotConfigured", nil)
		return
	}

	var req SimulateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return
	}

	if req.UserID == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.userIdRequired", nil)
		return
	}
	if req.PathwayID == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.pathwayIdRequired", nil)
		return
	}

	// Default profile if not specified
	if req.StudentProfile == "" {
		req.StudentProfile = simulation.ProfileGood
	}

	config := simulation.SimulationConfig{
		StudentProfile:    req.StudentProfile,
		Speed:             req.Speed,
		ModulesToComplete: req.ModulesToComplete,
	}

	result, err := m.simulationService.SimulatePathwayProgression(r.Context(), req.UserID, req.PathwayID, config)
	if err != nil {
		m.responder.SafeErrorResponse(w, err, "simulate pathway progression")
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, result)
}

func (m *SimulationManager) handleGetStudentProgress(w http.ResponseWriter, r *http.Request) {
	if m.simulationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "simulation.errors.serviceNotConfigured", nil)
		return
	}

	userID := chi.URLParam(r, "userID")
	if userID == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.userIDRequired", nil)
		return
	}

	progress, err := m.simulationService.GetStudentProgress(r.Context(), userID)
	if err != nil {
		m.logger.Error("Failed to get student progress", "error", err, "userID", userID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "simulation.errors.getStudentProgressFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, progress)
}

func (m *SimulationManager) handleResetStudentProgress(w http.ResponseWriter, r *http.Request) {
	if m.simulationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "simulation.errors.serviceNotConfigured", nil)
		return
	}

	userID := chi.URLParam(r, "userID")
	if userID == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.userIDRequired", nil)
		return
	}

	if err := m.simulationService.ResetStudentProgress(r.Context(), userID); err != nil {
		m.logger.Error("Failed to reset student progress", "error", err, "userID", userID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "simulation.errors.resetStudentProgressFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"message": "Student progress reset successfully",
		"userID":  userID,
	})
}

// handleQuickSimulation creates a test student and runs a full simulation
func (m *SimulationManager) handleQuickSimulation(w http.ResponseWriter, r *http.Request) {
	if m.simulationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "simulation.errors.serviceNotConfigured", nil)
		return
	}

	type QuickSimRequest struct {
		PathwayID      string                    `json:"pathwayId"`
		StudentProfile simulation.StudentProfile `json:"studentProfile"`
	}

	var req QuickSimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return
	}

	if req.PathwayID == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "simulation.errors.pathwayIdRequired", nil)
		return
	}

	if req.StudentProfile == "" {
		req.StudentProfile = simulation.ProfileGood
	}

	// Generate unique test student
	name := fmt.Sprintf("Test Student %s", time.Now().Format("150405"))
	email := fmt.Sprintf("test.student.%d@simulation.local", time.Now().UnixNano())

	// Create test student
	student, err := m.simulationService.CreateTestStudent(r.Context(), name, email)
	if err != nil {
		m.logger.Error("Failed to create test student", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "simulation.errors.createTestStudentFailed", nil)
		return
	}

	// Run simulation
	config := simulation.SimulationConfig{
		StudentProfile: req.StudentProfile,
		Speed:          0, // Instant
	}

	result, err := m.simulationService.SimulatePathwayProgression(r.Context(), student.ID, req.PathwayID, config)
	if err != nil {
		m.responder.SafeErrorResponse(w, err, "simulate pathway progression")
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, result)
}
