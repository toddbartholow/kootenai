// Package orchestrator provides pod and VM lifecycle management
package orchestrator

import (
	"context"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// CreatePodOpts carries optional context for pod creation, such as
// multi-tenancy org/team assignment. Zero-value is safe and backwards
// compatible (no org or team assigned).
type CreatePodOpts struct {
	OrganizationID *string
	TeamID         *string
}

// Client defines the interface for orchestrating pod and VM lifecycle operations.
// This interface enables dependency injection and testing of components that
// depend on the orchestrator.
type Client interface {
	// Pod lifecycle
	CreatePod(ctx context.Context, template *models.LabTemplate, templateID, ownerID, ownerName string, opts ...CreatePodOpts) (*models.Pod, error)
	GetPod(ctx context.Context, podID string) (*models.Pod, error)
	ListPods(ctx context.Context, owner string) ([]*models.Pod, error)
	ListPodsWithFilter(ctx context.Context, filter PodFilter) ([]*models.Pod, error)
	DestroyPod(ctx context.Context, podID string) error

	// Pod power management
	StartPod(ctx context.Context, podID string) (*models.Pod, error)
	StopPod(ctx context.Context, podID string) (*models.Pod, error)

	// VM operations
	StartVM(ctx context.Context, podID, vmName string) error
	StopVM(ctx context.Context, podID, vmName string) error
	SuspendVM(ctx context.Context, podID, vmName string) error
	ResumeVM(ctx context.Context, podID, vmName string) error
	ResetPodVM(ctx context.Context, podID, vmName, snapshotName string) error

	// Snapshots
	CreateVMSnapshot(ctx context.Context, podID, vmName, snapshotName, description string, includeRAM bool) error
	DeleteVMSnapshot(ctx context.Context, podID, vmName, snapshotName string) error
	ListVMSnapshots(ctx context.Context, podID, vmName string) ([]VMSnapshot, error)

	// Console access
	GetVMConsole(ctx context.Context, podID, vmName, consoleType string) (*ConsoleTicket, error)

	// Command execution (via QEMU Guest Agent for active verification)
	ExecuteCommand(ctx context.Context, podID, vmName, command string) (string, error)
}

// Compile-time check that Orchestrator implements Client
var _ Client = (*Orchestrator)(nil)
