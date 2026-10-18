package server

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/enterprise"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/sessions"
)

// CanvasManager handles Canvas LMS LTI integration including launch, deep linking,
// grade passback, and console rendering.
type CanvasManager struct {
	ltiService         *canvas.LTIService
	gradeService       *canvas.GradeService
	deepLinkingService *canvas.DeepLinkingService
	canvasSyncService  *canvas.SyncService
	// Uses concrete Orchestrator type for Proxmox-specific methods (StartVMDirect, etc.)
	orchestrator      *orchestrator.Orchestrator
	redisService      *redisclient.Service
	ltiStateCache     redisclient.LTIStateCacheClient // Optional override for testing
	ltiAssignmentRepo repositories.LTIAssignmentRepository
	userRepo          repositories.UserRepository
	labTemplateRepo   repositories.LabTemplateRepository
	sessionRepo       repositories.SessionRepository
	podRepo           repositories.PodRepository
	orgRepo           repositories.OrganizationRepository
	config            canvas.Config
	consoleSecret     string // HMAC secret for console token signing
	authService       *auth.Service
	cookieCfg         auth.CookieConfig
	logger            *slog.Logger
	responder         *httputil.Responder
	metrics           sessions.SessionLifecycleRecorder

	// In-memory LTI state store (fallback when Redis is unavailable)
	ltiStateStore    map[string]ltiState
	ltiStateStoreMu  sync.RWMutex
	ltiCleanupOnce   sync.Once
	ltiCleanupStopCh chan struct{}
}

// CanvasManagerConfig holds configuration for creating a CanvasManager
type CanvasManagerConfig struct {
	LTIService         *canvas.LTIService
	GradeService       *canvas.GradeService
	DeepLinkingService *canvas.DeepLinkingService
	CanvasSyncService  *canvas.SyncService
	Orchestrator       *orchestrator.Orchestrator
	RedisService       *redisclient.Service
	LTIStateCache      redisclient.LTIStateCacheClient // Optional override for testing
	LTIAssignmentRepo  repositories.LTIAssignmentRepository
	UserRepo           repositories.UserRepository
	LabTemplateRepo    repositories.LabTemplateRepository
	SessionRepo        repositories.SessionRepository
	PodRepo            repositories.PodRepository
	OrgRepo            repositories.OrganizationRepository
	Config             canvas.Config
	ConsoleSecret      string // HMAC secret for console token signing (defaults to JWT secret)
	AuthService        *auth.Service
	CookieConfig       auth.CookieConfig
	Logger             *slog.Logger
	Metrics            sessions.SessionLifecycleRecorder
}

// NewCanvasManager creates a new CanvasManager with the given configuration.
// Returns an error if a per-instance console secret cannot be generated
// (crypto/rand unavailable). Callers should treat that as a fatal startup error.
func NewCanvasManager(cfg CanvasManagerConfig) (*CanvasManager, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	consoleSecret := cfg.ConsoleSecret
	if consoleSecret == "" {
		s, err := generateRandomString(32)
		if err != nil {
			return nil, fmt.Errorf("canvas_manager: generating console secret: %w", err)
		}
		consoleSecret = s
	}

	return &CanvasManager{
		ltiService:         cfg.LTIService,
		gradeService:       cfg.GradeService,
		deepLinkingService: cfg.DeepLinkingService,
		canvasSyncService:  cfg.CanvasSyncService,
		orchestrator:       cfg.Orchestrator,
		redisService:       cfg.RedisService,
		ltiStateCache:      cfg.LTIStateCache,
		ltiAssignmentRepo:  cfg.LTIAssignmentRepo,
		userRepo:           cfg.UserRepo,
		labTemplateRepo:    cfg.LabTemplateRepo,
		sessionRepo:        cfg.SessionRepo,
		podRepo:            cfg.PodRepo,
		orgRepo:            cfg.OrgRepo,
		config:             cfg.Config,
		consoleSecret:      consoleSecret,
		authService:        cfg.AuthService,
		cookieCfg:          cfg.CookieConfig,
		logger:             logger,
		responder:          httputil.NewResponder(logger),
		metrics:            cfg.Metrics,
		ltiStateStore:      make(map[string]ltiState),
		ltiCleanupStopCh:   make(chan struct{}),
	}, nil
}

// MetricsRecorder exposes the configured recorder so the composition root can
// be tested; see sessions.Manager.MetricsRecorder. No production caller.
func (m *CanvasManager) MetricsRecorder() sessions.SessionLifecycleRecorder { return m.metrics }

// Stop signals the LTI cleanup goroutine to exit.
func (m *CanvasManager) Stop() {
	select {
	case <-m.ltiCleanupStopCh:
		// already stopped
	default:
		close(m.ltiCleanupStopCh)
	}
}

// getLTIStateCache returns the LTI state cache, preferring an injected
// override (set by tests via WithLTIStateCache) over the production one
// from redisService.
func (m *CanvasManager) getLTIStateCache() redisclient.LTIStateCacheClient {
	var fallback redisclient.LTIStateCacheClient
	if m.redisService != nil {
		fallback = m.redisService.LTIState
	}
	return preferOverride[redisclient.LTIStateCacheClient](m.ltiStateCache, fallback)
}

// SetupRoutes configures Canvas LTI routes on the given router
func (m *CanvasManager) SetupRoutes(r chi.Router) {
	// LTI 1.3 endpoints (outside API versioning)
	// All LTI routes require Enterprise Edition
	r.Route("/lti", func(r chi.Router) {
		r.Use(middleware.RequireEnterprise(enterprise.FeatureLTI))

		r.Get("/launch", m.handleLTILaunch())
		r.Post("/launch", m.handleLTILaunch())
		r.Get("/jwks", m.handleLTIJWKS())
		r.Post("/token", m.handleLTIToken())
		r.Post("/callback", m.handleLTICallback())
		r.Get("/select", m.handleLTISelectPage())
		r.Get("/console", m.handleLTIConsole())
	})
}

// SetupAPIRoutes configures Canvas LTI API routes (under /api/v1/lti)
func (m *CanvasManager) SetupAPIRoutes(r chi.Router) {
	r.Route("/lti", func(r chi.Router) {
		r.Use(middleware.RequireEnterprise(enterprise.FeatureLTI))

		r.Get("/templates", m.handleListLTITemplates())
		r.Post("/deep-link/submit", m.handleDeepLinkSubmit())
	})
}
