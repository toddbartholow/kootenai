// Package server provides the HTTP server and API routes
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// handleDeepLinkingRequest handles LTI deep linking requests for assignment creation
func (m *CanvasManager) handleDeepLinkingRequest(w http.ResponseWriter, r *http.Request, launch *canvas.LTILaunchRequest, idToken string) {
	// Parse the raw JWT to get deep linking settings (they're not in the typed struct)
	token, _, err := new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		m.logger.Error("Failed to parse token for deep linking", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "ltiDeeplink.errors.parseTokenFailed", nil)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "ltiDeeplink.errors.invalidTokenClaims", nil)
		return
	}

	// Extract deep linking settings
	settings, err := canvas.GetDeepLinkingSettings(claims)
	if err != nil {
		m.logger.Error("Failed to extract deep linking settings", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "ltiDeeplink.errors.missingDeeplinkSettings", nil)
		return
	}

	m.logger.Info("Deep linking request received",
		"courseId", launch.Context.ID,
		"returnUrl", settings.DeepLinkReturnURL,
		"acceptTypes", settings.AcceptTypes,
	)

	// Check if instructor
	if !launch.IsInstructor() {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "ltiDeeplink.errors.instructorRequired", nil)
		return
	}

	// Build redirect URL to template selection UI
	// Include necessary parameters for the selection process
	selectParams := url.Values{}
	selectParams.Set("return_url", settings.DeepLinkReturnURL)
	selectParams.Set("deployment_id", launch.DeploymentID)
	selectParams.Set("course_id", launch.Context.ID)
	selectParams.Set("course_name", launch.Context.Title)
	if settings.Data != "" {
		selectParams.Set("data", settings.Data)
	}

	// Canvas issuer for the response
	selectParams.Set("issuer", launch.Issuer)

	// Generate a session token for the instructor
	if m.authService != nil && launch.CanvasUser != nil {
		user := m.getOrCreateUserFromLaunch(r.Context(), launch)
		if user != nil {
			token, err := m.authService.GenerateToken(user)
			if err == nil {
				selectParams.Set("token", token)
			}
		}
	}

	// Redirect to the template selection UI
	selectURL := fmt.Sprintf("/lti/select?%s", selectParams.Encode())

	m.logger.Info("Redirecting to template selection",
		"selectUrl", selectURL,
	)

	http.Redirect(w, r, selectURL, http.StatusFound)
}

// DeepLinkTemplateRequest represents a request to create a deep link for a template
type DeepLinkTemplateRequest struct {
	TemplateID   string  `json:"templateId"`
	CustomTitle  string  `json:"customTitle,omitempty"`
	ReturnURL    string  `json:"returnUrl"`
	DeploymentID string  `json:"deploymentId"`
	CourseID     string  `json:"courseId"`
	Data         string  `json:"data,omitempty"`
	Issuer       string  `json:"issuer"`
	MaxPoints    float64 `json:"maxPoints,omitempty"`
}

// handleListLTITemplates returns available templates for deep linking selection
func (m *CanvasManager) handleListLTITemplates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get organization context if available (from auth token)
		var orgID *string
		if user, ok := auth.UserFromContext(ctx); ok && user.DefaultOrganizationID != "" {
			orgID = &user.DefaultOrganizationID
		}

		// Query templates
		if m.ltiAssignmentRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "lti.errors.notConfigured", nil)
			return
		}

		templates, err := m.ltiAssignmentRepo.ListActiveTemplates(ctx, orgID)
		if err != nil {
			m.logger.Error("Failed to list templates", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "ltiDeeplink.errors.listTemplatesFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"templates": templates,
		})
	}
}

