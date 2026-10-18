package version

import (
	"runtime"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	// Check that default values are set
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
	if info.Commit == "" {
		t.Error("Commit should not be empty")
	}
	if info.BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}

	// Check runtime info
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %s, want %s", info.GoVersion, runtime.Version())
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS = %s, want %s", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %s, want %s", info.Arch, runtime.GOARCH)
	}
}

func TestString(t *testing.T) {
	s := String()

	// Should contain version and commit
	if s == "" {
		t.Error("String() should not return empty string")
	}

	// Check format "version (commit)"
	expected := Version + " (" + Commit + ")"
	if s != expected {
		t.Errorf("String() = %s, want %s", s, expected)
	}
}

func TestGet_WithCustomValues(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := Commit
	origBuildTime := BuildTime

	// Set custom values
	Version = "v1.0.0"
	Commit = "abc123"
	BuildTime = "2024-01-01T00:00:00Z"

	defer func() {
		// Restore original values
		Version = origVersion
		Commit = origCommit
		BuildTime = origBuildTime
	}()

	info := Get()

	if info.Version != "v1.0.0" {
		t.Errorf("Version = %s, want v1.0.0", info.Version)
	}
	if info.Commit != "abc123" {
		t.Errorf("Commit = %s, want abc123", info.Commit)
	}
	if info.BuildTime != "2024-01-01T00:00:00Z" {
		t.Errorf("BuildTime = %s, want 2024-01-01T00:00:00Z", info.BuildTime)
	}
}

func TestString_WithCustomValues(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := Commit

	// Set custom values
	Version = "v2.0.0"
	Commit = "def456"

	defer func() {
		// Restore original values
		Version = origVersion
		Commit = origCommit
	}()

	s := String()
	expected := "v2.0.0 (def456)"

	if s != expected {
		t.Errorf("String() = %s, want %s", s, expected)
	}
}
