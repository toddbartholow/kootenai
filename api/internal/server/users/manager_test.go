package users

import (
	"testing"
)

// -----------------------------------------------------------------------------
// Utility Function Tests
// -----------------------------------------------------------------------------

func TestGenerateSecureToken(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "16 bytes",
			length: 16,
		},
		{
			name:   "32 bytes",
			length: 32,
		},
		{
			name:   "64 bytes",
			length: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := generateSecureToken(tt.length)
			if err != nil {
				t.Fatalf("generateSecureToken(%d) error = %v", tt.length, err)
			}

			// Token should be hex-encoded, so length is 2 * input length
			expectedLen := tt.length * 2
			if len(token) != expectedLen {
				t.Errorf("generateSecureToken(%d) len = %d, want %d", tt.length, len(token), expectedLen)
			}

			// Token should only contain hex characters
			for _, c := range token {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("generateSecureToken(%d) contains non-hex character: %c", tt.length, c)
				}
			}
		})
	}

	// Test that tokens are unique
	t.Run("tokens are unique", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := generateSecureToken(32)
			if err != nil {
				t.Fatalf("generateSecureToken(32) error = %v", err)
			}
			if tokens[token] {
				t.Error("generateSecureToken produced duplicate token")
			}
			tokens[token] = true
		}
	})
}

func TestHashToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "simple token",
			token: "abc123",
		},
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "long token",
			token: "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := hashToken(tt.token)

			// SHA256 produces 32 bytes, hex-encoded = 64 characters
			if len(hash) != 64 {
				t.Errorf("hashToken(%q) len = %d, want 64", tt.token, len(hash))
			}

			// Hash should only contain hex characters
			for _, c := range hash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("hashToken(%q) contains non-hex character: %c", tt.token, c)
				}
			}

			// Same input should produce same hash (deterministic)
			hash2 := hashToken(tt.token)
			if hash != hash2 {
				t.Errorf("hashToken(%q) not deterministic: %s != %s", tt.token, hash, hash2)
			}
		})
	}

	// Different inputs should produce different hashes
	t.Run("different inputs different hashes", func(t *testing.T) {
		hash1 := hashToken("token1")
		hash2 := hashToken("token2")
		if hash1 == hash2 {
			t.Error("hashToken produced same hash for different inputs")
		}
	})
}
