//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Dev namespace contains development workflow targets
type Dev mg.Namespace

// Setup initializes the development environment
// devComposeFile is the local dev stack. Compose does not auto-discover a
// -dev suffix, so every invocation has to name it explicitly.
const devComposeFile = "docker-compose.dev.yml"

func (Dev) Setup() error {
	color.Cyan("Setting up development environment...")

	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Check Go is installed
	if err := checkCommand("go"); err != nil {
		return fmt.Errorf("Go is required: %w", err)
	}

	// Check Node.js is installed
	if err := checkCommand("node"); err != nil {
		return fmt.Errorf("Node.js is required: %w", err)
	}

	// Check Docker is installed
	if err := checkCommand("docker"); err != nil {
		return fmt.Errorf("Docker is required: %w", err)
	}

	// Install Go dependencies
	color.Yellow("Installing Go dependencies...")
	if err := sh.Run("go", "mod", "download", "-x"); err != nil {
		return fmt.Errorf("failed to download Go dependencies: %w", err)
	}

	// Change to API directory and download dependencies
	if err := os.Chdir(cfg.APIDir); err == nil {
		if err := sh.Run("go", "mod", "download"); err != nil {
			color.Red("Warning: failed to download API dependencies: %v", err)
		}
	}

	// Install Node.js dependencies
	color.Yellow("Installing Node.js dependencies...")
	if err := os.Chdir(cfg.WebDir); err == nil {
		if err := sh.Run("npm", "install"); err != nil {
			return fmt.Errorf("failed to install npm dependencies: %w", err)
		}
	}

	// Start Docker services
	color.Yellow("Starting Docker services...")
	mg.Deps(Dev.Up)

	color.Green("✓ Development environment ready!")
	return nil
}

// Up starts local Docker services (PostgreSQL, NATS)
func (Dev) Up() error {
	color.Cyan("Starting local Docker services...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.ProjectRoot); err != nil {
		return err
	}

	if err := sh.RunV("docker", "compose", "-f", devComposeFile, "up", "-d"); err != nil {
		return fmt.Errorf("failed to start Docker services: %w", err)
	}

	color.Green("✓ Services started")
	return nil
}

// Down stops local Docker services
func (Dev) Down() error {
	color.Cyan("Stopping local Docker services...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.ProjectRoot); err != nil {
		return err
	}

	if err := sh.RunV("docker", "compose", "-f", devComposeFile, "down"); err != nil {
		return fmt.Errorf("failed to stop Docker services: %w", err)
	}

	color.Green("✓ Services stopped")
	return nil
}

// Status shows the status of local services
func (Dev) Status() error {
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	color.Cyan("Local Docker Services:")
	if err := os.Chdir(cfg.ProjectRoot); err != nil {
		return err
	}

	sh.RunV("docker", "compose", "-f", devComposeFile, "ps")

	fmt.Println()
	color.Cyan("API Server:")
	if _, err := sh.Output("pgrep", "-f", "labctl serve"); err == nil {
		color.Green("  Running")
	} else {
		color.Yellow("  Not running")
	}

	fmt.Println()
	color.Cyan("Web Dev Server:")
	if _, err := sh.Output("pgrep", "-f", "vite"); err == nil {
		color.Green("  Running")
	} else {
		color.Yellow("  Not running")
	}

	return nil
}

// Api runs the API server in development mode
func (Dev) Api() error {
	color.Cyan("Starting API server...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	// Set environment variables
	env := map[string]string{
		"DATABASE_HOST":     "localhost",
		"DATABASE_USER":     cfg.Database.User,
		"DATABASE_PASSWORD": cfg.Database.Password,
		"DATABASE_NAME":     cfg.Database.Name,
		"NATS_URL":          "nats://localhost:4222",
		"AUTH_DEMO_MODE":    "true",
		"DEV_MODE":          "true",
		"LOG_LEVEL":         "debug",
	}

	// Add Proxmox config if available
	if cfg.Proxmox.TokenID != "" {
		env["PROXMOX_HOST"] = cfg.Proxmox.Host
		env["PROXMOX_TOKEN_ID"] = cfg.Proxmox.TokenID
		env["PROXMOX_TOKEN"] = cfg.Proxmox.Token
		env["PROXMOX_INSECURE"] = "true"
	}

	// Set JWT secret
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		env["JWT_SECRET"] = secret
	} else {
		env["JWT_SECRET"] = "dev-secret-change-in-production-32chars"
	}

	return sh.RunWithV(env, "go", "run", "./cmd/labctl", "serve")
}

// Web runs the web dev server
func (Dev) Web() error {
	color.Cyan("Starting web dev server...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	return sh.RunV("npm", "run", "dev")
}

// Logs shows logs from Docker services
func (Dev) Logs() error {
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.ProjectRoot); err != nil {
		return err
	}

	return sh.RunV("docker", "compose", "-f", devComposeFile, "logs", "-f")
}

// Psql opens a psql shell to the local database
func (Dev) Psql() error {
	cfg := GetConfig()

	return sh.RunV("docker", "exec", "-it", "kootenai-postgres",
		"psql", "-U", cfg.Database.User, "-d", cfg.Database.Name)
}

// checkCommand checks if a command is available
func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in PATH", name)
	}
	return nil
}
