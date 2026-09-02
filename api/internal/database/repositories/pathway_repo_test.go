package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Pathway CRUD Tests
// -----------------------------------------------------------------------------

func TestPathwayRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		pathway *models.Pathway
		mockFn  func(mock sqlmock.Sqlmock, pathway *models.Pathway)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			pathway: &models.Pathway{
				ID:          "pathway-123",
				Name:        "Network Fundamentals",
				Slug:        "network-fundamentals",
				Description: "Learn networking basics",
				Difficulty:  "beginner",
				Status:      models.PathwayStatusPublished,
				Visibility:  "global",
				Tags:        []string{"networking", "fundamentals"},
			},
			mockFn: func(mock sqlmock.Sqlmock, pathway *models.Pathway) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO pathways`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			pathway: &models.Pathway{
				ID:   "pathway-err",
				Name: "Error Pathway",
				Slug: "error-pathway",
			},
			mockFn: func(mock sqlmock.Sqlmock, pathway *models.Pathway) {
				mock.ExpectQuery(`INSERT INTO pathways`).
					WillReturnError(errors.New("duplicate slug"))
			},
			wantErr: true,
			errMsg:  "inserting pathway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPathwayRepo(db)
			tt.mockFn(mock, tt.pathway)

			err = repo.Create(context.Background(), tt.pathway)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !pathwayContainsString(err.Error(), tt.errMsg) {
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

func TestPathwayRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	prereqJSON, _ := json.Marshal([]string{})

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing pathway",
			id:   "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-123", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", true, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WithArgs("pathway-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent pathway",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				// sql.ErrNoRows is wrapped by scanPathway with %w, but errors.Is
				// can still detect it in the error chain, so GetByID returns nil, nil
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,  // nil result for not found
			wantErr: false, // no error - standard Go pattern for not found
		},
		{
			name: "database error",
			id:   "pathway-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WillReturnError(errors.New("connection error"))
			},
			wantNil: false,
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

			repo := NewPathwayRepo(db)
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

func TestPathwayRepo_GetBySlug(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	prereqJSON, _ := json.Marshal([]string{})

	tests := []struct {
		name    string
		slug    string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing pathway",
			slug: "network-fundamentals",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-123", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", false, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE slug = \$1`).
					WithArgs("network-fundamentals").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent slug",
			slug: "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				// sql.ErrNoRows is wrapped by scanPathway with %w, but errors.Is
				// can still detect it, so GetBySlug returns nil, nil
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE slug = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,  // nil result for not found
			wantErr: false, // no error - standard Go pattern for not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetBySlug(context.Background(), tt.slug)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		pathway *models.Pathway
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			pathway: &models.Pathway{
				ID:          "pathway-123",
				Name:        "Updated Network Fundamentals",
				Description: "Updated description",
				Status:      models.PathwayStatusPublished,
				Tags:        []string{"networking", "updated"},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"updated_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`UPDATE pathways`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			pathway: &models.Pathway{
				ID:   "pathway-err",
				Name: "Error Update",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`UPDATE pathways`).
					WillReturnError(errors.New("update failed"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.pathway)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pathways WHERE id = \$1`).
					WithArgs("pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			id:   "pathway-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pathways`).
					WillReturnError(errors.New("foreign key constraint"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_UpdateStatus(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		status  models.PathwayStatus
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success",
			id:     "pathway-123",
			status: models.PathwayStatusPublished,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathways SET status = \$2 WHERE id = \$1`).
					WithArgs("pathway-123", models.PathwayStatusPublished).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			id:     "pathway-err",
			status: models.PathwayStatusDraft,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathways SET status`).
					WillReturnError(errors.New("update failed"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateStatus(context.Background(), tt.id, tt.status)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// List Tests
// -----------------------------------------------------------------------------

func TestPathwayRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	prereqJSON, _ := json.Marshal([]string{})

	tests := []struct {
		name      string
		opts      models.PathwayListOptions
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "list all",
			opts: models.PathwayListOptions{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-1", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", true, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				).AddRow(
					"pathway-2", "Security Basics", "security-basics",
					"Learn security", "Security short",
					"intermediate", 30, 2,
					"published", false, nil, "global",
					nil, prereqJSON, pq.Array([]string{"security"}), "security", "#5733FF", "cover2.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways p WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "list by status",
			opts: models.PathwayListOptions{
				Status: models.PathwayStatusPublished,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-1", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", true, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways p WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "empty result",
			opts: models.PathwayListOptions{
				Difficulty: "expert",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM pathways p WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			opts: models.PathwayListOptions{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathways p WHERE`).
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.List(context.Background(), tt.opts)

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
// GetWithModules Tests
// -----------------------------------------------------------------------------

func TestPathwayRepo_GetWithModules(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	prereqJSON, _ := json.Marshal([]string{})

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "success with modules and labs",
			id:   "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// GetByID query
				pathwayRows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-123", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", true, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WithArgs("pathway-123").
					WillReturnRows(pathwayRows)

				// ListModules query
				moduleRows := sqlmock.NewRows([]string{
					"id", "pathway_id", "name", "slug", "description",
					"display_order", "unlock_type", "required_module_ids",
					"is_active", "icon", "estimated_minutes", "created_at",
				}).AddRow(
					"module-1", "pathway-123", "Module 1", "module-1", "First module",
					0, "always", pq.Array([]string{}),
					true, "icon1", 30, fixedTime,
				).AddRow(
					"module-2", "pathway-123", "Module 2", "module-2", "Second module",
					1, "sequential", pq.Array([]string{"module-1"}),
					true, "icon2", 45, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules WHERE pathway_id = \$1`).
					WithArgs("pathway-123").
					WillReturnRows(moduleRows)

				// ListModuleLabs for module-1
				labRows1 := sqlmock.NewRows([]string{
					"id", "module_id", "lab_template_id", "display_order",
					"is_required", "pass_threshold_override",
					"name", "description", "difficulty", "duration_minutes", "max_points", "platform",
				}).AddRow(
					"ml-1", "module-1", "lab-1", 0,
					true, nil,
					"Lab 1", "Lab description", "beginner", 30, 100, "proxmox",
				)
				mock.ExpectQuery(`SELECT .+ FROM module_labs ml JOIN lab_templates lt`).
					WithArgs("module-1").
					WillReturnRows(labRows1)

				// ListModuleLabs for module-2
				labRows2 := sqlmock.NewRows([]string{
					"id", "module_id", "lab_template_id", "display_order",
					"is_required", "pass_threshold_override",
					"name", "description", "difficulty", "duration_minutes", "max_points", "platform",
				}).AddRow(
					"ml-2", "module-2", "lab-2", 0,
					true, 80,
					"Lab 2", "Lab 2 description", "intermediate", 45, 150, "proxmox",
				)
				mock.ExpectQuery(`SELECT .+ FROM module_labs ml JOIN lab_templates lt`).
					WithArgs("module-2").
					WillReturnRows(labRows2)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "pathway not found",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				// sql.ErrNoRows is wrapped by scanPathway with %w, but errors.Is
				// can still detect it, so GetWithModules returns nil, nil
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,  // nil result for not found
			wantErr: false, // no error - standard Go pattern for not found
		},
		{
			name: "module list error",
			id:   "pathway-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				pathwayRows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-err", "Test Pathway", "test-pathway",
					"Test", "Test",
					"beginner", 10, 1,
					"published", false, nil, "global",
					nil, prereqJSON, pq.Array([]string{}), "", "", "",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WillReturnRows(pathwayRows)

				mock.ExpectQuery(`SELECT .+ FROM pathway_modules`).
					WillReturnError(errors.New("module query error"))
			},
			wantNil: false,
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetWithModules(context.Background(), tt.id)

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

func TestPathwayRepo_GetWithModulesBySlug(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	prereqJSON, _ := json.Marshal([]string{})

	tests := []struct {
		name    string
		slug    string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "success",
			slug: "network-fundamentals",
			mockFn: func(mock sqlmock.Sqlmock) {
				// GetBySlug query
				pathwayRows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-123", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", true, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE slug = \$1`).
					WithArgs("network-fundamentals").
					WillReturnRows(pathwayRows)

				// GetWithModules -> GetByID
				pathwayRows2 := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "short_description",
					"difficulty", "estimated_hours", "display_order",
					"status", "is_featured", "organization_id", "visibility",
					"created_by", "prerequisites", "tags", "icon", "color", "cover_image_url",
					"created_at", "updated_at",
				}).AddRow(
					"pathway-123", "Network Fundamentals", "network-fundamentals",
					"Learn networking", "Short desc",
					"beginner", 20, 1,
					"published", true, nil, "global",
					nil, prereqJSON, pq.Array([]string{"networking"}), "network", "#FF5733", "cover.jpg",
					fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE id = \$1`).
					WithArgs("pathway-123").
					WillReturnRows(pathwayRows2)

				// ListModules
				moduleRows := sqlmock.NewRows([]string{
					"id", "pathway_id", "name", "slug", "description",
					"display_order", "unlock_type", "required_module_ids",
					"is_active", "icon", "estimated_minutes", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules WHERE pathway_id = \$1`).
					WillReturnRows(moduleRows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "slug not found",
			slug: "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				// sql.ErrNoRows is wrapped by scanPathway with %w, but errors.Is
				// can still detect it, so GetWithModulesBySlug returns nil, nil
				mock.ExpectQuery(`SELECT .+ FROM pathways WHERE slug = \$1`).
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,  // nil result for not found
			wantErr: false, // no error - standard Go pattern for not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetWithModulesBySlug(context.Background(), tt.slug)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Module CRUD Tests
// -----------------------------------------------------------------------------

func TestPathwayRepo_CreateModule(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		module  *models.PathwayModule
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			module: &models.PathwayModule{
				ID:           "module-123",
				PathwayID:    "pathway-123",
				Name:         "Network Basics",
				Slug:         "network-basics",
				Description:  "Learn basic networking",
				DisplayOrder: 0,
				UnlockType:   "always",
				IsActive:     true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO pathway_modules`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			module: &models.PathwayModule{
				ID:        "module-err",
				PathwayID: "pathway-123",
				Name:      "Error Module",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO pathway_modules`).
					WillReturnError(errors.New("foreign key violation"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.CreateModule(context.Background(), tt.module)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_GetModuleByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing module",
			id:   "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pathway_id", "name", "slug", "description",
					"display_order", "unlock_type", "required_module_ids",
					"is_active", "icon", "estimated_minutes", "created_at",
				}).AddRow(
					"module-123", "pathway-123", "Module 1", "module-1", "Description",
					0, "always", pq.Array([]string{}),
					true, "icon", 30, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules WHERE id = \$1`).
					WithArgs("module-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent module",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				// sql.ErrNoRows is wrapped by scanModule with %w, but errors.Is
				// can still detect it, so GetModuleByID returns nil, nil
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules WHERE id = \$1`).
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,  // nil result for not found
			wantErr: false, // no error - standard Go pattern for not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetModuleByID(context.Background(), tt.id)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_ListModules(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		pathwayID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "multiple modules",
			pathwayID: "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pathway_id", "name", "slug", "description",
					"display_order", "unlock_type", "required_module_ids",
					"is_active", "icon", "estimated_minutes", "created_at",
				}).AddRow(
					"module-1", "pathway-123", "Module 1", "module-1", "First module",
					0, "always", pq.Array([]string{}),
					true, "icon1", 30, fixedTime,
				).AddRow(
					"module-2", "pathway-123", "Module 2", "module-2", "Second module",
					1, "sequential", pq.Array([]string{"module-1"}),
					true, "icon2", 45, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules WHERE pathway_id = \$1`).
					WithArgs("pathway-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "empty result",
			pathwayID: "pathway-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "pathway_id", "name", "slug", "description",
					"display_order", "unlock_type", "required_module_ids",
					"is_active", "icon", "estimated_minutes", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "database error",
			pathwayID: "pathway-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_modules`).
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListModules(context.Background(), tt.pathwayID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_UpdateModule(t *testing.T) {
	tests := []struct {
		name    string
		module  *models.PathwayModule
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			module: &models.PathwayModule{
				ID:           "module-123",
				Name:         "Updated Module",
				Description:  "Updated description",
				DisplayOrder: 1,
				UnlockType:   "sequential",
				IsActive:     true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_modules`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			module: &models.PathwayModule{
				ID:   "module-err",
				Name: "Error Update",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_modules`).
					WillReturnError(errors.New("update failed"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateModule(context.Background(), tt.module)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_DeleteModule(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pathway_modules WHERE id = \$1`).
					WithArgs("module-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			id:   "module-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pathway_modules`).
					WillReturnError(errors.New("delete failed"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.DeleteModule(context.Background(), tt.id)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_ReorderModules(t *testing.T) {
	tests := []struct {
		name      string
		pathwayID string
		moduleIDs []string
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "success",
			pathwayID: "pathway-123",
			moduleIDs: []string{"module-3", "module-1", "module-2"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE pathway_modules SET display_order = \$1 WHERE id = \$2 AND pathway_id = \$3`).
					WithArgs(0, "module-3", "pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_modules SET display_order = \$1 WHERE id = \$2 AND pathway_id = \$3`).
					WithArgs(1, "module-1", "pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_modules SET display_order = \$1 WHERE id = \$2 AND pathway_id = \$3`).
					WithArgs(2, "module-2", "pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:      "begin transaction error",
			pathwayID: "pathway-err",
			moduleIDs: []string{"module-1"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("connection error"))
			},
			wantErr: true,
			errMsg:  "beginning transaction",
		},
		{
			name:      "update error",
			pathwayID: "pathway-123",
			moduleIDs: []string{"module-1", "module-2"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE pathway_modules`).
					WillReturnError(errors.New("update failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "updating module order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.ReorderModules(context.Background(), tt.pathwayID, tt.moduleIDs)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !pathwayContainsString(err.Error(), tt.errMsg) {
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

// -----------------------------------------------------------------------------
// Module Labs Tests
// -----------------------------------------------------------------------------

func TestPathwayRepo_AddLabToModule(t *testing.T) {
	tests := []struct {
		name      string
		moduleLab *models.ModuleLab
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "success",
			moduleLab: &models.ModuleLab{
				ID:            "ml-123",
				ModuleID:      "module-123",
				LabTemplateID: "lab-123",
				DisplayOrder:  0,
				IsRequired:    true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(sql.NullTime{})
				mock.ExpectQuery(`INSERT INTO module_labs`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			moduleLab: &models.ModuleLab{
				ID:            "ml-err",
				ModuleID:      "module-123",
				LabTemplateID: "lab-123",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO module_labs`).
					WillReturnError(errors.New("foreign key violation"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.AddLabToModule(context.Background(), tt.moduleLab)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_RemoveLabFromModule(t *testing.T) {
	tests := []struct {
		name          string
		moduleID      string
		labTemplateID string
		mockFn        func(mock sqlmock.Sqlmock)
		wantErr       bool
	}{
		{
			name:          "success",
			moduleID:      "module-123",
			labTemplateID: "lab-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM module_labs WHERE module_id = \$1 AND lab_template_id = \$2`).
					WithArgs("module-123", "lab-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:          "database error",
			moduleID:      "module-err",
			labTemplateID: "lab-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM module_labs`).
					WillReturnError(errors.New("delete failed"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.RemoveLabFromModule(context.Background(), tt.moduleID, tt.labTemplateID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_ListModuleLabs(t *testing.T) {
	tests := []struct {
		name      string
		moduleID  string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:     "multiple labs",
			moduleID: "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "module_id", "lab_template_id", "display_order",
					"is_required", "pass_threshold_override",
					"name", "description", "difficulty", "duration_minutes", "max_points", "platform",
				}).AddRow(
					"ml-1", "module-123", "lab-1", 0,
					true, nil,
					"Lab 1", "Lab description", "beginner", 30, 100, "proxmox",
				).AddRow(
					"ml-2", "module-123", "lab-2", 1,
					false, 80,
					"Lab 2", "Lab 2 description", "intermediate", 45, 150, "cloudstack",
				)
				mock.ExpectQuery(`SELECT .+ FROM module_labs ml JOIN lab_templates lt`).
					WithArgs("module-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "empty result",
			moduleID: "module-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "module_id", "lab_template_id", "display_order",
					"is_required", "pass_threshold_override",
					"name", "description", "difficulty", "duration_minutes", "max_points", "platform",
				})
				mock.ExpectQuery(`SELECT .+ FROM module_labs ml JOIN lab_templates lt`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:     "database error",
			moduleID: "module-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM module_labs ml JOIN lab_templates lt`).
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListModuleLabs(context.Background(), tt.moduleID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPathwayRepo_ReorderModuleLabs(t *testing.T) {
	tests := []struct {
		name           string
		moduleID       string
		labTemplateIDs []string
		mockFn         func(mock sqlmock.Sqlmock)
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "success",
			moduleID:       "module-123",
			labTemplateIDs: []string{"lab-3", "lab-1", "lab-2"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE module_labs SET display_order = \$1 WHERE module_id = \$2 AND lab_template_id = \$3`).
					WithArgs(0, "module-123", "lab-3").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE module_labs SET display_order = \$1 WHERE module_id = \$2 AND lab_template_id = \$3`).
					WithArgs(1, "module-123", "lab-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE module_labs SET display_order = \$1 WHERE module_id = \$2 AND lab_template_id = \$3`).
					WithArgs(2, "module-123", "lab-2").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:           "begin transaction error",
			moduleID:       "module-err",
			labTemplateIDs: []string{"lab-1"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("connection error"))
			},
			wantErr: true,
			errMsg:  "beginning transaction",
		},
		{
			name:           "update error",
			moduleID:       "module-123",
			labTemplateIDs: []string{"lab-1", "lab-2"},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE module_labs`).
					WillReturnError(errors.New("update failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "updating lab order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			err = repo.ReorderModuleLabs(context.Background(), tt.moduleID, tt.labTemplateIDs)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !pathwayContainsString(err.Error(), tt.errMsg) {
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

// -----------------------------------------------------------------------------
// Stats Tests
// -----------------------------------------------------------------------------

func TestPathwayRepo_GetStats(t *testing.T) {
	tests := []struct {
		name      string
		pathwayID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:      "existing stats",
			pathwayID: "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "module_count", "lab_count", "total_points",
					"total_duration_minutes", "enrollment_count", "completion_count",
				}).AddRow(
					"pathway-123", 5, 15, 1000, 300, 100, 50,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_stats WHERE id = \$1`).
					WithArgs("pathway-123").
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name:      "no stats - returns empty",
			pathwayID: "pathway-new",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_stats WHERE id = \$1`).
					WithArgs("pathway-new").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: false,
		},
		{
			name:      "database error",
			pathwayID: "pathway-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_stats`).
					WillReturnError(errors.New("connection error"))
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

			repo := NewPathwayRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetStats(context.Background(), tt.pathwayID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected non-nil result even for new pathways")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Helper Tests
// -----------------------------------------------------------------------------

func TestNewPathwayRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPathwayRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "Network Fundamentals",
			expected: "network-fundamentals",
		},
		{
			name:     "name with special characters",
			input:    "Security & Network Basics!",
			expected: "security--network-basics",
		},
		{
			name:     "name with numbers",
			input:    "Level 1 Introduction",
			expected: "level-1-introduction",
		},
		{
			name:     "already lowercase",
			input:    "already-lowercase",
			expected: "already-lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSlug(tt.input)
			if result != tt.expected {
				t.Errorf("GenerateSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func pathwayContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
