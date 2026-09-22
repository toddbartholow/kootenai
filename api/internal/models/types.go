// Package models contains shared data types for the lab platform
package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// -----------------------------------------------------------------------------
// Enums and Constants
// -----------------------------------------------------------------------------

// Platform represents the virtualization platform
type Platform string

const (
	PlatformProxmox    Platform = "proxmox"
	PlatformCloudStack Platform = "cloudstack"
	PlatformAny        Platform = "any"
)

// -----------------------------------------------------------------------------
// Lab Template Types
// -----------------------------------------------------------------------------

// CurrentAPIVersion is the schema identifier written into every lab template
// this codebase emits.
const CurrentAPIVersion = "v1"

// SupportedAPIVersions lists the apiVersion values a lab template may declare.
//
// "virtuallab.dev/v1" is retained for backwards compatibility only: it was the
// Python parser's default for the project's first few months and is baked into
// templates authored in that window. That domain belongs to a third party, not
// to this project, so it is never emitted for anything new.
//
// Keep this in sync with SUPPORTED_API_VERSIONS in python/lab_templates/parser.py.
var SupportedAPIVersions = map[string]bool{
	CurrentAPIVersion:   true,
	"virtuallab.dev/v1": true,
}

// LabTemplate defines a lab topology
type LabTemplate struct {
	APIVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   LabMetadata `yaml:"metadata" json:"metadata"`
	Spec       LabSpec     `yaml:"spec" json:"spec"`
}

// LabMetadata contains lab template metadata
type LabMetadata struct {
	Name         string            `yaml:"name" json:"name"`
	Description  string            `yaml:"description" json:"description"`
	Duration     string            `yaml:"duration" json:"duration"`
	Difficulty   string            `yaml:"difficulty" json:"difficulty"`
	Category     string            `yaml:"category,omitempty" json:"category,omitempty"`
	Tags         []string          `yaml:"tags" json:"tags"`
	Version      string            `yaml:"version,omitempty" json:"version,omitempty"`
	Author       string            `yaml:"author,omitempty" json:"author,omitempty"`
	Extends      string            `yaml:"extends,omitempty" json:"extends,omitempty"`     // Parent template name for inheritance
	Variables    map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"` // Template variables for substitution
	Instructions *LabInstructions  `yaml:"instructions,omitempty" json:"instructions,omitempty"`
}

// LabInstructions contains educational content for a lab
type LabInstructions struct {
	Overview           string                `yaml:"overview,omitempty" json:"overview,omitempty"`
	LearningObjectives []string              `yaml:"learning_objectives,omitempty" json:"learning_objectives,omitempty"`
	Prerequisites      []string              `yaml:"prerequisites,omitempty" json:"prerequisites,omitempty"`
	Steps              []InstructionStep     `yaml:"steps,omitempty" json:"steps,omitempty"`
	Summary            string                `yaml:"summary,omitempty" json:"summary,omitempty"`
	Tips               []string              `yaml:"tips,omitempty" json:"tips,omitempty"`
	Resources          []InstructionResource `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// InstructionStep represents a single step in lab instructions
type InstructionStep struct {
	ID          string `yaml:"id" json:"id"`
	Title       string `yaml:"title" json:"title"`
	ObjectiveID string `yaml:"objective_id,omitempty" json:"objective_id,omitempty"`
	Content     string `yaml:"content" json:"content"`
}

// InstructionResource represents an external resource link
type InstructionResource struct {
	Title string `yaml:"title" json:"title"`
	URL   string `yaml:"url" json:"url"`
}

// LabSpec defines the lab specification
type LabSpec struct {
	Platform    Platform            `yaml:"platform" json:"platform"`
	Network     NetworkSpec         `yaml:"network" json:"network"`
	VMs         []VMSpec            `yaml:"vms" json:"vms"`
	Checkpoints *CheckpointConfig   `yaml:"checkpoints,omitempty" json:"checkpoints,omitempty"`
	Objectives  []Checkpoint        `yaml:"objectives,omitempty" json:"objectives,omitempty"`
	Assessment  *AssessmentTemplate `yaml:"assessment,omitempty" json:"assessment,omitempty"` // Packet Tracer-style active checks
	Questions   []Question          `yaml:"questions,omitempty" json:"questions,omitempty"`   // Manual Q&A assessments
}

// NetworkSpec defines the network topology
type NetworkSpec struct {
	Segments []NetworkSegment `yaml:"segments" json:"segments"`
}

// NetworkSegment defines a network segment/VLAN
type NetworkSegment struct {
	Name    string `yaml:"name" json:"name"`
	VLAN    int    `yaml:"vlan" json:"vlan"`
	Subnet  string `yaml:"subnet" json:"subnet"`
	Gateway string `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	DHCP    bool   `yaml:"dhcp,omitempty" json:"dhcp,omitempty"`
}

