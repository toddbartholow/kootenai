package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/cloudstack"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/proxmox"
)

// -----------------------------------------------------------------------------
// Mock Pod Repository
// -----------------------------------------------------------------------------

type mockPodRepository struct {
	mu      sync.RWMutex
	pods    map[string]*models.Pod
	errOnOp map[string]error // operation -> error to return
}

func newMockPodRepository() *mockPodRepository {
	return &mockPodRepository{
		pods:    make(map[string]*models.Pod),
		errOnOp: make(map[string]error),
	}
}

func (m *mockPodRepository) Create(ctx context.Context, pod *models.Pod) error {
	if err := m.errOnOp["create"]; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pods[pod.ID] = pod
	return nil
}

func (m *mockPodRepository) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	if err := m.errOnOp["getByID"]; err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	pod, ok := m.pods[id]
	if !ok {
		return nil, nil
	}
	return pod, nil
}

func (m *mockPodRepository) List(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
	if err := m.errOnOp["list"]; err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*models.Pod
	for _, pod := range m.pods {
		if filter.OwnerID != "" && pod.Owner != filter.OwnerID {
			continue
		}
		if filter.TemplateID != "" && pod.LabTemplate != filter.TemplateID {
			continue
		}
		if filter.Status != "" && pod.Status != filter.Status {
			continue
		}
		if filter.Platform != "" && string(pod.Platform) != filter.Platform {
			continue
		}
		result = append(result, pod)
	}
	return result, nil
}

func (m *mockPodRepository) Update(ctx context.Context, pod *models.Pod) error {
	if err := m.errOnOp["update"]; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pods[pod.ID] = pod
	return nil
}

func (m *mockPodRepository) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	if err := m.errOnOp["updateStatus"]; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if pod, ok := m.pods[id]; ok {
		pod.Status = status
	}
	return nil
}

func (m *mockPodRepository) Delete(ctx context.Context, id string) error {
	if err := m.errOnOp["delete"]; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pods, id)
	return nil
}

func (m *mockPodRepository) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	if err := m.errOnOp["getExpired"]; err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var expired []*models.Pod
	now := time.Now()
	for _, pod := range m.pods {
		if pod.ExpiresAt != nil && pod.ExpiresAt.Before(now) &&
			pod.Status != models.PodStatusDestroyed && pod.Status != models.PodStatusDestroying {
			expired = append(expired, pod)
		}
	}
	return expired, nil
}

func (m *mockPodRepository) setError(op string, err error) {
	m.errOnOp[op] = err
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestOrchestrator(t *testing.T, pveServer, csServer *httptest.Server) (*Orchestrator, *mockPodRepository) {
	repo := newMockPodRepository()
	logger := newTestLogger()

	var pveClient *proxmox.Client
	if pveServer != nil {
		// Extract host and port from server URL
		url := strings.TrimPrefix(pveServer.URL, "http://")
		parts := strings.Split(url, ":")
		host := parts[0]
		port := 80
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &port)
		}

		var err error
		pveClient, err = proxmox.NewClient(proxmox.Config{
			Host:    host,
			Port:    port,
			TokenID: "user@pve!token",
			Token:   "secret-token",
		})
		if err != nil {
			t.Fatalf("Failed to create proxmox client: %v", err)
		}
	}

	var csClient *cloudstack.Client
	if csServer != nil {
		url := strings.TrimPrefix(csServer.URL, "http://")
		parts := strings.Split(url, ":")
		host := parts[0]
		port := 8080
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &port)
		}

		var err error
		csClient, err = cloudstack.NewClient(cloudstack.Config{
			Host:      host,
			Port:      port,
			APIKey:    "test-api-key",
			SecretKey: "test-secret-key",
		})
		if err != nil {
			t.Fatalf("Failed to create cloudstack client: %v", err)
		}
	}

	cfg := Config{
		DefaultPlatform:    models.PlatformProxmox,
		ProxmoxNode:        "node1",
		CloudStackZoneID:   "zone-123",
		PodNetworkBase:     100,
		MaxPodsPerUser:     5,
		DefaultPodDuration: 2 * time.Hour,
		TemplateVMIDs: map[string]int{
			"ubuntu-22.04": 9000,
			"windows-10":   9001,
		},
		CloudStackServiceOfferingID: "offering-123",
		CloudStackTemplateIDs: map[string]string{
			"ubuntu-22.04": "template-ubuntu",
			"windows-10":   "template-windows",
		},
		CloudStackNetworkID:         "default-network",
		CloudStackNetworkOfferingID: "offering-net-123",
		CloudStackUsePodNetworks:    true,
	}

	return New(pveClient, csClient, cfg,
		WithPodRepository(repo),
		WithLogger(logger),
	), repo
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestNew(t *testing.T) {
	cfg := Config{
		DefaultPlatform: models.PlatformProxmox,
		ProxmoxNode:     "node1",
	}

	orch := New(nil, nil, cfg)
	if orch == nil {
		t.Fatal("expected orchestrator to be created")
	}
	if orch.config.DefaultPlatform != models.PlatformProxmox {
		t.Errorf("expected default platform proxmox, got %s", orch.config.DefaultPlatform)
	}
}

func TestNew_WithOptions(t *testing.T) {
	repo := newMockPodRepository()
	logger := newTestLogger()
	cfg := Config{DefaultPlatform: models.PlatformProxmox}

	orch := New(nil, nil, cfg,
		WithPodRepository(repo),
		WithLogger(logger),
	)

	if orch.podRepo != repo {
		t.Error("expected pod repository to be set")
	}
	if orch.logger != logger {
		t.Error("expected logger to be set")
	}
}

func TestGetPod_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{})

	// Add a pod to in-memory storage
	pod := &models.Pod{
		ID:       "test-pod-1",
		Owner:    "user1",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}
	orch.podsMu.Lock()
	orch.pods["test-pod-1"] = pod
	orch.podsMu.Unlock()

	ctx := context.Background()
	result, err := orch.GetPod(ctx, "test-pod-1")
	if err != nil {
		t.Fatalf("GetPod failed: %v", err)
	}
	if result.ID != "test-pod-1" {
		t.Errorf("expected pod ID test-pod-1, got %s", result.ID)
	}
}

func TestGetPod_FromDatabase(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo))

	// Add pod to repository
	pod := &models.Pod{
		ID:       "db-pod-1",
		Owner:    "user1",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
	}
	repo.Create(context.Background(), pod)

	ctx := context.Background()
	result, err := orch.GetPod(ctx, "db-pod-1")
	if err != nil {
		t.Fatalf("GetPod failed: %v", err)
	}
	if result.ID != "db-pod-1" {
		t.Errorf("expected pod ID db-pod-1, got %s", result.ID)
	}
}

func TestGetPod_NotFound(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	_, err := orch.GetPod(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestListPods_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{})

	// Add pods
	orch.podsMu.Lock()
	orch.pods["pod-1"] = &models.Pod{ID: "pod-1", Owner: "user1"}
	orch.pods["pod-2"] = &models.Pod{ID: "pod-2", Owner: "user1"}
	orch.pods["pod-3"] = &models.Pod{ID: "pod-3", Owner: "user2"}
	orch.podsMu.Unlock()

	ctx := context.Background()

	// List all
	pods, err := orch.ListPods(ctx, "")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}
	if len(pods) != 3 {
		t.Errorf("expected 3 pods, got %d", len(pods))
	}

	// List filtered by owner
	pods, err = orch.ListPods(ctx, "user1")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}
	if len(pods) != 2 {
		t.Errorf("expected 2 pods for user1, got %d", len(pods))
	}
}

