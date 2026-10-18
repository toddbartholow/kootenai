// Package users provides HTTP handlers for user management (CRUD, roles, passwords).
package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// EmailSender interface for sending emails (allows dependency injection)
type EmailSender interface {
	SendPasswordResetEmail(ctx context.Context, email, userName, resetToken string) error
}

// Manager manages user operations (CRUD, roles, passwords)
type Manager struct {
	userRepo          repositories.UserRepository
	passwordResetRepo repositories.PasswordResetTokenRepository
	emailSender       EmailSender
	logger            *slog.Logger
	responder         *httputil.Responder
}

// Config configures Manager
type Config struct {
	UserRepo          repositories.UserRepository
	PasswordResetRepo repositories.PasswordResetTokenRepository
	EmailSender       EmailSender
	Logger            *slog.Logger
	Responder         *httputil.Responder
}

// NewManager creates a new Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		userRepo:          cfg.UserRepo,
		passwordResetRepo: cfg.PasswordResetRepo,
		emailSender:       cfg.EmailSender,
		logger:            cfg.Logger,
		responder:         cfg.Responder,
	}
}

// SetupRoutes registers user-related routes on the router
func (m *Manager) SetupRoutes(r chi.Router) {
	// User management routes
	r.Route("/users", func(r chi.Router) {
		r.Get("/", m.handleListUsers())
		r.Post("/", m.handleCreateUser())
		r.Get("/{userID}", m.handleGetUser())
		r.Put("/{userID}", m.handleUpdateUser())
		r.Delete("/{userID}", m.handleDeleteUser())
		r.Put("/{userID}/role", m.handleUpdateUserRole())
		r.Put("/{userID}/status", m.handleUpdateUserStatus())
	})

	// Password management routes (authenticated users)
	r.Put("/password/change", m.handleChangePassword())
	r.Put("/users/{userID}/password/reset", m.handleAdminResetPassword())
	r.Put("/users/{userID}/password", m.handleAdminSetPassword())
}

// SetupPublicRoutes registers password recovery routes (no auth required)
func (m *Manager) SetupPublicRoutes(r chi.Router) {
	r.Post("/auth/password/reset-request", m.HandlePasswordResetRequest())
	r.Post("/auth/password/reset-confirm", m.HandlePasswordResetConfirm())
}

// -----------------------------------------------------------------------------
// Validation helpers
// -----------------------------------------------------------------------------

// validRoles defines the set of accepted user roles.
var validRoles = map[string]bool{"student": true, "instructor": true, "admin": true}

func isValidRole(role string) bool {
	return validRoles[role]
}

// -----------------------------------------------------------------------------
// User CRUD Handlers
// -----------------------------------------------------------------------------

// handleListUsers returns a paginated list of users
func (m *Manager) handleListUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if user is admin
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		// Parse query parameters
		opts := repositories.UserListOptions{}

		if role := r.URL.Query().Get("role"); role != "" {
			opts.Role = role
		}
		if active := r.URL.Query().Get("active"); active != "" {
			isActive := active == "true"
			opts.IsActive = &isActive
		}
		if search := r.URL.Query().Get("search"); search != "" {
			opts.Search = search
		}
		if limit := r.URL.Query().Get("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil && l > 0 {
				opts.Limit = l
			}
		}
		if offset := r.URL.Query().Get("offset"); offset != "" {
			if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
				opts.Offset = o
			}
		}

		// Default limit
		if opts.Limit == 0 {
			opts.Limit = 50
		}

		users, total, err := m.userRepo.List(r.Context(), opts)
		if err != nil {
			m.logger.Error("Failed to list users", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.listFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"users":  users,
			"total":  total,
			"limit":  opts.Limit,
			"offset": opts.Offset,
		})
	}
}

