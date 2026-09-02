// Package server provides the HTTP server and API routes
package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
)

type ltiState struct {
	Nonce         string
	TargetLinkURI string
	CreatedAt     time.Time
}

// In-memory LTI state cleanup tunables. Used only when Redis isn't configured;
// the Redis-backed path has its own TTL set on the store.
const (
	// ltiStateCleanupInterval is how often the in-memory store is swept.
	ltiStateCleanupInterval = 5 * time.Minute

	// ltiStateMaxAge is the age past which an in-memory state entry is
	// considered expired and dropped from the store.
	ltiStateMaxAge = 10 * time.Minute
)

// storeLTIState stores the LTI state in Redis (preferred) or in-memory (fallback)
func (m *CanvasManager) storeLTIState(ctx context.Context, state string, data ltiState) error {
	// Use Redis if available (prefer injected cache for testing)
	if cache := m.getLTIStateCache(); cache != nil {
		return cache.Store(ctx, state, redisclient.LTIState{
			Nonce:         data.Nonce,
			TargetLinkURI: data.TargetLinkURI,
			CreatedAt:     data.CreatedAt,
		})
	}

	// Fallback to in-memory store
	m.ltiStateStoreMu.Lock()
	m.ltiStateStore[state] = data
	m.ltiStateStoreMu.Unlock()

	// Start periodic cleanup once (instead of spawning a goroutine per call)
	m.ltiCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(ltiStateCleanupInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					m.ltiStateStoreMu.Lock()
					for k, v := range m.ltiStateStore {
						if time.Since(v.CreatedAt) > ltiStateMaxAge {
							delete(m.ltiStateStore, k)
						}
					}
					m.ltiStateStoreMu.Unlock()
				case <-m.ltiCleanupStopCh:
					return
				}
			}
		}()
	})

	return nil
}

// getLTIState retrieves the LTI state from Redis (preferred) or in-memory (fallback)
func (m *CanvasManager) getLTIState(ctx context.Context, state string) (*ltiState, bool) {
	// Use Redis if available (prefer injected cache for testing)
	if cache := m.getLTIStateCache(); cache != nil {
		redisState, err := cache.Get(ctx, state)
		if err != nil {
			m.logger.Error("failed to get LTI state from Redis", "error", err)
			return nil, false
		}
		if redisState == nil {
			return nil, false
		}
		return &ltiState{
			Nonce:         redisState.Nonce,
			TargetLinkURI: redisState.TargetLinkURI,
			CreatedAt:     redisState.CreatedAt,
		}, true
	}

	// Fallback to in-memory store
	m.ltiStateStoreMu.RLock()
	storedState, exists := m.ltiStateStore[state]
	m.ltiStateStoreMu.RUnlock()
	if !exists {
		return nil, false
	}
	return &storedState, true
}

// deleteLTIState removes the LTI state from Redis (preferred) or in-memory (fallback)
func (m *CanvasManager) deleteLTIState(ctx context.Context, state string) {
	// Use Redis if available (prefer injected cache for testing)
	if cache := m.getLTIStateCache(); cache != nil {
		if err := cache.Delete(ctx, state); err != nil {
			m.logger.Error("failed to delete LTI state from Redis", "error", err)
		}
		return
	}

	// Fallback to in-memory store
	m.ltiStateStoreMu.Lock()
	delete(m.ltiStateStore, state)
	m.ltiStateStoreMu.Unlock()
}

// generateRandomString generates a cryptographically secure random string.
// Returns an error if the system random source fails so request-path callers
// can respond with 500 instead of taking the server down.
func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand read: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// -----------------------------------------------------------------------------
// LTI 1.3 Handlers
// -----------------------------------------------------------------------------

// handleLTILaunch handles LTI 1.3 OIDC login initiation from Canvas
// ltiLaunchInfo holds the fields extracted from a validated LTI launch
// request that downstream session/user/org helpers need.
type ltiLaunchInfo struct {
	CanvasUserID   string
	UserEmail      string
	UserName       string
	CourseID       string
	CourseName     string
	CourseCode     string
	ResourceLinkID string
	ResourceTitle  string
	LabTemplate    string
	UserRole       string
}

