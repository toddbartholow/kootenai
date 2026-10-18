// Package models contains shared data types for the lab platform
package models

import "time"

// Pod represents an instantiated lab for a student/group
type Pod struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`          // Human-readable name (e.g., "cryptic-waddling-rain")
	LabTemplateID string            `json:"labTemplateId"` // UUID referencing lab_templates.id
	LabTemplate   string            `json:"labTemplate"`   // Human-readable template name (for display)
	Platform      Platform          `json:"platform"`
	OwnerID       string            `json:"ownerId"` // UUID referencing users.id
	Owner         string            `json:"owner"`   // Human-readable owner name (for display)
	Status        PodStatus         `json:"status"`
	VMs           []PodVM           `json:"vms"`
	Networks      []PodNetwork      `json:"networks"`
	CreatedAt     time.Time         `json:"createdAt"`
	ExpiresAt     *time.Time        `json:"expiresAt,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	// Multi-tenancy fields
	OrganizationID *string `json:"organizationId,omitempty"`
	TeamID         *string `json:"teamId,omitempty"`
}

// PodStatus represents the state of a pod
type PodStatus string

const (
	PodStatusProvisioning PodStatus = "provisioning"
	PodStatusRunning      PodStatus = "running"
	PodStatusStopped      PodStatus = "stopped"
	PodStatusError        PodStatus = "error"
	PodStatusDestroying   PodStatus = "destroying"
	PodStatusDestroyed    PodStatus = "destroyed"
)

// PodVM represents a VM instance in a pod
type PodVM struct {
	Name            string   `json:"name"`
	PlatformID      string   `json:"platformId"`
	Platform        Platform `json:"platform"`
	Node            string   `json:"node,omitempty"`
	Status          string   `json:"status"`
	IPAddress       string   `json:"ipAddress,omitempty"`
	CurrentSnapshot string   `json:"currentSnapshot,omitempty"`
}

// PodNetwork represents a network instance in a pod
type PodNetwork struct {
	Name       string `json:"name"`
	PlatformID string `json:"platformId"`
	VLAN       int    `json:"vlan"`
	Subnet     string `json:"subnet"`
	Gateway    string `json:"gateway,omitempty"`
	Netmask    string `json:"netmask,omitempty"`
	Type       string `json:"type,omitempty"`  // isolated, shared, l2
	State      string `json:"state,omitempty"` // for health checks
}