// handleCreateUser creates a new user
func (m *Manager) handleCreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if user is admin
		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}
		if !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		var req struct {
			Username    string `json:"username"`
			Email       string `json:"email"`
			DisplayName string `json:"displayName"`
			Role        string `json:"role"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Validate required fields
		if req.Username == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.usernameRequired", nil)
			return
		}

		// Check if username already exists
		existing, err := m.userRepo.GetByUsername(r.Context(), req.Username)
		if err != nil {
			m.logger.Error("Failed to check existing user", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.createFailed", nil)
			return
		}
		if existing != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "user.errors.usernameAlreadyExists", nil)
			return
		}

		// Check if email already exists (if provided)
		if req.Email != "" {
			existingEmail, err := m.userRepo.GetByEmail(r.Context(), req.Email)
			if err != nil {
				m.logger.Error("Failed to check existing email", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.createFailed", nil)
				return
			}
			if existingEmail != nil {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "user.errors.emailAlreadyExists", nil)
				return
			}
		}

		// Validate role
		if req.Role == "" {
			req.Role = "student"
		}
		if !isValidRole(req.Role) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.invalidRoleDetail", nil)
			return
		}

		// Create user
		user := &models.User{
			ID:          uuid.New().String(),
			ExternalID:  fmt.Sprintf("local-%s", uuid.New().String()[:8]), // Generate unique external ID for local users
			Username:    req.Username,
			Email:       req.Email,
			DisplayName: req.DisplayName,
			Role:        req.Role,
			IsActive:    true,
		}

		if user.DisplayName == "" {
			user.DisplayName = user.Username
		}

		if err := m.userRepo.Create(r.Context(), user); err != nil {
			m.logger.Error("Failed to create user", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.createFailed", nil)
			return
		}

		m.logger.Info("User created", "userId", user.ID, "username", user.Username, "createdBy", currentUser.ID)
		m.responder.JSONResponse(w, http.StatusCreated, user)
	}
}

// handleGetUser retrieves a single user by ID
func (m *Manager) handleGetUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.idRequired", nil)
			return
		}

		// Check if user is admin or requesting their own info
		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}

		// Users can view their own profile, admins can view anyone
		if currentUser.ID != userID && !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		user, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.getFailed", nil)
			return
		}
		if user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "user.errors.notFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, user)
	}
}

// handleUpdateUser updates a user's profile
func (m *Manager) handleUpdateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.idRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}

		isAdmin := serverutil.IsAdminUser(currentUser)

		// Users can update their own profile (limited fields), admins can update anyone
		if currentUser.ID != userID && !isAdmin {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		// Get existing user
		user, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateFailed", nil)
			return
		}
		if user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "user.errors.notFound", nil)
			return
		}

		var req struct {
			Email       *string `json:"email"`
			DisplayName *string `json:"displayName"`
			Role        *string `json:"role"`
			IsActive    *bool   `json:"isActive"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Apply updates
		if req.Email != nil {
			// Check for duplicate email
			if *req.Email != "" && *req.Email != user.Email {
				existingEmail, err := m.userRepo.GetByEmail(r.Context(), *req.Email)
				if err != nil {
					m.logger.Error("Failed to check existing email", "error", err)
					m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateFailed", nil)
					return
				}
				if existingEmail != nil && existingEmail.ID != userID {
					m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "user.errors.emailAlreadyExists", nil)
					return
				}
			}
			user.Email = *req.Email
		}
		if req.DisplayName != nil {
			user.DisplayName = *req.DisplayName
		}

		// Only admins can change role and status
		if isAdmin {
			if req.Role != nil {
				if !isValidRole(*req.Role) {
					m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.invalidRole", nil)
					return
				}
				user.Role = *req.Role
			}
			if req.IsActive != nil {
				user.IsActive = *req.IsActive
			}
		}

		if err := m.userRepo.Update(r.Context(), user); err != nil {
			m.logger.Error("Failed to update user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateFailed", nil)
			return
		}

		m.logger.Info("User updated", "userId", userID, "updatedBy", currentUser.ID)
		m.responder.JSONResponse(w, http.StatusOK, user)
	}
}

// handleDeleteUser deletes a user
func (m *Manager) handleDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.idRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}
		if !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		// Prevent self-deletion
		if currentUser.ID == userID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.cannotDeleteOwnAccount", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		// Check if user exists
		user, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.deleteFailed", nil)
			return
		}
		if user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "user.errors.notFound", nil)
			return
		}

		if err := m.userRepo.Delete(r.Context(), userID); err != nil {
			m.logger.Error("Failed to delete user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.deleteFailed", nil)
			return
		}

		m.logger.Info("User deleted", "userId", userID, "username", user.Username, "deletedBy", currentUser.ID)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"message": "user deleted"})
	}
}

