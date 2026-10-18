// Package reservations provides reservation HTTP handlers.
package reservations

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// Config holds configuration for creating a Manager.
type Config struct {
	ReservationRepo repositories.ReservationRepository
	Logger          *slog.Logger
}

// Manager owns all reservation HTTP handlers.
type Manager struct {
	responder       *httputil.Responder
	reservationRepo repositories.ReservationRepository
	logger          *slog.Logger
}

// NewManager creates a Manager from the given config.
func NewManager(cfg Config) *Manager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		responder:       httputil.NewResponder(logger),
		reservationRepo: cfg.ReservationRepo,
		logger:          logger,
	}
}

// SetupRoutes registers reservation routes on the router.
func (m *Manager) SetupRoutes(r chi.Router) {
	r.Route("/reservations", func(r chi.Router) {
		r.Get("/", m.handleListReservations)
		r.Post("/", m.handleCreateReservation)
		r.Get("/availability", m.handleGetAvailability)
		r.Get("/{reservationID}", m.handleGetReservation)
		r.Delete("/{reservationID}", m.handleCancelReservation)
	})
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// CreateReservationRequest represents the request body for creating a reservation.
type CreateReservationRequest struct {
	LabTemplateID   string `json:"labTemplateId"`
	StartTime       string `json:"startTime"`
	DurationMinutes int    `json:"durationMinutes"`
	Resources       struct {
		CPU     int `json:"cpu"`
		Memory  int `json:"memory"`
		Storage int `json:"storage"`
	} `json:"resources"`
	Notes string `json:"notes,omitempty"`
}

// ReservationResponse represents the API response for a reservation.
type ReservationResponse struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	LabTemplateID   string    `json:"labTemplateId"`
	LabTemplateName string    `json:"labTemplateName,omitempty"`
	PodID           *string   `json:"podId,omitempty"`
	Status          string    `json:"status"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	DurationMinutes int       `json:"durationMinutes"`
	Resources       struct {
		CPU     int `json:"cpu"`
		Memory  int `json:"memory"`
		Storage int `json:"storage"`
	} `json:"resources"`
	Notes       string     `json:"notes,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`
}

