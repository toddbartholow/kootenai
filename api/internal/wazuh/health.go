// Package wazuh provides integration with Wazuh SIEM for the Kootenai platform
package wazuh

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/apperrors"
)

// ErrNotFound is the canonical not-found error from apperrors.
var ErrNotFound = apperrors.ErrNotFound

// AgentStatus represents the current status of a Wazuh agent
type AgentStatus string

const (
	// AgentStatusActive means the agent is connected and reporting
	AgentStatusActive AgentStatus = "active"
	// AgentStatusDisconnected means the agent has stopped reporting
	AgentStatusDisconnected AgentStatus = "disconnected"
	// AgentStatusNeverConnected means the agent was registered but never connected
	AgentStatusNeverConnected AgentStatus = "never_connected"
	// AgentStatusPending means the agent is pending connection
	AgentStatusPending AgentStatus = "pending"
)

// AgentHealthState tracks the health state of an agent
type AgentHealthState struct {
	AgentID           string      `json:"agentId"`
	PodID             string      `json:"podId"`
	VMName            string      `json:"vmName,omitempty"`
	Status            AgentStatus `json:"status"`
	LastSeen          time.Time   `json:"lastSeen,omitempty"`
	LastEvent         time.Time   `json:"lastEvent,omitempty"`
	DisconnectedAt    *time.Time  `json:"disconnectedAt,omitempty"`
	ReconnectedAt     *time.Time  `json:"reconnectedAt,omitempty"`
	DisconnectCount   int         `json:"disconnectCount"`
	ConsecutiveMisses int         `json:"consecutiveMisses"`
	ErrorMessage      string      `json:"errorMessage,omitempty"`
}

// HealthCallback is called when agent health status changes
type HealthCallback func(ctx context.Context, state *AgentHealthState)

// HealthMonitorConfig configures the health monitor
type HealthMonitorConfig struct {
	// CheckInterval is how often to check agent health
	CheckInterval time.Duration `yaml:"check_interval"`
	// DisconnectThreshold is how long without events before marking disconnected
	DisconnectThreshold time.Duration `yaml:"disconnect_threshold"`
	// MaxConsecutiveMisses before considering agent unhealthy
	MaxConsecutiveMisses int `yaml:"max_consecutive_misses"`
}

// DefaultHealthMonitorConfig returns the default health monitor configuration
func DefaultHealthMonitorConfig() HealthMonitorConfig {
	return HealthMonitorConfig{
		CheckInterval:        30 * time.Second,
		DisconnectThreshold:  5 * time.Minute,
		MaxConsecutiveMisses: 3,
	}
}

