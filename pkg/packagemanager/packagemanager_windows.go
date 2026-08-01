//go:build windows

package packagemanager

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

type WindowsBackend struct{}

func NewWindowsBackend() *WindowsBackend {
	return &WindowsBackend{}
}

func (w *WindowsBackend) Name() string {
	return "windows"
}

func (w *WindowsBackend) IsSupported() bool {
	return true
}

func (w *WindowsBackend) ListPackages() ([]*Package, error) {
	var packages []*Package
	seen := make(map[string]bool)

	// List of registry bases and paths to query
	targets := []struct {
		hive registry.Key
		path string
		user bool
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, false},
		{registry.LOCAL_MACHINE, `SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, false},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, true},
	}

	for _, target := range targets {
		k, err := registry.OpenKey(target.hive, target.path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}

		subkeys, err := k.ReadSubKeyNames(-1)
		if err != nil {
			k.Close()
			continue
		}

		for _, subkeyName := range subkeys {
			sk, err := registry.OpenKey(k, subkeyName, registry.QUERY_VALUE)
			if err != nil {
				continue
			}

			displayName, _, err := sk.GetStringValue("DisplayName")
			if err != nil || displayName == "" {
				sk.Close()
				continue
			}

			// Deduplicate by DisplayName
			if seen[displayName] {
				sk.Close()
				continue
			}
			seen[displayName] = true

			version, _, _ := sk.GetStringValue("DisplayVersion")
			installLocation, _, _ := sk.GetStringValue("InstallLocation")
			uninstallString, _, _ := sk.GetStringValue("UninstallString")

			var size int64
			sizeDWORD, _, err := sk.GetIntegerValue("EstimatedSize")
			if err == nil {
				// EstimatedSize is typically stored in Kilobytes (KiB)
				size = int64(sizeDWORD) * 1024
			}

			sk.Close()

			// Categorize system vs user packages
			reason := ReasonUser
			if !target.user && isWindowsSystemPackage(displayName) {
				reason = ReasonSystem
			}

			// Capture install folder / uninstall command as path
			binaryPaths := []string{}
			if installLocation != "" {
				binaryPaths = append(binaryPaths, installLocation)
			} else if uninstallString != "" {
				binaryPaths = append(binaryPaths, uninstallString)
			}

			packages = append(packages, &Package{
				Name:          displayName,
				Version:       version,
				InstalledSize: size,
				Reason:        reason,
				Summary:       "Windows installed application",
				BinaryPaths:   binaryPaths,
				Provides:      []string{displayName},
				Requires:      []string{},
			})
		}
		k.Close()
	}

	return packages, nil
}

func isWindowsSystemPackage(name string) bool {
	lowerName := strings.ToLower(name)
	systemKeywords := []string{
		"security update",
		"update for microsoft",
		"microsoft visual c++",
		"windows driver package",
		"microsoft .net",
		"windows software development kit",
	}
	for _, kw := range systemKeywords {
		if strings.Contains(lowerName, kw) {
			return true
		}
	}
	return false
}
