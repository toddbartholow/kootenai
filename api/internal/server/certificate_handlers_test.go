package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/certificate"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// mockCertificateService is a mock implementation of CertificateService for testing
type mockCertificateService struct {
	issueCertificateFn     func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error)
	getCertificateFn       func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error)
	verifyCertificateFn    func(ctx context.Context, code string) (*certificate.Certificate, error)
	listUserCertificatesFn func(ctx context.Context, userID string) ([]*certificate.Certificate, error)
}

func (m *mockCertificateService) IssueCertificate(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
	if m.issueCertificateFn != nil {
		return m.issueCertificateFn(ctx, enrollmentID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCertificateService) GetCertificate(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
	if m.getCertificateFn != nil {
		return m.getCertificateFn(ctx, enrollmentID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCertificateService) VerifyCertificate(ctx context.Context, code string) (*certificate.Certificate, error) {
	if m.verifyCertificateFn != nil {
		return m.verifyCertificateFn(ctx, code)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCertificateService) ListUserCertificates(ctx context.Context, userID string) ([]*certificate.Certificate, error) {
	if m.listUserCertificatesFn != nil {
		return m.listUserCertificatesFn(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

// mockEnrollmentRepoForCert is a mock enrollment repository for certificate tests
type mockEnrollmentRepoForCert struct {
	getByIDFn func(ctx context.Context, id string) (*models.PathwayEnrollment, error)
}

func (m *mockEnrollmentRepoForCert) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) UnlockModule(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepoForCert) UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}

func (m *mockEnrollmentRepoForCert) RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	return nil
}

func certTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHandleIssueCertificate(t *testing.T) {
	tests := []struct {
		name             string
		enrollmentID     string
		user             *auth.User
		enrollmentRepo   repositories.EnrollmentRepository
		certService      CertificateService
		expectedStatus   int
		expectedContains string
	}{
		{
			name:           "unauthorized - no user",
			enrollmentID:   "enroll-123",
			user:           nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "service unavailable - no certificate service",
			enrollmentID:   "enroll-123",
			user:           &auth.User{ID: "user-123", Roles: []string{"student"}},
			certService:    nil,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:         "enrollment repo error",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return nil, errors.New("database error")
				},
			},
			certService:    &mockCertificateService{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:         "enrollment not found",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return nil, nil
				},
			},
			certService:    &mockCertificateService{},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:         "forbidden - not owner",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-456", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService:    &mockCertificateService{},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:         "admin can issue for others",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "admin-user", Roles: []string{"admin"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService: &mockCertificateService{
				issueCertificateFn: func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
					return &certificate.Certificate{
						ID:               "cert-123",
						EnrollmentID:     enrollmentID,
						PathwayName:      "Security Basics",
						VerificationCode: "abc123",
					}, nil
				},
			},
			expectedStatus:   http.StatusCreated,
			expectedContains: "cert-123",
		},
		{
			name:         "service error - not completed",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService: &mockCertificateService{
				issueCertificateFn: func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
					return nil, certificate.ErrNotCompleted
				},
			},
			expectedStatus:   http.StatusBadRequest,
			expectedContains: "notCompleted",
		},
		{
			name:         "service error - already issued",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService: &mockCertificateService{
				issueCertificateFn: func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
					return nil, certificate.ErrAlreadyIssued
				},
			},
			expectedStatus:   http.StatusBadRequest,
			expectedContains: "alreadyIssued",
		},
		{
			name:         "success",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService: &mockCertificateService{
				issueCertificateFn: func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
					completedAt := time.Now()
					return &certificate.Certificate{
						ID:               "cert-456",
						EnrollmentID:     enrollmentID,
						UserID:           "user-123",
						PathwayName:      "Network Security",
						VerificationCode: "verify-abc123",
						CompletedAt:      completedAt,
						IssuedAt:         time.Now(),
					}, nil
				},
			},
			expectedStatus:   http.StatusCreated,
			expectedContains: "cert-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewCertificateManager(CertificateManagerConfig{
				CertificateService: tt.certService,
				EnrollmentRepo:     tt.enrollmentRepo,
				Logger:             certTestLogger(),
			})

			router := chi.NewRouter()
			router.Post("/enrollments/{enrollmentID}/certificate", mgr.handleIssueCertificate)

			req := httptest.NewRequest(http.MethodPost, "/enrollments/"+tt.enrollmentID+"/certificate", nil)
			if tt.user != nil {
				ctx := auth.ContextWithUser(req.Context(), tt.user)
				req = req.WithContext(ctx)
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedContains != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedContains)
			}
		})
	}
}

