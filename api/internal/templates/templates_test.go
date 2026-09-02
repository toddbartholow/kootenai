package templates

import (
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Loader Helper Tests
// -----------------------------------------------------------------------------

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "Network Basics",
			expected: "network-basics",
		},
		{
			name:     "already lowercase",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "with special characters",
			input:    "DNS & DHCP Lab!",
			expected: "dns--dhcp-lab",
		},
		{
			name:     "multiple spaces",
			input:    "Multiple   Spaces   Here",
			expected: "multiple---spaces---here",
		},
		{
			name:     "numbers included",
			input:    "Lab 101 Advanced",
			expected: "lab-101-advanced",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special chars",
			input:    "!@#$%",
			expected: "",
		},
		{
			name:     "mixed case with numbers",
			input:    "IPv4 vs IPv6",
			expected: "ipv4-vs-ipv6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSlug(tt.input)
			if result != tt.expected {
				t.Errorf("generateSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "minutes format",
			input:    "90m",
			expected: 90,
		},
		{
			name:     "hours format",
			input:    "2h",
			expected: 120,
		},
		{
			name:     "hours and minutes",
			input:    "1h30m",
			expected: 90,
		},
		{
			name:     "plain number",
			input:    "60",
			expected: 60,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "with whitespace",
			input:    "  45  ",
			expected: 45,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "seconds format",
			input:    "3600s",
			expected: 60,
		},
		{
			name:     "half hour",
			input:    "30m",
			expected: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDuration(tt.input)
			if result != tt.expected {
				t.Errorf("parseDuration(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// VariableProcessor Tests
// -----------------------------------------------------------------------------

func TestNewVariableProcessor(t *testing.T) {
	vars := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	}
	p := NewVariableProcessor(vars)

	if p == nil {
		t.Fatal("expected non-nil processor")
	}
	if len(p.variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(p.variables))
	}
}

func TestVariableProcessor_Substitute(t *testing.T) {
	vars := map[string]string{
		"NAME":    "test-lab",
		"VERSION": "1.0",
		"SUBNET":  "10.0.0.0/24",
	}
	p := NewVariableProcessor(vars)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "braced variable",
			input:    "${NAME}",
			expected: "test-lab",
		},
		{
			name:     "simple variable",
			input:    "$NAME",
			expected: "test-lab",
		},
		{
			name:     "multiple variables",
			input:    "${NAME} version ${VERSION}",
			expected: "test-lab version 1.0",
		},
		{
			name:     "mixed formats",
			input:    "$NAME-${VERSION}",
			expected: "test-lab-1.0",
		},
		{
			name:     "unknown variable - not replaced",
			input:    "${UNKNOWN}",
			expected: "${UNKNOWN}",
		},
		{
			name:     "no variables",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "variable in sentence",
			input:    "Using subnet $SUBNET for networking",
			expected: "Using subnet 10.0.0.0/24 for networking",
		},
		{
			name:     "adjacent variables",
			input:    "${NAME}${VERSION}",
			expected: "test-lab1.0",
		},
		{
			name:     "dollar sign without variable",
			input:    "$100 dollars",
			expected: "$100 dollars", // $1 is not a valid var name start
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.substitute(tt.input)
			if result != tt.expected {
				t.Errorf("substitute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestVariableProcessor_Process(t *testing.T) {
	vars := map[string]string{
		"LAB_NAME":    "Test Lab",
		"TEMPLATE_ID": "ubuntu-22.04",
		"SUBNET":      "192.168.1.0/24",
		"GATEWAY":     "192.168.1.1",
	}
	p := NewVariableProcessor(vars)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "${LAB_NAME}",
			Description: "A lab using $TEMPLATE_ID",
		},
		Spec: models.LabSpec{
			VMs: []models.VMSpec{
				{
					Name:     "server-${LAB_NAME}",
					Template: "${TEMPLATE_ID}",
				},
			},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{
						Name:    "internal",
						Subnet:  "${SUBNET}",
						Gateway: "${GATEWAY}",
					},
				},
			},
			Objectives: []models.Checkpoint{
				{
					ID:          "obj-${LAB_NAME}",
					Description: "Complete task in ${LAB_NAME}",
					Hint:        "Check $GATEWAY",
					Triggers: []models.CheckpointTrigger{
						{
							Target: "${GATEWAY}",
						},
					},
				},
			},
		},
	}

	result := p.Process(template)

	// Check metadata
	if result.Metadata.Name != "Test Lab" {
		t.Errorf("expected Name 'Test Lab', got %q", result.Metadata.Name)
	}
	if result.Metadata.Description != "A lab using ubuntu-22.04" {
		t.Errorf("expected Description 'A lab using ubuntu-22.04', got %q", result.Metadata.Description)
	}

	// Check VMs
	if len(result.Spec.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(result.Spec.VMs))
	}
	if result.Spec.VMs[0].Name != "server-Test Lab" {
		t.Errorf("expected VM name 'server-Test Lab', got %q", result.Spec.VMs[0].Name)
	}
	if result.Spec.VMs[0].Template != "ubuntu-22.04" {
		t.Errorf("expected VM template 'ubuntu-22.04', got %q", result.Spec.VMs[0].Template)
	}

	// Check network
	if len(result.Spec.Network.Segments) != 1 {
		t.Fatalf("expected 1 network segment, got %d", len(result.Spec.Network.Segments))
	}
	if result.Spec.Network.Segments[0].Subnet != "192.168.1.0/24" {
		t.Errorf("expected subnet '192.168.1.0/24', got %q", result.Spec.Network.Segments[0].Subnet)
	}
	if result.Spec.Network.Segments[0].Gateway != "192.168.1.1" {
		t.Errorf("expected gateway '192.168.1.1', got %q", result.Spec.Network.Segments[0].Gateway)
	}

	// Check objectives
	if len(result.Spec.Objectives) != 1 {
		t.Fatalf("expected 1 objective, got %d", len(result.Spec.Objectives))
	}
	if result.Spec.Objectives[0].ID != "obj-Test Lab" {
		t.Errorf("expected objective ID 'obj-Test Lab', got %q", result.Spec.Objectives[0].ID)
	}
	if result.Spec.Objectives[0].Description != "Complete task in Test Lab" {
		t.Errorf("expected objective description, got %q", result.Spec.Objectives[0].Description)
	}
	if result.Spec.Objectives[0].Hint != "Check 192.168.1.1" {
		t.Errorf("expected objective hint, got %q", result.Spec.Objectives[0].Hint)
	}
	if result.Spec.Objectives[0].Triggers[0].Target != "192.168.1.1" {
		t.Errorf("expected trigger target, got %q", result.Spec.Objectives[0].Triggers[0].Target)
	}
}

