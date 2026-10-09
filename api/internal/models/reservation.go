// Package models contains shared data types for the lab platform
package models

import "time"

// ReservationStatus represents the state of a reservation
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "pending"
	ReservationStatusConfirmed ReservationStatus = "confirmed"
	ReservationStatusActive    ReservationStatus = "active"
	ReservationStatusCompleted ReservationStatus = "completed"
	ReservationStatusCancelled ReservationStatus = "cancelled"
	ReservationStatusExpired   ReservationStatus = "expired"
)

// Reservation represents a scheduled lab time slot for a user
type Reservation struct {
	ID              string            `json:"id"`
	UserID          string            `json:"userId"`
	LabTemplateID   string            `json:"labTemplateId"`
	LabTemplateName string            `json:"labTemplateName,omitempty"`
	PodID           *string           `json:"podId,omitempty"` // Set when pod is provisioned
	Status          ReservationStatus `json:"status"`
	StartTime       time.Time         `json:"startTime"`
	EndTime         time.Time         `json:"endTime"`
	DurationMinutes int               `json:"durationMinutes"`
	Resources       ResourceSpec      `json:"resources"`
	Notes           string            `json:"notes,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	CancelledAt     *time.Time        `json:"cancelledAt,omitempty"`
	CancelReason    string            `json:"cancelReason,omitempty"`
}

// TimeSlot represents an available time slot for reservations
type TimeSlot struct {
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Available bool      `json:"available"`
	Capacity  int       `json:"capacity"` // Total capacity for this slot
	Reserved  int       `json:"reserved"` // How many are already reserved
}

// AvailabilityQuery represents query parameters for checking availability
type AvailabilityQuery struct {
	LabTemplateID   string    `json:"labTemplateId,omitempty"`
	StartDate       time.Time `json:"startDate"`
	EndDate         time.Time `json:"endDate"`
	DurationMinutes int       `json:"durationMinutes,omitempty"`
}
