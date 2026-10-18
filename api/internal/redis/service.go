package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Service provides access to all Redis functionality
type Service struct {
	Client       *Client
	Sessions     *SessionStore
	WebSocket    *WebSocketStore
	Cache        *Cache
	Templates    *TemplateCache
	Pods         *PodCache
	Users        *UserCache
	Achievements *AchievementCache
	Dashboard    DashboardCacheClient // Interface type for testability
	Pathways     *PathwayCache
	Permissions  *PermissionCache
	LTIState     LTIStateCacheClient // Interface type for testability
	Locks        *LockManager
	PodLocks     *PodLock
	RateLimiter  *RateLimiter
	APILimiter   *APIRateLimiter
	PodLimiter   *PodOperationRateLimiter
	AuthLimiter  *AuthRateLimiter
	TaskQueue    *TaskQueue
	PubSub       *PubSub
	Publisher    *EventPublisher
	Hub          *BroadcastHub

	config Config
	logger *slog.Logger
}

// ServiceConfig holds the configuration for the Redis service
type ServiceConfig struct {
	Redis      Config
	ServerNode string // Identifier for this server instance
	Logger     *slog.Logger
}

// NewService creates a new Redis service with all components initialized
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	client, err := New(cfg.Redis, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	// Initialize all components
	svc := &Service{
		Client:       client,
		Sessions:     NewSessionStore(client, DefaultSessionTTL),
		WebSocket:    NewWebSocketStore(client, 10*60), // 10 minutes
		Cache:        NewCache(client, DefaultCacheTTL, cfg.Logger),
		Templates:    NewTemplateCache(client, cfg.Logger),
		Pods:         NewPodCache(client, cfg.Logger),
		Users:        NewUserCache(client, cfg.Logger),
		Achievements: NewAchievementCache(client, cfg.Logger),
		Dashboard:    NewDashboardCache(client, cfg.Logger),
		Pathways:     NewPathwayCache(client, cfg.Logger),
		Permissions:  NewPermissionCache(client, cfg.Logger),
		LTIState:     NewLTIStateStore(client, cfg.Logger),
		Locks:        NewLockManager(client, cfg.Logger),
		PodLocks:     NewPodLock(client, cfg.Logger),
		RateLimiter:  NewRateLimiter(client, cfg.Logger),
		APILimiter:   NewAPIRateLimiter(client, cfg.Logger),
		PodLimiter:   NewPodOperationRateLimiter(client, cfg.Logger),
		AuthLimiter:  NewAuthRateLimiter(client, cfg.Logger),
		TaskQueue: NewTaskQueue(client, TaskQueueConfig{
			Name:   "labctl",
			Logger: cfg.Logger,
		}),
		PubSub: NewPubSub(client, cfg.Logger),
		config: cfg.Redis,
		logger: cfg.Logger,
	}

	// Create event publisher
	source := cfg.ServerNode
	if source == "" {
		source = "api"
	}
	svc.Publisher = NewEventPublisher(svc.PubSub, source)

	// Create broadcast hub for local WebSocket broadcasting
	svc.Hub = NewBroadcastHub(svc.PubSub, cfg.Logger)

	// Set up Redis pub/sub handlers for the broadcast hub
	svc.Hub.SetupRedisHandler(ChannelPodStatus)
	svc.Hub.SetupRedisHandler(ChannelCheckpointUpdate)
	svc.Hub.SetupRedisHandler(ChannelGradeSync)
	svc.Hub.SetupRedisHandler(ChannelSessionUpdate)

	return svc, nil
}

// Connect establishes the connection to Redis
func (s *Service) Connect(ctx context.Context) error {
	if err := s.Client.Connect(ctx); err != nil {
		return err
	}

	s.logger.Info("redis service connected",
		slog.String("host", s.config.Host),
		slog.Int("port", s.config.Port),
	)

	return nil
}

// Close closes all Redis connections
func (s *Service) Close() error {
	s.PubSub.Close()
	return s.Client.Close()
}

// HealthCheck performs a health check on the Redis service
func (s *Service) HealthCheck(ctx context.Context) error {
	return s.Client.HealthCheck(ctx)
}

// StartPubSub starts the pub/sub listener in a goroutine
func (s *Service) StartPubSub(ctx context.Context) {
	go func() {
		if err := s.PubSub.Start(ctx); err != nil {
			s.logger.Error("pub/sub listener stopped", slog.Any("error", err))
		}
	}()
}

// StartTaskWorker starts a task queue worker
func (s *Service) StartTaskWorker(ctx context.Context, handlers map[string]TaskHandler) {
	// Register handlers
	for taskType, handler := range handlers {
		s.TaskQueue.RegisterHandler(taskType, handler)
	}

	go func() {
		requeueTicker := time.NewTicker(5 * time.Second)
		defer requeueTicker.Stop()
		dequeueTicker := time.NewTicker(100 * time.Millisecond)
		defer dequeueTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-requeueTicker.C:
				// Requeue timed-out tasks
				if count, err := s.TaskQueue.RequeueTimedOut(ctx); err == nil && count > 0 {
					s.logger.Info("requeued timed-out tasks", slog.Int("count", count))
				}
			case <-dequeueTicker.C:
				task, err := s.TaskQueue.Dequeue(ctx)
				if err != nil {
					s.logger.Error("failed to dequeue task", slog.Any("error", err))
					continue
				}

				if task == nil {
					continue
				}

				handler, ok := handlers[task.Type]
				if !ok {
					s.logger.Warn("no handler for task type", slog.String("type", task.Type))
					if failErr := s.TaskQueue.Fail(ctx, task.ID, fmt.Errorf("no handler for task type: %s", task.Type)); failErr != nil {
						s.logger.Error("failed to mark task as failed", slog.String("taskID", task.ID), slog.Any("error", failErr))
					}
					continue
				}

				if err := handler(ctx, task); err != nil {
					if failErr := s.TaskQueue.Fail(ctx, task.ID, err); failErr != nil {
						s.logger.Error("failed to mark task as failed", slog.String("taskID", task.ID), slog.Any("error", failErr))
					}
				} else {
					if completeErr := s.TaskQueue.Complete(ctx, task.ID, nil); completeErr != nil {
						s.logger.Error("failed to mark task as complete", slog.String("taskID", task.ID), slog.Any("error", completeErr))
					}
				}
			}
		}
	}()

	s.logger.Info("task worker started")
}

// Stats returns service statistics
func (s *Service) Stats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// Task queue stats
	taskStats, err := s.TaskQueue.Stats(ctx)
	if err == nil {
		stats["task_queue"] = taskStats
	}

	// Memory info
	info, err := s.Client.rdb.Info(ctx, "memory", "clients").Result()
	if err == nil {
		stats["redis_info"] = info[:min(len(info), 500)]
	}

	return stats, nil
}
