package wazuh

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewHealthMonitor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultHealthMonitorConfig()

	monitor := NewHealthMonitor(config, logger, nil)

	if monitor == nil {
		t.Fatal("NewHealthMonitor returned nil")
	}

	if monitor.agents == nil {
		t.Error("agents map should be initialized")
	}

	if monitor.config.CheckInterval != 30*time.Second {
		t.Errorf("expected check interval 30s, got %v", monitor.config.CheckInterval)
	}
}

func TestHealthMonitor_RegisterUnregisterAgent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	// Register agent
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")

	state, err := monitor.GetAgentHealth("agent-001")
	if err != nil {
		t.Fatalf("GetAgentHealth error: %v", err)
	}

	if state.AgentID != "agent-001" {
		t.Errorf("expected agent ID 'agent-001', got %s", state.AgentID)
	}

	if state.PodID != "pod-123" {
		t.Errorf("expected pod ID 'pod-123', got %s", state.PodID)
	}

	if state.VMName != "workstation" {
		t.Errorf("expected VM name 'workstation', got %s", state.VMName)
	}

	if state.Status != AgentStatusPending {
		t.Errorf("expected status 'pending', got %s", state.Status)
	}

	// Unregister agent
	monitor.UnregisterAgent("agent-001")

	_, err = monitor.GetAgentHealth("agent-001")
	if err == nil {
		t.Error("expected error after unregistering agent")
	}
}

func TestHealthMonitor_GetAgentHealth_NotFound_WrapsErrNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	_, err := monitor.GetAgentHealth("nonexistent-agent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected error to wrap ErrNotFound, got: %v", err)
	}
}

func TestHealthMonitor_RecordEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	// Register and record event
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")
	monitor.RecordEvent("agent-001")

	state, _ := monitor.GetAgentHealth("agent-001")

	if state.Status != AgentStatusActive {
		t.Errorf("expected status 'active' after event, got %s", state.Status)
	}

	if state.LastEvent.IsZero() {
		t.Error("LastEvent should be set after RecordEvent")
	}

	if state.LastSeen.IsZero() {
		t.Error("LastSeen should be set after RecordEvent")
	}

	if state.ConsecutiveMisses != 0 {
		t.Errorf("expected 0 consecutive misses, got %d", state.ConsecutiveMisses)
	}
}

func TestHealthMonitor_RecordEventReconnect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	var mu sync.Mutex
	var callbackCalled bool
	var callbackState *AgentHealthState

	monitor.RegisterCallback(func(ctx context.Context, state *AgentHealthState) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackState = state
	})

	// Register agent and simulate disconnect then reconnect
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")

	// Set agent to disconnected state manually for testing
	monitor.agentsMu.Lock()
	monitor.agents["agent-001"].Status = AgentStatusDisconnected
	now := time.Now()
	monitor.agents["agent-001"].DisconnectedAt = &now
	monitor.agents["agent-001"].DisconnectCount = 1
	monitor.agentsMu.Unlock()

	// Record event (should trigger reconnect)
	monitor.RecordEvent("agent-001")

	// Wait for callback goroutine to execute
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return callbackCalled
	}, 1*time.Second, 10*time.Millisecond, "callback should have been called on reconnect")

	mu.Lock()
	cbState := callbackState
	mu.Unlock()

	require.NotNil(t, cbState, "callback state should not be nil")
	require.Equal(t, AgentStatusActive, cbState.Status, "expected status 'active' in callback")

	state, _ := monitor.GetAgentHealth("agent-001")
	if state.ReconnectedAt == nil {
		t.Error("ReconnectedAt should be set after reconnect")
	}
}

func TestHealthMonitor_GetPodAgentsHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	// Register multiple agents
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")
	monitor.RegisterAgent("agent-002", "pod-123", "server")
	monitor.RegisterAgent("agent-003", "pod-456", "client")

	podAgents := monitor.GetPodAgentsHealth("pod-123")

	if len(podAgents) != 2 {
		t.Errorf("expected 2 agents for pod-123, got %d", len(podAgents))
	}

	// Check that both expected agents are present
	foundWorkstation := false
	foundServer := false
	for _, state := range podAgents {
		if state.VMName == "workstation" {
			foundWorkstation = true
		}
		if state.VMName == "server" {
			foundServer = true
		}
	}

	if !foundWorkstation || !foundServer {
		t.Error("expected both workstation and server agents")
	}
}

func TestHealthMonitor_GetAllAgentsHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	// Register multiple agents
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")
	monitor.RegisterAgent("agent-002", "pod-456", "server")

	allAgents := monitor.GetAllAgentsHealth()

	if len(allAgents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(allAgents))
	}
}

func TestHealthMonitor_GetDisconnectedAgents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	// Register agents
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")
	monitor.RegisterAgent("agent-002", "pod-123", "server")
	monitor.RecordEvent("agent-001") // Make one active

	// Set one to disconnected
	monitor.agentsMu.Lock()
	monitor.agents["agent-002"].Status = AgentStatusDisconnected
	monitor.agentsMu.Unlock()

	disconnected := monitor.GetDisconnectedAgents()

	if len(disconnected) != 1 {
		t.Errorf("expected 1 disconnected agent, got %d", len(disconnected))
	}

	if disconnected[0].AgentID != "agent-002" {
		t.Errorf("expected agent-002 to be disconnected")
	}
}

