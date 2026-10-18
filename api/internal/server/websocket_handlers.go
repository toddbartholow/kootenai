// Package server provides the HTTP server and API routes
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// -----------------------------------------------------------------------------
// WebSocket Manager
// -----------------------------------------------------------------------------

// WebSocketManager manages WebSocket connection handlers.
type WebSocketManager struct {
	authService  *auth.Service
	wsHub        *websocket.Hub
	orchestrator orchestrator.Client
	evaluator    *checkpoint.Evaluator
	sessionRepo  repositories.SessionRepository
	userRepo     repositories.UserRepository
	logger       *slog.Logger
	responder    *httputil.Responder
}

// WebSocketManagerConfig configures WebSocketManager.
type WebSocketManagerConfig struct {
	AuthService  *auth.Service
	WsHub        *websocket.Hub
	Orchestrator orchestrator.Client
	Evaluator    *checkpoint.Evaluator
	SessionRepo  repositories.SessionRepository
	UserRepo     repositories.UserRepository
	Logger       *slog.Logger
}

// NewWebSocketManager creates a new WebSocketManager.
func NewWebSocketManager(cfg WebSocketManagerConfig) *WebSocketManager {
	return &WebSocketManager{
		authService:  cfg.AuthService,
		wsHub:        cfg.WsHub,
		orchestrator: cfg.Orchestrator,
		evaluator:    cfg.Evaluator,
		sessionRepo:  cfg.SessionRepo,
		userRepo:     cfg.UserRepo,
		logger:       cfg.Logger,
		responder:    httputil.NewResponder(cfg.Logger),
	}
}

// SetupRoutes registers WebSocket routes on the router.
// The upgrade handlers auth the token themselves (can't use HTTP-only auth
// middleware because browsers can't set Authorization headers on WS upgrades),
// so this is registered on the post-auth router solely to share the
// middleware chain's other concerns (tenancy, rate limiting, etc.).
func (m *WebSocketManager) SetupRoutes(r chi.Router) {
	r.Get("/ws", m.handleWebSocket)
	r.Get("/ws/{podID}", m.handleWebSocketPod)
}

// extractWebSocketToken extracts JWT token from WebSocket request.
// Checks Authorization header first, then Sec-WebSocket-Protocol, then cookie.
func (m *WebSocketManager) extractWebSocketToken(r *http.Request) string {
	// Check Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		return authHeader
	}

	// Check Sec-WebSocket-Protocol header (preferred for WebSocket auth)
	protocols := r.Header.Get("Sec-WebSocket-Protocol")
	if protocols != "" {
		for _, proto := range strings.Split(protocols, ",") {
			proto = strings.TrimSpace(proto)
			if strings.HasPrefix(proto, "access_token.") {
				return strings.TrimPrefix(proto, "access_token.")
			}
		}
	}

	// Fall back to auth cookie (browsers send cookies on WS upgrade)
	if c, err := r.Cookie(auth.DefaultCookieName); err == nil && c.Value != "" {
		return c.Value
	}

	return ""
}

// authenticateWebSocket validates the JWT token and returns the user ID.
// Returns empty string if authentication fails or is not configured.
func (m *WebSocketManager) authenticateWebSocket(r *http.Request) (string, bool) {
	// If auth service is not configured, allow anonymous connections (demo mode)
	if m.authService == nil {
		m.logger.Info("WebSocket auth: no auth service, allowing anonymous")
		return "00000000-0000-0000-0000-000000000001", true
	}

	// In demo mode, always use the fixed demo user ID
	// SECURITY: Do NOT allow userId from query param to prevent impersonation
	if m.authService.IsDemoMode() {
		m.logger.Info("WebSocket auth: demo mode enabled, using fixed demo user")
		return "00000000-0000-0000-0000-000000000001", true
	}

	m.logger.Debug("WebSocket auth: checking token",
		"hasDemoMode", m.authService.IsDemoMode(),
		"authServiceNil", m.authService == nil,
	)

	// Extract and validate token
	token := m.extractWebSocketToken(r)
	if token == "" {
		m.logger.Debug("WebSocket auth: no token found")
		return "", false
	}

	// Validate the token
	claims, err := m.authService.ValidateToken(token)
	if err != nil {
		m.logger.Debug("WebSocket token validation failed", "error", err)
		return "", false
	}

	return claims.UserID, true
}

