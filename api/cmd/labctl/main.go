// @title Kootenai API
// @version 1.0
// @description API for the Kootenai cybersecurity education platform
// @description Orchestrates lab environments across Proxmox and CloudStack
// No @termsOfService: the swaggo scaffold defaults it to swagger.io's terms,
// which have nothing to do with this project.

// No @contact.email. It read support@virtual-lab.local, which is a dead
// address on the mDNS-reserved .local TLD -- it never resolved and could not
// receive mail. The repository is the contact point.
// @contact.name Project repository
// @contact.url https://github.com/toddbartholow/kootenai

// @license.name Apache 2.0
// @license.url https://www.apache.org/licenses/LICENSE-2.0

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token. Format: "Bearer {token}"

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"gopkg.in/yaml.v3"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/auth"
	authldap "github.com/toddbartholow/kootenai/api/internal/auth/ldap"
	"github.com/toddbartholow/kootenai/api/internal/certificate"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/classroom"
	"github.com/toddbartholow/kootenai/api/internal/cli"
	"github.com/toddbartholow/kootenai/api/internal/cloudstack"
	"github.com/toddbartholow/kootenai/api/internal/config"
	"github.com/toddbartholow/kootenai/api/internal/database"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/logging"
	"github.com/toddbartholow/kootenai/api/internal/metrics"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	natsclient "github.com/toddbartholow/kootenai/api/internal/nats"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/proxmox"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server"
	"github.com/toddbartholow/kootenai/api/internal/simulation"
	"github.com/toddbartholow/kootenai/api/internal/templates"
	"github.com/toddbartholow/kootenai/api/internal/wazuh"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// Version information - set at build time via ldflags
// Example: go build -ldflags "-X main.Version=v0.2.1 -X main.Commit=abc1234 -X main.BuildTime=2024-12-24T12:00:00Z"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// AppConfig is an alias for config.Config for backwards compatibility
type AppConfig = config.Config

func main() {
	// Pop --locale out of argv before subcommand dispatch so
	// `labctl --locale es lab list` works without each FlagSet owning the
	// flag. Install the localizer immediately so printUsage/printVersion and
	// all subcommands use it. An unknown locale yields a one-shot stderr
	// warning and falls back to English (acceptance criterion on #119).
	args, localeFlag := cli.ExtractLocaleFlag(os.Args[1:])
	if _, warning := cli.InstallLocalizer(localeFlag); warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "serve":
		runServer()
	case "migrate":
		runMigrate()
	case "version":
		printVersion()
	case "lab":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.errors.requiresSubcommand", map[string]any{"Group": "lab"}))
			os.Exit(1)
		}
		handleLabCommand(args[1:])
	case "pod":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.errors.requiresSubcommand", map[string]any{"Group": "pod"}))
			os.Exit(1)
		}
		handlePodCommand(args[1:])
	case "snapshot":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.errors.requiresSubcommand", map[string]any{"Group": "snapshot"}))
			os.Exit(1)
		}
		handleSnapshotCommand(args[1:])
	case "session":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.errors.requiresSubcommand", map[string]any{"Group": "session"}))
			os.Exit(1)
		}
		handleSessionCommand(args[1:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(cli.T("labctl.usage.heading", nil))
	fmt.Println()
	fmt.Println(cli.T("labctl.usage.usageLabel", nil))
	fmt.Printf("  %s\n", cli.T("labctl.usage.pattern", nil))
	fmt.Println()
	fmt.Println(cli.T("labctl.usage.commandsLabel", nil))
	fmt.Printf("  serve       %s\n", cli.T("labctl.usage.cmdServe", nil))
	fmt.Printf("  migrate     %s\n", cli.T("labctl.usage.cmdMigrate", nil))
	fmt.Printf("  version     %s\n", cli.T("labctl.usage.cmdVersion", nil))
	fmt.Printf("  lab         %s\n", cli.T("labctl.usage.cmdLab", nil))
	fmt.Printf("  pod         %s\n", cli.T("labctl.usage.cmdPod", nil))
	fmt.Printf("  snapshot    %s\n", cli.T("labctl.usage.cmdSnapshot", nil))
	fmt.Printf("  session     %s\n", cli.T("labctl.usage.cmdSession", nil))
	fmt.Println()
	fmt.Println(cli.T("labctl.usage.helpHint", nil))
}

func printVersion() {
	fmt.Println(cli.T("labctl.version.line", map[string]any{"Version": Version}))
	fmt.Printf("  %s\n", cli.T("labctl.version.commit", map[string]any{"Commit": Commit}))
	fmt.Printf("  %s\n", cli.T("labctl.version.built", map[string]any{"BuildTime": BuildTime}))
	fmt.Printf("  %s\n", cli.T("labctl.version.goVersion", map[string]any{"GoVersion": runtime.Version()}))
	fmt.Printf("  %s\n", cli.T("labctl.version.osArch", map[string]any{"OS": runtime.GOOS, "Arch": runtime.GOARCH}))
}

func loadConfig() *AppConfig {
	// Read .env before anything else, so the `cp deploy/.env.example .env`
	// step both quickstarts open with actually affects the process. This used
	// to be reachable only through config.Load(), which has no callers, so
	// godotenv was wired up but dead: labctl fell back to compiled defaults
	// while docker-compose.dev.yml read the same .env and brought Postgres up
	// with different credentials. The result was a server that logged one
	// warning, ran with no database, and still answered /health/ready.
	// Missing or unreadable is fine -- environment and YAML still win.
	_ = config.LoadEnv()

	// Load defaults first
	cfg := config.LoadDefaults()

	// Try to load config file for YAML-based configuration
	configPaths := []string{
		"config.yaml",
		"config/config.yaml",
		"../config/config.yaml",
		"/app/config.yaml",
		"/etc/labctl/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".labctl", "config.yaml"),
	}

	for _, path := range configPaths {
		// #nosec G304 -- Config paths are hardcoded (current dir, home dir).
		// CLI is admin-only tooling, not exposed to end users.
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				fmt.Fprintln(os.Stderr, cli.T("labctl.config.parseWarning", map[string]any{"Path": path, "Err": err}))
			} else {
				fmt.Println(cli.T("labctl.config.loaded", map[string]any{"Path": path}))
				break
			}
		}
	}

	// Finally, override with environment variables (highest priority)
	cfg.LoadFromEnv()

	return cfg
}

