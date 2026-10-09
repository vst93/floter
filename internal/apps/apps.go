// Package apps finds the applications installed on the machine, for the
// launcher's search: .app bundles on macOS, .desktop entries on Linux, Start
// Menu shortcuts on Windows.
//
// The scan is pure I/O over explicit roots, so it is testable with a
// fixture directory on any platform; each format's reader is exported so
// the tests can pin it wherever they run.
package apps

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/egoist/mygo"
)

// maxApps bounds a scan, so a pathological directory tree cannot make the
// launcher's list enormous.
const maxApps = 500

// App is one installed application.
type App struct {
	// Name is what the user knows the application by: the Latin spelling
	// where a bundle has one, so it is typeable.
	Name string
	// Localized is the name the user's own desktop shows, when the bundle
	// carries one that differs (a Chinese-named application on a Chinese
	// desktop, say). A row is titled with it and subtitled with Name.
	Localized string
	// Aliases are extra spellings search may match: a bundle's executable,
	// the pieces of its identifier, its folder name.
	Aliases []string
	// Path is the bundle, desktop entry or shortcut to open.
	Path string
	// Icon is what the entry declares its icon to be (a Linux desktop
	// entry's `Icon=`), and IconPath the file that resolved to, empty when
	// the platform names none this build can read.
	Icon     string
	IconPath string
	// Exec, when set, is the program and arguments to run instead of
	// opening Path: Linux's .desktop Exec, which opening the file itself
	// would not launch.
	Exec []string
}

// Open launches the application: Exec when the entry carries one, else the
// path with the desktop's default handler.
func (a App) Open() error {
	if len(a.Exec) > 0 {
		cmd := exec.Command(a.Exec[0], a.Exec[1:]...)
		cmd.Dir = homeDir()
		// Detach: the launcher must not wait for the app, and the app must
		// not die with it.
		cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
		if err := cmd.Start(); err != nil {
			return err
		}
		return nil
	}
	return mygo.Shell.OpenPath(a.Path)
}

// Roots is the platform's application directories, existing ones only.
func Roots() []string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications",
			"/System/Applications",
			"/Applications/Utilities",
			"/System/Applications/Utilities",
			filepath.Join(homeDir(), "Applications"),
		}
	case "windows":
		for _, base := range []string{os.Getenv("APPDATA"), os.Getenv("PROGRAMDATA")} {
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs"))
			}
		}
	default:
		candidates = []string{
			"/usr/share/applications",
			"/usr/local/share/applications",
			"/var/lib/flatpak/exports/share/applications",
			"/var/lib/snapd/desktop/applications",
			filepath.Join(homeDir(), ".local", "share", "applications"),
			filepath.Join(homeDir(), ".local", "share", "flatpak", "exports", "share", "applications"),
		}
	}
	var roots []string
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			roots = append(roots, dir)
		}
	}
	return roots
}

// Scan reads the applications under the given roots, sorted by name.
func Scan(roots []string) []App {
	var found []App
	for _, root := range roots {
		switch runtime.GOOS {
		case "darwin":
			found = append(found, ScanDarwin(root)...)
		case "windows":
			found = append(found, ScanWindows(root)...)
		default:
			found = append(found, ScanLinux(root)...)
		}
		if len(found) >= maxApps {
			break
		}
	}
	return dedupe(found)
}

// ScanDarwin collects the .app bundles under root, descending into folders
// such as Utilities but never into a bundle.
func ScanDarwin(root string) []App {
	var out []App
	walk(root, func(path string, d os.DirEntry) bool {
		name := d.Name()
		if !d.IsDir() || !strings.HasSuffix(name, ".app") {
			return true
		}
		info := readBundle(path)
		out = append(out, App{
			Name:      info.Name,
			Localized: info.Localized,
			Aliases:   info.Aliases,
			Path:      path,
			IconPath:  info.IconPath,
		})
		return false // a bundle holds its own helpers; do not descend
	})
	sortApps(out)
	return out
}