// HealthMonitor monitors the health of Wazuh agents
type HealthMonitor struct {
	config      HealthMonitorConfig
	logger      *slog.Logger
	apiClient   *APIClient
	agents      map[string]*AgentHealthState
	agentsMu    sync.RWMutex
	callbacks   []HealthCallback
	callbacksMu sync.RWMutex
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewHealthMonitor creates a new agent health monitor
func NewHealthMonitor(config HealthMonitorConfig, logger *slog.Logger, apiClient *APIClient) *HealthMonitor {
	if config.CheckInterval == 0 {
		config = DefaultHealthMonitorConfig()
	}
	return &HealthMonitor{
		config:    config,
		logger:    logger,
		apiClient: apiClient,
		agents:    make(map[string]*AgentHealthState),
		stopChan:  make(chan struct{}),
	}
}

// RegisterCallback adds a callback that will be called on agent status changes
func (h *HealthMonitor) RegisterCallback(cb HealthCallback) {
	h.callbacksMu.Lock()
	defer h.callbacksMu.Unlock()
	h.callbacks = append(h.callbacks, cb)
}

// Start begins the health monitoring loop
func (h *HealthMonitor) Start(ctx context.Context) {
	h.wg.Add(1)
	go h.monitorLoop(ctx)
	h.logger.Info("Agent health monitor started",
		slog.Duration("check_interval", h.config.CheckInterval),
		slog.Duration("disconnect_threshold", h.config.DisconnectThreshold),
	)
}

// Stop stops the health monitoring loop
func (h *HealthMonitor) Stop() {
	close(h.stopChan)
	h.wg.Wait()
	h.logger.Info("Agent health monitor stopped")
}

// RegisterAgent adds an agent to the health monitor
func (h *HealthMonitor) RegisterAgent(agentID, podID, vmName string) {
	h.agentsMu.Lock()
	defer h.agentsMu.Unlock()

	if _, exists := h.agents[agentID]; exists {
		return
	}

	h.agents[agentID] = &AgentHealthState{
		AgentID: agentID,
		PodID:   podID,
		VMName:  vmName,
		Status:  AgentStatusPending,
	}
	h.logger.Debug("Registered agent for health monitoring",
		slog.String("agent_id", agentID),
		slog.String("pod_id", podID),
	)
}

// UnregisterAgent removes an agent from the health monitor
func (h *HealthMonitor) UnregisterAgent(agentID string) {
	h.agentsMu.Lock()
	defer h.agentsMu.Unlock()
	delete(h.agents, agentID)
	h.logger.Debug("Unregistered agent from health monitoring", slog.String("agent_id", agentID))
}

// RecordEvent records that an event was received from an agent
func (h *HealthMonitor) RecordEvent(agentID string) {
	h.agentsMu.Lock()
	defer h.agentsMu.Unlock()

	state, exists := h.agents[agentID]
	if !exists {
		return
	}

	now := time.Now()
	wasDisconnected := state.Status == AgentStatusDisconnected

	state.LastEvent = now
	state.LastSeen = now
	state.ConsecutiveMisses = 0

	if wasDisconnected {
		// Agent reconnected
		state.Status = AgentStatusActive
		state.ReconnectedAt = &now
		h.logger.Info("Agent reconnected",
			slog.String("agent_id", agentID),
			slog.String("pod_id", state.PodID),
		)
		h.notifyCallbacksUnlocked(context.Background(), state)
	} else if state.Status != AgentStatusActive {
		// First event from agent
		state.Status = AgentStatusActive
		h.logger.Info("Agent now active",
			slog.String("agent_id", agentID),
			slog.String("pod_id", state.PodID),
		)
		h.notifyCallbacksUnlocked(context.Background(), state)
	}
}

// GetAgentHealth returns the health state for an agent
func (h *HealthMonitor) GetAgentHealth(agentID string) (*AgentHealthState, error) {
	h.agentsMu.RLock()
	defer h.agentsMu.RUnlock()

	state, exists := h.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s: %w", agentID, ErrNotFound)
	}

	// Return a copy to avoid race conditions
	stateCopy := *state
	return &stateCopy, nil
}

// GetPodAgentsHealth returns health state for all agents in a pod
func (h *HealthMonitor) GetPodAgentsHealth(podID string) []*AgentHealthState {
	h.agentsMu.RLock()
	defer h.agentsMu.RUnlock()

	var states []*AgentHealthState
	for _, state := range h.agents {
		if state.PodID == podID {
			stateCopy := *state
			states = append(states, &stateCopy)
		}
	}
	return states
}

// GetAllAgentsHealth returns health state for all monitored agents
func (h *HealthMonitor) GetAllAgentsHealth() []*AgentHealthState {
	h.agentsMu.RLock()
	defer h.agentsMu.RUnlock()

	states := make([]*AgentHealthState, 0, len(h.agents))
	for _, state := range h.agents {
		stateCopy := *state
		states = append(states, &stateCopy)
	}
	return states
}

// GetDisconnectedAgents returns all disconnected agents
func (h *HealthMonitor) GetDisconnectedAgents() []*AgentHealthState {
	h.agentsMu.RLock()
	defer h.agentsMu.RUnlock()

	var states []*AgentHealthState
	for _, state := range h.agents {
		if state.Status == AgentStatusDisconnected {
			stateCopy := *state
			states = append(states, &stateCopy)
		}
	}
	return states
}

// monitorLoop runs the periodic health check
func (h *HealthMonitor) monitorLoop(ctx context.Context) {
	defer h.wg.Done()

	ticker := time.NewTicker(h.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.checkAgentHealth(ctx)
		}
	}
}

// checkAgentHealth checks the health of all registered agents
func (h *HealthMonitor) checkAgentHealth(ctx context.Context) {
	h.agentsMu.Lock()
	defer h.agentsMu.Unlock()

	now := time.Now()
	threshold := h.config.DisconnectThreshold

	for agentID, state := range h.agents {
		// Skip agents that never connected
		if state.LastSeen.IsZero() {
			// Try to get status from API if available
			if h.apiClient != nil {
				h.checkAgentViaAPIUnlocked(ctx, agentID, state)
			}
			continue
		}

		// Check if agent has gone silent
		timeSinceLastSeen := now.Sub(state.LastSeen)
		if timeSinceLastSeen > threshold && state.Status == AgentStatusActive {
			// Agent has disconnected
			state.Status = AgentStatusDisconnected
			state.DisconnectedAt = &now
			state.DisconnectCount++
			state.ConsecutiveMisses++

			h.logger.Warn("Agent disconnected",
				slog.String("agent_id", agentID),
				slog.String("pod_id", state.PodID),
				slog.Duration("silent_duration", timeSinceLastSeen),
				slog.Int("disconnect_count", state.DisconnectCount),
			)

			h.notifyCallbacksUnlocked(ctx, state)
		} else if state.Status == AgentStatusDisconnected {
			// Already disconnected, increment consecutive misses
			state.ConsecutiveMisses++
		}
	}
}

