package apps

import (
	"path/filepath"
	"testing"

	"floter/internal/progname"
)

// fileStem is Rust's `Path::file_stem`, which is what both `ShortcutTargetName`
// (`windows.rs:321-324`) and `desktopFileStem` (`linux.rs:434-440`) are built
// on: the last extension goes, a leading dot is not an extension, and a name
// with nothing to drop comes back unchanged.
func TestFileStemDropsTheFinalExtension(t *testing.T) {
	cases := []struct{ in, want string }{
		{"rg.exe", "rg"},
		{"RG.EXE", "RG"},
		{"report.tar.gz", "report.tar"}, // one segment only
		{"app.", "app"},                 // a trailing dot is a separator
		{".desktop", ".desktop"},        // a leading dot is not
		{".hidden.txt", ".hidden"},
		{"no-ext", "no-ext"},
		{"", ""},
		{"org.kde.konsole.desktop", "org.kde.konsole"},
	}
	for _, tc := range cases {
		if got := fileStem(tc.in); got != tc.want {
			t.Errorf("fileStem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// desktopFileStem had no direct pin at all: it is the `.desktop` half of the
// "any extension" rule, and a dotted id like `org.kde.konsole` must keep its
// inner dots so `identifierAliases` still sees the segments.
func TestDesktopFileStemIsTheBaseNameWithoutItsExtension(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/usr/share/applications/org.kde.konsole.desktop", "org.kde.konsole"},
		{"/usr/share/applications/kde4/konsole.desktop", "konsole"},
		{"/usr/share/applications/firefox.desktop", "firefox"},
		{"/usr/share/applications/no-ext", "no-ext"},
		{"/usr/share/applications/.desktop", ".desktop"},
	}
	for _, tc := range cases {
		if got := desktopFileStem(tc.in); got != tc.want {
			t.Errorf("desktopFileStem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// And the stem is what the aliases are built from: the id's segments are
	// still there to be split.
	if got := identifierAliases(desktopFileStem("/usr/share/applications/org.kde.konsole.desktop")); len(got) != 2 ||
		got[0] != "konsole" || got[1] != "kde" {
		t.Errorf("identifierAliases(desktopFileStem(org.kde.konsole.desktop)) = %v, want [konsole kde]", got)
	}
}

// The divergence pin. `apps` drops any final extension and keeps the case
// (`windows.rs:321` `file_stem`); `progname` drops only a launcher suffix and
// lowercases, because its answer is a connected program's command word and
// `report.tar.gz` must not become `report.tar`. Merging the two rules — in
// either direction — renames programs the user can see, so this test names the
// names they disagree on. If it goes red, the round that made it red has to say
// which surface it was willing to rename.
func TestTheTwoStemRulesDisagreeOnPurpose(t *testing.T) {
	cases := []struct {
		name string
		apps string // apps.fileStem
		prog string // progname.Stem
		same bool
	}{
		{"rg.exe", "rg", "rg", true}, // a launcher suffix: both agree
		{"rg.exe.txt", "rg.exe", "rg.exe.txt", false},
		{"report.tar.gz", "report.tar", "report.tar.gz", false},
		{"RG.CMD", "RG", "rg", false}, // progname lowercases, apps does not
		{"no-ext", "no-ext", "no-ext", true},
	}
	for _, tc := range cases {
		appsStem := fileStem(tc.name)
		progStem := progname.Stem(tc.name)
		if appsStem != tc.apps {
			t.Errorf("apps.fileStem(%q) = %q, want %q", tc.name, appsStem, tc.apps)
		}
		if progStem != tc.prog {
			t.Errorf("progname.Stem(%q) = %q, want %q", tc.name, progStem, tc.prog)
		}
		if (appsStem == progStem) != tc.same {
			t.Errorf("the two stem rules agree on %q (%q vs %q) but this pin says same=%v",
				tc.name, appsStem, progStem, tc.same)
		}
	}
	// The shortcut surface keeps the program's own spelling, so the alias of a
	// Chinese-named shortcut stays typeable the way the program is installed.
	if got := ShortcutTargetName(`C:\Program Files\WXWork\WXWork.exe`); got != "WXWork" {
		t.Errorf("ShortcutTargetName = %q, want WXWork (apps keeps the case)", got)
	}
	if got := progname.Stem("WXWork.exe"); got != "wxwork" {
		t.Errorf("progname.Stem(WXWork.exe) = %q, want wxwork (a command word is lowercase)", got)
	}
}

// The three faces of one file name, pinned against each other. P10-a pinned two
// of them (`fileStem` vs `progname.Stem`); the third — the alias a `.desktop`'s
// `Exec=` value contributes, `executableName` (`apps.go:732`, Rust's
// `executable_name` at `linux.rs:458-466`, i.e. `file_name`) — strips nothing,
// and it is what the launcher's search actually matches on. So one installed
// program has three answers:
//
//	alias face    `/c/bin/RG.EXE` → `RG.EXE`   (strips nothing, keeps case)
//	stem face     `/c/bin/RG.EXE` → `RG`       (any extension, keeps case)
//	command face  `/c/bin/RG.EXE` → `rg`       (a launcher suffix, lowercased)
//
// The command face is the narrow one on purpose: `extensions.validCustomCommand`
// (`customvalidate.go:118`) accepts lowercase letters, digits, `-` and `_` only,
// so the stem face's `RG` is not a command word at all. Merging any two of these
// three renames something the user can see — a search alias, or the word that
// summons a connected tool — so this test names the names they disagree on.
func TestTheThreeNameFacesDisagreeOnPurpose(t *testing.T) {
	cases := []struct {
		path string
		// alias is what `executableName` (the `Exec=`/`TryExec=` face) hands
		// back; stem is `fileStem`; cmd is `progname.FromPath`.
		alias, stem, cmd string
	}{
		{"/c/bin/RG.EXE", "RG.EXE", "RG", "rg"},
		{"/c/bin/rg.exe", "rg.exe", "rg", "rg"},
		{"/c/bin/rg.exe.txt", "rg.exe.txt", "rg.exe", "rg.exe.txt"},
		{"/opt/report.tar.gz", "report.tar.gz", "report.tar", "report.tar.gz"},
		{"/usr/bin/foo", "foo", "foo", "foo"},
	}
	for _, tc := range cases {
		if got := executableName([]string{tc.path, "%U"}); got != tc.alias {
			t.Errorf("alias face of %q = %q, want %q (strips nothing)", tc.path, got, tc.alias)
		}
		if got := fileStem(filepath.Base(tc.path)); got != tc.stem {
			t.Errorf("stem face of %q = %q, want %q (any extension)", tc.path, got, tc.stem)
		}
		if got := progname.FromPath(tc.path); got != tc.cmd {
			t.Errorf("command face of %q = %q, want %q (a launcher suffix, lowercased)", tc.path, got, tc.cmd)
		}
	}
	// The consequences, not just the spellings: the alias face never loses a
	// character, the stem face loses exactly one dot segment, and only the
	// command face is lowercase — which is what makes it a legal command word.
	if got, want := executableName([]string{"/c/bin/RG.EXE"}), "RG.EXE"; got != want {
		t.Fatalf("alias face = %q, want %q", got, want)
	}
	if got := fileStem("RG.EXE"); got != "RG" {
		t.Fatalf("stem face = %q, want RG (case kept, extension gone)", got)
	}
	if got := progname.FromPath("/c/bin/RG.EXE"); got != "rg" {
		t.Fatalf("command face = %q, want rg (lowercased)", got)
	}
}
