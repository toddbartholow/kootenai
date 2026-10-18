package checkpoint

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/wazuh"
)

func TestDefaultTamperingConfig(t *testing.T) {
	config := DefaultTamperingConfig()

	if !config.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if config.PenaltyPerIncident != 10 {
		t.Errorf("expected PenaltyPerIncident=10, got %d", config.PenaltyPerIncident)
	}
	if config.MaxPenalty != 50 {
		t.Errorf("expected MaxPenalty=50, got %d", config.MaxPenalty)
	}
	if config.OfflineThreshold != 2*time.Minute {
		t.Errorf("expected OfflineThreshold=2m, got %v", config.OfflineThreshold)
	}
	if config.GracePeriodOnStart != 5*time.Minute {
		t.Errorf("expected GracePeriodOnStart=5m, got %v", config.GracePeriodOnStart)
	}
	if config.RestartThreshold != 3 {
		t.Errorf("expected RestartThreshold=3, got %d", config.RestartThreshold)
	}
	if !config.AutoFlagForReview {
		t.Error("expected AutoFlagForReview to be true by default")
	}
}

func TestNewTamperingDetector(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()

	// Without health monitor
	td := NewTamperingDetector(config, logger, nil)
	if td == nil {
		t.Fatal("expected non-nil TamperingDetector")
	}
	if td.sessions == nil {
		t.Error("expected sessions map to be initialized")
	}

	// With health monitor
	hmConfig := wazuh.DefaultHealthMonitorConfig()
	hm := wazuh.NewHealthMonitor(hmConfig, logger, nil)
	td2 := NewTamperingDetector(config, logger, hm)
	if td2 == nil {
		t.Fatal("expected non-nil TamperingDetector with health monitor")
	}
}

func TestTamperingDetector_StartEndSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	sessionID := "test-session-1"
	podID := "test-pod-1"
	agentIDs := []string{"agent-1", "agent-2"}

	// Start session
	td.StartSession(sessionID, podID, agentIDs)

	// Verify session exists
	state, exists := td.GetSessionState(sessionID)
	if !exists {
		t.Fatal("expected session to exist after StartSession")
	}
	if state.SessionID != sessionID {
		t.Errorf("expected SessionID=%s, got %s", sessionID, state.SessionID)
	}
	if state.PodID != podID {
		t.Errorf("expected PodID=%s, got %s", podID, state.PodID)
	}
	if len(state.Incidents) != 0 {
		t.Errorf("expected 0 incidents initially, got %d", len(state.Incidents))
	}

	// End session
	finalState := td.EndSession(sessionID)
	if finalState == nil {
		t.Fatal("expected non-nil state from EndSession")
	}

	// Verify session no longer exists
	_, exists = td.GetSessionState(sessionID)
	if exists {
		t.Error("expected session to not exist after EndSession")
	}
}

func TestTamperingDetector_GetPenalty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	sessionID := "test-session-penalty"

	// No session - should return 0
	penalty := td.GetPenalty("nonexistent")
	if penalty != 0 {
		t.Errorf("expected 0 penalty for nonexistent session, got %d", penalty)
	}

	// Start session with no incidents
	td.StartSession(sessionID, "pod-1", nil)
	penalty = td.GetPenalty(sessionID)
	if penalty != 0 {
		t.Errorf("expected 0 penalty for new session, got %d", penalty)
	}
}

func TestTamperingDetector_RecordVerificationMismatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	sessionID := "test-session-mismatch"
	td.StartSession(sessionID, "pod-1", nil)

	// Record a verification mismatch
	td.RecordVerificationMismatch(sessionID, "checkpoint-1", true, false)

	// Wait briefly for async operations
	time.Sleep(10 * time.Millisecond)

	state, exists := td.GetSessionState(sessionID)
	if !exists {
		t.Fatal("expected session to exist")
	}

	if len(state.Incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(state.Incidents))
	}

	incident := state.Incidents[0]
	if incident.Type != "verification_mismatch" {
		t.Errorf("expected Type=verification_mismatch, got %s", incident.Type)
	}
	if incident.Severity != "critical" {
		t.Errorf("expected Severity=critical, got %s", incident.Severity)
	}
	if !state.FlaggedForReview {
		t.Error("expected session to be flagged for review after mismatch")
	}
	if state.TotalPenalty != config.PenaltyPerIncident {
		t.Errorf("expected TotalPenalty=%d, got %d", config.PenaltyPerIncident, state.TotalPenalty)
	}
}

