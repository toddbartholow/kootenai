package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
	"github.com/toddbartholow/kootenai/api/internal/session"
)

// -----------------------------------------------------------------------------
// Session DTOs
// -----------------------------------------------------------------------------

// ListSessionsResponse represents the response for listing sessions
type ListSessionsResponse struct {
	Sessions   []*models.Session          `json:"sessions"`
	Pagination *serverutil.PaginationMeta `json:"pagination,omitempty"`
}

// SessionResponse represents a single session response
type SessionResponse struct {
	Session *models.Session `json:"session"`
}

// CreateSessionResponse represents the response for creating a session
type CreateSessionResponse struct {
	Session      *models.Session              `json:"session"`
	Checkpoints  []*models.CheckpointProgress `json:"checkpoints"`
	Achievements []*models.UserAchievement    `json:"achievements,omitempty"`
}

// SessionSubmitResponse represents the response for submitting a session
type SessionSubmitResponse struct {
	SessionID    string                    `json:"sessionId"`
	Passed       bool                      `json:"passed"`
	EarnedPoints int                       `json:"earnedPoints"`
	MaxPoints    int                       `json:"maxPoints"`
	Percentage   float64                   `json:"percentage"`
	GradeSynced  bool                      `json:"gradeSynced,omitempty"`
	Achievements []*models.UserAchievement `json:"achievements,omitempty"`
}

// SessionStartedResponse represents the response for starting a new session
type SessionStartedResponse struct {
	SessionID   string `json:"sessionId"`
	Status      string `json:"status"`
	MaxPoints   int    `json:"maxPoints"`
	AgentStatus any    `json:"agentStatus,omitempty"`
}

// SessionProgressResponse represents session progress with checkpoints
type SessionProgressResponse struct {
	SessionID    string                  `json:"sessionId"`
	EarnedPoints int                     `json:"earnedPoints"`
	MaxPoints    int                     `json:"maxPoints"`
	Percentage   float64                 `json:"percentage"`
	Passed       bool                    `json:"passed"`
	Checkpoints  []CheckpointProgressDTO `json:"checkpoints"`
}

// CheckpointProgressDTO represents a single checkpoint's progress for API responses
type CheckpointProgressDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Score    int    `json:"score"`
	MaxScore int    `json:"maxScore"`
}

// SessionSubmitDetailResponse represents the detailed response for submitting a session
type SessionSubmitDetailResponse struct {
	SessionID           string                    `json:"sessionId"`
	Status              string                    `json:"status"`
	EarnedPoints        int                       `json:"earnedPoints"`
	MaxPoints           int                       `json:"maxPoints"`
	Percentage          float64                   `json:"percentage"`
	Passed              bool                      `json:"passed"`
	PassThreshold       int                       `json:"passThreshold"`
	Checkpoints         any                       `json:"checkpoints"`
	SubmittedAt         string                    `json:"submittedAt"`
	Achievements        []*models.UserAchievement `json:"achievements,omitempty"`
	AchievementsPending bool                      `json:"achievementsPending,omitempty"`
}

// CheckpointsResponse represents checkpoints for a session
type CheckpointsResponse struct {
	SessionID   string `json:"sessionId"`
	Checkpoints any    `json:"checkpoints"` // runtime type: map[string]*checkpoint.CheckpointProgress (any avoids circular import)
}

// GradeResponse represents grade information
type GradeResponse struct {
	SessionID    string                      `json:"sessionId"`
	EarnedPoints int                         `json:"earnedPoints"`
	MaxPoints    int                         `json:"maxPoints"`
	Percentage   float64                     `json:"percentage"`
	Passed       bool                        `json:"passed"`
	Checkpoints  []models.CheckpointProgress `json:"checkpoints,omitempty"`
}

// templateCheckpointShape is the subset of a LabTemplate.Checkpoints JSON
// object that session handlers care about when building a checkpoint list
// from the stored template. Extra keys in the source JSON are ignored.
type templateCheckpointShape struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Points      int    `json:"points"`
}

