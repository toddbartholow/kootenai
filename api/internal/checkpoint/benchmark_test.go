package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// =============================================================================
// Benchmark Helpers
// =============================================================================

func createBenchTemplate(numCheckpoints int) *models.LabTemplate {
	objectives := make([]models.Checkpoint, numCheckpoints)
	for i := 0; i < numCheckpoints; i++ {
		objectives[i] = models.Checkpoint{
			ID:          fmt.Sprintf("cp-%d", i),
			Description: fmt.Sprintf("Checkpoint %d", i),
			Points:      10,
			Triggers: []models.CheckpointTrigger{
				{
					Type:   models.TriggerTypeFileExists,
					Target: "vm",
					Match:  models.TriggerMatch{Path: fmt.Sprintf("/task%d", i)},
				},
			},
		}
	}

	return &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "Benchmark Lab"},
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				Enabled:       true,
				PassThreshold: 70,
			},
			Objectives: objectives,
		},
	}
}

func createBenchEvent(path string) *events.VMEvent {
	return &events.VMEvent{
		PodID:     "pod-bench",
		VMName:    "vm",
		EventType: events.EventTypeSyscheck,
		Data:      json.RawMessage(fmt.Sprintf(`{"path": "%s", "event": "added"}`, path)),
	}
}

// =============================================================================
// Benchmarks - Event Evaluation
// =============================================================================

func BenchmarkEvaluateEvent_10Checkpoints(b *testing.B) {
	benchmarkEvaluateEvent(b, 10)
}

func BenchmarkEvaluateEvent_50Checkpoints(b *testing.B) {
	benchmarkEvaluateEvent(b, 50)
}

func BenchmarkEvaluateEvent_100Checkpoints(b *testing.B) {
	benchmarkEvaluateEvent(b, 100)
}

func BenchmarkEvaluateEvent_500Checkpoints(b *testing.B) {
	benchmarkEvaluateEvent(b, 500)
}

func benchmarkEvaluateEvent(b *testing.B, numCheckpoints int) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(numCheckpoints)
	_ = evaluator.RegisterTemplate(template)
	_ = evaluator.StartSession("session-bench", "pod-bench", "user-bench", "Benchmark Lab")

	// Create an event that matches checkpoint 5 (or last if less than 5)
	cpIndex := 5
	if cpIndex >= numCheckpoints {
		cpIndex = numCheckpoints - 1
	}
	event := createBenchEvent(fmt.Sprintf("/task%d", cpIndex))

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.EvaluateEvent(ctx, event, "session-bench")
	}
}

// Benchmark non-matching events (should check all checkpoints)
func BenchmarkEvaluateEvent_NoMatch_100Checkpoints(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(100)
	_ = evaluator.RegisterTemplate(template)
	_ = evaluator.StartSession("session-bench", "pod-bench", "user-bench", "Benchmark Lab")

	// Event that matches nothing
	event := createBenchEvent("/nonexistent/path")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.EvaluateEvent(ctx, event, "session-bench")
	}
}

// =============================================================================
// Benchmarks - Session Management
// =============================================================================

func BenchmarkStartSession(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(50)
	_ = evaluator.RegisterTemplate(template)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		_ = evaluator.StartSession(sessionID, "pod-bench", "user-bench", "Benchmark Lab")
	}
}

func BenchmarkGetSessionProgress(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(50)
	_ = evaluator.RegisterTemplate(template)
	_ = evaluator.StartSession("session-bench", "pod-bench", "user-bench", "Benchmark Lab")

	// Complete some checkpoints
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		event := createBenchEvent(fmt.Sprintf("/task%d", i))
		_, _ = evaluator.EvaluateEvent(ctx, event, "session-bench")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.GetSessionProgress("session-bench")
	}
}

// =============================================================================
// Benchmarks - Concurrent Sessions
// =============================================================================

func BenchmarkConcurrentSessions_10(b *testing.B) {
	benchmarkConcurrentSessions(b, 10, 20)
}

func BenchmarkConcurrentSessions_50(b *testing.B) {
	benchmarkConcurrentSessions(b, 50, 20)
}

