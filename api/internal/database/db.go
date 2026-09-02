// Package database provides PostgreSQL database connectivity and repositories
package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds database configuration
type Config struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// DefaultConfig returns default database configuration
// Note: Password is intentionally empty to force explicit configuration
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		User:            "labctl",
		Password:        "", // Must be set via environment variable or config file
		Database:        "labctl",
		SSLMode:         "prefer",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// Validate checks that the configuration is valid
func (c *Config) Validate() error {
	var errs []error

	if c.Host == "" {
		errs = append(errs, errors.New("database host cannot be empty"))
	}

	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("database port must be between 1 and 65535, got %d", c.Port))
	}

	if c.User == "" {
		errs = append(errs, errors.New("database user cannot be empty"))
	}

	if c.Database == "" {
		errs = append(errs, errors.New("database name cannot be empty"))
	}

	validSSLModes := map[string]bool{
		"disable": true, "allow": true, "prefer": true, "require": true, "verify-ca": true, "verify-full": true,
	}
	if c.SSLMode != "" && !validSSLModes[c.SSLMode] {
		errs = append(errs, fmt.Errorf("invalid ssl_mode: %s (must be one of: disable, allow, prefer, require, verify-ca, verify-full)", c.SSLMode))
	}

	if c.MaxOpenConns < 1 {
		errs = append(errs, errors.New("max_open_conns must be at least 1"))
	}

	if c.MaxIdleConns < 0 {
		errs = append(errs, errors.New("max_idle_conns cannot be negative"))
	}

	if c.MaxIdleConns > c.MaxOpenConns {
		errs = append(errs, errors.New("max_idle_conns cannot exceed max_open_conns"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("database config validation failed: %w", errors.Join(errs...))
	}

	return nil
}

// DB wraps the sql.DB connection pool
type DB struct {
	*sql.DB
	logger *slog.Logger
	config Config
}

// New creates a new database connection
func New(cfg Config, logger *slog.Logger) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if cfg.SSLMode == "disable" {
		logger.Warn("Database SSL is disabled — connections are unencrypted",
			"host", cfg.Host,
			"sslmode", cfg.SSLMode,
		)
	}

	logger.Info("Connected to PostgreSQL",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Database,
		"sslmode", cfg.SSLMode,
	)

	return &DB{
		DB:     db,
		logger: logger,
		config: cfg,
	}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	return db.DB.Close()
}

// Health checks database health
func (db *DB) Health(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rolling back transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
