// Package orchestrator provides unified lab management across platforms
package orchestrator

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/apperrors"
	"github.com/toddbartholow/kootenai/api/internal/cloudstack"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/proxmox"
	"github.com/toddbartholow/kootenai/api/internal/util/namegen"
)

// ErrNotImplemented is returned when a platform operation is not yet supported.
var ErrNotImplemented = errors.New("operation not yet implemented for this platform")

// ErrNotFound is the canonical not-found error from apperrors.
var ErrNotFound = apperrors.ErrNotFound

// parseVMID converts a PlatformID string to an integer VMID for Proxmox.
func parseVMID(platformID string) (int, error) {
	vmid, err := strconv.Atoi(platformID)
	if err != nil {
		return 0, fmt.Errorf("invalid platform ID %q: %w", platformID, err)
	}
	if vmid <= 0 {
		return 0, fmt.Errorf("invalid VMID %d: must be positive", vmid)
	}
	return vmid, nil
}

// PodRepository defines the interface for pod persistence
type PodRepository interface {
	Create(ctx context.Context, pod *models.Pod) error
	GetByID(ctx context.Context, id string) (*models.Pod, error)
	List(ctx context.Context, filter PodFilter) ([]*models.Pod, error)
	Update(ctx context.Context, pod *models.Pod) error
	UpdateStatus(ctx context.Context, id string, status models.PodStatus) error
	Delete(ctx context.Context, id string) error
	GetExpired(ctx context.Context) ([]*models.Pod, error)
}

// PodFilter is an alias for the canonical filter type in the repositories package.
// This eliminates the need for adapter shims between orchestrator and database layers.
type PodFilter = repositories.PodFilter

// PodLifecycleRecorder is the subset of the metrics API needed to count pod
// creations and destructions. Implemented by *metrics.Metrics; nil means
// recording is disabled, which is the case in tests.
//
// Recording lives here rather than in the HTTP handlers because pods are also
// created by the async provisioner and destroyed by the lifecycle manager's
// expiry sweep. Those extra paths are why the counters live at this layer;
// the pods_active gauge is not maintained here at all — see metrics.PodCreated.
type PodLifecycleRecorder interface {
	PodCreated()
	PodDestroyed()
	VMOperationDuration(operation string, duration time.Duration)
}

// Orchestrator manages labs across Proxmox and CloudStack.
//
// Production callers MUST supply a PodRepository via WithPodRepository — the
// repo is the authoritative source for pod state. The in-memory pods map
// below exists only so tests can construct an Orchestrator without standing
// up a real database; every method has an `if o.hasPersistence() { ... } else
// { ... }` branch that doubles its size for this single test-only mode.
//
// Removing the in-memory path is a follow-up cleanup that would shrink this
// file by ~200 LOC; doing it now would touch ~20 test files at once.
type Orchestrator struct {
	proxmox    *proxmox.Client
	cloudstack *cloudstack.Client
	podRepo    PodRepository
	// pods is the test-only fallback store. Deprecated: tests only — never
	// nil in production. Audit item N2 (golang-pro pass 2) tracks removal.
	pods    map[string]*models.Pod
	podsMu  sync.RWMutex
	config  Config
	logger  *slog.Logger
	metrics PodLifecycleRecorder
}

// hasPersistence reports whether a real PodRepository is wired. False only in
// tests; production always passes one via WithPodRepository.
func (o *Orchestrator) hasPersistence() bool {
	return o.podRepo != nil
}

// Config holds orchestrator configuration
type Config struct {
	DefaultPlatform    models.Platform
	ProxmoxNode        string
	CloudStackZoneID   string
	PodNetworkBase     int
	MaxPodsPerUser     int
	DefaultPodDuration time.Duration
	// TemplateVMIDs maps template names (e.g., "ubuntu-22.04-desktop") to Proxmox VM IDs
	TemplateVMIDs map[string]int
	// CloudStackServiceOfferingID is the default service offering for CloudStack VMs
	CloudStackServiceOfferingID string
	// CloudStackTemplateIDs maps template names to CloudStack template IDs
	CloudStackTemplateIDs map[string]string
	// CloudStackNetworkID is the default network for CloudStack VMs (legacy fallback)
	CloudStackNetworkID string
	// CloudStackNetworkOfferingID is the network offering for isolated pod networks
	CloudStackNetworkOfferingID string
	// CloudStackUsePodNetworks enables per-pod network isolation (default: true when offering ID is set)
	CloudStackUsePodNetworks bool
}

// Option is a functional option for configuring the Orchestrator
type Option func(*Orchestrator)

// WithPodRepository sets the pod repository for database persistence
func WithPodRepository(repo PodRepository) Option {
	return func(o *Orchestrator) {
		o.podRepo = repo
	}
}

// WithMetrics sets the recorder notified when pods are created or destroyed.
// Injected at construction rather than set afterwards: the lifecycle sweep and
// the async provisioner read this field from their own goroutines, and those
// start before the HTTP server is built.
func WithMetrics(rec PodLifecycleRecorder) Option {
	return func(o *Orchestrator) {
		o.metrics = rec
	}
}

// WithLogger sets the logger
func WithLogger(logger *slog.Logger) Option {
	return func(o *Orchestrator) {
		o.logger = logger
	}
}

