// Package server provides the HTTP server and API routes
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	authldap "github.com/toddbartholow/kootenai/api/internal/auth/ldap"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
	"github.com/toddbartholow/kootenai/api/internal/models"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
	"github.com/toddbartholow/kootenai/api/internal/server/users"
)

// -----------------------------------------------------------------------------
// Auth Manager
// -----------------------------------------------------------------------------

// MiddlewareFunc is the chi-compatible middleware signature. We accept it as
// a plain function value so AuthManager does not need to know whether the
// limiter is Redis-backed, in-memory, or absent.
type MiddlewareFunc = func(http.Handler) http.Handler

// systemOrgID is the well-known ID of the system organization created by the
// database seed. Users are auto-provisioned into it so they can see global
// resources (labs, pathways) immediately after their first login.
const systemOrgID = "00000000-0000-0000-0000-000000000000"

// AuthManager manages authentication operations (login, logout, token refresh)
type AuthManager struct {
	authService       *auth.Service
	userRepo          repositories.UserRepository
	orgMembershipRepo repositories.OrganizationMembershipRepository
	tokenBlacklist    redisclient.TokenBlacklistClient
	cookieCfg         auth.CookieConfig
	ldapClient        *authldap.Client
	logger            *slog.Logger
	responder         *httputil.Responder
	userMgr           *users.Manager
	loginRL           MiddlewareFunc
	passwordResetRL   MiddlewareFunc
}

// AuthManagerConfig configures AuthManager
type AuthManagerConfig struct {
	AuthService       *auth.Service
	UserRepo          repositories.UserRepository
	OrgMembershipRepo repositories.OrganizationMembershipRepository
	TokenBlacklist    redisclient.TokenBlacklistClient
	CookieConfig      auth.CookieConfig
	LDAPClient        *authldap.Client
	Logger            *slog.Logger
	Responder         *httputil.Responder

	// UserMgr is used to wire password-reset handlers into the public auth
	// route tree. Optional — when nil, password reset routes are skipped.
	UserMgr *users.Manager

	// LoginRateLimit is applied to POST /auth/login. Pass nil to disable.
	LoginRateLimit MiddlewareFunc

	// PasswordResetRateLimit is applied to both /auth/password/reset-request
	// and /auth/password/reset-confirm. Pass nil to disable.
	PasswordResetRateLimit MiddlewareFunc
}

// NewAuthManager creates a new AuthManager
func NewAuthManager(cfg AuthManagerConfig) *AuthManager {
	return &AuthManager{
		authService:       cfg.AuthService,
		userRepo:          cfg.UserRepo,
		orgMembershipRepo: cfg.OrgMembershipRepo,
		tokenBlacklist:    cfg.TokenBlacklist,
		cookieCfg:         cfg.CookieConfig,
		ldapClient:        cfg.LDAPClient,
		logger:            cfg.Logger,
		responder:         cfg.Responder,
		userMgr:           cfg.UserMgr,
		loginRL:           cfg.LoginRateLimit,
		passwordResetRL:   cfg.PasswordResetRateLimit,
	}
}

// SetupPublicRoutes registers the public (unauthenticated) auth and
// password-recovery endpoints. Login and password-reset endpoints carry
// rate-limit middleware; refresh and logout do not (they identify the caller
// via the bearer token itself).
func (m *AuthManager) SetupPublicRoutes(r chi.Router) {
	// Login: rate-limited when a limiter was supplied.
	if m.loginRL != nil {
		r.With(m.loginRL).Post("/auth/login", m.handleLogin)
	} else {
		r.Post("/auth/login", m.handleLogin)
	}

	// Refresh and logout are authenticated-via-body/header; no extra rate limit.
	r.Post("/auth/refresh", m.handleRefreshToken)
	r.Post("/auth/logout", m.handleLogout)

	// Password reset endpoints — delegated to UserManager since UserManager
	// owns password storage. The rate limit applies to both endpoints
	// to prevent both enumeration and email-bombing.
	if m.userMgr != nil {
		pr := m.passwordResetRL
		wrap := func(next http.HandlerFunc) http.Handler {
			if pr == nil {
				return next
			}
			return pr(next)
		}
		r.Method(http.MethodPost, "/auth/password/reset-request", wrap(m.userMgr.HandlePasswordResetRequest()))
		r.Method(http.MethodPost, "/auth/password/reset-confirm", wrap(m.userMgr.HandlePasswordResetConfirm()))
	}
}