// handleUpdateUserRole updates a user's role (admin only)
func (m *Manager) handleUpdateUserRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.idRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}
		if !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		var req struct {
			Role string `json:"role"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if !isValidRole(req.Role) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.invalidRoleDetail", nil)
			return
		}

		user, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateRoleFailed", nil)
			return
		}
		if user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "user.errors.notFound", nil)
			return
		}

		user.Role = req.Role

		if err := m.userRepo.Update(r.Context(), user); err != nil {
			m.logger.Error("Failed to update user role", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateRoleFailed", nil)
			return
		}

		m.logger.Info("User role updated", "userId", userID, "newRole", req.Role, "updatedBy", currentUser.ID)
		m.responder.JSONResponse(w, http.StatusOK, user)
	}
}

// handleUpdateUserStatus activates or deactivates a user (admin only)
func (m *Manager) handleUpdateUserStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.idRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}
		if !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		// Prevent self-deactivation
		if currentUser.ID == userID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "user.errors.cannotChangeOwnStatus", nil)
			return
		}

		if m.userRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
			return
		}

		var req struct {
			IsActive bool `json:"isActive"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		user, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateStatusFailed", nil)
			return
		}
		if user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "user.errors.notFound", nil)
			return
		}

		user.IsActive = req.IsActive

		if err := m.userRepo.Update(r.Context(), user); err != nil {
			m.logger.Error("Failed to update user status", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateStatusFailed", nil)
			return
		}

		status := "activated"
		if !req.IsActive {
			status = "deactivated"
		}

		m.logger.Info("User status updated", "userId", userID, "status", status, "updatedBy", currentUser.ID)
		m.responder.JSONResponse(w, http.StatusOK, user)
	}
}

// -----------------------------------------------------------------------------
// Password Management Handlers
// -----------------------------------------------------------------------------

// Password reset token expiry duration
const passwordResetTokenExpiry = 1 * time.Hour

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashToken creates a SHA256 hash of a token for storage
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// handleChangePassword allows authenticated users to change their own password
func (m *Manager) handleChangePassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}

		var req struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.CurrentPassword == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.currentPasswordRequired", nil)
			return
		}

		if req.NewPassword == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.newPasswordRequired", nil)
			return
		}

		// Get user with password hash
		user, err := m.userRepo.GetByIDWithPassword(r.Context(), currentUser.ID)
		if err != nil {
			m.logger.Error("Failed to get user for password change", "error", err, "userId", currentUser.ID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.changeFailed", nil)
			return
		}
		if user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "password.errors.userNotFound", nil)
			return
		}

		// Verify current password
		if err := auth.VerifyPassword(req.CurrentPassword, user.PasswordHash); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "password.errors.currentPasswordIncorrect", nil)
			return
		}

		// Hash new password (this also validates password strength)
		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			if msgID := auth.PasswordValidationMessageID(err); msgID != "" {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, msgID, map[string]any{"Min": auth.MinPasswordLength})
			} else {
				m.responder.SafeErrorResponse(w, err, "hash password")
			}
			return
		}

		// Update password (with must_change_password = false since they just changed it)
		if err := m.userRepo.UpdatePassword(r.Context(), user.ID, newHash, false); err != nil {
			m.logger.Error("Failed to update password", "error", err, "userId", user.ID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.updateFailed", nil)
			return
		}

		m.logger.Info("User changed password", "userId", user.ID)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{
			"message": "password updated successfully",
		})
	}
}

