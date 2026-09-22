// Package nats provides NATS JetStream client functionality for the lab platform
package nats

import (
	"context"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

// EventPublisher defines the interface for publishing events to NATS.
// This interface enables dependency injection and testing of components
// that publish events without requiring a live NATS connection.
type EventPublisher interface {
	// Publish publishes raw data to a subject
	Publish(ctx context.Context, subject string, data []byte) error
	// PublishEvent publishes a VM event
	PublishEvent(ctx context.Context, event *events.VMEvent) error
	// PublishCheckpoint publishes a checkpoint update
	PublishCheckpoint(ctx context.Context, update *events.CheckpointUpdate) error
	// PublishSession publishes a session event
	PublishSession(ctx context.Context, event *events.SessionEvent) error
	// PublishGrade publishes a grade update
	PublishGrade(ctx context.Context, update *events.GradeUpdate) error
	// PublishAchievementJob publishes an achievement evaluation job
	PublishAchievementJob(ctx context.Context, job *events.AchievementEvaluationJob) error
	// PublishGradeSyncJob publishes a grade sync job
	PublishGradeSyncJob(ctx context.Context, job *events.GradeSyncJob) error
	// PublishEventProcessingJob publishes an event processing job
	PublishEventProcessingJob(ctx context.Context, job *events.EventProcessingJob) error
}

// ConnectionChecker defines the interface for checking NATS connection status.
// Used by health check endpoints.
type ConnectionChecker interface {
	// IsConnected returns true if connected to NATS
	IsConnected() bool
}

// ClientInterface combines all NATS client capabilities.
// This is the main interface for dependency injection in services.
type ClientInterface interface {
	EventPublisher
	ConnectionChecker
	// Close closes the NATS connection
	Close()
}

// Compile-time check that Client implements ClientInterface
var _ ClientInterface = (*Client)(nil)
