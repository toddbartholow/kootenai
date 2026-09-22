// Package templates provides functionality for loading lab templates from disk
package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// InheritanceResolver handles template inheritance
type InheritanceResolver struct {
	repo   repositories.LabTemplateRepository
	cache  map[string]*models.LabTemplate
	loaded map[string]bool // Track loading to detect cycles
}

// NewInheritanceResolver creates a new resolver
func NewInheritanceResolver(repo repositories.LabTemplateRepository) *InheritanceResolver {
	return &InheritanceResolver{
		repo:   repo,
		cache:  make(map[string]*models.LabTemplate),
		loaded: make(map[string]bool),
	}
}

// Resolve resolves template inheritance and returns the merged template
func (r *InheritanceResolver) Resolve(ctx context.Context, template *models.LabTemplate) (*models.LabTemplate, error) {
	// Check for inheritance
	if template.Metadata.Extends == "" {
		return template, nil
	}

	// Detect circular inheritance
	templateName := template.Metadata.Name
	if r.loaded[templateName] {
		return nil, fmt.Errorf("circular inheritance detected: %s", templateName)
	}
	r.loaded[templateName] = true
	defer func() { delete(r.loaded, templateName) }()

	// Load parent template
	parent, err := r.loadParent(ctx, template.Metadata.Extends)
	if err != nil {
		return nil, fmt.Errorf("loading parent template %s: %w", template.Metadata.Extends, err)
	}

	// Recursively resolve parent
	resolvedParent, err := r.Resolve(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Merge child onto parent
	merged := r.merge(resolvedParent, template)
	return merged, nil
}

// loadParent loads a parent template by name
func (r *InheritanceResolver) loadParent(ctx context.Context, name string) (*models.LabTemplate, error) {
	// Check cache
	if cached, ok := r.cache[name]; ok {
		return cached, nil
	}

	// Load from database
	record, err := r.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	// Parse spec back to LabTemplate
	template := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        record.Name,
			Description: record.Description,
			Version:     record.Version,
			Difficulty:  record.Difficulty,
		},
	}

	// Unmarshal spec
	if len(record.Spec) > 0 {
		if err := json.Unmarshal(record.Spec, &template.Spec); err != nil {
			return nil, fmt.Errorf("unmarshaling spec: %w", err)
		}
	}

	r.cache[name] = template
	return template, nil
}

// merge combines parent and child templates, with child values taking precedence
func (r *InheritanceResolver) merge(parent, child *models.LabTemplate) *models.LabTemplate {
	result := &models.LabTemplate{
		APIVersion: child.APIVersion,
		Kind:       child.Kind,
	}

	// Merge metadata (child overrides parent)
	result.Metadata = models.LabMetadata{
		Name:        child.Metadata.Name,
		Description: coalesce(child.Metadata.Description, parent.Metadata.Description),
		Duration:    coalesce(child.Metadata.Duration, parent.Metadata.Duration),
		Difficulty:  coalesce(child.Metadata.Difficulty, parent.Metadata.Difficulty),
		Version:     coalesce(child.Metadata.Version, parent.Metadata.Version),
		Author:      coalesce(child.Metadata.Author, parent.Metadata.Author),
		Tags:        mergeTags(parent.Metadata.Tags, child.Metadata.Tags),
	}

	// Merge spec
	result.Spec = models.LabSpec{
		Platform: coalesceP(child.Spec.Platform, parent.Spec.Platform),
	}

	// Merge VMs (child VMs override parent VMs by name, or add new ones)
	result.Spec.VMs = mergeVMs(parent.Spec.VMs, child.Spec.VMs)

	// Merge network
	result.Spec.Network = mergeNetwork(parent.Spec.Network, child.Spec.Network)

	// Merge checkpoints config
	if child.Spec.Checkpoints != nil {
		result.Spec.Checkpoints = child.Spec.Checkpoints
	} else if parent.Spec.Checkpoints != nil {
		result.Spec.Checkpoints = parent.Spec.Checkpoints
	}

	// Merge objectives (append child objectives to parent)
	result.Spec.Objectives = append(parent.Spec.Objectives, child.Spec.Objectives...)

	return result
}