func BenchmarkConcurrentSessions_100(b *testing.B) {
	benchmarkConcurrentSessions(b, 100, 20)
}

func benchmarkConcurrentSessions(b *testing.B, numSessions, numCheckpoints int) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(numCheckpoints)
	_ = evaluator.RegisterTemplate(template)

	// Start all sessions
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		_ = evaluator.StartSession(sessionID, fmt.Sprintf("pod-%d", i), fmt.Sprintf("user-%d", i), "Benchmark Lab")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(numSessions)

		for s := 0; s < numSessions; s++ {
			go func(sessionNum int) {
				defer wg.Done()
				sessionID := fmt.Sprintf("session-%d", sessionNum)
				cpIndex := sessionNum % numCheckpoints
				event := &events.VMEvent{
					PodID:     fmt.Sprintf("pod-%d", sessionNum),
					VMName:    "vm",
					EventType: events.EventTypeSyscheck,
					Data:      json.RawMessage(fmt.Sprintf(`{"path": "/task%d", "event": "added"}`, cpIndex)),
				}
				_, _ = evaluator.EvaluateEvent(ctx, event, sessionID)
			}(s)
		}

		wg.Wait()
	}
}

// =============================================================================
// Benchmarks - Different Trigger Types
// =============================================================================

func BenchmarkEvaluateEvent_PackageTrigger(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "Package Lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:     "cp-pkg",
					Points: 10,
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypePackage,
							Target: "vm",
							Match:  models.TriggerMatch{Package: "nginx", State: "installed"},
						},
					},
				},
			},
		},
	}
	_ = evaluator.RegisterTemplate(template)
	_ = evaluator.StartSession("session-bench", "pod-bench", "user-bench", "Package Lab")

	event := &events.VMEvent{
		PodID:     "pod-bench",
		VMName:    "vm",
		EventType: events.EventTypePackage,
		Data:      json.RawMessage(`{"package": "nginx", "action": "install"}`),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.EvaluateEvent(ctx, event, "session-bench")
	}
}

func BenchmarkEvaluateEvent_ServiceTrigger(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "Service Lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{
					ID:     "cp-svc",
					Points: 10,
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypeService,
							Target: "vm",
							Match:  models.TriggerMatch{Name: "nginx", State: "active"},
						},
					},
				},
			},
		},
	}
	_ = evaluator.RegisterTemplate(template)
	_ = evaluator.StartSession("session-bench", "pod-bench", "user-bench", "Service Lab")

	event := &events.VMEvent{
		PodID:     "pod-bench",
		VMName:    "vm",
		EventType: events.EventTypeService,
		Data:      json.RawMessage(`{"unit": "nginx", "state": "active"}`),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.EvaluateEvent(ctx, event, "session-bench")
	}
}

// =============================================================================
// Load Tests
// =============================================================================

func TestLoadTest_HighEventThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(50)
	_ = evaluator.RegisterTemplate(template)
	_ = evaluator.StartSession("session-load", "pod-load", "user-load", "Benchmark Lab")

	ctx := context.Background()
	numEvents := 10000

	start := time.Now()
	for i := 0; i < numEvents; i++ {
		cpIndex := i % 50
		event := createBenchEvent(fmt.Sprintf("/task%d", cpIndex))
		_, _ = evaluator.EvaluateEvent(ctx, event, "session-load")
	}
	duration := time.Since(start)

	eventsPerSecond := float64(numEvents) / duration.Seconds()
	avgEventTime := duration / time.Duration(numEvents)

	t.Logf("=== High Event Throughput Test ===")
	t.Logf("Total events: %d", numEvents)
	t.Logf("Duration: %v", duration)
	t.Logf("Events/second: %.2f", eventsPerSecond)
	t.Logf("Avg event time: %v", avgEventTime)
	t.Logf("==================================")

	// Should handle at least 25,000 events/second (lower bound for shared CI runners)
	if eventsPerSecond < 25000 {
		t.Errorf("Expected at least 25,000 events/sec, got %.2f", eventsPerSecond)
	}
}

