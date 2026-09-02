package validator

import (
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestValidator_ValidTemplate(t *testing.T) {
	v := NewValidator()

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab for validation testing",
			Duration:    "90m",
			Difficulty:  "beginner",
			Version:     "1.0.0",
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs: []models.VMSpec{
				{
					Name:     "vm1",
					Template: "ubuntu-22.04",
					Resources: models.ResourceSpec{
						CPU:    2,
						Memory: 4096,
					},
				},
			},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{
						Name:    "default",
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
		},
	}

	result := v.Validate(template)
	if !result.Valid {
		t.Errorf("Expected valid template, got errors: %v", result.Errors)
	}
}

func TestValidator_MissingAPIVersion(t *testing.T) {
	v := NewValidator()

	template := &models.LabTemplate{
		Kind: "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
		},
	}

	result := v.Validate(template)
	if result.Valid {
		t.Error("Expected validation to fail for missing apiVersion")
	}

	hasAPIVersionError := false
	for _, err := range result.Errors {
		if err.Field == "apiVersion" {
			hasAPIVersionError = true
			break
		}
	}
	if !hasAPIVersionError {
		t.Error("Expected apiVersion error")
	}
}

func TestValidator_InvalidName(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name     string
		expected bool
		message  string
	}{
		{"ab", false, "too short"},
		{"Test-Lab", false, "uppercase letters"},
		{"test_lab", false, "underscore"},
		{"-test-lab", false, "starts with hyphen"},
		{"test-lab-", false, "ends with hyphen"},
		{"test-lab-123", true, "valid with numbers"},
		{"a-very-long-name-that-exceeds-the-maximum-allowed-characters-limit-for-names", false, "too long"},
	}

	for _, tc := range tests {
		template := &models.LabTemplate{
			APIVersion: "v1",
			Kind:       "LabTemplate",
			Metadata: models.LabMetadata{
				Name:        tc.name,
				Description: "A test lab description",
			},
			Spec: models.LabSpec{
				VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
			},
		}

		result := v.Validate(template)
		if result.Valid != tc.expected {
			t.Errorf("Name %q: expected valid=%v (%s), got valid=%v, errors=%v",
				tc.name, tc.expected, tc.message, result.Valid, result.Errors)
		}
	}
}

func TestValidator_InvalidDifficulty(t *testing.T) {
	v := NewValidator()

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab description",
			Difficulty:  "super-hard",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
		},
	}

	result := v.Validate(template)
	if result.Valid {
		t.Error("Expected validation to fail for invalid difficulty")
	}

	hasDifficultyError := false
	for _, err := range result.Errors {
		if err.Field == "metadata.difficulty" {
			hasDifficultyError = true
			break
		}
	}
	if !hasDifficultyError {
		t.Error("Expected difficulty error")
	}
}

func TestValidator_DuplicateVMNames(t *testing.T) {
	v := NewValidator()

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab description",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{Name: "vm1", Template: "ubuntu"},
				{Name: "vm1", Template: "centos"}, // Duplicate
			},
		},
	}

	result := v.Validate(template)
	if result.Valid {
		t.Error("Expected validation to fail for duplicate VM names")
	}

	hasDuplicateError := false
	for _, err := range result.Errors {
		if err.Message == "duplicate VM name" {
			hasDuplicateError = true
			break
		}
	}
	if !hasDuplicateError {
		t.Error("Expected duplicate VM name error")
	}
}

func TestValidator_ResourceLimits(t *testing.T) {
	v := NewValidator()

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab description",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{
					Name:     "vm1",
					Template: "ubuntu",
					Resources: models.ResourceSpec{
						CPU:    32,     // Exceeds default limit of 16
						Memory: 100000, // Exceeds default limit of 65536
					},
				},
			},
		},
	}

	result := v.Validate(template)
	if result.Valid {
		t.Error("Expected validation to fail for resource limit violations")
	}

	hasCPUError := false
	hasMemoryError := false
	for _, err := range result.Errors {
		if err.Field == "spec.vms[0].resources.cpu" {
			hasCPUError = true
		}
		if err.Field == "spec.vms[0].resources.memory" {
			hasMemoryError = true
		}
	}
	if !hasCPUError {
		t.Error("Expected CPU limit error")
	}
	if !hasMemoryError {
		t.Error("Expected memory limit error")
	}
}

