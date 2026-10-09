package apps

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Application icons, for the launcher's rows.
//
// macOS bundles name theirs in `Info.plist` (`CFBundleIconFile`, an `.icns`
// container whose largest PNG entry is the icon). Linux desktop entries name
// theirs with `Icon=`, which is a path or a theme name; this build resolves
// absolute paths and the PNG files the common theme and pixmap directories
// hold, and shows nothing for a name it cannot find. Windows shortcuts carry
// their icon inside the shell's own database, which this build does not read.
//
// Icons are read and decoded lazily — a scan of five hundred applications must
// not read five hundred icons — and the PNG bytes are cached, so a redraw is a
// map lookup.

var (
	iconMu    sync.Mutex
	iconCache = map[string][]byte{}
)

// Icon returns an application's icon as PNG bytes, or nil when it has none
// this build can read.
func Icon(app App) []byte {
	if app.Path == "" {
		return nil
	}
	iconMu.Lock()
	cached, ok := iconCache[app.Path]
	iconMu.Unlock()
	if ok {
		return cached
	}
	icon := readIcon(app)
	iconMu.Lock()
	iconCache[app.Path] = icon
	iconMu.Unlock()
	return icon
}

// readIcon reads the icon for one application, by platform.
func readIcon(app App) []byte {
	if app.IconPath == "" {
		return nil
	}
	data, err := os.ReadFile(app.IconPath)
	if err != nil {
		return nil
	}
	if strings.HasSuffix(strings.ToLower(app.IconPath), ".icns") {
		return LargestPNGFromICNS(data)
	}
	if !isPNG(data) {
		return nil
	}
	return data
}

// isPNG reports whether data is a PNG image.
func isPNG(data []byte) bool {
	return len(data) > 8 && string(data[1:4]) == "PNG"
}

// iconFileIn is the icon an `.icns` file in a directory names: the file
// `CFBundleIconFile` points at, or the bundle's only icon.
func iconFileIn(dir, name string) string {
	if name != "" {
		if !strings.HasSuffix(strings.ToLower(name), ".icns") {
			name += ".icns"
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".icns") {
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}

// desktopIconFile resolves a Linux desktop entry's `Icon=`: an absolute path
// is used as it is, and a name is looked for as a PNG in the theme and pixmap
// directories the desktop uses.
func desktopIconFile(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		if info, err := os.Stat(name); err == nil && !info.IsDir() {
			return name
		}
		return ""
	}
	base := filepath.Base(name)
	for _, dir := range iconDirs() {
		for _, size := range iconSizes {
			candidate := filepath.Join(dir, size, "apps", base+".png")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		candidate := filepath.Join(dir, base+".png")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// iconSizes are the theme subdirectories tried, largest first so a row gets
// the sharpest icon the machine has.
var iconSizes = []string{"256x256", "128x128", "64x64", "48x48", "32x32", "scalable"}

// iconDirs are the icon roots, most specific first.
func iconDirs() []string {
	home, _ := os.UserHomeDir()
	var dirs []string
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "share", "icons", "hicolor"),
			filepath.Join(home, ".local", "share", "icons"),
			filepath.Join(home, ".local", "share", "pixmaps"),
		)
	}
	dirs = append(dirs,
		"/usr/share/icons/hicolor",
		"/usr/local/share/icons/hicolor",
		"/usr/share/icons",
		"/usr/local/share/icons",
		"/usr/share/pixmaps",
	)
	return dirs
}
