package inventory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"floter/internal/extensions"
	"floter/internal/progtype"
	"floter/internal/xdg"
)

// The per-platform sources: the fixed OS roots outside the search path
// (desktop entries, flatpak/snap/nix exports, Homebrew, Scoop, WinGet),
// each with the source a candidate names. Best-effort throughout: a
// directory that is not there contributes nothing.

// discoverLinux reads the desktop entries and the Linux package exports.
func discoverLinux(merge mergeFunc) {
	mergeEntries(merge, linuxDesktopRoots(), SourceDesktop, QualityNativeSupport)
	flatpakRoots, snapRoot := linuxExportRoots()
	for _, directory := range flatpakRoots {
		discoverExecutableDirectory(merge, directory, SourceFlatpak, QualityNativeSupport)
	}
	if home, err := os.UserHomeDir(); err == nil {
		discoverExecutableDirectory(merge, filepath.Join(home, ".nix-profile", "bin"), SourceNix, QualityNativeSupport)
	}
	discoverExecutableDirectory(merge, snapRoot, SourceSnap, QualityNativeSupport)
	discoverExecutableDirectory(merge, "/nix/var/nix/profiles/default/bin", SourceNix, QualityNativeSupport)
}

// linuxExportRoots is where a Linux package manager writes the wrappers that
// make a packaged application runnable from the search path. It is the one
// table both the export walk and a desktop entry's `flatpak run`/`snap run`
// mapping read, so a flatpak application is one candidate rather than a
// wrapper row next to a `/usr/bin/flatpak` row. A var so a test can point both
// at a fixture.
var linuxExportRoots = func() (flatpak []string, snap string) {
	var roots []string
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".local", "share", "flatpak", "exports", "bin"))
	}
	return append(roots, "/var/lib/flatpak/exports/bin"), "/snap/bin"
}

// linuxDesktopRoots is where a Linux publishes its desktop entries. It is the
// same table the launcher's application scan reads (`internal/xdg`), so one
// desktop entry is found — and its `Name=`/`Comment=` used — the same way by the
// application list and by the tool inventory. Before this package asked `xdg`,
// it kept its own copy: the fixed system roots were appended in the opposite
// order (`/usr/share` before `/usr/local/share`, so a local administrator's
// override lost), the environment's directories came first rather than after,
// and the two system package-export roots were missing altogether.
func linuxDesktopRoots() []string {
	return xdg.DesktopRootsFromEnv()
}

// mergeEntries reads desktop entries in from a list of directories: each
// entry's `Name` and `Comment` are the description discovery carries, and the
// path it reports is the file the entry actually runs — not the first token of
// its `Exec=`.
func mergeEntries(merge mergeFunc, directories []string, source DiscoverySource, quality DiscoveryQuality) {
	// A desktop-file id is unique across the whole search path, so the first
	// root that publishes one wins: a user override shadows the system copy.
	// The same rule the launcher's application scan follows.
	seen := map[string]bool{}
	for _, directory := range directories {
		xdg.WalkDesktopEntries(directory, xdg.DesktopWalkDepth, func(path string) {
			id := desktopFileID(directory, path)
			if seen[id] {
				return
			}
			seen[id] = true
			desktop, ok := parseDesktopEntry(path)
			if !ok {
				return
			}
			target, ok := desktopEntryTarget(desktop)
			if !ok {
				return
			}
			merge(target, desktop.Name, desktop.Comment, source, quality)
		})
	}
}

// desktopFileID is an entry's id: its path below the root it was read from,
// with the separators flattened.
func desktopFileID(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		relative = path
	}
	return strings.ReplaceAll(relative, string(filepath.Separator), "-")
}

// desktopEntry is the fields a discovery reads off a desktop entry: the name
// and description it publishes, and the command it runs.
type desktopEntry struct {
	Name      string
	Comment   string
	Exec      string
	TryExec   string
	Type      string
	Hidden    bool
	NoDisplay bool
}

