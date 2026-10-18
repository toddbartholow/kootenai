package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestSessionRepo_Create(t *testing.T) {
	tests := []struct {
		name    string
		session *models.Session
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			session: &models.Session{
				ID:                 "session-123",
				PodID:              "pod-1",
				UserID:             "user-1",
				LabTemplateID:      "template-1",
				MaxPoints:          100,
				CanvasCourseID:     "course-1",
				CanvasAssignmentID: "assign-1",
				CanvasUserID:       "canvas-user-1",
				Metadata:           map[string]string{"key": "value"},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"started_at"}).AddRow(time.Now())
				mock.ExpectQuery(`INSERT INTO lab_sessions`).
					WithArgs("session-123", "pod-1", "user-1", "template-1", 100,
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), // organization_id, team_id
						sqlmock.AnyArg(), sqlmock.AnyArg(), // enrollment_id, module_id
						sqlmock.AnyArg()). // passing_threshold
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success without canvas IDs",
			session: &models.Session{
				ID:            "session-456",
				PodID:         "pod-2",
				UserID:        "user-2",
				LabTemplateID: "template-2",
				MaxPoints:     50,
				Metadata:      map[string]string{},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"started_at"}).AddRow(time.Now())
				mock.ExpectQuery(`INSERT INTO lab_sessions`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), // organization_id, team_id
						sqlmock.AnyArg(), sqlmock.AnyArg(), // enrollment_id, module_id
						sqlmock.AnyArg()). // passing_threshold
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			session: &models.Session{
				ID:            "session-789",
				PodID:         "pod-3",
				UserID:        "user-3",
				LabTemplateID: "template-3",
				MaxPoints:     75,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO lab_sessions`).
					WillReturnError(errors.New("connection refused"))
			},
			wantErr: true,
			errMsg:  "inserting session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			err = repo.Create(context.Background(), tt.session)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
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

func TestSessionRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing session",
			id:   "session-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				metadata, _ := json.Marshal(map[string]string{"key": "value"})
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				}).AddRow(
					"session-123", "pod-1", "user-1", "template-1",
					100, 85, 85.0, true,
					"course-1", "assign-1", "canvas-user-1",
					fixedTime, nil, nil, nil, nil,
					metadata, nil, nil, nil, nil,
					70,
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE id = \$1`).
					WithArgs("session-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent session",
			id:   "session-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE id = \$1`).
					WithArgs("session-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "session-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE id = \$1`).
					WithArgs("session-456").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "scanning session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByID(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				if !tt.wantNil && got == nil {
					t.Error("expected session, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSessionRepo_GetByPodID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		podID   string
		mockFn  func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:  "multiple sessions",
			podID: "pod-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				metadata, _ := json.Marshal(map[string]string{})
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				}).
					AddRow("s1", "pod-1", "u1", "t1", 100, 85, 85.0, true, nil, nil, nil, fixedTime, nil, nil, nil, nil, metadata, nil, nil, nil, nil, 70).
					AddRow("s2", "pod-1", "u2", "t1", 100, 70, 70.0, true, nil, nil, nil, fixedTime, nil, nil, nil, nil, metadata, nil, nil, nil, nil, 70)

				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE pod_id = \$1`).
					WithArgs("pod-1").
					WillReturnRows(rows)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:  "no sessions",
			podID: "pod-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				})
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE pod_id = \$1`).
					WithArgs("pod-empty").
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name:  "database error",
			podID: "pod-error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE pod_id = \$1`).
					WillReturnError(errors.New("connection refused"))
			},
			want:    0,
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

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByPodID(context.Background(), tt.podID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.want {
					t.Errorf("got %d sessions, want %d", len(got), tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSessionRepo_GetActiveByUserID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "active sessions",
			userID: "user-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				metadata, _ := json.Marshal(map[string]string{})
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				}).
					AddRow("s1", "pod-1", "user-1", "t1", 100, 0, 0, false, nil, nil, nil, fixedTime, nil, nil, nil, nil, metadata, nil, nil, nil, nil, 70)

				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE user_id = \$1 AND ended_at IS NULL`).
					WithArgs("user-1").
					WillReturnRows(rows)
			},
			want:    1,
			wantErr: false,
		},
		{
			name:   "no active sessions",
			userID: "user-2",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				})
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE user_id = \$1 AND ended_at IS NULL`).
					WithArgs("user-2").
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetActiveByUserID(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.want {
					t.Errorf("got %d sessions, want %d", len(got), tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSessionRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		filter  SessionFilter
		mockFn  func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "no filter",
			filter: SessionFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				metadata, _ := json.Marshal(map[string]string{})
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				}).
					AddRow("s1", "p1", "u1", "t1", 100, 80, 80.0, true, nil, nil, nil, fixedTime, nil, nil, nil, nil, metadata, nil, nil, nil, nil, 70).
					AddRow("s2", "p2", "u2", "t2", 100, 70, 70.0, true, nil, nil, nil, fixedTime, nil, nil, nil, nil, metadata, nil, nil, nil, nil, 70)

				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE 1=1 ORDER BY started_at DESC LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:   "filter by user",
			filter: SessionFilter{UserID: "user-1"},
			mockFn: func(mock sqlmock.Sqlmock) {
				metadata, _ := json.Marshal(map[string]string{})
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				}).
					AddRow("s1", "p1", "user-1", "t1", 100, 80, 80.0, true, nil, nil, nil, fixedTime, nil, nil, nil, nil, metadata, nil, nil, nil, nil, 70)

				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE 1=1 AND user_id = \$1 .+ LIMIT \$2`).
					WithArgs("user-1", 100).
					WillReturnRows(rows)
			},
			want:    1,
			wantErr: false,
		},
		{
			name:   "filter active only",
			filter: SessionFilter{Active: boolPtr(true)},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				})
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE 1=1 AND ended_at IS NULL .+ LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name:   "with pagination",
			filter: SessionFilter{Limit: 10, Offset: 5},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pod_id", "user_id", "lab_template_id",
					"max_points", "earned_points", "percentage", "passed",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"started_at", "ended_at", "due_at", "grade_synced_at", "grade_sync_error",
					"metadata", "organization_id", "team_id", "enrollment_id", "module_id",
					"passing_threshold",
				})
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions WHERE 1=1 ORDER BY started_at DESC LIMIT \$1 OFFSET \$2`).
					WithArgs(10, 5).
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name:   "database error",
			filter: SessionFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_sessions`).
					WillReturnError(errors.New("connection refused"))
			},
			want:    0,
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

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			got, err := repo.List(context.Background(), tt.filter)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.want {
					t.Errorf("got %d sessions, want %d", len(got), tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSessionRepo_End(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "session-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET ended_at = \$2 WHERE id = \$1`).
					WithArgs("session-123", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			id:   "session-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET ended_at`).
					WillReturnError(errors.New("connection refused"))
			},
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

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			err = repo.End(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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

func TestSessionRepo_UpdateGrade(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		earnedPoints int
		passed       bool
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "success with points",
			id:           "session-123",
			earnedPoints: 85,
			passed:       true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET earned_points = \$2`).
					WithArgs("session-123", 85, true).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "success with zero points",
			id:           "session-456",
			earnedPoints: 0,
			passed:       false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET earned_points = \$2`).
					WithArgs("session-456", 0, false).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			id:           "session-789",
			earnedPoints: 50,
			passed:       true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET earned_points`).
					WillReturnError(errors.New("connection refused"))
			},
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

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateGrade(context.Background(), tt.id, tt.earnedPoints, tt.passed)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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

func TestSessionRepo_MarkGradeSynced(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		id       string
		syncedAt time.Time
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name:     "success",
			id:       "session-123",
			syncedAt: fixedTime,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET grade_synced_at = \$2, grade_sync_error = NULL WHERE id = \$1`).
					WithArgs("session-123", fixedTime).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:     "database error",
			id:       "session-456",
			syncedAt: fixedTime,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET grade_synced_at`).
					WillReturnError(errors.New("connection refused"))
			},
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

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			err = repo.MarkGradeSynced(context.Background(), tt.id, tt.syncedAt)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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

func TestSessionRepo_MarkGradeSyncFailed(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		errorMsg string
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name:     "success",
			id:       "session-123",
			errorMsg: "Canvas API error: rate limited",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET grade_sync_error = \$2 WHERE id = \$1`).
					WithArgs("session-123", "Canvas API error: rate limited").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:     "database error",
			id:       "session-456",
			errorMsg: "some error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_sessions SET grade_sync_error`).
					WillReturnError(errors.New("connection refused"))
			},
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

			repo := NewSessionRepo(db)
			tt.mockFn(mock)

			err = repo.MarkGradeSyncFailed(context.Background(), tt.id, tt.errorMsg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
