package termsession

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P13-a: the host subcommand's pins.
//
// The host is this binary called back, so the two things that can go wrong
// silently are the ones pinned here: the app must not treat a host's
// arguments as its own, and a bare session must start a **login** shell
// (`-zsh`, not `zsh`) — the plugin's own rule, and the reason a user's
// aliases and prompt are there when they come back.

// TestTheHostSubcommandIsNotTheApps pins the split: the subcommand is the
// first word and it is taken before anything else looks at the arguments.
func TestTheHostSubcommandIsNotTheApps(t *testing.T) {
	if _, ok := IsCommand(nil); ok {
		t.Error("no arguments were read as the host subcommand")
	}
	if _, ok := IsCommand([]string{"floter://register?cmd=rg"}); ok {
		t.Error("a deep link was read as the host subcommand")
	}
	rest, ok := IsCommand([]string{CommandName, "-program", "/bin/sh"})
	if !ok {
		t.Fatal("the host subcommand was not recognised")
	}
	if len(rest) != 2 || rest[0] != "-program" {
		t.Errorf("the host's own arguments = %q, want the ones after the name", rest)
	}
	// The name is a word of its own, not a prefix: `__termhostile` is the
	// app's.
	if _, ok := IsCommand([]string{CommandName + "ile"}); ok {
		t.Error("a word starting with the subcommand's name was taken as it")
	}
}

// TestTheBareSessionRunsALoginShell pins the shell a bare session starts:
// $SHELL when set, and argv[0] with the leading dash that makes it a login
// shell.
func TestTheBareSessionRunsALoginShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	path, args, err := UserShell()
	if err != nil {
		t.Fatal(err)
	}
	if path != "/bin/bash" {
		t.Errorf("shell = %q, want $SHELL", path)
	}
	if len(args) != 1 || args[0] != "-bash" {
		t.Errorf("argv = %q, want the login shell's own name", args)
	}
	// An empty $SHELL falls back to the platform's, and never to nothing.
	t.Setenv("SHELL", "")
	path, args, err = UserShell()
	if err != nil {
		t.Fatalf("no shell without $SHELL: %v", err)
	}
	if !strings.HasPrefix(args[0], "-") {
		t.Errorf("argv = %q, want a login shell", args)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Errorf("the fallback shell %q is not a program", path)
	}
}

// TestTheEnvironmentCarriesTheTerminal pins the three variables a program
// needs to know it is in a terminal: TERM, COLORTERM, and a UTF-8 locale when
// the app was started without one (a macOS app from the Finder has none).
func TestTheEnvironmentCarriesTheTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	env := Environment([]string{"FLOTER_TEST=1"})
	for _, want := range []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=en_US.UTF-8", "FLOTER_TEST=1"} {
		found := false
		for _, kv := range env {
			if kv == want {
				found = true
			}
		}
		if !found {
			t.Errorf("environment has no %q", want)
		}
	}
	// TERM is replaced, not appended: two TERMs would be a program reading
	// whichever came first.
	count := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, "TERM=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%d TERM entries, want one", count)
	}
}

// TestTheHostCommandRefusesAnUnknownFlag pins the command line's boundary: a
// host started with a flag it does not know must fail loudly rather than
// silently run a shell nobody asked for.
func TestTheHostCommandRefusesAnUnknownFlag(t *testing.T) {
	if code := Command([]string{"-nonsense"}); code == 0 {
		t.Error("the host accepted an unknown flag")
	}
	// A socket path is a directory's worth of meaning: the command must not
	// build one out of a relative name it was handed.
	if DefaultSocket() == "" {
		t.Error("the default socket has no name")
	}
	if !strings.Contains(DefaultSocket(), filepath.Join("floter", "terminal")) {
		t.Errorf("default socket = %q, want it under the terminal's own directory", DefaultSocket())
	}
}