func TestDefaultVariables(t *testing.T) {
	vars := DefaultVariables()

	if vars == nil {
		t.Fatal("expected non-nil variables")
	}

	// Check expected defaults
	expected := map[string]string{
		"LAB_SUBNET":  "10.0.0.0/24",
		"LAB_GATEWAY": "10.0.0.1",
		"LAB_DNS":     "10.0.0.1",
	}

	for k, v := range expected {
		if vars[k] != v {
			t.Errorf("expected %s=%q, got %q", k, v, vars[k])
		}
	}
}

// -----------------------------------------------------------------------------
// Helper Function Tests (from inheritance.go)
// -----------------------------------------------------------------------------

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{
			name:     "first non-empty",
			values:   []string{"first", "second"},
			expected: "first",
		},
		{
			name:     "second non-empty",
			values:   []string{"", "second"},
			expected: "second",
		},
		{
			name:     "all empty",
			values:   []string{"", "", ""},
			expected: "",
		},
		{
			name:     "single value",
			values:   []string{"only"},
			expected: "only",
		},
		{
			name:     "no values",
			values:   []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesce(tt.values...)
			if result != tt.expected {
				t.Errorf("coalesce(%v) = %q, want %q", tt.values, result, tt.expected)
			}
		})
	}
}

func TestCoalesceP(t *testing.T) {
	tests := []struct {
		name     string
		values   []models.Platform
		expected models.Platform
	}{
		{
			name:     "first non-empty",
			values:   []models.Platform{models.PlatformProxmox, models.PlatformCloudStack},
			expected: models.PlatformProxmox,
		},
		{
			name:     "second non-empty",
			values:   []models.Platform{"", models.PlatformCloudStack},
			expected: models.PlatformCloudStack,
		},
		{
			name:     "all empty",
			values:   []models.Platform{"", ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesceP(tt.values...)
			if result != tt.expected {
				t.Errorf("coalesceP(%v) = %q, want %q", tt.values, result, tt.expected)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		name     string
		parent   []string
		child    []string
		expected []string
	}{
		{
			name:     "distinct tags",
			parent:   []string{"networking", "security"},
			child:    []string{"advanced", "lab"},
			expected: []string{"networking", "security", "advanced", "lab"},
		},
		{
			name:     "overlapping tags",
			parent:   []string{"networking", "security"},
			child:    []string{"security", "advanced"}, // "security" is duplicate
			expected: []string{"networking", "security", "advanced"},
		},
		{
			name:     "empty parent",
			parent:   []string{},
			child:    []string{"tag1", "tag2"},
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "empty child",
			parent:   []string{"tag1", "tag2"},
			child:    []string{},
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "both empty",
			parent:   []string{},
			child:    []string{},
			expected: nil, // Note: nil slice when nothing to add
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeTags(tt.parent, tt.child)

			if len(result) != len(tt.expected) {
				t.Errorf("mergeTags() len = %d, want %d", len(result), len(tt.expected))
				return
			}

			// Create a set of expected values for comparison
			expectedSet := make(map[string]bool)
			for _, tag := range tt.expected {
				expectedSet[tag] = true
			}

			for _, tag := range result {
				if !expectedSet[tag] {
					t.Errorf("mergeTags() unexpected tag %q", tag)
				}
			}
		})
	}
}

func TestMergeVMs(t *testing.T) {
	tests := []struct {
		name        string
		parent      []models.VMSpec
		child       []models.VMSpec
		expectCount int
		expectNames []string
	}{
		{
			name: "child overrides parent VM",
			parent: []models.VMSpec{
				{Name: "server", Template: "ubuntu-20.04"},
			},
			child: []models.VMSpec{
				{Name: "server", Template: "ubuntu-22.04"},
			},
			expectCount: 1,
			expectNames: []string{"server"},
		},
		{
			name: "child adds new VM",
			parent: []models.VMSpec{
				{Name: "server", Template: "ubuntu-20.04"},
			},
			child: []models.VMSpec{
				{Name: "client", Template: "ubuntu-22.04"},
			},
			expectCount: 2,
			expectNames: []string{"server", "client"},
		},
		{
			name:        "empty parent",
			parent:      []models.VMSpec{},
			child:       []models.VMSpec{{Name: "server", Template: "ubuntu-22.04"}},
			expectCount: 1,
			expectNames: []string{"server"},
		},
		{
			name:        "empty child",
			parent:      []models.VMSpec{{Name: "server", Template: "ubuntu-22.04"}},
			child:       []models.VMSpec{},
			expectCount: 1,
			expectNames: []string{"server"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeVMs(tt.parent, tt.child)

			if len(result) != tt.expectCount {
				t.Errorf("mergeVMs() len = %d, want %d", len(result), tt.expectCount)
			}

			// Check that expected names exist
			nameSet := make(map[string]bool)
			for _, vm := range result {
				nameSet[vm.Name] = true
			}
			for _, name := range tt.expectNames {
				if !nameSet[name] {
					t.Errorf("mergeVMs() missing VM %q", name)
				}
			}
		})
	}
}

func TestMergeNetwork(t *testing.T) {
	tests := []struct {
		name           string
		parent         models.NetworkSpec
		child          models.NetworkSpec
		expectSegments int
	}{
		{
			name: "child overrides segment",
			parent: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal", Subnet: "10.0.0.0/24"},
				},
			},
			child: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal", Subnet: "192.168.0.0/24"}, // Override
				},
			},
			expectSegments: 1,
		},
		{
			name: "child adds segment",
			parent: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal", Subnet: "10.0.0.0/24"},
				},
			},
			child: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "external", Subnet: "192.168.0.0/24"}, // New
				},
			},
			expectSegments: 2,
		},
		{
			name: "empty child keeps parent",
			parent: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal", Subnet: "10.0.0.0/24"},
				},
			},
			child:          models.NetworkSpec{},
			expectSegments: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeNetwork(tt.parent, tt.child)

			if len(result.Segments) != tt.expectSegments {
				t.Errorf("mergeNetwork() segments = %d, want %d", len(result.Segments), tt.expectSegments)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// InheritanceResolver Tests
// -----------------------------------------------------------------------------

func TestNewInheritanceResolver(t *testing.T) {
	resolver := NewInheritanceResolver(nil)

	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
	if resolver.cache == nil {
		t.Error("expected non-nil cache")
	}
	if resolver.loaded == nil {
		t.Error("expected non-nil loaded map")
	}
}

func TestInheritanceResolver_Merge(t *testing.T) {
	resolver := NewInheritanceResolver(nil)

	parent := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "Parent Lab",
			Description: "Parent description",
			Duration:    "60m",
			Difficulty:  "intermediate",
			Version:     "1.0",
			Author:      "Parent Author",
			Tags:        []string{"networking"},
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs: []models.VMSpec{
				{Name: "server", Template: "ubuntu-20.04"},
			},
			Network: models.NetworkSpec{
				Segments: []models.NetworkSegment{
					{Name: "internal", Subnet: "10.0.0.0/24"},
				},
			},
			Checkpoints: &models.CheckpointConfig{
				PassThreshold: 70,
			},
			Objectives: []models.Checkpoint{
				{ID: "obj1", Description: "Parent objective", Points: 10},
			},
		},
	}

	child := &models.LabTemplate{
		APIVersion: "v2",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name: "Child Lab",
			// Description intentionally empty - should inherit from parent
			Tags: []string{"security"},
		},
		Spec: models.LabSpec{
			// Platform empty - should inherit from parent
			VMs: []models.VMSpec{
				{Name: "client", Template: "kali-linux"},
			},
			Objectives: []models.Checkpoint{
				{ID: "obj2", Description: "Child objective", Points: 20},
			},
		},
	}

	result := resolver.merge(parent, child)

	// Check APIVersion comes from child
	if result.APIVersion != "v2" {
		t.Errorf("expected APIVersion 'v2', got %q", result.APIVersion)
	}

	// Check metadata inheritance
	if result.Metadata.Name != "Child Lab" {
		t.Errorf("expected Name 'Child Lab', got %q", result.Metadata.Name)
	}
	if result.Metadata.Description != "Parent description" {
		t.Errorf("expected inherited description, got %q", result.Metadata.Description)
	}
	if result.Metadata.Duration != "60m" {
		t.Errorf("expected inherited duration '60m', got %q", result.Metadata.Duration)
	}

	// Check platform inheritance
	if result.Spec.Platform != models.PlatformProxmox {
		t.Errorf("expected inherited platform 'proxmox', got %q", result.Spec.Platform)
	}

	// Check VMs merged
	if len(result.Spec.VMs) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(result.Spec.VMs))
	}

	// Check objectives appended
	if len(result.Spec.Objectives) != 2 {
		t.Errorf("expected 2 objectives, got %d", len(result.Spec.Objectives))
	}

	// Check tags merged
	if len(result.Metadata.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(result.Metadata.Tags))
	}
}

// -----------------------------------------------------------------------------
// Loader Option Tests
// -----------------------------------------------------------------------------

func TestWithStrictMode(t *testing.T) {
	loader := &Loader{}
	opt := WithStrictMode()
	opt(loader)

	if !loader.strictMode {
		t.Error("expected strictMode to be true")
	}
}

func TestWithValidator(t *testing.T) {
	loader := &Loader{}
	// Create a custom validator (we can use nil for this test)
	opt := WithValidator(nil)
	opt(loader)

	if loader.validator != nil {
		t.Error("expected validator to be nil")
	}
}