func TestTamperingDetector_MaxPenaltyCap(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := TamperingConfig{
		Enabled:            true,
		PenaltyPerIncident: 20,
		MaxPenalty:         50,
	}
	td := NewTamperingDetector(config, logger, nil)

	sessionID := "test-session-cap"
	td.StartSession(sessionID, "pod-1", nil)

	// Record multiple mismatches
	for i := 0; i < 5; i++ {
		td.RecordVerificationMismatch(sessionID, "checkpoint-"+string(rune('1'+i)), true, false)
	}

	time.Sleep(10 * time.Millisecond)

	state, _ := td.GetSessionState(sessionID)
	if state.TotalPenalty > config.MaxPenalty {
		t.Errorf("penalty %d exceeds max %d", state.TotalPenalty, config.MaxPenalty)
	}
	if state.TotalPenalty != config.MaxPenalty {
		t.Errorf("expected penalty to reach max %d, got %d", config.MaxPenalty, state.TotalPenalty)
	}
}

func TestTamperingDetector_ApplyPenaltyToScore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	tests := []struct {
		name           string
		penalty        int
		earnedPoints   int
		maxPoints      int
		expectedEarned int
	}{
		{
			name:           "no penalty",
			penalty:        0,
			earnedPoints:   80,
			maxPoints:      100,
			expectedEarned: 80,
		},
		{
			name:           "10% penalty",
			penalty:        10,
			earnedPoints:   80,
			maxPoints:      100,
			expectedEarned: 70, // 80 - (100 * 10 / 100)
		},
		{
			name:           "50% penalty",
			penalty:        50,
			earnedPoints:   80,
			maxPoints:      100,
			expectedEarned: 30, // 80 - 50
		},
		{
			name:           "penalty exceeds earned",
			penalty:        50,
			earnedPoints:   30,
			maxPoints:      100,
			expectedEarned: 0, // min 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := "test-" + tt.name
			td.StartSession(sessionID, "pod-1", nil)

			// Manually set penalty via mismatches
			td.sessionsMu.Lock()
			if state, exists := td.sessions[sessionID]; exists {
				state.TotalPenalty = tt.penalty
			}
			td.sessionsMu.Unlock()

			adjusted, max := td.ApplyPenaltyToScore(sessionID, tt.earnedPoints, tt.maxPoints)
			if adjusted != tt.expectedEarned {
				t.Errorf("expected adjusted score %d, got %d", tt.expectedEarned, adjusted)
			}
			if max != tt.maxPoints {
				t.Errorf("expected max %d, got %d", tt.maxPoints, max)
			}

			td.EndSession(sessionID)
		})
	}
}

func TestTamperingDetector_AcknowledgeIncident(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	sessionID := "test-session-ack"
	td.StartSession(sessionID, "pod-1", nil)

	// Record a mismatch
	td.RecordVerificationMismatch(sessionID, "cp-1", true, false)
	time.Sleep(10 * time.Millisecond)

	state, _ := td.GetSessionState(sessionID)
	incidentID := state.Incidents[0].ID

	// Acknowledge it
	err := td.AcknowledgeIncident(sessionID, incidentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify acknowledged
	state, _ = td.GetSessionState(sessionID)
	if !state.Incidents[0].Acknowledged {
		t.Error("expected incident to be acknowledged")
	}

	// Try to acknowledge nonexistent
	err = td.AcknowledgeIncident(sessionID, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent incident")
	}

	err = td.AcknowledgeIncident("nonexistent-session", "any")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestTamperingDetector_GetFlaggedSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	// Start multiple sessions
	td.StartSession("session-1", "pod-1", nil)
	td.StartSession("session-2", "pod-2", nil)
	td.StartSession("session-3", "pod-3", nil)

	// Flag some sessions by recording mismatches
	td.RecordVerificationMismatch("session-1", "cp-1", true, false)
	td.RecordVerificationMismatch("session-3", "cp-1", true, false)

	time.Sleep(10 * time.Millisecond)

	flagged := td.GetFlaggedSessions()
	if len(flagged) != 2 {
		t.Errorf("expected 2 flagged sessions, got %d", len(flagged))
	}

	// Verify correct sessions are flagged
	flaggedIDs := make(map[string]bool)
	for _, s := range flagged {
		flaggedIDs[s.SessionID] = true
	}
	if !flaggedIDs["session-1"] || !flaggedIDs["session-3"] {
		t.Error("expected session-1 and session-3 to be flagged")
	}
	if flaggedIDs["session-2"] {
		t.Error("did not expect session-2 to be flagged")
	}
}

func TestTamperingDetector_RegisterCallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	var receivedIncident *TamperingIncident
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	td.RegisterCallback(func(ctx context.Context, incident *TamperingIncident) {
		mu.Lock()
		receivedIncident = incident
		mu.Unlock()
		wg.Done()
	})

	sessionID := "test-callback"
	td.StartSession(sessionID, "pod-1", nil)

	// Trigger an incident
	td.RecordVerificationMismatch(sessionID, "cp-1", true, false)

	// Wait for callback with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback")
	}

	mu.Lock()
	if receivedIncident == nil {
		t.Fatal("expected callback to receive incident")
	}
	if receivedIncident.SessionID != sessionID {
		t.Errorf("expected SessionID=%s, got %s", sessionID, receivedIncident.SessionID)
	}
	mu.Unlock()
}

