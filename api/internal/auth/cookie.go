package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

const (
	DefaultCookieName = "auth_token"
	DefaultCSRFName   = "csrf_token"
	csrfTokenBytes    = 32
)

// CookieConfig controls HttpOnly cookie auth behavior.
// When Enabled is false, no cookies are set and auth falls back to Bearer tokens only.
type CookieConfig struct {
	Enabled  bool
	Name     string
	CSRFName string
	MaxAge   time.Duration
}

// DefaultCookieConfig returns a CookieConfig with sensible defaults.
func DefaultCookieConfig() CookieConfig {
	return CookieConfig{
		Name:     DefaultCookieName,
		CSRFName: DefaultCSRFName,
		MaxAge:   24 * time.Hour,
	}
}

// SetAuthCookie writes the JWT as an HttpOnly, Secure, SameSite=Lax cookie.
// The Secure flag is set dynamically based on the request's transport.
func SetAuthCookie(w http.ResponseWriter, r *http.Request, token string, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.cookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(cfg.MaxAge.Seconds()),
		HttpOnly: true,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearAuthCookie expires the auth cookie.
func ClearAuthCookie(w http.ResponseWriter, r *http.Request, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.cookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// SetCSRFCookie writes a non-HttpOnly CSRF token cookie that JavaScript can read
// for the double-submit pattern.
func SetCSRFCookie(w http.ResponseWriter, r *http.Request, csrfToken string, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.csrfCookieName(),
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   int(cfg.MaxAge.Seconds()),
		HttpOnly: false,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCSRFCookie expires the CSRF cookie.
func ClearCSRFCookie(w http.ResponseWriter, r *http.Request, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.csrfCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   IsSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// GenerateCSRFToken returns a cryptographically random hex-encoded token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IsSecureRequest returns true when the request arrived over TLS or
// behind a reverse proxy that set X-Forwarded-Proto: https.
func IsSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

// TokenFromCookie extracts the auth token from the request cookie.
// Returns empty string if the cookie is absent.
func TokenFromCookie(r *http.Request, cfg CookieConfig) string {
	c, err := r.Cookie(cfg.cookieName())
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

func (c CookieConfig) cookieName() string {
	if c.Name != "" {
		return c.Name
	}
	return DefaultCookieName
}

func (c CookieConfig) csrfCookieName() string {
	if c.CSRFName != "" {
		return c.CSRFName
	}
	return DefaultCSRFName
}
