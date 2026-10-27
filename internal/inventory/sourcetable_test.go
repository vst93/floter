package inventory

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// platformFixture is one directory per source: what every platform root table
// is pointed at, so a pin walks a tree it wrote instead of the machine.
type platformFixture struct {
	desktop string
	flatpak []string
	snap    string
	nix     []string
	brew    []string
	macApps []string
	windows []toolRoot
}

// stubPlatformRoots replaces every platform root table, so a pin drives
// fixtures instead of the machine's package managers.
func stubPlatformRoots(t *testing.T, fixtures platformFixture) {
	t.Helper()
	previousDesktop, previousExport, previousNix := linuxDesktopRoots, linuxExportRoots, nixRoots
	previousBrew, previousMacApps, previousWindows := brewRoots, macAppRoots, windowsToolRoots
	linuxDesktopRoots = func() []string { return []string{fixtures.desktop} }
	linuxExportRoots = func() ([]string, string) { return fixtures.flatpak, fixtures.snap }
	nixRoots = func() []string { return fixtures.nix }
	brewRoots = func() []string { return fixtures.brew }
	macAppRoots = func() []string { return fixtures.macApps }
	windowsToolRoots = func() []toolRoot { return fixtures.windows }
	t.Cleanup(func() {
		linuxDesktopRoots, linuxExportRoots, nixRoots = previousDesktop, previousExport, previousNix
		brewRoots, macAppRoots, windowsToolRoots = previousBrew, previousMacApps, previousWindows
	})
}

// everyDeclaredSource is reached by exactly one root in the fixture tree. A
// declared source no discovery can produce would be a constant that only exists
// to be mentioned, and a source a discovery produces without being declared
// would be a value nothing knows about — the fixture walk catches both.
func TestEveryDeclaredSourceIsReachable(t *testing.T) {
	root := t.TempDir()
	program := func(parts ...string) string {
		return writeFile(t, filepath.Join(append([]string{root}, parts...)...), "#!/bin/sh\n")
	}

	pathDirectory := filepath.Dir(program("path", "pathtool"))
	desktopTarget := program("desktop-target", "desktopapp")
	desktopDirectory := desktopFixture(t, filepath.Join(root, "desktop"),
		"desktopapp.desktop",
		"[Desktop Entry]\nType=Application\nName=Desktop App\nExec="+desktopTarget+" %U\n")

	flatpakRoot := filepath.Dir(program("flatpak", "com.example.App"))
	nixRoot := filepath.Dir(program("nix", "nixapp"))
	snapRoot := filepath.Dir(program("snap", "snapapp"))
	brewRoot := filepath.Dir(program("brew", "brewtool"))
	program("apps", "Bundled.app", "Contents", "MacOS", "bundled")
	macAppsRoot := filepath.Join(root, "apps")
	chocolateyRoot := filepath.Dir(program("choco", "chocotool"))
	scoopRoot := filepath.Dir(program("scoop", "scooptool"))
	wingetRoot := filepath.Dir(program("winget", "wingettool"))

	stubPlatformRoots(t, platformFixture{
		desktop: desktopDirectory,
		flatpak: []string{flatpakRoot},
		snap:    snapRoot,
		nix:     []string{nixRoot},
		brew:    []string{brewRoot},
		macApps: []string{macAppsRoot},
		windows: []toolRoot{
			{chocolateyRoot, SourceChocolatey},
			{scoopRoot, SourceScoop},
			{wingetRoot, SourceWinGet},
		},
	})

	byPath := map[string]*Candidate{}
	merge := mergeInto(byPath)
	discoverPath(merge, []string{pathDirectory})
	discoverLinux(merge)
	discoverMacOS(merge)
	discoverWindows(merge)

	seen := map[DiscoverySource]bool{}
	for _, candidate := range byPath {
		for _, source := range candidate.Sources {
			seen[source] = true
		}
	}
	var got []string
	for source := range seen {
		got = append(got, string(source))
	}
	sort.Strings(got)

	var want []string
	for _, source := range discoverySources {
		want = append(want, string(source))
	}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources reached by the fixture walk =\n %v\nwant exactly the declared vocabulary\n %v", got, want)
	}
	if len(byPath) != 10 {
		t.Fatalf("fixture walk found %d candidates, want 10 (one per source): %v", len(byPath), byPath)
	}
}

// The vocabulary is closed. The reference's enum carries `dpkg`, `rpm`,
// `pacman`, `registry` and `wsl` and produces none of them, so this package
// naming them would be a source that can never appear on a candidate. `winget`
// is the one spelling where the two disagree (the reference's serde
// kebab-cases `WinGet` to `win-get`); this string never leaves the process —
// `Candidate` carries no json tags — and its only reader joins it into a
// human-readable chooser row, so it is pinned by value rather than renamed.
func TestTheSourceVocabularyIsClosed(t *testing.T) {
	seen := map[DiscoverySource]bool{}
	for _, source := range discoverySources {
		if seen[source] {
			t.Fatalf("discoverySources names %q twice", source)
		}
		seen[source] = true
		if string(source) != strings.ToLower(string(source)) {
			t.Fatalf("source %q is not lowercase", source)
		}
	}
	for _, absent := range []DiscoverySource{"dpkg", "rpm", "pacman", "registry", "wsl"} {
		if seen[absent] {
			t.Fatalf("discoverySources names %q, which the reference never produces", absent)
		}
	}
	if SourceWinGet != "winget" {
		t.Fatalf("SourceWinGet = %q, want \"winget\" (the reference's serde spelling is \"win-get\")", SourceWinGet)
	}
}