// New creates a new Orchestrator
func New(pve *proxmox.Client, cs *cloudstack.Client, cfg Config, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		proxmox:    pve,
		cloudstack: cs,
		pods:       make(map[string]*models.Pod),
		config:     cfg,
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// CreatePod creates a new lab pod from a template
// templateID and ownerID are the UUIDs for database foreign key references
func (o *Orchestrator) CreatePod(ctx context.Context, template *models.LabTemplate, templateID, ownerID, ownerName string, opts ...CreatePodOpts) (*models.Pod, error) {
	platform := template.Spec.Platform
	if platform == models.PlatformAny {
		platform = o.config.DefaultPlatform
	}

	podID := generatePodID(template.Metadata.Name, ownerName)
	podName := namegen.Generate()

	pod := &models.Pod{
		ID:            podID,
		Name:          podName,
		LabTemplateID: templateID,
		LabTemplate:   template.Metadata.Name,
		Platform:      platform,
		OwnerID:       ownerID,
		Owner:         ownerName,
		Status:        models.PodStatusProvisioning,
		VMs:           make([]models.PodVM, 0, len(template.Spec.VMs)),
		Networks:      make([]models.PodNetwork, 0, len(template.Spec.Network.Segments)),
		CreatedAt:     time.Now(),
	}

	// Apply multi-tenancy context if provided
	if len(opts) > 0 {
		pod.OrganizationID = opts[0].OrganizationID
		pod.TeamID = opts[0].TeamID
	}

	if o.config.DefaultPodDuration > 0 {
		exp := time.Now().Add(o.config.DefaultPodDuration)
		pod.ExpiresAt = &exp
	}

	// Persist pod - database is authoritative when configured
	if o.hasPersistence() {
		if err := o.podRepo.Create(ctx, pod); err != nil {
			return nil, fmt.Errorf("persisting pod to database: %w", err)
		}
	} else {
		// Fallback to in-memory when no database is configured
		o.podsMu.Lock()
		o.pods[podID] = pod
		o.podsMu.Unlock()
	}

	var err error
	provisionStart := time.Now()
	switch platform {
	case models.PlatformProxmox:
		err = o.provisionProxmoxPod(ctx, pod, template)
	case models.PlatformCloudStack:
		err = o.provisionCloudStackPod(ctx, pod, template)
	default:
		err = fmt.Errorf("unsupported platform: %s", platform)
	}
	if o.metrics != nil {
		o.metrics.VMOperationDuration("provision", time.Since(provisionStart))
	}

	if err != nil {
		pod.Status = models.PodStatusError
		_ = o.updatePodStatus(ctx, podID, models.PodStatusError)
		return pod, fmt.Errorf("provisioning pod: %w", err)
	}

	pod.Status = models.PodStatusRunning

	// Update final state with VM details
	if o.hasPersistence() {
		if err := o.podRepo.Update(ctx, pod); err != nil {
			// Pod was created but we couldn't update with VM details
			// Log error but return pod since resources were provisioned
			o.logger.Error("Failed to update pod in database after provisioning",
				"error", err, "podId", podID)
		}
	} else {
		// Update in-memory state
		o.podsMu.Lock()
		o.pods[podID] = pod
		o.podsMu.Unlock()
	}

	if o.metrics != nil {
		o.metrics.PodCreated()
	}
	return pod, nil
}

// updatePodStatus updates pod status in the authoritative store.
// When database is configured, it updates the database only.
// When no database is configured, it updates the in-memory store.
func (o *Orchestrator) updatePodStatus(ctx context.Context, podID string, status models.PodStatus) error {
	if o.hasPersistence() {
		if err := o.podRepo.UpdateStatus(ctx, podID, status); err != nil {
			o.logger.Error("Failed to update pod status in database",
				"error", err, "podId", podID, "status", status)
			return fmt.Errorf("updating pod status: %w", err)
		}
	} else {
		o.podsMu.Lock()
		if pod, ok := o.pods[podID]; ok {
			pod.Status = status
		}
		o.podsMu.Unlock()
	}
	return nil
}

// updateVMStatus updates the status of a specific VM within a pod.
// It fetches the pod, updates the VM status, and persists the change.
func (o *Orchestrator) updateVMStatus(ctx context.Context, podID, vmName, status string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return fmt.Errorf("getting pod: %w", err)
	}

	// Find and update the VM status
	found := false
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			pod.VMs[i].Status = status
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	// Persist the change
	if o.hasPersistence() {
		if err := o.podRepo.Update(ctx, pod); err != nil {
			o.logger.Error("Failed to update VM status in database",
				"error", err, "podId", podID, "vmName", vmName, "status", status)
			return fmt.Errorf("updating VM status: %w", err)
		}
	} else {
		o.podsMu.Lock()
		o.pods[podID] = pod
		o.podsMu.Unlock()
	}

	o.logger.Info("VM status updated", "podId", podID, "vmName", vmName, "status", status)
	return nil
}