// parseDesktopEntry reads an entry and answers the question a discovery asks of
// it: is this a tool at all? `Hidden`, `NoDisplay`, a `Type` that is not an
// application and an entry with no `Name`/`Exec` are not.
func parseDesktopEntry(path string) (desktopEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return desktopEntry{}, false
	}
	var entry desktopEntry
	inMain := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
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
				if entry.Name == "" {
					entry.Name = value
				}
			case "Comment":
				if entry.Comment == "" {
					entry.Comment = value
				}
			case "Exec":
				entry.Exec = value
			case "TryExec":
				entry.TryExec = value
			case "Type":
				entry.Type = value
			case "Hidden":
				entry.Hidden = strings.EqualFold(value, "true")
			case "NoDisplay":
				entry.NoDisplay = strings.EqualFold(value, "true")
			}
		}
	}
	if entry.Name == "" || entry.Exec == "" || entry.Hidden || entry.NoDisplay {
		return desktopEntry{}, false
	}
	// A missing `Type` defaults to an application.
	if entry.Type != "" && entry.Type != "Application" {
		return desktopEntry{}, false
	}
	return entry, true
}

// readDesktopEntry is an entry's Name, Comment and Exec program: the four
// values discovery carried before the program was resolved to a target.
func readDesktopEntry(path string) (name, comment, exec string, ok bool) {
	entry, ok := parseDesktopEntry(path)
	if !ok {
		return "", "", "", false
	}
	return entry.Name, entry.Comment, desktopExecCommand(entry.Exec), true
}

// desktopExecCommand is an Exec line's first token, with the field codes
// a desktop entry passes its arguments through dropped (`%f %F %u %U %i %c %k`).
func desktopExecCommand(value string) string {
	argv := desktopExecArgv(value)
	if len(argv) == 0 {
		return ""
	}
	return argv[0]
}

// desktopExecArgv splits an Exec line into the argv it declares: quotes group,
// a backslash escapes, and the field codes are stripped — the candidate is the
// bare command, not the arguments a launcher would pass it. A malformed line
// yields nothing.
func desktopExecArgv(value string) []string {
	var (
		argv    []string
		current strings.Builder
		quoted  bool
		open    bool
	)
	flush := func() bool {
		if !open && current.Len() == 0 {
			return true
		}
		token, ok := stripDesktopFieldCodes(current.String())
		current.Reset()
		open = false
		if !ok {
			return false
		}
		if token != "" {
			argv = append(argv, token)
		}
		return true
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch {
		case character == '"':
			quoted = !quoted
			open = true
		case character == '\\' && index+1 < len(value):
			index++
			current.WriteByte(value[index])
			open = true
		case (character == ' ' || character == '\t') && !quoted:
			if !flush() {
				return nil
			}
		default:
			current.WriteByte(character)
			open = true
		}
	}
	if quoted || !flush() {
		return nil
	}
	return argv
}

// stripDesktopFieldCodes drops the field codes a desktop entry interpolates
// its arguments through, and keeps a literal `%%` as `%`. An unknown code
// makes the line invalid.
func stripDesktopFieldCodes(value string) (string, bool) {
	var output strings.Builder
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character != '%' {
			output.WriteByte(character)
			continue
		}
		index++
		if index >= len(value) {
			return "", false
		}
		switch value[index] {
		case '%':
			output.WriteByte('%')
		case 'f', 'F', 'u', 'U', 'i', 'c', 'k':
		default:
			return "", false
		}
	}
	return output.String(), true
}

// desktopLookup is the search path a bare Exec program is resolved through: the
// same directories the walk reads, so an entry and a PATH hit meet. A var so a
// test can resolve against a fixture instead of the machine.
var desktopLookup = extensions.LookTool

