package auth

import (
	"crypto/subtle"
	"net/http"
)

const csrfHeaderName = "X-CSRF-Token"

// CSRFMiddleware enforces the double-submit cookie pattern for requests
// authenticated via cookie. Bearer-token requests are exempt because they
// are not vulnerable to cross-site request forgery.
//
// On mutating methods (POST, PUT, PATCH, DELETE) it reads the csrf_token
// cookie and compares it against the X-CSRF-Token header using constant-time
// comparison. Mismatches are rejected with 403.
func CSRFMiddleware(cfg CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Safe methods are never CSRF-checked.
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Requests authenticated via Bearer token are not subject to CSRF.
			src := AuthSourceFromContext(r.Context())
			if src != AuthSourceCookie {
				next.ServeHTTP(w, r)
				return
			}

			// Double-submit: cookie value must match header value.
			cookieVal := ""
			if c, err := r.Cookie(cfg.csrfCookieName()); err == nil {
				cookieVal = c.Value
			}
			headerVal := r.Header.Get(csrfHeaderName)

			if cookieVal == "" || headerVal == "" {
				http.Error(w, "CSRF token missing", http.StatusForbidden)
				return
			}

			if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(headerVal)) != 1 {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
