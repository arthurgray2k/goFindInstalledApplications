package analyzer

import (
	"strings"

	"goFindInstalledApplications/pkg/packagemanager"
)

// PackageNode represents a node in the package dependency graph.
type PackageNode struct {
	Pkg      *packagemanager.Package
	Deps     []*PackageNode
	RevDeps  []*PackageNode
}

// DependencyGraph manages the package dependency analysis.
type DependencyGraph struct {
	Nodes map[string]*PackageNode
}

// NewDependencyGraph creates a new Package Dependency Graph from a slice of packages.
func NewDependencyGraph(packages []*packagemanager.Package) *DependencyGraph {
	g := &DependencyGraph{
		Nodes: make(map[string]*PackageNode),
	}

	// 1. Create all nodes
	for _, pkg := range packages {
		g.Nodes[pkg.Name] = &PackageNode{
			Pkg:     pkg,
			Deps:    []*PackageNode{},
			RevDeps: []*PackageNode{},
		}
	}

	// 2. Build provides capability map (capability -> package names)
	providesMap := make(map[string][]string)
	for _, node := range g.Nodes {
		for _, provide := range node.Pkg.Provides {
			providesMap[provide] = append(providesMap[provide], node.Pkg.Name)
		}
		// Also add package name itself as a capability it provides
		providesMap[node.Pkg.Name] = append(providesMap[node.Pkg.Name], node.Pkg.Name)
	}

	// 3. Resolve dependencies and create directed edges
	for _, node := range g.Nodes {
		resolvedDeps := make(map[string]bool)
		for _, req := range node.Pkg.Requires {
			// Resolve capability to providers
			providers, exists := providesMap[req]
			if !exists {
				// Sometimes capabilities have version constraints like "glib2 >= 2.88.1"
				// Clean the requirement string to match raw capability
				cleanReq := req
				if idx := stringsIndexAny(req, " <>=("); idx != -1 {
					cleanReq = strings.TrimSpace(req[:idx])
				}
				providers, exists = providesMap[cleanReq]
			}

			if exists {
				for _, providerName := range providers {
					if providerNode, found := g.Nodes[providerName]; found {
						// Avoid self-dependencies and duplicate edges
						if providerName != node.Pkg.Name && !resolvedDeps[providerName] {
							resolvedDeps[providerName] = true
							node.Deps = append(node.Deps, providerNode)
							providerNode.RevDeps = append(providerNode.RevDeps, node)
						}
					}
				}
			}
		}
	}

	return g
}

func stringsIndexAny(s string, chars string) int {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}

// GetTransitiveDeps returns all packages transitively required by the given package name.
func (g *DependencyGraph) GetTransitiveDeps(startPkgName string) map[string]*packagemanager.Package {
	visited := make(map[string]*packagemanager.Package)
	startNode, found := g.Nodes[startPkgName]
	if !found {
		return visited
	}

	g.dfs(startNode, visited)
	// Remove self from dependencies
	delete(visited, startPkgName)
	return visited
}

func (g *DependencyGraph) dfs(node *PackageNode, visited map[string]*packagemanager.Package) {
	for _, dep := range node.Deps {
		if _, seen := visited[dep.Pkg.Name]; !seen {
			visited[dep.Pkg.Name] = dep.Pkg
			g.dfs(dep, visited)
		}
	}
}

// Footprint represents the size footprint of an application.
type Footprint struct {
	SelfSize      int64
	ExclusiveSize int64
	SharedSize    int64
	TotalSize     int64
}

// CalculateFootprints computes the size breakdown for all target applications.
// targetApps is the set of packages classified as user-facing applications.
func (g *DependencyGraph) CalculateFootprints(targetApps map[string]bool) map[string]Footprint {
	// Build map: dependency name -> list of target apps that require it transitively
	depToUserApps := make(map[string][]string)

	for appName := range targetApps {
		transitive := g.GetTransitiveDeps(appName)
		for depName := range transitive {
			depToUserApps[depName] = append(depToUserApps[depName], appName)
		}
	}

	footprints := make(map[string]Footprint)

	for appName := range targetApps {
		node, found := g.Nodes[appName]
		if !found {
			continue
		}

		var exclusiveSize int64
		var sharedSize int64

		transitive := g.GetTransitiveDeps(appName)
		for depName, depPkg := range transitive {
			// If the dependency is itself a displayed target app, we don't count its size in
			// the dependency metrics to avoid double-counting.
			if targetApps[depName] {
				continue
			}

			userAppsRequiringIt := depToUserApps[depName]
			if len(userAppsRequiringIt) <= 1 {
				// Only required by this app, so it's exclusive
				exclusiveSize += depPkg.InstalledSize
			} else {
				// Required by multiple user apps, so it's shared
				sharedSize += depPkg.InstalledSize
			}
		}

		footprints[appName] = Footprint{
			SelfSize:      node.Pkg.InstalledSize,
			ExclusiveSize: exclusiveSize,
			SharedSize:    sharedSize,
			TotalSize:     node.Pkg.InstalledSize + exclusiveSize, // Total size is self + exclusive (the space actually freed on deletion)
		}
	}

	return footprints
}
