package organizations

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Team Handlers
// -----------------------------------------------------------------------------

// CreateTeamRequest represents the request body for creating a team
type CreateTeamRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
}

// UpdateTeamRequest represents the request body for updating a team
type UpdateTeamRequest struct {
	Name        *string `json:"name,omitempty"`
	Slug        *string `json:"slug,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"isActive,omitempty"`
}

// handleListTeams lists teams in an organization
func (m *Manager) handleListTeams() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		if m.teamRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		activeOnly := r.URL.Query().Get("active_only") == "true"
		var activePtr *bool
		if activeOnly {
			activePtr = &activeOnly
		}

		filter := repositories.TeamFilter{
			Active: activePtr,
		}

		teams, err := m.teamRepo.ListByOrganization(r.Context(), tc.Organization.ID, filter)
		if err != nil {
			m.logger.Error("Failed to list teams", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.listFailed", nil)
			return
		}

		result := make([]map[string]any, 0, len(teams))
		for _, team := range teams {
			memberCount, _ := m.teamRepo.GetMemberCount(r.Context(), team.ID)
			result = append(result, map[string]any{
				"id":          team.ID,
				"name":        team.Name,
				"slug":        team.Slug,
				"description": team.Description,
				"isActive":    team.IsActive,
				"memberCount": memberCount,
				"createdAt":   team.CreatedAt,
			})
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"teams": result,
			"count": len(result),
		})
	}
}

// handleCreateTeam creates a new team
func (m *Manager) handleCreateTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require instructor or admin role to create teams
		if !tc.HasRole(models.OrgRoleInstructor) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "team.errors.instructorRequired", nil)
			return
		}

		if m.teamRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		// Check if teams are enabled
		if !tc.HasFeature("teams") && tc.Edition == models.EditionCommunity {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusPaymentRequired, "team.errors.editionRequired", nil)
			return
		}

		var req CreateTeamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Name == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "team.errors.nameRequired", nil)
			return
		}

		// Generate slug if not provided
		slug := req.Slug
		if slug == "" {
			slug = generateSlug(req.Name)
		}

		// Check if slug already exists in this organization
		existing, err := m.teamRepo.GetBySlug(r.Context(), tc.Organization.ID, slug)
		if err != nil {
			m.logger.Error("Failed to check team slug", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.createFailed", nil)
			return
		}
		if existing != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "team.errors.slugAlreadyExists", nil)
			return
		}

		team := &models.Team{
			ID:             uuid.New().String(),
			OrganizationID: tc.Organization.ID,
			Name:           req.Name,
			Slug:           slug,
			Description:    req.Description,
			IsActive:       true,
		}

		if err := m.teamRepo.Create(r.Context(), team); err != nil {
			m.logger.Error("Failed to create team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.createFailed", nil)
			return
		}

		// Add creator as team lead
		user, _ := auth.UserFromContext(r.Context())
		membership := &models.TeamMembership{
			ID:     uuid.New().String(),
			TeamID: team.ID,
			UserID: user.ID,
			Role:   models.TeamRoleLead,
		}

		if m.teamMembershipRepo != nil {
			if err := m.teamMembershipRepo.Create(r.Context(), membership); err != nil {
				m.logger.Warn("Failed to add creator as team lead", "error", err)
			}
		}

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":          team.ID,
			"name":        team.Name,
			"slug":        team.Slug,
			"description": team.Description,
			"isActive":    team.IsActive,
			"createdAt":   team.CreatedAt,
		})
	}
}

// handleGetTeam gets team details
func (m *Manager) handleGetTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")

		if m.teamRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.getFailed", nil)
			return
		}
		if team == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		// Verify team belongs to current organization
		if team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		memberCount, _ := m.teamRepo.GetMemberCount(r.Context(), team.ID)

		response := map[string]any{
			"id":          team.ID,
			"name":        team.Name,
			"slug":        team.Slug,
			"description": team.Description,
			"isActive":    team.IsActive,
			"memberCount": memberCount,
			"createdAt":   team.CreatedAt,
		}

		// Check if current user is a member
		user, hasUser := auth.UserFromContext(r.Context())
		if hasUser && m.teamMembershipRepo != nil {
			membership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), team.ID, user.ID)
			if err == nil && membership != nil {
				response["membership"] = map[string]any{
					"role":     membership.Role,
					"joinedAt": membership.CreatedAt,
				}
			}
		}

		m.responder.JSONResponse(w, http.StatusOK, response)
	}
}

