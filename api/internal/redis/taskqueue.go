package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// TaskState represents the state of a task
type TaskState string

const (
	TaskStatePending    TaskState = "pending"
	TaskStateProcessing TaskState = "processing"
	TaskStateCompleted  TaskState = "completed"
	TaskStateFailed     TaskState = "failed"
	TaskStateRetrying   TaskState = "retrying"
)

// Task represents a background job
type Task struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	State       TaskState       `json:"state"`
	Priority    int             `json:"priority"`
	MaxRetries  int             `json:"max_retries"`
	RetryCount  int             `json:"retry_count"`
	CreatedAt   time.Time       `json:"created_at"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Error       string          `json:"error,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

// TaskHandler is a function that processes a task
type TaskHandler func(ctx context.Context, task *Task) error

// TaskQueue provides a Redis-backed task queue
type TaskQueue struct {
	client     *Client
	name       string
	handlers   map[string]TaskHandler
	logger     *slog.Logger
	visibility time.Duration
}

// TaskQueueConfig holds task queue configuration
type TaskQueueConfig struct {
	Name           string
	VisibilityTime time.Duration
	Logger         *slog.Logger
}

// NewTaskQueue creates a new task queue
func NewTaskQueue(client *Client, cfg TaskQueueConfig) *TaskQueue {
	if cfg.Name == "" {
		cfg.Name = "default"
	}
	if cfg.VisibilityTime == 0 {
		cfg.VisibilityTime = DefaultTaskVisibility
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &TaskQueue{
		client:     client,
		name:       cfg.Name,
		handlers:   make(map[string]TaskHandler),
		logger:     cfg.Logger,
		visibility: cfg.VisibilityTime,
	}
}

// key helpers
func (tq *TaskQueue) pendingKey() string {
	return PrefixTaskQueue + tq.name + ":pending"
}

func (tq *TaskQueue) processingKey() string {
	return PrefixTaskQueue + tq.name + ":processing"
}

func (tq *TaskQueue) taskKey(taskID string) string {
	return PrefixTask + tq.name + ":" + taskID
}

func (tq *TaskQueue) scheduledKey() string {
	return PrefixTaskQueue + tq.name + ":scheduled"
}

func (tq *TaskQueue) deadLetterKey() string {
	return PrefixTaskQueue + tq.name + ":dead"
}

// RegisterHandler registers a handler for a task type
func (tq *TaskQueue) RegisterHandler(taskType string, handler TaskHandler) {
	tq.handlers[taskType] = handler
}

// Enqueue adds a task to the queue
func (tq *TaskQueue) Enqueue(ctx context.Context, taskType string, payload any) (*Task, error) {
	return tq.EnqueueWithOptions(ctx, taskType, payload, EnqueueOptions{})
}

// EnqueueOptions provides options for task enqueueing
type EnqueueOptions struct {
	ID          string
	Priority    int           // Higher = processed first
	MaxRetries  int           // Default: 3
	Delay       time.Duration // Delay before processing
	ScheduledAt time.Time     // Specific time to process
	Metadata    map[string]any
}

// EnqueueWithOptions adds a task with custom options
func (tq *TaskQueue) EnqueueWithOptions(ctx context.Context, taskType string, payload any, opts EnqueueOptions) (*Task, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	taskID := opts.ID
	if taskID == "" {
		taskID = uuid.New().String()
	}

	maxRetries := opts.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	now := time.Now()
	scheduledAt := now
	if opts.Delay > 0 {
		scheduledAt = now.Add(opts.Delay)
	} else if !opts.ScheduledAt.IsZero() {
		scheduledAt = opts.ScheduledAt
	}

	task := &Task{
		ID:          taskID,
		Type:        taskType,
		Payload:     payloadJSON,
		State:       TaskStatePending,
		Priority:    opts.Priority,
		MaxRetries:  maxRetries,
		RetryCount:  0,
		CreatedAt:   now,
		ScheduledAt: scheduledAt,
		Metadata:    opts.Metadata,
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task: %w", err)
	}

	// Store task data
	if err := tq.client.Set(ctx, tq.taskKey(taskID), taskJSON, 7*24*time.Hour); err != nil {
		return nil, fmt.Errorf("failed to store task: %w", err)
	}

	// Add to appropriate queue based on scheduling
	if scheduledAt.After(now) {
		// Delayed task - add to scheduled set with score as timestamp
		if err := tq.client.rdb.ZAdd(ctx, tq.client.Key(tq.scheduledKey()), redis.Z{
			Score:  float64(scheduledAt.UnixMilli()),
			Member: taskID,
		}).Err(); err != nil {
			return nil, fmt.Errorf("failed to schedule task: %w", err)
		}
	} else {
		// Immediate task - add to pending list with priority score
		score := float64(opts.Priority)*1e15 - float64(now.UnixMilli())
		if err := tq.client.rdb.ZAdd(ctx, tq.client.Key(tq.pendingKey()), redis.Z{
			Score:  score,
			Member: taskID,
		}).Err(); err != nil {
			return nil, fmt.Errorf("failed to enqueue task: %w", err)
		}
	}

	tq.logger.Info("task enqueued",
		slog.String("task_id", taskID),
		slog.String("type", taskType),
		slog.Int("priority", opts.Priority),
	)

	return task, nil
}

// Dequeue retrieves the next task from the queue
func (tq *TaskQueue) Dequeue(ctx context.Context) (*Task, error) {
	// First, move any due scheduled tasks to pending
	if err := tq.moveScheduledTasks(ctx); err != nil {
		tq.logger.Warn("failed to move scheduled tasks", slog.Any("error", err))
	}

	// Get the highest priority task
	results, err := tq.client.rdb.ZPopMax(ctx, tq.client.Key(tq.pendingKey()), 1).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	taskID, _ := results[0].Member.(string)

	// Get task data
	taskJSON, err := tq.client.GetBytes(ctx, tq.taskKey(taskID))
	if err != nil {
		return nil, fmt.Errorf("failed to get task data: %w", err)
	}

	var task Task
	if err := json.Unmarshal(taskJSON, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	// Mark as processing
	now := time.Now()
	task.State = TaskStateProcessing
	task.StartedAt = &now

	taskJSON, err = json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task state: %w", err)
	}
	if err := tq.client.Set(ctx, tq.taskKey(taskID), taskJSON, 7*24*time.Hour); err != nil {
		tq.logger.Error("failed to persist task processing state", slog.String("task_id", taskID), slog.Any("error", err))
		return nil, fmt.Errorf("persist task state: %w", err)
	}

	// Add to processing set with visibility timeout
	visibilityTime := float64(now.Add(tq.visibility).UnixMilli())
	if err := tq.client.rdb.ZAdd(ctx, tq.client.Key(tq.processingKey()), redis.Z{
		Score:  visibilityTime,
		Member: taskID,
	}).Err(); err != nil {
		tq.logger.Warn("failed to add task to processing set", slog.String("task_id", taskID), slog.Any("error", err))
	}

	return &task, nil
}

// moveScheduledTasks moves due scheduled tasks to the pending queue
func (tq *TaskQueue) moveScheduledTasks(ctx context.Context) error {
	now := time.Now().UnixMilli()

	// Get tasks that are due
	results, err := tq.client.rdb.ZRangeByScore(ctx, tq.client.Key(tq.scheduledKey()), &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now),
		Count: 100,
	}).Result()
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return nil
	}

	// Move each task to pending
	for _, taskID := range results {
		// Get task to retrieve priority
		taskJSON, err := tq.client.GetBytes(ctx, tq.taskKey(taskID))
		if err != nil {
			continue
		}

		var task Task
		if err := json.Unmarshal(taskJSON, &task); err != nil {
			continue
		}

		// Add to pending with priority
		score := float64(task.Priority)*1e15 - float64(now)
		if err := tq.client.rdb.ZAdd(ctx, tq.client.Key(tq.pendingKey()), redis.Z{
			Score:  score,
			Member: taskID,
		}).Err(); err != nil {
			tq.logger.Warn("failed to move task to pending", slog.String("task_id", taskID), slog.Any("error", err))
		}

		// Remove from scheduled
		if err := tq.client.rdb.ZRem(ctx, tq.client.Key(tq.scheduledKey()), taskID).Err(); err != nil {
			tq.logger.Warn("failed to remove task from scheduled", slog.String("task_id", taskID), slog.Any("error", err))
		}
	}

	return nil
}

