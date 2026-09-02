package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/wazuh"
)

// ---------------------------------------------------------------------------
// SecurityManager
// ---------------------------------------------------------------------------

// SecurityManagerConfig holds configuration for creating a SecurityManager.
type SecurityManagerConfig struct {
	WazuhHealthMonitor *wazuh.HealthMonitor
	WebhookValidator   *wazuh.WebhookValidator
	Logger             *slog.Logger
}

// SecurityManager owns all security and monitoring HTTP handlers.
type SecurityManager struct {
	responder          *httputil.Responder
	wazuhHealthMonitor *wazuh.HealthMonitor
	webhookValidator   *wazuh.WebhookValidator
	logger             *slog.Logger
}

// NewSecurityManager creates a SecurityManager from the given config.
func NewSecurityManager(cfg SecurityManagerConfig) *SecurityManager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &SecurityManager{
		responder:          httputil.NewResponder(logger),
		wazuhHealthMonitor: cfg.WazuhHealthMonitor,
		webhookValidator:   cfg.WebhookValidator,
		logger:             logger,
	}
}

// SetupRoutes registers security and monitoring API routes on the given router.
func (m *SecurityManager) SetupRoutes(r chi.Router) {
	r.Route("/security", func(r chi.Router) {
		// Wazuh agent health monitoring
		// Note: More specific routes must come before parameterized routes
		r.Get("/agents", m.handleListAgentHealth)
		r.Get("/agents/summary", m.handleGetAgentHealthSummary)
		r.Get("/agents/disconnected", m.handleGetDisconnectedAgents)
		r.Get("/agents/pod/{podID}", m.handleGetPodAgentHealth)
		r.Get("/agents/{agentID}", m.handleGetAgentHealth)

		// Webhook validator status (admin only)
		r.Get("/webhook/status", m.handleWebhookValidatorStatus)
	})
}

// @Summary List all agent health states
// @Description Returns health status for all monitored Wazuh agents
// @Tags security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /security/agents [get]
func (m *SecurityManager) handleListAgentHealth(w http.ResponseWriter, r *http.Request) {
	if m.wazuhHealthMonitor == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"agents":  []any{},
			"message": "Wazuh health monitoring not configured",
		})
		return
	}

	agents := m.wazuhHealthMonitor.GetAllAgentsHealth()
	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"agents": agents,
		"count":  len(agents),
	})
}

// @Summary Get agent health by ID
// @Description Returns health status for a specific Wazuh agent
// @Tags security
// @Accept json
// @Produce json
// @Param agentID path string true "Agent ID"
// @Success 200 {object} wazuh.AgentHealthState
// @Failure 404 {object} map[string]string
// @Router /security/agents/{agentID} [get]
func (m *SecurityManager) handleGetAgentHealth(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	if m.wazuhHealthMonitor == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "security.errors.wazuhNotConfigured", nil)
		return
	}

	state, err := m.wazuhHealthMonitor.GetAgentHealth(agentID)
	if err != nil {
		m.responder.SafeErrorResponse(w, err, "get agent health")
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, state)
}

// @Summary Get agent health for a pod
// @Description Returns health status for all Wazuh agents in a specific pod
// @Tags security
// @Accept json
// @Produce json
// @Param podID path string true "Pod ID"
// @Success 200 {object} map[string]any
// @Router /security/agents/pod/{podID} [get]
func (m *SecurityManager) handleGetPodAgentHealth(w http.ResponseWriter, r *http.Request) {
	podID := chi.URLParam(r, "podID")

	if m.wazuhHealthMonitor == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"agents":  []any{},
			"podId":   podID,
			"message": "Wazuh health monitoring not configured",
		})
		return
	}

	agents := m.wazuhHealthMonitor.GetPodAgentsHealth(podID)
	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"agents": agents,
		"podId":  podID,
		"count":  len(agents),
	})
}

// @Summary Get agent health summary
// @Description Returns aggregated health summary for all monitored agents
// @Tags security
// @Accept json
// @Produce json
// @Success 200 {object} wazuh.HealthSummary
// @Router /security/agents/summary [get]
func (m *SecurityManager) handleGetAgentHealthSummary(w http.ResponseWriter, r *http.Request) {
	if m.wazuhHealthMonitor == nil {
		m.responder.JSONResponse(w, http.StatusOK, wazuh.HealthSummary{})
		return
	}

	summary := m.wazuhHealthMonitor.GetHealthSummary()
	m.responder.JSONResponse(w, http.StatusOK, summary)
}

// @Summary Get disconnected agents
// @Description Returns all agents currently in disconnected state
// @Tags security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /security/agents/disconnected [get]
func (m *SecurityManager) handleGetDisconnectedAgents(w http.ResponseWriter, r *http.Request) {
	if m.wazuhHealthMonitor == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"agents":  []any{},
			"message": "Wazuh health monitoring not configured",
		})
		return
	}

	agents := m.wazuhHealthMonitor.GetDisconnectedAgents()
	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"agents": agents,
		"count":  len(agents),
	})
}

// WebhookValidatorStatus represents the status of the webhook validator
type WebhookValidatorStatus struct {
	Enabled          bool     `json:"enabled"`
	RequireSignature bool     `json:"requireSignature"`
	AllowedIPsCount  int      `json:"allowedIpsCount"`
	RateLimitPerMin  int      `json:"rateLimitPerMinute"`
	AllowedIPs       []string `json:"allowedIps,omitempty"`
}

// @Summary Get webhook validator status
// @Description Returns configuration status of the Wazuh webhook validator
// @Tags security
// @Accept json
// @Produce json
// @Success 200 {object} WebhookValidatorStatus
// @Router /security/webhook/status [get]
func (m *SecurityManager) handleWebhookValidatorStatus(w http.ResponseWriter, r *http.Request) {
	if m.webhookValidator == nil {
		m.responder.JSONResponse(w, http.StatusOK, WebhookValidatorStatus{
			Enabled: false,
		})
		return
	}

	// Note: We don't expose the actual secret or full IP list for security
	status := WebhookValidatorStatus{
		Enabled: true,
		// These would need to be exposed via the validator
		// For now, just indicate it's enabled
	}

	m.responder.JSONResponse(w, http.StatusOK, status)
}