func setupLogger(cfg *AppConfig) (*logging.Logger, error) {
	// Support LOG_OUTPUT and LOG_FILE environment variables
	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		cfg.Logging.Output = output
	}
	if filePath := os.Getenv("LOG_FILE"); filePath != "" {
		cfg.Logging.FilePath = filePath
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Logging.Format = format
	}

	return logging.Setup(cfg.Logging)
}

func runServer() {
	cfg := loadConfig()
	logWrapper, err := setupLogger(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, cli.T("labctl.errors.setupLoggerFailed", map[string]any{"Err": err}))
		os.Exit(1)
	}
	defer logWrapper.Close()
	logger := logWrapper.Logger

	logger.Info("Starting labctl API server",
		"version", Version,
		"commit", Commit,
		"build_time", BuildTime,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Initialize core services
	db, eventRepo := initDatabase(ctx, cfg, logger)
	if db != nil {
		defer db.Close()
	}

	redisService := initRedis(ctx, cfg, logger)
	if redisService != nil {
		defer redisService.Close()
	}

	// One registry, built before the subsystems that write into it. The
	// active-check runner and the NATS checkpoint consumer start their own
	// goroutines during the two calls below, so the recorder has to be in
	// place at construction rather than set afterwards.
	appMetrics := metrics.New()

	proxmoxClient, cloudstackClient := initInfraClients(cfg, logger)
	orch := initOrchestrator(ctx, cfg, proxmoxClient, cloudstackClient, db, appMetrics, logger)
	evaluator, activeCheckRunner, _ := setupCheckpointEvaluator(ctx, db, proxmoxClient, orch, appMetrics, logger)
	defer activeCheckRunner.Stop()

	// Initialize Wazuh security components
	wazuhSecurityOpts, tamperingDetector := initWazuhSecurity(ctx, cfg, evaluator, logger)
	defer func() {
		if err := tamperingDetector.Stop(5 * time.Second); err != nil {
			logger.Warn("Tampering detector did not stop cleanly", "error", err)
		}
	}()

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(logger)
	go wsHub.Run(ctx)

	// Initialize NATS and wire up checkpoint callbacks
	natsClient, asyncProvisioner := initNATSClient(ctx, cfg, db, evaluator, wsHub, orch, eventRepo, logger)
	if natsClient != nil {
		defer natsClient.Close()
	}

	// Build server options and start
	serverOpts := buildServerOptions(cfg, db, redisService, activeCheckRunner, asyncProvisioner, logger)
	serverOpts = append(serverOpts, wazuhSecurityOpts...)
	serverCfg := cfg.Server
	serverCfg.DemoMode = cfg.Auth.DemoMode
	serverCfg.MetricsToken = os.Getenv("METRICS_TOKEN")
	serverCfg.Environment = os.Getenv("ENV")
	serverCfg.CORSAllowWildcard = os.Getenv("CORS_ALLOW_WILDCARD") == "true"
	serverCfg.LTIAllowInsecureKeys = os.Getenv("LTI_ALLOW_INSECURE_KEYS") == "true"
	serverOpts = append(serverOpts, server.WithMetrics(appMetrics))
	srv, err := server.New(serverCfg, orch, evaluator, wsHub, logger, serverOpts...)
	if err != nil {
		logger.Error("Failed to construct server", "error", err)
		os.Exit(1)
	}

	srv.StartBackgroundServices(ctx)
	defer srv.StopBackgroundServices()

	if err := srv.Run(ctx); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}

// initDatabase connects to the database, runs migrations, and loads templates from disk.
func initDatabase(ctx context.Context, cfg *AppConfig, logger *slog.Logger) (*database.DB, repositories.EventRepository) {
	db, err := database.New(cfg.Database, logger)
	if err != nil {
		logger.Warn("Failed to connect to database", "error", err)
		logger.Info("Running without database - events will not be persisted")
		return nil, nil
	}

	if err := db.Migrate(ctx); err != nil {
		logger.Error("Failed to run database migrations", "error", err)
		db.Close()
		os.Exit(1) //nolint:gocritic // db.Close() called explicitly above
	}

	eventRepo := repositories.NewEventRepo(db.DB)

	labTemplateRepo := repositories.NewLabTemplateRepo(db.DB)
	templateLoader := templates.NewLoader(labTemplateRepo, logger)
	if err := templateLoader.LoadFromDirectory(ctx, cfg.TemplatesDir); err != nil {
		logger.Error("Failed to load templates", "error", err)
	}

	return db, eventRepo
}

// initRedis connects to Redis if configured.
func initRedis(ctx context.Context, cfg *AppConfig, logger *slog.Logger) *redisclient.Service {
	if !cfg.HasRedis() {
		return nil
	}

	redisService, err := redisclient.NewService(redisclient.ServiceConfig{
		Redis:      cfg.Redis,
		ServerNode: fmt.Sprintf("api-%d", os.Getpid()),
		Logger:     logger,
	})
	if err != nil {
		logger.Warn("Failed to initialize Redis service", "error", err)
		return nil
	}

	if err := redisService.Connect(ctx); err != nil {
		logger.Warn("Failed to connect to Redis", "error", err)
		return nil
	}

	redisService.StartPubSub(ctx)
	logger.Info("Redis service initialized",
		slog.String("host", cfg.Redis.Host),
		slog.Int("port", cfg.Redis.Port),
	)
	return redisService
}

