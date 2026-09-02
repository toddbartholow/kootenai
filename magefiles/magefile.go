//go:build mage

// Package main provides the Mage build system for kootenai.
// Run 'mage -l' to list all available targets.
// v2.0 - Prefer external ssh for reliability
package main

import (
	"os"

	"github.com/magefile/mage/mg"
)

// Default target when running `mage` without arguments
var Default = Dev.Status

func init() {
	// Change to project root directory
	if err := os.Chdir(".."); err != nil {
		// Already in project root or magefiles doesn't exist
		_ = err
	}
}

// Aliases for common commands
var Aliases = map[string]interface{}{
	"b":  Build.All,
	"t":  Test.All,
	"d":  Deploy.All,
	"up": Dev.Up,
}

// Clean removes build artifacts
func Clean() error {
	mg.Deps(Build.Clean)
	return nil
}
