package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/certificate"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// ---------------------------------------------------------------------------
// CertificateManager
// ---------------------------------------------------------------------------

// CertificateManagerConfig holds configuration for creating a CertificateManager.
type CertificateManagerConfig struct {
	CertificateService CertificateService
	EnrollmentRepo     repositories.EnrollmentRepository
	Logger             *slog.Logger
}

// CertificateManager owns all certificate HTTP handlers.
type CertificateManager struct {
	responder          *httputil.Responder
	certificateService CertificateService
	enrollmentRepo     repositories.EnrollmentRepository
	logger             *slog.Logger
}

// NewCertificateManager creates a CertificateManager from the given config.
func NewCertificateManager(cfg CertificateManagerConfig) *CertificateManager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &CertificateManager{
		responder:          httputil.NewResponder(logger),
		certificateService: cfg.CertificateService,
		enrollmentRepo:     cfg.EnrollmentRepo,
		logger:             logger,
	}
}

// handleIssueCertificate issues a certificate for a completed pathway enrollment
// @Summary Issue Certificate
// @Description Issue a completion certificate for a pathway enrollment
// @Tags certificates
// @Accept json
// @Produce json
// @Param enrollmentID path string true "Enrollment ID"
// @Success 201 {object} certificate.Certificate
// @Failure 400 {object} ErrorResponse "Pathway not completed or certificate already issued"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Enrollment not found"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /enrollments/{enrollmentID}/certificate [post]
func (m *CertificateManager) handleIssueCertificate(w http.ResponseWriter, r *http.Request) {
	enrollmentID := chi.URLParam(r, "enrollmentID")

	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	if m.certificateService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "certificate.errors.serviceNotAvailable", nil)
		return
	}

	// Verify user owns the enrollment
	if m.enrollmentRepo != nil {
		enrollment, err := m.enrollmentRepo.GetByID(ctx, enrollmentID)
		if err != nil {
			m.logger.Error("Failed to get enrollment", "error", err, "enrollmentId", enrollmentID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "certificate.errors.verifyEnrollmentFailed", nil)
			return
		}
		if enrollment == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "certificate.errors.enrollmentNotFound", nil)
			return
		}
		if enrollment.UserID != user.ID && !isAdminOrInstructor(user) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
			return
		}
	}

	cert, err := m.certificateService.IssueCertificate(ctx, enrollmentID)
	if err != nil {
		status := http.StatusInternalServerError
		msgID := "certificate.errors.issueFailed"

		if errors.Is(err, certificate.ErrEnrollmentNotFound) {
			status = http.StatusNotFound
			msgID = "certificate.errors.enrollmentNotFound"
		} else if errors.Is(err, certificate.ErrNotCompleted) {
			status = http.StatusBadRequest
			msgID = "certificate.errors.notCompleted"
		} else if errors.Is(err, certificate.ErrAlreadyIssued) {
			status = http.StatusBadRequest
			msgID = "certificate.errors.alreadyIssued"
		}

		m.responder.LocalizedErrorResponse(r.Context(), w, status, msgID, nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusCreated, cert)
}

// handleGetCertificate retrieves a certificate for an enrollment
// @Summary Get Certificate
// @Description Get a completion certificate for a pathway enrollment
// @Tags certificates
// @Produce json
// @Param enrollmentID path string true "Enrollment ID"
// @Success 200 {object} certificate.Certificate
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Certificate not found"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /enrollments/{enrollmentID}/certificate [get]
func (m *CertificateManager) handleGetCertificate(w http.ResponseWriter, r *http.Request) {
	enrollmentID := chi.URLParam(r, "enrollmentID")

	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	if m.certificateService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "certificate.errors.serviceNotAvailable", nil)
		return
	}

	// Verify user owns the enrollment or is admin
	if m.enrollmentRepo != nil {
		enrollment, err := m.enrollmentRepo.GetByID(ctx, enrollmentID)
		if err != nil {
			m.logger.Error("Failed to get enrollment", "error", err, "enrollmentId", enrollmentID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "certificate.errors.verifyEnrollmentFailed", nil)
			return
		}
		if enrollment == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "certificate.errors.enrollmentNotFound", nil)
			return
		}
		if enrollment.UserID != user.ID && !isAdminOrInstructor(user) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
			return
		}
	}

	cert, err := m.certificateService.GetCertificate(ctx, enrollmentID)
	if err != nil {
		status := http.StatusInternalServerError
		msgID := "certificate.errors.getFailed"

		if errors.Is(err, certificate.ErrEnrollmentNotFound) {
			status = http.StatusNotFound
			msgID = "certificate.errors.enrollmentNotFound"
		} else if errors.Is(err, certificate.ErrCertificateNotFound) {
			status = http.StatusNotFound
			msgID = "certificate.errors.notFound"
		}

		m.responder.LocalizedErrorResponse(r.Context(), w, status, msgID, nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, cert)
}

// handleListUserCertificates lists all certificates for the authenticated user
// @Summary List User Certificates
// @Description Get all certificates for the authenticated user
// @Tags certificates
// @Produce json
// @Success 200 {object} map[string]any "certificates and count"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /certificates [get]
func (m *CertificateManager) handleListUserCertificates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	if m.certificateService == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"certificates": []any{},
			"count":        0,
		})
		return
	}

	certs, err := m.certificateService.ListUserCertificates(ctx, user.ID)
	if err != nil {
		m.logger.Error("Failed to list certificates", "error", err, "userId", user.ID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "certificate.errors.listFailed", nil)
		return
	}

	if certs == nil {
		certs = []*certificate.Certificate{}
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"certificates": certs,
		"count":        len(certs),
	})
}

// handleVerifyCertificate verifies a certificate by its verification code
// @Summary Verify Certificate
// @Description Verify a certificate using its verification code (public endpoint)
// @Tags certificates
// @Produce json
// @Param code path string true "Verification Code"
// @Success 200 {object} certificate.Certificate
// @Failure 404 {object} ErrorResponse "Certificate not found or invalid code"
// @Router /certificates/verify/{code} [get]
func (m *CertificateManager) handleVerifyCertificate(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	if m.certificateService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "certificate.errors.serviceNotAvailable", nil)
		return
	}

	cert, err := m.certificateService.VerifyCertificate(r.Context(), code)
	if err != nil {
		if errors.Is(err, certificate.ErrInvalidCode) || errors.Is(err, certificate.ErrCertificateNotFound) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "certificate.errors.notFoundOrInvalidCode", nil)
			return
		}
		m.logger.Error("Failed to verify certificate", "error", err, "code", code[:8])
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "certificate.errors.verifyFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, cert)
}
