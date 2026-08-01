package packagemanager

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type DNF5Backend struct{}

func NewDNF5Backend() *DNF5Backend {
	return &DNF5Backend{}
}

func (d *DNF5Backend) Name() string {
	return "dnf5"
}

func (d *DNF5Backend) IsSupported() bool {
	_, err := exec.LookPath("dnf5")
	return err == nil
}

func (d *DNF5Backend) ListPackages() ([]*Package, error) {
	// 1. Query package basic info
	packagesMap, err := d.queryBasicInfo()
	if err != nil {
		return nil, fmt.Errorf("error querying basic package info: %w", err)
	}

	// 2. Query provides (capabilities)
	if err := d.queryProvides(packagesMap); err != nil {
		return nil, fmt.Errorf("error querying provides: %w", err)
	}

	// 3. Query requires (dependencies)
	if err := d.queryRequires(packagesMap); err != nil {
		return nil, fmt.Errorf("error querying requires: %w", err)
	}

	// 4. Query files (for identifying executables and desktop files)
	if err := d.queryFiles(packagesMap); err != nil {
		return nil, fmt.Errorf("error querying files: %w", err)
	}

	// Convert map to slice
	var packages []*Package
	for _, pkg := range packagesMap {
		packages = append(packages, pkg)
	}

	return packages, nil
}

func (d *DNF5Backend) queryBasicInfo() (map[string]*Package, error) {
	cmd := exec.Command("dnf5", "repoquery", "-C", "-q", "--installed", "--queryformat", "%{name}|%{version}-%{release}|%{reason}|%{installsize}|%{summary}\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	packagesMap := make(map[string]*Package)
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 4 {
			continue
		}

		name := parts[0]
		version := parts[1]
		reasonStr := parts[2]
		sizeStr := parts[3]
		summary := ""
		if len(parts) == 5 {
			summary = parts[4]
		}

		size, _ := strconv.ParseInt(sizeStr, 10, 64)

		var reason InstallReason
		switch reasonStr {
		case "User":
			reason = ReasonUser
		case "Dependency", "Weak Dependency":
			reason = ReasonDependency
		default:
			reason = ReasonSystem
		}

		packagesMap[name] = &Package{
			Name:          name,
			Version:       version,
			InstalledSize: size,
			Reason:        reason,
			Summary:       summary,
			Provides:      []string{},
			Requires:      []string{},
			BinaryPaths:   []string{},
		}
	}

	return packagesMap, scanner.Err()
}

func (d *DNF5Backend) queryProvides(packagesMap map[string]*Package) error {
	cmd := exec.Command("dnf5", "repoquery", "-C", "-q", "--installed", "--queryformat", "%{name}|%{provides}\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var currentPkg *Package

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if idx := strings.Index(line, "|"); idx != -1 {
			pkgName := line[:idx]
			provide := line[idx+1:]
			currentPkg = packagesMap[pkgName]
			if currentPkg != nil && provide != "" {
				currentPkg.Provides = append(currentPkg.Provides, provide)
			}
		} else if currentPkg != nil {
			currentPkg.Provides = append(currentPkg.Provides, line)
		}
	}

	return scanner.Err()
}

func (d *DNF5Backend) queryRequires(packagesMap map[string]*Package) error {
	cmd := exec.Command("dnf5", "repoquery", "-C", "-q", "--installed", "--queryformat", "%{name}|%{requires}\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var currentPkg *Package

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if idx := strings.Index(line, "|"); idx != -1 {
			pkgName := line[:idx]
			req := line[idx+1:]
			currentPkg = packagesMap[pkgName]
			if currentPkg != nil && req != "" {
				currentPkg.Requires = append(currentPkg.Requires, req)
			}
		} else if currentPkg != nil {
			currentPkg.Requires = append(currentPkg.Requires, line)
		}
	}

	return scanner.Err()
}

func (d *DNF5Backend) queryFiles(packagesMap map[string]*Package) error {
	cmd := exec.Command("dnf5", "repoquery", "-C", "-q", "--installed", "--queryformat", "%{name}|%{files}\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var currentPkg *Package

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var pkgName, filePath string
		if idx := strings.Index(line, "|"); idx != -1 {
			pkgName = line[:idx]
			filePath = line[idx+1:]
			currentPkg = packagesMap[pkgName]
		} else {
			filePath = line
		}

		if currentPkg == nil || filePath == "" {
			continue
		}

		// Optimization: We only care about executable paths in standard bin directories and .desktop files.
		// This keeps memory low by filtering out library/resource files.
		if isBinaryPath(filePath) {
			currentPkg.BinaryPaths = append(currentPkg.BinaryPaths, filePath)
		} else if strings.HasSuffix(filePath, ".desktop") {
			// We can also store desktop files or handle them separately, but let's keep them in BinaryPaths or another way
			// For now, let's treat the .desktop file as a binary path so the detector knows it's a desktop app.
			currentPkg.BinaryPaths = append(currentPkg.BinaryPaths, filePath)
		}
	}

	return scanner.Err()
}
