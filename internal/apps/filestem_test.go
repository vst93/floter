package apps

import (
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
