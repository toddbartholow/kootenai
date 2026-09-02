// Package sessions provides session HTTP handlers.
package sessions

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	natsclient "github.com/toddbartholow/kootenai/api/internal/nats"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/session"
	"github.com/toddbartholow/kootenai/api/internal/wazuh"
)

// SessionLifecycleRecorder is the subset of the metrics API needed to count
// session starts and ends. Implemented by *metrics.Metrics; nil disables
// recording.
//
// These are monotonic counters only. The sessions_active gauge is owned by
// the reconcile loop against the database — see metrics.SetActiveSessions —
// because sessions are also created by the LTI launch path and ended in bulk
// by the stale-session sweeps.
type SessionLifecycleRecorder interface {
	SessionCreated()
	SessionEnded()
	SessionsEnded(n int64)
}

// Manager manages lab session operations including progress tracking and questions
type Manager struct {
	// Service layer (preferred for new code)
	sessionService *session.Service

	// Repositories (kept for handlers not yet migrated to service)
	sessionRepo          repositories.SessionRepository
	checkpointRepo       repositories.CheckpointProgressRepository
	checkpointHintRepo   repositories.CheckpointHintRepository
	labTemplateRepo      repositories.LabTemplateRepository
	questionResponseRepo repositories.QuestionResponseRepository
	enrollmentRepo       repositories.EnrollmentRepository
	pathwayRepo          repositories.PathwayRepository
	gradeSyncRepo        repositories.GradeSyncRepository
	orchestrator         orchestrator.Client
	evaluator            *checkpoint.Evaluator
	activeCheckRunner    *checkpoint.ActiveCheckRunner
	achievementService   *achievements.Service
	natsClient           *natsclient.Client
	redisService         *redisclient.Service
	wazuhService         *wazuh.Service
	tamperingDetector    *checkpoint.TamperingDetector
	logger               *slog.Logger
	responder            *httputil.Responder
	permissionService    *custommiddleware.PermissionService
	ownershipService     *custommiddleware.OwnershipService
	metrics              SessionLifecycleRecorder
}

// Config configures Manager
type Config struct {
	// Service layer (preferred for new code)
	SessionService *session.Service

	// Repositories (kept for handlers not yet migrated to service)
	SessionRepo          repositories.SessionRepository
	CheckpointRepo       repositories.CheckpointProgressRepository
	CheckpointHintRepo   repositories.CheckpointHintRepository
	LabTemplateRepo      repositories.LabTemplateRepository
	QuestionResponseRepo repositories.QuestionResponseRepository
	EnrollmentRepo       repositories.EnrollmentRepository
	PathwayRepo          repositories.PathwayRepository
	GradeSyncRepo        repositories.GradeSyncRepository
	Orchestrator         orchestrator.Client
	Evaluator            *checkpoint.Evaluator
	ActiveCheckRunner    *checkpoint.ActiveCheckRunner
	AchievementService   *achievements.Service
	NATSClient           *natsclient.Client
	RedisService         *redisclient.Service
	WazuhService         *wazuh.Service
	TamperingDetector    *checkpoint.TamperingDetector
	Logger               *slog.Logger
	Responder            *httputil.Responder
	PermissionService    *custommiddleware.PermissionService
	OwnershipService     *custommiddleware.OwnershipService
	Metrics              SessionLifecycleRecorder
}

// NewManager creates a new Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		sessionService:       cfg.SessionService,
		sessionRepo:          cfg.SessionRepo,
		checkpointRepo:       cfg.CheckpointRepo,
		checkpointHintRepo:   cfg.CheckpointHintRepo,
		labTemplateRepo:      cfg.LabTemplateRepo,
		questionResponseRepo: cfg.QuestionResponseRepo,
		enrollmentRepo:       cfg.EnrollmentRepo,
		pathwayRepo:          cfg.PathwayRepo,
		gradeSyncRepo:        cfg.GradeSyncRepo,
		orchestrator:         cfg.Orchestrator,
		evaluator:            cfg.Evaluator,
		activeCheckRunner:    cfg.ActiveCheckRunner,
		achievementService:   cfg.AchievementService,
		natsClient:           cfg.NATSClient,
		redisService:         cfg.RedisService,
		wazuhService:         cfg.WazuhService,
		tamperingDetector:    cfg.TamperingDetector,
		logger:               cfg.Logger,
		responder:            cfg.Responder,
		permissionService:    cfg.PermissionService,
		ownershipService:     cfg.OwnershipService,
		metrics:              cfg.Metrics,
	}
}

// MetricsRecorder exposes the configured recorder so the composition root can
// be tested. server.New wires this, and a missing wire is otherwise invisible:
// this package's own tests inject a recorder directly. No production caller.
func (m *Manager) MetricsRecorder() SessionLifecycleRecorder { return m.metrics }

// SetupRoutes registers session-related routes on the router
func (m *Manager) SetupRoutes(r chi.Router) {
	ps := m.permissionService

	r.Route("/sessions", func(r chi.Router) {
		r.Get("/", m.handleListSessions())
		r.With(requirePerm(ps, custommiddleware.PermSessionsCreate)).Post("/", m.handleCreateSession())
		r.With(requirePerm(ps, custommiddleware.PermAdminSystem)).Post("/cleanup", m.handleCleanupStaleSessions())

		// Session-specific routes with ownership validation
		r.Route("/{sessionID}", func(r chi.Router) {
			if m.ownershipService != nil {
				r.Use(m.ownershipService.SessionOwnershipMiddleware())
			} else {
				m.logger.Warn("Session routes configured without ownership service — resource access control is disabled")
			}
			r.Get("/", m.handleGetSession())
			r.Delete("/", m.handleDeleteSession())
			r.Post("/end", m.handleEndSession())
			r.Post("/submit", m.handleSubmitSession())
			r.Get("/progress", m.handleGetProgress())
			r.Get("/checkpoints", m.handleGetCheckpoints())

			// Question-based assessment routes
			r.Get("/questions", m.handleGetSessionQuestions())
			r.Post("/questions/{questionID}/answer", m.handleSubmitQuestionAnswer())
			r.Post("/questions/{questionID}/hint", m.handleGetQuestionHint())

			// Checkpoint hint route
			r.Post("/checkpoints/{checkpointID}/hint", m.handleGetCheckpointHint())
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
