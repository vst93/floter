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
	// Localized is the name the user's own desktop shows, when the bundle or
	// `.desktop` entry carries one that differs (a Chinese-named application on
	// a Chinese desktop, say). A row is titled with it, and the Latin name is
	// deliberately *not* printed as a subtitle: the pair is one app's name in
	// two languages, and the old build dropped that second line as zero
	// information ("企业微信 / WeCom", R14). The name stays searchable instead.
	Localized string
	// Comment is the entry's one-line description, when the platform ships one:
	// a Linux `.desktop` entry's `Comment=`. It is the launcher row's subtitle —
	// and only when there is no localized name, because the old build's subtitle
	// slot preferred the alternate name (which it then dropped) over the
	// comment. macOS and Windows ship none.
	Comment string
	// Aliases are extra spellings search may match: a bundle's executable,
	// the pieces of its identifier, its folder name.
	Aliases []string
	// Initials is the Latin search key of the name: every letter and digit as
	// it is, every CJK character replaced by the first letter of its pinyin
	// ("网易云音乐" → "wyyyy"). It is how a Chinese name is reachable from a
	// Latin keyboard.
	Initials string
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
	// DBusActivatable is Linux's `DBusActivatable=true`: the entry asks the
	// session bus to start it, which is also the only way single-instance
	// activation works, so Open launches it through `gio` instead of its argv.
	DBusActivatable bool
	// tryExec is Linux's `TryExec=`: the program whose presence decides whether
	// the entry is installed at all. It is not part of an App's identity — only
	// ScanLinux reads it, to drop entries for programs that are not here.
	tryExec string
	// target is what a Windows shortcut points at, when it could be read. The
	// registry's entries name executables directly, so this is what lets the
	// second source skip a program the Start Menu already listed (the
	// reference's shared `seen_targets`, `windows.rs:60-76`).
	target string
}

// launchCommand is the program and arguments Open runs for an app that carries
// its own command (a Linux `.desktop` entry): `gio launch <path>` for a
// D-Bus-activatable entry — the session bus starts it in its own scope, which
// is what single-instance activation needs — and the entry's own argv
// otherwise. Both are empty for an app that has neither, which Open opens by
// path instead.
func (a App) launchCommand() (string, []string) {
	if a.DBusActivatable {
		return "gio", []string{"launch", a.Path}
	}
	if len(a.Exec) > 0 {
		return a.Exec[0], a.Exec[1:]
	}
	return "", nil
}

// Open launches the application: Exec when the entry carries one, else the
// path with the desktop's default handler.
func (a App) Open() error {
	program, args := a.launchCommand()
	if program == "" {
		return mygo.Shell.OpenPath(a.Path)
	}
	err := launchDetached(program, args...)
	// A D-Bus-activatable entry whose `gio` is missing still has its own argv
	// to fall back to (the original did the same after `gio` failed).
	if err != nil && a.DBusActivatable && len(a.Exec) > 0 {
		return launchDetached(a.Exec[0], a.Exec[1:]...)
	}
	return err
}

// launchDetached starts a program that outlives the launcher: the launcher must
// not wait for it, it must not die with the launcher, and - the Linux case this
// build ships for - it must not share the launcher's process group or session,
// where a signal meant for either would reach both. (internal/spawn's own
// detach, restated here so the two cannot drift.)
func launchDetached(program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir = homeDir()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detachExec(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
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
		candidates = linuxRoots(linuxDataHome(), filepath.SplitList(os.Getenv("XDG_DATA_DIRS")))
	}
	var roots []string
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			roots = append(roots, dir)
		}
	}
	return roots
}

// linuxDataHome is the per-user data directory the XDG Base Directory
// Specification defines: `$XDG_DATA_HOME` when it is set to an absolute path,
// and `~/.local/share` otherwise (the spec's own fallback for a relative or
// missing value).
func linuxDataHome() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" && filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(homeDir(), ".local", "share")
}

