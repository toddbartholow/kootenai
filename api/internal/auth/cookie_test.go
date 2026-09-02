package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetAuthCookie(t *testing.T) {
	cfg := DefaultCookieConfig()
	cfg.MaxAge = 2 * time.Hour

	tests := []struct {
		name     string
		secure   bool
		wantAttr string
	}{
		{"plain HTTP", false, ""},
		{"HTTPS via TLS", true, "Secure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.secure {
				r.TLS = &tls.ConnectionState{}
			}

			SetAuthCookie(w, r, "test-jwt-token", cfg)

			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("expected 1 cookie, got %d", len(cookies))
			}
			c := cookies[0]
			if c.Name != DefaultCookieName {
				t.Errorf("name = %q, want %q", c.Name, DefaultCookieName)
			}
			if c.Value != "test-jwt-token" {
				t.Errorf("value = %q, want %q", c.Value, "test-jwt-token")
			}
			if !c.HttpOnly {
				t.Error("expected HttpOnly")
			}
			if c.Path != "/" {
				t.Errorf("path = %q, want /", c.Path)
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax", c.SameSite)
			}
			if c.Secure != tt.secure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.secure)
			}
			if c.MaxAge != 7200 {
				t.Errorf("MaxAge = %d, want 7200", c.MaxAge)
			}
		})
	}
}

func TestClearAuthCookie(t *testing.T) {
	cfg := DefaultCookieConfig()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	ClearAuthCookie(w, r, cfg)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != DefaultCookieName {
		t.Errorf("name = %q, want %q", c.Name, DefaultCookieName)
	}
	if c.Value != "" {
		t.Errorf("value = %q, want empty", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
}

func TestSetCSRFCookie(t *testing.T) {
	cfg := DefaultCookieConfig()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	SetCSRFCookie(w, r, "csrf-token-value", cfg)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != DefaultCSRFName {
		t.Errorf("name = %q, want %q", c.Name, DefaultCSRFName)
	}
	if c.HttpOnly {
		t.Error("CSRF cookie must NOT be HttpOnly (JS needs to read it)")
	}
	if c.Value != "csrf-token-value" {
		t.Errorf("value = %q, want %q", c.Value, "csrf-token-value")
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token1, err := GenerateCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(token1))
	}

	token2, err := GenerateCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if token1 == token2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestIsSecureRequest(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(r *http.Request)
		expect bool
	}{
		{
			"plain HTTP",
			func(r *http.Request) {},
			false,
		},
		{
			"direct TLS",
			func(r *http.Request) { r.TLS = &tls.ConnectionState{} },
			true,
		},
		{
			"X-Forwarded-Proto https",
			func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") },
			true,
		},
		{
			"X-Forwarded-Proto http",
			func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "http") },
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(r)
			if got := IsSecureRequest(r); got != tt.expect {
				t.Errorf("IsSecureRequest() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestTokenFromCookie(t *testing.T) {
	cfg := DefaultCookieConfig()

	t.Run("cookie present", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: "my-token"})
		if got := TokenFromCookie(r, cfg); got != "my-token" {
			t.Errorf("got %q, want %q", got, "my-token")
		}
	})

	t.Run("cookie absent", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if got := TokenFromCookie(r, cfg); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("cookie empty value", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: ""})
		if got := TokenFromCookie(r, cfg); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
