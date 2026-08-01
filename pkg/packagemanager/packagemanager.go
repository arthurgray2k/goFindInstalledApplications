package packagemanager

import "strings"

// InstallReason defines why a package was installed.
type InstallReason string

const (
	ReasonUser       InstallReason = "User"       // Explicitly installed by the user
	ReasonSystem     InstallReason = "System"     // Pre-installed or part of the system base
	ReasonDependency InstallReason = "Dependency" // Automatically installed as a dependency
)

// Package represents metadata and dependency info for an installed package.
type Package struct {
	Name          string        `json:"name"`
	Version       string        `json:"version"`
	InstalledSize int64         `json:"installed_size"` // Size in bytes
	Reason        InstallReason `json:"reason"`
	Summary       string        `json:"summary"`
	BinaryPaths   []string      `json:"binary_paths"`   // Resolved executable binary paths
	Provides      []string      `json:"provides"`       // Capabilities or files provided
	Requires      []string      `json:"requires"`       // Capabilities or packages required
	IsFlatpak     bool          `json:"is_flatpak"`
}

// Backend defines the interface that all package manager backends must implement.
type Backend interface {
	Name() string
	IsSupported() bool
	ListPackages() ([]*Package, error)
}

func isBinaryPath(path string) bool {
	binDirs := []string{
		"/bin/",
		"/sbin/",
		"/usr/bin/",
		"/usr/sbin/",
		"/usr/local/bin/",
		"/usr/local/sbin/",
		"/usr/games/",
	}
	for _, dir := range binDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}
