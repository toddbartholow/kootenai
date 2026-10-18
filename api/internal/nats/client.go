// Package nats provides NATS JetStream client functionality for the lab platform
package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

// Client wraps NATS connection and JetStream context
type Client struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
	logger *slog.Logger
}

// Config holds NATS connection configuration
type Config struct {
	URL            string        `yaml:"url"`
	Name           string        `yaml:"name"`
	Token          string        `yaml:"token,omitempty"`
	User           string        `yaml:"user,omitempty"`
	Password       string        `yaml:"password,omitempty"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	ReconnectWait  time.Duration `yaml:"reconnect_wait"`
	MaxReconnects  int           `yaml:"max_reconnects"`
}

// DefaultConfig returns a default NATS configuration
func DefaultConfig() Config {
	return Config{
		URL:            "nats://localhost:4222",
		Name:           "lab-platform",
		ConnectTimeout: 10 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  -1, // Unlimited
	}
}

// NewClient creates a new NATS client with JetStream
func NewClient(cfg Config, logger *slog.Logger) (*Client, error) {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.Timeout(cfg.ConnectTimeout),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error", "error", err, "subject", sub.Subject)
		}),
	}

	// Add authentication if configured
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	} else if cfg.User != "" && cfg.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	// Connect to NATS
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS: %w", err)
	}

	// Create JetStream context
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating JetStream context: %w", err)
	}

	client := &Client{
		conn:   conn,
		js:     js,
		logger: logger,
	}

	// Ensure stream exists
	if err := client.ensureStream(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensuring stream: %w", err)
	}

	logger.Info("Connected to NATS", "url", conn.ConnectedUrl())
	return client, nil
}

// ensureStream creates or updates the JetStream stream
func (c *Client) ensureStream(ctx context.Context) error {
	streamCfg := jetstream.StreamConfig{
		Name:        events.StreamName,
		Description: "Lab platform events, checkpoints, sessions, pod provisioning, and background jobs",
		Subjects: []string{
			events.SubjectAllEvents,
			events.SubjectAllCheckpoints,
			events.SubjectAllSessions,
			events.SubjectAllGrades,
			events.SubjectAllPods,
			events.SubjectAllJobs,
		},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,     // Keep events for 7 days
		MaxBytes:  1 * 1024 * 1024 * 1024, // 1GB max
		MaxMsgs:   -1,                     // Unlimited messages
		Discard:   jetstream.DiscardOld,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	}

	stream, err := c.js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return fmt.Errorf("creating/updating stream: %w", err)
	}

	c.stream = stream
	c.logger.Info("Stream configured", "name", events.StreamName)
	return nil
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c.conn != nil {
		_ = c.conn.Drain()
		c.conn.Close()
	}
}

// Publish publishes a message to a subject
func (c *Client) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := c.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publishing to %s: %w", subject, err)
	}
	return nil
}

// PublishEvent publishes a VM event
func (c *Client) PublishEvent(ctx context.Context, event *events.VMEvent) error {
	subject := events.BuildEventSubject(event.PodID, event.VMName, event.EventType)
	data, err := events.ToJSON(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	return c.Publish(ctx, subject, data)
}

// PublishCheckpoint publishes a checkpoint update
func (c *Client) PublishCheckpoint(ctx context.Context, update *events.CheckpointUpdate) error {
	subject := events.BuildCheckpointSubject(update.PodID, update.CheckpointID)
	data, err := events.ToJSON(update)
	if err != nil {
		return fmt.Errorf("marshaling checkpoint update: %w", err)
	}
	return c.Publish(ctx, subject, data)
}

// PublishSession publishes a session event
func (c *Client) PublishSession(ctx context.Context, event *events.SessionEvent) error {
	subject := events.BuildSessionSubject(event.SessionID, event.Action)
	data, err := events.ToJSON(event)
	if err != nil {
		return fmt.Errorf("marshaling session event: %w", err)
	}
	return c.Publish(ctx, subject, data)
}

// PublishGrade publishes a grade update
func (c *Client) PublishGrade(ctx context.Context, update *events.GradeUpdate) error {
	subject := events.BuildGradeSubject(update.SessionID)
	data, err := events.ToJSON(update)
	if err != nil {
		return fmt.Errorf("marshaling grade update: %w", err)
	}
	return c.Publish(ctx, subject, data)
}

// PublishAchievementJob publishes an achievement evaluation job
func (c *Client) PublishAchievementJob(ctx context.Context, job *events.AchievementEvaluationJob) error {
	data, err := events.ToJSON(job)
	if err != nil {
		return fmt.Errorf("marshaling achievement job: %w", err)
	}
	return c.Publish(ctx, events.SubjectJobsAchievements, data)
}

// PublishGradeSyncJob publishes a grade sync job
func (c *Client) PublishGradeSyncJob(ctx context.Context, job *events.GradeSyncJob) error {
	data, err := events.ToJSON(job)
	if err != nil {
		return fmt.Errorf("marshaling grade sync job: %w", err)
	}
	return c.Publish(ctx, events.SubjectJobsGradeSync, data)
}

// PublishEventProcessingJob publishes an event processing job
func (c *Client) PublishEventProcessingJob(ctx context.Context, job *events.EventProcessingJob) error {
	data, err := events.ToJSON(job)
	if err != nil {
		return fmt.Errorf("marshaling event processing job: %w", err)
	}
	return c.Publish(ctx, events.SubjectJobsEvents, data)
}

// JetStream returns the JetStream context for creating consumers
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Stream returns the lab events stream
func (c *Client) Stream() jetstream.Stream {
	return c.stream
}

// IsConnected returns true if connected to NATS
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}
