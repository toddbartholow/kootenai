// Package websocket provides WebSocket functionality for real-time updates
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// BroadcastPayload defines the types that can be broadcast via WebSocket
// This type constraint ensures only valid payload types are used
type BroadcastPayload interface {
	*events.CheckpointUpdate | *events.SessionEvent | *events.GradeUpdate | *models.AssessmentUpdate | *events.MonitoringEvent
}

// marshalPayload is a type-safe wrapper for marshaling broadcast payloads
func marshalPayload[T BroadcastPayload](payload T) json.RawMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512

	// Size of the send buffer
	sendBufferSize = 256
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients by pod ID
	clients map[string]map[*Client]bool

	// All clients
	allClients map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast channel for messages
	broadcast chan *BroadcastMessage

	// Allowed origins for CSRF protection (empty = allow all for dev)
	allowedOrigins []string

	mu       sync.RWMutex
	wg       sync.WaitGroup // Tracks goroutines for graceful shutdown
	logger   *slog.Logger
	upgrader websocket.Upgrader
}

// Client represents a WebSocket connection
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Pod ID this client is subscribed to (optional)
	podID string

	// Session ID this client is associated with (optional)
	sessionID string

	// User ID
	userID string
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	// Target pod ID (empty for all clients)
	PodID string

	// Target session ID (empty for all in pod)
	SessionID string

	// Message type
	Type string

	// Message payload (any is Go 1.18+ alias for any)
	Payload any
}

// HubOption is a functional option for configuring the Hub
type HubOption func(*Hub)

// WithAllowedOrigins sets the allowed origins for CSRF protection
func WithAllowedOrigins(origins []string) HubOption {
	return func(h *Hub) {
		h.allowedOrigins = origins
	}
}