// cleanupStaleSessionsResponse is the JSON body returned by
// POST /sessions/cleanup-stale. Typed rather than an inline
// map[string]any so the wire contract is obvious from the code.
// Cleaned is int64 to match the repo's return type (SQL RowsAffected).
type cleanupStaleSessionsResponse struct {
	Cleaned int64  `json:"cleaned"`
	MaxAge  string `json:"maxAge"`
}

// -----------------------------------------------------------------------------
// Helper Functions
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

// canAccessSession checks if a user has access to view a session
// Users can access their own sessions, or any session if they're an admin
// Organization members can see sessions within their organization
func canAccessSession(user *auth.User, session *models.Session) bool {
	if user == nil {
		return false
	}
	// Admins can access any session
	if serverutil.IsAdminOrInstructor(user) {
		return true
	}
	// Users can always access their own sessions
	if session.UserID == user.ID {
		return true
	}
	// Organization members can see sessions within their org
	if session.OrganizationID != nil && user.DefaultOrganizationID != "" &&
		*session.OrganizationID == user.DefaultOrganizationID {
		return true
	}
	return false
}

// canModifySession checks if a user can modify/delete/end a session
// Only the session owner or an admin can modify sessions
func canModifySession(user *auth.User, session *models.Session) bool {
	if user == nil {
		return false
	}
	// Admins can modify any session
	if serverutil.IsAdminOrInstructor(user) {
		return true
	}
	// Only the session owner can modify their own session
	return session.UserID == user.ID
}

// -----------------------------------------------------------------------------
// Session Handlers
// -----------------------------------------------------------------------------

// handleListSessions godoc
// @Summary List Lab Sessions
// @Description List lab sessions visible to the authenticated user
// @Tags sessions
// @Accept json
// @Produce json
// @Success 200 {object} ListSessionsResponse
// @Failure 500 {object} serverutil.ErrorResponse
// @Security BearerAuth
// @Router /sessions [get]
func (m *Manager) handleListSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.sessionRepo == nil {
			m.responder.JSONResponse(w, http.StatusOK, ListSessionsResponse{
				Sessions: []*models.Session{},
			})
			return
		}

		// Get authenticated user for filtering
		user, _ := auth.UserFromContext(r.Context())

		// Parse query parameters
		userID := r.URL.Query().Get("userId")
		activeOnly := r.URL.Query().Get("active") == "true"

		filter := repositories.SessionFilter{
			UserID: userID,
		}
		if activeOnly {
			active := true
			filter.Active = &active
		}

		// Apply organization filtering — prefer tenant context, fall back to
		// user's default org.
		tc := tenantFromCtx(r.Context())
		resolvedOrgID := ""
		if tc != nil && tc.Organization != nil {
			resolvedOrgID = tc.Organization.ID
		} else if user != nil {
			resolvedOrgID = user.DefaultOrganizationID
		}

		if user != nil && !serverutil.IsAdminOrInstructor(user) {
			// Non-admins can only see their own sessions or sessions in their org
			if userID == "" {
				filter.UserID = user.ID
			} else if userID != user.ID {
				if resolvedOrgID != "" {
					filter.OrganizationID = resolvedOrgID
				} else {
					filter.UserID = user.ID
				}
			}
		} else if resolvedOrgID != "" && userID == "" {
			// Admins with an org see their org's sessions by default
			filter.OrganizationID = resolvedOrgID
		}

		sessions, err := m.sessionRepo.List(r.Context(), filter)
		if err != nil {
			m.logger.Error("Failed to list sessions", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.listFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, ListSessionsResponse{
			Sessions: sessions,
		})
	}
}

// createSessionRequest mirrors the JSON body accepted by handleCreateSession.
type createSessionRequest struct {
	PodID              string `json:"podId"`
	UserID             string `json:"userId"`
	LabTemplate        string `json:"labTemplate"`
	CanvasAssignmentID string `json:"canvasAssignmentId,omitempty"`
	CanvasCourseID     string `json:"canvasCourseId,omitempty"`
	CanvasUserID       string `json:"canvasUserId,omitempty"`
	// Pathway context — optional, links session to pathway progress.
	EnrollmentID string `json:"enrollmentId,omitempty"`
	ModuleID     string `json:"moduleId,omitempty"`
}

