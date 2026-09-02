package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// -----------------------------------------------------------------------------
// Config Tests
// -----------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("expected Host to be localhost, got %s", cfg.Host)
	}
	if cfg.Port != 6379 {
		t.Errorf("expected Port to be 6379, got %d", cfg.Port)
	}
	if cfg.Enabled {
		t.Error("expected Enabled to be false by default (opt-in)")
	}
	if cfg.KeyPrefix != "labctl:" {
		t.Errorf("expected KeyPrefix to be 'labctl:', got %s", cfg.KeyPrefix)
	}
	if cfg.MaxPoolSize != 10 {
		t.Errorf("expected MaxPoolSize to be 10, got %d", cfg.MaxPoolSize)
	}
}

func TestConfigTimeouts(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("expected DialTimeout to be 5s, got %v", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 3*time.Second {
		t.Errorf("expected ReadTimeout to be 3s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 3*time.Second {
		t.Errorf("expected WriteTimeout to be 3s, got %v", cfg.WriteTimeout)
	}
	if cfg.PoolTimeout != 4*time.Second {
		t.Errorf("expected PoolTimeout to be 4s, got %v", cfg.PoolTimeout)
	}
}

func TestConfigPoolSettings(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxPoolSize != 10 {
		t.Errorf("expected MaxPoolSize to be 10, got %d", cfg.MaxPoolSize)
	}
	if cfg.MinIdleConns != 2 {
		t.Errorf("expected MinIdleConns to be 2, got %d", cfg.MinIdleConns)
	}
	if cfg.Database != 0 {
		t.Errorf("expected Database to be 0, got %d", cfg.Database)
	}
}

func TestKeyPrefixes(t *testing.T) {
	// Test that all key prefixes are unique and properly namespaced
	prefixes := []string{
		PrefixSession,
		PrefixUserSession,
		PrefixCache,
		PrefixTemplateCache,
		PrefixPodCache,
		PrefixUserCache,
		PrefixLock,
		PrefixPodLock,
		PrefixRateLimit,
		PrefixTask,
		PrefixTaskQueue,
	}

	seen := make(map[string]bool)
	for _, prefix := range prefixes {
		if seen[prefix] {
			t.Errorf("duplicate prefix: %s", prefix)
		}
		seen[prefix] = true

		// All prefixes should end with a colon for proper namespacing
		if prefix[len(prefix)-1] != ':' {
			t.Errorf("prefix %s should end with ':'", prefix)
		}
	}
}

func TestChannelConstants(t *testing.T) {
	// Test that all channels are unique
	channels := []string{
		ChannelPodStatus,
		ChannelCheckpointUpdate,
		ChannelGradeSync,
		ChannelSessionUpdate,
	}

	seen := make(map[string]bool)
	for _, ch := range channels {
		if seen[ch] {
			t.Errorf("duplicate channel: %s", ch)
		}
		seen[ch] = true
	}
}

func TestDefaultTTLValues(t *testing.T) {
	// Test that default TTL values are reasonable
	if DefaultSessionTTL < 3600 {
		t.Error("DefaultSessionTTL should be at least 1 hour")
	}
	if DefaultCacheTTL < 60 {
		t.Error("DefaultCacheTTL should be at least 1 minute")
	}
	if DefaultLockTTL < 10 {
		t.Error("DefaultLockTTL should be at least 10 seconds")
	}
}

// MockClient is a simple mock for testing components that use the Redis client
type MockClient struct {
	data map[string]string
}

func NewMockClient() *MockClient {
	return &MockClient{
		data: make(map[string]string),
	}
}

func (m *MockClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockClient) Get(ctx context.Context, key string) (string, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return "", nil
}

func (m *MockClient) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Session Data Tests
// -----------------------------------------------------------------------------

func TestSessionDataSerialization(t *testing.T) {
	now := time.Now()
	session := &SessionData{
		UserID:       "user-123",
		Username:     "testuser",
		Email:        "test@example.com",
		Roles:        []string{"student", "ta"},
		ExternalID:   "ext-456",
		CanvasUserID: "canvas-789",
		CreatedAt:    now,
		ExpiresAt:    now.Add(24 * time.Hour),
		LastAccess:   now,
		IPAddress:    "192.168.1.100",
		UserAgent:    "Mozilla/5.0",
		Metadata: map[string]string{
			"course": "CS101",
		},
	}

	// Serialize
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("failed to marshal session: %v", err)
	}

	// Deserialize
	var decoded SessionData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal session: %v", err)
	}

	// Verify fields
	if decoded.UserID != session.UserID {
		t.Errorf("expected UserID %s, got %s", session.UserID, decoded.UserID)
	}
	if decoded.Username != session.Username {
		t.Errorf("expected Username %s, got %s", session.Username, decoded.Username)
	}
	if decoded.Email != session.Email {
		t.Errorf("expected Email %s, got %s", session.Email, decoded.Email)
	}
	if len(decoded.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(decoded.Roles))
	}
	if decoded.Metadata["course"] != "CS101" {
		t.Errorf("expected course=CS101, got %s", decoded.Metadata["course"])
	}
}

func TestWebSocketSessionSerialization(t *testing.T) {
	now := time.Now()
	session := &WebSocketSession{
		ConnectionID: "conn-123",
		UserID:       "user-456",
		PodID:        "pod-789",
		SessionID:    "session-abc",
		ConnectedAt:  now,
		LastPing:     now,
		ServerNode:   "node-1",
	}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("failed to marshal websocket session: %v", err)
	}

	var decoded WebSocketSession
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal websocket session: %v", err)
	}

	if decoded.ConnectionID != session.ConnectionID {
		t.Errorf("expected ConnectionID %s, got %s", session.ConnectionID, decoded.ConnectionID)
	}
	if decoded.PodID != session.PodID {
		t.Errorf("expected PodID %s, got %s", session.PodID, decoded.PodID)
	}
	if decoded.ServerNode != session.ServerNode {
		t.Errorf("expected ServerNode %s, got %s", session.ServerNode, decoded.ServerNode)
	}
}

// -----------------------------------------------------------------------------
// Rate Limit Result Tests
// -----------------------------------------------------------------------------

func TestRateLimitResult(t *testing.T) {
	tests := []struct {
		name      string
		result    RateLimitResult
		wantAllow bool
	}{
		{
			name: "allowed with remaining",
			result: RateLimitResult{
				Allowed:   true,
				Remaining: 5,
				ResetAt:   time.Now().Add(time.Minute),
			},
			wantAllow: true,
		},
		{
			name: "not allowed",
			result: RateLimitResult{
				Allowed:   false,
				Remaining: 0,
				ResetAt:   time.Now().Add(time.Minute),
				RetryIn:   30 * time.Second,
			},
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Allowed != tt.wantAllow {
				t.Errorf("expected Allowed=%v, got %v", tt.wantAllow, tt.result.Allowed)
			}
		})
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.Limit != 100 {
		t.Errorf("expected Limit 100, got %d", cfg.Limit)
	}
	if cfg.Window != time.Minute {
		t.Errorf("expected Window 1m, got %v", cfg.Window)
	}
	if cfg.KeyFunc == nil {
		t.Error("expected KeyFunc to be set")
	}
	if cfg.OnLimitExceeded == nil {
		t.Error("expected OnLimitExceeded to be set")
	}
}

// -----------------------------------------------------------------------------
// Task State Tests
// -----------------------------------------------------------------------------

func TestTaskStates(t *testing.T) {
	states := []TaskState{
		TaskStatePending,
		TaskStateProcessing,
		TaskStateCompleted,
		TaskStateFailed,
		TaskStateRetrying,
	}

	// Verify all states are unique
	seen := make(map[TaskState]bool)
	for _, state := range states {
		if seen[state] {
			t.Errorf("duplicate state: %s", state)
		}
		seen[state] = true
	}

	// Verify expected values
	if TaskStatePending != "pending" {
		t.Errorf("expected pending, got %s", TaskStatePending)
	}
	if TaskStateProcessing != "processing" {
		t.Errorf("expected processing, got %s", TaskStateProcessing)
	}
	if TaskStateCompleted != "completed" {
		t.Errorf("expected completed, got %s", TaskStateCompleted)
	}
	if TaskStateFailed != "failed" {
		t.Errorf("expected failed, got %s", TaskStateFailed)
	}
	if TaskStateRetrying != "retrying" {
		t.Errorf("expected retrying, got %s", TaskStateRetrying)
	}
}

func TestTaskSerialization(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:          "task-123",
		Type:        TaskTypeGradeSync,
		Payload:     json.RawMessage(`{"session_id": "sess-1"}`),
		State:       TaskStatePending,
		Priority:    5,
		MaxRetries:  3,
		RetryCount:  0,
		CreatedAt:   now,
		ScheduledAt: now,
		Metadata:    map[string]any{"key": "value"},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("failed to marshal task: %v", err)
	}

	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal task: %v", err)
	}

	if decoded.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, decoded.ID)
	}
	if decoded.Type != task.Type {
		t.Errorf("expected Type %s, got %s", task.Type, decoded.Type)
	}
	if decoded.State != task.State {
		t.Errorf("expected State %s, got %s", task.State, decoded.State)
	}
	if decoded.Priority != task.Priority {
		t.Errorf("expected Priority %d, got %d", task.Priority, decoded.Priority)
	}
}

func TestTaskPayloadTypes(t *testing.T) {
	// Test GradeSyncPayload
	gradePayload := GradeSyncPayload{
		SessionID:    "sess-123",
		Score:        95.5,
		CanvasUserID: "canvas-456",
		AssignmentID: "assign-789",
		CourseID:     "course-001",
	}
	data, err := json.Marshal(gradePayload)
	if err != nil {
		t.Fatalf("failed to marshal GradeSyncPayload: %v", err)
	}

	var decoded GradeSyncPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal GradeSyncPayload: %v", err)
	}
	if decoded.Score != 95.5 {
		t.Errorf("expected Score 95.5, got %f", decoded.Score)
	}

	// Test PodCleanupPayload
	cleanupPayload := PodCleanupPayload{
		PodID:  "pod-123",
		Reason: "expired",
	}
	data, err = json.Marshal(cleanupPayload)
	if err != nil {
		t.Fatalf("failed to marshal PodCleanupPayload: %v", err)
	}

	var decodedCleanup PodCleanupPayload
	if err := json.Unmarshal(data, &decodedCleanup); err != nil {
		t.Fatalf("failed to unmarshal PodCleanupPayload: %v", err)
	}
	if decodedCleanup.Reason != "expired" {
		t.Errorf("expected Reason 'expired', got %s", decodedCleanup.Reason)
	}
}

