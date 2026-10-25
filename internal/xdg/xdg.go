// Package xdg answers the XDG Base Directory Specification's questions the
// two halves of this app both have to ask: where the per-user data directory
// is, and where the system's data directories are.
//
// It exists because there used to be two answers. `internal/apps` computed the
// Linux desktop-entry roots one way (the reference's precedence order, reading
// `$XDG_DATA_HOME` and `$XDG_DATA_DIRS`) and `internal/inventory` computed them
// another (the fixed system roots reversed, the environment's directories
// first, and two package-export roots missing). One binary, two answers to
// "where does a Linux publish its applications" — so a local override under
// `/usr/local/share/applications` won in the launcher and lost in the tool
// list, for the same desktop entry.
//
// Everything here is a pure function of its arguments, so the whole table can
// be pinned from any host, and `DesktopRootsFromEnv` is the only function that
// reads the environment.
package xdg

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DataHome is the per-user data directory: `$XDG_DATA_HOME` when it is set to
// an absolute path, and `~/.local/share` otherwise — the specification's own
// fallback for a relative or missing value.
func DataHome() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" && filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(homeDir(), ".local", "share")
}

// DataDirs is `$XDG_DATA_DIRS` split into its entries, with empty entries
// dropped. The specification's default (`/usr/local/share:/usr/share`) is not
// substituted here: the fixed system roots are part of DesktopRoots' table, so
// a machine with the variable unset still gets them.
func DataDirs() []string {
	var dirs []string
	for _, dir := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if dir = strings.TrimSpace(dir); dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// DesktopRoots is where a Linux publishes its desktop entries, in **precedence
// order**: the user's own directory first, then the system's, then the package
// exports, then whatever else `$XDG_DATA_DIRS` advertises.
//
// The order is the reference's (`roots()`, `main:src-tauri/src/commands/apps/
// linux.rs:186-205`), and it is the half of the rule that makes a user's
// override win: a `.desktop` file under `$XDG_DATA_HOME` shadows the system
// copy of the same id, which only works if the user's directory is scanned
// first (see `apps.desktopFileID`). `$XDG_DATA_HOME` and `$XDG_DATA_DIRS` are
// read here rather than assumed to be `~/.local/share` and `/usr/share`: a
// custom prefix, a container, or a distribution that publishes its
// applications somewhere else is where a hard-coded list finds nothing.
//
// `/usr/local/share` comes **before** `/usr/share`, which is the specification's
// own default order and the order that lets a local administrator's override
// win over the distribution's copy. A `dataDir` that is already in the list
// (the default pair usually is) contributes nothing, exactly as the
// reference's `if !roots.contains(&candidate)` does.
func DesktopRoots(dataHome string, dataDirs []string) []string {
	var roots []string
	add := func(dir string) {
		if dir == "" {
			return
		}
		for _, existing := range roots {
			if existing == dir {
				return
			}
		}
		roots = append(roots, dir)
	}
	if dataHome != "" {
		add(filepath.Join(dataHome, "applications"))
		add(filepath.Join(dataHome, "flatpak", "exports", "share", "applications"))
	}
	add("/usr/local/share/applications")
	add("/usr/share/applications")
	add("/var/lib/flatpak/exports/share/applications")
	add("/var/lib/snapd/desktop/applications")
	for _, dataDir := range dataDirs {
		if dataDir = strings.TrimSpace(dataDir); dataDir != "" {
			add(filepath.Join(dataDir, "applications"))
		}
	}
	return roots
}

// DesktopRootsFromEnv is DesktopRoots applied to this machine's environment.
// It is the only function in this package that reads it.
func DesktopRootsFromEnv() []string {
	return DesktopRoots(DataHome(), DataDirs())
}

// DesktopWalkDepth is how deep below a desktop root the walk descends for a
// `.desktop` entry: distributions nest entries a couple of levels deep
// (`kde4/`, `screensavers/`), and the reference implementation caps the walk at
// three levels (`collect_entries`/`collect_desktop_paths`,
// `main:src-tauri/src/commands/apps/linux.rs`, `if depth > 3 { return; }`). The
// launcher's application scan and the tool inventory both pass this one value,
// so a nested entry is a row in both lists or in neither.
const DesktopWalkDepth = 3

// WalkDesktopEntries visits every `*.desktop` file below root, in the
// order the directories are read. Distributions nest entries a couple of levels
// deep (`kde4/`, `screensavers/`), so the walk descends into subdirectories up
// to maxDepth levels below root (0 = only root's own files) — the same depth
// cap the reference implementation applies (`collect_entries`/
// `collect_desktop_paths`, `main:src-tauri/src/commands/apps/linux.rs`, whose
// guard is `if depth > 3 { return; }`).
//
// A directory entry is judged with `os.Stat`, so a symlinked directory is
// descended into exactly as the reference's `Path::is_dir()` does; the depth cap
// keeps a symlink cycle finite. This is the one walk both the launcher's
// application scan (`internal/apps`) and the tool inventory
// (`internal/inventory`) read a desktop entry through, so a nested entry is
// either a row in both lists or in neither.
func WalkDesktopEntries(root string, maxDepth int, visit func(path string)) {
	WalkEntries(root, maxDepth, func(path string, d fs.DirEntry, isDir bool) bool {
		if !isDir && filepath.Ext(d.Name()) == ".desktop" {
			visit(path)
		}
		return true
	})
}

// WalkEntries visits every entry below root, depth-first, in the order the
// directories are read, descending into directories up to maxDepth levels below
// root (0 = only root's own entries) — the shape the reference implementation's
// collectors have (`collect_entries`, `collect_shortcuts`, `collect_apps` in
// `main:src-tauri/src/commands/apps/`), each with its own depth cap.
//
// It is the one walk the application scans share: the Linux desktop-entry scans
// in `apps` and `inventory`, the macOS `.app` scan, the Windows `.lnk` scan, and
// the cache's signature walk all read the filesystem through it. So "how deep
// does a scan look" and "does a symlinked folder count" have one answer per
// platform rather than one per collector — which is what they used to have:
// the Linux scans stopped at three levels while the macOS and Windows ones had
// no cap at all, and only the Linux one followed a symlink.
//
// visit is handed the entry's path, its `fs.DirEntry`, and whether it is a
// directory (symlinks resolved, see isDirectory); returning true descends into
// it. The depth cap keeps a symlink cycle finite.
func WalkEntries(root string, maxDepth int, visit func(path string, d fs.DirEntry, isDir bool) bool) {
	walkTree(root, 0, maxDepth, visit)
}

func walkTree(dir string, depth, maxDepth int, visit func(path string, d fs.DirEntry, isDir bool) bool) {
	if depth > maxDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		isDir := directoryOf(path, entry)
		if visit(path, entry, isDir) && isDir {
			walkTree(path, depth+1, maxDepth, visit)
		}
	}
}

// directoryOf is the walk's one "is this a directory" probe; it is a variable
// so a pin can drive the walk's descent decision without creating a symlink,
// which needs a privilege Windows does not always grant. Production never
// replaces it.
var directoryOf = isDirectory

// isDirectory reports whether path is a directory, following a symlink: a
// symlinked directory holds entries the walk has to see, the same way the
// reference's `Path::is_dir()` follows it.
func isDirectory(path string, entry fs.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// homeDir is the user's home, `/` when the platform will not say — the same
// fallback `apps.homeDir` has always used, so a machine with no `$HOME` keeps
// resolving to the same paths it did before this package existed.
func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "/"
}
