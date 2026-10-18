package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with application-specific functionality
type Client struct {
	rdb       *redis.Client
	config    Config
	logger    *slog.Logger
	keyPrefix string
}

// New creates a new Redis client with the given configuration
func New(cfg Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	opts := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.Database,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.MaxPoolSize,
		MinIdleConns: cfg.MinIdleConns,
		PoolTimeout:  cfg.PoolTimeout,
	}

	if cfg.TLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	rdb := redis.NewClient(opts)

	client := &Client{
		rdb:       rdb,
		config:    cfg,
		logger:    logger,
		keyPrefix: cfg.KeyPrefix,
	}

	return client, nil
}

// Connect verifies the connection to Redis
func (c *Client) Connect(ctx context.Context) error {
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	c.logger.Info("connected to Redis",
		slog.String("host", c.config.Host),
		slog.Int("port", c.config.Port),
		slog.Int("database", c.config.Database),
	)
	return nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks if Redis is reachable
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Client returns the underlying redis.Client for advanced operations
func (c *Client) Client() *redis.Client {
	return c.rdb
}

// Key returns a fully-qualified key with the configured prefix
func (c *Client) Key(parts ...string) string {
	key := c.keyPrefix
	for _, part := range parts {
		key += part
	}
	return key
}

// --- Basic Operations ---

// Set stores a value with an optional TTL
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.Key(key), value, ttl).Err()
}

// Get retrieves a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, c.Key(key)).Result()
}

// GetBytes retrieves a value as bytes
func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return c.rdb.Get(ctx, c.Key(key)).Bytes()
}

// Delete removes one or more keys
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = c.Key(k)
	}
	return c.rdb.Del(ctx, prefixedKeys...).Err()
}

// Exists checks if a key exists
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, c.Key(key)).Result()
	return n > 0, err
}

// Expire sets a TTL on an existing key
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, c.Key(key), ttl).Err()
}

// TTL returns the remaining time-to-live for a key
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, c.Key(key)).Result()
}

// --- Hash Operations ---

// HSet sets fields in a hash
func (c *Client) HSet(ctx context.Context, key string, values ...any) error {
	return c.rdb.HSet(ctx, c.Key(key), values...).Err()
}

// HGet retrieves a field from a hash
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.rdb.HGet(ctx, c.Key(key), field).Result()
}

// HGetAll retrieves all fields from a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, c.Key(key)).Result()
}

// HDel removes fields from a hash
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	return c.rdb.HDel(ctx, c.Key(key), fields...).Err()
}

// --- Increment/Decrement ---

// Incr increments a key by 1
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, c.Key(key)).Result()
}

// IncrBy increments a key by a specified amount
func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.rdb.IncrBy(ctx, c.Key(key), value).Result()
}

// Decr decrements a key by 1
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Decr(ctx, c.Key(key)).Result()
}

// --- List Operations ---

// LPush prepends values to a list
func (c *Client) LPush(ctx context.Context, key string, values ...any) error {
	return c.rdb.LPush(ctx, c.Key(key), values...).Err()
}

// RPush appends values to a list
func (c *Client) RPush(ctx context.Context, key string, values ...any) error {
	return c.rdb.RPush(ctx, c.Key(key), values...).Err()
}

// LPop removes and returns the first element
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	return c.rdb.LPop(ctx, c.Key(key)).Result()
}

// RPop removes and returns the last element
func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	return c.rdb.RPop(ctx, c.Key(key)).Result()
}

// LRange returns a range of elements from a list
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.rdb.LRange(ctx, c.Key(key), start, stop).Result()
}

// LLen returns the length of a list
func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	return c.rdb.LLen(ctx, c.Key(key)).Result()
}

// --- Set Operations ---

// SAdd adds members to a set
func (c *Client) SAdd(ctx context.Context, key string, members ...any) error {
	return c.rdb.SAdd(ctx, c.Key(key), members...).Err()
}

// SRem removes members from a set
func (c *Client) SRem(ctx context.Context, key string, members ...any) error {
	return c.rdb.SRem(ctx, c.Key(key), members...).Err()
}

// SMembers returns all members of a set
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.rdb.SMembers(ctx, c.Key(key)).Result()
}

// SIsMember checks if a value is a member of a set
func (c *Client) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return c.rdb.SIsMember(ctx, c.Key(key), member).Result()
}

// SCard returns the cardinality (size) of a set
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	return c.rdb.SCard(ctx, c.Key(key)).Result()
}

// --- Pub/Sub ---

// Publish publishes a message to a channel
func (c *Client) Publish(ctx context.Context, channel string, message any) error {
	return c.rdb.Publish(ctx, c.Key(channel), message).Err()
}

// Subscribe returns a PubSub for the given channels
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	prefixedChannels := make([]string, len(channels))
	for i, ch := range channels {
		prefixedChannels[i] = c.Key(ch)
	}
	return c.rdb.Subscribe(ctx, prefixedChannels...)
}

// --- Script Execution ---

// Eval executes a Lua script
func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = c.Key(k)
	}
	return c.rdb.Eval(ctx, script, prefixedKeys, args...)
}

// --- Health Check ---

// HealthCheck performs a comprehensive health check
func (c *Client) HealthCheck(ctx context.Context) error {
	// Check basic connectivity
	if err := c.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Check memory usage
	info, err := c.rdb.Info(ctx, "memory").Result()
	if err != nil {
		return fmt.Errorf("failed to get memory info: %w", err)
	}

	c.logger.Debug("redis health check passed",
		slog.String("memory_info", info[:min(len(info), 200)]),
	)

	return nil
}