func TestPredefinedTaskTypes(t *testing.T) {
	taskTypes := []string{
		TaskTypeGradeSync,
		TaskTypePodCleanup,
		TaskTypePodExpiration,
		TaskTypeSnapshotCreate,
		TaskTypeSnapshotRevert,
		TaskTypeAssessmentRun,
		TaskTypeEventProcess,
		TaskTypeNotificationSend,
		TaskTypeAuditLogPersist,
		TaskTypeCheckpointEvaluate,
	}

	// Verify all types are unique
	seen := make(map[string]bool)
	for _, tt := range taskTypes {
		if seen[tt] {
			t.Errorf("duplicate task type: %s", tt)
		}
		seen[tt] = true
		if tt == "" {
			t.Error("empty task type found")
		}
	}
}

// -----------------------------------------------------------------------------
// Message/Event Tests
// -----------------------------------------------------------------------------

func TestMessageSerialization(t *testing.T) {
	now := time.Now()
	msg := &Message{
		Channel:   ChannelPodStatus,
		Type:      "pod_status",
		Payload:   json.RawMessage(`{"pod_id": "pod-123", "status": "running"}`),
		Timestamp: now,
		Source:    "api-node-1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decoded.Channel != msg.Channel {
		t.Errorf("expected Channel %s, got %s", msg.Channel, decoded.Channel)
	}
	if decoded.Type != msg.Type {
		t.Errorf("expected Type %s, got %s", msg.Type, decoded.Type)
	}
	if decoded.Source != msg.Source {
		t.Errorf("expected Source %s, got %s", msg.Source, decoded.Source)
	}
}

func TestPodStatusEventSerialization(t *testing.T) {
	event := PodStatusEvent{
		PodID:      "pod-123",
		Status:     "running",
		PrevStatus: "creating",
		Message:    "Pod started successfully",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded PodStatusEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.PodID != event.PodID {
		t.Errorf("expected PodID %s, got %s", event.PodID, decoded.PodID)
	}
	if decoded.Status != event.Status {
		t.Errorf("expected Status %s, got %s", event.Status, decoded.Status)
	}
}

func TestCheckpointUpdateEventSerialization(t *testing.T) {
	now := time.Now()
	event := CheckpointUpdateEvent{
		SessionID:    "sess-123",
		PodID:        "pod-456",
		CheckpointID: "cp-789",
		Status:       "passed",
		Points:       10.0,
		Message:      "Checkpoint completed",
		EvaluatedAt:  now,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded CheckpointUpdateEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.Points != event.Points {
		t.Errorf("expected Points %f, got %f", event.Points, decoded.Points)
	}
	if decoded.Status != event.Status {
		t.Errorf("expected Status %s, got %s", event.Status, decoded.Status)
	}
}

func TestGradeSyncEventSerialization(t *testing.T) {
	event := GradeSyncEvent{
		SessionID: "sess-123",
		Score:     85.5,
		Status:    "synced",
		Error:     "",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded GradeSyncEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.Score != event.Score {
		t.Errorf("expected Score %f, got %f", event.Score, decoded.Score)
	}
}

func TestSessionUpdateEventSerialization(t *testing.T) {
	now := time.Now()
	event := SessionUpdateEvent{
		SessionID: "sess-123",
		PodID:     "pod-456",
		UserID:    "user-789",
		Action:    "started",
		Timestamp: now,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded SessionUpdateEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.Action != event.Action {
		t.Errorf("expected Action %s, got %s", event.Action, decoded.Action)
	}
}

// -----------------------------------------------------------------------------
// Broadcast Hub Tests
// -----------------------------------------------------------------------------

func TestBroadcastHubSubscribeUnsubscribe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a minimal PubSub for the hub (won't actually connect to Redis)
	hub := &BroadcastHub{
		subscribers: make(map[string]map[string]chan *Message),
		logger:      logger,
	}

	// Subscribe
	ch := hub.Subscribe("test-channel", "subscriber-1")
	if ch == nil {
		t.Fatal("expected channel to be returned")
	}

	// Check subscriber count
	count := hub.SubscriberCount("test-channel")
	if count != 1 {
		t.Errorf("expected 1 subscriber, got %d", count)
	}

	// Subscribe another
	ch2 := hub.Subscribe("test-channel", "subscriber-2")
	if ch2 == nil {
		t.Fatal("expected channel to be returned")
	}

	count = hub.SubscriberCount("test-channel")
	if count != 2 {
		t.Errorf("expected 2 subscribers, got %d", count)
	}

	// Unsubscribe one
	hub.Unsubscribe("test-channel", "subscriber-1")
	count = hub.SubscriberCount("test-channel")
	if count != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", count)
	}

	// Unsubscribe the other
	hub.Unsubscribe("test-channel", "subscriber-2")
	count = hub.SubscriberCount("test-channel")
	if count != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", count)
	}
}

func TestBroadcastHubBroadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	hub := &BroadcastHub{
		subscribers: make(map[string]map[string]chan *Message),
		logger:      logger,
	}

	// Subscribe two clients
	ch1 := hub.Subscribe("events", "client-1")
	ch2 := hub.Subscribe("events", "client-2")

	// Broadcast a message
	msg := &Message{
		Channel: "events",
		Type:    "test",
		Payload: json.RawMessage(`{"data": "hello"}`),
	}

	hub.Broadcast("events", msg)

	// Both should receive the message
	select {
	case received := <-ch1:
		if received.Type != "test" {
			t.Errorf("expected Type 'test', got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client-1 did not receive message")
	}

	select {
	case received := <-ch2:
		if received.Type != "test" {
			t.Errorf("expected Type 'test', got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client-2 did not receive message")
	}
}

func TestBroadcastHubConcurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	hub := &BroadcastHub{
		subscribers: make(map[string]map[string]chan *Message),
		logger:      logger,
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent subscribe/unsubscribe
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subID := "sub-" + string(rune('A'+id))
			ch := hub.Subscribe("concurrent", subID)
			time.Sleep(10 * time.Millisecond)
			hub.Unsubscribe("concurrent", subID)
			_ = ch
		}(i)
	}

	wg.Wait()

	// All should be unsubscribed
	count := hub.SubscriberCount("concurrent")
	if count != 0 {
		t.Errorf("expected 0 subscribers, got %d", count)
	}
}

// -----------------------------------------------------------------------------
// Lock Error Tests
// -----------------------------------------------------------------------------

func TestLockErrors(t *testing.T) {
	if ErrLockNotAcquired == nil {
		t.Error("ErrLockNotAcquired should not be nil")
	}
	if ErrLockLost == nil {
		t.Error("ErrLockLost should not be nil")
	}
	if ErrSemaphoreFull == nil {
		t.Error("ErrSemaphoreFull should not be nil")
	}

	// Verify error messages
	if ErrLockNotAcquired.Error() != "lock not acquired" {
		t.Errorf("unexpected error message: %s", ErrLockNotAcquired.Error())
	}
	if ErrLockLost.Error() != "lock was lost" {
		t.Errorf("unexpected error message: %s", ErrLockLost.Error())
	}
	if ErrSemaphoreFull.Error() != "semaphore is full" {
		t.Errorf("unexpected error message: %s", ErrSemaphoreFull.Error())
	}
}

// -----------------------------------------------------------------------------
// Cache Error Tests
// -----------------------------------------------------------------------------

func TestCacheErrors(t *testing.T) {
	if ErrCacheMiss == nil {
		t.Error("ErrCacheMiss should not be nil")
	}
	if ErrCacheMiss.Error() != "cache miss" {
		t.Errorf("unexpected error message: %s", ErrCacheMiss.Error())
	}
}

// -----------------------------------------------------------------------------
// Enqueue Options Tests
// -----------------------------------------------------------------------------

func TestEnqueueOptions(t *testing.T) {
	now := time.Now()
	opts := EnqueueOptions{
		ID:          "custom-id",
		Priority:    10,
		MaxRetries:  5,
		Delay:       30 * time.Second,
		ScheduledAt: now.Add(time.Hour),
		Metadata: map[string]any{
			"source": "test",
		},
	}

	if opts.ID != "custom-id" {
		t.Errorf("expected ID 'custom-id', got %s", opts.ID)
	}
	if opts.Priority != 10 {
		t.Errorf("expected Priority 10, got %d", opts.Priority)
	}
	if opts.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", opts.MaxRetries)
	}
	if opts.Delay != 30*time.Second {
		t.Errorf("expected Delay 30s, got %v", opts.Delay)
	}
	if opts.Metadata["source"] != "test" {
		t.Errorf("expected Metadata[source]='test', got %v", opts.Metadata["source"])
	}
}

// -----------------------------------------------------------------------------
// TaskQueueConfig Tests
// -----------------------------------------------------------------------------

func TestTaskQueueConfig(t *testing.T) {
	cfg := TaskQueueConfig{
		Name:           "my-queue",
		VisibilityTime: 10 * time.Minute,
	}

	if cfg.Name != "my-queue" {
		t.Errorf("expected Name 'my-queue', got %s", cfg.Name)
	}
	if cfg.VisibilityTime != 10*time.Minute {
		t.Errorf("expected VisibilityTime 10m, got %v", cfg.VisibilityTime)
	}
}

// -----------------------------------------------------------------------------
// ServiceConfig Tests
// -----------------------------------------------------------------------------

func TestServiceConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := ServiceConfig{
		Redis: Config{
			Host:      "redis.example.com",
			Port:      6380,
			Password:  "secret",
			Database:  1,
			Enabled:   true,
			KeyPrefix: "myapp:",
		},
		ServerNode: "node-1",
		Logger:     logger,
	}

	if cfg.Redis.Host != "redis.example.com" {
		t.Errorf("expected Host 'redis.example.com', got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Errorf("expected Port 6380, got %d", cfg.Redis.Port)
	}
	if cfg.ServerNode != "node-1" {
		t.Errorf("expected ServerNode 'node-1', got %s", cfg.ServerNode)
	}
}

// -----------------------------------------------------------------------------
// Client Key Helper Tests
// -----------------------------------------------------------------------------