// Complete marks a task as completed
func (tq *TaskQueue) Complete(ctx context.Context, taskID string, result any) error {
	taskJSON, err := tq.client.GetBytes(ctx, tq.taskKey(taskID))
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	var task Task
	if err := json.Unmarshal(taskJSON, &task); err != nil {
		return err
	}

	now := time.Now()
	task.State = TaskStateCompleted
	task.CompletedAt = &now

	if result != nil {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			tq.logger.Warn("failed to marshal task result", slog.String("task_id", taskID), slog.Any("error", err))
		} else {
			task.Result = resultJSON
		}
	}

	taskJSON, err = json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal completed task: %w", err)
	}
	if err := tq.client.Set(ctx, tq.taskKey(taskID), taskJSON, 24*time.Hour); err != nil {
		tq.logger.Error("failed to persist completed task state", slog.String("task_id", taskID), slog.Any("error", err))
	}

	// Remove from processing
	if err := tq.client.rdb.ZRem(ctx, tq.client.Key(tq.processingKey()), taskID).Err(); err != nil {
		tq.logger.Warn("failed to remove task from processing", slog.String("task_id", taskID), slog.Any("error", err))
	}

	tq.logger.Info("task completed",
		slog.String("task_id", taskID),
		slog.String("type", task.Type),
	)

	return nil
}