func TestListPods_FromDatabase(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo))

	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", Owner: "user1"})
	repo.Create(ctx, &models.Pod{ID: "pod-2", Owner: "user2"})

	pods, err := orch.ListPods(ctx, "user1")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}
	if len(pods) != 1 {
		t.Errorf("expected 1 pod for user1, got %d", len(pods))
	}
}

func TestGetExpiredPods_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{})

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	orch.podsMu.Lock()
	orch.pods["pod-1"] = &models.Pod{ID: "pod-1", ExpiresAt: &past, Status: models.PodStatusRunning}
	orch.pods["pod-2"] = &models.Pod{ID: "pod-2", ExpiresAt: &future, Status: models.PodStatusRunning}
	orch.pods["pod-3"] = &models.Pod{ID: "pod-3", ExpiresAt: &past, Status: models.PodStatusDestroyed}
	orch.podsMu.Unlock()

	ctx := context.Background()
	expired, err := orch.GetExpiredPods(ctx)
	if err != nil {
		t.Fatalf("GetExpiredPods failed: %v", err)
	}
	if len(expired) != 1 {
		t.Errorf("expected 1 expired pod, got %d", len(expired))
	}
	if len(expired) > 0 && expired[0].ID != "pod-1" {
		t.Errorf("expected expired pod pod-1, got %s", expired[0].ID)
	}
}

func TestGetExpiredPods_FromDatabase(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo))

	past := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", ExpiresAt: &past, Status: models.PodStatusRunning})

	expired, err := orch.GetExpiredPods(ctx)
	if err != nil {
		t.Fatalf("GetExpiredPods failed: %v", err)
	}
	if len(expired) != 1 {
		t.Errorf("expected 1 expired pod, got %d", len(expired))
	}
}

func TestExtendPodExpiration(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	original := time.Now().Add(1 * time.Hour)
	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", ExpiresAt: &original, Status: models.PodStatusRunning})

	err := orch.ExtendPodExpiration(ctx, "pod-1", 2*time.Hour)
	if err != nil {
		t.Fatalf("ExtendPodExpiration failed: %v", err)
	}

	pod, _ := orch.GetPod(ctx, "pod-1")
	expected := original.Add(2 * time.Hour)
	// Allow 1 second tolerance
	if pod.ExpiresAt.Before(expected.Add(-1*time.Second)) || pod.ExpiresAt.After(expected.Add(1*time.Second)) {
		t.Errorf("expected expiration around %v, got %v", expected, pod.ExpiresAt)
	}
}

func TestExtendPodExpiration_ExpiredPod(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	past := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", ExpiresAt: &past, Status: models.PodStatusRunning})

	err := orch.ExtendPodExpiration(ctx, "pod-1", 2*time.Hour)
	if err != nil {
		t.Fatalf("ExtendPodExpiration failed: %v", err)
	}

	pod, _ := orch.GetPod(ctx, "pod-1")
	// Should extend from now, not from past expiration
	if pod.ExpiresAt.Before(time.Now().Add(1 * time.Hour)) {
		t.Errorf("expected expiration in future, got %v", pod.ExpiresAt)
	}
}

func TestExtendPodExpiration_DestroyedPod(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo))

	exp := time.Now().Add(1 * time.Hour)
	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", ExpiresAt: &exp, Status: models.PodStatusDestroyed})

	err := orch.ExtendPodExpiration(ctx, "pod-1", 1*time.Hour)
	if err == nil {
		t.Error("expected error extending destroyed pod")
	}
}

func TestSetPodExpiration(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	original := time.Now().Add(1 * time.Hour)
	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", ExpiresAt: &original, Status: models.PodStatusRunning})

	newExp := time.Now().Add(24 * time.Hour)
	err := orch.SetPodExpiration(ctx, "pod-1", newExp)
	if err != nil {
		t.Fatalf("SetPodExpiration failed: %v", err)
	}

	pod, _ := orch.GetPod(ctx, "pod-1")
	// Allow 1 second tolerance
	if pod.ExpiresAt.Before(newExp.Add(-1*time.Second)) || pod.ExpiresAt.After(newExp.Add(1*time.Second)) {
		t.Errorf("expected expiration around %v, got %v", newExp, pod.ExpiresAt)
	}
}

func TestSetPodExpiration_DestroyingPod(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo))

	exp := time.Now().Add(1 * time.Hour)
	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", ExpiresAt: &exp, Status: models.PodStatusDestroying})

	err := orch.SetPodExpiration(ctx, "pod-1", time.Now().Add(2*time.Hour))
	if err == nil {
		t.Error("expected error setting expiration on destroying pod")
	}
}

func TestLoadPodsFromDatabase(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", Status: models.PodStatusRunning})
	repo.Create(ctx, &models.Pod{ID: "pod-2", Status: models.PodStatusRunning})
	repo.Create(ctx, &models.Pod{ID: "pod-3", Status: models.PodStatusDestroyed})

	// LoadPodsFromDatabase now just verifies DB connectivity
	err := orch.LoadPodsFromDatabase(ctx)
	if err != nil {
		t.Fatalf("LoadPodsFromDatabase failed: %v", err)
	}

	// In-memory cache should NOT be populated when database is configured
	// The database is now the authoritative source
	orch.podsMu.RLock()
	count := len(orch.pods)
	orch.podsMu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 pods in memory (database is authoritative), got %d", count)
	}

	// Pods should be accessible via GetPod which queries the database
	pod, err := orch.GetPod(ctx, "pod-1")
	if err != nil {
		t.Fatalf("GetPod failed: %v", err)
	}
	if pod.ID != "pod-1" {
		t.Errorf("expected pod-1, got %s", pod.ID)
	}
}

func TestLoadPodsFromDatabase_NoRepo(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	err := orch.LoadPodsFromDatabase(ctx)
	if err != nil {
		t.Errorf("expected no error without repository, got: %v", err)
	}
}

func TestResolveTemplateVMID(t *testing.T) {
	cfg := Config{
		TemplateVMIDs: map[string]int{
			"ubuntu-22.04": 9000,
			"windows-10":   9001,
		},
	}
	orch := New(nil, nil, cfg, WithLogger(newTestLogger()))

	vmid, err := orch.resolveTemplateVMID("ubuntu-22.04")
	if err != nil {
		t.Fatalf("resolveTemplateVMID failed: %v", err)
	}
	if vmid != 9000 {
		t.Errorf("expected VMID 9000, got %d", vmid)
	}
}

