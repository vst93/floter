package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/settings"
)

// The paste rule, one table: the two halves are independent (safe paste is
// about the trailing break, the separator rule is unconditional), and the
// report is what decides whether the framework's own paste path is left alone.
func TestPasteText(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		safe    bool
		want    string
		changed bool
	}{
		{"clean text is left alone", "git status", true, "git status", false},
		{"safe paste takes one break", "git status\n", true, "git status", true},
		{"safe paste takes a CRLF", "git status\r\n", true, "git status", true},
		{"without safe paste the break stays", "git status\n", false, "git status\n", false},
		{"interior breaks stay", "echo a\necho b\n", true, "echo a\necho b", true},
		{"a separator becomes a space", "go\u3000version", false, "go version", true},
		{"a separator becomes a space with safe paste", "go\u3000version\n", true, "go version", true},
		{"tabs and newlines are not separators", "a\tb\nc", false, "a\tb\nc", false},
		{"empty stays empty", "", true, "", false},
		{"a lone break is just a break", "\n", false, "\n", false},
	}
	for _, tc := range cases {
		got, changed := pasteText(tc.text, tc.safe)
		if got != tc.want || changed != tc.changed {
			t.Errorf("%s: pasteText(%q, %v) = %q, %v; want %q, %v",
				tc.name, tc.text, tc.safe, got, changed, tc.want, tc.changed)
		}
	}
}

// The command line a bare session is handed: the boundary's normalization and
// the reference's trim. An empty result is not a command, and a line that is
// already clean is handed over untouched.
func TestShellCommandLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"git status", "git status"},
		{"  git status  ", "git status"},
		{"go\u3000version", "go version"},
		{"npm\u00a0install\u2009left-pad", "npm install left-pad"},
		{"go\u3000version\r\n", "go version"},
		{"\u3000", ""},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := shellCommandLine(tc.in); got != tc.want {
			t.Errorf("shellCommandLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The paste hook is installed on every session, and it is the app's own rule
// that answers: text that needs rewriting is taken (the app sends the rewritten
// bytes through the terminal's own Paste, so bracketed paste stays the plugin's
// business), and a paste that is already clean is left to the framework.
func TestEverySessionCarriesThePasteBoundary(t *testing.T) {
	store := settings.NewStore(settings.Default())
	var opts terminal.Options
	created := 0
	a := New(store, Actions{}, func(o terminal.Options) (*terminal.Terminal, error) {
		opts = o
		created++
		return &terminal.Terminal{}, nil
	})
	if _, err := a.startSession("", nil); err != nil {
		t.Fatalf("startSession: %v", err)
	}
	if created != 1 {
		t.Fatalf("the terminal was created %d times", created)
	}
	if opts.OnPaste == nil {
		t.Fatal("the session carries no paste hook: safe paste and the separator rule are both dead")
	}

	// Safe paste off: only the separator rule applies.
	if !opts.OnPaste("go\u3000version") {
		t.Error("a paste carrying a Unicode separator was not taken")
	}
	if opts.OnPaste("go version") {
		t.Error("a clean paste must be left to the framework")
	}
	if opts.OnPaste("go version\n") {
		t.Error("with safe paste off a trailing break must stay")
	}

	// Safe paste on: the trailing break goes too. The setting is read per
	// paste, so the drawer's switch reaches the session on screen.
	if err := store.Update(func(s *settings.Settings) { s.TerminalPasteSafe = true }); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !opts.OnPaste("go version\n") {
		t.Error("safe paste did not take the trailing break")
	}
	if opts.OnPaste("go version") {
		t.Error("safe paste must still leave a clean paste alone")
	}
}

// Safe paste and the separator rule share one boundary: a copied command that
// carries both a separator and the newline that ran it is one rewrite, not two.
func TestSafePasteAndSeparatorsAreOneBoundary(t *testing.T) {
	got, changed := pasteText("go\u3000version\n", true)
	if got != "go version" || !changed {
		t.Errorf("pasteText = %q, %v", got, changed)
	}
}
