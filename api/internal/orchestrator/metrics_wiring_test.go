package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// fakeLifecycleRecorder counts the calls the Orchestrator makes into the
// metrics API, so the tests can assert the pod counters are actually fed
// rather than merely registered.
type fakeLifecycleRecorder struct {
	created    int
	destroyed  int
	operations []string
}

func (f *fakeLifecycleRecorder) PodCreated()   { f.created++ }
func (f *fakeLifecycleRecorder) PodDestroyed() { f.destroyed++ }
func (f *fakeLifecycleRecorder) VMOperationDuration(operation string, _ time.Duration) {
	f.operations = append(f.operations, operation)
}

func TestDestroyPodRecordsDestruction(t *testing.T) {
	rec := &fakeLifecycleRecorder{}
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()), WithMetrics(rec))

	orch.podsMu.Lock()
	orch.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{},
	}
	orch.podsMu.Unlock()

	if err := orch.DestroyPod(context.Background(), "pod-1"); err != nil {
		t.Fatalf("DestroyPod: %v", err)
	}

	if rec.destroyed != 1 {
		t.Errorf("destroyed count = %d, want 1", rec.destroyed)
	}
	if rec.created != 0 {
		t.Errorf("created count = %d, want 0", rec.created)
	}
}

// A pod that could not be found is never destroyed, so the gauge must not be
// decremented for it.
func TestDestroyPodDoesNotRecordWhenPodMissing(t *testing.T) {
	rec := &fakeLifecycleRecorder{}
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()), WithMetrics(rec))

	if err := orch.DestroyPod(context.Background(), "nonexistent"); err == nil {
		t.Fatal("expected an error for a nonexistent pod")
	}

	if rec.destroyed != 0 {
		t.Errorf("destroyed count = %d, want 0", rec.destroyed)
	}
}

// A nil recorder is the default wherever metrics are not wired; it must not
// panic.
func TestDestroyPodWithoutRecorderDoesNotPanic(t *testing.T) {
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()))

	orch.podsMu.Lock()
	orch.pods["pod-1"] = &models.Pod{
		ID:       "pod-1",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{},
	}
	orch.podsMu.Unlock()

	if err := orch.DestroyPod(context.Background(), "pod-1"); err != nil {
		t.Fatalf("DestroyPod: %v", err)
	}
}

// The HighPodProvisioningTime alert selects operation="provision"; nothing
// emitted that label until the provisioning span was timed.
func TestCreatePodTimesProvisioning(t *testing.T) {
	rec := &fakeLifecycleRecorder{}
	orch := New(nil, nil, Config{DefaultPlatform: models.PlatformProxmox}, WithLogger(newTestLogger()), WithMetrics(rec))

	// Provisioning fails without a hypervisor client, which is fine: the span
	// is timed either way, because a provision that failed slowly was slow.
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "timing-template"},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs:      []models.VMSpec{{Name: "vm1", Template: "base"}},
		},
	}
	_, _ = orch.CreatePod(context.Background(), template, "tmpl-1", "user-1", "user")

	var found bool
	for _, op := range rec.operations {
		if op == "provision" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("operations = %v, want one labelled \"provision\"", rec.operations)
	}
}

// Nothing asserted that PodCreated is ever called: every existing case drives
// a failing CreatePod. This drives the success path via the in-memory store.
func TestCreatePodRecordsCreationOnSuccess(t *testing.T) {
	rec := &fakeLifecycleRecorder{}
	orch := New(nil, nil, Config{DefaultPlatform: models.PlatformProxmox},
		WithLogger(newTestLogger()), WithMetrics(rec))

	// A template with no VMs provisions successfully without a hypervisor.
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "empty-template"},
		Spec:     models.LabSpec{Platform: models.PlatformProxmox},
	}
	if _, err := orch.CreatePod(context.Background(), template, "tmpl-1", "user-1", "user"); err != nil {
		t.Fatalf("CreatePod: %v", err)
	}

	if rec.created != 1 {
		t.Errorf("created count = %d, want 1", rec.created)
	}
	if rec.destroyed != 0 {
		t.Errorf("destroyed count = %d, want 0", rec.destroyed)
	}
}

// A provision that fails must not count a creation -- the pod row exists but
// no pod does, and counting it here is what used to drive pods_active
// negative when that row was later destroyed.
func TestCreatePodDoesNotRecordCreationOnFailure(t *testing.T) {
	rec := &fakeLifecycleRecorder{}
	orch := New(nil, nil, Config{DefaultPlatform: models.PlatformProxmox},
		WithLogger(newTestLogger()), WithMetrics(rec))

	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "failing-template"},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs:      []models.VMSpec{{Name: "vm1", Template: "base"}},
		},
	}
	if _, err := orch.CreatePod(context.Background(), template, "tmpl-1", "user-1", "user"); err == nil {
		t.Fatal("expected CreatePod to fail without a hypervisor client")
	}

	if rec.created != 0 {
		t.Errorf("created count = %d, want 0 for a failed provision", rec.created)
	}
}

// Destroy is a soft delete: the row survives and GetPod keeps returning it, so
// a replayed DELETE used to run the whole body again and count a second
// destruction. The existing destroy tests run without a repository, where the
// pod is removed from the in-memory map and a replay 404s instead — so they
// cannot reach the guard at all.
func TestDestroyPodIsIdempotentWithPersistence(t *testing.T) {
	repo := newMockPodRepository()
	rec := &fakeLifecycleRecorder{}
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()),
		WithPodRepository(repo), WithMetrics(rec))

	ctx := context.Background()
	pod := &models.Pod{
		ID:       "pod-1",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{},
	}
	if err := repo.Create(ctx, pod); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := orch.DestroyPod(ctx, "pod-1"); err != nil {
		t.Fatalf("first DestroyPod: %v", err)
	}
	if err := orch.DestroyPod(ctx, "pod-1"); err != nil {
		t.Fatalf("replayed DestroyPod should succeed, got: %v", err)
	}
	if err := orch.DestroyPod(ctx, "pod-1"); err != nil {
		t.Fatalf("third DestroyPod: %v", err)
	}

	if rec.destroyed != 1 {
		t.Errorf("destroyed count = %d, want 1 — a replayed DELETE must not count again", rec.destroyed)
	}
}
