package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// ReservationRepo Tests
// -----------------------------------------------------------------------------

func TestReservationRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name        string
		reservation *models.Reservation
		mockFn      func(mock sqlmock.Sqlmock)
		wantErr     bool
		errMsg      string
	}{
		{
			name: "success with all fields",
			reservation: &models.Reservation{
				ID:              "res-123",
				UserID:          "user-123",
				LabTemplateID:   "lab-123",
				Status:          models.ReservationStatusPending,
				StartTime:       fixedTime,
				EndTime:         endTime,
				DurationMinutes: 120,
				Resources: models.ResourceSpec{
					CPU:    4,
					Memory: 8,
					Disk:   50,
				},
				Notes: "Test reservation",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO reservations`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			reservation: &models.Reservation{
				ID:            "res-err",
				UserID:        "user-123",
				LabTemplateID: "lab-123",
				Status:        models.ReservationStatusPending,
				StartTime:     fixedTime,
				EndTime:       endTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO reservations`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting reservation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			err = repo.Create(context.Background(), tt.reservation)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !reservationContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing reservation",
			id:   "res-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-123", "user-123", "lab-123", "Network Lab",
					nil, "pending", fixedTime, endTime,
					120, 4, 8, 50,
					"Test notes", nil, nil,
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WithArgs("res-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent reservation",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "res-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WillReturnError(errors.New("connection error"))
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByID(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && result != nil {
					t.Error("expected nil result")
				}
				if !tt.wantNil && result == nil {
					t.Error("expected non-nil result")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)
	startAfter := fixedTime.Add(-1 * time.Hour)
	startBefore := fixedTime.Add(24 * time.Hour)

	tests := []struct {
		name      string
		filter    ReservationFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list all reservations",
			filter: ReservationFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-1", "user-1", "lab-1", "Lab 1",
					nil, "pending", fixedTime, endTime,
					120, 4, 8, 50,
					"", nil, nil,
					fixedTime, fixedTime,
				).AddRow(
					"res-2", "user-2", "lab-2", "Lab 2",
					"pod-123", "active", fixedTime, endTime,
					120, 2, 4, 25,
					"", nil, nil,
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "filter by user ID",
			filter: ReservationFilter{
				UserID: "user-123",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-1", "user-123", "lab-1", "Lab 1",
					nil, "pending", fixedTime, endTime,
					120, 4, 8, 50,
					"", nil, nil,
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ AND r.user_id = \$1 .+ LIMIT \$2`).
					WithArgs("user-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "filter by status",
			filter: ReservationFilter{
				Status: "active",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ AND r.status = \$1 .+ LIMIT \$2`).
					WithArgs("active", 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "filter by time range",
			filter: ReservationFilter{
				StartAfter:  &startAfter,
				StartBefore: &startBefore,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ AND r.start_time >= \$1 AND r.start_time <= \$2 .+ LIMIT \$3`).
					WithArgs(startAfter, startBefore, 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "with limit and offset",
			filter: ReservationFilter{
				Limit:  10,
				Offset: 5,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ LIMIT \$1 OFFSET \$2`).
					WithArgs(10, 5).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			filter: ReservationFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WillReturnError(errors.New("connection error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			result, err := repo.List(context.Background(), tt.filter)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_GetByUserID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name      string
		userID    string
		filter    ReservationFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "success",
			userID: "user-123",
			filter: ReservationFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-1", "user-123", "lab-1", "Lab 1",
					nil, "pending", fixedTime, endTime,
					120, 4, 8, 50,
					"", nil, nil,
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ LIMIT \$2`).
					WithArgs("user-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByUserID(context.Background(), tt.userID, tt.filter)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name        string
		reservation *models.Reservation
		mockFn      func(mock sqlmock.Sqlmock)
		wantErr     bool
		errMsg      string
	}{
		{
			name: "success",
			reservation: &models.Reservation{
				ID:              "res-123",
				LabTemplateID:   "lab-456",
				Status:          models.ReservationStatusConfirmed,
				StartTime:       fixedTime,
				EndTime:         endTime,
				DurationMinutes: 120,
				Resources: models.ResourceSpec{
					CPU:    8,
					Memory: 16,
					Disk:   100,
				},
				Notes: "Updated notes",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			reservation: &models.Reservation{
				ID:     "res-err",
				Status: models.ReservationStatusPending,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating reservation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.reservation)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !reservationContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_UpdateStatus(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		status  models.ReservationStatus
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success",
			id:     "res-123",
			status: models.ReservationStatusActive,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations SET status = \$2 WHERE id = \$1`).
					WithArgs("res-123", models.ReservationStatusActive).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			id:     "res-err",
			status: models.ReservationStatusActive,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations SET status = \$2 WHERE id = \$1`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating reservation status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateStatus(context.Background(), tt.id, tt.status)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !reservationContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_Cancel(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		reason  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success",
			id:     "res-123",
			reason: "User requested cancellation",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations SET status = \$2, cancel_reason = \$3, cancelled_at = \$4 WHERE id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			id:     "res-err",
			reason: "Error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations SET status = \$2`).
					WillReturnError(errors.New("cancel failed"))
			},
			wantErr: true,
			errMsg:  "canceling reservation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			err = repo.Cancel(context.Background(), tt.id, tt.reason)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !reservationContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_SetPodID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		podID   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:  "success",
			id:    "res-123",
			podID: "pod-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations SET pod_id = \$2, status = \$3 WHERE id = \$1`).
					WithArgs("res-123", "pod-456", models.ReservationStatusActive).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:  "database error",
			id:    "res-err",
			podID: "pod-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE reservations SET pod_id = \$2`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "setting pod ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			err = repo.SetPodID(context.Background(), tt.id, tt.podID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !reservationContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_GetOverlapping(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	startTime := fixedTime
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		excludeID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "found overlapping",
			startTime: startTime,
			endTime:   endTime,
			excludeID: "",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-1", "user-1", "lab-1", "Lab 1",
					nil, "confirmed", fixedTime, endTime,
					120, 4, 8, 50,
					"", nil, nil,
					fixedTime, fixedTime,
				)
				// Placeholders/args are assigned by QueryBuilder in the order
				// AddCondition is called: $1=endTime (start_time < endTime),
				// $2=startTime (end_time > startTime). The overlap predicate
				// is the same; only the argument order through the driver
				// changed when the implementation moved to QueryBuilder.
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ WHERE .+ AND r.start_time < \$1 AND r.end_time > \$2`).
					WithArgs(endTime, startTime).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "with exclude ID",
			startTime: startTime,
			endTime:   endTime,
			excludeID: "res-current",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ AND r.id != \$3`).
					WithArgs(endTime, startTime, "res-current").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "database error",
			startTime: startTime,
			endTime:   endTime,
			excludeID: "",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WillReturnError(errors.New("connection error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetOverlapping(context.Background(), tt.startTime, tt.endTime, tt.excludeID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_GetUpcoming(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name          string
		withinMinutes int
		mockFn        func(mock sqlmock.Sqlmock)
		wantCount     int
		wantErr       bool
	}{
		{
			name:          "found upcoming",
			withinMinutes: 30,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-1", "user-1", "lab-1", "Lab 1",
					nil, "confirmed", fixedTime, endTime,
					120, 4, 8, 50,
					"", nil, nil,
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WithArgs(30).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:          "database error",
			withinMinutes: 30,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WillReturnError(errors.New("connection error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetUpcoming(context.Background(), tt.withinMinutes)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestReservationRepo_GetExpired(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := fixedTime.Add(2 * time.Hour)

	tests := []struct {
		name      string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "found expired",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				}).AddRow(
					"res-1", "user-1", "lab-1", "Lab 1",
					"pod-1", "active", fixedTime, endTime,
					120, 4, 8, 50,
					"", nil, nil,
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM reservations r .+ WHERE .+ AND r.end_time < NOW\(\)`).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "no expired",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "lab_template_id", "lab_template_name",
					"pod_id", "status", "start_time", "end_time",
					"duration_minutes", "cpu_cores", "memory_gb", "storage_gb",
					"notes", "cancel_reason", "cancelled_at",
					"created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM reservations r`).
					WillReturnError(errors.New("connection error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewReservationRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetExpired(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func TestNewReservationRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewReservationRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func reservationContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