// VMSpec defines a virtual machine in the lab
type VMSpec struct {
	Name          string          `yaml:"name" json:"name"`
	Template      string          `yaml:"template" json:"template"`
	Resources     ResourceSpec    `yaml:"resources" json:"resources"`
	Networks      []VMNetworkSpec `yaml:"networks" json:"networks"`
	Snapshots     []SnapshotSpec  `yaml:"snapshots,omitempty" json:"snapshots,omitempty"`
	StartOnCreate bool            `yaml:"startOnCreate,omitempty" json:"startOnCreate,omitempty"`
	WazuhAgent    bool            `yaml:"wazuhAgent,omitempty" json:"wazuhAgent,omitempty"`
	AgentConfig   *AgentConfig    `yaml:"agentConfig,omitempty" json:"agentConfig,omitempty"`
}

// AgentConfig defines Wazuh agent configuration for a VM
type AgentConfig struct {
	MonitorPaths  []string `yaml:"monitor_paths,omitempty" json:"monitorPaths,omitempty"`
	AuditCommands bool     `yaml:"audit_commands,omitempty" json:"auditCommands,omitempty"`
	AuditSyscalls bool     `yaml:"audit_syscalls,omitempty" json:"auditSyscalls,omitempty"`
	RealTime      bool     `yaml:"realtime,omitempty" json:"realtime,omitempty"`
}

// ResourceSpec defines VM resource allocation
type ResourceSpec struct {
	CPU    int `yaml:"cpu" json:"cpu"`
	Memory int `yaml:"memory" json:"memory"`
	Disk   int `yaml:"disk,omitempty" json:"disk,omitempty"`
}

// VMNetworkSpec defines a VM's network connection
type VMNetworkSpec struct {
	Segment string `yaml:"segment" json:"segment"`
	IP      string `yaml:"ip,omitempty" json:"ip,omitempty"`
}

// SnapshotSpec defines a named snapshot point
type SnapshotSpec struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Default     bool   `yaml:"default,omitempty" json:"default,omitempty"`
	IncludeRAM  bool   `yaml:"includeRAM,omitempty" json:"includeRAM,omitempty"`
}

// -----------------------------------------------------------------------------
// Lab Template Database Record
// -----------------------------------------------------------------------------

// LabTemplateRecord represents a lab template stored in the database
type LabTemplateRecord struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"` // URL-friendly identifier
	Description     string          `json:"description,omitempty"`
	Version         string          `json:"version"`
	Platform        Platform        `json:"platform"`
	DurationMinutes int             `json:"durationMinutes,omitempty"`
	Difficulty      string          `json:"difficulty,omitempty"`
	Category        string          `json:"category,omitempty"` // Lab category for achievements
	Tags            []string        `json:"tags,omitempty"`     // Flexible tags for categorization
	MaxPoints       int             `json:"maxPoints"`
	PassThreshold   int             `json:"passThreshold"`
	Spec            json.RawMessage `json:"spec"`         // Full LabSpec as JSON
	Checkpoints     json.RawMessage `json:"checkpoints"`  // Checkpoint definitions as JSON
	Instructions    json.RawMessage `json:"instructions"` // Educational content as JSON
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	IsActive        bool            `json:"isActive"`
	// Multi-tenancy fields
	OrganizationID *string       `json:"organizationId,omitempty"`
	Visibility     LabVisibility `json:"visibility"`
	CreatedBy      *string       `json:"createdBy,omitempty"`
	// Edition control
	MinEdition Edition `json:"minEdition"`
}

// ToLabTemplate converts a database record to a LabTemplate
func (r *LabTemplateRecord) ToLabTemplate() (*LabTemplate, error) {
	var spec LabSpec
	if err := json.Unmarshal(r.Spec, &spec); err != nil {
		return nil, err
	}

	// Also unmarshal checkpoints (objectives) from the separate column
	if len(r.Checkpoints) > 0 {
		var objectives []Checkpoint
		if err := json.Unmarshal(r.Checkpoints, &objectives); err != nil {
			return nil, err
		}
		spec.Objectives = objectives
	}

	// Unmarshal instructions from the separate column
	var instructions *LabInstructions
	if len(r.Instructions) > 0 {
		instructions = &LabInstructions{}
		if err := json.Unmarshal(r.Instructions, instructions); err != nil {
			return nil, err
		}
	}

	return &LabTemplate{
		APIVersion: CurrentAPIVersion,
		Kind:       "LabTemplate",
		Metadata: LabMetadata{
			Name:         r.Name,
			Description:  r.Description,
			Duration:     formatDuration(r.DurationMinutes),
			Difficulty:   r.Difficulty,
			Version:      r.Version,
			Instructions: instructions,
		},
		Spec: spec,
	}, nil
}

