package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/labs"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// -----------------------------------------------------------------------------
// DTOs
// -----------------------------------------------------------------------------

// ListPodsResponse represents the response for listing pods
type ListPodsResponse struct {
	Pods []*models.Pod `json:"pods"`
}

// ListSnapshotsResponse represents the response for listing VM snapshots
type ListSnapshotsResponse struct {
	PodID     string `json:"podId"`
	VMName    string `json:"vmName"`
	Snapshots any    `json:"snapshots"` // runtime type: []orchestrator.VMSnapshot (any avoids circular import)
	Count     int    `json:"count"`
}

// VMActionResponse represents a response for VM actions (start/stop/etc)
type VMActionResponse struct {
	Status   string `json:"status"`
	VMName   string `json:"vmName,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
	PodID    string `json:"podId,omitempty"`
}

// CreatePodRequest represents the request body for creating a pod
type CreatePodRequest struct {
	LabTemplate string `json:"labTemplate" validate:"required" example:"linux-basics-101"`
	Owner       string `json:"owner" validate:"required" example:"jsmith"`
}

// AsyncPodResponse represents the response for async pod creation
type AsyncPodResponse struct {
	PodID     string `json:"podId" example:"abc123-def456"`
	RequestID string `json:"requestId" example:"req-789xyz"`
	Status    string `json:"status" example:"provisioning"`
	Subject   string `json:"subject" example:"labs.pods.abc123.provision"`
	Message   string `json:"message" example:"Pod provisioning started"`
}

// TopologyNetworkSegment represents a network segment in the topology
type TopologyNetworkSegment struct {
	Name    string `json:"name"`
	VLAN    int    `json:"vlan"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway,omitempty"`
	DHCP    bool   `json:"dhcp"`
}

// TopologyVMNetwork represents a VM's network connection
type TopologyVMNetwork struct {
	Segment string `json:"segment"`
	IP      string `json:"ip,omitempty"`
}

// TopologyVMResources represents VM resource allocation
type TopologyVMResources struct {
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
	Disk   int `json:"disk,omitempty"`
}

// TopologyVM represents a VM in the topology visualization
type TopologyVM struct {
	Name       string              `json:"name"`
	PlatformID string              `json:"platformId"`
	Status     string              `json:"status"`
	IPAddress  string              `json:"ipAddress,omitempty"`
	Template   string              `json:"template"`
	Networks   []TopologyVMNetwork `json:"networks"`
	Resources  TopologyVMResources `json:"resources"`
}

// TopologyResponse represents the topology data for a pod
type TopologyResponse struct {
	PodID       string                   `json:"podId"`
	LabTemplate string                   `json:"labTemplate"`
	Segments    []TopologyNetworkSegment `json:"segments"`
	VMs         []TopologyVM             `json:"vms"`
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// tenantFromCtx is a nil-safe wrapper for custommiddleware.TenantFromContext.
func tenantFromCtx(ctx context.Context) *models.TenantContext {
	tc, _ := custommiddleware.TenantFromContext(ctx)
	return tc
}

// resolveTemplate looks up a lab template by name first, then falls back to
// lookup by ID. Returns the matching record or an error. If no record is found
// by either method, both return values are nil.
func resolveTemplate(ctx context.Context, repo repositories.LabTemplateRepository, nameOrID string) (*models.LabTemplateRecord, error) {
	record, err := repo.GetByName(ctx, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("lookup by name: %w", err)
	}
	if record != nil {
		return record, nil
	}

	// Name lookup returned nothing; try by ID
	record, err = repo.GetByID(ctx, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("lookup by ID: %w", err)
	}
	return record, nil
}

// resolveTemplateWithAccess wraps resolveTemplate with a visibility check.
// Returns nil (not found) when the template exists but the caller lacks access,
// preventing information disclosure of org-private templates.
func resolveTemplateWithAccess(ctx context.Context, repo repositories.LabTemplateRepository, nameOrID string) (*models.LabTemplateRecord, error) {
	record, err := resolveTemplate(ctx, repo, nameOrID)
	if err != nil || record == nil {
		return record, err
	}
	user, _ := auth.UserFromContext(ctx)
	// TenantContext is extracted by the caller and lives on ctx — labs.CanAccessLabWithTenant
	// handles the nil-tc case gracefully (falls back to user.DefaultOrganizationID).
	tc := tenantFromCtx(ctx)
	if !labs.CanAccessLabWithTenant(user, tc, record) {
		return nil, nil // mask as not-found
	}
	return record, nil
}

// resolveCreatePodOpts extracts multi-tenancy org/team context for pod creation.
// Prefers the tenant middleware's TenantContext (set via X-Organization header
// or URL path); falls back to the authenticated user's default organization.
func resolveCreatePodOpts(ctx context.Context) orchestrator.CreatePodOpts {
	var opts orchestrator.CreatePodOpts
	if tc, ok := custommiddleware.TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
		opts.OrganizationID = &tc.Organization.ID
		if tc.Team != nil {
			opts.TeamID = &tc.Team.ID
		}
	} else if user, ok := auth.UserFromContext(ctx); ok && user != nil && user.DefaultOrganizationID != "" {
		opts.OrganizationID = &user.DefaultOrganizationID
	}
	return opts
}

