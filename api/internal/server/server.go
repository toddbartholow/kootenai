// Package server provides the HTTP server and API routes
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/auth"
	authldap "github.com/toddbartholow/kootenai/api/internal/auth/ldap"
	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/certificate"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/classroom"
	"github.com/toddbartholow/kootenai/api/internal/dashboard"
	"github.com/toddbartholow/kootenai/api/internal/database"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/enterprise"
	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
	"github.com/toddbartholow/kootenai/api/internal/metrics"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	natsclient "github.com/toddbartholow/kootenai/api/internal/nats"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/pathway"
	"github.com/toddbartholow/kootenai/api/internal/recommendation"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server/audit"
	"github.com/toddbartholow/kootenai/api/internal/server/consoleaccess"
	"github.com/toddbartholow/kootenai/api/internal/server/dashboards"
	"github.com/toddbartholow/kootenai/api/internal/server/features"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/labs"
	"github.com/toddbartholow/kootenai/api/internal/server/organizations"
	"github.com/toddbartholow/kootenai/api/internal/server/pods"
	"github.com/toddbartholow/kootenai/api/internal/server/reservations"
	"github.com/toddbartholow/kootenai/api/internal/server/sessions"
	"github.com/toddbartholow/kootenai/api/internal/server/users"
	"github.com/toddbartholow/kootenai/api/internal/session"
	"github.com/toddbartholow/kootenai/api/internal/simulation"
	"github.com/toddbartholow/kootenai/api/internal/version"
	"github.com/toddbartholow/kootenai/api/internal/wazuh"
	"github.com/toddbartholow/kootenai/api/internal/websocket"

	// Import docs for swagger
	_ "github.com/toddbartholow/kootenai/api/docs"
)

// serverDeps holds dependencies that are only needed during construction in New().
// They are set via ServerOption functions, consumed to build managers, then released.
type serverDeps struct {
	achievementRepo       repositories.AchievementRepository
	achievementService    *achievements.Service
	dashboardService      *dashboard.Service
	reservationRepo       repositories.ReservationRepository
	simulationService     SimulationService
	certificateService    CertificateService
	recommendationService *recommendation.Service
	classroomService      *classroom.Service
	classroomRunner       *classroom.ClassroomRunner
	webhookValidator      *wazuh.WebhookValidator
	eventDeduplicator     *wazuh.EventDeduplicator
	wazuhHealthMonitor    *wazuh.HealthMonitor
	tamperingDetector     *checkpoint.TamperingDetector
	wazuhService          *wazuh.Service
	eventRepo             repositories.EventRepository
	emailSender           EmailSender
	passwordResetRepo     repositories.PasswordResetTokenRepository

	// Wiring-only repositories (Phase B1): consumed by managers during New
	// and never referenced by Server methods at runtime. sessionRepo, podRepo
	// and rbacRepo stay on Server because background loops still need them.
	labTemplateRepo      repositories.LabTemplateRepository
	podRepo              repositories.PodRepository
	checkpointRepo       repositories.CheckpointProgressRepository
	checkpointHintRepo   repositories.CheckpointHintRepository
	questionResponseRepo repositories.QuestionResponseRepository
	gradeSyncRepo        repositories.GradeSyncRepository
	userRepo             repositories.UserRepository
	pathwayRepo          repositories.PathwayRepository
	enrollmentRepo       repositories.EnrollmentRepository
	orgRepo              repositories.OrganizationRepository
	orgMembershipRepo    repositories.OrganizationMembershipRepository
	teamRepo             repositories.TeamRepository
	teamMembershipRepo   repositories.TeamMembershipRepository
	featureRepo          repositories.FeatureRepository
	licenseRepo          repositories.LicenseRepository
	auditRepo            repositories.AuditLogRepository
	ltiAssignmentRepo    repositories.LTIAssignmentRepository

	// Wiring-only services (Phase B4): constructed or injected during New,
	// passed to managers, and released afterwards. authService, tenantService,
	// permissionService, cookieCfg, and gradeSyncHandler stay on Server because
	// middleware or background goroutines reference them at runtime.
	ltiService         *canvas.LTIService
	gradeService       *canvas.GradeService
	canvasSyncService  *canvas.SyncService
	deepLinkingService *canvas.DeepLinkingService
	ownershipService   *custommiddleware.OwnershipService
	asyncProvisioner   *orchestrator.AsyncProvisioner
	tokenBlacklist     redisclient.TokenBlacklistClient
	ldapClient         *authldap.Client
	auditService       *audit.Service
	featuresService    *features.Service
}

// Background cleanup-loop intervals and thresholds. Tuned for production
// workloads; tests that need different values should accept these via a
// Server config field rather than redefining them locally.
const (
	// roleExpirationCleanupInterval is the cadence of runRoleExpirationCleanup.
	roleExpirationCleanupInterval = 24 * time.Hour

	// sessionCleanupInterval is the cadence of runSessionCleanup.
	sessionCleanupInterval = 1 * time.Hour

	// metricsReconcileInterval is the cadence of runMetricsReconcile, which
	// resets the pods_active and sessions_active gauges from the database.
	metricsReconcileInterval = 60 * time.Second

	// metricsReconcileTimeout bounds a single reconcile pass.
	metricsReconcileTimeout = 10 * time.Second

	// sessionStaleThreshold is the age past which an active session is
	// considered abandoned and gets force-ended by the cleanup loop.
	sessionStaleThreshold = 24 * time.Hour

	// sessionRetentionPeriod is how long ended sessions are retained before
	// runSessionCleanup deletes them.
	sessionRetentionPeriod = 7 * 24 * time.Hour
)

// Server represents the HTTP server.
type Server struct {
	router             *chi.Mux
	orchestrator       orchestrator.Client
	evaluator          *checkpoint.Evaluator
	activeCheckRunner  *checkpoint.ActiveCheckRunner
	wsHub              *websocket.Hub
	authService        *auth.Service
	assessmentMgr      *AssessmentManager
	consoleMgr         *consoleaccess.Manager
	labMgr             *labs.Manager
	pathwayMgr         *PathwayManager
	userMgr            *users.Manager
	sessionMgr         *sessions.Manager
	podMgr             *pods.Manager
	orgMgr             *organizations.Manager
	canvasMgr          *CanvasManager
	authMgr            *AuthManager
	reservationMgr     *reservations.Manager
	dashboardMgr       *dashboards.Manager
	simulationMgr      *SimulationManager
	classroomMgr       *ClassroomManager
	securityMgr        *SecurityManager
	eventMonitoringMgr *EventMonitoringManager
	certificateMgr     *CertificateManager
	recommendationMgr  *RecommendationManager
	achievementMgr     *AchievementManager
	webSocketMgr       *WebSocketManager
	wazuhEventMgr      *WazuhEventManager
	gradeSyncHandler   *GradeSyncHandler
	lifecycleManager   *orchestrator.LifecycleManager
	db                 *database.DB
	natsClient         *natsclient.Client
	redisService       *redisclient.Service
	dashboardCache     redisclient.DashboardCacheClient // Optional override for testing
	ltiStateCache      redisclient.LTIStateCacheClient  // Optional override for testing
	natsHealthChecker  NATSHealthChecker                // Optional override for testing
	redisHealthChecker RedisHealthChecker               // Optional override for testing
	dbHealthChecker    DatabaseHealthChecker            // Optional override for testing
	rateLimiter        *redisclient.APIRateLimiter
	authRateLimiter    *redisclient.AuthRateLimiter
	metrics            *metrics.Metrics
	// sessionRepo stays on Server because runSessionCleanup uses it at runtime.
	sessionRepo repositories.SessionRepository
	// podRepo stays on Server because runMetricsReconcile uses it at runtime.
	podRepo repositories.PodRepository
	// Tenant service for middleware
	tenantService *custommiddleware.TenantService
	// rbacRepo stays on Server because runRoleExpirationCleanup uses it at
	// runtime, plus permissionService construction and a getter.
	rbacRepo repositories.RBACRepository
	// Permission service for RBAC middleware
	permissionService *custommiddleware.PermissionService
	// Cookie auth config (ADR-0002), used by setupMiddleware.
	cookieCfg auth.CookieConfig
	logger    *slog.Logger
	// responder is the server-level HTTP response helper for the small
	// set of endpoints that still live on *Server directly (health,
	// readiness, version, metrics auth, admin-middleware 403). All other
	// responses are written through a manager-owned *httputil.Responder.
	responder *httputil.Responder
	config    Config
	startTime time.Time // Server start time for uptime tracking
	// deps holds construction-only dependencies; nil after New() completes.
	deps *serverDeps
}

