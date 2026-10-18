// Package models contains shared data types for the lab platform
package models

import "time"

// LTIAssignment represents a Canvas assignment linked to a lab template via LTI deep linking
type LTIAssignment struct {
	ID             string    `json:"id"`
	CanvasCourseID string    `json:"canvasCourseId"`
	ResourceLinkID string    `json:"resourceLinkId"` // LTI resource link ID from deep linking
	LabTemplateID  string    `json:"labTemplateId"`
	CustomTitle    string    `json:"customTitle,omitempty"`
	MaxPoints      float64   `json:"maxPoints"`
	OrganizationID *string   `json:"organizationId,omitempty"`
	CreatedBy      *string   `json:"createdBy,omitempty"`
	DeploymentID   string    `json:"deploymentId,omitempty"`
	LTIVersion     string    `json:"ltiVersion,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// LTITemplateInfo is a simplified template record for LTI selection UI
type LTITemplateInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Difficulty      string   `json:"difficulty"`
	DurationMinutes int      `json:"durationMinutes"`
	MaxPoints       int      `json:"maxPoints"`
	CheckpointCount int      `json:"checkpointCount"`
	Tags            []string `json:"tags"`
}