// desktopProgram resolves the program an Exec line names to the file it would
// run: a path with a separator has to be executable as written, a bare name is
// looked up in the same directories the walk reads. A var so a test can resolve
// against a fixture instead of the machine.
var desktopProgram = func(program string) (string, bool) {
	program = strings.TrimSpace(program)
	if program == "" {
		return "", false
	}
	if !strings.ContainsRune(program, filepath.Separator) && !strings.Contains(program, "/") {
		return desktopLookup(program)
	}
	absolute, err := filepath.Abs(program)
	if err != nil {
		return "", false
	}
	if !executableFile(absolute) {
		return "", false
	}
	return absolute, true
}

// desktopEntryTarget is the path a desktop entry actually runs, read the way
// the reference implementation reads it: an `env VAR=…` prefix is unwrapped, a
// `flatpak run <app-id>` names the package manager's own export wrapper, a
// `snap run <name>` names the snap's bin, and a `sh -c` command string is not a
// structured target at all.
//
// Before this, the first token was the answer: every snap entry became a
// candidate called `env` at the working directory, every flatpak entry landed
// on `/usr/bin/flatpak` and overwrote the `flatpak` row's description with the
// application's, and a bare `Exec=firefox` never met the PATH row that already
// carried it.
func desktopEntryTarget(entry desktopEntry) (string, bool) {
	if entry.TryExec != "" {
		if _, ok := desktopProgram(entry.TryExec); !ok {
			return "", false
		}
	}
	argv := unwrapDesktopEnv(desktopExecArgv(entry.Exec))
	if len(argv) == 0 {
		return "", false
	}
	switch filepath.Base(argv[0]) {
	case "flatpak":
		// `flatpak` without a `run <app-id>` is not a tool the entry names.
		appID, ok := flatpakRunAppID(argv)
		if !ok {
			return "", false
		}
		flatpakRoots, _ := linuxExportRoots()
		for _, root := range flatpakRoots {
			if wrapper, ok := desktopProgram(filepath.Join(root, appID)); ok {
				return wrapper, true
			}
		}
		// Installed, but its export is not on this machine's roots: there is
		// no path to offer, and a candidate without one is not connectable.
		return "", false
	case "snap":
		if len(argv) > 2 && argv[1] == "run" {
			_, snapRoot := linuxExportRoots()
			return desktopProgram(filepath.Join(snapRoot, argv[2]))
		}
	case "sh", "bash", "zsh", "fish":
		if len(argv) > 1 && argv[1] == "-c" {
			return "", false
		}
	}
	return desktopProgram(argv[0])
}

// unwrapDesktopEnv drops the `env VAR=value …` prefix a desktop entry uses to
// hand an application an environment: the command is the first token after it.
func unwrapDesktopEnv(argv []string) []string {
	if len(argv) == 0 || filepath.Base(argv[0]) != "env" {
		return argv
	}
	for index := 1; index < len(argv); index++ {
		argument := argv[index]
		switch {
		case argument == "-u" || argument == "--unset":
			index++
		case strings.HasPrefix(argument, "-") || strings.Contains(argument, "="):
		default:
			return argv[index:]
		}
	}
	return nil
}

// flatpakRunAppID is the application id a `flatpak run` argv names: the first
// argument after `run` that is not an option.
func flatpakRunAppID(argv []string) (string, bool) {
	run := -1
	for index, argument := range argv {
		if argument == "run" {
			run = index
			break
		}
	}
	if run < 0 {
		return "", false
	}
	for _, argument := range argv[run+1:] {
		if strings.HasPrefix(argument, "-") {
			continue
		}
		return argument, true
	}
	return "", false
}

// discoverMacOS reads Homebrew's bins and the applications LaunchServices
// knows.
func discoverMacOS(merge mergeFunc) {
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
func discoverMacOSApps(merge mergeFunc, directory string) {
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
			if !executableFile(path) {
				continue
			}
			merge(path, executable.Name(), bundleName, SourceLaunchService, QualityNativeSupport)
		}
	}
}

// discoverWindows reads Chocolatey's, Scoop's and WinGet's shims.
func discoverWindows(merge mergeFunc) {
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
		if progtype.Host(candidate) {
			return candidate, true
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, true
	}
	return "", false
}
