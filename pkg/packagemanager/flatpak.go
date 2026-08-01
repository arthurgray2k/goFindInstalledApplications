package packagemanager

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

type FlatpakBackend struct{}

func NewFlatpakBackend() *FlatpakBackend {
	return &FlatpakBackend{}
}

func (f *FlatpakBackend) Name() string {
	return "flatpak"
}

func (f *FlatpakBackend) IsSupported() bool {
	_, err := exec.LookPath("flatpak")
	return err == nil
}

func (f *FlatpakBackend) ListPackages() ([]*Package, error) {
	var packages []*Package

	// 1. List Applications
	apps, err := f.queryFlatpaks(false)
	if err != nil {
		return nil, err
	}
	packages = append(packages, apps...)

	// 2. List Runtimes (which serve as dependencies)
	runtimes, err := f.queryFlatpaks(true)
	if err != nil {
		return nil, err
	}
	packages = append(packages, runtimes...)

	return packages, nil
}

func (f *FlatpakBackend) queryFlatpaks(isRuntime bool) ([]*Package, error) {
	arg := "--app"
	if isRuntime {
		arg = "--runtime"
	}

	cmd := exec.Command("flatpak", "list", arg, "--columns=application,name,version,runtime,installation,size,description")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// If flatpak list fails or has nothing, return empty slice
	if err := cmd.Run(); err != nil {
		return []*Package{}, nil
	}

	var packages []*Package
	scanner := bufio.NewScanner(&stdout)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}

		appID := parts[0]
		name := parts[1]
		version := parts[2]
		runtimeRef := parts[3]
		installation := parts[4]
		sizeStr := parts[5]
		summary := name
		if len(parts) > 6 && parts[6] != "" {
			summary = name + " - " + parts[6]
		}

		size := parseFlatpakSize(sizeStr)

		// Reason classification:
		// Apps are ReasonUser (unless system-wide preinstalled is specified, but flatpaks are user-installed apps by design)
		// Runtimes are ReasonDependency
		reason := ReasonUser
		if isRuntime {
			reason = ReasonDependency
		} else if installation == "system" {
			// Even if system installation, it is a user application, but we can tag it.
			// Let's keep it ReasonUser because it's a flatpak app that user can run.
			reason = ReasonUser
		}

		// Set binary path to "flatpak run <appID>"
		var binaryPaths []string
		if !isRuntime {
			binaryPaths = []string{"flatpak run " + appID}
		}

		// Runtimes are required by apps
		var requires []string
		if runtimeRef != "" && !isRuntime {
			// Extract runtime ID (first part of ref before slash)
			rParts := strings.Split(runtimeRef, "/")
			if len(rParts) > 0 {
				requires = append(requires, rParts[0])
			}
		}

		// Flatpak applications provide their own AppID
		provides := []string{appID}

		packages = append(packages, &Package{
			Name:          appID,
			Version:       version,
			InstalledSize: size,
			Reason:        reason,
			Summary:       summary,
			BinaryPaths:   binaryPaths,
			Provides:      provides,
			Requires:      requires,
			IsFlatpak:     true,
		})
	}

	return packages, scanner.Err()
}

func parseFlatpakSize(sizeStr string) int64 {
	sizeStr = strings.ToLower(strings.TrimSpace(sizeStr))
	if sizeStr == "" || sizeStr == "0" || sizeStr == "no size" || strings.Contains(sizeStr, "unknown") {
		return 0
	}

	// Examples: "1.2 gb", "240.5 mb", "10 kb", "500 bytes", "500 b"
	var numberStr string
	var multiplier float64 = 1

	if idx := strings.IndexFunc(sizeStr, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	}); idx != -1 {
		numberStr = strings.TrimSpace(sizeStr[:idx])
		unit := strings.TrimSpace(sizeStr[idx:])
		switch {
		case strings.Contains(unit, "gb") || strings.Contains(unit, "g"):
			multiplier = 1024 * 1024 * 1024
		case strings.Contains(unit, "mb") || strings.Contains(unit, "m"):
			multiplier = 1024 * 1024
		case strings.Contains(unit, "kb") || strings.Contains(unit, "k"):
			multiplier = 1024
		default:
			multiplier = 1
		}
	} else {
		numberStr = sizeStr
	}

	val, err := strconv.ParseFloat(numberStr, 64)
	if err != nil {
		return 0
	}

	return int64(val * multiplier)
}
