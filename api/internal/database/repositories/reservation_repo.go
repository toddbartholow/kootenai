package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// ReservationRepo implements ReservationRepository
type ReservationRepo struct {
	db DBTX
}

// NewReservationRepo creates a new reservation repository
func NewReservationRepo(db DBTX) *ReservationRepo {
	return &ReservationRepo{db: db}
}

// Create inserts a new reservation
func (r *ReservationRepo) Create(ctx context.Context, reservation *models.Reservation) error {
	query := `
		INSERT INTO reservations (
			id, user_id, lab_template_id, status,
			start_time, end_time, duration_minutes,
			cpu_cores, memory_gb, storage_gb, notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		reservation.ID,
		reservation.UserID,
		reservation.LabTemplateID,
		reservation.Status,
		reservation.StartTime,
		reservation.EndTime,
		reservation.DurationMinutes,
		reservation.Resources.CPU,
		reservation.Resources.Memory,
		reservation.Resources.Disk,
		nullString(reservation.Notes),
	).Scan(&reservation.CreatedAt, &reservation.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting reservation: %w", err)
	}

	return nil
}

// GetByID retrieves a reservation by ID
func (r *ReservationRepo) GetByID(ctx context.Context, id string) (*models.Reservation, error) {
	query := `
		SELECT r.id, r.user_id, r.lab_template_id, lt.name,
		       r.pod_id, r.status, r.start_time, r.end_time,
		       r.duration_minutes, r.cpu_cores, r.memory_gb, r.storage_gb,
		       r.notes, r.cancel_reason, r.cancelled_at,
		       r.created_at, r.updated_at
		FROM reservations r
		LEFT JOIN lab_templates lt ON r.lab_template_id = lt.id
		WHERE r.id = $1`

	reservation, err := r.scanReservation(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return reservation, err
}

// GetByUserID retrieves reservations for a user
func (r *ReservationRepo) GetByUserID(ctx context.Context, userID string, filter ReservationFilter) ([]*models.Reservation, error) {
	filter.UserID = userID
	return r.List(ctx, filter)
}

// List retrieves reservations matching the filter
func (r *ReservationRepo) List(ctx context.Context, filter ReservationFilter) ([]*models.Reservation, error) {
	qb := NewQueryBuilder(`
		SELECT r.id, r.user_id, r.lab_template_id, lt.name,
		       r.pod_id, r.status, r.start_time, r.end_time,
		       r.duration_minutes, r.cpu_cores, r.memory_gb, r.storage_gb,
		       r.notes, r.cancel_reason, r.cancelled_at,
		       r.created_at, r.updated_at
		FROM reservations r
		LEFT JOIN lab_templates lt ON r.lab_template_id = lt.id
		WHERE 1=1`)

	if filter.UserID != "" {
		qb.AddCondition("r.user_id = $%d", filter.UserID)
	}

	if filter.LabTemplateID != "" {
		qb.AddCondition("r.lab_template_id = $%d", filter.LabTemplateID)
	}

	if filter.Status != "" {
		qb.AddCondition("r.status = $%d", filter.Status)
	}

	if filter.StartAfter != nil {
		qb.AddCondition("r.start_time >= $%d", *filter.StartAfter)
	}

	if filter.StartBefore != nil {
		qb.AddCondition("r.start_time <= $%d", *filter.StartBefore)
	}

	qb.OrderByRaw("r.start_time ASC")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying reservations: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// Update updates a reservation
func (r *ReservationRepo) Update(ctx context.Context, reservation *models.Reservation) error {
	query := `
		UPDATE reservations
		SET lab_template_id = $2, status = $3,
		    start_time = $4, end_time = $5, duration_minutes = $6,
		    cpu_cores = $7, memory_gb = $8, storage_gb = $9,
		    notes = $10
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		reservation.ID,
		reservation.LabTemplateID,
		reservation.Status,
		reservation.StartTime,
		reservation.EndTime,
		reservation.DurationMinutes,
		reservation.Resources.CPU,
		reservation.Resources.Memory,
		reservation.Resources.Disk,
		nullString(reservation.Notes),
	)

	if err != nil {
		return fmt.Errorf("updating reservation: %w", err)
	}

	return nil
}

// UpdateStatus updates the reservation status
func (r *ReservationRepo) UpdateStatus(ctx context.Context, id string, status models.ReservationStatus) error {
	query := `UPDATE reservations SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("updating reservation status: %w", err)
	}
	return nil
}

// Cancel cancels a reservation
func (r *ReservationRepo) Cancel(ctx context.Context, id string, reason string) error {
	query := `
		UPDATE reservations
		SET status = $2, cancel_reason = $3, cancelled_at = $4
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, models.ReservationStatusCancelled, reason, time.Now())
	if err != nil {
		return fmt.Errorf("canceling reservation: %w", err)
	}
	return nil
}

