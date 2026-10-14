//go:build unix

package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// A failed atomic write leaves the old file exactly where it was: the
// temporary file is created beside it, so a directory that cannot take one
// fails before the target is touched.
func TestWriteFileAtomicLeavesTheOldFileOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	old := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatalf("seed old file: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("make dir read-only: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	if err := writeFileAtomic(path, []byte(`{"theme":"light"}`), 0o600); err == nil {
		t.Fatal("writeFileAtomic succeeded in a read-only directory")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read old file: %v", err)
	}
	if string(got) != string(old) {
		t.Errorf("old file changed on a failed write: %s", got)
	}
}
