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

func TestLabTemplateRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		record  *models.LabTemplateRecord
		mockFn  func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			record: &models.LabTemplateRecord{
				Name:            "Test Lab",
				Slug:            "test-lab",
				Description:     "A test lab",
				Version:         "1.0.0",
				Platform:        models.PlatformProxmox,
				DurationMinutes: 60,
				Difficulty:      "intermediate",
				MaxPoints:       100,
				PassThreshold:   70,
				Spec:            json.RawMessage(`{"vms":[]}`),
				Checkpoints:     json.RawMessage(`[]`),
				IsActive:        true,
				OrganizationID:  stringPtr("org-123"),
				Visibility:      "organization",
				CreatedBy:       stringPtr("user-123"),
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
					AddRow("template-123", fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO lab_templates`).
					WithArgs(
						record.Name,
						record.Slug,
						sqlmock.AnyArg(), // description (NullString)
						record.Version,
						record.Platform,
						sqlmock.AnyArg(), // duration_minutes (NullInt64)
						sqlmock.AnyArg(), // difficulty (NullString)
						record.MaxPoints,
						record.PassThreshold,
						record.Spec,
						record.Checkpoints,
						record.IsActive,
						sqlmock.AnyArg(), // organization_id (NullString)
						sqlmock.AnyArg(), // visibility
						sqlmock.AnyArg(), // created_by (NullString)
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success with minimal fields",
			record: &models.LabTemplateRecord{
				Name:          "Minimal Lab",
				Slug:          "minimal-lab",
				Version:       "1.0.0",
				Platform:      models.PlatformCloudStack,
				MaxPoints:     50,
				PassThreshold: 40,
				Spec:          json.RawMessage(`{}`),
				Checkpoints:   json.RawMessage(`[]`),
				IsActive:      false,
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
					AddRow("template-456", fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO lab_templates`).
					WithArgs(
						record.Name,
						record.Slug,
						sqlmock.AnyArg(), // description
						record.Version,
						record.Platform,
						sqlmock.AnyArg(), // duration_minutes
						sqlmock.AnyArg(), // difficulty
						record.MaxPoints,
						record.PassThreshold,
						record.Spec,
						record.Checkpoints,
						record.IsActive,
						sqlmock.AnyArg(), // organization_id
						sqlmock.AnyArg(), // visibility
						sqlmock.AnyArg(), // created_by
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			record: &models.LabTemplateRecord{
				Name:          "Error Lab",
				Version:       "1.0.0",
				Platform:      models.PlatformProxmox,
				MaxPoints:     100,
				PassThreshold: 70,
				Spec:          json.RawMessage(`{}`),
				Checkpoints:   json.RawMessage(`[]`),
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				mock.ExpectQuery(`INSERT INTO lab_templates`).
					WillReturnError(errors.New("duplicate key violation"))
			},
			wantErr: true,
			errMsg:  "inserting lab template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock, tt.record)

			err = repo.Create(context.Background(), tt.record)

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
				if tt.record.ID == "" {
					t.Error("expected ID to be set")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLabTemplateRepo_GetByID(t *testing.T) {
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
			name: "existing template",
			id:   "template-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}).AddRow(
					"template-123", "Test Lab", "test-lab", "A test lab", "1.0.0", "proxmox", 60,
					"intermediate", "security", []byte(`["linux", "networking"]`), 100, 70, json.RawMessage(`{"vms":[]}`), json.RawMessage(`[]`), []byte(`{}`),
					fixedTime, fixedTime, true,
					nil, "global", nil, "community",
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE id = \$1`).
					WithArgs("template-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent template",
			id:   "template-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE id = \$1`).
					WithArgs("template-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false, // Repository returns (nil, nil) for not found
			errMsg:  "",
		},
		{
			name: "database error",
			id:   "template-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE id = \$1`).
					WithArgs("template-456").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "getting lab template by ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
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
					t.Error("expected record, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLabTemplateRepo_GetByName(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		labName string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name:    "existing template",
			labName: "Test Lab",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}).AddRow(
					"template-123", "Test Lab", "test-lab", "A test lab", "1.0.0", "proxmox", 60,
					"intermediate", "security", []byte(`["linux", "networking"]`), 100, 70, json.RawMessage(`{"vms":[]}`), json.RawMessage(`[]`), []byte(`{}`),
					fixedTime, fixedTime, true,
					nil, "global", nil, "community",
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE name = \$1`).
					WithArgs("Test Lab").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "non-existent template",
			labName: "Unknown Lab",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE name = \$1`).
					WithArgs("Unknown Lab").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false, // Repository returns (nil, nil) for not found
			errMsg:  "",
		},
		{
			name:    "database error",
			labName: "Error Lab",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE name = \$1`).
					WithArgs("Error Lab").
					WillReturnError(errors.New("timeout"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "getting lab template by name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByName(context.Background(), tt.labName)

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
					t.Error("expected record, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLabTemplateRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	trueVal := true

	tests := []struct {
		name      string
		filter    LabTemplateFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
		errMsg    string
	}{
		{
			name:   "no filter",
			filter: LabTemplateFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}).
					AddRow("t1", "Lab A", "lab-a", "Desc A", "1.0", "proxmox", 30, "easy", "security", []byte(`[]`), 50, 40, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true, nil, "global", nil, "community").
					AddRow("t2", "Lab B", "lab-b", "Desc B", "1.0", "cloudstack", 60, "hard", "networking", []byte(`[]`), 100, 80, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true, nil, "global", nil, "community")
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE 1=1 ORDER BY name ASC LIMIT \$1`).
					WithArgs(100).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "filter by platform",
			filter: LabTemplateFilter{Platform: "proxmox"},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}).
					AddRow("t1", "Proxmox Lab", "proxmox-lab", "Desc", "1.0", "proxmox", 30, "easy", "security", []byte(`[]`), 50, 40, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true, nil, "global", nil, "community")
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE 1=1 AND platform = \$1 ORDER BY name ASC LIMIT \$2`).
					WithArgs("proxmox", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "filter by active",
			filter: LabTemplateFilter{Active: &trueVal},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}).
					AddRow("t1", "Active Lab", "active-lab", "Desc", "1.0", "proxmox", 30, "easy", "security", []byte(`[]`), 50, 40, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true, nil, "global", nil, "community")
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE 1=1 AND is_active = \$1 ORDER BY name ASC LIMIT \$2`).
					WithArgs(true, 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "with pagination",
			filter: LabTemplateFilter{Limit: 10, Offset: 5},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				})
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE 1=1 ORDER BY name ASC LIMIT \$1 OFFSET \$2`).
					WithArgs(10, 5).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			filter: LabTemplateFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_templates`).
					WillReturnError(errors.New("connection lost"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "listing lab templates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock)

			got, err := repo.List(context.Background(), tt.filter)

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
					t.Errorf("got %d records, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLabTemplateRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		record  *models.LabTemplateRecord
		mockFn  func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			record: &models.LabTemplateRecord{
				ID:              "template-123",
				Name:            "Updated Lab",
				Slug:            "updated-lab",
				Description:     "Updated description",
				Version:         "2.0.0",
				Platform:        models.PlatformProxmox,
				DurationMinutes: 90,
				Difficulty:      "advanced",
				MaxPoints:       150,
				PassThreshold:   100,
				Spec:            json.RawMessage(`{"vms":["vm1"]}`),
				Checkpoints:     json.RawMessage(`[{"id":"cp1"}]`),
				IsActive:        true,
				OrganizationID:  stringPtr("org-123"),
				Visibility:      "organization",
				CreatedBy:       stringPtr("user-123"),
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				mock.ExpectBegin()
				// SELECT FOR UPDATE to snapshot existing state
				existingCols := []string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}
				existingRows := sqlmock.NewRows(existingCols).AddRow(
					"template-123", "Old Lab", "old-lab", "", "1.0.0", "proxmox", nil,
					nil, nil, []byte(`[]`), 100, 70, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true,
					nil, "global", nil, "community",
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE id = \$1`).
					WithArgs(record.ID).
					WillReturnRows(existingRows)
				// Version snapshot INSERT
				versionRows := sqlmock.NewRows([]string{"id", "version_number", "created_at"}).
					AddRow("ver-1", 1, fixedTime)
				mock.ExpectQuery(`INSERT INTO lab_template_versions`).
					WillReturnRows(versionRows)
				// Actual UPDATE
				rows := sqlmock.NewRows([]string{"updated_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`UPDATE lab_templates SET`).
					WithArgs(
						record.ID,
						record.Name,
						record.Slug,
						sqlmock.AnyArg(), // description
						record.Version,
						record.Platform,
						sqlmock.AnyArg(), // duration_minutes
						sqlmock.AnyArg(), // difficulty
						record.MaxPoints,
						record.PassThreshold,
						record.Spec,
						record.Checkpoints,
						sqlmock.AnyArg(), // instructions
						record.IsActive,
						sqlmock.AnyArg(), // organization_id
						sqlmock.AnyArg(), // visibility
						sqlmock.AnyArg(), // created_by
					).
					WillReturnRows(rows)
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name: "database error",
			record: &models.LabTemplateRecord{
				ID:            "template-123",
				Name:          "Error Lab",
				Version:       "1.0.0",
				Platform:      models.PlatformProxmox,
				MaxPoints:     100,
				PassThreshold: 70,
				Spec:          json.RawMessage(`{}`),
				Checkpoints:   json.RawMessage(`[]`),
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
			},
			wantErr: true,
			errMsg:  "beginning update transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock, tt.record)

			err = repo.Update(context.Background(), tt.record)

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

func TestLabTemplateRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "template-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM lab_templates WHERE id = \$1`).
					WithArgs("template-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "non-existent template",
			id:   "template-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM lab_templates WHERE id = \$1`).
					WithArgs("template-nonexistent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "no rows",
		},
		{
			name: "database error",
			id:   "template-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM lab_templates WHERE id = \$1`).
					WithArgs("template-456").
					WillReturnError(errors.New("foreign key constraint"))
			},
			wantErr: true,
			errMsg:  "deleting lab template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

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

func TestLabTemplateRepo_SetActive(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		active  bool
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "set active true",
			id:     "template-123",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_templates SET is_active = \$2 WHERE id = \$1`).
					WithArgs("template-123", true).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "set active false",
			id:     "template-123",
			active: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_templates SET is_active = \$2 WHERE id = \$1`).
					WithArgs("template-123", false).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			id:     "template-456",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_templates SET is_active = \$2 WHERE id = \$1`).
					WithArgs("template-456", true).
					WillReturnError(errors.New("connection refused"))
			},
			wantErr: true,
			errMsg:  "setting lab template active status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock)

			err = repo.SetActive(context.Background(), tt.id, tt.active)

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

func TestLabTemplateRepo_Upsert(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		record  *models.LabTemplateRecord
		mockFn  func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord)
		wantErr bool
		errMsg  string
	}{
		{
			name: "create new template",
			record: &models.LabTemplateRecord{
				Name:          "New Lab",
				Slug:          "new-lab",
				Version:       "1.0.0",
				Platform:      models.PlatformProxmox,
				MaxPoints:     100,
				PassThreshold: 70,
				Spec:          json.RawMessage(`{}`),
				Checkpoints:   json.RawMessage(`[]`),
				IsActive:      true,
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				// GetByName returns no rows (empty result set)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE name = \$1`).
					WithArgs("New Lab").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "slug", "description", "version", "platform", "duration_minutes",
						"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
						"created_at", "updated_at", "is_active",
						"organization_id", "visibility", "created_by", "min_edition",
					}))
				// GetBySlug returns no rows (empty result set)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE slug = \$1`).
					WithArgs("new-lab").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "slug", "description", "version", "platform", "duration_minutes",
						"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
						"created_at", "updated_at", "is_active",
						"organization_id", "visibility", "created_by", "min_edition",
					}))
				// Create is called - expect INSERT
				insertRows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
					AddRow("template-new", fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO lab_templates`).
					WillReturnRows(insertRows)
			},
			wantErr: false,
		},
		{
			name: "update existing template",
			record: &models.LabTemplateRecord{
				Name:          "Existing Lab",
				Slug:          "existing-lab",
				Version:       "2.0.0",
				Platform:      models.PlatformProxmox,
				MaxPoints:     100,
				PassThreshold: 70,
				Spec:          json.RawMessage(`{}`),
				Checkpoints:   json.RawMessage(`[]`),
				IsActive:      true,
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				templateCols := []string{
					"id", "name", "slug", "description", "version", "platform", "duration_minutes",
					"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
					"created_at", "updated_at", "is_active",
					"organization_id", "visibility", "created_by", "min_edition",
				}
				// GetByName returns existing
				existingRows := sqlmock.NewRows(templateCols).AddRow(
					"template-existing", "Existing Lab", "existing-lab", "", "1.0.0", "proxmox", nil,
					nil, nil, []byte(`[]`), 100, 70, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true,
					nil, "public", nil, "community",
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE name = \$1`).
					WithArgs("Existing Lab").
					WillReturnRows(existingRows)
				// Update starts a transaction
				mock.ExpectBegin()
				// SELECT FOR UPDATE inside transaction
				forUpdateRows := sqlmock.NewRows(templateCols).AddRow(
					"template-existing", "Existing Lab", "existing-lab", "", "1.0.0", "proxmox", nil,
					nil, nil, []byte(`[]`), 100, 70, []byte(`{}`), []byte(`[]`), []byte(`{}`), fixedTime, fixedTime, true,
					nil, "public", nil, "community",
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE id = \$1`).
					WithArgs("template-existing").
					WillReturnRows(forUpdateRows)
				// Version snapshot INSERT
				versionRows := sqlmock.NewRows([]string{"id", "version_number", "created_at"}).
					AddRow("ver-1", 1, fixedTime)
				mock.ExpectQuery(`INSERT INTO lab_template_versions`).
					WillReturnRows(versionRows)
				// Actual UPDATE
				updateRows := sqlmock.NewRows([]string{"updated_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`UPDATE lab_templates SET`).
					WillReturnRows(updateRows)
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name: "error checking existing",
			record: &models.LabTemplateRecord{
				Name:          "Error Lab",
				Version:       "1.0.0",
				Platform:      models.PlatformProxmox,
				MaxPoints:     100,
				PassThreshold: 70,
				Spec:          json.RawMessage(`{}`),
				Checkpoints:   json.RawMessage(`[]`),
			},
			mockFn: func(mock sqlmock.Sqlmock, record *models.LabTemplateRecord) {
				mock.ExpectQuery(`SELECT .+ FROM lab_templates WHERE name = \$1`).
					WithArgs("Error Lab").
					WillReturnError(errors.New("connection timeout"))
			},
			wantErr: true,
			errMsg:  "checking existing template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLabTemplateRepo(db)
			tt.mockFn(mock, tt.record)

			err = repo.Upsert(context.Background(), tt.record)

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

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
