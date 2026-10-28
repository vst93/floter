package homepath

import (
	"os"
	"path/filepath"
	"testing"
)

// The rule is the reference's resolve_path, spelled out as a table. Every row
// is an answer a user can see: the action bar's path row, the open-path action,
// a dropped file, and a connect link's manifest value all resolve here.
func TestExpandIsTheReferencesResolvePath(t *testing.T) {
	home := t.TempDir()
	abs := filepath.Join(t.TempDir(), "x")
	// `~\x` is two answers by design, the reference's own: a separator on
	// Windows, a filename character on POSIX.
	backslash := filepath.Join(home, `~\x`)
	if os.IsPathSeparator('\\') {
		backslash = filepath.Join(home, "x")
	}
	cases := []struct {
		in   string
		want string
		why  string
	}{
		{"~", home, "the whole path is home"},
		{"~/", home, "a trailing separator is still home"},
		{"~//notes", filepath.Join(home, "notes"), "separators come off before the join, never go into it"},
		{"~/notes/a.md", filepath.Join(home, "notes", "a.md"), "the ordinary case"},
		{"~alice", filepath.Join(home, "~alice"), "a directory literally called ~alice, not another user"},
		{"~alice/x", filepath.Join(home, "~alice", "x"), "and its children"},
		{`~\x`, backslash, "the separator test is the platform's own (reference: std::path::is_separator)"},
		{"notes/a.md", "notes/a.md", "relative stays relative: the caller picks the base"},
		{abs, abs, "absolute is left alone"},
	}
	for _, c := range cases {
		got, ok := Expand(c.in, home)
		if !ok {
			t.Errorf("Expand(%q) refused a path it could resolve: %s", c.in, c.why)
			continue
		}
		if got != c.want {
			t.Errorf("Expand(%q) = %q, want %q: %s", c.in, got, c.want, c.why)
		}
	}
}

// A home that cannot be named cannot produce the path the user asked for:
// joining against "" would silently mean the working directory instead.
func TestExpandRefusesWhenHomeIsUnknown(t *testing.T) {
	for _, in := range []string{"~", "~/notes", "~alice"} {
		if got, ok := Expand(in, ""); ok {
			t.Errorf("Expand(%q, \"\") = %q, true, want a refusal", in, got)
		}
	}
	// A path that needs no home is not refused by one being missing.
	if got, ok := Expand("/etc/hosts", ""); !ok || got != "/etc/hosts" {
		t.Errorf(`Expand("/etc/hosts", "") = %q, %v, want the path unchanged`, got, ok)
	}
	if got, ok := Expand("notes/a.md", ""); !ok || got != "notes/a.md" {
		t.Errorf(`Expand("notes/a.md", "") = %q, %v, want the path unchanged`, got, ok)
	}
}

// Resolve is the typed-path answer: what is still relative after the tilde is
// joined to home. This is the half a dropped path does not get (it joins to the
// drop's own directory), so the two functions are tested apart.
func TestResolveJoinsWhatIsStillRelativeToHome(t *testing.T) {
	home := t.TempDir()
	abs := filepath.Join(t.TempDir(), "x")
	cases := []struct{ in, want string }{
		{"~", home},
		{"~/notes", filepath.Join(home, "notes")},
		{"~alice/x", filepath.Join(home, "~alice", "x")},
		{"notes/a.md", filepath.Join(home, "notes", "a.md")},
		{abs, abs},
	}
	for _, c := range cases {
		got, ok := Resolve(c.in, home)
		if !ok {
			t.Errorf("Resolve(%q) refused a path it could resolve", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("Resolve(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got, ok := Resolve("notes/a.md", ""); ok {
		t.Errorf(`Resolve("notes/a.md", "") = %q, true, want a refusal: "" would mean the working directory`, got)
	}
	if got, ok := Resolve(abs, ""); !ok || got != abs {
		t.Errorf(`Resolve(%q, "") = %q, %v, want the path unchanged`, abs, got, ok)
	}
}
