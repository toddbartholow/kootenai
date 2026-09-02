//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds configuration for mage targets
type Config struct {
	// Project paths
	ProjectRoot string
	APIDir      string
	WebDir      string
	MigrationsDir string

	// Infra VM configuration
	InfraVM InfraVMConfig

	// Proxmox configuration
	Proxmox ProxmoxConfig

	// Database configuration
	Database DatabaseConfig
}

// InfraVMConfig holds infra VM connection details
type InfraVMConfig struct {
	Host     string
	User     string
	Password string
	RepoPath string
}

// ProxmoxConfig holds Proxmox API configuration
type ProxmoxConfig struct {
	Host     string
	TokenID  string
	Token    string
	Insecure bool
}

// DatabaseConfig holds database connection details
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// cfg is the global configuration instance
var cfg *Config

// GetConfig returns the global configuration, initializing if needed
func GetConfig() *Config {
	if cfg == nil {
		cfg = loadConfig()
	}
	return cfg
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	// Find project root (where go.mod is in api/)
	root := findProjectRoot()

	return &Config{
		ProjectRoot:   root,
		APIDir:        filepath.Join(root, "api"),
		WebDir:        filepath.Join(root, "web"),
		MigrationsDir: filepath.Join(root, "api", "internal", "database", "migrations"),

		InfraVM: InfraVMConfig{
			Host:     getEnv("INFRA_HOST", ""),
			User:     getEnv("INFRA_USER", "labadmin"),
			Password: getEnv("INFRA_PASSWORD", ""),
			RepoPath: getEnv("INFRA_REPO_PATH", "/home/labadmin/kootenai"),
		},

		Proxmox: ProxmoxConfig{
			Host:     getEnv("PROXMOX_HOST", ""),
			TokenID:  getEnv("PROXMOX_TOKEN_ID", ""),
			Token:    getEnv("PROXMOX_TOKEN", ""),
			Insecure: getEnv("PROXMOX_INSECURE", "false") == "true",
		},

		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST", "localhost"),
			Port:     getEnv("DATABASE_PORT", "5432"),
			User:     getEnv("DATABASE_USER", "labadmin"),
			Password: getEnv("DATABASE_PASSWORD", ""),
			Name:     getEnv("DATABASE_NAME", "virtuallab"),
		},
	}
}

// findProjectRoot finds the project root directory
func findProjectRoot() string {
	// First, check if PROJECT_ROOT is set explicitly
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}

	// Try to get the directory where this source file resides
	// The magefiles are in <project>/magefiles/, so project root is parent
	_, filename, _, ok := runtimeCallerFunc(0)
	if ok {
		// filename is the path to this file (config.go)
		// Go up two levels: config.go -> magefiles -> project root
		magefilesDir := filepath.Dir(filename)
		projectRoot := filepath.Dir(magefilesDir)
		if _, err := os.Stat(filepath.Join(projectRoot, "api", "go.mod")); err == nil {
			return projectRoot
		}
	}

	// Start from current directory as fallback
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	// Look for api/go.mod to identify project root
	for {
		if _, err := os.Stat(filepath.Join(dir, "api", "go.mod")); err == nil {
			return dir
		}
		// Also check if we're in magefiles/
		if _, err := os.Stat(filepath.Join(dir, "..", "api", "go.mod")); err == nil {
			return filepath.Join(dir, "..")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

// runtimeCallerFunc is a var to allow for testing - uses runtime.Caller
var runtimeCallerFunc = runtime.Caller

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Validate checks that required configuration is set
func (c *Config) Validate() error {
	if c.InfraVM.Password == "" {
		return fmt.Errorf("INFRA_PASSWORD environment variable is required")
	}
	return nil
}

// ValidateProxmox checks that Proxmox configuration is set
func (c *Config) ValidateProxmox() error {
	if c.Proxmox.TokenID == "" {
		return fmt.Errorf("PROXMOX_TOKEN_ID environment variable is required")
	}
	if c.Proxmox.Token == "" {
		return fmt.Errorf("PROXMOX_TOKEN environment variable is required")
	}
	return nil
}

// ValidateDatabase checks that database configuration is set
func (c *Config) ValidateDatabase() error {
	if c.Database.Password == "" {
		return fmt.Errorf("DATABASE_PASSWORD environment variable is required")
	}
	return nil
}

// DSN returns the PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
	)
}