func TestTamperingDetector_DisabledConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := TamperingConfig{
		Enabled: false,
	}

	hmConfig := wazuh.DefaultHealthMonitorConfig()
	hm := wazuh.NewHealthMonitor(hmConfig, logger, nil)
	td := NewTamperingDetector(config, logger, hm)

	sessionID := "test-disabled"
	td.StartSession(sessionID, "pod-1", []string{"agent-1"})

	// Simulate agent health change - should be ignored when disabled
	ctx := context.Background()
	healthState := &wazuh.AgentHealthState{
		AgentID: "agent-1",
		PodID:   "pod-1",
		Status:  wazuh.AgentStatusDisconnected,
	}

	// Directly call the handler (normally called by health monitor)
	td.onAgentHealthChange(ctx, healthState)

	state, _ := td.GetSessionState(sessionID)
	if len(state.Incidents) != 0 {
		t.Error("expected no incidents when detector is disabled")
	}
}

func TestTamperingDetector_GracePeriod(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := TamperingConfig{
		Enabled:            true,
		PenaltyPerIncident: 10,
		MaxPenalty:         50,
		GracePeriodOnStart: time.Hour, // Legacy: kept for backward compat
		GracePeriodTimeout: time.Hour, // Long grace timeout for test
	}

	td := NewTamperingDetector(config, logger, nil)

	sessionID := "test-grace"
	td.StartSession(sessionID, "pod-1", []string{"agent-1"})

	// Simulate agent going offline during grace period
	ctx := context.Background()
	healthState := &wazuh.AgentHealthState{
		AgentID: "agent-1",
		PodID:   "pod-1",
		Status:  wazuh.AgentStatusDisconnected,
	}

	td.onAgentHealthChange(ctx, healthState)

	state, _ := td.GetSessionState(sessionID)
	if len(state.Incidents) != 0 {
		t.Error("expected no incidents during grace period")
	}
}

func TestTamperingDetector_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultTamperingConfig()
	td := NewTamperingDetector(config, logger, nil)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent session operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sessionID := "session-" + string(rune('A'+n))

			td.StartSession(sessionID, "pod-"+string(rune('A'+n)), nil)
			td.GetSessionState(sessionID)
			td.GetPenalty(sessionID)
			td.RecordVerificationMismatch(sessionID, "cp-1", true, false)
			td.GetFlaggedSessions()
			td.EndSession(sessionID)
		}(i)
	}

	wg.Wait()
	// If we get here without data races, the test passes
}

func TestTamperingIncident_Fields(t *testing.T) {
	incident := TamperingIncident{
		ID:             "inc-1",
		SessionID:      "session-1",
		PodID:          "pod-1",
		AgentID:        "agent-1",
		Type:           "agent_offline",
		Severity:       "critical",
		Description:    "Test incident",
		DetectedAt:     time.Now(),
		Duration:       5 * time.Minute,
		PenaltyApplied: 10,
		Acknowledged:   false,
	}

	if incident.ID != "inc-1" {
		t.Error("ID field mismatch")
	}
	if incident.SessionID != "session-1" {
		t.Error("SessionID field mismatch")
	}
	if incident.Type != "agent_offline" {
		t.Error("Type field mismatch")
	}
	if incident.Severity != "critical" {
		t.Error("Severity field mismatch")
	}
	if incident.PenaltyApplied != 10 {
		t.Error("PenaltyApplied field mismatch")
	}
}

func TestSessionTamperingState_Fields(t *testing.T) {
	state := SessionTamperingState{
		SessionID:        "session-1",
		PodID:            "pod-1",
		StartedAt:        time.Now(),
		Incidents:        []TamperingIncident{},
		TotalPenalty:     20,
		FlaggedForReview: true,
		AgentRestarts:    2,
		LastAgentSeen:    time.Now(),
	}

	if state.SessionID != "session-1" {
		t.Error("SessionID field mismatch")
	}
	if state.TotalPenalty != 20 {
		t.Error("TotalPenalty field mismatch")
	}
	if !state.FlaggedForReview {
		t.Error("FlaggedForReview field mismatch")
	}
	if state.AgentRestarts != 2 {
		t.Error("AgentRestarts field mismatch")
	}
}
