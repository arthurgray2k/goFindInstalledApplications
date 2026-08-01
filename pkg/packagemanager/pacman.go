package packagemanager

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type PacmanBackend struct{}

func NewPacmanBackend() *PacmanBackend {
	return &PacmanBackend{}
}

func (p *PacmanBackend) Name() string {
	return "pacman"
}

func (p *PacmanBackend) IsSupported() bool {
	_, err := exec.LookPath("pacman")
	return err == nil
}

func (p *PacmanBackend) ListPackages() ([]*Package, error) {
	// 1. Query basic package info and dependencies from pacman -Qi
	packagesMap, err := p.queryBasicInfo()
	if err != nil {
		return nil, fmt.Errorf("error querying pacman package info: %w", err)
	}

	// 2. Query file lists from pacman -Ql (bulk list for all packages)
	if err := p.queryFiles(packagesMap); err != nil {
		return nil, fmt.Errorf("error querying pacman package files: %w", err)
	}

	// Convert map to slice
	var packages []*Package
	for _, pkg := range packagesMap {
		packages = append(packages, pkg)
	}

	return packages, nil
}

func (p *PacmanBackend) queryBasicInfo() (map[string]*Package, error) {
	cmd := exec.Command("pacman", "-Qi")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	packagesMap := make(map[string]*Package)
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var currentPkg *Package
	var currentKey string

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		// Handle indentation-wrapped multiline values (e.g. Depends On / Provides)
		if strings.HasPrefix(line, "    ") && currentPkg != nil && currentKey != "" {
			d := trimmedLine
			if currentKey == "Depends On" {
				currentPkg.Requires = append(currentPkg.Requires, parsePacmanDeps(d)...)
			} else if currentKey == "Provides" {
				currentPkg.Provides = append(currentPkg.Provides, parsePacmanDeps(d)...)
			}
			continue
		}

		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		if key == "Name" {
			if currentPkg != nil {
				packagesMap[currentPkg.Name] = currentPkg
			}
			currentPkg = &Package{
				Name:        val,
				Provides:    []string{val}, // Provide itself
				Requires:    []string{},
				BinaryPaths: []string{},
			}
			currentKey = "Name"
		} else if currentPkg != nil {
			currentKey = key
			switch key {
			case "Version":
				currentPkg.Version = val
			case "Description":
				currentPkg.Summary = val
			case "Installed Size":
				currentPkg.InstalledSize = parsePacmanSize(val)
			case "Install Reason":
				if val == "Explicitly installed" {
					currentPkg.Reason = ReasonUser
				} else {
					currentPkg.Reason = ReasonDependency
				}
			case "Depends On":
				currentPkg.Requires = append(currentPkg.Requires, parsePacmanDeps(val)...)
			case "Provides":
				currentPkg.Provides = append(currentPkg.Provides, parsePacmanDeps(val)...)
			}
		}
	}

	if currentPkg != nil {
		packagesMap[currentPkg.Name] = currentPkg
	}

	return packagesMap, scanner.Err()
}

func parsePacmanSize(sizeStr string) int64 {
	sizeStr = strings.ToLower(strings.TrimSpace(sizeStr))
	parts := strings.Fields(sizeStr)
	if len(parts) < 2 {
		return 0
	}

	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}

	unit := parts[1]
	var multiplier float64 = 1

	switch {
	case strings.Contains(unit, "gib") || strings.Contains(unit, "g"):
		multiplier = 1024 * 1024 * 1024
	case strings.Contains(unit, "mib") || strings.Contains(unit, "m"):
		multiplier = 1024 * 1024
	case strings.Contains(unit, "kib") || strings.Contains(unit, "k"):
		multiplier = 1024
	default:
		multiplier = 1
	}

	return int64(val * multiplier)
}

func parsePacmanDeps(depsStr string) []string {
	var deps []string
	if depsStr == "None" {
		return deps
	}

	// Split by space
	for _, dep := range strings.Fields(depsStr) {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}

		// Strip version constraints: >=, <=, =, >, <
		if idx := strings.IndexAny(dep, ">=<"); idx != -1 {
			dep = dep[:idx]
		}
		if dep != "" {
			deps = append(deps, dep)
		}
	}
	return deps
}

func (p *PacmanBackend) queryFiles(packagesMap map[string]*Package) error {
	// Query files for all packages: pacman -Ql
	cmd := exec.Command("pacman", "-Ql")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		pkgName := parts[0]
		filePath := parts[1]

		pkg, exists := packagesMap[pkgName]
		if !exists {
			continue
		}

		if isBinaryPath(filePath) {
			pkg.BinaryPaths = append(pkg.BinaryPaths, filePath)
		} else if strings.HasSuffix(filePath, ".desktop") {
			pkg.BinaryPaths = append(pkg.BinaryPaths, filePath)
		}
	}

	return scanner.Err()
}
