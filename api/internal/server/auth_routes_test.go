package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/users"
)

// These tests cover AuthManager.SetupPublicRoutes composition only — they
// exist to pin the middleware-injection contract that replaced the inline
// route block in server.setupRoutes. Handler-level behavior (login flow,
// password reset email send, etc.) is covered by auth_handlers_test.go and
// password_handlers_test.go and is intentionally not re-exercised here.

// markerMiddleware returns a MiddlewareFunc that short-circuits with the
// given status code without calling next. Using it as a probe lets the test
// assert "this middleware was attached to that route" cleanly: if we see
// the sentinel status, the middleware ran; if we see a handler-produced
// status (400/503) or 404, it didn't.
func markerMiddleware(status int) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})
	}
}

// newAuthManagerForRoutes builds an AuthManager with everything needed to
// exercise SetupPublicRoutes. authService and userRepo are intentionally
// left nil — the handlers are allowed to fail downstream; what matters is
// which middleware wraps them.
func newAuthManagerForRoutes(cfg AuthManagerConfig) *AuthManager {
	if cfg.Logger == nil {
		cfg.Logger = newTestLogger()
	}
	if cfg.Responder == nil {
		cfg.Responder = httputil.NewResponder(cfg.Logger)
	}
	return NewAuthManager(cfg)
}

func TestAuthManagerSetupPublicRoutes_LoginRateLimit(t *testing.T) {
	am := newAuthManagerForRoutes(AuthManagerConfig{
		LoginRateLimit: markerMiddleware(http.StatusTeapot),
	})
	r := chi.NewRouter()
	am.SetupPublicRoutes(r)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("LoginRateLimit middleware was not applied to /auth/login: got %d, want 418", rr.Code)
	}
}

func TestAuthManagerSetupPublicRoutes_RefreshAndLogoutBypassLoginLimit(t *testing.T) {
	// Same config as above — login limiter present — but refresh and logout
	// must NOT carry it. If they did, both would return 418.
	am := newAuthManagerForRoutes(AuthManagerConfig{
		LoginRateLimit: markerMiddleware(http.StatusTeapot),
	})
	r := chi.NewRouter()
	am.SetupPublicRoutes(r)

	for _, path := range []string{"/auth/refresh", "/auth/logout"} {
		t.Run(path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			r.ServeHTTP(rr, req)
			if rr.Code == http.StatusTeapot {
				t.Errorf("LoginRateLimit leaked onto %s", path)
			}
			if rr.Code == http.StatusNotFound {
				t.Errorf("%s not registered", path)
			}
		})
	}
}

func TestAuthManagerSetupPublicRoutes_PasswordResetDelegatesToUserMgr(t *testing.T) {
	// The password-reset routes must:
	//   1) be registered only when UserMgr is supplied;
	//   2) carry PasswordResetRateLimit when supplied;
	//   3) NOT carry LoginRateLimit.
	um := users.NewManager(users.Config{
		Logger:    newTestLogger(),
		Responder: httputil.NewResponder(newTestLogger()),
	})
	am := newAuthManagerForRoutes(AuthManagerConfig{
		UserMgr:                um,
		LoginRateLimit:         markerMiddleware(http.StatusTeapot),
		PasswordResetRateLimit: markerMiddleware(http.StatusForbidden), // distinct sentinel
	})
	r := chi.NewRouter()
	am.SetupPublicRoutes(r)

	for _, path := range []string{
		"/auth/password/reset-request",
		"/auth/password/reset-confirm",
	} {
		t.Run(path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			r.ServeHTTP(rr, req)
			if rr.Code == http.StatusTeapot {
				t.Errorf("LoginRateLimit leaked onto %s", path)
			}
			if rr.Code != http.StatusForbidden {
				t.Errorf("PasswordResetRateLimit was not applied to %s: got %d, want 403", path, rr.Code)
			}
		})
	}
}

func TestAuthManagerSetupPublicRoutes_PasswordResetAbsentWithoutUserMgr(t *testing.T) {
	// No UserMgr supplied → password routes must not register. 404 is the
	// expected chi default when a route is missing.
	am := newAuthManagerForRoutes(AuthManagerConfig{
		PasswordResetRateLimit: markerMiddleware(http.StatusForbidden),
	})
	r := chi.NewRouter()
	am.SetupPublicRoutes(r)

	for _, path := range []string{
		"/auth/password/reset-request",
		"/auth/password/reset-confirm",
	} {
		t.Run(path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			r.ServeHTTP(rr, req)
			if rr.Code != http.StatusNotFound {
				t.Errorf("expected 404 when UserMgr is nil, got %d", rr.Code)
			}
		})
	}
}

func TestAuthManagerSetupPublicRoutes_NoRateLimitersRegistersEveryRoute(t *testing.T) {
	// With both rate limiters nil, SetupPublicRoutes must still register
	// login/refresh/logout and (when UserMgr is present) the two password
	// routes, without panicking on nil middleware.
	um := users.NewManager(users.Config{
		Logger:    newTestLogger(),
		Responder: httputil.NewResponder(newTestLogger()),
	})
	am := newAuthManagerForRoutes(AuthManagerConfig{
		UserMgr: um,
		// Both rate limiters intentionally nil.
	})
	r := chi.NewRouter()
	am.SetupPublicRoutes(r)

	paths := []string{
		"/auth/login",
		"/auth/refresh",
		"/auth/logout",
		"/auth/password/reset-request",
		"/auth/password/reset-confirm",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			r.ServeHTTP(rr, req)
			// Any status other than 404 proves the route is registered.
			// Handlers will return 400/503 depending on missing deps; we
			// don't assert specifics because handler behavior isn't the
			// subject of this test.
			if rr.Code == http.StatusNotFound {
				t.Errorf("%s was not registered", path)
			}
		})
	}
}
