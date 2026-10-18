package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// asyncTestEventPublisher implements EventPublisher for async provisioning tests
type asyncTestEventPublisher struct {
	mu        sync.Mutex
	events    [][]byte
	subjects  []string
	shouldErr bool
}

func newAsyncTestEventPublisher() *asyncTestEventPublisher {
	return &asyncTestEventPublisher{
		events:   make([][]byte, 0),
		subjects: make([]string, 0),
	}
}

func (m *asyncTestEventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	if m.shouldErr {
		return context.DeadlineExceeded
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, data)
	m.subjects = append(m.subjects, subject)
	return nil
}

func (m *asyncTestEventPublisher) eventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func (m *asyncTestEventPublisher) getEvent(idx int) (*PodProvisioningEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx >= len(m.events) {
		return nil, nil
	}
	var event PodProvisioningEvent
	if err := json.Unmarshal(m.events[idx], &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// asyncTestBroadcaster implements ProvisioningBroadcaster for testing
type asyncTestBroadcaster struct {
	mu     sync.Mutex
	events []PodProvisioningEvent
}

func newAsyncTestBroadcaster() *asyncTestBroadcaster {
	return &asyncTestBroadcaster{
		events: make([]PodProvisioningEvent, 0),
	}
}

func (m *asyncTestBroadcaster) BroadcastProvisioningEvent(event PodProvisioningEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *asyncTestBroadcaster) eventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func asyncTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// -----------------------------------------------------------------------------
// NewAsyncProvisioner Tests
// -----------------------------------------------------------------------------

func TestNewAsyncProvisioner_DefaultSubjectPrefix(t *testing.T) {
	orch := New(nil, nil, Config{})
	publisher := newAsyncTestEventPublisher()
	logger := asyncTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger)

	if ap.subjectPrefix != "labs.pods" {
		t.Errorf("expected default subject prefix 'labs.pods', got %q", ap.subjectPrefix)
	}
}

func TestNewAsyncProvisioner_WithSubjectPrefix(t *testing.T) {
	orch := New(nil, nil, Config{})
	publisher := newAsyncTestEventPublisher()
	logger := asyncTestLogger()

	ap := NewAsyncProvisioner(orch, publisher, logger, WithSubjectPrefix("custom.prefix"))

	if ap.subjectPrefix != "custom.prefix" {
		t.Errorf("expected subject prefix 'custom.prefix', got %q", ap.subjectPrefix)
	}
}

func TestNewAsyncProvisioner_WithBroadcaster(t *testing.T) {
	orch := New(nil, nil, Config{})
	publisher := newAsyncTestEventPublisher()
	logger := asyncTestLogger()
	broadcaster := newAsyncTestBroadcaster()

	ap := NewAsyncProvisioner(orch, publisher, logger, WithBroadcaster(broadcaster))

	if ap.broadcaster != broadcaster {
		t.Error("expected broadcaster to be set")
	}
}

// -----------------------------------------------------------------------------
// ProvisioningPhase Tests
// -----------------------------------------------------------------------------

func TestProvisioningPhase_Constants(t *testing.T) {
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
			t.Error("provisioning phase should not be empty")
		}
	}

	// Check specific values
	if PhaseQueued != "queued" {
		t.Errorf("expected PhaseQueued 'queued', got %q", PhaseQueued)
	}
	if PhaseCompleted != "completed" {
		t.Errorf("expected PhaseCompleted 'completed', got %q", PhaseCompleted)
	}
	if PhaseFailed != "failed" {
		t.Errorf("expected PhaseFailed 'failed', got %q", PhaseFailed)
	}
}

// -----------------------------------------------------------------------------
// PodProvisioningEvent Tests
// -----------------------------------------------------------------------------

func TestPodProvisioningEvent_JSONSerialization(t *testing.T) {
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseStarting,
		Message:   "Starting provisioning",
		Progress:  25,
		VMName:    "vm-web-01",
		VMStatus:  "cloning",
		Timestamp: time.Now(),
		RequestID: "req-789",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded PodProvisioningEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.PodID != event.PodID {
		t.Errorf("expected PodID %q, got %q", event.PodID, decoded.PodID)
	}
	if decoded.Phase != event.Phase {
		t.Errorf("expected Phase %q, got %q", event.Phase, decoded.Phase)
	}
	if decoded.Progress != event.Progress {
		t.Errorf("expected Progress %d, got %d", event.Progress, decoded.Progress)
	}
}

func TestPodProvisioningEvent_WithError(t *testing.T) {
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusError,
		Phase:     PhaseFailed,
		Message:   "Provisioning failed",
		Progress:  0,
		Error:     "Proxmox connection timeout",
		Timestamp: time.Now(),
		RequestID: "req-789",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded PodProvisioningEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.Error != event.Error {
		t.Errorf("expected Error %q, got %q", event.Error, decoded.Error)
	}
}

// -----------------------------------------------------------------------------
// PodProvisionRequest Tests
// -----------------------------------------------------------------------------

func TestPodProvisionRequest_JSONSerialization(t *testing.T) {
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        "Test Template",
			Description: "A test template",
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
		},
	}

	req := PodProvisionRequest{
		RequestID:    "req-123",
		TemplateID:   "template-123",
		TemplateName: "Test Template",
		Template:     template,
		OwnerID:      "user-456",
		OwnerName:    "testuser",
		Timestamp:    time.Now(),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded PodProvisionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.RequestID != req.RequestID {
		t.Errorf("expected RequestID %q, got %q", req.RequestID, decoded.RequestID)
	}
	if decoded.TemplateID != req.TemplateID {
		t.Errorf("expected TemplateID %q, got %q", req.TemplateID, decoded.TemplateID)
	}
}