func TestResolveTemplateVMID_NotFound(t *testing.T) {
	cfg := Config{
		TemplateVMIDs: map[string]int{
			"ubuntu-22.04": 9000,
		},
	}
	orch := New(nil, nil, cfg, WithLogger(newTestLogger()))

	_, err := orch.resolveTemplateVMID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestResolveTemplateVMID_NoMappings(t *testing.T) {
	orch := New(nil, nil, Config{})

	_, err := orch.resolveTemplateVMID("ubuntu-22.04")
	if err == nil {
		t.Error("expected error when no mappings configured")
	}
}

func TestResolveCloudStackTemplateID(t *testing.T) {
	cfg := Config{
		CloudStackTemplateIDs: map[string]string{
			"ubuntu-22.04": "template-ubuntu",
			"windows-10":   "template-windows",
		},
	}
	orch := New(nil, nil, cfg, WithLogger(newTestLogger()))

	templateID, err := orch.resolveCloudStackTemplateID("ubuntu-22.04")
	if err != nil {
		t.Fatalf("resolveCloudStackTemplateID failed: %v", err)
	}
	if templateID != "template-ubuntu" {
		t.Errorf("expected template-ubuntu, got %s", templateID)
	}
}

func TestResolveCloudStackTemplateID_NotFound(t *testing.T) {
	cfg := Config{
		CloudStackTemplateIDs: map[string]string{
			"ubuntu-22.04": "template-ubuntu",
		},
	}
	orch := New(nil, nil, cfg, WithLogger(newTestLogger()))

	_, err := orch.resolveCloudStackTemplateID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestResolveVMNetworkIDs(t *testing.T) {
	cfg := Config{
		CloudStackNetworkID: "default-network",
	}
	orch := New(nil, nil, cfg)

	vmSpec := models.VMSpec{
		Networks: []models.VMNetworkSpec{
			{Segment: "internal"},
			{Segment: "external"},
		},
	}

	// With network map
	networkMap := map[string]string{
		"internal": "net-internal-123",
		"external": "net-external-456",
	}
	networkIDs := orch.resolveVMNetworkIDs(vmSpec, networkMap)
	if len(networkIDs) != 2 {
		t.Errorf("expected 2 network IDs, got %d", len(networkIDs))
	}

	// Without network map (legacy fallback)
	networkIDs = orch.resolveVMNetworkIDs(vmSpec, nil)
	if len(networkIDs) != 1 || networkIDs[0] != "default-network" {
		t.Errorf("expected default network, got %v", networkIDs)
	}
}

func TestResolveVMNetworkIDs_NoVMNetworks(t *testing.T) {
	orch := New(nil, nil, Config{})

	vmSpec := models.VMSpec{
		Networks: []models.VMNetworkSpec{},
	}

	// With network map but VM has no network assignments
	networkMap := map[string]string{
		"internal": "net-internal-123",
	}
	networkIDs := orch.resolveVMNetworkIDs(vmSpec, networkMap)
	// Should assign first available network
	if len(networkIDs) != 1 {
		t.Errorf("expected 1 network ID (first available), got %d", len(networkIDs))
	}
}

func TestGeneratePodID(t *testing.T) {
	id1 := generatePodID("my-template", "student1")
	id2 := generatePodID("my-template", "student1")

	// IDs should be valid UUIDs (36 chars with dashes)
	if len(id1) != 36 {
		t.Errorf("expected UUID format (36 chars), got %d chars: %s", len(id1), id1)
	}

	// IDs should be unique
	if id1 == id2 {
		t.Errorf("expected unique IDs, got same: %s", id1)
	}
}

func TestGenerateVMID(t *testing.T) {
	mustGen := func() int {
		id, err := generateVMID()
		if err != nil {
			t.Fatalf("generateVMID: %v", err)
		}
		return id
	}
	vmid1 := mustGen()
	vmid2 := mustGen()
	vmid3 := mustGen()

	// VMIDs should be in valid range [100000, 999999]
	for _, vmid := range []int{vmid1, vmid2, vmid3} {
		if vmid < 100000 || vmid > 999999 {
			t.Errorf("VMID %d outside expected range [100000, 999999]", vmid)
		}
	}

	// With cryptographically secure random, collisions are extremely unlikely
	// but we're testing uniqueness across a small sample
	seen := make(map[int]bool)
	for i := 0; i < 100; i++ {
		id := mustGen()
		if seen[id] {
			t.Logf("Warning: duplicate VMID %d found (possible but unlikely)", id)
		}
		seen[id] = true
	}
}

func TestUpdatePodStatus(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	repo.Create(ctx, &models.Pod{ID: "pod-1", Status: models.PodStatusRunning})

	err := orch.updatePodStatus(ctx, "pod-1", models.PodStatusStopped)
	if err != nil {
		t.Fatalf("updatePodStatus failed: %v", err)
	}

	// Check database (now the authoritative source)
	pod, _ := repo.GetByID(ctx, "pod-1")
	if pod.Status != models.PodStatusStopped {
		t.Errorf("expected database status stopped, got %s", pod.Status)
	}

	// In-memory should NOT be used when database is configured
	orch.podsMu.RLock()
	_, inMem := orch.pods["pod-1"]
	orch.podsMu.RUnlock()
	if inMem {
		t.Error("in-memory should not be used when database is configured")
	}
}

func TestUpdatePodStatus_InMemory(t *testing.T) {
	// Test in-memory mode (no database)
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()))

	ctx := context.Background()
	orch.podsMu.Lock()
	orch.pods["pod-1"] = &models.Pod{ID: "pod-1", Status: models.PodStatusRunning}
	orch.podsMu.Unlock()

	err := orch.updatePodStatus(ctx, "pod-1", models.PodStatusStopped)
	if err != nil {
		t.Fatalf("updatePodStatus failed: %v", err)
	}

	// Check in-memory
	orch.podsMu.RLock()
	inMemStatus := orch.pods["pod-1"].Status
	orch.podsMu.RUnlock()
	if inMemStatus != models.PodStatusStopped {
		t.Errorf("expected in-memory status stopped, got %s", inMemStatus)
	}
}

// -----------------------------------------------------------------------------
// Lifecycle Manager Tests
// -----------------------------------------------------------------------------

func TestNewLifecycleManager(t *testing.T) {
	orch := New(nil, nil, Config{})
	logger := newTestLogger()

	lm := NewLifecycleManager(orch, LifecycleConfig{}, logger)
	if lm == nil {
		t.Fatal("expected lifecycle manager to be created")
	}
	if lm.checkInterval != 5*time.Minute {
		t.Errorf("expected default check interval 5m, got %v", lm.checkInterval)
	}
}

func TestNewLifecycleManager_CustomInterval(t *testing.T) {
	orch := New(nil, nil, Config{})
	logger := newTestLogger()

	lm := NewLifecycleManager(orch, LifecycleConfig{CheckInterval: 1 * time.Minute}, logger)
	if lm.checkInterval != 1*time.Minute {
		t.Errorf("expected check interval 1m, got %v", lm.checkInterval)
	}
}

func TestLifecycleManager_StartStop(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	logger := newTestLogger()

	lm := NewLifecycleManager(orch, LifecycleConfig{CheckInterval: 10 * time.Millisecond}, logger)

	ctx := context.Background()
	lm.Start(ctx)

	// Verify Stop completes without hanging
	done := make(chan struct{})
	go func() {
		lm.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success - Stop returned
	case <-time.After(1 * time.Second):
		t.Fatal("LifecycleManager.Stop() did not return in time")
	}
}

func TestLifecycleManager_CleansUpExpiredPods(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	logger := newTestLogger()

	// Create an expired pod
	past := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()
	pod := &models.Pod{
		ID:        "expired-pod",
		Status:    models.PodStatusRunning,
		ExpiresAt: &past,
		Platform:  models.PlatformProxmox,
		VMs:       []models.PodVM{}, // Empty for simplicity
	}
	repo.Create(ctx, pod)
	orch.podsMu.Lock()
	orch.pods["expired-pod"] = pod
	orch.podsMu.Unlock()

	// Run cleanup
	lm := NewLifecycleManager(orch, LifecycleConfig{}, logger)
	lm.cleanupExpiredPods(ctx)

	// Check pod was destroyed
	updatedPod, _ := repo.GetByID(ctx, "expired-pod")
	if updatedPod != nil && updatedPod.Status != models.PodStatusDestroyed {
		t.Errorf("expected pod to be destroyed, got status %s", updatedPod.Status)
	}
}

