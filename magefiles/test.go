//go:build mage

package main

import (
	"os"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Test namespace contains test targets
type Test mg.Namespace

// All runs all tests (API and Web)
func (Test) All() error {
	mg.SerialDeps(Test.Api, Test.Web)
	color.Green("All tests passed")
	return nil
}

// Api runs Go API tests
func (Test) Api() error {
	color.Cyan("Running API tests...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	if err := sh.RunV("go", "test", "-v", "./..."); err != nil {
		return err
	}

	color.Green("API tests passed")
	return nil
}

// ApiCoverage runs Go API tests with coverage report
func (Test) ApiCoverage() error {
	color.Cyan("Running API tests with coverage...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	// Run tests with coverage
	if err := sh.RunV("go", "test", "-v", "-race", "-coverprofile=coverage.out", "-covermode=atomic", "./..."); err != nil {
		return err
	}

	// Generate HTML report
	if err := sh.RunV("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html"); err != nil {
		color.Yellow("Warning: failed to generate HTML coverage report")
	}

	// Show coverage summary
	sh.RunV("go", "tool", "cover", "-func=coverage.out")

	color.Green("Coverage report: api/coverage.html")
	return nil
}

// ApiRace runs Go API tests with race detection
func (Test) ApiRace() error {
	color.Cyan("Running API tests with race detection...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	if err := sh.RunV("go", "test", "-v", "-race", "./..."); err != nil {
		return err
	}

	color.Green("API tests passed (with race detection)")
	return nil
}

// Web runs Vue.js tests
func (Test) Web() error {
	color.Cyan("Running Web tests...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	if err := sh.RunV("npm", "run", "test:run"); err != nil {
		return err
	}

	color.Green("Web tests passed")
	return nil
}

// WebCoverage runs Vue.js tests with coverage
func (Test) WebCoverage() error {
	color.Cyan("Running Web tests with coverage...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	if err := sh.RunV("npm", "run", "test:coverage"); err != nil {
		return err
	}

	color.Green("Web coverage report: web/coverage/")
	return nil
}

// Lint runs linters on all code
func (Test) Lint() error {
	mg.SerialDeps(Test.LintApi, Test.LintWeb)
	color.Green("All linting passed")
	return nil
}

// LintApi runs Go linters
func (Test) LintApi() error {
	color.Cyan("Linting API code...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.APIDir); err != nil {
		return err
	}

	// Run go vet
	if err := sh.RunV("go", "vet", "./..."); err != nil {
		return err
	}

	// Run go fmt check
	output, err := sh.Output("gofmt", "-l", ".")
	if err != nil {
		return err
	}
	if output != "" {
		color.Yellow("Unformatted files:\n%s", output)
		color.Yellow("Run 'go fmt ./...' to fix")
	}

	color.Green("API linting passed")
	return nil
}

// LintWeb runs Vue.js/TypeScript linters
func (Test) LintWeb() error {
	color.Cyan("Linting Web code...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(cfg.WebDir); err != nil {
		return err
	}

	// Run typecheck
	if err := sh.RunV("npm", "run", "typecheck"); err != nil {
		return err
	}

	color.Green("Web linting passed")
	return nil
}
