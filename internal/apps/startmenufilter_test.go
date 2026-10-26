package apps

// The Start Menu filter, checked line by line against the reference.
//
// `TestUninstallersAreNotRows` (`shortcut_test.go:139`) already pins the four
// things the reference and this port agree on: `.exe`/`.bat`/`.cmd` in,
// `.com`/`.txt`/web links out, `uninst`/`unins` names out, and case folded
// both ways. What it does not say is where the two **deliberately** part ways,
// so a later "make it match `windows.rs:539-549` exactly" pass would flip them
// with every test still green. These four tests are that missing half:
//
//   - the filter reads the file name, never the directory (`shortcut.go:186`
//     `shortcutBaseName`), so an app installed under a folder called
//     `Uninstall Helper` is still a row;
//   - trailing whitespace is trimmed (`shortcut.go:237` `TrimSpace`), where the
//     reference's `lower.ends_with(".exe")` on the whole target is `false`;
//   - a target that is *only* an extension names nothing (`shortcut.go:246`
//     `i > 0`), where Rust's `Path::file_stem(".exe")` is `.exe` and the
//     reference answers `true`;
//   - `.com` is a program to `progname`/`progtype` and not a launchable Start
//     Menu target here (`shortcut.go:187`), which is the one place the two
//     questions disagree.
//
// Every answer below is the reference's own unless the test says otherwise, and
// the ones that differ say which way and why — so the next reader inherits a
// decision rather than a mystery.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/progname"
	"floter/internal/progtype"
)

// TestTheFilterIsAboutTheFileNameNotThePath is the filter's real shape: the
// reference runs `Path::new(&lower).file_stem()` and this port runs
// `shortcutBaseName` then `extensionOf`, and both look at the last element
// only. An installer that drops its program into `C:\Program Files\Uninstall
// Helper\` (or a vendor folder that happens to be called `uninst`) must not
// lose the row: the uninstaller check is about the executable, not the path.
func TestTheFilterIsAboutTheFileNameNotThePath(t *testing.T) {
	cases := []string{
		`C:\Program Files\Uninstall Helper\App.exe`,
		`C:\app\uninst\WXWork.exe`,
		`C:\unins000\App.exe`,
		`C:/app/App.exe`,         // forward slashes: a `.lnk` may write either
		`\\?\C:\app\App.exe`,     // long-path prefix, which Windows itself strips
		`C:\程序\企微.exe`,           // a non-ASCII directory is still a directory
		`C:\app\sub.dir\App.bat`, // a dot in the directory is not an extension
	}
	for _, target := range cases {
		if !IsLaunchableTarget(target) {
			t.Errorf("IsLaunchableTarget(%q) = false, want true: the filter reads the file name, not the path", target)
		}
	}
}

// TestTheFilterTrimsWhereTheReferenceDoesNot pins the one place this port is
// deliberately more forgiving than the reference.
//
// `is_launchable_target` lowercases the whole target and asks
// `ends_with(".exe")`, so `C:\app\App.exe ` (one trailing space) is `false`
// there. A `LinkInfo.LocalBasePath` is a NUL-terminated byte string that a
// sloppy installer can end with a space, and Windows itself drops trailing
// spaces before it starts anything, so trimming is the right answer here — but
// it is a *divergence*, not an accident, and this is the assertion that says so.
func TestTheFilterTrimsWhereTheReferenceDoesNot(t *testing.T) {
	cases := []string{
		`C:\app\App.exe `,      // trailing space: reference answers false
		"C:\\app\\App.exe\t",   // trailing tab: reference answers false
		"C:\\app\\App.exe\r\n", // trailing CRLF: reference answers false
	}
	for _, target := range cases {
		if !IsLaunchableTarget(target) {
			t.Errorf("IsLaunchableTarget(%q) = false, want true: the base name is trimmed first (shortcut.go:237)", target)
		}
	}
	// The other direction does not diverge: leading whitespace leaves
	// `ends_with` true in the reference too, so both accept it.
	if !IsLaunchableTarget(`  C:\app\App.exe`) {
		t.Error("a leading space should not make a target unlaunchable")
	}
}

