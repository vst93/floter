package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

// The report 「很多界面经常卡死」, second path: the action bar is rebuilt every
// frame, so a query that is a path ran `os.Stat` on it sixty times a second. On
// a network share or a sleeping disk that single call is the window stopped.
// The memo holds the answer for a quarter second, and a path that left the
// filesystem inside that band is still answered from the memo.
func TestTheTypedPathIsStatOncePerBand(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "typed")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := testApp()
	if !a.pathExists(dir) || !a.pathIsDir(dir) {
		t.Fatal("the path was not read")
	}
	file := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !a.pathExists(file) || a.pathIsDir(file) {
		t.Fatal("a file read as a directory or as missing")
	}
	// Inside the memo's band the answer stands: the second and third frames
	// of the same query cost nothing.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if !a.pathExists(file) {
		t.Error("the memo did not hold the answer inside its band")
	}
	// A different path is a different question, asked at once.
	if a.pathExists(dir) {
		t.Error("a path that is gone was reported as there")
	}
}
