// Package pods provides pod HTTP handlers.
package pods

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server/consoleaccess"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// VMOperationRecorder is the subset of metrics API needed to observe VM
// operation latency. Implemented by *metrics.Metrics; nil means metrics are
// disabled.
type VMOperationRecorder interface {
	VMOperationDuration(operation string, duration time.Duration)
}

// MetricsRecorder exposes the configured recorder so the composition root can
// be tested; a missing wire silently zeroes all VM-operation latency. No
// production caller.
func (m *Manager) MetricsRecorder() VMOperationRecorder { return m.metrics }

// Manager manages pod operations including lifecycle and VM control
type Manager struct {
	orchestrator      orchestrator.Client
	asyncProvisioner  *orchestrator.AsyncProvisioner
	labTemplateRepo   repositories.LabTemplateRepository
	userRepo          repositories.UserRepository
	consoleMgr        *consoleaccess.Manager
	logger            *slog.Logger
	responder         *httputil.Responder
	permissionService *custommiddleware.PermissionService
	ownershipService  *custommiddleware.OwnershipService
	rateLimiter       podRateLimiter
	rateLimitConfig   RateLimitConfig
	metrics           VMOperationRecorder
}

// podRateLimiter is the subset of rate limiter API needed by Manager.
type podRateLimiter interface {
	PerIPMiddleware(limit int64, window time.Duration) func(http.Handler) http.Handler
}

// RateLimitConfig holds rate limiting settings for pod operations.
type RateLimitConfig struct {
	Enabled          bool
	PodCreatePerHour int64
}

// Config configures Manager
type Config struct {
	Orchestrator      orchestrator.Client
	AsyncProvisioner  *orchestrator.AsyncProvisioner
	LabTemplateRepo   repositories.LabTemplateRepository
	UserRepo          repositories.UserRepository
	ConsoleMgr        *consoleaccess.Manager
	Logger            *slog.Logger
	Responder         *httputil.Responder
	PermissionService *custommiddleware.PermissionService
	OwnershipService  *custommiddleware.OwnershipService
	RateLimiter       podRateLimiter
	RateLimitConfig   RateLimitConfig
	Metrics           VMOperationRecorder
}

// AsPodRateLimiter safely converts a concrete rate limiter pointer to the interface.
// Returns nil if the underlying pointer is nil, preventing the Go nil-interface trap.
func AsPodRateLimiter(rl *redisclient.APIRateLimiter) podRateLimiter {
	if rl == nil {
		return nil
	}
	return rl
}

// NewManager creates a new Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		orchestrator:      cfg.Orchestrator,
		asyncProvisioner:  cfg.AsyncProvisioner,
		labTemplateRepo:   cfg.LabTemplateRepo,
		userRepo:          cfg.UserRepo,
		consoleMgr:        cfg.ConsoleMgr,
		logger:            cfg.Logger,
		responder:         cfg.Responder,
		permissionService: cfg.PermissionService,
		ownershipService:  cfg.OwnershipService,
		rateLimiter:       cfg.RateLimiter,
		rateLimitConfig:   cfg.RateLimitConfig,
		metrics:           cfg.Metrics,
	}
}

// SetupRoutes registers pod-related routes on the router
func (m *Manager) SetupRoutes(r chi.Router) {
	ps := m.permissionService

	r.Route("/pods", func(r chi.Router) {
		r.Get("/", m.handleListPods())
		// Pod creation has stricter rate limiting (expensive operation)
		r.Group(func(r chi.Router) {
			if m.rateLimiter != nil && m.rateLimitConfig.Enabled {
				r.Use(m.rateLimiter.PerIPMiddleware(
					m.rateLimitConfig.PodCreatePerHour,
					time.Hour,
				))
			}
			if ps != nil {
				r.Use(ps.RequirePermission(custommiddleware.PermPodsCreate))
			}
			r.Post("/", m.handleCreatePod())
			// Async pod creation - returns immediately with NATS subject for status updates
			r.Post("/async", m.handleCreatePodAsync())
		})

		// Pod-specific routes with ownership validation
		r.Route("/{podID}", func(r chi.Router) {
			if m.ownershipService != nil {
				r.Use(m.ownershipService.PodOwnershipMiddleware())
			} else {
				m.logger.Warn("Pod routes configured without ownership service — resource access control is disabled")
			}
			r.Get("/", m.handleGetPod())
			r.With(requirePerm(ps, custommiddleware.PermPodsDelete)).Delete("/", m.handleDeletePod())
			r.With(requirePerm(ps, custommiddleware.PermPodsStart)).Post("/start", m.handleStartPod())
			r.With(requirePerm(ps, custommiddleware.PermPodsStop)).Post("/stop", m.handleStopPod())
			r.Post("/reset", m.handleResetPod())
			r.Get("/topology", m.handleGetTopology())
			r.Post("/vms/{vmName}/reset", m.handleResetVM())
			r.Post("/vms/{vmName}/start", m.handleStartVM())
			r.Post("/vms/{vmName}/stop", m.handleStopVM())
			r.Post("/vms/{vmName}/suspend", m.handleSuspendVM())
			r.Post("/vms/{vmName}/resume", m.handleResumeVM())
			// Console access (delegated to consoleaccess.Manager)
			if m.consoleMgr != nil {
				m.consoleMgr.SetupPodConsoleRoutes(r)
			}
			r.Get("/vms/{vmName}/snapshots", m.handleListSnapshots())
			r.Post("/vms/{vmName}/snapshots", m.handleCreateSnapshot())
			r.Delete("/vms/{vmName}/snapshots/{snapshotName}", m.handleDeleteSnapshot())
		})
	})
}

// requirePerm returns a middleware that checks for a specific permission.
// If ps is nil (RBAC not configured), it returns a no-op passthrough middleware.
func requirePerm(ps *custommiddleware.PermissionService, perm string) func(http.Handler) http.Handler {
	if ps == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return ps.RequirePermission(perm)
}
