package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/reservations"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// mockReservationRepository implements repositories.ReservationRepository for testing
type mockReservationRepository struct {
	reservations []*models.Reservation
	createErr    error
	updateErr    error
	cancelErr    error
}

func newMockReservationRepository() *mockReservationRepository {
	return &mockReservationRepository{
		reservations: make([]*models.Reservation, 0),
	}
}

func (r *mockReservationRepository) Create(ctx context.Context, reservation *models.Reservation) error {
	if r.createErr != nil {
		return r.createErr
	}
	reservation.CreatedAt = time.Now()
	r.reservations = append(r.reservations, reservation)
	return nil
}

func (r *mockReservationRepository) GetByID(ctx context.Context, id string) (*models.Reservation, error) {
	for _, res := range r.reservations {
		if res.ID == id {
			return res, nil
		}
	}
	return nil, nil
}

func (r *mockReservationRepository) GetByUserID(ctx context.Context, userID string, filter repositories.ReservationFilter) ([]*models.Reservation, error) {
	var result []*models.Reservation
	for _, res := range r.reservations {
		if res.UserID == userID {
			if filter.Status != "" && res.Status != filter.Status {
				continue
			}
			result = append(result, res)
			if filter.Limit > 0 && len(result) >= filter.Limit {
				break
			}
		}
	}
	return result, nil
}

func (r *mockReservationRepository) List(ctx context.Context, filter repositories.ReservationFilter) ([]*models.Reservation, error) {
	result := make([]*models.Reservation, 0)
	for _, res := range r.reservations {
		if filter.Status != "" && res.Status != filter.Status {
			continue
		}
		result = append(result, res)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (r *mockReservationRepository) Update(ctx context.Context, reservation *models.Reservation) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i, res := range r.reservations {
		if res.ID == reservation.ID {
			r.reservations[i] = reservation
			return nil
		}
	}
	return nil
}

func (r *mockReservationRepository) UpdateStatus(ctx context.Context, id string, status models.ReservationStatus) error {
	for _, res := range r.reservations {
		if res.ID == id {
			res.Status = status
			return nil
		}
	}
	return nil
}

func (r *mockReservationRepository) Cancel(ctx context.Context, id string, reason string) error {
	if r.cancelErr != nil {
		return r.cancelErr
	}
	for _, res := range r.reservations {
		if res.ID == id {
			res.Status = models.ReservationStatusCancelled
			now := time.Now()
			res.CancelledAt = &now
			return nil
		}
	}
	return nil
}

func (r *mockReservationRepository) SetPodID(ctx context.Context, id string, podID string) error {
	for _, res := range r.reservations {
		if res.ID == id {
			res.PodID = &podID
			return nil
		}
	}
	return nil
}

func (r *mockReservationRepository) GetOverlapping(ctx context.Context, startTime, endTime time.Time, excludeID string) ([]*models.Reservation, error) {
	var result []*models.Reservation
	for _, res := range r.reservations {
		if res.ID == excludeID {
			continue
		}
		if res.Status == models.ReservationStatusCancelled {
			continue
		}
		// Check for overlap
		if res.StartTime.Before(endTime) && res.EndTime.After(startTime) {
			result = append(result, res)
		}
	}
	return result, nil
}

func (r *mockReservationRepository) GetUpcoming(ctx context.Context, withinMinutes int) ([]*models.Reservation, error) {
	var result []*models.Reservation
	now := time.Now()
	cutoff := now.Add(time.Duration(withinMinutes) * time.Minute)
	for _, res := range r.reservations {
		if res.Status == models.ReservationStatusConfirmed &&
			res.StartTime.After(now) && res.StartTime.Before(cutoff) {
			result = append(result, res)
		}
	}
	return result, nil
}

func (r *mockReservationRepository) GetExpired(ctx context.Context) ([]*models.Reservation, error) {
	var result []*models.Reservation
	now := time.Now()
	for _, res := range r.reservations {
		if (res.Status == models.ReservationStatusConfirmed || res.Status == models.ReservationStatusActive) &&
			res.EndTime.Before(now) {
			result = append(result, res)
		}
	}
	return result, nil
}

func (r *mockReservationRepository) AddReservation(res *models.Reservation) {
	r.reservations = append(r.reservations, res)
}

// -----------------------------------------------------------------------------
// handleListReservations Tests
// -----------------------------------------------------------------------------

func TestHandleListReservations(t *testing.T) {
	t.Run("without authentication returns unauthorized", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv := newTestServerWithReservationRepo(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations", nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("returns empty list when no reservations", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response []reservations.ReservationResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 0 {
			t.Errorf("expected 0 reservations, got %d", len(response))
		}
	})

	t.Run("returns user reservations", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "user-123",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-2",
			UserID:          "other-user",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response []reservations.ReservationResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 1 {
			t.Errorf("expected 1 reservation (user's own), got %d", len(response))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "user-123",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-2",
			UserID:          "user-123",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusCancelled,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations?status=confirmed", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response []reservations.ReservationResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 1 {
			t.Errorf("expected 1 confirmed reservation, got %d", len(response))
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetReservation Tests
// -----------------------------------------------------------------------------

func TestHandleGetReservation(t *testing.T) {
	t.Run("reservation not found", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations/nonexistent", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rr.Code, rr.Body.String())
		}
	})

	t.Run("returns reservation for owner", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "user-123",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations/res-1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response reservations.ReservationResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.ID != "res-1" {
			t.Errorf("expected reservation ID 'res-1', got '%s'", response.ID)
		}
	})

	t.Run("forbidden for non-owner non-admin", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "other-user",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations/res-1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})
}