// linuxRoots is where a Linux publishes its desktop entries, in **precedence
// order**: the user's own directory first, then the system's, then the package
// exports, then whatever else `$XDG_DATA_DIRS` advertises.
//
// The order is the reference's (`roots()`, `main:src-tauri/src/commands/apps/
// linux.rs:186-205`), and it is the half of the rule that makes a user's
// override win: a `.desktop` file under `$XDG_DATA_HOME` shadows the system
// copy of the same id, which only works if the user's directory is scanned
// first (see desktopFileID). `$XDG_DATA_HOME` and `$XDG_DATA_DIRS` are read
// here rather than assumed to be `~/.local/share` and `/usr/share`: a custom
// prefix, a container, or a distribution that publishes its applications
// somewhere else is where a hard-coded list finds nothing.
//
// A `dataDir` that is already in the list (the default `/usr/local/share` and
// `/usr/share` usually are) contributes nothing, exactly as the reference's
// `if !roots.contains(&candidate)` does.
func linuxRoots(dataHome string, dataDirs []string) []string {
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

// Scan reads the applications under the given roots, sorted by name.
//
// The roots are in precedence order, and each platform's scanner is given the
// state that decides which of two entries is the same application (a Linux
// desktop-file id, a Windows shortcut target or name) — so one application is
// one row, and the row that survives is the one from the earlier root.
func Scan(roots []string) []App {
	var found []App
	switch runtime.GOOS {
	case "darwin":
		for _, root := range roots {
			found = append(found, ScanDarwin(root)...)
			if len(found) >= maxApps {
				break
			}
		}
	case "windows":
		found = append(found, scanWindowsRoots(roots)...)
		// The Start Menu is not the whole of what is installed (see
		// scanRegistryPrograms): the uninstall keys fill the gaps.
		found = append(found, scanRegistryPrograms(found)...)
	default:
		found = append(found, scanLinuxRoots(roots)...)
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
			Initials:  computeInitials(info.Name, info.Localized),
			Path:      path,
			IconPath:  info.IconPath,
		})
		return false // a bundle holds its own helpers; do not descend
	})
	sortApps(out)
	return out
}

// scanWindowsRoots collects the .lnk shortcuts under every root, in precedence
// order, keeping one row per program.
//
// The two identity keys — the lowercased target and the lowercased name — are
// shared across every root, as the reference shares them (`scan()`,
// `main:src-tauri/src/commands/apps/windows.rs:52-63`): one program's shortcut
// in the user's Start Menu and in the machine's is one row, and the user's
// comes first. They are then handed to the registry pass, which is why it can
// see what the shortcuts already listed.
func scanWindowsRoots(roots []string) []App {
	var found []App
	seenTargets := map[string]bool{}
	seenNames := map[string]bool{}
	for _, root := range roots {
		found = append(found, scanWindowsRoot(root, seenTargets, seenNames)...)
		if len(found) >= maxApps {
			break
		}
	}
	return found
}

// ScanWindows collects the .lnk shortcuts under root.
//
// `seenTargets` and `seenNames` are the scan's own identity keys, lowercased:
// they are passed in rather than created here so that every root shares them
// (see Scan). `ScanWindows` itself starts empty ones, which is one root's worth
// of deduplication.
//
// Each shortcut's own target is read (see shortcut.go), and everything the
// reference derives from it comes with it:
//
//   - the target must be a program a user launches, or the shortcut is not a
//     row at all — the Start Menu also holds the uninstallers and the read-me
//     links every installer drops beside its program
//     (`is_launchable_target`, `windows.rs:539-549`);
//   - two shortcuts pointing at one program are one row (the reference dedupes
//     on the lowercased target, `windows.rs:234-247`);
//   - the executable's own name is a search alias, which is what keeps
//     「企业微信.lnk」 reachable from a Latin keyboard as `WXWork`
//     (`build_aliases`, `executable_name`), and the name's pinyin initials are
//     the Chinese front door the macOS and Linux scans already have.
//
// A shortcut whose target cannot be read is still a row: Windows resolves it
// itself when the file is opened, which is the reference's own decision.
func ScanWindows(root string) []App {
	return scanWindowsRoot(root, map[string]bool{}, map[string]bool{})
}

// scanWindowsRoot is ScanWindows with the scan's identity keys handed in.
func scanWindowsRoot(root string, seenTargets, seenNames map[string]bool) []App {
	var out []App
	walk(root, func(path string, d os.DirEntry) bool {
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".lnk") {
			return true
		}
		base := name[:len(name)-len(".lnk")]
		target, hasTarget := ShortcutTarget(path)
		if hasTarget {
			if !IsLaunchableTarget(target) {
				return true
			}
			key := strings.ToLower(target)
			if seenTargets[key] {
				return true
			}
			seenTargets[key] = true
		}
		nameKey := strings.ToLower(base)
		if seenNames[nameKey] {
			return true
		}
		seenNames[nameKey] = true

		// A shortcut named in Chinese still points at a program with a Latin
		// name, so the executable's own name is the alias that keeps the row
		// typeable (and it is dropped when it repeats the shortcut's name, as
		// `build_aliases` does for every scanner).
		var candidates []string
		if executable := ShortcutTargetName(target); executable != "" {
			candidates = append(candidates, executable)
		}
		out = append(out, App{
			Name:     base,
			Path:     path,
			Aliases:  buildAliases(base, "", candidates),
			Initials: computeInitials(base, ""),
			target:   target,
		})
		return true
	})
	sortApps(out)
	return out
}

