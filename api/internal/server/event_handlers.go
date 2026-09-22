// Package server provides the HTTP server and API routes
package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/wazuh"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// -----------------------------------------------------------------------------
// Wazuh Event Manager
// -----------------------------------------------------------------------------

// WazuhEventManager manages Wazuh webhook event processing handlers.
type WazuhEventManager struct {
	webhookValidator   *wazuh.WebhookValidator
	eventDeduplicator  *wazuh.EventDeduplicator
	wazuhHealthMonitor *wazuh.HealthMonitor
	tamperingDetector  *checkpoint.TamperingDetector
	sessionRepo        repositories.SessionRepository
	activeCheckRunner  *checkpoint.ActiveCheckRunner
	evaluator          *checkpoint.Evaluator
	labTemplateRepo    repositories.LabTemplateRepository
	eventRepo          repositories.EventRepository
	wsHub              *websocket.Hub
	logger             *slog.Logger
	responder          *httputil.Responder
}

// WazuhEventManagerConfig configures WazuhEventManager.
type WazuhEventManagerConfig struct {
	WebhookValidator   *wazuh.WebhookValidator
	EventDeduplicator  *wazuh.EventDeduplicator
	WazuhHealthMonitor *wazuh.HealthMonitor
	TamperingDetector  *checkpoint.TamperingDetector
	SessionRepo        repositories.SessionRepository
	ActiveCheckRunner  *checkpoint.ActiveCheckRunner
	Evaluator          *checkpoint.Evaluator
	LabTemplateRepo    repositories.LabTemplateRepository
	EventRepo          repositories.EventRepository
	WsHub              *websocket.Hub
	Logger             *slog.Logger
}

// NewWazuhEventManager creates a new WazuhEventManager.
func NewWazuhEventManager(cfg WazuhEventManagerConfig) *WazuhEventManager {
	return &WazuhEventManager{
		webhookValidator:   cfg.WebhookValidator,
		eventDeduplicator:  cfg.EventDeduplicator,
		wazuhHealthMonitor: cfg.WazuhHealthMonitor,
		tamperingDetector:  cfg.TamperingDetector,
		sessionRepo:        cfg.SessionRepo,
		activeCheckRunner:  cfg.ActiveCheckRunner,
		evaluator:          cfg.Evaluator,
		labTemplateRepo:    cfg.LabTemplateRepo,
		eventRepo:          cfg.EventRepo,
		wsHub:              cfg.WsHub,
		logger:             cfg.Logger,
		responder:          httputil.NewResponder(cfg.Logger),
	}
}

// SetupRoutes registers the Wazuh webhook route on the router.
// This is mounted on the PUBLIC sub-router — Wazuh posts here unauthenticated
// and HMAC signatures are verified inside the handler.
func (m *WazuhEventManager) SetupRoutes(r chi.Router) {
	r.Route("/events", func(r chi.Router) {
		r.Post("/wazuh", m.handleWazuhWebhook)
	})
}

