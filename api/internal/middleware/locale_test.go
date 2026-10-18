package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
)

// handlerWritesLocalized renders the `auth.errors.invalidCredentials`
// message through the request-scoped localizer so the test can inspect the
// output for the expected English string.
func handlerWritesLocalized(w http.ResponseWriter, r *http.Request) {
	msg := appi18n.Localize(r.Context(), "auth.errors.invalidCredentials", nil)
	_, _ = w.Write([]byte(msg))
}

func TestLocale_AttachesLocalizerFromAcceptLanguage(t *testing.T) {
	bundle := appi18n.MustNewBundle()
	h := middleware.Locale(bundle)(http.HandlerFunc(handlerWritesLocalized))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, "invalid credentials", rr.Body.String())
}

func TestLocale_NoHeaderFallsBackToDefault(t *testing.T) {
	bundle := appi18n.MustNewBundle()
	h := middleware.Locale(bundle)(http.HandlerFunc(handlerWritesLocalized))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Default language is English; should still resolve to the English catalog.
	require.Equal(t, "invalid credentials", rr.Body.String())
}

func TestLocale_UnsupportedLocaleFallsBackToDefault(t *testing.T) {
	bundle := appi18n.MustNewBundle()
	h := middleware.Locale(bundle)(http.HandlerFunc(handlerWritesLocalized))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, "invalid credentials", rr.Body.String())
}

func TestLocale_NilBundleIsNoop(t *testing.T) {
	h := middleware.Locale(nil)(http.HandlerFunc(handlerWritesLocalized))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// No localizer attached → Localize() returns the message ID unchanged.
	require.Equal(t, "auth.errors.invalidCredentials", rr.Body.String())
}

func TestLocale_MalformedHeaderFallsBackToDefault(t *testing.T) {
	bundle := appi18n.MustNewBundle()
	h := middleware.Locale(bundle)(http.HandlerFunc(handlerWritesLocalized))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "not-a-language-tag!@#")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, "invalid credentials", rr.Body.String())
}