// handleCreateSession godoc
// @Summary Create Lab Session
// @Description Start a new lab session for the authenticated user
// @Tags sessions
// @Accept json
// @Produce json
// @Param request body createSessionRequest true "Pod, user and lab template to start a session for"
// @Success 201 {object} SessionStartedResponse
// @Failure 400 {object} serverutil.ErrorResponse "Invalid request"
// @Failure 403 {object} serverutil.ErrorResponse "Access denied"
// @Failure 409 {object} serverutil.ErrorResponse "An active session already exists for this pod"
// @Failure 500 {object} serverutil.ErrorResponse
// @Security BearerAuth
// @Router /sessions [post]
//
// handleCreateSession composes per-step helpers. Each helper returns a `bool`
// indicating "continue" — false means a response has already been written and
// the caller should return immediately.
func (m *Manager) handleCreateSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		req, ok := m.parseCreateSessionRequest(w, r)
		if !ok {
			return
		}

		if !m.ensureNoActiveSessionForPod(w, r, req.PodID, req.UserID) {
			return
		}

		// IDOR: bind the new session to the authenticated user.
		user, _ := auth.UserFromContext(ctx)
		if user != nil {
			req.UserID = user.ID
		}

		maxPoints, labTemplateID := m.resolveLabTemplateInfo(ctx, req.LabTemplate)

		enrollmentID, moduleID, ok := m.resolvePathwayContext(w, r, req)
		if !ok {
			return
		}

		sessionID := uuid.New().String()

		if !m.persistSession(w, r, sessionID, req, labTemplateID, maxPoints, enrollmentID, moduleID, user) {
			return
		}

		if m.metrics != nil {
			m.metrics.SessionCreated()
		}

		m.registerActiveChecks(ctx, sessionID, req.PodID, req.LabTemplate)
		agentStatus := m.preflightAgentsAndStartTampering(ctx, sessionID, req.PodID)

		m.logger.Info("Session created", "sessionId", sessionID, "podId", req.PodID, "userId", req.UserID)
		m.responder.JSONResponse(w, http.StatusCreated, SessionStartedResponse{
			SessionID:   sessionID,
			Status:      "started",
			MaxPoints:   maxPoints,
			AgentStatus: agentStatus,
		})
	}
}

// parseCreateSessionRequest decodes + validates required fields. Returns
// ok=false after writing an error response.
func (m *Manager) parseCreateSessionRequest(w http.ResponseWriter, r *http.Request) (*createSessionRequest, bool) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return nil, false
	}
	switch {
	case req.PodID == "":
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "session.errors.podIdRequired", nil)
		return nil, false
	case req.UserID == "":
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "session.errors.userIdRequired", nil)
		return nil, false
	case req.LabTemplate == "":
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "session.errors.labTemplateRequired", nil)
		return nil, false
	}
	return &req, true
}

// ensureNoActiveSessionForPod rejects with 409 if another active session
// already exists for this pod. Returns ok=false after writing the response.
func (m *Manager) ensureNoActiveSessionForPod(w http.ResponseWriter, r *http.Request, podID, userID string) bool {
	if m.sessionRepo == nil {
		return true
	}
	ctx := r.Context()
	existing, err := m.sessionRepo.GetByPodID(ctx, podID)
	if err != nil {
		m.logger.Error("Failed to check existing sessions", "error", err, "podId", podID)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "session.errors.checkExistingFailed", nil)
		return false
	}
	for _, sess := range existing {
		if sess.EndedAt == nil {
			m.logger.Warn("Attempted to create duplicate session for pod",
				"podId", podID,
				"existingSessionId", sess.ID,
				"userId", userID,
			)
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusConflict, "session.errors.activeExistsForPod", nil)
			return false
		}
	}
	return true
}

// resolveLabTemplateInfo looks up MaxPoints / canonical template ID. Errors
// are logged but not returned — sessions can be created without a template
// (template lookup is best-effort).
func (m *Manager) resolveLabTemplateInfo(ctx context.Context, templateRef string) (maxPoints int, labTemplateID string) {
	if m.labTemplateRepo != nil {
		record, err := resolveTemplate(ctx, m.labTemplateRepo, templateRef)
		if err != nil {
			m.logger.Warn("Failed to load template for session", "error", err, "template", templateRef)
		}
		if record != nil {
			maxPoints = record.MaxPoints
			labTemplateID = record.ID
		}
	}
	if labTemplateID == "" {
		labTemplateID = templateRef
	}
	return maxPoints, labTemplateID
}

