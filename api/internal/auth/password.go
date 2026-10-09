package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing.
	// 12 provides good security (~250ms on modern hardware) while remaining responsive.
	BcryptCost = 12

	// MinPasswordLength is the minimum required password length.
	MinPasswordLength = 8

	// DefaultGeneratedPasswordLength is the length of randomly generated passwords.
	DefaultGeneratedPasswordLength = 16
)

// Password validation errors
var (
	ErrPasswordTooShort    = errors.New("password must be at least 8 characters")
	ErrPasswordNoUppercase = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLowercase = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoDigit     = errors.New("password must contain at least one digit")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrPasswordRequired    = errors.New("password is required")
)

// IsPasswordValidationError returns true if the error is a user-facing password
// validation error (e.g., too short, missing uppercase). These are safe to return
// to clients. Returns false for internal errors like bcrypt failures.
func IsPasswordValidationError(err error) bool {
	return PasswordValidationMessageID(err) != ""
}

// PasswordValidationMessageID maps a password-validation sentinel to the
// catalog message ID that renders its user-facing text. Returns "" for
// non-validation errors. Pair with Responder.LocalizedErrorResponse; length
// errors need {{.Min}} template data set to MinPasswordLength.
func PasswordValidationMessageID(err error) string {
	switch {
	case errors.Is(err, ErrPasswordTooShort):
		return "auth.password.tooShort"
	case errors.Is(err, ErrPasswordNoUppercase):
		return "auth.password.noUppercase"
	case errors.Is(err, ErrPasswordNoLowercase):
		return "auth.password.noLowercase"
	case errors.Is(err, ErrPasswordNoDigit):
		return "auth.password.noDigit"
	case errors.Is(err, ErrPasswordRequired):
		return "auth.password.required"
	default:
		return ""
	}
}

// HashPassword hashes a password using bcrypt.
// It first validates password strength requirements.
func HashPassword(password string) (string, error) {
	if err := ValidatePasswordStrength(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compares a password against a bcrypt hash.
// Returns nil if the password matches, ErrInvalidPassword otherwise.
func VerifyPassword(password, hash string) error {
	if hash == "" {
		return ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}

	return nil
}

// ValidatePasswordStrength checks that a password meets complexity requirements:
// - At least 8 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUppercase
	}
	if !hasLower {
		return ErrPasswordNoLowercase
	}
	if !hasDigit {
		return ErrPasswordNoDigit
	}

	return nil
}

// GenerateRandomPassword creates a cryptographically secure random password.
// The password is guaranteed to meet strength requirements.
func GenerateRandomPassword(length int) (string, error) {
	if length < MinPasswordLength {
		length = DefaultGeneratedPasswordLength
	}

	// Character sets for password generation
	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits    = "0123456789"
		special   = "!@#$%^&*"
		allChars  = lowercase + uppercase + digits + special
	)

	password := make([]byte, length)

	// Ensure at least one character from each required set
	requiredSets := []string{lowercase, uppercase, digits}
	for i, set := range requiredSets {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		if err != nil {
			return "", fmt.Errorf("generating random password: %w", err)
		}
		password[i] = set[idx.Int64()]
	}

	// Fill remaining positions with random characters from all sets
	for i := len(requiredSets); i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(allChars))))
		if err != nil {
			return "", fmt.Errorf("generating random password: %w", err)
		}
		password[i] = allChars[idx.Int64()]
	}

	// Shuffle the password to randomize position of required characters
	for i := length - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", fmt.Errorf("shuffling password: %w", err)
		}
		j := jBig.Int64()
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

// DummyHash is used for timing attack prevention.
// When a user is not found, we still perform a hash comparison
// to prevent timing-based user enumeration. This must be a
// structurally valid bcrypt hash so CompareHashAndPassword
// performs the full constant-time comparison.
const DummyHash = "$2a$12$2pLq2vZwGR1DN9vaen4RAu3Pj0g4Zwc1KfRkx5DREkY/5TlRUUamm"
