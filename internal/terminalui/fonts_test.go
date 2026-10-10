package terminalui

import (
	"os"
	"path/filepath"
	"testing"
)

// The family key folds the spellings a machine uses: the display name, the
// file name and a hand-typed one compare equal.
func TestFontKey(t *testing.T) {
	for _, pair := range [][2]string{
		{"JetBrains Mono", "jetbrainsmono"},
		{"JetBrainsMono", "jetbrainsmono"},
		{"jetbrains mono", "jetbrainsmono"},
		{"DejaVu Sans Mono", "dejavusansmono"},
	} {
		if got := fontKey(pair[0]); got != pair[1] {
			t.Errorf("fontKey(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

// The walk reads a directory tree's font files and matches them to the
// candidates by name, style suffixes and all.
func TestCollectFontNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"JetBrainsMono-Regular.ttf",
		"DejaVuSansMono.ttf",
		"FiraCode-Bold.otf",
		"not-a-font.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "Menlo.ttc"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	collectFontNames(dir, names, 0)
	for _, want := range []string{"jetbrainsmono", "dejavusansmono", "firacode", "menlo"} {
		if !names[want] {
			t.Errorf("%q was not collected: %v", want, names)
		}
	}
	if names["notafont"] || names["not-a-font"] {
		t.Error("a non-font file was collected")
	}
}

// The picker's list always carries the current value, so a hand-set family is
// never lost.
func TestFontFamilyOptionsKeepsTheCurrent(t *testing.T) {
	options := FontFamilyOptions("My Hand Picked Face")
	found := false
	for _, option := range options {
		if option == "My Hand Picked Face" {
			found = true
		}
	}
	if !found {
		t.Errorf("the current family is missing from %v", options)
	}
	// A detected list may be shorter than the shipped fallback — that is the
	// point of probing — but it is never empty.
	if len(options) == 0 {
		t.Error("the picker has no options at all")
	}
}