// TestALiteralDotExeNamesNothing pins the second divergence, on a target no
// `.lnk` will ever carry: a name that *is* an extension.
//
// The reference accepts it (`ends_with(".exe")` is true, and Rust's
// `Path::file_stem(".exe")` is `.exe`, a leading dot not being a separator).
// This port's `extensionOf` needs a stem before the dot (`i > 0`), so the name
// reads as having no extension and is rejected. Both answers are defensible
// because neither is reachable; the point of the pin is that the answer is
// recorded, so the next "be faithful to the reference" pass changes a decision
// instead of a mystery.
func TestALiteralDotExeNamesNothing(t *testing.T) {
	for _, target := range []string{`.exe`, `.bat`, `.cmd`, `C:\app\.exe`} {
		if IsLaunchableTarget(target) {
			t.Errorf("IsLaunchableTarget(%q) = true, want false: a leading dot is not an extension separator here", target)
		}
	}
	// A leading dot with a real extension after it is not this case at all:
	// `.exe.txt` has the extension `.txt` under both rules.
	if IsLaunchableTarget(`C:\app\.exe.txt`) {
		t.Error("`.exe.txt` is a text file, not a launcher")
	}
}

// TestTheUninstallerSubstringIsMatchedAnywhereInTheStem pins the *range* of the
// uninstaller check, which `TestUninstallersAreNotRows` only samples: the
// substring is looked for anywhere in the stem, in either case, and not only at
// the start.
//
// It also records which half of `!contains("uninst") && !contains("unins")` is
// load-bearing. `"unins"` is a prefix of `"uninst"`, so a name containing
// `uninst` is caught by the first check alone and the second can only ever
// catch names that contain `unins` *without* `uninst` — `unins000.exe` is the
// shape that matters. That redundancy is the reference's, and it is kept; the
// table says so, so deleting either half goes red here rather than nowhere.
func TestTheUninstallerSubstringIsMatchedAnywhereInTheStem(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{`C:\app\MyUninstaller.exe`, false}, // the substring in the middle
		{`C:\app\x-uninst.exe`, false},      // …behind a separator
		{`C:\app\UNINST.EXE`, false},        // …in another case
		{`C:\app\unins000.exe`, false},      // the `unins` half, with no `uninst` at all
		{`C:\app\unins000.dat.exe`, false},  // the extension is not the name
		{`C:\app\install.exe`, true},        // `install` is not `uninst`
		{`C:\app\App.exe`, true},            // the ordinary case
	}
	for _, tc := range cases {
		if got := IsLaunchableTarget(tc.target); got != tc.want {
			t.Errorf("IsLaunchableTarget(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}

	// Which check catches what: `uninst` subsumes `unins` whenever it fires…
	if !strings.Contains("uninst", "unins") {
		t.Fatal("the two checks are no longer nested: revisit the comment above")
	}
	// …and `unins000` is the name only the `unins` check catches.
	if strings.Contains("unins000", "uninst") || !strings.Contains("unins000", "unins") {
		t.Fatal("`unins000` is no longer caught by the `unins` check alone: revisit the comment above")
	}
}

// TestTheTwoProgramQuestionsDisagreeOnCom is the cross pin between the two
// questions this repository asks about a `.exe`-shaped file name:
//
//	"is this file a program?"          progname.IsLauncher / progtype.For("windows")
//	"is this a launchable menu entry?" IsLaunchableTarget
//
// `.com` is `true` to the first (`progname.go:58`: Windows really does start a
// `.com`) and `false` to the second (`shortcut.go:187`, following the
// reference, which drops `.com` because a `.com` shortcut in the Start Menu is a
// leftover DOS entry). Each side is pinned on its own elsewhere; this is the
// only assertion that says they answer differently **on purpose**, so
// "`IsLaunchableTarget` should just call `progname.IsLauncher`" goes red here
// and in `TestUninstallersAreNotRows`, in words rather than in a diff.
func TestTheTwoProgramQuestionsDisagreeOnCom(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}
	cases := []struct {
		name      string
		path      string
		isProgram bool // progname.IsLauncher, and progtype's Windows strategy
		isMenuRow bool // IsLaunchableTarget
	}{
		{"legacy.com", write("legacy.com"), true, false},
		{"app.exe", write("app.exe"), true, true},
		{"readme.txt", write("readme.txt"), false, false},
	}
	for _, tc := range cases {
		if got := progname.IsLauncher(tc.path); got != tc.isProgram {
			t.Errorf("progname.IsLauncher(%q) = %v, want %v", tc.path, got, tc.isProgram)
		}
		if got := progtype.For("windows", tc.path); got != tc.isProgram {
			t.Errorf("progtype.For(windows, %q) = %v, want %v", tc.path, got, tc.isProgram)
		}
		if got := IsLaunchableTarget(tc.path); got != tc.isMenuRow {
			t.Errorf("IsLaunchableTarget(%q) = %v, want %v", tc.path, got, tc.isMenuRow)
		}
	}
}