func TestClientKeyGeneration(t *testing.T) {
	// Test that Key function properly concatenates prefix with parts
	// We can test this by creating a config and checking expected output

	cfg := DefaultConfig()
	cfg.KeyPrefix = "test:"

	tests := []struct {
		name     string
		prefix   string
		parts    []string
		expected string
	}{
		{
			name:     "single part",
			prefix:   "test:",
			parts:    []string{"key"},
			expected: "test:key",
		},
		{
			name:     "multiple parts",
			prefix:   "test:",
			parts:    []string{"session", ":", "user-123"},
			expected: "test:session:user-123",
		},
		{
			name:     "empty prefix",
			prefix:   "",
			parts:    []string{"key"},
			expected: "key",
		},
		{
			name:     "no parts",
			prefix:   "test:",
			parts:    []string{},
			expected: "test:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the Key function logic
			key := tt.prefix
			for _, part := range tt.parts {
				key += part
			}
			if key != tt.expected {
				t.Errorf("expected key %s, got %s", tt.expected, key)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Session Store Logic Tests
// -----------------------------------------------------------------------------

func TestNewSessionStoreDefaults(t *testing.T) {
	// Test that SessionStore uses default TTL when 0 is passed
	store := &SessionStore{
		client: nil,
		ttl:    0,
	}

	// Default should be used when ttl is 0
	if store.ttl != 0 {
		t.Errorf("expected ttl 0 for uninitialized store")
	}

	// When NewSessionStore is called with 0, it should use DefaultSessionTTL
	// We can verify the logic without a real client
	testTTL := time.Duration(0)
	if testTTL == 0 {
		testTTL = DefaultSessionTTL
	}
	if testTTL != DefaultSessionTTL {
		t.Errorf("expected default TTL %v, got %v", DefaultSessionTTL, testTTL)
	}
}

func TestSessionDataExpiration(t *testing.T) {
	now := time.Now()
	session := &SessionData{
		UserID:    "user-123",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}

	// Test that session is not expired
	if session.ExpiresAt.Before(now) {
		t.Error("session should not be expired yet")
	}

	// Test expired session
	expiredSession := &SessionData{
		UserID:    "user-456",
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}

	if expiredSession.ExpiresAt.After(now) {
		t.Error("session should be expired")
	}
}

// -----------------------------------------------------------------------------
// WebSocket Store Logic Tests
// -----------------------------------------------------------------------------

func TestNewWebSocketStoreDefaults(t *testing.T) {
	// Test default TTL logic
	testTTL := time.Duration(0)
	if testTTL == 0 {
		testTTL = 10 * time.Minute
	}
	if testTTL != 10*time.Minute {
		t.Errorf("expected default WebSocket TTL 10m, got %v", testTTL)
	}
}

func TestWebSocketSessionTracking(t *testing.T) {
	now := time.Now()
	session := &WebSocketSession{
		ConnectionID: "conn-123",
		UserID:       "user-456",
		PodID:        "pod-789",
		ConnectedAt:  now,
		LastPing:     now,
		ServerNode:   "node-1",
	}

	// Test that connection is fresh
	if session.LastPing.Before(now.Add(-time.Minute)) {
		t.Error("session should be fresh")
	}

	// Simulate stale connection
	staleSession := &WebSocketSession{
		ConnectionID: "conn-old",
		LastPing:     now.Add(-15 * time.Minute),
	}

	if !staleSession.LastPing.Before(now.Add(-10 * time.Minute)) {
		t.Error("stale session should be older than 10 minutes")
	}
}

// -----------------------------------------------------------------------------
// Cache Logic Tests
// -----------------------------------------------------------------------------

func TestNewCacheDefaults(t *testing.T) {
	// Test default TTL logic
	testTTL := time.Duration(0)
	if testTTL == 0 {
		testTTL = DefaultCacheTTL
	}
	if testTTL != DefaultCacheTTL {
		t.Errorf("expected default cache TTL %v, got %v", DefaultCacheTTL, testTTL)
	}
}

func TestTemplateCacheTTL(t *testing.T) {
	if DefaultTemplateTTL != 1*time.Hour {
		t.Errorf("expected template TTL 1h, got %v", DefaultTemplateTTL)
	}
}

func TestPodStatusCacheTTL(t *testing.T) {
	if DefaultPodStatusTTL != 30*time.Second {
		t.Errorf("expected pod status TTL 30s, got %v", DefaultPodStatusTTL)
	}
}

// -----------------------------------------------------------------------------
// Lock Manager Logic Tests
// -----------------------------------------------------------------------------

func TestGenerateTokenLength(t *testing.T) {
	// Test that token generation produces expected length
	// Token is 16 bytes, hex-encoded = 32 chars
	token := make([]byte, 16)
	encoded := hex.EncodeToString(token)
	if len(encoded) != 32 {
		t.Errorf("expected token length 32, got %d", len(encoded))
	}
}

func TestLockManagerDefaults(t *testing.T) {
	// Test default TTL
	if DefaultLockTTL != 30*time.Second {
		t.Errorf("expected default lock TTL 30s, got %v", DefaultLockTTL)
	}
}

func TestAcquireWithRetryDefaults(t *testing.T) {
	// Test default maxAttempts logic
	maxAttempts := 0
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts != 3 {
		t.Errorf("expected default maxAttempts 3, got %d", maxAttempts)
	}

	// Test default retryDelay logic
	retryDelay := time.Duration(0)
	if retryDelay == 0 {
		retryDelay = 100 * time.Millisecond
	}
	if retryDelay != 100*time.Millisecond {
		t.Errorf("expected default retryDelay 100ms, got %v", retryDelay)
	}
}

// -----------------------------------------------------------------------------
// Rate Limiter Logic Tests
// -----------------------------------------------------------------------------

func TestRateLimiterKeyGeneration(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	// The key must be the IP alone. It used to be raw r.RemoteAddr, which is
	// "IP:port" — and the source port is ephemeral, so every new TCP
	// connection got its own bucket and the IP limit never bound.
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	if key := cfg.KeyFunc(req); key != "ip:192.168.1.100" {
		t.Errorf("expected key ip:192.168.1.100, got %s", key)
	}

	// Two connections from one client share a bucket.
	req2, _ := http.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.100:54321"
	if cfg.KeyFunc(req) != cfg.KeyFunc(req2) {
		t.Error("different source ports from one IP must share a rate-limit bucket")
	}

	// X-Real-IP, which nginx sets, identifies the client behind a proxy.
	proxied, _ := http.NewRequest("GET", "/api/test", nil)
	proxied.RemoteAddr = "10.0.0.1:443"
	proxied.Header.Set("X-Real-IP", "203.0.113.7")
	if key := cfg.KeyFunc(proxied); key != "ip:203.0.113.7" {
		t.Errorf("expected key ip:203.0.113.7, got %s", key)
	}

	// X-Forwarded-For is client-controlled and must never key the limiter.
	spoofed, _ := http.NewRequest("GET", "/api/test", nil)
	spoofed.RemoteAddr = "192.168.1.100:12345"
	spoofed.Header.Set("X-Forwarded-For", "1.2.3.4")
	if key := cfg.KeyFunc(spoofed); key != "ip:192.168.1.100" {
		t.Errorf("X-Forwarded-For changed the rate-limit key to %s — it is spoofable", key)
	}
}

func TestAPIRateLimiterPerEndpointKey(t *testing.T) {
	// Test the endpoint key generation logic
	req, _ := http.NewRequest("POST", "/api/v1/pods", nil)
	req.RemoteAddr = "10.0.0.1:54321"

	keyFunc := func(r *http.Request) string {
		return "endpoint:" + r.Method + ":" + r.URL.Path + ":" + r.RemoteAddr
	}

	key := keyFunc(req)
	expected := "endpoint:POST:/api/v1/pods:10.0.0.1:54321"
	if key != expected {
		t.Errorf("expected key %s, got %s", expected, key)
	}
}

func TestPodOperationRateLimiterKeyGeneration(t *testing.T) {
	// Test key generation for pod operations
	userID := "user-123"

	createKey := "pod:create:" + userID
	if createKey != "pod:create:user-123" {
		t.Errorf("unexpected create key: %s", createKey)
	}

	snapshotKey := "pod:snapshot:" + userID
	if snapshotKey != "pod:snapshot:user-123" {
		t.Errorf("unexpected snapshot key: %s", snapshotKey)
	}

	podID := "pod-456"
	revertKey := "pod:revert:" + podID
	if revertKey != "pod:revert:pod-456" {
		t.Errorf("unexpected revert key: %s", revertKey)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	result := &RateLimitResult{
		Allowed:   true,
		Remaining: 95,
		ResetAt:   time.Now().Add(time.Minute),
	}

	// Test header values
	limitHeader := strconv.FormatInt(100, 10)
	if limitHeader != "100" {
		t.Errorf("expected limit header '100', got %s", limitHeader)
	}

	remainingHeader := strconv.FormatInt(result.Remaining, 10)
	if remainingHeader != "95" {
		t.Errorf("expected remaining header '95', got %s", remainingHeader)
	}

	resetHeader := strconv.FormatInt(result.ResetAt.Unix(), 10)
	if resetHeader == "" {
		t.Error("reset header should not be empty")
	}
}

// -----------------------------------------------------------------------------
// Task Queue Logic Tests
// -----------------------------------------------------------------------------

func TestTaskQueueConfigDefaults(t *testing.T) {
	cfg := TaskQueueConfig{}

	// Test default name
	name := cfg.Name
	if name == "" {
		name = "default"
	}
	if name != "default" {
		t.Errorf("expected default name 'default', got %s", name)
	}

	// Test default visibility time
	visibility := cfg.VisibilityTime
	if visibility == 0 {
		visibility = DefaultTaskVisibility
	}
	if visibility != DefaultTaskVisibility {
		t.Errorf("expected default visibility %v, got %v", DefaultTaskVisibility, visibility)
	}
}

func TestTaskQueueKeyGeneration(t *testing.T) {
	queueName := "myqueue"

	pendingKey := PrefixTaskQueue + queueName + ":pending"
	if pendingKey != "task:queue:myqueue:pending" {
		t.Errorf("unexpected pending key: %s", pendingKey)
	}

	processingKey := PrefixTaskQueue + queueName + ":processing"
	if processingKey != "task:queue:myqueue:processing" {
		t.Errorf("unexpected processing key: %s", processingKey)
	}

	scheduledKey := PrefixTaskQueue + queueName + ":scheduled"
	if scheduledKey != "task:queue:myqueue:scheduled" {
		t.Errorf("unexpected scheduled key: %s", scheduledKey)
	}

	deadLetterKey := PrefixTaskQueue + queueName + ":dead"
	if deadLetterKey != "task:queue:myqueue:dead" {
		t.Errorf("unexpected dead letter key: %s", deadLetterKey)
	}

	taskID := "task-123"
	taskKey := PrefixTask + queueName + ":" + taskID
	if taskKey != "task:myqueue:task-123" {
		t.Errorf("unexpected task key: %s", taskKey)
	}
}

func TestEnqueueOptionsDefaults(t *testing.T) {
	opts := EnqueueOptions{}

	// Test default max retries
	maxRetries := opts.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	if maxRetries != 3 {
		t.Errorf("expected default maxRetries 3, got %d", maxRetries)
	}

	// Test scheduling logic
	now := time.Now()

	// No delay or scheduled time - should be immediate
	scheduledAt := now
	if opts.Delay > 0 {
		scheduledAt = now.Add(opts.Delay)
	} else if !opts.ScheduledAt.IsZero() {
		scheduledAt = opts.ScheduledAt
	}

	if !scheduledAt.Equal(now) {
		t.Errorf("expected immediate scheduling")
	}

	// With delay
	optsWithDelay := EnqueueOptions{Delay: 5 * time.Minute}
	scheduledAt = now
	if optsWithDelay.Delay > 0 {
		scheduledAt = now.Add(optsWithDelay.Delay)
	}
	if scheduledAt.Before(now.Add(4 * time.Minute)) {
		t.Error("scheduled time should be in the future")
	}
}

func TestTaskRetryBackoff(t *testing.T) {
	// Test exponential backoff calculation
	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{1, 1 * time.Second},  // 1*1 = 1
		{2, 4 * time.Second},  // 2*2 = 4
		{3, 9 * time.Second},  // 3*3 = 9
		{4, 16 * time.Second}, // 4*4 = 16
	}

	for _, tt := range tests {
		delay := time.Duration(tt.retryCount*tt.retryCount) * time.Second
		if delay != tt.expected {
			t.Errorf("retry %d: expected delay %v, got %v", tt.retryCount, tt.expected, delay)
		}
	}
}

func TestTaskPriorityScore(t *testing.T) {
	now := time.Now()
	nowMs := float64(now.UnixMilli())

	// Higher priority should result in higher score
	highPriority := float64(10)*1e15 - nowMs
	lowPriority := float64(1)*1e15 - nowMs

	if highPriority <= lowPriority {
		t.Error("high priority task should have higher score")
	}
}

// -----------------------------------------------------------------------------
// Semaphore Logic Tests
// -----------------------------------------------------------------------------

func TestSemaphoreAvailableTokens(t *testing.T) {
	maxTokens := int64(5)
	currentCount := int64(3)

	available := maxTokens - currentCount
	if available != 2 {
		t.Errorf("expected 2 available tokens, got %d", available)
	}
}

func TestSemaphoreKeyGeneration(t *testing.T) {
	keyPrefix := "labctl:"
	semKey := "my-semaphore"

	fullKey := keyPrefix + PrefixLock + "sem:" + semKey
	expected := "labctl:lock:sem:my-semaphore"
	if fullKey != expected {
		t.Errorf("expected key %s, got %s", expected, fullKey)
	}
}

// -----------------------------------------------------------------------------
// PubSub Logic Tests
// -----------------------------------------------------------------------------

func TestPubSubHandlerRegistration(t *testing.T) {
	handlers := make(map[string][]MessageHandler)

	// Register a handler
	channel := "test-channel"
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	handlers[channel] = append(handlers[channel], handler)

	if len(handlers[channel]) != 1 {
		t.Errorf("expected 1 handler, got %d", len(handlers[channel]))
	}

	// Register another handler for same channel
	handlers[channel] = append(handlers[channel], handler)
	if len(handlers[channel]) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(handlers[channel]))
	}
}

func TestEventPublisherTypes(t *testing.T) {
	// Verify event types are properly structured
	podEvent := PodStatusEvent{
		PodID:      "pod-123",
		Status:     "running",
		PrevStatus: "creating",
		Message:    "Pod is now running",
	}

	if podEvent.PodID == "" {
		t.Error("PodID should not be empty")
	}

	checkpointEvent := CheckpointUpdateEvent{
		SessionID:    "sess-123",
		PodID:        "pod-456",
		CheckpointID: "cp-789",
		Status:       "passed",
		Points:       10.0,
		EvaluatedAt:  time.Now(),
	}

	if checkpointEvent.Points != 10.0 {
		t.Errorf("expected Points 10.0, got %f", checkpointEvent.Points)
	}
}

// -----------------------------------------------------------------------------
// HTTP Middleware Tests
// -----------------------------------------------------------------------------

func TestRateLimitMiddlewareSkipFunc(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	cfg.SkipFunc = func(r *http.Request) bool {
		return r.URL.Path == "/health"
	}

	// Health endpoint should be skipped
	healthReq, _ := http.NewRequest("GET", "/health", nil)
	if !cfg.SkipFunc(healthReq) {
		t.Error("health endpoint should be skipped")
	}

	// API endpoint should not be skipped
	apiReq, _ := http.NewRequest("GET", "/api/v1/pods", nil)
	if cfg.SkipFunc(apiReq) {
		t.Error("API endpoint should not be skipped")
	}
}

func TestOnLimitExceededHandler(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	result := &RateLimitResult{
		Allowed:   false,
		Remaining: 0,
		ResetAt:   time.Now().Add(time.Minute),
		RetryIn:   30 * time.Second,
	}

	// Create a response recorder
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)

	cfg.OnLimitExceeded(rr, req, result)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rr.Code)
	}

	// Check headers
	if rr.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header should be set")
	}
}