// canAccessPod checks if a user has access to a pod based on organization.
// Admins/instructors can access all pods.
func canAccessPod(user *auth.User, pod *models.Pod) bool {
	// Admins/instructors can access all pods
	if user != nil && serverutil.IsAdminOrInstructor(user) {
		return true
	}

	// Pod owner can always access
	if user != nil && (pod.OwnerID == user.ID || pod.Owner == user.Name) {
		return true
	}

	// Check organization match
	if pod.OrganizationID != nil && user != nil && user.DefaultOrganizationID == *pod.OrganizationID {
		return true
	}

	// Pods without organization are only accessible by their owner (checked above) or admins
	if pod.OrganizationID == nil {
		return false
	}

	return false
}

// canModifyPod checks if a user can modify/delete a pod.
// Admins/instructors can modify all pods.
func canModifyPod(user *auth.User, pod *models.Pod) bool {
	// No user context - cannot modify
	if user == nil {
		return false
	}

	// Admins/instructors can modify all pods
	if serverutil.IsAdminOrInstructor(user) {
		return true
	}

	// Pod owner can modify their own pods
	if pod.OwnerID == user.ID || pod.Owner == user.Name {
		return true
	}

	return false
}

// -----------------------------------------------------------------------------
// Pod Handlers
// -----------------------------------------------------------------------------

// handleListPods lists all pods for a user
// @Summary List Pods
// @Description Get all pods, optionally filtered by owner
// @Tags pods
// @Produce json
// @Param owner query string false "Filter by owner username"
// @Param platform query string false "Filter by platform (proxmox, cloudstack)"
// @Param all query boolean false "Show all pods (admin only, bypasses org filtering)"
// @Success 200 {object} ListPodsResponse
// @Failure 500 {object} serverutil.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /pods [get]
func (m *Manager) handleListPods() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user from context for org filtering
		user, _ := auth.UserFromContext(r.Context())

		owner := r.URL.Query().Get("owner")
		platform := r.URL.Query().Get("platform")
		showAll := r.URL.Query().Get("all") == "true"

		// Build filter with organization context
		filter := orchestrator.PodFilter{
			OwnerID:  owner,
			Platform: platform,
		}

		// Apply org-scoped filtering — prefer tenant context (X-Organization
		// header), fall back to user's default org. Admins with ?all=true bypass.
		tc := tenantFromCtx(r.Context())
		if showAll && user != nil && serverutil.IsAdminOrInstructor(user) {
			// Admin requesting all pods - no org filter
		} else if tc != nil && tc.Organization != nil {
			filter.OrganizationID = tc.Organization.ID
		} else if user != nil && user.DefaultOrganizationID != "" {
			filter.OrganizationID = user.DefaultOrganizationID
		}

		pods, err := m.orchestrator.ListPodsWithFilter(r.Context(), filter)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "list pods")
			return
		}
		m.responder.JSONResponse(w, http.StatusOK, ListPodsResponse{Pods: pods})
	}
}