func TestHandleGetCertificate(t *testing.T) {
	tests := []struct {
		name             string
		enrollmentID     string
		user             *auth.User
		enrollmentRepo   repositories.EnrollmentRepository
		certService      CertificateService
		expectedStatus   int
		expectedContains string
	}{
		{
			name:           "unauthorized - no user",
			enrollmentID:   "enroll-123",
			user:           nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "service unavailable - no certificate service",
			enrollmentID:   "enroll-123",
			user:           &auth.User{ID: "user-123", Roles: []string{"student"}},
			certService:    nil,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:         "enrollment repo error",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return nil, errors.New("database error")
				},
			},
			certService:    &mockCertificateService{},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:         "enrollment not found",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return nil, nil
				},
			},
			certService:    &mockCertificateService{},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:         "forbidden - not owner",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-456", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService:    &mockCertificateService{},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:         "service error - certificate not found",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService: &mockCertificateService{
				getCertificateFn: func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
					return nil, certificate.ErrCertificateNotFound
				},
			},
			expectedStatus:   http.StatusNotFound,
			expectedContains: "certificate.errors.notFound",
		},
		{
			name:         "success",
			enrollmentID: "enroll-123",
			user:         &auth.User{ID: "user-123", Roles: []string{"student"}},
			enrollmentRepo: &mockEnrollmentRepoForCert{
				getByIDFn: func(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
					return &models.PathwayEnrollment{ID: id, UserID: "user-123"}, nil
				},
			},
			certService: &mockCertificateService{
				getCertificateFn: func(ctx context.Context, enrollmentID string) (*certificate.Certificate, error) {
					return &certificate.Certificate{
						ID:               "cert-789",
						EnrollmentID:     enrollmentID,
						UserID:           "user-123",
						PathwayName:      "Security Fundamentals",
						VerificationCode: "verify-xyz789",
						CompletedAt:      time.Now(),
						IssuedAt:         time.Now(),
					}, nil
				},
			},
			expectedStatus:   http.StatusOK,
			expectedContains: "cert-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewCertificateManager(CertificateManagerConfig{
				CertificateService: tt.certService,
				EnrollmentRepo:     tt.enrollmentRepo,
				Logger:             certTestLogger(),
			})

			router := chi.NewRouter()
			router.Get("/enrollments/{enrollmentID}/certificate", mgr.handleGetCertificate)

			req := httptest.NewRequest(http.MethodGet, "/enrollments/"+tt.enrollmentID+"/certificate", nil)
			if tt.user != nil {
				ctx := auth.ContextWithUser(req.Context(), tt.user)
				req = req.WithContext(ctx)
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedContains != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedContains)
			}
		})
	}
}

