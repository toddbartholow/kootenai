// Package certificate provides certificate generation and management for pathway completions.
package certificate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Service errors
var (
	ErrEnrollmentNotFound  = errors.New("enrollment not found")
	ErrNotCompleted        = errors.New("pathway not completed")
	ErrAlreadyIssued       = errors.New("certificate already issued")
	ErrCertificateNotFound = errors.New("certificate not found")
	ErrInvalidCode         = errors.New("invalid verification code")
	ErrRepoNotAvailable    = errors.New("repository not available")
	ErrCertificateRevoked  = errors.New("certificate has been revoked")
)

// Certificate represents a pathway completion certificate (API response type)
type Certificate struct {
	ID               string     `json:"id"`
	EnrollmentID     string     `json:"enrollmentId"`
	UserID           string     `json:"userId"`
	PathwayID        string     `json:"pathwayId"`
	PathwayName      string     `json:"pathwayName"`
	UserName         string     `json:"userName"`
	UserEmail        string     `json:"userEmail,omitempty"`
	EarnedPoints     int        `json:"earnedPoints"`
	MaxPoints        int        `json:"maxPoints"`
	Percentage       float64    `json:"percentage"`
	CompletedAt      time.Time  `json:"completedAt"`
	IssuedAt         time.Time  `json:"issuedAt"`
	VerificationCode string     `json:"verificationCode"`
	VerificationURL  string     `json:"verificationUrl"`
	CertificateURL   string     `json:"certificateUrl,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	IsValid          bool       `json:"isValid"`
}

// Service handles certificate operations
type Service struct {
	certificateRepo repositories.CertificateRepository
	enrollmentRepo  repositories.EnrollmentRepository
	pathwayRepo     repositories.PathwayRepository
	userRepo        repositories.UserRepository
	baseURL         string // Base URL for verification links
	logger          *slog.Logger
}

// Config holds certificate service configuration
type Config struct {
	BaseURL string // e.g., "https://lab.example.com"
}

// NewService creates a new certificate service
func NewService(
	certificateRepo repositories.CertificateRepository,
	enrollmentRepo repositories.EnrollmentRepository,
	pathwayRepo repositories.PathwayRepository,
	userRepo repositories.UserRepository,
	cfg Config,
	logger *slog.Logger,
) *Service {
	return &Service{
		certificateRepo: certificateRepo,
		enrollmentRepo:  enrollmentRepo,
		pathwayRepo:     pathwayRepo,
		userRepo:        userRepo,
		baseURL:         cfg.BaseURL,
		logger:          logger,
	}
}

// IssueCertificate generates a certificate for a completed pathway enrollment
func (s *Service) IssueCertificate(ctx context.Context, enrollmentID string) (*Certificate, error) {
	if s.enrollmentRepo == nil || s.pathwayRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Get enrollment with pathway details
	enrollment, err := s.enrollmentRepo.GetByID(ctx, enrollmentID)
	if err != nil {
		s.logger.Error("Failed to get enrollment", "error", err, "enrollmentId", enrollmentID)
		return nil, err
	}
	if enrollment == nil {
		return nil, ErrEnrollmentNotFound
	}

	// Check if pathway is completed
	if enrollment.Status != models.EnrollmentStatusCompleted || enrollment.CompletedAt == nil {
		return nil, ErrNotCompleted
	}

	// Check if already issued (check database if available)
	if s.certificateRepo != nil {
		existingCert, err := s.certificateRepo.GetByEnrollmentID(ctx, enrollmentID)
		if err != nil {
			s.logger.Error("Failed to check existing certificate", "error", err, "enrollmentId", enrollmentID)
			return nil, err
		}
		if existingCert != nil {
			return nil, ErrAlreadyIssued
		}
	} else if enrollment.CertificateIssued {
		return nil, ErrAlreadyIssued
	}

	// Get pathway details
	pathway, err := s.pathwayRepo.GetByID(ctx, enrollment.PathwayID)
	if err != nil {
		s.logger.Error("Failed to get pathway", "error", err, "pathwayId", enrollment.PathwayID)
		return nil, err
	}

	// Get user details
	var userName, userEmail string
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, enrollment.UserID)
		if err == nil && user != nil {
			userName = user.DisplayName
			if userName == "" {
				userName = user.Username
			}
			userEmail = user.Email
		}
	}

	// Generate certificate ID and verification code
	certID := uuid.New().String()
	verificationCode := s.generateVerificationCode(certID, enrollmentID)
	issuedAt := time.Now()

	// Persist certificate to database if repository is available
	if s.certificateRepo != nil {
		dbCert := &models.Certificate{
			ID:               certID,
			EnrollmentID:     enrollmentID,
			UserID:           enrollment.UserID,
			PathwayID:        enrollment.PathwayID,
			VerificationCode: verificationCode,
			IssuedAt:         issuedAt,
		}

		if err := s.certificateRepo.Create(ctx, dbCert); err != nil {
			s.logger.Error("Failed to persist certificate", "error", err, "enrollmentId", enrollmentID)
			return nil, fmt.Errorf("persisting certificate: %w", err)
		}
	}

	// Build API response certificate
	cert := &Certificate{
		ID:               certID,
		EnrollmentID:     enrollmentID,
		UserID:           enrollment.UserID,
		PathwayID:        enrollment.PathwayID,
		PathwayName:      pathway.Name,
		UserName:         userName,
		UserEmail:        userEmail,
		EarnedPoints:     enrollment.EarnedPoints,
		MaxPoints:        enrollment.MaxPoints,
		Percentage:       enrollment.Percentage,
		CompletedAt:      *enrollment.CompletedAt,
		IssuedAt:         issuedAt,
		VerificationCode: verificationCode,
		VerificationURL:  fmt.Sprintf("%s/verify/%s", s.baseURL, verificationCode),
		CertificateURL:   fmt.Sprintf("%s/api/v1/enrollments/%s/certificate", s.baseURL, enrollmentID),
		IsValid:          true,
	}

	// Update enrollment with certificate info
	enrollment.CertificateIssued = true
	enrollment.CertificateURL = cert.CertificateURL

	if err := s.enrollmentRepo.Update(ctx, enrollment); err != nil {
		s.logger.Error("Failed to update enrollment with certificate", "error", err, "enrollmentId", enrollmentID)
		return nil, err
	}

	s.logger.Info("Certificate issued",
		"certificateId", certID,
		"enrollmentId", enrollmentID,
		"userId", enrollment.UserID,
		"pathwayId", enrollment.PathwayID,
		"verificationCode", verificationCode,
	)

	return cert, nil
}

// GetCertificate retrieves a certificate by enrollment ID
func (s *Service) GetCertificate(ctx context.Context, enrollmentID string) (*Certificate, error) {
	if s.enrollmentRepo == nil || s.pathwayRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	// Try to get from certificate repository first
	if s.certificateRepo != nil {
		dbCert, err := s.certificateRepo.GetByEnrollmentID(ctx, enrollmentID)
		if err != nil {
			s.logger.Error("Failed to get certificate from database", "error", err, "enrollmentId", enrollmentID)
			return nil, err
		}
		if dbCert != nil {
			return s.buildCertificateFromDB(ctx, dbCert)
		}
	}

	// Fallback to enrollment-based lookup (for backwards compatibility)
	enrollment, err := s.enrollmentRepo.GetByID(ctx, enrollmentID)
	if err != nil {
		return nil, err
	}
	if enrollment == nil {
		return nil, ErrEnrollmentNotFound
	}

	if !enrollment.CertificateIssued || enrollment.CertificateURL == "" {
		return nil, ErrCertificateNotFound
	}

	// Get pathway details
	pathway, err := s.pathwayRepo.GetByID(ctx, enrollment.PathwayID)
	if err != nil {
		return nil, err
	}

	// Get user details
	var userName, userEmail string
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, enrollment.UserID)
		if err == nil && user != nil {
			userName = user.DisplayName
			if userName == "" {
				userName = user.Username
			}
			userEmail = user.Email
		}
	}

	// Generate a legacy certificate response (without database-backed verification code)
	certID := uuid.New().String()
	verificationCode := s.generateVerificationCode(certID, enrollmentID)

	return &Certificate{
		ID:               certID,
		EnrollmentID:     enrollmentID,
		UserID:           enrollment.UserID,
		PathwayID:        enrollment.PathwayID,
		PathwayName:      pathway.Name,
		UserName:         userName,
		UserEmail:        userEmail,
		EarnedPoints:     enrollment.EarnedPoints,
		MaxPoints:        enrollment.MaxPoints,
		Percentage:       enrollment.Percentage,
		CompletedAt:      *enrollment.CompletedAt,
		IssuedAt:         *enrollment.CompletedAt, // Approximate
		VerificationCode: verificationCode,
		VerificationURL:  fmt.Sprintf("%s/verify/%s", s.baseURL, verificationCode),
		CertificateURL:   enrollment.CertificateURL,
		IsValid:          true,
	}, nil
}

// VerifyCertificate validates a certificate verification code
func (s *Service) VerifyCertificate(ctx context.Context, verificationCode string) (*Certificate, error) {
	if len(verificationCode) < 16 {
		return nil, ErrInvalidCode
	}

	// Look up certificate by verification code in database
	if s.certificateRepo != nil {
		certWithDetails, err := s.certificateRepo.GetWithDetailsByVerificationCode(ctx, verificationCode)
		if err != nil {
			s.logger.Error("Failed to verify certificate", "error", err, "code", verificationCode[:8]+"...")
			return nil, err
		}
		if certWithDetails != nil {
			// Check if revoked
			if certWithDetails.RevokedAt != nil {
				return nil, ErrCertificateRevoked
			}

			// Check if expired
			if certWithDetails.ExpiresAt != nil && time.Now().After(*certWithDetails.ExpiresAt) {
				return nil, ErrCertificateNotFound
			}

			completedAt := time.Time{}
			if certWithDetails.CompletedAt != nil {
				completedAt = *certWithDetails.CompletedAt
			}

			return &Certificate{
				ID:               certWithDetails.ID,
				EnrollmentID:     certWithDetails.EnrollmentID,
				UserID:           certWithDetails.UserID,
				PathwayID:        certWithDetails.PathwayID,
				PathwayName:      certWithDetails.PathwayName,
				UserName:         certWithDetails.UserName,
				UserEmail:        "", // Don't expose email in public verification
				EarnedPoints:     certWithDetails.EarnedPoints,
				MaxPoints:        certWithDetails.MaxPoints,
				Percentage:       certWithDetails.Percentage,
				CompletedAt:      completedAt,
				IssuedAt:         certWithDetails.IssuedAt,
				VerificationCode: certWithDetails.VerificationCode,
				VerificationURL:  fmt.Sprintf("%s/verify/%s", s.baseURL, certWithDetails.VerificationCode),
				CertificateURL:   fmt.Sprintf("%s/api/v1/enrollments/%s/certificate", s.baseURL, certWithDetails.EnrollmentID),
				ExpiresAt:        certWithDetails.ExpiresAt,
				IsValid:          certWithDetails.Certificate.IsValid(),
			}, nil
		}
	}

	s.logger.Info("Certificate verification attempted - not found", "code", verificationCode[:8]+"...")
	return nil, ErrCertificateNotFound
}

// ListUserCertificates returns all certificates for a user
func (s *Service) ListUserCertificates(ctx context.Context, userID string) ([]*Certificate, error) {
	// Use certificate repository if available
	if s.certificateRepo != nil {
		dbCerts, err := s.certificateRepo.ListWithDetailsByUserID(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to list certificates", "error", err, "userId", userID)
			return nil, err
		}

		var certificates []*Certificate
		for _, dbCert := range dbCerts {
			// Skip revoked certificates
			if dbCert.RevokedAt != nil {
				continue
			}

			completedAt := time.Time{}
			if dbCert.CompletedAt != nil {
				completedAt = *dbCert.CompletedAt
			}

			cert := &Certificate{
				ID:               dbCert.ID,
				EnrollmentID:     dbCert.EnrollmentID,
				UserID:           dbCert.UserID,
				PathwayID:        dbCert.PathwayID,
				PathwayName:      dbCert.PathwayName,
				UserName:         dbCert.UserName,
				UserEmail:        dbCert.UserEmail,
				EarnedPoints:     dbCert.EarnedPoints,
				MaxPoints:        dbCert.MaxPoints,
				Percentage:       dbCert.Percentage,
				CompletedAt:      completedAt,
				IssuedAt:         dbCert.IssuedAt,
				VerificationCode: dbCert.VerificationCode,
				VerificationURL:  fmt.Sprintf("%s/verify/%s", s.baseURL, dbCert.VerificationCode),
				CertificateURL:   fmt.Sprintf("%s/api/v1/enrollments/%s/certificate", s.baseURL, dbCert.EnrollmentID),
				ExpiresAt:        dbCert.ExpiresAt,
				IsValid:          dbCert.Certificate.IsValid(),
			}
			certificates = append(certificates, cert)
		}

		return certificates, nil
	}

	// Fallback to enrollment-based lookup
	if s.enrollmentRepo == nil {
		return nil, ErrRepoNotAvailable
	}

	enrollments, err := s.enrollmentRepo.List(ctx, models.EnrollmentListOptions{
		UserID: userID,
		Status: models.EnrollmentStatusCompleted,
	})
	if err != nil {
		return nil, err
	}

	var certificates []*Certificate
	for _, enrollment := range enrollments {
		if enrollment.CertificateIssued {
			cert, err := s.GetCertificate(ctx, enrollment.ID)
			if err == nil && cert != nil {
				certificates = append(certificates, cert)
			}
		}
	}

	return certificates, nil
}

// buildCertificateFromDB creates a Certificate response from a database certificate
func (s *Service) buildCertificateFromDB(ctx context.Context, dbCert *models.Certificate) (*Certificate, error) {
	// Get enrollment for points/percentage
	enrollment, err := s.enrollmentRepo.GetByID(ctx, dbCert.EnrollmentID)
	if err != nil {
		return nil, err
	}
	if enrollment == nil {
		return nil, ErrEnrollmentNotFound
	}

	// Get pathway name
	pathway, err := s.pathwayRepo.GetByID(ctx, dbCert.PathwayID)
	if err != nil {
		return nil, err
	}

	// Get user details
	var userName, userEmail string
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, dbCert.UserID)
		if err == nil && user != nil {
			userName = user.DisplayName
			if userName == "" {
				userName = user.Username
			}
			userEmail = user.Email
		}
	}

	completedAt := time.Time{}
	if enrollment.CompletedAt != nil {
		completedAt = *enrollment.CompletedAt
	}

	return &Certificate{
		ID:               dbCert.ID,
		EnrollmentID:     dbCert.EnrollmentID,
		UserID:           dbCert.UserID,
		PathwayID:        dbCert.PathwayID,
		PathwayName:      pathway.Name,
		UserName:         userName,
		UserEmail:        userEmail,
		EarnedPoints:     enrollment.EarnedPoints,
		MaxPoints:        enrollment.MaxPoints,
		Percentage:       enrollment.Percentage,
		CompletedAt:      completedAt,
		IssuedAt:         dbCert.IssuedAt,
		VerificationCode: dbCert.VerificationCode,
		VerificationURL:  fmt.Sprintf("%s/verify/%s", s.baseURL, dbCert.VerificationCode),
		CertificateURL:   fmt.Sprintf("%s/api/v1/enrollments/%s/certificate", s.baseURL, dbCert.EnrollmentID),
		ExpiresAt:        dbCert.ExpiresAt,
		IsValid:          dbCert.IsValid(),
	}, nil
}

// generateVerificationCode creates a unique verification code for a certificate
func (s *Service) generateVerificationCode(certID, enrollmentID string) string {
	// Use a stable hash based on certID and enrollmentID (without date for consistency)
	data := certID + ":" + enrollmentID
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

// CertificateData returns data suitable for PDF generation
type CertificateData struct {
	RecipientName  string
	PathwayName    string
	CompletionDate string
	EarnedPoints   int
	MaxPoints      int
	Percentage     float64
	VerifyURL      string
	IssueDate      string
}

// GetCertificateData prepares data for rendering a certificate PDF or HTML.
// Dates are formatted through the i18n catalog so the output respects the
// caller's locale (e.g. "April 16, 2026" in English vs "16 de abril de 2026"
// in Spanish). When ctx carries no localizer the English fallback fires.
func (c *Certificate) GetCertificateData(ctx context.Context) CertificateData {
	return CertificateData{
		RecipientName:  c.UserName,
		PathwayName:    c.PathwayName,
		CompletionDate: localizeDate(ctx, "certificate.render.completionDate", c.CompletedAt),
		EarnedPoints:   c.EarnedPoints,
		MaxPoints:      c.MaxPoints,
		Percentage:     c.Percentage,
		VerifyURL:      c.VerificationURL,
		IssueDate:      localizeDate(ctx, "certificate.render.issuedDate", c.IssuedAt),
	}
}

// localizeDate formats a time.Time through the i18n catalog using the given
// message ID. The template receives Month (full English month name for the
// English catalog; the Spanish catalog reorders the tokens), Day, and Year.
func localizeDate(ctx context.Context, messageID string, t time.Time) string {
	return appi18n.Localize(ctx, messageID, map[string]any{
		"Month": t.Month().String(),
		"Day":   strconv.Itoa(t.Day()),
		"Year":  strconv.Itoa(t.Year()),
	})
}
