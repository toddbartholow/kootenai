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

func TestPodRepo_Create(t *testing.T) {
	tests := []struct {
		name    string
		pod     *models.Pod
		mockFn  func(mock sqlmock.Sqlmock, pod *models.Pod)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			pod: &models.Pod{
				ID:            "pod-123",
				Name:          "Test Pod",
				LabTemplateID: "template-uuid-1",
				LabTemplate:   "template-1",
				OwnerID:       "user-uuid-1",
				Owner:         "user-1",
				Platform:      "proxmox",
				Status:        models.PodStatusRunning,
				VMs:           []models.PodVM{{Name: "vm1", PlatformID: "100"}},
				Networks:      []models.PodNetwork{{Name: "net1", VLAN: 100}},
				Metadata:      map[string]string{"key": "value"},
			},
			mockFn: func(mock sqlmock.Sqlmock, pod *models.Pod) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now())
				mock.ExpectQuery(`INSERT INTO pods`).
					WithArgs(pod.ID, sqlmock.AnyArg(), pod.LabTemplateID, pod.OwnerID, pod.Platform, pod.Status,
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg()). // name, vms, networks, expires_at, metadata, organization_id, team_id
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success with nil expiration",
			pod: &models.Pod{
				ID:            "pod-456",
				Name:          "Test Pod 2",
				LabTemplateID: "template-uuid-2",
				LabTemplate:   "template-2",
				OwnerID:       "user-uuid-2",
				Owner:         "user-2",
				Platform:      "cloudstack",
				Status:        models.PodStatusProvisioning,
				VMs:           []models.PodVM{},
				Networks:      []models.PodNetwork{},
				ExpiresAt:     nil,
			},
			mockFn: func(mock sqlmock.Sqlmock, pod *models.Pod) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now())
				mock.ExpectQuery(`INSERT INTO pods`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()). // 12 args total
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			pod: &models.Pod{
				ID:            "pod-789",
				Name:          "Test Pod 3",
				LabTemplateID: "template-uuid-3",
				LabTemplate:   "template-3",
				OwnerID:       "user-uuid-3",
				Owner:         "user-3",
				Platform:      "proxmox",
				Status:        models.PodStatusRunning,
				VMs:           []models.PodVM{},
				Networks:      []models.PodNetwork{},
			},
			mockFn: func(mock sqlmock.Sqlmock, pod *models.Pod) {
				mock.ExpectQuery(`INSERT INTO pods`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()). // 12 args total
					WillReturnError(errors.New("connection refused"))
			},
			wantErr: true,
			errMsg:  "inserting pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPodRepo(db)
			tt.mockFn(mock, tt.pod)

			err = repo.Create(context.Background(), tt.pod)

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

func TestPodRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.Add(time.Hour)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		want    *models.Pod
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing pod",
			id:   "pod-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				vms, _ := json.Marshal([]models.PodVM{{Name: "vm1", PlatformID: "100"}})
				networks, _ := json.Marshal([]models.PodNetwork{{Name: "net1", VLAN: 100}})
				metadata, _ := json.Marshal(map[string]string{"key": "value"})

				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id", "lab_template_name", "owner_name",
				}).AddRow(
					"pod-123", "Test Pod", "template-uuid-1", "user-uuid-1", "proxmox", models.PodStatusRunning,
					vms, networks, fixedTime, expiresAt, metadata,
					nil, nil, "Test Template", "testuser",
				)
				mock.ExpectQuery(`SELECT .+ FROM pods p`).
					WithArgs("pod-123").
					WillReturnRows(rows)
			},
			want: &models.Pod{
				ID:          "pod-123",
				Name:        "Test Pod",
				LabTemplate: "Test Template",
				Owner:       "testuser",
				Platform:    "proxmox",
				Status:      models.PodStatusRunning,
			},
			wantErr: false,
		},
		{
			name: "non-existent pod",
			id:   "pod-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pods p`).
					WithArgs("pod-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "pod-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pods p`).
					WithArgs("pod-456").
					WillReturnError(errors.New("connection refused"))
			},
			want:    nil,
			wantErr: true,
			errMsg:  "querying pod",
		},
		{
			name: "invalid JSON in VMs",
			id:   "pod-bad-json",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id", "lab_template_name", "owner_name",
				}).AddRow(
					"pod-bad-json", "Bad Pod", "template-1", "user-1", "proxmox", models.PodStatusRunning,
					[]byte("invalid json"), []byte("[]"), fixedTime, nil, []byte("{}"),
					nil, nil, "Test Template", "testuser",
				)
				mock.ExpectQuery(`SELECT .+ FROM pods p`).
					WithArgs("pod-bad-json").
					WillReturnRows(rows)
			},
			want:    nil,
			wantErr: true,
			errMsg:  "unmarshaling VMs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPodRepo(db)
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
				if tt.want == nil && got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				if tt.want != nil && got == nil {
					t.Error("expected pod, got nil")
				}
				if tt.want != nil && got != nil {
					if got.ID != tt.want.ID {
						t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
					}
					if got.Status != tt.want.Status {
						t.Errorf("Status = %v, want %v", got.Status, tt.want.Status)
					}
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPodRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		filter  PodFilter
		mockFn  func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "no filter",
			filter: PodFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				vms, _ := json.Marshal([]models.PodVM{})
				networks, _ := json.Marshal([]models.PodNetwork{})
				metadata, _ := json.Marshal(map[string]string{})

				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id", "lab_template_name", "owner_name",
				}).
					AddRow("pod-1", "Pod 1", "t1", "u1", "proxmox", "running", vms, networks, fixedTime, nil, metadata, nil, nil, "Template 1", "user1").
					AddRow("pod-2", "Pod 2", "t2", "u2", "cloudstack", "running", vms, networks, fixedTime, nil, metadata, nil, nil, "Template 2", "user2")

				mock.ExpectQuery(`SELECT .+ FROM pods p .+ LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:   "filter by owner",
			filter: PodFilter{OwnerID: "user-1"},
			mockFn: func(mock sqlmock.Sqlmock) {
				vms, _ := json.Marshal([]models.PodVM{})
				networks, _ := json.Marshal([]models.PodNetwork{})
				metadata, _ := json.Marshal(map[string]string{})

				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id", "lab_template_name", "owner_name",
				}).
					AddRow("pod-1", "Pod 1", "t1", "user-1", "proxmox", "running", vms, networks, fixedTime, nil, metadata, nil, nil, "Template 1", "user1")

				mock.ExpectQuery(`SELECT .+ FROM pods p .+ LIMIT \$2`).
					WithArgs("user-1", 100).
					WillReturnRows(rows)
			},
			want:    1,
			wantErr: false,
		},
		{
			name:   "filter by status",
			filter: PodFilter{Status: models.PodStatusRunning},
			mockFn: func(mock sqlmock.Sqlmock) {
				vms, _ := json.Marshal([]models.PodVM{})
				networks, _ := json.Marshal([]models.PodNetwork{})
				metadata, _ := json.Marshal(map[string]string{})

				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id", "lab_template_name", "owner_name",
				}).
					AddRow("pod-1", "Pod 1", "t1", "u1", "proxmox", "running", vms, networks, fixedTime, nil, metadata, nil, nil, "Template 1", "user1")

				mock.ExpectQuery(`SELECT .+ FROM pods p .+ LIMIT \$2`).
					WithArgs(models.PodStatusRunning, 100).
					WillReturnRows(rows)
			},
			want:    1,
			wantErr: false,
		},
		{
			name:   "with pagination",
			filter: PodFilter{Limit: 10, Offset: 5},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id", "lab_template_name", "owner_name",
				})
				mock.ExpectQuery(`SELECT .+ FROM pods p`).
					WithArgs(10, 5).
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name:   "database error",
			filter: PodFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pods p`).
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

			repo := NewPodRepo(db)
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
					t.Errorf("got %d pods, want %d", len(got), tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPodRepo_Update(t *testing.T) {
	tests := []struct {
		name    string
		pod     *models.Pod
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			pod: &models.Pod{
				ID:       "pod-123",
				Status:   models.PodStatusRunning,
				VMs:      []models.PodVM{{Name: "vm1"}},
				Networks: []models.PodNetwork{{Name: "net1"}},
				Metadata: map[string]string{"updated": "true"},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pods SET status = \$2, vms = \$3, networks = \$4, expires_at = \$5, metadata = \$6, organization_id = \$7, team_id = \$8 WHERE id = \$1`).
					WithArgs("pod-123", models.PodStatusRunning, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			pod: &models.Pod{
				ID:       "pod-456",
				Status:   models.PodStatusRunning,
				VMs:      []models.PodVM{},
				Networks: []models.PodNetwork{},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pods`).
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

			repo := NewPodRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.pod)

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

func TestPodRepo_UpdateStatus(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		status  models.PodStatus
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success",
			id:     "pod-123",
			status: models.PodStatusDestroyed,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pods SET status = \$2 WHERE id = \$1`).
					WithArgs("pod-123", models.PodStatusDestroyed).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			id:     "pod-456",
			status: models.PodStatusRunning,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pods SET status`).
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

			repo := NewPodRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateStatus(context.Background(), tt.id, tt.status)

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

func TestPodRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "pod-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pods WHERE id = \$1`).
					WithArgs("pod-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "non-existent pod returns error",
			id:   "pod-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pods WHERE id = \$1`).
					WithArgs("pod-nonexistent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
		},
		{
			name: "database error",
			id:   "pod-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pods`).
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

			repo := NewPodRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

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

func TestPodRepo_GetExpired(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		mockFn  func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "has expired pods",
			mockFn: func(mock sqlmock.Sqlmock) {
				vms, _ := json.Marshal([]models.PodVM{})
				networks, _ := json.Marshal([]models.PodNetwork{})
				metadata, _ := json.Marshal(map[string]string{})
				expiredTime := fixedTime.Add(-time.Hour)

				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id",
				}).
					AddRow("pod-1", "Pod 1", "t1", "u1", "proxmox", "running", vms, networks, fixedTime, expiredTime, metadata, nil, nil).
					AddRow("pod-2", "Pod 2", "t2", "u2", "cloudstack", "running", vms, networks, fixedTime, expiredTime, metadata, nil, nil)

				mock.ExpectQuery(`SELECT .+ FROM pods WHERE expires_at IS NOT NULL AND expires_at < \$1`).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(rows)
			},
			want:    2,
			wantErr: false,
		},
		{
			name: "no expired pods",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "lab_template_id", "owner_id", "platform", "status",
					"vms", "networks", "created_at", "expires_at", "metadata",
					"organization_id", "team_id",
				})
				mock.ExpectQuery(`SELECT .+ FROM pods WHERE expires_at IS NOT NULL`).
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "database error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pods`).
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

			repo := NewPodRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetExpired(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.want {
					t.Errorf("got %d pods, want %d", len(got), tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// CountActive was the only PodRepo method without a sqlmock test, and the only
// one whose predicate changed. The predicate is the whole contract here: it
// decides what labctl_pods_active counts, so pin the SQL itself.
func TestPodRepo_CountActive(t *testing.T) {
	tests := []struct {
		name    string
		mockFn  func(mock sqlmock.Sqlmock)
		want    int64
		wantErr bool
	}{
		{
			name: "counts every pod that is not destroyed",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Anchored on the exact predicate: a destroying pod is
				// mid-teardown and still holds VMs, so it must be counted.
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pods WHERE status <> 'destroyed'`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
			},
			want: 7,
		},
		{
			name: "zero when nothing is active",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pods WHERE status <> 'destroyed'`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			want: 0,
		},
		{
			name: "query error is wrapped, not swallowed",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pods`).
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

			repo := NewPodRepo(db)
			tt.mockFn(mock)

			got, err := repo.CountActive(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("CountActive: %v", err)
				}
				if got != tt.want {
					t.Errorf("CountActive = %d, want %d", got, tt.want)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}