// mergeVMs combines VM lists, with child VMs overriding parent VMs by name
func mergeVMs(parent, child []models.VMSpec) []models.VMSpec {
	vmMap := make(map[string]models.VMSpec)

	// Add parent VMs
	for _, vm := range parent {
		vmMap[vm.Name] = vm
	}

	// Override/add child VMs
	for _, vm := range child {
		vmMap[vm.Name] = vm
	}

	// Convert back to slice
	result := make([]models.VMSpec, 0, len(vmMap))
	for _, vm := range vmMap {
		result = append(result, vm)
	}
	return result
}

// mergeNetwork combines network configs
func mergeNetwork(parent, child models.NetworkSpec) models.NetworkSpec {
	result := parent

	// If child has segments, use them; otherwise keep parent segments
	if len(child.Segments) > 0 {
		// Merge segments by name
		segmentMap := make(map[string]models.NetworkSegment)
		for _, seg := range parent.Segments {
			segmentMap[seg.Name] = seg
		}
		for _, seg := range child.Segments {
			segmentMap[seg.Name] = seg
		}
		result.Segments = make([]models.NetworkSegment, 0, len(segmentMap))
		for _, seg := range segmentMap {
			result.Segments = append(result.Segments, seg)
		}
	}

	return result
}

// mergeTags combines tag slices, removing duplicates
func mergeTags(parent, child []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, tag := range parent {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	for _, tag := range child {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

// coalesce returns the first non-empty string
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// coalesceP returns the first non-empty Platform
func coalesceP(values ...models.Platform) models.Platform {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// VariableProcessor handles variable substitution in templates
type VariableProcessor struct {
	variables map[string]string
}

// NewVariableProcessor creates a new processor with the given variables
func NewVariableProcessor(vars map[string]string) *VariableProcessor {
	return &VariableProcessor{
		variables: vars,
	}
}

// Process substitutes variables in template strings
// Variables use the format ${VAR_NAME} or $VAR_NAME
func (p *VariableProcessor) Process(template *models.LabTemplate) *models.LabTemplate {
	// Deep copy and process
	result := *template

	// Process metadata
	result.Metadata.Name = p.substitute(result.Metadata.Name)
	result.Metadata.Description = p.substitute(result.Metadata.Description)

	// Process VMs
	for i := range result.Spec.VMs {
		result.Spec.VMs[i].Name = p.substitute(result.Spec.VMs[i].Name)
		result.Spec.VMs[i].Template = p.substitute(result.Spec.VMs[i].Template)
	}

	// Process network segments
	for i := range result.Spec.Network.Segments {
		result.Spec.Network.Segments[i].Subnet = p.substitute(result.Spec.Network.Segments[i].Subnet)
		result.Spec.Network.Segments[i].Gateway = p.substitute(result.Spec.Network.Segments[i].Gateway)
	}

	// Process objectives
	for i := range result.Spec.Objectives {
		result.Spec.Objectives[i].ID = p.substitute(result.Spec.Objectives[i].ID)
		result.Spec.Objectives[i].Description = p.substitute(result.Spec.Objectives[i].Description)
		result.Spec.Objectives[i].Hint = p.substitute(result.Spec.Objectives[i].Hint)
		// Process triggers
		for j := range result.Spec.Objectives[i].Triggers {
			result.Spec.Objectives[i].Triggers[j].Target = p.substitute(result.Spec.Objectives[i].Triggers[j].Target)
		}
	}

	return &result
}

// substitute replaces variables in a string
func (p *VariableProcessor) substitute(s string) string {
	if s == "" || !strings.Contains(s, "$") {
		return s
	}

	// Match ${VAR_NAME} or $VAR_NAME
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name
		var varName string
		if strings.HasPrefix(match, "${") {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}

		// Look up variable
		if val, ok := p.variables[varName]; ok {
			return val
		}
		// Return original if not found
		return match
	})
}

// DefaultVariables returns common default variables
func DefaultVariables() map[string]string {
	return map[string]string{
		"LAB_SUBNET":  "10.0.0.0/24",
		"LAB_GATEWAY": "10.0.0.1",
		"LAB_DNS":     "10.0.0.1",
	}
}