// initInfraClients initializes Proxmox and CloudStack clients.
func initInfraClients(cfg *AppConfig, logger *slog.Logger) (*proxmox.Client, *cloudstack.Client) {
	proxmoxClient, err := proxmox.NewClient(cfg.Proxmox, proxmox.WithLogger(logger))
	if err != nil {
		logger.Warn("Failed to initialize Proxmox client", "error", err)
	}

	cloudstackClient, err := cloudstack.NewClient(cfg.CloudStack)
	if err != nil {
		logger.Warn("Failed to initialize CloudStack client", "error", err)
	}

	return proxmoxClient, cloudstackClient
}

// initOrchestrator creates and configures the lab orchestrator.
func initOrchestrator(
	ctx context.Context,
	cfg *AppConfig,
	proxmoxClient *proxmox.Client,
	cloudstackClient *cloudstack.Client,
	db *database.DB,
	appMetrics *metrics.Metrics,
	logger *slog.Logger,
) *orchestrator.Orchestrator {
	orchConfig := orchestrator.Config{
		DefaultPlatform:    models.PlatformProxmox,
		ProxmoxNode:        cfg.Proxmox.DefaultNode,
		MaxPodsPerUser:     5,
		DefaultPodDuration: 4 * time.Hour,
		TemplateVMIDs:      cfg.TemplateVMIDs,
	}

	if orchConfig.ProxmoxNode == "" {
		orchConfig.ProxmoxNode = "pve"
	}

	if len(cfg.TemplateVMIDs) > 0 {
		logger.Info("Loaded template VMID mappings", "count", len(cfg.TemplateVMIDs))
		for name, vmid := range cfg.TemplateVMIDs {
			logger.Debug("Template mapping", "template", name, "vmid", vmid)
		}
	} else {
		logger.Warn("No template VMID mappings configured - pod creation will fail")
	}

	var orchOpts []orchestrator.Option
	orchOpts = append(orchOpts, orchestrator.WithLogger(logger))
	if appMetrics != nil {
		orchOpts = append(orchOpts, orchestrator.WithMetrics(appMetrics))
	}

	if db != nil {
		podRepo := repositories.NewPodRepo(db.DB)
		orchOpts = append(orchOpts, orchestrator.WithPodRepository(podRepo))
	}

	orch := orchestrator.New(proxmoxClient, cloudstackClient, orchConfig, orchOpts...)

	if db != nil {
		if err := orch.LoadPodsFromDatabase(ctx); err != nil {
			logger.Warn("Failed to load pods from database", "error", err)
		}
	}

	return orch
}

// setupCheckpointEvaluator creates the checkpoint evaluator, active verifier, and active check runner.
func setupCheckpointEvaluator(
	ctx context.Context,
	db *database.DB,
	proxmoxClient *proxmox.Client,
	orch *orchestrator.Orchestrator,
	appMetrics *metrics.Metrics,
	logger *slog.Logger,
) (*checkpoint.Evaluator, *checkpoint.ActiveCheckRunner, *checkpoint.ActiveVerifier) {
	var evalOpts []checkpoint.EvaluatorOption
	if appMetrics != nil {
		evalOpts = append(evalOpts, checkpoint.WithMetrics(appMetrics))
	}
	evaluator := checkpoint.NewEvaluator(logger, evalOpts...)

	// Load lab templates from database and register with evaluator
	if db != nil {
		labTemplateRepo := repositories.NewLabTemplateRepo(db.DB)
		activeTrue := true
		tmplRecords, err := labTemplateRepo.List(ctx, repositories.LabTemplateFilter{Active: &activeTrue})
		if err != nil {
			logger.Warn("Failed to load lab templates for evaluator", "error", err)
		} else {
			for _, record := range tmplRecords {
				labTemplate, err := record.ToLabTemplate()
				if err != nil {
					logger.Warn("Failed to convert template record", "name", record.Name, "error", err)
					continue
				}
				if err := evaluator.RegisterTemplate(labTemplate); err != nil {
					logger.Warn("Failed to register template with evaluator", "name", record.Name, "error", err)
				}
			}
			logger.Info("Registered lab templates with checkpoint evaluator", "count", len(tmplRecords))
		}
	}

	// Create active verifier for cross-checking passive event results via QEMU guest agent
	var activeVerifier *checkpoint.ActiveVerifier
	if orch != nil {
		activeVerifier = checkpoint.NewActiveVerifier(orch, logger, checkpoint.DefaultActiveVerifyConfig())
		evaluator.SetActiveVerifier(activeVerifier)
		logger.Info("Active verifier enabled — checkpoints will be cross-verified via QEMU guest agent")
	}

	var activeCheckOpts []checkpoint.ActiveCheckRunnerOption
	if proxmoxClient != nil {
		activeCheckOpts = append(activeCheckOpts, checkpoint.WithVMIPLookup(proxmoxClient))
	}
	activeCheckRunner := checkpoint.NewActiveCheckRunner(evaluator, logger, activeCheckOpts...)
	go activeCheckRunner.Start(ctx)

	return evaluator, activeCheckRunner, activeVerifier
}