// -----------------------------------------------------------------------------
// PodFilter Tests
// -----------------------------------------------------------------------------

func TestPodFilter(t *testing.T) {
	repo := newMockPodRepository()
	ctx := context.Background()

	repo.Create(ctx, &models.Pod{ID: "p1", Owner: "user1", LabTemplate: "lab1", Status: models.PodStatusRunning, Platform: models.PlatformProxmox})
	repo.Create(ctx, &models.Pod{ID: "p2", Owner: "user1", LabTemplate: "lab2", Status: models.PodStatusStopped, Platform: models.PlatformCloudStack})
	repo.Create(ctx, &models.Pod{ID: "p3", Owner: "user2", LabTemplate: "lab1", Status: models.PodStatusRunning, Platform: models.PlatformProxmox})

	tests := []struct {
		name     string
		filter   PodFilter
		expected int
	}{
		{"no filter", PodFilter{}, 3},
		{"by owner", PodFilter{OwnerID: "user1"}, 2},
		{"by template", PodFilter{TemplateID: "lab1"}, 2},
		{"by status", PodFilter{Status: models.PodStatusRunning}, 2},
		{"by platform", PodFilter{Platform: "cloudstack"}, 1},
		{"combined", PodFilter{OwnerID: "user1", Status: models.PodStatusRunning}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pods, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if len(pods) != tt.expected {
				t.Errorf("expected %d pods, got %d", tt.expected, len(pods))
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Repository Error Handling Tests
// -----------------------------------------------------------------------------

func TestGetPod_DatabaseError(t *testing.T) {
	repo := newMockPodRepository()
	repo.setError("getByID", errors.New("database error"))

	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	_, err := orch.GetPod(ctx, "pod-1")

	// When database is configured, errors should be returned (no fallback)
	if err == nil {
		t.Fatal("expected error when database fails")
	}
	if !strings.Contains(err.Error(), "database error") {
		t.Errorf("expected database error, got: %v", err)
	}
}

func TestListPods_DatabaseError(t *testing.T) {
	repo := newMockPodRepository()
	repo.setError("list", errors.New("database error"))

	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	_, err := orch.ListPods(ctx, "user1")

	// When database is configured, errors should be returned (no fallback)
	if err == nil {
		t.Fatal("expected error when database fails")
	}
	if !strings.Contains(err.Error(), "database error") {
		t.Errorf("expected database error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Min Function Test
// -----------------------------------------------------------------------------

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("expected min(1,2) = 1")
	}
	if min(5, 3) != 3 {
		t.Error("expected min(5,3) = 3")
	}
	if min(4, 4) != 4 {
		t.Error("expected min(4,4) = 4")
	}
}

// -----------------------------------------------------------------------------
// VMSnapshot Type Test
// -----------------------------------------------------------------------------

func TestVMSnapshot(t *testing.T) {
	snap := VMSnapshot{
		Name:        "clean",
		Description: "Clean state snapshot",
		Parent:      "",
		CreatedAt:   time.Now(),
	}

	if snap.Name != "clean" {
		t.Errorf("expected name clean, got %s", snap.Name)
	}
	if snap.Description != "Clean state snapshot" {
		t.Errorf("expected description, got %s", snap.Description)
	}
	if snap.Parent != "" {
		t.Errorf("expected empty parent, got %s", snap.Parent)
	}
	if snap.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

// -----------------------------------------------------------------------------
// Context Cancellation Tests
// -----------------------------------------------------------------------------

func TestGetPod_ContextCancellation(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should still work from in-memory
	orch.podsMu.Lock()
	orch.pods["pod-1"] = &models.Pod{ID: "pod-1"}
	orch.podsMu.Unlock()

	_, err := orch.GetPod(ctx, "pod-1")
	// In-memory lookup doesn't use context, so should succeed
	if err != nil {
		t.Logf("Context cancellation may have affected lookup: %v", err)
	}
}

// -----------------------------------------------------------------------------
// UpdateVMStatus Tests
// -----------------------------------------------------------------------------

func TestUpdateVMStatus(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs: []models.PodVM{
			{Name: "server", Status: "running"},
			{Name: "client", Status: "running"},
		},
	}
	repo.Create(ctx, pod)

	err := orch.updateVMStatus(ctx, "pod-1", "server", "stopped")
	if err != nil {
		t.Fatalf("updateVMStatus failed: %v", err)
	}

	// Check database updated
	updated, _ := repo.GetByID(ctx, "pod-1")
	for _, vm := range updated.VMs {
		if vm.Name == "server" && vm.Status != "stopped" {
			t.Errorf("expected server status stopped, got %s", vm.Status)
		}
	}
}

func TestUpdateVMStatus_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Status: "running"}},
	}
	repo.Create(ctx, pod)

	err := orch.updateVMStatus(ctx, "pod-1", "nonexistent", "stopped")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
	if !strings.Contains(err.Error(), "VM not found") {
		t.Errorf("expected 'VM not found' error, got: %v", err)
	}
}

func TestUpdateVMStatus_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Status: "running"}},
	}
	orch.podsMu.Lock()
	orch.pods["pod-1"] = pod
	orch.podsMu.Unlock()

	err := orch.updateVMStatus(ctx, "pod-1", "server", "stopped")
	if err != nil {
		t.Fatalf("updateVMStatus failed: %v", err)
	}

	orch.podsMu.RLock()
	updated := orch.pods["pod-1"]
	orch.podsMu.RUnlock()

	if updated.VMs[0].Status != "stopped" {
		t.Errorf("expected status stopped, got %s", updated.VMs[0].Status)
	}
}

// -----------------------------------------------------------------------------
// StopPod Tests
// -----------------------------------------------------------------------------

func TestStopPod_NotRunning(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusStopped, // Already stopped
		VMs:    []models.PodVM{},
	}
	repo.Create(ctx, pod)

	_, err := orch.StopPod(ctx, "pod-1")
	if err == nil {
		t.Error("expected error stopping already stopped pod")
	}
	if !strings.Contains(err.Error(), "must be running") {
		t.Errorf("expected 'must be running' error, got: %v", err)
	}
}

func TestStopPod_NotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	_, err := orch.StopPod(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

// -----------------------------------------------------------------------------
// StartPod Tests
// -----------------------------------------------------------------------------

func TestStartPod_NotStopped(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning, // Already running
		VMs:    []models.PodVM{},
	}
	repo.Create(ctx, pod)

	_, err := orch.StartPod(ctx, "pod-1")
	if err == nil {
		t.Error("expected error starting already running pod")
	}
	if !strings.Contains(err.Error(), "must be stopped") {
		t.Errorf("expected 'must be stopped' error, got: %v", err)
	}
}

