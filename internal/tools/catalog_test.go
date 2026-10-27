package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeExecutable(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range Catalog {
		if entry.ID == "" || entry.Name == "" {
			t.Errorf("entry without identity: %+v", entry)
		}
		if seen[entry.ID] {
			t.Errorf("duplicate id %q", entry.ID)
		}
		seen[entry.ID] = true
		for _, platform := range []string{PlatformMacOS, PlatformLinux, PlatformWindows} {
			if len(entry.Probes.For(platform)) == 0 {
				t.Errorf("%s has no %s probe", entry.ID, platform)
			}
		}
		// Every recipe names a manager the table knows and a package.
		for platform, recipes := range entry.Recipes {
			if platform != PlatformMacOS && platform != PlatformLinux && platform != PlatformWindows {
				t.Errorf("%s has recipes for %q", entry.ID, platform)
			}
			for _, recipe := range recipes {
				if knownManager(recipe.Manager) == nil {
					t.Errorf("%s uses unknown manager %q", entry.ID, recipe.Manager)
				}
				if recipe.Package == "" {
					t.Errorf("%s has an empty package for %s", entry.ID, recipe.Manager)
				}
				if InstallCommand(recipe.Manager, recipe.Package) == "" {
					t.Errorf("%s: no command for %s", entry.ID, recipe.Manager)
				}
			}
		}
	}
}

func knownManager(id string) *Manager {
	for i := range Managers {
		if Managers[i].ID == id {
			return &Managers[i]
		}
	}
	return nil
}

func TestInstallCommands(t *testing.T) {
	cases := map[string]string{
		"brew":   "brew install jq",
		"winget": "winget install --id jq -e",
		"pacman": "sudo pacman -S jq",
		"apt":    "sudo apt install jq",
		"dnf":    "sudo dnf install jq",
		"npm":    "npm install -g tldr",
		"cargo":  "cargo install ripgrep",
		"pipx":   "pipx install httpie",
	}
	for manager, want := range cases {
		pkg := "jq"
		switch manager {
		case "npm":
			pkg = "tldr"
		case "cargo":
			pkg = "ripgrep"
		case "pipx":
			pkg = "httpie"
		}
		if got := InstallCommand(manager, pkg); got != want {
			t.Errorf("InstallCommand(%s) = %q, want %q", manager, got, want)
		}
	}
	if got := InstallCommand("nonsense", "x"); got != "" {
		t.Errorf("an unknown manager = %q", got)
	}
}

func TestLookDetectsToolsAndManagers(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "rg")
	writeExecutable(t, dir, "fdfind")
	writeExecutable(t, dir, "brew")
	dirs := []string{dir}

	states := Look(dirs, PlatformMacOS)
	byID := map[string]State{}
	for _, state := range states {
		byID[state.ID] = state
	}
	if !byID["ripgrep"].Installed {
		t.Error("ripgrep was not detected")
	}
	// P10-o 重定基线：旧结构（一张扁平探测表）→ 新结构（按平台分表）。`fdfind`
	// is Debian's spelling of `fd`, so it counts on Linux and nowhere else — the
	// old build's `probe_candidates` was per platform for exactly this reason.
	if byID["fd"].Installed {
		t.Error("fdfind was detected as fd on macos, where that name does not exist")
	}
	// A Windows host emulates every file as 0666, so the Unix rule cannot be
	// exercised there; the negative half above still runs everywhere.
	if runtime.GOOS != "windows" {
		linux := map[string]State{}
		for _, state := range Look(dirs, PlatformLinux) {
			linux[state.ID] = state
		}
		if !linux["fd"].Installed {
			t.Error("fdfind was not detected as fd on linux")
		}
	}
	if byID["jq"].Installed {
		t.Error("jq was detected without being there")
	}
	// The chosen recipe is the detected manager's.
	if byID["jq"].Manager != "brew" || byID["jq"].Command != "brew install jq" {
		t.Errorf("jq recipe = %+v", byID["jq"])
	}
	// A tool that is not installed still gets a command, so the row can offer
	// it.
	if byID["lazygit"].Command == "" || byID["lazygit"].Manager != "brew" {
		t.Errorf("lazygit = %+v", byID["lazygit"])
	}
	// A full-screen tool says it needs a terminal.
	if !byID["lazygit"].NeedsTerminal || byID["jq"].NeedsTerminal {
		t.Error("the terminal hint is wrong")
	}

	// With no manager installed, the platform's first recipe is offered.
	empty := Look([]string{t.TempDir()}, PlatformLinux)
	for _, state := range empty {
		if state.Installed {
			t.Errorf("%s was detected in an empty path", state.ID)
		}
		if state.ID == "ripgrep" && state.Manager != "pacman" {
			t.Errorf("the fallback recipe = %+v", state)
		}
		if state.ID == "lazygit" && state.Manager != "pacman" {
			t.Errorf("lazygit's fallback = %+v", state)
		}
	}

	// Windows has no recipe for some tools: those get no command rather than a
	// guessed one.
	windows := Look(nil, PlatformWindows)
	for _, state := range windows {
		if state.ID == "eza" && state.Manager != "winget" {
			t.Errorf("eza on windows = %+v", state)
		}
	}
}

