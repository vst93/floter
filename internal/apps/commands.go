package apps

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// CommandDirs is the directories the shell's commands live in: the process's
// PATH, split, with empty entries dropped. A tool found here is run in the
// terminal rather than launched as an application.
func CommandDirs() []string {
	var out []string
	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

// ScanCommands lists the runnable commands in the given directories, one
// entry per name (the first directory wins, as a shell resolves them),
// sorted by name.
//
// It is deliberately shallow: only the directories themselves are listed,
// and a name is taken when it is a regular, executable file. `maxCommands`
// bounds the result so a huge PATH cannot make the launcher's list enormous.
func ScanCommands(dirs []string) []App {
	byName := map[string]App{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if _, taken := byName[name]; taken {
				continue
			}
			if entry.IsDir() || !isCommandName(name) {
				continue
			}
			path := filepath.Join(dir, name)
			if !runnable(path) {
				continue
			}
			byName[name] = App{Name: name, Path: path}
		}
	}

	out := make([]App, 0, len(byName))
	for _, app := range byName {
		out = append(out, app)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > maxCommands {
		out = out[:maxCommands]
	}
	return out
}

// maxCommands bounds a scan.
const maxCommands = 800

// isCommandName skips what is never a command: dot files, editor backups and
// the like.
func isCommandName(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return false
	}
	switch filepath.Ext(name) {
	case ".dylib", ".so", ".a", ".o", ".h", ".txt", ".md", ".json", ".1", ".gz", ".zip":
		return false
	}
	return true
}

// runnable reports whether a path is a regular file the user can run.
func runnable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		// On Windows a command is a .exe, .cmd or .bat; anything else is
		// data.
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe", ".cmd", ".bat", ".com":
			return true
		default:
			return false
		}
	}
	return info.Mode().Perm()&0o111 != 0
}
