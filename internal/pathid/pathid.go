// Package pathid answers one question for the whole build: do these two
// strings name the same file?
//
// The question came up in six places and got six answers. They disagreed on
// every axis that matters — the empty string, letter case, symlinks, hardlinks —
// and two of those disagreements were live bugs: a Linux migration treated
// `/tmp/A` and `/tmp/a` as one file (`strings.EqualFold` folds on a filesystem
// that does not), and the "detected tools" list kept offering a tool the user
// had already connected, because the set it looked in held the resolved
// spelling while the lookup used the symlink's (detectedtools.go).
//
// The answer here is the filesystem's own: two paths that both exist are the
// same file exactly when `os.SameFile` says so. That single call already
// covers what a string comparison cannot — a hardlink (two names, one inode),
// a Windows 8.3 short name (`PROGRA~1`), a case-insensitive volume (Windows
// always, macOS by default) — and it covers them on all three platforms,
// because each platform's stat is the authority on its own filesystem.
//
// The string key stays for the case the filesystem cannot answer: a path that
// is not there any more (an uninstalled tool, a settings file a migration is
// about to write). `Key` is that spelling: absolute, cleaned, forward-slashed,
// and lowercased **only on Windows**, because that is the one place where two
// spellings differing in case are guaranteed to be one file.
package pathid

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Key returns path's canonical spelling, for set membership.
//
// It is the fallback identity, not the authority: use `Same` when both paths
// can be handed to the filesystem. Empty in, empty out — an empty path is not
// a file, and must not become the working directory via `filepath.Abs`.
func Key(path string) string { return KeyFor(runtime.GOOS, path) }

// KeyFor is Key for a named platform. The source tables in this build carry
// Windows paths on every host (the inventory ranks `C:\ProgramData\...` roots
// while running on Linux), and a test can only assert the Windows fold from a
// non-Windows host through this entry point.
func KeyFor(platform, path string) string {
	if path == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = filepath.Clean(path)
	}
	key := filepath.ToSlash(filepath.Clean(absolute))
	if platform == "windows" {
		// The filesystem compares without case there, so the key must too:
		// `C:\Program Files\Foo\bar.exe` and the same path in lower case are
		// one file, and a set that holds one must answer for the other.
		key = strings.ToLower(key)
	}
	return key
}

// Keys returns every spelling of path that names the same file: the key
// itself, plus the key of the resolved target when the filesystem can follow
// the path. A caller that keeps a set of paths (rather than comparing two at a
// time) has to store all of them, and look up with all of them, or a symlink
// and its target read as two different programs.
func Keys(path string) []string {
	if path == "" {
		return nil
	}
	key := Key(path)
	keys := []string{key}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		if resolvedKey := Key(resolved); resolvedKey != key {
			keys = append(keys, resolvedKey)
		}
	}
	return keys
}

// Same reports whether a and b name the same file.
//
// An empty path matches nothing, including another empty path: "" is the
// absence of a program, not a program two callers agree on. Two paths that
// share a spelling are the same file even when it is gone (the migration
// compares a source it read with a destination it is about to write); anything
// else is the filesystem's call.
func Same(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	for _, left := range Keys(a) {
		for _, right := range Keys(b) {
			if left == right {
				return true
			}
		}
	}
	// Two paths that exist can still be one file without sharing a spelling:
	// a hardlink, a Windows short name, or a case-insensitive volume whose
	// case the string fold above deliberately keeps (macOS).
	leftInfo, leftErr := os.Stat(a)
	rightInfo, rightErr := os.Stat(b)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

// SameSpelling reports whether two paths are the same *spelling* of a path,
// folding separators and case unconditionally.
//
// This is the answer for a path table that describes another platform: the
// tool roots in `internal/inventory` are Windows paths (`C:\ProgramData\
// chocolatey\bin`) read on every host, so "the same root spelled two ways"
// cannot be asked of this filesystem, and Windows accepts either separator and
// compares without case. It must not be used for two files that exist here —
// on Linux `/opt/RG` and `/opt/rg` are two directories.
func SameSpelling(a, b string) bool {
	fold := func(value string) string {
		value = strings.ReplaceAll(value, `\`, "/")
		return strings.ToLower(filepath.ToSlash(filepath.Clean(value)))
	}
	return fold(a) == fold(b)
}
