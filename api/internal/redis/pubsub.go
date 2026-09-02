package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message represents a pub/sub message
type Message struct {
	Channel   string          `json:"channel"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source,omitempty"`
}

// MessageHandler handles incoming pub/sub messages
type MessageHandler func(ctx context.Context, msg *Message) error

// PubSub provides Redis pub/sub functionality
type PubSub struct {
	client   *Client
	logger   *slog.Logger
	handlers map[string][]MessageHandler
	mu       sync.RWMutex
	wg       sync.WaitGroup
	closed   chan struct{}
}

// NewPubSub creates a new pub/sub instance
func NewPubSub(client *Client, logger *slog.Logger) *PubSub {
	if logger == nil {
		logger = slog.Default()
	}
	return &PubSub{
		client:   client,
		logger:   logger,
		handlers: make(map[string][]MessageHandler),
		closed:   make(chan struct{}),
	}
}

// Publish publishes a message to a channel
func (ps *PubSub) Publish(ctx context.Context, channel string, msgType string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	msg := Message{
		Channel:   channel,
		Type:      msgType,
		Payload:   payloadJSON,
		Timestamp: time.Now(),
	}

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := ps.client.Publish(ctx, channel, msgJSON); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	ps.logger.Debug("message published",
		slog.String("channel", channel),
		slog.String("type", msgType),
	)

	return nil
}

// Subscribe registers a handler for a channel
func (ps *PubSub) Subscribe(channel string, handler MessageHandler) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.handlers[channel] = append(ps.handlers[channel], handler)
}

// Start begins listening for messages on all subscribed channels
func (ps *PubSub) Start(ctx context.Context) error {
	ps.mu.RLock()
	channels := make([]string, 0, len(ps.handlers))
	for ch := range ps.handlers {
		channels = append(channels, ch)
	}
	ps.mu.RUnlock()

	if len(channels) == 0 {
		ps.logger.Info("no channels to subscribe to")
		return nil
	}

	sub := ps.client.Subscribe(ctx, channels...)
	defer sub.Close()

	ps.logger.Info("started pub/sub listener",
		slog.Int("channels", len(channels)),
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ps.closed:
			return nil
		default:
			msg, err := sub.ReceiveMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				ps.logger.Error("failed to receive message", slog.Any("error", err))
				continue
			}

			ps.wg.Add(1)
			go ps.handleMessage(ctx, msg)
		}
	}
}

// handleMessage processes an incoming message
func (ps *PubSub) handleMessage(ctx context.Context, redisMsg *redis.Message) {
	defer ps.wg.Done()

	var msg Message
	if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		ps.logger.Error("failed to unmarshal message",
			slog.String("channel", redisMsg.Channel),
			slog.Any("error", err),
		)
		return
	}

	// Strip the key prefix from channel name
	channel := redisMsg.Channel
	if len(ps.client.keyPrefix) > 0 && len(channel) > len(ps.client.keyPrefix) {
		channel = channel[len(ps.client.keyPrefix):]
	}

	ps.mu.RLock()
	handlers, ok := ps.handlers[channel]
	ps.mu.RUnlock()

	if !ok {
		return
	}

	for _, handler := range handlers {
		if err := handler(ctx, &msg); err != nil {
			ps.logger.Error("handler error",
				slog.String("channel", channel),
				slog.String("type", msg.Type),
				slog.Any("error", err),
			)
		}
	}
}

// Close stops the pub/sub listener
func (ps *PubSub) Close() {
	close(ps.closed)
	ps.wg.Wait()
}

// --- Event Types ---

// PodStatusEvent represents a pod status change
type PodStatusEvent struct {
	PodID      string `json:"pod_id"`
	Status     string `json:"status"`
	PrevStatus string `json:"prev_status,omitempty"`
	Message    string `json:"message,omitempty"`
}

// CheckpointUpdateEvent represents a checkpoint progress update
type CheckpointUpdateEvent struct {
	SessionID    string    `json:"session_id"`
	PodID        string    `json:"pod_id"`
	CheckpointID string    `json:"checkpoint_id"`
	Status       string    `json:"status"`
	Points       float64   `json:"points,omitempty"`
	Message      string    `json:"message,omitempty"`
	EvaluatedAt  time.Time `json:"evaluated_at"`
}

// GradeSyncEvent represents a grade sync status update
type GradeSyncEvent struct {
	SessionID string  `json:"session_id"`
	Score     float64 `json:"score"`
	Status    string  `json:"status"` // pending, synced, failed
	Error     string  `json:"error,omitempty"`
}