func TestLoadTest_ConcurrentEventProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	numSessions := 50
	numCheckpoints := 20
	numEventsPerSession := 100

	template := createBenchTemplate(numCheckpoints)
	_ = evaluator.RegisterTemplate(template)

	// Start all sessions
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		_ = evaluator.StartSession(sessionID, fmt.Sprintf("pod-%d", i), fmt.Sprintf("user-%d", i), "Benchmark Lab")
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, numSessions*numEventsPerSession)

	start := time.Now()

	for s := 0; s < numSessions; s++ {
		wg.Add(1)
		go func(sessionNum int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", sessionNum)

			for e := 0; e < numEventsPerSession; e++ {
				cpIndex := e % numCheckpoints
				event := &events.VMEvent{
					PodID:     fmt.Sprintf("pod-%d", sessionNum),
					VMName:    "vm",
					EventType: events.EventTypeSyscheck,
					Data:      json.RawMessage(fmt.Sprintf(`{"path": "/task%d", "event": "added"}`, cpIndex)),
				}
				_, err := evaluator.EvaluateEvent(ctx, event, sessionID)
				if err != nil {
					errors <- err
				}
			}
		}(s)
	}

	wg.Wait()
	close(errors)

	duration := time.Since(start)
	totalEvents := numSessions * numEventsPerSession
	eventsPerSecond := float64(totalEvents) / duration.Seconds()

	var errorCount int
	for range errors {
		errorCount++
	}

	t.Logf("=== Concurrent Event Processing Test ===")
	t.Logf("Sessions: %d", numSessions)
	t.Logf("Events per session: %d", numEventsPerSession)
	t.Logf("Total events: %d", totalEvents)
	t.Logf("Duration: %v", duration)
	t.Logf("Events/second: %.2f", eventsPerSecond)
	t.Logf("Errors: %d", errorCount)
	t.Logf("=========================================")

	if errorCount > 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}
	if eventsPerSecond < 10000 {
		t.Errorf("Expected at least 10,000 events/sec, got %.2f", eventsPerSecond)
	}
}

func TestLoadTest_ManyActiveSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	evaluator := NewEvaluator(logger)

	template := createBenchTemplate(20)
	_ = evaluator.RegisterTemplate(template)

	// Start many sessions
	numSessions := 1000
	start := time.Now()
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		_ = evaluator.StartSession(sessionID, fmt.Sprintf("pod-%d", i), fmt.Sprintf("user-%d", i), "Benchmark Lab")
	}
	sessionStartDuration := time.Since(start)

	// Process events across all sessions
	ctx := context.Background()
	numEventsTotal := 5000

	start = time.Now()
	for i := 0; i < numEventsTotal; i++ {
		sessionNum := i % numSessions
		sessionID := fmt.Sprintf("session-%d", sessionNum)
		cpIndex := i % 20
		event := &events.VMEvent{
			PodID:     fmt.Sprintf("pod-%d", sessionNum),
			VMName:    "vm",
			EventType: events.EventTypeSyscheck,
			Data:      json.RawMessage(fmt.Sprintf(`{"path": "/task%d", "event": "added"}`, cpIndex)),
		}
		_, _ = evaluator.EvaluateEvent(ctx, event, sessionID)
	}
	eventProcessDuration := time.Since(start)

	eventsPerSecond := float64(numEventsTotal) / eventProcessDuration.Seconds()

	t.Logf("=== Many Active Sessions Test ===")
	t.Logf("Active sessions: %d", numSessions)
	t.Logf("Session start time: %v", sessionStartDuration)
	t.Logf("Events processed: %d", numEventsTotal)
	t.Logf("Event processing time: %v", eventProcessDuration)
	t.Logf("Events/second: %.2f", eventsPerSecond)
	t.Logf("=================================")

	// Should still be fast with many sessions (use lower threshold for CI environments)
	if eventsPerSecond < 1000 {
		t.Errorf("Expected at least 1,000 events/sec with %d sessions, got %.2f", numSessions, eventsPerSecond)
	}
}
