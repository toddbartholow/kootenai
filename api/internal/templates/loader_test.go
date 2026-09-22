package templates

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/templates/validator"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func setupMockRepo(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *repositories.LabTemplateRepo) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	repo := repositories.NewLabTemplateRepo(db)
	return db, mock, repo
}

func TestNewLoader(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	// Basic creation
	loader := NewLoader(repo, logger)
	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
	if loader.repo == nil {
		t.Error("repo should not be nil")
	}
	if loader.logger == nil {
		t.Error("logger should not be nil")
	}
	if loader.validator == nil {
		t.Error("validator should not be nil")
	}
	if loader.resolver == nil {
		t.Error("resolver should not be nil")
	}
	if loader.strictMode {
		t.Error("strictMode should be false by default")
	}
}

func TestNewLoader_WithOptions(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()
	customValidator := validator.NewValidator()

	loader := NewLoader(repo, logger, WithStrictMode(), WithValidator(customValidator))

	if !loader.strictMode {
		t.Error("strictMode should be true")
	}
	if loader.validator != customValidator {
		t.Error("validator should be the custom validator")
	}
}

func TestLoader_LoadFromDirectory_NonexistentDir(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), "/nonexistent/path/to/templates")

	// Should return nil (just logs a warning)
	if err != nil {
		t.Errorf("LoadFromDirectory should not error for nonexistent dir, got: %v", err)
	}
}

func TestLoader_LoadFromDirectory_NotADirectory(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	// Create a temp file (not a directory)
	tmpFile, err := os.CreateTemp("", "notadir*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	loader := NewLoader(repo, logger)
	err = loader.LoadFromDirectory(context.Background(), tmpFile.Name())

	if err == nil {
		t.Error("LoadFromDirectory should error when path is not a directory")
	}
}

func TestLoader_LoadFromDirectory_EmptyDir(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), tmpDir)

	if err != nil {
		t.Errorf("LoadFromDirectory should not error for empty dir: %v", err)
	}
}

func TestLoader_LoadFromDirectory_SkipsNonYAML(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()

	// Create non-YAML files
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "data.json"), []byte("{}"), 0644)

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), tmpDir)

	if err != nil {
		t.Errorf("LoadFromDirectory should not error: %v", err)
	}
}

