package nats

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

// startTestServer starts an embedded NATS server for testing
func startTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // Random available port
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create test NATS server: %v", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready for connections")
	}

	return ns, ns.ClientURL()
}

func TestClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ns, url := startTestServer(t)
	defer ns.Shutdown()

	t.Run("connects successfully", func(t *testing.T) {
		cfg := Config{
			URL:            url,
			Name:           "test-client",
			ConnectTimeout: 5 * time.Second,
			ReconnectWait:  1 * time.Second,
			MaxReconnects:  3,
		}

		client, err := NewClient(cfg, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		if !client.IsConnected() {
			t.Error("client should be connected")
		}

		if client.JetStream() == nil {
			t.Error("JetStream should not be nil")
		}

		if client.Stream() == nil {
			t.Error("Stream should not be nil")
		}
	})

	t.Run("creates stream with correct subjects", func(t *testing.T) {
		cfg := Config{
			URL:            url,
			Name:           "stream-test-client",
			ConnectTimeout: 5 * time.Second,
			ReconnectWait:  1 * time.Second,
			MaxReconnects:  3,
		}

		client, err := NewClient(cfg, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		stream := client.Stream()
		info, err := stream.Info(context.Background())
		if err != nil {
			t.Fatalf("failed to get stream info: %v", err)
		}

		if info.Config.Name != events.StreamName {
			t.Errorf("expected stream name %q, got %q", events.StreamName, info.Config.Name)
		}

		// Verify all expected subjects are configured
		expectedSubjects := []string{
			events.SubjectAllEvents,
			events.SubjectAllCheckpoints,
			events.SubjectAllSessions,
			events.SubjectAllGrades,
		}

		for _, expected := range expectedSubjects {
			found := false
			for _, actual := range info.Config.Subjects {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected subject %q not found in stream config", expected)
			}
		}
	})
}

func TestClient_Publish_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ns, url := startTestServer(t)
	defer ns.Shutdown()

	cfg := Config{
		URL:            url,
		Name:           "publish-test-client",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  3,
	}

	client, err := NewClient(cfg, newTestLogger())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	t.Run("Publish raw message", func(t *testing.T) {
		ctx := context.Background()
		subject := "labs.events.pod123.vm1.syscheck"
		data := []byte(`{"test": "data"}`)

		err := client.Publish(ctx, subject, data)
		if err != nil {
			t.Errorf("failed to publish: %v", err)
		}
	})

	t.Run("PublishEvent", func(t *testing.T) {
		ctx := context.Background()
		event := &events.VMEvent{
			MessageHeader: events.NewMessageHeader("test"),
			PodID:         "pod123",
			VMName:        "workstation",
			EventType:     events.EventTypeSyscheck,
			Data:          json.RawMessage(`{"file": "/etc/passwd"}`),
		}

		err := client.PublishEvent(ctx, event)
		if err != nil {
			t.Errorf("failed to publish event: %v", err)
		}
	})

	t.Run("PublishCheckpoint", func(t *testing.T) {
		ctx := context.Background()
		update := &events.CheckpointUpdate{
			MessageHeader: events.NewMessageHeader("test"),
			PodID:         "pod123",
			SessionID:     "session456",
			CheckpointID:  "enable-ufw",
			Action:        events.CheckpointActionPassed,
			Points:        10,
			EarnedPoints:  10,
		}

		err := client.PublishCheckpoint(ctx, update)
		if err != nil {
			t.Errorf("failed to publish checkpoint: %v", err)
		}
	})

	t.Run("PublishSession", func(t *testing.T) {
		ctx := context.Background()
		event := &events.SessionEvent{
			MessageHeader: events.NewMessageHeader("test"),
			SessionID:     "session456",
			PodID:         "pod123",
			UserID:        "user789",
			Action:        events.SessionActionStarted,
			LabTemplate:   "nginx-lab",
		}

		err := client.PublishSession(ctx, event)
		if err != nil {
			t.Errorf("failed to publish session event: %v", err)
		}
	})

	t.Run("PublishGrade", func(t *testing.T) {
		ctx := context.Background()
		update := &events.GradeUpdate{
			MessageHeader: events.NewMessageHeader("test"),
			SessionID:     "session456",
			UserID:        "user789",
			EarnedPoints:  85,
			MaxPoints:     100,
			Percentage:    85.0,
			Passed:        true,
			RequiresSync:  false,
		}

		err := client.PublishGrade(ctx, update)
		if err != nil {
			t.Errorf("failed to publish grade: %v", err)
		}
	})
}