// SetPodID associates a pod with the reservation
func (r *ReservationRepo) SetPodID(ctx context.Context, id string, podID string) error {
	query := `UPDATE reservations SET pod_id = $2, status = $3 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, podID, models.ReservationStatusActive)
	if err != nil {
		return fmt.Errorf("setting pod ID: %w", err)
	}
	return nil
}

// GetOverlapping finds reservations that overlap with a time range.
// Uses QueryBuilder so the optional excludeID condition is appended without
// hand-managing positional placeholders. The status filter is static so it
// stays in the base query.
func (r *ReservationRepo) GetOverlapping(ctx context.Context, startTime, endTime time.Time, excludeID string) ([]*models.Reservation, error) {
	qb := NewQueryBuilder(`
		SELECT r.id, r.user_id, r.lab_template_id, lt.name,
		       r.pod_id, r.status, r.start_time, r.end_time,
		       r.duration_minutes, r.cpu_cores, r.memory_gb, r.storage_gb,
		       r.notes, r.cancel_reason, r.cancelled_at,
		       r.created_at, r.updated_at
		FROM reservations r
		LEFT JOIN lab_templates lt ON r.lab_template_id = lt.id
		WHERE r.status IN ('pending', 'confirmed', 'active')`)

	// Overlap predicate: existing.start < candidate.end AND existing.end > candidate.start.
	qb.AddCondition("r.start_time < $%d", endTime)
	qb.AddCondition("r.end_time > $%d", startTime)

	if excludeID != "" {
		qb.AddCondition("r.id != $%d", excludeID)
	}

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying overlapping reservations: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// GetUpcoming retrieves reservations that start within the specified minutes
func (r *ReservationRepo) GetUpcoming(ctx context.Context, withinMinutes int) ([]*models.Reservation, error) {
	query := `
		SELECT r.id, r.user_id, r.lab_template_id, lt.name,
		       r.pod_id, r.status, r.start_time, r.end_time,
		       r.duration_minutes, r.cpu_cores, r.memory_gb, r.storage_gb,
		       r.notes, r.cancel_reason, r.cancelled_at,
		       r.created_at, r.updated_at
		FROM reservations r
		LEFT JOIN lab_templates lt ON r.lab_template_id = lt.id
		WHERE r.status = 'confirmed'
		  AND r.start_time <= NOW() + $1 * INTERVAL '1 minute'
		  AND r.start_time > NOW()
		ORDER BY r.start_time ASC`

	rows, err := r.db.QueryContext(ctx, query, withinMinutes)
	if err != nil {
		return nil, fmt.Errorf("querying upcoming reservations: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// GetExpired retrieves reservations that have passed their end time
func (r *ReservationRepo) GetExpired(ctx context.Context) ([]*models.Reservation, error) {
	query := `
		SELECT r.id, r.user_id, r.lab_template_id, lt.name,
		       r.pod_id, r.status, r.start_time, r.end_time,
		       r.duration_minutes, r.cpu_cores, r.memory_gb, r.storage_gb,
		       r.notes, r.cancel_reason, r.cancelled_at,
		       r.created_at, r.updated_at
		FROM reservations r
		LEFT JOIN lab_templates lt ON r.lab_template_id = lt.id
		WHERE r.status IN ('pending', 'confirmed', 'active')
		  AND r.end_time < NOW()
		ORDER BY r.end_time ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying expired reservations: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// Helper methods

func (r *ReservationRepo) scanMultiple(rows *sql.Rows) ([]*models.Reservation, error) {
	var reservations []*models.Reservation
	for rows.Next() {
		reservation, err := r.scanReservationRow(rows)
		if err != nil {
			return nil, err
		}
		reservations = append(reservations, reservation)
	}
	return reservations, rows.Err()
}

type reservationScannable interface {
	Scan(dest ...any) error
}

func (r *ReservationRepo) scanReservation(row reservationScannable) (*models.Reservation, error) {
	var reservation models.Reservation
	var labTemplateName, notes, cancelReason sql.NullString
	var podID sql.NullString
	var cancelledAt sql.NullTime
	var cpuCores, memoryGB, storageGB int

	err := row.Scan(
		&reservation.ID,
		&reservation.UserID,
		&reservation.LabTemplateID,
		&labTemplateName,
		&podID,
		&reservation.Status,
		&reservation.StartTime,
		&reservation.EndTime,
		&reservation.DurationMinutes,
		&cpuCores,
		&memoryGB,
		&storageGB,
		&notes,
		&cancelReason,
		&cancelledAt,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning reservation: %w", err)
	}

	reservation.LabTemplateName = labTemplateName.String
	reservation.Notes = notes.String
	reservation.CancelReason = cancelReason.String

	if podID.Valid {
		reservation.PodID = &podID.String
	}

	if cancelledAt.Valid {
		reservation.CancelledAt = &cancelledAt.Time
	}

	reservation.Resources = models.ResourceSpec{
		CPU:    cpuCores,
		Memory: memoryGB,
		Disk:   storageGB,
	}

	return &reservation, nil
}

func (r *ReservationRepo) scanReservationRow(rows *sql.Rows) (*models.Reservation, error) {
	return r.scanReservation(rows)
}