// -----------------------------------------------------------------------------
// Auth Handlers
// -----------------------------------------------------------------------------

// ErrorResponse is an alias for serverutil.ErrorResponse.
type ErrorResponse = serverutil.ErrorResponse

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" example:"Password123!"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token              string    `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
	User               auth.User `json:"user"`
	MustChangePassword bool      `json:"mustChangePassword" example:"false"`
}

// handleLogin handles user login with password validation
// @Summary Login
// @Description Authenticate user with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid credentials"
// @Failure 503 {object} ErrorResponse "Auth not configured"
// @Router /auth/login [post]
func (m *AuthManager) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if m.authService == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "auth.errors.notConfigured", nil)
		return
	}

	req, validationErrors := decodeAndValidate[LoginRequest](r)
	if validationErrors != nil {
		details := make([]httputil.ValidationError, len(validationErrors))
		for i, e := range validationErrors {
			details[i] = httputil.ValidationError{Field: e.Field, Message: e.Message}
		}
		m.responder.RespondWithValidationError(w, details)
		return
	}

	// Demo mode: allow login without password for development
	if m.authService.IsDemoMode() && req.Password == "" {
		m.handleDemoLogin(w, req.Email)
		return
	}

	// Production mode: require password
	if req.Password == "" {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "auth.errors.passwordRequired", nil)
		return
	}

	if m.userRepo == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "auth.errors.userDatabaseUnavailable", nil)
		return
	}

	// Look up user by email
	dbUser, err := m.userRepo.GetByEmailForAuth(ctx, req.Email)
	if err != nil {
		m.logger.Error("Failed to get user for login", "error", err, "email", req.Email)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "auth.errors.loginFailed", nil)
		return
	}

	// --- LDAP auth path ---
	if m.ldapClient != nil && m.shouldUseLDAP(req.Email, dbUser) {
		ldapInfo, ldapErr := m.ldapClient.Authenticate(ctx, m.ldapUsername(req.Email), req.Password)
		if ldapErr != nil {
			// If "auth first" mode, fall through to local bcrypt if user has a password
			if m.ldapClient.Config().AuthFirst && dbUser != nil && dbUser.PasswordHash != "" {
				m.logger.Info("LDAP auth failed, falling back to local", "email", req.Email, "error", ldapErr)
				// fall through to local auth below
			} else {
				m.logger.Info("LDAP login failed", "email", req.Email, "error", ldapErr)
				m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.invalidCredentials", nil)
				return
			}
		} else {
			// LDAP succeeded — find or provision user, then complete login
			user, provErr := m.findOrProvisionLDAPUser(ctx, ldapInfo, req.Email)
			if provErr != nil {
				m.logger.Error("Failed to provision LDAP user", "error", provErr, "email", req.Email)
				m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "auth.errors.loginFailed", nil)
				return
			}
			m.completeLoginForUser(w, r, user)
			return
		}
	}

	// --- Local auth path ---

	// User not found - still verify a dummy hash to prevent timing attacks
	if dbUser == nil {
		_ = auth.VerifyPassword(req.Password, auth.DummyHash)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.invalidCredentials", nil)
		return
	}

	// Verify password
	if err := auth.VerifyPassword(req.Password, dbUser.PasswordHash); err != nil {
		m.logger.Info("Failed login attempt", "email", req.Email)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.invalidCredentials", nil)
		return
	}

	m.completeLoginForUser(w, r, dbUser)
}

// handleDemoLogin handles login in demo mode without password
func (m *AuthManager) handleDemoLogin(w http.ResponseWriter, email string) {
	// Use deterministic UUID v5 from email so different demo users get different IDs
	user := &auth.User{
		ID:                    uuid.NewSHA1(uuid.NameSpaceDNS, []byte(email)).String(),
		Email:                 email,
		Name:                  "Demo User",
		Roles:                 []string{m.authService.DemoRole()}, // Use configured demo role (default: student)
		DefaultOrganizationID: systemOrgID,                        // System organization from seed
	}

	// Generate JWT token
	token, err := m.authService.GenerateToken(user)
	if err != nil {
		m.logger.Error("Failed to generate token", "error", err, "email", email)
		// Demo login has no *http.Request in scope; falls back to English when
		// no localizer is on the context, which is fine — demo mode is dev-only.
		m.responder.LocalizedErrorResponse(context.Background(), w, http.StatusInternalServerError, "auth.errors.generateTokenFailed", nil)
		return
	}

	m.setAuthCookies(w, nil, token)

	m.logger.Info("User logged in (demo mode)", "email", email, "userId", user.ID)
	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"token":              token,
		"user":               user,
		"mustChangePassword": false,
	})
}

