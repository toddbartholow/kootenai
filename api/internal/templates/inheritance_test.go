package templates

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestInheritanceResolver_Resolve_NoInheritance(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "standalone-lab",
			Description: "A standalone lab",
		},
	}

	result, err := resolver.Resolve(context.Background(), template)

	if err != nil {
		t.Fatalf("Resolve should succeed: %v", err)
	}
	if result != template {
		t.Error("should return same template when no inheritance")
	}
}

func TestInheritanceResolver_Resolve_CircularInheritance(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	// First template extends parent but mark it as already loading
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:    "child-lab",
			Extends: "parent-lab",
		},
	}

	// Manually mark child-lab as loaded to simulate circular reference
	resolver.loaded["child-lab"] = true

	_, err := resolver.Resolve(context.Background(), template)

	if err == nil {
		t.Error("Resolve should fail with circular inheritance")
	}
}

func TestInheritanceResolver_Resolve_ParentNotFound(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:    "child-lab",
			Extends: "nonexistent-parent",
		},
	}

	// Mock parent not found - return a direct error
	mock.ExpectQuery("SELECT .+ FROM lab_templates WHERE name = \\$1").
		WithArgs("nonexistent-parent").
		WillReturnError(errors.New("record not found"))

	_, err := resolver.Resolve(context.Background(), template)

	if err == nil {
		t.Error("Resolve should fail when parent not found")
	}
}

func mockLabTemplateRow(mock sqlmock.Sqlmock, name string, spec models.LabSpec) {
	specJSON, _ := json.Marshal(spec)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "name", "slug", "description", "version", "platform", "duration_minutes",
		"difficulty", "category", "tags", "max_points", "pass_threshold", "spec", "checkpoints", "instructions",
		"created_at", "updated_at", "is_active",
		"organization_id", "visibility", "created_by", "min_edition",
	}).AddRow(
		"test-id", name, name, "Test description", "1.0.0", "proxmox", 60,
		"beginner", nil, nil, 100, 70, specJSON, nil, nil,
		now, now, true,
		nil, "global", nil, "community",
	)

	mock.ExpectQuery("SELECT .+ FROM lab_templates WHERE name = \\$1").
		WithArgs(name).
		WillReturnRows(rows)
}

func TestInheritanceResolver_loadParent_Success(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	// Mock parent template with proper time values
	parentSpec := models.LabSpec{
		Platform: models.PlatformProxmox,
		VMs: []models.VMSpec{
			{Name: "parent-vm", Template: "ubuntu"},
		},
	}

	mockLabTemplateRow(mock, "parent-lab", parentSpec)

	parent, err := resolver.loadParent(context.Background(), "parent-lab")

	if err != nil {
		t.Fatalf("loadParent should succeed: %v", err)
	}
	if parent.Metadata.Name != "parent-lab" {
		t.Errorf("expected name 'parent-lab', got '%s'", parent.Metadata.Name)
	}
	if len(parent.Spec.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(parent.Spec.VMs))
	}
}

func TestInheritanceResolver_loadParent_Cached(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	// Pre-populate cache
	cachedTemplate := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name: "cached-parent",
		},
	}
	resolver.cache["cached-parent"] = cachedTemplate

	// Should return cached without DB call
	result, err := resolver.loadParent(context.Background(), "cached-parent")

	if err != nil {
		t.Fatalf("loadParent should succeed: %v", err)
	}
	if result != cachedTemplate {
		t.Error("should return cached template")
	}
}

func TestInheritanceResolver_loadParent_DBError(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	mock.ExpectQuery("SELECT .+ FROM lab_templates WHERE name = \\$1").
		WithArgs("error-parent").
		WillReturnError(errors.New("database error"))

	_, err := resolver.loadParent(context.Background(), "error-parent")

	if err == nil {
		t.Error("loadParent should fail with DB error")
	}
}