func (m *CanvasManager) handleLTILaunch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.ltiService == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "lti.errors.notConfigured", nil)
			return
		}
		if err := r.ParseForm(); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.parseFormFailed", nil)
			return
		}

		idToken := r.FormValue("id_token")
		loginHint := r.FormValue("login_hint")
		// OIDC step 1: login_hint with no id_token redirects to the platform for auth.
		if loginHint != "" && idToken == "" {
			m.handleOIDCInitiation(w, r)
			return
		}
		if idToken == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.missingIdToken", nil)
			return
		}

		launchRequest, err := m.ltiService.ValidateLaunch(r.Context(), idToken)
		if err != nil {
			m.logger.Error("LTI launch validation failed", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "lti.errors.invalidLaunch", nil)
			return
		}
		m.logger.Info("LTI launch received",
			"userId", launchRequest.Subject,
			"courseId", launchRequest.Context.ID,
			"resourceLinkId", launchRequest.ResourceLink.ID,
			"canSubmitGrades", launchRequest.CanSubmitGrades(),
		)

		info := extractLTILaunchInfo(launchRequest)
		orgID := m.mapCourseToOrg(r.Context(), info)
		userID := m.resolveCanvasUser(r.Context(), info)

		launchData := buildLaunchData(info, userID, orgID, launchRequest)
		token := m.issueLTIToken(launchData, info, userID, orgID, launchRequest)
		redirectURL := m.buildLTIRedirect(w, r, info.LabTemplate, token)

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"status":   "launched",
			"launch":   launchData,
			"redirect": redirectURL,
		})
	}
}

// extractLTILaunchInfo flattens the validated launch request into the subset
// of fields downstream helpers actually need.
func extractLTILaunchInfo(launch *canvas.LTILaunchRequest) ltiLaunchInfo {
	info := ltiLaunchInfo{
		CanvasUserID:   launch.Subject,
		CourseID:       launch.Context.ID,
		CourseName:     launch.Context.Label,
		ResourceLinkID: launch.ResourceLink.ID,
		ResourceTitle:  launch.ResourceLink.Title,
		UserRole:       "student",
	}
	if info.CourseName == "" {
		info.CourseName = launch.Context.Title
	}
	if launch.CanvasCourse != nil {
		info.CourseCode = launch.CanvasCourse.CourseCode
	}
	if launch.CanvasUser != nil {
		info.UserName = launch.CanvasUser.Name
		info.UserEmail = launch.CanvasUser.Email
		if info.UserEmail == "" {
			info.UserEmail = launch.CanvasUser.LoginID
		}
	}
	if launch.Custom != nil {
		if t, ok := launch.Custom["lab_template"]; ok {
			info.LabTemplate = t
		}
	}
	if launch.IsInstructor() {
		info.UserRole = "instructor"
	}
	return info
}

// mapCourseToOrg looks up (or creates) the organization that owns this
// Canvas course. Best-effort: a failure logs and returns "".
func (m *CanvasManager) mapCourseToOrg(ctx context.Context, info ltiLaunchInfo) string {
	if m.canvasSyncService == nil || m.orgRepo == nil {
		return ""
	}
	org, err := m.canvasSyncService.GetOrCreateOrganizationForCourse(ctx, info.CourseID, info.CourseName, info.CourseCode)
	if err != nil {
		m.logger.Warn("Failed to map Canvas course to organization", "courseId", info.CourseID, "error", err)
		return ""
	}
	m.logger.Info("Mapped Canvas course to organization", "courseId", info.CourseID, "orgId", org.ID, "orgName", org.Name)
	return org.ID
}

// resolveCanvasUser returns the local user ID for the Canvas user, falling
// back to the Canvas subject ID when no matching local user is found.
func (m *CanvasManager) resolveCanvasUser(ctx context.Context, info ltiLaunchInfo) string {
	if m.userRepo != nil && info.UserEmail != "" {
		user, err := m.userRepo.GetByEmail(ctx, info.UserEmail)
		if err != nil {
			m.logger.Warn("Failed to look up user by email", "email", info.UserEmail, "error", err)
		}
		if user != nil {
			return user.ID
		}
	}
	return info.CanvasUserID
}

