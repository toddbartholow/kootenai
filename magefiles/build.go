//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Build namespace contains build targets
type Build mg.Namespace

// All builds everything (API, Web, Docker images)
func (Build) All() error {
	mg.SerialDeps(Build.Api, Build.Web)
	color.Green("✓ All builds completed")
	return nil
}

// Api builds the Go API binary for the current OS
func (Build) Api() error {
	color.Cyan("Building API binary...")
	cfg := GetConfig()

	// Save current directory and restore it after
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return fmt.Errorf("chdir %s: %w", cfg.APIDir, err)
	}

	ldflags := buildLdflags()

	env := map[string]string{
		"CGO_ENABLED": "0",
	}

	outputPath := filepath.Join("bin", "labctl")
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

	if err := sh.RunWithV(env, "go", "build", "-ldflags", ldflags, "-o", outputPath, "./cmd/labctl"); err != nil {
		return fmt.Errorf("failed to build API: %w", err)
	}

	color.Green("✓ API binary built: %s", outputPath)
	return nil
}

// ApiLinux cross-compiles the API for Linux amd64 (for deployment)
func (Build) ApiLinux() error {
	color.Cyan("Cross-compiling API for Linux amd64...")
	cfg := GetConfig()

	// Save current directory and restore it after
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	ldflags := buildLdflags()

	env := map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
		"GOARCH":      "amd64",
	}

	outputPath := filepath.Join("bin", "labctl-linux-amd64")

	if err := sh.RunWithV(env, "go", "build", "-ldflags", ldflags, "-o", outputPath, "./cmd/labctl"); err != nil {
		return fmt.Errorf("failed to cross-compile API: %w", err)
	}

	color.Green("✓ Linux binary built: %s", outputPath)
	return nil
}

// Web builds the Vue.js frontend
func (Build) Web() error {
	color.Cyan("Building web frontend...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	// Run typecheck first
	color.Yellow("  Running typecheck...")
	if err := sh.RunV("npm", "run", "typecheck"); err != nil {
		return fmt.Errorf("typecheck failed: %w", err)
	}

	// Build
	color.Yellow("  Building production bundle...")
	if err := sh.RunV("npm", "run", "build"); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	color.Green("✓ Web frontend built: dist/")
	return nil
}

// Docker builds all Docker images
func (Build) Docker() error {
	mg.SerialDeps(Build.DockerApi, Build.DockerWeb)
	color.Green("✓ All Docker images built")
	return nil
}

// DockerApi builds the API Docker image
func (Build) DockerApi() error {
	color.Cyan("Building API Docker image...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	tag := fmt.Sprintf("kootenai-api:%s", getVersion())
	if err := sh.RunV("docker", "build", "-t", tag, "-t", "kootenai-api:latest", "."); err != nil {
		return fmt.Errorf("failed to build API image: %w", err)
	}

	color.Green("✓ API image built: %s", tag)
	return nil
}

// DockerWeb builds the Web Docker image
func (Build) DockerWeb() error {
	color.Cyan("Building Web Docker image...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	tag := fmt.Sprintf("kootenai-web:%s", getVersion())
	if err := sh.RunV("docker", "build", "-t", tag, "-t", "kootenai-web:latest", "."); err != nil {
		return fmt.Errorf("failed to build Web image: %w", err)
	}

	color.Green("✓ Web image built: %s", tag)
	return nil
}

// Swagger generates OpenAPI/Swagger documentation
func (Build) Swagger() error {
	color.Cyan("Generating Swagger documentation...")
	cfg := GetConfig()

	// Save current directory and restore it after
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return fmt.Errorf("chdir %s: %w", cfg.APIDir, err)
	}

	// Generate swagger docs with comprehensive directory scanning
	// --parseDependency: Parse dependencies to include external models
	// --parseInternal: Parse internal packages
	// --dir: Include server handlers directory for annotations
	if err := sh.RunV("swag", "init",
		"-g", "cmd/labctl/main.go",
		"-o", "docs",
		"--parseDependency",
		"--parseInternal",
		"--dir", "./,./internal/server",
	); err != nil {
		return fmt.Errorf("failed to generate swagger docs: %w", err)
	}

	color.Green("✓ Swagger documentation generated: api/docs/")
	return nil
}

// Clean removes build artifacts
func (Build) Clean() error {
	color.Cyan("Cleaning build artifacts...")
	cfg := GetConfig()

	// Clean API binaries
	binDir := filepath.Join(cfg.APIDir, "bin")
	if err := os.RemoveAll(binDir); err != nil {
		color.Yellow("Warning: failed to remove %s: %v", binDir, err)
	}

	// Clean web dist
	distDir := filepath.Join(cfg.WebDir, "dist")
	if err := os.RemoveAll(distDir); err != nil {
		color.Yellow("Warning: failed to remove %s: %v", distDir, err)
	}

	color.Green("✓ Build artifacts cleaned")
	return nil
}

// buildLdflags returns ldflags for embedding version info into binaries
func buildLdflags() string {
	version := getVersion()
	commit := getGitCommit()
	buildTime := time.Now().Format(time.RFC3339)

	// Set version info in both main package (for CLI) and version package (for API)
	versionPkg := "github.com/toddbartholow/kootenai/api/internal/version"
	return fmt.Sprintf("-s -w -X main.Version=%s -X main.Commit=%s -X main.BuildTime=%s -X %s.Version=%s -X %s.Commit=%s -X %s.BuildTime=%s",
		version, commit, buildTime,
		versionPkg, version, versionPkg, commit, versionPkg, buildTime)
}

// getVersion returns the version from git tag or commit
func getVersion() string {
	cfg := GetConfig()
	// Try to get version from git tag, run from project root
	if version, err := sh.Output("git", "-C", cfg.ProjectRoot, "describe", "--tags", "--always"); err == nil {
		return version
	}
	return "dev"
}

// getGitCommit returns the current git commit hash
func getGitCommit() string {
	cfg := GetConfig()
	if commit, err := sh.Output("git", "-C", cfg.ProjectRoot, "rev-parse", "--short", "HEAD"); err == nil {
		return commit
	}
	return "unknown"
}