// -----------------------------------------------------------------------------
// handleCreateReservation Tests
// -----------------------------------------------------------------------------

func TestHandleCreateReservation(t *testing.T) {
	t.Run("missing lab template ID", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		body := `{"startTime": "` + time.Now().Add(time.Hour).Format(time.RFC3339) + `", "durationMinutes": 60}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] != "labTemplateId is required" {
			t.Errorf("unexpected error: %s", response["error"])
		}
	})

	t.Run("missing start time", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		body := `{"labTemplateId": "lab-1", "durationMinutes": 60}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid start time format", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		body := `{"labTemplateId": "lab-1", "startTime": "not-a-date", "durationMinutes": 60}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		body := `{"labTemplateId": "lab-1", "startTime": "` + time.Now().Add(time.Hour).Format(time.RFC3339) + `", "durationMinutes": 0}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("start time in the past", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
		body := `{"labTemplateId": "lab-1", "startTime": "` + pastTime + `", "durationMinutes": 60}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("success creates reservation", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		futureTime := time.Now().Add(time.Hour).Format(time.RFC3339)
		body := `{"labTemplateId": "lab-1", "startTime": "` + futureTime + `", "durationMinutes": 60, "notes": "Test reservation"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response reservations.ReservationResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.ID == "" {
			t.Error("expected reservation ID to be set")
		}
		if response.Status != "confirmed" {
			t.Errorf("expected status 'confirmed', got '%s'", response.Status)
		}
		if response.Notes != "Test reservation" {
			t.Errorf("expected notes 'Test reservation', got '%s'", response.Notes)
		}
	})

	t.Run("sets default resources", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		futureTime := time.Now().Add(time.Hour).Format(time.RFC3339)
		body := `{"labTemplateId": "lab-1", "startTime": "` + futureTime + `", "durationMinutes": 60}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response reservations.ReservationResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Check default resources are set
		if response.Resources.CPU != 4 {
			t.Errorf("expected default CPU 4, got %d", response.Resources.CPU)
		}
		if response.Resources.Memory != 8 {
			t.Errorf("expected default memory 8, got %d", response.Resources.Memory)
		}
		if response.Resources.Storage != 60 {
			t.Errorf("expected default storage 60, got %d", response.Resources.Storage)
		}
	})
}

// -----------------------------------------------------------------------------
// handleCancelReservation Tests
// -----------------------------------------------------------------------------

func TestHandleCancelReservation(t *testing.T) {
	t.Run("reservation not found", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reservations/nonexistent", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rr.Code, rr.Body.String())
		}
	})

	t.Run("forbidden for non-owner", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "other-user",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reservations/res-1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})

	t.Run("cannot cancel completed reservation", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "user-123",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusCompleted,
			StartTime:       time.Now().Add(-2 * time.Hour),
			EndTime:         time.Now().Add(-time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reservations/res-1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("success cancels reservation", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		reservationRepo.AddReservation(&models.Reservation{
			ID:              "res-1",
			UserID:          "user-123",
			LabTemplateID:   "lab-1",
			Status:          models.ReservationStatusConfirmed,
			StartTime:       time.Now().Add(time.Hour),
			EndTime:         time.Now().Add(2 * time.Hour),
			DurationMinutes: 60,
		})

		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		body := `{"reason": "No longer needed"}`
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reservations/res-1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["status"] != "cancelled" {
			t.Errorf("expected status 'cancelled', got '%s'", response["status"])
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetAvailability Tests
// -----------------------------------------------------------------------------

func TestHandleGetAvailability(t *testing.T) {
	t.Run("returns availability for date range", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		startDate := time.Now().Format("2006-01-02")
		endDate := time.Now().AddDate(0, 0, 3).Format("2006-01-02")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations/availability?startDate="+startDate+"&endDate="+endDate, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response []reservations.AvailabilityResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 4 { // 4 days including start and end
			t.Errorf("expected 4 days of availability, got %d", len(response))
		}
	})

	t.Run("invalid start date format", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations/availability?startDate=invalid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("uses defaults when no dates provided", func(t *testing.T) {
		reservationRepo := newMockReservationRepository()
		srv, token := newTestServerWithReservationRepoAndAuth(t, reservationRepo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reservations/availability", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response []reservations.AvailabilityResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Default is 7 days from today
		if len(response) != 8 { // 8 days including today
			t.Errorf("expected 8 days of availability (default), got %d", len(response))
		}
	})
}

// -----------------------------------------------------------------------------
// ToReservationResponse Tests
// -----------------------------------------------------------------------------

func TestToReservationResponse(t *testing.T) {
	now := time.Now()
	cancelledAt := now.Add(-time.Hour)
	podID := "pod-123"

	reservation := &models.Reservation{
		ID:              "res-1",
		UserID:          "user-1",
		LabTemplateID:   "lab-1",
		LabTemplateName: "Test Lab",
		PodID:           &podID,
		Status:          models.ReservationStatusCancelled,
		StartTime:       now,
		EndTime:         now.Add(time.Hour),
		DurationMinutes: 60,
		Resources: models.ResourceSpec{
			CPU:    4,
			Memory: 8,
			Disk:   100,
		},
		Notes:       "Test notes",
		CreatedAt:   now,
		CancelledAt: &cancelledAt,
	}

	response := reservations.ToReservationResponse(reservation)

	if response.ID != reservation.ID {
		t.Errorf("expected ID %s, got %s", reservation.ID, response.ID)
	}
	if response.UserID != reservation.UserID {
		t.Errorf("expected UserID %s, got %s", reservation.UserID, response.UserID)
	}
	if response.LabTemplateName != reservation.LabTemplateName {
		t.Errorf("expected LabTemplateName %s, got %s", reservation.LabTemplateName, response.LabTemplateName)
	}
	if *response.PodID != *reservation.PodID {
		t.Errorf("expected PodID %s, got %s", *reservation.PodID, *response.PodID)
	}
	if response.Status != string(reservation.Status) {
		t.Errorf("expected Status %s, got %s", reservation.Status, response.Status)
	}
	if response.Resources.CPU != reservation.Resources.CPU {
		t.Errorf("expected CPU %d, got %d", reservation.Resources.CPU, response.Resources.CPU)
	}
	if response.Resources.Memory != reservation.Resources.Memory {
		t.Errorf("expected Memory %d, got %d", reservation.Resources.Memory, response.Resources.Memory)
	}
	if response.Resources.Storage != reservation.Resources.Disk {
		t.Errorf("expected Storage %d, got %d", reservation.Resources.Disk, response.Resources.Storage)
	}
	if response.Notes != reservation.Notes {
		t.Errorf("expected Notes %s, got %s", reservation.Notes, response.Notes)
	}
	if response.CancelledAt == nil {
		t.Error("expected CancelledAt to be set")
	}
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

// newTestServerWithReservationRepo creates a test server with a reservation repository
func newTestServerWithReservationRepo(t *testing.T, reservationRepo *mockReservationRepository) *Server {
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
		WithReservationRepo(reservationRepo),
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)
	return srv
}

// newTestServerWithReservationRepoAndAuth creates a test server with reservation repo and auth token
func newTestServerWithReservationRepoAndAuth(t *testing.T, reservationRepo *mockReservationRepository) (*Server, string) {
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
		WithReservationRepo(reservationRepo),
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)

	// Generate token for a regular user
	user := &auth.User{
		ID:    "user-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"student"},
	}
	token, err := authSvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	return srv, token
}
