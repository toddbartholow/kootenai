//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// postgresContainer resolves which Postgres container the db: targets should
// talk to. The production stack (deploy/docker-compose.yml) names it
// kootenai-postgres; the local dev stack (docker-compose.dev.yml) names it
// labctl-postgres. These targets hardcoded the production name, so every
// `mage db:*` after a `mage dev:up` failed with "No such container" -- which
// is the sequence the README documents. test_integration.go and test_e2e.go
// already probe for both; this brings db.go in line.
//
// POSTGRES_CONTAINER overrides the probe for anything named differently.
func postgresContainer() string {
	if name := os.Getenv("POSTGRES_CONTAINER"); name != "" {
		return name
	}
	for _, name := range []string{"kootenai-postgres", "labctl-postgres"} {
		out, err := sh.Output("docker", "ps", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}")
		if err == nil && strings.TrimSpace(out) == name {
			return name
		}
	}
	// Nothing running: name the production container so the error message
	// matches what a deploy-time failure would say.
	return "kootenai-postgres"
}

// DB namespace contains database targets
type DB mg.Namespace

// Migrate runs all pending migrations on the local database
func (DB) Migrate() error {
	color.Cyan("Running database migrations...")
	cfg := GetConfig()

	// Ensure schema_migrations tracking table exists
	initSQL := `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`
	initCmd := fmt.Sprintf("cat <<'EOSQL' | docker exec -i %s psql -U %s -d %s\n%s\nEOSQL",
		postgresContainer(), cfg.Database.User, cfg.Database.Name, initSQL)
	if err := sh.Run("bash", "-c", initCmd); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get already-applied migrations
	appliedCmd := fmt.Sprintf("docker exec %s psql -U %s -d %s -t -A -c \"SELECT version FROM schema_migrations\"",
		postgresContainer(), cfg.Database.User, cfg.Database.Name)
	appliedOut, err := sh.Output("bash", "-c", appliedCmd)
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	applied := make(map[string]bool)
	for _, v := range strings.Split(strings.TrimSpace(appliedOut), "\n") {
		if v != "" {
			applied[v] = true
		}
	}

	// Get migration files
	migrations, err := getMigrationFiles(cfg.MigrationsDir)
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	if len(migrations) == 0 {
		color.Yellow("No migration files found")
		return nil
	}

	// Run each pending migration via Docker
	pending := 0
	for _, migration := range migrations {
		version := filepath.Base(migration)
		if applied[version] {
			continue
		}
		pending++
		color.Yellow("Applying: %s", version)

		// Read migration file and extract UP section
		content, err := os.ReadFile(migration)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migration, err)
		}

		upSQL := extractUpSection(string(content))
		if upSQL == "" {
			color.Yellow("Skipping (no UP section)")
			continue
		}

		// Run migration and record it in schema_migrations
		fullSQL := fmt.Sprintf("%s\nINSERT INTO schema_migrations (version) VALUES ('%s');",
			upSQL, strings.ReplaceAll(version, "'", "''"))
		cmd := fmt.Sprintf("cat <<'EOSQL' | docker exec -i %s psql -U %s -d %s\n%s\nEOSQL",
			postgresContainer(), cfg.Database.User, cfg.Database.Name, fullSQL)

		if err := sh.Run("bash", "-c", cmd); err != nil {
			return fmt.Errorf("migration %s failed: %w", version, err)
		}
	}

	if pending == 0 {
		color.Green("All migrations already applied")
	} else {
		color.Green("%d migration(s) applied", pending)
	}
	return nil
}

// MigrateRemote runs migrations on the infra VM database
func (DB) MigrateRemote() error {
	color.Cyan("Running migrations on infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Get migration files
	migrations, err := getMigrationFiles(cfg.MigrationsDir)
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	for _, migration := range migrations {
		color.Yellow("Applying: %s", filepath.Base(migration))

		// Read migration file
		content, err := os.ReadFile(migration)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migration, err)
		}

		upSQL := extractUpSection(string(content))
		if upSQL == "" {
			color.Yellow("Skipping (no UP section)")
			continue
		}

		// Run via docker on remote
		cmd := fmt.Sprintf("cat <<'EOSQL' | docker exec -i %s psql -U %s -d %s\n%s\nEOSQL",
			postgresContainer(), cfg.Database.User, cfg.Database.Name, upSQL)
		if _, err := client.Run(cmd); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	color.Green("Remote migrations completed")
	return nil
}

