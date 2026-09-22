package certificate

import (
	"context"
	"testing"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

func TestGenerateVerificationCode(t *testing.T) {
	svc := &Service{baseURL: "https://example.com"}

	code1 := svc.generateVerificationCode("cert-123", "enrollment-456")
	code2 := svc.generateVerificationCode("cert-123", "enrollment-456")

	// Same inputs should produce same output (deterministic within the day)
	assert.Equal(t, code1, code2)
	assert.Len(t, code1, 16)

	// Different inputs should produce different codes
	code3 := svc.generateVerificationCode("cert-789", "enrollment-456")
	assert.NotEqual(t, code1, code3)
}

func ctxWithLocale(t *testing.T, lang string) context.Context {
	t.Helper()
	bundle := appi18n.MustNewBundle()
	l := i18n.NewLocalizer(bundle, lang)
	return appi18n.WithLocalizer(context.Background(), l)
}

func TestCertificate_GetCertificateData(t *testing.T) {
	// Use a fixed date so assertions are stable
	fixed := time.Date(2026, time.April, 16, 10, 0, 0, 0, time.UTC)
	cert := &Certificate{
		ID:               "cert-123",
		EnrollmentID:     "enrollment-456",
		UserID:           "user-789",
		PathwayID:        "pathway-abc",
		PathwayName:      "Cybersecurity Fundamentals",
		UserName:         "John Doe",
		UserEmail:        "john@example.com",
		EarnedPoints:     850,
		MaxPoints:        1000,
		Percentage:       85.0,
		CompletedAt:      fixed,
		IssuedAt:         fixed,
		VerificationCode: "abc123def456ghij",
		VerificationURL:  "https://example.com/verify/abc123def456ghij",
		CertificateURL:   "https://example.com/certificates/cert-123",
	}

	ctx := ctxWithLocale(t, "en")
	data := cert.GetCertificateData(ctx)

	assert.Equal(t, "John Doe", data.RecipientName)
	assert.Equal(t, "Cybersecurity Fundamentals", data.PathwayName)
	assert.Equal(t, "April 16, 2026", data.CompletionDate)
	assert.Equal(t, "April 16, 2026", data.IssueDate)
	assert.Equal(t, 850, data.EarnedPoints)
	assert.Equal(t, 1000, data.MaxPoints)
	assert.Equal(t, 85.0, data.Percentage)
	assert.Equal(t, "https://example.com/verify/abc123def456ghij", data.VerifyURL)
}

func TestCertificate_GetCertificateData_Spanish(t *testing.T) {
	fixed := time.Date(2026, time.April, 16, 10, 0, 0, 0, time.UTC)
	cert := &Certificate{
		CompletedAt: fixed,
		IssuedAt:    fixed,
	}

	ctx := ctxWithLocale(t, "es")
	data := cert.GetCertificateData(ctx)

	assert.Equal(t, "16 de April de 2026", data.CompletionDate)
	assert.Equal(t, "16 de April de 2026", data.IssueDate)
}

func TestNewService(t *testing.T) {
	cfg := Config{BaseURL: "https://lab.example.com"}
	svc := NewService(nil, nil, nil, nil, cfg, nil)

	assert.NotNil(t, svc)
	assert.Equal(t, "https://lab.example.com", svc.baseURL)
}

func TestService_IssueCertificate_NoRepos(t *testing.T) {
	svc := &Service{
		enrollmentRepo: nil,
		pathwayRepo:    nil,
	}

	cert, err := svc.IssueCertificate(t.Context(), "enrollment-123")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, cert)
}

func TestService_GetCertificate_NoRepos(t *testing.T) {
	svc := &Service{
		enrollmentRepo: nil,
		pathwayRepo:    nil,
	}

	cert, err := svc.GetCertificate(t.Context(), "enrollment-123")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, cert)
}

func TestService_ListUserCertificates_NoRepos(t *testing.T) {
	svc := &Service{
		enrollmentRepo: nil,
	}

	certs, err := svc.ListUserCertificates(t.Context(), "user-123")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, certs)
}

func TestService_VerifyCertificate_InvalidCode(t *testing.T) {
	svc := &Service{}

	// Too short
	cert, err := svc.VerifyCertificate(t.Context(), "short")
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, cert)
}

func TestErrors(t *testing.T) {
	assert.Equal(t, "enrollment not found", ErrEnrollmentNotFound.Error())
	assert.Equal(t, "pathway not completed", ErrNotCompleted.Error())
	assert.Equal(t, "certificate already issued", ErrAlreadyIssued.Error())
	assert.Equal(t, "certificate not found", ErrCertificateNotFound.Error())
	assert.Equal(t, "invalid verification code", ErrInvalidCode.Error())
	assert.Equal(t, "repository not available", ErrRepoNotAvailable.Error())
	assert.Equal(t, "certificate has been revoked", ErrCertificateRevoked.Error())
}