// NewLabTemplateRecord creates a database record from a LabTemplate
func NewLabTemplateRecord(template *LabTemplate) (*LabTemplateRecord, error) {
	spec, err := json.Marshal(template.Spec)
	if err != nil {
		return nil, err
	}

	// Marshal objectives (checkpoints) separately for easy access
	var checkpoints json.RawMessage
	if len(template.Spec.Objectives) > 0 {
		checkpoints, err = json.Marshal(template.Spec.Objectives)
		if err != nil {
			return nil, err
		}
	}

	// Calculate max points from objectives
	maxPoints := 0
	for _, obj := range template.Spec.Objectives {
		maxPoints += obj.Points
	}

	// Get pass threshold from checkpoint config or use default
	passThreshold := 70
	if template.Spec.Checkpoints != nil && template.Spec.Checkpoints.PassThreshold > 0 {
		passThreshold = template.Spec.Checkpoints.PassThreshold
	}

	// Instructions live in their own column, so they marshal separately from Spec.
	var instructions json.RawMessage
	if template.Metadata.Instructions != nil {
		instructions, err = json.Marshal(template.Metadata.Instructions)
		if err != nil {
			return nil, err
		}
	}

	return &LabTemplateRecord{
		Name:            template.Metadata.Name,
		Description:     template.Metadata.Description,
		Version:         template.Metadata.Version,
		Platform:        template.Spec.Platform,
		DurationMinutes: parseDuration(template.Metadata.Duration),
		Difficulty:      template.Metadata.Difficulty,
		Category:        template.Metadata.Category,
		Tags:            template.Metadata.Tags,
		MaxPoints:       maxPoints,
		PassThreshold:   passThreshold,
		Spec:            spec,
		Checkpoints:     checkpoints,
		Instructions:    instructions,
		IsActive:        true,
	}, nil
}

// LabTemplateVersion represents a versioned snapshot of a lab template
type LabTemplateVersion struct {
	ID              string          `json:"id"`
	LabTemplateID   string          `json:"labTemplateId"`
	VersionNumber   int             `json:"versionNumber"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description,omitempty"`
	Version         string          `json:"version"`
	Platform        Platform        `json:"platform"`
	DurationMinutes int             `json:"durationMinutes,omitempty"`
	Difficulty      string          `json:"difficulty,omitempty"`
	Category        string          `json:"category,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	MaxPoints       int             `json:"maxPoints"`
	PassThreshold   int             `json:"passThreshold"`
	Spec            json.RawMessage `json:"spec"`
	Checkpoints     json.RawMessage `json:"checkpoints"`
	Instructions    json.RawMessage `json:"instructions"`
	IsActive        bool            `json:"isActive"`
	OrganizationID  *string         `json:"organizationId,omitempty"`
	Visibility      LabVisibility   `json:"visibility"`
	CreatedBy       *string         `json:"createdBy,omitempty"`
	MinEdition      Edition         `json:"minEdition"`
	ChangeSummary   string          `json:"changeSummary,omitempty"`
	CreatedByUserID *string         `json:"createdByUserId,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

// ToLabTemplateRecord converts a version snapshot back to a LabTemplateRecord for restore
func (v *LabTemplateVersion) ToLabTemplateRecord() *LabTemplateRecord {
	return &LabTemplateRecord{
		ID:              v.LabTemplateID,
		Name:            v.Name,
		Slug:            v.Slug,
		Description:     v.Description,
		Version:         v.Version,
		Platform:        v.Platform,
		DurationMinutes: v.DurationMinutes,
		Difficulty:      v.Difficulty,
		Category:        v.Category,
		Tags:            v.Tags,
		MaxPoints:       v.MaxPoints,
		PassThreshold:   v.PassThreshold,
		Spec:            v.Spec,
		Checkpoints:     v.Checkpoints,
		Instructions:    v.Instructions,
		IsActive:        v.IsActive,
		OrganizationID:  v.OrganizationID,
		Visibility:      v.Visibility,
		CreatedBy:       v.CreatedBy,
		MinEdition:      v.MinEdition,
	}
}

// formatDuration converts minutes to a human-readable string
func formatDuration(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// ParseDurationPublic parses a duration string like "2h" or "90m" to minutes (exported)
func ParseDurationPublic(s string) int {
	return parseDuration(s)
}

// parseDuration parses a duration string like "2h" or "90m" to minutes
func parseDuration(s string) int {
	if s == "" {
		return 0
	}
	// Simple parsing for common formats
	var value int
	var unit string
	_, err := fmt.Sscanf(s, "%d%s", &value, &unit)
	if err != nil {
		return 0
	}
	switch unit {
	case "h":
		return value * 60
	case "m":
		return value
	default:
		return 0
	}
}
