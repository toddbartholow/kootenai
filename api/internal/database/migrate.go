// Package database provides PostgreSQL database connectivity and migrations.
//
// Note: The migration system is forward-only. There are no rollback/down migrations.
// Migration files are sorted lexicographically (e.g., 014b_ comes after 014_).
// Consider adopting goose or golang-migrate for up/down migration support.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MigrationRecord represents a record of an applied migration
type MigrationRecord struct {
	Version   string
	AppliedAt time.Time
}

// Migrate runs all pending database migrations
func (db *DB) Migrate(ctx context.Context) error {
	db.logger.Info("Starting database migrations")

	// Create migrations tracking table if it doesn't exist
	if err := db.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Get list of applied migrations
	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	appliedSet := make(map[string]bool)
	for _, m := range applied {
		appliedSet[m.Version] = true
	}

	// Get all migration files from embedded filesystem
	migrations, err := db.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("reading migration files: %w", err)
	}

	// Apply pending migrations
	appliedCount := 0
	for _, migration := range migrations {
		if appliedSet[migration] {
			db.logger.Debug("Migration already applied", "version", migration)
			continue
		}

		db.logger.Info("Applying migration", "version", migration)
		if err := db.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("applying migration %s: %w", migration, err)
		}
		appliedCount++
	}

	if appliedCount == 0 {
		db.logger.Info("Database is up to date, no migrations needed")
	} else {
		db.logger.Info("Migrations completed", "applied", appliedCount)
	}

	return nil
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func (db *DB) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

// getAppliedMigrations returns a list of all applied migrations
func (db *DB) getAppliedMigrations(ctx context.Context) ([]MigrationRecord, error) {
	query := `SELECT version, applied_at FROM schema_migrations ORDER BY version`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []MigrationRecord
	for rows.Next() {
		var m MigrationRecord
		if err := rows.Scan(&m.Version, &m.AppliedAt); err != nil {
			return nil, err
		}
		migrations = append(migrations, m)
	}

	return migrations, rows.Err()
}

// getMigrationFiles returns a sorted list of migration file names
func (db *DB) getMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".sql") {
			migrations = append(migrations, name)
		}
	}

	// Sort migrations by filename (001_, 002_, etc.)
	sort.Strings(migrations)
	return migrations, nil
}

// applyMigration applies a single migration file
func (db *DB) applyMigration(ctx context.Context, filename string) error {
	// Read migration content
	content, err := fs.ReadFile(migrationsFS, filepath.Join("migrations", filename))
	if err != nil {
		return fmt.Errorf("reading migration file: %w", err)
	}

	// Execute migration in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute the migration SQL
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("executing migration SQL: %w", err)
	}

	// Record the migration
	if err := db.recordMigration(ctx, tx, filename); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// recordMigration records a migration as applied
func (db *DB) recordMigration(ctx context.Context, tx *sql.Tx, version string) error {
	query := `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, NOW())`
	_, err := tx.ExecContext(ctx, query, version)
	return err
}

// MigrationStatus returns the current migration status
func (db *DB) MigrationStatus(ctx context.Context) ([]MigrationStatusEntry, error) {
	// Get all migration files
	allMigrations, err := db.getMigrationFiles()
	if err != nil {
		return nil, err
	}

	// Get applied migrations
	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]time.Time)
	for _, m := range applied {
		appliedMap[m.Version] = m.AppliedAt
	}

	// Build status list
	var status []MigrationStatusEntry
	for _, migration := range allMigrations {
		entry := MigrationStatusEntry{
			Version: migration,
			Applied: false,
		}
		if appliedAt, ok := appliedMap[migration]; ok {
			entry.Applied = true
			entry.AppliedAt = &appliedAt
		}
		status = append(status, entry)
	}

	return status, nil
}

// MigrationStatusEntry represents the status of a single migration
type MigrationStatusEntry struct {
	Version   string
	Applied   bool
	AppliedAt *time.Time
}

// PrintMigrationStatus logs the current migration status
func (db *DB) PrintMigrationStatus(ctx context.Context, logger *slog.Logger) error {
	status, err := db.MigrationStatus(ctx)
	if err != nil {
		return err
	}

	for _, entry := range status {
		if entry.Applied {
			logger.Info("Migration status",
				"version", entry.Version,
				"status", "applied",
				"applied_at", entry.AppliedAt.Format(time.RFC3339),
			)
		} else {
			logger.Info("Migration status",
				"version", entry.Version,
				"status", "pending",
			)
		}
	}

	return nil
}
