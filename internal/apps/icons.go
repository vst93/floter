package apps

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"floter/internal/xdg"
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

// platformExtract is the platform's own extraction, the answer for an
// application whose icon is not a file this build can read. It is a variable
// so the fall-through in readIcon can be pinned from a host whose own platform
// has nothing to extract (the gate runs on Linux).
var platformExtract = extractIcon

// readIcon reads the icon for one application: a file the platform named, or
// a platform's own extraction.
//
// A file that yields no PNG is **not** the end of the answer. An `.icns` may
// hold only the pre-10.7 bitmap entries (`is32`, `il32`, `it32`), which this
// build's reader skips on purpose; the reference hands that same file to
// `/usr/bin/sips` (`main:src-tauri/src/commands/apps/macos.rs:509-530`) and
// gets an icon, so stopping here would show a first letter where it shows an
// icon. The same fall-through covers a file that cannot be read at all.
func readIcon(app App) []byte {
	if app.IconPath != "" {
		if data, err := os.ReadFile(app.IconPath); err == nil {
			if png := iconFromFile(app.IconPath, data); png != nil {
				return png
			}
		}
	}
	return platformExtract(app)
}

// iconFromFile turns one icon file's bytes into PNG, or nil when this build
// cannot read them as an image.
func iconFromFile(path string, data []byte) []byte {
	if strings.HasSuffix(strings.ToLower(path), ".icns") {
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
// is used as it is, and a name is looked for in the pixmap directories and
// then through every theme under every icon base, exactly as the reference's
// `resolve_icon_uncached` does (`main:src-tauri/src/commands/apps/linux.rs:603`).
//
// The two halves of that rule are separate, and both matter:
//
//   - the **name**: some entries spell `Icon=name.png` (or `.svg`), so the
//     value is reduced to the name a theme indexes before anything is looked
//     up. Only a *known icon extension* is dropped — never just the last dot,
//     because icon names carry dots of their own
//     (`org.cachyos.KernelManager.png`, `org.gnome.gedit.png` are real files,
//     and `/usr/share/applications` has two dozen `Icon=org.gnome.X` values).
//     The reference's `Path::file_stem` truncates those to `org.gnome` and so
//     finds nothing; this build keeps the whole name.
//   - the **search path**: pixmaps first, then every theme directory under
//     every base, with the reference's size table and its three layouts. The
//     old build spelled `hicolor` into the directory list and only tried
//     `<base>/<size>/apps`, which is why a Papirus/breeze/Adwaita-only icon
//     resolved to nothing.
//
// One deliberate difference: only `.png` candidates are returned. This build
// renders icons with `ui.DecodeBitmap` (`internal/launcher/launcher.go:506`),
// so a `.svg`/`.xpm` answer would be a file `readIcon` rejects — and a rejected
// answer would shadow the smaller PNG the same name has. Alacritty (whose only
// icon on a stock Arch install is `/usr/share/pixmaps/Alacritty.svg`) keeps its
// initials rather than getting a file that cannot be drawn.
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
	base := iconName(name)
	if base == "" {
		return ""
	}
	file := base + iconExtension
	for _, dir := range pixmapDirs() {
		if path := regularFile(filepath.Join(dir, file)); path != "" {
			return path
		}
	}
	for _, root := range iconBaseDirs() {
		for _, theme := range themesIn(root) {
			if path := findInTheme(filepath.Join(root, theme), base, file); path != "" {
				return path
			}
		}
	}
	return ""
}

// iconExtension is the one image format this build can draw (see
// desktopIconFile). Every candidate below is spelled with it, and the bounded
// recursive walk only accepts it, so a name that exists solely as `.svg`
// resolves to nothing instead of to an undecodable file.
const iconExtension = ".png"

// iconNameExtensions are the extensions an `Icon=` value may be spelled with
// and still name the icon itself: the reference's `ICON_EXTENSIONS` (linux.rs:
// 27), minus the ones this build cannot draw. Dropping `.svg` as well as `.png`
// is what makes `Icon=foo.svg` look for `foo.png` — the entry named an icon,
// and the icon's format is this build's business, not the entry's.
var iconNameExtensions = []string{".png", ".svg", ".xpm"}

// iconName is the name a theme indexes for an `Icon=` value: the value with a
// known icon extension dropped. `Icon=firefox.png`, `Icon=firefox.svg` and
// `Icon=firefox` are the same icon, while `Icon=org.gnome.gedit` keeps its dots
// (`org.gnome.gedit.png` is a real file, and `Path::file_stem` would ask for
// `org.gnome`).
func iconName(name string) string {
	base := filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(base))
	if slices.Contains(iconNameExtensions, ext) {
		if trimmed := base[:len(base)-len(ext)]; trimmed != "" {
			return trimmed
		}
	}
	return base
}