// buildLaunchData composes the JSON payload returned to the client.
func buildLaunchData(info ltiLaunchInfo, userID, orgID string, launch *canvas.LTILaunchRequest) map[string]any {
	data := map[string]any{
		"userId":          userID,
		"canvasUserId":    info.CanvasUserID,
		"courseId":        info.CourseID,
		"courseName":      info.CourseName,
		"resourceLinkId":  info.ResourceLinkID,
		"resourceTitle":   info.ResourceTitle,
		"labTemplate":     info.LabTemplate,
		"canSubmitGrades": launch.CanSubmitGrades(),
		"role":            info.UserRole,
	}
	if orgID != "" {
		data["organizationId"] = orgID
	}
	if launch.AGS != nil {
		data["lineItemUrl"] = launch.AGS.LineItem
		data["lineItemsUrl"] = launch.AGS.LineItems
	}
	return data
}

// issueLTIToken mints a JWT for the launched user when an auth service is
// configured. The token (if any) is also written into launchData["token"].
func (m *CanvasManager) issueLTIToken(launchData map[string]any, info ltiLaunchInfo, userID, orgID string, launch *canvas.LTILaunchRequest) string {
	if m.authService == nil {
		return ""
	}
	authUser := &auth.User{
		ID:                    userID,
		Email:                 info.UserEmail,
		Name:                  info.UserName,
		Roles:                 []string{info.UserRole},
		DefaultOrganizationID: orgID,
	}
	if launch.CanvasUser != nil && authUser.Name == "" {
		authUser.Name = launch.CanvasUser.Name
	}
	token, err := m.authService.GenerateToken(authUser)
	if err != nil {
		m.logger.Warn("Failed to generate token for LTI user", "error", err)
		return ""
	}
	launchData["token"] = token
	return token
}

// buildLTIRedirect picks where the iframe should jump next: cookie-mode keeps
// the token out of the URL; bearer-mode passes it through as a query param.
func (m *CanvasManager) buildLTIRedirect(w http.ResponseWriter, r *http.Request, labTemplate, token string) string {
	redirectURL := fmt.Sprintf("/labs?template=%s", labTemplate)
	if token == "" {
		return redirectURL
	}
	if m.cookieCfg.Enabled {
		auth.SetAuthCookie(w, r, token, m.cookieCfg)
		if csrfToken, err := auth.GenerateCSRFToken(); err == nil {
			auth.SetCSRFCookie(w, r, csrfToken, m.cookieCfg)
		}
		return redirectURL
	}
	return fmt.Sprintf("/lti/callback?token=%s&redirect=/labs?template=%s", token, labTemplate)
}

