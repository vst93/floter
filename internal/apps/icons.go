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
	// iconPending is the set of applications whose icon a background
	// extraction is producing right now, so a redraw does not start a second
	// one for the same row.
	iconPending = map[string]bool{}
	// iconInvalidate asks the interface for another frame once an icon
	// landed. It is set by the shell (SetIconInvalidate); nil simply means the
	// next frame that would draw the row draws it.
	iconInvalidate func()
)

// iconExtractions bounds the background extractions: each one is a process
// launch, and a scan of hundreds of applications must not start hundreds of
// them at once.
var iconExtractions = make(chan struct{}, 2)

// SetIconInvalidate installs the callback that asks the interface for a frame
// when a background icon extraction finishes. Safe from any goroutine.
func SetIconInvalidate(fn func()) {
	iconMu.Lock()
	iconInvalidate = fn
	iconMu.Unlock()
}

// Icon returns an application's icon as PNG bytes, or nil when it has none
// this build can read.
//
// An icon the platform has to *extract* (a Windows shortcut's, which lives in
// the shell's own database and takes a PowerShell launch) is never extracted on
// the calling goroutine: this is the drawing thread, and a launch per row froze
// the whole window for seconds on the first summon (the report 「很多界面经常
// 卡死」). The first call starts the extraction, returns nil, and asks for a
// frame when the bytes arrive; IconPending reports that a row is waiting, so its
// reader does not remember the empty answer as final.
func Icon(app App) []byte {
	if app.Path == "" {
		return nil
	}
	iconMu.Lock()
	cached, ok := iconCache[app.Path]
	pending := iconPending[app.Path]
	iconMu.Unlock()
	if ok {
		return cached
	}
	if deferredIcon(app) {
		if !pending {
			extractIconAsync(app)
		}
		return nil
	}
	icon := readIcon(app)
	iconMu.Lock()
	iconCache[app.Path] = icon
	iconMu.Unlock()
	return icon
}

// IconPending reports whether an application's icon is being extracted in the
// background: an empty Icon now is not the final answer for that row.
func IconPending(app App) bool {
	if app.Path == "" {
		return false
	}
	iconMu.Lock()
	defer iconMu.Unlock()
	return iconPending[app.Path]
}

// extractIconAsync runs one platform extraction off the drawing thread and asks
// for a frame when it lands. The path is marked pending first, so the frames in
// between ask for the icon without starting a second extraction.
func extractIconAsync(app App) {
	iconMu.Lock()
	if iconPending[app.Path] {
		iconMu.Unlock()
		return
	}
	iconPending[app.Path] = true
	iconMu.Unlock()
	go func() {
		iconExtractions <- struct{}{}
		icon := readIcon(app)
		<-iconExtractions
		iconMu.Lock()
		iconCache[app.Path] = icon
		delete(iconPending, app.Path)
		invalidate := iconInvalidate
		iconMu.Unlock()
		if invalidate != nil && len(icon) > 0 {
			invalidate()
		}
	}()
}

// readIcon reads the icon for one application: a file the platform named, or
// a platform's own extraction.
func readIcon(app App) []byte {
	if app.IconPath != "" {
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
	return extractIcon(app)
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
