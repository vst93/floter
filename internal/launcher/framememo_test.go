package launcher

import (
	"testing"

	"floter/internal/settings"
)

// A frame reads the settings from every row it draws — the row's tokens, its
// glyphs, its badge — and each read used to be a full Snapshot (a copy of the
// whole value, the map of keys this build does not own included). The memo is
// keyed by the store's version, so it must serve the current value *and* stop
// serving it the moment the store changes: a stale frame is a slider that does
// not move.
func TestTheFramesSettingsMemoFollowsTheStore(t *testing.T) {
	a := testApp()
	before := a.settings()
	if got := a.settings(); got.Language != before.Language || got.UIScale != before.UIScale {
		t.Fatalf("two reads in one frame disagree: %+v vs %+v", got, before)
	}
	// One snapshot per version, not per row: the second read copies nothing.
	if n := testing.AllocsPerRun(50, func() { _ = a.settings() }); n > 0 {
		t.Errorf("a memoized settings read allocates %.1f times, want 0", n)
	}
	// A change lands without a frame boundary in between.
	if err := a.Store.Update(func(s *settings.Settings) { s.UIScale = "large" }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, want := a.settings().UIScale, "large"; got != want {
		t.Errorf("after the change the frame reads UIScale %q, want %q (a stale memo)", got, want)
	}
}

// The numbered rows ask for the select-result binding every frame, and
// `settings.ShortcutsOf` rebuilds the whole binding map for each answer. The
// memo is the frame's, so a rebound key must still be what the rows wear on the
// very next read.
func TestTheFramesShortcutAndBadgeMemosFollowTheBinding(t *testing.T) {
	a := testApp()
	first := a.shortcut(settings.ShortcutSelectResult)
	if first == "" {
		t.Fatal("the select-result binding is empty")
	}
	if n := testing.AllocsPerRun(50, func() { _ = a.shortcut(settings.ShortcutSelectResult) }); n > 0 {
		t.Errorf("a memoized binding read allocates %.1f times, want 0", n)
	}
	badge := a.resultNumber(1)
	if n := testing.AllocsPerRun(50, func() { _ = a.resultNumber(1) }); n > 0 {
		t.Errorf("a memoized badge allocates %.1f times, want 0", n)
	}

	if err := a.Store.Update(func(s *settings.Settings) {
		if !s.SetShortcut(settings.ShortcutSelectResult, "Alt+1") {
			t.Fatalf("SetShortcut refused Alt+1")
		}
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := a.shortcut(settings.ShortcutSelectResult); got != "Alt+1" {
		t.Fatalf("after the rebind the frame reads %q, want Alt+1 (a stale memo)", got)
	}
	if rebadged := a.resultNumber(1); rebadged == badge {
		t.Errorf("the badge still reads %q after the binding moved to Alt+1", rebadged)
	}
}