// handleUpdateTeam updates a team
func (m *Manager) handleUpdateTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")

		if m.teamRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateFailed", nil)
			return
		}
		if team == nil || team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		// Check permissions: org admin or team lead can update
		canUpdate := tc.HasRole(models.OrgRoleAdmin)
		if !canUpdate && m.teamMembershipRepo != nil {
			user, hasUser := auth.UserFromContext(r.Context())
			if hasUser {
				membership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), team.ID, user.ID)
				if err == nil && membership != nil && membership.Role == models.TeamRoleLead {
					canUpdate = true
				}
			}
		}

		if !canUpdate {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "team.errors.leadOrAdminRequired", nil)
			return
		}

		var req UpdateTeamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Apply updates
		if req.Name != nil {
			team.Name = *req.Name
		}
		if req.Slug != nil {
			// Check if new slug already exists
			existing, err := m.teamRepo.GetBySlug(r.Context(), tc.Organization.ID, *req.Slug)
			if err != nil {
				m.logger.Error("Failed to check team slug", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateFailed", nil)
				return
			}
			if existing != nil && existing.ID != team.ID {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "team.errors.slugAlreadyExists", nil)
				return
			}
			team.Slug = *req.Slug
		}
		if req.Description != nil {
			team.Description = *req.Description
		}
		if req.IsActive != nil {
			team.IsActive = *req.IsActive
		}

		if err := m.teamRepo.Update(r.Context(), team); err != nil {
			m.logger.Error("Failed to update team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"id":          team.ID,
			"name":        team.Name,
			"slug":        team.Slug,
			"description": team.Description,
			"isActive":    team.IsActive,
		})
	}
}

// handleDeleteTeam deletes a team
func (m *Manager) handleDeleteTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require admin role to delete teams
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")

		if m.teamRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.deleteFailed", nil)
			return
		}
		if team == nil || team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		if err := m.teamRepo.Delete(r.Context(), team.ID); err != nil {
			m.logger.Error("Failed to delete team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.deleteFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message": "team deleted",
		})
	}
}

// -----------------------------------------------------------------------------
// Team Membership Handlers
// -----------------------------------------------------------------------------

// AddTeamMemberRequest represents the request body for adding a team member
type AddTeamMemberRequest struct {
	UserID string          `json:"userId"`
	Role   models.TeamRole `json:"role"`
}

// UpdateTeamMemberRequest represents the request body for updating a team member
type UpdateTeamMemberRequest struct {
	Role models.TeamRole `json:"role"`
}

// handleListTeamMembers lists members of a team
func (m *Manager) handleListTeamMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")

		if m.teamRepo == nil || m.teamMembershipRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		// Verify team belongs to organization
		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.listMembersFailed", nil)
			return
		}
		if team == nil || team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		memberships, err := m.teamMembershipRepo.ListByTeam(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to list team members", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.listMembersFailed", nil)
			return
		}

		members := make([]map[string]any, 0, len(memberships))
		for _, mem := range memberships {
			member := map[string]any{
				"id":        mem.ID,
				"userId":    mem.UserID,
				"role":      mem.Role,
				"createdAt": mem.CreatedAt,
			}

			// Get user details if available
			if m.userRepo != nil {
				user, err := m.userRepo.GetByID(r.Context(), mem.UserID)
				if err == nil && user != nil {
					member["user"] = map[string]any{
						"id":          user.ID,
						"username":    user.Username,
						"email":       user.Email,
						"displayName": user.DisplayName,
					}
				}
			}

			members = append(members, member)
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"members": members,
			"count":   len(members),
		})
	}
}

// handleAddTeamMember adds a member to a team
func (m *Manager) handleAddTeamMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")

		if m.teamRepo == nil || m.teamMembershipRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		// Verify team belongs to organization
		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.addMemberFailed", nil)
			return
		}
		if team == nil || team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		// Check permissions: org admin/instructor or team lead can add members
		canAddMembers := tc.HasRole(models.OrgRoleInstructor)
		if !canAddMembers {
			user, hasUser := auth.UserFromContext(r.Context())
			if hasUser {
				membership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), team.ID, user.ID)
				if err == nil && membership != nil && membership.Role == models.TeamRoleLead {
					canAddMembers = true
				}
			}
		}

		if !canAddMembers {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "team.errors.leadOrInstructorRequired", nil)
			return
		}

		var req AddTeamMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.UserID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "team.errors.userIdRequired", nil)
			return
		}

		// Set default role
		if req.Role == "" {
			req.Role = models.TeamRoleMember
		}

		// Verify user is a member of the organization
		if m.orgMembershipRepo != nil {
			orgMembership, err := m.orgMembershipRepo.GetByOrgAndUser(r.Context(), tc.Organization.ID, req.UserID)
			if err != nil {
				m.logger.Error("Failed to verify org membership", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.addMemberFailed", nil)
				return
			}
			if orgMembership == nil || orgMembership.AcceptedAt == nil {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "team.errors.userNotOrgMember", nil)
				return
			}
		}

		// Check if already a team member
		existing, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), teamID, req.UserID)
		if err != nil {
			m.logger.Error("Failed to check existing team membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.addMemberFailed", nil)
			return
		}
		if existing != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "team.errors.userAlreadyMember", nil)
			return
		}

		membership := &models.TeamMembership{
			ID:        uuid.New().String(),
			TeamID:    teamID,
			UserID:    req.UserID,
			Role:      req.Role,
			CreatedAt: time.Now(),
		}

		if err := m.teamMembershipRepo.Create(r.Context(), membership); err != nil {
			m.logger.Error("Failed to add team member", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.addMemberFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":        membership.ID,
			"teamId":    membership.TeamID,
			"userId":    membership.UserID,
			"role":      membership.Role,
			"createdAt": membership.CreatedAt,
		})
	}
}

