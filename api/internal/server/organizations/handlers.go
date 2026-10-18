package organizations

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Organization Handlers
// -----------------------------------------------------------------------------

// CreateOrganizationRequest represents the request body for creating an organization
type CreateOrganizationRequest struct {
	Name         string              `json:"name"`
	Slug         string              `json:"slug"`
	Type         models.OrgType      `json:"type"`
	ContactEmail string              `json:"contactEmail,omitempty"`
	Settings     *models.OrgSettings `json:"settings,omitempty"`
}

// UpdateOrganizationRequest represents the request body for updating an organization
type UpdateOrganizationRequest struct {
	Name         *string             `json:"name,omitempty"`
	Slug         *string             `json:"slug,omitempty"`
	Type         *models.OrgType     `json:"type,omitempty"`
	ContactEmail *string             `json:"contactEmail,omitempty"`
	LogoURL      *string             `json:"logoUrl,omitempty"`
	Settings     *models.OrgSettings `json:"settings,omitempty"`
	IsActive     *bool               `json:"isActive,omitempty"`
}

// handleListOrganizations lists organizations the current user belongs to
func (m *Manager) handleListOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.orgMembershipRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "organization.errors.repositoryNotConfigured", nil)
			return
		}

		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}

		memberships, err := m.orgMembershipRepo.ListByUser(r.Context(), user.ID)
		if err != nil {
			m.logger.Error("Failed to list user organizations", "error", err, "userID", user.ID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.listFailed", nil)
			return
		}

		// Collect org IDs for batch queries (eliminates N+1 problem)
		orgIDs := make([]string, 0, len(memberships))
		memByOrg := make(map[string]*models.OrganizationMembership, len(memberships))
		for i := range memberships {
			orgIDs = append(orgIDs, memberships[i].OrganizationID)
			memByOrg[memberships[i].OrganizationID] = memberships[i]
		}

		orgList, err := m.orgRepo.GetByIDs(r.Context(), orgIDs)
		if err != nil {
			m.logger.Error("Failed to batch fetch organizations", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.listFailed", nil)
			return
		}

		memberCounts, _ := m.orgRepo.GetMemberCountBatch(r.Context(), orgIDs)
		teamCounts, _ := m.orgRepo.GetTeamCountBatch(r.Context(), orgIDs)

		orgs := make([]map[string]any, 0, len(orgList))
		for _, org := range orgList {
			mem := memByOrg[org.ID]
			orgs = append(orgs, map[string]any{
				"id":           org.ID,
				"name":         org.Name,
				"slug":         org.Slug,
				"type":         org.Type,
				"edition":      org.Edition,
				"role":         mem.Role,
				"isPrimary":    mem.IsPrimary,
				"memberCount":  memberCounts[org.ID],
				"teamCount":    teamCounts[org.ID],
				"isActive":     org.IsActive,
				"contactEmail": org.ContactEmail,
				"logoUrl":      org.LogoURL,
				"createdAt":    org.CreatedAt,
			})
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"organizations": orgs,
			"count":         len(orgs),
		})
	}
}

// handleCreateOrganization creates a new organization
func (m *Manager) handleCreateOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.orgRepo == nil || m.orgMembershipRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "organization.errors.repositoryNotConfigured", nil)
			return
		}

		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}

		var req CreateOrganizationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Validate required fields
		if req.Name == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.nameRequired", nil)
			return
		}

		// Generate slug if not provided
		slug := req.Slug
		if slug == "" {
			slug = generateSlug(req.Name)
		}

		// Check if slug already exists
		existing, err := m.orgRepo.GetBySlug(r.Context(), slug)
		if err != nil {
			m.logger.Error("Failed to check slug uniqueness", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.createFailed", nil)
			return
		}
		if existing != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "organization.errors.slugAlreadyExists", nil)
			return
		}

		// Set defaults
		orgType := req.Type
		if orgType == "" {
			orgType = models.OrgTypeStandard
		}

		settings := models.OrgSettings{
			AllowPublicSignup:  false,
			RequireApproval:    true,
			DefaultUserRole:    models.OrgRoleMember,
			EnableTeams:        true,
			EnableAchievements: true,
			EnableLeaderboards: false,
		}
		if req.Settings != nil {
			settings = *req.Settings
		}

		// Create organization
		org := &models.Organization{
			ID:           uuid.New().String(),
			Name:         req.Name,
			Slug:         slug,
			Type:         orgType,
			Edition:      models.EditionCommunity, // Default to community
			ContactEmail: req.ContactEmail,
			Settings:     settings,
			IsActive:     true,
		}

		if err := m.orgRepo.Create(r.Context(), org); err != nil {
			m.logger.Error("Failed to create organization", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.createFailed", nil)
			return
		}

		// Add creator as owner
		now := time.Now()
		membership := &models.OrganizationMembership{
			ID:             uuid.New().String(),
			OrganizationID: org.ID,
			UserID:         user.ID,
			Role:           models.OrgRoleOwner,
			IsPrimary:      true,
			InvitedAt:      now,
			AcceptedAt:     &now,
		}

		if err := m.orgMembershipRepo.Create(r.Context(), membership); err != nil {
			m.logger.Error("Failed to create owner membership", "error", err)
			// Rollback org creation
			if rbErr := m.orgRepo.Delete(r.Context(), org.ID); rbErr != nil {
				m.logger.Error("Failed to rollback organization creation", "error", rbErr, "orgId", org.ID)
			}
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.createFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":        org.ID,
			"name":      org.Name,
			"slug":      org.Slug,
			"type":      org.Type,
			"edition":   org.Edition,
			"isActive":  org.IsActive,
			"createdAt": org.CreatedAt,
		})
	}
}