func TestValidator_InvalidVLAN(t *testing.T) {
	v := NewValidator()

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab description",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{
						Name: "default",
						VLAN: 5000, // Invalid: must be 1-4094
					},
				},
			},
		},
	}

	result := v.Validate(template)
	if result.Valid {
		t.Error("Expected validation to fail for invalid VLAN")
	}

	hasVLANError := false
	for _, err := range result.Errors {
		if err.Field == "spec.network.segments[0].vlan" {
			hasVLANError = true
			break
		}
	}
	if !hasVLANError {
		t.Error("Expected VLAN error")
	}
}

func TestValidator_CustomLimits(t *testing.T) {
	limits := PlatformLimits{
		MaxCPU:      4,
		MaxMemoryMB: 8192,
		MaxDiskGB:   100,
		MaxVMs:      2,
		MaxNetworks: 1,
		Platforms:   []models.Platform{models.PlatformProxmox},
	}
	v := NewValidatorWithLimits(limits)

	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "test-lab",
			Description: "A test lab description",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{
					Name:     "vm1",
					Template: "ubuntu",
					Resources: models.ResourceSpec{
						CPU: 8, // Exceeds custom limit of 4
					},
				},
			},
		},
	}

	result := v.Validate(template)
	if result.Valid {
		t.Error("Expected validation to fail with custom limits")
	}
}

func TestValidationResult_Error(t *testing.T) {
	result := &ValidationResult{Valid: true}
	if result.Error() != "" {
		t.Error("Valid result should have empty error string")
	}

	result.AddError("field1", "error1", nil)
	result.AddError("field2", "error2", "value")

	errStr := result.Error()
	if errStr == "" {
		t.Error("Invalid result should have non-empty error string")
	}
	if result.Valid {
		t.Error("AddError should set Valid to false")
	}
}

func TestValidator_ValidDurations(t *testing.T) {
	v := NewValidator()

	validDurations := []string{"90m", "2h", "1h30m", "120", ""}

	for _, dur := range validDurations {
		template := &models.LabTemplate{
			APIVersion: "v1",
			Kind:       "LabTemplate",
			Metadata: models.LabMetadata{
				Name:        "test-lab",
				Description: "A test lab description",
				Duration:    dur,
			},
			Spec: models.LabSpec{
				VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
			},
		}

		result := v.Validate(template)
		hasDurationError := false
		for _, err := range result.Errors {
			if err.Field == "metadata.duration" {
				hasDurationError = true
				break
			}
		}
		if hasDurationError {
			t.Errorf("Duration %q should be valid", dur)
		}
	}
}

func TestValidator_ValidVersion(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"v1.0", true},
		{"v1.0.0", true},
		{"1.0", true},
		{"1.0.0-beta", true},
		{"invalid", false},
	}

	for _, tc := range tests {
		template := &models.LabTemplate{
			APIVersion: "v1",
			Kind:       "LabTemplate",
			Metadata: models.LabMetadata{
				Name:        "test-lab",
				Description: "A test lab description",
				Version:     tc.version,
			},
			Spec: models.LabSpec{
				VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
			},
		}

		result := v.Validate(template)
		hasVersionError := false
		for _, err := range result.Errors {
			if err.Field == "metadata.version" {
				hasVersionError = true
				break
			}
		}

		if tc.valid && hasVersionError {
			t.Errorf("Version %q should be valid", tc.version)
		}
		if !tc.valid && !hasVersionError {
			t.Errorf("Version %q should be invalid", tc.version)
		}
	}
}

// -----------------------------------------------------------------------------
// Checkpoint Validation Tests
// -----------------------------------------------------------------------------

