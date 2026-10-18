// Package validator provides YAML schema validation for lab templates
package validator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

func (e ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("%s: %s (got: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors"`
}

// AddError adds a validation error
func (r *ValidationResult) AddError(field, message string, value any) {
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
	r.Valid = false
}

// Error returns a combined error message
func (r *ValidationResult) Error() string {
	if r.Valid {
		return ""
	}
	var msgs []string
	for _, e := range r.Errors {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

// PlatformLimits defines resource limits for a platform
type PlatformLimits struct {
	MaxCPU      int               // Maximum vCPUs per VM
	MaxMemoryMB int               // Maximum memory in MB per VM
	MaxDiskGB   int               // Maximum disk size in GB per VM
	MaxVMs      int               // Maximum VMs per template
	MaxNetworks int               // Maximum networks per template
	Platforms   []models.Platform // Supported platforms
}

// DefaultLimits provides reasonable defaults for resource validation
var DefaultLimits = PlatformLimits{
	MaxCPU:      16,
	MaxMemoryMB: 65536, // 64GB
	MaxDiskGB:   500,
	MaxVMs:      10,
	MaxNetworks: 5,
	Platforms:   []models.Platform{models.PlatformProxmox, models.PlatformCloudStack, models.PlatformAny},
}

// Pre-compiled regexes for input validation (avoid per-call compilation).
var (
	reLabName   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	reVersion   = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?(-[a-zA-Z0-9]+)?$`)
	reDuration  = regexp.MustCompile(`^(\d+h)?(\d+m)?(\d+s)?$|^\d+$`)
	reCIDR      = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/\d{1,2}$`)
	reIPAddress = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
)

// Validator validates lab templates
type Validator struct {
	limits PlatformLimits
}

// NewValidator creates a new validator with default limits
func NewValidator() *Validator {
	return &Validator{
		limits: DefaultLimits,
	}
}

// NewValidatorWithLimits creates a validator with custom limits
func NewValidatorWithLimits(limits PlatformLimits) *Validator {
	return &Validator{
		limits: limits,
	}
}

// Validate performs comprehensive validation on a lab template
func (v *Validator) Validate(t *models.LabTemplate) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate structure
	v.validateStructure(t, result)

	// Validate metadata
	v.validateMetadata(&t.Metadata, result)

	// Validate spec
	v.validateSpec(&t.Spec, result)

	// Validate cross-references
	v.validateReferences(t, result)

	return result
}

// validateStructure checks required top-level fields
func (v *Validator) validateStructure(t *models.LabTemplate, result *ValidationResult) {
	if t.APIVersion == "" {
		result.AddError("apiVersion", "required field is missing", nil)
	} else if !models.SupportedAPIVersions[t.APIVersion] {
		result.AddError(
			"apiVersion",
			fmt.Sprintf("unsupported value; expected one of %s", supportedAPIVersionList()),
			t.APIVersion,
		)
	}

	if t.Kind == "" {
		result.AddError("kind", "required field is missing", nil)
	} else if t.Kind != "LabTemplate" {
		result.AddError("kind", "must be 'LabTemplate'", t.Kind)
	}
}

// supportedAPIVersionList renders the accepted apiVersion values in a stable
// order so validation messages don't vary between runs.
func supportedAPIVersionList() string {
	versions := make([]string, 0, len(models.SupportedAPIVersions))
	for v := range models.SupportedAPIVersions {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return strings.Join(versions, ", ")
}

// validateMetadata checks metadata fields
func (v *Validator) validateMetadata(m *models.LabMetadata, result *ValidationResult) {
	// Name validation
	switch {
	case m.Name == "":
		result.AddError("metadata.name", "required field is missing", nil)
	case len(m.Name) < 3:
		result.AddError("metadata.name", "must be at least 3 characters", m.Name)
	case len(m.Name) > 64:
		result.AddError("metadata.name", "must not exceed 64 characters", m.Name)
	default:
		if !reLabName.MatchString(m.Name) {
			result.AddError("metadata.name", "must contain only lowercase letters, numbers, and hyphens, and start/end with alphanumeric", m.Name)
		}
	}

	// Description validation
	if m.Description == "" {
		result.AddError("metadata.description", "required field is missing", nil)
	} else if len(m.Description) < 10 {
		result.AddError("metadata.description", "should be at least 10 characters for clarity", m.Description)
	}

	// Difficulty validation
	validDifficulties := map[string]bool{
		"beginner":     true,
		"intermediate": true,
		"advanced":     true,
		"expert":       true,
	}
	if m.Difficulty != "" && !validDifficulties[strings.ToLower(m.Difficulty)] {
		result.AddError("metadata.difficulty", "must be one of: beginner, intermediate, advanced, expert", m.Difficulty)
	}

	// Duration validation
	if m.Duration != "" {
		if !isValidDuration(m.Duration) {
			result.AddError("metadata.duration", "must be a valid duration (e.g., '90m', '2h', '1h30m')", m.Duration)
		}
	}

	// Version validation
	if m.Version != "" {
		if !reVersion.MatchString(m.Version) {
			result.AddError("metadata.version", "should follow semantic versioning (e.g., '1.0.0', 'v1.0')", m.Version)
		}
	}
}

// validateSpec checks spec fields
func (v *Validator) validateSpec(s *models.LabSpec, result *ValidationResult) {
	// Platform validation
	if s.Platform != "" {
		valid := false
		for _, p := range v.limits.Platforms {
			if s.Platform == p {
				valid = true
				break
			}
		}
		if !valid {
			result.AddError("spec.platform", "must be one of: proxmox, cloudstack, any", s.Platform)
		}
	}

	// VMs validation
	if len(s.VMs) == 0 {
		result.AddError("spec.vms", "at least one VM is required", nil)
	} else if len(s.VMs) > v.limits.MaxVMs {
		result.AddError("spec.vms", fmt.Sprintf("cannot exceed %d VMs", v.limits.MaxVMs), len(s.VMs))
	}

	vmNames := make(map[string]bool)
	for i, vm := range s.VMs {
		prefix := fmt.Sprintf("spec.vms[%d]", i)
		v.validateVM(&vm, prefix, vmNames, result)
	}

	// Network validation
	v.validateNetwork(&s.Network, vmNames, result)

	// Checkpoints validation
	if s.Checkpoints != nil {
		v.validateCheckpoints(s.Checkpoints, result)
	}

	// Objectives validation
	for i, obj := range s.Objectives {
		prefix := fmt.Sprintf("spec.objectives[%d]", i)
		v.validateObjective(&obj, prefix, result)
	}
}

// validateVM checks a single VM definition
func (v *Validator) validateVM(vm *models.VMSpec, prefix string, vmNames map[string]bool, result *ValidationResult) {
	// Name validation
	if vm.Name == "" {
		result.AddError(prefix+".name", "required field is missing", nil)
	} else {
		if vmNames[vm.Name] {
			result.AddError(prefix+".name", "duplicate VM name", vm.Name)
		}
		vmNames[vm.Name] = true
	}

	// Template validation
	if vm.Template == "" {
		result.AddError(prefix+".template", "required field is missing", nil)
	}

	// Resource validation (resources are in a nested struct)
	if vm.Resources.CPU > 0 && vm.Resources.CPU > v.limits.MaxCPU {
		result.AddError(prefix+".resources.cpu", fmt.Sprintf("cannot exceed %d vCPUs", v.limits.MaxCPU), vm.Resources.CPU)
	}

	if vm.Resources.Memory > 0 && vm.Resources.Memory > v.limits.MaxMemoryMB {
		result.AddError(prefix+".resources.memory", fmt.Sprintf("cannot exceed %d MB", v.limits.MaxMemoryMB), vm.Resources.Memory)
	}

	if vm.Resources.Disk > 0 && vm.Resources.Disk > v.limits.MaxDiskGB {
		result.AddError(prefix+".resources.disk", fmt.Sprintf("cannot exceed %d GB", v.limits.MaxDiskGB), vm.Resources.Disk)
	}
}

// validateNetwork checks network configuration
func (v *Validator) validateNetwork(n *models.NetworkSpec, vmNames map[string]bool, result *ValidationResult) {
	// Validate network segments
	segmentNames := make(map[string]bool)
	for i, seg := range n.Segments {
		prefix := fmt.Sprintf("spec.network.segments[%d]", i)

		// Name validation
		if seg.Name == "" {
			result.AddError(prefix+".name", "required field is missing", nil)
		} else {
			if segmentNames[seg.Name] {
				result.AddError(prefix+".name", "duplicate segment name", seg.Name)
			}
			segmentNames[seg.Name] = true
		}

		// VLAN validation
		if seg.VLAN != 0 && (seg.VLAN < 1 || seg.VLAN > 4094) {
			result.AddError(prefix+".vlan", "must be between 1 and 4094", seg.VLAN)
		}

		// Subnet validation
		if seg.Subnet != "" && !isValidCIDR(seg.Subnet) {
			result.AddError(prefix+".subnet", "must be a valid CIDR (e.g., '10.0.0.0/24')", seg.Subnet)
		}

		// Gateway validation
		if seg.Gateway != "" && !isValidIP(seg.Gateway) {
			result.AddError(prefix+".gateway", "must be a valid IP address", seg.Gateway)
		}
	}
}

// validateCheckpoints checks checkpoint configuration
func (v *Validator) validateCheckpoints(c *models.CheckpointConfig, result *ValidationResult) {
	if c.PassThreshold < 0 || c.PassThreshold > 100 {
		result.AddError("spec.checkpoints.passThreshold", "must be between 0 and 100", c.PassThreshold)
	}
}

// validateObjective checks a single objective/checkpoint
func (v *Validator) validateObjective(o *models.Checkpoint, prefix string, result *ValidationResult) {
	if o.ID == "" {
		result.AddError(prefix+".id", "required field is missing", nil)
	}

	if o.Description == "" {
		result.AddError(prefix+".description", "required field is missing", nil)
	}

	if o.Points < 0 {
		result.AddError(prefix+".points", "cannot be negative", o.Points)
	}

	// Validate triggers if present. Derived from models.AllTriggerTypes rather
	// than hand-listed here, which is how this fell seven types behind the
	// evaluator.
	validTypes := make(map[models.TriggerType]bool, len(models.AllTriggerTypes))
	for _, t := range models.AllTriggerTypes {
		validTypes[t] = true
	}

	for j, trigger := range o.Triggers {
		triggerPrefix := fmt.Sprintf("%s.triggers[%d]", prefix, j)
		if trigger.Type != "" && !validTypes[trigger.Type] {
			result.AddError(triggerPrefix+".type", "unknown trigger type", string(trigger.Type))
		}
	}
}

// validateReferences checks cross-references between template sections
func (v *Validator) validateReferences(t *models.LabTemplate, result *ValidationResult) {
	// Build VM name map
	vmNames := make(map[string]bool)
	for _, vm := range t.Spec.VMs {
		vmNames[vm.Name] = true
	}

	// Check objectives reference valid VMs (Target field contains VM name)
	for i, obj := range t.Spec.Objectives {
		for j, trigger := range obj.Triggers {
			if trigger.Target != "" && !vmNames[trigger.Target] {
				result.AddError(
					fmt.Sprintf("spec.objectives[%d].triggers[%d].target", i, j),
					"references undefined VM",
					trigger.Target,
				)
			}
		}
	}
}

// Helper functions

func isValidDuration(s string) bool {
	// Accept formats like "90m", "2h", "1h30m", "120"
	return reDuration.MatchString(s)
}

func isValidCIDR(s string) bool {
	return reCIDR.MatchString(s)
}

func isValidIP(s string) bool {
	return reIPAddress.MatchString(s)
}