func (o *Orchestrator) provisionProxmoxPod(ctx context.Context, pod *models.Pod, template *models.LabTemplate) error {
	node := o.config.ProxmoxNode

	for _, vmSpec := range template.Spec.VMs {
		templateVMID, err := o.resolveTemplateVMID(vmSpec.Template)
		if err != nil {
			return fmt.Errorf("resolving template %q: %w", vmSpec.Template, err)
		}
		newVMID, err := generateVMID()
		if err != nil {
			return fmt.Errorf("generating VMID for %s: %w", vmSpec.Name, err)
		}

		vmName := fmt.Sprintf("%s-%s", pod.Name, vmSpec.Name)
		if err := o.proxmox.CloneVM(ctx, node, templateVMID, newVMID, vmName, true); err != nil {
			return fmt.Errorf("cloning VM %s: %w", vmSpec.Name, err)
		}

		for _, snap := range vmSpec.Snapshots {
			err := o.proxmox.CreateSnapshot(ctx, node, newVMID, snap.Name, snap.Description, snap.IncludeRAM)
			if err != nil {
				return fmt.Errorf("creating snapshot %s: %w", snap.Name, err)
			}
		}

		if vmSpec.StartOnCreate {
			// Wait for any snapshot locks to be released before starting
			for i := 0; i < 30; i++ {
				vm, err := o.proxmox.GetVM(ctx, node, newVMID)
				if err != nil {
					o.logger.Warn("Failed to check VM lock status", "vmid", newVMID, "error", err)
					break
				}
				if vm.Lock == "" {
					break
				}
				o.logger.Debug("VM locked, waiting", "vmid", newVMID, "lock", vm.Lock, "attempt", i+1)
				select {
				case <-ctx.Done():
					return fmt.Errorf("waiting for VM %d lock release: %w", newVMID, ctx.Err())
				case <-time.After(1 * time.Second):
				}
			}

			if err := o.proxmox.StartVM(ctx, node, newVMID); err != nil {
				return fmt.Errorf("starting VM %s: %w", vmSpec.Name, err)
			}
		}

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

func (o *Orchestrator) provisionCloudStackPod(ctx context.Context, pod *models.Pod, template *models.LabTemplate) error {
	if o.cloudstack == nil {
		return fmt.Errorf("CloudStack client not configured")
	}

	zoneID := o.config.CloudStackZoneID
	if zoneID == "" {
		return fmt.Errorf("CloudStack zone ID not configured")
	}

	serviceOfferingID := o.config.CloudStackServiceOfferingID
	if serviceOfferingID == "" {
		return fmt.Errorf("CloudStack service offering ID not configured")
	}

	// Create pod networks first (if isolation is enabled)
	networks, networkMap, err := o.createPodNetworks(ctx, pod, template)
	if err != nil {
		return fmt.Errorf("creating pod networks: %w", err)
	}
	pod.Networks = networks

	// Rollback networks if VM deployment fails
	provisionSuccess := false
	defer func() {
		if !provisionSuccess && len(networks) > 0 {
			o.logger.Info("Rolling back pod networks due to provisioning failure", "podId", pod.ID)
			o.cleanupPodNetworks(ctx, networks)
		}
	}()

	for _, vmSpec := range template.Spec.VMs {
		// Resolve CloudStack template ID
		templateID, err := o.resolveCloudStackTemplateID(vmSpec.Template)
		if err != nil {
			return fmt.Errorf("resolving template %q: %w", vmSpec.Template, err)
		}

		vmName := fmt.Sprintf("%s-%s", pod.Name, vmSpec.Name)

		// Build deployment params
		deployParams := cloudstack.DeployVirtualMachineParams{
			Name:              vmName,
			DisplayName:       vmName,
			ZoneID:            zoneID,
			TemplateID:        templateID,
			ServiceOfferingID: serviceOfferingID,
			StartVM:           vmSpec.StartOnCreate,
		}

		// Resolve network IDs for this VM
		deployParams.NetworkIDs = o.resolveVMNetworkIDs(vmSpec, networkMap)

		o.logger.Info("Deploying CloudStack VM",
			"podId", pod.ID,
			"vmName", vmName,
			"template", vmSpec.Template,
			"templateId", templateID,
			"networkIds", deployParams.NetworkIDs,
		)

		// Deploy the VM
		vm, err := o.cloudstack.DeployVirtualMachine(ctx, deployParams)
		if err != nil {
			return fmt.Errorf("deploying VM %s: %w", vmSpec.Name, err)
		}

		o.logger.Info("CloudStack VM deployed",
			"podId", pod.ID,
			"vmName", vmName,
			"vmId", vm.ID,
			"state", vm.State,
		)

		// Create snapshots if specified
		for _, snap := range vmSpec.Snapshots {
			o.logger.Info("Creating CloudStack VM snapshot",
				"vmId", vm.ID,
				"snapshot", snap.Name,
			)

			_, err := o.cloudstack.CreateVMSnapshot(ctx, vm.ID, snap.Name, snap.Description, snap.IncludeRAM)
			if err != nil {
				return fmt.Errorf("creating snapshot %s for VM %s: %w", snap.Name, vmSpec.Name, err)
			}
		}

		// Add VM to pod
		pod.VMs = append(pod.VMs, models.PodVM{
			Name:       vmSpec.Name,
			PlatformID: vm.ID,
			Platform:   models.PlatformCloudStack,
			Node:       zoneID, // Use zone as "node" for CloudStack
			Status:     vm.State,
			IPAddress:  vm.IPAddress,
		})
	}

	provisionSuccess = true
	return nil
}

// resolveCloudStackTemplateID looks up the CloudStack template ID for a template name
func (o *Orchestrator) resolveCloudStackTemplateID(templateName string) (string, error) {
	if o.config.CloudStackTemplateIDs == nil {
		return "", fmt.Errorf("no CloudStack template ID mappings configured")
	}

	templateID, ok := o.config.CloudStackTemplateIDs[templateName]
	if !ok {
		// Log available templates to help with debugging
		available := make([]string, 0, len(o.config.CloudStackTemplateIDs))
		for name := range o.config.CloudStackTemplateIDs {
			available = append(available, name)
		}
		o.logger.Error("CloudStack template not found in ID mappings",
			"template", templateName,
			"available", available)
		return "", fmt.Errorf("template %q not found in CloudStack template ID mappings", templateName)
	}

	return templateID, nil
}

// createPodNetworks creates isolated networks for a pod based on template network segments
func (o *Orchestrator) createPodNetworks(ctx context.Context, pod *models.Pod, template *models.LabTemplate) ([]models.PodNetwork, map[string]string, error) {
	// Check if pod network isolation is enabled
	if !o.config.CloudStackUsePodNetworks || o.config.CloudStackNetworkOfferingID == "" {
		// Return empty - will use legacy global network
		return nil, nil, nil
	}

	networks := make([]models.PodNetwork, 0, len(template.Spec.Network.Segments))
	networkMap := make(map[string]string) // segmentName -> networkID

	for _, segment := range template.Spec.Network.Segments {
		networkName := fmt.Sprintf("%s-%s", pod.ID, segment.Name)

		params := cloudstack.CreateNetworkParams{
			Name:              networkName,
			DisplayText:       fmt.Sprintf("Pod %s network: %s", pod.ID, segment.Name),
			NetworkOfferingID: o.config.CloudStackNetworkOfferingID,
			ZoneID:            o.config.CloudStackZoneID,
		}

		o.logger.Info("Creating pod network",
			"podId", pod.ID,
			"segment", segment.Name,
			"networkName", networkName,
		)

		network, err := o.cloudstack.CreateNetwork(ctx, params)
		if err != nil {
			// Cleanup any networks we already created
			o.cleanupPodNetworks(ctx, networks)
			return nil, nil, fmt.Errorf("creating network %s: %w", segment.Name, err)
		}

		o.logger.Info("Created pod network",
			"podId", pod.ID,
			"networkId", network.ID,
			"networkName", network.Name,
		)

		podNetwork := models.PodNetwork{
			Name:       segment.Name,
			PlatformID: network.ID,
			VLAN:       segment.VLAN,
			Subnet:     segment.Subnet,
			Gateway:    network.Gateway,
			Netmask:    network.Netmask,
			Type:       "isolated",
			State:      network.State,
		}
		networks = append(networks, podNetwork)
		networkMap[segment.Name] = network.ID
	}

	return networks, networkMap, nil
}

// resolveVMNetworkIDs maps VM network specs to created network IDs
func (o *Orchestrator) resolveVMNetworkIDs(vmSpec models.VMSpec, networkMap map[string]string) []string {
	// If no pod networks were created, use legacy global network
	if len(networkMap) == 0 {
		if o.config.CloudStackNetworkID != "" {
			return []string{o.config.CloudStackNetworkID}
		}
		return nil
	}

	// Map VM's network connections to network IDs
	networkIDs := make([]string, 0, len(vmSpec.Networks))
	for _, netConn := range vmSpec.Networks {
		if networkID, ok := networkMap[netConn.Segment]; ok {
			networkIDs = append(networkIDs, networkID)
		}
	}

	// If VM has no specific network assignments, use first pod network
	if len(networkIDs) == 0 && len(networkMap) > 0 {
		for _, networkID := range networkMap {
			networkIDs = append(networkIDs, networkID)
			break
		}
	}

	return networkIDs
}

// cleanupPodNetworks deletes pod networks (for rollback and destroy)
func (o *Orchestrator) cleanupPodNetworks(ctx context.Context, networks []models.PodNetwork) {
	for _, net := range networks {
		if net.PlatformID == "" {
			continue
		}
		o.logger.Info("Cleaning up pod network", "networkId", net.PlatformID, "name", net.Name)
		if err := o.cloudstack.DeleteNetwork(ctx, net.PlatformID); err != nil {
			if errors.Is(err, cloudstack.ErrNetworkNotFound) {
				// Already deleted, skip
				continue
			}
			if errors.Is(err, cloudstack.ErrNetworkInUse) {
				// Network still has VMs attached - will be cleaned up later
				o.logger.Warn("Network still in use, will retry or be cleaned up by CloudStack GC",
					"networkId", net.PlatformID,
					"name", net.Name,
				)
				continue
			}
			o.logger.Error("Failed to cleanup network",
				"networkId", net.PlatformID,
				"name", net.Name,
				"error", err,
			)
		}
	}
}

// resolveTemplateVMID looks up the Proxmox VMID for a template name
func (o *Orchestrator) resolveTemplateVMID(templateName string) (int, error) {
	if o.config.TemplateVMIDs == nil {
		return 0, fmt.Errorf("no template VMID mappings configured")
	}

	vmid, ok := o.config.TemplateVMIDs[templateName]
	if !ok {
		// Log available templates to help with debugging
		available := make([]string, 0, len(o.config.TemplateVMIDs))
		for name := range o.config.TemplateVMIDs {
			available = append(available, name)
		}
		o.logger.Error("Template not found in VMID mappings",
			"template", templateName,
			"available", available)
		return 0, fmt.Errorf("template %q not found in VMID mappings", templateName)
	}

	return vmid, nil
}

// GetPod returns a pod by ID.
// Database is the authoritative source when configured.
func (o *Orchestrator) GetPod(ctx context.Context, podID string) (*models.Pod, error) {
	var pod *models.Pod

	if o.hasPersistence() {
		var err error
		pod, err = o.podRepo.GetByID(ctx, podID)
		if err != nil {
			return nil, fmt.Errorf("getting pod from database: %w", err)
		}
		if pod == nil {
			return nil, fmt.Errorf("pod %s: %w", podID, ErrNotFound)
		}
	} else {
		// Fallback to in-memory when no database is configured
		o.podsMu.RLock()
		p, ok := o.pods[podID]
		o.podsMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("pod %s: %w", podID, ErrNotFound)
		}
		pod = p
	}

	// Enrich running Proxmox VMs with IP addresses from the guest agent
	if pod.Status == "running" && pod.Platform == models.PlatformProxmox && o.proxmox != nil {
		for i := range pod.VMs {
			if pod.VMs[i].IPAddress == "" && pod.VMs[i].PlatformID != "" {
				ip, err := o.proxmox.GetVMIP(ctx, pod.VMs[i].PlatformID)
				if err != nil {
					o.logger.Debug("could not get VM IP from guest agent",
						"vmid", pod.VMs[i].PlatformID, "error", err)
					continue
				}
				pod.VMs[i].IPAddress = ip
			}
		}
	}

	return pod, nil
}

// ListPods returns all pods, optionally filtered by owner.
// Database is the authoritative source when configured.
// Deprecated: Use ListPodsWithFilter for more control.
func (o *Orchestrator) ListPods(ctx context.Context, owner string) ([]*models.Pod, error) {
	return o.ListPodsWithFilter(ctx, PodFilter{OwnerID: owner})
}

// ListPodsWithFilter returns all pods matching the filter criteria.
// Database is the authoritative source when configured.
func (o *Orchestrator) ListPodsWithFilter(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
	if o.hasPersistence() {
		pods, err := o.podRepo.List(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("listing pods from database: %w", err)
		}
		return pods, nil
	}

	// Fallback to in-memory when no database is configured
	o.podsMu.RLock()
	defer o.podsMu.RUnlock()

	var result []*models.Pod
	for _, pod := range o.pods {
		// Apply owner filter
		if filter.OwnerID != "" && pod.Owner != filter.OwnerID && pod.OwnerID != filter.OwnerID {
			continue
		}
		// Apply organization filter
		if filter.OrganizationID != "" {
			if pod.OrganizationID == nil || *pod.OrganizationID != filter.OrganizationID {
				continue
			}
		}
		// Apply status filter
		if filter.Status != "" && pod.Status != filter.Status {
			continue
		}
		// Apply template filter
		if filter.TemplateID != "" && pod.LabTemplateID != filter.TemplateID && pod.LabTemplate != filter.TemplateID {
			continue
		}
		// Apply platform filter
		if filter.Platform != "" && string(pod.Platform) != filter.Platform {
			continue
		}
		result = append(result, pod)
	}

	return result, nil
}

// ResetPodVM resets a VM to a named snapshot
func (o *Orchestrator) ResetPodVM(ctx context.Context, podID, vmName, snapshotName string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}

	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", targetVM.Name, err)
		}
		if err := o.proxmox.RevertSnapshot(ctx, targetVM.Node, vmid, snapshotName); err != nil {
			return fmt.Errorf("reverting snapshot: %w", err)
		}
	case models.PlatformCloudStack:
		if err := o.cloudstack.RevertToVMSnapshot(ctx, targetVM.PlatformID); err != nil {
			return fmt.Errorf("reverting snapshot: %w", err)
		}
	}

	targetVM.CurrentSnapshot = snapshotName
	return nil
}