// initWazuhSecurity creates Wazuh security components: webhook validator, event deduplicator,
// health monitor, and tampering detector. Returns server options to wire them in.
func initWazuhSecurity(
	ctx context.Context,
	cfg *AppConfig,
	evaluator *checkpoint.Evaluator,
	logger *slog.Logger,
) ([]server.ServerOption, *checkpoint.TamperingDetector) {
	var opts []server.ServerOption

	// Webhook validator (HMAC signature verification, IP allowlist, rate limiting)
	securityCfg := wazuh.SecurityConfig{
		WebhookSecret:      cfg.Wazuh.WebhookSecret,
		RequireSignature:   cfg.Wazuh.RequireSignature,
		AllowedIPs:         cfg.Wazuh.AllowedIPs,
		RateLimitPerMinute: 100,
		MaxTimestampDrift:  5 * time.Minute,
	}
	if securityCfg.WebhookSecret != "" || securityCfg.RequireSignature {
		validator, err := wazuh.NewWebhookValidator(ctx, securityCfg, logger)
		if err != nil {
			logger.Error("Failed to create webhook validator", "error", err)
		} else {
			opts = append(opts, server.WithWebhookValidator(validator))
		}
	}

	// Event deduplicator (replay attack prevention)
	dedup := wazuh.NewEventDeduplicator(10 * time.Minute)
	opts = append(opts, server.WithEventDeduplicator(dedup))

	// Health monitor (agent heartbeat tracking)
	var apiClient *wazuh.APIClient
	if cfg.Wazuh.ManagerURL != "" {
		apiClient = wazuh.NewAPIClient(wazuh.APIClientConfig{
			ManagerURL:         cfg.Wazuh.ManagerURL,
			Username:           cfg.Wazuh.ManagerUsername,
			Password:           cfg.Wazuh.ManagerPassword,
			InsecureSkipVerify: cfg.Wazuh.InsecureSkipVerify,
		})
	}
	healthMonitor := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), logger, apiClient)
	go healthMonitor.Start(ctx)
	opts = append(opts, server.WithWazuhHealthMonitor(healthMonitor))

	// Tampering detector (penalties for offline agents, verification mismatches)
	tamperingDetector := checkpoint.NewTamperingDetector(
		checkpoint.DefaultTamperingConfig(),
		logger,
		healthMonitor,
	)
	evaluator.SetTamperingDetector(tamperingDetector)
	opts = append(opts, server.WithTamperingDetector(tamperingDetector))

	// Wazuh service for agent status queries (used by pre-session health check)
	if cfg.Wazuh.ManagerURL != "" {
		wazuhService := wazuh.NewService(cfg.Wazuh, logger, nil, nil, nil, nil, nil)
		opts = append(opts, server.WithWazuhService(wazuhService))
	}

	logger.Info("Wazuh security initialized",
		"webhookAuth", securityCfg.WebhookSecret != "" || securityCfg.RequireSignature,
		"deduplication", true,
		"healthMonitor", true,
		"tamperingDetection", true,
	)

	return opts, tamperingDetector
}

// initNATSClient initializes NATS, wires up checkpoint callbacks, and creates the async provisioner.
func initNATSClient(
	ctx context.Context,
	cfg *AppConfig,
	db *database.DB,
	evaluator *checkpoint.Evaluator,
	wsHub *websocket.Hub,
	orch *orchestrator.Orchestrator,
	eventRepo repositories.EventRepository,
	logger *slog.Logger,
) (*natsclient.Client, *orchestrator.AsyncProvisioner) {
	// Create checkpoint repos for persistence callbacks
	var checkpointRepo *repositories.CheckpointProgressRepo
	var sessionRepo *repositories.SessionRepo
	if db != nil {
		checkpointRepo = repositories.NewCheckpointProgressRepo(db.DB)
		sessionRepo = repositories.NewSessionRepo(db.DB)
	}

	natsClient, err := natsclient.NewClient(cfg.NATS, logger)
	if err != nil {
		logger.Warn("Failed to connect to NATS", "error", err)
		logger.Info("Running without NATS - events will not be persisted")

		// Without NATS, still broadcast checkpoint updates via WebSocket
		evaluator.SetCheckpointUpdateCallback(func(ctx context.Context, update *events.CheckpointUpdate) error {
			if checkpointRepo != nil && sessionRepo != nil {
				persistCheckpointUpdate(ctx, update, checkpointRepo, sessionRepo, logger)
			}
			wsHub.BroadcastCheckpoint(update)
			return nil
		})
		return nil, nil
	}

	// Set up checkpoint update callback to publish to NATS and persist to database
	evaluator.SetCheckpointUpdateCallback(func(ctx context.Context, update *events.CheckpointUpdate) error {
		if checkpointRepo != nil && sessionRepo != nil {
			persistCheckpointUpdate(ctx, update, checkpointRepo, sessionRepo, logger)
		}
		if err := natsClient.PublishCheckpoint(ctx, update); err != nil {
			logger.Error("Failed to publish checkpoint to NATS", "error", err)
		}
		wsHub.BroadcastCheckpoint(update)
		return nil
	})

	startEventConsumers(ctx, natsClient, evaluator, wsHub, eventRepo, logger)

	// Create async provisioner
	broadcaster := &provisioningBroadcaster{hub: wsHub}
	asyncProvisioner := orchestrator.NewAsyncProvisioner(
		orch,
		natsClient,
		logger,
		orchestrator.WithBroadcaster(broadcaster),
	)
	logger.Info("Async pod provisioning enabled")

	return natsClient, asyncProvisioner
}

