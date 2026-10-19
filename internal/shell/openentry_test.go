package shell

import (
	"testing"

	"github.com/egoist/mygo"

	"floter/internal/settingsui"
)

// Opening the surface that is already on screen is the same surface, not a new
// entry. The report 「这边的某些快捷键会导致反复弹出终端，从而导致卡死」 is the
// storm this closes: a shortcut repeats while its key is held (30/s on a normal
// desktop), and every repeat ran the whole entry path — the terminal's
// appearance re-applied to every live session, the window shown, focused and
// invalidated — so the panel was pulled forward once per repeat and the frame
// had nothing left for the user.
func TestReopeningTheSurfaceOnScreenIsNotANewEntry(t *testing.T) {
	a, store, refreshes := newAppearanceApp(t)

	a.Open(SurfaceTerminal)
	if *refreshes != 1 {
		t.Fatalf("the first entry refreshed the appearance %d times, want 1", *refreshes)
	}
	// Nine more presses of the same key: nothing to mount, so nothing to do.
	for i := 0; i < 9; i++ {
		a.Open(SurfaceTerminal)
	}
	if *refreshes != 1 {
		t.Errorf("a repeated entry refreshed the appearance %d times, want 1", *refreshes)
	}
	if a.Surf != SurfaceTerminal {
		t.Errorf("surf = %v, want terminal", a.Surf)
	}

	// A real switch is a new entry again, both ways.
	a.Open(SurfaceLauncher)
	a.Open(SurfaceTerminal)
	if *refreshes != 2 {
		t.Errorf("a surface switch refreshed the appearance %d times, want 2", *refreshes)
	}
	_ = store
}

// The settings page is restored when the surface is entered, not on every
// press: a repeat of the shortcut that opened the settings must not undo the
// page the user has since switched to.
func TestARepeatedSettingsEntryKeepsThePageTheUserChose(t *testing.T) {
	a := newChromeTestApp(t)
	a.Open(SurfaceSettings)
	a.Settings.Page = settingsui.PageIntegrations
	a.Open(SurfaceSettings)
	if a.Settings.Page != settingsui.PageIntegrations {
		t.Errorf("page = %d, want the page the user switched to", a.Settings.Page)
	}
	// A real re-entry still restores the stored page, which is what the old
	// build's settings view did when it mounted.
	a.Open(SurfaceLauncher)
	a.Settings.Page = settingsui.PageAbout
	a.Open(SurfaceSettings)
	if a.Settings.Page != settingsui.PageGeneral {
		t.Errorf("re-entry page = %d, want the stored general page", a.Settings.Page)
	}
}

// revealNeeded is the window side of the same rule: a surface switch is seen,
// a window that is not on screen or not frontmost is brought forward, and a
// window already showing the surface the press asks for is left alone.
func TestRevealNeeded(t *testing.T) {
	cases := []struct {
		name                     string
		entering, visible, front bool
		want                     bool
	}{
		{"a surface switch is always shown", true, true, true, true},
		{"a hidden window comes forward", false, false, true, true},
		{"a window behind another app comes forward", false, true, false, true},
		{"the surface already in front is left alone", false, true, true, false},
	}
	for _, tc := range cases {
		if got := revealNeeded(tc.entering, tc.visible, tc.front); got != tc.want {
			t.Errorf("%s: revealNeeded(%v,%v,%v) = %v, want %v",
				tc.name, tc.entering, tc.visible, tc.front, got, tc.want)
		}
	}
}

// The `off` glass stop is the one stop that must not make the window opaque.
// On Windows a window's transparency is a style derived from the material it
// was built with, and that style is the window's for life, so the window is
// always built with the platform's material and the `off` stop clears only the
// backdrop behind it. macOS and Linux keep the plain "no material" spelling,
// which is their own `off` (「关了玻璃还是有磨砂」 was the Windows half).
func TestTheWindowKeepsItsMaterialOnTheOffStop(t *testing.T) {
	materials := []struct {
		goos, step string
		want       mygo.Vibrancy
	}{
		{"windows", "off", mygo.VibrancyAcrylic},
		{"windows", "regular", mygo.VibrancyAcrylic},
		{"windows", "liquid", mygo.VibrancyAcrylic},
		{"darwin", "off", mygo.VibrancyNone},
		{"darwin", "regular", mygo.VibrancyUnderWindow},
		{"darwin", "frosted", mygo.VibrancyUnderPage},
		{"darwin", "liquid", mygo.VibrancySidebar},
		{"linux", "off", mygo.VibrancyNone},
		{"linux", "regular", mygo.VibrancyNone},
	}
	for _, tc := range materials {
		if got := windowVibrancyFor(tc.goos, tc.step); got != tc.want {
			t.Errorf("windowVibrancyFor(%q, %q) = %q, want %q", tc.goos, tc.step, got, tc.want)
		}
	}
}

// What a live window is asked for: Windows clears the backdrop for `off` and
// keeps its material, everything else swaps the material itself.
func TestTheOffStopClearsTheWindowsBackdropOnly(t *testing.T) {
	cases := []struct {
		goos, step    string
		material      mygo.Vibrancy
		clearBackdrop bool
	}{
		{"windows", "off", mygo.VibrancyNone, true},
		{"windows", "regular", mygo.VibrancyAcrylic, false},
		{"darwin", "off", mygo.VibrancyNone, false},
		{"linux", "off", mygo.VibrancyNone, false},
	}
	for _, tc := range cases {
		material, clearBackdrop := windowMaterialCall(tc.goos, tc.step)
		if material != tc.material || clearBackdrop != tc.clearBackdrop {
			t.Errorf("windowMaterialCall(%q, %q) = %q,%v, want %q,%v",
				tc.goos, tc.step, material, clearBackdrop, tc.material, tc.clearBackdrop)
		}
	}
}
