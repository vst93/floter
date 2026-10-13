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

// normalize resolves one path, reporting whether it is there.
func normalize(path, home, cwd string) (File, bool) {
	// The trim decides only whether there is anything to look at. The
	// reference trims for its emptiness test and then hands the *raw* string to
	// its resolver, so `" /tmp/a "` is a path that is not there rather than one
	// silently repaired into `/tmp/a`.
	if strings.TrimSpace(path) == "" {
		return File{}, false
	}
	if strings.HasPrefix(path, "~") {
		if home == "" {
			return File{}, false
		}
		// A leading `~` that is the whole path or is followed by a separator
		// is the home directory. `~alice` is *not* another user's home: a
		// name that just happens to start with a tilde is a directory
		// literally called that, so it is joined to home rather than resolved
		// — exactly the rule `resolve` in the reference follows.
		rest := path[1:]
		if rest != "" && !os.IsPathSeparator(rest[0]) {
			path = filepath.Join(home, path)
		} else {
			path = filepath.Join(home, rest)
		}
	}
	if !filepath.IsAbs(path) {
		// A relative path resolves against the directory the drop was
		// resolved in, never against the process's own working directory.
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)
	// Existence is `symlink_metadata`: a dangling symlink is still a thing the
	// user dropped and can still be opened, entered and copied — refusing it
	// because its target is gone would drop the row silently. Whether it is a
	// directory is the opposite question and follows the link (the reference's
	// `is_dir`), so a dangling symlink reports `false` and a symlink to a
	// directory reports `true`.
	if _, err := os.Lstat(path); err != nil {
		return File{}, false
	}
	isDirectory := false
	if info, err := os.Stat(path); err == nil {
		isDirectory = info.IsDir()
	}
	directory := filepath.Dir(path)
	if _, err := os.Stat(directory); err != nil {
		directory = path
	}
	return File{
		Path:        path,
		Name:        filepath.Base(path),
		Directory:   directory,
		IsDirectory: isDirectory,
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