// ensureDeps lazily initializes the serverDeps struct.
func (s *Server) ensureDeps() *serverDeps {
	if s.deps == nil {
		s.deps = &serverDeps{}
	}
	return s.deps
}

// EmailSender interface for sending emails (allows dependency injection)
type EmailSender interface {
	SendPasswordResetEmail(ctx context.Context, email, userName, resetToken string) error
}

// NATSHealthChecker interface for NATS health checks (allows test injection)
type NATSHealthChecker interface {
	IsConnected() bool
}

// RedisHealthChecker interface for Redis health checks (allows test injection)
type RedisHealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// DatabaseHealthChecker interface for database health checks (allows test injection)
type DatabaseHealthChecker interface {
	PingContext(ctx context.Context) error
}

// CertificateService interface for certificate operations (allows test injection)
type CertificateService interface {
	IssueCertificate(ctx context.Context, enrollmentID string) (*certificate.Certificate, error)
	GetCertificate(ctx context.Context, enrollmentID string) (*certificate.Certificate, error)
	VerifyCertificate(ctx context.Context, verificationCode string) (*certificate.Certificate, error)
	ListUserCertificates(ctx context.Context, userID string) ([]*certificate.Certificate, error)
}

// SimulationService interface for simulation operations (allows test injection)
type SimulationService interface {
	ListTestStudents(ctx context.Context) ([]*models.User, error)
	CreateTestStudent(ctx context.Context, name, email string) (*models.User, error)
	SimulatePathwayProgression(ctx context.Context, userID, pathwayID string, config simulation.SimulationConfig) (*simulation.SimulationResult, error)
	GetStudentProgress(ctx context.Context, userID string) (map[string]any, error)
	ResetStudentProgress(ctx context.Context, userID string) error
}

