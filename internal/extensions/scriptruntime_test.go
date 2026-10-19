//go:build !windows

package extensions

import (
	"context"
	"testing"

	"floter/internal/scriptlang"
)

// The runtime check answers for a language the way the editor's status line
// needs: which interpreter is here, and what a machine without one has to
// install. The PATH lookup is injected, so the test does not read this
// machine's.
func TestScriptRuntimeFor(t *testing.T) {
	original := lookTool
	defer func() { lookTool = original }()

	var asked []string
	lookTool = func(names ...string) (string, bool) {
		asked = append(asked, names[0])
		if names[0] == "python3" {
			return "/nonexistent/floter-test/python3", true
		}
		return "", false
	}

	check := ScriptRuntimeFor(context.Background(), "python")
	if !check.Available || check.Path != "/nonexistent/floter-test/python3" {
		t.Fatalf("check = %+v", check)
	}
	if len(check.Candidates) == 0 || check.Candidates[0] != "python3" {
		t.Errorf("candidates = %v", check.Candidates)
	}
	if check.Compiled {
		t.Error("python was marked compiled")
	}
	// The version comes from the interpreter itself. The injected path is not
	// a real program, so the probe answers nothing — which is what a toolchain
	// that does not print a version looks like: the check still succeeds.
	if asked[0] != "python3" {
		t.Errorf("asked for %v", asked)
	}
	if check.Version != "" {
		t.Errorf("a fake path reported version %q", check.Version)
	}

	// A language whose interpreter is not here: not found is an answer.
	check = ScriptRuntimeFor(context.Background(), "ruby")
	if check.Available || check.Path != "" {
		t.Errorf("ruby = %+v", check)
	}
	if len(check.Candidates) == 0 || check.Candidates[0] != "ruby" {
		t.Errorf("ruby candidates = %v", check.Candidates)
	}

	// A compiled language looks for its toolchain, not an interpreter.
	asked = nil
	check = ScriptRuntimeFor(context.Background(), "go")
	if len(asked) == 0 || asked[0] != "go" {
		t.Errorf("go asked for %v", asked)
	}
	if !check.Compiled {
		t.Error("go was not marked compiled")
	}
	if len(check.Candidates) == 0 || check.Candidates[0] != "go" {
		t.Errorf("go candidates = %v", check.Candidates)
	}
	// And an unknown language resolves to the first one, as the picker does.
	check = ScriptRuntimeFor(context.Background(), "cobol")
	if len(check.Candidates) == 0 || check.Candidates[0] != "node" {
		t.Errorf("an unknown language asked for %v", check.Candidates)
	}
}

// The interpreter names behind the check are the ones each platform's toolchain
// is installed as, and the compiled languages have none: their toolchain builds
// the source at install time.
func TestInterpreterNamesMatchTheVocabulary(t *testing.T) {
	for _, language := range scriptlang.Languages {
		names := InterpreterNames(language.ID)
		if language.Compiled {
			if len(names) != 0 {
				t.Errorf("%s is compiled but names interpreters: %v", language.ID, names)
			}
			continue
		}
		if len(names) == 0 {
			t.Errorf("%s names no interpreter", language.ID)
			continue
		}
		if names[0] != language.Toolchain {
			t.Errorf("%s looks for %q, the vocabulary says %q", language.ID, names[0], language.Toolchain)
		}
	}
	// The version probe tolerates the shapes real tools print.
	if got := SemverFromVersionOutput("Python 3.12.1"); got != "3.12.1" {
		t.Errorf("python version = %q", got)
	}
	if got := SemverFromVersionOutput("go version go1.22.3 linux/amd64"); got != "1.22.3" {
		t.Errorf("go version = %q", got)
	}
}
