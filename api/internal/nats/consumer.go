package nats

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

// MessageHandler processes a single message
type MessageHandler func(ctx context.Context, msg jetstream.Msg) error

// Consumer wraps a JetStream consumer for processing messages
type Consumer struct {
	client   *Client
	consumer jetstream.Consumer
	name     string
	logger   *slog.Logger
	handler  MessageHandler
	cancel   context.CancelFunc
}

// ConsumerConfig configures a JetStream consumer
type ConsumerConfig struct {
	Name          string
	Durable       bool
	FilterSubject string
	MaxDeliver    int
	AckWait       int // seconds
}

// NewConsumer creates a new consumer for processing messages
func NewConsumer(client *Client, cfg ConsumerConfig, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	consumerCfg := jetstream.ConsumerConfig{
		Name:          cfg.Name,
		FilterSubject: cfg.FilterSubject,
		MaxDeliver:    cfg.MaxDeliver,
		AckPolicy:     jetstream.AckExplicitPolicy,
	}

	if cfg.Durable {
		consumerCfg.Durable = cfg.Name
	}

	consumer, err := client.Stream().CreateOrUpdateConsumer(context.Background(), consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("creating consumer %s: %w", cfg.Name, err)
	}

	return &Consumer{
		client:   client,
		consumer: consumer,
		name:     cfg.Name,
		logger:   logger,
		handler:  handler,
	}, nil
}

// Start begins consuming messages in a goroutine
func (c *Consumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	cons, err := c.consumer.Consume(func(msg jetstream.Msg) {
		if err := c.handler(ctx, msg); err != nil {
			c.logger.Error("Error processing message",
				"consumer", c.name,
				"subject", msg.Subject(),
				"error", err,
			)
			// NAK the message for redelivery
			_ = msg.Nak()
			return
		}
		// ACK successful processing
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("starting consumer %s: %w", c.name, err)
	}

	c.logger.Info("Consumer started", "name", c.name)

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		cons.Stop()
		c.logger.Info("Consumer stopped", "name", c.name)
	}()

	return nil
}

// Stop stops the consumer
func (c *Consumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// -----------------------------------------------------------------------------
// Pre-configured Consumer Factories
// -----------------------------------------------------------------------------

// NewEventStoreConsumer creates a consumer for storing events to the database
func NewEventStoreConsumer(client *Client, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	return NewConsumer(client, ConsumerConfig{
		Name:          events.ConsumerEventStore,
		Durable:       true,
		FilterSubject: events.SubjectAllEvents,
		MaxDeliver:    5,
	}, handler, logger)
}

// NewCheckpointEvaluatorConsumer creates a consumer for evaluating checkpoints
func NewCheckpointEvaluatorConsumer(client *Client, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	return NewConsumer(client, ConsumerConfig{
		Name:          events.ConsumerCheckpointEval,
		Durable:       true,
		FilterSubject: events.SubjectAllEvents,
		MaxDeliver:    3,
	}, handler, logger)
}

// NewWebSocketBroadcastConsumer creates a consumer for broadcasting to WebSocket clients
func NewWebSocketBroadcastConsumer(client *Client, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	// This consumer listens to checkpoint updates and session events
	return NewConsumer(client, ConsumerConfig{
		Name:          events.ConsumerWebSocketBroadcast,
		Durable:       false, // Ephemeral - only active connections matter
		FilterSubject: events.SubjectAllCheckpoints,
		MaxDeliver:    1, // Don't retry broadcasts
	}, handler, logger)
}

// NewGradeSyncConsumer creates a consumer for syncing grades to Canvas
func NewGradeSyncConsumer(client *Client, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	return NewConsumer(client, ConsumerConfig{
		Name:          events.ConsumerGradeSync,
		Durable:       true,
		FilterSubject: events.SubjectAllGrades,
		MaxDeliver:    10, // Retry grade sync more times
	}, handler, logger)
}

// NewAchievementWorkerConsumer creates a consumer for background achievement evaluation
func NewAchievementWorkerConsumer(client *Client, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	return NewConsumer(client, ConsumerConfig{
		Name:          events.ConsumerAchievementWorker,
		Durable:       true,
		FilterSubject: events.SubjectJobsAchievements,
		MaxDeliver:    5,
	}, handler, logger)
}

// NewEventProcessorConsumer creates a consumer for batch event processing
func NewEventProcessorConsumer(client *Client, handler MessageHandler, logger *slog.Logger) (*Consumer, error) {
	return NewConsumer(client, ConsumerConfig{
		Name:          events.ConsumerEventProcessor,
		Durable:       true,
		FilterSubject: events.SubjectJobsEvents,
		MaxDeliver:    5,
	}, handler, logger)
}