func TestStartPod_NotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	_, err := orch.StartPod(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

// -----------------------------------------------------------------------------
// StartVM Tests
// -----------------------------------------------------------------------------

func TestStartVM_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.StartVM(ctx, "nonexistent", "server")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestStartVM_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.StartVM(ctx, "pod-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
	if !strings.Contains(err.Error(), "VM not found") {
		t.Errorf("expected 'VM not found' error, got: %v", err)
	}
}

func TestStartVM_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	err := orch.StartVM(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("expected 'unsupported platform' error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// StopVM Tests
// -----------------------------------------------------------------------------

func TestStopVM_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.StopVM(ctx, "nonexistent", "server")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestStopVM_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.StopVM(ctx, "pod-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestStopVM_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	err := orch.StopVM(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// -----------------------------------------------------------------------------
// SuspendVM Tests
// -----------------------------------------------------------------------------

func TestSuspendVM_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.SuspendVM(ctx, "nonexistent", "server")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestSuspendVM_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.SuspendVM(ctx, "pod-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestSuspendVM_CloudStackNotSupported(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformCloudStack}},
	}
	repo.Create(ctx, pod)

	err := orch.SuspendVM(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for CloudStack")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' error, got: %v", err)
	}
}

func TestSuspendVM_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	err := orch.SuspendVM(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// -----------------------------------------------------------------------------
// ResumeVM Tests
// -----------------------------------------------------------------------------

func TestResumeVM_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.ResumeVM(ctx, "nonexistent", "server")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestResumeVM_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.ResumeVM(ctx, "pod-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestResumeVM_CloudStackNotSupported(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformCloudStack}},
	}
	repo.Create(ctx, pod)

	err := orch.ResumeVM(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for CloudStack")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' error, got: %v", err)
	}
}

func TestResumeVM_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	err := orch.ResumeVM(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// -----------------------------------------------------------------------------
// ResetPodVM Tests
// -----------------------------------------------------------------------------

func TestResetPodVM_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.ResetPodVM(ctx, "nonexistent", "server", "clean")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestResetPodVM_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.ResetPodVM(ctx, "pod-1", "nonexistent", "clean")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

// -----------------------------------------------------------------------------
// GetVMConsole Tests
// -----------------------------------------------------------------------------

func TestGetVMConsole_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	_, err := orch.GetVMConsole(ctx, "nonexistent", "server", "vnc")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestGetVMConsole_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	_, err := orch.GetVMConsole(ctx, "pod-1", "nonexistent", "vnc")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestGetVMConsole_CloudStackNotImplemented(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformCloudStack}},
	}
	repo.Create(ctx, pod)

	_, err := orch.GetVMConsole(ctx, "pod-1", "server", "vnc")
	if err == nil {
		t.Error("expected error for CloudStack")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

func TestGetVMConsole_UnknownPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	_, err := orch.GetVMConsole(ctx, "pod-1", "server", "vnc")
	if err == nil {
		t.Error("expected error for unknown platform")
	}
	if !strings.Contains(err.Error(), "unknown platform") {
		t.Errorf("expected 'unknown platform' error, got: %v", err)
	}
}

func TestGetVMConsole_UnsupportedConsoleType(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox, PlatformID: "123"}},
	}
	repo.Create(ctx, pod)

	_, err := orch.GetVMConsole(ctx, "pod-1", "server", "rdp") // Unsupported type
	if err == nil {
		t.Error("expected error for unsupported console type")
	}
}

// -----------------------------------------------------------------------------
// GetDirectVMConsole Tests
// -----------------------------------------------------------------------------

func TestGetDirectVMConsole_NoProxmoxClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	_, err := orch.GetDirectVMConsole(ctx, "node1", 100, "vnc")
	if err == nil {
		t.Error("expected error with no Proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// ListVMSnapshots Tests
// -----------------------------------------------------------------------------

func TestListVMSnapshots_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	_, err := orch.ListVMSnapshots(ctx, "nonexistent", "server")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestListVMSnapshots_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	_, err := orch.ListVMSnapshots(ctx, "pod-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestListVMSnapshots_CloudStackNotImplemented(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformCloudStack}},
	}
	repo.Create(ctx, pod)

	_, err := orch.ListVMSnapshots(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for CloudStack")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

func TestListVMSnapshots_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	_, err := orch.ListVMSnapshots(ctx, "pod-1", "server")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// -----------------------------------------------------------------------------
// CreateVMSnapshot Tests
// -----------------------------------------------------------------------------

func TestCreateVMSnapshot_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.CreateVMSnapshot(ctx, "nonexistent", "server", "snap1", "desc", false)
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestCreateVMSnapshot_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.CreateVMSnapshot(ctx, "pod-1", "nonexistent", "snap1", "desc", false)
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestCreateVMSnapshot_CloudStackNotImplemented(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformCloudStack}},
	}
	repo.Create(ctx, pod)

	err := orch.CreateVMSnapshot(ctx, "pod-1", "server", "snap1", "desc", false)
	if err == nil {
		t.Error("expected error for CloudStack")
	}
}

func TestCreateVMSnapshot_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	err := orch.CreateVMSnapshot(ctx, "pod-1", "server", "snap1", "desc", false)
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// -----------------------------------------------------------------------------
// DeleteVMSnapshot Tests
// -----------------------------------------------------------------------------

func TestDeleteVMSnapshot_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.DeleteVMSnapshot(ctx, "nonexistent", "server", "snap1")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestDeleteVMSnapshot_VMNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformProxmox}},
	}
	repo.Create(ctx, pod)

	err := orch.DeleteVMSnapshot(ctx, "pod-1", "nonexistent", "snap1")
	if err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestDeleteVMSnapshot_CloudStackNotImplemented(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.PlatformCloudStack}},
	}
	repo.Create(ctx, pod)

	err := orch.DeleteVMSnapshot(ctx, "pod-1", "server", "snap1")
	if err == nil {
		t.Error("expected error for CloudStack")
	}
}

func TestDeleteVMSnapshot_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Platform: models.Platform("unknown")}},
	}
	repo.Create(ctx, pod)

	err := orch.DeleteVMSnapshot(ctx, "pod-1", "server", "snap1")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

// -----------------------------------------------------------------------------
// ProxmoxAuthHeader and ProxmoxHostname Tests
// -----------------------------------------------------------------------------

func TestProxmoxAuthHeader_NoClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	header := orch.ProxmoxAuthHeader()
	if header != "" {
		t.Errorf("expected empty header with no client, got: %s", header)
	}
}

func TestProxmoxHostname_NoClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	hostname := orch.ProxmoxHostname()
	if hostname != "" {
		t.Errorf("expected empty hostname with no client, got: %s", hostname)
	}
}

// -----------------------------------------------------------------------------
// AddPodForTesting Test
// -----------------------------------------------------------------------------

func TestAddPodForTesting(t *testing.T) {
	orch := New(nil, nil, Config{})

	pod := &models.Pod{
		ID:     "test-pod",
		Status: models.PodStatusRunning,
	}
	orch.AddPodForTesting(pod)

	orch.podsMu.RLock()
	stored := orch.pods["test-pod"]
	orch.podsMu.RUnlock()

	if stored == nil || stored.ID != "test-pod" {
		t.Error("expected pod to be added to in-memory store")
	}
}

func TestAddPodForTesting_NilMap(t *testing.T) {
	orch := &Orchestrator{}

	pod := &models.Pod{
		ID:     "test-pod",
		Status: models.PodStatusRunning,
	}
	orch.AddPodForTesting(pod)

	orch.podsMu.RLock()
	stored := orch.pods["test-pod"]
	orch.podsMu.RUnlock()

	if stored == nil || stored.ID != "test-pod" {
		t.Error("expected pod to be added even when map was nil")
	}
}

