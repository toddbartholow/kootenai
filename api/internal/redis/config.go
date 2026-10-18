// Package redis provides Redis client configuration and initialization
package redis

import "time"

// Config holds Redis connection configuration
type Config struct {
	// Host is the Redis server hostname
	Host string `yaml:"host"`

	// Port is the Redis server port
	Port int `yaml:"port"`

	// Password for Redis authentication (empty for no auth)
	Password string `yaml:"password"`

	// Database selects the Redis database (0-15)
	Database int `yaml:"database"`

	// MaxPoolSize is the maximum number of socket connections
	MaxPoolSize int `yaml:"max_pool_size"`

	// MinIdleConns is the minimum number of idle connections
	MinIdleConns int `yaml:"min_idle_conns"`

	// DialTimeout is the timeout for establishing new connections
	DialTimeout time.Duration `yaml:"dial_timeout"`

	// ReadTimeout is the timeout for socket reads
	ReadTimeout time.Duration `yaml:"read_timeout"`

	// WriteTimeout is the timeout for socket writes
	WriteTimeout time.Duration `yaml:"write_timeout"`

	// PoolTimeout is the timeout for getting a connection from the pool
	PoolTimeout time.Duration `yaml:"pool_timeout"`

	// Enabled controls whether Redis integration is active
	Enabled bool `yaml:"enabled"`

	// TLS enables TLS/SSL connections
	TLS bool `yaml:"tls"`

	// KeyPrefix is prepended to all Redis keys for namespacing
	KeyPrefix string `yaml:"key_prefix"`
}

// DefaultConfig returns a Config with sensible defaults for development
func DefaultConfig() Config {
	return Config{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		Database:     0,
		MaxPoolSize:  10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		Enabled:      false,
		TLS:          false,
		KeyPrefix:    "labctl:",
	}
}

// Key prefixes for different data types
const (
	// Session keys
	PrefixSession     = "session:"
	PrefixUserSession = "user_session:"

	// Cache keys
	PrefixCache         = "cache:"
	PrefixTemplateCache = "cache:template:"
	PrefixPodCache      = "cache:pod:"
	PrefixUserCache     = "cache:user:"

	// Lock keys
	PrefixLock    = "lock:"
	PrefixPodLock = "lock:pod:"

	// Rate limiting keys
	PrefixRateLimit = "ratelimit:"

	// Task queue keys
	PrefixTask      = "task:"
	PrefixTaskQueue = "task:queue:"

	// Pub/Sub channels
	ChannelPodStatus        = "pubsub:pod:status"
	ChannelCheckpointUpdate = "pubsub:checkpoint:update"
	ChannelGradeSync        = "pubsub:grade:sync"
	ChannelSessionUpdate    = "pubsub:session:update"
)

// Default TTL values
const (
	DefaultSessionTTL     = 24 * time.Hour
	DefaultCacheTTL       = 5 * time.Minute
	DefaultTemplateTTL    = 1 * time.Hour
	DefaultPodStatusTTL   = 30 * time.Second
	DefaultLockTTL        = 30 * time.Second
	DefaultRateLimitTTL   = 1 * time.Minute
	DefaultTaskVisibility = 5 * time.Minute
)
