package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/settings"
)

// P11-co · **一帧只拷一份设置**（终端页）。
//
// `Store.Snapshot` deep-copies the whole value, and this surface asked for it
// from four draw-time readers — the panel body, the empty page's badge, the
// bar's inks and the drawer's grid — measured at **5** copies per frame. They
// read `App.settings` now, so a frame takes one copy after a change and none
// while nothing moved.
//
// The readers that stay on `Store.Snapshot` are deliberate: the paste callback
// runs on the terminal's own goroutine (a Memo is one reader's and is not safe
// for concurrent use), and the label/options functions are read off the render
// thread too.
func TestATerminalFrameCopiesTheSettingsOnce(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{Settings: func() {}, Close: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, "floter — zsh")
	tt := render(t, a, 800, 600)

	if err := store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("the update failed: %v", err)
	}
	before := store.Snapshots()
	tt.Frame()
	if got := store.Snapshots() - before; got != 1 {
		t.Errorf("one terminal frame took %d settings snapshots, want 1", got)
	}

	before = store.Snapshots()
	tt.Frame()
	if got := store.Snapshots() - before; got != 0 {
		t.Errorf("a steady terminal frame took %d settings snapshots, want 0", got)
	}
}

// A frame after a change draws the changed value: the memo is keyed by the
// store's version, so it can never serve a value the store no longer holds.
func TestATerminalFrameDrawsTheChangedValue(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{Settings: func() {}, Close: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	tt := render(t, a, 800, 600)

	if err := store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("the update failed: %v", err)
	}
	tt.Frame()
	if got := a.settings().Language; got != "zh" {
		t.Fatalf("the frame served language %q, want zh", got)
	}
}
