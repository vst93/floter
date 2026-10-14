package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

// writeExecutable writes a runnable fixture file, so a candidate enumeration
// finds it the way it would find a real program.
func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// Every directory that holds a matching name is a candidate, in search-path
// order — the reference's inventory returned all of them so a row could ask
// which one the user meant, rather than the first one the path happened to
// offer.
func TestLookToolsListsEveryMatchInSearchOrder(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeExecutable(t, filepath.Join(first, "tool"))
	writeExecutable(t, filepath.Join(second, "tool"))
	writeExecutable(t, filepath.Join(second, "other"))

	got := lookToolsIn([]string{first, second}, "tool")
	if len(got) != 2 {
		t.Fatalf("candidates = %+v, want two", got)
	}
	if got[0].Path != filepath.Join(first, "tool") || got[1].Path != filepath.Join(second, "tool") {
		t.Errorf("order = %q, %q; want search-path order", got[0].Path, got[1].Path)
	}
	if got[0].Name != "tool" || len(got[0].Sources) != 1 || got[0].Sources[0] != "path" {
		t.Errorf("candidate = %+v, want the name and the path source", got[0])
	}
}

// A symlink to a program already found is the same program, not a second
// candidate: the chooser must not ask about a distinction the filesystem does
// not make.
func TestLookToolsDoesNotCountASymlinkTwice(t *testing.T) {
	real, link := t.TempDir(), t.TempDir()
	writeExecutable(t, filepath.Join(real, "tool"))
	if err := os.Symlink(filepath.Join(real, "tool"), filepath.Join(link, "tool")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := lookToolsIn([]string{real, link}, "tool"); len(got) != 1 {
		t.Errorf("candidates = %+v, want the symlink folded into its target", got)
	}
}

// The recorded program is the reference's `preferred_locator`: when it is
// still there it is the whole answer, so a caller never asks about a binding
// the integration already has even when the search path offers others.
func TestSystemRuntimeCandidatesPreferTheRecordedProgram(t *testing.T) {
	dir := t.TempDir()
	recorded := filepath.Join(dir, "tool")
	writeExecutable(t, recorded)
	writeExecutable(t, filepath.Join(dir, "tool-copy"))

	restore := StubToolCandidates(func(names ...string) []ExecutableCandidate {
		return []ExecutableCandidate{
			{Name: "tool", Path: filepath.Join(dir, "tool-copy"), Sources: []string{"path"}},
			{Name: "tool", Path: recorded, Sources: []string{"path"}},
		}
	})
	defer restore()

	manifest := Manifest{
		ID:      "dev.floter.tool",
		Runtime: Runtime{Type: "system", ExecutableNames: []string{"tool"}},
	}
	got := SystemRuntimeCandidates(manifest, recorded)
	if len(got) != 1 || got[0].Path != recorded {
		t.Fatalf("candidates = %+v, want just the recorded program", got)
	}

	// Without a recorded program every match is a candidate, and more than one
	// is the state the chooser exists for.
	if got := SystemRuntimeCandidates(manifest, ""); len(got) != 2 {
		t.Errorf("candidates = %+v, want both matches", got)
	}
}

// A non-system runtime has nothing to choose from: its program ships with the
// package or is the interpreter the run path resolves.
func TestSystemRuntimeCandidatesIgnoreNonSystemRuntimes(t *testing.T) {
	manifest := Manifest{ID: "dev.floter.script", Runtime: Runtime{Type: "script", Language: "shell"}}
	if got := SystemRuntimeCandidates(manifest, ""); got != nil {
		t.Errorf("candidates = %+v, want none for a script runtime", got)
	}
}

// ReconnectAt with an explicit program binds to exactly that program, which
// is what the chooser's answer has to reach: the user picked it, so the search
// path is not asked again.
func TestReconnectAtBindsToTheChosenProgram(t *testing.T) {
	stale := filepath.Join(t.TempDir(), "systool")
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = stale
	})
	chosen := filepath.Join(t.TempDir(), "chosen", "systool")
	writeExecutable(t, chosen)

	report, err := ReconnectAt(paths, "dev.floter.sys", chosen)
	if err != nil {
		t.Fatalf("ReconnectAt: %v", err)
	}
	if !report.Reconnected || report.Path != chosen {
		t.Fatalf("report = %+v, want reconnected at %s", report, chosen)
	}
	if entry := entryOf(t, paths, "dev.floter.sys"); entry.ExecutablePath != chosen {
		t.Errorf("ExecutablePath = %q, want %q", entry.ExecutablePath, chosen)
	}
}

// A chosen program that is not runnable is refused the same way a missing one
// is: a binding to a file that cannot run is not a repair.
func TestReconnectAtRefusesAProgramThatCannotRun(t *testing.T) {
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = filepath.Join(t.TempDir(), "systool")
	})
	missing := filepath.Join(t.TempDir(), "systool")
	if _, err := ReconnectAt(paths, "dev.floter.sys", missing); err == nil {
		t.Fatal("ReconnectAt accepted a program that is not there")
	}
}