// NewHub creates a new WebSocket hub
func NewHub(logger *slog.Logger, opts ...HubOption) *Hub {
	h := &Hub{
		clients:        make(map[string]map[*Client]bool),
		allClients:     make(map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		broadcast:      make(chan *BroadcastMessage, 256),
		allowedOrigins: []string{}, // Empty = development mode (allow all with warning)
		logger:         logger,
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	// Configure upgrader with origin check
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	return h
}

// checkOrigin validates the Origin header for CSRF protection
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// If no allowed origins configured (dev mode), allow but log warning
	if len(h.allowedOrigins) == 0 {
		if origin != "" {
			h.logger.Warn("WebSocket connection accepted without origin validation (configure allowed_origins for production)",
				"origin", origin,
				"remoteAddr", r.RemoteAddr,
			)
		}
		return true
	}

	// No origin header (same-origin request or non-browser client)
	if origin == "" {
		return true
	}

	// Check against allowed origins
	for _, allowed := range h.allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	h.logger.Warn("WebSocket connection rejected: origin not allowed",
		"origin", origin,
		"allowedOrigins", h.allowedOrigins,
		"remoteAddr", r.RemoteAddr,
	)
	return false
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("WebSocket hub shutting down")
			h.closeAllClients()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.allClients[client] = true
			if client.podID != "" {
				if h.clients[client.podID] == nil {
					h.clients[client.podID] = make(map[*Client]bool)
				}
				h.clients[client.podID][client] = true
			}
			h.mu.Unlock()
			h.logger.Debug("Client registered",
				"podId", client.podID,
				"sessionId", client.sessionID,
				"total", len(h.allClients),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.allClients[client]; ok {
				delete(h.allClients, client)
				if client.podID != "" {
					delete(h.clients[client.podID], client)
					if len(h.clients[client.podID]) == 0 {
						delete(h.clients, client.podID)
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debug("Client unregistered",
				"podId", client.podID,
				"total", len(h.allClients),
			)

		case msg := <-h.broadcast:
			h.broadcastMessage(msg)
		}
	}
}

// closeAllClients closes all connected client connections
func (h *Hub) closeAllClients() {
	h.mu.Lock()
	for client := range h.allClients {
		close(client.send)
	}
	h.mu.Unlock()
}

// Shutdown waits for all client goroutines to finish
// Call this after Run() returns to ensure graceful shutdown
func (h *Hub) Shutdown(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		h.logger.Info("WebSocket hub shutdown complete")
		return nil
	case <-time.After(timeout):
		h.logger.Warn("WebSocket hub shutdown timed out", "timeout", timeout)
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}

// broadcastMessage sends a message to appropriate clients
func (h *Hub) broadcastMessage(msg *BroadcastMessage) {
	data, err := json.Marshal(events.WebSocketMessage{
		Type:    msg.Type,
		Payload: mustMarshal(msg.Payload),
	})
	if err != nil {
		h.logger.Error("Failed to marshal broadcast message", "error", err)
		return
	}

	var stale []*Client

	h.mu.RLock()
	var targets map[*Client]bool

	if msg.PodID != "" {
		// Send to specific pod
		targets = h.clients[msg.PodID]
	} else {
		// Send to all clients
		targets = h.allClients
	}

	for client := range targets {
		// Filter by session if specified
		if msg.SessionID != "" && client.sessionID != msg.SessionID {
			continue
		}

		select {
		case client.send <- data:
		default:
			stale = append(stale, client)
		}
	}
	h.mu.RUnlock()

	// Remove stale clients outside the read lock to avoid map mutation under RLock
	if len(stale) > 0 {
		h.mu.Lock()
		for _, client := range stale {
			if _, ok := h.allClients[client]; ok {
				close(client.send)
				delete(h.allClients, client)
				if client.podID != "" {
					delete(h.clients[client.podID], client)
					if len(h.clients[client.podID]) == 0 {
						delete(h.clients, client.podID)
					}
				}
			}
		}
		h.mu.Unlock()
	}
}

// Broadcast sends a message to clients
func (h *Hub) Broadcast(msg *BroadcastMessage) {
	select {
	case h.broadcast <- msg:
	default:
		h.logger.Warn("Broadcast channel full, dropping message")
	}
}

// BroadcastCheckpoint broadcasts a checkpoint update
func (h *Hub) BroadcastCheckpoint(update *events.CheckpointUpdate) {
	h.Broadcast(&BroadcastMessage{
		PodID:     update.PodID,
		SessionID: update.SessionID,
		Type:      "checkpoint",
		Payload:   update,
	})
}

// BroadcastSession broadcasts a session event
func (h *Hub) BroadcastSession(event *events.SessionEvent) {
	h.Broadcast(&BroadcastMessage{
		PodID:     event.PodID,
		SessionID: event.SessionID,
		Type:      "session",
		Payload:   event,
	})
}

// BroadcastGrade broadcasts a grade update
func (h *Hub) BroadcastGrade(update *events.GradeUpdate) {
	h.Broadcast(&BroadcastMessage{
		SessionID: update.SessionID,
		Type:      "grade",
		Payload:   update,
	})
}

// BroadcastAssessmentUpdate broadcasts an assessment update (Packet Tracer-style)
func (h *Hub) BroadcastAssessmentUpdate(update *models.AssessmentUpdate) {
	if update == nil {
		return
	}

	h.Broadcast(&BroadcastMessage{
		SessionID: update.SessionID,
		Type:      "assessment",
		Payload:   update,
	})
}

// PodProvisioningEvent represents a provisioning status update for WebSocket broadcast
type PodProvisioningEvent struct {
	PodID     string `json:"podId"`
	OwnerID   string `json:"ownerId"`
	Status    string `json:"status"`
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	Progress  int    `json:"progress"`
	VMName    string `json:"vmName,omitempty"`
	VMStatus  string `json:"vmStatus,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"requestId"`
}

// BroadcastPodProvisioning broadcasts a pod provisioning status update
func (h *Hub) BroadcastPodProvisioning(event *PodProvisioningEvent) {
	if event == nil {
		return
	}

	h.Broadcast(&BroadcastMessage{
		PodID:   event.PodID,
		Type:    "pod_provisioning",
		Payload: event,
	})
}

// BroadcastHintNudge broadcasts a hint nudge to a specific session
func (h *Hub) BroadcastHintNudge(sessionID, podID string, payload any) {
	h.Broadcast(&BroadcastMessage{
		PodID:     podID,
		SessionID: sessionID,
		Type:      "hint_nudge",
		Payload:   payload,
	})
}

// BroadcastMonitoringEvent broadcasts a new event to the monitoring dashboard
// This is broadcast to all clients (no pod/session filter) for admin monitoring
func (h *Hub) BroadcastMonitoringEvent(event *events.MonitoringEvent) {
	if event == nil {
		return
	}

	h.Broadcast(&BroadcastMessage{
		Type:    "monitoring_event",
		Payload: event,
	})
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.allClients)
}

// PodClientCount returns the number of clients subscribed to a pod
func (h *Hub) PodClientCount(podID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[podID])
}

// ServeWS handles WebSocket connection requests
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, podID, sessionID, userID string) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:       h,
		conn:      conn,
		send:      make(chan []byte, sendBufferSize),
		podID:     podID,
		sessionID: sessionID,
		userID:    userID,
	}

	// Reserve goroutine slots BEFORE sending to register so Shutdown can't
	// race past Wait() while pumps are still being spawned. If register
	// processing triggers a fast shutdown (ctx cancel + closeAllClients),
	// pumps still hold the WaitGroup and Shutdown will block on them.
	h.wg.Add(2)

	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.wg.Done()
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Debug("WebSocket read error", "error", err)
			}
			break
		}
		// We don't process incoming messages for now, just keep connection alive
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		c.hub.wg.Done()
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// mustMarshal marshals to JSON or returns empty object
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}
