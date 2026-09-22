//go:build mage

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Security namespace contains security scanning targets
type Security mg.Namespace

// All runs all security scans
func (Security) All() error {
	mg.SerialDeps(Security.Govulncheck, Security.Gosec, Security.NpmAudit)
	color.Green("All security scans completed")
	return nil
}

// Govulncheck runs Go vulnerability checker on dependencies
func (Security) Govulncheck() error {
	color.Cyan("Running govulncheck on Go dependencies...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Check if govulncheck is installed
	if err := sh.Run("which", "govulncheck"); err != nil {
		color.Yellow("govulncheck not found, installing...")
		if err := sh.RunV("go", "install", "golang.org/x/vuln/cmd/govulncheck@latest"); err != nil {
			return fmt.Errorf("failed to install govulncheck: %w", err)
		}
	}

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	if err := sh.RunV("govulncheck", "./..."); err != nil {
		return fmt.Errorf("govulncheck found vulnerabilities: %w", err)
	}

	color.Green("No known vulnerabilities found in Go dependencies")
	return nil
}

// Gosec runs Go security static analysis
func (Security) Gosec() error {
	color.Cyan("Running gosec static security analysis...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Check if gosec is installed
	if err := sh.Run("which", "gosec"); err != nil {
		color.Yellow("gosec not found, installing...")
		if err := sh.RunV("go", "install", "github.com/securego/gosec/v2/cmd/gosec@latest"); err != nil {
			return fmt.Errorf("failed to install gosec: %w", err)
		}
	}

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	// Run gosec with medium+ severity, exclude tests
	output, err := sh.Output("gosec", "-fmt=text", "-severity=medium", "-exclude-dir=vendor", "./...")
	if err != nil {
		// gosec returns non-zero when it finds issues
		color.Yellow("WARNING: gosec found potential security issues:")
		fmt.Println(output)
		return nil // Don't fail, just warn
	}

	if strings.TrimSpace(output) != "" {
		fmt.Println(output)
	}

	color.Green("No security issues found by gosec")
	return nil
}

// NpmAudit runs npm security audit on JavaScript dependencies
func (Security) NpmAudit() error {
	color.Cyan("Running npm audit on JavaScript dependencies...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	// Check if node_modules exists
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		color.Yellow("node_modules not found, running npm install...")
		if err := sh.RunV("npm", "ci"); err != nil {
			return fmt.Errorf("failed to install npm dependencies: %w", err)
		}
	}

	output, err := sh.Output("npm", "audit", "--audit-level=high")
	if err != nil {
		// npm audit returns non-zero when it finds issues
		color.Yellow("WARNING: npm audit found vulnerabilities:")
		fmt.Println(output)
		color.Yellow("Run 'npm audit fix' to attempt automatic fixes")
		return nil // Don't fail, just warn
	}

	if strings.TrimSpace(output) != "" {
		fmt.Println(output)
	}

	color.Green("No high/critical vulnerabilities found in npm dependencies")
	return nil
}

// Report generates a security scan report
func (Security) Report() error {
	color.Cyan("Generating security scan report...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	reportFile := fmt.Sprintf("%s/security-report.txt", cfg.ProjectRoot)
	f, err := os.Create(reportFile)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "SECURITY SCAN REPORT\n")
	fmt.Fprintf(f, "====================\n\n")

	// Go vulnerability check
	fmt.Fprintf(f, "## Go Dependency Vulnerabilities (govulncheck)\n\n")
	if err := os.Chdir(cfg.APIDir); err == nil {
		output, _ := sh.Output("govulncheck", "./...")
		fmt.Fprintf(f, "%s\n\n", output)
	}

	// Gosec report
	fmt.Fprintf(f, "## Go Static Security Analysis (gosec)\n\n")
	if err := os.Chdir(cfg.APIDir); err == nil {
		output, _ := sh.Output("gosec", "-fmt=text", "-severity=low", "./...")
		fmt.Fprintf(f, "%s\n\n", output)
	}

	// npm audit
	fmt.Fprintf(f, "## JavaScript Dependency Vulnerabilities (npm audit)\n\n")
	if err := os.Chdir(cfg.WebDir); err == nil {
		output, _ := sh.Output("npm", "audit")
		fmt.Fprintf(f, "%s\n\n", output)
	}

	color.Green("Security report generated: %s", reportFile)
	return nil
}

// Fix attempts to fix security issues automatically
func (Security) Fix() error {
	color.Cyan("Attempting to fix security issues...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Fix npm vulnerabilities
	color.Cyan("Running npm audit fix...")
	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	if err := sh.RunV("npm", "audit", "fix"); err != nil {
		color.Yellow("WARNING: Some npm vulnerabilities could not be auto-fixed")
	}

	// Update Go dependencies
	color.Cyan("Updating Go dependencies...")
	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	if err := sh.RunV("go", "get", "-u", "./..."); err != nil {
		color.Yellow("WARNING: Some Go dependencies could not be updated")
	}

	if err := sh.RunV("go", "mod", "tidy"); err != nil {
		return fmt.Errorf("failed to tidy go.mod: %w", err)
	}

	color.Green("Security fix attempt completed")
	color.Yellow("Note: Re-run 'mage security:all' to verify fixes")
	return nil
}

// Deps checks for outdated dependencies
func (Security) Deps() error {
	color.Cyan("Checking for outdated dependencies...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Check Go dependencies
	color.Cyan("\n--- Go Dependencies ---")
	if err := os.Chdir(cfg.APIDir); err == nil {
		output, _ := sh.Output("go", "list", "-m", "-u", "all")
		lines := strings.Split(output, "\n")
		outdated := 0
		for _, line := range lines {
			if strings.Contains(line, "[") {
				fmt.Println(line)
				outdated++
			}
		}
		if outdated == 0 {
			color.Green("All Go dependencies are up to date")
		} else {
			color.Yellow("%d Go dependencies have updates available", outdated)
		}
	}

	// Check npm dependencies
	color.Cyan("\n--- npm Dependencies ---")
	if err := os.Chdir(cfg.WebDir); err == nil {
		output, _ := sh.Output("npm", "outdated")
		if strings.TrimSpace(output) == "" {
			color.Green("All npm dependencies are up to date")
		} else {
			fmt.Println(output)
		}
	}

	return nil
}
