// Package server provides the HTTP server and API routes
package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// -----------------------------------------------------------------------------
// Event Monitoring Manager
// -----------------------------------------------------------------------------

// EventMonitoringManager manages event monitoring operations (list, stats, filtering)
type EventMonitoringManager struct {
	eventRepo repositories.EventRepository
	logger    *slog.Logger
	responder *httputil.Responder
}

// EventMonitoringManagerConfig configures EventMonitoringManager
type EventMonitoringManagerConfig struct {
	EventRepo repositories.EventRepository
	Logger    *slog.Logger
	Responder *httputil.Responder
}

// NewEventMonitoringManager creates a new EventMonitoringManager
func NewEventMonitoringManager(cfg EventMonitoringManagerConfig) *EventMonitoringManager {
	return &EventMonitoringManager{
		eventRepo: cfg.EventRepo,
		logger:    cfg.Logger,
		responder: cfg.Responder,
	}
}

// -----------------------------------------------------------------------------
// Event Monitoring API - Real-time event dashboard
// -----------------------------------------------------------------------------

// EventSummary represents a summarized event for the dashboard
type EventSummary struct {
	ID                 int64     `json:"id"`
	Timestamp          time.Time `json:"timestamp"`
	PodID              string    `json:"podId"`
	SessionID          string    `json:"sessionId,omitempty"`
	VMName             string    `json:"vmName"`
	EventType          string    `json:"eventType"`
	RuleID             string    `json:"ruleId,omitempty"`
	RuleLevel          int       `json:"ruleLevel,omitempty"`
	Description        string    `json:"description,omitempty"`
	Processed          bool      `json:"processed"`
	MatchedCheckpoints []string  `json:"matchedCheckpoints,omitempty"`
}

