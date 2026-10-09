package server

import (
	"testing"
)

func TestIsAllowedRedirect(t *testing.T) {
	h := &OAuth2Handlers{
		config: OAuth2Config{
			AllowedRedirects: []string{
				"https://app.example.com",
				"http://localhost:3000",
			},
			BaseURL: "https://lab.example.com",
		},
	}

	tests := []struct {
		name        string
		redirectURL string
		want        bool
	}{
		// Relative URLs — always allowed
		{"relative root", "/", true},
		{"relative path", "/dashboard", true},
		{"relative deep path", "/labs/session/123", true},

		// Protocol-relative — must be blocked
		{"protocol-relative attack", "//evil.com/steal", false},

		// Exact host matches — allowed
		{"exact match https", "https://app.example.com/callback", true},
		{"exact match localhost", "http://localhost:3000/auth", true},
		{"base URL match", "https://lab.example.com/dashboard", true},

		// Prefix bypass attempts — MUST be blocked (this was the vulnerability)
		{"prefix bypass .evil.com", "https://app.example.com.evil.com/steal", false},
		{"prefix bypass subdomain", "https://app.example.com-evil.com/steal", false},

		// Scheme mismatch — blocked
		{"scheme mismatch http vs https", "http://app.example.com/callback", false},
		{"scheme mismatch on base", "http://lab.example.com/dashboard", false},

		// Port mismatch — blocked
		{"port mismatch", "http://localhost:9999/callback", false},
		{"added port", "https://app.example.com:8443/callback", false},

		// Completely different domain — blocked
		{"different domain", "https://evil.com/steal", false},

		// Invalid URLs — blocked
		{"empty string", "", false},
		{"just a word", "notaurl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.isAllowedRedirect(tt.redirectURL)
			if got != tt.want {
				t.Errorf("isAllowedRedirect(%q) = %v, want %v", tt.redirectURL, got, tt.want)
			}
		})
	}
}

func TestIsAllowedRedirect_NoConfig(t *testing.T) {
	h := &OAuth2Handlers{
		config: OAuth2Config{},
	}

	tests := []struct {
		name        string
		redirectURL string
		want        bool
	}{
		{"relative still allowed", "/dashboard", true},
		{"absolute blocked", "https://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.isAllowedRedirect(tt.redirectURL)
			if got != tt.want {
				t.Errorf("isAllowedRedirect(%q) = %v, want %v", tt.redirectURL, got, tt.want)
			}
		})
	}
}