// resolvePathwayContext validates enrollment/module pairing when provided.
// Returns the enrollment+module IDs to persist (nil pointers when no pathway
// context is requested). ok=false after writing an error response.
func (m *Manager) resolvePathwayContext(w http.ResponseWriter, r *http.Request, req *createSessionRequest) (*string, *string, bool) {
	ctx := r.Context()
	bothEmpty := req.EnrollmentID == "" && req.ModuleID == ""
	bothSet := req.EnrollmentID != "" && req.ModuleID != ""
	if bothEmpty {
		return nil, nil, true
	}
	if !bothSet {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "session.errors.enrollmentAndModuleRequired", nil)
		return nil, nil, false
	}
	if m.enrollmentRepo == nil {
		// Caller provided pathway context but server has no enrollment store —
		// silently drop the linkage rather than failing the request.
		return nil, nil, true
	}

	enrollment, err := m.enrollmentRepo.GetByID(ctx, req.EnrollmentID)
	if err != nil {
		m.logger.Error("Failed to verify enrollment", "error", err, "enrollmentId", req.EnrollmentID)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "session.errors.enrollmentVerifyFailed", nil)
		return nil, nil, false
	}
	if enrollment == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "session.errors.enrollmentNotFound", nil)
		return nil, nil, false
	}
	if enrollment.UserID != req.UserID {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusForbidden, "session.errors.enrollmentNotOwned", nil)
		return nil, nil, false
	}

	if m.pathwayRepo != nil {
		module, err := m.pathwayRepo.GetModuleByID(ctx, req.ModuleID)
		if err != nil {
			m.logger.Error("Failed to verify module", "error", err, "moduleId", req.ModuleID)
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "session.errors.moduleVerifyFailed", nil)
			return nil, nil, false
		}
		if module == nil {
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "session.errors.moduleNotFound", nil)
			return nil, nil, false
		}
		if module.PathwayID != enrollment.PathwayID {
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "session.errors.moduleNotInPathway", nil)
			return nil, nil, false
		}
	}

	m.logger.Info("Session linked to pathway", "enrollmentId", req.EnrollmentID, "moduleId", req.ModuleID)
	return &req.EnrollmentID, &req.ModuleID, true
}

// persistSession starts the evaluator session (which validates the template)
// and writes the DB row. Ordering matters: evaluator first so a template
// failure doesn't leave an orphaned DB record. ok=false after responding.
func (m *Manager) persistSession(
	w http.ResponseWriter,
	r *http.Request,
	sessionID string,
	req *createSessionRequest,
	labTemplateID string,
	maxPoints int,
	enrollmentID, moduleID *string,
	user *auth.User,
) bool {
	if m.sessionRepo == nil {
		return true
	}
	ctx := r.Context()

	sess := &models.Session{
		ID:                 sessionID,
		PodID:              req.PodID,
		UserID:             req.UserID,
		LabTemplateID:      labTemplateID,
		MaxPoints:          maxPoints,
		CanvasCourseID:     req.CanvasCourseID,
		CanvasAssignmentID: req.CanvasAssignmentID,
		CanvasUserID:       req.CanvasUserID,
		EnrollmentID:       enrollmentID,
		ModuleID:           moduleID,
		Metadata:           make(map[string]string),
	}
	// Org from tenant context (header/URL) with user-default fallback.
	if tc, ok := custommiddleware.TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
		sess.OrganizationID = &tc.Organization.ID
	} else if user != nil && user.DefaultOrganizationID != "" {
		sess.OrganizationID = &user.DefaultOrganizationID
	}

	if err := m.evaluator.StartSession(sessionID, req.PodID, req.UserID, req.LabTemplate); err != nil {
		m.logger.Error("start evaluator session failed", "error", err, "sessionId", sessionID, "labTemplate", req.LabTemplate)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "sessions.errors.invalidLabTemplate", nil)
		return false
	}
	if err := m.sessionRepo.Create(ctx, sess); err != nil {
		m.logger.Error("Failed to persist session to database", "error", err, "sessionId", sessionID)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "session.errors.createFailed", nil)
		return false
	}
	return true
}

