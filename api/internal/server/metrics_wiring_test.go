package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/metrics"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/pods"
	"github.com/toddbartholow/kootenai/api/internal/server/sessions"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// The recorder interfaces are declared by their consumers, so nothing forces
// *metrics.Metrics to keep satisfying them. Renaming a method on Metrics would
// otherwise only surface as a silently unfed counter at the wiring site.
var (
	_ orchestrator.PodLifecycleRecorder    = (*metrics.Metrics)(nil)
	_ sessions.SessionLifecycleRecorder    = (*metrics.Metrics)(nil)
	_ checkpoint.CheckpointOutcomeRecorder = (*metrics.Metrics)(nil)
)

func newMetricsTestConfig() Config {
	cfg := DefaultConfig()
	cfg.CORSOrigins = []string{"http://localhost:3000"}
	return cfg
}

// main.go builds one registry and hands the same instance to the orchestrator,
// the evaluator and the server, so /metrics serves what every subsystem wrote.
// This asserts the server half of that contract.
func TestWithMetricsSharesTheRegistry(t *testing.T) {
	shared := metrics.New()

	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()),
		newTestLogger(), WithMetrics(shared))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if srv.metrics != shared {
		t.Fatal("server is not using the supplied registry")
	}

	// Prove it end to end rather than by pointer identity alone.
	shared.PodCreated()
	if got := srv.metrics.Snapshot().PodsCreatedTotal; got != 1 {
		t.Errorf("PodsCreatedTotal = %d, want 1", got)
	}
}

// Without the option the server keeps a private registry, which is what most
// tests want and what every existing New(...) call site relies on.
func TestWithoutMetricsOptionServerHasItsOwnRegistry(t *testing.T) {
	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if srv.metrics == nil {
		t.Fatal("server has no registry")
	}
}

// The two active-* gauges are owned by the reconcile, not by the lifecycle
// counters. This is the assertion that would fail if anyone reinstated
// inc/dec at the call sites.
func TestReconcileSetsGaugesFromRepositories(t *testing.T) {
	shared := metrics.New()
	podRepo := mocks.NewFakePodRepository()
	sessionRepo := mocks.NewFakeSessionRepository()

	ctx := context.Background()
	// "Active" is everything except `destroyed`. A destroying pod still holds
	// its VMs — DestroyPod sets that status before touching any of them — so
	// only the destroyed one is excluded.
	statuses := []models.PodStatus{
		models.PodStatusRunning,
		models.PodStatusProvisioning,
		models.PodStatusStopped,
		models.PodStatusError,
		models.PodStatusDestroying,
		models.PodStatusDestroyed,
	}
	for i, st := range statuses {
		if err := podRepo.Create(ctx, &models.Pod{ID: fmt.Sprintf("pod-%d", i), Status: st}); err != nil {
			t.Fatalf("pod Create: %v", err)
		}
	}
	// One open session, one already ended.
	ended := time.Now()
	if err := sessionRepo.Create(ctx, &models.Session{ID: "s-open"}); err != nil {
		t.Fatalf("session Create: %v", err)
	}
	if err := sessionRepo.Create(ctx, &models.Session{ID: "s-ended", EndedAt: &ended}); err != nil {
		t.Fatalf("session Create: %v", err)
	}

	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
		WithMetrics(shared), WithPodRepo(podRepo), WithSessionRepo(sessionRepo))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	srv.reconcileMetricsOnce(ctx)

	snap := shared.Snapshot()
	if snap.ActivePods != 5 {
		t.Errorf("ActivePods = %d, want 5 (all but destroyed)", snap.ActivePods)
	}
	if snap.ActiveSessions != 1 {
		t.Errorf("ActiveSessions = %d, want 1", snap.ActiveSessions)
	}
}

// A reconcile that runs again must overwrite, not accumulate — that is the
// whole point of it being a gauge set rather than a delta.
func TestReconcileIsIdempotent(t *testing.T) {
	shared := metrics.New()
	podRepo := mocks.NewFakePodRepository()

	ctx := context.Background()
	if err := podRepo.Create(ctx, &models.Pod{ID: "p1", Status: models.PodStatusRunning}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
		WithMetrics(shared), WithPodRepo(podRepo), WithSessionRepo(mocks.NewFakeSessionRepository()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	srv.reconcileMetricsOnce(ctx)
	srv.reconcileMetricsOnce(ctx)
	srv.reconcileMetricsOnce(ctx)

	if got := shared.Snapshot().ActivePods; got != 1 {
		t.Errorf("ActivePods = %d, want 1 after three reconciles", got)
	}
}

// The freshness stamp is what MetricsReconcileStale alerts on. It must be
// written on a complete pass and withheld on every incomplete one — three
// separate mutations of that logic previously survived the whole suite.
func TestReconcileStampsFreshnessOnlyOnCompletePass(t *testing.T) {
	ctx := context.Background()

	t.Run("stamped when both gauges are set", func(t *testing.T) {
		shared := metrics.New()
		srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
			WithMetrics(shared), WithPodRepo(mocks.NewFakePodRepository()),
			WithSessionRepo(mocks.NewFakeSessionRepository()))
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		srv.reconcileMetricsOnce(ctx)

		if got := shared.Snapshot().ReconcileSuccessTimestamp; got == 0 {
			t.Error("freshness not stamped after a complete pass")
		}
	})

	t.Run("withheld when a query fails", func(t *testing.T) {
		shared := metrics.New()
		podRepo := mocks.NewFakePodRepository()
		podRepo.CountActiveErr = errors.New("database is down")
		srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
			WithMetrics(shared), WithPodRepo(podRepo), WithSessionRepo(mocks.NewFakeSessionRepository()))
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		srv.reconcileMetricsOnce(ctx)

		if got := shared.Snapshot().ReconcileSuccessTimestamp; got != 0 {
			t.Errorf("freshness stamped despite a failed query (got %d)", got)
		}
	})

	t.Run("withheld when the session query fails", func(t *testing.T) {
		shared := metrics.New()
		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.CountActiveErr = errors.New("database is down")
		srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
			WithMetrics(shared), WithPodRepo(mocks.NewFakePodRepository()), WithSessionRepo(sessionRepo))
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		srv.reconcileMetricsOnce(ctx)

		if got := shared.Snapshot().ReconcileSuccessTimestamp; got != 0 {
			t.Errorf("freshness stamped despite a failed session query (got %d)", got)
		}
	})
}