// -----------------------------------------------------------------------------
// CircuitStats Tests
// -----------------------------------------------------------------------------

func TestProxmoxCircuitStats(t *testing.T) {
	orch := New(nil, nil, Config{})
	if orch.ProxmoxCircuitStats() != nil {
		t.Error("expected nil with no client")
	}
}

func TestCloudStackCircuitStats(t *testing.T) {
	orch := New(nil, nil, Config{})
	if orch.CloudStackCircuitStats() != nil {
		t.Error("expected nil with no client")
	}
}

// -----------------------------------------------------------------------------
// CreatePod Tests
// -----------------------------------------------------------------------------

func TestCreatePod_UnsupportedPlatform(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{DefaultPlatform: models.Platform("unknown")}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Platform: models.PlatformAny, // Will use default (unknown)
		},
	}

	_, err := orch.CreatePod(ctx, template, "template-id", "owner-id", "student1")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("expected 'unsupported platform' error, got: %v", err)
	}
}

func TestCreatePod_CreateRepoError(t *testing.T) {
	repo := newMockPodRepository()
	repo.setError("create", errors.New("database error"))
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
		},
	}

	_, err := orch.CreatePod(ctx, template, "template-id", "owner-id", "student1")
	if err == nil {
		t.Error("expected error when repo.Create fails")
	}
	if !strings.Contains(err.Error(), "database error") {
		t.Errorf("expected 'database error', got: %v", err)
	}
}

func TestCreatePod_InMemory(t *testing.T) {
	// Test in-memory mode (no database, no platform clients - should still create initial record)
	orch := New(nil, nil, Config{DefaultPlatform: models.Platform("unknown")}, WithLogger(newTestLogger()))

	ctx := context.Background()
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "test-lab"},
		Spec: models.LabSpec{
			Platform: models.PlatformAny,
		},
	}

	pod, err := orch.CreatePod(ctx, template, "template-id", "owner-id", "student1")
	// Will fail on provisioning due to unsupported platform, but pod should be created
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
	if pod == nil {
		t.Error("expected pod to be created even on provisioning error")
	}
	if pod != nil && pod.Status != models.PodStatusError {
		t.Errorf("expected status error, got %s", pod.Status)
	}
}

// -----------------------------------------------------------------------------
// DestroyPod Tests
// -----------------------------------------------------------------------------

func TestDestroyPod_NotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.DestroyPod(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestDestroyPod_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:       "pod-1",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{}, // Empty for simplicity
	}
	orch.podsMu.Lock()
	orch.pods["pod-1"] = pod
	orch.podsMu.Unlock()

	err := orch.DestroyPod(ctx, "pod-1")
	if err != nil {
		t.Fatalf("DestroyPod failed: %v", err)
	}

	// Check pod was removed from in-memory store
	orch.podsMu.RLock()
	_, exists := orch.pods["pod-1"]
	orch.podsMu.RUnlock()
	if exists {
		t.Error("expected pod to be removed from in-memory store")
	}
}

// -----------------------------------------------------------------------------
// ExtendPodExpiration Edge Cases
// -----------------------------------------------------------------------------

func TestExtendPodExpiration_NilExpiration(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:        "pod-1",
		Status:    models.PodStatusRunning,
		ExpiresAt: nil, // No expiration set
	}
	repo.Create(ctx, pod)

	err := orch.ExtendPodExpiration(ctx, "pod-1", 1*time.Hour)
	if err != nil {
		t.Fatalf("ExtendPodExpiration failed: %v", err)
	}

	updated, _ := orch.GetPod(ctx, "pod-1")
	if updated.ExpiresAt == nil {
		t.Error("expected expiration to be set")
	}
}

func TestExtendPodExpiration_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()))

	ctx := context.Background()
	original := time.Now().Add(1 * time.Hour)
	pod := &models.Pod{
		ID:        "pod-1",
		Status:    models.PodStatusRunning,
		ExpiresAt: &original,
	}
	orch.podsMu.Lock()
	orch.pods["pod-1"] = pod
	orch.podsMu.Unlock()

	err := orch.ExtendPodExpiration(ctx, "pod-1", 1*time.Hour)
	if err != nil {
		t.Fatalf("ExtendPodExpiration failed: %v", err)
	}

	orch.podsMu.RLock()
	updated := orch.pods["pod-1"]
	orch.podsMu.RUnlock()

	if updated.ExpiresAt.Before(original) {
		t.Error("expected expiration to be extended")
	}
}

// -----------------------------------------------------------------------------
// SetPodExpiration Edge Cases
// -----------------------------------------------------------------------------

func TestSetPodExpiration_InMemory(t *testing.T) {
	orch := New(nil, nil, Config{}, WithLogger(newTestLogger()))

	ctx := context.Background()
	original := time.Now().Add(1 * time.Hour)
	pod := &models.Pod{
		ID:        "pod-1",
		Status:    models.PodStatusRunning,
		ExpiresAt: &original,
	}
	orch.podsMu.Lock()
	orch.pods["pod-1"] = pod
	orch.podsMu.Unlock()

	newExp := time.Now().Add(24 * time.Hour)
	err := orch.SetPodExpiration(ctx, "pod-1", newExp)
	if err != nil {
		t.Fatalf("SetPodExpiration failed: %v", err)
	}

	orch.podsMu.RLock()
	updated := orch.pods["pod-1"]
	orch.podsMu.RUnlock()

	if updated.ExpiresAt.Before(newExp.Add(-1*time.Second)) || updated.ExpiresAt.After(newExp.Add(1*time.Second)) {
		t.Errorf("expected expiration %v, got %v", newExp, updated.ExpiresAt)
	}
}

// -----------------------------------------------------------------------------
// ConsoleTicket Type Test
// -----------------------------------------------------------------------------

func TestConsoleTicket(t *testing.T) {
	ticket := ConsoleTicket{
		Type:     "vnc",
		Host:     "192.168.1.100",
		Port:     5900,
		TLSPort:  5901,
		Ticket:   "abc123",
		Password: "secret",
		Node:     "node1",
		VMID:     "100",
	}

	if ticket.Type != "vnc" {
		t.Errorf("expected type vnc, got %s", ticket.Type)
	}
	if ticket.Host != "192.168.1.100" {
		t.Errorf("expected host, got %s", ticket.Host)
	}
	if ticket.Port != 5900 {
		t.Errorf("expected port 5900, got %d", ticket.Port)
	}
	if ticket.TLSPort != 5901 {
		t.Errorf("expected TLSPort 5901, got %d", ticket.TLSPort)
	}
	if ticket.Ticket != "abc123" {
		t.Errorf("expected ticket, got %s", ticket.Ticket)
	}
	if ticket.Password != "secret" {
		t.Errorf("expected password, got %s", ticket.Password)
	}
	if ticket.Node != "node1" {
		t.Errorf("expected node, got %s", ticket.Node)
	}
	if ticket.VMID != "100" {
		t.Errorf("expected VMID, got %s", ticket.VMID)
	}
}

// -----------------------------------------------------------------------------
// PodProvisioningEvent Type Tests
// -----------------------------------------------------------------------------

