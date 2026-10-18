package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/wazuh"
)

func TestHandleListAgentHealth(t *testing.T) {
	t.Run("wazuh not configured returns empty list", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		agents, ok := response["agents"].([]interface{})
		if !ok {
			t.Fatalf("agents is not an array: %v", response["agents"])
		}
		if len(agents) != 0 {
			t.Errorf("expected empty agents list, got %d", len(agents))
		}

		if response["message"] != "Wazuh health monitoring not configured" {
			t.Errorf("unexpected message: %s", response["message"])
		}
	})

	t.Run("with health monitor returns agents", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		hm.RegisterAgent("agent-001", "pod-123", "vm-web")
		hm.RegisterAgent("agent-002", "pod-123", "vm-db")

		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		agents, ok := response["agents"].([]interface{})
		if !ok {
			t.Fatalf("agents is not an array: %v", response["agents"])
		}
		if len(agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(agents))
		}

		count, ok := response["count"].(float64)
		if !ok || int(count) != 2 {
			t.Errorf("expected count 2, got %v", response["count"])
		}
	})
}

func TestHandleGetAgentHealth(t *testing.T) {
	t.Run("wazuh not configured returns 503", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/agent-001", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("agent not found returns 404", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/nonexistent", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("existing agent returns health state", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		hm.RegisterAgent("agent-001", "pod-123", "vm-web")

		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/agent-001", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var state wazuh.AgentHealthState
		if err := json.NewDecoder(rr.Body).Decode(&state); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if state.AgentID != "agent-001" {
			t.Errorf("expected agent ID agent-001, got %s", state.AgentID)
		}
		if state.PodID != "pod-123" {
			t.Errorf("expected pod ID pod-123, got %s", state.PodID)
		}
		if state.VMName != "vm-web" {
			t.Errorf("expected VM name vm-web, got %s", state.VMName)
		}
	})
}

func TestHandleGetPodAgentHealth(t *testing.T) {
	t.Run("wazuh not configured returns empty list", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/pod/pod-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["podId"] != "pod-123" {
			t.Errorf("expected podId pod-123, got %s", response["podId"])
		}
	})

	t.Run("returns agents for specific pod", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		hm.RegisterAgent("agent-001", "pod-123", "vm-web")
		hm.RegisterAgent("agent-002", "pod-123", "vm-db")
		hm.RegisterAgent("agent-003", "pod-456", "vm-other")

		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/pod/pod-123", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		agents, ok := response["agents"].([]interface{})
		if !ok {
			t.Fatalf("agents is not an array: %v", response["agents"])
		}
		if len(agents) != 2 {
			t.Errorf("expected 2 agents for pod-123, got %d", len(agents))
		}

		count, ok := response["count"].(float64)
		if !ok || int(count) != 2 {
			t.Errorf("expected count 2, got %v", response["count"])
		}

		if response["podId"] != "pod-123" {
			t.Errorf("expected podId pod-123, got %s", response["podId"])
		}
	})

	t.Run("empty pod returns empty list", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		hm.RegisterAgent("agent-001", "pod-123", "vm-web")

		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/pod/pod-999", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// agents can be nil or empty array when no agents match
		agents, _ := response["agents"].([]interface{})
		if len(agents) != 0 {
			t.Errorf("expected 0 agents for pod-999, got %d", len(agents))
		}

		count, ok := response["count"].(float64)
		if !ok || int(count) != 0 {
			t.Errorf("expected count 0, got %v", response["count"])
		}
	})
}

func TestHandleGetAgentHealthSummary(t *testing.T) {
	t.Run("wazuh not configured returns empty summary", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/summary", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var summary wazuh.HealthSummary
		if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if summary.TotalAgents != 0 {
			t.Errorf("expected 0 total agents, got %d", summary.TotalAgents)
		}
	})

	t.Run("returns correct summary", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		// Register agents in pending state
		hm.RegisterAgent("agent-001", "pod-123", "vm-web")
		hm.RegisterAgent("agent-002", "pod-123", "vm-db")
		hm.RegisterAgent("agent-003", "pod-456", "vm-other")

		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/summary", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var summary wazuh.HealthSummary
		if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if summary.TotalAgents != 3 {
			t.Errorf("expected 3 total agents, got %d", summary.TotalAgents)
		}
		// Newly registered agents are in pending state
		if summary.PendingAgents != 3 {
			t.Errorf("expected 3 pending agents, got %d", summary.PendingAgents)
		}
	})
}

func TestHandleGetDisconnectedAgents(t *testing.T) {
	t.Run("wazuh not configured returns empty list", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/disconnected", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		agents, ok := response["agents"].([]interface{})
		if !ok {
			t.Fatalf("agents is not an array: %v", response["agents"])
		}
		if len(agents) != 0 {
			t.Errorf("expected empty agents list, got %d", len(agents))
		}
	})

	t.Run("returns only disconnected agents", func(t *testing.T) {
		hm := wazuh.NewHealthMonitor(wazuh.DefaultHealthMonitorConfig(), newTestLogger(), nil)
		// Register agents - they start in pending state
		hm.RegisterAgent("agent-001", "pod-123", "vm-web")
		hm.RegisterAgent("agent-002", "pod-123", "vm-db")

		srv := newTestServer(t, WithWazuhHealthMonitor(hm))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/agents/disconnected", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// agents can be nil or empty array when no agents are disconnected
		agents, _ := response["agents"].([]interface{})
		// Pending agents are not disconnected
		if len(agents) != 0 {
			t.Errorf("expected 0 disconnected agents (pending != disconnected), got %d", len(agents))
		}

		count, ok := response["count"].(float64)
		if !ok || int(count) != 0 {
			t.Errorf("expected count 0, got %v", response["count"])
		}
	})
}

func TestHandleWebhookValidatorStatus(t *testing.T) {
	t.Run("validator not configured returns disabled status", func(t *testing.T) {
		srv := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/webhook/status", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var status WebhookValidatorStatus
		if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if status.Enabled {
			t.Error("expected validator to be disabled")
		}
	})

	t.Run("validator configured returns enabled status", func(t *testing.T) {
		validator, err := wazuh.NewWebhookValidator(t.Context(), wazuh.SecurityConfig{
			WebhookSecret:      "test-secret",
			RateLimitPerMinute: 100,
		}, newTestLogger())
		if err != nil {
			t.Fatalf("failed to create webhook validator: %v", err)
		}
		srv := newTestServer(t, WithWebhookValidator(validator))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/webhook/status", nil)
		rr := httptest.NewRecorder()

		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var status WebhookValidatorStatus
		if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !status.Enabled {
			t.Error("expected validator to be enabled")
		}
	})
}