// ConsoleTicket represents console access information for a VM
type ConsoleTicket struct {
	Type     string `json:"type"` // "spice" or "vnc"
	Host     string `json:"host"`
	Port     int    `json:"port"`
	TLSPort  int    `json:"tlsPort,omitempty"`
	Ticket   string `json:"ticket"`
	Password string `json:"password,omitempty"`
	Node     string `json:"node"`
	VMID     string `json:"vmid"`
}

// GetVMConsole returns console access information for a VM in a pod
func (o *Orchestrator) GetVMConsole(ctx context.Context, podID, vmName, consoleType string) (*ConsoleTicket, error) {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return nil, err
	}

	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}

	if targetVM == nil {
		return nil, fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return nil, fmt.Errorf("parsing VM ID for %s: %w", targetVM.Name, err)
		}

		// Get the Proxmox host for console URL
		proxmoxHost := ""
		if o.proxmox != nil {
			proxmoxHost = o.proxmox.Host()
		}

		if consoleType == "spice" {
			// Explicitly requested SPICE
			ticket, err := o.proxmox.GetSpiceProxy(ctx, targetVM.Node, vmid)
			if err != nil {
				return nil, fmt.Errorf("getting SPICE proxy: %w", err)
			}
			return &ConsoleTicket{
				Type:     "spice",
				Host:     ticket.Host,
				Port:     ticket.Port,
				TLSPort:  ticket.TLSPort,
				Ticket:   ticket.Ticket,
				Password: ticket.Password,
				Node:     targetVM.Node,
				VMID:     targetVM.PlatformID,
			}, nil
		} else if consoleType == "vnc" || consoleType == "" {
			// VNC requested or auto-detect (try VNC first as most VMs use VNC by default)
			ticket, err := o.proxmox.GetVNCProxy(ctx, targetVM.Node, vmid)
			if err != nil {
				// If VNC fails and auto-detect, try SPICE as fallback
				if consoleType == "" {
					spiceTicket, spiceErr := o.proxmox.GetSpiceProxy(ctx, targetVM.Node, vmid)
					if spiceErr == nil {
						return &ConsoleTicket{
							Type:     "spice",
							Host:     spiceTicket.Host,
							Port:     spiceTicket.Port,
							TLSPort:  spiceTicket.TLSPort,
							Ticket:   spiceTicket.Ticket,
							Password: spiceTicket.Password,
							Node:     targetVM.Node,
							VMID:     targetVM.PlatformID,
						}, nil
					}
				}
				return nil, fmt.Errorf("getting VNC proxy: %w", err)
			}
			return &ConsoleTicket{
				Type:   "vnc",
				Host:   proxmoxHost,
				Port:   ticket.PortInt(),
				Ticket: ticket.Ticket,
				Node:   targetVM.Node,
				VMID:   targetVM.PlatformID,
			}, nil
		}
		return nil, fmt.Errorf("unsupported console type: %s", consoleType)

	case models.PlatformCloudStack:
		// CloudStack console access would go here
		return nil, fmt.Errorf("console access for CloudStack: %w", ErrNotImplemented)

	default:
		return nil, fmt.Errorf("unknown platform: %s", targetVM.Platform)
	}
}