// handleOIDCInitiation handles the OIDC login initiation (step 1 of LTI 1.3)
func (m *CanvasManager) handleOIDCInitiation(w http.ResponseWriter, r *http.Request) {
	// Extract parameters from the login initiation request
	iss := r.FormValue("iss")
	loginHint := r.FormValue("login_hint")
	targetLinkURI := r.FormValue("target_link_uri")
	ltiMessageHint := r.FormValue("lti_message_hint")
	clientID := r.FormValue("client_id")

	m.logger.Info("OIDC login initiation received",
		"iss", iss,
		"loginHint", loginHint,
		"targetLinkUri", targetLinkURI,
		"clientId", clientID,
	)

	// Validate required parameters
	if iss == "" || loginHint == "" {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.missingOidcParams", nil)
		return
	}

	// Use configured client ID if not provided
	if clientID == "" {
		clientID = m.config.ClientID
	}

	// Generate state and nonce for security
	state, err := generateRandomString(16)
	if err != nil {
		m.logger.Error("failed to generate LTI state", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "lti.errors.initiateLoginFailed", nil)
		return
	}
	nonce, err := generateRandomString(16)
	if err != nil {
		m.logger.Error("failed to generate LTI nonce", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "lti.errors.initiateLoginFailed", nil)
		return
	}

	// Store state for validation on callback (uses Redis if available, otherwise in-memory)
	if err := m.storeLTIState(r.Context(), state, ltiState{
		Nonce:         nonce,
		TargetLinkURI: targetLinkURI,
		CreatedAt:     time.Now(),
	}); err != nil {
		m.logger.Error("failed to store LTI state", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "lti.errors.initiateLoginFailed", nil)
		return
	}

	// Build the authorization URL
	// Canvas authorization endpoint is typically: {canvas_url}/api/lti/authorize_redirect
	authURL := m.config.AuthorizationURL
	if authURL == "" {
		authURL = m.config.CanvasURL + "/api/lti/authorize_redirect"
	}

	// Build redirect URI (where Canvas will send the id_token)
	redirectURI := m.config.ToolIssuer + "/lti/callback"

	// Construct authorization request parameters
	params := url.Values{}
	params.Set("scope", "openid")
	params.Set("response_type", "id_token")
	params.Set("response_mode", "form_post")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("login_hint", loginHint)
	params.Set("state", state)
	params.Set("nonce", nonce)
	if ltiMessageHint != "" {
		params.Set("lti_message_hint", ltiMessageHint)
	}
	params.Set("prompt", "none")

	// Redirect to Canvas authorization endpoint
	authRedirectURL := authURL + "?" + params.Encode()

	m.logger.Info("Redirecting to Canvas authorization",
		"authUrl", authURL,
		"redirectUri", redirectURI,
		"state", state,
	)

	// Use HTTP redirect - this works better for cookie handling
	// The browser will follow the redirect and send cookies appropriately
	http.Redirect(w, r, authRedirectURL, http.StatusFound)
}

// handleLTICallback handles the OIDC callback with id_token (step 3 of LTI 1.3)
func (m *CanvasManager) handleLTICallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.ltiService == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "lti.errors.notConfigured", nil)
			return
		}

		if err := r.ParseForm(); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.parseFormFailed", nil)
			return
		}

		idToken := r.FormValue("id_token")
		state := r.FormValue("state")

		if idToken == "" {
			// Check for error response
			errorMsg := r.FormValue("error")
			errorDesc := r.FormValue("error_description")
			if errorMsg != "" {
				m.logger.Error("LTI authorization error", "error", errorMsg, "description", errorDesc)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.authorizationError", map[string]any{"Code": errorMsg, "Description": errorDesc})
				return
			}
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.missingIdToken", nil)
			return
		}

		// Validate state (uses Redis if available, otherwise in-memory)
		storedState, exists := m.getLTIState(r.Context(), state)
		if !exists {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "lti.errors.invalidState", nil)
			return
		}

		// Remove used state
		m.deleteLTIState(r.Context(), state)

		// Validate the LTI launch
		launchRequest, err := m.ltiService.ValidateLaunch(r.Context(), idToken)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "validate LTI launch")
			return
		}

		m.logger.Info("LTI launch validated successfully",
			"userId", launchRequest.Subject,
			"courseId", launchRequest.Context.ID,
			"messageType", launchRequest.MessageType,
			"nonce", storedState.Nonce,
		)

		// Check message type and route accordingly
		if launchRequest.IsDeepLinkingRequest() {
			m.handleDeepLinkingRequest(w, r, launchRequest, idToken)
			return
		}

		// Process regular resource link launch
		m.processLTILaunch(w, r, launchRequest)
	}
}