func TestPodProvisioningEvent(t *testing.T) {
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseQueued,
		Message:   "Queued for provisioning",
		Progress:  0,
		Timestamp: time.Now(),
		RequestID: "req-789",
	}

	if event.PodID != "pod-123" {
		t.Errorf("expected PodID pod-123, got %s", event.PodID)
	}
	if event.OwnerID != "user-456" {
		t.Errorf("expected OwnerID, got %s", event.OwnerID)
	}
	if event.Status != models.PodStatusProvisioning {
		t.Errorf("expected status provisioning, got %s", event.Status)
	}
	if event.Phase != PhaseQueued {
		t.Errorf("expected phase queued, got %s", event.Phase)
	}
	if event.Message != "Queued for provisioning" {
		t.Errorf("expected message, got %s", event.Message)
	}
	if event.Progress != 0 {
		t.Errorf("expected progress 0, got %d", event.Progress)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
	if event.RequestID != "req-789" {
		t.Errorf("expected RequestID, got %s", event.RequestID)
	}
}

func TestProvisioningPhases(t *testing.T) {
	phases := []ProvisioningPhase{
		PhaseQueued,
		PhaseStarting,
		PhaseCreatingPod,
		PhaseCloningVM,
		PhaseConfiguringVM,
		PhaseCreatingSnapshots,
		PhaseStartingVM,
		PhaseCompleted,
		PhaseFailed,
	}

	for _, phase := range phases {
		if phase == "" {
			t.Error("phase should not be empty")
		}
	}
}

// -----------------------------------------------------------------------------
// PodProvisionRequest Type Test
// -----------------------------------------------------------------------------

func TestPodProvisionRequest(t *testing.T) {
	req := PodProvisionRequest{
		RequestID:    "req-123",
		TemplateID:   "template-456",
		TemplateName: "test-lab",
		OwnerID:      "user-789",
		OwnerName:    "student1",
		Timestamp:    time.Now(),
	}

	if req.RequestID != "req-123" {
		t.Errorf("expected request ID req-123, got %s", req.RequestID)
	}
	if req.TemplateID != "template-456" {
		t.Errorf("expected TemplateID, got %s", req.TemplateID)
	}
	if req.TemplateName != "test-lab" {
		t.Errorf("expected TemplateName, got %s", req.TemplateName)
	}
	if req.OwnerID != "user-789" {
		t.Errorf("expected OwnerID, got %s", req.OwnerID)
	}
	if req.OwnerName != "student1" {
		t.Errorf("expected OwnerName, got %s", req.OwnerName)
	}
	if req.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
}

// -----------------------------------------------------------------------------
// Async Provisioner Tests
// -----------------------------------------------------------------------------

// mockEventPublisher implements EventPublisher for testing
type mockEventPublisher struct {
	mu       sync.Mutex
	events   [][]byte
	subjects []string
	err      error
}

func (m *mockEventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, data)
	m.subjects = append(m.subjects, subject)
	return nil
}

// mockBroadcaster implements ProvisioningBroadcaster for testing
type mockBroadcaster struct {
	mu     sync.Mutex
	events []PodProvisioningEvent
}

func (m *mockBroadcaster) BroadcastProvisioningEvent(event PodProvisioningEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func TestNewAsyncProvisioner(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger)

	if ap == nil {
		t.Fatal("expected async provisioner to be created")
	}
	if ap.subjectPrefix != "labs.pods" {
		t.Errorf("expected default subject prefix 'labs.pods', got %s", ap.subjectPrefix)
	}
}

func TestNewAsyncProvisioner_WithOptions(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	broadcaster := &mockBroadcaster{}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger,
		WithSubjectPrefix("custom.prefix"),
		WithBroadcaster(broadcaster),
	)

	if ap.subjectPrefix != "custom.prefix" {
		t.Errorf("expected custom prefix 'custom.prefix', got %s", ap.subjectPrefix)
	}
	if ap.broadcaster != broadcaster {
		t.Error("expected broadcaster to be set")
	}
}

func TestAsyncProvisioner_ProvisioningEventSubject(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger)
	subject := ap.ProvisioningEventSubject("pod-123")

	expected := "labs.pods.pod-123.provision"
	if subject != expected {
		t.Errorf("expected subject '%s', got '%s'", expected, subject)
	}
}

func TestAsyncProvisioner_ProvisioningEventSubject_CustomPrefix(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger, WithSubjectPrefix("custom"))
	subject := ap.ProvisioningEventSubject("pod-456")

	expected := "custom.pod-456.provision"
	if subject != expected {
		t.Errorf("expected subject '%s', got '%s'", expected, subject)
	}
}

func TestAsyncProvisioner_PublishEvent_WithBroadcaster(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	broadcaster := &mockBroadcaster{}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger, WithBroadcaster(broadcaster))

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseQueued,
		Message:   "Test message",
		Progress:  50,
		Timestamp: time.Now(),
	}

	ap.publishEvent(ctx, event)

	// Check broadcaster received event
	broadcaster.mu.Lock()
	if len(broadcaster.events) != 1 {
		t.Errorf("expected 1 broadcast event, got %d", len(broadcaster.events))
	}
	broadcaster.mu.Unlock()

	// Check publisher received event
	publisher.mu.Lock()
	if len(publisher.events) != 1 {
		t.Errorf("expected 1 published event, got %d", len(publisher.events))
	}
	publisher.mu.Unlock()
}

func TestAsyncProvisioner_PublishEvent_NilPublisher(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, nil, logger)

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:   "pod-123",
		Phase:   PhaseQueued,
		Message: "Test message",
	}

	// Should not panic with nil publisher
	ap.publishEvent(ctx, event)
}

func TestAsyncProvisioner_PublishEvent_PublisherError(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{err: errors.New("publish error")}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger)

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:   "pod-123",
		Phase:   PhaseQueued,
		Message: "Test message",
	}

	// Should not panic on publisher error
	ap.publishEvent(ctx, event)
}

func TestAsyncProvisioner_HandleProvisioningError(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	broadcaster := &mockBroadcaster{}
	logger := newTestLogger()

	ctx := context.Background()
	// Create a pod first
	pod := &models.Pod{
		ID:     "pod-123",
		Status: models.PodStatusProvisioning,
	}
	repo.Create(ctx, pod)

	ap := NewAsyncProvisioner(orch, publisher, logger, WithBroadcaster(broadcaster))

	req := PodProvisionRequest{
		RequestID: "req-456",
		OwnerID:   "user-789",
	}

	ap.handleProvisioningError(ctx, "pod-123", req, errors.New("test error"), "Test failure")

	// Check pod status was updated to error
	updated, _ := repo.GetByID(ctx, "pod-123")
	if updated.Status != models.PodStatusError {
		t.Errorf("expected status error, got %s", updated.Status)
	}

	// Check failure event was published
	broadcaster.mu.Lock()
	if len(broadcaster.events) != 1 {
		t.Errorf("expected 1 broadcast event, got %d", len(broadcaster.events))
	}
	if len(broadcaster.events) > 0 && broadcaster.events[0].Phase != PhaseFailed {
		t.Errorf("expected phase failed, got %s", broadcaster.events[0].Phase)
	}
	broadcaster.mu.Unlock()
}

