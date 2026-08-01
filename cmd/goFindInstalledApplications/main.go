package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"goFindInstalledApplications/pkg/analyzer"
)

func main() {
	// 1. Define command-line flags
	showSystem := flag.Bool("system", false, "Include system-installed applications (filtered for libraries)")
	formatFlag := flag.String("format", "table", "Output format: table, json")
	sortFlag := flag.String("sort", "name", "Sort order: name, size/selfsize (individual size), excludedepsize (exclusive deps), shareddepsize (shared deps), footprintsize (total footprint), sharedtotal (total size with shared), tag (user vs system)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "goFindInstalledApplications - Analyze installed applications and their disk footprint\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  goFindInstalledApplications [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Validate sort option(s)
	sorts := strings.Split(*sortFlag, ",")
	validSorts := map[string]bool{
		"name":           true,
		"size":           true,
		"selfsize":       true,
		"excludedepsize": true,
		"shareddepsize":  true,
		"footprintsize":  true,
		"total":          true,
		"sharedtotal":    true,
		"tag":            true,
	}

	for _, s := range sorts {
		s = strings.TrimSpace(s)
		if !validSorts[s] {
			log.Fatalf("Invalid sort option: %q (must be one or more of: name, size/selfsize, excludedepsize, shareddepsize, footprintsize, sharedtotal, tag)", s)
		}
	}

	// Validate format option
	switch *formatFlag {
	case "table", "json":
	default:
		log.Fatalf("Invalid format option: %s (must be table or json)", *formatFlag)
	}

	// 2. Run analysis
	result, err := analyzer.Run(*showSystem)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// 3. Sort applications
	sortApplications(result, sorts)

	// 4. Output results
	if *formatFlag == "json" {
		printJSON(result)
	} else {
		printTable(result, *showSystem)
	}
}

func sortApplications(result *analyzer.RunResult, sortByList []string) {
	sort.Slice(result.Apps, func(i, j int) bool {
		appI := result.Apps[i]
		appJ := result.Apps[j]
		footI := result.Footprints[appI.Pkg.Name]
		footJ := result.Footprints[appJ.Pkg.Name]

		for _, sortBy := range sortByList {
			sortBy = strings.TrimSpace(sortBy)
			switch sortBy {
			case "size", "selfsize":
				if footI.SelfSize != footJ.SelfSize {
					return footI.SelfSize > footJ.SelfSize
				}
			case "excludedepsize":
				if footI.ExclusiveSize != footJ.ExclusiveSize {
					return footI.ExclusiveSize > footJ.ExclusiveSize
				}
			case "shareddepsize":
				if footI.SharedSize != footJ.SharedSize {
					return footI.SharedSize > footJ.SharedSize
				}
			case "footprintsize", "total":
				if footI.TotalSize != footJ.TotalSize {
					return footI.TotalSize > footJ.TotalSize
				}
			case "sharedtotal":
				sumI := footI.SelfSize + footI.ExclusiveSize + footI.SharedSize
				sumJ := footJ.SelfSize + footJ.ExclusiveSize + footJ.SharedSize
				if sumI != sumJ {
					return sumI > sumJ
				}
			case "tag":
				if appI.IsSystem != appJ.IsSystem {
					// User-installed ([User]) apps first, then System-installed ([System])
					return !appI.IsSystem
				}
			case "name":
				cmp := strings.Compare(strings.ToLower(appI.Pkg.Name), strings.ToLower(appJ.Pkg.Name))
				if cmp != 0 {
					return cmp < 0
				}
			}
		}

		// Absolute fallback: alphabetical by name
		return strings.Compare(strings.ToLower(appI.Pkg.Name), strings.ToLower(appJ.Pkg.Name)) < 0
	})
}

func formatSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024.0)
	} else if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024.0*1024.0))
	} else {
		return fmt.Sprintf("%.1f GB", float64(b)/(1024.0*1024.0*1024.0))
	}
}