// handleDeepLinkSubmit processes template selection and returns the deep linking response
func (m *CanvasManager) handleDeepLinkSubmit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var req DeepLinkTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Validate required fields
		if req.TemplateID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "ltiDeeplink.errors.templateIdRequired", nil)
			return
		}
		if req.ReturnURL == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "ltiDeeplink.errors.returnUrlRequired", nil)
			return
		}
		if req.CourseID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "ltiDeeplink.errors.courseIdRequired", nil)
			return
		}

		// Check services
		if m.deepLinkingService == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "ltiDeeplink.errors.notConfigured", nil)
			return
		}
		if m.ltiAssignmentRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "ltiDeeplink.errors.repoNotConfigured", nil)
			return
		}

		// Get template info
		templates, err := m.ltiAssignmentRepo.ListActiveTemplates(ctx, nil)
		if err != nil {
			m.logger.Error("Failed to list templates", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "ltiDeeplink.errors.getTemplatesFailed", nil)
			return
		}

		var selectedTemplate *models.LTITemplateInfo
		for _, t := range templates {
			if t.ID == req.TemplateID {
				selectedTemplate = t
				break
			}
		}

		if selectedTemplate == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "ltiDeeplink.errors.templateNotFound", nil)
			return
		}

		// Generate resource link ID for this assignment
		resourceLinkID := uuid.New().String()

		// Determine title and max points
		title := req.CustomTitle
		if title == "" {
			title = selectedTemplate.Name
		}
		maxPoints := req.MaxPoints
		if maxPoints == 0 {
			maxPoints = float64(selectedTemplate.MaxPoints)
		}

		// Store the assignment mapping
		assignment := &models.LTIAssignment{
			ID:             uuid.New().String(),
			CanvasCourseID: req.CourseID,
			ResourceLinkID: resourceLinkID,
			LabTemplateID:  selectedTemplate.ID,
			CustomTitle:    req.CustomTitle,
			MaxPoints:      maxPoints,
			DeploymentID:   req.DeploymentID,
			LTIVersion:     "1.3",
		}

		if err := m.ltiAssignmentRepo.Create(ctx, assignment); err != nil {
			m.logger.Error("Failed to create LTI assignment", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "ltiDeeplink.errors.saveAssignmentFailed", nil)
			return
		}

		m.logger.Info("Created LTI assignment",
			"assignmentId", assignment.ID,
			"courseId", req.CourseID,
			"templateId", selectedTemplate.ID,
			"resourceLinkId", resourceLinkID,
		)

		// Build the launch URL
		launchURL := m.config.ToolIssuer + "/lti/launch"

		// Create content item for deep linking response
		contentItem := canvas.CreateResourceLinkItem(
			title,
			selectedTemplate.Description,
			launchURL,
			selectedTemplate.ID,
			resourceLinkID,
			maxPoints,
		)

		// Build deep linking settings for response
		settings := &canvas.DeepLinkingSettings{
			DeepLinkReturnURL: req.ReturnURL,
			Data:              req.Data,
		}

		// Determine audience (Canvas issuer)
		audience := req.Issuer
		if audience == "" {
			audience = m.config.CanvasURL
		}

		// Build signed JWT response
		jwtResponse, err := m.deepLinkingService.BuildDeepLinkingResponse(
			ctx,
			settings,
			[]canvas.ContentItem{contentItem},
			req.DeploymentID,
			m.config.ClientID,
			audience,
		)
		if err != nil {
			m.logger.Error("Failed to build deep linking response", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "ltiDeeplink.errors.buildResponseFailed", nil)
			return
		}

		// Return auto-submitting HTML form
		html := m.deepLinkingService.BuildAutoSubmitForm(req.ReturnURL, jwtResponse)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// handleLTISelectPage serves the template selection page for deep linking
