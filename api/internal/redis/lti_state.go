package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// LTIState represents the OIDC state stored during LTI launch
type LTIState struct {
	Nonce         string    `json:"nonce"`
	TargetLinkURI string    `json:"target_link_uri"`
	CreatedAt     time.Time `json:"created_at"`
}

// LTIStateStore manages LTI OIDC state in Redis
type LTIStateStore struct {
	client *Client
	logger *slog.Logger
	ttl    time.Duration
}

// NewLTIStateStore creates a new LTI state store
func NewLTIStateStore(client *Client, logger *slog.Logger) *LTIStateStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &LTIStateStore{
		client: client,
		logger: logger,
		ttl:    10 * time.Minute, // States expire after 10 minutes
	}
}

// ltiStateKey returns the Redis key for an LTI state
func (s *LTIStateStore) ltiStateKey(state string) string {
	return "lti:state:" + state
}

// Store saves an LTI state with automatic expiration
func (s *LTIStateStore) Store(ctx context.Context, state string, data LTIState) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling LTI state: %w", err)
	}

	if err := s.client.Set(ctx, s.ltiStateKey(state), jsonData, s.ttl); err != nil {
		return fmt.Errorf("storing LTI state in Redis: %w", err)
	}

	s.logger.Debug("stored LTI state",
		slog.String("state", state[:8]+"..."),
		slog.Duration("ttl", s.ttl),
	)

	return nil
}

// Get retrieves an LTI state by its key
// Returns nil, nil if the state does not exist (expired or never stored)
func (s *LTIStateStore) Get(ctx context.Context, state string) (*LTIState, error) {
	data, err := s.client.GetBytes(ctx, s.ltiStateKey(state))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting LTI state from Redis: %w", err)
	}

	var ltiState LTIState
	if err := json.Unmarshal(data, &ltiState); err != nil {
		return nil, fmt.Errorf("unmarshaling LTI state: %w", err)
	}

	return &ltiState, nil
}

// Delete removes an LTI state (called after successful validation)
func (s *LTIStateStore) Delete(ctx context.Context, state string) error {
	if err := s.client.Delete(ctx, s.ltiStateKey(state)); err != nil {
		return fmt.Errorf("deleting LTI state from Redis: %w", err)
	}

	s.logger.Debug("deleted LTI state",
		slog.String("state", state[:8]+"..."),
	)

	return nil
}

// Exists checks if an LTI state exists
func (s *LTIStateStore) Exists(ctx context.Context, state string) (bool, error) {
	return s.client.Exists(ctx, s.ltiStateKey(state))
}
