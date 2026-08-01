package detector

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"goFindInstalledApplications/pkg/packagemanager"
)

// AppType represents the category of the application.
type AppType string

const (
	TypeDesktop AppType = "Desktop"
	TypeCLI     AppType = "CLI"
	TypeService AppType = "Service"
)

// DetectedApp wraps a package with its classified details.
type DetectedApp struct {
	Pkg         *packagemanager.Package
	Type        AppType
	InstallPath string
	IsSystem    bool
}

// Detector handles classification and filtering of packages.
type Detector struct {
	showSystem bool
}

// NewDetector creates a new Detector.
func NewDetector(showSystem bool) *Detector {
	return &Detector{
		showSystem: showSystem,
	}
}
var coreSystemPackages = map[string]bool{
	"acl":                true,
	"attr":               true,
	"bash":               true,
	"coreutils":          true,
	"iproute":            true,
	"iputils":            true,
	"util-linux":         true,
	"sudo":               true,
	"rpm":                true,
	"dnf":                true,
	"dnf5":               true,
	"shadow-utils":       true,
	"systemd":            true,
	"tar":                true,
	"gzip":               true,
	"bzip2":              true,
	"xz":                 true,
	"grep":               true,
	"sed":                true,
	"gawk":               true,
	"findutils":          true,
	"which":              true,
	"less":               true,
	"filesystem":         true,
	"setup":              true,
	"ncurses":            true,
	"pam":                true,
	"audit":              true,
	"libreport":          true,
	"polkit":             true,
	"dbus":               true,
	"openssh-clients":    true,
	"openssh-server":     true,
	"curl":               true,
	"wget":               true,
	"procps-ng":          true,
	"systemd-resolved":   true,
	"systemd-udev":       true,
	"wsl-setup":          true,
	"lsof":               true,
	"man-db":             true,
	"pciutils":           true,
	"rsync":              true,
	"shadow-utils-subid": true,
	"libc6":              true,
	"dpkg":               true,
	"apt":                true,
	"dash":               true,
	"debianutils":        true,
	"base-files":         true,
	"base-passwd":        true,
	"login":              true,
	"passwd":             true,
	"sysvinit-utils":     true,
	"diffutils":          true,
	"bsdutils":           true,
	"gpgv":               true,
	"sensible-utils":     true,
	"linux":              true,
	"linux-firmware":     true,
	"pacman":             true,
	"iproute2":           true,
	"shadow":             true,
	"gcc-libs":           true,
	"iana-etc":           true,
	"licenses":           true,
	"systemd-sysvcompat": true,
	"device-mapper":      true,
	"cryptsetup":         true,
	"vi":                 true,
}

// DetectAndFilter processes packages and returns the list of detected applications.
func (d *Detector) DetectAndFilter(packages []*packagemanager.Package) ([]*DetectedApp, map[string]bool) {
	var apps []*DetectedApp
	targetAppsSet := make(map[string]bool)

	for _, pkg := range packages {
		// Filter out based on reasons:
		// - If we are only showing user-installed, skip packages that are not ReasonUser.
		// - Dependencies (ReasonDependency) are always skipped, unless they are Flatpak apps
		//   (flatpak.go sets ReasonUser for apps and ReasonDependency for runtimes).
		if pkg.Reason == packagemanager.ReasonDependency && !pkg.IsFlatpak {
			continue
		}

		isSystem := pkg.Reason == packagemanager.ReasonSystem || coreSystemPackages[strings.ToLower(pkg.Name)]

		if !d.showSystem && isSystem {
			continue
		}

		// Apply library and development package filters
		if d.isLibraryOrSystemPackage(pkg) {
			continue
		}

		// Resolve application type and executable path
		appType, installPath := d.resolveAppDetails(pkg)
		if installPath == "" {
			// No executable path or entry point found, so it is not an executable application
			continue
		}

		apps = append(apps, &DetectedApp{
			Pkg:         pkg,
			Type:        appType,
			InstallPath: installPath,
			IsSystem:    isSystem,
		})

		targetAppsSet[pkg.Name] = true
	}

	return apps, targetAppsSet
}