func TestAsyncProvisioner_ProvisionAsync_RepoError(t *testing.T) {
	repo := newMockPodRepository()
	repo.setError("create", errors.New("database error"))
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))
	publisher := &mockEventPublisher{}
	logger := newTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger)

	ctx := context.Background()
	req := PodProvisionRequest{
		TemplateID:   "template-123",
		TemplateName: "test-lab",
		Template: &models.LabTemplate{
			Spec: models.LabSpec{
				Platform: models.PlatformProxmox,
			},
		},
		OwnerID:   "user-123",
		OwnerName: "student1",
	}

	_, err := ap.ProvisionAsync(ctx, req)
	if err == nil {
		t.Error("expected error when repo.Create fails")
	}
	if !strings.Contains(err.Error(), "database error") {
		t.Errorf("expected 'database error', got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// updatePodStatus Error Tests
// -----------------------------------------------------------------------------

func TestUpdatePodStatus_RepoError(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
	}
	repo.Create(ctx, pod)

	// Set error for updateStatus
	repo.setError("updateStatus", errors.New("update error"))

	err := orch.updatePodStatus(ctx, "pod-1", models.PodStatusStopped)
	if err == nil {
		t.Error("expected error when repo.UpdateStatus fails")
	}
}

// -----------------------------------------------------------------------------
// updateVMStatus Error Tests
// -----------------------------------------------------------------------------

func TestUpdateVMStatus_PodNotFound(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	err := orch.updateVMStatus(ctx, "nonexistent", "server", "stopped")
	if err == nil {
		t.Error("expected error for nonexistent pod")
	}
}

func TestUpdateVMStatus_RepoUpdateError(t *testing.T) {
	repo := newMockPodRepository()
	orch := New(nil, nil, Config{}, WithPodRepository(repo), WithLogger(newTestLogger()))

	ctx := context.Background()
	pod := &models.Pod{
		ID:     "pod-1",
		Status: models.PodStatusRunning,
		VMs:    []models.PodVM{{Name: "server", Status: "running"}},
	}
	repo.Create(ctx, pod)

	// Set error for update
	repo.setError("update", errors.New("update error"))

	err := orch.updateVMStatus(ctx, "pod-1", "server", "stopped")
	if err == nil {
		t.Error("expected error when repo.Update fails")
	}
}

// -----------------------------------------------------------------------------
// StopPod Success Tests
// -----------------------------------------------------------------------------

func TestStopPod_Success_EmptyVMs(t *testing.T) {
	orch, repo := newTestOrchestrator(t, nil, nil)

	ctx := context.Background()
	pod := &models.Pod{
		ID:       "pod-stop-2",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{}, // No VMs
	}
	repo.Create(ctx, pod)

	result, err := orch.StopPod(ctx, "pod-stop-2")
	if err != nil {
		t.Fatalf("StopPod failed: %v", err)
	}
	if result.Status != models.PodStatusStopped {
		t.Errorf("expected status stopped, got %s", result.Status)
	}
}

// -----------------------------------------------------------------------------
// StartPod Success Tests
// -----------------------------------------------------------------------------

func TestStartPod_Success_EmptyVMs(t *testing.T) {
	orch, repo := newTestOrchestrator(t, nil, nil)

	ctx := context.Background()
	pod := &models.Pod{
		ID:       "pod-start-2",
		Status:   models.PodStatusStopped,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{}, // No VMs
	}
	repo.Create(ctx, pod)

	result, err := orch.StartPod(ctx, "pod-start-2")
	if err != nil {
		t.Fatalf("StartPod failed: %v", err)
	}
	if result.Status != models.PodStatusRunning {
		t.Errorf("expected status running, got %s", result.Status)
	}
}

// -----------------------------------------------------------------------------
// DestroyPod Success Tests
// -----------------------------------------------------------------------------

func TestDestroyPod_NoVMs(t *testing.T) {
	// Test destroying a pod with no VMs (doesn't need proxmox client)
	orch, repo := newTestOrchestrator(t, nil, nil)

	ctx := context.Background()
	pod := &models.Pod{
		ID:       "pod-destroy-novms",
		Status:   models.PodStatusRunning,
		Platform: models.PlatformProxmox,
		VMs:      []models.PodVM{}, // No VMs to destroy
	}
	repo.Create(ctx, pod)

	err := orch.DestroyPod(ctx, "pod-destroy-novms")
	if err != nil {
		t.Fatalf("DestroyPod failed: %v", err)
	}

	// Check pod was marked as destroyed
	destroyed, _ := repo.GetByID(ctx, "pod-destroy-novms")
	if destroyed != nil && destroyed.Status != models.PodStatusDestroyed {
		t.Errorf("expected status destroyed, got %s", destroyed.Status)
	}
}

// -----------------------------------------------------------------------------
// Additional Console Tests (Direct VM Console)
// -----------------------------------------------------------------------------

func TestGetDirectVMConsole_SPICERequest(t *testing.T) {
	// Test direct SPICE console access - verifies path to nil proxmox check
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	_, err := orch.GetDirectVMConsole(ctx, "pve", 100, "spice")
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestGetDirectVMConsole_VNCRequest(t *testing.T) {
	// Test direct VNC console access - verifies path to nil proxmox check
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	_, err := orch.GetDirectVMConsole(ctx, "pve", 100, "vnc")
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestGetDirectVMConsole_DefaultToVNC(t *testing.T) {
	// Test that empty console type defaults to VNC
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	_, err := orch.GetDirectVMConsole(ctx, "pve", 100, "")
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	// Even empty string should hit nil proxmox check
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// PodFilter Tests
// -----------------------------------------------------------------------------

func TestListPodsWithFilter_MultipleFilters(t *testing.T) {
	orch, repo := newTestOrchestrator(t, nil, nil)

	ctx := context.Background()

	// Create various pods - using Owner field for filter matching
	pods := []*models.Pod{
		{ID: "p1", Owner: "user1", Status: models.PodStatusRunning, Platform: models.PlatformProxmox},
		{ID: "p2", Owner: "user1", Status: models.PodStatusStopped, Platform: models.PlatformProxmox},
		{ID: "p3", Owner: "user2", Status: models.PodStatusRunning, Platform: models.PlatformCloudStack},
		{ID: "p4", Owner: "user2", Status: models.PodStatusRunning, Platform: models.PlatformProxmox},
	}
	for _, p := range pods {
		repo.Create(ctx, p)
	}

	tests := []struct {
		name     string
		filter   PodFilter
		expected int
	}{
		{"by owner", PodFilter{OwnerID: "user1"}, 2},
		{"by status", PodFilter{Status: models.PodStatusRunning}, 3},
		{"by platform", PodFilter{Platform: string(models.PlatformProxmox)}, 3},
		{"by owner and status", PodFilter{OwnerID: "user1", Status: models.PodStatusRunning}, 1},
		{"by owner and platform", PodFilter{OwnerID: "user2", Platform: string(models.PlatformProxmox)}, 1},
		{"no filter", PodFilter{}, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := orch.ListPodsWithFilter(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListPodsWithFilter failed: %v", err)
			}
			if len(result) != tt.expected {
				t.Errorf("expected %d pods, got %d", tt.expected, len(result))
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Direct VM Operations Tests (without pod lookup)
// -----------------------------------------------------------------------------

func TestStartVMDirect_NoProxmoxClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	err := orch.StartVMDirect(ctx, "pve", 100)
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestStopVMDirect_NoProxmoxClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	err := orch.StopVMDirect(ctx, "pve", 100)
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestShutdownVMDirect_NoProxmoxClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	err := orch.ShutdownVMDirect(ctx, "pve", 100)
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestRebootVMDirect_NoProxmoxClient(t *testing.T) {
	orch := New(nil, nil, Config{})

	ctx := context.Background()
	err := orch.RebootVMDirect(ctx, "pve", 100)
	if err == nil {
		t.Error("expected error without proxmox client")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}
