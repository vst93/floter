package xdg

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// symlinkEntry is a directory entry the test makes up: `os.ReadDir` only
// produces one of these for a real symlink, and creating a symlink needs a
// privilege Windows does not always grant. What the walk does with the entry is
// the thing under test, so the entry itself can be fabricated.
type symlinkEntry struct{ name string }

func (e symlinkEntry) Name() string               { return e.name }
func (e symlinkEntry) IsDir() bool                { return false }
func (e symlinkEntry) Type() fs.FileMode          { return os.ModeSymlink }
func (e symlinkEntry) Info() (fs.FileInfo, error) { return nil, errors.New("not needed") }

// A symlinked folder counts as a folder: the reference's `Path::is_dir()`
// follows the link, so a distribution that publishes its entries through a
// symlinked directory (`/usr/share/applications` is one on some systems) keeps
// working. A symlink to a file is still a file.
func TestASymlinkedDirectoryCountsAsADirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "linked-dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := writeEntry(t, filepath.Join(root, "entry.desktop"), "[Desktop Entry]\n")

	if !isDirectory(dir, symlinkEntry{name: "linked-dir"}) {
		t.Error("a symlink to a directory was not treated as a directory")
	}
	if isDirectory(file, symlinkEntry{name: "entry.desktop"}) {
		t.Error("a symlink to a file was treated as a directory")
	}
	// A plain file entry is not stat'ed at all: the type bits already answer.
	if isDirectory(file, entryNamed(t, file)) {
		t.Error("a plain file was treated as a directory")
	}
}

// entryNamed is the `os.DirEntry` a real `os.ReadDir` would hand out for path.
func entryNamed(t *testing.T, path string) fs.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == filepath.Base(path) {
			return entry
		}
	}
	t.Fatalf("%s is not in its own directory", path)
	return nil
}

// The walk asks the directory probe rather than reading the entry's type bits
// itself: that is the seam a symlinked directory is followed through, so the
// wiring has to be visible to a pin (a probe that says "no" has to stop the
// descent, on every platform and without creating a symlink).
func TestWalkEntriesAsksTheDirectoryProbe(t *testing.T) {
	root := t.TempDir()
	writeEntry(t, filepath.Join(root, "nested", "deep.desktop"), "[Desktop Entry]\n")

	walked := func() []string {
		var got []string
		WalkEntries(root, DesktopWalkDepth, func(path string, _ fs.DirEntry, _ bool) bool {
			got = append(got, filepath.Base(path))
			return true
		})
		return got
	}

	// The positive control first: with the real probe the nested entry is read.
	if got := walked(); len(got) != 2 {
		t.Fatalf("walked %v, want the nested directory and its entry", got)
	}

	original := directoryOf
	directoryOf = func(string, fs.DirEntry) bool { return false }
	defer func() { directoryOf = original }()
	if got := walked(); len(got) != 1 {
		t.Fatalf("walked %v with a probe that never says directory; want only the top level", got)
	}
}