func (m *CanvasManager) handleLTISelectPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// This serves a simple HTML page that will load the Vue component
		// or can be a standalone page for template selection

		returnURL := r.URL.Query().Get("return_url")
		deploymentID := r.URL.Query().Get("deployment_id")
		courseID := r.URL.Query().Get("course_id")
		courseName := r.URL.Query().Get("course_name")
		data := r.URL.Query().Get("data")
		issuer := r.URL.Query().Get("issuer")

		// Serve a self-contained selection page
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Select Lab Template</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #f5f5f5;
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 20px;
            border-radius: 12px;
            margin-bottom: 24px;
        }
        .header h1 { font-size: 24px; margin-bottom: 4px; }
        .header p { opacity: 0.9; font-size: 14px; }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 20px;
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 20px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
            border: 2px solid transparent;
        }
        .card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 16px rgba(0,0,0,0.15);
            border-color: #667eea;
        }
        .card.selected {
            border-color: #667eea;
            background: #f0f4ff;
        }
        .card h3 { font-size: 18px; margin-bottom: 8px; color: #333; }
        .card p { font-size: 14px; color: #666; margin-bottom: 12px; line-height: 1.5; }
        .tags { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
        .tag {
            background: #e0e7ff;
            color: #4338ca;
            padding: 4px 10px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 500;
        }
        .tag.difficulty { background: #fef3c7; color: #92400e; }
        .tag.points { background: #d1fae5; color: #065f46; }
        .meta { font-size: 12px; color: #888; }
        .loading {
            text-align: center;
            padding: 60px;
            color: #666;
        }
        .spinner {
            width: 40px;
            height: 40px;
            border: 3px solid #e0e0e0;
            border-top-color: #667eea;
            border-radius: 50%%;
            animation: spin 1s linear infinite;
            margin: 0 auto 16px;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
        .error { color: #dc2626; text-align: center; padding: 40px; }
        .empty { text-align: center; padding: 60px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Select Lab Assignment</h1>
            <p>Course: %s</p>
        </div>
        <div id="templates" class="grid">
            <div class="loading">
                <div class="spinner"></div>
                <p>Loading templates...</p>
            </div>
        </div>
    </div>

    <script>
        const config = {
            returnUrl: %q,
            deploymentId: %q,
            courseId: %q,
            data: %q,
            issuer: %q
        };

        async function loadTemplates() {
            try {
                const response = await fetch('/api/v1/lti/templates');
                if (!response.ok) throw new Error('Failed to load templates');
                const data = await response.json();
                renderTemplates(data.templates || []);
            } catch (error) {
                document.getElementById('templates').innerHTML =
                    '<div class="error">Failed to load templates: ' + error.message + '</div>';
            }
        }

        function renderTemplates(templates) {
            const container = document.getElementById('templates');
            if (templates.length === 0) {
                container.innerHTML = '<div class="empty">No lab templates available</div>';
                return;
            }

            container.innerHTML = templates.map(t => {
                const tags = (t.tags || []).map(tag =>
                    '<span class="tag">' + escapeHtml(tag) + '</span>'
                ).join('');

                return '<div class="card" onclick="selectTemplate(\'' + t.id + '\', \'' + escapeHtml(t.name) + '\', ' + t.maxPoints + ')">' +
                    '<h3>' + escapeHtml(t.name) + '</h3>' +
                    '<p>' + escapeHtml(t.description || 'No description') + '</p>' +
                    '<div class="tags">' +
                        '<span class="tag difficulty">' + escapeHtml(t.difficulty || 'Intermediate') + '</span>' +
                        '<span class="tag points">' + t.maxPoints + ' pts</span>' +
                        '<span class="tag">' + t.checkpointCount + ' objectives</span>' +
                        tags +
                    '</div>' +
                    '<div class="meta">' + t.durationMinutes + ' minutes</div>' +
                '</div>';
            }).join('');
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        async function selectTemplate(templateId, templateName, maxPoints) {
            // Show loading state
            document.querySelectorAll('.card').forEach(c => c.classList.remove('selected'));
            event.currentTarget.classList.add('selected');
            event.currentTarget.innerHTML = '<div class="loading"><div class="spinner"></div><p>Creating assignment...</p></div>';

            try {
                const response = await fetch('/api/v1/lti/deep-link/submit', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        templateId: templateId,
                        returnUrl: config.returnUrl,
                        deploymentId: config.deploymentId,
                        courseId: config.courseId,
                        data: config.data,
                        issuer: config.issuer,
                        maxPoints: maxPoints
                    })
                });

                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(error.error || 'Failed to create assignment');
                }

                // Response is HTML form that auto-submits
                const html = await response.text();
                document.open();
                document.write(html);
                document.close();
            } catch (error) {
                alert('Error: ' + error.message);
                loadTemplates(); // Reload to reset state
            }
        }

        // Load templates on page load
        loadTemplates();
    </script>
</body>
</html>`, courseName, returnURL, deploymentID, courseID, data, issuer)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// getOrCreateUserFromLaunch creates an auth.User from LTI launch data
func (m *CanvasManager) getOrCreateUserFromLaunch(ctx context.Context, launch *canvas.LTILaunchRequest) *auth.User {
	if launch.CanvasUser == nil {
		return nil
	}

	// Determine role
	role := "student"
	if launch.IsInstructor() {
		role = "instructor"
	}

	// Get email
	email := launch.CanvasUser.Email
	if email == "" {
		email = launch.CanvasUser.LoginID
	}

	// Get user ID (try to find existing user or use Canvas subject)
	userID := launch.Subject
	if m.userRepo != nil && email != "" {
		user, err := m.userRepo.GetByEmail(ctx, email)
		if err == nil && user != nil {
			userID = user.ID
		}
	}

	return &auth.User{
		ID:    userID,
		Email: email,
		Name:  launch.CanvasUser.Name,
		Roles: []string{role},
	}
}