func (m *WebSocketManager) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate the WebSocket connection
	userID, authenticated := m.authenticateWebSocket(r)
	if !authenticated {
		m.logger.Warn("WebSocket connection rejected: authentication failed",
			"remoteAddr", r.RemoteAddr,
		)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")

	// Validate session exists and check ownership
	if sessionID != "" {
		if !m.validateAndAuthorizeSession(r.Context(), w, sessionID, userID) {
			return
		}
	}

	m.wsHub.ServeWS(w, r, "", sessionID, userID)
}

func (m *WebSocketManager) handleWebSocketPod(w http.ResponseWriter, r *http.Request) {
	// Authenticate the WebSocket connection
	userID, authenticated := m.authenticateWebSocket(r)
	if !authenticated {
		m.logger.Warn("WebSocket connection rejected: authentication failed",
			"remoteAddr", r.RemoteAddr,
		)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	podID := chi.URLParam(r, "podID")
	sessionID := r.URL.Query().Get("sessionId")

	// Validate pod exists and check ownership
	if podID != "" {
		pod, err := m.orchestrator.GetPod(r.Context(), podID)
		if err != nil {
			m.logger.Warn("WebSocket connection rejected: invalid pod",
				"podId", podID,
				"remoteAddr", r.RemoteAddr,
			)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "websocket.errors.invalidPod", nil)
			return
		}
		if pod.OwnerID != userID && !m.isUserAdmin(r.Context(), userID) {
			m.logger.Warn("WebSocket connection rejected: not pod owner",
				"podId", podID,
				"podOwner", pod.OwnerID,
				"userID", userID,
				"remoteAddr", r.RemoteAddr,
			)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
			return
		}
	}

	// Validate session if provided and check ownership
	if sessionID != "" {
		if !m.validateAndAuthorizeSession(r.Context(), w, sessionID, userID) {
			return
		}
	}

	m.wsHub.ServeWS(w, r, podID, sessionID, userID)
}

// validateSession checks if a session exists in either the evaluator cache or the database.
func (m *WebSocketManager) validateSession(ctx context.Context, sessionID string) bool {
	// First check in-memory evaluator cache (fast path)
	if _, err := m.evaluator.GetSessionProgress(sessionID); err == nil {
		return true
	}

	// Fallback to database check
	if m.sessionRepo != nil {
		session, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err == nil && session != nil {
			return true
		}
	}

	return false
}

// validateAndAuthorizeSession checks if a session exists and the user owns it.
// Returns true if authorized, false otherwise (and writes the error response).
func (m *WebSocketManager) validateAndAuthorizeSession(ctx context.Context, w http.ResponseWriter, sessionID, userID string) bool {
	// First check in-memory evaluator cache (fast path, no ownership info)
	if _, err := m.evaluator.GetSessionProgress(sessionID); err == nil {
		// Session exists in cache; check ownership via DB if available
		if m.sessionRepo != nil {
			session, err := m.sessionRepo.GetByID(ctx, sessionID)
			if err == nil && session != nil {
				if session.UserID != userID && !m.isUserAdmin(ctx, userID) {
					m.logger.Warn("WebSocket connection rejected: not session owner",
						"sessionId", sessionID,
						"sessionOwner", session.UserID,
						"userID", userID,
					)
					m.responder.LocalizedErrorResponse(ctx, w, http.StatusForbidden, "authz.accessDenied", nil)
					return false
				}
			}
		}
		return true
	}

	// Fallback to database check
	if m.sessionRepo != nil {
		session, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err == nil && session != nil {
			if session.UserID != userID && !m.isUserAdmin(ctx, userID) {
				m.logger.Warn("WebSocket connection rejected: not session owner",
					"sessionId", sessionID,
					"sessionOwner", session.UserID,
					"userID", userID,
				)
				m.responder.LocalizedErrorResponse(ctx, w, http.StatusForbidden, "authz.accessDenied", nil)
				return false
			}
			return true
		}
	}

	m.logger.Warn("WebSocket connection rejected: invalid session",
		"sessionId", sessionID,
	)
	m.responder.LocalizedErrorResponse(ctx, w, http.StatusUnauthorized, "websocket.errors.invalidSession", nil)
	return false
}

// isUserAdmin checks if a user has admin role by looking up the user repository.
// This is used in WebSocket handlers where auth middleware context is not available.
func (m *WebSocketManager) isUserAdmin(ctx context.Context, userID string) bool {
	if m.userRepo == nil {
		return false
	}
	user, err := m.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return false
	}
	return user.Role == "admin"
}