// handleCreatePod creates a new pod from a lab template
// @Summary Create Pod
// @Description Create a new pod from a lab template
// @Tags pods
// @Accept json
// @Produce json
// @Param request body CreatePodRequest true "Pod creation request"
// @Success 201 {object} models.Pod
// @Failure 400 {object} serverutil.ErrorResponse "Invalid request"
// @Failure 404 {object} serverutil.ErrorResponse "Template not found"
// @Failure 500 {object} serverutil.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /pods [post]
func (m *Manager) handleCreatePod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, validationErrors := serverutil.DecodeAndValidate[CreatePodRequest](r)
		if validationErrors != nil {
			m.responder.JSONResponse(w, http.StatusBadRequest, serverutil.ValidationErrorResponse{
				Error:   "validation failed",
				Details: validationErrors,
			})
			return
		}

		// Load template from database
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pod.errors.labTemplateRepoNotConfigured", nil)
			return
		}

		ctx := r.Context()

		// Resolve template by name or ID (with visibility check)
		record, err := resolveTemplateWithAccess(ctx, m.labTemplateRepo, req.LabTemplate)
		if err != nil {
			m.logger.Error("Failed to load template", "error", err, "template", req.LabTemplate)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pod.errors.loadTemplateFailed", nil)
			return
		}

		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.labTemplateNotFound", nil)
			return
		}

		if !record.IsActive {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pod.errors.labTemplateInactive", nil)
			return
		}

		// Convert database record to LabTemplate
		template, err := record.ToLabTemplate()
		if err != nil {
			m.logger.Error("Failed to convert template record", "error", err, "template", req.LabTemplate)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pod.errors.parseTemplateFailed", nil)
			return
		}

		// Get or create user for the owner
		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pod.errors.userRepoNotConfigured", nil)
			return
		}

		user, err := m.userRepo.GetOrCreateByUsername(ctx, req.Owner)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "get or create user")
			return
		}

		// Resolve org/team context for multi-tenancy scoping
		podOpts := resolveCreatePodOpts(ctx)

		// Create the pod via orchestrator with proper UUIDs
		pod, err := m.orchestrator.CreatePod(ctx, template, record.ID, user.ID, user.Username, podOpts)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "create pod")
			return
		}

		m.logger.Info("Pod created", "podId", pod.ID, "template", req.LabTemplate, "owner", req.Owner)
		m.responder.JSONResponse(w, http.StatusCreated, pod)
	}
}