func TestHealthMonitor_GetHealthSummary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	// Register agents in different states
	monitor.RegisterAgent("agent-001", "pod-1", "vm1")
	monitor.RegisterAgent("agent-002", "pod-2", "vm2")
	monitor.RegisterAgent("agent-003", "pod-3", "vm3")
	monitor.RegisterAgent("agent-004", "pod-4", "vm4")

	monitor.RecordEvent("agent-001") // Active

	monitor.agentsMu.Lock()
	monitor.agents["agent-002"].Status = AgentStatusDisconnected
	monitor.agents["agent-003"].Status = AgentStatusNeverConnected
	// agent-004 remains pending
	monitor.agentsMu.Unlock()

	summary := monitor.GetHealthSummary()

	if summary.TotalAgents != 4 {
		t.Errorf("expected 4 total agents, got %d", summary.TotalAgents)
	}

	if summary.ActiveAgents != 1 {
		t.Errorf("expected 1 active agent, got %d", summary.ActiveAgents)
	}

	if summary.DisconnectedAgents != 1 {
		t.Errorf("expected 1 disconnected agent, got %d", summary.DisconnectedAgents)
	}

	if summary.PendingAgents != 1 {
		t.Errorf("expected 1 pending agent, got %d", summary.PendingAgents)
	}

	if summary.NeverConnected != 1 {
		t.Errorf("expected 1 never connected agent, got %d", summary.NeverConnected)
	}
}

func TestHealthMonitor_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := HealthMonitorConfig{
		CheckInterval:        10 * time.Millisecond,
		DisconnectThreshold:  50 * time.Millisecond,
		MaxConsecutiveMisses: 2,
	}
	monitor := NewHealthMonitor(config, logger, nil)

	ctx := context.Background()
	monitor.Start(ctx)

	// Register an active agent
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")
	monitor.RecordEvent("agent-001")

	// Wait for health check loop to detect disconnection
	require.Eventually(t, func() bool {
		state, _ := monitor.GetAgentHealth("agent-001")
		return state.Status == AgentStatusDisconnected
	}, 1*time.Second, 10*time.Millisecond, "agent should be disconnected after threshold")

	state, _ := monitor.GetAgentHealth("agent-001")
	require.Equal(t, 1, state.DisconnectCount, "expected disconnect count 1")

	monitor.Stop()
}

func TestHealthMonitor_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	monitor := NewHealthMonitor(DefaultHealthMonitorConfig(), logger, nil)

	var wg sync.WaitGroup
	numAgents := 50
	eventsPerAgent := 100

	// Register agents concurrently
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := string(rune('a' + id%26))
			monitor.RegisterAgent(agentID, "pod-1", "vm")
		}(i)
	}
	wg.Wait()

	// Record events concurrently
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := string(rune('a' + id%26))
			for j := 0; j < eventsPerAgent; j++ {
				monitor.RecordEvent(agentID)
			}
		}(i)
	}
	wg.Wait()

	// Query health concurrently
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			monitor.GetAllAgentsHealth()
			monitor.GetHealthSummary()
		}()
	}
	wg.Wait()
}

func TestHealthMonitor_Callback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := HealthMonitorConfig{
		CheckInterval:        10 * time.Millisecond,
		DisconnectThreshold:  20 * time.Millisecond,
		MaxConsecutiveMisses: 1,
	}
	monitor := NewHealthMonitor(config, logger, nil)

	var mu sync.Mutex
	var callbackStates []*AgentHealthState

	monitor.RegisterCallback(func(ctx context.Context, state *AgentHealthState) {
		mu.Lock()
		defer mu.Unlock()
		callbackStates = append(callbackStates, state)
	})

	ctx := context.Background()
	monitor.Start(ctx)

	// Register agent and make it active
	monitor.RegisterAgent("agent-001", "pod-123", "workstation")
	monitor.RecordEvent("agent-001")

	// Wait for at least 2 callbacks: active, then disconnected
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(callbackStates) >= 2
	}, 1*time.Second, 10*time.Millisecond, "expected at least 2 callbacks")

	monitor.Stop()

	mu.Lock()
	defer mu.Unlock()

	// First callback should be for active status
	require.Equal(t, AgentStatusActive, callbackStates[0].Status, "first callback expected 'active'")

	// Second callback should be for disconnected status
	require.Equal(t, AgentStatusDisconnected, callbackStates[1].Status, "second callback expected 'disconnected'")
}

func TestAgentStatus_Values(t *testing.T) {
	tests := []struct {
		status AgentStatus
		want   string
	}{
		{AgentStatusActive, "active"},
		{AgentStatusDisconnected, "disconnected"},
		{AgentStatusNeverConnected, "never_connected"},
		{AgentStatusPending, "pending"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("expected status %q, got %q", tt.want, string(tt.status))
		}
	}
}