// EventsResponse wraps the events list response
type EventsResponse struct {
	Events []EventSummary `json:"events"`
	Count  int            `json:"count"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// EventStatsResponse provides event monitoring statistics
type EventStatsResponse struct {
	TotalEvents        int64            `json:"totalEvents"`
	EventsToday        int64            `json:"eventsToday"`
	EventsThisHour     int64            `json:"eventsThisHour"`
	ProcessedEvents    int64            `json:"processedEvents"`
	PendingEvents      int64            `json:"pendingEvents"`
	EventsByType       map[string]int64 `json:"eventsByType"`
	CheckpointsPassed  int64            `json:"checkpointsPassed"`
	CheckpointsMatched int64            `json:"checkpointsMatched"`
	GeneratedAt        time.Time        `json:"generatedAt"`
}

// SetupRoutes registers the events monitoring routes
func (m *EventMonitoringManager) SetupRoutes(r chi.Router) {
	r.Route("/monitoring", func(r chi.Router) {
		// List recent events with filtering
		r.Get("/events", m.handleListEvents)
		// Get event statistics
		r.Get("/stats", m.handleEventStats)
		// Get events for a specific session
		r.Get("/events/session/{sessionID}", m.handleSessionEvents)
		// Get events for a specific pod
		r.Get("/events/pod/{podID}", m.handlePodEvents)
	})
}

// handleListEvents returns recent events with filtering
// @Summary List events
// @Description Get recent events with optional filtering by type, pod, session
// @Tags events
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of events (default 50, max 500)"
// @Param offset query int false "Offset for pagination"
// @Param eventType query string false "Filter by event type (syscheck, audit, auth, ssh)"
// @Param podId query string false "Filter by pod ID"
// @Param sessionId query string false "Filter by session ID"
// @Param vmName query string false "Filter by VM name"
// @Param start query string false "Start time (RFC3339 format)"
// @Param end query string false "End time (RFC3339 format)"
// @Success 200 {object} EventsResponse
// @Router /events [get]
func (m *EventMonitoringManager) handleListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	limit := 50
	offset := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Build filter from query parameters
	filter := repositories.EventFilter{
		PodID:     r.URL.Query().Get("podId"),
		SessionID: r.URL.Query().Get("sessionId"),
		EventType: r.URL.Query().Get("eventType"),
		VMName:    r.URL.Query().Get("vmName"),
		Limit:     limit,
		Offset:    offset,
	}

	// Parse time range
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if start, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartTime = &start
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if end, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndTime = &end
		}
	}

	// Check if event repository is available
	if m.eventRepo == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "events.errors.notConfigured", nil)
		return
	}

	events, err := m.eventRepo.Query(ctx, filter)
	if err != nil {
		m.logger.Error("Failed to query events", "error", err)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "events.errors.queryFailed", nil)
		return
	}

	// Convert to summaries
	summaries := make([]EventSummary, len(events))
	for i, e := range events {
		summaries[i] = EventSummary{
			ID:                 e.ID,
			Timestamp:          e.Timestamp,
			PodID:              e.PodID,
			SessionID:          e.SessionID,
			VMName:             e.VMName,
			EventType:          e.EventType,
			RuleID:             e.WazuhRuleID,
			RuleLevel:          e.WazuhLevel,
			Description:        e.WazuhRuleDesc,
			Processed:          e.Processed,
			MatchedCheckpoints: e.MatchedCheckpoints,
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, EventsResponse{
		Events: summaries,
		Count:  len(summaries),
		Limit:  limit,
		Offset: offset,
	})
}

// handleEventStats returns event monitoring statistics
// @Summary Get event statistics
// @Description Get aggregated statistics about events
// @Tags events
// @Produce json
// @Success 200 {object} EventStatsResponse
// @Router /events/stats [get]
func (m *EventMonitoringManager) handleEventStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	stats := EventStatsResponse{
		EventsByType: make(map[string]int64),
		GeneratedAt:  now,
	}

	if m.eventRepo == nil {
		m.responder.JSONResponse(w, http.StatusOK, stats)
		return
	}

	// Count today's events
	todayEvents, err := m.eventRepo.Query(ctx, repositories.EventFilter{
		StartTime: &startOfDay,
		Limit:     10000, // Large limit to get all
	})
	if err == nil {
		stats.EventsToday = int64(len(todayEvents))
		// Count by type
		for _, e := range todayEvents {
			stats.EventsByType[e.EventType]++
			if e.Processed {
				stats.ProcessedEvents++
			}
			if len(e.MatchedCheckpoints) > 0 {
				stats.CheckpointsMatched++
				stats.CheckpointsPassed += int64(len(e.MatchedCheckpoints))
			}
		}
	}

	// Count this hour's events
	hourEvents, err := m.eventRepo.Query(ctx, repositories.EventFilter{
		StartTime: &startOfHour,
		Limit:     10000,
	})
	if err == nil {
		stats.EventsThisHour = int64(len(hourEvents))
	}

	// Get pending events count
	unprocessed, err := m.eventRepo.GetUnprocessed(ctx, 10000)
	if err == nil {
		stats.PendingEvents = int64(len(unprocessed))
	}

	// Total events (rough estimate based on max ID or actual count)
	allEvents, err := m.eventRepo.Query(ctx, repositories.EventFilter{
		Limit: 10000,
	})
	if err == nil {
		stats.TotalEvents = int64(len(allEvents))
	}

	m.responder.JSONResponse(w, http.StatusOK, stats)
}

// handleSessionEvents returns events for a specific session
// @Summary Get session events
// @Description Get all events for a specific session
// @Tags events
// @Produce json
// @Param sessionID path string true "Session ID"
// @Param limit query int false "Maximum number of events (default 100)"
// @Success 200 {object} EventsResponse
// @Router /events/session/{sessionID} [get]
func (m *EventMonitoringManager) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID := chi.URLParam(r, "sessionID")

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	if m.eventRepo == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "events.errors.notConfigured", nil)
		return
	}

	events, err := m.eventRepo.GetBySessionID(ctx, sessionID, limit)
	if err != nil {
		m.logger.Error("Failed to get session events", "error", err, "sessionId", sessionID)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "events.errors.sessionEventsFailed", nil)
		return
	}

	summaries := make([]EventSummary, len(events))
	for i, e := range events {
		summaries[i] = EventSummary{
			ID:                 e.ID,
			Timestamp:          e.Timestamp,
			PodID:              e.PodID,
			SessionID:          e.SessionID,
			VMName:             e.VMName,
			EventType:          e.EventType,
			RuleID:             e.WazuhRuleID,
			RuleLevel:          e.WazuhLevel,
			Description:        e.WazuhRuleDesc,
			Processed:          e.Processed,
			MatchedCheckpoints: e.MatchedCheckpoints,
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, EventsResponse{
		Events: summaries,
		Count:  len(summaries),
		Limit:  limit,
	})
}

// handlePodEvents returns events for a specific pod
// @Summary Get pod events
// @Description Get all events for a specific pod
// @Tags events
// @Produce json
// @Param podID path string true "Pod ID"
// @Param limit query int false "Maximum number of events (default 100)"
// @Success 200 {object} EventsResponse
// @Router /events/pod/{podID} [get]
func (m *EventMonitoringManager) handlePodEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	podID := chi.URLParam(r, "podID")

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	if m.eventRepo == nil {
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "events.errors.notConfigured", nil)
		return
	}

	events, err := m.eventRepo.GetByPodID(ctx, podID, limit)
	if err != nil {
		m.logger.Error("Failed to get pod events", "error", err, "podId", podID)
		m.responder.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "events.errors.podEventsFailed", nil)
		return
	}

	summaries := make([]EventSummary, len(events))
	for i, e := range events {
		summaries[i] = EventSummary{
			ID:                 e.ID,
			Timestamp:          e.Timestamp,
			PodID:              e.PodID,
			SessionID:          e.SessionID,
			VMName:             e.VMName,
			EventType:          e.EventType,
			RuleID:             e.WazuhRuleID,
			RuleLevel:          e.WazuhLevel,
			Description:        e.WazuhRuleDesc,
			Processed:          e.Processed,
			MatchedCheckpoints: e.MatchedCheckpoints,
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, EventsResponse{
		Events: summaries,
		Count:  len(summaries),
		Limit:  limit,
	})
}

// WithEventRepo sets the event repository option
func WithEventRepo(repo repositories.EventRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().eventRepo = repo
	}
}
