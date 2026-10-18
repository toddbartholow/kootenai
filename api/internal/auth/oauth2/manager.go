package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// StateStore defines the interface for storing OAuth2 state
type StateStore interface {
	// Set stores a state value with expiration
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	// Get retrieves a state value
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete removes a state value
	Delete(ctx context.Context, key string) error
}

// OAuthState holds the state data for an OAuth2 flow
type OAuthState struct {
	Provider    string    `json:"provider"`
	Nonce       string    `json:"nonce"`
	RedirectURL string    `json:"redirectUrl,omitempty"` // Where to redirect after login
	CreatedAt   time.Time `json:"createdAt"`
}

// Manager handles OAuth2/OIDC provider management
type Manager struct {
	providers   map[string]Provider
	stateStore  StateStore
	logger      *slog.Logger
	mu          sync.RWMutex
	stateExpiry time.Duration
}

// ManagerConfig holds configuration for the OAuth2 manager
type ManagerConfig struct {
	StateExpiry time.Duration
}

// DefaultManagerConfig returns default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		StateExpiry: 10 * time.Minute,
	}
}

// NewManager creates a new OAuth2 manager
func NewManager(stateStore StateStore, logger *slog.Logger, cfg ManagerConfig) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.StateExpiry == 0 {
		cfg.StateExpiry = 10 * time.Minute
	}

	return &Manager{
		providers:   make(map[string]Provider),
		stateStore:  stateStore,
		logger:      logger,
		stateExpiry: cfg.StateExpiry,
	}
}

// RegisterProvider registers an OAuth2 provider
func (m *Manager) RegisterProvider(id string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[id] = provider
	m.logger.Info("Registered OAuth2 provider",
		slog.String("id", id),
		slog.String("type", string(provider.Type())),
		slog.String("name", provider.Name()),
	)
}

// GetProvider returns a registered provider by ID
func (m *Manager) GetProvider(id string) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[id]
	return p, ok
}

// ListProviders returns all registered providers
func (m *Manager) ListProviders() []ProviderInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var providers []ProviderInfo
	for id, p := range m.providers {
		providers = append(providers, ProviderInfo{
			ID:   id,
			Type: p.Type(),
			Name: p.Name(),
		})
	}
	return providers
}

// ProviderInfo contains basic provider information for the frontend
type ProviderInfo struct {
	ID   string       `json:"id"`
	Type ProviderType `json:"type"`
	Name string       `json:"name"`
}

// InitiateLogin starts the OAuth2 login flow
func (m *Manager) InitiateLogin(ctx context.Context, providerID, redirectURL string) (authURL string, err error) {
	provider, ok := m.GetProvider(providerID)
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerID)
	}

	// Generate state and nonce
	state, err := GenerateState()
	if err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}

	nonce, err := GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	// Store state data
	stateData := OAuthState{
		Provider:    providerID,
		Nonce:       nonce,
		RedirectURL: redirectURL,
		CreatedAt:   time.Now(),
	}

	stateBytes, err := json.Marshal(stateData)
	if err != nil {
		return "", fmt.Errorf("marshaling state: %w", err)
	}

	// Store state with key prefix
	stateKey := "oauth2:state:" + state
	if err := m.stateStore.Set(ctx, stateKey, stateBytes, m.stateExpiry); err != nil {
		return "", fmt.Errorf("storing state: %w", err)
	}

	// Generate auth URL
	authURL = provider.AuthURL(state, nonce)
	if authURL == "" {
		return "", errors.New("failed to generate auth URL")
	}

	m.logger.Debug("Initiated OAuth2 login",
		slog.String("provider", providerID),
		slog.String("state", state[:8]+"..."), // Log truncated for security
	)

	return authURL, nil
}

