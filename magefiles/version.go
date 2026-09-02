//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fatih/color"
)

// VersionManifest represents the deployed versions on the infra VM
type VersionManifest struct {
	API            ComponentVersion      `json:"api"`
	Web            ComponentVersion      `json:"web"`
	DBSchema       DBSchemaVersion       `json:"db_schema"`
	Infrastructure InfrastructureVersion `json:"infrastructure"`
}

// ComponentVersion represents version info for a deployed component
type ComponentVersion struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildTime  string `json:"build_time"`
	DeployedAt string `json:"deployed_at"`
}

// DBSchemaVersion represents the database schema version
type DBSchemaVersion struct {
	Version        string `json:"version"`
	MigrationCount int    `json:"migration_count"`
}

// InfrastructureVersion represents versions of infrastructure services
type InfrastructureVersion struct {
	PostgreSQL string `json:"postgresql"`
	NATS       string `json:"nats"`
	Nginx      string `json:"nginx"`
	Redis      string `json:"redis"`
}

// APIVersionResponse matches the response from /version endpoint
type APIVersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// getRemoteAPIVersion fetches version info from the deployed API
func getRemoteAPIVersion(cfg *Config) (*APIVersionResponse, error) {
	url := fmt.Sprintf("http://%s:8080/version", cfg.InfraVM.Host)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var version APIVersionResponse
	if err := json.Unmarshal(body, &version); err != nil {
		return nil, fmt.Errorf("failed to parse version response: %w", err)
	}

	return &version, nil
}

// getRemoteInfraVersions queries infrastructure service versions via SSH
func getRemoteInfraVersions(cfg *Config) (*InfrastructureVersion, error) {
	versions := &InfrastructureVersion{}

	// PostgreSQL version - use password from config
	dbPassword := cfg.Database.Password
	if dbPassword == "" {
		// Skip PostgreSQL version check if no password configured
		versions.PostgreSQL = "(DATABASE_PASSWORD not set)"
	} else {
		pgCmd := fmt.Sprintf("PGPASSWORD='%s' psql -U %s -d %s -t -c \"SELECT version()\" 2>/dev/null | head -1",
			dbPassword, cfg.Database.User, cfg.Database.Name)
		pgOutput, err := RunOnInfraVM(pgCmd)
		if err == nil && pgOutput != "" {
			// Extract just the version number (e.g., "PostgreSQL 15.4")
			versions.PostgreSQL = strings.TrimSpace(strings.Split(pgOutput, ",")[0])
		}
	}

	// NATS version
	natsOutput, err := RunOnInfraVM("nats-server --version 2>/dev/null || echo 'not installed'")
	if err == nil {
		versions.NATS = strings.TrimSpace(natsOutput)
	}

	// Nginx version
	nginxOutput, err := RunOnInfraVM("nginx -v 2>&1 | head -1")
	if err == nil {
		versions.Nginx = strings.TrimSpace(nginxOutput)
	}

	// Redis version
	redisOutput, err := RunOnInfraVM("redis-cli INFO server 2>/dev/null | grep redis_version | cut -d: -f2")
	if err == nil && redisOutput != "" {
		versions.Redis = strings.TrimSpace(redisOutput)
	}

	return versions, nil
}

// getRemoteDBSchemaVersion queries the database migration version
func getRemoteDBSchemaVersion(cfg *Config) (*DBSchemaVersion, error) {
	dbPassword := cfg.Database.Password
	if dbPassword == "" {
		return nil, fmt.Errorf("DATABASE_PASSWORD environment variable is required")
	}

	// Get latest migration version
	versionCmd := fmt.Sprintf(
		"PGPASSWORD='%s' psql -U %s -d %s -t -c \"SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1\" 2>/dev/null",
		dbPassword, cfg.Database.User, cfg.Database.Name)
	versionOutput, err := RunOnInfraVM(versionCmd)
	if err != nil {
		return nil, err
	}

	// Get migration count
	countCmd := fmt.Sprintf(
		"PGPASSWORD='%s' psql -U %s -d %s -t -c \"SELECT COUNT(*) FROM schema_migrations\" 2>/dev/null",
		dbPassword, cfg.Database.User, cfg.Database.Name)
	countOutput, err := RunOnInfraVM(countCmd)
	if err != nil {
		return nil, err
	}

	count := 0
	fmt.Sscanf(strings.TrimSpace(countOutput), "%d", &count)

	return &DBSchemaVersion{
		Version:        strings.TrimSpace(versionOutput),
		MigrationCount: count,
	}, nil
}

// shouldRebuildAPI checks if the API needs to be rebuilt
func shouldRebuildAPI(cfg *Config) (bool, string, error) {
	localCommit := getGitCommit()

	remoteVersion, err := getRemoteAPIVersion(cfg)
	if err != nil {
		// Can't reach API - assume rebuild needed
		return true, fmt.Sprintf("cannot reach remote API: %v", err), nil
	}

	if remoteVersion.Commit == localCommit {
		return false, fmt.Sprintf("API already up to date (commit: %s)", localCommit), nil
	}

	return true, fmt.Sprintf("local commit %s differs from deployed %s", localCommit, remoteVersion.Commit), nil
}

// printVersionComparison shows local vs remote versions
func printVersionComparison(cfg *Config) {
	localVersion := getVersion()
	localCommit := getGitCommit()

	color.Cyan("Local Version:")
	fmt.Printf("  Version: %s\n", localVersion)
	fmt.Printf("  Commit:  %s\n", localCommit)

	remoteVersion, err := getRemoteAPIVersion(cfg)
	if err != nil {
		color.Yellow("\nRemote API: %v", err)
	} else {
		color.Cyan("\nRemote API:")
		fmt.Printf("  Version:    %s\n", remoteVersion.Version)
		fmt.Printf("  Commit:     %s\n", remoteVersion.Commit)
		fmt.Printf("  Built:      %s\n", remoteVersion.BuildTime)
		fmt.Printf("  Go:         %s\n", remoteVersion.GoVersion)

		if localCommit == remoteVersion.Commit {
			color.Green("\n✓ API is up to date")
		} else {
			color.Yellow("\n⚠ API needs update (local: %s, remote: %s)", localCommit, remoteVersion.Commit)
		}
	}

	infraVersions, err := getRemoteInfraVersions(cfg)
	if err != nil {
		color.Yellow("\nInfrastructure: %v", err)
	} else {
		color.Cyan("\nInfrastructure Services:")
		if infraVersions.PostgreSQL != "" {
			fmt.Printf("  PostgreSQL: %s\n", infraVersions.PostgreSQL)
		}
		if infraVersions.NATS != "" {
			fmt.Printf("  NATS:       %s\n", infraVersions.NATS)
		}
		if infraVersions.Nginx != "" {
			fmt.Printf("  Nginx:      %s\n", infraVersions.Nginx)
		}
		if infraVersions.Redis != "" {
			fmt.Printf("  Redis:      %s\n", infraVersions.Redis)
		}
	}

	dbVersion, err := getRemoteDBSchemaVersion(cfg)
	if err != nil {
		color.Yellow("\nDB Schema: %v", err)
	} else {
		color.Cyan("\nDatabase Schema:")
		fmt.Printf("  Latest Migration: %s\n", dbVersion.Version)
		fmt.Printf("  Total Migrations: %d\n", dbVersion.MigrationCount)
	}
}
