package apps

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"floter/internal/pathlist"
	"floter/internal/progtype"
)

// CommandDirs is the directories the shell's commands live in: the process's
// PATH, split the one way this repository splits one (pathlist.Split, which
// also drops the empty entries). A tool found here is run in the terminal
// rather than launched as an application.
func CommandDirs() []string { return pathlist.Split(os.Getenv("PATH")) }

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
			// `progtype.Host` is the same "is this a program" judgement the
			// app scan and the tool inventory use: the executable bit on Unix,
			// one of the four program extensions on Windows.
			if !progtype.Host(path) {
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