func TestConsumer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ns, url := startTestServer(t)
	defer ns.Shutdown()

	cfg := Config{
		URL:            url,
		Name:           "consumer-test-client",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  3,
	}

	client, err := NewClient(cfg, newTestLogger())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	t.Run("NewConsumer creates consumer", func(t *testing.T) {
		received := make(chan []byte, 1)
		handler := func(ctx context.Context, msg jetstream.Msg) error {
			received <- msg.Data()
			return nil
		}

		consumer, err := NewConsumer(client, ConsumerConfig{
			Name:          "test-consumer",
			Durable:       true,
			FilterSubject: events.SubjectAllEvents,
			MaxDeliver:    3,
		}, handler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create consumer: %v", err)
		}
		defer consumer.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = consumer.Start(ctx)
		if err != nil {
			t.Fatalf("failed to start consumer: %v", err)
		}

		// Publish a message
		testData := []byte(`{"test": "consumer"}`)
		err = client.Publish(context.Background(), "labs.events.pod1.vm1.test", testData)
		if err != nil {
			t.Fatalf("failed to publish test message: %v", err)
		}

		// Wait for message
		select {
		case data := <-received:
			if string(data) != string(testData) {
				t.Errorf("expected %q, got %q", testData, data)
			}
		case <-time.After(2 * time.Second):
			t.Error("timeout waiting for message")
		}
	})

	t.Run("consumer stops gracefully", func(t *testing.T) {
		handler := func(ctx context.Context, msg jetstream.Msg) error {
			return nil
		}

		consumer, err := NewConsumer(client, ConsumerConfig{
			Name:          "stop-test-consumer",
			Durable:       false,
			FilterSubject: events.SubjectAllCheckpoints,
			MaxDeliver:    1,
		}, handler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create consumer: %v", err)
		}

		ctx := context.Background()
		err = consumer.Start(ctx)
		if err != nil {
			t.Fatalf("failed to start consumer: %v", err)
		}

		// Stop should not panic
		consumer.Stop()
	})
}

func TestConsumerFactories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ns, url := startTestServer(t)
	defer ns.Shutdown()

	cfg := Config{
		URL:            url,
		Name:           "factory-test-client",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  3,
	}

	client, err := NewClient(cfg, newTestLogger())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	dummyHandler := func(ctx context.Context, msg jetstream.Msg) error {
		return nil
	}

	t.Run("NewEventStoreConsumer", func(t *testing.T) {
		consumer, err := NewEventStoreConsumer(client, dummyHandler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create event store consumer: %v", err)
		}
		defer consumer.Stop()

		if consumer.name != events.ConsumerEventStore {
			t.Errorf("expected name %q, got %q", events.ConsumerEventStore, consumer.name)
		}
	})

	t.Run("NewCheckpointEvaluatorConsumer", func(t *testing.T) {
		consumer, err := NewCheckpointEvaluatorConsumer(client, dummyHandler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create checkpoint evaluator consumer: %v", err)
		}
		defer consumer.Stop()

		if consumer.name != events.ConsumerCheckpointEval {
			t.Errorf("expected name %q, got %q", events.ConsumerCheckpointEval, consumer.name)
		}
	})

	t.Run("NewWebSocketBroadcastConsumer", func(t *testing.T) {
		consumer, err := NewWebSocketBroadcastConsumer(client, dummyHandler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create websocket broadcast consumer: %v", err)
		}
		defer consumer.Stop()

		if consumer.name != events.ConsumerWebSocketBroadcast {
			t.Errorf("expected name %q, got %q", events.ConsumerWebSocketBroadcast, consumer.name)
		}
	})

	t.Run("NewGradeSyncConsumer", func(t *testing.T) {
		consumer, err := NewGradeSyncConsumer(client, dummyHandler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create grade sync consumer: %v", err)
		}
		defer consumer.Stop()

		if consumer.name != events.ConsumerGradeSync {
			t.Errorf("expected name %q, got %q", events.ConsumerGradeSync, consumer.name)
		}
	})
}

func TestClient_Reconnection_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ns, url := startTestServer(t)

	cfg := Config{
		URL:            url,
		Name:           "reconnect-test-client",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  100 * time.Millisecond,
		MaxReconnects:  5,
	}

	client, err := NewClient(cfg, newTestLogger())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("client should be connected initially")
	}

	// Shutdown server
	ns.Shutdown()

	// Give time for disconnect
	time.Sleep(200 * time.Millisecond)

	// Client should detect disconnection
	if client.IsConnected() {
		t.Error("client should be disconnected after server shutdown")
	}
}

func TestConsumer_MessageHandling_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ns, url := startTestServer(t)
	defer ns.Shutdown()

	cfg := Config{
		URL:            url,
		Name:           "message-test-client",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  3,
	}

	client, err := NewClient(cfg, newTestLogger())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	t.Run("processes multiple messages", func(t *testing.T) {
		var mu sync.Mutex
		received := make([]string, 0)

		handler := func(ctx context.Context, msg jetstream.Msg) error {
			mu.Lock()
			received = append(received, string(msg.Data()))
			mu.Unlock()
			return nil
		}

		consumer, err := NewConsumer(client, ConsumerConfig{
			Name:          "multi-msg-consumer",
			Durable:       true,
			FilterSubject: events.SubjectAllCheckpoints,
			MaxDeliver:    3,
		}, handler, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create consumer: %v", err)
		}
		defer consumer.Stop()

		ctx := context.Background()
		err = consumer.Start(ctx)
		if err != nil {
			t.Fatalf("failed to start consumer: %v", err)
		}

		// Publish multiple messages
		messages := []string{"msg1", "msg2", "msg3"}
		for _, msg := range messages {
			err = client.Publish(ctx, "labs.checkpoints.pod1.cp1", []byte(msg))
			if err != nil {
				t.Fatalf("failed to publish: %v", err)
			}
		}

		// Wait for all messages
		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		if len(received) != len(messages) {
			t.Errorf("expected %d messages, got %d", len(messages), len(received))
		}
		mu.Unlock()
	})
}
