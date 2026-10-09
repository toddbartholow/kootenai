package main

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/metrics"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// The composition root is where the metrics registry is handed to each
// subsystem, and it is the one place a missing WithMetrics(...) would not show
// up anywhere else: the packages themselves are fully tested with fakes, so
// dropping the wiring here silently zeroes the metrics in production while
// leaving every other test green. These tests fail if that happens.

func newWiringTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestInitOrchestratorWiresMetrics(t *testing.T) {
	appMetrics := metrics.New()
	logger := newWiringTestLogger()

	orch := initOrchestrator(context.Background(), &AppConfig{}, nil, nil, nil, appMetrics, logger)
	if orch == nil {
		t.Fatal("initOrchestrator returned nil")
	}

	// Drive a real pod creation through the orchestrator and assert the
	// registry this test owns saw it. Pointer identity would not prove the
	// recorder is actually reached from the pod lifecycle. A template with no
	// VMs provisions successfully without a hypervisor client.
	before := appMetrics.Snapshot().PodsCreatedTotal

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "wiring-template"},
		Spec:     models.LabSpec{Platform: models.PlatformProxmox},
	}
	if _, err := orch.CreatePod(context.Background(), template, "tmpl-1", "user-1", "user"); err != nil {
		t.Fatalf("CreatePod: %v", err)
	}

	if got := appMetrics.Snapshot().PodsCreatedTotal; got != before+1 {
		t.Errorf("PodsCreatedTotal = %d, want %d — initOrchestrator is not passing orchestrator.WithMetrics",
			got, before+1)
	}
}

func TestSetupCheckpointEvaluatorWiresMetrics(t *testing.T) {
	appMetrics := metrics.New()
	logger := newWiringTestLogger()

	evaluator, runner, _ := setupCheckpointEvaluator(context.Background(), nil, nil, nil, appMetrics, logger)
	if runner != nil {
		defer runner.Stop()
	}
	if evaluator == nil {
		t.Fatal("setupCheckpointEvaluator returned a nil evaluator")
	}

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "wiring-template"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{{ID: "cp1", Points: 10}},
		},
	}
	if err := evaluator.RegisterTemplate(template); err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	if err := evaluator.StartSession("s1", "p1", "u1", "wiring-template"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	before := appMetrics.Snapshot().CheckpointsPassedTotal
	if err := evaluator.MarkCheckpointPassed(context.Background(), "s1", "cp1"); err != nil {
		t.Fatalf("MarkCheckpointPassed: %v", err)
	}

	if got := appMetrics.Snapshot().CheckpointsPassedTotal; got != before+1 {
		t.Errorf("CheckpointsPassedTotal = %d, want %d — setupCheckpointEvaluator is not passing checkpoint.WithMetrics",
			got, before+1)
	}
}

// Both setup functions must tolerate a nil registry, which is how they behave
// if metrics are ever made optional.
func TestSetupFunctionsAcceptNilMetrics(t *testing.T) {
	logger := newWiringTestLogger()

	if orch := initOrchestrator(context.Background(), &AppConfig{}, nil, nil, nil, nil, logger); orch == nil {
		t.Error("initOrchestrator returned nil with a nil registry")
	}
	evaluator, runner, _ := setupCheckpointEvaluator(context.Background(), nil, nil, nil, nil, logger)
	if runner != nil {
		defer runner.Stop()
	}
	if evaluator == nil {
		t.Error("setupCheckpointEvaluator returned nil with a nil registry")
	}
}