func TestValidator_CheckpointConfig(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name          string
		passThreshold int
		shouldError   bool
	}{
		{"valid threshold 0", 0, false},
		{"valid threshold 70", 70, false},
		{"valid threshold 100", 100, false},
		{"invalid threshold -1", -1, true},
		{"invalid threshold 101", 101, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := &models.LabTemplate{
				APIVersion: "v1",
				Kind:       "LabTemplate",
				Metadata: models.LabMetadata{
					Name:        "test-lab",
					Description: "A test lab description",
				},
				Spec: models.LabSpec{
					VMs: []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
					Checkpoints: &models.CheckpointConfig{
						PassThreshold: tc.passThreshold,
					},
				},
			}

			result := v.Validate(template)
			hasThresholdError := false
			for _, err := range result.Errors {
				if err.Field == "spec.checkpoints.passThreshold" {
					hasThresholdError = true
					break
				}
			}

			if tc.shouldError && !hasThresholdError {
				t.Errorf("expected passThreshold error for %d", tc.passThreshold)
			}
			if !tc.shouldError && hasThresholdError {
				t.Errorf("unexpected passThreshold error for %d", tc.passThreshold)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Objective Validation Tests
// -----------------------------------------------------------------------------

func TestValidator_ObjectiveValidation(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		objective   models.Checkpoint
		expectError bool
		errorField  string
	}{
		{
			name: "valid objective",
			objective: models.Checkpoint{
				ID:          "obj-1",
				Description: "Valid objective",
				Points:      10,
			},
			expectError: false,
		},
		{
			name: "missing id",
			objective: models.Checkpoint{
				ID:          "",
				Description: "Objective without ID",
				Points:      10,
			},
			expectError: true,
			errorField:  ".id",
		},
		{
			name: "missing description",
			objective: models.Checkpoint{
				ID:          "obj-2",
				Description: "",
				Points:      10,
			},
			expectError: true,
			errorField:  ".description",
		},
		{
			name: "negative points",
			objective: models.Checkpoint{
				ID:          "obj-3",
				Description: "Negative points",
				Points:      -5,
			},
			expectError: true,
			errorField:  ".points",
		},
		{
			name: "zero points valid",
			objective: models.Checkpoint{
				ID:          "obj-4",
				Description: "Zero points",
				Points:      0,
			},
			expectError: false,
		},
		{
			name: "valid trigger type",
			objective: models.Checkpoint{
				ID:          "obj-5",
				Description: "With valid trigger",
				Points:      10,
				Triggers: []models.CheckpointTrigger{
					{Type: models.TriggerTypeFileExists, Target: "vm1"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid trigger type",
			objective: models.Checkpoint{
				ID:          "obj-6",
				Description: "With invalid trigger",
				Points:      10,
				Triggers: []models.CheckpointTrigger{
					{Type: "invalid_type", Target: "vm1"},
				},
			},
			expectError: true,
			errorField:  ".triggers[0].type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := &models.LabTemplate{
				APIVersion: "v1",
				Kind:       "LabTemplate",
				Metadata: models.LabMetadata{
					Name:        "test-lab",
					Description: "A test lab description",
				},
				Spec: models.LabSpec{
					VMs:        []models.VMSpec{{Name: "vm1", Template: "ubuntu"}},
					Objectives: []models.Checkpoint{tc.objective},
				},
			}

			result := v.Validate(template)
			hasExpectedError := false
			for _, err := range result.Errors {
				if tc.errorField != "" && containsString(err.Field, tc.errorField) {
					hasExpectedError = true
					break
				}
			}

			if tc.expectError && !hasExpectedError {
				t.Errorf("expected error containing %q", tc.errorField)
			}
			if !tc.expectError && len(result.Errors) > 0 {
				// Check if errors are specifically about our objective
				for _, err := range result.Errors {
					if containsString(err.Field, "objectives[0]") {
						t.Errorf("unexpected error for valid objective: %v", err)
					}
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Reference Validation Tests
// -----------------------------------------------------------------------------

func TestValidator_ReferenceValidation(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		vms         []models.VMSpec
		objectives  []models.Checkpoint
		expectError bool
	}{
		{
			name: "valid reference",
			vms:  []models.VMSpec{{Name: "server", Template: "ubuntu"}},
			objectives: []models.Checkpoint{
				{
					ID:          "obj-1",
					Description: "Check server",
					Triggers: []models.CheckpointTrigger{
						{Target: "server"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid reference",
			vms:  []models.VMSpec{{Name: "server", Template: "ubuntu"}},
			objectives: []models.Checkpoint{
				{
					ID:          "obj-1",
					Description: "Check client",
					Triggers: []models.CheckpointTrigger{
						{Target: "client"}, // Non-existent VM
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty target is valid",
			vms:  []models.VMSpec{{Name: "server", Template: "ubuntu"}},
			objectives: []models.Checkpoint{
				{
					ID:          "obj-1",
					Description: "Check something",
					Triggers: []models.CheckpointTrigger{
						{Target: ""}, // Empty target should be allowed
					},
				},
			},
			expectError: false,
		},
		{
			name: "multiple triggers mixed validity",
			vms: []models.VMSpec{
				{Name: "server", Template: "ubuntu"},
				{Name: "client", Template: "centos"},
			},
			objectives: []models.Checkpoint{
				{
					ID:          "obj-1",
					Description: "Multiple checks",
					Triggers: []models.CheckpointTrigger{
						{Target: "server"},  // Valid
						{Target: "client"},  // Valid
						{Target: "gateway"}, // Invalid
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := &models.LabTemplate{
				APIVersion: "v1",
				Kind:       "LabTemplate",
				Metadata: models.LabMetadata{
					Name:        "test-lab",
					Description: "A test lab description",
				},
				Spec: models.LabSpec{
					VMs:        tc.vms,
					Objectives: tc.objectives,
				},
			}

			result := v.Validate(template)
			hasReferenceError := false
			for _, err := range result.Errors {
				if containsString(err.Message, "references undefined VM") {
					hasReferenceError = true
					break
				}
			}

			if tc.expectError && !hasReferenceError {
				t.Error("expected reference validation error")
			}
			if !tc.expectError && hasReferenceError {
				t.Error("unexpected reference validation error")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// All Trigger Types Test
// -----------------------------------------------------------------------------

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "test.field",
		Message: "test message",
		Value:   "test value",
	}

	errStr := err.Error()
	if !containsString(errStr, "test.field") {
		t.Error("error string should contain field name")
	}
	if !containsString(errStr, "test message") {
		t.Error("error string should contain message")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

// TestAllTriggerTypesAccepted guards against the validator's accepted set
// drifting behind models.AllTriggerTypes again. It previously carried its own
// hand-written list and fell seven types behind the checkpoint evaluator, so
// disk_usage, cpu_load, memory_usage, process_running, port_listening,
// cron_job and firewall_rule were implemented but unusable.
func TestAllTriggerTypesAccepted(t *testing.T) {
	// Floor: an emptied AllTriggerTypes would otherwise make every assertion
	// below vacuous and the test would still pass.
	if len(models.AllTriggerTypes) < 18 {
		t.Fatalf("AllTriggerTypes has %d entries, expected at least 18",
			len(models.AllTriggerTypes))
	}

	for _, triggerType := range models.AllTriggerTypes {
		t.Run(string(triggerType), func(t *testing.T) {
			tmpl := &models.LabTemplate{
				APIVersion: models.CurrentAPIVersion,
				Kind:       "LabTemplate",
				Metadata: models.LabMetadata{
					Name:        "trigger-type-lab",
					Description: "Fixture exercising every declared trigger type",
					Duration:    "60m",
					Difficulty:  "beginner",
					Version:     "1.0.0",
				},
				Spec: models.LabSpec{
					Platform: models.PlatformProxmox,
					Network: models.NetworkSpec{
						Segments: []models.NetworkSegment{
							{Name: "net", Subnet: "10.0.0.0/24"},
						},
					},
					VMs: []models.VMSpec{
						{
							Name:      "vm",
							Template:  "ubuntu-22.04",
							Resources: models.ResourceSpec{CPU: 1, Memory: 512},
						},
					},
					Objectives: []models.Checkpoint{
						{
							ID:          "obj",
							Description: "Objective for the trigger-type fixture",
							Points:      10,
							Triggers: []models.CheckpointTrigger{
								{Type: triggerType, Target: "vm"},
							},
						},
					},
				},
			}

			// Assert the whole template validates, not just that no .type
			// error appeared. A filtered negative would pass even if the
			// fixture were invalid for some unrelated reason.
			result := NewValidator().Validate(tmpl)
			if !result.Valid {
				t.Errorf("trigger type %q: template did not validate: %v", triggerType, result.Errors)
			}
		})
	}
}
