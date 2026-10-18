// Package jobs provides background job processing for the lab platform
package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/nats"
)

// WorkerPool manages background job workers
type WorkerPool struct {
	natsClient         *nats.Client
	achievementService *achievements.Service
	sessionRepo        repositories.SessionRepository
	eventRepo          repositories.EventRepository
	checkpointEval     *checkpoint.Evaluator
	logger             *slog.Logger

	consumers []*nats.Consumer
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc

	// Stats tracking
	mu             sync.RWMutex
	jobsProcessed  int64
	jobsFailed     int64
	totalLatencyMs int64
}

// WorkerConfig configures the worker pool
type WorkerConfig struct {
	AchievementWorkers int // Number of achievement evaluation workers
	EventWorkers       int // Number of event processing workers
}

// DefaultWorkerConfig returns default worker configuration
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		AchievementWorkers: 2,
		EventWorkers:       2,
	}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	natsClient *nats.Client,
	achievementService *achievements.Service,
	sessionRepo repositories.SessionRepository,
	logger *slog.Logger,
) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		natsClient:         natsClient,
		achievementService: achievementService,
		sessionRepo:        sessionRepo,
		logger:             logger.With("component", "worker-pool"),
		consumers:          make([]*nats.Consumer, 0),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// WithEventRepo adds an event repository to the worker pool
func (wp *WorkerPool) WithEventRepo(repo repositories.EventRepository) *WorkerPool {
	wp.eventRepo = repo
	return wp
}

// WithCheckpointEvaluator adds a checkpoint evaluator to the worker pool
func (wp *WorkerPool) WithCheckpointEvaluator(eval *checkpoint.Evaluator) *WorkerPool {
	wp.checkpointEval = eval
	return wp
}

// Start starts all workers
func (wp *WorkerPool) Start(cfg WorkerConfig) error {
	wp.logger.Info("Starting worker pool",
		"achievementWorkers", cfg.AchievementWorkers,
		"eventWorkers", cfg.EventWorkers,
	)

	// Check if NATS client is available - workers require NATS
	if wp.natsClient == nil {
		wp.logger.Info("NATS client not available, skipping worker startup")
		return nil
	}

	// Start achievement workers
	if cfg.AchievementWorkers > 0 && wp.achievementService != nil {
		consumer, err := wp.startAchievementWorker()
		if err != nil {
			return err
		}
		wp.consumers = append(wp.consumers, consumer)
	}

	// Start event processing workers
	if cfg.EventWorkers > 0 && wp.eventRepo != nil {
		consumer, err := wp.startEventWorker()
		if err != nil {
			return err
		}
		wp.consumers = append(wp.consumers, consumer)
	}

	wp.logger.Info("Worker pool started")
	return nil
}

// Stop gracefully stops all workers
func (wp *WorkerPool) Stop() {
	wp.logger.Info("Stopping worker pool...")
	wp.cancel()

	for _, consumer := range wp.consumers {
		consumer.Stop()
	}

	wp.wg.Wait()
	wp.logger.Info("Worker pool stopped")
}

// startAchievementWorker starts the achievement evaluation worker
func (wp *WorkerPool) startAchievementWorker() (*nats.Consumer, error) {
	handler := func(ctx context.Context, msg jetstream.Msg) error {
		return wp.handleAchievementJob(ctx, msg)
	}

	consumer, err := nats.NewConsumer(wp.natsClient, nats.ConsumerConfig{
		Name:          events.ConsumerAchievementWorker,
		Durable:       true,
		FilterSubject: events.SubjectJobsAchievements,
		MaxDeliver:    5,
	}, handler, wp.logger)
	if err != nil {
		return nil, err
	}

	if err := consumer.Start(wp.ctx); err != nil {
		return nil, err
	}

	wp.logger.Info("Achievement worker started")
	return consumer, nil
}

// handleAchievementJob processes an achievement evaluation job
func (wp *WorkerPool) handleAchievementJob(ctx context.Context, msg jetstream.Msg) error {
	start := time.Now()

	job, err := events.ParseAchievementJob(msg.Data())
	if err != nil {
		wp.logger.Error("Failed to parse achievement job", "error", err)
		return err
	}

	wp.logger.Debug("Processing achievement job",
		"jobId", job.ID,
		"sessionId", job.SessionID,
		"userId", job.UserID,
	)

	// Get the session
	session, err := wp.sessionRepo.GetByID(ctx, job.SessionID)
	if err != nil {
		wp.logger.Error("Failed to get session for achievement evaluation",
			"error", err,
			"sessionId", job.SessionID,
		)
		return err
	}

	if session == nil {
		wp.logger.Warn("Session not found for achievement evaluation",
			"sessionId", job.SessionID,
		)
		return nil // Don't retry - session doesn't exist
	}

	// Evaluate achievements
	awarded, err := wp.achievementService.CheckAndAwardAchievements(ctx, session, job.UserID)
	if err != nil {
		wp.logger.Error("Achievement evaluation failed",
			"error", err,
			"sessionId", job.SessionID,
			"userId", job.UserID,
		)
		return err
	}

	duration := time.Since(start)
	wp.logger.Info("Achievement job completed",
		"jobId", job.ID,
		"sessionId", job.SessionID,
		"userId", job.UserID,
		"achievementsAwarded", len(awarded),
		"durationMs", duration.Milliseconds(),
	)

	// Log awarded achievements
	for _, a := range awarded {
		wp.logger.Info("Achievement awarded via background job",
			"userId", job.UserID,
			"achievementId", a.AchievementID,
			"achievementName", a.Achievement.Name,
		)
	}

	return nil
}

