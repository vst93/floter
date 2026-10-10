package inventory

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// The per-platform sources: the fixed OS roots outside the search path
// (desktop entries, flatpak/snap/nix exports, Homebrew, Scoop, WinGet),
// each with the source a candidate names. Best-effort throughout: a
// directory that is not there contributes nothing.

// discoverLinux reads the desktop entries and the Linux package exports.
func discoverLinux(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality)) {
	mergeEntries(merge, linuxDesktopRoots(), SourceDesktop, QualityNativeSupport)
	if home, err := os.UserHomeDir(); err == nil {
		discoverExecutableDirectory(merge, filepath.Join(home, ".local", "share", "flatpak", "exports", "bin"), SourceFlatpak, QualityNativeSupport)
		discoverExecutableDirectory(merge, filepath.Join(home, ".nix-profile", "bin"), SourceNix, QualityNativeSupport)
	}
	discoverExecutableDirectory(merge, "/var/lib/flatpak/exports/bin", SourceFlatpak, QualityNativeSupport)
	discoverExecutableDirectory(merge, "/snap/bin", SourceSnap, QualityNativeSupport)
	discoverExecutableDirectory(merge, "/nix/var/nix/profiles/default/bin", SourceNix, QualityNativeSupport)
}

// linuxDesktopRoots is where a Linux publishes its desktop entries:
// XDG_DATA_HOME (or ~/.local/share) first, then the system's data dirs,
// then the flatpak exports beside them.
func linuxDesktopRoots() []string {
	var roots []string
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" || !filepath.IsAbs(dataHome) {
		if home, err := os.UserHomeDir(); err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	if dataHome != "" {
		roots = append(roots,
			filepath.Join(dataHome, "applications"),
			filepath.Join(dataHome, "flatpak", "exports", "share", "applications"),
		)
	}
	for _, dataDir := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if dataDir == "" {
			continue
		}
		roots = append(roots,
			filepath.Join(dataDir, "applications"),
			filepath.Join(dataDir, "flatpak", "exports", "share", "applications"),
		)
	}
	if runtime.GOOS == "linux" {
		roots = append(roots, "/usr/share/applications", "/usr/local/share/applications")
	}
	return roots
}

// mergeEntries reads desktop entries in from a list of directories: each
// entry's `Name` and `Comment` are the description discovery carries.
func mergeEntries(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality), directories []string, source DiscoverySource, quality DiscoveryQuality) {
	seen := map[string]bool{}
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			if seen[normalized(path)] {
				continue
			}
			seen[normalized(path)] = true
			name, comment, exec, ok := readDesktopEntry(path)
			if !ok || exec == "" {
				continue
			}
			merge(exec, name, comment, source, quality)
		}
	}
}

// readDesktopEntry reads the fields a discovery needs off a desktop entry:
// Name, Comment and Exec. The Exec's field codes (`%f`, `%u`, …) are
// dropped — the candidate is the bare command. Best-effort: a file that
// does not parse yields nothing.
func readDesktopEntry(path string) (name, comment, exec string, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", false
	}
	inMain := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "["):
			inMain = line == "[Desktop Entry]"
		case inMain:
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			switch key {
			case "Name":
				if name == "" {
					name = value
				}
			case "Comment":
				if comment == "" {
					comment = value
				}
			case "Exec":
				exec = desktopExecCommand(value)
			case "NoDisplay":
				if value == "true" {
					return "", "", "", false
				}
			}
		}
	}
	return name, comment, exec, name != "" && exec != ""
}

// desktopExecCommand is an Exec line's first token, with the field codes
// a desktop entry passes its arguments through dropped (`%f %F %u %U %i %c %k`).
func desktopExecCommand(value string) string {
	tokens := strings.Fields(value)
	for _, token := range tokens {
		if strings.HasPrefix(token, "%") {
			continue
		}
		return token
	}
	return ""
}

// discoverMacOS reads Homebrew's bins and the applications LaunchServices
// knows.
func discoverMacOS(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality)) {
	for _, directory := range []string{"/usr/local/bin", "/opt/homebrew/bin"} {
		discoverExecutableDirectory(merge, directory, SourceBrew, QualityNativeSupport)
	}
	home, _ := os.UserHomeDir()
	for _, directory := range []string{"/Applications", "/System/Applications", filepath.Join(home, "Applications")} {
		discoverMacOSApps(merge, directory)
	}
}

// discoverMacOSApps reads an applications directory's bundles: the
// executable inside Contents/MacOS is the candidate, the bundle's name its
// description.
func discoverMacOSApps(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality), directory string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".app") {
			continue
		}
		bundleName := strings.TrimSuffix(entry.Name(), ".app")
		bin := filepath.Join(directory, entry.Name(), "Contents", "MacOS")
		bins, err := os.ReadDir(bin)
		if err != nil {
			continue
		}
		for _, executable := range bins {
			if executable.IsDir() {
				continue
			}
			path := filepath.Join(bin, executable.Name())
			if !isExecutable(path) {
				continue
			}
			merge(path, executable.Name(), bundleName, SourceLaunchService, QualityNativeSupport)
		}
	}
}

// discoverWindows reads Chocolatey's, Scoop's and WinGet's shims.
func discoverWindows(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality)) {
	discoverExecutableDirectory(merge, `C:\ProgramData\chocolatey\bin`, SourceChocolatey, QualityNativeSupport)
	if home, err := os.UserHomeDir(); err == nil {
		discoverExecutableDirectory(merge, filepath.Join(home, "scoop", "shims"), SourceScoop, QualityNativeSupport)
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		discoverExecutableDirectory(merge, filepath.Join(localAppData, "Microsoft", "WinGet", "Links"), SourceWinGet, QualityNativeSupport)
	}
}

// LookPath is the inventory's own answer to `exec.LookPath`, for the
// caller that asks where a discovered tool lives: the first directory of
// the search path that carries it.
func LookPath(name string) (string, bool) {
	for _, directory := range strings.Split(os.Getenv("PATH"), string(filepath.ListSeparator)) {
		if directory == "" {
			continue
		}
		candidate := filepath.Join(directory, name)
		if isExecutable(candidate) {
			return candidate, true
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, true
	}
	return "", false
}
