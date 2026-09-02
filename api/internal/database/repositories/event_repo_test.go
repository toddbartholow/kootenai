package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestEventRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		event   *models.Event
		mockFn  func(mock sqlmock.Sqlmock, event *models.Event)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			event: &models.Event{
				PodID:         "pod-123",
				SessionID:     "session-1",
				VMName:        "attacker",
				AgentID:       "agent-001",
				EventType:     "wazuh",
				WazuhRuleDesc: "File integrity check",
				Data:          []byte(`{"alert":"test"}`),
			},
			mockFn: func(mock sqlmock.Sqlmock, event *models.Event) {
				rows := sqlmock.NewRows([]string{"id", "timestamp"}).
					AddRow(int64(1), fixedTime)
				mock.ExpectQuery(`INSERT INTO events`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			event: &models.Event{
				PodID:     "pod-err",
				VMName:    "vm1",
				AgentID:   "agent-001",
				EventType: "wazuh",
			},
			mockFn: func(mock sqlmock.Sqlmock, event *models.Event) {
				mock.ExpectQuery(`INSERT INTO events`).
					WillReturnError(errors.New("insertion failed"))
			},
			wantErr: true,
			errMsg:  "inserting event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEventRepo(db)
			tt.mockFn(mock, tt.event)

			err = repo.Create(context.Background(), tt.event)

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

func TestEventRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      int64
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing event",
			id:   1,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				}).AddRow(
					int64(1), fixedTime, "pod-123", "session-1", "attacker", "agent-001",
					"wazuh", int64(1001), int64(5), "Test rule", []byte(`{}`),
					false, pq.StringArray{"cp1", "cp2"},
				)
				mock.ExpectQuery(`SELECT .+ FROM events WHERE id = \$1`).
					WithArgs(int64(1)).
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent event",
			id:   999,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM events WHERE id = \$1`).
					WithArgs(int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   2,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM events WHERE id = \$1`).
					WithArgs(int64(2)).
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "scanning event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEventRepo(db)
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
					t.Error("expected event, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestEventRepo_GetByPodID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		podID     string
		limit     int
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "multiple events",
			podID: "pod-123",
			limit: 10,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				}).
					AddRow(int64(1), fixedTime, "pod-123", "session-1", "vm1", "agent-001", "wazuh", nil, nil, nil, []byte(`{}`), false, pq.StringArray{}).
					AddRow(int64(2), fixedTime, "pod-123", "session-1", "vm2", "agent-002", "wazuh", nil, nil, nil, []byte(`{}`), true, pq.StringArray{"cp1"})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE pod_id = \$1`).
					WithArgs("pod-123", 10).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "no events",
			podID: "pod-empty",
			limit: 10,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE pod_id = \$1`).
					WithArgs("pod-empty", 10).
					WillReturnRows(rows)
			},
			wantCount: 0,
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

			repo := NewEventRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByPodID(context.Background(), tt.podID, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d events, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestEventRepo_GetBySessionID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		sessionID string
		limit     int
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "has events",
			sessionID: "session-1",
			limit:     10,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				}).
					AddRow(int64(1), fixedTime, "pod-123", "session-1", "vm1", "agent-001", "wazuh", nil, nil, nil, []byte(`{}`), false, pq.StringArray{})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE session_id = \$1`).
					WithArgs("session-1", 10).
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

			repo := NewEventRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetBySessionID(context.Background(), tt.sessionID, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d events, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestEventRepo_GetUnprocessed(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		limit     int
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "has unprocessed events",
			limit: 100,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				}).
					AddRow(int64(1), fixedTime, "pod-1", nil, "vm1", "agent-001", "wazuh", nil, nil, nil, []byte(`{}`), false, pq.StringArray{}).
					AddRow(int64(2), fixedTime, "pod-2", nil, "vm2", "agent-002", "wazuh", nil, nil, nil, []byte(`{}`), false, pq.StringArray{})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE processed = false`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "no unprocessed events",
			limit: 100,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE processed = false`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			wantCount: 0,
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

			repo := NewEventRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetUnprocessed(context.Background(), tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d events, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestEventRepo_MarkProcessed(t *testing.T) {
	tests := []struct {
		name               string
		id                 int64
		matchedCheckpoints []string
		mockFn             func(mock sqlmock.Sqlmock)
		wantErr            bool
		errMsg             string
	}{
		{
			name:               "success with checkpoints",
			id:                 1,
			matchedCheckpoints: []string{"cp1", "cp2"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE events SET processed = true, matched_checkpoints = \$2 WHERE id = \$1`).
					WithArgs(int64(1), pq.Array([]string{"cp1", "cp2"})).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:               "success with empty checkpoints",
			id:                 2,
			matchedCheckpoints: []string{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE events SET processed = true, matched_checkpoints = \$2 WHERE id = \$1`).
					WithArgs(int64(2), pq.Array([]string{})).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:               "database error",
			id:                 3,
			matchedCheckpoints: nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE events SET processed = true, matched_checkpoints = \$2 WHERE id = \$1`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "marking event processed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEventRepo(db)
			tt.mockFn(mock)

			err = repo.MarkProcessed(context.Background(), tt.id, tt.matchedCheckpoints)

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

func TestEventRepo_Query(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		filter    EventFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
		errMsg    string
	}{
		{
			name:   "no filter",
			filter: EventFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				}).
					AddRow(int64(1), fixedTime, "pod-1", nil, "vm1", "agent-001", "wazuh", nil, nil, nil, []byte(`{}`), false, pq.StringArray{})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE 1=1 ORDER BY timestamp DESC LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "filter by pod",
			filter: EventFilter{PodID: "pod-123"},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE 1=1 AND pod_id = \$1 ORDER BY timestamp DESC LIMIT \$2`).
					WithArgs("pod-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "filter by event type with pagination",
			filter: EventFilter{EventType: "wazuh", Limit: 10, Offset: 5},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "timestamp", "pod_id", "session_id", "vm_name", "agent_id",
					"event_type", "rule_id", "rule_level", "description", "data",
					"processed", "matched_checkpoints",
				})
				mock.ExpectQuery(`SELECT .+ FROM events WHERE 1=1 AND event_type = \$1 ORDER BY timestamp DESC LIMIT \$2 OFFSET \$3`).
					WithArgs("wazuh", 10, 5).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			filter: EventFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM events`).
					WillReturnError(errors.New("connection lost"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "querying events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEventRepo(db)
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
					t.Errorf("got %d events, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
