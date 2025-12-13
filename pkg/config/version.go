// Package config provides configuration management for BlazeLog.
package config

import (
	"fmt"
	"runtime"
)

// Build information. Populated at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// BuildInfo contains all build information.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// GetBuildInfo returns the current build information.
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// VersionString returns a formatted version string.
func VersionString() string {
	return fmt.Sprintf("blazelog %s (%s) built at %s with %s",
		Version, Commit, BuildTime, runtime.Version())
}

// ShortVersionString returns a short version string.
func ShortVersionString() string {
	return Version
}