// buildServerOptions constructs the server options slice from all available services and repositories.
func buildServerOptions(
	cfg *AppConfig,
	db *database.DB,
	redisService *redisclient.Service,
	activeCheckRunner *checkpoint.ActiveCheckRunner,
	asyncProvisioner *orchestrator.AsyncProvisioner,
	logger *slog.Logger,
) []server.ServerOption {
	authService, err := auth.NewService(cfg.Auth)
	if err != nil {
		logger.Error("Failed to initialize auth service", "error", err)
		os.Exit(1)
	}

	// Build cookie config from auth settings (ADR-0002 Phase B)
	cookieCfg := auth.DefaultCookieConfig()
	cookieCfg.Enabled = cfg.Auth.CookieMode
	cookieCfg.MaxAge = cfg.Auth.JWTExpiration
	if cookieCfg.Enabled {
		logger.Info("HttpOnly cookie auth enabled (ADR-0002 Phase B)")
	}

	serverOpts := []server.ServerOption{
		server.WithAuthService(authService),
		server.WithActiveCheckRunner(activeCheckRunner),
		server.WithCookieConfig(cookieCfg),
	}

	// Initialize LDAP client if configured
	if cfg.HasLDAP() {
		if err := cfg.LDAP.Validate(); err != nil {
			logger.Error("Invalid LDAP configuration", "error", err)
			os.Exit(1)
		}
		ldapClient, err := authldap.NewClient(cfg.LDAP, logger)
		if err != nil {
			logger.Error("Failed to create LDAP client", "error", err)
			os.Exit(1)
		}
		logger.Info("LDAP authentication enabled", "host", cfg.LDAP.Host)
		serverOpts = append(serverOpts, server.WithLDAPClient(ldapClient))
	}
	if redisService != nil {
		serverOpts = append(serverOpts, server.WithRedisService(redisService))
	}
	if asyncProvisioner != nil {
		serverOpts = append(serverOpts, server.WithAsyncProvisioner(asyncProvisioner))
	}
	if db == nil {
		return serverOpts
	}

	// Create base repositories
	sessionRepo := repositories.NewSessionRepo(db.DB)
	checkpointRepo := repositories.NewCheckpointProgressRepo(db.DB)
	checkpointHintRepo := repositories.NewCheckpointHintRepo(db.DB)
	questionResponseRepo := repositories.NewQuestionResponseRepo(db.DB)
	baseLabTemplateRepo := repositories.NewLabTemplateRepo(db.DB)
	basePodRepo := repositories.NewPodRepo(db.DB)
	baseUserRepo := repositories.NewUserRepo(db.DB)
	achievementRepo := repositories.NewAchievementRepo(db.DB)
	pathwayRepo := repositories.NewPathwayRepo(db.DB)
	enrollmentRepo := repositories.NewEnrollmentRepo(db.DB)
	certificateRepo := repositories.NewCertificateRepo(db.DB)
	passwordResetRepo := repositories.NewPasswordResetTokenRepo(db.DB)
	eventRepo := repositories.NewEventRepo(db.DB)
	baseRBACRepo := repositories.NewRBACRepo(db.DB)

	// Wrap repositories with Redis caching if available
	var labTemplateRepo repositories.LabTemplateRepository = baseLabTemplateRepo
	var podRepo repositories.PodRepository = basePodRepo
	var userRepo repositories.UserRepository = baseUserRepo
	var cachedAchievementRepo repositories.AchievementRepository = achievementRepo
	var rbacRepo repositories.RBACRepository = baseRBACRepo

	if redisService != nil {
		labTemplateRepo = repositories.NewCachedLabTemplateRepo(baseLabTemplateRepo, redisService.Templates, logger)
		podRepo = repositories.NewCachedPodRepo(basePodRepo, redisService.Pods, logger)
		userRepo = repositories.NewCachedUserRepo(baseUserRepo, redisService.Users, logger)
		cachedAchievementRepo = repositories.NewCachedAchievementRepo(achievementRepo, redisService.Achievements, logger)
		rbacRepo = repositories.NewCachedRBACRepo(baseRBACRepo, redisService.Permissions, logger)
		logger.Info("Redis caching enabled for repositories")
	}

	// Create ownership service for resource access control
	ownershipService := middleware.NewOwnershipService(
		func(ctx context.Context, resourceID string, user *auth.User) (bool, error) {
			return podRepo.IsOwner(ctx, resourceID, user.ID)
		},
		func(ctx context.Context, resourceID string, user *auth.User) (bool, error) {
			return sessionRepo.IsOwner(ctx, resourceID, user.ID)
		},
	)

	achievementService := achievements.NewService(cachedAchievementRepo, sessionRepo, logger).
		WithLabTemplateRepo(labTemplateRepo).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	certificateService := certificate.NewService(
		certificateRepo,
		enrollmentRepo,
		pathwayRepo,
		userRepo,
		certificate.Config{BaseURL: "https://lab.example.com"}, // TODO: Make configurable via env
		logger,
	)

	// Create multi-tenancy repositories
	orgRepo := repositories.NewOrganizationRepo(db.DB)
	orgMembershipRepo := repositories.NewOrganizationMembershipRepo(db.DB)
	teamRepo := repositories.NewTeamRepo(db.DB)
	teamMembershipRepo := repositories.NewTeamMembershipRepo(db.DB)
	featureRepo := repositories.NewFeatureRepo(db.DB)
	licenseRepo := repositories.NewLicenseRepo(db.DB)
	ltiAssignmentRepo := repositories.NewLTIAssignmentRepo(db.DB)

	tenantConfig := middleware.DefaultTenantConfig()
	tenantService := middleware.NewTenantService(orgRepo, orgMembershipRepo, featureRepo, tenantConfig)

	serverOpts = append(serverOpts,
		server.WithLabTemplateRepo(labTemplateRepo),
		server.WithPodRepo(podRepo),
		server.WithSessionRepo(sessionRepo),
		server.WithCheckpointRepo(checkpointRepo),
		server.WithCheckpointHintRepo(checkpointHintRepo),
		server.WithQuestionResponseRepo(questionResponseRepo),
		server.WithUserRepo(userRepo),
		server.WithAchievementRepo(cachedAchievementRepo),
		server.WithAchievementService(achievementService),
		server.WithCertificateService(certificateService),
		server.WithPathwayRepo(pathwayRepo),
		server.WithEnrollmentRepo(enrollmentRepo),
		server.WithPasswordResetRepo(passwordResetRepo),
		server.WithOwnershipService(ownershipService),
		server.WithEventRepo(eventRepo),
		server.WithOrganizationRepo(orgRepo),
		server.WithOrganizationMembershipRepo(orgMembershipRepo),
		server.WithTeamRepo(teamRepo),
		server.WithTeamMembershipRepo(teamMembershipRepo),
		server.WithFeatureRepo(featureRepo),
		server.WithLicenseRepo(licenseRepo),
		server.WithRBACRepo(rbacRepo),
		server.WithTenantService(tenantService),
		server.WithLTIAssignmentRepo(ltiAssignmentRepo),
	)

	// Initialize simulation service for test students
	simService := simulation.NewService(
		db.DB,
		enrollmentRepo,
		pathwayRepo,
		achievementRepo,
		logger,
	)
	serverOpts = append(serverOpts, server.WithSimulationService(simService))

	// Initialize classroom simulation service (enterprise feature)
	classroomRepo := classroom.NewPostgresRepository(db.DB)
	classroomService := classroom.NewService(classroomRepo, logger)
	serverOpts = append(serverOpts, server.WithClassroomService(classroomService))

	// Initialize classroom runner with optional LLM and Canvas clients
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	var llmClient *classroom.AnthropicClient
	if anthropicKey != "" {
		llmClient = classroom.NewAnthropicClient(anthropicKey, logger)
		logger.Info("Classroom LLM client initialized")
	}

	canvasURL := os.Getenv("CANVAS_BASE_URL")
	canvasToken := os.Getenv("CANVAS_API_TOKEN")
	var canvasClient *classroom.CanvasClient
	if canvasURL != "" && canvasToken != "" {
		canvasClient = classroom.NewCanvasClient(canvasURL, canvasToken, logger)
		logger.Info("Classroom Canvas client initialized")
	}

	labExec := classroom.NewLabExecutor(os.Getenv("LABTEST_PATH"), logger)
	apiClient := classroom.NewLabAPIClient("http://localhost:8080/api/v1", logger)

	classroomRunner := classroom.NewClassroomRunner(
		classroomRepo,
		classroomService,
		llmClient,
		canvasClient,
		labExec,
		logger,
		classroom.WithAPIClient(apiClient),
		classroom.WithPathwayRepo(pathwayRepo),
	)
	serverOpts = append(serverOpts, server.WithClassroomRunner(classroomRunner))

	return serverOpts
}