// Fail marks a task as failed and optionally retries
func (tq *TaskQueue) Fail(ctx context.Context, taskID string, taskErr error) error {
	taskJSON, err := tq.client.GetBytes(ctx, tq.taskKey(taskID))
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	var task Task
	if err := json.Unmarshal(taskJSON, &task); err != nil {
		return err
	}

	task.RetryCount++
	task.Error = taskErr.Error()

	// Remove from processing
	if err := tq.client.rdb.ZRem(ctx, tq.client.Key(tq.processingKey()), taskID).Err(); err != nil {
		tq.logger.Warn("failed to remove failed task from processing", slog.String("task_id", taskID), slog.Any("error", err))
	}

	if task.RetryCount < task.MaxRetries {
		// Retry with exponential backoff
		task.State = TaskStateRetrying
		delay := time.Duration(task.RetryCount*task.RetryCount) * time.Second
		task.ScheduledAt = time.Now().Add(delay)

		if taskJSON, err = json.Marshal(task); err != nil {
			return fmt.Errorf("marshal retrying task: %w", err)
		}
		if err := tq.client.Set(ctx, tq.taskKey(taskID), taskJSON, 7*24*time.Hour); err != nil {
			tq.logger.Error("failed to persist retrying task state", slog.String("task_id", taskID), slog.Any("error", err))
		}

		// Add to scheduled queue
		if err := tq.client.rdb.ZAdd(ctx, tq.client.Key(tq.scheduledKey()), redis.Z{
			Score:  float64(task.ScheduledAt.UnixMilli()),
			Member: taskID,
		}).Err(); err != nil {
			tq.logger.Warn("failed to add task to scheduled queue", slog.String("task_id", taskID), slog.Any("error", err))
		}

		tq.logger.Warn("task retrying",
			slog.String("task_id", taskID),
			slog.Int("retry_count", task.RetryCount),
			slog.Duration("delay", delay),
			slog.String("error", taskErr.Error()),
		)
	} else {
		// Max retries exceeded - move to dead letter queue
		task.State = TaskStateFailed

		if taskJSON, err = json.Marshal(task); err != nil {
			return fmt.Errorf("marshal failed task: %w", err)
		}
		if err := tq.client.Set(ctx, tq.taskKey(taskID), taskJSON, 7*24*time.Hour); err != nil {
			tq.logger.Error("failed to persist failed task state", slog.String("task_id", taskID), slog.Any("error", err))
		}

		if err := tq.client.rdb.LPush(ctx, tq.client.Key(tq.deadLetterKey()), taskID).Err(); err != nil {
			tq.logger.Warn("failed to add task to dead letter queue", slog.String("task_id", taskID), slog.Any("error", err))
		}

		tq.logger.Error("task failed permanently",
			slog.String("task_id", taskID),
			slog.String("type", task.Type),
			slog.String("error", taskErr.Error()),
		)
	}

	return nil
}

