package scriptlang

import (
	"strings"
	"testing"
)

// The vocabulary is the eight languages, each with the extension the provider
// file is written with and the toolchain that has to be on PATH.
func TestLanguagesAreComplete(t *testing.T) {
	if len(Languages) != 8 {
		t.Fatalf("languages = %d", len(Languages))
	}
	want := map[string]struct{ extension, toolchain string }{
		"js":         {"js", "node"},
		"shell":      {"sh", "sh"},
		"powershell": {"ps1", "pwsh"},
		"python":     {"py", "python3"},
		"ruby":       {"rb", "ruby"},
		"php":        {"php", "php"},
		"go":         {"go", "go"},
		"rust":       {"rs", "rustc"},
	}
	for _, language := range Languages {
		expect, ok := want[language.ID]
		if !ok {
			t.Errorf("unexpected language %q", language.ID)
			continue
		}
		if language.Extension != expect.extension {
			t.Errorf("%s extension = %q, want %q", language.ID, language.Extension, expect.extension)
		}
		if language.Toolchain != expect.toolchain {
			t.Errorf("%s toolchain = %q, want %q", language.ID, language.Toolchain, expect.toolchain)
		}
		if language.Label == "" || language.Template == "" {
			t.Errorf("%s has no label or template", language.ID)
		}
		delete(want, language.ID)
	}
	for id := range want {
		t.Errorf("%s is missing", id)
	}
	// Only the compiled two build their source; the rest are interpreted.
	for _, language := range Languages {
		compiled := language.ID == "go" || language.ID == "rust"
		if language.Compiled != compiled {
			t.Errorf("%s compiled = %v", language.ID, language.Compiled)
		}
	}
}

// A template says what a provider is and how its parameters arrive, in both
// languages' comments: it is the first thing a user reads.
func TestTemplatesExplainThemselves(t *testing.T) {
	for _, language := range Languages {
		if !strings.Contains(language.Template, "argv") {
			t.Errorf("%s's template does not mention argv", language.ID)
		}
		if !strings.Contains(language.Template, "参数") {
			t.Errorf("%s's template is not bilingual", language.ID)
		}
	}
}

// A language is looked up by its id, by the interpreter a legacy manifest may
// have named instead, and — for anything else — falls back to the first
// language, so the picker is never empty.
func TestLookupFallsBack(t *testing.T) {
	if got := Lookup("python").ID; got != "python" {
		t.Errorf("Lookup(python) = %q", got)
	}
	for _, alias := range []string{"python3", "PYTHON3", " python3 "} {
		if got := Lookup(alias).ID; got != "python" {
			t.Errorf("Lookup(%q) = %q", alias, got)
		}
	}
	for alias, want := range map[string]string{
		"node": "js", "javascript": "js", "bash": "shell", "sh": "shell",
		"zsh": "shell", "pwsh": "powershell",
	} {
		if got := Lookup(alias).ID; got != want {
			t.Errorf("Lookup(%q) = %q, want %q", alias, got, want)
		}
	}
	if got := Lookup("cobol").ID; got != Languages[0].ID {
		t.Errorf("an unknown language resolved to %q", got)
	}
	if got := Lookup("").ID; got != Languages[0].ID {
		t.Errorf("an empty language resolved to %q", got)
	}
	// The helpers go through the same lookup.
	if Extension("python") != "py" || Template("python") == "" {
		t.Errorf("Extension/Template disagree with Lookup")
	}
	if len(Templates()) != len(Languages) {
		t.Errorf("Templates = %d", len(Templates()))
	}
}

// The status line makes one of three claims: we have not looked, it is there,
// or it is not. An absent toolchain names what to install.
func TestStatusOf(t *testing.T) {
	// Not asked yet: the check is zero, so nothing claims it is missing.
	status := StatusOf("python", Check{})
	if status.State != Unknown {
		t.Errorf("a zero check = %v", status.State)
	}
	if status.Name != "python3" {
		t.Errorf("name = %q", status.Name)
	}
	if len(status.Names) != 0 {
		t.Errorf("a not-asked check named what to install: %v", status.Names)
	}

	// Found: the interpreter's own file name and the version it reported.
	status = StatusOf("python", Check{
		Available: true,
		Path:      "/usr/bin/python3",
		Version:   "3.12.1",
	})
	if status.State != Available {
		t.Fatalf("state = %v", status.State)
	}
	if status.Name != "python3" || status.Version != "3.12.1" || status.Path != "/usr/bin/python3" {
		t.Errorf("status = %+v", status)
	}
	// A Windows-style path is named by its last segment too.
	if got := StatusOf("js", Check{Available: true, Path: `C:\Program Files\nodejs\node.exe`}); got.Name != "node.exe" {
		t.Errorf("windows name = %q", got.Name)
	}

	// The candidates the check found are what a missing toolchain says.
	status = StatusOf("python", Check{Candidates: []string{"python3", "python"}})
	if status.State != Missing || len(status.Names) != 2 || status.Names[0] != "python3" {
		t.Errorf("status = %+v", status)
	}

	// A compiled language's status says so: the toolchain builds the source.
	status = StatusOf("go", Check{Available: true, Path: "/usr/local/go/bin/go"})
	if !status.Compiled {
		t.Errorf("go is not marked compiled: %+v", status)
	}
	status = StatusOf("go", Check{})
	if status.State != Unknown || !status.Compiled || status.Name != "go" {
		t.Errorf("go not asked = %+v", status)
	}
	// An interpreted language is never marked compiled.
	if StatusOf("shell", Check{Available: true, Path: "/bin/sh"}).Compiled {
		t.Error("shell was marked compiled")
	}
}

// Asked separates "we looked" from "compiled is a fact about the language":
// a check carrying only the compiled flag has no observation behind it, so it
// must still read as not-asked.
func TestCheckAsked(t *testing.T) {
	for _, tc := range []struct {
		name  string
		check Check
		want  bool
	}{
		{"zero", Check{}, false},
		{"only compiled", Check{Compiled: true}, false},
		{"availability", Check{Available: true}, true},
		{"path", Check{Path: "/usr/bin/node"}, true},
		{"version", Check{Version: "3.12.1"}, true},
		{"candidates", Check{Candidates: []string{"node"}}, true},
		{"missing", Check{Candidates: []string{"python3"}, Compiled: true}, true},
	} {
		if got := tc.check.Asked(); got != tc.want {
			t.Errorf("%s: Asked() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