func runMigrate() {
	cfg := loadConfig()
	logWrapper, err := setupLogger(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, cli.T("labctl.errors.setupLoggerFailed", map[string]any{"Err": err}))
		os.Exit(1)
	}
	defer logWrapper.Close()
	logger := logWrapper.Logger

	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	statusOnly := fs.Bool("status", false, "Show migration status without applying")
	fs.Parse(os.Args[2:])

	ctx := context.Background()

	// Connect to database
	db, err := database.New(cfg.Database, logger)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		logWrapper.Close()
		os.Exit(1) //nolint:gocritic // logWrapper.Close() called explicitly above
	}
	defer db.Close()

	if *statusOnly {
		// Just show status
		if err := db.PrintMigrationStatus(ctx, logger); err != nil {
			logger.Error("Failed to get migration status", "error", err)
			os.Exit(1)
		}
		return
	}

	// Run migrations
	if err := db.Migrate(ctx); err != nil {
		logger.Error("Migration failed", "error", err)
		os.Exit(1)
	}

	// Print final status
	logger.Info("Migration completed successfully")
	if err := db.PrintMigrationStatus(ctx, logger); err != nil {
		logger.Error("Failed to get migration status", "error", err)
	}
}

func startEventConsumers(
	ctx context.Context,
	nc *natsclient.Client,
	evaluator *checkpoint.Evaluator,
	wsHub *websocket.Hub,
	eventRepo repositories.EventRepository,
	logger *slog.Logger,
) {
	// Event store consumer - stores all events to database
	eventStoreConsumer, err := natsclient.NewEventStoreConsumer(nc, func(ctx context.Context, msg jetstream.Msg) error {
		logger.Debug("Storing event", "subject", msg.Subject())

		// Parse the NATS message into an Event
		var event models.Event
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("Failed to unmarshal event", "error", err, "subject", msg.Subject())
			return fmt.Errorf("unmarshaling event: %w", err)
		}

		// Store event to database if repository is available
		if eventRepo != nil {
			if err := eventRepo.Create(ctx, &event); err != nil {
				logger.Error("Failed to store event", "error", err, "podId", event.PodID)
				return fmt.Errorf("storing event: %w", err)
			}
			logger.Debug("Event stored successfully", "id", event.ID, "podId", event.PodID, "type", event.EventType)
		} else {
			logger.Debug("Skipping event storage - no database configured")
		}

		return nil
	}, logger)
	if err != nil {
		logger.Error("Failed to create event store consumer", "error", err)
	} else {
		go eventStoreConsumer.Start(ctx)
	}

	// Checkpoint evaluator consumer
	checkpointConsumer, err := natsclient.NewCheckpointEvaluatorConsumer(nc, func(ctx context.Context, msg jetstream.Msg) error {
		logger.Debug("Evaluating event for checkpoints", "subject", msg.Subject())

		// Parse the NATS message as VMEvent for checkpoint evaluation
		vmEvent, err := events.ParseVMEvent(msg.Data())
		if err != nil {
			logger.Error("Failed to parse event for checkpoint evaluation", "error", err)
			return fmt.Errorf("parsing event: %w", err)
		}

		// Skip events without a session ID - can't evaluate checkpoints
		if vmEvent.SessionID == "" {
			logger.Debug("Skipping event without session ID", "podId", vmEvent.PodID)
			return nil
		}

		// Evaluate the event against session checkpoints
		passedCheckpoints, err := evaluator.EvaluateEvent(ctx, vmEvent, vmEvent.SessionID)
		if err != nil {
			// Session not found is expected for events from sessions not yet registered
			logger.Debug("Could not evaluate event",
				"error", err,
				"sessionId", vmEvent.SessionID,
				"podId", vmEvent.PodID,
			)
			return nil // Don't return error - message processed successfully
		}

		if len(passedCheckpoints) > 0 {
			logger.Info("Checkpoints passed from event",
				"sessionId", vmEvent.SessionID,
				"checkpoints", passedCheckpoints,
				"eventType", vmEvent.EventType,
			)
		}

		return nil
	}, logger)
	if err != nil {
		logger.Error("Failed to create checkpoint consumer", "error", err)
	} else {
		go checkpointConsumer.Start(ctx)
	}

}

