// Package middleware provides HTTP middleware for request validation and security
package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	// DefaultMaxBodySize is the default maximum request body size (1MB)
	DefaultMaxBodySize = 1 << 20 // 1MB

	// LargeBodySize for file uploads (10MB)
	LargeBodySize = 10 << 20 // 10MB
)

// ValidationConfig holds configuration for validation middleware
type ValidationConfig struct {
	MaxBodySize    int64
	RequireJSON    bool
	AllowEmptyBody bool
}

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		MaxBodySize:    DefaultMaxBodySize,
		RequireJSON:    false,
		AllowEmptyBody: true,
	}
}

// ValidateRequestBody creates middleware that validates request body size
// and optionally validates JSON content
func ValidateRequestBody(cfg ValidationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for GET and HEAD requests (no body)
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			// Check content length if provided
			if r.ContentLength > cfg.MaxBodySize {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodySize)

			// For methods that typically have a body
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				contentType := r.Header.Get("Content-Type")

				// If RequireJSON is set, validate Content-Type
				if cfg.RequireJSON {
					if !strings.HasPrefix(contentType, "application/json") {
						http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireJSON creates middleware that requires JSON Content-Type for write methods
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip for methods that don't typically have a body
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// MaxBodySize creates middleware that limits request body size
func MaxBodySize(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxSize {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			next.ServeHTTP(w, r)
		})
	}
}

// ValidateJSON validates that the request body is valid JSON
// Note: This reads and replaces the body, so it should be used carefully
func ValidateJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip for methods that don't typically have a body
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Skip if Content-Type is not JSON
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			next.ServeHTTP(w, r)
			return
		}

		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		// Empty body is valid for some cases
		if len(body) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Validate JSON syntax
		if !json.Valid(body) {
			http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
			return
		}

		// Replace body for downstream handlers
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		r.ContentLength = int64(len(body))

		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersConfig holds configuration for security headers
type SecurityHeadersConfig struct {
	// EnableHSTS enables HTTP Strict Transport Security (only enable in production with HTTPS)
	EnableHSTS bool
	// HSTSMaxAge is the max-age for HSTS in seconds (default: 1 year)
	HSTSMaxAge int
	// EnableCSP enables Content-Security-Policy header
	EnableCSP bool
	// CSPPolicy is the Content-Security-Policy value (if empty, uses default)
	CSPPolicy string
	// PermissionsPolicy controls browser features (formerly Feature-Policy)
	PermissionsPolicy string
	// LTIFrameAncestors restricts which origins can embed LTI pages in iframes.
	// If empty, defaults to "'self'" for safety. Set to the Canvas URL in production.
	LTIFrameAncestors string
}

// DefaultSecurityHeadersConfig returns a secure default configuration
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		EnableHSTS:        os.Getenv("ENABLE_HSTS") == "true",
		HSTSMaxAge:        31536000, // 1 year
		EnableCSP:         true,
		CSPPolicy:         "default-src 'self'",
		PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
		LTIFrameAncestors: os.Getenv("LTI_FRAME_ANCESTORS"),
	}
}

// SecureHeaders adds security-related headers to responses
func SecureHeaders(next http.Handler) http.Handler {
	return SecureHeadersWithConfig(DefaultSecurityHeadersConfig())(next)
}

// SecureHeadersWithConfig adds security-related headers with custom configuration
func SecureHeadersWithConfig(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking - but allow LTI endpoints to be framed by Canvas
			if strings.HasPrefix(r.URL.Path, "/lti") {
				// LTI endpoints need to be embeddable in Canvas iframes
				// Don't set X-Frame-Options, use CSP frame-ancestors instead
			} else {
				w.Header().Set("X-Frame-Options", "DENY")
			}

			// XSS protection (legacy, but still useful for older browsers)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// HTTP Strict Transport Security (only for HTTPS in production)
			if cfg.EnableHSTS {
				w.Header().Set("Strict-Transport-Security",
					fmt.Sprintf("max-age=%d; includeSubDomains; preload", cfg.HSTSMaxAge))
			}

			// Permissions Policy (controls browser features)
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}

			// Content Security Policy
			if cfg.EnableCSP {
				// Swagger UI requires inline scripts and styles, so use a relaxed policy for /swagger
				if strings.HasPrefix(r.URL.Path, "/swagger") {
					w.Header().Set("Content-Security-Policy",
						"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; media-src 'self' data:")
				} else if r.URL.Path == "/lti/console" {
					// VNC console needs unsafe-eval for noVNC RFB decoder
					frameAncestors := cfg.LTIFrameAncestors
					if frameAncestors == "" {
						frameAncestors = "'self'"
					}
					w.Header().Set("Content-Security-Policy",
						fmt.Sprintf("default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data: blob:; media-src 'self' data: blob:; frame-ancestors %s", frameAncestors))
				} else if strings.HasPrefix(r.URL.Path, "/lti") {
					// LTI endpoints need to be embeddable in Canvas iframes
					frameAncestors := cfg.LTIFrameAncestors
					if frameAncestors == "" {
						frameAncestors = "'self'"
					}
					w.Header().Set("Content-Security-Policy",
						fmt.Sprintf("default-src 'self'; frame-ancestors %s; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'", frameAncestors))
				} else {
					csp := cfg.CSPPolicy
					if csp == "" {
						csp = "default-src 'self'"
					}
					w.Header().Set("Content-Security-Policy", csp)
				}
			}

			// Cache control for sensitive endpoints
			if strings.HasPrefix(r.URL.Path, "/api/v1/auth") ||
				strings.HasPrefix(r.URL.Path, "/api/v1/users") ||
				strings.HasPrefix(r.URL.Path, "/api/v1/password") {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
				w.Header().Set("Pragma", "no-cache")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SanitizeInput is a helper function for basic input sanitization
// It removes potentially dangerous characters from strings
func SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	return input
}

// ValidateNonEmpty checks if a string is non-empty after sanitization
func ValidateNonEmpty(input string) bool {
	return len(SanitizeInput(input)) > 0
}

// ValidateMaxLength checks if a string is within the max length
func ValidateMaxLength(input string, maxLen int) bool {
	return len(input) <= maxLen
}

// ValidateMinLength checks if a string meets the minimum length
func ValidateMinLength(input string, minLen int) bool {
	return len(input) >= minLen
}
