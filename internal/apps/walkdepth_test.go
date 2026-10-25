package apps

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"floter/internal/xdg"
)

// writeTreeFile creates path (and its parents) with a byte in it. The new tests
// run on every platform, so they do not use `write` from apps_test.go, which is
// built on Unix only.
func writeTreeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stampTreeFile(t *testing.T, path string, when int64) {
	t.Helper()
	stamp := time.Unix(when, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

// The macOS scan stops where the reference's does: a bundle three folders below
// a root is a row, one four folders below is not (`collect_apps`,
// `main:src-tauri/src/commands/apps/macos.rs`, `if depth > 2 { return; }`).
// Before this the walk had no cap at all, so a helper bundle nested inside a
// folder that is not itself a bundle (`Xcode.app/Contents/...` has no `.app`
// suffix, so the walk descends into it) was a row.
func TestTheDarwinScanStopsAtTheReferencesDepth(t *testing.T) {
	root := t.TempDir()
	writeTreeFile(t, filepath.Join(root, "a", "b", "Kept.app", "Contents", "Info.plist"))
	writeTreeFile(t, filepath.Join(root, "a", "b", "c", "Dropped.app", "Contents", "Info.plist"))

	got := names(ScanDarwin(root))
	if want := []string{"Kept"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ScanDarwin names = %v, want %v", got, want)
	}
}

// The Windows scan stops where the reference's does: a shortcut five folders
// below a root is a row, one six folders below is not (`collect_shortcuts`,
// `main:src-tauri/src/commands/apps/windows.rs`, `if depth > 4 { return; }`).
func TestTheWindowsScanStopsAtTheReferencesDepth(t *testing.T) {
	root := t.TempDir()
	writeTreeFile(t, filepath.Join(root, "a", "b", "c", "d", "Kept.lnk"))
	writeTreeFile(t, filepath.Join(root, "a", "b", "c", "d", "e", "Dropped.lnk"))

	got := names(ScanWindows(root))
	if want := []string{"Kept"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ScanWindows names = %v, want %v", got, want)
	}
}

// The three depths are the reference's numbers, not one number picked here:
// macOS publishes at two levels, Linux at three, Windows at four. The table is
// also what the cache's signature uses, so it is asked as a value rather than
// read from the host.
func TestTheWalkDepthsAreTheReferences(t *testing.T) {
	if darwinWalkDepth != 2 {
		t.Errorf("darwinWalkDepth = %d, want 2", darwinWalkDepth)
	}
	if windowsWalkDepth != 4 {
		t.Errorf("windowsWalkDepth = %d, want 4", windowsWalkDepth)
	}
	if xdg.DesktopWalkDepth != 3 {
		t.Errorf("xdg.DesktopWalkDepth = %d, want the reference's 3", xdg.DesktopWalkDepth)
	}
	if got := appWalkDepth("linux"); got != 3 {
		t.Errorf("appWalkDepth(linux) = %d, want the desktop-entry walk's 3", got)
	}
	if got := appWalkDepth("darwin"); got != darwinWalkDepth {
		t.Errorf("appWalkDepth(darwin) = %d, want %d", got, darwinWalkDepth)
	}
	if got := appWalkDepth("windows"); got != windowsWalkDepth {
		t.Errorf("appWalkDepth(windows) = %d, want %d", got, windowsWalkDepth)
	}
}

// The signature has to look exactly as deep as the platform's scan reads: an
// entry the scan lists but the signature never hashes is a cache that keeps
// serving a list the disk no longer matches. It used to stop at two levels on
// every platform, which is only macOS's depth — so on Linux a nested
// `.desktop` four levels down, and on Windows a `.lnk` five levels down, could
// appear without the scan being rerun.
func TestTheSignatureCoversTheDepthEachPlatformsScanReads(t *testing.T) {
	root := t.TempDir()
	at3 := filepath.Join(root, "a", "b", "three")
	at4 := filepath.Join(root, "a", "b", "c", "four")
	at5 := filepath.Join(root, "a", "b", "c", "d", "five")
	at6 := filepath.Join(root, "a", "b", "c", "d", "e", "six")
	for _, path := range []string{at3, at4, at5, at6} {
		writeTreeFile(t, path)
	}

	for _, tc := range []struct {
		platform string
		deepest  string // the last level that platform's scan reads
		tooDeep  string // one level below it
	}{
		{"darwin", at3, at4},
		{"linux", at4, at5},
		{"windows", at5, at6},
	} {
		before := signatureFor(tc.platform, []string{root})
		stampTreeFile(t, tc.tooDeep, 1000000)
		if got := signatureFor(tc.platform, []string{root}); got != before {
			t.Errorf("%s: a change below the scan's depth changed the signature", tc.platform)
		}
		stampTreeFile(t, tc.deepest, 2000000)
		if got := signatureFor(tc.platform, []string{root}); got == before {
			t.Errorf("%s: a change at the scan's depth did not change the signature", tc.platform)
		}
	}
}