// iconSizes are the theme subdirectories tried, in the reference's order
// (`ICON_SIZES`, `main:src-tauri/src/commands/apps/linux.rs:21-24`): largest
// first, so a row gets the sharpest icon the machine has, with `scalable`
// after 48x48 exactly where the reference puts it.
var iconSizes = []string{
	"512x512", "256x256", "192x192", "128x128", "96x96", "72x72", "64x64",
	"48x48", "scalable", "36x36", "32x32", "24x24", "16x16",
}

// preferredThemes are searched before the rest of a base's themes, in the
// reference's order (`PREFERRED_THEMES`, linux.rs:26-30).
var preferredThemes = []string{"hicolor", "Adwaita", "breeze", "Papirus", "gnome", "default"}

// pixmapDirs are the flat icon directories, most specific first — the
// reference's `pixmap_dirs` (linux.rs:658). They are searched **before** the
// theme bases, which is the reference's order: a name in a pixmap directory is
// an explicit flat install of that one icon.
func pixmapDirs() []string {
	dirs := []string{filepath.Join(xdg.DataHome(), "pixmaps")}
	return append(dirs, "/usr/share/pixmaps")
}

// iconBaseDirs are the directories whose subdirectories are themes, in the
// reference's `icon_base_dirs` order (linux.rs:637). `$XDG_DATA_HOME` and
// `$XDG_DATA_DIRS` come from `internal/xdg` — the one place in this build that
// answers where a Linux publishes its data — rather than a second hard-coded
// `~/.local/share`.
func iconBaseDirs() []string {
	var dirs []string
	add := func(dir string) {
		if dir == "" {
			return
		}
		for _, existing := range dirs {
			if existing == dir {
				return
			}
		}
		dirs = append(dirs, dir)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".icons"))
	}
	dataHome := xdg.DataHome()
	add(filepath.Join(dataHome, "icons"))
	add(filepath.Join(dataHome, "flatpak", "exports", "share", "icons"))
	add("/usr/local/share/icons")
	add("/usr/share/icons")
	add("/var/lib/flatpak/exports/share/icons")
	for _, dataDir := range xdg.DataDirs() {
		add(filepath.Join(dataDir, "icons"))
	}
	return dirs
}