// provisioningBroadcaster adapts WebSocket hub to orchestrator.ProvisioningBroadcaster interface
type provisioningBroadcaster struct {
	hub *websocket.Hub
}

// BroadcastProvisioningEvent implements orchestrator.ProvisioningBroadcaster
func (b *provisioningBroadcaster) BroadcastProvisioningEvent(event orchestrator.PodProvisioningEvent) {
	b.hub.BroadcastPodProvisioning(&websocket.PodProvisioningEvent{
		PodID:     event.PodID,
		OwnerID:   event.OwnerID,
		Status:    string(event.Status),
		Phase:     string(event.Phase),
		Message:   event.Message,
		Progress:  event.Progress,
		VMName:    event.VMName,
		VMStatus:  event.VMStatus,
		Error:     event.Error,
		Timestamp: event.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		RequestID: event.RequestID,
	})
}

// persistCheckpointUpdate saves checkpoint progress to the database and updates session earned points
func persistCheckpointUpdate(
	ctx context.Context,
	update *events.CheckpointUpdate,
	checkpointRepo *repositories.CheckpointProgressRepo,
	sessionRepo *repositories.SessionRepo,
	logger *slog.Logger,
) {
	// Check if checkpoint progress already exists
	existing, err := checkpointRepo.GetBySessionAndCheckpoint(ctx, update.SessionID, update.CheckpointID)
	if err != nil {
		logger.Error("Failed to check existing checkpoint progress", "error", err)
		return
	}

	if existing == nil {
		// Create new checkpoint progress record with a proper UUID
		now := time.Now()
		progress := &models.CheckpointProgress{
			ID:           uuid.New().String(),
			SessionID:    update.SessionID,
			CheckpointID: update.CheckpointID,
			Status:       update.Status,
			Points:       update.Points,
			EarnedPoints: update.EarnedPoints,
			PassedAt:     &now,
			AttemptCount: 1,
		}
		if err := checkpointRepo.Create(ctx, progress); err != nil {
			logger.Error("Failed to create checkpoint progress", "error", err)
		} else {
			logger.Info("Persisted checkpoint progress to database",
				"sessionId", update.SessionID,
				"checkpointId", update.CheckpointID,
				"earnedPoints", update.EarnedPoints,
			)
		}
	} else if existing.Status != models.CheckpointStatusPassed {
		// Update existing record to passed
		existing.Status = update.Status
		existing.EarnedPoints = update.EarnedPoints
		now := time.Now()
		existing.PassedAt = &now
		existing.AttemptCount++
		if err := checkpointRepo.Update(ctx, existing); err != nil {
			logger.Error("Failed to update checkpoint progress", "error", err)
		}
	}

	// Update session earned points
	if sessionRepo != nil {
		session, err := sessionRepo.GetByID(ctx, update.SessionID)
		if err != nil {
			logger.Error("Failed to get session for points update", "error", err)
			return
		}
		if session != nil {
			session.EarnedPoints = update.SessionEarnedPoints
			session.Percentage = update.SessionPercentage
			threshold := float64(session.PassingThreshold)
			if threshold == 0 {
				threshold = 70
			}
			session.Passed = update.SessionPercentage >= threshold
			if err := sessionRepo.Update(ctx, session); err != nil {
				logger.Error("Failed to update session earned points", "error", err)
			} else {
				logger.Info("Updated session earned points",
					"sessionId", update.SessionID,
					"earnedPoints", update.SessionEarnedPoints,
					"percentage", update.SessionPercentage,
				)
			}
		}
	}
}

func handleLabCommand(args []string) {
	cfg := loadConfig()
	client := cli.NewClient(cfg.CLI)
	labCmd := cli.NewLabCommands(client, cfg.TemplatesDir)
	ctx := context.Background()

	fs := flag.NewFlagSet("lab", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println(cli.T("labctl.lab.usage", nil))
	}

	if len(args) == 0 {
		fs.Usage()
		return
	}

	switch args[0] {
	case "list":
		if err := labCmd.List(ctx); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}
	case "show":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.lab.hints.show", nil))
			os.Exit(1)
		}
		if err := labCmd.Show(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}
	case "validate":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.lab.hints.validate", nil))
			os.Exit(1)
		}
		if err := labCmd.Validate(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}
	default:
		fmt.Println(cli.T("labctl.errors.unknownCommand", map[string]any{"Group": "lab", "Command": args[0]}))
		fs.Usage()
		os.Exit(1)
	}
}

