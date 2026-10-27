package statefile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// leftoverTemps reports the write-in-flight files a directory still holds.
func leftoverTemps(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var left []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".floter-state-") && strings.HasSuffix(entry.Name(), ".tmp") {
			left = append(left, entry.Name())
		}
	}
	return left
}

// One write leaves exactly the new bytes at the target, with the caller's
// mode, and nothing else in the directory.
func TestWriteLeavesTheNewBytesAndNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Write(path, []byte("new\n"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new\n" {
		t.Errorf("target = %q, want the new bytes verbatim", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("mode = %o, want 600", mode)
		}
	}
	if left := leftoverTemps(t, dir); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

// The directory a first run needs is created, not required.
func TestWriteCreatesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "floter", "nested", "index.json")

	if err := Write(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

// Bytes are not re-spelled on the way through: an empty document stays empty
// and a trailing newline stays a trailing newline (the callers' own contract,
// which a marshal-then-write helper must not quietly change).
func TestWriteKeepsTheBytesVerbatim(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"empty.json":   "",
		"newline.json": "{\"a\": 1}\n",
	} {
		path := filepath.Join(dir, name)
		if err := Write(path, []byte(body), 0o600); err != nil {
			t.Fatalf("%s: Write: %v", name, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: read: %v", name, err)
		}
		if string(got) != body {
			t.Errorf("%s = %q, want %q", name, got, body)
		}
	}
}

// A flush that fails stops the write: the target keeps the old document and
// the temporary file is gone. This is what pins "flushed *before* renamed" —
// dropping the flush makes this test see a successful write.
func TestASyncFailureLeavesTheTargetUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	refused := errors.New("disk said no")
	previous := syncFile
	syncFile = func(*os.File) error { return refused }
	t.Cleanup(func() { syncFile = previous })

	err := Write(path, []byte("new\n"), 0o600)
	if !errors.Is(err, refused) {
		t.Fatalf("Write error = %v, want the refused flush", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "old\n" {
		t.Errorf("target = %q, want the old bytes", got)
	}
	if left := leftoverTemps(t, dir); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

// A refused rename stops the write the same way, and the target is still the
// old document: the new bytes only ever live in the temporary file. This is
// what pins "renamed, not written in place" — a helper that opened the target
// directly would have already replaced its bytes.
func TestARenameFailureLeavesTheTargetUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	refused := errors.New("rename said no")
	previous := renameFile
	renameFile = func(string, string) error { return refused }
	t.Cleanup(func() { renameFile = previous })

	err := Write(path, []byte("new\n"), 0o600)
	if !errors.Is(err, refused) {
		t.Fatalf("Write error = %v, want the refused rename", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "old\n" {
		t.Errorf("target = %q, want the old bytes", got)
	}
	if left := leftoverTemps(t, dir); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}
