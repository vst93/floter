package extensions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"floter/internal/pathlist"
	"floter/internal/progname"
	"floter/internal/progtype"
)

// The conventional tool directories a Finder/Dock launch never inherits. The
// process PATH comes first, so a directory the user actually has can never be
// shadowed by a fallback.
var baselineDirs = map[string][]string{
	"darwin": {"/usr/local/bin", "/usr/local/sbin", "/opt/homebrew/bin", "/opt/homebrew/sbin"},
	"linux":  {"/usr/local/bin", "/usr/local/sbin"},
}

var (
	searchOnce sync.Once
	searchPath string
)

// SearchPath is the PATH the extension system resolves tools through: the
// process environment, then a macOS login shell's answer (a version manager
// puts its toolchain where only the shell knows), then the conventional
// directories above. It is computed once per process.
func SearchPath() string {
	searchOnce.Do(func() {
		// The process PATH first, deduplicated as merge_path_entries did: a
		// corrupted environment can carry the same directory twice.
		var merged []string
		for _, dir := range pathlist.Split(os.Getenv("PATH")) {
			merged = appendUnique(merged, dir)
		}
		for _, dir := range loginShellDirs() {
			merged = appendUnique(merged, dir)
		}
		for _, dir := range baselineDirs[runtime.GOOS] {
			merged = appendUnique(merged, dir)
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			merged = appendUnique(merged, filepath.Join(home, ".local", "bin"))
		}
		searchPath = strings.Join(merged, string(os.PathListSeparator))
	})
	return searchPath
}

// SearchDirectories is SearchPath split, in order.
func SearchDirectories() []string {
	return pathlist.Split(SearchPath())
}

// LookTool finds the first of the names that exists as a runnable file in
// the search path, and returns its full path.
func LookTool(names ...string) (string, bool) {
	return lookToolIn(runtime.GOOS, SearchDirectories(), names...)
}

// lookTool and findInterpreter are the two lookups runtime resolution goes
// through. They are variables so a test can resolve against a fixture instead
// of depending on the machine having the package's tool installed.
var (
	lookTool        = LookTool
	findInterpreter = FindInterpreter
)

// StubToolLookup replaces the tool lookup for a test, and returns the function
// that puts it back. It is the seam the tests use to install a package whose
// toolchain is not on the machine (a fixture executable, or a tool the CI
// runner does not have).
func StubToolLookup(lookup func(names ...string) (string, bool)) func() {
	previous := lookTool
	lookTool = lookup
	return func() { lookTool = previous }
}

// StubInterpreterLookup replaces the interpreter lookup for a test, and returns
// the function that puts it back: the seam a test uses to connect a script in a
// language this machine does not have.
func StubInterpreterLookup(lookup func(language string) (string, bool)) func() {
	previous := findInterpreter
	findInterpreter = lookup
	return func() { findInterpreter = previous }
}

