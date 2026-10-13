package extensions

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A descriptor's workingDirectory names a behaviour, not a path: "current" is
// the launch's own cwd, "home" the user's home, and "inherit" nothing at all.
func TestWorkingDirectory(t *testing.T) {
	cases := []struct {
		mode      string
		launchDir string
		home      string
		want      string
	}{
		{"current", "/tool/data", "/home/u", "/tool/data"},
		{"", "/tool/data", "/home/u", "/tool/data"},
		{"CURRENT", "/tool/data", "/home/u", "/tool/data"},
		{"  current  ", "/tool/data", "/home/u", "/tool/data"},
		{"home", "/tool/data", "/home/u", "/home/u"},
		{"Home", "/tool/data", "/home/u", "/home/u"},
		{"inherit", "/tool/data", "/home/u", ""},
		{"INHERIT", "/tool/data", "/home/u", ""},
		// An unknown word is the schema default, not an error.
		{"somewhere-else", "/tool/data", "/home/u", "/tool/data"},
		// No launch cwd resolved: current still names nothing.
		{"current", "", "/home/u", ""},
		// A home that could not be found is the old host's empty answer.
		{"home", "/tool/data", "", ""},
	}
	for _, c := range cases {
		got := WorkingDirectory(c.mode, c.launchDir, c.home)
		if got != c.want {
			t.Errorf("WorkingDirectory(%q, %q, %q) = %q, want %q", c.mode, c.launchDir, c.home, got, c.want)
		}
	}
}

// The declared color names TERM and COLORTERM; an absent block is the shipped
// 256-color pair (launch.rs `terminal_environment`).
func TestTerminalEnvironment(t *testing.T) {
	cases := []struct{ color, term, colorterm string }{
		{"", "floter-256color", "truecolor"},
		{"truecolor", "floter-256color", "truecolor"},
		{"256", "floter-256color", ""},
		{"8", "xterm", ""},
		{"none", "dumb", ""},
		{"NONE", "dumb", ""},
		{"nonsense", "floter-256color", "truecolor"},
	}
	for _, c := range cases {
		env := terminalEnvironment(c.color)
		found := map[string]string{}
		for _, pair := range env {
			key, value, _ := strings.Cut(pair, "=")
			found[key] = value
		}
		if found["TERM"] != c.term || found["COLORTERM"] != c.colorterm || found["TERM_PROGRAM"] != "floter" {
			t.Errorf("color %q: env = %v, want TERM=%q COLORTERM=%q TERM_PROGRAM=floter",
				c.color, found, c.term, c.colorterm)
		}
	}
}

// The registry resolves a provider descriptor's workingDirectory through the
// launch's own cwd: the two behaviours the plugin used to ignore (home) or
// hand to the OS as a literal path (current, inherit) now reach the process as
// real answers.
func TestDescriptorWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, "dev.floter.cwd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.cwd", "name": "Cwd",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable"},
	  "lifecycle": {"launch": {"cwdPolicy": "toolData", "terminal": {"color": "none"}}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte("#!/bin/sh\ncat <<'JSONEOF'\n"+
		`{"protocolVersion":"1.0","provider":{"id":"dev.floter.cwd","name":"Cwd","version":"1.0.0"},
		  "commands":[
		    {"id":"here","name":"Here","execution":{"program":"self","workingDirectory":"current","mode":"pty"}},
		    {"id":"dotted","name":"Dotted","execution":{"program":"self","mode":"pty"}},
		    {"id":"somewhere","name":"Somewhere","execution":{"program":"self","workingDirectory":"home","mode":"pty"}},
		    {"id":"anywhere","name":"Anywhere","execution":{"program":"self","workingDirectory":"inherit","mode":"pty"}}
		  ]}`+
		"\nJSONEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.cwd": {"id": "dev.floter.cwd", "name": "Cwd",
	    "state": "enabled", "enabled": true, "manifestPath": "`+filepath.Join(dir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store := OpenStore(paths)
	store.Refresh(context.Background())
	entries := store.CommandEntries()
	if len(entries) != 4 {
		t.Fatalf("entries = %+v", entries)
	}
	toolData := filepath.Join(paths.Data, "dev.floter.cwd")
	home, _ := os.UserHomeDir()
	want := map[string]string{
		"here":      toolData, // "current" is the launch's own cwd
		"dotted":    toolData, // the schema's default is "current"
		"somewhere": home,     // "home" is the user's home
		"anywhere":  "",       // "inherit" leaves the host's cwd alone
	}
	for _, entry := range entries {
		expected, ok := want[entry.Command.ID]
		if !ok {
			t.Fatalf("unexpected command %q", entry.Command.ID)
		}
		if entry.Dir != expected {
			t.Errorf("%s dir = %q, want %q", entry.Command.ID, entry.Dir, expected)
		}
	}
	// The declared color travels with the launch.
	found := map[string]string{}
	for _, pair := range entries[0].Env {
		key, value, _ := strings.Cut(pair, "=")
		found[key] = value
	}
	if found["TERM"] != "dumb" || found["COLORTERM"] != "" {
		t.Errorf("env = %v", found)
	}
}