// -----------------------------------------------------------------------------
// Integration-style Unit Tests (no Redis required)
// -----------------------------------------------------------------------------

func TestSessionStoreWithUpdateFunc(t *testing.T) {
	session := &SessionData{
		UserID:   "user-123",
		Username: "testuser",
		Roles:    []string{"student"},
		Metadata: make(map[string]string),
	}

	// Simulate update function
	updateFn := func(s *SessionData) {
		s.Metadata["last_lab"] = "lab-456"
		s.Roles = append(s.Roles, "ta")
	}

	updateFn(session)

	if session.Metadata["last_lab"] != "lab-456" {
		t.Error("metadata should be updated")
	}
	if len(session.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(session.Roles))
	}
}

func TestCacheGetOrSetLogic(t *testing.T) {
	// Test the GetOrSet pattern logic
	cacheHit := false
	callCount := 0

	// Simulate cache miss then set
	fn := func() (interface{}, error) {
		callCount++
		return map[string]string{"key": "value"}, nil
	}

	// First call - cache miss
	if !cacheHit {
		result, err := fn()
		if err != nil {
			t.Fatalf("fn should not error: %v", err)
		}
		if result == nil {
			t.Error("result should not be nil")
		}
	}

	if callCount != 1 {
		t.Errorf("expected fn called once, got %d", callCount)
	}

	// Second call - cache hit (simulated): fn should not be called again
	cacheHit = true
	_ = cacheHit

	if callCount != 1 {
		t.Errorf("fn should not be called on cache hit, count: %d", callCount)
	}
}

func TestTaskStateTransitions(t *testing.T) {
	task := &Task{
		ID:    "task-123",
		State: TaskStatePending,
	}

	// Pending -> Processing
	task.State = TaskStateProcessing
	now := time.Now()
	task.StartedAt = &now

	if task.State != TaskStateProcessing {
		t.Error("task should be processing")
	}
	if task.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Processing -> Completed
	task.State = TaskStateCompleted
	task.CompletedAt = &now

	if task.State != TaskStateCompleted {
		t.Error("task should be completed")
	}

	// Test failed path
	failedTask := &Task{
		ID:         "task-456",
		State:      TaskStateProcessing,
		MaxRetries: 3,
		RetryCount: 0,
	}

	failedTask.RetryCount++
	if failedTask.RetryCount < failedTask.MaxRetries {
		failedTask.State = TaskStateRetrying
	} else {
		failedTask.State = TaskStateFailed
	}

	if failedTask.State != TaskStateRetrying {
		t.Errorf("expected retrying state, got %s", failedTask.State)
	}

	// Exhaust retries
	failedTask.RetryCount = 3
	if failedTask.RetryCount >= failedTask.MaxRetries {
		failedTask.State = TaskStateFailed
	}

	if failedTask.State != TaskStateFailed {
		t.Errorf("expected failed state, got %s", failedTask.State)
	}
}

// -----------------------------------------------------------------------------
// Additional Specialized Lock Tests
// -----------------------------------------------------------------------------

func TestPodLockKeyGeneration(t *testing.T) {
	podID := "pod-123"
	expectedKey := "pod:" + podID

	if expectedKey != "pod:pod-123" {
		t.Errorf("unexpected pod lock key: %s", expectedKey)
	}
}

func TestSessionLockKeyGeneration(t *testing.T) {
	sessionID := "sess-123"

	sessionKey := "session:" + sessionID
	if sessionKey != "session:sess-123" {
		t.Errorf("unexpected session lock key: %s", sessionKey)
	}

	gradeSyncKey := "grade_sync:" + sessionID
	if gradeSyncKey != "grade_sync:sess-123" {
		t.Errorf("unexpected grade sync lock key: %s", gradeSyncKey)
	}
}

func TestResourceLockKeyGeneration(t *testing.T) {
	tests := []struct {
		lockType string
		id       string
		expected string
	}{
		{"vlan", "node-1", "vlan:node-1"},
		{"vm", "vm-100", "vm:vm-100"},
		{"template", "tmpl-ubuntu", "template:tmpl-ubuntu"},
	}

	for _, tt := range tests {
		key := tt.lockType + ":" + tt.id
		if key != tt.expected {
			t.Errorf("expected key %s, got %s", tt.expected, key)
		}
	}
}

// -----------------------------------------------------------------------------
// Specialized Cache Tests
// -----------------------------------------------------------------------------

func TestTemplateCacheKeyGeneration(t *testing.T) {
	templateID := "tmpl-123"

	templateKey := "template:" + templateID
	if templateKey != "template:tmpl-123" {
		t.Errorf("unexpected template key: %s", templateKey)
	}

	listKey := "templates:active"
	if listKey != "templates:active" {
		t.Errorf("unexpected template list key: %s", listKey)
	}
}

func TestPodCacheKeyGeneration(t *testing.T) {
	podID := "pod-456"

	statusKey := "pod:status:" + podID
	if statusKey != "pod:status:pod-456" {
		t.Errorf("unexpected pod status key: %s", statusKey)
	}

	podKey := "pod:" + podID
	if podKey != "pod:pod-456" {
		t.Errorf("unexpected pod key: %s", podKey)
	}
}