// RefreshTokenRequest represents the token refresh request body
type RefreshTokenRequest struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
}

// RefreshTokenResponse represents the token refresh response
type RefreshTokenResponse struct {
	Token string    `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
	User  auth.User `json:"user"`
}

// handleRefreshToken refreshes an existing JWT token
// @Summary Refresh Token
// @Description Refresh an existing JWT token. The old token is blacklisted upon successful refresh.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Token to refresh"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid token"
// @Failure 503 {object} ErrorResponse "Auth not configured"
// @Router /auth/refresh [post]
func (m *AuthManager) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if m.authService == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "auth.errors.notConfigured", nil)
		return
	}

	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "auth.errors.invalidRequestBody", nil)
		return
	}

	// Fall back to cookie if no token in body
	if req.Token == "" && m.cookieCfg.Enabled {
		req.Token = auth.TokenFromCookie(r, m.cookieCfg)
	}
	if req.Token == "" {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "auth.errors.tokenRequired", nil)
		return
	}

	// Check if token is blacklisted
	if m.tokenBlacklist != nil {
		tokenHash := auth.HashToken(req.Token)
		blacklisted, err := m.tokenBlacklist.IsBlacklisted(ctx, tokenHash)
		if err != nil {
			m.logger.Error("Token blacklist check failed, rejecting refresh", "error", err)
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "auth.errors.serviceUnavailable", nil)
			return
		} else if blacklisted {
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.tokenRevoked", nil)
			return
		}
	}

	// Validate existing token
	claims, err := m.authService.ValidateToken(req.Token)
	if err != nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.invalidToken", nil)
		return
	}

	// Create user from claims
	user := &auth.User{
		ID:    claims.UserID,
		Email: claims.Email,
		Name:  claims.Name,
		Roles: claims.Roles,
	}

	// Blacklist the old token BEFORE generating a new one to prevent
	// a dual-valid-token window if blacklisting fails after token generation
	if m.tokenBlacklist != nil {
		tokenHash := auth.HashToken(req.Token)
		ttl := claims.RemainingTTL()
		if err := m.tokenBlacklist.Blacklist(ctx, tokenHash, ttl); err != nil {
			m.logger.Error("Failed to blacklist old token during refresh", "error", err, "userId", user.ID)
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "auth.errors.serviceUnavailable", nil)
			return
		}
	}

	// Generate new token only after old token is safely blacklisted
	newToken, err := m.authService.GenerateToken(user)
	if err != nil {
		// Old token is already blacklisted — user must re-authenticate
		m.logger.Error("Failed to generate new token after blacklisting old", "error", err, "userId", user.ID)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "auth.errors.refreshTokenFailed", nil)
		return
	}

	m.setAuthCookies(w, r, newToken)

	m.logger.Info("Token refreshed", "userId", user.ID)
	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"token": newToken,
		"user":  user,
	})
}

// CurrentUserResponse represents the current user response
type CurrentUserResponse struct {
	User auth.User `json:"user"`
}

// handleGetCurrentUser returns the current authenticated user
// @Summary Get Current User
// @Description Get the currently authenticated user's information
// @Tags auth
// @Produce json
// @Success 200 {object} CurrentUserResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Security BearerAuth
// @Router /auth/me [get]
func (m *AuthManager) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "auth.errors.notAuthenticated", nil)
		return
	}

	// Pull the preferred_locale alongside the auth claims so the UI can hydrate
	// the locale store without a second round-trip. A missing preference is
	// represented as a JSON null, which the client treats as "fall back to
	// Accept-Language".
	resp := map[string]any{"user": user}
	if m.userRepo != nil {
		if locale, err := m.userRepo.GetPreferredLocale(r.Context(), user.ID); err == nil {
			resp["preferredLocale"] = locale
		} else {
			m.logger.Warn("failed to read preferred_locale", "user_id", user.ID, "error", err)
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, resp)
}

// PreferredLocaleRequest is the JSON body for the PUT endpoint. A nil or
// absent value clears the stored preference.
type PreferredLocaleRequest struct {
	PreferredLocale *string `json:"preferredLocale"`
}

// supportedLocaleTags mirrors web/src/locales/index.ts. Keep in sync when new
// catalogs land. The DB CHECK constraint validates BCP-47 shape, but the
// app narrows to locales we actually ship.
var supportedLocaleTags = map[string]struct{}{
	"en": {},
	"es": {},
}

// handleUpdatePreferredLocale stores the authenticated user's preferred
// locale. Body: {"preferredLocale": "es"} or {"preferredLocale": null} to
// clear. Rejects unknown tags with 400.
//
// @Summary  Update preferred locale
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body PreferredLocaleRequest true "Locale preference"
// @Success  204
// @Failure  400 {object} ErrorResponse
// @Failure  401 {object} ErrorResponse
// @Security BearerAuth
// @Router   /auth/me/preferred-locale [put]
func (m *AuthManager) handleUpdatePreferredLocale(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "auth.errors.notAuthenticated", nil)
		return
	}
	if m.userRepo == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "user.errors.serviceUnavailable", nil)
		return
	}

	var req PreferredLocaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
		return
	}

	if req.PreferredLocale != nil {
		if _, ok := supportedLocaleTags[*req.PreferredLocale]; !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "auth.errors.unsupportedLocale", nil)
			return
		}
	}

	if err := m.userRepo.UpdatePreferredLocale(r.Context(), user.ID, req.PreferredLocale); err != nil {
		m.logger.Error("failed to update preferred_locale", "user_id", user.ID, "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "user.errors.updateFailed", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleLogout logs out the current user by blacklisting their token
// @Summary Logout
// @Description Logout the current user and invalidate their JWT token
// @Tags auth
// @Produce json
// @Success 200 {object} LogoutResponse "Successfully logged out"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Security BearerAuth
// @Router /auth/logout [post]
func (m *AuthManager) handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract token: try Bearer header first, then cookie
	var tokenStr string
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.invalidAuthorizationHeader", nil)
			return
		}
		tokenStr = parts[1]
	} else if m.cookieCfg.Enabled {
		tokenStr = auth.TokenFromCookie(r, m.cookieCfg)
	}
	if tokenStr == "" {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.notAuthenticated", nil)
		return
	}

	// Validate the token to get claims (for TTL and user info)
	if m.authService == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "auth.errors.notConfigured", nil)
		return
	}

	claims, err := m.authService.ValidateToken(tokenStr)
	if err != nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "auth.errors.invalidToken", nil)
		return
	}

	// Blacklist the token so it cannot be reused
	if m.tokenBlacklist != nil {
		tokenHash := auth.HashToken(tokenStr)
		ttl := claims.RemainingTTL()
		if err := m.tokenBlacklist.Blacklist(ctx, tokenHash, ttl); err != nil {
			m.logger.Error("Failed to blacklist token on logout", "error", err, "userId", claims.UserID)
			m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "auth.errors.logoutFailed", nil)
			return
		}
	}

	m.clearAuthCookies(w, r)

	m.logger.Info("User logged out", "userId", claims.UserID)
	m.responder.JSONResponse(w, http.StatusOK, map[string]string{
		"message": appi18n.Localize(ctx, "auth.logoutSuccess", nil),
	})
}

// setAuthCookies sets both the HttpOnly auth cookie and the JS-readable CSRF
// cookie when cookie mode is enabled. r may be nil (demo login path).
func (m *AuthManager) setAuthCookies(w http.ResponseWriter, r *http.Request, token string) {
	if !m.cookieCfg.Enabled {
		return
	}
	auth.SetAuthCookie(w, r, token, m.cookieCfg)
	csrfToken, err := auth.GenerateCSRFToken()
	if err != nil {
		m.logger.Error("Failed to generate CSRF token", "error", err)
		return
	}
	auth.SetCSRFCookie(w, r, csrfToken, m.cookieCfg)
}

// clearAuthCookies expires both auth and CSRF cookies when cookie mode is enabled.
func (m *AuthManager) clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	if !m.cookieCfg.Enabled {
		return
	}
	auth.ClearAuthCookie(w, r, m.cookieCfg)
	auth.ClearCSRFCookie(w, r, m.cookieCfg)
}

// ---------------------------------------------------------------------------
// LDAP authentication helpers
// ---------------------------------------------------------------------------

// shouldUseLDAP determines whether LDAP auth should be attempted for this login.
func (m *AuthManager) shouldUseLDAP(email string, dbUser *models.User) bool {
	// 1. User already linked to LDAP
	if dbUser != nil && strings.HasPrefix(dbUser.ExternalID, "ldap:") {
		return true
	}
	// 2. Email domain matches LDAP default domain
	cfg := m.ldapClient.Config()
	if cfg.DefaultDomain != "" {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 && strings.EqualFold(parts[1], cfg.DefaultDomain) {
			return true
		}
	}
	// 3. Auth-first mode: try LDAP before local
	if cfg.AuthFirst {
		return true
	}
	return false
}

// ldapUsername extracts the username portion of an email for LDAP search.
// If the LDAP search filter uses uid, we strip the domain.
func (m *AuthManager) ldapUsername(email string) string {
	if i := strings.IndexByte(email, '@'); i >= 0 {
		return email[:i]
	}
	return email
}

// findOrProvisionLDAPUser finds an existing user by LDAP external ID or email,
// or creates a new user, and guarantees the user holds a system-organization
// membership. Mirrors the OAuth2 findOrCreateUser pattern.
//
// Membership provisioning runs on every LDAP login, not just at user creation.
// It is idempotent (see ensureSystemOrgMembership), so a membership insert that
// failed on a previous login — leaving a persisted user with no primary org and
// therefore an empty dashboard — is repaired on the next one rather than
// requiring a manual DB insert.
func (m *AuthManager) findOrProvisionLDAPUser(ctx context.Context, ldapInfo *authldap.UserInfo, email string) (*models.User, error) {
	user, err := m.resolveLDAPUser(ctx, ldapInfo, email)
	if err != nil {
		return nil, err
	}

	// Non-fatal: a user who cannot be placed in the system org can still log in.
	// The next login retries, so this degrades rather than locks the account out.
	if err := m.ensureSystemOrgMembership(ctx, user.ID); err != nil {
		m.logger.Warn("Failed to ensure system organization membership", "error", err, "userId", user.ID)
	}

	return user, nil
}

// ensureSystemOrgMembership makes the caller's system-organization membership
// exist, creating it only when the user has no primary organization yet.
//
// Reading before writing is what makes this safe to call on every login: it is
// a no-op for the common case and self-healing for users whose membership
// insert previously failed. GetPrimaryOrganization filters on
// `is_primary = true AND accepted_at IS NOT NULL`, and the tenant middleware
// requires AcceptedAt to be set, so the created row must satisfy both.
func (m *AuthManager) ensureSystemOrgMembership(ctx context.Context, userID string) error {
	if m.orgMembershipRepo == nil {
		return nil
	}

	primaryOrg, err := m.orgMembershipRepo.GetPrimaryOrganization(ctx, userID)
	if err != nil {
		return fmt.Errorf("look up primary organization: %w", err)
	}
	if primaryOrg != nil {
		return nil
	}

	// InvitedAt is a non-pointer time.Time that the repository passes straight
	// into the INSERT, bypassing the column's DEFAULT NOW(). Leaving it zero
	// would write 0001-01-01 into a NOT NULL TIMESTAMPTZ.
	now := time.Now()
	membership := &models.OrganizationMembership{
		ID:             uuid.New().String(),
		OrganizationID: systemOrgID,
		UserID:         userID,
		Role:           models.OrgRoleMember,
		IsPrimary:      true,
		InvitedAt:      now,
		AcceptedAt:     &now,
	}
	if err := m.orgMembershipRepo.Create(ctx, membership); err != nil {
		return fmt.Errorf("create system organization membership: %w", err)
	}

	m.logger.Info("Provisioned system organization membership", "userId", userID, "organizationId", systemOrgID)
	return nil
}

// resolveLDAPUser finds an existing user by LDAP external ID or email, or
// creates a new one. It does not touch organization membership — see
// findOrProvisionLDAPUser, which owns that concern for every branch here.
func (m *AuthManager) resolveLDAPUser(ctx context.Context, ldapInfo *authldap.UserInfo, email string) (*models.User, error) {
	externalID := "ldap:" + ldapInfo.UID

	// 1. Lookup by external ID (already linked)
	existing, err := m.userRepo.GetByExternalID(ctx, externalID)
	if err != nil {
		return nil, fmt.Errorf("lookup by external ID: %w", err)
	}
	if existing != nil {
		// Update attributes if changed
		changed := false
		if ldapInfo.Email != "" && existing.Email != ldapInfo.Email {
			existing.Email = ldapInfo.Email
			changed = true
		}
		if ldapInfo.DisplayName != "" && existing.DisplayName != ldapInfo.DisplayName {
			existing.DisplayName = ldapInfo.DisplayName
			changed = true
		}
		if ldapInfo.Role != "" && existing.Role != ldapInfo.Role {
			existing.Role = ldapInfo.Role
			changed = true
		}
		if changed {
			if err := m.userRepo.Update(ctx, existing); err != nil {
				m.logger.Warn("Failed to update LDAP user attributes", "error", err, "userId", existing.ID)
			}
		}
		if err := m.userRepo.UpdateLastLogin(ctx, existing.ID); err != nil {
			m.logger.Debug("UpdateLastLogin failed (non-fatal)", "error", err, "userId", existing.ID)
		}
		return existing, nil
	}

	// 2. Lookup by email (link existing local user to LDAP)
	if email != "" {
		byEmail, err := m.userRepo.GetByEmail(ctx, email)
		if err != nil {
			m.logger.Warn("Failed to lookup by email for LDAP linking", "error", err, "email", email)
		}
		if byEmail != nil {
			byEmail.ExternalID = externalID
			if err := m.userRepo.Update(ctx, byEmail); err != nil {
				m.logger.Warn("Failed to link user to LDAP", "error", err, "userId", byEmail.ID)
			}
			if err := m.userRepo.UpdateLastLogin(ctx, byEmail.ID); err != nil {
				m.logger.Debug("UpdateLastLogin failed (non-fatal)", "error", err, "userId", byEmail.ID)
			}
			return byEmail, nil
		}
	}

	// 3. Create new user
	userEmail := ldapInfo.Email
	if userEmail == "" {
		userEmail = email
	}
	username := ldapInfo.UID
	if username == "" {
		username = m.ldapUsername(email)
	}
	role := ldapInfo.Role
	if role == "" {
		role = "student"
	}

	newUser := &models.User{
		ID:          uuid.New().String(),
		ExternalID:  externalID,
		Username:    username,
		Email:       userEmail,
		DisplayName: ldapInfo.DisplayName,
		Role:        role,
		IsActive:    true,
	}

	if err := m.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("create LDAP user: %w", err)
	}

	// The caller auto-provisions the system-org membership, and
	// completeLoginForUser then populates the JWT's DefaultOrganizationID from
	// it via GetPrimaryOrganization().

	m.logger.Info("Provisioned new LDAP user", "userId", newUser.ID, "email", newUser.Email, "uid", ldapInfo.UID)
	return newUser, nil
}

// completeLoginForUser handles the post-authentication flow shared between
// local and LDAP login: update last login, build auth token, set cookies,
// return JSON response.
func (m *AuthManager) completeLoginForUser(w http.ResponseWriter, r *http.Request, dbUser *models.User) {
	ctx := r.Context()

	// Update last login timestamp
	if err := m.userRepo.UpdateLastLogin(ctx, dbUser.ID); err != nil {
		m.logger.Warn("Failed to update last login", "error", err, "userId", dbUser.ID)
	}

	// Convert to auth.User for token generation
	authUser := &auth.User{
		ID:    dbUser.ID,
		Email: dbUser.Email,
		Name:  dbUser.DisplayName,
		Roles: []string{dbUser.Role},
	}

	// Get user's primary organization if available
	if m.orgMembershipRepo != nil {
		primaryOrg, err := m.orgMembershipRepo.GetPrimaryOrganization(ctx, dbUser.ID)
		if err != nil {
			m.logger.Warn("Failed to get primary organization", "error", err, "userId", dbUser.ID)
		} else if primaryOrg != nil {
			authUser.DefaultOrganizationID = primaryOrg.ID
		}
	}

	// Generate JWT token
	token, err := m.authService.GenerateToken(authUser)
	if err != nil {
		m.logger.Error("Failed to generate token", "error", err, "email", dbUser.Email)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "auth.errors.generateTokenFailed", nil)
		return
	}

	m.setAuthCookies(w, r, token)

	m.logger.Info("User logged in", "email", dbUser.Email, "userId", dbUser.ID)
	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"token":              token,
		"user":               authUser,
		"mustChangePassword": dbUser.MustChangePassword,
	})
}
