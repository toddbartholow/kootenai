package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionData represents a user session stored in Redis
type SessionData struct {
	UserID       string            `json:"user_id"`
	Username     string            `json:"username"`
	Email        string            `json:"email,omitempty"`
	Roles        []string          `json:"roles"`
	ExternalID   string            `json:"external_id,omitempty"`
	CanvasUserID string            `json:"canvas_user_id,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
	LastAccess   time.Time         `json:"last_access"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// WebSocketSession represents an active WebSocket connection
type WebSocketSession struct {
	ConnectionID string    `json:"connection_id"`
	UserID       string    `json:"user_id"`
	PodID        string    `json:"pod_id,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastPing     time.Time `json:"last_ping"`
	ServerNode   string    `json:"server_node,omitempty"`
}

// SessionStore manages user sessions in Redis
type SessionStore struct {
	client *Client
	ttl    time.Duration
}

// NewSessionStore creates a new session store
func NewSessionStore(client *Client, ttl time.Duration) *SessionStore {
	if ttl == 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionStore{
		client: client,
		ttl:    ttl,
	}
}

// Create stores a new session and returns the session ID
func (s *SessionStore) Create(ctx context.Context, sessionID string, data *SessionData) error {
	data.CreatedAt = time.Now()
	data.LastAccess = time.Now()
	if data.ExpiresAt.IsZero() {
		data.ExpiresAt = time.Now().Add(s.ttl)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := PrefixSession + sessionID
	if err := s.client.Set(ctx, key, jsonData, s.ttl); err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	// Also track sessions by user ID for listing/invalidation
	userSessionsKey := PrefixUserSession + data.UserID
	if err := s.client.SAdd(ctx, userSessionsKey, sessionID); err != nil {
		s.client.logger.Warn("failed to track user session",
			"user_id", data.UserID,
			"error", err,
		)
	}

	return nil
}

// Get retrieves a session by ID
func (s *SessionStore) Get(ctx context.Context, sessionID string) (*SessionData, error) {
	key := PrefixSession + sessionID
	data, err := s.client.GetBytes(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &session, nil
}

// Touch updates the last access time and extends TTL
func (s *SessionStore) Touch(ctx context.Context, sessionID string) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("session not found")
	}

	session.LastAccess = time.Now()
	session.ExpiresAt = time.Now().Add(s.ttl)

	jsonData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := PrefixSession + sessionID
	return s.client.Set(ctx, key, jsonData, s.ttl)
}

// Delete removes a session
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	// Get session to find user ID
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	key := PrefixSession + sessionID
	if err := s.client.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove from user's session list
	if session != nil {
		userSessionsKey := PrefixUserSession + session.UserID
		_ = s.client.SRem(ctx, userSessionsKey, sessionID)
	}

	return nil
}

// DeleteByUserID removes all sessions for a user
func (s *SessionStore) DeleteByUserID(ctx context.Context, userID string) error {
	userSessionsKey := PrefixUserSession + userID
	sessionIDs, err := s.client.SMembers(ctx, userSessionsKey)
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	var firstErr error
	for _, sessionID := range sessionIDs {
		key := PrefixSession + sessionID
		if err := s.client.Delete(ctx, key); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("deleting session %s: %w", sessionID, err)
		}
	}

	if err := s.client.Delete(ctx, userSessionsKey); err != nil {
		return fmt.Errorf("deleting user session index: %w", err)
	}
	return firstErr
}

// ListByUserID returns all active sessions for a user
func (s *SessionStore) ListByUserID(ctx context.Context, userID string) ([]*SessionData, error) {
	userSessionsKey := PrefixUserSession + userID
	sessionIDs, err := s.client.SMembers(ctx, userSessionsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	var sessions []*SessionData
	var expiredIDs []string

	for _, sessionID := range sessionIDs {
		session, err := s.Get(ctx, sessionID)
		if err != nil {
			continue
		}
		if session == nil {
			expiredIDs = append(expiredIDs, sessionID)
			continue
		}
		sessions = append(sessions, session)
	}

	// Clean up expired session references
	if len(expiredIDs) > 0 {
		for _, id := range expiredIDs {
			_ = s.client.SRem(ctx, userSessionsKey, id)
		}
	}

	return sessions, nil
}

// Exists checks if a session exists
func (s *SessionStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	key := PrefixSession + sessionID
	return s.client.Exists(ctx, key)
}

// Update modifies session data
func (s *SessionStore) Update(ctx context.Context, sessionID string, updateFn func(*SessionData)) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("session not found")
	}

	updateFn(session)
	session.LastAccess = time.Now()

	jsonData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := PrefixSession + sessionID
	ttl, _ := s.client.TTL(ctx, key)
	if ttl <= 0 {
		ttl = s.ttl
	}

	return s.client.Set(ctx, key, jsonData, ttl)
}

// --- WebSocket Session Management ---

// WebSocketStore manages WebSocket connection sessions
type WebSocketStore struct {
	client *Client
	ttl    time.Duration
}

// NewWebSocketStore creates a new WebSocket session store
func NewWebSocketStore(client *Client, ttl time.Duration) *WebSocketStore {
	if ttl == 0 {
		ttl = 10 * time.Minute // WebSocket sessions expire faster without pings
	}
	return &WebSocketStore{
		client: client,
		ttl:    ttl,
	}
}

// Register registers a new WebSocket connection
func (w *WebSocketStore) Register(ctx context.Context, session *WebSocketSession) error {
	session.ConnectedAt = time.Now()
	session.LastPing = time.Now()

	jsonData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal websocket session: %w", err)
	}

	// Store connection by ID
	connKey := "ws:conn:" + session.ConnectionID
	if err := w.client.Set(ctx, connKey, jsonData, w.ttl); err != nil {
		return fmt.Errorf("failed to store websocket session: %w", err)
	}

	// Track by pod ID if present
	if session.PodID != "" {
		podKey := "ws:pod:" + session.PodID
		if err := w.client.SAdd(ctx, podKey, session.ConnectionID); err != nil {
			return err
		}
		_ = w.client.Expire(ctx, podKey, w.ttl)
	}

	// Track by user ID
	if session.UserID != "" {
		userKey := "ws:user:" + session.UserID
		if err := w.client.SAdd(ctx, userKey, session.ConnectionID); err != nil {
			return err
		}
		_ = w.client.Expire(ctx, userKey, w.ttl)
	}

	return nil
}

// Unregister removes a WebSocket connection
func (w *WebSocketStore) Unregister(ctx context.Context, connectionID string) error {
	// Get session first
	connKey := "ws:conn:" + connectionID
	data, err := w.client.GetBytes(ctx, connKey)
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get websocket session: %w", err)
	}

	if data != nil {
		var session WebSocketSession
		if err := json.Unmarshal(data, &session); err == nil {
			if session.PodID != "" {
				podKey := "ws:pod:" + session.PodID
				_ = w.client.SRem(ctx, podKey, connectionID)
			}
			if session.UserID != "" {
				userKey := "ws:user:" + session.UserID
				_ = w.client.SRem(ctx, userKey, connectionID)
			}
		}
	}

	return w.client.Delete(ctx, connKey)
}

// Ping updates the last ping time for a connection
func (w *WebSocketStore) Ping(ctx context.Context, connectionID string) error {
	connKey := "ws:conn:" + connectionID
	data, err := w.client.GetBytes(ctx, connKey)
	if err != nil {
		return err
	}

	var session WebSocketSession
	if err := json.Unmarshal(data, &session); err != nil {
		return err
	}

	session.LastPing = time.Now()
	jsonData, _ := json.Marshal(session)

	return w.client.Set(ctx, connKey, jsonData, w.ttl)
}

// GetByPodID returns all WebSocket connections for a pod
func (w *WebSocketStore) GetByPodID(ctx context.Context, podID string) ([]WebSocketSession, error) {
	podKey := "ws:pod:" + podID
	connIDs, err := w.client.SMembers(ctx, podKey)
	if err != nil {
		return nil, err
	}

	var sessions []WebSocketSession
	for _, connID := range connIDs {
		connKey := "ws:conn:" + connID
		data, err := w.client.GetBytes(ctx, connKey)
		if err != nil {
			continue
		}

		var session WebSocketSession
		if err := json.Unmarshal(data, &session); err == nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// GetByUserID returns all WebSocket connections for a user
func (w *WebSocketStore) GetByUserID(ctx context.Context, userID string) ([]WebSocketSession, error) {
	userKey := "ws:user:" + userID
	connIDs, err := w.client.SMembers(ctx, userKey)
	if err != nil {
		return nil, err
	}

	var sessions []WebSocketSession
	for _, connID := range connIDs {
		connKey := "ws:conn:" + connID
		data, err := w.client.GetBytes(ctx, connKey)
		if err != nil {
			continue
		}

		var session WebSocketSession
		if err := json.Unmarshal(data, &session); err == nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// ConnectionCount returns the number of active connections for a pod
func (w *WebSocketStore) ConnectionCount(ctx context.Context, podID string) (int64, error) {
	podKey := "ws:pod:" + podID
	return w.client.SCard(ctx, podKey)
}