func handlePodCommand(args []string) {
	cfg := loadConfig()
	client := cli.NewClient(cfg.CLI)
	podCmd := cli.NewPodCommands(client)
	ctx := context.Background()

	fs := flag.NewFlagSet("pod", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println(cli.T("labctl.pod.usage", nil))
	}

	if len(args) == 0 {
		fs.Usage()
		return
	}

	switch args[0] {
	case "list":
		listFs := flag.NewFlagSet("pod list", flag.ExitOnError)
		owner := listFs.String("owner", "", "Filter by owner")
		listFs.Parse(args[1:])

		if err := podCmd.List(ctx, *owner); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "create":
		createFs := flag.NewFlagSet("pod create", flag.ExitOnError)
		template := createFs.String("template", "", "Lab template name (required)")
		owner := createFs.String("owner", "", "Pod owner (required)")
		duration := createFs.Duration("duration", 0, "Pod duration (e.g., 4h)")
		createFs.Parse(args[1:])

		if *template == "" || *owner == "" {
			fmt.Println(cli.T("labctl.pod.hints.create", nil))
			os.Exit(1)
		}

		if err := podCmd.Create(ctx, *template, *owner, *duration); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "status":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.pod.hints.status", nil))
			os.Exit(1)
		}
		if err := podCmd.Status(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "destroy":
		destroyFs := flag.NewFlagSet("pod destroy", flag.ExitOnError)
		force := destroyFs.Bool("force", false, "Force destroy without confirmation")
		destroyFs.Parse(args[1:])

		remaining := destroyFs.Args()
		if len(remaining) < 1 {
			fmt.Println(cli.T("labctl.pod.hints.destroy", nil))
			os.Exit(1)
		}

		if err := podCmd.Destroy(ctx, remaining[0], *force); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "reset":
		resetFs := flag.NewFlagSet("pod reset", flag.ExitOnError)
		snapshot := resetFs.String("snapshot", "", "Snapshot to reset to")
		resetFs.Parse(args[1:])

		remaining := resetFs.Args()
		if len(remaining) < 1 {
			fmt.Println(cli.T("labctl.pod.hints.reset", nil))
			os.Exit(1)
		}

		if err := podCmd.Reset(ctx, remaining[0], *snapshot); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "reset-vm":
		resetVMFs := flag.NewFlagSet("pod reset-vm", flag.ExitOnError)
		snapshot := resetVMFs.String("snapshot", "", "Snapshot to reset to")
		resetVMFs.Parse(args[1:])

		remaining := resetVMFs.Args()
		if len(remaining) < 2 {
			fmt.Println(cli.T("labctl.pod.hints.resetVm", nil))
			os.Exit(1)
		}

		if err := podCmd.ResetVM(ctx, remaining[0], remaining[1], *snapshot); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	default:
		fmt.Println(cli.T("labctl.errors.unknownCommand", map[string]any{"Group": "pod", "Command": args[0]}))
		fs.Usage()
		os.Exit(1)
	}
}

func handleSnapshotCommand(args []string) {
	cfg := loadConfig()
	client := cli.NewClient(cfg.CLI)
	snapshotCmd := cli.NewSnapshotCommands(client)
	ctx := context.Background()

	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println(cli.T("labctl.snapshot.usage", nil))
	}

	if len(args) == 0 {
		fs.Usage()
		return
	}

	// Parse flags
	podID := fs.String("pod", "", "Pod ID")
	vmName := fs.String("vm", "", "VM name")
	snapshotName := fs.String("name", "", "Snapshot name")
	description := fs.String("desc", "", "Snapshot description")
	includeRAM := fs.Bool("ram", false, "Include RAM state in snapshot")

	switch args[0] {
	case "list":
		fs.Parse(args[1:])
		if *podID == "" || *vmName == "" {
			fmt.Println(cli.T("labctl.snapshot.errors.podVmRequired", nil))
			fs.Usage()
			os.Exit(1)
		}
		if err := snapshotCmd.List(ctx, *podID, *vmName); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "create":
		fs.Parse(args[1:])
		if *podID == "" || *vmName == "" || *snapshotName == "" {
			fmt.Println(cli.T("labctl.snapshot.errors.podVmNameRequired", nil))
			fs.Usage()
			os.Exit(1)
		}
		if err := snapshotCmd.Create(ctx, *podID, *vmName, *snapshotName, *description, *includeRAM); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "revert":
		fs.Parse(args[1:])
		if *podID == "" || *vmName == "" || *snapshotName == "" {
			fmt.Println(cli.T("labctl.snapshot.errors.podVmNameRequired", nil))
			fs.Usage()
			os.Exit(1)
		}
		if err := snapshotCmd.Revert(ctx, *podID, *vmName, *snapshotName); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "delete":
		fs.Parse(args[1:])
		if *podID == "" || *vmName == "" || *snapshotName == "" {
			fmt.Println(cli.T("labctl.snapshot.errors.podVmNameRequired", nil))
			fs.Usage()
			os.Exit(1)
		}
		if err := snapshotCmd.Delete(ctx, *podID, *vmName, *snapshotName); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	default:
		fmt.Println(cli.T("labctl.errors.unknownCommand", map[string]any{"Group": "snapshot", "Command": args[0]}))
		fs.Usage()
		os.Exit(1)
	}
}

func handleSessionCommand(args []string) {
	cfg := loadConfig()
	client := cli.NewClient(cfg.CLI)
	sessionCmd := cli.NewSessionCommands(client)
	ctx := context.Background()

	fs := flag.NewFlagSet("session", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println(cli.T("labctl.session.usage", nil))
	}

	if len(args) == 0 {
		fs.Usage()
		return
	}

	switch args[0] {
	case "list":
		listFs := flag.NewFlagSet("session list", flag.ExitOnError)
		userID := listFs.String("user", "", "Filter by user ID")
		active := listFs.Bool("active", false, "Show only active sessions")
		listFs.Parse(args[1:])

		if err := sessionCmd.List(ctx, *userID, *active); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "start":
		startFs := flag.NewFlagSet("session start", flag.ExitOnError)
		podID := startFs.String("pod", "", "Pod ID (required)")
		userID := startFs.String("user", "", "User ID (required)")
		template := startFs.String("template", "", "Lab template name (required)")
		startFs.Parse(args[1:])

		if *podID == "" || *userID == "" || *template == "" {
			fmt.Println(cli.T("labctl.session.hints.start", nil))
			os.Exit(1)
		}

		if err := sessionCmd.Start(ctx, *podID, *userID, *template); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "status":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.session.hints.status", nil))
			os.Exit(1)
		}
		if err := sessionCmd.Status(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "progress":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.session.hints.progress", nil))
			os.Exit(1)
		}
		if err := sessionCmd.Progress(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "checkpoints":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.session.hints.checkpoints", nil))
			os.Exit(1)
		}
		if err := sessionCmd.Checkpoints(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	case "end":
		if len(args) < 2 {
			fmt.Println(cli.T("labctl.session.hints.end", nil))
			os.Exit(1)
		}
		if err := sessionCmd.End(ctx, args[1]); err != nil {
			cli.PrintErr(err)
			os.Exit(1)
		}

	default:
		fmt.Println(cli.T("labctl.errors.unknownCommand", map[string]any{"Group": "session", "Command": args[0]}))
		fs.Usage()
		os.Exit(1)
	}
}