func TestHandleListUserCertificates(t *testing.T) {
	tests := []struct {
		name             string
		user             *auth.User
		certService      CertificateService
		expectedStatus   int
		expectedContains string
	}{
		{
			name:           "unauthorized - no user",
			user:           nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:             "no service - returns empty list",
			user:             &auth.User{ID: "user-123", Roles: []string{"student"}},
			certService:      nil,
			expectedStatus:   http.StatusOK,
			expectedContains: `"count":0`,
		},
		{
			name: "service error",
			user: &auth.User{ID: "user-123", Roles: []string{"student"}},
			certService: &mockCertificateService{
				listUserCertificatesFn: func(ctx context.Context, userID string) ([]*certificate.Certificate, error) {
					return nil, errors.New("database error")
				},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "success - no certificates",
			user: &auth.User{ID: "user-123", Roles: []string{"student"}},
			certService: &mockCertificateService{
				listUserCertificatesFn: func(ctx context.Context, userID string) ([]*certificate.Certificate, error) {
					return nil, nil
				},
			},
			expectedStatus:   http.StatusOK,
			expectedContains: `"count":0`,
		},
		{
			name: "success - with certificates",
			user: &auth.User{ID: "user-123", Roles: []string{"student"}},
			certService: &mockCertificateService{
				listUserCertificatesFn: func(ctx context.Context, userID string) ([]*certificate.Certificate, error) {
					return []*certificate.Certificate{
						{
							ID:          "cert-1",
							PathwayName: "Security Basics",
							IssuedAt:    time.Now(),
						},
						{
							ID:          "cert-2",
							PathwayName: "Network Defense",
							IssuedAt:    time.Now(),
						},
					}, nil
				},
			},
			expectedStatus:   http.StatusOK,
			expectedContains: `"count":2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewCertificateManager(CertificateManagerConfig{
				CertificateService: tt.certService,
				Logger:             certTestLogger(),
			})

			router := chi.NewRouter()
			router.Get("/certificates", mgr.handleListUserCertificates)

			req := httptest.NewRequest(http.MethodGet, "/certificates", nil)
			if tt.user != nil {
				ctx := auth.ContextWithUser(req.Context(), tt.user)
				req = req.WithContext(ctx)
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedContains != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedContains)
			}
		})
	}
}

func TestHandleVerifyCertificate(t *testing.T) {
	tests := []struct {
		name             string
		code             string
		certService      CertificateService
		expectedStatus   int
		expectedContains string
	}{
		{
			name:           "service unavailable",
			code:           "abc123def456ghij",
			certService:    nil,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name: "invalid code",
			code: "invalid-code1234",
			certService: &mockCertificateService{
				verifyCertificateFn: func(ctx context.Context, code string) (*certificate.Certificate, error) {
					return nil, certificate.ErrInvalidCode
				},
			},
			expectedStatus: http.StatusNotFound,
			// Test invokes the handler via a bare chi.Router without the
			// locale middleware, so Localize() falls back to the message
			// ID. The English text for this ID is
			// "certificate not found or invalid verification code".
			expectedContains: "certificate.errors.notFoundOrInvalidCode",
		},
		{
			name: "certificate not found",
			code: "notfound12345678",
			certService: &mockCertificateService{
				verifyCertificateFn: func(ctx context.Context, code string) (*certificate.Certificate, error) {
					return nil, certificate.ErrCertificateNotFound
				},
			},
			expectedStatus:   http.StatusNotFound,
			expectedContains: "certificate.errors.notFoundOrInvalidCode",
		},
		{
			name: "service error",
			code: "servererror12345",
			certService: &mockCertificateService{
				verifyCertificateFn: func(ctx context.Context, code string) (*certificate.Certificate, error) {
					return nil, errors.New("internal error")
				},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "success",
			code: "valid123456789ab",
			certService: &mockCertificateService{
				verifyCertificateFn: func(ctx context.Context, code string) (*certificate.Certificate, error) {
					return &certificate.Certificate{
						ID:               "cert-verified",
						EnrollmentID:     "enroll-456",
						UserID:           "user-789",
						PathwayName:      "Advanced Security",
						VerificationCode: code,
						CompletedAt:      time.Now(),
						IssuedAt:         time.Now(),
					}, nil
				},
			},
			expectedStatus:   http.StatusOK,
			expectedContains: "cert-verified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewCertificateManager(CertificateManagerConfig{
				CertificateService: tt.certService,
				Logger:             certTestLogger(),
			})

			router := chi.NewRouter()
			router.Get("/certificates/verify/{code}", mgr.handleVerifyCertificate)

			req := httptest.NewRequest(http.MethodGet, "/certificates/verify/"+tt.code, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedContains != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedContains)
			}
		})
	}
}