// processLTILaunch handles the actual launch after token validation
// processLTILaunch handles a deep-link / resource-link launch by either
// resuming an existing session or provisioning a new pod and session, then
// rendering the VNC console page.
func (m *CanvasManager) processLTILaunch(w http.ResponseWriter, r *http.Request, launchRequest any) {
	launch, ok := launchRequest.(*canvas.LTILaunchRequest)
	if !ok {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "lti.errors.invalidLaunchType", nil)
		return
	}
	ctx := r.Context()

	resourceLinkID, canvasCourseID, canvasUserID := extractLTILaunchIDs(launch)
	m.logger.Info("LTI resource link launch",
		"resourceLinkId", resourceLinkID,
		"courseId", canvasCourseID,
		"userId", canvasUserID,
	)

	labTemplateID, ltiAssignment := m.resolveLaunchTemplate(ctx, launch, canvasCourseID, resourceLinkID)
	if labTemplateID == "" {
		m.logger.Warn("No lab template found for LTI launch", "resourceLinkId", resourceLinkID)
		m.renderLTIErrorPage(w, "No Lab Template Configured",
			"This assignment has not been linked to a lab template. Please contact your instructor.")
		return
	}

	userName := canvasUserID
	if launch.CanvasUser != nil && launch.CanvasUser.Name != "" {
		userName = launch.CanvasUser.Name
	}
	user, err := m.getOrCreateLTIUser(ctx, canvasUserID, userName, launch)
	if err != nil {
		m.logger.Error("Failed to get or create user", "error", err)
		m.renderLTIErrorPage(w, "User Error", "Failed to set up user account.")
		return
	}

	pod, ok := m.resumeOrProvisionLTISession(ctx, w, user, labTemplateID, ltiAssignment, canvasCourseID, canvasUserID)
	if !ok {
		return // error page already rendered
	}

	if pod == nil || len(pod.VMs) == 0 {
		m.renderLTIErrorPage(w, "No VM Available", "Your lab environment is still being prepared. Please refresh in a moment.")
		return
	}
	m.renderVNCConsolePage(w, r, pod, launch)
}

// extractLTILaunchIDs returns (resourceLinkID, canvasCourseID, canvasUserID),
// applying any `canvas_course_id` / `canvas_user_id` custom-claim overrides.
func extractLTILaunchIDs(launch *canvas.LTILaunchRequest) (resourceLinkID, canvasCourseID, canvasUserID string) {
	resourceLinkID = launch.ResourceLink.ID
	canvasCourseID = launch.Context.ID
	canvasUserID = launch.Subject
	if launch.Custom != nil {
		if cid := launch.Custom["canvas_course_id"]; cid != "" {
			canvasCourseID = cid
		}
		if uid := launch.Custom["canvas_user_id"]; uid != "" {
			canvasUserID = uid
		}
	}
	return resourceLinkID, canvasCourseID, canvasUserID
}

// resolveLaunchTemplate finds the lab template for this launch. Prefers a
// stored LTIAssignment (course+resource_link → template) over custom claims.
// Returns ("", nil) when no template can be resolved.
func (m *CanvasManager) resolveLaunchTemplate(ctx context.Context, launch *canvas.LTILaunchRequest, canvasCourseID, resourceLinkID string) (string, *models.LTIAssignment) {
	var labTemplateID string
	var ltiAssignment *models.LTIAssignment

	if m.ltiAssignmentRepo != nil && resourceLinkID != "" {
		var err error
		ltiAssignment, err = m.ltiAssignmentRepo.GetByResourceLink(ctx, canvasCourseID, resourceLinkID)
		if err != nil {
			m.logger.Error("Failed to look up LTI assignment", "error", err, "resourceLinkId", resourceLinkID)
		}
		if ltiAssignment != nil {
			labTemplateID = ltiAssignment.LabTemplateID
			m.logger.Info("Found LTI assignment", "assignmentId", ltiAssignment.ID, "templateId", labTemplateID)
		}
	}
	if labTemplateID == "" && launch.Custom != nil {
		labTemplateID = launch.Custom["lab_template_id"]
		if labTemplateID == "" {
			labTemplateID = launch.Custom["lab_template"]
		}
	}
	return labTemplateID, ltiAssignment
}

// resumeOrProvisionLTISession finds an existing active session for this
// (user, assignment) pair, or provisions a fresh pod + session. Returns
// ok=false after rendering an error page on failure.
func (m *CanvasManager) resumeOrProvisionLTISession(
	ctx context.Context,
	w http.ResponseWriter,
	user *models.User,
	labTemplateID string,
	ltiAssignment *models.LTIAssignment,
	canvasCourseID, canvasUserID string,
) (*models.Pod, bool) {
	if existing := m.findExistingLTISession(ctx, user.ID, labTemplateID, ltiAssignment); existing != nil {
		m.logger.Info("Using existing session", "sessionId", existing.ID)
		var pod *models.Pod
		if m.podRepo != nil && existing.PodID != "" {
			pod, _ = m.podRepo.GetByID(ctx, existing.PodID)
		}
		return pod, true
	}
	return m.provisionLTIPodAndSession(ctx, w, user, labTemplateID, ltiAssignment, canvasCourseID, canvasUserID)
}