func TestInheritanceResolver_merge_ChildOverridesVM(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	parent := &models.LabTemplate{
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "workstation", Template: "ubuntu-20.04"},
			},
		},
	}

	child := &models.LabTemplate{
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "workstation", Template: "ubuntu-22.04"}, // Override
			},
		},
	}

	result := resolver.merge(parent, child)

	// Should have only one VM with child's template
	if len(result.Spec.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(result.Spec.VMs))
	}

	// Find the workstation VM
	var workstation *models.VMSpec
	for i := range result.Spec.VMs {
		if result.Spec.VMs[i].Name == "workstation" {
			workstation = &result.Spec.VMs[i]
			break
		}
	}

	if workstation == nil {
		t.Fatal("workstation VM not found")
	}
	if workstation.Template != "ubuntu-22.04" {
		t.Errorf("expected template 'ubuntu-22.04', got '%s'", workstation.Template)
	}
}

func TestInheritanceResolver_merge_Checkpoints(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	parent := &models.LabTemplate{
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				PassThreshold: 70,
				AllowRetry:    true,
				ShowHints:     true,
			},
		},
	}

	child := &models.LabTemplate{
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				PassThreshold: 80,
			},
		},
	}

	result := resolver.merge(parent, child)

	// Child checkpoints should override parent
	if result.Spec.Checkpoints.PassThreshold != 80 {
		t.Errorf("expected pass threshold 80, got %d", result.Spec.Checkpoints.PassThreshold)
	}
}

func TestInheritanceResolver_merge_ParentCheckpointsOnly(t *testing.T) {
	db, _, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	parent := &models.LabTemplate{
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				PassThreshold: 75,
			},
		},
	}

	child := &models.LabTemplate{
		Spec: models.LabSpec{
			// No checkpoints
		},
	}

	result := resolver.merge(parent, child)

	// Should inherit parent's checkpoints
	if result.Spec.Checkpoints == nil {
		t.Fatal("expected checkpoints to be inherited from parent")
	}
	if result.Spec.Checkpoints.PassThreshold != 75 {
		t.Errorf("expected pass threshold 75, got %d", result.Spec.Checkpoints.PassThreshold)
	}
}

func TestMergeNetwork_Additional(t *testing.T) {
	parent := models.NetworkSpec{
		Segments: []models.NetworkSegment{
			{Name: "internal", Subnet: "10.0.0.0/24"},
			{Name: "dmz", Subnet: "10.0.1.0/24"},
		},
	}

	child := models.NetworkSpec{
		Segments: []models.NetworkSegment{
			{Name: "dmz", Subnet: "10.0.2.0/24"},         // Override
			{Name: "external", Subnet: "192.168.0.0/24"}, // Add
		},
	}

	result := mergeNetwork(parent, child)

	if len(result.Segments) != 3 {
		t.Errorf("expected 3 segments, got %d", len(result.Segments))
	}

	// Check segments
	segmentMap := make(map[string]string)
	for _, seg := range result.Segments {
		segmentMap[seg.Name] = seg.Subnet
	}

	if segmentMap["internal"] != "10.0.0.0/24" {
		t.Errorf("internal should be 10.0.0.0/24, got %s", segmentMap["internal"])
	}
	if segmentMap["dmz"] != "10.0.2.0/24" {
		t.Errorf("dmz should be overridden to 10.0.2.0/24, got %s", segmentMap["dmz"])
	}
	if segmentMap["external"] != "192.168.0.0/24" {
		t.Errorf("external should be 192.168.0.0/24, got %s", segmentMap["external"])
	}
}

func TestInheritanceResolver_Resolve_WithParent(t *testing.T) {
	db, mock, repo := setupMockRepo(t)
	defer db.Close()

	resolver := NewInheritanceResolver(repo)

	// Child template that extends a parent
	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:    "child-lab",
			Extends: "parent-lab",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "child-vm", Template: "kali"},
			},
		},
	}

	// Mock parent template (no extends)
	parentSpec := models.LabSpec{
		Platform: models.PlatformProxmox,
		VMs: []models.VMSpec{
			{Name: "parent-vm", Template: "ubuntu"},
		},
	}

	mockLabTemplateRow(mock, "parent-lab", parentSpec)

	result, err := resolver.Resolve(context.Background(), template)

	if err != nil {
		t.Fatalf("Resolve should succeed: %v", err)
	}

	// Should merge VMs
	if len(result.Spec.VMs) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(result.Spec.VMs))
	}

	// Should inherit platform from parent
	if result.Spec.Platform != models.PlatformProxmox {
		t.Errorf("expected platform proxmox, got %s", result.Spec.Platform)
	}
}