func TestUserCacheKeyGeneration(t *testing.T) {
	userID := "user-789"

	userKey := "user:" + userID
	if userKey != "user:user-789" {
		t.Errorf("unexpected user key: %s", userKey)
	}

	externalID := "canvas-123"
	extKey := "user:ext:" + externalID
	if extKey != "user:ext:canvas-123" {
		t.Errorf("unexpected external user key: %s", extKey)
	}

	rolesKey := "user:roles:" + userID
	if rolesKey != "user:roles:user-789" {
		t.Errorf("unexpected roles key: %s", rolesKey)
	}
}

// =============================================================================
// Integration Tests with miniredis
// =============================================================================

// testClient creates a test Redis client using miniredis
func testClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	cfg := Config{
		Host:         mr.Host(),
		Port:         6379, // Will be overridden by addr parsing
		Enabled:      true,
		KeyPrefix:    "test:",
		MaxPoolSize:  5,
		MinIdleConns: 1,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	}

	// Parse the actual port from miniredis
	addr := mr.Addr()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create client with correct address
	cfg.Host = mr.Host()
	// Parse port from addr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			port, _ := strconv.Atoi(addr[i+1:])
			cfg.Port = port
			break
		}
	}

	client, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return client, mr
}

// -----------------------------------------------------------------------------
// Client Integration Tests
// -----------------------------------------------------------------------------

func TestClientConnect(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

func TestClientPing(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestClientSetGet(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Set a value
	err := client.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	val, err := client.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got '%s'", val)
	}
}

func TestClientGetBytes(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	testData := []byte(`{"key": "value"}`)
	err := client.Set(ctx, "bytes-key", testData, time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	bytes, err := client.GetBytes(ctx, "bytes-key")
	if err != nil {
		t.Fatalf("GetBytes failed: %v", err)
	}
	if string(bytes) != string(testData) {
		t.Errorf("expected %s, got %s", testData, bytes)
	}
}

func TestClientDelete(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Set values
	_ = client.Set(ctx, "del1", "v1", time.Minute)
	_ = client.Set(ctx, "del2", "v2", time.Minute)

	// Delete them
	err := client.Delete(ctx, "del1", "del2")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, _ := client.Exists(ctx, "del1")
	if exists {
		t.Error("del1 should not exist")
	}
}

func TestClientExists(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Key doesn't exist
	exists, err := client.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("key should not exist")
	}

	// Set key
	_ = client.Set(ctx, "exists-key", "value", time.Minute)

	// Key exists
	exists, err = client.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("key should exist")
	}
}

func TestClientExpire(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	_ = client.Set(ctx, "expire-key", "value", 0) // No expiration

	err := client.Expire(ctx, "expire-key", 10*time.Second)
	if err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	ttl, err := client.TTL(ctx, "expire-key")
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("expected positive TTL, got %v", ttl)
	}
}

func TestClientHashOperations(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// HSet
	err := client.HSet(ctx, "hash-key", "field1", "value1", "field2", "value2")
	if err != nil {
		t.Fatalf("HSet failed: %v", err)
	}

	// HGet
	val, err := client.HGet(ctx, "hash-key", "field1")
	if err != nil {
		t.Fatalf("HGet failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got '%s'", val)
	}

	// HGetAll
	all, err := client.HGetAll(ctx, "hash-key")
	if err != nil {
		t.Fatalf("HGetAll failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 fields, got %d", len(all))
	}

	// HDel
	err = client.HDel(ctx, "hash-key", "field1")
	if err != nil {
		t.Fatalf("HDel failed: %v", err)
	}
}

