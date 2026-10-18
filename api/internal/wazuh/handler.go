package wazuh

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// Handler handles Wazuh webhook requests
type Handler struct {
	service *Service
	logger  *slog.Logger
}

// NewHandler creates a new Wazuh webhook handler
func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// HandleWebhook processes incoming Wazuh alerts from the integrator
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error("failed to decode webhook payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	h.logger.Info("received Wazuh alert",
		"agent_id", payload.Alert.Agent.ID,
		"agent_name", payload.Alert.Agent.Name,
		"rule_id", payload.Alert.Rule.ID,
		"rule_level", payload.Alert.Rule.Level,
		"rule_description", payload.Alert.Rule.Description,
	)

	// Process the alert asynchronously with a detached context
	// (r.Context() is cancelled when the handler returns 202)
	detachedCtx := context.WithoutCancel(r.Context())
	go func() {
		if err := h.service.ProcessAlert(detachedCtx, &payload.Alert); err != nil {
			h.logger.Error("failed to process alert",
				"error", err,
				"agent_id", payload.Alert.Agent.ID,
				"rule_id", payload.Alert.Rule.ID,
			)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// HandleBulkWebhook processes multiple alerts in a single request
func (h *Handler) HandleBulkWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var alerts []Alert
	if err := json.NewDecoder(r.Body).Decode(&alerts); err != nil {
		h.logger.Error("failed to decode bulk webhook payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	h.logger.Info("received bulk Wazuh alerts", "count", len(alerts))

	// Process alerts asynchronously with a detached context
	// (r.Context() is cancelled when the handler returns 202)
	detachedCtx := context.WithoutCancel(r.Context())
	go func() {
		for _, alert := range alerts {
			alertCopy := alert // avoid closure capture issue
			if err := h.service.ProcessAlert(detachedCtx, &alertCopy); err != nil {
				h.logger.Error("failed to process alert",
					"error", err,
					"agent_id", alert.Agent.ID,
					"rule_id", alert.Rule.ID,
				)
			}
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
		"count":  strconv.Itoa(len(alerts)),
	})
}

// HandleHealthCheck returns the health status of the Wazuh integration
func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := h.service.HealthCheck(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if status.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(status)
}

// RegisterRoutes registers the Wazuh webhook routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/wazuh/webhook", h.HandleWebhook)
	mux.HandleFunc("/api/v1/wazuh/webhook/bulk", h.HandleBulkWebhook)
	mux.HandleFunc("/api/v1/wazuh/health", h.HandleHealthCheck)
}
