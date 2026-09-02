// Package checkpoint provides checkpoint evaluation and verification
package checkpoint

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/wazuh"
)

// TamperingIncident represents a detected tampering attempt
type TamperingIncident struct {
	ID             string        `json:"id"`
	SessionID      string        `json:"sessionId"`
	PodID          string        `json:"podId"`
	AgentID        string        `json:"agentId"`
	Type           string        `json:"type"`     // "agent_offline", "agent_restart", "config_change", "verification_mismatch"
	Severity       string        `json:"severity"` // "warning", "critical"
	Description    string        `json:"description"`
	DetectedAt     time.Time     `json:"detectedAt"`
	Duration       time.Duration `json:"duration,omitempty"` // For offline incidents
	PenaltyApplied int           `json:"penaltyApplied"`
	Acknowledged   bool          `json:"acknowledged"`
}

// TamperingConfig configures the tampering detector
type TamperingConfig struct {
	// Enabled controls whether tampering detection is active
	Enabled bool `yaml:"enabled"`

	// PenaltyPerIncident is the points deducted per tampering incident
	PenaltyPerIncident int `yaml:"penalty_per_incident"`

	// MaxPenalty is the maximum total penalty that can be applied
	MaxPenalty int `yaml:"max_penalty"`

	// OfflineThreshold is how long an agent can be offline before it's considered tampering
	OfflineThreshold time.Duration `yaml:"offline_threshold"`

	// GracePeriodOnStart allows time for agent to initialize after pod starts
	GracePeriodOnStart time.Duration `yaml:"grace_period_on_start"`

	// RestartThreshold is the number of restarts that triggers a tampering alert
	RestartThreshold int `yaml:"restart_threshold"`

	// AutoFlagForReview automatically flags sessions with tampering for instructor review
	AutoFlagForReview bool `yaml:"auto_flag_for_review"`

	// GracePeriodTimeout is the maximum grace period if no events arrive (default 2 minutes).
	// Grace period ends on the first Wazuh event OR when this timeout expires.
	GracePeriodTimeout time.Duration `yaml:"grace_period_timeout"`
}

// DefaultTamperingConfig returns sensible defaults
func DefaultTamperingConfig() TamperingConfig {
	return TamperingConfig{
		Enabled:            true,
		PenaltyPerIncident: 10, // 10% penalty per incident
		MaxPenalty:         50, // Maximum 50% penalty
		OfflineThreshold:   2 * time.Minute,
		GracePeriodOnStart: 5 * time.Minute, // Legacy: kept for backward compat
		GracePeriodTimeout: 2 * time.Minute, // Event-based grace: max wait for first event
		RestartThreshold:   3,
		AutoFlagForReview:  true,
	}
}

// SessionTamperingState tracks tampering state for a session
type SessionTamperingState struct {
	SessionID            string              `json:"sessionId"`
	PodID                string              `json:"podId"`
	StartedAt            time.Time           `json:"startedAt"`
	Incidents            []TamperingIncident `json:"incidents"`
	TotalPenalty         int                 `json:"totalPenalty"`
	FlaggedForReview     bool                `json:"flaggedForReview"`
	AgentRestarts        int                 `json:"agentRestarts"`
	LastAgentSeen        time.Time           `json:"lastAgentSeen,omitempty"`
	FirstEventReceivedAt *time.Time          `json:"firstEventReceivedAt,omitempty"`
	GracePeriodEnded     bool                `json:"gracePeriodEnded"`
}

// TamperingCallback is called when a tampering incident is detected
type TamperingCallback func(ctx context.Context, incident *TamperingIncident)

// TamperingDetector monitors sessions for tampering attempts
type TamperingDetector struct {
	config        TamperingConfig
	logger        *slog.Logger
	healthMonitor *wazuh.HealthMonitor
	sessions      map[string]*SessionTamperingState
	sessionsMu    sync.RWMutex
	callbacks     []TamperingCallback
	callbacksMu   sync.RWMutex
	// ctx is the detector-lifecycle context; cancelled by Stop so callback
	// goroutines that survive past shutdown signaling get a cancel signal.
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	wg       sync.WaitGroup
	// stopped is set by Stop so concurrent shutdowns don't double-cancel and
	// so new callback goroutines aren't launched after Stop begins.
	stopped atomic.Bool
}