// handleAdminResetPassword allows admins to reset a user's password
func (m *Manager) handleAdminResetPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.userIdRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}
		if !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		var req struct {
			Password         string `json:"password"`         // Optional: specific password to set
			GeneratePassword bool   `json:"generatePassword"` // If true, generate a random password
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Check target user exists
		targetUser, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user for password reset", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.resetFailed", nil)
			return
		}
		if targetUser == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "password.errors.userNotFound", nil)
			return
		}

		var password string
		var passwordHash string

		if req.GeneratePassword {
			// Generate a secure random password
			password, err = auth.GenerateRandomPassword(16)
			if err != nil {
				m.logger.Error("Failed to generate random password", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.generateFailed", nil)
				return
			}
		} else if req.Password != "" {
			password = req.Password
		} else {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.eitherPasswordOrGenerate", nil)
			return
		}

		// Hash password (this also validates password strength)
		passwordHash, err = auth.HashPassword(password)
		if err != nil {
			if msgID := auth.PasswordValidationMessageID(err); msgID != "" {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, msgID, map[string]any{"Min": auth.MinPasswordLength})
			} else {
				m.responder.SafeErrorResponse(w, err, "hash password")
			}
			return
		}

		// Update password with must_change_password = true so user must change on next login
		if err := m.userRepo.UpdatePassword(r.Context(), userID, passwordHash, true); err != nil {
			m.logger.Error("Failed to reset password", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.resetFailed", nil)
			return
		}

		m.logger.Info("Admin reset user password", "userId", userID, "adminId", currentUser.ID)

		response := map[string]any{
			"message":            "password reset successfully",
			"mustChangePassword": true,
		}

		// Only return the generated password, never a manually-set password
		// This is a security measure - admins shouldn't see passwords they set
		if req.GeneratePassword {
			response["temporaryPassword"] = password
		}

		m.responder.JSONResponse(w, http.StatusOK, response)
	}
}

