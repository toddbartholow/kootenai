package middleware

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the client IP for rate limiting and audit logging.
// Prefers X-Real-IP (set by nginx, single value, harder to spoof) over RemoteAddr.
// X-Forwarded-For is NOT used as it can be trivially spoofed by clients.
func GetClientIP(r *http.Request) string {
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
