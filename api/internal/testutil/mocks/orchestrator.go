// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
)

// Compile-time check that FakeOrchestrator implements orchestrator.Client
var _ orchestrator.Client = (*FakeOrchestrator)(nil)

// FakeOrchestrator is a mock implementation of the Orchestrator for testing
type FakeOrchestrator struct {
	mu   sync.RWMutex
	pods map[string]*models.Pod

	// Error injection
	CreatePodErr        error
	GetPodErr           error
	ListPodsErr         error
	DestroyPodErr       error
	StartPodErr         error
	StopPodErr          error
	StartVMErr          error
	StopVMErr           error
	SuspendVMErr        error
	ResumeVMErr         error
	ResetPodVMErr       error
	GetVMConsoleErr     error
	ListVMSnapshotsErr  error
	CreateVMSnapshotErr error
	DeleteVMSnapshotErr error
	ExecuteCommandErr   error

	// Call tracking
	CreatePodCalls        []CreatePodCall
	GetPodCalls           []string
	DestroyPodCalls       []string
	StartPodCalls         []string
	StopPodCalls          []string
	StartVMCalls          []VMActionCall
	StopVMCalls           []VMActionCall
	SuspendVMCalls        []VMActionCall
	ResumeVMCalls         []VMActionCall
	ResetPodVMCalls       []ResetVMCall
	GetVMConsoleCalls     []ConsoleCall
	ListVMSnapshotsCalls  []VMActionCall
	CreateVMSnapshotCalls []CreateSnapshotCall
	DeleteVMSnapshotCalls []DeleteSnapshotCall
	ExecuteCommandCalls   []ExecuteCommandCall

	// Return values for specific operations
	ConsoleTicket        *orchestrator.ConsoleTicket
	Snapshots            []orchestrator.VMSnapshot
	ExecuteCommandOutput string // Return value for ExecuteCommand
}

// CreatePodCall records a call to CreatePod
type CreatePodCall struct {
	Template   *models.LabTemplate
	TemplateID string
	OwnerID    string
	OwnerName  string
}

// VMActionCall records a VM action call
type VMActionCall struct {
	PodID  string
	VMName string
}

// ResetVMCall records a ResetPodVM call
type ResetVMCall struct {
	PodID        string
	VMName       string
	SnapshotName string
}

// ConsoleCall records a GetVMConsole call
type ConsoleCall struct {
	PodID       string
	VMName      string
	ConsoleType string
}

// CreateSnapshotCall records a CreateVMSnapshot call
type CreateSnapshotCall struct {
	PodID       string
	VMName      string
	Name        string
	Description string
	IncludeRAM  bool
}

// DeleteSnapshotCall records a DeleteVMSnapshot call
type DeleteSnapshotCall struct {
	PodID        string
	VMName       string
	SnapshotName string
}

// ExecuteCommandCall records an ExecuteCommand call
type ExecuteCommandCall struct {
	PodID   string
	VMName  string
	Command string
}

// NewFakeOrchestrator creates a new mock orchestrator
func NewFakeOrchestrator() *FakeOrchestrator {
	return &FakeOrchestrator{
		pods: make(map[string]*models.Pod),
	}
}

// CreatePod creates a new pod (mock implementation)
func (m *FakeOrchestrator) CreatePod(ctx context.Context, template *models.LabTemplate, templateID, ownerID, ownerName string, opts ...orchestrator.CreatePodOpts) (*models.Pod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreatePodCalls = append(m.CreatePodCalls, CreatePodCall{
		Template:   template,
		TemplateID: templateID,
		OwnerID:    ownerID,
		OwnerName:  ownerName,
	})

	if m.CreatePodErr != nil {
		return nil, m.CreatePodErr
	}

	pod := &models.Pod{
		ID:            fmt.Sprintf("pod-%s-%s", template.Metadata.Name, ownerName),
		LabTemplateID: templateID,
		LabTemplate:   template.Metadata.Name,
		Platform:      template.Spec.Platform,
		OwnerID:       ownerID,
		Owner:         ownerName,
		Status:        models.PodStatusRunning,
		VMs:           make([]models.PodVM, 0),
	}

	// Create mock VMs based on template
	for _, vmSpec := range template.Spec.VMs {
		pod.VMs = append(pod.VMs, models.PodVM{
			Name:       vmSpec.Name,
			PlatformID: fmt.Sprintf("%d", 100+len(pod.VMs)),
			Platform:   template.Spec.Platform,
			Node:       "pve",
			Status:     "running",
		})
	}

	m.pods[pod.ID] = pod
	return pod, nil
}