// findExistingLTISession returns the active session for this user+assignment
// pair, or nil if none. Errors in the lookup are logged and treated as "no
// existing session" so the launch falls through to fresh provisioning.
func (m *CanvasManager) findExistingLTISession(ctx context.Context, userID, labTemplateID string, ltiAssignment *models.LTIAssignment) *models.Session {
	if m.sessionRepo == nil || ltiAssignment == nil {
		return nil
	}
	activeFilter := true
	sessions, err := m.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID:     userID,
		TemplateID: labTemplateID,
		Active:     &activeFilter,
	})
	if err != nil {
		m.logger.Warn("Failed to check for existing sessions", "error", err)
		return nil
	}
	for _, sess := range sessions {
		if sess.LTIAssignmentID != nil && *sess.LTIAssignmentID == ltiAssignment.ID {
			return sess
		}
	}
	return nil
}

// provisionLTIPodAndSession loads the template, creates a pod, starts its
// VMs, and persists a session. Returns ok=false after rendering an error
// page when any required step fails.
func (m *CanvasManager) provisionLTIPodAndSession(
	ctx context.Context,
	w http.ResponseWriter,
	user *models.User,
	labTemplateID string,
	ltiAssignment *models.LTIAssignment,
	canvasCourseID, canvasUserID string,
) (*models.Pod, bool) {
	m.logger.Info("Creating new pod for LTI launch", "templateId", labTemplateID, "userId", user.ID)

	if m.labTemplateRepo == nil {
		m.renderLTIErrorPage(w, "Configuration Error", "Lab template system not configured.")
		return nil, false
	}

	templateRecord, err := m.labTemplateRepo.GetByID(ctx, labTemplateID)
	if err != nil || templateRecord == nil {
		m.logger.Error("Failed to load template", "error", err, "templateId", labTemplateID)
		m.renderLTIErrorPage(w, "Template Not Found", "The lab template for this assignment could not be found.")
		return nil, false
	}
	template, err := templateRecord.ToLabTemplate()
	if err != nil {
		m.logger.Error("Failed to parse template", "error", err)
		m.renderLTIErrorPage(w, "Template Error", "Failed to load lab template configuration.")
		return nil, false
	}

	pod, err := m.orchestrator.CreatePod(ctx, template, templateRecord.ID, user.ID, user.Username)
	if err != nil {
		m.logger.Error("Failed to create pod", "error", err)
		m.renderLTIErrorPage(w, "Pod Creation Failed", "Failed to create your lab environment. Please try again.")
		return nil, false
	}
	m.logger.Info("Pod created for LTI launch", "podId", pod.ID)
	m.startPodVMs(ctx, pod)

	session := &models.Session{
		ID:            uuid.New().String(),
		PodID:         pod.ID,
		UserID:        user.ID,
		LabTemplateID: labTemplateID,
		Status:        "active",
		MaxPoints:     templateRecord.MaxPoints,
	}
	if ltiAssignment != nil {
		session.LTIAssignmentID = &ltiAssignment.ID
		session.CanvasCourseID = canvasCourseID
		session.CanvasUserID = canvasUserID
	}
	if m.sessionRepo != nil {
		if err := m.sessionRepo.Create(ctx, session); err != nil {
			// Pod was created but session failed — continue anyway so the
			// user can still see their environment.
			m.logger.Error("Failed to create session", "error", err)
		} else if m.metrics != nil {
			// The LTI launch is a second session-creation path alongside
			// handleCreateSession; without this, every Canvas-launched
			// session is ended later without ever having been counted.
			m.metrics.SessionCreated()
		}
	}
	return pod, true
}

