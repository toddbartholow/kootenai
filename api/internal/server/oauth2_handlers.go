package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/auth/oauth2"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// localizedHTTPError writes a plaintext error via net/http.Error, localized
// from the OAuth2 handlers' request context. Used for browser-facing OAuth2
// callbacks where text/plain is appropriate.
func localizedHTTPError(r *http.Request, w http.ResponseWriter, status int, messageID string) {
	http.Error(w, appi18n.Localize(r.Context(), messageID, nil), status)
}

// OAuth2Config holds configuration for OAuth2 handlers
type OAuth2Config struct {
	// BaseURL is the base URL of the application (for redirects)
	BaseURL string
	// DefaultRedirect is the default redirect after successful login
	DefaultRedirect string
	// AllowedRedirects is a list of allowed redirect URL origins (scheme+host)
	AllowedRedirects []string
}

// OAuth2Handlers handles OAuth2/OIDC authentication endpoints
type OAuth2Handlers struct {
	manager     *oauth2.Manager
	authService *auth.Service
	userRepo    repositories.UserRepository
	config      OAuth2Config
	cookieCfg   auth.CookieConfig
	logger      *slog.Logger
}

// NewOAuth2Handlers creates new OAuth2 handlers
func NewOAuth2Handlers(
	manager *oauth2.Manager,
	authService *auth.Service,
	userRepo repositories.UserRepository,
	config OAuth2Config,
	cookieCfg auth.CookieConfig,
	logger *slog.Logger,
) *OAuth2Handlers {
	if logger == nil {
		logger = slog.Default()
	}
	if config.DefaultRedirect == "" {
		config.DefaultRedirect = "/"
	}

	return &OAuth2Handlers{
		manager:     manager,
		authService: authService,
		userRepo:    userRepo,
		config:      config,
		cookieCfg:   cookieCfg,
		logger:      logger,
	}
}

// RegisterRoutes registers OAuth2 routes
func (h *OAuth2Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/oauth2/providers", h.handleListProviders)
	r.Get("/oauth2/login/{provider}", h.handleLogin)
	r.Get("/oauth2/callback/{provider}", h.handleCallback)
}

// handleListProviders returns available OAuth2 providers
func (h *OAuth2Handlers) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.manager.ListProviders()

	// Transform to public format
	type PublicProvider struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	var result []PublicProvider
	for _, p := range providers {
		result = append(result, PublicProvider{
			ID:   p.ID,
			Name: p.Name,
			Type: string(p.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"providers": result,
	})
}

// handleLogin initiates the OAuth2 login flow
func (h *OAuth2Handlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "provider")
	if providerID == "" {
		localizedHTTPError(r, w, http.StatusBadRequest, "oauth2.errors.providerIdRequired")
		return
	}

	// Get optional redirect URL
	redirectURL := r.URL.Query().Get("redirect")
	if redirectURL != "" && !h.isAllowedRedirect(redirectURL) {
		h.logger.Warn("Blocked disallowed redirect URL",
			slog.String("redirect", redirectURL),
		)
		redirectURL = h.config.DefaultRedirect
	}

	// Initiate login
	authURL, err := h.manager.InitiateLogin(r.Context(), providerID, redirectURL)
	if err != nil {
		h.logger.Error("Failed to initiate OAuth2 login",
			slog.String("provider", providerID),
			slog.String("error", err.Error()),
		)
		localizedHTTPError(r, w, http.StatusInternalServerError, "oauth2.errors.initiateLoginFailed")
		return
	}

	// Redirect to provider
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// handleCallback handles the OAuth2 callback
func (h *OAuth2Handlers) handleCallback(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "provider")

	// Check for error from provider
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		errDesc := r.URL.Query().Get("error_description")
		h.logger.Error("OAuth2 provider returned error",
			slog.String("provider", providerID),
			slog.String("error", errCode),
			slog.String("description", errDesc),
		)
		h.redirectWithError(w, r, "Login failed: "+errDesc)
		return
	}

	// Get code and state
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		localizedHTTPError(r, w, http.StatusBadRequest, "oauth2.errors.missingCodeOrState")
		return
	}

	// Complete login
	result, err := h.manager.CompleteLogin(r.Context(), state, code)
	if err != nil {
		h.logger.Error("Failed to complete OAuth2 login",
			slog.String("provider", providerID),
			slog.String("error", err.Error()),
		)
		h.redirectWithError(w, r, "Login failed")
		return
	}

	// Find or create user
	user, err := h.findOrCreateUser(r, result)
	if err != nil {
		h.logger.Error("Failed to find or create user",
			slog.String("email", result.UserInfo.Email),
			slog.String("error", err.Error()),
		)
		h.redirectWithError(w, r, "Failed to create account")
		return
	}

	// Generate platform token
	authUser := &auth.User{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.DisplayName,
		Roles: []string{user.Role},
	}

	token, err := h.authService.GenerateToken(authUser)
	if err != nil {
		h.logger.Error("Failed to generate token",
			slog.String("userId", user.ID),
			slog.String("error", err.Error()),
		)
		h.redirectWithError(w, r, "Failed to complete login")
		return
	}

	// Redirect with token
	redirectURL := result.RedirectURL
	if redirectURL == "" {
		redirectURL = h.config.DefaultRedirect
	}

	h.redirectWithToken(w, r, redirectURL, token)
}