// handleGetOrganization gets organization details
func (m *Manager) handleGetOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		org := tc.Organization
		memberCount, _ := m.orgRepo.GetMemberCount(r.Context(), org.ID)
		teamCount, _ := m.orgRepo.GetTeamCount(r.Context(), org.ID)

		response := map[string]any{
			"id":           org.ID,
			"name":         org.Name,
			"slug":         org.Slug,
			"type":         org.Type,
			"edition":      org.Edition,
			"contactEmail": org.ContactEmail,
			"logoUrl":      org.LogoURL,
			"isActive":     org.IsActive,
			"memberCount":  memberCount,
			"teamCount":    teamCount,
			"settings":     org.Settings,
			"createdAt":    org.CreatedAt,
			"updatedAt":    org.UpdatedAt,
		}

		// Include membership info if available
		if tc.Membership != nil {
			response["membership"] = map[string]any{
				"role":      tc.Membership.Role,
				"isPrimary": tc.Membership.IsPrimary,
				"joinedAt":  tc.Membership.AcceptedAt,
			}
		}

		// Include limits for admins
		if tc.HasRole(models.OrgRoleAdmin) {
			response["limits"] = map[string]any{
				"maxUsers":          org.MaxUsers,
				"maxConcurrentPods": org.MaxConcurrentPods,
				"maxStorageGb":      org.MaxStorageGB,
			}
			if org.LicenseExpiresAt != nil {
				response["licenseExpiresAt"] = org.LicenseExpiresAt
			}
		}

		m.responder.JSONResponse(w, http.StatusOK, response)
	}
}

// handleUpdateOrganization updates an organization (admin only)
func (m *Manager) handleUpdateOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require admin role
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		var req UpdateOrganizationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		org := tc.Organization

		// Apply updates
		if req.Name != nil {
			org.Name = *req.Name
		}
		if req.Slug != nil {
			// Check if new slug already exists
			existing, err := m.orgRepo.GetBySlug(r.Context(), *req.Slug)
			if err != nil {
				m.logger.Error("Failed to check slug uniqueness", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.updateFailed", nil)
				return
			}
			if existing != nil && existing.ID != org.ID {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "organization.errors.slugAlreadyExists", nil)
				return
			}
			org.Slug = *req.Slug
		}
		if req.Type != nil {
			org.Type = *req.Type
		}
		if req.ContactEmail != nil {
			org.ContactEmail = *req.ContactEmail
		}
		if req.LogoURL != nil {
			org.LogoURL = *req.LogoURL
		}
		if req.Settings != nil {
			org.Settings = *req.Settings
		}
		if req.IsActive != nil {
			// Only owner can deactivate
			if !tc.HasRole(models.OrgRoleOwner) && !*req.IsActive {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.onlyOwnerCanDeactivate", nil)
				return
			}
			org.IsActive = *req.IsActive
		}

		if err := m.orgRepo.Update(r.Context(), org); err != nil {
			m.logger.Error("Failed to update organization", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.updateFailed", nil)
			return
		}

		// Audit log: organization updated
		m.logAdminAction(r, "organization.updated", "organization", org.ID, map[string]any{
			"name": org.Name,
			"slug": org.Slug,
		})

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"id":        org.ID,
			"name":      org.Name,
			"slug":      org.Slug,
			"type":      org.Type,
			"isActive":  org.IsActive,
			"updatedAt": time.Now(),
		})
	}
}