// One failing query must not freeze the other gauge. Returning early on the
// first error made sessions_active go stale whenever the pod query failed,
// even though the session database was healthy — and the asymmetry depended
// only on which query came first.
func TestReconcileUpdatesEachGaugeIndependently(t *testing.T) {
	ctx := context.Background()

	newPods := func(n int) *mocks.FakePodRepository {
		r := mocks.NewFakePodRepository()
		for i := 0; i < n; i++ {
			if err := r.Create(ctx, &models.Pod{ID: fmt.Sprintf("p%d", i), Status: models.PodStatusRunning}); err != nil {
				t.Fatalf("pod Create: %v", err)
			}
		}
		return r
	}
	newSessions := func(n int) *mocks.FakeSessionRepository {
		r := mocks.NewFakeSessionRepository()
		for i := 0; i < n; i++ {
			if err := r.Create(ctx, &models.Session{ID: fmt.Sprintf("s%d", i)}); err != nil {
				t.Fatalf("session Create: %v", err)
			}
		}
		return r
	}

	t.Run("pod query fails, sessions still refresh", func(t *testing.T) {
		shared := metrics.New()
		podRepo, sessionRepo := newPods(2), newSessions(3)
		srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
			WithMetrics(shared), WithPodRepo(podRepo), WithSessionRepo(sessionRepo))
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		podRepo.CountActiveErr = errors.New("pods table unavailable")
		srv.reconcileMetricsOnce(ctx)

		if got := shared.Snapshot().ActiveSessions; got != 3 {
			t.Errorf("ActiveSessions = %d, want 3 — a pod-side failure froze a healthy gauge", got)
		}
	})

	t.Run("session query fails, pods still refresh", func(t *testing.T) {
		shared := metrics.New()
		podRepo, sessionRepo := newPods(4), newSessions(1)
		srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
			WithMetrics(shared), WithPodRepo(podRepo), WithSessionRepo(sessionRepo))
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		sessionRepo.CountActiveErr = errors.New("sessions table unavailable")
		srv.reconcileMetricsOnce(ctx)

		if got := shared.Snapshot().ActivePods; got != 4 {
			t.Errorf("ActivePods = %d, want 4 — a session-side failure froze a healthy gauge", got)
		}
	})
}

// A server with no repositories must not panic in the reconcile.
func TestReconcileWithoutRepositoriesDoesNotPanic(t *testing.T) {
	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.reconcileMetricsOnce(context.Background())
}

// A failing reconcile must hold the last good values rather than zeroing —
// zero reads as "no load" and silently disarms TooManyActivePods.
func TestReconcileHoldsGaugesOnError(t *testing.T) {
	shared := metrics.New()
	podRepo := mocks.NewFakePodRepository()

	ctx := context.Background()
	if err := podRepo.Create(ctx, &models.Pod{ID: "p1", Status: models.PodStatusRunning}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
		WithMetrics(shared), WithPodRepo(podRepo), WithSessionRepo(mocks.NewFakeSessionRepository()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	srv.reconcileMetricsOnce(ctx)
	if got := shared.Snapshot().ActivePods; got != 1 {
		t.Fatalf("ActivePods = %d, want 1 before the failure", got)
	}

	podRepo.CountActiveErr = errors.New("database is down")
	srv.reconcileMetricsOnce(ctx)

	if got := shared.Snapshot().ActivePods; got != 1 {
		t.Errorf("ActivePods = %d, want 1 held over the failure", got)
	}
}

// server.New hands its registry to the sub-managers it builds. Removing
// `Metrics: s.metrics` from either the sessions or the canvas manager config
// previously left the whole suite green — the manager-level tests inject their
// own recorder, so they cannot notice that production never supplies one.
func TestNewWiresMetricsIntoSubManagers(t *testing.T) {
	shared := metrics.New()
	sessionRepo := mocks.NewFakeSessionRepository()

	srv, err := New(newMetricsTestConfig(), nil, nil, websocket.NewHub(newTestLogger()), newTestLogger(),
		WithMetrics(shared), WithSessionRepo(sessionRepo))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if srv.sessionMgr == nil {
		t.Fatal("session manager was not constructed")
	}
	if srv.sessionMgr.MetricsRecorder() != sessions.SessionLifecycleRecorder(shared) {
		t.Error("session manager did not receive the server's registry")
	}
	if srv.canvasMgr != nil && srv.canvasMgr.MetricsRecorder() != sessions.SessionLifecycleRecorder(shared) {
		t.Error("canvas manager did not receive the server's registry")
	}
	// The pods manager feeds labctl_vm_operation_duration_seconds for eight
	// operations; losing this wire zeroes all of them silently.
	if srv.podMgr == nil {
		t.Fatal("pod manager was not constructed")
	}
	if srv.podMgr.MetricsRecorder() != pods.VMOperationRecorder(shared) {
		t.Error("pod manager did not receive the server's registry")
	}
}
