package clipboard

import (
	"path/filepath"
	"strings"
)

// Caps on one capture, the reference's own numbers (the old build's
// `clipboard_history/monitor.rs:24-30`). A clipboard holding more than these is
// not a copy the history can keep, so the whole sample is skipped rather than
// truncated: half a screenshot or half a file list is worse than nothing.
const (
	// MaxFiles is the most paths one file-list capture may carry.
	MaxFiles = 4096
	// MaxFilePathBytes is the ceiling on the sum of a file list's path bytes.
	MaxFilePathBytes = 512 * 1024
	// MaxTextBytes is the ceiling on a text capture, and on an image's caption.
	MaxTextBytes = 512 * 1024
	// MaxImagePNGBytes is the ceiling on an image capture's encoded PNG.
	MaxImagePNGBytes = 20 * 1024 * 1024
)

// FilesCapture is a validated file-list capture: the path strings to store,
// exactly as the clipboard handed them over, plus the identity hash computed
// from their canonical forms.
//
// The two differ on purpose. Canonicalizing is how the same file copied through
// a symlink or a `..` spelling dedupes against itself, but history never copies
// contents and has no business rewriting the path the user copied either.
type FilesCapture struct {
	Paths []string
	Hash  string
}

// PrepareFiles decides whether a clipboard file list is worth recording,
// without touching the filesystem beyond the caller's `exists` oracle (so the
// decision stays fs-free in tests).
//
// Paths that are empty, repeated or no longer there are dropped; if none
// survive, the copy is skipped and the caller falls through to the text and
// image renditions of the same copy. The caps are checked against the list as
// it arrived, before any filtering, because they are about what the clipboard
// is asking the history to swallow.
func PrepareFiles(paths []string, exists func(string) bool) (FilesCapture, bool) {
	if len(paths) > MaxFiles {
		return FilesCapture{}, false
	}
	total := 0
	for _, path := range paths {
		total += len(path)
	}
	if total > MaxFilePathBytes {
		return FilesCapture{}, false
	}
	if exists == nil {
		exists = func(string) bool { return true }
	}
	originals := make([]string, 0, len(paths))
	canonical := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, raw := range paths {
		if strings.TrimSpace(raw) == "" || seen[raw] {
			continue
		}
		seen[raw] = true
		if !exists(raw) {
			continue
		}
		originals = append(originals, raw)
		canonical = append(canonical, canonicalPath(raw))
	}
	if len(originals) == 0 {
		return FilesCapture{}, false
	}
	return FilesCapture{Paths: originals, Hash: HashText(strings.Join(canonical, "\n"))}, true
}

// canonicalPath is the path's resolved spelling, or the path itself when it
// cannot be resolved: an entry the filesystem refuses to follow still counts as
// the path the user copied.
func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