// CompleteLogin completes the OAuth2 login flow
func (m *Manager) CompleteLogin(ctx context.Context, state, code string) (*LoginResult, error) {
	// Retrieve and validate state
	stateKey := "oauth2:state:" + state
	stateBytes, err := m.stateStore.Get(ctx, stateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired state: %w", err)
	}

	// Delete state immediately to prevent replay
	if err := m.stateStore.Delete(ctx, stateKey); err != nil {
		m.logger.Warn("Failed to delete OAuth2 state", slog.String("error", err.Error()))
	}

	var stateData OAuthState
	if err := json.Unmarshal(stateBytes, &stateData); err != nil {
		return nil, fmt.Errorf("invalid state data: %w", err)
	}

	// Get provider
	provider, ok := m.GetProvider(stateData.Provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", stateData.Provider)
	}

	// Exchange code for tokens
	tokens, err := provider.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Get user info
	var userInfo *UserInfo

	// Try ID token first if available (OIDC)
	if tokens.IDToken != "" {
		userInfo, err = provider.ValidateIDToken(ctx, tokens.IDToken, stateData.Nonce)
		if err != nil {
			m.logger.Debug("ID token validation failed, falling back to userinfo",
				slog.String("error", err.Error()),
			)
		}
	}

	// Fall back to userinfo endpoint
	if userInfo == nil {
		userInfo, err = provider.UserInfo(ctx, tokens.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to get user info: %w", err)
		}
	}

	m.logger.Info("OAuth2 login completed",
		slog.String("provider", stateData.Provider),
		slog.String("email", userInfo.Email),
	)

	return &LoginResult{
		UserInfo:    userInfo,
		Tokens:      tokens,
		Provider:    stateData.Provider,
		RedirectURL: stateData.RedirectURL,
	}, nil
}

// LoginResult contains the result of a successful OAuth2 login
type LoginResult struct {
	UserInfo    *UserInfo
	Tokens      *TokenResponse
	Provider    string
	RedirectURL string
}

// RefreshAccessToken refreshes an OAuth2 access token
func (m *Manager) RefreshAccessToken(ctx context.Context, providerID, refreshToken string) (*TokenResponse, error) {
	provider, ok := m.GetProvider(providerID)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}

	return provider.RefreshToken(ctx, refreshToken)
}

// =============================================================================
// In-Memory State Store (for development/testing)
// =============================================================================

// MemoryStateStore is an in-memory implementation of StateStore
type MemoryStateStore struct {
	data   map[string]storeEntry
	mu     sync.RWMutex
	stopCh chan struct{}
}

type storeEntry struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryStateStore creates a new in-memory state store
func NewMemoryStateStore() *MemoryStateStore {
	s := &MemoryStateStore{
		data:   make(map[string]storeEntry),
		stopCh: make(chan struct{}),
	}
	// Start cleanup goroutine
	go s.cleanup()
	return s
}

// Close stops the cleanup goroutine.
func (s *MemoryStateStore) Close() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
}

func (s *MemoryStateStore) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = storeEntry{
		value:     value,
		expiresAt: time.Now().Add(expiration),
	}
	return nil
}

func (s *MemoryStateStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok {
		return nil, errors.New("key not found")
	}
	if time.Now().After(entry.expiresAt) {
		return nil, errors.New("key expired")
	}
	return entry.value, nil
}

func (s *MemoryStateStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *MemoryStateStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for key, entry := range s.data {
				if now.After(entry.expiresAt) {
					delete(s.data, key)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// =============================================================================
// Redis State Store Adapter
// =============================================================================

// RedisClient defines the interface for Redis operations needed by OAuth2
type RedisClient interface {
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// RedisStateStore wraps a Redis client as a StateStore
type RedisStateStore struct {
	client RedisClient
	prefix string
}

// NewRedisStateStore creates a Redis-backed state store
func NewRedisStateStore(client RedisClient, prefix string) *RedisStateStore {
	if prefix == "" {
		prefix = "oauth2:"
	}
	return &RedisStateStore{
		client: client,
		prefix: prefix,
	}
}

func (s *RedisStateStore) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	return s.client.Set(ctx, s.prefix+key, value, expiration)
}

func (s *RedisStateStore) Get(ctx context.Context, key string) ([]byte, error) {
	return s.client.Get(ctx, s.prefix+key)
}

func (s *RedisStateStore) Delete(ctx context.Context, key string) error {
	return s.client.Delete(ctx, s.prefix+key)
}