// GetPod returns a pod by ID
func (m *FakeOrchestrator) GetPod(ctx context.Context, podID string) (*models.Pod, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetPodCalls = append(m.GetPodCalls, podID)

	if m.GetPodErr != nil {
		return nil, m.GetPodErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return nil, fmt.Errorf("pod not found: %s", podID)
	}

	return pod, nil
}

// ListPods returns all pods filtered by owner
func (m *FakeOrchestrator) ListPods(ctx context.Context, owner string) ([]*models.Pod, error) {
	return m.ListPodsWithFilter(ctx, orchestrator.PodFilter{OwnerID: owner})
}

// ListPodsWithFilter returns pods matching the filter
func (m *FakeOrchestrator) ListPodsWithFilter(ctx context.Context, filter orchestrator.PodFilter) ([]*models.Pod, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ListPodsErr != nil {
		return nil, m.ListPodsErr
	}

	result := make([]*models.Pod, 0)
	for _, pod := range m.pods {
		if filter.OwnerID != "" && pod.Owner != filter.OwnerID && pod.OwnerID != filter.OwnerID {
			continue
		}
		if filter.Platform != "" && string(pod.Platform) != filter.Platform {
			continue
		}
		if filter.Status != "" && pod.Status != filter.Status {
			continue
		}
		result = append(result, pod)
	}

	return result, nil
}

// DestroyPod destroys a pod
func (m *FakeOrchestrator) DestroyPod(ctx context.Context, podID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DestroyPodCalls = append(m.DestroyPodCalls, podID)

	if m.DestroyPodErr != nil {
		return m.DestroyPodErr
	}

	if _, ok := m.pods[podID]; !ok {
		return fmt.Errorf("pod not found: %s", podID)
	}

	delete(m.pods, podID)
	return nil
}

// StartPod starts all VMs in a pod
func (m *FakeOrchestrator) StartPod(ctx context.Context, podID string) (*models.Pod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartPodCalls = append(m.StartPodCalls, podID)

	if m.StartPodErr != nil {
		return nil, m.StartPodErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return nil, fmt.Errorf("pod not found: %s", podID)
	}

	pod.Status = models.PodStatusRunning
	for i := range pod.VMs {
		pod.VMs[i].Status = "running"
	}

	return pod, nil
}

// StopPod stops all VMs in a pod
func (m *FakeOrchestrator) StopPod(ctx context.Context, podID string) (*models.Pod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StopPodCalls = append(m.StopPodCalls, podID)

	if m.StopPodErr != nil {
		return nil, m.StopPodErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return nil, fmt.Errorf("pod not found: %s", podID)
	}

	pod.Status = models.PodStatusStopped
	for i := range pod.VMs {
		pod.VMs[i].Status = "stopped"
	}

	return pod, nil
}

// StartVM starts a single VM in a pod
func (m *FakeOrchestrator) StartVM(ctx context.Context, podID, vmName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartVMCalls = append(m.StartVMCalls, VMActionCall{PodID: podID, VMName: vmName})

	if m.StartVMErr != nil {
		return m.StartVMErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return fmt.Errorf("pod not found: %s", podID)
	}

	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			pod.VMs[i].Status = "running"
			return nil
		}
	}

	return fmt.Errorf("VM not found: %s", vmName)
}

// StopVM stops a single VM in a pod
func (m *FakeOrchestrator) StopVM(ctx context.Context, podID, vmName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StopVMCalls = append(m.StopVMCalls, VMActionCall{PodID: podID, VMName: vmName})

	if m.StopVMErr != nil {
		return m.StopVMErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return fmt.Errorf("pod not found: %s", podID)
	}

	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			pod.VMs[i].Status = "stopped"
			return nil
		}
	}

	return fmt.Errorf("VM not found: %s", vmName)
}

// SuspendVM suspends a VM
func (m *FakeOrchestrator) SuspendVM(ctx context.Context, podID, vmName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SuspendVMCalls = append(m.SuspendVMCalls, VMActionCall{PodID: podID, VMName: vmName})

	if m.SuspendVMErr != nil {
		return m.SuspendVMErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return fmt.Errorf("pod not found: %s", podID)
	}

	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			pod.VMs[i].Status = "suspended"
			return nil
		}
	}

	return fmt.Errorf("VM not found: %s", vmName)
}

// ResumeVM resumes a suspended VM
func (m *FakeOrchestrator) ResumeVM(ctx context.Context, podID, vmName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ResumeVMCalls = append(m.ResumeVMCalls, VMActionCall{PodID: podID, VMName: vmName})

	if m.ResumeVMErr != nil {
		return m.ResumeVMErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return fmt.Errorf("pod not found: %s", podID)
	}

	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			pod.VMs[i].Status = "running"
			return nil
		}
	}

	return fmt.Errorf("VM not found: %s", vmName)
}

