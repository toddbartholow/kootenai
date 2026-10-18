//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/magefile/mage/sh"
)

// LoadSmoke runs a quick load test smoke test (requires k6)
func (Test) LoadSmoke() error {
	color.Cyan("Running load test smoke test...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	loadtestDir := fmt.Sprintf("%s/tools/loadtest", cfg.ProjectRoot)

	// Check if k6 is installed
	if err := sh.Run("which", "k6"); err != nil {
		return fmt.Errorf("k6 not installed. Install with: brew install k6")
	}

	// Check if loadtest directory exists
	if _, err := os.Stat(loadtestDir); os.IsNotExist(err) {
		return fmt.Errorf("loadtest directory not found: %s", loadtestDir)
	}

	if err := os.Chdir(loadtestDir); err != nil {
		return err
	}

	// Set environment variables
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://<INFRA_VM_IP>:8080"
	}
	os.Setenv("API_URL", apiURL)

	if err := sh.RunV("k6", "run", "api-smoke.js"); err != nil {
		return fmt.Errorf("smoke test failed: %w", err)
	}

	color.Green("Load test smoke test passed")
	return nil
}

// LoadStandard runs a standard load test (requires k6)
func (Test) LoadStandard() error {
	color.Cyan("Running standard load test (5 min)...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	loadtestDir := fmt.Sprintf("%s/tools/loadtest", cfg.ProjectRoot)

	if err := sh.Run("which", "k6"); err != nil {
		return fmt.Errorf("k6 not installed. Install with: brew install k6")
	}

	if _, err := os.Stat(loadtestDir); os.IsNotExist(err) {
		return fmt.Errorf("loadtest directory not found: %s", loadtestDir)
	}

	if err := os.Chdir(loadtestDir); err != nil {
		return err
	}

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://<INFRA_VM_IP>:8080"
	}
	os.Setenv("API_URL", apiURL)

	if err := sh.RunV("k6", "run", "api-load.js"); err != nil {
		return fmt.Errorf("load test failed: %w", err)
	}

	color.Green("Standard load test completed")
	return nil
}

// LoadStress runs a stress test (requires k6)
func (Test) LoadStress() error {
	color.Cyan("Running stress test (~23 min)...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	loadtestDir := fmt.Sprintf("%s/tools/loadtest", cfg.ProjectRoot)

	if err := sh.Run("which", "k6"); err != nil {
		return fmt.Errorf("k6 not installed. Install with: brew install k6")
	}

	if _, err := os.Stat(loadtestDir); os.IsNotExist(err) {
		return fmt.Errorf("loadtest directory not found: %s", loadtestDir)
	}

	if err := os.Chdir(loadtestDir); err != nil {
		return err
	}

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://<INFRA_VM_IP>:8080"
	}
	os.Setenv("API_URL", apiURL)

	if err := sh.RunV("k6", "run", "api-stress.js"); err != nil {
		return fmt.Errorf("stress test failed: %w", err)
	}

	color.Green("Stress test completed")
	return nil
}

// CePublish runs the CE publishing workflow locally in dry-run mode
// This validates the sanitization and filtering without pushing to the public repo
func (Test) CePublish() error {
	color.Cyan("Running CE publishing workflow test...")
	cfg := GetConfig()

	scriptPath := fmt.Sprintf("%s/scripts/publish-ce-local.sh", cfg.ProjectRoot)

	// Check if script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("CE publish script not found: %s", scriptPath)
	}

	// Run the local test script in dry-run mode
	if err := sh.RunV("bash", scriptPath, "--dry-run"); err != nil {
		return fmt.Errorf("CE publish test failed: %w", err)
	}

	color.Green("CE publishing workflow test passed")
	return nil
}

// CePublishVerbose runs the CE publishing workflow with verbose output and keeps staging
func (Test) CePublishVerbose() error {
	color.Cyan("Running CE publishing workflow test (verbose)...")
	cfg := GetConfig()

	scriptPath := fmt.Sprintf("%s/scripts/publish-ce-local.sh", cfg.ProjectRoot)

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("CE publish script not found: %s", scriptPath)
	}

	// Run with verbose output and keep staging directory for inspection
	if err := sh.RunV("bash", scriptPath, "--dry-run", "--verbose", "--keep"); err != nil {
		return fmt.Errorf("CE publish test failed: %w", err)
	}

	color.Green("CE publishing workflow test passed")
	color.Cyan("Staging directory preserved at: /tmp/kootenai-ce-local")
	return nil
}
