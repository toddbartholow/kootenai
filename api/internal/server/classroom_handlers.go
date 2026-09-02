package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/classroom"
	"github.com/toddbartholow/kootenai/api/internal/enterprise"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// -----------------------------------------------------------------------------
// Classroom Manager
// -----------------------------------------------------------------------------

// ClassroomManager manages classroom simulation operations
type ClassroomManager struct {
	classroomService *classroom.Service
	classroomRunner  *classroom.ClassroomRunner
	logger           *slog.Logger
	responder        *httputil.Responder
}

// ClassroomManagerConfig configures ClassroomManager
type ClassroomManagerConfig struct {
	ClassroomService *classroom.Service
	ClassroomRunner  *classroom.ClassroomRunner
	Logger           *slog.Logger
	Responder        *httputil.Responder
}

// NewClassroomManager creates a new ClassroomManager
func NewClassroomManager(cfg ClassroomManagerConfig) *ClassroomManager {
	return &ClassroomManager{
		classroomService: cfg.ClassroomService,
		classroomRunner:  cfg.ClassroomRunner,
		logger:           cfg.Logger,
		responder:        cfg.Responder,
	}
}

// SetupRoutes registers classroom simulation routes under /simulation.
func (m *ClassroomManager) SetupRoutes(r chi.Router) {
	r.Route("/classrooms", func(r chi.Router) {
		// Gate all routes behind enterprise feature
		r.Use(custommiddleware.RequireEnterprise(enterprise.FeatureAIClassroom))

		r.Post("/", m.handleCreateClassroom)
		r.Get("/", m.handleListClassrooms)
		r.Get("/{id}", m.handleGetClassroom)
		r.Post("/{id}/start", m.handleStartClassroom)
		r.Post("/{id}/pause", m.handlePauseClassroom)
		r.Post("/{id}/stop", m.handleStopClassroom)
		r.Delete("/{id}", m.handleDeleteClassroom)
		r.Get("/{id}/activities", m.handleListClassroomActivities)
		r.Get("/{id}/feedback", m.handleListClassroomFeedback)
		r.Get("/{id}/feedback/summary", m.handleGetClassroomFeedbackSummary)
		r.Get("/{id}/students/{studentId}", m.handleGetClassroomStudent)
	})
}

func (m *ClassroomManager) handleCreateClassroom(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	var req classroom.CreateSimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return
	}

	if req.Name == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "classroom.errors.nameRequired", nil)
		return
	}

	sim, err := m.classroomService.CreateSimulation(r.Context(), req)
	if err != nil {
		m.logger.Error("Failed to create classroom simulation", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.createFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusCreated, sim)
}

func (m *ClassroomManager) handleListClassrooms(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	sims, err := m.classroomService.ListSimulations(r.Context())
	if err != nil {
		m.logger.Error("Failed to list classroom simulations", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.listFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"simulations": sims,
		"count":       len(sims),
	})
}

func (m *ClassroomManager) handleGetClassroom(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	sim, err := m.classroomService.GetSimulation(r.Context(), id)
	if err != nil {
		m.logger.Error("Failed to get classroom simulation", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.getFailed", nil)
		return
	}
	if sim == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "classroom.errors.simulationNotFound", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, sim)
}

func (m *ClassroomManager) handleStartClassroom(w http.ResponseWriter, r *http.Request) {
	if m.classroomRunner == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.runnerNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if err := m.classroomRunner.Start(r.Context(), id); err != nil {
		m.logger.Error("Failed to start classroom simulation", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.startFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"message": "simulation started",
		"id":      id,
		"status":  "running",
	})
}

func (m *ClassroomManager) handlePauseClassroom(w http.ResponseWriter, r *http.Request) {
	if m.classroomRunner == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.runnerNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if err := m.classroomRunner.Pause(id); err != nil {
		m.logger.Error("Failed to pause classroom simulation", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.pauseFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"message": "simulation paused",
		"id":      id,
		"status":  "paused",
	})
}

func (m *ClassroomManager) handleStopClassroom(w http.ResponseWriter, r *http.Request) {
	if m.classroomRunner == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.runnerNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if err := m.classroomRunner.Stop(id); err != nil {
		m.logger.Error("Failed to stop classroom simulation", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.stopFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"message": "simulation stopped",
		"id":      id,
		"status":  "completed",
	})
}

func (m *ClassroomManager) handleDeleteClassroom(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")

	// Stop running goroutines before deleting from DB
	if m.classroomRunner != nil && m.classroomRunner.IsRunning(id) {
		if err := m.classroomRunner.Stop(id); err != nil {
			m.logger.Warn("Failed to stop simulation before delete", "error", err, "id", id)
		}
	}

	if err := m.classroomService.DeleteSimulation(r.Context(), id); err != nil {
		m.logger.Error("Failed to delete classroom simulation", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.deleteFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"message": "simulation deleted",
		"id":      id,
	})
}

func (m *ClassroomManager) handleListClassroomActivities(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	// Clamp pagination values
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	activities, total, err := m.classroomService.ListActivities(r.Context(), id, limit, offset)
	if err != nil {
		m.logger.Error("Failed to list classroom activities", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.listActivitiesFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"activities": activities,
		"total":      total,
	})
}

func (m *ClassroomManager) handleGetClassroomStudent(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	simID := chi.URLParam(r, "id")
	studentID := chi.URLParam(r, "studentId")
	student, err := m.classroomService.GetStudent(r.Context(), studentID)
	if err != nil {
		m.logger.Error("Failed to get classroom student", "error", err, "studentId", studentID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.getStudentFailed", nil)
		return
	}
	if student == nil || student.SimulationID != simID {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "classroom.errors.studentNotFound", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, student)
}

func (m *ClassroomManager) handleListClassroomFeedback(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	feedbackType := r.URL.Query().Get("type")
	labID := r.URL.Query().Get("labId")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	feedback, total, err := m.classroomService.ListFeedback(r.Context(), id, feedbackType, labID, limit, offset)
	if err != nil {
		m.logger.Error("Failed to list classroom feedback", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.listFeedbackFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"feedback": feedback,
		"total":    total,
	})
}

func (m *ClassroomManager) handleGetClassroomFeedbackSummary(w http.ResponseWriter, r *http.Request) {
	if m.classroomService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "classroom.errors.serviceNotConfigured", nil)
		return
	}

	id := chi.URLParam(r, "id")
	summary, err := m.classroomService.GetFeedbackSummary(r.Context(), id)
	if err != nil {
		m.logger.Error("Failed to get feedback summary", "error", err, "id", id)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "classroom.errors.feedbackSummaryFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, summary)
}
