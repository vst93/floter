package shell

import (
	"testing"

	"floter/internal/settings"
	"floter/internal/valuedomain"
)

// TestEveryAppIconHasArtwork keeps the tray's artwork table in step with the
// app-icon ids the picker offers.
//
// The ids are `valuedomain.AppIcons`; the PNGs are this package's. Before
// P11-w the two were held together by an `if name == "light"` written twice
// (here and in the settings page) — one more icon meant editing both, and a
// missing PNG would have shown the dark one without a word.
func TestEveryAppIconHasArtwork(t *testing.T) {
	for _, id := range valuedomain.AppIcons {
		icon, ok := trayIcons[id]
		if !ok {
			t.Errorf("app icon %q has no artwork", id)
			continue
		}
		if len(icon) == 0 {
			t.Errorf("app icon %q is empty", id)
		}
	}
	for id := range trayIcons {
		if !valuedomain.Has(valuedomain.AppIcons, id) {
			t.Errorf("artwork for %q, which is not an offered app icon", id)
		}
	}
	if len(trayIcons[valuedomain.DefaultAppIcon]) == 0 {
		t.Fatalf("the default icon %q has no artwork", valuedomain.DefaultAppIcon)
	}
}

// TestTheTwoAppIconFacesUseOneRule pins the shape that made this a round: the
// tray (which picks the bytes) and the settings page (which picks the row) must
// agree on what a stored value means, including the values neither ships.
func TestTheTwoAppIconFacesUseOneRule(t *testing.T) {
	for _, stored := range []string{"", "dark", "light", "Light", "bogus"} {
		s := settings.Default()
		s.SetExtra("app_icon", stored)
		fromSettings := storedAppIcon(s)
		if fromSettings != valuedomain.NormalizeAppIcon(stored) {
			t.Errorf("storedAppIcon(%q) = %q, want the shared rule's %q",
				stored, fromSettings, valuedomain.NormalizeAppIcon(stored))
		}
		if string(appIconBytes(stored)) != string(trayIcons[fromSettings]) {
			t.Errorf("the tray paints a different icon than the settings page names for %q", stored)
		}
	}
}