// Config holds server configuration
type Config struct {
	Host            string          `yaml:"host"`
	Port            int             `yaml:"port"`
	ReadTimeout     time.Duration   `yaml:"read_timeout"`
	WriteTimeout    time.Duration   `yaml:"write_timeout"`
	ShutdownTimeout time.Duration   `yaml:"shutdown_timeout"`
	CORSOrigins     []string        `yaml:"cors_origins"`
	LTI             canvas.Config   `yaml:"lti"`
	RateLimit       RateLimitConfig `yaml:"rate_limit"`

	// DemoMode disables admin enforcement on protected routes. Mirrors
	// Auth.DemoMode from the application config and must only be true when
	// authentication is intentionally bypassed (e.g. AUTH_DEMO_MODE=true).
	DemoMode bool `yaml:"demo_mode"`

	// MetricsToken, when non-empty, requires `Authorization: Bearer <token>`
	// to access /metrics. Empty disables the check (development only).
	MetricsToken string `yaml:"metrics_token"`

	// Environment identifies the deployment environment ("production"/"prod"
	// triggers stricter CORS validation). Sourced from the ENV env var by the
	// binary; promoted to Config so server construction has no implicit env reads.
	Environment string `yaml:"environment"`

	// CORSAllowWildcard, when true, permits a "*" entry in CORSOrigins by
	// disabling credentials (browsers reject "*" + credentials anyway). For
	// development only; the constructor rejects this in production.
	CORSAllowWildcard bool `yaml:"cors_allow_wildcard"`

	// LTIAllowInsecureKeys disables the 2048-bit minimum on Canvas LTI JWKS
	// keys. Required to run against the upstream Canvas docker-compose dev
	// stack, which still ships 512-bit demo keys. Never enable in production.
	LTIAllowInsecureKeys bool `yaml:"lti_allow_insecure_keys"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool  `yaml:"enabled"`
	RequestsPerMinute int64 `yaml:"requests_per_minute"`
	PodCreatePerHour  int64 `yaml:"pod_create_per_hour"`
	BurstSize         int64 `yaml:"burst_size"`
}

// DefaultConfig returns a default server configuration
func DefaultConfig() Config {
	return Config{
		Host:            "0.0.0.0",
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		CORSOrigins:     []string{"http://localhost:3000"},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 100,
			PodCreatePerHour:  10,
			BurstSize:         20,
		},
	}
}

// Validate checks that the configuration is valid
func (c *Config) Validate() error {
	var errs []error

	if c.Host == "" {
		errs = append(errs, errors.New("host cannot be empty"))
	}

	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port must be between 1 and 65535, got %d", c.Port))
	}

	if c.ReadTimeout <= 0 {
		errs = append(errs, errors.New("read_timeout must be positive"))
	}

	if c.WriteTimeout <= 0 {
		errs = append(errs, errors.New("write_timeout must be positive"))
	}

	if c.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("shutdown_timeout must be positive"))
	}

	// Note: CORS wildcard validation is handled at runtime in setupMiddleware()
	// to allow for environment-specific configuration via CORS_ALLOW_WILDCARD

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %w", errors.Join(errs...))
	}

	return nil
}

// ServerOption is a functional option for configuring the Server
type ServerOption func(*Server)

// WithLabTemplateRepo sets the lab template repository
func WithLabTemplateRepo(repo repositories.LabTemplateRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().labTemplateRepo = repo
	}
}

// WithSessionRepo sets the session repository
func WithSessionRepo(repo repositories.SessionRepository) ServerOption {
	return func(s *Server) {
		s.sessionRepo = repo
	}
}

// WithCheckpointRepo sets the checkpoint progress repository
func WithCheckpointRepo(repo repositories.CheckpointProgressRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().checkpointRepo = repo
	}
}

// WithCheckpointHintRepo sets the checkpoint hint repository
func WithCheckpointHintRepo(repo repositories.CheckpointHintRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().checkpointHintRepo = repo
	}
}

// WithQuestionResponseRepo sets the question response repository
func WithQuestionResponseRepo(repo repositories.QuestionResponseRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().questionResponseRepo = repo
	}
}

// WithGradeSyncRepo sets the grade sync repository
func WithGradeSyncRepo(repo repositories.GradeSyncRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().gradeSyncRepo = repo
	}
}

// WithUserRepo sets the user repository
func WithUserRepo(repo repositories.UserRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().userRepo = repo
	}
}

// WithPasswordResetRepo sets the password reset token repository
func WithPasswordResetRepo(repo repositories.PasswordResetTokenRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().passwordResetRepo = repo
	}
}

// WithGradeSyncHandler sets the grade sync handler for Canvas grade passback
func WithGradeSyncHandler(handler *GradeSyncHandler) ServerOption {
	return func(s *Server) {
		s.gradeSyncHandler = handler
	}
}

// WithLifecycleManager sets the pod lifecycle manager for automatic cleanup
func WithLifecycleManager(lm *orchestrator.LifecycleManager) ServerOption {
	return func(s *Server) {
		s.lifecycleManager = lm
	}
}

// WithAuthService sets the authentication service
func WithAuthService(authSvc *auth.Service) ServerOption {
	return func(s *Server) {
		s.authService = authSvc
	}
}

// WithCookieConfig enables HttpOnly cookie auth (ADR-0002 Phase B)
func WithCookieConfig(cfg auth.CookieConfig) ServerOption {
	return func(s *Server) {
		s.cookieCfg = cfg
	}
}

// WithLDAPClient sets the LDAP client for FreeIPA/LDAP authentication
func WithLDAPClient(client *authldap.Client) ServerOption {
	return func(s *Server) {
		s.ensureDeps().ldapClient = client
	}
}

// WithRedisService sets the Redis service for caching, sessions, and rate limiting
func WithRedisService(redisSvc *redisclient.Service) ServerOption {
	return func(s *Server) {
		s.redisService = redisSvc
	}
}

// WithDashboardCache sets the dashboard cache client (for testing)
func WithDashboardCache(cache redisclient.DashboardCacheClient) ServerOption {
	return func(s *Server) {
		s.dashboardCache = cache
	}
}

// WithLTIStateCache sets the LTI state cache client (for testing)
func WithLTIStateCache(cache redisclient.LTIStateCacheClient) ServerOption {
	return func(s *Server) {
		s.ltiStateCache = cache
	}
}

// WithTokenBlacklist sets the token blacklist client for invalidating tokens on refresh
func WithTokenBlacklist(blacklist redisclient.TokenBlacklistClient) ServerOption {
	return func(s *Server) {
		s.ensureDeps().tokenBlacklist = blacklist
	}
}

// WithReservationRepo sets the reservation repository
func WithReservationRepo(repo repositories.ReservationRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().reservationRepo = repo
	}
}

func WithDatabase(db *database.DB) ServerOption {
	return func(s *Server) {
		s.db = db
	}
}

func WithNATSClient(natsClient *natsclient.Client) ServerOption {
	return func(s *Server) {
		s.natsClient = natsClient
	}
}

// WithActiveCheckRunner sets the active check runner for the server
func WithActiveCheckRunner(runner *checkpoint.ActiveCheckRunner) ServerOption {
	return func(s *Server) {
		s.activeCheckRunner = runner
	}
}

// WithOrganizationRepo sets the organization repository
func WithOrganizationRepo(repo repositories.OrganizationRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().orgRepo = repo
	}
}

// WithOrganizationMembershipRepo sets the organization membership repository
func WithOrganizationMembershipRepo(repo repositories.OrganizationMembershipRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().orgMembershipRepo = repo
	}
}

// WithTeamRepo sets the team repository
func WithTeamRepo(repo repositories.TeamRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().teamRepo = repo
	}
}

// WithTeamMembershipRepo sets the team membership repository
func WithTeamMembershipRepo(repo repositories.TeamMembershipRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().teamMembershipRepo = repo
	}
}

// WithFeatureRepo sets the feature repository
func WithFeatureRepo(repo repositories.FeatureRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().featureRepo = repo
	}
}

// WithTenantService sets the tenant service for multi-tenancy middleware
func WithTenantService(svc *custommiddleware.TenantService) ServerOption {
	return func(s *Server) {
		s.tenantService = svc
	}
}

// WithSimulationService sets the simulation service for test students
func WithSimulationService(svc *simulation.Service) ServerOption {
	return func(s *Server) {
		s.ensureDeps().simulationService = svc
	}
}

// WithCertificateService sets the certificate service for pathway completion certificates
func WithCertificateService(svc CertificateService) ServerOption {
	return func(s *Server) {
		s.ensureDeps().certificateService = svc
	}
}

// WithRecommendationService sets the recommendation service for personalized suggestions
func WithRecommendationService(svc *recommendation.Service) ServerOption {
	return func(s *Server) {
		s.ensureDeps().recommendationService = svc
	}
}

// WithEmailSender sets the email sender for password reset emails
func WithEmailSender(sender EmailSender) ServerOption {
	return func(s *Server) {
		s.ensureDeps().emailSender = sender
	}
}

// WithPodRepo sets the pod repository. Stored on both serverDeps (consumed by
// the pod manager during New) and Server (read by runMetricsReconcile).
func WithPodRepo(repo repositories.PodRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().podRepo = repo
		s.podRepo = repo
	}
}

// WithOwnershipService sets the ownership service for resource access control
func WithOwnershipService(svc *custommiddleware.OwnershipService) ServerOption {
	return func(s *Server) {
		s.ensureDeps().ownershipService = svc
	}
}

// WithAsyncProvisioner sets the async pod provisioner for non-blocking pod creation
func WithAsyncProvisioner(ap *orchestrator.AsyncProvisioner) ServerOption {
	return func(s *Server) {
		s.ensureDeps().asyncProvisioner = ap
	}
}

// WithOrchestrator sets the orchestrator client (enables dependency injection for testing)
func WithOrchestrator(orch orchestrator.Client) ServerOption {
	return func(s *Server) {
		s.orchestrator = orch
	}
}

// WithLicenseRepo sets the license repository
func WithLicenseRepo(repo repositories.LicenseRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().licenseRepo = repo
	}
}

// WithAuditRepo sets the audit log repository for security logging
func WithAuditRepo(repo repositories.AuditLogRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().auditRepo = repo
	}
}

// WithRBACRepo sets the RBAC repository for permission checking
func WithRBACRepo(repo repositories.RBACRepository) ServerOption {
	return func(s *Server) {
		s.rbacRepo = repo
	}
}

// WithPermissionService sets the permission service for RBAC middleware
func WithPermissionService(svc *custommiddleware.PermissionService) ServerOption {
	return func(s *Server) {
		s.permissionService = svc
	}
}

// WithClassroomService sets the classroom simulation service
func WithClassroomService(svc *classroom.Service) ServerOption {
	return func(s *Server) {
		s.ensureDeps().classroomService = svc
	}
}

// WithClassroomRunner sets the classroom simulation runner
func WithClassroomRunner(runner *classroom.ClassroomRunner) ServerOption {
	return func(s *Server) {
		s.ensureDeps().classroomRunner = runner
	}
}

// WithCanvasSyncService sets the Canvas sync service
func WithCanvasSyncService(svc *canvas.SyncService) ServerOption {
	return func(s *Server) {
		s.ensureDeps().canvasSyncService = svc
	}
}

// WithWebhookValidator sets the Wazuh webhook security validator
func WithWebhookValidator(v *wazuh.WebhookValidator) ServerOption {
	return func(s *Server) {
		s.ensureDeps().webhookValidator = v
	}
}

func WithWazuhHealthMonitor(hm *wazuh.HealthMonitor) ServerOption {
	return func(s *Server) {
		s.ensureDeps().wazuhHealthMonitor = hm
	}
}

// WithEventDeduplicator sets the event deduplicator for replay attack prevention
func WithEventDeduplicator(d *wazuh.EventDeduplicator) ServerOption {
	return func(s *Server) {
		s.ensureDeps().eventDeduplicator = d
	}
}

// WithTamperingDetector sets the tampering detector for session integrity monitoring
func WithTamperingDetector(td *checkpoint.TamperingDetector) ServerOption {
	return func(s *Server) {
		s.ensureDeps().tamperingDetector = td
	}
}

// WithWazuhService sets the Wazuh service for agent status queries
func WithWazuhService(ws *wazuh.Service) ServerOption {
	return func(s *Server) {
		s.ensureDeps().wazuhService = ws
	}
}

func WithLTIAssignmentRepo(repo repositories.LTIAssignmentRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().ltiAssignmentRepo = repo
	}
}

// WithNATSHealthChecker sets an override for NATS health checks (for testing)
func WithNATSHealthChecker(checker NATSHealthChecker) ServerOption {
	return func(s *Server) {
		s.natsHealthChecker = checker
	}
}

// WithRedisHealthChecker sets an override for Redis health checks (for testing)
func WithRedisHealthChecker(checker RedisHealthChecker) ServerOption {
	return func(s *Server) {
		s.redisHealthChecker = checker
	}
}

// WithDBHealthChecker sets an override for database health checks (for testing)
func WithDBHealthChecker(checker DatabaseHealthChecker) ServerOption {
	return func(s *Server) {
		s.dbHealthChecker = checker
	}
}

// WithMetrics supplies the shared metrics registry. main.go builds one
// instance and hands the same one to the orchestrator, the evaluator and this
// server, so every subsystem writes into the registry that /metrics serves.
// Without it the server keeps a private registry, which is what most tests
// want.
func WithMetrics(m *metrics.Metrics) ServerOption {
	return func(s *Server) {
		if m != nil {
			s.metrics = m
		}
	}
}

// New creates a new HTTP server
func New(
	cfg Config,
	orch *orchestrator.Orchestrator,
	eval *checkpoint.Evaluator,
	wsHub *websocket.Hub,
	logger *slog.Logger,
	opts ...ServerOption,
) (*Server, error) {
	s := &Server{
		router:       chi.NewRouter(),
		orchestrator: orch,
		evaluator:    eval,
		wsHub:        wsHub,
		logger:       logger,
		responder:    httputil.NewResponder(logger),
		config:       cfg,
		metrics:      metrics.New(),
		startTime:    time.Now(),
	}

	// Wire i18n catalog-miss telemetry through the Metrics instance. The miss
	// handler is package-global in i18n; the last Server constructed wins.
	// Multi-server test harnesses do not need isolated counters.
	appi18n.SetMissHandler(func(messageID, locale string) {
		s.metrics.I18nCatalogMiss()
	})

	// Apply functional options
	for _, opt := range opts {
		opt(s)
	}

	// Grab construction-only deps (may be nil if no With* options used those fields)
	d := s.ensureDeps()

	// Warn if no ownership service is configured — pod/session routes will
	// use a deny-all middleware as a safety net to prevent silent access control bypass
	if d.ownershipService == nil {
		logger.Warn("No ownership service configured — pod/session routes will deny access by default. " +
			"Call WithOwnershipService() to enable proper resource ownership validation.")
	}

	// Initialize LTI services if configured and enterprise feature is enabled
	if cfg.LTI.ClientID != "" {
		if !enterprise.Default.IsEnabled(enterprise.FeatureLTI) {
			logger.Info("LTI integration requires Enterprise Edition",
				"edition", enterprise.Default.Edition(),
				"feature", enterprise.FeatureLTI)
		} else {
			// LTIAllowInsecureKeys is dev-only; populated from env by the binary.
			ltiSvc, err := canvas.NewLTIServiceWithOptions(cfg.LTI, logger, cfg.LTIAllowInsecureKeys)
			if err != nil {
				logger.Warn("Failed to initialize LTI service", "error", err)
			} else {
				d.ltiService = ltiSvc
				d.gradeService = canvas.NewGradeService(cfg.LTI, logger)
				logger.Info("LTI integration enabled", "clientId", cfg.LTI.ClientID)

				// Initialize deep linking service
				deepLinkSvc, err := canvas.NewDeepLinkingService(cfg.LTI, logger)
				if err != nil {
					logger.Warn("Failed to initialize deep linking service", "error", err)
				} else {
					d.deepLinkingService = deepLinkSvc
					logger.Info("LTI deep linking enabled")
				}
			}
		}
	}

	// Initialize rate limiter if Redis is available and rate limiting is enabled
	if s.redisService != nil && cfg.RateLimit.Enabled {
		s.rateLimiter = s.redisService.APILimiter
		s.authRateLimiter = s.redisService.AuthLimiter
		logger.Info("Rate limiting enabled",
			"requests_per_minute", cfg.RateLimit.RequestsPerMinute,
			"pod_create_per_hour", cfg.RateLimit.PodCreatePerHour,
		)
	}

	// Initialize assessment manager
	s.assessmentMgr = NewAssessmentManager(AssessmentManagerConfig{
		SessionRepo:     s.sessionRepo,
		LabTemplateRepo: d.labTemplateRepo,
		Evaluator:       s.evaluator,
		Orchestrator:    orch,
		WsHub:           s.wsHub,
		Logger:          logger,
		Responder:       s.responder,
	})

	// Initialize console manager
	s.consoleMgr = consoleaccess.NewManager(consoleaccess.Config{
		Orchestrator:    orch,
		Logger:          logger,
		Responder:       s.responder,
		AdminMiddleware: s.requireAdmin,
	})

	// Initialize lab manager
	s.labMgr = labs.NewManager(labs.Config{
		LabTemplateRepo:   d.labTemplateRepo,
		Logger:            logger,
		Responder:         s.responder,
		PermissionService: s.permissionService,
	})

	// Initialize pathway service
	pathwaySvc := pathway.NewService(d.pathwayRepo, d.enrollmentRepo, logger)

	// Initialize pathway manager
	s.pathwayMgr = NewPathwayManager(PathwayManagerConfig{
		PathwayService:    pathwaySvc,
		PathwayRepo:       d.pathwayRepo,
		EnrollmentRepo:    d.enrollmentRepo,
		Logger:            logger,
		Responder:         s.responder,
		PermissionService: s.permissionService,
	})

	// Initialize dashboard service
	d.dashboardService = dashboard.NewService(logger).
		WithSessionRepo(s.sessionRepo).
		WithLabTemplateRepo(d.labTemplateRepo).
		WithPathwayRepo(d.pathwayRepo).
		WithEnrollmentRepo(d.enrollmentRepo).
		WithAchievementRepo(d.achievementRepo).
		WithAchievementService(d.achievementService).
		WithUserRepo(d.userRepo)

	// Initialize user manager
	s.userMgr = users.NewManager(users.Config{
		UserRepo:          d.userRepo,
		PasswordResetRepo: d.passwordResetRepo,
		EmailSender:       d.emailSender,
		Logger:            logger,
		Responder:         s.responder,
	})

	// Initialize session service (business logic layer)
	sessionSvc := session.NewService(s.sessionRepo, d.checkpointRepo, logger).
		WithLabTemplateRepo(d.labTemplateRepo).
		WithEnrollmentRepo(d.enrollmentRepo).
		WithPathwayRepo(d.pathwayRepo).
		WithGradeSyncRepo(d.gradeSyncRepo).
		WithEvaluator(s.evaluator).
		WithAchievementService(d.achievementService).
		WithNATSClient(s.natsClient).
		WithRedisService(s.redisService)

	// Initialize session manager (HTTP adapter layer)
	// Initialize domain services (extracted from monolithic handlers)
	d.auditService = audit.NewService(d.auditRepo, logger)
	d.featuresService = features.NewService(d.featureRepo, logger)

	// Initialize permission service for RBAC if repo is available
	// (must happen before managers that depend on it)
	if s.rbacRepo != nil && s.permissionService == nil {
		s.permissionService = custommiddleware.NewPermissionService(s.rbacRepo, logger)
		if d.auditService != nil {
			s.permissionService.WithAuditLogger(audit.NewPermissionAuditAdapter(d.auditService))
		}
		logger.Info("RBAC permission service initialized")
	}
	if s.permissionService == nil {
		logger.Warn("RBAC permission service not configured — all permission checks are disabled")
	}

	s.sessionMgr = sessions.NewManager(sessions.Config{
		SessionService:       sessionSvc,
		SessionRepo:          s.sessionRepo,
		CheckpointRepo:       d.checkpointRepo,
		CheckpointHintRepo:   d.checkpointHintRepo,
		LabTemplateRepo:      d.labTemplateRepo,
		QuestionResponseRepo: d.questionResponseRepo,
		EnrollmentRepo:       d.enrollmentRepo,
		PathwayRepo:          d.pathwayRepo,
		GradeSyncRepo:        d.gradeSyncRepo,
		Orchestrator:         orch,
		Evaluator:            s.evaluator,
		ActiveCheckRunner:    s.activeCheckRunner,
		AchievementService:   d.achievementService,
		NATSClient:           s.natsClient,
		RedisService:         s.redisService,
		WazuhService:         d.wazuhService,
		TamperingDetector:    d.tamperingDetector,
		Logger:               logger,
		Responder:            s.responder,
		PermissionService:    s.permissionService,
		OwnershipService:     d.ownershipService,
		Metrics:              s.metrics,
	})

	// Initialize pod manager
	s.podMgr = pods.NewManager(pods.Config{
		Orchestrator:      orch,
		AsyncProvisioner:  d.asyncProvisioner,
		LabTemplateRepo:   d.labTemplateRepo,
		UserRepo:          d.userRepo,
		ConsoleMgr:        s.consoleMgr,
		Logger:            logger,
		Responder:         s.responder,
		PermissionService: s.permissionService,
		OwnershipService:  d.ownershipService,
		RateLimiter:       pods.AsPodRateLimiter(s.rateLimiter),
		RateLimitConfig: pods.RateLimitConfig{
			Enabled:          s.config.RateLimit.Enabled,
			PodCreatePerHour: s.config.RateLimit.PodCreatePerHour,
		},
		Metrics: s.metrics,
	})

	// Initialize organization manager
	s.orgMgr = organizations.NewManager(organizations.Config{
		OrgRepo:            d.orgRepo,
		OrgMembershipRepo:  d.orgMembershipRepo,
		TeamRepo:           d.teamRepo,
		TeamMembershipRepo: d.teamMembershipRepo,
		FeatureRepo:        d.featureRepo,
		LicenseRepo:        d.licenseRepo,
		UserRepo:           d.userRepo,
		RBACRepo:           s.rbacRepo,
		LabMgr:             s.labMgr,
		TenantService:      s.tenantService,
		FeaturesService:    d.featuresService,
		AuditService:       d.auditService,
		Logger:             logger,
		Responder:          s.responder,
		PermissionService:  s.permissionService,
	})

	// Initialize canvas manager for LTI integration. A crypto/rand failure
	// at startup is fatal; propagate to the binary's main.
	canvasMgr, err := NewCanvasManager(CanvasManagerConfig{
		LTIService:         d.ltiService,
		GradeService:       d.gradeService,
		DeepLinkingService: d.deepLinkingService,
		CanvasSyncService:  d.canvasSyncService,
		Orchestrator:       orch,
		RedisService:       s.redisService,
		LTIAssignmentRepo:  d.ltiAssignmentRepo,
		UserRepo:           d.userRepo,
		LabTemplateRepo:    d.labTemplateRepo,
		SessionRepo:        s.sessionRepo,
		PodRepo:            d.podRepo,
		OrgRepo:            d.orgRepo,
		Config:             cfg.LTI,
		CookieConfig:       s.cookieCfg,
		Logger:             logger,
		Metrics:            s.metrics,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing canvas manager: %w", err)
	}
	s.canvasMgr = canvasMgr

	// Initialize Phase 3 managers (converted from inline Server handlers)
	// Build rate-limit middleware for public auth endpoints once at
	// construction time. Redis-backed when available; otherwise fall back
	// to an in-memory limiter. AuthManager itself doesn't care which.
	var loginRL, passwordResetRL MiddlewareFunc
	if s.authRateLimiter != nil {
		authCfg := redisclient.DefaultAuthRateLimitConfig()
		loginRL = s.authRateLimiter.LoginMiddleware(authCfg, nil)
		passwordResetRL = s.authRateLimiter.PasswordResetMiddleware(authCfg, nil)
	} else {
		logger.Warn("Using in-memory rate limiting fallback for auth endpoints (Redis unavailable)")
		fallbackLimiter := custommiddleware.DefaultAuthRateLimiter()
		loginRL = fallbackLimiter.Middleware
		passwordResetRL = fallbackLimiter.Middleware
	}

	s.authMgr = NewAuthManager(AuthManagerConfig{
		AuthService:            s.authService,
		UserRepo:               d.userRepo,
		OrgMembershipRepo:      d.orgMembershipRepo,
		TokenBlacklist:         d.tokenBlacklist,
		CookieConfig:           s.cookieCfg,
		LDAPClient:             d.ldapClient,
		Logger:                 logger,
		Responder:              s.responder,
		UserMgr:                s.userMgr,
		LoginRateLimit:         loginRL,
		PasswordResetRateLimit: passwordResetRL,
	})

	s.reservationMgr = reservations.NewManager(reservations.Config{
		ReservationRepo: d.reservationRepo,
		Logger:          logger,
	})

	s.dashboardMgr = dashboards.NewManager(dashboards.Config{
		DashboardService: d.dashboardService,
		DashboardCache:   s.getDashboardCache(),
		SessionRepo:      s.sessionRepo,
		LabTemplateRepo:  d.labTemplateRepo,
		Logger:           logger,
		Responder:        s.responder,
	})

	s.certificateMgr = NewCertificateManager(CertificateManagerConfig{
		CertificateService: d.certificateService,
		EnrollmentRepo:     d.enrollmentRepo,
		Logger:             logger,
	})

	s.recommendationMgr = NewRecommendationManager(RecommendationManagerConfig{
		RecommendationService: d.recommendationService,
		Logger:                logger,
	})

	s.securityMgr = NewSecurityManager(SecurityManagerConfig{
		WazuhHealthMonitor: d.wazuhHealthMonitor,
		WebhookValidator:   d.webhookValidator,
		Logger:             logger,
	})

	s.eventMonitoringMgr = NewEventMonitoringManager(EventMonitoringManagerConfig{
		EventRepo: d.eventRepo,
		Logger:    logger,
		Responder: s.responder,
	})

	s.simulationMgr = NewSimulationManager(SimulationManagerConfig{
		SimulationService: d.simulationService,
		Logger:            logger,
		ClassroomRouteHook: func(r chi.Router) {
			if s.classroomMgr != nil {
				s.classroomMgr.SetupRoutes(r)
			}
		},
	})

	s.classroomMgr = NewClassroomManager(ClassroomManagerConfig{
		ClassroomService: d.classroomService,
		ClassroomRunner:  d.classroomRunner,
		Logger:           logger,
		Responder:        s.responder,
	})

	s.achievementMgr = NewAchievementManager(AchievementManagerConfig{
		AchievementRepo:    d.achievementRepo,
		AchievementService: d.achievementService,
		Logger:             logger,
	})

	s.webSocketMgr = NewWebSocketManager(WebSocketManagerConfig{
		AuthService:  s.authService,
		WsHub:        s.wsHub,
		Orchestrator: s.orchestrator,
		Evaluator:    s.evaluator,
		SessionRepo:  s.sessionRepo,
		UserRepo:     d.userRepo,
		Logger:       logger,
	})

	s.wazuhEventMgr = NewWazuhEventManager(WazuhEventManagerConfig{
		WebhookValidator:   d.webhookValidator,
		EventDeduplicator:  d.eventDeduplicator,
		WazuhHealthMonitor: d.wazuhHealthMonitor,
		TamperingDetector:  d.tamperingDetector,
		SessionRepo:        s.sessionRepo,
		ActiveCheckRunner:  s.activeCheckRunner,
		Evaluator:          s.evaluator,
		LabTemplateRepo:    d.labTemplateRepo,
		EventRepo:          d.eventRepo,
		WsHub:              s.wsHub,
		Logger:             logger,
	})

	// Release construction-only deps; managers now own their references.
	s.deps = nil

	if err := s.setupMiddleware(); err != nil {
		return nil, fmt.Errorf("setting up middleware: %w", err)
	}
	s.setupRoutes()

	return s, nil
}

// StartBackgroundServices starts all background services (assessment manager, grade sync, lifecycle, etc.)
func (s *Server) StartBackgroundServices(ctx context.Context) {
	if s.assessmentMgr != nil {
		s.assessmentMgr.Start(ctx)
		s.logger.Info("Assessment manager background services started")
	}

	if s.gradeSyncHandler != nil {
		if err := s.gradeSyncHandler.Start(ctx); err != nil {
			s.logger.Error("Failed to start grade sync handler", "error", err)
		} else {
			s.logger.Info("Grade sync handler background services started")
			// Process any pending grades from before restart
			go func() {
				if err := s.gradeSyncHandler.ProcessPendingGrades(ctx); err != nil {
					s.logger.Error("Failed to process pending grades", "error", err)
				}
			}()
		}
	}

	if s.lifecycleManager != nil {
		s.lifecycleManager.Start(ctx)
		s.logger.Info("Pod lifecycle manager background services started")
	}

	// Start role expiration cleanup (runs every 24 hours)
	if s.rbacRepo != nil {
		go s.runRoleExpirationCleanup(ctx)
	}

	// Start session cleanup (ends stale, deletes old ended)
	if s.sessionRepo != nil {
		go s.runSessionCleanup(ctx)
	}

	// Start the metrics reconcile (owns the two active-* gauges). Both
	// repositories are required: a pass with one missing would leave that
	// gauge at zero forever while stamping the other as fresh. main.go wires
	// them together or not at all, so this is all-or-nothing in practice.
	if s.podRepo != nil && s.sessionRepo != nil {
		go s.runMetricsReconcile(ctx)
	} else if s.podRepo != nil || s.sessionRepo != nil {
		s.logger.Warn("Metrics reconcile not started: both pod and session repositories are required",
			"podRepo", s.podRepo != nil, "sessionRepo", s.sessionRepo != nil)
	}
}

// StopBackgroundServices stops all background services
func (s *Server) StopBackgroundServices() {
	if s.lifecycleManager != nil {
		s.lifecycleManager.Stop()
		s.logger.Info("Pod lifecycle manager background services stopped")
	}

	if s.assessmentMgr != nil {
		s.assessmentMgr.Stop()
		s.logger.Info("Assessment manager background services stopped")
	}

	if s.gradeSyncHandler != nil {
		s.gradeSyncHandler.Stop()
		s.logger.Info("Grade sync handler background services stopped")
	}

	// CanvasManager starts a periodic LTI state cleanup goroutine on first
	// use (see lti_handlers.go storeLTIState). Stop() closes its signal
	// channel so that goroutine exits on shutdown rather than leaking.
	if s.canvasMgr != nil {
		s.canvasMgr.Stop()
		s.logger.Info("Canvas manager background services stopped")
	}
}

// runRoleExpirationCleanup periodically removes expired role assignments.
func (s *Server) runRoleExpirationCleanup(ctx context.Context) {
	ticker := time.NewTicker(roleExpirationCleanupInterval)
	defer ticker.Stop()
	s.logger.Info("Role expiration cleanup started", "interval", roleExpirationCleanupInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get all users with expired roles and clean them up
			// Use a simple approach: list and revoke expired
			s.logger.Info("Running role expiration cleanup")
			if s.rbacRepo != nil {
				rows, err := s.rbacRepo.CleanupExpiredRoles(ctx)
				if err != nil {
					s.logger.Error("Role expiration cleanup failed", "error", err)
				} else if rows > 0 {
					s.logger.Info("Role expiration cleanup completed", "expired_roles", rows)
				}
			}
		}
	}
}

// runSessionCleanup periodically ends stale sessions and deletes old ended
// sessions. Thresholds come from package-level constants — tune via consts,
// not by editing this function.
func (s *Server) runSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()
	s.logger.Info("Session cleanup started", "interval", sessionCleanupInterval)

	// Run immediately on startup, then on ticker
	for first := true; ; first = false {
		if !first {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}

		// 1. End stale active sessions
		ended, err := s.sessionRepo.EndStaleSessions(ctx, sessionStaleThreshold)
		if err != nil {
			s.logger.Error("Failed to end stale sessions", "error", err)
		} else if ended > 0 {
			s.metrics.SessionsEnded(ended)
			s.logger.Info("Ended stale sessions", "count", ended)
		}

		// 2. Delete old ended sessions
		cutoff := time.Now().Add(-sessionRetentionPeriod)
		deleted, err := s.sessionRepo.DeleteEndedBefore(ctx, cutoff)
		if err != nil {
			s.logger.Error("Failed to delete old sessions", "error", err)
		} else if deleted > 0 {
			s.logger.Info("Deleted old ended sessions", "count", deleted)
		}
	}
}

// runMetricsReconcile periodically resets the pods_active and sessions_active
// gauges from the database.
//
// These two gauges are NOT maintained by the lifecycle counters. Incrementing
// and decrementing them at the call sites cannot be made correct here: pods
// are created down three paths and destroyed down two, a failed provision
// leaves a row that is later destroyed (decrementing something that was never
// incremented), a partially-failed destroy returns before decrementing, and
// sessions are created by the LTI launch and ended in bulk by the stale
// sweeps. On top of that, any in-process gauge resets to zero on restart while
// the rows survive in Postgres, so a delta-maintained gauge goes negative on
// the first sweep after a deploy and silently disables the alerts that read it.
//
// Reading the count from the database is authoritative, self-healing, and
// survives restarts. The monotonic counters (pods_created_total and friends)
// remain at their call sites, where they are safe.
func (s *Server) runMetricsReconcile(ctx context.Context) {
	ticker := time.NewTicker(metricsReconcileInterval)
	defer ticker.Stop()
	s.logger.Info("Metrics reconcile started", "interval", metricsReconcileInterval)

	// Run immediately on startup so a restart re-seeds the gauges rather than
	// serving zeroes until the first tick.
	for first := true; ; first = false {
		if !first {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
		s.reconcileMetricsOnce(ctx)
	}
}

// reconcileMetricsOnce performs a single reconcile pass. Split out so tests
// can drive it without a ticker.
func (s *Server) reconcileMetricsOnce(ctx context.Context) {
	// Bound each pass: these run on the server-lifetime context, so a hung
	// query would otherwise park this goroutine indefinitely.
	ctx, cancel := context.WithTimeout(ctx, metricsReconcileTimeout)
	defer cancel()

	// Each gauge is updated independently: a failure reading pods says nothing
	// about the health of the sessions query, and freezing both on one error
	// would make a gauge stale for no reason. Freshness is the thing gated on
	// every gauge succeeding, not the updates themselves.
	// A nil repository counts as not-ok for the same reason a failed query
	// does: that gauge cannot be refreshed, so freshness must not be stamped
	// over it. StartBackgroundServices will not launch the loop in that state,
	// but this is called directly too.
	ok := true

	if s.podRepo == nil {
		ok = false
	} else if n, err := s.podRepo.CountActive(ctx); err != nil {
		s.logger.Error("Failed to reconcile active pods gauge", "error", err)
		ok = false
	} else {
		s.metrics.SetActivePods(n)
	}

	if s.sessionRepo == nil {
		ok = false
	} else if n, err := s.sessionRepo.CountActive(ctx); err != nil {
		s.logger.Error("Failed to reconcile active sessions gauge", "error", err)
		ok = false
	} else {
		s.metrics.SetActiveSessions(n)
	}

	// Withheld unless every gauge was set from the database. A gauge that could
	// not be refreshed keeps its previous value rather than dropping to zero —
	// zero reads as "no load" — and the missing stamp is what makes that
	// staleness visible via MetricsReconcileStale.
	if ok {
		s.metrics.ReconcileSucceeded(time.Now())
	}
}

// setupMiddleware configures middleware. Returns an error if CORS is
// misconfigured (wildcard origin + credentials in production, or no explicit
// override in non-prod). All other middleware wiring is infallible.
func (s *Server) setupMiddleware() error {
	// Security headers (should be early in chain)
	s.router.Use(custommiddleware.SecureHeaders)

	// Metrics middleware (early to capture full request duration)
	s.router.Use(s.metrics.Middleware)

	// Request ID
	s.router.Use(middleware.RequestID)

	// Deliberately NOT chi's middleware.RealIP. It rewrites r.RemoteAddr from
	// X-Forwarded-For / True-Client-IP / X-Real-IP whether or not the
	// infrastructure sets them, so any client can choose its own address
	// (GHSA-3fxj-6jh8-hvhx, GHSA-rjr7-jggh-pgcp, GHSA-9g5q-2w5x-hmxf; chi
	// deprecated it in v5.3.0).
	//
	// That directly defeated this codebase's own policy: middleware.GetClientIP
	// and redis.GetRealIP both refuse X-Forwarded-For as spoofable and fall
	// back to r.RemoteAddr — but RealIP ran first, so the value they fell back
	// to had already been replaced by the header they were avoiding. Use those
	// helpers to resolve a client IP; r.RemoteAddr now stays the real peer.

	// Logging
	s.router.Use(middleware.Logger)

	// Panic recovery
	s.router.Use(middleware.Recoverer)

	// Request timeout
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Request body size limit (1MB default)
	s.router.Use(custommiddleware.MaxBodySize(custommiddleware.DefaultMaxBodySize))

	// Locale negotiation: parse Accept-Language and attach a per-request
	// Localizer to the context. Placed before handlers so every user-facing
	// response can be localized. Operator logs remain English.
	s.router.Use(custommiddleware.Locale(appi18n.MustNewBundle()))

	// CORS configuration with security validation
	corsOpts := cors.Options{
		AllowedOrigins:   s.config.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-CSRF-Token", "X-Organization"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	// Security check: AllowCredentials=true with wildcard origins is dangerous
	// Browsers will reject this combination, and it indicates a misconfiguration
	hasWildcard := false
	for _, origin := range s.config.CORSOrigins {
		if origin == "*" {
			hasWildcard = true
			break
		}
	}
	if hasWildcard && corsOpts.AllowCredentials {
		if s.config.CORSAllowWildcard {
			// SECURITY: Block wildcard CORS in production (same pattern as demo mode)
			env := strings.ToLower(s.config.Environment)
			if env == "production" || env == "prod" {
				return fmt.Errorf("CORS_ALLOW_WILDCARD cannot be enabled in production: " +
					"set Environment to something other than 'production' or configure specific CORSOrigins")
			}
			s.logger.Warn("CORS: Wildcard origin (*) configured with explicit override - disabling credentials",
				slog.String("config_field", "CORSAllowWildcard=true"),
				slog.String("note", "Configure specific origins for production use"),
			)
			corsOpts.AllowCredentials = false
		} else {
			return fmt.Errorf("CORS misconfiguration: wildcard origin (*) cannot be used with credentials. " +
				"Fix: configure specific allowed origins in CORSOrigins instead of '*'. " +
				"For development only, set CORSAllowWildcard=true to disable credentials and allow wildcard. " +
				"See: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#credentialed_requests_and_wildcards")
		}
	}

	s.router.Use(cors.Handler(corsOpts))

	// Global rate limiting
	if s.rateLimiter != nil && s.config.RateLimit.Enabled {
		s.router.Use(s.rateLimiter.PerIPMiddleware(
			s.config.RateLimit.RequestsPerMinute,
			time.Minute,
		))
	} else if s.config.RateLimit.Enabled {
		// Fallback to in-memory rate limiter when Redis is unavailable
		memLimiter := custommiddleware.NewInMemoryRateLimiter(
			int(s.config.RateLimit.RequestsPerMinute), time.Minute,
		)
		s.router.Use(memLimiter.Middleware)
		s.logger.Info("Global rate limiting enabled (in-memory fallback, no Redis)",
			"requests_per_minute", s.config.RateLimit.RequestsPerMinute,
		)
	}

	return nil
}

// metricsAuth gates the /metrics endpoint with a bearer-token check when
// Config.MetricsToken is set. With no token configured the middleware is a
// no-op so local development and unauthenticated scrapers keep working.
func (s *Server) metricsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.config.MetricsToken
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) ||
			subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), []byte(token)) != 1 {
			s.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "admin.errors.bearerTokenRequired", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAdmin is middleware that enforces admin role on a route group.
// Bypassed only in demo mode (Config.DemoMode true) or when no auth service is
// wired (legacy safety belt — both conditions imply unauthenticated operation).
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.DemoMode || s.authService == nil {
			next.ServeHTTP(w, r)
			return
		}
		if !isAdminRequest(r) {
			s.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "admin.errors.adminAccessRequired", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	r := s.router

	// Health check and version (public endpoints)
	r.Get("/health", s.handleHealth)
	r.Get("/health/live", s.handleLive)
	r.Get("/health/ready", s.handleReady)
	r.Get("/ready", s.handleReady) // Alias for backwards compatibility
	r.Get("/version", s.handleVersion)

	// Metrics endpoint (Prometheus-compatible).
	// Requires `Authorization: Bearer <METRICS_TOKEN>` when token is configured;
	// passthrough otherwise (development).
	r.With(s.metricsAuth).Get("/metrics", s.metrics.Handler())

	// Swagger UI (public)
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Wazuh webhook (public, unauthenticated — HMAC is verified in the handler).
		if s.wazuhEventMgr != nil {
			s.wazuhEventMgr.SetupRoutes(r)
		}

		// Public auth + password-recovery routes.
		// AuthManager owns rate limiting and password-reset delegation; the
		// composition root only decides which middleware factory to inject,
		// which happened back in New().
		r.Group(func(r chi.Router) {
			s.authMgr.SetupPublicRoutes(r)
		})

		// Protected routes group
		r.Group(func(r chi.Router) {
			// Apply auth middleware (with demo fallback) for protected routes
			if s.authService != nil {
				if s.cookieCfg.Enabled {
					r.Use(s.authService.CookieOrBearerMiddleware(s.cookieCfg))
					r.Use(auth.CSRFMiddleware(s.cookieCfg))
				} else {
					r.Use(s.authService.Middleware)
				}
			}

			// Resolve tenant (organization) context from X-Organization header,
			// URL path, subdomain, or user's default org. RequireOrg is false
			// so requests without org context proceed normally (backwards compat).
			if s.tenantService != nil {
				r.Use(s.tenantService.Middleware)
			}

			// Load RBAC permissions into context for all protected routes
			if s.permissionService != nil {
				r.Use(s.permissionService.LoadPermissions)
			}

			// Current user (requires auth)
			r.Get("/auth/me", s.authMgr.handleGetCurrentUser)
			r.Put("/auth/me/preferred-locale", s.authMgr.handleUpdatePreferredLocale)

			// Labs/Templates (delegated to labs.Manager)
			s.labMgr.SetupRoutes(r)

			// Pods (delegated to pods.Manager)
			if s.podMgr != nil {
				s.podMgr.SetupRoutes(r)
			}

			// Sessions (delegated to SessionManager)
			if s.sessionMgr != nil {
				s.sessionMgr.SetupRoutes(r)
			}

			// Reservations (delegated to ReservationManager)
			if s.reservationMgr != nil {
				s.reservationMgr.SetupRoutes(r)
			}

			// Achievements — both /achievements and /users/{userID}/achievements
			// live on AchievementManager.
			if s.achievementMgr != nil {
				s.achievementMgr.SetupRoutes(r)
			}

			// Dashboard and analytics (delegated to DashboardManager)
			if s.dashboardMgr != nil {
				s.dashboardMgr.SetupRoutes(r)
			}

			// Direct Proxmox VM access (bypasses pod lookup, useful for dev/testing)
			// Delegated to consoleaccess.Manager
			s.consoleMgr.SetupProxmoxRoutes(r)

			// Assessment routes (Packet Tracer-style grading)
			if s.assessmentMgr != nil {
				s.assessmentMgr.SetupRoutes(r)
			}

			// Pathway routes (learning tracks) - delegated to PathwayManager.
			// Recommendation routes registered separately so they own their
			// own SetupRoutes rather than being injected as external handlers.
			s.pathwayMgr.SetupRoutes(r, PathwayExternalHandlers{
				IssueCertificate:     s.certificateMgr.handleIssueCertificate,
				GetCertificate:       s.certificateMgr.handleGetCertificate,
				ListUserCertificates: s.certificateMgr.handleListUserCertificates,
				VerifyCertificate:    s.certificateMgr.handleVerifyCertificate,
			})
			s.recommendationMgr.SetupRoutes(r)

			// Organization, team, feature, and license routes - delegated to organizations.Manager
			s.orgMgr.SetupRoutes(r)

			// User management routes (admin checks internally) - delegated to UserManager
			s.userMgr.SetupRoutes(r)

			// Admin-only routes
			r.Group(func(r chi.Router) {
				if s.permissionService != nil {
					r.Use(s.permissionService.RequirePermission(custommiddleware.PermAdminSystem))
				} else {
					r.Use(s.requireAdmin)
				}

				// Simulation routes (admin/development only)
				if s.simulationMgr != nil {
					s.simulationMgr.SetupRoutes(r)
				}

				// Event monitoring routes (admin dashboard)
				if s.eventMonitoringMgr != nil {
					s.eventMonitoringMgr.SetupRoutes(r)
				}

				// Security and monitoring routes (Wazuh agent health, tampering)
				if s.securityMgr != nil {
					s.securityMgr.SetupRoutes(r)
				}
			})
			// WebSocket endpoints (auth is performed inside the handler via
			// Sec-WebSocket-Protocol because browsers can't set Authorization
			// on upgrade requests).
			if s.webSocketMgr != nil {
				s.webSocketMgr.SetupRoutes(r)
			}
		}) // End of protected routes group

		// LTI deep linking API endpoints (no auth - used by Canvas iframe)
		s.canvasMgr.SetupAPIRoutes(r)
	})

	// Static files (noVNC, etc.)
	r.Mount("/static/", http.StripPrefix("/static/", s.StaticHandler()))

	// LTI 1.3 endpoints (delegated to CanvasManager)
	s.canvasMgr.SetupRoutes(r)
}

// Run starts the HTTP server
func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       s.config.ReadTimeout,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      s.config.WriteTimeout,
	}

	// Channel to receive server errors
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("Starting HTTP server", "addr", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Router returns the chi router for testing
func (s *Server) Router() *chi.Mux {
	return s.router
}

// PermissionService returns the RBAC permission service for conditional checks in handlers
func (s *Server) PermissionService() *custommiddleware.PermissionService {
	return s.permissionService
}

// RBACRepo returns the RBAC repository for use by managers
func (s *Server) RBACRepo() repositories.RBACRepository {
	return s.rbacRepo
}

// requirePerm returns a middleware that checks for a specific permission.
// If ps is nil (RBAC not configured), it returns a no-op passthrough middleware.
// NOTE: When RBAC is disabled, all permission checks are bypassed. This is logged
// at startup (see server.go New()) so operators are aware.
func requirePerm(ps *custommiddleware.PermissionService, perm string) func(http.Handler) http.Handler {
	if ps == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return ps.RequirePermission(perm)
}

// preferOverride returns override when it's non-nil, otherwise fallback.
// Used to let tests inject mock implementations of optional dependencies
// without burying the override-vs-production pick in business code. The nil
// check matches the typed-interface behaviour the previous hand-rolled
// getters had — only untyped nil counts as "no override."
func preferOverride[T any](override, fallback T) T {
	if any(override) == nil {
		return fallback
	}
	return override
}

// getDashboardCache returns the dashboard cache, preferring an injected
// override (set by tests via WithDashboardCache) over the production one
// from redisService.
func (s *Server) getDashboardCache() redisclient.DashboardCacheClient {
	var fallback redisclient.DashboardCacheClient
	if s.redisService != nil {
		fallback = s.redisService.Dashboard
	}
	return preferOverride[redisclient.DashboardCacheClient](s.dashboardCache, fallback)
}

// -----------------------------------------------------------------------------
// Health Handlers
// -----------------------------------------------------------------------------

// handleHealth godoc
// @Summary Basic health check
// @Description Simple liveness check - returns OK if server is running.
// @Description SERVED AT THE HOST ROOT, not under the /api/v1 basePath shown above -- Swagger 2.0 cannot express a per-path basePath, so the path below reads as /api/v1/... but the real URL omits that prefix.
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string "Server is healthy"
// @Router /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Basic liveness check - server is running
	s.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleLive godoc
// @Summary Kubernetes liveness probe
// @Description Simple liveness probe for Kubernetes - confirms server is running.
// @Description SERVED AT THE HOST ROOT, not under the /api/v1 basePath shown above -- Swagger 2.0 cannot express a per-path basePath, so the path below reads as /api/v1/... but the real URL omits that prefix.
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string "Server is alive"
// @Router /health/live [get]
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	// Simple liveness probe for Kubernetes - just confirms the server is running
	s.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady godoc
// @Summary Readiness check with dependency status
// @Description Full readiness check that verifies all dependencies (database, NATS, Redis, Proxmox) and returns their status with latency measurements.
// @Description SERVED AT THE HOST ROOT, not under the /api/v1 basePath shown above -- Swagger 2.0 cannot express a per-path basePath, so the path below reads as /api/v1/... but the real URL omits that prefix.
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "All dependencies healthy"
// @Failure 503 {object} HealthResponse "One or more dependencies unhealthy (degraded state)"
// @Router /health/ready [get]
// @Router /ready [get]
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Bound the entire readiness probe at 5 seconds. Check functions that
	// take a context honour the cancel; ones that don't (NATS/Proxmox ping)
	// are fast synchronous calls, so the bound is effectively the network
	// timeout of the slowest underlying call.
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type checkResult struct {
		name  string
		check *HealthCheck
	}

	// Buffered to the number of possible checks so goroutines never block
	// on send even if the collector exits on timeout.
	resultCh := make(chan checkResult, 4)
	var wg sync.WaitGroup

	addCheck := func(name string, fn func() *HealthCheck) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resultCh <- checkResult{name, fn()}
		}()
	}

	if s.db != nil || s.dbHealthChecker != nil {
		addCheck("database", func() *HealthCheck { return s.checkDatabaseWithLatency(checkCtx) })
	} else {
		// No database at all. main.go's initDatabase logs a warning and returns
		// (nil, nil) on any connect failure, so the server starts regardless --
		// a typo'd DATABASE_PASSWORD produces exactly this state. Skipping the
		// check here left allHealthy true, so /health/ready answered 200 with no
		// "database" entry at all while every request that touched storage
		// failed. Readiness means "can serve traffic"; without persistence this
		// instance cannot, so report it unready and name the reason.
		addCheck("database", func() *HealthCheck {
			return &HealthCheck{
				Status: "unhealthy",
				Error:  "no database connection configured; the server started without persistence",
			}
		})
	}
	if s.natsClient != nil || s.natsHealthChecker != nil {
		addCheck("nats", func() *HealthCheck { return s.checkNATSWithLatency() })
	}
	if s.redisService != nil || s.redisHealthChecker != nil {
		addCheck("redis", func() *HealthCheck { return s.checkRedisWithLatency(checkCtx) })
	}
	if s.orchestrator != nil {
		addCheck("proxmox", func() *HealthCheck { return s.checkProxmoxWithLatency() })
	}

	// Close resultCh once every goroutine has reported — the collector
	// uses the closed-channel signal as "done" rather than counting items.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	checks := make(map[string]*HealthCheck)
	allHealthy := true

collect:
	for {
		select {
		case result, ok := <-resultCh:
			if !ok {
				break collect // every check reported
			}
			checks[result.name] = result.check
			// Proxmox and CloudStack are optional — their failure doesn't
			// flip the overall readiness state.
			if result.name != "proxmox" && result.name != "cloudstack" && result.check.Status != "healthy" {
				allHealthy = false
			}
		case <-checkCtx.Done():
			allHealthy = false
			break collect
		}
	}

	response := HealthResponse{
		Status:  "healthy",
		Version: version.Get().Version,
		Uptime:  time.Since(s.startTime).Round(time.Second).String(),
		Checks:  checks,
	}
	if allHealthy {
		s.responder.JSONResponse(w, http.StatusOK, response)
	} else {
		response.Status = "degraded"
		s.responder.JSONResponse(w, http.StatusServiceUnavailable, response)
	}
}

// checkDatabaseWithLatency verifies database connectivity and measures latency
func (s *Server) checkDatabaseWithLatency(ctx context.Context) *HealthCheck {
	result := &HealthCheck{
		Status: "healthy",
	}

	start := time.Now()

	// Use test override if set, otherwise use real DB
	var err error
	if s.dbHealthChecker != nil {
		err = s.dbHealthChecker.PingContext(ctx)
	} else if s.db != nil {
		err = s.db.DB.PingContext(ctx)
	}

	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "unhealthy"
		result.Error = err.Error()
		s.logger.Error("Database health check failed", "error", err, "latency_ms", result.LatencyMs)
	}

	return result
}

// checkNATSWithLatency verifies NATS connectivity and measures latency
func (s *Server) checkNATSWithLatency() *HealthCheck {
	result := &HealthCheck{
		Status: "healthy",
	}

	start := time.Now()

	// Use test override if set, otherwise use real NATS client
	var connected bool
	if s.natsHealthChecker != nil {
		connected = s.natsHealthChecker.IsConnected()
	} else if s.natsClient != nil {
		connected = s.natsClient.IsConnected()
	}

	result.LatencyMs = time.Since(start).Milliseconds()

	if !connected {
		result.Status = "unhealthy"
		result.Error = "not connected"
		s.logger.Error("NATS health check failed", "latency_ms", result.LatencyMs)
	}

	return result
}

// checkRedisWithLatency verifies Redis connectivity and measures latency
func (s *Server) checkRedisWithLatency(ctx context.Context) *HealthCheck {
	result := &HealthCheck{
		Status: "healthy",
	}

	start := time.Now()

	// Use test override if set, otherwise use real Redis service
	var err error
	if s.redisHealthChecker != nil {
		err = s.redisHealthChecker.HealthCheck(ctx)
	} else if s.redisService != nil {
		err = s.redisService.HealthCheck(ctx)
	}

	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "unhealthy"
		result.Error = err.Error()
		s.logger.Error("Redis health check failed", "error", err, "latency_ms", result.LatencyMs)
	}

	return result
}

// checkProxmoxWithLatency verifies Proxmox availability using circuit breaker state
func (s *Server) checkProxmoxWithLatency() *HealthCheck {
	result := &HealthCheck{
		Status: "not_configured",
	}

	start := time.Now()

	// Type assertion to access Proxmox-specific methods (not part of Client interface)
	orch, ok := s.orchestrator.(*orchestrator.Orchestrator)
	if !ok {
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}

	client := orch.ProxmoxCircuitStats()
	if client == nil {
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}

	stats := client.CircuitStats()
	if stats.Name == "" {
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	result.CircuitState = stats.State
	result.Failures = stats.Failures

	switch stats.State {
	case "closed":
		result.Status = "healthy"
	case "half-open":
		result.Status = "recovering"
	case "open":
		result.Status = "unhealthy"
		result.LastFailure = stats.LastFailure.Format(time.RFC3339)
	}

	return result
}

// handleVersion godoc
// @Summary Build version
// @Description Returns the version, git commit and build time embedded at link time.
// @Description SERVED AT THE HOST ROOT, not under the /api/v1 basePath shown above -- Swagger 2.0 cannot express a per-path basePath, so the path below reads as /api/v1/... but the real URL omits that prefix.
// @Tags health
// @Produce json
// @Success 200 {object} version.Info "Build metadata"
// @Router /version [get]
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.responder.JSONResponse(w, http.StatusOK, version.Get())
}

// Lab handlers are in lab_handlers.go

// Pod handlers are in pods/ sub-package

// Session handlers are in sessions/ sub-package

// Event handlers are in event_handlers.go

// WebSocket handlers are in websocket_handlers.go

// -----------------------------------------------------------------------------
// Response Helpers
// -----------------------------------------------------------------------------
//
// Response writing for the handful of Server-owned endpoints (health,
// readiness, version, metrics auth, admin middleware) goes through
// s.responder (*httputil.Responder). All manager-owned handlers have their
// own responder instance. The previous s.jsonResponse / s.errorResponse /
// s.JSONResponse / s.ErrorResponse adapters were removed when the last
// caller migrated — they duplicated httputil.Responder methods verbatim
// and were never actually plumbed into the audit.ResponseWriter or
// features.ResponseWriter interfaces they claimed to satisfy.

// LTI handlers are in lti_handlers.go

// Auth handlers are in auth_handlers.go
