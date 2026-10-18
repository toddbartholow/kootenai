package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestAuditLogRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		entry   *models.AuditEntry
		mockFn  func(mock sqlmock.Sqlmock, entry *models.AuditEntry)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			entry: &models.AuditEntry{
				ActorID:      "user-123",
				ActorType:    "user",
				Action:       "create_pod",
				ResourceType: "pod",
				ResourceID:   "pod-456",
				Details:      json.RawMessage(`{"template":"basic-lab"}`),
				IPAddress:    "192.168.1.1",
				UserAgent:    "Mozilla/5.0",
			},
			mockFn: func(mock sqlmock.Sqlmock, entry *models.AuditEntry) {
				rows := sqlmock.NewRows([]string{"id", "timestamp"}).
					AddRow(int64(1), fixedTime)
				mock.ExpectQuery(`INSERT INTO audit_log`).
					WithArgs(
						entry.ActorID,
						entry.ActorType,
						entry.Action,
						entry.ResourceType,
						entry.ResourceID,
						entry.Details,
						entry.IPAddress,
						entry.UserAgent,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success with minimal fields",
			entry: &models.AuditEntry{
				ActorID:      "system",
				ActorType:    "system",
				Action:       "cleanup",
				ResourceType: "session",
			},
			mockFn: func(mock sqlmock.Sqlmock, entry *models.AuditEntry) {
				rows := sqlmock.NewRows([]string{"id", "timestamp"}).
					AddRow(int64(2), fixedTime)
				mock.ExpectQuery(`INSERT INTO audit_log`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			entry: &models.AuditEntry{
				ActorID:      "user-err",
				ActorType:    "user",
				Action:       "login",
				ResourceType: "auth",
			},
			mockFn: func(mock sqlmock.Sqlmock, entry *models.AuditEntry) {
				mock.ExpectQuery(`INSERT INTO audit_log`).
					WillReturnError(errors.New("insertion failed"))
			},
			wantErr: true,
			errMsg:  "inserting audit entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAuditLogRepo(db)
			tt.mockFn(mock, tt.entry)

			err = repo.Create(context.Background(), tt.entry)

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

func TestAuditLogRepo_Query(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		filter    AuditFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
		errMsg    string
	}{
		{
			name:   "no filter",
			filter: AuditFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "actor_id", "actor_type", "action", "resource_type",
					"resource_id", "details", "ip_address", "user_agent",
				}).
					AddRow(int64(1), fixedTime, "user-1", "user", "login", "auth", nil, nil, "192.168.1.1", "Browser").
					AddRow(int64(2), fixedTime, "user-2", "user", "create_pod", "pod", "pod-1", []byte(`{}`), nil, nil)
				mock.ExpectQuery(`SELECT .+ FROM audit_log WHERE 1=1 ORDER BY timestamp DESC LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "filter by actor",
			filter: AuditFilter{ActorID: "user-123"},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "actor_id", "actor_type", "action", "resource_type",
					"resource_id", "details", "ip_address", "user_agent",
				}).
					AddRow(int64(1), fixedTime, "user-123", "user", "login", "auth", nil, nil, nil, nil)
				mock.ExpectQuery(`SELECT .+ FROM audit_log WHERE 1=1 AND actor_id = \$1 ORDER BY timestamp DESC LIMIT \$2`).
					WithArgs("user-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "filter by action",
			filter: AuditFilter{Action: "create_pod"},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "actor_id", "actor_type", "action", "resource_type",
					"resource_id", "details", "ip_address", "user_agent",
				})
				mock.ExpectQuery(`SELECT .+ FROM audit_log WHERE 1=1 AND action = \$1 ORDER BY timestamp DESC LIMIT \$2`).
					WithArgs("create_pod", 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "filter by resource type and ID",
			filter: AuditFilter{ResourceType: "pod", ResourceID: "pod-456"},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "actor_id", "actor_type", "action", "resource_type",
					"resource_id", "details", "ip_address", "user_agent",
				}).
					AddRow(int64(1), fixedTime, "user-1", "user", "delete_pod", "pod", "pod-456", nil, nil, nil)
				mock.ExpectQuery(`SELECT .+ FROM audit_log WHERE 1=1 AND resource_type = \$1 AND resource_id = \$2 ORDER BY timestamp DESC LIMIT \$3`).
					WithArgs("pod", "pod-456", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "with pagination",
			filter: AuditFilter{Limit: 10, Offset: 5},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "actor_id", "actor_type", "action", "resource_type",
					"resource_id", "details", "ip_address", "user_agent",
				})
				mock.ExpectQuery(`SELECT .+ FROM audit_log WHERE 1=1 ORDER BY timestamp DESC LIMIT \$1 OFFSET \$2`).
					WithArgs(10, 5).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "filter by time range",
			filter: AuditFilter{StartTime: &fixedTime, EndTime: &fixedTime},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "actor_id", "actor_type", "action", "resource_type",
					"resource_id", "details", "ip_address", "user_agent",
				})
				mock.ExpectQuery(`SELECT .+ FROM audit_log WHERE 1=1 AND timestamp >= \$1 AND timestamp <= \$2 ORDER BY timestamp DESC LIMIT \$3`).
					WithArgs(fixedTime, fixedTime, 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			filter: AuditFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM audit_log`).
					WillReturnError(errors.New("connection lost"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "querying audit logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAuditLogRepo(db)
			tt.mockFn(mock)

			got, err := repo.Query(context.Background(), tt.filter)

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
				if len(got) != tt.wantCount {
					t.Errorf("got %d entries, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
