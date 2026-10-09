// Package drops normalizes what the desktop hands a drag-and-drop.
//
// The platform reports absolute paths on the desktops this app ships for, but
// the event is a transport, not a contract: this package is the contract. It
// expands `~`, resolves a relative path against the directory the drop was
// resolved in, and drops entries that are not there — and it does nothing else.
// A dropped file becomes rows the user can run; nothing is opened, spawned or
// copied on arrival.
package drops

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// File is one dropped path, normalized and ready to render.
type File struct {
	// Path is the absolute path every action is built on.
	Path string
	// Name is the file name, the row's title.
	Name string
	// Directory is the containing directory, the row's second line. A path
	// whose parent cannot be read (a root, say) is its own directory.
	Directory string
	// IsDirectory: a folder is a file too — it opens in the file manager,
	// enters itself, and copies its own path.
	IsDirectory bool
}

// Normalize resolves and filters dropped paths. Order is preserved: the drop
// order is the order the user selected the files in.
func Normalize(paths []string, home, cwd string) []File {
	files := make([]File, 0, len(paths))
	for _, path := range paths {
		if file, ok := normalize(path, home, cwd); ok {
			files = append(files, file)
		}
	}
	return files
}

// normalize resolves one path, reporting whether it exists.
func normalize(path, home, cwd string) (File, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return File{}, false
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home == "" {
			return File{}, false
		}
		path = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
	}
	if !filepath.IsAbs(path) {
		// A relative path resolves against the directory the drop was
		// resolved in, never against the process's own working directory.
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return File{}, false
	}
	directory := filepath.Dir(path)
	if _, err := os.Stat(directory); err != nil {
		directory = path
	}
	return File{
		Path:        path,
		Name:        filepath.Base(path),
		Directory:   directory,
		IsDirectory: info.IsDir(),
	}, true
}

// DirectoryForCD is the directory an "open a terminal here" action uses: for a
// folder, entering it; for a file, standing next to it — the rule a shell's
// tab-completion teaches.
func DirectoryForCD(file File) string {
	if file.IsDirectory {
		return file.Path
	}
	return file.Directory
}

// ShellQuote quotes a path for an interactive shell, but only when it needs
// it: a plain path is left alone, so the command a user sees is the command
// they would have typed. A path with a space, a quote or a shell metacharacter
// is single-quoted, with an embedded quote closed and reopened the POSIX way.
// Windows' cmd has no such escape, so there a double-quoted string is used and
// embedded quotes are dropped rather than guessed at.
func ShellQuote(value string, windows bool) string {
	if windows {
		if !strings.ContainsAny(value, " \t\"&|<>^%") {
			return value
		}
		return `"` + strings.ReplaceAll(value, `"`, "") + `"`
	}
	if !strings.ContainsAny(value, " \t'\"\\$`&|;<>()*?[]{}~!#") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// CDCommand is the command line that moves a shell into a directory.
func CDCommand(directory string) string {
	return "cd " + ShellQuote(directory, runtime.GOOS == "windows")
}