// scanRegistryPrograms is the Windows scan's second source: the programs the
// uninstall registry lists but no Start Menu shortcut names — a driver's control
// panel, a per-user install into a folder nobody put in `Programs`, a program
// whose installer the user told to create no shortcut at all
// (`collect_registry_programs`, `windows.rs:260-330`).
//
// The rows it adds are the same shape the shortcut scan produces: the
// executable's name is the Latin alias of a Chinese display name, the pinyin
// initials make the name reachable the way they are on macOS and Linux, and a
// program already listed — by target or by name — is not listed twice. Off
// Windows the sources are nil and this is a no-op.
func scanRegistryPrograms(existing []App) []App {
	sources := registrySources()
	if len(sources) == 0 {
		return nil
	}
	seenTargets := make(map[string]bool, len(existing))
	seenNames := make(map[string]bool, len(existing))
	for _, app := range existing {
		if app.target != "" {
			seenTargets[strings.ToLower(app.target)] = true
		}
		seenNames[strings.ToLower(app.Name)] = true
	}
	var out []App
	for _, text := range sources {
		for _, program := range ParseRegistryPrograms(text) {
			executable := program.Executable()
			if executable == "" {
				continue
			}
			targetKey := strings.ToLower(executable)
			nameKey := strings.ToLower(program.DisplayName)
			if seenTargets[targetKey] || seenNames[nameKey] {
				continue
			}
			seenTargets[targetKey] = true
			seenNames[nameKey] = true
			var candidates []string
			if name := ShortcutTargetName(executable); name != "" {
				candidates = append(candidates, name)
			}
			out = append(out, App{
				Name:     program.DisplayName,
				Path:     executable,
				Aliases:  buildAliases(program.DisplayName, "", candidates),
				Initials: computeInitials(program.DisplayName, ""),
				target:   executable,
			})
		}
	}
	return out
}

// scanLinuxRoots collects the .desktop entries under every root, in precedence
// order, and keeps one row per desktop-file id.
//
// A desktop-file id is unique across the whole search path: the entry under the
// user's own directory *is* the application, and the system copy of the same id
// is shadowed by it. That is the reference's `seen_ids` (`scan()`,
// `main:src-tauri/src/commands/apps/linux.rs:56-67`), and without it a user
// override is a second row beside the original rather than an override. The id
// is claimed before the entry is parsed, exactly as the reference claims it, so
// a broken user entry hides the system one instead of showing both.
func scanLinuxRoots(roots []string) []App {
	var found []App
	seenIDs := make(map[string]bool)
	for _, root := range roots {
		found = append(found, scanLinuxRoot(root, seenIDs)...)
		if len(found) >= maxApps {
			break
		}
	}
	return found
}

// ScanLinux collects the .desktop entries under root, skipping the ones the
// desktop hides and the ones with no program to run.
//
// It is one root on its own, so it has no id table to consult; `Scan` uses
// scanLinuxRoots, which does.
func ScanLinux(root string) []App {
	return scanLinuxRoot(root, nil)
}

