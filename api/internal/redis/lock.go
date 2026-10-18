package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Lock represents a distributed lock
type Lock struct {
	client *Client
	key    string
	token  string
	ttl    time.Duration
	logger *slog.Logger
}

// LockManager provides distributed locking functionality
type LockManager struct {
	client     *Client
	logger     *slog.Logger
	defaultTTL time.Duration
}

// NewLockManager creates a new lock manager
func NewLockManager(client *Client, logger *slog.Logger) *LockManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &LockManager{
		client:     client,
		logger:     logger,
		defaultTTL: DefaultLockTTL,
	}
}

// generateToken creates a unique token for lock ownership.
// Returns an error on random-source failure so callers can surface a 503
// instead of taking the process down.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand read: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Acquire attempts to acquire a lock with the given key
func (lm *LockManager) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	if ttl == 0 {
		ttl = lm.defaultTTL
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate lock token: %w", err)
	}
	fullKey := lm.client.Key(PrefixLock + key)

	// Try to set the lock with NX (only if not exists)
	ok, setErr := lm.client.rdb.SetNX(ctx, fullKey, token, ttl).Result()
	if setErr != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", setErr)
	}

	if !ok {
		return nil, ErrLockNotAcquired
	}

	lm.logger.Debug("lock acquired",
		slog.String("key", key),
		slog.Duration("ttl", ttl),
	)

	return &Lock{
		client: lm.client,
		key:    fullKey,
		token:  token,
		ttl:    ttl,
		logger: lm.logger,
	}, nil
}

// AcquireWithRetry attempts to acquire a lock with retries
func (lm *LockManager) AcquireWithRetry(ctx context.Context, key string, ttl time.Duration, maxAttempts int, retryDelay time.Duration) (*Lock, error) {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if retryDelay == 0 {
		retryDelay = 100 * time.Millisecond
	}

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		lock, err := lm.Acquire(ctx, key, ttl)
		if err == nil {
			return lock, nil
		}

		if !errors.Is(err, ErrLockNotAcquired) {
			return nil, err
		}

		lastErr = err

		// Wait before retry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
			// Exponential backoff
			retryDelay = retryDelay * 2
		}
	}

	return nil, fmt.Errorf("failed to acquire lock after %d attempts: %w", maxAttempts, lastErr)
}

// TryLock attempts to acquire a lock without blocking
func (lm *LockManager) TryLock(ctx context.Context, key string, ttl time.Duration) (*Lock, bool) {
	lock, err := lm.Acquire(ctx, key, ttl)
	if err != nil {
		return nil, false
	}
	return lock, true
}

// Release releases the lock
func (l *Lock) Release(ctx context.Context) error {
	// Lua script to ensure we only delete if we own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.client.rdb.Eval(ctx, script, []string{l.key}, l.token).Int64()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result == 0 {
		return ErrLockLost
	}

	l.logger.Debug("lock released", slog.String("key", l.key))
	return nil
}

// Extend extends the lock TTL
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	if ttl == 0 {
		ttl = l.ttl
	}

	// Lua script to extend only if we still own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.client.rdb.Eval(ctx, script, []string{l.key}, l.token, int64(ttl/time.Millisecond)).Int64()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result == 0 {
		return ErrLockLost
	}

	l.ttl = ttl
	l.logger.Debug("lock extended",
		slog.String("key", l.key),
		slog.Duration("ttl", ttl),
	)
	return nil
}

// IsHeld checks if we still hold the lock
func (l *Lock) IsHeld(ctx context.Context) bool {
	val, err := l.client.rdb.Get(ctx, l.key).Result()
	if err != nil {
		return false
	}
	return val == l.token
}

// Errors
var (
	ErrLockNotAcquired = errors.New("lock not acquired")
	ErrLockLost        = errors.New("lock was lost")
)

// --- Specialized Lock Types ---

// PodLock provides locking for pod operations
type PodLock struct {
	manager *LockManager
}

// NewPodLock creates a pod lock manager
func NewPodLock(client *Client, logger *slog.Logger) *PodLock {
	return &PodLock{
		manager: NewLockManager(client, logger),
	}
}

// LockPod acquires a lock for pod operations
func (pl *PodLock) LockPod(ctx context.Context, podID string, ttl time.Duration) (*Lock, error) {
	return pl.manager.Acquire(ctx, "pod:"+podID, ttl)
}

// LockPodWithRetry acquires a lock for pod operations with retries
func (pl *PodLock) LockPodWithRetry(ctx context.Context, podID string, ttl time.Duration, maxAttempts int) (*Lock, error) {
	return pl.manager.AcquireWithRetry(ctx, "pod:"+podID, ttl, maxAttempts, 200*time.Millisecond)
}

// TryLockPod attempts to lock a pod without blocking
func (pl *PodLock) TryLockPod(ctx context.Context, podID string, ttl time.Duration) (*Lock, bool) {
	return pl.manager.TryLock(ctx, "pod:"+podID, ttl)
}

// SessionLock provides locking for session operations
type SessionLock struct {
	manager *LockManager
}