// handleUpdateTeamMember updates a team member's role
func (m *Manager) handleUpdateTeamMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")
		userID := chi.URLParam(r, "userID")

		if m.teamRepo == nil || m.teamMembershipRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		// Verify team belongs to organization
		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateMemberFailed", nil)
			return
		}
		if team == nil || team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		// Check permissions: org admin or team lead can update members
		canUpdate := tc.HasRole(models.OrgRoleAdmin)
		if !canUpdate {
			user, hasUser := auth.UserFromContext(r.Context())
			if hasUser {
				membership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), team.ID, user.ID)
				if err == nil && membership != nil && membership.Role == models.TeamRoleLead {
					canUpdate = true
				}
			}
		}

		if !canUpdate {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "team.errors.leadOrAdminRequired", nil)
			return
		}

		var req UpdateTeamMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Get existing membership
		membership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), teamID, userID)
		if err != nil {
			m.logger.Error("Failed to get team membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateMemberFailed", nil)
			return
		}
		if membership == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.memberNotFound", nil)
			return
		}

		// Prevent demoting the last team lead
		if membership.Role == models.TeamRoleLead && req.Role != models.TeamRoleLead {
			leads, err := m.teamMembershipRepo.ListByTeamAndRole(r.Context(), teamID, models.TeamRoleLead)
			if err != nil {
				m.logger.Error("Failed to count team leads", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateMemberFailed", nil)
				return
			}
			if len(leads) <= 1 {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "team.errors.cannotDemoteLastLead", nil)
				return
			}
		}

		if err := m.teamMembershipRepo.UpdateRole(r.Context(), membership.ID, req.Role); err != nil {
			m.logger.Error("Failed to update team member role", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.updateMemberFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"userId": userID,
			"role":   req.Role,
		})
	}
}

// handleRemoveTeamMember removes a member from a team
func (m *Manager) handleRemoveTeamMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		teamID := chi.URLParam(r, "teamID")
		userID := chi.URLParam(r, "userID")

		if m.teamRepo == nil || m.teamMembershipRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "team.errors.repositoryNotConfigured", nil)
			return
		}

		// Verify team belongs to organization
		team, err := m.teamRepo.GetByID(r.Context(), teamID)
		if err != nil {
			m.logger.Error("Failed to get team", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.removeMemberFailed", nil)
			return
		}
		if team == nil || team.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.notFound", nil)
			return
		}

		// Get existing membership
		membership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), teamID, userID)
		if err != nil {
			m.logger.Error("Failed to get team membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.removeMemberFailed", nil)
			return
		}
		if membership == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "team.errors.memberNotFound", nil)
			return
		}

		// Check permissions: org admin, team lead, or self can remove
		currentUser, hasUser := auth.UserFromContext(r.Context())
		canRemove := tc.HasRole(models.OrgRoleAdmin)
		if !canRemove && hasUser {
			// Users can remove themselves
			if currentUser.ID == userID {
				canRemove = true
			} else {
				// Check if current user is team lead
				currentMembership, err := m.teamMembershipRepo.GetByTeamAndUser(r.Context(), team.ID, currentUser.ID)
				if err == nil && currentMembership != nil && currentMembership.Role == models.TeamRoleLead {
					canRemove = true
				}
			}
		}

		if !canRemove {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "team.errors.leadOrAdminRequired", nil)
			return
		}

		// Prevent removing the last team lead
		if membership.Role == models.TeamRoleLead {
			leads, err := m.teamMembershipRepo.ListByTeamAndRole(r.Context(), teamID, models.TeamRoleLead)
			if err != nil {
				m.logger.Error("Failed to count team leads", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.removeMemberFailed", nil)
				return
			}
			if len(leads) <= 1 {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "team.errors.cannotRemoveLastLead", nil)
				return
			}
		}

		if err := m.teamMembershipRepo.Delete(r.Context(), membership.ID); err != nil {
			m.logger.Error("Failed to remove team member", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "team.errors.removeMemberFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message": "team member removed",
		})
	}
}
