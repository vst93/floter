package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// A frame resolves the tokens from every row it draws (the settings surface has
// 97 call sites). The memo is keyed by the store's version, the appearance and
// the desktop's preferences — the three things `ForPreferences` reads — so it
// must build once for a key, serve that value without touching the settings
// again, and stop serving it the moment any of the three changes: a stale
// palette is a slider that does not move.
func TestTheMemoBuildsOnceAndNeverServesAStaleLook(t *testing.T) {
	var m Memo
	// The appearance follows the system when the setting does not pin it.
	base := settings.Settings{Theme: "system"}
	loads := 0
	load := func() settings.Settings { loads++; return base }

	first := m.Tokens(1, false, ui.Preferences{}, load)
	if m.Builds() != 1 || loads != 1 {
		t.Fatalf("the first read built %d palettes and loaded the settings %d times, want 1 and 1", m.Builds(), loads)
	}
	if got := m.Tokens(1, false, ui.Preferences{}, func() settings.Settings {
		t.Fatal("a hit loaded the settings")
		return settings.Settings{}
	}); got.Theme != first.Theme {
		t.Fatalf("two reads of one key disagree: %p vs %p", got.Theme, first.Theme)
	}
	// A hit copies nothing: the settings are loaded on a miss only, which is
	// what turns 97 snapshots into one.
	if n := testing.AllocsPerRun(50, func() {
		_ = m.Tokens(1, false, ui.Preferences{}, func() settings.Settings { return base })
	}); n > 0 {
		t.Errorf("a memoized read allocates %.1f times, want 0", n)
	}

	// The settings changed: the version bumped, and the new look lands on the
	// very next read (no frame boundary needed — a preview slider is one frame).
	large := settings.Settings{Theme: "system", UIScale: "large"}
	grown := m.Tokens(2, false, ui.Preferences{}, func() settings.Settings { return large })
	if m.Builds() != 2 {
		t.Fatalf("a settings change left the memo at %d builds, want 2 (a stale key)", m.Builds())
	}
	if grown.Scale == first.Scale {
		t.Errorf("the interface step did not reach the tokens: both scale %.2f", grown.Scale)
	}

	// The appearance flipped.
	dark := m.Tokens(2, true, ui.Preferences{}, func() settings.Settings { return large })
	if m.Builds() != 3 || dark.Appearance == grown.Appearance {
		t.Fatalf("the dark read built %d palettes, appearance %v (want 3 builds and the dark look)", m.Builds(), dark.Appearance)
	}
	// The desktop asked for more contrast.
	contrast := m.Tokens(2, true, ui.Preferences{HighContrast: true}, func() settings.Settings { return large })
	if m.Builds() != 4 || contrast.Theme == dark.Theme {
		t.Fatalf("the high-contrast read built %d palettes, want 4 and its own theme", m.Builds())
	}

	// A reader whose store changed identity starts over rather than comparing
	// versions across two stores, which both begin at zero.
	m.Reset()
	if m.Builds() != 0 {
		t.Fatalf("Reset kept %d builds", m.Builds())
	}
}