// GetDirectVMConsole returns console access for a VM by VMID directly (bypasses pod lookup)
// This is useful for development/testing when pods aren't registered in the database
func (o *Orchestrator) GetDirectVMConsole(ctx context.Context, node string, vmid int, consoleType string) (*ConsoleTicket, error) {
	if o.proxmox == nil {
		return nil, fmt.Errorf("Proxmox client not configured")
	}

	proxmoxHost := o.proxmox.Host()
	vmidStr := fmt.Sprintf("%d", vmid)

	if consoleType == "spice" {
		ticket, err := o.proxmox.GetSpiceProxy(ctx, node, vmid)
		if err != nil {
			return nil, fmt.Errorf("getting SPICE proxy: %w", err)
		}
		return &ConsoleTicket{
			Type:     "spice",
			Host:     ticket.Host,
			Port:     ticket.Port,
			TLSPort:  ticket.TLSPort,
			Ticket:   ticket.Ticket,
			Password: ticket.Password,
			Node:     node,
			VMID:     vmidStr,
		}, nil
	}

	// Default to VNC
	ticket, err := o.proxmox.GetVNCProxy(ctx, node, vmid)
	if err != nil {
		// Try SPICE as fallback
		spiceTicket, spiceErr := o.proxmox.GetSpiceProxy(ctx, node, vmid)
		if spiceErr == nil {
			return &ConsoleTicket{
				Type:     "spice",
				Host:     spiceTicket.Host,
				Port:     spiceTicket.Port,
				TLSPort:  spiceTicket.TLSPort,
				Ticket:   spiceTicket.Ticket,
				Password: spiceTicket.Password,
				Node:     node,
				VMID:     vmidStr,
			}, nil
		}
		return nil, fmt.Errorf("getting VNC proxy: %w", err)
	}

	return &ConsoleTicket{
		Type:   "vnc",
		Host:   proxmoxHost,
		Port:   ticket.PortInt(),
		Ticket: ticket.Ticket,
		Node:   node,
		VMID:   vmidStr,
	}, nil
}

// DestroyPod destroys all resources in a pod
func (o *Orchestrator) DestroyPod(ctx context.Context, podID string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	// Destroy is a soft delete, so the row survives and GetPod keeps returning
	// it. Without this guard, replaying DELETE on a pod with no VMs left runs
	// the whole body again — the VM loop is a no-op, errs stays empty, and
	// pods_destroyed_total increments once per request, unbounded.
	if pod.Status == models.PodStatusDestroyed {
		return nil
	}

	// Update status to destroying
	if err := o.updatePodStatus(ctx, podID, models.PodStatusDestroying); err != nil {
		o.logger.Error("Failed to set destroying status", "error", err, "podId", podID)
		// Continue with destruction anyway
	}

	var errs []error
	hasCloudStackVMs := false

	for _, vm := range pod.VMs {
		switch vm.Platform {
		case models.PlatformProxmox:
			vmid, err := parseVMID(vm.PlatformID)
			if err != nil {
				errs = append(errs, fmt.Errorf("parsing VM ID for %s: %w", vm.Name, err))
				continue
			}
			// Stop is best-effort: if the VM is already stopped, StopVM will
			// return an error that WaitForVMStopped + DeleteVM still tolerate.
			if err := o.proxmox.StopVM(ctx, vm.Node, vmid); err != nil {
				o.logger.Debug("StopVM failed during pod destroy (often already stopped)", "vmid", vmid, "error", err)
			}
			// Wait for VM to fully stop before deleting (max 30 seconds)
			if err := o.proxmox.WaitForVMStopped(ctx, vm.Node, vmid, 30*time.Second); err != nil {
				o.logger.Warn("Timeout waiting for VM to stop, attempting delete anyway", "vmid", vmid, "error", err)
			}
			if err := o.proxmox.DeleteVM(ctx, vm.Node, vmid); err != nil {
				errs = append(errs, err)
			}
		case models.PlatformCloudStack:
			hasCloudStackVMs = true
			if err := o.cloudstack.DestroyVirtualMachine(ctx, vm.PlatformID, true); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Clean up pod networks (CloudStack only)
	if hasCloudStackVMs && len(pod.Networks) > 0 {
		// Wait for VMs to fully detach from networks (race condition fix)
		o.logger.Info("Waiting for VMs to detach before cleaning up networks", "podId", podID)
		select {
		case <-ctx.Done():
			// Context canceled, but still cleanup networks using background context
			// to prevent resource leak
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			o.cleanupPodNetworks(cleanupCtx, pod.Networks)
		case <-time.After(5 * time.Second):
			o.cleanupPodNetworks(ctx, pod.Networks)
		}
	}

	if len(errs) > 0 {
		_ = o.updatePodStatus(ctx, podID, models.PodStatusError)
		return fmt.Errorf("destroying pod: %w", errors.Join(errs...))
	}

	// Mark as destroyed
	if err := o.updatePodStatus(ctx, podID, models.PodStatusDestroyed); err != nil {
		o.logger.Error("Failed to set destroyed status", "error", err, "podId", podID)
	}

	// Remove from in-memory store (only when no database is configured)
	if !o.hasPersistence() {
		o.podsMu.Lock()
		delete(o.pods, podID)
		o.podsMu.Unlock()
	}

	if o.metrics != nil {
		o.metrics.PodDestroyed()
	}
	return nil
}

// StopPod stops all VMs in a running pod without destroying it
func (o *Orchestrator) StopPod(ctx context.Context, podID string) (*models.Pod, error) {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return nil, err
	}

	// Verify pod is in a stoppable state
	if pod.Status != models.PodStatusRunning {
		return nil, fmt.Errorf("pod must be running to stop (current status: %s)", pod.Status)
	}

	o.logger.Info("Stopping pod", "podId", podID, "vmCount", len(pod.VMs))

	var errs []error
	for _, vm := range pod.VMs {
		switch vm.Platform {
		case models.PlatformProxmox:
			vmid, err := parseVMID(vm.PlatformID)
			if err != nil {
				errs = append(errs, fmt.Errorf("parsing VM ID for %s: %w", vm.Name, err))
				continue
			}
			o.logger.Info("Stopping Proxmox VM", "podId", podID, "vm", vm.Name, "vmid", vmid)
			if err := o.proxmox.StopVM(ctx, vm.Node, vmid); err != nil {
				o.logger.Error("Failed to stop VM", "error", err, "vm", vm.Name)
				errs = append(errs, fmt.Errorf("stopping VM %s: %w", vm.Name, err))
			}
		case models.PlatformCloudStack:
			o.logger.Info("Stopping CloudStack VM", "podId", podID, "vm", vm.Name, "vmId", vm.PlatformID)
			if err := o.cloudstack.StopVirtualMachine(ctx, vm.PlatformID, false); err != nil {
				o.logger.Error("Failed to stop VM", "error", err, "vm", vm.Name)
				errs = append(errs, fmt.Errorf("stopping VM %s: %w", vm.Name, err))
			}
		}
	}

	if len(errs) > 0 {
		_ = o.updatePodStatus(ctx, podID, models.PodStatusError)
		return pod, fmt.Errorf("stopping pod: %w", errors.Join(errs...))
	}

	// Update status to stopped
	pod.Status = models.PodStatusStopped
	if err := o.updatePodStatus(ctx, podID, models.PodStatusStopped); err != nil {
		o.logger.Error("Failed to update pod status", "error", err, "podId", podID)
	}

	o.logger.Info("Pod stopped successfully", "podId", podID)
	return pod, nil
}

// StartPod starts all VMs in a stopped pod
func (o *Orchestrator) StartPod(ctx context.Context, podID string) (*models.Pod, error) {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return nil, err
	}

	// Verify pod is in a startable state
	if pod.Status != models.PodStatusStopped {
		return nil, fmt.Errorf("pod must be stopped to start (current status: %s)", pod.Status)
	}

	o.logger.Info("Starting pod", "podId", podID, "vmCount", len(pod.VMs))

	var errs []error
	for _, vm := range pod.VMs {
		switch vm.Platform {
		case models.PlatformProxmox:
			vmid, err := parseVMID(vm.PlatformID)
			if err != nil {
				errs = append(errs, fmt.Errorf("parsing VM ID for %s: %w", vm.Name, err))
				continue
			}
			o.logger.Info("Starting Proxmox VM", "podId", podID, "vm", vm.Name, "vmid", vmid)
			if err := o.proxmox.StartVM(ctx, vm.Node, vmid); err != nil {
				o.logger.Error("Failed to start VM", "error", err, "vm", vm.Name)
				errs = append(errs, fmt.Errorf("starting VM %s: %w", vm.Name, err))
			}
		case models.PlatformCloudStack:
			o.logger.Info("Starting CloudStack VM", "podId", podID, "vm", vm.Name, "vmId", vm.PlatformID)
			if err := o.cloudstack.StartVirtualMachine(ctx, vm.PlatformID); err != nil {
				o.logger.Error("Failed to start VM", "error", err, "vm", vm.Name)
				errs = append(errs, fmt.Errorf("starting VM %s: %w", vm.Name, err))
			}
		}
	}

	if len(errs) > 0 {
		_ = o.updatePodStatus(ctx, podID, models.PodStatusError)
		return pod, fmt.Errorf("starting pod: %w", errors.Join(errs...))
	}

	// Update status to running
	pod.Status = models.PodStatusRunning
	if err := o.updatePodStatus(ctx, podID, models.PodStatusRunning); err != nil {
		o.logger.Error("Failed to update pod status", "error", err, "podId", podID)
	}

	o.logger.Info("Pod started successfully", "podId", podID)
	return pod, nil
}