// registerActiveChecks wires the session into the active-check runner so
// periodic checkpoint probes start firing. Best-effort: failure to fetch the
// pod or zero checkpoints just means no registration.
func (m *Manager) registerActiveChecks(ctx context.Context, sessionID, podID, labTemplate string) {
	if m.activeCheckRunner == nil {
		return
	}
	checkpoints := m.evaluator.GetTemplateCheckpoints(labTemplate)
	if len(checkpoints) == 0 {
		return
	}

	vmIPs := make(map[string]string)
	vmPlatformIDs := make(map[string]string)
	if pod, err := m.orchestrator.GetPod(ctx, podID); err == nil && pod != nil {
		for _, vm := range pod.VMs {
			if vm.IPAddress != "" {
				vmIPs[vm.Name] = vm.IPAddress
			}
			if vm.PlatformID != "" {
				vmPlatformIDs[vm.Name] = vm.PlatformID
			}
		}
	}

	m.activeCheckRunner.RegisterSession(sessionID, podID, checkpoints, vmIPs, vmPlatformIDs)
}

// preflightAgentsAndStartTampering queries Wazuh for agent status; if every
// agent is dead at session start, it shortens the tampering grace period so
// the first incident fires immediately. Non-blocking — session creation
// always succeeds regardless of pre-flight outcome.
func (m *Manager) preflightAgentsAndStartTampering(ctx context.Context, sessionID, podID string) map[string]string {
	if m.wazuhService == nil || m.orchestrator == nil {
		return nil
	}

	pod, err := m.orchestrator.GetPod(ctx, podID)
	if err != nil || pod == nil {
		return nil
	}
	if len(pod.VMs) == 0 {
		return nil
	}

	vmNames := make([]string, 0, len(pod.VMs))
	for _, vm := range pod.VMs {
		vmNames = append(vmNames, vm.Name)
	}

	agentStatus, err := m.wazuhService.CheckPodAgents(ctx, podID, vmNames)
	if err != nil {
		m.logger.Warn("Pre-flight agent check failed (non-blocking)", "error", err, "podId", podID)
		return nil
	}
	m.logger.Info("Pre-flight agent status", "podId", podID, "agentStatus", agentStatus)

	allDead := len(agentStatus) > 0
	for _, status := range agentStatus {
		if status == "active" {
			allDead = false
			break
		}
	}
	if allDead && m.tamperingDetector != nil {
		agentIDs := make([]string, 0, len(vmNames))
		for _, vmName := range vmNames {
			agentIDs = append(agentIDs, podID+"-"+vmName)
		}
		m.tamperingDetector.StartSessionWithGraceOverride(sessionID, podID, 0, agentIDs)
		m.logger.Warn("All agents dead at session start — grace period skipped", "sessionId", sessionID, "podId", podID)
	}
	return agentStatus
}

func (m *Manager) handleGetSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()

		// Get authenticated user for access control
		user, _ := auth.UserFromContext(ctx)

		// Try database first
		if m.sessionRepo != nil {
			sess, err := m.sessionRepo.GetByID(ctx, sessionID)
			if err != nil {
				m.logger.Error("Failed to get session from database", "error", err, "sessionId", sessionID)
			} else if sess != nil {
				// Check access permission
				if !canAccessSession(user, sess) {
					m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
					return
				}
				// Merge with evaluator progress if available
				if progress, err := m.evaluator.GetSessionProgress(sessionID); err == nil {
					sess.EarnedPoints = progress.EarnedPoints
					if progress.MaxPoints > 0 {
						sess.Percentage = float64(progress.EarnedPoints) / float64(progress.MaxPoints) * 100
					}
				}
				m.responder.JSONResponse(w, http.StatusOK, sess)
				return
			}
		}

		// Fallback to evaluator only
		progress, err := m.evaluator.GetSessionProgress(sessionID)
		if err != nil {
			m.logger.Warn("Session not found in evaluator", "error", err, "sessionId", sessionID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, progress)
	}
}

