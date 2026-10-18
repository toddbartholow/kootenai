// Package version provides build-time version information
package version

import (
	"runtime"

	"github.com/toddbartholow/kootenai/api/internal/enterprise"
)

// Build-time variables - set via ldflags
// Example: go build -ldflags "-X github.com/toddbartholow/kootenai/api/internal/version.Version=v0.2.1"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Info contains build-time version information
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Edition   string `json:"edition"`
}

// Get returns the current version information
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Edition:   enterprise.Default.Edition(),
	}
}

// String returns a formatted version string
func String() string {
	return Version + " (" + Commit + ")"
}
