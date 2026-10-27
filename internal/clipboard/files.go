package clipboard

import (
	"encoding/binary"
	"path/filepath"
	"slices"
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
	return FilesCapture{Paths: originals, Hash: FilesHash(canonical)}, true
}

// FilesHash is a file list's identity: the **set** of path strings, not the
// order they arrived in. Sorting and deduplicating first is what makes a list
// copied in another selection order the same clip — a file picker's arrival
// order is the user's click order, not a fact about the copy (the old build's
// `files_hash`, `main:src-tauri/src/clipboard_history/store.rs:220-230`).
//
// The encoding is length-prefixed rather than joined: a path may contain a
// newline on Linux and macOS, so `["/a\n/b"]` and `["/a", "/b"]` are two
// different copies and must not share a digest. The `files\0` tag keeps a file
// list from ever colliding with a text entry whose text is the same bytes.
func FilesHash(paths []string) string {
	sorted := make([]string, len(paths))
	copy(sorted, paths)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	encoded := make([]byte, 0, len(paths)*16+len("files\x00"))
	encoded = append(encoded, "files\x00"...)
	for _, path := range sorted {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(path)))
		encoded = append(encoded, length[:]...)
		encoded = append(encoded, path...)
	}
	return hashBytes(encoded)
}

// filesIdentity is the one place a file list's stored identity is computed:
// the set of its **canonical** spellings, so the same file reached through a
// symlink or a `..` is one entry. PrepareFiles, Store.AddFiles and the index
// load all call it, so a list cannot hash one way on capture and another way
// after a restart.
func filesIdentity(paths []string) string {
	canonical := make([]string, len(paths))
	for index, path := range paths {
		canonical[index] = canonicalPath(path)
	}
	return FilesHash(canonical)
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
