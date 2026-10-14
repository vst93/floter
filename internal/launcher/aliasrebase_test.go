package launcher

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// An exact alias is a search surface only: the run reaches the command the
// alias stands for, so `gfm status` executes `git status`. The arguments ride
// along byte for byte, and the comparison is case-insensitive like the search.
func TestRebaseAliasArgvSwapsAnExactAliasForItsCommand(t *testing.T) {
	aliases := settings.CommandAliases{"git": "gfm"}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"alias with args", []string{"gfm", "status"}, []string{"git", "status"}},
		{"alias alone", []string{"gfm"}, []string{"git"}},
		{"alias case-insensitively", []string{"GFM", "-m", "x"}, []string{"git", "-m", "x"}},
		{"a prefix is its own program", []string{"gfmt", "status"}, []string{"gfmt", "status"}},
		{"a contains hit is its own program", []string{"xgfm", "status"}, []string{"xgfm", "status"}},
		{"no alias at all", []string{"ls", "-la"}, []string{"ls", "-la"}},
		{"empty argv", []string{}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RebaseAliasArgv(aliases, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("RebaseAliasArgv(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// The input argv is left untouched — the caller may still hold the parsed
// tokens — and no aliases means no map walk changes anything.
func TestRebaseAliasArgvDoesNotMutateTheInput(t *testing.T) {
	in := []string{"gfm", "status"}
	_ = RebaseAliasArgv(settings.CommandAliases{"git": "gfm"}, in)
	if in[0] != "gfm" {
		t.Errorf("the input argv was mutated: %v", in)
	}
	if got := RebaseAliasArgv(nil, in); !reflect.DeepEqual(got, in) {
		t.Errorf("with no aliases argv = %v, want %v", got, in)
	}
}

// The conflict policy is the settings one: two commands may claim the same
// alias, and the first in ascending name order owns it. A whitespace-only
// alias is dropped, so it can never rebase a program onto a blank name.
func TestRebaseAliasArgvResolvesConflictsAndDropsBlankAliases(t *testing.T) {
	aliases := settings.CommandAliases{
		"zeta":  "g",
		"alpha": "g",
		"beta":  "   ",
	}
	if got := RebaseAliasArgv(aliases, []string{"g", "x"}); !reflect.DeepEqual(got, []string{"alpha", "x"}) {
		t.Errorf("a contested alias rebased to %v, want [alpha x]", got)
	}
	if got := RebaseAliasArgv(aliases, []string{"g"}); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Errorf("a contested alias rebased to %v, want [alpha]", got)
	}
}

// The run path applies the rule: a typed `gfm status` reaches the terminal as
// `git status`, not as a program called `gfm` that does not exist. The tool
// row is what the alias names, and the line matches no catalog row, so Enter
// takes the no-result fallback the old build's `!launcherResults.length`
// covered.
func TestTheRunPathRebasesATypedAliasCommandLine(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	a.SetCommandAliases(settings.CommandAliases{"git": "gfm"})
	got := []string{}
	a.Actions.RunInTerminal = func(argv []string) {
		if len(got) == 0 {
			got = argv
		}
	}

	tt := render(t, a)
	tt.Type("gfm status")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !reflect.DeepEqual(got, []string{"git", "status"}) {
		t.Errorf("argv = %v, want [git status]", got)
	}
}

// The environment route shares the argv, so it is rebased too: a leading
// NAME=value assignment is dropped before the run, not mistaken for the
// command token, and its value is never rewritten.
func TestTheEnvironmentRunPathRebasesTheCommandToken(t *testing.T) {
	a := testApp()
	a.SetCommandAliases(settings.CommandAliases{"git": "gfm"})
	gotArgv, gotEnv := []string{}, map[string]string{}
	a.Actions.RunInTerminalWithEnv = func(argv []string, env []string) {
		if len(gotArgv) > 0 {
			return
		}
		gotArgv = argv
		for _, pair := range env {
			key, value, _ := strings.Cut(pair, "=")
			gotEnv[key] = value
		}
	}
	a.Actions.RunInTerminal = func([]string) { t.Error("the plain run was used") }

	tt := render(t, a)
	tt.Type(`FOO=gfm gfm status`)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !reflect.DeepEqual(gotArgv, []string{"git", "status"}) {
		t.Errorf("argv = %v, want [git status]", gotArgv)
	}
	if gotEnv["FOO"] != "gfm" {
		t.Errorf("the assignment's value was rewritten: %v", gotEnv)
	}
}

// A line with shell syntax is handed to the user's own shell as the reference
// typed it; the rebase owns the structured argv route alone, so `gfm | grep`
// is not silently rewritten behind the shell's back.
func TestShellSyntaxLinesAreNotRebased(t *testing.T) {
	a := testApp()
	a.SetCommandAliases(settings.CommandAliases{"git": "gfm"})
	ranLine := ""
	a.Actions.RunInShell = func(line string) {
		if ranLine != "" {
			return
		}
		ranLine = line
	}
	a.Actions.RunInTerminal = func([]string) { t.Error("a shell-syntax line went to argv") }

	tt := render(t, a)
	tt.Type("gfm | grep x")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ranLine != "gfm | grep x" {
		t.Errorf("the shell received %q", ranLine)
	}
}