// SessionUpdateEvent represents a session state change
type SessionUpdateEvent struct {
	SessionID string    `json:"session_id"`
	PodID     string    `json:"pod_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"` // started, ended, extended
	Timestamp time.Time `json:"timestamp"`
}

// --- Event Publishers ---

// EventPublisher provides typed event publishing
type EventPublisher struct {
	pubsub *PubSub
	source string
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(pubsub *PubSub, source string) *EventPublisher {
	return &EventPublisher{
		pubsub: pubsub,
		source: source,
	}
}

// PublishPodStatus publishes a pod status change event
func (ep *EventPublisher) PublishPodStatus(ctx context.Context, event PodStatusEvent) error {
	return ep.pubsub.Publish(ctx, ChannelPodStatus, "pod_status", event)
}

// PublishCheckpointUpdate publishes a checkpoint update event
func (ep *EventPublisher) PublishCheckpointUpdate(ctx context.Context, event CheckpointUpdateEvent) error {
	return ep.pubsub.Publish(ctx, ChannelCheckpointUpdate, "checkpoint_update", event)
}

// PublishGradeSync publishes a grade sync event
func (ep *EventPublisher) PublishGradeSync(ctx context.Context, event GradeSyncEvent) error {
	return ep.pubsub.Publish(ctx, ChannelGradeSync, "grade_sync", event)
}

// PublishSessionUpdate publishes a session update event
func (ep *EventPublisher) PublishSessionUpdate(ctx context.Context, event SessionUpdateEvent) error {
	return ep.pubsub.Publish(ctx, ChannelSessionUpdate, "session_update", event)
}

// --- Broadcast Hub ---

// BroadcastHub manages broadcasting messages to multiple subscribers
type BroadcastHub struct {
	pubsub      *PubSub
	subscribers map[string]map[string]chan *Message // channel -> subscriber_id -> message chan
	mu          sync.RWMutex
	logger      *slog.Logger
}

// NewBroadcastHub creates a new broadcast hub
func NewBroadcastHub(pubsub *PubSub, logger *slog.Logger) *BroadcastHub {
	if logger == nil {
		logger = slog.Default()
	}
	return &BroadcastHub{
		pubsub:      pubsub,
		subscribers: make(map[string]map[string]chan *Message),
		logger:      logger,
	}
}

// Subscribe adds a subscriber to a channel
func (bh *BroadcastHub) Subscribe(channel, subscriberID string) <-chan *Message {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	if bh.subscribers[channel] == nil {
		bh.subscribers[channel] = make(map[string]chan *Message)
	}

	ch := make(chan *Message, 100)
	bh.subscribers[channel][subscriberID] = ch

	bh.logger.Debug("subscriber added",
		slog.String("channel", channel),
		slog.String("subscriber", subscriberID),
	)

	return ch
}

// Unsubscribe removes a subscriber from a channel
func (bh *BroadcastHub) Unsubscribe(channel, subscriberID string) {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	if subs, ok := bh.subscribers[channel]; ok {
		if ch, ok := subs[subscriberID]; ok {
			close(ch)
			delete(subs, subscriberID)
		}
	}

	bh.logger.Debug("subscriber removed",
		slog.String("channel", channel),
		slog.String("subscriber", subscriberID),
	)
}

// Broadcast sends a message to all subscribers on a channel
func (bh *BroadcastHub) Broadcast(channel string, msg *Message) {
	bh.mu.RLock()
	subs := bh.subscribers[channel]
	bh.mu.RUnlock()

	for subID, ch := range subs {
		select {
		case ch <- msg:
		default:
			bh.logger.Warn("subscriber channel full, dropping message",
				slog.String("channel", channel),
				slog.String("subscriber", subID),
			)
		}
	}
}

// SetupRedisHandler registers a handler that broadcasts Redis pub/sub messages
func (bh *BroadcastHub) SetupRedisHandler(channel string) {
	bh.pubsub.Subscribe(channel, func(ctx context.Context, msg *Message) error {
		bh.Broadcast(channel, msg)
		return nil
	})
}

// SubscriberCount returns the number of subscribers for a channel
func (bh *BroadcastHub) SubscriberCount(channel string) int {
	bh.mu.RLock()
	defer bh.mu.RUnlock()

	return len(bh.subscribers[channel])
}