func (m *Manager) handleEndSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()

		// Get authenticated user for access control
		user, _ := auth.UserFromContext(ctx)

		// Fetch session for ownership check and threshold
		var sess *models.Session
		if m.sessionRepo != nil {
			var err error
			sess, err = m.sessionRepo.GetByID(ctx, sessionID)
			if err != nil {
				m.logger.Error("Failed to get session", "error", err, "sessionId", sessionID)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.getFailed", nil)
				return
			}
			if sess == nil {
				// GetByID returns (nil, nil) for a missing row. Without this
				// the ownership check below is skipped, both repo writes fail
				// silently into the log, and the handler answers 200 for a
				// session that never existed.
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
				return
			}
			if !canModifySession(user, sess) {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
				return
			}
		}

		// Capture whether this session was open BEFORE any write below. The
		// repository may hand back a pointer it also mutates in End(), so
		// reading sess.EndedAt afterwards is not reliable.
		wasOpen := sess == nil || sess.EndedAt == nil

		// Get final progress before ending (use configurable threshold, default 70%)
		var earnedPoints int
		var passed bool
		if progress, err := m.evaluator.GetSessionProgress(sessionID); err == nil {
			earnedPoints = progress.EarnedPoints
			percentage := 0.0
			if progress.MaxPoints > 0 {
				percentage = float64(earnedPoints) / float64(progress.MaxPoints) * 100
			}
			passThreshold := 70
			if sess != nil && sess.PassingThreshold > 0 {
				passThreshold = sess.PassingThreshold
			}
			passed = percentage >= float64(passThreshold)
		}

		// End in evaluator
		m.evaluator.EndSession(sessionID)

		// End in database
		if m.sessionRepo != nil {
			// Update grade first
			if err := m.sessionRepo.UpdateGrade(ctx, sessionID, earnedPoints, passed); err != nil {
				m.logger.Error("Failed to update session grade", "error", err, "sessionId", sessionID)
			}
			// Then mark as ended
			if err := m.sessionRepo.End(ctx, sessionID); err != nil {
				m.logger.Error("Failed to end session in database", "error", err, "sessionId", sessionID)
			}
		}

		// Ending an already-ended session is a no-op in the repository, so only
		// the transition is an outcome. Matches handleDeleteSession.
		if m.metrics != nil && wasOpen {
			m.metrics.SessionEnded()
		}

		m.logger.Info("Session ended", "sessionId", sessionID, "earnedPoints", earnedPoints, "passed", passed)
		m.responder.JSONResponse(w, http.StatusOK, SessionSubmitResponse{
			SessionID:    sessionID,
			Passed:       passed,
			EarnedPoints: earnedPoints,
		})
	}
}