// lookToolIn is LookTool over an explicit directory list and platform key, so a
// test can point it at a fixture instead of depending on the machine's PATH.
//
// A name is a *probe*, not a file name: on Windows the program a bare name asks
// for is `rg.exe`, so the join is made of `progname.Ladder`. That table already
// answered this question for the tool catalog's probes (`tools.Probe`,
// catalog.go:366-370), and the reference answered it the same way
// (`executable_name_matches`, src-tauri/src/extensions/resolver.rs:118-133: an
// expected name with no extension also matches the four launcher spellings).
// Without it a bare name resolved on Linux and macOS and nowhere on Windows:
// `floter://register?cmd=rg` — the spelling `docs/extensions/README.zh-CN.md`
// documents — said "not found" on a machine whose Detected row connects the same
// `rg` in one press, and `FindInterpreter("powershell")` (whose names are the
// bare `pwsh` / `powershell`) could not resolve `pwsh.exe` at all.
//
// The directory is the outer loop and the ladder the inner one, so the search
// path keeps its priority over the suffix: `dir1/rg.cmd` still beats
// `dir2/rg.exe`.
//
// The judgement is `progtype.For`, the extension strategy: a name here is a
// *probe* the system makes on the user's behalf, not a path the user picked, so
// the file it lands on has to be a program by name. Every other name probe in
// this build asks the same question the same way (`tools.Probe`,
// catalog.go:307; the `.desktop` program probe, apps.go:485; the inventory's
// PATH walk, inventory.go:146), and the reference's PATH discovery filters each
// directory entry through its extension strategy too (inventory.rs:439,
// is_executable at :1088). The permissive `progtype.ForLenient` belongs to the
// three places where the user hands over a path that already exists
// (`provider.go:53`, `reconnect.go:68`, `custom.go:253`) — a wrong "no" there
// would hide the program they just chose. Getting *this* one wrong is the other
// direction: a wrong "yes" puts a file Windows cannot run in front of the user
// as an integration to connect (`floter://register?cmd=notes.txt`), as a
// `.desktop` entry's target, or as a chooser row.
func lookToolIn(platform string, dirs []string, names ...string) (string, bool) {
	for _, dir := range dirs {
		for _, name := range names {
			if name == "" {
				continue
			}
			for _, probe := range progname.Ladder(platform, name) {
				candidate := filepath.Join(dir, probe)
				if progtype.For(platform, candidate) {
					return candidate, true
				}
			}
		}
	}
	return "", false
}

// InterpreterNames lists the interpreters that can run a script language, in
// the order the old host tried them (from script_toolchain).
func InterpreterNames(language string) []string {
	switch language {
	case "js":
		return []string{"node"}
	case "shell":
		return []string{"sh"}
	case "powershell":
		return []string{"pwsh", "powershell"}
	case "python":
		return []string{
			"python3", "python",
			"python3.14", "python3.13", "python3.12", "python3.11",
			"python3.10", "python3.9", "python3.8",
		}
	case "ruby":
		return []string{"ruby"}
	case "php":
		return []string{"php"}
	default:
		// go and rust are compiled: the toolchain builds the source at
		// install time and the artifact runs, so there is no interpreter to
		// find here.
		return nil
	}
}

// InterpreterArgs is the argv a script needs after the interpreter: the
// script path, with PowerShell's -File flag.
func InterpreterArgs(language, script string) []string {
	if language == "powershell" {
		return []string{"-File", script}
	}
	return []string{script}
}

// FindInterpreter resolves a script language's interpreter through the
// search path.
func FindInterpreter(language string) (string, bool) {
	names := InterpreterNames(language)
	if len(names) == 0 {
		return "", false
	}
	return LookTool(names...)
}

// loginShellDirs asks the user's login shell for its PATH once, on macOS: a
// Finder launch has the launchd baseline, which knows nothing of a version
// manager's toolchain. Best-effort by contract — a failure, a timeout or
// unparseable output yields nothing, and the static baseline still applies.
func loginShellDirs() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/zsh"
	}
	done := make(chan string, 1)
	go func() {
		// -ilc: interactive and login, which is where path_helper and a
		// version manager's hooks both live. The five seconds are the same
		// bound the abandoned-goroutine version used; now the shell dies with
		// it instead of living on with no reader.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := newProbeCommand(ctx, shell, "-ilc", `printf "__floter_path__%s" "$PATH"`)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := runExtensionCommand(cmd); err != nil {
			done <- ""
			return
		}
		done <- out.String()
	}()
	select {
	case out := <-done:
		const sentinel = "__floter_path__"
		index := strings.LastIndex(out, sentinel)
		if index < 0 {
			return nil
		}
		return pathlist.Split(strings.TrimSpace(out[index+len(sentinel):]))
	case <-time.After(5 * time.Second):
		return nil
	}
}

func appendUnique(list []string, value string) []string {
	if value == "" {
		return list
	}
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