// Status shows migration status
func (DB) Status() error {
	color.Cyan("Migration status:")
	cfg := GetConfig()

	migrations, err := getMigrationFiles(cfg.MigrationsDir)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		fmt.Printf("  %s\n", filepath.Base(m))
	}

	fmt.Printf("\nTotal: %d migration files\n", len(migrations))
	return nil
}

// Reset drops and recreates the local database
func (DB) Reset() error {
	color.Cyan("Resetting local database...")
	cfg := GetConfig()

	// Drop and recreate database
	cmds := []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS %s", cfg.Database.Name),
		fmt.Sprintf("CREATE DATABASE %s OWNER %s", cfg.Database.Name, cfg.Database.User),
	}

	for _, sql := range cmds {
		cmd := fmt.Sprintf("docker exec %s psql -U %s -c '%s'",
			postgresContainer(), cfg.Database.User, sql)
		if err := sh.Run("bash", "-c", cmd); err != nil {
			color.Yellow("Warning: %v", err)
		}
	}

	// Run migrations
	mg.Deps(DB.Migrate)

	color.Green("Database reset complete")
	return nil
}

// Seed inserts seed data into the local database.
// Runs all .sql files from the seeds directory, plus legacy seed migrations.
func (DB) Seed() error {
	color.Cyan("Seeding database...")
	cfg := GetConfig()

	// First run legacy seed migration (005_seed_data.sql) for backwards compat
	legacySeedFile := filepath.Join(cfg.MigrationsDir, "005_seed_data.sql")
	if _, err := os.Stat(legacySeedFile); err == nil {
		color.Yellow("Running legacy seed: 005_seed_data.sql")
		content, err := os.ReadFile(legacySeedFile)
		if err != nil {
			return err
		}
		upSQL := extractUpSection(string(content))
		cmd := fmt.Sprintf("echo '%s' | docker exec -i %s psql -U %s -d %s",
			strings.ReplaceAll(upSQL, "'", "'\"'\"'"),
			postgresContainer(), cfg.Database.User, cfg.Database.Name)
		if err := sh.Run("bash", "-c", cmd); err != nil {
			return fmt.Errorf("legacy seeding failed: %w", err)
		}
	}

	// Then run any seed files from the seeds directory
	seedsDir := filepath.Join(filepath.Dir(cfg.MigrationsDir), "seeds")
	seedFiles, err := getMigrationFiles(seedsDir)
	if err != nil {
		// Seeds directory may not have .sql files yet — that's fine
		color.Green("Database seeded (no standalone seed files)")
		return nil
	}

	for _, seedFile := range seedFiles {
		color.Yellow("Running seed: %s", filepath.Base(seedFile))
		content, err := os.ReadFile(seedFile)
		if err != nil {
			return fmt.Errorf("failed to read seed %s: %w", seedFile, err)
		}
		cmd := fmt.Sprintf("echo '%s' | docker exec -i %s psql -U %s -d %s",
			strings.ReplaceAll(string(content), "'", "'\"'\"'"),
			postgresContainer(), cfg.Database.User, cfg.Database.Name)
		if err := sh.Run("bash", "-c", cmd); err != nil {
			return fmt.Errorf("seed %s failed: %w", filepath.Base(seedFile), err)
		}
	}

	color.Green("Database seeded")
	return nil
}

// Psql opens a psql shell to the local database
func (DB) Psql() error {
	cfg := GetConfig()
	return sh.RunV("docker", "exec", "-it", postgresContainer(),
		"psql", "-U", cfg.Database.User, "-d", cfg.Database.Name)
}

// getMigrationFiles returns sorted list of migration files
func getMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var migrations []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			migrations = append(migrations, filepath.Join(dir, e.Name()))
		}
	}

	sort.Strings(migrations)
	return migrations, nil
}

// extractUpSection extracts the UP migration section from SQL file
func extractUpSection(content string) string {
	// Look for +migrate Up marker
	upMarker := "+migrate Up"
	downMarker := "+migrate Down"

	upIdx := strings.Index(content, upMarker)
	if upIdx == -1 {
		// No marker, return full content (might be a simple migration)
		return content
	}

	// Get content after UP marker
	content = content[upIdx+len(upMarker):]

	// Find DOWN marker
	downIdx := strings.Index(content, downMarker)
	if downIdx != -1 {
		content = content[:downIdx]
	}

	return strings.TrimSpace(content)
}
