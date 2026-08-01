package analyzer

import (
	"fmt"
	"goFindInstalledApplications/pkg/detector"
	"goFindInstalledApplications/pkg/packagemanager"
)

// RunResult holds the output data of the analyzer.
type RunResult struct {
	Apps       []*detector.DetectedApp
	Footprints map[string]Footprint
}

// Run orchestrates the whole analysis flow.
func Run(showSystem bool) (*RunResult, error) {
	// 1. Detect and register package manager backends
	var backends []packagemanager.Backend

	dnf5Backend := packagemanager.NewDNF5Backend()
	if dnf5Backend.IsSupported() {
		backends = append(backends, dnf5Backend)
	}

	flatpakBackend := packagemanager.NewFlatpakBackend()
	if flatpakBackend.IsSupported() {
		backends = append(backends, flatpakBackend)
	}

	dpkgBackend := packagemanager.NewDpkgBackend()
	if dpkgBackend.IsSupported() {
		backends = append(backends, dpkgBackend)
	}

	pacmanBackend := packagemanager.NewPacmanBackend()
	if pacmanBackend.IsSupported() {
		backends = append(backends, pacmanBackend)
	}

	windowsBackend := packagemanager.NewWindowsBackend()
	if windowsBackend.IsSupported() {
		backends = append(backends, windowsBackend)
	}

	if len(backends) == 0 {
		return nil, fmt.Errorf("no supported package managers found on the system")
	}

	// 2. Fetch packages from all supported backends
	var allPackages []*packagemanager.Package
	for _, backend := range backends {
		pkgs, err := backend.ListPackages()
		if err != nil {
			return nil, fmt.Errorf("error listing packages from backend %s: %w", backend.Name(), err)
		}
		allPackages = append(allPackages, pkgs...)
	}

	// 3. Detect and filter user applications
	det := detector.NewDetector(showSystem)
	detectedApps, targetAppsSet := det.DetectAndFilter(allPackages)

	// 4. Build dependency graph and calculate footprints
	graph := NewDependencyGraph(allPackages)
	footprints := graph.CalculateFootprints(targetAppsSet)

	return &RunResult{
		Apps:       detectedApps,
		Footprints: footprints,
	}, nil
}