// checkAgentViaAPIUnlocked checks agent status via Wazuh API (must be called with lock held)
func (h *HealthMonitor) checkAgentViaAPIUnlocked(ctx context.Context, agentID string, state *AgentHealthState) {
	if h.apiClient == nil {
		return
	}

	info, err := h.apiClient.GetAgent(ctx, agentID)
	if err != nil {
		state.ErrorMessage = fmt.Sprintf("API error: %v", err)
		return
	}

	// Update state based on API response
	switch info.Status {
	case "active":
		if state.Status != AgentStatusActive {
			now := time.Now()
			state.Status = AgentStatusActive
			state.LastSeen = now
			if state.Status == AgentStatusDisconnected {
				state.ReconnectedAt = &now
			}
			h.notifyCallbacksUnlocked(ctx, state)
		}
	case "disconnected":
		if state.Status != AgentStatusDisconnected {
			now := time.Now()
			state.Status = AgentStatusDisconnected
			state.DisconnectedAt = &now
			state.DisconnectCount++
			h.notifyCallbacksUnlocked(ctx, state)
		}
	case "never_connected":
		state.Status = AgentStatusNeverConnected
	case "pending":
		state.Status = AgentStatusPending
	}

	state.ErrorMessage = ""
}

// notifyCallbacksUnlocked notifies all registered callbacks (must be called with agentsMu lock held or from goroutine)
func (h *HealthMonitor) notifyCallbacksUnlocked(ctx context.Context, state *AgentHealthState) {
	h.callbacksMu.RLock()
	callbacks := make([]HealthCallback, len(h.callbacks))
	copy(callbacks, h.callbacks)
	h.callbacksMu.RUnlock()

	// Make a copy of state for callbacks
	stateCopy := *state

	// Run callbacks in goroutines to avoid blocking
	for _, cb := range callbacks {
		go func(callback HealthCallback) {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("Health callback panicked",
						slog.Any("panic", r),
						slog.String("agent_id", stateCopy.AgentID),
					)
				}
			}()
			callback(ctx, &stateCopy)
		}(cb)
	}
}

// HealthSummary returns a summary of all agent health states
type HealthSummary struct {
	TotalAgents        int `json:"totalAgents"`
	ActiveAgents       int `json:"activeAgents"`
	DisconnectedAgents int `json:"disconnectedAgents"`
	PendingAgents      int `json:"pendingAgents"`
	NeverConnected     int `json:"neverConnected"`
}

// GetHealthSummary returns a summary of agent health
func (h *HealthMonitor) GetHealthSummary() HealthSummary {
	h.agentsMu.RLock()
	defer h.agentsMu.RUnlock()

	summary := HealthSummary{
		TotalAgents: len(h.agents),
	}

	for _, state := range h.agents {
		switch state.Status {
		case AgentStatusActive:
			summary.ActiveAgents++
		case AgentStatusDisconnected:
			summary.DisconnectedAgents++
		case AgentStatusPending:
			summary.PendingAgents++
		case AgentStatusNeverConnected:
			summary.NeverConnected++
		}
	}

	return summary
}

// TryReconnectAgent attempts to reconnect a disconnected agent
func (h *HealthMonitor) TryReconnectAgent(ctx context.Context, agentID string) error {
	if h.apiClient == nil {
		return fmt.Errorf("API client not configured")
	}

	h.agentsMu.RLock()
	state, exists := h.agents[agentID]
	h.agentsMu.RUnlock()

	if !exists {
		return fmt.Errorf("agent %s: %w", agentID, ErrNotFound)
	}

	if state.Status != AgentStatusDisconnected {
		return fmt.Errorf("agent is not disconnected: %s", state.Status)
	}

	// Try to restart the agent via API
	err := h.apiClient.RestartAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("restarting agent: %w", err)
	}

	h.logger.Info("Initiated agent restart",
		slog.String("agent_id", agentID),
		slog.String("pod_id", state.PodID),
	)

	return nil
}
