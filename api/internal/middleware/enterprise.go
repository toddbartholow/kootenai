package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/toddbartholow/kootenai/api/internal/enterprise"
)

// EnterpriseErrorResponse represents an error response for enterprise feature requests.
//
// There is deliberately no upgrade URL. Community Edition has no upgrade
// destination to offer -- Enterprise Edition is not distributed from this
// repository -- and pointing users at a URL the project does not control is
// worse than pointing them nowhere. If an Enterprise build ever needs one, pass
// it to RequireEnterprise so it is captured per-instance rather than held in
// package state.
type EnterpriseErrorResponse struct {
	Error   string `json:"error"`
	Feature string `json:"feature"`
	Edition string `json:"edition"`
}

// RequireEnterprise returns middleware that blocks requests requiring enterprise features.
// If the feature is not enabled, it returns a 403 Forbidden naming the feature and edition.
func RequireEnterprise(feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enterprise.Default.IsEnabled(feature) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)

				resp := EnterpriseErrorResponse{
					Error:   "This feature requires Kootenai Enterprise Edition",
					Feature: feature,
					Edition: enterprise.Default.Edition(),
				}

				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireEnterpriseOr returns middleware that checks if a feature is enabled,
// but falls back to a community-compatible handler if not.
// This is useful for endpoints that have limited functionality in CE.
func RequireEnterpriseOr(feature string, fallback http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enterprise.Default.IsEnabled(feature) {
				fallback.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// EnterpriseInfo returns middleware that adds enterprise edition info to response headers.
// This is useful for clients that want to know what features are available.
func EnterpriseInfo() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Kootenai-Edition", enterprise.Default.Edition())
			next.ServeHTTP(w, r)
		})
	}
}
