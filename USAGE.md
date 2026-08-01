# Usage Guide: gfia (goFindInstalledApplications)

`gfia` is a command-line tool that lists installed applications and analyzes their disk size footprints, separating the core application size from its exclusive and shared dependencies.

---

## 1. Building the Application

Compile the Go application to create the `gfia` executable:

```bash
go build -o gfia ./cmd/goFindInstalledApplications/main.go
```

---

## 2. Command Reference

### Base Syntax
```bash
./gfia [options]
```

### Options

| Flag | Description | Values / Defaults |
| :--- | :--- | :--- |
| `-system` | Include system-installed applications (filtered to ignore development libraries and core system headers). | `bool` (default: `false`, only user-installed apps shown) |
| `-format` | Format of the command line output. | `table`, `json` (default: `table`) |
| `-sort` | Field to sort the output by. Sorting is descending for sizes and alphabetical for names. | `name`, `size`, `selfsize`, `excludedepsize`, `shareddepsize`, `footprintsize`, `sharedtotal`, `tag` (default: `name`) |
| `-help` or `--help` | Prints the usage summary. | N/A |

---

## 3. Sort Options Explained

* **`name`** (default): Sorts applications alphabetically by their name.
* **`size`** or **`selfsize`**: Sorts applications by their individual package size (`Self Size` column).
* **`excludedepsize`**: Sorts applications by the size of dependencies they alone require (`Exclusive Deps` column).
* **`shareddepsize`**: Sorts applications by the size of dependencies they share with other user-installed applications (`Shared Deps` column).
* **`footprintsize`** (or `total`): Sorts applications by their actual disk footprint (`Total Footprint` column).
* **`sharedtotal`**: Sorts applications by the total size of the application and all its dependencies (exclusive + shared).
* **`tag`**: Groups user-installed applications (`[User]`) at the top, and system-installed applications (`[System]`) at the bottom, sorting them alphabetically within each group.

---

## 4. Size Metrics Reference

When listing applications, the tool displays four size fields:

1. **Self Size**: The installed size of the application's package itself (e.g. `/usr/bin/firefox` binary and assets).
2. **Exclusive Deps**: The size of dependencies required **only** by this application. If you uninstall this app, these dependencies can be safely deleted (e.g., using `dnf autoremove`).
3. **Shared Deps**: The size of dependencies required by this application that are **also shared with other applications** (e.g. `glibc`). These cannot be deleted on uninstallation because other programs still need them.
4. **Total Footprint**: Calculated as `Self Size` + `Exclusive Deps`. This represents the **exact amount of disk space you will reclaim** if you uninstall this application and run package cleanup.

---

## 5. Usage Examples

### View User-Installed Apps (Default Name Sorting)
Lists all user-installed applications sorted alphabetically:
```bash
./gfia
```

### Find the Largest Applications by Footprint
Lists user-installed apps sorted from largest to smallest disk footprint (Self + Exclusive):
```bash
./gfia -sort footprintsize
```

### Find Apps with the Largest Exclusive Dependency Chains
Lists user-installed apps sorted by the size of their exclusive dependencies:
```bash
./gfia -sort excludedepsize
```

### Include Pre-installed System Applications
Shows both user-installed and pre-installed system applications (like `firewall-config` or `falkon`), while filtering out base system libraries:
```bash
./gfia -system
```

### Export Results to JSON
Outputs the complete analysis as JSON, which can be piped to other command line tools like `jq` or written to a file:
```bash
./gfia -format json > report.json
```

---

## 6. Distro & Platform Support
Currently supports:
- **Fedora/RHEL** (via `dnf5` toolchain)
- **Debian/Ubuntu** (via `dpkg`/`apt` toolchain)
- **Arch Linux** (via `pacman` toolchain)
- **Flatpak Applications** (runtimes are analyzed as shared dependencies)