// themesIn is the theme directories inside one base, preferred themes first
// and the rest of the base's directories after them — the reference's
// `themes_in` (linux.rs:667). The remainder is sorted by name, which the
// reference does not do (it keeps `read_dir`'s order): the sort is what makes
// the answer the same on two machines with the same themes, and the preferred
// list — the themes that actually ship icons — is ordered identically on both
// sides.
func themesIn(base string) []string {
	var themes []string
	for _, theme := range preferredThemes {
		if isDir(filepath.Join(base, theme)) {
			themes = append(themes, theme)
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return themes
	}
	var rest []string
	for _, entry := range entries {
		name := entry.Name()
		if !isDir(filepath.Join(base, name)) || slices.Contains(themes, name) {
			continue
		}
		rest = append(rest, name)
	}
	sort.Strings(rest)
	return append(themes, rest...)
}

// findInTheme is the reference's `find_in_theme` (linux.rs:685) over PNG files
// only: the layouts the common themes use (hicolor/Adwaita spell the size
// `48x48`, breeze-style themes `48`), then a theme's flat `apps/` directory,
// then a bounded walk for the unusual layouts.
func findInTheme(themeDir, name, file string) string {
	if !isDir(themeDir) {
		return ""
	}
	for _, size := range iconSizes {
		numeric := size
		if index := strings.IndexByte(size, 'x'); index >= 0 {
			numeric = size[:index]
		}
		for _, dir := range []string{
			filepath.Join(themeDir, size, "apps"),
			filepath.Join(themeDir, "apps", size),
			filepath.Join(themeDir, "apps", numeric),
		} {
			if path := regularFile(filepath.Join(dir, file)); path != "" {
				return path
			}
		}
	}
	for _, candidate := range []string{
		filepath.Join(themeDir, "apps", file),
		filepath.Join(themeDir, file),
	} {
		if path := regularFile(candidate); path != "" {
			return path
		}
	}
	return searchIconDir(themeDir, name)
}

// searchIconDepth is how deep the unusual-layout walk descends below a theme
// directory: the reference's `if depth > 3 { return None; }` (linux.rs:727).
const searchIconDepth = 3

// iconIndexes memoizes one theme's name→path table, keyed by the theme
// directory. The walk it replaces is the expensive half of a **miss**: a name
// that is nowhere has to walk every theme under every base, and most rows are
// names this machine does not have. The reference memoizes the answer per
// `Icon=` value (`icon_cache`, linux.rs:581); indexing the theme instead makes
// the *first* miss pay the walk and every later one a map lookup, which is the
// same trade with a table that cannot go stale per name.
//
// Like the reference's table it is never invalidated: a theme installed while
// the app is running shows up on the next start.
var (
	iconIndexMu sync.Mutex
	iconIndexes = map[string]map[string]string{}
)

// searchIconDir is the bounded walk for a theme whose layout none of the fast
// paths match: the files of a directory first, then its subdirectories, to
// depth `searchIconDepth`. The walk itself lives in `indexIconDir`, so a theme
// is walked once and then answered from the table.
func searchIconDir(themeDir, name string) string {
	iconIndexMu.Lock()
	index, ok := iconIndexes[themeDir]
	iconIndexMu.Unlock()
	if !ok {
		index = map[string]string{}
		indexIconDir(themeDir, index, 0)
		iconIndexMu.Lock()
		iconIndexes[themeDir] = index
		iconIndexMu.Unlock()
	}
	return index[name]
}

// indexIconDir records the first PNG each name has in a depth-first walk of the
// theme: files before subdirectories, subdirectories in name order — the
// reference's `search_recursive` order (linux.rs:730), so the table's answer is
// the walk's answer.
func indexIconDir(dir string, index map[string]string, depth int) {
	if depth > searchIconDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var subdirs []string
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if isDir(path) {
			subdirs = append(subdirs, path)
			continue
		}
		ext := filepath.Ext(entry.Name())
		if !strings.EqualFold(ext, iconExtension) {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), ext)
		if _, seen := index[stem]; !seen {
			index[stem] = path
		}
	}
	for _, subdir := range subdirs {
		indexIconDir(subdir, index, depth+1)
	}
}

// resetIconCaches drops the per-theme tables. It is for tests, which build a
// theme tree, resolve a name, and then add a file to the same tree.
func resetIconCaches() {
	iconIndexMu.Lock()
	iconIndexes = map[string]map[string]string{}
	iconIndexMu.Unlock()
}

// regularFile is the path when it names an existing file (a symlinked file
// counts, exactly as the reference's `Path::is_file`), and empty otherwise.
func regularFile(path string) string {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

// isDir reports whether path is a directory, following symlinks as the
// reference's `Path::is_dir` does.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