// handleDeleteOrganization deletes an organization (owner only)
func (m *Manager) handleDeleteOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require owner role
		if !tc.HasRole(models.OrgRoleOwner) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.ownerAccessRequired", nil)
			return
		}

		// Prevent deleting the system organization
		if tc.Organization.Slug == "system" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.cannotDeleteSystemOrg", nil)
			return
		}

		orgID := tc.Organization.ID
		orgName := tc.Organization.Name

		if err := m.orgRepo.Delete(r.Context(), orgID); err != nil {
			m.logger.Error("Failed to delete organization", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.deleteFailed", nil)
			return
		}

		// Audit log: organization deleted
		m.logAdminAction(r, "organization.deleted", "organization", orgID, map[string]any{
			"name": orgName,
		})

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message": "organization deleted",
		})
	}
}

// -----------------------------------------------------------------------------
// Membership Handlers
// -----------------------------------------------------------------------------

// InviteMemberRequest represents the request body for inviting a member
type InviteMemberRequest struct {
	UserID string         `json:"userId,omitempty"`
	Email  string         `json:"email,omitempty"`
	Role   models.OrgRole `json:"role"`
}

// UpdateMemberRequest represents the request body for updating a member
type UpdateMemberRequest struct {
	Role models.OrgRole `json:"role"`
}

// handleListMembers lists members of an organization
func (m *Manager) handleListMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		includePending := r.URL.Query().Get("include_pending") == "true"

		filter := repositories.MembershipFilter{
			IncludePending: includePending,
		}

		memberships, err := m.orgMembershipRepo.ListByOrganization(r.Context(), tc.Organization.ID, filter)
		if err != nil {
			m.logger.Error("Failed to list members", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.listMembersFailed", nil)
			return
		}

		members := make([]map[string]any, 0, len(memberships))
		for _, mem := range memberships {
			member := map[string]any{
				"id":        mem.ID,
				"userId":    mem.UserID,
				"role":      mem.Role,
				"isPrimary": mem.IsPrimary,
				"invitedAt": mem.InvitedAt,
				"isPending": mem.AcceptedAt == nil,
			}
			if mem.AcceptedAt != nil {
				member["acceptedAt"] = mem.AcceptedAt
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

// handleInviteMember invites a user to an organization
func (m *Manager) handleInviteMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require admin role to invite
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		user, _ := auth.UserFromContext(r.Context())

		var req InviteMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Validate request
		if req.UserID == "" && req.Email == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.userIdOrEmailRequired", nil)
			return
		}

		// Set default role
		if req.Role == "" {
			req.Role = models.OrgRoleMember
		}

		// Only owner can invite admins/owners
		if (req.Role == models.OrgRoleAdmin || req.Role == models.OrgRoleOwner) && !tc.HasRole(models.OrgRoleOwner) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.onlyOwnerCanInviteAdmins", nil)
			return
		}

		// Find user by ID or email
		var targetUserID string
		if req.UserID != "" {
			targetUserID = req.UserID
		} else if m.userRepo != nil {
			// Try to find user by email (username in this case)
			existingUser, err := m.userRepo.GetByUsername(r.Context(), req.Email)
			if err != nil {
				m.logger.Error("Failed to find user", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.inviteMemberFailed", nil)
				return
			}
			if existingUser == nil {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "organization.errors.userNotFound", nil)
				return
			}
			targetUserID = existingUser.ID
		}

		// Check if already a member
		existing, err := m.orgMembershipRepo.GetByOrgAndUser(r.Context(), tc.Organization.ID, targetUserID)
		if err != nil {
			m.logger.Error("Failed to check existing membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.inviteMemberFailed", nil)
			return
		}
		if existing != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "organization.errors.userAlreadyMember", nil)
			return
		}

		// Generate invitation token
		invitationToken := uuid.New().String()

		membership := &models.OrganizationMembership{
			ID:              uuid.New().String(),
			OrganizationID:  tc.Organization.ID,
			UserID:          targetUserID,
			Role:            req.Role,
			IsPrimary:       false,
			InvitedBy:       &user.ID,
			InvitationToken: &invitationToken,
			InvitedAt:       time.Now(),
		}

		if err := m.orgMembershipRepo.Create(r.Context(), membership); err != nil {
			m.logger.Error("Failed to create membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.inviteMemberFailed", nil)
			return
		}

		// Auto-assign default RBAC role
		assignDefaultRole(r, m.rbacRepo, targetUserID, tc.Organization.ID, &user.ID)

		// Audit log: member invited
		m.logAdminAction(r, "member.invited", "membership", membership.ID, map[string]any{
			"organizationId": tc.Organization.ID,
			"userId":         membership.UserID,
			"role":           string(membership.Role),
		})

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":              membership.ID,
			"userId":          membership.UserID,
			"role":            membership.Role,
			"invitationToken": invitationToken,
			"invitedAt":       membership.InvitedAt,
		})
	}
}

