# goFindInstalledApplications (gfia)

A Go-based command-line tool that lists installed user applications and analyzes their disk footprint, separating the core application size from its exclusive and shared dependencies.

Currently supports:
- **Fedora/RHEL** (via DNF5)
- **Debian/Ubuntu** (via Apt/DPKG)
- **Arch Linux** (via Pacman)
- **Flatpaks** (cross-distro)
- **Windows** (via native Registry queries)

---

## How It Works

1. **Detection**:
   - The tool lists all installed packages and runtimes on the system.
   - It filters out libraries and dependency packages (such as packages starting with `lib` or development headers `-devel`), focusing only on user-facing executable applications.
   - It scans package files to classify applications into **Desktop (GUI)** and **CLI** tools, resolving their main execution binaries or `.desktop` launch commands.

2. **Footprint Graph Analysis**:
   - Constructs a complete package dependency graph in memory.
   - For each user application, it calculates:
     - **Self Size**: The installed disk size of the package itself.
     - **Exclusive Deps**: The size of dependencies transitively required *only* by this application. (Removing the app would allow auto-removing these dependencies).
     - **Shared Deps**: The size of dependencies shared with other user-installed applications.
     - **Total Footprint**: The sum of Self Size + Exclusive Deps size (representing the total space that would be freed if you uninstalled this application).

---

## Building and Running

### Prerequisites
- Go 1.22 or higher
- Linux distribution with DNF5, APT/DPKG, Pacman, or Flatpak support, OR Windows 10/11

### Build
Compile the binary using:
```bash
go build -o gfia ./cmd/goFindInstalledApplications/main.go
```

### Run
Run the compiled binary (use `./gfia.exe` on Windows):
```bash
./gfia
```

---

## Command-Line Options

The tool supports several options:

* **`-system`**: Include system-installed applications as well as user-installed applications (tagged as `[System]` or `[User]`). Common libraries are still filtered out.
* **`-format <table|json>`**: Select the output format (default is `table`). `json` is useful for scripting and programmatic ingestion.
* **`-sort <name|size|selfsize|excludedepsize|shareddepsize|footprintsize|sharedtotal|tag>`**: Sort the applications list:
  - `name`: Sort alphabetically by name (default).
  - `size` or `selfsize`: Sort by individual package size descending.
  - `excludedepsize`: Sort by exclusive dependency size descending.
  - `shareddepsize`: Sort by shared dependency size descending.
  - `footprintsize`: Sort by total footprint size descending.
  - `sharedtotal`: Sort by total size including all shared dependencies descending.
  - `tag`: Sort by installation source tag (User-installed first, then System-installed).

### Examples

**Default view (only user-installed apps):**
```bash
./gfia
```

**Sort by total footprint size descending:**
```bash
./gfia -sort footprintsize
```

**Sort by individual application size descending:**
```bash
./gfia -sort size
```