// scanLinuxRoot is ScanLinux with the scan's id table handed in: a nil table
// deduplicates nothing, which is what a single root needs.
func scanLinuxRoot(root string, seenIDs map[string]bool) []App {
	var out []App
	walk(root, func(path string, d os.DirEntry) bool {
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".desktop") {
			return true
		}
		if seenIDs != nil {
			id := desktopFileID(root, path)
			if seenIDs[id] {
				return true
			}
			seenIDs[id] = true
		}
		entry, ok := ParseDesktopFile(path)
		if !ok {
			return true
		}
		// `TryExec` is the entry's own "is this installed" probe: a program it
		// names that is not here means the application is not installed either
		// (the original's `executable_available`, linux.rs:406). This is the one
		// check that needs the filesystem/PATH, so it lives in the scan rather
		// than in the pure reader.
		if entry.tryExec != "" && !executableAvailable(entry.tryExec) {
			return true
		}
		entry.IconPath = desktopIconFile(entry.Icon)
		entry.Initials = computeInitials(entry.Name, entry.Localized)
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

// executableAvailable reports whether a `TryExec=` program can be run: an
// absolute or relative path must name an executable file, a bare name must be
// found in `PATH` (the original's `executable_available`).
func executableAvailable(program string) bool {
	if strings.ContainsRune(program, os.PathSeparator) || strings.ContainsRune(program, '/') {
		return isExecutableFile(program)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		if isExecutableFile(filepath.Join(dir, program)) {
			return true
		}
	}
	return false
}

// isExecutableFile reports whether path is a regular file with any execute bit
// set. The mode bits are emulated on Windows (files read as 0666), so a probe
// there is permissive rather than wrong: it never hides an installed entry.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// desktopLocales is the locale ladder for a `.desktop` entry's localized
// `Name[locale]`/`Comment[locale]`, Chinese first: the old build
// (`src-tauri/src/commands/apps/linux.rs`) read `Name[zh_CN]` ahead of every
// other locale, so a Chinese desktop shows the same localized/original name
// pair macOS does. Only these five spellings are recognized; anything else
// (`Name[fr]`) is ignored, exactly as the original did.
var desktopLocales = []string{"zh_CN", "zh_SG", "zh", "zh_TW", "zh_HK"}

// desktopLocaleRank is the position of a locale in the ladder, or -1 when the
// ladder does not know it.
func desktopLocaleRank(locale string) int {
	for i, candidate := range desktopLocales {
		if candidate == locale {
			return i
		}
	}
	return -1
}

// unescapeDesktop applies the Desktop Entry Specification's string escapes to
// a `Name=`/`Comment=` value (`\s`, `\n`, `\t`, `\r`, `\\`); an unknown escape
// keeps its backslash, as the original's `unescape` did.
func unescapeDesktop(value string) string {
	if !strings.Contains(value, "\\") {
		return value
	}
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			out.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 's':
			out.WriteByte(' ')
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		default:
			out.WriteByte('\\')
			out.WriteByte(value[i])
		}
	}
	return out.String()
}