// StartVM starts a single VM in a pod
func (o *Orchestrator) StartVM(ctx context.Context, podID, vmName string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	// Find the VM
	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}
	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		o.logger.Info("Starting Proxmox VM", "podId", podID, "vm", vmName, "vmid", vmid)
		if err := o.proxmox.StartVM(ctx, targetVM.Node, vmid); err != nil {
			return fmt.Errorf("starting VM: %w", err)
		}
	case models.PlatformCloudStack:
		o.logger.Info("Starting CloudStack VM", "podId", podID, "vm", vmName, "vmId", targetVM.PlatformID)
		if err := o.cloudstack.StartVirtualMachine(ctx, targetVM.PlatformID); err != nil {
			return fmt.Errorf("starting VM: %w", err)
		}
	default:
		return fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}

	// Update VM status in database
	if err := o.updateVMStatus(ctx, podID, vmName, "running"); err != nil {
		o.logger.Warn("Failed to update VM status after start", "error", err)
	}

	o.logger.Info("VM started", "podId", podID, "vm", vmName)
	return nil
}

// StartVMDirect starts a Proxmox VM directly by node and vmid without pod lookup
// This is useful for starting VMs immediately after cloning when the pod status is "running"
func (o *Orchestrator) StartVMDirect(ctx context.Context, node string, vmid int) error {
	if o.proxmox == nil {
		return fmt.Errorf("proxmox client not configured")
	}
	o.logger.Info("Starting VM directly", "node", node, "vmid", vmid)
	return o.proxmox.StartVM(ctx, node, vmid)
}

// StopVMDirect force-stops a Proxmox VM directly by node and vmid
func (o *Orchestrator) StopVMDirect(ctx context.Context, node string, vmid int) error {
	if o.proxmox == nil {
		return fmt.Errorf("proxmox client not configured")
	}
	o.logger.Info("Stopping VM directly", "node", node, "vmid", vmid)
	return o.proxmox.StopVM(ctx, node, vmid)
}

// ShutdownVMDirect gracefully shuts down a Proxmox VM directly by node and vmid
func (o *Orchestrator) ShutdownVMDirect(ctx context.Context, node string, vmid int) error {
	if o.proxmox == nil {
		return fmt.Errorf("proxmox client not configured")
	}
	o.logger.Info("Shutting down VM directly", "node", node, "vmid", vmid)
	return o.proxmox.ShutdownVM(ctx, node, vmid)
}

// RebootVMDirect reboots a Proxmox VM directly by node and vmid
func (o *Orchestrator) RebootVMDirect(ctx context.Context, node string, vmid int) error {
	if o.proxmox == nil {
		return fmt.Errorf("proxmox client not configured")
	}
	o.logger.Info("Rebooting VM directly", "node", node, "vmid", vmid)
	return o.proxmox.RebootVM(ctx, node, vmid)
}

// StopVM stops a single VM in a pod
func (o *Orchestrator) StopVM(ctx context.Context, podID, vmName string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	// Find the VM
	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}
	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		o.logger.Info("Stopping Proxmox VM", "podId", podID, "vm", vmName, "vmid", vmid)
		if err := o.proxmox.StopVM(ctx, targetVM.Node, vmid); err != nil {
			return fmt.Errorf("stopping VM: %w", err)
		}
	case models.PlatformCloudStack:
		o.logger.Info("Stopping CloudStack VM", "podId", podID, "vm", vmName, "vmId", targetVM.PlatformID)
		if err := o.cloudstack.StopVirtualMachine(ctx, targetVM.PlatformID, false); err != nil {
			return fmt.Errorf("stopping VM: %w", err)
		}
	default:
		return fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}

	// Update VM status in database
	if err := o.updateVMStatus(ctx, podID, vmName, "stopped"); err != nil {
		o.logger.Warn("Failed to update VM status after stop", "error", err)
	}

	o.logger.Info("VM stopped", "podId", podID, "vm", vmName)
	return nil
}