func TestLoader_LoadFromDirectory_SkipsNonLabTemplate(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()

	// Create a valid YAML file but not a LabTemplate
	yamlContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
`
	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(yamlContent), 0644)

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), tmpDir)

	if err != nil {
		t.Errorf("LoadFromDirectory should not error for non-template YAML: %v", err)
	}
}

func TestLoader_LoadFromDirectory_InvalidYAML(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()

	// Create invalid YAML
	os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte("{{{{invalid yaml"), 0644)

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), tmpDir)

	// Should not return an error (just logs it and continues)
	if err != nil {
		t.Errorf("LoadFromDirectory should not return error for invalid YAML: %v", err)
	}
}

func mockGetByNameNotFound(mock sqlmock.Sqlmock, name string) {
	mock.ExpectQuery("SELECT .+ FROM lab_templates WHERE name = \\$1").
		WithArgs(name).
		WillReturnError(sql.ErrNoRows)
}

func mockGetBySlugNotFound(mock sqlmock.Sqlmock, slug string) {
	mock.ExpectQuery("SELECT .+ FROM lab_templates WHERE slug = \\$1").
		WithArgs(slug).
		WillReturnError(sql.ErrNoRows)
}

func mockCreateSuccess(mock sqlmock.Sqlmock) {
	now := time.Now()
	mock.ExpectQuery("INSERT INTO lab_templates").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow("test-id", now, now))
}

func TestLoader_loadFile_Success(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "test-lab.yaml")

	yamlContent := `
apiVersion: v1
kind: LabTemplate
metadata:
  name: test-lab-template
  description: A test lab template for unit testing
  version: "1.0.0"
  difficulty: beginner
  duration: "90m"
spec:
  platform: proxmox
  vms:
    - name: workstation
      template: ubuntu-22.04
      resources:
        cpu: 2
        memory: 2048
  objectives:
    - id: task-1
      description: Complete the first task
      points: 10
`
	if err := os.WriteFile(templatePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Mock the upsert operation (GetByName -> GetBySlug -> Create)
	mockGetByNameNotFound(mock, "test-lab-template")
	mockGetBySlugNotFound(mock, "test-lab-template")
	mockCreateSuccess(mock)

	loader := NewLoader(repo, logger)
	err := loader.loadFile(context.Background(), templatePath)

	if err != nil {
		t.Errorf("loadFile should succeed: %v", err)
	}
}

func TestLoader_loadFile_StrictModeValidationFailure(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "invalid-lab.yaml")

	// Missing required fields
	yamlContent := `
apiVersion: v1
kind: LabTemplate
metadata:
  name: ab
  description: short
spec:
  vms: []
`
	if err := os.WriteFile(templatePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	loader := NewLoader(repo, logger, WithStrictMode())
	err := loader.loadFile(context.Background(), templatePath)

	if err == nil {
		t.Error("loadFile should fail in strict mode with validation errors")
	}
}

func TestLoader_loadFile_NonStrictModeValidationWarnings(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "warning-lab.yaml")

	// Has validation warnings but not in strict mode
	yamlContent := `
apiVersion: v1
kind: LabTemplate
metadata:
  name: test-lab-with-warnings
  description: short
  version: "1.0"
  difficulty: beginner
spec:
  vms:
    - name: vm1
      template: ubuntu
`
	if err := os.WriteFile(templatePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Mock the upsert operation
	mockGetByNameNotFound(mock, "test-lab-with-warnings")
	mockGetBySlugNotFound(mock, "test-lab-with-warnings")
	mockCreateSuccess(mock)

	loader := NewLoader(repo, logger)
	err := loader.loadFile(context.Background(), templatePath)

	// Should succeed despite warnings
	if err != nil {
		t.Errorf("loadFile should succeed in non-strict mode with warnings: %v", err)
	}
}

func TestLoader_processVariables_NoVariables(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name: "test-lab",
		},
	}

	result := loader.processVariables(template)

	if result.Metadata.Name != "test-lab" {
		t.Errorf("expected name 'test-lab', got '%s'", result.Metadata.Name)
	}
}

func TestLoader_processVariables_WithVariables(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "${LAB_NAME}",
			Description: "Using subnet $LAB_SUBNET",
			Variables: map[string]string{
				"LAB_NAME": "my-custom-lab",
			},
		},
	}

	result := loader.processVariables(template)

	if result.Metadata.Name != "my-custom-lab" {
		t.Errorf("expected name 'my-custom-lab', got '%s'", result.Metadata.Name)
	}
	if result.Metadata.Description != "Using subnet 10.0.0.0/24" {
		t.Errorf("expected description with subnet, got '%s'", result.Metadata.Description)
	}
}

func TestLoader_templateToRecord_Basic(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "Test Lab",
			Description: "A test lab",
			Version:     "1.0.0",
			Duration:    "90m",
			Difficulty:  "beginner",
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs: []models.VMSpec{
				{Name: "vm1", Template: "ubuntu"},
			},
			Objectives: []models.Checkpoint{
				{ID: "task-1", Description: "Do something", Points: 10},
				{ID: "task-2", Description: "Do something else", Points: 20},
			},
		},
	}

	record, err := loader.templateToRecord(template)
	if err != nil {
		t.Fatalf("templateToRecord failed: %v", err)
	}

	if record.Name != "Test Lab" {
		t.Errorf("expected name 'Test Lab', got '%s'", record.Name)
	}
	if record.Slug != "test-lab" {
		t.Errorf("expected slug 'test-lab', got '%s'", record.Slug)
	}
	if record.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", record.Version)
	}
	if record.DurationMinutes != 90 {
		t.Errorf("expected duration 90, got %d", record.DurationMinutes)
	}
	if record.Platform != models.PlatformProxmox {
		t.Errorf("expected platform 'proxmox', got '%s'", record.Platform)
	}
	if record.MaxPoints != 30 {
		t.Errorf("expected max points 30, got %d", record.MaxPoints)
	}
	if record.PassThreshold != 70 {
		t.Errorf("expected pass threshold 70, got %d", record.PassThreshold)
	}
	if !record.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestLoader_templateToRecord_WithCustomPassThreshold(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab",
		},
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				PassThreshold: 80,
			},
		},
	}

	record, err := loader.templateToRecord(template)
	if err != nil {
		t.Fatalf("templateToRecord failed: %v", err)
	}

	if record.PassThreshold != 80 {
		t.Errorf("expected pass threshold 80, got %d", record.PassThreshold)
	}
}

func TestLoader_templateToRecord_DefaultPlatform(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab",
		},
		Spec: models.LabSpec{
			Platform: "", // Empty platform
		},
	}

	record, err := loader.templateToRecord(template)
	if err != nil {
		t.Fatalf("templateToRecord failed: %v", err)
	}

	if record.Platform != models.PlatformAny {
		t.Errorf("expected default platform 'any', got '%s'", record.Platform)
	}
}

func TestLoader_templateToRecord_SpecMarshal(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	loader := NewLoader(repo, logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab",
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs: []models.VMSpec{
				{
					Name:     "workstation",
					Template: "ubuntu-22.04",
					Resources: models.ResourceSpec{
						CPU:    2,
						Memory: 4096,
						Disk:   50,
					},
				},
			},
		},
	}

	record, err := loader.templateToRecord(template)
	if err != nil {
		t.Fatalf("templateToRecord failed: %v", err)
	}

	// Verify spec is valid JSON
	var spec models.LabSpec
	if err := json.Unmarshal(record.Spec, &spec); err != nil {
		t.Fatalf("failed to unmarshal spec: %v", err)
	}

	if spec.Platform != models.PlatformProxmox {
		t.Errorf("expected platform proxmox, got %s", spec.Platform)
	}
	if len(spec.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(spec.VMs))
	}
}

func TestLoader_LoadFromDirectory_WithSubdirectories(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	// Create template in subdirectory
	yamlContent := `
apiVersion: v1
kind: LabTemplate
metadata:
  name: nested-lab-template
  description: A nested lab template for testing subdirectories
  version: "1.0.0"
spec:
  vms:
    - name: vm1
      template: ubuntu
`
	os.WriteFile(filepath.Join(subDir, "nested.yaml"), []byte(yamlContent), 0644)

	// Mock upsert
	mockGetByNameNotFound(mock, "nested-lab-template")
	mockGetBySlugNotFound(mock, "nested-lab-template")
	mockCreateSuccess(mock)

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), tmpDir)

	if err != nil {
		t.Errorf("LoadFromDirectory should handle subdirectories: %v", err)
	}
}

func TestLoader_LoadFromDirectory_YAMLAndYML(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()
	logger := newTestLogger()

	tmpDir := t.TempDir()

	yamlContent := `
apiVersion: v1
kind: LabTemplate
metadata:
  name: yaml-extension-lab
  description: A lab with yaml extension for testing
  version: "1.0.0"
spec:
  vms:
    - name: vm1
      template: ubuntu
`
	ymlContent := `
apiVersion: v1
kind: LabTemplate
metadata:
  name: yml-extension-lab
  description: A lab with yml extension for testing
  version: "1.0.0"
spec:
  vms:
    - name: vm1
      template: ubuntu
`
	os.WriteFile(filepath.Join(tmpDir, "lab1.yaml"), []byte(yamlContent), 0644)
	os.WriteFile(filepath.Join(tmpDir, "lab2.yml"), []byte(ymlContent), 0644)

	// Mock upserts for both files - order may vary due to filesystem
	mockGetByNameNotFound(mock, "yaml-extension-lab")
	mockGetBySlugNotFound(mock, "yaml-extension-lab")
	mockCreateSuccess(mock)
	mockGetByNameNotFound(mock, "yml-extension-lab")
	mockGetBySlugNotFound(mock, "yml-extension-lab")
	mockCreateSuccess(mock)

	loader := NewLoader(repo, logger)
	err := loader.LoadFromDirectory(context.Background(), tmpDir)

	if err != nil {
		t.Errorf("LoadFromDirectory should handle both .yaml and .yml: %v", err)
	}
}

// TestTemplateToRecord_CarriesCategoryAndTags guards the mapping on the path
// imports actually take. An earlier attempt at this fix added the same mapping
// to models.NewLabTemplateRecord, which reads like the obvious place and has
// zero non-test callers — so 28 templates' categories and 61 tag sets still
// vanished on import while the tests went green. loadFile calls this method,
// not that constructor. Assert each field separately so a future field that
// stops being carried names itself in the failure.
func TestTemplateToRecord_CarriesCategoryAndTags(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()
	loader := NewLoader(repo, newTestLogger())

	tmpl := &models.LabTemplate{
		APIVersion: models.CurrentAPIVersion,
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:       "Categorised Lab",
			Version:    "1.0.0",
			Duration:   "45m",
			Difficulty: "beginner",
			Category:   "networking",
			Tags:       []string{"routing", "ospf"},
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs:      []models.VMSpec{{Name: "router", Template: "ubuntu-22.04"}},
		},
	}

	record, err := loader.templateToRecord(tmpl)
	if err != nil {
		t.Fatalf("templateToRecord returned error: %v", err)
	}
	if record.Category != "networking" {
		t.Errorf("Category = %q, want %q", record.Category, "networking")
	}
	if len(record.Tags) != 2 || record.Tags[0] != "routing" || record.Tags[1] != "ospf" {
		t.Errorf("Tags = %v, want [routing ospf]", record.Tags)
	}
	// Slug is what makes this the real path rather than the dead constructor:
	// lab_templates.slug is NOT NULL, and only this method derives one.
	if record.Slug == "" {
		t.Error("Slug is empty; the record would violate the NOT NULL constraint")
	}
}
