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

// homeDir is the user's home, `/` when the platform will not say — the same
// fallback `apps.homeDir` has always used, so a machine with no `$HOME` keeps
// resolving to the same paths it did before this package existed.
func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "/"
}