// NewSessionLock creates a session lock manager
func NewSessionLock(client *Client, logger *slog.Logger) *SessionLock {
	return &SessionLock{
		manager: NewLockManager(client, logger),
	}
}

// LockSession acquires a lock for session operations
func (sl *SessionLock) LockSession(ctx context.Context, sessionID string, ttl time.Duration) (*Lock, error) {
	return sl.manager.Acquire(ctx, "session:"+sessionID, ttl)
}

// LockGradeSync acquires a lock for grade sync operations
func (sl *SessionLock) LockGradeSync(ctx context.Context, sessionID string, ttl time.Duration) (*Lock, error) {
	return sl.manager.Acquire(ctx, "grade_sync:"+sessionID, ttl)
}

// ResourceLock provides locking for resource allocation
type ResourceLock struct {
	manager *LockManager
}

// NewResourceLock creates a resource lock manager
func NewResourceLock(client *Client, logger *slog.Logger) *ResourceLock {
	return &ResourceLock{
		manager: NewLockManager(client, logger),
	}
}

// LockVLAN acquires a lock for VLAN allocation
func (rl *ResourceLock) LockVLAN(ctx context.Context, node string, ttl time.Duration) (*Lock, error) {
	return rl.manager.Acquire(ctx, "vlan:"+node, ttl)
}

// LockVM acquires a lock for VM operations
func (rl *ResourceLock) LockVM(ctx context.Context, vmID string, ttl time.Duration) (*Lock, error) {
	return rl.manager.Acquire(ctx, "vm:"+vmID, ttl)
}

// LockTemplate acquires a lock for template operations
func (rl *ResourceLock) LockTemplate(ctx context.Context, templateID string, ttl time.Duration) (*Lock, error) {
	return rl.manager.Acquire(ctx, "template:"+templateID, ttl)
}

// WithLock is a helper that acquires a lock, executes a function, and releases the lock
func WithLock(ctx context.Context, lm *LockManager, key string, ttl time.Duration, fn func() error) error {
	lock, err := lm.Acquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := lock.Release(ctx); releaseErr != nil {
			// Log but don't override the original error
			lm.logger.Warn("failed to release lock",
				slog.String("key", key),
				slog.Any("error", releaseErr),
			)
		}
	}()

	return fn()
}

// Semaphore provides a distributed counting semaphore
type Semaphore struct {
	client    *Client
	key       string
	maxTokens int64
	ttl       time.Duration
	logger    *slog.Logger
}

// NewSemaphore creates a new distributed semaphore
func NewSemaphore(client *Client, key string, maxTokens int64, ttl time.Duration, logger *slog.Logger) *Semaphore {
	if logger == nil {
		logger = slog.Default()
	}
	return &Semaphore{
		client:    client,
		key:       client.Key(PrefixLock + "sem:" + key),
		maxTokens: maxTokens,
		ttl:       ttl,
		logger:    logger,
	}
}

// Acquire attempts to acquire a semaphore token
func (s *Semaphore) Acquire(ctx context.Context) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate semaphore token: %w", err)
	}

	// Use a sorted set with scores as timestamps
	script := `
		local key = KEYS[1]
		local max = tonumber(ARGV[1])
		local token = ARGV[2]
		local now = tonumber(ARGV[3])
		local ttl = tonumber(ARGV[4])

		-- Remove expired entries
		redis.call("zremrangebyscore", key, "-inf", now - ttl * 1000)

		-- Check current count
		local count = redis.call("zcard", key)
		if count >= max then
			return 0
		end

		-- Add new token
		redis.call("zadd", key, now, token)
		redis.call("expire", key, ttl * 2)

		return 1
	`

	now := time.Now().UnixMilli()
	result, err := s.client.rdb.Eval(ctx, script, []string{s.key}, s.maxTokens, token, now, int64(s.ttl.Seconds())).Int64()
	if err != nil {
		return "", fmt.Errorf("semaphore acquire failed: %w", err)
	}

	if result == 0 {
		return "", ErrSemaphoreFull
	}

	return token, nil
}

// Release releases a semaphore token
func (s *Semaphore) Release(ctx context.Context, token string) error {
	result, err := s.client.rdb.ZRem(ctx, s.key, token).Result()
	if err != nil {
		return fmt.Errorf("semaphore release failed: %w", err)
	}

	if result == 0 {
		return errors.New("token not found in semaphore")
	}

	return nil
}

// Available returns the number of available tokens
func (s *Semaphore) Available(ctx context.Context) (int64, error) {
	// Clean up expired entries first
	now := time.Now().UnixMilli()
	expiry := now - s.ttl.Milliseconds()
	s.client.rdb.ZRemRangeByScore(ctx, s.key, "-inf", fmt.Sprintf("%d", expiry))

	count, err := s.client.rdb.ZCard(ctx, s.key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return s.maxTokens, nil
		}
		return 0, err
	}

	return s.maxTokens - count, nil
}

// ErrSemaphoreFull indicates no tokens available
var ErrSemaphoreFull = errors.New("semaphore is full")