// handleAdminSetPassword allows admins to set a user's password (with password in response for initial setup)
func (m *Manager) handleAdminSetPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.userIdRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}
		if !serverutil.IsAdminUser(currentUser) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
			return
		}

		var req struct {
			Password           string `json:"password"`
			MustChangePassword bool   `json:"mustChangePassword"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Password == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.passwordRequired", nil)
			return
		}

		// Check target user exists
		targetUser, err := m.userRepo.GetByID(r.Context(), userID)
		if err != nil {
			m.logger.Error("Failed to get user for password set", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.setFailed", nil)
			return
		}
		if targetUser == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "password.errors.userNotFound", nil)
			return
		}

		// Hash password (this also validates password strength)
		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			if msgID := auth.PasswordValidationMessageID(err); msgID != "" {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, msgID, map[string]any{"Min": auth.MinPasswordLength})
			} else {
				m.responder.SafeErrorResponse(w, err, "hash password")
			}
			return
		}

		// Update password
		if err := m.userRepo.UpdatePassword(r.Context(), userID, passwordHash, req.MustChangePassword); err != nil {
			m.logger.Error("Failed to set password", "error", err, "userId", userID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.setFailed", nil)
			return
		}

		m.logger.Info("Admin set user password", "userId", userID, "adminId", currentUser.ID, "mustChange", req.MustChangePassword)

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message":            "password set successfully",
			"mustChangePassword": req.MustChangePassword,
		})
	}
}

// -----------------------------------------------------------------------------
// Password Recovery Handlers (unauthenticated)
// -----------------------------------------------------------------------------

// HandlePasswordResetRequest initiates a password reset flow.
// This endpoint does NOT require authentication.
// Exported so AuthManager can wire it into the public route tree.
func (m *Manager) HandlePasswordResetRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Email == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.emailRequired", nil)
			return
		}

		// Always return success to prevent email enumeration attacks
		// We log the actual result for debugging
		defer func() {
			m.responder.JSONResponse(w, http.StatusOK, map[string]string{
				"message": "If an account with that email exists, a password reset link has been sent.",
			})
		}()

		// Look up user by email
		user, err := m.userRepo.GetByEmail(r.Context(), req.Email)
		if err != nil {
			m.logger.Error("Failed to look up user for password reset", "error", err, "email", req.Email)
			return
		}
		if user == nil {
			m.logger.Info("Password reset requested for non-existent email", "email", req.Email)
			return
		}

		// Check if user is active
		if !user.IsActive {
			m.logger.Info("Password reset requested for inactive user", "userId", user.ID)
			return
		}

		// Generate secure token
		token, err := generateSecureToken(32) // 64 hex characters
		if err != nil {
			m.logger.Error("Failed to generate reset token", "error", err)
			return
		}

		// Create token record
		resetToken := &models.PasswordResetToken{
			ID:        uuid.New().String(),
			UserID:    user.ID,
			TokenHash: hashToken(token),
			ExpiresAt: time.Now().Add(passwordResetTokenExpiry),
			CreatedAt: time.Now(),
		}

		// Delete any existing tokens for this user first
		if err := m.passwordResetRepo.DeleteByUserID(r.Context(), user.ID); err != nil {
			m.logger.Warn("Failed to delete old reset tokens", "error", err, "userId", user.ID)
			// Continue: old token cleanup is best-effort; new token creation is the critical path
		}

		// Store new token
		if err := m.passwordResetRepo.Create(r.Context(), resetToken); err != nil {
			m.logger.Error("Failed to store reset token", "error", err, "userId", user.ID)
			return
		}

		// Send email with reset link
		if m.emailSender != nil {
			if err := m.emailSender.SendPasswordResetEmail(r.Context(), user.Email, user.DisplayName, token); err != nil {
				m.logger.Error("Failed to send password reset email", "error", err, "userId", user.ID)
				// Don't return error to user - we still created the token
				// They can request again if email fails
			} else {
				m.logger.Info("Password reset email sent", "userId", user.ID, "email", user.Email)
			}
		} else {
			// No email sender configured - log warning (NEVER log the actual token)
			env := os.Getenv("ENV")
			if env == "development" || env == "dev" {
				// Only show token hint in development for debugging
				m.logger.Warn("No email sender configured",
					"userId", user.ID,
					"email", user.Email,
					"hint", "Check console for reset link or configure email sender",
					"tokenPreview", token[:8]+"...", // Only first 8 chars for debugging
					"expiresAt", resetToken.ExpiresAt,
				)
			} else {
				// In non-dev environments, just warn about missing email config
				m.logger.Warn("Password reset requested but no email sender configured",
					"userId", user.ID,
					"action", "Configure EMAIL_* environment variables",
				)
			}
		}
	}
}

// HandlePasswordResetConfirm completes a password reset using a token.
// This endpoint does NOT require authentication.
// Exported so AuthManager can wire it into the public route tree.
func (m *Manager) HandlePasswordResetConfirm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token       string `json:"token"`
			NewPassword string `json:"newPassword"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Token == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.tokenRequired", nil)
			return
		}

		if req.NewPassword == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.newPasswordRequired", nil)
			return
		}

		// Look up token by hash
		tokenHash := hashToken(req.Token)
		resetToken, err := m.passwordResetRepo.GetByTokenHash(r.Context(), tokenHash)
		if err != nil {
			m.logger.Error("Failed to look up reset token", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.verifyTokenFailed", nil)
			return
		}

		// Validate token
		if resetToken == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.invalidOrExpiredToken", nil)
			return
		}

		if resetToken.UsedAt != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.tokenAlreadyUsed", nil)
			return
		}

		if time.Now().After(resetToken.ExpiresAt) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "password.errors.tokenExpired", nil)
			return
		}

		// Validate password strength and hash
		passwordHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			if msgID := auth.PasswordValidationMessageID(err); msgID != "" {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, msgID, map[string]any{"Min": auth.MinPasswordLength})
			} else {
				m.responder.SafeErrorResponse(w, err, "hash password")
			}
			return
		}

		// Update user's password (no must_change_password since they just set it)
		if err := m.userRepo.UpdatePassword(r.Context(), resetToken.UserID, passwordHash, false); err != nil {
			m.logger.Error("Failed to update password after reset", "error", err, "userId", resetToken.UserID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "password.errors.updateFailed", nil)
			return
		}

		// Mark token as used
		if err := m.passwordResetRepo.MarkUsed(r.Context(), resetToken.ID); err != nil {
			m.logger.Warn("Failed to mark token as used", "error", err, "tokenId", resetToken.ID)
			// Continue: password was already updated; token marking is cleanup
		}

		// Delete all tokens for this user (cleanup)
		if err := m.passwordResetRepo.DeleteByUserID(r.Context(), resetToken.UserID); err != nil {
			m.logger.Warn("Failed to delete old tokens after reset", "error", err, "userId", resetToken.UserID)
		}

		m.logger.Info("Password reset completed", "userId", resetToken.UserID)

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{
			"message": "password has been reset successfully",
		})
	}
}