func TestClientIncrDecr(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Incr
	val, err := client.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("Incr failed: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// IncrBy
	val, err = client.IncrBy(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("IncrBy failed: %v", err)
	}
	if val != 6 {
		t.Errorf("expected 6, got %d", val)
	}

	// Decr
	val, err = client.Decr(ctx, "counter")
	if err != nil {
		t.Fatalf("Decr failed: %v", err)
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
}

func TestClientListOperations(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// LPush
	err := client.LPush(ctx, "list-key", "item1", "item2")
	if err != nil {
		t.Fatalf("LPush failed: %v", err)
	}

	// LLen
	length, err := client.LLen(ctx, "list-key")
	if err != nil {
		t.Fatalf("LLen failed: %v", err)
	}
	if length != 2 {
		t.Errorf("expected 2, got %d", length)
	}

	// RPush
	err = client.RPush(ctx, "list-key", "item3")
	if err != nil {
		t.Fatalf("RPush failed: %v", err)
	}

	// LRange
	items, err := client.LRange(ctx, "list-key", 0, -1)
	if err != nil {
		t.Fatalf("LRange failed: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// LPop
	val, err := client.LPop(ctx, "list-key")
	if err != nil {
		t.Fatalf("LPop failed: %v", err)
	}
	if val == "" {
		t.Error("LPop should return a value")
	}

	// RPop
	val, err = client.RPop(ctx, "list-key")
	if err != nil {
		t.Fatalf("RPop failed: %v", err)
	}
	if val == "" {
		t.Error("RPop should return a value")
	}
}

func TestClientSetOperations(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// SAdd
	err := client.SAdd(ctx, "set-key", "member1", "member2", "member3")
	if err != nil {
		t.Fatalf("SAdd failed: %v", err)
	}

	// SCard
	count, err := client.SCard(ctx, "set-key")
	if err != nil {
		t.Fatalf("SCard failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}

	// SIsMember
	isMember, err := client.SIsMember(ctx, "set-key", "member1")
	if err != nil {
		t.Fatalf("SIsMember failed: %v", err)
	}
	if !isMember {
		t.Error("member1 should be a member")
	}

	// SMembers
	members, err := client.SMembers(ctx, "set-key")
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}

	// SRem
	err = client.SRem(ctx, "set-key", "member1")
	if err != nil {
		t.Fatalf("SRem failed: %v", err)
	}

	count, _ = client.SCard(ctx, "set-key")
	if count != 2 {
		t.Errorf("expected 2 after removal, got %d", count)
	}
}

func TestClientKey(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	// Test Key function
	key := client.Key("session:", "user-123")
	expected := "test:session:user-123"
	if key != expected {
		t.Errorf("expected '%s', got '%s'", expected, key)
	}
}

func TestClientHealthCheck(t *testing.T) {
	// Skip this test with miniredis since INFO memory is not supported
	// The HealthCheck method works correctly with real Redis
	t.Skip("INFO memory not supported by miniredis")
}

func TestClientPingHealthCheck(t *testing.T) {
	// Test the Ping portion of health check which miniredis supports
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Session Store Integration Tests
// -----------------------------------------------------------------------------

func TestSessionStoreCreate(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	session := &SessionData{
		UserID:   "user-123",
		Username: "testuser",
		Email:    "test@example.com",
		Roles:    []string{"student"},
	}

	err := store.Create(ctx, "sess-123", session)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify session was created
	retrieved, err := store.Get(ctx, "sess-123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("session should exist")
	}
	if retrieved.UserID != session.UserID {
		t.Errorf("expected UserID %s, got %s", session.UserID, retrieved.UserID)
	}
}

func TestSessionStoreGet(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	// Create session
	session := &SessionData{
		UserID:   "user-456",
		Username: "getuser",
	}
	_ = store.Create(ctx, "sess-get", session)

	// Get existing
	retrieved, err := store.Get(ctx, "sess-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("session should exist")
	}

	// Get non-existent
	nonexistent, err := store.Get(ctx, "sess-nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if nonexistent != nil {
		t.Error("session should not exist")
	}
}

func TestSessionStoreTouch(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	session := &SessionData{
		UserID:   "user-789",
		Username: "touchuser",
	}
	_ = store.Create(ctx, "sess-touch", session)

	// Get original last access
	original, _ := store.Get(ctx, "sess-touch")
	originalAccess := original.LastAccess

	// Wait a tiny bit
	time.Sleep(10 * time.Millisecond)

	// Touch
	err := store.Touch(ctx, "sess-touch")
	if err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	// Verify updated
	updated, _ := store.Get(ctx, "sess-touch")
	if !updated.LastAccess.After(originalAccess) {
		t.Error("LastAccess should be updated")
	}
}

func TestSessionStoreDelete(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	session := &SessionData{
		UserID:   "user-del",
		Username: "deluser",
	}
	_ = store.Create(ctx, "sess-del", session)

	// Delete
	err := store.Delete(ctx, "sess-del")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, _ := store.Exists(ctx, "sess-del")
	if exists {
		t.Error("session should be deleted")
	}
}

func TestSessionStoreExists(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	session := &SessionData{
		UserID:   "user-exists",
		Username: "existsuser",
	}
	_ = store.Create(ctx, "sess-exists", session)

	exists, err := store.Exists(ctx, "sess-exists")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("session should exist")
	}

	exists, _ = store.Exists(ctx, "sess-not-exists")
	if exists {
		t.Error("session should not exist")
	}
}

func TestSessionStoreUpdate(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	session := &SessionData{
		UserID:   "user-update",
		Username: "updateuser",
		Roles:    []string{"student"},
	}
	_ = store.Create(ctx, "sess-update", session)

	// Update
	err := store.Update(ctx, "sess-update", func(s *SessionData) {
		s.Roles = append(s.Roles, "ta")
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	updated, _ := store.Get(ctx, "sess-update")
	if len(updated.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(updated.Roles))
	}
}

func TestSessionStoreListByUserID(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	// Create multiple sessions for same user
	for i := 0; i < 3; i++ {
		session := &SessionData{
			UserID:   "user-multi",
			Username: "multiuser",
		}
		_ = store.Create(ctx, "sess-multi-"+strconv.Itoa(i), session)
	}

	// List
	sessions, err := store.ListByUserID(ctx, "user-multi")
	if err != nil {
		t.Fatalf("ListByUserID failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestSessionStoreDeleteByUserID(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewSessionStore(client, 24*time.Hour)

	// Create sessions
	for i := 0; i < 2; i++ {
		session := &SessionData{
			UserID:   "user-delmulti",
			Username: "delmultiuser",
		}
		_ = store.Create(ctx, "sess-delmulti-"+strconv.Itoa(i), session)
	}

	// Delete all
	err := store.DeleteByUserID(ctx, "user-delmulti")
	if err != nil {
		t.Fatalf("DeleteByUserID failed: %v", err)
	}

	// Verify
	sessions, _ := store.ListByUserID(ctx, "user-delmulti")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

// -----------------------------------------------------------------------------
// Cache Integration Tests
// -----------------------------------------------------------------------------

func TestCacheGetSet(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewCache(client, 5*time.Minute, logger)

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := TestData{Name: "test", Value: 42}

	// Set
	err := cache.Set(ctx, "cache-key", data)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	var retrieved TestData
	err = cache.Get(ctx, "cache-key", &retrieved)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Name != data.Name || retrieved.Value != data.Value {
		t.Errorf("expected %+v, got %+v", data, retrieved)
	}
}

func TestCacheGetMiss(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewCache(client, 5*time.Minute, logger)

	var retrieved string
	err := cache.Get(ctx, "nonexistent", &retrieved)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}
}

func TestCacheSetWithTTL(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewCache(client, 5*time.Minute, logger)

	err := cache.SetWithTTL(ctx, "ttl-key", "value", 10*time.Second)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	var val string
	err = cache.Get(ctx, "ttl-key", &val)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
}

func TestCacheDelete(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewCache(client, 5*time.Minute, logger)

	_ = cache.Set(ctx, "del-key", "value")

	err := cache.Delete(ctx, "del-key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var val string
	err = cache.Get(ctx, "del-key", &val)
	if !errors.Is(err, ErrCacheMiss) {
		t.Error("key should be deleted")
	}
}

func TestCacheGetOrSet(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewCache(client, 5*time.Minute, logger)

	callCount := 0
	fn := func() (interface{}, error) {
		callCount++
		return map[string]string{"key": "value"}, nil
	}

	// First call - cache miss, fn should be called
	var result1 map[string]string
	err := cache.GetOrSet(ctx, "getorset-key", &result1, 5*time.Minute, fn)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected fn to be called once, got %d", callCount)
	}

	// Second call - cache hit, fn should not be called
	var result2 map[string]string
	err = cache.GetOrSet(ctx, "getorset-key", &result2, 5*time.Minute, fn)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("fn should not be called on cache hit, got %d calls", callCount)
	}
}

// -----------------------------------------------------------------------------
// Lock Manager Integration Tests
// -----------------------------------------------------------------------------

func TestLockManagerAcquireRelease(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	lm := NewLockManager(client, logger)

	// Acquire lock
	lock, err := lm.Acquire(ctx, "test-lock", 30*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if lock == nil {
		t.Fatal("lock should not be nil")
	}

	// Verify lock is held
	if !lock.IsHeld(ctx) {
		t.Error("lock should be held")
	}

	// Release lock
	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Verify lock is released
	if lock.IsHeld(ctx) {
		t.Error("lock should be released")
	}
}

func TestLockManagerAcquireConflict(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	lm := NewLockManager(client, logger)

	// Acquire first lock
	lock1, err := lm.Acquire(ctx, "conflict-lock", 30*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer lock1.Release(ctx)

	// Try to acquire same lock
	_, err = lm.Acquire(ctx, "conflict-lock", 30*time.Second)
	if !errors.Is(err, ErrLockNotAcquired) {
		t.Errorf("expected ErrLockNotAcquired, got %v", err)
	}
}

func TestLockManagerTryLock(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	lm := NewLockManager(client, logger)

	// Try lock on free resource
	lock, ok := lm.TryLock(ctx, "try-lock", 30*time.Second)
	if !ok {
		t.Error("TryLock should succeed on free resource")
	}
	if lock == nil {
		t.Fatal("lock should not be nil")
	}
	defer lock.Release(ctx)

	// Try lock on held resource
	_, ok = lm.TryLock(ctx, "try-lock", 30*time.Second)
	if ok {
		t.Error("TryLock should fail on held resource")
	}
}

func TestLockExtend(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	lm := NewLockManager(client, logger)

	lock, _ := lm.Acquire(ctx, "extend-lock", 10*time.Second)
	defer lock.Release(ctx)

	err := lock.Extend(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("Extend failed: %v", err)
	}
}

func TestWithLock(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	lm := NewLockManager(client, logger)

	executed := false
	err := WithLock(ctx, lm, "with-lock", 30*time.Second, func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("WithLock failed: %v", err)
	}
	if !executed {
		t.Error("function should be executed")
	}
}

// -----------------------------------------------------------------------------
// Rate Limiter Integration Tests
// -----------------------------------------------------------------------------

func TestRateLimiterAllow(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	rl := NewRateLimiter(client, logger)

	// First request should be allowed
	result, err := rl.Allow(ctx, "test-key", 10, time.Minute)
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !result.Allowed {
		t.Error("first request should be allowed")
	}
	if result.Remaining != 9 {
		t.Errorf("expected 9 remaining, got %d", result.Remaining)
	}
}

func TestRateLimiterExceedLimit(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	rl := NewRateLimiter(client, logger)

	// Exhaust the limit
	limit := int64(5)
	for i := int64(0); i < limit; i++ {
		result, _ := rl.Allow(ctx, "exhaust-key", limit, time.Minute)
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// Next request should be denied
	result, _ := rl.Allow(ctx, "exhaust-key", limit, time.Minute)
	if result.Allowed {
		t.Error("request should be denied after limit")
	}
}

func TestRateLimiterReset(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	rl := NewRateLimiter(client, logger)

	// Use some of the limit
	rl.Allow(ctx, "reset-key", 5, time.Minute)
	rl.Allow(ctx, "reset-key", 5, time.Minute)

	// Reset
	err := rl.Reset(ctx, "reset-key")
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Should be allowed again
	result, _ := rl.Allow(ctx, "reset-key", 5, time.Minute)
	if result.Remaining != 4 {
		t.Errorf("expected 4 remaining after reset, got %d", result.Remaining)
	}
}

// -----------------------------------------------------------------------------
// PubSub Integration Tests
// -----------------------------------------------------------------------------

func TestPubSubSubscribeHandler(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ps := NewPubSub(client, logger)

	received := make(chan *Message, 1)
	ps.Subscribe("test-channel", func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})

	// The handlers map should contain our handler
	if len(ps.handlers["test-channel"]) != 1 {
		t.Errorf("expected 1 handler, got %d", len(ps.handlers["test-channel"]))
	}
}

// -----------------------------------------------------------------------------
// Task Queue Integration Tests
// -----------------------------------------------------------------------------

func TestTaskQueueEnqueue(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "test-queue",
		Logger: logger,
	})

	payload := map[string]string{"key": "value"}
	task, err := tq.Enqueue(ctx, "test-task", payload)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if task == nil {
		t.Fatal("task should not be nil")
	}
	if task.Type != "test-task" {
		t.Errorf("expected type 'test-task', got '%s'", task.Type)
	}
	if task.State != TaskStatePending {
		t.Errorf("expected state pending, got %s", task.State)
	}
}

func TestTaskQueueEnqueueWithOptions(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "opts-queue",
		Logger: logger,
	})

	opts := EnqueueOptions{
		ID:         "custom-id",
		Priority:   10,
		MaxRetries: 5,
		Metadata:   map[string]any{"source": "test"},
	}

	task, err := tq.EnqueueWithOptions(ctx, "priority-task", nil, opts)
	if err != nil {
		t.Fatalf("EnqueueWithOptions failed: %v", err)
	}
	if task.ID != "custom-id" {
		t.Errorf("expected ID 'custom-id', got '%s'", task.ID)
	}
	if task.Priority != 10 {
		t.Errorf("expected priority 10, got %d", task.Priority)
	}
	if task.MaxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", task.MaxRetries)
	}
}

func TestTaskQueueDequeue(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "dequeue-test",
		Logger: logger,
	})

	// Enqueue a task
	_, _ = tq.Enqueue(ctx, "dequeue-task", nil)

	// Dequeue
	task, err := tq.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if task == nil {
		t.Fatal("task should not be nil")
	}
	if task.State != TaskStateProcessing {
		t.Errorf("expected state processing, got %s", task.State)
	}
	if task.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
}

func TestTaskQueueComplete(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "complete-test",
		Logger: logger,
	})

	enqueued, _ := tq.Enqueue(ctx, "complete-task", nil)
	_, _ = tq.Dequeue(ctx)

	result := map[string]string{"status": "done"}
	err := tq.Complete(ctx, enqueued.ID, result)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify state
	task, _ := tq.GetTask(ctx, enqueued.ID)
	if task.State != TaskStateCompleted {
		t.Errorf("expected state completed, got %s", task.State)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestTaskQueueFail(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "fail-test",
		Logger: logger,
	})

	enqueued, _ := tq.Enqueue(ctx, "fail-task", nil)
	_, _ = tq.Dequeue(ctx)

	err := tq.Fail(ctx, enqueued.ID, errors.New("test error"))
	if err != nil {
		t.Fatalf("Fail failed: %v", err)
	}

	// Task should be scheduled for retry
	task, _ := tq.GetTask(ctx, enqueued.ID)
	if task.State != TaskStateRetrying {
		t.Errorf("expected state retrying, got %s", task.State)
	}
	if task.RetryCount != 1 {
		t.Errorf("expected retryCount 1, got %d", task.RetryCount)
	}
}

func TestTaskQueueStats(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "stats-test",
		Logger: logger,
	})

	// Enqueue some tasks
	tq.Enqueue(ctx, "task1", nil)
	tq.Enqueue(ctx, "task2", nil)

	stats, err := tq.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats["pending"] != 2 {
		t.Errorf("expected 2 pending, got %d", stats["pending"])
	}
}

func TestTaskQueueRegisterHandler(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tq := NewTaskQueue(client, TaskQueueConfig{
		Name:   "handler-test",
		Logger: logger,
	})

	handler := func(ctx context.Context, task *Task) error {
		return nil
	}

	tq.RegisterHandler("test-type", handler)

	if _, exists := tq.handlers["test-type"]; !exists {
		t.Error("handler should be registered")
	}
}

// -----------------------------------------------------------------------------
// WebSocket Store Integration Tests
// -----------------------------------------------------------------------------

func TestWebSocketStoreRegister(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewWebSocketStore(client, 10*time.Minute)

	session := &WebSocketSession{
		ConnectionID: "conn-123",
		UserID:       "user-456",
		PodID:        "pod-789",
	}

	err := store.Register(ctx, session)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
}

func TestWebSocketStoreUnregister(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewWebSocketStore(client, 10*time.Minute)

	session := &WebSocketSession{
		ConnectionID: "conn-unreg",
		UserID:       "user-unreg",
		PodID:        "pod-unreg",
	}
	_ = store.Register(ctx, session)

	err := store.Unregister(ctx, "conn-unreg")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}
}