// ParseDesktopFile reads one .desktop file: its [Desktop Entry] section's
// Name (plus the best Chinese `Name[locale]`), Comment (same ladder), Exec,
// Icon and hidden flags, along with `Type` and `DBusActivatable`. It reports
// false for hidden entries, entries of a type other than `Application` (a
// `Link=` or `Directory=` entry is not something to launch), entries without an
// Exec, or files that cannot be read. The one availability check it does not
// make is `TryExec` — that needs the filesystem and PATH, so ScanLinux does it.
func ParseDesktopFile(path string) (App, bool) {
	f, err := os.Open(path)
	if err != nil {
		return App{}, false
	}
	defer f.Close()

	inEntry := false
	hidden, noDisplay := false, false
	var name, exec, icon, entryType string
	dbusActivatable := false
	// The Latin search keys a `.desktop` entry advertises besides its name:
	// the untranslated spellings only (the original read `GenericName`,
	// `Keywords`, `StartupWMClass` and `TryExec` with no locale suffix).
	var genericName, keywords, startupWMClass, tryExec string
	// The localized name/comment ladders: the lowest rank seen wins, and an
	// unlocalized value is only a fallback that a localized one may override
	// (the original's `best_*_locale` against `usize::MAX`).
	var localized string
	localizedRank := len(desktopLocales)
	var comment string
	commentRank := len(desktopLocales)
	commentSet := false
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
		base, locale, hasLocale := strings.Cut(key, "[")
		if hasLocale {
			locale = strings.TrimSuffix(locale, "]")
		}
		switch base {
		case "Name":
			if !hasLocale {
				name = unescapeDesktop(value)
				continue
			}
			if rank := desktopLocaleRank(locale); rank >= 0 && rank < localizedRank {
				localizedRank = rank
				localized = unescapeDesktop(value)
			}
		case "Comment":
			if !hasLocale {
				if !commentSet {
					comment = unescapeDesktop(value)
					commentSet = true
				}
				continue
			}
			if rank := desktopLocaleRank(locale); rank >= 0 && rank < commentRank {
				commentRank = rank
				comment = unescapeDesktop(value)
				commentSet = true
			}
		case "Exec":
			exec = value
		case "Icon":
			icon = value
		case "GenericName":
			if !hasLocale {
				genericName = unescapeDesktop(value)
			}
		case "Keywords":
			if !hasLocale {
				keywords = unescapeDesktop(value)
			}
		case "StartupWMClass":
			if !hasLocale {
				startupWMClass = unescapeDesktop(value)
			}
		case "TryExec":
			if !hasLocale {
				tryExec = unescapeDesktop(value)
			}
		case "Hidden":
			hidden = strings.EqualFold(value, "true")
		case "NoDisplay":
			noDisplay = strings.EqualFold(value, "true")
		case "Type":
			if !hasLocale {
				entryType = value
			}
		case "DBusActivatable":
			if !hasLocale {
				dbusActivatable = value == "true"
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return App{}, false
	}
	argv := SplitDesktopExec(exec)
	if name == "" || len(argv) == 0 || hidden || noDisplay {
		return App{}, false
	}
	// Only `Application` entries launch; an absent `Type` is kept, because the
	// original only rejected an explicit non-Application type.
	if entryType != "" && entryType != "Application" {
		return App{}, false
	}
	// A localized name equal to the plain one is not a second name (the
	// original's `filter(|value| value != &name)`), and an empty comment is
	// dropped rather than drawn as a blank line.
	if localized == name {
		localized = ""
	}
	return App{
		Name:            name,
		Localized:       localized,
		Comment:         comment,
		Aliases:         desktopAliases(path, name, localized, genericName, keywords, startupWMClass, tryExec, argv),
		Path:            path,
		Exec:            argv,
		Icon:            icon,
		DBusActivatable: dbusActivatable,
		tryExec:         tryExec,
	}, true
}

// desktopAliases are the Latin spellings a query can reach a `.desktop` entry
// by besides its name: the program it runs, the class its window reports, the
// words it advertises itself with, and the application half of its
// desktop-file id. This is the original's `into_application` candidate list
// (`src-tauri/src/commands/apps/linux.rs:423-440`) run through
// `build_aliases` (`src-tauri/src/commands/apps/mod.rs:326`).
func desktopAliases(path, name, localized, genericName, keywords, startupWMClass, tryExec string, argv []string) []string {
	var candidates []string
	candidates = append(candidates, genericName, startupWMClass)
	candidates = append(candidates, executableName(argv))
	candidates = append(candidates, executableName(SplitDesktopExec(tryExec)))
	candidates = append(candidates, strings.Split(keywords, ";")...)
	candidates = append(candidates, identifierAliases(desktopFileStem(path))...)
	return buildAliases(name, localized, candidates)
}

// desktopFileStem is a `.desktop` path's file name without its extension, the
// stem the original fed to `identifier_aliases`.
func desktopFileStem(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" && len(ext) < len(base) {
		return base[:len(base)-len(ext)]
	}
	return base
}

// desktopFileID is a `.desktop` entry's id: its path below the applications
// directory with the separators turned into dashes (`kde4/konsole.desktop` →
// `kde4-konsole.desktop`). It is the key the search path is unique on — the
// same id in two roots is one application — and the reference computes it the
// same way (`desktop_file_id`, `main:src-tauri/src/commands/apps/linux.rs:265`).
func desktopFileID(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "" || rel == "." {
		return filepath.Base(path)
	}
	return strings.ReplaceAll(filepath.ToSlash(rel), "/", "-")
}

// executableName is the program an `Exec=`/`TryExec=` value runs, without its
// path or arguments: `/usr/bin/google-chrome-stable %U` is searchable as
// `google-chrome-stable` (the original's `executable_name`).
func executableName(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	return filepath.Base(argv[0])
}

// buildAliases reduces the raw strings a scanner collected to the alias set
// stored on an App (the original's `build_aliases`). Dropped: anything that
// repeats a name the user can already see, anything already in the list, and
// anything with no ASCII letter or digit in it — being reachable from a Latin
// keyboard is the whole purpose of the field.
func buildAliases(name, localized string, candidates []string) []string {
	var aliases []string
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !hasASCIIAlnum(candidate) {
			continue
		}
		if strings.EqualFold(candidate, name) {
			continue
		}
		if localized != "" && strings.EqualFold(candidate, localized) {
			continue
		}
		duplicate := false
		for _, existing := range aliases {
			if strings.EqualFold(existing, candidate) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		aliases = append(aliases, candidate)
	}
	return aliases
}

// hasASCIIAlnum reports whether a value carries a character a Latin keyboard
// can type.
func hasASCIIAlnum(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
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