// handleCreatePodAsync creates a new pod asynchronously from a lab template
// @Summary Create Pod (Async)
// @Description Create a new pod asynchronously from a lab template. Returns immediately with pod ID and NATS subject for status updates.
// @Tags pods
// @Accept json
// @Produce json
// @Param request body CreatePodRequest true "Pod creation request"
// @Success 202 {object} AsyncPodResponse
// @Failure 400 {object} serverutil.ErrorResponse "Invalid request"
// @Failure 404 {object} serverutil.ErrorResponse "Template not found"
// @Failure 503 {object} serverutil.ErrorResponse "Async provisioning not available"
// @Security BearerAuth
// @Router /pods/async [post]
func (m *Manager) handleCreatePodAsync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if async provisioner is available
		if m.asyncProvisioner == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pod.errors.asyncProvisioningNotConfigured", nil)
			return
		}

		req, validationErrors := serverutil.DecodeAndValidate[CreatePodRequest](r)
		if validationErrors != nil {
			m.responder.JSONResponse(w, http.StatusBadRequest, serverutil.ValidationErrorResponse{
				Error:   "validation failed",
				Details: validationErrors,
			})
			return
		}

		// Load template from database
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pod.errors.labTemplateRepoNotConfigured", nil)
			return
		}

		ctx := r.Context()

		// Resolve template by name or ID (with visibility check)
		record, err := resolveTemplateWithAccess(ctx, m.labTemplateRepo, req.LabTemplate)
		if err != nil {
			m.logger.Error("Failed to load template", "error", err, "template", req.LabTemplate)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pod.errors.loadTemplateFailed", nil)
			return
		}

		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.labTemplateNotFound", nil)
			return
		}

		if !record.IsActive {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pod.errors.labTemplateInactive", nil)
			return
		}

		// Convert database record to LabTemplate
		template, err := record.ToLabTemplate()
		if err != nil {
			m.logger.Error("Failed to convert template record", "error", err, "template", req.LabTemplate)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pod.errors.parseTemplateFailed", nil)
			return
		}

		// Get or create user for the owner - prefer authenticated user if available
		var ownerID, ownerName string
		if user, ok := auth.UserFromContext(ctx); ok && user != nil {
			ownerID = user.ID
			ownerName = user.Name
		} else if m.userRepo != nil {
			user, err := m.userRepo.GetOrCreateByUsername(ctx, req.Owner)
			if err != nil {
				m.responder.SafeErrorResponse(w, err, "get or create user")
				return
			}
			ownerID = user.ID
			ownerName = user.Username
		} else {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pod.errors.userRepoNotConfigured", nil)
			return
		}

		// Resolve org/team context for multi-tenancy scoping
		podOpts := resolveCreatePodOpts(ctx)

		// Create provision request
		provisionReq := orchestrator.PodProvisionRequest{
			TemplateID:     record.ID,
			TemplateName:   record.Name,
			Template:       template,
			OwnerID:        ownerID,
			OwnerName:      ownerName,
			OrganizationID: podOpts.OrganizationID,
			TeamID:         podOpts.TeamID,
		}

		// Start async provisioning
		podID, err := m.asyncProvisioner.ProvisionAsync(ctx, provisionReq)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "start async provisioning")
			return
		}

		// Return immediately with pod ID and subscription info
		response := AsyncPodResponse{
			PodID:     podID,
			RequestID: provisionReq.RequestID,
			Status:    "provisioning",
			Subject:   m.asyncProvisioner.ProvisioningEventSubject(podID),
			Message:   "Pod provisioning started. Subscribe to the NATS subject for status updates.",
		}

		m.logger.Info("Async pod provisioning started", "podId", podID, "template", req.LabTemplate, "owner", req.Owner)
		m.responder.JSONResponse(w, http.StatusAccepted, response)
	}
}

// handleGetPod gets a specific pod by ID
// @Summary Get Pod
// @Description Get detailed information about a specific pod
// @Tags pods
// @Produce json
// @Param podID path string true "Pod ID"
// @Success 200 {object} models.Pod
// @Failure 403 {object} serverutil.ErrorResponse "Access denied"
// @Failure 404 {object} serverutil.ErrorResponse "Pod not found"
// @Security BearerAuth
// @Router /pods/{podID} [get]
func (m *Manager) handleGetPod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")

		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}

		// Check access based on organization
		user, _ := auth.UserFromContext(r.Context())
		if !canAccessPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDeniedToPod", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, pod)
	}
}

// handleDeletePod deletes a pod
// @Summary Delete Pod
// @Description Destroy a pod and its resources
// @Tags pods
// @Produce json
// @Param podID path string true "Pod ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} serverutil.ErrorResponse "Access denied"
// @Failure 404 {object} serverutil.ErrorResponse "Pod not found"
// @Failure 500 {object} serverutil.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /pods/{podID} [delete]
func (m *Manager) handleDeletePod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")

		// Get pod to check access
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}

		// Check access - only owner or admin can delete
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.deletePermissionDenied", nil)
			return
		}

		if err := m.orchestrator.DestroyPod(r.Context(), podID); err != nil {
			m.responder.SafeErrorResponse(w, err, "destroy pod")
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, serverutil.NewStatusResponse("destroyed"))
	}
}