func TestChosenRecipePrefersADetectedManager(t *testing.T) {
	entry := Catalog[0] // flameshot: brew on macOS
	if recipe, ok := ChosenRecipe(entry, PlatformMacOS, nil); !ok || recipe.Manager != "brew" {
		t.Errorf("no manager detected = %+v", recipe)
	}
	// A detected manager that has no recipe for the tool falls through to the
	// table's first.
	if recipe, ok := ChosenRecipe(entry, PlatformMacOS, []string{"apt", "npm"}); !ok || recipe.Manager != "brew" {
		t.Errorf("a manager without a recipe = %+v", recipe)
	}
	unknown := Entry{ID: "x", Recipes: map[string][]Recipe{PlatformMacOS: {}}}
	if _, ok := ChosenRecipe(unknown, PlatformMacOS, nil); ok {
		t.Error("an empty table produced a recipe")
	}
}

func TestMatchesAndProbe(t *testing.T) {
	var ripgrep State
	for _, state := range Catalog {
		if state.ID == "ripgrep" {
			ripgrep = State{Entry: state}
		}
	}
	for _, query := range []string{"ripgrep", "rg", "搜索", "sousuo", "grep"} {
		if !ripgrep.Matches([]string{query}) {
			t.Errorf("%q did not match ripgrep", query)
		}
	}
	if ripgrep.Matches([]string{"nonsense"}) {
		t.Error("a nonsense query matched")
	}
	if ripgrep.Matches(nil) {
		t.Error("an empty query matched")
	}

	// Probe prefers the first name that resolves.
	dir := t.TempDir()
	writeExecutable(t, dir, "batcat")
	if got := Probe(ripgrep.Entry, []string{dir}, PlatformLinux); got != "" {
		t.Errorf("ripgrep probed %q", got)
	}
	var bat Entry
	for _, entry := range Catalog {
		if entry.ID == "bat" {
			bat = entry
		}
	}
	if got := Probe(bat, []string{dir}, PlatformLinux); got != "batcat" {
		t.Errorf("bat probed %q", got)
	}
	// A probe prefers an earlier candidate name over a later one.
	writeExecutable(t, dir, "bat")
	if got := Probe(bat, []string{dir}, PlatformLinux); got != "bat" {
		t.Errorf("bat probed %q, want the first candidate", got)
	}
	if got := Probe(bat, nil, PlatformLinux); got != "" {
		t.Errorf("an empty path probed %q", got)
	}
}

func TestPlatformKey(t *testing.T) {
	got := Platform()
	switch runtime.GOOS {
	case "darwin":
		if got != PlatformMacOS {
			t.Errorf("Platform() = %q", got)
		}
	case "windows":
		if got != PlatformWindows {
			t.Errorf("Platform() = %q", got)
		}
	default:
		if got != PlatformLinux {
			t.Errorf("Platform() = %q", got)
		}
	}
}

// ManagerName is the manager's own proper noun, which the install row's source
// slot prints: "Homebrew", not the executable's "brew". An id with no manager
// answers empty rather than guessing one.
func TestManagerName(t *testing.T) {
	if got := ManagerName("brew"); got != "Homebrew" {
		t.Errorf("ManagerName(brew) = %q, want Homebrew", got)
	}
	if got := ManagerName("cargo"); got != "Cargo" {
		t.Errorf("ManagerName(cargo) = %q, want Cargo", got)
	}
	if got := ManagerName("nope"); got != "" {
		t.Errorf("ManagerName(nope) = %q, want empty", got)
	}
}
