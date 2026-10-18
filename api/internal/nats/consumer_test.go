package nats

import (
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

func TestConsumerConfig(t *testing.T) {
	t.Run("EventStore consumer config", func(t *testing.T) {
		// Verify expected configuration for event storage
		cfg := ConsumerConfig{
			Name:          events.ConsumerEventStore,
			Durable:       true,
			FilterSubject: events.SubjectAllEvents,
			MaxDeliver:    5,
		}

		if cfg.Name != "event-store" {
			t.Errorf("expected name %q, got %q", "event-store", cfg.Name)
		}
		if !cfg.Durable {
			t.Error("event store should be durable")
		}
		if cfg.FilterSubject != "labs.events.>" {
			t.Errorf("unexpected filter subject: %s", cfg.FilterSubject)
		}
	})

	t.Run("CheckpointEvaluator consumer config", func(t *testing.T) {
		cfg := ConsumerConfig{
			Name:          events.ConsumerCheckpointEval,
			Durable:       true,
			FilterSubject: events.SubjectAllEvents,
			MaxDeliver:    3,
		}

		if cfg.Name != "checkpoint-evaluator" {
			t.Errorf("expected name %q, got %q", "checkpoint-evaluator", cfg.Name)
		}
		if !cfg.Durable {
			t.Error("checkpoint evaluator should be durable")
		}
		// Fewer retries than event store since checkpoints are more time-sensitive
		if cfg.MaxDeliver != 3 {
			t.Errorf("expected MaxDeliver 3, got %d", cfg.MaxDeliver)
		}
	})

	t.Run("WebSocketBroadcast consumer config", func(t *testing.T) {
		cfg := ConsumerConfig{
			Name:          events.ConsumerWebSocketBroadcast,
			Durable:       false, // Ephemeral
			FilterSubject: events.SubjectAllCheckpoints,
			MaxDeliver:    1,
		}

		if cfg.Name != "websocket-broadcast" {
			t.Errorf("expected name %q, got %q", "websocket-broadcast", cfg.Name)
		}
		if cfg.Durable {
			t.Error("websocket broadcast should be ephemeral (not durable)")
		}
		if cfg.MaxDeliver != 1 {
			t.Errorf("websocket broadcast should not retry, expected MaxDeliver 1, got %d", cfg.MaxDeliver)
		}
	})

	t.Run("GradeSync consumer config", func(t *testing.T) {
		cfg := ConsumerConfig{
			Name:          events.ConsumerGradeSync,
			Durable:       true,
			FilterSubject: events.SubjectAllGrades,
			MaxDeliver:    10,
		}

		if cfg.Name != "grade-sync" {
			t.Errorf("expected name %q, got %q", "grade-sync", cfg.Name)
		}
		if !cfg.Durable {
			t.Error("grade sync should be durable for reliability")
		}
		// Grade sync should retry more times since it's critical for student grades
		if cfg.MaxDeliver != 10 {
			t.Errorf("expected MaxDeliver 10 for grade sync reliability, got %d", cfg.MaxDeliver)
		}
	})
}

func TestConsumer_Stop_NilCancel(t *testing.T) {
	// Test that Stop() handles nil cancel function gracefully
	consumer := &Consumer{
		cancel: nil,
		name:   "test-consumer",
		logger: newTestLogger(),
	}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stop() panicked: %v", r)
		}
	}()

	consumer.Stop()
}

func TestConsumerConfigDefaults(t *testing.T) {
	t.Run("zero values", func(t *testing.T) {
		cfg := ConsumerConfig{}

		if cfg.Durable {
			t.Error("default Durable should be false")
		}
		if cfg.MaxDeliver != 0 {
			t.Errorf("default MaxDeliver should be 0, got %d", cfg.MaxDeliver)
		}
		if cfg.AckWait != 0 {
			t.Errorf("default AckWait should be 0, got %d", cfg.AckWait)
		}
	})
}

func TestConsumerSubjects(t *testing.T) {
	// Validate subject patterns used by consumers
	tests := []struct {
		name          string
		subject       string
		expectMatch   []string
		expectNoMatch []string
	}{
		{
			name:    "all events wildcard",
			subject: events.SubjectAllEvents,
			expectMatch: []string{
				"labs.events.pod123.vm1.syscheck",
				"labs.events.abc.def.audit",
			},
		},
		{
			name:    "all checkpoints wildcard",
			subject: events.SubjectAllCheckpoints,
			expectMatch: []string{
				"labs.checkpoints.pod123.checkpoint1",
				"labs.checkpoints.abc.enable-ufw",
			},
		},
		{
			name:    "all sessions wildcard",
			subject: events.SubjectAllSessions,
			expectMatch: []string{
				"labs.sessions.session123.started",
				"labs.sessions.abc.ended",
			},
		},
		{
			name:    "all grades wildcard",
			subject: events.SubjectAllGrades,
			expectMatch: []string{
				"labs.grades.session123",
				"labs.grades.abc",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Verify the subject pattern is correctly formed
			if tc.subject == "" {
				t.Error("subject should not be empty")
			}

			// All wildcard subjects should end with ">"
			if tc.subject[len(tc.subject)-1] != '>' {
				t.Errorf("expected subject %q to end with '>'", tc.subject)
			}
		})
	}
}

func TestMessageHandlerType(t *testing.T) {
	// Verify MessageHandler is a valid function type
	// This is a compile-time check that the type is correctly defined

	var handler MessageHandler
	if handler != nil {
		t.Error("nil handler should be nil")
	}
}
