// Package middleware provides HTTP middleware helpers used by the server.
package middleware

import (
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

// Locale returns middleware that parses the request's Accept-Language
// header, selects the best-matching catalog from bundle, and attaches a
// request-scoped *i18n.Localizer to the context.
//
// If bundle is nil the middleware is a no-op — the handler chain continues
// without a localizer, and Localize() falls back to the message ID.
//
// Precedence note: this middleware runs before authentication, so it cannot
// see the authenticated user. Instead, the web client hydrates its locale
// store from GET /auth/me (which includes preferredLocale) and sets
// Accept-Language on every subsequent request. Backend code that needs to
// render in a specific locale outside a request context (e.g. email jobs)
// must construct its own i18n.Localizer directly from the bundle.
func Locale(bundle *i18n.Bundle) func(http.Handler) http.Handler {
	if bundle == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accept := r.Header.Get("Accept-Language")
			localizer := i18n.NewLocalizer(bundle, accept)
			ctx := appi18n.WithLocalizer(r.Context(), localizer)
			ctx = appi18n.WithRequestedLocale(ctx, accept)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
