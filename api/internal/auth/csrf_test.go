package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func csrfTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCSRFMiddleware_Disabled(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = false
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("disabled CSRF should pass, got %d", w.Code)
	}
}

func TestCSRFMiddleware_SafeMethods(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/", nil)
			ctx := contextWithAuthSource(r.Context(), AuthSourceCookie)
			r = r.WithContext(ctx)
			w := httptest.NewRecorder()
			mw.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("%s should pass without CSRF, got %d", method, w.Code)
			}
		})
	}
}

func TestCSRFMiddleware_SkipsBearer(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := contextWithAuthSource(r.Context(), AuthSourceBearer)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Bearer auth should skip CSRF, got %d", w.Code)
	}
}

func TestCSRFMiddleware_SkipsDemo(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := contextWithAuthSource(r.Context(), AuthSourceDemo)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("demo auth should skip CSRF, got %d", w.Code)
	}
}

func TestCSRFMiddleware_MutatingWithValidToken(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/", nil)
			ctx := contextWithAuthSource(r.Context(), AuthSourceCookie)
			r = r.WithContext(ctx)
			r.AddCookie(&http.Cookie{Name: DefaultCSRFName, Value: "valid-csrf-token"})
			r.Header.Set("X-CSRF-Token", "valid-csrf-token")
			w := httptest.NewRecorder()
			mw.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("%s with valid CSRF should pass, got %d", method, w.Code)
			}
		})
	}
}

func TestCSRFMiddleware_MutatingWithMismatch(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := contextWithAuthSource(r.Context(), AuthSourceCookie)
	r = r.WithContext(ctx)
	r.AddCookie(&http.Cookie{Name: DefaultCSRFName, Value: "correct-token"})
	r.Header.Set("X-CSRF-Token", "wrong-token")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("mismatched CSRF should be 403, got %d", w.Code)
	}
}

func TestCSRFMiddleware_MissingCSRFCookie(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := contextWithAuthSource(r.Context(), AuthSourceCookie)
	r = r.WithContext(ctx)
	r.Header.Set("X-CSRF-Token", "some-value")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("missing CSRF cookie should be 403, got %d", w.Code)
	}
}

func TestCSRFMiddleware_MissingCSRFHeader(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.Enabled = true
	mw := CSRFMiddleware(cfg)(csrfTestHandler())

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := contextWithAuthSource(r.Context(), AuthSourceCookie)
	r = r.WithContext(ctx)
	r.AddCookie(&http.Cookie{Name: DefaultCSRFName, Value: "some-value"})
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("missing CSRF header should be 403, got %d", w.Code)
	}
}

func TestAuthSourceFromContext(t *testing.T) {
	t.Run("no source set", func(t *testing.T) {
		ctx := context.Background()
		if got := AuthSourceFromContext(ctx); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("cookie source", func(t *testing.T) {
		ctx := contextWithAuthSource(context.Background(), AuthSourceCookie)
		if got := AuthSourceFromContext(ctx); got != AuthSourceCookie {
			t.Errorf("got %q, want %q", got, AuthSourceCookie)
		}
	})

	t.Run("bearer source", func(t *testing.T) {
		ctx := contextWithAuthSource(context.Background(), AuthSourceBearer)
		if got := AuthSourceFromContext(ctx); got != AuthSourceBearer {
			t.Errorf("got %q, want %q", got, AuthSourceBearer)
		}
	})
}