// NewTamperingDetector creates a new tampering detector
func NewTamperingDetector(config TamperingConfig, logger *slog.Logger, healthMonitor *wazuh.HealthMonitor) *TamperingDetector {
	ctx, cancel := context.WithCancel(context.Background())
	td := &TamperingDetector{
		config:        config,
		logger:        logger,
		healthMonitor: healthMonitor,
		sessions:      make(map[string]*SessionTamperingState),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Register for health monitor callbacks
	if healthMonitor != nil {
		healthMonitor.RegisterCallback(td.onAgentHealthChange)
	}

	return td
}

// Stop signals the detector to stop accepting new callback goroutines and
// waits for in-flight callback goroutines to finish (up to timeout).
// Safe to call multiple times. Returns an error if the timeout expires.
func (td *TamperingDetector) Stop(timeout time.Duration) error {
	var stopErr error
	td.stopOnce.Do(func() {
		td.stopped.Store(true)
		td.cancel()

		done := make(chan struct{})
		go func() {
			td.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			td.logger.Info("Tampering detector shutdown complete")
		case <-time.After(timeout):
			td.logger.Warn("Tampering detector shutdown timed out", "timeout", timeout)
			stopErr = fmt.Errorf("tampering detector shutdown timed out after %v", timeout)
		}
	})
	return stopErr
}

// RegisterCallback adds a callback for tampering incidents
func (td *TamperingDetector) RegisterCallback(cb TamperingCallback) {
	td.callbacksMu.Lock()
	defer td.callbacksMu.Unlock()
	td.callbacks = append(td.callbacks, cb)
}

// StartSession begins monitoring a session for tampering
func (td *TamperingDetector) StartSession(sessionID, podID string, agentIDs []string) {
	td.sessionsMu.Lock()
	defer td.sessionsMu.Unlock()

	state := &SessionTamperingState{
		SessionID: sessionID,
		PodID:     podID,
		StartedAt: time.Now(),
		Incidents: make([]TamperingIncident, 0),
	}
	td.sessions[sessionID] = state

	// Register agents with health monitor
	if td.healthMonitor != nil {
		for _, agentID := range agentIDs {
			td.healthMonitor.RegisterAgent(agentID, podID, "")
		}
	}

	td.logger.Info("Started tampering monitoring for session",
		"sessionId", sessionID,
		"podId", podID,
		"agents", len(agentIDs),
	)
}

// StartSessionWithGraceOverride begins monitoring with a custom grace period.
// If graceOverride is 0, the grace period is skipped entirely (agent already known to be dead).
func (td *TamperingDetector) StartSessionWithGraceOverride(sessionID, podID string, graceOverride time.Duration, agentIDs []string) {
	td.sessionsMu.Lock()
	defer td.sessionsMu.Unlock()

	state := &SessionTamperingState{
		SessionID: sessionID,
		PodID:     podID,
		StartedAt: time.Now(),
		Incidents: make([]TamperingIncident, 0),
	}
	if graceOverride == 0 {
		state.GracePeriodEnded = true
	}
	td.sessions[sessionID] = state

	if td.healthMonitor != nil {
		for _, agentID := range agentIDs {
			td.healthMonitor.RegisterAgent(agentID, podID, "")
		}
	}

	td.logger.Info("Started tampering monitoring for session",
		"sessionId", sessionID,
		"podId", podID,
		"agents", len(agentIDs),
		"graceOverride", graceOverride,
	)
}

// OnFirstEvent records that the first Wazuh event was received for a session.
// This ends the grace period, proving the agent is alive.
// The method is idempotent — only the first call has an effect.
func (td *TamperingDetector) OnFirstEvent(sessionID string) {
	td.sessionsMu.Lock()
	defer td.sessionsMu.Unlock()

	state, exists := td.sessions[sessionID]
	if !exists {
		return
	}

	if state.FirstEventReceivedAt != nil {
		return // Already recorded
	}

	now := time.Now()
	state.FirstEventReceivedAt = &now
	state.GracePeriodEnded = true

	td.logger.Info("First Wazuh event received for session — grace period ended",
		"sessionId", sessionID,
		"elapsed", now.Sub(state.StartedAt),
	)
}

// EndSession stops monitoring a session
func (td *TamperingDetector) EndSession(sessionID string) *SessionTamperingState {
	td.sessionsMu.Lock()
	defer td.sessionsMu.Unlock()

	state, exists := td.sessions[sessionID]
	if !exists {
		return nil
	}

	delete(td.sessions, sessionID)

	td.logger.Info("Ended tampering monitoring for session",
		"sessionId", sessionID,
		"incidents", len(state.Incidents),
		"totalPenalty", state.TotalPenalty,
	)

	return state
}

// GetSessionState returns the current tampering state for a session
func (td *TamperingDetector) GetSessionState(sessionID string) (*SessionTamperingState, bool) {
	td.sessionsMu.RLock()
	defer td.sessionsMu.RUnlock()

	state, exists := td.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Return a copy
	stateCopy := *state
	stateCopy.Incidents = make([]TamperingIncident, len(state.Incidents))
	copy(stateCopy.Incidents, state.Incidents)
	return &stateCopy, true
}

// GetPenalty returns the current penalty for a session
func (td *TamperingDetector) GetPenalty(sessionID string) int {
	td.sessionsMu.RLock()
	defer td.sessionsMu.RUnlock()

	if state, exists := td.sessions[sessionID]; exists {
		return state.TotalPenalty
	}
	return 0
}

// onAgentHealthChange handles health monitor callbacks. Mutations to session
// state happen under sessionsMu; callback dispatch happens after the lock is
// released so a slow callback (e.g. NATS publish, DB write) cannot serialise
// health-state processing across every session.
func (td *TamperingDetector) onAgentHealthChange(_ context.Context, healthState *wazuh.AgentHealthState) {
	if !td.config.Enabled {
		return
	}

	td.sessionsMu.Lock()

	var sessionState *SessionTamperingState
	for _, state := range td.sessions {
		if state.PodID == healthState.PodID {
			sessionState = state
			break
		}
	}

	if sessionState == nil {
		td.sessionsMu.Unlock()
		return
	}

	var pendingIncidents []TamperingIncident

	// Event-based grace period: ends on first Wazuh event OR after GracePeriodTimeout
	if !sessionState.GracePeriodEnded {
		switch {
		case sessionState.FirstEventReceivedAt != nil:
			// First event received — grace period is over
			sessionState.GracePeriodEnded = true
			td.logger.Info("Grace period ended (first event received)",
				"sessionId", sessionState.SessionID,
				"elapsed", time.Since(sessionState.StartedAt),
			)
		case time.Since(sessionState.StartedAt) > td.config.GracePeriodTimeout:
			// Timeout expired with no events — flag for review
			sessionState.GracePeriodEnded = true
			sessionState.FlaggedForReview = true
			incident := TamperingIncident{
				ID:          fmt.Sprintf("%s-%d", sessionState.SessionID, len(sessionState.Incidents)+1),
				SessionID:   sessionState.SessionID,
				PodID:       sessionState.PodID,
				Type:        "grace_period_timeout",
				Severity:    "warning",
				Description: "No Wazuh events received during grace period — agent may not be running",
				DetectedAt:  time.Now(),
			}
			sessionState.Incidents = append(sessionState.Incidents, incident)
			pendingIncidents = append(pendingIncidents, incident)
			td.logger.Warn("Grace period timeout with no events",
				"sessionId", sessionState.SessionID,
				"agentId", healthState.AgentID,
			)
		default:
			// Still within grace timeout and no events yet — ignore
			td.logger.Debug("Ignoring health change during grace period",
				"sessionId", sessionState.SessionID,
				"agentId", healthState.AgentID,
			)
			td.sessionsMu.Unlock()
			return
		}
	}

	switch healthState.Status {
	case wazuh.AgentStatusDisconnected:
		if inc := td.handleAgentOffline(sessionState, healthState); inc != nil {
			pendingIncidents = append(pendingIncidents, *inc)
		}
	case wazuh.AgentStatusActive:
		if healthState.ReconnectedAt != nil {
			if inc := td.handleAgentReconnect(sessionState, healthState); inc != nil {
				pendingIncidents = append(pendingIncidents, *inc)
			}
		}
	}

	td.sessionsMu.Unlock()

	for i := range pendingIncidents {
		td.notifyCallbacks(&pendingIncidents[i])
	}
}

// handleAgentOffline mutates state under sessionsMu and returns the incident
// (if any) so the caller can dispatch callbacks after releasing the lock.
func (td *TamperingDetector) handleAgentOffline(state *SessionTamperingState, health *wazuh.AgentHealthState) *TamperingIncident {
	incident := TamperingIncident{
		ID:          fmt.Sprintf("%s-%d", state.SessionID, len(state.Incidents)+1),
		SessionID:   state.SessionID,
		PodID:       state.PodID,
		AgentID:     health.AgentID,
		Type:        "agent_offline",
		Severity:    "critical",
		Description: "Wazuh agent went offline - possible tampering attempt",
		DetectedAt:  time.Now(),
	}

	// Apply penalty
	penalty := td.config.PenaltyPerIncident
	if state.TotalPenalty+penalty > td.config.MaxPenalty {
		penalty = td.config.MaxPenalty - state.TotalPenalty
	}
	incident.PenaltyApplied = penalty
	state.TotalPenalty += penalty

	// Flag for review if configured
	if td.config.AutoFlagForReview {
		state.FlaggedForReview = true
	}

	state.Incidents = append(state.Incidents, incident)

	td.logger.Warn("Tampering incident detected: agent offline",
		"sessionId", state.SessionID,
		"agentId", health.AgentID,
		"penalty", penalty,
		"totalPenalty", state.TotalPenalty,
	)

	return &incident
}

// handleAgentReconnect tracks agent restarts. Returns the incident (if any)
// for the caller to dispatch after releasing the lock; nil when no incident.
func (td *TamperingDetector) handleAgentReconnect(state *SessionTamperingState, health *wazuh.AgentHealthState) *TamperingIncident {
	state.AgentRestarts++
	state.LastAgentSeen = time.Now()

	if state.AgentRestarts < td.config.RestartThreshold {
		return nil
	}

	incident := TamperingIncident{
		ID:          fmt.Sprintf("%s-%d", state.SessionID, len(state.Incidents)+1),
		SessionID:   state.SessionID,
		PodID:       state.PodID,
		AgentID:     health.AgentID,
		Type:        "agent_restart",
		Severity:    "warning",
		Description: fmt.Sprintf("Agent restarted %d times - suspicious activity", state.AgentRestarts),
		DetectedAt:  time.Now(),
	}

	// Calculate offline duration if available
	if health.DisconnectedAt != nil && health.ReconnectedAt != nil {
		incident.Duration = health.ReconnectedAt.Sub(*health.DisconnectedAt)
	}

	// Apply penalty for excessive restarts
	penalty := td.config.PenaltyPerIncident / 2 // Half penalty for restarts
	if state.TotalPenalty+penalty > td.config.MaxPenalty {
		penalty = td.config.MaxPenalty - state.TotalPenalty
	}
	incident.PenaltyApplied = penalty
	state.TotalPenalty += penalty

	state.Incidents = append(state.Incidents, incident)

	td.logger.Warn("Tampering incident detected: excessive agent restarts",
		"sessionId", state.SessionID,
		"agentId", health.AgentID,
		"restarts", state.AgentRestarts,
		"penalty", penalty,
	)

	return &incident
}

// RecordVerificationMismatch records when active verification doesn't match passive events.
// Mutation happens under sessionsMu; callbacks fire after the lock is released.
func (td *TamperingDetector) RecordVerificationMismatch(sessionID, checkpointID string, passiveResult, activeResult bool) {
	td.sessionsMu.Lock()

	state, exists := td.sessions[sessionID]
	if !exists {
		td.sessionsMu.Unlock()
		return
	}

	incident := TamperingIncident{
		ID:          fmt.Sprintf("%s-%d", sessionID, len(state.Incidents)+1),
		SessionID:   sessionID,
		PodID:       state.PodID,
		Type:        "verification_mismatch",
		Severity:    "critical",
		Description: fmt.Sprintf("Checkpoint %s: passive=%v but active=%v - possible event spoofing", checkpointID, passiveResult, activeResult),
		DetectedAt:  time.Now(),
	}

	// Apply full penalty for verification mismatch (likely spoofing)
	penalty := td.config.PenaltyPerIncident
	if state.TotalPenalty+penalty > td.config.MaxPenalty {
		penalty = td.config.MaxPenalty - state.TotalPenalty
	}
	incident.PenaltyApplied = penalty
	state.TotalPenalty += penalty

	// Always flag verification mismatches for review
	state.FlaggedForReview = true

	state.Incidents = append(state.Incidents, incident)

	td.logger.Error("Tampering incident detected: verification mismatch",
		"sessionId", sessionID,
		"checkpointId", checkpointID,
		"passiveResult", passiveResult,
		"activeResult", activeResult,
		"penalty", penalty,
	)

	td.sessionsMu.Unlock()

	td.notifyCallbacks(&incident)
}

// notifyCallbacks fires all registered callbacks for an incident. Each
// callback runs in its own goroutine, tracked by td.wg so Stop can wait.
// If the detector has stopped, callbacks are silently dropped — there is
// no useful post-shutdown handler. Caller MUST NOT hold sessionsMu when
// invoking this (callbacks may take arbitrary time on network I/O).
func (td *TamperingDetector) notifyCallbacks(incident *TamperingIncident) {
	if td.stopped.Load() {
		return
	}

	td.callbacksMu.RLock()
	callbacks := make([]TamperingCallback, len(td.callbacks))
	copy(callbacks, td.callbacks)
	td.callbacksMu.RUnlock()

	for _, cb := range callbacks {
		td.wg.Add(1)
		go func(callback TamperingCallback) {
			defer td.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					td.logger.Error("Tampering callback panicked", "panic", r)
				}
			}()
			callback(td.ctx, incident)
		}(cb)
	}
}

