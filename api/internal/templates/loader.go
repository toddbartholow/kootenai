// Package templates provides functionality for loading lab templates from disk
package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/templates/validator"
)

// Loader handles loading lab templates from disk and storing them in the database
type Loader struct {
	repo       *repositories.LabTemplateRepo
	logger     *slog.Logger
	validator  *validator.Validator
	resolver   *InheritanceResolver
	strictMode bool // If true, validation errors prevent loading
}

// LoaderOption configures the loader
type LoaderOption func(*Loader)

// WithStrictMode enables strict validation (errors prevent loading)
func WithStrictMode() LoaderOption {
	return func(l *Loader) {
		l.strictMode = true
	}
}

// WithValidator sets a custom validator
func WithValidator(v *validator.Validator) LoaderOption {
	return func(l *Loader) {
		l.validator = v
	}
}

// NewLoader creates a new template loader
func NewLoader(repo *repositories.LabTemplateRepo, logger *slog.Logger, opts ...LoaderOption) *Loader {
	l := &Loader{
		repo:      repo,
		logger:    logger,
		validator: validator.NewValidator(),
		resolver:  NewInheritanceResolver(repo),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// LoadFromDirectory loads all YAML template files from a directory recursively
func (l *Loader) LoadFromDirectory(ctx context.Context, dir string) error {
	l.logger.Info("Loading templates from directory", "dir", dir)

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Warn("Templates directory does not exist", "dir", dir)
			return nil
		}
		return fmt.Errorf("checking templates directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("templates path is not a directory: %s", dir)
	}

	// Walk directory and find YAML files
	var loaded, errors int
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			l.logger.Error("Error walking directory", "path", path, "error", err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Load and store the template
		if err := l.loadFile(ctx, path); err != nil {
			l.logger.Error("Failed to load template", "path", path, "error", err)
			errors++
		} else {
			loaded++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking templates directory: %w", err)
	}

	l.logger.Info("Template loading complete", "loaded", loaded, "errors", errors)
	return nil
}

// loadFile loads a single template file and stores it in the database
func (l *Loader) loadFile(ctx context.Context, path string) error {
	// Read file
	// #nosec G304 -- Path comes from walking the configured template directory,
	// which is an admin-controlled server configuration setting.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse YAML
	var template models.LabTemplate
	if err := yaml.Unmarshal(data, &template); err != nil {
		return fmt.Errorf("parsing YAML at %s: %w", path, err)
	}

	// Validate it's a LabTemplate
	if template.Kind != "LabTemplate" {
		l.logger.Debug("Skipping non-template file", "path", path, "kind", template.Kind)
		return nil
	}

	// Validate template schema
	validationResult := l.validator.Validate(&template)
	if !validationResult.Valid {
		if l.strictMode {
			return fmt.Errorf("validation failed for %s: %s", path, validationResult.Error())
		}
		// Log warnings but continue
		for _, verr := range validationResult.Errors {
			l.logger.Warn("Template validation warning",
				"path", path,
				"field", verr.Field,
				"message", verr.Message,
			)
		}
	}

	// Process variable substitution
	processedTemplate := l.processVariables(&template)

	// Resolve template inheritance
	resolvedTemplate, err := l.resolver.Resolve(ctx, processedTemplate)
	if err != nil {
		return fmt.Errorf("resolving inheritance for %s: %w", path, err)
	}

	// Convert to database record
	record, err := l.templateToRecord(resolvedTemplate)
	if err != nil {
		return fmt.Errorf("converting template: %w", err)
	}

	// Upsert into database
	if err := l.repo.Upsert(ctx, record); err != nil {
		return fmt.Errorf("upserting template: %w", err)
	}

	l.logger.Info("Loaded template",
		"name", record.Name,
		"version", record.Version,
		"platform", record.Platform,
		"extends", template.Metadata.Extends,
	)

	return nil
}

// processVariables applies variable substitution to a template
func (l *Loader) processVariables(template *models.LabTemplate) *models.LabTemplate {
	// Merge default variables with template-specific variables
	vars := DefaultVariables()
	for k, v := range template.Metadata.Variables {
		vars[k] = v
	}

	// If no variables defined, return as-is
	if len(vars) == 0 {
		return template
	}

	processor := NewVariableProcessor(vars)
	return processor.Process(template)
}

// templateToRecord converts a LabTemplate to a LabTemplateRecord
func (l *Loader) templateToRecord(t *models.LabTemplate) (*models.LabTemplateRecord, error) {
	// Marshal spec to JSON
	specJSON, err := json.Marshal(t.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling spec: %w", err)
	}

	// Extract checkpoints from spec objectives
	checkpointsJSON, err := json.Marshal(t.Spec.Objectives)
	if err != nil {
		return nil, fmt.Errorf("marshaling checkpoints: %w", err)
	}

	// Calculate max points from objectives
	var maxPoints int
	for _, obj := range t.Spec.Objectives {
		maxPoints += obj.Points
	}

	// Get pass threshold from checkpoints config
	passThreshold := 70 // default
	if t.Spec.Checkpoints != nil && t.Spec.Checkpoints.PassThreshold > 0 {
		passThreshold = t.Spec.Checkpoints.PassThreshold
	}

	// Parse duration
	durationMinutes := parseDuration(t.Metadata.Duration)

	// Determine platform
	platform := t.Spec.Platform
	if platform == "" {
		platform = models.PlatformAny
	}

	// Generate slug from name: lowercase, replace spaces with hyphens
	slug := generateSlug(t.Metadata.Name)

	// Marshal instructions to JSON if present
	var instructionsJSON json.RawMessage
	if t.Metadata.Instructions != nil {
		instrBytes, err := json.Marshal(t.Metadata.Instructions)
		if err != nil {
			return nil, fmt.Errorf("marshaling instructions: %w", err)
		}
		instructionsJSON = instrBytes
	}

	return &models.LabTemplateRecord{
		Name:            t.Metadata.Name,
		Slug:            slug,
		Description:     t.Metadata.Description,
		Version:         t.Metadata.Version,
		Platform:        platform,
		DurationMinutes: durationMinutes,
		Difficulty:      t.Metadata.Difficulty,
		MaxPoints:       maxPoints,
		PassThreshold:   passThreshold,
		Spec:            specJSON,
		Checkpoints:     checkpointsJSON,
		Instructions:    instructionsJSON,
		IsActive:        true,
	}, nil
}

// generateSlug creates a URL-friendly slug from a name
func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// parseDuration parses a duration string like "90m" or "2h" to minutes
func parseDuration(s string) int {
	if s == "" {
		return 0
	}

	// Try Go duration parsing
	if d, err := time.ParseDuration(s); err == nil {
		return int(d.Minutes())
	}

	// Try simple number parsing (assume minutes)
	s = strings.TrimSpace(s)
	if num, err := strconv.Atoi(s); err == nil {
		return num
	}

	return 0
}