// ScanWindows collects the .lnk shortcuts under root.
func ScanWindows(root string) []App {
	var out []App
	walk(root, func(path string, d os.DirEntry) bool {
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".lnk") {
			return true
		}
		base := name[:len(name)-len(".lnk")]
		out = append(out, App{Name: base, Path: path})
		return true
	})
	sortApps(out)
	return out
}

// ScanLinux collects the .desktop entries under root, skipping the ones the
// desktop hides and the ones with no program to run.
func ScanLinux(root string) []App {
	var out []App
	walk(root, func(path string, d os.DirEntry) bool {
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".desktop") {
			return true
		}
		entry, ok := ParseDesktopFile(path)
		if !ok {
			return true
		}
		entry.IconPath = desktopIconFile(entry.Icon)
		out = append(out, entry)
		return true
	})
	sortApps(out)
	return out
}

// walk visits every entry under root, depth-first; visit returns whether to
// descend into a directory.
func walk(root string, visit func(path string, d os.DirEntry) bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, d := range entries {
		path := filepath.Join(root, d.Name())
		if visit(path, d) && d.IsDir() {
			walk(path, visit)
		}
	}
}

// ParseDesktopFile reads one .desktop file: its [Desktop Entry] section's
// Name and Exec. It reports false for hidden entries, entries without an
// Exec, or files that cannot be read.
func ParseDesktopFile(path string) (App, bool) {
	f, err := os.Open(path)
	if err != nil {
		return App{}, false
	}
	defer f.Close()

	inEntry := false
	hidden, noDisplay := false, false
	var name, exec, icon string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "["):
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "Name":
			if name == "" {
				name = value
			}
		case "Exec":
			exec = value
		case "Icon":
			icon = value
		case "Hidden":
			hidden = strings.EqualFold(value, "true")
		case "NoDisplay":
			noDisplay = strings.EqualFold(value, "true")
		}
	}
	if err := scanner.Err(); err != nil {
		return App{}, false
	}
	argv := SplitDesktopExec(exec)
	if name == "" || len(argv) == 0 || hidden || noDisplay {
		return App{}, false
	}
	return App{Name: name, Path: path, Exec: argv, Icon: icon}, true
}

// SplitDesktopExec splits a .desktop Exec value into argv: whitespace
// separated, with single and double quotes and backslash escapes, and with
// the field codes (%U, %f, ...) removed as the Desktop Entry Specification
// says.
func SplitDesktopExec(exec string) []string {
	var args []string
	var current strings.Builder
	started := false
	quote := byte(0)

	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}

	for i := 0; i < len(exec); i++ {
		c := exec[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			if c == '\\' && quote == '"' && i+1 < len(exec) {
				i++
				current.WriteByte(exec[i])
				continue
			}
			current.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
			started = true
		case c == ' ' || c == '\t':
			flush()
		case c == '%':
			if i+1 < len(exec) {
				i++
				if exec[i] == '%' {
					current.WriteByte('%')
					started = true
				}
				// Any other field code stands for a file list the launcher
				// has none of: leave it out.
				continue
			}
		case c == '\\' && i+1 < len(exec):
			i++
			current.WriteByte(exec[i])
			started = true
		default:
			current.WriteByte(c)
			started = true
		}
	}
	flush()
	return args
}

// dedupe sorts the applications by name and drops duplicates of the same
// path, keeping the shortest name.
func dedupe(in []App) []App {
	byPath := map[string]App{}
	for _, a := range in {
		if existing, ok := byPath[a.Path]; ok && len(existing.Name) <= len(a.Name) {
			continue
		}
		byPath[a.Path] = a
	}
	out := make([]App, 0, len(byPath))
	for _, a := range byPath {
		out = append(out, a)
	}
	sortApps(out)
	if len(out) > maxApps {
		out = out[:maxApps]
	}
	return out
}

// sortApps orders applications by name, then path, for a stable list.
func sortApps(out []App) {
	sort.Slice(out, func(i, j int) bool {
		left, right := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if left == right {
			return out[i].Path < out[j].Path
		}
		return left < right
	})
}

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "/"
}
