package models

import (
	"encoding/json"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// OrgRole Tests
// -----------------------------------------------------------------------------

func TestOrgRole_Priority(t *testing.T) {
	tests := []struct {
		role     OrgRole
		expected int
	}{
		{OrgRoleOwner, 100},
		{OrgRoleAdmin, 80},
		{OrgRoleInstructor, 50},
		{OrgRoleMember, 10},
		{OrgRole("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.Priority(); got != tt.expected {
				t.Errorf("OrgRole(%q).Priority() = %d, want %d", tt.role, got, tt.expected)
			}
		})
	}
}

func TestOrgRole_HasPermission(t *testing.T) {
	tests := []struct {
		name     string
		role     OrgRole
		minRole  OrgRole
		expected bool
	}{
		{"owner has owner", OrgRoleOwner, OrgRoleOwner, true},
		{"owner has admin", OrgRoleOwner, OrgRoleAdmin, true},
		{"owner has instructor", OrgRoleOwner, OrgRoleInstructor, true},
		{"owner has member", OrgRoleOwner, OrgRoleMember, true},
		{"admin has admin", OrgRoleAdmin, OrgRoleAdmin, true},
		{"admin has instructor", OrgRoleAdmin, OrgRoleInstructor, true},
		{"admin lacks owner", OrgRoleAdmin, OrgRoleOwner, false},
		{"instructor has instructor", OrgRoleInstructor, OrgRoleInstructor, true},
		{"instructor lacks admin", OrgRoleInstructor, OrgRoleAdmin, false},
		{"member has member", OrgRoleMember, OrgRoleMember, true},
		{"member lacks instructor", OrgRoleMember, OrgRoleInstructor, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.HasPermission(tt.minRole); got != tt.expected {
				t.Errorf("OrgRole(%q).HasPermission(%q) = %v, want %v",
					tt.role, tt.minRole, got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// OrgSettings Tests
// -----------------------------------------------------------------------------

func TestOrgSettings_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		check   func(*testing.T, *OrgSettings)
	}{
		{
			name:    "nil value",
			input:   nil,
			wantErr: false,
			check: func(t *testing.T, s *OrgSettings) {
				// Should remain unchanged
			},
		},
		{
			name:    "valid JSON bytes",
			input:   []byte(`{"allowPublicSignup":true,"enableTeams":true}`),
			wantErr: false,
			check: func(t *testing.T, s *OrgSettings) {
				if !s.AllowPublicSignup {
					t.Error("expected AllowPublicSignup to be true")
				}
				if !s.EnableTeams {
					t.Error("expected EnableTeams to be true")
				}
			},
		},
		{
			name:    "invalid JSON",
			input:   []byte(`{invalid}`),
			wantErr: true,
			check:   func(t *testing.T, s *OrgSettings) {},
		},
		{
			name:    "non-byte value",
			input:   "string value",
			wantErr: false, // Returns nil for non-byte
			check:   func(t *testing.T, s *OrgSettings) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s OrgSettings
			err := s.Scan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				tt.check(t, &s)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// OrganizationMembership Tests
// -----------------------------------------------------------------------------

func TestOrganizationMembership_IsPending(t *testing.T) {
	tests := []struct {
		name     string
		accepted *time.Time
		expected bool
	}{
		{
			name:     "pending - nil accepted",
			accepted: nil,
			expected: true,
		},
		{
			name:     "accepted - non-nil",
			accepted: func() *time.Time { t := time.Now(); return &t }(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &OrganizationMembership{AcceptedAt: tt.accepted}
			if got := m.IsPending(); got != tt.expected {
				t.Errorf("IsPending() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// OrganizationFeature Tests
// -----------------------------------------------------------------------------

func TestOrganizationFeature_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		expires  *time.Time
		expected bool
	}{
		{
			name:     "no expiration",
			expires:  nil,
			expected: false,
		},
		{
			name:     "expired",
			expires:  &past,
			expected: true,
		},
		{
			name:     "not expired",
			expires:  &future,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &OrganizationFeature{ExpiresAt: tt.expires}
			if got := f.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// License Tests
// -----------------------------------------------------------------------------

func TestLicense_IsValid(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		status   LicenseStatus
		expires  *time.Time
		expected bool
	}{
		{
			name:     "valid and not expired",
			status:   LicenseStatusValid,
			expires:  &future,
			expected: true,
		},
		{
			name:     "valid but expired",
			status:   LicenseStatusValid,
			expires:  &past,
			expected: false,
		},
		{
			name:     "pending",
			status:   LicenseStatusPending,
			expires:  &future,
			expected: false,
		},
		{
			name:     "expired status",
			status:   LicenseStatusExpired,
			expires:  &future,
			expected: false,
		},
		{
			name:     "revoked",
			status:   LicenseStatusRevoked,
			expires:  &future,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &License{ValidationStatus: tt.status, ExpiresAt: tt.expires, IsActive: true}
			if got := l.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLicense_MaskedKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "long key",
			key:      "ABCD-1234-EFGH-5678",
			expected: "ABCD...5678",
		},
		{
			name:     "exactly 8 chars",
			key:      "12345678",
			expected: "1234...5678",
		},
		{
			name:     "short key",
			key:      "ABC",
			expected: "****",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &License{LicenseKey: tt.key}
			if got := l.MaskedKey(); got != tt.expected {
				t.Errorf("MaskedKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// TenantContext Tests
// -----------------------------------------------------------------------------

func TestTenantContext_HasFeature(t *testing.T) {
	tc := &TenantContext{
		Features: map[string]bool{
			"feature1": true,
			"feature2": false,
		},
	}

	tests := []struct {
		feature  string
		expected bool
	}{
		{"feature1", true},
		{"feature2", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			if got := tc.HasFeature(tt.feature); got != tt.expected {
				t.Errorf("HasFeature(%q) = %v, want %v", tt.feature, got, tt.expected)
			}
		})
	}
}

func TestTenantContext_HasRole(t *testing.T) {
	tests := []struct {
		name     string
		userRole OrgRole
		minRole  OrgRole
		expected bool
	}{
		{"admin has instructor", OrgRoleAdmin, OrgRoleInstructor, true},
		{"member lacks admin", OrgRoleMember, OrgRoleAdmin, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantContext{
				Membership: &OrganizationMembership{Role: tt.userRole},
			}
			if got := tc.HasRole(tt.minRole); got != tt.expected {
				t.Errorf("HasRole(%q) = %v, want %v", tt.minRole, got, tt.expected)
			}
		})
	}
}

func TestTenantContext_HasRole_NilMembership(t *testing.T) {
	tc := &TenantContext{Membership: nil}
	if got := tc.HasRole(OrgRoleMember); got != false {
		t.Errorf("HasRole() with nil membership = %v, want false", got)
	}
}

func TestTenantContext_RoleChecks(t *testing.T) {
	tests := []struct {
		name         string
		role         OrgRole
		isOwner      bool
		isAdmin      bool
		isInstructor bool
	}{
		{"owner", OrgRoleOwner, true, true, true},
		{"admin", OrgRoleAdmin, false, true, true},
		{"instructor", OrgRoleInstructor, false, false, true},
		{"member", OrgRoleMember, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantContext{
				Membership: &OrganizationMembership{Role: tt.role},
			}

			if got := tc.IsOwner(); got != tt.isOwner {
				t.Errorf("IsOwner() = %v, want %v", got, tt.isOwner)
			}
			if got := tc.IsAdmin(); got != tt.isAdmin {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.isAdmin)
			}
			if got := tc.IsInstructor(); got != tt.isInstructor {
				t.Errorf("IsInstructor() = %v, want %v", got, tt.isInstructor)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// PathwayEnrollment Tests
// -----------------------------------------------------------------------------

func TestPathwayEnrollment_IsCompleted(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		status      EnrollmentStatus
		completedAt *time.Time
		expected    bool
	}{
		{"completed with date", EnrollmentStatusCompleted, &now, true},
		{"completed without date", EnrollmentStatusCompleted, nil, false},
		{"enrolled", EnrollmentStatusEnrolled, nil, false},
		{"abandoned", EnrollmentStatusAbandoned, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &PathwayEnrollment{Status: tt.status, CompletedAt: tt.completedAt}
			if got := e.IsCompleted(); got != tt.expected {
				t.Errorf("IsCompleted() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPathwayEnrollment_ProgressPercentage(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		expected  float64
	}{
		{"full progress", 10, 10, 100.0},
		{"half progress", 5, 10, 50.0},
		{"zero total", 5, 0, 0.0},
		{"zero progress", 0, 10, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &PathwayEnrollment{CompletedModules: tt.completed, TotalModules: tt.total}
			if got := e.ProgressPercentage(); got != tt.expected {
				t.Errorf("ProgressPercentage() = %f, want %f", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// ModuleProgress Tests
// -----------------------------------------------------------------------------

func TestModuleProgress_IsCompleted(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		status      ModuleProgressStatus
		completedAt *time.Time
		expected    bool
	}{
		{"completed status and date", ModuleStatusCompleted, &now, true},
		{"completed status but no date", ModuleStatusCompleted, nil, false},
		{"in progress status with date", ModuleStatusInProgress, &now, false},
		{"locked", ModuleStatusLocked, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ModuleProgress{Status: tt.status, CompletedAt: tt.completedAt}
			if got := m.IsCompleted(); got != tt.expected {
				t.Errorf("IsCompleted() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModuleProgress_ProgressPercentage(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		expected  float64
	}{
		{"full progress", 10, 10, 100.0},
		{"partial progress", 3, 4, 75.0},
		{"zero total", 5, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ModuleProgress{CompletedLabs: tt.completed, TotalLabs: tt.total}
			if got := m.ProgressPercentage(); got != tt.expected {
				t.Errorf("ProgressPercentage() = %f, want %f", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// LabProgress Tests
// -----------------------------------------------------------------------------

func TestLabProgress_ScorePercentage(t *testing.T) {
	tests := []struct {
		name     string
		best     int
		max      int
		expected float64
	}{
		{"perfect score", 100, 100, 100.0},
		{"partial score", 85, 100, 85.0},
		{"zero max", 50, 0, 0.0},
		{"zero score", 0, 100, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LabProgress{BestScore: tt.best, MaxPoints: tt.max}
			if got := l.ScorePercentage(); got != tt.expected {
				t.Errorf("ScorePercentage() = %f, want %f", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// PathwayStats Tests
// -----------------------------------------------------------------------------

func TestPathwayStats_ComputeCompletionRate(t *testing.T) {
	tests := []struct {
		name         string
		enrolled     int
		completed    int
		expectedRate float64
	}{
		{"50% completion", 100, 50, 50.0},
		{"100% completion", 10, 10, 100.0},
		{"no enrollments", 0, 0, 0.0},
		{"no completions", 100, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &PathwayStats{
				EnrollmentCount: tt.enrolled,
				CompletionCount: tt.completed,
			}
			s.ComputeCompletionRate()

			if s.CompletionRate != tt.expectedRate {
				t.Errorf("CompletionRate = %f, want %f", s.CompletionRate, tt.expectedRate)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// LabTemplateRecord Tests
// -----------------------------------------------------------------------------

func TestLabTemplateRecord_ToLabTemplate(t *testing.T) {
	spec := `{"platform":"proxmox","vms":[{"name":"server"}]}`
	checkpoints := `[{"id":"cp1","description":"Test checkpoint","points":10}]`

	record := &LabTemplateRecord{
		ID:          "tmpl-123",
		Name:        "Test Lab",
		Slug:        "test-lab",
		Description: "A test laboratory",
		Version:     "1.0",
		Platform:    PlatformProxmox,
		Difficulty:  "intermediate",
		Spec:        json.RawMessage(spec),
		Checkpoints: json.RawMessage(checkpoints),
	}

	template, err := record.ToLabTemplate()
	if err != nil {
		t.Fatalf("ToLabTemplate() error: %v", err)
	}

	if template.Kind != "LabTemplate" {
		t.Errorf("expected Kind 'LabTemplate', got %q", template.Kind)
	}
	if template.Metadata.Name != "Test Lab" {
		t.Errorf("expected Name 'Test Lab', got %q", template.Metadata.Name)
	}
	if template.Spec.Platform != PlatformProxmox {
		t.Errorf("expected Platform 'proxmox', got %q", template.Spec.Platform)
	}
	if len(template.Spec.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(template.Spec.VMs))
	}
}

func TestLabTemplateRecord_ToLabTemplate_InvalidSpec(t *testing.T) {
	record := &LabTemplateRecord{
		Name: "Test Lab",
		Spec: json.RawMessage(`{invalid json}`),
	}

	_, err := record.ToLabTemplate()
	if err == nil {
		t.Error("expected error for invalid spec JSON")
	}
}

// -----------------------------------------------------------------------------
// Type Constants Tests
// -----------------------------------------------------------------------------

func TestPlatformConstants(t *testing.T) {
	if PlatformProxmox != "proxmox" {
		t.Errorf("expected PlatformProxmox='proxmox', got %q", PlatformProxmox)
	}
	if PlatformCloudStack != "cloudstack" {
		t.Errorf("expected PlatformCloudStack='cloudstack', got %q", PlatformCloudStack)
	}
	if PlatformAny != "any" {
		t.Errorf("expected PlatformAny='any', got %q", PlatformAny)
	}
}

func TestEnrollmentStatusConstants(t *testing.T) {
	if EnrollmentStatusEnrolled != "enrolled" {
		t.Errorf("expected EnrollmentStatusEnrolled='enrolled', got %q", EnrollmentStatusEnrolled)
	}
	if EnrollmentStatusCompleted != "completed" {
		t.Errorf("expected EnrollmentStatusCompleted='completed', got %q", EnrollmentStatusCompleted)
	}
	if EnrollmentStatusAbandoned != "abandoned" {
		t.Errorf("expected EnrollmentStatusAbandoned='abandoned', got %q", EnrollmentStatusAbandoned)
	}
	if EnrollmentStatusInProgress != "in_progress" {
		t.Errorf("expected EnrollmentStatusInProgress='in_progress', got %q", EnrollmentStatusInProgress)
	}
}

func TestPodStatusConstants(t *testing.T) {
	if PodStatusProvisioning != "provisioning" {
		t.Errorf("expected PodStatusProvisioning='provisioning', got %q", PodStatusProvisioning)
	}
	if PodStatusRunning != "running" {
		t.Errorf("expected PodStatusRunning='running', got %q", PodStatusRunning)
	}
	if PodStatusStopped != "stopped" {
		t.Errorf("expected PodStatusStopped='stopped', got %q", PodStatusStopped)
	}
	if PodStatusError != "error" {
		t.Errorf("expected PodStatusError='error', got %q", PodStatusError)
	}
	if PodStatusDestroying != "destroying" {
		t.Errorf("expected PodStatusDestroying='destroying', got %q", PodStatusDestroying)
	}
	if PodStatusDestroyed != "destroyed" {
		t.Errorf("expected PodStatusDestroyed='destroyed', got %q", PodStatusDestroyed)
	}
}

func TestAchievementTypeConstants(t *testing.T) {
	if AchievementTypeLabCompletion != "lab_completion" {
		t.Errorf("expected AchievementTypeLabCompletion='lab_completion', got %q", AchievementTypeLabCompletion)
	}
	if AchievementTypeMilestone != "milestone" {
		t.Errorf("expected AchievementTypeMilestone='milestone', got %q", AchievementTypeMilestone)
	}
	if AchievementTypePathway != "pathway" {
		t.Errorf("expected AchievementTypePathway='pathway', got %q", AchievementTypePathway)
	}
}

func TestAchievementTierConstants(t *testing.T) {
	if AchievementTierBronze != "bronze" {
		t.Errorf("expected AchievementTierBronze='bronze', got %q", AchievementTierBronze)
	}
	if AchievementTierSilver != "silver" {
		t.Errorf("expected AchievementTierSilver='silver', got %q", AchievementTierSilver)
	}
	if AchievementTierGold != "gold" {
		t.Errorf("expected AchievementTierGold='gold', got %q", AchievementTierGold)
	}
	if AchievementTierPlatinum != "platinum" {
		t.Errorf("expected AchievementTierPlatinum='platinum', got %q", AchievementTierPlatinum)
	}
}

// -----------------------------------------------------------------------------
// NewLabTemplateRecord Tests
// -----------------------------------------------------------------------------

func TestNewLabTemplateRecord_Success(t *testing.T) {
	template := &LabTemplate{
		Metadata: LabMetadata{
			Name:        "Test Lab",
			Description: "A test laboratory",
			Version:     "1.0.0",
			Duration:    "2h",
			Difficulty:  "intermediate",
		},
		Spec: LabSpec{
			Platform: PlatformProxmox,
			VMs: []VMSpec{
				{Name: "server", Template: "ubuntu-22.04"},
			},
			Objectives: []Checkpoint{
				{ID: "cp1", Description: "First checkpoint", Points: 10},
				{ID: "cp2", Description: "Second checkpoint", Points: 20},
			},
		},
	}

	record, err := NewLabTemplateRecord(template)
	if err != nil {
		t.Fatalf("NewLabTemplateRecord() error: %v", err)
	}

	if record.Name != "Test Lab" {
		t.Errorf("expected Name 'Test Lab', got %q", record.Name)
	}
	if record.Description != "A test laboratory" {
		t.Errorf("expected Description 'A test laboratory', got %q", record.Description)
	}
	if record.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got %q", record.Version)
	}
	if record.Platform != PlatformProxmox {
		t.Errorf("expected Platform 'proxmox', got %q", record.Platform)
	}
	if record.DurationMinutes != 120 {
		t.Errorf("expected DurationMinutes 120, got %d", record.DurationMinutes)
	}
	if record.Difficulty != "intermediate" {
		t.Errorf("expected Difficulty 'intermediate', got %q", record.Difficulty)
	}
	if record.MaxPoints != 30 {
		t.Errorf("expected MaxPoints 30, got %d", record.MaxPoints)
	}
	if record.PassThreshold != 70 { // default
		t.Errorf("expected PassThreshold 70, got %d", record.PassThreshold)
	}
	if !record.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestNewLabTemplateRecord_WithPassThreshold(t *testing.T) {
	template := &LabTemplate{
		Metadata: LabMetadata{Name: "Test"},
		Spec: LabSpec{
			Platform: PlatformProxmox,
			VMs:      []VMSpec{{Name: "server", Template: "ubuntu"}},
			Checkpoints: &CheckpointConfig{
				Enabled:       true,
				PassThreshold: 80,
			},
		},
	}

	record, err := NewLabTemplateRecord(template)
	if err != nil {
		t.Fatalf("NewLabTemplateRecord() error: %v", err)
	}

	if record.PassThreshold != 80 {
		t.Errorf("expected PassThreshold 80, got %d", record.PassThreshold)
	}
}

func TestNewLabTemplateRecord_NoObjectives(t *testing.T) {
	template := &LabTemplate{
		Metadata: LabMetadata{Name: "Test"},
		Spec: LabSpec{
			Platform: PlatformProxmox,
			VMs:      []VMSpec{{Name: "server", Template: "ubuntu"}},
		},
	}

	record, err := NewLabTemplateRecord(template)
	if err != nil {
		t.Fatalf("NewLabTemplateRecord() error: %v", err)
	}

	if record.MaxPoints != 0 {
		t.Errorf("expected MaxPoints 0, got %d", record.MaxPoints)
	}
	if record.Checkpoints != nil {
		t.Error("expected Checkpoints to be nil")
	}
}

// -----------------------------------------------------------------------------
// parseDuration Tests
// -----------------------------------------------------------------------------

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"2h", 120},
		{"30m", 30},
		{"1h", 60},
		{"90m", 90},
		{"", 0},
		{"invalid", 0},
		{"5x", 0}, // unknown unit
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseDuration(tt.input); got != tt.expected {
				t.Errorf("parseDuration(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// formatDuration Tests
// -----------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		minutes  int
		expected string
	}{
		{0, ""},
		{-5, ""},
		{30, "30m"},
		{59, "59m"},
		{60, "1h"},
		{90, "1h30m"},
		{120, "2h"},
		{150, "2h30m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := formatDuration(tt.minutes); got != tt.expected {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.minutes, got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Additional License Tests
// -----------------------------------------------------------------------------

func TestLicense_IsValid_InactiveLicense(t *testing.T) {
	future := time.Now().Add(time.Hour)
	l := &License{
		ValidationStatus: LicenseStatusValid,
		ExpiresAt:        &future,
		IsActive:         false, // Inactive
	}

	if l.IsValid() {
		t.Error("expected inactive license to be invalid")
	}
}

func TestLicense_IsValid_NoExpiration(t *testing.T) {
	l := &License{
		ValidationStatus: LicenseStatusValid,
		ExpiresAt:        nil, // No expiration
		IsActive:         true,
	}

	if !l.IsValid() {
		t.Error("expected license without expiration to be valid")
	}
}

// -----------------------------------------------------------------------------
// Additional TenantContext Tests
// -----------------------------------------------------------------------------

func TestTenantContext_HasFeature_NilFeatures(t *testing.T) {
	tc := &TenantContext{
		Features: nil,
	}

	if tc.HasFeature("any-feature") {
		t.Error("expected HasFeature to return false when Features is nil")
	}
}