// findOrCreateUser finds an existing user or creates a new one
func (h *OAuth2Handlers) findOrCreateUser(r *http.Request, result *oauth2.LoginResult) (*models.User, error) {
	ctx := r.Context()
	info := result.UserInfo

	// Generate external ID from provider and subject
	externalID := result.Provider + ":" + info.Sub

	// Try to find by external ID
	user, err := h.userRepo.GetByExternalID(ctx, externalID)
	if err == nil && user != nil {
		// Update last login
		h.userRepo.UpdateLastLogin(ctx, user.ID)
		return user, nil
	}

	// Try to find by email
	if info.Email != "" {
		user, err = h.userRepo.GetByEmail(ctx, info.Email)
		if err == nil && user != nil {
			// Link external ID to existing user by updating the user
			user.ExternalID = externalID
			if err := h.userRepo.Update(ctx, user); err != nil {
				h.logger.Warn("Failed to link external ID",
					slog.String("userId", user.ID),
					slog.String("externalId", externalID),
					slog.String("error", err.Error()),
				)
			}
			h.userRepo.UpdateLastLogin(ctx, user.ID)
			return user, nil
		}
	}

	// Create new user
	name := info.Name
	if name == "" {
		name = info.GivenName
		if info.FamilyName != "" {
			name += " " + info.FamilyName
		}
	}
	if name == "" && info.Email != "" {
		name = strings.Split(info.Email, "@")[0]
	}

	// Use first role or default
	role := "student"
	if len(info.Roles) > 0 {
		role = info.Roles[0]
	}

	// Generate username from email
	username := info.Email
	if username == "" {
		username = externalID
	}

	newUser := &models.User{
		ID:          uuid.New().String(),
		ExternalID:  externalID,
		Username:    username,
		Email:       info.Email,
		DisplayName: name,
		Role:        role,
		IsActive:    true,
	}

	if err := h.userRepo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	h.logger.Info("Created new user via OAuth2",
		slog.String("userId", newUser.ID),
		slog.String("email", newUser.Email),
		slog.String("provider", result.Provider),
	)

	return newUser, nil
}

// isAllowedRedirect checks if a redirect URL is allowed.
// Uses exact scheme+host matching instead of prefix matching to prevent
// bypass via domains like "https://app.example.com.evil.com".
func (h *OAuth2Handlers) isAllowedRedirect(redirectURL string) bool {
	// Always allow relative URLs (but not protocol-relative //evil.com)
	if strings.HasPrefix(redirectURL, "/") && !strings.HasPrefix(redirectURL, "//") {
		return true
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil || parsed.Host == "" {
		return false
	}

	// Check against allowed origins (exact scheme+host match)
	for _, allowed := range h.config.AllowedRedirects {
		allowedURL, err := url.Parse(allowed)
		if err != nil {
			continue
		}
		if parsed.Scheme == allowedURL.Scheme && parsed.Host == allowedURL.Host {
			return true
		}
	}

	// Check against base URL
	if h.config.BaseURL != "" {
		baseURL, err := url.Parse(h.config.BaseURL)
		if err == nil && parsed.Scheme == baseURL.Scheme && parsed.Host == baseURL.Host {
			return true
		}
	}

	return false
}

// redirectWithToken redirects with a token. When cookie mode is enabled, the
// token is set as an HttpOnly cookie and the redirect URL is clean (no fragment).
// Otherwise, falls back to the URL fragment approach for backwards compat.
func (h *OAuth2Handlers) redirectWithToken(w http.ResponseWriter, r *http.Request, redirectURL, token string) {
	if h.cookieCfg.Enabled {
		auth.SetAuthCookie(w, r, token, h.cookieCfg)
		if csrfToken, err := auth.GenerateCSRFToken(); err == nil {
			auth.SetCSRFCookie(w, r, csrfToken, h.cookieCfg)
		}
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	separator := "#"
	if strings.Contains(redirectURL, "#") {
		separator = "&"
	}
	fullURL := redirectURL + separator + "token=" + token
	http.Redirect(w, r, fullURL, http.StatusTemporaryRedirect)
}

// redirectWithError redirects with an error message
func (h *OAuth2Handlers) redirectWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	redirectURL := h.config.DefaultRedirect + "#error=" + errMsg
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// HandleRefreshToken handles OAuth2 token refresh requests
func (h *OAuth2Handlers) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider     string `json:"provider"`
		RefreshToken string `json:"refreshToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		localizedHTTPError(r, w, http.StatusBadRequest, "oauth2.errors.invalidRequestBody")
		return
	}

	if req.Provider == "" || req.RefreshToken == "" {
		localizedHTTPError(r, w, http.StatusBadRequest, "oauth2.errors.providerAndRefreshRequired")
		return
	}

	tokens, err := h.manager.RefreshAccessToken(r.Context(), req.Provider, req.RefreshToken)
	if err != nil {
		h.logger.Error("Failed to refresh OAuth2 token",
			slog.String("provider", req.Provider),
			slog.String("error", err.Error()),
		)
		localizedHTTPError(r, w, http.StatusUnauthorized, "oauth2.errors.refreshTokenFailed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}