func (m *Manager) handleResetPod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")

		var req struct {
			Snapshot string `json:"snapshot"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Get pod to reset all VMs
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}

		// Check ownership
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDenied", nil)
			return
		}

		// Reset each VM in the pod
		for _, vm := range pod.VMs {
			if err := m.orchestrator.ResetPodVM(r.Context(), podID, vm.Name, req.Snapshot); err != nil {
				m.responder.SafeErrorResponse(w, err, "reset pod VM")
				return
			}
		}

		m.responder.JSONResponse(w, http.StatusOK, serverutil.NewStatusResponse("reset"))
	}
}

func (m *Manager) handleResetVM() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		var req struct {
			Snapshot string `json:"snapshot"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Validate snapshot name is provided
		if req.Snapshot == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pod.errors.snapshotNameRequired", nil)
			return
		}

		// Check ownership
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDenied", nil)
			return
		}

		// Reset the specific VM via orchestrator
		start := time.Now()
		err = m.orchestrator.ResetPodVM(r.Context(), podID, vmName, req.Snapshot)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("reset", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "reset VM")
			return
		}

		m.logger.Info("VM reset successful", "podId", podID, "vmName", vmName, "snapshot", req.Snapshot)
		m.responder.JSONResponse(w, http.StatusOK, VMActionResponse{
			Status:   "reset",
			VMName:   vmName,
			Snapshot: req.Snapshot,
		})
	}
}

// handleListSnapshots lists all snapshots for a VM
func (m *Manager) handleListSnapshots() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		// Check ownership
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !canAccessPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDenied", nil)
			return
		}

		snapshots, err := m.orchestrator.ListVMSnapshots(r.Context(), podID, vmName)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "list VM snapshots")
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, ListSnapshotsResponse{
			PodID:     podID,
			VMName:    vmName,
			Snapshots: snapshots,
			Count:     len(snapshots),
		})
	}
}