// -----------------------------------------------------------------------------
// handleProvisioningError Tests
// -----------------------------------------------------------------------------

func TestHandleProvisioningError(t *testing.T) {
	publisher := newAsyncTestEventPublisher()
	broadcaster := newAsyncTestBroadcaster()
	logger := asyncTestLogger()
	orch := New(nil, nil, Config{})

	ap := NewAsyncProvisioner(orch, publisher, logger, WithBroadcaster(broadcaster))

	ctx := context.Background()
	podID := "pod-error-test"
	req := PodProvisionRequest{
		RequestID: "req-456",
		OwnerID:   "user-123",
	}
	testErr := errors.New("test error message")

	// Add a pod to the orchestrator
	orch.podsMu.Lock()
	orch.pods[podID] = &models.Pod{
		ID:     podID,
		Status: models.PodStatusProvisioning,
	}
	orch.podsMu.Unlock()

	ap.handleProvisioningError(ctx, podID, req, testErr, "provisioning failed")

	// Wait for async event publishing
	require.Eventually(t, func() bool { return publisher.eventCount() >= 1 }, 1*time.Second, 10*time.Millisecond,
		"expected at least one error event to be published")

	require.Eventually(t, func() bool { return broadcaster.eventCount() >= 1 }, 1*time.Second, 10*time.Millisecond,
		"expected at least one event to be broadcast")

	// Check pod status was updated to error
	orch.podsMu.RLock()
	pod := orch.pods[podID]
	orch.podsMu.RUnlock()

	if pod.Status != models.PodStatusError {
		t.Errorf("expected pod status %q, got %q", models.PodStatusError, pod.Status)
	}
}

func TestDeployCloudStackVM_ReturnsErrNotImplemented(t *testing.T) {
	publisher := newAsyncTestEventPublisher()
	logger := asyncTestLogger()
	orch := New(nil, nil, Config{})

	ap := NewAsyncProvisioner(orch, publisher, logger)

	pod := &models.Pod{ID: "pod-1", Name: "test-pod"}
	vmSpec := models.VMSpec{Name: "vm1", Template: "ubuntu-22.04"}

	_, err := ap.deployCloudStackVM(context.Background(), pod, vmSpec, nil, nil)
	if err == nil {
		t.Fatal("expected error from deployCloudStackVM")
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// ProvisioningEventSubject Tests
// -----------------------------------------------------------------------------

func TestProvisioningEventSubject(t *testing.T) {
	tests := []struct {
		prefix   string
		podID    string
		expected string
	}{
		{"labs.pods", "pod-123", "labs.pods.pod-123.provision"},
		{"custom.prefix", "pod-abc", "custom.prefix.pod-abc.provision"},
		{"", "pod-xyz", ".pod-xyz.provision"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+"/"+tt.podID, func(t *testing.T) {
			publisher := newAsyncTestEventPublisher()
			logger := asyncTestLogger()
			orch := New(nil, nil, Config{})

			ap := NewAsyncProvisioner(orch, publisher, logger, WithSubjectPrefix(tt.prefix))

			result := ap.ProvisioningEventSubject(tt.podID)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// publishEvent Tests
// -----------------------------------------------------------------------------

func TestPublishEvent_WithPublisher(t *testing.T) {
	publisher := newAsyncTestEventPublisher()
	logger := asyncTestLogger()
	orch := New(nil, nil, Config{})

	ap := NewAsyncProvisioner(orch, publisher, logger)

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseStarting,
		Message:   "Test event",
		Timestamp: time.Now(),
	}

	ap.publishEvent(ctx, event)

	// Wait for async publish
	require.Eventually(t, func() bool { return publisher.eventCount() == 1 }, 1*time.Second, 10*time.Millisecond,
		"expected 1 event to be published")

	published, err := publisher.getEvent(0)
	if err != nil {
		t.Fatalf("failed to get published event: %v", err)
	}

	if published.PodID != event.PodID {
		t.Errorf("expected PodID %q, got %q", event.PodID, published.PodID)
	}
}

func TestPublishEvent_WithBroadcaster(t *testing.T) {
	publisher := newAsyncTestEventPublisher()
	broadcaster := newAsyncTestBroadcaster()
	logger := asyncTestLogger()
	orch := New(nil, nil, Config{})

	ap := NewAsyncProvisioner(orch, publisher, logger, WithBroadcaster(broadcaster))

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseStarting,
		Message:   "Test event",
		Timestamp: time.Now(),
	}

	ap.publishEvent(ctx, event)

	// Wait for async broadcast
	require.Eventually(t, func() bool { return broadcaster.eventCount() == 1 }, 1*time.Second, 10*time.Millisecond,
		"expected 1 broadcast event")
}

func TestPublishEvent_WithNilPublisher(t *testing.T) {
	logger := asyncTestLogger()
	orch := New(nil, nil, Config{})

	// Create provisioner with nil publisher
	ap := NewAsyncProvisioner(orch, nil, logger)

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseStarting,
		Message:   "Test event",
		Timestamp: time.Now(),
	}

	// Should not panic
	ap.publishEvent(ctx, event)
}

func TestPublishEvent_PublisherError(t *testing.T) {
	publisher := newAsyncTestEventPublisher()
	publisher.shouldErr = true
	logger := asyncTestLogger()
	orch := New(nil, nil, Config{})

	ap := NewAsyncProvisioner(orch, publisher, logger)

	ctx := context.Background()
	event := PodProvisioningEvent{
		PodID:     "pod-123",
		OwnerID:   "user-456",
		Status:    models.PodStatusProvisioning,
		Phase:     PhaseStarting,
		Message:   "Test event",
		Timestamp: time.Now(),
	}

	// Should not panic even when publisher returns error
	ap.publishEvent(ctx, event)
}
