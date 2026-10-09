# Async Job Processing Architecture

This document describes the asynchronous job processing system used for background tasks like achievement evaluation and event processing.

## Overview

The Kootenai platform uses NATS JetStream for reliable, persistent message queuing. Background jobs are processed by a worker pool that consumes messages from dedicated job streams.

## Components

### Job Messages

Job messages are published to NATS JetStream subjects and consumed by dedicated workers.

```
labs.jobs.achievements  - Achievement evaluation jobs
labs.jobs.gradesync     - Canvas LMS grade sync jobs
labs.jobs.events        - Event batch processing jobs
```

### Worker Pool

The `WorkerPool` (`api/internal/jobs/worker.go`) manages background workers:

```go
// Create worker pool
wp := jobs.NewWorkerPool(natsClient, achievementService, sessionRepo, logger).
    WithEventRepo(eventRepo).
    WithCheckpointEvaluator(checkpointEval)

// Start workers
wp.Start(jobs.DefaultWorkerConfig())

// Graceful shutdown
wp.Stop()
```

### Job Types

#### AchievementEvaluationJob

Triggered when a session is submitted successfully. Evaluates all achievement criteria for the user.

```go
type AchievementEvaluationJob struct {
    SessionID     string
    UserID        string
    LabTemplateID string
    Passed        bool
    Percentage    float64
    EarnedPoints  int
    MaxPoints     int
    EnrollmentID  *string  // Optional pathway context
    ModuleID      *string
    Priority      JobPriority
    MaxRetry      int
}
```

#### GradeSyncJob

Triggered when grades need to be synced to Canvas LMS.

```go
type GradeSyncJob struct {
    SessionID          string
    UserID             string
    EarnedPoints       int
    MaxPoints          int
    Percentage         float64
    CanvasAssignmentID string
    CanvasCourseID     string
    CanvasUserID       string
    Priority           JobPriority
    MaxRetry           int
}
```

#### EventProcessingJob

Triggered for batch event processing (checkpoint evaluation).

```go
type EventProcessingJob struct {
    PodID     string
    SessionID string
    EventIDs  []int64   // Specific events to process
    BatchSize int       // For bulk processing
    Priority  JobPriority
    MaxRetry  int
}
```

## Flow Diagrams

### Achievement Evaluation Flow

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐     ┌────────────┐
│   Session   │────▶│  Publish Job │────▶│  NATS JetStream │────▶│   Worker   │
│   Submit    │     │  (async)     │     │  labs.jobs.*    │     │   Pool     │
└─────────────┘     └──────────────┘     └─────────────────┘     └────────────┘
       │                                                               │
       │ Response with                                                 │
       │ achievementsPending: true                                     ▼
       │                                          ┌────────────────────────────┐
       │                                          │  Achievement Service       │
       │                                          │  - Check criteria          │
       │                                          │  - Award achievements      │
       │                                          │  - Update progress         │
       │                                          └────────────────────────────┘
       ▼
┌─────────────┐
│   Client    │  Poll /users/`{id}`/achievements
│   (async)   │  or WebSocket notification
└─────────────┘
```

### Fallback Behavior

When NATS is unavailable, the system falls back to synchronous processing:

```go
if natsClient != nil && natsClient.IsConnected() {
    // Queue async job
    natsClient.PublishAchievementJob(ctx, job)
    response["achievementsPending"] = true
} else {
    // Synchronous evaluation
    achievements := achievementService.CheckAndAwardAchievements(ctx, session, userID)
    response["achievements"] = achievements
}
```

## Configuration

### Worker Configuration

```go
type WorkerConfig struct {
    AchievementWorkers int  // Default: 2
    EventWorkers       int  // Default: 2
}
```

### NATS JetStream Stream

The `LABS` stream includes job subjects:

```go
streamCfg := jetstream.StreamConfig{
    Name:        "LABS",
    Subjects: []string{
        "labs.events.>",
        "labs.checkpoints.>",
        "labs.sessions.>",
        "labs.grades.>",
        "labs.pods.>",
        "labs.jobs.>",  // Job messages
    },
    Retention: jetstream.LimitsPolicy,
    MaxAge:    7 * 24 * time.Hour,
}
```

### Consumer Configuration

```go
ConsumerConfig{
    Name:          "achievement-worker",
    Durable:       true,
    FilterSubject: "labs.jobs.achievements",
    MaxDeliver:    5,  // Retry up to 5 times
}
```

## Error Handling

### Retry Logic

- Jobs are automatically retried on failure (NAK)
- `MaxDeliver` controls retry limit per job type
- Achievement jobs: 5 retries
- Grade sync jobs: 10 retries (external API)
- Event processing: 5 retries

### Dead Letter Handling

Failed jobs after max retries are logged but not re-queued. Monitor logs for:

```
level=ERROR msg="Achievement evaluation failed" error="..." sessionId="..."
```

## Monitoring

### Worker Stats

```go
stats := workerPool.Stats()
// Returns:
type WorkerStats struct {
    Running          bool
    ConsumersActive  int
    JobsProcessed    int64
    JobsFailed       int64
    AverageLatencyMs int64
}
```

### Health Check

The `/ready` endpoint includes NATS status:

```json
{
  "checks": {
    "nats": { "status": "healthy" }
  }
}
```

## Best Practices

1. **Idempotency**: Jobs should be safe to retry. Achievement evaluation checks if already earned before awarding.

2. **Graceful Degradation**: Always implement sync fallback when NATS is unavailable.

3. **Monitoring**: Track `JobsFailed` metric and alert on high failure rates.

4. **Testing**: Use `TestWorkerPoolStartWithoutNATS` pattern for testing without NATS dependency.

## Related Files

- `api/internal/jobs/worker.go` - Worker pool implementation
- `api/internal/events/messages.go` - Job message types
- `api/internal/events/subjects.go` - NATS subjects and consumers
- `api/internal/nats/client.go` - NATS client with job publishing
- `api/internal/nats/consumer.go` - Consumer factories