// handleUpdateMember updates a member's role
func (m *Manager) handleUpdateMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require admin role
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		userID := chi.URLParam(r, "userID")

		var req UpdateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Get existing membership
		membership, err := m.orgMembershipRepo.GetByOrgAndUser(r.Context(), tc.Organization.ID, userID)
		if err != nil {
			m.logger.Error("Failed to get membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.updateMemberFailed", nil)
			return
		}
		if membership == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "organization.errors.memberNotFound", nil)
			return
		}

		// Only owner can change admin/owner roles
		if (req.Role == models.OrgRoleAdmin || req.Role == models.OrgRoleOwner) && !tc.HasRole(models.OrgRoleOwner) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.onlyOwnerCanAssignAdminRoles", nil)
			return
		}

		// Prevent demoting the last owner
		if membership.Role == models.OrgRoleOwner && req.Role != models.OrgRoleOwner {
			// Count owners
			filter := repositories.MembershipFilter{Role: models.OrgRoleOwner}
			owners, err := m.orgMembershipRepo.ListByOrganization(r.Context(), tc.Organization.ID, filter)
			if err != nil {
				m.logger.Error("Failed to count owners", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.updateMemberFailed", nil)
				return
			}
			if len(owners) <= 1 {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.cannotDemoteLastOwner", nil)
				return
			}
		}

		oldRole := membership.Role

		if err := m.orgMembershipRepo.UpdateRole(r.Context(), membership.ID, req.Role); err != nil {
			m.logger.Error("Failed to update member role", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.updateMemberFailed", nil)
			return
		}

		// Audit log: member role changed
		m.logAdminAction(r, "member.role_changed", "membership", membership.ID, map[string]any{
			"organizationId": tc.Organization.ID,
			"userId":         userID,
			"oldRole":        string(oldRole),
			"newRole":        string(req.Role),
		})

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"userId": userID,
			"role":   req.Role,
		})
	}
}

// handleRemoveMember removes a member from an organization
func (m *Manager) handleRemoveMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.contextRequired", nil)
			return
		}

		// Require admin role
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		userID := chi.URLParam(r, "userID")
		currentUser, _ := auth.UserFromContext(r.Context())

		// Get existing membership
		membership, err := m.orgMembershipRepo.GetByOrgAndUser(r.Context(), tc.Organization.ID, userID)
		if err != nil {
			m.logger.Error("Failed to get membership", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.removeMemberFailed", nil)
			return
		}
		if membership == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "organization.errors.memberNotFound", nil)
			return
		}

		// Prevent removing the last owner
		if membership.Role == models.OrgRoleOwner {
			filter := repositories.MembershipFilter{Role: models.OrgRoleOwner}
			owners, err := m.orgMembershipRepo.ListByOrganization(r.Context(), tc.Organization.ID, filter)
			if err != nil {
				m.logger.Error("Failed to count owners", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.removeMemberFailed", nil)
				return
			}
			if len(owners) <= 1 {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.cannotRemoveLastOwner", nil)
				return
			}
		}

		// Only owner can remove admins
		if membership.Role == models.OrgRoleAdmin && !tc.HasRole(models.OrgRoleOwner) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.onlyOwnerCanRemoveAdmins", nil)
			return
		}

		// Users can remove themselves
		if userID != currentUser.ID && !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "organization.errors.cannotRemoveOtherMembers", nil)
			return
		}

		if err := m.orgMembershipRepo.Delete(r.Context(), membership.ID); err != nil {
			m.logger.Error("Failed to remove member", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.removeMemberFailed", nil)
			return
		}

		// Audit log: member removed
		m.logAdminAction(r, "member.removed", "membership", membership.ID, map[string]any{
			"organizationId": tc.Organization.ID,
			"userId":         userID,
			"role":           string(membership.Role),
		})

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message": "member removed",
		})
	}
}

// handleAcceptInvitation accepts an organization invitation
func (m *Manager) handleAcceptInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")

		membership, err := m.orgMembershipRepo.AcceptInvitation(r.Context(), token)
		if err != nil {
			m.logger.Error("Failed to accept invitation", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.invalidOrExpiredInvitation", nil)
			return
		}
		if membership == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "organization.errors.invalidOrExpiredInvitation", nil)
			return
		}

		// Auto-assign default RBAC role on acceptance
		assignDefaultRole(r, m.rbacRepo, membership.UserID, membership.OrganizationID, nil)

		// Get organization details
		org, err := m.orgRepo.GetByID(r.Context(), membership.OrganizationID)
		if err != nil || org == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "organization.errors.getFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message": "invitation accepted",
			"organization": map[string]any{
				"id":   org.ID,
				"name": org.Name,
				"slug": org.Slug,
			},
			"role": membership.Role,
		})
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// generateSlug creates a URL-safe slug from a name
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