// startEventWorker starts the event processing worker
func (wp *WorkerPool) startEventWorker() (*nats.Consumer, error) {
	handler := func(ctx context.Context, msg jetstream.Msg) error {
		return wp.handleEventJob(ctx, msg)
	}

	consumer, err := nats.NewConsumer(wp.natsClient, nats.ConsumerConfig{
		Name:          events.ConsumerEventProcessor,
		Durable:       true,
		FilterSubject: events.SubjectJobsEvents,
		MaxDeliver:    5,
	}, handler, wp.logger)
	if err != nil {
		return nil, err
	}

	if err := consumer.Start(wp.ctx); err != nil {
		return nil, err
	}

	wp.logger.Info("Event processing worker started")
	return consumer, nil
}

// handleEventJob processes an event processing job
func (wp *WorkerPool) handleEventJob(ctx context.Context, msg jetstream.Msg) error {
	start := time.Now()

	job, err := events.ParseEventProcessingJob(msg.Data())
	if err != nil {
		wp.logger.Error("Failed to parse event job", "error", err)
		wp.recordJobFailure()
		return err
	}

	wp.logger.Debug("Processing event job",
		"jobId", job.ID,
		"podId", job.PodID,
		"sessionId", job.SessionID,
		"eventCount", len(job.EventIDs),
	)

	var processedCount int
	var errCount int

	// Process events by ID if specified
	if len(job.EventIDs) > 0 {
		for _, eventID := range job.EventIDs {
			event, err := wp.eventRepo.GetByID(ctx, eventID)
			if err != nil {
				wp.logger.Error("Failed to get event", "error", err, "eventId", eventID)
				errCount++
				continue
			}
			if event == nil {
				continue
			}

			// If we have a checkpoint evaluator and session ID, evaluate the event
			if wp.checkpointEval != nil && job.SessionID != "" {
				vmEvent := &events.VMEvent{
					PodID:     job.PodID,
					SessionID: job.SessionID,
					VMName:    event.VMName,
					EventType: event.EventType,
					Data:      event.Data,
				}
				if _, err := wp.checkpointEval.EvaluateEvent(ctx, vmEvent, job.SessionID); err != nil {
					wp.logger.Error("Failed to evaluate event", "error", err, "eventId", eventID)
					errCount++
					continue
				}
			}

			// Mark event as processed (with empty matched checkpoints - will be filled by evaluator callback)
			if err := wp.eventRepo.MarkProcessed(ctx, eventID, nil); err != nil {
				wp.logger.Error("Failed to mark event processed", "error", err, "eventId", eventID)
				errCount++
				continue
			}

			processedCount++
		}
	} else {
		// Process unprocessed events (global query with limit)
		unprocessed, err := wp.eventRepo.GetUnprocessed(ctx, job.BatchSize)
		if err != nil {
			wp.logger.Error("Failed to get unprocessed events", "error", err)
			wp.recordJobFailure()
			return err
		}

		// Filter by pod if specified
		for _, event := range unprocessed {
			if job.PodID != "" && event.PodID != job.PodID {
				continue
			}

			// If we have a checkpoint evaluator and session ID, evaluate
			if wp.checkpointEval != nil && job.SessionID != "" {
				vmEvent := &events.VMEvent{
					PodID:     job.PodID,
					SessionID: job.SessionID,
					VMName:    event.VMName,
					EventType: event.EventType,
					Data:      event.Data,
				}
				if _, err := wp.checkpointEval.EvaluateEvent(ctx, vmEvent, job.SessionID); err != nil {
					wp.logger.Error("Failed to evaluate event", "error", err, "eventId", event.ID)
					errCount++
					continue
				}
			}

			// Mark as processed
			if err := wp.eventRepo.MarkProcessed(ctx, event.ID, nil); err != nil {
				wp.logger.Error("Failed to mark event processed", "error", err, "eventId", event.ID)
				errCount++
				continue
			}

			processedCount++
		}
	}

	duration := time.Since(start)
	wp.recordJobSuccess(duration)

	wp.logger.Info("Event job completed",
		"jobId", job.ID,
		"podId", job.PodID,
		"eventsProcessed", processedCount,
		"errors", errCount,
		"durationMs", duration.Milliseconds(),
	)

	return nil
}

// recordJobSuccess records a successful job execution
func (wp *WorkerPool) recordJobSuccess(duration time.Duration) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.jobsProcessed++
	wp.totalLatencyMs += duration.Milliseconds()
}

// recordJobFailure records a failed job execution
func (wp *WorkerPool) recordJobFailure() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.jobsFailed++
}

// Stats returns worker pool statistics
type WorkerStats struct {
	Running          bool  `json:"running"`
	ConsumersActive  int   `json:"consumersActive"`
	JobsProcessed    int64 `json:"jobsProcessed"`
	JobsFailed       int64 `json:"jobsFailed"`
	AverageLatencyMs int64 `json:"averageLatencyMs"`
}

// Stats returns current worker pool statistics
func (wp *WorkerPool) Stats() WorkerStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	var avgLatency int64
	if wp.jobsProcessed > 0 {
		avgLatency = wp.totalLatencyMs / wp.jobsProcessed
	}

	return WorkerStats{
		Running:          wp.ctx.Err() == nil,
		ConsumersActive:  len(wp.consumers),
		JobsProcessed:    wp.jobsProcessed,
		JobsFailed:       wp.jobsFailed,
		AverageLatencyMs: avgLatency,
	}
}
