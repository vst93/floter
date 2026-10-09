package extensions

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// The conventional tool directories a Finder/Dock launch never inherits,
// from runtime_path.rs. The process PATH comes first, so a directory the user
// actually has can never be shadowed by a fallback.
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
		for _, dir := range splitPath(os.Getenv("PATH")) {
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
	return splitPath(SearchPath())
}

// LookTool finds the first of the names that exists as a runnable file in
// the search path, and returns its full path.
func LookTool(names ...string) (string, bool) {
	return lookToolIn(SearchDirectories(), names...)
}

// lookToolIn is LookTool over an explicit directory list, so a test can point
// it at a fixture instead of depending on the machine's PATH.
func lookToolIn(dirs []string, names ...string) (string, bool) {
	for _, dir := range dirs {
		for _, name := range names {
			if name == "" {
				continue
			}
			candidate := filepath.Join(dir, name)
			if isExecutableFile(candidate) {
				return candidate, true
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
		// version manager's hooks both live.
		cmd := exec.Command(shell, "-ilc", `printf "__floter_path__%s" "$PATH"`)
		out, err := cmd.Output()
		if err != nil {
			done <- ""
			return
		}
		done <- string(out)
	}()
	select {
	case out := <-done:
		const sentinel = "__floter_path__"
		index := strings.LastIndex(out, sentinel)
		if index < 0 {
			return nil
		}
		return splitPath(strings.TrimSpace(out[index+len(sentinel):]))
	case <-time.After(5 * time.Second):
		return nil
	}
}

// isExecutableFile reports whether path is a regular file the user can run.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func splitPath(path string) []string {
	var out []string
	for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
		if dir != "" {
			out = append(out, dir)
		}
	}
	return out
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
