// Package orchestrator provides unified lab management across platforms
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/util/namegen"
)

// PodProvisioningEvent represents a status update during pod provisioning
type PodProvisioningEvent struct {
	PodID     string            `json:"podId"`
	OwnerID   string            `json:"ownerId"`
	Status    models.PodStatus  `json:"status"`
	Phase     ProvisioningPhase `json:"phase"`
	Message   string            `json:"message"`
	Progress  int               `json:"progress"` // 0-100
	VMName    string            `json:"vmName,omitempty"`
	VMStatus  string            `json:"vmStatus,omitempty"`
	Error     string            `json:"error,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	RequestID string            `json:"requestId"`
}

// ProvisioningPhase represents the current phase of pod provisioning
type ProvisioningPhase string

const (
	PhaseQueued            ProvisioningPhase = "queued"
	PhaseStarting          ProvisioningPhase = "starting"
	PhaseCreatingPod       ProvisioningPhase = "creating_pod"
	PhaseCloningVM         ProvisioningPhase = "cloning_vm"
	PhaseConfiguringVM     ProvisioningPhase = "configuring_vm"
	PhaseCreatingSnapshots ProvisioningPhase = "creating_snapshots"
	PhaseStartingVM        ProvisioningPhase = "starting_vm"
	PhaseCompleted         ProvisioningPhase = "completed"
	PhaseFailed            ProvisioningPhase = "failed"
)

// PodProvisionRequest represents a request to provision a new pod asynchronously
type PodProvisionRequest struct {
	RequestID      string              `json:"requestId"`
	TemplateID     string              `json:"templateId"`
	TemplateName   string              `json:"templateName"`
	Template       *models.LabTemplate `json:"template"`
	OwnerID        string              `json:"ownerId"`
	OwnerName      string              `json:"ownerName"`
	Timestamp      time.Time           `json:"timestamp"`
	OrganizationID *string             `json:"organizationId,omitempty"`
	TeamID         *string             `json:"teamId,omitempty"`
}

// EventPublisher defines the interface for publishing provisioning events
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// ProvisioningBroadcaster defines the interface for broadcasting provisioning events via WebSocket
type ProvisioningBroadcaster interface {
	BroadcastProvisioningEvent(event PodProvisioningEvent)
}

// AsyncProvisioner handles asynchronous pod provisioning with status updates
type AsyncProvisioner struct {
	orchestrator  *Orchestrator
	publisher     EventPublisher
	broadcaster   ProvisioningBroadcaster
	logger        *slog.Logger
	subjectPrefix string
	wg            sync.WaitGroup // tracks in-flight provisioning goroutines
	sem           chan struct{}  // bounds concurrent provisioning
	ctx           context.Context
	cancel        context.CancelFunc
}

// AsyncProvisionerOption is a functional option for configuring AsyncProvisioner
type AsyncProvisionerOption func(*AsyncProvisioner)

// WithSubjectPrefix sets the NATS subject prefix for events
func WithSubjectPrefix(prefix string) AsyncProvisionerOption {
	return func(ap *AsyncProvisioner) {
		ap.subjectPrefix = prefix
	}
}

// WithBroadcaster sets the WebSocket broadcaster for provisioning events
func WithBroadcaster(broadcaster ProvisioningBroadcaster) AsyncProvisionerOption {
	return func(ap *AsyncProvisioner) {
		ap.broadcaster = broadcaster
	}
}

// NewAsyncProvisioner creates a new async provisioner
func NewAsyncProvisioner(orch *Orchestrator, publisher EventPublisher, logger *slog.Logger, opts ...AsyncProvisionerOption) *AsyncProvisioner {
	ctx, cancel := context.WithCancel(context.Background())
	ap := &AsyncProvisioner{
		orchestrator:  orch,
		publisher:     publisher,
		logger:        logger,
		subjectPrefix: "labs.pods",
		sem:           make(chan struct{}, 10), // max 10 concurrent provisions
		ctx:           ctx,
		cancel:        cancel,
	}

	for _, opt := range opts {
		opt(ap)
	}

	return ap
}

// Shutdown cancels in-flight provisioning and waits for goroutines to finish.
func (ap *AsyncProvisioner) Shutdown(ctx context.Context) error {
	ap.cancel()
	done := make(chan struct{})
	go func() {
		ap.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out with in-flight provisions")
	}
}

// ProvisionAsync starts asynchronous pod provisioning and returns immediately
// Status updates are published to NATS on subject: labs.pods.{podId}.provision
func (ap *AsyncProvisioner) ProvisionAsync(ctx context.Context, req PodProvisionRequest) (string, error) {
	// Generate request ID if not provided
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	req.Timestamp = time.Now()

	// Create initial pod record in database with provisioning status
	podID := uuid.New().String()
	podName := namegen.Generate()
	pod := &models.Pod{
		ID:             podID,
		Name:           podName,
		LabTemplateID:  req.TemplateID,
		LabTemplate:    req.TemplateName,
		Platform:       req.Template.Spec.Platform,
		OwnerID:        req.OwnerID,
		Owner:          req.OwnerName,
		Status:         models.PodStatusProvisioning,
		VMs:            make([]models.PodVM, 0, len(req.Template.Spec.VMs)),
		Networks:       make([]models.PodNetwork, 0, len(req.Template.Spec.Network.Segments)),
		CreatedAt:      time.Now(),
		OrganizationID: req.OrganizationID,
		TeamID:         req.TeamID,
	}

	if ap.orchestrator.config.DefaultPodDuration > 0 {
		exp := time.Now().Add(ap.orchestrator.config.DefaultPodDuration)
		pod.ExpiresAt = &exp
	}

	// Persist initial pod record
	if ap.orchestrator.hasPersistence() {
		if err := ap.orchestrator.podRepo.Create(ctx, pod); err != nil {
			return "", fmt.Errorf("persisting pod to database: %w", err)
		}
	} else {
		ap.orchestrator.podsMu.Lock()
		ap.orchestrator.pods[podID] = pod
		ap.orchestrator.podsMu.Unlock()
	}

	// Publish queued event
	ap.publishEvent(ctx, PodProvisioningEvent{
		PodID:     podID,
		OwnerID:   req.OwnerID,
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseQueued,
		Message:   "Pod provisioning request queued",
		Progress:  0,
		Timestamp: time.Now(),
		RequestID: req.RequestID,
	})

	// Start provisioning in background goroutine with lifecycle tracking
	ap.wg.Add(1)
	go func() {
		defer ap.wg.Done()
		select {
		case ap.sem <- struct{}{}:
			defer func() { <-ap.sem }()
		case <-ap.ctx.Done():
			ap.logger.Warn("Provisioning cancelled before start", "podId", podID)
			return
		}
		ap.provisionPod(ap.ctx, podID, req)
	}()

	return podID, nil
}

// provisionPod performs the actual pod provisioning with status updates
func (ap *AsyncProvisioner) provisionPod(ctx context.Context, podID string, req PodProvisionRequest) {
	logger := ap.logger.With("podId", podID, "requestId", req.RequestID)

	template := req.Template
	platform := template.Spec.Platform
	if platform == models.PlatformAny {
		platform = ap.orchestrator.config.DefaultPlatform
	}

	// Publish starting event
	ap.publishEvent(ctx, PodProvisioningEvent{
		PodID:     podID,
		OwnerID:   req.OwnerID,
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseStarting,
		Message:   "Starting pod provisioning",
		Progress:  5,
		Timestamp: time.Now(),
		RequestID: req.RequestID,
	})

	// Get the pod to update
	pod, err := ap.orchestrator.GetPod(ctx, podID)
	if err != nil {
		ap.handleProvisioningError(ctx, podID, req, err, "Failed to get pod")
		return
	}

	// Platform-specific provisioning
	provisionStart := time.Now()
	var provisionErr error
	switch platform {
	case models.PlatformProxmox:
		provisionErr = ap.provisionProxmoxPodWithUpdates(ctx, pod, template, req)
	case models.PlatformCloudStack:
		provisionErr = ap.provisionCloudStackPodWithUpdates(ctx, pod, template, req)
	default:
		provisionErr = fmt.Errorf("unsupported platform: %s", platform)
	}
	if ap.orchestrator.metrics != nil {
		ap.orchestrator.metrics.VMOperationDuration("provision", time.Since(provisionStart))
	}

	if provisionErr != nil {
		ap.handleProvisioningError(ctx, podID, req, provisionErr, "Provisioning failed")
		return
	}

	// Update pod status to running. The pod is live from here on, so this is
	// where the async path counts a creation -- ProvisionAsync returns 202
	// before any VM exists, and counting there would score failed provisions
	// as successes.
	if ap.orchestrator.metrics != nil {
		ap.orchestrator.metrics.PodCreated()
	}
	pod.Status = models.PodStatusRunning
	if ap.orchestrator.hasPersistence() {
		if err := ap.orchestrator.podRepo.Update(ctx, pod); err != nil {
			logger.Error("Failed to update pod in database after provisioning", "error", err)
		}
	} else {
		ap.orchestrator.podsMu.Lock()
		ap.orchestrator.pods[podID] = pod
		ap.orchestrator.podsMu.Unlock()
	}

	// Publish completion event
	ap.publishEvent(ctx, PodProvisioningEvent{
		PodID:     podID,
		OwnerID:   req.OwnerID,
		Status:    models.PodStatusRunning,
		Phase:     PhaseCompleted,
		Message:   "Pod provisioning completed successfully",
		Progress:  100,
		Timestamp: time.Now(),
		RequestID: req.RequestID,
	})

	logger.Info("Pod provisioning completed successfully")
}

// provisionProxmoxPodWithUpdates provisions a Proxmox pod with progress updates
func (ap *AsyncProvisioner) provisionProxmoxPodWithUpdates(ctx context.Context, pod *models.Pod, template *models.LabTemplate, req PodProvisionRequest) error {
	node := ap.orchestrator.config.ProxmoxNode
	totalVMs := len(template.Spec.VMs)

	for i, vmSpec := range template.Spec.VMs {
		baseProgress := 10 + (i * 80 / totalVMs)

		// Publish cloning event
		ap.publishEvent(ctx, PodProvisioningEvent{
			PodID:     pod.ID,
			OwnerID:   req.OwnerID,
			Status:    models.PodStatusProvisioning,
			Phase:     PhaseCloningVM,
			Message:   fmt.Sprintf("Cloning VM: %s (%d/%d)", vmSpec.Name, i+1, totalVMs),
			Progress:  baseProgress,
			VMName:    vmSpec.Name,
			VMStatus:  "cloning",
			Timestamp: time.Now(),
			RequestID: req.RequestID,
		})

		templateVMID, err := ap.orchestrator.resolveTemplateVMID(vmSpec.Template)
		if err != nil {
			return fmt.Errorf("resolving template %q: %w", vmSpec.Template, err)
		}
		newVMID, err := generateVMID()
		if err != nil {
			return fmt.Errorf("generating VMID for %s: %w", vmSpec.Name, err)
		}

		vmName := fmt.Sprintf("%s-%s", pod.Name, vmSpec.Name)
		if err := ap.orchestrator.proxmox.CloneVM(ctx, node, templateVMID, newVMID, vmName, true); err != nil {
			return fmt.Errorf("cloning VM %s: %w", vmSpec.Name, err)
		}

		// Create snapshots
		if len(vmSpec.Snapshots) > 0 {
			ap.publishEvent(ctx, PodProvisioningEvent{
				PodID:     pod.ID,
				OwnerID:   req.OwnerID,
				Status:    models.PodStatusProvisioning,
				Phase:     PhaseCreatingSnapshots,
				Message:   fmt.Sprintf("Creating snapshots for VM: %s", vmSpec.Name),
				Progress:  baseProgress + 15,
				VMName:    vmSpec.Name,
				VMStatus:  "snapshotting",
				Timestamp: time.Now(),
				RequestID: req.RequestID,
			})

			for _, snap := range vmSpec.Snapshots {
				err := ap.orchestrator.proxmox.CreateSnapshot(ctx, node, newVMID, snap.Name, snap.Description, snap.IncludeRAM)
				if err != nil {
					return fmt.Errorf("creating snapshot %s: %w", snap.Name, err)
				}
			}
		}

		// Start VM if configured
		if vmSpec.StartOnCreate {
			ap.publishEvent(ctx, PodProvisioningEvent{
				PodID:     pod.ID,
				OwnerID:   req.OwnerID,
				Status:    models.PodStatusProvisioning,
				Phase:     PhaseStartingVM,
				Message:   fmt.Sprintf("Starting VM: %s", vmSpec.Name),
				Progress:  baseProgress + 25,
				VMName:    vmSpec.Name,
				VMStatus:  "starting",
				Timestamp: time.Now(),
				RequestID: req.RequestID,
			})

			if err := ap.orchestrator.proxmox.StartVM(ctx, node, newVMID); err != nil {
				return fmt.Errorf("starting VM %s: %w", vmSpec.Name, err)
			}
		}

		// Add VM to pod
		pod.VMs = append(pod.VMs, models.PodVM{
			Name:       vmSpec.Name,
			PlatformID: fmt.Sprintf("%d", newVMID),
			Platform:   models.PlatformProxmox,
			Node:       node,
			Status:     "created",
		})
	}

	return nil
}

// provisionCloudStackPodWithUpdates provisions a CloudStack pod with progress updates
func (ap *AsyncProvisioner) provisionCloudStackPodWithUpdates(ctx context.Context, pod *models.Pod, template *models.LabTemplate, req PodProvisionRequest) error {
	if ap.orchestrator.cloudstack == nil {
		return fmt.Errorf("CloudStack client not configured")
	}

	// Create pod networks
	ap.publishEvent(ctx, PodProvisioningEvent{
		PodID:     pod.ID,
		OwnerID:   req.OwnerID,
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseCreatingPod,
		Message:   "Creating pod networks",
		Progress:  10,
		Timestamp: time.Now(),
		RequestID: req.RequestID,
	})

	networks, networkMap, err := ap.orchestrator.createPodNetworks(ctx, pod, template)
	if err != nil {
		return fmt.Errorf("creating pod networks: %w", err)
	}
	pod.Networks = networks

	// Provision VMs
	totalVMs := len(template.Spec.VMs)
	for i, vmSpec := range template.Spec.VMs {
		baseProgress := 20 + (i * 70 / totalVMs)

		ap.publishEvent(ctx, PodProvisioningEvent{
			PodID:     pod.ID,
			OwnerID:   req.OwnerID,
			Status:    models.PodStatusProvisioning,
			Phase:     PhaseCloningVM,
			Message:   fmt.Sprintf("Deploying VM: %s (%d/%d)", vmSpec.Name, i+1, totalVMs),
			Progress:  baseProgress,
			VMName:    vmSpec.Name,
			VMStatus:  "deploying",
			Timestamp: time.Now(),
			RequestID: req.RequestID,
		})

		// Use the actual CloudStack provisioning from the orchestrator
		vm, err := ap.deployCloudStackVM(ctx, pod, vmSpec, template, networkMap)
		if err != nil {
			// Cleanup networks on failure
			ap.orchestrator.cleanupPodNetworks(ctx, networks)
			return err
		}

		pod.VMs = append(pod.VMs, *vm)
	}

	return nil
}

// deployCloudStackVM deploys a single VM to CloudStack.
// Not yet implemented — returns ErrNotImplemented instead of silently fabricating a result.
func (ap *AsyncProvisioner) deployCloudStackVM(_ context.Context, pod *models.Pod, vmSpec models.VMSpec, _ *models.LabTemplate, _ map[string]string) (*models.PodVM, error) {
	vmName := fmt.Sprintf("%s-%s", pod.Name, vmSpec.Name)
	return nil, fmt.Errorf("CloudStack async VM deployment for %q: %w", vmName, ErrNotImplemented)
}

// handleProvisioningError handles provisioning errors by updating status and publishing events
func (ap *AsyncProvisioner) handleProvisioningError(ctx context.Context, podID string, req PodProvisionRequest, err error, message string) {
	ap.logger.Error(message, "error", err, "podId", podID)

	// Update pod status to error
	_ = ap.orchestrator.updatePodStatus(ctx, podID, models.PodStatusError)

	// Publish failure event
	ap.publishEvent(ctx, PodProvisioningEvent{
		PodID:     podID,
		OwnerID:   req.OwnerID,
		Status:    models.PodStatusError,
		Phase:     PhaseFailed,
		Message:   message,
		Error:     err.Error(),
		Progress:  0,
		Timestamp: time.Now(),
		RequestID: req.RequestID,
	})
}

// publishEvent publishes a provisioning event to NATS and broadcasts via WebSocket
func (ap *AsyncProvisioner) publishEvent(ctx context.Context, event PodProvisioningEvent) {
	// Broadcast via WebSocket for clients without NATS access
	if ap.broadcaster != nil {
		ap.broadcaster.BroadcastProvisioningEvent(event)
	}

	// Publish to NATS for service-to-service communication
	if ap.publisher == nil {
		return
	}

	subject := fmt.Sprintf("%s.%s.provision", ap.subjectPrefix, event.PodID)
	data, err := json.Marshal(event)
	if err != nil {
		ap.logger.Error("Failed to marshal provisioning event", "error", err)
		return
	}

	if err := ap.publisher.Publish(ctx, subject, data); err != nil {
		ap.logger.Error("Failed to publish provisioning event", "error", err, "subject", subject)
	}
}

// ProvisioningEventSubject returns the NATS subject for a pod's provisioning events
func (ap *AsyncProvisioner) ProvisioningEventSubject(podID string) string {
	return fmt.Sprintf("%s.%s.provision", ap.subjectPrefix, podID)
}