func (d *Detector) isLibraryOrSystemPackage(pkg *packagemanager.Package) bool {
	name := strings.ToLower(pkg.Name)

	// Rule 1: Exclude packages ending with dev suffixes (common in packages)
	devSuffixes := []string{
		"-devel", "-dev", "-headers", "-libs", "-common",
		"-static", "-compat", "-helper", "-debuginfo", "-dbg",
	}
	for _, suffix := range devSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	// Rule 2: Exclude packages starting with "lib" (unless they are flatpaks or have desktop entries)
	if strings.HasPrefix(name, "lib") && !pkg.IsFlatpak {
		hasDesktopFile := false
		for _, path := range pkg.BinaryPaths {
			if strings.HasSuffix(path, ".desktop") {
				hasDesktopFile = true
				break
			}
		}
		if !hasDesktopFile {
			return true
		}
	}

	// Rule 3: Exclude known base system/infrastructure library and packaging tools
	systemExclusions := map[string]bool{
		"glibc":                       true,
		"glibc-common":                true,
		"glibc-minimal-langpack":      true,
		"glibc-gconv-extra":           true,
		"coreutils-common":            true,
		"setup":                       true,
		"filesystem":                  true,
		"tzdata":                      true,
		"systemd":                     true,
		"pam":                         true,
		"shadow-utils":                true,
		"ncurses":                     true,
		"ncurses-base":                true,
		"ncurses-libs":                true,
		"bash-completion":             true,
		"bash-color-prompt":           true,
		"fedora-release":              true,
		"fedora-repos":                true,
		"fedora-gpg-keys":             true,
		"publicsuffix-list-dafsa":     true,
		"adwaita-icon-theme":          true,
		"adwaita-cursor-theme":        true,
		"hicolor-icon-theme":          true,
		"breeze-icon-theme":           true,
		"breeze-cursor-theme":         true,
		"breeze-gtk":                  true,
		"desktop-file-utils":          true,
		"shared-mime-info":            true,
		"ca-certificates":             true,
		"crypto-policies":             true,
		"alternatives":                true,
		"authselect":                  true,
		"authselect-libs":             true,
	}
	if systemExclusions[name] {
		return true
	}

	return false
}

func (d *Detector) resolveAppDetails(pkg *packagemanager.Package) (AppType, string) {
	// If it is a Flatpak application, flatpak.go sets its BinaryPaths to "flatpak run <AppID>"
	if pkg.IsFlatpak && len(pkg.BinaryPaths) > 0 {
		return TypeDesktop, pkg.BinaryPaths[0]
	}

	// Look for a .desktop file in the package binaries to classify as Desktop/GUI
	var desktopPath string
	for _, path := range pkg.BinaryPaths {
		if strings.HasSuffix(path, ".desktop") {
			desktopPath = path
			break
		}
	}

	if desktopPath != "" {
		// Read desktop file to resolve actual Exec command
		execCmd := parseDesktopExec(desktopPath)
		if execCmd != "" {
			return TypeDesktop, execCmd
		}
	}

	// If no desktop file, look for standard CLI binaries
	var binaries []string
	for _, path := range pkg.BinaryPaths {
		if !strings.HasSuffix(path, ".desktop") {
			binaries = append(binaries, path)
		}
	}

	if len(binaries) > 0 {
		// On Windows, registry-installed apps are typically GUI Desktop applications
		bin := binaries[0]
		if strings.Contains(bin, "\\") || strings.HasSuffix(strings.ToLower(bin), ".exe") {
			return TypeDesktop, bin
		}

		// Try to find a binary that matches the package name
		for _, bin := range binaries {
			base := filepath.Base(bin)
			if strings.EqualFold(base, pkg.Name) {
				return TypeCLI, bin
			}
		}
		// Fallback to the first executable binary
		return TypeCLI, binaries[0]
	}

	// If it has systemd files or config scripts but no binaries in PATH, check if it's a service.
	// But since we are looking for "executable user applications", we return empty if there are no binaries.
	return "", ""
}

func parseDesktopExec(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Exec=") {
			cmd := strings.TrimPrefix(line, "Exec=")
			// Strip field codes like %u, %U, %f, %F, %k, %i
			cmd = stripDesktopFieldCodes(cmd)
			return strings.TrimSpace(cmd)
		}
	}
	return ""
}

func stripDesktopFieldCodes(cmd string) string {
	fields := []string{"%u", "%U", "%f", "%F", "%k", "%i", "%c", "%k", "%v"}
	parts := strings.Fields(cmd)
	var cleanParts []string

	for _, part := range parts {
		isFieldCode := false
		for _, field := range fields {
			if strings.EqualFold(part, field) {
				isFieldCode = true
				break
			}
		}
		if !isFieldCode {
			cleanParts = append(cleanParts, part)
		}
	}

	return strings.Join(cleanParts, " ")
}