// AvailabilityResponse represents available time slots.
type AvailabilityResponse struct {
	Date  string `json:"date"`
	Slots []struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		Available bool   `json:"available"`
	} `json:"slots"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ToReservationResponse(r *models.Reservation) ReservationResponse {
	resp := ReservationResponse{
		ID:              r.ID,
		UserID:          r.UserID,
		LabTemplateID:   r.LabTemplateID,
		LabTemplateName: r.LabTemplateName,
		PodID:           r.PodID,
		Status:          string(r.Status),
		StartTime:       r.StartTime,
		EndTime:         r.EndTime,
		DurationMinutes: r.DurationMinutes,
		Notes:           r.Notes,
		CreatedAt:       r.CreatedAt,
		CancelledAt:     r.CancelledAt,
	}
	resp.Resources.CPU = r.Resources.CPU
	resp.Resources.Memory = r.Resources.Memory
	resp.Resources.Storage = r.Resources.Disk
	return resp
}

func getUserID(r *http.Request) string {
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil {
		return ""
	}
	return user.ID
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (m *Manager) handleListReservations(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	filter := repositories.ReservationFilter{
		Limit: 100,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = models.ReservationStatus(status)
	}

	reservations, err := m.reservationRepo.GetByUserID(r.Context(), userID, filter)
	if err != nil {
		m.logger.Error("Failed to list reservations", "error", err, "userId", userID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.listFailed", nil)
		return
	}

	responses := make([]ReservationResponse, 0, len(reservations))
	for _, res := range reservations {
		responses = append(responses, ToReservationResponse(res))
	}

	m.responder.JSONResponse(w, http.StatusOK, responses)
}

func (m *Manager) handleGetReservation(w http.ResponseWriter, r *http.Request) {
	reservationID := chi.URLParam(r, "reservationID")
	userID := getUserID(r)

	reservation, err := m.reservationRepo.GetByID(r.Context(), reservationID)
	if err != nil {
		m.logger.Error("Failed to get reservation", "error", err, "id", reservationID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.getFailed", nil)
		return
	}

	if reservation == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "reservation.errors.notFound", nil)
		return
	}

	if reservation.UserID != userID && !serverutil.IsAdminRequest(r) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "reservation.errors.viewNotAuthorized", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, ToReservationResponse(reservation))
}

func (m *Manager) handleCreateReservation(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	var req CreateReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return
	}

	if req.LabTemplateID == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.labTemplateIdRequired", nil)
		return
	}
	if req.StartTime == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.startTimeRequired", nil)
		return
	}
	if req.DurationMinutes <= 0 {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.durationMustBePositive", nil)
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.startTimeInvalid", nil)
		return
	}

	endTime := startTime.Add(time.Duration(req.DurationMinutes) * time.Minute)

	if startTime.Before(time.Now()) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.startTimeMustBeFuture", nil)
		return
	}

	if req.Resources.CPU == 0 {
		req.Resources.CPU = 4
	}
	if req.Resources.Memory == 0 {
		req.Resources.Memory = 8
	}
	if req.Resources.Storage == 0 {
		req.Resources.Storage = 60
	}

	overlapping, err := m.reservationRepo.GetOverlapping(r.Context(), startTime, endTime, "")
	if err != nil {
		m.logger.Error("Failed to check overlapping reservations", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.checkAvailabilityFailed", nil)
		return
	}

	if len(overlapping) >= 20 {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "reservation.errors.noCapacity", nil)
		return
	}

	reservation := &models.Reservation{
		ID:              uuid.New().String(),
		UserID:          userID,
		LabTemplateID:   req.LabTemplateID,
		Status:          models.ReservationStatusConfirmed,
		StartTime:       startTime,
		EndTime:         endTime,
		DurationMinutes: req.DurationMinutes,
		Resources: models.ResourceSpec{
			CPU:    req.Resources.CPU,
			Memory: req.Resources.Memory,
			Disk:   req.Resources.Storage,
		},
		Notes: req.Notes,
	}

	if err := m.reservationRepo.Create(r.Context(), reservation); err != nil {
		m.logger.Error("Failed to create reservation", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.createFailed", nil)
		return
	}

	m.logger.Info("Reservation created", "id", reservation.ID, "userId", userID, "start", startTime)
	m.responder.JSONResponse(w, http.StatusCreated, ToReservationResponse(reservation))
}

func (m *Manager) handleCancelReservation(w http.ResponseWriter, r *http.Request) {
	reservationID := chi.URLParam(r, "reservationID")
	userID := getUserID(r)

	reservation, err := m.reservationRepo.GetByID(r.Context(), reservationID)
	if err != nil {
		m.logger.Error("Failed to get reservation", "error", err, "id", reservationID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.getFailed", nil)
		return
	}

	if reservation == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "reservation.errors.notFound", nil)
		return
	}

	if reservation.UserID != userID && !serverutil.IsAdminRequest(r) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "reservation.errors.cancelNotAuthorized", nil)
		return
	}

	if reservation.Status != models.ReservationStatusPending &&
		reservation.Status != models.ReservationStatusConfirmed {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.cannotCancel", nil)
		return
	}

	var cancelReq struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&cancelReq)

	if err := m.reservationRepo.Cancel(r.Context(), reservationID, cancelReq.Reason); err != nil {
		m.logger.Error("Failed to cancel reservation", "error", err, "id", reservationID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.cancelFailed", nil)
		return
	}

	m.logger.Info("Reservation cancelled", "id", reservationID, "userId", userID)
	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"status":  "cancelled",
		"id":      reservationID,
		"message": "Reservation has been cancelled",
	})
}

func (m *Manager) handleGetAvailability(w http.ResponseWriter, r *http.Request) {
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	if startDate == "" {
		startDate = time.Now().Format("2006-01-02")
	}
	if endDate == "" {
		start, _ := time.Parse("2006-01-02", startDate)
		endDate = start.AddDate(0, 0, 7).Format("2006-01-02")
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.startDateInvalid", nil)
		return
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "reservation.errors.endDateInvalid", nil)
		return
	}

	endOfDay := end.Add(24 * time.Hour)
	overlapping, err := m.reservationRepo.GetOverlapping(r.Context(), start, endOfDay, "")
	if err != nil {
		m.logger.Error("Failed to get reservations for availability", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "reservation.errors.checkAvailabilityFailed", nil)
		return
	}

	var availability []AvailabilityResponse

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dayAvail := AvailabilityResponse{
			Date: d.Format("2006-01-02"),
		}

		for hour := 8; hour < 20; hour++ {
			slotStart := time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, d.Location())
			slotEnd := slotStart.Add(time.Hour)

			count := 0
			for _, res := range overlapping {
				if res.StartTime.Before(slotEnd) && res.EndTime.After(slotStart) {
					count++
				}
			}

			available := count < 20

			dayAvail.Slots = append(dayAvail.Slots, struct {
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
				Available bool   `json:"available"`
			}{
				StartTime: slotStart.Format("15:04"),
				EndTime:   slotEnd.Format("15:04"),
				Available: available,
			})
		}

		availability = append(availability, dayAvail)
	}

	m.responder.JSONResponse(w, http.StatusOK, availability)
}