func (m *Manager) handleGetProgress() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()

		// Get authenticated user for access control
		user, _ := auth.UserFromContext(ctx)

		// Check access permission first if session exists in database
		if m.sessionRepo != nil {
			sess, err := m.sessionRepo.GetByID(ctx, sessionID)
			if err == nil && sess != nil {
				if !canAccessSession(user, sess) {
					m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
					return
				}
			}
		}

		// Try evaluator first for live progress with descriptions (includes checkpoint names)
		checkpointsWithDesc, err := m.evaluator.GetSessionProgressWithDescriptions(sessionID)
		if err == nil {
			// Also get the basic progress for totals
			progress, _ := m.evaluator.GetSessionProgress(sessionID)

			percentage := 0.0
			maxPoints := 0
			earnedPoints := 0
			if progress != nil {
				maxPoints = progress.MaxPoints
				earnedPoints = progress.EarnedPoints
				if maxPoints > 0 {
					percentage = float64(earnedPoints) / float64(maxPoints) * 100
				}
			}

			// Format checkpoints for frontend with all required fields
			formattedCheckpoints := make([]CheckpointProgressDTO, 0, len(checkpointsWithDesc))
			for _, cp := range checkpointsWithDesc {
				formattedCheckpoints = append(formattedCheckpoints, CheckpointProgressDTO{
					ID:       cp.CheckpointID,
					Name:     cp.Description,
					Status:   string(cp.Status),
					Score:    cp.EarnedPoints,
					MaxScore: cp.Points,
				})
			}

			m.responder.JSONResponse(w, http.StatusOK, SessionProgressResponse{
				SessionID:    sessionID,
				EarnedPoints: earnedPoints,
				MaxPoints:    maxPoints,
				Percentage:   percentage,
				Passed:       earnedPoints >= maxPoints/2,
				Checkpoints:  formattedCheckpoints,
			})
			return
		}

		// Fallback to database
		if m.sessionRepo != nil {
			sess, dbErr := m.sessionRepo.GetByID(ctx, sessionID)
			if dbErr != nil {
				m.logger.Error("Failed to get session from database", "error", dbErr, "sessionId", sessionID)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
				return
			}
			if sess != nil {
				// For database fallback, try to get checkpoints from checkpoint_progress table
				var formattedCheckpoints []CheckpointProgressDTO
				if m.checkpointRepo != nil {
					cpProgress, cpErr := m.checkpointRepo.GetBySessionID(ctx, sessionID)
					if cpErr == nil && len(cpProgress) > 0 {
						formattedCheckpoints = make([]CheckpointProgressDTO, 0, len(cpProgress))
						for _, cp := range cpProgress {
							formattedCheckpoints = append(formattedCheckpoints, CheckpointProgressDTO{
								ID:       cp.CheckpointID,
								Name:     cp.CheckpointID, // Use ID as name if description not available
								Status:   string(cp.Status),
								Score:    cp.EarnedPoints,
								MaxScore: cp.Points,
							})
						}
					}
				}

				// If no checkpoint progress records, try to get checkpoints from the lab template
				if len(formattedCheckpoints) == 0 && m.labTemplateRepo != nil {
					template, tplErr := m.labTemplateRepo.GetByID(ctx, sess.LabTemplateID)
					if tplErr == nil && template != nil && len(template.Checkpoints) > 0 {
						// Parse checkpoints from the template JSON into a typed
						// local shape — only three fields are used, so any extra
						// keys in the source JSON are ignored.
						var checkpointList []templateCheckpointShape
						if jsonErr := json.Unmarshal(template.Checkpoints, &checkpointList); jsonErr == nil {
							formattedCheckpoints = make([]CheckpointProgressDTO, 0, len(checkpointList))
							for _, cp := range checkpointList {
								formattedCheckpoints = append(formattedCheckpoints, CheckpointProgressDTO{
									ID:       cp.ID,
									Name:     cp.Description,
									Status:   "pending",
									Score:    0,
									MaxScore: cp.Points,
								})
							}
						} else {
							m.logger.Warn("Failed to parse checkpoints JSON", "error", jsonErr, "sessionId", sessionID)
						}
					}
				}

				if formattedCheckpoints == nil {
					formattedCheckpoints = []CheckpointProgressDTO{}
				}

				m.responder.JSONResponse(w, http.StatusOK, SessionProgressResponse{
					SessionID:    sess.ID,
					EarnedPoints: sess.EarnedPoints,
					MaxPoints:    sess.MaxPoints,
					Percentage:   sess.Percentage,
					Passed:       sess.Passed,
					Checkpoints:  formattedCheckpoints,
				})
				return
			}
		}

		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
	}
}

func (m *Manager) handleGetCheckpoints() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()

		// Get authenticated user for access control
		user, _ := auth.UserFromContext(ctx)

		// Check access permission first if session exists in database
		if m.sessionRepo != nil {
			sess, err := m.sessionRepo.GetByID(ctx, sessionID)
			if err == nil && sess != nil {
				if !canAccessSession(user, sess) {
					m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
					return
				}
			}
		}

		progress, err := m.evaluator.GetSessionProgress(sessionID)
		if err != nil {
			m.logger.Warn("Session not found in evaluator", "error", err, "sessionId", sessionID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, CheckpointsResponse{
			SessionID:   sessionID,
			Checkpoints: progress.Checkpoints,
		})
	}
}