// Each platform's table is pinned by value, the way the desktop-entry roots
// are: a root that quietly disappears from the table is a package manager whose
// shims stop being found, and nothing else in the suite would notice — the
// fixture walk below stubs the tables, so it can prove a source is reachable
// but not that the production table wires it.
func TestThePlatformRootTablesAreTheReferenceTables(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("ProgramData", "")
	t.Setenv("LOCALAPPDATA", "")

	if got, want := linuxPackageRoots(), []toolRoot{
		{"/home/tester/.local/share/flatpak/exports/bin", SourceFlatpak},
		{"/var/lib/flatpak/exports/bin", SourceFlatpak},
		{"/home/tester/.nix-profile/bin", SourceNix},
		{"/nix/var/nix/profiles/default/bin", SourceNix},
		{"/snap/bin", SourceSnap},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxPackageRoots =\n %v\nwant\n %v", got, want)
	}
	if got, want := brewRoots(), []string{"/usr/local/bin", "/opt/homebrew/bin"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("brewRoots = %v, want %v", got, want)
	}
	if got, want := macAppRoots(), []string{"/Applications", "/System/Applications", "/home/tester/Applications"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("macAppRoots = %v, want %v", got, want)
	}
	if got, want := windowsToolRoots(), []toolRoot{
		{`C:\ProgramData\chocolatey\bin`, SourceChocolatey},
		{"/home/tester/scoop/shims", SourceScoop},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("windowsToolRoots =\n %v\nwant\n %v", got, want)
	}
}

// The Chocolatey root is named twice — the literal default install and
// `%ProgramData%` — because a machine that redirected the shared data directory
// has no shims at the literal path. On a default install the two spell one
// directory, and the table must walk it once.
//
// The roots are compared through `sameRoot` rather than by string: this test
// runs on whatever host the gate runs on, and `filepath.Join` spells a Windows
// path with the host's separator. `%ProgramData%` being a Windows variable, its
// spelling only matters where `filepath.Join` is Windows' — the composition is
// what is pinned here.
func TestTheWindowsChocolateyRootFollowsProgramData(t *testing.T) {
	literal := `C:\ProgramData\chocolatey\bin`

	dirsFor := func(t *testing.T, source DiscoverySource) []string {
		t.Helper()
		var dirs []string
		for _, root := range windowsToolRoots() {
			if root.source == source {
				dirs = append(dirs, root.dir)
			}
		}
		return dirs
	}

	t.Run("program data unset", func(t *testing.T) {
		t.Setenv("ProgramData", "")
		chocolatey := dirsFor(t, SourceChocolatey)
		if len(chocolatey) != 1 || !sameRoot(chocolatey[0], literal) {
			t.Fatalf("Chocolatey roots = %v, want just the literal one", chocolatey)
		}
	})

	t.Run("program data is the literal path", func(t *testing.T) {
		t.Setenv("ProgramData", `C:\ProgramData`)
		chocolatey := dirsFor(t, SourceChocolatey)
		if len(chocolatey) != 1 {
			t.Fatalf("Chocolatey roots = %v, want the default install walked once", chocolatey)
		}
	})

	t.Run("program data is redirected", func(t *testing.T) {
		t.Setenv("ProgramData", `D:\CorpData`)
		chocolatey := dirsFor(t, SourceChocolatey)
		want := filepath.Join(`D:\CorpData`, "chocolatey", "bin")
		if len(chocolatey) != 2 || !sameRoot(chocolatey[0], literal) || !sameRoot(chocolatey[1], want) {
			t.Fatalf("Chocolatey roots = %v, want [%s %s]", chocolatey, literal, want)
		}
	})

	t.Run("local app data adds the winget links", func(t *testing.T) {
		t.Setenv("ProgramData", "")
		t.Setenv("LOCALAPPDATA", `C:\Users\tester\AppData\Local`)
		winget := dirsFor(t, SourceWinGet)
		want := filepath.Join(`C:\Users\tester\AppData\Local`, "Microsoft", "WinGet", "Links")
		if len(winget) != 1 || !sameRoot(winget[0], want) {
			t.Fatalf("WinGet roots = %v, want [%s]", winget, want)
		}
	})
}

// A root named two ways is one root: Windows accepts either separator and
// compares paths without case, so the duplicate check has to fold both or a
// redirected `ProgramData` that lands on the default directory is walked twice.
func TestTwoSpellingsOfOneRootAreOneRoot(t *testing.T) {
	for _, testCase := range []struct {
		left, right string
		same        bool
	}{
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData\chocolatey\bin`, true},
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData/chocolatey/bin`, true},
		{`C:\ProgramData\chocolatey\bin`, `c:\programdata\CHOCOLATEY\bin`, true},
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData\chocolatey\bin\.`, true},
		{`C:\ProgramData\chocolatey\bin`, `D:\CorpData\chocolatey\bin`, false},
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData\chocolatey`, false},
	} {
		if got := sameRoot(testCase.left, testCase.right); got != testCase.same {
			t.Fatalf("sameRoot(%q, %q) = %v, want %v", testCase.left, testCase.right, got, testCase.same)
		}
	}

	roots := dedupeRoots([]toolRoot{
		{`C:\ProgramData\chocolatey\bin`, SourceChocolatey},
		{`c:\programdata\chocolatey\bin`, SourceChocolatey},
		{`D:\CorpData\chocolatey\bin`, SourceChocolatey},
	})
	if len(roots) != 2 || roots[0].dir != `C:\ProgramData\chocolatey\bin` || roots[1].dir != `D:\CorpData\chocolatey\bin` {
		t.Fatalf("dedupeRoots = %v, want the first spelling kept and the redirected root after it", roots)
	}
}