func (m *WazuhEventManager) handleWazuhWebhook(w http.ResponseWriter, r *http.Request) {
	// Read request body first — needed for HMAC signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "event.errors.readBodyFailed", nil)
		return
	}
	defer r.Body.Close()

	// SECURITY: Validate webhook request (IP, rate limit, signature)
	if m.webhookValidator != nil {
		if err := m.webhookValidator.ValidateRequest(r, body); err != nil {
			m.logger.Warn("Webhook security validation failed",
				"error", err,
				"remote_addr", r.RemoteAddr,
				"x_forwarded_for", r.Header.Get("X-Forwarded-For"),
			)
			// Return generic error to avoid information leakage
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "event.errors.webhookValidationFailed", nil)
			return
		}
	}

	// SECURITY: Check for duplicate events (replay attack prevention)
	if m.eventDeduplicator != nil {
		// Peek at the alert ID from the body for dedup check
		var peek struct {
			Alert struct {
				ID string `json:"id"`
			} `json:"alert"`
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &peek) == nil {
			alertID := peek.Alert.ID
			if alertID == "" {
				alertID = peek.ID
			}
			if alertID != "" && m.eventDeduplicator.IsDuplicate(alertID) {
				m.logger.Warn("Duplicate event rejected",
					"alertId", alertID,
					"remote_addr", r.RemoteAddr,
				)
				m.responder.JSONResponse(w, http.StatusConflict, map[string]string{
					"status": "duplicate",
					"error":  "event already processed",
				})
				return
			}
		}
	}

	// Log incoming request for debugging
	m.logger.Debug("Received Wazuh webhook",
		"bodyLength", len(body),
		"bodyPreview", string(body[:min(200, len(body))]),
	)

	// Parse Wazuh alert - first try wrapped format {"alert": ...}, then raw format
	var alert *wazuh.Alert
	var payload wazuh.WebhookPayload
	unmarshalErr := json.Unmarshal(body, &payload)

	if unmarshalErr == nil && payload.Alert.ID != "" {
		// Wrapped format from integration script
		alert = &payload.Alert
		m.logger.Info("Parsed wrapped alert format", "alertId", alert.ID)
	} else if unmarshalErr == nil && payload.Alert.Timestamp != "" {
		// Wrapped format but without ID field - still valid, use it
		alert = &payload.Alert
		m.logger.Info("Parsed wrapped alert format (no ID)",
			"timestamp", alert.Timestamp,
			"ruleId", alert.Rule.ID,
			"agentName", alert.Agent.Name,
		)
	} else {
		// Raw alert format (direct API call)
		alert, err = wazuh.ParseAlert(body)
		if err != nil {
			m.logger.Error("Failed to parse Wazuh alert", "error", err, "bodyPreview", string(body[:min(200, len(body))]))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "event.errors.invalidAlertFormat", nil)
			return
		}
		m.logger.Info("Parsed raw alert format", "alertId", alert.ID)
	}

	// Convert to internal VMEvent
	vmEvent, err := alert.ToVMEvent()
	if err != nil {
		m.logger.Error("Failed to convert alert to VMEvent", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "event.errors.processAlertFailed", nil)
		return
	}

	// SECURITY: Validate event timestamp to prevent replay attacks
	if m.webhookValidator != nil && alert.Timestamp != "" {
		eventTime, parseErr := time.Parse(time.RFC3339, alert.Timestamp)
		if parseErr == nil {
			if tsErr := m.webhookValidator.ValidateTimestamp(eventTime); tsErr != nil {
				m.logger.Warn("Event timestamp validation failed",
					"error", tsErr,
					"alertId", alert.ID,
					"eventTimestamp", alert.Timestamp,
				)
				// Log but don't reject - timestamp drift may be due to clock skew
				// In production, consider rejecting: m.responder.ErrorResponse(w, http.StatusBadRequest, "event timestamp out of range")
			}
		}
	}

	m.logger.Debug("Received Wazuh event",
		"alertId", alert.ID,
		"ruleId", alert.Rule.ID,
		"podId", vmEvent.PodID,
		"vmName", vmEvent.VMName,
		"eventType", vmEvent.EventType,
	)

	// SECURITY: Record event for agent health monitoring (tamper detection)
	// This tracks agent activity to detect if students disable the Wazuh agent
	if m.wazuhHealthMonitor != nil && alert.Agent.ID != "" {
		m.wazuhHealthMonitor.RecordEvent(alert.Agent.ID)
	}

	// Notify tampering detector that we received an event (ends grace period)
	if m.tamperingDetector != nil {
		// Try to find session ID early for grace period tracking
		earlySessionID := alert.GetSessionID()
		if earlySessionID == "" {
			earlySessionID = r.URL.Query().Get("sessionId")
		}
		if earlySessionID != "" {
			m.tamperingDetector.OnFirstEvent(earlySessionID)
		}
	}

	// Get session ID from alert labels, query parameter, or by looking up pod
	sessionID := alert.GetSessionID()
	if sessionID == "" {
		sessionID = r.URL.Query().Get("sessionId")
	}

	// If no session ID, try to extract pod ID from agent name and look up session
	if sessionID == "" && m.sessionRepo != nil {
		podID := extractPodIDFromAgentName(alert.Agent.Name)
		if podID != "" {
			vmEvent.PodID = podID
			// Find active session for this pod
			activeFilter := true
			sessions, err := m.sessionRepo.List(r.Context(), repositories.SessionFilter{
				PodID:  podID,
				Active: &activeFilter,
			})
			if err == nil && len(sessions) > 0 {
				sessionID = sessions[0].ID
				m.logger.Debug("Found session from pod ID",
					"podId", podID,
					"sessionId", sessionID,
				)
			}
		}
	}

	// If still no session, try to look up by agent IP address
	if sessionID == "" && m.activeCheckRunner != nil && alert.Agent.IP != "" {
		foundSessionID, foundPodID := m.activeCheckRunner.FindSessionByVMIP(alert.Agent.IP)
		if foundSessionID != "" {
			sessionID = foundSessionID
			vmEvent.PodID = foundPodID
			m.logger.Debug("Found session from agent IP",
				"agentIP", alert.Agent.IP,
				"sessionId", sessionID,
				"podId", foundPodID,
			)
		}
	}

	// Track passed checkpoints for storage
	var passedCheckpoints []string

	// Notify tampering detector now that we have the resolved session ID
	if m.tamperingDetector != nil && sessionID != "" {
		m.tamperingDetector.OnFirstEvent(sessionID)
	}

	// If we have a session, evaluate against checkpoints
	if sessionID != "" {
		m.logger.Info("Evaluating event against checkpoints",
			"sessionId", sessionID,
			"eventType", vmEvent.EventType,
			"podId", vmEvent.PodID,
		)
		var evalErr error
		passedCheckpoints, evalErr = m.evaluator.EvaluateEvent(r.Context(), vmEvent, sessionID)
		if evalErr != nil {
			// Session not registered in evaluator - try to auto-register it
			if strings.Contains(evalErr.Error(), "not found") && m.sessionRepo != nil {
				session, dbErr := m.sessionRepo.GetByID(r.Context(), sessionID)
				if dbErr == nil && session != nil {
					// Get template info from database
					if m.labTemplateRepo != nil {
						template, tmplErr := m.labTemplateRepo.GetByID(r.Context(), session.LabTemplateID)
						if tmplErr == nil && template != nil {
							// Auto-register the session with the evaluator
							startErr := m.evaluator.StartSession(sessionID, session.PodID, session.UserID, template.Name)
							if startErr == nil {
								m.logger.Info("Auto-registered session with evaluator",
									"sessionId", sessionID,
									"template", template.Name,
								)
								// Retry evaluation
								passedCheckpoints, evalErr = m.evaluator.EvaluateEvent(r.Context(), vmEvent, sessionID)
								if evalErr == nil && len(passedCheckpoints) > 0 {
									m.logger.Info("Checkpoints passed after auto-registration",
										"sessionId", sessionID,
										"checkpoints", passedCheckpoints,
									)
								}
							}
						}
					}
				}
			}
			if evalErr != nil {
				m.logger.Warn("Failed to evaluate event", "error", evalErr, "sessionId", sessionID)
			}
		} else if len(passedCheckpoints) > 0 {
			m.logger.Info("Checkpoints passed",
				"sessionId", sessionID,
				"checkpoints", passedCheckpoints,
			)
		} else {
			m.logger.Info("No checkpoints matched", "sessionId", sessionID)
		}
	} else {
		m.logger.Warn("No session found for event",
			"agentName", alert.Agent.Name,
			"podId", vmEvent.PodID,
		)
	}

	// Store event to database for monitoring dashboard
	// Only store if we have a valid pod ID (required by database schema)
	if m.eventRepo != nil && vmEvent.PodID != "" {
		// Create event record
		event := &models.Event{
			Timestamp:          time.Now(),
			PodID:              vmEvent.PodID,
			SessionID:          sessionID,
			VMName:             vmEvent.VMName,
			AgentID:            alert.Agent.ID,
			EventType:          vmEvent.EventType,
			WazuhAlertID:       alert.ID,
			WazuhRuleID:        alert.Rule.ID,
			WazuhRuleDesc:      alert.Rule.Description,
			WazuhLevel:         alert.Rule.Level,
			Data:               vmEvent.Data,
			Processed:          len(passedCheckpoints) > 0,
			MatchedCheckpoints: passedCheckpoints,
		}

		if err := m.eventRepo.Create(r.Context(), event); err != nil {
			m.logger.Error("Failed to store event", "error", err)
		} else {
			m.logger.Debug("Event stored", "eventId", event.ID, "eventType", event.EventType)

			// Broadcast to WebSocket clients for real-time monitoring
			if m.wsHub != nil {
				monitoringEvent := &events.MonitoringEvent{
					ID:                 event.ID,
					Timestamp:          event.Timestamp,
					PodID:              event.PodID,
					SessionID:          event.SessionID,
					VMName:             event.VMName,
					AgentID:            event.AgentID,
					EventType:          event.EventType,
					RuleID:             event.WazuhRuleID,
					RuleLevel:          event.WazuhLevel,
					Description:        event.WazuhRuleDesc,
					Data:               event.Data,
					Processed:          event.Processed,
					MatchedCheckpoints: event.MatchedCheckpoints,
				}
				m.wsHub.BroadcastMonitoringEvent(monitoringEvent)
			}
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"status":    "received",
		"alertId":   alert.ID,
		"eventType": vmEvent.EventType,
		"podId":     vmEvent.PodID,
		"vmName":    vmEvent.VMName,
	})
}

// extractPodIDFromAgentName extracts the pod ID from a Wazuh agent name
// Agent names follow the format: {podID}-{vmName} (e.g., ad78a3b9-235c-4213-8a91-b40c35953707-linux-vm)
func extractPodIDFromAgentName(agentName string) string {
	// Pod IDs are UUIDs (8-4-4-4-12 format)
	// The agent name is {podID}-{vmName}, so we need to extract the first 36 characters
	// (the UUID) from names that are longer than 36 characters

	if len(agentName) < 36 {
		return ""
	}

	// Check if it looks like a UUID at the start
	potential := agentName[:36]
	if isValidUUID(potential) {
		return potential
	}

	// Try finding a UUID pattern in the name
	parts := strings.Split(agentName, "-")
	if len(parts) >= 5 {
		// UUID has 5 parts: 8-4-4-4-12 chars
		potential = strings.Join(parts[:5], "-")
		if len(potential) == 36 && isValidUUID(potential) {
			return potential
		}
	}

	return ""
}

// isValidUUID checks if a string is a valid UUID format
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