// ResetPodVM resets a VM to a snapshot
func (m *FakeOrchestrator) ResetPodVM(ctx context.Context, podID, vmName, snapshotName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ResetPodVMCalls = append(m.ResetPodVMCalls, ResetVMCall{
		PodID:        podID,
		VMName:       vmName,
		SnapshotName: snapshotName,
	})

	if m.ResetPodVMErr != nil {
		return m.ResetPodVMErr
	}

	pod, ok := m.pods[podID]
	if !ok {
		return fmt.Errorf("pod not found: %s", podID)
	}

	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			pod.VMs[i].CurrentSnapshot = snapshotName
			return nil
		}
	}

	return fmt.Errorf("VM not found: %s", vmName)
}

// GetVMConsole returns console access info for a VM
func (m *FakeOrchestrator) GetVMConsole(ctx context.Context, podID, vmName, consoleType string) (*orchestrator.ConsoleTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetVMConsoleCalls = append(m.GetVMConsoleCalls, ConsoleCall{
		PodID:       podID,
		VMName:      vmName,
		ConsoleType: consoleType,
	})

	if m.GetVMConsoleErr != nil {
		return nil, m.GetVMConsoleErr
	}

	if m.ConsoleTicket != nil {
		return m.ConsoleTicket, nil
	}

	// Return default ticket
	return &orchestrator.ConsoleTicket{
		Type:   "vnc",
		Host:   "localhost",
		Port:   5900,
		Ticket: "mock-ticket",
		Node:   "pve",
		VMID:   "100",
	}, nil
}

// ListVMSnapshots lists snapshots for a VM
func (m *FakeOrchestrator) ListVMSnapshots(ctx context.Context, podID, vmName string) ([]orchestrator.VMSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ListVMSnapshotsCalls = append(m.ListVMSnapshotsCalls, VMActionCall{PodID: podID, VMName: vmName})

	if m.ListVMSnapshotsErr != nil {
		return nil, m.ListVMSnapshotsErr
	}

	if m.Snapshots != nil {
		return m.Snapshots, nil
	}

	// Return default snapshots
	return []orchestrator.VMSnapshot{
		{Name: "baseline", Description: "Initial state", CreatedAt: time.Now()},
		{Name: "checkpoint1", Description: "First checkpoint", CreatedAt: time.Now()},
	}, nil
}

// CreateVMSnapshot creates a snapshot for a VM
func (m *FakeOrchestrator) CreateVMSnapshot(ctx context.Context, podID, vmName, name, description string, includeRAM bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateVMSnapshotCalls = append(m.CreateVMSnapshotCalls, CreateSnapshotCall{
		PodID:       podID,
		VMName:      vmName,
		Name:        name,
		Description: description,
		IncludeRAM:  includeRAM,
	})

	return m.CreateVMSnapshotErr
}

// DeleteVMSnapshot deletes a snapshot from a VM
func (m *FakeOrchestrator) DeleteVMSnapshot(ctx context.Context, podID, vmName, snapshotName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteVMSnapshotCalls = append(m.DeleteVMSnapshotCalls, DeleteSnapshotCall{
		PodID:        podID,
		VMName:       vmName,
		SnapshotName: snapshotName,
	})

	return m.DeleteVMSnapshotErr
}

// ExecuteCommand executes a command on a VM (mock implementation)
func (m *FakeOrchestrator) ExecuteCommand(ctx context.Context, podID, vmName, command string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ExecuteCommandCalls = append(m.ExecuteCommandCalls, ExecuteCommandCall{
		PodID:   podID,
		VMName:  vmName,
		Command: command,
	})

	if m.ExecuteCommandErr != nil {
		return "", m.ExecuteCommandErr
	}

	// Check that pod exists
	pod, ok := m.pods[podID]
	if !ok {
		return "", fmt.Errorf("pod not found: %s", podID)
	}

	// Check that VM exists in pod
	found := false
	for _, vm := range pod.VMs {
		if vm.Name == vmName {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("VM not found: %s", vmName)
	}

	// Return configured output or default
	if m.ExecuteCommandOutput != "" {
		return m.ExecuteCommandOutput, nil
	}

	return "", nil
}

// AddPod adds a pod directly to the mock (for test setup)
func (m *FakeOrchestrator) AddPod(pod *models.Pod) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pods[pod.ID] = pod
}