func printTable(result *analyzer.RunResult, showSystem bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	title := "User Applications"
	if showSystem {
		title = "User & System Applications"
	}

	fmt.Fprintf(os.Stdout, "Installed %s:\n", title)
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 125))
	fmt.Fprintf(w, "Tag\tApplication\tType\tInstall Path\tSelf Size\tExclusive Deps\tShared Deps\tTotal Footprint\tSummary\n")

	var totalSelfSize int64
	var totalExclusiveDepsSize int64
	var totalApps int
	var userCount int
	var systemCount int

	for _, app := range result.Apps {
		footprint, exists := result.Footprints[app.Pkg.Name]
		if !exists {
			continue
		}

		tag := "[User]"
		if app.IsSystem {
			tag = "[System]"
			systemCount++
		} else {
			userCount++
		}
		totalApps++

		totalSelfSize += footprint.SelfSize
		totalExclusiveDepsSize += footprint.ExclusiveSize

		pkgName := app.Pkg.Name
		if app.Pkg.IsFlatpak {
			pkgName += " (FP)"
		}

		summary := app.Pkg.Summary
		if len(summary) > 45 {
			summary = summary[:42] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			tag,
			pkgName,
			app.Type,
			app.InstallPath,
			formatSize(footprint.SelfSize),
			formatSize(footprint.ExclusiveSize),
			formatSize(footprint.SharedSize),
			formatSize(footprint.TotalSize),
			summary,
		)
	}

	w.Flush()
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 125))
	fmt.Fprintf(os.Stdout, "Total installed applications shown: %d (User: %d, System: %d)\n", totalApps, userCount, systemCount)
	fmt.Fprintf(os.Stdout, "Total cumulative size: %s (With exclusive dependencies: %s)\n", formatSize(totalSelfSize), formatSize(totalSelfSize+totalExclusiveDepsSize))
}

type JSONAppEntry struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	Type             string `json:"type"`
	Tag              string `json:"tag"`
	InstallPath      string `json:"install_path"`
	SelfSize         int64  `json:"self_size"`
	SelfSizeStr      string `json:"self_size_formatted"`
	ExclusiveDeps    int64  `json:"exclusive_deps_size"`
	ExclusiveDepsStr string `json:"exclusive_deps_size_formatted"`
	SharedDeps       int64  `json:"shared_deps_size"`
	SharedDepsStr    string `json:"shared_deps_size_formatted"`
	TotalFootprint   int64  `json:"total_footprint_size"`
	TotalFootprintStr string `json:"total_footprint_formatted"`
	Summary          string `json:"summary"`
	IsFlatpak        bool   `json:"is_flatpak"`
}

type JSONOutput struct {
	Applications []*JSONAppEntry `json:"applications"`
	Stats        struct {
		TotalShown           int    `json:"total_shown"`
		UserCount            int    `json:"user_count"`
		SystemCount          int    `json:"system_count"`
		TotalSelfSize        int64  `json:"total_self_size_bytes"`
		TotalSelfSizeStr     string `json:"total_self_size_formatted"`
		TotalFootprintSize   int64  `json:"total_footprint_size_bytes"`
		TotalFootprintSizeStr string `json:"total_footprint_size_formatted"`
	} `json:"stats"`
}

func printJSON(result *analyzer.RunResult) {
	out := JSONOutput{}
	out.Applications = make([]*JSONAppEntry, 0)

	var totalSelf int64
	var totalExclusive int64
	var userCount int
	var systemCount int

	for _, app := range result.Apps {
		foot, exists := result.Footprints[app.Pkg.Name]
		if !exists {
			continue
		}

		tag := "User"
		if app.IsSystem {
			tag = "System"
			systemCount++
		} else {
			userCount++
		}

		totalSelf += foot.SelfSize
		totalExclusive += foot.ExclusiveSize

		entry := &JSONAppEntry{
			Name:              app.Pkg.Name,
			Version:           app.Pkg.Version,
			Type:              string(app.Type),
			Tag:               tag,
			InstallPath:       app.InstallPath,
			SelfSize:          foot.SelfSize,
			SelfSizeStr:       formatSize(foot.SelfSize),
			ExclusiveDeps:     foot.ExclusiveSize,
			ExclusiveDepsStr:  formatSize(foot.ExclusiveSize),
			SharedDeps:        foot.SharedSize,
			SharedDepsStr:     formatSize(foot.SharedSize),
			TotalFootprint:    foot.TotalSize,
			TotalFootprintStr: formatSize(foot.TotalSize),
			Summary:           app.Pkg.Summary,
			IsFlatpak:         app.Pkg.IsFlatpak,
		}

		out.Applications = append(out.Applications, entry)
	}

	out.Stats.TotalShown = len(out.Applications)
	out.Stats.UserCount = userCount
	out.Stats.SystemCount = systemCount
	out.Stats.TotalSelfSize = totalSelf
	out.Stats.TotalSelfSizeStr = formatSize(totalSelf)
	out.Stats.TotalFootprintSize = totalSelf + totalExclusive
	out.Stats.TotalFootprintSizeStr = formatSize(totalSelf + totalExclusive)

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	fmt.Println(string(data))
}