// SuspendVM suspends a single VM in a pod (Proxmox only)
func (o *Orchestrator) SuspendVM(ctx context.Context, podID, vmName string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	// Find the VM
	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}
	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		o.logger.Info("Suspending Proxmox VM", "podId", podID, "vm", vmName, "vmid", vmid)
		if err := o.proxmox.SuspendVM(ctx, targetVM.Node, vmid); err != nil {
			return fmt.Errorf("suspending VM: %w", err)
		}
	case models.PlatformCloudStack:
		return fmt.Errorf("suspend not supported for CloudStack VMs")
	default:
		return fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}

	// Update VM status in database
	if err := o.updateVMStatus(ctx, podID, vmName, "suspended"); err != nil {
		o.logger.Warn("Failed to update VM status after suspend", "error", err)
	}

	o.logger.Info("VM suspended", "podId", podID, "vm", vmName)
	return nil
}

// ResumeVM resumes a suspended VM in a pod (Proxmox only)
func (o *Orchestrator) ResumeVM(ctx context.Context, podID, vmName string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	// Find the VM
	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}
	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		o.logger.Info("Resuming Proxmox VM", "podId", podID, "vm", vmName, "vmid", vmid)
		if err := o.proxmox.ResumeVM(ctx, targetVM.Node, vmid); err != nil {
			return fmt.Errorf("resuming VM: %w", err)
		}
	case models.PlatformCloudStack:
		return fmt.Errorf("resume not supported for CloudStack VMs")
	default:
		return fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}

	// Update VM status in database
	if err := o.updateVMStatus(ctx, podID, vmName, "running"); err != nil {
		o.logger.Warn("Failed to update VM status after resume", "error", err)
	}

	o.logger.Info("VM resumed", "podId", podID, "vm", vmName)
	return nil
}

// ProxmoxAuthHeader returns the Proxmox API token authentication header.
// Returns empty string if Proxmox client is not configured.
func (o *Orchestrator) ProxmoxAuthHeader() string {
	if o.proxmox == nil {
		return ""
	}
	return o.proxmox.AuthHeader()
}

// ProxmoxHostname returns the Proxmox server hostname/IP.
// Returns empty string if Proxmox client is not configured.
func (o *Orchestrator) ProxmoxHostname() string {
	if o.proxmox == nil {
		return ""
	}
	return o.proxmox.Hostname()
}

// LoadPodsFromDatabase is a no-op when database persistence is configured.
// Previously this loaded pods into an in-memory cache, but now the database
// is the authoritative source and queries go directly to it.
// This method is retained for backward compatibility but does nothing.
func (o *Orchestrator) LoadPodsFromDatabase(ctx context.Context) error {
	if !o.hasPersistence() {
		return nil
	}

	// Verify database connectivity by doing a simple query
	_, err := o.podRepo.List(ctx, PodFilter{Limit: 1})
	if err != nil {
		return fmt.Errorf("verifying database connectivity: %w", err)
	}

	o.logger.Info("Database persistence configured and verified")
	return nil
}

// GetExpiredPods returns pods that have passed their expiration time.
// Database is the authoritative source when configured.
func (o *Orchestrator) GetExpiredPods(ctx context.Context) ([]*models.Pod, error) {
	if o.hasPersistence() {
		return o.podRepo.GetExpired(ctx)
	}

	// Fallback to in-memory check when no database is configured
	o.podsMu.RLock()
	defer o.podsMu.RUnlock()

	var expired []*models.Pod
	now := time.Now()
	for _, pod := range o.pods {
		if pod.ExpiresAt != nil && pod.ExpiresAt.Before(now) &&
			pod.Status != models.PodStatusDestroyed && pod.Status != models.PodStatusDestroying {
			expired = append(expired, pod)
		}
	}

	return expired, nil
}

func generatePodID(templateName, owner string) string {
	// Use full UUID for database compatibility
	// The database schema requires a valid UUID format
	return uuid.New().String()
}

// generateVMID generates a unique VM ID using cryptographically secure random numbers.
// Proxmox VMIDs must be positive integers. We use the range 100000-999999 to avoid
// conflicts with system VMs (typically < 1000) and provide a large enough space
// to minimize collision probability.
//
// Returns an error on random-source failure so provisioning flows can fail the
// request cleanly instead of crashing the orchestrator process.
func generateVMID() (int, error) {
	// Generate a random number in range [100000, 999999]
	// This gives us 900,000 possible VMIDs
	maxID := big.NewInt(900000)
	n, err := rand.Int(rand.Reader, maxID)
	if err != nil {
		return 0, fmt.Errorf("crypto/rand for VMID: %w", err)
	}
	return 100000 + int(n.Int64()), nil
}

