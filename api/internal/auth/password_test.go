package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid password",
			password: "SecurePass123",
			wantErr:  false,
		},
		{
			name:     "password with special chars",
			password: "Secure!@#$%Pass1",
			wantErr:  false,
		},
		{
			name:     "minimum valid password",
			password: "Abcdef1g",
			wantErr:  false,
		},
		{
			name:     "password too short",
			password: "Ab1cdef",
			wantErr:  true,
			errMsg:   "at least 8 characters",
		},
		{
			name:     "password no uppercase",
			password: "abcdefgh1",
			wantErr:  true,
			errMsg:   "uppercase",
		},
		{
			name:     "password no lowercase",
			password: "ABCDEFGH1",
			wantErr:  true,
			errMsg:   "lowercase",
		},
		{
			name:     "password no digit",
			password: "Abcdefghij",
			wantErr:  true,
			errMsg:   "digit",
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
			errMsg:   "at least 8 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if hash == "" {
					t.Error("expected non-empty hash")
				}
				// Bcrypt hashes start with $2a$ or $2b$
				if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
					t.Errorf("hash should be bcrypt format, got: %s", hash[:10])
				}
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	// Create a known hash for testing
	validPassword := "TestPassword123"
	hash, err := HashPassword(validPassword)
	if err != nil {
		t.Fatalf("failed to hash password for test setup: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		wantErr  bool
	}{
		{
			name:     "correct password",
			password: validPassword,
			hash:     hash,
			wantErr:  false,
		},
		{
			name:     "wrong password",
			password: "WrongPassword123",
			hash:     hash,
			wantErr:  true,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			wantErr:  true,
		},
		{
			name:     "empty hash",
			password: validPassword,
			hash:     "",
			wantErr:  true,
		},
		{
			name:     "invalid hash format",
			password: validPassword,
			hash:     "not-a-valid-bcrypt-hash",
			wantErr:  true,
		},
		{
			name:     "dummy hash for timing attack prevention",
			password: "AnyPassword123",
			hash:     DummyHash,
			wantErr:  true, // DummyHash should never match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.password, tt.hash)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid password - all requirements",
			password: "MySecure1Password",
			wantErr:  false,
		},
		{
			name:     "valid - minimum length",
			password: "Abcde12f",
			wantErr:  false,
		},
		{
			name:     "valid - special characters",
			password: "Test!@#123",
			wantErr:  false,
		},
		{
			name:     "too short",
			password: "Ab1cdef",
			wantErr:  true,
			errMsg:   "at least 8 characters",
		},
		{
			name:     "no uppercase",
			password: "abcdefgh1",
			wantErr:  true,
			errMsg:   "uppercase",
		},
		{
			name:     "no lowercase",
			password: "ABCDEFGH1",
			wantErr:  true,
			errMsg:   "lowercase",
		},
		{
			name:     "no digit",
			password: "ABCDefghij",
			wantErr:  true,
			errMsg:   "digit",
		},
		{
			name:     "empty",
			password: "",
			wantErr:  true,
			errMsg:   "at least 8 characters",
		},
		{
			name:     "only numbers",
			password: "12345678",
			wantErr:  true,
			errMsg:   "uppercase",
		},
		{
			name:     "only lowercase",
			password: "abcdefghij",
			wantErr:  true,
			errMsg:   "uppercase",
		},
		{
			name:     "only uppercase",
			password: "ABCDEFGHIJ",
			wantErr:  true,
			errMsg:   "lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			name:    "standard length 16",
			length:  16,
			wantErr: false,
		},
		{
			name:    "minimum length 8",
			length:  8,
			wantErr: false,
		},
		{
			name:    "longer length 32",
			length:  32,
			wantErr: false,
		},
		{
			name:    "below minimum uses minimum length",
			length:  4,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := GenerateRandomPassword(tt.length)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				// GenerateRandomPassword uses default (16) if length < MinPasswordLength (8)
				expectedLen := tt.length
				if expectedLen < MinPasswordLength {
					expectedLen = DefaultGeneratedPasswordLength
				}
				if len(password) != expectedLen {
					t.Errorf("expected length %d, got %d", expectedLen, len(password))
				}
			}
		})
	}

	// Test uniqueness - generate multiple passwords and ensure they're different
	t.Run("uniqueness", func(t *testing.T) {
		passwords := make(map[string]bool)
		for i := 0; i < 100; i++ {
			pwd, err := GenerateRandomPassword(16)
			if err != nil {
				t.Fatalf("failed to generate password: %v", err)
			}
			if passwords[pwd] {
				t.Error("generated duplicate password")
			}
			passwords[pwd] = true
		}
	})

	// Test that generated passwords often meet strength requirements (probabilistic)
	t.Run("strength requirements often met", func(t *testing.T) {
		validCount := 0
		iterations := 100
		for i := 0; i < iterations; i++ {
			pwd, err := GenerateRandomPassword(16)
			if err != nil {
				t.Fatalf("failed to generate password: %v", err)
			}
			if ValidatePasswordStrength(pwd) == nil {
				validCount++
			}
		}
		// With 16 chars and mixed charset, should usually be valid
		// Allow some tolerance since it's random
		if validCount < iterations*90/100 {
			t.Errorf("expected at least 90%% valid passwords, got %d/%d", validCount, iterations)
		}
	})
}

func TestDummyHash(t *testing.T) {
	// DummyHash should be a valid bcrypt format
	if !strings.HasPrefix(DummyHash, "$2a$") {
		t.Error("DummyHash should be a valid bcrypt hash")
	}

	// DummyHash must be structurally valid so bcrypt performs a full
	// constant-time comparison (not an early format-error return).
	cost, err := bcrypt.Cost([]byte(DummyHash))
	if err != nil {
		t.Fatalf("DummyHash is not a valid bcrypt hash: %v", err)
	}
	if cost != 12 {
		t.Errorf("DummyHash cost = %d, want 12", cost)
	}

	// DummyHash should never match any reasonable password
	// This is important for timing attack prevention
	commonPasswords := []string{
		"password",
		"Password1",
		"12345678",
		"admin",
		"test",
		"",
	}

	for _, pwd := range commonPasswords {
		err := VerifyPassword(pwd, DummyHash)
		if err == nil {
			t.Errorf("DummyHash should not match password %q", pwd)
		}
	}
}

func TestHashPasswordCost(t *testing.T) {
	// Verify the hash uses the expected cost factor
	password := "TestPassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// Bcrypt hash format: $2a$<cost>$<22-char-salt><31-char-hash>
	// The cost is after the second $
	parts := strings.Split(hash, "$")
	if len(parts) < 3 {
		t.Fatalf("invalid hash format: %s", hash)
	}

	cost := parts[2]
	expectedCost := "12" // BcryptCost constant
	if cost != expectedCost {
		t.Errorf("expected cost %s, got %s", expectedCost, cost)
	}
}
