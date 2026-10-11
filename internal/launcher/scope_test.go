package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/drops"
	"floter/internal/settings"
)

// The scope marker: a plugin mode's field names its owner at the field's left,
// and the trigger word that entered the mode is gone — the marker is the only
// place the mode is named, as the old build's `.collapsed-card__scope` was.
func TestScopeMarker(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	tt.Type("clip ")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the mode did not open")
	}
	// The trigger word was consumed on entry.
	if a.Query != "" {
		t.Fatalf("the field holds %q", a.Query)
	}
	copy := a.copy()
	if !tt.HasText(copy.ScopeClipboard) {
		t.Errorf("the scope marker is missing: %q", tt.Texts())
	}
	// Typing the needle leaves the marker alone and filters the list.
	tt.Type("secret")
	tt.Frame()
	if !tt.HasText(copy.ScopeClipboard) || !tt.HasText("secret") {
		t.Errorf("the marker or the needle is missing: %q", tt.Texts())
	}
	// Leaving keeps the needle as an ordinary query, and the marker goes.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.clipboard {
		t.Error("Escape left the mode on")
	}
	if a.Query != "secret" {
		t.Errorf("the needle did not survive the dismissal: %q", a.Query)
	}
	if tt.HasText(copy.ScopeClipboard) {
		t.Errorf("the marker outlived its mode: %q", tt.Texts())
	}
	if _, _, ok := a.scopeMark(copy); ok {
		t.Error("the search page has an owner")
	}
	// A plugin mode with no trigger word in the field is still the mode it is:
	// deleting everything does not leave it.
	a.EnterClipboard()
	tt.Frame()
	if !a.clipboard || a.Query != "" {
		t.Fatalf("the mode did not reopen: %v %q", a.clipboard, a.Query)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
}

// The files mode is entered the same way, and the word is consumed with it.
func TestFilesModeConsumesItsWord(t *testing.T) {
	a := testApp()
	a.SetDropped([]drops.File{{Path: "/tmp/a.txt", Name: "a.txt", Directory: "/tmp"}})
	tt := render(t, a)
	tt.Type("files ")
	tt.Frame()
	if !a.InFilesMode() || a.Query != "" {
		t.Fatalf("the mode did not open cleanly: %v %q", a.InFilesMode(), a.Query)
	}
	_ = settings.Default
	_ = ui.KeyEscape
}