// VMSnapshot represents a snapshot of a VM
type VMSnapshot struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Parent      string    `json:"parent,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

// ListVMSnapshots lists all snapshots for a VM in a pod
func (o *Orchestrator) ListVMSnapshots(ctx context.Context, podID, vmName string) ([]VMSnapshot, error) {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return nil, err
	}

	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}

	if targetVM == nil {
		return nil, fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return nil, fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		pveSnapshots, err := o.proxmox.GetSnapshots(ctx, targetVM.Node, vmid)
		if err != nil {
			return nil, fmt.Errorf("getting snapshots: %w", err)
		}

		snapshots := make([]VMSnapshot, 0, len(pveSnapshots))
		for _, s := range pveSnapshots {
			// Skip "current" which is the live state
			if s.Name == "current" {
				continue
			}
			snapshots = append(snapshots, VMSnapshot{
				Name:        s.Name,
				Description: s.Description,
				Parent:      s.Parent,
			})
		}
		return snapshots, nil

	case models.PlatformCloudStack:
		return nil, fmt.Errorf("CloudStack snapshot listing: %w", ErrNotImplemented)

	default:
		return nil, fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}
}

// CreateVMSnapshot creates a new snapshot for a VM in a pod
func (o *Orchestrator) CreateVMSnapshot(ctx context.Context, podID, vmName, snapshotName, description string, includeRAM bool) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}

	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		if err := o.proxmox.CreateSnapshot(ctx, targetVM.Node, vmid, snapshotName, description, includeRAM); err != nil {
			return fmt.Errorf("creating snapshot: %w", err)
		}
		return nil

	case models.PlatformCloudStack:
		return fmt.Errorf("CloudStack snapshot creation: %w", ErrNotImplemented)

	default:
		return fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}
}

// DeleteVMSnapshot deletes a snapshot from a VM in a pod
func (o *Orchestrator) DeleteVMSnapshot(ctx context.Context, podID, vmName, snapshotName string) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}

	if targetVM == nil {
		return fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		if err := o.proxmox.DeleteSnapshot(ctx, targetVM.Node, vmid, snapshotName); err != nil {
			return fmt.Errorf("deleting snapshot: %w", err)
		}
		return nil

	case models.PlatformCloudStack:
		return fmt.Errorf("CloudStack snapshot deletion: %w", ErrNotImplemented)

	default:
		return fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}
}

// -----------------------------------------------------------------------------
// Lifecycle Manager - Background cleanup of expired pods
// -----------------------------------------------------------------------------

// LifecycleManager handles automatic cleanup of expired pods
type LifecycleManager struct {
	orchestrator  *Orchestrator
	checkInterval time.Duration
	logger        *slog.Logger
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// LifecycleConfig configures the lifecycle manager
type LifecycleConfig struct {
	// CheckInterval is how often to check for expired pods (default: 5 minutes)
	CheckInterval time.Duration
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(orch *Orchestrator, cfg LifecycleConfig, logger *slog.Logger) *LifecycleManager {
	interval := cfg.CheckInterval
	if interval == 0 {
		interval = 5 * time.Minute
	}

	return &LifecycleManager{
		orchestrator:  orch,
		checkInterval: interval,
		logger:        logger,
	}
}

// Start begins the background cleanup goroutine
func (lm *LifecycleManager) Start(ctx context.Context) {
	ctx, lm.cancel = context.WithCancel(ctx)

	lm.wg.Add(1)
	go lm.cleanupLoop(ctx)

	lm.logger.Info("Lifecycle manager started",
		"checkInterval", lm.checkInterval,
	)
}

// Stop stops the lifecycle manager
func (lm *LifecycleManager) Stop() {
	if lm.cancel != nil {
		lm.cancel()
	}
	lm.wg.Wait()
	lm.logger.Info("Lifecycle manager stopped")
}

// cleanupLoop periodically checks for and destroys expired pods
func (lm *LifecycleManager) cleanupLoop(ctx context.Context) {
	defer lm.wg.Done()

	ticker := time.NewTicker(lm.checkInterval)
	defer ticker.Stop()

	// Run once immediately on startup
	lm.cleanupExpiredPods(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lm.cleanupExpiredPods(ctx)
		}
	}
}

// cleanupExpiredPods finds and destroys expired pods
func (lm *LifecycleManager) cleanupExpiredPods(ctx context.Context) {
	expired, err := lm.orchestrator.GetExpiredPods(ctx)
	if err != nil {
		lm.logger.Error("Failed to get expired pods", "error", err)
		return
	}

	if len(expired) == 0 {
		return
	}

	lm.logger.Info("Found expired pods", "count", len(expired))

	for _, pod := range expired {
		lm.logger.Info("Destroying expired pod",
			"podId", pod.ID,
			"owner", pod.Owner,
			"template", pod.LabTemplate,
			"expiredAt", pod.ExpiresAt,
		)

		if err := lm.orchestrator.DestroyPod(ctx, pod.ID); err != nil {
			lm.logger.Error("Failed to destroy expired pod",
				"error", err,
				"podId", pod.ID,
			)
			continue
		}

		lm.logger.Info("Successfully destroyed expired pod", "podId", pod.ID)
	}
}

// ExtendPodExpiration extends the expiration time of a pod
func (o *Orchestrator) ExtendPodExpiration(ctx context.Context, podID string, extension time.Duration) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	if pod.Status == models.PodStatusDestroyed || pod.Status == models.PodStatusDestroying {
		return fmt.Errorf("cannot extend destroyed/destroying pod")
	}

	newExpiration := time.Now().Add(extension)
	if pod.ExpiresAt != nil {
		// Extend from current expiration if it's in the future
		if pod.ExpiresAt.After(time.Now()) {
			newExpiration = pod.ExpiresAt.Add(extension)
		}
	}

	pod.ExpiresAt = &newExpiration

	// Persist to authoritative store
	if o.hasPersistence() {
		if err := o.podRepo.Update(ctx, pod); err != nil {
			return fmt.Errorf("updating pod expiration: %w", err)
		}
	} else {
		// Update in-memory when no database is configured
		o.podsMu.Lock()
		o.pods[podID] = pod
		o.podsMu.Unlock()
	}

	o.logger.Info("Extended pod expiration",
		"podId", podID,
		"newExpiration", newExpiration,
	)

	return nil
}

// SetPodExpiration sets the expiration time of a pod to a specific time
func (o *Orchestrator) SetPodExpiration(ctx context.Context, podID string, expiresAt time.Time) error {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return err
	}

	if pod.Status == models.PodStatusDestroyed || pod.Status == models.PodStatusDestroying {
		return fmt.Errorf("cannot set expiration on destroyed/destroying pod")
	}

	pod.ExpiresAt = &expiresAt

	// Persist to authoritative store
	if o.hasPersistence() {
		if err := o.podRepo.Update(ctx, pod); err != nil {
			return fmt.Errorf("updating pod expiration: %w", err)
		}
	} else {
		// Update in-memory when no database is configured
		o.podsMu.Lock()
		o.pods[podID] = pod
		o.podsMu.Unlock()
	}

	o.logger.Info("Set pod expiration",
		"podId", podID,
		"expiresAt", expiresAt,
	)

	return nil
}

// AddPodForTesting adds a pod directly to the in-memory store for testing purposes.
// This method should only be used in tests.
func (o *Orchestrator) AddPodForTesting(pod *models.Pod) {
	o.podsMu.Lock()
	defer o.podsMu.Unlock()
	if o.pods == nil {
		o.pods = make(map[string]*models.Pod)
	}
	o.pods[pod.ID] = pod
}

// ProxmoxCircuitStats returns the circuit breaker statistics for Proxmox
func (o *Orchestrator) ProxmoxCircuitStats() *proxmox.Client {
	return o.proxmox
}

// CloudStackCircuitStats returns the circuit breaker statistics for CloudStack
func (o *Orchestrator) CloudStackCircuitStats() *cloudstack.Client {
	return o.cloudstack
}

// ExecuteCommand executes a command on a VM using the QEMU guest agent.
// This is used for active verification of checkpoint conditions.
// Requires the qemu-guest-agent to be installed and running in the VM.
func (o *Orchestrator) ExecuteCommand(ctx context.Context, podID, vmName, command string) (string, error) {
	pod, err := o.GetPod(ctx, podID)
	if err != nil {
		return "", err
	}

	// Find the VM
	var targetVM *models.PodVM
	for i := range pod.VMs {
		if pod.VMs[i].Name == vmName {
			targetVM = &pod.VMs[i]
			break
		}
	}
	if targetVM == nil {
		return "", fmt.Errorf("VM not found: %s", vmName)
	}

	switch targetVM.Platform {
	case models.PlatformProxmox:
		if o.proxmox == nil {
			return "", fmt.Errorf("proxmox client not configured")
		}
		vmid, err := parseVMID(targetVM.PlatformID)
		if err != nil {
			return "", fmt.Errorf("parsing VM ID for %s: %w", vmName, err)
		}
		o.logger.Debug("Executing command via guest agent",
			"podId", podID,
			"vmName", vmName,
			"vmid", vmid,
			"command", command,
		)
		return o.proxmox.ExecuteGuestCommand(ctx, targetVM.Node, vmid, command)

	case models.PlatformCloudStack:
		return "", fmt.Errorf("guest command execution for CloudStack: %w", ErrNotImplemented)

	default:
		return "", fmt.Errorf("unsupported platform: %s", targetVM.Platform)
	}
}