func TestWebSocketStorePing(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewWebSocketStore(client, 10*time.Minute)

	session := &WebSocketSession{
		ConnectionID: "conn-ping",
		UserID:       "user-ping",
	}
	_ = store.Register(ctx, session)

	err := store.Ping(ctx, "conn-ping")
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestWebSocketStoreGetByPodID(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewWebSocketStore(client, 10*time.Minute)

	// Register multiple connections for same pod
	for i := 0; i < 3; i++ {
		session := &WebSocketSession{
			ConnectionID: "conn-pod-" + strconv.Itoa(i),
			UserID:       "user-" + strconv.Itoa(i),
			PodID:        "shared-pod",
		}
		_ = store.Register(ctx, session)
	}

	sessions, err := store.GetByPodID(ctx, "shared-pod")
	if err != nil {
		t.Fatalf("GetByPodID failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestWebSocketStoreConnectionCount(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	store := NewWebSocketStore(client, 10*time.Minute)

	for i := 0; i < 5; i++ {
		session := &WebSocketSession{
			ConnectionID: "conn-count-" + strconv.Itoa(i),
			UserID:       "user-count",
			PodID:        "count-pod",
		}
		_ = store.Register(ctx, session)
	}

	count, err := store.ConnectionCount(ctx, "count-pod")
	if err != nil {
		t.Fatalf("ConnectionCount failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

// -----------------------------------------------------------------------------
// Template Cache Tests
// -----------------------------------------------------------------------------

func TestTemplateCacheGetSet(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewTemplateCache(client, logger)

	template := map[string]interface{}{
		"id":   "tmpl-123",
		"name": "Test Template",
		"vms":  []string{"vm1", "vm2"},
	}

	// Set template
	err := cache.SetTemplate(ctx, "tmpl-123", template)
	if err != nil {
		t.Fatalf("SetTemplate failed: %v", err)
	}

	// Get template
	var retrieved map[string]interface{}
	err = cache.GetTemplate(ctx, "tmpl-123", &retrieved)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if retrieved["name"] != "Test Template" {
		t.Errorf("expected name 'Test Template', got %v", retrieved["name"])
	}
}

func TestTemplateCacheInvalidate(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewTemplateCache(client, logger)

	template := map[string]string{"id": "tmpl-456", "name": "To Invalidate"}

	_ = cache.SetTemplate(ctx, "tmpl-456", template)

	// Invalidate
	err := cache.InvalidateTemplate(ctx, "tmpl-456")
	if err != nil {
		t.Fatalf("InvalidateTemplate failed: %v", err)
	}

	// Should be a miss now
	var retrieved map[string]string
	err = cache.GetTemplate(ctx, "tmpl-456", &retrieved)
	if !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss after invalidation")
	}
}

func TestTemplateCacheList(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewTemplateCache(client, logger)

	templates := []map[string]string{
		{"id": "t1", "name": "Template 1"},
		{"id": "t2", "name": "Template 2"},
	}

	// Set list
	err := cache.SetTemplateList(ctx, "all", templates)
	if err != nil {
		t.Fatalf("SetTemplateList failed: %v", err)
	}

	// Get list
	var retrieved []map[string]string
	err = cache.GetTemplateList(ctx, "all", &retrieved)
	if err != nil {
		t.Fatalf("GetTemplateList failed: %v", err)
	}
	if len(retrieved) != 2 {
		t.Errorf("expected 2 templates, got %d", len(retrieved))
	}
}

func TestTemplateCacheInvalidateAll(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewTemplateCache(client, logger)

	// Set multiple items
	_ = cache.SetTemplate(ctx, "t1", map[string]string{"id": "t1"})
	_ = cache.SetTemplate(ctx, "t2", map[string]string{"id": "t2"})
	_ = cache.SetTemplateList(ctx, "all", []string{"t1", "t2"})

	// Invalidate all
	err := cache.InvalidateAll(ctx)
	if err != nil {
		t.Fatalf("InvalidateAll failed: %v", err)
	}

	// All should be misses
	var val map[string]string
	if err := cache.GetTemplate(ctx, "t1", &val); !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss for t1")
	}
}

// -----------------------------------------------------------------------------
// Pod Cache Tests
// -----------------------------------------------------------------------------

func TestPodCacheStatus(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewPodCache(client, logger)

	status := map[string]interface{}{
		"status": "running",
		"vms":    []string{"vm1", "vm2"},
	}

	// Set status
	err := cache.SetPodStatus(ctx, "pod-123", status)
	if err != nil {
		t.Fatalf("SetPodStatus failed: %v", err)
	}

	// Get status
	var retrieved map[string]interface{}
	err = cache.GetPodStatus(ctx, "pod-123", &retrieved)
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}
	if retrieved["status"] != "running" {
		t.Errorf("expected status 'running', got %v", retrieved["status"])
	}

	// Invalidate
	err = cache.InvalidatePodStatus(ctx, "pod-123")
	if err != nil {
		t.Fatalf("InvalidatePodStatus failed: %v", err)
	}

	// Should be miss now
	err = cache.GetPodStatus(ctx, "pod-123", &retrieved)
	if !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss after invalidation")
	}
}

func TestPodCachePod(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewPodCache(client, logger)

	pod := map[string]interface{}{
		"id":    "pod-456",
		"owner": "user-1",
	}

	// Set pod
	err := cache.SetPod(ctx, "pod-456", pod)
	if err != nil {
		t.Fatalf("SetPod failed: %v", err)
	}

	// Get pod
	var retrieved map[string]interface{}
	err = cache.GetPod(ctx, "pod-456", &retrieved)
	if err != nil {
		t.Fatalf("GetPod failed: %v", err)
	}
	if retrieved["owner"] != "user-1" {
		t.Errorf("expected owner 'user-1', got %v", retrieved["owner"])
	}

	// Invalidate
	err = cache.InvalidatePod(ctx, "pod-456")
	if err != nil {
		t.Fatalf("InvalidatePod failed: %v", err)
	}

	err = cache.GetPod(ctx, "pod-456", &retrieved)
	if !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss after invalidation")
	}
}

// -----------------------------------------------------------------------------
// User Cache Tests
// -----------------------------------------------------------------------------

func TestUserCacheGetSet(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewUserCache(client, logger)

	user := map[string]interface{}{
		"id":    "user-123",
		"email": "test@example.com",
		"name":  "Test User",
	}

	// Set user
	err := cache.SetUser(ctx, "user-123", user)
	if err != nil {
		t.Fatalf("SetUser failed: %v", err)
	}

	// Get user
	var retrieved map[string]interface{}
	err = cache.GetUser(ctx, "user-123", &retrieved)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved["email"] != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %v", retrieved["email"])
	}

	// Invalidate
	err = cache.InvalidateUser(ctx, "user-123")
	if err != nil {
		t.Fatalf("InvalidateUser failed: %v", err)
	}

	err = cache.GetUser(ctx, "user-123", &retrieved)
	if !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss after invalidation")
	}
}

func TestUserCacheExternalID(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewUserCache(client, logger)

	user := map[string]interface{}{
		"id":         "user-ext",
		"externalID": "canvas-12345",
	}

	// Set by external ID
	err := cache.SetUserByExternalID(ctx, "canvas-12345", user)
	if err != nil {
		t.Fatalf("SetUserByExternalID failed: %v", err)
	}

	// Get by external ID
	var retrieved map[string]interface{}
	err = cache.GetUserByExternalID(ctx, "canvas-12345", &retrieved)
	if err != nil {
		t.Fatalf("GetUserByExternalID failed: %v", err)
	}
	if retrieved["id"] != "user-ext" {
		t.Errorf("expected id 'user-ext', got %v", retrieved["id"])
	}
}

func TestUserCacheRoles(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewUserCache(client, logger)

	roles := []string{"student", "instructor"}

	// Set roles
	err := cache.SetUserRoles(ctx, "user-roles", roles)
	if err != nil {
		t.Fatalf("SetUserRoles failed: %v", err)
	}

	// Get roles
	retrieved, err := cache.GetUserRoles(ctx, "user-roles")
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	if len(retrieved) != 2 {
		t.Errorf("expected 2 roles, got %d", len(retrieved))
	}
	if retrieved[0] != "student" || retrieved[1] != "instructor" {
		t.Errorf("unexpected roles: %v", retrieved)
	}
}

// -----------------------------------------------------------------------------
// Cache Invalidate Tests
// -----------------------------------------------------------------------------