// AcknowledgeIncident marks an incident as reviewed by an instructor
func (td *TamperingDetector) AcknowledgeIncident(sessionID, incidentID string) error {
	td.sessionsMu.Lock()
	defer td.sessionsMu.Unlock()

	state, exists := td.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	for i := range state.Incidents {
		if state.Incidents[i].ID == incidentID {
			state.Incidents[i].Acknowledged = true
			return nil
		}
	}

	return fmt.Errorf("incident not found: %s", incidentID)
}

// GetFlaggedSessions returns all sessions flagged for review
func (td *TamperingDetector) GetFlaggedSessions() []*SessionTamperingState {
	td.sessionsMu.RLock()
	defer td.sessionsMu.RUnlock()

	var flagged []*SessionTamperingState
	for _, state := range td.sessions {
		if state.FlaggedForReview {
			stateCopy := *state
			stateCopy.Incidents = make([]TamperingIncident, len(state.Incidents))
			copy(stateCopy.Incidents, state.Incidents)
			flagged = append(flagged, &stateCopy)
		}
	}

	return flagged
}

// ApplyPenaltyToScore calculates the final score after applying tampering penalties
func (td *TamperingDetector) ApplyPenaltyToScore(sessionID string, earnedPoints, maxPoints int) (adjustedEarned, adjustedMax int) {
	penalty := td.GetPenalty(sessionID)
	if penalty == 0 {
		return earnedPoints, maxPoints
	}

	// Calculate penalty as percentage of max points
	penaltyPoints := (maxPoints * penalty) / 100
	adjustedScore := earnedPoints - penaltyPoints
	if adjustedScore < 0 {
		adjustedScore = 0
	}

	return adjustedScore, maxPoints
}