// handleCreateSnapshot creates a new snapshot for a VM
func (m *Manager) handleCreateSnapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		// Check ownership
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDenied", nil)
			return
		}

		var req struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			IncludeRAM  bool   `json:"includeRam,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Name == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pod.errors.snapshotNameRequired", nil)
			return
		}

		start := time.Now()
		err = m.orchestrator.CreateVMSnapshot(r.Context(), podID, vmName, req.Name, req.Description, req.IncludeRAM)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("create_snapshot", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "create VM snapshot")
			return
		}

		m.logger.Info("Snapshot created", "podId", podID, "vmName", vmName, "snapshot", req.Name)
		m.responder.JSONResponse(w, http.StatusCreated, VMActionResponse{
			Status:   "created",
			PodID:    podID,
			VMName:   vmName,
			Snapshot: req.Name,
		})
	}
}

// handleDeleteSnapshot deletes a snapshot from a VM
func (m *Manager) handleDeleteSnapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")
		snapshotName := chi.URLParam(r, "snapshotName")

		// Check ownership
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDenied", nil)
			return
		}

		start := time.Now()
		err = m.orchestrator.DeleteVMSnapshot(r.Context(), podID, vmName, snapshotName)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("delete_snapshot", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "delete VM snapshot")
			return
		}

		m.logger.Info("Snapshot deleted", "podId", podID, "vmName", vmName, "snapshot", snapshotName)
		m.responder.JSONResponse(w, http.StatusOK, VMActionResponse{
			Status:   "deleted",
			PodID:    podID,
			VMName:   vmName,
			Snapshot: snapshotName,
		})
	}
}

// handleStartPod starts all VMs in a stopped pod
func (m *Manager) handleStartPod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")

		// Check access - only owner or admin can start
		existingPod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, existingPod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.startPermissionDenied", nil)
			return
		}

		start := time.Now()
		pod, err := m.orchestrator.StartPod(r.Context(), podID)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("start_pod", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "start pod")
			return
		}

		m.logger.Info("Pod started", "podId", podID)
		m.responder.JSONResponse(w, http.StatusOK, pod)
	}
}

// handleStopPod stops all VMs in a running pod without destroying it
func (m *Manager) handleStopPod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")

		// Check access - only owner or admin can stop
		existingPod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !canModifyPod(user, existingPod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.stopPermissionDenied", nil)
			return
		}

		start := time.Now()
		pod, err := m.orchestrator.StopPod(r.Context(), podID)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("stop_pod", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "stop pod")
			return
		}

		m.logger.Info("Pod stopped", "podId", podID)
		m.responder.JSONResponse(w, http.StatusOK, pod)
	}
}

// checkVMOwnership verifies the requesting user can modify the pod's VMs.
// Returns the pod on success, or writes an error response and returns nil.
func (m *Manager) checkVMOwnership(w http.ResponseWriter, r *http.Request, podID string) *models.Pod {
	pod, err := m.orchestrator.GetPod(r.Context(), podID)
	if err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
		return nil
	}
	user, _ := auth.UserFromContext(r.Context())
	if !canModifyPod(user, pod) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDenied", nil)
		return nil
	}
	return pod
}

// handleStartVM starts a single VM in a pod
// POST /api/v1/pods/{podID}/vms/{vmName}/start
func (m *Manager) handleStartVM() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		if m.checkVMOwnership(w, r, podID) == nil {
			return
		}

		start := time.Now()
		err := m.orchestrator.StartVM(r.Context(), podID, vmName)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("start", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "start VM")
			return
		}

		m.logger.Info("VM started", "podId", podID, "vmName", vmName)
		m.responder.JSONResponse(w, http.StatusOK, VMActionResponse{Status: "started", VMName: vmName})
	}
}

// handleStopVM stops a single VM in a pod
// POST /api/v1/pods/{podID}/vms/{vmName}/stop
func (m *Manager) handleStopVM() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		if m.checkVMOwnership(w, r, podID) == nil {
			return
		}

		start := time.Now()
		err := m.orchestrator.StopVM(r.Context(), podID, vmName)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("stop", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "stop VM")
			return
		}

		m.logger.Info("VM stopped", "podId", podID, "vmName", vmName)
		m.responder.JSONResponse(w, http.StatusOK, VMActionResponse{Status: "stopped", VMName: vmName})
	}
}

// handleSuspendVM suspends a single VM in a pod
// POST /api/v1/pods/{podID}/vms/{vmName}/suspend
func (m *Manager) handleSuspendVM() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		if m.checkVMOwnership(w, r, podID) == nil {
			return
		}

		start := time.Now()
		err := m.orchestrator.SuspendVM(r.Context(), podID, vmName)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("suspend", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "suspend VM")
			return
		}

		m.logger.Info("VM suspended", "podId", podID, "vmName", vmName)
		m.responder.JSONResponse(w, http.StatusOK, VMActionResponse{Status: "suspended", VMName: vmName})
	}
}

// handleResumeVM resumes a suspended VM in a pod
// POST /api/v1/pods/{podID}/vms/{vmName}/resume
func (m *Manager) handleResumeVM() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		if m.checkVMOwnership(w, r, podID) == nil {
			return
		}

		start := time.Now()
		err := m.orchestrator.ResumeVM(r.Context(), podID, vmName)
		if m.metrics != nil {
			m.metrics.VMOperationDuration("resume", time.Since(start))
		}
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "resume VM")
			return
		}

		m.logger.Info("VM resumed", "podId", podID, "vmName", vmName)
		m.responder.JSONResponse(w, http.StatusOK, VMActionResponse{Status: "running", VMName: vmName})
	}
}

// handleGetTopology returns network topology data for a pod
// @Summary Get Pod Topology
// @Description Get network topology visualization data for a pod
// @Tags pods
// @Produce json
// @Param podID path string true "Pod ID"
// @Success 200 {object} TopologyResponse
// @Failure 404 {object} serverutil.ErrorResponse "Pod or template not found"
// @Failure 500 {object} serverutil.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /pods/{podID}/topology [get]
func (m *Manager) handleGetTopology() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		ctx := r.Context()

		// Get the pod
		pod, err := m.orchestrator.GetPod(ctx, podID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pod.errors.notFound", nil)
			return
		}

		// Check access
		user, _ := auth.UserFromContext(ctx)
		if !canAccessPod(user, pod) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pod.errors.accessDeniedToPod", nil)
			return
		}

		// Get the lab template to extract network topology
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pod.errors.labTemplateRepoNotConfigured", nil)
			return
		}

		// Try to get template by pod's template ID first, then by name
		var template *models.LabTemplate
		if pod.LabTemplateID != "" {
			record, err := m.labTemplateRepo.GetByID(ctx, pod.LabTemplateID)
			if err == nil && record != nil {
				template, _ = record.ToLabTemplate()
			}
		}

		if template == nil {
			record, err := m.labTemplateRepo.GetByName(ctx, pod.LabTemplate)
			if err == nil && record != nil {
				template, _ = record.ToLabTemplate()
			}
		}

		// Build topology response
		response := TopologyResponse{
			PodID:       pod.ID,
			LabTemplate: pod.LabTemplate,
			Segments:    []TopologyNetworkSegment{},
			VMs:         []TopologyVM{},
		}

		// If we have the template, use its network spec
		if template != nil && template.Spec.Network.Segments != nil {
			for _, seg := range template.Spec.Network.Segments {
				response.Segments = append(response.Segments, TopologyNetworkSegment{
					Name:    seg.Name,
					VLAN:    seg.VLAN,
					Subnet:  seg.Subnet,
					Gateway: seg.Gateway,
					DHCP:    seg.DHCP,
				})
			}

			// Build VM topology from template spec and current pod state
			vmStatusMap := make(map[string]*models.PodVM)
			for i := range pod.VMs {
				vmStatusMap[pod.VMs[i].Name] = &pod.VMs[i]
			}

			for _, vmSpec := range template.Spec.VMs {
				topologyVM := TopologyVM{
					Name:     vmSpec.Name,
					Template: vmSpec.Template,
					Resources: TopologyVMResources{
						CPU:    vmSpec.Resources.CPU,
						Memory: vmSpec.Resources.Memory,
						Disk:   vmSpec.Resources.Disk,
					},
					Networks: []TopologyVMNetwork{},
				}

				// Get current status from pod
				if podVM, ok := vmStatusMap[vmSpec.Name]; ok {
					topologyVM.PlatformID = podVM.PlatformID
					topologyVM.Status = podVM.Status
					topologyVM.IPAddress = podVM.IPAddress
				} else {
					topologyVM.Status = "unknown"
				}

				// Add network connections from template
				for _, net := range vmSpec.Networks {
					topologyVM.Networks = append(topologyVM.Networks, TopologyVMNetwork{
						Segment: net.Segment,
						IP:      net.IP,
					})
				}

				response.VMs = append(response.VMs, topologyVM)
			}
		} else {
			// Fallback: build topology from pod state only (limited info)
			// Create a default segment based on IPs we see
			defaultSegment := TopologyNetworkSegment{
				Name:   "default",
				VLAN:   1,
				Subnet: "10.0.0.0/24",
				DHCP:   true,
			}
			response.Segments = append(response.Segments, defaultSegment)

			for _, vm := range pod.VMs {
				topologyVM := TopologyVM{
					Name:       vm.Name,
					PlatformID: vm.PlatformID,
					Status:     vm.Status,
					IPAddress:  vm.IPAddress,
					Template:   "unknown",
					Resources:  TopologyVMResources{CPU: 1, Memory: 1024},
					Networks: []TopologyVMNetwork{
						{Segment: "default", IP: vm.IPAddress},
					},
				}
				response.VMs = append(response.VMs, topologyVM)
			}
		}

		m.responder.JSONResponse(w, http.StatusOK, response)
	}
}
