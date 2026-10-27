package tools

import (
	"strings"
	"testing"
)

// The three tables in `catalog.go` are the criterion for the rows the launcher
// builds, so each one has to hold on its own terms: the probe table per
// platform, the recipe table per manager platform, and the catalog entry per
// its search and launch vocabulary.
//
// These are the old build's `tool_catalog.rs` invariants
// (`r67_every_tool_has_a_recipe_on_every_platform`,
// `r67_every_recipe_names_a_manager_valid_on_that_platform`,
// `r67_tool_detection_uses_the_platform_candidates`,
// `r68_keywords_are_two_to_five_unique_and_non_empty`,
// `r69_only_gui_and_tui_tools_carry_a_launch_hint`,
// `r70_needs_terminal_separates_gui_from_tui`) — the data already satisfied
// them, nothing was checking.

// allPlatforms is the vocabulary every per-platform table is addressed by, in
// the order the tables are read.
func allPlatforms() []string {
	return []string{PlatformMacOS, PlatformLinux, PlatformWindows}
}

func entryByID(t *testing.T, id string) Entry {
	t.Helper()
	for _, entry := range Catalog {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("no catalog entry %q", id)
	return Entry{}
}

// TestEveryToolHasARecipeOnEveryPlatform: a tool with no recipe for this
// platform renders an install row with an empty command, which is a row that
// cannot do anything. Every entry carries at least one recipe on all three.
func TestEveryToolHasARecipeOnEveryPlatform(t *testing.T) {
	for _, entry := range Catalog {
		for _, platform := range allPlatforms() {
			if len(entry.Recipes[platform]) == 0 {
				t.Errorf("%s has no %s recipe", entry.ID, platform)
			}
		}
	}
}

// TestEveryRecipeNamesAManagerValidOnThatPlatform pins the recipe table against
// the managers' own declared platforms: `winget` is a Windows manager, so a
// Linux recipe naming it would render `winget install …` on a machine that
// cannot run it. The mapping lives in one place (`Manager.Platforms`) and this
// is the only reader of it.
func TestEveryRecipeNamesAManagerValidOnThatPlatform(t *testing.T) {
	for _, entry := range Catalog {
		for _, platform := range allPlatforms() {
			for _, recipe := range entry.Recipes[platform] {
				platforms := ManagerPlatforms(recipe.Manager)
				if platforms == nil {
					t.Errorf("%s names an unknown manager %q", entry.ID, recipe.Manager)
					continue
				}
				valid := false
				for _, declared := range platforms {
					if declared == platform {
						valid = true
					}
				}
				if !valid {
					t.Errorf("%s uses %s on %s, where it is not available (it declares %v)",
						entry.ID, recipe.Manager, platform, platforms)
				}
			}
		}
	}
	// The accessor itself: an id nobody declared has no platforms, and the
	// three native managers are single-platform.
	if got := ManagerPlatforms("nonsense"); got != nil {
		t.Errorf("ManagerPlatforms(nonsense) = %v, want nil", got)
	}
	for id, want := range map[string]string{
		"winget": PlatformWindows,
		"pacman": PlatformLinux,
		"apt":    PlatformLinux,
		"dnf":    PlatformLinux,
	} {
		got := ManagerPlatforms(id)
		if len(got) != 1 || got[0] != want {
			t.Errorf("ManagerPlatforms(%s) = %v, want [%s]", id, got, want)
		}
	}
	// Homebrew is the macOS answer and also a Linux one (Linuxbrew); the three
	// language managers are everywhere.
	if got := ManagerPlatforms("brew"); len(got) != 2 {
		t.Errorf("ManagerPlatforms(brew) = %v, want two platforms", got)
	}
	for _, id := range []string{"npm", "cargo", "pipx"} {
		if got := ManagerPlatforms(id); len(got) != 3 {
			t.Errorf("ManagerPlatforms(%s) = %v, want three platforms", id, got)
		}
	}
}

// TestTheProbeTableIsScopedToItsPlatform is the behaviour the flat table used to
// get wrong: `fdfind` and `batcat` are Debian's spellings of `fd` and `bat`, so
// a file with one of those names means the tool on Linux and means nothing on
// macOS or Windows.
func TestTheProbeTableIsScopedToItsPlatform(t *testing.T) {
	fd := entryByID(t, "fd")
	bat := entryByID(t, "bat")
	for _, testCase := range []struct {
		entry    Entry
		name     string
		platform string
		want     bool
	}{
		{fd, "fdfind", PlatformLinux, true},
		{fd, "fdfind", PlatformMacOS, false},
		{fd, "fdfind", PlatformWindows, false},
		{fd, "fd", PlatformMacOS, true},
		{bat, "batcat", PlatformLinux, true},
		{bat, "batcat", PlatformMacOS, false},
		{bat, "bat", PlatformWindows, true},
	} {
		got := false
		for _, name := range testCase.entry.Probes.For(testCase.platform) {
			if name == testCase.name {
				got = true
			}
		}
		if got != testCase.want {
			t.Errorf("%s probes %q on %s: got %v, want %v",
				testCase.entry.ID, testCase.name, testCase.platform, got, testCase.want)
		}
	}

	// The consequence, not just the table: the probe itself agrees.
	dir := t.TempDir()
	writeExecutable(t, dir, "fdfind")
	if Probe(fd, []string{dir}, PlatformMacOS) != "" {
		t.Error("a macos probe resolved fdfind")
	}
	if got := Probe(fd, []string{dir}, PlatformLinux); got != "fdfind" {
		t.Errorf("a linux probe resolved %q, want fdfind", got)
	}

	// An empty key is the host, the same rule `candidateNames` has: the answer
	// is a real platform's list, never an empty one.
	if got := fd.Probes.For(""); len(got) == 0 {
		t.Error("the empty platform key gave no probe names")
	}
	if got := fd.Probes.For("beos"); len(got) == 0 {
		t.Error("an unknown platform key gave no probe names")
	}
	// Every entry has names on all three platforms, so `For` never has to
	// invent one.
	for _, entry := range Catalog {
		for _, platform := range allPlatforms() {
			if len(entry.Probes.For(platform)) == 0 {
				t.Errorf("%s has no %s probe", entry.ID, platform)
			}
		}
	}
	// The union is the search vocabulary: platform order, deduplicated.
	if got := fd.Probes.All(); len(got) != 2 || got[0] != "fd" || got[1] != "fdfind" {
		t.Errorf("fd.Probes.All() = %v, want [fd fdfind]", got)
	}
}

// TestTheSearchHaystackIsTheUnionOfEveryPlatformsProbes: a user who knows the
// tool as `batcat` or `rg` must find it whatever platform they are on, so the
// haystack is every name in the table. It is also the one string the launcher
// puts on a row and the one `Matches` reads.
func TestTheSearchHaystackIsTheUnionOfEveryPlatformsProbes(t *testing.T) {
	bat := State{Entry: entryByID(t, "bat")}
	for _, term := range []string{"bat", "batcat", "高亮"} {
		if !bat.Matches([]string{term}) {
			t.Errorf("bat does not match %q", term)
		}
	}
	if bat.Matches([]string{"fdfind"}) {
		t.Error("bat matched another tool's probe name")
	}
	ripgrep := State{Entry: entryByID(t, "ripgrep")}
	if !ripgrep.Matches([]string{"rg"}) {
		t.Error("ripgrep does not match its own binary name rg")
	}
	haystack := bat.SearchHaystack()
	for _, want := range []string{"bat", "batcat"} {
		if !strings.Contains(haystack, want) {
			t.Errorf("the haystack %q does not carry %q", haystack, want)
		}
	}
	if haystack != strings.ToLower(haystack) {
		t.Errorf("the haystack %q is not lowercase", haystack)
	}
	// The empty query matches nothing, so no row is offered for a blank box.
	if bat.Matches(nil) {
		t.Error("an empty query matched")
	}
}

// TestTheCatalogStaysACuratedDozen pins the entry table itself: the ids are
// unique, the keywords are two to five unique non-empty words, and the catalog
// is a curated dozen rather than a directory dump.
func TestTheCatalogStaysACuratedDozen(t *testing.T) {
	if len(Catalog) < 10 || len(Catalog) > 14 {
		t.Errorf("the catalog has %d entries, want 10..14", len(Catalog))
	}
	seen := map[string]bool{}
	for _, entry := range Catalog {
		if seen[entry.ID] {
			t.Errorf("duplicate id %q", entry.ID)
		}
		seen[entry.ID] = true
		if len(entry.Keywords) < 2 || len(entry.Keywords) > 5 {
			t.Errorf("%s has %d keywords, outside the 2..5 budget", entry.ID, len(entry.Keywords))
		}
		unique := map[string]bool{}
		for _, keyword := range entry.Keywords {
			if keyword == "" {
				t.Errorf("%s has a blank keyword", entry.ID)
			}
			if unique[keyword] {
				t.Errorf("%s repeats the keyword %q", entry.ID, keyword)
			}
			unique[keyword] = true
		}
	}
}

// TestLaunchHintsAreWellFormed: a launch hint is the argv the launcher starts,
// so it has to be non-empty, and only a tool that has one can claim it needs a
// terminal. Only the GUI (flameshot) and the TUI (lazygit) carry one: a pure CLI
// filter reads standard input and has nothing sensible to start detached.
func TestLaunchHintsAreWellFormed(t *testing.T) {
	launched := map[string]bool{}
	for _, entry := range Catalog {
		if len(entry.Launch) == 0 {
			if entry.NeedsTerminal {
				t.Errorf("%s needs a terminal but carries no launch argv", entry.ID)
			}
			continue
		}
		launched[entry.ID] = true
		for _, arg := range entry.Launch {
			if arg == "" {
				t.Errorf("%s has a blank argv element", entry.ID)
			}
		}
	}
	if len(launched) != 2 || !launched["flameshot"] || !launched["lazygit"] {
		t.Errorf("the launch hints are %v, want flameshot and lazygit", launched)
	}
	if got := entryByID(t, "flameshot").Launch; len(got) != 2 || got[0] != "flameshot" || got[1] != "gui" {
		t.Errorf("flameshot's argv = %v, want [flameshot gui]", got)
	}
	if entryByID(t, "flameshot").NeedsTerminal {
		t.Error("a GUI spawns detached, so it does not need a terminal")
	}
	if !entryByID(t, "lazygit").NeedsTerminal {
		t.Error("a full-screen TUI needs a real terminal")
	}
}