func (m *Manager) handleDeleteSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()

		// Get authenticated user for access control
		user, _ := auth.UserFromContext(ctx)

		if m.sessionRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.repositoryUnavailable", nil)
			return
		}

		// Check if session exists
		sess, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err != nil {
			m.logger.Error("Failed to get session", "error", err, "sessionId", sessionID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.getFailed", nil)
			return
		}
		if sess == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
			return
		}

		// Check modify permission (owner or admin)
		if !canModifySession(user, sess) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
			return
		}

		// Captured before the delete: the repository may hand back a pointer
		// it also mutates, so this cannot be read afterwards. Same reason as
		// handleEndSession.
		wasOpen := sess.EndedAt == nil

		// Delete the session (this also deletes associated checkpoint progress)
		if err := m.sessionRepo.Delete(ctx, sessionID); err != nil {
			m.logger.Error("Failed to delete session", "error", err, "sessionId", sessionID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.deleteFailed", nil)
			return
		}

		// Also remove from evaluator if it's tracking this session
		m.evaluator.EndSession(sessionID)

		// Only an open session counts as an end; one that had already ended
		// was counted when it ended.
		if m.metrics != nil && wasOpen {
			m.metrics.SessionEnded()
		}

		m.logger.Info("Session deleted", "sessionId", sessionID)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{
			"status":    "deleted",
			"sessionId": sessionID,
		})
	}
}

func (m *Manager) handleSubmitSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()

		// Get authenticated user for access control
		user, _ := auth.UserFromContext(ctx)

		// Check access control before calling service
		// First, get the session to verify ownership
		sess, err := m.sessionService.GetByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
				return
			}
			m.logger.Error("Failed to get session", "error", err, "sessionId", sessionID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.getFailed", nil)
			return
		}

		// Check modify permission (only owner or admin can submit)
		if !canModifySession(user, sess) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
			return
		}

		// Call service to submit the session
		result, err := m.sessionService.Submit(ctx, sessionID)
		if err != nil {
			// Map service errors to HTTP status codes
			switch {
			case errors.Is(err, session.ErrSessionNotFound):
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "session.errors.notFound", nil)
			case errors.Is(err, session.ErrSessionAlreadySubmitted):
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "session.errors.alreadySubmitted", nil)
			case errors.Is(err, session.ErrForbidden):
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "session.errors.accessDenied", nil)
			case errors.Is(err, session.ErrProgressUnavailable):
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.progressUnavailable", nil)
			case errors.Is(err, session.ErrSessionRepoNotAvailable):
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.repositoryUnavailable", nil)
			default:
				m.logger.Error("Failed to submit session", "error", err, "sessionId", sessionID)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.submitFailed", nil)
			}
			return
		}

		// Format response for API consumers
		m.responder.JSONResponse(w, http.StatusOK, SessionSubmitDetailResponse{
			SessionID:           result.SessionID,
			Status:              result.Status,
			EarnedPoints:        result.EarnedPoints,
			MaxPoints:           result.MaxPoints,
			Percentage:          result.Percentage,
			Passed:              result.Passed,
			PassThreshold:       result.PassThreshold,
			Checkpoints:         result.Checkpoints,
			SubmittedAt:         result.SubmittedAt.Format(time.RFC3339),
			Achievements:        result.Achievements,
			AchievementsPending: result.AchievementsPending,
		})
	}
}

// handleCleanupStaleSessions ends active sessions that have been running
// longer than the specified maxAge. Admin-only.
func (m *Manager) handleCleanupStaleSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.sessionRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.repositoryUnavailable", nil)
			return
		}

		// Parse maxAge from query parameter (default: 24h)
		maxAgeStr := r.URL.Query().Get("maxAge")
		if maxAgeStr == "" {
			maxAgeStr = "24h"
		}
		maxAge, err := time.ParseDuration(maxAgeStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "session.errors.maxAgeInvalid", nil)
			return
		}
		if maxAge < time.Hour {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "session.errors.maxAgeTooSmall", nil)
			return
		}

		count, err := m.sessionRepo.EndStaleSessions(r.Context(), maxAge)
		if err != nil {
			m.logger.Error("Failed to clean up stale sessions", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "session.errors.cleanupStaleFailed", nil)
			return
		}

		if m.metrics != nil {
			m.metrics.SessionsEnded(count)
		}

		m.logger.Info("Stale sessions cleaned up", "count", count, "maxAge", maxAge.String())
		m.responder.JSONResponse(w, http.StatusOK, cleanupStaleSessionsResponse{
			Cleaned: count,
			MaxAge:  maxAge.String(),
		})
	}
}
