package packagemanager

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type DpkgBackend struct{}

func NewDpkgBackend() *DpkgBackend {
	return &DpkgBackend{}
}

func (d *DpkgBackend) Name() string {
	return "dpkg"
}

func (d *DpkgBackend) IsSupported() bool {
	_, err := exec.LookPath("dpkg-query")
	return err == nil
}

func (d *DpkgBackend) ListPackages() ([]*Package, error) {
	// 1. Fetch manual packages via apt-mark showmanual
	manualPkgs, err := d.queryManualPackages()
	if err != nil {
		return nil, fmt.Errorf("error querying manual packages: %w", err)
	}

	// 2. Query basic package info from dpkg-query
	packagesMap, err := d.queryBasicInfo(manualPkgs)
	if err != nil {
		return nil, fmt.Errorf("error querying basic package info: %w", err)
	}

	// 3. Scan /var/lib/dpkg/info/*.list files directly for files mapping (very fast fallback)
	if err := d.scanPackageFiles(packagesMap); err != nil {
		return nil, fmt.Errorf("error scanning package files: %w", err)
	}

	// Convert map to slice
	var packages []*Package
	for _, pkg := range packagesMap {
		packages = append(packages, pkg)
	}

	return packages, nil
}

func (d *DpkgBackend) queryManualPackages() (map[string]bool, error) {
	manualPkgs := make(map[string]bool)
	_, err := exec.LookPath("apt-mark")
	if err != nil {
		// If apt-mark is not found, we assume no manual package marking is available (fallback to all User/System)
		return manualPkgs, nil
	}

	cmd := exec.Command("apt-mark", "showmanual")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return manualPkgs, nil
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		pkg := strings.TrimSpace(scanner.Text())
		if pkg != "" {
			manualPkgs[pkg] = true
		}
	}
	return manualPkgs, nil
}

func (d *DpkgBackend) queryBasicInfo(manualPkgs map[string]bool) (map[string]*Package, error) {
	// Query format fields: Package|Version|Installed-Size|Depends|Recommends|Summary
	cmd := exec.Command("dpkg-query", "-W", "-f=${Package}|${Version}|${Installed-Size}|${Depends}|${Recommends}|${Summary}\n")
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

		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 3 {
			continue
		}

		name := parts[0]
		version := parts[1]
		sizeStr := parts[2]
		dependsStr := ""
		recommendsStr := ""
		summary := ""

		if len(parts) > 3 {
			dependsStr = parts[3]
		}
		if len(parts) > 4 {
			recommendsStr = parts[4]
		}
		if len(parts) > 5 {
			summary = parts[5]
		}

		// dpkg-query reports size in Kilobytes (KiB). Convert to bytes.
		sizeKB, _ := strconv.ParseInt(sizeStr, 10, 64)
		size := sizeKB * 1024

		// Determine InstallReason
		reason := ReasonDependency
		if manualPkgs[name] {
			reason = ReasonUser
		} else {
			// In Debian, essential/standard/required packages are pre-installed system packages
			// We can categorize them as System packages if they aren't marked as user-installed
			reason = ReasonSystem
		}

		// Parse requires (Depends + Recommends)
		var requires []string
		requires = parseDebianDeps(dependsStr, requires)
		requires = parseDebianDeps(recommendsStr, requires)

		// Provides includes the package name itself
		provides := []string{name}

		packagesMap[name] = &Package{
			Name:          name,
			Version:       version,
			InstalledSize: size,
			Reason:        reason,
			Summary:       summary,
			Provides:      provides,
			Requires:      requires,
			BinaryPaths:   []string{},
		}
	}

	return packagesMap, nil
}

func parseDebianDeps(depsStr string, list []string) []string {
	if depsStr == "" {
		return list
	}

	// Example: "libc6 (>= 2.14), libstdc++6 (>= 5.2) | libz-dev, zlib1g"
	for _, dep := range strings.Split(depsStr, ",") {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}

		// Handle alternatives (split by |)
		for _, alt := range strings.Split(dep, "|") {
			alt = strings.TrimSpace(alt)
			// Strip version constraints like (>= 1.2.3)
			if idx := strings.Index(alt, " "); idx != -1 {
				alt = alt[:idx]
			}
			// Strip architecture qualifiers like :any or :amd64
			if idx := strings.Index(alt, ":"); idx != -1 {
				alt = alt[:idx]
			}
			alt = strings.TrimSpace(alt)
			if alt != "" {
				list = append(list, alt)
			}
		}
	}
	return list
}

func (d *DpkgBackend) scanPackageFiles(packagesMap map[string]*Package) error {
	dpkgInfoPath := "/var/lib/dpkg/info"
	files, err := os.ReadDir(dpkgInfoPath)
	if err != nil {
		// If we can't read the directory (e.g. permission or not Debian), skip it
		return nil
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".list") {
			continue
		}

		// Get package name by stripping suffix
		fileName := file.Name()
		pkgName := strings.TrimSuffix(fileName, ".list")

		// Handle architecture qualifiers in filename (e.g., btop:amd64.list)
		if idx := strings.Index(pkgName, ":"); idx != -1 {
			pkgName = pkgName[:idx]
		}

		pkg, exists := packagesMap[pkgName]
		if !exists {
			continue
		}

		// Open and parse list file
		filePath := filepath.Join(dpkgInfoPath, fileName)
		listFile, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(listFile)
		for scanner.Scan() {
			path := strings.TrimSpace(scanner.Text())
			if path == "" {
				continue
			}

			if isBinaryPath(path) {
				pkg.BinaryPaths = append(pkg.BinaryPaths, path)
			} else if strings.HasSuffix(path, ".desktop") {
				pkg.BinaryPaths = append(pkg.BinaryPaths, path)
			}
		}
		listFile.Close()
	}

	return nil
}
