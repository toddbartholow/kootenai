package checkpoint

import (
	"context"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// fakeOutcomeRecorder counts the calls the Evaluator makes into the metrics
// API, so the tests can assert the counters are actually fed rather than
// merely registered.
type fakeOutcomeRecorder struct {
	passed int
	failed int
}

func (f *fakeOutcomeRecorder) CheckpointPassed() { f.passed++ }
func (f *fakeOutcomeRecorder) CheckpointFailed() { f.failed++ }

func newRecordingEvaluator(t *testing.T) (*Evaluator, *fakeOutcomeRecorder) {
	t.Helper()

	rec := &fakeOutcomeRecorder{}
	eval := NewEvaluator(newTestLogger(), WithMetrics(rec))

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "metrics-template"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp1", Description: "First checkpoint", Points: 10},
			},
		},
	}
	if err := eval.RegisterTemplate(template); err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	eval.StartSession("session-1", "pod-1", "user-1", "metrics-template")

	return eval, rec
}

func TestEvaluatorRecordsPassedCheckpoint(t *testing.T) {
	eval, rec := newRecordingEvaluator(t)

	if err := eval.MarkCheckpointPassed(context.Background(), "session-1", "cp1"); err != nil {
		t.Fatalf("MarkCheckpointPassed: %v", err)
	}

	if rec.passed != 1 {
		t.Errorf("passed count = %d, want 1", rec.passed)
	}
	if rec.failed != 0 {
		t.Errorf("failed count = %d, want 0", rec.failed)
	}
}

// A checkpoint that is already passed is a no-op, so it must not be counted
// twice — the counter tracks outcomes, not calls.
func TestEvaluatorDoesNotDoubleCountPassedCheckpoint(t *testing.T) {
	eval, rec := newRecordingEvaluator(t)

	ctx := context.Background()
	if err := eval.MarkCheckpointPassed(ctx, "session-1", "cp1"); err != nil {
		t.Fatalf("MarkCheckpointPassed: %v", err)
	}
	if err := eval.MarkCheckpointPassed(ctx, "session-1", "cp1"); err != nil {
		t.Fatalf("second MarkCheckpointPassed: %v", err)
	}

	if rec.passed != 1 {
		t.Errorf("passed count = %d, want 1 (second call is a no-op)", rec.passed)
	}
}

func TestEvaluatorRecordsFailedVerification(t *testing.T) {
	eval, rec := newRecordingEvaluator(t)

	// Verification outcomes only apply to a checkpoint awaiting verification.
	eval.sessionMu.Lock()
	eval.sessions["session-1"].Checkpoints["cp1"].Status = models.CheckpointStatusPendingVerification
	eval.sessionMu.Unlock()

	eval.MarkCheckpointVerificationFailed(context.Background(), "session-1", "cp1")

	if rec.failed != 1 {
		t.Errorf("failed count = %d, want 1", rec.failed)
	}
	if rec.passed != 0 {
		t.Errorf("passed count = %d, want 0", rec.passed)
	}
}

func TestEvaluatorRecordsVerifiedCheckpoint(t *testing.T) {
	eval, rec := newRecordingEvaluator(t)

	eval.sessionMu.Lock()
	eval.sessions["session-1"].Checkpoints["cp1"].Status = models.CheckpointStatusPendingVerification
	eval.sessionMu.Unlock()

	eval.MarkCheckpointVerified(context.Background(), "session-1", "cp1")

	if rec.passed != 1 {
		t.Errorf("passed count = %d, want 1", rec.passed)
	}
	if rec.failed != 0 {
		t.Errorf("failed count = %d, want 0", rec.failed)
	}
}

// A nil recorder is the default in tests and in any deployment that has not
// wired metrics; it must not panic.
func TestEvaluatorWithoutRecorderDoesNotPanic(t *testing.T) {
	eval := NewEvaluator(newTestLogger())
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "metrics-template"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{{ID: "cp1", Points: 10}},
		},
	}
	if err := eval.RegisterTemplate(template); err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	eval.StartSession("session-1", "pod-1", "user-1", "metrics-template")

	if err := eval.MarkCheckpointPassed(context.Background(), "session-1", "cp1"); err != nil {
		t.Fatalf("MarkCheckpointPassed: %v", err)
	}
}