// GetTask retrieves task info by ID
func (tq *TaskQueue) GetTask(ctx context.Context, taskID string) (*Task, error) {
	taskJSON, err := tq.client.GetBytes(ctx, tq.taskKey(taskID))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(taskJSON, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

// RequeueTimedOut moves tasks that have exceeded visibility timeout back to pending
func (tq *TaskQueue) RequeueTimedOut(ctx context.Context) (int, error) {
	now := time.Now().UnixMilli()

	// Get timed out tasks
	results, err := tq.client.rdb.ZRangeByScore(ctx, tq.client.Key(tq.processingKey()), &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now),
		Count: 100,
	}).Result()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, taskID := range results {
		// Get task to mark as pending again
		taskJSON, err := tq.client.GetBytes(ctx, tq.taskKey(taskID))
		if err != nil {
			continue
		}

		var task Task
		if err := json.Unmarshal(taskJSON, &task); err != nil {
			continue
		}

		task.State = TaskStatePending
		task.StartedAt = nil

		taskJSON, err = json.Marshal(task)
		if err != nil {
			tq.logger.Warn("failed to marshal task during requeue",
				slog.String("task_id", taskID), slog.Any("error", err))
			continue
		}
		if err := tq.client.Set(ctx, tq.taskKey(taskID), taskJSON, 7*24*time.Hour); err != nil {
			tq.logger.Warn("failed to persist requeued task",
				slog.String("task_id", taskID), slog.Any("error", err))
			continue
		}

		// Add back to pending
		score := float64(task.Priority)*1e15 - float64(now)
		if err := tq.client.rdb.ZAdd(ctx, tq.client.Key(tq.pendingKey()), redis.Z{
			Score:  score,
			Member: taskID,
		}).Err(); err != nil {
			tq.logger.Warn("failed to re-add task to pending queue",
				slog.String("task_id", taskID), slog.Any("error", err))
			continue
		}

		// Remove from processing
		if err := tq.client.rdb.ZRem(ctx, tq.client.Key(tq.processingKey()), taskID).Err(); err != nil {
			tq.logger.Warn("failed to remove task from processing queue",
				slog.String("task_id", taskID), slog.Any("error", err))
		}

		count++
		tq.logger.Warn("task requeued after timeout",
			slog.String("task_id", taskID),
			slog.String("type", task.Type),
		)
	}

	return count, nil
}

// Stats returns queue statistics
func (tq *TaskQueue) Stats(ctx context.Context) (map[string]int64, error) {
	stats := make(map[string]int64)

	pending, _ := tq.client.rdb.ZCard(ctx, tq.client.Key(tq.pendingKey())).Result()
	processing, _ := tq.client.rdb.ZCard(ctx, tq.client.Key(tq.processingKey())).Result()
	scheduled, _ := tq.client.rdb.ZCard(ctx, tq.client.Key(tq.scheduledKey())).Result()
	dead, _ := tq.client.rdb.LLen(ctx, tq.client.Key(tq.deadLetterKey())).Result()

	stats["pending"] = pending
	stats["processing"] = processing
	stats["scheduled"] = scheduled
	stats["dead_letter"] = dead

	return stats, nil
}

// --- Predefined Task Types ---

const (
	TaskTypeGradeSync          = "grade_sync"
	TaskTypePodCleanup         = "pod_cleanup"
	TaskTypePodExpiration      = "pod_expiration"
	TaskTypeSnapshotCreate     = "snapshot_create"
	TaskTypeSnapshotRevert     = "snapshot_revert"
	TaskTypeAssessmentRun      = "assessment_run"
	TaskTypeEventProcess       = "event_process"
	TaskTypeNotificationSend   = "notification_send"
	TaskTypeAuditLogPersist    = "audit_log_persist"
	TaskTypeCheckpointEvaluate = "checkpoint_evaluate"
)

// GradeSyncPayload is the payload for grade sync tasks
type GradeSyncPayload struct {
	SessionID    string  `json:"session_id"`
	Score        float64 `json:"score"`
	CanvasUserID string  `json:"canvas_user_id"`
	AssignmentID string  `json:"assignment_id"`
	CourseID     string  `json:"course_id"`
}

// PodCleanupPayload is the payload for pod cleanup tasks
type PodCleanupPayload struct {
	PodID  string `json:"pod_id"`
	Reason string `json:"reason"`
}

// PodExpirationPayload is the payload for pod expiration tasks
type PodExpirationPayload struct {
	PodID     string    `json:"pod_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SnapshotPayload is the payload for snapshot operations
type SnapshotPayload struct {
	PodID        string `json:"pod_id"`
	SnapshotName string `json:"snapshot_name"`
	VMID         string `json:"vm_id,omitempty"`
}

// AssessmentRunPayload is the payload for assessment run tasks
type AssessmentRunPayload struct {
	SessionID    string `json:"session_id"`
	CheckpointID string `json:"checkpoint_id"`
	TriggerType  string `json:"trigger_type"`
}