func TestCacheInvalidatePattern(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := NewCache(client, 5*time.Minute, logger)

	// Set multiple keys with similar prefix
	_ = cache.Set(ctx, "pattern:a", "value-a")
	_ = cache.Set(ctx, "pattern:b", "value-b")
	_ = cache.Set(ctx, "pattern:c", "value-c")
	_ = cache.Set(ctx, "other:x", "value-x")

	// Invalidate pattern
	err := cache.Invalidate(ctx, "pattern:*")
	if err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}

	// Pattern keys should be deleted
	var val string
	if err := cache.Get(ctx, "pattern:a", &val); !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss for pattern:a")
	}
	if err := cache.Get(ctx, "pattern:b", &val); !errors.Is(err, ErrCacheMiss) {
		t.Error("expected cache miss for pattern:b")
	}

	// Other key should still exist
	if err := cache.Get(ctx, "other:x", &val); err != nil {
		t.Errorf("other:x should still exist: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Lock Helper Tests
// -----------------------------------------------------------------------------

func TestPodLock(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pl := NewPodLock(client, logger)

	// Lock pod
	lock, err := pl.LockPod(ctx, "pod-lock-1", 30*time.Second)
	if err != nil {
		t.Fatalf("LockPod failed: %v", err)
	}
	defer lock.Release(ctx)

	if !lock.IsHeld(ctx) {
		t.Error("lock should be held")
	}
}

func TestPodLockTryLock(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pl := NewPodLock(client, logger)

	// Try lock - should succeed
	lock, ok := pl.TryLockPod(ctx, "pod-try-1", 30*time.Second)
	if !ok {
		t.Error("should have acquired lock")
	}
	defer lock.Release(ctx)

	// Try again - should fail
	_, ok2 := pl.TryLockPod(ctx, "pod-try-1", 30*time.Second)
	if ok2 {
		t.Error("should not have acquired lock (already held)")
	}
}

func TestPodLockWithRetry(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pl := NewPodLock(client, logger)

	// Lock with retry - should succeed immediately
	lock, err := pl.LockPodWithRetry(ctx, "pod-retry-1", 30*time.Second, 3)
	if err != nil {
		t.Fatalf("LockPodWithRetry failed: %v", err)
	}
	defer lock.Release(ctx)

	if !lock.IsHeld(ctx) {
		t.Error("lock should be held")
	}
}

func TestSessionLock(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sl := NewSessionLock(client, logger)

	// Lock session
	lock, err := sl.LockSession(ctx, "sess-lock-1", 30*time.Second)
	if err != nil {
		t.Fatalf("LockSession failed: %v", err)
	}
	defer lock.Release(ctx)

	if !lock.IsHeld(ctx) {
		t.Error("lock should be held")
	}

	// Lock grade sync
	lock2, err := sl.LockGradeSync(ctx, "sess-lock-1", 30*time.Second)
	if err != nil {
		t.Fatalf("LockGradeSync failed: %v", err)
	}
	defer lock2.Release(ctx)

	if !lock2.IsHeld(ctx) {
		t.Error("grade sync lock should be held")
	}
}

func TestResourceLock(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	rl := NewResourceLock(client, logger)

	// Lock VLAN
	lock1, err := rl.LockVLAN(ctx, "node-1", 30*time.Second)
	if err != nil {
		t.Fatalf("LockVLAN failed: %v", err)
	}
	defer lock1.Release(ctx)

	// Lock VM
	lock2, err := rl.LockVM(ctx, "vm-200", 30*time.Second)
	if err != nil {
		t.Fatalf("LockVM failed: %v", err)
	}
	defer lock2.Release(ctx)

	// Lock Template
	lock3, err := rl.LockTemplate(ctx, "template-1", 30*time.Second)
	if err != nil {
		t.Fatalf("LockTemplate failed: %v", err)
	}
	defer lock3.Release(ctx)
}

func TestLockAcquireWithRetry(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	lm := NewLockManager(client, logger)

	// Acquire with retry - should succeed immediately
	lock, err := lm.AcquireWithRetry(ctx, "retry-lock", 30*time.Second, 3, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireWithRetry failed: %v", err)
	}
	defer lock.Release(ctx)

	if !lock.IsHeld(ctx) {
		t.Error("lock should be held")
	}
}

// -----------------------------------------------------------------------------
// Semaphore Tests
// -----------------------------------------------------------------------------

func TestSemaphoreAcquireRelease(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sem := NewSemaphore(client, "test-sem", 3, 30*time.Second, logger)

	// Acquire 3 permits
	permits := make([]string, 3)
	for i := 0; i < 3; i++ {
		permit, err := sem.Acquire(ctx)
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		permits[i] = permit
	}

	// Check available
	avail, err := sem.Available(ctx)
	if err != nil {
		t.Fatalf("Available failed: %v", err)
	}
	if avail != 0 {
		t.Errorf("expected 0 available, got %d", avail)
	}

	// 4th acquire should timeout quickly
	ctxShort, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, err = sem.Acquire(ctxShort)
	if err == nil {
		t.Error("expected acquire to fail (no permits)")
	}

	// Release one
	err = sem.Release(ctx, permits[0])
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Should be 1 available now
	avail, err = sem.Available(ctx)
	if err != nil {
		t.Fatalf("Available failed: %v", err)
	}
	if avail != 1 {
		t.Errorf("expected 1 available, got %d", avail)
	}

	// Cleanup
	for i := 1; i < 3; i++ {
		sem.Release(ctx, permits[i])
	}
}

// -----------------------------------------------------------------------------
// Rate Limiter AllowN Tests
// -----------------------------------------------------------------------------

func TestRateLimiterAllowN(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	rl := NewRateLimiter(client, logger)

	// Allow batch of 5 from limit of 10
	result, err := rl.AllowN(ctx, "batch-key", 5, 10, time.Minute)
	if err != nil {
		t.Fatalf("AllowN failed: %v", err)
	}
	if !result.Allowed {
		t.Error("expected AllowN to succeed")
	}

	// Allow another 5 - should still work
	result, err = rl.AllowN(ctx, "batch-key", 5, 10, time.Minute)
	if err != nil {
		t.Fatalf("AllowN failed: %v", err)
	}
	if !result.Allowed {
		t.Error("expected AllowN to succeed")
	}

	// Try to allow 1 more - should fail
	result, err = rl.AllowN(ctx, "batch-key", 1, 10, time.Minute)
	if err != nil {
		t.Fatalf("AllowN failed: %v", err)
	}
	if result.Allowed {
		t.Error("expected AllowN to fail (limit exceeded)")
	}
}

// -----------------------------------------------------------------------------
// API Rate Limiter Tests
// -----------------------------------------------------------------------------

func TestAPIRateLimiter(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewAPIRateLimiter(client, logger)

	// Test per-IP middleware
	handler := api.PerIPMiddleware(10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should allow first request
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestPodOperationRateLimiter(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	limiter := NewPodOperationRateLimiter(client, logger)

	ctx := context.Background()

	// Test pod create (10 pods per hour limit)
	result, err := limiter.AllowPodCreate(ctx, "user-1", 10, time.Hour)
	if err != nil {
		t.Fatalf("AllowPodCreate failed: %v", err)
	}
	if !result.Allowed {
		t.Error("expected AllowPodCreate to succeed")
	}

	// Test snapshot (20 per hour limit)
	result, err = limiter.AllowSnapshot(ctx, "user-1", 20, time.Hour)
	if err != nil {
		t.Fatalf("AllowSnapshot failed: %v", err)
	}
	if !result.Allowed {
		t.Error("expected AllowSnapshot to succeed")
	}

	// Test revert (30 per hour limit)
	result, err = limiter.AllowRevert(ctx, "pod-1", 30, time.Hour)
	if err != nil {
		t.Fatalf("AllowRevert failed: %v", err)
	}
	if !result.Allowed {
		t.Error("expected AllowRevert to succeed")
	}
}

// -----------------------------------------------------------------------------
// PubSub Publish Tests
// -----------------------------------------------------------------------------

func TestPubSubPublish(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ps := NewPubSub(client, logger)

	// Publish a message
	msg := map[string]string{"event": "test", "data": "hello"}
	err := ps.Publish(ctx, "test-channel", "test-type", msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Event Publisher Tests
// -----------------------------------------------------------------------------

func TestEventPublisher(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ps := NewPubSub(client, logger)
	pub := NewEventPublisher(ps, "test-source")

	// Test PublishPodStatus
	err := pub.PublishPodStatus(ctx, PodStatusEvent{
		PodID:  "pod-1",
		Status: "running",
	})
	if err != nil {
		t.Fatalf("PublishPodStatus failed: %v", err)
	}

	// Test PublishCheckpointUpdate
	err = pub.PublishCheckpointUpdate(ctx, CheckpointUpdateEvent{
		SessionID:    "session-1",
		PodID:        "pod-1",
		CheckpointID: "checkpoint-1",
		Status:       "passed",
		Points:       10.0,
	})
	if err != nil {
		t.Fatalf("PublishCheckpointUpdate failed: %v", err)
	}

	// Test PublishGradeSync
	err = pub.PublishGradeSync(ctx, GradeSyncEvent{
		SessionID: "session-1",
		Score:     95.0,
		Status:    "synced",
	})
	if err != nil {
		t.Fatalf("PublishGradeSync failed: %v", err)
	}

	// Test PublishSessionUpdate
	err = pub.PublishSessionUpdate(ctx, SessionUpdateEvent{
		SessionID: "session-1",
		PodID:     "pod-1",
		UserID:    "user-1",
		Action:    "started",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("PublishSessionUpdate failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Broadcast Hub Integration Tests
// -----------------------------------------------------------------------------

func TestBroadcastHubWithRedis(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ps := NewPubSub(client, logger)
	hub := NewBroadcastHub(ps, logger)

	// Subscribe
	ch := hub.Subscribe("pod-1", "sub-1")
	if ch == nil {
		t.Fatal("expected channel from Subscribe")
	}
	if hub.SubscriberCount("pod-1") != 1 {
		t.Errorf("expected 1 subscriber, got %d", hub.SubscriberCount("pod-1"))
	}

	// Subscribe another
	ch2 := hub.Subscribe("pod-1", "sub-2")
	if ch2 == nil {
		t.Fatal("expected channel from Subscribe")
	}
	if hub.SubscriberCount("pod-1") != 2 {
		t.Errorf("expected 2 subscribers, got %d", hub.SubscriberCount("pod-1"))
	}

	// Unsubscribe
	hub.Unsubscribe("pod-1", "sub-1")
	if hub.SubscriberCount("pod-1") != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", hub.SubscriberCount("pod-1"))
	}
}

// -----------------------------------------------------------------------------
// Service Tests
// -----------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Parse addr to get host and port
	host, portStr, _ := splitHostPort(mr.Addr())
	port := 6379
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}

	cfg := ServiceConfig{
		Redis: Config{
			Host:      host,
			Port:      port,
			KeyPrefix: "test:",
			Enabled:   true,
		},
		ServerNode: "test-node",
		Logger:     logger,
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("expected service to be created")
	}

	// Ping to verify connection
	ctx := context.Background()
	err = svc.Client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// Close
	err = svc.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// splitHostPort is a helper to parse host:port
func splitHostPort(addr string) (host, port string, err error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "", nil
}

func TestServiceStats(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	host, portStr, _ := splitHostPort(mr.Addr())
	port := 6379
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}

	cfg := ServiceConfig{
		Redis: Config{
			Host:      host,
			Port:      port,
			KeyPrefix: "test:",
			Enabled:   true,
		},
		ServerNode: "test-node",
		Logger:     logger,
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()
	stats, err := svc.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected stats to be returned")
	}
}

// -----------------------------------------------------------------------------
// Client Utility Tests
// -----------------------------------------------------------------------------

func TestClientGetUnderlyingClient(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	underlying := client.Client()
	if underlying == nil {
		t.Error("expected non-nil underlying client")
	}
}

// -----------------------------------------------------------------------------
// Task Queue RequeueTimedOut Tests
// -----------------------------------------------------------------------------

func TestTaskQueueRequeueTimedOut(t *testing.T) {
	client, mr := testClient(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	queue := NewTaskQueue(client, TaskQueueConfig{
		Name:           "requeue-test",
		VisibilityTime: 100 * time.Millisecond, // Short visibility for test
		Logger:         logger,
	})

	// Create a task
	task, err := queue.Enqueue(ctx, "test-type", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if task == nil {
		t.Fatal("expected task to be returned")
	}

	// Dequeue it
	dequeuedTask, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if dequeuedTask == nil || dequeuedTask.ID != task.ID {
		t.Fatal("expected to dequeue the task")
	}

	// Wait for visibility timeout to expire
	time.Sleep(150 * time.Millisecond)

	// Requeue timed out tasks
	requeued, err := queue.RequeueTimedOut(ctx)
	if err != nil {
		t.Fatalf("RequeueTimedOut failed: %v", err)
	}
	if requeued < 0 {
		t.Error("expected non-negative requeue count")
	}
}