// startPodVMs starts every VM in the pod directly. Failures are logged but
// not fatal — the user can retry from the console.
func (m *CanvasManager) startPodVMs(ctx context.Context, pod *models.Pod) {
	for _, vm := range pod.VMs {
		if vm.Platform != "proxmox" || vm.PlatformID == "" {
			continue
		}
		vmid := 0
		if _, err := fmt.Sscanf(vm.PlatformID, "%d", &vmid); err != nil || vmid <= 0 {
			continue
		}
		m.logger.Info("Starting VM for LTI launch", "vmid", vmid, "node", vm.Node, "podId", pod.ID)
		if err := m.orchestrator.StartVMDirect(ctx, vm.Node, vmid); err != nil {
			m.logger.Error("Failed to start VM", "error", err, "vmid", vmid)
			continue
		}
		m.logger.Info("VM started for LTI launch", "vmid", vmid, "podId", pod.ID)
	}
}

// getOrCreateLTIUser gets or creates a user based on LTI launch info
func (m *CanvasManager) getOrCreateLTIUser(ctx context.Context, canvasUserID, displayName string, launch *canvas.LTILaunchRequest) (*models.User, error) {
	if m.userRepo == nil {
		return nil, fmt.Errorf("user repository not configured")
	}

	// Try to find existing user by Canvas ID
	username := fmt.Sprintf("canvas_%s", canvasUserID)

	user, err := m.userRepo.GetOrCreateByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	// Update display name if available
	if displayName != "" && user.DisplayName != displayName {
		user.DisplayName = displayName
		if err := m.userRepo.Update(ctx, user); err != nil {
			m.logger.Debug("Best-effort display name update failed", "error", err, "userId", user.ID)
		}
	}

	return user, nil
}

// renderLTIErrorPage renders an error page suitable for LTI iframe.
// Returns HTTP 200 even on error so Canvas shows the page inside the iframe
// rather than its own error chrome. Title/Message are passed through
// html/template which auto-escapes them.
func (m *CanvasManager) renderLTIErrorPage(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	data := struct {
		Title   string
		Message string
	}{Title: title, Message: message}

	if err := ltiErrorTemplate.Execute(w, data); err != nil {
		m.logger.Error("Failed to render LTI error page", "error", err)
	}
}

// handleLTIJWKS returns the public JWKS for LTI tool authentication
func (m *CanvasManager) handleLTIJWKS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return our tool's public key in JWK format for Canvas to verify our JWTs
		keys := []any{}

		// Parse public key from config if available
		if m.config.ToolPublicKey != "" {
			pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(m.config.ToolPublicKey))
			if err != nil {
				m.logger.Error("Failed to parse public key for JWKS", "error", err)
			} else {
				jwk := rsaPublicKeyToJWK(pubKey, "kootenai-lti-key")
				keys = append(keys, jwk)
			}
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"keys": keys,
		})
	}
}

// rsaPublicKeyToJWK converts an RSA public key to JWK format
func rsaPublicKeyToJWK(pubKey *rsa.PublicKey, kid string) map[string]any {
	// Convert modulus (n) to base64url
	nBytes := pubKey.N.Bytes()
	n := base64.RawURLEncoding.EncodeToString(nBytes)

	// Convert exponent (e) to base64url
	eBytes := make([]byte, 0, 4)
	e := pubKey.E
	for e > 0 {
		eBytes = append([]byte{byte(e & 0xff)}, eBytes...)
		e >>= 8
	}
	eEncoded := base64.RawURLEncoding.EncodeToString(eBytes)

	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": kid,
		"n":   n,
		"e":   eEncoded,
	}
}

// handleLTIToken handles OAuth2 token exchange (for tool-initiated launches)
func (m *CanvasManager) handleLTIToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.ltiService == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "lti.errors.notConfigured", nil)
			return
		}

		// This endpoint is used for platform-initiated token exchange
		// In basic LTI 1.3, the platform (Canvas) handles OAuth2
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotImplemented, "lti.errors.tokenEndpointNotImplemented", nil)
	}
}

// SubmitGrade submits a grade back to Canvas via AGS
func (m *CanvasManager) SubmitGrade(ctx context.Context, sessionID string, lineItemURL string, userID string, percentage float64, maxPoints float64, comment string) error {
	if m.gradeService == nil {
		return fmt.Errorf("LTI grade service not configured")
	}

	return m.gradeService.SubmitGrade(ctx, lineItemURL, userID, percentage, maxPoints, comment)
}
