package settings

import (
	"encoding/json"
	"testing"
)

func TestCommandSwitches(t *testing.T) {
	// Absence means off, and a command that was never switched on is not
	// summonable.
	switches := CommandSwitchesOf(Default())
	if switches.Enabled("io.github.vst93.v", "jv") {
		t.Error("a command with no entry read as enabled")
	}

	s, err := Parse([]byte(`{"plugin_command_switches": {"ext": {"run": true, "  ": true, "other": false}, "": {"x": true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	switches = CommandSwitchesOf(s)
	if !switches.Enabled("ext", "run") {
		t.Errorf("switches = %+v", switches)
	}
	if switches.Enabled("ext", "other") {
		t.Error("an explicit false read as enabled")
	}
	if _, ok := switches[""]; ok {
		t.Error("a blank extension id was kept")
	}
	if _, ok := switches["ext"]["  "]; ok {
		t.Error("a blank command id was kept")
	}
	if !switches.AnyEnabled("ext") || switches.AnyEnabled("nope") {
		t.Errorf("AnyEnabled = %v / %v", switches.AnyEnabled("ext"), switches.AnyEnabled("nope"))
	}

	// A write lands in the same map, keeps every other extension's entries,
	// and survives a round trip through the file.
	s.SetCommandSwitch("ext", "third", true)
	encoded, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Switches map[string]map[string]bool `json:"plugin_command_switches"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Switches["ext"]["run"] || !decoded.Switches["ext"]["third"] || decoded.Switches["ext"]["other"] {
		t.Errorf("encoded switches = %+v", decoded.Switches)
	}
	if got := CommandSwitchesOf(mustParse(t, encoded)); !got.Enabled("ext", "third") {
		t.Errorf("round trip = %+v", got)
	}
	// A blank id is a no-op, not a new entry.
	s.SetCommandSwitch("", "x", true)
	if got := CommandSwitchesOf(s); len(got) != 1 {
		t.Errorf("a blank extension id was written: %+v", got)
	}
}

func TestCommandAliasesResolve(t *testing.T) {
	// The first command in ascending name order locks an alias; a later
	// command whose alias collides is inert, and an empty alias is ignored.
	resolved := ResolveCommandAliases(CommandAliases{
		"git":     "gfm",
		"gitlab":  "gfm",
		"ripgrep": "rg",
		"fd":      "  ",
		"  ":      "x",
	})
	if len(resolved) != 2 {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved["git"] != "gfm" {
		t.Errorf("git alias = %q", resolved["git"])
	}
	if _, ok := resolved["gitlab"]; ok {
		t.Error("a colliding alias was kept")
	}
	if resolved["ripgrep"] != "rg" {
		t.Errorf("ripgrep alias = %q", resolved["ripgrep"])
	}

	// An alias that only differs in case from one already claimed is inert,
	// and the claim is resolved in ascending command-name order.
	caseOnly := ResolveCommandAliases(CommandAliases{"bat": "GFM", "git": "gfm"})
	if len(caseOnly) != 1 || caseOnly["bat"] != "GFM" {
		t.Errorf("case-only collision = %+v", caseOnly)
	}

	// The map is read from the settings file and written back; clearing the
	// alias removes the entry.
	s, err := Parse([]byte(`{"command_aliases": {"git": "gfm", "nope": 5}}`))
	if err != nil {
		t.Fatal(err)
	}
	aliases := CommandAliasesOf(s)
	if aliases["git"] != "gfm" {
		t.Errorf("aliases = %+v", aliases)
	}
	if _, ok := aliases["nope"]; ok {
		t.Error("a non-string alias was kept")
	}
	s.SetCommandAlias("ripgrep", "rg")
	s.SetCommandAlias("git", "  ")
	encoded, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Aliases map[string]any `json:"command_aliases"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Aliases["ripgrep"] != "rg" {
		t.Errorf("the new alias is missing: %+v", decoded.Aliases)
	}
	if _, ok := decoded.Aliases["git"]; ok {
		t.Errorf("a cleared alias survived: %+v", decoded.Aliases)
	}
}

func TestLauncherSwitchesAndLaunchCounts(t *testing.T) {
	// The shipped values.
	s := Default()
	if ShowCommandsInSearch(s) {
		t.Error("show_commands_in_search is on by default")
	}
	if !ShowRecentInLauncher(s) || !ShowMenubarIcon(s) {
		t.Error("the recent list or the tray is off by default")
	}
	if got := LastSettingsPage(s); got != "" {
		t.Errorf("last page = %q", got)
	}

	s.SetShowCommandsInSearch(true)
	s.SetShowRecentInLauncher(false)
	s.SetShowMenubarIcon(false)
	s.SetLastSettingsPage("plugins")
	parsed := mustParse(t, mustEncode(t, s))
	if !ShowCommandsInSearch(parsed) || ShowRecentInLauncher(parsed) || ShowMenubarIcon(parsed) {
		t.Errorf("round trip = %v / %v / %v",
			ShowCommandsInSearch(parsed), ShowRecentInLauncher(parsed), ShowMenubarIcon(parsed))
	}
	if got := LastSettingsPage(parsed); got != "plugins" {
		t.Errorf("last page = %q", got)
	}

	// Launch counts keep positive integers and drop the rest.
	counts := LaunchCountsOf(mustParse(t, []byte(`{"launch_counts": {"/a": 3, "/b": 0, "/c": "x"}}`)))
	if len(counts) != 1 || counts["/a"] != 3 {
		t.Errorf("counts = %+v", counts)
	}
	s.SetLaunchCounts(LaunchCounts{"/a": 4, "": 1, "/b": -2})
	if got := LaunchCountsOf(s); len(got) != 1 || got["/a"] != 4 {
		t.Errorf("counts after a write = %+v", got)
	}
}

func mustEncode(t *testing.T, s Settings) []byte {
	t.Helper()
	data, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
